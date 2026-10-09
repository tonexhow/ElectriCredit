package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"electricredit/database"
)

type modulePairingRequest struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Location string `json:"location"`
	Status   string `json:"status"`
}

type moduleConnectRequest struct {
	SessionID   string         `json:"session_id"`
	MACAddress  string         `json:"mac_address"`
	Information map[string]any `json:"information"`
}

type hotspotSnapshot struct {
	Enabled   bool   `json:"enabled"`
	SSID      string `json:"ssid"`
	Password  string `json:"password,omitempty"`
	Interface string `json:"interface,omitempty"`
	IP        string `json:"ip,omitempty"`
}

var modulePairingWatchers = struct {
	sync.Mutex
	items map[string]map[chan struct{}]struct{}
}{items: map[string]map[chan struct{}]struct{}{}}

func subscribeModulePairing(sessionID string) (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	modulePairingWatchers.Lock()
	if modulePairingWatchers.items[sessionID] == nil {
		modulePairingWatchers.items[sessionID] = map[chan struct{}]struct{}{}
	}
	modulePairingWatchers.items[sessionID][ch] = struct{}{}
	modulePairingWatchers.Unlock()
	return ch, func() {
		modulePairingWatchers.Lock()
		if watchers := modulePairingWatchers.items[sessionID]; watchers != nil {
			delete(watchers, ch)
			if len(watchers) == 0 {
				delete(modulePairingWatchers.items, sessionID)
			}
		}
		modulePairingWatchers.Unlock()
	}
}

func notifyModulePairing(sessionID string) {
	modulePairingWatchers.Lock()
	defer modulePairingWatchers.Unlock()
	for ch := range modulePairingWatchers.items[sessionID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func normalizePairingKind(value string) (string, string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "hub":
		return "hub", "hubs", nil
	case "registry":
		return "registry", "registries", nil
	default:
		return "", "", errors.New("choose Hub or Registry")
	}
}

func normalizeModuleStatus(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = "active"
	}
	if value != "active" && value != "inactive" && value != "disabled" {
		return "", errors.New("invalid module status")
	}
	return value, nil
}

func makePairingCode() (string, error) {
	raw, err := randomChallenge(12, true)
	if err != nil {
		return "", err
	}
	return raw[:4] + "-" + raw[4:8] + "-" + raw[8:], nil
}

func pairingCodeCompact(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "-", "")
}

