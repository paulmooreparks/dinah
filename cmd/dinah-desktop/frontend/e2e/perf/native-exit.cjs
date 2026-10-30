const {chromium}=require('playwright-core'),fs=require('fs'),path=require('path');
(async()=>{
 const root=path.resolve(process.argv[2]||'');if(!path.basename(root).startsWith('dinah-exit-live-'))throw Error('A disposable exit test directory is required.');
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});
 const check=(value,label)=>{if(!value)throw Error(label);console.log('PASS '+label);};

 try {
  const page=await browser.newPage();await page.goto(fs.readFileSync(path.join(root,'entry.txt'),'utf8'));
  await page.waitForFunction(()=>window.dinahNativeWindow&&window.go?.main?.App);
  const token=await page.evaluate(()=>go.main.App.BeginExitWork('Native exit smoke test'));
  const first=page.evaluate(()=>{window.__exitAttempt=1;return go.main.App.NativeWindowAction('close');});await page.waitForFunction(()=>window.__exitAttempt===1);
  await page.getByRole('dialog',{name:'Exit Dinah?'}).getByRole('button',{name:'Cancel',exact:true}).click();await first;
  check((await page.evaluate(()=>go.main.App.ExitStatus())).unfinishedRequests>0,'Cancel preserves the running activity and native host');
  const second=page.evaluate(()=>{window.__exitAttempt=2;return go.main.App.NativeWindowAction('close');}).catch(()=>{});await page.waitForFunction(()=>window.__exitAttempt===2);
  await page.getByRole('dialog',{name:'Exit Dinah?'}).getByRole('button',{name:'Exit Dinah',exact:true}).click();await second;
  const quit=await page.waitForFunction(async()=>{try{await go.main.App.NativeWindowState();return false;}catch{return true;}},{},{timeout:10000});
  check(!!quit&&!!token&&JSON.parse(fs.readFileSync(path.join(root,'state/native-window.json'),'utf8')).version===1,'confirmed exit flushes state and closes the native host');
 }finally{await browser.close();}
})().catch(error=>{console.error(error.stack);process.exitCode=1;});
