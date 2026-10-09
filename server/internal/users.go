package internal

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"electricredit/database"
)

const defaultProfileImage = "assets/default-image.png"

type publicUser struct {
	ID       int64        `json:"id"`
	Name     string       `json:"name"`
	Email    string       `json:"email"`
	Phone    string       `json:"phone"`
	Room     string       `json:"room"`
	Location string       `json:"location"`
	Status   string       `json:"status"`
	ImageURL string       `json:"image_url"`
	Cards    []publicCard `json:"cards,omitempty"`
}

type publicCard struct {
	ID        int64  `json:"id"`
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Balance   string `json:"balance"`
	DebtLimit string `json:"debt_limit"`
}

func (api *API) registerPublicUserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/public/users", api.handlePublicUsers)
	mux.HandleFunc("/api/public/users/", api.handlePublicUserDetail)
}

func (api *API) handlePublicUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	rows, err := api.DB.Query(`
		SELECT
			u.id,
			u.first_name || ' ' || u.last_name,
			COALESCE(u.email, ''),
			COALESCE(u.phone, ''),
			COALESCE(u.room, ''),
			COALESCE(u.profile_image, ''),COALESCE(u.profile_image_blob,X''),COALESCE(u.profile_image_mime,''),
			u.status
		FROM users u
		ORDER BY u.first_name, u.last_name;
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to load users",
		})
		return
	}
	defer rows.Close()

	users := []publicUser{}

	for rows.Next() {
		var imagePath, imageMime string
		var imageBlob []byte
		var user publicUser

		if err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.Phone,
			&user.Room,
			&imagePath,
			&imageBlob,
			&imageMime,
			&user.Status,
		); err != nil {
			continue
		}

		user.Location = user.Room
		user.ImageURL = userProfileImageURL(imagePath, imageBlob, imageMime)
		user.Email = maskEmailForPublic(user.Email)
		user.Phone = maskPhoneForPublic(user.Phone)
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to finish loading users",
		})
		return
	}

	for index := range users {
		cards, err := api.listPublicCards(users[index].ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "unable to load tenant cards",
			})
			return
		}
		users[index].Cards = cards
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
	})
}

func (api *API) handlePublicUserDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	identifier := strings.TrimPrefix(r.URL.Path, "/api/public/users/")
	userID, ok := parsePublicUserID(identifier)

	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid user identifier",
		})
		return
	}

	user, err := api.getPublicUser(userID)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "user not found",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to load user",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
	})
}

func (api *API) getPublicUser(userID int64) (publicUser, error) {
	var user publicUser
	var imagePath, imageMime string
	var imageBlob []byte

	err := api.DB.QueryRow(`
		SELECT
			u.id,
			u.first_name || ' ' || u.last_name,
			COALESCE(u.email, ''),
			COALESCE(u.phone, ''),
			COALESCE(u.room, ''),
			COALESCE(u.profile_image, ''),COALESCE(u.profile_image_blob,X''),COALESCE(u.profile_image_mime,''),
			u.status
		FROM users u
		WHERE u.id = ?
		LIMIT 1;
	`, userID).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Phone,
		&user.Room,
		&imagePath,
		&imageBlob,
		&imageMime,
		&user.Status,
	)
	if err != nil {
		return publicUser{}, err
	}

	user.Location = user.Room
	user.ImageURL = userProfileImageURL(imagePath, imageBlob, imageMime)
	user.Email = maskEmailForPublic(user.Email)
	user.Phone = maskPhoneForPublic(user.Phone)

	user.Cards, err = api.listPublicCards(userID)
	if err != nil {
		return publicUser{}, err
	}

	return user, nil
}

func (api *API) listPublicCards(userID int64) ([]publicCard, error) {
	rows, err := api.DB.Query(`
		SELECT c.id,c.uid,c.status,c.balance_units,COALESCE(u.debt_limit_units,(SELECT debt_limit_units FROM price_configuration WHERE id=1))
		FROM cards c JOIN users u ON u.id=c.user_id WHERE c.user_id=? ORDER BY c.id;
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []publicCard{}
	for rows.Next() {
		var card publicCard
		var balanceUnits, debtLimitUnits int64
		if err := rows.Scan(&card.ID, &card.UID, &card.Status, &balanceUnits, &debtLimitUnits); err != nil {
			return nil, err
		}
		card.Name = "RFID Card"
		card.Balance = database.FormatMoneyUnits(balanceUnits)
		card.DebtLimit = database.FormatMoneyUnits(debtLimitUnits)
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cards, nil
}

func publicImageURL(value string) string {
	value = strings.TrimSpace(value)

	if value == "" {
		return defaultProfileImage
	}

	return value
}

func parsePublicUserID(value string) (int64, bool) {
	value = strings.TrimSpace(value)

	if strings.HasPrefix(strings.ToUpper(value), "USR-") {
		value = value[4:]
	}

	id, err := strconv.ParseInt(value, 10, 64)

	return id, err == nil && id > 0
}

func maskEmailForPublic(email string) string {
	email = strings.TrimSpace(email)

	if email == "" {
		return ""
	}

	parts := strings.SplitN(email, "@", 2)

	if len(parts) != 2 {
		return maskText(email)
	}

	name := parts[0]
	visible := 2

	if len(name) < visible {
		visible = len(name)
	}

	return name[:visible] + "****@" + parts[1]
}

func maskPhoneForPublic(phone string) string {
	phone = strings.TrimSpace(phone)

	if len(phone) <= 4 {
		return maskText(phone)
	}

	return phone[:2] + "*******" + phone[len(phone)-2:]
}

func maskText(value string) string {
	if value == "" {
		return ""
	}

	if len(value) <= 2 {
		return strings.Repeat("*", len(value))
	}

	return value[:2] + "****"
}
