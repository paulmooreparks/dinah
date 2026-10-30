// Drive a real native host through its authenticated page in isolated state.
const {chromium}=require('playwright-core'),fs=require('fs'),path=require('path'),{execFileSync}=require('child_process');
(async()=>{
 const root=path.resolve(process.argv[2]||'');
 if(!path.basename(root).startsWith('dinah-fullscreen-live-'))throw Error('A disposable fullscreen test directory is required.');
 const probe=(action='inspect')=>JSON.parse(execFileSync('pwsh',['-NoProfile','-File',path.join(__dirname,'native-window-probe.ps1'),'-Root',root,'-Action',action],{encoding:'utf8'}));
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});
 const check=(value,label)=>{if(!value)throw Error(label);console.log('PASS '+label);};
 try{
  const context=await browser.newContext({viewport:{width:1440,height:1000},...(fs.existsSync(path.join(root,'browser-auth.json'))?{storageState:path.join(root,'browser-auth.json')}:{})}),page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  const entry=fs.readFileSync(path.join(root,'entry.txt'),'utf8');await page.goto(fs.existsSync(path.join(root,'browser-auth.json'))?new URL('/',entry).href:entry);await context.storageState({path:path.join(root,'browser-auth.json')});await page.waitForFunction(()=>window.dinahNativeWindow&&window.go?.main?.App);
  await page.waitForFunction(()=>dinahCommands.rows('view').some(row=>row.label==='Full Screen'));
  check(!(await page.evaluate(()=>go.main.App.NativeWindowState())).fullscreen,'native host begins outside fullscreen');
  const placement=probe();
  await page.evaluate(()=>{const input=document.createElement('input');input.id='fullscreen-fixture-input';input.setAttribute('aria-label','Fullscreen fixture');document.querySelector('.topbar').append(input);});
  const input=page.locator('#fullscreen-fixture-input');await input.fill('Preserve this draft');await input.press('F11');
  await page.locator('.native-window-controls:not([hidden])').waitFor();
  check((await page.evaluate(()=>go.main.App.NativeWindowState())).fullscreen,'F11 enters real native fullscreen');
  check(!probe().caption,'native fullscreen removes the OS titlebar');
  check(await input.inputValue()==='Preserve this draft'&&await input.evaluate(n=>document.activeElement===n),'native transition preserves draft and focus');
  check(await page.evaluate(()=>document.fullscreenElement===null),'browser fullscreen remains unused');
  await page.screenshot({path:path.join(root,'fullscreen.png')});
  await input.press('F11');await page.locator('.native-window-controls').waitFor({state:'hidden'});
  check(!(await page.evaluate(()=>go.main.App.NativeWindowState())).fullscreen,'F11 leaves real native fullscreen');
  const restored=probe();check(restored.caption&&['left','top','width','height'].every(key=>Math.abs(restored[key]-placement[key])<=2),'native normal placement is restored');
  check(probe('maximize').maximized,'fixture is natively maximized before the next transition');
  await page.evaluate(()=>dinahCommands.run('fullscreen'));await page.locator('.native-window-controls:not([hidden])').waitFor();
  await page.getByRole('button',{name:'Minimize application window',exact:true}).click();
  check(probe().minimized,'PUDL Minimize minimizes the real native window');probe('unminimize');
  await page.getByRole('button',{name:'Leave full screen',exact:true}).click();await page.locator('.native-window-controls').waitFor({state:'hidden'});
  check(!(await page.evaluate(()=>go.main.App.NativeWindowState())).fullscreen,'PUDL Restore leaves real native fullscreen');
  check(probe().maximized,'leaving fullscreen restores native maximization');
  check(errors.length===0,'native host page has no errors');
 }finally{try{probe('hide');}catch{}await browser.close();}
})().catch(error=>{console.error(error.stack);process.exitCode=1;});
