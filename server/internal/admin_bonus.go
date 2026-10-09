package internal

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"electricredit/database"
)

type adminBonusRequest struct {
	TargetType string  `json:"target_type"`
	Scope      string  `json:"scope"`
	IDs        []int64 `json:"ids"`
	Amount     string  `json:"amount"`
	Challenge  string  `json:"challenge"`
}

func (api *API) handleAdminBonusChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "developer")
	if !ok {
		return
	}
	code, err := api.issueChallenge(user.ID, "bonus")
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"challenge": code, "length": 8, "expires_in": 300})
}

func (api *API) handleAdminBonusApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "developer")
	if !ok {
		return
	}
	var req adminBonusRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid balance update request"})
		return
	}
	req.TargetType = strings.ToLower(strings.TrimSpace(req.TargetType))
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	if req.TargetType != "cards" && req.TargetType != "users" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose Cards or Users"})
		return
	}
	if req.Scope != "all" && req.Scope != "custom" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose All or Custom"})
		return
	}
	if req.Scope == "custom" && len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose at least one target"})
		return
	}
	amount, err := database.ParseMoneyUnits(req.Amount)
	if err != nil || amount == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter a non-zero amount with up to 5 decimal places"})
		return
	}
	if !api.consumeChallenge(user.ID, "bonus", req.Challenge) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "challenge is incorrect or expired"})
		return
	}

	tx, err := api.DB.Begin()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	defer tx.Rollback()

	result, err := applyBonusUpdate(tx, req, amount)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no RFID cards matched the selected targets"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeAdminDataError(w, err)
		return
	}

	direction := "added"
	if amount < 0 {
		direction = "deducted"
	}
	api.logAction(user.ID, "balance.bonus", "card", 0, fmt.Sprintf("%s %s on %d RFID card(s) through Developer Bonus", strings.ToUpper(direction[:1])+direction[1:], database.FormatMoneyUnits(amount), affected))
	writeJSON(w, http.StatusOK, map[string]any{"updated_cards": affected, "amount": database.FormatMoneyUnits(amount)})
}

func applyBonusUpdate(tx *sql.Tx, req adminBonusRequest, amount int64) (sql.Result, error) {
	if req.Scope == "all" {
		if req.TargetType == "cards" {
			return tx.Exec(`UPDATE cards SET balance_units=balance_units+?,updated_at=?;`, amount, database.UnixTime())
		}
		return tx.Exec(`UPDATE cards SET balance_units=balance_units+?,updated_at=? WHERE user_id IN (SELECT id FROM users);`, amount, database.UnixTime())
	}

	placeholders := make([]string, len(req.IDs))
	args := make([]any, 0, len(req.IDs)+2)
	args = append(args, amount, database.UnixTime())
	for i, id := range req.IDs {
		if id < 1 {
			return nil, fmt.Errorf("invalid selected ID")
		}
		placeholders[i] = "?"
		args = append(args, id)
	}
	joined := strings.Join(placeholders, ",")
	if req.TargetType == "cards" {
		return tx.Exec(`UPDATE cards SET balance_units=balance_units+?,updated_at=? WHERE id IN (`+joined+`);`, args...)
	}
	return tx.Exec(`UPDATE cards SET balance_units=balance_units+?,updated_at=? WHERE user_id IN (`+joined+`);`, args...)
}
