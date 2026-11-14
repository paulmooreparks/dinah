// Real hidden desktop, real bus MCP helper, and an isolated ACP stand-in.
// Usage: node decisions-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
const fs = require('node:fs');
const path = require('node:path');
const {spawn} = require('node:child_process');
const {chromium} = require('playwright');
const assert = require('node:assert/strict');
const [desktop, agent, out] = process.argv.slice(2);
if (!desktop || !agent || !out || fs.existsSync(out)) throw Error('Supply binaries and a fresh output folder');
fs.mkdirSync(out, {recursive:true});
const home = path.join(out,'home'), exe = path.join(out,'dinah-desktop.exe'), entry = path.join(out,'entry.txt');
fs.copyFileSync(desktop,exe);
const log = fs.openSync(path.join(out,'desktop.log'),'w');
const child = spawn(exe,[],{env:{...process.env,DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entry,DECISIONS_FIXTURE:out},windowsHide:true,stdio:['ignore',log,log]});
const receipts = [], errors = [];
let browser, page;
const read = name => {try{return fs.readFileSync(path.join(out,name),'utf8');}catch{return '';}};
const records = () => read('agent.jsonl').trim().split('\n').filter(Boolean).map(JSON.parse);
async function until(fn,label) {
 const end=Date.now()+60000;
 while(Date.now()<end){const v=await fn();if(v)return v;await new Promise(r=>setTimeout(r,100));}
 throw Error('Timed out: '+label);
}
const call = (name,...args) => page.evaluate(({name,args})=>go.main.App[name](...args),{name,args});
const decisions = () => call('Decisions',0);
async function choose(id){await page.locator('.decisions-row[data-request-id="'+id+'"] > a').click();await page.locator('.decisions-detail h2').waitFor();}
async function sendAnswers(){await page.waitForFunction(()=>!document.querySelector('.decisions-detail .btn-primary').disabled);await page.getByRole('button',{name:'Send Answers',exact:true}).click();}
(async()=>{
 try {
  const url=await until(()=>read('entry.txt').trim(),'hidden entry');
  browser=await chromium.launch({headless:true}); page=await browser.newPage({viewport:{width:1440,height:1000}});
  page.on('pageerror',e=>errors.push(String(e)));
  await page.goto(url); await page.waitForFunction(()=>window.dinahUIState && window.dinahWorkbench); await page.evaluate(()=>dinahUIState.ready);
  await call('SetAgentExecutable','devin',path.resolve(agent)); await call('SetWorkspace',out);
  const create = async title => {const s=await call('NewSessionWithProviderAt',out,'devin');await call('RenameSession',s.sessionId,title);return s.sessionId;};
  const asker=await create('Fixture Asker'), active=await create('Fixture Working'), idle=await create('Fixture Idle');
  receipts.push({sessions:{asker,active,idle},pid:child.pid});
  await page.evaluate(()=>dinahWorkbench.open('decisions'));
  await call('Send',asker,'fixture ask',null,null);
  const request=await until(async()=>{const p=await decisions();return p.items.find(x=>x.kind==='request');},'request from MCP');
  const mcp=await until(()=>records().find(x=>x.tool==='ask_person'),'MCP receipt'); assert.ok(!mcp.isError,JSON.stringify(mcp));
  await choose(request.id);
  const detail=page.locator('.decisions-detail');
  assert.ok((await detail.innerText()).includes('Use the reviewed release.'));
  assert.ok((await detail.innerText()).includes('Proceed tomorrow.'));
  receipts.push({request,mcp});
  await page.locator('[data-question="1"]').getByRole('button',{name:'As Recommended',exact:true}).click();
  assert.equal((await decisions()).items[0].revision,1);
  await sendAnswers();
  const partial=await until(async()=>{const p=await decisions();return p.items.find(x=>x.id===request.id && x.revision===2);},'partial revision');
  assert.equal(partial.questions[0].answer,'Use the reviewed release.');assert.equal(partial.questions[1].answered,false);
  await until(()=>records().find(x=>x.session===asker && x.prompt && x.prompt.includes('Use the reviewed release.') && x.prompt.includes('Which release should we use?')),'first readable reply');
  receipts.push({partial});
  await page.getByLabel('Answer 2',{exact:true}).fill('Proceed next Monday after review.'); await sendAnswers();
  await until(async()=>!(await decisions()).items.some(x=>x.id===request.id),'closed request');
  await until(()=>records().find(x=>x.session===asker && x.prompt && x.prompt.includes('Proceed next Monday after review.') && x.prompt.includes('When should we proceed?')),'second readable reply');
  await until(async()=>(await call('UpdateState')).working===0,'asker idle');
  await until(async()=>await page.locator('.decisions-row[data-request-id="'+request.id+'"]').count()===0,'request removed from view');
  receipts.push({answers:records().filter(x=>x.session===asker && x.prompt && x.prompt!=='fixture ask')});
  await call('BusStartGroup','Fixture Review',[{id:asker,cwd:out,title:'Fixture Asker'}]);
  await call('Send',asker,'fixture mention',null,null);
  const mention=await until(async()=>{const p=await decisions();return p.items.find(x=>x.kind==='mention' && x.text.includes('Fixture plain decision'));},'plain mention');
  assert.equal(mention.questions.length,0);await choose(mention.id);
  await page.getByRole('button',{name:'Dismiss Message',exact:true}).click();await page.locator('#ask[open]').waitFor();await page.click('#ask-yes');
  await until(async()=>!(await decisions()).items.some(x=>x.id===mention.id),'mention dismissal');
  receipts.push({dismissedMention:mention});
  await until(async()=>(await call('UpdateState')).working===0,'all idle before broadcast');
  await call('Send',active,'fixture hold',null,null);
  await until(()=>records().some(x=>x.session===active && x.prompt==='fixture hold'),'working prompt');
  assert.equal((await call('UpdateState')).working,1);
  await page.evaluate(()=>dinahCommands.run('broadcast-active'));
  const broadcast='Fixture broadcast only to working chats.';
  await page.getByLabel('Broadcast Message',{exact:true}).fill(broadcast);
  await page.getByRole('button',{name:'Preview Recipients',exact:true}).click();
  await page.locator('.decisions-preview li').waitFor();
  assert.equal(await page.locator('.decisions-preview li').count(),1);
  const frozen=await page.evaluate(()=>dinahDecisionDrafts.read('broadcast-active').preview);
  assert.equal(frozen.count,1);assert.equal(frozen.targets[0].title,'Fixture Working');
  const preview=await page.locator('.decisions-preview').innerText();assert.ok(preview.includes('Fixture Working'));assert.ok(!preview.includes('Fixture Idle'));
  assert.ok(!records().some(x=>x.prompt && x.prompt.includes(broadcast)));
  receipts.push({preview,frozen});
  await page.getByRole('button',{name:'Confirm Broadcast…',exact:true}).click();await page.locator('#ask[open]').waitFor();
  assert.ok((await page.locator('#ask-body').innerText()).includes('Fixture Working'));
  await page.click('#ask-yes');
  await page.waitForFunction(()=>document.querySelector('.decisions-receipt').textContent.includes('Broadcast queued'));
  const committed=await page.evaluate(()=>dinahDecisionDrafts.read('broadcast-active').operation.receipt);
  assert.equal(committed.state,'committed');assert.equal(committed.messageIds.length,1);
  // This stand-in has no steering hook. The bus waits for its held turn.
  fs.writeFileSync(path.join(out,'finish'),'end held turn');
  await until(()=>records().some(x=>x.session===active && x.prompt && x.prompt.includes(broadcast)),'broadcast delivery');
  await until(async()=>(await call('UpdateState')).working===0,'broadcast turn finished');
  assert.ok(!records().some(x=>x.session===idle && x.prompt));
  assert.equal(records().filter(x=>x.prompt && x.prompt.includes(broadcast)).length,1);
  receipts.push({broadcast:records().filter(x=>x.prompt && x.prompt.includes(broadcast)),idlePrompts:0,committed});
  assert.deepEqual(errors,[]);receipts.push({passed:true,pageErrors:errors});
 } catch(error) {
  receipts.push({failure:String(error),pageErrors:errors});
  if(page)fs.writeFileSync(path.join(out,'failure-page.txt'),await page.locator('body').innerText().catch(()=>''));
  throw error;
 } finally {
  fs.writeFileSync(path.join(out,'receipt.json'),JSON.stringify(receipts,null,2));
  if(browser)await browser.close();
  const script=`Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq '${exe.replaceAll("'","''")}' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`;
  await new Promise(r=>spawn('powershell.exe',['-NoProfile','-NonInteractive','-Command',script],{windowsHide:true,stdio:'ignore'}).on('exit',r));
 }
 console.log('PASS real MCP questions, partial answers, mention dismissal and active broadcast');
})().catch(e=>{console.error(e);process.exitCode=1;});
