package internal

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"electricredit/database"
)

type developerChallenge struct {
	Code      string
	Action    string
	UserID    int64
	ExpiresAt time.Time
}

type moduleRequest struct {
	MACAddress string `json:"mac_address"`
	Name       string `json:"name"`
	Location   string `json:"location"`
	Status     string `json:"status"`
}

type developerAccountRequest struct {
	FirstName string            `json:"first_name"`
	LastName  string            `json:"last_name"`
	Email     string            `json:"email"`
	Phone     string            `json:"phone"`
	Password  string            `json:"password"`
	Status    string            `json:"status"`
	ImageData string            `json:"image_data"`
	Links     map[string]string `json:"links"`
	IsHead    bool              `json:"is_head"`
}

type databaseChallengeRequest struct {
	Action string `json:"action"`
}

type databaseUpdateRequest struct {
	Table     string         `json:"table"`
	ID        int64          `json:"id"`
	Changes   map[string]any `json:"changes"`
	Challenge string         `json:"challenge"`
}

type databaseResetRequest struct {
	Challenge string `json:"challenge"`
}

type databaseSQLRequest struct {
	SQL       string `json:"sql"`
	Challenge string `json:"challenge"`
}

type wifiConnectRequest struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

func (api *API) registerDeveloperRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/developer/overview", api.handleDeveloperOverview)
	mux.HandleFunc("/api/developer/registries", api.handleDeveloperRegistries)
	mux.HandleFunc("/api/developer/registries/", api.handleDeveloperRegistryByID)
	mux.HandleFunc("/api/developer/hubs", api.handleDeveloperHubs)
	mux.HandleFunc("/api/developer/hubs/", api.handleDeveloperHubByID)
	mux.HandleFunc("/api/developer/module-pairing", api.handleDeveloperModulePairing)
	mux.HandleFunc("/api/developer/module-pairing/ws", api.handleDeveloperModulePairingWebSocket)
	mux.HandleFunc("/api/developer/module-files", api.handleDeveloperModuleFiles)
	mux.HandleFunc("/api/developer/module-code", api.handleDeveloperModuleCode)
	mux.HandleFunc("/api/developer/module-library", api.handleDeveloperModuleLibrary)
	mux.HandleFunc("/api/modules/connect", api.handleModuleConnect)
	mux.HandleFunc("/module-setup", api.handleModuleSetupPage)
	mux.HandleFunc("/api/developer/database/tables", api.handleDeveloperDatabaseTables)
	mux.HandleFunc("/api/developer/database/table", api.handleDeveloperDatabaseTable)
	mux.HandleFunc("/api/developer/database/download", api.handleDeveloperDatabaseDownload)
	mux.HandleFunc("/api/developer/database/challenge", api.handleDeveloperDatabaseChallenge)
	mux.HandleFunc("/api/developer/database/update", api.handleDeveloperDatabaseUpdate)
	mux.HandleFunc("/api/developer/database/reset", api.handleDeveloperDatabaseReset)
	mux.HandleFunc("/api/developer/database/sql/check", api.handleDeveloperDatabaseSQLCheck)
	mux.HandleFunc("/api/developer/database/sql/execute", api.handleDeveloperDatabaseSQLExecute)
	mux.HandleFunc("/api/developer/logs", api.handleDeveloperLogs)
	mux.HandleFunc("/api/developer/logs/export", api.handleDeveloperLogsExport)
	mux.HandleFunc("/api/developer/logs/delete", api.handleDeveloperLogsDelete)
	mux.HandleFunc("/api/developer/network", api.handleDeveloperNetwork)
	mux.HandleFunc("/api/developer/network/wifi", api.handleDeveloperWiFiConnect)
	mux.HandleFunc("/api/developer/network/scan", api.handleDeveloperWiFiScan)
	mux.HandleFunc("/api/developer/system", api.handleDeveloperSystem)
	mux.HandleFunc("/api/developer/system/ws", api.handleDeveloperSystemWebSocket)
	mux.HandleFunc("/api/developer/network/hotspot", api.handleDeveloperHotspot)
	mux.HandleFunc("/api/developer/developers", api.handleDeveloperAccounts)
	mux.HandleFunc("/api/developer/developers/", api.handleDeveloperAccountByID)
}

func (api *API) requireDeveloper(w http.ResponseWriter, r *http.Request) (authenticatedUser, bool) {
	return api.requireRoles(w, r, "developer")
}

func (api *API) isHeadDeveloper(userID int64) bool {
	var head int
	err := api.DB.QueryRow(`
        SELECT COALESCE(s.is_head_developer,0)
        FROM superusers s
        WHERE s.id=? AND s.rank='developer' AND s.status='active';
    `, userID).Scan(&head)
	return err == nil && head == 1
}

func (api *API) requireHeadDeveloper(w http.ResponseWriter, r *http.Request) (authenticatedUser, bool) {
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return authenticatedUser{}, false
	}
	if !api.isHeadDeveloper(user.ID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Head Developer access is required for this page"})
		return authenticatedUser{}, false
	}
	return user, true
}

func (api *API) handleDeveloperOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	counts := map[string]int64{}
	queries := map[string]string{
		"registries":           `SELECT COUNT(*) FROM registries;`,
		"hubs":                 `SELECT COUNT(*) FROM hubs;`,
		"connected_registries": `SELECT COUNT(*) FROM registries WHERE mac_address NOT LIKE 'PENDING-%' AND status!='disabled';`,
		"connected_hubs":       `SELECT COUNT(*) FROM hubs WHERE mac_address NOT LIKE 'PENDING-%' AND status!='disabled';`,
		"pairing_waiting":      `SELECT COUNT(*) FROM module_pairing_sessions WHERE status='waiting' AND expires_at>strftime('%s','now');`,
		"logs":                 `SELECT COUNT(*) FROM logs;`,
		"developers":           `SELECT COUNT(*) FROM superusers WHERE rank='developer';`,
	}
	for key, query := range queries {
		var value int64
		_ = api.DB.QueryRow(query).Scan(&value)
		counts[key] = value
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "head_developer": api.isHeadDeveloper(user.ID), "counts": counts})
}

