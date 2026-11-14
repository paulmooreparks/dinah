// Real hidden desktop with five closed chats, a two-chat limit and real preview expiry.
// Usage: node broadcast-all-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {spawn}=require('node:child_process'),{chromium}=require('playwright');
const [desktop,agent,out]=process.argv.slice(2);
if(!desktop||!agent||!out||fs.existsSync(out))throw Error('Supply binaries and a fresh output folder');
fs.mkdirSync(out,{recursive:true});
const exe=path.join(out,'dinah-desktop.exe'),entry=path.join(out,'entry.txt'),home=path.join(out,'home');
fs.copyFileSync(desktop,exe);
const receipts=[],errors=[];let browser,page;
const read=f=>{try{return fs.readFileSync(path.join(out,f),'utf8')}catch{return ''}};
const records=()=>read('agent.jsonl').trim().split('\n').filter(Boolean).map(JSON.parse);
const broadcast='Fixture held broadcast to all.';
const prompts=()=>records().filter(x=>x.prompt&&x.prompt.includes(broadcast));
async function until(fn,label,ms=60000){const end=Date.now()+ms;while(Date.now()<end){const v=await fn();if(v)return v;await new Promise(r=>setTimeout(r,100));}throw Error('Timed out: '+label);}
const call=(name,...args)=>page.evaluate(({name,args})=>go.main.App[name](...args),{name,args});
async function stop(){if(page)await page.close();const script=`Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq '${exe.replaceAll("'","''")}' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`;await new Promise(r=>spawn('powershell.exe',['-NoProfile','-NonInteractive','-Command',script],{windowsHide:true,stdio:'ignore'}).on('exit',r));}
async function start(){fs.rmSync(entry,{force:true});const fd=fs.openSync(path.join(out,'desktop.log'),'a');const child=spawn(exe,[],{env:{...process.env,DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entry,DECISIONS_FIXTURE:out},windowsHide:true,stdio:['ignore',fd,fd]});fs.closeSync(fd);const url=await until(()=>read('entry.txt').trim(),'entry');page=await browser.newPage({viewport:{width:1440,height:1000}});page.on('pageerror',e=>errors.push(String(e)));await page.goto(url);await page.waitForFunction(()=>window.dinahUIState&&window.dinahWorkbench);await page.evaluate(()=>dinahUIState.ready);return child.pid;}
async function preview(text){await page.evaluate(()=>dinahCommands.run('broadcast-all'));await page.getByLabel('Broadcast Message',{exact:true}).fill(text);await page.getByRole('button',{name:'Preview Recipients',exact:true}).click();await page.locator('.decisions-preview li').first().waitFor();return page.evaluate(()=>dinahDecisionDrafts.read('broadcast-all').preview);}
async function confirmDialog(){await page.getByRole('button',{name:'Confirm Broadcast…',exact:true}).click();await page.locator('#ask[open]').waitFor();}
(async()=>{try{
 browser=await chromium.launch({headless:true});receipts.push({initialPID:await start()});
 await call('SetAgentExecutable','devin',path.resolve(agent));await call('SetWorkspace',out);await call('SetBackgroundChatLimit',2);
 const ids=[];
 for(let i=0;i<5;i++){const s=await call('NewSessionWithProviderAt',out,'devin');ids.push(s.sessionId);await call('RenameSession',s.sessionId,'Broadcast Fixture '+i);await call('Send',s.sessionId,'fixture seed',null,null);await until(()=>records().some(x=>x.session===s.sessionId&&x.prompt==='fixture seed'),'seed');await until(async()=>(await call('UpdateState')).working===0,'seed turn end');}
 receipts.push({ids});await stop();receipts.push({restartedPID:await start()});
 await call('ChatWindows',[]);assert.equal(await call('BackgroundChatLimit'),2);
 const closed=await call('ChatStates');assert.equal(closed.chats.filter(x=>ids.includes(x.sessionId)&&x.life==='open').length,0);assert.equal(records().filter(x=>x.loaded).length,0);receipts.push({closed});
 await page.evaluate(()=>{window.fixtureEvents=[];for(const name of ['chat:connect','agent:session','agent:chatstate'])dinahHost.on(name,data=>fixtureEvents.push({name,data,at:Date.now()}));});
 const frozen=await preview(broadcast);assert.equal(frozen.count,5);assert.deepEqual(frozen.targets.map(x=>x.title).sort(),Array.from({length:5},(_,i)=>'Broadcast Fixture '+i));assert.equal(await page.locator('.decisions-preview li').count(),5);assert.equal(prompts().length,0);receipts.push({frozen});
 await confirmDialog();await page.click('#ask-yes');await page.waitForFunction(()=>document.querySelector('.decisions-receipt').textContent.includes('Broadcast queued'));
 const committed=await page.evaluate(()=>dinahDecisionDrafts.read('broadcast-all').operation.receipt);assert.equal(committed.messageIds.length,5);assert.equal(new Set(committed.messageIds).size,5);receipts.push({committed});
 // Replay the same confirmation identity through the real bridge. It must not enqueue again.
 const input=await page.evaluate(()=>dinahDecisionDrafts.read('broadcast-all').operation.input);assert.deepEqual(await call('ConfirmDecisionBroadcast',input),committed);
 await until(()=>prompts().length===2,'first two deliveries');
 // Hold both admitted turns across multiple real watcher rounds.
 const heldUntil=Date.now()+5000;await until(()=>Date.now()>=heldUntil,'held observation',10000);assert.equal(prompts().length,2);assert.equal((await call('UpdateState')).working,2);
 receipts.push({firstWave:prompts(),state:await call('ChatStates')});console.log('First two chats held; remaining three still queued');
 for(const count of [2,4,5]){
  await until(()=>prompts().length===count,'delivery wave '+count,90000);
  const wave=prompts();assert.equal(new Set(wave.map(x=>x.session)).size,count);
  assert.ok((await call('UpdateState')).working<=2);
  for(const p of wave)fs.writeFileSync(path.join(out,'release-'+p.session),'finish');
  receipts.push({wave:count,at:new Date().toISOString(),state:await call('ChatStates')});
 }
 await until(async()=>(await call('UpdateState')).working===0,'all finished');
 assert.deepEqual(prompts().map(x=>x.session).sort(),ids.slice().sort());
 const events=await page.evaluate(()=>fixtureEvents);const sessions=new Map();let peak=0;
 for(const e of events){if(e.name==='agent:session'){sessions.set(e.data.sessionId,e.data);const used=[...sessions.values()].filter(s=>s.inBackground&&!s.parked).length;peak=Math.max(peak,used);assert.ok(used<=2,'event stream exceeded two slots');}}
 assert.equal(peak,2);receipts.push({deliveries:prompts(),events,peak});console.log('All five received exactly once; background slot peak two');
 const expiredText='Fixture expired broadcast must never arrive.';
 const expiring=await preview(expiredText);await confirmDialog();receipts.push({expiring});
 console.log('Waiting for the real five-minute preview expiry with confirmation dialog open');
 await until(()=>Date.now()>Date.parse(expiring.expiresAt)+100,'real preview expiry',310000);
 await page.click('#ask-yes');await page.waitForFunction(()=>document.querySelector('.decisions-receipt').textContent.includes('preview expired'));
 const rejected=await page.evaluate(()=>dinahDecisionDrafts.read('broadcast-all').operation.receipt);assert.equal(rejected.state,'rejected');assert.equal(rejected.reason.code,'preview_expired');assert.deepEqual(rejected.messageIds,[]);
 assert.ok(!records().some(x=>x.prompt&&x.prompt.includes(expiredText)));assert.equal(prompts().length,5);assert.deepEqual(errors,[]);receipts.push({rejected,visibleReason:await page.locator('.decisions-receipt').innerText(),passed:true,pageErrors:errors});
} catch(e){receipts.push({failure:String(e),pageErrors:errors});if(page)fs.writeFileSync(path.join(out,'failure-page.txt'),await page.locator('body').innerText().catch(()=>''));throw e;}finally{fs.writeFileSync(path.join(out,'receipt.json'),JSON.stringify(receipts,null,2));await stop();if(browser)await browser.close();}console.log('PASS Broadcast to All, bounded admission, replay and real expiry');})().catch(e=>{console.error(e);process.exitCode=1;});


