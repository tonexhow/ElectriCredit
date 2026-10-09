/** ElectriCredit Administrator workspace shell and route coordinator. */
const AdminApp = (() => {
    const state = {
        user: null,
        profile: null,
        route: "overview",
        data: {},
        mounted: false,
        hashBound: false,
        themeBound: false,
        staffMessages: [],
        chatTab: "support",
        dashboardFilter: { range:"30d", from:"", to:"" },
        chatBadgeBound: false,
        preloaded: null
    };
    const S = window.AdminShared;
    const routes = [
        {id:"overview",label:"Overview",icon:S.icon("home")},
        {id:"dashboard",label:"Dashboard",icon:S.icon("chart")},
        {id:"tenant",label:"Tenant",icon:S.icon("tenant")},
        {id:"administrators",label:"Administrator",icon:S.icon("administrator"),privileged:true},
        {id:"rate",label:"Rate",icon:S.icon("rate")},
        {id:"notification",label:"Notification",icon:S.icon("bell")},
        {id:"module",label:"Module",icon:S.icon("hub")},
        {id:"chats",label:"Chats",icon:S.icon("chat"),badge:true}
    ];

    function role(){return String(state.user?.role||"").toLowerCase();}
    function fullName(){return S.profileName(state.user);}
    function normalizeRoute(value){if(value==="admins")value="administrators";return ["overview","dashboard","tenant","administrators","rate","notification","module","chats","profile"].includes(value)?value:"overview";}
    function routeTitle(value){return {overview:"Overview",dashboard:"Dashboard",tenant:"Tenant",administrators:"Administrator",rate:"Rate",notification:"Notification",module:"Module",chats:"Chats",profile:"Profile"}[value]||"Overview";}
    function findTenant(id){return (state.data.tenants||[]).find(item=>String(item.id)===String(id));}
    function findCard(id){for(const tenant of state.data.tenants||[]){const card=(tenant.cards||[]).find(item=>String(item.id)===String(id));if(card)return{tenant,card};}return null;}
    function findAdministrator(id){return (state.data.administrators||[]).find(item=>String(item.id)===String(id));}

    const context = {
        state,
        get user(){return state.user;},
        get profile(){return state.profile;},
        role,
        fullName,
        navigate,
        findTenant,
        findCard,
        findAdministrator,
        refreshProfile,
        logout,
        ...S
    };

    async function mount(user, options={}){
        if(state.mounted)return;
        state.mounted=true;
        state.user=user||window.Auth?.getUser?.();
        state.preloaded=readAdminPreload();
        state.profile=state.preloaded?.profile||window.UIEnhance?.getCachedProfile?.()||state.user||null;
        renderShell();
        bindShell();
        window.ChatClient?.start?.().catch(()=>{});
        const requested=normalizeRoute(location.hash.replace(/^#/,""));
        const profilePromise=state.preloaded?.profile
            ? Promise.resolve(state.preloaded.profile).then(profile=>{window.UIEnhance?.cacheProfile?.(profile);return profile;})
            : refreshProfile(false).then(profile=>{if(profile)window.UIEnhance?.cacheProfile?.(profile);return profile;}).catch(()=>null);
        await navigate(requested||"overview",false,{loading:options.loading!==false});
        await Promise.allSettled([profilePromise]);
    }

    async function refreshProfile(refreshShell=true){
        try{const response=await window.API.get("/admin/profile");state.profile=response?.profile||null;if(state.profile)window.UIEnhance?.cacheProfile?.(state.profile);}catch(_){state.profile=state.profile||null;}
        if(refreshShell&&document.querySelector(".ec-admin-shell")){renderShell();bindShell();window.Sidebar.setActive(document.querySelector(".ec-admin-shell"),state.route);}
        return state.profile;
    }

    function renderShell(){
        const privileged=["owner","developer"].includes(role());
        const items=routes.map(item=>({...item,disabled:item.privileged&&!privileged}));
        const root=document.getElementById("app");
        root.innerHTML=`<div class="ec-shell ec-admin-shell">
            ${window.Sidebar.render(items,{title:"ElectriCredit",subtitle:`${S.formatRole(role())} Workspace`,footer:sidebarFooter()})}
            <div class="ec-main">
                <header class="ec-admin-header">
                    <div class="ec-admin-header-left"><button class="ec-header-menu" id="sidebar-toggle" type="button" aria-label="Open navigation">${S.icon("menu")}</button><div><strong id="admin-page-title">${S.escapeHTML(routeTitle(state.route))}</strong><small>${S.escapeHTML(`${fullName()} · ${S.formatRole(role())}`)}</small></div></div>
                    <div class="ec-admin-header-actions">
                        <div class="ec-admin-header-theme" aria-label="Theme mode"><button type="button" data-theme-option="light" title="Light mode">${S.icon("sun")}</button><button type="button" data-theme-option="system" title="Use device theme">${S.icon("monitor")}</button><button type="button" data-theme-option="dark" title="Dark mode">${S.icon("moon")}</button></div>
                        <button class="ec-button ec-button-secondary ec-button-sm" id="admin-developer" type="button" ${role()!=="developer"?"disabled":""} title="${role()==="developer"?"Open Developer management":"Developer access only"}">${S.icon("management")}<span>Management</span></button>
                    </div>
                </header>
                <main class="ec-content ec-admin-content" id="admin-content"></main>
                <div id="admin-footer"></div>
            </div>
        </div>`;
        document.getElementById("admin-footer").innerHTML=window.Footer?.render?.()||"";
        window.Theme?.bindControls?.(document); window.Auth?.applyRoleAccess?.(document);
        updateChatBadge(window.ChatUI?.getUnreadCount?.() || 0);
    }

    function sidebarFooter(){
        const profile=state.profile||state.user||{};
        const avatar=profile.image_url?`<img src="${S.escapeAttr(profile.image_url)}" alt="" loading="lazy">`:`<span>${S.escapeHTML(S.initials(profile))}</span>`;
        return `<div class="ec-admin-sidebar-tools"><button class="ec-admin-sidebar-profile" type="button" data-route="profile"><span class="ec-admin-sidebar-avatar">${avatar}</span><span><strong>${S.escapeHTML(S.profileName(profile))}</strong><small>${S.escapeHTML(S.formatRole(role()))}</small></span><b aria-hidden="true">${S.icon("chevron")}</b></button><button class="ec-admin-sidebar-logout" id="admin-logout" type="button">${S.icon("logout")}<span>Log out</span></button></div>`;
    }

    function bindShell(){
        window.Sidebar.bind(document.querySelector(".ec-admin-shell"),name=>navigate(name));
        document.getElementById("admin-developer")?.addEventListener("click",()=>{if(role()==="developer")location.href="developer.html";});
        document.getElementById("admin-logout")?.addEventListener("click",logout);
        window.Theme?.bindControls?.(document);
        if(!state.hashBound){state.hashBound=true;window.addEventListener("hashchange",()=>navigate(normalizeRoute(location.hash.replace(/^#/,"")),false));}
        if(!state.themeBound){state.themeBound=true;window.addEventListener("electricredit:themechange",()=>window.ElectriCreditAdminPages?.dashboard?.refresh?.(context));}
        if(!state.chatBadgeBound){state.chatBadgeBound=true;window.addEventListener("electricredit:chatunread",event=>updateChatBadge(event.detail?.total||0));}
        updateChatBadge(window.ChatUI?.getUnreadCount?.() || 0);
    }

    function updateChatBadge(total){
        total=Number(total||0);
        document.querySelectorAll('[data-route-badge="chats"], [data-chat-tab-badge="support"]').forEach(badge=>{
            badge.hidden=total<=0;
            badge.textContent=total>99?"99+":String(total);
        });
    }

    async function navigate(routeName,updateHash=true,options={}){
        routeName=normalizeRoute(routeName||"overview");
        if(routeName==="administrators"&&!["owner","developer"].includes(role())){window.Notification?.warning?.("Administrator management is available to Owner and Developer accounts.");routeName="overview";}
        window.ElectriCreditAdminPages?.dashboard?.dispose?.(context);
        state.route=routeName;
        if(updateHash&&location.hash!==`#${routeName}`)history.pushState({},"",`#${routeName}`);
        window.Sidebar.setActive(document.querySelector(".ec-admin-shell"),routeName);
        const title=document.getElementById("admin-page-title");if(title)title.textContent=routeTitle(routeName);
        document.body.classList.toggle("ec-admin-chat-page",routeName==="chats");
        const content=document.getElementById("admin-content");
        window.UIEnhance?.showInlineLoading?.(content,{title:`Opening ${routeTitle(routeName)}`,detail:"Loading the latest workspace data for this section."});
        try{
            const page=window.ElectriCreditAdminPages?.[routeName];
            if(!page)throw new Error("This page is not available.");
            await page.render(context,content);
            await window.UIEnhance?.waitForCriticalImages?.(content,{timeout:2200,margin:320});
            await window.UIEnhance?.nextPaint?.();
        }catch(error){
            content.innerHTML=S.errorPanel(error);
        }
    }


function readAdminPreload(){
    try{
        const value=JSON.parse(sessionStorage.getItem('electricredit-admin-preload-v1')||'null');
        if(!value||Date.now()-Number(value.saved_at||0)>30000)return null;
        sessionStorage.removeItem('electricredit-admin-preload-v1');
        return value;
    }catch(_){return null}
}

    async function logout(){
        const confirmed=await window.Modal.confirm({title:"Log out",message:`Are you sure you want to log out as ${S.formatRole(role())}?`,confirmText:"Log out",cancelText:"Stay signed in",danger:true});
        if(!confirmed)return;
        window.ChatClient?.stop?.(); await window.Auth?.logout?.(); location.href="index.html";
    }

    return {mount,navigate,getContext:()=>context};
})();
window.AdminApp=AdminApp;
