/**
 * ElectriCredit - Global Header
 */

const Header = (() => {
    function render(options = {}) {
        const {
            title = "ElectriCredit",
            subtitle = "",
            showMenu = false,
            showProfile = true
        } = options;

        return `
            <header class="ec-header">
                <div class="ec-header-left">
                    ${
                        showMenu
                            ? `
                                <button
                                    class="ec-header-menu"
                                    id="sidebar-toggle"
                                    type="button"
                                    aria-label="Open navigation"
                                >
                                    ☰
                                </button>
                            `
                            : ""
                    }

                    <div class="ec-header-brand">
                        <a href="index.html" class="ec-brand">
                            ElectriCredit
                        </a>

                        ${
                            subtitle
                                ? `<span class="ec-header-subtitle">${escapeHTML(subtitle)}</span>`
                                : ""
                        }
                    </div>
                </div>

                <div class="ec-header-center">
                    <span class="ec-page-title">
                        ${escapeHTML(title)}
                    </span>
                </div>

                <div class="ec-header-actions">
                    <button
                        class="ec-header-action"
                        id="theme-toggle"
                        type="button"
                        aria-label="Change theme"
                    >
                        ◐
                    </button>

                    ${
                        showProfile
                            ? `
                                <button
                                    class="ec-header-action ec-profile-button"
                                    id="profile-button"
                                    type="button"
                                    aria-label="Open profile"
                                >
                                    <span class="ec-profile-avatar">
                                        ${getInitial()}
                                    </span>
                                </button>
                            `
                            : ""
                    }
                </div>
            </header>
        `;
    }

    function getInitial() {
        const user = window.Auth?.getUser?.();

        const name =
            user?.first_name ||
            user?.name ||
            "";

        return escapeHTML(
            name ? name.charAt(0).toUpperCase() : "E"
        );
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
        render
    };
})();

window.Header = Header;