func validateModuleRequest(req *moduleRequest) error {
	req.MACAddress = strings.ToUpper(strings.TrimSpace(req.MACAddress))
	req.Name = strings.TrimSpace(req.Name)
	req.Location = strings.TrimSpace(req.Location)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Name == "" || req.MACAddress == "" {
		return errors.New("name and MAC address are required")
	}
	if hw, err := net.ParseMAC(req.MACAddress); err != nil || len(hw) < 6 {
		return errors.New("enter a valid MAC address")
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Status != "active" && req.Status != "inactive" && req.Status != "disabled" {
		return errors.New("invalid module status")
	}
	return nil
}

func (api *API) moduleList(table string) ([]map[string]any, error) {
	kind := "hub"
	if table == "registries" {
		kind = "registry"
	}
	rows, err := api.DB.Query(fmt.Sprintf(`SELECT m.id,m.mac_address,m.name,COALESCE(m.location,''),m.status,m.created_at,m.updated_at,
		COALESCE(m.health_status,'offline'),COALESCE(m.last_seen_at,0),COALESCE(m.last_warning,''),
		COALESCE((SELECT ps.session_code FROM module_pairing_sessions ps WHERE ps.module_type=? AND ps.module_id=m.id AND ps.status='waiting' AND ps.expires_at>strftime('%%s','now') ORDER BY ps.id DESC LIMIT 1),'')
		FROM %s m ORDER BY m.id;`, table), kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, created, updated, lastSeen int64
		var mac, name, location, status, health, lastWarning, pairingSession string
		if err := rows.Scan(&id, &mac, &name, &location, &status, &created, &updated, &health, &lastSeen, &lastWarning, &pairingSession); err != nil {
			return nil, err
		}
		connected := !strings.HasPrefix(mac, "PENDING-")
		if !connected {
			mac = ""
		}
		out = append(out, map[string]any{"id": id, "mac_address": mac, "connected": connected, "pairing_session": pairingSession, "name": name, "location": location, "status": status, "health_status": health, "last_seen_at": lastSeen, "last_warning": lastWarning, "created_at": created, "updated_at": updated})
	}
	return out, rows.Err()
}

func (api *API) handleDeveloperRegistries(w http.ResponseWriter, r *http.Request) {
	api.handleDeveloperModules(w, r, "registries")
}
func (api *API) handleDeveloperHubs(w http.ResponseWriter, r *http.Request) {
	api.handleDeveloperModules(w, r, "hubs")
}
func (api *API) handleDeveloperModules(w http.ResponseWriter, r *http.Request, table string) {
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := api.moduleList(table)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{table: items})
	case http.MethodPost:
		var req moduleRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid module request"})
			return
		}
		if err := validateModuleRequest(&req); err != nil {
			writeAdminDataError(w, err)
			return
		}
		now := database.UnixTime()
		res, err := api.DB.Exec(fmt.Sprintf(`INSERT INTO %s(mac_address,name,location,status,created_at,updated_at) VALUES(?,?,?,?,?,?);`, table), req.MACAddress, req.Name, nullIfEmpty(req.Location), req.Status, now, now)
		if err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		id, _ := res.LastInsertId()
		api.logAction(user.ID, table+".create", strings.TrimSuffix(table, "s"), id, "Added "+req.Name)
		items, _ := api.moduleList(table)
		writeJSON(w, http.StatusCreated, map[string]any{table: items})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}
func (api *API) handleDeveloperRegistryByID(w http.ResponseWriter, r *http.Request) {
	api.handleDeveloperModuleByID(w, r, "registries", "/api/developer/registries/")
}
func (api *API) handleDeveloperHubByID(w http.ResponseWriter, r *http.Request) {
	api.handleDeveloperModuleByID(w, r, "hubs", "/api/developer/hubs/")
}
func (api *API) handleDeveloperModuleByID(w http.ResponseWriter, r *http.Request, table, prefix string) {
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "invalid module id"})
		return
	}
	kind := "hub"
	if table == "registries" {
		kind = "registry"
	}
	switch r.Method {
	case http.MethodPut:
		if !api.requireManagementActionChallenge(w, r, user.ID, "module-"+kind+"-edit") {
			return
		}
		var req moduleRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid module request"})
			return
		}
		if err := validateModuleRequest(&req); err != nil {
			writeAdminDataError(w, err)
			return
		}
		res, err := api.DB.Exec(fmt.Sprintf(`UPDATE %s SET mac_address=?,name=?,location=?,status=?,updated_at=? WHERE id=?;`, table), req.MACAddress, req.Name, nullIfEmpty(req.Location), req.Status, database.UnixTime(), id)
		if err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeJSON(w, 404, map[string]string{"error": "module not found"})
			return
		}
		api.logAction(user.ID, table+".update", strings.TrimSuffix(table, "s"), id, "Updated "+req.Name)
		writeJSON(w, 200, map[string]any{"updated": true})
	case http.MethodDelete:
		if !api.requireManagementActionChallenge(w, r, user.ID, "module-"+kind+"-delete") {
			return
		}
		var active int
		if kind == "hub" {
			_ = api.DB.QueryRow(`SELECT COUNT(*) FROM power_sessions WHERE hub_id=? AND status='active';`, id).Scan(&active)
		} else {
			_ = api.DB.QueryRow(`SELECT COUNT(*) FROM registry_sessions WHERE registry_id=? AND status='active';`, id).Scan(&active)
		}
		if active > 0 {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "This module has an active session. Terminate the session before deleting the module."})
			return
		}
		res, err := api.DB.Exec(fmt.Sprintf(`DELETE FROM %s WHERE id=?;`, table), id)
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "This module has related activity and cannot be deleted yet. Disable it instead."})
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeJSON(w, 404, map[string]string{"error": "module not found"})
			return
		}
		api.logAction(user.ID, table+".delete", strings.TrimSuffix(table, "s"), id, "Deleted module")
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodPut, http.MethodDelete)
	}
}

