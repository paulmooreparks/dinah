const {start} = require('./lib/org-chart-fixture');
(async()=>{
  const t=await start(),p=t.page,chart=t.chart,ok=t.ok;
  try {
    ok('Org Chart is an applet launcher and command, outside the Window menu',await p.evaluate(()=>dinahWorkbench.launchers().some(r=>r.id==='org-chart' && r.glyph==='subagents') && !dinahCommands.rows('window').some(r=>r.label==='Org Chart') && !dinahCommands.rows('view').some(r=>r.label==='Org Chart') && dinahCommands.searchable().some(c=>c.label==='Org Chart' && c.menu==='Tools'))); // not in View since 19 November 2026 (docs/design/menus.md)
    ok('Clean Up Chats is in the launcher and Chats header',await p.evaluate(()=>dinahWorkbench.launchers().some(r=>r.id==='chat-cleanup') && !!document.querySelector('#chat-browser-view [data-cmd="clean-up-chats"]') && !!document.querySelector('#chat-browser-view [data-cmd="org-chart"]')));
    ok('legend is in the toolbar and count is in the status bar',await chart.locator('.org-chart-toolbar .org-chart-legend').count()===1 && await chart.locator('.org-chart-statusbar-count').innerText()==='11 chats, some in folded teams');
    ok('chats appear once and repair rows have no opening or action authority',await chart.locator('[data-org-chat]').evaluateAll(rows=>new Set(rows.map(r=>r.dataset.orgChat)).size===rows.length) && await chart.locator('[data-org-chat="repair"] a, [data-org-chat="repair"] button').count()===0);
    ok('chat titles are real links',await chart.locator('[data-org-chat="nina"] .oc-chat-title').evaluate(a=>a.matches('a[href]') && a.tabIndex===0));
    ok('groups list their members and show raised labels on chat boxes',await chart.locator('[data-org-group="review"] .oc-group-members li').count()===3 && await chart.locator('[data-org-chat="nina"] .oc-group-label').innerText()==='Release Review');
    ok('the chart draws reporting lines without dotted group routes',await chart.locator('svg path').count()>0 && await chart.locator('svg [stroke-dasharray]').count()===0);
    for(const width of [1440,390])for(const theme of ['light','dark']){
      await p.setViewportSize({width,height:1000});await p.evaluate(theme=>document.documentElement.dataset.theme=theme,theme);await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
      const observations=await chart.evaluate(root=>({widths:[...root.querySelectorAll('[data-org-chat]')].map(el=>el.getBoundingClientRect().width),disclosures:[...root.querySelectorAll('[data-org-fold]')].map(el=>({width:el.getBoundingClientRect().width,height:el.getBoundingClientRect().height,cls:el.className,mask:getComputedStyle(el,'::before').maskImage})),actions:[...root.querySelectorAll('.org-chart-actions-button')].map(el=>({visible:el.checkVisibility(),tab:el.tabIndex,width:el.getBoundingClientRect().width})),overflow:root.querySelector('.org-chart-scroll').scrollWidth>root.querySelector('.org-chart-scroll').clientWidth+1}));
      ok('all chat boxes are 216px at '+width+'/'+theme,observations.widths.every(w=>w===216),JSON.stringify(observations));
      ok('standard PUDL disclosures render at 16px at '+width+'/'+theme,observations.disclosures.every(d=>d.cls==='card-toggle' && d.width===16 && d.height===16 && d.mask!=='none'));
      ok('actions remain visible and keyboard reachable at '+width+'/'+theme,observations.actions.every(a=>a.visible && a.tab===0 && a.width>0));
      ok('chart fits at '+width+'/'+theme,!observations.overflow);
      await t.shot('org-chart-'+theme+'-'+width);
      await chart.locator('[data-org-group="review"]').scrollIntoViewIfNeeded();await t.shot('org-chart-groups-'+theme+'-'+width);
      await chart.locator('.org-chart-scroll').evaluate(el=>el.scrollTop=0);
    }
    const group=chart.locator('[data-org-group="review"]');
    await group.locator('.org-chart-actions-button').click();
    const pointer=await p.locator('.context-menu:popover-open .menu-action').allTextContents();await p.keyboard.press('Escape');
    await group.focus();await p.keyboard.press('Shift+F10');
    ok('group actions use the same menu from pointer and keyboard',JSON.stringify(pointer)===JSON.stringify(await p.locator('.context-menu:popover-open .menu-action').allTextContents()) && pointer[0].includes('Open Group Chat'));await p.keyboard.press('Escape');
    await chart.locator('[data-org-fold="research"]').focus();await p.keyboard.press('Enter');await chart.locator('[data-org-chat="bo"]').waitFor();
    ok('Enter expands a team and retains disclosure focus',await chart.locator('[data-org-fold="research"]').getAttribute('aria-expanded')==='true' && await chart.locator('[data-org-fold="research"]').evaluate(el=>el===document.activeElement));
    await p.keyboard.press('Space');ok('Space collapses the same team',await chart.locator('[data-org-chat="bo"]').count()===0);
    await chart.locator('[data-org-chat="nina"]').focus();await p.keyboard.press('Shift+F10');await p.locator('.context-menu:popover-open, .menu-panel:popover-open').last().getByText('Unavailable in this fixture.').first().waitFor();
    ok('Open Chat is first and unavailable actions keep the host reasons',await p.locator('.context-menu:popover-open, .menu-panel:popover-open').last().evaluate(el=>el.querySelector('.menu-action').textContent.includes('Open Chat') && [...el.querySelectorAll('.menu-action[aria-disabled="true"]')].length===6));await p.keyboard.press('Escape');
    await chart.getByRole('searchbox').fill('Nina');ok('search uses the same matching chat set',await chart.locator('[data-org-chat]').count()===1 && await chart.locator('[data-org-chat="nina"]').count()===1);
    await p.waitForFunction(()=>decodeURIComponent(location.search).includes('Nina'));await p.reload();try { await chart.locator('[data-org-chat="nina"]').waitFor(); } catch(error) { console.log(JSON.stringify(await p.evaluate(()=>({url:location.href,ready:document.documentElement.dataset.uiStateReady,charts:[...document.querySelectorAll('.org-chart')].map(r=>({status:r.querySelector('[role=status]').textContent,find:r.querySelector('input').value,boxes:[...r.querySelectorAll('[data-org-chat]')].map(b=>({id:b.dataset.orgChat,rect:b.getBoundingClientRect().toJSON()}))}))}))));await t.shot('org-chart-reload-failure');throw error; }ok('the address restores search',await chart.getByRole('searchbox').inputValue()==='Nina');
    await chart.getByRole('searchbox').fill('');await chart.getByRole('combobox',{name:'Workspace'}).selectOption('C:/Orchard');ok('workspace filter matches the shared inventory',await chart.locator('[data-org-chat="mara"]').count()===0 && await chart.locator('[data-org-chat="uma"]').count()===1);
    await p.evaluate(()=>{__orgError='The chart could not be read.';__emit('bus:changed',{});});await chart.getByRole('status').getByText('The chart could not be read.',{exact:true}).waitFor();ok('read failure keeps the existing chart and offers Retry',await chart.locator('[data-org-chat="uma"]').count()===1 && await chart.getByRole('button',{name:'Retry Loading'}).isVisible());
    await p.evaluate(()=>{__orgError='';});await chart.getByRole('button',{name:'Retry Loading'}).click();await p.waitForFunction(()=>document.querySelector('.org-chart [role="status"]').textContent==='');ok('a successful retry clears its old error',await chart.getByRole('button',{name:'Retry Loading'}).isHidden());
    await p.evaluate(async()=>{await pudlWindows.close('org-chart');const style=document.createElement('style');style.id='fixture-zero-width';style.textContent='.org-chart-scroll { display:none !important; }';document.head.append(style);await dinahWorkbench.open('org-chart','q=Nina');});
    await p.waitForFunction(()=>document.querySelector('.org-chart [role="status"]').textContent==='');
    await p.evaluate(()=>document.querySelector('#fixture-zero-width').remove());
    await chart.locator('[data-org-chat="nina"]').waitFor();
    ok('a chart loaded before its surface has width renders when revealed',await chart.locator('[data-org-chat]').count()===1);

  }finally{await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
