// Original turn/item identity survives notices, completion and saved/replay paths.
const {launch,OUT,PAGE}=require('./lib/page');
const fs=require('fs'),path=require('path');
(async()=>{
 const t=await launch({goto:false,viewport:{width:1440,height:950},app:()=>{
  const update=(turn,item,mode,text,kind='agent_message_chunk')=>({sessionUpdate:kind,turnId:turn,itemId:item,contentMode:mode,content:{type:'text',text}});
  const notice={sessionUpdate:'dinah_delivered',message:{role:'delivered',text:'INTERVENING-NOTICE',delivered:{id:71,kind:'text',from:'Review',time:'2026-11-01T00:00:00Z'}}};
  const events=[update('original','old','append','Earlier partial'),notice,update('original','old','append',' continued'),update('original','new','append','New commentary'),update('original','new','append','Reasoning only','agent_thought_chunk'),update('original','new','snapshot','New commentary complete'),update('original','new','append',' LATE-IGNORED'),update('original','old','snapshot','Earlier partial continued complete'),update('original','new','snapshot','New commentary complete'),update('other-turn','new','snapshot','Other turn commentary'),update('original','empty','append','TO-BE-ERASED'),update('original','empty','snapshot',''),update('original','empty','append','DO-NOT-RESURRECT'),{sessionUpdate:'agent_message_chunk',content:{type:'text',text:'Legacy A'}},{sessionUpdate:'agent_message_chunk',turnId:'original',contentMode:'snapshot',content:{type:'text',text:' B'}},{sessionUpdate:'agent_message_chunk',turnId:'original',itemId:'new',contentMode:'legacy',content:{type:'text',text:' C'}}];
  window.__boundaryEvents=events;window.__boundaryUpdate=update;
  window.__sendBoundary=(id,list)=>list.forEach(u=>__emit('agent:update',{sessionId:id,update:u}));
  const saved=[{role:'agent',text:'Earlier partial continued complete',turnId:'original',itemId:'old',messageComplete:true},notice.message,{role:'agent',text:'New commentary complete',turnId:'original',itemId:'new',messageComplete:true},{role:'agent',text:'Other turn commentary',turnId:'other-turn',itemId:'new',messageComplete:true},{role:'agent',text:'',turnId:'original',itemId:'empty',messageComplete:true},{role:'agent',text:'Legacy A B C'}];
  return {ReadUIState:async()=>({version:1,values:{'dinah-workbench-layout':'?open=workspace'}}),SaveUIState:async()=>{},
   SessionContinuation:async()=>null,SavedConversation:async id=>id==='partial-saved'?{available:true,complete:true,streamGen:'partial-journal',streamThrough:1,messages:[{role:'agent',text:'Partial ',turnId:'partial-turn',itemId:'partial-item',messageComplete:false}]}:id==='saved'?{available:true,complete:true,messages:saved}:id==='legacy-saved'?{available:true,complete:true,messages:[{role:'agent',text:'First legacy row'},{role:'agent',text:'Second legacy row'}]}:{available:false},
   ChatEvents:async(id,gen)=>({gen,head:2,events:id==='partial-saved'?[{n:2,name:'agent:update',data:{update:update('partial-turn','partial-item','append','continued')}}]:[]}),
   LoadFullHistory:async(cwd,sessionId)=>{[update('original','old','snapshot','RESET-ORIGINAL'),update('original','new','snapshot','RESET-NEW')].forEach((u,i)=>__emit('agent:update',{sessionId,gen:'full-reset',n:i+1,update:u}));return {sessionId,cwd,provider:'codex',open:true,streamGen:'full-reset',streamHead:2};},
   OpenSessionAt:async(cwd,sessionId)=>{if(sessionId==='saved'||sessionId==='legacy-saved')return new Promise(()=>{});if(sessionId==='partial-saved')return {sessionId,cwd,provider:'codex',open:true,resumed:true,streamGen:'partial-journal',streamHead:2};if(sessionId==='replay')__emit('agent:updates',{sessionId,updates:events});return {sessionId,cwd,provider:'codex',title:sessionId,open:true,working:false,queued:[]};}};
 }});
 const {page:p,ok}=t,evidence={};
 async function open(id){const key=await p.evaluate(id=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:id}),id);return {key,win:p.locator('[data-win="'+key+'"]')};}
 const texts=win=>win.locator('[data-view-id=logs] .msg-row:not([hidden]) .msg-text').allTextContents();
 try{
  if(process.env.DINAH_BOUNDARY_SCRIPT)await t.context.route('http://dinah.test/chat-applet.js',route=>route.fulfill({path:process.env.DINAH_BOUNDARY_SCRIPT}));
  if(process.env.DINAH_BOUNDARY_LIFECYCLE)await t.context.route('http://dinah.test/session-lifecycle.js',route=>route.fulfill({path:process.env.DINAH_BOUNDARY_LIFECYCLE}));
  await p.goto(PAGE);await t.waitReady();
  const live=await open('live');await live.win.locator('[data-view-id=prompt]').waitFor();await p.evaluate(()=>__sendBoundary('live',__boundaryEvents));
  await live.win.getByText('Other turn commentary',{exact:true}).waitFor();await live.win.getByText('Legacy A B C',{exact:true}).waitFor();
  const expected=['Earlier partial continued complete','New commentary complete','Other turn commentary','Legacy A B C'];
  evidence.live=await texts(live.win);ok('routed live output keeps original items separate around notices',JSON.stringify(evidence.live)===JSON.stringify(expected),JSON.stringify(evidence.live));
  ok('reasoning has its own accumulator',await live.win.locator('.thought').textContent()==='Reasoning only');
  ok('an empty authoritative snapshot clears text and rejects late appends',!/(TO-BE-ERASED|DO-NOT-RESURRECT|LATE-IGNORED)/.test(await live.win.locator('[data-view-id=logs]').innerText()));
  const order=await live.win.locator('[data-view-id=logs]').innerText();ok('late old output updates its existing bubble before the notice',order.indexOf('Earlier partial continued complete')<order.indexOf('INTERVENING-NOTICE')&&order.indexOf('INTERVENING-NOTICE')<order.indexOf('New commentary complete'));
  await p.evaluate(()=>{__emit('agent:turn',{sessionId:'live',stopReason:'steered'});__sendBoundary('live',[__boundaryUpdate('original','old','snapshot','Earlier partial continued complete')]);});
  await p.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  ok('completion after a steering interruption retains the original item row',JSON.stringify(await texts(live.win))===JSON.stringify(expected));
  await t.shot('item-boundaries-live');
  const replay=await open('replay');await replay.win.getByText('Legacy A B C',{exact:true}).waitFor();evidence.replay=await texts(replay.win);ok('batched replay has the same item text and boundaries',JSON.stringify(evidence.replay)===JSON.stringify(expected),JSON.stringify(evidence.replay));
  const saved=await open('saved');await saved.win.getByText('Legacy A B C',{exact:true}).waitFor();evidence.saved=await texts(saved.win);ok('saved complete rows match live and replay boundaries',JSON.stringify(evidence.saved)===JSON.stringify(expected),JSON.stringify(evidence.saved));
  const partial=await open('partial-saved');await partial.win.getByText('Partial continued',{exact:true}).waitFor();
  await p.evaluate(()=>__emit('agent:update',{sessionId:'partial-saved',gen:'partial-journal',n:3,update:__boundaryUpdate('partial-turn','partial-item','snapshot','Partial continued complete')}));
  await partial.win.getByText('Partial continued complete',{exact:true}).waitFor();ok('an incomplete saved item accepts catch-up append and authoritative completion once',JSON.stringify(await texts(partial.win))===JSON.stringify(['Partial continued complete']));
  const legacy=await open('legacy-saved');await legacy.win.getByText('Second legacy row',{exact:true}).waitFor();ok('existing saved rows retain their explicit boundaries',JSON.stringify(await texts(legacy.win))===JSON.stringify(['First legacy row','Second legacy row']));
  await p.evaluate(key=>pudlWindows.close(key),live.key);await p.evaluate(()=>__sendBoundary('live',[__boundaryUpdate('original','new','append','CLOSED-LATE-APPEND')]));await open('live');ok('reopening a retained log preserves finalized identity',!(await texts(live.win)).some(text=>text.includes('CLOSED-LATE-APPEND')));
  await p.evaluate(key=>pudlWindows.raise(key),live.key);
  await live.win.locator('.msg-row').first().evaluate(el=>{window.__preResetRow=el;});
  await p.evaluate(()=>__emit('agent:session',{sessionId:'live',provider:'codex',cwd:'C:/Fixture',title:'live',open:true,working:false,resumed:true,streamGen:'before-reset',streamHead:0}));
  await p.locator('[data-menubar] .menubar-title').filter({hasText:/^Actions$/}).click();await p.locator('#menubar-panel .menu-action').filter({hasText:'Load Full History'}).first().click();
  await live.win.getByText('RESET-NEW',{exact:true}).waitFor();
  ok('replay reset replaces DOM rows and restores only its current items',await p.evaluate(()=>!__preResetRow.isConnected)&&JSON.stringify(await texts(live.win))===JSON.stringify(['RESET-ORIGINAL','RESET-NEW']));
  await p.evaluate(()=>{__emit('agent:update',{sessionId:'live',gen:'before-reset',n:1,update:__boundaryUpdate('original','new','append','STALE-GENERATION')});__emit('agent:update',{sessionId:'live',gen:'full-reset',n:3,update:__boundaryUpdate('original','new','append','AFTER-RESET-COMPLETION')});});
  await p.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  ok('stale generations and finalized late appends cannot mutate reset rows',JSON.stringify(await texts(live.win))===JSON.stringify(['RESET-ORIGINAL','RESET-NEW']));
  const trim=await open('trim');await trim.win.locator('[data-view-id=prompt]').waitFor();
  await p.evaluate(()=>__sendBoundary('trim',Array.from({length:650},(_,n)=>__boundaryUpdate('trim-turn','item-'+n,'snapshot','TRIM-ITEM-'+n))));
  await trim.win.getByText('TRIM-ITEM-649',{exact:true}).waitFor();await p.waitForFunction(key=>document.querySelectorAll('[data-win="'+key+'"] [data-view-id=logs] > .activity-row').length<=600,trim.key);
  const count=await trim.win.locator('[data-view-id=logs] .msg-row').count();await p.evaluate(()=>__sendBoundary('trim',[__boundaryUpdate('trim-turn','item-0','append','REVIVED-OLD-APPEND'),__boundaryUpdate('trim-turn','item-0','snapshot','REVIVED-OLD-SNAPSHOT')]));await p.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  ok('trimmed identities cannot revive removed history',await trim.win.locator('[data-view-id=logs] .msg-row').count()===count&&!/REVIVED-OLD/.test(await trim.win.locator('[data-view-id=logs]').innerText()));
  evidence.trim={retained:count};await t.shot('item-boundaries-trim');
 }catch(error){evidence.failure=await p.locator('.win[data-chat-current] [data-view-id=logs]').allTextContents();await t.shot('item-boundaries-failure');throw error;}finally{fs.writeFileSync(path.join(OUT,'message-boundaries.json'),JSON.stringify(evidence,null,2));await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
