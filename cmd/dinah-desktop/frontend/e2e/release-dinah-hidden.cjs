const fs=require('node:fs'),path=require('node:path'),os=require('node:os');
const assert=require('node:assert/strict');
const {spawn,execFileSync}=require('node:child_process');
const {createRequire}=require('node:module');
const {chromium}=createRequire('C:/Users/paul/source/repos/pudl/package.json')('playwright');
const binaries=path.resolve(process.argv[2]);
const root=fs.mkdtempSync(path.join(os.tmpdir(),'dinah-release-fresh-'));
const home=path.join(root,'home'),bin=path.join(home,'bin'),entry=path.join(root,'entry.txt');
const result={root,checks:[],versions:{}};
const check=(name,condition)=>{assert.ok(condition,name);result.checks.push(name);};
let child,browser;
const until=async(fn)=>{const end=Date.now()+60000;while(Date.now()<end){if(fn())return;await new Promise(r=>setTimeout(r,100));}throw Error('Hidden entry did not arrive');};
async function stop(){if(browser){await browser.close();browser=null;}if(child&&child.exitCode===null){const done=new Promise(r=>child.once('exit',r));child.kill();await done;}child=null;}
(async()=>{try{
  for(const d of ['home/bin','person','appdata','local','tmp','codex','claude'])fs.mkdirSync(path.join(root,d),{recursive:true});
  const env={SystemRoot:process.env.SystemRoot,WINDIR:process.env.WINDIR,COMSPEC:process.env.COMSPEC,
    PATH:path.join(process.env.SystemRoot,'System32'),USERPROFILE:path.join(root,'person'),
    APPDATA:path.join(root,'appdata'),LOCALAPPDATA:path.join(root,'local'),TEMP:path.join(root,'tmp'),TMP:path.join(root,'tmp'),
    CODEX_HOME:path.join(root,'codex'),CLAUDE_CONFIG_DIR:path.join(root,'claude'),DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entry};
  for(const [name,arg] of [['dinah.exe','version'],['dinah-desktop.exe','--version']]){
    fs.copyFileSync(path.join(binaries,name),path.join(bin,name));
    result.versions[name]=execFileSync(path.join(bin,name),[arg],{env,cwd:root,windowsHide:true,encoding:'utf8'}).trim();
    check(name+' starts and identifies Dinah',/^dinah(?:-desktop)?\s/.test(result.versions[name]));
  }
  for(let phase=0;phase<2;phase++){
    if(fs.existsSync(entry))fs.unlinkSync(entry);
    child=spawn(path.join(bin,'dinah-desktop.exe'),[],{env,cwd:root,windowsHide:true,stdio:'ignore'});
    await until(()=>fs.existsSync(entry));
    browser=await chromium.launch({headless:true});
    const page=await browser.newPage({viewport:{width:1400,height:1000}});
    const errors=[];page.on('pageerror',e=>errors.push(e.message));
    await page.route('**/*',r=>['127.0.0.1','localhost','[::1]'].includes(new URL(r.request().url()).hostname)?r.continue():r.abort());
    await page.goto(fs.readFileSync(entry,'utf8').trim());
    await page.waitForFunction(()=>window.dinahUIState&&window.dinahWorkbench);
    await page.evaluate(()=>dinahUIState.ready);
    await page.waitForFunction(()=>window.dinahSettingsSections&&window.dinahWhatsNew&&window.pudlWindows,null,{timeout:30000});
    await page.evaluate(()=>dinahWorkbench.open('needs-you'));
    if(phase===0){
      const card=page.locator('.whats-new-card');await card.waitFor();
      check('one first-start card',await card.count()===1);
      check('no product-specific Updates button',await card.getByRole('button',{name:'Open Updates',exact:true}).count()===0);
      for(const [label,selector] of [['Open Decisions','.decisions'],['Open Working with Chats','[data-win="help-chats"] article'],['Open Background Chats','#settings-chat'],['Open Set-aside Programs','#settings-set-aside']]){
        await page.evaluate(()=>dinahWorkbench.open('needs-you'));
        await card.getByRole('button',{name:label,exact:true}).click();
        await page.locator(selector).waitFor({state:'visible'});check(label+' works',true);
      }
      check('Updates section is unavailable',!await page.locator('#settings-tab-updates').isVisible());
      if(process.env.DINAH_FORBIDDEN_TERMS){
        const text=await page.locator('body').innerText();
        let patterns;
        try {patterns=fs.readFileSync(process.env.DINAH_FORBIDDEN_TERMS,'utf8').split(/\r?\n/).map(s=>s.trim()).filter(s=>s&&!s.startsWith('#')).map(s=>new RegExp(s.replace(/^\(\?i\)/,''),'i'));}
        catch {throw Error('The private terms file could not be read or compiled');}
        check('visible text contains no private terms',patterns.every(p=>!p.test(text)));
      }else{result.skipped=['Private terms assertion: DINAH_FORBIDDEN_TERMS is not set.'];}
      check('no product-specific update service',!await page.evaluate(async()=>(await go.main.App.ProductServices()).includes('updates')));
      await page.evaluate(()=>dinahWorkbench.open('needs-you'));await card.getByRole('button',{name:'Dismiss',exact:true}).click();await card.waitFor({state:'detached'});
      await page.screenshot({path:path.join(root,'first-start.png')});
    }else{
      const items=await page.evaluate(()=>go.main.App.Notifications());
      check('dismissed feature card stays dismissed after restart',!items.some(n=>n.id==='whats-new-20261114'&&!n.dismissed));
    }
    check('no page errors in phase '+phase,errors.length===0);await stop();
  }
  result.passed=true;
}catch(e){result.failure=e.stack;result.passed=false;process.exitCode=1;}finally{await stop();fs.writeFileSync(path.join(root,'result.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result,null,2));}})();
