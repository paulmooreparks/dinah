const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1400 },
    app: () => {
      const result = (id, status, action, outputJSON) => ({ id, status, action, outputJSON, version: 1, workspace: 'C:/Fixture', ownerKind: 'chat', ownerId: 'chat', error: status === 'failed' ? 'The dice could not be rolled.' : '' });
      const results = [
        result('failed', 'failed', 'parks.dice/roll@1', ''),
        result('timeout', 'timeout', 'dinah.notifications/request@1', ''),
        result('notice', 'done', 'dinah.notifications/post@1', JSON.stringify({ title: 'Review Ready', body: 'The document is ready to read.' })),
        result('success', 'done', 'parks.dice/roll@1', ''),
        result('cancelled', 'cancelled', 'dinah.notifications/request@1', '')
      ];
      return {
        AgentProviders: async () => [{ id: 'codex', installed: true, available: true }],
        AppletAttention: async () => { window.__attentionLoads = (window.__attentionLoads || 0) + 1; return { requests: [], results }; },
        DismissAppletResultAt: async () => {}, Answer: async () => {}
      };
    },
    ready: () => document.querySelector('#applet-result-success'),
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => {
      __emit('agent:session', { sessionId: 'chat', provider: 'codex', cwd: 'C:/Fixture', title: 'Notification Example', open: true });
      __emit('agent:permission', { sessionId: 'chat', key: 'approval', toolCall: { title: 'Read the selected page' }, options: [{ optionId: 'once', name: 'Allow Once', kind: 'allow_once' }] });
      __emit('agent:turn', { sessionId: 'chat', stopReason: 'error', error: 'The agent connection ended unexpectedly.' });
    });
    await page.evaluate(async () => { await dinahWorkbench.open('needs-you'); pudlWindows.snap('needs-you', 'maximized'); });
    const failedTurn = page.locator('#questions .heads-up:not(.conn-notice)');
    await failedTurn.waitFor();
    ok('a failed turn is classified as an error', await failedTurn.getAttribute('data-severity') === 'error' && await failedTurn.evaluate(el => el.classList.contains('danger')));
    ok('failed applet actions are errors', await page.locator('#applet-result-failed').getAttribute('data-severity') === 'error');
    ok('approval, timeout and cancellation cards are warnings', await page.locator('#question-approval').getAttribute('data-severity') === 'warning' && await page.locator('#applet-result-timeout').getAttribute('data-severity') === 'warning' && await page.locator('#applet-result-cancelled').getAttribute('data-severity') === 'warning');
    ok('posted notifications remain neutral and completed actions are positive', await page.locator('#applet-result-notice').getAttribute('data-severity') === 'info' && await page.locator('#applet-result-success').getAttribute('data-severity') === 'success');
    for (const theme of ['light', 'dark']) {
      await page.evaluate(theme => pudlSetTheme(theme), theme);
      const styles = await page.evaluate(() => {
        const values = {};
        for (const severity of ['error', 'warning', 'info', 'success']) {
          const card = document.querySelector('.question.notification-' + severity), style = getComputedStyle(card);
          const probe = document.createElement('span'); probe.style.color = 'var(' + ({ error: '--danger', warning: '--warn', info: '--accent', success: '--positive' }[severity]) + ')'; card.appendChild(probe);
          values[severity] = { edge: style.borderInlineStartColor, token: getComputedStyle(probe).color, background: style.backgroundColor, glyph: getComputedStyle(card, '::before').maskImage };
          probe.remove();
        }
        return values;
      });
      ok(theme + ' notifications use the brand severity edges and distinct status glyphs', Object.values(styles).every(style => style.edge === style.token && style.glyph !== 'none') && new Set(Object.values(styles).map(style => style.glyph)).size === 4);
      ok(theme + ' severity backgrounds remain distinct', new Set(Object.values(styles).map(style => style.background)).size === 4);
      await t.shot('notification-severity-' + theme);
    }
    // A burst of applet changes, such as a script calling an action in a
    // loop, makes one load of Notifications' applet entries.
    const loads = await page.evaluate(async () => {
      const before = window.__attentionLoads;
      for (let i = 0; i < 25; i++) __emit('applet:invocation', {});
      await new Promise(resolve => setTimeout(resolve, 600));
      return window.__attentionLoads - before;
    });
    ok('a burst of applet changes makes one load', loads === 1);
  } finally { await t.close(); }
})();
