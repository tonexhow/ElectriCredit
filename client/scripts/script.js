/**
 * ElectriCredit - Frontend Bootstrap
 * Index is complete here; Admin/Developer routing will be connected later.
 */

const ElectriCredit = (() => {
    const state = {
        shell: detectShell(),
        currentRoute: "",
        currentPage: null,
        initialized: false,
        sessionUser: null,
        supportAvailable: 0,
        supportStream: null,
        publicData: new Map(),
        prefetching: new Map(),
        routeSequence: 0,
        workspacePreloadStarted: false
    };

    const INDEX_ROUTES = {
        home: () => window.HomeLogic,
        registry: () => window.RegistryLogic,
        hub: () => window.HubLogic,
        tenant: () => window.TenantLogic,
        about: () => window.AboutLogic,
        chat: () => window.IndexChatLogic
    };

    const INDEX_ROUTE_ALIASES = {
        users: "tenant"
    };
    const INDEX_DATA_ENDPOINTS = {
        home: { endpoint: "/public/home", pick: value => value || {} },
        registry: { endpoint: "/public/registries", pick: value => value?.registries || value || [] },
        hub: { endpoint: "/public/hubs", pick: value => value?.hubs || value || [] },
        tenant: { endpoint: "/public/users", pick: value => value?.users || value || [] },
        about: { endpoint: "/public/about", pick: value => value || {} }
    };
    const PUBLIC_CACHE_PREFIX = "electricredit-public-v1:";
    const PUBLIC_CACHE_TTL = 90 * 1000;

    function detectShell() {
        const file = window.location.pathname
            .split("/")
            .pop()
            .toLowerCase();

        if (file === "admin.html") {
            return "admin";
        }

        if (file === "developer.html") {
            return "developer";
        }

        return "index";
    }

    async function initialize() {
        if (state.initialized) {
            return;
        }

        state.initialized = true;
        window.Theme?.initialize?.();

        const shellName = state.shell === "developer"
            ? "Management"
            : state.shell === "admin"
                ? "Administrator"
                : "ElectriCredit";
        const loader = window.UIEnhance?.beginLoading?.({
            title: shellName,
            total: 5,
            stages: state.shell === "index"
                ? ["Interface", "Page data", "Session", "Support", "Ready"]
                : ["Interface", "Session", "Access", "Workspace", "Ready"],
            detail: "Starting the page from its real dependencies."
        });

        window.UIEnhance?.advanceLoading?.(loader, "Theme and shell controls are initialized.");

        let allowed = true;
        if (state.shell === "index") {
            await initializeIndex(loader);
        } else {
            allowed = await initializeProtectedShell(loader);
        }

        if (!allowed) {
            window.UIEnhance?.cancelLoading?.(loader);
            return;
        }

        await window.UIEnhance?.finishLoading?.(loader, "All required startup tasks have completed.");

        window.dispatchEvent(
            new CustomEvent("electricredit:ready", {
                detail: { shell: state.shell }
            })
        );
    }

    async function initializeProtectedShell(loader) {
        state.sessionUser = await window.Auth?.loadSession?.() || null;
        window.UIEnhance?.advanceLoading?.(loader, "Session lookup completed.");

        if (!state.sessionUser) {
            window.location.replace("index.html");
            return false;
        }

        const role = String(state.sessionUser.role || "").trim().toLowerCase();

        if (!["owner", "admin", "developer"].includes(role)) {
            await window.Auth?.logout?.();
            window.location.replace("index.html");
            return false;
        }

        if (state.shell === "developer" && role !== "developer") {
            window.location.replace("admin.html");
            return false;
        }

        window.Auth?.applyRoleAccess?.(document);
        bindDeveloperAccessGuard();
        window.UIEnhance?.advanceLoading?.(loader, "Role and workspace access are verified.");

        if (state.shell === "admin") {
            await window.AdminApp?.mount?.(state.sessionUser, { loading: false });
        } else if (state.shell === "developer") {
            await window.DeveloperApp?.mount?.(state.sessionUser, { loading: false });
        }
        window.UIEnhance?.advanceLoading?.(loader, "The requested workspace and its current page are rendered.");

        const accessObserver = new MutationObserver(() => {
            window.Auth?.applyRoleAccess?.(document);
        });

        accessObserver.observe(document.body, {
            childList: true,
            subtree: true
        });

        window.UIEnhance?.advanceLoading?.(loader, "Live controls and access guards are active.");
        return true;
    }

    function bindDeveloperAccessGuard() {
        document.addEventListener("click", event => {
            const control = event.target.closest(
                '[data-developer-access], [data-route="developer"], a[href$="developer.html"]'
            );

            if (!control || window.Auth?.canAccessDeveloper?.()) {
                return;
            }

            event.preventDefault();
            event.stopPropagation();
            window.Notification?.info?.(
                "Developer tools are available only to Developer accounts."
            );
        }, true);
    }

    async function initializeIndex(loader) {
        renderFooter();
        bindIndexNavigation();
        bindIndexActions();
        window.Theme?.bindControls?.(document);

        const initialRoute = getRouteFromHash() || "home";
        await loadIndexRoute(initialRoute, {
            updateHistory: false,
            replaceHistory: !getRouteFromHash(),
            loading: false
        });
        window.UIEnhance?.advanceLoading?.(loader, `${humanRoute(initialRoute)} data and interface are ready.`);

        const sessionPromise = refreshSession()
            .finally(() => window.UIEnhance?.advanceLoading?.(loader, "Login/session status is ready."));
        const supportPromise = refreshSupportAvailability()
            .finally(() => window.UIEnhance?.advanceLoading?.(loader, "Support availability check is complete."));
        await Promise.allSettled([sessionPromise, supportPromise]);

        startSupportAvailabilityStream();
        scheduleIndexPrefetch(initialRoute);
        window.UIEnhance?.advanceLoading?.(loader, "Live support status and background prefetch are active.");
    }

    function renderFooter() {
        const root = document.getElementById("footer-root");

        if (root && window.Footer?.render) {
            root.innerHTML = window.Footer.render();
        }
    }

    function bindIndexNavigation() {
        document.addEventListener("click", event => {
            const routeButton = event.target.closest("[data-index-route]");

            if (!routeButton) {
                return;
            }

            event.preventDefault();
            setRoute(routeButton.dataset.indexRoute);
            closeMobileNavigation();
        });

        document.querySelectorAll("[data-index-route]").forEach(control => {
            const warm = () => prefetchIndexData(control.dataset.indexRoute);
            control.addEventListener("pointerenter", warm, { passive: true });
            control.addEventListener("focus", warm);
            control.addEventListener("touchstart", warm, { passive: true, once: true });
        });

        window.addEventListener("popstate", () => {
            loadIndexRoute(getRouteFromHash() || "home", {
                updateHistory: false
            });
        });

        const menuButton = document.getElementById("index-menu-toggle");

        menuButton?.addEventListener("click", () => {
            const nav = document.getElementById("index-navigation");
            const open = nav?.classList.toggle("is-open") || false;

            menuButton.setAttribute("aria-expanded", String(open));
        });
    }

    function bindIndexActions() {
        document
            .getElementById("index-login-button")
            ?.addEventListener("click", () => {
                if (state.sessionUser) {
                    window.Auth?.redirectByRole?.();
                    return;
                }

                window.LoginLogic?.open?.();
            });

        document
            .getElementById("index-chat-button")
            ?.addEventListener("click", () => {
                if (state.supportAvailable <= 0) {
                    return;
                }

                if (window.matchMedia("(max-width: 760px)").matches) {
                    setRoute("chat");
                    return;
                }

                window.ChatUI?.openPublic?.();
            });
    }

    async function refreshSession() {
        state.sessionUser = await window.Auth?.loadSession?.() || null;
        updateLoginAction();
        scheduleAdminWorkspacePreload(state.sessionUser);
        return state.sessionUser;
    }

    function updateLoginAction() {
        const button = document.getElementById("index-login-button");
        const label = document.getElementById("index-login-label");

        if (!button || !label) {
            return;
        }

        if (!state.sessionUser) {
            label.textContent = "Login";
            button.title = "Authorized login";
            return;
        }

        const role = formatRole(state.sessionUser.role);
        label.textContent =
            state.sessionUser.first_name ||
            role ||
            "Open";

        button.title = `Signed in as ${role}. Open your workspace.`;
    }

    async function refreshSupportAvailability() {
        let count = 0;

        try {
            const response = await window.API?.get?.(
                "/public/support/availability",
                { timeoutMs: 4000 }
            );

            count = Number(response?.available || 0);
        } catch {
            count = 0;
        }

        state.supportAvailable =
            Number.isFinite(count) && count > 0
                ? Math.floor(count)
                : 0;

        updateChatAction();
        return state.supportAvailable;
    }

    function startSupportAvailabilityStream() {
        if (!window.EventSource || state.supportStream) {
            return;
        }

        try {
            const base = window.API?.getBaseURL?.() || "/api";
            const source = new EventSource(`${base}/public/support/events`, {
                withCredentials: true
            });
            state.supportStream = source;

            source.onmessage = event => {
                try {
                    const response = JSON.parse(event.data || "{}");
                    const count = Number(response?.available || 0);
                    state.supportAvailable = Number.isFinite(count) && count > 0
                        ? Math.floor(count)
                        : 0;
                    updateChatAction();
                } catch (_) {}
            };

            source.onerror = () => {
                // The browser automatically reconnects the live status stream.
            };
        } catch (_) {}
    }

    function updateChatAction() {
        const button = document.getElementById("index-chat-button");
        const badge = document.getElementById("index-chat-count");

        if (!button || !badge) {
            return;
        }

        const available = state.supportAvailable;

        badge.classList.toggle("is-visible", available > 0);
        button.disabled = available === 0;
        button.title = available > 0 ? "Open ElectriCredit chat" : "Chat is currently unavailable";
    }

    async function setRoute(route, options = {}) {
        return loadIndexRoute(route, {
            updateHistory: options.updateHistory !== false
        });
    }

    async function loadIndexRoute(route, options = {}) {
        if (state.shell !== "index") {
            return "";
        }

        const requestID = ++state.routeSequence;
        const requestedRoute = normalizeRoute(route);
        const canonicalRoute = INDEX_ROUTE_ALIASES[requestedRoute] || requestedRoute;
        const normalized = Object.prototype.hasOwnProperty.call(INDEX_ROUTES, canonicalRoute)
            ? canonicalRoute
            : "home";
        const page = INDEX_ROUTES[normalized]?.();

        if (!page?.mount) {
            console.error(`[ElectriCredit] Page "${normalized}" is unavailable.`);
            return "";
        }

        const content = document.getElementById("page-content");
        const changingPage = Boolean(state.currentPage) && state.currentRoute !== normalized;

        if (changingPage && content) {
            content.classList.add("is-leaving");
            await window.UIEnhance?.nextPaint?.();
            if (requestID !== state.routeSequence) return "";
        }

        if (state.currentPage?.unmount) {
            state.currentPage.unmount();
        }

        state.currentPage = page;
        state.currentRoute = normalized;
        document.body.classList.toggle("ec-index-chat-page-active", normalized === "chat");
        updateNavigationState(normalized);

        if (options.updateHistory !== false) {
            const next = `#${normalized}`;
            if (window.location.hash !== next) {
                history.pushState({ route: normalized }, "", next);
            }
        } else if (options.replaceHistory) {
            history.replaceState({ route: normalized }, "", `#${normalized}`);
        }

        if (content) {
            content.innerHTML = "";
            content.classList.remove("is-leaving");
            if (changingPage) content.classList.add("is-entering");
            if (options.loading !== false) {
                const routeDetail = normalized === "tenant"
                    ? "Loading tenant records and profile cards…"
                    : normalized === "about"
                        ? "Loading team profiles and About information…"
                        : "Loading the data and interface for this section.";
                window.UIEnhance?.showInlineLoading?.(content, {
                    title: `Opening ${humanRoute(normalized)}`,
                    detail: routeDetail
                });
            }
        }

        let providedData;
        if (INDEX_DATA_ENDPOINTS[normalized]) {
            providedData = readPublicCache(normalized);
            if (providedData === undefined) {
                try {
                    providedData = await prefetchIndexData(normalized, true);
                    if (requestID !== state.routeSequence) return "";
                } catch (_) {
                    // Do not perform a second startup fetch when the backend is
                    // unreachable. Render the page with its safe empty shape and
                    // let later navigation retry normally.
                    providedData = (normalized === "home" || normalized === "about") ? {} : [];
                }
            } else {
                prefetchIndexData(normalized, true).then(fresh => {
                    if (state.currentRoute === normalized && requestID === state.routeSequence && fresh !== undefined && JSON.stringify(fresh) !== JSON.stringify(providedData)) {
                        state.currentPage?.mount?.({ target: "#page-content", data: fresh });
                    }
                }).catch(() => {});
            }
        } else {
        }

        await page.mount({
            target: "#page-content",
            data: providedData
        });
        if (requestID !== state.routeSequence) return "";

        await window.UIEnhance?.waitForCriticalImages?.(content, { timeout: 2400, margin: 360 });
        if (requestID !== state.routeSequence) return "";

        if (content && changingPage) {
            requestAnimationFrame(() => requestAnimationFrame(() => content.classList.remove("is-entering")));
        }

        content?.focus({ preventScroll: true });
        window.scrollTo({ top: 0, behavior: changingPage ? "smooth" : "auto" });
        await window.UIEnhance?.nextPaint?.();
        if (requestID !== state.routeSequence) return "";

        window.dispatchEvent(
            new CustomEvent("electricredit:routechange", {
                detail: { shell: state.shell, route: normalized }
            })
        );

        return normalized;
    }

    function humanRoute(route) {
        const value = INDEX_ROUTE_ALIASES[normalizeRoute(route)] || normalizeRoute(route) || "home";
        return value.charAt(0).toUpperCase() + value.slice(1);
    }

    function cacheKey(route) { return PUBLIC_CACHE_PREFIX + route; }
    function readPublicCache(route) {
        if (state.publicData.has(route)) return state.publicData.get(route);
        try {
            const item = JSON.parse(localStorage.getItem(cacheKey(route)) || "null");
            if (!item || Date.now() - Number(item.saved_at || 0) > PUBLIC_CACHE_TTL) return undefined;
            state.publicData.set(route, item.data);
            return item.data;
        } catch (_) { return undefined; }
    }
    function writePublicCache(route, data) {
        state.publicData.set(route, data);
        try { localStorage.setItem(cacheKey(route), JSON.stringify({ saved_at: Date.now(), data })); } catch (_) {}
        warmRouteImages(route, data);
        return data;
    }

    function warmRouteImages(route, data) {
        const urls = [];
        const add = value => {
            const url = String(value || "").trim();
            if (!url || /default[-_ ]?image|placeholder/i.test(url) || urls.includes(url)) return;
            urls.push(url);
        };
        if (route === "tenant" && Array.isArray(data)) {
            data.slice(0, 10).forEach(item => add(item?.image_url || item?.imageUrl));
        } else if (route === "about" && data && typeof data === "object") {
            add(data.owner?.image_url || data.owner?.imageUrl);
            [...(data.developers || []), ...(data.admins || data.administrators || [])]
                .slice(0, 10)
                .forEach(item => add(item?.image_url || item?.imageUrl));
        }
        urls.slice(0, 10).forEach(url => {
            const image = new Image();
            image.decoding = "async";
            image.src = url;
        });
    }
    async function prefetchIndexData(route, force = false) {
        route = INDEX_ROUTE_ALIASES[normalizeRoute(route)] || normalizeRoute(route);
        const spec = INDEX_DATA_ENDPOINTS[route];
        if (!spec) return undefined;
        if (!force) {
            const cached = readPublicCache(route);
            if (cached !== undefined) return cached;
        }
        if (state.prefetching.has(route)) return state.prefetching.get(route);
        const task = window.API.get(spec.endpoint, { timeoutMs: 2500 }).then(value => writePublicCache(route, spec.pick(value))).finally(() => state.prefetching.delete(route));
        state.prefetching.set(route, task);
        return task;
    }
    function scheduleIndexPrefetch(current) {
        const priority = ["tenant", "about", "registry", "hub", "home"].filter(route => route !== current && INDEX_DATA_ENDPOINTS[route]);
        const run = () => priority.forEach((route, index) => {
            const delay = index < 2 ? index * 45 : 140 + ((index - 2) * 110);
            setTimeout(() => prefetchIndexData(route).catch(() => {}), delay);
        });
        if ("requestIdleCallback" in window) requestIdleCallback(run, { timeout: 700 });
        else setTimeout(run, 120);
    }


    function scheduleAdminWorkspacePreload(user) {
        const role = String(user?.role || "").trim().toLowerCase();
        if (!["admin", "owner", "developer"].includes(role) || state.workspacePreloadStarted) return;
        state.workspacePreloadStarted = true;
        const run = () => preloadAdminWorkspace().catch(() => {});
        if ("requestIdleCallback" in window) requestIdleCallback(run, { timeout: 1200 });
        else setTimeout(run, 180);
    }

    async function preloadAdminWorkspaceAssets() {
        const response = await fetch("admin.html", { credentials: "same-origin", cache: "force-cache" });
        if (!response.ok) return false;
        const html = await response.text();
        const doc = new DOMParser().parseFromString(html, "text/html");
        const resources = [...doc.querySelectorAll('script[src],link[rel="stylesheet"][href]')]
            .map(node => node.getAttribute("src") || node.getAttribute("href"))
            .filter(Boolean);
        await Promise.allSettled(resources.map(url => fetch(url, { credentials: "same-origin", cache: "force-cache" })));
        return true;
    }

    async function preloadAdminWorkspace() {
        await preloadAdminWorkspaceAssets();
        const [profileResult, overviewResult] = await Promise.allSettled([
            window.API.get("/admin/profile"),
            window.API.get("/admin/overview")
        ]);
        const payload = {
            saved_at: Date.now(),
            profile: profileResult.status === "fulfilled" ? profileResult.value?.profile || null : null,
            overview: overviewResult.status === "fulfilled" ? overviewResult.value || null : null
        };
        try { sessionStorage.setItem("electricredit-admin-preload-v1", JSON.stringify(payload)); } catch (_) {}
    }

    function wait(milliseconds) {
        return new Promise(resolve => {
            window.setTimeout(resolve, milliseconds);
        });
    }

    function updateNavigationState(route) {
        document.querySelectorAll("[data-index-route]").forEach(item => {
            const active = item.dataset.indexRoute === route;

            item.classList.toggle("is-active", active);

            if (item.closest("nav")) {
                if (active) {
                    item.setAttribute("aria-current", "page");
                } else {
                    item.removeAttribute("aria-current");
                }
            }
        });
    }

    function closeMobileNavigation() {
        const nav = document.getElementById("index-navigation");
        const button = document.getElementById("index-menu-toggle");

        nav?.classList.remove("is-open");
        button?.setAttribute("aria-expanded", "false");
    }

    function getRoute() {
        return state.currentRoute || getRouteFromHash();
    }

    function getRouteFromHash() {
        return normalizeRoute(
            window.location.hash.replace(/^#/, "")
        );
    }

    function normalizeRoute(route) {
        return String(route || "")
            .trim()
            .toLowerCase()
            .replace(/[^a-z0-9/_-]/g, "");
    }

    function getShell() {
        return state.shell;
    }

    function formatRole(role) {
        const value = String(role || "").trim();

        if (!value) {
            return "";
        }

        return value.charAt(0).toUpperCase() + value.slice(1).toLowerCase();
    }

    function mount(target, content) {
        const element =
            typeof target === "string"
                ? document.querySelector(target)
                : target;

        if (!element) {
            return null;
        }

        if (typeof content === "string") {
            element.innerHTML = content;
        } else if (content instanceof Node) {
            element.replaceChildren(content);
        }

        return element;
    }

    function append(target, content) {
        const element =
            typeof target === "string"
                ? document.querySelector(target)
                : target;

        if (!element) {
            return null;
        }

        if (typeof content === "string") {
            element.insertAdjacentHTML("beforeend", content);
        } else if (content instanceof Node) {
            element.appendChild(content);
        }

        return element;
    }

    function clear(target) {
        const element =
            typeof target === "string"
                ? document.querySelector(target)
                : target;

        if (element) {
            element.innerHTML = "";
        }
    }

    function escapeHTML(value) {
        return String(value ?? "")
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;")
            .replaceAll('"', "&quot;")
            .replaceAll("'", "&#039;");
    }

    function formatCurrency(value = 0) {
        return new Intl.NumberFormat("en-PH", {
            style: "currency",
            currency: "PHP",
            minimumFractionDigits: 2,
            maximumFractionDigits: 5
        }).format(Number(value));
    }

    function formatDateTime(value) {
        const date =
            value instanceof Date
                ? value
                : new Date(value);

        if (Number.isNaN(date.getTime())) {
            return "";
        }

        return new Intl.DateTimeFormat("en-PH", {
            dateStyle: "medium",
            timeStyle: "short"
        }).format(date);
    }

    return {
        initialize,
        setRoute,
        getRoute,
        getShell,
        mount,
        append,
        clear,
        escapeHTML,
        formatCurrency,
        formatDateTime,
        refreshSession,
        refreshSupportAvailability,
        preloadAdminWorkspaceAssets
    };
})();

window.ElectriCredit = ElectriCredit;

function startElectriCredit() {
    const startupWatchdog = window.setTimeout(() => {
        document.getElementById("ec-boot-loader")?.classList.add("is-leaving");
        document.body?.classList.remove("ec-boot-active");
        window.Notification?.error?.(
            "Startup is taking too long. The server may be blocked or unavailable."
        );
    }, 8000);

    ElectriCredit.initialize()
        .catch(error => {
            console.error("[ElectriCredit] Startup failed.", error);
            document.getElementById("ec-boot-loader")?.classList.add("is-leaving");
            document.body?.classList.remove("ec-boot-active");
            window.Notification?.error?.(
                "ElectriCredit could not finish loading. Check that the Go server is running, then reload."
            );
        })
        .finally(() => window.clearTimeout(startupWatchdog));
}

if (document.readyState === "loading") {
    document.addEventListener(
        "DOMContentLoaded",
        startElectriCredit,
        { once: true }
    );
} else {
    startElectriCredit();
}
