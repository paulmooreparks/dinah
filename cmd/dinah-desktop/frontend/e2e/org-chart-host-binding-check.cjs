'use strict';
const {spawn}=require('node:child_process');
const readline=require('node:readline');
const path=require('node:path');
const fs=require('node:fs');
const assert=require('node:assert/strict');
const {runHostScenario}=require('./lib/org-chart-host-scenarios.cjs');
const [binary,output]=process.argv.slice(2);
if(!binary||!output) throw Error('Supply the tagged test binary and an isolated artifact folder.');
async function run(scenario) {
  const dir=path.join(output,scenario);fs.mkdirSync(dir,{recursive:true});
  const child=spawn(binary,['-test.run=^TestOrgChartHostFixtureRPC$','-test.timeout=90s'],{windowsHide:true,env:{...process.env,DINAH_ORG_FIXTURE_RPC:'1',DINAH_TEST_DIAGNOSTICS_DIR:dir},stdio:['pipe','pipe','pipe']});
  let seq=0, pending=new Map();
  const exit=new Promise((resolve,reject)=>child.once('close',code=>code===0?resolve():reject(Error('Fixture exited '+code))));
  exit.catch(()=>{});
  const log=fs.createWriteStream(path.join(dir,'fixture-private.log'));
  child.stderr.pipe(log,{end:false});
  readline.createInterface({input:child.stdout}).on('line',line=>{
    if(!line.startsWith('ORG_REPLY ')){log.write(line+'\n');return;}
    const reply=JSON.parse(line.slice(10)), waiter=pending.get(reply.id);pending.delete(reply.id);
    if(reply.error)waiter.reject(Error(reply.error));else waiter.resolve(reply.value);
  });
  const call=(name,...args)=>new Promise((resolve,reject)=>{const id=++seq;pending.set(id,{resolve,reject});child.stdin.write(JSON.stringify({id,name,args})+'\n');});
  try {
    const e=await runHostScenario({call,product:'Dinah',scenario,artifactDirectory:dir,browserErrors:[],
      // This diagnostic checks host translation only. The combined receiving
      // binding must supply the visible reason from the real rendered surface.
      visibleReason:async({surface,id,kind})=>{
        if(surface==='cleanup'){const raw=await call('OrgChartHostFixtureSnapshot');return raw.cleanup.rows.find(r=>r.sessionId===id).refusal;}
        return (await call('OrgChartHostFixtureActions',id)).unavailable.find(r=>r.kind===kind).reason;
      }});
    assert.equal(e.remoteWrites,0);
    if(e.nativeWrites) assert.deepEqual(e.nativeWrites,[]);
    if(scenario.startsWith('lead-delete-')) assert.ok(e.nativeMutationPositiveControl.some(t=>t.kind==='session/set_config_option'));
    if(scenario==='replace-lead') {
      assert.equal(e.response.ok,true);const expected=structuredClone(e.before),team=expected.teams.find(t=>t.id===e.teamId),old=team.lead;
      team.lead=e.memberId;team.members=team.members.map(id=>id===e.memberId?old:id);
      assert.deepEqual(e.after,expected);assert.deepEqual(e.afterRestart,expected);
      for(const observed of e.observedCommittedSnapshots)assert.ok(JSON.stringify(observed)===JSON.stringify(e.before)||JSON.stringify(observed)===JSON.stringify(expected));
    } else if(scenario==='single-chat-assignment') {
      assert.equal(e.response.ok,true);const expected=structuredClone(e.before);
      const source=expected.teams.find(t=>t.members.includes(e.chatId)), target=expected.teams.find(t=>t.id===e.targetId);
      const oldChannel=expected.channels.find(c=>c.id===source.channelId), newChannel=expected.channels.find(c=>c.id===target.channelId);
      source.members=source.members.filter(id=>id!==e.chatId);target.members.push(e.chatId);
      oldChannel.members=oldChannel.members.filter(id=>id!==e.chatId);newChannel.members.push(e.chatId);
      assert.deepEqual(e.after,expected);
      for(const attempt of e.wholeTeamAttempts){assert.equal(attempt.response.ok,false);assert.deepEqual(attempt.after,attempt.before);}
    } else if(scenario==='lead-delete-bulk') {
      assert.equal(e.results.find(r=>r.chatId===e.leadId).ok,false);
      for(const id of e.eligibleIds)assert.equal(e.results.find(r=>r.chatId===id).ok,true);
      assert.deepEqual(e.newBinIds.sort(),e.eligibleIds.sort());assert.deepEqual(e.leadAndTeamAfter,e.leadAndTeamBefore);assert.deepEqual(e.unrelatedAfter,e.unrelatedBefore);
    } else if(scenario.startsWith('lead-delete-after-')) {
      assert.equal(e.beforeResponse.ok,false);assert.equal(e.transitionResponse.ok,true);assert.equal(e.afterResponse.ok,true);
      assert.ok(e.binIds.includes(e.formerLeadId));assert.deepEqual(e.groupIdsAfter,e.groupIdsBefore);assert.equal(e.channelIdAfter,e.channelIdBefore);
    } else {
      assert.equal(e.response.ok,false);assert.deepEqual(e.after,e.before);
      if(e.staleOfferResponse){assert.equal(e.staleOfferResponse.ok,false);assert.deepEqual(e.staleAfter,e.staleBefore);}
    }
    console.log(JSON.stringify({scenario,pass:true,scope:'host translation diagnostic; rendered reasons unverified'}));
  } finally {child.stdin.end();try{await exit;}finally{log.end();}}
}
(async()=>{for(const scenario of ['lead-assignment','replace-lead','nested-lead','single-chat-assignment','lead-delete-cleanup','lead-delete-chart','lead-delete-agent','lead-delete-bulk','lead-delete-after-reassign-members','lead-delete-after-replace-lead']){try{await run(scenario);}catch(error){console.error(scenario,error);process.exitCode=1;}}})().catch(error=>{console.error(error);process.exitCode=1;});
