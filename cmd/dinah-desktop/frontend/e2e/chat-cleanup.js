// Clean Up Chats (chat-cleanup.js): two tabs, Chats and Recycle Bin, each a
// bar of controls, a table and a line that says what is shown or what just
// happened. The host's rules are the same as before the window was redrawn
// on 17 November 2026 (docs/mockups/clean-up-chats.html): which chats may be
// deleted and why not, what staging and final deletion keep and forget, and
// the confirmations in the host's own words. What changed is how results
// are shown: a chat that was changed leaves its table at once, and one that
// could not be changed keeps its row with the reason under its title.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 900 }, app: () => {
    const make = (id, extra) => Object.assign({ sessionId: id, title: 'Chat ' + id, workspace: 'C:/Fixture', workspaceName: 'Fixture', provider: 'codex', providerName: 'Codex', lastActivity: '2025-01-01T00:00:00Z', messageCount: 17, messageCountKnown: true, open: false, eligible: true, refusal: '', restorable: false }, extra);
    const rows = [make('old', { programCopies: [{ provider: 'claude', providerName: 'Claude' }], lastActivity: '2024-06-01T00:00:00Z', messageCount: 1204 }), make('working', { eligible: false, refusal: 'The chat is working.', working: true, open: true }), make('pinned', { eligible: false, refusal: 'Unpin this chat first.', pinned: true }), make('other', { workspace: 'C:/Other', workspaceName: 'Other', provider: 'devin', providerName: 'Devin', lastActivity: '2025-03-01T00:00:00Z', messageCount: 3 }), make('unknown', { lastActivity: '', messageCountKnown: false })];
    const bin = ['bin', 'bin2', 'external'].map(id => make(id, { removedAt: '2026-11-01T00:00:00Z', expiresAt: '2026-12-01T00:00:00Z', restorable: true, runtimeNote: 'Still loaded by Codex until the program stops or the window restarts.', finalDeleteConfirmation: 'This chat will be permanently deleted. Codex keeps its own copy.' }));
    window.__externalRestore = id => {
      bin.splice(bin.findIndex(row => row.sessionId === id), 1);
      const restored = make(id); rows.push(restored); dinahChats.restore(restored); __emit('chat:restored', { sessionId: id });
    };
    window.__cleanupCalls = []; window.__listings = 0;
    return {
      Sessions: async () => rows.map(r => ({ sessionId: r.sessionId, title: r.title, cwd: r.workspace, provider: r.provider, open: r.open })),
      ListChatCleanup: async () => {
        window.__listings++;
        const snapshot = { rows: rows.slice(), removed: bin.slice(), retentionDays: 30, finalDeleteConfirmation: 'The chats will be permanently deleted. Agent copies without a delete command remain.' };
        if (window.__holdCleanupRead) { window.__holdCleanupRead = false; return new Promise(resolve => { window.__releaseCleanupRead = () => resolve(snapshot); }); }
        return snapshot;
      },
      RemoveChats: ids => new Promise(resolve => { window.__cleanupCalls.push(ids); window.__finishCleanup = () => resolve(ids.map(id => {
        if (id !== 'old') return { sessionId: id, ok: false, status: 'refused', error: 'The chat started working.' };
        const removed = make(id, { restorable: true, removedAt: '2026-11-15T00:00:00Z', expiresAt: '2026-12-15T00:00:00Z', finalDeleteConfirmation: 'This chat will be permanently deleted.' });
        rows.splice(rows.findIndex(row => row.sessionId === id), 1); bin.push(removed);
        return { sessionId: id, ok: true, status: 'removed', row: removed };
      })); }),
      RestoreRemovedChat: async id => {
        if (id === 'old' && !window.__restoreRefused) { window.__restoreRefused = true; return { sessionId: id, ok: false, status: 'refused', error: 'This chat cannot be restored yet.' }; }
        bin.splice(bin.findIndex(row => row.sessionId === id), 1); const restored = make(id); rows.push(restored);
        return { sessionId: id, ok: true, status: 'restored', row: restored };
      },
      DeleteRemovedChat: async id => {
        if (!window.__deleteRefused) { window.__deleteRefused = true; return { sessionId: id, ok: false, status: 'failed', error: 'The local copy could not be erased.' }; }
        bin.splice(bin.findIndex(row => row.sessionId === id), 1);
        return { sessionId: id, ok: true, status: 'deleted', providerCopyRemains: true, providerDeletionReason: 'Codex does not offer deletion.' };
      },
      EmptyChatRecycleBin: async () => { bin.splice(bin.findIndex(row => row.sessionId === 'bin2'), 1); return [{ sessionId: 'bin2', ok: true, status: 'deleted' }]; }
    };
  } });
  const p = t.page;
  try {
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.locator('#chat-browser-view [data-cmd="clean-up-chats"]').waitFor();
    const key = await p.evaluate(() => dinahWorkbench.open('chat-cleanup'));
    const win = p.locator('[data-win="' + key + '"]'), chatsPanel = win.locator('#chat-cleanup-chats'), binPanel = win.locator('#chat-cleanup-bin');
    const chat = id => chatsPanel.locator('tr[data-chat-id="' + id + '"]'), binRow = id => binPanel.locator('tr[data-chat-id="' + id + '"]');
    const order = () => chatsPanel.locator('tbody tr[data-chat-id]:not([hidden])').evaluateAll(rows => rows.map(r => r.dataset.chatId).join(','));
    const foot = panel => panel.locator('.cleanup-foot [role="status"]').innerText();
    const confirmBox = p.locator('dialog.cleanup-confirm');
    await chat('old').waitFor();
    await p.evaluate(() => { dinahUIState.set('dinah-draft:chat:old', { text: 'Unsent chat message.' }); dinahUIState.set('dinah-draft:chat:bin', { text: 'Retained chat message.' }); });
    const discoveryCount = await p.evaluate(() => __bridgeCalls.filter(call => ['Sessions', 'SessionsAt', 'OpenSessionAt'].includes(call[0])).length);
    await t.shot('chat-cleanup-initial');

    // The window's shape.
    t.ok('the window has two tabs, Chats and Recycle Bin, each with its count', (await win.locator('[role="tab"]').allInnerTexts()).map(s => s.replace(/\s+/g, ' ').trim()).join('|') === 'Chats 5|Recycle Bin 3', (await win.locator('[role="tab"]').allInnerTexts()).join('|'));
    t.ok('the Chats tab is a table with a heading for each column', (await chatsPanel.locator('thead th').allInnerTexts()).map(s => s.trim()).filter(Boolean).join('|') === 'Chat|Workspace|Agent Programs|Last Activity|Messages|State');
    t.ok('a chat says its workspace, its agent programs, its messages and its state on one row', /Chat old\s+Fixture\s+Codex, Claude\s+1 Jun 2024.*\s+1,204\s+\S+/.test((await chat('old').innerText()).replace(/\n+/g, ' ')), await chat('old').innerText());
    t.ok('the line under the table says how many chats there are and what deleting does', /^5 chats\. A deleted chat goes to the Recycle Bin and can be restored for 30 days\.$/.test(await foot(chatsPanel)), await foot(chatsPanel));

    // Why a chat cannot be deleted, and what the host does not know.
    t.ok('working and pinned chats cannot be selected and say why under their titles', await chat('working').locator('input').isDisabled() && await chat('pinned').locator('input').isDisabled() && (await chat('pinned').locator('.cleanup-reason').innerText()).includes('Unpin') && await chat('pinned').locator('.cleanup-reason .glyph').count() === 1);
    t.ok('unknown counts and dates are explicit', await chat('unknown').locator('td.cleanup-when').innerText() === 'Unknown' && await chat('unknown').locator('td.num').innerText() === 'Unknown');

    // What is shown: the filters, the search and the order.
    await win.getByLabel('Agent Program', { exact: true }).selectOption('claude');
    t.ok('an earlier agent program finds the whole switched chat', await chat('old').isVisible() && !await chat('other').isVisible());
    await win.getByLabel('Agent Program', { exact: true }).selectOption('');
    await win.getByLabel('Find a chat by title').fill('chat ot');
    t.ok('the search finds chats by title and the line says how many are shown', await order() === 'other' && /^1 of 5 chats shown\./.test(await foot(chatsPanel)), await order() + ' / ' + await foot(chatsPanel));
    await win.getByLabel('Find a chat by title').fill('no such chat');
    t.ok('a search that finds nothing says so in the table', /No chat matches/.test(await chatsPanel.locator('.data-table-empty').innerText()));
    await win.getByLabel('Find a chat by title').fill('');
    t.ok('it opens with the oldest activity first, a chat of unknown activity before them', await order() === 'unknown,old,pinned,working,other' && await chatsPanel.locator('th[aria-sort="ascending"]').innerText() === 'Last Activity', await order());
    await chatsPanel.getByRole('button', { name: 'Messages', exact: true }).click();
    await chatsPanel.getByRole('button', { name: 'Messages', exact: true }).click();
    t.ok('pressing a heading sorts by it, and again turns the order round', await order() === 'old,pinned,working,other,unknown' && await chatsPanel.locator('th[aria-sort="descending"]').innerText() === 'Messages', await order());
    t.ok('the order is in the window\'s address', decodeURIComponent(decodeURIComponent(await p.url())).includes('sort=messages&dir=descending'), await p.url());
    await chatsPanel.getByRole('button', { name: 'Last Activity', exact: true }).click();
    t.ok('and the first order comes back', await order() === 'unknown,old,pinned,working,other', await order());

    // Selecting.
    const geometry = () => win.evaluate(root => Array.from(root.querySelectorAll('#chat-cleanup-chats .cleanup-bar, #chat-cleanup-chats .cleanup-actions, #chat-cleanup-chats .cleanup-actions > *, #chat-cleanup-chats .cleanup-delete, #chat-cleanup-chats tbody tr[data-chat-id], #chat-cleanup-chats .cleanup-foot')).map(el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height].map(Math.round); }));
    // Pressing a heading at the table's far side scrolled the table sideways; it is measured from its start.
    await chatsPanel.locator('.cleanup-table').evaluate(wrap => { wrap.scrollLeft = 0; });
    const before = await geometry();
    await chatsPanel.getByLabel('Select every chat shown').check();
    t.ok('the heading\'s checkbox selects every chat shown that may be deleted', await chatsPanel.locator('.cleanup-selected').innerText() === '3 selected' && await chat('old').locator('input').isChecked() && !await chat('pinned').locator('input').isChecked());
    const picked = await geometry();
    t.ok('selection keeps every row, bar and button fixed', JSON.stringify(before) === JSON.stringify(picked), before.map((box, i) => JSON.stringify(box) === JSON.stringify(picked[i]) ? '' : i + ': ' + box + ' > ' + picked[i]).filter(Boolean).join(' | '));
    await chatsPanel.getByRole('button', { name: 'Delete 3 Chats…', exact: true }).click();
    t.ok('the question lists the chats and says they can be restored', (await confirmBox.locator('li').allInnerTexts()).sort().join('|') === 'Chat old|Chat other|Chat unknown' && /restored for 30 days/.test(await confirmBox.locator('p').innerText()));
    t.ok('deleting to the Recycle Bin is not marked as dangerous, and Cancel has the focus', await confirmBox.getByRole('button', { name: 'Delete', exact: true }).evaluate(b => b.classList.contains('btn-primary') && !b.classList.contains('btn-danger')) && await p.evaluate(() => document.activeElement.textContent) === 'Cancel');
    await confirmBox.getByRole('button', { name: 'Cancel', exact: true }).click();
    t.ok('cancelling changes no chats', await p.evaluate(() => __cleanupCalls.length === 0));
    await chatsPanel.getByRole('button', { name: 'Delete 3 Chats…', exact: true }).click();
    await confirmBox.getByRole('button', { name: 'Delete', exact: true }).click();
    await p.waitForFunction(() => !!window.__finishCleanup);
    t.ok('a pending batch keeps the list fixed and its controls off', JSON.stringify(before) === JSON.stringify(await geometry()) && await chatsPanel.getByLabel('Select every chat shown').isDisabled() && await chatsPanel.getByRole('button', { name: 'Refresh' }).isDisabled());

    // Results: the bars, the line under the table and the taskbar never move; a deleted chat leaves, a refused one stays.
    await p.evaluate(() => {
      window.__cleanupFrames = [];
      const sample = () => Object.fromEntries(Array.from(document.querySelectorAll('#chat-cleanup-chats .cleanup-bar, #chat-cleanup-chats .cleanup-actions, #chat-cleanup-chats .cleanup-table, #chat-cleanup-chats .cleanup-foot, .chat-cleanup .tablist, .win-bar')).map(el => { const r = el.getBoundingClientRect(); return [el.className, [r.x, r.y, r.width, r.height]]; }));
      window.__cleanupFrames.push(sample()); window.__finishCleanup(); let left = 10;
      function frame() { window.__cleanupFrames.push(sample()); if (--left) requestAnimationFrame(frame); else window.__cleanupSampled = true; } requestAnimationFrame(frame);
    });
    await p.waitForFunction(() => window.__cleanupSampled);
    t.ok('mixed results never move a bar, the table\'s frame, the line under it or the taskbar in any sampled frame', await p.evaluate(() => __cleanupFrames.every(frame => Object.entries(__cleanupFrames[0]).every(([key, value]) => JSON.stringify(frame[key]) === JSON.stringify(value)))));
    require('fs').writeFileSync(require('path').join(process.argv[3] || require('os').tmpdir(), 'chat-cleanup-frames.json'), JSON.stringify(await p.evaluate(() => __cleanupFrames), null, 2));
    t.ok('the deleted chat leaves the table, and the ones that could not be deleted stay with the reason under their titles', await chat('old').count() === 0 && (await chat('other').locator('.cleanup-reason').innerText()).includes('started working') && (await chat('unknown').locator('.cleanup-reason').innerText()).includes('started working'));
    t.ok('the line under the table says what happened to each', /^"Chat old" was moved to the Recycle Bin\. 2 chats could not be changed; the reason is under each title\.$/.test(await foot(chatsPanel)), await foot(chatsPanel));
    t.ok('the counts follow: four chats, four in the Recycle Bin', (await win.locator('[role="tab"]').allInnerTexts()).map(s => s.replace(/\s+/g, ' ').trim()).join('|') === 'Chats 4|Recycle Bin 4');
    t.ok('staging preserves the unsent chat message', await p.evaluate(() => dinahUIState.get('dinah-draft:chat:old', null).text === 'Unsent chat message.'));
    const listings = await p.evaluate(() => __listings);
    await p.waitForTimeout(400);
    t.ok('the host is not asked again and again about the chat just deleted', await p.evaluate(() => __listings) === listings, String(await p.evaluate(() => __listings) - listings));

    // The Recycle Bin.
    await chatsPanel.getByRole('button', { name: 'Show Recycle Bin', exact: true }).click();
    await binRow('bin').waitFor();
    t.ok('Show Recycle Bin opens the other tab', await win.locator('#chat-cleanup-tab-bin').getAttribute('aria-selected') === 'true' && await binPanel.isVisible());
    t.ok('the bin says once what it is, and each chat when it is deleted for good', /stays here for 30 days/.test(await binPanel.locator('.cleanup-note').innerText()) && /1 Dec 2026/.test(await binRow('bin').innerText()), await binRow('bin').innerText());
    t.ok('a bin row discloses a program that still has the chat loaded, and keeps the long deletion text for the question', (await binRow('bin').locator('.cleanup-reason').innerText()).includes('Still loaded by Codex') && !(await binRow('bin').innerText()).includes('permanently deleted'));
    await binRow('bin').getByRole('button', { name: 'Restore Chat bin', exact: true }).click();
    await p.waitForFunction(() => !document.querySelector('#chat-cleanup-bin tr[data-chat-id="bin"]'));
    t.ok('restore takes the chat out of the bin and says it is a closed chat again', /^"Chat bin" was restored as a closed chat\.$/.test(await foot(binPanel)) && await chat('bin').count() === 1, await foot(binPanel));
    await p.evaluate(() => __emit('chat:restored', { sessionId: 'bin' }));
    t.ok('Restore returns closed metadata without provider discovery', await p.evaluate(before => !!dinahChats.ref('bin') && __bridgeCalls.filter(call => ['Sessions', 'SessionsAt', 'OpenSessionAt'].includes(call[0])).length === before, discoveryCount));
    t.ok('Restore preserves the unsent chat message', await p.evaluate(() => dinahUIState.get('dinah-draft:chat:bin', null).text === 'Retained chat message.'));
    await binRow('old').getByRole('button', { name: 'Restore Chat old', exact: true }).click();
    await binRow('old').locator('.cleanup-reason:not([hidden])').filter({ hasText: 'cannot be restored yet' }).waitFor();
    t.ok('a refused Restore is reported under its bin row, and the chat stays in the bin', await binRow('old').count() === 1 && await chat('old').count() === 0);
    await binRow('old').getByRole('button', { name: 'Delete Chat old now', exact: true }).click();
    t.ok('final deletion uses the row confirmation verbatim, and is marked as dangerous', (await confirmBox.locator('p').innerText()) === 'This chat will be permanently deleted.' && await confirmBox.getByRole('button', { name: 'Delete for Good', exact: true }).evaluate(b => b.classList.contains('btn-danger')));
    await confirmBox.getByRole('button', { name: 'Delete for Good', exact: true }).click();
    await binRow('old').locator('.cleanup-reason').filter({ hasText: 'local copy could not be erased' }).waitFor();
    t.ok('a failed final deletion stays on its bin row', await binRow('old').count() === 1);
    await binRow('old').getByRole('button', { name: 'Delete Chat old now', exact: true }).click();
    await confirmBox.getByRole('button', { name: 'Delete for Good', exact: true }).click();
    await p.waitForFunction(() => !document.querySelector('#chat-cleanup-bin tr[data-chat-id="old"]'));
    t.ok('a remaining agent copy is disclosed after local deletion', /^"Chat old" was deleted for good\. Codex does not offer deletion\.$/.test(await foot(binPanel)), await foot(binPanel));
    t.ok('final deletion forgets only that chat composer value', await p.evaluate(() => dinahUIState.get('dinah-draft:chat:old', null) === null && !!dinahUIState.get('dinah-draft:chat:bin', null)));
    await binPanel.getByRole('button', { name: 'Empty Recycle Bin…', exact: true }).click();
    t.ok('emptying the bin uses the aggregate confirmation verbatim and lists the chats', (await confirmBox.locator('p').innerText()) === 'The chats will be permanently deleted. Agent copies without a delete command remain.' && (await confirmBox.locator('li').allInnerTexts()).sort().join('|') === 'Chat bin2|Chat external');
    await confirmBox.getByRole('button', { name: 'Delete All for Good', exact: true }).click();
    await p.waitForFunction(() => !document.querySelector('#chat-cleanup-bin tr[data-chat-id="bin2"]'));
    await p.evaluate(() => { window.__holdCleanupRead = true; __emit('chat:removed', { sessionId: 'external' }); });
    await p.waitForFunction(() => !!window.__releaseCleanupRead);
    t.ok('bin actions are disabled while their authority is being refreshed', await binRow('external').getByRole('button', { name: 'Restore Chat external', exact: true }).isDisabled());
    await p.evaluate(() => { __externalRestore('external'); __releaseCleanupRead(); });
    await p.waitForFunction(() => !document.querySelector('#chat-cleanup-bin tr[data-chat-id="external"]'));
    t.ok('an external restore during an older listing removes its stale actionable bin row', await chat('external').count() === 1 && await binRow('external').count() === 0);
    t.ok('an empty bin says so, and cannot be emptied', /The Recycle Bin is empty\./.test(await binPanel.locator('.data-table-empty').innerText()) && await binPanel.getByRole('button', { name: 'Empty Recycle Bin…', exact: true }).isDisabled());
    await t.shot('chat-cleanup-bin');

    // The address, and a narrow window.
    await win.locator('#chat-cleanup-tab-chats').click();
    await win.getByLabel('Workspace', { exact: true }).selectOption('C:/Other');
    t.ok('filter state is addressable', (await p.url()).includes('workspace%3DC%253A%252FOther'));
    await t.shot('chat-cleanup-wide');
    await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForFunction(() => document.querySelector('.chat-cleanup').getBoundingClientRect().width < 400);
    t.ok('narrow, the bars fit their page and every control is in sight', await win.evaluate(root => Array.from(root.querySelectorAll('#chat-cleanup-chats .cleanup-bar, #chat-cleanup-chats .cleanup-actions, #chat-cleanup-chats .cleanup-foot')).every(el => el.scrollWidth <= el.clientWidth + 1) && Array.from(root.querySelectorAll('#chat-cleanup-chats .cleanup-bar > *, #chat-cleanup-chats .cleanup-actions button')).every(el => { const r = el.getBoundingClientRect(), box = root.getBoundingClientRect(); return r.width > 0 && r.left >= box.left - 1 && r.right <= box.right + 1; })));
    await t.shot('chat-cleanup-narrow');
    await p.reload();
    await p.locator('.chat-cleanup [aria-label="Workspace"]').waitFor();
    t.ok('reloading the page restores its filter from the URL', await p.locator('.chat-cleanup [aria-label="Workspace"]').inputValue() === 'C:/Other');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
