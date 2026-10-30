const { launch } = require('./lib/page');
const path = require('path'), fs = require('fs');
(async()=>{
 const t=await launch({
  viewport:{width:1440,height:1000},
  app:()=>{
   const workspace={id:'fixture',name:'Fixture',path:'C:/Fixture',sources:[]};
   window.__calls=[];window.__cancelled=[];window.__enabled=true;window.__nativeFull=false;window.__nativeActions=[];window.__nativeFail=false;window.__exitReplies=[];
   const manifest={id:'test.fixture',version:'0.1.0',title:'Extension Fixture',description:'Fixture',publisher:{name:'Test'},permissions:['workspace.read'],actions:[{name:'inspect'},{name:'wait'}],views:[{id:'fixture',title:'Extension Fixture',page:'index.html'},{id:'window',title:'Fixture Window',page:'index.html'}]};
   return{AnswerExitConfirmation:async(id,accepted)=>{__exitReplies.push({id,accepted});__emit('dinah:exit-confirmation-resolved',{id});},NativeWindowState:async()=>({available:true,fullscreen:__nativeFull}),NativeWindowAction:async(action)=>{__nativeActions.push(action);if(__nativeFail)throw Error('Native action failed');if(action==='toggle')__nativeFull=!__nativeFull;if(action==='restore')__nativeFull=false;},State:async()=>({workspaceId:'fixture',workspace:'C:/Fixture',recent:['C:/Fixture'],workspaces:['C:/Fixture'],workspaceRecords:[{id:'fixture',name:'Fixture',primarySourceId:'local',sources:[{id:'local',kind:'local-folder',name:'Fixture',path:'C:/Fixture'}]}]}),Extensions:async()=>[{manifest,enabled:window.__enabled}],ExtensionWorkspace:async()=>workspace,ExtensionCall:async(id,action,ws,input,requestID)=>{window.__calls.push({id,action,ws,input,requestID});if(action==='wait')return new Promise(()=>{});return{workspace:ws,answer:'ready'};},CancelExtensionCall:async(id,ws,requestID)=>window.__cancelled.push(requestID)};
  },
  // The fixture's stub runs in the extension's sandboxed frames too; an
  // extension sees no bridge, so the frames are cleared of it.
  init:()=>{if(window!==parent)for(const k of ['go','runtime','__emit','__bridgeCalls','__runtime','__state','__calls','__cancelled','__enabled','__nativeFull','__nativeActions','__nativeFail','__exitReplies'])delete window[k];},
  goto:false,
 });
 const {page,ok}=t;
 try{
  await t.context.route('https://dinah.test/**',async route=>{
   const pathname=new URL(route.request().url()).pathname;
   if(pathname==='/extension-sdk/dinah-extension.js')return route.fulfill({contentType:'text/javascript',body:fs.readFileSync(path.resolve(__dirname,'../../../../../dinah-extension-go/web/dinah-extension.js'),'utf8')+'\n'+fs.readFileSync(path.resolve(__dirname,'../native-frame.js'),'utf8'),headers:{'Access-Control-Allow-Origin':'*'}});
   if(pathname.endsWith('/fixture.js'))return route.fulfill({contentType:'text/javascript',body:`dinah.ready.then(async()=>{document.querySelector('output').textContent=JSON.stringify(await dinah.call('inspect',{}));await dinah.commands([{id:'refresh',label:'Refresh fixture'}]);});dinah.on('command',()=>{document.querySelector('output').textContent='command received';});document.querySelector('#navigate').onclick=()=>dinah.navigate('selected-file');document.querySelector('#pending').onclick=()=>dinah.call('wait',{}).catch(()=>{});`});
   if(pathname.startsWith('/extension-assets/'))return route.fulfill({contentType:'text/html',headers:{'Content-Security-Policy':"sandbox allow-scripts allow-forms; default-src 'none'; script-src https://dinah.test/extension-assets/ https://dinah.test/extension-sdk/; style-src 'unsafe-inline'; form-action 'none'; frame-ancestors 'self'"},body:'<!doctype html><main tabindex="-1" data-extension-focus><label>Name <input id="name"></label><button id="navigate">Navigate</button><button id="pending">Pending call</button><output></output></main><script defer src="/extension-sdk/dinah-extension.js"></script><script defer src="fixture.js"></script>'});
   const file=path.join(__dirname,'..',pathname);
   // A script or stylesheet the page lists before its file lands is a 404, as it would be from the host, not a failed route.
   return fs.existsSync(file)?route.fulfill({path:file}):route.fulfill({status:404,body:''});
  });
  await page.goto('https://dinah.test/index.html');
  await page.waitForFunction(()=>window.dinahExtensions?.catalog().length===1);
  await page.evaluate(()=>dinahWorkbench.open({kind:'extension-view',workspace:'fixture',path:JSON.stringify(['test.fixture','fixture'])}));
  const frame=page.frameLocator('.extension-frame');await frame.locator('output').filter({hasText:'ready'}).waitFor();
  ok('sandboxed frame calls only its declared backend with host workspace',await page.evaluate(()=>__calls[0].ws==='fixture'&&__calls[0].id==='test.fixture'));
  ok('opaque frame cannot inspect parent DOM or storage',await frame.locator('main').evaluate(()=>{let dom=false,storage=false;try{parent.document.body;}catch{dom=true;}try{localStorage.getItem('x');}catch{storage=true;}return dom&&storage&&window.go===undefined;}));
  ok('appearance arrives before frame is revealed',await frame.locator('html').evaluate(n=>['light','dark'].includes(n.dataset.theme)&&!!n.style.getPropertyValue('--accent')));
  ok('undeclared action is refused by the parent bridge',await frame.locator('main').evaluate(async()=>{try{await dinah.call('undeclared',{});return false;}catch(e){return e.message.includes('not declared');}}));
  await page.waitForFunction(()=>dinahCommands.rows('view').some(row=>row.label==='Full Screen'));
  await frame.locator('#name').press('F11');
  await page.locator('.native-window-controls:not([hidden])').waitFor();
  ok('native controls match normal window button dimensions and shape',await page.evaluate(()=>{const native=document.querySelector('.native-window-controls [data-native-action="minimize"]'),normal=document.querySelector('.win[data-win-kind="extension-view"] .win-btn[data-win-action="minimize"]');const a=getComputedStyle(native),b=getComputedStyle(normal);return ['width','height','borderRadius','backgroundImage','boxShadow'].every(key=>a[key]===b[key]);}));
  ok('native application menu uses the Dinah icon',await page.locator('.native-window-icon').evaluate(n=>n.getAttribute('src')==='/dinah-mark.svg'&&n.naturalWidth>0));
  await t.shot('native-controls-light');
  const mode=await page.evaluate(()=>{const old=document.documentElement.dataset.theme;document.documentElement.dataset.theme='dark';return old;});
  await t.shot('native-controls-dark');
  await page.evaluate(mode=>document.documentElement.dataset.theme=mode,mode);

  ok('F11 inside an opaque extension enters native fullscreen',await page.evaluate(()=>__nativeActions.at(-1)==='toggle'&&__nativeFull&&document.fullscreenElement===null));
  await frame.locator('#name').fill('native focus');
  await page.locator('.native-window-menu > button').click();
  await page.locator('.native-window-menu .menu-panel:popover-open').waitFor();
  await page.keyboard.press('Escape');await page.keyboard.press('Escape');
  await page.waitForFunction(()=>document.activeElement?.classList.contains('extension-frame'));
  await frame.locator('#name').press('End');await page.keyboard.type(' returned');
  ok('native menu returns focus to the extension input through its token',await frame.locator('#name').inputValue()==='native focus returned');
  ok('View command shows F11 and checked fullscreen state',await page.evaluate(()=>{const row=dinahCommands.rows('view').find(row=>row.label==='Full Screen');return row.shortcut==='F11'&&row.checked;}));
  await page.locator('nav[data-menubar] .menubar-title').first().focus();
  await page.keyboard.press('F11');await page.locator('.native-window-controls').waitFor({state:'hidden'});
  ok('F11 exits fullscreen from the host document',await page.evaluate(()=>!__nativeFull));
  await page.locator('nav[data-menubar] .menubar-title').filter({hasText:/^View$/}).click();
  const fullscreenRow=page.locator('#menubar-panel [role^="menuitem"]').filter({hasText:'Full Screen'});
  await fullscreenRow.waitFor();
  const fullscreenMenuState={checked:await fullscreenRow.getAttribute('aria-checked'),text:await fullscreenRow.innerText()};
  ok('View menu exposes the unchecked Full Screen command with F11',fullscreenMenuState.checked==='false'&&fullscreenMenuState.text.includes('F11'));
  if(fullscreenMenuState.checked!=='false'||!fullscreenMenuState.text.includes('F11'))console.log('Fullscreen menu state: '+JSON.stringify(fullscreenMenuState));
  await fullscreenRow.click();await page.locator('.native-window-controls:not([hidden])').waitFor();
  await page.locator('nav[data-menubar] .menubar-title').filter({hasText:/^View$/}).click();
  await fullscreenRow.waitFor();await page.keyboard.press('F11');
  await page.locator('.native-window-controls').waitFor({state:'hidden'});
  await page.waitForFunction(()=>document.querySelector('#menubar-panel:popover-open [role=menuitemcheckbox][aria-keyshortcuts=F11]')?.getAttribute('aria-checked')==='false');
  ok('an open View menu reflects F11 leaving fullscreen',true);
  await frame.locator('#name').press('F11');await page.locator('.native-window-controls:not([hidden])').waitFor();

  await page.getByRole('button',{name:'Minimize application window',exact:true}).click();
  await page.waitForFunction(()=>__nativeActions.includes('minimize'));
  await page.getByRole('button',{name:'Close application window',exact:true}).click();
  await page.waitForFunction(()=>__nativeActions.includes('close'));
  ok('native controls invoke application minimize and close',true);
  await page.setViewportSize({width:390,height:800});
  ok('native controls reach both physical topbar edges on narrow screens',await page.locator('.topbar').evaluate(bar=>{const menu=bar.querySelector('.native-window-menu').getBoundingClientRect(),buttons=bar.querySelector('.native-window-controls').getBoundingClientRect(),box=bar.getBoundingClientRect();return Math.abs(menu.left-box.left)<2&&Math.abs(buttons.right-box.right)<2;}));
  await page.getByRole('button',{name:'Leave full screen',exact:true}).click();await page.locator('.native-window-controls').waitFor({state:'hidden'});
  ok('Restore leaves fullscreen through the native host',await page.evaluate(()=>__nativeActions.at(-1)==='restore'&&!__nativeFull));
  await page.setViewportSize({width:1440,height:1000});
  ok('modified, repeated and composing F11 do not toggle',await page.evaluate(()=>{const before=__nativeActions.length;for(const options of [{shiftKey:true},{repeat:true},{isComposing:true}])document.dispatchEvent(new KeyboardEvent('keydown',{key:'F11',bubbles:true,cancelable:true,...options}));return __nativeActions.length===before;}));
  await page.waitForFunction(()=>dinahCommands.rows('view').some(row=>row.label==='Full Screen'&&!row.disabled));
  await page.evaluate(async()=>{__nativeFail=true;try{await dinahNativeWindow.toggle();}catch(error){window.__nativeFailure=error.message;}finally{__nativeFail=false;}});
  ok('failed native action keeps authoritative mode and permits later actions',await page.evaluate(()=>__nativeFailure==='Native action failed'&&!__nativeFull));
  await frame.locator('#name').fill('before');await frame.locator('#name').press('F10');await page.waitForFunction(()=>document.activeElement?.classList.contains('menubar-title'));
  await page.waitForFunction(()=>document.activeElement?.classList.contains('menubar-title'));
  await page.keyboard.press('Escape');await frame.locator('#name').press('End');await page.keyboard.type(' after');
  ok('F10 enters host menubar and Escape explicitly restores child input',await frame.locator('#name').inputValue()==='before after');
  await page.evaluate(()=>{window.__focusReturns=0;document.addEventListener('pudl:menubar-return-focus',()=>__focusReturns++);});
  await frame.locator('#name').press('F10');await page.waitForFunction(()=>document.activeElement?.classList.contains('menubar-title'));await page.keyboard.press('ArrowDown');
  await page.locator('#menubar-panel:popover-open').waitFor();
  await page.evaluate(()=>pudlMenubar.focus({returnTo:document.querySelector('[data-win="workspace"] button')}));
  await page.keyboard.press('Escape');await page.keyboard.press('Escape');
  // The assertion is unchanged; a failure now says which element had focus in the frame and in the page.
  const frameFocus=await frame.locator('#name').evaluate(n=>({same:document.activeElement===n,active:document.activeElement?document.activeElement.tagName+(document.activeElement.id?'#'+document.activeElement.id:''):'none'}));
  const pageFocus=await page.evaluate(()=>{const el=document.activeElement;return el?el.tagName+(el.id?'#'+el.id:'')+(el.className&&typeof el.className==='string'?'.'+el.className.split(' ')[0]:''):'none';});
  ok('repeated entry from a menu panel retains the original frame return target',frameFocus.same,'the frame had focus on '+frameFocus.active+' and the page on '+pageFocus);
  await frame.locator('#name').press('F10');await page.waitForFunction(()=>document.activeElement?.classList.contains('menubar-title'));
  await page.evaluate(()=>{document.querySelector('.extension-frame').hidden=true;window.__returnsBefore=__focusReturns;});
  await page.keyboard.press('Escape');
  ok('hidden frame does not receive a focus-return event',await page.evaluate(()=>__focusReturns===__returnsBefore));
  await page.evaluate(()=>{document.querySelector('.extension-frame').hidden=false;});
  ok('hidden menubar reports failed explicit entry',await page.evaluate(()=>{const bar=document.querySelector('[data-menubar]');bar.hidden=true;const result=pudlMenubar.focus({returnTo:document.querySelector('.extension-frame')});bar.hidden=false;return result===false;}));
  const nativeTarget=await page.evaluate(()=>{const details=document.createElement('details'),summary=document.createElement('summary');summary.textContent='Focus target';summary.id='extension-native-summary';details.append(summary);document.body.prepend(details);return pudlMenubar.focus({returnTo:summary});});
  await page.keyboard.press('Escape');
  ok('native summary is accepted as an explicit focus return target',nativeTarget&&await page.evaluate(()=>document.activeElement?.id==='extension-native-summary'));
  await page.evaluate(()=>{const node=document.createElement('div');node.id='extension-display-target';node.textContent='Display';node.setAttribute('contenteditable','false');document.body.prepend(node);window.__returnsBefore=__focusReturns;pudlMenubar.focus({returnTo:node});});
  await page.keyboard.press('Escape');
  ok('unfocusable contenteditable=false target emits no focus-return event',await page.evaluate(()=>{const node=document.querySelector('#extension-display-target'),valid=__focusReturns===__returnsBefore&&document.activeElement!==node;node.remove();return valid;}));
  await page.locator('#extension-native-summary').evaluate(n=>n.parentElement.remove());
  await frame.locator('#navigate').focus();await frame.locator('#navigate').press('Enter');await page.waitForFunction(()=>location.href.includes('selected-file'));
  ok('extension navigation becomes host URL state',page.url().includes('selected-file'));
  await frame.locator('main').evaluate(()=>dinah.hostCommand('open-view/window'));
  const other=page.frameLocator('.extension-frame[title="Fixture Window"]');await other.locator('output').filter({hasText:'ready'}).waitFor();
  ok('declared view opens as a separate window with addressable state',await other.locator('main').evaluate(async()=>(await dinah.ready).state)==='selected-file');
  await page.locator('.extension-frame[title="Fixture Window"]').evaluate(n=>parent.pudlWindows.close(n.closest('.win').dataset.win));
  await page.setViewportSize({width:390,height:800});
  ok('frame remains inside narrow host window',await page.locator('.extension-frame').evaluate(n=>n.getBoundingClientRect().width<=innerWidth));
  await page.setViewportSize({width:1440,height:1000});
  await frame.locator('#pending').focus();await frame.locator('#pending').press('Enter');await page.waitForFunction(()=>__calls.some(c=>c.action==='wait'));
  await page.evaluate(()=>{__enabled=false;__emit('extensions:changed',{});});
  await page.locator('.extension-window').getByText('This extension was disabled, removed or updated. Close and reopen its window.').waitFor();
  ok('disabling invalidates frame and cancels pending invocation',await page.evaluate(()=>__cancelled.length===1));
  const exitDialog=page.getByRole('dialog',{name:'Exit Dinah?'});
  await page.evaluate(()=>__emit('dinah:exit-confirmation',{id:'exit-default',status:{chats:2,terminals:1,nonResumableActions:true,unfinishedRequests:1}}));
  await exitDialog.waitFor();
  ok('exit confirmation identifies active chats, terminal sessions and unfinished actions',await exitDialog.innerText().then(text=>text.includes('Active chats: 2')&&text.includes('Native terminal sessions: 1')&&text.includes('not finished')));
  ok('Cancel is the focused default exit choice',await exitDialog.getByRole('button',{name:'Cancel',exact:true}).evaluate(n=>document.activeElement===n));
  await t.shot('exit-confirmation');
  await page.keyboard.press('F11');await page.locator('.native-window-controls:not([hidden])').waitFor();
  await page.keyboard.press('F11');await page.locator('.native-window-controls').waitFor({state:'hidden'});
  ok('F11 remains available without dismissing the exit confirmation',await exitDialog.isVisible());

  await page.keyboard.press('Enter');await exitDialog.waitFor({state:'hidden'});
  ok('Enter on the default button cancels exit',await page.evaluate(()=>__exitReplies.at(-1).accepted===false));
  await page.evaluate(()=>__emit('dinah:exit-confirmation',{id:'exit-escape',status:{chats:1}}));await exitDialog.waitFor();await page.keyboard.press('Escape');await exitDialog.waitFor({state:'hidden'});
  ok('Escape cancels exit without interrupting work',await page.evaluate(()=>__exitReplies.at(-1).id==='exit-escape'&&!__exitReplies.at(-1).accepted));
  await page.evaluate(()=>__emit('dinah:exit-confirmation',{id:'exit-confirm',status:{nonResumableActions:true}}));await exitDialog.waitFor();
  await exitDialog.getByRole('button',{name:'Exit Dinah',exact:true}).click();await exitDialog.waitFor({state:'hidden'});
  ok('Exit Dinah sends explicit confirmation',await page.evaluate(()=>__exitReplies.at(-1).id==='exit-confirm'&&__exitReplies.at(-1).accepted));
 }catch(error){ok(error.stack,false);}finally{await t.close();}
})();
