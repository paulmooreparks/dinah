// Failed steps have an opt-in review in the run. Failed turns still alert.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    state: { workspace: 'C:\\W', recent: ['C:\\W'], approvals: 'all', devin: 'd', titles: {}, templates: [] },
    app: () => {
      let n = 0;
      return {
        NewSessionAt: async () => ({ sessionId: 's' + (++n), cwd: 'C:\\W', title: '', open: true, working: false, queued: [] }),
        Send: async () => {}, Answer: async () => {}
      };
    },
  });
  const { page: p, ok } = t;
  require('./lib/windows')(p);
  try {
    await p.waitForTimeout(400);
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Install the setup');
    await p.click('.win[data-chat-current] [data-view-id=send]');
    await p.waitForTimeout(300);
    await p.evaluate(() => {
      const id = 's1', up = (u) => window.__emit('agent:update', { sessionId: id, update: u });
      up({ sessionUpdate: 'tool_call', toolCallId: 't1', title: 'Ran command', kind: 'execute', status: 'in_progress', rawInput: { command: 'git push' } });
      up({ sessionUpdate: 'tool_call_update', toolCallId: 't1', status: 'failed', content: [{ type: 'content', content: { type: 'text', text: 'Denied by a permission rule' } }] });
      up({ sessionUpdate: 'tool_call', toolCallId: 't2', title: 'Ran command', kind: 'execute', status: 'completed', rawInput: { command: 'ls' } });
      up({ sessionUpdate: 'tool_call', toolCallId: 't3', title: 'Edited a file', kind: 'edit', status: 'in_progress' });
      up({ sessionUpdate: 'tool_call_update', toolCallId: 't3', status: 'failed', content: [{ type: 'content', content: { type: 'text', text: 'The file is locked' } }] });
    });
    await p.waitForTimeout(150);
    ok('nothing is told while the turn runs', (await p.$$('#questions .heads-up')).length === 0);
    await p.evaluate(() => window.__emit('agent:turn', { sessionId: 's1', stopReason: 'end_turn', seconds: 12 }));
    await p.waitForTimeout(150);
    ok('failed tools create no notification entries or summary by default', await p.locator('#questions .heads-up, .failure-summary').count() === 0);
    await p.evaluate(() => pudlCommands.run('failure-summaries', undefined, document.querySelector('.win[data-chat-current] .chat-instance')));
    await p.locator('.failure-summary').waitFor();
    ok('opting in shows one summary inside the scrolling chat history', await p.locator('.activity .failure-summary').count() === 1 && (await p.locator('.failure-summary').textContent()).includes('2 steps failed.'));
    ok('failed tools do not increment Notifications', await p.isHidden('#status-needs .badge'));
    await p.locator('.failure-summary').getByRole('button', { name: 'Review Failed Commands' }).click();
    await p.locator('.failure-review details').first().waitFor();
    const items = await p.locator('.failure-review details').allTextContents();
    ok('review opens a floating window with command and output details', items.length === 2 && items[0].includes('git push') && items[0].includes('Denied by a permission rule') && items[1].includes('The file is locked'));
    await t.shot('headsup');
    await p.evaluate(() => dinahWorkbench.open({kind: 'chat-session', workspace: 'C:\\W', path: 's1'}));
    await p.locator('.failure-summary').getByRole('button', { name: 'Dismiss', exact: true }).click();
    ok('Dismiss removes the summary and its retained review details', await p.locator('.failure-summary').count() === 0);
    // A step the person rejected is not told.
    await p.evaluate(() => {
      window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call', toolCallId: 't4', title: 'Ran command', kind: 'execute', status: 'pending', rawInput: { command: 'rm x' } } });
      window.__emit('agent:permission', { sessionId: 's1', key: 'k1', toolCall: { toolCallId: 't4', rawInput: { command: 'rm x' } }, options: [{ optionId: 'allow_once', name: 'Allow', kind: 'allow_once' }, { optionId: 'reject_once', name: 'Reject', kind: 'reject_once' }] });
    });
    await p.waitForTimeout(100);
    await p.click('#questions .question button:has-text("Reject")');
    await p.evaluate(() => {
      window.__emit('agent:answered', { sessionId: 's1', key: 'k1' });
      window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call_update', toolCallId: 't4', status: 'failed' } });
      window.__emit('agent:turn', { sessionId: 's1', stopReason: 'end_turn', seconds: 3 });
    });
    await p.waitForTimeout(150);
    ok('a step the person rejected is not told', (await p.$$('#questions .heads-up')).length === 0);
    await p.evaluate(() => {
      for (let i = 0; i < 120; i++) __emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call', toolCallId: 'limit-' + i, title: 'Failure ' + i, kind: 'execute', status: 'failed' } });
      __emit('agent:turn', { sessionId: 's1', stopReason: 'end_turn' });
    });
    await p.waitForFunction(() => document.querySelector('.failure-summary')?.textContent.includes('120 steps failed.'));
    ok('new failures after dismissal create one new summary', await p.locator('.failure-summary').count() === 1 && (await p.locator('.failure-summary').textContent()).includes('120 steps failed.'));
    await p.locator('.failure-summary').getByRole('button', { name: 'Review Failed Commands' }).click();
    await p.waitForFunction(() => document.querySelectorAll('.failure-review details').length === 100);
    ok('review limits retained steps and explains the limit', (await p.locator('.failure-review').textContent()).includes('Showing the latest 100 steps'));
    await p.evaluate(() => dinahWorkbench.open({kind: 'chat-session', workspace: 'C:\\W', path: 's1'}));
    await p.evaluate(() => pudlCommands.run('failure-summaries', undefined, document.querySelector('.win[data-chat-current] .chat-instance')));
    ok('turning summaries off removes the history row', await p.locator('.failure-summary').count() === 0);
    // A turn that ends with an error is told.
    await p.evaluate(() => window.__emit('agent:turn', { sessionId: 's1', stopReason: 'error', error: 'The model is overloaded.', seconds: 1 }));
    await p.waitForTimeout(150);
    ok('a turn ending in an error is told', (await p.textContent('#questions .heads-up .notice-title')) === 'The turn ended with an error.' && (await p.textContent('#questions .heads-up .notice-body')).includes('The model is overloaded.'));
    // It survives Devin stopping; questions do not.
    await p.evaluate(() => window.__emit('agent:state', { state: 'stopped', detail: 'Devin stopped' }));
    await p.waitForTimeout(100);
    ok('it stays when Devin stops', (await p.$$('#questions .heads-up')).length === 1);
  } finally { await t.close(); }
})();
