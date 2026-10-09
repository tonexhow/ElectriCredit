/** ElectriCredit browser-to-browser support chat. WebSocket handles signaling; WebRTC carries visitor messages and media. */
const ChatClient = (() => {
    const state = {
        registered: false,
        peerId: "",
        deviceId: "",
        token: "",
        self: null,
        peers: new Map(),
        socket: null,
        connections: new Map(),
        pendingCalls: new Map(),
        pendingICE: new Map(),
        incomingFiles: new Map(),
        starting: null,
        startOptions: {},
        reconnectTimer: null,
        reconnectAttempts: 0,
        peerWaiters: []
    };

    function uuid(prefix = "") {
        const raw = crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
        return `${prefix}${raw}`.replace(/[^a-zA-Z0-9_.-]/g, "-");
    }

    function ensureIdentifiers() {
        state.deviceId = localStorage.getItem("electricredit-chat-device-id") || uuid("dev-");
        localStorage.setItem("electricredit-chat-device-id", state.deviceId);
        state.peerId = sessionStorage.getItem("electricredit-chat-peer-id") || uuid("peer-");
        sessionStorage.setItem("electricredit-chat-peer-id", state.peerId);
    }

    function detectDevice() {
        const ua = navigator.userAgent || "";
        let browser = "Browser";
        if (/Edg\//.test(ua)) browser = "Microsoft Edge";
        else if (/Chrome\//.test(ua)) browser = "Chrome";
        else if (/Firefox\//.test(ua)) browser = "Firefox";
        else if (/Safari\//.test(ua)) browser = "Safari";
        let os = navigator.platform || "Unknown OS";
        if (/Windows/i.test(ua)) os = "Windows";
        else if (/Android/i.test(ua)) os = "Android";
        else if (/iPhone|iPad|iPod/i.test(ua)) os = "iOS/iPadOS";
        else if (/Mac OS X/i.test(ua)) os = "macOS";
        else if (/Linux/i.test(ua)) os = "Linux";
        let device = "Desktop/Laptop";
        if (/iPad|Tablet/i.test(ua)) device = "Tablet";
        else if (/Android|iPhone|iPod|Mobile/i.test(ua)) device = "Mobile";
        return {
            user_agent: ua,
            platform: navigator.userAgentData?.platform || navigator.platform || "",
            browser, os, device,
            screen: `${screen.width}x${screen.height}`,
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || ""
        };
    }

    function closeSocket() {
        const socket = state.socket;
        state.socket = null;
        if (!socket) return;
        try {
            socket.onopen = socket.onmessage = socket.onerror = socket.onclose = null;
            socket.close();
        } catch (_) {}
    }

    function resetRegistration({ keepReconnect = false } = {}) {
        closeSocket();
        [...state.connections.keys()].forEach(closeConnection);
        state.registered = false;
        state.token = "";
        state.self = null;
        state.peers.clear();
        state.peerWaiters.splice(0).forEach(resolve => resolve([]));
        if (!keepReconnect) {
            clearTimeout(state.reconnectTimer);
            state.reconnectTimer = null;
            state.reconnectAttempts = 0;
        }
    }

    function applyPeers(peers = []) {
        state.peers.clear();
        peers.forEach(peer => { if (peer?.id) state.peers.set(peer.id, peer); });
        emit("electricredit:chatpeers", { peers: getPeers() });
        state.peerWaiters.splice(0).forEach(resolve => resolve(getPeers()));
    }

    async function register(options = {}) {
        ensureIdentifiers();
        const response = await window.API.post("/chat/register", {
            peer_id: state.peerId,
            device_id: state.deviceId,
            name: options.name || "Visitor",
            ...detectDevice()
        });
        state.self = response.peer;
        state.token = response.token || "";
        state.registered = true;
        applyPeers(response.peers || []);
        await openSocket();
        return state.self;
    }

    async function start(options = {}) {
        state.startOptions = { ...state.startOptions, ...options };
        if (state.starting) return state.starting;
        state.starting = (async () => {
            if (state.registered && state.socket?.readyState === WebSocket.OPEN) return state.self;
            if (state.registered) {
                try {
                    await openSocket();
                    return state.self;
                } catch (_) {
                    resetRegistration({ keepReconnect: true });
                }
            }
            return register(state.startOptions);
        })();
        try { return await state.starting; }
        finally { state.starting = null; }
    }

    function chatWebSocketURL() {
        const base = window.API.getBaseURL();
        const url = new URL(`${String(base).replace(/\/$/, "")}/chat/ws`, window.location.href);
        url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
        url.searchParams.set("peer_id", state.peerId);
        if (state.token) url.searchParams.set("token", state.token);
        return url.toString();
    }

    function openSocket() {
        if (!state.registered) return Promise.reject(new Error("Chat is not registered."));
        if (state.socket?.readyState === WebSocket.OPEN) return Promise.resolve(state.socket);
        if (state.socket?.readyState === WebSocket.CONNECTING) {
            return new Promise((resolve, reject) => {
                const timer = setTimeout(() => reject(new Error("Unable to connect this chat right now.")), 7000);
                state.socket.addEventListener("open", () => { clearTimeout(timer); resolve(state.socket); }, { once: true });
                state.socket.addEventListener("close", () => { clearTimeout(timer); reject(new Error("Unable to connect this chat right now.")); }, { once: true });
            });
        }

        closeSocket();
        return new Promise((resolve, reject) => {
            const socket = new WebSocket(chatWebSocketURL());
            state.socket = socket;
            let settled = false;
            const timeout = setTimeout(() => {
                if (settled) return;
                settled = true;
                try { socket.close(); } catch (_) {}
                reject(new Error("Unable to connect this chat right now."));
            }, 7000);

            socket.onopen = () => {
                if (socket !== state.socket) return;
                clearTimeout(timeout);
                settled = true;
                state.reconnectAttempts = 0;
                emit("electricredit:chatstatus", { connected: true });
                resolve(socket);
            };
            socket.onmessage = event => {
                if (socket !== state.socket) return;
                try { handleSignal(JSON.parse(event.data)); }
                catch (error) { console.warn("[ElectriCredit Chat] Invalid WebSocket event", error); }
            };
            socket.onerror = () => {
                if (!settled) {
                    clearTimeout(timeout);
                    settled = true;
                    reject(new Error("Unable to connect this chat right now."));
                }
            };
            socket.onclose = () => {
                if (socket !== state.socket) return;
                clearTimeout(timeout);
                state.socket = null;
                emit("electricredit:chatstatus", { connected: false });
                if (state.registered) scheduleReconnect();
            };
        });
    }

    function scheduleReconnect() {
        if (state.reconnectTimer) return;
        state.reconnectAttempts += 1;
        const delay = Math.min(900 * state.reconnectAttempts, 5000);
        state.reconnectTimer = setTimeout(async () => {
            state.reconnectTimer = null;
            // Server restart removes the in-memory anonymous token. Register again
            // instead of retrying an authorization value the server no longer knows.
            resetRegistration({ keepReconnect: true });
            try { await start(state.startOptions); }
            catch (_) { scheduleReconnect(); }
        }, delay);
    }

    async function refreshPeers() {
        if (!state.registered || state.socket?.readyState !== WebSocket.OPEN) await start(state.startOptions);
        if (state.socket?.readyState !== WebSocket.OPEN) return getPeers();
        return new Promise(resolve => {
            const timer = setTimeout(() => {
                const index = state.peerWaiters.indexOf(done);
                if (index >= 0) state.peerWaiters.splice(index, 1);
                resolve(getPeers());
            }, 1200);
            const done = peers => { clearTimeout(timer); resolve(peers); };
            state.peerWaiters.push(done);
            state.socket.send(JSON.stringify({ type: "peers" }));
        });
    }

    async function signal(to, type, data = {}) {
        if (!state.registered || state.socket?.readyState !== WebSocket.OPEN) await start(state.startOptions);
        if (state.socket?.readyState !== WebSocket.OPEN) throw new Error("Unable to connect this chat right now.");
        state.socket.send(JSON.stringify({ type: "signal", to, signal_type: type, data }));
    }

    async function handleSignal(event) {
        if (event.type === "ready" || event.type === "peers") {
            applyPeers(event.data?.peers || []);
            return;
        }
        if (["peer-joined", "peer-started"].includes(event.type) && event.peer) {
            const selfKind = state.self?.kind;
            if (event.peer.kind === selfKind) return;
            if (selfKind === "staff" && event.peer.kind === "guest" && event.type === "peer-joined" && !event.peer.started) return;
            state.peers.set(event.peer.id, event.peer);
            emit("electricredit:chatpeers", { peers: getPeers() });
            if (event.type === "peer-started" && selfKind === "staff") {
                refreshPeers().catch(() => {});
            }
            return;
        }
        if (event.type === "peer-left" && event.peer) {
            state.peers.delete(event.peer.id);
            closeConnection(event.peer.id);
            emit("electricredit:chatpeers", { peers: getPeers() });
            return;
        }
        if (event.type === "control") {
            const action = event.data?.action;
            if (action === "clear") clearAllHistory();
            if (action === "terminate" || action === "ban") resetRegistration();
            emit("electricredit:chatcontrol", { action });
            return;
        }
        if (event.type === "staff-group-message") {
            emit("electricredit:staffgroupmessage", { message: event.data || null });
            return;
        }
        if (event.type === "staff-group-typing") {
            emit("electricredit:staffgrouptyping", event.data || {});
            return;
        }
        if (event.type === "error") {
            emit("electricredit:chatstatus", { connected: true, warning: event.data?.message || "Chat signaling error." });
            return;
        }
        const from = event.from;
        if (!from) return;
        if (event.type === "offer") return handleOffer(from, event.data || {});
        if (event.type === "answer") return handleAnswer(from, event.data || {});
        if (event.type === "ice") return handleICE(from, event.data || {});
        if (event.type === "hangup") {
            closeConnection(from);
            emit("electricredit:chatcallended", { peer: peerById(from) });
        }
    }

    function rtcConfig() {
        return { iceServers: Array.isArray(window.ELECTRICREDIT_ICE_SERVERS) ? window.ELECTRICREDIT_ICE_SERVERS : [] };
    }

    function createConnection(peerId) {
        let item = state.connections.get(peerId);
        if (item) return item;
        const pc = new RTCPeerConnection(rtcConfig());
        item = { pc, channel: null, localStream: null, remoteStream: new MediaStream(), mediaKind: "data" };
        state.connections.set(peerId, item);
        pc.onicecandidate = event => { if (event.candidate) signal(peerId, "ice", { candidate: event.candidate }).catch(() => {}); };
        pc.ondatachannel = event => bindChannel(peerId, event.channel);
        pc.ontrack = event => {
            event.streams[0]?.getTracks().forEach(track => item.remoteStream.addTrack(track));
            emit("electricredit:chatstream", { peer: peerById(peerId), stream: item.remoteStream, localStream: item.localStream });
        };
        pc.onconnectionstatechange = () => {
            emit("electricredit:chatconnection", { peer: peerById(peerId), state: pc.connectionState });
            if (["failed", "closed"].includes(pc.connectionState)) closeConnection(peerId);
        };
        return item;
    }

    function bindChannel(peerId, channel) {
        const item = createConnection(peerId);
        item.channel = channel;
        channel.onmessage = event => receiveData(peerId, event.data);
        channel.onopen = () => emit("electricredit:chatconnection", { peer: peerById(peerId), state: "connected" });
        channel.onclose = () => emit("electricredit:chatconnection", { peer: peerById(peerId), state: "disconnected" });
    }

    async function ensureDataConnection(peerId) {
        const item = createConnection(peerId);
        if (item.channel?.readyState === "open") return item;
        if (!item.channel) bindChannel(peerId, item.pc.createDataChannel("electricredit-messages"));
        if (item.pc.signalingState === "stable" && !item.pc.localDescription) {
            item.mediaKind = "data";
            const offer = await item.pc.createOffer();
            await item.pc.setLocalDescription(offer);
            await signal(peerId, "offer", { description: item.pc.localDescription, media_kind: "data" });
        }
        await waitForChannel(item.channel, 9000);
        return item;
    }

    async function handleOffer(peerId, data) {
        const kind = data.media_kind || "data";
        if (kind !== "data") {
            state.pendingCalls.set(peerId, data);
            emit("electricredit:chatincomingcall", { peer: peerById(peerId), kind });
            return;
        }
        closeConnection(peerId);
        const item = createConnection(peerId);
        await item.pc.setRemoteDescription(data.description);
        await flushICE(peerId);
        const answer = await item.pc.createAnswer();
        await item.pc.setLocalDescription(answer);
        await signal(peerId, "answer", { description: item.pc.localDescription, media_kind: kind });
    }

    async function handleAnswer(peerId, data) {
        const item = state.connections.get(peerId);
        if (!item || !data.description) return;
        await item.pc.setRemoteDescription(data.description);
        await flushICE(peerId);
    }

    async function handleICE(peerId, data) {
        if (!data.candidate) return;
        const item = state.connections.get(peerId);
        if (!item || !item.pc.remoteDescription) {
            const queued = state.pendingICE.get(peerId) || [];
            queued.push(data.candidate);
            state.pendingICE.set(peerId, queued);
            return;
        }
        try { await item.pc.addIceCandidate(data.candidate); } catch (_) {}
    }

    async function flushICE(peerId) {
        const item = state.connections.get(peerId);
        if (!item?.pc?.remoteDescription) return;
        const queued = state.pendingICE.get(peerId) || [];
        state.pendingICE.delete(peerId);
        for (const candidate of queued) { try { await item.pc.addIceCandidate(candidate); } catch (_) {} }
    }

    async function startCall(peerId, kind = "audio") {
        if (!navigator.mediaDevices?.getUserMedia) throw new Error("Camera or microphone access is not supported by this browser.");
        closeConnection(peerId);
        const item = createConnection(peerId);
        item.mediaKind = kind;
        item.localStream = await navigator.mediaDevices.getUserMedia(kind === "video" ? { audio: true, video: true } : { audio: true, video: false });
        item.localStream.getTracks().forEach(track => item.pc.addTrack(track, item.localStream));
        const offer = await item.pc.createOffer();
        await item.pc.setLocalDescription(offer);
        await signal(peerId, "offer", { description: item.pc.localDescription, media_kind: kind });
        emit("electricredit:chatstream", { peer: peerById(peerId), stream: item.remoteStream, localStream: item.localStream });
    }

    async function acceptCall(peerId) {
        const data = state.pendingCalls.get(peerId);
        if (!data) throw new Error("The incoming call is no longer available.");
        state.pendingCalls.delete(peerId);
        const kind = data.media_kind || "audio";
        closeConnection(peerId);
        const item = createConnection(peerId);
        item.mediaKind = kind;
        item.localStream = await navigator.mediaDevices.getUserMedia(kind === "video" ? { audio: true, video: true } : { audio: true, video: false });
        item.localStream.getTracks().forEach(track => item.pc.addTrack(track, item.localStream));
        await item.pc.setRemoteDescription(data.description);
        await flushICE(peerId);
        const answer = await item.pc.createAnswer();
        await item.pc.setLocalDescription(answer);
        await signal(peerId, "answer", { description: item.pc.localDescription, media_kind: kind });
        emit("electricredit:chatstream", { peer: peerById(peerId), stream: item.remoteStream, localStream: item.localStream });
    }

    async function declineCall(peerId) { state.pendingCalls.delete(peerId); await signal(peerId, "hangup", {}); }
    async function hangup(peerId) { await signal(peerId, "hangup", {}).catch(() => {}); closeConnection(peerId); }

    function authorPayload() {
        return { name: state.self?.name || "Visitor", role: state.self?.role || "", image_url: state.self?.image_url || "" };
    }

    async function sendMessage(peerId, text, replyTo = null, options = {}) {
        text = String(text || "").trim();
        if (!text) return null;
        const item = await ensureDataConnection(peerId);
        const payload = { id: options.id || uuid("msg-"), type: "text", text, reply_to: replyTo || null, author: authorPayload(), at: Date.now(), from: state.peerId };
        item.channel.send(JSON.stringify(payload));
        if (options.store !== false) {
            storeHistory(peerId, { ...payload, peer_id: peerId, direction: "out" });
            emit("electricredit:chatmessage", { peer: peerById(peerId), message: { ...payload, peer_id: peerId, direction: "out" } });
        }
        return payload;
    }

    async function sendSupportMessage(text, replyTo = null) {
        const id = uuid("msg-");
        const payload = { id, type: "text", text: String(text || "").trim(), reply_to: replyTo || null, author: authorPayload(), at: Date.now(), from: state.peerId };
        if (!payload.text) return null;
        let delivered = 0;
        let deliveredPeer = null;
        for (let attempt = 0; attempt < 2 && delivered === 0; attempt += 1) {
            let staff = getPeers().filter(peer => peer.kind === "staff");
            if (!staff.length) {
                try { await refreshPeers(); } catch (_) {}
                staff = getPeers().filter(peer => peer.kind === "staff");
            }
            for (const peer of staff) {
                try {
                    const item = await ensureDataConnection(peer.id);
                    item.channel.send(JSON.stringify(payload));
                    delivered += 1;
                    deliveredPeer ||= peer;
                } catch (_) {}
            }
            if (!delivered && attempt === 0) {
                resetRegistration();
                try { await start({ name: "Visitor" }); } catch (_) {}
            }
        }
        if (!delivered) throw new Error("Unable to connect this chat right now. Please try again.");
        storeSupportHistory({ ...payload, peer_id: deliveredPeer?.id || "", direction: "out" });
        emit("electricredit:chatmessage", { peer: deliveredPeer, message: { ...payload, direction: "out" }, support: true });
        return payload;
    }

    async function sendTyping(peerId, active) {
        const item = await ensureDataConnection(peerId);
        item.channel.send(JSON.stringify({ type: "typing", active: Boolean(active), at: Date.now(), from: state.peerId }));
    }

    async function sendSupportTyping(active) {
        for (const peer of getPeers().filter(item => item.kind === "staff")) {
            try { await sendTyping(peer.id, active); } catch (_) {}
        }
    }

    async function sendReaction(peerId, messageId, reaction) {
        reaction = String(reaction || "").trim().slice(0, 16);
        if (!reaction || !messageId) return;
        const item = await ensureDataConnection(peerId);
        const payload = { type: "reaction", message_id: messageId, reaction, at: Date.now(), from: state.peerId };
        item.channel.send(JSON.stringify(payload));
        applyReaction(peerId, messageId, reaction, state.peerId);
    }

    async function sendFile(peerId, file) {
        if (!file) return;
        if (file.size > 1500000) throw new Error("Attachments are limited to 1.5 MB.");
        const data = await fileToDataURL(file);
        const item = await ensureDataConnection(peerId);
        const transferId = uuid("file-");
        const messageId = uuid("msg-");
        const messageType = file.type.startsWith("image/") ? "image" : "audio";
        const chunkSize = 12000;
        const total = Math.ceil(data.length / chunkSize);
        item.channel.send(JSON.stringify({ type: "file-start", id: messageId, transfer_id: transferId, message_type: messageType, name: file.name, mime: file.type, total, author: authorPayload(), at: Date.now(), from: state.peerId }));
        for (let index = 0; index < total; index += 1) {
            await waitForBuffer(item.channel);
            item.channel.send(JSON.stringify({ type: "file-chunk", transfer_id: transferId, index, data: data.slice(index * chunkSize, (index + 1) * chunkSize) }));
        }
        item.channel.send(JSON.stringify({ type: "file-end", transfer_id: transferId }));
        const payload = { id: messageId, type: messageType, name: file.name, mime: file.type, data, author: authorPayload(), at: Date.now(), from: state.peerId, peer_id: peerId, direction: "out" };
        if (state.self?.kind === "guest") storeSupportHistory(payload); else storeHistory(peerId, payload);
        emit("electricredit:chatmessage", { peer: peerById(peerId), message: payload, support: state.self?.kind === "guest" });
    }

    function receiveData(peerId, raw) {
        try {
            const message = JSON.parse(raw);
            if (message.type === "typing") { emit("electricredit:chattyping", { peer: peerById(peerId), active: Boolean(message.active) }); return; }
            if (message.type === "reaction") {
                applyReaction(peerId, message.message_id, message.reaction, message.from || peerId);
                if (state.self?.kind === "guest" && !message.relayed) relayGuestEvent(peerId, { ...message, relayed: true }).catch(() => {});
                return;
            }
            if (message.type === "file-start") { state.incomingFiles.set(message.transfer_id, { ...message, peer_id: peerId, chunks: new Array(message.total || 0) }); return; }
            if (message.type === "file-chunk") {
                const transfer = state.incomingFiles.get(message.transfer_id);
                if (transfer && Number.isInteger(message.index) && message.index >= 0 && message.index < transfer.chunks.length) transfer.chunks[message.index] = message.data || "";
                return;
            }
            if (message.type === "file-end") {
                const transfer = state.incomingFiles.get(message.transfer_id);
                if (!transfer || transfer.chunks.some(chunk => typeof chunk !== "string")) return;
                state.incomingFiles.delete(message.transfer_id);
                const complete = { id: transfer.id || uuid("msg-"), type: transfer.message_type, name: transfer.name, mime: transfer.mime, data: transfer.chunks.join(""), author: transfer.author, at: transfer.at || Date.now(), from: transfer.from, peer_id: peerId, direction: "in" };
                saveIncoming(peerId, complete);
                return;
            }
            message.id ||= uuid("msg-");
            const relayPayload = { ...message };
            message.peer_id = peerId;
            message.direction = "in";
            saveIncoming(peerId, message);
            if (state.self?.kind === "guest" && !relayPayload.relayed) relayGuestEvent(peerId, { ...relayPayload, relayed: true }).catch(() => {});
        } catch (_) {}
    }


    async function relayGuestEvent(originPeerId, payload) {
        if (state.self?.kind !== "guest") return;
        const staff = getPeers().filter(peer => peer.kind === "staff" && peer.id !== originPeerId);
        for (const peer of staff) {
            try {
                const connection = await ensureDataConnection(peer.id);
                connection.channel.send(JSON.stringify(payload));
            } catch (_) {}
        }
    }

    function saveIncoming(peerId, message) {
        if (state.self?.kind === "guest") storeSupportHistory(message); else storeHistory(peerId, message);
        emit("electricredit:chatmessage", { peer: peerById(peerId), message, support: state.self?.kind === "guest" });
    }

    function applyReaction(peerId, messageId, reaction, reactorId) {
        const support = state.self?.kind === "guest";
        const history = support ? getSupportHistory() : getHistory(peerId);
        const item = history.find(message => message.id === messageId);
        if (!item) return;
        item.reactions ||= {};
        const key = String(reactorId || peerId || "peer");
        if (item.reactions[key] === reaction) delete item.reactions[key];
        else item.reactions[key] = reaction;
        if (support) setSupportHistory(history); else setHistory(peerId, history);
        emit("electricredit:chatreaction", { peer: peerById(peerId), message_id: messageId, reaction, reactor_id: key });
    }

    async function waitForBuffer(channel) {
        if (channel.bufferedAmount < 256000) return;
        channel.bufferedAmountLowThreshold = 64000;
        await new Promise((resolve, reject) => {
            const timer = setTimeout(() => reject(new Error("Attachment transfer timed out.")), 10000);
            channel.addEventListener("bufferedamountlow", () => { clearTimeout(timer); resolve(); }, { once: true });
        });
    }

    function historyKey(peerId) {
        const peer = peerById(peerId);
        return `electricredit-chat-history:${peer?.device_id || peerId}`;
    }
    function getHistory(peerId) { try { return JSON.parse(localStorage.getItem(historyKey(peerId)) || "[]"); } catch { return []; } }
    function setHistory(peerId, history) { saveLocal(historyKey(peerId), history); }
    function storeHistory(peerId, message) { const history = getHistory(peerId); history.push(message); setHistory(peerId, history); }
    function getSupportHistory() { try { return JSON.parse(localStorage.getItem("electricredit-chat-history:support") || "[]"); } catch { return []; } }
    function setSupportHistory(history) { saveLocal("electricredit-chat-history:support", history); }
    function storeSupportHistory(message) { const history = getSupportHistory(); history.push(message); setSupportHistory(history); }
    function saveLocal(key, history) {
        let kept = history.slice(-160);
        while (kept.length) {
            try { localStorage.setItem(key, JSON.stringify(kept)); return; }
            catch { kept = kept.slice(Math.max(1, Math.ceil(kept.length / 4))); }
        }
    }
    function clearHistory(peerId) { localStorage.removeItem(historyKey(peerId)); emit("electricredit:chathistorycleared", { peer: peerById(peerId) }); }
    function clearSupportHistory() { localStorage.removeItem("electricredit-chat-history:support"); emit("electricredit:chathistorycleared", { support: true }); }
    function clearAllHistory() { Object.keys(localStorage).filter(key => key.startsWith("electricredit-chat-history:")).forEach(key => localStorage.removeItem(key)); emit("electricredit:chathistorycleared", { all: true }); }
    async function control(action, peerId) { return window.API.post("/chat/control", { action, target: peerId }); }

    function closeConnection(peerId) {
        const item = state.connections.get(peerId);
        if (!item) return;
        item.localStream?.getTracks().forEach(track => track.stop());
        try { item.channel?.close(); } catch (_) {}
        try { item.pc?.close(); } catch (_) {}
        state.connections.delete(peerId);
    }
    function stop() { resetRegistration(); state.pendingCalls.clear(); state.pendingICE.clear(); state.incomingFiles.clear(); }
    function peerById(id) { return state.peers.get(id) || (state.self?.id === id ? state.self : null); }
    function getPeers() { return [...state.peers.values()]; }
    function getSelf() { return state.self; }

    function waitForChannel(channel, timeout) {
        if (channel.readyState === "open") return Promise.resolve();
        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => reject(new Error("Unable to connect this chat right now.")), timeout);
            const done = () => { clearTimeout(timer); resolve(); };
            const fail = () => { clearTimeout(timer); reject(new Error("Unable to connect this chat right now.")); };
            channel.addEventListener("open", done, { once: true });
            channel.addEventListener("close", fail, { once: true });
            channel.addEventListener("error", fail, { once: true });
        });
    }
    function fileToDataURL(file) { return new Promise((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result || "")); reader.onerror = () => reject(new Error("Unable to read attachment.")); reader.readAsDataURL(file); }); }
    function emit(name, detail) { window.dispatchEvent(new CustomEvent(name, { detail })); }

    return {
        start, stop, refreshPeers, getPeers, getSelf, peerById,
        sendMessage, sendSupportMessage, sendFile, sendTyping, sendSupportTyping, sendReaction,
        getHistory, getSupportHistory, clearHistory, clearSupportHistory, clearAllHistory,
        startCall, acceptCall, declineCall, hangup, control
    };
})();

