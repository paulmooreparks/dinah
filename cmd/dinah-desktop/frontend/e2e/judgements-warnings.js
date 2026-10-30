// The warning badge counts the engine's undismissed warnings. Dismissal
// updates the open popup while a continuing pause remains visible.
const { launch, OUT } = require('./lib/page');
const fs = require('fs');
const path = require('path');
(async () => {
 const t = await launch({viewport:{width:1440,height:900},app:()=>{
  window.__warnings = [];
  window.__warningVer = 0;
  window.__warningState = () => ({stateGen:1,stateVer:++__warningVer,state:'warning',why:'Paused: provider quota reached',calls:9,fallbacks:5,streak:5,warnStreak:3,
   unseen:__warnings.length,warnings:structuredClone(__warnings),paused:{provider:'ollama',until:new Date(Date.now()+600000).toISOString(),reason:'provider quota reached'}});
  window.__publishWarnings = () => __emit('judgements:changed',__warningState());
  return {
   AgentProviders:async()=>[__provider('codex'),__provider('ollama')],
   JudgementState:async()=>__warningState(),
   JudgementSummary:async()=>({calls:9,fallbacks:5,latest:new Date().toISOString()}),
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
  for(const theme of ['light','dark']) for(const width of [1440,500]) for(const scale of [1,2]) {
   await p.setViewportSize({width,height:900});
   await p.evaluate(({theme,scale,base})=>{
    pudlSetTheme(theme);
    for(const [k,v] of Object.entries(base)) document.documentElement.style.setProperty(k,parseFloat(v)*scale+'px');
    __warnings=[1,2,3].map(id=>({id,at:new Date(Date.now()-id*60000).toISOString(),kind:'chat-title',why:'error',reason:'provider quota reached ('+id+')'}));
    __publishWarnings();
   },{theme,scale,base});
   await p.waitForFunction(()=>document.querySelector('#status-judgements .badge').textContent==='3');
   await opener.click();
   await popup.waitFor();
   await popup.locator('.judgements-warning').nth(2).waitFor();
   const inset=await popup.evaluate(el=>{
    const panel=el.getBoundingClientRect(),first=el.querySelector('.judgements-popup-line').getBoundingClientRect();
    return {left:first.left-panel.left,right:panel.right-first.right,padding:getComputedStyle(el).paddingLeft};
   });
   t.ok('the shared popover inset surrounds the state line at '+theme+'/'+width+'/'+scale,inset.left>=parseFloat(inset.padding)&&parseFloat(inset.padding)>0,JSON.stringify(inset));
   t.ok('the current condition is visible independently of its warning count',/paused until.*provider quota reached/i.test(await popup.locator('.judgements-popup-line').textContent()));
   await t.shot('judgements-warnings-'+theme+'-'+width+'-'+scale);
   const one=popup.locator('[data-warning-id="2"] button');
   await one.focus(); await one.click();
   await p.waitForFunction(()=>document.querySelector('#status-judgements .badge').textContent==='2');
   t.ok('dismissing one removes only that warning',await popup.locator('.judgements-warning').count()===2&&await popup.locator('[data-warning-id="2"]').count()===0);
   t.ok('focus moves to a remaining warning',await popup.locator('[data-warning-id="3"] button').evaluate(el=>el===document.activeElement));
   await popup.getByRole('button',{name:'Dismiss All',exact:true}).click();
   await p.waitForFunction(()=>document.querySelector('#status-judgements .badge').hidden);
   t.ok('dismissing all clears the badge while retaining the condition',await popup.locator('.judgements-warning').count()===0&&/provider quota reached/.test(await popup.locator('.judgements-popup-line').textContent()));
   await p.evaluate(()=>__resync());
   await p.waitForFunction(()=>__bridgeCalls.filter(c=>c[0]==='JudgementState').length>1);
   await p.evaluate(()=>__publishWarnings());
   t.ok('a state refresh does not restore dismissed counts',await opener.locator('.badge').isHidden());
   await p.keyboard.press('Escape');
   await opener.click();
   t.ok('reopening keeps the warnings dismissed',await popup.locator('.judgements-warning').count()===0&&await opener.locator('.badge').isHidden());
   await p.evaluate(()=>{__warnings=[{id:4,at:new Date().toISOString(),kind:'chat-title',why:'error',reason:'the next provider probe failed'}];__publishWarnings();});
   await p.waitForFunction(()=>document.querySelector('#status-judgements .badge').textContent==='1'&&!document.querySelector('#status-judgements .badge').hidden);
   t.ok('a new warning counts after dismissal',await popup.locator('[data-warning-id="4"]').count()===1);
   evidence.push({theme,width,scale,inset});
   await p.keyboard.press('Escape');
  }
  t.ok('the individual dismissal calls the engine with the warning id',(await t.calls('DismissJudgementWarning')).every(c=>c[1]===2));
  t.ok('Dismiss All calls the engine once per variant',(await t.calls('DismissJudgementWarnings')).length===8);
  fs.writeFileSync(path.join(OUT,'judgements-warnings.json'),JSON.stringify(evidence,null,2));
 } finally {await t.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
