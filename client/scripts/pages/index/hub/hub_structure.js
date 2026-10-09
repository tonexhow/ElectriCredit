/**
 * ElectriCredit - Public Hub Structure
 */

const HubStructure = (() => {
    function render(hubs = []) {
        return `
            <section class="ec-section ec-index-hub">
                <div class="ec-page-header">
                    <div>
                        <span class="ec-badge ec-index-hub-eyebrow">
                            Hub Modules
                        </span>

                        <h1 class="ec-page-title">
                            Registered Hub Modules
                        </h1>

                        <p class="ec-page-description">
                            View ElectriCredit Hub Modules, their assigned
                            locations, and current availability.
                        </p>
                    </div>

                    <div class="ec-index-hub-count">
                        <span class="ec-stat-label">Registered</span>
                        <strong class="ec-stat-value">
                            ${formatCount(hubs.length)}
                        </strong>
                    </div>
                </div>

                ${hubs.length
                    ? renderHubGrid(hubs)
                    : renderEmptyState()}
            </section>
        `;
    }

    function renderHubGrid(hubs) {
        return `
            <div class="ec-index-hub-grid">
                ${hubs.map(renderHubCard).join("")}
            </div>
        `;
    }

    function renderHubCard(hub) {
        const status = normalizeStatus(hub.status);

        return `
            <article class="ec-card ec-index-hub-card">
                <div class="ec-card-header">
                    <div class="ec-row-between">
                        <div class="ec-index-hub-heading">
                            <span class="ec-index-hub-icon" aria-hidden="true">
                                ${moduleIcon("hub")}
                            </span>
                            <div>
                                <span class="ec-index-hub-type">Hub</span>
                                <h2 class="ec-card-title">
                                    ${escapeHTML(
                                        hub.name ||
                                        hub.display_name ||
                                        "Hub Module"
                                    )}
                                </h2>
                            </div>
                        </div>

                        <span class="ec-badge ${getStatusClass(status)}">
                            <span
                                class="ec-status-dot"
                                data-hub-status="${escapeHTML(status)}"
                            ></span>
                            ${escapeHTML(getStatusLabel(status))}
                        </span>
                    </div>
                </div>

                <div class="ec-card-body">
                    <dl class="ec-index-hub-details">
                        <div>
                            <dt>Hub ID</dt>
                            <dd>
                                ${escapeHTML(
                                    hub.public_id ||
                                    hub.hub_id ||
                                    hub.id ||
                                    "—"
                                )}
                            </dd>
                        </div>

                        <div>
                            <dt>Location</dt>
                            <dd>
                                ${escapeHTML(
                                    hub.location ||
                                    hub.name_location ||
                                    "Not specified"
                                )}
                            </dd>
                        </div>

                        ${status === "in-use" ? `
                        <div class="ec-index-hub-session">
                            <dt>${moduleIcon("card")} In use by</dt>
                            <dd>Card ID ${escapeHTML(hub.active_card_id || "—")}</dd>
                        </div>` : ""}
                    </dl>
                </div>
            </article>
        `;
    }

    function renderEmptyState() {
        return `
            <div class="ec-panel ec-index-hub-empty">
                <div class="ec-index-hub-empty-mark" aria-hidden="true">
                    ${moduleIcon("hub")}
                </div>

                <div>
                    <h2 class="ec-section-title">
                        No Hub Modules to display
                    </h2>

                    <p class="ec-section-description">
                        Registered Hub Modules will appear here when
                        records become available.
                    </p>
                </div>
            </div>
        `;
    }

    function normalizeStatus(value) {
        const status = String(value || "idle")
            .trim()
            .toLowerCase()
            .replaceAll("_", "-")
            .replaceAll(" ", "-");

        if (["in-use", "inused", "busy", "active"].includes(status)) {
            return "in-use";
        }

        if (["disabled", "inactive"].includes(status)) {
            return "disabled";
        }

        return "idle";
    }

    function getStatusLabel(status) {
        switch (status) {
            case "in-use":
                return "In Use";
            case "disabled":
                return "Disabled";
            default:
                return "Idle";
        }
    }

    function getStatusClass(status) {
        switch (status) {
            case "in-use":
                return "ec-badge-warning";
            case "disabled":
                return "ec-badge-danger";
            default:
                return "ec-badge-success";
        }
    }

    function moduleIcon(name) {
        const paths = {
            hub: '<rect x="5" y="5" width="14" height="14" rx="2"/><path d="M9 9h6v6H9zM9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/>',
            card: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 9h18M7 15h4"/>'
        };
        return `<svg class="ec-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${paths[name] || paths.hub}</svg>`;
    }

    function formatCount(value) {
        return Number(value || 0).toLocaleString("en-PH");
    }

    function escapeHTML(value) {
        if (window.ElectriCredit?.escapeHTML) {
            return window.ElectriCredit.escapeHTML(value);
        }

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

window.HubStructure = HubStructure;
