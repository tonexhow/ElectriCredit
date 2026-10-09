package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	stdmail "net/mail"
	"strconv"
	"strings"
	"time"

	"electricredit/database"
	appmailer "electricredit/internal/mailer"
)

type adminTenantRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	BirthDate string `json:"birth_date"`
	Gender    string `json:"gender"`
	ImageData string `json:"image_data"`
	Status    string `json:"status"`
}

type adminCardRequest struct {
	UID       string `json:"uid"`
	ScanID    string `json:"scan_id"`
	Balance   string `json:"balance"`
	DebtLimit string `json:"debt_limit"`
	Status    string `json:"status"`
}

type adminAccountRequest struct {
	FirstName string            `json:"first_name"`
	LastName  string            `json:"last_name"`
	Email     string            `json:"email"`
	Phone     string            `json:"phone"`
	Password  string            `json:"password"`
	Status    string            `json:"status"`
	Role      string            `json:"role"`
	ImageData string            `json:"image_data"`
	Links     map[string]string `json:"links"`
}

type adminRateRequest struct {
	BaseRate   string `json:"base_rate"`
	TenantRate string `json:"tenant_rate"`
	ChangeNote string `json:"change_note"`
}

type adminNotificationRequest struct {
	Audience     string   `json:"audience"`
	CustomEmails []string `json:"custom_emails"`
	Subject      string   `json:"subject"`
	Preset       string   `json:"preset"`
	Message      string   `json:"message"`
	ManualHTML   bool     `json:"manual_html"`
}

type profileUpdateRequest struct {
	CurrentPassword string            `json:"current_password"`
	FirstName       string            `json:"first_name"`
	LastName        string            `json:"last_name"`
	BirthDate       string            `json:"birth_date"`
	Phone           string            `json:"phone"`
	ImageData       string            `json:"image_data"`
	Links           map[string]string `json:"links"`
}

type adminTenant struct {
	ID        int64       `json:"id"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Email     string      `json:"email"`
	Phone     string      `json:"phone"`
	BirthDate string      `json:"birth_date"`
	Gender    string      `json:"gender"`
	ImageURL  string      `json:"image_url"`
	DebtLimit string      `json:"debt_limit"`
	Status    string      `json:"status"`
	CreatedAt int64       `json:"created_at"`
	UpdatedAt int64       `json:"updated_at"`
	Cards     []adminCard `json:"cards"`
}

type adminCard struct {
	ID        int64  `json:"id"`
	UID       string `json:"uid"`
	Balance   string `json:"balance"`
	DebtLimit string `json:"debt_limit"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (api *API) registerAdminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/overview", api.handleAdminOverview)
	mux.HandleFunc("/api/admin/dashboard", api.handleAdminDashboard)
	mux.HandleFunc("/api/admin/tenants", api.handleAdminTenants)
	mux.HandleFunc("/api/admin/tenants/", api.handleAdminTenantByID)
	mux.HandleFunc("/api/admin/cards/", api.handleAdminCardByID)
	mux.HandleFunc("/api/admin/challenge", api.handleAdminActionChallenge)
	mux.HandleFunc("/api/admin/admins", api.handleAdminAccounts)
	mux.HandleFunc("/api/admin/admins/", api.handleAdminAccountByID)
	mux.HandleFunc("/api/admin/rates", api.handleAdminRates)
	mux.HandleFunc("/api/admin/notification/status", api.handleAdminNotificationStatus)
	mux.HandleFunc("/api/admin/notifications/send", api.handleAdminNotificationSend)
	mux.HandleFunc("/api/admin/profile", api.handleAdminProfile)
	mux.HandleFunc("/api/admin/bonus/challenge", api.handleAdminBonusChallenge)
	mux.HandleFunc("/api/admin/bonus/apply", api.handleAdminBonusApply)
	api.registerCardScanRoutes(mux)
	api.registerProfileSecurityRoutes(mux)
	api.registerStaffChatRoutes(mux)
	api.registerAdminModuleRoutes(mux)
}

func (api *API) handleAdminActionChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	var req databaseChallengeRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid challenge request"})
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	allowed := req.Action == "tenant-edit" || req.Action == "tenant-delete" || req.Action == "tenant-card-delete"
	if !allowed {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid challenge action"})
		return
	}
	code, err := api.issueChallenge(user.ID, req.Action)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"challenge": code, "length": len(code), "expires_in": 300})
}

func (api *API) requireAuth(w http.ResponseWriter, r *http.Request) (authenticatedUser, bool) {
	user, _, ok := api.authenticateRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not signed in"})
		return authenticatedUser{}, false
	}
	return user, true
}

func (api *API) requireRoles(w http.ResponseWriter, r *http.Request, roles ...string) (authenticatedUser, bool) {
	user, ok := api.requireAuth(w, r)
	if !ok {
		return authenticatedUser{}, false
	}
	for _, role := range roles {
		if strings.EqualFold(user.Role, role) {
			return user, true
		}
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "your account does not have permission for this action"})
	return authenticatedUser{}, false
}

