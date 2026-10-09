window.ElectriCreditDeveloperPages=window.ElectriCreditDeveloperPages||{};

window.ElectriCreditDeveloperPages.network={
 async render(ctx,root){
  const data=await window.API.get(`/developer/network`);
  const ifaces=data.interfaces||[],wifi=data.wireless_networks||[],hotspot=data.hotspot||{},saved=data.hotspot_config||{};
  const setup=data.setup_network||{},scanState=data.wifi_scan||{},connectState=data.wifi_connect||{};
  const ethernet=ifaces.filter(i=>i.kind==='ethernet'),usb=ifaces.filter(i=>i.kind==='usb'),wireless=ifaces.filter(i=>i.kind==='wireless');
  const currentMode=hotspot.enabled?'hotspot':data.connected_wifi?'wifi':'offline';

  root.innerHTML=`<section class="ec-page ec-admin-page ec-developer-page">
   ${ctx.pageHeading('Management','Network','Control the Raspberry Pi local hotspot, hand over to Wi-Fi, and prepare the network used by ElectriCredit modules.',`<button class="ec-management-icon-button" data-network-refresh type="button" title="Scan and refresh" data-tooltip="Scan and refresh">${scanIcon()}</button>`)}

   <div class="ec-network-status-grid">
    ${connectionCard(ctx,'Ethernet','server',ethernet)}
    ${connectionCard(ctx,'USB tethering','phone',usb)}
    ${connectionCard(ctx,'Wi-Fi radio','wifi',wireless,hotspot.enabled?`Hosting ${hotspot.ssid||saved.ssid||'local hotspot'}`:data.connected_wifi?`Connected to ${data.connected_wifi}`:'Not connected')}
   </div>

   <article class="ec-network-mode-card">
    <div class="ec-network-mode-copy">
     <span class="ec-badge">Raspberry Pi network mode</span>
     <h3>${currentMode==='hotspot'?'Local hotspot active':currentMode==='wifi'?'Connected to Wi-Fi':'Wireless radio is idle'}</h3>
     <p>${currentMode==='hotspot'?'Hub, Registry, phones, and laptops can use the dedicated ElectriCredit network.':currentMode==='wifi'?`The server Wi-Fi radio is currently joined to ${ctx.escapeHTML(data.connected_wifi||'the selected network')}.`:'Start the saved hotspot or scan for a Wi-Fi network.'}</p>
    </div>
    <div class="ec-network-mode-flow" aria-label="Hotspot and Wi-Fi handover">
     <span class="${currentMode==='hotspot'?'is-current':''}">${powerIcon()} Hotspot</span><i>⇄</i><span class="${currentMode==='wifi'?'is-current':''}">${ctx.icon('wifi')} Wi-Fi</span>
    </div>
   </article>

   <article class="ec-management-card ec-hotspot-card">
    <div class="ec-management-card-head">
     <div><strong>ElectriCredit local hotspot</strong><small>${hotspot.enabled?'Running now':`Saved profile · ${ctx.escapeHTML(saved.ssid||'ElectriCredit Server')}`}</small></div>
     <div class="ec-hotspot-head-actions">
      <span class="ec-badge ${hotspot.enabled?'ec-badge-success':'ec-badge-warning'}">${hotspot.enabled?'On':'Off'}</span>
      <span class="ec-badge ${saved.boot_enabled?'ec-badge-success':''}">${saved.boot_enabled?'Starts on boot':'Boot start off'}</span>
      ${data.hotspot_control?hotspot.enabled?`<button class="ec-management-icon-button" type="button" data-hotspot-edit title="Edit hotspot" data-tooltip="Edit hotspot">${ctx.icon('edit')}</button><button class="ec-management-icon-button is-danger" type="button" data-hotspot-stop title="Switch hotspot off" data-tooltip="Switch hotspot off">${powerIcon()}</button>`:`<button class="ec-button ec-button-primary ec-button-sm" type="button" data-hotspot-start>${powerIcon()}<span>Start hotspot</span></button>`:''}
     </div>
    </div>
    <div class="ec-management-card-body">
     ${hotspot.enabled?hotspotPanel(ctx,hotspot,data):savedHotspotPanel(ctx,saved)}
     ${!data.hotspot_control?`<div class="ec-network-warning">Hotspot controls are unavailable on this host. Raspberry Pi deployment should use NetworkManager / nmcli.</div>`:''}
    </div>
   </article>

   <article class="ec-management-card ec-network-section-card">
    <div class="ec-management-card-head"><div><strong>Module setup network</strong><small>${setup.mode==='hotspot'?'Using the ElectriCredit hotspot':setup.mode==='wifi'?`Using current Wi-Fi · ${ctx.escapeHTML(setup.ssid||'')}`:'No usable local network detected'}</small></div>${ctx.icon(setup.mode==='hotspot'?'hub':'wifi')}</div>
    <div class="ec-management-card-body">${setupNetworkCard(ctx,setup)}</div>
   </article>

   <article class="ec-management-card ec-network-section-card">
    <div class="ec-management-card-head">
     <div><strong>Nearby Wi-Fi</strong><small>${scanSummary(ctx,scanState,data.connected_wifi,wifi.length)}</small></div>
     <button class="ec-button ec-button-secondary ec-button-sm" type="button" data-wifi-scan>${scanIcon()}<span>Scan Wi-Fi</span></button>
    </div>
    <div class="ec-management-card-body">
     ${networkOperationBanner(ctx,scanState,connectState)}
     ${hotspot.enabled?`<div class="ec-network-handover-banner">${ctx.icon('alert')}<div><strong>Switching to Wi-Fi will turn this hotspot off first.</strong><span>The server first verifies that the SSID exists, then tries the supplied password. If the handover fails, ElectriCredit automatically restores this local hotspot and reports the reason.</span></div></div>`:''}
     <div class="ec-wifi-network-list">${wifi.length?wifi.map(n=>networkRow(ctx,n,data.connected_wifi)).join(''):`<div class="ec-network-empty">${ctx.icon('wifi')}<div><strong>${scanState.status==='success'?'No Wi-Fi networks detected':'Wi-Fi scan not started'}</strong><p>${scanState.status==='success'?'Try again closer to the access point, or enter the SSID manually below.':'Press Scan Wi-Fi to discover nearby networks.'}</p></div></div>`}</div>
     ${data.wireless_control?wifiConnectForm(ctx,wifi,data.connected_wifi):`<div class="ec-network-warning">Wi-Fi status can be viewed here. Connection controls require Linux NetworkManager/nmcli or Windows netsh.</div>`}
    </div>
   </article>
  </section>`;

  if(hotspot.enabled&&hotspot.qr_text)window.ECQRCode?.render?.(root.querySelector('[data-hotspot-qr]'),hotspot.qr_text,{size:210});
  if(setup.qr_text)window.ECQRCode?.render?.(root.querySelector('[data-setup-network-qr]'),setup.qr_text,{size:180});

  root.querySelector('[data-hotspot-reveal]')?.addEventListener('click',e=>toggleSecret(ctx,e.currentTarget,root.querySelector('[data-hotspot-secret]'),hotspot.password,hotspot.password_masked));
  root.querySelector('[data-saved-hotspot-reveal]')?.addEventListener('click',e=>toggleSecret(ctx,e.currentTarget,root.querySelector('[data-saved-hotspot-secret]'),saved.password,saved.password_masked));
  root.querySelector('[data-network-refresh]')?.addEventListener('click',()=>this.render(ctx,root));
  root.querySelector('[data-wifi-scan]')?.addEventListener('click',()=>startWiFiScan(ctx,root,data));
  root.querySelector('[data-hotspot-start]')?.addEventListener('click',()=>openHotspot(ctx,root,data,'start'));
  root.querySelector('[data-hotspot-edit]')?.addEventListener('click',()=>openHotspot(ctx,root,data,'update'));
  root.querySelector('[data-hotspot-stop]')?.addEventListener('click',()=>stopHotspot(ctx,root,data));

  root.querySelectorAll('[data-wifi-ssid]').forEach(button=>button.addEventListener('click',()=>{
   const form=root.querySelector('[data-wifi-form]');
   if(!form)return;
   form.elements.ssid.value=button.dataset.wifiSsid||'';
   form.elements.password?.focus();
   root.querySelectorAll('[data-wifi-ssid]').forEach(row=>row.classList.toggle('is-selected',row===button));
  }));

  root.querySelector('[data-wifi-form]')?.addEventListener('submit',async e=>{
   e.preventDefault();
   const button=e.currentTarget.querySelector('button[type=submit]');
   const body=ctx.S.formObject(e.currentTarget);
   body.ssid=String(body.ssid||'').trim();
   if(!body.ssid){window.Notification?.warning?.('Choose a scanned Wi-Fi network or enter its SSID.');return;}
   ctx.setBusy(button,true);
   try{
    const challenge=await window.ManagementMaintenance.challenge(ctx,'network-wifi-connect','Confirm Wi-Fi handover',`Move the Raspberry Pi Wi-Fi radio to ${body.ssid}? The local hotspot will be released first when it is active.`);
    if(!challenge)return;
    const result=await window.ManagementMaintenance.requestWithChallenge('/developer/network/wifi',{method:'POST',body,challenge});
    window.Notification?.info?.(result.message||`Wi-Fi handover started for ${body.ssid}.`);
    scheduleNetworkRefresh(ctx,root,'connect',!!result.hotspot_will_stop);
   }catch(err){window.Notification?.error?.(err.message)}finally{ctx.setBusy(button,false)}
  });

  window.UIEnhance?.init?.(root);
 }
};

