window.ElectriCreditAdminPages = window.ElectriCreditAdminPages || {};
window.ElectriCreditAdminPages.overview = {
    id:"overview", label:"Overview",
    async render(ctx,root){
        const data=ctx.state.preloaded?.overview||await window.API.get("/admin/overview");
        if(ctx.state.preloaded)ctx.state.preloaded.overview=null;
        ctx.state.data.overview=data;
        const counts=data.counts||{}, staff=data.active_staff||[];
        root.innerHTML=`<section class="ec-page ec-admin-page">
            ${ctx.pageHeading("Overview","Overview","A clear snapshot of ElectriCredit today.")}
            <div class="ec-admin-stat-grid">
                ${ctx.statCard("Tenants",counts.tenants||0,"Registered tenants","tenant")}
                ${ctx.statCard("RFID Cards",counts.cards||0,"Assigned cards","card")}
                ${ctx.statCard("Hubs",counts.hubs||0,"Registered hubs","hub")}
                ${ctx.statCard("Registry",counts.registries||0,"Registry stations","registry")}
                ${ctx.statCard("Administrators",counts.administrators||0,"Active administrator accounts","administrator")}
                ${ctx.statCard("Power Sessions",counts.active_power||0,"Currently active","bolt")}
            </div>
            <div class="ec-admin-overview-grid">
                <article class="ec-card"><div class="ec-card-header"><div><h2 class="ec-card-title">Currently signed in</h2><p class="ec-card-description">Staff with a recent active session.</p></div><span class="ec-badge ec-badge-success">${staff.length} active</span></div><div class="ec-card-body ec-admin-active-staff">${activeStaff(staff,ctx)}</div></article>
                <article class="ec-card"><div class="ec-card-header"><div><h2 class="ec-card-title">Electricity pricing</h2><p class="ec-card-description">Current company and tenant rates.</p></div></div><div class="ec-card-body ec-admin-detail-grid"><div><span>Base rate</span><strong>${ctx.peso(data.rates?.base_rate)}</strong></div><div><span>Tenant rate</span><strong>${ctx.peso(data.rates?.tenant_rate)}</strong></div><div><span>Income / kWh</span><strong>${ctx.peso(data.rates?.income_per_kwh)}</strong></div><div><span>Markup</span><strong>${ctx.escapeHTML(data.rates?.markup_percentage||"0.00")}%</strong></div></div></article>
            </div>
        </section>`;
    }
};
function activeStaff(staff,ctx){if(!staff.length)return`<div class="ec-admin-inline-empty">No other active staff sessions are listed right now.</div>`;return staff.map(person=>`<div class="ec-admin-staff-person">${ctx.avatarHTML(person,"ec-admin-avatar")}<div><strong>${ctx.escapeHTML(`${person.first_name||""} ${person.last_name||""}`.trim())}</strong><span>${ctx.escapeHTML(ctx.formatRole(person.role))}</span></div><span class="ec-admin-online-dot" title="Active"></span></div>`).join("");}
