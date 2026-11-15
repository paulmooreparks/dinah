const fs = require('fs');
const path = require('path');
const { launch, OUT } = require('./lib/page');
const { choose, shows, rowsOf } = require('./lib/commands');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const ws = 'C:/Fixture';
    const model = { id: 'model', category: 'model', name: 'Model', type: 'select', currentValue: 'one', options: [{ value: 'one', name: 'Fixture Model' }] };
    window.__thinking = { id: 'arbitrary-effort', category: 'thought_level', name: 'Agent effort', type: 'select', currentValue: 'calm', options: [{ value: 'calm', name: 'A little thought' }, { value: 'deep', name: 'The agent\'s most detailed reasoning level' }] };
    window.__options = [model, __thinking]; window.__savedChoices = []; window.__failChoice = false;
    return {
      State: async () => ({ workspace: ws, recent: [ws], defaultProvider: 'codex', catalog: __options, defaults: {}, pinned: ['one'] }),
      AgentProviders: async () => [window.__provider('codex')],
      Sessions: async () => [{ sessionId: 'fixture', cwd: ws, provider: 'codex', title: 'Thinking fixture' }],
      OpenSessionAt: async () => { setTimeout(() => __emit('agent:options', { sessionId: 'fixture', options: __options }), 0); return { sessionId: 'fixture', cwd: ws, provider: 'codex', title: 'Thinking fixture', open: true, working: false, queued: [] }; },
      SetSessionOption: async (id, option, value) => { if (__failChoice) throw new Error('The setting could not be changed.'); __savedChoices.push([id, option, value]); __options = __options.map(o => o.id === option ? { ...o, currentValue: value } : o); return __options; }
    };
  } });
  const p = t.page, ok = t.ok;
  try {
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.locator('#sessions a').filter({ hasText: 'Thinking fixture' }).click();
    const chat = p.locator('.win[data-chat-current]');
    await chat.locator('[data-win-action="maximize"]').click();
    ok('the toolbar shows the reported current Thinking name', await shows(chat, 'thinking', 'A little thought'));
    const rows = await rowsOf(p, chat, 'thinking');
    ok('the menu uses the agent names without invented levels', rows.join('|') === "A little thought|The agent's most detailed reasoning level", rows.join('|'));
    await p.keyboard.press('Escape');
    await choose(p, chat, 'thinking', "The agent's most detailed reasoning level");
    ok('the selected value uses the ordinary per-chat option call', await p.evaluate(() => JSON.stringify(__savedChoices) === JSON.stringify([['fixture', 'arbitrary-effort', 'deep']])) && await shows(chat, 'thinking', "The agent's most detailed reasoning level"));
    const evidence = [];
    for (const width of [1440, 390]) {
      await p.setViewportSize({ width, height: 1000 });
      await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      if (width === 1440) ok('a wide chat exposes Thinking beside Model', await chat.locator('[data-command="thinking"]').isVisible());
      const frames = await chat.evaluate(async root => {
        const measure = () => Array.from(root.querySelectorAll('.composer-bar,.composer-toolbar,[data-command="model"],[data-command="thinking"],textarea')).flatMap(el => { const r = el.getBoundingClientRect(); return [r.x,r.y,r.width,r.height]; });
        const baseline = measure(), samples = [];
        for (let step = 0; step < 4; step++) {
          __options = __options.map(o => o.category === 'thought_level' ? { ...o, currentValue: step % 2 ? 'deep' : 'calm' } : o);
          __emit('agent:options', { sessionId: 'fixture', options: __options });
          samples.push(measure());
          for (let frame = 0; frame < 8; frame++) { await new Promise(requestAnimationFrame); samples.push(measure()); }
        }
        return { baseline, samples };
      });
      evidence.push({width,...frames});
      ok('Thinking value updates keep every sampled toolbar and composer box fixed at ' + width, frames.samples.every(s => JSON.stringify(s) === JSON.stringify(frames.baseline)));
      ok('the chat fits the viewport at ' + width, await chat.evaluate(root => { const r = root.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth + 1; }));
      ok('the narrow composer has no horizontal overflow at ' + width, await chat.locator('.composer-bar').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
      await t.shot('thinking-' + width);
      await rowsOf(p, chat, 'thinking'); await t.shot('thinking-menu-' + width); await p.keyboard.press('Escape');
    }
    fs.writeFileSync(path.join(OUT, 'thinking-frames.json'), JSON.stringify(evidence,null,2));
    await p.evaluate(() => { __options = __options.filter(o => o.category !== 'thought_level'); __emit('agent:options', { sessionId: 'fixture', options: __options }); });
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-command="thinking"]').hidden);
    ok('an unreported option is absent rather than disabled', await chat.locator('[data-command="thinking"]').evaluate(el => el.hidden));
    await p.evaluate(() => { __options.push(__thinking); __emit('agent:options', {sessionId:'fixture',options:__options}); });
    ok('a reported option returns with its authoritative current value', await shows(chat, 'thinking', 'A little thought'));
    await p.evaluate(() => { __failChoice = true; });
    await choose(p, chat, 'thinking', "The agent's most detailed reasoning level");
    ok('a refused change preserves the current option', await shows(chat, 'thinking', 'A little thought') && await p.evaluate(() => __savedChoices.length === 1));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
