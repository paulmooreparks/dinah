const {launch}=require('./lib/page');
(async()=>{const t=await launch(),p=t.page;try{
 await p.waitForFunction(()=>window.dinahTaskbarPins);
 await p.evaluate(async()=>{
  dinahWorkbench.register('label-fixture',{title:'Important document title',glyph:'file',init:()=>({})});
  window.__labelKey=await dinahWorkbench.open({kind:'label-fixture',path:'one'});
  dinahWorkbench.retitle(__labelKey,'Important document title','Long workspace name');
 });
 const tab=p.locator('.win-bar .win-tab[data-win-tab]').filter({hasText:'Important document title'}).first();
 await tab.waitFor();
 async function check(label){
  t.ok(label,await tab.evaluate(el=>{
   const title=el.querySelector('.win-item-title'),detail=el.querySelector('.win-item-detail');
   el.style.maxWidth='140px';el.style.width='140px';
   const style=getComputedStyle(title);
   return title.textContent==='Important document title · Long workspace name'&&detail.hidden&&style.textOverflow==='ellipsis'&&title.scrollWidth>title.clientWidth&&title.clientWidth>30&&el.title.includes('Important document title · Long workspace name');
  }));
 }
 await check('ordinary taskbar truncates combined title and workspace');
 await p.evaluate(()=>{const r=dinahWorkbench.pinReference({kind:'label-fixture',path:'one'});pudlWindows.pinTaskbar(r.id,true);});
 await p.waitForFunction(()=>document.querySelector('.win-bar [data-win-pin] .win-item-title')?.textContent==='Important document title · Long workspace name');
 await check('pinned taskbar preserves combined title and workspace');
 await t.shot('taskbar-combined-label');
}finally{await t.close()}})();