async function startWiFiScan(ctx,root,data){
 const hotspot=data.hotspot||{},active=!!hotspot.enabled;
 const confirmed=await window.Modal.confirm({
  title:'Scan nearby Wi-Fi',
  message:active?'The ElectriCredit hotspot will turn off briefly so the Raspberry Pi Wi-Fi radio can scan nearby networks. The scan is saved in memory, then the same hotspot is turned back on automatically. Devices connected to the hotspot will disconnect briefly.':'Scan nearby Wi-Fi networks now? The result will be saved temporarily on the server and shown on this page.',
  confirmText:'Scan Wi-Fi',cancelText:'Cancel'
 });
 if(!confirmed)return;
 try{
  const result=await window.API.post('/developer/network/scan',{});
  window.Notification?.info?.(result.message||'Wi-Fi scan started.');
  scheduleNetworkRefresh(ctx,root,'scan',!!result.hotspot_will_restart);
 }catch(err){window.Notification?.error?.(err.message)}
}

function scheduleNetworkRefresh(ctx,root,kind,networkWillDrop){
 let completed=false;
 const refresh=async()=>{
  if(completed)return;
  try{
   await window.ElectriCreditDeveloperPages.network.render(ctx,root);
   completed=true;
   window.removeEventListener('online',refresh);
  }catch(err){
   if(!networkWillDrop){completed=true;window.Notification?.error?.(err.message||'Unable to refresh Network status.');}
  }
 };
 if(networkWillDrop)window.addEventListener('online',refresh,{once:true});
 setTimeout(refresh,networkWillDrop?(kind==='scan'?12000:15000):6500);
}