func (api *API) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}

	counts := map[string]int64{}
	queries := map[string]string{
		"tenants":        `SELECT COUNT(*) FROM users;`,
		"cards":          `SELECT COUNT(*) FROM cards;`,
		"administrators": `SELECT COUNT(*) FROM superusers WHERE rank='admin' AND status='active';`,
		"hubs":           `SELECT COUNT(*) FROM hubs;`,
		"registries":     `SELECT COUNT(*) FROM registries;`,
		"active_power":   `SELECT COUNT(*) FROM power_sessions WHERE status='active';`,
	}
	for key, query := range queries {
		var count int64
		if err := api.DB.QueryRow(query).Scan(&count); err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to load overview"})
			return
		}
		counts[key] = count
	}

	activeStaff, err := api.listActiveStaff()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	baseRate, tenantRate, income, markup := api.rateSummary()

	writeJSON(w, http.StatusOK, map[string]any{
		"user":         user,
		"counts":       counts,
		"active_staff": activeStaff,
		"rates": map[string]any{
			"base_rate":         database.FormatMoneyUnits(baseRate),
			"tenant_rate":       database.FormatMoneyUnits(tenantRate),
			"income_per_kwh":    database.FormatMoneyUnits(income),
			"markup_percentage": fmt.Sprintf("%.2f", markup),
		},
	})
}

