// Runs only an isolated hidden product and headless Chromium.
// Usage: node restart-hidden.cjs <desktop.exe> <stand-in.exe> <output-folder>
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const [desktop, agent, out] = process.argv.slice(2);
if (!desktop || !agent || !out) throw Error('three arguments required');
fs.mkdirSync(out, {recursive:true});
const home = path.join(out, 'home');
const bin = path.join(home, 'bin');
fs.mkdirSync(bin, {recursive:true});
const exe = path.join(bin, 'dinah-desktop.exe');
fs.copyFileSync(desktop, exe);
const entryFile = path.join(out, 'entry.txt');
const env = {...process.env, DINAH_HOME: home, DINAH_SHELL_ENTRY_FILE:entryFile, RESTART_FIXTURE:out};
const log = fs.openSync(path.join(out,'desktop.log'),'w');
const child = spawn(exe, [], {env, windowsHide:true, stdio:['ignore',log,log]});
let browser;
const receipts=[];
function read(file) {try{return fs.readFileSync(file,'utf8');}catch{return '';}}
async function until(fn, label, ms=60000) {
 const end=Date.now()+ms;
 while(Date.now()<end) {const value=await fn(); if(value)return value; await new Promise(r=>setTimeout(r,100));}
 throw Error('Timed out: '+label);
}
async function call(page, name, ...args) {return Promise.race([page.evaluate(({name,args})=>go.main.App[name](...args),{name,args}),new Promise((_,reject)=>{const timer=setTimeout(()=>reject(Error('Bridge timeout: '+name)),30000);timer.unref();})]);}
async function open(entry) {
 const p=await browser.newPage();
 p.on('pageerror', e=>receipts.push({pageError:String(e)}));
 await p.goto(entry);
 await p.waitForFunction(()=>window.dinahUIState && window.dinahWorkbench);
 await p.evaluate(()=>dinahUIState.ready);
 return p;
}
(async()=>{
 try {
  const entry=await until(()=>read(entryFile).trim(),'entry');
  browser=await chromium.launch({headless:true});
  const p=await open(entry);
  await call(p,'SetAgentExecutable','devin',path.resolve(agent));
  await call(p,'SetWorkspace',out);
  const info=await call(p,'NewSessionWithProviderAt',out,'devin');
  receipts.push({session:info,oldPID:child.pid});
  const id=info.sessionId || info.id;
  assert.ok(id,'session identity');
  if (!process.env.RESTART_FIRST_TURN) {
   await call(p,'Send',id,'fixture completed first turn',null,null);
   await until(async()=>(await call(p,'UpdateState')).working===0,'first turn complete');
  }
  await call(p,'Send',id,'fixture original work',null,null);
  await until(()=>read(path.join(out,'prompts.jsonl')).includes('fixture original work'),'original prompt');
  if (process.env.RESTART_STEER) await until(()=>call(p,'CanSteer',id,null),'hook steering available');
  if (!process.env.RESTART_SAME_PROGRAM) {
  fs.renameSync(exe,exe+'.old');
  fs.copyFileSync(desktop,exe);
  fs.appendFileSync(exe,Buffer.from('\0restart fixture'));
  await until(async()=>(await call(p,'UpdateState')).ready,'update detected',30000);
  } else {
   assert.equal((await call(p,'UpdateState')).ready,false);
   receipts.push({sameProgram:true,executable:exe});
  }
  await call(p,'PauseAndRestart');
  if (!process.env.RESTART_STEER) {
  const pending=await call(p,'UpdateState');
  assert.equal(pending.pending,true); assert.equal(pending.working,1);
  assert.equal(pending.pausing,true);
  receipts.push({waiting:pending});
  }
  if (process.env.RESTART_STEER) {
   const steering=await until(()=>read(path.join(out,'steering.json')),'pause steering received');
   assert.ok(steering.includes('Stop at the next safe point'));
   receipts.push({steering:JSON.parse(steering)});
  } else if (process.env.RESTART_FINISH) {
   fs.writeFileSync(path.join(out,'finish'),'finish the stand-in turn');
  } else {
   await call(p,'RestartToUpdate',true);
   const confirm=await until(()=>call(p,'PendingExitConfirmation'),'exit confirmation');
   // Closing can tear down the bridge before its reply reaches the driver.
   call(p,'AnswerExitConfirmation',confirm.id,true).catch(()=>{});
  }
  await until(()=>child.exitCode!==null,'old process exit');
  const next=await until(()=>{const v=read(entryFile).trim();return v && v!==entry && v;},'new process entry');
  const q=await open(next);
  const expected=process.env.RESTART_STEER ? 'You paused your work' : process.env.RESTART_FINISH ? 'Your last turn ended just before' : 'cut short';
  receipts.push({oldExited:true,newEntry:true});
  if (process.env.RESTART_FIRST_TURN) {
   const notices=await until(async()=>{const n=await call(q,'Notifications');return n.find(x=>x.kind==='restart-left') && n;},'restart recovery notification');
   assert.equal(notices.filter(x=>x.kind==='restart-left').length,1);
   assert.equal(notices.find(x=>x.kind==='restart-left').chats[0].id,id);
   await q.evaluate(()=>dinahWorkbench.open('needs-you'));
   const notice=notices.find(x=>x.kind==='restart-left');
   const button=q.locator('[data-notice-id="'+notice.id+'"]').getByRole('button',{name:'Open '+notice.chats[0].title,exact:true});
   await button.click();
   await q.getByText('The first turn in this session could not finish.',{exact:true}).first().waitFor();
   await q.getByRole('button',{name:'Continue from Saved History',exact:true}).first().waitFor();
   receipts.push({recoveryButtonOpenedChat:true});
   receipts.push({notifications:notices});
   console.log(JSON.stringify(receipts,null,2));return;
  }
  await until(()=>read(path.join(out,'prompts.jsonl')).includes(expected),'resume prompt');
  const lines=read(path.join(out,'prompts.jsonl')).trim().split('\n').map(JSON.parse);
  assert.equal(lines.filter(x=>JSON.stringify(x).includes(expected)).length,1);
  if (process.env.RESTART_SAME_PROGRAM) assert.ok(!lines.some(x=>JSON.stringify(x).includes('with its update')));
  receipts.push({resumePrompts:lines});
  // Allow a second delivery round to expose immediate duplicate wakeups.
  await q.waitForTimeout(1500);
  assert.equal(read(path.join(out,'prompts.jsonl')).split('\n').filter(x=>x.includes(expected)).length,1);
  console.log(JSON.stringify(receipts,null,2));
 } finally {
  fs.writeFileSync(path.join(out,'receipt.json'),JSON.stringify(receipts,null,2));
  if(browser)await browser.close();
  // All descendants inherit this isolated executable path. Never target a product by name.
  const script=`Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq '${exe.replaceAll("'","''")}' -or $_.ExecutablePath -eq '${(exe+'.old').replaceAll("'","''")}' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`;
  await new Promise(r=>spawn('powershell.exe',['-NoProfile','-NonInteractive','-Command',script],{windowsHide:true,stdio:'ignore'}).on('exit',r));
 }
})().catch(e=>{console.error(e);process.exitCode=1;});