func normalizePairingCodeServer(value string) string {
	compact := pairingCodeCompact(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", "")))
	if len(compact) != 12 {
		return ""
	}
	for _, r := range compact {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return compact[:4] + "-" + compact[4:8] + "-" + compact[8:]
}

func (api *API) handleDeveloperModulePairing(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req modulePairingRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid module setup request"})
			return
		}
		kind, table, err := normalizePairingKind(req.Kind)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Location = strings.TrimSpace(req.Location)
		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "module name is required"})
			return
		}
		desired, err := normalizeModuleStatus(req.Status)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		code, err := makePairingCode()
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		now := database.UnixTime()
		expires := now + 30*60
		placeholderMAC := "PENDING-" + pairingCodeCompact(code)
		serverURL := preferredLocalServerURL(r)

		tx, err := api.DB.Begin()
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		defer tx.Rollback()
		result, err := tx.Exec(fmt.Sprintf(`INSERT INTO %s(mac_address,name,location,status,created_at,updated_at) VALUES(?,?,?,'inactive',?,?);`, table), placeholderMAC, req.Name, nullIfEmpty(req.Location), now, now)
		if err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		moduleID, _ := result.LastInsertId()
		_, err = tx.Exec(`INSERT INTO module_pairing_sessions(session_code,module_type,module_id,desired_status,server_url,created_by,status,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?,'waiting',?,?,?);`, code, kind, moduleID, desired, serverURL, user.ID, expires, now, now)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			writeAdminDataError(w, err)
			return
		}
		api.logAction(user.ID, kind+".pairing.start", kind, moduleID, "Started module setup for "+req.Name)
		payload := api.modulePairingPayload(code, kind, moduleID, req.Name, req.Location, desired, serverURL, expires)
		writeJSON(w, http.StatusCreated, payload)

	case http.MethodGet:
		code := normalizePairingCodeServer(r.URL.Query().Get("session_id"))
		if code == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session id is required"})
			return
		}
		payload, err := api.readModulePairing(code)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "module setup session not found"})
			return
		}
		writeJSON(w, http.StatusOK, payload)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (api *API) modulePairingPayload(code, kind string, moduleID int64, name, location, desired, serverURL string, expires int64) map[string]any {
	hotspot := detectHotspotSnapshot(context.Background())
	setup := modulePairingNetworkSnapshot(context.Background())
	serverSetupURL := fmt.Sprintf("%s/module-setup?session=%s", strings.TrimRight(serverURL, "/"), code)
	portalQuery := url.Values{}
	portalQuery.Set("session", code)
	portalQuery.Set("kind", kind)
	portalQuery.Set("connect", "/api/modules/connect")
	portalQuery.Set("runtime", "/api/modules/runtime")
	if setup.SSID != "" {
		portalQuery.Set("ssid", setup.SSID)
	}
	if setup.Password != "" {
		portalQuery.Set("wifi", setup.Password)
	}
	// A hotspot QR explicitly requests gateway discovery. This is important for
	// modules that still have an old LAN/server URL saved from a previous setup:
	// scanning a fresh Management QR must reset that stale route instead of
	// silently keeping it. Normal LAN setup carries the explicit server URL.
	if setup.Mode == "hotspot" {
		portalQuery.Set("gateway", "1")
	} else if strings.TrimSpace(serverURL) != "" {
		portalQuery.Set("server", strings.TrimRight(serverURL, "/"))
	}
	// One Management QR now contains the module network, password, server (when
	// needed), and one-time pairing session. The phone only opens 192.168.4.1;
	// the ESP32 portal imports the values and performs the actual Save & Test.
	connectionText := "http://192.168.4.1/?" + portalQuery.Encode()
	return map[string]any{
		"status":        "waiting",
		"session_id":    code,
		"module":        map[string]any{"id": moduleID, "type": kind, "name": name, "location": location, "status": "waiting", "desired_status": desired},
		"server_url":    serverURL,
		"expires_at":    expires,
		"hotspot":       map[string]any{"enabled": hotspot.Enabled, "ssid": hotspot.SSID, "password": hotspot.Password, "password_masked": maskSecret(hotspot.Password), "interface": hotspot.Interface, "ip": hotspot.IP},
		"setup_network": map[string]any{"mode": setup.Mode, "ssid": setup.SSID, "password": setup.Password, "password_masked": setup.PasswordMasked, "interface": setup.Interface, "ip": setup.IP, "active": setup.Mode != "hotspot" || hotspot.Enabled},
		"connection": map[string]any{
			"qr_text": connectionText, "session_id": code, "endpoint": serverURL + "/api/modules/connect",
			"setup_portal_url": connectionText, "server_setup_url": serverSetupURL,
		},
	}
}

func (api *API) readModulePairing(code string) (map[string]any, error) {
	var kind, desired, serverURL, status string
	var moduleID, expires int64
	var confirmed any
	err := api.DB.QueryRow(`SELECT module_type,module_id,desired_status,server_url,status,expires_at,confirmed_at FROM module_pairing_sessions WHERE session_code=?;`, code).Scan(&kind, &moduleID, &desired, &serverURL, &status, &expires, &confirmed)
	if err != nil {
		return nil, err
	}
	if status == "waiting" && database.UnixTime() > expires {
		status = "expired"
		_, _ = api.DB.Exec(`UPDATE module_pairing_sessions SET status='expired',updated_at=? WHERE session_code=? AND status='waiting';`, database.UnixTime(), code)
	}
	table := "hubs"
	if kind == "registry" {
		table = "registries"
	}
	var name, location, mac, moduleStatus string
	if err := api.DB.QueryRow(fmt.Sprintf(`SELECT name,COALESCE(location,''),mac_address,status FROM %s WHERE id=?;`, table), moduleID).Scan(&name, &location, &mac, &moduleStatus); err != nil {
		return nil, err
	}
	if strings.HasPrefix(mac, "PENDING-") {
		mac = ""
	}
	payload := api.modulePairingPayload(code, kind, moduleID, name, location, desired, serverURL, expires)
	payload["status"] = status
	payload["module"] = map[string]any{"id": moduleID, "type": kind, "name": name, "location": location, "status": moduleStatus, "desired_status": desired, "mac_address": mac}
	return payload, nil
}

