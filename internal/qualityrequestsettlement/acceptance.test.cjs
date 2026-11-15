'use strict';
const assert=require('node:assert/strict');
const {test}=require('node:test');
const path=require('node:path');
const same=(a,b)=>assert.deepEqual(a,b);
const sources=['request','mention','attention-report','partial-request','task-single','task-text','task-multiple'];
const views=source=>source.startsWith('task-')?['notifications','decisions','task-run']:['notifications','decisions'];
async function capture(t,scenario,source='request',input={}) {
  assert.ok(process.env.QUALITY_REQUEST_SETTLEMENT_BINDING,'Request-settlement acceptance is unbound');
  const f=await require(path.resolve(process.env.QUALITY_REQUEST_SETTLEMENT_BINDING)).open({scenario,source,...input});
  t.after(()=>f.close());
  const e=await f.run();
  assert.equal(e.scenario,scenario);assert.equal(e.source,source);assert.ok(e.artifactDirectory);
  same(e.providerRequests,[]);same(e.actionRequests,[]);assert.equal(e.remoteWriteAttempts,0);
  same(e.protectedAfter,e.protectedBefore);same(e.unrelatedAfter,e.unrelatedBefore);
  return e;
}
function refusal(c,code) {
  assert.equal(c.response.state,'refused');assert.ok(c.response.code);assert.ok(c.response.text);
  if(code) assert.equal(c.response.code,code);
  same(c.after,c.before);
}
function closed(e,outcome,source=e.source) {
  assert.equal(e.before.record.status,'open');assert.equal(e.after.record.status,outcome);
  assert.equal(e.after.record.id,e.before.record.id);
  same(e.after.record.request,e.before.record.request);same(e.after.record.sourceLink,e.before.record.sourceLink);
  same(e.after.record.answers,e.before.record.answers);
  assert.ok(e.after.record.settledAt);
  assert.equal(e.after.record.actorKind,outcome==='dismissed'?'person':'owner');
  assert.equal(e.receipt.state,'committed');assert.equal(e.receipt.outcome,outcome);
  assert.equal(e.receipt.id,e.before.record.id);assert.equal(e.receipt.operationId,e.operationId);
  assert.ok(e.receipt.revision);assert.ok(e.receipt.settledAt);
  same(Object.keys(e.after.views).sort(),views(source).sort());
  for(const name of views(source)) {
    assert.equal(e.after.views[name].open.some(row=>row.id===e.before.record.id),false);
    assert.equal(e.after.views[name].count,e.before.views[name].count-1);
  }
  const history=e.after.history.find(row=>row.id===e.before.record.id);
  assert.ok(history && history.address && history.wording);
  assert.equal(history.status,outcome);
  assert.match(history.wording,outcome==='dismissed'?/person.*dismiss|dismiss.*person/i:/chat.*withdr|withdr.*chat|owner.*withdr|withdr.*owner/i);
  assert.equal(e.answerDeliveries.length,0);
  assert.equal(e.logicalCompletions.length,1);
  assert.equal(e.logicalCompletions[0].outcome,outcome);
  assert.equal(e.logicalCompletions[0].hasAnswer,false);
  if(e.seed.askingChat) {
    assert.equal(e.chatNotices.length,1);assert.equal(e.chatNotices[0].recipient,e.seed.askingChat);
    assert.equal(e.chatNotices[0].outcome,outcome);assert.equal(e.chatNotices[0].hasAnswer,false);
  } else assert.equal(e.chatNotices.length,0);
  if(source.startsWith('task-')) {
    assert.equal(e.waiter.released,true);assert.equal(e.waiter.disposition,outcome);
    assert.equal(e.waiter.hasAnswer,false);assert.equal(e.operationReceipt.disposition,outcome);
    assert.equal(e.operationReceipt.hasAnswer,false);
    same(e.afterRebind.operationReceipt,e.operationReceipt);
    assert.equal(e.questionReplayCount,0);assert.equal(e.afterRebind.pendingKeys.includes(e.originalQuestionKey),false);
  }
  if(source==='partial-request') assert.ok(e.before.record.answers.length>0);
}
for(const source of sources) for(const outcome of ['dismissed','withdrawn']) test(`${source}: ${outcome} closes without answering`,async t=>{
  const e=await capture(t,'settle',source,{outcome});closed(e,outcome);
});
for(const source of sources) test(`${source}: Answer Dismiss and Withdraw compete once`,async t=>{
  const e=await capture(t,'concurrent-settlement',source);
  assert.deepEqual(e.commands,source==='attention-report'?['Dismiss','Withdraw']:['Answer','Dismiss','Withdraw']);
  if(source==='attention-report') {
    assert.equal(e.unsupportedAnswer.accepted,false);
    assert.equal(e.unsupportedAnswer.authorityGranted,false);
    assert.equal(e.unsupportedAnswer.sideEffects,0);
  }
  assert.equal(e.responses.filter(r=>r.accepted).length,1);
  const winner=e.responses.find(r=>r.accepted);
  assert.ok((source==='attention-report'?['dismissed','withdrawn']:['answered','dismissed','withdrawn']).includes(winner.outcome));
  assert.equal(e.after.record.status,winner.outcome);
  assert.equal(e.after.record.id,e.before.record.id);same(e.after.record.request,e.before.record.request);
  same(e.after.record.sourceLink,e.before.record.sourceLink);
  for(const old of e.before.record.answers) same(e.after.record.answers.find(a=>a.questionId===old.questionId),old);
  assert.equal(e.terminalRecords.length,1);assert.equal(e.logicalCompletions.length,1);
  assert.equal(e.answerDeliveries.length,winner.outcome==='answered'?1:0);
  same(e.afterRestart.record,e.after.record);same(e.afterRestart.completions,e.logicalCompletions);
  assert.equal(e.repeatRequests,0);assert.equal(e.questionReplayCount,0);
  for(const loser of e.responses.filter(r=>!r.accepted)) assert.equal(loser.code,'request_closed');
});
test('Dismiss is person-only despite caller flags or owner status',async t=>{
  const e=await capture(t,'person-only-dismiss');
  same(e.cases.map(c=>c.name),['asking-chat','foreign-chat','service','forged-person']);
  for(const c of e.cases) refusal(c);
  assert.equal(e.personResult.state,'committed');assert.equal(e.personResult.outcome,'dismissed');
});
test('Withdraw resolves ownership from authenticated chat context',async t=>{
  const e=await capture(t,'authenticated-chat-withdraw');
  same(e.cases.map(c=>c.name),['foreign-chat','same-group','shared-workspace','same-provider','same-title','forged-owner','removed-owner','unresolvable-owner']);
  for(const c of e.cases) refusal(c);
  assert.equal(e.ownerResult.state,'committed');assert.equal(e.ownerResult.outcome,'withdrawn');
});
test('consumer withdrawal cannot answer dismiss or cross scope',async t=>{
  const e=await capture(t,'consumer-authority','task-text');
  same(e.cases.map(c=>c.name),['foreign-service','foreign-workspace','foreign-task','stale-generation','wrong-admission-chat','service-answer','service-dismiss','forged-person']);
  for(const c of e.cases) refusal(c);
  assert.equal(e.ownResult.state,'committed');assert.equal(e.ownResult.outcome,'withdrawn');
  assert.equal(e.waiter.hasAnswer,false);
});
test('commands remain represented with truthful disabled reasons',async t=>{
  const e=await capture(t,'command-representations');
  same(e.states.map(s=>s.name),['open','answered','dismissed','withdrawn','ownerless','owner-unavailable','stale-generation']);
  for(const state of e.states) {
    const names=state.commands.map(c=>c.name);
    for(const name of ['Dismiss','Withdraw',...e.seed.answerCommands]) assert.ok(names.includes(name));
    for(const c of state.commands) {
      assert.equal(typeof c.enabled,'boolean');assert.equal(c.callable,c.enabled);
      if(!c.enabled) assert.ok(c.reason.trim());
    }
    if(['answered','dismissed','withdrawn','stale-generation'].includes(state.name)) assert.equal(state.commands.some(c=>c.enabled),false);
    if(['ownerless','owner-unavailable'].includes(state.name)) assert.equal(state.commands.find(c=>c.name==='Withdraw').enabled,false);
    if(state.name==='open') assert.equal(state.commands.find(c=>c.name==='Dismiss').enabled,true);
  }
});
for(const source of ['request','task-multiple']) test(`${source}: receipts are actor and command scoped and idempotent`,async t=>{
  const e=await capture(t,'receipt-idempotency',source);
  assert.equal(e.first.state,'committed');same(e.repeat,e.first);same(e.lookup,e.first);
  same(e.afterRepeat,e.afterFirst);
  refusal(e.changedInput);refusal(e.foreignLookup);refusal(e.differentOperation,'request_closed');
  assert.equal(e.logicalCompletions.length,1);
  assert.equal(e.otherActorSameId.state,'committed');assert.equal(e.otherCommandSameId.state,'committed');
  assert.notEqual(e.otherActorSameId.id,e.first.id);assert.notEqual(e.otherCommandSameId.id,e.first.id);
});
for(const source of ['request','task-text']) test(`${source}: operation IDs use Unicode scalar limits`,async t=>{
  const input={valid:['x','😀'.repeat(128)],invalid:['','😀'.repeat(129),null,4,'\ud800']};
  const e=await capture(t,'operation-id-bounds',source,input);
  same(e.valid.map(x=>x.operationId),input.valid);same(e.invalid.map(x=>x.operationId),input.invalid);
  for(const c of e.valid) assert.equal(c.response.state,'committed');
  for(const c of e.invalid) refusal(c);
});
for(const cause of ['stale-revision','stale-generation','cancelled','shutdown']) test(`${cause}: late settlement cannot revive authority`,async t=>{
  const e=await capture(t,'late-settlement','task-text',{cause});
  assert.ok(e.cases.length>=2);
  same(e.cases.map(c=>c.command).sort(),['Dismiss','Withdraw']);
  for(const c of e.cases) refusal(c,cause==='stale-revision'?'revision_changed':undefined);
  same(e.afterRestart,e.beforeRestart);
  assert.equal(e.answerDeliveries.length,0);assert.equal(e.questionReplayCount,0);
  if(cause==='cancelled'||cause==='stale-generation') assert.notEqual(e.historyStatus,'withdrawn');
});
for(const cause of ['cancel','generation-change','shutdown']) test(`${cause}: revocation competes at the settlement boundary`,async t=>{
  const e=await capture(t,'concurrent-revocation','task-text',{cause});
  same(e.runs.map(r=>r.name),['settlement-wins','revocation-wins']);
  for(const run of e.runs) {
    assert.equal(run.terminalRecords.length,1);assert.equal(run.logicalCompletions.length,1);
    assert.equal(run.answerDeliveries.length,0);assert.equal(run.questionReplayCount,0);
    same(run.afterRestart,run.beforeRestart);
    if(run.name==='revocation-wins') {
      assert.equal(run.settlementCommitted,false);
      assert.equal(['dismissed','withdrawn','answered'].includes(run.record.status),false);
    } else {
      assert.equal(run.settlementCommitted,true);
      assert.ok(['dismissed','withdrawn'].includes(run.record.status));
    }
  }
});
for(const point of ['before-authoritative-commit','unknown-commit-result','lost-response-after-commit','notice-dispatch-interrupted']) test(`${point}: reconcile the original settlement without another attempt`,async t=>{
  const e=await capture(t,'interrupted-settlement','task-text',{point});
  assert.ok(e.injectedAtActualBoundary);
  if(point==='before-authoritative-commit') {
    assert.equal(e.afterFault.record.status,'open');assert.equal(e.afterFault.completionObligations.length,0);
    assert.equal(e.afterRestart.record.status,'open');assert.equal(e.terminalRecords.length,0);
    assert.equal(e.logicalCompletions.length,0);assert.equal(e.lookupCommitted,false);
    assert.equal(e.settlementAttemptsAfterReconcile,e.settlementAttemptsBeforeReconcile);
    return;
  } else if(point==='unknown-commit-result') {
    assert.equal(e.responseKnown,false);assert.equal(e.newOperationAttempts,0);
  }
  assert.equal(e.settlementAttemptsAfterReconcile,e.settlementAttemptsBeforeReconcile);
  same(e.reconciledReceipt,e.committedReceipt);
  assert.equal(e.afterRestart.record.status,e.committedReceipt.outcome);
  assert.equal(e.terminalRecords.length,1);assert.equal(e.logicalCompletions.length,1);
  assert.equal(e.answerDeliveries.length,0);assert.equal(e.questionReplayCount,0);
  assert.equal(e.durableObligationCommittedWithSource,true);
  assert.ok(e.commitSnapshots.length>=3);
  for(const snapshot of e.commitSnapshots) assert.equal(snapshot.obligationCount,snapshot.sourceClosed?1:0);
  assert.ok(e.commitSnapshots.some(x=>!x.sourceClosed));assert.ok(e.commitSnapshots.some(x=>x.sourceClosed));
  assert.equal(e.reconciledByOriginalOperationId,true);
});
for(const adapter of ['DismissDecision','DismissNotification']) test(`${adapter}: settle source before projecting dismissal`,async t=>{
  const e=await capture(t,'legacy-dismiss-adapter','task-single',{adapter});
  assert.equal(e.sourceReceipt.outcome,'dismissed');assert.equal(e.waiter.released,true);
  assert.equal(e.waiter.hasAnswer,false);assert.equal(e.pendingAfter.length,0);
  assert.ok(e.sourceCommitSequence<e.projectionCommitSequence);
  same(e.afterRestart.sourceReceipt,e.sourceReceipt);assert.equal(e.questionReplayCount,0);
});
for(const outcome of ['dismissed','withdrawn']) test(`offline asker: durable ${outcome} completion retains its destination`,async t=>{
  const e=await capture(t,'unavailable-asker','request',{outcome});
  assert.equal(e.afterOffline.record.status,outcome);
  assert.equal(e.afterOffline.obligations.length,1);assert.equal(e.afterOffline.dispatches.length,0);
  assert.equal(e.afterOffline.obligations[0].recipient,e.seed.askingChat);
  assert.equal(e.afterRestart.obligations[0].recipient,e.seed.askingChat);
  assert.equal(e.afterReconnect.logicalCompletions.length,1);
  assert.equal(e.afterReconnect.logicalCompletions[0].recipient,e.seed.askingChat);
  assert.equal(e.answerDeliveries.length,0);assert.equal(e.redirectedCompletions.length,0);
});
test('ownerless report can be dismissed without inventing a recipient',async t=>{
  const e=await capture(t,'ownerless-report','ownerless-report');
  refusal(e.withdrawal);assert.equal(e.dismissal.state,'committed');assert.equal(e.after.record.status,'dismissed');
  assert.equal(e.chatNotices.length,0);assert.equal(e.completionRecipients.length,0);
  assert.equal(e.localDispositions.length,1);
});
test('Cannot be delivered dismissal preserves unknown work and later incidents',async t=>{
  const e=await capture(t,'unknown-underlying-report','attention-report');
  assert.equal(e.dismissal.outcome,'dismissed');same(e.underlyingAfter,e.underlyingBefore);
  assert.ok(e.underlyingBefore.unknownOutcomes.length>0);
  assert.equal(e.deliveryRetries,0);assert.equal(e.allocations,0);
  assert.ok(e.newIncident.id!==e.oldIncident.id || e.newIncident.revision!==e.oldIncident.revision);
  assert.equal(e.newIncident.status,'open');refusal(e.oldDismissalAgainstNewIncident);
});
test('missed view events repair from source without replaying settlement',async t=>{
  const e=await capture(t,'missed-projection-event');
  assert.equal(e.eventDropped,true);assert.equal(e.sourceRecord.status,'withdrawn');
  for(const state of [e.afterReread,e.afterRestart]) for(const name of views('request')) {
    assert.equal(state.views[name].open.some(row=>row.id===e.sourceRecord.id),false);
    assert.equal(state.views[name].history.find(row=>row.id===e.sourceRecord.id).status,'withdrawn');
  }
  assert.equal(e.settlementAttempts,1);assert.equal(e.logicalCompletions.length,1);
});
test('existing answers and typed bounds survive settlement extension',async t=>{
  const e=await capture(t,'legacy-and-typed-preservation');
  same(e.legacyAfter,e.legacyBefore);same(e.answeredTypedAfter,e.answeredTypedBefore);
  same(e.pendingSchemasAfter,e.pendingSchemasBefore);
  same(e.types,['single','text','multiple']);
  for(const c of e.invalidAnswerCases) refusal(c);
  assert.ok(e.invalidAnswerCases.length>=3);
});
