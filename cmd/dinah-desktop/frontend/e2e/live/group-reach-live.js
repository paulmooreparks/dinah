// group-reach.js's composer and Settings checks, against the engine itself
// rather than stubs. cmd/dinah-desktop's TestGroupReachPageAgainstEngine
// starts it with the address of a real App on a bus of its own and the
// group's name; the page's bus calls go to that App, its events come back
// as the desktop's bridge would send them, and every other call is a stub.
// It is in a folder of its own, so run.ps1, which runs the suites without
// an engine, leaves it out.
const { launch, PAGE } = require('../lib/page');
// The harness takes its screenshot folder from the second argument (lib/page.js, OUT).
const [server, , group] = process.argv.slice(2);
const REAL = ['BusOverview', 'BusGroupMessages', 'BusMarkGroupRead', 'BusSendToGroup', 'BusGroupReach', 'BusGroupAttentionSettings', 'SetBusGroupAttentionSettings'];
(async () => {
  if (!server || !group) { console.log('FAIL the check needs the engine\'s address and the group\'s name'); process.exitCode = 1; return; }
  const t = await launch({
    viewport: { width: 1280, height: 900 }, goto: false, args: REAL,
    app: real => Object.fromEntries(real.map(name => [name, (...args) => window.__engine(name, args).then(r => {
      if (r.error) throw r.error;
      return r.result;
    })])),
  });
  const { page: p, ok } = t;
  let polling = true;
  try {
    await p.exposeFunction('__engine', async (method, args) => (await fetch(server + '/call', { method: 'POST', body: JSON.stringify({ method, args }) })).json());
    await p.goto(PAGE);
    await t.waitReady();
    // The engine's events reach the page as the bridge sends them.
    (async () => {
      while (polling) {
        const events = await (await fetch(server + '/events')).json().catch(() => null);
        for (const e of events || []) await p.evaluate(([n, d]) => window.__emit(n, d), [e.name, e.data]).catch(() => {});
        await new Promise(r => setTimeout(r, 150));
      }
    })();
    await p.evaluate(name => dinahBus.openGroup(name), group);
    const room = p.locator('.win[data-win-kind="bus-room"]');
    const box = room.locator('.group-composer textarea');
    await box.waitFor();
    const lineIs = words => p.waitForFunction(w => { const l = document.querySelector('.win[data-win-kind="bus-room"] .group-reach'); return l && l.textContent === w; }, words, { timeout: 5000 }).then(() => true, () => false);
    const line = () => room.locator('.group-reach').textContent();
    ok('with no one in conversation, the engine\'s answer reads everyone, and why', await lineIs('Reaches everyone, since no one is in conversation'), await line());
    ok('and the placeholder offers everyone', await box.getAttribute('placeholder') === 'Type @ to choose who answers, or send to everyone', await box.getAttribute('placeholder'));
    await box.pressSequentially('@Alpha please look', { delay: 20 });
    ok('a typed mention of a member reaches that member, as the engine says', await lineIs('Reaches Alpha'), await line());
    const asked = (await t.calls('BusGroupReach')).pop();
    ok('the page asked the engine with the member\'s ID', asked && asked[2].length === 1 && asked[2][0] !== 'Alpha', JSON.stringify(asked));
    await box.press('Control+Enter');
    await room.locator('[data-bus-message]').last().waitFor();
    ok('the message the engine stored reached whom the line said, so no note is drawn', await p.waitForTimeout(400).then(() => room.locator('.group-reach-note').count()) === 0);
    ok('after the send, Alpha is in conversation, and the empty box says so', await lineIs('Reaches Alpha'), await line());
    ok('the placeholder says where replies go', await box.getAttribute('placeholder') === 'Reaches Alpha. Type @ to choose others', await box.getAttribute('placeholder'));
    await box.pressSequentially('@everyone hello', { delay: 20 });
    ok('a typed @everyone reads everyone in the group, as the engine marks it explicit', await lineIs('Reaches everyone in the group'), await line());
    await box.fill('');
    ok('cleared again, the line is back to the engaged member', await lineIs('Reaches Alpha'), await line());
    await t.shot('group-reach-live');

    await p.evaluate(() => dinahWorkbench.open('settings'));
    const settings = p.locator('#settings');
    await settings.getByRole('tab', { name: 'Chat', exact: true }).click();
    const limit = settings.locator('#set-attention-limit'), minutes = settings.locator('#set-attention-minutes');
    await p.waitForFunction(() => document.getElementById('set-attention-limit').value !== '');
    ok('Settings shows the engine\'s limit and minutes', await limit.inputValue() === '3' && await minutes.inputValue() === '30', (await limit.inputValue()) + ' ' + (await minutes.inputValue()));
    const ranges = await p.evaluate(() => ['set-attention-limit', 'set-attention-minutes'].map(id => { const f = document.getElementById(id); return f.min + '-' + f.max; }));
    ok('each field\'s range is the engine\'s', JSON.stringify(ranges) === JSON.stringify(['1-10', '1-1440']), JSON.stringify(ranges));
    await minutes.fill('45'); await minutes.press('Tab');
    await p.waitForFunction(() => !document.getElementById('set-attention-minutes').disabled);
    const stored = await p.evaluate(() => window.go.main.App.BusGroupAttentionSettings());
    ok('a change is stored by the engine, and the field shows it', stored.minutes === 45 && stored.limit === 3 && await minutes.inputValue() === '45', JSON.stringify(stored));
    await limit.fill('11'); await limit.press('Tab');
    await p.waitForFunction(() => !document.getElementById('set-attention-limit').disabled);
    await p.waitForTimeout(300);
    const kept = await p.evaluate(() => window.go.main.App.BusGroupAttentionSettings());
    ok('a value past the engine\'s range is refused, and the field goes back to what the engine has', kept.limit === 3 && await limit.inputValue() === '3', JSON.stringify(kept) + ' field ' + (await limit.inputValue()));
  } catch (error) {
    ok('the check ran to its end', false, String(error && error.stack || error));
  } finally {
    polling = false;
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
