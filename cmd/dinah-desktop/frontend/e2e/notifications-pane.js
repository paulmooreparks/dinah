// Notifications: every toast is an entry, the
// entries filter by type and by text, Clear All dismisses the reports and
// leaves the questions, the count is of questions and errors, a repeated
// report is one entry, and a document's report reopens it.
const path = require('path');
const { launch, OUT } = require('./lib/page');
const STATE = { workspace: 'C:/W', workspaceId: 'w1', recent: ['C:/W'], approvals: 'all', devin: 'd', titles: {}, templates: [] };
(async () => {
  const t = await launch({
    viewport: { width: 1300, height: 860 },
    state: STATE,
    app: () => ({
      Notifications: async () => __store.filter(n => !n.dismissed),
      SaveNotification: async n => { const item = Object.assign({}, n, { created: new Date().toISOString(), dismissed: false }); __store.push(item); return item; },
      DismissNotification: async id => { __store.find(n => n.id === id).dismissed = true; },
      ClearNotifications: async () => { __calls.push(['clear']); __store.forEach(n => { n.dismissed = true; }); },
      GetDraft: async id => { __calls.push(['get-draft', id]); return { id, title: 'Note 09', text: '# Note 09\n', revision: 'r1', origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: 'note.md' }, baseline: { text: '# Note 09\n', revision: 'b', title: 'Note 09' }, actions: [], suggestions: [], spans: {} }; },
      PreviewDraft: async () => ({ revision: 'r1', blocks: [], problems: [] })
    }),
    init: () => { window.__store = []; window.__calls = []; },
    ready: 'workbench',
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    const entries = () => p.locator('#questions > .notice:not([hidden])');

    // Every kind of toast is an entry, newest first, and is kept.
    await p.evaluate(() => {
      dinahHost.toast('Saved "notes.md".', 'positive');
      dinahHost.toast('The workspace could not be read.', 'danger');
      dinahHost.toast('Choose a workspace first.', 'warn');
      dinahHost.toast('The export finished.', 'info');
    });
    await p.waitForFunction(() => __store.length === 4);
    ok('every toast is an entry, kept with its severity', await entries().count() === 4 && (await p.evaluate(() => __store.map(n => n.severity).join(','))) === 'success,error,warning,info');
    ok('the newest report is first', (await entries().first().textContent()).startsWith('The export finished.'));
    ok('the count is of questions and errors only', await p.textContent('#status-needs .badge') === '1');

    // The same report again is one entry, though it toasts again.
    await p.evaluate(() => dinahHost.toast('The workspace could not be read.', 'danger'));
    await p.waitForTimeout(100);
    ok('a repeated report is one entry', await p.locator('#questions .notification-report', { hasText: 'The workspace could not be read.' }).count() === 1 && await p.evaluate(() => __store.length) === 4);

    // A question stands first, and is counted.
    await p.evaluate(() => __emit('agent:permission', { sessionId: 'chat-1', key: 'q1', toolCall: null, options: [{ optionId: 'allow_once', name: 'Allow', kind: 'allow_once' }] }));
    await p.locator('#questions [data-permission-key="q1"]').waitFor();
    ok('a question stands before the reports and is counted', await entries().first().getAttribute('data-permission-key') === 'q1' && await p.textContent('#status-needs .badge') === '2');
    ok('a question is announced by a toast', await p.locator('.toast', { hasText: 'has a question for you' }).count() >= 1);

    // Filtering by kind, several at once, and by text. The kinds are boxes
    // in the Show menu; none ticked shows every kind.
    const showType = async (...types) => {
      await p.click('#needs-type');
      await p.locator('#needs-type-menu:popover-open').waitFor();
      for (const box of await p.locator('#needs-type-menu input').all()) await box.setChecked(types.includes(await box.getAttribute('value')));
      await p.keyboard.press('Escape');
    };
    await showType('error');
    ok('Errors shows only the errors', await entries().count() === 1 && (await entries().first().textContent()).includes('could not be read'));
    await showType('question');
    ok('Questions shows only the questions', await entries().count() === 1 && await entries().first().getAttribute('data-permission-key') === 'q1');
    await showType('other');
    ok('Other shows successes and information', await entries().count() === 2);
    await showType('question', 'error');
    ok('Questions and Errors together show both, and the button says so', await entries().count() === 2 && (await p.textContent('#needs-type')).includes('Questions, Errors'));
    await t.shot('notifications-filter');
    await showType();
    ok('with nothing ticked every kind shows, and the button says All', (await p.textContent('#needs-type')).includes('All'));
    await p.fill('#needs-filter', 'notes.md');
    ok('the text filter narrows the entries', await entries().count() === 1 && (await entries().first().textContent()).includes('notes.md'));
    await p.fill('#needs-filter', 'nothing like this');
    ok('a filter that matches nothing says so', await entries().count() === 0 && (await p.textContent('#needs-empty')) === 'No notifications match the filter.');
    await t.shot('notifications-filtered');
    await p.fill('#needs-filter', '');

    // A document's report reopens the document.
    await p.evaluate(() => dinahHost.documentFailed('f8036f48ac6d471e39aeb00fce5929e8', 'Saving "Note 09" failed.', 'The file changed on disk since it was opened.'));
    const doc = p.locator('#questions .notification-report', { hasText: 'Saving "Note 09" failed.' });
    await doc.waitFor();
    ok('a document failure is a toast and an entry naming the document', await p.locator('.toast', { hasText: 'Saving "Note 09" failed.' }).count() >= 1 &&
      await p.evaluate(() => __store.some(n => n.kind === 'document' && n.draftId === 'f8036f48ac6d471e39aeb00fce5929e8')));
    await t.shot('notifications-pane');
    await doc.getByRole('button', { name: 'Open the Document' }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'get-draft'));
    await p.locator('.win[data-win-kind="editor"]').waitFor();
    ok('Open the Document opens its draft in the Editor', true);

    // Clear All asks, dismisses every report and leaves the question. An
    // applet's result is dismissed in Go and its card goes when that is done.
    // The entry is added to the notifications with its own dismissal, as
    // the applet SDK adds it.
    await p.evaluate(() => {
      dinahHost.notices.add({ id: 'applet-result-inv1', type: 'success', description: 'Applet Result', className: 'applet-result-notice', dismissible: true,
        dismiss: async () => { __calls.push(['dismiss-applet', 'C:/W', 'inv1']); } });
    });
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.click('#needs-clear');
    await p.locator('#ask[open]').waitFor();
    // A danger button shows PUDL's warning glyph with its colour, as every
    // status does (PUDL 0.66.0).
    ok('the danger button of a question carries a glyph as well as its colour', await p.locator('#ask-yes').evaluate(b => {
      const s = getComputedStyle(b, '::before');
      return b.classList.contains('btn-danger') && s.content !== 'none' && s.maskImage.includes('svg') && parseFloat(s.width) > 0;
    }));
    await p.click('#ask-yes');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'clear'));
    await p.waitForTimeout(150);
    ok('Clear All leaves only the question', await entries().count() === 1 && await entries().first().getAttribute('data-permission-key') === 'q1' && await p.evaluate(() => __store.every(n => n.dismissed)));
    ok('Clear All dismisses an applet\'s result in Go and removes its card', await p.evaluate(() => __calls.some(c => c[0] === 'dismiss-applet' && c[2] === 'inv1')) && await p.locator('.applet-result-notice').count() === 0);

    // The filter is kept for the next session.
    await showType('error');
    await p.waitForTimeout(300);
    ok('the filter is kept', JSON.stringify(await p.evaluate(() => dinahUIState.get('dinah-notifications-filter'))) === '{"types":["error"],"text":""}');
  } finally { await t.close(); }

  // A failed turn's report stays under Notifications but does not put its
  // chat under Needs You; its long path and its chat's long title stay in
  // the card in a narrow panel.
  const t2 = await launch({
    viewport: { width: 1300, height: 860 },
    state: STATE,
    app: () => {
      const title = 'Design Review (15 Oct 07:44) for Parser Note 07 and Note 08';
      window.__store = [{ id: 'tf1', kind: 'turn-failure', sessionId: 'chat-9', workspace: 'C:/W', provider: 'devin', sessionTitle: title, severity: 'error', dismissed: false,
        title: 'A step failed in this turn.', body: 'Devin may have recovered; its messages show whether it did. Ended at 08:16.',
        details: ["Read file: Failed to read file 'C:\\Users\\paul\\source\\repos\\parser\\docs\\workspaces\\.devin\\skills\\document-review-edit\\SKILL.md': open C:\\Users\\paul\\source\\repos\\parser\\docs\\workspaces\\.devin\\skills"] }];
      return {
        Sessions: async () => [{ sessionId: 'chat-9', cwd: 'C:/W', title: title, provider: 'devin' }],
        Notifications: async () => __store.filter(n => !n.dismissed), SaveNotification: async n => n, DismissNotification: async () => {}
      };
    },
    ready: 'workbench',
  });
  const q = t2.page;
  try {
    const ok = t2.ok;
    await q.evaluate(() => dinahWorkbench.open('needs-you'));
    const report = q.locator('#questions .heads-up[data-notification-id="tf1"]');
    await report.waitFor();
    await q.evaluate(() => { const w = document.querySelector('#questions').closest('[data-win]'); if (w) w.style.width = '260px'; document.querySelector('#questions').style.width = '240px'; });
    await q.waitForTimeout(100);
    const spill = await report.evaluate(card => {
      const edge = card.getBoundingClientRect().right + 1;
      return Array.from(card.querySelectorAll('*')).filter(el => el.getBoundingClientRect().right > edge).map(el => el.tagName + ':' + el.textContent.slice(0, 30));
    });
    ok('a report\'s text and buttons stay inside its card', spill.length === 0);
    if (spill.length) console.log(spill.join('\n'));
    const owner = report.locator('.notices-open');
    ok('the button that opens the chat keeps the whole title in its tooltip', (await owner.getAttribute('title')).endsWith('Note 08'));
    await report.screenshot({ path: path.join(OUT, 'notifications-narrow-report.png') });
    await q.evaluate(() => dinahWorkbench.open('workspace'));
    await q.waitForTimeout(200);
    ok('a failed turn\'s report does not put its chat under Needs You', await q.evaluate(() => !Array.from(document.querySelectorAll('#workspace-chats, #sessions')).some(h => h.textContent.includes('Needs You'))) &&
      await q.evaluate(() => !!document.querySelector('a[data-session="chat-9"]')));
  } finally { await t2.close(); }
})();