function scanSummary(ctx,state,current,count){
 if(state.status==='running')return 'Scanning nearby networks… the hotspot may disappear briefly';
 if(state.status==='failed')return ctx.escapeHTML(state.message||'The last Wi-Fi scan failed');
 if(state.status==='success')return `${Number(count||0)} network${Number(count||0)===1?'':'s'} found${state.hotspot_was_active&&state.hotspot_restored?' · local hotspot restored':''}`;
 if(current)return `Connected to ${ctx.escapeHTML(current)}`;
 return 'Scan before selecting a network';
}

function networkOperationBanner(ctx,scanState,connectState){
 if(connectState.status==='running')return `<div class="ec-network-handover-banner">${ctx.icon('wifi')}<div><strong>Wi-Fi handover in progress</strong><span>${ctx.escapeHTML(connectState.message||'ElectriCredit is validating the SSID and credentials.')}</span></div></div>`;
 if(connectState.status==='failed')return `<div class="ec-network-warning"><strong>${connectState.error_code==='ssid_not_found'?'SSID cannot be found':connectState.error_code==='password_incorrect'?'SSID found, password is incorrect':'Wi-Fi handover failed'}</strong><br>${ctx.escapeHTML(connectState.message||'ElectriCredit returned to local mode.')}</div>`;
 if(connectState.status==='success')return `<div class="ec-network-handover-banner">${ctx.icon('wifi')}<div><strong>Connected to ${ctx.escapeHTML(connectState.ssid||'Wi-Fi')}</strong><span>${ctx.escapeHTML(connectState.message||'Wi-Fi mode is active.')}</span></div></div>`;
 if(scanState.status==='running')return `<div class="ec-network-handover-banner">${scanIcon()}<div><strong>Scanning nearby Wi-Fi</strong><span>The hotspot is temporarily released, the results are cached, then local mode is restored.</span></div></div>`;
 if(scanState.status==='failed')return `<div class="ec-network-warning"><strong>Wi-Fi scan problem</strong><br>${ctx.escapeHTML(scanState.message||'The scan could not be completed.')}</div>`;
 return '';
}

