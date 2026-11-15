const { measure } = require('./lib/quality-frames');
const { start } = require('./lib/decision-answer-fixture');
const { setup, makeRequest } = require('./lib/decisions-fixture');
(async()=>{
  let t=await start();
  try {
    const p=t.page;
    await t.chat.locator('.msg:not(.msg-you)').waitFor();
    await p.evaluate(()=>{
      window.__restart={ready:true,pending:false,working:1,writing:0,blockers:['Fixture Chat']};
      go.main.App.UpdateState=async()=>__restart;
      go.main.App.PauseAndRestart=async()=>{__restart={...__restart,pending:true,pausing:true};__emit('update:state',__restart);};
      go.main.App.CancelRestart=async()=>{__restart={...__restart,pending:false,pausing:false,overdue:false};__emit('update:state',__restart);};
      __emit('update:state',__restart);
    });
    await p.locator('#status-update').click();
    const roots=['.win','.win-bar','#update-menu','#ask'];
    await measure(t,'frames-restart-asking',roots,()=>p.locator('#update-pause').evaluate(e=>e.click()),()=>p.waitForFunction(()=>document.querySelector('#update-note').textContent.includes('asked')));
    await p.keyboard.press('Escape');
    await p.evaluate(()=>dinahWorkbench.open('needs-you'));
    await p.locator('#questions').waitFor();
    await measure(t,'frames-restart-overdue',roots,()=>p.evaluate(()=>{__restart={...__restart,overdue:true};__emit('update:state',__restart);}),()=>p.locator('#questions [data-notice-id="dinah-pause-overdue"]').waitFor());
    const card=p.locator('#questions [data-notice-id="dinah-pause-overdue"]');
    t.ok('overdue card has both choices',await card.getByRole('button',{name:'Cancel the Restart',exact:true}).count()===1 && await card.getByRole('button',{name:'Restart Now…',exact:true}).count()===1);
    await measure(t,'frames-restart-now-question',roots,()=>card.getByRole('button',{name:'Restart Now…',exact:true}).evaluate(e=>e.click()),()=>p.locator('#ask[open]').waitFor());
    await measure(t,'frames-restart-now-decline',roots,()=>p.locator('#ask-no').evaluate(e=>e.click()),()=>p.locator('#ask[open]').waitFor({state:'hidden'}));
    await measure(t,'frames-restart-cancel',roots,()=>card.getByRole('button',{name:'Cancel the Restart',exact:true}).evaluate(e=>e.click()),()=>card.waitFor({state:'hidden'}));
    await p.evaluate(()=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'live'}));
    await measure(t,'frames-carry-on-arriving',roots,()=>p.evaluate(()=>__emit('agent:arriving',{sessionId:'live',messages:[{id:701,from:'Dinah',text:'The restart has finished. Carry on with your work.'}],more:0})),()=>t.chat.locator('[data-arriving="701"]').waitFor());
    await measure(t,'frames-carry-on-delivered',roots,()=>p.evaluate(()=>__emit('agent:sent',{sessionId:'live',kind:'bus',delivered:[{role:'delivered',text:'The restart has finished. Carry on with your work.',provider:'codex',delivered:{id:701,kind:'text',from:'Dinah',time:'2026-11-15T04:00:00Z'}}]})));
  } finally { await t.close(); }
  t=await setup([],{noOpen:true,entryPoints:true});
  try {
    const p=t.page;
    await p.waitForFunction(()=>document.querySelector('#status-decisions').dataset.statusCountKnown!==undefined);
    for(const n of [1,9,10,99,103,0]) {
      const items=Array.from({length:n},(_,i)=>makeRequest('request:'+(i+1)));
      await measure(t,'frames-decision-count-'+n,['.win-bar'],()=>p.evaluate(items=>{__decisionItems=items;return dinahDecisionEntries.refresh();},items),()=>p.waitForFunction(n=>document.querySelector('#status-decisions').dataset.statusCount===String(n),n));
    }
  } finally { await t.close(); }
})().catch(e=>{console.error(e);process.exitCode=1;});



