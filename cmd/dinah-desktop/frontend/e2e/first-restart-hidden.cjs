// Synthetic historical-state rehearsal. No real home or provider is used.
const fs=require('node:fs'), path=require('node:path'), os=require('node:os'), http=require('node:http');
const assert=require('node:assert/strict');
const {spawn,execFileSync}=require('node:child_process');
const {createRequire}=require('node:module');
const {chromium}=createRequire('C:/Users/paul/source/repos/pudl/package.json')('playwright');
const [oldBin,newBin,seedExe]=process.argv.slice(2);
if(!oldBin||!newBin||!seedExe)throw Error('Supply both binary folders and the old-source bus fixture');
const root=fs.mkdtempSync(path.join(os.tmpdir(),'dinah-first-restart-'));
const home=path.join(root,'home'),bin=path.join(home,'bin'),entry=path.join(root,'entry.txt');
for(const d of ['home/bin','person','roaming','local','tmp','codex','claude','alpha','beta','gamma'])fs.mkdirSync(path.join(root,d),{recursive:true});
const result={root,checks:[],phases:[],errors:[]};
const check=(name,yes)=>{assert.ok(yes,name);result.checks.push(name);};
let browser,page,child;
const prompts=[];
const fake=http.createServer(async(req,res)=>{
 let text='';for await(const chunk of req)text+=chunk;
 res.setHeader('Content-Type','application/json');
 if(req.url==='/api/version')return res.end(JSON.stringify({version:'0.34.1'}));
 if(req.url==='/api/tags'||req.url==='/api/ps')return res.end(JSON.stringify({models:[{name:'fixture:latest',model:'fixture:latest',details:{}}]}));
 if(req.url==='/api/show')return res.end(JSON.stringify({capabilities:['completion','tools'],model_info:{'general.architecture':'fixture','fixture.context_length':32768}}));
 if(req.url==='/api/chat') {prompts.push(JSON.parse(text));return res.end(JSON.stringify({message:{role:'assistant',content:'Invented fixture response.'},done:true,done_reason:'stop'})+'\n');}
 res.statusCode=404;res.end('{}');
});
const env={SystemRoot:process.env.SystemRoot,WINDIR:process.env.WINDIR,COMSPEC:process.env.COMSPEC,PATH:path.join(process.env.SystemRoot,'System32'),USERPROFILE:path.join(root,'person'),HOME:path.join(root,'person'),APPDATA:path.join(root,'roaming'),LOCALAPPDATA:path.join(root,'local'),TEMP:path.join(root,'tmp'),TMP:path.join(root,'tmp'),CODEX_HOME:path.join(root,'codex'),CLAUDE_CONFIG_DIR:path.join(root,'claude'),DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entry};
const until=async(fn,label)=>{const end=Date.now()+60000;while(Date.now()<end){const v=await fn();if(v)return v;await new Promise(r=>setTimeout(r,100));}throw Error('Timed out: '+label);};
const call=(name,...args)=>page.evaluate(({name,args})=>go.main.App[name](...args),{name,args});
const settings={approvals:'dinah',defaultProvider:'ollama',hostNotificationsOff:true,promptSendKey:'ctrl-enter',showThinking:true,newWindowMode:'floating',ollamaApprovals:'ask',ollamaFileScope:'workspace',ollamaRounds:7,codexApprovals:'workspace',documentViews:{'text/markdown':'editor'},templates:[{name:'Invented template',text:'Invented template body'}]};
async function start(binaryFolder,phase){
 for(const n of ['dinah.exe','dinah-desktop.exe'])fs.copyFileSync(path.join(binaryFolder,n),path.join(bin,n));
 if(fs.existsSync(entry))fs.unlinkSync(entry);
 const log=fs.openSync(path.join(root,phase+'.log'),'w');
 child=spawn(path.join(bin,'dinah-desktop.exe'),[],{env,cwd:root,windowsHide:true,stdio:['ignore',log,log]});
 await until(()=>fs.existsSync(entry),'entry '+phase);
 browser=await chromium.launch({headless:true});page=await browser.newPage({viewport:{width:1440,height:1000}});
 page.on('pageerror',e=>result.errors.push({phase,error:e.message}));
 await page.route('**/*',route=>['127.0.0.1','localhost','[::1]'].includes(new URL(route.request().url()).hostname)?route.continue():route.abort());
 await page.goto(fs.readFileSync(entry,'utf8').trim());
 await page.waitForFunction(()=>window.dinahUIState&&!dinahUIState.isRestoring()&&window.dinahWorkbench);
}
async function close(){
 // This is the File > Exit runtime path. Keep the live page until its flush ACK closes the process.
 const diagnostic={status:await call('ExitStatus'),dialogs:await page.locator('dialog[open]').allTextContents()};
 diagnostic.dispatched=await page.evaluate(()=>dinahCommands.run('quit'));result.closeAttempts??=[];result.closeAttempts.push(diagnostic);
 if(!diagnostic.dispatched)throw Error('Quit command was unavailable or blocked by a dialog');
 try {await until(async()=>{
  if(child.exitCode!==null)return true;
  const pending=await call('PendingExitConfirmation').catch(()=>null);
  if(pending){diagnostic.confirmation=pending;await call('AnswerExitConfirmation',pending.id,true).catch(()=>{});}
  return child.exitCode!==null;
 },'normal process close');}catch(error){diagnostic.finalDialogs=await page.locator('dialog[open]').allTextContents();diagnostic.finalStatus=await call('ExitStatus');throw error;}
 await browser.close();browser=null;child=null;
}
async function snapshot(workspaces){
 await page.evaluate(()=>dinahUIState.flush());
 const state=await call('State');
 const sessions=[];for(const ws of workspaces)sessions.push(await call('SessionsAt',ws));
 return {state,sessions,ui:await call('ReadUIState'),notifications:await call('Notifications'),notificationStore:JSON.parse(fs.readFileSync(path.join(home,'notifications.json'),'utf8')),drafts:await call('ListDrafts'),overview:await call('BusOverview'),windows:await page.evaluate(()=>({open:pudlWindows.state().open,pins:pudlWindows.taskbarState().pins}))};
}
(async()=>{try{
 await new Promise(resolve=>fake.listen(0,'127.0.0.1',resolve));env.OLLAMA_HOST='http://127.0.0.1:'+fake.address().port;
 fs.writeFileSync(path.join(home,'settings.json'),JSON.stringify(settings));
 await start(oldBin,'old');
 const workspaces=['alpha','beta','gamma'].map(n=>path.join(root,n));
 const chats=[];
 for(let i=0;i<workspaces.length;i++){
  await call('SetWorkspace',workspaces[i]);
  const chat=await call('CreateNamedChat',workspaces[i],'ollama','','Invented Chat '+i);chats.push(chat);
 }
 result.createdChats=chats;
 await call('Send',chats[0].sessionId,'Invented history before restart',null,null);
 await until(async()=>prompts.length>0&&(await call('UpdateState')).working===0,'invented completed turn');
 result.oldChatHistory=await call('ChatHistory',chats[0].sessionId);
 const keys=[];
 for(let i=0;i<chats.length;i++)keys.push(await page.evaluate(async({workspace,id})=>dinahWorkbench.open({kind:'chat-session',workspace,path:id}),{workspace:workspaces[i],id:chats[i].sessionId}));
 await page.evaluate(key=>pudlWindows.close(key),keys[2]);
 await page.evaluate(async keys=>{await pudlWindows.refreshTaskbarDestinations();for(const key of keys)pudlWindows.pinTaskbar(key,true);},keys.slice(0,2));
 await page.evaluate(()=>dinahWorkbench.open('settings'));
 await page.evaluate(()=>dinahWorkbench.open('needs-you'));
 const group=await call('BusStartGroup','Invented Team',chats.map((c,i)=>({id:c.sessionId,cwd:workspaces[i],title:'Invented Chat '+i})));
 await call('BusSendToGroup',group.name,'Invented historical group message',[],[]);
 const unread=JSON.parse(execFileSync(seedExe,[workspaces[0],'create','Invented unread before restart'],{env:{...env,TEMP:os.tmpdir(),TMP:os.tmpdir()},cwd:root,windowsHide:true,encoding:'utf8'}));
 result.unreadGroup=unread.group;
 await call('SaveNotification',{id:'quality-kept-notice',kind:'app',severity:'info',title:'Invented notice',body:'Keep this invented notice',created:'2026-11-13T02:28:00Z'});
 await call('SaveNotification',{id:'quality-dismissed-notice',kind:'app',severity:'warning',title:'Invented dismissed notice',body:'Keep its dismissal',created:'2026-11-13T02:29:00Z'});
 await call('DismissNotification','quality-dismissed-notice');
 await page.evaluate(key=>pudlWindows.open(key),keys[0]);
 await page.locator('.win[data-win="'+keys[0]+'"] [data-view-id="prompt"]').fill('Invented unsent draft');
 let draft=await call('NewEmptyDraft','');result.newDraft=draft;
 draft=await call('UpdateDraft',draft.id,draft.revision,'Invented unsaved document');result.documentDraft=draft;
 const before=await snapshot(workspaces);result.phases.push({name:'old',snapshot:before});
 check('old unread fixture exists',(await call('BusGroup',unread.group)).unread>0);
 await close();check('old closes through its normal runtime and layout flush',true);
 result.oldSavedSettings=JSON.parse(fs.readFileSync(path.join(home,'settings.json'),'utf8'));
 for(let phase=0;phase<2;phase++){
  await start(newBin,'new-'+phase);
  const after=await snapshot(workspaces);result.phases.push({name:'new-'+phase,snapshot:after});
  for(let i=0;i<chats.length;i++)check('chat '+i+' preserved in pass '+phase,after.sessions[i].some(c=>c.sessionId===chats[i].sessionId));
  assert.deepEqual(await call('ChatHistory',chats[0].sessionId),result.oldChatHistory);check('chat history preserved pass '+phase,true);
  for(const key of before.windows.open)check('open window preserved '+key+' pass '+phase,after.windows.open.includes(key));
  check('closed window stays closed pass '+phase,!after.windows.open.includes(keys[2]));
  assert.deepEqual(after.windows.pins,before.windows.pins);check('pins preserved pass '+phase,true);
  for(const n of before.notifications.filter(n=>n.id.startsWith('quality-'))){const kept=after.notifications.find(v=>v.id===n.id);assert.deepEqual(kept,n);}
  check('notifications and dismissal preserved pass '+phase,true);
  for(const n of before.notificationStore.items.filter(n=>n.id.startsWith('quality-')))assert.deepEqual(after.notificationStore.items.find(v=>v.id===n.id),n);
  assert.deepEqual(after.ui.values['dinah-draft:chat:'+chats[0].sessionId],before.ui.values['dinah-draft:chat:'+chats[0].sessionId]);
  check('composer draft preserved pass '+phase,true);
  const keptDraft=await call('GetDraft',draft.id);assert.equal(keptDraft.text,'Invented unsaved document');check('document draft preserved pass '+phase,true);
  const currentSettings=JSON.parse(fs.readFileSync(path.join(home,'settings.json'),'utf8'));
  for(const key of Object.keys(settings))assert.deepEqual(currentSettings[key],result.oldSavedSettings[key],key);
  check('historical settings preserved pass '+phase,true);
  check('unread group stays unread pass '+phase,(await call('BusGroup',unread.group)).unread>0);
  const delivered=JSON.parse(execFileSync(seedExe,[workspaces[0],unread.group,'Invented delivery after restart '+phase],{env:{...env,TEMP:os.tmpdir(),TMP:os.tmpdir()},cwd:root,windowsHide:true,encoding:'utf8'}));
  assert.ok(Number.isSafeInteger(delivered.message.ID));
  check('bus delivers after restart '+phase,(await call('BusGroupMessages',unread.group)).some(m=>m.id===delivered.message.ID&&m.text===delivered.message.Text));
  if(phase===0){
   await page.evaluate(()=>dinahWorkbench.open('decisions'));await page.locator('.decisions').waitFor({state:'visible'});
   check('Decisions opens on carried state',true);
   await page.evaluate(()=>dinahWorkbench.open('settings'));await page.locator('#settings-tab-set-aside').click();await page.locator('#settings-set-aside').waitFor({state:'visible'});
   check('Set-aside Programs opens on carried state',true);
   result.restart=await call('UpdateState');check('restart state service works',typeof result.restart==='object');
   await call('CancelRestart');
   await page.screenshot({path:path.join(root,'carried-state.png')});
  }
  if(phase===1){
   const previousEntry=fs.readFileSync(entry,'utf8').trim();
   await page.evaluate(()=>dinahCommands.run('pause-and-restart')).catch(()=>{});
   await until(()=>child.exitCode!==null,'Pause Chats and Restart old process closes');
   const next=await until(()=>{const s=fs.readFileSync(entry,'utf8').trim();return s!==previousEntry&&s;},'restarted entry');
   await page.goto(next);await page.waitForFunction(()=>window.dinahUIState&&!dinahUIState.isRestoring());
   const resumed=await snapshot(workspaces);result.pauseRestart=resumed;
   check('Pause Chats and Restart launches a fresh hidden process',true);
   check('pause restart keeps the document draft',resumed.drafts.some(d=>d.id===draft.id&&d.text===draft.text));
   await page.evaluate(()=>dinahCommands.run('quit')).catch(()=>{});
   await until(async()=>{try{await fetch(next);return false;}catch{return true;}},'restarted process closes normally');
   await browser.close();browser=null;child=null;
  }else await close();
 }
 const a=result.phases[1].snapshot,b=result.phases[2].snapshot;
 const sessionIDs=s=>s.sessions.map(list=>list.map(c=>c.sessionId).sort());
 assert.deepEqual(sessionIDs(b),sessionIDs(a),'sessions duplicated on second start');
 assert.deepEqual(b.drafts,a.drafts,'drafts duplicated or changed on second start');
 for(const id of ['quality-kept-notice','quality-dismissed-notice','whats-new-20261114'])check('no duplicate notice '+id,b.notifications.filter(n=>n.id===id).length<=1);
 check('no duplicate sessions or drafts on second start',true);result.passed=true;
}catch(e){result.failure=e.stack;result.passed=false;process.exitCode=1;}finally{
 if(browser)await browser.close();if(child&&child.exitCode===null)child.kill();fake.close();
 const ownedExe=path.join(bin,'dinah-desktop.exe').replaceAll("'","''");
 const cleanup=`Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq '${ownedExe}' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`;
 await new Promise(resolve=>spawn('powershell.exe',['-NoProfile','-NonInteractive','-Command',cleanup],{windowsHide:true,stdio:'ignore'}).on('exit',resolve));
 fs.writeFileSync(path.join(root,'result.json'),JSON.stringify(result,null,2));console.log(JSON.stringify({root,passed:result.passed,checks:result.checks,failure:result.failure},null,2));
}})();