func modulePairingNetworkSnapshot(ctx context.Context) setupNetworkInfo {
	// Module onboarding deliberately prefers ElectriCredit's configured setup
	// hotspot instead of the server's current uplink. The Raspberry Pi/laptop
	// may use 5 GHz for Internet, while ESP32 Hub/Registry modules require
	// 2.4 GHz. The module derives the server address from this hotspot gateway.
	hotspot := detectHotspotSnapshot(ctx)
	if hotspot.SSID != "" && hotspot.Password != "" {
		ip := hotspot.IP
		if ip == "" {
			if cfg, err := loadHotspotFileConfig(); err == nil {
				ip = strings.TrimSpace(cfg.IPv4Address)
				if slash := strings.Index(ip, "/"); slash >= 0 {
					ip = ip[:slash]
				}
			}
		}
		return setupNetworkInfo{Mode: "hotspot", SSID: hotspot.SSID, Password: hotspot.Password, PasswordMasked: maskSecret(hotspot.Password), Interface: hotspot.Interface, IP: ip, QRText: ""}
	}
	return setupNetworkSnapshot(ctx)
}

func (api *API) handleDeveloperModulePairingWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "websocket upgrade required"})
		return
	}
	if !chatWebSocketOriginAllowed(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin is not allowed"})
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	code := normalizePairingCodeServer(r.URL.Query().Get("session_id"))
	if code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid session id is required"})
		return
	}
	initial, err := api.readModulePairing(code)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "module setup session not found"})
		return
	}
	ws, err := upgradeSimpleWebSocket(w, r)
	if err != nil {
		return
	}
	defer ws.close()

	updates, unsubscribe := subscribeModulePairing(code)
	defer unsubscribe()
	closed := make(chan struct{}, 1)
	go func() {
		for {
			if _, err := ws.readText(); err != nil {
				if err != io.EOF {
					// Browser/network closes are intentionally quiet.
				}
				select {
				case closed <- struct{}{}:
				default:
				}
				return
			}
		}
	}()

	send := func(payload map[string]any) bool {
		return ws.writeJSON(map[string]any{"type": "module-pairing", "data": payload}) == nil
	}
	if !send(initial) {
		return
	}
	if status := fmt.Sprint(initial["status"]); status == "connected" || status == "expired" {
		return
	}
	var expiry <-chan time.Time
	var expiryTimer *time.Timer
	if expiresAt := parseIntLoose(initial["expires_at"]); expiresAt > 0 {
		remaining := time.Until(time.Unix(expiresAt, 0))
		if remaining > 0 {
			expiryTimer = time.NewTimer(remaining + 100*time.Millisecond)
			expiry = expiryTimer.C
			defer expiryTimer.Stop()
		}
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-closed:
			return
		case <-expiry:
			fresh, err := api.readModulePairing(code)
			if err == nil {
				_ = send(fresh)
			}
			return
		case <-updates:
			fresh, err := api.readModulePairing(code)
			if err != nil || !send(fresh) {
				return
			}
			if status := fmt.Sprint(fresh["status"]); status == "connected" || status == "expired" {
				return
			}
		}
	}
}

