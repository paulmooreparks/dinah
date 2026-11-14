// Exercise the real Settings bridge and service using only invented temporary files.
const fs = require('node:fs'), path = require('node:path'), os = require('node:os');
const assert = require('node:assert/strict');
const { createHash } = require('node:crypto');
const { spawn } = require('node:child_process');
const { chromium } = require('playwright-core');
const [product, helper] = process.argv.slice(2);
if (!product || !helper) throw Error('Supply built product and hidden holder executables');
const root = fs.mkdtempSync(path.join(os.tmpdir(), 'dinah-set-aside-live-'));
const home = path.join(root, 'home'), bin = path.join(home, 'bin');
const entry = path.join(root, 'entry.txt');
const names = {
 held: 'dinah-desktop.exe.old-20261110010000000',
 fallback: 'dinah-desktop.exe.old-20261114010000000',
 old: 'dinah-desktop.exe.old-20261111010000000',
 manual: 'dinah-desktop.exe.before-fixture-20261115-010000'
};
let holder, child, browser;
const results = { root, checks: [] };
function check(name, value) { assert.ok(value, name); results.checks.push(name); }
const digest = p => createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const until = async (fn, label) => {
 const end = Date.now() + 120000;
 while (Date.now() < end) { if (await fn()) return; await new Promise(r => setTimeout(r, 100)); }
 throw Error(label);
};
async function stop(p) {
 if (!p || p.exitCode !== null || p.signalCode !== null) return;
 const ended = new Promise(resolve => p.once('exit', resolve));
 p.kill(); await ended;
}
(async () => {
 try {
  for (const name of ['home/bin','profile','appdata','local','tmp','workspace']) fs.mkdirSync(path.join(root,name),{recursive:true});
  const exe=path.join(root,'dinah-desktop.exe'); fs.copyFileSync(product,exe);
  fs.copyFileSync(helper,path.join(bin,names.held));
  fs.writeFileSync(path.join(bin,names.fallback),'invented fallback');
  fs.writeFileSync(path.join(bin,names.old),Buffer.alloc(1024,1));
  fs.writeFileSync(path.join(bin,names.manual),Buffer.alloc(2048,2));
  const fingerprints = Object.fromEntries(Object.entries(names).map(([key,name]) => [key,{digest:digest(path.join(bin,name)),ino:fs.statSync(path.join(bin,name),{bigint:true}).ino.toString()}]));
  const expectedTotal=Object.values(names).reduce((sum,n)=>sum+fs.statSync(path.join(bin,n)).size,0);
  fs.writeFileSync(path.join(home,'settings.json'),JSON.stringify({hostNotificationsOff:true,workspace:path.join(root,'workspace')}));
  const env={SystemRoot:process.env.SystemRoot,WINDIR:process.env.WINDIR,COMSPEC:process.env.COMSPEC,
   PATH:path.join(process.env.SystemRoot,'System32'),USERPROFILE:path.join(root,'profile'),HOME:path.join(root,'profile'),
   APPDATA:path.join(root,'appdata'),LOCALAPPDATA:path.join(root,'local'),TEMP:path.join(root,'tmp'),TMP:path.join(root,'tmp'),
   CODEX_HOME:path.join(root,'profile','.codex'),CLAUDE_CONFIG_DIR:path.join(root,'profile','.claude'),DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entry};
  holder=spawn(path.join(bin,names.held),[],{env,cwd:root,windowsHide:true,stdio:['pipe','pipe','ignore']});
  let ready=''; holder.stdout.on('data',b=>ready+=b.toString());
  await until(()=>ready.includes('ready'),'holder readiness');
  child=spawn(exe,[],{env,cwd:root,windowsHide:true,stdio:'ignore'});
  await until(()=>fs.existsSync(entry),'hidden entry');
  browser=await chromium.launch({headless:true,channel:'msedge'});
  const page=await browser.newPage({viewport:{width:1400,height:1000}});
  let errors=0; results.pageErrors=[]; page.on('pageerror',e=>{ errors++; results.pageErrors.push(String(e.message)); });
  await page.route('**/*',route=>['127.0.0.1','localhost','[::1]'].includes(new URL(route.request().url()).hostname)?route.continue():route.abort());
  await page.goto(fs.readFileSync(entry,'utf8').trim());
  await page.waitForFunction(()=>window.dinahUIState&&window.dinahWorkbench);
  await page.evaluate(()=>dinahUIState.ready);
  results.shellState=await page.evaluate(()=>({ready:document.readyState,sections:!!window.dinahSettingsSections,scripts:Array.from(document.scripts).map(s=>s.getAttribute('src')).filter(s=>s&&s.includes('shell'))}));
  await page.waitForFunction(()=>window.dinahSettingsSections, null, {timeout:15000});
  await page.evaluate(()=>dinahWorkbench.open('settings','set-aside'));
  await page.waitForFunction(()=>document.querySelector('#set-aside-status').textContent.includes('4 copies listed'));
  const inventory=await page.evaluate(()=>go.main.App.ListSetAsidePrograms());
  check('inventory matches all four invented files',JSON.stringify(inventory.items.map(x=>x.name).sort())===JSON.stringify(Object.values(names).sort())&&inventory.totalBytes===expectedTotal);
  const byName=new Map(inventory.items.map(x=>[x.name,x]));
  const row=name=>page.locator('[data-copy-id="'+byName.get(name).id+'"] input');
  check('held copy is identified and disabled',byName.get(names.held).runningPids.includes(holder.pid)&&await row(names.held).isDisabled());
  check('newest old fallback is disabled',byName.get(names.fallback).fallback&&await row(names.fallback).isDisabled());
  await row(names.old).check();await row(names.manual).check();
  check('selected total is 3072 bytes',(await page.locator('#set-aside-total').innerText()).includes('3,072 bytes'));
  await page.locator('#set-aside-remove').click();await page.locator('#ask[open]').waitFor();
  const prompt=await page.locator('#ask').innerText();
  check('confirmation names both copies and total',prompt.includes(names.old)&&prompt.includes(names.manual)&&prompt.includes('3,072 bytes'));
  check('confirmation precedes deletion',fs.existsSync(path.join(bin,names.old))&&fs.existsSync(path.join(bin,names.manual)));
  await page.locator('#ask-yes').click();
  await page.waitForFunction(()=>document.querySelector('#set-aside-results').textContent.includes('Reported removed size: 3,072 bytes.'));
  await page.waitForFunction(()=>document.querySelector('#set-aside-status').textContent.includes('2 copies listed'));
  const text=await page.locator('#set-aside-results').innerText();
  check('both results report removal',text.includes(names.old+': Removed.')&&text.includes(names.manual+': Removed.'));
  check('only the two selected files disappeared',!fs.existsSync(path.join(bin,names.old))&&!fs.existsSync(path.join(bin,names.manual)));
  for(const key of ['held','fallback']){
   const p=path.join(bin,names[key]);check(key+' retains identity and bytes',digest(p)===fingerprints[key].digest&&fs.statSync(p,{bigint:true}).ino.toString()===fingerprints[key].ino);
  }
  const after=await page.evaluate(()=>go.main.App.ListSetAsidePrograms());
  check('real service total decreased by the displayed logical bytes',after.items.length===2&&inventory.totalBytes-after.totalBytes===3072);
  check('holder remains running',holder.exitCode===null&&holder.signalCode===null);
  check('no page errors',errors===0);
  results.passed=true;
 } catch(e) {results.passed=false;results.error=String(e.stack||e);process.exitCode=1;}
 finally {
  if(browser)await browser.close();await stop(child);await stop(holder);
  results.childrenStopped=true;
  console.log(JSON.stringify(results,null,2));
 }
})();


