'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const {spawn} = require('node:child_process');

const rows = (raw, table) => raw.tables[table];
const question = raw => raw.questions.find(q => q.question.key === raw.source.id);
function completion(raw) {
  const id = raw.source.id;
  if (id.startsWith('task-q-')) return question(raw).completion;
  if (id.startsWith('notification:')) return raw.reports.reportSettlements[id]?.completion;
  return rows(raw, 'request_completion_obligations').find(r => r.request_id === id);
}
function owner(raw) {
  const s = raw.source;
  if (s.kind === 'task-question') return question(raw).recipient || '';
  if (s.kind === 'attention-report') return s.request.requestOwner || '';
  return s.request.askerSession || '';
}
function canonical(raw, source) {
  const s = raw.source, original = structuredClone(s.request);
  for (const k of ['revision','status','settledAt','actorKind','canDismiss','dismissalReason']) delete original[k];
  const answers = [];
  for (const q of original.questions || []) {
    if (q.answered) answers.push({questionId:q.number, answer:q.answer});
    delete q.answered; delete q.answer;
  }
  const record = {id:s.id, status:s.status, request:original, sourceLink:{id:s.id, owner:owner(raw)}, answers,
    settledAt:s.settledAt, actorKind:s.actorKind};
  const pending = raw.pending.map(q => ({id:q.key}));
  const notificationRows = raw.notifications.map(n => ({id:n.requestId || (n.permissionKey || 'notification:' + n.id)}));
  const unique = list => [...new Map(list.map(r=>[r.id,r])).values()];
  const open = {
    notifications:unique([...notificationRows,...pending]),
    decisions:unique([...raw.entries.items,...pending]),
    'task-run':pending
  };
  const names = source === 'attention-report' ? ['notifications'] : source.startsWith('task-') ? ['notifications','decisions','task-run'] : ['notifications','decisions'];
  const history = s.status === 'open' ? [] : [{id:s.id,status:s.status,wording:s.wording,address:'RequestState('+JSON.stringify(s.id)+')'}];
  const views = Object.fromEntries(names.map(name => [name,{open:open[name],count:open[name].length,history}]));
  return {record,views,history};
}
function logical(raw) {
  const c = completion(raw);
  if (!c) {
    if(raw.source.status==='answered' && raw.operationReceipt?.disposition==='answered')return [{outcome:'answered',hasAnswer:operation(raw).hasAnswer}];
    if(raw.source.status==='answered')return rows(raw,'decision_operations').map(r=>JSON.parse(r.receipt)).filter(r=>r.id===raw.source.id&&r.state==='committed'&&r.closed).map(r=>({outcome:'answered',hasAnswer:true,messageIds:r.messageIds}));
    return [];
  }
  return [{outcome:c.outcome,recipient:c.recipient || '',hasAnswer:Object.hasOwn(c,'answer')}];
}
function notices(raw) {
  const out = [];
  for (const m of rows(raw,'messages')) {
    let report; try { report = JSON.parse(m.text); } catch { continue; }
    if (report.invocationId !== 'request-settlement:'+raw.source.id) continue;
    for (const d of rows(raw,'deliveries').filter(d=>d.message_id===m.id && d.recipient_kind==='session')) {
      out.push({recipient:d.recipient,outcome:report.output.outcome,hasAnswer:Object.hasOwn(report.output,'answer'),messageId:m.id});
    }
  }
  return out;
}
function operation(raw) {
  return {...raw.operationReceipt,hasAnswer:raw.operationReceipt.result !== undefined && raw.operationReceipt.result !== null};
}