function toggleSecret(ctx,button,target,plain,masked){
 if(!target)return;
 const shown=button.dataset.shown==='1';
 button.dataset.shown=shown?'0':'1';
 target.textContent=shown?(masked||'—'):(plain||'—');
 button.innerHTML=ctx.icon(shown?'eye':'eyeoff');
}

function savedHotspotPanel(ctx,saved){
 return `<div class="ec-saved-hotspot-panel">
  <div class="ec-saved-hotspot-mark">${ctx.icon('wifi')}</div>
  <div class="ec-saved-hotspot-copy"><span class="ec-badge">Saved local profile</span><h3>${ctx.escapeHTML(saved.ssid||'ElectriCredit Server')}</h3><p>This profile is stored in <code>[BOOT] Scripts/hotspot.json</code> and is used by Raspberry Pi startup and the Network page.</p></div>
  <div class="ec-saved-hotspot-details">
   <div><span>Password</span><strong data-saved-hotspot-secret>${ctx.escapeHTML(saved.password_masked||'••••••••')}</strong><button class="ec-management-icon-button ec-management-icon-button-sm" type="button" data-saved-hotspot-reveal title="Show saved password" data-tooltip="Show saved password">${ctx.icon('eye')}</button></div>
   <div><span>Interface</span><strong>${ctx.escapeHTML(saved.interface||'wlan0')}</strong></div>
   <div><span>Local address</span><strong>${ctx.escapeHTML(saved.ipv4_address||'10.42.0.1/24')}</strong></div>
   <div><span>Boot behavior</span><strong>${saved.boot_enabled?'Start hotspot automatically':'Do not start automatically'}</strong></div>
  </div>
 </div>`;
}

function hotspotPanel(ctx,hotspot,data){
 return `<div class="ec-hotspot-layout"><div class="ec-hotspot-qr" data-hotspot-qr></div><div class="ec-hotspot-details"><div><span>SSID</span><strong>${ctx.escapeHTML(hotspot.ssid||'—')}</strong></div><div><span>Password</span><strong data-hotspot-secret>${ctx.escapeHTML(hotspot.password_masked||'—')}</strong><button class="ec-management-icon-button ec-management-icon-button-sm" type="button" data-hotspot-reveal title="Show password" data-tooltip="Show password">${ctx.icon('eye')}</button></div><div><span>Interface</span><strong>${ctx.escapeHTML(hotspot.interface||'—')}</strong></div><div><span>Local IP</span><strong>${ctx.escapeHTML(hotspot.ip||'—')}</strong></div><div><span>Active modules</span><strong>${Number(data.active_modules||0)}</strong></div><div class="ec-network-warning">The same saved hotspot profile is used at Raspberry Pi boot. Updating the SSID/password here also updates <code>[BOOT] Scripts/hotspot.json</code>.</div></div></div>`;
}

