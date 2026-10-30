/* The additions the Git applet's overhaul asked of the extension host: shared
   state between an extension's frames, open-view reusing an open window, a
   view that follows the workspace with a pin, and open-diff routing the diff's
   actions back to the frame. */
const { launch } = require('./lib/page');
const path = require('path'), fs = require('fs');
(async()=>{
 const t=await launch({
  viewport:{width:1440,height:1000},
  app:()=>{
   const workspaces={fixture:{id:'fixture',name:'Fixture',path:'C:/Fixture',sources:[]},other:{id:'other',name:'Other',path:'C:/Other',sources:[]}};
   window.__selected='fixture';
   const manifest={id:'test.fixture',version:'0.1.0',title:'Extension Fixture',description:'Fixture',publisher:{name:'Test'},permissions:['workspace.read'],actions:[{name:'inspect'}],views:[{id:'pane',title:'Fixture Pane',page:'index.html',pane:true},{id:'window',title:'Fixture Window',page:'index.html'},{id:'follower',title:'Fixture Follower',page:'index.html',follows:true}]};
   // The selected workspace is read from State by the page, and ExtensionWorkspace resolves '' to it, as the host does.
   return{NativeWindowState:async()=>({available:false}),State:async()=>({workspaceId:__selected,workspace:workspaces[__selected].path,recent:['C:/Fixture','C:/Other'],workspaces:['C:/Fixture','C:/Other'],workspaceRecords:Object.values(workspaces).map(ws=>({id:ws.id,name:ws.name,primarySourceId:'local',sources:[{id:'local',kind:'local-folder',name:ws.name,path:ws.path}]}))}),Extensions:async()=>[{manifest,enabled:true}],ExtensionWorkspace:async id=>{const ws=workspaces[id||__selected];if(!ws)throw Error('open a registered workspace before using extensions');return{...ws};},ExtensionCall:async(id,action,ws)=>({workspace:ws}),CancelExtensionCall:async()=>{}};
  },
  // The fixture's stub runs in the extension's sandboxed frames too; an
  // extension sees no bridge, so the frames are cleared of it.
  init:()=>{if(window!==parent)for(const k of ['go','runtime','__emit','__bridgeCalls','__runtime','__state','__selected'])delete window[k];},
  goto:false,
 });
 const {page,ok}=t;
 try{
  await t.context.route('https://dinah.test/**',async route=>{
   const pathname=new URL(route.request().url()).pathname;
   if(pathname==='/extension-sdk/dinah-extension.js')return route.fulfill({contentType:'text/javascript',body:fs.readFileSync(path.resolve(__dirname,'../../../../../dinah-extension-go/web/dinah-extension.js'),'utf8'),headers:{'Access-Control-Allow-Origin':'*'}});
   if(pathname.endsWith('/fixture.js'))return route.fulfill({contentType:'text/javascript',body:`
    const show=(id,value)=>{document.getElementById(id).textContent=typeof value==='string'?value:JSON.stringify(value);};
    window.__events={shared:[],state:[],command:[],workspace:[]};
    dinah.ready.then(ready=>{show('status','ready');show('initial',dinah.shared.get());show('workspace',ready.workspace.id);});
    dinah.on('shared',change=>{__events.shared.push(change);show('shared',change);});
    dinah.on('state',value=>{__events.state.push(value);show('state',value);});
    dinah.on('command',value=>{__events.command.push(value);show('command',value);});
    dinah.on('workspace',value=>{__events.workspace.push(value);show('workspace',value.id+(value.pinned?' pinned':''));});`});
   if(pathname.startsWith('/extension-assets/'))return route.fulfill({contentType:'text/html',headers:{'Content-Security-Policy':"sandbox allow-scripts allow-forms; default-src 'none'; script-src https://dinah.test/extension-assets/ https://dinah.test/extension-sdk/; style-src 'unsafe-inline'; form-action 'none'; frame-ancestors 'self'"},body:'<!doctype html><main tabindex="-1" data-extension-focus><output id="status"></output><output id="initial"></output><output id="shared"></output><output id="state"></output><output id="command"></output><output id="workspace"></output></main><script defer src="/extension-sdk/dinah-extension.js"></script><script defer src="fixture.js"></script>'});
   const file=path.join(__dirname,'..',pathname);
   // A script or stylesheet the page lists before its file lands is a 404, as it would be from the host, not a failed route.
   return fs.existsSync(file)?route.fulfill({path:file}):route.fulfill({status:404,body:''});
  });
  await page.goto('https://dinah.test/index.html');
  await page.waitForFunction(()=>window.dinahExtensions?.catalog().length===1);
  const frameOf=title=>page.frameLocator('.extension-frame[title="'+title+'"]');
  const framesTitled=title=>page.locator('.extension-frame[title="'+title+'"]');
  const evalIn=(title,fn,arg)=>frameOf(title).locator('main').evaluate(fn,arg);
  const selectWorkspace=async id=>{await page.evaluate(id=>{__selected=id;},id);await page.evaluate(()=>window.go.main.App.State().then(st=>{document.dispatchEvent(new Event('dinah:workspace-selected'));return st;}));};

  // 1. Shared state between two frames of the same extension in one workspace.
  await page.evaluate(()=>dinahWorkbench.open({kind:'extension-view',workspace:'',path:JSON.stringify(['test.fixture','pane'])}));
  await frameOf('Fixture Pane').locator('#status').filter({hasText:'ready'}).waitFor();
  await page.evaluate(()=>dinahWorkbench.open({kind:'extension-view',workspace:'fixture',path:JSON.stringify(['test.fixture','window'])}));
  await frameOf('Fixture Window').locator('#status').filter({hasText:'ready'}).waitFor();
  await evalIn('Fixture Pane',()=>dinah.shared.set('selection',{file:'README.md',staged:false}));
  await frameOf('Fixture Window').locator('#shared').filter({hasText:'README.md'}).waitFor();
  ok('a shared key set by the pane reaches the window with the setter\'s view ID',await evalIn('Fixture Window',()=>{const change=__events.shared[0];return change.key==='selection'&&change.value.file==='README.md'&&change.from==='pane'&&dinah.shared.get().selection.file==='README.md';}));
  ok('the setter does not receive its own shared event',await evalIn('Fixture Pane',()=>__events.shared.length===0&&dinah.shared.get().selection.file==='README.md'));
  ok('an invalid shared key and an oversized value are refused',await evalIn('Fixture Pane',async()=>{const refused=[];for(const [key,value] of [['Bad Key','x'],['big','x'.repeat(65*1024)]]){try{await dinah.shared.set(key,value);refused.push(false);}catch(error){refused.push(true);}}return refused.every(Boolean);}));
  await page.evaluate(()=>dinahWorkbench.open({kind:'extension-view',workspace:'',path:JSON.stringify(['test.fixture','follower'])}));
  await frameOf('Fixture Follower').locator('#status').filter({hasText:'ready'}).waitFor();
  ok('a newly opened frame receives the current shared map with its init message',await evalIn('Fixture Follower',()=>dinah.shared.get().selection?.file==='README.md'&&__events.shared.length===0));
  await evalIn('Fixture Window',()=>dinah.shared.set('selection',null));
  await frameOf('Fixture Pane').locator('#shared').filter({hasText:'"value":null'}).waitFor();
  ok('removing a key reaches the other frames as a null value',await evalIn('Fixture Pane',()=>!('selection' in dinah.shared.get()))&&await evalIn('Fixture Follower',()=>!('selection' in dinah.shared.get())));

  // 2. open-view reuses the open window in the same workspace; {reuse:false} opens another.
  await evalIn('Fixture Pane',()=>dinah.navigate('first-selection'));
  await evalIn('Fixture Pane',()=>dinah.hostCommand('open-view/window'));
  await frameOf('Fixture Window').locator('#state').filter({hasText:'first-selection'}).waitFor();
  await evalIn('Fixture Pane',()=>dinah.navigate('second-selection'));
  await evalIn('Fixture Pane',()=>dinah.hostCommand('open-view/window'));
  await frameOf('Fixture Window').locator('#state').filter({hasText:'second-selection'}).waitFor();
  ok('open-view raises the open window and sends it the new state instead of opening another',await framesTitled('Fixture Window').count()===1&&await page.evaluate(()=>{const s=pudlWindows.state();return s.top===document.querySelector('.extension-frame[title="Fixture Window"]').closest('.win').dataset.win;}));
  const opened=await evalIn('Fixture Pane',()=>dinah.hostCommand('open-view/window',{reuse:false}));
  await page.waitForFunction(()=>document.querySelectorAll('.extension-frame[title="Fixture Window"]').length===2);
  ok('open-view with reuse false opens a second window of the view',typeof opened.window==='string'&&await framesTitled('Fixture Window').count()===2);
  await page.evaluate(key=>pudlWindows.close(key),opened.window);
  await page.waitForFunction(()=>document.querySelectorAll('.extension-frame[title="Fixture Window"]').length===1);

  // 3. A following view takes the selected workspace, shows its name, and can be pinned.
  const followerWin=()=>page.locator('.extension-frame[title="Fixture Follower"]').locator('xpath=ancestor::section[contains(@class,"win")]');
  ok('a following window shows the workspace name as its title detail',await followerWin().getAttribute('data-win-detail')==='Fixture');
  await selectWorkspace('other');
  await frameOf('Fixture Follower').locator('#workspace').filter({hasText:'other'}).waitFor();
  await frameOf('Fixture Pane').locator('#workspace').filter({hasText:'other'}).waitFor();
  ok('the following window and the pane move with the selected workspace; the pinned window stays',await evalIn('Fixture Window',()=>dinah.workspace.id==='fixture'&&__events.workspace.length===0)&&await followerWin().getAttribute('data-win-detail')==='Other');
  ok('a frame moving to another workspace sees that workspace\'s shared map',await evalIn('Fixture Follower',()=>Object.keys(dinah.shared.get()).length===0));
  const pinRow=async()=>page.evaluate(()=>{const root=document.querySelector('.extension-frame[title="Fixture Follower"]').closest('.dinah-applet');return pudlApplets.commandsIn(root).filter(c=>c.label==='Pin to This Workspace').map(c=>({checked:c.checked}));});
  ok('the window\'s commands include an unchecked Pin to This Workspace, and the frame cannot take its ID',JSON.stringify(await pinRow())==='[{"checked":false}]'&&await evalIn('Fixture Follower',async()=>{try{await dinah.commands([{id:'pin-workspace',label:'Mine'}]);return false;}catch(error){return true;}}));
  await page.evaluate(()=>{const root=document.querySelector('.extension-frame[title="Fixture Follower"]').closest('.dinah-applet');return pudlApplets.commandsIn(root).find(c=>c.label==='Pin to This Workspace').run();});
  await frameOf('Fixture Follower').locator('#workspace').filter({hasText:'other pinned'}).waitFor();
  await selectWorkspace('fixture');
  await frameOf('Fixture Pane').locator('#workspace').filter({hasText:/^fixture$/}).waitFor();
  ok('a pinned following window keeps its workspace while the pane follows the selection',JSON.stringify(await pinRow())==='[{"checked":true}]'&&await evalIn('Fixture Follower',()=>dinah.workspace.id==='other'&&dinah.workspace.pinned===true)&&await followerWin().getAttribute('data-win-detail')==='Other');
  ok('a pin is kept with the UI state under the window\'s key',await page.evaluate(()=>{const key=document.querySelector('.extension-frame[title="Fixture Follower"]').closest('.win').dataset.win;return dinahUIState.get('dinah-extension-pins',{})[key]==='other';}));
  await page.evaluate(()=>{const root=document.querySelector('.extension-frame[title="Fixture Follower"]').closest('.dinah-applet');return pudlApplets.commandsIn(root).find(c=>c.label==='Pin to This Workspace').run();});
  await frameOf('Fixture Follower').locator('#workspace').filter({hasText:/^fixture$/}).waitFor();
  ok('unpinning lets the window follow the selected workspace again',await evalIn('Fixture Follower',()=>dinah.workspace.id==='fixture'&&dinah.workspace.pinned===false)&&await followerWin().getAttribute('data-win-detail')==='Fixture');
  // The follower is open at the view's default key but a second one in another workspace must not take it over.
  await page.evaluate(()=>{const root=document.querySelector('.extension-frame[title="Fixture Follower"]').closest('.dinah-applet');return pudlApplets.commandsIn(root).find(c=>c.label==='Pin to This Workspace').run();});
  await frameOf('Fixture Follower').locator('#workspace').filter({hasText:'fixture pinned'}).waitFor();
  await selectWorkspace('other');
  await frameOf('Fixture Pane').locator('#workspace').filter({hasText:/^other$/}).waitFor();
  await evalIn('Fixture Pane',()=>dinah.hostCommand('open-view/follower'));
  await page.waitForFunction(()=>document.querySelectorAll('.extension-frame[title="Fixture Follower"]').length===2);
  ok('open-view from another workspace opens a second following window rather than reusing the pinned one',await framesTitled('Fixture Follower').count()===2);
  await selectWorkspace('fixture');
  await frameOf('Fixture Pane').locator('#workspace').filter({hasText:/^fixture$/}).waitFor();

  // 4. open-diff validates the specification, opens the Editor's diff and routes actions back.
  await page.evaluate(()=>{window.__diffs=[];const real=window.dinahDiff;window.__realDiff=real;window.dinahDiff={open(spec){__diffs.push(spec);return real&&typeof real.open==='function'?real.open(spec):Promise.resolve();}};});
  const diffResult=await evalIn('Fixture Pane',()=>dinah.hostCommand('open-diff',{key:'README.md@index',title:'README.md',detail:'Staged',path:'README.md',language:'markdown',before:'one\n',after:'two\n',mode:'split',actions:[{id:'stage-hunk',label:'Stage Hunk',glyph:'plus',scope:'hunk',primary:true},{id:'discard',label:'Discard',scope:'line',danger:true}]}));
  ok('open-diff opens the diff under the extension\'s own namespace with its actions',diffResult.opened===true&&diffResult.diff==='README.md@index'&&await page.evaluate(()=>{const spec=__diffs[0];return spec.key==='test.fixture/README.md@index'&&spec.title==='README.md'&&spec.actions.length===2&&spec.actions[0].primary===true&&spec.actions[1].danger===true&&typeof spec.onAction==='function';}));
  await page.evaluate(()=>__diffs[0].onAction('stage-hunk',{hunk:2,from:10,to:18}));
  await frameOf('Fixture Pane').locator('#command').filter({hasText:'diff/stage-hunk'}).waitFor();
  ok('a diff action comes back to the frame as a command event with the diff key and place',await evalIn('Fixture Pane',()=>{const c=__events.command.at(-1);return c.id==='diff/stage-hunk'&&c.diff==='README.md@index'&&c.where.hunk===2&&c.where.to===18;}));
  await page.evaluate(()=>__diffs[0].onAction('undeclared',{}));
  ok('an action the specification did not declare is not routed',await evalIn('Fixture Pane',()=>__events.command.length===1));
  ok('open-diff refuses bad action IDs, oversized sides and an unknown scope',await evalIn('Fixture Pane',async()=>{const bad=[{key:'k',title:'t',before:'',after:'',actions:[{id:'Stage Hunk',label:'x',scope:'hunk'}]},{key:'k',title:'t',before:'x'.repeat(2*1024*1024+1),after:''},{key:'k',title:'t',before:'',after:'',actions:[{id:'a',label:'A',scope:'repository'}]},{key:'',title:'t',before:'',after:''}];const refused=[];for(const spec of bad){try{await dinah.hostCommand('open-diff',spec);refused.push(false);}catch(error){refused.push(true);}}return refused.every(Boolean);}));
  await page.evaluate(()=>{delete window.dinahDiff;});
  ok('without the Editor\'s diff view the command fails with a clear error',await evalIn('Fixture Pane',async()=>{try{await dinah.hostCommand('open-diff',{key:'k',title:'t',before:'',after:''});return false;}catch(error){return error.message==="The Editor's diff view is not available";}}));
  await page.evaluate(()=>{if(window.__realDiff)window.dinahDiff=window.__realDiff;});

  // Shared state lives only while a frame of the extension does.
  await page.evaluate(()=>{for(const frame of document.querySelectorAll('.extension-frame'))pudlWindows.close(frame.closest('.win').dataset.win);});
  await page.waitForFunction(()=>document.querySelectorAll('.extension-frame').length===0);
  await page.evaluate(()=>dinahWorkbench.open({kind:'extension-view',workspace:'fixture',path:JSON.stringify(['test.fixture','window'])}));
  await frameOf('Fixture Window').locator('#status').filter({hasText:'ready'}).waitFor();
  ok('shared state is dropped when the extension\'s last frame closes',await evalIn('Fixture Window',()=>Object.keys(dinah.shared.get()).length===0));
 }catch(error){ok(error.stack,false);}finally{await t.close();}
})();
