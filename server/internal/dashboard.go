package internal

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"electricredit/database"
)

type dashboardWindow struct {
	From  int64
	To    int64
	Label string
	Key   string
}

func (api *API) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireRoles(w, r, "admin", "owner", "developer"); !ok {
		return
	}

	window, err := parseDashboardWindow(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var totalBalance, activeTenants, cardCount int64
	if err := api.DB.QueryRow(`SELECT COALESCE(SUM(balance_units),0),COUNT(*) FROM cards;`).Scan(&totalBalance, &cardCount); err != nil {
		writeAdminDataError(w, err)
		return
	}
	if err := api.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE status='active';`).Scan(&activeTenants); err != nil {
		writeAdminDataError(w, err)
		return
	}

	var totalEnergyWh, sessionCount, activeSessions, recordedRevenue int64
	if err := api.DB.QueryRow(`SELECT COALESCE(SUM(energy_wh),0),COUNT(*),COALESCE(SUM(CASE WHEN status='active' THEN 1 ELSE 0 END),0),COALESCE(SUM(cost_units),0) FROM power_sessions WHERE started_at>=? AND started_at<?;`, window.From, window.To).Scan(&totalEnergyWh, &sessionCount, &activeSessions, &recordedRevenue); err != nil {
		writeAdminDataError(w, err)
		return
	}

	activeStaff, err := api.listActiveStaff()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	powerTrend, err := api.dashboardPowerTrend(window)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	hubs, err := api.dashboardHubRows(window)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	tenants, err := api.dashboardTenantRows(window)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	hourly, err := api.dashboardHourlyUsage(window)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	recent, err := api.dashboardRecentSessions(window)
	if err != nil {
		writeAdminDataError(w, err)
		return
	}
	hardwareWarnings, err := api.dashboardHardwareWarnings()
	if err != nil {
		writeAdminDataError(w, err)
		return
	}

	base, tenantRate, income, markup := api.rateSummary()
	estimatedBaseCost := dashboardEnergyCost(totalEnergyWh, base)
	projectedIncome := dashboardEnergyCost(totalEnergyWh, income)
	grossProfit := recordedRevenue - estimatedBaseCost
	profitMargin := 0.0
	if recordedRevenue > 0 {
		profitMargin = float64(grossProfit) / float64(recordedRevenue) * 100
	}
	busyHour, busyKWh := peakHour(hourly)
	topTenant := topDashboardRow(tenants, "kwh")
	topHub := topDashboardRow(hubs, "kwh")
	avgSessionKWh := 0.0
	if sessionCount > 0 {
		avgSessionKWh = float64(totalEnergyWh) / 1000 / float64(sessionCount)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"filter": map[string]any{"key": window.Key, "label": window.Label, "from": window.From, "to": window.To},
		"summary": map[string]any{
			"total_balance":   database.FormatMoneyUnits(totalBalance),
			"active_tenants":  activeTenants,
			"cards":           cardCount,
			"staff_online":    len(activeStaff),
			"total_kwh":       float64(totalEnergyWh) / 1000,
			"sessions":        sessionCount,
			"active_sessions": activeSessions,
			"average_kwh":     avgSessionKWh,
			"busy_hour":       busyHour,
			"busy_hour_kwh":   busyKWh,
		},
		"financials": map[string]any{
			"revenue":             database.FormatMoneyUnits(recordedRevenue),
			"estimated_base_cost": database.FormatMoneyUnits(estimatedBaseCost),
			"projected_income":    database.FormatMoneyUnits(projectedIncome),
			"gross_profit":        database.FormatMoneyUnits(grossProfit),
			"profit_margin":       fmt.Sprintf("%.2f", profitMargin),
		},
		"highlights":  map[string]any{"top_consumer": topTenant, "top_hub": topHub},
		"power_trend": powerTrend, "hubs": hubs, "tenants": tenants, "hourly_usage": hourly,
		"recent_sessions": recent, "active_staff": activeStaff, "hardware_warnings": hardwareWarnings,
		"rates": map[string]any{
			"base_rate": database.FormatMoneyUnits(base), "tenant_rate": database.FormatMoneyUnits(tenantRate),
			"income_per_kwh": database.FormatMoneyUnits(income), "markup_percentage": fmt.Sprintf("%.2f", markup),
		},
	})
}

func (api *API) dashboardHardwareWarnings() ([]map[string]any, error) {
	rows, err := api.DB.Query(`
		SELECT 'Hub' AS module_type,id,name,COALESCE(location,''),COALESCE(last_warning,''),COALESCE(last_seen_at,0)
		FROM hubs WHERE health_status='warning'
		UNION ALL
		SELECT 'Registry',id,name,COALESCE(location,''),COALESCE(last_warning,''),COALESCE(last_seen_at,0)
		FROM registries WHERE health_status='warning'
		ORDER BY 6 DESC LIMIT 8;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var kind, name, location, warning string
		var id, seen int64
		if err := rows.Scan(&kind, &id, &name, &location, &warning, &seen); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"type": kind, "id": id, "name": name, "location": location, "warning": warning, "last_seen_at": seen})
	}
	return items, rows.Err()
}

