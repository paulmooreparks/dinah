// The Add button, personal templates, link chips, and context, time and
// cost per turn.
const { launch } = require('./lib/page');
const { openMenu } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const ws = 'C:\\Work\\parser';
      const skills = [{ name: 'edit-request', description: 'Turn a request into an edit request.', hint: '', source: 'workspace' }];
      let templates = [{ name: 'weekly', text: 'Summarise what changed this week.' }];
      let n = 0;
      return {
        AgentProviders: async () => ['devin', 'codex', 'claude', 'ollama'].map(id => window.__provider(id)),
        State: async () => ({ defaultProvider: 'devin', workspace: ws, recent: [ws], approvals: 'dinah', devin: 'd', catalog: [], defaults: {}, pinned: [], titles: {}, templates, usagePage: 'https://devin.example/usage' }),
        // The fixture defines SkillsForProviderAt, which the page prefers to
        // SkillsAt, so both answer with the workspace's skills.
        SkillsAt: async () => skills, SkillsForProviderAt: async () => skills,
        NewSessionAt: async () => ({ sessionId: 's' + (++n), provider: 'devin', cwd: ws, title: '', open: true, working: false, queued: [] }),
        Send: async (id, text, doc, atts) => { window.__calls.push({ id, text, atts }); },
        SaveTemplate: async (name, text) => { templates = templates.filter(t => t.name.toLowerCase() !== name.toLowerCase()).concat([{ name, text }]); return templates; },
        DeleteTemplate: async (name) => { templates = templates.filter(t => t.name !== name); return templates; },
        OpenUsage: async () => { window.__opened.push('usage'); },
        // The engine answers for a provider whose usage page is a link the person gave (the usage fact is link).
        ProviderUsage: async () => ({ status: 'external', detail: 'Devin reports account usage on its usage page.', url: true }),
        OpenProviderUsage: async () => { window.__opened.push('usage'); },
        // Go turns a file address into its path and says the system opens it
        // (document_links.go).
        ResolveDocumentLink: async (cwd, ref) => ({ ref, found: true, opens: 'system', path: decodeURIComponent(ref.replace(/^file:\/\/\//i, '')).replace(/\//g, '\\') }),
        OpenPath: async (p) => { window.__opened.push('path:' + p); return 'opened'; }
      };
    },
    // The addresses the browser is asked to open are kept in the same list
    // as the usage page and the files, in the order they were opened.
    init: () => { window.__calls = []; window.__opened = []; window.runtime.BrowserOpenURL = u => window.__opened.push(u); },
  });
  const { page: p, ok } = t;
  require('./lib/windows')(p);
  try {
    await p.waitForTimeout(500);

    // The Add button.
    const chat = p.locator('.win[data-chat-current]');
    let addMenu = await openMenu(p, chat, 'add');
    ok('Add opens its menu', (await addMenu.textContent()).includes('Files From This Computer'));
    await t.shot('add-menu');
    const [chooser] = await Promise.all([p.waitForEvent('filechooser'), addMenu.locator('.menu-action', { hasText: 'Files From This Computer' }).click()]);
    await chooser.setFiles([{ name: 'notes.txt', mimeType: 'text/plain', buffer: Buffer.from('hello') }, { name: 'shot.png', mimeType: 'image/png', buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64') }]);
    await p.waitForTimeout(300);
    ok('files chosen in the dialog become chips', (await p.$$('.win[data-chat-current] [data-view-id=attachments] .attachment')).length === 2);
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Look at ');
    addMenu = await openMenu(p, chat, 'add');
    await addMenu.locator('.menu-action', { hasText: 'Mention an Open Document or Chat' }).click();
    await p.waitForTimeout(250);
    ok('the mention entry opens the @ pop-up', await p.isVisible('.win[data-chat-current] [data-view-id=slash-pop]') && (await p.textContent('.win[data-chat-current] [data-view-id=slash-pop]')).includes('Editor') && (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === 'Look at @');
    await p.keyboard.press('Escape');
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');
    await p.evaluate(() => { document.querySelectorAll('.attachment-remove').forEach(b => b.click()); });

    // Templates.
    await p.click('.win[data-chat-current] [data-view-id=prompt]');
    await p.type('.win[data-chat-current] [data-view-id=prompt]', '/wee');
    await p.waitForTimeout(150);
    ok('"/" offers templates by name', (await p.textContent('.win[data-chat-current] [data-view-id=slash-pop]')).includes('My Templates') && (await p.textContent('.win[data-chat-current] [data-view-id=slash-pop]')).includes('weekly'));
    await t.shot('tpl-pop');
    await p.keyboard.press('Enter');
    ok('a template puts its text in the box', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === 'Summarise what changed this week.', await p.inputValue('.win[data-chat-current] [data-view-id=prompt]'));
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Check the notes against the plan.');
    const skillsMenu = await openMenu(p, chat, 'skills');
    const skillsText = await skillsMenu.textContent();
    ok('the Skills menu lists templates first', skillsText.indexOf('My Templates') >= 0 && skillsText.indexOf('My Templates') < skillsText.indexOf('Workspace Skills'), skillsText);
    await skillsMenu.locator('.menu-action', { hasText: 'Save the Prompt as a Template' }).click();
    await p.waitForTimeout(150);
    ok('saving opens the dialog with the prompt', await p.isVisible('.win[data-chat-current] [data-view-id=templates]') && (await p.inputValue('.win[data-chat-current] [data-view-id=template-text]')) === 'Check the notes against the plan.');
    await p.fill('.win[data-chat-current] [data-view-id=template-name]', 'check');
    await p.click('.win[data-chat-current] [data-view-id=template-save]');
    await p.waitForTimeout(200);
    ok('a saved template is listed', (await p.$$('.win[data-chat-current] [data-view-id=templates-list] .template-item')).length === 2);
    await t.shot('tpl-dialog');
    await p.click('.win[data-chat-current] [data-view-id=templates-list] .template-item:nth-child(1) [data-t="edit"]');
    await p.fill('.win[data-chat-current] [data-view-id=template-name]', 'weekly summary');
    await p.click('.win[data-chat-current] [data-view-id=template-save]');
    await p.waitForTimeout(200);
    const names = await p.$$eval('.win[data-chat-current] [data-view-id=templates-list] .template-name', els => els.map(e => e.textContent));
    ok('renaming replaces the old template', names.join('|') === 'check|weekly summary', names.join('|'));
    const del = '.win[data-chat-current] [data-view-id=templates-list] .template-item:nth-child(1) [data-t="delete"]';
    await p.click(del);
    ok('delete asks again first', (await p.textContent(del)) === 'Delete It?' && (await p.$$('.win[data-chat-current] [data-view-id=templates-list] .template-item')).length === 2);
    await p.click(del);
    await p.waitForTimeout(200);
    ok('the second press deletes', (await p.$$('.win[data-chat-current] [data-view-id=templates-list] .template-item')).length === 1);
    await p.keyboard.press('Escape');

    // Smart chips, and time, context and cost.
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Hello');
    await p.click('.win[data-chat-current] [data-view-id=send]');
    await p.waitForTimeout(300);
    const sid = await p.evaluate(() => window.__calls[window.__calls.length - 1].id);
    await p.evaluate((sid) => {
      const say = t => window.__emit('agent:update', { sessionId: sid, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: t } } });
      say('See TASKZ-39 and UTF-8, [the guide](https://docs.devin.ai/x) and https://example.com/a.\n\n');
      say('The notes are in file:///C:/Work/parser/notes%20today.md.\n\n');
      say('```\nhttps://example.com/in-code\n```\n');
      window.__emit('agent:update', { sessionId: sid, update: { sessionUpdate: 'usage_update', used: 170000, size: 200000, cost: { amount: 0.42, currency: 'USD' } } });
      window.__emit('agent:turn', { sessionId: sid, stopReason: 'end_turn', seconds: 130 });
    }, sid);
    await p.waitForTimeout(300);
    const chips = await p.$$eval('.win[data-chat-current] [data-view-id=logs] .smart-chip', els => els.map(e => e.textContent));
    // markdown-it shows an address it links with its escapes undone.
    ok('links and file addresses become chips', chips.join('|') === 'the guide|https://example.com/a|file:///C:/Work/parser/notes today.md', chips.join('|'));
    ok('ticket-like words and code blocks are left alone', !chips.some(c => /TASKZ-39|UTF-8|in-code/.test(c)));
    await p.click('.win[data-chat-current] [data-view-id=logs] .smart-chip >> text=the guide');
    await p.click('.win[data-chat-current] [data-view-id=logs] .smart-chip >> text=https://example.com/a');
    const opened = await p.evaluate(() => window.__opened.slice());
    ok('web chips open in the browser', opened[0] === 'https://docs.devin.ai/x' && opened[1] === 'https://example.com/a', JSON.stringify(opened));
    await p.click('.win[data-chat-current] [data-view-id=logs] .smart-chip >> text=notes today');
    await p.waitForFunction(() => window.__opened.some(o => o.startsWith('path:')));
    ok('a file chip opens the file by its Windows path', await p.evaluate(() => window.__opened.includes('path:C:\\Work\\parser\\notes today.md')));
    await p.evaluate(() => window.pudlWindows.raise(document.querySelector('.win[data-chat-current]').dataset.win));
    const turn = await p.textContent('.win[data-chat-current] [data-view-id=logs] .turn-row .turn-meta');
    ok('the turn line shows time, context and cost', turn === '2 min 10 s · context 85% full · 0.42 USD', turn);
    ok('the meter shows the context', await p.isVisible('.win[data-chat-current] [data-view-id=usage-button]') && (await p.textContent('.win[data-chat-current] [data-view-id=usage-words]')) === '85%' && await p.evaluate(() => document.querySelector('.win[data-chat-current] [data-view-id=usage-button]').classList.contains('usage-high')));
    await p.click('.win[data-chat-current] [data-view-id=usage-button]');
    await p.waitForTimeout(150);
    const detail = await p.textContent('.win[data-chat-current] [data-view-id=usage-detail]');
    ok('the usage menu has the figures', detail.includes('170,000 of 200,000 tokens (85%)') && detail.includes('2 min 10 s') && detail.includes('0.42 USD'), detail);
    await t.shot('usage');
    // On a subscription the agent's figure is what the work would cost at API
    // prices, and the chat has cost nothing beyond the plan.
    await p.evaluate(sid => {
      ['devin', 'claude', 'codex'].forEach(provider => window.__emit('agent:account', { provider, kind: 'account', label: 'Claude Max', subscription: true }));
      window.__emit('agent:update', { sessionId: sid, update: { sessionUpdate: 'usage_update', used: 171000, size: 200000, cost: { amount: 0.90, currency: 'USD' } } });
      window.__emit('agent:turn', { sessionId: sid, stopReason: 'end_turn', seconds: 20 });
    }, sid);
    await p.waitForTimeout(200);
    const lastTurn = await p.$$eval('.win[data-chat-current] [data-view-id=logs] .turn-row .turn-meta', els => els[els.length - 1].textContent);
    ok('on a subscription a turn shows no price', !/USD/.test(lastTurn), lastTurn);
    const subDetail = await p.textContent('.win[data-chat-current] [data-view-id=usage-detail]');
    ok('on a subscription Usage shows the figure at API prices, covered by the plan', subDetail.includes('At API prices') && subDetail.includes('0.90 USD') && subDetail.includes('Covered by your Claude Max plan') && !subDetail.includes('Cost so far'), subDetail);
    await p.click('.win[data-chat-current] [data-view-id=usage-menu] [data-cmd="usage-page"]');
    ok('the usage page opens', (await p.evaluate(() => window.__opened)).includes('usage'));
    await t.shot('chips');
  } finally { await t.close(); }
})();
