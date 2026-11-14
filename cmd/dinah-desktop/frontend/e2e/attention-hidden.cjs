// Real hidden desktop attention cards and Decisions across process restart.
// Usage: node attention-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
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

const cards=()=>page.locator('#questions .attention-kept');
const count=async n=>page.waitForFunction(n=>document.querySelector('#status-decisions').dataset.statusCount===String(n),n);
const notices=()=>page.evaluate(()=>dinahHost.notices.list(e=>e.data&&e.data.attentionChat));
async function openNotifications(){await page.evaluate(()=>dinahWorkbench.open('needs-you'));}
async function answerAll(){for(const b of await page.getByRole('button',{name:'As Recommended',exact:true}).all())await b.click();await page.waitForFunction(()=>!document.querySelector('.decisions-detail .btn-primary').disabled);await page.getByRole('button',{name:'Send Answers',exact:true}).click();}
(async()=>{try{
 browser=await chromium.launch({headless:true});receipts.push({initialPID:await start()});
 await call('SetNotifyWhenNeeded',false);await call('SetAgentExecutable','devin',path.resolve(agent));await call('SetWorkspace',out);
 const create=async title=>{const s=await call('NewSessionWithProviderAt',out,'devin');await call('RenameSession',s.sessionId,title);return s.sessionId;};
 const sender=await create('Attention Sender'),asker=await create('Decision Asker');
 const group=await call('BusStartGroup','Fixture Review',[{id:sender,cwd:out,title:'Attention Sender'}]);receipts.push({sender,asker,group});
 await page.evaluate(()=>dinahBus.openGroup('Fixture Review'));await openNotifications();await count(0);
 assert.equal(await page.locator('.win[data-win-kind="bus-room"]').count(),1);
 assert.ok(!await page.locator('.win[data-win-kind="bus-room"]').evaluate(e=>e.classList.contains('active')));
 await call('Send',sender,'fixture mention',null,null);
 await until(async()=>await cards().count()===1,'first card');await count(1);
 let kept=await notices();assert.equal(kept[0].data.requestCount,1);assert.ok((await cards().innerText()).includes('Fixture plain decision'));
 const firstId=await cards().getAttribute('data-notice-id');
 // Read the real group while it remains covered, then refresh the actual group controller.
 await call('BusGroupMessages',group.name);await page.evaluate(()=>dinahBus.load());
 assert.equal(await cards().count(),1);assert.equal(await cards().getAttribute('data-notice-id'),firstId);
 await until(async()=>(await call('UpdateState')).working===0,'sender idle');
 await call('Send',sender,'fixture mention second',null,null);
 await page.waitForFunction(()=>dinahHost.notices.list(e=>e.data&&e.data.requestCount===2).length===1);await count(2);
 assert.equal(await cards().count(),1);const text=await cards().innerText();assert.ok(text.includes('Second fixture request has a newer first line.'));assert.ok(!text.includes('This second line must not appear'));
 receipts.push({coveredGroupCards:await notices(),text,stored:await call('Notifications')});
 await call('Send',asker,'fixture ask',null,null);
 await count(3);await until(async()=>await cards().count()===2,'request card');
 const request=await until(async()=>(await call('Decisions',0)).items.find(x=>x.kind==='request'),'request');
 const before=await notices();receipts.push({beforeRestart:before,request,count:3});
 await page.evaluate(()=>dinahUIState.flush());await stop();receipts.push({restartedPID:await start()});
 await openNotifications();await count(3);await until(async()=>await cards().count()===2,'restored cards');
 const restored=await notices();assert.equal(restored.find(x=>x.data.attentionChat===sender).data.requestCount,2);assert.equal(restored.find(x=>x.data.attentionChat===asker).data.requestCount,1);
 assert.equal((await call('Decisions',0)).items.find(x=>x.id===request.id).revision,1);receipts.push({restored,count:3});
 // A plain mention is a Decisions row in this build, so its primary action opens that row.
 const senderNotice=restored.find(x=>x.data.attentionChat===sender);
 await page.locator('#questions [data-notice-id="'+senderNotice.id+'"]').getByRole('button',{name:'Open Decision',exact:true}).click();
 await page.locator('.decisions-row[data-request-id="mention:'+senderNotice.data.message+'"] > a[aria-current="true"]').waitFor();
 await until(async()=>!(await notices()).some(x=>x.data.attentionChat===sender),'group card acknowledged');
 for(const m of (await call('Decisions',0)).items.filter(x=>x.kind==='mention')){
  await page.locator('.decisions-row[data-request-id="'+m.id+'"] > a').click();await page.getByRole('button',{name:'Dismiss Message',exact:true}).click();await page.locator('#ask[open]').waitFor();await page.click('#ask-yes');await until(async()=>!(await call('Decisions',0)).items.some(x=>x.id===m.id),'mention dismissed');
 }
 await count(1);await openNotifications();
 const askNotice=(await notices()).find(x=>x.data.attentionChat===asker);
 await page.locator('#questions [data-notice-id="'+askNotice.id+'"]').getByRole('button',{name:'Open Decision',exact:true}).click();
 await page.locator('.decisions-row[data-request-id="'+request.id+'"] > a[aria-current="true"]').waitFor();
 assert.ok((await page.locator('.decisions-detail').innerText()).includes('Use the reviewed release.'));
 await until(async()=>!(await notices()).some(x=>x.data.attentionChat===asker),'request attention acknowledged');await count(1);
 await answerAll();await count(0);await until(async()=>!(await call('Decisions',0)).items.some(x=>x.id===request.id),'answered request');
 assert.equal((await notices()).length,0);assert.equal((await call('Notifications')).filter(x=>x.kind==='mention'&&!x.dismissed).length,0);
  // Enter through the status bar for a fresh request, leaving its card unacknowledged until the answer.
 const unanswered=await create('Unopened Request');await call('Send',unanswered,'fixture ask',null,null);
 await count(1);await until(async()=>(await notices()).some(x=>x.data.attentionChat===unanswered),'fresh request card');
 const fresh=(await call('Decisions',0)).items.find(x=>x.kind==='request');
 await page.locator('#status-decisions').click();await page.locator('.decisions-row[data-request-id="'+fresh.id+'"] > a').click();
 assert.ok((await notices()).some(x=>x.data.attentionChat===unanswered));receipts.push({cardBeforeAnswer:await notices()});
 await answerAll();await count(0);await until(async()=>!(await notices()).some(x=>x.data.attentionChat===unanswered),'answer clears unacknowledged card');
 receipts.push({answered:request.id,finalCount:0,finalCards:await notices(),stored:await call('Notifications'),passed:true,pageErrors:errors});assert.deepEqual(errors,[]);
} catch(e){receipts.push({failure:String(e),pageErrors:errors});if(page)fs.writeFileSync(path.join(out,'failure-page.txt'),await page.locator('body').innerText().catch(()=>''));throw e;}finally{fs.writeFileSync(path.join(out,'receipt.json'),JSON.stringify(receipts,null,2));await stop();if(browser)await browser.close();}console.log('PASS real attention, covered refresh, counted cards, Decisions entries and restart');})().catch(e=>{console.error(e);process.exitCode=1;});