func (api *API) handleAdminTenants(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		tenants, err := api.listAdminTenants()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to load tenants"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tenants": tenants})
	case http.MethodPost:
		var request adminTenantRequest
		if err := readJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tenant request"})
			return
		}
		id, err := api.createTenant(request)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		api.logAction(user.ID, "tenant.create", "user", id, fmt.Sprintf("Created tenant %s %s", request.FirstName, request.LastName))
		tenant, _ := api.getAdminTenant(id)
		writeJSON(w, http.StatusCreated, map[string]any{"tenant": tenant})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (api *API) handleAdminTenantByID(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/admin/tenants/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant id is required"})
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tenant id"})
		return
	}

	if len(parts) == 2 && parts[1] == "cards" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		var request adminCardRequest
		if err := readJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid card request"})
			return
		}
		cardID, err := api.createTenantCard(id, user.ID, request)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		api.logAction(user.ID, "card.create", "card", cardID, fmt.Sprintf("Added RFID card to tenant %d", id))
		card, _ := api.getAdminCard(cardID)
		writeJSON(w, http.StatusCreated, map[string]any{"card": card})
		return
	}
	if len(parts) != 1 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "admin tenant endpoint not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		tenant, err := api.getAdminTenant(id)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "tenant not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to load tenant"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tenant": tenant})
	case http.MethodPut:
		if !api.requireManagementActionChallenge(w, r, user.ID, "tenant-edit") {
			return
		}
		var request adminTenantRequest
		if err := readJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tenant request"})
			return
		}
		if err := api.updateTenant(id, request); err != nil {
			writeAdminDataError(w, err)
			return
		}
		api.logAction(user.ID, "tenant.update", "user", id, fmt.Sprintf("Updated tenant %d", id))
		tenant, _ := api.getAdminTenant(id)
		writeJSON(w, http.StatusOK, map[string]any{"tenant": tenant})
	case http.MethodDelete:
		if !api.requireManagementActionChallenge(w, r, user.ID, "tenant-delete") {
			return
		}
		if err := api.deleteTenant(id); err != nil {
			writeAdminDataError(w, err)
			return
		}
		api.logAction(user.ID, "tenant.delete", "user", id, fmt.Sprintf("Deleted tenant %d", id))
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (api *API) listAdminTenants() ([]adminTenant, error) {
	rows, err := api.DB.Query(`
		SELECT u.id,u.first_name,u.last_name,COALESCE(u.email,''),COALESCE(u.phone,''),
		       COALESCE(u.birth_date,''),COALESCE(u.gender,''),COALESCE(u.profile_image,''),
		       COALESCE(u.profile_image_blob,X''),COALESCE(u.profile_image_mime,''),
		       COALESCE(u.debt_limit_units,(SELECT debt_limit_units FROM price_configuration WHERE id=1)),
		       u.status,u.created_at,u.updated_at
		FROM users u
		ORDER BY u.first_name,u.last_name,u.id;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tenants := []adminTenant{}
	for rows.Next() {
		var tenant adminTenant
		var debtLimit int64
		var imagePath, imageMime string
		var imageBlob []byte
		if err := rows.Scan(&tenant.ID, &tenant.FirstName, &tenant.LastName, &tenant.Email, &tenant.Phone, &tenant.BirthDate, &tenant.Gender, &imagePath, &imageBlob, &imageMime, &debtLimit, &tenant.Status, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
			return nil, err
		}
		tenant.DebtLimit = database.FormatMoneyUnits(debtLimit)
		tenant.ImageURL = userProfileImageURL(imagePath, imageBlob, imageMime)
		tenants = append(tenants, tenant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range tenants {
		cards, err := api.listTenantCards(tenants[i].ID)
		if err != nil {
			return nil, err
		}
		tenants[i].Cards = cards
	}
	return tenants, nil
}

func (api *API) getAdminTenant(id int64) (adminTenant, error) {
	var tenant adminTenant
	var debtLimit int64
	var imagePath, imageMime string
	var imageBlob []byte
	err := api.DB.QueryRow(`
		SELECT u.id,u.first_name,u.last_name,COALESCE(u.email,''),COALESCE(u.phone,''),
		       COALESCE(u.birth_date,''),COALESCE(u.gender,''),COALESCE(u.profile_image,''),
		       COALESCE(u.profile_image_blob,X''),COALESCE(u.profile_image_mime,''),
		       COALESCE(u.debt_limit_units,(SELECT debt_limit_units FROM price_configuration WHERE id=1)),
		       u.status,u.created_at,u.updated_at
		FROM users u
		WHERE u.id=?
		LIMIT 1;
	`, id).Scan(&tenant.ID, &tenant.FirstName, &tenant.LastName, &tenant.Email, &tenant.Phone, &tenant.BirthDate, &tenant.Gender, &imagePath, &imageBlob, &imageMime, &debtLimit, &tenant.Status, &tenant.CreatedAt, &tenant.UpdatedAt)
	if err != nil {
		return adminTenant{}, err
	}
	tenant.DebtLimit = database.FormatMoneyUnits(debtLimit)
	tenant.ImageURL = userProfileImageURL(imagePath, imageBlob, imageMime)
	tenant.Cards, err = api.listTenantCards(id)
	return tenant, err
}

func (api *API) listTenantCards(userID int64) ([]adminCard, error) {
	rows, err := api.DB.Query(`
		SELECT c.id,c.uid,c.balance_units,c.status,c.created_at,c.updated_at,
		       COALESCE(u.debt_limit_units,(SELECT debt_limit_units FROM price_configuration WHERE id=1))
		FROM cards c JOIN users u ON u.id=c.user_id
		WHERE c.user_id=? ORDER BY c.id;
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []adminCard{}
	for rows.Next() {
		var card adminCard
		var balance, debtLimit int64
		if err := rows.Scan(&card.ID, &card.UID, &balance, &card.Status, &card.CreatedAt, &card.UpdatedAt, &debtLimit); err != nil {
			return nil, err
		}
		card.Balance = database.FormatMoneyUnits(balance)
		card.DebtLimit = database.FormatMoneyUnits(debtLimit)
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cards, nil
}

func (api *API) getAdminCard(id int64) (adminCard, error) {
	var card adminCard
	var balance, debtLimit int64
	err := api.DB.QueryRow(`
		SELECT c.id,c.uid,c.balance_units,c.status,c.created_at,c.updated_at,
		       COALESCE(u.debt_limit_units,(SELECT debt_limit_units FROM price_configuration WHERE id=1))
		FROM cards c JOIN users u ON u.id=c.user_id WHERE c.id=?;
	`, id).Scan(&card.ID, &card.UID, &balance, &card.Status, &card.CreatedAt, &card.UpdatedAt, &debtLimit)
	if err == nil {
		card.Balance = database.FormatMoneyUnits(balance)
		card.DebtLimit = database.FormatMoneyUnits(debtLimit)
	}
	return card, err
}

func validateTenantRequest(request *adminTenantRequest) error {
	request.FirstName = strings.TrimSpace(request.FirstName)
	request.LastName = strings.TrimSpace(request.LastName)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	request.Phone = normalizePhone(request.Phone)
	request.BirthDate = strings.TrimSpace(request.BirthDate)
	request.Gender = strings.ToLower(strings.TrimSpace(request.Gender))
	request.Status = strings.ToLower(strings.TrimSpace(request.Status))
	if request.Status == "" {
		request.Status = "active"
	}
	if request.FirstName == "" || request.LastName == "" || request.Email == "" {
		return errors.New("first name, last name, and email are required")
	}
	if _, err := stdmail.ParseAddress(request.Email); err != nil {
		return errors.New("enter a valid tenant email")
	}
	if request.Phone == "" {
		return errors.New("phone number is required")
	}
	if request.BirthDate == "" {
		return errors.New("birth date is required")
	}
	birthDate, err := time.Parse("2006-01-02", request.BirthDate)
	if err != nil || birthDate.After(time.Now()) {
		return errors.New("enter a valid birth date")
	}
	if request.Gender != "female" && request.Gender != "male" && request.Gender != "other" && request.Gender != "prefer_not_to_say" {
		return errors.New("select a valid gender")
	}
	if request.Status != "active" && request.Status != "inactive" && request.Status != "disabled" {
		return errors.New("invalid tenant status")
	}
	return nil
}

func normalizePhone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r == '+' && i == 0 {
			b.WriteRune(r)
		}
	}
	phone := b.String()
	if !strings.HasPrefix(phone, "+") || len(phone) < 8 || len(phone) > 16 {
		return ""
	}
	return phone
}

