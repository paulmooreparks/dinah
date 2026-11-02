// The Chats pane shares the Workspace pane's indent rule (app.css). Each
// group's rows start where the group heading's words start, in Needs You,
// Working and Chats alike, and a chat's Subagents part, one level under
// its chat, starts one step further in.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 900 }, app: () => {
    const cwd = 'C:/Fixture';
    const states = { need: 'waiting', busy: 'working', calm: 'idle' };
    const titles = { need: 'Needs an Answer', busy: 'Busy Chat', calm: 'Calm Chat' };
    const info = id => ({ sessionId: id, title: titles[id], cwd, provider: 'claude', open: true, chatState: { life: 'open', window: true, exec: { state: states[id], why: '' } } });
    return { State: async () => ({ workspace: cwd, recent: [cwd], workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Fixture', executionPath: cwd, sources: [] }] }),
      Sessions: async () => Object.keys(titles).map(info), SessionsAt: async () => Object.keys(titles).map(info),
      ChatStates: async () => ({ chats: Object.keys(titles).map(id => ({ sessionId: id, bus: '', ...info(id).chatState })), groups: [], stateGen: 'i', stateVer: 1 }),
      OpenSessionAt: async (w, id) => info(id), SavedConversation: async () => ({ available: false }), ChatLead: async () => null, Team: async () => null };
  } });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.locator('#sessions .browser-group-title').filter({ hasText: 'Needs You' }).waitFor();
    await p.locator('#sessions .browser-group-title').filter({ hasText: 'Working' }).waitFor();
    // A Subagents part under the calm chat, as the pane draws one.
    await p.evaluate(() => {
      const body = [...document.querySelectorAll('#sessions .browser-group')].find(g => /^Chats/i.test(g.querySelector('.browser-group-title').textContent.trim())).querySelector('.browser-group-body');
      const part = window.dinahBrowserList.row({ title: 'Subagents', meta: ['1 from an earlier run'] }); part.classList.add('browser-row-part', 'chat-subagents-part'); body.appendChild(part);
    });
    await p.waitForTimeout(300);
    const m = await p.evaluate(() => {
      const textLeft = el => { const n = [...el.querySelectorAll('*')].concat([el]).find(x => [...x.childNodes].some(c => c.nodeType === 3 && c.textContent.trim())); const tn = [...n.childNodes].find(c => c.nodeType === 3 && c.textContent.trim()); const r = document.createRange(); r.selectNodeContents(tn); return Math.round(r.getBoundingClientRect().left); };
      const view = document.getElementById('chat-browser-view');
      return {
        heads: [...view.querySelectorAll('#sessions .browser-group-title')].map(h => [h.textContent.trim(), textLeft(h)]),
        rows: [...view.querySelectorAll('#sessions .browser-row:not(.browser-row-part) .browser-row-title')].map(r => [r.textContent.trim(), textLeft(r)]),
        part: textLeft(view.querySelector('#sessions .browser-row-part .browser-row-title')),
        overflow: view.scrollWidth > view.clientWidth + 1,
      };
    });
    const start = m.heads[0][1];
    ok('the words of every group heading start at one line', m.heads.length === 3 && m.heads.every(h => h[1] === start), JSON.stringify(m.heads));
    ok('every chat row starts where its heading\u2019s words start', m.rows.length === 3 && m.rows.every(r => r[1] === start), JSON.stringify(m.rows));
    ok('a chat\u2019s Subagents part starts one step further in', m.part === start + 16, JSON.stringify({ start, part: m.part }));
    ok('nothing in the pane runs past its edge', !m.overflow);
    await t.shot('chats-indent', { clip: await p.locator('#chat-browser-view').boundingBox() });
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
