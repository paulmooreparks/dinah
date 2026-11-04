const {launch}=require('./lib/page');
(async()=>{
 const t=await launch({viewport:{width:1440,height:1000},ready:'workbench',app:()=>({
  ReadAppearance:async()=>JSON.parse(localStorage.getItem('fixture-appearance')||'{"state":{"version":1,"active":"clear-chime","skin":"standard","themes":[],"overrides":{}},"revision":"0"}'),
  SaveAppearance:async(state,revision)=>{const result={state,revision:String(Number(revision)+1)};localStorage.setItem('fixture-appearance',JSON.stringify(result));return result;}
 })});
 const {page,ok}=t;
 try {
  await page.evaluate(()=>dinahAppearance.ready);
  ok('Clear Chime is the product default',await page.evaluate(()=>dinahAppearance.defaultTheme()==='clear-chime'));
  await page.evaluate(()=>dinahAppearance.select('clear-chime'));
  ok('Clear Chime passes all palette admission checks',await page.evaluate(()=>dinahAppearance.contrast(dinahAppearance.document('clear-chime')).length===0));
  console.log('Clear Chime diff tints',await page.evaluate(()=>Object.fromEntries(Object.entries(dinahAppearance.document('clear-chime').palettes).map(([m,p])=>[m,dinahAppearance.diffTints(p)]))));
  await page.evaluate(()=>dinahWorkbench.open('settings'));
  for(const mode of ['light','dark']){
   await page.evaluate(m=>pudlSetTheme(m),mode);
   ok(mode+' applies the reviewed surfaces',await page.evaluate(m=>getComputedStyle(document.documentElement).getPropertyValue('--surface').trim()===(m==='dark'?'#282f32':'#faf8f1'),mode));
   ok(mode+' retains Standard',await page.evaluate(()=>pudlSkins.current()==='standard'));
   await t.shot('clear-chime-'+mode);
  }
  await page.reload();await page.evaluate(()=>dinahAppearance.ready);
  ok('reload retains Clear Chime',await page.evaluate(()=>dinahAppearance.snapshot().state.active==='clear-chime'));
 } finally {await t.close();}
})().catch(e=>{console.error(e);process.exit(1)});
