const { measure } = require('./lib/quality-frames');
const { start } = require('./lib/decision-answer-fixture');
// The four bodies are resumeText's public output, with the sender and kind
// from Bus.EnqueueResult. Each starts in a separate invented chat fixture.
const messages = {
  cancelled:'The restart of Dinah was cancelled. If you paused your work for it, carry on from where you stopped.',
  paused:'Dinah has restarted with its update. You paused your work for the restart; carry on from where you stopped.',
  ended:'Dinah has restarted with its update. Your last turn ended just before it; if work was left to do, carry on.',
  interrupted:'Dinah has restarted with its update, and your turn was cut short by the restart. Look at what you had done, since the last step may be half finished, and carry on from there.'
};
(async()=>{
  for(const [name,text] of Object.entries(messages)) {
    const t=await start();
    try {
      const p=t.page, roots=['.win','.win-bar'];
      await t.chat.locator('.msg:not(.msg-you)').waitFor();
      await measure(t,'frames-carry-'+name+'-arriving',roots,()=>p.evaluate(text=>__emit('agent:arriving',{sessionId:'live',messages:[{id:701,from:'Dinah',kind:'host_result',text}],more:0}),text),()=>t.chat.locator('[data-arriving="701"]').waitFor());
      await measure(t,'frames-carry-'+name+'-delivered',roots,()=>p.evaluate(text=>__emit('agent:sent',{sessionId:'live',kind:'bus',delivered:[{role:'delivered',text,provider:'codex',delivered:{id:701,kind:'host_result',from:'Dinah',time:'2026-11-15T04:00:00Z'}}]}),text));
    } finally { await t.close(); }
  }
})().catch(e=>{console.error(e);process.exitCode=1;});
