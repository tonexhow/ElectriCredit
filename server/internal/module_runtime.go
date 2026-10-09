package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"electricredit/database"
)

// moduleRuntimeMessage is shared by WebSocket, MQTT and the optional HTTP
// event endpoint. Work-mode communication is event-driven; no module polls the
// server for changes.
type moduleRuntimeMessage struct {
	Type          string         `json:"type"`
	Token         string         `json:"token,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	UID           string         `json:"uid,omitempty"`
	ScanID        string         `json:"scan_id,omitempty"`
	SessionID     int64          `json:"session_id,omitempty"`
	OperationID   string         `json:"operation_id,omitempty"`
	EnergyWh      int64          `json:"energy_wh_total,omitempty"`
	AmountUnits   int64          `json:"amount_units,omitempty"`
	PZEMOK        *bool          `json:"pzem_ok,omitempty"`
	RFIDOK        *bool          `json:"rfid_ok,omitempty"`
	Warning       string         `json:"warning,omitempty"`
	Sensors       map[string]any `json:"sensors,omitempty"`
}

type moduleIdentity struct {
	Kind   string
	Table  string
	ID     int64
	Status string
}

func normalizeRuntimeModule(kind string, id int64) (moduleIdentity, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if id < 1 {
		return moduleIdentity{}, errors.New("invalid module id")
	}
	switch kind {
	case "hub":
		return moduleIdentity{Kind: kind, Table: "hubs", ID: id}, nil
	case "registry":
		return moduleIdentity{Kind: kind, Table: "registries", ID: id}, nil
	default:
		return moduleIdentity{}, errors.New("invalid module type")
	}
}

func (api *API) moduleTokenDetails(kind string, id int64, token string) (valid bool, status, name, location string) {
	ident, err := normalizeRuntimeModule(kind, id)
	if err != nil || strings.TrimSpace(token) == "" {
		return false, "", "", ""
	}
	var expected string
	query := fmt.Sprintf(`SELECT COALESCE(auth_token_hash,''),status,name,COALESCE(location,'') FROM %s WHERE id=? LIMIT 1;`, ident.Table)
	if err := api.DB.QueryRow(query, id).Scan(&expected, &status, &name, &location); err != nil {
		return false, "", "", ""
	}
	valid = expected != "" && expected == hashSessionToken(strings.TrimSpace(token))
	return valid, status, name, location
}

func (api *API) authorizeModuleToken(kind string, id int64, token string) bool {
	valid, status, _, _ := api.moduleTokenDetails(kind, id, token)
	return valid && status != "disabled"
}

func (api *API) touchModule(kind string, id int64, health, warning string) {
	ident, err := normalizeRuntimeModule(kind, id)
	if err != nil {
		return
	}
	if health == "" {
		health = "ok"
	}
	if health != "ok" && health != "warning" && health != "offline" {
		health = "warning"
	}
	now := database.UnixTime()
	_, _ = api.DB.Exec(fmt.Sprintf(`UPDATE %s SET health_status=?,last_seen_at=?,last_warning=?,updated_at=? WHERE id=?;`, ident.Table), health, now, nullIfEmpty(warning), now, id)
}

func (api *API) moduleWarning(kind string, id int64, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "Module reported a hardware warning."
	}
	now := database.UnixTime()
	api.touchModule(kind, id, "warning", message)
	// A failed sensor can heartbeat repeatedly. Keep the live warning fresh but
	// avoid flooding Logs with the same fault every few seconds.
	var last int64
	_ = api.DB.QueryRow(`SELECT COALESCE(MAX(created_at),0) FROM logs WHERE action=? AND entity_type=? AND entity_id=? AND message=?;`, kind+".warning", kind, id, message).Scan(&last)
	if last > 0 && now-last < 300 {
		return
	}
	_, _ = api.DB.Exec(`INSERT INTO logs(actor_id,level,action,entity_type,entity_id,message,created_at) VALUES(NULL,'warning',?,?,?,?,?);`, kind+".warning", kind, id, message, now)
}

func (api *API) moduleCardDetails(uid string) (map[string]any, error) {
	uid = strings.ToUpper(strings.TrimSpace(uid))
	if uid == "" {
		return nil, errors.New("RFID UID is required")
	}
	var (
		cardID, userID, balance, debtLimit                                 int64
		cardUID, cardStatus, firstName, lastName, email, phone, userStatus string
		tenantRate                                                         int64
	)
	err := api.DB.QueryRow(`
        SELECT c.id,c.uid,c.user_id,c.balance_units,c.status,
               u.first_name,u.last_name,COALESCE(u.email,''),COALESCE(u.phone,''),u.status,
               COALESCE(u.debt_limit_units,(SELECT debt_limit_units FROM price_configuration WHERE id=1)),
               (SELECT tenant_rate_units FROM price_configuration WHERE id=1)
        FROM cards c JOIN users u ON u.id=c.user_id
        WHERE UPPER(c.uid)=UPPER(?) LIMIT 1;
    `, uid).Scan(&cardID, &cardUID, &userID, &balance, &cardStatus, &firstName, &lastName, &email, &phone, &userStatus, &debtLimit, &tenantRate)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]any{"valid": false, "code": "CARD_NOT_REGISTERED", "message": "Card is not registered."}, nil
	}
	if err != nil {
		return nil, err
	}
	allowed := cardStatus == "active" && userStatus == "active" && balance > debtLimit
	reason := ""
	switch {
	case cardStatus != "active":
		reason = "Card is not active."
	case userStatus != "active":
		reason = "Tenant account is not active."
	case balance <= debtLimit:
		reason = "Debit limit reached."
	}
	return map[string]any{
		"valid":   true,
		"allowed": allowed,
		"reason":  reason,
		"card": map[string]any{
			"id": cardID, "uid": cardUID, "status": cardStatus,
			"balance_units": balance, "balance": database.FormatMoneyUnits(balance),
			"debit_limit_units": debtLimit, "debit_limit": database.FormatMoneyUnits(debtLimit),
		},
		"tenant": map[string]any{
			"id": userID, "first_name": firstName, "last_name": lastName,
			"name": strings.TrimSpace(firstName + " " + lastName), "email": email, "phone": phone,
			"status": userStatus,
		},
		"rate": map[string]any{"tenant_rate_units": tenantRate, "tenant_rate": database.FormatMoneyUnits(tenantRate)},
	}, nil
}

func boolValue(v *bool) bool { return v != nil && *v }

func bearerToken(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 7 || !strings.EqualFold(value[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}

func (api *API) handleModuleRuntimeMessage(kind string, id int64, msg moduleRuntimeMessage) map[string]any {
	eventType := strings.ToLower(strings.TrimSpace(msg.Type))
	validToken, moduleStatus, moduleName, moduleLocation := api.moduleTokenDetails(kind, id, msg.Token)
	response := map[string]any{"type": eventType, "correlation_id": msg.CorrelationID, "module": map[string]any{"type": kind, "id": id, "status": moduleStatus, "name": moduleName, "location": moduleLocation}}
	if !validToken {
		response["ok"] = false
		if moduleStatus == "" {
			response["code"] = "MODULE_NOT_FOUND"
			response["message"] = "Module record was not found. Pair the module again."
		} else {
			response["code"] = "UNAUTHORIZED"
			response["message"] = "Module credentials were rejected."
		}
		return response
	}
	// Inactive is a reversible pause: the module stays paired and may heartbeat,
	// but cannot begin electricity/top-up/card transactions. Disabled is a
	// stronger administrative lock with the same runtime safety behavior.
	if eventType != "heartbeat" && eventType != "warning" && moduleStatus != "active" {
		response["ok"] = false
		if moduleStatus == "disabled" {
			response["code"] = "MODULE_DISABLED"
			response["message"] = "Module is disabled by management."
		} else {
			response["code"] = "MODULE_INACTIVE"
			response["message"] = "Module is inactive. Set it to Active before use."
		}
		return response
	}
	api.touchModule(kind, id, "ok", "")

	// Transport-neutral Admin RFID registration: when a scan is armed but the
	// server cannot push a command (for example Windows without Mosquitto), the
	// next normal RFID-bearing event is captured here before it can open a Hub
	// electricity session or Registry top-up session.
	if api.CardScans != nil && (eventType == "card-present" || eventType == "card-validate" || eventType == "hub-session-start") {
		if scanID, captured := api.CardScans.completePendingForModule(moduleControlKey(kind, id), msg.UID); captured {
			response["ok"] = true
			response["accepted"] = true
			response["admin_scan"] = true
			response["scan_id"] = scanID
			response["message"] = "RFID captured for Administrator card registration."
			if kind == "hub" {
				response["power_allowed"] = false
			}
			return response
		}
	}

	switch eventType {
	case "rfid-scan-result":
		accepted := false
		if api.CardScans != nil {
			accepted = api.CardScans.complete(strings.TrimSpace(msg.ScanID), moduleControlKey(kind, id), msg.UID)
		}
		response["ok"] = accepted
		response["accepted"] = accepted
		response["scan_id"] = strings.TrimSpace(msg.ScanID)
		if !accepted {
			response["code"] = "RFID_SCAN_UNAVAILABLE"
			response["message"] = "RFID scan session is unavailable, expired, or belongs to another module."
		}

	case "heartbeat":
		healthOK := true
		if kind == "hub" && msg.PZEMOK != nil && !*msg.PZEMOK {
			api.moduleWarning(kind, id, "PZEM power sensor is not responding.")
			healthOK = false
		}
		if msg.RFIDOK != nil && !*msg.RFIDOK {
			api.moduleWarning(kind, id, "RFID reader is not responding.")
			healthOK = false
		}
		if healthOK {
			response["health"] = "ok"
		} else {
			response["health"] = "warning"
			response["power_allowed"] = false
		}
		response["ok"] = true

	case "warning":
		api.moduleWarning(kind, id, msg.Warning)
		response["ok"] = true
		response["health"] = "warning"

	case "card-present", "card-validate":
		details, err := api.moduleCardDetails(msg.UID)
		if err != nil {
			response["ok"] = false
			response["code"] = "SERVER_ERROR"
			response["message"] = "Card validation failed."
			return response
		}
		response["ok"] = true
		response["result"] = details
		valid, _ := details["valid"].(bool)
		allowed, _ := details["allowed"].(bool)
		response["valid"] = valid
		response["allowed"] = allowed
		if !valid {
			response["code"] = "CARD_NOT_REGISTERED"
			response["message"] = "Card is not registered."
		}
		if kind == "hub" {
			sensorOK := boolValue(msg.PZEMOK)
			if !sensorOK {
				api.moduleWarning(kind, id, "PZEM power sensor is not responding; relay remains off.")
			}
			response["power_allowed"] = allowed && sensorOK
			response["sensor_ok"] = sensorOK
			if !sensorOK {
				response["message"] = "Power sensor unavailable. Electricity remains off."
			}
		}

	case "hub-session-start":
		if kind != "hub" {
			return runtimeError(response, "WRONG_MODULE", "Only a Hub can start an electricity session.")
		}
		if !boolValue(msg.PZEMOK) {
			api.moduleWarning(kind, id, "PZEM power sensor is not responding; session was refused.")
			return runtimeError(response, "PZEM_UNAVAILABLE", "Power sensor unavailable. Electricity remains off.")
		}
		if msg.RFIDOK != nil && !*msg.RFIDOK {
			api.moduleWarning(kind, id, "RFID reader is not responding; session was refused.")
			return runtimeError(response, "RFID_UNAVAILABLE", "RFID reader unavailable. Electricity remains off.")
		}
		details, err := api.moduleCardDetails(msg.UID)
		if err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to validate card.")
		}
		valid, _ := details["valid"].(bool)
		allowed, _ := details["allowed"].(bool)
		if !valid || !allowed {
			response["ok"] = false
			response["result"] = details
			response["power_allowed"] = false
			return response
		}
		card := details["card"].(map[string]any)
		cardID := card["id"].(int64)
		var existing int64
		err = api.DB.QueryRow(`SELECT id FROM power_sessions WHERE hub_id=? AND card_id=? AND status='active' ORDER BY id DESC LIMIT 1;`, id, cardID).Scan(&existing)
		if err == nil {
			response["ok"] = true
			response["session_id"] = existing
			response["power_allowed"] = true
			response["result"] = details
			return response
		}
		tenantRate := details["rate"].(map[string]any)["tenant_rate_units"].(int64)
		result, err := api.DB.Exec(`INSERT INTO power_sessions(card_id,hub_id,started_at,energy_wh,cost_units,rate_units,status) VALUES(?,?,?,0,0,?,'active');`, cardID, id, database.UnixTime(), tenantRate)
		if err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to start electricity session.")
		}
		sessionID, _ := result.LastInsertId()
		response["ok"] = true
		response["session_id"] = sessionID
		response["power_allowed"] = true
		response["result"] = details

	case "hub-telemetry":
		if kind != "hub" || msg.SessionID < 1 {
			return runtimeError(response, "INVALID_SESSION", "Electricity session is missing.")
		}
		if !boolValue(msg.PZEMOK) {
			api.moduleWarning(kind, id, "PZEM power sensor stopped responding during an active session.")
			_, _ = api.DB.Exec(`UPDATE power_sessions SET status='fault',ended_at=? WHERE id=? AND hub_id=? AND status='active';`, database.UnixTime(), msg.SessionID, id)
			response["ok"] = true
			response["power_allowed"] = false
			response["cutoff"] = true
			response["code"] = "PZEM_UNAVAILABLE"
			return response
		}
		if msg.RFIDOK != nil && !*msg.RFIDOK {
			api.moduleWarning(kind, id, "RFID reader stopped responding during an active session.")
			_, _ = api.DB.Exec(`UPDATE power_sessions SET status='fault',ended_at=? WHERE id=? AND hub_id=? AND status='active';`, database.UnixTime(), msg.SessionID, id)
			response["ok"] = true
			response["power_allowed"] = false
			response["cutoff"] = true
			response["code"] = "RFID_UNAVAILABLE"
			return response
		}
		if msg.EnergyWh < 0 {
			return runtimeError(response, "INVALID_READING", "Energy reading is invalid.")
		}
		tx, err := api.DB.Begin()
		if err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to update electricity usage.")
		}
		defer tx.Rollback()
		var cardID, storedWh, storedCost, sessionRate int64
		if err = tx.QueryRow(`SELECT card_id,energy_wh,cost_units,COALESCE(rate_units,(SELECT tenant_rate_units FROM price_configuration WHERE id=1)) FROM power_sessions WHERE id=? AND hub_id=? AND status='active';`, msg.SessionID, id).Scan(&cardID, &storedWh, &storedCost, &sessionRate); err != nil {
			return runtimeError(response, "SESSION_NOT_FOUND", "Active electricity session was not found.")
		}
		incomingWh := msg.EnergyWh
		if incomingWh < storedWh {
			incomingWh = storedWh
		}
		deltaWh := incomingWh - storedWh
		targetCost := incomingWh * sessionRate / 1000
		deltaCost := targetCost - storedCost
		if deltaCost < 0 {
			deltaCost = 0
		}
		if deltaWh > 0 {
			if _, err = tx.Exec(`UPDATE power_sessions SET energy_wh=?,cost_units=? WHERE id=?;`, incomingWh, targetCost, msg.SessionID); err != nil {
				return runtimeError(response, "SERVER_ERROR", "Unable to save electricity usage.")
			}
			if _, err = tx.Exec(`UPDATE cards SET balance_units=balance_units-?,updated_at=? WHERE id=?;`, deltaCost, database.UnixTime(), cardID); err != nil {
				return runtimeError(response, "SERVER_ERROR", "Unable to update card balance.")
			}
		}
		var balance, debt int64
		if err = tx.QueryRow(`SELECT c.balance_units,COALESCE(u.debt_limit_units,(SELECT debt_limit_units FROM price_configuration WHERE id=1)) FROM cards c JOIN users u ON u.id=c.user_id WHERE c.id=?;`, cardID).Scan(&balance, &debt); err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to read updated balance.")
		}
		if err = tx.Commit(); err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to commit electricity usage.")
		}
		cutoff := balance <= debt
		response["ok"] = true
		response["power_allowed"] = !cutoff
		response["cutoff"] = cutoff
		response["energy_wh"] = incomingWh
		response["delta_wh"] = deltaWh
		response["delta_cost_units"] = deltaCost
		response["balance_units"] = balance
		response["balance"] = database.FormatMoneyUnits(balance)

	case "hub-card-removed", "hub-session-stop":
		if kind != "hub" || msg.SessionID < 1 {
			return runtimeError(response, "INVALID_SESSION", "Electricity session is missing.")
		}
		// Apply a final absolute meter reading first when supplied.
		if msg.EnergyWh > 0 && boolValue(msg.PZEMOK) {
			telemetry := msg
			telemetry.Type = "hub-telemetry"
			partial := api.handleModuleRuntimeMessage(kind, id, telemetry)
			if ok, _ := partial["ok"].(bool); !ok {
				return partial
			}
		}
		_, _ = api.DB.Exec(`UPDATE power_sessions SET status=CASE WHEN status='fault' THEN status ELSE 'completed' END,ended_at=COALESCE(ended_at,?) WHERE id=? AND hub_id=?;`, database.UnixTime(), msg.SessionID, id)
		response["ok"] = true
		response["power_allowed"] = false

	case "registry-topup-start":
		if kind != "registry" {
			return runtimeError(response, "WRONG_MODULE", "Only a Registry can start coin top-up.")
		}
		details, err := api.moduleCardDetails(msg.UID)
		if err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to validate card.")
		}
		valid, _ := details["valid"].(bool)
		if !valid {
			response["ok"] = false
			response["result"] = details
			return response
		}
		card := details["card"].(map[string]any)
		tenant := details["tenant"].(map[string]any)
		if card["status"] != "active" || tenant["status"] != "active" {
			response["ok"] = false
			response["code"] = "ACCOUNT_NOT_AVAILABLE"
			response["message"] = "Tenant and RFID card must both be active before top-up can start."
			response["result"] = details
			return response
		}
		cardID := card["id"].(int64)
		var existingID, existingCardID int64
		err = api.DB.QueryRow(`SELECT id,card_id FROM registry_sessions WHERE registry_id=? AND status='active' ORDER BY id DESC LIMIT 1;`, id).Scan(&existingID, &existingCardID)
		if err == nil {
			if existingCardID != cardID {
				return runtimeError(response, "REGISTRY_BUSY", "Registry already has an active top-up session.")
			}
			response["ok"] = true
			response["session_id"] = existingID
			response["result"] = details
			return response
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return runtimeError(response, "SERVER_ERROR", "Unable to check the Registry top-up session.")
		}
		res, err := api.DB.Exec(`INSERT INTO registry_sessions(registry_id,card_id,started_at,status) VALUES(?,?,?,'active');`, id, cardID, database.UnixTime())
		if err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to start top-up session.")
		}
		sid, _ := res.LastInsertId()
		response["ok"] = true
		response["session_id"] = sid
		response["result"] = details

	case "registry-card-removed":
		if kind != "registry" || msg.SessionID < 1 {
			return runtimeError(response, "INVALID_TOPUP_SESSION", "Registry session is missing.")
		}
		_, _ = api.DB.Exec(`UPDATE registry_sessions SET status='completed',ended_at=COALESCE(ended_at,?) WHERE id=? AND registry_id=? AND status='active';`, database.UnixTime(), msg.SessionID, id)
		response["ok"] = true

	case "registry-topup-finish":
		if kind != "registry" || strings.TrimSpace(msg.OperationID) == "" || msg.AmountUnits <= 0 {
			return runtimeError(response, "INVALID_TOPUP", "Top-up result is incomplete.")
		}
		details, err := api.moduleCardDetails(msg.UID)
		if err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to validate card.")
		}
		valid, _ := details["valid"].(bool)
		if !valid {
			response["ok"] = false
			response["result"] = details
			return response
		}
		cardID := details["card"].(map[string]any)["id"].(int64)
		tx, err := api.DB.Begin()
		if err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to save top-up.")
		}
		defer tx.Rollback()
		res, err := tx.Exec(`INSERT OR IGNORE INTO registry_topups(operation_id,registry_id,card_id,amount_units,created_at) VALUES(?,?,?,?,?);`, strings.TrimSpace(msg.OperationID), id, cardID, msg.AmountUnits, database.UnixTime())
		if err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to save top-up.")
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			if _, err = tx.Exec(`UPDATE cards SET balance_units=balance_units+?,updated_at=? WHERE id=?;`, msg.AmountUnits, database.UnixTime(), cardID); err != nil {
				return runtimeError(response, "SERVER_ERROR", "Unable to update card balance.")
			}
		}
		if msg.SessionID > 0 {
			_, _ = tx.Exec(`UPDATE registry_sessions SET status='completed',ended_at=? WHERE id=? AND registry_id=?;`, database.UnixTime(), msg.SessionID, id)
		}
		var balance int64
		if err = tx.QueryRow(`SELECT balance_units FROM cards WHERE id=?;`, cardID).Scan(&balance); err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to read updated balance.")
		}
		if err = tx.Commit(); err != nil {
			return runtimeError(response, "SERVER_ERROR", "Unable to commit top-up.")
		}
		response["ok"] = true
		response["applied"] = affected > 0
		response["balance_units"] = balance
		response["balance"] = database.FormatMoneyUnits(balance)

	default:
		return runtimeError(response, "UNKNOWN_EVENT", "Unsupported module event.")
	}
	return response
}

func runtimeError(response map[string]any, code, message string) map[string]any {
	response["ok"] = false
	response["code"] = code
	response["message"] = message
	return response
}

func (api *API) registerModuleRuntimeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/modules/runtime", api.handleModuleRuntimeHTTP)
}

func (api *API) handleModuleRuntimeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	kind := strings.ToLower(strings.TrimSpace(r.Header.Get("X-ElectriCredit-Module-Type")))
	id, _ := strconv.ParseInt(strings.TrimSpace(r.Header.Get("X-ElectriCredit-Module-ID")), 10, 64)
	token := bearerToken(r.Header.Get("Authorization"))
	var msg moduleRuntimeMessage
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	if err := dec.Decode(&msg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid module event"})
		return
	}
	msg.Token = token
	response := api.handleModuleRuntimeMessage(kind, id, msg)
	if ok, _ := response["ok"].(bool); !ok && response["code"] == "UNAUTHORIZED" {
		writeJSON(w, http.StatusUnauthorized, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
