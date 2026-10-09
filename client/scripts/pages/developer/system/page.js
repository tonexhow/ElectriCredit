window.ElectriCreditDeveloperPages=window.ElectriCreditDeveloperPages||{};
(()=>{
 let socket=null,reconnectTimer=null,chart=null,active=false,currentCtx=null,currentRoot=null,history=[];
 const page={
  async render(ctx,root){
   dispose(); active=true; currentCtx=ctx; currentRoot=root; history=loadHistory();
   const cached=loadSnapshot();
   root.innerHTML=layout(ctx,cached||{});
   bind(ctx,root);
   if(cached)applySnapshot(cached,true);else setLive(false,'Connecting');
   connectLive();
   ensureSystemCharts().then(()=>renderHistory()).catch(()=>{});
   window.API.get('/developer/system').then(d=>applySnapshot(d,false)).catch(()=>{});
   window.UIEnhance?.init?.(root);
  },
  dispose
 };
 window.ElectriCreditDeveloperPages.system=page;

 function layout(ctx,d){return `<section class="ec-page ec-admin-page ec-developer-page">
  ${ctx.pageHeading('Management','System','Live health and resource activity from the ElectriCredit server.',`<div class="ec-system-live"><span class="ec-network-dot" data-system-live-dot></span><small data-system-live-label>Connecting</small><button class="ec-management-icon-button" data-system-refresh type="button" title="Refresh now" data-tooltip="Refresh now">${refreshIcon()}</button></div>`)}
  <div class="ec-system-hero-grid">
   ${ringCard('CPU','cpu')}${ringCard('RAM','ram')}${ringCard('Storage','storage')}${ringCard('Temperature','temp')}${ringCard('GPU','gpu')}
  </div>
  <div class="ec-management-grid ec-system-chart-grid">
   <article class="ec-management-card"><div class="ec-management-card-head"><div><strong>Resource history</strong><small>Updates automatically while this page is open</small></div>${ctx.icon('chart')}</div><div class="ec-management-card-body"><div class="ec-system-chart" data-system-history></div></div></article>
   <article class="ec-management-card"><div class="ec-management-card-head"><div><strong>Host</strong><small data-system-hostname>${ctx.escapeHTML(d.hostname||'ElectriCredit server')}</small></div>${ctx.icon('server')}</div><div class="ec-management-card-body"><div class="ec-system-detail-grid">
    ${detail('Operating system','os')}${detail('Go runtime','go')}${detail('GPU','gpu-name')}${detail('GPU memory','gpu-memory')}${detail('Network link','network')}${detail('Received','rx')}${detail('Sent','tx')}${detail('Sampled','sampled')}
   </div></div></article>
  </div>
  <article class="ec-management-card" style="margin-top:.85rem"><div class="ec-management-card-head"><div><strong>Network interfaces</strong><small>Usable interfaces reported by the server</small></div>${ctx.icon('wifi')}</div><div class="ec-management-card-body"><div class="ec-system-interface-list" data-system-interfaces></div></div></article>
 </section>`}
 function ringCard(label,key){return `<article class="ec-system-ring-card" data-system-card="${key}"><div class="ec-system-ring" data-system-ring="${key}" style="--ec-ring:0"><div><strong data-system-value="${key}">—</strong><span>${label}</span></div></div><small data-system-meta="${key}">Waiting for data</small></article>`}
 function detail(label,key){return `<div class="ec-system-detail"><span>${label}</span><strong data-system-detail="${key}">—</strong></div>`}

 function bind(ctx,root){root.querySelector('[data-system-refresh]')?.addEventListener('click',async e=>{ctx.setBusy(e.currentTarget,true);try{applySnapshot(await window.API.get('/developer/system'),false)}catch(err){window.Notification?.error?.(err.message)}finally{ctx.setBusy(e.currentTarget,false)}})}
 function connectLive(){
  if(!active)return; closeSocket();
  try{socket=new WebSocket(systemWSURL())}catch(_){scheduleReconnect();return}
  socket.addEventListener('open',()=>setLive(true,'Live'));
  socket.addEventListener('message',event=>{try{const msg=JSON.parse(event.data);if(msg.type==='system'&&msg.data)applySnapshot(msg.data,false)}catch{}});
  socket.addEventListener('close',()=>{if(!active)return;setLive(false,'Reconnecting');scheduleReconnect()});
  socket.addEventListener('error',()=>setLive(false,'Reconnecting'));
 }
 function scheduleReconnect(){clearTimeout(reconnectTimer);if(active)reconnectTimer=setTimeout(connectLive,2500)}
 function closeSocket(){if(socket){try{socket.close()}catch{}socket=null}}
 function dispose(){active=false;clearTimeout(reconnectTimer);reconnectTimer=null;closeSocket();if(chart){try{chart.dispose()}catch{}chart=null}currentCtx=null;currentRoot=null}
 function setLive(on,label){const dot=currentRoot?.querySelector('[data-system-live-dot]'),text=currentRoot?.querySelector('[data-system-live-label]');dot?.classList.toggle('is-on',on);if(text)text.textContent=label}
 function systemWSURL(){const u=new URL(window.API.getBaseURL(),location.href);u.protocol=u.protocol==='https:'?'wss:':'ws:';u.pathname=u.pathname.replace(/\/api\/?$/,'/api/developer/system/ws');u.search='';u.hash='';return u.toString()}

 function applySnapshot(d,first){if(!active||!currentRoot)return;const ctx=currentCtx;
  const ram=pct(d.ram_used_bytes,d.ram_total_bytes),storage=pct(d.storage_used_bytes,d.storage_total_bytes),cpu=safePercent(d.cpu_percent);
  const tempRaw=d.temperature_c;const temp=tempRaw===null||tempRaw===undefined||tempRaw===''?null:Number(tempRaw);const safeTemp=Number.isFinite(temp)?temp:null;
  const gpuRaw=d.gpu_percent;const gpu=gpuRaw===null||gpuRaw===undefined||gpuRaw===''?null:Number(gpuRaw);const safeGPU=Number.isFinite(gpu)?clamp(gpu):null;
  setRing('cpu',cpu??0,cpu===null?'—':`${cpu.toFixed(1)}%`,`${Number(d.cpu_threads||0)} threads`);
  setRing('ram',ram??0,ram===null?'—':`${ram.toFixed(1)}%`,ram===null?'Memory reading unavailable':`${d.ram_used||'—'} / ${d.ram_total||'—'}`);
  setRing('storage',storage??0,storage===null?'—':`${storage.toFixed(1)}%`,storage===null?'Storage reading unavailable':`${d.storage_used||'—'} / ${d.storage_total||'—'}`);
  setRing('temp',safeTemp===null?0:Math.min(100,Math.max(0,safeTemp)),safeTemp===null?'—':`${safeTemp.toFixed(1)}°C`,safeTemp===null?'Sensor unavailable':tempLabel(safeTemp));
  const gpuMeta=[d.gpu_name&&d.gpu_name!=='Unavailable'?d.gpu_name:'GPU information unavailable',d.gpu_memory_total&&d.gpu_memory_total!=='Unavailable'?d.gpu_memory_total:null].filter(Boolean).join(' · ');
  setRing('gpu',safeGPU??0,safeGPU===null?'—':`${safeGPU.toFixed(1)}%`,gpuMeta);
  setDetail('os',`${d.os||'—'} · ${d.architecture||'—'}`);setDetail('go',d.go_version||'—');setDetail('gpu-name',d.gpu_name||'Unavailable');setDetail('gpu-memory',gpuMemory(d));setDetail('network',d.network_speed||'Unavailable');setDetail('rx',bytes(d.network_rx_bytes));setDetail('tx',bytes(d.network_tx_bytes));setDetail('sampled',new Date(Number(d.sampled_at||0)*1000).toLocaleTimeString());
  const hostname=currentRoot.querySelector('[data-system-hostname]');if(hostname)hostname.textContent=d.hostname||'ElectriCredit server';
  renderInterfaces(d.interfaces||[],ctx);
  saveSnapshot(d);
  const point={t:Number(d.sampled_at||Date.now()/1000),cpu,ram,storage,temp:safeTemp,gpu:safeGPU};const last=history.at(-1);if(!last||last.t!==point.t)history.push(point);else history[history.length-1]=point;while(history.length>60)history.shift();saveHistory(history);renderHistory();
  if(first)setLive(false,'Connecting');
 }
 function setRing(key,value,display,meta){const ring=currentRoot.querySelector(`[data-system-ring="${key}"]`),v=currentRoot.querySelector(`[data-system-value="${key}"]`),m=currentRoot.querySelector(`[data-system-meta="${key}"]`);if(ring)ring.style.setProperty('--ec-ring',String(clamp(value)));if(v)v.textContent=display;if(m)m.textContent=meta||'—'}
 function setDetail(key,value){const el=currentRoot.querySelector(`[data-system-detail="${key}"]`);if(el)el.textContent=value??'—'}
 function renderInterfaces(items,ctx){const host=currentRoot.querySelector('[data-system-interfaces]');if(!host)return;const rows=items.filter(i=>i.kind!=='loopback').map(i=>{const ips=(i.addresses||[]);const ipv4=ips.find(v=>/^\d+\.\d+\.\d+\.\d+/.test(v))||ips[0]||'—';return `<article class="ec-system-interface"><div><strong>${ctx.escapeHTML(i.name)}</strong><span class="ec-badge">${ctx.escapeHTML(interfaceLabel(i.kind))}</span></div><span>IP ${ctx.escapeHTML(ipv4)}</span><span>MAC ${ctx.escapeHTML(i.mac||'—')}</span>${i.speed?`<span>${ctx.escapeHTML(i.speed)}</span>`:''}</article>`}).join('');host.innerHTML=rows||'<p class="ec-help">No active network interface was reported.</p>'}
 function renderHistory(){const el=currentRoot?.querySelector('[data-system-history]');if(!el)return;if(!window.echarts){el.innerHTML='<div class="ec-help">The live figures above are available, but the local chart library could not be loaded.</div>';return}if(!chart)chart=echarts.init(el);const ctx=currentCtx,times=history.map(x=>new Date(x.t*1000).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'}));const percentSeries=[['CPU','cpu'],['RAM','ram'],['Storage','storage'],['GPU','gpu']].map(([name,key])=>({name,type:'line',smooth:true,showSymbol:false,connectNulls:false,data:history.map(x=>x[key]===null?null:Number(x[key]||0)),areaStyle:{opacity:.035}}));chart.setOption({animationDuration:250,tooltip:{trigger:'axis'},legend:{data:['CPU','RAM','Storage','GPU','Temperature'],textStyle:{color:ctx.S.getCSS('--ec-text-muted','#777')}},grid:{left:42,right:44,top:42,bottom:30},xAxis:{type:'category',data:times,axisLabel:{color:ctx.S.getCSS('--ec-text-muted','#777')}},yAxis:[{type:'value',min:0,max:100,axisLabel:{formatter:'{value}%',color:ctx.S.getCSS('--ec-text-muted','#777')}},{type:'value',min:0,max:100,axisLabel:{formatter:'{value}°',color:ctx.S.getCSS('--ec-text-muted','#777')}}],series:[...percentSeries,{name:'Temperature',type:'line',yAxisIndex:1,smooth:true,showSymbol:false,connectNulls:false,data:history.map(x=>x.temp===null?null:x.temp)}]});chart.resize()}
 function pct(used,total){used=Number(used||0);total=Number(total||0);return total>0?clamp(used/total*100):null}function safePercent(v){if(v===null||v===undefined||v==='')return null;v=Number(v);return Number.isFinite(v)?clamp(v):null}function clamp(v){v=Number(v);return Math.max(0,Math.min(100,Number.isFinite(v)?v:0))}
 function tempLabel(v){if(v>=80)return'High temperature';if(v>=65)return'Warm';return'Normal range'}
 function gpuMemory(d){const used=d.gpu_memory_used&&d.gpu_memory_used!=='Unavailable'?d.gpu_memory_used:null,total=d.gpu_memory_total&&d.gpu_memory_total!=='Unavailable'?d.gpu_memory_total:null;if(used&&total)return`${used} / ${total}`;return total||'Unavailable'}
 function bytes(v){v=Number(v||0);if(!v)return'—';const units=['B','KB','MB','GB','TB'];let i=0;while(v>=1024&&i<units.length-1){v/=1024;i++}return`${v.toFixed(i?1:0)} ${units[i]}`}
 function interfaceLabel(v){return({wireless:'Wi-Fi',usb:'USB tethering',ethernet:'Ethernet'})[v]||v||'Interface'}
 function loadHistory(){try{return JSON.parse(sessionStorage.getItem('ec-system-history-v3')||'[]').filter(x=>x&&x.t).slice(-60)}catch{return[]}}
 function saveHistory(v){try{sessionStorage.setItem('ec-system-history-v3',JSON.stringify(v))}catch{}}
 function loadSnapshot(){try{return JSON.parse(sessionStorage.getItem('ec-system-snapshot-v1')||'null')}catch{return null}}
 function saveSnapshot(v){try{sessionStorage.setItem('ec-system-snapshot-v1',JSON.stringify(v))}catch{}}
 function refreshIcon(){return '<svg class="ec-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6v5h-5"/><path d="M4 18v-5h5"/><path d="M6.1 9a7 7 0 0 1 11.5-2.5L20 9M4 15l2.4 2.4A7 7 0 0 0 18 15"/></svg>'}
 function ensureSystemCharts(){if(window.echarts)return Promise.resolve();if(window.__ecEChartsPromise)return window.__ecEChartsPromise;window.__ecEChartsPromise=new Promise((resolve,reject)=>{const script=document.createElement('script');script.src='scripts/pages/admin/dashboard/echarts.min.js';script.onload=resolve;script.onerror=reject;document.head.appendChild(script)});return window.__ecEChartsPromise}
})();
