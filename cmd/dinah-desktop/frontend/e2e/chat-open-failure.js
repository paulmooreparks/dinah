// A failed native opening may find a saved copy only in its fallback read.
const {launch}=require('./lib/page');
(async()=>{
 for(const kind of ['fallback','no-saved-copy']){
  const t=await launch({viewport:{width:1440,height:950},args:{kind},state:{titles:{'codex:failure':'Known Failure Chat'}},app:({kind})=>{
   window.__failedNative=false;window.__openingGate=new Promise(resolve=>{window.__rejectOpening=resolve;});
   return {ResolveSessionContinuation:async id=>id,SessionContinuation:async()=>null,
    SavedConversation:async()=>__failedNative&&kind==='fallback'?{available:true,complete:true,recoverable:true,messages:[{role:'agent',text:'FALLBACK-SAVED-CONTENT'}]}:{available:false,messages:[]},
    OpenSessionAt:async()=>{await __openingGate;window.__failedNative=true;throw Error('thread not loaded: fixture');}};
  }});
  const {page:p,ok}=t;
  try{
   const key=await p.evaluate(()=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'codex:failure'}));const win=p.locator('[data-win="'+key+'"]');
   await p.waitForFunction(key=>{const el=document.querySelector('[data-win="'+key+'"] .chat-loading-chip');return el&&!el.hidden;},key);
   ok(kind+' reports actual pending native opening',await win.locator('.chat-loading-chip').evaluate(el=>!el.hidden));
   await p.evaluate(()=>__rejectOpening());
   await win.locator('[data-view-id=session-note]').waitFor();
   await p.waitForFunction(key=>document.querySelector('[data-win="'+key+'"] .chat-loading-chip').hidden,key);
   ok(kind+' retains the known title',await win.locator('.win-title').textContent()==='Known Failure Chat');
   ok(kind+' retains failure and retry controls',(await win.locator('[data-view-id=session-note]').textContent()).includes('thread not loaded')&&await win.getByRole('button',{name:'Try Again',exact:true}).isVisible());
   if(kind==='fallback')ok('fallback saved content and recovery remain available',await win.getByText('FALLBACK-SAVED-CONTENT',{exact:true}).isVisible()&&await win.getByRole('button',{name:'Continue from Saved History',exact:true}).isVisible());
   await t.shot('resume-failure-'+kind);
  }finally{await t.close();}
 }
})().catch(error=>{console.error(error);process.exitCode=1;});