func dashboardEnergyCost(energyWh, rateUnits int64) int64 {
	if energyWh <= 0 || rateUnits == 0 {
		return 0
	}
	// Dashboard figures are estimates at the current rate. Use integer
	// arithmetic so the five-decimal money scale stays deterministic.
	product := energyWh * rateUnits
	if product >= 0 {
		return (product + 500) / 1000
	}
	return (product - 500) / 1000
}

func parseDashboardWindow(r *http.Request) (dashboardWindow, error) {
	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()).Unix()
	key := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("range")))
	if key == "" {
		key = "30d"
	}
	window := dashboardWindow{To: end, Key: key}
	switch key {
	case "7d":
		window.From = time.Unix(end, 0).AddDate(0, 0, -7).Unix()
		window.Label = "Last 7 days"
	case "30d":
		window.From = time.Unix(end, 0).AddDate(0, 0, -30).Unix()
		window.Label = "Last 30 days"
	case "90d":
		window.From = time.Unix(end, 0).AddDate(0, 0, -90).Unix()
		window.Label = "Last 90 days"
	case "all":
		window.From = 0
		window.Label = "All recorded activity"
	case "custom":
		fromText := strings.TrimSpace(r.URL.Query().Get("from"))
		toText := strings.TrimSpace(r.URL.Query().Get("to"))
		from, err := time.ParseInLocation("2006-01-02", fromText, now.Location())
		if err != nil {
			return dashboardWindow{}, fmt.Errorf("choose a valid start date")
		}
		to, err := time.ParseInLocation("2006-01-02", toText, now.Location())
		if err != nil {
			return dashboardWindow{}, fmt.Errorf("choose a valid end date")
		}
		if to.Before(from) {
			return dashboardWindow{}, fmt.Errorf("end date must be on or after the start date")
		}
		window.From = from.Unix()
		window.To = to.AddDate(0, 0, 1).Unix()
		window.Label = fmt.Sprintf("%s to %s", from.Format("Jan 2, 2006"), to.Format("Jan 2, 2006"))
	default:
		return dashboardWindow{}, fmt.Errorf("invalid dashboard date range")
	}
	return window, nil
}

