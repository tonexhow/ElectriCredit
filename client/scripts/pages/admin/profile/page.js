window.ElectriCreditAdminPages = window.ElectriCreditAdminPages || {};
(() => {
    const page = {
        id: "profile", label: "Profile",
        async render(ctx, root) {
            const [profileResponse, emailStatus] = await Promise.all([
                window.API.get("/admin/profile"),
                window.API.get("/admin/profile/email/status").catch(() => ({ available:false }))
            ]);
            const profile = profileResponse.profile || {};
            ctx.state.profile = profile;
            const links = profile.links || {};
            root.innerHTML = `<section class="ec-page ec-admin-page ec-profile-page">
                ${ctx.pageHeading("Profile", "My profile", "Manage your personal information and account security.")}
                <div class="ec-admin-profile-hero ec-card">
                    ${ctx.avatarHTML(profile,"ec-admin-profile-photo")}
                    <div><span class="ec-admin-section-pill">${ctx.escapeHTML(ctx.formatRole(profile.role || ctx.role()))}</span><h2>${ctx.escapeHTML(ctx.profileName(profile))}</h2><p>${ctx.escapeHTML(profile.email || "")}</p></div>
                </div>
                <form class="ec-card ec-admin-profile-details" id="profile-details-form">
                    <div class="ec-card-header"><div><h2 class="ec-card-title">Personal details</h2><p class="ec-card-description">Profile changes are protected by your current password.</p></div></div>
                    <div class="ec-card-body ec-stack">
                        ${ctx.imagePickerHTML("profile", profile.image_url, "Profile")}
                        <div class="ec-form-grid"><label class="ec-form-group"><span class="ec-label">First name</span><input class="ec-input" name="first_name" required value="${ctx.escapeAttr(profile.first_name||"")}"></label><label class="ec-form-group"><span class="ec-label">Last name</span><input class="ec-input" name="last_name" required value="${ctx.escapeAttr(profile.last_name||"")}"></label></div>
                        <div class="ec-form-grid"><label class="ec-form-group"><span class="ec-label">Birth date</span><input class="ec-input" type="date" name="birth_date" value="${ctx.escapeAttr(profile.birth_date||"")}"></label><label class="ec-form-group"><span class="ec-label">Phone</span><input class="ec-input" inputmode="tel" name="phone" value="${ctx.escapeAttr(profile.phone||"")}" placeholder="+639938859567"></label></div>
                        <fieldset class="ec-admin-links-fieldset"><legend>Links</legend><div class="ec-form-grid">${ctx.profileLinkField("Facebook","facebook",links.facebook)}${ctx.profileLinkField("GitHub","github",links.github)}${ctx.profileLinkField("LinkedIn","linkedin",links.linkedin)}${ctx.profileLinkField("Website","website",links.website)}${ctx.profileLinkField("Telegram","telegram",links.telegram)}${ctx.profileLinkField("WhatsApp","whatsapp",links.whatsapp)}</div></fieldset>
                        ${ctx.passwordField("Current password","current_password","current-password")}
                    </div>
                    <div class="ec-card-footer"><button class="ec-button ec-button-primary" type="submit">${ctx.icon("save")} Save profile</button></div>
                </form>
                <div class="ec-admin-security-grid">
                    <form class="ec-card" id="profile-password-form"><div class="ec-card-header"><div><h2 class="ec-card-title">Password</h2><p class="ec-card-description">Change the password used to protect this account.</p></div></div><div class="ec-card-body ec-stack">${ctx.passwordField("Current password","current_password","current-password")}${ctx.passwordField("New password","new_password","new-password")}${ctx.passwordField("Confirm new password","confirm_password","new-password")}</div><div class="ec-card-footer"><button class="ec-button ec-button-primary" type="submit">Change password</button></div></form>
                    <form class="ec-card" id="profile-email-form"><div class="ec-card-header"><div><h2 class="ec-card-title">Email</h2><p class="ec-card-description">A verification code is required before a new email becomes active.</p></div></div><div class="ec-card-body ec-stack"><label class="ec-form-group"><span class="ec-label">Current email</span><input class="ec-input" value="${ctx.escapeAttr(profile.email||"")}" disabled></label><label class="ec-form-group"><span class="ec-label">New email</span><input class="ec-input" type="email" name="email" required placeholder="name@example.com"></label>${ctx.passwordField("Current password","current_password","current-password")}<div class="ec-profile-email-otp" id="profile-email-otp" hidden><span class="ec-label">6-digit verification code</span><div class="ec-otp-row">${Array.from({length:6},(_,i)=>`<input class="ec-input" inputmode="numeric" pattern="[0-9]*" maxlength="1" data-profile-otp="${i}" aria-label="Digit ${i+1}">`).join("")}</div><button class="ec-button ec-button-primary" type="button" id="profile-email-verify">Verify & change email</button></div></div><div class="ec-card-footer"><button class="ec-button ec-button-secondary" type="submit" ${emailStatus.available?"":"disabled"}>Send verification code</button></div></form>
                </div>
            </section>`;
            bind(ctx, root, Boolean(emailStatus.available));
        }
    };

    function bind(ctx, root, emailAvailable) {
        ctx.bindEyes(root);
        const details = root.querySelector("#profile-details-form");
        ctx.bindImagePicker(root,"profile",data=>{const hidden=root.querySelector('[data-image-picker="profile"] [data-image-value]');if(hidden)hidden.name="image_data";});
        const imageValue=root.querySelector('[data-image-picker="profile"] [data-image-value]'); if(imageValue) imageValue.name="image_data";
        details?.addEventListener("submit",async event=>{event.preventDefault();const button=details.querySelector('button[type="submit"]');ctx.setBusy(button,true);try{const body=ctx.formObject(details);body.links={};["facebook","github","linkedin","website","telegram","whatsapp"].forEach(name=>{const value=body[`link_${name}`]?.trim();if(value)body.links[name]=value;delete body[`link_${name}`];});await window.API.put("/admin/profile",body);window.Notification?.success?.("Profile updated.");ctx.state.user=await window.Auth.loadSession();await ctx.refreshProfile();await page.render(ctx,document.getElementById("admin-content"));}catch(error){window.Notification?.error?.(error.message);}finally{ctx.setBusy(button,false);}});
        const password=root.querySelector("#profile-password-form");password?.addEventListener("submit",async event=>{event.preventDefault();const button=password.querySelector('button[type="submit"]');ctx.setBusy(button,true);try{await window.API.put("/admin/profile/password",ctx.formObject(password));password.reset();window.Notification?.success?.("Password changed. Use the new password for future protected changes.");}catch(error){window.Notification?.error?.(error.message);}finally{ctx.setBusy(button,false);}});
        bindEmail(ctx,root,emailAvailable);
    }

    function bindEmail(ctx,root,available){const form=root.querySelector("#profile-email-form"),wrap=root.querySelector("#profile-email-otp"),verify=root.querySelector("#profile-email-verify"),inputs=[...root.querySelectorAll("[data-profile-otp]")];bindOTP(inputs);form?.addEventListener("submit",async event=>{event.preventDefault();if(!available){window.Notification?.warning?.("Email verification needs an internet connection.");return;}const button=form.querySelector('button[type="submit"]');ctx.setBusy(button,true);try{await window.API.post("/admin/profile/email/request",{email:form.elements.email.value,current_password:form.elements.current_password.value});wrap.hidden=false;inputs[0]?.focus();window.Notification?.success?.("Verification code sent. It expires in 15 minutes.");}catch(error){window.Notification?.error?.(error.message);}finally{ctx.setBusy(button,false);}});verify?.addEventListener("click",async()=>{const otp=inputs.map(x=>x.value).join("");if(!/^\d{6}$/.test(otp)){window.Notification?.warning?.("Enter the complete 6-digit code.");return;}ctx.setBusy(verify,true);try{await window.API.post("/admin/profile/email/verify",{email:form.elements.email.value,otp,current_password:form.elements.current_password.value});window.Notification?.success?.("Email verified and updated.");ctx.state.user=await window.Auth.loadSession();await ctx.refreshProfile();await page.render(ctx,document.getElementById("admin-content"));}catch(error){window.Notification?.error?.(error.message);}finally{ctx.setBusy(verify,false);}});}
    function bindOTP(inputs){inputs.forEach((input,index)=>{input.addEventListener("input",()=>{input.value=input.value.replace(/\D/g,"").slice(-1);if(input.value)inputs[index+1]?.focus();});input.addEventListener("keydown",e=>{if(e.key==="Backspace"&&!input.value)inputs[index-1]?.focus();});input.addEventListener("paste",e=>{const digits=e.clipboardData?.getData("text")?.replace(/\D/g,"").slice(0,6);if(!digits)return;e.preventDefault();digits.split("").forEach((d,i)=>{if(inputs[i])inputs[i].value=d;});inputs[Math.min(digits.length,6)-1]?.focus();});});}

    window.ElectriCreditAdminPages.profile = page;
})();
