// Files per Message (file_limits.go, docs/plans/group-attachments.md). Each
// provider's card in Settings › Agents has the limit on the files in one
// message, its documented default and its source beside it; Devin and
// Ollama document none and start empty. A chat's box refuses the file past
// its provider's limit with the reason and the setting's name, and its
// Add › Files From This Computer… is disabled with the reason at the limit.
// A group's box uses the limit Go gives the group, the smallest among its
// member chats, with the member that set it, and disables Attach… there.
const { launch, OUT } = require('./lib/page');
const path = require('path');
const { openMenu } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__calls = []; window.__toasts = [];
      window.__pixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';
      // The limits as State gives them (fileLimitViews), Codex's set to 2.
      const where = name => '(Settings › Agents › ' + name + ' › Files per Message).';
      const limits = {
        claude: { provider: 'claude', limit: 20, default: 20, chosen: false, ceiling: 600, source: 'Anthropic\'s vision guide allows up to 600 images in one API request, and 100 for models with a 200k-token context window.', sourceUrl: 'https://platform.claude.com/docs/en/build-with-claude/vision', reason: 'Claude takes 20 files per message ' + where('Claude') },
        codex: { provider: 'codex', limit: 2, default: 20, chosen: true, ceiling: 1500, source: 'OpenAI\'s images and vision guide allows up to 1,500 images and 512 MB in one request. The default of 20 is Paul\'s own choice, lower than that ceiling.', sourceUrl: 'https://developers.openai.com/api/docs/guides/images-vision', reason: 'Codex takes 2 files per message ' + where('Codex') },
        devin: { provider: 'devin', limit: 0, default: 0, chosen: false, ceiling: 0, source: 'Devin\'s documentation gives no limit on the files in one message, so there is none unless you set one.', reason: '' },
        ollama: { provider: 'ollama', limit: 0, default: 0, chosen: false, ceiling: 0, source: 'Ollama\'s API documentation gives no limit on the images in one request, so there is none unless you set one.', reason: '' }
      };
      window.__limits = limits;
      const providers = ['devin', 'codex', 'claude', 'ollama'].map(id => ({ id, name: id[0].toUpperCase() + id.slice(1), installed: true, available: true, status: 'detected', path: 'C:/Tools/' + id + '.exe' }));
      providers[2].claude = { adapterVersion: '0.87.0', adapterInstalled: true, canInstall: true, apiKey: false };
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const members = [{ id: 'window-a', chat: 'claude:a', title: 'Reviewer', workspace: 'docs', address: 'Reviewer (docs)', state: 'open' },
        { id: 'window-b', chat: 'codex:b', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' }, person];
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [], approvals: 'devin', promptSendKey: 'enter', fileLimits: JSON.parse(JSON.stringify(limits)) }),
        AgentProviders: async () => { __calls.push(['scan']); return providers; },
        SetProviderFileLimit: async (id, n) => { __calls.push(['limit', id, n]); Object.assign(limits[id], { limit: n, chosen: true, reason: n ? id + ' takes ' + n : '' }); },
        ResetProviderFileLimit: async id => { __calls.push(['reset', id]); Object.assign(limits[id], { limit: limits[id].default, chosen: false }); },
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        // The group's limit as Go works it out: the smallest among its
        // members that have one, Docs Pass's Codex at 1.
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members,
          fileLimit: { limit: 1, provider: 'codex', member: 'Docs Pass', reason: 'Docs Pass uses Codex, which takes 1 file per message ' + where('Codex') } }],
          workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => [], BusMarkGroupRead: async () => {},
        PickAttachments: async () => { __calls.push(['pick']); return []; }
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.waitForFunction(() => __calls.some(c => c[0] === 'scan'));
    await p.evaluate(() => { const t = window.pudlToast; window.pudlToast = (text, o) => { __toasts.push(String(text)); return t && t(text, o); }; });
    const calls = kind => p.evaluate(k => __calls.filter(c => c[0] === k), kind);
    // The source opens in the browser through the runtime, which the fixture records.
    const urls = () => p.evaluate(() => window.__runtime.filter(r => r[0] === 'BrowserOpenURL').map(r => ['url', r[1]]));

    // Settings › Agents: the field on each provider's card, with its source.
    await p.evaluate(() => dinahWorkbench.open('settings'));
    const settings = p.locator('#settings');
    await settings.getByRole('tab', { name: 'Agents', exact: true }).click();
    const field = id => settings.locator('[data-provider="' + id + '"] [data-file-limit="' + id + '"]');
    await field('claude').waitFor();
    const words = async id => (await field(id).innerText());
    ok('each provider\'s card has Files per Message', (await Promise.all(['devin', 'codex', 'claude', 'ollama'].map(async id => await field(id).isVisible() && (await words(id)).includes('Files per Message')))).every(Boolean));
    const claude = await words('claude');
    ok('Claude starts at 20, with Anthropic\'s vision guide beside it', await field('claude').locator('input').inputValue() === '20' && claude.includes('Default: 20.') && claude.includes('600 images') &&
      await field('claude').locator('a').getAttribute('href') === 'https://platform.claude.com/docs/en/build-with-claude/vision' && await field('claude').getByRole('button', { name: 'Use Default' }).isDisabled(), claude);
    const codex = await words('codex');
    ok('Codex shows the limit chosen, the default of 20 and the documented ceiling of 1,500', await field('codex').locator('input').inputValue() === '2' && codex.includes('Default: 20.') && codex.includes('1,500 images') &&
      codex.includes('Paul\'s own choice') && await field('codex').getByRole('button', { name: 'Use Default' }).isEnabled(), codex);
    ok('Devin and Ollama start empty, saying their documentation gives no limit', await field('devin').locator('input').inputValue() === '' && (await words('devin')).includes('Default: no limit.') &&
      (await words('devin')).includes('gives no limit') && await field('ollama').locator('input').inputValue() === '' && (await words('ollama')).includes('gives no limit') &&
      await field('devin').locator('a').count() === 0 && await field('devin').locator('input').getAttribute('placeholder') === 'No Limit');
    await field('claude').locator('a').click();
    ok('the source opens in the browser', JSON.stringify(await urls()) === JSON.stringify([['url', 'https://platform.claude.com/docs/en/build-with-claude/vision']]));
    await field('devin').locator('input').fill('5');
    await field('devin').getByRole('button', { name: 'Save' }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'limit' && c[1] === 'devin'));
    ok('Save sets the limit', JSON.stringify((await calls('limit'))[0]) === JSON.stringify(['limit', 'devin', 5]));
    await field('ollama').locator('input').fill('2.5');
    await field('ollama').getByRole('button', { name: 'Save' }).click();
    ok('a number that is not whole is refused before saving', (await words('ollama')).includes('Enter a whole number') && (await calls('limit')).length === 1);
    await field('codex').getByRole('button', { name: 'Use Default' }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'reset'));
    ok('Use Default returns to the documented default', JSON.stringify(await calls('reset')) === JSON.stringify([['reset', 'codex']]));
    await t.shot('file-limits-settings');
    // Codex back to 2 for the chat's box.
    await p.evaluate(() => { Object.assign(__limits.codex, { limit: 2, chosen: true }); __emit('settings:changed', {}); });
    await p.waitForFunction(() => document.querySelector('[data-file-limit="codex"] input').value === '2');

    // A Codex chat's box takes two files and refuses the third.
    await p.evaluate(() => dinahWorkbench.open('chat'));
    const chat = p.locator('.win.active');
    const prompt = chat.locator('[data-view-id="prompt"]');
    await prompt.waitFor();
    const pixel = await p.evaluate(() => __pixel);
    const paste = names => prompt.evaluate((el, a) => {
      const bytes = Uint8Array.from(atob(a.pixel), c => c.charCodeAt(0));
      const dt = new DataTransfer(); a.names.forEach(n => dt.items.add(new File([bytes], n, { type: 'image/png' })));
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
    }, { names, pixel });
    await paste(['one.png', 'two.png', 'three.png']);
    const chips = chat.locator('.attachments .attachment');
    await p.waitForFunction(() => __toasts.some(t => t.includes('three.png was not attached')));
    await p.waitForFunction(() => document.querySelectorAll('.win.active .attachments .attachment').length === 2);
    ok('the file past the limit is refused with the reason and the setting', await chips.count() === 2 &&
      (await p.evaluate(() => __toasts.find(t => t.includes('three.png')))).includes('Codex takes 2 files per message (Settings › Agents › Codex › Files per Message).'));
    ok('the box says no more files can be attached', (await chat.locator('.attachments .attachment-note').allInnerTexts()).some(t => t.startsWith('No more files can be attached. Codex takes 2 files per message')));
    let menu = await openMenu(p, chat, 'add');
    let row = menu.locator('.menu-action', { hasText: 'Files From This Computer…' });
    if (!await row.count()) { await menu.locator('.menu-action', { hasText: 'Add' }).first().click(); menu = p.locator('.menu-panel:popover-open').last(); row = menu.locator('.menu-action', { hasText: 'Files From This Computer…' }); }
    const state = await row.first().evaluate(el => ({ disabled: el.disabled || el.getAttribute('aria-disabled') === 'true', text: el.innerText + ' ' + (el.title || '') }));
    ok('Add › Files From This Computer… is disabled at the limit, with the reason', state.disabled && state.text.includes('Codex takes 2 files per message'), JSON.stringify(state));
    await p.keyboard.press('Escape');
    await chips.first().getByRole('button', { name: 'Remove one.png' }).click();
    ok('removing a file lifts the limit', await chips.count() === 1 && await chat.locator('.attachments .attachment-note', { hasText: 'No more files' }).count() === 0);
    await t.shot('file-limits-chat');

    // A group's box uses the group's limit, set by its smallest member.
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    const room = p.locator('.win[data-win-kind="bus-room"]');
    const box = room.locator('textarea');
    await box.waitFor();
    await box.evaluate((el, pixel) => {
      const bytes = Uint8Array.from(atob(pixel), c => c.charCodeAt(0));
      const dt = new DataTransfer(); ['first.png', 'second.png'].forEach(n => dt.items.add(new File([bytes], n, { type: 'image/png' })));
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
    }, await p.evaluate(() => __pixel));
    await p.waitForFunction(() => __toasts.some(t => t.includes('second.png was not attached')));
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="bus-room"] .group-attachments .attachment').length === 1);
    ok('the group\'s box refuses the file past the smallest member\'s limit, saying which member set it',
      (await p.evaluate(() => __toasts.find(t => t.includes('second.png')))).includes('Docs Pass uses Codex, which takes 1 file per message (Settings › Agents › Codex › Files per Message).'));
    const attach = room.getByRole('button', { name: 'Attach…' });
    ok('Attach… is disabled at the limit, with the reason as its tip', await attach.isDisabled() && (await attach.getAttribute('title')).includes('Docs Pass uses Codex'));
    await room.locator('.group-attachments .attachment').first().getByRole('button', { name: 'Remove first.png' }).click();
    ok('removing the file enables Attach… again', await attach.isEnabled() && await attach.getAttribute('title') === 'Attach files from this computer');
    // The room alone, which t.shot does not offer.
    await room.screenshot({ path: path.join(OUT, 'file-limits-group.png') });
  } catch (err) {
    t.errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
