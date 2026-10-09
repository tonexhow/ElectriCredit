/**
 * ElectriCredit - Global Sidebar
 * Shared sidebar renderer for Admin and Developer shells.
 */

const Sidebar = (() => {
    function render(items = [], options = {}) {
        const {
            active = "",
            title = "ElectriCredit",
            subtitle = "",
            footer = ""
        } = options;

        const links = items.map(item => {
            if (item.type === "separator") {
                return `<div class="ec-sidebar-separator"></div>`;
            }

            if (item.type === "label") {
                return `
                    <div class="ec-sidebar-label">
                        ${escapeHTML(item.label || "")}
                    </div>
                `;
            }

            const isActive = item.id === active;

            return `
                <button
                    class="ec-sidebar-item ${isActive ? "is-active" : ""}"
                    type="button"
                    data-route="${escapeHTML(item.id || "")}"
                    ${item.disabled ? "disabled" : ""}
                >
                    ${item.icon ? `<span class="ec-sidebar-icon">${item.icon}</span>` : ""}
                    <span class="ec-sidebar-text">${escapeHTML(item.label || "")}</span>
                    ${item.badge ? `<span class="ec-sidebar-badge" data-route-badge="${escapeHTML(item.id || "")}" hidden></span>` : ""}
                </button>
            `;
        }).join("");

        return `
            <aside class="ec-sidebar" id="ec-sidebar">
                <div class="ec-sidebar-head">
                    <a class="ec-sidebar-brand" href="index.html">
                        <strong>${escapeHTML(title)}</strong>
                        ${subtitle ? `<span>${escapeHTML(subtitle)}</span>` : ""}
                    </a>

                    <button
                        class="ec-sidebar-close"
                        id="sidebar-close"
                        type="button"
                        aria-label="Close navigation"
                    >
                        &times;
                    </button>
                </div>

                <nav class="ec-sidebar-nav" aria-label="Sidebar navigation">
                    ${links}
                </nav>
                ${footer ? `<div class="ec-sidebar-footer">${footer}</div>` : ""}
            </aside>

            <div class="ec-sidebar-backdrop" id="ec-sidebar-backdrop"></div>
        `;
    }

    function bind(container, onNavigate) {
        const root =
            typeof container === "string"
                ? document.querySelector(container)
                : container;

        if (!root) {
            return;
        }

        root.addEventListener("click", event => {
            const button = event.target.closest("[data-route]");

            if (!button || button.disabled) {
                return;
            }

            if (typeof onNavigate === "function") {
                onNavigate(button.dataset.route, button);
            }

            close();
        });

        document
            .getElementById("sidebar-toggle")
            ?.addEventListener("click", toggle);

        document
            .getElementById("sidebar-close")
            ?.addEventListener("click", close);

        document
            .getElementById("ec-sidebar-backdrop")
            ?.addEventListener("click", close);
    }

    function open() {
        document.getElementById("ec-sidebar")?.classList.add("is-open");
        document.getElementById("ec-sidebar-backdrop")?.classList.add("is-active");
        document.body.classList.add("sidebar-open");
    }

    function close() {
        document.getElementById("ec-sidebar")?.classList.remove("is-open");
        document.getElementById("ec-sidebar-backdrop")?.classList.remove("is-active");
        document.body.classList.remove("sidebar-open");
    }

    function toggle() {
        const sidebar = document.getElementById("ec-sidebar");

        if (!sidebar) {
            return;
        }

        sidebar.classList.contains("is-open")
            ? close()
            : open();
    }

    function setActive(container, route) {
        const root =
            typeof container === "string"
                ? document.querySelector(container)
                : container;

        if (!root) {
            return;
        }

        root.querySelectorAll("[data-route]").forEach(item => {
            item.classList.toggle(
                "is-active",
                item.dataset.route === route
            );
        });
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
        render,
        bind,
        open,
        close,
        toggle,
        setActive
    };
})();

window.Sidebar = Sidebar;
