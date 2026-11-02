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
        // Random reorders of 200 rows with one focused: the order always
        // comes out as asked, and the focused row is never put back.
        const rows3 = new Map(), list3 = document.createElement('ul'); document.body.appendChild(list3);
        const make = k => { const li = document.createElement('li'); const b = document.createElement('button'); b.textContent = k; li.appendChild(b); li.dataset.key = k; return { node: li }; };
        const draw3 = items => window.dinahHelpers.reconcile(list3, items, { rows: rows3, key: k => k, create: make });
        let keys = Array.from({ length: 200 }, (_, i) => 'r' + i), wrong = 0, focusMoved = 0, seed = 7;
        const random = () => (seed = (seed * 1103515245 + 12345) % 2147483648) / 2147483648;
        draw3(keys);
        const watch = new MutationObserver(records => records.forEach(r => r.addedNodes.forEach(n => { if (n.contains(document.activeElement)) focusMoved++; })));
        watch.observe(list3, { childList: true });
        for (let round = 0; round < 50; round++) {
          list3.children[Math.floor(random() * keys.length)].querySelector('button').focus();
          keys = keys.slice();
          for (let n = 0; n < 5; n++) { const a = Math.floor(random() * keys.length), b = Math.floor(random() * keys.length); const t = keys[a]; keys[a] = keys[b]; keys[b] = t; }
          draw3(keys);
          if ([...list3.children].map(li => li.dataset.key).join() !== keys.join()) wrong++;
        }
        watch.disconnect();
        out.random = { wrong, focusMoved };
        // A chat list of 5000 rows: a redraw that changes nothing and one
        // that reverses the list, timed.
        const rows4 = new Map(), list4 = document.createElement('ul'); document.body.appendChild(list4);
        const big = Array.from({ length: 5000 }, (_, i) => 'k' + i);
        const draw4 = items => window.dinahHelpers.reconcile(list4, items, { rows: rows4, key: k => k, create: make });
        draw4(big);
        let start = performance.now(); draw4(big); const same = performance.now() - start;
        start = performance.now(); draw4(big.slice().reverse()); const reversed = performance.now() - start;
        out.timing = { same: Math.round(same), reversed: Math.round(reversed) };
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
    ok('fifty random reorders of 200 rows come out in order, and the focused row is never moved', result.random.wrong === 0 && result.random.focusMoved === 0, JSON.stringify(result.random));
    // Generous bounds, for a loaded machine: the search is n log n and an
    // unchanged list skips it, where the first version compared every pair.
    console.log('timing for 5000 rows (ms): ' + JSON.stringify(result.timing));
    ok('5000 rows redraw unchanged in under 100 ms and reversed in under 500 ms', result.timing.same < 100 && result.timing.reversed < 500, JSON.stringify(result.timing));
    ok('in a production build it is logged and the row is drawn once', result.logged.errors && result.logged.errors.length === 1 && /"c"/.test(result.logged.errors[0]) && result.logged.order === 'b,c,d', JSON.stringify(result.logged));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
