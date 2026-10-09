package internal

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// handleDeveloperSystemWebSocket streams live server telemetry while the
// Management System page is open. WebSocket is used here because telemetry is
// server-to-browser state, while WebRTC remains reserved for peer-to-peer chat.
func (api *API) handleDeveloperSystemWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "websocket upgrade required"})
		return
	}
	if !chatWebSocketOriginAllowed(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin is not allowed"})
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}

	ws, err := upgradeSimpleWebSocket(w, r)
	if err != nil {
		return
	}
	defer ws.close()

	closed := make(chan struct{}, 1)
	go func() {
		for {
			if _, err := ws.readText(); err != nil {
				if err != io.EOF {
					// Browser/network closes are intentionally quiet.
				}
				select {
				case closed <- struct{}{}:
				default:
				}
				return
			}
		}
	}()

	send := func() bool {
		payload := map[string]any{"type": "system", "data": developerSystemSnapshot()}
		return ws.writeJSON(payload) == nil
	}
	if !send() {
		return
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-closed:
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

func marshalSystemSnapshot(snapshot map[string]any) json.RawMessage {
	raw, _ := json.Marshal(snapshot)
	return raw
}
