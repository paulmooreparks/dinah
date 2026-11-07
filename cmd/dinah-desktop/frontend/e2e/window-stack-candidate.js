const { launch, PAGE } = require('./lib/page');
const fs = require('fs'), path = require('path'), crypto = require('crypto');
const candidate = path.join(__dirname, '..', 'pudl');
const out = process.argv[3];
(async () => {
  let nativeState = { version:1, values:{} }, t;
  const evidence = [], failures = [], errors = [];
  async function start(delay, min) {
    t = await launch({ goto:false, viewport:{width:1400,height:1000}, args:{delay}, app: ({delay}) => {
      let release; const gate = delay ? new Promise(r => release=r) : Promise.resolve();
      window.__release = () => release && release();
      return {
        ReadUIState: () => nativeRead(), SaveUIState: s => nativeSave(s),
        GetDraft: async id => { await gate; return {id,title:'Stack editor',text:'Stack fixture',revision:'1',documentType:'text/plain',origin:{kind:'new',workspaceId:'C:/Fixture'},actions:['update','discard','save-as'],suggestions:[],spans:{},changed:false}; },
        OpenSessionAt: async (cwd,sessionId) => { await gate; return {cwd,sessionId,provider:'codex',open:true,working:false}; }
      };
    }, init: () => {
      window.__events=[];
      for(const name of ['pudl:windows-change','focusin','dinah:editor-focus']) document.addEventListener(name,e => {
        __events.push({name,time:performance.now(),target:e.target.closest?.('[data-win]')?.dataset.win,state:window.pudlWindows?.state()});
      },true);
    }});
    await t.context.route(/\/pudl\/(pudl-windows\.(js|css)|pudl-regions\.js|pudl-launcher\.js)$/, route => {
      const name = new URL(route.request().url()).pathname.split('/').pop();
      const file = min ? path.join(candidate,'min',name) : path.join(candidate,name);
      return route.fulfill({path:file});
    });
    await t.context.exposeFunction('nativeRead',()=>nativeState);
    await t.context.exposeFunction('nativeSave',s=>{nativeState=s;});
    await t.page.goto(PAGE); await t.waitReady();
  }
  async function snapshot(label) {
    const s = await t.page.evaluate(() => {
      const state=pudlWindows.state();
      const hit=document.elementFromPoint(700,380)?.closest('[data-win]')?.dataset.win;
      return {state,hit,events:__events,focus:document.activeElement?.closest('[data-win]')?.dataset.win};
    }); evidence.push({label,...s}); return s;
  }
  async function stop(){failures.push(...t.failures);errors.push(...t.errors);await t.browser.close();}
  try {
    for(const min of [false,true]) {
      nativeState={version:1,values:{}};
      await start(false,min);
      const keys=await t.page.evaluate(async()=> {
        const keys=[];
        keys.push(await dinahWorkbench.open({kind:'editor',workspace:'C:/Fixture',path:'a'.repeat(32),title:'Stack editor'}));
        for(const id of ['stack-a','stack-b']) keys.push(await dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:id}));
        const u=new URL(location.href);
        for(const k of keys)u.searchParams.set('p.'+k,'floating:0.2,0.15,0.65,0.7');
        history.replaceState(null,'',u);dispatchEvent(new PopStateEvent('popstate'));
        return keys;
      });
      await t.page.locator('.cm-content').waitFor({state:'attached'});
      await t.page.evaluate(keys=>{for(const k of [keys[2],keys[0],keys[1]])pudlWindows.raise(k);},keys);
      const before=await snapshot('saved-'+min);
      await t.page.evaluate(()=>dinahUIState.flush());
      const saved=JSON.parse(JSON.stringify(nativeState));
      t.ok('saved full stack '+min,saved.values['dinah-workbench-layout'].includes('z.order'));
      await stop();
      for(const activate of [false,true]) {
        nativeState=JSON.parse(JSON.stringify(saved));
        await start(true,min);
        await t.page.waitForFunction(keys=>keys.every(k=>document.querySelector('[data-win="'+k+'"]')),keys);
        const initial=await snapshot('restored-before-ready-'+min+'-'+activate);
        t.ok('fresh context restores full stack and top '+min+' '+activate,JSON.stringify(initial.state.zOrder)===JSON.stringify(before.state.zOrder)&&initial.state.top===before.state.top);
        t.ok('rendered overlap restored '+min+' '+activate,initial.hit===before.hit && before.hit===keys[1]);
        // Use the public activation API as a taskbar selection would, without depending on covered title geometry.
        if(activate) await t.page.evaluate(k=>pudlWindows.raise(k),keys[2]);
        const selected=await snapshot('selected-'+min+'-'+activate);
        await t.page.evaluate(()=>__release());
        await t.page.locator('.cm-content').waitFor({state:'attached'});
        await t.page.waitForFunction(()=>__bridgeCalls.filter(c=>c[0]==='OpenSessionAt').length>=2);
        await t.page.waitForTimeout(500);
        const after=await snapshot('ready-'+min+'-'+activate);
        t.ok('delayed editor and chat preserve selection '+min+' '+activate,JSON.stringify(after.state.zOrder)===JSON.stringify(selected.state.zOrder)&&after.state.top===selected.state.top&&after.hit===selected.hit);
        t.ok('restart does not dispatch editor-focus '+min+' '+activate,!after.events.some(e=>e.name==='dinah:editor-focus'));
        await t.shot('stack-'+min+'-'+activate);
        await stop();
      }
    }
  } catch(e){failures.push(String(e.stack||e));} finally {
    fs.writeFileSync(path.join(out,'stack-evidence.json'),JSON.stringify(evidence,null,2));
    console.log(JSON.stringify({failures,errors},null,2));
    if(t?.browser.isConnected())await t.browser.close();
    if(failures.length||errors.length)process.exitCode=1;
  }
})();




