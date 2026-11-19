// A suggested reply in the prompt box (docs/plans/reply-suggestions.md).
// When a turn ends in a chat with a window the page asks for a suggestion and
// draws it over the empty box. It is never the box's text until Tab takes it,
// the box does not grow when it arrives, Esc drops it, typing hides it, and a
// turn under way forgets it.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 800 },
    app: () => {
      window.__asked = []; window.__outcomes = []; window.__suggestion = 'Yes, rename the pinned one too.'; window.__ver = 0; window.__call = 100;
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'] }),
        Sessions: async () => [{ sessionId: 'chat-s', title: 'Suggested Reply', provider: 'codex', cwd: 'C:/Fixture', open: false }],
        OpenSessionAt: async (w, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: w, open: true, working: false, queued: [], chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } }),
        Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Answer: async () => {}, Cancel: async () => {},
        SuggestReply: async id => { window.__asked.push(id); const text = window.__suggestion; await new Promise(r => setTimeout(r, 50)); return { text, call: text ? ++window.__call : 0 }; },
        ReplySuggestionOutcome: async (id, call, outcome) => { window.__outcomes.push([id, call, outcome]); },
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    await p.goto(PAGE);
    await t.waitReady();
    const key = await p.evaluate(() => window.dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'chat-s' }));
    const win = p.locator('[data-win="' + key + '"]');
    const box = win.locator('[data-view-id="prompt"]');
    const ghost = win.locator('[data-view-id="reply-suggestion"]');
    const cap = win.locator('[data-view-id="reply-suggestion-key"]');
    await box.waitFor();
    const session = (working) => p.evaluate(w => window.__emit('agent:session', { sessionId: 'chat-s', stateGen: 'g', stateVer: ++window.__ver, provider: 'codex', cwd: 'C:/Fixture', open: true, working: w, queued: [],
      chatState: { life: 'open', window: true, exec: { state: w ? 'working' : 'idle', why: '' } } }), working);
    const turnEnds = async (stopReason) => { await p.evaluate(r => window.__emit('agent:turn', { sessionId: 'chat-s', stopReason: r, seconds: 2 }), stopReason || 'end_turn'); await session(false); };
    const shown = () => ghost.evaluate(g => g.hidden ? null : g.textContent);
    const height = () => box.evaluate(b => Math.round(b.getBoundingClientRect().height));
    const PLACEHOLDER = 'Say what you want the agent to do, or type / for a skill.';

    // The window that opens on an idle chat asks once.
    await p.waitForFunction(() => window.__asked.length === 1);
    await ghost.waitFor({ state: 'visible' });
    ok('a chat whose window opens is asked for once, and the suggestion shows over the empty box', (await shown()) === 'Yes, rename the pinned one too.' && await cap.isVisible() && (await cap.textContent()) === 'Tab');
    ok('the suggestion is not the box\'s text, and Send stays greyed', (await box.inputValue()) === '' && await win.locator('[data-view-id="send"]').isDisabled());
    ok('the box\'s own placeholder gives way to it', (await box.getAttribute('placeholder')) === '');
    ok('a screen reader is told of it and of the keys', (await win.locator('[data-view-id="reply-suggestion-help"]').textContent()) === 'A reply is suggested. Press Tab to use it, or Escape to drop it. It reads: Yes, rename the pinned one too.');
    ok('no draft is kept of a suggestion', await p.evaluate(() => !Object.keys(localStorage).some(k => /draft/i.test(k) && /rename the pinned/.test(localStorage.getItem(k) || ''))));

    // The box's height with a short suggestion over it, which a long one must not change.
    const empty = await height();
    // Typing hides it, and an emptied box shows it again.
    await box.focus();
    await p.keyboard.type('x');
    ok('typing hides the suggestion', (await shown()) === null && !(await cap.isVisible()));
    await p.keyboard.press('Backspace');
    ok('an emptied box shows it again', (await shown()) === 'Yes, rename the pinned one too.');

    // Tab takes it.
    await p.keyboard.press('Tab');
    const taken = await box.evaluate(b => ({ value: b.value, caret: [b.selectionStart, b.selectionEnd], focused: document.activeElement === b }));
    ok('Tab makes it the box\'s text with the caret at its end, and the box keeps the keyboard', taken.value === 'Yes, rename the pinned one too.' && taken.caret[0] === taken.value.length && taken.caret[1] === taken.value.length && taken.focused, JSON.stringify(taken));
    ok('Send comes alive, and nothing was sent', await win.locator('[data-view-id="send"]').isEnabled() && !(await t.calls('Send')).length);
    ok('the suggestion is gone from over the box', (await shown()) === null);
    await p.waitForFunction(() => window.__outcomes.length === 1);
    ok('the record is told it was taken', JSON.stringify(await p.evaluate(() => window.__outcomes)) === JSON.stringify([['chat-s', 101, 'taken']]));

    // A turn under way forgets a suggestion, and the next turn's end asks again.
    await box.fill('');
    await session(true);
    await p.waitForFunction(k => /Working/.test(document.querySelector('[data-win="' + k + '"]').dataset.winStateLabel || ''), key);
    ok('nothing shows while a turn is under way', (await shown()) === null);
    // A long suggestion is cut to the box's lines, and the box does not grow when it arrives.
    await p.evaluate(() => { window.__suggestion = 'Take the two installer findings first, since they block the trial on the locked-down laptop, then the update check. Leave the wording of the first-start card until the others are in, and tell me before you change any text the tests pin.'; });
    await turnEnds();
    await p.waitForFunction(() => window.__asked.length === 2);
    await ghost.waitFor({ state: 'visible' });
    const laid = await p.evaluate(k => { const w = document.querySelector('[data-win="' + k + '"]'), g = w.querySelector('[data-view-id="reply-suggestion"]').getBoundingClientRect(), b = w.querySelector('[data-view-id="prompt"]').getBoundingClientRect();
      return { inside: g.top >= b.top - 1 && g.bottom <= b.bottom + 1 && g.left >= b.left - 1 && g.right <= b.right + 1, ghost: Math.round(g.height), box: Math.round(b.height) }; }, key);
    ok('a long suggestion lies inside the box, and the box is no taller for it', laid.inside && laid.box === empty && (await height()) === empty, JSON.stringify({ laid, empty }));
    await t.shot('reply-suggestion');
    await box.focus();
    await p.keyboard.press('Tab');
    ok('Tab gives the whole of a long suggestion', (await box.inputValue()).endsWith('any text the tests pin.') && (await height()) > empty);

    // Esc drops it, the placeholder returns, and Tab moves on as before.
    await box.fill('');
    await p.evaluate(() => { window.__suggestion = '1. Yes\n2. Renamed'; });
    await session(true); await turnEnds();
    await p.waitForFunction(() => window.__asked.length === 3);
    await ghost.waitFor({ state: 'visible' });
    ok('a suggestion of two lines keeps its lines', (await shown()) === '1. Yes\n2. Renamed');
    await box.focus();
    await p.keyboard.press('Escape');
    ok('Esc drops the suggestion and the box has its own words again', (await shown()) === null && (await box.getAttribute('placeholder')) === PLACEHOLDER && (await box.inputValue()) === '');
    await p.waitForFunction(() => window.__outcomes.length === 3);
    ok('the record is told it was dropped', (await p.evaluate(() => window.__outcomes[2].join('|'))) === 'chat-s|103|dropped');
    await p.keyboard.press('Tab');
    ok('with no suggestion, Tab moves to the next control as before', await box.evaluate(b => document.activeElement !== b) && (await box.inputValue()) === '');

    // A judge that proposes nothing leaves the box as it always is, and a
    // turn the person stopped asks for nothing.
    await p.evaluate(() => { window.__suggestion = ''; });
    await session(true); await turnEnds();
    await p.waitForFunction(() => window.__asked.length === 4);
    await p.waitForTimeout(200);
    ok('a judge that proposes nothing leaves the box as it is', (await shown()) === null && (await box.getAttribute('placeholder')) === PLACEHOLDER && !(await cap.isVisible()));
    await p.evaluate(() => { window.__suggestion = 'Go on.'; });
    await session(true); await turnEnds('cancelled');
    await p.waitForTimeout(250);
    ok('a turn the person stopped asks for no suggestion', (await p.evaluate(() => window.__asked.length)) === 4 && (await shown()) === null);

    // An answer that comes back after the next turn has started is not shown.
    await session(true); await p.evaluate(() => window.__emit('agent:turn', { sessionId: 'chat-s', stopReason: 'end_turn', seconds: 1 }));
    await session(true);
    await p.waitForFunction(() => window.__asked.length === 5);
    await p.waitForTimeout(250);
    ok('a suggestion that arrives after the next turn began is not shown', (await shown()) === null);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
