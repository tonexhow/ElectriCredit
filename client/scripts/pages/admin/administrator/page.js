window.ElectriCreditAdminPages = window.ElectriCreditAdminPages || {};
(() => {
    const page = {
        id: "administrators", label: "Administrator", privileged: true,
        async render(ctx, root) {
            if (!["owner","developer"].includes(ctx.role())) return ctx.navigate("overview");
            const response = await window.API.get("/admin/admins");
            const items = response.admins || [];
            ctx.state.data.administrators = items;
            const owners = items.filter(item => item.role === "owner").length;
            const admins = items.filter(item => item.role === "admin").length;
            root.innerHTML = `<section class="ec-page ec-admin-page ec-administrator-page">
                ${ctx.pageHeading("Administrator", "Administrator", "Manage Administrator and Owner accounts.", `<div class="ec-admin-page-action-group"><button class="ec-button ec-button-primary" id="administrator-add">${ctx.icon("plus")} Add Administrator</button>${ctx.role()==="developer"?`<button class="ec-button ec-button-secondary" id="owner-add">${ctx.icon("owner")} Add Owner</button>`:""}</div>`)}
                <div class="ec-admin-account-stats"><span>${ctx.icon("administrator")} <strong>${admins}</strong> Administrators</span><span>${ctx.icon("owner")} <strong>${owners}</strong> Owners</span></div>
                <div class="ec-admin-record-list">${cards(ctx, items)}</div>
            </section>`;
            root.querySelector("#administrator-add")?.addEventListener("click", () => openModal(ctx, null, "admin"));
            root.querySelector("#owner-add")?.addEventListener("click", () => openModal(ctx, null, "owner"));
            root.querySelectorAll("[data-administrator-edit]").forEach(button => button.addEventListener("click", () => openModal(ctx, ctx.findAdministrator(button.dataset.administratorEdit))));
            root.querySelectorAll("[data-administrator-delete]").forEach(button => button.addEventListener("click", () => remove(ctx, button.dataset.administratorDelete, root)));
        }
    };

    function cards(ctx, items) {
        if (!items.length) return ctx.emptyState("administrator", "No staff accounts yet", "Add an Administrator when another staff account is needed.");
        return items.map(item => {
            const ownerProtected = item.role === "owner" && ctx.role() !== "developer";
            const self = Number(item.user_id) === Number(ctx.user?.id);
            const avatar = item.image_url ? `<span class="ec-admin-avatar ec-admin-avatar-photo"><img src="${ctx.escapeAttr(item.image_url)}" alt=""></span>` : `<span class="ec-admin-avatar ec-admin-avatar-photo">${ctx.icon(item.role === "owner" ? "owner" : "administrator")}</span>`;
            return `<article class="ec-card ec-admin-account-row">
                <div class="ec-admin-account-main">${avatar}<div><strong>${ctx.escapeHTML(ctx.profileName(item))}</strong><span>${ctx.escapeHTML(item.email || "—")}</span><small>${ctx.escapeHTML(ctx.formatPhone(item.phone) || "No phone added")}</small>${ctx.staffLinksHTML(item.links)}</div></div>
                <div class="ec-admin-account-meta"><span class="ec-badge ec-admin-role-badge">${ctx.icon(item.role === "owner" ? "owner" : "administrator")} ${ctx.escapeHTML(ctx.formatRole(item.role))}</span><span class="ec-badge ${ctx.statusBadge(item.status)}">${ctx.escapeHTML(ctx.formatRole(item.status))}</span></div>
                <div class="ec-admin-actions">${ownerProtected ? `<span class="ec-help">Owner account</span>` : `<button class="ec-button ec-button-secondary ec-button-sm" data-administrator-edit="${item.id}">${ctx.icon("edit")} Edit</button><button class="ec-button ec-button-danger ec-button-sm" data-administrator-delete="${item.id}" ${self?"disabled":""}>${ctx.icon("trash")} Delete</button>`}</div>
            </article>`;
        }).join("");
    }

    function openModal(ctx, item = null, requestedRole = "admin") {
        const editing = Boolean(item);
        const canChooseOwner = ctx.role() === "developer";
        const selectedRole = item?.role || (canChooseOwner && requestedRole === "owner" ? "owner" : "admin");
        const phone = ctx.splitPhone(item?.phone || "");
        const overlay = window.Modal.open({
            title: editing ? `Edit ${ctx.formatRole(selectedRole)}` : `Add ${ctx.formatRole(selectedRole)}`,
            hideCancel: true,
            content: `<form class="ec-stack" id="administrator-form">
                ${ctx.imagePickerHTML("staff", item?.image_url, item ? ctx.profileName(item) : "Staff account")}
                <div class="ec-form-grid"><label class="ec-form-group"><span class="ec-label">First name</span><input class="ec-input" name="first_name" required value="${ctx.escapeAttr(item?.first_name||"")}"></label><label class="ec-form-group"><span class="ec-label">Last name</span><input class="ec-input" name="last_name" required value="${ctx.escapeAttr(item?.last_name||"")}"></label></div>
                <label class="ec-form-group"><span class="ec-label">Email</span><input class="ec-input" type="email" name="email" required value="${ctx.escapeAttr(item?.email||"")}"></label>
                <label class="ec-form-group"><span class="ec-label">Phone</span><div class="ec-phone-input"><select class="ec-select" name="phone_country">${ctx.countryCodeOptions(phone.code)}</select><input class="ec-input" inputmode="numeric" pattern="[0-9]*" name="phone_number" value="${ctx.escapeAttr(phone.number)}" placeholder="9938859567"></div></label>
                <fieldset class="ec-admin-links-fieldset"><legend>Links</legend><div class="ec-form-grid">${ctx.profileLinkField("Website","website",item?.links?.website)}${ctx.profileLinkField("Facebook","facebook",item?.links?.facebook)}${ctx.profileLinkField("GitHub","github",item?.links?.github)}${ctx.profileLinkField("LinkedIn","linkedin",item?.links?.linkedin)}</div></fieldset>
                <label class="ec-form-group"><span class="ec-label">Rank</span>${canChooseOwner ? `<select class="ec-select" name="role"><option value="admin" ${selectedRole==="admin"?"selected":""}>Administrator</option><option value="owner" ${selectedRole==="owner"?"selected":""}>Owner</option></select>` : `<input type="hidden" name="role" value="admin"><div class="ec-admin-readonly-field">${ctx.icon("administrator")} Administrator</div>`}</label>
                <label class="ec-form-group"><span class="ec-label">${editing?"New password (optional)":"Password"}</span><div class="ec-admin-password"><input class="ec-input" type="password" name="password" ${editing?"":"required"} minlength="8" autocomplete="new-password"><button type="button" data-eye aria-label="Show password">${ctx.icon("eye")}</button></div></label>
                <label class="ec-form-group"><span class="ec-label">Status</span><select class="ec-select" name="status">${ctx.options(["active","inactive","disabled"], item?.status || "active")}</select></label>
                <button class="ec-button ec-button-primary" type="submit">${ctx.icon("save")} ${editing?"Save account":"Add account"}</button>
            </form>`
        });
        ctx.bindEyes(overlay);
        const form = overlay.querySelector("#administrator-form");
        ctx.bindImagePicker(overlay, "staff", data => { if (form) { let input=form.elements.image_data; if(!input){input=document.createElement("input");input.type="hidden";input.name="image_data";form.appendChild(input);} input.value=data; } });
        const hidden = overlay.querySelector('[data-image-picker="staff"] [data-image-value]'); if (hidden) hidden.name = "image_data";
        form?.querySelector('[name="phone_number"]')?.addEventListener("input", e => { e.target.value=e.target.value.replace(/\D/g,"").slice(0,14); });
        form?.addEventListener("submit", async event => {
            event.preventDefault(); const button=form.querySelector('button[type="submit"]'); ctx.setBusy(button,true);
            try {
                const body=ctx.formObject(form); const digits=String(body.phone_number||"").replace(/\D/g,""); body.phone=digits?`${body.phone_country}${digits}`:""; delete body.phone_country; delete body.phone_number;
                body.links={website:body.link_website||"",facebook:body.link_facebook||"",github:body.link_github||"",linkedin:body.link_linkedin||""}; ["link_website","link_facebook","link_github","link_linkedin"].forEach(k=>delete body[k]);
                if(editing) await window.API.put(`/admin/admins/${item.id}`,body); else await window.API.post("/admin/admins",body);
                window.Modal.close({force:true}); window.Notification?.success?.(editing?"Account updated.":"Account added."); await page.render(ctx,document.getElementById("admin-content"));
            } catch(error){window.Notification?.error?.(error.message);} finally {ctx.setBusy(button,false);}
        });
    }

    async function remove(ctx,id,root){const item=ctx.findAdministrator(id);if(!item)return;const ok=await window.Modal.confirm({title:`Delete ${ctx.formatRole(item.role)}`,message:`Delete the ${ctx.formatRole(item.role)} account for ${ctx.profileName(item)}?`,confirmText:"Delete",cancelText:"Cancel",danger:true});if(!ok)return;try{await window.API.delete(`/admin/admins/${id}`);window.Notification?.success?.("Account deleted.");await page.render(ctx,root);}catch(error){window.Notification?.error?.(error.message);}}

    window.ElectriCreditAdminPages.administrators = page;
})();
