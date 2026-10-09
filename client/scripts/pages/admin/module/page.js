(() => {
    const pages = window.ElectriCreditAdminPages = window.ElectriCreditAdminPages || {};

    pages.module = {
        async render(ctx, root) {
            const response = await window.API.get("/admin/modules");
            const hubs = response?.hubs || [];
            const registries = response?.registries || [];
            root.innerHTML = `<section class="ec-page ec-admin-page ec-admin-module-page">
                ${ctx.pageHeading("Modules", "Module", "Check Hub and Registry status and send immediate PING or TERMINATE commands.")}
                ${moduleSection(ctx, "hub", "Hub", hubs)}
                ${moduleSection(ctx, "registry", "Registry", registries)}
            </section>`;
            bind(ctx, root);
            window.UIEnhance?.init?.(root);
        }
    };

    function moduleSection(ctx, kind, title, items) {
        return `<section class="ec-admin-module-section">
            <div class="ec-admin-module-section-head"><div><h2>${title}</h2><p>${items.length} registered ${items.length === 1 ? "module" : "modules"}</p></div></div>
            <div class="ec-admin-module-grid">${items.length ? items.map(item => card(ctx, kind, item)).join("") : ctx.emptyState(kind === "hub" ? "hub" : "registry", `No ${title} modules`, `No ${title} modules have been registered yet.`)}</div>
        </section>`;
    }

    function card(ctx, kind, item) {
        const online = Boolean(item.online);
        const activeSession = Number(item.active_session_id || 0);
        const health = item.health_status === "warning" ? "Warning" : online ? "Online" : item.health_status === "ok" ? "Recently seen" : "Offline";
        const healthClass = item.health_status === "warning" ? "ec-badge-warning" : online ? "ec-badge-success" : "ec-badge-warning";
        return `<article class="ec-card ec-admin-module-card">
            <div class="ec-admin-module-card-head">
                <span class="ec-admin-module-icon">${ctx.icon(kind === "hub" ? "hub" : "registry")}</span>
                <div><strong>${ctx.escapeHTML(item.name || `${kind} #${item.id}`)}</strong><small>${ctx.escapeHTML(item.location || "No location")} · ID ${item.id}</small></div>
                <span class="ec-badge ${healthClass}">${ctx.escapeHTML(health)}</span>
            </div>
            <div class="ec-admin-module-details">
                <span>MAC <b>${ctx.escapeHTML(item.mac_address || "Not paired")}</b></span>
                <span>Status <b>${ctx.escapeHTML(ctx.formatRole(item.status || "unknown"))}</b></span>
                <span>Active session <b>${activeSession > 0 ? `#${activeSession}` : "None"}</b></span>
                <span>Last seen <b>${ctx.escapeHTML(ctx.formatDateTime(item.last_seen_at))}</b></span>
                ${item.last_warning ? `<p class="ec-admin-module-warning">${ctx.icon("alert")}<span>${ctx.escapeHTML(item.last_warning)}</span></p>` : ""}
            </div>
            <div class="ec-admin-module-controls">
                <button class="ec-button ec-button-secondary ec-button-sm" type="button" data-module-control="ping" data-kind="${kind}" data-id="${item.id}" ${!item.connected || item.status === "disabled" ? "disabled" : ""}>${ctx.icon("wifi")}<span>PING</span></button>
                <button class="ec-button ec-button-danger ec-button-sm" type="button" data-module-control="terminate" data-kind="${kind}" data-id="${item.id}" ${!item.connected || item.status === "disabled" ? "disabled" : ""}>${ctx.icon("alert")}<span>TERMINATE</span></button>
            </div>
        </article>`;
    }

    function bind(ctx, root) {
        root.querySelectorAll("[data-module-control]").forEach(button => button.addEventListener("click", async () => {
            const action = button.dataset.moduleControl;
            const kind = button.dataset.kind;
            const id = Number(button.dataset.id || 0);
            if (action === "terminate") {
                const confirmed = await window.Modal.confirm({
                    title: `Terminate ${kind === "hub" ? "Hub" : "Registry"} session`,
                    message: kind === "hub"
                        ? "This immediately tells the Hub to turn electricity OFF, finalize its current usage, and require the RFID to be removed/reinserted before service can start again."
                        : "This immediately tells the Registry to stop the active top-up, preserve accepted coin data, and require the RFID to be removed/reinserted.",
                    confirmText: "Send TERMINATE",
                    cancelText: "Cancel",
                    danger: true
                });
                if (!confirmed) return;
            }
            ctx.setBusy(button, true);
            try {
                const result = await window.API.post("/admin/modules/control", { kind, id, action });
                window.Notification?.success?.(result?.message || `${action.toUpperCase()} sent.`);
                setTimeout(() => ctx.navigate("module", false), action === "terminate" ? 600 : 350);
            } catch (error) {
                window.Notification?.error?.(error.message || `Unable to ${action} module.`);
            } finally {
                ctx.setBusy(button, false);
            }
        }));
    }
})();
