// Drives only the trial's hidden native WebView2, never a desktop input device.
const fs = require('node:fs');
const path = require('node:path');
const net = require('node:net');
const {spawn} = require('node:child_process');
const assert = require('node:assert/strict');
const {chromium} = require('playwright-core');
const [first, second, output, manifestFile] = process.argv.slice(2);
if (!first || !second || !output || !manifestFile) throw Error('expected two executables, output folder and asset manifest');
const out = path.resolve(output);
const home = path.join(out, 'home');
const profileRoot = path.join(home, 'webview2');
const versions = ['0.0.0-profiletrial.1', '0.0.0-profiletrial.2'];
const marker = 'profile-trial-storage-marker';
const key = 'dinah-profile-trial';
const database = 'dinah-profile-trial';
const assets = JSON.parse(fs.readFileSync(manifestFile, 'utf8'));
const report = {versions, seededLegacyFolder:true, runs:[], limitations:[]};
fs.mkdirSync(path.join(profileRoot, versions[0]), {recursive:true});
const children = [];
const nativeProbe = process.env.PROFILE_TRIAL_NATIVE === '1';
report.storageBrowser = nativeProbe ? 'native WebView2 over CDP' : 'separate persistent headless Edge';
if(!nativeProbe)report.limitations.push('The production WebView2 loader clears debugging overrides. Storage is exercised in a separate persistent headless Edge profile, not the native WebView2 profile. Native storage continuity is not proved.');
function save() {fs.writeFileSync(path.join(out, 'receipt.json'), JSON.stringify(report, null, 2));}
function read(file) {try {return fs.readFileSync(file, 'utf8');} catch {return '';}}
async function until(fn, label, milliseconds=60000) {
 const end = Date.now() + milliseconds;
 while (Date.now() < end) {const value = await fn(); if (value) return value; await new Promise(resolve => setTimeout(resolve, 100));}
 throw Error('Timed out: ' + label);
}
async function command(executable, args) {
 return new Promise((resolve, reject) => {
  const child = spawn(executable, args, {windowsHide:true, stdio:['ignore','pipe','pipe']});
  let stdout='', stderr='';
  child.stdout.on('data', data => stdout += data); child.stderr.on('data', data => stderr += data);
  child.on('error', reject); child.on('exit', code => code === 0 ? resolve(stdout) : reject(Error(stderr || 'helper exited '+code)));
 });
}
async function freePort() {
 const server=net.createServer();
 await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});
 const port=server.address().port;
 await new Promise(resolve=>server.close(resolve));return port;
}
async function storage(page, write) {
 return page.evaluate(async ({write,key,database,marker}) => {
  if (write) localStorage.setItem(key, marker);
  const names = await indexedDB.databases();
  let indexedValue=null;
  if (write || names.some(x=>x.name===database)) {
   indexedValue = await new Promise((resolve,reject) => {
    const request=indexedDB.open(database,1);
    request.onupgradeneeded=()=>request.result.createObjectStore('values');
    request.onerror=()=>reject(request.error);
    request.onsuccess=()=>{
     const db=request.result;
     const transaction=db.transaction('values',write?'readwrite':'readonly');
     const store=transaction.objectStore('values');
     if(write)store.put(marker,key);
     const get=store.get(key); let value;
     get.onsuccess=()=>value=get.result;
     transaction.oncomplete=()=>{db.close();resolve(value??null);};
     transaction.onerror=()=>{db.close();reject(transaction.error);};
     transaction.onabort=()=>{db.close();reject(transaction.error||Error('transaction aborted'));};
    };
   });
  }
  return {origin:location.origin,localValue:localStorage.getItem(key),indexedValue,databasePresent:write||names.some(x=>x.name===database)};
 }, {write,key,database,marker});
}
async function trial(executable,index) {
 const run={version:versions[index],responses:[],pageErrors:[]}; report.runs.push(run);save();
 const entryFile=path.join(out,'entry-'+index+'.txt');
 const debugPort=await freePort();
 const log=fs.openSync(path.join(out,'desktop-'+index+'.log'),'w');
 const child=spawn(path.resolve(executable),[],{windowsHide:true,stdio:['ignore',log,log],env:{...process.env,DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entryFile,WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS:'--remote-debugging-port='+debugPort}});
 fs.closeSync(log);children.push(child);run.nativePid=child.pid;
 let browser;
 try {
  const entry=await until(()=>read(entryFile).trim(),'authenticated entry');
  let context;
  if(nativeProbe) {
   await until(async()=>{try {return (await fetch('http://127.0.0.1:'+debugPort+'/json/version')).ok;} catch {return false;}},'native WebView2 debugging endpoint');
   browser=await chromium.connectOverCDP('http://127.0.0.1:'+debugPort);
   context=browser.contexts()[0];
  } else {
   browser=await chromium.launchPersistentContext(path.join(out,'headless-driver-profile'),{channel:'msedge',headless:true});
   context=browser;
  }
  const page=await until(()=>context.pages()[0],'probe page');
  page.on('pageerror',error=>run.pageErrors.push(String(error)));
  page.on('response',response=>{
   const url=new URL(response.url());
   if(url.protocol==='http:'||url.protocol==='https:') run.responses.push({path:url.pathname,status:response.status(),cacheControl:response.headers()['cache-control']??null});
  });
  if(nativeProbe) {
   run.bootstrapOrigin=new URL(page.url()).origin;
   const bootstrap=await page.reload();
   run.bootstrapCacheControl=bootstrap?.headers()['cache-control']??null;
  }
  await page.goto(entry);
  await page.waitForFunction(()=>window.dinahUIState && window.dinahWorkbench);
  await page.evaluate(()=>dinahUIState.ready);
  run.reportedVersion=await page.evaluate(async()=> (await go.main.App.State()).version);
  assert.ok(run.reportedVersion===versions[index] || run.reportedVersion.startsWith(versions[index]+' ('),'reported version has the injected version and optional build identity');
  run.storage=await storage(page,index===0);
  run.origin=run.storage.origin;run.port=new URL(run.origin).port;
  run.profileRecord=JSON.parse(fs.readFileSync(path.join(profileRoot,'profiles.json'),'utf8'));
  run.profileDirectories=fs.readdirSync(profileRoot,{withFileTypes:true}).filter(x=>x.isDirectory()).map(x=>x.name).sort();
  run.processState=JSON.parse(await command('powershell.exe',['-NoProfile','-NonInteractive','-File',path.join(__dirname,'profile-process-state.ps1'),'-NativePid',String(child.pid),'-ProfileRoot',profileRoot]));
  assert.ok(run.processState.windows.length>0,'native window was enumerated');
  assert.ok(run.processState.windows.every(visible=>visible===false),'all fixture native windows remain hidden');
  const expected=path.join(profileRoot,run.profileRecord.main).toLowerCase();
  assert.ok(run.processState.webviews.some(x=>{
   const relative=path.relative(expected,x.userDataPath.toLowerCase());
   return relative===''||(!relative.startsWith('..')&&!path.isAbsolute(relative));
  }),'native WebView2 uses the recorded directory or its own descendant');
  run.assetResponses=[];
  for(let offset=0;offset<assets.length;offset+=8) {
   const batch=await Promise.all(assets.slice(offset,offset+8).map(async name=>{
    const response=await context.request.get(run.origin+'/'+name.split('/').map(encodeURIComponent).join('/'));
    const result={path:name,status:response.status(),cacheControl:response.headers()['cache-control']??null};
    await response.dispose();return result;
   }));
   run.assetResponses.push(...batch);
  }
  assert.ok(run.assetResponses.length>0);
  assert.ok(run.assetResponses.every(x=>x.status===200&&x.cacheControl==='no-store'),'every embedded page file returns 200 and no-store');
  run.nonNoStoreResponses=run.responses.filter(x=>x.cacheControl!=='no-store');
  assert.ok(run.responses.filter(x=>x.status>=200&&x.status<300).every(x=>x.cacheControl==='no-store'),'successful observed page responses use no-store');
  if(nativeProbe)assert.equal(run.bootstrapCacheControl,'no-store');
  if(index===0){assert.equal(run.storage.localValue,marker);assert.equal(run.storage.indexedValue,marker);}
  save();
  // The normal runtime quit performs the host/page flush handshake.
  await page.evaluate(()=>window.runtime.Quit()).catch(error=>{run.quitCallError=String(error);});
  await until(()=>child.exitCode!==null,'normal native process exit',30000);
  run.exitCode=child.exitCode;assert.equal(child.exitCode,0);
  await until(async()=>{
   const state=JSON.parse(await command('powershell.exe',['-NoProfile','-NonInteractive','-File',path.join(__dirname,'profile-process-state.ps1'),'-NativePid',String(child.pid),'-ProfileRoot',profileRoot]));
   return state.webviews.length===0;
  },'fixture WebView2 processes exit',15000);
  run.webviewsExited=true;save();
 } finally {
  if(browser)await browser.close().catch(()=>{});
 }
}
(async()=>{
 try {
  await trial(first,0);await trial(second,1);
  const [a,b]=report.runs;
  assert.deepEqual(b.profileRecord,a.profileRecord);
  assert.equal(b.profileRecord.main,versions[0]);
  assert.ok(!b.profileDirectories.includes(versions[1]));
  report.sameOrigin=a.origin===b.origin;
  report.profileReused=true;report.newVersionFolderCreated=false;
  if(!report.sameOrigin){
   report.limitations.push('The hidden hook changed the port. This run cannot establish storage continuity across versions at the normal fixed production origin. No port was forced or storage copied.');
   assert.equal(b.storage.localValue,null);assert.equal(b.storage.indexedValue,null);
  }else{assert.equal(b.storage.localValue,marker);assert.equal(b.storage.indexedValue,marker);}
  report.completed=true;save();
  console.log(JSON.stringify({completed:true,sameOrigin:report.sameOrigin,profileReused:true,ports:report.runs.map(x=>x.port),assetsPerVersion:assets.length,storedData:report.runs.map(x=>x.storage)},null,2));
 } catch(error) {report.failure=String(error.stack||error);save();throw error;}
 finally {
  // Cleanup is confined to the two absolute fixture executable paths.
  for(const child of children)if(child.exitCode===null)child.kill();
 }
})().catch(error=>{console.error(error);process.exitCode=1;});
