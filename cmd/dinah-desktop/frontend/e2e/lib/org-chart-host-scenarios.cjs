'use strict';
const fs = require('node:fs');
const path = require('node:path');

// The runner supplies a real bridge call and the rendered unavailable reason.
// No function here changes the store or supplies a mutation outcome.
const prefix = 'OrgChartHostFixture';
const ordered = value => Array.isArray(value) ? value.map(ordered) : value && typeof value === 'object'
  ? Object.fromEntries(Object.keys(value).sort().map(k => [k, ordered(value[k])])) : value;
function canonical(raw) {
  const settings = raw.settings;
  const ids = Object.fromEntries(Object.entries(settings.busSessions || {}).map(([chat, recipient]) => [recipient, chat]));
  const chat = id => ids[id] || id;
  const rooms = raw.relations.Rooms;
  const reporting = raw.relations.Members;
  return ordered({
    teams: rooms.filter(r => r.Lead).map(r => ({id: 'team:' + r.Name, lead: chat(r.Lead),
      members: reporting.filter(m => m.Lead === r.Lead).map(m => chat(m.Session)), channelId: r.Name,
      title:r.Title})).sort((a,b) => a.id.localeCompare(b.id)),
    channels: rooms.map(r => ({id:r.Name, title:r.Title, members:r.Members.map(m=>chat(m.Key)),
      budget:r.Budget, used:r.Used, allowances:raw.relations.Allowances[r.Name],
      record:Object.fromEntries(Object.entries(raw.relations.RoomRecords[r.Name]).filter(([k])=>!['name','title','lead','turn_budget','turns_used'].includes(k)))})).sort((a,b)=>a.id.localeCompare(b.id)),
    groups: rooms.filter(r => !r.Lead).map(r => ({id:r.Name, title:r.Title,
      channelId:r.Name})).sort((a,b) => a.id.localeCompare(b.id)),
    // Settings include every persisted chat identity, override and reservation.
    // Transient relation-row Since timestamps remain in the raw observations.
    recipients: raw.relations.SessionRecords, transcripts: raw.transcripts, chatArtifacts: raw.chatArtifacts, chats: settings, workspaces: raw.workspaces, protectedRecords: raw.protectedRecords
  });
}
const result = raw => ({ok: raw.status === 'succeeded' || raw.ok === true, reason: raw.message || raw.error || '', raw});
async function runHostScenario({call, product, scenario, artifactDirectory, visibleReason, browserErrors}) {
	if (!Array.isArray(browserErrors)) throw Error('Supply the runner\'s observed browser-error array.');
  if (typeof visibleReason !== 'function' && ['lead-assignment','lead-delete-cleanup','lead-delete-chart'].includes(scenario)) {
    throw Error('The rendered unavailable-reason binding is required for this host scenario.');
  }
  const observations = [];
  async function invoke(name, ...args) {
    try { const value = await call(name, ...args); observations.push({name,args,value}); return value; }
    catch (error) { observations.push({name,args,error:String(error.message || error)}); throw error; }
  }
  const manifest = await invoke(prefix+'Manifest');
  const ids = manifest.chats;
  const snapshot = () => invoke(prefix+'Snapshot');
  const actions = id => invoke(prefix+'Actions',id);
  const control = request => invoke(prefix+'Control',request);
  async function apply(request, offered) {
    if (!offered) offered = (await actions(request.chatId || '')).actions.find(a => a.kind === request.kind);
    return result(await invoke(prefix+'Apply',{...request,token:offered?.token || 'forged-absent-offer'}));
  }
  async function deletion(surface, id, offered) {
    if (surface === 'chart') return apply({kind:'delete_chat',chatId:id}, offered);
    try {
      if (surface === 'cleanup') {
        const rows = await control({kind:'bulk-delete',ids:[id]});
        return result(rows[0]);
      }
      await control({kind:'delete',chatId:id}); return {ok:true,reason:''};
    } catch (error) { return {ok:false,reason:String(error.message || error)}; }
  }
  const initial = await snapshot();
  const e = {product,scenario,artifactDirectory,browserErrors,remoteWrites:initial.remoteWrites};
  if (scenario === 'lead-assignment' || scenario === 'nested-lead') {
    const nested = scenario === 'nested-lead';
    const id = ids[nested?'nested-lead':'lead-a'], kind = nested?'replace_team_lead':'assign_team';
    const offered = await actions(id);
    e.before = canonical(initial);
    e.offeredActions = offered.actions.map(a=>a.kind);
    if (nested) {
      e.parentId='team:'+manifest.teams['lead-a']; e.childId='team:'+manifest.teams['nested-lead'];
      e.response = await apply({kind,chatId:id,groupId:manifest.teams['lead-a']});
    } else {
      e.visibleReason = await visibleReason({surface:'chart',id,kind});
      e.response = await apply({kind,chatId:id,leadId:ids['lead-b']});
    }
    e.after=canonical(await snapshot());
  } else if (scenario === 'replace-lead') {
    e.before=canonical(initial); e.teamId='team:'+manifest.teams['lead-a']; e.memberId=ids['member-a'];
    await control({kind:'observe-swap'});
    e.response=await apply({kind:'replace_team_lead',chatId:e.memberId,groupId:manifest.teams['lead-a']});
    const after=await snapshot(); e.after=canonical(after);
    e.observedCommittedSnapshots=after.observedCommittedSnapshots.map(canonical);
    e.afterRestart=canonical(await control({kind:'restart'}));
  } else if (scenario === 'single-chat-assignment') {
    e.chatId=ids['member-a']; e.targetId='team:'+manifest.teams['lead-b']; e.before=canonical(initial);
    const offered=await actions(e.chatId); e.offeredActions=offered.actions.map(a=>a.kind);
    // The request contract targets one canonical chat; retain every offered kind.
    e.offeredActionTargets=offered.actions.map(a=>({kind:a.kind,scope:a.kind==='assign_team'?'single-chat':'declared-'+a.kind,raw:a}));
    e.response=await apply({kind:'assign_team',chatId:e.chatId,leadId:ids['lead-b']});
    e.after=canonical(await snapshot()); e.wholeTeamAttempts=[];
    async function attempt(request, offer) {
      const before=canonical(await snapshot()), response=await apply(request,offer), after=canonical(await snapshot());
      e.wholeTeamAttempts.push({before,response,after,request});
    }
    await attempt({kind:'assign_team',chatId:ids['lead-a'],leadId:ids['lead-b']});
    const stale=(await actions(ids['free-b'])).actions.find(a=>a.kind==='assign_team');
    await control({kind:'set-lead',chatId:ids.closed,leadId:ids['free-b']});
    await attempt({kind:'assign_team',chatId:ids['free-b'],leadId:ids['lead-b']},stale);
    await attempt({kind:'assign_team',chatId:ids['lead-a'],leadId:ids['lead-b'],memberIds:[ids['member-b'],ids['nested-lead']]});
  } else if (['lead-delete-cleanup','lead-delete-chart','lead-delete-agent'].includes(scenario)) {
    const surface=scenario.slice('lead-delete-'.length), id=ids['lead-a'];
    const offered=await actions(id), unavailable=offered.unavailable.find(a=>a.kind==='delete_chat');
    e.before=canonical(initial); e.reportingMemberIds=e.before.teams.find(t=>t.lead===id).members;
    e.deletionOffered=surface==='cleanup'?initial.cleanup.rows.find(r=>r.sessionId===id).eligible:offered.actions.some(a=>a.kind==='delete_chat');
    e.response=await deletion(surface,id);
    e.visibleReason=surface==='agent'?e.response.reason:await visibleReason({surface,id,kind:'delete_chat'});
    e.hostUnavailable=unavailable; const after=await snapshot(); e.after=canonical(after);
    e.nativeWrites=after.trace.slice(initial.trace.length);
    e.removalEventCount=after.events.slice(initial.events.length).filter(v=>/removed|deleted/.test(v.name)).length;
    const old=(await actions(ids['free-b'])).actions.find(a=>a.kind==='delete_chat');
    if (!old) throw Error('The stale deletion control did not obtain a valid offer.');
    await control({kind:'set-lead',chatId:ids.closed,leadId:ids['free-b']});
    e.staleBefore=canonical(await snapshot());
    e.staleOfferResponse=await deletion(surface,ids['free-b'],old);
    e.staleAfter=canonical(await snapshot());
  } else if (scenario==='lead-delete-bulk') {
    e.leadId=ids['lead-a']; e.eligibleIds=[ids['free-b'],ids.closed];
    const stable = raw => {
      const snapshot=canonical(raw), eligible=new Set(e.eligibleIds);
      // Binning removes the selected recipients from channels and records their
      // bin entries. Every other persisted field remains in this comparison.
      for(const channel of snapshot.channels) channel.members=channel.members.filter(id=>!eligible.has(id));
      const rawIDs=new Set(e.eligibleIds.map(id=>raw.settings.busSessions[id]));
      for(const row of snapshot.recipients) if(rawIDs.has(row.id)) {
        // BinRecipients changes only these two lifecycle columns. Keep every
        // other field of even the selected recipients under comparison.
        delete row.removed_at; delete row.ended_at;
      }
      if(snapshot.chats.removedChats) {
        for(const id of e.eligibleIds) delete snapshot.chats.removedChats[id];
        if(!Object.keys(snapshot.chats.removedChats).length) delete snapshot.chats.removedChats;
      }
      return snapshot;
    };
    e.leadAndTeamBefore=stable(initial); e.unrelatedBefore=stable(initial);
    const rows=await control({kind:'bulk-delete',ids:[e.eligibleIds[0],e.leadId,e.eligibleIds[1]]});
    e.results=rows.map(row=>({chatId:row.sessionId,...result(row)}));
    const after=await snapshot(); e.newBinIds=after.cleanup.removed.map(r=>r.sessionId).filter(id=>!initial.cleanup.removed.some(r=>r.sessionId===id));
    e.leadAndTeamAfter=stable(after); e.unrelatedAfter=stable(after); e.nativeWrites=after.trace.slice(initial.trace.length);
  } else if (scenario==='lead-delete-after-reassign-members'||scenario==='lead-delete-after-replace-lead') {
    e.formerLeadId=ids['lead-a']; e.beforeResponse=await deletion('agent',e.formerLeadId);
    e.groupIdsBefore=initial.relations.Rooms.map(r=>r.Name).sort(); e.channelIdBefore=manifest.teams['lead-a'];
    if(scenario.endsWith('replace-lead')) {
      e.transitionResponse=await apply({kind:'replace_team_lead',chatId:ids['member-a'],groupId:e.channelIdBefore});
    } else {
      // Resolve the seeded legacy child first. This is a real clear operation.
      await control({kind:'clear-lead',chatId:ids['nested-member']});
      for(const role of ['member-a','member-b','nested-lead']) e.transitionResponse=await apply({kind:'assign_team',chatId:ids[role],leadId:ids['lead-b']});
    }
    const transition=await snapshot(); e.groupIdsAfter=transition.relations.Rooms.map(r=>r.Name).sort();
    e.channelIdAfter=transition.relations.Rooms.find(r=>r.Name===e.channelIdBefore)?.Name;
    e.afterResponse=await deletion('agent',e.formerLeadId);
    const after=await snapshot(); e.binIds=after.cleanup.removed.map(r=>r.sessionId);
    e.reportingToFormerLead=canonical(after).teams.filter(t=>t.lead===e.formerLeadId).flatMap(t=>t.members);
    e.nativeWrites=after.trace.slice(initial.trace.length);
  } else throw Error('Unknown host scenario '+scenario);
  const final=await snapshot();
  e.providerRequests=final.trace.length-initial.trace.length;
  if('nativeWrites' in e) e.nativeWrites=final.trace.slice(initial.trace.length);
  e.remoteWrites=final.remoteWrites;
  if(scenario.startsWith('lead-delete-')) {
    await control({kind:'provider-mutation-positive-control',chatId:manifest.open});
    const positive=await snapshot();
    e.nativeMutationPositiveControl=positive.trace.slice(final.trace.length);
    if(!e.nativeMutationPositiveControl.some(t=>t.kind==='session/set_config_option')) throw Error('The non-delete provider mutation was not observed.');
  }
  fs.mkdirSync(artifactDirectory,{recursive:true});
  fs.writeFileSync(path.join(artifactDirectory,'host-raw.json'),JSON.stringify({manifest,observations},null,2));
  fs.writeFileSync(path.join(artifactDirectory,'host-evidence.json'),JSON.stringify(e,null,2));
  return e;
}
module.exports={canonical,runHostScenario};
