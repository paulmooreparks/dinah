const {launch}=require('./lib/page');
(async()=>{const t=await launch(),p=t.page;try{
 await p.waitForFunction(()=>window.dinahTaskbarPins);
 await p.evaluate(async()=>{
  dinahWorkbench.register('stack-probe',{title:'Picture tool',glyph:'file',icon:'product-mark.svg',launcher:{run:()=>dinahWorkbench.open({kind:'stack-probe',path:'new'})},init:()=>({})});
  window.__stackKeys=[];for(const path of ['one','two'])__stackKeys.push(await dinahWorkbench.open({kind:'stack-probe',path}));
  await pudlWindows.refreshTaskbarDestinations();pudlWindows.pinTaskbar('app-stack-probe',true);
 });
 const button=p.locator('.win-bar [data-win-pin="app-stack-probe"]');await button.waitFor();
 t.ok('multiple instances share one pinned stack',await button.count()===1&&await button.getAttribute('data-win-stack')!==null);
 await button.click();t.ok('stack click opens chooser',await p.locator('.win-stack-panel:visible').count()===1);
 await p.keyboard.press('Escape');
 t.ok('running picture loaded',await button.locator('img').evaluate(n=>n.complete&&n.naturalWidth>0));
 await p.evaluate(()=>__stackKeys.forEach(k=>pudlWindows.close(k)));
 t.ok('dormant pin keeps picture',await button.locator('img').evaluate(n=>n.complete&&n.naturalWidth>0));
 t.ok('closed stack leaves no ordinary duplicates',await p.evaluate(()=>__stackKeys.every(k=>!document.querySelector('.win-bar [data-win-tab="'+k+'"]'))));
 await t.shot('candidate-dormant-picture');
}finally{await t.close()}})();
