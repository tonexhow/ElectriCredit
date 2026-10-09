package internal

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"electricredit/database"
)

type ChatHub struct {
	mu        sync.RWMutex
	peers     map[string]*chatPeer
	listeners map[string]map[chan chatEvent]struct{}
	bans      map[string]time.Time
}

type chatPeer struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	DeviceID  string `json:"device_id"`
	Name      string `json:"name"`
	Role      string `json:"role,omitempty"`
	UserID    int64  `json:"-"`
	Token     string `json:"-"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	Platform  string `json:"platform,omitempty"`
	Browser   string `json:"browser,omitempty"`
	OS        string `json:"os,omitempty"`
	Device    string `json:"device,omitempty"`
	Screen    string `json:"screen,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
	ImageURL  string `json:"image_url,omitempty"`
	JoinedAt  int64  `json:"joined_at"`
	LastSeen  int64  `json:"last_seen"`
	Started   bool   `json:"started,omitempty"`
}

type chatEvent struct {
	Type string          `json:"type"`
	From string          `json:"from,omitempty"`
	To   string          `json:"to,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
	Peer *chatPeer       `json:"peer,omitempty"`
}

type chatRegisterRequest struct {
	PeerID    string `json:"peer_id"`
	DeviceID  string `json:"device_id"`
	Name      string `json:"name"`
	UserAgent string `json:"user_agent"`
	Platform  string `json:"platform"`
	Browser   string `json:"browser"`
	OS        string `json:"os"`
	Device    string `json:"device"`
	Screen    string `json:"screen"`
	Timezone  string `json:"timezone"`
}

type chatSignalRequest struct {
	From  string          `json:"from"`
	To    string          `json:"to"`
	Token string          `json:"token"`
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data"`
}

type chatControlRequest struct {
	Action string `json:"action"`
	Target string `json:"target"`
}

func newChatHub() *ChatHub {
	return &ChatHub{
		peers:     map[string]*chatPeer{},
		listeners: map[string]map[chan chatEvent]struct{}{},
		bans:      map[string]time.Time{},
	}
}

func (api *API) registerChatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/chat/register", api.handleChatRegister)
	mux.HandleFunc("/api/chat/ws", api.handleChatWebSocket)
	// Legacy HTTP/SSE chat routes remain available for older clients.
	mux.HandleFunc("/api/chat/peers", api.handleChatPeers)
	mux.HandleFunc("/api/chat/events", api.handleChatEvents)
	mux.HandleFunc("/api/chat/signal", api.handleChatSignal)
	mux.HandleFunc("/api/chat/control", api.handleChatControl)
}

