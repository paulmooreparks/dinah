const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'] },
    app: () => {
      const long = Array.from({ length: 60 }, (_, i) => 'word' + i).join(' ');
      window.__long = long;
      const history = [
        { kind: 'send', text: 'first chat, older', chat: 'codex:first' },
        { kind: 'send', text: 'second chat only', chat: 'codex:second' },
        { kind: 'send', text: long, chat: 'codex:first' },
        { kind: 'send', text: 'from before chats were recorded' }];
      return {
        History: async () => history,
        ChatHistory: async id => history.filter(e => e.chat === id).map(e => e.text),
        OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'codex', title: id, cwd: ws, open: true }),
        Send: async () => {}
      };
    },
    ready: () => window.dinahWorkbench && window.dinahEditor,
  });
  const { page, ok } = t;
  try {
    const first = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:first' }));
    const second = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:second' }));
    const long = await page.evaluate(() => __long);
    const box = key => page.locator('[data-win="' + key + '"] [data-view-id="prompt"]');
    const caret = key => box(key).evaluate(el => el.selectionStart);

    await page.evaluate(key => pudlWindows.raise(key), second);
    await box(second).focus();
    await box(second).press('ArrowUp');
    ok('Up brings back the chat\'s own prompt', await box(second).inputValue() === 'second chat only');
    await box(second).press('ArrowUp');
    ok('Up stops at the chat\'s oldest prompt instead of reaching another chat\'s', await box(second).inputValue() === 'second chat only');
    await box(second).press('ArrowDown');
    ok('Down goes back to what was being typed', await box(second).inputValue() === '');

    await page.evaluate(key => pudlWindows.raise(key), first);
    await box(first).focus();
    await box(first).press('ArrowUp');
    ok('another chat brings back its own latest prompt', await box(first).inputValue() === long);
    const rows = await box(first).evaluate(el => Math.round(el.scrollHeight / parseFloat(getComputedStyle(el).lineHeight)));
    ok('the long prompt wraps onto several lines', rows >= 2);
    await box(first).press('ArrowDown');
    ok('Down on the first of wrapped lines moves the caret instead of leaving the prompt', await box(first).inputValue() === long && await caret(first) > 0);
    await box(first).evaluate(el => el.setSelectionRange(el.value.length, el.value.length));
    await box(first).press('ArrowUp');
    ok('Up on the last of wrapped lines moves the caret to the line above', await box(first).inputValue() === long && await caret(first) < long.length);
    await box(first).evaluate(el => el.setSelectionRange(0, 0));
    await box(first).press('ArrowUp');
    ok('Up on the first line brings back the earlier prompt', await box(first).inputValue() === 'first chat, older');
    await box(first).press('ArrowUp');
    ok('prompts recorded before chats were known are not recalled', await box(first).inputValue() === 'first chat, older');

    await box(first).evaluate(el => { el.value = 'a new prompt'; el.dispatchEvent(new Event('input', { bubbles: true })); });
    await box(first).press('Control+Enter');
    await box(second).focus();
    await box(second).press('ArrowUp');
    ok('a prompt sent in one chat is not recalled in another', await box(second).inputValue() === 'second chat only');
  } catch (e) {
    console.error(e); t.failures.push('exception');
  } finally {
    await t.close();
  }
})();
