package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	appmailer "electricredit/internal/mailer"
)

type API struct {
	DB                   *sql.DB
	Mailer               *appmailer.Mailer
	RecoveryMu           sync.Mutex
	Chat                 *ChatHub
	ModuleControl        *moduleControlHub
	CardScans            *cardScanHub
	ManagementMu         sync.Mutex
	ManagementChallenges map[string]developerChallenge
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	api := &API{
		DB:            db,
		Mailer:        appmailer.NewFromEnv(),
		Chat:          newChatHub(),
		ModuleControl: newModuleControlHub(),
		CardScans:     newCardScanHub(),
	}

	api.registerAuthRoutes(mux)
	api.registerRecoveryRoutes(mux)
	api.registerPublicConfigurationRoutes(mux)
	api.registerPublicHardwareRoutes(mux)
	api.registerPublicUserRoutes(mux)
	api.registerAdminRoutes(mux)
	api.registerDeveloperRoutes(mux)
	api.registerModuleControlRoutes(mux)
	api.registerModuleRuntimeRoutes(mux)
	api.registerChatRoutes(mux)
	newModuleMQTTBridge(api).start(context.Background())
}

func WithLocalDevCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if isAllowedDevelopmentOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-ElectriCredit-Challenge")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAllowedDevelopmentOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:")
}

func (api *API) registerPublicHardwareRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/public/registries", api.handlePublicRegistries)
	mux.HandleFunc("/api/public/hubs", api.handlePublicHubs)
	mux.HandleFunc("/api/public/support/availability", api.handleSupportAvailability)
	mux.HandleFunc("/api/public/support/events", api.handleSupportEvents)
}

func (api *API) handleSupportEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "live support status is unavailable"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	send := func() bool {
		payload, _ := json.Marshal(map[string]any{"available": api.Chat.activeStaffCount()})
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send() {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

func (api *API) handlePublicRegistries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	rows, err := api.DB.Query(`
		SELECT
			r.id,
			r.name,
			COALESCE(r.location, ''),
			r.status,
			EXISTS(
				SELECT 1
				FROM registry_sessions rs
				WHERE rs.registry_id = r.id
				  AND rs.status = 'active'
			) AS in_use,
			COALESCE((
				SELECT rs.card_id
				FROM registry_sessions rs
				WHERE rs.registry_id = r.id
				  AND rs.status = 'active'
				ORDER BY rs.id DESC
				LIMIT 1
			), 0) AS active_card_id
		FROM registries r
		ORDER BY r.name, r.id;
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to load Registry Stations",
		})
		return
	}
	defer rows.Close()

	registries := []map[string]any{}

	for rows.Next() {
		var (
			id           int64
			name         string
			location     string
			status       string
			inUse        bool
			activeCardID int64
		)

		if err := rows.Scan(
			&id,
			&name,
			&location,
			&status,
			&inUse,
			&activeCardID,
		); err != nil {
			continue
		}

		registries = append(registries, map[string]any{
			"id":             id,
			"public_id":      fmt.Sprintf("REG-%03d", id),
			"name":           name,
			"location":       location,
			"status":         publicModuleStatus(status, inUse),
			"active_card_id": activeCardID,
		})
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to finish loading Registry Stations",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registries": registries,
	})
}

func (api *API) handlePublicHubs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	rows, err := api.DB.Query(`
		SELECT
			h.id,
			h.name,
			COALESCE(h.location, ''),
			h.status,
			EXISTS(
				SELECT 1
				FROM power_sessions ps
				WHERE ps.hub_id = h.id
				  AND ps.status = 'active'
			) AS in_use,
			COALESCE((
				SELECT ps.card_id
				FROM power_sessions ps
				WHERE ps.hub_id = h.id
				  AND ps.status = 'active'
				ORDER BY ps.id DESC
				LIMIT 1
			), 0) AS active_card_id
		FROM hubs h
		ORDER BY h.name, h.id;
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to load Hub Modules",
		})
		return
	}
	defer rows.Close()

	hubs := []map[string]any{}

	for rows.Next() {
		var (
			id           int64
			name         string
			location     string
			status       string
			inUse        bool
			activeCardID int64
		)

		if err := rows.Scan(
			&id,
			&name,
			&location,
			&status,
			&inUse,
			&activeCardID,
		); err != nil {
			continue
		}

		hubs = append(hubs, map[string]any{
			"id":             id,
			"public_id":      fmt.Sprintf("HUB-%03d", id),
			"name":           name,
			"location":       location,
			"status":         publicModuleStatus(status, inUse),
			"active_card_id": activeCardID,
		})
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to finish loading Hub Modules",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"hubs": hubs,
	})
}

func (api *API) handleSupportAvailability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	available := 0
	if api.Chat != nil {
		available = api.Chat.activeStaffCount()
	}
	writeJSON(w, http.StatusOK, map[string]any{"available": available})
}

func publicModuleStatus(status string, inUse bool) string {
	if status == "disabled" || status == "inactive" {
		return "disabled"
	}

	if inUse {
		return "in-use"
	}

	return "idle"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	_ = json.NewEncoder(w).Encode(payload)
}

func readJSON(r *http.Request, destination any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var extra any

	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}

	return nil
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
	}

	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
		"error": "method not allowed",
	})
}
