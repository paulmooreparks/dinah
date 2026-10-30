// A skill's parameters as a form, from the skills' argument hints.
const { launch } = require('./lib/page');
const { openMenu } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const skills = [
        { name: 'release-notes', description: 'Write the release notes for a version.', hint: '<version> <area> "<release title>"', source: 'workspace' },
        { name: 'edit-request', description: 'Turn a request to change a document into a written edit request.', hint: '["<document.md>"] <the change, in your words>', source: 'workspace' },
        { name: 'publish-notes', description: 'Write the notes back to their file.', hint: '"<working-copy.md>" [apply|suggest]', source: 'workspace' },
        { name: 'project-init', description: 'Start a new project.', hint: '<name> [--folder <path>] [--scaffold]', source: 'workspace' },
        { name: 'compact', description: 'Force conversation compaction', source: 'devin' }];
      const ws = 'C:\\x\\parser';
      return {
        State: async () => ({ workspace: ws, recent: [ws], approvals: 'dinah', devin: 'devin.exe', catalog: [], defaults: {}, pinned: [], titles: {} }),
        AgentProviders: async () => [window.__provider('devin')],
        // The fixture defines SkillsForProviderAt, which the page prefers to
        // SkillsAt when it exists, so both give this suite's skills.
        SkillsAt: async () => skills, SkillsForProviderAt: async () => skills,
        NewSessionAt: async () => ({ sessionId: 's1', cwd: ws, title: '', open: true, working: false, queued: [] }),
        Send: async (id, t) => { window.__calls.push(t); }, SetSessionOption: async () => []
      };
    },
    init: () => { window.__calls = []; },
  });
  const p = require('./lib/windows')(t.page);
  const { ok } = t;
  const last = () => p.evaluate(() => window.__calls[window.__calls.length - 1]);

  // 1. /release-notes from the pop-up, with a document open in the Editor,
  // which tells the chat its draft and path.
  await p.evaluate(() => document.dispatchEvent(new CustomEvent('dinah:item-context', { detail: { kind: '', path: 'notes/Release Plan.md', title: 'Release Plan', draftId: 'd1', selection: '' } })));
  await p.waitForTimeout(200);
  ok('the Editor\'s document is the chat\'s context', (await p.textContent('.win[data-chat-current] [data-view-id=context-chip]')) === 'Document: Release Plan', await p.textContent('.win[data-chat-current] [data-view-id=context-chip]'));
  await p.click('.win[data-chat-current] [data-view-id=prompt]');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', '/release-n');
  await p.keyboard.press('Enter');
  await p.waitForTimeout(150);
  ok('choosing a skill with a hint opens its form', await p.isVisible('.win[data-chat-current] [data-view-id=skill-form]'));
  const labels = await p.$$eval('.win[data-chat-current] [data-view-id=skill-form-fields] .skill-field > label', ls => ls.map(l => l.textContent.trim()));
  ok('one field per parameter', JSON.stringify(labels) === JSON.stringify(['version', 'area', 'release title']), JSON.stringify(labels));
  ok('focus waits in the first empty field', await p.evaluate(() => document.activeElement && document.activeElement.id.endsWith('-skill-field-0')));
  await p.fill('.win[data-chat-current] [id$=-skill-field-0]', '2.4');
  await p.fill('.win[data-chat-current] [id$=-skill-field-1]', 'CLI');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForTimeout(100);
  ok('an empty required field is marked, nothing is sent', (await p.$$('.win[data-chat-current] [data-view-id=skill-form] .skill-field.missing')).length === 1 && (await p.evaluate(() => window.__calls.length)) === 0);
  await p.fill('.win[data-chat-current] [id$=-skill-field-2]', 'Config Reload');
  ok('the preview shows the command', (await p.textContent('.win[data-chat-current] [data-view-id=skill-form-preview]')) === '/release-notes 2.4 CLI "Config Reload"');
  await t.shot('form-release');
  await p.keyboard.press('Enter');
  await p.waitForTimeout(300);
  ok('Enter sends the assembled command', (await last()) === '/release-notes 2.4 CLI "Config Reload"', await last());
  ok('and the form closes', await p.isHidden('.win[data-chat-current] [data-view-id=skill-form]'));

  // 2. /edit-request: the document field and the box as "the change".
  await p.evaluate(() => window.__emit('agent:session', { sessionId: 's1', cwd: 'C:\\x\\parser', title: 'x', open: true, working: false, queued: [] }));
  await (await openMenu(p, p.locator('.win[data-chat-current]'), 'skills')).locator('.menu-action', { hasText: '/edit-request' }).first().click();
  await p.waitForTimeout(150);
  ok('the box becomes "the change"', (await p.getAttribute('.win[data-chat-current] [data-view-id=prompt]', 'placeholder')) === 'The change, in your words');
  await p.fill('.win[data-chat-current] [id$=-skill-field-0]', 'notes/Release Plan.md');
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Add the retry interval to Open Issues.');
  await t.shot('form-edit');
  await p.keyboard.press('Control+Enter');
  await p.waitForTimeout(200);
  ok('the document and the words make the command', (await last()) === '/edit-request "notes/Release Plan.md" Add the retry interval to Open Issues.', await last());

  // 3. /publish-notes: a choice.
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', '/publish-n');
  await p.keyboard.press('Tab');
  await p.waitForTimeout(150);
  const choices = await p.$$eval('.win[data-chat-current] [id$=-skill-field-1] option', os => os.map(o => o.textContent));
  ok('apply|suggest becomes a choice', JSON.stringify(choices) === JSON.stringify(['(let the skill ask)', 'apply', 'suggest']), JSON.stringify(choices));
  await p.fill('.win[data-chat-current] [id$=-skill-field-0]', 'notes/plan.md');
  await p.selectOption('.win[data-chat-current] [id$=-skill-field-1]', 'suggest');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForTimeout(200);
  ok('the choice is sent', (await last()) === '/publish-notes "notes/plan.md" suggest', await last());

  // 4. /project-init: a flag with a value and a tick box.
  await p.type('.win[data-chat-current] [data-view-id=prompt]', '/project-i');
  await p.keyboard.press('Enter');
  await p.waitForTimeout(150);
  await p.fill('.win[data-chat-current] [id$=-skill-field-0]', 'Lexer');
  await p.fill('.win[data-chat-current] [id$=-skill-field-1]', 'Projects/Lexer 2026');
  await p.check('.win[data-chat-current] [id$=-skill-field-2]');
  await t.shot('form-init');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForTimeout(200);
  ok('flags are sent with their values', (await last()) === '/project-init Lexer --folder "Projects/Lexer 2026" --scaffold', await last());

  // 5. Esc: back to typing by hand, with what the form held.
  await p.type('.win[data-chat-current] [data-view-id=prompt]', '/release-n');
  await p.keyboard.press('Enter');
  await p.waitForTimeout(150);
  await p.fill('.win[data-chat-current] [id$=-skill-field-0]', '2.5');
  await p.focus('.win[data-chat-current] [data-view-id=prompt]');
  await p.keyboard.press('Escape');
  await p.waitForTimeout(100);
  ok('Esc closes the form and leaves the command in the box', await p.isHidden('.win[data-chat-current] [data-view-id=skill-form]') && (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')).startsWith('/release-notes 2.5'), await p.inputValue('.win[data-chat-current] [data-view-id=prompt]'));

  // 6. A skill with no hint inserts as before.
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', '/comp');
  await p.keyboard.press('Enter');
  ok('a skill without a hint just inserts', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === '/compact ' && await p.isHidden('.win[data-chat-current] [data-view-id=skill-form]'));
  await t.close();
})();
