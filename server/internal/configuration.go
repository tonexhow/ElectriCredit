package internal

import (
	"database/sql"
	"net/http"

	"electricredit/database"
)

func (api *API) registerPublicConfigurationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/public/home", api.handlePublicHome)
	mux.HandleFunc("/api/public/about", api.handlePublicAbout)
}

func (api *API) handlePublicHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	var baseRateUnits int64

	if err := api.DB.QueryRow(`
		SELECT tenant_rate_units
		FROM price_configuration
		WHERE id = 1;
	`).Scan(&baseRateUnits); err != nil && err != sql.ErrNoRows {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to load home information",
		})
		return
	}

	var registryCount, hubCount, userCount int64

	_ = api.DB.QueryRow(`SELECT COUNT(*) FROM registries;`).Scan(&registryCount)
	_ = api.DB.QueryRow(`SELECT COUNT(*) FROM hubs;`).Scan(&hubCount)
	_ = api.DB.QueryRow(`SELECT COUNT(*) FROM users;`).Scan(&userCount)

	writeJSON(w, http.StatusOK, map[string]any{
		"price_per_kwh":  database.FormatMoneyUnits(baseRateUnits),
		"registry_count": registryCount,
		"hub_count":      hubCount,
		"user_count":     userCount,
	})
}

func (api *API) handlePublicAbout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	rows, err := api.DB.Query(`
		SELECT
			s.first_name || ' ' || s.last_name,
			s.rank,
			COALESCE(s.profile_image, ''),COALESCE(s.profile_image_blob,X''),COALESCE(s.profile_image_mime,'')
		FROM superusers s
		WHERE s.status = 'active'
		ORDER BY
			CASE s.rank
				WHEN 'owner' THEN 1
				WHEN 'developer' THEN 2
				WHEN 'admin' THEN 3
				ELSE 4
			END,
			s.first_name,
			s.last_name;
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to load team information",
		})
		return
	}
	defer rows.Close()

	response := map[string]any{
		"owner":      nil,
		"developers": []map[string]string{},
		"admins":     []map[string]string{},
	}

	developers := []map[string]string{}
	admins := []map[string]string{}

	for rows.Next() {
		var name, role, imagePath, imageMime string
		var imageBlob []byte

		if err := rows.Scan(&name, &role, &imagePath, &imageBlob, &imageMime); err != nil {
			continue
		}

		member := map[string]string{
			"name":      name,
			"role":      role,
			"image_url": userProfileImageURL(imagePath, imageBlob, imageMime),
		}

		switch role {
		case "owner":
			if response["owner"] == nil {
				response["owner"] = member
			}
		case "developer":
			developers = append(developers, member)
		case "admin":
			admins = append(admins, member)
		}
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to finish loading team information",
		})
		return
	}

	response["developers"] = developers
	response["admins"] = admins

	writeJSON(w, http.StatusOK, response)
}
