// Shared helpers retain the presentation choices of their callers.
const { launch, FRONTEND, PAGE } = require('./lib/page');
const path = require('path');
const fs = require('fs');

(async () => {
  const t = await launch({ goto: false });
  try {
    await t.page.addScriptTag({ path: path.join(FRONTEND, 'helpers.js') });
    const result = await t.page.evaluate(() => {
      const h = window.dinahHelpers;
      const original = window.go;
      window.go = undefined;
      const absent = h.app() === undefined;
      const replacement = { marker: true };
      window.go = { main: { App: replacement } };
      const replaced = h.app() === replacement;
      window.go = original;
      const node = h.node('span', 'sample', '<b>text</b>');
      return {
        bridge: absent && replaced,
        node: node.className === 'sample' && node.textContent === '<b>text</b>' && node.children.length === 0,
        values: [h.node('span', '', 0).textContent, h.node('span', '', false).textContent, h.node('span', '', null).textContent],
        errors: [h.message(new Error('failed')), h.message('Error: failed'), h.message(' failed ', 'capital'), h.message(' failed ', 'sentence'), h.message('failed!', 'sentence'), h.message('Error: failed', 'raw')],
        durations: [h.duration(0), h.duration(0, 0), h.duration(61), h.duration(3601)],
        formats: [h.num(1234), h.money({ amount: 1.234, currency: 'USD' }), h.money(null), h.contentPct({ used: 1, size: 3 }), h.contentPct(null), h.fmtSize(1024)]
      };
    });
    t.ok('bridge lookup follows the current bridge', result.bridge);
    t.ok('node text remains literal', result.node);
    t.ok('node values preserve zero, false and empty text', JSON.stringify(result.values) === '["0","false",""]');
    t.ok('error styles retain caller punctuation and capitalization', JSON.stringify(result.errors) === '["failed","failed","Failed","Failed.","Failed!","Error: failed"]');
    t.ok('durations retain chat and subagent minimums', JSON.stringify(result.durations) === '["1 s","0 s","1 min 1 s","1 h 0 min"]');
    t.ok('number, cost, context and size formats are preserved', JSON.stringify(result.formats) === '["1,234","1.23 USD","",33,0,"1.0 KB"]');
  } finally {
    await t.close();
  }

  const integrated = await launch({
    goto: false,
    init: () => {
      window.__subscriptions = {};
      const on = window.runtime.EventsOn;
      window.runtime.EventsOn = (name, handler) => {
        window.__subscriptions[name] = (window.__subscriptions[name] || 0) + 1;
        return on(name, handler);
      };
    }
  });
  try {
    await integrated.context.route('http://dinah.test/helpers.js', route => route.fulfill({
      contentType: 'text/javascript',
      body: fs.readFileSync(path.join(FRONTEND, 'helpers.js'), 'utf8') + `
        window.__earlyHost = window.dinahHost;
        window.__received = [];
        window.__earlyOff = dinahHost.on('helpers:test', value => __received.push('early:' + value));
        dinahHost.on('agent:helpers-test', value => __received.push('agent:' + value));
        window.__emit('helpers:test', 'before');
      `
    }));
    await integrated.page.goto(PAGE);
    await integrated.waitReady();
    const state = await integrated.page.evaluate(() => {
      const same = window.__earlyHost === window.dinahHost;
      const off = dinahHost.on('helpers:test', value => __received.push('late:' + value));
      __emit('helpers:test', 'after');
      __earlyOff();
      __emit('helpers:test', 'removed');
      off();
      __emit('helpers:test', 'empty');
      const again = dinahHost.on('helpers:test', value => __received.push('again:' + value));
      __emit('helpers:test', 'resubscribed');
      again();
      __emit('agent:helpers-test', 'queued');
      return { same, values: __received.slice(), counts: __subscriptions };
    });
    integrated.ok('startup extends the original host object', state.same);
    integrated.ok('early and late subscriptions survive startup and unsubscribe independently', JSON.stringify(state.values) === '["early:before","early:after","late:after","late:removed","again:resubscribed"]');
    integrated.ok('all events have one runtime subscription, including resubscriptions', Object.values(state.counts).every(count => count === 1));
    await integrated.page.waitForFunction(() => __received.includes('agent:queued'));
    integrated.ok('early agent subscriptions use the render queue after startup', await integrated.page.evaluate(() => __received.filter(value => value === 'agent:queued').length === 1));
  } finally {
    await integrated.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
