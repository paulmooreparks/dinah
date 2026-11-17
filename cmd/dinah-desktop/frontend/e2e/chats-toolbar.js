// The Chats pane's commands are a folding PUDL toolbar: Org Chart and Clean
// Up Chats with their glyphs, then New Chat at the row's end. As the pane
// narrows the words go and the glyphs stay, and it keeps to one row. They
// were two buttons the full width of the pane (Paul, 17 November 2026; the
// approved mockup is docs/mockups/chats-toolbar.html).
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1900, height: 900 }, ready: 'workbench' });
  const { page: p, ok } = t;
  const read = () => p.evaluate(() => {
    const b = document.querySelector('.chat-commands'), r = b.getBoundingClientRect();
    const buttons = [...b.querySelectorAll('.toolbar-group > button')];
    const find = document.getElementById('chat-filter').getBoundingClientRect(), scope = document.getElementById('chat-scope').getBoundingClientRect();
    return {
      width: Math.round(r.width), height: Math.round(r.height), overflow: b.scrollWidth > b.clientWidth + 1,
      shown: buttons.filter(x => x.getClientRects().length).map(x => ({ words: x.querySelector('.toolbar-words').textContent, wordsShown: !!x.querySelector('.toolbar-words').getClientRects().length, glyph: x.querySelector('.glyph').style.getPropertyValue('--glyph').trim().replace(/var\(--glyph-|\)/g, ''), glyphShown: !!x.querySelector('.glyph').getClientRects().length, name: x.getAttribute('aria-label') || x.title || x.textContent.trim(), left: Math.round(x.getBoundingClientRect().left), right: Math.round(x.getBoundingClientRect().right) })),
      sameRow: Math.abs(find.top - scope.top) < 2, findWidth: Math.round(find.width), scopeWidth: Math.round(scope.width),
    };
  });
  const fitted = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  try {
    await p.waitForFunction(() => !document.body.hasAttribute('aria-busy'));
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    const bar = p.locator('.chat-commands');
    await bar.waitFor();
    await fitted();
    const wide = await read();
    ok('the Chats pane has a PUDL toolbar', await bar.getAttribute('role') === 'toolbar');
    ok('wide, it shows Org Chart, Clean Up Chats and New Chat in that order, each with its glyph and words, on one row',
      wide.shown.map(x => x.words).join('|') === 'Org Chart|Clean Up Chats|New Chat' && wide.shown.every(x => x.wordsShown && x.glyphShown) && wide.height < 40 && !wide.overflow, JSON.stringify(wide));
    ok('the glyphs are the chart\'s, the bin and the chat', wide.shown.map(x => x.glyph).join('|') === 'subagents|bin|comments', JSON.stringify(wide.shown.map(x => x.glyph)));
    ok('New Chat is at the row\'s end', wide.shown[2].left > wide.shown[1].right && wide.width - (wide.shown[2].right - wide.shown[0].left) >= 0);
    ok('no button fills the pane\'s width', wide.shown.every(x => x.right - x.left < wide.width * 0.6), JSON.stringify(wide));
    await t.shot('chats-toolbar-wide', { clip: await p.locator('#chat-browser-view').boundingBox() });

    // A narrow dock: the words go, the glyphs stay, and each button keeps its name.
    await p.setViewportSize({ width: 1000, height: 900 });
    await p.waitForFunction(() => document.querySelector('.chat-commands').getBoundingClientRect().width < 260);
    await fitted();
    const narrow = await read();
    ok('narrow, it keeps one row in the same order without overflowing', narrow.height < 40 && !narrow.overflow && narrow.shown.map(x => x.words).join('|') === 'Org Chart|Clean Up Chats|New Chat', JSON.stringify(narrow));
    ok('the first two show their glyphs alone, and every button keeps its name', narrow.shown.slice(0, 2).every(x => !x.wordsShown && x.glyphShown) && narrow.shown.every(x => x.name), JSON.stringify(narrow.shown));
    ok('the field and the scope are both in sight, each at a width that can be read', narrow.findWidth >= 120 && narrow.scopeWidth >= 120, JSON.stringify(narrow));
    await t.shot('chats-toolbar-narrow', { clip: await p.locator('#chat-browser-view').boundingBox() });

    // Each button does what it did.
    await p.setViewportSize({ width: 1900, height: 900 });
    await fitted();
    await bar.getByRole('button', { name: 'Org Chart' }).click();
    await p.locator('.win[data-win="org-chart"]').waitFor();
    ok('Org Chart opens the chart', true);
    await bar.getByRole('button', { name: 'Clean Up Chats' }).click();
    await p.locator('.win[data-win="chat-cleanup"]').waitFor();
    ok('Clean Up Chats opens its window', true);
    // The toolbar is one Tab stop, and the arrow keys move along it.
    await bar.getByRole('button', { name: 'Org Chart' }).focus();
    await p.keyboard.press('ArrowRight');
    await p.keyboard.press('ArrowRight');
    ok('Right moves from Org Chart to Clean Up Chats and on to New Chat', await p.evaluate(() => document.activeElement.dataset.cmd) === 'new-session');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
