package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	stdmail "net/mail"
	"strings"
	"time"

	"electricredit/database"
)

type profileEmailRequest struct {
	Email           string `json:"email"`
	CurrentPassword string `json:"current_password"`
}

type profileEmailVerifyRequest struct {
	Email           string `json:"email"`
	OTP             string `json:"otp"`
	CurrentPassword string `json:"current_password"`
}

type profilePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (api *API) registerProfileSecurityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/profile/email/status", api.handleProfileEmailStatus)
	mux.HandleFunc("/api/admin/profile/email/request", api.handleProfileEmailRequest)
	mux.HandleFunc("/api/admin/profile/email/verify", api.handleProfileEmailVerify)
	mux.HandleFunc("/api/admin/profile/password", api.handleProfilePassword)
}

func (api *API) handleProfileEmailStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireRoles(w, r, "admin", "owner", "developer"); !ok {
		return
	}
	configured := api.Mailer != nil && api.Mailer.Configured()
	if !configured {
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "online": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := api.Mailer.Available(ctx)
	writeJSON(w, http.StatusOK, map[string]any{"available": err == nil, "online": err == nil})
}

func (api *API) handleProfileEmailRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	var request profileEmailRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email verification request"})
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if _, err := stdmail.ParseAddress(request.Email); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter a valid email address"})
		return
	}
	if request.CurrentPassword == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "current password is required"})
		return
	}

	superuserID, passwordHash, err := api.superuserCredentials(user.ID)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	if !verifyPassword(request.CurrentPassword, passwordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}
	if strings.EqualFold(request.Email, user.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter a different email address"})
		return
	}
	var conflict int
	if err := api.DB.QueryRow(`SELECT (SELECT COUNT(*) FROM superusers WHERE LOWER(email)=? AND id<>?)+(SELECT COUNT(*) FROM users WHERE LOWER(COALESCE(email,''))=?);`, request.Email, user.ID, request.Email).Scan(&conflict); err != nil {
		writeAdminDataError(w, err)
		return
	}
	if conflict > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "that email address is already in use"})
		return
	}
	if api.Mailer == nil || !api.Mailer.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "email verification is unavailable right now"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if err := api.Mailer.Available(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "connect to the internet and try again"})
		return
	}

	api.RecoveryMu.Lock()
	defer api.RecoveryMu.Unlock()
	now := database.UnixTime()
	var activeExpiry int64
	err = api.DB.QueryRow(`SELECT expires_at FROM profile_email_otps WHERE superuser_id=? AND new_email=? AND consumed_at IS NULL AND expires_at>? ORDER BY id DESC LIMIT 1;`, superuserID, request.Email, now).Scan(&activeExpiry)
	if err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "a verification code is already active for this email", "active": true, "expires_at": activeExpiry})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeAdminDataError(w, err)
		return
	}

	windowStart := now - int64(recoverySendWindow.Seconds())
	var sendCount int
	if err := api.DB.QueryRow(`SELECT COUNT(*) FROM profile_email_otps WHERE superuser_id=? AND created_at>=?;`, superuserID, windowStart).Scan(&sendCount); err != nil {
		writeAdminDataError(w, err)
		return
	}
	if sendCount >= recoveryMaxSendsPerWindow {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "email verification limit reached; try again later"})
		return
	}

	otp, err := generateRecoveryOTP()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	otpHash, err := hashPassword(otp)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	expiresAt := time.Now().Add(recoveryOTPLifetime).UTC()
	result, err := api.DB.Exec(`INSERT INTO profile_email_otps(superuser_id,new_email,otp_hash,expires_at,attempts,created_at,updated_at) VALUES(?,?,?,?,0,?,?);`, superuserID, request.Email, otpHash, expiresAt.Unix(), now, now)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	otpID, _ := result.LastInsertId()
	sendCtx, sendCancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer sendCancel()
	if err := api.Mailer.SendEmailChangeOTP(sendCtx, request.Email, otp, expiresAt); err != nil {
		_, _ = api.DB.Exec(`DELETE FROM profile_email_otps WHERE id=?;`, otpID)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unable to send the verification email"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"sent": true, "expires_at": expiresAt.Unix(), "message": "A 6-digit verification code was sent to the new email."})
}