func (api *API) createTenant(request adminTenantRequest) (int64, error) {
	if err := validateTenantRequest(&request); err != nil {
		return 0, err
	}
	var defaultDebt int64
	if err := api.DB.QueryRow(`SELECT debt_limit_units FROM price_configuration WHERE id=1;`).Scan(&defaultDebt); err != nil {
		return 0, err
	}
	now := database.UnixTime()
	var email any
	if request.Email != "" {
		email = request.Email
	}
	imageBlob, imageMime, err := decodeProfileImageBlob(request.ImageData)
	if err != nil {
		return 0, err
	}
	result, err := api.DB.Exec(`
		INSERT INTO users(first_name,last_name,email,phone,room,profile_image,profile_image_blob,profile_image_mime,birth_date,gender,debt_limit_units,status,created_at,updated_at)
		VALUES(?,?,?,?,NULL,'assets/default-image.png',?,?,?,?,?, ?,?,?);
	`, request.FirstName, request.LastName, email, request.Phone, imageBlob, nullIfEmpty(imageMime), request.BirthDate, request.Gender, defaultDebt, request.Status, now, now)
	if err != nil {
		return 0, normalizeConstraintError(err)
	}
	return result.LastInsertId()
}

func (api *API) updateTenant(id int64, request adminTenantRequest) error {
	if err := validateTenantRequest(&request); err != nil {
		return err
	}
	var exists int
	if err := api.DB.QueryRow(`SELECT COUNT(*) FROM users u WHERE u.id=?;`, id).Scan(&exists); err != nil || exists == 0 {
		return sql.ErrNoRows
	}
	var email any
	if request.Email != "" {
		email = request.Email
	}
	imageBlob, imageMime, imageErr := decodeProfileImageBlob(request.ImageData)
	if imageErr != nil {
		return imageErr
	}
	var err error
	if len(imageBlob) > 0 {
		_, err = api.DB.Exec(`
			UPDATE users SET first_name=?,last_name=?,email=?,phone=?,room=NULL,birth_date=?,gender=?,profile_image_blob=?,profile_image_mime=?,status=?,updated_at=? WHERE id=?;
		`, request.FirstName, request.LastName, email, request.Phone, request.BirthDate, request.Gender, imageBlob, imageMime, request.Status, database.UnixTime(), id)
	} else {
		_, err = api.DB.Exec(`
			UPDATE users SET first_name=?,last_name=?,email=?,phone=?,room=NULL,birth_date=?,gender=?,status=?,updated_at=? WHERE id=?;
		`, request.FirstName, request.LastName, email, request.Phone, request.BirthDate, request.Gender, request.Status, database.UnixTime(), id)
	}
	return normalizeConstraintError(err)
}

func (api *API) deleteTenant(id int64) error {
	var activePower, activeRegistry int
	if err := api.DB.QueryRow(`SELECT COUNT(*) FROM power_sessions ps JOIN cards c ON c.id=ps.card_id WHERE c.user_id=? AND ps.status='active';`, id).Scan(&activePower); err != nil {
		return err
	}
	if err := api.DB.QueryRow(`SELECT COUNT(*) FROM registry_sessions rs JOIN cards c ON c.id=rs.card_id WHERE c.user_id=? AND rs.status='active';`, id).Scan(&activeRegistry); err != nil {
		return err
	}
	if activePower > 0 || activeRegistry > 0 {
		return errors.New("this tenant has an active electricity or top-up session; terminate the session before deleting the tenant")
	}
	result, err := api.DB.Exec(`DELETE FROM users WHERE id=?;`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func validateCardRequest(request *adminCardRequest) (balance, debtLimit int64, err error) {
	request.UID = strings.ToUpper(strings.TrimSpace(request.UID))
	request.Balance = strings.TrimSpace(request.Balance)
	request.DebtLimit = strings.TrimSpace(request.DebtLimit)
	request.Status = strings.ToLower(strings.TrimSpace(request.Status))
	if request.Status == "" {
		request.Status = "active"
	}
	if request.Status != "active" && request.Status != "inactive" && request.Status != "blocked" && request.Status != "lost" {
		return 0, 0, errors.New("invalid card status")
	}
	balance, err = database.ParseMoneyUnits(request.Balance)
	if err != nil {
		return 0, 0, err
	}
	if request.DebtLimit == "" {
		request.DebtLimit = "-100.00000"
	}
	debtLimit, err = database.ParseMoneyUnits(request.DebtLimit)
	if err != nil || debtLimit > 0 {
		return 0, 0, errors.New("debit limit must be zero or a negative amount with up to 5 decimals")
	}
	return balance, debtLimit, nil
}

func (api *API) createTenantCard(userID, actorID int64, request adminCardRequest) (int64, error) {
	balance, debtLimit, err := validateCardRequest(&request)
	if err != nil {
		return 0, err
	}
	request.UID = strings.ToUpper(strings.TrimSpace(request.UID))
	request.ScanID = strings.TrimSpace(request.ScanID)
	if request.UID == "" || request.ScanID == "" {
		return 0, errors.New("scan an RFID card with a connected Hub or Registry before creating the card")
	}
	if api.CardScans == nil || !api.CardScans.consume(request.ScanID, actorID, request.UID) {
		return 0, errors.New("RFID scan is unavailable, expired, or does not match the scanned card")
	}
	var exists int
	if err := api.DB.QueryRow(`SELECT COUNT(*) FROM users u WHERE u.id=?;`, userID).Scan(&exists); err != nil || exists == 0 {
		return 0, sql.ErrNoRows
	}
	tx, err := api.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := database.UnixTime()
	if _, err := tx.Exec(`UPDATE users SET debt_limit_units=?,updated_at=? WHERE id=?;`, debtLimit, now, userID); err != nil {
		return 0, err
	}
	result, err := tx.Exec(`INSERT INTO cards(uid,user_id,balance_units,status,created_at,updated_at) VALUES(?,?,?,?,?,?);`, request.UID, userID, balance, request.Status, now, now)
	if err != nil {
		return 0, normalizeConstraintError(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	api.notifyTenantCardConnected(userID, id, request.UID)
	return id, nil
}

func (api *API) handleAdminCardByID(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	idText := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/cards/"), "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid card id"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var request adminCardRequest
		if err := readJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid card request"})
			return
		}
		balance, debtLimit, err := validateCardRequest(&request)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		tx, err := api.DB.Begin()
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		defer tx.Rollback()
		var userID int64
		if err := tx.QueryRow(`SELECT user_id FROM cards WHERE id=?;`, id).Scan(&userID); err != nil {
			writeAdminDataError(w, err)
			return
		}
		now := database.UnixTime()
		if _, err := tx.Exec(`UPDATE users SET debt_limit_units=?,updated_at=? WHERE id=?;`, debtLimit, now, userID); err != nil {
			writeAdminDataError(w, err)
			return
		}
		// The physical RFID UID is immutable from the management screen.
		if _, err := tx.Exec(`UPDATE cards SET balance_units=?,status=?,updated_at=? WHERE id=?;`, balance, request.Status, now, id); err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		if err := tx.Commit(); err != nil {
			writeAdminDataError(w, err)
			return
		}
		api.logAction(user.ID, "card.update", "card", id, fmt.Sprintf("Updated RFID card %d", id))
		card, _ := api.getAdminCard(id)
		writeJSON(w, http.StatusOK, map[string]any{"card": card})
	case http.MethodDelete:
		if !api.requireManagementActionChallenge(w, r, user.ID, "tenant-card-delete") {
			return
		}
		var activePower, activeRegistry int
		_ = api.DB.QueryRow(`SELECT COUNT(*) FROM power_sessions WHERE card_id=? AND status='active';`, id).Scan(&activePower)
		_ = api.DB.QueryRow(`SELECT COUNT(*) FROM registry_sessions WHERE card_id=? AND status='active';`, id).Scan(&activeRegistry)
		if activePower > 0 || activeRegistry > 0 {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "this card is being used by an active electricity or top-up session; terminate the session before deleting the card"})
			return
		}
		result, err := api.DB.Exec(`DELETE FROM cards WHERE id=?;`, id)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "card not found"})
			return
		}
		api.logAction(user.ID, "card.delete", "card", id, fmt.Sprintf("Deleted RFID card %d", id))
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodPut, http.MethodDelete)
	}
}

