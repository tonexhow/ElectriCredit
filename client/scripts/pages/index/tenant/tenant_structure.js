/**
 * ElectriCredit - Public Tenant Structure
 */

const TenantStructure = (() => {
    function render(users = []) {
        return `
            <section class="ec-section ec-index-users">
                <div class="ec-page-header">
                    <div>
                        <span class="ec-badge ec-index-users-eyebrow">
                            Registered Tenants
                        </span>

                        <h1 class="ec-page-title">ElectriCredit Tenants</h1>

                        <p class="ec-page-description">
                            Browse registered tenants and view permitted account
                            and RFID card information.
                        </p>
                    </div>

                    <div class="ec-index-users-count">
                        <span class="ec-stat-label">Registered</span>
                        <strong class="ec-stat-value">
                            ${formatCount(users.length)}
                        </strong>
                    </div>
                </div>

                ${users.length ? renderTable(users) : renderEmptyState()}
            </section>

            <div id="users-modal-root"></div>
        `;
    }

    function renderTable(users) {
        return `
            <div class="ec-card ec-index-users-table-card">
                <div class="ec-index-users-table-wrap">
                    <table class="ec-table ec-index-users-table">
                        <thead>
                            <tr>
                                <th>Tenant</th>
                                <th>Tenant ID</th>
                                <th>Card IDs</th>
                                <th>Status</th>
                                <th class="ec-index-users-action-column">
                                    Action
                                </th>
                            </tr>
                        </thead>

                        <tbody>
                            ${users.map(renderRow).join("")}
                        </tbody>
                    </table>
                </div>
            </div>
        `;
    }

    function renderRow(user) {
        const status = normalizeStatus(user.status);

        return `
            <tr>
                <td data-label="Tenant">
                    <div class="ec-index-user-cell">
                        <img
                            class="ec-index-user-avatar"
                            src="${escapeHTML(user.image_url || "assets/default-image.png")}"
                            alt=""
                            loading="lazy"
                            onerror="this.onerror=null;this.src='assets/default-image.png';"
                        >

                        <div class="ec-index-user-primary">
                            <strong>${escapeHTML(maskName(user.name))}</strong>
                            <span>
                                ${escapeHTML(maskEmail(user.email))}
                            </span>
                        </div>
                    </div>
                </td>

                <td data-label="Tenant ID"><strong class="ec-index-id-code">${escapeHTML(user.id || "—")}</strong></td>

                <td data-label="Card IDs">
                    ${Array.isArray(user.cards) && user.cards.length
                        ? `<div class="ec-index-card-code-list">${user.cards.slice(0, 2).map(card => `<span>${escapeHTML(card.id || "—")}</span>`).join("")}${user.cards.length > 2 ? `<small>+${user.cards.length - 2} more</small>` : ""}</div>`
                        : `<span class="ec-text-muted">No card</span>`}
                </td>

                <td data-label="Status">
                    <span class="ec-badge ${getStatusClass(status)}">
                        ${escapeHTML(getStatusLabel(status))}
                    </span>
                </td>

                <td class="ec-index-users-action-column" data-label="Action">
                    <button
                        class="ec-button ec-button-secondary ec-button-sm"
                        type="button"
                        data-user-view="${escapeHTML(user.id)}"
                    >
                        View More
                    </button>
                </td>
            </tr>
        `;
    }

    function renderUserModal(user) {
        if (!user) return "";
        const cards = Array.isArray(user.cards) ? user.cards : [];
        const status = normalizeStatus(user.status);

        return `
            <div class="ec-index-user-modal-backdrop" id="index-user-modal" role="dialog" aria-modal="true" aria-labelledby="index-user-modal-title">
                <div class="ec-card ec-index-user-modal">
                    <button class="ec-index-user-modal-close" type="button" data-user-modal-close aria-label="Close tenant information">×</button>

                    <div class="ec-index-user-modal-hero">
                        <img class="ec-index-user-modal-photo" src="${escapeHTML(user.image_url || "assets/default-image.png")}" alt="" onerror="this.onerror=null;this.src='assets/default-image.png';">
                        <div class="ec-index-user-modal-title-wrap">
                            <span class="ec-index-user-modal-label">REGISTERED TENANT</span>
                            <h2 class="ec-card-title" id="index-user-modal-title">${escapeHTML(maskName(user.name))}</h2>
                            <div class="ec-index-user-modal-meta">
                                <span class="ec-index-id-code">${escapeHTML(user.id || "—")}</span>
                                <span class="ec-badge ${getStatusClass(status)}">${escapeHTML(getStatusLabel(status))}</span>
                            </div>
                        </div>
                    </div>

                    <div class="ec-card-body ec-index-user-modal-scroll">
                        <section class="ec-index-user-modal-section">
                            <div class="ec-index-user-section-head"><div><span class="ec-index-user-section-kicker">Account</span><h3>Tenant information</h3></div></div>
                            <dl class="ec-index-user-details ec-index-user-details-grid">
                                ${detail("Tenant ID", user.id || "—")}
                                ${detail("Name", maskName(user.name))}
                                ${detail("Email", maskEmail(user.email))}
                                ${detail("Phone", maskPhone(user.phone))}
                            </dl>
                        </section>

                        <section class="ec-index-user-modal-section">
                            <div class="ec-index-user-section-head"><div><span class="ec-index-user-section-kicker">RFID access</span><h3>Registered cards</h3></div><span class="ec-badge">${formatCount(cards.length)}</span></div>
                            ${cards.length
                                ? `<div class="ec-index-user-card-grid">${cards.map(renderRFIDCard).join("")}</div>`
                                : `<div class="ec-panel ec-index-user-no-cards">No registered cards to display.</div>`}
                        </section>
                    </div>
                </div>
            </div>`;
    }

    function renderRFIDCard(card) {
        const status = normalizeCardStatus(card.status);

        return `
            <article class="ec-panel ec-index-rfid-card">
                <div class="ec-row-between">
                    <strong>
                        ${escapeHTML(
                            card.name ||
                            card.label ||
                            "RFID Card"
                        )}
                    </strong>

                    <span class="ec-badge ${getStatusClass(status)}">
                        ${escapeHTML(getStatusLabel(status))}
                    </span>
                </div>

                <dl class="ec-index-rfid-details">
                    ${detail("ID", card.id || "—")}
                    ${detail("RFID", card.uid || "—")}

                    ${
                        card.balance !== undefined &&
                        card.balance !== null
                            ? detail(
                                "Balance",
                                formatCurrency(card.balance)
                              )
                            : ""
                    }
                    ${card.debt_limit !== undefined && card.debt_limit !== null ? detail("Debit Limit", formatCurrency(card.debt_limit)) : ""}
                </dl>
            </article>
        `;
    }

    function detail(label, value) {
        return `
            <div>
                <dt>${escapeHTML(label)}</dt>
                <dd>${escapeHTML(value ?? "—")}</dd>
            </div>
        `;
    }

    function renderEmptyState() {
        return `
            <div class="ec-panel ec-index-users-empty">
                <div class="ec-index-users-empty-mark" aria-hidden="true">
                    T
                </div>

                <div>
                    <h2 class="ec-section-title">
                        No tenants to display
                    </h2>

                    <p class="ec-section-description">
                        Registered ElectriCredit tenants will appear here when
                        records become available.
                    </p>
                </div>
            </div>
        `;
    }

    function maskName(value) {
        const name = String(value || "Tenant").trim();
        const parts = name.split(/\s+/);

        return parts.map((part, index) => {
            if (index === 0 || part.length <= 1) {
                return part;
            }

            return `${part.slice(0, 1)}${"*".repeat(
                Math.min(Math.max(part.length - 1, 2), 5)
            )}`;
        }).join(" ");
    }

    function maskEmail(value) {
        const email = String(value || "").trim();

        if (!email.includes("@")) {
            return email ? maskGeneric(email) : "—";
        }

        const [name, domain] = email.split("@");
        const visible = name.slice(0, Math.min(3, name.length));

        return `${visible}${"*".repeat(
            Math.min(Math.max(name.length - visible.length, 3), 6)
        )}@${domain}`;
    }

    function maskPhone(value) {
        const phone = String(value || "").trim();

        if (!phone) {
            return "—";
        }

        if (phone.length <= 4) {
            return "*".repeat(phone.length);
        }

        return `${phone.slice(0, 2)}${"*".repeat(
            Math.max(phone.length - 4, 4)
        )}${phone.slice(-2)}`;
    }

    function maskCardID(value) {
        const id = String(value || "").trim();

        if (!id || id === "—") {
            return "—";
        }

        if (id.length <= 4) {
            return `${id.slice(0, 1)}***`;
        }

        return `${id.slice(0, 3)}${"*".repeat(
            Math.min(Math.max(id.length - 5, 3), 6)
        )}${id.slice(-2)}`;
    }

    function maskGeneric(value) {
        const text = String(value || "");

        if (text.length <= 2) {
            return "*".repeat(text.length);
        }

        return `${text.slice(0, 2)}${"*".repeat(
            Math.min(Math.max(text.length - 2, 3), 6)
        )}`;
    }

    function normalizeStatus(value) {
        const status = String(value || "active").trim().toLowerCase();

        if (["disabled", "inactive", "blocked"].includes(status)) {
            return "disabled";
        }

        return "active";
    }

    function normalizeCardStatus(value) {
        return normalizeStatus(value);
    }

    function getStatusLabel(status) {
        return status === "disabled" ? "Disabled" : "Active";
    }

    function getStatusClass(status) {
        return status === "disabled"
            ? "ec-badge-danger"
            : "ec-badge-success";
    }

    function formatCurrency(value) {
        const amount = Number(value);

        if (!Number.isFinite(amount)) {
            return "₱—";
        }

        return new Intl.NumberFormat("en-PH", {
            style: "currency",
            currency: "PHP",
            minimumFractionDigits: 2,
            maximumFractionDigits: 5
        }).format(amount);
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
        render,
        renderUserModal
    };
})();

window.TenantStructure = TenantStructure;
