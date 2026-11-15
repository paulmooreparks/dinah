const { launch } = require('./lib/page');
const { model } = require('./lib/org-chart-fixture');
const fs = require('node:fs'), path = require('node:path');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, args: model(), app: value => {
    window.__orgModel = value; window.__actions = []; window.__actionReads = []; window.__retained = false; window.__pageSuccess = false;
    const href = (kind, chat, group) => '?open=org-chart&view.org-chart=' + encodeURIComponent(new URLSearchParams(Object.assign({ action: kind, chat }, group ? { group } : {})).toString());
    return {
      SavedChats: async () => ({ generation: __orgModel.generation, chats: __orgModel.chats.map(row => ({ ...row, sessionId: row.head || row.chatId, cwd: row.workspace, chatState: row.state })) }),
      OrgChart: async () => JSON.parse(JSON.stringify(__orgModel)),
      OrgChartActions: async chatId => {
        __actionReads.push(chatId);
        const chats = __orgModel.chats.filter(c => c.resolution === 'resolved').map(c => ({ id: c.chatId, title: c.title }));
        const action = (kind, label, extra) => Object.assign({ kind, label, token: 'fresh-' + kind, href: href(kind, chatId), explanation: '', confirmation: '', chats: [], leads: [], groups: [], workspaces: [], providers: [] }, extra);
        return { chatId, title: (__orgModel.chats.find(c => c.chatId === chatId) || {}).title || 'Org Chart', actions: [
          action('assign_team', 'Assign to a Team', { chats: chats.filter(c => c.id === 'mara') }),
          action('replace_team_lead', 'Make Team Lead', { groups: [{id:'platform-channel', title:'Platform'}] }),
          action('make_lead', 'Make Lead', { groups: [{ id: 'review', title: 'Release Review' }].concat(__retained ? [{ id: 'retained-group', title: 'Retained Review' }] : []), explanation: 'Turn the selected ordinary group into a team led by this chat.' }),
          action('create_team', 'Create Team', { chats, leads: chats.filter(c => ['nina', 'uma'].includes(c.id)) }),
          action('create_group', 'Create Group', { chats }),
          action('start_chat', 'Start a Chat', { workspaces: [{ id: 'saved-harbor', title: 'Harbor' }], providers: [{ id: 'codex', title: 'Codex' }] }),
          action('delete_chat', 'Delete Chat…', { confirmation: 'Authoritative confirmation. The chat moves to the Recycle Bin for 30 days. Linked documents stay unchanged.' })
        ], unavailable: [] };
      },
      ApplyOrgChartAction: async request => {
        __actions.push(request);
        if (__pageSuccess) return {kind:request.kind,status:'succeeded',message:'The requested change is complete.',members:[],recovery:[]};
        if (request.kind === 'create_team') { __retained = true; return { kind: request.kind, status: 'partial', chatId: request.chatId, groupId: 'retained-group', message: 'The ordinary group was created, but it could not become a team.', members: request.memberIds.map(chatId => ({ chatId, status: 'left_out', reason: 'Conversion failed.' })), recovery: [{ kind: 'make_lead', id: 'retained-group', label: 'Finish This Team', href: href('make_lead', request.leadId, 'retained-group') }] }; }
        if (request.kind === 'create_group') throw Error('Lost response');
        return { kind: request.kind, status: request.kind === 'assign_team' ? 'refused' : 'succeeded', chatId: request.chatId, groupId: '', message: request.kind === 'assign_team' ? 'The assignment changed. Refresh the chart.' : 'The requested change is complete.', members: [], recovery: [], created: request.kind === 'start_chat' ? { sessionId: 'created-chat', cwd: 'C:/Harbor', provider: 'codex', title: 'New Review', open: true, configOptions: [] } : null, cleanup: request.kind === 'delete_chat' ? { sessionId: request.chatId, ok: true, status: 'removed' } : null };
      },
      OpenSessionAt: async (ws, id) => ({ sessionId: id, cwd: ws, provider: 'codex', title: 'New Review', open: true, configOptions: [] })
    };
  } });
  const p = t.page;
  try {
    const key = await p.evaluate(() => dinahWorkbench.open('org-chart', 'fold=research'));
    const chart = p.locator('[data-win="' + key + '"] .org-chart'), panel = chart.locator('.org-action');
    await chart.locator('[data-org-chat="nina"]').waitFor();
    await chart.getByRole('searchbox', { name: 'Find a Chat' }).fill('Nina');
    await chart.locator('[data-org-chat="nina"]').focus(); await p.keyboard.press('Shift+F10');
    const menu = p.locator('.context-menu:popover-open, .menu-panel:popover-open').last();
    await menu.locator('.menu-action').filter({ hasText: 'Create Team' }).click();
    await panel.getByRole('heading', { name: 'Create Team', exact: true }).waitFor();
    t.ok('the context action is addressable and retains filter and fold state', await p.evaluate(() => { const s = new URLSearchParams(new URLSearchParams(location.search).get('view.org-chart')); return s.get('action') === 'create_team' && s.get('chat') === 'nina' && s.get('q') === 'Nina' && s.getAll('fold').includes('research'); }));
    t.ok('lead choices are distinct from member choices', await panel.getByLabel('Lead', { exact: true }).locator('option').evaluateAll(rows => rows.some(r => r.value === 'nina') && !rows.some(r => r.value === 'mara')) && await panel.locator('fieldset').getByText('Mara').count() === 1);
    t.ok('loading and choosing a form perform no mutation', await p.evaluate(() => __actions.length === 0));
    await panel.getByLabel('Name', { exact: true }).fill('Review');
    await panel.locator('fieldset label').filter({ hasText: 'Mara' }).locator('input').check();
    await panel.locator('fieldset label').filter({ hasText: 'Lena' }).locator('input').check();
    await p.evaluate(() => { window.__orgActionFrames = []; const panel = document.querySelector('.org-action'); window.__measureOrgAction = () => Array.from(panel.querySelectorAll('form,.org-action-result')).map(el => { const r=el.getBoundingClientRect(); return [r.x,r.y,r.width,r.height]; }); __orgActionFrames.push(__measureOrgAction()); });
    await panel.getByRole('button', { name: 'Create Team', exact: true }).click();
    await panel.getByRole('link', { name: 'Finish This Team', exact: true }).waitFor();
    await p.evaluate(async () => { for(let n=0;n<12;n++){await new Promise(requestAnimationFrame);__orgActionFrames.push(__measureOrgAction());} });
    t.ok('partial results keep the chart visible behind the dialog', await chart.locator('.org-chart-scroll').isVisible() && await panel.evaluate(el => el.open && el.matches('dialog.dialog')));
    fs.writeFileSync(path.join(process.argv[3], 'org-chart-action-frames.json'), JSON.stringify(await p.evaluate(() => __orgActionFrames), null, 2));
    t.ok('partial creation retains every member refusal and disables duplicate creation', (await panel.innerText()).includes('Mara: Conversion failed.') && (await panel.innerText()).includes('Lena: Conversion failed.') && await panel.getByRole('button', { name: 'Create Team', exact: true }).isDisabled());
    await t.shot('org-chart-actions-partial');
    await panel.getByRole('link', { name: 'Finish This Team', exact: true }).click();
    await panel.getByRole('heading', { name: 'Make Lead', exact: true }).waitFor();
    t.ok('recovery selects the retained group', await panel.getByLabel('Ordinary Group', { exact: true }).inputValue() === 'retained-group');
    await p.goBack(); await panel.getByRole('link', { name: 'Finish This Team', exact: true }).waitFor();
    t.ok('browser Back retains the partial result instead of offering another creation', await panel.getByRole('button', { name: 'Create Team', exact: true }).isDisabled() && (await panel.innerText()).includes('ordinary group was created'));
    await p.goForward(); await panel.getByRole('heading', { name: 'Make Lead', exact: true }).waitFor();
    await panel.getByRole('button', { name: 'Make Lead', exact: true }).click();
    await p.waitForFunction(() => !document.querySelector('.org-action').open);
    t.ok('recovery converts the same group without creating another', await p.evaluate(() => __actions.filter(r => r.kind === 'create_team').length === 1 && __actions[1].kind === 'make_lead' && __actions[1].groupId === 'retained-group' && __actions[1].chatId === 'nina'));
    if (await panel.evaluate(el=>el.open)) await panel.getByRole('button', { name: 'Cancel', exact: true }).click();
    await chart.getByRole('searchbox').fill('');
    await chart.getByRole('button', { name: 'Manage Chats…', exact: true }).click();
    await p.locator('.context-menu:popover-open .menu-action').filter({ hasText: 'Create Group' }).click();
    await panel.getByLabel('Name', { exact: true }).fill('Cross Team');
    await panel.locator('fieldset label').filter({ hasText: 'Nina' }).locator('input').check();
    await p.reload(); await p.locator('.org-action').getByRole('heading', { name: 'Create Group', exact: true }).waitFor();
    t.ok('reload restores the form draft without replaying its mutation', await panel.getByLabel('Name', { exact: true }).inputValue() === 'Cross Team' && await panel.locator('fieldset label').filter({ hasText: 'Nina' }).locator('input').isChecked() && await p.evaluate(() => __actions.length === 0));
    await panel.getByRole('button', { name: 'Create Group', exact: true }).click();
    await panel.locator('.org-action-result').filter({ hasText: 'result could not be read' }).waitFor();
    t.ok('a lost creation result cannot automatically create a duplicate', await panel.getByRole('button', { name: 'Create Group', exact: true }).isDisabled() && await p.evaluate(() => __actions.length === 1));
    for(const width of [1440,390])for(const theme of ['light','dark']){
      await p.setViewportSize({width,height:1000}); await p.evaluate(theme=>document.documentElement.dataset.theme=theme,theme); await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
      // PUDL's sticky action band extends into the dialog padding by design.
      // Check the visible controls against the dialog, rather than its form's scrollWidth.
      const fitting=await panel.evaluate(el=>{const r=el.getBoundingClientRect();return [...el.querySelectorAll('input,select,button,fieldset,.org-action-result')].filter(x=>x.checkVisibility()).map(x=>{const b=x.getBoundingClientRect();return {tag:x.tagName,left:b.left,right:b.right,inside:b.left>=r.left&&b.right<=r.right};});});
      t.ok('the management form fits '+width+' in '+theme,fitting.every(x=>x.inside),JSON.stringify(fitting));
      await t.shot('org-chart-actions-'+theme+'-'+width);
    }
    if (await panel.evaluate(el=>el.open)) await panel.getByRole('button', { name: 'Cancel', exact: true }).click();
    await chart.locator('[data-org-chat="nina"]').focus(); await p.keyboard.press('Shift+F10');
    await p.locator('.context-menu:popover-open .menu-action').filter({hasText:'Delete Chat'}).click();
    await panel.getByRole('heading', { name: 'Delete Chat…', exact:true }).waitFor();
    t.ok('deletion shows the authoritative confirmation verbatim', await panel.locator('.org-action-confirmation').innerText()==='Authoritative confirmation. The chat moves to the Recycle Bin for 30 days. Linked documents stay unchanged.');
    t.ok('opening deletion makes no deletion request',await p.evaluate(()=>__actions.every(r=>r.kind!=='delete_chat')));
    await panel.getByRole('button',{name:'Delete Chat',exact:true}).click();
    await p.waitForFunction(() => !document.querySelector('.org-action').open);
    t.ok('only submission moves the canonical chat to the bin',await p.evaluate(()=>__actions.at(-1).kind==='delete_chat'&&__actions.at(-1).chatId==='nina'));
    if (await panel.evaluate(el=>el.open)) await panel.getByRole('button',{name:'Cancel',exact:true}).click();
    await chart.getByRole('button',{name:'Manage Chats…',exact:true}).click();
    await p.locator('.context-menu:popover-open .menu-action').filter({hasText:'Start a Chat'}).click();
    await panel.getByRole('heading',{name:'Start a Chat',exact:true}).waitFor();
    t.ok('starting a chat requires explicit workspace and program choices', await panel.getByRole('button',{name:'Start a Chat',exact:true}).isDisabled() && await panel.getByLabel('Workspace',{exact:true}).inputValue()==='' && await panel.getByLabel('Agent Program',{exact:true}).inputValue()==='');
    await panel.getByLabel('Workspace',{exact:true}).selectOption('saved-harbor');
    await panel.getByLabel('Agent Program',{exact:true}).selectOption('codex');
    await panel.getByRole('button',{name:'Start a Chat',exact:true}).click();
    await p.waitForFunction(()=>__actions.some(r=>r.kind==='start_chat'));
    await p.waitForFunction(()=>Array.from(document.querySelectorAll('[data-win]')).some(el=>el.dataset.winKind==='chat-session'));
    t.ok('the returned chat opens without a second creation request',await p.evaluate(()=>__actions.filter(r=>r.kind==='start_chat').length===1&&!__bridgeCalls.some(c=>c[0]==='NewSessionWithProviderAt'||c[0]==='NewSessionAt')));
    await p.evaluate(()=>{__pageSuccess=true;return dinahWorkbench.open('org-chart');});
    const geometry=()=>chart.evaluate(root=>{const origin=root.querySelector('.org-chart-scroll').getBoundingClientRect();return {scroll:root.querySelector('.org-chart-scroll').scrollTop,boxes:[...root.querySelectorAll('[data-org-chat]')].map(el=>{const r=el.getBoundingClientRect();return [el.dataset.orgChat,r.x-origin.x,r.y-origin.y,r.width,r.height];})};});
    const actions=[['assign_team','Assign to a Team'],['replace_team_lead','Make Team Lead'],['create_team','Create Team'],['create_group','Create Group'],['start_chat','Start a Chat'],['delete_chat','Delete Chat…']];
    for(const width of [1440,390])for(const theme of ['light','dark']){
      await p.setViewportSize({width,height:1000});await p.evaluate(theme=>document.documentElement.dataset.theme=theme,theme);
      await chart.getByRole('searchbox').fill('');
      for(const [kind,label] of actions){
        const box=chart.locator('[data-org-chat="nina"]');await box.scrollIntoViewIfNeeded();await box.focus();
        await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
        const before=JSON.stringify(await geometry());
        async function open(){await box.locator('.org-chart-actions-button').click();await p.locator('.context-menu:popover-open .menu-action').getByText(label,{exact:true}).click();await panel.getByRole('heading',{name:label,exact:true}).waitFor();}
        await open();t.ok(kind+' keeps chart geometry behind its dialog at '+width+'/'+theme,JSON.stringify(await geometry())===before);
        await panel.getByRole('button',{name:'Cancel',exact:true}).click();
        t.ok(kind+' Cancel restores focus and scroll at '+width+'/'+theme,await box.locator('.org-chart-actions-button').evaluate(el=>el===document.activeElement) && JSON.stringify(await geometry())===before);
        await open();await p.keyboard.press('Escape');
        t.ok(kind+' Escape restores focus and scroll at '+width+'/'+theme,await box.locator('.org-chart-actions-button').evaluate(el=>el===document.activeElement) && JSON.stringify(await geometry())===before);
        await open();
        const name=panel.getByLabel('Name',{exact:true});if(await name.count())await name.fill('Page control '+kind+' '+width+' '+theme);
        const lead=panel.getByLabel('Lead',{exact:true});if(await lead.count())await lead.selectOption('nina');
        const team=panel.getByLabel('Team Lead',{exact:true});if(await team.count())await team.selectOption('mara');
        const ws=panel.getByLabel('Workspace',{exact:true});if(await ws.count())await ws.selectOption('saved-harbor');
        const provider=panel.getByLabel('Agent Program',{exact:true});if(await provider.count())await provider.selectOption('codex');
        const member=panel.locator('fieldset label').filter({hasText:'Uma'}).locator('input');if(await member.count())await member.check();
        await panel.locator('button[type=submit]').click();await p.waitForFunction(()=>!document.querySelector('.org-action').open);
        t.ok(kind+' success closes its dialog and keeps scroll at '+width+'/'+theme,await box.locator('.org-chart-actions-button').evaluate(el=>el===document.activeElement) && JSON.stringify(await geometry())===before && !!await chart.locator('.org-chart-status').innerText());
      }
    }


  } finally { await t.close(); }
})().catch(error=>{console.error(error);process.exitCode=1;});

