package internal

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"electricredit/database"
)

type adminModuleControlRequest struct {
	Kind   string `json:"kind"`
	ID     int64  `json:"id"`
	Action string `json:"action"`
}

func (api *API) registerAdminModuleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/modules", api.handleAdminModules)
	mux.HandleFunc("/api/admin/modules/control", api.handleAdminModuleControl)
}

func (api *API) handleAdminModules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireRoles(w, r, "admin", "owner", "developer"); !ok {
		return
	}

	hubs, err := api.moduleList("hubs")
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	registries, err := api.moduleList("registries")
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	enrichAdminModuleSessions(api, "hub", hubs)
	enrichAdminModuleSessions(api, "registry", registries)
	writeJSON(w, http.StatusOK, map[string]any{"hubs": hubs, "registries": registries})
}

func enrichAdminModuleSessions(api *API, kind string, items []map[string]any) {
	now := database.UnixTime()
	for _, item := range items {
		id, _ := item["id"].(int64)
		var sessionID int64
		var err error
		if kind == "hub" {
			err = api.DB.QueryRow(`SELECT id FROM power_sessions WHERE hub_id=? AND status='active' ORDER BY id DESC LIMIT 1;`, id).Scan(&sessionID)
		} else {
			err = api.DB.QueryRow(`SELECT id FROM registry_sessions WHERE registry_id=? AND status='active' ORDER BY id DESC LIMIT 1;`, id).Scan(&sessionID)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			sessionID = 0
		}
		lastSeen, _ := item["last_seen_at"].(int64)
		health, _ := item["health_status"].(string)
		item["active_session_id"] = sessionID
		item["online"] = health != "offline" && lastSeen > 0 && now-lastSeen <= 60
	}
}

func (api *API) handleAdminModuleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	var req adminModuleControlRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid module control request"})
		return
	}
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if (req.Kind != "hub" && req.Kind != "registry") || req.ID < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose a valid Hub or Registry"})
		return
	}
	if req.Action != "ping" && req.Action != "terminate" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "module action must be ping or terminate"})
		return
	}

	table := "hubs"
	if req.Kind == "registry" {
		table = "registries"
	}
	var moduleName, status string
	if err := api.DB.QueryRow(fmt.Sprintf(`SELECT name,status FROM %s WHERE id=?;`, table), req.ID).Scan(&moduleName, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "module was not found"})
			return
		}
		writeAdminDataError(w, err)
		return
	}
	if status == "disabled" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "disabled modules cannot receive control commands"})
		return
	}

	var sessionID int64
	if req.Action == "terminate" {
		if req.Kind == "hub" {
			_ = api.DB.QueryRow(`SELECT id FROM power_sessions WHERE hub_id=? AND status='active' ORDER BY id DESC LIMIT 1;`, req.ID).Scan(&sessionID)
		} else {
			_ = api.DB.QueryRow(`SELECT id FROM registry_sessions WHERE registry_id=? AND status='active' ORDER BY id DESC LIMIT 1;`, req.ID).Scan(&sessionID)
		}
	}

	commandID := fmt.Sprintf("admin-%s-%s-%d-%d", req.Action, req.Kind, req.ID, time.Now().UnixNano())
	command := map[string]any{
		"type":       "command",
		"command":    req.Action,
		"command_id": commandID,
		"session_id": sessionID,
		"reason":     "administrator_request",
	}

	delivered := false
	transport := ""
	mqtt := newModuleMQTTBridge(api)
	if mqtt.enabled() {
		topic := fmt.Sprintf("electricredit/%s/%d/command", req.Kind, req.ID)
		if mqtt.publish(topic, command) {
			delivered = true
			transport = "mqtt"
		}
	}
	if !delivered && api.ModuleControl != nil {
		if api.ModuleControl.send(moduleControlKey(req.Kind, req.ID), command) > 0 {
			delivered = true
			transport = "websocket"
		}
	}
	if !delivered {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "module is not reachable through MQTT or WebSocket"})
		return
	}

	api.logAction(user.ID, "module."+req.Action, req.Kind, req.ID, strings.Title(req.Action)+" sent to "+moduleName)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"kind":       req.Kind,
		"id":         req.ID,
		"action":     req.Action,
		"command_id": commandID,
		"session_id": sessionID,
		"transport":  transport,
		"message":    strings.Title(req.Action) + " command sent to " + moduleName + ".",
		"module":     map[string]any{"id": req.ID, "name": moduleName, "status": status},
		"sequence":   strconv.FormatInt(time.Now().UnixNano(), 10),
	})
}