func (api *API) handleChatRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var request chatRegisterRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat registration"})
		return
	}
	request.PeerID = sanitizePeerID(request.PeerID)
	request.DeviceID = sanitizeDeviceID(request.DeviceID)
	if request.PeerID == "" || request.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "chat peer and device identifiers are required"})
		return
	}

	api.Chat.mu.Lock()
	if until, banned := api.Chat.bans[request.DeviceID]; banned {
		if until.IsZero() || until.After(time.Now()) {
			api.Chat.mu.Unlock()
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "this device is blocked from ElectriCredit chat"})
			return
		}
		delete(api.Chat.bans, request.DeviceID)
	}
	api.Chat.mu.Unlock()

	user, _, authenticated := api.authenticateRequest(r)
	peer := &chatPeer{
		ID:        request.PeerID,
		DeviceID:  request.DeviceID,
		IP:        requestIP(r),
		UserAgent: trimLimit(request.UserAgent, 512),
		Platform:  trimLimit(request.Platform, 120),
		Browser:   trimLimit(request.Browser, 120),
		OS:        trimLimit(request.OS, 120),
		Device:    trimLimit(request.Device, 120),
		Screen:    trimLimit(request.Screen, 64),
		Timezone:  trimLimit(request.Timezone, 80),
		JoinedAt:  time.Now().UTC().Unix(),
		LastSeen:  time.Now().UTC().Unix(),
	}

	if authenticated {
		peer.Kind = "staff"
		peer.UserID = user.ID
		peer.Role = strings.ToLower(user.Role)
		peer.Name = strings.TrimSpace(user.FirstName + " " + user.LastName)
		var image, imageMime string
		var imageBlob []byte
		if err := api.DB.QueryRow(`SELECT COALESCE(profile_image,''),COALESCE(profile_image_blob,X''),COALESCE(profile_image_mime,'') FROM superusers WHERE id=?;`, user.ID).Scan(&image, &imageBlob, &imageMime); err == nil {
			peer.ImageURL = userProfileImageURL(image, imageBlob, imageMime)
		}
	} else {
		peer.Kind = "guest"
		peer.Name = strings.TrimSpace(request.Name)
		if peer.Name == "" {
			peer.Name = "Visitor"
		}

		// Reuse the anonymous token when the same browser races a second
		// registration for the same peer. This prevents one request from
		// invalidating another while a WebSocket is being established.
		api.Chat.mu.RLock()
		existing := api.Chat.peers[peer.ID]
		api.Chat.mu.RUnlock()
		if existing != nil && existing.Kind == "guest" && existing.DeviceID == peer.DeviceID && existing.Token != "" {
			peer.Token = existing.Token
			peer.JoinedAt = existing.JoinedAt
			peer.Started = existing.Started
		} else {
			token, err := newChatToken()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to start chat"})
				return
			}
			peer.Token = token
		}
	}

	api.Chat.mu.Lock()
	api.Chat.peers[peer.ID] = peer
	api.Chat.mu.Unlock()
	api.Chat.broadcast(chatEvent{Type: "peer-joined", Peer: cloneChatPeer(peer)}, peer.ID)

	response := map[string]any{
		"peer":  publicChatPeer(peer, authenticated),
		"peers": api.chatPeerList(peer, authenticated),
	}
	if peer.Kind == "guest" {
		response["token"] = peer.Token
	}
	writeJSON(w, http.StatusCreated, response)
}

func (api *API) handleChatPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	peer, authenticated, ok := api.authorizeChatPeer(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "chat session is not authorized"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"peers": api.chatPeerList(peer, authenticated)})
}

