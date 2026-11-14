const fs=require('node:fs'),readline=require('node:readline'),path=require('node:path');
const dir=process.env.QUALITY_PAUSE_DIR,mode=process.env.QUALITY_PAUSE_MODE;
let pending=null,turn=0;
function emit(x){process.stdout.write(JSON.stringify({jsonrpc:'2.0',...x})+'\n');}
function record(x){fs.appendFileSync(path.join(dir,'wire.jsonl'),JSON.stringify(x)+'\n');}
function text(p){return (p.input||p.prompt||[]).map(x=>x.text||'').join('\n');}
function classify(s){return {work:s.includes('quality working turn'),pause:s.includes('Stop at the next safe point'),resume:s.includes('has restarted')};}
function finish(){if(!pending)return;const p=pending;pending=null;if(mode==='codex')emit({method:'turn/completed',params:{threadId:'fixture',turn:{id:p.turn,status:'completed'}}});else emit({id:p.id,result:{stopReason:'end_turn'}});}
setInterval(()=>{if(fs.existsSync(path.join(dir,'finish')))finish();},10).unref();
readline.createInterface({input:process.stdin}).on('line',line=>{
 const m=JSON.parse(line),p=m.params||{};
 const reply=result=>emit({id:m.id,result});
 if(m.id===undefined)return;
 record({method:m.method,...classify(text(p)),expectedTurnId:p.expectedTurnId,idleBehavior:p._meta?.steering?.idleBehavior});
 switch(m.method){
 case 'initialize': reply(mode==='codex'?{}:{protocolVersion:1,agentCapabilities:{loadSession:true},_meta:mode==='claude'?{steering:{supported:true}}:{}});break;
 case 'config/read':reply({config:{}});break;
 case 'model/list':reply({data:[{id:'test',model:'test',displayName:'Test',inputModalities:['text'],defaultReasoningEffort:'medium',supportedReasoningEfforts:[{reasoningEffort:'medium',description:'Medium'}]}]});break;
 case 'thread/start':case 'thread/resume':reply({thread:{id:'fixture',turns:[]},model:'test',reasoningEffort:'medium'});break;
 case 'thread/read':reply({thread:{id:'fixture',turns:[]}});break;
 case 'session/new':reply({sessionId:'fixture',configOptions:[]});break;
 case 'session/load':reply({configOptions:[]});break;
 case 'turn/start':case 'session/prompt':{
  if(pending){record({violation:'concurrent ordinary prompt'});emit({id:m.id,error:{code:-32600,message:'concurrent prompt'}});break;}
  const s=text(p);const id='turn-'+(++turn);pending={id:m.id,turn:id};
  if(mode==='devin')fs.writeFileSync(path.join(dir,'prompt'),s);
  if(mode==='codex')reply({turn:{id,status:'inProgress'}});
  if(!classify(s).work)finish();
  break;
 }
 case 'turn/steer':
  if(!pending||p.expectedTurnId!==pending.turn)throw Error('incorrect active turn');
  reply({turnId:pending.turn});break;
 case '_session/steering':
  if(p._meta?.steering?.idleBehavior!=='promptRequired')throw Error('unsafe idle behavior');
  reply(pending?{outcome:'injected'}:{outcome:'promptRequired',reason:'noRunningTurn'});break;
 case 'session/cancel':case 'turn/interrupt':record({violation:'unexpected cancellation'});reply({});break;
 default:reply({});
 }
});
