/**
 * ElectriCredit - Public Home Structure
 */

const HomeStructure = (() => {
    function render(data = {}) {
        const {
            greeting = getGreeting(),
            pricePerKWh = null,
            registryCount = 0,
            hubCount = 0,
            userCount = 0
        } = data;

        return `
            <section class="ec-section ec-index-home">
                <div class="ec-index-home-hero">
                    <div class="ec-index-home-copy">
                        <span class="ec-badge ec-index-home-greeting">
                            ${escapeHTML(greeting)}
                        </span>

                        <h1 class="ec-index-home-title">
                            Welcome to ElectriCredit
                        </h1>

                        <p class="ec-index-home-lead">
                            A prepaid electricity management system designed to
                            make electricity usage, credits, and shared rental
                            billing easier to understand.
                        </p>
                    </div>

                    <div class="ec-card ec-index-price-card">
                        <div class="ec-card-body">
                            <span class="ec-stat-label">
                                Current Electricity Price
                            </span>

                            <strong
                                class="ec-index-price"
                                id="home-price-value"
                            >
                                ${formatPrice(pricePerKWh)}
                            </strong>

                            <span class="ec-stat-meta">
                                per kilowatt-hour
                            </span>
                        </div>
                    </div>
                </div>
            </section>

            <section class="ec-section">
                <div class="ec-section-header">
                    <div>
                        <h2 class="ec-section-title">
                            ElectriCredit Overview
                        </h2>
                        <p class="ec-section-description">
                            View registered modules and tenants in the system.
                        </p>
                    </div>
                </div>

                <div class="ec-grid-auto">
                    ${summaryCard(
                        "Registry Stations",
                        registryCount,
                        "registry",
                        "View Registry"
                    )}

                    ${summaryCard(
                        "Hub Modules",
                        hubCount,
                        "hub",
                        "View Hubs"
                    )}

                    ${summaryCard(
                        "Registered Tenants",
                        userCount,
                        "tenant",
                        "View Tenants"
                    )}
                </div>
            </section>

            <section class="ec-section">
                <div class="ec-panel ec-index-home-info">
                    <div>
                        <span class="ec-index-home-info-label">
                            PREPAID ELECTRICITY
                        </span>

                        <h2 class="ec-section-title">
                            Electricity information in one place.
                        </h2>

                        <p class="ec-section-description">
                            Browse available Registry Stations and Hub Modules,
                            view registered tenants, and learn more about
                            ElectriCredit.
                        </p>
                    </div>

                    <button
                        class="ec-button ec-button-accent"
                        type="button"
                        data-home-route="about"
                    >
                        About ElectriCredit
                    </button>
                </div>
            </section>
        `;
    }

    function summaryCard(title, count, route, action) {
        return `
            <article class="ec-card ec-index-summary-card">
                <div class="ec-card-body">
                    <div class="ec-stat">
                        <span class="ec-stat-label">
                            ${escapeHTML(title)}
                        </span>

                        <strong class="ec-stat-value">
                            ${formatCount(count)}
                        </strong>

                        <span class="ec-stat-meta">
                            Registered
                        </span>
                    </div>
                </div>

                <div class="ec-card-footer">
                    <button
                        class="ec-button ec-button-secondary ec-button-sm"
                        type="button"
                        data-home-route="${escapeHTML(route)}"
                    >
                        ${escapeHTML(action)}
                    </button>
                </div>
            </article>
        `;
    }

    function getGreeting() {
        const hour = new Date().getHours();

        if (hour < 12) {
            return "Good morning";
        }

        if (hour < 18) {
            return "Good afternoon";
        }

        return "Good evening";
    }

    function formatPrice(value) {
        if (
            value === null ||
            value === undefined ||
            value === "" ||
            Number.isNaN(Number(value))
        ) {
            return "₱— / kWh";
        }

        return `${formatPeso(value)} / kWh`;
    }

    function formatPeso(value) {
        return new Intl.NumberFormat("en-PH", {
            style: "currency",
            currency: "PHP",
            minimumFractionDigits: 2,
            maximumFractionDigits: 5
        }).format(Number(value));
    }

    function formatCount(value) {
        const count = Number(value);

        return Number.isFinite(count)
            ? count.toLocaleString("en-PH")
            : "0";
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

window.HomeStructure = HomeStructure;
