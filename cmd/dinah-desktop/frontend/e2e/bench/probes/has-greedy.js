// The smallest set of :has() rules whose removal stops a streamed chunk restyling elements it does not concern.
// Each round takes out, one at a time, every rule still in, keeps out the one whose removal saves most, and repeats.
//   PROBE=bench/probes/has-greedy.js node bench/scaling.js [out] 1
module.exports = async ({ p, cdp, count, settle }) => {
  const restyled = async () => {
    const events = [], on = d => events.push(...d.value);
    cdp.on('Tracing.dataCollected', on);
    await cdp.send('Tracing.start', { transferMode: 'ReportEvents', traceConfig: { includedCategories: ['devtools.timeline'] } });
    await p.evaluate(async () => { for (let i = 0; i < 3; i++) { __emit('agent:update', { sessionId: __ids[__ids.length - 1], update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed words that make up part of an answer. ' } } }); await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); } });
    const done = new Promise(r => cdp.once('Tracing.tracingComplete', r)); await cdp.send('Tracing.end'); await done;
    cdp.off('Tracing.dataCollected', on);
    return events.filter(e => e.name === 'UpdateLayoutTree' && e.args && e.args.elementCount != null).reduce((a, e) => a + e.args.elementCount, 0);
  };
  const total = await p.evaluate(() => {
    window.__has = [];
    const walk = (owner, sheet) => { for (let i = 0; i < owner.cssRules.length; i++) { const r = owner.cssRules[i]; if (r.cssRules && r.cssRules.length && !r.selectorText) walk(r, sheet); else if (r.selectorText && r.selectorText.includes(':has(')) __has.push({ rule: r, text: r.selectorText, sheet: (sheet.href || '').split('/').pop(), out: false }); } };
    for (const sheet of document.styleSheets) { try { walk(sheet, sheet); } catch (e) {} }
    // A rule is taken out by giving it a selector that matches nothing, and put back with its own.
    window.__out = (i, out) => { __has[i].rule.selectorText = out ? '.never-matches-' + i : __has[i].text; __has[i].out = out; };
    return __has.length;
  });
  let now = await restyled();
  console.log('PROBE ' + count + ' chats: three chunks restyle ' + now + ' elements with all ' + total + ' :has() rules in');
  for (let round = 0; round < Number(process.env.ROUNDS || 8) && now > 400; round++) {
    let best = -1, least = now;
    for (let i = 0; i < total; i++) {
      if (await p.evaluate(i => __has[i].out, i)) continue;
      await p.evaluate(i => __out(i, true), i);
      const n = await restyled();
      if (n < least) { least = n; best = i; }
      await p.evaluate(i => __out(i, false), i);
    }
    if (best < 0) break;
    await p.evaluate(i => __out(i, true), best);
    await settle(p);
    now = await restyled();
    console.log('  out: ' + (await p.evaluate(i => __has[i].sheet + '  ' + __has[i].text, best)).slice(0, 300) + '\n       leaves ' + now);
  }
};
