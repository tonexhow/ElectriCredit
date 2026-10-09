/**
 * ElectriCredit - Global WebSocket Client
 * Handles real-time browser <-> Go backend communication.
 */

const WebSocketClient = (() => {
    let socket = null;
    let reconnectTimer = null;
    let reconnectAttempts = 0;
    let manuallyClosed = false;

    const listeners = new Map();

    const DEFAULT_OPTIONS = {
        url: "ws://127.0.0.1:8080/ws",
        reconnect: true,
        reconnectDelay: 1500,
        maxReconnectDelay: 10000
    };

    let settings = { ...DEFAULT_OPTIONS };

    function connect(options = {}) {
        settings = {
            ...settings,
            ...options
        };

        if (
            socket &&
            (
                socket.readyState === WebSocket.OPEN ||
                socket.readyState === WebSocket.CONNECTING
            )
        ) {
            return socket;
        }

        manuallyClosed = false;

        try {
            socket = new WebSocket(settings.url);
        } catch (error) {
            emit("error", error);
            scheduleReconnect();
            return null;
        }

        socket.addEventListener("open", () => {
            reconnectAttempts = 0;
            updateConnectionStatus("online");
            emit("open");
        });

        socket.addEventListener("message", event => {
            let payload = event.data;

            try {
                payload = JSON.parse(event.data);
            } catch {
                // Keep plain-text messages unchanged.
            }

            emit("message", payload);

            if (payload && typeof payload === "object" && payload.type) {
                emit(payload.type, payload);
            }
        });

        socket.addEventListener("close", event => {
            updateConnectionStatus("offline");
            emit("close", event);

            if (!manuallyClosed) {
                scheduleReconnect();
            }
        });

        socket.addEventListener("error", event => {
            emit("error", event);
        });

        return socket;
    }

    function disconnect() {
        manuallyClosed = true;

        if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }

        if (socket) {
            socket.close();
            socket = null;
        }

        updateConnectionStatus("offline");
    }

    function send(type, data = {}) {
        if (!socket || socket.readyState !== WebSocket.OPEN) {
            return false;
        }

        socket.send(
            JSON.stringify({
                type,
                data
            })
        );

        return true;
    }

    function on(eventName, callback) {
        if (typeof callback !== "function") {
            return () => {};
        }

        if (!listeners.has(eventName)) {
            listeners.set(eventName, new Set());
        }

        listeners.get(eventName).add(callback);

        return () => off(eventName, callback);
    }

    function off(eventName, callback) {
        listeners.get(eventName)?.delete(callback);
    }

    function emit(eventName, payload) {
        listeners.get(eventName)?.forEach(callback => {
            try {
                callback(payload);
            } catch (error) {
                console.error(
                    `[ElectriCredit WebSocket] Listener error for "${eventName}":`,
                    error
                );
            }
        });
    }

    function scheduleReconnect() {
        if (!settings.reconnect || manuallyClosed || reconnectTimer) {
            return;
        }

        reconnectAttempts += 1;

        const delay = Math.min(
            settings.reconnectDelay * reconnectAttempts,
            settings.maxReconnectDelay
        );

        reconnectTimer = window.setTimeout(() => {
            reconnectTimer = null;
            connect();
        }, delay);
    }

    function updateConnectionStatus(status) {
        const element =
            document.getElementById("system-connection-status");

        if (!element) {
            return;
        }

        element.dataset.status = status;

        element.innerHTML = `
            <span class="ec-status-dot"></span>
            ${status === "online" ? "Online" : "Offline"}
        `;
    }

    function getState() {
        if (!socket) {
            return "closed";
        }

        switch (socket.readyState) {
            case WebSocket.CONNECTING:
                return "connecting";
            case WebSocket.OPEN:
                return "open";
            case WebSocket.CLOSING:
                return "closing";
            default:
                return "closed";
        }
    }

    return {
        connect,
        disconnect,
        send,
        on,
        off,
        getState
    };
})();

window.WebSocketClient = WebSocketClient;