func (api *API) handleAdminAccounts(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "owner", "developer")
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := api.DB.Query(`SELECT id,first_name,last_name,email,COALESCE(phone,''),COALESCE(profile_image,''),COALESCE(profile_image_blob,X''),COALESCE(profile_image_mime,''),COALESCE(links_json,'{}'),rank,status,created_at,updated_at FROM superusers WHERE rank IN ('admin','owner') ORDER BY CASE rank WHEN 'owner' THEN 0 ELSE 1 END,first_name,last_name;`)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		defer rows.Close()
		accounts := []map[string]any{}
		for rows.Next() {
			var sid, created, updated int64
			var first, last, email, phone, imagePath, imageMime, linksJSON, accountRole, status string
			var imageBlob []byte
			if err := rows.Scan(&sid, &first, &last, &email, &phone, &imagePath, &imageBlob, &imageMime, &linksJSON, &accountRole, &status, &created, &updated); err != nil {
				writeAdminDataError(w, err)
				return
			}
			accounts = append(accounts, map[string]any{"id": sid, "first_name": first, "last_name": last, "email": email, "phone": phone, "image_url": userProfileImageURL(imagePath, imageBlob, imageMime), "links": decodeProfileLinks(linksJSON), "role": accountRole, "status": status, "created_at": created, "updated_at": updated})
		}
		if err := rows.Err(); err != nil {
			writeAdminDataError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"admins": accounts})
	case http.MethodPost:
		var request adminAccountRequest
		if err := readJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account request"})
			return
		}
		if err := validateAdminAccountRequest(&request, true); err != nil {
			writeAdminDataError(w, err)
			return
		}
		if user.Role != "developer" && request.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "owner accounts can add administrators only"})
			return
		}
		if err := CreateSuperuser(api.DB, request.Role, request.FirstName, request.LastName, request.Email, request.Password); err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		var sid int64
		if err := api.DB.QueryRow(`SELECT id FROM superusers WHERE LOWER(email)=?;`, request.Email).Scan(&sid); err != nil {
			writeAdminDataError(w, err)
			return
		}
		imageBlob, imageMime, imageErr := decodeProfileImageBlob(request.ImageData)
		if imageErr != nil {
			writeAdminDataError(w, imageErr)
			return
		}
		linksJSON := encodeProfileLinks(request.Links)
		_, err := api.DB.Exec(`UPDATE superusers SET phone=?,links_json=?,profile_image_blob=CASE WHEN length(?)>0 THEN ? ELSE profile_image_blob END,profile_image_mime=CASE WHEN length(?)>0 THEN ? ELSE profile_image_mime END,status=?,updated_at=? WHERE id=?;`, nullIfEmpty(request.Phone), linksJSON, imageBlob, imageBlob, imageBlob, imageMime, request.Status, database.UnixTime(), sid)
		if err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		api.logAction(user.ID, "staff.create", "superuser", sid, fmt.Sprintf("Created %s account", request.Role))
		writeJSON(w, http.StatusCreated, map[string]any{"id": sid, "role": request.Role})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func validateAdminAccountRequest(request *adminAccountRequest, passwordRequired bool) error {
	request.FirstName = strings.TrimSpace(request.FirstName)
	request.LastName = strings.TrimSpace(request.LastName)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	request.Phone = strings.TrimSpace(request.Phone)
	request.Status = strings.ToLower(strings.TrimSpace(request.Status))
	request.Role = strings.ToLower(strings.TrimSpace(request.Role))
	if request.Status == "" {
		request.Status = "active"
	}
	if request.Role == "" {
		request.Role = "admin"
	}
	if request.FirstName == "" || request.LastName == "" || request.Email == "" {
		return errors.New("first name, last name, and email are required")
	}
	if _, err := stdmail.ParseAddress(request.Email); err != nil {
		return errors.New("enter a valid account email")
	}
	if request.Phone != "" {
		if normalized := normalizePhone(request.Phone); normalized == "" {
			return errors.New("enter a valid phone number with country code")
		} else {
			request.Phone = normalized
		}
	}
	if passwordRequired && len(request.Password) < 8 {
		return errors.New("password must contain at least 8 characters")
	}
	if request.Password != "" && len(request.Password) < 8 {
		return errors.New("password must contain at least 8 characters")
	}
	if request.Status != "active" && request.Status != "inactive" && request.Status != "disabled" {
		return errors.New("invalid account status")
	}
	if request.Role != "admin" && request.Role != "owner" {
		return errors.New("this page supports Administrator and Owner accounts only")
	}
	return nil
}

