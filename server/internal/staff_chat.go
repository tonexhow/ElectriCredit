package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"electricredit/database"
)

type staffChatRequest struct {
	Message string `json:"message"`
}

type staffTypingRequest struct {
	Typing bool `json:"typing"`
}

type staffChatMessage struct {
	ID        int64  `json:"id"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	ImageURL  string `json:"image_url"`
}

func (api *API) registerStaffChatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/staff-chat/messages", api.handleStaffChatMessages)
	mux.HandleFunc("/api/admin/staff-chat/typing", api.handleStaffChatTyping)
}

func (api *API) handleStaffChatMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		messages, err := api.listStaffChatMessages(120)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
	case http.MethodPost:
		var request staffChatRequest
		if err := readJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid team message"})
			return
		}
		request.Message = strings.TrimSpace(request.Message)
		if request.Message == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "write a message first"})
			return
		}
		if len([]rune(request.Message)) > 4000 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is too long"})
			return
		}
		superuserID := user.ID
		now := database.UnixTime()
		result, err := api.DB.Exec(`INSERT INTO staff_group_messages(superuser_id,message,created_at,updated_at) VALUES(?,?,?,?);`, superuserID, request.Message, now, now)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		id, _ := result.LastInsertId()
		message, err := api.getStaffChatMessage(id)
		if err != nil {
			writeAdminDataError(w, err)
			return
		}
		payload, _ := json.Marshal(message)
		api.Chat.broadcastStaff(chatEvent{Type: "staff-group-message", Data: payload}, "")
		writeJSON(w, http.StatusCreated, map[string]any{"message": message})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (api *API) handleStaffChatTyping(w http.ResponseWriter, r *http.Request) {
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request staffTypingRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid typing state"})
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"user_id": user.ID,
		"name":    strings.TrimSpace(user.FirstName + " " + user.LastName),
		"role":    user.Role,
		"typing":  request.Typing,
	})
	api.Chat.broadcastStaff(chatEvent{Type: "staff-group-typing", Data: payload}, "")
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) listStaffChatMessages(limit int) ([]staffChatMessage, error) {
	if limit < 1 || limit > 500 {
		limit = 120
	}
	rows, err := api.DB.Query(`
		SELECT x.id,x.message,x.created_at,s.id,s.first_name,s.last_name,s.rank,COALESCE(s.profile_image,''),COALESCE(s.profile_image_blob,X''),COALESCE(s.profile_image_mime,'')
		FROM (
			SELECT id,superuser_id,message,created_at FROM staff_group_messages ORDER BY id DESC LIMIT ?
		) x
		JOIN superusers s ON s.id=x.superuser_id
		ORDER BY x.id ASC;
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []staffChatMessage{}
	for rows.Next() {
		var message staffChatMessage
		var image, imageMime string
		var imageBlob []byte
		if err := rows.Scan(&message.ID, &message.Message, &message.CreatedAt, &message.UserID, &message.FirstName, &message.LastName, &message.Role, &image, &imageBlob, &imageMime); err != nil {
			return nil, err
		}
		message.ImageURL = userProfileImageURL(image, imageBlob, imageMime)
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (api *API) getStaffChatMessage(id int64) (staffChatMessage, error) {
	var message staffChatMessage
	var image, imageMime string
	var imageBlob []byte
	err := api.DB.QueryRow(`
		SELECT m.id,m.message,m.created_at,s.id,s.first_name,s.last_name,s.rank,COALESCE(s.profile_image,''),COALESCE(s.profile_image_blob,X''),COALESCE(s.profile_image_mime,'')
		FROM staff_group_messages m
		JOIN superusers s ON s.id=m.superuser_id
		WHERE m.id=?;
	`, id).Scan(&message.ID, &message.Message, &message.CreatedAt, &message.UserID, &message.FirstName, &message.LastName, &message.Role, &image, &imageBlob, &imageMime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return staffChatMessage{}, sql.ErrNoRows
		}
		return staffChatMessage{}, err
	}
	message.ImageURL = userProfileImageURL(image, imageBlob, imageMime)
	return message, nil
}
