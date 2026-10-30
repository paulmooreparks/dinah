// Steering text recovered after a restart, and what a failed or uncertain
// steer leaves in the composer.
const { launch } = require('./lib/page');
(async()=>{
 const t = await launch({
  viewport:{width:1440,height:1000},
  state:{workspace:'C:/Test',recent:['C:/Test']},
  app:()=>({
   OpenSessionAt:async(_,id)=>({sessionId:id,provider:'codex',cwd:'C:/Test',open:true,working:false,queued:[]}),
   RecoverSteering:async id=>[{id:42,sessionId:id,state:'pending',payload:JSON.stringify({text:'Recovered input',attachments:[{kind:'file',name:'saved.txt',mimeType:'text/plain',data:'YWJj'}]})}],
   DismissSteering:async()=>{__calls.push('dismiss');},
   CanSteer:async()=>true,
   Steer:async(id,text,doc,atts)=>{
    __calls.push({text,atts});
    if(__mode==='uncertain') {
     __emit('agent:steer',{sessionId:id,steer:-1,state:'pending'});
     __emit('agent:steer',{sessionId:id,steer:-1,state:'failed'});
    }
    throw new Error(__mode==='uncertain'?'Receipt lost':'Local validation failed');
   }
  }),
  init:()=>{window.__calls=[];window.__mode='reject';},
  ready:()=>window.dinahWorkbench,
 });
 const {page,ok,errors}=t;
 try {
  const key=await page.evaluate(()=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Test',path:'codex:test'}));
  const win=page.locator('[data-win="'+key+'"]'),box=win.locator('[data-view-id=prompt]');
  const recovered=win.locator('[data-steering-recovery="42"]');
  await recovered.waitFor();
  ok('restart recovery does not send automatically',await page.evaluate(()=>__calls.length===0));
  await box.fill('New draft');
  await recovered.getByRole('button',{name:'Copy to Draft'}).click();
  ok('recovery preserves the newer draft',await box.inputValue()==='New draft\n\nRecovered input');
  ok('recovery retains attachment bytes',await win.locator('.attachment-name').textContent()==='saved.txt');
  await recovered.getByRole('button',{name:'Dismiss',exact:true}).click();await recovered.waitFor({state:'detached'});
  await page.evaluate(()=>__emit('agent:session',{sessionId:'codex:test',working:true,chatState:{life:'open',window:true,exec:{state:'working',why:'Turn in progress'}},open:true,queued:[]}));
  await page.waitForFunction(()=>document.querySelector('.win[data-chat-current] [data-view-id=steer-send]').classList.contains('btn-primary'));
  await win.locator('.chat-working-chip .chat-state-working').waitFor();
  ok('active chat has a Working chip in its title bar', true);
  ok('Steer is highlighted as the effective default',!(await win.locator('[data-view-id=send]').evaluate(el=>el.classList.contains('btn-primary'))));
  await box.press('Control+Enter');
  await page.waitForFunction(()=>__calls.some(c=>c.text));
  await page.waitForFunction(()=>document.querySelector('.win[data-chat-current] [data-view-id=prompt]').value.includes('Recovered input'));
  ok('local rejection restores text and attachments',await box.inputValue()==='New draft\n\nRecovered input'&&await win.locator('.attachment-name').count()===1);
  ok('restored attachment payload is unchanged',await page.evaluate(()=>__calls.find(c=>c.text).atts[0].data==='YWJj'));
  await page.evaluate(()=>{__mode='uncertain';});
  await box.press('Control+Enter');
  const uncertain=win.locator('[data-steer=failed]');await uncertain.waitFor();
  await uncertain.getByRole('button',{name:'Copy to Draft'}).waitFor();
  ok('uncertain delivery does not silently restore a resendable draft',await box.inputValue()==='');
  await uncertain.getByRole('button',{name:'Copy to Draft'}).click();
  ok('uncertain delivery can be explicitly copied with its attachments',(await box.inputValue()).includes('Recovered input')&&await win.locator('.attachment-name').count()===1);
  ok('no browser errors',errors.length===0);
 }finally{await t.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
