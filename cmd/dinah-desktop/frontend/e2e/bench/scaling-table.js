// Prints scaling.json's actions split into script, style and layout time for one unit.
//   node bench/scaling-table.js [outFolder]
const r = require(require('path').join(process.argv[2] || require('os').tmpdir(), 'scaling.json'));
const pad = (s, n) => String(s).padEnd(n);
for (const run of r) {
  console.log('\n' + run.count + ' chats');
  console.log(pad('action', 18) + pad('task', 9) + pad('script', 9) + pad('style', 9) + pad('layout', 9) + pad('styles', 8) + pad('layouts', 8) + 'bridge calls');
  for (const [name, a] of Object.entries(run.actions)) {
    const f = v => pad((v / a.per).toFixed(2), 9);
    console.log(pad(name, 18) + f(a.task) + f(a.script) + f(a.style) + f(a.layout) + pad(((a.styles || 0) / a.per).toFixed(1), 8) + pad(((a.layouts || 0) / a.per).toFixed(1), 8) + JSON.stringify(a.calls || {}) + (a.saves ? ' saves=' + a.saves + ' bytes=' + a.saveBytes : ''));
  }
}
