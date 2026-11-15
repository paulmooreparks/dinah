// Measures the installed local model through documented Ollama APIs.
// Usage: node ollama-memory-trial.cjs <copied-model-store> <context> <fresh-output-folder>
const fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {spawn}=require('node:child_process');
const [models,contextSize,out]=process.argv.slice(2);
if(!models||!contextSize||!out||fs.existsSync(out))throw Error('Supply copied models, context and a fresh output folder');
fs.mkdirSync(out,{recursive:true});
const osHome=path.join(out,'ollama-home');fs.mkdirSync(osHome,{recursive:true});

const events=[];let ollama;
const at=()=>new Date().toISOString();
const record=value=>{events.push({at:at(),...value});fs.appendFileSync(path.join(out,'observations.jsonl'),JSON.stringify(events.at(-1))+'\n');};
async function until(fn,label,ms=180000){const end=Date.now()+ms;while(Date.now()<end){const v=await fn();if(v)return v;await new Promise(r=>setTimeout(r,100));}throw Error('Timed out: '+label);}
async function listen(server){await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});return server.address().port;}
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
  const cap=Number(contextSize);
  if(![8192,32768,65536].includes(cap))throw Error('Use 8192, 32768 or 65536 tokens');
  const shown=await (await fetch(server+'/api/show',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({model:'qwen3.5:4b'})})).json();
  record({kind:'model',architecture:shown.model_info['general.architecture'],context:shown.model_info[shown.model_info['general.architecture']+'.context_length']});
  for(let trial=0;trial<4;trial++){
   const request={model:'qwen3.5:4b',messages:[{role:'user',content:'In one short sentence, explain a context window. Trial '+trial+'.'}],think:false,stream:true,keep_alive:'5m',options:{num_ctx:cap,num_predict:32,temperature:0}};
   const started=performance.now();let first=null,last=null,text='',buffer='';
   const response=await fetch(server+'/api/chat',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(request)});
   if(!response.ok)throw Error(await response.text());
   for await(const raw of response.body){
    buffer+=Buffer.from(raw).toString('utf8');let n;
    while((n=buffer.indexOf('\n'))>=0){
     const line=buffer.slice(0,n);buffer=buffer.slice(n+1);if(!line.trim())continue;
     const chunk=JSON.parse(line);if(chunk.error)throw Error(chunk.error);
     if(chunk.message?.content){if(first===null)first=performance.now()-started;text+=chunk.message.content;}
     if(chunk.done)last=chunk;
    }
   }
   if(first===null||!last)throw Error('The streamed answer was incomplete');
   const ps=await (await fetch(server+'/api/ps')).json();
   if(ps.models.length!==1||ps.models[0].context_length!==cap)throw Error('Allocation does not match the requested context');
   record({kind:'measurement',cap,trial,loadedBefore:trial>0,firstTokenMs:first,totalMs:performance.now()-started,ps,last,text});
  }
 }catch(e){record({kind:'failure',error:String(e)});throw e;}
 finally{
  if(ollama&&ollama.exitCode===null){await new Promise(r=>spawn('taskkill.exe',['/PID',String(ollama.pid),'/T','/F'],{windowsHide:true,stdio:'ignore'}).on('exit',r));await new Promise(r=>{if(ollama.exitCode!==null)r();else ollama.once('exit',r);});}
 }
 console.log('MEASUREMENT COMPLETE: '+out);
})().catch(e=>{console.error(e);process.exitCode=1;});