func (api *API) handleModuleSetupPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	code := normalizePairingCodeServer(r.URL.Query().Get("session"))
	if code == "" {
		http.Error(w, "This ElectriCredit setup session is invalid.", http.StatusBadRequest)
		return
	}
	payload, err := api.readModulePairing(code)
	if err != nil {
		http.Error(w, "This ElectriCredit setup session was not found or is no longer available.", http.StatusNotFound)
		return
	}
	module, _ := payload["module"].(map[string]any)
	serverURL := fmt.Sprint(payload["server_url"])
	status := fmt.Sprint(payload["status"])
	kind := fmt.Sprint(module["type"])
	name := fmt.Sprint(module["name"])
	location := fmt.Sprint(module["location"])
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><title>ElectriCredit Module Setup</title><style>body{margin:0;font:15px system-ui;background:#f6f6f3;color:#191919}main{width:min(560px,calc(100%% - 28px));margin:7vh auto;padding:22px;border:1px solid #ddd8c9;border-radius:20px;background:#fff;box-shadow:0 16px 45px #0001}.mark{width:52px;height:52px;display:grid;place-items:center;border-radius:16px;background:#fff3b0;font-size:24px}h1{font-size:22px;margin:14px 0 5px}p{color:#6f6a60;line-height:1.55}.code{margin:18px 0;padding:15px;border:1px dashed #b7aa76;border-radius:14px;background:#fffaf0;text-align:center;font:800 24px ui-monospace,monospace;letter-spacing:.08em}.grid{display:grid;gap:8px}.row{padding:10px 12px;border-radius:12px;background:#f7f7f4}.row small,.row strong{display:block}.row small{color:#777;font-size:11px}.steps{margin-top:18px;padding-top:14px;border-top:1px solid #eee}.status{display:inline-flex;padding:5px 9px;border-radius:999px;background:#eff8e9;color:#326b27;font-size:12px;font-weight:700}@media(prefers-color-scheme:dark){body{background:#111;color:#eee}main{background:#1a1a1a;border-color:#333}.row{background:#222}.code{background:#242017;border-color:#665c38}p,.row small{color:#aaa}}</style></head><body><main><div class="mark">⚡</div><h1>ElectriCredit module setup</h1><p>Use this page while connecting the ESP32 to the ElectriCredit local network. If the device has no camera, enter the session ID shown below in its setup screen.</p><span class="status">%s</span><div class="code">%s</div><div class="grid"><div class="row"><small>Module</small><strong>%s · %s</strong></div><div class="row"><small>Location</small><strong>%s</strong></div><div class="row"><small>Local server</small><strong>%s</strong></div></div><div class="steps"><strong>Next</strong><p>Connect the ESP32 to the ElectriCredit hotspot or the same Wi-Fi network as the server, then provide this session ID to the module. The ESP32 confirms the setup by sending its own MAC address to ElectriCredit. If the network changes later, Registration Mode remains the manual recovery path.</p></div></main></body></html>`, html.EscapeString(status), html.EscapeString(code), html.EscapeString(strings.Title(kind)), html.EscapeString(name), html.EscapeString(location), html.EscapeString(serverURL))
}

func (api *API) handleModuleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var req moduleConnectRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "INVALID_REQUEST", "error": "invalid module connection request"})
		return
	}
	req.SessionID = normalizePairingCodeServer(req.SessionID)
	req.MACAddress = strings.ToUpper(strings.TrimSpace(req.MACAddress))
	if req.SessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "INVALID_SESSION", "error": "invalid session id"})
		return
	}
	if hw, err := net.ParseMAC(req.MACAddress); err != nil || len(hw) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "INVALID_MAC", "error": "invalid module MAC address"})
		return
	}

	var kind, desired, status string
	var moduleID, expires int64
	if err := api.DB.QueryRow(`SELECT module_type,module_id,desired_status,status,expires_at FROM module_pairing_sessions WHERE session_code=?;`, req.SessionID).Scan(&kind, &moduleID, &desired, &status, &expires); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": "SESSION_NOT_FOUND", "error": "module setup session was not found"})
		return
	}
	if status == "expired" || (status == "waiting" && database.UnixTime() > expires) {
		_, _ = api.DB.Exec(`UPDATE module_pairing_sessions SET status='expired',updated_at=? WHERE session_code=? AND status='waiting';`, database.UnixTime(), req.SessionID)
		writeJSON(w, http.StatusGone, map[string]any{"code": "SESSION_EXPIRED", "error": "this setup session has expired"})
		return
	}
	if status != "waiting" {
		writeJSON(w, http.StatusConflict, map[string]any{"code": "SESSION_NOT_WAITING", "error": "this setup session is no longer waiting for a module"})
		return
	}
	reportedType := ""
	if rawType, exists := req.Information["module_type"]; exists && rawType != nil {
		reportedType = strings.ToLower(strings.TrimSpace(fmt.Sprint(rawType)))
	}
	if reportedType != "" && reportedType != kind {
		writeJSON(w, http.StatusConflict, map[string]any{"code": "WRONG_MODULE_TYPE", "error": "this setup session belongs to a different module type"})
		return
	}
	table := "hubs"
	if kind == "registry" {
		table = "registries"
	}
	moduleToken, moduleTokenHash, err := newSessionToken()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	now := database.UnixTime()
	tx, err := api.DB.Begin()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(fmt.Sprintf(`UPDATE %s SET mac_address=?,auth_token_hash=?,health_status='offline',last_seen_at=NULL,last_warning=NULL,status=?,updated_at=? WHERE id=?;`, table), req.MACAddress, moduleTokenHash, desired, now, moduleID); err != nil {
		writeAdminDataError(w, normalizeConstraintError(err))
		return
	}
	if _, err = tx.Exec(`UPDATE module_pairing_sessions SET status='connected',confirmed_at=?,updated_at=? WHERE session_code=?;`, now, now, req.SessionID); err != nil {
		writeAdminDataError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeAdminDataError(w, err)
		return
	}
	notifyModulePairing(req.SessionID)
	var moduleName, moduleLocation string
	_ = api.DB.QueryRow(fmt.Sprintf(`SELECT name,COALESCE(location,'') FROM %s WHERE id=?;`, table), moduleID).Scan(&moduleName, &moduleLocation)
	serverURL := preferredLocalServerURL(r)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"connected":    true,
		"module":       map[string]any{"id": moduleID, "type": kind, "name": moduleName, "location": moduleLocation, "status": desired, "mac_address": req.MACAddress},
		"module_token": moduleToken,
		"control": map[string]any{
			"websocket_url":  moduleWebSocketURL(serverURL, kind, moduleID),
			"http_event_url": serverURL + "/api/modules/runtime",
			"auth":           "Authorization: Bearer <module_token>",
		},
		"mqtt": moduleMQTTBootstrap(serverURL, kind, moduleID, moduleToken),
	})
}