func (api *API) handleChatEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	peer, _, ok := api.authorizeChatPeer(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "chat session is not authorized"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming is unavailable"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	channel := make(chan chatEvent, 32)
	api.Chat.subscribe(peer.ID, channel)
	defer api.Chat.unsubscribe(peer.ID, channel)

	fmt.Fprintf(w, "event: ready\ndata: {\"ok\":true}\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	lastSessionTouch := time.Time{}

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-channel:
			payload, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: chat\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-keepAlive.C:
			api.Chat.touch(peer.ID)
			if peer.Kind == "staff" && (lastSessionTouch.IsZero() || time.Since(lastSessionTouch) >= time.Minute) {
				if _, sessionID, authenticated := api.authenticateRequest(r); authenticated {
					_, _ = api.DB.Exec(`UPDATE login_sessions SET last_seen_at=? WHERE id=?;`, database.UnixTime(), sessionID)
				}
				lastSessionTouch = time.Now()
			}
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (api *API) handleChatSignal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request chatSignalRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat signal"})
		return
	}
	request.From = sanitizePeerID(request.From)
	request.To = sanitizePeerID(request.To)
	request.Type = strings.ToLower(strings.TrimSpace(request.Type))
	if request.From == "" || request.To == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "chat sender and recipient are required"})
		return
	}
	if !allowedChatSignal(request.Type) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported chat signal"})
		return
	}

	peer, _, ok := api.authorizeChatPeerWithValues(r, request.From, request.Token)
	if !ok || peer.ID != request.From {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "chat sender is not authorized"})
		return
	}

	api.Chat.mu.RLock()
	target := api.Chat.peers[request.To]
	api.Chat.mu.RUnlock()
	if target == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "chat recipient is no longer available"})
		return
	}
	if peer.Kind == target.Kind {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "private staff-to-staff and visitor-to-visitor chat is not available"})
		return
	}

	if peer.Kind == "guest" && target.Kind == "staff" && request.Type == "offer" {
		api.Chat.mu.Lock()
		if live := api.Chat.peers[peer.ID]; live != nil && !live.Started {
			live.Started = true
			peer.Started = true
		}
		api.Chat.mu.Unlock()
		api.Chat.broadcastStaff(chatEvent{Type: "peer-started", Peer: cloneChatPeer(peer)}, "")
	}

	api.Chat.send(request.To, chatEvent{Type: request.Type, From: request.From, To: request.To, Data: request.Data})
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) handleChatControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := api.requireRoles(w, r, "admin", "owner", "developer")
	if !ok {
		return
	}
	var request chatControlRequest
	if err := readJSON(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat control request"})
		return
	}
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.Target = sanitizePeerID(request.Target)
	if request.Target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "chat target is required"})
		return
	}

	api.Chat.mu.Lock()
	target := api.Chat.peers[request.Target]
	if target == nil {
		api.Chat.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "chat device is no longer connected"})
		return
	}
	if target.Kind == "staff" {
		api.Chat.mu.Unlock()
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "staff accounts cannot be cleared, terminated, or banned from support chat"})
		return
	}

	switch request.Action {
	case "terminate":
		delete(api.Chat.peers, request.Target)
	case "ban":
		api.Chat.bans[target.DeviceID] = time.Time{} // until server restart
		delete(api.Chat.peers, request.Target)
	case "clear":
		// Peer remains connected; client receives a clear-cache command.
	default:
		api.Chat.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "chat action must be terminate, clear, or ban"})
		return
	}
	api.Chat.mu.Unlock()

	api.Chat.send(request.Target, chatEvent{Type: "control", Data: mustJSON(map[string]string{"action": request.Action})})
	if request.Action != "clear" {
		api.Chat.broadcast(chatEvent{Type: "peer-left", Peer: cloneChatPeer(target)}, request.Target)
	}
	api.logAction(user.ID, "chat."+request.Action, "chat_device", 0, fmt.Sprintf("%s chat device %s", request.Action, target.DeviceID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (api *API) authorizeChatPeer(r *http.Request) (*chatPeer, bool, bool) {
	return api.authorizeChatPeerWithValues(r, r.URL.Query().Get("peer_id"), r.URL.Query().Get("token"))
}

func (api *API) authorizeChatPeerWithValues(r *http.Request, peerID, token string) (*chatPeer, bool, bool) {
	peerID = sanitizePeerID(peerID)
	api.Chat.mu.RLock()
	peer := api.Chat.peers[peerID]
	if peer == nil {
		api.Chat.mu.RUnlock()
		return nil, false, false
	}
	expectedToken := peer.Token
	copyPeer := cloneChatPeer(peer)
	api.Chat.mu.RUnlock()

	if copyPeer.Kind == "staff" {
		user, _, ok := api.authenticateRequest(r)
		return copyPeer, true, ok && user.ID == copyPeer.UserID
	}
	return copyPeer, false, token != "" && expectedToken != "" && token == expectedToken
}

func (hub *ChatHub) subscribe(peerID string, channel chan chatEvent) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.listeners[peerID] == nil {
		hub.listeners[peerID] = map[chan chatEvent]struct{}{}
	}
	hub.listeners[peerID][channel] = struct{}{}
}

func (hub *ChatHub) unsubscribe(peerID string, channel chan chatEvent) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	delete(hub.listeners[peerID], channel)
	if len(hub.listeners[peerID]) == 0 {
		delete(hub.listeners, peerID)
	}
}

