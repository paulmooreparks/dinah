const { launch } = require('./lib/page');
const path = require('path');
const fs = require('fs');
(async () => {
  const t = await launch({
    viewport: { width: 1360, height: 920 },
    app: () => ({
      AgentProviders: async () => [{ id: 'codex', installed: true, available: true }],
      SavedConversation: async () => ({ available: false }),
      OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'codex', title: 'Dinah Development', open: true })
    }),
    ready: 'chat',
  });
  const { page, ok } = t;
  try {
    await page.evaluate(async () => {
      const key = await dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:appearance' });
      pudlWindows.snap(key, 'maximized');
      __emit('agent:update', { sessionId: 'codex:appearance', update: { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Use Mineral for the default light and dark palettes.' } } });
      __emit('agent:update', { sessionId: 'codex:appearance', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Mineral uses tinted content surfaces, blue accents and clearer dark-mode boundaries.\n\nLinen remains the alternative for the planned theme selector.\n\n```js\nconst theme = "Mineral";\n```' } } });
    });
    await page.locator('.win[data-chat-current] .msg-text').filter({ hasText: 'Mineral uses tinted' }).waitFor();
    const prompt = page.locator('.win[data-chat-current] [data-view-id=prompt]');
    await prompt.fill('Keep this draft while changing the theme.');
    for (const theme of ['light', 'dark']) {
      await page.evaluate(value => pudlSetTheme(value), theme);
      await page.evaluate(() => {
        const button = document.createElement('button'); button.id = 'appearance-danger'; button.className = 'btn btn-danger'; button.textContent = 'Remove';
        button.style.cssText = 'position:fixed;bottom:40px;right:16px;z-index:9999'; document.body.append(button);
      });
      await page.locator('#appearance-danger').hover();
      const result = await page.evaluate(() => {
        function color(token) { const probe = document.createElement('span'); probe.style.color = 'var(' + token + ')'; document.body.append(probe); const value = getComputedStyle(probe).color; probe.remove(); return value; }
        const canvas = document.createElement('canvas'), ctx = canvas.getContext('2d');
        function luminance(value) { ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1); const rgb = [...ctx.getImageData(0, 0, 1, 1).data].slice(0, 3).map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; }); return rgb.reduce((sum, value, i) => sum + value * [.2126, .7152, .0722][i], 0); }
        function ratio(a, b) { const x = luminance(a), y = luminance(b); return (Math.max(x, y) + .05) / (Math.min(x, y) + .05); }
        const backgrounds = ['--bg', '--surface', '--surface-alt', '--input-bg'].map(color);
        const tokens = ['--text', '--text-muted', '--accent', '--warn', '--danger', '--positive', '--dinah-skill', ...['keyword', 'string', 'number', 'comment', 'name', 'tag', 'attr', 'heading', 'link', 'error'].map(name => '--syntax-' + name)];
        const failures = [];
        tokens.forEach(token => backgrounds.forEach(background => { const n = ratio(color(token), background); if (n < 4.5) failures.push(token + ': ' + n.toFixed(2)); }));
        const doc = document.createElement('div'); doc.className = 'doc-page'; document.body.append(doc);
        const docColor = getComputedStyle(doc).backgroundColor; doc.remove();
        const dangerButton = document.getElementById('appearance-danger'), dangerText = getComputedStyle(dangerButton).color;
        const dangerProbe = document.createElement('span'); dangerProbe.style.color = 'color-mix(in srgb, var(--danger) 78%, #fff)'; dangerButton.append(dangerProbe);
        const dangerHoverRatio = ratio(dangerText, getComputedStyle(dangerProbe).color); dangerProbe.style.color = 'var(--danger)';
        const dangerBaseRatio = ratio(dangerText, getComputedStyle(dangerProbe).color); dangerButton.remove();
        return { failures, backgrounds, docColor, surface: color('--surface'), body: getComputedStyle(document.body).backgroundColor, text: color('--text'), prompt: getComputedStyle(document.querySelector('.win[data-chat-current] [data-view-id=prompt]')).backgroundColor,
          dangerHoverRatio, dangerBaseRatio,
          actionRatio: ratio(color('--on-accent'), color('--accent')), warningRatio: ratio(color('--on-warn'), color('--warn')), edgeRatio: ratio(color('--input-border'), color('--input-bg')), menuAccent: (() => { const el = document.createElement('div'); el.className = 'menu-panel'; el.style.color = 'var(--accent)'; document.body.append(el); const value = getComputedStyle(el).color; el.remove(); return value; })(), accent: color('--accent') };
      });
      ok(theme + ' semantic text and syntax colors meet 4.5:1 across application surfaces', result.failures.length === 0);
      if (result.failures.length) console.log(JSON.stringify(result.failures));
      ok(theme + ' action text and warning badge text meet 4.5:1', result.actionRatio >= 4.5 && result.warningRatio >= 4.5);
      ok(theme + ' destructive hover text meets 4.5:1 at both gradient stops', result.dangerHoverRatio >= 4.5 && result.dangerBaseRatio >= 4.5);
      ok(theme + ' input boundaries meet 3:1', result.edgeRatio >= 3);
      ok(theme + ' document and composer use tinted theme surfaces', result.docColor === result.surface && result.prompt !== 'rgb(255, 255, 255)' && !result.backgrounds.includes('rgb(255, 255, 255)'));
      ok(theme + ' menus inherit the shared accent', result.menuAccent === result.accent);
      ok(theme + ' palette preserves the composer draft', await prompt.inputValue() === 'Keep this draft while changing the theme.');
      ok(theme + ' workspace matches the approved Mineral palette', result.body === (theme === 'light' ? 'rgb(230, 236, 238)' : 'rgb(12, 20, 27)'));
      await t.shot('appearance-' + theme);
    }
    ok('desktop and bus page share the same default palette', fs.readFileSync(path.join(__dirname, '../brand.css'), 'utf8') === fs.readFileSync(path.join(__dirname, '../../../../internal/web/static/brand.css'), 'utf8'));
    await page.evaluate(() => pudlSetTheme('system'));
    await page.emulateMedia({ colorScheme: 'light' });
    await page.waitForFunction(() => document.documentElement.dataset.theme === 'light');
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.waitForFunction(() => document.documentElement.dataset.theme === 'dark');
    ok('system preference still follows operating-system appearance', await page.evaluate(() => pudlThemePreference()) === 'system');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