func moduleWebSocketURL(serverURL, kind string, moduleID int64) string {
	parsed, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		parsed.Scheme = "wss"
	default:
		parsed.Scheme = "ws"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/modules/ws"
	query := parsed.Query()
	query.Set("type", kind)
	query.Set("id", strconv.FormatInt(moduleID, 10))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func preferredLocalServerURL(r *http.Request) string {
	if configured := strings.TrimRight(strings.TrimSpace(os.Getenv("ELECTRICREDIT_LOCAL_URL")), "/"); configured != "" {
		return configured
	}

	// The address the browser used to reach Management is the best identity for
	// a normal LAN setup. Prefer it before guessing among multiple interfaces.
	// Skip loopback names because an ESP32 cannot reach the server's localhost.
	if r != nil {
		host := strings.TrimSpace(r.Host)
		if host != "" {
			hostOnly := host
			if parsedHost, _, err := net.SplitHostPort(host); err == nil {
				hostOnly = parsedHost
			}
			hostOnly = strings.Trim(hostOnly, "[]")
			if !strings.EqualFold(hostOnly, "localhost") && hostOnly != "127.0.0.1" && hostOnly != "::1" {
				if !strings.Contains(host, ":") || (strings.Count(host, ":") > 1 && !strings.Contains(host, "]:")) {
					host = net.JoinHostPort(hostOnly, "8080")
				}
				return "http://" + host
			}
		}
	}

	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	hotspot := detectHotspotSnapshot(ctx)
	if hotspot.IP != "" {
		return "http://" + net.JoinHostPort(hotspot.IP, "8080")
	}
	for _, iface := range activeInterfaceSnapshots() {
		if !iface.Up {
			continue
		}
		for _, raw := range iface.Addresses {
			ip := parseInterfaceIP(raw)
			if ip != nil && ip.To4() != nil && isUsableInterfaceIP(ip) {
				return "http://" + net.JoinHostPort(ip.String(), "8080")
			}
		}
	}
	host := "127.0.0.1:8080"
	if r != nil && strings.TrimSpace(r.Host) != "" {
		host = strings.TrimSpace(r.Host)
		if !strings.Contains(host, ":") {
			host += ":8080"
		}
	}
	return "http://" + host
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 2 {
		return strings.Repeat("•", len(value))
	}
	return strings.Repeat("•", minInt(len(value), 10))
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func escapeWiFiQR(value string) string {
	r := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", ":", "\\:")
	return r.Replace(value)
}

// networkInterfaceSnapshot is deliberately small and JSON-friendly.
type networkInterfaceSnapshot struct {
	Name, MAC, Kind string
	Addresses       []string
	Up              bool
}

func activeInterfaceSnapshots() []networkInterfaceSnapshot {
	raw, _ := net.Interfaces()
	out := []networkInterfaceSnapshot{}
	for _, iface := range raw {
		addrs, _ := iface.Addrs()
		addresses := []string{}
		for _, a := range addrs {
			addresses = append(addresses, a.String())
		}
		out = append(out, networkInterfaceSnapshot{Name: iface.Name, MAC: iface.HardwareAddr.String(), Kind: classifyInterface(iface.Name), Addresses: addresses, Up: iface.Flags&net.FlagUp != 0})
	}
	return out
}
func classifyInterface(name string) string {
	low := strings.ToLower(name)
	switch {
	case strings.Contains(low, "loopback"):
		return "loopback"
	case strings.Contains(low, "wi-fi") || strings.Contains(low, "wifi") || strings.Contains(low, "wlan") || strings.Contains(low, "wireless"):
		return "wireless"
	case strings.Contains(low, "usb") || strings.Contains(low, "rndis") || strings.Contains(low, "tether") || strings.Contains(low, "mobile"):
		return "usb"
	default:
		return "ethernet"
	}
}
func parseInterfaceIP(raw string) net.IP {
	if i := strings.Index(raw, "/"); i >= 0 {
		raw = raw[:i]
	}
	return net.ParseIP(raw)
}

func detectHotspotSnapshot(ctx context.Context) hotspotSnapshot {
	cfg, cfgErr := loadHotspotFileConfig()
	h := hotspotSnapshot{}
	if cfgErr == nil {
		h.SSID = cfg.SSID
		h.Password = cfg.Password
		h.Interface = cfg.Interface
		if ip := strings.TrimSpace(cfg.IPv4Address); ip != "" {
			if slash := strings.Index(ip, "/"); slash >= 0 {
				ip = ip[:slash]
			}
			h.IP = ip
		}
	} else {
		h.SSID = strings.TrimSpace(os.Getenv("ELECTRICREDIT_HOTSPOT_SSID"))
		h.Password = os.Getenv("ELECTRICREDIT_HOTSPOT_PASSWORD")
		h.Interface = strings.TrimSpace(os.Getenv("ELECTRICREDIT_HOTSPOT_INTERFACE"))
		h.IP = strings.TrimSpace(os.Getenv("ELECTRICREDIT_HOTSPOT_IP"))
	}

	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("nmcli"); err == nil {
			connectionName := cfg.ConnectionName
			if connectionName == "" {
				connectionName = defaultHotspotConnectionName
			}
			cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
			out, err := exec.CommandContext(cctx, "nmcli", "-t", "-f", "NAME,TYPE,DEVICE", "connection", "show", "--active").Output()
			if err == nil {
				for _, line := range strings.Split(string(out), "\n") {
					parts := strings.SplitN(line, ":", 3)
					if len(parts) < 1 || parts[0] != connectionName {
						continue
					}
					h.Enabled = true
					if len(parts) >= 3 && strings.TrimSpace(parts[2]) != "" {
						h.Interface = strings.TrimSpace(parts[2])
					}
					if ssidOut, ssidErr := exec.CommandContext(cctx, "nmcli", "-g", "802-11-wireless.ssid", "connection", "show", connectionName).Output(); ssidErr == nil && strings.TrimSpace(string(ssidOut)) != "" {
						h.SSID = strings.TrimSpace(string(ssidOut))
					}
					if passOut, passErr := runManagedNMCLI(cctx, "--show-secrets", "-g", "802-11-wireless-security.psk", "connection", "show", connectionName); passErr == nil && strings.TrimSpace(string(passOut)) != "" {
						h.Password = strings.TrimSpace(string(passOut))
					}
					break
				}
			}
			cancel()
		}

		// Keep hostapd detection for older Raspberry Pi installations.
		if !h.Enabled {
			if out, err := exec.CommandContext(ctx, "systemctl", "is-active", "hostapd").Output(); err == nil && strings.TrimSpace(string(out)) == "active" {
				h.Enabled = true
			}
		}
	} else if runtime.GOOS == "windows" {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(cctx, "netsh", "wlan", "show", "hostednetwork").CombinedOutput(); err == nil {
			txt := string(out)
			if strings.Contains(strings.ToLower(txt), "status") && strings.Contains(strings.ToLower(txt), "started") {
				h.Enabled = true
			}
		}
	}

	if !h.Enabled {
		// Keep the saved hotspot profile visible even while the radio is in Wi-Fi
		// client mode. The IP is not considered active until the hotspot is up.
		h.IP = ""
		return h
	}

	for _, iface := range activeInterfaceSnapshots() {
		if h.Interface != "" && iface.Name != h.Interface {
			continue
		}
		if h.Interface == "" && classifyInterface(iface.Name) != "wireless" {
			continue
		}
		for _, a := range iface.Addresses {
			ip := parseInterfaceIP(a)
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				h.IP = ip.String()
				if h.Interface == "" {
					h.Interface = iface.Name
				}
				break
			}
		}
		if h.IP != "" {
			break
		}
	}
	return h
}

