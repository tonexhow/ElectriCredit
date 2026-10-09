window.ElectriCreditDeveloperPages=window.ElectriCreditDeveloperPages||{};
window.ElectriCreditDeveloperPages.overview={async render(ctx,root){
 const data=ctx.state.overview||await window.API.get('/developer/overview'); ctx.state.overview=null; ctx.state.headDeveloper=Boolean(data.head_developer); const c=data.counts||{};
 const connected=Number(c.connected_registries||0)+Number(c.connected_hubs||0), modules=Number(c.registries||0)+Number(c.hubs||0);
 root.innerHTML=`<section class="ec-page ec-admin-page ec-developer-page">${ctx.pageHeading('Management','Overview','Developer controls for ElectriCredit modules, server health, connectivity, records, and maintenance.')}
 <div class="ec-admin-stat-grid">
  ${ctx.statCard('Modules',modules,`${connected} connected`,'hub')}
  ${ctx.statCard('Registry',c.registries||0,`${c.connected_registries||0} connected`,'registry')}
  ${ctx.statCard('Hub',c.hubs||0,`${c.connected_hubs||0} connected`,'hub')}
  ${ctx.statCard('Setup sessions',c.pairing_waiting||0,'Waiting for a device','wifi')}
  ${ctx.statCard('Developers',c.developers||0,ctx.state.headDeveloper?'Head Developer access':'Developer access','developer')}
  ${ctx.statCard('Logs',c.logs||0,'Recorded management activity','logs')}
 </div>
 <div class="ec-management-grid ec-management-overview-grid">
  <article class="ec-management-card"><div class="ec-management-card-head"><div><strong>Module setup</strong><small>Register a Hub or Registry without manually entering its MAC address.</small></div>${ctx.icon('registry')}</div><div class="ec-management-card-body ec-management-quick-actions"><button class="ec-button ec-button-primary" data-go="registry">${ctx.icon('plus')}<span>Add Registry</span></button><button class="ec-button ec-button-secondary" data-go="hub">${ctx.icon('plus')}<span>Add Hub</span></button></div></article>
  <article class="ec-management-card"><div class="ec-management-card-head"><div><strong>Server operations</strong><small>Review host health, networking, database records, and system activity.</small></div>${ctx.icon('server')}</div><div class="ec-management-card-body ec-management-quick-links"><button data-go="system">${ctx.icon('server')}<span>System</span></button><button data-go="network">${ctx.icon('wifi')}<span>Network</span></button><button data-go="database">${ctx.icon('database')}<span>Database</span></button><button data-go="logs">${ctx.icon('logs')}<span>Logs</span></button></div></article>
 </div></section>`;
 root.querySelectorAll('[data-go]').forEach(b=>b.addEventListener('click',()=>ctx.navigate(b.dataset.go)));
}};
