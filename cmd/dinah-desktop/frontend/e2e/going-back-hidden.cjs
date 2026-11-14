// Uses only invented state from going-back-trial.ps1 and a hidden old build.
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const {spawn} = require('node:child_process');
const assert = require('node:assert/strict');
const {chromium} = require('playwright-core');
const [desktop, root] = process.argv.slice(2);
if (!desktop || !root) throw Error('Supply the old executable and synthetic fixture root');
if (!path.resolve(root).toLowerCase().startsWith((path.resolve(os.tmpdir())+path.sep).toLowerCase())) throw Error('Fixture must be under TEMP');
const restored = process.argv[4] === 'restored';
const home = path.join(root, 'native'), user = path.join(root, 'isolated-user');
if (!fs.existsSync(path.join(home, 'restart-resume.json'))) throw Error('Synthetic fixture is missing');
if (restored) for (const name of ['settings.json','notifications.json']) fs.copyFileSync(path.join(home,name+'.before-decisions-2026-11-14'),path.join(home,name));
const entry = path.join(root, restored ? 'old-restored-entry.txt' : 'old-entry.txt');
if(fs.existsSync(entry)) throw Error('Use a fresh entry file');
const env = {...process.env, DINAH_HOME:home,
  DINAH_SHELL_ENTRY_FILE:entry, USERPROFILE:user, HOME:user, APPDATA:path.join(user,'roaming'),
  LOCALAPPDATA:path.join(user,'local'), CODEX_HOME:path.join(user,'.codex'),
  CLAUDE_CONFIG_DIR:path.join(user,'.claude'), XDG_CONFIG_HOME:path.join(user,'config'),
  XDG_DATA_HOME:path.join(user,'data')};
for (const key of ['USERPROFILE','APPDATA','LOCALAPPDATA','CODEX_HOME','CLAUDE_CONFIG_DIR','XDG_CONFIG_HOME','XDG_DATA_HOME']) fs.mkdirSync(env[key],{recursive:true});
const before = Object.fromEntries(['restart-resume.json','restart-awaiting.json','webview2/profiles.json','notifications.json'].map(name=>[name,fs.readFileSync(path.join(home,name),'utf8')]));
const log = fs.openSync(path.join(root,'old-native.log'),'w');
const child = spawn(path.resolve(desktop),[],{env,windowsHide:true,stdio:['ignore',log,log]});
let browser;
const receipt = {restored,pid:child.pid, isolatedHome:home, pageErrors:[]};
async function until(fn,label) {
  const end=Date.now()+60000;
  while(Date.now()<end) {const value=await fn(); if(value)return value; if(child.exitCode!==null)throw Error('Old build exited '+child.exitCode); await new Promise(resolve=>setTimeout(resolve,100));}
  throw Error('Timed out: '+label);
}
(async()=>{
  try {
    const url=await until(()=>{try{return fs.readFileSync(entry,'utf8').trim();}catch{return '';}},'old entry');
    browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});
    const page=await browser.newPage(); page.on('pageerror',error=>receipt.pageErrors.push(String(error)));
    await page.goto(url);
    await page.waitForFunction(()=>window.dinahUIState && window.dinahWorkbench);
    await page.evaluate(()=>dinahUIState.ready);
    const call=(name,...args)=>page.evaluate(({name,args})=>go.main.App[name](...args),{name,args});
    const state=await call('State');
    assert.equal(state.approvals,'ask');
    const inbox=await call('BusInboxPage','all',0);
    assert.ok(inbox.items.some(item=>item.text.includes('Which invented option?')));
    const ui=await call('ReadUIState');
    assert.ok(ui.values['dinah-decisions:request%3Afixture']);
    let notificationError='';
    try {const list=await call('Notifications'); if(restored)assert.equal(list.length,0);} catch(error) {notificationError=String(error);}
    if(restored)assert.equal(notificationError,''); else assert.ok(notificationError.includes('invalid durable notification'),notificationError);
    receipt.started=true; receipt.inboxRows=inbox.items.length; receipt.decisionDraftPresent=true;
    receipt.notificationError=notificationError;
    receipt.filesUnchanged={};
    for(const [name,value] of Object.entries(before)) {
      receipt.filesUnchanged[name]=fs.readFileSync(path.join(home,name),'utf8')===value;
      assert.ok(receipt.filesUnchanged[name],name+' changed');
    }
    const settings=JSON.parse(fs.readFileSync(path.join(home,'settings.json'),'utf8'));
    receipt.newSettingsStillPresent=Object.hasOwn(settings,'backgroundChatLimit') && Object.hasOwn(settings,'attentionReadSafe');
    assert.deepEqual(receipt.pageErrors,[]);
    console.log(JSON.stringify(receipt,null,2));
  } finally {
    fs.writeFileSync(path.join(root,restored ? 'old-restored-native-receipt.json' : 'old-native-receipt.json'),JSON.stringify(receipt,null,2));
    if(browser)await browser.close();
    if(child.exitCode===null)child.kill();
    await new Promise(resolve=>{if(child.exitCode!==null)return resolve();child.once('exit',resolve);});
    fs.closeSync(log);
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
