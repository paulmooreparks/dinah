// Which :has() rules make one streamed chunk restyle elements it does not concern.
// Every rule with :has() is taken out of the page's stylesheets, and each is put back alone in turn;
// the number of elements the browser restyles for five streamed chunks is reported for each.
//   PROBE=bench/probes/has-rules.js node bench/scaling.js [out] 1
module.exports = async ({ p, cdp, count, settle }) => {
  const restyled = async () => {
    const events = [], on = d => events.push(...d.value);
    cdp.on('Tracing.dataCollected', on);
    await cdp.send('Tracing.start', { transferMode: 'ReportEvents', traceConfig: { includedCategories: ['devtools.timeline'] } });
    await p.evaluate(async () => { for (let i = 0; i < 5; i++) { __emit('agent:update', { sessionId: __ids[__ids.length - 1], update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed words that make up part of an answer. ' } } }); await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); } });
    const done = new Promise(r => cdp.once('Tracing.tracingComplete', r)); await cdp.send('Tracing.end'); await done;
    cdp.off('Tracing.dataCollected', on);
    return events.filter(e => e.name === 'UpdateLayoutTree' && e.args && e.args.elementCount != null).reduce((a, e) => a + e.args.elementCount, 0);
  };
  const base = await restyled();
  // Take every style rule whose selector has :has() out, remembering where each was.
  const total = await p.evaluate(() => {
    window.__has = [];
    const walk = (owner, sheet) => { for (let i = owner.cssRules.length - 1; i >= 0; i--) { const r = owner.cssRules[i]; if (r.cssRules && r.cssRules.length && !r.selectorText) walk(r, sheet); else if (r.selectorText && r.selectorText.includes(':has(')) { __has.push({ owner, index: i, text: r.cssText, selector: r.selectorText, sheet: (sheet.href || '').split('/').pop() }); owner.deleteRule(i); } } };
    for (const sheet of document.styleSheets) { try { walk(sheet, sheet); } catch (e) {} }
    return __has.length;
  });
  await settle(p);
  const none = await restyled();
  console.log('PROBE ' + count + ' chats: five chunks restyle ' + base + ' elements as it is, ' + none + ' with all ' + total + ' :has() rules out');
  const rows = [];
  for (let i = 0; i < total; i++) {
    await p.evaluate(i => { const h = __has[i]; h.at = h.owner.insertRule(h.text, h.owner.cssRules.length); }, i);
    await settle(p);
    const n = await restyled();
    rows.push([n - none, await p.evaluate(i => __has[i].sheet + '  ' + __has[i].selector, i)]);
    await p.evaluate(i => { const h = __has[i]; h.owner.deleteRule(h.at); }, i);
  }
  rows.sort((a, b) => b[0] - a[0]);
  for (const [n, name] of rows.filter(r => r[0] > 20)) console.log('  +' + String(n).padStart(6) + '  ' + name.slice(0, 230));
  console.log('  ' + rows.filter(r => r[0] <= 20).length + ' rules add 20 elements or fewer');
};
