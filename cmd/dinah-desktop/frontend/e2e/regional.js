// Dates and times follow the computer's regional format, which HostLocale
// reads from the operating system: a chat's times, the chat list, the
// notifications and the formats regional.js offers. A format that arrives
// after the page has drawn its times is written into them, and a tag the
// browser cannot use leaves the page's default.
const { launch } = require('./lib/page');
(async () => {
  // Each format is its own launch; the failures and page errors of the one
  // before are carried into it, so the last close reports them all.
  let run;
  const ok = (name, cond, extra) => run.ok(name, cond, extra);

  async function open(locale, held = false) {
    const prev = run;
    if (prev) await prev.browser.close();
    run = await launch({
      viewport: { width: 1280, height: 900 },
      // The format and its hold flag ride in State, where the standalone app
      // function can read them; the page ignores the extra field.
      state: { workspace: 'C:\\W', recent: ['C:\\W'], approvals: 'devin', devin: 'd', defaultProvider: 'devin', titles: {}, templates: [], regionalFixture: { locale, held } },
      app: () => {
        const ws = 'C:\\W';
        const up = (id, u) => window.__emit('agent:update', { sessionId: id, update: u });
        window.__store = [];
        return {
          HostLocale: () => new Promise(resolve => {
            window.__releaseLocale = () => resolve(window.__state.regionalFixture.locale);
            if (!window.__state.regionalFixture.held) window.__releaseLocale();
          }),
          Sessions: async () => [{ sessionId: 'old', cwd: ws, title: 'Planned yesterday', updatedAt: '2026-10-07T13:00:00' }],
          Notifications: async () => [],
          SaveNotification: async n => { const item = Object.assign({}, n, { created: new Date().toISOString(), dismissed: false }); __store.push(item); return item; },
          OpenSessionAt: async (workspace, id) => {
            up(id, { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'First question' } });
            up(id, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'First answer' } });
            up(id, { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Second question' } });
            up(id, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Second answer' } });
            return { sessionId: id, cwd: ws, title: '', open: true, working: false, queued: [] };
          },
          SessionTimes: async () => [
            { kind: 'you', at: '2026-10-07T09:05:00' }, { kind: 'agent', at: '2026-10-07T09:06:00' },
            { kind: 'you', at: '2026-10-07T21:15:00' }, { kind: 'agent', at: '2026-10-07T21:16:00' }]
        };
      },
      ready: 'workbench',
    });
    if (prev) { run.errors.push(...prev.errors); run.failures.push(...prev.failures); }
    const p = require('./lib/windows')(run.page);
    return { p, errors: run.errors };
  }
  const times = p => p.$$eval('.win[data-chat-current] [data-view-id=logs] ol:not([hidden]) > li > time.activity-time', ts => ts.map(t => t.textContent));
  async function listed(p) {
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.waitForTimeout(200);
    return p.evaluate(() => document.querySelector('#sessions .md-meta').textContent);
  }
  // A notification's time, and its time as the regional format writes it.
  async function noticeTime(p, kind) {
    await p.evaluate(() => { if (!document.querySelector('#questions time.notices-time')) dinahHost.toast('The export finished.', 'info'); dinahWorkbench.open('needs-you'); });
    await p.waitForFunction(() => document.querySelector('#questions time.notices-time'));
    return p.evaluate(tag => {
      const t = document.querySelector('#questions time.notices-time');
      return { text: t.textContent, want: new Intl.DateTimeFormat(tag, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }).format(new Date(t.dateTime)) };
    }, kind);
  }

  try {
    // A 12-hour clock in the United States format.
    {
      const { p, errors } = await open('en-US-u-hc-h12', 0);
      await p.waitForFunction(() => dinahRegional.locale === 'en-US-u-hc-h12');
      await p.click('#sessions a:has-text("Planned yesterday")');
      await p.waitForTimeout(500);
      const t = await times(p);
      ok('a chat\'s times use the 12-hour clock of the regional format', /^09:05\sAM 09:06\sAM 09:15\sPM 09:16\sPM$/.test(t.join(' ')), t.join(' '));
      const title = await p.getAttribute('.win[data-chat-current] [data-view-id=logs] ol:not([hidden]) > li > time.activity-time', 'title');
      ok('a time\'s title is the full date in the regional format', /^10\/7\/2026, 09:05:00\sAM$/.test(title), title);
      const meta = await listed(p);
      ok('the chat list says when in the regional format', /Oct 7, 01:00\sPM/.test(meta), meta);
      const n = await noticeTime(p, 'en-US-u-hc-h12');
      ok('a notification\'s time is in the regional format', n.text === n.want && /^[A-Z][a-z]{2} \d+, \d\d:\d\d\s[AP]M$/.test(n.text), JSON.stringify(n));
      const forms = await p.evaluate(() => { const d = new Date(2026, 9, 7, 21, 5, 9); return ['clock', 'time', 'dayTime', 'date', 'dateTime', 'stamp'].map(k => dinahRegional.format(d, k)); });
      ok('each form follows the regional format', /^09:05\sPM\|09:05:09\sPM\|Oct 7, 09:05\sPM\|Oct 7, 2026\|10\/7\/2026, 09:05:09\sPM\|10\/07\/2026, 09:05\sPM$/.test(forms.join('|')), forms.join('|'));
      ok('a date that is not a date is written as nothing', await p.evaluate(() => dinahRegional.format('not a date', 'clock') === ''));
      ok('no page errors in the United States format', errors.length === 0, errors.join(' | '));
      await run.shot('regional-en-us');
    }

    // A 24-hour clock in the British format, which arrives after the page
    // has drawn its times.
    {
      const { p, errors } = await open('en-GB-u-hc-h23', true);
      await p.click('#sessions a:has-text("Planned yesterday")');
      await p.waitForFunction(() => {
        const ts = document.querySelectorAll('.win[data-chat-current] [data-view-id=logs] ol:not([hidden]) > li > time.activity-time');
        return ts.length === 4 && Array.from(ts).every(t => t.textContent);
      });
      const before = await noticeTime(p, undefined);
      await p.waitForFunction(() => document.querySelector('#questions time.notices-time')?.textContent);
      ok('the format had not arrived while the times were drawn', await p.evaluate(() => dinahRegional.locale === ''));
      await p.evaluate(() => window.__releaseLocale());
      await p.waitForFunction(() => dinahRegional.locale === 'en-GB-u-hc-h23');
      await p.waitForFunction(() => Array.from(document.querySelectorAll('.win[data-chat-current] [data-view-id=logs] ol:not([hidden]) > li > time.activity-time')).map(t => t.textContent).join(' ') === '09:05 09:06 21:15 21:16');
      const t = await times(p);
      ok('a chat\'s times drawn earlier are written again in the regional format', t.join(' ') === '09:05 09:06 21:15 21:16', t.join(' '));
      await p.evaluate(() => dinahWorkbench.open('chat-browser'));
      await p.waitForFunction(() => /7 Oct, 13:00/.test(document.querySelector('#sessions .md-meta')?.textContent || ''));
      const meta = await listed(p);
      ok('the chat list is drawn again in the regional format', /7 Oct, 13:00/.test(meta), meta);
      await p.evaluate(() => dinahWorkbench.open('needs-you'));
      await p.waitForFunction(() => {
        const t = document.querySelector('#questions time.notices-time');
        return t && t.textContent === new Intl.DateTimeFormat('en-GB-u-hc-h23', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }).format(new Date(t.dateTime));
      });
      const again = await noticeTime(p, 'en-GB-u-hc-h23');
      ok('a notification drawn earlier is written again in the regional format', again.text === again.want && /^\d+ [A-Z][a-z]{2}, \d\d:\d\d$/.test(again.text), again.text + ' was ' + before.text);
      ok('no page errors in the British format', errors.length === 0, errors.join(' | '));
    }

    // A tag the browser cannot use leaves the page's default.
    {
      const { p, errors } = await open('not a tag!', 0);
      await p.waitForTimeout(300);
      ok('an unusable tag is not used', await p.evaluate(() => dinahRegional.locale === ''));
      await p.click('#sessions a:has-text("Planned yesterday")');
      await p.waitForTimeout(500);
      const t = await times(p);
      ok('the times are still written in the page\'s default', t.length === 4 && t.every(x => /09|21|9:|\d/.test(x)), t.join(' '));
      ok('no page errors with an unusable tag', errors.length === 0, errors.join(' | '));
    }
  } finally { if (run) await run.close(); }
})();
