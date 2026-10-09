/**
 * ElectriCredit - Authentication State
 * The real session token stays in an HTTP-only cookie owned by the Go server.
 */

const Auth = (() => {
    let currentUser = null;

    async function loadSession() {
        try {
            const response = await window.API.get("/auth/session", { timeoutMs: 2500 });
            currentUser = response?.user || response || null;
            return currentUser;
        } catch (error) {
            if (error?.status !== 401) {
                console.warn("[ElectriCredit Auth] Session check unavailable.");
            }

            currentUser = null;
            return null;
        }
    }

    function getUser() {
        return currentUser;
    }

    function isAuthenticated() {
        return Boolean(currentUser);
    }

    function hasRole(...roles) {
        const role = String(currentUser?.role || "").toLowerCase();
        return roles.map(value => String(value).toLowerCase()).includes(role);
    }

    async function login(credentials) {
        const response = await window.API.post("/auth/login", credentials);
        currentUser = response?.user || response || null;
        return response;
    }

    async function logout() {
        try {
            await window.API.post("/auth/logout", {});
        } finally {
            currentUser = null;
        }
    }


    async function getRecoveryStatus() {
        return window.API.get("/auth/recovery/status");
    }

    async function requestRecoveryOTP(email) {
        return window.API.post("/auth/recovery/request", { email });
    }

    async function verifyRecoveryOTP(email, otp) {
        return window.API.post("/auth/recovery/verify", { email, otp });
    }

    async function resetRecoveredPassword(payload) {
        return window.API.post("/auth/recovery/reset", payload);
    }

    function canAccessDeveloper() {
        return hasRole("developer");
    }

    function applyRoleAccess(root = document) {
        const canUseDeveloper = canAccessDeveloper();
        const controls = root.querySelectorAll?.(
            '[data-developer-access], [data-route="developer"], a[href$="developer.html"]'
        ) || [];

        controls.forEach(control => {
            const shouldDisable = !canUseDeveloper;
            control.classList.toggle("ec-role-disabled", shouldDisable);
            control.setAttribute("aria-disabled", String(shouldDisable));

            if ("disabled" in control) {
                control.disabled = shouldDisable;
            }

            if (shouldDisable) {
                control.setAttribute("tabindex", "-1");
            } else {
                control.removeAttribute("tabindex");
            }
        });
    }

    function redirectByRole() {
        const role = String(currentUser?.role || "").toLowerCase();

        if (role === "developer" || role === "owner" || role === "admin") {
            window.location.href = "admin.html";
            return;
        }

        window.location.href = "index.html";
    }

    return {
        loadSession,
        getUser,
        isAuthenticated,
        hasRole,
        login,
        logout,
        getRecoveryStatus,
        requestRecoveryOTP,
        verifyRecoveryOTP,
        resetRecoveredPassword,
        canAccessDeveloper,
        applyRoleAccess,
        redirectByRole
    };
})();

window.Auth = Auth;
