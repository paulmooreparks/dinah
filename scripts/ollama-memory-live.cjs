// Checks Settings memory readings through a real hidden window and local model.
// Usage: node ollama-memory-live.cjs <desktop.exe> <copied-model-store> <fresh-output-folder>
const fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {spawn}=require('node:child_process');
const {chromium}=require('playwright-core');
const [desktop,models,out]=process.argv.slice(2);
if(!desktop||!models||!out||fs.existsSync(out))throw Error('Supply binaries, copied models and a fresh output folder');
fs.mkdirSync(out,{recursive:true});
const workspace=path.join(out,'work'),home=path.join(out,'home'),osHome=path.join(out,'ollama-home');
for(const dir of [workspace,home,osHome])fs.mkdirSync(dir,{recursive:true});
const entry=path.join(out,'entry.txt'),exe=path.join(out,'dinah-desktop.exe');fs.copyFileSync(desktop,exe);
const events=[],errors=[];let phase='starting',browser,page,child,ollama,proxy;
const at=()=>new Date().toISOString();
const record=value=>{events.push({at:at(),...value});fs.appendFileSync(path.join(out,'observations.jsonl'),JSON.stringify(events.at(-1))+'\n');};
const files=()=>fs.readdirSync(workspace).filter(x=>x.endsWith('.txt')).sort();
const read=file=>{try{return fs.readFileSync(file,'utf8');}catch{return '';}};
async function until(fn,label,ms=180000){const end=Date.now()+ms;while(Date.now()<end){const v=await fn();if(v)return v;await new Promise(r=>setTimeout(r,100));}throw Error('Timed out: '+label);}
async function listen(server){await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});return server.address().port;}
const call=(name,...args)=>page.evaluate(({name,args})=>go.main.App[name](...args),{name,args});
async function ready(url){await page.goto(url);await page.waitForFunction(()=>window.dinahUIState&&window.dinahWorkbench);await page.evaluate(()=>dinahUIState.ready);}
(async()=>{
 try{
  const reservation=http.createServer();const ollamaPort=await listen(reservation);await new Promise(r=>reservation.close(r));
  const baseEnv=Object.fromEntries(Object.entries(process.env).filter(([key])=>['PATH','PATHEXT','SYSTEMROOT','WINDIR','COMSPEC','PROGRAMFILES','PROGRAMFILES(X86)','PROGRAMW6432','PROGRAMDATA','TEMP','TMP','NUMBER_OF_PROCESSORS','PROCESSOR_ARCHITECTURE','DINAH_BROWSER_TEST_TICKET'].includes(key.toUpperCase())));
  const environment={...baseEnv,USERPROFILE:osHome,HOME:osHome,APPDATA:path.join(osHome,'AppData/Roaming'),LOCALAPPDATA:path.join(osHome,'AppData/Local'),OLLAMA_MODELS:path.resolve(models),OLLAMA_NO_CLOUD:'1',OLLAMA_NOPRUNE:'1',OLLAMA_CONTEXT_LENGTH:'8192',OLLAMA_HOST:'127.0.0.1:'+ollamaPort};
  const log=fs.openSync(path.join(out,'ollama.log'),'w');
  ollama=spawn('C:/Users/paul/AppData/Local/Programs/Ollama/ollama.exe',['serve'],{env:environment,windowsHide:true,stdio:['ignore',log,log]});
  const server='http://127.0.0.1:'+ollamaPort;
  await until(async()=>{try{return (await fetch(server+'/api/version')).ok;}catch{return false;}},'isolated Ollama');
  const version=await (await fetch(server+'/api/version')).json();record({kind:'server',version,cloudDisabled:true});
  const tags=await (await fetch(server+'/api/tags')).json();record({kind:'catalog',tags});
  if(tags.models.length!==1||tags.models[0].name!=='qwen3.5:4b'||tags.models[0].remote_host)throw Error('The isolated catalog must contain only the selected local model');
  let sequence=0;
  proxy=http.createServer((req,res)=>{const chunks=[];req.on('data',x=>chunks.push(x));req.on('end',()=>{
   const body=Buffer.concat(chunks);let id,started=at();
   if(req.url==='/api/chat'){
    const q=JSON.parse(body);id=++sequence;
    record({kind:'request',id,phase,model:q.model,think:q.think,options:q.options,files:files(),users:(q.messages||[]).filter(x=>x.role==='user').map(x=>(x.content||'').split('[Dinah Applet Context]')[0])});
   }
   const upstream=http.request(server+req.url,{method:req.method,headers:{...req.headers,host:'127.0.0.1:'+ollamaPort}},response=>{
    res.writeHead(response.statusCode,response.headers);const responseChunks=[];
    response.on('data',x=>{responseChunks.push(x);res.write(x);});response.on('end',()=>{
     if(id){let lines=[];const raw=Buffer.concat(responseChunks).toString();for(const line of raw.trim().split('\n')){try{lines.push(JSON.parse(line));}catch{}}
      record({kind:'response',id,started,phase,status:response.statusCode,files:files(),text:lines.map(x=>x.message?.content||'').join(''),tools:lines.flatMap(x=>x.message?.tool_calls||[]),done:lines.at(-1)});
     }res.end();
    });
   });upstream.on('error',e=>{record({kind:'proxy-error',error:String(e)});res.destroy(e);});upstream.end(body);
  });});
  const proxyPort=await listen(proxy);
  const appLog=fs.openSync(path.join(out,'desktop.log'),'w');
  child=spawn(exe,[],{env:{...environment,DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entry,OLLAMA_HOST:'http://127.0.0.1:'+proxyPort},windowsHide:true,stdio:['ignore',appLog,appLog]});
  browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});page=await browser.newPage({viewport:{width:1440,height:1000}});page.on('pageerror',e=>errors.push(String(e)));
  const first=await until(()=>read(entry).trim(),'hidden entry');await ready(first);
  await call('SetWorkspace',workspace);await call('SetOllamaFileScope','workspace',[]);await call('SetOllamaRoundLimit',40);
  const info=await call('NewSessionWithProviderAt',workspace,'ollama'),id=info.sessionId;
  const opts=await call('SetSessionOption',id,'model','qwen3.5:4b');
  if(opts.some(x=>x.id==='think'))await call('SetSessionOption',id,'think','false');
  await call('RenameSession',id,'Local Memory Observation');
  const shown=await (await fetch(server+'/api/show',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({model:'qwen3.5:4b'})})).json();
  record({kind:'model-context',architecture:shown.model_info['general.architecture'],limit:shown.model_info[shown.model_info['general.architecture']+'.context_length']});
  const defaultCap=(await call('State')).ollamaContextCap;
  if(defaultCap!==32768)throw Error('The new default was not applied');
  await page.evaluate(()=>dinahWorkbench.open('settings'));
  const settings=page.locator('#settings');await settings.getByRole('tab',{name:'Agents',exact:true}).click();
  await page.waitForFunction(()=>document.querySelector('#ollama-memory-reading').textContent.startsWith('No local model'));
  record({kind:'unloaded-settings',text:await settings.locator('#ollama-memory-reading').innerText(),requests:events.filter(x=>x.kind==='request').length});
  if(events.some(x=>x.kind==='request'))throw Error('Opening Settings or creating a chat sent a prompt');
  const before=await settings.locator('#ollama-memory-reading').boundingBox();
  phase='default';await call('Send',id,'Reply with the single word Ready. Do not call tools.',null,null);
  await until(async()=>(await call('UpdateState')).working===0,'default turn ended',360000);
  const requests=events.filter(x=>x.kind==='request'&&x.phase==='default'&&x.users.some(u=>u.includes('single word Ready')));
  if(requests.length!==1||requests[0].options?.num_ctx!==defaultCap)throw Error('The automatic request did not use the new default');
  const reading=await call('OllamaMemory');
  if(reading.limit!==32768||reading.models.length!==1||reading.models[0].context!==32768||reading.models[0].targetContext!==32768)throw Error('The live memory query does not match the allocation');
  await settings.locator('#ollama-memory-refresh').click();
  await page.waitForFunction(()=>document.querySelector('#ollama-memory-reading').textContent.includes('size used at the saved ceiling'));
  const text=await settings.locator('#ollama-memory-reading').innerText(),after=await settings.locator('#ollama-memory-reading').boundingBox();
  if(!text.includes((reading.models[0].size/1073741824).toFixed(2)+' GiB'))throw Error('The page did not show the reported memory');
  if(before.height!==after.height||before.width!==after.width)throw Error('The memory reading changed its allocated space');
  record({kind:'settings-memory',reading,text,before,after,pageErrors:errors,ps:await (await fetch(server+'/api/ps')).json()});
  await page.screenshot({path:path.join(out,'settings-memory.png')});
  fs.writeFileSync(path.join(out,'result-page.txt'),await page.locator('body').innerText());

 }catch(e){record({kind:'failure',error:String(e),pageErrors:errors});if(page)fs.writeFileSync(path.join(out,'failure-page.txt'),await page.locator('body').innerText().catch(()=>''));throw e;}
 finally{
  if(browser)await browser.close();
  const script=`Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq '${exe.replaceAll("'","''")}' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`;
  await new Promise(r=>spawn('powershell.exe',['-NoProfile','-NonInteractive','-Command',script],{windowsHide:true,stdio:'ignore'}).on('exit',r));
  if(proxy)await new Promise(r=>proxy.close(r));
  if(ollama&&ollama.exitCode===null){await new Promise(r=>spawn('taskkill.exe',['/PID',String(ollama.pid),'/T','/F'],{windowsHide:true,stdio:'ignore'}).on('exit',r));await new Promise(r=>{if(ollama.exitCode!==null)r();else ollama.once('exit',r);});}
 }
 console.log('OBSERVATION COMPLETE: '+out);
})().catch(e=>{console.error(e);process.exitCode=1;});
