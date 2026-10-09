window.ElectriCreditAdminPages = window.ElectriCreditAdminPages || {};

window.ElectriCreditAdminPages.rate = {
    id: "rate",
    label: "Rate",

    async render(ctx, root) {
        const data = await window.API.get("/admin/rates");
        const updatedBy = data.updated_by_name
            ? `${data.updated_by_name}${data.updated_by_rank ? ` · ${ctx.formatRole(data.updated_by_rank)}` : ""}`
            : "Initial configuration";
        const updatedAt = data.updated_at ? new Date(Number(data.updated_at) * 1000).toLocaleString() : "—";
        const createdAt = data.created_at ? new Date(Number(data.created_at) * 1000).toLocaleString() : "—";

        root.innerHTML = `<section class="ec-page ec-admin-page ec-rate-page">
            ${ctx.pageHeading("Rate", "Electricity rate", "Set a clear tenant price and see the expected earnings before you save any change.")}

            <div class="ec-rate-hero-strip">
                <div class="ec-rate-hero-copy">
                    <span class="ec-admin-section-pill">Current pricing</span>
                    <h2>Simple pricing, visible margin.</h2>
                    <p>ElectriCredit keeps the provider cost and tenant price side by side so every rate change is easy to review.</p>
                </div>
                <div class="ec-rate-hero-profit">
                    <span>${ctx.icon("rate")}</span>
                    <div><small>Profit per kWh</small><strong>${ctx.peso(data.income_per_kwh)}</strong><em>${ctx.escapeHTML(data.markup_percentage || "0.00")}% markup</em></div>
                </div>
            </div>

            <div class="ec-rate-summary-grid">
                ${summaryCard(ctx, "Base rate", data.base_rate, "Provider cost / kWh", "bolt")}
                ${summaryCard(ctx, "Tenant rate", data.tenant_rate, "Tenant charge / kWh", "tenant")}
                ${summaryCard(ctx, "Profit", data.income_per_kwh, "Tenant − Base", "rate", true)}
            </div>

            <div class="ec-rate-workspace">
                <article class="ec-card ec-rate-editor-card">
                    <div class="ec-card-header ec-rate-card-header">
                        <div>
                            <span class="ec-admin-section-pill">Pricing editor</span>
                            <h2 class="ec-card-title">Update electricity rates</h2>
                            <p class="ec-card-description">Use up to five decimal places for small electricity charges. The preview updates instantly.</p>
                        </div>
                    </div>
                    <form class="ec-card-body ec-rate-editor" id="rate-form">
                        <div class="ec-rate-input-grid">
                            <label class="ec-form-group ec-rate-field-card">
                                <span class="ec-label">Base rate / kWh</span>
                                <span class="ec-rate-money-input"><b>₱</b><input class="ec-input" name="base_rate" inputmode="decimal" autocomplete="off" value="${ctx.escapeAttr(data.base_rate || "")}" required></span>
                                <small class="ec-help">What the electricity provider charges.</small>
                            </label>
                            <label class="ec-form-group ec-rate-field-card">
                                <span class="ec-label">Tenant rate / kWh</span>
                                <span class="ec-rate-money-input"><b>₱</b><input class="ec-input" name="tenant_rate" inputmode="decimal" autocomplete="off" value="${ctx.escapeAttr(data.tenant_rate || "")}" required></span>
                                <small class="ec-help">What ElectriCredit deducts from tenant balances.</small>
                            </label>
                        </div>

                        <div class="ec-rate-live" aria-live="polite">
                            <div class="ec-rate-live-heading">
                                <span>${ctx.icon("chart")}</span>
                                <div><strong>Live profit preview</strong><small>Tenant rate minus base rate</small></div>
                            </div>
                            <div class="ec-rate-equation">
                                <div><span>Tenant rate</span><strong data-live-tenant>${ctx.peso(data.tenant_rate)}</strong></div>
                                <b aria-hidden="true">−</b>
                                <div><span>Base rate</span><strong data-live-base>${ctx.peso(data.base_rate)}</strong></div>
                                <b aria-hidden="true">=</b>
                                <div class="is-profit"><span>Profit / kWh</span><strong data-live-profit>${ctx.peso(data.income_per_kwh)}</strong></div>
                            </div>
                            <div class="ec-rate-live-foot"><span>Markup from base cost</span><strong data-live-markup>${ctx.escapeHTML(data.markup_percentage || "0.00")}%</strong></div>
                        </div>

                        <label class="ec-form-group ec-admin-rate-note ec-rate-note-card">
                            <span class="ec-label">Change note <small>(optional)</small></span>
                            <textarea class="ec-input" name="change_note" rows="3" maxlength="240" placeholder="Why is this rate being changed?"></textarea>
                            <small class="ec-help">This note appears in the latest pricing activity.</small>
                        </label>

                        <div class="ec-rate-save-row">
                            <div class="ec-rate-save-note">${ctx.icon("alert")}<span>The saved tenant rate becomes active immediately.</span></div>
                            <button class="ec-button ec-button-primary" type="submit">${ctx.icon("save")} Save rates</button>
                        </div>
                    </form>
                </article>

                <aside class="ec-rate-side-column">
                    <article class="ec-card ec-rate-history-card">
                        <div class="ec-rate-history-head">
                            <span class="ec-rate-history-icon">${ctx.icon("clock")}</span>
                            <div><span class="ec-admin-section-pill">Latest activity</span><h2>Rate update</h2><p>The most recent saved pricing change.</p></div>
                        </div>
                        <div class="ec-rate-history-body">
                            <div class="ec-rate-history-author">
                                <span class="ec-rate-history-avatar">${ctx.icon("developer")}</span>
                                <div><small>Changed by</small><strong>${ctx.escapeHTML(updatedBy)}</strong></div>
                            </div>
                            <div class="ec-rate-history-time-grid">
                                <div><small>Updated</small><strong>${ctx.escapeHTML(updatedAt)}</strong></div>
                                <div><small>Created</small><strong>${ctx.escapeHTML(createdAt)}</strong></div>
                            </div>
                            <div class="ec-rate-history-note"><span>${ctx.icon("edit")}</span><div><small>Change note</small><p>${ctx.escapeHTML(data.change_note || "No note was provided for this update.")}</p></div></div>
                        </div>
                    </article>

                    <article class="ec-rate-guide-card">
                        <span>${ctx.icon("bolt")}</span>
                        <div><strong>Pricing reminder</strong><p>Keep the tenant rate at or above the base rate when a positive margin is intended.</p></div>
                    </article>
                </aside>
            </div>
        </section>`;

        const form = root.querySelector("#rate-form");
        bindLivePreview(ctx, form);

        form?.addEventListener("submit", async event => {
            event.preventDefault();
            const button = event.currentTarget.querySelector('button[type="submit"]');
            ctx.setBusy(button, true);
            try {
                await window.API.put("/admin/rates", ctx.formObject(event.currentTarget));
                window.Notification?.success?.("Electricity rates updated.");
                await this.render(ctx, root);
            } catch (error) {
                window.Notification?.error?.(error.message);
            } finally {
                ctx.setBusy(button, false);
            }
        });
    }
};

