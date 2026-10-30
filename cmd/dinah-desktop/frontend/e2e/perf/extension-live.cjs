// Run only against a disposable DINAH_HOME and disposable Git repository.
// Arguments are the isolated instance directory and a signed Git package.
const {chromium}=require('playwright-core'),fs=require('fs'),path=require('path'),{execFileSync}=require('child_process');
(async()=>{
 const root=path.resolve(process.argv[2]||''),packageRoot=path.resolve(process.argv[3]||'');
 if(!path.basename(root).startsWith('dinah-extension-live-'))throw Error('A disposable extension test directory is required.');
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});
 const errors=[];const assert=(value,label)=>{if(!value)throw Error(label);console.log('PASS '+label);};
 try{
  const context=await browser.newContext({viewport:{width:1440,height:1000},...(fs.existsSync(path.join(root,'browser-auth.json'))?{storageState:path.join(root,'browser-auth.json')}:{})}),page=await context.newPage();page.setDefaultTimeout(20000);page.on('pageerror',e=>errors.push(e.message));
  page.on('console',message=>{if(message.type()==='error')console.log('BROWSER '+message.text());});
  const entry=fs.readFileSync(path.join(root,'entry.txt'),'utf8');await page.goto(fs.existsSync(path.join(root,'browser-auth.json'))?new URL('/',entry).href:entry);await context.storageState({path:path.join(root,'browser-auth.json')});
  await page.waitForFunction(()=>window.go?.main?.App&&window.dinahWorkbench);
  const repository=fs.mkdtempSync(path.join(root,'repository-'));
  const git=(...args)=>execFileSync('git',['-C',repository,...args],{stdio:'pipe'});
  git('init','-q');git('config','user.name','Extension Test');git('config','user.email','test@example.invalid');git('config','commit.gpgsign','false');
  fs.writeFileSync(path.join(repository,'notes.txt'),'first\nsecond\nthird\n');git('add','--','notes.txt');git('commit','-qm','Initial fixture');fs.writeFileSync(path.join(repository,'notes.txt'),'first\nchanged\nthird\n');
  await page.evaluate(async({repository,packageRoot})=>{const app=go.main.App;for(const terminal of document.querySelectorAll('.win[data-win-kind="extension-terminal"]'))pudlWindows.close(terminal.dataset.win);for(const frame of document.querySelectorAll(".extension-frame"))pudlWindows.close(frame.closest(".win").dataset.win);if((await app.Extensions()).length)await app.RemoveExtension("com.parkscomputing.dinah-git");await app.SetWorkspace(repository);const preview=await app.PreviewExtension(packageRoot);await app.InstallExtension(packageRoot,{fingerprint:preview.fingerprint,signature:preview.manifest.signature});},{repository,packageRoot});
  await page.reload();await page.waitForFunction(()=>window.dinahExtensions?.catalog().length===1);
  await page.evaluate(()=>dinahWorkbench.launchers().find(l=>l.id.startsWith('extension-com.parkscomputing.dinah-git')).run());
  const frame=page.frameLocator('.extension-frame');
  await frame.locator('#repository option').first().waitFor({state:'attached'});
  await frame.locator('html:not([data-busy])').waitFor();
  await frame.locator('#sections').getByRole('button',{name:'Changes',exact:true}).click();
  await frame.getByRole('button',{name:'notes.txt',exact:false}).first().waitFor();
  assert(await frame.locator('#summary').textContent(),'actual MCP backend returns repository status through authenticated host');
  await frame.getByRole('button',{name:'notes.txt',exact:false}).first().click();
  await frame.getByRole('button',{name:'Stage this hunk',exact:true}).waitFor();
  assert((await frame.locator('.patch').textContent()).includes('+changed'),'actual file diff is rendered');
  await frame.getByRole('button',{name:'Open Diff Window',exact:true}).click();
  const diffWindow=page.frameLocator('.extension-frame[title="Git Window"]');await diffWindow.locator('.patch').filter({hasText:'+changed'}).waitFor();
  assert(true,'selected diff opens in a separate workspace-pinned window');
  await page.locator('.extension-frame[title="Git Window"]').evaluate(n=>pudlWindows.close(n.closest('.win').dataset.win));
  await frame.locator('.hunk-choice input[type=checkbox]').check();await frame.getByRole('button',{name:'Stage this hunk',exact:true}).click();
  await frame.locator('#list').getByText('Staged (1)',{exact:true}).waitFor({state:'attached'});
  await frame.locator('#message').fill('Commit through extension');await frame.getByRole('button',{name:'Commit staged changes',exact:true}).click();
  await frame.getByText('Working tree clean.',{exact:true}).waitFor({state:'attached'});
  assert(true,'hunk stage and commit complete through extension process');
  await frame.locator('#sections').getByRole('button',{name:'History',exact:true}).click();
  await frame.getByRole('button',{name:/Commit through extension/}).waitFor();
  assert(true,'history includes the extension-created commit');
  await frame.locator('#sections').getByRole('button',{name:'Commands',exact:true}).click();
  await frame.locator('.log-entry').first().waitFor();assert(await frame.locator('.log-entry').count()>5,'host retains real command arguments and output');
  await page.screenshot({path:path.join(root,'git-commands.png')});
  await frame.getByRole('button',{name:'Run in Terminal…',exact:true}).first().click();await page.locator('.win').filter({hasText:'Review the prepared command. It has not run.'}).waitFor();
  assert(true,'Run in Terminal prepares the command without executing it');
  await page.evaluate(()=>pudlWindows.raise([...document.querySelectorAll('.win')].find(n=>n.querySelector('.extension-frame')).dataset.win));
  await page.evaluate(()=>go.main.App.SetExtensionCommandReview('com.parkscomputing.dinah-git',true));
  await frame.locator('#sections').getByRole('button',{name:'Changes',exact:true}).click();
  const review=page.getByRole('dialog').filter({hasText:'Arguments (JSON array)'});await review.waitFor();
  assert((await review.locator('textarea').inputValue()).includes('rev-parse'),'Show Me First intercepts actual host command');
  await review.getByRole('button',{name:'Cancel',exact:true}).click();await frame.locator('#notice.error').waitFor();
  await page.evaluate(()=>go.main.App.SetExtensionCommandReview('com.parkscomputing.dinah-git',false));
  await page.setViewportSize({width:390,height:800});await page.screenshot({path:path.join(root,'git-narrow.png')});
  assert(await frame.locator('main').evaluate(n=>n.scrollWidth<=n.clientWidth+1),'Git view fits a narrow frame');
  assert(errors.length===0,'no page errors: '+errors.join('; '));
  const cli=path.join(root,'dinah.exe');
  if(fs.existsSync(cli)){
   const runCLI=(...args)=>execFileSync(cli,['ext',...args],{env:{...process.env,DINAH_HOME:path.join(root,'state')},stdio:'pipe'});
   runCLI('disable','com.parkscomputing.dinah-git');
   await page.waitForFunction(async()=>!(await go.main.App.Extensions())[0].enabled);
   runCLI('enable','com.parkscomputing.dinah-git');
   await page.waitForFunction(()=>dinahExtensions.catalog().some(e=>e.enabled));
   await page.evaluate(async()=>{for(const win of document.querySelectorAll('.win[data-win-kind="extension-view"]'))pudlWindows.close(win.dataset.win);await dinahWorkbench.launchers().find(l=>l.id.startsWith('extension-com.parkscomputing.dinah-git')).run();});
   await frame.locator('#repository option').first().waitFor({state:'attached'});await frame.locator('html:not([data-busy])').waitFor();
   execFileSync(cli,['ext','remove','com.parkscomputing.dinah-git'],{env:{...process.env,DINAH_HOME:path.join(root,'state')},input:'com.parkscomputing.dinah-git\n',stdio:'pipe'});
   await page.waitForFunction(async()=>(await go.main.App.Extensions()).length===0);
   assert(true,'CLI disable, enable and removal coordinate with a running native host');
   runCLI('install','--accept-key','f0dd31aa88467c21dd6d59f887843348b475b973cf70ed888d2168453f332d08',packageRoot);
   await page.waitForFunction(()=>dinahExtensions.catalog().some(e=>e.enabled));
   await page.evaluate(async()=>{for(const win of document.querySelectorAll('.win[data-win-kind="extension-view"]'))pudlWindows.close(win.dataset.win);await dinahWorkbench.launchers().find(l=>l.id.startsWith('extension-com.parkscomputing.dinah-git')).run();});
   await frame.locator('#repository option').first().waitFor({state:'attached'});await frame.locator('html:not([data-busy])').waitFor();
   assert(true,'CLI reinstallation resumes the native backend');
  }
 }finally{await browser.close();}
})().catch(error=>{console.error(error.stack);process.exitCode=1;});
