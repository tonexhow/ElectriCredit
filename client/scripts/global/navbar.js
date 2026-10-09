/**
 * ElectriCredit - Global Navbar
 * Reusable top/secondary navigation renderer.
 */

const Navbar = (() => {
    function render(items = [], options = {}) {
        const {
            active = "",
            className = ""
        } = options;

        const links = items.map(item => {
            const isActive = item.id === active;

            return `
                <button
                    class="ec-navbar-item ${isActive ? "is-active" : ""}"
                    type="button"
                    data-route="${escapeHTML(item.id || "")}"
                    ${item.disabled ? "disabled" : ""}
                >
                    ${item.icon ? `<span class="ec-navbar-icon">${item.icon}</span>` : ""}
                    <span>${escapeHTML(item.label || "")}</span>
                </button>
            `;
        }).join("");

        return `
            <nav class="ec-navbar ${escapeHTML(className)}" aria-label="Page navigation">
                <div class="ec-navbar-content">
                    ${links}
                </div>
            </nav>
        `;
    }

    function bind(container, onNavigate) {
        const root =
            typeof container === "string"
                ? document.querySelector(container)
                : container;

        if (!root || typeof onNavigate !== "function") {
            return;
        }

        root.addEventListener("click", event => {
            const button = event.target.closest("[data-route]");

            if (!button || button.disabled) {
                return;
            }

            onNavigate(button.dataset.route, button);
        });
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
        setActive
    };
})();

window.Navbar = Navbar;