func (api *API) handleAdminAccountByID(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "owner", "developer")
	if !ok {
		return
	}
	idText := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/admins/"), "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}
	var accountRole string
	if err := api.DB.QueryRow(`SELECT rank FROM superusers WHERE id=?;`, id).Scan(&accountRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		} else {
			writeAdminDataError(w, err)
		}
		return
	}
	if accountRole != "admin" && accountRole != "owner" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "this account is managed from the Developer workspace"})
		return
	}
	if accountRole == "owner" && user.Role != "developer" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Owner accounts are view-only from your account"})
		return
	}
	if id == user.ID && r.Method == http.MethodDelete {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "you cannot delete the account you are currently using"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var request adminAccountRequest
		if err := readJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account request"})
			return
		}
		if request.Role == "" {
			request.Role = accountRole
		}
		if err := validateAdminAccountRequest(&request, false); err != nil {
			writeAdminDataError(w, err)
			return
		}
		if user.Role != "developer" && request.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Owner accounts can manage administrators only"})
			return
		}
		tx, err := api.DB.Begin()
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		defer tx.Rollback()
		now := database.UnixTime()
		imageBlob, imageMime, imageErr := decodeProfileImageBlob(request.ImageData)
		if imageErr != nil {
			writeAdminDataError(w, imageErr)
			return
		}
		linksJSON := encodeProfileLinks(request.Links)
		if len(imageBlob) > 0 {
			_, err = tx.Exec(`UPDATE superusers SET first_name=?,last_name=?,email=?,phone=?,links_json=?,profile_image_blob=?,profile_image_mime=?,rank=?,status=?,updated_at=? WHERE id=?;`, request.FirstName, request.LastName, request.Email, nullIfEmpty(request.Phone), linksJSON, imageBlob, imageMime, request.Role, request.Status, now, id)
		} else {
			_, err = tx.Exec(`UPDATE superusers SET first_name=?,last_name=?,email=?,phone=?,links_json=?,rank=?,status=?,updated_at=? WHERE id=?;`, request.FirstName, request.LastName, request.Email, nullIfEmpty(request.Phone), linksJSON, request.Role, request.Status, now, id)
		}
		if err != nil {
			writeAdminDataError(w, normalizeConstraintError(err))
			return
		}
		if request.Password != "" {
			hash, err := hashPassword(request.Password)
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
		api.logAction(user.ID, "staff.update", "superuser", id, fmt.Sprintf("Updated %s account", request.Role))
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "role": request.Role})
	case http.MethodDelete:
		result, err := api.DB.Exec(`DELETE FROM superusers WHERE id=?;`, id)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
			return
		}
		api.logAction(user.ID, "staff.delete", "superuser", id, fmt.Sprintf("Deleted %s account", accountRole))
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodPut, http.MethodDelete)
	}
}

