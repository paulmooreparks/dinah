// Focus stays on its chat when the last keyboard or pointer action expires.
const fs = require('fs'), path = require('path');
const { launch, OUT } = require('./lib/page');
(async () => {
  const evidence = [];
  for (const theme of ['light', 'dark']) for (const width of [1440, 390]) {
    const t = await launch({ viewport: { width, height: 950 }, app: () => {
      let state = 'working', version = 1;
      const chats = () => Array.from({ length: 70 }, (_, i) => ({ sessionId: 'chat-' + i, title: 'Invented chat ' + i, cwd: 'C:/Fixture', provider: 'codex', open: true,
        chatState: { life: 'open', window: true, exec: { state: i === 3 ? state : i < 30 ? 'working' : 'idle', why: '' } } }));
      window.__actionState = next => { state = next; version++; __emit('agent:chatstate', { stateGen: 'action-hold', stateVer: version, chats: chats().map(c => ({ sessionId: c.sessionId, ...c.chatState })), groups: [] }); };
      return { Sessions: async () => chats(), SessionsAt: async () => chats() };
    } });
    const p = t.page, row = p.locator('#sessions [data-session-row="chat-3"]'), link = row.locator('a[data-session]');
    try {
      await p.evaluate(theme => pudlSetTheme(theme), theme);
      const key = await p.evaluate(() => dinahWorkbench.open('chat-browser'));
      await p.evaluate(key => { pudlWindows.dock(key, null); pudlWindows.snap(key, 'maximized'); }, key);
      await row.waitFor(); await p.mouse.move(1, 1);
      await p.evaluate(() => {
        const host = document.querySelector('#sessions');
        window.__lastAction = 0;
        ['keydown', 'pointermove'].forEach(name => host.addEventListener(name, () => { __lastAction = performance.now(); }, true));
        window.__actionRead = () => {
          const row = host.querySelector('[data-session-row="chat-3"]'), active = document.activeElement;
          const scrolls = []; for (let el = host; el; el = el.parentElement) scrolls.push([el.scrollTop, el.scrollLeft]);
          return { at: performance.now(), action: __lastAction, group: row.closest('.browser-group').dataset.section,
            focused: active === row.querySelector('a[data-session]'), scrolls };
        };
        window.__actionTraceStart = () => {
          __actionTrace = []; __actionRecording = true;
          function frame() { if (!__actionRecording) return; __actionTrace.push(__actionRead()); requestAnimationFrame(frame); }
          frame();
        };
        window.__actionTraceEnd = () => { __actionRecording = false; __actionTrace.push(__actionRead()); return __actionTrace; };
      });
      const group = name => p.waitForFunction(name => __actionRead().group === 'chats-' + name, name);
      async function reset() {
        await p.mouse.move(1, 1); await p.evaluate(() => { document.activeElement.blur(); __actionState('working'); });
        await group('Working'); await link.focus();
      }
      function released(trace) {
        const first = trace.find(frame => frame.group === 'chats-Chats');
        return first && first.at - first.action >= 1995 && first.at - first.action < 3500;
      }
      await reset();
      await row.evaluate(row => {
        for (let el = row.parentElement; el; el = el.parentElement) if (el.scrollHeight > el.clientHeight && /auto|scroll/.test(getComputedStyle(el).overflowY)) { el.scrollTop = 40; break; }
      });
      await p.keyboard.press('Shift');
      const baseline = await p.evaluate(() => { __actionTraceStart(); const value = __actionRead(); __actionState('idle'); return value; });
      await group('Chats');
      const focusTrace = await p.evaluate(() => __actionTraceEnd());
      t.ok('keyboard hold expires with focus still on the same chat ' + theme + width, released(focusTrace) && focusTrace.every(frame => frame.focused));
      t.ok('regrouping retains every ancestor scroll position ' + theme + width, baseline.scrolls.some(pair => pair[0] > 0) && focusTrace.every(frame => JSON.stringify(frame.scrolls) === JSON.stringify(baseline.scrolls)), JSON.stringify({ baseline, last: focusTrace.at(-1) }));
      evidence.push({ theme, width, kind: 'focused-idle', baseline, trace: focusTrace });

      await reset(); await p.keyboard.press('Shift');
      await p.evaluate(() => { __actionTraceStart(); __actionState('idle'); });
      await p.waitForTimeout(1200); await p.keyboard.press('Shift');
      await group('Chats');
      const keyTrace = await p.evaluate(() => __actionTraceEnd());
      const renewed = keyTrace.find(frame => frame.action > keyTrace[0].action);
      t.ok('a further key press renews the full two-second hold ' + theme + width, !!renewed && released(keyTrace) && keyTrace.filter(frame => frame.at - frame.action < 1995).every(frame => frame.group === 'chats-Working'));
      evidence.push({ theme, width, kind: 'key-renewal', trace: keyTrace });

      await reset(); await row.hover();
      await p.evaluate(() => { __actionTraceStart(); __actionState('idle'); });
      const box = await row.boundingBox();
      await p.waitForTimeout(1200); await p.mouse.move(box.x + 12, box.y + 12);
      await group('Chats');
      const pointerTrace = await p.evaluate(() => __actionTraceEnd());
      t.ok('pointer movement renews the hold while focus remains ' + theme + width, released(pointerTrace) && pointerTrace.filter(frame => frame.at - frame.action < 1995).every(frame => frame.group === 'chats-Working'));
      evidence.push({ theme, width, kind: 'pointer-renewal', trace: pointerTrace });

      await reset(); await row.hover(); await row.locator('[data-row-actions]').click();
      await p.waitForFunction(() => document.querySelector('#sessions [data-menu-for]'));
      await p.mouse.move(1, 1); await p.evaluate(() => __actionState('idle'));
      await p.waitForTimeout(2200);
      t.ok('an open row menu still holds beyond two seconds ' + theme + width, await row.evaluate(el => el.hasAttribute('data-menu-for') && el.closest('.browser-group').dataset.section === 'chats-Working'));
      await p.keyboard.press('Escape'); await group('Chats');
      await t.shot('list-action-hold-' + theme + '-' + width);
    } finally { await t.close(); }
  }
  fs.writeFileSync(path.join(OUT, 'list-action-hold.json'), JSON.stringify(evidence, null, 2));
})().catch(error => { console.error(error); process.exitCode = 1; });
