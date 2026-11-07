const {launch}=require('./lib/page');
(async()=>{
 const t=await launch({viewport:{width:1280,height:900}}),p=t.page;
 try {
  const root=p.locator('#dinah-launcher');
  await p.waitForFunction(()=>pudlLauncher.state(document.getElementById('dinah-launcher'))!==null);
  t.ok('launcher is closed before opening',await p.evaluate(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='closed'));
  await p.evaluate(()=>pudlMenubar.launcherButton().click());
  await root.waitFor();
  t.ok('identity opens the launcher',await root.isVisible());
  t.ok('launcher chrome remains right aligned',await root.evaluate(el=>{
    const right=el.getBoundingClientRect().right,close=el.querySelector('[data-launcher-close]').getBoundingClientRect(),size=el.querySelector('[data-launcher-size]').getBoundingClientRect();
    return right-close.right<25&&right-size.right<25;
  }));
  await root.locator('[data-start-query]').fill('Files');
  t.ok('catalog filters real destinations',await root.locator('[data-start-item]:visible').count()>=1);
  await root.locator('[data-start-clear]').click();
  await root.locator('[data-launcher-pin]').click();
  t.ok('launcher pins beside workspace',await root.getAttribute('data-launcher-presentation')==='pane');
  t.ok('taskbar remains outside launcher split',await p.locator('.win-bar').evaluate(el=>!el.closest('#launcher-workspace')));
  await t.shot('launcher-pinned');
  await root.locator('[data-start-query]').focus();
  await p.keyboard.press('Escape');
  t.ok('close collapses pinned catalog',await root.getAttribute('data-launcher-presentation')==='collapsed');

  await p.evaluate(()=>pudlMenubar.launcherButton().click());
  await root.locator('[data-launcher-pin]').click();
  await p.evaluate(()=>pudlLauncher.open(document.getElementById('dinah-launcher')));
  await root.locator('[data-launcher-action="chat-browser"]').click();
  await p.waitForFunction(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='closed');
  t.ok('destination executes and dismisses popup',await p.locator('[data-win="chat-browser"]').count()===1);

  await p.evaluate(()=>pudlMenubar.launcherButton().click());
  await root.locator('[data-start-query]').fill('Document');
  await t.shot('launcher-popup');
  await root.locator('[data-start-clear]').click();
  await root.locator('[data-launcher-size]').click();
  await p.locator('[data-launcher-width]').fill('650');
  await p.locator('[data-launcher-width]').dispatchEvent('change');
  await p.locator('#launcher-size [popovertargetaction="hide"]').click();
  t.ok('size settings resize the popup',await p.evaluate(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).width===650));
  await root.locator('[data-start-query]').fill('Document');
  t.ok('search is addressable',await p.evaluate(()=>new URLSearchParams(location.search).get('launcher-query')==='Document'));
  await p.evaluate(()=>dinahUIState.flush());
  t.ok('launcher preferences use host store',await p.evaluate(()=>!!dinahUIState.get('dinah-title-launcher')));
  await p.evaluate(()=>{var u=new URL(location.href);u.searchParams.set('launcher','pane');u.searchParams.set('launcher-category','chat');history.pushState(null,'',u);dispatchEvent(new PopStateEvent('popstate'));});
  t.ok('URL restoration applies category and presentation',await p.evaluate(()=>pudlStart.state(document.getElementById('dinah-launcher')).category==='chat'&&pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='pane'));
  await p.goBack();
  await p.waitForFunction(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='popup');
  await p.setViewportSize({width:640,height:800});
  t.ok('popup fits narrow viewport',await root.evaluate(el=>el.getBoundingClientRect().right<=innerWidth+1&&el.getBoundingClientRect().left>=-1));
  await p.evaluate(()=>pudlLauncher.close(document.getElementById('dinah-launcher')));
  await p.evaluate(()=>pudlMenubar.launcherButton({visible:true}).focus());
  await p.keyboard.press('ArrowDown');
  t.ok('visible menu control opens launcher with keyboard',await root.isVisible());
  await p.evaluate(()=>document.documentElement.setAttribute('data-theme','dark'));
  await t.shot('launcher-narrow-dark');
 }finally{await t.close();}
})();
