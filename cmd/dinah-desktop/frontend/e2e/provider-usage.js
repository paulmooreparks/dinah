const { launch } = require('./lib/page');
const { choose } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'codex', usagePage: 'https://devin.example/usage' },
    app: () => {
      window.__requests = []; window.__opened = [];
      const info = id => ({ sessionId: id, title: id, cwd: 'C:/Test', provider: id.startsWith('Codex') ? 'codex' : 'devin', open: false });
      return {
        HostLocale: async () => 'en-GB-u-hc-h23',
        AgentProviders: async () => ['devin', 'codex'].map(id => ({ id, name: id === 'devin' ? 'Devin' : 'Codex', installed: true, available: true })),
        Sessions: async () => ['Codex One', 'Codex Two', 'Devin One'].map(info),
        OpenSessionAt: async (workspace, id) => ({ ...info(id), open: true }),
        ProviderUsage: provider => new Promise((resolve, reject) => window.__requests.push({ provider, resolve, reject })),
        OpenProviderUsage: async provider => { window.__opened.push(provider); }
      };
    },
  });
  const { page, ok } = t;
  try {
    const front = () => page.locator('.win.active');
    const menu = () => front().locator('[data-view-id="usage-menu"]');
    const account = () => front().locator('[data-view-id="account-usage"]');
    async function open(id) {
      await page.evaluate(() => dinahWorkbench.open('workspace'));
      await page.locator('#workspace-chats [data-session="' + id + '"]').click();
      await front().locator('[data-view-id="session-title"]').getByText(id, { exact: true }).waitFor();
    }
    async function requestUsage() {
      const count = await page.evaluate(() => __requests.length);
      await front().locator('[data-view-id="usage-button"]').click();
      await page.waitForFunction(n => __requests.length > n, count); return count;
    }
    async function finish(index, value, error) {
      await page.evaluate(({ index, value, error }) => { const request = __requests[index]; error ? request.reject(new Error(error)) : request.resolve(value); }, { index, value, error });
    }
    await open('Codex One');
    await page.evaluate(() => __emit('agent:update', { sessionId: 'Codex One', update: { sessionUpdate: 'usage_update', used: 11000, size: 100000 } }));
    let request = await requestUsage();
    ok('the account section loads independently of current context', await account().getByText('Loading Codex account usage…', { exact: true }).isVisible() && (await front().locator('[data-view-id="usage-detail"]').textContent()).includes('(11%)'));
    const codexUsage = { provider: 'codex', status: 'available', accountType: 'chatgpt', accountLabel: 'ChatGPT', planType: 'pro', checkedAt: 1791532800, url: 'https://chatgpt.com/codex/settings/usage', buckets: [
      { id: 'codex', name: 'Codex', primary: { usedPercent: 75, windowDurationMins: 300, resetsAt: 1791547200 }, secondary: { usedPercent: 33, windowDurationMins: 10080, resetsAt: 1792137600 } },
      { id: 'additional', name: 'Additional Reported Limit', primary: { usedPercent: 0, windowDurationMins: 90, resetsAt: 1791540000 } }
    ] };
    await finish(request, codexUsage);
    await account().getByText('75% used', { exact: true }).waitFor();
    ok('all reported buckets and durations are shown separately from context', (await account().textContent()).includes('Additional Reported Limit') && (await account().textContent()).includes('5 Hour Window') && (await account().textContent()).includes('7 Day Window') && (await account().textContent()).includes('0% used'));
    ok('quota windows show their reset times in the regional format without invented daily or weekly names', (await account().textContent()).includes(await page.evaluate(() => new Intl.DateTimeFormat('en-GB-u-hc-h23', { year: 'numeric', month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(1791547200 * 1000)))) &&!/daily|weekly/i.test(await account().textContent()));
    await t.shot('provider-usage-codex');
    await menu().locator('[data-cmd="usage-page"]').click();
    ok('the Codex panel opens Codex account usage', await page.evaluate(() => __opened.at(-1) === 'codex'));
    await page.setViewportSize({ width: 900, height: 600 });
    request = await requestUsage();
    await finish(request, codexUsage);
    await account().getByText('75% used', { exact: true }).waitFor();
    const popupFits = await menu().evaluate(el => {
      const bounds = el.getBoundingClientRect();
      return bounds.top >= 0 && bounds.bottom <= innerHeight && el.scrollHeight > el.clientHeight;
    });
    ok('multiple account buckets scroll within a short viewport', popupFits);
    await t.shot('provider-usage-short');
    await menu().evaluate(el => el.hidePopover());
    await page.setViewportSize({ width: 1400, height: 1000 });
    request = await requestUsage();
    await open('Devin One');
    const devinRequest = await requestUsage();
    await finish(devinRequest, { provider: 'devin', status: 'external', detail: 'Devin reports account usage on the configured usage page.', url: 'https://devin.example/usage', buckets: [] });
    await account().getByText('Devin reports account usage on the configured usage page.', { exact: true }).waitFor();
    await finish(request, null, 'Late Codex failure');
    ok('a late response cannot replace another window account panel', !(await account().textContent()).includes('Codex') && await front().locator('[data-view-id="account-usage-title"]').textContent() === 'Devin Account Usage');
    await menu().locator('[data-cmd="usage-page"]').click();
    ok('Devin retains its configured account page', await page.evaluate(() => __opened.at(-1) === 'devin'));
    await menu().evaluate(el => { if (el.matches(':popover-open')) el.hidePopover(); });
    // The agent menu in the status area says the account usage of the agent
    // it is for, on the Usage… row's own line, and the row opens that
    // agent's usage page.
    await open('Codex One');
    const menuRequest = await page.evaluate(() => __requests.length);
    await page.locator('#status-agent').click();
    await page.waitForFunction(n => __requests.length > n, menuRequest);
    await finish(menuRequest, codexUsage);
    await page.locator('#agent-menu-usage').getByText(/^Codex: 75% of the 5 hour window used; 33% of the 7 day window used; 0% of the 90 minute window used$/).waitFor();
    ok('the menu for Codex says Codex\'s account usage, window by window', (await page.locator('#agent-menu-usage').textContent()).startsWith('Codex: 75% of the 5 hour window used; 33% of the 7 day window used') &&
      (await page.locator('#agent-menu [data-cmd="restart"]').textContent()).includes('Codex'));
    await page.locator('#agent-usage-row [data-cmd="usage-page"]').click();
    ok('Usage… for Codex opens Codex\'s usage page', await page.evaluate(() => __opened.at(-1) === 'codex'));
    await open('Devin One');
    await page.evaluate(() => document.querySelector('#agent-usage-row [data-cmd="usage-page"]').click());
    ok('Help follows the focused chat provider', await page.evaluate(() => __opened.at(-1) === 'devin'));
    await page.evaluate(() => dinahWorkbench.open('settings'));
    await page.evaluate(() => document.querySelector('#agent-usage-row [data-cmd="usage-page"]').click());
    ok('Help uses the default provider outside a chat', await page.evaluate(() => __opened.at(-1) === 'codex'));
    await page.evaluate(() => dinahWorkbench.open('chat'));
    request = await requestUsage();
    ok('draft chats can inspect account usage without a session', await page.evaluate(index => __requests[index].provider === 'codex', request) && (await front().locator('[data-view-id="usage-detail"]').textContent()).includes('Nothing reported yet'));
    await choose(page, front(), 'provider', 'Devin');
    if (!await menu().evaluate(el => el.matches(':popover-open'))) await requestUsage();
    await page.waitForFunction(() => __requests.at(-1).provider === 'devin');
    await finish(await page.evaluate(() => __requests.length - 1), { provider: 'devin', status: 'external', detail: 'Draft Devin usage.', url: 'https://devin.example/usage', buckets: [] });
    await finish(request, { provider: 'codex', status: 'available', detail: 'STALE CODEX DATA', buckets: [] });
    await account().getByText('Draft Devin usage.', { exact: true }).waitFor();
    ok('changing the draft provider discards the previous request', !(await account().textContent()).includes('STALE'));
    await choose(page, front(), 'provider', 'Codex');
    if (!await menu().evaluate(el => el.matches(':popover-open'))) await requestUsage();
    await account().getByText('Loading Codex account usage…', { exact: true }).waitFor();
    ok('changing provider clears an already-loaded account snapshot', !(await account().textContent()).includes('Draft Devin usage.') && await front().locator('[data-view-id="account-usage-link"]').isHidden());
    await open('Codex Two');
    request = await requestUsage();
    await finish(request, { provider: 'codex', status: 'available', buckets: [{ id: 'missing', name: 'Incomplete Report', primary: { usedPercent: null, windowDurationMins: null, resetsAt: null }, secondary: {} }] });
    await account().getByText('Usage not reported', { exact: true }).first().waitFor();
    ok('missing quota numbers stay unknown instead of becoming zero', !(await account().textContent()).includes('0%') && (await account().textContent()).includes('Duration Not Reported') && await front().locator('[data-view-id="account-usage-link"]').isHidden());
    await menu().evaluate(el => el.hidePopover());
    request = await requestUsage();
    await finish(request, { provider: 'codex', status: 'external', accountType: 'apiKey', accountLabel: 'API Key (Separate Billing)', linkLabel: 'Open OpenAI API Usage', detail: 'API usage is billed separately.', url: 'https://platform.openai.com/usage', buckets: [] });
    await account().getByText('API Key (Separate Billing)', { exact: true }).waitFor();
    ok('API-key accounts are identified without fabricated subscription quotas', !(await account().textContent()).includes('% used') && await front().locator('[data-view-id="account-usage-link"]').getByText('Open OpenAI API Usage', { exact: true }).isVisible());
    await menu().evaluate(el => el.hidePopover());
    request = await requestUsage();
    await finish(request, null, 'Sign in is required.');
    await account().getByText(/Account usage could not be loaded/).waitFor();
    ok('unavailable account usage does not fall back to Devin', await front().locator('[data-view-id="account-usage-link"]').isHidden() && !(await account().textContent()).includes('Devin'));
    await t.shot('provider-usage-unavailable');
  } finally { await t.close(); }
})();