func connectedWiFiSSID(ctx context.Context) string {
	if runtime.GOOS == "windows" {
		c, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		out, err := exec.CommandContext(c, "netsh", "wlan", "show", "interfaces").CombinedOutput()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				trim := strings.TrimSpace(line)
				if strings.HasPrefix(trim, "SSID") && !strings.HasPrefix(trim, "BSSID") {
					if _, v, ok := strings.Cut(trim, ":"); ok {
						return strings.TrimSpace(v)
					}
				}
			}
		}
	}
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("nmcli"); err == nil {
			cfg, _ := loadHotspotFileConfig()
			c, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			out, err := exec.CommandContext(c, "nmcli", "-t", "-f", "NAME,TYPE", "connection", "show", "--active").Output()
			if err == nil {
				for _, line := range strings.Split(string(out), "\n") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) != 2 {
						continue
					}
					name, kind := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
					if name == "" || name == cfg.ConnectionName || (kind != "802-11-wireless" && kind != "wifi") {
						continue
					}
					ssidOut, ssidErr := exec.CommandContext(c, "nmcli", "-g", "802-11-wireless.ssid", "connection", "show", name).Output()
					if ssidErr == nil && strings.TrimSpace(string(ssidOut)) != "" {
						return strings.TrimSpace(string(ssidOut))
					}
				}
			}
		}
	}
	return ""
}

