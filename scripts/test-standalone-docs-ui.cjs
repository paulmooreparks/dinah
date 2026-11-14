// Hidden real-service check. The executable and all fixture state live in TEMP.
const assert = require('node:assert/strict');
const fs = require('node:fs'), os = require('node:os'), path = require('node:path');
const {spawn} = require('node:child_process');
const {chromium} = require('playwright-core');
const exe = path.resolve(process.argv[2]);
const work = fs.mkdtempSync(path.join(os.tmpdir(), 'standalone-docs-ui-'));
const home = path.join(work, 'home'), state = path.join(home, 'documents');
fs.mkdirSync(state, {recursive:true});
const files = path.join(work, 'files'); fs.mkdirSync(files);
const file = name => path.join(files, name);
const wait = ms => new Promise(r => setTimeout(r, ms));
async function until(check) { const end=Date.now()+20000; while(Date.now()<end) { const v=await check(); if(v)return v; await wait(50); } throw new Error('Fixture state did not settle.'); }
let child, browser, control, phase='start'; const errors=[], windows=[];
(async()=>{
 try {
  child=spawn(exe,['--document-service','--headless','--dir',state],{windowsHide:true,stdio:'ignore',env:{...process.env,DINAH_HOME:home,SAVELOCK_TEST_DIR:path.join(work,'locks')}});
  const announcement=path.join(state,'service.json');
  await until(()=>fs.existsSync(announcement)&&fs.readFileSync(announcement,'utf8').trim().endsWith('}'));
  const service=JSON.parse(fs.readFileSync(announcement,'utf8')), origin='http://'+service.addr;
  control=async(method,route,body)=>{const r=await fetch(origin+'/control'+route,{method,headers:{Authorization:'Bearer '+service.token,'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});const text=await r.text();if(!r.ok)throw new Error(text);return text.trim()?JSON.parse(text):null;};
  browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});
  async function show(entry){const c=await browser.newContext();const p=await c.newPage();p.setDefaultTimeout(20000);p.on('pageerror',e=>errors.push(e.message));await p.goto(entry);await running(p);return p;}
  async function running(p,applet){await p.waitForFunction(a=>{const m=document.querySelector('#document');return m.dataset.appletState==='running'&&!m.hasAttribute('data-applet-busy')&&(!a||m.dataset.applet===a);},applet);}
  const open=async name=>{const d=await control('POST','/open',{application:'editor',path:name?file(name):''});return {id:d.id,page:await show(d.entry)};};
  async function launched(program,args){
    const entry=path.join(work,'entry-'+windows.length+'.jsonl');
    const process=spawn(program,args,{windowsHide:true,stdio:'ignore',env:{...global.process.env,DINAH_HOME:home,DINAH_DOCUMENT_ENTRY_FILE:entry,SAVELOCK_TEST_DIR:path.join(work,'locks')}});windows.push(process);
    const d=await until(()=>{if(!fs.existsSync(entry))return false;try{return fs.readFileSync(entry,'utf8').trim().split('\n').map(s=>JSON.parse(s)).find(d=>d.entry);}catch{return false;}});
    return {id:d.id,page:await show(d.entry)};
  }
  const run=(p,id)=>p.evaluate(id=>pudlCommands.run(id,null,document.querySelector('#document')),id);
  const text=p=>p.evaluate(()=>CM.EditorView.findFromDOM(document.querySelector('#document .cm-editor')).state.doc.toString());
  const type=async(p,s)=>{await run(p,'view-text').catch(()=>{});await p.locator('#document .cm-content').click();await p.keyboard.press('ControlOrMeta+End');await p.keyboard.type(s);};
  const flush=p=>p.evaluate(()=>dinahDocument.flush());
  const disk=name=>fs.readFileSync(file(name),'utf8');
  fs.writeFileSync(file('note.md'),'# Heading\n\nA paragraph.\n');
  fs.writeFileSync(file('plain.txt'),'Plain text.\n');
  phase='Docs chrome';
  const md=await launched(exe,['--document','editor',file('note.md')]);await running(md.page,'docs');
  const p=md.page;
  const menu=async name=>{await p.keyboard.press('Escape');await p.locator('.menubar-title:visible').filter({hasText:new RegExp('^'+name+'$')}).first().click();return p.locator('.menubar-panel .menu-action:visible');};
  let rows=await menu('File');
  assert.ok((await rows.allTextContents()).some(s=>/^Save As/.test(s)));
  assert.ok(!(await rows.allTextContents()).some(s=>/^Publish/.test(s)));
  const fileRows=await rows.allTextContents();
  await p.keyboard.press('Escape');
  assert.equal(await p.title(),'note.md - YAVC Editor');
  assert.match(await p.locator('.ed-status').innerText(),/note.md/);
  assert.match(await p.locator('.ed-status').innerText(),/Markdown/);
  await type(p,'New words.\n');
  await p.waitForFunction(()=>document.title.startsWith('• '));
  rows=await menu('File');await rows.filter({hasText:/^Save(?:Ctrl|⌘|$)/}).first().click();
  await until(()=>fs.readFileSync(file('note.md'),'utf8').includes('New words.'));
  await p.waitForFunction(()=>document.title==='note.md - YAVC Editor');
  rows=await menu('Document');
  assert.deepEqual(await rows.allTextContents(),['Save As…']);
  assert.equal(await rows.first().isEnabled(),true);
  await control('POST','/dialogs',{save:[file('copy.md')]});await rows.first().click();
  await until(()=>fs.existsSync(file('copy.md')));
  await p.waitForFunction(()=>document.title==='copy.md - YAVC Editor');
  assert.match(fs.readFileSync(file('copy.md'),'utf8'),/New words/);
  phase='Help and editing keys';
  rows=await menu('Help');await rows.filter({hasText:'Working with Documents'}).click();
  await p.locator('#document-guide[open]').waitFor();
  assert.match(await p.locator('#document-guide').innerText(),/Publish is available in the main window/);
  await p.locator('#document-guide button').click();
  await p.keyboard.press('F1');await p.locator('#document-keys[open]').waitFor();
  const shortcuts=await p.locator('#document-keys-rows').innerText();
  assert.match(shortcuts,/Save As/);assert.match(shortcuts,/Save/);assert.match(shortcuts,/F1/);
  assert.doesNotMatch(shortcuts,/Publish|Request a Change/);
  await p.locator('#document-keys button').click();
  rows=await menu('Help');await rows.filter({hasText:/^Key Bindings/}).click();
  await p.getByRole('heading',{name:'Docs Key Bindings',exact:true}).waitFor();
  await p.keyboard.press('Escape');
  phase='narrow Help and stable chrome';
  await p.setViewportSize({width:390,height:844});
  await p.evaluate(()=>document.getElementById('document-help').click());await p.locator('#document-guide[open]').waitFor();
  const bounds=await p.locator('#document-guide').boundingBox();assert.ok(bounds.x>=0&&bounds.x+bounds.width<=390);
  await p.screenshot({path:path.join(work,'docs-help-narrow.png')});await p.locator('#document-guide button').click();
  await p.setViewportSize({width:1280,height:900});
  await p.screenshot({path:path.join(work,'docs-chrome.png')});
  phase='plain text and blank controls';
  const plain=await open('plain.txt');await running(plain.page,'editor');
  assert.equal(await plain.page.title(),'plain.txt - YAVC Editor');
  await plain.page.waitForFunction(()=>!document.getElementById('document-key-bindings').disabled);
  await plain.page.evaluate(()=>document.getElementById('document-key-bindings').click());
  await plain.page.getByRole('heading',{name:'Editor Key Bindings',exact:true}).waitFor();
  const blank=await open('');await running(blank.page,'editor');
  assert.match(await blank.page.title(),/^Untitled - YAVC Editor$/);
  fs.writeFileSync(file('chosen.md'),'# Chosen for the blank window\n');
  await control('POST','/dialogs',{open:[file('chosen.md')]});await run(blank.page,'open');
  await running(blank.page,'docs');
  await blank.page.waitForFunction(()=>!document.getElementById('document-save-as').disabled);
  await blank.page.evaluate(()=>document.getElementById('document-key-bindings').click());
  await blank.page.getByRole('heading',{name:'Docs Key Bindings',exact:true}).waitFor();
  assert.deepEqual(errors,[]);
  fs.writeFileSync(path.join(work,'chrome-check.json'),JSON.stringify({fileRows,shortcuts,checks:'Docs File save, Document Save As, title, status, Help, keys, narrow layout, Editor and remount'},null,2));
  console.log('PASS Docs chrome, save, Save As, source-dependent commands, status, Help, keys, narrow layout and Editor control'); } catch(e){console.error('FAIL in '+phase,errors);throw e;}
 finally{if(browser)await browser.close();if(control)await control('POST','/stop',{}).catch(()=>{});for(const p of [...windows,child].filter(Boolean)){if(p.exitCode!==null||p.signalCode)continue;const ended=new Promise(r=>p.once('exit',r));await Promise.race([ended,wait(5000)]);if(p.exitCode===null&&!p.signalCode){p.kill();await Promise.race([ended,wait(5000)]);}}console.log('Retained invented native launch artifacts: '+work);}
})().catch(e=>{console.error(e);process.exitCode=1;});