function setupNetworkCard(ctx,setup){
 if(setup.mode==='none')return `<div class="ec-network-empty">${ctx.icon('alert')}<div><strong>No setup network</strong><p>Connect the server to Wi-Fi or start its hotspot before provisioning a module.</p></div></div>`;
 return `<div class="ec-setup-network-card">${setup.qr_text?`<div class="ec-hotspot-qr ec-hotspot-qr-small" data-setup-network-qr></div>`:`<span class="ec-setup-network-symbol">${ctx.icon('wifi')}</span>`}<div><span class="ec-badge">${setup.mode==='hotspot'?'Hotspot':'Current Wi-Fi'}</span><h3>${ctx.escapeHTML(setup.ssid||'Local network')}</h3><p>${setup.mode==='hotspot'?'This 2.4 GHz local network is used by the single Hub/Registry setup QR. Management fills the SSID, password, and Session ID for the ESP32 portal.':'The server is currently using Wi-Fi. Module setup should normally use the ElectriCredit 2.4 GHz hotspot so ESP32 devices do not inherit a 5 GHz-only uplink.'}</p>${setup.ip?`<small>Server network address: ${ctx.escapeHTML(setup.ip)}</small>`:''}</div></div>`;
}

function wifiConnectForm(ctx,wifi,current){
 return `<form class="ec-network-connect-form ec-network-connect-form-updated" data-wifi-form>
  <label class="ec-form-group"><span class="ec-label">Wi-Fi name (SSID)</span><input class="ec-input" name="ssid" list="ec-nearby-wifi" value="${ctx.escapeAttr(current||'')}" placeholder="Select above or type the SSID" required><datalist id="ec-nearby-wifi">${wifi.map(n=>`<option value="${ctx.escapeAttr(n.ssid)}"></option>`).join('')}</datalist></label>
  <label class="ec-form-group"><span class="ec-label">Password</span><input class="ec-input" type="password" name="password" autocomplete="new-password" placeholder="Enter the Wi-Fi password"></label>
  <button class="ec-button ec-button-primary" type="submit">${ctx.icon('wifi')}<span>Switch to Wi-Fi</span></button>
 </form>`;
}

