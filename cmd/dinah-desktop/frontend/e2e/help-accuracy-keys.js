const {setup,makeRequest,choose}=require('./lib/decisions-fixture');
(async()=>{for(const key of ['Enter','Space']){
 const t=await setup([makeRequest('r5'),makeRequest('r4'),makeRequest('r3')]);const {page:p,ok}=t;
 // A button takes Enter or Space. One of a question's choices is chosen with Space, so the choice is pressed with Space in both rounds.
 const activate=async(loc,press=key,name)=>{await loc.focus();await p.keyboard.press('Shift+Tab');await p.keyboard.press('Tab');ok(press+' reaches '+(name||await loc.innerText()),await loc.evaluate(e=>e===document.activeElement));await p.keyboard.press(press);};
 try{
  await p.keyboard.press('Control+Shift+I');await p.locator('.win[data-win="needs-you"]').waitFor();
  ok('Ctrl+Shift+I opens Notifications',await p.evaluate(()=>pudlWindows.state().top==='needs-you'));
  // The reference in Help lists each control pressed below under the words the page shows on it, with the keys that work on it.
  if(key==='Enter'){
   await p.evaluate(()=>dinahCommands.run('keyboard-shortcuts'));const help=p.locator('dialog.keyboard-help[open]');await help.waitFor();
   await help.locator('#keyboard-help-for').selectOption('Decisions');
   const rows=await help.locator('tbody tr').evaluateAll(trs=>trs.map(tr=>[...tr.cells].map(td=>td.textContent)));
   const keys=label=>(rows.find(r=>r[0]===label)||[])[2]||'';
   ok('the reference says As recommended is chosen with Space',/Space/.test(keys('As recommended'))&&!/Enter/.test(keys('As recommended')),JSON.stringify(rows));
   for(const label of ['Send','Dismiss','Show More Requests'])ok('the reference says '+label+' takes Enter or Space',/Enter or Space/.test(keys(label)),JSON.stringify(rows));
   await help.locator('[data-close]').click();
  }
  await p.evaluate(()=>dinahWorkbench.open('decisions'));
  await p.evaluate(()=>{__pageSize=2;__emit('bus:changed',{});});await p.waitForFunction(()=>document.querySelectorAll('.decisions-row').length===2);
  await activate(p.getByRole('button',{name:'Show More Requests',exact:true}));await p.locator('[data-request-id="r3"]').waitFor();
  const recommended=p.locator('[data-question="1"]').getByRole('radio',{name:'As recommended',exact:true});
  await choose(t,'r3');await activate(recommended,'Space','As recommended');
  ok(key+' round: As recommended is chosen without sending',await recommended.isChecked()&&(await t.calls('AnswerDecision')).length===0);
  await p.waitForFunction(()=>!document.querySelector('.decisions-detail .btn-primary').disabled);
  await p.evaluate(async()=>{await dinahUIState.flush();pudlWindows.close('decisions');});
  await p.evaluate(()=>dinahWorkbench.open('decisions'));await choose(t,'r3');
  ok(key+' draft survives closing Decisions',await recommended.isChecked());
  await activate(p.locator('[data-dismiss-decision]'),key,'Dismiss');await p.locator('#ask[open]').waitFor();
  ok(key+' on Dismiss asks before dismissing anything',(await t.calls('DismissRequest')).length===0&&await p.evaluate(()=>document.activeElement===document.getElementById('ask-no')));await p.locator('#ask-no').click();
  await activate(p.getByRole('button',{name:'Send',exact:true}));await p.waitForFunction(()=>__delivered.length===1);
  ok(key+' sends only answered question',(await t.calls('AnswerDecision'))[0][1].answers.length===1);
  await p.evaluate(()=>dinahCommands.run('broadcast-active'));await p.getByLabel('Broadcast Message',{exact:true}).fill('Fixture message');
  await activate(p.getByRole('button',{name:'Preview Recipients',exact:true}));await p.waitForFunction(()=>!document.querySelector('[data-broadcast="confirm"]').disabled);
  await activate(p.getByRole('button',{name:'Confirm Broadcast…',exact:true}));await p.locator('#ask[open]').waitFor();
  ok(key+' broadcast still requires confirmation',(await t.calls('ConfirmDecisionBroadcast')).length===0);await p.locator('#ask-no').click();
 }finally{await t.close();}
}})().catch(e=>{console.error(e);process.exitCode=1;});
