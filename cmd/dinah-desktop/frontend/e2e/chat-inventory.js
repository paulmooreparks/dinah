const { launch, OUT } = require('./lib/page');
const fs = require('fs'), path = require('path');
(async () => {
  const t = await launch({ ready: 'workbench', app: () => {
    const programs = ['devin','claude','codex','ollama'];
    const native = programs.flatMap(provider => [
      {sessionId:provider+':native',title:provider+' native title',cwd:'C:/Fixture',provider},
      {sessionId:provider+':old',title:provider+' predecessor',cwd:'C:/Fixture',provider},
      {sessionId:provider+':head',title:provider+' head title',cwd:'C:/Fixture',provider}
    ]);
    const local = programs.flatMap(provider => [
      {chatId:provider+':closed',head:provider+':closed',title:provider+' closed title',workspace:'C:/Fixture',provider,state:{life:'closed'},resolution:'resolved'},
      {chatId:provider+':old',head:provider+':head',title:provider+' head title',workspace:'C:/Fixture',provider,state:{life:'closed'},resolution:'resolved'}
    ]);
    local.push({chatId:'separate',head:'separate',title:'Separate fork',workspace:'C:/Other',provider:'devin',state:{life:'closed'},resolution:'resolved'});
    const held = new Map(); window.__inventoryNative = native; window.__inventoryLocal = local;
    const rows = () => [...new Map([...local.map(c=>({sessionId:c.head,title:c.title,workspace:c.workspace,provider:c.provider})),...native.filter(r=>!r.sessionId.endsWith(':old')).map(r=>({sessionId:r.sessionId,title:r.title,workspace:r.cwd,provider:r.provider}))].map(r=>[r.sessionId,{...r,eligible:true,open:false,programCopies:[],sessionIds:[r.sessionId]}])).values()].filter(r=>!held.has(r.sessionId));
    const view = () => ({generation:'fixture',chats:local.filter(c=>!held.has(c.head)),teams:[],groups:[]});
    return {
      Sessions: async () => native.filter(r=>!held.has(r.sessionId)), SessionsAt: async () => native.filter(r=>!held.has(r.sessionId)),
      OrgChart: async () => view(),
      ListChatCleanup: async () => ({rows:rows(),removed:[...held.values()],retentionDays:30,finalDeleteConfirmation:'Erase the invented copies.'}),
      RenameSession: async (id,title) => { native.filter(r=>r.sessionId===id).forEach(r=>r.title=title); local.filter(c=>c.head===id).forEach(c=>c.title=title); __state.titles={...__state.titles,[id]:title}; __emit('agent:titles',__state.titles); return __state.titles; },
      RemoveChat: async id => { const row=rows().find(r=>r.sessionId===id); held.set(id,{...row,restorable:true,finalDeleteConfirmation:'Erase invented copy.'}); __emit('chat:removed',{sessionId:id}); return {sessionId:id,ok:true,status:'removed',row:held.get(id)}; },
      RestoreRemovedChat: async id => { const row=held.get(id); held.delete(id); if(!local.some(c=>c.head===id)) local.push({chatId:id,head:id,title:row.title,workspace:row.workspace,provider:row.provider,state:{life:'closed'},resolution:'resolved'}); __emit('chat:restored',{sessionId:id}); return {sessionId:id,ok:true,status:'restored',row}; }
    };
  }});
  const p=t.page, evidence=[];
  try {
    await p.locator('#chat-scope').selectOption('all');
    await p.evaluate(()=>dinahWorkbench.open('org-chart'));
    await p.evaluate(()=>dinahWorkbench.open('chat-cleanup'));
    await p.waitForFunction(()=>document.querySelectorAll('.cleanup-row:not(.cleanup-bin-row)').length===13);
    async function compare(label, expected) {
      await p.waitForFunction(expected=>{
        return document.querySelectorAll('.org-chart [data-org-chat]').length===expected;
      },expected);
      const read=()=>p.evaluate(()=>{
        const visible=el=>!el.hidden&&!el.closest('[hidden]');
        const sidebar=[...document.querySelectorAll('#sessions [data-session-row]')].filter(el=>visible(el)).map(el=>({id:el.dataset.sessionRow,title:el.querySelector('.session-title').textContent,state:el.querySelector('.session-badge > :first-child').textContent.trim()}));
        const chart=[...document.querySelectorAll('.org-chart [data-org-chat]')].map(el=>({id:el.dataset.orgChat,title:el.querySelector('.org-chart-title').textContent,state:el.querySelector('.org-chart-state').textContent.trim()}));
        const cleanup=[...document.querySelectorAll('.cleanup-row:not(.cleanup-bin-row)')].filter(el=>visible(el)&&!el.dataset.outcome).map(el=>({id:el.dataset.chatId,title:el.querySelector('.cleanup-title').textContent,state:el.querySelector('.cleanup-state')?.textContent.trim()}));
        return Object.fromEntries(Object.entries({sidebar,chart,cleanup}).map(([key,rows])=>[key,rows.sort((a,b)=>a.id.localeCompare(b.id))]));
      });
      await p.waitForFunction(()=>[...document.querySelectorAll('.cleanup-row:not(.cleanup-bin-row):not([hidden])')].every(el=>el.querySelector('.cleanup-state')));
      const value=await read(); evidence.push({label,...value});
      t.ok(label+' agrees by identity, title and state on all three surfaces',JSON.stringify(value.sidebar)===JSON.stringify(value.chart)&&JSON.stringify(value.sidebar)===JSON.stringify(value.cleanup),JSON.stringify(value));
      return value;
    }
    const baseline=await compare('all programs, closed chats, switched heads and a separate fork',13);
    t.ok('native-only Devin remains Unknown under its real title',baseline.sidebar.some(r=>r.id==='devin:native'&&r.title==='devin native title'&&r.state==='Unknown'));
    t.ok('every switched predecessor collapses to its head',baseline.sidebar.every(r=>!r.id.endsWith(':old')));
    for(const provider of ['devin','claude','codex','ollama']) {
      await p.evaluate(provider=>go.main.App.RenameSession(provider+':head',provider+' renamed'),provider);
      await p.waitForFunction(provider=>document.querySelector('[data-org-chat="'+provider+':head"] .org-chart-title')?.textContent===provider+' renamed',provider);
      await compare(provider+' rename',13);
      await p.evaluate(provider=>go.main.App.RemoveChat(provider+':native'),provider); await compare(provider+' native bin',12);
      await p.evaluate(provider=>go.main.App.RestoreRemovedChat(provider+':native'),provider); await compare(provider+' native restore',13);
    }
    const ordering=await p.evaluate(async()=>{
      const app=go.main.App, originalSessions=app.Sessions, originalChart=app.OrgChart; let pending=false, reads=0;
      app.Sessions=()=>new Promise(resolve=>{pending=true;window.__finishInventory=async()=>{const rows=await originalSessions();pending=false;resolve(rows);};});
      app.OrgChart=()=>{reads++;if(pending)throw Error('Chat ownership changed while the chart was read. Refresh the chart.');return originalChart();};
      const request=dinahChats.refreshInventory();await Promise.resolve();const before=reads;await __finishInventory();await request;app.Sessions=originalSessions;app.OrgChart=originalChart;return {before,reads};
    });
    t.ok('logical ownership is captured after native listing writes finish',ordering.before===0&&ordering.reads===1,JSON.stringify(ordering));
    await p.evaluate(async()=>{const app=go.main.App;window.__workingChart=app.OrgChart;app.OrgChart=async()=>{throw Error('Invented inventory read failed.');};await dinahChats.refreshInventory();dinahHost.toast('An unrelated error.','danger');});
    const inventoryToast=p.locator('.toast-region .toast').filter({hasText:'Invented inventory read failed.'});
    t.ok('a current inventory failure is visible',await inventoryToast.count()===1);
    await p.evaluate(async()=>{go.main.App.OrgChart=__workingChart;await dinahChats.refreshInventory();});
    await inventoryToast.waitFor({state:'detached'});
    t.ok('recovery dismisses only its own inventory error',await p.locator('.toast-region .toast').filter({hasText:'An unrelated error.'}).count()===1);
    const superseded=await p.evaluate(async()=>{const app=go.main.App;app.OrgChart=()=>new Promise((resolve,reject)=>{window.__rejectInventory=reject;});const request=dinahChats.refreshInventory();while(!window.__rejectInventory)await Promise.resolve();dinahChats.restore({sessionId:'devin:native',title:'devin native title',workspace:'C:/Fixture',provider:'devin'});__rejectInventory(Error('Superseded inventory failure.'));await request;app.OrgChart=__workingChart;return [...document.querySelectorAll('.toast-text')].some(el=>el.textContent.includes('Superseded inventory failure.'));});
    t.ok('a superseded inventory failure raises no stale toast',!superseded);
    await p.locator('#chat-scope').selectOption('current');
    await p.evaluate(()=>dinahWorkbench.setState('org-chart','workspace=c%3A%2Ffixture%2F'));
    await p.locator('.chat-cleanup select[aria-label="Workspace"]').selectOption('C:/Fixture');
    await compare('matching workspace filters normalize case and trailing separators',12);
    for(const width of [1440,390]) {await p.setViewportSize({width,height:1000});await t.shot('chat-inventory-'+width);}
    fs.writeFileSync(path.join(OUT,'chat-inventory-equality.json'),JSON.stringify(evidence,null,2));
  } finally { await t.close(); }
})().catch(error=>{console.error(error);process.exitCode=1;});