func scanWiFiNetworks(ctx context.Context) []map[string]any {
	networks := []map[string]any{}
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("nmcli"); err == nil {
			c, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			out, err := exec.CommandContext(c, "nmcli", "-t", "-f", "SSID,SIGNAL,SECURITY", "dev", "wifi", "list", "--rescan", "yes").Output()
			if err == nil {
				return parseNMCLIWiFi(string(out))
			}
		}
	}
	if runtime.GOOS == "windows" {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		out, err := exec.CommandContext(c, "netsh", "wlan", "show", "networks", "mode=bssid").CombinedOutput()
		if err == nil {
			return parseNetshWiFi(string(out))
		}
	}
	return networks
}
func parseNMCLIWiFi(text string) []map[string]any {
	seen := map[string]bool{}
	out := []map[string]any{}
	for _, line := range strings.Split(text, "\n") {
		parts := splitNMCLITerseLine(line)
		if len(parts) < 2 {
			continue
		}
		ssid := strings.TrimSpace(parts[0])
		if ssid == "" || seen[ssid] {
			continue
		}
		seen[ssid] = true
		sig := strings.TrimSpace(parts[1])
		sec := ""
		if len(parts) > 2 {
			sec = strings.Join(parts[2:], ":")
		}
		out = append(out, map[string]any{"ssid": ssid, "signal": sig, "security": sec})
	}
	return out
}
func splitNMCLITerseLine(line string) []string {
	parts := []string{}
	var current strings.Builder
	escaped := false
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ':' {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	parts = append(parts, current.String())
	return parts
}

func parseNetshWiFi(text string) []map[string]any {
	out := []map[string]any{}
	seen := map[string]bool{}
	ssid := ""
	security := ""
	signal := ""
	flush := func() {
		if ssid != "" && !seen[ssid] {
			seen[ssid] = true
			out = append(out, map[string]any{"ssid": ssid, "signal": strings.TrimSuffix(signal, "%"), "security": security})
		}
	}
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "SSID ") {
			flush()
			_, v, _ := strings.Cut(trim, ":")
			ssid = strings.TrimSpace(v)
			security = ""
			signal = ""
		} else if strings.HasPrefix(trim, "Authentication") {
			_, v, _ := strings.Cut(trim, ":")
			security = strings.TrimSpace(v)
		} else if strings.HasPrefix(trim, "Signal") {
			_, v, _ := strings.Cut(trim, ":")
			signal = strings.TrimSpace(v)
		}
	}
	flush()
	return out
}

func windowsInterfaceKindHints(ctx context.Context) map[string]string {
	hints := map[string]string{}
	if runtime.GOOS != "windows" {
		return hints
	}
	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	script := "Get-NetAdapter | ForEach-Object { \"$($_.Name)`t$($_.InterfaceDescription)\" }"
	out, err := exec.CommandContext(c, "powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return hints
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(parts) != 2 {
			continue
		}
		desc := strings.ToLower(parts[1])
		kind := "ethernet"
		if strings.Contains(desc, "wireless") || strings.Contains(desc, "wi-fi") || strings.Contains(desc, "wifi") || strings.Contains(desc, "802.11") {
			kind = "wireless"
		} else if strings.Contains(desc, "rndis") || strings.Contains(desc, "usb") || strings.Contains(desc, "tether") || strings.Contains(desc, "mobile") || strings.Contains(desc, "remote ndis") {
			kind = "usb"
		}
		hints[strings.ToLower(strings.TrimSpace(parts[0]))] = kind
	}
	return hints
}

