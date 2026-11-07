const {launch}=require('./lib/page');
(async()=>{
 const t=await launch(); const p=t.page;
 try {
  const fixed=p.locator('.win-bar > button[data-cmd]');
  t.ok('taskbar has no duplicate Settings or Quit commands',await fixed.count()===0);
  await p.evaluate(()=>pudlMenubar.launcherButton().click());
  await p.locator('#dinah-launcher [data-cmd="settings"]').click();
  await p.locator('[data-win="settings"] #settings').waitFor();
  t.ok('settings uses existing command handler',await p.locator('[data-win="settings"] #settings').isVisible());
  t.ok('workspace menu keeps creation and workspace commands',await p.locator('[data-command-menu="workspace"] [data-cmd]').count()===6);
  t.ok('recent workspaces retain their named navigation group',await p.locator('[data-command-menu="workspace"] [data-recent-menu]').count()===1);
  t.ok('Tools menu is removed and package installation remains in launcher', await p.locator('[data-menubar-id=tools]').count()===0 && await p.locator('[data-launcher-action=install-packages-file]').count()===1);
  await p.waitForFunction(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='closed');
  await p.evaluate(()=>pudlLauncher.open(document.getElementById('dinah-launcher')));
  await p.locator('#dinah-launcher [data-cmd="quit"]').click();
  await p.waitForFunction(()=>__runtime.some(x=>x[0]==='Quit'));
  t.ok('quit retains native shutdown route',await p.evaluate(()=>__runtime.filter(x=>x[0]==='Quit').length===1));
  await t.shot('launcher-command-preparation');
 }finally{await t.close();}
})();