var hiddenDatabaseColumns = map[string]bool{"password_hash": true, "token_hash": true, "auth_token_hash": true, "otp_hash": true, "reset_token_hash": true, "profile_image_blob": true}
var protectedDatabaseColumns = map[string]bool{"id": true, "password_hash": true, "token_hash": true, "auth_token_hash": true, "otp_hash": true, "reset_token_hash": true, "profile_image_blob": true}

func (api *API) databaseTableNames() ([]string, error) {
	rows, err := api.DB.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}
func containsString(items []string, value string) bool {
	for _, v := range items {
		if v == value {
			return true
		}
	}
	return false
}

func (api *API) handleDeveloperDatabaseTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	names, err := api.databaseTableNames()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	out := []map[string]any{}
	for _, name := range names {
		var count int64
		_ = api.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s";`, strings.ReplaceAll(name, "\"", "\"\""))).Scan(&count)
		out = append(out, map[string]any{"name": name, "rows": count})
	}
	writeJSON(w, 200, map[string]any{"tables": out})
}

func (api *API) handleDeveloperDatabaseTable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	name := r.URL.Query().Get("name")
	tables, err := api.databaseTableNames()
	if err != nil || !containsString(tables, name) {
		writeJSON(w, 400, map[string]string{"error": "invalid database table"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 100
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	columns, pk, err := api.tableColumns(name)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	rows, err := api.DB.Query(fmt.Sprintf(`SELECT * FROM "%s" LIMIT ? OFFSET ?;`, strings.ReplaceAll(name, "\"", "\"\"")), limit, offset)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	defer rows.Close()
	rawCols, _ := rows.Columns()
	items := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(rawCols))
		ptr := make([]any, len(rawCols))
		for i := range values {
			ptr[i] = &values[i]
		}
		if err := rows.Scan(ptr...); err != nil {
			continue
		}
		item := map[string]any{}
		for i, col := range rawCols {
			v := values[i]
			if hiddenDatabaseColumns[col] {
				if b, ok := v.([]byte); ok {
					item[col] = fmt.Sprintf("[protected %d bytes]", len(b))
				} else if v != nil {
					item[col] = "[protected]"
				}
				continue
			}
			if b, ok := v.([]byte); ok {
				item[col] = fmt.Sprintf("[BLOB %d bytes]", len(b))
			} else {
				item[col] = v
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeAdminDataError(w, err)
		return
	}
	if name == "announcements" {
		columns = append(columns, map[string]any{"name": "announced_by", "type": "TEXT", "primary": false, "editable": false, "protected": false})
		for _, item := range items {
			staffID := parseIntLoose(item["created_by"])
			announcer := "System"
			if staffID > 0 {
				_ = api.DB.QueryRow(`SELECT TRIM(first_name||' '||last_name) FROM superusers WHERE id=?;`, staffID).Scan(&announcer)
			}
			item["announced_by"] = announcer
		}
	}
	writeJSON(w, 200, map[string]any{"table": name, "columns": columns, "primary_key": pk, "rows": items, "limit": limit, "offset": offset})
}

func (api *API) tableColumns(table string) ([]map[string]any, string, error) {
	rows, err := api.DB.Query(fmt.Sprintf(`PRAGMA table_info("%s");`, strings.ReplaceAll(table, "\"", "\"\"")))
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	cols := []map[string]any{}
	pk := ""
	for rows.Next() {
		var cid, notnull, primary int
		var name, kind string
		var def any
		if err := rows.Scan(&cid, &name, &kind, &notnull, &def, &primary); err != nil {
			return nil, "", err
		}
		editable := primary == 0 && !protectedDatabaseColumns[name]
		cols = append(cols, map[string]any{"name": name, "type": kind, "primary": primary > 0, "editable": editable, "protected": hiddenDatabaseColumns[name]})
		if primary > 0 {
			pk = name
		}
	}
	return cols, pk, rows.Err()
}

func (api *API) handleDeveloperDatabaseDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	_, _ = api.DB.Exec(`PRAGMA wal_checkpoint(FULL);`)
	path, err := filepath.Abs("database/database.db")
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="electricredit-database.db"`)
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	http.ServeFile(w, r, path)
}

func randomChallenge(length int, digitsOnly bool) (string, error) {
	const digits = "0123456789"
	// Management challenges deliberately mix letters, numbers, and symbols.
	// Ambiguous characters (0/O, 1/l/I) are omitted to reduce typing errors.
	const all = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%*-_+=?"
	alphabet := all
	if digitsOnly {
		alphabet = digits
	}
	buf := make([]byte, length)
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i, b := range raw {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}
func challengeKey(userID int64, action string) string { return fmt.Sprintf("%d:%s", userID, action) }

func managementChallengeSpec(action string) (length int, digitsOnly bool, ok bool) {
	action = strings.ToLower(strings.TrimSpace(action))
	// Bonus intentionally preserves the separately requested 8-digit flow.
	if action == "bonus" {
		return 8, true, true
	}
	specs := map[string]int{
		"edit":                   12,
		"reset":                  18,
		"sql":                    32,
		"logs-download":          10,
		"logs-delete":            18,
		"module-hub-edit":        12,
		"module-hub-delete":      14,
		"module-registry-edit":   12,
		"module-registry-delete": 14,
		"network-wifi-connect":   12,
		"network-hotspot-start":  12,
		"network-hotspot-update": 14,
		"network-hotspot-stop":   14,
		"developer-create":       14,
		"developer-edit":         14,
		"developer-delete":       16,
		"tenant-edit":            6,
		"tenant-delete":          12,
		"tenant-card-delete":     8,
	}
	length, ok = specs[action]
	return length, false, ok
}

func (api *API) issueChallenge(userID int64, action string) (string, error) {
	length, digits, ok := managementChallengeSpec(action)
	if !ok {
		return "", errors.New("unsupported challenge action")
	}
	code, err := randomChallenge(length, digits)
	if err != nil {
		return "", err
	}
	api.ManagementMu.Lock()
	defer api.ManagementMu.Unlock()
	if api.ManagementChallenges == nil {
		api.ManagementChallenges = map[string]developerChallenge{}
	}
	api.ManagementChallenges[challengeKey(userID, action)] = developerChallenge{Code: code, Action: action, UserID: userID, ExpiresAt: time.Now().Add(5 * time.Minute)}
	return code, nil
}
func (api *API) consumeChallenge(userID int64, action, code string) bool {
	api.ManagementMu.Lock()
	defer api.ManagementMu.Unlock()
	key := challengeKey(userID, action)
	ch, ok := api.ManagementChallenges[key]
	if ok {
		delete(api.ManagementChallenges, key)
	}
	return ok && time.Now().Before(ch.ExpiresAt) && ch.Code == strings.TrimSpace(code)
}

func (api *API) requireManagementActionChallenge(w http.ResponseWriter, r *http.Request, userID int64, action string) bool {
	code := strings.TrimSpace(r.Header.Get("X-ElectriCredit-Challenge"))
	if code == "" || !api.consumeChallenge(userID, action, code) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "action challenge is incorrect or expired"})
		return false
	}
	return true
}

func (api *API) handleDeveloperDatabaseChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	var req databaseChallengeRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid challenge request"})
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if _, _, allowed := managementChallengeSpec(req.Action); !allowed || req.Action == "bonus" {
		writeJSON(w, 400, map[string]string{"error": "invalid challenge action"})
		return
	}
	code, err := api.issueChallenge(user.ID, req.Action)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"challenge": code, "length": len(code), "expires_in": 300})
}

func (api *API) handleDeveloperDatabaseUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	var req databaseUpdateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid database update"})
		return
	}
	if !api.consumeChallenge(user.ID, "edit", req.Challenge) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "challenge is incorrect or expired"})
		return
	}
	tables, err := api.databaseTableNames()
	if err != nil || !containsString(tables, req.Table) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid database table"})
		return
	}
	columns, pk, err := api.tableColumns(req.Table)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	if pk != "id" || req.ID < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "this table does not support row editing from the management page"})
		return
	}
	allowed := map[string]bool{}
	for _, col := range columns {
		if editable, _ := col["editable"].(bool); editable {
			allowed[col["name"].(string)] = true
		}
	}
	keys := make([]string, 0, len(req.Changes))
	for key := range req.Changes {
		if allowed[key] && key != "updated_at" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no editable values were supplied"})
		return
	}
	sets := make([]string, 0, len(keys)+1)
	args := make([]any, 0, len(keys)+2)
	for _, key := range keys {
		sets = append(sets, fmt.Sprintf(`"%s"=?`, strings.ReplaceAll(key, `"`, `""`)))
		args = append(args, req.Changes[key])
	}
	if allowed["updated_at"] {
		sets = append(sets, `"updated_at"=?`)
		args = append(args, database.UnixTime())
	}
	args = append(args, req.ID)
	query := fmt.Sprintf(`UPDATE "%s" SET %s WHERE id=?;`, strings.ReplaceAll(req.Table, `"`, `""`), strings.Join(sets, ","))
	res, err := api.DB.Exec(query, args...)
	if err != nil {
		writeAdminDataError(w, normalizeConstraintError(err))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "database row not found"})
		return
	}
	api.logAction(user.ID, "database.edit", req.Table, req.ID, "Edited database row from Management")
	writeJSON(w, http.StatusOK, map[string]any{"updated": true})
}

func (api *API) handleDeveloperDatabaseReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	var req databaseResetRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid reset request"})
		return
	}
	if !api.consumeChallenge(user.ID, "reset", req.Challenge) {
		writeJSON(w, 403, map[string]string{"error": "challenge is incorrect or expired"})
		return
	}
	tx, err := api.DB.Begin()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	defer tx.Rollback()
	statements := []string{
		`DELETE FROM staff_group_messages;`, `DELETE FROM profile_email_otps;`, `DELETE FROM password_recovery_otps;`,
		`DELETE FROM announcements;`, `DELETE FROM logs;`, `DELETE FROM registry_topups;`, `DELETE FROM power_sessions;`, `DELETE FROM registry_sessions;`, `DELETE FROM cards;`, `DELETE FROM module_pairing_sessions;`, `DELETE FROM hubs;`, `DELETE FROM registries;`,
		`DELETE FROM login_sessions WHERE superuser_id NOT IN (SELECT id FROM superusers WHERE rank='developer');`,
		`DELETE FROM superusers WHERE rank<>'developer';`,
		`DELETE FROM users;`,
		`DELETE FROM sqlite_sequence WHERE name IN ('announcements','cards','hubs','logs','module_pairing_sessions','password_recovery_otps','power_sessions','profile_email_otps','registries','registry_sessions','registry_topups','staff_group_messages','users');`,
		`UPDATE price_configuration SET base_rate_units=1350000,tenant_rate_units=1350000,debt_limit_units=-10000000,change_note='Reset to default rate',updated_by=NULL,updated_at=strftime('%s','now') WHERE id=1;`,
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			writeAdminDataError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeAdminDataError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"reset": true, "message": "Database reset completed. Developer accounts were preserved and cleared table IDs were reset."})
}

type developerSQLSafety struct {
	Safe      bool   `json:"safe"`
	Statement string `json:"statement"`
	Mutation  bool   `json:"mutation"`
	Message   string `json:"message"`
}

func sqlSafetyTokens(query string) []string {
	replacer := strings.NewReplacer(
		"(", " ", ")", " ", ",", " ", "=", " ", "<", " ", ">", " ",
		"+", " ", "-", " ", "*", " ", "/", " ", "\\n", " ", "\\r", " ", "\\t", " ",
		"`", " ", `"`, " ", "[", " ", "]", " ", ".", " ", ":", " ", "$", " ", "@", " ",
	)
	return strings.Fields(strings.ToUpper(replacer.Replace(query)))
}

