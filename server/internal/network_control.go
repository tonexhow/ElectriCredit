package internal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type hotspotControlRequest struct {
	Action           string `json:"action"`
	SSID             string `json:"ssid"`
	Password         string `json:"password"`
	Interface        string `json:"interface"`
	BootEnabled      *bool  `json:"boot_enabled,omitempty"`
	FallbackSSID     string `json:"fallback_ssid"`
	FallbackPassword string `json:"fallback_password"`
}

type setupNetworkInfo struct {
	Mode           string
	SSID           string
	Password       string
	PasswordMasked string
	Interface      string
	IP             string
	QRText         string
}

type wifiScanState struct {
	Status           string           `json:"status"`
	Networks         []map[string]any `json:"networks"`
	ErrorCode        string           `json:"error_code,omitempty"`
	Message          string           `json:"message,omitempty"`
	StartedAt        int64            `json:"started_at,omitempty"`
	CompletedAt      int64            `json:"completed_at,omitempty"`
	HotspotWasActive bool             `json:"hotspot_was_active"`
	HotspotRestored  bool             `json:"hotspot_restored"`
}

type wifiConnectState struct {
	Status           string `json:"status"`
	SSID             string `json:"ssid,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
	Message          string `json:"message,omitempty"`
	StartedAt        int64  `json:"started_at,omitempty"`
	CompletedAt      int64  `json:"completed_at,omitempty"`
	HotspotWasActive bool   `json:"hotspot_was_active"`
	HotspotRestored  bool   `json:"hotspot_restored"`
	Connected        bool   `json:"connected"`
}

var developerNetworkOperations = struct {
	sync.Mutex
	active  string
	scan    wifiScanState
	connect wifiConnectState
}{
	scan:    wifiScanState{Status: "idle", Networks: []map[string]any{}},
	connect: wifiConnectState{Status: "idle"},
}

func cloneWiFiNetworks(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		copyItem := make(map[string]any, len(item))
		for key, value := range item {
			copyItem[key] = value
		}
		out = append(out, copyItem)
	}
	return out
}

func currentWiFiScanState() wifiScanState {
	developerNetworkOperations.Lock()
	defer developerNetworkOperations.Unlock()
	state := developerNetworkOperations.scan
	state.Networks = cloneWiFiNetworks(state.Networks)
	return state
}

func currentWiFiConnectState() wifiConnectState {
	developerNetworkOperations.Lock()
	defer developerNetworkOperations.Unlock()
	return developerNetworkOperations.connect
}

func beginWiFiScan(hotspotActive bool) (wifiScanState, bool) {
	developerNetworkOperations.Lock()
	defer developerNetworkOperations.Unlock()
	if developerNetworkOperations.active != "" {
		return developerNetworkOperations.scan, false
	}
	developerNetworkOperations.active = "scan"
	developerNetworkOperations.scan = wifiScanState{
		Status:           "running",
		Networks:         cloneWiFiNetworks(developerNetworkOperations.scan.Networks),
		Message:          "Wi-Fi scan is running.",
		StartedAt:        time.Now().Unix(),
		HotspotWasActive: hotspotActive,
	}
	return developerNetworkOperations.scan, true
}

func finishWiFiScan(networks []map[string]any, code, message string, hotspotActive, hotspotRestored bool) {
	developerNetworkOperations.Lock()
	defer developerNetworkOperations.Unlock()
	status := "success"
	if code != "" {
		status = "failed"
	}
	developerNetworkOperations.scan = wifiScanState{
		Status:           status,
		Networks:         cloneWiFiNetworks(networks),
		ErrorCode:        code,
		Message:          message,
		StartedAt:        developerNetworkOperations.scan.StartedAt,
		CompletedAt:      time.Now().Unix(),
		HotspotWasActive: hotspotActive,
		HotspotRestored:  hotspotRestored,
	}
	developerNetworkOperations.active = ""
}

func beginWiFiConnect(ssid string, hotspotActive bool) (wifiConnectState, bool) {
	developerNetworkOperations.Lock()
	defer developerNetworkOperations.Unlock()
	if developerNetworkOperations.active != "" {
		return developerNetworkOperations.connect, false
	}
	developerNetworkOperations.active = "connect"
	developerNetworkOperations.connect = wifiConnectState{
		Status:           "running",
		SSID:             ssid,
		Message:          "Wi-Fi handover is running.",
		StartedAt:        time.Now().Unix(),
		HotspotWasActive: hotspotActive,
	}
	return developerNetworkOperations.connect, true
}

func finishWiFiConnect(ssid, code, message string, hotspotActive, hotspotRestored, connected bool) {
	developerNetworkOperations.Lock()
	defer developerNetworkOperations.Unlock()
	status := "success"
	if code != "" {
		status = "failed"
	}
	developerNetworkOperations.connect = wifiConnectState{
		Status:           status,
		SSID:             ssid,
		ErrorCode:        code,
		Message:          message,
		StartedAt:        developerNetworkOperations.connect.StartedAt,
		CompletedAt:      time.Now().Unix(),
		HotspotWasActive: hotspotActive,
		HotspotRestored:  hotspotRestored,
		Connected:        connected,
	}
	developerNetworkOperations.active = ""
}

func setupNetworkSnapshot(ctx context.Context) setupNetworkInfo {
	hotspot := detectHotspotSnapshot(ctx)
	if hotspot.Enabled && hotspot.SSID != "" {
		qr := ""
		if hotspot.Password != "" {
			qr = fmt.Sprintf("WIFI:T:WPA;S:%s;P:%s;;", escapeWiFiQR(hotspot.SSID), escapeWiFiQR(hotspot.Password))
		}
		return setupNetworkInfo{Mode: "hotspot", SSID: hotspot.SSID, Password: hotspot.Password, PasswordMasked: maskSecret(hotspot.Password), Interface: hotspot.Interface, IP: hotspot.IP, QRText: qr}
	}

	ssid := connectedWiFiSSID(ctx)
	if ssid == "" {
		return setupNetworkInfo{Mode: "none"}
	}
	password := os.Getenv("ELECTRICREDIT_WIFI_PASSWORD")
	qr := ""
	if password != "" {
		qr = fmt.Sprintf("WIFI:T:WPA;S:%s;P:%s;;", escapeWiFiQR(ssid), escapeWiFiQR(password))
	}
	iface, ip := activeWirelessInterface()
	return setupNetworkInfo{Mode: "wifi", SSID: ssid, Password: password, PasswordMasked: maskSecret(password), Interface: iface, IP: ip, QRText: qr}
}

func activeWirelessInterface() (string, string) {
	for _, iface := range activeInterfaceSnapshots() {
		if !iface.Up || classifyInterface(iface.Name) != "wireless" {
			continue
		}
		for _, raw := range iface.Addresses {
			ip := parseInterfaceIP(raw)
			if isUsableInterfaceIP(ip) {
				return iface.Name, ip.String()
			}
		}
	}
	return "", ""
}

func isUsableInterfaceIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		// 169.254/16 is an APIPA/self-assigned address and does not mean an
		// Ethernet cable has a usable network connection.
		if v4[0] == 169 && v4[1] == 254 {
			return false
		}
	}
	return true
}

func (api *API) handleDeveloperHotspot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "Hotspot control is supported on Windows and Linux hosts."})
		return
	}

	var req hotspotControlRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid hotspot request"})
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	req.SSID = strings.TrimSpace(req.SSID)
	req.Interface = strings.TrimSpace(req.Interface)
	req.FallbackSSID = strings.TrimSpace(req.FallbackSSID)

	current := detectHotspotSnapshot(r.Context())
	stored, _ := loadHotspotFileConfig()
	if req.SSID == "" {
		if current.SSID != "" {
			req.SSID = current.SSID
		} else {
			req.SSID = stored.SSID
		}
	}
	if req.Password == "" {
		if current.Password != "" {
			req.Password = current.Password
		} else {
			req.Password = stored.Password
		}
	}
	if req.Interface == "" {
		if current.Interface != "" {
			req.Interface = current.Interface
		} else {
			req.Interface = stored.Interface
		}
	}

	mode, notifySSID, notifyPassword := "hotspot", req.SSID, req.Password
	switch req.Action {
	case "start", "update":
		if req.SSID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hotspot SSID is required"})
			return
		}
		if len([]byte(req.SSID)) > 32 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hotspot SSID must contain at most 32 bytes"})
			return
		}
		if len(req.Password) < 8 || len(req.Password) > 63 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hotspot password must contain 8 to 63 characters"})
			return
		}
	case "stop":
		mode = "wifi"
		if req.FallbackSSID == "" {
			req.FallbackSSID = connectedWiFiSSID(r.Context())
		}
		if req.FallbackPassword == "" {
			req.FallbackPassword = os.Getenv("ELECTRICREDIT_WIFI_PASSWORD")
		}
		notifySSID, notifyPassword = req.FallbackSSID, req.FallbackPassword
		if notifySSID == "" {
			mode = "registration"
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose start, update, or stop"})
		return
	}

	if !api.requireManagementActionChallenge(w, r, user.ID, "network-hotspot-"+req.Action) {
		return
	}

	serverURL := preferredLocalServerURL(r)
	if req.Action != "stop" {
		ip := strings.TrimSpace(stored.IPv4Address)
		if slash := strings.Index(ip, "/"); slash >= 0 {
			ip = ip[:slash]
		}
		if ip != "" {
			serverURL = "http://" + net.JoinHostPort(ip, "8080")
		}
	}
	notified := api.broadcastModuleNetworkUpdate(mode, notifySSID, notifyPassword, serverURL, 1800)
	if notified > 0 {
		time.Sleep(1200 * time.Millisecond)
	}

	operationCtx, cancelOperation := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancelOperation()
	var applyErr error
	if runtime.GOOS == "windows" {
		applyErr = applyWindowsHotspot(operationCtx, req)
	} else {
		applyErr = applyLinuxHotspot(operationCtx, req)
	}
	if applyErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": applyErr.Error(), "notified_modules": notified})
		return
	}

	if req.Action == "start" || req.Action == "update" {
		stored.SSID = req.SSID
		stored.Password = req.Password
		stored.Interface = req.Interface
		if req.BootEnabled != nil {
			stored.BootEnabled = *req.BootEnabled
		}
		if err := saveHotspotFileConfig(stored); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hotspot was applied but hotspot.json could not be saved: " + err.Error()})
			return
		}
	} else if req.FallbackPassword != "" {
		_ = persistEnvSetting(".env", "ELECTRICREDIT_WIFI_PASSWORD", req.FallbackPassword)
		_ = os.Setenv("ELECTRICREDIT_WIFI_PASSWORD", req.FallbackPassword)
	}

	api.logAction(user.ID, "network.hotspot."+req.Action, "network", 0, fmt.Sprintf("Hotspot %s; notified %d active module connection(s)", req.Action, notified))
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "notified_modules": notified, "hotspot": api.networkManagementSnapshot(r.Context(), false)["hotspot"]})
}

func runManagedNMCLI(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "nmcli", args...).CombinedOutput()
	if err == nil || os.Geteuid() == 0 {
		return out, err
	}
	message := strings.ToLower(string(out) + " " + err.Error())
	if !strings.Contains(message, "not authorized") && !strings.Contains(message, "permission") && !strings.Contains(message, "polkit") {
		return out, err
	}
	if _, sudoErr := exec.LookPath("sudo"); sudoErr != nil {
		return out, err
	}
	return exec.CommandContext(ctx, "sudo", append([]string{"-n", "nmcli"}, args...)...).CombinedOutput()
}

func scanWiFiNetworksManaged(ctx context.Context, iface string) ([]map[string]any, error) {
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("nmcli"); err != nil {
			return nil, errors.New("NetworkManager/nmcli is not available")
		}
		args := []string{"--wait", "12", "-t", "-f", "SSID,SIGNAL,SECURITY", "device", "wifi", "list", "--rescan", "yes"}
		if strings.TrimSpace(iface) != "" {
			args = append(args, "ifname", strings.TrimSpace(iface))
		}
		out, err := runManagedNMCLI(ctx, args...)
		if err != nil {
			message := strings.TrimSpace(string(out))
			if message == "" {
				message = err.Error()
			}
			return nil, errors.New(message)
		}
		return parseNMCLIWiFi(string(out)), nil
	}
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("netsh"); err != nil {
			return nil, errors.New("Windows wireless controls require netsh")
		}
		out, err := exec.CommandContext(ctx, "netsh", "wlan", "show", "networks", "mode=bssid").CombinedOutput()
		if err != nil {
			message := strings.TrimSpace(string(out))
			if message == "" {
				message = err.Error()
			}
			return nil, errors.New(message)
		}
		return parseNetshWiFi(string(out)), nil
	}
	return nil, errors.New("Wi-Fi scanning is supported only on Linux and Windows hosts")
}

func findScannedWiFi(networks []map[string]any, ssid string) (map[string]any, bool) {
	for _, network := range networks {
		if fmt.Sprint(network["ssid"]) == ssid {
			return network, true
		}
	}
	return nil, false
}

func scannedWiFiRequiresPassword(network map[string]any) bool {
	security := strings.ToLower(strings.TrimSpace(fmt.Sprint(network["security"])))
	return security != "" && security != "--" && security != "none" && security != "open" && security != "open / unknown security"
}

func classifyWiFiConnectFailure(output []byte, err error) (string, string) {
	message := strings.TrimSpace(string(output))
	if message == "" && err != nil {
		message = err.Error()
	}
	lower := strings.ToLower(message)
	for _, marker := range []string{
		"secrets were required", "no secrets", "invalid secret", "invalid password", "wrong password",
		"key-mgmt", "pre-shared key", "wpa_supplicant", "supplicant failed", "authentication failed",
	} {
		if strings.Contains(lower, marker) {
			return "password_incorrect", "SSID found, but the Wi-Fi password is incorrect."
		}
	}
	if strings.Contains(lower, "no network with ssid") || strings.Contains(lower, "network could not be found") || strings.Contains(lower, "not found") {
		return "ssid_not_found", "SSID cannot be found. Check the Wi-Fi name and try again."
	}
	if message == "" {
		message = "The Wi-Fi connection could not be completed."
	}
	return "connection_failed", "SSID was found, but the connection failed: " + message
}

func savedHotspotSnapshot() hotspotSnapshot {
	cfg, err := loadHotspotFileConfig()
	if err != nil {
		cfg = defaultHotspotFileConfig()
	}
	return hotspotSnapshot{
		Enabled:   true,
		SSID:      cfg.SSID,
		Password:  cfg.Password,
		Interface: cfg.Interface,
	}
}

func restorePreviousHotspot(h hotspotSnapshot) error {
	if !h.Enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	req := hotspotControlRequest{Action: "start", SSID: h.SSID, Password: h.Password, Interface: h.Interface}
	if runtime.GOOS == "windows" {
		return applyWindowsHotspot(ctx, req)
	}
	return applyLinuxHotspot(ctx, req)
}

func stopPreviousHotspot(h hotspotSnapshot) error {
	if !h.Enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req := hotspotControlRequest{Action: "stop", SSID: h.SSID, Password: h.Password, Interface: h.Interface}
	if runtime.GOOS == "windows" {
		return applyWindowsHotspot(ctx, req)
	}
	return applyLinuxHotspot(ctx, req)
}

func (api *API) handleDeveloperWiFiScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "Wi-Fi scanning is supported only on Linux and Windows hosts."})
		return
	}
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("nmcli"); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "NetworkManager is not available on this Linux host."})
			return
		}
	} else if _, err := exec.LookPath("netsh"); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Windows wireless controls are unavailable because netsh was not found."})
		return
	}

	hotspotBefore := detectHotspotSnapshot(r.Context())
	state, started := beginWiFiScan(hotspotBefore.Enabled)
	if !started {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "Another Wi-Fi operation is already running.", "wifi_scan": state, "wifi_connect": currentWiFiConnectState()})
		return
	}
	iface := hotspotBefore.Interface
	if iface == "" {
		stored, _ := loadHotspotFileConfig()
		iface = stored.Interface
	}
	go api.runDeveloperWiFiScan(user.ID, hotspotBefore, iface)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"started":              true,
		"hotspot_will_restart": hotspotBefore.Enabled,
		"message":              "Wi-Fi scan started. The local hotspot may disappear briefly and will be restored automatically.",
	})
}

func (api *API) handleDeveloperWiFiConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireDeveloper(w, r)
	if !ok {
		return
	}
	var req wifiConnectRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid wireless request"})
		return
	}
	req.SSID = strings.TrimSpace(req.SSID)
	if req.SSID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter a Wi-Fi SSID"})
		return
	}
	if len([]byte(req.SSID)) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Wi-Fi SSID must contain at most 32 bytes"})
		return
	}
	if req.Password != "" && (len(req.Password) < 8 || len(req.Password) > 63) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Wi-Fi password must contain 8 to 63 characters"})
		return
	}
	if !api.requireManagementActionChallenge(w, r, user.ID, "network-wifi-connect") {
		return
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "Wireless connection control is supported on Windows and Linux hosts."})
		return
	}
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("nmcli"); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "NetworkManager is not available on this Linux host."})
			return
		}
	} else if _, err := exec.LookPath("netsh"); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Windows wireless controls are unavailable because netsh was not found."})
		return
	}

	hotspotBefore := detectHotspotSnapshot(r.Context())
	state, started := beginWiFiConnect(req.SSID, hotspotBefore.Enabled)
	if !started {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "Another Wi-Fi operation is already running.", "wifi_connect": state, "wifi_scan": currentWiFiScanState()})
		return
	}
	iface := hotspotBefore.Interface
	if iface == "" {
		stored, _ := loadHotspotFileConfig()
		iface = stored.Interface
	}
	serverURL := preferredLocalServerURL(r)
	notified := api.broadcastModuleNetworkUpdate("wifi", req.SSID, req.Password, serverURL, 1800)
	go api.runDeveloperWiFiConnect(user.ID, req, hotspotBefore, iface, notified)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"started":           true,
		"ssid":              req.SSID,
		"hotspot_will_stop": hotspotBefore.Enabled,
		"notified_modules":  notified,
		"message":           "Wi-Fi handover started. If it fails, the ElectriCredit hotspot will be restored automatically.",
	})
}

func (api *API) runDeveloperWiFiConnect(userID int64, req wifiConnectRequest, hotspotBefore hotspotSnapshot, iface string, notified int) {
	// Give the HTTP 202 response enough time to reach a browser connected through
	// the hotspot before wlan0 is intentionally reconfigured.
	time.Sleep(750 * time.Millisecond)

	fallbackHotspot := hotspotBefore
	if !fallbackHotspot.Enabled {
		fallbackHotspot = savedHotspotSnapshot()
	}
	hotspotRestored := false
	fail := func(code, message string) {
		if err := restorePreviousHotspot(fallbackHotspot); err != nil {
			hotspotRestored = false
			message += " The Wi-Fi handover failed and the ElectriCredit hotspot also could not be restored: " + err.Error()
			code = "hotspot_restore_failed"
		} else {
			hotspotRestored = true
			message += " ElectriCredit returned to local hotspot mode."
		}
		finishWiFiConnect(req.SSID, code, message, hotspotBefore.Enabled, hotspotRestored, false)
		api.logAction(userID, "network.wifi.connect", "network", 0, message)
	}

	if notified > 0 {
		time.Sleep(500 * time.Millisecond)
	}
	if hotspotBefore.Enabled {
		if err := stopPreviousHotspot(hotspotBefore); err != nil {
			finishWiFiConnect(req.SSID, "hotspot_stop_failed", "Unable to release the local hotspot before Wi-Fi handover: "+err.Error(), true, false, false)
			return
		}
		time.Sleep(650 * time.Millisecond)
	}

	scanCtx, scanCancel := context.WithTimeout(context.Background(), 18*time.Second)
	networks, scanErr := scanWiFiNetworksManaged(scanCtx, iface)
	scanCancel()
	if scanErr != nil {
		fail("scan_failed", "Unable to verify the requested SSID: "+scanErr.Error()+".")
		return
	}
	target, found := findScannedWiFi(networks, req.SSID)
	if !found {
		fail("ssid_not_found", "SSID cannot be found. Check the Wi-Fi name and try again.")
		return
	}
	if scannedWiFiRequiresPassword(target) && req.Password == "" {
		fail("password_required", "SSID found, but this Wi-Fi requires a password.")
		return
	}

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 30*time.Second)
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = connectWindowsWiFi(connectCtx, req.SSID, req.Password)
	} else {
		args := []string{"--wait", "22", "device", "wifi", "connect", req.SSID}
		if req.Password != "" {
			args = append(args, "password", req.Password)
		}
		if strings.TrimSpace(iface) != "" {
			args = append(args, "ifname", strings.TrimSpace(iface))
		}
		args = append(args, "name", "ElectriCredit Uplink")
		out, err = runManagedNMCLI(connectCtx, args...)
	}
	connectCancel()
	if err != nil {
		code, message := classifyWiFiConnectFailure(out, err)
		fail(code, message)
		return
	}

	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	time.Sleep(700 * time.Millisecond)
	connectedSSID := connectedWiFiSSID(verifyCtx)
	verifyCancel()
	if connectedSSID != req.SSID {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = "NetworkManager did not report the requested SSID as active."
		}
		fail("connection_failed", "SSID was found, but the connection could not be verified: "+message)
		return
	}

	if req.Password != "" {
		_ = persistEnvSetting(".env", "ELECTRICREDIT_WIFI_PASSWORD", req.Password)
		_ = os.Setenv("ELECTRICREDIT_WIFI_PASSWORD", req.Password)
	}
	message := "Connected to " + req.SSID + ". The ElectriCredit hotspot remains off while Wi-Fi mode is active."
	finishWiFiConnect(req.SSID, "", message, hotspotBefore.Enabled, false, true)
	api.logAction(userID, "network.wifi.connect", "network", 0, fmt.Sprintf("%s Notified %d active module connection(s).", message, notified))
}

func (api *API) runDeveloperWiFiScan(userID int64, hotspotBefore hotspotSnapshot, iface string) {
	// Let the accepted response flush before temporarily taking down the AP that
	// may be carrying the Management browser connection.
	time.Sleep(750 * time.Millisecond)

	hotspotRestored := !hotspotBefore.Enabled
	oldNetworks := currentWiFiScanState().Networks
	if hotspotBefore.Enabled {
		if err := stopPreviousHotspot(hotspotBefore); err != nil {
			finishWiFiScan(oldNetworks, "hotspot_stop_failed", "Unable to pause the local hotspot for Wi-Fi scanning: "+err.Error(), true, false)
			return
		}
		time.Sleep(650 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	networks, scanErr := scanWiFiNetworksManaged(ctx, iface)
	cancel()

	var restoreErr error
	if hotspotBefore.Enabled {
		restoreErr = restorePreviousHotspot(hotspotBefore)
		hotspotRestored = restoreErr == nil
	}
	if restoreErr != nil {
		message := "Wi-Fi scan finished, but the ElectriCredit hotspot could not be restored: " + restoreErr.Error()
		finishWiFiScan(networks, "hotspot_restore_failed", message, hotspotBefore.Enabled, false)
		api.logAction(userID, "network.wifi.scan", "network", 0, message)
		return
	}
	if scanErr != nil {
		message := "Wi-Fi scan failed: " + scanErr.Error()
		if hotspotBefore.Enabled {
			message += " The ElectriCredit hotspot was restored."
		}
		finishWiFiScan(oldNetworks, "scan_failed", message, hotspotBefore.Enabled, hotspotRestored)
		api.logAction(userID, "network.wifi.scan", "network", 0, message)
		return
	}

	message := fmt.Sprintf("Wi-Fi scan completed. %d network(s) found.", len(networks))
	if hotspotBefore.Enabled {
		message += " The ElectriCredit hotspot was restored."
	}
	finishWiFiScan(networks, "", message, hotspotBefore.Enabled, hotspotRestored)
	api.logAction(userID, "network.wifi.scan", "network", 0, message)
}

func applyLinuxHotspot(ctx context.Context, req hotspotControlRequest) error {
	cfg, _ := loadHotspotFileConfig()
	if req.SSID == "" {
		req.SSID = cfg.SSID
	}
	if req.Password == "" {
		req.Password = cfg.Password
	}
	if req.Interface == "" {
		req.Interface = cfg.Interface
	}
	connectionName := cfg.ConnectionName
	if connectionName == "" {
		connectionName = defaultHotspotConnectionName
	}

	// Raspberry Pi OS Bookworm uses NetworkManager by default. Prefer a named
	// NetworkManager profile so boot scripts and the web UI control the exact
	// same hotspot instead of maintaining two competing configurations.
	if _, err := exec.LookPath("nmcli"); err == nil {
		if req.Action == "stop" {
			c, cancel := context.WithTimeout(ctx, 12*time.Second)
			defer cancel()
			out, err := runManagedNMCLI(c, "connection", "down", connectionName)
			if err != nil {
				msg := strings.ToLower(string(out))
				if !strings.Contains(msg, "not active") && !strings.Contains(msg, "unknown connection") {
					return fmt.Errorf("unable to stop hotspot: %s", strings.TrimSpace(string(out)))
				}
			}
			return nil
		}

		c, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		_, _ = runManagedNMCLI(c, "radio", "wifi", "on")

		profileExists := exec.CommandContext(c, "nmcli", "-t", "-f", "NAME", "connection", "show", connectionName).Run() == nil
		if !profileExists {
			out, err := runManagedNMCLI(c, "connection", "add", "type", "wifi", "ifname", req.Interface, "con-name", connectionName, "ssid", req.SSID)
			if err != nil {
				return fmt.Errorf("unable to create hotspot profile: %s", strings.TrimSpace(string(out)))
			}
		}

		args := []string{"connection", "modify", connectionName,
			"connection.interface-name", req.Interface,
			"connection.autoconnect", "no",
			"802-11-wireless.ssid", req.SSID,
			"802-11-wireless.mode", "ap",
			"802-11-wireless.band", "bg",
			"802-11-wireless.channel", "6",
			"wifi-sec.key-mgmt", "wpa-psk",
			"wifi-sec.proto", "rsn",
			"wifi-sec.pairwise", "ccmp",
			"wifi-sec.group", "ccmp",
			"wifi-sec.pmf", "disable",
			"wifi-sec.psk", req.Password,
			"ipv4.method", "shared",
			"ipv4.addresses", cfg.IPv4Address,
			"ipv6.method", "disabled"}
		if out, err := runManagedNMCLI(c, args...); err != nil {
			return fmt.Errorf("unable to configure hotspot: %s", strings.TrimSpace(string(out)))
		}
		out, err := runManagedNMCLI(c, "connection", "up", connectionName)
		if err != nil {
			return fmt.Errorf("unable to start hotspot: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}

	// Older Raspberry Pi installations may still use hostapd. Keep this as a
	// fallback, but NetworkManager is the primary supported path.
	configPath := strings.TrimSpace(os.Getenv("ELECTRICREDIT_HOSTAPD_CONFIG"))
	if configPath == "" {
		configPath = "/etc/hostapd/hostapd.conf"
	}
	if _, err := os.Stat(configPath); err != nil {
		return errors.New("no supported hotspot manager was found (NetworkManager/nmcli is recommended)")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return errors.New("systemctl is required for the hostapd hotspot fallback")
	}
	if req.Action == "stop" {
		out, err := exec.CommandContext(ctx, "systemctl", "stop", "hostapd").CombinedOutput()
		if err != nil {
			return fmt.Errorf("unable to stop hotspot: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := updateHostapdConfig(configPath, req.SSID, req.Password, req.Interface); err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "systemctl", "restart", "hostapd").CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to restart hotspot: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func connectWindowsWiFi(ctx context.Context, ssid, password string) ([]byte, error) {
	if _, err := exec.LookPath("netsh"); err != nil {
		return nil, errors.New("Windows wireless controls require netsh")
	}
	// Prefer an existing Windows Wi-Fi profile first.
	out, err := exec.CommandContext(ctx, "netsh", "wlan", "connect", "name="+ssid, "ssid="+ssid).CombinedOutput()
	if err == nil && !strings.Contains(strings.ToLower(string(out)), "not found") {
		return out, nil
	}

	// If there is no saved profile, create a short-lived XML profile. This is
	// suitable for normal WPA2-Personal and open networks used during laptop testing.
	auth := "open"
	encryption := "none"
	shared := ""
	if password != "" {
		if len(password) < 8 || len(password) > 63 {
			return out, errors.New("Wi-Fi password must contain 8 to 63 characters")
		}
		auth = "WPA2PSK"
		encryption = "AES"
		shared = "<sharedKey><keyType>passPhrase</keyType><protected>false</protected><keyMaterial>" + xmlTextEscape(password) + "</keyMaterial></sharedKey>"
	}
	profile := `<?xml version="1.0"?><WLANProfile xmlns="http://www.microsoft.com/networking/WLAN/profile/v1"><name>` + xmlTextEscape(ssid) + `</name><SSIDConfig><SSID><name>` + xmlTextEscape(ssid) + `</name></SSID></SSIDConfig><connectionType>ESS</connectionType><connectionMode>auto</connectionMode><MSM><security><authEncryption><authentication>` + auth + `</authentication><encryption>` + encryption + `</encryption><useOneX>false</useOneX></authEncryption>` + shared + `</security></MSM></WLANProfile>`
	f, ferr := os.CreateTemp("", "electricredit-wifi-*.xml")
	if ferr != nil {
		return out, ferr
	}
	path := f.Name()
	defer os.Remove(path)
	if _, ferr = f.WriteString(profile); ferr != nil {
		_ = f.Close()
		return out, ferr
	}
	_ = f.Close()
	if addOut, addErr := exec.CommandContext(ctx, "netsh", "wlan", "add", "profile", "filename="+path, "user=current").CombinedOutput(); addErr != nil {
		return addOut, fmt.Errorf("unable to add Wi-Fi profile: %s", strings.TrimSpace(string(addOut)))
	}
	return exec.CommandContext(ctx, "netsh", "wlan", "connect", "name="+ssid, "ssid="+ssid).CombinedOutput()
}

func xmlTextEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return replacer.Replace(value)
}

func applyWindowsHotspot(ctx context.Context, req hotspotControlRequest) error {
	if _, err := exec.LookPath("netsh"); err != nil {
		return errors.New("Windows hotspot control requires netsh")
	}
	if req.Action == "stop" {
		out, err := exec.CommandContext(ctx, "netsh", "wlan", "stop", "hostednetwork").CombinedOutput()
		if err != nil {
			return fmt.Errorf("unable to stop Windows hotspot: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	out, err := exec.CommandContext(ctx, "netsh", "wlan", "set", "hostednetwork", "mode=allow", "ssid="+req.SSID, "key="+req.Password).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to configure Windows hotspot: %s", strings.TrimSpace(string(out)))
	}
	out, err = exec.CommandContext(ctx, "netsh", "wlan", "start", "hostednetwork").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("unable to start Windows hotspot (the Wi-Fi adapter may not support Hosted Network mode): %s", msg)
	}
	return nil
}

func updateHostapdConfig(path, ssid, password, iface string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("unable to read hostapd configuration: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	values := map[string]string{
		"ssid":           ssid,
		"wpa_passphrase": password,
		"hw_mode":        "g",
		"channel":        "6",
		"wpa":            "2",
		"wpa_key_mgmt":   "WPA-PSK",
		"rsn_pairwise":   "CCMP",
		"ieee80211w":     "0",
	}
	if iface != "" {
		values["interface"] = iface
	}
	found := make(map[string]bool, len(values))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if value, exists := values[key]; exists {
			lines[i] = key + "=" + value
			found[key] = true
		}
	}
	for _, key := range []string{"interface", "ssid", "hw_mode", "channel", "wpa", "wpa_key_mgmt", "rsn_pairwise", "ieee80211w", "wpa_passphrase"} {
		value, exists := values[key]
		if exists && !found[key] {
			lines = append(lines, key+"="+value)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return fmt.Errorf("unable to update hostapd configuration (run ElectriCredit with sufficient permission): %w", err)
	}
	return nil
}

func persistEnvSetting(path, key, value string) error {
	input, err := os.Open(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := []string{}
	found := false
	if input != nil {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, "export "+key+"=") {
				lines = append(lines, key+"="+value)
				found = true
			} else {
				lines = append(lines, line)
			}
		}
		_ = input.Close()
		if err := scanner.Err(); err != nil {
			return err
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
