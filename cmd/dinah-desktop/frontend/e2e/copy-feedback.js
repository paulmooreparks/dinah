const { launch } = require('./lib/page');
(async () => {
  const t = await launch();
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => {
      window.__copyNotices = [];
      dinahHost.toast = (text, kind) => __copyNotices.push({ text, kind });
      runtime.ClipboardSetText = async text => { window.__copyValue = text; return true; };
      const button = document.createElement('button');
      button.id = 'copy-fixture'; button.className = 'btn'; button.textContent = 'Copy path';
      button.setAttribute('aria-label', 'Copy path');
      document.body.appendChild(button);
      button.onclick = () => dinahChatText.copy('C:/Fixture/a.txt', '', button);
    });
    const b = p.locator('#copy-fixture'), before = await b.boundingBox();
    await b.click();
    await p.waitForFunction(() => document.querySelector('[role="status"].visually-hidden')?.textContent === 'Copied');
    const after = await b.boundingBox();
    ok('copy confirms locally without changing geometry', before.width === after.width && before.height === after.height && await b.getAttribute('aria-label') === 'Copied');
    ok('copy writes the exact value without success notices', await p.evaluate(() => __copyValue === 'C:/Fixture/a.txt' && __copyNotices.length === 0));
    await b.click();
    ok('repeated copy has only one confirmation', await b.locator('.copy-feedback').count() === 1);
    await p.waitForFunction(() => document.querySelector('#copy-fixture').getAttribute('aria-label') === 'Copy path');
    ok('copy restores the original button', await b.textContent() === 'Copy path' && await b.locator('.copy-feedback').count() === 0);
    await p.evaluate(() => dinahChatText.copy('menu value'));
    ok('menu copy has no visible confirmation or notification', await p.locator('.copy-feedback').count() === 0 && await p.evaluate(() => __copyNotices.length === 0));
    for (const mode of ['false', 'reject', 'throw']) {
      const result = await p.evaluate(async mode => {
        runtime.ClipboardSetText = mode === 'false' ? () => false : mode === 'reject' ? () => Promise.reject(new Error('Denied')) : () => { throw new Error('Unavailable'); };
        return dinahChatText.copy('value', '', document.querySelector('#copy-fixture'));
      }, mode);
      ok(mode + ' clipboard failure reports an error without success feedback', result === false && await p.locator('.copy-feedback').count() === 0 && await p.evaluate(() => __copyNotices.at(-1).kind === 'danger'));
    }
  } finally { await t.close(); }
})();
