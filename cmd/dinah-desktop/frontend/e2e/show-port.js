// A slide show on another display, as the editing window's page sees it
// (content-host.js, openShow): with no other display the answer is null,
// and with one it is a port whose messages reach the program in the order
// posted, whose onmessage hears the show's page, whose onclose hears a
// window the person closed, and whose close ends the show after what was
// posted (docs/design/detached-slide-show.md).
const { launch } = require('./lib/page');
(async () => {
  const ws = 'C:\\W';
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    state: { workspace: ws, recent: [ws], approvals: 'devin', devin: 'd', titles: {}, templates: [] },
    app: () => {
      const id = 'd'.repeat(32);
      const draft = { id, title: 'deck.slides.md', text: '# One\n', revision: 'r1', documentType: '', origin: { kind: 'local-folder', workspaceId: 'C:\\W', sourceId: 's', path: 'decks/deck.slides.md' }, actions: ['update'], suggestions: [], spans: {}, changed: false, writeBack: 'save' };
      window.__show = { displays: 2, opened: [], posted: [], closed: [], next: 1 };
      return {
        GetDraft: async () => draft,
        ResolveWorkspaceResourceMount: async () => ({ path: '/workspaces/W/decks/deck.slides.md' }),
        OpenShow: async request => {
          window.__show.opened.push(request);
          return { id: window.__show.displays > 1 ? 'show-' + window.__show.next++ : '' };
        },
        // The first message is slow, so a later one would overtake it if the page did not keep the order.
        PostShow: async (show, message) => {
          if (!window.__show.posted.length) await new Promise(r => setTimeout(r, 150));
          window.__show.posted.push([show, message]);
        },
        CloseShow: async show => { window.__show.closed.push([show, window.__show.posted.length]); },
      };
    },
  });
  const p = t.page;
  const { ok } = t;
  try {
    await p.waitForFunction(() => window.dinahContentHost);
    const resource = await p.evaluate(async () => {
      const host = window.dinahContentHost;
      const resource = host.resourceFor(await go.main.App.GetDraft('d'.repeat(32)));
      await host.read({ resource });
      return resource;
    });
    const none = await p.evaluate(async resource => {
      window.__show.displays = 1;
      const port = await window.dinahContentHost.openShow({ application: 'slides', resource, channel: 'c' });
      window.__show.displays = 2;
      return { port, unknown: await window.dinahContentHost.openShow({ application: 'slides', resource: 'no such deck', channel: 'c' }) };
    }, resource);
    ok('one display gives no port', none.port === null, JSON.stringify(none));
    ok('a deck the page does not know gives no port', none.unknown === null, JSON.stringify(none));

    await p.evaluate(async resource => {
      const port = await window.dinahContentHost.openShow({ application: 'slides', resource, channel: 'c' });
      window.__port = port; window.__heard = []; window.__gone = 0;
      port.onmessage = m => window.__heard.push(m);
      port.onclose = () => { window.__gone++; };
      port.postMessage({ v: 1, type: 'state', text: '# One\n', slide: 0, step: 0 });
      port.postMessage({ v: 1, type: 'at', slide: 1, step: 0 });
    }, resource);
    await p.waitForTimeout(400);
    let seen = await p.evaluate(() => window.__show);
    const asked = seen.opened[seen.opened.length - 1];
    ok('the program is asked with the deck, its draft, its folder and the mode', asked.resource === resource && asked.draft === 'd'.repeat(32) && asked.folder === '/workspaces/W/decks' && (asked.mode === 'light' || asked.mode === 'dark'), JSON.stringify(asked));
    ok('messages reach the program in the order posted', seen.posted.length === 2 && JSON.parse(seen.posted[0][1]).type === 'state' && JSON.parse(seen.posted[1][1]).type === 'at' && seen.posted[0][0] === 'show-1', JSON.stringify(seen.posted));

    await t.emit('show:message', { id: 'show-1', message: JSON.stringify({ v: 1, type: 'ready' }) });
    await t.emit('show:message', { id: 'another', message: JSON.stringify({ v: 1, type: 'leave' }) });
    await t.emit('show:message', { id: 'show-1', message: 'not json' });
    await p.waitForTimeout(100);
    const heard = await p.evaluate(() => window.__heard);
    ok('the port hears its own show and no other', heard.length === 1 && heard[0].type === 'ready', JSON.stringify(heard));

    await p.evaluate(() => { window.__port.postMessage({ v: 1, type: 'end' }); window.__port.close(); window.__port.postMessage({ v: 1, type: 'at', slide: 3, step: 0 }); });
    await p.waitForTimeout(200);
    seen = await p.evaluate(() => window.__show);
    ok('closing ends the show after what was posted, and nothing is posted after', seen.closed.length === 1 && seen.closed[0][0] === 'show-1' && seen.closed[0][1] === 3 && seen.posted.length === 3, JSON.stringify([seen.closed, seen.posted.length]));
    await t.emit('show:closed', { id: 'show-1' });
    await p.waitForTimeout(100);
    ok('a show the page closed itself is not reported as closed', await p.evaluate(() => window.__gone) === 0);

    await p.evaluate(async resource => {
      const port = await window.dinahContentHost.openShow({ application: 'slides', resource, channel: 'c' });
      window.__gone = 0;
      port.onclose = () => { window.__gone++; };
    }, resource);
    await t.emit('show:closed', { id: 'show-2' });
    await p.waitForTimeout(100);
    ok('a show whose window the person closed is the port\'s onclose', await p.evaluate(() => window.__gone) === 1);

    await p.evaluate(async resource => {
      await window.dinahContentHost.openShow({ application: 'slides', resource, channel: 'c' });
      window.dinahContentHost.forget(resource);
    }, resource);
    await p.waitForTimeout(200);
    seen = await p.evaluate(() => window.__show);
    ok('a deck whose window closes takes its show with it', seen.closed.some(c => c[0] === 'show-3'), JSON.stringify(seen.closed));
  } finally { await t.close(); }
})();
