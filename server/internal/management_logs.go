package internal

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"electricredit/database"
)

type logMaintenanceRequest struct {
	Challenge string `json:"challenge"`
	Level     string `json:"level"`
	Search    string `json:"search"`
}

func (api *API) handleDeveloperLogsExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	var req logMaintenanceRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid export request"})
		return
	}
	if !api.consumeChallenge(user.ID, "logs-download", req.Challenge) {
		writeJSON(w, 403, map[string]string{"error": "challenge is incorrect or expired"})
		return
	}
	where, args := logFilterSQL(req.Level, req.Search)
	rows, err := api.DB.Query(`SELECT l.id,l.level,l.action,COALESCE(l.entity_type,''),COALESCE(l.entity_id,0),l.message,l.created_at,COALESCE(s.first_name||' '||s.last_name,''),COALESCE(s.email,'') FROM logs l LEFT JOIN superusers s ON s.id=l.actor_id`+where+` ORDER BY l.id DESC`, args...)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="electricredit-logs-%s.csv"`, time.Now().Format("20060102-150405")))
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{"ID", "Level", "Action", "Entity", "Entity ID", "Message", "Actor", "Email", "Created At"})
	for rows.Next() {
		var id, entityID, created int64
		var level, action, entity, message, actor, email string
		if err := rows.Scan(&id, &level, &action, &entity, &entityID, &message, &created, &actor, &email); err != nil {
			continue
		}
		_ = writer.Write([]string{strconv.FormatInt(id, 10), level, action, entity, strconv.FormatInt(entityID, 10), message, actor, email, time.Unix(created, 0).Format(time.RFC3339)})
	}
	if err := rows.Err(); err != nil {
		return
	}
	api.logAction(user.ID, "logs.export", "logs", 0, "Exported system logs")
}

func (api *API) handleDeveloperLogsDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	var req logMaintenanceRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid delete request"})
		return
	}
	if !api.consumeChallenge(user.ID, "logs-delete", req.Challenge) {
		writeJSON(w, 403, map[string]string{"error": "challenge is incorrect or expired"})
		return
	}
	where, args := logFilterSQL(req.Level, req.Search)
	result, err := api.DB.Exec(`DELETE FROM logs WHERE id IN (SELECT l.id FROM logs l LEFT JOIN superusers s ON s.id=l.actor_id `+where+`)`, args...)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	count, _ := result.RowsAffected()
	// Record the destructive action after deletion so the audit trail starts again immediately.
	api.logAction(user.ID, "logs.delete", "logs", 0, fmt.Sprintf("Deleted %d log records", count))
	writeJSON(w, 200, map[string]any{"deleted": count, "at": database.UnixTime()})
}

func logFilterSQL(level, search string) (string, []any) {
	level = strings.ToLower(strings.TrimSpace(level))
	search = strings.TrimSpace(search)
	where := " WHERE 1=1 "
	args := []any{}
	if level != "" && level != "all" {
		where += " AND l.level=?"
		args = append(args, level)
	}
	if search != "" {
		where += " AND (l.action LIKE ? OR l.message LIKE ? OR COALESCE(s.email,'') LIKE ? OR COALESCE(s.first_name||' '||s.last_name,'') LIKE ?)"
		like := "%" + search + "%"
		args = append(args, like, like, like, like)
	}
	return where, args
}