func containsSQLToken(tokens []string, wanted string) bool {
	wanted = strings.ToUpper(wanted)
	for _, token := range tokens {
		if token == wanted {
			return true
		}
	}
	return false
}

func validateDeveloperSQLText(raw string) (string, developerSQLSafety, error) {
	query := strings.TrimSpace(raw)
	safety := developerSQLSafety{Safe: false, Message: "Statement is not ready."}
	if query == "" {
		return "", safety, errors.New("enter a SQLite statement")
	}
	if len(query) > 12000 {
		return "", safety, errors.New("SQL statement exceeds the 12,000 character safety limit")
	}
	if strings.ContainsRune(query, '\x00') {
		return "", safety, errors.New("SQL statement contains an invalid control character")
	}
	if strings.Contains(query, "--") || strings.Contains(query, "/*") || strings.Contains(query, "*/") {
		return "", safety, errors.New("SQL comments are disabled in the Management console")
	}
	query = strings.TrimSpace(strings.TrimSuffix(query, ";"))
	if strings.Contains(query, ";") {
		return "", safety, errors.New("only one SQL statement can run at a time")
	}
	if strings.Contains(query, "?") {
		return "", safety, errors.New("parameter placeholders are not supported in the SQL console")
	}

	fields := strings.Fields(query)
	if len(fields) == 0 {
		return "", safety, errors.New("enter a SQLite statement")
	}
	statement := strings.ToUpper(fields[0])
	allowed := map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true}
	if !allowed[statement] {
		return "", safety, errors.New("only SELECT, INSERT, UPDATE, and DELETE statements are allowed")
	}
	tokens := sqlSafetyTokens(query)
	blocked := []string{"PRAGMA", "ATTACH", "DETACH", "VACUUM", "REINDEX", "ALTER", "DROP", "CREATE", "TRIGGER", "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT", "RELEASE", "LOAD_EXTENSION", "READFILE", "WRITEFILE", "RETURNING"}
	for _, keyword := range blocked {
		if containsSQLToken(tokens, keyword) {
			return "", safety, fmt.Errorf("%s is blocked by SQL safety", keyword)
		}
	}
	if containsSQLToken(tokens, "SQLITE_MASTER") || containsSQLToken(tokens, "SQLITE_SCHEMA") || containsSQLToken(tokens, "SQLITE_TEMP_MASTER") {
		return "", safety, errors.New("SQLite system schema access is blocked")
	}
	for column := range hiddenDatabaseColumns {
		if containsSQLToken(tokens, strings.ToUpper(column)) {
			return "", safety, fmt.Errorf("protected field %s cannot be accessed from the SQL console", column)
		}
	}
	if (statement == "UPDATE" || statement == "DELETE") && !containsSQLToken(tokens, "WHERE") {
		return "", safety, fmt.Errorf("%s statements must include a WHERE clause", statement)
	}

	safety.Safe = true
	safety.Statement = statement
	safety.Mutation = statement != "SELECT"
	if safety.Mutation {
		safety.Message = statement + " passed safety checks. A 32-character challenge is required before execution."
	} else {
		safety.Message = "SELECT passed safety checks. Results are limited to 200 rows and protected fields remain hidden."
	}
	return query, safety, nil
}

func (api *API) compileDeveloperSQL(query string) error {
	rows, err := api.DB.Query("EXPLAIN " + query)
	if err != nil {
		return err
	}
	return rows.Close()
}

func (api *API) handleDeveloperDatabaseSQLCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	var req databaseSQLRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid SQL safety request"})
		return
	}
	query, safety, err := validateDeveloperSQLText(req.SQL)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"safe": false, "statement": safety.Statement, "mutation": safety.Mutation, "message": err.Error()})
		return
	}
	if err := api.compileDeveloperSQL(query); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"safe": false, "statement": safety.Statement, "mutation": safety.Mutation, "message": normalizeConstraintError(err).Error()})
		return
	}
	writeJSON(w, http.StatusOK, safety)
}

func (api *API) handleDeveloperDatabaseSQLExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	var req databaseSQLRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid SQL execution request"})
		return
	}
	query, safety, err := validateDeveloperSQLText(req.SQL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := api.compileDeveloperSQL(query); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": normalizeConstraintError(err).Error()})
		return
	}
	if !api.consumeChallenge(user.ID, "sql", req.Challenge) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "32-character SQL challenge is incorrect or expired"})
		return
	}

	if safety.Statement == "SELECT" {
		rows, err := api.DB.Query(query)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		defer rows.Close()
		columns, err := rows.Columns()
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		items := make([]map[string]any, 0, 64)
		truncated := false
		for rows.Next() {
			if len(items) >= 200 {
				truncated = true
				break
			}
			values := make([]any, len(columns))
			ptrs := make([]any, len(columns))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				writeAdminDataError(w, err)
				return
			}
			item := make(map[string]any, len(columns))
			for i, column := range columns {
				if hiddenDatabaseColumns[column] {
					item[column] = "[protected]"
					continue
				}
				if b, ok := values[i].([]byte); ok {
					item[column] = string(b)
				} else {
					item[column] = values[i]
				}
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			writeAdminDataError(w, err)
			return
		}
		api.logAction(user.ID, "database.sql.select", "database", 0, "Executed a guarded SELECT from Management")
		writeJSON(w, http.StatusOK, map[string]any{"statement": "SELECT", "columns": columns, "rows": items, "truncated": truncated})
		return
	}

	tx, err := api.DB.Begin()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec(query)
	if err != nil {
		writeAdminDataError(w, normalizeConstraintError(err))
		return
	}
	if err := tx.Commit(); err != nil {
		writeAdminDataError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	preview := query
	if len(preview) > 180 {
		preview = preview[:180] + "…"
	}
	api.logAction(user.ID, "database.sql."+strings.ToLower(safety.Statement), "database", 0, preview)
	writeJSON(w, http.StatusOK, map[string]any{"statement": safety.Statement, "rows_affected": affected})
}

