/**
 * ElectriCredit - Global Notification Manager
 * Handles toast-style UI notifications.
 */

const Notification = (() => {
    const DEFAULT_DURATION = 4000;

    function ensureContainer() {
        let container = document.getElementById("ec-notification-container");

        if (!container) {
            container = document.createElement("div");
            container.id = "ec-notification-container";
            container.className = "ec-notification-container";
            container.setAttribute("aria-live", "polite");
            container.setAttribute("aria-atomic", "true");
            document.body.appendChild(container);
        }

        return container;
    }

    function show(message, options = {}) {
        const {
            type = "info",
            title = "",
            duration = DEFAULT_DURATION,
            persistent = false
        } = options;

        const container = ensureContainer();
        const toast = document.createElement("div");

        toast.className = `ec-notification ec-notification-${normalizeType(type)}`;
        toast.setAttribute("role", type === "error" ? "alert" : "status");

        toast.innerHTML = `
            <div class="ec-notification-content">
                ${title ? `<strong class="ec-notification-title">${escapeHTML(title)}</strong>` : ""}
                <div class="ec-notification-message">${escapeHTML(message)}</div>
            </div>

            <button
                class="ec-notification-close"
                type="button"
                aria-label="Dismiss notification"
            >
                &times;
            </button>
        `;

        container.appendChild(toast);

        toast
            .querySelector(".ec-notification-close")
            ?.addEventListener("click", () => remove(toast));

        requestAnimationFrame(() => {
            toast.classList.add("is-visible");
        });

        if (!persistent && duration > 0) {
            window.setTimeout(() => {
                remove(toast);
            }, duration);
        }

        return toast;
    }

    function remove(toast) {
        if (!toast || !toast.isConnected) {
            return;
        }

        toast.classList.remove("is-visible");

        window.setTimeout(() => {
            toast.remove();
        }, 200);
    }

    function clear() {
        document
            .querySelectorAll(".ec-notification")
            .forEach(remove);
    }

    function success(message, options = {}) {
        return show(message, { ...options, type: "success" });
    }

    function info(message, options = {}) {
        return show(message, { ...options, type: "info" });
    }

    function warning(message, options = {}) {
        return show(message, { ...options, type: "warning" });
    }

    function error(message, options = {}) {
        return show(message, { ...options, type: "error" });
    }

    function normalizeType(type) {
        return ["success", "info", "warning", "error"].includes(type)
            ? type
            : "info";
    }

    function escapeHTML(value) {
        return String(value)
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;")
            .replaceAll('"', "&quot;")
            .replaceAll("'", "&#039;");
    }

    return {
        show,
        success,
        info,
        warning,
        error,
        clear
    };
})();

window.Notification = Notification;
