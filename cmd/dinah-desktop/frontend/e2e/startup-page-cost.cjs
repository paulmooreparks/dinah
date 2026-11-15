// Counted acceptance, never a default suite. Machine-specific baseline traces
// stay outside the public repository.
// Run through scripts/with-test-lock.ps1.
// node startup-page-cost.cjs <tagged-test.exe> <peer.exe> <seed.exe> <fresh-output>
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const {spawn}=require('node:child_process');const {chromium}=require('playwright-core');
const [host,peer,seed,root]=process.argv.slice(2);if(!root||fs.existsSync(root))throw Error('Supply the three fixture programs and a fresh output folder.');fs.mkdirSync(root,{recursive:true});
const digest=p=>crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const evidence={hostSha256:digest(host),peerSha256:digest(peer),cases:[]};
async function until(fn){const end=Date.now()+30000;while(Date.now()<end){const value=fn();if(value)return value;await new Promise(r=>setTimeout(r,50));}throw Error('Fixture did not become ready.');}
async function run(name,provider,mode,query,judge){
 const out=path.join(root,name);fs.mkdirSync(out);for(const dir of ['profile','appdata','local','tmp','peers'])fs.mkdirSync(path.join(out,dir));
 const home=path.join(out,'home'),entry=path.join(out,'entry.txt'),peerLog=path.join(out,'peer.jsonl');
 const env={SystemRoot:process.env.SystemRoot,WINDIR:process.env.WINDIR,COMSPEC:process.env.COMSPEC,PATH:path.join(out,'peers')+';'+path.join(process.env.SystemRoot,'System32'),USERPROFILE:path.join(out,'profile'),HOME:path.join(out,'profile'),APPDATA:path.join(out,'appdata'),LOCALAPPDATA:path.join(out,'local'),TEMP:path.join(out,'tmp'),TMP:path.join(out,'tmp'),CODEX_HOME:path.join(out,'profile','.codex'),CLAUDE_CONFIG_DIR:path.join(out,'profile','.claude'),DINAH_HOME:home,DINAH_STARTUP_AUDIT_ENTRY:entry,DINAH_STARTUP_PEER_DIR:path.join(out,'peers'),DINAH_STARTUP_PEER_LOG:peerLog,DINAH_STARTUP_PEER_REFUSE:mode==='refusal'?'1':'0'};
 const seedProcess=spawn(seed,[home],{env,windowsHide:true,stdio:'ignore'});assert.equal(await new Promise(r=>seedProcess.once('exit',r)),0);
 const settingsPath=path.join(home,'settings.json'),settings=JSON.parse(fs.readFileSync(settingsPath));settings.defaultProvider=provider;settings.providerExecutables={};
 for(const id of ['codex','devin','claude']){const dest=path.join(out,'peers',id+'.exe');fs.copyFileSync(peer,dest);settings.providerExecutables[id]=dest;}
 fs.copyFileSync(peer,path.join(out,'peers','dinah.exe'));
 const controlProvider=mode==='page'&&!judge?provider:'devin';
 const controlID=controlProvider==='codex'?'codex:fixture-open':controlProvider==='claude'?'claude:fixture-open':controlProvider==='ollama'?'ollama:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa':'fixture-open';
 env.DINAH_STARTUP_CONTROL_ID=controlID;env.DINAH_STARTUP_CONTROL_PROVIDER=controlProvider;
 env.DINAH_STARTUP_CONTROL_WORKSPACE=settings.workspace;
 settings.sessionTitles[controlID]='Positive control';settings.sessionProviders[controlID]=controlProvider;settings.nativeSessions[controlID]={provider:controlProvider};
 fs.writeFileSync(settingsPath,JSON.stringify(settings));
 if(judge)fs.writeFileSync(path.join(home,'judgements.json'),JSON.stringify({enabled:false,provider:judge,model:'',retentionDays:90,warnStreak:3}));
 const log=fs.openSync(path.join(out,'host.log'),'w'),proc=spawn(host,['-test.run=^TestStartupPageCostHost$','-test.timeout=170s','-test.v'],{env,windowsHide:true,stdio:['pipe',log,log]});let browser;
 const result={name,provider,mode,judge:judge||'',errors:[],calls:[]};evidence.cases.push(result);
 try{
  const url=await until(()=>fs.existsSync(entry)&&fs.readFileSync(entry,'utf8').trim());
  browser=await chromium.launch({headless:true,channel:'msedge'});const p=await browser.newPage({viewport:{width:1440,height:1000}});
  p.on('pageerror',e=>result.errors.push(e.message));
  await p.route('**/*',route=>{const u=new URL(route.request().url());if(!['127.0.0.1','localhost'].includes(u.hostname))return route.abort();if(mode==='query'&&u.pathname==='/')return route.fulfill({contentType:'text/html',body:'<!doctype html><html><head><script src="/bridge/app.js"></script></head><body></body></html>'});return route.continue();});
  await p.addInitScript(()=>{
   window.__auditCalls=[];window.__auditPending=0;window.__auditPhase=sessionStorage.getItem('startup-audit-phase')||'startup';const original=window.fetch;
   window.fetch=function(input,options){let record;try{const u=new URL(typeof input==='string'?input:input.url,location.href);if(u.pathname==='/bridge/call'){const q=JSON.parse(options.body);record={method:q.method,args:q.args,phase:__auditPhase,stack:new Error().stack.split('\n').slice(2,7),start:performance.now()};__auditCalls.push(record);__auditPending++;}}catch(_){}
    const request=original.apply(this,arguments);return record?request.then(r=>{record.end=performance.now();__auditPending--;r.clone().json().then(value=>{if(value.error)record.error=value.error;}).catch(()=>{});return r;},e=>{record.end=performance.now();record.failed=true;__auditPending--;throw e;}):request;
   };
  });
  await p.goto(url);await p.waitForFunction(()=>window.go&&go.main.App.StartupPageCounters);
  if(mode!=='query')await p.waitForFunction(()=>document.body&&!document.body.hasAttribute('aria-busy')&&window.pudlWindows&&__auditPending===0);
  await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
  const call=(method,...args)=>p.evaluate(({method,args})=>go.main.App[method](...args),{method,args});
  result.startup=await call('StartupPageCounters');result.initialWindows=await p.evaluate(()=>window.pudlWindows?pudlWindows.state().open:[]);
  assert.equal(result.startup['host/loaded-chats'],0,'startup has no loaded chat');
  const repeat=await call('StartupPageCounters');assert.deepEqual(repeat,result.startup,'diagnostic reads initialize nothing');
  result.startupPeer=fs.existsSync(peerLog)?fs.readFileSync(peerLog,'utf8').trim().split('\n').filter(Boolean).map(s=>JSON.parse(s)):[];
  if(mode!=='query'){
   assert.equal(await p.evaluate(()=>pudlWindows.state().open.some(key=>dinahWorkbench.item(key)?.kind==='chat')),false,'fresh startup opens no blank New Chat window');
   assert.deepEqual(Object.entries(result.startup).filter(([key,value])=>value!==0),[],'zero-open-chat page startup contacts no provider');
   assert.deepEqual(result.startupPeer,[],'page startup launches no program and sends no native request');
  }
  if(mode==='query'){
   result.queries=[];
   for(const [method,args]of query?[query]:[['State',[]],['ProviderModelPreferences',[provider]]]){
    await p.evaluate(method=>{__auditPhase=method;},method);const before=await call('StartupPageCounters');await call(method,...args);
    if(method==='ModelPrices')await until(()=>fs.existsSync(peerLog)&&fs.readFileSync(peerLog,'utf8').includes('"models","list"'));
    const after=await call('StartupPageCounters');result.queries.push({method,before,after});
   }
  }
  if(mode==='page'&&!judge){
   await p.evaluate(()=>{__auditPhase='explicit-open';});const before=await call('StartupPageCounters');
   result.openOutcome=await p.evaluate(id=>dinahChats.open(id).then(s=>s?{info:s.info,failed:s.failed,loading:s.loading}:null),controlID);
   await p.waitForFunction(()=>__auditPending===0);
   assert.equal(result.openOutcome?.info.open,true,JSON.stringify(result.openOutcome));
   await p.waitForFunction(id=>dinahChats.ref(id)?.life==='open',controlID);
   result.open=await p.evaluate(id=>dinahChats.ref(id),controlID);
   result.openStates=await call('ChatStates');
   const after=await call('StartupPageCounters');result.positive={before,after};assert.equal(result.open.life,'open');
   if(provider==='codex')assert.equal(fs.readFileSync(peerLog,'utf8').split('\n').filter(line=>line.includes('"kind":"request"')&&line.includes('"thread/resume"')).length,1);
   else assert.equal((after[provider+'/session/load']||0)-(before[provider+'/session/load']||0),1);
   await p.evaluate(()=>dinahUIState.flush());
   result.openCalls=await p.evaluate(()=>__auditCalls);
   await p.evaluate(()=>sessionStorage.setItem('startup-audit-phase','restored-open'));
   await p.reload();
   await p.waitForFunction(()=>document.body&&!document.body.hasAttribute('aria-busy')&&window.dinahChats&&__auditPending===0);
   await p.waitForFunction(id=>dinahChats.ref(id)&&dinahChats.ref(id).life==='open',controlID);
   result.restored=await p.evaluate(id=>dinahChats.ref(id),controlID);
   assert.equal(result.restored.id,controlID);assert.equal(result.restored.provider,result.open.provider);
   if(process.env.DINAH_STARTUP_AUDIT_TURNS==='1'){
    await p.evaluate(()=>{__auditPhase='invented-turn';});
    const composer=p.locator('.win[data-chat-current] textarea[data-view-id="prompt"]');
    await composer.fill('Complete this invented receiving turn.');await composer.press('Enter');
    await p.locator('.win[data-chat-current]').getByText('Invented answer.',{exact:true}).first().waitFor();
    await p.waitForFunction(id=>dinahChats.ref(id)?.exec?.state==='idle',controlID);
    result.completedInventedTurn=true;
   }
  }
  result.final=await call('StartupPageCounters');result.calls=(result.openCalls||[]).concat(await p.evaluate(()=>__auditCalls));
  result.boundaryTrace=await call('StartupPageTrace');
  assert.equal(result.calls.filter(c=>c.method==='ModelPrices'&&c.args[0]!==controlProvider).length,0,'prices are fetched only for the open provider');
  assert.equal(result.calls.filter(c=>c.phase==='startup'&&/OpenSession|NewSession/.test(c.method)).length,0,'startup has no requested chat');
  if(mode!=='query')await p.screenshot({path:path.join(out,'page.png')});
  result.pass=true;
 }finally{
  if(browser){try{const pages=browser.contexts().flatMap(context=>context.pages());if(pages[0])result.calls=(result.openCalls||[]).concat(await pages[0].evaluate(()=>__auditCalls));}catch(error){result.traceError=String(error);}}
  if(browser)await browser.close();const ended=proc.exitCode===null?new Promise(r=>proc.once('exit',r)):Promise.resolve(proc.exitCode);if(proc.exitCode===null)proc.stdin.end('done\n');result.hostExit=await ended;fs.closeSync(log);
  result.peer=fs.existsSync(peerLog)?fs.readFileSync(peerLog,'utf8').trim().split('\n').filter(Boolean).map(s=>JSON.parse(s)):[];
  fs.writeFileSync(path.join(out,'result.json'),JSON.stringify(result,null,2));
 }
 assert.equal(result.hostExit,0,'fixture exits cleanly');assert.equal(result.errors.length,0,'page has no script errors');
 console.log('PASS '+name);
}
(async()=>{try{
 for(const id of ['codex','devin','claude','ollama'])await run('page-'+id,id,'page');
 for(const id of ['codex','devin','claude'])await run('page-judge-'+id,'codex','page',null,id);
 await run('query-cached','codex','query');
 for(const method of ['AgentProviders','JudgementModels','JudgementCalibration','JudgementModelPrices','OllamaMemory','Sessions'])await run('query-'+method,'codex','query',[method,[]]);
 await run('query-ChatSkillCatalog','codex','query',['ChatSkillCatalog',[path.join(root,'query-ChatSkillCatalog','home','workspaces','Harbor'),'codex']]);
 await run('query-ModelPrices','codex','query',['ModelPrices',['devin']]);
 await run('refusal-codex','codex','refusal');
}finally{fs.writeFileSync(path.join(root,'results.json'),JSON.stringify(evidence,null,2));}})().catch(e=>{console.error(e);process.exitCode=1;});
