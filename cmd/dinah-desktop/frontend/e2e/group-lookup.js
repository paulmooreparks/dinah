// Canonical group navigation, availability checks and restored draft collisions.
const { launch, PAGE } = require('./lib/page');
(async () => {
  let saved = { version: 1, values: {} }, acceptingWrites = true;
  const keyFor = name => 'item-' + Buffer.from(JSON.stringify(['bus-room', '', name])).toString('base64url');
  const canonicalKey = keyFor('stable-room'), aliasKey = keyFor('Old Title');
  const t = await launch({
    goto: false,
    app: () => {
      window.__room = { name: 'stable-room', title: 'Current Title', budget: -1, used: 0, members: [
        { id: 'writer', chat: 'codex:writer', title: 'Writer', workspace: 'Fixture', state: 'open' },
        { id: 'paul', title: 'Paul', state: 'person' }
      ] };
      window.__lookups = []; window.__heldLookups = []; window.__heldOverviews = []; window.__sentGroups = [];
      const overview = () => ({ inbox: [], unread: 0, rooms: window.__omitRoom || window.__deleted ? [] : [structuredClone(__room)], workspaces: [], links: [] });
      const result = address => {
        if (window.__lookupFailure) throw new Error('Fixture group lookup failed');
        if (window.__wrongCanonical && address === 'stable-room') return { state: 'resolved', room: Object.assign({}, structuredClone(__room), { name: 'another-room' }) };
        if (address === 'Ambiguous Title') return { state: 'ambiguous' };
        if (!window.__deleted && ['stable-room', 'Old Title', 'Current Title', __room.title].includes(address)) return { state: 'resolved', room: structuredClone(__room) };
        return { state: 'not-found' };
      };
      return {
        ReadUIState: async () => { const value = await window.__groupStorage('read'); window.__wrongCanonical = value.values['fixture-wrong-canonical'] === true; return value; },
        SaveUIState: async value => { await window.__groupStorage('write', value); },
        BusOverview: async () => {
          if (window.__overviewFailure) throw new Error('Fixture overview failed');
          if (window.__pauseOverview) return new Promise(resolve => __heldOverviews.push(() => resolve(overview())));
          return overview();
        },
        BusResolveGroup: async address => {
          __lookups.push(address);
          if (window.__pauseLookup) return new Promise((resolve, reject) => __heldLookups.push({ address, resolve, reject }));
          return result(address);
        },
        BusGroupMessages: async () => [{ id: 11, createdAt: '2026-10-31T04:00:00Z', kind: 'text', from: 'Writer', fromSession: 'codex:writer', fromPerson: false,
          room: 'stable-room', text: 'The loaded conversation stays readable.', mentions: [], attachments: [], replyTo: 0 }],
        BusMarkGroupRead: async () => {},
        BusSendToGroup: async (key, text, mentions, files) => { __sentGroups.push({ key, text, mentions, files }); }
      };
    }
  });
  const { page: p, ok } = t;
  const canonical = () => p.locator('.win[data-win="' + canonicalKey + '"]');
  const box = () => canonical().getByRole('textbox', { name: 'Message to the Group', exact: true });
  try {
    await t.context.exposeBinding('__groupStorage', (source, action, value) => {
      if (action === 'write' && acceptingWrites) saved = structuredClone(value);
      return structuredClone(saved);
    });
    await p.goto(PAGE); await t.waitReady();
    await p.evaluate(() => { __pauseOverview = true; __pauseLookup = true; return dinahWorkbench.open({ kind: 'bus-room', path: 'stable-room' }); });
    await p.waitForFunction(() => __heldLookups.length === 1 && __heldOverviews.length >= 1);
    ok('a stable-key window shows loading while its first facts are unresolved', (await canonical().innerText()).includes('Loading the group chat') && await canonical().locator('textarea').count() === 0 && !(await canonical().innerText()).includes('has been deleted'));
    await p.evaluate(() => { __pauseLookup = false; __heldLookups[0].resolve({ state: 'resolved', room: structuredClone(__room) }); });
    await box().waitFor();
    await canonical().getByText('The loaded conversation stays readable.', { exact: true }).waitFor();
    ok('authoritative resolution opens the stable group without waiting for overview', await box().isEnabled());
    await p.evaluate(() => { __pauseOverview = false; __heldOverviews.forEach(release => release()); __heldLookups = []; });
    const keys = await p.evaluate(async () => {
      const keys = [];
      for (const address of ['stable-room', 'Current Title', 'Old Title']) keys.push(await dinahBus.openGroup(address));
      return keys;
    });
    await box().waitFor();
    ok('stable, current and historical names open one canonical group window', keys.every(key => key === canonicalKey) && await p.locator('.win[data-win-kind="bus-room"]').count() === 1);
    await p.evaluate(() => dinahBus.openGroup('Old Title', { message: 11, mention: { id: 'writer' } }));
    await canonical().locator('.bus-target-message[data-bus-message="11"]').waitFor();
    await p.waitForFunction(() => document.querySelector('.group-composer textarea').value.includes('@Writer'));
    ok('canonical opening preserves the message and mention target', (await box().inputValue()).includes('@Writer'));
    await box().fill('Keep this unsent draft');

    // A successful overview that omits the known room starts a separate lookup.
    await p.evaluate(() => { __omitRoom = true; __pauseLookup = true; return dinahBus.load(); });
    await p.waitForFunction(() => __heldLookups.length === 1);
    ok('a missing overview keeps the known draft, members and conversation while checking', await box().isDisabled() && await box().inputValue() === 'Keep this unsent draft' && (await canonical().innerText()).includes('The loaded conversation stays readable.') && await canonical().locator('.member-title', { hasText: 'Writer' }).count() === 1 && !(await canonical().innerText()).includes('has been deleted'));
    await p.evaluate(() => dinahBus.load());
    await p.waitForFunction(() => __heldLookups.length === 2);
    await p.evaluate(() => { __room.title = 'Renamed During Lookup'; __pauseLookup = false; __heldLookups[1].resolve({ state: 'resolved', room: structuredClone(__room) }); });
    await p.waitForFunction(() => !document.querySelector('.group-composer textarea').disabled);
    await p.evaluate(() => __heldLookups[0].resolve({ state: 'not-found' }));
    await p.evaluate(() => new Promise(requestAnimationFrame));
    ok('an obsolete missing result cannot replace the newer canonical resolution', await box().isEnabled() && (await canonical().locator('.win-title').textContent()) === 'Renamed During Lookup' && await box().inputValue() === 'Keep this unsent draft');

    await p.evaluate(() => { __overviewFailure = true; __lookupFailure = true; return dinahBus.load().catch(() => {}); });
    await canonical().locator('.group-lookup-status').filter({ hasText: 'lookup failed' }).waitFor();
    ok('failed refresh preserves loaded content and reports lookup failure honestly', await box().isDisabled() && await box().inputValue() === 'Keep this unsent draft' && (await canonical().innerText()).includes('The loaded conversation stays readable.') && !(await canonical().innerText()).includes('has been deleted'));
    await p.evaluate(() => { __lookupFailure = false; });
    await canonical().getByRole('button', { name: 'Retry Group Lookup', exact: true }).click();
    await p.waitForFunction(() => !document.querySelector('.group-composer textarea').disabled);
    ok('authoritative retry works even while the overview is unavailable', await box().isEnabled());
    await p.evaluate(() => { __overviewFailure = false; __deleted = true; return dinahBus.load(); });
    await canonical().locator('.group-lookup-status').filter({ hasText: 'unavailable' }).waitFor();
    ok('actual deletion without a retained deletion record shows unavailable and keeps the draft', await box().isDisabled() && await box().inputValue() === 'Keep this unsent draft' && !(await canonical().innerText()).includes('has been deleted'));
    await p.evaluate(() => { __deleted = false; __omitRoom = false; return dinahBus.load(); });
    await p.waitForFunction(() => !document.querySelector('.group-composer textarea').disabled);

    const refused = await p.evaluate(async () => {
      const errors = [];
      for (const name of ['Unknown Group', 'Ambiguous Title']) {
        try { await dinahBus.openGroup(name); } catch (error) { errors.push(String(error)); }
      }
      return errors;
    });
    ok('unknown and ambiguous names do not invent group identities', refused.length === 2 && /unavailable/.test(refused[0]) && /More than one/.test(refused[1]) && await p.locator('.win[data-win-kind="bus-room"]').count() === 1, JSON.stringify(refused));
    const unknown = await p.evaluate(() => dinahWorkbench.open({ kind: 'bus-room', path: 'Unknown Group' }));
    const unknownWindow = p.locator('.win[data-win="' + unknown + '"]');
    await unknownWindow.locator('.group-lookup').filter({ hasText: 'unavailable' }).waitFor();
    ok('an unresolved restored address shows lookup failure without an empty conversation', await unknownWindow.locator('textarea, .bus-log').count() === 0 && !(await unknownWindow.innerText()).includes('has been deleted'));
    await p.evaluate(key => pudlWindows.close(key), unknown);

    // A disposed legacy window must not apply its delayed message target.
    await p.evaluate(() => { __pauseLookup = true; __heldLookups = []; });
    const obsolete = await p.evaluate(() => dinahWorkbench.open({ kind: 'bus-room', path: 'Old Title' }, JSON.stringify({ message: 999 })));
    await p.waitForFunction(() => __heldLookups.length === 1);
    await p.evaluate(key => pudlWindows.close(key), obsolete);
    await p.evaluate(() => { __pauseLookup = false; __heldLookups[0].resolve({ state: 'resolved', room: structuredClone(__room) }); });
    await p.evaluate(() => new Promise(requestAnimationFrame));
    ok('a delayed result after disposal neither reopens an alias nor changes the target', await p.locator('.win[data-win-kind="bus-room"]').count() === 1 && !new URL(p.url()).searchParams.get('view.' + canonicalKey).includes('999'));

    await p.evaluate(() => dinahUIState.flush());
    acceptingWrites = false;
    saved.values['dinah-workbench-layout'] = '?open=' + aliasKey + ',' + canonicalKey + '&view.' + aliasKey + '=' + encodeURIComponent(JSON.stringify({ message: 11, mention: { id: 'writer' } }));
    saved.values['dinah-draft:group:Old Title'] = { text: 'Recovered draft @Writer', mentioned: [{ id: 'writer', title: 'Writer' }] };
    saved.values['dinah-draft:group:stable-room'] = { text: 'Canonical draft', mentioned: [] };
    await p.goto(PAGE); await t.waitReady();
    await canonical().locator('.group-draft-choices').waitFor();
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="bus-room"]').length === 1);
    acceptingWrites = true;
    ok('restored title and canonical windows converge without overwriting distinct drafts', (await box().inputValue()).startsWith('Canonical draft') && (await canonical().getByRole('combobox', { name: 'Other saved drafts' }).innerText()).includes('Recovered draft @Writer'));
    await canonical().getByRole('button', { name: 'Use Selected Draft', exact: true }).click();
    ok('choosing a recovered draft keeps the previous draft available', await box().inputValue() === 'Recovered draft @Writer' && (await canonical().getByRole('combobox', { name: 'Other saved drafts' }).innerText()).includes('Canonical draft'));
    const preserved = await p.evaluate(() => dinahUIState.get('dinah-draft:group:stable-room'));
    ok('recovered drafts preserve mention identities', preserved.mentioned.some(m => m.id === 'writer' && m.title === 'Writer'));
    await canonical().locator('.bus-target-message[data-bus-message="11"]').waitFor();
    await p.evaluate(() => dinahUIState.flush());
    await p.goto(PAGE); await t.waitReady(); await box().waitFor();
    ok('both draft choices survive another reload under the canonical identity', await box().inputValue() === 'Recovered draft @Writer' && (await canonical().getByRole('combobox', { name: 'Other saved drafts' }).innerText()).includes('Canonical draft') && await p.locator('.win[data-win-kind="bus-room"]').count() === 1);
    await t.shot('group-lookup-recovered-drafts');
    await p.evaluate(() => dinahUIState.flush());
    acceptingWrites = false; saved.values['fixture-wrong-canonical'] = true;
    await p.goto(PAGE); await t.waitReady();
    await canonical().locator('.group-lookup').filter({ hasText: 'lookup matched another group' }).waitFor();
    ok('a previously canonical window cannot rebind by title after reload', await canonical().locator('textarea').count() === 0 && await p.locator('.win[data-win-kind="bus-room"]').count() === 1 && (await p.evaluate(() => dinahUIState.get('dinah-draft:group:stable-room'))).text === 'Recovered draft @Writer');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
