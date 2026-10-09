const UIEnhance = (() => {
    const PROFILE_CACHE_KEY = "electricredit-profile-cache-v1";
    const developerProfiles = [
        { name: "Jhon Anthony Pano", role: "Developer", image_url: "assets/developers/Jhon Anthony Pano.jpg" },
        { name: "Joselito Jr. Tambacan", role: "Developer", image_url: "assets/developers/Joselito Jr. Tambacan.jpg" },
        { name: "Jellen Anos", role: "Developer", image_url: "assets/developers/Jellen Anos.jpg" },
        { name: "Jaycob Lumayag", role: "Developer", image_url: "assets/developers/Jaycob Lumayag.jpg" },
        { name: "Joshane Rhea Paquibot", role: "Developer", image_url: "assets/developers/Joshane Rhea Paquibot.jpg" }
    ];
    const loaderMessages = [
        "Routing power to the right place…",
        "Checking the wires before we light the room…",
        "Fetching the records this screen actually needs…",
        "Keeping the kilowatts tidy while the page gets ready…",
        "Reading the latest ElectriCredit state…",
        "Making sure the numbers arrive before the interface does…",
        "Preparing a clean view of the current records…",
        "Connecting the pieces without making you chase them…",
        "Bringing the useful data forward first…",
        "Checking what changed since your last visit…",
        "Getting this page ready, not just pretending to…",
        "Waiting on the real request, not a decorative timer…",
        "Organizing the live data into something readable…",
        "Giving the server a moment to answer properly…",
        "Preparing the next view with fewer surprises…",
        "Loading the records behind this screen…",
        "Making sure the interface has something real to show…",
        "Reading local-first data from ElectriCredit…",
        "Checking the requested page from end to end…",
        "Putting the current module state in order…",
        "Fetching first, rendering second, showing it when ready…",
        "Keeping the transition smooth while the data catches up…",
        "Verifying the page before handing it over…",
        "Preparing the view your next click asked for…",
        "Waiting for the important bits to settle…",
        "Getting the page state into a usable shape…",
        "Loading only what this screen needs…",
        "Checking the latest tenant and module information…",
        "Turning server responses into a cleaner workspace…",
        "Making the next screen worth the click…",
        "Keeping the progress bar honest…",
        "Finishing the data pass before the reveal…",
        "Syncing the interface with the latest available state…",
        "Checking the page, the data, then the final paint…",
        "Preparing a quieter, faster handoff…",
        "Reading the source before drawing the screen…",
        "Getting the details right before showing them…",
        "Almost there — the real work is finishing up…",
        "Waiting for the current request to complete…",
        "Letting the browser finish the final paint…",
        "Loading the page with no imaginary percentages…",
        "Checking the records behind this route…",
        "Preparing a fresh ElectriCredit view…",
        "Keeping the UI calm while the server does its part…",
        "Getting the data settled before the transition ends…",
        "Making sure the page is ready for interaction…",
        "One request at a time, one clean screen at the end…",
        "Finishing the last real task for this view…",
        "The page is nearly ready to take over…",
        "ElectriCredit is finishing the requested view…"
    ];

    let activeLoad = null;
    let lastMessageIndex = -1;
    let loaderProgress = 0;
    let hideTimer = null;
    let lastDeveloperIndex = -1;

    function init(root = document) {
        root.querySelectorAll?.("[title]:not([data-tooltip])").forEach(el => {
            const text = el.getAttribute("title");
            if (text) {
                el.dataset.tooltip = text;
                el.removeAttribute("title");
            }
        });
    }


function loaderMarkup() {
    return `
        <div class="ec-boot-scene" data-loader-version="4">
            <section class="ec-boot-brand-panel" aria-hidden="true">
                <div class="ec-boot-brand-orbit"><span>⚡</span><i></i><b></b></div>
                <div class="ec-boot-brand-copy">
                    <span class="ec-boot-shell" data-boot-shell>ElectriCredit</span>
                    <strong>Power that stays accountable.</strong>
                    <p>Local-first prepaid electricity management for shared rental spaces.</p>
                </div>
                <div class="ec-boot-brand-meter"><i></i><i></i><i></i><i></i><i></i></div>
            </section>
            <section class="ec-boot-work-panel">
                <div class="ec-boot-topline">
                    <span class="ec-boot-live"><i></i> Loading real page data</span>
                    <button class="ec-boot-shuffle" type="button" data-boot-shuffle aria-label="Show another loading message">↻ <span>Another line</span></button>
                </div>
                <div class="ec-boot-developer" data-boot-randomize>
                    <div class="ec-boot-visual" aria-hidden="true">
                        <span class="ec-boot-loader-mark" data-boot-logo>⚡</span>
                        <img class="ec-boot-profile" data-boot-profile alt="" hidden>
                        <span class="ec-boot-pulse"></span>
                        <span class="ec-boot-ring ec-boot-ring-a"></span>
                        <span class="ec-boot-ring ec-boot-ring-b"></span>
                    </div>
                    <div class="ec-boot-developer-copy">
                        <span>Developer spotlight</span>
                        <strong data-boot-profile-name>ElectriCredit Team</strong>
                        <small data-boot-profile-role>Building the system</small>
                    </div>
                </div>
                <div class="ec-boot-copy">
                    <span class="ec-boot-eyebrow">Preparing your workspace</span>
                    <strong data-boot-title>ElectriCredit</strong>
                    <p data-boot-message>Preparing the requested view…</p>
                </div>
                <div class="ec-boot-meter">
                    <div class="ec-boot-progress-row">
                        <div class="ec-boot-progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0"><i></i></div>
                        <strong class="ec-boot-percent" data-boot-percent>0%</strong>
                    </div>
                    <div class="ec-boot-stages" data-boot-stages aria-label="Loading stages"></div>
                </div>
                <div class="ec-boot-detail-row">
                    <small data-boot-detail>Waiting for the first real task to complete.</small>
                    <span class="ec-boot-hint">Space changes the line</span>
                </div>
            </section>
        </div>`;
}
    function ensureLoader() {
        let loader = document.getElementById("ec-boot-loader");
        if (!loader) {
            loader = document.createElement("div");
            loader.id = "ec-boot-loader";
            loader.className = "ec-boot-loader is-leaving";
            loader.setAttribute("role", "status");
            loader.setAttribute("aria-live", "polite");
            loader.setAttribute("aria-hidden", "true");
            document.body.appendChild(loader);
        }

        if (!loader.querySelector('[data-loader-version="4"]')) {
            loader.innerHTML = loaderMarkup();
        }

        bindLoaderInteraction(loader);
        return loader;
    }

    function bindLoaderInteraction(loader) {
        if (!loader || loader.dataset.interactionBound === "1") return;
        loader.dataset.interactionBound = "1";
        const shuffle = () => { randomizeMessage(loader); randomizeDeveloper(loader); };
        loader.querySelector("[data-boot-shuffle]")?.addEventListener("click", shuffle);
        const visual = loader.querySelector("[data-boot-randomize]");
        if (visual) {
            visual.style.cursor = "pointer";
            visual.setAttribute("title", "Show another loading line");
            visual.addEventListener("click", shuffle);
        }
        if (!document.documentElement.dataset.ecLoaderKeyboardBound) {
            document.documentElement.dataset.ecLoaderKeyboardBound = "1";
            document.addEventListener("keydown", event => {
                const visible = document.body?.classList.contains("ec-boot-active");
                const typing = /^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement?.tagName || "");
                if (visible && !typing && event.code === "Space") {
                    event.preventDefault();
                    randomizeMessage(ensureLoader());
                }
            });
        }
    }

    function prepareBootLoader() {
        const loader = ensureLoader();
        const shell = document.body?.dataset?.shell || "index";
        const shellLabel = loader.querySelector("[data-boot-shell]");
        if (shellLabel) {
            shellLabel.textContent = shell === "developer"
                ? "Management"
                : shell === "admin"
                    ? "Administrator"
                    : "Public portal";
        }
        randomizeDeveloper(loader);
        randomizeMessage(loader);
    }

    function randomizeDeveloper(loader = ensureLoader()) {
        if (!loader || !developerProfiles.length) return null;
        let index = Math.floor(Math.random() * developerProfiles.length);
        if (developerProfiles.length > 1 && index === lastDeveloperIndex) {
            index = (index + 1) % developerProfiles.length;
        }
        lastDeveloperIndex = index;
        const profile = developerProfiles[index];
        applyProfileToLoader(loader, {
            first_name: profile.name,
            last_name: "",
            role: profile.role,
            image_url: profile.image_url
        });
        return profile;
    }

    function randomizeMessage(loader = ensureLoader()) {
        const message = loader?.querySelector("[data-boot-message]");
        if (!message || !loaderMessages.length) return "";
        let index = Math.floor(Math.random() * loaderMessages.length);
        if (loaderMessages.length > 1 && index === lastMessageIndex) {
            index = (index + 1) % loaderMessages.length;
        }
        lastMessageIndex = index;
        message.textContent = loaderMessages[index];
        return loaderMessages[index];
    }

    function resetProgress(value = 0) {
        loaderProgress = Math.max(0, Math.min(100, Math.round(Number(value) || 0)));
        renderProgress(loaderProgress, true);
    }

    function renderProgress(value, allowDecrease = false) {
        const loader = ensureLoader();
        const raw = Math.max(0, Math.min(100, Math.round(Number(value) || 0)));
        const next = allowDecrease ? raw : Math.max(loaderProgress, raw);
        loaderProgress = next;
        loader.style.setProperty("--ec-loader-progress", `${next}%`);
        const percent = loader.querySelector("[data-boot-percent]");
        if (percent) percent.textContent = `${next}%`;
        loader.querySelector(".ec-boot-progress")?.setAttribute("aria-valuenow", String(next));
    }

    function setProgress(value) {
        renderProgress(value, false);
    }

    function showLoader() {
        const loader = ensureLoader();
        if (hideTimer) {
            clearTimeout(hideTimer);
            hideTimer = null;
        }
        loader.classList.remove("is-leaving");
        loader.setAttribute("aria-hidden", "false");
        document.body?.classList.add("ec-boot-active");
        return loader;
    }

    function setStageLabels(labels = []) {
        const loader = ensureLoader();
        const root = loader.querySelector("[data-boot-stages]");
        if (!root) return;
        const safe = Array.isArray(labels) ? labels.filter(Boolean).slice(0, 6) : [];
        root.innerHTML = safe.map((label, index) => `
            <span class="ec-boot-stage" data-boot-stage="${index}">
                <i></i><b>${escapeHTML(label)}</b>
            </span>`).join("");
    }

    function updateStages(completed) {
        const loader = ensureLoader();
        loader.querySelectorAll("[data-boot-stage]").forEach((stage, index) => {
            stage.classList.toggle("is-done", index < completed);
            stage.classList.toggle("is-current", index === completed && activeLoad && completed < activeLoad.total);
        });
    }

    function beginLoading(options = {}) {
        const loader = showLoader();
        const total = Math.max(1, Number(options.total || options.steps?.length || 1));
        const session = {
            id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
            total,
            completed: 0,
            title: String(options.title || "ElectriCredit"),
            stages: Array.isArray(options.stages) ? options.stages.slice(0, total) : []
        };
        activeLoad = session;
        resetProgress(0);

        const title = loader.querySelector("[data-boot-title]");
        if (title) title.textContent = session.title;
        randomizeDeveloper(loader);
        const detail = loader.querySelector("[data-boot-detail]");
        if (detail) detail.textContent = String(options.detail || "Starting the requested work.");
        if (options.message) {
            const message = loader.querySelector("[data-boot-message]");
            if (message) message.textContent = String(options.message);
        } else {
            randomizeMessage(loader);
        }
        setStageLabels(session.stages);
        updateStages(0);
        return session;
    }

    function isCurrent(session) {
        return Boolean(session && activeLoad && session.id === activeLoad.id);
    }

    function advanceLoading(session, detail = "", message = "") {
        if (!isCurrent(session)) return false;
        session.completed = Math.min(session.total, session.completed + 1);
        const progress = Math.round((session.completed / session.total) * 100);
        renderProgress(progress, true);
        const loader = ensureLoader();
        const detailEl = loader.querySelector("[data-boot-detail]");
        if (detailEl && detail) detailEl.textContent = String(detail);
        if (message) {
            const messageEl = loader.querySelector("[data-boot-message]");
            if (messageEl) messageEl.textContent = String(message);
        } else if (session.completed < session.total) {
            randomizeMessage(loader);
        }
        updateStages(session.completed);
        if (session.completed < session.total) randomizeDeveloper(loader);
        return true;
    }

    async function finishLoading(session, detail = "Ready for interaction.") {
        if (!isCurrent(session)) return false;
        session.completed = session.total;
        renderProgress(100, true);
        updateStages(session.total);
        const loader = ensureLoader();
        const detailEl = loader.querySelector("[data-boot-detail]");
        if (detailEl) detailEl.textContent = String(detail || "Ready for interaction.");
        await nextPaint();
        if (!isCurrent(session)) return false;
        activeLoad = null;
        hideBootLoader();
        return true;
    }

    function cancelLoading(session) {
        if (!isCurrent(session)) return;
        activeLoad = null;
        hideBootLoader();
    }

    function setLoading(message, detail = "", progress = null) {
        const loader = showLoader();
        const messageEl = loader.querySelector("[data-boot-message]");
        const detailEl = loader.querySelector("[data-boot-detail]");
        if (messageEl && message) messageEl.textContent = String(message);
        if (detailEl && detail) detailEl.textContent = String(detail);
        if (progress !== null && progress !== undefined) renderProgress(progress, true);
    }

    function hideBootLoader() {
        const loader = ensureLoader();
        loader.classList.add("is-leaving");
        loader.setAttribute("aria-hidden", "true");
        document.body?.classList.remove("ec-boot-active");
        hideTimer = setTimeout(() => {
            hideTimer = null;
        }, 320);
    }


function inlineLoaderMarkup(options = {}) {
    const title = escapeHTML(options.title || "Loading");
    const detail = escapeHTML(options.detail || "Getting the latest data ready for this view.");
    return `<div class="ec-inline-loader" role="status" aria-live="polite">
        <div class="ec-inline-spinner" aria-hidden="true"><i></i><span>⚡</span></div>
        <div class="ec-inline-loader-copy">
            <strong>${title}</strong>
            <span>${detail}</span>
            <div class="ec-inline-loader-track"><i></i></div>
        </div>
    </div>`;
}

function showInlineLoading(root, options = {}) {
    const target = typeof root === "string" ? document.querySelector(root) : root;
    if (!target) return null;
    target.innerHTML = inlineLoaderMarkup(options);
    return target.querySelector(".ec-inline-loader");
}

    function showBusy(message = "Working…") {
        let el = document.getElementById("ec-global-busy");
        if (!el) {
            el = document.createElement("div");
            el.id = "ec-global-busy";
            el.className = "ec-global-busy";
            document.body.appendChild(el);
        }
        el.innerHTML = `<span class="ec-global-spinner"></span><strong>${escapeHTML(message)}</strong>`;
        el.hidden = false;
        return () => { el.hidden = true; };
    }

    async function waitForCriticalImages(root, options = {}) {
        if (!root?.querySelectorAll) return;
        const margin = Number(options.margin ?? 320);
        const timeout = Number(options.timeout ?? 2400);
        const viewportBottom = window.innerHeight + margin;
        const images = [...root.querySelectorAll("img")].filter(img => {
            const rect = img.getBoundingClientRect();
            return rect.top <= viewportBottom && rect.bottom >= -margin && !img.complete;
        });
        if (!images.length) return;

        const settled = Promise.allSettled(images.map(img => new Promise(resolve => {
            const done = () => resolve();
            img.addEventListener("load", done, { once: true });
            img.addEventListener("error", done, { once: true });
            if (img.complete) done();
        })));

        if (timeout <= 0) {
            await settled;
            return;
        }
        await Promise.race([
            settled,
            new Promise(resolve => setTimeout(resolve, timeout))
        ]);
    }

    function nextPaint() {
        return new Promise(resolve => {
            requestAnimationFrame(() => requestAnimationFrame(resolve));
        });
    }

    function cacheProfile(profile) {
        if (!profile || typeof profile !== "object") return;
        const compact = {
            first_name: String(profile.first_name || "").trim(),
            last_name: String(profile.last_name || "").trim(),
            role: String(profile.role || profile.rank || "").trim(),
            image_url: usableProfileImage(profile.image_url) ? String(profile.image_url) : ""
        };
        try {
            localStorage.setItem(PROFILE_CACHE_KEY, JSON.stringify(compact));
        } catch (_) {}
    }

    function getCachedProfile() {
        try {
            const value = JSON.parse(localStorage.getItem(PROFILE_CACHE_KEY) || "null");
            return value && typeof value === "object" ? value : null;
        } catch (_) {
            return null;
        }
    }

    function applyProfileToLoader(loader, profile) {
        if (!loader) return;
        const image = loader.querySelector("[data-boot-profile]");
        const logo = loader.querySelector("[data-boot-logo]");
        const name = loader.querySelector("[data-boot-profile-name]");
        const role = loader.querySelector("[data-boot-profile-role]");
        const validImage = usableProfileImage(profile?.image_url);
        if (image) {
            image.hidden = !validImage;
            if (validImage) image.src = profile.image_url;
            else image.removeAttribute("src");
        }
        if (logo) logo.hidden = Boolean(validImage);
        if (name) {
            const displayName = [profile?.first_name, profile?.last_name].filter(Boolean).join(" ").trim();
            name.textContent = displayName || "ElectriCredit Team";
        }
        if (role) role.textContent = String(profile?.role || "Developer");
    }

    function usableProfileImage(value) {
        const text = String(value || "").trim();
        if (!text) return false;
        return !/default[-_ ]?image|default[-_ ]?profile|placeholder/i.test(text);
    }

    function readLocal(key) {
        try { return localStorage.getItem(key); } catch (_) { return null; }
    }

    function writeLocal(key, value) {
        try { localStorage.setItem(key, value); } catch (_) {}
    }

    function escapeHTML(value) {
        return String(value ?? "")
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;")
            .replaceAll('"', "&quot;")
            .replaceAll("'", "&#039;");
    }

    document.addEventListener("DOMContentLoaded", () => {
        init();
        prepareBootLoader();
        const observer = new MutationObserver(records => records.forEach(record => record.addedNodes.forEach(node => {
            if (node.nodeType === 1) init(node);
        })));
        observer.observe(document.body, { childList: true, subtree: true });
    }, { once: true });

    return {
        init,
        beginLoading,
        advanceLoading,
        finishLoading,
        cancelLoading,
        hideBootLoader,
        inlineLoaderMarkup,
        showInlineLoading,
        showBusy,
        setLoading,
        setProgress,
        waitForCriticalImages,
        nextPaint,
        cacheProfile,
        getCachedProfile,
        readLocal,
        writeLocal,
        randomizeMessage,
        randomizeDeveloper,
        loaderMessages: [...loaderMessages]
    };
})();

window.UIEnhance = UIEnhance;