function openHotspot(ctx,root,data,action){
 const h=data.hotspot||{},saved=data.hotspot_config||{},editing=action==='update',activeModules=Number(data.active_modules||0);
 const currentSSID=h.ssid||saved.ssid||'ElectriCredit Server';
 const currentInterface=h.interface||saved.interface||'wlan0';
 const modal=window.Modal.open({
  title:editing?'Update local hotspot':'Start local hotspot',hideCancel:true,
  content:`<div class="ec-hotspot-modal">
   <div class="ec-hotspot-modal-hero"><span class="ec-hotspot-modal-icon">${ctx.icon('wifi')}</span><div><span class="ec-badge">${editing?'Hotspot settings':'Local network'}</span><h3>${editing?'Update the saved local network':'Start the saved ElectriCredit hotspot'}</h3><p>The SSID and password are written to <code>[BOOT] Scripts/hotspot.json</code>, so they remain readable and editable from Raspberry Pi desktop mode.</p></div></div>
   ${editing?`<div class="ec-hotspot-current"><div><span>Current SSID</span><strong>${ctx.escapeHTML(h.ssid||'—')}</strong></div><div><span>Interface</span><strong>${ctx.escapeHTML(h.interface||'Automatic')}</strong></div><div><span>Active modules</span><strong>${activeModules}</strong></div></div>`:''}
   <form class="ec-form ec-hotspot-form" data-hotspot-form>
    <section class="ec-hotspot-form-section">
     <div class="ec-hotspot-form-heading"><span>Saved hotspot profile</span><small>Default: ElectriCredit Server / 1234567890</small></div>
     <div class="ec-hotspot-form-grid">
      <label class="ec-form-group"><span class="ec-label">Hotspot name (SSID)</span><input class="ec-input" name="ssid" required maxlength="32" value="${ctx.escapeAttr(currentSSID)}" placeholder="ElectriCredit Server"></label>
      <label class="ec-form-group"><span class="ec-label">Password</span><div class="ec-admin-password"><input class="ec-input" type="password" name="password" minlength="8" maxlength="63" placeholder="${saved.password?'Leave blank to keep saved password':'1234567890'}"><button type="button" data-eye aria-label="Show hotspot password">${ctx.icon('eye')}</button></div><small class="ec-help">8–63 characters. Leave blank to keep the saved password.</small></label>
     </div>
     <label class="ec-form-group"><span class="ec-label">Wireless interface</span><input class="ec-input" name="interface" value="${ctx.escapeAttr(currentInterface)}" placeholder="wlan0"><small class="ec-help">Raspberry Pi normally uses wlan0.</small></label>
     <label class="ec-hotspot-boot-toggle"><input type="checkbox" data-hotspot-boot ${saved.boot_enabled!==false?'checked':''}><span><strong>Start this hotspot automatically when Raspberry Pi boots</strong><small>The boot service reads the same hotspot.json file.</small></span></label>
    </section>
    <div class="ec-hotspot-handover-note">${ctx.icon('alert')}<div><strong>Single-radio handover</strong><span>${activeModules?`${activeModules} active module${activeModules===1?'':'s'} will be notified before the hotspot changes.`:'Starting the hotspot moves the Raspberry Pi Wi-Fi radio into access-point mode.'}</span></div></div>
    <div class="ec-hotspot-form-actions"><button class="ec-button ec-button-primary" type="submit">${powerIcon()}<span>${editing?'Save & restart hotspot':'Save & start hotspot'}</span></button></div>
   </form>
  </div>`
 });
 ctx.S.bindEyes(modal);
 modal.querySelector('[data-hotspot-form]')?.addEventListener('submit',async e=>{
  e.preventDefault();const button=e.currentTarget.querySelector('button[type=submit]');ctx.setBusy(button,true);
  try{
   const body={...ctx.S.formObject(e.currentTarget),action,boot_enabled:!!e.currentTarget.querySelector('[data-hotspot-boot]')?.checked};
   const challenge=await window.ManagementMaintenance.challenge(ctx,`network-hotspot-${action}`,editing?'Confirm hotspot update':'Confirm hotspot start',editing?'Apply the saved hotspot changes and restart the local AP?':'Start the Raspberry Pi local hotspot with these settings?');
   if(!challenge)return;
   const result=await window.ManagementMaintenance.requestWithChallenge('/developer/network/hotspot',{method:'POST',body,challenge});
   window.Modal.close({force:true});
   window.Notification?.success?.(`Hotspot updated${result.notified_modules?` · ${result.notified_modules} active module(s) notified`:''}.`);
   await window.ElectriCreditDeveloperPages.network.render(ctx,root);
  }catch(err){window.Notification?.error?.(err.message)}finally{ctx.setBusy(button,false)}
 });
}

