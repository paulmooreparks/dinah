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
      // A focused row already in place never moves while another row moves
      // past it: it is not blurred and focused again.
      const rows2 = new Map(), list2 = document.createElement('ul'); document.body.appendChild(list2);
      const draw2 = items => window.dinahHelpers.reconcile(list2, items, { rows: rows2, key: k => k, create: k => { const li = document.createElement('li'); const b = document.createElement('button'); b.textContent = k; li.appendChild(b); li.dataset.key = k; return { node: li }; } });
      draw2(['a', 'b', 'c', 'd']);
      const focusedButton = list2.querySelector('[data-key="b"] button');
      let blurs = 0, moved = 0;
      focusedButton.addEventListener('blur', () => blurs++);
      focusedButton.focus();
      new MutationObserver(records => records.forEach(r => r.addedNodes.forEach(n => { if (n.dataset && n.dataset.key === 'b') moved++; }))).observe(list2, { childList: true });
      draw2(['b', 'c', 'd', 'a']);
      return new Promise(resolve => setTimeout(() => {
        out.passed = { order: [...list2.children].map(li => li.dataset.key).join(','), blurs, moved, focused: document.activeElement === focusedButton };
        // A key given to two items: thrown in the suites (dinahStrict), and
        // logged with the row drawn once in a production build.
        try { draw2(['b', 'c', 'c', 'd']); out.thrown = ''; } catch (e) { out.thrown = e.message; }
        const errors = [], log = console.error;
        console.error = text => errors.push(String(text));
        window.dinahStrict = false;
        try { draw2(['b', 'c', 'c', 'd']); out.logged = { errors, order: [...list2.children].map(li => li.dataset.key).join(',') }; }
        catch (e) { out.logged = { threw: e.message }; }
        finally { window.dinahStrict = true; console.error = log; }
        resolve(out);
      }, 0));
    });
    ok('new items get rows in order', result.inserted === 'a,b,c', result.inserted);
    ok('a changed order moves rows, keeping each row rather than rebuilding it', result.moved === 'c,a,b' && result.kept, JSON.stringify(result));
    ok('a row with focus that moves keeps its focus', result.focusKept);
    ok('a rebuilt row takes the old row\'s place and its focus', result.rebuilt);
    ok('a gone item loses its row, the list says where focus was lost, and the row is forgotten', result.removed === 'c,b' && result.lost === '1' && result.forgotten, JSON.stringify(result));
    ok('arrange orders the given elements, takes out the rest and keeps focus', result.arranged === 'y,x focus kept', result.arranged);
    ok('a focused row in place stays put, unblurred and focused, while another row moves past it', result.passed.order === 'b,c,d,a' && result.passed.blurs === 0 && result.passed.moved === 0 && result.passed.focused, JSON.stringify(result.passed));
    ok('a key given to two items is thrown in the suites, naming the key', /"c"/.test(result.thrown) && /more than one item/.test(result.thrown), result.thrown);
    ok('in a production build it is logged and the row is drawn once', result.logged.errors && result.logged.errors.length === 1 && /"c"/.test(result.logged.errors[0]) && result.logged.order === 'b,c,d', JSON.stringify(result.logged));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
