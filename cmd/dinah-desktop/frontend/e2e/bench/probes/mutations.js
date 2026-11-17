// What the page writes to the document for one action, by window and kind of write, and how many of the
// attribute writes left the value as it was. MUTATION_ACTION picks the action: chunk (the default), session, raise or key.
//   PROBE=bench/probes/mutations.js node bench/scaling.js [out] 8
module.exports = async ({ p, count, settle }) => {
  const action = process.env.MUTATION_ACTION || 'chunk';
  const rows = await p.evaluate(async action => {
    const seen = {};
    const where = n => { const el = n.nodeType === 1 ? n : n.parentElement; if (!el) return '(detached)'; const win = el.closest('.win'); const place = win ? (win.dataset.winKind || win.id || 'win') + (win.classList.contains('active') ? '*' : '') : (el.closest('.win-taskbar, [data-win-taskbar]') ? 'taskbar' : el.closest('.menubar, .topbar') ? 'menubar' : 'page'); return place; };
    const what = el => (el.nodeType === 1 ? (el.className && typeof el.className === 'string' ? '.' + el.className.split(' ').slice(0, 2).join('.') : el.tagName.toLowerCase()) : '#text');
    const observer = new MutationObserver(records => records.forEach(r => {
      let key;
      if (r.type === 'attributes') { const same = r.oldValue === r.target.getAttribute(r.attributeName); key = where(r.target) + ' | attr ' + r.attributeName + (same ? ' (same value)' : '') + ' | ' + what(r.target); }
      else if (r.type === 'characterData') key = where(r.target) + ' | text | ' + what(r.target.parentElement || r.target);
      else key = where(r.target) + ' | children +' + r.addedNodes.length + ' -' + r.removedNodes.length + ' | ' + what(r.target);
      seen[key] = (seen[key] || 0) + 1;
    }));
    observer.observe(document.documentElement, { subtree: true, childList: true, attributes: true, attributeOldValue: true, characterData: true });
    const frame = () => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
    const id = __ids[0];
    for (let i = 0; i < 5; i++) {
      if (action === 'chunk') __emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed words. ' } } });
      else if (action === 'session') { const working = i % 2 === 0; __emit('agent:session', { sessionId: id, title: 'Chat ' + id.split(':')[1], provider: 'codex', cwd: 'C:/Fixture', open: true, working, queued: [], chatState: { life: 'open', window: true, exec: { state: working ? 'working' : 'idle', why: working ? 'Working' : 'No work in progress' } } }); }
      else if (action === 'raise') { const wins = [...document.querySelectorAll('.win[data-win-kind="chat-session"]')].map(w => w.dataset.win); pudlWindows.raise(wins[i % wins.length]); }
      else if (action === 'key') { const box = document.querySelector('.win.active [data-view-id="prompt"]'); box.value += 'x'; box.dispatchEvent(new InputEvent('input', { bubbles: true, data: 'x', inputType: 'insertText' })); }
      await frame();
    }
    await new Promise(r => setTimeout(r, 400));
    observer.disconnect();
    return Object.entries(seen).sort((a, b) => b[1] - a[1]);
  }, action);
  const total = rows.reduce((a, r) => a + r[1], 0), same = rows.filter(r => r[0].includes('(same value)')).reduce((a, r) => a + r[1], 0);
  console.log('PROBE ' + count + ' chats, action ' + action + ' five times: ' + total + ' writes, ' + same + ' of them attributes set to the value they had');
  for (const [k, n] of rows.slice(0, Number(process.env.ROWS || 45))) console.log('  ' + String(n).padStart(6) + '  ' + k);
};
