/**
 * ElectriCredit - Public Registry Structure
 */

const RegistryStructure = (() => {
    function render(registries = []) {
        return `
            <section class="ec-section ec-index-registry">
                <div class="ec-page-header">
                    <div>
                        <span class="ec-badge ec-index-registry-eyebrow">
                            Registry Stations
                        </span>

                        <h1 class="ec-page-title">
                            Registered Registry Modules
                        </h1>

                        <p class="ec-page-description">
                            View ElectriCredit Registry Stations and their
                            current availability.
                        </p>
                    </div>

                    <div class="ec-index-registry-count">
                        <span class="ec-stat-label">Registered</span>
                        <strong class="ec-stat-value">
                            ${formatCount(registries.length)}
                        </strong>
                    </div>
                </div>

                ${registries.length
                    ? renderRegistryGrid(registries)
                    : renderEmptyState()}
            </section>
        `;
    }

    function renderRegistryGrid(registries) {
        return `
            <div class="ec-index-module-grid">
                ${registries.map(renderRegistryCard).join("")}
            </div>
        `;
    }

    function renderRegistryCard(registry) {
        const status = normalizeStatus(registry.status);
        const statusLabel = getStatusLabel(status);

        return `
            <article class="ec-card ec-index-module-card">
                <div class="ec-card-header">
                    <div class="ec-row-between">
                        <div class="ec-index-module-heading">
                            <span class="ec-index-module-icon" aria-hidden="true">
                                ${moduleIcon("registry")}
                            </span>
                            <div>
                                <span class="ec-index-module-type">Registry</span>
                                <h2 class="ec-card-title">
                                    ${escapeHTML(
                                        registry.name ||
                                        registry.display_name ||
                                        "Registry Station"
                                    )}
                                </h2>
                            </div>
                        </div>

                        <span class="ec-badge ${getStatusClass(status)}">
                            <span
                                class="ec-status-dot"
                                data-module-status="${escapeHTML(status)}"
                            ></span>
                            ${escapeHTML(statusLabel)}
                        </span>
                    </div>
                </div>

                <div class="ec-card-body">
                    <dl class="ec-index-module-details">
                        <div>
                            <dt>Registry ID</dt>
                            <dd>
                                ${escapeHTML(
                                    registry.public_id ||
                                    registry.registry_id ||
                                    registry.id ||
                                    "—"
                                )}
                            </dd>
                        </div>

                        <div>
                            <dt>Location</dt>
                            <dd>
                                ${escapeHTML(
                                    registry.location ||
                                    registry.name_location ||
                                    "Not specified"
                                )}
                            </dd>
                        </div>

                        ${status === "in-use" ? `
                        <div class="ec-index-module-session">
                            <dt>${moduleIcon("card")} In use by</dt>
                            <dd>Card ID ${escapeHTML(registry.active_card_id || "—")}</dd>
                        </div>` : ""}
                    </dl>
                </div>
            </article>
        `;
    }

    function renderEmptyState() {
        return `
            <div class="ec-panel ec-index-registry-empty">
                <div class="ec-index-registry-empty-mark" aria-hidden="true">
                    ${moduleIcon("registry")}
                </div>

                <div>
                    <h2 class="ec-section-title">
                        No Registry Stations to display
                    </h2>

                    <p class="ec-section-description">
                        Registered Registry Stations will appear here when
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
            registry: '<path d="M5 3H3v4M19 3h2v4M5 21H3v-4M19 21h2v-4"/><rect x="7" y="7" width="10" height="10" rx="2"/><path d="M10 10h4v4h-4z"/>',
            card: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 9h18M7 15h4"/>'
        };
        return `<svg class="ec-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${paths[name] || paths.registry}</svg>`;
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

window.RegistryStructure = RegistryStructure;