const ChatUI = (() => {
    let publicDrawer = null;
    let publicPageRoot = null;
    let adminRoot = null;
    let adminSelected = "";
    const replyTargets = new Map();
    const unreadByPeer = loadUnread();
    function loadUnread(){try{const raw=JSON.parse(sessionStorage.getItem("ec-chat-unread")||"{}");return new Map(Object.entries(raw).map(([k,v])=>[k,Number(v)||0]).filter(([,v])=>v>0))}catch{return new Map()}}
    function saveUnread(){try{sessionStorage.setItem("ec-chat-unread",JSON.stringify(Object.fromEntries(unreadByPeer)))}catch{}}
    function unreadTotal(){ return [...unreadByPeer.values()].reduce((sum,count)=>sum+Number(count||0),0); }
    function emitUnread(){
        saveUnread();
        window.dispatchEvent(new CustomEvent("electricredit:chatunread", { detail: { total: unreadTotal(), peers: Object.fromEntries(unreadByPeer) } }));
    }
    function addUnread(peerId){ if(!peerId)return; unreadByPeer.set(String(peerId),(unreadByPeer.get(String(peerId))||0)+1); emitUnread(); }
    function markRead(peerId){ if(!peerId||!unreadByPeer.has(String(peerId)))return; unreadByPeer.delete(String(peerId)); emitUnread(); }
    function getUnreadCount(peerId=""){ return peerId ? (unreadByPeer.get(String(peerId))||0) : unreadTotal(); }

    async function openPublic() {
        try { await ChatClient.start({ name: "Visitor" }); }
        catch (error) { window.Notification?.error?.(error.message || "Unable to start chat."); return; }
        if (!publicDrawer?.isConnected) {
            publicDrawer = document.createElement("aside");
            publicDrawer.className = "ec-public-chat-drawer";
            publicDrawer.setAttribute("aria-label", "ElectriCredit chat");
            publicDrawer.innerHTML = `<div class="ec-public-chat-drawer-head"><strong>Chat</strong><div class="ec-public-chat-head-actions"><button class="ec-chat-round-action" type="button" data-public-chat-call="audio" title="Audio call" aria-label="Audio call">${chatIcon("phone")}</button><button class="ec-chat-round-action" type="button" data-public-chat-call="video" title="Video call" aria-label="Video call">${chatIcon("video")}</button><button class="ec-public-chat-close" type="button" data-public-chat-close aria-label="Close chat">×</button></div></div><div id="ec-public-chat-ui" class="ec-public-chat-drawer-body"></div>`;
            document.body.appendChild(publicDrawer);
            publicDrawer.querySelector("[data-public-chat-close]")?.addEventListener("click", closePublic);
            publicDrawer.querySelectorAll("[data-public-chat-call]").forEach(button => button.addEventListener("click", () => startPublicCall(button.dataset.publicChatCall)));
            requestAnimationFrame(() => publicDrawer.classList.add("is-open"));
        }
        renderPublic();
    }

    function closePublic() {
        if (!publicDrawer) return;
        publicDrawer.classList.remove("is-open");
        setTimeout(() => { publicDrawer?.remove(); publicDrawer = null; }, 180);
    }

    async function renderPublicPage(root) {
        publicPageRoot = root;
        try { await ChatClient.start({ name: "Visitor" }); }
        catch (error) { root.innerHTML = unavailableHTML(error.message); return; }
        renderPublic();
    }

    function renderPublic() {
        const roots = [document.getElementById("ec-public-chat-ui"), publicPageRoot].filter(root => root?.isConnected);
        if (!roots.length) return;
        const staff = ChatClient.getPeers().filter(peer => peer.kind === "staff");
        const primary = staff[0] || null;
        roots.forEach(root => {
            root.innerHTML = primary ? renderPublicConversation(primary) : unavailableHTML("Support is unavailable right now. Please try again later.");
            if (primary) bindPublicConversation(root, primary);
        });
    }

    function renderPublicConversation(primary) {
        const history = ChatClient.getSupportHistory();
        const role = primary?.role ? (primary.role === "admin" ? "Administrator" : primary.role.charAt(0).toUpperCase() + primary.role.slice(1)) : "Support";
        return `<section class="ec-chat-conversation ec-chat-public-direct">
            <div class="ec-chat-profile ec-chat-public-profile"><div class="ec-chat-profile-main"><span class="ec-chat-profile-avatar">${peerAvatar(primary, "S")}</span><div><strong>${escapeHTML(primary?.name || "ElectriCredit Support")}</strong><span>${escapeHTML(role)} · Online support</span></div></div></div>
            <div class="ec-chat-media" hidden><video data-chat-remote autoplay playsinline></video><video data-chat-local autoplay muted playsinline></video><button class="ec-button ec-button-danger ec-button-sm" data-chat-hangup>End call</button></div>
            <div class="ec-chat-messages ec-chat-messenger-thread" data-chat-messages>${history.length ? history.map(message => messageHTML(message, false)).join("") : `<div class="ec-chat-thread-empty">Send a message to start a conversation.</div>`}</div>
            <div class="ec-chat-typing" data-chat-typing aria-live="polite"></div>
            ${replyPreviewHTML("support")}
            ${composerHTML("support")}
        </section>`;
    }

    async function startPublicCall(kind = "audio") {
        try {
            await ChatClient.start({ name: "Visitor" });
            const staff = ChatClient.getPeers().filter(peer => peer.kind === "staff");
            if (!staff.length) throw new Error("Support is unavailable right now.");
            await ChatClient.startCall(staff[0].id, kind);
        } catch (error) {
            window.Notification?.error?.(error.message || "Unable to start the call.");
        }
    }

    async function renderAdmin(root) {
        adminRoot = root;
        emitUnread();
        try { await ChatClient.start(); }
        catch (error) { root.innerHTML = unavailableHTML(error.message); return; }
        renderAdminNow();
    }

    function renderAdminNow() {
        if (!adminRoot?.isConnected) return;
        const peers = ChatClient.getPeers().filter(peer => peer.kind === "guest" && peer.started !== false);
        if (adminSelected && !peers.some(peer => peer.id === adminSelected)) adminSelected = "";
        if (!adminSelected && peers[0]) adminSelected = peers[0].id;
        const peer = peers.find(item => item.id === adminSelected);
        adminRoot.innerHTML = `<div class="ec-chat-layout is-staff-view">
            <aside class="ec-chat-peer-list"><div class="ec-chat-peer-head"><strong>Visitor chats</strong><span>${peers.length}</span></div><div class="ec-chat-peers">${peers.length ? peers.map(item => peerButton(item, adminSelected)).join("") : `<div class="ec-chat-empty"><strong>No visitor conversation yet</strong><span>Visitors appear here after they send their first message.</span></div>`}</div></aside>
            <section class="ec-chat-conversation">${peer ? renderAdminConversation(peer, peers) : `<div class="ec-chat-empty ec-chat-empty-large"><span class="ec-chat-empty-icon">${chatIcon("chat")}</span><strong>Visitor support</strong><p>Waiting for a visitor to start a conversation.</p></div>`}</section>
        </div>`;
        if (peer) { markRead(peer.id); bindAdminConversation(adminRoot, peer); }
        adminRoot.querySelectorAll("[data-chat-peer]").forEach(button => button.addEventListener("click", () => { adminSelected = button.dataset.chatPeer; markRead(adminSelected); renderAdminNow(); }));
    }

    function renderAdminConversation(peer, peers = []) {
        const history = ChatClient.getHistory(peer.id);
        return `<div class="ec-chat-profile ec-chat-visitor-toolbar">
            <button class="ec-chat-round-action ec-chat-visitor-info" type="button" data-chat-info-toggle title="Visitor information" aria-label="Visitor information">${chatIcon("info")}</button>
            <div class="ec-chat-call-actions ec-chat-visitor-actions">
                <button class="ec-chat-round-action" type="button" data-chat-call="audio" title="Audio call" aria-label="Audio call">${chatIcon("phone")}</button>
                <button class="ec-chat-round-action" type="button" data-chat-call="video" title="Video call" aria-label="Video call">${chatIcon("video")}</button>
                <button class="ec-chat-round-action" type="button" data-chat-control="clear" title="Clear chat" aria-label="Clear chat">${chatIcon("clear")}</button>
                <button class="ec-chat-round-action" type="button" data-chat-control="terminate" title="End conversation" aria-label="End conversation">${chatIcon("end")}</button>
                <button class="ec-chat-round-action is-danger" type="button" data-chat-control="ban" title="Block visitor" aria-label="Block visitor">${chatIcon("block")}</button>
            </div>
        </div>
        <div class="ec-chat-media" hidden><video data-chat-remote autoplay playsinline></video><video data-chat-local autoplay muted playsinline></video><button class="ec-button ec-button-danger ec-button-sm" data-chat-hangup>End call</button></div>
        <div class="ec-chat-messages ec-chat-messenger-thread" data-chat-messages>${history.length ? history.map(message => messageHTML(message, true)).join("") : `<div class="ec-chat-thread-empty">Reply when the visitor's message arrives.</div>`}</div>
        <div class="ec-chat-typing" data-chat-typing aria-live="polite"></div>
        ${replyPreviewHTML(peer.id)}
        ${composerHTML(peer.id)}`;
    }

    function composerHTML(key) {
        return `<form class="ec-chat-composer ec-chat-messenger-composer" data-chat-form>
            <div class="ec-chat-attachment-tools">
                <label class="ec-chat-tool" title="Send image">${chatIcon("image")}<input type="file" accept="image/*" data-chat-file="image" hidden></label>
                <label class="ec-chat-tool" title="Send audio">${chatIcon("audio")}<input type="file" accept="audio/*" data-chat-file="audio" hidden></label>
            </div>
            <div class="ec-chat-input-shell"><textarea class="ec-chat-input" data-chat-input rows="1" autocomplete="off" placeholder="Aa" aria-label="Message"></textarea><div class="ec-chat-emoji-wrap"><button class="ec-chat-tool ec-chat-emoji-button" type="button" data-chat-emoji aria-label="Add emoji">${chatIcon("smile")}</button><div class="ec-chat-emoji-panel" data-chat-emoji-panel hidden>${["😀","😂","😊","👍","❤️","🙏","⚡","✅","👋","🎉"].map(emoji => `<button type="button" data-chat-emoji-value="${emoji}">${emoji}</button>`).join("")}</div></div></div>
            <button class="ec-chat-send-round" type="submit" title="Send">${chatIcon("send")}</button>
        </form>`;
    }

    function replyPreviewHTML(key) {
        const reply = replyTargets.get(String(key));
        if (!reply) return `<div class="ec-chat-reply-preview" data-chat-reply-preview hidden></div>`;
        return `<div class="ec-chat-reply-preview" data-chat-reply-preview><div><strong>Replying to ${escapeHTML(reply.author?.name || (reply.direction === "out" ? "your message" : "message"))}</strong><span>${escapeHTML(replySnippet(reply))}</span></div><button type="button" data-chat-reply-cancel aria-label="Cancel reply">×</button></div>`;
    }

    function peerButton(peer, selected) {
        const unread=getUnreadCount(peer.id);
        return `<button class="ec-chat-peer ${peer.id === selected ? "is-active" : ""}" type="button" data-chat-peer="${escapeAttr(peer.id)}"><span class="ec-chat-avatar ec-chat-anonymous-avatar">${chatIcon("user")}</span><span class="ec-chat-peer-copy"><strong>Visitor</strong><small>${escapeHTML(`${peer.device || "Device"} · ${peer.os || "Unknown"}`)}</small></span>${unread?`<span class="ec-chat-unread-badge">${unread>99?"99+":unread}</span>`:""}</button>`;
    }

    function bindPublicConversation(root, primary) {
        const key = "support";
        bindComposer(root, key, async (text, reply) => ChatClient.sendSupportMessage(text, reply), active => ChatClient.sendSupportTyping(active), async file => {
            const staff = ChatClient.getPeers().filter(peer => peer.kind === "staff");
            if (!staff.length) throw new Error("Support is unavailable right now.");
            await ChatClient.sendFile(staff[0].id, file);
        });
        bindMessageActions(root, key, false);
        root.querySelector("[data-chat-hangup]")?.addEventListener("click", () => ChatClient.hangup(primary.id));
        scrollMessages(root);
    }

    function bindAdminConversation(root, peer) {
        const key = peer.id;
        bindComposer(root, key, (text, reply) => ChatClient.sendMessage(peer.id, text, reply), active => ChatClient.sendTyping(peer.id, active), file => ChatClient.sendFile(peer.id, file));
        bindMessageActions(root, key, true);
        root.querySelectorAll("[data-chat-call]").forEach(button => button.addEventListener("click", async () => { try { await ChatClient.startCall(peer.id, button.dataset.chatCall); } catch (error) { window.Notification?.error?.(error.message); } }));
        root.querySelector("[data-chat-info-toggle]")?.addEventListener("click",()=>openVisitorInfo(peer));
        root.querySelector("[data-chat-hangup]")?.addEventListener("click", () => ChatClient.hangup(peer.id));
        root.querySelectorAll("[data-chat-control]").forEach(button => button.addEventListener("click", async () => {
            const action = button.dataset.chatControl;
            const confirmed = await window.Modal.confirm({ title: action === "ban" ? "Block visitor" : action === "terminate" ? "End conversation" : "Clear conversation", message: action === "ban" ? "Block this visitor from chat until the server restarts?" : action === "terminate" ? "End this visitor conversation?" : "Clear this conversation from this browser?", confirmText: action === "ban" ? "Block" : action === "terminate" ? "End" : "Clear" });
            if (!confirmed) return;
            try { await ChatClient.control(action, peer.id); if (action === "clear") ChatClient.clearHistory(peer.id); } catch (error) { window.Notification?.error?.(error.message); }
        }));
        scrollMessages(root);
    }

    function bindComposer(root, key, send, typingSender, fileSender) {
        const input = root.querySelector("[data-chat-input]");
        input?.addEventListener("keydown", event => { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); root.querySelector("[data-chat-form]")?.requestSubmit(); } });
        root.querySelector("[data-chat-form]")?.addEventListener("submit", async event => {
            event.preventDefault();
            const text = input?.value || "";
            if (!text.trim()) return;
            const reply = replyTargets.get(String(key));
            const replyPayload = reply ? { id: reply.id, text: replySnippet(reply), type: reply.type, author: reply.author || null } : null;
            input.disabled = true;
            try { await send(text, replyPayload); input.value = ""; replyTargets.delete(String(key)); typingSender(false).catch?.(() => {}); rerender(); }
            catch (error) { window.Notification?.error?.(error.message); }
            finally { input.disabled = false; input.focus(); }
        });
        root.querySelectorAll("[data-chat-file]").forEach(fileInput => fileInput.addEventListener("change", async event => { const file = event.target.files?.[0]; if (!file) return; try { await fileSender(file); } catch (error) { window.Notification?.error?.(error.message); } event.target.value = ""; }));
        const emojiButton = root.querySelector("[data-chat-emoji]");
        const emojiPanel = root.querySelector("[data-chat-emoji-panel]");
        emojiButton?.addEventListener("click", () => { emojiPanel.hidden = !emojiPanel.hidden; });
        root.querySelectorAll("[data-chat-emoji-value]").forEach(button => button.addEventListener("click", () => { if (!input) return; input.value += button.dataset.chatEmojiValue || ""; emojiPanel.hidden = true; input.focus(); input.dispatchEvent(new Event("input")); }));
        let typing = false; let timer = null;
        input?.addEventListener("input", () => {
            if (input.value.trim() && !typing) { typing = true; Promise.resolve(typingSender(true)).catch(() => {}); }
            clearTimeout(timer);
            timer = setTimeout(() => { if (typing) { typing = false; Promise.resolve(typingSender(false)).catch(() => {}); } }, 1000);
        });
        root.querySelector("[data-chat-reply-cancel]")?.addEventListener("click", () => { replyTargets.delete(String(key)); rerender(); });
    }

    function bindMessageActions(root, key, admin) {
        root.querySelectorAll("[data-chat-reply]").forEach(button => button.addEventListener("click", () => {
            const message = messageById(key, button.dataset.chatReply, admin);
            if (!message) return;
            replyTargets.set(String(key), message);
            rerender();
        }));
        root.querySelectorAll("[data-chat-react-open]").forEach(button => button.addEventListener("click", event => {
            const message = messageById(key, button.dataset.chatReactOpen, admin);
            if (message) openReactionPicker(message, key, admin, event.currentTarget.getBoundingClientRect());
        }));
        root.querySelectorAll(".ec-chat-message[data-message-id]").forEach(article => {
            let timer = null;
            let startX = 0;
            let startY = 0;
            const cancel = () => { clearTimeout(timer); timer = null; };
            article.addEventListener("touchstart", event => {
                if (event.touches.length !== 1) return;
                startX = event.touches[0].clientX; startY = event.touches[0].clientY;
                timer = setTimeout(() => {
                    const message = messageById(key, article.dataset.messageId, admin);
                    if (!message) return;
                    openReactionPicker(message, key, admin, { left:startX, right:startX, top:startY, bottom:startY, width:0, height:0 });
                    navigator.vibrate?.(25);
                }, 520);
            }, { passive:true });
            article.addEventListener("touchmove", event => {
                const touch = event.touches[0]; if (!touch) return;
                if (Math.abs(touch.clientX-startX)>10 || Math.abs(touch.clientY-startY)>10) cancel();
            }, { passive:true });
            article.addEventListener("touchend", cancel, { passive:true });
            article.addEventListener("touchcancel", cancel, { passive:true });
            article.addEventListener("contextmenu", event => { if (matchMedia("(pointer: coarse)").matches) event.preventDefault(); });
        });
    }

    let reactionPopover = null;
    function closeReactionPicker() {
        reactionPopover?.remove(); reactionPopover = null;
        document.removeEventListener("pointerdown", reactionOutsideHandler, true);
        window.removeEventListener("resize", closeReactionPicker);
        window.removeEventListener("scroll", closeReactionPicker, true);
    }
    function reactionOutsideHandler(event) { if (reactionPopover && !reactionPopover.contains(event.target)) closeReactionPicker(); }
    function openReactionPicker(message, key, admin, rect) {
        closeReactionPicker();
        const options = [["👍","Like"],["❤️","Love"],["😂","Haha"],["🥰","Care"],["😢","Sad"],["😡","Angry"],["👏","Clap"],["🔥","Fire"],["🙏","Thanks"],["🎉","Celebrate"],["⚡","Energy"],["🤝","Agree"]];
        const selfId = String(ChatClient.getSelf()?.id || "");
        const selected = message.reactions?.[selfId] || "";
        const panel = document.createElement("div");
        panel.className = "ec-chat-reaction-popover";
        panel.setAttribute("role","menu");
        panel.innerHTML = options.map(([emoji,label]) => `<button type="button" title="${label}" aria-label="${label}" data-reaction="${emoji}" class="${selected===emoji?"is-selected":""}">${emoji}</button>`).join("");
        document.body.appendChild(panel); reactionPopover = panel;
        const box = panel.getBoundingClientRect();
        const viewportW = document.documentElement.clientWidth;
        const viewportH = document.documentElement.clientHeight;
        let left = Math.min(Math.max(8, (rect.left + rect.right)/2 - box.width/2), viewportW-box.width-8);
        let top = rect.top - box.height - 10;
        if (top < 8) top = Math.min(viewportH-box.height-8, rect.bottom + 10);
        panel.style.left = `${left}px`; panel.style.top = `${Math.max(8,top)}px`;
        panel.querySelectorAll("[data-reaction]").forEach(button => button.addEventListener("click", async () => {
            const peerId = admin ? String(key) : (message.peer_id || ChatClient.getPeers().find(peer => peer.kind === "staff")?.id);
            if (!peerId) return closeReactionPicker();
            try { await ChatClient.sendReaction(peerId, message.id, button.dataset.reaction); }
            catch (error) { window.Notification?.error?.(error.message); }
            closeReactionPicker();
        }));
        setTimeout(()=>document.addEventListener("pointerdown",reactionOutsideHandler,true),0);
        window.addEventListener("resize",closeReactionPicker,{once:true});
        window.addEventListener("scroll",closeReactionPicker,{once:true,capture:true});
    }

    function openVisitorInfo(peer) {
        const peers = ChatClient.getPeers().filter(item => item.kind === "guest" && item.started !== false);
        const conversations = peers.length > 1 ? `<section class="ec-chat-visitor-conversations"><strong>Conversations</strong><div>${peers.map((item,index)=>`<button type="button" class="${item.id===peer.id?"is-active":""}" data-visitor-switch="${escapeAttr(item.id)}"><span>${chatIcon("chat")}</span><b>Visitor ${index+1}</b>${getUnreadCount(item.id)?`<em>${getUnreadCount(item.id)>99?"99+":getUnreadCount(item.id)}</em>`:""}</button>`).join("")}</div></section>` : "";
        const modal = window.Modal.open({ title:"Visitor information", hideCancel:true, content:`<div class="ec-chat-visitor-modal">${conversations}<div class="ec-chat-device-details"><span><b>Device</b>${escapeHTML(peer.device||"Unknown")}</span><span><b>Browser</b>${escapeHTML(peer.browser||"Browser")}</span><span><b>Platform</b>${escapeHTML(peer.platform||peer.os||"Unknown")}</span><span><b>Screen</b>${escapeHTML(peer.screen||"—")}</span><span><b>Time zone</b>${escapeHTML(peer.timezone||"—")}</span><span><b>Network</b>${escapeHTML(peer.ip||"—")}</span></div></div>` });
        modal.querySelectorAll("[data-visitor-switch]").forEach(button=>button.addEventListener("click",()=>{adminSelected=button.dataset.visitorSwitch;markRead(adminSelected);window.Modal.close({force:true});renderAdminNow()}));
    }

    function messageById(key, id, admin) {
        const history = admin ? ChatClient.getHistory(String(key)) : ChatClient.getSupportHistory();
        return history.find(message => message.id === id);
    }

    function messageHTML(message, admin) {
        const own = message.direction === "out";
        const content = message.type === "image"
            ? `<img class="ec-chat-image" src="${escapeAttr(message.data || "")}" alt="${escapeAttr(message.name || "Image")}">`
            : message.type === "audio"
                ? `<audio controls src="${escapeAttr(message.data || "")}"></audio>`
                : `<div class="ec-chat-markdown">${renderMarkdown(message.text || "")}</div>`;
        const authorName = message.author?.name || (own ? "You" : "Support");
        const avatar = message.author?.image_url
            ? `<img src="${escapeAttr(message.author.image_url)}" alt="" loading="lazy">`
            : (admin && !own ? chatIcon("user") : escapeHTML(authorName.charAt(0).toUpperCase()));
        const reactions = Object.values(message.reactions || {});
        const counts = reactions.reduce((map, emoji) => (map[emoji] = (map[emoji] || 0) + 1, map), {});
        const reactionHTML = Object.entries(counts).map(([emoji,count]) => `<span>${emoji}${count > 1 ? ` ${count}` : ""}</span>`).join("");
        const reply = message.reply_to ? `<div class="ec-chat-message-reply"><strong>${escapeHTML(message.reply_to.author?.name || "Reply")}</strong><span>${escapeHTML(message.reply_to.text || "Message")}</span></div>` : "";
        const time = new Date(message.at || Date.now()).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
        return `<article class="ec-chat-message ${own ? "is-own" : "is-other"}" data-message-id="${escapeAttr(message.id || "")}">
            ${own ? "" : `<span class="ec-chat-message-avatar" title="${escapeAttr(authorName)}">${avatar}</span>`}
            <div class="ec-chat-message-stack">
                ${own ? "" : `<span class="ec-chat-message-author">${escapeHTML(authorName)}</span>`}
                <div class="ec-chat-bubble">${reply}${content}</div>
                <div class="ec-chat-message-meta"><time>${escapeHTML(time)}</time>${reactionHTML ? `<div class="ec-chat-message-reactions">${reactionHTML}</div>` : ""}</div>
                <div class="ec-chat-message-actions"><button type="button" data-chat-reply="${escapeAttr(message.id || "")}">Reply</button><button type="button" data-chat-react-open="${escapeAttr(message.id || "")}" title="React">${chatIcon("smile")}<span>React</span></button></div>
            </div>
        </article>`;
    }

    function renderMarkdown(value) {
        let text = escapeHTML(value || "");
        text = text.replace(/```([\s\S]*?)```/g, (_, code) => `<pre><code>${code.trim()}</code></pre>`);
        text = text.replace(/`([^`\n]+)`/g, "<code>$1</code>");
        text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
        text = text.replace(/__([^_]+)__/g, "<strong>$1</strong>");
        text = text.replace(/(^|[^*])\*([^*\n]+)\*/g, "$1<em>$2</em>");
        text = text.replace(/~~([^~]+)~~/g, "<del>$1</del>");
        text = text.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
        return text.replace(/\n/g, "<br>");
    }

    function replySnippet(message) {
        if (message.type === "image") return "Photo";
        if (message.type === "audio") return "Audio";
        return String(message.text || "Message").replace(/\s+/g, " ").slice(0, 100);
    }

    function unavailableHTML(message) { return `<div class="ec-chat-empty ec-chat-empty-large"><span class="ec-chat-empty-icon">${chatIcon("chat")}</span><strong>Chat unavailable</strong><p>${escapeHTML(message || "Unable to connect this chat right now.")}</p></div>`; }
    function peerAvatar(peer, fallback) { return peer?.image_url ? `<img src="${escapeAttr(peer.image_url)}" alt="" loading="lazy">` : escapeHTML((peer?.name || fallback || "?").charAt(0).toUpperCase()); }
    function scrollMessages(root) { const el = root.querySelector("[data-chat-messages]"); if (el) el.scrollTop = el.scrollHeight; }

    function showStreams(detail) {
        const roots = [document.getElementById("ec-public-chat-ui"), publicPageRoot, adminRoot].filter(root => root?.isConnected);
        roots.forEach(root => {
            const media = root.querySelector(".ec-chat-media"); if (!media) return;
            if (root === adminRoot && detail.peer?.id !== adminSelected) return;
            media.hidden = false;
            const remote = media.querySelector("[data-chat-remote]"); const local = media.querySelector("[data-chat-local]");
            if (remote && detail.stream) remote.srcObject = detail.stream;
            if (local && detail.localStream) local.srcObject = detail.localStream;
        });
    }

    function rerender() {
        if (publicDrawer?.isConnected || publicPageRoot?.isConnected) renderPublic();
        if (adminRoot?.isConnected) renderAdminNow();
    }

    window.addEventListener("electricredit:chatpeers", rerender);
    window.addEventListener("electricredit:chatmessage", event => {
        const peerId=event.detail?.peer?.id||event.detail?.message?.peer_id||"";
        const incoming=event.detail?.message?.direction==="in";
        const staffView=ChatClient.getSelf()?.kind==="staff";
        const viewing=Boolean(adminRoot?.isConnected&&adminRoot.querySelector("[data-chat-messages]")&&adminSelected===peerId&&document.body.classList.contains("ec-admin-chat-page")&&document.visibilityState==="visible");
        if(staffView&&incoming&&peerId&&!viewing)addUnread(peerId);
        else if(staffView&&incoming&&viewing)markRead(peerId);
        rerender();
    });
    window.addEventListener("electricredit:chatreaction", rerender);
    window.addEventListener("electricredit:chathistorycleared", rerender);
    window.addEventListener("electricredit:chatstream", event => showStreams(event.detail));
    window.addEventListener("electricredit:chatcallended", rerender);
    window.addEventListener("electricredit:chattyping", event => {
        const peerId = event.detail?.peer?.id;
        [document.getElementById("ec-public-chat-ui"), publicPageRoot, adminRoot].filter(root => root?.isConnected).forEach(root => {
            if (root === adminRoot && adminSelected !== peerId) return;
            const el = root.querySelector("[data-chat-typing]");
            if (el) el.textContent = event.detail?.active ? `${event.detail?.peer?.name || "Support"} is typing…` : "";
        });
    });
    document.addEventListener("visibilitychange",()=>{
        if(document.visibilityState==="visible"&&adminSelected&&adminRoot?.isConnected&&adminRoot.querySelector("[data-chat-messages]"))markRead(adminSelected);
    });
    window.addEventListener("electricredit:chatincomingcall", async event => {
        const peer = event.detail.peer; const kind = event.detail.kind;
        const accept = await window.Modal.confirm({ title: `Incoming ${kind} call`, message: `${peer?.name || "Chat user"} is calling.`, confirmText: "Accept", cancelText: "Decline" });
        try { if (accept) await ChatClient.acceptCall(peer.id); else await ChatClient.declineCall(peer.id); } catch (error) { window.Notification?.error?.(error.message); }
    });

    function chatIcon(name) {
        const paths = {
            phone:'<path d="M7 4 4 6c0 7.7 6.3 14 14 14l2-3-4-3-2 2c-3.1-1.2-5.8-3.9-7-7l2-2-2-3Z"/>',
            video:'<rect x="3" y="6" width="13" height="12" rx="2"/><path d="m16 10 5-3v10l-5-3"/>',
            image:'<rect x="3" y="5" width="18" height="14" rx="2"/><circle cx="9" cy="10" r="2"/><path d="m4 17 5-4 3 3 2-2 6 5"/>',
            audio:'<path d="M9 18V5l10-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="16" cy="16" r="3"/>',
            smile:'<circle cx="12" cy="12" r="9"/><path d="M8 10h.01M16 10h.01M8 14c1 2 2.5 3 4 3s3-1 4-3"/>',
            send:'<path d="m3 11 18-8-7 18-3-7-8-3Z"/><path d="m11 14 4-4"/>',
            chat:'<path d="M4 5h16v11H9l-5 4V5Z"/><path d="M8 9h8M8 12h5"/>',
            user:'<circle cx="12" cy="8" r="3.2"/><path d="M5.5 20c.8-4 3-6 6.5-6s5.7 2 6.5 6"/>',
            chevron:'<path d="m8 10 4 4 4-4"/>',
            clear:'<path d="M4 7h16M9 3h6l1 4H8l1-4ZM7 7l1 14h8l1-14"/>',
            end:'<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6M15 9l-6 6"/>',
            block:'<circle cx="12" cy="12" r="9"/><path d="m5.7 5.7 12.6 12.6"/>',
            info:'<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>'
        };
        return `<svg class="ec-icon" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${paths[name] || paths.send}</svg>`;
    }
    function formatRole(value) { value = String(value || ""); return value ? value.charAt(0).toUpperCase() + value.slice(1) : ""; }
    function escapeHTML(value) { return String(value ?? "").replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;"); }
    function escapeAttr(value) { return escapeHTML(value); }

    return { openPublic, closePublic, renderPublicPage, renderAdmin, renderMarkdown, startPublicCall, getUnreadCount };
})();

window.ChatClient = ChatClient;
window.ChatUI = ChatUI;
