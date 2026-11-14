// Today's surfaces use their existing applets and service fixtures in every admitted skin.
const {matrix}=require('./lib/today-themes-audit');
const {setup,makeRequest,choose}=require('./lib/decisions-fixture');
const updates=require('./lib/updates-fixture'),aside=require('./lib/set-aside-fixture'),features=require('./lib/whats-new-fixture'),answers=require('./lib/decision-answer-fixture');
const {launch}=require('./lib/page');
const only=process.env.DINAH_TODAY_SURFACE;
async function decisions(){const t=await setup([makeRequest('request:101'),makeRequest('mention:102',{kind:'mention',questions:[],text:'Please review the plan before we proceed.'}),makeRequest('request:103')]);const p=t.page;
try{await p.evaluate(()=>{__contextMessages=[{id:99,room:'#Review',from:'Design Review',text:'The candidate is ready.\nThe checks passed.\nThe third line stays collapsed.',createdAt:'2026-11-13T23:50:00Z'}];pudlWindows.snap('decisions','maximized');});
 await matrix(t,'decisions-list','.decisions');await p.setViewportSize({width:1440,height:1000});await choose(t,'request:101');await p.locator('.decisions-context-message').waitFor();
 await matrix(t,'decisions-context','.decisions');
 await p.evaluate(()=>dinahWorkbench.open('decisions','request=mention%3A102'));await p.locator('.decisions-detail h2').waitFor();await matrix(t,'decisions-mention','.decisions');
 await p.evaluate(()=>dinahCommands.run('broadcast-all'));await p.getByLabel('Broadcast Message',{exact:true}).fill('Please pause after the current task.');await p.getByRole('button',{name:'Preview Recipients',exact:true}).click();await p.locator('.decisions-preview li').last().waitFor();
 await matrix(t,'broadcast-preview','.decisions');
 await matrix(t,'broadcast-confirm','#ask',async()=>{if(!await p.locator('#ask').evaluate(el=>el.open)){await p.getByRole('button',{name:'Confirm Broadcast…',exact:true}).click();await p.locator('#ask[open]').waitFor();}});
 await p.click('#ask-no'); await p.evaluate(()=>{__rejectBroadcast=true;});await p.getByRole('button',{name:'Confirm Broadcast…',exact:true}).click();await p.locator('#ask[open]').waitFor();await p.click('#ask-yes');await p.waitForFunction(()=>document.querySelector('.decisions-receipt').textContent.includes('expired'));await matrix(t,'broadcast-error','.decisions');
}finally{await t.close();}}
async function notification(){const t=await features.start({updates:true}),p=t.page;try{
 await p.evaluate(()=>pudlWindows.snap('needs-you','maximized'));await features.card(t).waitFor();await matrix(t,'whats-new','#questions');
 await t.emit('notifications:added',{id:'theme-attention',kind:'mention',severity:'info',title:'Review Group asks you',body:'Please review the release and say when it can proceed.',group:'review',groupTitle:'Review Group',sessionId:'review',sourceId:'review',sessionTitle:'Release Review',message:101,created:'2026-11-14T00:00:00Z'});
 await p.locator('#questions .attention-kept').waitFor();await matrix(t,'attention','#questions',async()=>p.locator('.attention-kept').scrollIntoViewIfNeeded());
 await t.emit('update:state',{ready:false,pending:true,pausing:true,overdue:true,working:2,writing:1,blockers:['First Review','Second Review']});await p.locator('#questions [data-notice-id="dinah-pause-overdue"]').waitFor();
 await matrix(t,'pause-overdue','#questions',async()=>p.locator('#questions [data-notice-id="dinah-pause-overdue"]').scrollIntoViewIfNeeded());
 await matrix(t,'pause-note','#update-menu',async()=>{if(!await p.locator('#update-menu').evaluate(el=>el.matches(':popover-open')))await p.locator('#status-update').click();});await p.keyboard.press('Escape');
 await p.evaluate(()=>dinahWorkbench.open('help-chats'));await p.evaluate(()=>pudlWindows.snap('help-chats','maximized'));await matrix(t,'help-chats','[data-win="help-chats"] .win-body');
}finally{await t.close();}}
async function settings(){const t=await updates.start({asideItems:[aside.copy(1),aside.copy(2,{fallback:true,canRemove:false,reasonCode:'fallback',reason:'This is the newest fallback.'}),aside.copy(3,{runningPids:[123],canRemove:false,reasonCode:'in_use',reason:'A running process uses this copy.'})]}),p=t.page;try{
 await updates.settle(t);await p.evaluate(()=>pudlWindows.snap('settings','maximized'));await updates.setRelease(t,{latest:'2.1.0',available:true,notes:'This release improves the document tools.\nIt preserves your drafts.'});
 for(const name of ['programs','release','running','packages','repair']){if(name==='repair'){await p.locator('#updates-repair-check').click();}const selector='section[aria-labelledby="updates-'+name+'-title"]';await matrix(t,'updates-'+name,selector,async()=>p.locator(selector).scrollIntoViewIfNeeded());}
 await p.locator('#settings-tab-set-aside').click();await aside.ready(t);await matrix(t,'set-aside','#settings-set-aside',async()=>p.locator('#set-aside-card').scrollIntoViewIfNeeded());
 await p.locator('#settings-set-aside input:not(:disabled)').first().check();
 await matrix(t,'set-aside-confirm','#ask',async()=>{if(!await p.locator('#ask').evaluate(el=>el.open)){await p.locator('#set-aside-remove').click();await p.locator('#ask[open]').waitFor();}});await p.click('#ask-no');
}finally{await t.close();}}
async function answer(){const t=await answers.start(),p=t.page;try{await t.chat.locator('.msg:not(.msg-you)').waitFor();await answers.deliver(t,answers.entry(501));await p.locator('.decision-answer-card').waitFor();await p.evaluate(k=>pudlWindows.snap(k,'maximized'),t.chatKey);await matrix(t,'answer-card','.decision-answer-card',async()=>p.locator('.decision-answer-card').scrollIntoViewIfNeeded());await p.locator('.decision-answer-card a').click();await p.locator('.decisions-record').waitFor();await p.waitForFunction(()=>document.querySelector('.decisions-record').textContent.includes('This request is closed.'));await matrix(t,'closed-request','.decisions');}finally{await t.close();}}
async function standalone(){const t=await launch({goto:false,ready:false,app:()=>({Attach:async()=>({id:'fixture-docs',application:'editor',defaultApplet:'editor',applet:'docs',applicationName:'YAVC Editor',name:'fixture.md',resource:'C:/Fixture/fixture.md',headless:true,fileApplets:{'.md':'docs'}}),Read:async()=>({name:'fixture.md',text:'# A document\n\nA paragraph for the skin check.\n',revision:'r1',writable:true}),Directory:async()=> 'C:/Fixture',State:async()=>{},ConfigLoad:async()=>({}),ConfigSet:async(_,text)=>JSON.parse(text),PutSnapshot:async()=>{},ReadText:async()=>({found:false,text:''})})}),p=t.page;
try{await p.goto('http://dinah.test/document.html');await p.waitForFunction(()=>document.querySelector('#document').dataset.appletState==='running'&&!document.querySelector('#document').hasAttribute('data-applet-busy'));await matrix(t,'standalone-docs','body');await matrix(t,'standalone-help','#document-guide',async()=>{if(!await p.locator('#document-guide').evaluate(el=>el.open))await p.evaluate(()=>document.getElementById('document-help').click());});}finally{await t.close();}}
async function extras(){
 const t=await updates.start({},false),p=t.page;
 try{
  await p.evaluate(()=>dinahWorkbench.open('settings','chat'));await p.evaluate(()=>pudlWindows.snap('settings','maximized'));
  await matrix(t,'background-chats','section[aria-labelledby="set-background-chats"]',async()=>p.locator('#set-background-chat-limit').scrollIntoViewIfNeeded());
  await p.locator('#settings-tab-updates').click();await updates.settle(t);
  await p.evaluate(()=>{__fail.Check='The release could not be checked.';});await p.locator('#updates-check').click();await updates.settle(t);
  await matrix(t,'updates-release-error','section[aria-labelledby="updates-release-title"]',async()=>p.locator('#updates-release-status').scrollIntoViewIfNeeded());
  await p.evaluate(()=>{__u.packages.updates=[{name:'Writing skills',target:'me',from:'1.0.0',to:'1.1.0',bundle:'C:/Fixture/package.zip'},{name:'Project skills',target:'workspace',workspace:'C:/Fixture',from:'1.0.0',to:'1.1.0',bundle:'C:/Fixture/blocked.zip',reason:'Locally edited files are kept.'}];});await p.locator('#updates-package-check').click();
  await p.waitForFunction(()=>document.querySelectorAll('#updates-packages button').length===2);
  await matrix(t,'updates-packages-available','section[aria-labelledby="updates-packages-title"]',async()=>p.locator('#updates-packages').scrollIntoViewIfNeeded());
  await p.evaluate(()=>{__fail.PreviewAgentSettings='Agent settings could not be read.';});await p.locator('#updates-repair-check').click();await p.waitForFunction(()=>document.querySelector('#updates-repair-status').classList.contains('danger'));
  await matrix(t,'updates-repair-error','section[aria-labelledby="updates-repair-title"]',async()=>p.locator('#updates-repair-status').scrollIntoViewIfNeeded());
 }finally{await t.close();}
 const fs=require('fs'),path=require('path'),{FRONTEND}=require('./lib/page');
 const body=JSON.parse(fs.readFileSync(path.join(FRONTEND,'..','whats_new.go'),'utf8').match(/Body:\s*("(?:[^"\\]|\\.)*")/)[1]);
 const n=await features.start({updates:true,feature:{...features.feature,body}}),q=n.page;
 try{
  await q.evaluate(()=>pudlWindows.snap('needs-you','maximized'));await features.card(n).waitFor();await matrix(n,'whats-new','#questions .whats-new-card',async()=>features.card(n).scrollIntoViewIfNeeded());
  await n.emit('notifications:added',{id:'theme-attention-current',kind:'mention',severity:'info',title:'Review Group asks you',body:'Please review the release and say when it can proceed.',group:'review',groupTitle:'Review Group',sessionId:'review',sourceId:'review',sessionTitle:'Release Review',message:101,created:'2026-11-14T00:00:00Z'});
  await q.locator('#questions .attention-kept').waitFor();await matrix(n,'attention','#questions .attention-kept',async()=>q.locator('#questions .attention-kept').scrollIntoViewIfNeeded());
  await n.emit('update:state',{ready:false,pending:true,pausing:true,overdue:true,working:2,writing:1,blockers:['First Review','Second Review']});await q.locator('#questions [data-notice-id="dinah-pause-overdue"]').waitFor();
  await matrix(n,'pause-overdue','#questions [data-notice-id="dinah-pause-overdue"]',async()=>q.locator('#questions [data-notice-id="dinah-pause-overdue"]').scrollIntoViewIfNeeded());
 }finally{await n.close();}
}
(async()=>{for(const [name,run] of Object.entries({decisions,notification,settings,answer,standalone,extras}))if(!only||only===name)await run();})().catch(error=>{console.error(error);process.exitCode=1;});