function stopHotspot(ctx,root,data){
 const current=data.connected_wifi||'',h=data.hotspot||{},activeModules=Number(data.active_modules||0);
 const modal=window.Modal.open({
  title:'Turn off local hotspot',hideCancel:true,
  content:`<div class="ec-hotspot-modal is-danger-zone">
   <div class="ec-hotspot-modal-hero"><span class="ec-hotspot-modal-icon is-danger">${powerIcon()}</span><div><span class="ec-badge ec-badge-warning">Network handover</span><h3>Turn off ${ctx.escapeHTML(h.ssid||'the local hotspot')}</h3><p>The saved hotspot profile is kept in hotspot.json for the next manual or automatic startup.</p></div></div>
   <div class="ec-hotspot-current"><div><span>Current hotspot</span><strong>${ctx.escapeHTML(h.ssid||'—')}</strong></div><div><span>Active modules</span><strong>${activeModules}</strong></div><div><span>Fallback detected</span><strong>${ctx.escapeHTML(current||'None')}</strong></div></div>
   <form class="ec-form ec-hotspot-form" data-hotspot-stop-form>
    <section class="ec-hotspot-form-section"><div class="ec-hotspot-form-heading"><span>Optional fallback for modules</span><small>Use this only when you already know the Wi-Fi credentials. You can also stop this modal and use Scan Wi-Fi on the Network page.</small></div><label class="ec-form-group"><span class="ec-label">Wi-Fi name (SSID)</span><input class="ec-input" name="fallback_ssid" value="${ctx.escapeAttr(current)}" placeholder="Fallback Wi-Fi SSID"></label><label class="ec-form-group"><span class="ec-label">Wi-Fi password</span><div class="ec-admin-password"><input class="ec-input" type="password" name="fallback_password" autocomplete="new-password"><button type="button" data-eye aria-label="Show fallback password">${ctx.icon('eye')}</button></div></label></section>
    <div class="ec-hotspot-handover-note">${ctx.icon('alert')}<div><strong>Boot preference is unchanged</strong><span>Stopping the hotspot now does not delete hotspot.json or its automatic-start setting.</span></div></div>
    <div class="ec-hotspot-form-actions"><button class="ec-button ec-button-danger" type="submit">${powerIcon()}<span>Turn off hotspot</span></button></div>
   </form>
  </div>`
 });
 ctx.S.bindEyes(modal);
 modal.querySelector('[data-hotspot-stop-form]')?.addEventListener('submit',async e=>{
  e.preventDefault();const button=e.currentTarget.querySelector('button[type=submit]');ctx.setBusy(button,true);
  try{
   const body={...ctx.S.formObject(e.currentTarget),action:'stop'};
   const challenge=await window.ManagementMaintenance.challenge(ctx,'network-hotspot-stop','Confirm hotspot shutdown','Turn off the local hotspot? The saved boot profile will remain available.');
   if(!challenge)return;
   const result=await window.ManagementMaintenance.requestWithChallenge('/developer/network/hotspot',{method:'POST',body,challenge});
   window.Modal.close({force:true});
   window.Notification?.success?.(`Hotspot turned off${result.notified_modules?` · ${result.notified_modules} active module(s) notified`:''}.`);
   await window.ElectriCreditDeveloperPages.network.render(ctx,root);
  }catch(err){window.Notification?.error?.(err.message)}finally{ctx.setBusy(button,false)}
 });
}

function connectionCard(ctx,label,icon,items,subtitle=''){
 const connected=items.some(i=>i.connected),first=items.find(i=>i.connected)||items[0];
 const ip=first?.addresses?.find(a=>/^\d+\.\d+\.\d+\.\d+/.test(a)&&!/^169\.254\./.test(a))||first?.addresses?.find(a=>!String(a).startsWith('fe80:'))||'—';
 return `<article class="ec-network-status-card"><span class="ec-network-status-icon">${ctx.icon(icon==='phone'?'phone':icon)}</span><div><header><strong>${label}</strong><span class="ec-network-dot ${connected?'is-on':''}"></span></header><p>${subtitle||(connected?'Connected':'Not connected')}</p><small>${first?`${ctx.escapeHTML(first.name)} · ${ctx.escapeHTML(ip)}`:'No interface detected'}</small></div></article>`;
}

function networkRow(ctx,n,current){
 const active=String(n.ssid||'')===String(current||'');
 return `<button type="button" class="ec-wifi-row ${active?'is-active':''}" data-wifi-ssid="${ctx.escapeAttr(n.ssid||'')}"><span>${ctx.icon('wifi')}</span><div><strong>${ctx.escapeHTML(n.ssid||'Unnamed network')}</strong><small>${ctx.escapeHTML(n.security||'Open / unknown security')}</small></div><div><b>${ctx.escapeHTML(n.signal?`${n.signal}%`:'—')}</b><small>${active?'Connected':'Select'}</small></div></button>`;
}

function scanIcon(){return '<svg class="ec-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M5 9a10 10 0 0 1 14 0M8 12a6 6 0 0 1 8 0M10.5 15a2.5 2.5 0 0 1 3 0"/><circle cx="12" cy="18" r="1"/><path d="M19 4h3v3"/><path d="M22 4l-3 3"/></svg>'}
function powerIcon(){return '<svg class="ec-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v10"/><path d="M6.4 5.6a8 8 0 1 0 11.2 0"/></svg>'}
