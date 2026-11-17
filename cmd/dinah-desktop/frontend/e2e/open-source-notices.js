// Help, About has an Open-Source Notices button, and the window it opens
// lists every open-source component in the programs with its notices
// (open-source-notices.js). The list is the real one the build writes
// (third-party-notices.json), so this also checks that the page can read it.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 900 }, ready: 'workbench' });
  const { page: p, ok } = t;
  try {
    ok('nothing is read at start', !(await p.evaluate(() => performance.getEntriesByType('resource').some(e => /third-party-notices/.test(e.name)))));
    await p.evaluate(() => document.getElementById('about').showModal());
    const button = p.locator('#about').getByRole('button', { name: 'Open-Source Notices…', exact: true });
    ok('the About box has the button, always shown', await button.isVisible() && /includes open-source software/.test(await p.locator('#about').innerText()));
    await button.click();
    const win = p.locator('.win[data-win-kind="open-source-notices"]');
    await win.locator('.notices-rows tbody tr').first().waitFor();
    ok('the About box closes as the window opens', !(await p.evaluate(() => document.getElementById('about').open)));
    const total = await win.locator('.notices-rows tbody tr').count();
    ok('the window lists the components', total > 50 && (await win.locator('.notices-count').textContent()) === total + ' components', String(total));
    ok('the first component is chosen and its notice is shown', /Go runtime/.test(await win.locator('.notices-name').textContent()) && /Redistribution and use in source and binary forms/.test(await win.locator('.notices-text').textContent()) && await win.locator('tbody tr[aria-selected="true"]').count() === 1);
    ok('PUDL is listed by its name, with its version', await win.locator('tbody tr', { hasText: 'PUDL, the Pleasantly Usable Design Language' }).locator('td').nth(1).textContent().then(v => /^\d+\.\d+\.\d+$/.test(v)));

    // Both panes keep their place and size whatever is chosen or filtered.
    const boxes = () => win.evaluate(w => ['.notices-list', '.notices-detail', '.notices-rows', '.notices-text', '.notices-actions'].map(s => { const r = w.querySelector(s).getBoundingClientRect(); return [r.left, r.top, r.width, r.height].map(Math.round).join(','); }).join(' | '));
    const before = await boxes();
    await win.locator('.notices-open', { hasText: 'oss.terrastruct.com/d2' }).first().click();
    ok('choosing a component shows its notice', /Mozilla Public License/.test(await win.locator('.notices-text').textContent()) && /MPL-2\.0/.test(await win.locator('.notices-meta').textContent()));
    await win.locator('.form-input').fill('mpl');
    const shown = await win.locator('.notices-rows tbody tr:not([hidden])').count();
    ok('the filter narrows the list and says how many are shown', shown >= 1 && shown < total && (await win.locator('.notices-count').textContent()) === shown + ' of ' + total + ' components', String(shown));
    ok('nothing moved or changed size', await boxes() === before, before + ' :: ' + await boxes());
    await win.locator('.form-input').fill('');
    ok('clearing the filter shows them all again', await win.locator('.notices-rows tbody tr:not([hidden])').count() === total);

    await win.getByRole('button', { name: 'Copy This Notice', exact: true }).click();
    await p.waitForFunction(() => window.__runtime.some(c => c[0] === 'ClipboardSetText'));
    ok('Copy This Notice copies the chosen notice', /Mozilla Public License/.test(await p.evaluate(() => window.__runtime.find(c => c[0] === 'ClipboardSetText')[1])));
    ok('the copy button keeps its width while it says so', await boxes() === before);
    ok('Save All Notices is offered', await win.getByRole('button', { name: 'Save All Notices…', exact: true }).isEnabled());

    await t.shot('open-source-notices');
    // A second press of the button brings the same window forward.
    await p.evaluate(() => document.getElementById('about').showModal());
    await button.click();
    await p.waitForFunction(() => !document.getElementById('about').open);
    ok('the window is opened once', await p.locator('.win[data-win-kind="open-source-notices"]').count() === 1);
  } finally { await t.close(); }
})();
