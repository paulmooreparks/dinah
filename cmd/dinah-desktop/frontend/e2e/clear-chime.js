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
  await page.evaluate(()=>dinahAppearance.select('mineral'));
  await page.evaluate(()=>dinahAppearance.select('clear-chime'));
  ok('switching away and back admits Clear Chime',await page.evaluate(()=>dinahAppearance.snapshot().state.active==='clear-chime'));
  ok('Clear Chime passes all palette admission checks',await page.evaluate(()=>dinahAppearance.contrast(dinahAppearance.document('clear-chime')).length===0));
  ok('light primary gradient has contrast margin even at its brightest endpoint',await page.evaluate(()=>{
   const p=dinahAppearance.document('clear-chime').palettes.light;
   const rgb=h=>[1,3,5].map(i=>parseInt(h.slice(i,i+2),16)/255);
   const lum=c=>c.map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
   const foreground=lum(rgb(p['on-accent'])),background=lum(rgb(p.accent).map(v=>v*.78+.22));
   return (Math.max(foreground,background)+.05)/(Math.min(foreground,background)+.05)>=4.6;
  }));
  const audit=await page.evaluate(async()=>{const records=[];const failures=await dinahSkinContrast.check(dinahAppearance.document('clear-chime'),records);return {failures,primary:records.filter(r=>r.name==='primary action')};});
  ok('rendered skin admission passes for both modes and every control state',audit.failures.length===0,JSON.stringify(audit));
  console.log('Clear Chime primary contrast',JSON.stringify(audit.primary));
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
