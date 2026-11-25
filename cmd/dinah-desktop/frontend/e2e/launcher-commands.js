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
  // Eleven since step 2 of 19 November 2026: New Chat left for Chat, and Add a Source… and the three section rows joined (docs/design/menus.md).
  t.ok('workspace menu keeps creation and workspace commands',await p.locator('[data-command-menu="workspace"] [data-cmd]').count()===11);
  t.ok('recent workspaces retain their named navigation group',await p.locator('[data-command-menu="workspace"] [data-recent-menu]').count()===1);
  t.ok('Tools menu is removed and package installation remains in launcher', await p.locator('[data-menubar-id=tools]').count()===0 && await p.locator('[data-launcher-action=install-packages-file]').count()===1);
  await p.waitForFunction(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='closed');
  t.ok('a launcher with no commands of its own has only Settings and Exit before its pin', await p.locator('#dinah-launcher .start-header > [data-generated]').count()===0);
  // A product's command for the launcher is a button at the start of its header, with its full label as the tooltip, and it closes the launcher.
  await p.evaluate(()=>{ window.__launcherRuns=0; dinahCommands.register({id:'product-check',menu:'launcher',glyph:'download',menuLabel:'Updates…',label:'Update the Product…',run:()=>{ window.__launcherRuns++; }}); document.dispatchEvent(new CustomEvent('pudl:commands-change')); });
  await p.evaluate(()=>pudlLauncher.open(document.getElementById('dinah-launcher')));
  const own=p.locator('#dinah-launcher .start-header > [data-generated]');
  await own.first().waitFor();
  t.ok('the product command stands first in the header', await p.evaluate(()=>{ const head=document.querySelector('#dinah-launcher .start-header'); return head.firstElementChild.hasAttribute('data-generated') && head.firstElementChild.nextElementSibling.dataset.cmd==='settings'; }));
  t.ok('it shows its short label and glyph, with the full label as its tooltip', (await own.textContent()).trim()==='Updates…' && await own.getAttribute('title')==='Update the Product…' && await own.locator('.glyph').count()===1);
  await own.click();
  await p.waitForFunction(()=>window.__launcherRuns===1);
  await p.waitForFunction(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='closed');
  t.ok('pressing it runs the command once and closes the launcher', await p.evaluate(()=>window.__launcherRuns===1));
  t.ok('command search files it under Launcher', await p.evaluate(()=>dinahCommands.searchable().some(c=>c.label==='Update the Product…' && c.menu==='Launcher')));
  await p.evaluate(()=>pudlLauncher.open(document.getElementById('dinah-launcher')));
  await p.locator('#dinah-launcher [data-cmd="quit"]').click();
  await p.waitForFunction(()=>__runtime.some(x=>x[0]==='Quit'));
  t.ok('quit retains native shutdown route',await p.evaluate(()=>__runtime.filter(x=>x[0]==='Quit').length===1));
  await t.shot('launcher-command-preparation');
 }finally{await t.close();}
})();

