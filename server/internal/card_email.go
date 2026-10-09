package internal

import (
	"context"
	"strings"
	"time"
)

func (api *API) notifyTenantCardConnected(userID, cardID int64, uid string) {
	if api.Mailer == nil || !api.Mailer.Configured() {
		return
	}
	var email, first, last string
	if err := api.DB.QueryRow(`SELECT COALESCE(email,''),first_name,last_name FROM users WHERE id=? AND status='active';`, userID).Scan(&email, &first, &last); err != nil || strings.TrimSpace(email) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := api.Mailer.Available(ctx); err != nil {
			return
		}
		_ = api.Mailer.SendCardConnected(ctx, strings.TrimSpace(email), strings.TrimSpace(first+" "+last), cardID, uid)
	}()
}
