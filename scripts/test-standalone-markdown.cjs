// Hidden real-service check. The executable and all fixture state live in TEMP.
const assert = require('node:assert/strict');
const fs = require('node:fs'), os = require('node:os'), path = require('node:path');
const {spawn} = require('node:child_process');
const {chromium} = require('playwright-core');
const exe = path.resolve(process.argv[2]);
const work = fs.mkdtempSync(path.join(os.tmpdir(), 'standalone-markdown-'));
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
  fs.writeFileSync(file('note.md'),'# Heading\n\nA paragraph.\n');fs.writeFileSync(file('long.MARKDOWN'),'# Other\n');fs.writeFileSync(file('plain.txt'),'Plain text.\n');
  phase='initial Markdown and plain text routing';
  let md=await launched(exe,['--document','editor',file('note.md')]);await running(md.page,'docs');
  await run(md.page,'view-visual');await md.page.locator('#document .ProseMirror').waitFor();
  const alias=path.join(work,'yavc-editor.exe');fs.copyFileSync(exe,alias);
  const upper=await launched(alias,[file('long.MARKDOWN')]);await running(upper.page,'docs');
  const plain=await open('plain.txt');await running(plain.page,'editor');assert.equal(await plain.page.locator('.ProseMirror').count(),0);
  fs.writeFileSync(file('deck.slides.md'),'# Deck\n');const deck=await open('deck.slides.md');await running(deck.page,'editor');assert.equal(await text(deck.page),'# Deck\n');
  phase='an Editor-produced Markdown draft is recovered in Docs';
  await control('POST','/dialogs',{save:[file('legacy.md')]});await run(plain.page,'save-as');await type(plain.page,'Legacy draft.\n');await flush(plain.page);await control('POST','/window/'+plain.id+'/closed',{});await plain.page.context().close();
  const legacy=await open('legacy.md');await running(legacy.page,'docs');assert.match(await text(legacy.page),/Legacy draft/);assert.doesNotMatch(disk('legacy.md'),/Legacy draft/);
  phase='Markdown save, stale save and Save As';
  await type(md.page,'Saved words.\n');await run(md.page,'save');assert.match(disk('note.md'),/Saved words/);
  await type(md.page,'Draft words.\n');fs.writeFileSync(file('note.md'),'# External change\n');await run(md.page,'save').catch(()=>{});assert.equal(disk('note.md'),'# External change\n');assert.match(await text(md.page),/Draft words/);
  await control('POST','/dialogs',{save:[file('copy.md')]});await run(md.page,'save-as');assert.match(disk('copy.md'),/Draft words/);
  phase='Markdown recovery after close';
  await type(md.page,'Recovered words.\n');await flush(md.page);await control('POST','/window/'+md.id+'/closed',{});await md.page.context().close();
  md=await open('copy.md');await running(md.page,'docs');assert.match(await text(md.page),/Recovered words/);assert.doesNotMatch(disk('copy.md'),/Recovered words/);
  phase='permission dialog on Docs';
  let action=control('POST','/documents/'+md.id+'/invoke',{from:'invented test',request:{operation:'document',input:{}}}).then(v=>({value:v}),e=>({error:e.message}));
  await md.page.locator('#document-permission').waitFor();await md.page.locator('[value=refuse]').click();assert.ok((await action).error);
  action=control('POST','/documents/'+md.id+'/invoke',{from:'invented test',request:{operation:'document',input:{}}});await md.page.locator('#document-permission').waitFor();await md.page.locator('[value=once]').click();assert.ok(await action);
  phase='blank Open replaces Editor with Docs';
  fs.writeFileSync(file('chosen.markdown'),'# Chosen\n');const blank=await open('');
  await control('POST','/dialogs',{open:[file('chosen.markdown')]});await run(blank.page,'open');await running(blank.page,'docs');assert.match(await text(blank.page),/Chosen/);assert.equal((await control('GET','/documents')).find(d=>d.id===blank.id).path,file('chosen.markdown'));
  phase='Docs Open routes plain text to another window';
  fs.writeFileSync(file('second.txt'),'Second plain.\n');let count=(await control('GET','/started')).length;
  await control('POST','/dialogs',{open:[file('second.txt')]});const opening=run(blank.page,'open');const started=await until(async()=>{const all=await control('GET','/started');return all.length>count&&all.at(-1);});const other=await show(started.entry);await opening;await running(other,'editor');assert.equal(await text(other),'Second plain.\n');assert.match(await text(blank.page),/Chosen/);
  phase='Docs New retains Docs';
  count=(await control('GET','/started')).length;const creating=run(blank.page,'new');const created=await until(async()=>{const all=await control('GET','/started');return all.length>count&&all.at(-1);});const fresh=await show(created.entry);await creating;await running(fresh,'docs');
  phase='plain text blank Open remains Editor';
  const empty=await open('');fs.writeFileSync(file('third.txt'),'Third.\n');await control('POST','/dialogs',{open:[file('third.txt')]});await run(empty.page,'open');await running(empty.page,'editor');assert.equal(await text(empty.page),'Third.\n');
  await flush(blank.page);const record=JSON.parse(fs.readFileSync(path.join(state,'recovery',require('node:crypto').createHash('sha256').update('file:editor:'+file('chosen.markdown')).digest('hex')+'.json'),'utf8'));assert.equal(record.path,file('chosen.markdown'));
  assert.deepEqual(errors,[]);console.log('PASS Markdown routing, rich view, save, conflict, Save As, recovery, permission, blank reuse, cross-format Open, Docs New and plain text control');
 } catch(e){console.error('FAIL in '+phase,errors);throw e;}
 finally{if(browser)await browser.close();if(control)await control('POST','/stop',{}).catch(()=>{});for(const p of [...windows,child].filter(Boolean)){if(p.exitCode!==null||p.signalCode)continue;const ended=new Promise(r=>p.once('exit',r));await Promise.race([ended,wait(5000)]);if(p.exitCode===null&&!p.signalCode){p.kill();await Promise.race([ended,wait(5000)]);}}console.log('Retained invented native launch artifacts: '+work);}
})().catch(e=>{console.error(e);process.exitCode=1;});
