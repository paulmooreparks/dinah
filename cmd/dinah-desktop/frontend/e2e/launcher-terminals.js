const {launch}=require('./lib/page');
(async()=>{const t=await launch({app:()=>({TerminalShells:async()=>({shells:[{id:'pwsh',name:'PowerShell'},{id:'cmd',name:'Command Prompt'}]})})}),p=t.page;try{
await p.waitForFunction(()=>document.querySelector('[data-launcher-action="terminal:pwsh"]'));
t.ok('recommended destinations and creation commands are available',await p.evaluate(()=>['skills-browser','in-progress','needs-you','new-workspace','new-empty-workspace','import-workspace','new-document','open-document','keyboard-shortcuts','about'].every(id=>document.querySelector('[data-launcher-action="'+id+'"]'))));
await p.evaluate(()=>{window.__terminalChoice=[];dinahResources.currentPath=async()=> 'C:/Fixture';dinahResources.openTerminal=async(path,unused,id)=>__terminalChoice.push({path,id});});
await p.evaluate(()=>pudlMenubar.launcherButton().click());
await p.locator('[data-start-category-key="terminals"]').click();
t.ok('terminal category contains default, each detected shell, and refresh',await p.locator('#dinah-launcher [data-start-item]:visible').count()===4);
await p.locator('[data-launcher-action="terminal:cmd"]').click();
await p.waitForFunction(()=>__terminalChoice.length===1);
await p.waitForFunction(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='closed');
t.ok('explicit shell preserves workspace path and shell identity',await p.evaluate(()=>__terminalChoice[0].path==='C:/Fixture'&&__terminalChoice[0].id==='cmd'));
await p.evaluate(()=>pudlMenubar.launcherButton().click());
await p.evaluate(()=>{go.main.App.TerminalShells=async()=>({shells:[{id:'wsl',name:'WSL'}]});});
await p.locator('[data-launcher-action="terminal:refresh"]').click();
await p.waitForFunction(()=>document.querySelector('[data-launcher-action="terminal:wsl"]'));

t.ok('refresh replaces stale shell entries',await p.locator('[data-launcher-action="terminal:cmd"]').count()===0);
await p.evaluate(()=>pudlLauncher.open(document.getElementById('dinah-launcher')));
t.ok('launcher has accessible name without redundant heading',await p.locator('#dinah-launcher').getAttribute('aria-label')==='Launcher'&&await p.locator('#launcher-title').count()===0);
await p.locator('#dinah-launcher [data-cmd="settings"]').click();
await p.locator('[data-win="settings"] #settings').waitFor();
await p.waitForFunction(()=>pudlLauncher.state(document.getElementById('dinah-launcher')).presentation==='closed');
await p.evaluate(()=>pudlLauncher.open(document.getElementById('dinah-launcher')));
await t.shot('launcher-header-terminals');
await p.locator('#dinah-launcher [data-cmd="quit"]').click();
await p.waitForFunction(()=>__runtime.some(x=>x[0]==='Quit'));
t.ok('header Quit dispatches once through existing shutdown handler',await p.evaluate(()=>__runtime.filter(x=>x[0]==='Quit').length===1));
}finally{await t.close()}})();
