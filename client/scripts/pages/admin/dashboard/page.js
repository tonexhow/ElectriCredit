window.ElectriCreditAdminPages = window.ElectriCreditAdminPages || {};
(() => {
    let charts = [];
    let activeView = "power";

    const P = {
        id: "dashboard",
        label: "Dashboard",
        async render(ctx, root) {
            const filter = ctx.state.dashboardFilter || { range: "30d", from: "", to: "" };
            const params = new URLSearchParams({ range: filter.range || "30d" });
            if (filter.range === "custom") {
                if (filter.from) params.set("from", filter.from);
                if (filter.to) params.set("to", filter.to);
            }
            const data = await window.API.get(`/admin/dashboard?${params}`);
            ctx.state.data.dashboard = data;
            const topTenant = data.highlights?.top_consumer || {};
            const topHub = data.highlights?.top_hub || {};
            const sessions = Number(data.summary?.sessions || 0);
            const activeSessions = Number(data.summary?.active_sessions || 0);
            const hubs = data.hubs || [];
            const activeHubs = hubs.filter(item => String(item.status).toLowerCase() === "active").length;
            const sessionPct = sessions ? Math.round(activeSessions / sessions * 100) : 0;
            const hubPct = hubs.length ? Math.round(activeHubs / hubs.length * 100) : 0;

            root.innerHTML = `<section class="ec-page ec-admin-page ec-dashboard-page">
                ${ctx.pageHeading("Dashboard", "Dashboard", "Review electricity activity, collections, usage patterns, and operational performance.")}
                <div class="ec-admin-dashboard-summary">
                    ${ctx.statCard("Total consumption", `${ctx.number(data.summary?.total_kwh, 3)} kWh`, data.filter?.label || "Selected period", "bolt", true)}
                    ${ctx.statCard("Top consumer", topTenant.name || "No usage yet", topTenant.name ? `${ctx.number(topTenant.kwh, 3)} kWh` : "No recorded usage", "tenant", true)}
                    ${ctx.statCard("Top usage hub", topHub.name || "No usage yet", topHub.name ? `${ctx.number(topHub.kwh, 3)} kWh` : "No recorded usage", "hub", true)}
                    ${ctx.statCard("Busy hour", sessions ? data.summary?.busy_hour : "—", sessions ? `${ctx.number(data.summary?.busy_hour_kwh, 3)} kWh recorded` : "No sessions recorded", "clock", true)}
                </div>

                <div class="ec-admin-dashboard-summary ec-dashboard-financials">
                    ${ctx.statCard("Total revenue", ctx.peso(data.financials?.revenue), "Recorded tenant electricity charges", "rate", true)}
                    ${ctx.statCard("Electricity cost", ctx.peso(data.financials?.estimated_base_cost), "Estimated at the current base rate", "bolt", true)}
                    ${ctx.statCard("Projected income", ctx.peso(data.financials?.projected_income), "Usage × current income per kWh", "chart", true)}
                    ${ctx.statCard("Gross profit", ctx.peso(data.financials?.gross_profit), `${ctx.escapeHTML(data.financials?.profit_margin || "0.00")}% of recorded revenue`, "rate", true)}
                </div>

                ${hardwareWarningPanel(data.hardware_warnings || [], ctx)}

                <div class="ec-dashboard-figures">
                    ${ring("Active sessions", sessionPct, `${activeSessions}/${sessions}`, "Running now", ctx)}
                    ${ring("Hub availability", hubPct, `${activeHubs}/${hubs.length}`, "Active hubs", ctx)}
                    <article class="ec-card ec-dashboard-figure"><span class="ec-dashboard-figure-icon">${ctx.icon("bolt")}</span><div><small>Average usage / session</small><strong>${ctx.number(data.summary?.average_kwh, 3)} kWh</strong><span>${sessions.toLocaleString("en-PH")} sessions in this period</span></div></article>
                    <article class="ec-card ec-dashboard-figure"><span class="ec-dashboard-figure-icon">${ctx.icon("rate")}</span><div><small>Income / kWh</small><strong>${ctx.peso(data.rates?.income_per_kwh)}</strong><span>${ctx.escapeHTML(data.rates?.markup_percentage || "0.00")}% markup</span></div></article>
                </div>

                <section class="ec-card ec-admin-dashboard-monitor">
                    <div class="ec-admin-dashboard-monitor-head">
                        <div><span class="ec-admin-section-pill">Analytics</span><h2>Electricity activity</h2><p>Compare activity by power, hub, tenant, or hour.</p></div>
                        <div class="ec-admin-dashboard-tabs" role="tablist">
                            ${tab("power", "Power", "bolt", activeView === "power", ctx)}
                            ${tab("hubs", "Hubs", "hub", activeView === "hubs", ctx)}
                            ${tab("tenants", "Tenants", "tenant", activeView === "tenants", ctx)}
                            ${tab("usage", "Usage", "clock", activeView === "usage", ctx)}
                        </div>
                    </div>
                    <form class="ec-dashboard-filter ec-dashboard-filter-inline" id="dashboard-filter-form">
                        <div><small>Showing</small><strong>${ctx.escapeHTML(data.filter?.label || "Last 30 days")}</strong></div>
                        <label><span>Period</span><select class="ec-select" name="range">${rangeOptions(filter.range)}</select></label>
                        <div class="ec-dashboard-custom-dates" data-dashboard-custom ${filter.range === "custom" ? "" : "hidden"}>
                            <label><span>From</span><input class="ec-input" type="date" name="from" value="${ctx.escapeAttr(filter.from || "")}"></label>
                            <label><span>To</span><input class="ec-input" type="date" name="to" value="${ctx.escapeAttr(filter.to || "")}"></label>
                        </div>
                        <button class="ec-button ec-button-primary ec-button-sm" type="submit">Apply</button>
                    </form>
                    <div class="ec-admin-dashboard-panels">
                        ${panel("power", activeView, chartCard("Power trend", `Electricity consumption · ${data.filter?.label || "selected period"}`, "dashboard-power-chart", ctx) + tableCard("Recent electricity sessions", "Latest recorded usage", recentTable(data.recent_sessions || [], ctx), ctx))}
                        ${panel("hubs", activeView, chartCard("Hub performance", "Compare electricity usage across hubs", "dashboard-hub-chart", ctx) + tableCard("Hub summary", "Sessions, usage, and collected amount", hubTable(data.hubs || [], ctx), ctx))}
                        ${panel("tenants", activeView, chartCard("Tenant usage", "Compare recorded consumption by tenant", "dashboard-tenant-chart", ctx) + tableCard("Tenant summary", "Balances and electricity activity", tenantTable(data.tenants || [], ctx), ctx))}
                        ${panel("usage", activeView, chartCard("Hourly usage", "Electricity activity throughout the day", "dashboard-hourly-chart", ctx) + tableCard("Hourly summary", "Consumption and sessions by hour", hourlyTable(data.hourly_usage || [], ctx), ctx))}
                    </div>
                </section>

                <div class="ec-admin-dashboard-summary ec-admin-dashboard-account-summary">
                    ${ctx.statCard("Active tenants", Number(data.summary?.active_tenants || 0).toLocaleString("en-PH"), "Currently active tenant accounts", "tenant", true)}
                    ${ctx.statCard("RFID cards", Number(data.summary?.cards || 0).toLocaleString("en-PH"), "Registered electricity access cards", "card", true)}
                    ${ctx.statCard("Combined balance", ctx.peso(data.summary?.total_balance), "Balance across registered cards", "rate", true)}
                    ${ctx.statCard("Team online", Number(data.summary?.staff_online || 0).toLocaleString("en-PH"), "Staff currently active", "administrator", true)}
                </div>
                ${tableCard("Team online", "Staff currently active in ElectriCredit", staffTable(data.active_staff || [], ctx), ctx)}
            </section>`;

            bindFilter(ctx, root);
            bindTabs(ctx, root);
            requestAnimationFrame(() => draw(ctx, data, activeView));
        },
        refresh(ctx) {
            if (ctx.state.route !== "dashboard") return;
            dispose();
            requestAnimationFrame(() => draw(ctx, ctx.state.data.dashboard || {}, activeView));
        },
        dispose
    };

    function hardwareWarningPanel(items,c){if(!items.length)return "";return `<article class="ec-card ec-dashboard-hardware-warning"><div class="ec-card-header"><div><h2 class="ec-card-title">Hardware needs attention</h2><p class="ec-card-description">A connected module reported a sensor or device fault.</p></div><span class="ec-badge ec-badge-warning">${items.length} warning${items.length===1?"":"s"}</span></div><div class="ec-card-body"><div class="ec-dashboard-warning-list">${items.map(x=>`<div class="ec-dashboard-warning-row"><span class="ec-dashboard-warning-icon">${c.icon("alert")}</span><div><strong>${c.escapeHTML(x.name||`${x.type} #${x.id}`)}</strong><small>${c.escapeHTML(x.warning||"Hardware warning")}${x.location?` · ${c.escapeHTML(x.location)}`:""}</small></div><time>${c.escapeHTML(c.formatDateTime(x.last_seen_at))}</time></div>`).join("")}</div></div></article>`;}
    function rangeOptions(selected) { return [["7d","Last 7 days"],["30d","Last 30 days"],["90d","Last 90 days"],["all","All activity"],["custom","Custom dates"]].map(([v,l]) => `<option value="${v}" ${selected===v?"selected":""}>${l}</option>`).join(""); }
    function ring(label, pct, value, caption, c) { const safe=Math.max(0,Math.min(100,Number(pct||0))); return `<article class="ec-card ec-dashboard-ring-card"><div class="ec-dashboard-ring" style="--value:${safe}"><span>${safe}%</span></div><div><small>${c.escapeHTML(label)}</small><strong>${c.escapeHTML(value)}</strong><span>${c.escapeHTML(caption)}</span></div></article>`; }
    function tab(id,label,iconName,active,c){return `<button type="button" data-dashboard-tab="${id}" class="${active?"is-active":""}" aria-selected="${active}">${c.icon(iconName)}<span>${c.escapeHTML(label)}</span></button>`;}
    function panel(id,current,body){return `<section class="ec-admin-dashboard-panel ${current===id?"is-active":""}" data-dashboard-panel="${id}" ${current===id?"":"hidden"}>${body}</section>`;}
    function chartCard(title,desc,id,c){return `<article class="ec-card ec-admin-chart-card"><div class="ec-card-header"><div><h2 class="ec-card-title">${c.escapeHTML(title)}</h2><p class="ec-card-description">${c.escapeHTML(desc)}</p></div></div><div class="ec-card-body"><div class="ec-admin-chart" id="${id}"></div></div></article>`;}
    function tableCard(title,desc,body,c){return `<article class="ec-card ec-admin-dashboard-table-card"><div class="ec-card-header"><div><h2 class="ec-card-title">${c.escapeHTML(title)}</h2><p class="ec-card-description">${c.escapeHTML(desc)}</p></div></div><div class="ec-card-body">${body}</div></article>`;}
    function table(headers, rows, empty, c){if(!rows.length)return `<div class="ec-admin-inline-empty">${c.escapeHTML(empty)}</div>`;return `<div class="ec-admin-table-wrap"><table class="ec-table ec-admin-data-table"><thead><tr>${headers.map(x=>`<th>${c.escapeHTML(x)}</th>`).join("")}</tr></thead><tbody>${rows.join("")}</tbody></table></div>`;}
    function recentTable(items,c){return table(["Tenant","Hub","Started","kWh","Cost","Status"],items.map(x=>`<tr><td><strong>${c.escapeHTML(x.tenant||"—")}</strong><small>Card ${c.escapeHTML(x.card_id||"—")} · ${c.escapeHTML(x.card_uid||"—")}</small></td><td>${c.escapeHTML(x.hub||"—")}</td><td>${c.escapeHTML(c.formatDateTime(x.started_at))}</td><td>${c.number(x.kwh,3)}</td><td>${c.peso(x.cost)}</td><td><span class="ec-badge ${c.statusBadge(x.status)}">${c.escapeHTML(x.status||"—")}</span></td></tr>`),"No electricity sessions in this period.",c);}
    function hubTable(items,c){return table(["Hub","Status","Sessions","kWh","Revenue"],items.map(x=>`<tr><td><strong>${c.escapeHTML(x.name)}</strong><small>${c.escapeHTML(x.location||"No location")}</small></td><td><span class="ec-badge ${c.statusBadge(x.status)}">${c.escapeHTML(x.status)}</span></td><td>${Number(x.sessions||0)}</td><td>${c.number(x.kwh,3)}</td><td>${c.peso(x.revenue)}</td></tr>`),"No hubs registered.",c);}
    function tenantTable(items,c){return table(["Tenant","Cards","Sessions","kWh","Balance"],items.map(x=>`<tr><td><strong>${c.escapeHTML(x.name)}</strong><small>${c.escapeHTML(x.email||"No email")}</small></td><td>${Number(x.cards||0)}</td><td>${Number(x.sessions||0)}</td><td>${c.number(x.kwh,3)}</td><td>${c.peso(x.balance)}</td></tr>`),"No tenants registered.",c);}
    function hourlyTable(items,c){const rows=items.filter(x=>Number(x.kwh||0)>0||Number(x.sessions||0)>0);return table(["Hour","Sessions","kWh"],rows.map(x=>`<tr><td><strong>${String(x.hour).padStart(2,"0")}:00</strong></td><td>${Number(x.sessions||0)}</td><td>${c.number(x.kwh,3)}</td></tr>`),"No hourly activity in this period.",c);}
    function staffTable(items,c){return table(["Staff member","Rank","Email","Last active"],items.map(x=>`<tr><td><div class="ec-admin-table-person">${c.avatarHTML(x,"ec-admin-table-avatar")}<strong>${c.escapeHTML(c.profileName(x))}</strong></div></td><td>${c.escapeHTML(c.formatRole(x.role))}</td><td>${c.escapeHTML(x.email||"—")}</td><td>${c.escapeHTML(c.formatDateTime(x.last_seen_at))}</td></tr>`),"No staff currently active.",c);}

    function bindFilter(ctx, root){const form=root.querySelector("#dashboard-filter-form");if(!form)return;const range=form.elements.range,custom=form.querySelector("[data-dashboard-custom]");const sync=()=>{custom.hidden=range.value!=="custom";};range.addEventListener("change",sync);sync();form.addEventListener("submit",async e=>{e.preventDefault();ctx.state.dashboardFilter={range:range.value,from:form.elements.from?.value||"",to:form.elements.to?.value||""};if(range.value==="custom"&&(!ctx.state.dashboardFilter.from||!ctx.state.dashboardFilter.to)){window.Notification?.warning?.("Choose both a start and end date.");return;}await P.render(ctx,root);});}
    function bindTabs(ctx, root){root.querySelectorAll("[data-dashboard-tab]").forEach(btn=>btn.addEventListener("click",()=>{activeView=btn.dataset.dashboardTab;root.querySelectorAll("[data-dashboard-tab]").forEach(b=>{const on=b===btn;b.classList.toggle("is-active",on);b.setAttribute("aria-selected",String(on));});root.querySelectorAll("[data-dashboard-panel]").forEach(p=>{const on=p.dataset.dashboardPanel===activeView;p.hidden=!on;p.classList.toggle("is-active",on);});dispose();requestAnimationFrame(()=>draw(ctx,ctx.state.data.dashboard||{},activeView));}));}

    function draw(ctx,data,view){if(!window.echarts){const id={power:"dashboard-power-chart",hubs:"dashboard-hub-chart",tenants:"dashboard-tenant-chart",usage:"dashboard-hourly-chart"}[view];const el=document.getElementById(id);if(el)el.innerHTML=`<div class="ec-admin-inline-empty">Charts are unavailable, but the table below remains available.</div>`;return;}dispose();const text=ctx.getCSS("--ec-text","#20242a"),muted=ctx.getCSS("--ec-text-muted","#6b7280"),border=ctx.getCSS("--ec-border","#dde2e8"),accent=ctx.getCSS("--ec-accent","#f2b705");const base={textStyle:{color:text},tooltip:{trigger:"axis"},grid:{left:44,right:20,top:25,bottom:45},xAxis:{axisLabel:{color:muted},axisLine:{lineStyle:{color:border}}},yAxis:{axisLabel:{color:muted},splitLine:{lineStyle:{color:border}}}};let id,option;
        if(view==="power"){const rows=data.power_trend||[];id="dashboard-power-chart";option={...base,xAxis:{...base.xAxis,type:"category",data:rows.map(x=>x.date)},yAxis:{...base.yAxis,type:"value",name:"kWh"},series:[{type:"line",smooth:true,data:rows.map(x=>Number(x.kwh||0)),areaStyle:{opacity:.1},lineStyle:{width:3},itemStyle:{color:accent}}]};}
        if(view==="hubs"){const rows=data.hubs||[];id="dashboard-hub-chart";option={...base,xAxis:{...base.xAxis,type:"category",data:rows.map(x=>x.name),axisLabel:{color:muted,rotate:20}},yAxis:{...base.yAxis,type:"value",name:"kWh"},series:[{type:"bar",data:rows.map(x=>Number(x.kwh||0)),itemStyle:{color:accent,borderRadius:[6,6,0,0]}}]};}
        if(view==="tenants"){const rows=(data.tenants||[]).slice(0,10);id="dashboard-tenant-chart";option={...base,xAxis:{...base.xAxis,type:"category",data:rows.map(x=>ctx.shortName(x.name)),axisLabel:{color:muted,rotate:20}},yAxis:{...base.yAxis,type:"value",name:"kWh"},series:[{type:"bar",data:rows.map(x=>Number(x.kwh||0)),itemStyle:{color:accent,borderRadius:[6,6,0,0]}}]};}
        if(view==="usage"){const rows=data.hourly_usage||[];id="dashboard-hourly-chart";option={...base,xAxis:{...base.xAxis,type:"category",data:rows.map(x=>`${String(x.hour).padStart(2,"0")}:00`)},yAxis:{...base.yAxis,type:"value",name:"kWh"},series:[{type:"line",smooth:true,data:rows.map(x=>Number(x.kwh||0)),lineStyle:{width:3},itemStyle:{color:accent}}]};}
        const el=document.getElementById(id);if(!el||!option)return;const chart=window.echarts.init(el);chart.setOption(option);charts.push(chart);window.addEventListener("resize",chart.resize,{passive:true,once:true});
    }
    function dispose(){charts.forEach(chart=>{try{chart.dispose();}catch(_){}});charts=[];}

    window.ElectriCreditAdminPages.dashboard = P;
})();
