package internal

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"electricredit/database"
)

const webSocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type chatWSClientMessage struct {
	Type       string          `json:"type"`
	To         string          `json:"to,omitempty"`
	SignalType string          `json:"signal_type,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

type simpleWebSocket struct {
	conn    net.Conn
	reader  *bufio.Reader
	writer  *bufio.Writer
	writeMu sync.Mutex
	closed  chan struct{}
	once    sync.Once
}

func (api *API) handleChatWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "websocket upgrade required"})
		return
	}
	if !chatWebSocketOriginAllowed(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "chat origin is not allowed"})
		return
	}

	peer, authenticated, ok := api.authorizeChatPeer(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "chat session is not authorized"})
		return
	}

	ws, err := upgradeSimpleWebSocket(w, r)
	if err != nil {
		return
	}
	defer ws.close()

	events := make(chan chatEvent, 64)
	api.Chat.subscribe(peer.ID, events)
	defer func() {
		api.Chat.unsubscribe(peer.ID, events)
		api.removeChatPeerIfDisconnected(peer.ID)
	}()

	readyData, _ := json.Marshal(map[string]any{
		"peers": api.chatPeerList(peer, authenticated),
	})
	if err := ws.writeJSON(chatEvent{Type: "ready", Data: readyData}); err != nil {
		return
	}

	readMessages := make(chan chatWSClientMessage, 16)
	readErr := make(chan error, 1)
	go func() {
		for {
			payload, err := ws.readText()
			if err != nil {
				readErr <- err
				return
			}
			var message chatWSClientMessage
			if err := json.Unmarshal(payload, &message); err != nil {
				continue
			}
			select {
			case readMessages <- message:
			case <-ws.closed:
				return
			}
		}
	}()

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	lastSessionTouch := time.Time{}

	for {
		select {
		case <-r.Context().Done():
			return
		case err := <-readErr:
			if !errors.Is(err, io.EOF) {
				// Normal browser/network closes are intentionally quiet.
			}
			return
		case event := <-events:
			if err := ws.writeJSON(event); err != nil {
				return
			}
		case message := <-readMessages:
			api.Chat.touch(peer.ID)
			if err := api.handleChatWSMessage(peer, authenticated, message); err != nil {
				data, _ := json.Marshal(map[string]string{"message": err.Error()})
				_ = ws.writeJSON(chatEvent{Type: "error", Data: data})
			}
		case <-keepAlive.C:
			api.Chat.touch(peer.ID)
			if peer.Kind == "staff" && (lastSessionTouch.IsZero() || time.Since(lastSessionTouch) >= time.Minute) {
				if _, sessionID, authenticatedRequest := api.authenticateRequest(r); authenticatedRequest {
					_, _ = api.DB.Exec(`UPDATE login_sessions SET last_seen_at=? WHERE id=?;`, database.UnixTime(), sessionID)
				}
				lastSessionTouch = time.Now()
			}
			if err := ws.writePing(); err != nil {
				return
			}
		}
	}
}

func (api *API) handleChatWSMessage(peer *chatPeer, authenticated bool, request chatWSClientMessage) error {
	request.Type = strings.ToLower(strings.TrimSpace(request.Type))
	switch request.Type {
	case "ping":
		return nil
	case "peers":
		data, _ := json.Marshal(map[string]any{"peers": api.chatPeerList(peer, authenticated)})
		api.Chat.send(peer.ID, chatEvent{Type: "peers", Data: data})
		return nil
	case "signal":
		request.To = sanitizePeerID(request.To)
		request.SignalType = strings.ToLower(strings.TrimSpace(request.SignalType))
		if request.To == "" {
			return errors.New("chat recipient is required")
		}
		if !allowedChatSignal(request.SignalType) {
			return errors.New("unsupported chat signal")
		}

		api.Chat.mu.RLock()
		target := api.Chat.peers[request.To]
		api.Chat.mu.RUnlock()
		if target == nil {
			return errors.New("chat recipient is no longer available")
		}
		if peer.Kind == target.Kind {
			return errors.New("private staff-to-staff and visitor-to-visitor chat is not available")
		}

		if peer.Kind == "guest" && target.Kind == "staff" && request.SignalType == "offer" {
			api.Chat.mu.Lock()
			if live := api.Chat.peers[peer.ID]; live != nil && !live.Started {
				live.Started = true
				peer.Started = true
			}
			api.Chat.mu.Unlock()
			api.Chat.broadcastStaff(chatEvent{Type: "peer-started", Peer: cloneChatPeer(peer)}, "")
		}

		api.Chat.send(request.To, chatEvent{
			Type: request.SignalType,
			From: peer.ID,
			To:   request.To,
			Data: request.Data,
		})
		return nil
	default:
		return errors.New("unsupported websocket message")
	}
}

func (api *API) chatPeerList(peer *chatPeer, authenticated bool) []map[string]any {
	api.Chat.mu.RLock()
	defer api.Chat.mu.RUnlock()
	peers := make([]map[string]any, 0, len(api.Chat.peers))
	for _, candidate := range api.Chat.peers {
		if candidate == nil || candidate.ID == peer.ID {
			continue
		}
		if authenticated && (candidate.Kind != "guest" || !candidate.Started) {
			continue
		}
		if !authenticated && candidate.Kind != "staff" {
			continue
		}
		peers = append(peers, publicChatPeer(candidate, authenticated))
	}
	return peers
}

func (api *API) removeChatPeerIfDisconnected(peerID string) {
	peerID = sanitizePeerID(peerID)
	if peerID == "" {
		return
	}
	api.Chat.mu.Lock()
	if len(api.Chat.listeners[peerID]) > 0 {
		api.Chat.mu.Unlock()
		return
	}
	peer := api.Chat.peers[peerID]
	if peer != nil {
		delete(api.Chat.peers, peerID)
	}
	api.Chat.mu.Unlock()
	if peer != nil {
		api.Chat.broadcast(chatEvent{Type: "peer-left", Peer: cloneChatPeer(peer)}, peerID)
	}
}

func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	for _, part := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(part), "upgrade") {
			return r.Header.Get("Sec-WebSocket-Version") == "13" && strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key")) != ""
		}
	}
	return false
}

func chatWebSocketOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	return isAllowedDevelopmentOrigin(origin)
}

func upgradeSimpleWebSocket(w http.ResponseWriter, r *http.Request) (*simpleWebSocket, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket is unavailable", http.StatusInternalServerError)
		return nil, errors.New("http hijacking is unavailable")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	hash := sha1.Sum([]byte(key + webSocketGUID))
	accept := base64.StdEncoding.EncodeToString(hash[:])
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &simpleWebSocket{conn: conn, reader: rw.Reader, writer: rw.Writer, closed: make(chan struct{})}, nil
}

func (ws *simpleWebSocket) close() {
	ws.once.Do(func() {
		close(ws.closed)
		_ = ws.conn.Close()
	})
}

func (ws *simpleWebSocket) readText() ([]byte, error) {
	for {
		first, err := ws.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		second, err := ws.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		fin := first&0x80 != 0
		opcode := first & 0x0f
		masked := second&0x80 != 0
		length := uint64(second & 0x7f)
		switch length {
		case 126:
			var size uint16
			if err := binary.Read(ws.reader, binary.BigEndian, &size); err != nil {
				return nil, err
			}
			length = uint64(size)
		case 127:
			if err := binary.Read(ws.reader, binary.BigEndian, &length); err != nil {
				return nil, err
			}
		}
		if length > 2*1024*1024 {
			return nil, errors.New("websocket frame is too large")
		}
		if !masked {
			return nil, errors.New("client websocket frame must be masked")
		}
		mask := make([]byte, 4)
		if _, err := io.ReadFull(ws.reader, mask); err != nil {
			return nil, err
		}
		payload := make([]byte, int(length))
		if _, err := io.ReadFull(ws.reader, payload); err != nil {
			return nil, err
		}
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
		if !fin {
			return nil, errors.New("fragmented websocket frames are not supported")
		}
		switch opcode {
		case 0x1:
			return payload, nil
		case 0x8:
			_ = ws.writeFrame(0x8, nil)
			return nil, io.EOF
		case 0x9:
			if err := ws.writeFrame(0xA, payload); err != nil {
				return nil, err
			}
		case 0xA:
			continue
		default:
			// Binary and unknown frames are ignored; signaling is JSON text.
		}
	}
}

func (ws *simpleWebSocket) writeJSON(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return ws.writeFrame(0x1, payload)
}

func (ws *simpleWebSocket) writePing() error {
	return ws.writeFrame(0x9, []byte("ec"))
}

func (ws *simpleWebSocket) writeFrame(opcode byte, payload []byte) error {
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()
	if _, err := ws.writer.Write([]byte{0x80 | opcode}); err != nil {
		return err
	}
	length := len(payload)
	switch {
	case length < 126:
		if err := ws.writer.WriteByte(byte(length)); err != nil {
			return err
		}
	case length <= 65535:
		if err := ws.writer.WriteByte(126); err != nil {
			return err
		}
		if err := binary.Write(ws.writer, binary.BigEndian, uint16(length)); err != nil {
			return err
		}
	default:
		if err := ws.writer.WriteByte(127); err != nil {
			return err
		}
		if err := binary.Write(ws.writer, binary.BigEndian, uint64(length)); err != nil {
			return err
		}
	}
	if length > 0 {
		if _, err := ws.writer.Write(payload); err != nil {
			return err
		}
	}
	return ws.writer.Flush()
}
