package internal

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type cardScanSession struct {
	ID        string
	UserID    int64
	ModuleKey string
	UID       string
	ExpiresAt time.Time
	Consumed  bool
	Watchers  map[*simpleWebSocket]struct{}
}

type cardScanHub struct {
	mu       sync.Mutex
	sessions map[string]*cardScanSession
}

func newCardScanHub() *cardScanHub { return &cardScanHub{sessions: map[string]*cardScanSession{}} }

func newCardScanID() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *cardScanHub) cleanupLocked() {
	now := time.Now()
	for id, s := range h.sessions {
		if now.After(s.ExpiresAt) || s.Consumed {
			delete(h.sessions, id)
		}
	}
}

func (h *cardScanHub) create(userID int64, moduleKey string) (*cardScanSession, error) {
	id, err := newCardScanID()
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cleanupLocked()
	s := &cardScanSession{ID: id, UserID: userID, ModuleKey: moduleKey, ExpiresAt: time.Now().Add(90 * time.Second), Watchers: map[*simpleWebSocket]struct{}{}}
	h.sessions[id] = s
	copy := *s
	copy.Watchers = nil
	return &copy, nil
}

func (h *cardScanHub) subscribe(id string, userID int64, ws *simpleWebSocket) (*cardScanSession, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cleanupLocked()
	s, ok := h.sessions[id]
	if !ok || s.UserID != userID || s.Consumed {
		return nil, false
	}
	s.Watchers[ws] = struct{}{}
	copy := *s
	copy.Watchers = nil
	return &copy, true
}
func (h *cardScanHub) unsubscribe(id string, ws *simpleWebSocket) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.sessions[id]; s != nil {
		delete(s.Watchers, ws)
	}
}
func (h *cardScanHub) complete(id, moduleKey, uid string) bool {
	uid = strings.ToUpper(strings.TrimSpace(uid))
	if uid == "" {
		return false
	}
	h.mu.Lock()
	s, ok := h.sessions[id]
	if !ok || s.Consumed || time.Now().After(s.ExpiresAt) || s.ModuleKey != moduleKey {
		h.mu.Unlock()
		return false
	}
	s.UID = uid
	watchers := make([]*simpleWebSocket, 0, len(s.Watchers))
	for ws := range s.Watchers {
		watchers = append(watchers, ws)
	}
	h.mu.Unlock()
	for _, ws := range watchers {
		_ = ws.writeJSON(map[string]any{"type": "rfid", "scan_id": id, "uid": uid})
	}
	return true
}

// completePendingForModule is the transport-neutral RFID registration fallback.
// If MQTT/WebSocket command delivery is unavailable (common during Windows
// development), the module's next normal RFID event can still complete the
// already-armed Admin scan. No polling is introduced.
func (h *cardScanHub) completePendingForModule(moduleKey, uid string) (string, bool) {
	uid = strings.ToUpper(strings.TrimSpace(uid))
	if uid == "" {
		return "", false
	}
	h.mu.Lock()
	h.cleanupLocked()
	var chosen *cardScanSession
	for _, s := range h.sessions {
		if s == nil || s.Consumed || s.ModuleKey != moduleKey || s.UID != "" || time.Now().After(s.ExpiresAt) {
			continue
		}
		if chosen == nil || s.ExpiresAt.After(chosen.ExpiresAt) {
			chosen = s
		}
	}
	if chosen == nil {
		h.mu.Unlock()
		return "", false
	}
	chosen.UID = uid
	scanID := chosen.ID
	watchers := make([]*simpleWebSocket, 0, len(chosen.Watchers))
	for ws := range chosen.Watchers {
		watchers = append(watchers, ws)
	}
	h.mu.Unlock()
	for _, ws := range watchers {
		_ = ws.writeJSON(map[string]any{"type": "rfid", "scan_id": scanID, "uid": uid})
	}
	return scanID, true
}
func (h *cardScanHub) consume(id string, userID int64, uid string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cleanupLocked()
	s, ok := h.sessions[id]
	if !ok || s.UserID != userID || s.UID == "" || !strings.EqualFold(s.UID, strings.TrimSpace(uid)) {
		return false
	}
	s.Consumed = true
	return true
}

func (api *API) registerCardScanRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/card-scan/modules", api.handleCardScanModules)
	mux.HandleFunc("/api/admin/card-scan/start", api.handleCardScanStart)
	mux.HandleFunc("/api/admin/card-scan/ws", api.handleCardScanWebSocket)
}