func (api *API) handleAdminRates(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		var base, tenant, created, updated int64
		var note, changedByName, changedByRank string
		if err := api.DB.QueryRow(`
			SELECT p.base_rate_units,p.tenant_rate_units,p.created_at,p.updated_at,COALESCE(p.change_note,''),
			       COALESCE(TRIM(s.first_name||' '||s.last_name),''),COALESCE(s.rank,'')
			FROM price_configuration p
			LEFT JOIN superusers s ON s.id=p.updated_by
			WHERE p.id=1;
		`).Scan(&base, &tenant, &created, &updated, &note, &changedByName, &changedByRank); err != nil {
			writeAdminDataError(w, err)
			return
		}
		income := tenant - base
		markup := 0.0
		if base > 0 {
			markup = float64(income) / float64(base) * 100
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"base_rate": database.FormatMoneyUnits(base), "tenant_rate": database.FormatMoneyUnits(tenant),
			"income_per_kwh": database.FormatMoneyUnits(income), "markup_percentage": fmt.Sprintf("%.2f", markup),
			"created_at": created, "updated_at": updated, "change_note": note, "updated_by_name": changedByName, "updated_by_rank": changedByRank,
		})
	case http.MethodPut:
		var request adminRateRequest
		if err := readJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid rate request"})
			return
		}
		base, err := database.ParseMoneyUnits(request.BaseRate)
		if err != nil || base < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "base rate must be a non-negative amount with up to 5 decimals"})
			return
		}
		tenant, err := database.ParseMoneyUnits(request.TenantRate)
		if err != nil || tenant < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant rate must be a non-negative amount with up to 5 decimals"})
			return
		}
		now := database.UnixTime()
		request.ChangeNote = strings.TrimSpace(request.ChangeNote)
		if len(request.ChangeNote) > 240 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "change note must be 240 characters or fewer"})
			return
		}
		changedBy := user.ID
		if _, err := api.DB.Exec(`UPDATE price_configuration SET base_rate_units=?,tenant_rate_units=?,change_note=?,updated_by=?,updated_at=? WHERE id=1;`, base, tenant, nullIfEmpty(request.ChangeNote), changedBy, now); err != nil {
			writeAdminDataError(w, err)
			return
		}
		income := tenant - base
		markup := 0.0
		if base > 0 {
			markup = float64(income) / float64(base) * 100
		}
		api.logAction(user.ID, "rate.update", "price_configuration", 1, fmt.Sprintf("Updated electricity rates to base %s and tenant %s", database.FormatMoneyUnits(base), database.FormatMoneyUnits(tenant)))
		writeJSON(w, http.StatusOK, map[string]any{
			"base_rate": database.FormatMoneyUnits(base), "tenant_rate": database.FormatMoneyUnits(tenant),
			"income_per_kwh": database.FormatMoneyUnits(income), "markup_percentage": fmt.Sprintf("%.2f", markup), "updated_at": now,
			"change_note": request.ChangeNote, "updated_by_name": strings.TrimSpace(user.FirstName + " " + user.LastName), "updated_by_rank": user.Role,
		})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

func (api *API) handleAdminNotificationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireRoles(w, r, "admin", "owner", "developer"); !ok {
		return
	}
	configured := api.Mailer != nil && api.Mailer.Configured()
	if !configured {
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "configured": false, "online": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := api.Mailer.Available(ctx)
	writeJSON(w, http.StatusOK, map[string]any{"available": err == nil, "configured": true, "online": err == nil})
}

func (api *API) handleAdminNotificationSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	if api.Mailer == nil || !api.Mailer.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "SMTP email is not configured"})
		return
	}
	var request adminNotificationRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid notification request"})
		return
	}
	request.Audience = strings.ToLower(strings.TrimSpace(request.Audience))
	request.Subject = strings.TrimSpace(request.Subject)
	request.Preset = strings.ToLower(strings.TrimSpace(request.Preset))
	request.Message = strings.TrimSpace(request.Message)
	if request.Subject == "" || request.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "subject and message are required"})
		return
	}
	recipients, err := api.notificationRecipients(request.Audience, request.CustomEmails)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	if len(recipients) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no active email recipients were found"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	if err := api.Mailer.Available(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "internet or email service is unavailable"})
		return
	}
	result, err := api.Mailer.SendNotification(ctx, recipients, request.Subject, request.Message, request.Preset, request.ManualHTML)
	if err != nil {
		message := "unable to send notification email"
		if errors.Is(err, appmailer.ErrNotConfigured) {
			message = "SMTP email is not configured"
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": message})
		return
	}
	api.logAction(user.ID, "notification.send", "email", 0, fmt.Sprintf("Sent email notification to %d recipient(s)", result.Sent))
	writeJSON(w, http.StatusOK, map[string]any{"sent": result.Sent, "failed": result.Failed})
}

