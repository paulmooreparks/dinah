// Real hidden desktop and bus MCP request, with headless page reload and program restart.
// Usage: node answer-origin-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
const fs = require('node:fs');
const path = require('node:path');
const {spawn} = require('node:child_process');
const {chromium} = require('playwright-core');
const assert = require('node:assert/strict');
const [desktop, agent, out] = process.argv.slice(2);
if (!desktop || !agent || !out || fs.existsSync(out)) throw Error('Supply binaries and a fresh output folder');
fs.mkdirSync(out,{recursive:true});
const home=path.join(out,'home'), exe=path.join(out,'dinah-desktop.exe'), entry=path.join(out,'entry.txt');
fs.copyFileSync(desktop,exe);
const log=fs.openSync(path.join(out,'desktop.log'),'w');
const child=spawn(exe,[],{env:{...process.env,DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entry,DECISIONS_FIXTURE:out},windowsHide:true,stdio:['ignore',log,log]});
const receipts=[], errors=[];
let browser,page;
const read=name=>{try{return fs.readFileSync(path.join(out,name),'utf8');}catch{return '';}};
const records=()=>read('agent.jsonl').trim().split('\n').filter(Boolean).map(JSON.parse);
async function until(fn,label){const end=Date.now()+60000;while(Date.now()<end){const v=await fn();if(v)return v;await new Promise(r=>setTimeout(r,100));}throw Error('Timed out: '+label);}
const call=(name,...args)=>page.evaluate(({name,args})=>go.main.App[name](...args),{name,args});
async function ready(url){await page.goto(url);await page.waitForFunction(()=>window.dinahUIState&&window.dinahWorkbench);await page.evaluate(()=>dinahUIState.ready);}
(async()=>{
 try {
  const url=await until(()=>read('entry.txt').trim(),'entry');
  browser=await chromium.launch({headless:true,executablePath:process.env.ANSWER_ORIGIN_BROWSER || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});page=await browser.newPage({viewport:{width:1440,height:1000}});page.on('pageerror',e=>errors.push(String(e)));
  await ready(url);
  await call('SetAgentExecutable','devin',path.resolve(agent));await call('SetWorkspace',out);
  const info=await call('NewSessionWithProviderAt',out,'devin'), asker=info.sessionId;
  await call('RenameSession',asker,'Answer Origin Asker');
  await page.evaluate(()=>dinahWorkbench.open('decisions'));
  await call('Send',asker,'fixture ask',null,null);
  const request=await until(async()=>{const p=await call('Decisions',0);return p.items.find(x=>x.kind==='request');},'MCP request');
  const mcp=await until(()=>records().find(x=>x.tool==='ask_person'),'ask receipt');assert.equal(mcp.isError,false);
  await page.locator('.decisions-row[data-request-id="'+request.id+'"] > a').click();
  await page.locator('[data-question="1"]').getByRole('button',{name:'As Recommended',exact:true}).click();
  await page.getByLabel('Answer 2',{exact:true}).fill('Proceed next Monday after review.');
  await page.getByRole('button',{name:'Send Answers',exact:true}).click();
  await until(async()=>!(await call('Decisions',0)).items.some(x=>x.id===request.id),'closed request');
  await until(()=>records().find(x=>x.session===asker&&x.prompt&&x.prompt.includes('Use the reviewed release.')&&x.prompt.includes('Proceed next Monday after review.')),'agent answer');
  await until(async()=>(await call('UpdateState')).working===0,'idle');
  const openChat=async()=>page.evaluate(({out,asker})=>dinahWorkbench.open({kind:'chat-session',workspace:out,path:asker}),{out,asker});
  let chatKey=await openChat();
  async function verify(stage){
   const card=page.locator('[data-win="'+chatKey+'"] .decision-answer-card');await card.waitFor();assert.equal(await card.count(),1);
   assert.deepEqual(await card.locator('h4').allTextContents(),['Question 1','Answer','Question 2','Answer']);
   const text=await card.innerText();for(const s of ['Which release should we use?','Use the reviewed release.','When should we proceed?','Proceed next Monday after review.'])assert.ok(text.includes(s),s);
   assert.equal(await card.locator('.decision-answer-mode').count(),1);
   const saved=await call('SavedConversation',asker);
   const answers=saved.messages.filter(x=>x.delivered&&x.delivered.answerOrigin);
   assert.equal(answers.length,1);const origin=answers[0].delivered.answerOrigin;
   assert.equal(origin.requestId,request.id);assert.deepEqual(origin.questions.map(q=>q.asRecommended),[true,false]);
   receipts.push({stage,text,origin});
   await page.evaluate(key=>pudlWindows.snap(key,'maximized'),chatKey);
   await card.scrollIntoViewIfNeeded(); await card.locator('.decision-answer-body').evaluate(el=>{el.scrollTop=0;});
   await page.screenshot({path:path.join(out,stage+'-question-1.png')});
   await card.locator('.decision-answer-body').evaluate(el=>{el.scrollTop=el.scrollHeight;});
   receipts.push({stage,scroll:await card.locator('.decision-answer-body').evaluate(el=>({top:el.scrollTop,height:el.clientHeight,total:el.scrollHeight}))});
   await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
   const visible=await card.locator('.decision-answer-question').last().evaluate(el=>{const q=el.getBoundingClientRect(),b=el.parentElement.getBoundingClientRect();return {question:{top:q.top,bottom:q.bottom},body:{top:b.top,bottom:b.bottom},scrollTop:el.parentElement.scrollTop};});
   receipts.push({stage,visible});assert.ok(visible.question.top>=visible.body.top-1&&visible.question.bottom<=visible.body.bottom+1,'second answer visible inside card');
   await page.screenshot({path:path.join(out,stage+'-question-2.png')});return card;
  }
  await verify('delivered');
  await ready(page.url());chatKey=await openChat();await verify('page-reloaded');
  const oldEntry=read('entry.txt').trim();
  call('PauseAndRestart').catch(()=>{});
  await until(()=>child.exitCode!==null,'old process exit');
  const newEntry=await until(()=>{const v=read('entry.txt').trim();return v&&v!==oldEntry&&v;},'new program entry');
  await ready(newEntry);chatKey=await openChat();const card=await verify('program-restarted');
  await card.getByRole('link',{name:'Open Request in Decisions',exact:true}).click();
  await page.waitForFunction(()=>document.querySelector('.decisions-record')?.textContent.includes('This request is closed.'));
  const record=page.locator('.decisions-record');assert.equal(await record.locator('input,textarea').count(),0);assert.equal(await page.getByRole('button',{name:'Send Answers',exact:true}).count(),0);
  const text=await record.innerText();assert.ok(text.includes('Use the reviewed release.'));assert.ok(text.includes('Proceed next Monday after review.'));
  const lookup=await call('DecisionByID',request.id);assert.equal(lookup.closed,true);assert.equal(lookup.readOnly,true);
  assert.deepEqual(errors,[]);receipts.push({closedRecord:text,lookup,oldPID:child.pid,restarted:true,pageErrors:errors,passed:true});await page.screenshot({path:path.join(out,'closed-request.png')});
 }catch(error){receipts.push({failure:String(error),pageErrors:errors});if(page)fs.writeFileSync(path.join(out,'failure-page.txt'),await page.locator('body').innerText().catch(()=>''));throw error;}
 finally{
  fs.writeFileSync(path.join(out,'receipt.json'),JSON.stringify(receipts,null,2));if(browser)await browser.close();
  const script=`Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq '${exe.replaceAll("'","''")}' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`;
  await new Promise(r=>spawn('powershell.exe',['-NoProfile','-NonInteractive','-Command',script],{windowsHide:true,stdio:'ignore'}).on('exit',r));
 }
 console.log('PASS typed answer card, page reload, hidden program restart and closed read-only link');
})().catch(e=>{console.error(e);process.exitCode=1;});
