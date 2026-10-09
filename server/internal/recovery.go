package internal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	stdmail "net/mail"
	"strings"
	"time"

	"electricredit/database"
	appmailer "electricredit/internal/mailer"
)

const (
	recoveryOTPLifetime       = 15 * time.Minute
	recoverySendWindow        = 2 * time.Hour
	recoveryMaxSendsPerWindow = 5
	recoveryMaxVerifyAttempts = 6
)

type recoveryEmailRequest struct {
	Email string `json:"email"`
}

type recoveryVerifyRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

type recoveryResetRequest struct {
	Email           string `json:"email"`
	ResetToken      string `json:"reset_token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (api *API) registerRecoveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/recovery/status", api.handleRecoveryStatus)
	mux.HandleFunc("/api/auth/recovery/request", api.handleRecoveryRequest)
	mux.HandleFunc("/api/auth/recovery/verify", api.handleRecoveryVerify)
	mux.HandleFunc("/api/auth/recovery/reset", api.handleRecoveryReset)
}

func (api *API) handleRecoveryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	configured := api.Mailer != nil && api.Mailer.Configured()
	if !configured {
		writeJSON(w, http.StatusOK, map[string]any{
			"available":  false,
			"configured": false,
			"online":     false,
			"message":    "Email recovery is unavailable until SMTP is configured.",
		})
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	if err := api.Mailer.Available(ctx); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"available":  false,
			"configured": true,
			"online":     false,
			"message":    "ElectriCredit cannot reach the email service. Check the internet connection and try again.",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"available":  true,
		"configured": true,
		"online":     true,
	})
}

func (api *API) handleRecoveryRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var request recoveryEmailRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid recovery request",
		})
		return
	}

	email, ok := normalizeRecoveryEmail(request.Email)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "enter a valid email address",
		})
		return
	}

	if api.Mailer == nil || !api.Mailer.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "email recovery is not configured",
		})
		return
	}

	ctx, cancel := contextWithTimeout(r, 8*time.Second)
	defer cancel()

	if err := api.Mailer.Available(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "internet or email service is unavailable",
		})
		return
	}

	api.RecoveryMu.Lock()
	defer api.RecoveryMu.Unlock()

	var superuserID int64

	err := api.DB.QueryRow(`
		SELECT s.id
		FROM superusers s
		WHERE LOWER(s.email) = ? AND s.status = 'active'
		LIMIT 1;
	`, email).Scan(&superuserID)

	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "no active ElectriCredit account was found for this email",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to start account recovery",
		})
		return
	}

	now := database.UnixTime()

	_, _ = api.DB.Exec(`
		DELETE FROM password_recovery_otps
		WHERE created_at < ?
		  AND (consumed_at IS NOT NULL OR expires_at <= ?);
	`, now-int64((48*time.Hour).Seconds()), now)

	var activeExpiry int64
	err = api.DB.QueryRow(`
		SELECT expires_at
		FROM password_recovery_otps
		WHERE superuser_id = ?
		  AND consumed_at IS NULL
		  AND expires_at > ?
		ORDER BY id DESC
		LIMIT 1;
	`, superuserID, now).Scan(&activeExpiry)

	if err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":      "an account recovery OTP is already active for this email",
			"active":     true,
			"expires_at": activeExpiry,
		})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to check account recovery status",
		})
		return
	}

	windowStart := now - int64(recoverySendWindow.Seconds())
	var sendCount int
	if err := api.DB.QueryRow(`
		SELECT COUNT(*)
		FROM password_recovery_otps
		WHERE superuser_id = ?
		  AND created_at >= ?;
	`, superuserID, windowStart).Scan(&sendCount); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to check recovery send limit",
		})
		return
	}

	if sendCount >= recoveryMaxSendsPerWindow {
		var earliest int64
		_ = api.DB.QueryRow(`
			SELECT MIN(created_at)
			FROM password_recovery_otps
			WHERE superuser_id = ?
			  AND created_at >= ?;
		`, superuserID, windowStart).Scan(&earliest)

		retryAfter := int64(recoverySendWindow.Seconds())
		if earliest > 0 {
			retryAfter = earliest + int64(recoverySendWindow.Seconds()) - now
			if retryAfter < 1 {
				retryAfter = 1
			}
		}

		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":               "recovery OTP send limit reached; try again later",
			"retry_after_seconds": retryAfter,
		})
		return
	}

	otp, err := generateRecoveryOTP()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to generate account recovery OTP",
		})
		return
	}

	otpHash, err := hashPassword(otp)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to secure account recovery OTP",
		})
		return
	}

	expiresAt := time.Now().Add(recoveryOTPLifetime).UTC()
	expiresUnix := expiresAt.Unix()

	result, err := api.DB.Exec(`
		INSERT INTO password_recovery_otps(
			superuser_id,
			otp_hash,
			expires_at,
			attempts,
			created_at,
			updated_at
		)
		VALUES(?, ?, ?, 0, ?, ?);
	`, superuserID, otpHash, expiresUnix, now, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to save account recovery OTP",
		})
		return
	}

	recoveryID, _ := result.LastInsertId()

	sendCtx, sendCancel := contextWithTimeout(r, 12*time.Second)
	defer sendCancel()

	if err := api.Mailer.SendRecoveryOTP(sendCtx, email, otp, expiresAt); err != nil {
		_, _ = api.DB.Exec(`DELETE FROM password_recovery_otps WHERE id = ?;`, recoveryID)

		message := "unable to send recovery email"
		if errors.Is(err, appmailer.ErrNotConfigured) {
			message = "email recovery is not configured"
		}

		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": message,
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"sent":               true,
		"expires_at":         expiresUnix,
		"expires_in_seconds": int64(recoveryOTPLifetime.Seconds()),
		"message":            fmt.Sprintf("A 6-digit OTP was sent to %s.", maskRecoveryEmail(email)),
	})
}

func (api *API) handleRecoveryVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var request recoveryVerifyRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid OTP verification request",
		})
		return
	}

	email, ok := normalizeRecoveryEmail(request.Email)
	otp := strings.TrimSpace(request.OTP)

	if !ok || !isSixDigitOTP(otp) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "enter the complete 6-digit OTP",
		})
		return
	}

	api.RecoveryMu.Lock()
	defer api.RecoveryMu.Unlock()

	now := database.UnixTime()

	var (
		recoveryID int64
		otpHash    string
		attempts   int
	)

	err := api.DB.QueryRow(`
		SELECT pr.id, pr.otp_hash, pr.attempts
		FROM password_recovery_otps pr
		JOIN superusers s ON s.id = pr.superuser_id
		WHERE LOWER(s.email) = ?
		  AND pr.consumed_at IS NULL
		  AND pr.verified_at IS NULL
		  AND pr.expires_at > ?
		  AND s.status = 'active'
		ORDER BY pr.id DESC
		LIMIT 1;
	`, email, now).Scan(&recoveryID, &otpHash, &attempts)

	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusGone, map[string]string{
			"error": "the OTP is expired or no longer available; request a new one",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to verify account recovery OTP",
		})
		return
	}

	if attempts >= recoveryMaxVerifyAttempts {
		_, _ = api.DB.Exec(`
			UPDATE password_recovery_otps
			SET expires_at = ?, updated_at = ?
			WHERE id = ?;
		`, now, now, recoveryID)

		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "too many incorrect OTP attempts; request a new OTP",
		})
		return
	}

	if !verifyPassword(otp, otpHash) {
		attempts++
		newExpiry := any(nil)
		if attempts >= recoveryMaxVerifyAttempts {
			newExpiry = now
		}

		if newExpiry == nil {
			_, _ = api.DB.Exec(`
				UPDATE password_recovery_otps
				SET attempts = ?, updated_at = ?
				WHERE id = ?;
			`, attempts, now, recoveryID)
		} else {
			_, _ = api.DB.Exec(`
				UPDATE password_recovery_otps
				SET attempts = ?, expires_at = ?, updated_at = ?
				WHERE id = ?;
			`, attempts, now, now, recoveryID)
		}

		remaining := recoveryMaxVerifyAttempts - attempts
		if remaining < 0 {
			remaining = 0
		}

		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":              "incorrect OTP",
			"remaining_attempts": remaining,
		})
		return
	}

	resetToken, resetTokenHash, err := newSessionToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to continue account recovery",
		})
		return
	}

	resetExpires := time.Now().Add(recoveryOTPLifetime).UTC().Unix()

	if _, err := api.DB.Exec(`
		UPDATE password_recovery_otps
		SET verified_at = ?,
			reset_token_hash = ?,
			expires_at = ?,
			updated_at = ?
		WHERE id = ?;
	`, now, resetTokenHash, resetExpires, now, recoveryID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to continue account recovery",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"verified":         true,
		"reset_token":      resetToken,
		"reset_expires_at": resetExpires,
	})
}

func (api *API) handleRecoveryReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var request recoveryResetRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid password reset request",
		})
		return
	}

	email, ok := normalizeRecoveryEmail(request.Email)
	request.ResetToken = strings.TrimSpace(request.ResetToken)

	if !ok || request.ResetToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "account recovery authorization is missing",
		})
		return
	}

	if request.NewPassword != request.ConfirmPassword {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "new password and confirmation do not match",
		})
		return
	}

	if len(request.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "new password must contain at least 8 characters",
		})
		return
	}
	if len(request.NewPassword) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "new password is too long",
		})
		return
	}

	passwordHash, err := hashPassword(request.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to secure the new password",
		})
		return
	}

	api.RecoveryMu.Lock()
	defer api.RecoveryMu.Unlock()

	tx, err := api.DB.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to update account password",
		})
		return
	}
	defer tx.Rollback()

	now := database.UnixTime()
	resetTokenHash := hashSessionToken(request.ResetToken)

	var (
		recoveryID  int64
		superuserID int64
	)

	err = tx.QueryRow(`
		SELECT pr.id, s.id
		FROM password_recovery_otps pr
		JOIN superusers s ON s.id = pr.superuser_id
		WHERE LOWER(s.email) = ?
		  AND pr.reset_token_hash = ?
		  AND pr.verified_at IS NOT NULL
		  AND pr.consumed_at IS NULL
		  AND pr.expires_at > ?
		  AND s.status = 'active'
		ORDER BY pr.id DESC
		LIMIT 1;
	`, email, resetTokenHash, now).Scan(&recoveryID, &superuserID)

	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusGone, map[string]string{
			"error": "account recovery authorization expired; start again",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to validate account recovery authorization",
		})
		return
	}

	if _, err := tx.Exec(`
		UPDATE superusers
		SET password_hash = ?, updated_at = ?
		WHERE id = ?;
	`, passwordHash, now, superuserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to update account password",
		})
		return
	}

	if _, err := tx.Exec(`
		UPDATE password_recovery_otps
		SET consumed_at = ?,
			reset_token_hash = NULL,
			updated_at = ?
		WHERE id = ?;
	`, now, now, recoveryID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to finish account recovery",
		})
		return
	}

	if _, err := tx.Exec(`
		DELETE FROM login_sessions
		WHERE superuser_id = ?;
	`, superuserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to close existing login sessions",
		})
		return
	}

	_, _ = tx.Exec(`
		INSERT INTO logs(
			actor_id,
			level,
			action,
			entity_type,
			entity_id,
			message,
			created_at
		)
		VALUES(NULL, 'security', 'password_recovery', 'superuser', ?, ?, ?);
	`, superuserID, "Account password updated through email OTP recovery.", now)

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to finish account recovery",
		})
		return
	}

	clearSessionCookie(w)

	writeJSON(w, http.StatusOK, map[string]any{
		"updated": true,
		"message": "Password updated successfully. You can now sign in.",
	})
}

func contextWithTimeout(r *http.Request, duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), duration)
}

func normalizeRecoveryEmail(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", false
	}

	address, err := stdmail.ParseAddress(value)
	if err != nil || strings.ToLower(strings.TrimSpace(address.Address)) != value {
		return "", false
	}

	return value, true
}

func generateRecoveryOTP() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", value.Int64()), nil
}

func isSixDigitOTP(value string) bool {
	if len(value) != 6 {
		return false
	}

	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}

	return true
}

func maskRecoveryEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return email
	}

	name := parts[0]
	visible := 2
	if len(name) < visible {
		visible = len(name)
	}

	return name[:visible] + strings.Repeat("*", max(2, len(name)-visible)) + "@" + parts[1]
}