func (api *API) notificationRecipients(audience string, custom []string) ([]string, error) {
	seen := map[string]bool{}
	recipients := []string{}
	add := func(email string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || seen[email] {
			return
		}
		if _, err := stdmail.ParseAddress(email); err != nil {
			return
		}
		seen[email] = true
		recipients = append(recipients, email)
	}
	if audience == "custom" {
		for _, email := range custom {
			add(email)
		}
		if len(recipients) == 0 {
			return nil, errors.New("enter at least one valid custom email")
		}
		return recipients, nil
	}
	if audience == "all" {
		rows, err := api.DB.Query(`SELECT email FROM users WHERE status='active' AND email IS NOT NULL AND TRIM(email)<>'' UNION SELECT email FROM superusers WHERE status='active' AND TRIM(email)<>'' ORDER BY 1;`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var email string
			if err := rows.Scan(&email); err != nil {
				return nil, err
			}
			add(email)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return recipients, nil
	}
	roleMap := map[string]string{"admins": "admin", "owners": "owner", "developers": "developer"}
	role, ok := roleMap[audience]
	if !ok {
		return nil, errors.New("invalid notification audience")
	}
	rows, err := api.DB.Query(`SELECT email FROM superusers WHERE status='active' AND rank=? AND TRIM(email)<>'' ORDER BY id;`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		add(email)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return recipients, nil
}

func (api *API) handleAdminProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		var phone, image, imageMime, birthDate, linksJSON string
		var imageBlob []byte
		if err := api.DB.QueryRow(`SELECT COALESCE(phone,''),COALESCE(profile_image,''),COALESCE(profile_image_blob,X''),COALESCE(profile_image_mime,''),COALESCE(birth_date,''),COALESCE(links_json,'{}') FROM superusers WHERE id=?;`, user.ID).Scan(&phone, &image, &imageBlob, &imageMime, &birthDate, &linksJSON); err != nil {
			writeAdminDataError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"profile": map[string]any{
			"id": user.ID, "first_name": user.FirstName, "last_name": user.LastName,
			"email": user.Email, "phone": phone, "birth_date": birthDate,
			"role": user.Role, "image_url": userProfileImageURL(image, imageBlob, imageMime), "links": decodeProfileLinks(linksJSON),
		}})
		return
	}
	if r.Method != http.MethodPut {
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
		return
	}
	var request profileUpdateRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid profile request"})
		return
	}
	request.FirstName = strings.TrimSpace(request.FirstName)
	request.LastName = strings.TrimSpace(request.LastName)
	request.Phone = strings.TrimSpace(request.Phone)
	if request.CurrentPassword == "" || request.FirstName == "" || request.LastName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "current password, first name, and last name are required"})
		return
	}
	birthDate, err := normalizeProfileBirthDate(request.BirthDate)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	imageBlob, imageMime, err := decodeProfileImageBlob(request.ImageData)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	linksJSON := encodeProfileLinks(request.Links)
	var passwordHash string
	var superuserID int64
	if err := api.DB.QueryRow(`SELECT id,password_hash FROM superusers WHERE id=?;`, user.ID).Scan(&superuserID, &passwordHash); err != nil {
		writeAdminDataError(w, err)
		return
	}
	if !verifyPassword(request.CurrentPassword, passwordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}
	if len(imageBlob) > 0 {
		_, err = api.DB.Exec(`UPDATE superusers SET first_name=?,last_name=?,phone=?,birth_date=?,links_json=?,profile_image_blob=?,profile_image_mime=?,updated_at=? WHERE id=?;`, request.FirstName, request.LastName, nullIfEmpty(request.Phone), nullIfEmpty(birthDate), linksJSON, imageBlob, imageMime, database.UnixTime(), user.ID)
	} else {
		_, err = api.DB.Exec(`UPDATE superusers SET first_name=?,last_name=?,phone=?,birth_date=?,links_json=?,updated_at=? WHERE id=?;`, request.FirstName, request.LastName, nullIfEmpty(request.Phone), nullIfEmpty(birthDate), linksJSON, database.UnixTime(), user.ID)
	}
	if err != nil {
		writeAdminDataError(w, normalizeConstraintError(err))
		return
	}
	api.logAction(user.ID, "profile.update", "superuser", superuserID, "Updated account profile")
	writeJSON(w, http.StatusOK, map[string]any{"updated": true})
}

func (api *API) logAction(superuserID int64, action, entityType string, entityID int64, message string) {
	var entity any
	if entityID > 0 {
		entity = entityID
	}
	_, _ = api.DB.Exec(`INSERT INTO logs(actor_id,level,action,entity_type,entity_id,message,created_at) VALUES(?,'info',?,?,?,?,?);`, superuserID, action, entityType, entity, message, database.UnixTime())
}

func nullIfEmpty(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func normalizeConstraintError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed") {
		if strings.Contains(message, "email") {
			return errors.New("that email address is already in use")
		}
		if strings.Contains(message, "uid") {
			return errors.New("that RFID card UID is already registered")
		}
		return errors.New("the requested value conflicts with an existing record")
	}
	return err
}

func writeAdminDataError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "record not found"})
	case strings.Contains(strings.ToLower(err.Error()), "cannot be permanently deleted"):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case strings.Contains(strings.ToLower(err.Error()), "required"), strings.Contains(strings.ToLower(err.Error()), "invalid"), strings.Contains(strings.ToLower(err.Error()), "already"), strings.Contains(strings.ToLower(err.Error()), "at most 5"), strings.Contains(strings.ToLower(err.Error()), "valid "):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}
