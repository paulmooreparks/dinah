const {setup,makeRequest,choose}=require('./lib/decisions-fixture');
(async()=>{for(const key of ['Enter','Space']){
 const t=await setup([makeRequest('r5'),makeRequest('r4'),makeRequest('r3')]);const {page:p,ok}=t;
 const activate=async loc=>{await loc.focus();await p.keyboard.press('Shift+Tab');await p.keyboard.press('Tab');ok(key+' reaches '+await loc.innerText(),await loc.evaluate(e=>e===document.activeElement));await p.keyboard.press(key);};
 try{
  await p.keyboard.press('Control+Shift+I');await p.locator('.win[data-win="needs-you"]').waitFor();
  ok('Ctrl+Shift+I opens Notifications',await p.evaluate(()=>pudlWindows.state().top==='needs-you'));
  await p.evaluate(()=>dinahWorkbench.open('decisions'));
  await p.evaluate(()=>{__pageSize=2;__emit('bus:changed',{});});await p.waitForFunction(()=>document.querySelectorAll('.decisions-row').length===2);
  await activate(p.getByRole('button',{name:'Earlier Requests',exact:true}));await p.locator('[data-request-id="r3"]').waitFor();
  await choose(t,'r3');await activate(p.locator('[data-question="1"]').getByRole('button',{name:'As Recommended',exact:true}));
  ok(key+' recommendation prepares without sending',(await t.calls('AnswerDecision')).length===0);
  await p.waitForFunction(()=>!document.querySelector('.decisions-detail .btn-primary').disabled);
  await p.evaluate(async()=>{await dinahUIState.flush();pudlWindows.close('decisions');});
  await p.evaluate(()=>dinahWorkbench.open('decisions'));await choose(t,'r3');
  ok(key+' draft survives closing Decisions',await p.getByLabel('Answer 1',{exact:true}).inputValue()==='Use the reviewed release.');
  await activate(p.getByRole('button',{name:'Send Answers',exact:true}));await p.waitForFunction(()=>__delivered.length===1);
  ok(key+' sends only answered question',(await t.calls('AnswerDecision'))[0][1].answers.length===1);
  await p.evaluate(()=>dinahCommands.run('broadcast-active'));await p.getByLabel('Broadcast Message',{exact:true}).fill('Fixture message');
  await activate(p.getByRole('button',{name:'Preview Recipients',exact:true}));await p.waitForFunction(()=>!document.querySelector('[data-broadcast="confirm"]').disabled);
  await activate(p.getByRole('button',{name:'Confirm Broadcast…',exact:true}));await p.locator('#ask[open]').waitFor();
  ok(key+' broadcast still requires confirmation',(await t.calls('ConfirmDecisionBroadcast')).length===0);await p.locator('#ask-no').click();
 }finally{await t.close();}
}})().catch(e=>{console.error(e);process.exitCode=1;});
