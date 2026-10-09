/** Shared presentation helpers for the ElectriCredit Administrator workspace. */
const AdminShared = (() => {
    const phoneCountries = [
        {code:"+63",label:"PH +63"},{code:"+1",label:"US/CA +1"},{code:"+65",label:"SG +65"},
        {code:"+60",label:"MY +60"},{code:"+61",label:"AU +61"},{code:"+81",label:"JP +81"},{code:"+82",label:"KR +82"}
    ];

    function escapeHTML(value) { return String(value ?? "").replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;"); }
    function escapeAttr(value) { return escapeHTML(value); }
    function formatRole(value) {
        value = String(value || "").toLowerCase();
        if (value === "admin") return "Administrator";
        if (value === "owner") return "Owner";
        if (value === "developer") return "Developer";
        return value ? value.charAt(0).toUpperCase() + value.slice(1) : "";
    }
    function peso(value) {
        if (value === null || value === undefined || value === "") return "₱—";
        const parsed = Number(value);
        if (!Number.isFinite(parsed)) return "₱—";
        return new Intl.NumberFormat("en-PH", { style:"currency", currency:"PHP", minimumFractionDigits:2, maximumFractionDigits:5 }).format(parsed);
    }
    function number(value, decimals = 0) {
        const parsed = Number(value || 0);
        return Number.isFinite(parsed) ? parsed.toLocaleString("en-PH", { minimumFractionDigits:decimals, maximumFractionDigits:decimals }) : "0";
    }
    function formatDateTime(unix) {
        const date = new Date(Number(unix || 0) * 1000);
        return !unix || Number.isNaN(date.getTime()) ? "—" : date.toLocaleString("en-PH", { month:"short", day:"numeric", year:"numeric", hour:"2-digit", minute:"2-digit" });
    }
    function todayISO() { const now = new Date(); const local = new Date(now.getTime() - now.getTimezoneOffset()*60000); return local.toISOString().slice(0,10); }
    function tenantAge(birthDate) {
        if (!birthDate) return "—";
        const date = new Date(`${birthDate}T00:00:00`); if (Number.isNaN(date.getTime())) return "—";
        const now = new Date(); let age = now.getFullYear()-date.getFullYear();
        if (now.getMonth() < date.getMonth() || (now.getMonth() === date.getMonth() && now.getDate() < date.getDate())) age--;
        return age >= 0 ? `${age} years old` : "—";
    }
    function formatGender(value) { return ({female:"Female",male:"Male",other:"Other",prefer_not_to_say:"Prefer not to say"})[String(value||"").toLowerCase()] || "—"; }
    function genderOptions(selected="") { return [["female","Female"],["male","Male"],["other","Other"],["prefer_not_to_say","Prefer not to say"]].map(([v,l])=>`<option value="${v}" ${v===selected?"selected":""}>${l}</option>`).join(""); }
    function countryCodeOptions(selected="+63") { return phoneCountries.map(item=>`<option value="${item.code}" ${item.code===selected?"selected":""}>${item.label}</option>`).join(""); }
    function splitPhone(value) {
        const phone=String(value||"").replace(/[\s()-]/g,"");
        const match=[...phoneCountries].sort((a,b)=>b.code.length-a.code.length).find(item=>phone.startsWith(item.code));
        return match ? {code:match.code,number:phone.slice(match.code.length).replace(/\D/g,"")} : {code:"+63",number:phone.replace(/\D/g,"").replace(/^63/,"")};
    }
    function formatPhone(value) { const phone=String(value||"").trim(); if(!phone)return""; const p=splitPhone(phone); return `${p.code} ${p.number}`; }
    function statusBadge(status) { return status === "active" ? "ec-badge-success" : ["disabled","blocked","lost"].includes(status) ? "ec-badge-danger" : "ec-badge-warning"; }
    function options(values, selected) { return values.map(v=>`<option value="${escapeAttr(v)}" ${v===selected?"selected":""}>${escapeHTML(formatRole(v))}</option>`).join(""); }
    function pageHeading(title,pill,description,action="") { return `<div class="ec-admin-page-head"><div><span class="ec-admin-section-pill">${escapeHTML(pill)}</span><h1>${escapeHTML(title)}</h1><p>${escapeHTML(description)}</p></div>${action?`<div class="ec-admin-page-action">${action}</div>`:""}</div>`; }
    function statCard(title,value,meta,iconName,raw=false) { return `<article class="ec-card ec-admin-stat-card"><div class="ec-card-body"><span class="ec-admin-stat-icon">${icon(iconName)}</span><div><span>${escapeHTML(title)}</span><strong>${raw?escapeHTML(value):Number(value||0).toLocaleString("en-PH")}</strong><small>${escapeHTML(meta)}</small></div></div></article>`; }
    function emptyState(iconName,title,text) { return `<div class="ec-card ec-admin-empty"><div class="ec-card-body"><span>${icon(iconName)}</span><strong>${escapeHTML(title)}</strong><p>${escapeHTML(text)}</p></div></div>`; }
    function errorPanel(error) { return `<div class="ec-card ec-admin-error"><div class="ec-card-body"><strong>Unable to load this page</strong><p>${escapeHTML(error?.message||"Something went wrong.")}</p><button class="ec-button ec-button-secondary" onclick="location.reload()">Reload</button></div></div>`; }
    function formObject(form) { return Object.fromEntries(new FormData(form).entries()); }
    function setBusy(button,busy) { if(!button)return; button.disabled=busy; button.classList.toggle("is-loading",busy); if(busy){button.dataset.label=button.innerHTML; button.innerHTML=`<span class="ec-admin-button-spinner"></span><span>Please wait…</span>`;} else if(button.dataset.label){button.innerHTML=button.dataset.label; delete button.dataset.label;} }
    function bindEyes(root) { root.querySelectorAll?.("[data-eye]").forEach(button=>button.addEventListener("click",()=>{const input=button.parentElement.querySelector("input");if(!input)return;input.type=input.type==="password"?"text":"password";button.innerHTML=icon(input.type==="password"?"eye":"eyeoff");})); }
    function avatarHTML(person,className="ec-admin-avatar") { const name=`${person?.first_name||""} ${person?.last_name||""}`.trim(); return person?.image_url?`<span class="${className}"><img src="${escapeAttr(person.image_url)}" alt="${escapeAttr(name)}" loading="lazy"></span>`:`<span class="${className}">${icon("tenant")}</span>`; }
    function profileName(user={}) { return `${user?.first_name||""} ${user?.last_name||""}`.trim() || "ElectriCredit User"; }
    function initials(user={}) { return `${user?.first_name?.[0]||"E"}${user?.last_name?.[0]||""}`.toUpperCase(); }
    function staffLinksHTML(links={}) { const items=[["website","Website"],["facebook","Facebook"],["github","GitHub"],["linkedin","LinkedIn"]].filter(([key])=>links?.[key]); return items.length?`<div class="ec-admin-staff-links">${items.map(([key,label])=>`<a href="${escapeAttr(links[key])}" target="_blank" rel="noopener noreferrer">${label}</a>`).join("")}</div>`:""; }
    function profileLinkField(label,name,value) { return `<label class="ec-form-group"><span class="ec-label">${escapeHTML(label)}</span><input class="ec-input" type="url" name="link_${name}" value="${escapeAttr(value||"")}" placeholder="https://"></label>`; }
    function passwordField(label,name,autocomplete) { return `<label class="ec-form-group"><span class="ec-label">${escapeHTML(label)}</span><div class="ec-admin-password"><input class="ec-input" type="password" name="${name}" required minlength="${name==="current_password"?1:8}" autocomplete="${autocomplete}"><button type="button" data-eye aria-label="Show password">${icon("eye")}</button></div></label>`; }
    function challengeModal(ctx,result,title,text){
        const expected=String(result?.challenge||"");
        const length=Number(result?.length||expected.length||12);
        return new Promise(resolve=>{
            let done=false;
            const modal=window.Modal.open({
                title,
                confirmText:"Confirm action",
                cancelText:"Cancel",
                content:`<div class="ec-action-challenge" data-action-challenge>
                    <div class="ec-action-challenge-note"><span>${ctx.icon?.("alert")||""}</span><p>${escapeHTML(text)}</p></div>
                    <div class="ec-action-challenge-code" data-challenge-code aria-label="Security challenge">${[...expected].map(ch=>`<span>${escapeHTML(ch)}</span>`).join("")}</div>
                    <div class="ec-action-challenge-status"><span data-challenge-status>Type the challenge exactly</span><strong data-challenge-percent>0%</strong></div>
                    <div class="ec-action-challenge-progress" aria-hidden="true"><i data-challenge-progress></i></div>
                    <label class="ec-form-group"><span class="ec-label">Action challenge</span><input class="ec-input" data-management-challenge autocomplete="off" autocapitalize="off" spellcheck="false" maxlength="${length}" placeholder="Type the code shown above"></label>
                    <small class="ec-help">This challenge expires in 5 minutes and is accepted once.</small>
                </div>`,
                onConfirm(){
                    const value=modal.querySelector("[data-management-challenge]")?.value||"";
                    if(value!==expected){window.Notification?.warning?.("The challenge does not match yet.");return false}
                    done=true;resolve(value);return true;
                },
                beforeClose(){if(!done){done=true;resolve(null)}return true}
            });
            const input=modal.querySelector("[data-management-challenge]");
            const code=modal.querySelector("[data-challenge-code]");
            const bar=modal.querySelector("[data-challenge-progress]");
            const label=modal.querySelector("[data-challenge-status]");
            const pct=modal.querySelector("[data-challenge-percent]");
            const stop=e=>{e.preventDefault();return false};
            ["copy","cut","contextmenu","dragstart","selectstart"].forEach(type=>code?.addEventListener(type,stop));
            code?.setAttribute("draggable","false");
            input?.addEventListener("paste",stop);input?.addEventListener("drop",stop);
            const update=()=>{
                const value=input?.value||"";let matched=0;
                for(let i=0;i<Math.min(value.length,expected.length);i++){if(value[i]!==expected[i])break;matched++}
                const exact=value===expected,mismatch=value.length>matched,progress=Math.round((matched/Math.max(expected.length,1))*100);
                if(bar){bar.style.width=`${progress}%`;bar.classList.toggle("is-error",mismatch);bar.classList.toggle("is-match",exact)}
                if(pct)pct.textContent=`${progress}%`;
                if(label)label.textContent=exact?"Challenge matched":mismatch?"Character does not match":"Keep typing…";
            };
            input?.addEventListener("input",update);setTimeout(()=>input?.focus(),60);
        });
    }
    async function actionChallenge(ctx,action,title,text,endpoint="/admin/challenge"){
        const result=await window.API.post(endpoint,{action});
        return challengeModal(ctx,result,title,text);
    }
    function getCSS(name,fallback){return getComputedStyle(document.documentElement).getPropertyValue(name).trim()||fallback;}
    function shortName(name){const parts=String(name||"").trim().split(/\s+/);return parts.length>1?`${parts[0]} ${parts[parts.length-1].charAt(0)}.`:parts[0]||"Tenant";}

    function imagePickerHTML(prefix,imageURL,label="Profile") { return `<div class="ec-admin-image-picker" data-image-picker="${prefix}"><div class="ec-admin-image-preview" data-image-preview>${imageURL?`<img src="${escapeAttr(imageURL)}" alt="">`:icon("tenant")}</div><div><strong>${escapeHTML(label)} image</strong><small>Upload a photo or use the device camera.</small><div class="ec-admin-image-actions"><label class="ec-button ec-button-secondary ec-button-sm">${icon("image")} Upload<input type="file" accept="image/*" data-image-file hidden></label><button class="ec-button ec-button-secondary ec-button-sm" type="button" data-image-camera>${icon("camera")} Camera</button></div></div><input type="hidden" data-image-value></div>`; }
    function bindImagePicker(root,prefix,onChange) {
        const picker=root.querySelector(`[data-image-picker="${prefix}"]`); if(!picker)return;
        const preview=picker.querySelector("[data-image-preview]"); const hidden=picker.querySelector("[data-image-value]");
        const apply=data=>{hidden.value=data||""; preview.innerHTML=data?`<img src="${escapeAttr(data)}" alt="">`:icon("tenant"); onChange?.(data||"");};
        picker.querySelector("[data-image-file]")?.addEventListener("change",async e=>{const file=e.target.files?.[0];if(!file)return;try{apply(await resizeImage(file));}catch(err){window.Notification?.error?.(err.message);}});
        picker.querySelector("[data-image-camera]")?.addEventListener("click",()=>openCamera(apply));
    }
    async function openCamera(apply) {
        if(!navigator.mediaDevices?.getUserMedia){document.querySelector('[data-image-file]')?.click();return;}
        let stream;
        const overlay=window.Modal.open({title:"Take profile photo",content:`<div class="ec-admin-camera"><video autoplay playsinline data-camera-preview></video><canvas data-camera-canvas hidden></canvas></div>`,confirmText:"Take photo",cancelText:"Cancel",onConfirm:async()=>{
            const video=overlay.querySelector("[data-camera-preview]"); const canvas=overlay.querySelector("[data-camera-canvas]");
            if(!video?.videoWidth) return false; const max=640; const scale=Math.min(1,max/video.videoWidth); canvas.width=Math.round(video.videoWidth*scale); canvas.height=Math.round(video.videoHeight*scale); canvas.getContext("2d").drawImage(video,0,0,canvas.width,canvas.height); apply(canvas.toDataURL("image/jpeg",.82)); return true;
        },onClose:()=>stream?.getTracks().forEach(t=>t.stop())});
        try{stream=await navigator.mediaDevices.getUserMedia({video:{facingMode:"user"},audio:false}); overlay.querySelector("[data-camera-preview]").srcObject=stream;}catch(err){overlay.remove();window.Notification?.error?.("Camera access was not available.");}
    }
    function resizeImage(file){return new Promise((resolve,reject)=>{if(!file.type.startsWith("image/"))return reject(new Error("Choose an image file."));if(file.size>8_000_000)return reject(new Error("Choose an image smaller than 8 MB."));const img=new Image();const url=URL.createObjectURL(file);img.onload=()=>{const max=640,scale=Math.min(1,max/Math.max(img.width,img.height));const c=document.createElement("canvas");c.width=Math.round(img.width*scale);c.height=Math.round(img.height*scale);c.getContext("2d").drawImage(img,0,0,c.width,c.height);URL.revokeObjectURL(url);resolve(c.toDataURL("image/jpeg",.82));};img.onerror=()=>{URL.revokeObjectURL(url);reject(new Error("Unable to read this image."));};img.src=url;});}

    function icon(name) {
        const paths={
            menu:'<path d="M4 7h16M4 12h16M4 17h16"/>', home:'<path d="M3 11.5 12 4l9 7.5"/><path d="M5.5 10.5V20h13v-9.5"/><path d="M9.5 20v-6h5v6"/>', chart:'<path d="M4 19V9"/><path d="M10 19V5"/><path d="M16 19v-7"/><path d="M22 19H2"/>', tenant:'<circle cx="12" cy="8" r="3.5"/><path d="M5 20c.7-4 3-6 7-6s6.3 2 7 6"/>', administrator:'<path d="M12 3 5 6v5c0 4.6 2.9 8 7 10 4.1-2 7-5.4 7-10V6l-7-3Z"/><circle cx="12" cy="10" r="2"/><path d="M8.8 16c.7-1.9 1.8-2.8 3.2-2.8s2.5.9 3.2 2.8"/>', owner:'<path d="m4 9 3 2 5-6 5 6 3-2-1.5 9h-13L4 9Z"/><path d="M7 21h10"/>', rate:'<path d="M4 7h16v12H4z"/><path d="M8 11h8M8 15h5"/><path d="M7 4h10"/>', bell:'<path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9"/><path d="M10 21h4"/>', chat:'<path d="M4 5h16v11H9l-5 4V5Z"/><path d="M8 9h8M8 12h5"/>', sun:'<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>', moon:'<path d="M20 15.5A8.5 8.5 0 0 1 8.5 4 8.5 8.5 0 1 0 20 15.5Z"/>', monitor:'<rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8M12 17v4"/>', management:'<path d="M4 6h10M18 6h2M4 12h2M10 12h10M4 18h7M15 18h5"/><circle cx="16" cy="6" r="2"/><circle cx="8" cy="12" r="2"/><circle cx="13" cy="18" r="2"/>', chevron:'<path d="m9 18 6-6-6-6"/>', logout:'<path d="M10 4H5v16h5"/><path d="M14 8l4 4-4 4M18 12H9"/>', plus:'<path d="M12 5v14M5 12h14"/>', search:'<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>', card:'<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 9h18M7 15h4"/>', edit:'<path d="M4 20h4l11-11-4-4L4 16v4Z"/><path d="m13.5 6.5 4 4"/>', trash:'<path d="M4 7h16M9 3h6l1 4H8l1-4ZM7 7l1 14h8l1-14M10 11v6M14 11v6"/>', save:'<path d="M5 3h12l2 2v16H5V3Z"/><path d="M8 3v6h8V3M8 21v-7h8v7"/>', alert:'<path d="M12 3 2.5 20h19L12 3Z"/><path d="M12 9v5M12 17h.01"/>', eye:'<path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12Z"/><circle cx="12" cy="12" r="2.5"/>', eyeoff:'<path d="m3 3 18 18"/><path d="M10.6 6.2A11 11 0 0 1 12 6c6.5 0 10 6 10 6a17 17 0 0 1-3 3.8M6.3 6.3C3.5 8.1 2 12 2 12s3.5 6 10 6a10 10 0 0 0 4-.8"/>', bolt:'<path d="M13 2 5 14h6l-1 8 9-13h-6V2Z"/>', hub:'<rect x="5" y="5" width="14" height="14" rx="2"/><path d="M9 9h6v6H9zM9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/>', registry:'<path d="M5 3H3v4M19 3h2v4M5 21H3v-4M19 21h2v-4"/><rect x="7" y="7" width="10" height="10" rx="2"/><path d="M10 10h4v4h-4z"/>', clock:'<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>', image:'<rect x="3" y="5" width="18" height="14" rx="2"/><circle cx="9" cy="10" r="2"/><path d="m4 17 5-4 3 3 2-2 6 5"/>', camera:'<path d="M4 7h4l1.5-2h5L16 7h4v12H4z"/><circle cx="12" cy="13" r="4"/>', phone:'<path d="M7 4 4 6c0 7.7 6.3 14 14 14l2-3-4-3-2 2c-3.1-1.2-5.8-3.9-7-7l2-2-2-3Z"/>', mail:'<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m4 7 8 6 8-6"/>', database:'<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v7c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 12v7c0 1.7 3.6 3 8 3s8-1.3 8-3v-7"/>', download:'<path d="M12 3v12M7 10l5 5 5-5"/><path d="M5 20h14"/>', logs:'<path d="M6 3h12v18H6z"/><path d="M9 8h6M9 12h6M9 16h4"/>', wifi:'<path d="M5 9a10 10 0 0 1 14 0M8 12a6 6 0 0 1 8 0M10.5 15a2.5 2.5 0 0 1 3 0"/><circle cx="12" cy="18" r="1"/>', developer:'<path d="m8 9-4 3 4 3M16 9l4 3-4 3M14 5l-4 14"/>', server:'<rect x="4" y="4" width="16" height="6" rx="1"/><rect x="4" y="14" width="16" height="6" rx="1"/><path d="M8 7h.01M8 17h.01M12 7h5M12 17h5"/>'
        };
        return `<svg class="ec-icon" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${paths[name]||paths.chart}</svg>`;
    }

    return { phoneCountries,escapeHTML,escapeAttr,formatRole,peso,number,formatDateTime,todayISO,tenantAge,formatGender,genderOptions,countryCodeOptions,splitPhone,formatPhone,statusBadge,options,pageHeading,statCard,emptyState,errorPanel,formObject,setBusy,bindEyes,avatarHTML,profileName,initials,staffLinksHTML,profileLinkField,passwordField,challengeModal,actionChallenge,getCSS,shortName,imagePickerHTML,bindImagePicker,resizeImage,icon };
})();
window.AdminShared = AdminShared;