func (api *API) networkManagementSnapshot(ctx context.Context, scan bool) map[string]any {
	ifaceRows := []map[string]any{}
	kindHints := windowsInterfaceKindHints(ctx)
	for _, i := range activeInterfaceSnapshots() {
		if hint := kindHints[strings.ToLower(i.Name)]; hint != "" {
			i.Kind = hint
		}
		connected := false
		for _, a := range i.Addresses {
			ip := parseInterfaceIP(a)
			if isUsableInterfaceIP(ip) {
				connected = true
				break
			}
		}
		ifaceRows = append(ifaceRows, map[string]any{"name": i.Name, "mac": i.MAC, "addresses": i.Addresses, "kind": i.Kind, "up": i.Up, "connected": connected})
	}
	hotspot := detectHotspotSnapshot(ctx)
	connectedSSID := connectedWiFiSSID(ctx)
	// Nearby Wi-Fi results are cached by the explicit scan operation. On a
	// single-radio Raspberry Pi, scanning while the hotspot is active requires a
	// short AP shutdown, so a normal page GET must never trigger that disruption.
	_ = scan // retained for backwards-compatible handler signatures
	scanState := currentWiFiScanState()
	connectState := currentWiFiConnectState()
	networks := cloneWiFiNetworks(scanState.Networks)
	setup := setupNetworkSnapshot(ctx)
	activeModules := 0
	if api.ModuleControl != nil {
		activeModules = api.ModuleControl.count()
	}
	wirelessControl := false
	hotspotControl := false
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("nmcli")
		wirelessControl = err == nil
		// Hotspot can use either NetworkManager or hostapd/systemctl.
		_, nmErr := exec.LookPath("nmcli")
		_, systemctlErr := exec.LookPath("systemctl")
		hotspotControl = nmErr == nil || systemctlErr == nil
	} else if runtime.GOOS == "windows" {
		_, err := exec.LookPath("netsh")
		wirelessControl = err == nil
		hotspotControl = err == nil
	}
	storedHotspot, _ := loadHotspotFileConfig()
	return map[string]any{
		"os": runtime.GOOS, "interfaces": ifaceRows, "wireless_networks": networks, "connected_wifi": connectedSSID, "wireless_control": wirelessControl, "hotspot_control": hotspotControl, "active_modules": activeModules,
		"wifi_scan": scanState, "wifi_connect": connectState,
		"setup_network": map[string]any{"mode": setup.Mode, "ssid": setup.SSID, "password_masked": setup.PasswordMasked, "interface": setup.Interface, "ip": setup.IP, "qr_text": setup.QRText},
		"hotspot": map[string]any{"enabled": hotspot.Enabled, "ssid": hotspot.SSID, "password": hotspot.Password, "password_masked": maskSecret(hotspot.Password), "interface": hotspot.Interface, "ip": hotspot.IP, "qr_text": func() string {
			if hotspot.Enabled && hotspot.SSID != "" && hotspot.Password != "" {
				return fmt.Sprintf("WIFI:T:WPA;S:%s;P:%s;;", escapeWiFiQR(hotspot.SSID), escapeWiFiQR(hotspot.Password))
			}
			return ""
		}()},
		"hotspot_config": map[string]any{"boot_enabled": storedHotspot.BootEnabled, "ssid": storedHotspot.SSID, "password": storedHotspot.Password, "password_masked": maskSecret(storedHotspot.Password), "interface": storedHotspot.Interface, "connection_name": storedHotspot.ConnectionName, "ipv4_address": storedHotspot.IPv4Address, "path": hotspotConfigFilePath()},
		"warning":        "The Raspberry Pi normally uses one Wi-Fi radio. Switching from hotspot to Wi-Fi intentionally hands that radio to the selected network; use Ethernet/desktop access during the handover when possible.",
	}
}

func encodeJSONCompact(v any) string { b, _ := json.Marshal(v); return string(b) }
func parseIntLoose(v any) int64      { n, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64); return n }
