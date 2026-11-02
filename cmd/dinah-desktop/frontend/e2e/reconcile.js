// dinahHelpers.reconcile and arrange (FE-18), the one way a list keeps its
// rows in step with its items: a new item gets a row, a changed order moves
// rows, a gone item loses its row, a row is kept, not rebuilt, across draws,
// focus stays on a row that moves, a rebuilt row takes its focus, and a
// removed row with focus calls lost with the place it had.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1024, height: 768 }, app: () => ({}) });
  const { page: p, ok } = t;
  try {
    const result = await p.evaluate(() => {
      const list = document.createElement('ul'); document.body.appendChild(list);
      const rows = new Map(), made = [], lostAt = [];
      const draw = (items, versions) => window.dinahHelpers.reconcile(list, items, {
        rows, key: item => item,
        create: (item, key) => { made.push(key); const li = document.createElement('li'); const b = document.createElement('button'); b.textContent = key; li.appendChild(b); li.dataset.key = key; return { node: li, version: (versions || {})[key] || 1 }; },
        update: (view, item) => {
          const version = (versions || {})[item] || 1;
          if (view.version !== version) { const li = document.createElement('li'); const b = document.createElement('button'); b.textContent = item + ' v' + version; li.appendChild(b); li.dataset.key = item; view.node = li; view.version = version; }
        },
        lost: index => lostAt.push(index)
      });
      const order = () => [...list.children].map(li => li.dataset.key).join(',');
      const out = {};
      draw(['a', 'b', 'c']);
      out.inserted = order();
      const bRow = list.children[1];
      list.querySelector('[data-key="c"] button').focus();
      draw(['c', 'a', 'b']);
      out.moved = order();
      out.kept = list.querySelector('[data-key="b"]') === bRow && made.join() === 'a,b,c';
      out.focusKept = document.activeElement === list.querySelector('[data-key="c"] button');
      draw(['c', 'a', 'b'], { a: 2 });
      list.querySelector('[data-key="a"] button').focus();
      draw(['c', 'a', 'b'], { a: 3 });
      out.rebuilt = list.querySelector('[data-key="a"] button').textContent === 'a v3' && document.activeElement === list.querySelector('[data-key="a"] button');
      list.querySelector('[data-key="a"] button').focus();
      draw(['c', 'b']);
      out.removed = order();
      out.lost = lostAt.join();
      out.forgotten = !rows.has('a');
      // arrange alone: other children are taken out, and focus stays.
      const box = document.createElement('div'); document.body.appendChild(box);
      const x = document.createElement('button'), y = document.createElement('button'), z = document.createElement('span');
      x.textContent = 'x'; y.textContent = 'y'; box.append(x, z, y);
      y.focus();
      window.dinahHelpers.arrange(box, [y, x]);
      out.arranged = [...box.children].map(n => n.textContent).join(',') + (document.activeElement === y ? ' focus kept' : ' focus lost');
      return out;
    });
    ok('new items get rows in order', result.inserted === 'a,b,c', result.inserted);
    ok('a changed order moves rows, keeping each row rather than rebuilding it', result.moved === 'c,a,b' && result.kept, JSON.stringify(result));
    ok('a row with focus that moves keeps its focus', result.focusKept);
    ok('a rebuilt row takes the old row\'s place and its focus', result.rebuilt);
    ok('a gone item loses its row, the list says where focus was lost, and the row is forgotten', result.removed === 'c,b' && result.lost === '1' && result.forgotten, JSON.stringify(result));
    ok('arrange orders the given elements, takes out the rest and keeps focus', result.arranged === 'y,x focus kept', result.arranged);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
