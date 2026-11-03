// Settings › Agent File Access (docs/design/agent-file-access.md), against
// a stubbed Go that keeps the contract in provider-file-reads.md. The rules
// are one list in three kinds, each drawn with only the actions Go returns:
// a protected rule has none and a shield glyph, a default has a switch, and
// a custom rule has Remove. Turning a default off and removing a rule ask
// first; a stale version reads the policy again. Adding a place previews it
// and refuses an invalid one. The log filters write the Settings address,
// and an address brings its filters back. A read arriving at the top is
// added; one arriving while the reader is further down leaves every row on
// screen where it is and is counted instead. Refused reads are acknowledged
// once shown. PUDL categories: buttons raised, fields and filters sunken,
// lists flat, and each outcome a glyph, a colour and a word.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      window.__calls = [];
      const rule = (id, kind, path, reason, extra) => Object.assign({ id, kind, pattern: path, reason, resolvedPaths: [path], examples: { windows: path }, actions: [] }, extra);
      window.__policy = { version: 1, rules: [
        rule('p1', 'protected', 'C:/Users/me/.dinah', 'Dinah\'s own settings and approvals.'),
        rule('d1', 'default', 'C:/Users/me/.ssh', 'Your SSH keys. An agent that read them could sign in to your servers as you.', { enabled: true, actions: ['disable'] }),
        rule('d2', 'default', 'C:/Users/me/AppData/Local/Browser', 'Your browser profile, with its saved passwords and cookies.', { enabled: true, actions: ['disable'], resolvedPaths: [] }),
        rule('c1', 'custom', 'D:/Secrets', 'Added by you.', { actions: ['remove'] }),
        // A network folder, verified, as Engine's policy gives it (fileaccess/network.go).
        rule('n1', 'custom', '//files/finance', 'Added by you.', { actions: ['remove'], verification: 'verified',
          networkRoots: [{ path: '//files/finance', status: 'verified', lastVerifiedAt: '2026-11-03T03:00:00Z' }] })
      ] };
      const rows = [];
      for (let i = 80; i >= 1; i--) rows.push({ id: i, at: new Date(Date.UTC(2026, 10, 2, 3, 0, 0) - (80 - i) * 60000).toISOString(), chat: i % 2 ? 'chat-a' : 'chat-b', chatTitle: i % 2 ? 'Release Notes' : 'Parser Fix', workspace: 'C:/Fixture', path: 'C:/Other/file' + i + '.txt', bytes: i % 5 ? 2048 : 0, outcome: i % 5 ? 'allowed' : 'denied', ruleID: i % 5 ? '' : 'd1', reason: i % 5 ? '' : 'Your SSH keys' });
      rows.find(r => r.id === 75).count = 3;
      Object.assign(rows.find(r => r.id === 74), { operation: 'search', visited: 120, denied: 3, errors: 1 });
      rows.find(r => r.id === 73).unverifiedRuleIDs = ['n1'];
      window.__rows = rows;
      const copy = v => JSON.parse(JSON.stringify(v));
      const stale = v => { if (v !== __policy.version) throw new Error('the policy version is stale: it changed'); };
      return {
        FileAccessPolicy: async () => copy(__policy),
        CheckFileAccessPath: async (path, ws) => {
          __calls.push(['check', path, ws]);
          if (!/^[A-Za-z]:\//.test(path)) return { valid: false, error: 'A place must be a full path, such as C:/Users/me/notes.' };
          return { valid: true, resolvedPath: path.replace(/\/+$/, ''), matchedRuleIDs: /\.ssh/.test(path) ? ['d1'] : [], insideWorkspace: path.startsWith('C:/Fixture') };
        },
        AddFileAccessEntry: async (path, v) => { stale(v); __calls.push(['add', path]); __policy.version++; __policy.rules.push(rule('c' + __policy.version, 'custom', path, 'Added by you.', { actions: ['remove'] })); return copy(__policy); },
        RemoveFileAccessEntry: async (id, v) => { stale(v); __calls.push(['remove', id]); __policy.version++; __policy.rules = __policy.rules.filter(r => r.id !== id); return copy(__policy); },
        SetFileAccessDefault: async (id, enabled, v) => { stale(v); __calls.push(['default', id, enabled]); __policy.version++; const r = __policy.rules.find(x => x.id === id); r.enabled = enabled; r.actions = [enabled ? 'disable' : 'enable']; return copy(__policy); },
        FileAccessLog: async f => {
          __calls.push(['log', JSON.stringify(f)]);
          let list = __rows.filter(r => (!f.chat || r.chat === f.chat) && (!f.outcome || r.outcome === f.outcome) && (!f.since || r.at >= f.since) && (!f.before || r.id < f.before));
          const page = list.slice(0, f.limit || 50);
          return { rows: page, nextBefore: list.length > page.length ? page[page.length - 1].id : 0, highWater: __rows[0].id, retentionStart: 1 };
        },
        AcknowledgeFileAccessLog: async (chat, through) => { __calls.push(['ack', chat, through]); return 0; }
      };
    },
  });
  const { page: p, ok } = t;
  const calls = kind => p.evaluate(k => __calls.filter(c => c[0] === k), kind);
  const address = () => p.evaluate(() => new URL(location.href).searchParams.get('view.settings'));
  try {
    await p.evaluate(() => dinahWorkbench.open('settings'));
    await p.locator('#settings-tab-file-access').click();
    const rules = p.locator('#file-access-rules');
    await rules.locator('[data-rule="c1"]').waitFor();
    await p.locator('#file-access-log tbody tr').first().waitFor();

    // The three kinds, each with only the controls Go offered it.
    const kinds = await p.evaluate(() => [...document.querySelectorAll('#file-access-rules [data-rule]')].map(r => ({
      id: r.dataset.rule, kind: r.dataset.kind, buttons: [...r.querySelectorAll('button')].map(b => b.getAttribute('role') === 'switch' ? 'switch' : b.textContent.trim()),
      shield: !!r.querySelector('.glyph') && /shield/.test(r.querySelector('.glyph').style.getPropertyValue('--glyph')), text: r.textContent })));
    const by = id => kinds.find(k => k.id === id);
    ok('a protected rule has no control and a shield glyph', by('p1').buttons.length === 0 && by('p1').shield && /Always protected/.test(by('p1').text), JSON.stringify(by('p1')));
    ok('a default has a switch and a custom rule has Remove', JSON.stringify(by('d1').buttons) === '["switch"]' && JSON.stringify(by('c1').buttons) === '["Remove"]', JSON.stringify(kinds));
    ok('a default absent from this computer is still listed and says so', /Not present on this computer/.test(by('d2').text));

    // Turning a default off asks first, keeps the row, and says it is off;
    // the switch keeps its size.
    const sw = rules.locator('[data-rule="d1"] [role="switch"]');
    const before = await sw.boundingBox();
    await sw.click();
    const dialog = p.locator('dialog[open]').filter({ hasText: 'Stop Protecting This Place?' });
    await dialog.waitFor();
    ok('nothing changes before the person confirms', (await calls('default')).length === 0);
    await dialog.getByRole('button', { name: 'Stop Protecting' }).click();
    await p.waitForFunction(() => document.querySelector('#file-access-rules [data-rule="d1"] [role="switch"]').getAttribute('aria-checked') === 'false');
    const after = await rules.locator('[data-rule="d1"] [role="switch"]').boundingBox();
    ok('turning a default off keeps the row, marks it off and keeps the switch\'s size', JSON.stringify(await calls('default')) === '[["default","d1",false]]' && /Off\. Agents can read this place/.test(await rules.locator('[data-rule="d1"]').textContent()) && Math.round(before.width) === Math.round(after.width), JSON.stringify({ before, after }));
    await rules.locator('[data-rule="d1"] [role="switch"]').click();
    await p.waitForFunction(() => document.querySelector('#file-access-rules [data-rule="d1"] [role="switch"]').getAttribute('aria-checked') === 'true');
    ok('turning it on again does not ask', (await calls('default')).length === 2);

    // Adding a place: an invalid one is refused, a valid one previewed and added.
    const field = p.locator('#file-access-path'), add = p.locator('#file-access-add');
    await field.fill('notes');
    await p.waitForFunction(() => /full path/.test(document.getElementById('file-access-preview').textContent));
    ok('an invalid place disables Add and says why', await add.isDisabled());
    await field.fill('C:/Users/me/.ssh/id_rsa');
    await p.waitForFunction(() => /already protected/.test(document.getElementById('file-access-preview').textContent));
    ok('the preview says when a place is already protected', /C:\/Users\/me\/\.ssh/.test(await p.textContent('#file-access-preview')));
    await field.fill('E:/Vault');
    await p.waitForFunction(() => !document.getElementById('file-access-add').disabled);
    const workspace = await p.evaluate(() => dinahHost.workspaceId() || '');
    ok('the preview is asked for with the window\'s workspace', (await calls('check')).every(c => c[2] === workspace));
    await add.click();
    await rules.locator('[data-rule] >> text=E:/Vault').waitFor();
    ok('a valid place is added and the field cleared', JSON.stringify(await calls('add')) === '[["add","E:/Vault"]]' && (await field.inputValue()) === '');

    // Removing asks first.
    await rules.locator('[data-rule="c1"] button', { hasText: 'Remove' }).click();
    const removeDialog = p.locator('dialog[open]').filter({ hasText: 'Remove This Place?' });
    await removeDialog.getByRole('button', { name: 'Remove' }).click();
    await rules.locator('[data-rule="c1"]').waitFor({ state: 'detached' });
    ok('removing a custom rule asks first and then removes it', JSON.stringify(await calls('remove')) === '[["remove","c1"]]');

    // A change made in another window first: the policy is read again.
    await p.evaluate(() => { __policy.version += 5; });
    await rules.locator('[data-rule="d1"] [role="switch"]').click();
    await p.locator('dialog[open]').getByRole('button', { name: 'Stop Protecting' }).click();
    await p.waitForFunction(() => /changed in another window/.test(document.getElementById('file-access-status').textContent));
    ok('a stale change says the places changed elsewhere and shows them as they are', (await rules.locator('[data-rule="d1"] [role="switch"]').getAttribute('aria-checked')) === 'true');

    // A change made elsewhere arrives as the policy's new version.
    await p.evaluate(() => { __policy.version++; __policy.rules.push({ id: 'c99', kind: 'custom', pattern: 'F:/Elsewhere', reason: 'Added by you.', resolvedPaths: ['F:/Elsewhere'], actions: ['remove'] }); __emit('file-access:policy', __policy.version); });
    await rules.locator('[data-rule="c99"]').waitFor();
    ok('a policy change made elsewhere redraws the list', true);

    // A network folder's state, as file-access:network announces it: the
    // badge and the words follow it, and nothing in the list moves.
    const net = rules.locator('[data-rule="n1"] .file-access-network-root');
    const state = () => net.evaluate(el => ({ status: el.dataset.status, lastKnown: 'lastKnown' in el.dataset, stalled: 'stalled' in el.dataset,
      badges: [...el.querySelectorAll('.badge')].map(b => b.className.replace('badge ', '') + ':' + b.textContent), words: el.querySelector('.file-access-network-detail').textContent }));
    const ruleBoxes = () => p.evaluate(() => [...document.querySelectorAll('#file-access-rules [data-rule], #file-access-rules .file-access-network-root')].map(el => { const r = el.getBoundingClientRect(); return [Math.round(r.top), Math.round(r.height), Math.round(r.width)]; }));
    let s = await state();
    ok('a verified network folder says so, and that other paths to it are refused too', s.status === 'verified' && JSON.stringify(s.badges) === '["positive:Verified"]' && /^Verified .*Other paths to this folder are refused too\.$/.test(s.words), JSON.stringify(s));
    const steady = await ruleBoxes();
    const network = (root, workers) => p.evaluate(([r, w]) => { const n = __policy.rules.find(x => x.id === 'n1'); n.networkRoots = [Object.assign({ path: '//files/finance' }, r)]; n.verification = r.status; __policy.networkWorkers = w; __emit('file-access:network', null); }, [root, workers || []]);
    await network({ status: 'unverified', reason: 'The file system did not give the folder a lasting identity.' });
    await p.waitForFunction(() => document.querySelector('[data-rule="n1"] .file-access-network-root').dataset.status === 'unverified');
    s = await state();
    ok('an unverified folder says why, and that a mapped drive letter to it is not refused', JSON.stringify(s.badges) === '["warn:Unverified"]' &&
      s.words === 'Not verified: The file system did not give the folder a lasting identity. Until it is, this rule matches the folder by its name only, so another path to the same folder, such as a mapped drive letter, is not refused.', s.words);
    ok('and the list has not moved or changed size', JSON.stringify(await ruleBoxes()) === JSON.stringify(steady), JSON.stringify({ steady, now: await ruleBoxes() }));
    await network({ status: 'unverified', reason: 'The server did not answer.', usingLastKnownIdentity: true, lastVerifiedAt: '2026-11-03T03:00:00Z' },
      [{ root: '//files/finance', roots: ['//files/finance'], ruleIDs: ['n1'], startedAt: '2026-11-03T03:10:00Z', stalled: true }]);
    await p.waitForFunction(() => 'stalled' in document.querySelector('[data-rule="n1"] .file-access-network-root').dataset);
    s = await state();
    ok('a folder on its last known identity says so plainly, with when it was verified', s.lastKnown && s.badges[0] === 'warn:Last Known Identity' && /keeps the identity last verified /.test(s.words), JSON.stringify(s));
    ok('a stalled check is shown on its rule, with when it started', s.stalled && s.badges[1] === 'warn:Check Stalled' && / The check of this folder has not finished since .*.$/.test(s.words) && /^Not verified just now/.test(s.words), JSON.stringify(s));
    ok('still nothing has moved or changed size', JSON.stringify(await ruleBoxes()) === JSON.stringify(steady), JSON.stringify({ steady, now: await ruleBoxes() }));
    ok('the whole text is in the tooltip, whatever the two lines show', (await net.locator('.file-access-network-detail').getAttribute('title')) === s.words);

    // The log: newest first, a page at a time, refused reads acknowledged.
    const firstPage = await p.evaluate(() => [...document.querySelectorAll('#file-access-log tbody tr')].map(r => +r.dataset.read));
    ok('the log shows the newest page first', firstPage.length === 50 && firstPage[0] === 80 && firstPage[49] === 31, JSON.stringify([firstPage.length, firstPage[0], firstPage[49]]));
    await p.locator('#file-access-older').click();
    await p.waitForFunction(() => document.querySelectorAll('#file-access-log tbody tr').length === 80);
    ok('Load Older reads the next page and then goes away', await p.locator('#file-access-older').isHidden());
    const acks = await calls('ack');
    ok('refused reads are acknowledged for each chat through its newest one shown', acks.some(a => a[1] === 'chat-a' && a[2] === 75) && acks.some(a => a[1] === 'chat-b' && a[2] === 80), JSON.stringify(acks));
    // PUDL's status badge draws its kind's glyph before the word.
    const outcome = await p.evaluate(() => { const look = b => [b.className, b.textContent, getComputedStyle(b, '::before').getPropertyValue('--glyph').trim(), getComputedStyle(b).backgroundColor, b.querySelectorAll('.glyph').length]; return { denied: look(document.querySelector('#file-access-log tr[data-read="80"] .badge')), allowed: look(document.querySelector('#file-access-log tr[data-read="79"] .badge')) }; });
    const summed = await p.evaluate(() => { const r = document.querySelector('#file-access-log tr[data-read="75"]'); return [r.querySelector('.badge').textContent, r.querySelector('.file-access-place').title]; });
    const operation = await p.evaluate(() => (document.querySelector('#file-access-log tr[data-read="74"] .file-access-operation') || {}).textContent || '');
    ok('a search shows how many places it looked at, refused and failed', operation === 'Search: 120 places looked at, 3 refused, 1 failed.', operation);
    ok('a read judged while a network folder of its rule was unverified says so', (await p.locator('#file-access-log [data-read="73"] .file-access-unverified').textContent()) === 'Judged while a network folder in its rule was not verified, so the rule matched that folder by name only.');
    ok('a refused row that sums several says how many, and its path is the latest', /^Refused 3 times/.test(summed[0]) && /latest of 3/.test(summed[1]), JSON.stringify(summed));
    ok('each outcome is one PUDL glyph, a colour and a word', /danger/.test(outcome.denied[0]) && /Refused/.test(outcome.denied[1]) && /positive/.test(outcome.allowed[0]) && /Read, 2\.0 KB/.test(outcome.allowed[1]) && outcome.denied[2] && outcome.allowed[2] && outcome.denied[2] !== outcome.allowed[2] && outcome.denied[3] !== outcome.allowed[3] && outcome.denied[4] === 0 && outcome.allowed[4] === 0, JSON.stringify(outcome));

    // Filters write the address, and an address brings them back.
    await p.selectOption('#file-access-outcome', 'denied');
    await p.waitForFunction(() => [...document.querySelectorAll('#file-access-log tbody tr')].length === 16);
    ok('choosing refused reads only shows them and writes the address', (await address()) === 'file-access?outcome=denied', await address());
    await p.selectOption('#file-access-chat', 'chat-a');
    await p.waitForFunction(() => document.querySelectorAll('#file-access-log tbody tr').length === 8);
    ok('choosing a chat adds it to the address', (await address()) === 'file-access?chat=chat-a&outcome=denied', await address());
    await p.evaluate(() => dinahWorkbench.setState('settings', 'file-access?chat=chat-b&since=week'));
    await p.waitForFunction(() => document.getElementById('file-access-chat').value === 'chat-b' && document.getElementById('file-access-since').value === 'week' && document.getElementById('file-access-outcome').value === '');
    const restored = JSON.parse((await calls('log')).pop()[1]);
    ok('an address brings its filters back and reads the log with them', restored.chat === 'chat-b' && !restored.outcome && !!restored.since, JSON.stringify(restored));
    await p.evaluate(() => dinahWorkbench.setState('settings', 'file-access'));
    await p.waitForFunction(() => document.getElementById('file-access-chat').value === '' && document.querySelectorAll('#file-access-log tbody tr').length === 50);

    // A read arriving at the top is added there.
    const newest = id => ({ id, at: new Date().toISOString(), chat: 'chat-a', chatTitle: 'Release Notes', workspace: 'C:/Fixture', path: 'C:/Other/new' + id + '.txt', bytes: 10, outcome: 'allowed' });
    await p.locator('#settings-file-access').evaluate(el => { el.scrollTop = 0; });
    await p.evaluate(r => { __rows.unshift(r); __emit('file-access:read', null); }, newest(81));
    await p.waitForFunction(() => document.querySelector('#file-access-log tbody tr').dataset.read === '81');
    ok('a read arriving while the reader is at the top is added there', await p.locator('#file-access-newer').isHidden());
    // A read arriving while the reader is further down moves nothing.
    await p.locator('#file-access-log tbody tr').nth(30).scrollIntoViewIfNeeded();
    await p.locator('#settings-file-access').evaluate(el => { el.scrollTop += 200; });
    const still = () => p.evaluate(() => [...document.querySelectorAll('#file-access-log tbody tr')].slice(20, 40).map(r => Math.round(r.getBoundingClientRect().top)));
    const boxes = await still();
    await p.evaluate(r => { __rows.unshift(r); __emit('file-access:read', null); }, newest(82));
    await p.waitForFunction(() => !document.getElementById('file-access-newer').hidden);
    ok('a read arriving further down moves no row and is counted', JSON.stringify(await still()) === JSON.stringify(boxes) && /1 Newer Read/.test(await p.textContent('#file-access-newer')), JSON.stringify({ boxes, after: await still() }));
    await p.locator('#file-access-newer').click();
    await p.waitForFunction(() => document.querySelector('#file-access-log tbody tr').dataset.read === '82');
    ok('the count shows the newer reads', await p.locator('#file-access-newer').isHidden());

    // PUDL categories as drawn.
    const looks = await p.evaluate(() => {
      const cs = el => getComputedStyle(el);
      return { add: document.getElementById('file-access-add').classList.contains('btn'), field: document.getElementById('file-access-path').classList.contains('form-input'),
        filters: ['file-access-chat', 'file-access-since', 'file-access-outcome'].every(id => document.getElementById(id).classList.contains('form-select')),
        flatRules: cs(document.getElementById('file-access-rules')).boxShadow === 'none' && cs(document.querySelector('#file-access-rules [data-rule]')).boxShadow === 'none',
        table: document.getElementById('file-access-log').classList.contains('data-table') };
    });
    ok('buttons are PUDL buttons, the field and filters sunken controls, and the lists flat', Object.values(looks).every(Boolean), JSON.stringify(looks));
    await t.shot('agent-file-access-narrow');
    await p.evaluate(() => pudlWindows.snap('settings', 'maximized'));
    await p.locator('#settings-file-access').evaluate(el => { el.scrollTop = 0; });
    await p.waitForTimeout(300);
    await t.shot('agent-file-access-rules');
    await p.locator('#file-access-log').scrollIntoViewIfNeeded();
    await t.shot('agent-file-access-log');
  } catch (error) {
    ok('the suite ran', false, String(error && error.stack || error));
  } finally {
    await t.close();
  }
})();