func (api *API) handleDeveloperLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	args := []any{}
	where := ` WHERE 1=1 `
	if level != "" && level != "all" {
		where += ` AND l.level=?`
		args = append(args, level)
	}
	if search != "" {
		where += ` AND (l.action LIKE ? OR l.message LIKE ? OR COALESCE(s.email,'') LIKE ?)`
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}
	args = append(args, 300)
	rows, err := api.DB.Query(`SELECT l.id,l.level,l.action,COALESCE(l.entity_type,''),COALESCE(l.entity_id,0),l.message,l.created_at,COALESCE(s.first_name||' '||s.last_name,''),COALESCE(s.email,'') FROM logs l LEFT JOIN superusers s ON s.id=l.actor_id`+where+` ORDER BY l.id DESC LIMIT ?`, args...)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, entityID, created int64
		var levelV, action, entity, message, actor, email string
		if err := rows.Scan(&id, &levelV, &action, &entity, &entityID, &message, &created, &actor, &email); err != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "level": levelV, "action": action, "entity_type": entity, "entity_id": entityID, "message": message, "created_at": created, "actor": actor, "email": email})
	}
	if err := rows.Err(); err != nil {
		writeAdminDataError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"logs": items})
}

func (api *API) handleDeveloperNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	snapshot := api.networkManagementSnapshot(r.Context(), r.URL.Query().Get("scan") == "1")
	writeJSON(w, 200, snapshot)
}
func developerNetworkSnapshot(ctx context.Context) map[string]any {
	interfaces := []map[string]any{}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		ips := []string{}
		for _, a := range addrs {
			ips = append(ips, a.String())
		}
		kind := "wired"
		low := strings.ToLower(iface.Name)
		if strings.Contains(low, "wlan") || strings.Contains(low, "wifi") || strings.Contains(low, "wireless") {
			kind = "wireless"
		}
		interfaces = append(interfaces, map[string]any{"name": iface.Name, "mac": iface.HardwareAddr.String(), "addresses": ips, "kind": kind})
	}
	wifi := []map[string]any{}
	managed := false
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("nmcli"); err == nil {
			managed = true
			cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if out, err := exec.CommandContext(cctx, "nmcli", "-t", "-f", "SSID,SIGNAL,SECURITY", "dev", "wifi", "list").Output(); err == nil {
				seen := map[string]bool{}
				for _, line := range strings.Split(string(out), "\n") {
					parts := strings.Split(line, ":")
					if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || seen[parts[0]] {
						continue
					}
					seen[parts[0]] = true
					signal := parts[1]
					security := ""
					if len(parts) > 2 {
						security = strings.Join(parts[2:], ":")
					}
					wifi = append(wifi, map[string]any{"ssid": parts[0], "signal": signal, "security": security})
				}
			}
		}
	}
	return map[string]any{"os": runtime.GOOS, "interfaces": interfaces, "wireless_networks": wifi, "wireless_control": managed, "warning": "For a Raspberry Pi server, a wired uplink is recommended when the same wireless adapter is used for the local hotspot."}
}

func (api *API) handleDeveloperSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	writeJSON(w, 200, developerSystemSnapshot())
}

