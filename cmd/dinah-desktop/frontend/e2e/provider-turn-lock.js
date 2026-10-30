const { launch } = require('./lib/page');
(async()=>{
  const t = await launch({
    viewport:{width:1600,height:1000},
    app:()=>{
      const providers=['codex','ollama','claude','devin'];
      const info=id=>({sessionId:id,provider:id,title:id,cwd:'C:/Fixture',open:true,working:true,queued:[]});
      return {
        Sessions:async()=>providers.map(id=>({...info(id),open:false})),
        AgentProviders:async()=>providers.map(id=>({id,name:id,installed:true,available:true})),
        OpenSessionAt:async(_,id)=>info(id)
      };
    },
  });
  const {page,ok}=t;
  try {
    const windows={};
    for(const provider of ['codex','ollama','claude','devin']) {
      await page.evaluate(()=>dinahWorkbench.open('chat-browser'));
      await page.locator('#sessions a[data-session="'+provider+'"]').click();
      const win=page.locator('.win.active');
      await win.locator('.win-title').getByText(provider,{exact:true}).waitFor();
      windows[provider]=await win.getAttribute('data-win');
      // Opening an already-running session may return a snapshot without
      // emitting a new agent:session event.
      await page.evaluate(()=>Promise.resolve().then(()=>Promise.resolve()));
      // The snapshot is applied after the window is drawn; wait for the
      // composer to show it before the assertions below read it.
      await page.waitForFunction(key=>{
        const w=document.querySelector('[data-win="'+key+'"]');
        return w&&w.querySelector('[data-command=provider]').disabled&&w.querySelector('[data-view-id=send]').textContent==='Queue';
      },windows[provider]);
      ok(provider+' provider is locked from the open-session snapshot',await win.locator('[data-command=provider]').isDisabled());
      ok(provider+' composer shows the active turn',await win.locator('[data-view-id=send]').textContent()==='Queue');
    }
    // Background session events must update the correct window even when
    // another chat has focus.
    for(const provider of ['codex','ollama','claude','devin']) {
      await page.evaluate(id=>__emit('agent:session',{sessionId:id,working:false,queued:[],open:true}),provider);
      const control=page.locator('[data-win="'+windows[provider]+'"] [data-command=provider]');
      await page.waitForFunction(key=>!document.querySelector('[data-win="'+key+'"] [data-command=provider]').disabled,windows[provider]);
      ok(provider+' provider unlocks after the turn',await control.isEnabled());
      await page.evaluate(id=>__emit('agent:session',{sessionId:id,working:true,queued:[],open:true}),provider);
      await page.waitForFunction(key=>document.querySelector('[data-win="'+key+'"] [data-command=provider]').disabled,windows[provider]);
      ok(provider+' provider relocks on a background turn',await control.isDisabled());
      await page.evaluate(id=>__emit('agent:session',{sessionId:id,title:id+' renamed'}),provider);
      await page.evaluate(()=>Promise.resolve().then(()=>Promise.resolve()));
      ok(provider+' partial metadata preserves the active-turn lock',await control.isDisabled());
    }
  } finally {await t.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