exports.open = async function(spec) {
  if (!['settle','concurrent-settlement'].includes(spec.scenario)) throw new Error('Receiving scenario remains unbound: '+spec.scenario);
  const binary = process.env.QUALITY_REQUEST_SETTLEMENT_BINARY;
  const sourceCommit = process.env.QUALITY_REQUEST_SETTLEMENT_SOURCE;
  if (!binary || !sourceCommit) throw new Error('Set QUALITY_REQUEST_SETTLEMENT_BINARY and QUALITY_REQUEST_SETTLEMENT_SOURCE');
  const artifactDirectory = fs.mkdtempSync(path.join(os.tmpdir(),'request-settlement-'));
  const ready = path.join(artifactDirectory,'ready.txt');
  fs.writeFileSync(path.join(artifactDirectory,'identity.json'),JSON.stringify({sourceCommit,binarySHA256:crypto.createHash('sha256').update(fs.readFileSync(binary)).digest('hex'),spec},null,2));
  const fd = fs.openSync(path.join(artifactDirectory,'host.log'),'w');
  const child = spawn(binary,['-test.run=^TestRequestSettlementAcceptanceServer$','-test.timeout=150s','-test.v'],{
    windowsHide:true,stdio:['ignore',fd,fd],env:{...process.env,QUALITY_REQUEST_SETTLEMENT_READY:ready,QUALITY_REQUEST_SETTLEMENT_ARTIFACT:artifactDirectory}
  });
  fs.closeSync(fd);
  const exited = new Promise((resolve,reject)=>{child.once('error',reject);child.once('exit',(code,signal)=>resolve({code,signal}));});
  let url;
  try {
    url = await new Promise((resolve,reject)=>{
      let finished = false;
      const timer = setTimeout(()=>finish(new Error('Host readiness timed out; '+artifactDirectory)),30000);
      const watcher = fs.watch(artifactDirectory,read);
      function finish(err,value) { if(finished)return;finished=true;clearTimeout(timer);watcher.close();err?reject(err):resolve(value); }
      function read() { try {const text=fs.readFileSync(ready,'utf8');if(text)finish(null,text);}catch(err){if(err.code!=='ENOENT')finish(err);} }
      child.once('exit',code=>finish(new Error('Host exited before ready: '+code)));
      read();
    });
  } catch(err) {child.kill();await exited;throw err;}
  async function call(method,input={}) {
    const request={method,input};
    const response=await fetch(url,{method:'POST',body:JSON.stringify(request),signal:AbortSignal.timeout(30000)});
    if(!response.ok)throw new Error(method+': '+response.status+' '+await response.text());
    const body=await response.json();
    fs.appendFileSync(path.join(artifactDirectory,'binding-calls.jsonl'),JSON.stringify({request,response:body})+'\n');
    if(body.error)throw new Error(method+': '+body.error);
    return body.result;
  }
  let closePromise;
  return {
    async run() {
      const unrelated=await call('seed',{source:'request'});
      const seeded=await call('seed',{source:spec.source});
      const unrelatedBefore=await call('state',{id:unrelated.id});
      const beforeRaw=await call('inspect',{id:seeded.id});
      const operationId='frozen-original-settlement';
      const input={id:seeded.id,revision:seeded.revision,operationId};
      let receipt, race;
      if(spec.scenario==='concurrent-settlement')race=await call('race',input);
      else {
        const result=await call(spec.outcome==='dismissed'?'dismiss':'withdraw',input);
        receipt=spec.outcome==='dismissed'?result:JSON.parse(result.OutputJSON);
      }
      if(spec.source.startsWith('task-'))await call('wait',{id:seeded.id});
      await call('reconcile');
      const afterRaw=await call('inspect',{id:seeded.id});
      const unrelatedAfter=await call('state',{id:unrelated.id});
      const previous=new Set(rows(beforeRaw,'messages').map(m=>m.id));
      const answerDeliveries=rows(afterRaw,'messages').filter(m=>!previous.has(m.id)&&m.kind==='text'&&m.reply_to===seeded.request.messageId);
      const evidence={scenario:spec.scenario,source:spec.source,artifactDirectory,operationId,receipt,
        before:canonical(beforeRaw,spec.source),after:canonical(afterRaw,spec.source),seed:{askingChat:owner(beforeRaw)},
        answerDeliveries,logicalCompletions:logical(afterRaw),chatNotices:notices(afterRaw),
        unrelatedBefore,unrelatedAfter,...afterRaw.observations};
      if(race) {
        evidence.commands=race.commands;
        evidence.responses=race.responses.map(r=>{
          const value=r.result?.OutputJSON?JSON.parse(r.result.OutputJSON):r.result;
          const accepted=!r.error && (r.command==='Answer' && value===null || value?.state==='committed');
          let code=r.errorCode || value?.code || value?.reason?.code;
          // Canonical settlement names unify the existing legacy closed codes;
          // the unmodified responses stay in binding-calls.jsonl.
          if(code==='decision_closed' || r.command==='Answer' && code==='stale' && afterRaw.source.status!=='open')code='request_closed';
          return {command:r.command,accepted,outcome:accepted?(r.command==='Answer'?'answered':value.outcome):undefined,code};
        });
        if(race.unsupportedAnswer)evidence.unsupportedAnswer={accepted:!race.unsupportedAnswer.error&&race.unsupportedAnswer.result?.state==='committed',authorityGranted:beforeRaw.source.commands.some(c=>c.name==='Answer'&&c.enabled),sideEffects:rows(afterRaw,'decision_answer_origins').filter(r=>!rows(beforeRaw,'decision_answer_origins').some(b=>b.message_id===r.message_id)).length};
        evidence.terminalRecords=afterRaw.source.status==='open'?[]:[evidence.after.record];
        evidence.repeatRequests=Math.max(0,rows(afterRaw,'person_requests').length-rows(beforeRaw,'person_requests').length)+afterRaw.observations.questionReplayCount;
        const restart=await call('restart');
        if(restart && restart.accepted===false)throw new Error('Actual restart refused');
        const restarted=await call('inspect',{id:seeded.id});
        evidence.afterRestart={record:canonical(restarted,spec.source).record,completions:logical(restarted)};
        evidence.questionReplayCount=restarted.observations.questionReplayCount;
      }
      if(spec.source.startsWith('task-')) {
        evidence.waiter={released:afterRaw.waiterReleased,disposition:afterRaw.operationReceipt.disposition,hasAnswer:operation(afterRaw).hasAnswer};
        evidence.operationReceipt=operation(afterRaw);
        const restart=await call('restart');
        if(restart && restart.accepted===false)throw new Error('Actual task rebind refused: '+JSON.stringify(restart));
        const rebound=await call('inspect',{id:seeded.id});
        evidence.afterRebind={operationReceipt:operation(rebound),pendingKeys:rebound.pending.map(q=>q.key)};
        evidence.questionReplayCount=rebound.observations.questionReplayCount;
      }
      fs.writeFileSync(path.join(artifactDirectory,'evidence.json'),JSON.stringify(evidence,null,2));
      return evidence;
    },
    close() {
      if(closePromise)return closePromise;
      closePromise=(async()=>{
        let failure;
        try {if(await call('close')!==true)throw new Error('Actual host close was refused');}catch(err){failure=err;}
        let timer;
        const stopped=await Promise.race([exited,new Promise(resolve=>{timer=setTimeout(()=>resolve(null),10000);})]);
        clearTimeout(timer);
        if(stopped===null){child.kill();await exited;throw new Error('Host teardown watchdog; '+artifactDirectory);}
        if(stopped.code!==0)throw new Error('Host teardown failed '+JSON.stringify(stopped)+'; '+artifactDirectory);
        if(failure)throw failure;
      })();
      return closePromise;
    }
  };
};
