package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type moduleControlHub struct {
	mu      sync.RWMutex
	clients map[string]map[*simpleWebSocket]struct{}
}

func newModuleControlHub() *moduleControlHub {
	return &moduleControlHub{clients: map[string]map[*simpleWebSocket]struct{}{}}
}

func moduleControlKey(kind string, id int64) string {
	return fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(kind)), id)
}

func (h *moduleControlHub) add(key string, ws *simpleWebSocket) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[key] == nil {
		h.clients[key] = map[*simpleWebSocket]struct{}{}
	}
	h.clients[key][ws] = struct{}{}
}

func (h *moduleControlHub) remove(key string, ws *simpleWebSocket) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.clients[key]; set != nil {
		delete(set, ws)
		if len(set) == 0 {
			delete(h.clients, key)
		}
	}
}

func (h *moduleControlHub) broadcast(message any) int {
	h.mu.RLock()
	clients := make([]*simpleWebSocket, 0)
	for _, set := range h.clients {
		for ws := range set {
			clients = append(clients, ws)
		}
	}
	h.mu.RUnlock()

	sent := 0
	for _, ws := range clients {
		if ws.writeJSON(message) == nil {
			sent++
		}
	}
	return sent
}

func (h *moduleControlHub) online(key string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[key]) > 0
}

func (h *moduleControlHub) send(key string, message any) int {
	h.mu.RLock()
	clients := make([]*simpleWebSocket, 0, len(h.clients[key]))
	for ws := range h.clients[key] {
		clients = append(clients, ws)
	}
	h.mu.RUnlock()
	sent := 0
	for _, ws := range clients {
		if ws.writeJSON(message) == nil {
			sent++
		}
	}
	return sent
}

func (h *moduleControlHub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, set := range h.clients {
		total += len(set)
	}
	return total
}

func (api *API) registerModuleControlRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/modules/ws", api.handleModuleControlWebSocket)
}

func (api *API) handleModuleControlWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "websocket upgrade required"})
		return
	}

	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	id, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("id")), 10, 64)
	// Prefer the Authorization header so long-lived module credentials are not
	// exposed in URLs, browser history, reverse-proxy logs, or diagnostics.
	// Query-token support is retained only for older firmware during migration.
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	mac := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("mac")))
	if (kind != "hub" && kind != "registry") || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "module identity is incomplete"})
		return
	}
	authorized := api.authorizeModuleToken(kind, id, token)
	if !authorized && sessionID != "" && mac != "" {
		authorized = api.authorizeModuleControl(kind, id, sessionID, mac)
	}
	if !authorized {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "module control session is not authorized"})
		return
	}

	ws, err := upgradeSimpleWebSocket(w, r)
	if err != nil {
		return
	}
	defer ws.close()
	key := moduleControlKey(kind, id)
	api.ModuleControl.add(key, ws)
	defer api.ModuleControl.remove(key, ws)

	_ = ws.writeJSON(map[string]any{
		"type":    "ready",
		"module":  map[string]any{"type": kind, "id": id},
		"network": api.currentModuleNetwork(r),
	})

	for {
		payload, err := ws.readText()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// Connection failures are expected when ESP32 changes networks.
			}
			return
		}
		var message moduleRuntimeMessage
		if json.Unmarshal(payload, &message) != nil {
			continue
		}
		messageType := strings.ToLower(strings.TrimSpace(message.Type))
		switch messageType {
		case "ping":
			api.touchModule(kind, id, "ok", "")
			_ = ws.writeJSON(map[string]any{"type": "pong"})
		case "rfid-scan-result":
			if api.CardScans != nil && api.CardScans.complete(strings.TrimSpace(message.ScanID), key, message.UID) {
				_ = ws.writeJSON(map[string]any{"type": "rfid-scan-accepted", "scan_id": message.ScanID})
			}
		default:
			message.Token = token
			response := api.handleModuleRuntimeMessage(kind, id, message)
			_ = ws.writeJSON(response)
		}
	}
}

func (api *API) authorizeModuleControl(kind string, id int64, sessionID, mac string) bool {
	table := "hubs"
	if kind == "registry" {
		table = "registries"
	}
	var storedMAC string
	query := fmt.Sprintf(`SELECT m.mac_address FROM module_pairing_sessions ps JOIN %s m ON m.id=ps.module_id WHERE ps.session_code=? AND ps.module_type=? AND ps.module_id=? AND ps.status='connected' LIMIT 1;`, table)
	if err := api.DB.QueryRow(query, sessionID, kind, id).Scan(&storedMAC); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(storedMAC), strings.TrimSpace(mac))
}

func (api *API) broadcastModuleNetworkUpdate(mode, ssid, password, serverURL string, applyAfterMS int) int {
	if api.ModuleControl == nil {
		return 0
	}
	if applyAfterMS < 1000 {
		applyAfterMS = 1000
	}
	return api.ModuleControl.broadcast(map[string]any{
		"type": "network-change",
		"network": map[string]any{
			"mode":           mode,
			"ssid":           ssid,
			"password":       password,
			"server_url":     serverURL,
			"apply_after_ms": applyAfterMS,
		},
	})
}

func (api *API) currentModuleNetwork(r *http.Request) map[string]any {
	setup := setupNetworkSnapshot(r.Context())
	return map[string]any{
		"mode":       setup.Mode,
		"ssid":       setup.SSID,
		"password":   setup.Password,
		"server_url": preferredLocalServerURL(r),
	}
}