func developerSystemSnapshot() map[string]any {
	host, _ := os.Hostname()
	result := map[string]any{
		"hostname": host, "os": runtime.GOOS, "architecture": runtime.GOARCH, "go_version": runtime.Version(),
		"cpu_threads": runtime.NumCPU(), "cpu_percent": nil, "temperature": "Unavailable", "temperature_c": nil,
		"gpu_name": "Unavailable", "gpu_percent": nil, "gpu_memory_total": "Unavailable", "gpu_memory_used": "Unavailable", "gpu_memory_total_bytes": int64(0), "gpu_memory_used_bytes": int64(0),
		"ram_total": "Unavailable", "ram_used": "Unavailable", "ram_total_bytes": int64(0), "ram_used_bytes": int64(0),
		"storage_total": "Unavailable", "storage_used": "Unavailable", "storage_total_bytes": int64(0), "storage_used_bytes": int64(0),
		"network_speed": "Unavailable", "network_link_mbps": float64(0), "network_rx_bytes": int64(0), "network_tx_bytes": int64(0),
		"interfaces": []map[string]any{}, "sampled_at": database.UnixTime(),
	}
	ifaces, _ := net.Interfaces()
	ifaceRows := []map[string]any{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		ips := []string{}
		for _, a := range addrs {
			ips = append(ips, a.String())
		}
		speed := ""
		if runtime.GOOS == "linux" {
			if b, err := os.ReadFile("/sys/class/net/" + iface.Name + "/speed"); err == nil {
				v := strings.TrimSpace(string(b))
				if v != "" && v != "-1" {
					speed = v + " Mbps"
					if n, _ := strconv.ParseFloat(v, 64); n > result["network_link_mbps"].(float64) {
						result["network_link_mbps"] = n
						result["network_speed"] = speed
					}
				}
			}
			for key, file := range map[string]string{"rx": "rx_bytes", "tx": "tx_bytes"} {
				if b, err := os.ReadFile("/sys/class/net/" + iface.Name + "/statistics/" + file); err == nil {
					v, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
					if key == "rx" {
						result["network_rx_bytes"] = result["network_rx_bytes"].(int64) + v
					} else {
						result["network_tx_bytes"] = result["network_tx_bytes"].(int64) + v
					}
				}
			}
		}
		ifaceRows = append(ifaceRows, map[string]any{"name": iface.Name, "mac": iface.HardwareAddr.String(), "addresses": ips, "speed": speed, "kind": classifyInterface(iface.Name)})
	}
	result["interfaces"] = ifaceRows

	if runtime.GOOS == "linux" {
		// Prefer Raspberry Pi firmware telemetry when available. Do not label an
		// arbitrary thermal zone as CPU temperature; some Linux hosts expose
		// battery/Wi-Fi/ACPI zones at thermal_zone0.
		if out, err := exec.Command("vcgencmd", "measure_temp").Output(); err == nil {
			raw := strings.TrimSpace(string(out))
			raw = strings.TrimPrefix(raw, "temp=")
			raw = strings.TrimSuffix(raw, "'C")
			if c, err := strconv.ParseFloat(raw, 64); err == nil && c > 0 && c < 125 {
				result["temperature_c"] = c
				result["temperature"] = fmt.Sprintf("%.1f °C", c)
			}
		} else if kindBytes, kindErr := os.ReadFile("/sys/class/thermal/thermal_zone0/type"); kindErr == nil {
			kind := strings.ToLower(strings.TrimSpace(string(kindBytes)))
			if strings.Contains(kind, "cpu") || strings.Contains(kind, "soc") || strings.Contains(kind, "x86_pkg") {
				if b, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
					if milli, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64); err == nil {
						c := milli / 1000
						if c > 0 && c < 125 {
							result["temperature_c"] = c
							result["temperature"] = fmt.Sprintf("%.1f °C", c)
						}
					}
				}
			}
		}
		if b, err := os.ReadFile("/proc/meminfo"); err == nil {
			var total, available int64
			for _, line := range strings.Split(string(b), "\n") {
				f := strings.Fields(line)
				if len(f) < 2 {
					continue
				}
				v, _ := strconv.ParseInt(f[1], 10, 64)
				if f[0] == "MemTotal:" {
					total = v
				}
				if f[0] == "MemAvailable:" {
					available = v
				}
			}
			if total > 0 {
				tb := total * 1024
				ub := (total - available) * 1024
				result["ram_total_bytes"] = tb
				result["ram_used_bytes"] = ub
				result["ram_total"] = humanBytes(tb)
				result["ram_used"] = humanBytes(ub)
			}
		}
		if out, err := exec.Command("df", "-k", "/").Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 1 {
				f := strings.Fields(lines[len(lines)-1])
				if len(f) >= 4 {
					total, _ := strconv.ParseInt(f[1], 10, 64)
					used, _ := strconv.ParseInt(f[2], 10, 64)
					tb := total * 1024
					ub := used * 1024
					result["storage_total_bytes"] = tb
					result["storage_used_bytes"] = ub
					result["storage_total"] = humanBytes(tb)
					result["storage_used"] = humanBytes(ub)
				}
			}
		}
		if cpu, ok := linuxCPUPercent(); ok {
			result["cpu_percent"] = cpu
		} else {
			time.Sleep(120 * time.Millisecond)
			if cpu, ok := linuxCPUPercent(); ok {
				result["cpu_percent"] = cpu
			}
		}
	} else if runtime.GOOS == "windows" {
		populateWindowsSystemStats(result)
	}
	populateGPUUtilization(result)
	if result["temperature_c"] == nil && runtime.GOOS == "linux" {
		if out, err := exec.Command("vcgencmd", "measure_temp").Output(); err == nil {
			raw := strings.TrimSpace(string(out))
			raw = strings.TrimPrefix(raw, "temp=")
			raw = strings.TrimSuffix(raw, "'C")
			if c, err := strconv.ParseFloat(raw, 64); err == nil {
				result["temperature_c"] = c
				result["temperature"] = fmt.Sprintf("%.1f °C", c)
			}
		}
	}
	return result
}

func humanBytes(bytes int64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if bytes >= gb {
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(gb))
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
}

func (api *API) handleDeveloperAccounts(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := api.listDevelopers()
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"developers": items})
	case http.MethodPost:
		if !api.isHeadDeveloper(user.ID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Only the Head Developer can add Developer accounts."})
			return
		}
		if !api.requireManagementActionChallenge(w, r, user.ID, "developer-create") {
			return
		}
		var req developerAccountRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid Developer request"})
			return
		}
		if err := validateDeveloperRequest(&req, true); err != nil {
			writeAdminDataError(w, err)
			return
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		blob, mime, err := decodeProfileImageBlob(req.ImageData)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		links := encodeProfileLinks(req.Links)
		head := 0
		if req.IsHead {
			head = 1
		}
		now := database.UnixTime()
		res, err := api.DB.Exec(`INSERT INTO superusers(first_name,last_name,email,phone,profile_image_blob,profile_image_mime,links_json,rank,password_hash,status,is_head_developer,created_at,updated_at) VALUES(?,?,?,?,?,?,?,'developer',?,?,?,?,?);`, req.FirstName, req.LastName, req.Email, nullIfEmpty(req.Phone), blob, nullIfEmpty(mime), links, hash, req.Status, head, now, now)
		if err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		sid, _ := res.LastInsertId()
		api.logAction(user.ID, "developer.create", "superuser", sid, "Added Developer account")
		writeJSON(w, 201, map[string]any{"created": true, "id": sid})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func validateDeveloperRequest(req *developerAccountRequest, passwordRequired bool) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Status == "" {
		req.Status = "active"
	}
	if req.FirstName == "" || req.LastName == "" || req.Email == "" {
		return errors.New("first name, last name, and email are required")
	}
	if passwordRequired && len(req.Password) < 8 {
		return errors.New("password must contain at least 8 characters")
	}
	if req.Password != "" && len(req.Password) < 8 {
		return errors.New("password must contain at least 8 characters")
	}
	if req.Status != "active" && req.Status != "inactive" && req.Status != "disabled" {
		return errors.New("invalid Developer status")
	}
	return nil
}

