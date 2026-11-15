const fs = require('fs'), path = require('path');
const { OUT } = require('./page');
module.exports = async function replayFrames(t) {
  const p = t.page;
  const key = await p.evaluate(async () => {
    const saved = go.main.App.SavedConversation, open = go.main.App.OpenSessionAt;
    const text = 'Replay answer paragraph.\n\n'.repeat(12);
    go.main.App.SavedConversation = async id => id === 'audit-replay' ? { available: true, messages: [{ role: 'you', text: 'Replay question.' }, { role: 'agent', text }] } : saved(id);
    go.main.App.OpenSessionAt = async (cwd, id) => {
      if (id !== 'audit-replay') return open(cwd, id);
      __emit('agent:updates', { sessionId: id, updates: [
        { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Replay question.' } },
        { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } }
      ] });
      await new Promise(resolve => { window.__finishAuditReplay = resolve; });
      return { sessionId: id, cwd, provider: 'codex', open: true, working: false, queued: [] };
    };
    return dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'audit-replay' });
  });
  await p.waitForFunction(key => !!window.__finishAuditReplay && [...document.querySelectorAll('[data-win="' + key + '"] .msg-text')].some(el => el.textContent.includes('Replay answer paragraph.')), key);
  const result = await p.evaluate(async key => {
    const root = document.querySelector('[data-win="' + key + '"]'), frame = () => new Promise(requestAnimationFrame);
    const box = el => { if (!el) return null; const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; };
    const activity = root.querySelector('[data-view-id=activity]');
    const sample = () => {
      const text = [...root.querySelectorAll('.msg-text')].find(el => el.textContent.includes('Replay answer paragraph.'));
      return { scroll: activity.scrollTop, extent: activity.scrollHeight, viewport: box(activity), following: activity.dataset.following, preserve: text && text.closest('.run-log').hasAttribute('data-preserve-scroll-layout'), row: box(text && text.closest('.activity-row')), shown: !!text && text.checkVisibility({ contentVisibilityAuto: true }), chrome: [box(root), box(root.querySelector('.composer')), box(document.querySelector('.win-bar'))] };
    };
    await frame(); await frame();
    const setup = []; for (let i = 0; i < 5; i++) { await frame(); setup.push(sample()); }
    const before = sample(), frames = [];
    __finishAuditReplay(); frames.push(sample());
    for (let i = 0; i < 16; i++) { await frame(); frames.push(sample()); }
    return { setup, before, frames, rows: [...root.querySelectorAll('.run-log > .activity-row')].map(el => ({ text: el.textContent.slice(0, 80), box: box(el) })) };
  }, key);
  fs.writeFileSync(path.join(OUT, 'transcript-replay-frames.json'), JSON.stringify(result, null, 2));
  const close = (a, b) => a && b && a.every((v, i) => Math.abs(v - b[i]) < 1);
  t.ok('replay swaps the same history without a hidden or estimated-height frame', result.before.shown && result.frames.every(f => f.shown && f.row && Math.abs(f.row[3] - result.before.row[3]) < 1 && f.row[1] >= Math.min(result.before.row[1], result.frames.at(-1).row[1]) - 1 && f.row[1] <= Math.max(result.before.row[1], result.frames.at(-1).row[1]) + 1), JSON.stringify(result));
  t.ok('replay leaves the window, composer and taskbar fixed', result.frames.every(f => f.chrome.every((b, i) => close(b, result.before.chrome[i]))));
};
