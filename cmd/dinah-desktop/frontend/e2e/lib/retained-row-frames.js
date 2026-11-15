const fs = require('fs'), path = require('path');
const { OUT } = require('./page');
module.exports = async function recordRows(scope) {
  await scope.evaluate(root => {
    const rows = [...root.querySelectorAll('.run-log > .activity-row')];
    const rect = el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; };
    const sample = () => ({ rows: rows.map(row => ({ connected: row.isConnected, box: rect(row) })), chrome: [rect(root), rect(root.querySelector('.composer')), rect(document.querySelector('.win-bar'))] });
    const audit = window.__retainedRowFrames = { before: sample(), frames: [], running: true };
    function tick() { if (!audit.running) return; audit.frames.push(sample()); requestAnimationFrame(tick); }
    requestAnimationFrame(tick);
  });
  return async function finish(t, name) {
    const result = await scope.page().evaluate(async () => {
      for (let i = 0; i < 3; i++) await new Promise(requestAnimationFrame);
      const audit = window.__retainedRowFrames; audit.running = false;
      return { before: audit.before, frames: audit.frames };
    });
    fs.mkdirSync(OUT, { recursive: true }); fs.writeFileSync(path.join(OUT, name + '-frames.json'), JSON.stringify(result, null, 2));
    t.ok(name + ' retains every measured row without an estimated-height frame', result.before.rows.length > 0 && result.frames.every(f => f.rows.every((r, i) => r.connected && Math.abs(r.box[3] - result.before.rows[i].box[3]) < 1)));
    t.ok(name + ' keeps the window and taskbar fixed in every frame', result.frames.every(f => [0, 2].every(i => f.chrome[i].every((v, j) => Math.abs(v - result.before.chrome[i][j]) < 1))));
  };
};
