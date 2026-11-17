// Settings, Agents, Applet Actions Allowed Always: one group for each
// workspace, its name and folder first, then its actions as a table of one
// line each with the applet each belongs to, a Forget button on every line
// and Forget All for the workspace. They were lines of "action in folder"
// with a button after each, which broke wherever the folder's length put
// the break (Paul, 17 November 2026; the approved mockup is
// docs/mockups/applet-actions-allowed.html).
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      const long = 'C:\\Users\\sam\\Documents\\A Very Long Folder Name for Work\\customers\\parser', site = 'C:\\Work\\website';
      window.__grants = [
        { workspace: long, action: 'fixture.slides/load@1', title: 'Load a Deck', applet: 'Slides' },
        { workspace: long, action: 'fixture.slides/export-picture@1', title: 'Export a Slide as a Picture With a Long Name', applet: 'Slides' },
        { workspace: long, action: 'fixture.files/open@1', title: 'Open a File', applet: 'Files' },
        { workspace: long, action: 'fixture.editor/open@1', title: 'Open a File', applet: 'Editor' },
        { workspace: site, action: 'fixture.chats/ask@1', title: 'Ask Another Chat', applet: 'Chats' },
        { workspace: site, action: 'fixture.gone/act@1', title: 'fixture.gone/act@1', applet: '' },
      ];
      window.__forgot = [];
      return {
        AppletGrants: async () => window.__grants.slice(),
        ForgetAppletGrant: async (workspace, action) => {
          if (window.__refuse === action) throw new Error('The settings file is locked.');
          window.__forgot.push(action);
          window.__grants = window.__grants.filter(g => !(g.workspace === workspace && g.action === action));
        },
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.waitForFunction(() => !document.body.hasAttribute('aria-busy'));
    await p.evaluate(() => dinahWorkbench.open('settings'));
    await p.locator('#settings-tab-agents').click();
    const card = p.locator('#applet-grants');
    await card.locator('.grants-group').first().waitFor();
    await card.scrollIntoViewIfNeeded();
    const groups = card.locator('.grants-group');
    ok('there is one group for each workspace', await groups.count() === 2);
    const first = groups.nth(0), second = groups.nth(1);
    ok('a group says its workspace, how many actions it has, and its whole folder in a tooltip', /parser\s*4$/.test((await first.locator('.grants-name').innerText()).replace(/\n/g, ' ')) && /A Very Long Folder Name/.test(await first.locator('.grants-folder').getAttribute('title')), await first.locator('.grants-name').innerText());
    ok('its actions are a table with the applet each belongs to', (await first.locator('tbody tr').allInnerTexts()).map(r => r.replace(/\s+/g, ' ').trim()).join('|') === 'Load a Deck Slides Forget|Export a Slide as a Picture With a Long Name Slides Forget|Open a File Files Forget|Open a File Editor Forget', (await first.locator('tbody tr').allInnerTexts()).join('|'));
    ok('every line has its Forget button in sight, each with a name of its own', await first.locator('tbody .btn').count() === 4 && new Set(await first.locator('tbody .btn').evaluateAll(bs => bs.map(b => b.getAttribute('aria-label')))).size === 4);
    ok('an action whose applet is gone is still listed, with no applet', (await second.locator('tbody tr').nth(1).innerText()).replace(/\s+/g, ' ').trim() === 'fixture.gone/act@1 Forget');
    // Each line is one row high, and nothing runs out of the card.
    const shape = await card.evaluate(c => { const rows = [...c.querySelectorAll('tbody tr')].map(r => Math.round(r.getBoundingClientRect().height)), box = c.getBoundingClientRect(); return { rows, inside: [...c.querySelectorAll('.btn, .grants-folder, td')].every(e => { const r = e.getBoundingClientRect(); return r.left >= box.left - 1 && r.right <= box.right + 1; }), buttons: [...c.querySelectorAll('tbody .btn')].map(b => Math.round(b.getBoundingClientRect().right)) }; });
    ok('nothing runs out of the card, and the Forget buttons stand in one column', shape.inside && new Set(shape.buttons).size === 1, JSON.stringify(shape));
    await t.shot('applet-grants', { clip: await card.boundingBox() });

    // Forget takes one action off and says so.
    await first.getByRole('button', { name: 'Forget Open a File of Files in parser' }).click();
    await p.waitForFunction(() => document.querySelectorAll('#applet-grants .grants-group')[0].querySelectorAll('tbody tr').length === 3);
    ok('Forget forgets that action alone', JSON.stringify(await p.evaluate(() => __forgot)) === '["fixture.files/open@1"]' && /Open a File will be asked about again in parser\./.test(await p.locator('#applet-grants-status').innerText()));
    // A failure says why and leaves the line.
    await p.evaluate(() => { window.__refuse = 'fixture.slides/load@1'; });
    await card.locator('.grants-group').nth(0).getByRole('button', { name: 'Forget Load a Deck of Slides in parser' }).click();
    await p.locator('#applet-grants-status.notice.danger').waitFor();
    ok('an action that could not be forgotten stays, with the reason', await card.locator('.grants-group').nth(0).locator('tbody tr').count() === 3 && /The settings file is locked/.test(await p.locator('#applet-grants-status').innerText()));
    await p.evaluate(() => { window.__refuse = null; });

    // Forget All asks first, and Cancel changes nothing.
    await card.locator('.grants-group').nth(1).getByRole('button', { name: 'Forget all actions in website' }).click();
    const dialog = p.locator('#ask[open]');
    await dialog.waitFor();
    ok('Forget All asks, saying how many and where', /Forget All 2 Actions in website\?/.test(await dialog.innerText()));
    await dialog.getByRole('button', { name: 'Cancel' }).click();
    ok('Cancel forgets nothing', await p.evaluate(() => __forgot.length) === 1);
    await card.locator('.grants-group').nth(1).getByRole('button', { name: 'Forget all actions in website' }).click();
    await p.locator('#ask[open]').getByRole('button', { name: 'Forget All', exact: true }).click();
    await p.waitForFunction(() => document.querySelectorAll('#applet-grants .grants-group').length === 1);
    ok('Forget All forgets every action of that workspace and no other', JSON.stringify(await p.evaluate(() => __forgot.slice(1))) === '["fixture.chats/ask@1","fixture.gone/act@1"]' && await p.evaluate(() => __grants.length) === 3);

    // With nothing left the card says so.
    await p.evaluate(() => { window.__grants = []; });
    await card.locator('.grants-group').nth(0).locator('tbody .btn').first().click().catch(() => {});
    await card.locator('.empty-state').waitFor();
    ok('with no action allowed always the card says so', /No Action Is Allowed Always/.test(await card.innerText()));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
