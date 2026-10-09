/**
 * ElectriCredit - Public About Structure
 */

const AboutStructure = (() => {
    function render(data = {}) {
        const owner = data.owner || null;
        const developers = Array.isArray(data.developers) ? data.developers : [];
        const admins = Array.isArray(data.admins) ? data.admins : [];

        return `
            <section class="ec-section ec-index-about">
                <div class="ec-page-header ec-index-about-header">
                    <div class="ec-index-about-copy">
                        <span class="ec-badge ec-index-about-eyebrow">
                            About ElectriCredit
                        </span>

                        <h1 class="ec-page-title ec-index-about-title">
                            Fairer prepaid electricity management for rental spaces.
                        </h1>

                        <p class="ec-page-description ec-index-about-lead">
                            ElectriCredit helps property owners and tenants monitor
                            electricity usage, manage prepaid credits, and support
                            clearer billing in boarding houses, dormitories,
                            apartments, rental rooms, and similar spaces.
                        </p>
                    </div>

                    <aside class="ec-panel ec-index-about-purpose" aria-label="Project purpose">
                        <span class="ec-stat-label">Capstone Focus</span>
                        <strong>Fair usage. Clear credits.</strong>
                        <p>
                            A practical prepaid electricity experience designed
                            around shared rental spaces.
                        </p>
                    </aside>
                </div>
            </section>

            <section class="ec-section">
                <div class="ec-section-header">
                    <div>
                        <h2 class="ec-section-title">What ElectriCredit does</h2>
                        <p class="ec-section-description">
                            A simple system focused on electricity access, monitoring,
                            credits, and fairer usage management.
                        </p>
                    </div>
                </div>

                <div class="ec-grid-auto">
                    ${aboutCard(
                        "Prepaid Access",
                        "Registered RFID cards are used to request electricity access through ElectriCredit Hub Modules."
                    )}

                    ${aboutCard(
                        "Usage Monitoring",
                        "Electricity consumption is measured during active use so credits can reflect actual energy usage."
                    )}

                    ${aboutCard(
                        "Credit Management",
                        "Tenants can maintain electricity credits and add balance through supported Registry Station top-up methods."
                    )}

                    ${aboutCard(
                        "Rental-Friendly",
                        "The system is intended for small rental properties where electricity needs to be managed per tenant."
                    )}
                </div>
            </section>

            <section class="ec-section">
                <div class="ec-section-header">
                    <div>
                        <h2 class="ec-section-title">How it works</h2>
                        <p class="ec-section-description">
                            ElectriCredit brings together tenants, RFID cards,
                            Registry Stations, and Hub Modules in one coordinated system.
                        </p>
                    </div>
                </div>

                <div class="ec-index-about-flow">
                    ${flowItem("01", "Register", "Tenants and RFID cards are registered in ElectriCredit.")}
                    ${flowItem("02", "Top Up", "Electricity credits can be added through an authorized Registry Station.")}
                    ${flowItem("03", "Access", "A registered RFID card is presented to a Hub Module.")}
                    ${flowItem("04", "Use", "The Hub allows electricity while consumption is measured.")}
                    ${flowItem("05", "Monitor", "Usage and remaining credit can be reviewed through ElectriCredit.")}
                </div>
            </section>

            <section class="ec-section">
                <div class="ec-section-header">
                    <div>
                        <h2 class="ec-section-title">Project Team</h2>
                        <p class="ec-section-description">
                            People responsible for the ownership, development,
                            and administration of ElectriCredit.
                        </p>
                    </div>
                </div>

                <div class="ec-index-about-team-grid">
                    ${owner ? renderOwner(owner) : renderEmptyTeamGroup("Owner", "No Owner is set yet.")}
                    ${renderTeamGroup("Developers", developers, "Developer", "No Developer information is available yet.")}
                    ${renderTeamGroup("Administrators", admins, "Administrator", "No Administrator is set yet.")}
                </div>
            </section>

            <section class="ec-section">
                <div class="ec-panel ec-index-about-final">
                    <div>
                        <span class="ec-index-about-final-label">
                            ELECTRICREDIT
                        </span>

                        <h2 class="ec-section-title">
                            Designed for practical electricity management.
                        </h2>

                        <p class="ec-section-description">
                            Simple for tenants to understand and easier for property
                            managers to oversee.
                        </p>
                    </div>

                    <button
                        class="ec-button ec-button-primary"
                        type="button"
                        data-about-route="home"
                    >
                        Back to Home
                    </button>
                </div>
            </section>
        `;
    }

    function aboutCard(title, description) {
        return `
            <article class="ec-card ec-index-about-card">
                <div class="ec-card-body">
                    <h3 class="ec-card-title">${escapeHTML(title)}</h3>
                    <p class="ec-card-description">${escapeHTML(description)}</p>
                </div>
            </article>
        `;
    }

    function flowItem(number, title, description) {
        return `
            <article class="ec-panel ec-index-about-flow-item">
                <span class="ec-index-about-flow-number">
                    ${escapeHTML(number)}
                </span>

                <div>
                    <h3>${escapeHTML(title)}</h3>
                    <p>${escapeHTML(description)}</p>
                </div>
            </article>
        `;
    }

    function renderOwner(owner) {
        const name = owner.name || "Team Member";
        const role = formalRole(owner.role || "Owner");
        const image = owner.image_url || "assets/default-image.png";

        return `
            <article class="ec-card ec-index-about-team-card ec-index-about-owner-card">
                <div class="ec-card-body ec-index-about-owner-body">
                    <img
                        class="ec-index-about-avatar"
                        src="${escapeHTML(image)}"
                        alt=""
                        loading="lazy"
                        onerror="this.onerror=null;this.src='assets/default-image.png';"
                    >

                    <div>
                        <span class="ec-index-about-team-label">Owner</span>
                        <h3 class="ec-card-title">${escapeHTML(name)}</h3>
                        <p class="ec-card-description">${escapeHTML(role)}</p>
                    </div>
                </div>
            </article>
        `;
    }

    function renderEmptyTeamGroup(title, message) {
        return `
            <article class="ec-card ec-index-about-team-card">
                <div class="ec-card-header">
                    <h3 class="ec-card-title">${escapeHTML(title)}</h3>
                </div>
                <div class="ec-card-body">
                    <p class="ec-card-description">${escapeHTML(message)}</p>
                </div>
            </article>
        `;
    }

    function renderTeamGroup(title, members, fallbackRole, emptyMessage = "Team information will appear here when available.") {
        return `
            <article class="ec-card ec-index-about-team-card">
                <div class="ec-card-header">
                    <h3 class="ec-card-title">${escapeHTML(title)}</h3>
                </div>

                <div class="ec-card-body">
                    ${
                        members.length
                            ? `<div class="ec-index-about-member-list">
                                ${members.map(member => renderMember(member, fallbackRole)).join("")}
                               </div>`
                            : `<p class="ec-card-description">
                                ${escapeHTML(emptyMessage)}
                               </p>`
                    }
                </div>
            </article>
        `;
    }

    function renderMember(member, fallbackRole) {
        const image = member.image_url || "assets/default-image.png";

        return `
            <div class="ec-index-about-member">
                <img
                    class="ec-index-about-avatar ec-index-about-avatar-sm"
                    src="${escapeHTML(image)}"
                    alt=""
                    loading="lazy"
                    onerror="this.onerror=null;this.src='assets/default-image.png';"
                >

                <div class="ec-index-about-member-copy">
                    <strong>${escapeHTML(member.name || "Team Member")}</strong>
                    <span>${escapeHTML(formalRole(member.role || fallbackRole))}</span>
                </div>
            </div>
        `;
    }

    function formalRole(value) {
        const role = String(value || "").trim().toLowerCase();
        if (role === "owner") return "Owner";
        if (role === "developer") return "Developer";
        if (role === "admin" || role === "administrator") return "Administrator";
        return value || "Team Member";
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

window.AboutStructure = AboutStructure;
