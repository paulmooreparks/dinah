// Rebuilding a restored chat must not mutate the displayed saved transcript.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    app: () => {
      const messages = [{role:'you',text:'Original question'}, {role:'tool',text:'Original tool\nStatus: completed'}, {role:'agent',text:'Original answer'}];
      return {
        AgentProviders: async () => [{id:'codex',installed:true,available:true}],
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({history:[]}),
        SavedConversation: async () => ({available:true,messages}),
        OpenSessionAt: async (cwd,sessionId) => {
          await new Promise(resolve => { window.__beginReplay=resolve; });
          __emit('agent:updates',{sessionId,updates:[
            {sessionUpdate:'tool_call',toolCallId:'replay-tool',title:'Replayed tool',status:'completed'},
            {sessionUpdate:'agent_message_chunk',content:{type:'text',text:'Replayed answer'}}
          ]});
          await new Promise(resolve => { window.__finishReplay=resolve; });
          return {sessionId,cwd,provider:'codex',open:true};
        }
      };
    }
  });
  try {
    const p=t.page;
    await p.evaluate(() => dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'codex:isolation'}));
    await p.waitForFunction(() => !!window.__beginReplay);
    await p.locator('.win.active .msg-text', {hasText:'Original answer'}).waitFor();
    await p.evaluate(() => __beginReplay());
    await p.waitForFunction(() => !!window.__finishReplay);
    await p.waitForTimeout(400);
    t.ok('replay tools remain outside the displayed saved log', await p.locator('.win.active .tool-title', {hasText:'Replayed tool'}).count() === 0);
    t.ok('the saved answer remains visible while replay is held', await p.locator('.win.active .msg-text', {hasText:'Original answer'}).isVisible());
    await t.shot('chat-replay-isolation-held');
    await p.evaluate(() => __finishReplay());
    await p.locator('.win.active .msg-text', {hasText:'Replayed answer'}).waitFor();
    t.ok('the completed replay keeps its response', true);
  } finally { await t.close(); }
})().catch(error => {console.error(error);process.exitCode=1;});
