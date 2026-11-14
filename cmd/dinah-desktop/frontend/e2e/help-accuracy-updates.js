const {start,settle,ledger,finish}=require('./lib/updates-fixture');
(async()=>{for(const key of ['Enter','Space']){
 const t=await start();const {page:p,ok}=t;
 const activate=async(id,label)=>{const el=p.locator(id);ok('named control '+label,(await el.innerText()).trim()===label);await el.focus();await p.keyboard.press('Shift+Tab');await p.keyboard.press('Tab');ok(key+' can Tab to '+label,await el.evaluate(e=>e===document.activeElement));await p.keyboard.press(key);};
 try{await settle(t);
  await activate('#updates-check','Check for Updates');await settle(t);ok(key+' checks releases',(await ledger(t)).some(c=>c[0]==='Check'));
  await p.evaluate(()=>{__u.release={current:'2.0.0',latest:'2.0.1',available:true,notes:'Fixture release'};});
  await activate('#updates-check','Check for Updates');await settle(t);
  await activate('#updates-install','Install Update');await p.waitForFunction(()=>__updateCalls.some(c=>c[0]==='Download'));
  await activate('#updates-package-check','Check Skill Packages');await p.waitForFunction(()=>!document.querySelector('#updates-package-check').disabled);
  await p.evaluate(()=>{__u.packages.updates=[{name:'Writing skills',target:'me',from:'1.0.0',to:'1.1.0',bundle:'C:/Fixture/package.zip'}];});
  await activate('#updates-package-check','Check Skill Packages');await p.waitForFunction(()=>document.querySelector('#updates-packages button').getAttribute('aria-disabled')==='false');
  await activate('#updates-packages > div:first-child button','Update');await p.locator('.install-plan').waitFor();
  ok(key+' package update requests authorization',(await ledger(t)).some(c=>c[0]==='PlanPersonalPackages'));await p.locator('.install-plan [data-no]').click();
  await activate('#updates-repair-check','Check Agent Settings');await p.waitForFunction(()=>document.querySelector('#updates-repair-apply').getAttribute('aria-disabled')==='false');
  await activate('#updates-repair-apply','Repair Agent Settings');await p.waitForFunction(()=>__updateCalls.some(c=>c[0]==='RepairAgentSettings'));
 }finally{await finish(t,'help-keys-'+key);}
}})().catch(e=>{console.error(e);process.exitCode=1;});