func (api *API) handleCardScanModules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireRoles(w, r, "admin", "owner", "developer"); !ok {
		return
	}
	items := []map[string]any{}
	now := time.Now().Unix()
	for _, spec := range []struct{ kind, table string }{{"registry", "registries"}, {"hub", "hubs"}} {
		rows, err := api.DB.Query("SELECT id,name,COALESCE(location,''),status,mac_address,health_status,COALESCE(last_seen_at,0) FROM " + spec.table + " WHERE status='active' ORDER BY id")
		if err != nil {
			continue
		}
		type scannerRow struct {
			id, lastSeen                        int64
			name, location, status, mac, health string
		}
		loaded := []scannerRow{}
		for rows.Next() {
			var item scannerRow
			if err := rows.Scan(&item.id, &item.name, &item.location, &item.status, &item.mac, &item.health, &item.lastSeen); err != nil {
				_ = rows.Close()
				writeAdminDataError(w, err)
				return
			}
			loaded = append(loaded, item)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			writeAdminDataError(w, err)
			return
		}
		_ = rows.Close() // release the single SQLite connection before busy checks

		for _, item := range loaded {
			wsOnline := api.ModuleControl != nil && api.ModuleControl.online(moduleControlKey(spec.kind, item.id))
			recentEvent := item.health != "offline" && item.lastSeen > 0 && now-item.lastSeen <= 75
			busy := api.cardScanModuleBusy(spec.kind, item.id)
			online := !busy && (wsOnline || recentEvent)
			items = append(items, map[string]any{"id": item.id, "type": spec.kind, "name": item.name, "location": item.location, "status": item.status, "mac_address": item.mac, "health_status": item.health, "last_seen_at": item.lastSeen, "busy": busy, "online": online})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"modules": items})
}

func (api *API) cardScanModuleBusy(kind string, id int64) bool {
	var active int
	if kind == "hub" {
		_ = api.DB.QueryRow(`SELECT COUNT(*) FROM power_sessions WHERE hub_id=? AND status='active';`, id).Scan(&active)
	} else {
		_ = api.DB.QueryRow(`SELECT COUNT(*) FROM registry_sessions WHERE registry_id=? AND status='active';`, id).Scan(&active)
	}
	return active > 0
}

type cardScanStartRequest struct {
	ModuleType string `json:"module_type"`
	ModuleID   int64  `json:"module_id"`
}

func (api *API) handleCardScanStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	var req cardScanStartRequest
	if readJSON(r, &req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid RFID scan request"})
		return
	}
	req.ModuleType = strings.ToLower(strings.TrimSpace(req.ModuleType))
	if (req.ModuleType != "hub" && req.ModuleType != "registry") || req.ModuleID < 1 {
		writeJSON(w, 400, map[string]string{"error": "choose a Hub or Registry"})
		return
	}
	key := moduleControlKey(req.ModuleType, req.ModuleID)
	table := "hubs"
	if req.ModuleType == "registry" {
		table = "registries"
	}
	var status, health string
	var lastSeen int64
	if err := api.DB.QueryRow("SELECT status,health_status,COALESCE(last_seen_at,0) FROM "+table+" WHERE id=?;", req.ModuleID).Scan(&status, &health, &lastSeen); err != nil {
		writeJSON(w, 404, map[string]string{"error": "The selected module was not found."})
		return
	}
	if status != "active" {
		writeJSON(w, 409, map[string]string{"error": "Only active modules can be used as RFID scanners."})
		return
	}
	if api.cardScanModuleBusy(req.ModuleType, req.ModuleID) {
		writeJSON(w, 409, map[string]string{"error": "The selected module is currently serving an active electricity/top-up session."})
		return
	}
	wsOnline := api.ModuleControl != nil && api.ModuleControl.online(key)
	mqtt := newModuleMQTTBridge(api)
	recentEvent := health != "offline" && lastSeen > 0 && time.Now().Unix()-lastSeen <= 75
	mqttOnline := mqtt.enabled() && recentEvent
	if !wsOnline && !recentEvent {
		writeJSON(w, 409, map[string]string{"error": "The selected module has not contacted the server recently."})
		return
	}
	s, err := api.CardScans.create(user.ID, key)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	command := map[string]any{"type": "rfid-scan", "scan_id": s.ID, "timeout_ms": 90000}
	delivered := false
	if wsOnline && api.ModuleControl.send(key, command) > 0 {
		delivered = true
	}
	if !delivered && mqttOnline {
		topic := "electricredit/" + req.ModuleType + "/" + strconv.FormatInt(req.ModuleID, 10) + "/command"
		delivered = mqtt.publish(topic, command)
	}
	// If no push transport is available, keep the scan armed. The module's next
	// authenticated RFID event is captured by the runtime handler instead of
	// starting electricity/top-up. This keeps Windows development functional
	// without requiring Mosquitto and still uses no polling.
	delivery := "event-capture"
	if delivered {
		delivery = "push"
	}
	writeJSON(w, 201, map[string]any{"scan_id": s.ID, "expires_in": 90, "delivery": delivery})
}

func (api *API) handleCardScanWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !isWebSocketUpgrade(r) {
		writeJSON(w, 400, map[string]string{"error": "websocket upgrade required"})
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	scanID := strings.TrimSpace(r.URL.Query().Get("scan_id"))
	ws, err := upgradeSimpleWebSocket(w, r)
	if err != nil {
		return
	}
	defer ws.close()
	s, ok := api.CardScans.subscribe(scanID, user.ID, ws)
	if !ok {
		_ = ws.writeJSON(map[string]any{"type": "error", "message": "RFID scan session is unavailable or expired"})
		return
	}
	defer api.CardScans.unsubscribe(scanID, ws)
	_ = ws.writeJSON(map[string]any{"type": "ready", "scan_id": scanID, "expires_at": s.ExpiresAt.Unix()})
	if s.UID != "" {
		_ = ws.writeJSON(map[string]any{"type": "rfid", "scan_id": scanID, "uid": s.UID})
	}
	for {
		if _, err := ws.readText(); err != nil {
			if !errors.Is(err, io.EOF) {
			}
			return
		}
	}
}