func (hub *ChatHub) send(peerID string, event chatEvent) {
	hub.mu.RLock()
	listeners := make([]chan chatEvent, 0, len(hub.listeners[peerID]))
	for channel := range hub.listeners[peerID] {
		listeners = append(listeners, channel)
	}
	hub.mu.RUnlock()
	for _, channel := range listeners {
		select {
		case channel <- event:
		default:
		}
	}
}

func (hub *ChatHub) broadcast(event chatEvent, exclude string) {
	hub.mu.RLock()
	ids := make([]string, 0, len(hub.listeners))
	for id := range hub.listeners {
		if id != exclude {
			ids = append(ids, id)
		}
	}
	hub.mu.RUnlock()
	for _, id := range ids {
		hub.send(id, event)
	}
}

func (hub *ChatHub) broadcastStaff(event chatEvent, exclude string) {
	hub.mu.RLock()
	ids := make([]string, 0, len(hub.peers))
	for id, peer := range hub.peers {
		if id != exclude && peer != nil && peer.Kind == "staff" {
			ids = append(ids, id)
		}
	}
	hub.mu.RUnlock()
	for _, id := range ids {
		hub.send(id, event)
	}
}

func (hub *ChatHub) activeStaffCount() int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	users := map[int64]struct{}{}
	cutoff := time.Now().UTC().Add(-90 * time.Second).Unix()
	for _, peer := range hub.peers {
		if peer != nil && peer.Kind == "staff" && peer.UserID > 0 && peer.LastSeen >= cutoff {
			users[peer.UserID] = struct{}{}
		}
	}
	return len(users)
}

func (hub *ChatHub) touch(peerID string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if peer := hub.peers[peerID]; peer != nil {
		peer.LastSeen = time.Now().UTC().Unix()
	}
}

func publicChatPeer(peer *chatPeer, staffViewer bool) map[string]any {
	result := map[string]any{
		"id":        peer.ID,
		"kind":      peer.Kind,
		"device_id": peer.DeviceID,
		"name":      peer.Name,
		"role":      peer.Role,
		"image_url": peer.ImageURL,
		"joined_at": peer.JoinedAt,
		"last_seen": peer.LastSeen,
		"started":   peer.Started,
	}
	if staffViewer {
		result["ip"] = peer.IP
		result["user_agent"] = peer.UserAgent
		result["platform"] = peer.Platform
		result["browser"] = peer.Browser
		result["os"] = peer.OS
		result["device"] = peer.Device
		result["screen"] = peer.Screen
		result["timezone"] = peer.Timezone
	}
	return result
}

func cloneChatPeer(peer *chatPeer) *chatPeer {
	if peer == nil {
		return nil
	}
	copyPeer := *peer
	copyPeer.Token = ""
	copyPeer.IP = ""
	copyPeer.UserAgent = ""
	copyPeer.Platform = ""
	copyPeer.Browser = ""
	copyPeer.OS = ""
	copyPeer.Device = ""
	copyPeer.Screen = ""
	copyPeer.Timezone = ""
	return &copyPeer
}

func requestIP(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); value != "" {
		return value
	}
	if value := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); value != "" {
		return strings.TrimSpace(strings.Split(value, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func sanitizePeerID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 96 {
		value = value[:96]
	}
	for _, r := range value {
		if !(r == '-' || r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return ""
		}
	}
	return value
}

func sanitizeDeviceID(value string) string {
	return sanitizePeerID(value)
}

func trimLimit(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return value[:max]
	}
	return value
}

func allowedChatSignal(value string) bool {
	switch value {
	case "offer", "answer", "ice", "hangup", "call-audio", "call-video", "typing", "ping":
		return true
	default:
		return false
	}
}

func newChatToken() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func mustJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func validateChatHub(hub *ChatHub) error {
	if hub == nil {
		return errors.New("chat hub is nil")
	}
	return nil
}