func (api *API) handleProfileEmailVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	var request profileEmailVerifyRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email verification request"})
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	request.OTP = strings.TrimSpace(request.OTP)
	if _, err := stdmail.ParseAddress(request.Email); err != nil || !isSixDigitOTP(request.OTP) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter the new email and complete 6-digit code"})
		return
	}

	superuserID, passwordHash, err := api.superuserCredentials(user.ID)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	if !verifyPassword(request.CurrentPassword, passwordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}

	api.RecoveryMu.Lock()
	defer api.RecoveryMu.Unlock()
	now := database.UnixTime()
	var otpID int64
	var otpHash string
	var attempts int
	err = api.DB.QueryRow(`SELECT id,otp_hash,attempts FROM profile_email_otps WHERE superuser_id=? AND new_email=? AND consumed_at IS NULL AND verified_at IS NULL AND expires_at>? ORDER BY id DESC LIMIT 1;`, superuserID, request.Email, now).Scan(&otpID, &otpHash, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "the verification code has expired; request a new one"})
		return
	}
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	if attempts >= recoveryMaxVerifyAttempts {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many incorrect attempts; request a new code"})
		return
	}
	if !verifyPassword(request.OTP, otpHash) {
		attempts++
		_, _ = api.DB.Exec(`UPDATE profile_email_otps SET attempts=?,expires_at=CASE WHEN ?>=? THEN ? ELSE expires_at END,updated_at=? WHERE id=?;`, attempts, attempts, recoveryMaxVerifyAttempts, now, now, otpID)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "incorrect verification code"})
		return
	}

	tx, err := api.DB.Begin()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE superusers SET email=?,updated_at=? WHERE id=?;`, request.Email, now, user.ID); err != nil {
		writeAdminDataError(w, normalizeConstraintError(err))
		return
	}
	if _, err := tx.Exec(`UPDATE profile_email_otps SET verified_at=?,consumed_at=?,updated_at=? WHERE id=?;`, now, now, now, otpID); err != nil {
		writeAdminDataError(w, err)
		return
	}
	_, _ = tx.Exec(`DELETE FROM password_recovery_otps WHERE superuser_id=?;`, superuserID)
	if err := tx.Commit(); err != nil {
		writeAdminDataError(w, err)
		return
	}
	api.logAction(user.ID, "profile.email", "superuser", superuserID, "Updated account email")
	writeJSON(w, http.StatusOK, map[string]any{"updated": true, "email": request.Email})
}

func (api *API) handleProfilePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w, http.MethodPut)
		return
	}
	user, sessionID, authenticated := api.authenticateRequest(r)
	if !authenticated || !containsRole(user.Role, "admin", "owner", "developer") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not signed in"})
		return
	}
	var request profilePasswordRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid password request"})
		return
	}
	if len(request.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new password must contain at least 8 characters"})
		return
	}
	if request.NewPassword != request.ConfirmPassword {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new password and confirmation do not match"})
		return
	}
	superuserID, passwordHash, err := api.superuserCredentials(user.ID)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	if !verifyPassword(request.CurrentPassword, passwordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}
	newHash, err := hashPassword(request.NewPassword)
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
	if _, err := tx.Exec(`UPDATE superusers SET password_hash=?,updated_at=? WHERE id=?;`, newHash, now, superuserID); err != nil {
		writeAdminDataError(w, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM login_sessions WHERE superuser_id=? AND id<>?;`, superuserID, sessionID); err != nil {
		writeAdminDataError(w, err)
		return
	}
	_, _ = tx.Exec(`DELETE FROM password_recovery_otps WHERE superuser_id=?;`, superuserID)
	if err := tx.Commit(); err != nil {
		writeAdminDataError(w, err)
		return
	}
	api.logAction(user.ID, "profile.password", "superuser", superuserID, "Changed account password")
	writeJSON(w, http.StatusOK, map[string]any{"updated": true})
}

func (api *API) superuserCredentials(userID int64) (int64, string, error) {
	var id int64
	var hash string
	err := api.DB.QueryRow(`SELECT id,password_hash FROM superusers WHERE id=?;`, userID).Scan(&id, &hash)
	return id, hash, err
}

func containsRole(role string, allowed ...string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	for _, item := range allowed {
		if role == item {
			return true
		}
	}
	return false
}

func encodeProfileLinks(links map[string]string) string {
	allowed := map[string]bool{"website": true, "facebook": true, "github": true, "linkedin": true, "telegram": true, "whatsapp": true}
	clean := map[string]string{}
	for key, value := range links {
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !allowed[key] || value == "" {
			continue
		}
		if len(value) > 300 {
			value = value[:300]
		}
		clean[key] = value
	}
	data, _ := json.Marshal(clean)
	return string(data)
}

func decodeProfileLinks(raw string) map[string]string {
	result := map[string]string{}
	_ = json.Unmarshal([]byte(strings.TrimSpace(raw)), &result)
	return result
}

func normalizeProfileBirthDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", errors.New("enter a valid birth date")
	}
	if date.After(time.Now()) {
		return "", errors.New("birth date cannot be in the future")
	}
	return value, nil
}

func validateProfileImageData(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > 900000 {
		return "", errors.New("profile image is too large")
	}
	if !(strings.HasPrefix(value, "data:image/jpeg;base64,") || strings.HasPrefix(value, "data:image/png;base64,") || strings.HasPrefix(value, "data:image/webp;base64,")) {
		return "", errors.New("profile image must be a JPEG, PNG, or WebP image")
	}
	return value, nil
}
