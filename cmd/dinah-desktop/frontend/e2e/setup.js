// The guided set-up (setup.js, setup.go; the approved mockup is
// docs/mockups/guided-setup): it opens by itself once, says of each agent
// whether it was found and whether the chosen one answers, asks the chosen
// agent's own choices for what it asks, takes answers from the person's own
// words, shows everything before it is set, and undoes what it set. Nothing
// in the window moves or changes size while this happens.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      window.__s = { shown: false, noted: 0, checks: [], signedIn: false, applied: [], undone: 0, pages: [], read: [] };
      const agents = [
        { id: 'alpha', name: 'Alpha', found: true, words: '', signIn: false, installPage: true, judges: true },
        { id: 'beta', name: 'Beta', found: true, words: '', signIn: true, installPage: true, judges: false },
        { id: 'gamma', name: 'Gamma', found: false, words: 'Its program was not found.', signIn: true, installPage: true, judges: true },
      ];
      return {
        State: async () => ({ workspace: '', recent: [], workspaceRecords: [] }),
        Sessions: async () => [],
        SetupShown: async () => window.__s.shown,
        NoteSetupShown: async () => { window.__s.shown = true; window.__s.noted++; },
        SetupOffer: async () => ({ shown: window.__s.shown, default: 'alpha', agents, personName: '', accountName: 'Alex Morgan', workspace: '', judgeOn: false, judgeName: 'Alpha', judgeCalls: 200,
          uses: [{ id: 'code', label: 'Writing code with an agent', windows: ['decisions', 'org-chart'], opens: 'Decisions and Org Chart' }, { id: 'docs', label: 'Writing documents and diagrams', windows: ['workspace'], opens: 'Workspace' }] }),
        AgentCheck: async id => {
          window.__s.checks.push(id);
          await new Promise(r => setTimeout(r, 150));
          if (id === 'beta' && !window.__s.signedIn) return { id, status: 'sign-in-required', detail: 'Nobody is signed in.' };
          return { id, status: 'connected', detail: '' };
        },
        SetupAsking: async id => id === 'alpha' ? { choices: [{ id: 'ask', label: 'Ask Before Commands' }, { id: 'full', label: 'Full Access, Unattended' }], current: 'ask' } : null,
        AgentSignIn: async () => { window.__s.signedIn = true; },
        OpenInstallPage: async id => { window.__s.pages.push(id); },
        ChooseImportFolder: async () => 'C:/Work/Pricing',
        ReadSetupWords: async (words, agent, open) => {
          window.__s.read.push({ words, agent, open: open.map(q => q.id) });
          return /nothing|^Sam$/.test(words) ? { answers: {}, used: 'rule' } : { answers: { ask: 'full', folder: 'C:/Work/Pricing' }, used: 'model' };
        },
        ApplySetup: async c => { window.__s.applied.push(c); },
        UndoSetup: async () => { window.__s.undone++; },
      };
    },
  });
  const { page: p, ok } = t;
  try {
    const win = p.locator('.win[data-win-kind="setup"]');
    await win.waitFor({ timeout: 15000 });
    ok('the set-up opens by itself and says so to the engine first', await p.evaluate(() => window.__s.noted) === 1);
    const where = await p.evaluate(() => { const f = document.querySelector('.win[data-win-kind="setup"]').getBoundingClientRect(), room = document.querySelector('[data-win-layer] > .win-inner').getBoundingClientRect(); return { mode: pudlWindows.state().place[pudlWindows.state().top].mode, top: pudlWindows.state().top, dx: Math.abs((f.left + f.right) / 2 - (room.left + room.right) / 2), dy: Math.abs((f.top + f.bottom) / 2 - (room.top + room.bottom) / 2), inside: f.left >= room.left - 1 && f.right <= room.right + 1 }; });
    ok('it floats in the middle of the room the docked windows leave, in front and wholly in sight', where.mode === 'floating' && /setup/.test(where.top) && where.dx < 2 && where.dy < 2 && where.inside, JSON.stringify(where));
    ok('its title has the product’s name', /^Set Up \S/.test((await win.locator('.win-title').textContent()).trim()));
    const rows = async () => (await win.locator('.setup-agents tr').allTextContents()).join(' | ');
    const next = win.locator('.setup-foot .btn-primary, .setup-foot .btn').last();
    const status = () => win.locator('.setup-status').textContent();
    const box = async sel => JSON.stringify(await win.locator(sel).boundingBox());
    await p.waitForFunction(() => /Answers/.test(document.querySelector('.setup-agents').textContent));
    const size = await box('.setup-body'), table = await box('.setup-agents');
    ok('each agent is found or not, the default one is chosen and tested, and one not found offers its install page', await rows() === 'AlphaAnswers | BetaFoundSign In… | GammaNot foundHow to Install', await rows());
    ok('an agent that was not found cannot be chosen', await win.locator('.setup-agents tr', { hasText: 'Gamma' }).locator('input').isDisabled());
    await win.locator('.setup-agents tr', { hasText: 'Gamma' }).getByRole('button').click();
    await p.waitForFunction(() => window.__s.pages.length === 1);
    ok('How to Install opens that agent’s page', (await p.evaluate(() => window.__s.pages)).join() === 'gamma');

    // An agent that does not answer cannot be continued with; signing in and testing again lets it.
    await win.locator('.setup-agents tr', { hasText: 'Beta' }).locator('input').check();
    await p.waitForFunction(() => /Does not answer/.test(document.querySelector('.setup-agents').textContent));
    ok('an agent that does not answer says why at the foot, and the step’s button tests it again', /Beta does not answer\. Nobody is signed in\./.test(await status()) && (await next.textContent()) === 'Test Again', await status());
    ok('the rows did not move or change size while an agent was tested', await box('.setup-agents') === table && await box('.setup-body') === size);
    await win.locator('.setup-agents tr', { hasText: 'Beta' }).getByRole('button').click();
    await p.waitForFunction(() => /BetaAnswers/.test(document.querySelector('.setup-agents').textContent));
    ok('after a sign-in the agent answers', (await next.textContent()) === 'Continue' && await next.isEnabled());

    // Beta has no setting for what it asks, and cannot answer in the background, so Alpha would.
    await next.click();
    const walk = async () => { const seen = []; for (let i = 0; i < 6 && await win.locator('.setup-question').isVisible(); i++) { seen.push(await win.locator('.setup-question').textContent()); await win.locator('.setup-choices .md-item').last().click(); await next.click(); } return seen; };
    const beta = await walk();
    ok('an agent with no setting for what it asks is not asked about it, and another agent answers in the background', beta.length === 4 && !beta.some(q => /ask you before/.test(q)) && /^May Alpha answer small questions/.test(beta[3]), JSON.stringify(beta));
    ok('review says what Undo cannot take back', /Agent\s*Beta · Step 1 · Undo does not sign you out of Beta/.test((await win.locator('.setup-review tbody tr').first().textContent())));

    // Back to step 1 for Alpha, whose own choices are offered.
    await win.locator('.setup-review tbody tr').first().getByRole('button').click();
    await win.locator('.setup-agents tr', { hasText: 'Alpha' }).locator('input').check();
    await p.waitForFunction(() => /AlphaAnswers/.test(document.querySelector('.setup-agents').textContent));
    await next.click();
    ok('the question in hand is the first one not answered, which is what the agent asks', (await win.locator('.setup-question').textContent()) === 'How much should Alpha ask you before it acts?' && (await win.locator('.setup-choices .md-item').allTextContents()).join('|') === 'Ask Before Commands|Full Access, Unattended');
    ok('Continue waits for a choice', await next.isDisabled());

    // The person's own words answer several questions at once.
    await win.locator('.setup-own textarea').fill('nothing useful');
    await p.keyboard.press('Enter');
    await win.locator('.setup-said .guide', { hasText: 'I could not take an answer from that.' }).waitFor();
    await win.locator('.setup-own textarea').fill('full access please, and my work is in C:/Work/Pricing');
    await p.keyboard.press('Enter');
    await win.locator('.setup-review').waitFor();
    const read = await p.evaluate(() => window.__s.read[1]);
    ok('the words go to the chosen agent with every question, so that what is said now replaces an earlier answer', read.agent === 'alpha' && read.open.join() === 'use,ask,folder,name,judge', JSON.stringify(read));
    ok('what was taken is said, and review follows when nothing is left', /From that I take before alpha acts: Full Access, Unattended; workspace: A workspace is made of C:\/Work\/Pricing\./.test(await win.locator('.setup-said').textContent()), await win.locator('.setup-said').textContent());
    const review = async () => (await win.locator('.setup-review tbody tr').allTextContents()).join(' | ');
    ok('review shows every setting with where it came from', /Before Alpha acts\s*Full Access, Unattended · What you said/.test(await review()) && /Workspace\s*A workspace is made of C:\/Work\/Pricing · What you said/.test(await review()) && /What agents call you\s*Alex Morgan · Your answer/.test(await review()), await review());
    ok('nothing is set before Set It Up', (await p.evaluate(() => window.__s.applied.length)) === 0 && (await next.textContent()) === 'Set It Up');

    // Change goes back to a question; a typed name is the name.
    await win.locator('.setup-review tbody tr', { hasText: 'What agents call you' }).getByRole('button').click();
    await win.locator('.setup-own textarea').fill('Sam');
    await p.keyboard.press('Enter');
    await win.locator('.setup-review').waitFor();
    ok('words typed at the question about the name are the name', /What agents call you\s*Sam · What you said/.test(await review()), await review());
    await win.locator('.setup-review tbody tr', { hasText: 'Small questions' }).getByRole('button').click();
    await win.locator('.setup-choices .md-item', { hasText: 'Yes' }).click();
    await next.click();
    await win.locator('.setup-review tbody tr', { hasText: 'Windows open' }).getByRole('button').click();
    await win.locator('.setup-choices .md-item').first().click();
    await next.click();
    ok('the row about the background says Undo turns it off', /On, at most 200 calls a day · Your answer · Undo turns this off/.test(await review()), await review());

    // Set It Up sets the answers and opens the windows; Undo puts both back.
    const before = await p.evaluate(() => pudlWindows.state().open.length);
    await next.click();
    await p.waitForFunction(() => /Everything is set/.test(document.querySelector('.setup-status').textContent));
    const sent = await p.evaluate(() => window.__s.applied[0]);
    ok('the engine is given the answers', JSON.stringify(sent) === JSON.stringify({ agent: 'alpha', asking: 'full', folder: 'C:/Work/Pricing', judge: true, setName: true, name: 'Sam' }), JSON.stringify(sent));
    ok('the windows chosen are open, and the set-up is in front of them', (await p.evaluate(() => pudlWindows.state().open.length)) === before + 2 && await p.evaluate(() => document.querySelector('.win.active').dataset.winKind) === 'setup');
    ok('Undo takes Set It Up’s place, and the way out says Close', (await next.textContent()) === 'Undo' && (await win.locator('.setup-foot .btn').first().textContent()) === 'Close');
    ok('the window kept its size through every step', await box('.setup-body') === size);
    await next.click();
    await p.waitForFunction(() => /back as they were/.test(document.querySelector('.setup-status').textContent));
    ok('Undo asks the engine and closes the windows the set-up opened', (await p.evaluate(() => window.__s.undone)) === 1 && (await p.evaluate(() => pudlWindows.state().open.length)) === before && (await next.textContent()) === 'Set It Up');

    // Skip closes the window; Help opens it again, and it does not open by itself a second time.
    await win.locator('.setup-foot .btn').first().click();
    await win.waitFor({ state: 'detached' });
    await p.evaluate(() => dinahCommands.run('setup'));
    await win.waitFor();
    ok('Help’s command opens the set-up again, and it was noted as shown once only', (await p.evaluate(() => window.__s.noted)) === 1);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
