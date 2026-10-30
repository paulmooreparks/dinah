// Settings groups, keyboard controls and durable preference changes.
const { launch, OUT } = require('./lib/page');
const path = require('path');
// Element screenshots need a path of their own; t.shot shoots the whole page.
const shotPath = name => path.join(OUT, name + '.png');
(async () => {
  const t = await launch({
    viewport: {width:1280,height:900},
    app: () => {
      const preferences = {newWindowMode:'floating',defaultProvider:'devin',showThinking:false,usagePage:'',approvals:'devin',codexAppletApprovals:'ask',notifyWhenNeeded:true,};
      window.__preferences = preferences;
      const state = () => ({workspace:'C:/Fixture',recent:['C:/Fixture'],catalog:[],defaults:{},pinned:[],...preferences});
      async function save(key, value) {
        __calls.push([key,value]);
        if (window.__holdPreference) await new Promise(resolve => {window.__releasePreference = resolve;});
        if (window.__failPreference) throw new Error('Fixture preference write failed');
        preferences[key] = value;
      }
      return {
        State:async()=>{const snapshot=state();if(window.__holdState)return new Promise(resolve=>__stateResponses.push(()=>resolve(snapshot)));return snapshot;},
        ReadUIState:async()=>JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'),
        SaveUIState:async value=>localStorage.setItem('fixture-ui',JSON.stringify(value)),
        AgentProviders:async()=>[{id:'devin',name:'Devin',installed:true,available:true},{id:'codex',name:'Codex',installed:true,available:true},{id:'ollama',name:'Ollama',installed:true,available:true}],
        SetNewWindowMode: value=>save('newWindowMode',value), SetDefaultProvider:value=>save('defaultProvider',value),
        SetShowThinking:value=>save('showThinking',value),SetUsagePage:value=>save('usagePage',value),SetCodexAppletApprovals:value=>save('codexAppletApprovals',value),
        SetOllamaApprovals:value=>save('ollamaApprovals',value),
        SetLogLevel:value=>save('logLevel',value),SetNotifyWhenNeeded:value=>save('notifyWhenNeeded',value),
        ReportWindowFocus:async value=>{__focusReports.push(value);}
      };
    },
    init: () => { window.__calls = []; window.__stateResponses = []; window.__focusReports = []; },
  });
  const { page, ok, errors } = t;
  try {
    await page.evaluate(()=>dinahWorkbench.open('settings'));
    const settings = page.locator('#settings');
    const tab = name=>settings.getByRole('tab',{name,exact:true});
    const mode = value=>settings.locator('#new-window-mode [role=radio][data-value="'+value+'"]');
    ok('Settings presents seven named sections', (await settings.getByRole('tab').allTextContents()).map(x=>x.trim()).join('|')==='Appearance|Agents|Judgements|Chat|Terminal|Packages|Diagnostics');
    ok('Appearance is initially selected',await tab('Appearance').getAttribute('aria-selected')==='true');
    // Every tab shows its own panel when it is clicked, the last added among them.
    const shown=[];
    for (const name of ['Agents','Judgements','Chat','Packages','Diagnostics','Appearance']) {
      await tab(name).click();
      const panel=await tab(name).getAttribute('aria-controls');
      shown.push(await settings.locator('#'+panel).isVisible() && await tab(name).getAttribute('aria-selected')==='true');
    }
    ok('every Settings tab shows its panel when clicked', shown.every(Boolean), JSON.stringify(shown));
    await tab('Diagnostics').click();
    await settings.locator('#log-level [data-value="all"]').click();
    await page.waitForFunction(()=>document.querySelector('#log-status').textContent.includes('everything'));
    ok('Diagnostics saves how much the log keeps', await settings.locator('#log-level [data-value="all"]').getAttribute('aria-checked')==='true');
    ok('a saved preference shows no error',!(await settings.locator('#log-status').getAttribute('class')).includes('danger'));
    await page.evaluate(()=>{__failPreference=true;});
    await settings.locator('#log-level [data-value="problems"]').click();
    await page.waitForFunction(()=>!document.querySelector('#log-level button').disabled && document.querySelector('#log-status').classList.contains('danger'));
    ok('a failed save is a danger notice in Settings', await settings.locator('#log-status.notice.danger').isVisible() && (await settings.locator('#log-status').textContent()).includes('could not be saved'));
    ok('a failed save keeps the saved choice', await settings.locator('#log-level [data-value="all"]').getAttribute('aria-checked')==='true');
    ok('a failed save is under Notifications', await page.locator('#questions').evaluate(el=>el.textContent.includes('Fixture preference write failed')));
    await settings.locator('#log-status').screenshot({path:shotPath('settings-save-failed')});
    await page.evaluate(()=>{__failPreference=false;});
    await settings.locator('#log-level [data-value="off"]').click();
    await page.waitForFunction(()=>document.querySelector('#log-status').textContent.includes('nothing'));
    ok('the next good save clears the notice', !(await settings.locator('#log-status').getAttribute('class')).includes('notice'));
    await tab('Appearance').click();
    await tab('Appearance').focus(); await page.keyboard.press('ArrowRight');
    ok('arrow keys move Settings tab focus without changing the panel',await tab('Agents').evaluate(el=>el===document.activeElement) && await tab('Appearance').getAttribute('aria-selected')==='true');
    await page.keyboard.press('Enter');
    ok('keyboard navigation selects Agents and updates its address',await tab('Agents').getAttribute('aria-selected')==='true' && await page.evaluate(()=>new URLSearchParams(location.search).get('view.settings')==='agents'));
    ok('the agent preference is a dropdown inside its card',await settings.locator('#default-provider').evaluate(el=>el.tagName==='SELECT' && !!el.closest('.card')));
    const ollamaApproval = value=>settings.locator('#ollama-approvals [data-value="'+value+'"]');
    ok('Ollama approvals default to once in each chat',await ollamaApproval('chat').getAttribute('aria-checked')==='true');
    await ollamaApproval('allow').click();
    await page.waitForFunction(()=>__preferences.ollamaApprovals==='allow' && !document.querySelector('#ollama-approvals button').disabled);
    ok('Ollama Never selection is saved',await ollamaApproval('allow').getAttribute('aria-checked')==='true');
    const codexApproval = value=>settings.locator('#codex-applet-approvals [data-value="'+value+'"]');
    ok('Codex applet approvals initially ask independently of Devin',await codexApproval('ask').getAttribute('aria-checked')==='true' && await settings.locator('input[name=approvals][value=devin]').isChecked());
    await codexApproval('ask').focus();
    await page.evaluate(()=>{__holdPreference=true;});
    await page.keyboard.press('ArrowRight');
    await page.waitForFunction(()=>typeof __releasePreference==='function');
    ok('Codex approval choice disables both controls until persisted',await codexApproval('ask').isDisabled() && await codexApproval('allow').isDisabled() && await page.evaluate(()=>__preferences.codexAppletApprovals==='ask'));
    await page.evaluate(()=>{__holdPreference=false;__releasePreference();});
    await page.waitForFunction(()=>!document.querySelector('#codex-applet-approvals button').disabled);
    ok('Codex applet permission is provider scoped and explains existing chats',await codexApproval('allow').getAttribute('aria-checked')==='true' && await page.evaluate(()=>__preferences.approvals==='devin' && __preferences.defaultProvider==='devin') && (await settings.locator('#codex-applet-approvals-status').textContent()).includes('next turn'));
    await settings.locator('[aria-labelledby="set-codex-applet-approvals"]').first().screenshot({path:shotPath('settings-codex-applet-approvals')});
    await page.evaluate(()=>{__failPreference=true;});
    await codexApproval('ask').click();
    await page.waitForFunction(()=>!document.querySelector('#codex-applet-approvals button').disabled);
    ok('failed Codex approval save restores the saved selection',await codexApproval('allow').getAttribute('aria-checked')==='true' && (await settings.locator('#codex-applet-approvals-status').textContent()).includes('could not be saved'));
    await page.evaluate(()=>{__failPreference=false;delete window.__releasePreference;});
    for (const [id, key] of [['codex-applet-approvals','codexAppletApprovals'],['ollama-approvals','ollamaApprovals']]) {
      const control = settings.locator('#'+id);
      await control.locator('[data-value="all"]').click();
      await page.waitForFunction(({id,key})=>__preferences[key]==='all' && !document.querySelector('#'+id+' button').disabled,{id,key});
      ok(id+' saves Approve Everything',await control.locator('[data-value="all"]').getAttribute('aria-checked')==='true');
      await page.evaluate(()=>__emit('settings:changed',{}));
      ok(id+' retains the saved choice after refresh',await control.locator('[data-value="all"]').getAttribute('aria-checked')==='true');
      await control.screenshot({path:shotPath(id+'-all')});
    }
    await page.setViewportSize({width:1920,height:900});
    await page.locator('[data-win="settings"] [data-win-action="maximize"]').click();
    const agentsPanel=page.locator('#settings-agents');
    // Maximized, the window fills the space between the docks.
    await page.waitForFunction(()=>document.querySelector('[data-win="settings"]').getAttribute('data-win-mode')==='maximized' && document.querySelector('#settings-agents').clientWidth>700);
    const panelBox=await agentsPanel.boundingBox(), bodyBox=await settings.locator('.settings-body').boundingBox();
    ok('Settings scroll area spans the window body',Math.abs(panelBox.x-bodyBox.x)<2 && Math.abs(panelBox.width-bodyBox.width)<2);
    for (const side of ['left','right']) {
      await agentsPanel.evaluate(el=>{el.scrollTop=0;});
      await page.mouse.move(side==='left'?panelBox.x+24:panelBox.x+panelBox.width-32,panelBox.y+100);
      await page.mouse.wheel(0,240);
      await page.waitForFunction(()=>document.querySelector('#settings-agents').scrollTop>0);
      ok('mouse wheel scrolls Agents from the '+side+' margin',true);
    }
    await t.shot('settings-full-width-scroll');
    await page.locator('[data-win="settings"] [data-win-action="maximize"]').click();
    await page.setViewportSize({width:1280,height:900});
    await tab('Appearance').click();
    await settings.locator('#settings-theme [data-theme-choice="dark"]').click();
    ok('Appearance theme uses the shared PUDL preference',await page.evaluate(()=>pudlThemePreference()==='dark') && await settings.locator('#settings-theme [data-theme-choice="dark"]').getAttribute('aria-checked')==='true');
    await settings.locator('#settings-theme [data-theme-choice="light"]').click();
    await mode('floating').focus();
    await page.evaluate(()=>{__holdPreference=true;});
    await page.keyboard.press('ArrowRight');
    await page.waitForFunction(()=>typeof __releasePreference==='function');
    ok('window mode blocks repeated changes while saving',await mode('floating').isDisabled() && await mode('maximized').isDisabled());
    ok('pending mode has not changed future window behavior',await page.evaluate(()=>dinahUIState.newWindowMode()==='floating'));
    await page.evaluate(()=>{__holdPreference=false;__releasePreference();});
    await page.waitForFunction(()=>dinahUIState.newWindowMode()==='maximized');
    ok('keyboard selection saves Maximized',await mode('maximized').getAttribute('aria-checked')==='true');
    await page.evaluate(()=>{__failPreference=true;});
    await mode('floating').click();
    await page.waitForFunction(()=>!document.querySelector('#new-window-mode [role=radio]').disabled);
    ok('failed mode save restores the prior selection and behavior',await mode('maximized').getAttribute('aria-checked')==='true' && await page.evaluate(()=>dinahUIState.newWindowMode()==='maximized'));
    await page.evaluate(()=>{__failPreference=false;});
    for (const name of ['Appearance','Agents','Chat','Packages']) {
      await tab(name).click();
      ok(name+' controls are grouped in cards',await settings.locator('[role=tabpanel]:visible').evaluate(panel=>Array.from(panel.querySelectorAll('input,select,textarea,button')).every(el=>!!el.closest('.card'))));
    }
    await tab('Chat').click();
    await page.evaluate(()=>{
      const preferences={newWindowMode:'floating',defaultProvider:'codex',showThinking:true};
      Object.assign(__preferences,preferences);
      __emit('settings:changed',{preferences,revision:'fixture-2'});
    });
    await page.waitForFunction(()=>document.querySelector('#set-thinking').getAttribute('aria-checked')==='true');
    ok('Settings updates chat controls from an interface change',await settings.locator('#set-thinking').getAttribute('aria-checked')==='true');
    await page.evaluate(()=>{__holdPreference=true;__failPreference=true;delete window.__releasePreference;});
    await settings.locator('#set-thinking').click();
    await page.waitForFunction(()=>typeof __releasePreference==='function');
    ok('thinking preference blocks another save while pending',await settings.locator('#set-thinking').isDisabled());
    await page.evaluate(()=>{__holdPreference=false;__releasePreference();});
    await page.waitForFunction(()=>!document.querySelector('#set-thinking').disabled);
    ok('failed thinking preference restores its prior value',await settings.locator('#set-thinking').getAttribute('aria-checked')==='true');
    await page.evaluate(()=>{__failPreference=false;});
    // Notify Me When I Am Needed is on by default, saves when switched and
    // shows the saved value again when the save fails.
    const notify=settings.locator('#set-notify-needed');
    ok('Notify Me When I Am Needed is on by default and sits in a card',await notify.getAttribute('aria-checked')==='true' && await notify.evaluate(el=>!!el.closest('.card')) && (await notify.textContent()).trim()==='Notify Me When I Am Needed');
    await notify.click();
    await page.waitForFunction(()=>__preferences.notifyWhenNeeded===false && !document.querySelector('#set-notify-needed').disabled);
    ok('switching Notify Me When I Am Needed off saves it',await notify.getAttribute('aria-checked')==='false');
    await page.evaluate(()=>{__failPreference=true;});
    await notify.click();
    await page.waitForFunction(()=>!document.querySelector('#set-notify-needed').disabled);
    ok('a failed save of Notify Me When I Am Needed restores the saved value',await notify.getAttribute('aria-checked')==='false');
    await page.evaluate(()=>{__failPreference=false;});
    await notify.click();
    await page.waitForFunction(()=>__preferences.notifyWhenNeeded===true);
    // Devin's billing month is read from its usage page, so Settings has no
    // day to choose.
    await tab('Agents').click();
    ok('Settings has no Devin billing day to choose',await settings.locator('#devin-reset-day').count()===0);
    ok('the page reports its focus to the app',await page.evaluate(()=>__focusReports.length>0 && __focusReports.every(v=>typeof v==='boolean')));
    await tab('Appearance').click();
    ok('interface changes update window behavior and its selected control',await mode('floating').getAttribute('aria-checked')==='true' && await page.evaluate(()=>dinahUIState.newWindowMode()==='floating'));
    await tab('Agents').click();
    ok('interface changes update the default agent dropdown',await settings.locator('#default-provider').inputValue()==='codex');
    await page.evaluate(()=>{
      __holdState=true;
      __preferences.newWindowMode='maximized';
      __emit('settings:changed',{preferences:{newWindowMode:'floating'},revision:'stale-event'});
    });
    await page.waitForFunction(()=>__stateResponses.length===1);
    await page.evaluate(()=>{__preferences.newWindowMode='floating';__emit('settings:changed',{preferences:{newWindowMode:'maximized'},revision:'latest-event'});});
    await page.waitForFunction(()=>__stateResponses.length===2);
    await page.evaluate(()=>{__stateResponses[1]();});
    await page.waitForFunction(()=>dinahUIState.newWindowMode()==='floating');
    await page.evaluate(async()=>{__holdState=false;__stateResponses[0]();await new Promise(resolve=>requestAnimationFrame(resolve));});
    ok('stale settings responses cannot replace the newest canonical preferences',await page.evaluate(()=>dinahUIState.newWindowMode()==='floating'));
    async function scrollPanel(name, top) {
      await tab(name).click();
      const id='settings-'+name.toLowerCase();
      const actual=await page.locator('#'+id).evaluate((el,value)=>{el.scrollTop=value;return el.scrollTop;},top);
      await page.waitForFunction(({id,actual})=>dinahUIState.get('dinah-scroll:settings:'+id)?.top===actual,{id,actual});
      return actual;
    }
    const chatScroll=await scrollPanel('Chat',180);
    const agentScroll=await scrollPanel('Agents',300);
    ok('Settings sections have independent scroll positions',chatScroll>80 && agentScroll>80);
    await tab('Chat').click();
    ok('switching sections retains the previous reading position',await page.locator('#settings-chat').evaluate(el=>el.scrollTop)===chatScroll);
    await page.evaluate(async()=>{await dinahUIState.flush();pudlWindows.close('settings');await dinahWorkbench.open('settings');});
    await page.waitForFunction(top=>document.querySelector('#settings-chat').scrollTop===top,chatScroll);
    ok('closing and reopening restores the section and scroll',await tab('Chat').getAttribute('aria-selected')==='true');
    await tab('Agents').click();
    await page.waitForFunction(top=>document.querySelector('#settings-agents').scrollTop===top,agentScroll);
    ok('another section retains its scroll after reopening',await page.locator('#settings-agents').evaluate(el=>el.scrollTop)===agentScroll);
    await tab('Chat').click();
    await page.reload();
    await settings.waitFor({state:'visible'});
    ok('Settings restores its addressable section',await tab('Chat').getAttribute('aria-selected')==='true');
    await t.shot('settings-chat');
    await page.setViewportSize({width:560,height:800});
    for (const name of ['Appearance','Agents','Chat','Packages','Diagnostics']) {
      await tab(name).click();
      ok(name+' fits a narrow window without horizontal scrolling',await settings.evaluate(el=>el.scrollWidth<=el.clientWidth+1) && await settings.locator('[role=tabpanel]:visible').evaluate(el=>el.scrollWidth<=el.clientWidth+1));
      await t.shot('settings-'+name.toLowerCase()+'-narrow');
    }
    ok('no page errors or browser dialogs',errors.length===0);
    if (errors.length) console.log(errors.join('\n'));
  } finally {await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
