const {launch}=require('./lib/page');
(async()=>{
 const t=await launch({viewport:{width:1400,height:900},ready:'workbench'});
 const {page,ok}=t;
 try {
  for(const mode of ['light','dark']){
   await page.evaluate(m=>pudlSetTheme(m),mode);
   const title=page.locator('.menubar-title[data-menu-id="site"]');
   ok(mode+' site menu has the logo before Dinah',await title.evaluate(el=>{
    const img=el.querySelector('img'),words=el.querySelector('[data-product-name]');
    return img?.complete&&img.naturalWidth>0&&img.alt===''&&words?.textContent==='Dinah'&&img.getBoundingClientRect().right<=words.getBoundingClientRect().left;
   }));
   await title.click();ok(mode+' logo title opens the site menu',await title.getAttribute('aria-expanded')==='true');await page.keyboard.press('Escape');
   await page.evaluate(()=>pudlMenubar.refresh());
   ok(mode+' logo survives menu refresh',await title.locator('img').count()===1);
   await t.shot('branding-'+mode);
  }
  await page.setViewportSize({width:600,height:800});
  await t.shot('branding-narrow');
  ok('narrow bar retains accessible Dinah text',await page.locator('[data-menubar]').getAttribute('aria-label')==='Dinah');
 }finally{await t.close();}
})().catch(e=>{console.error(e);process.exit(1)});
