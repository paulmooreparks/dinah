// A bus-opened chat may still be loading when its restored window mounts.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    init: () => {
      let store;
      Object.defineProperty(window,'dinahSessionStore',{get:()=>store,set:value=>{store={...value,create:adapter=>{const result=value.create(adapter);window.__records=result.records;return result;}};}});
    },
    app: () => ({
      AgentProviders: async () => [{id:'codex',installed:true,available:true}],
      ResolveSessionContinuation: async id => id,
      SessionContinuation: async () => ({history:[]}),
      SavedConversation: async () => ({available:true,streamGen:window.__finished?'background':undefined,streamThrough:3,
        messages:[{role:'you',text:'The original question'}, {role:'agent',text:window.__finished?'The final recovered answer':'The saved answer'}]}),
      OpenSessionAt: async (cwd,sessionId) => {
        window.__joined=true;
        await new Promise(resolve => {window.__finish=resolve;});
        window.__finished=true;
        return {sessionId,cwd,provider:'codex',open:true,historyLoading:false,streamGen:'background',streamHead:3};
      },
      ChatEvents: async () => ({gen:'background',head:3,events:[]})
    })
  });
  try {
    const p=t.page;
    await t.emit('agent:session',{sessionId:'codex:background',cwd:'C:/Fixture',provider:'codex',open:true,historyLoading:true,streamGen:'background',streamHead:0});
    await p.waitForFunction(()=>window.__records['codex:background']?.info.historyLoading);
    await p.evaluate(() => dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'codex:background'}));
    await p.waitForFunction(() => !!window.__joined, {}, {timeout:3000}).catch(()=>{});
    const joined=await p.evaluate(()=>!!window.__joined);
    t.ok('restoring window joins the engine open already in progress', joined, await p.evaluate(()=>JSON.stringify({info:__records['codex:background'].info,loading:__records['codex:background'].loading,failed:__records['codex:background'].failed})));
    if (!joined) return;
    await p.locator('.win.active .msg-text',{hasText:'The saved answer'}).waitFor();
    await t.emit('agent:updates',{sessionId:'codex:background',gen:'background',ns:[1,2,3],updates:[
      {sessionUpdate:'user_message_chunk',content:{type:'text',text:'An old replayed question'}},
      {sessionUpdate:'tool_call',toolCallId:'replay',title:'Replayed operation',status:'completed'},
      {sessionUpdate:'agent_message_chunk',content:{type:'text',text:'An old replayed answer'}}
    ]});
    await p.waitForTimeout(300);
    t.ok('saved response stays visible during background replay',await p.locator('.win.active .msg-text',{hasText:'The saved answer'}).isVisible());
    t.ok('old replay messages stay out of the visible transcript',await p.locator('.win.active .msg-text',{hasText:'An old replayed answer'}).count()===0);
    await p.evaluate(()=>__finish());
    await p.locator('.win.active .msg-text',{hasText:'The final recovered answer'}).waitFor();
    t.ok('completed checkpoint replaces the replay once',await p.locator('.win.active .msg-row').count()===2);
    t.ok('retired replay work is not moved into the checkpoint',await p.locator('.win.active .work-group').count()===0);
    await t.shot('chat-background-replay');
  } finally {await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