func (api *API) dashboardPowerTrend(window dashboardWindow) ([]map[string]any, error) {
	rows, err := api.DB.Query(`
		SELECT strftime('%Y-%m-%d',started_at,'unixepoch','localtime') AS day,
		       COALESCE(SUM(energy_wh),0),COUNT(*),COALESCE(SUM(cost_units),0)
		FROM power_sessions WHERE started_at>=? AND started_at<?
		GROUP BY day ORDER BY day;`, window.From, window.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var day string
		var energyWh, sessions, cost int64
		if err := rows.Scan(&day, &energyWh, &sessions, &cost); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"date": day, "kwh": float64(energyWh) / 1000, "sessions": sessions, "cost": database.FormatMoneyUnits(cost)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (api *API) dashboardHubRows(window dashboardWindow) ([]map[string]any, error) {
	rows, err := api.DB.Query(`
		SELECT h.id,h.name,COALESCE(h.location,''),h.status,
		       COUNT(ps.id),COALESCE(SUM(ps.energy_wh),0),COALESCE(SUM(ps.cost_units),0)
		FROM hubs h LEFT JOIN power_sessions ps ON ps.hub_id=h.id AND ps.started_at>=? AND ps.started_at<?
		GROUP BY h.id,h.name,h.location,h.status ORDER BY SUM(COALESCE(ps.energy_wh,0)) DESC,h.id;`, window.From, window.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, sessions, energyWh, revenue int64
		var name, location, status string
		if err := rows.Scan(&id, &name, &location, &status, &sessions, &energyWh, &revenue); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "name": name, "location": location, "status": status, "sessions": sessions, "kwh": float64(energyWh) / 1000, "revenue": database.FormatMoneyUnits(revenue)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (api *API) dashboardTenantRows(window dashboardWindow) ([]map[string]any, error) {
	rows, err := api.DB.Query(`
		WITH card_totals AS (
			SELECT user_id,COUNT(*) card_count,COALESCE(SUM(balance_units),0) balance_units FROM cards GROUP BY user_id
		), usage_totals AS (
			SELECT c.user_id,COUNT(ps.id) session_count,COALESCE(SUM(ps.energy_wh),0) energy_wh,COALESCE(SUM(ps.cost_units),0) cost_units
			FROM cards c LEFT JOIN power_sessions ps ON ps.card_id=c.id AND ps.started_at>=? AND ps.started_at<? GROUP BY c.user_id
		)
		SELECT u.id,u.first_name || ' ' || u.last_name,COALESCE(u.email,''),u.status,
		       COALESCE(ct.card_count,0),COALESCE(ut.session_count,0),COALESCE(ut.energy_wh,0),COALESCE(ut.cost_units,0),COALESCE(ct.balance_units,0)
		FROM users u LEFT JOIN card_totals ct ON ct.user_id=u.id LEFT JOIN usage_totals ut ON ut.user_id=u.id
		ORDER BY COALESCE(ut.energy_wh,0) DESC,u.id;`, window.From, window.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, cards, sessions, energyWh, cost, balance int64
		var name, email, status string
		if err := rows.Scan(&id, &name, &email, &status, &cards, &sessions, &energyWh, &cost, &balance); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "name": name, "email": email, "status": status, "cards": cards, "sessions": sessions, "kwh": float64(energyWh) / 1000, "cost": database.FormatMoneyUnits(cost), "balance": database.FormatMoneyUnits(balance)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (api *API) dashboardHourlyUsage(window dashboardWindow) ([]map[string]any, error) {
	rows, err := api.DB.Query(`SELECT CAST(strftime('%H',started_at,'unixepoch','localtime') AS INTEGER) hour,COALESCE(SUM(energy_wh),0),COUNT(*) FROM power_sessions WHERE started_at>=? AND started_at<? GROUP BY hour ORDER BY hour;`, window.From, window.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byHour := make(map[int]map[string]any, 24)
	for rows.Next() {
		var hour int
		var energyWh, sessions int64
		if err := rows.Scan(&hour, &energyWh, &sessions); err != nil {
			return nil, err
		}
		byHour[hour] = map[string]any{"hour": hour, "kwh": float64(energyWh) / 1000, "sessions": sessions}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, 24)
	for hour := 0; hour < 24; hour++ {
		if item, ok := byHour[hour]; ok {
			items = append(items, item)
		} else {
			items = append(items, map[string]any{"hour": hour, "kwh": 0.0, "sessions": 0})
		}
	}
	return items, nil
}

func (api *API) dashboardRecentSessions(window dashboardWindow) ([]map[string]any, error) {
	rows, err := api.DB.Query(`
		SELECT ps.id,u.first_name || ' ' || u.last_name,c.id,c.uid,h.name,ps.started_at,
		       COALESCE(ps.ended_at,0),ps.energy_wh,ps.cost_units,ps.status
		FROM power_sessions ps JOIN cards c ON c.id=ps.card_id JOIN users u ON u.id=c.user_id JOIN hubs h ON h.id=ps.hub_id
		WHERE ps.started_at>=? AND ps.started_at<? ORDER BY ps.started_at DESC,ps.id DESC LIMIT 30;`, window.From, window.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, cardID, started, ended, energyWh, cost int64
		var tenant, uid, hub, status string
		if err := rows.Scan(&id, &tenant, &cardID, &uid, &hub, &started, &ended, &energyWh, &cost, &status); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "tenant": tenant, "card_id": cardID, "card_uid": uid, "hub": hub, "started_at": started, "ended_at": ended, "kwh": float64(energyWh) / 1000, "cost": database.FormatMoneyUnits(cost), "status": status})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func peakHour(items []map[string]any) (string, float64) {
	bestHour := 0
	bestKWh := 0.0
	for _, item := range items {
		hour, _ := item["hour"].(int)
		kwh, _ := item["kwh"].(float64)
		if kwh > bestKWh {
			bestHour, bestKWh = hour, kwh
		}
	}
	if bestKWh == 0 {
		return "—", 0
	}
	return fmt.Sprintf("%02d:00", bestHour), bestKWh
}

func topDashboardRow(items []map[string]any, metric string) map[string]any {
	if len(items) == 0 {
		return map[string]any{}
	}
	best := items[0]
	bestValue, _ := best[metric].(float64)
	for _, item := range items[1:] {
		value, _ := item[metric].(float64)
		if value > bestValue {
			best, bestValue = item, value
		}
	}
	if bestValue <= 0 {
		return map[string]any{}
	}
	return best
}

func (api *API) listActiveStaff() ([]map[string]any, error) {
	now := database.UnixTime()
	cutoff := now - int64((5 * time.Minute).Seconds())
	rows, err := api.DB.Query(`
		SELECT s.id,s.first_name,s.last_name,s.email,s.rank,COALESCE(s.profile_image,''),COALESCE(s.profile_image_blob,X''),COALESCE(s.profile_image_mime,''),MAX(ls.last_seen_at)
		FROM login_sessions ls JOIN superusers s ON s.id=ls.superuser_id
		WHERE ls.expires_at>? AND ls.last_seen_at>=? AND s.status='active' AND s.rank IN ('admin','owner','developer')
		GROUP BY s.id,s.first_name,s.last_name,s.email,s.rank,s.profile_image,s.profile_image_blob,s.profile_image_mime
		ORDER BY MAX(ls.last_seen_at) DESC,s.id;`, now, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	staff := []map[string]any{}
	for rows.Next() {
		var id, lastSeen int64
		var first, last, email, role, image, imageMime string
		var imageBlob []byte
		if err := rows.Scan(&id, &first, &last, &email, &role, &image, &imageBlob, &imageMime, &lastSeen); err != nil {
			return nil, err
		}
		staff = append(staff, map[string]any{"id": id, "first_name": first, "last_name": last, "email": email, "role": role, "image_url": userProfileImageURL(image, imageBlob, imageMime), "last_seen_at": lastSeen})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return staff, nil
}

func (api *API) rateSummary() (base, tenant, income int64, markup float64) {
	_ = api.DB.QueryRow(`SELECT base_rate_units,tenant_rate_units FROM price_configuration WHERE id=1;`).Scan(&base, &tenant)
	income = tenant - base
	if base > 0 {
		markup = float64(income) / float64(base) * 100
	}
	return
}
