// The warning badge counts the engine's undismissed warnings. Dismissal
// updates the open popup while a continuing pause remains visible.
const { launch, OUT } = require('./lib/page');
const fs = require('fs');
const {openStatus}=require('./lib/status');
const path = require('path');
(async () => {
 const t = await launch({viewport:{width:1440,height:900},app:()=>{
  window.__warnings = []; window.__paused = true;
  window.__warningVer = 0;
  window.__warningState = () => ({stateGen:1,stateVer:++__warningVer,state:__paused?'warning':'on',why:__paused?'Paused: provider quota reached':'',calls:9,fallbacks:5,streak:5,warnStreak:3,
   unseen:__warnings.length,warnings:structuredClone(__warnings),paused:__paused?{provider:'ollama',until:new Date(Date.now()+600000).toISOString(),reason:'provider quota reached'}:null});
  window.__publishWarnings = () => __emit('judgements:changed',__warningState());
  return {
   AgentProviders:async()=>[__provider('codex'),__provider('ollama')],
   JudgementState:async()=>__warningState(),
   JudgementSummary:async()=>window.__lateSummary?new Promise(resolve=>{window.__resolveSummary=resolve;}):({calls:9,fallbacks:5,latest:new Date().toISOString()}),
   JudgementRecords:async()=>({records:[],next:0}), JudgementKinds:async()=>[], JudgementModels:async()=>[],
   JudgementSettings:async()=>({settings:{enabled:true,provider:'ollama',model:'fixture',retentionDays:90,warnStreak:3,kindsOff:[]},revision:'r1'}),
   DismissJudgementWarning:async id=>{__warnings=__warnings.filter(w=>w.id!==id);__publishWarnings();},
   DismissJudgementWarnings:async()=>{__warnings=[];__publishWarnings();}
  };
 }});
 try {
  const p=t.page, opener=p.locator('#status-judgements'), popup=p.locator('#judgements-status-menu');
  const base=await p.evaluate(()=>Object.fromEntries(['--text-2xs','--text-xs','--text-sm','--text-md','--text-base','--text-lg','--text-xl','--text-2xl','--text-3xl'].map(k=>[k,getComputedStyle(document.documentElement).getPropertyValue(k)])));
  const evidence=[];
  // A cleared count is a visible neutral 0 badge, never the warning badge (11d4c27).
  const neutralZero=badge=>badge.evaluate(el=>!el.hidden&&el.textContent==='0'&&!el.classList.contains('warn'));
  for(const theme of ['light','dark']) for(const width of [1440,500,390]) for(const scale of [1,2]) {
   await p.setViewportSize({width,height:900});
   await p.evaluate(({theme,scale,base})=>{
    pudlSetTheme(theme);
    for(const [k,v] of Object.entries(base)) document.documentElement.style.setProperty(k,parseFloat(v)*scale+'px');
    __warnings=[1,2,3].map(id=>({id,at:new Date(Date.now()-id*60000).toISOString(),kind:'chat-title',why:'error',reason:'provider quota reached ('+id+')'}));
    __publishWarnings();
   },{theme,scale,base});
   await p.waitForFunction(()=>document.querySelector('#status-judgements .badge').textContent==='3');
   // Folded, the tray is PUDL's Status item and its menu (data-status-fold,
   // PUDL 0.97.0); each menu row is named as its item is.
   await p.waitForFunction(() => innerWidth > 500 || !!document.querySelector('.status-area > [data-status-folded]'));
   const foldedControl=p.locator('.status-area > .status-fold'), statusMenu=p.locator('.menu-panel:popover-open');
   const folded=await p.evaluate(()=>!!document.querySelector('.status-area > [data-status-folded]'));
   const rowFor=async id=>{const name=await p.locator('#'+id).evaluate(el=>(el.getAttribute('aria-label')||el.getAttribute('title')||el.textContent).replace(/\s+/g,' ').trim());return statusMenu.locator('.menu-action').filter({hasText:name}).first();};
   if(folded) {
    await foldedControl.focus(); await p.keyboard.press('Enter');
    await statusMenu.waitFor();
    t.ok('keyboard opening Status enters its first row',await statusMenu.locator('.menu-action').first().evaluate(el=>el===document.activeElement));
    await p.keyboard.press('Escape');
    await statusMenu.waitFor({state:'hidden'});
    await foldedControl.focus();
   }
   const geometry=await p.evaluate(async()=>{
    const samples=[],failures=[];
    __paused=false;__warnings=[];__publishWarnings();
    for(const entry of dinahHost.notices.list()) dinahHost.notices.remove(entry.id);
    const boxes=()=>Object.fromEntries([...document.querySelectorAll('.status-area button')].map((el,i)=>[el.id||'status-'+i,el.getBoundingClientRect().toJSON()]));
    const isFolded=()=>!!document.querySelector('.status-area > [data-status-folded]');
    await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
    const before=boxes(), initialFolded=isFolded(), focused=document.activeElement;
    for(const [index,state] of ['stopped','starting','ready','failed'].entries()) {
     __emit('agent:state',{provider:'codex',state,detail:'Fixture provider state',stateGen:'badge',stateVer:__warningVer*10+index});
     for(let frame=0;frame<6;frame++) {
      await new Promise(requestAnimationFrame);
      const after=boxes();
      if(isFolded()!==initialFolded) failures.push({frame,issue:'state changed tray folding'});
      if(initialFolded&&document.activeElement!==focused) failures.push({frame,issue:'the folded Status item lost focus'});
      for(const id of Object.keys(before)) if(['x','y','width','height'].some(k=>Math.abs(before[id][k]-after[id][k])>.02)) failures.push({state,frame,id,before:before[id],after:after[id]});
      // PUDL's Status item takes the most urgent tone of the folded badges:
      // the agent's badge is the only one with a tone here, until a failed
      // agent's own error notification makes Notifications stuck.
      if(frame===5&&initialFolded&&state!=='failed'&&(document.querySelector('.status-area > .status-fold').getAttribute('data-tone')||'')!==({stopped:'',starting:'accent',ready:'positive'})[state]) failures.push({state,frame,issue:'wrong folded tone',actual:document.querySelector('.status-area > .status-fold').getAttribute('data-tone')});
      if(frame===5&&initialFolded&&state==='failed'&&!['danger','stuck'].includes(document.querySelector('.status-area > .status-fold').getAttribute('data-tone'))) failures.push({state,frame,issue:'wrong folded tone',actual:document.querySelector('.status-area > .status-fold').getAttribute('data-tone')});
      samples.push({state,frame,boxes:after});
     }
    }
    // A failed provider emits its own notification asynchronously. Settle
    // that state before publishing exact notification-count fixtures.
    __paused=true;
    for(const count of [3,0,1,0,9,10,99,100,0,3]) {
     __warnings=Array.from({length:count},(_,i)=>({id:i+1,at:new Date().toISOString(),kind:'chat-title',why:'error',reason:'provider quota reached ('+(i+1)+')'}));
     __publishWarnings();
     for(const entry of dinahHost.notices.list()) dinahHost.notices.remove(entry.id);
     for(let i=0;i<count;i++) dinahHost.notices.add({id:'count-'+i,type:'error',description:'Fixture notification '+i});
     for(let frame=0;frame<6;frame++) {
      await new Promise(requestAnimationFrame);
      const after=boxes();
      if(isFolded()!==initialFolded) failures.push({frame,issue:'state changed tray folding'});
      if(initialFolded&&document.activeElement!==focused) failures.push({frame,issue:'the folded Status item lost focus'});
      for(const id of new Set([...Object.keys(before),...Object.keys(after)])) {
       if(!before[id]||!after[id]||['x','y','width','height'].some(k=>Math.abs(before[id][k]-after[id][k])>.02)) failures.push({count,frame,id,before:before[id],after:after[id]});
      }
      const badges=['status-needs','status-judgements'].map(id=>{const button=document.getElementById(id),badge=button.querySelector('[data-status-badge]'),reserve=button.querySelector('.status-badge-reserve');return {id,text:badge.textContent,hidden:badge.hidden,warn:badge.classList.contains('warn'),reserveHidden:reserve.getAttribute('aria-hidden')==='true'&&getComputedStyle(reserve).visibility==='hidden',label:button.getAttribute('aria-label')};});
      // With none waiting, each item shows a neutral 0 (11d4c27); above 0 it is a warning badge.
      if(frame===5&&badges.some(b=>count===0?b.hidden||b.text!=='0'||b.warn: b.hidden||!b.warn||b.text!==(count>99?'99+':String(count)))) failures.push({count,frame,issue:'wrong visible count',badges});
      if(badges.some(b=>!b.reserveHidden)) failures.push({count,frame,issue:'reserve is exposed'});
      const aggregate=document.querySelector('.status-area > .status-fold .badge');
      if(frame===5&&initialFolded&&(aggregate.hidden||aggregate.textContent!==(count*2>99?'99+':String(count*2)))) failures.push({count,frame,issue:'wrong folded total',value:aggregate.textContent});
      samples.push({count,frame,boxes:after,badges,folded:initialFolded});
     }
    }
    return {before,samples,failures,folded:initialFolded};
   });
   t.ok('all status controls stay fixed through zero, digit and cap boundaries at '+theme+'/'+width+'/'+scale,geometry.failures.length===0,JSON.stringify(geometry.failures.slice(0,6)));
   if(folded) {
    await t.shot('status-tray-'+theme+'-'+width+'-'+scale);
    // Each row opens its item's own panel, placed against the Status item,
    // and closing the panel returns focus to the Status item (PUDL 0.97.0).
    for(const id of ['status-workspace','status-needs','status-agent','status-judgements']) {
     await foldedControl.focus(); await p.keyboard.press('Enter');
     await statusMenu.waitFor();
     const row=await rowFor(id);
     await row.focus(); await p.keyboard.press('Enter');
     const target=await p.locator('#'+id).getAttribute('popovertarget');
     await p.locator('#'+target).waitFor();
     t.ok('the folded '+id+' row opens its original panel',await p.locator('#'+target).evaluate(el=>el.matches(':popover-open')));
     await p.keyboard.press('Escape');
     await p.locator('#'+target).waitFor({state:'hidden'});
     await p.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
     t.ok('closing '+id+' returns to the Status item',await foldedControl.evaluate(el=>el===document.activeElement),await p.evaluate(()=>document.activeElement.id||document.activeElement.className));
    }
   }
   await p.evaluate(()=>{for(const entry of dinahHost.notices.list()) dinahHost.notices.remove(entry.id); __emit('agent:state',{provider:'codex',state:'ready',detail:'Ready',stateGen:'badge',stateVer:__warningVer*10+9});});
   await p.evaluate(()=>{window.__lateSummary=true;});
   await openStatus(p,'status-judgements');
   await popup.waitFor();
   await popup.locator('.judgements-warning').nth(2).waitFor();
   const inset=await popup.evaluate(el=>{
    const panel=el.getBoundingClientRect(),first=el.querySelector('.judgements-popup-line').getBoundingClientRect();
    return {left:first.left-panel.left,right:panel.right-first.right,padding:getComputedStyle(el).paddingLeft};
   });
   t.ok('the shared popover inset surrounds the state line at '+theme+'/'+width+'/'+scale,inset.left>=parseFloat(inset.padding)&&parseFloat(inset.padding)>0,JSON.stringify(inset));
   t.ok('the current condition is visible independently of its warning count',/paused until.*provider quota reached/i.test(await popup.locator('.judgements-popup-line').textContent()));
   await t.shot('judgements-warnings-'+theme+'-'+width+'-'+scale);
   await popup.evaluate(el=>{
    window.__popupTrace={before:null,frames:[],active:true};
    function sample(){
     return Object.fromEntries([['panel',el],['summary',el.querySelector('.judgements-popup-summary')],['line',el.querySelector('.judgements-popup-line')],['all',el.querySelector('.judgements-dismiss-all')]].map(([key,node])=>[key,node.getBoundingClientRect().toJSON()]));
    }
    __popupTrace.before=sample();
    function frame(){if(!__popupTrace.active)return;__popupTrace.frames.push(sample());requestAnimationFrame(frame);}
    requestAnimationFrame(frame);
   });
   await p.waitForFunction(()=>!!window.__resolveSummary);
   await p.evaluate(()=>{__lateSummary=false;__resolveSummary({calls:9,fallbacks:5,latest:new Date().toISOString()});delete window.__resolveSummary;});
   await p.waitForFunction(()=>document.querySelector('.judgements-popup-today').textContent.includes('Today: 9 calls'));
   for(const [id,next,count] of [[2,3,2],[3,1,1],[1,null,0]]) {
    await popup.locator('[data-warning-id="'+id+'"] button').focus();
    const removedAt=await popup.locator('[data-warning-id="'+id+'"] button').boundingBox();
    await p.keyboard.press('Enter');
    await p.waitForFunction(count=>document.querySelectorAll('#judgements-status-menu .judgements-warning').length===count,count);
    await p.evaluate(()=>new Promise(resolve=>{let n=0;function frame(){if(++n===8)resolve();else requestAnimationFrame(frame);}requestAnimationFrame(frame);}));
    const target=next?popup.locator('[data-warning-id="'+next+'"] button'):popup.getByRole('link',{name:'Open Judgements',exact:true});
    t.ok('keyboard dismissal '+id+' retains focus at '+theme+'/'+width+'/'+scale,await target.evaluate(el=>el===document.activeElement));
    if(id===2) {
     const movedTo=await target.boundingBox();
     t.ok('the following Dismiss occupies the removed control position at '+theme+'/'+width+'/'+scale,Math.abs(movedTo.x-removedAt.x)<.02&&Math.abs(movedTo.y-removedAt.y)<.02,JSON.stringify({removedAt,movedTo}));
    }
   }
   t.ok('the empty state retains Dismiss All in place',await popup.locator('.judgements-warnings-empty').isVisible()&&await popup.getByRole('button',{name:'Dismiss All',exact:true}).isDisabled());
   await t.shot('judgements-empty-'+theme+'-'+width+'-'+scale);
   await p.evaluate(()=>{__warnings=Array.from({length:10},(_,i)=>i+1).map(id=>({id,at:new Date().toISOString(),reason:'another provider warning ('+id+')'}));__publishWarnings();});
   await popup.locator('[data-warning-id="10"]').waitFor();
   await popup.locator('[data-warning-id="10"] button').click();
   await p.waitForFunction(()=>document.querySelectorAll('#judgements-status-menu .judgements-warning').length===9);
   await popup.getByRole('button',{name:'Dismiss All',exact:true}).click();
   await p.waitForFunction(()=>document.querySelector('#status-judgements .badge').textContent==='0');
   await p.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
   const trace=await p.evaluate(()=>{__popupTrace.active=false;return __popupTrace;});
   const popupFailures=[];
   trace.frames.forEach((frame,index)=>Object.keys(trace.before).forEach(key=>{
    if(['x','y','width','height'].some(axis=>Math.abs(frame[key][axis]-trace.before[key][axis])>.02))popupFailures.push({index,key,before:trace.before[key],after:frame[key]});
   }));
   t.ok('popup, summary and Dismiss All stay fixed on every dismissal frame at '+theme+'/'+width+'/'+scale,trace.frames.length>=24&&popupFailures.length===0,JSON.stringify(popupFailures.slice(0,6)));
   t.ok('the full-tray paused glyph stays a warning after dismissal',await opener.locator('.glyph').evaluate(el=>el.style.getPropertyValue('--glyph')==='var(--glyph-warning)'&&el.style.color==='var(--warn)'));
   if(folded) {
    // The paused judge gives the folded Status item the waiting tone itself
    // (data-status-tone, PUDL 0.97.2) while its own badge stays a neutral 0.
    await p.waitForFunction(()=>document.querySelector('.status-area > .status-fold .badge').textContent==='0');
    t.ok('a paused judge keeps the folded waiting tone after its count clears',await foldedControl.getAttribute('data-tone')==='waiting'&&await foldedControl.locator('.badge').evaluate(el=>!el.hidden&&el.textContent==='0'&&!el.classList.contains('warn'))&&await neutralZero(p.locator('#status-judgements [data-status-badge]')));
   }
   t.ok('dismissing all clears the badge while retaining the condition',await popup.locator('.judgements-warning').count()===0&&/provider quota reached/.test(await popup.locator('.judgements-popup-line').textContent()));
   await p.evaluate(()=>__resync());
   await p.waitForFunction(()=>__bridgeCalls.filter(c=>c[0]==='JudgementState').length>1);
   await p.evaluate(()=>__publishWarnings());
   t.ok('a state refresh does not restore dismissed counts',await neutralZero(opener.locator('[data-status-badge]')));
   await p.keyboard.press('Escape');
   await openStatus(p,'status-judgements');
   await p.waitForFunction(()=>document.querySelector('#judgements-status-menu').style.getPropertyValue('--judgements-popup-height'));
   t.ok('reopening sizes to the empty content at '+theme+'/'+width+'/'+scale,(await popup.boundingBox()).height<trace.before.panel.height);
   t.ok('reopening keeps the warnings dismissed',await popup.locator('.judgements-warning').count()===0&&await neutralZero(opener.locator('[data-status-badge]')));
   await p.evaluate(()=>{__warnings=[{id:4,at:new Date().toISOString(),kind:'chat-title',why:'error',reason:'the next provider probe failed'}];__publishWarnings();});
   await p.waitForFunction(()=>document.querySelector('#status-judgements .badge').textContent==='1'&&!document.querySelector('#status-judgements .badge').hidden);
   t.ok('a new warning counts after dismissal',await popup.locator('[data-warning-id="4"]').count()===1);
   evidence.push({theme,width,scale,inset,geometry,popupTrace:trace,popupFailures});
   await p.keyboard.press('Escape');
   if(await statusMenu.isVisible()) await p.keyboard.press('Escape');
  }
  t.ok('individual dismissals call the engine with their warning ids',(await t.calls('DismissJudgementWarning')).map(c=>c[1]).join(',')===Array(12).fill('2,3,1,10').join(','));
  t.ok('Dismiss All calls the engine once per variant',(await t.calls('DismissJudgementWarnings')).length===12);
  fs.writeFileSync(path.join(OUT,'judgements-warnings.json'),JSON.stringify(evidence,null,2));
 } finally {await t.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
