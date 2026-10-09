package internal

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"electricredit/database"
)

const (
	sessionCookieName  = "electricredit_session"
	sessionLifetime    = 12 * time.Hour
	passwordIterations = 180000
	passwordKeyLength  = 32
)

type authenticatedUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (api *API) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", api.handleLogin)
	mux.HandleFunc("/api/auth/session", api.handleSession)
	mux.HandleFunc("/api/auth/logout", api.handleLogout)
}

func (api *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var request loginRequest

	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid login request",
		})
		return
	}

	request.Email = strings.ToLower(strings.TrimSpace(request.Email))

	if request.Email == "" || request.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "email and password are required",
		})
		return
	}

	var (
		user         authenticatedUser
		passwordHash string
	)

	err := api.DB.QueryRow(`
		SELECT s.id,s.first_name,s.last_name,s.email,s.rank,s.password_hash
		FROM superusers s
		WHERE LOWER(s.email) = ? AND s.status = 'active'
		LIMIT 1;
	`, request.Email).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.Role,
		&passwordHash,
	)

	if errors.Is(err, sql.ErrNoRows) ||
		err == nil && !verifyPassword(request.Password, passwordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid email or password",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to sign in",
		})
		return
	}

	rawToken, tokenHash, err := newSessionToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to create session",
		})
		return
	}

	now := database.UnixTime()
	expires := time.Now().Add(sessionLifetime).UTC().Unix()

	if _, err := api.DB.Exec(`
		INSERT INTO login_sessions(superuser_id,token_hash,expires_at,created_at,last_seen_at)
		VALUES(?,?,?,?,?);
	`, user.ID, tokenHash, expires, now, now); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to create session",
		})
		return
	}

	setSessionCookie(w, rawToken, expires)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
	})
}

func (api *API) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	user, sessionID, ok := api.authenticateRequest(r)

	if !ok {
		clearSessionCookie(w)
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "not signed in",
		})
		return
	}

	_, _ = api.DB.Exec(`
		UPDATE login_sessions
		SET last_seen_at = ?
		WHERE id = ?;
	`, database.UnixTime(), sessionID)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
	})
}

func (api *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	cookie, err := r.Cookie(sessionCookieName)

	if err == nil && cookie.Value != "" {
		tokenHash := hashSessionToken(cookie.Value)

		_, _ = api.DB.Exec(`
			DELETE FROM login_sessions
			WHERE token_hash = ?;
		`, tokenHash)
	}

	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) authenticateRequest(r *http.Request) (authenticatedUser, int64, bool) {
	var user authenticatedUser

	cookie, err := r.Cookie(sessionCookieName)

	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return user, 0, false
	}

	tokenHash := hashSessionToken(cookie.Value)
	now := database.UnixTime()

	var sessionID int64

	err = api.DB.QueryRow(`
		SELECT ls.id,s.id,s.first_name,s.last_name,s.email,s.rank
		FROM login_sessions ls
		JOIN superusers s ON s.id = ls.superuser_id
		WHERE ls.token_hash = ? AND ls.expires_at > ? AND s.status = 'active'
		LIMIT 1;
	`, tokenHash, now).Scan(
		&sessionID,
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.Role,
	)

	if err != nil {
		return authenticatedUser{}, 0, false
	}

	return user, sessionID, true
}

func CreateSuperuser(
	db *sql.DB,
	roleName string,
	firstName string,
	lastName string,
	email string,
	password string,
) error {
	roleName = strings.ToLower(strings.TrimSpace(roleName))
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	email = strings.ToLower(strings.TrimSpace(email))
	if roleName != "owner" && roleName != "admin" && roleName != "developer" {
		return errors.New("role must be owner, admin, or developer")
	}
	if firstName == "" || lastName == "" || email == "" || password == "" {
		return errors.New("first name, last name, email, and password are required")
	}
	if len(password) < 8 {
		return errors.New("password must contain at least 8 characters")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := database.UnixTime()
	_, err = db.Exec(`INSERT INTO superusers(first_name,last_name,email,rank,password_hash,status,created_at,updated_at) VALUES(?,?,?,?,?,'active',?,?);`, firstName, lastName, email, roleName, passwordHash, now, now)
	if err != nil {
		return fmt.Errorf("create superuser: %w", err)
	}
	return nil
}

// ResetSuperuserPassword safely replaces a superuser password by email.
// The password is PBKDF2-hashed and all existing sessions/recovery tokens for
// the account are invalidated so the new password takes effect immediately.
func ResetSuperuserPassword(db *sql.DB, email string, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	if email == "" || password == "" {
		return errors.New("email and password are required")
	}
	if len(password) < 8 {
		return errors.New("password must contain at least 8 characters")
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var superuserID int64
	if err := tx.QueryRow(`SELECT id FROM superusers WHERE LOWER(email)=? LIMIT 1;`, email).Scan(&superuserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("no ElectriCredit superuser account was found for this email")
		}
		return err
	}

	now := database.UnixTime()
	if _, err := tx.Exec(`
		UPDATE superusers
		SET password_hash = ?, updated_at = ?
		WHERE id = ?;
	`, passwordHash, now, superuserID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM login_sessions WHERE superuser_id = ?;`, superuserID); err != nil {
		return fmt.Errorf("clear login sessions: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM password_recovery_otps WHERE superuser_id = ?;`, superuserID); err != nil {
		return fmt.Errorf("clear recovery tokens: %w", err)
	}

	return tx.Commit()
}

func newSessionToken() (string, string, error) {
	raw := make([]byte, 32)

	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashSessionToken(token), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func setSessionCookie(w http.ResponseWriter, token string, expires int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // Set true when deployed behind HTTPS.
		Expires:  time.Unix(expires, 0).UTC(),
		MaxAge:   int(sessionLifetime.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	})
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)

	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key := pbkdf2SHA256(
		[]byte(password),
		salt,
		passwordIterations,
		passwordKeyLength,
	)

	return strings.Join([]string{
		"pbkdf2_sha256",
		strconv.Itoa(passwordIterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$"), nil
}

func verifyPassword(password string, encoded string) bool {
	parts := strings.Split(encoded, "$")

	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}

	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false
	}

	actual := pbkdf2SHA256(
		[]byte(password),
		salt,
		iterations,
		len(expected),
	)

	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	hashLength := sha256.Size
	blocks := (keyLength + hashLength - 1) / hashLength
	derived := make([]byte, 0, blocks*hashLength)

	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{
			byte(block >> 24),
			byte(block >> 16),
			byte(block >> 8),
			byte(block),
		})

		u := mac.Sum(nil)
		t := append([]byte(nil), u...)

		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)

			for j := range t {
				t[j] ^= u[j]
			}
		}

		derived = append(derived, t...)
	}

	return derived[:keyLength]
}
