'use strict';
// The receiving binding for Quality's frozen Effort checks
// (docs/review/effort-independent-freeze-2026-11-16.md on quality/effort-control).
// It opens the real page with the shipped PUDL command, menu and toolbar
// scripts, seeds invented chats through the stubbed Go side, drives the page
// with real pointer and keyboard input, and reports what it observes. It never
// computes an outcome from the fixture's expectations.
//
// Two setups are explicit in each case's evidence (setup):
// - At 390 pixels the composer's toolbar folds the Agent group into a menu.
//   The button and keyboard scenarios need the Effort button itself, so for
//   them the binding removes the toolbar's data-toolbar-fold attribute, which
//   chat-applet.js sets to let the toolbar fold, and the toolbar stays
//   expanded. The fold scenario uses the real folded layout.
// - At 1440 pixels the fold scenario narrows the chat window itself, by an
//   inline width on its element, to make the toolbar fold, and restores it.
//
//   QUALITY_EFFORT_BINDING=<this file> pwsh scripts/with-test-lock.ps1 node --test internal/qualityeffort/acceptance.test.cjs
const fs = require('fs'), os = require('os'), path = require('path'), { execFileSync } = require('child_process');
const { launch, EDGE } = require('./page');

const LEVELS = {
  one: { provider: 'ollama', title: 'One Level', option: 'think', current: 'true', list: [['true', 'On']] },
  three: { provider: 'codex', title: 'Three Levels', option: 'reasoning_effort', current: 'medium', list: [['low', 'low'], ['medium', 'medium'], ['high', 'High Effort']] },
  five: { provider: 'claude', title: 'Five Levels', option: 'effort', current: 'High', list: [['Low', 'Low'], ['Medium', 'Medium'], ['High', 'High'], ['xHigh', 'Extra High Reasoning'], ['max', 'Max']] },
  devin: { provider: 'devin', title: 'Devin Levels', option: 'thinking', current: 'Normal', list: [['Low', 'Low'], ['Normal', 'Normal'], ['High', 'High']] },
  local: { provider: 'ollama', title: 'Local Levels', option: 'think', current: 'false', list: [['false', 'Off'], ['true', 'On']] },
  none: { provider: 'codex', title: 'No Setting' },
  other: { provider: 'codex', title: 'Other Chat', option: 'reasoning_effort', current: 'low', list: [['low', 'low'], ['medium', 'medium'], ['high', 'high']] }
};

function sourceIdentity() {
  try { return execFileSync('git', ['rev-parse', 'HEAD'], { cwd: __dirname, windowsHide: true }).toString().trim(); } catch (e) { return 'unknown: ' + e.message; }
}

