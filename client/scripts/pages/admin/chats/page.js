window.ElectriCreditAdminPages = window.ElectriCreditAdminPages || {};
const ECStaffTypingPeers = new Map();
window.ElectriCreditAdminPages.chats = {
    id:"chats", label:"Chats",
    async render(ctx,root){
        bindLiveEvents(ctx);
        root.innerHTML=`<section class="ec-page ec-admin-page ec-admin-chats-page">
            <div class="ec-admin-chat-mobilebar">
                <button class="ec-chat-page-action" type="button" data-admin-chat-back aria-label="Back"><svg class="ec-icon" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/><path d="M9 12h11"/></svg></button>
                <strong>Chats</strong>
                <div class="ec-admin-chat-mobile-tabs" role="tablist" aria-label="Chat type">
                    <button type="button" data-chat-tab="support" class="${ctx.state.chatTab==="support"?"is-active":""}">Visitor <span class="ec-chat-tab-badge" data-chat-tab-badge="support" hidden></span></button>
                    <button type="button" data-chat-tab="team" class="${ctx.state.chatTab==="team"?"is-active":""}">Team</button>
                </div>
            </div>
            ${ctx.pageHeading("Chats","Chats","Help visitors and stay connected with the ElectriCredit team.")}
            <div class="ec-admin-chat-tabs" role="tablist"><button type="button" data-chat-tab="support" class="${ctx.state.chatTab==="support"?"is-active":""}">Visitor <span class="ec-chat-tab-badge" data-chat-tab-badge="support" hidden></span></button><button type="button" data-chat-tab="team" class="${ctx.state.chatTab==="team"?"is-active":""}">Team room</button></div>
            <div id="admin-chat-panel"></div>
        </section>`;
        root.querySelector("[data-admin-chat-back]")?.addEventListener("click",()=>ctx.navigate("overview"));
        root.querySelectorAll("[data-chat-tab]").forEach(button=>button.addEventListener("click",async()=>{ctx.state.chatTab=button.dataset.chatTab;await this.render(ctx,root);}));
        const panel=root.querySelector("#admin-chat-panel");
        if(ctx.state.chatTab==="team") await renderTeam(ctx,panel); else await window.ChatUI.renderAdmin(panel);
        const unread=window.ChatUI?.getUnreadCount?.()||0;
        root.querySelectorAll('[data-chat-tab-badge="support"]').forEach(badge=>{badge.hidden=!unread;badge.textContent=unread>99?"99+":String(unread);});
    }
};
async function renderTeam(ctx,root){
    await window.ChatClient?.start?.().catch(()=>{});
    const response=await window.API.get("/admin/staff-chat/messages");ctx.state.staffMessages=response.messages||[];
    root.innerHTML=`<div class="ec-card ec-staff-room"><div class="ec-card-header"><div><h2 class="ec-card-title">Team room</h2><p class="ec-card-description">Shared messages for Administrators, Owners, and Developers.</p></div><span class="ec-badge ec-badge-info">Team</span></div><div class="ec-staff-room-messages" id="staff-room-messages">${teamMessages(ctx)}</div><div class="ec-chat-typing" id="staff-room-typing" aria-live="polite"></div><form class="ec-staff-room-composer" id="staff-room-form"><textarea class="ec-input" name="message" rows="1" maxlength="4000" placeholder="Write a team message…" required></textarea><button class="ec-button ec-button-primary" type="submit">Send</button></form></div>`;
    scrollTeam();const input=root.querySelector('textarea[name="message"]');input?.addEventListener("keydown",e=>{if(e.key==="Enter"&&!e.shiftKey){e.preventDefault();root.querySelector("#staff-room-form")?.requestSubmit();}});
    let typing=false,timer=null;const sendTyping=active=>window.API.post("/admin/staff-chat/typing",{typing:active}).catch(()=>{});
    input?.addEventListener("input",()=>{const active=Boolean(input.value.trim());if(active&&!typing){typing=true;sendTyping(true);}clearTimeout(timer);timer=setTimeout(()=>{if(typing){typing=false;sendTyping(false);}},1200);});
    root.querySelector("#staff-room-form")?.addEventListener("submit",async e=>{e.preventDefault();const field=e.currentTarget.elements.message,button=e.currentTarget.querySelector('button[type="submit"]'),text=field.value.trim();if(!text)return;ctx.setBusy(button,true);try{const result=await window.API.post("/admin/staff-chat/messages",{message:text});field.value="";if(typing){typing=false;sendTyping(false);}addTeamMessage(ctx,result.message);}catch(err){window.Notification?.error?.(err.message);}finally{ctx.setBusy(button,false);field.focus();}});
}
function teamMessages(ctx){if(!ctx.state.staffMessages.length)return`<div class="ec-chat-empty ec-chat-empty-large"><span class="ec-chat-empty-icon">${ctx.icon("chat")}</span><strong>No team messages yet</strong><p>Start the room with a short message.</p></div>`;const current=Number(ctx.user?.id||0);return ctx.state.staffMessages.map(message=>`<article class="ec-staff-message ${Number(message.user_id)===current?"is-own":""}">${ctx.avatarHTML({first_name:message.first_name,last_name:message.last_name,image_url:message.image_url},"ec-staff-message-avatar")}<div><header><strong>${ctx.escapeHTML(`${message.first_name||""} ${message.last_name||""}`.trim())}</strong><span>${ctx.escapeHTML(ctx.formatRole(message.role))}</span><time>${formatMessageTime(message.created_at)}</time></header><div class="ec-chat-markdown">${window.ChatUI?.renderMarkdown?.(message.message||"")||ctx.escapeHTML(message.message||"")}</div></div></article>`).join("");}
function bindLiveEvents(ctx){if(window.__ecStaffGroupBound)return;window.__ecStaffGroupBound=true;window.addEventListener("electricredit:staffgroupmessage",e=>addTeamMessage(ctx,e.detail?.message||e.detail));window.addEventListener("electricredit:staffgrouptyping",e=>updateTyping(ctx,e.detail));}
function addTeamMessage(ctx,message){if(!message?.id)return;if(!ctx.state.staffMessages.some(item=>String(item.id)===String(message.id)))ctx.state.staffMessages.push(message);if(ctx.state.staffMessages.length>150)ctx.state.staffMessages=ctx.state.staffMessages.slice(-150);if(ctx.state.route==="chats"&&ctx.state.chatTab==="team"){const list=document.getElementById("staff-room-messages");if(list){list.innerHTML=teamMessages(ctx);scrollTeam();}}}
function updateTyping(ctx,detail){if(!detail||Number(detail.user_id)===Number(ctx.user?.id))return;const key=String(detail.user_id||detail.name||"staff");clearTimeout(ECStaffTypingPeers.get(key)?.timer);if(!detail.typing)ECStaffTypingPeers.delete(key);else ECStaffTypingPeers.set(key,{name:detail.name||ctx.formatRole(detail.role)||"Someone",timer:setTimeout(()=>{ECStaffTypingPeers.delete(key);renderTyping();},1800)});renderTyping();}
function renderTyping(){const el=document.getElementById("staff-room-typing");if(!el)return;const names=[...ECStaffTypingPeers.values()].map(i=>i.name);el.textContent=names.length?`${names.slice(0,2).join(" and ")}${names.length>2?` +${names.length-2}`:""} ${names.length===1?"is":"are"} typing…`:"";}
function scrollTeam(){const list=document.getElementById("staff-room-messages");if(list)list.scrollTop=list.scrollHeight;}
function formatMessageTime(unix){const d=new Date(Number(unix||0)*1000);return Number.isNaN(d.getTime())?"":d.toLocaleString([], {month:"short",day:"numeric",hour:"2-digit",minute:"2-digit"});}