function summaryCard(ctx, label, value, helper, icon, profit = false) {
    return `<article class="ec-card ec-rate-summary-card ${profit ? "is-profit" : ""}">
        <div class="ec-card-body">
            <span class="ec-rate-summary-icon">${ctx.icon(icon)}</span>
            <div><span>${ctx.escapeHTML(label)}</span><strong>${ctx.peso(value)}</strong><small>${ctx.escapeHTML(helper)}</small></div>
        </div>
    </article>`;
}

function bindLivePreview(ctx, form) {
    if (!form) return;
    const baseInput = form.elements.base_rate;
    const tenantInput = form.elements.tenant_rate;
    const baseOut = form.querySelector("[data-live-base]");
    const tenantOut = form.querySelector("[data-live-tenant]");
    const profitOut = form.querySelector("[data-live-profit]");
    const markupOut = form.querySelector("[data-live-markup]");

    const update = () => {
        const base = parseRate(baseInput.value);
        const tenant = parseRate(tenantInput.value);
        const profit = tenant - base;
        const markup = base > 0 ? (profit / base) * 100 : 0;
        baseOut.textContent = formatRatePeso(base);
        tenantOut.textContent = formatRatePeso(tenant);
        profitOut.textContent = formatRatePeso(profit);
        markupOut.textContent = `${Number.isFinite(markup) ? markup.toFixed(2) : "0.00"}%`;
        profitOut.closest(".is-profit")?.classList.toggle("is-negative", profit < 0);
    };

    baseInput.addEventListener("input", update);
    tenantInput.addEventListener("input", update);
    update();
}

function parseRate(value) {
    const normalized = String(value ?? "").replace(/[^0-9.-]/g, "");
    const parsed = Number(normalized);
    return Number.isFinite(parsed) ? parsed : 0;
}

function formatRatePeso(value) {
    const number = Number(value || 0);
    return `${number < 0 ? "-" : ""}₱${Math.abs(number).toLocaleString("en-PH", { minimumFractionDigits: 2, maximumFractionDigits: 5 })}`;
}
