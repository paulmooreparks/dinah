// What Dinah said in a chat (turns set aside to make room) is kept in the saved copy as a status row, and a chat
// reopened from its saved copy draws it once, where it was said.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      // The agent is slow to open, as it is in life, so the saved copy is what the page has to show meanwhile.
      let release;
      window.__gate = new Promise(r => { release = r; });
      window.__release = release;
      return {
      AgentProviders: async () => [window.__provider('ollama')],
      SavedConversation: async () => ({ available: true, complete: true, messages: [
        { role: 'you', text: 'First question.' },
        { role: 'agent', text: 'First answer.' },
        { role: 'status', text: 'The earliest 3 turns are set aside, and the rest of the chat goes on.', kind: 'warning' },
        { role: 'you', text: 'Second question.' }
      ] }),
      OpenSessionAt: async (ws, id) => { await window.__gate; return { sessionId: id, provider: 'ollama', cwd: ws, open: true }; }
      };
    },
  });
  const { page, ok } = t;
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'ollama:saved' }));
    const win = page.locator('[data-win="' + key + '"]');
    // The log shows when the whole copy has been drawn, so its last row is what to wait for.
    await win.locator('[data-view-id="logs"]').first().getByText('Second question.').waitFor();
    const text = await win.locator('[data-view-id="logs"]').first().innerText();
    ok('a chat drawn from its saved copy has the marker once', text.split('earliest 3 turns').length === 2, text);
    ok('and it is where it was said, between the two turns', /First answer\.\s*The earliest 3 turns[^]*Second question/.test(text), text);
    ok('and a warning is still drawn as a warning', await win.locator('.status-warning', { hasText: 'earliest 3 turns' }).count() === 1);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