func (api *API) listDevelopers() ([]map[string]any, error) {
	rows, err := api.DB.Query(`SELECT id,first_name,last_name,email,COALESCE(phone,''),status,COALESCE(is_head_developer,0),COALESCE(profile_image,''),COALESCE(profile_image_blob,X''),COALESCE(profile_image_mime,''),COALESCE(links_json,'') FROM superusers WHERE rank='developer' ORDER BY is_head_developer DESC,id;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var sid int64
		var first, last, email, phone, status, imgPath, imgMime, linksRaw string
		var head int
		var blob []byte
		if err := rows.Scan(&sid, &first, &last, &email, &phone, &status, &head, &imgPath, &blob, &imgMime, &linksRaw); err != nil {
			return nil, err
		}
		links := map[string]string{}
		_ = json.Unmarshal([]byte(linksRaw), &links)
		imageURL := userProfileImageURL(imgPath, blob, imgMime)
		if len(blob) == 0 && (imgPath == "" || strings.Contains(strings.ToLower(imgPath), "default-image")) {
			imageURL = developerDefaultImage(first, last)
		}
		items = append(items, map[string]any{"id": sid, "first_name": first, "last_name": last, "email": email, "phone": phone, "status": status, "is_head": head == 1, "image_url": imageURL, "links": links})
	}
	return items, rows.Err()
}

func developerDefaultImage(first, last string) string {
	name := strings.ToLower(strings.TrimSpace(first + " " + last))
	assets := map[string]string{
		"joshane rhea paquibot":     "assets/developers/Joshane Rhea Paquibot.jpg",
		"jellen anos":               "assets/developers/Jellen Anos.jpg",
		"joselito jr. tambacan":     "assets/developers/Joselito Jr. Tambacan.jpg",
		"jaycob lumayag":            "assets/developers/Jaycob Lumayag.jpg",
		"jhon anthony catamco pano": "assets/developers/Jhon Anthony Pano.jpg",
	}
	if path := assets[name]; path != "" {
		return path
	}
	return "assets/default-image.png"
}

func (api *API) handleDeveloperAccountByID(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/developer/developers/"), "/"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "invalid Developer id"})
		return
	}
	var oldHead int
	var oldEmail, oldStatus string
	if err := api.DB.QueryRow(`SELECT COALESCE(is_head_developer,0),email,status FROM superusers WHERE id=? AND rank='developer';`, id).Scan(&oldHead, &oldEmail, &oldStatus); err != nil {
		writeJSON(w, 404, map[string]string{"error": "Developer account not found"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		isHeadCaller := api.isHeadDeveloper(user.ID)
		if !isHeadCaller && id != user.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "A Developer can edit only their own profile. Head Developer access is required to edit another Developer."})
			return
		}
		if !api.requireManagementActionChallenge(w, r, user.ID, "developer-edit") {
			return
		}
		var req developerAccountRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid Developer request"})
			return
		}
		if err := validateDeveloperRequest(&req, false); err != nil {
			writeAdminDataError(w, err)
			return
		}
		if !isHeadCaller {
			// Self-service on Management is intentionally limited to profile fields.
			// Email/password/access/status changes stay in the protected Account/Profile flows.
			req.Email = oldEmail
			req.Status = oldStatus
			req.IsHead = oldHead == 1
			req.Password = ""
		}
		if oldHead == 1 && !req.IsHead {
			var heads int64
			_ = api.DB.QueryRow(`SELECT COUNT(*) FROM superusers WHERE rank='developer' AND is_head_developer=1 AND id<>?;`, id).Scan(&heads)
			if heads == 0 {
				writeJSON(w, 409, map[string]string{"error": "At least one Head Developer must remain."})
				return
			}
		}
		tx, err := api.DB.Begin()
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		defer tx.Rollback()
		blob, mime, err := decodeProfileImageBlob(req.ImageData)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		links := encodeProfileLinks(req.Links)
		now := database.UnixTime()
		head := 0
		if req.IsHead {
			head = 1
		}
		if len(blob) > 0 {
			_, err = tx.Exec(`UPDATE superusers SET first_name=?,last_name=?,email=?,phone=?,profile_image_blob=?,profile_image_mime=?,links_json=?,status=?,is_head_developer=?,updated_at=? WHERE id=?;`, req.FirstName, req.LastName, req.Email, nullIfEmpty(req.Phone), blob, mime, links, req.Status, head, now, id)
		} else {
			_, err = tx.Exec(`UPDATE superusers SET first_name=?,last_name=?,email=?,phone=?,links_json=?,status=?,is_head_developer=?,updated_at=? WHERE id=?;`, req.FirstName, req.LastName, req.Email, nullIfEmpty(req.Phone), links, req.Status, head, now, id)
		}
		if err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		if req.Password != "" {
			hash, err := hashPassword(req.Password)
			if err != nil {
				writeAdminDataError(w, err)
				return
			}
			if _, err := tx.Exec(`UPDATE superusers SET password_hash=? WHERE id=?;`, hash, id); err != nil {
				writeAdminDataError(w, err)
				return
			}
			_, _ = tx.Exec(`DELETE FROM login_sessions WHERE superuser_id=?;`, id)
		}
		if err := tx.Commit(); err != nil {
			writeAdminDataError(w, err)
			return
		}
		api.logAction(user.ID, "developer.update", "superuser", id, "Updated Developer account")
		writeJSON(w, 200, map[string]any{"updated": true})
	case http.MethodDelete:
		if !api.isHeadDeveloper(user.ID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Only the Head Developer can delete Developer accounts."})
			return
		}
		if !api.requireManagementActionChallenge(w, r, user.ID, "developer-delete") {
			return
		}
		if id == user.ID {
			writeJSON(w, 409, map[string]string{"error": "You cannot delete the Developer account you are currently using."})
			return
		}
		if oldHead == 1 {
			var heads int64
			_ = api.DB.QueryRow(`SELECT COUNT(*) FROM superusers WHERE rank='developer' AND is_head_developer=1 AND id<>?;`, id).Scan(&heads)
			if heads == 0 {
				writeJSON(w, 409, map[string]string{"error": "At least one Head Developer must remain."})
				return
			}
		}
		res, err := api.DB.Exec(`DELETE FROM superusers WHERE id=?;`, id)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeJSON(w, 404, map[string]string{"error": "Developer account not found"})
			return
		}
		api.logAction(user.ID, "developer.delete", "superuser", id, "Deleted Developer account")
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodPut, http.MethodDelete)
	}
}
