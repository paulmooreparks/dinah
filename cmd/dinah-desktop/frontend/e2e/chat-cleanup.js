const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ app: () => {
    const make = (id, extra) => Object.assign({ sessionId: id, title: 'Chat ' + id, workspace: 'C:/Fixture', workspaceName: 'Fixture', provider: 'codex', providerName: 'Codex', lastActivity: '2025-01-01T00:00:00Z', messageCount: 17, messageCountKnown: true, open: false, eligible: true, refusal: '', restorable: false }, extra);
    const rows = [make('old', { programCopies: [{ provider: 'claude', providerName: 'Claude' }] }), make('working', { eligible: false, refusal: 'The chat is working.', working: true, open: true }), make('pinned', { eligible: false, refusal: 'Unpin this chat first.', pinned: true }), make('other', { workspace: 'C:/Other', workspaceName: 'Other', provider: 'devin', providerName: 'Devin' }), make('unknown', { lastActivity: '', messageCountKnown: false })].sort((a, b) => a.lastActivity.localeCompare(b.lastActivity) || a.sessionId.localeCompare(b.sessionId));
    const bin = ['bin', 'bin2'].map(id => make(id, { removedAt: '2026-11-01T00:00:00Z', expiresAt: '2026-12-01T00:00:00Z', restorable: true, runtimeNote: 'Still loaded by Codex until the program stops or the window restarts.', finalDeleteConfirmation: 'This chat will be permanently deleted. Codex keeps its own copy.' }));
    window.__cleanupCalls = [];
    return {
      ListChatCleanup: async () => ({ rows, removed: bin, retentionDays: 30, finalDeleteConfirmation: 'The chats will be permanently deleted. Agent copies without a delete command remain.' }),
      RemoveChats: ids => new Promise(resolve => { window.__cleanupCalls.push(ids); window.__finishCleanup = () => resolve(ids.map(id => id === 'old' ? { sessionId: id, ok: true, status: 'removed', row: make(id, { restorable: true, removedAt: '2026-11-15T00:00:00Z', expiresAt: '2026-12-15T00:00:00Z', finalDeleteConfirmation: 'This chat will be permanently deleted.' }) } : { sessionId: id, ok: false, status: 'refused', error: 'The chat started working.' })); }),
      RestoreRemovedChat: async id => {
        if (id === 'old' && !window.__restoreRefused) { window.__restoreRefused = true; return { sessionId: id, ok: false, status: 'refused', error: 'This chat cannot be restored yet.' }; }
        return { sessionId: id, ok: true, status: 'restored', row: make(id) };
      },
      DeleteRemovedChat: async id => {
        if (!window.__deleteRefused) { window.__deleteRefused = true; return { sessionId: id, ok: false, status: 'failed', error: 'The local copy could not be erased.' }; }
        return { sessionId: id, ok: true, status: 'deleted', providerCopyRemains: true, providerDeletionReason: 'Codex does not offer deletion.' };
      },
      EmptyChatRecycleBin: async () => [{ sessionId: 'bin2', ok: true, status: 'deleted' }]
    };
  } });
  const p = t.page;
  try {
    await p.locator('#chat-browser-view [data-cmd="clean-up-chats"]').waitFor();
    const key = await p.evaluate(() => dinahWorkbench.open('chat-cleanup'));
    const win = p.locator('[data-win="' + key + '"]'), rows = win.locator('.cleanup-section').first().locator('.cleanup-row');
    const chat = id => win.locator('.cleanup-section').first().locator('[data-chat-id="' + id + '"]');
    await rows.first().waitFor();
    await p.evaluate(() => { dinahUIState.set('dinah-draft:chat:old', { text: 'Unsent chat message.' }); dinahUIState.set('dinah-draft:chat:bin', { text: 'Retained chat message.' }); });
    const discoveryCount = await p.evaluate(() => __bridgeCalls.filter(call => ['Sessions', 'SessionsAt', 'OpenSessionAt'].includes(call[0])).length);
    await t.shot('chat-cleanup-initial');
    t.ok('the bin discloses a loaded program without hiding the deletion policy', (await win.locator('.cleanup-bin-row').first().innerText()).includes('Still loaded by Codex'));
    await win.getByLabel('Agent Program', { exact: true }).selectOption('claude');
    t.ok('an earlier agent program finds the whole switched chat', await chat('old').isVisible() && !await chat('other').isVisible());
    await win.getByLabel('Agent Program', { exact: true }).selectOption('');
    const geometry = () => win.evaluate(root => Array.from(root.querySelectorAll('.cleanup-toolbar,.cleanup-row')).map(el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; }));
    const before = await geometry();
    t.ok('working and pinned chats cannot be selected and explain why', await chat('working').locator('input').isDisabled() && await chat('pinned').locator('input').isDisabled() && (await chat('pinned').innerText()).includes('Unpin'));
    t.ok('unknown counts and dates are explicit', (await chat('unknown').innerText()).includes('Unknown'));
    await win.getByRole('button', { name: 'Select All Shown', exact: true }).click();
    t.ok('selection keeps every row and toolbar fixed', JSON.stringify(before) === JSON.stringify(await geometry()));
    await win.getByRole('button', { name: 'Delete 3 Chats…', exact: true }).click();
    await p.locator('dialog.cleanup-confirm').getByRole('button', { name: 'Cancel', exact: true }).click();
    t.ok('cancelling changes no chats', await p.evaluate(() => __cleanupCalls.length === 0));
    await win.getByRole('button', { name: 'Delete 3 Chats…', exact: true }).click();
    await p.locator('dialog.cleanup-confirm').getByRole('button', { name: 'Delete', exact: true }).click();
    await p.waitForFunction(() => !!window.__finishCleanup);
    const pending = await geometry();
    t.ok('a pending batch keeps the list fixed', JSON.stringify(before) === JSON.stringify(pending));
    await p.evaluate(() => {
      const root = document.querySelector('.chat-cleanup'); window.__cleanupFrames = [];
      const sample = () => Object.fromEntries(Array.from(root.querySelectorAll('.cleanup-row,.cleanup-toolbar')).concat(Array.from(document.querySelectorAll('.win-bar'))).map(el => {
        const r = el.getBoundingClientRect(), list = el.closest('ul');
        return [(list ? list.getAttribute('aria-label') + ':' + el.dataset.chatId : el.className), [r.x, r.y, r.width, r.height]];
      }));
      window.__cleanupFrames.push(sample()); window.__finishCleanup(); let left = 10;
      function frame() { window.__cleanupFrames.push(sample()); if (--left) requestAnimationFrame(frame); else window.__cleanupSampled = true; } requestAnimationFrame(frame);
    });
    await p.waitForFunction(() => window.__cleanupSampled);
    t.ok('mixed results never move an existing row or the taskbar in any sampled frame', await p.evaluate(() => __cleanupFrames.every(frame => Object.entries(__cleanupFrames[0]).every(([key, value]) => JSON.stringify(frame[key]) === JSON.stringify(value)))));
    require('fs').writeFileSync(require('path').join(process.argv[3], 'chat-cleanup-frames.json'), JSON.stringify(await p.evaluate(() => __cleanupFrames), null, 2));
    t.ok('success and later failures are reported independently', (await chat('old').innerText()).includes('Recycle Bin') && (await chat('other').innerText()).includes('started working'));
    t.ok('staging preserves the unsent chat message', await p.evaluate(() => dinahUIState.get('dinah-draft:chat:old', null).text === 'Unsent chat message.'));
    await win.locator('.cleanup-row-actions').getByRole('button', { name: 'Restore', exact: true }).first().click();
    await win.locator('[data-outcome="restored"]').waitFor();
    t.ok('restore leaves the chat closed and reports its result', (await win.locator('[data-outcome="restored"]').innerText()).includes('Restored as a closed chat'));
    await p.evaluate(() => __emit('chat:restored', { sessionId: 'bin' }));
    t.ok('Restore returns closed metadata without provider discovery', await p.evaluate(before => !!dinahChats.ref('bin') && __bridgeCalls.filter(call => ['Sessions', 'SessionsAt', 'OpenSessionAt'].includes(call[0])).length === before, discoveryCount));
    t.ok('Restore preserves the unsent chat message', await p.evaluate(() => dinahUIState.get('dinah-draft:chat:bin', null).text === 'Retained chat message.'));
    const binRows = win.locator('.cleanup-section').nth(1);
    await binRows.locator('[data-chat-id="old"]').getByRole('button', { name: 'Restore', exact: true }).click();
    t.ok('a refused Restore is reported on its bin row rather than the retained active-list result', (await binRows.locator('[data-chat-id="old"]').innerText()).includes('cannot be restored yet') && !(await chat('old').innerText()).includes('cannot be restored yet'));
    await binRows.locator('[data-chat-id="old"]').getByRole('button', { name: 'Delete Now…', exact: true }).click();
    t.ok('final deletion uses the row confirmation verbatim', (await p.locator('dialog.cleanup-confirm').locator('p').innerText()) === 'This chat will be permanently deleted.');
    await p.locator('dialog.cleanup-confirm').getByRole('button', { name: 'Delete', exact: true }).click();
    t.ok('a failed final deletion stays on its bin row', (await binRows.locator('[data-chat-id="old"]').innerText()).includes('local copy could not be erased'));
    await binRows.locator('[data-chat-id="old"]').getByRole('button', { name: 'Delete Now…', exact: true }).click();
    await p.locator('dialog.cleanup-confirm').getByRole('button', { name: 'Delete', exact: true }).click();
    await binRows.locator('[data-chat-id="old"][data-outcome="deleted"]').waitFor();
    t.ok('a remaining agent copy is disclosed after local deletion', (await binRows.locator('[data-chat-id="old"]').innerText()).includes('Codex does not offer deletion.'));
    t.ok('final deletion forgets only that chat composer value', await p.evaluate(() => dinahUIState.get('dinah-draft:chat:old', null) === null && !!dinahUIState.get('dinah-draft:chat:bin', null)));
    await win.getByRole('button', { name: 'Empty Recycle Bin…', exact: true }).click();
    t.ok('emptying the bin uses the aggregate confirmation verbatim', (await p.locator('dialog.cleanup-confirm').locator('p').innerText()) === 'The chats will be permanently deleted. Agent copies without a delete command remain.');
    await p.locator('dialog.cleanup-confirm').getByRole('button', { name: 'Delete', exact: true }).click();
    await binRows.locator('[data-chat-id="bin2"][data-outcome="deleted"]').waitFor();
    await win.getByLabel('Workspace', { exact: true }).selectOption('C:/Other');
    t.ok('filter state is addressable', (await p.url()).includes('workspace%3DC%253A%252FOther'));
    await t.shot('chat-cleanup-wide');
    await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForFunction(() => document.querySelector('.chat-cleanup').getBoundingClientRect().width < 400);
    t.ok('narrow controls and rows fit their page', await win.evaluate(root => Array.from(root.querySelectorAll('.cleanup-filters,.cleanup-toolbar,.cleanup-row:not([hidden])')).every(el => el.scrollWidth <= el.clientWidth + 1)));
    await t.shot('chat-cleanup-narrow');
    await p.reload();
    await p.locator('.chat-cleanup [aria-label="Workspace"]').waitFor();
    t.ok('reloading the page restores its filter from the URL', await p.locator('.chat-cleanup [aria-label="Workspace"]').inputValue() === 'C:/Other');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