async function open({ scenario, theme, width }) {
  const artifactDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'effort-receiving-' + scenario + '-' + theme + '-' + width + '-'));
  const t = await launch({ viewport: { width, height: 1000 }, app: (levels) => {
    const ws = 'C:/Fixture';
    const model = { id: 'model', category: 'model', name: 'Model', type: 'select', currentValue: 'one', options: [{ value: 'one', name: 'Fixture Model' }] };
    window.__chats = {};
    Object.keys(levels).forEach(id => {
      const l = levels[id], options = [model];
      if (l.list) options.push({ id: l.option, category: 'thought_level', name: 'Reasoning effort', type: 'select', currentValue: l.current,
        options: l.list.map(([value, name]) => ({ value, name, description: 'A provider description of ' + name + ' that the panel must not show.' })) });
      window.__chats[id] = { provider: l.provider, title: l.title, options };
    });
    window.__requests = [];
    window.__optionPayloads = [];
    const info = id => ({ sessionId: id, cwd: ws, provider: __chats[id].provider, title: __chats[id].title, open: true, working: false, queued: [] });
    return {
      State: async () => ({ workspace: ws, recent: [ws], defaultProvider: 'codex', catalog: [model], defaults: {}, pinned: ['one'] }),
      AgentProviders: async () => ['codex', 'claude', 'ollama', 'devin'].map(id => window.__provider(id, { default: id === 'codex' })),
      Sessions: async () => Object.keys(__chats).map(id => ({ sessionId: id, cwd: ws, provider: __chats[id].provider, title: __chats[id].title })),
      OpenSessionAt: async (w, id) => { setTimeout(() => { __optionPayloads.push({ sessionId: id, options: __chats[id].options }); __emit('agent:options', { sessionId: id, options: __chats[id].options }); }, 0); return info(id); },
      SetSessionOption: async (id, option, value) => {
        __requests.push({ chatId: id, option, value });
        __chats[id].options = __chats[id].options.map(o => o.id === option ? { ...o, currentValue: value } : o);
        return __chats[id].options;
      }
    };
  }, args: LEVELS });
  const p = t.page, errors = [], focusTrail = [], setup = [];
  p.on('pageerror', e => errors.push(String(e && e.message || e)));
  /* The theme is set as the appearance suite sets it, through PUDL's own
     setter; a theme that does not take stops the case rather than passing it. */
  await p.evaluate(th => pudlSetTheme(th), theme);
  await p.waitForFunction(th => document.documentElement.dataset.theme === th, theme, { timeout: 5000 }).catch(() => { throw new Error('the page did not take the ' + theme + ' theme'); });
  await p.evaluate(() => { window.__focusTrail = []; document.addEventListener('focusin', e => { const a = e.target; window.__focusTrail.push(a.dataset.level || a.dataset.command || a.getAttribute('aria-label') || a.tagName); }, true); });

  const frame = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  const settle = async () => { await p.evaluate(() => document.fonts.ready); await frame(); await frame(); };
  const chat = p.locator('.win[data-chat-current]');
  const effort = () => chat.locator('[data-command="effort"]');
  const openPanel = () => p.locator('.menu-panel:popover-open .effort-panel');
  const agentFold = () => chat.locator('.composer-toolbar .menu-btn[aria-label="Agent"]');
  const focused = () => p.evaluate(() => { const a = document.activeElement; return a ? (a.dataset.level || a.dataset.command || a.getAttribute('aria-label') || a.tagName) : ''; });
  const requests = () => p.evaluate(() => window.__requests.slice());
  const current = id => p.evaluate(([id, opt]) => { const o = __chats[id].options.find(x => x.id === opt); return o ? o.currentValue : null; }, [id, LEVELS[id].option]);
  const offered = id => (LEVELS[id].list || []).map(([value, name]) => ({ value, name }));
  async function openChat(id) {
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.locator('#sessions a').filter({ hasText: LEVELS[id].title }).click();
    if ((await chat.getAttribute('data-win-mode')) !== 'maximized') await chat.locator('[data-win-action="maximize"]').click().catch(() => {});
    await p.waitForFunction(() => { const m = document.querySelector('.win[data-chat-current] [data-command="model"] .toolbar-words'); return !m || m.textContent === 'Fixture Model'; });
    await settle();
  }
  /* The toolbar's uncollapsed presentation: no folding, and every group at
     stage 1, where each control shows its glyph and its words. */
  async function expandToolbar() {
    if (width < 640) { await chat.evaluate(r => { r.style.width = '1200px'; r.style.maxWidth = 'none'; }); setup.push('chat window widened to 1200px by an inline width, wider than the ' + width + ' viewport, to give the toolbar room'); }
    const was = await chat.evaluate(r => {
      const bar = r.querySelector('.composer-toolbar'), had = bar.hasAttribute('data-toolbar-fold');
      bar.removeAttribute('data-toolbar-fold');
      bar.querySelectorAll('.toolbar-group').forEach(g => g.setAttribute('data-toolbar-stage', '1'));
      return had;
    });
    if (was) setup.push('toolbar fold attribute removed and every group set to stage 1 (glyph and words), so the Effort button shows uncollapsed at ' + width);
    await settle();
  }
  async function seed(id, value) {
    await p.evaluate(([id, opt, value]) => { __chats[id].options = __chats[id].options.map(o => o.id === opt ? { ...o, currentValue: value } : o); __optionPayloads.push({ sessionId: id, options: __chats[id].options }); __emit('agent:options', { sessionId: id, options: __chats[id].options }); }, [id, LEVELS[id].option, value]);
    await settle();
  }
  async function showPanel() {
    if (await effort().isVisible()) await effort().click(); else await agentFold().click();
    await openPanel().waitFor(); await settle();
  }
  async function choose(value) {
    await showPanel();
    await openPanel().locator('[data-level="' + value + '"]').click();
    await settle();
  }
  const returnControl = async () => (await effort().isVisible()) ? 'effort' : 'Agent';
  const button = () => effort().evaluate(b => { const m = b.querySelector('.effort-mark'), r = b.getBoundingClientRect(); return { visibleLabel: b.querySelector('.toolbar-words').textContent, accessibleDescription: b.getAttribute('aria-label') || '', cellCount: m.children.length, filledCellCount: m.querySelectorAll('[data-on]').length, markWidth: m.getBoundingClientRect().width, buttonWidth: r.width }; });
  const panelState = () => openPanel().evaluate(el => {
    const visible = n => n.getClientRects().length && getComputedStyle(n).visibility !== 'hidden';
    const r = el.getBoundingClientRect();
    /* Inside a folded toolbar's menu, the menu heads the panel with the
       command's name, and the panel's own heading is hidden. */
    const own = el.querySelector('.md-section-label'), box = el.parentElement, above = box && box.classList.contains('menu-command-panel') ? box.previousElementSibling : null;
    const heading = own && visible(own) ? [] : above && above.classList.contains('md-section-label') && visible(above) ? [above.textContent] : [];
    return {
      visibleText: heading.concat(Array.from(el.querySelectorAll('.md-section-label, button')).filter(visible).map(n => n.textContent)),
      headingsAbove: Array.from(el.closest('.menu-panel').querySelectorAll('.md-section-label')).filter(n => visible(n) && !el.contains(n)).map(n => n.textContent),
      choiceValues: Array.from(el.querySelectorAll('[data-level]')).map(b => b.dataset.level),
      pressedValues: Array.from(el.querySelectorAll('[aria-pressed="true"]')).map(b => b.dataset.level),
      panelRect: { x: r.x, y: r.y, width: r.width, height: r.height },
      descriptionElementCount: Array.from(el.querySelectorAll('*')).filter(n => visible(n) && /description/i.test(n.textContent) && !n.children.length).length + el.querySelectorAll('[class*="desc"], [class*="detail"], [class*="note"]').length,
      runningSentenceCount: (el.textContent.match(/turn|request to the model/gi) || []).length
    };
  });
  const rects = () => chat.evaluate(root => {
    const out = {};
    root.querySelectorAll('.composer-toolbar button').forEach(b => { if (!b.getClientRects().length) return; const k = b.dataset.command || b.getAttribute('aria-label'); if (!k || out[k]) return; const r = b.getBoundingClientRect(); out[k] = { x: r.x, y: r.y, width: r.width, height: r.height }; });
    const ta = root.querySelector('textarea.composer-input').getBoundingClientRect(); out.composer = { x: ta.x, y: ta.y, width: ta.width, height: ta.height };
    return out;
  });
  const working = (id, on) => p.evaluate(([id, on]) => __emit('agent:session', { sessionId: id, cwd: 'C:/Fixture', provider: __chats[id].provider, title: __chats[id].title, open: true, working: on, queued: [], chatState: { life: 'open', window: true, exec: { state: on ? 'working' : 'idle', why: on ? 'Turn in progress' : 'No work in progress' } } }), [id, on]).then(settle);
  const workingLine = () => chat.evaluate(root => { const n = root.querySelector('.live-work-note'); return n && !n.hidden && n.getClientRects().length ? n.textContent : ''; });
  const timingElsewhere = () => p.evaluate(() => {
    const note = document.querySelector('.win[data-chat-current] .live-work-note');
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT), found = [];
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const el = n.parentElement; if (!el || (note && note.contains(el)) || !el.getClientRects().length) continue;
      if (/next turn|turn already running|next request to the model/i.test(n.textContent)) found.push(n.textContent.trim());
    }
    return found.join(' | ');
  });
  async function reachEffortByKeyboard() {
    await chat.locator('textarea.composer-input').click();
    const inBar = () => p.evaluate(() => !!(document.activeElement && document.activeElement.closest('.win[data-chat-current] .composer-toolbar')));
    for (let i = 0; i < 6 && !(await inBar()); i++) await p.keyboard.press('Shift+Tab');
    for (let i = 0; i < 10 && (await focused()) !== 'effort'; i++) {
      const before = await p.evaluate(() => { const a = document.activeElement, all = Array.from(document.querySelectorAll('.win[data-chat-current] .composer-toolbar button')).filter(b => b.getClientRects().length); return all.indexOf(a) < all.findIndex(b => b.dataset.command === 'effort'); });
      await p.keyboard.press(before ? 'ArrowRight' : 'ArrowLeft');
    }
  }
  const panelOpen = async () => (await p.locator('.menu-panel:popover-open .effort-panel').count()) > 0;

  async function run() {
    const e = { scenario, theme, width, artifactDirectory, browserErrors: errors };
    if (scenario === 'button') {
      e.levelCounts = []; e.samples = [];
      setup.push('button samples: each level put in effect by an agent:options update');
      for (const id of ['one', 'three', 'five']) {
        await openChat(id); await expandToolbar();
        e.levelCounts.push(offered(id).length);
        for (let i = 0; i < offered(id).length; i++) {
          /* Each level is put in effect by the chat's own options update, as the
             engine reports a change; choosing by pointer and keyboard is
             checked in the pointer and keyboard scenarios. */
          if ((await current(id)) !== offered(id)[i].value) await seed(id, offered(id)[i].value);
          const b = await button(), now = await current(id);
          e.samples.push(Object.assign({ chatId: id, offered: offered(id), selectedIndex: offered(id).findIndex(o => o.value === now), thinkingButtonCount: await chat.locator('[data-command="thinking"]').count() }, b));
        }
        await p.screenshot({ path: path.join(artifactDirectory, 'button-' + id + '.png') });
      }
      await openChat('none');
      e.noSetting = { effortButtonCount: await chat.locator('[data-command="effort"]:visible').count() };
      /* The folded presentation: at 1440 the chat window is narrowed to fold its toolbar. */
      if (width >= 640) { await chat.evaluate(r => { r.style.width = '400px'; r.style.maxWidth = '400px'; }); setup.push('no-setting chat narrowed to 400px to look in its folded Agent menu'); await settle(); }
      await agentFold().click(); await frame();
      e.noSetting.effortMenuEntryCount = await p.locator('.menu-panel:popover-open .effort-panel').count();
      await p.keyboard.press('Escape');
      if (width >= 640) await chat.evaluate(r => { r.style.width = ''; r.style.maxWidth = ''; });
    }
    if (scenario === 'panel') {
      e.cases = [];
      for (const id of ['one', 'three', 'five']) {
        await openChat(id);
        const c = { chatId: id, offered: offered(id), states: [] };
        for (const running of [false, true]) {
          await working(id, running);
          for (const o of offered(id)) {
            if ((await current(id)) !== o.value) await choose(o.value);
            await showPanel();
            c.states.push(Object.assign({ running, currentValue: await current(id) }, await panelState()));
            await p.keyboard.press('Escape'); await settle();
          }
        }
        await working(id, false);
        e.cases.push(c);
      }
      await p.screenshot({ path: path.join(artifactDirectory, 'panel.png') });
    }
    if (scenario === 'pointer') {
      e.journeys = [];
      for (const id of ['one', 'three', 'five']) {
        await openChat(id);
        const before = await current(id), list = offered(id);
        const chosen = list.length > 1 ? list.find(o => o.value !== before).value : before;
        const expectedReturnControl = await returnControl();
        const otherChatBefore = await current('other');
        const beforeRects = await rects(), start = (await requests()).length;
        await showPanel();
        await p.screenshot({ path: path.join(artifactDirectory, 'pointer-open-' + id + '.png') });
        await openPanel().locator('[data-level="' + chosen + '"]').click(); await settle();
        const afterRects = await rects();
        const shown = await effort().evaluate(b => b.getAttribute('aria-label') || '');
        const rendered = list.find(o => shown.includes(', ' + o.name + ', level')) || {};
        e.journeys.push({ chatId: id, beforeValue: before, chosenValue: chosen, expectedReturnControl, focusAfter: await focused(), panelOpenAfter: await panelOpen(),
          renderedCurrentValue: rendered.value, authoritativeCurrentValue: await current(id), requests: (await requests()).slice(start), otherChatBefore, otherChatAfter: await current('other'), beforeRects, afterRects });
      }
    }
    if (scenario === 'keyboard') {
      e.openKeys = ['Enter', 'Space']; e.journeys = [];
      await openChat('three'); await expandToolbar();
      const list = offered('three').map(o => o.value);
      for (const key of e.openKeys) {
        await seed('three', 'medium');
        const j = { openKey: key, currentValue: await current('three'), navigationKeys: ['ArrowLeft', 'ArrowRight', 'Home', 'End'], expectedReturnControl: 'effort' };
        let at = list.indexOf(j.currentValue);
        j.expectedFocusAfterKeys = j.navigationKeys.map(k => { at = k === 'ArrowLeft' ? Math.max(0, at - 1) : k === 'ArrowRight' ? Math.min(list.length - 1, at + 1) : k === 'Home' ? 0 : list.length - 1; return list[at]; });
        await reachEffortByKeyboard();
        const start = (await requests()).length;
        await p.keyboard.press(key === 'Space' ? ' ' : key); await openPanel().waitFor(); await settle();
        j.focusOnOpen = await focused();
        j.focusAfterKeys = [];
        for (const k of j.navigationKeys) { await p.keyboard.press(k); j.focusAfterKeys.push(await focused()); }
        j.requestsBeforeCommit = (await requests()).slice(start);
        j.lastFocusedValue = await focused();
        await p.keyboard.press(key === 'Space' ? ' ' : key); await settle();
        j.chosenValue = await current('three');
        j.requestsAfterCommit = (await requests()).slice(start);
        j.focusAfterCommit = await focused(); j.panelOpenAfterCommit = await panelOpen();
        await p.keyboard.press(key === 'Space' ? ' ' : key); await openPanel().waitFor(); await settle();
        j.valueBeforeEscape = await current('three');
        const escStart = (await requests()).length;
        await p.keyboard.press('ArrowLeft'); await p.keyboard.press('Escape'); await settle();
        j.requestsDuringEscape = (await requests()).slice(escStart);
        j.valueAfterEscape = await current('three'); j.panelOpenAfterEscape = await panelOpen(); j.focusAfterEscape = await focused();
        e.journeys.push(j);
      }
    }
    if (scenario === 'fold') {
      await openChat('five');
      e.offered = offered('five'); e.currentValue = await current('five'); e.agentMenuControl = 'Agent';
      if (width >= 640) { await chat.evaluate(r => { r.dataset.receivingWidth = r.style.width; r.style.width = '400px'; r.style.maxWidth = '400px'; }); setup.push('chat window narrowed to 400px by an inline width to fold its toolbar'); await settle(); }
      await agentFold().click(); await openPanel().waitFor(); await settle();
      const fp = await panelState();
      const headings = await p.evaluate(() => Array.from(document.querySelectorAll('.menu-panel:popover-open .md-section-label')).filter(n => n.getClientRects().length && n.textContent === 'Effort').length);
      e.foldedPanel = { effortHeadingCount: headings, choiceValues: fp.choiceValues, currentValue: fp.pressedValues[0] };
      /* Overflow and the panel's bounds are measured in the folded layout
         itself, while the panel is open and again after the choice. */
      const overflowNow = () => p.evaluate(() => ({ page: document.documentElement.scrollWidth > innerWidth, viewport: innerWidth }));
      e.foldedOverflowOpen = await overflowNow();
      e.foldedPanelBounds = Object.assign({ viewportWidth: await p.evaluate(() => innerWidth) }, fp.panelRect);
      e.foldedPanelInsideViewport = fp.panelRect.x >= 0 && fp.panelRect.x + fp.panelRect.width <= e.foldedPanelBounds.viewportWidth + 0.5;
      await p.screenshot({ path: path.join(artifactDirectory, 'fold.png') });
      e.chosenValue = e.offered.find(o => o.value !== e.currentValue).value;
      await openPanel().locator('[data-level="' + e.chosenValue + '"]').click(); await settle();
      e.foldedPanel.focusAfterChoice = await focused(); e.foldedPanel.openAfterChoice = await panelOpen();
      e.foldedOverflowAfterChoice = await overflowNow();
      await p.screenshot({ path: path.join(artifactDirectory, 'fold-after-choice.png') });
      if (width >= 640) { await chat.evaluate(r => { r.style.width = r.dataset.receivingWidth || ''; r.style.maxWidth = ''; }); }
      else { await p.setViewportSize({ width: 1440, height: 1000 }); setup.push('viewport widened to 1440 to unfold the toolbar after the folded journey'); }
      await settle();
      const shown = await effort().evaluate(b => b.getAttribute('aria-label') || '');
      e.selectedValueAfterUnfold = (e.offered.find(o => shown.includes(', ' + o.name + ', level')) || {}).value;
      e.unfoldedEffortButtonCount = await chat.locator('[data-command="effort"]:visible').count();
      e.unfoldedThinkingButtonCount = await chat.locator('[data-command="thinking"]').count();
      e.unfoldedOpenPanelCount = await p.locator('.menu-panel:popover-open').count();
      e.unfoldedOverflow = await overflowNow();
      e.horizontalPageOverflow = e.foldedOverflowOpen.page || e.foldedOverflowAfterChoice.page || e.unfoldedOverflow.page || !e.foldedPanelInsideViewport;
      if (width < 640) await p.setViewportSize({ width, height: 1000 });
    }
    if (scenario === 'running') {
      e.programs = [];
      for (const [program, id] of [['Codex', 'three'], ['Claude', 'five'], ['Devin', 'devin'], ['Ollama', 'local']]) {
        await openChat(id);
        const list = offered(id), old = await current(id), to = list.find(o => o.value !== old);
        await showPanel(); const before = await panelState(); await p.keyboard.press('Escape'); await settle();
        await working(id, true);
        await showPanel(); const during = await panelState();
        await openPanel().locator('[data-level="' + to.value + '"]').click(); await settle();
        const line = await workingLine(), elsewhere = await timingElsewhere();
        await p.screenshot({ path: path.join(artifactDirectory, 'running-' + program + '.png') });
        await working(id, false); await working(id, true);
        const stale = await workingLine();
        await working(id, false);
        e.programs.push({ program, chatId: id, offered: list, oldName: list.find(o => o.value === old).name, newName: to.name, workingLine: line, timingTextOutsideWorkingLine: elsewhere,
          panelText: during.visibleText, panelBefore: before.panelRect, panelAfter: during.panelRect, nextTurnStaleNotice: stale });
      }
    }
    e.setup = setup;
    e.focusTrail = await p.evaluate(() => window.__focusTrail.slice());
    e.requestLog = await requests();
    e.optionPayloads = await p.evaluate(() => window.__optionPayloads.slice());
    e.source = sourceIdentity();
    e.runner = { node: process.version, browser: EDGE, headless: true };
    fs.writeFileSync(path.join(artifactDirectory, 'evidence.json'), JSON.stringify(e, null, 2));
    return e;
  }
  return { run, close: () => t.close() };
}
module.exports = { open };
