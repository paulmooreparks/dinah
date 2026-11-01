// Both Open Team entry points resolve current membership without sending.
const {launch} = require('./lib/page');
(async () => {
  const t = await launch({viewport:{width:1440,height:1000},app:()=>{
    const cwd='C:/Fixture', member={id:'bus-member',chat:'member',title:'Member',life:'open',window:true,exec:{state:'idle',why:''}};
    const rooms=[{name:'team-a',title:'Original Team',members:[member]},{name:'team-b',title:'Other Workspace Team',members:[member]}];
    let lead={bus:'bus-lead-a',chat:'lead-a',title:'Lead A',group:'team-a',workspace:'Other workspace'},version=1;
    window.__navigationCalls=[];
    window.__membership=value=>{lead=value?{bus:'bus-lead-b',chat:'lead-b',title:'Lead B',group:'team-b',workspace:'Other workspace'}:null;__emit('agent:session',{sessionId:'member',cwd,provider:'codex',open:true,lead,stateGen:'membership',stateVer:++version});};
    window.__renameTeam=()=>{rooms[0].title='Renamed Team';__emit('bus:changed',{});};
    const info=id=>({sessionId:id,title:id,cwd,provider:'codex',open:true,working:false,lead:id==='member'?lead:null});
    return {State:async()=>({workspace:cwd,recent:[cwd]}),Sessions:async()=>[info('member'),info('ordinary')],SessionsAt:async()=>[info('member'),info('ordinary')],
      ReadUIState:async()=>({version:1,values:{'dinah-workbench-layout':'?open=workspace','dinah-draft:chat:member':{text:'Keep the member draft'}}}),SaveUIState:async()=>{},
      OpenSessionAt:async(cwd,id)=>{__navigationCalls.push(['open',id]);return info(id);},
      ChatLead:async id=>id==='member'?lead:null,
      Team:async id=>({lead:{chat:id,bus:'bus-'+id,title:id},group:id==='member'?'child-team':'',stateGen:'own-team',stateVer:1,counts:{top:'idle',idle:1},members:id==='member'?[{chat:'child',bus:'bus-child',title:'Child',life:'open',window:true,exec:{state:'idle',why:''}}]:[]}),
      BusOverview:async()=>({inbox:[],rooms,workspaces:[],links:[]}),BusResolveGroup:async name=>({state:'resolved',room:rooms.find(r=>r.name===name)}),BusGroupMessages:async()=>[],BusMarkGroupRead:async()=>{},
      BusSendToGroup:async()=>{__navigationCalls.push(['send']);},Send:async()=>{__navigationCalls.push(['provider-turn']);},Queue:async()=>{__navigationCalls.push(['provider-turn']);}};
  }});
  const {page:p,ok}=t;
  try {
    const key=await p.evaluate(()=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'member'})),win=p.locator('[data-win="'+key+'"]');
    const open=win.getByRole('button',{name:'Open Team',exact:true});await open.waitFor();
    ok('a team member’s Last Read comes before its team controls, at the bar’s left edge',await p.evaluate(k=>(win => { const shown = [...win.querySelectorAll('.chat-team button')].filter(el => el.getClientRects().length && !el.closest('.team-content')); const first = shown[0]; if (!first || !first.classList.contains('chat-last-read')) return false; const r = first.getBoundingClientRect(); return shown.slice(1).every(el => el.getBoundingClientRect().left >= r.right - 0.5); })(document.querySelector('[data-win="'+k+'"]')),key));
    await p.evaluate(()=>{for(let n=0;n<30;n++)__emit('agent:update',{sessionId:'member',update:{sessionUpdate:'agent_message_chunk',content:{type:'text',text:'Retained report '+n+'. '+'Paragraph text. '.repeat(12)+'\n\n'}}});});
    await win.getByText('Retained report 29.',{exact:false}).waitFor();
    const transcript=await win.locator('[data-view-id=logs]').textContent(),draft=await win.locator('[data-view-id=prompt]').inputValue();
    const activity=win.locator('[data-view-id=activity]');await activity.hover();await p.mouse.wheel(0,-600);
    await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
    const position=await activity.evaluate(el=>el.scrollTop);
    await open.click();const group=p.locator('.win[data-win-kind="bus-room"]');await group.getByText('Nothing Said Yet',{exact:true}).waitFor();
    const groupKey=await group.getAttribute('data-win');await group.locator('textarea').fill('Keep the group draft');
    await p.evaluate(()=>__renameTeam());await p.evaluate(key=>pudlWindows.raise(key),key);
    await p.locator('[data-menubar] .menubar-title').filter({hasText:/^Actions$/}).click();
    await p.locator('#menubar-panel .menu-action').filter({hasText:/^Open Team$/}).click();
    ok('Actions reveals the existing renamed team without duplicates',await group.count()===1&&await group.getAttribute('data-win')===groupKey&&await group.locator('textarea').inputValue()==='Keep the group draft');
    await p.evaluate(key=>pudlWindows.close(key),groupKey);await p.evaluate(key=>pudlWindows.raise(key),key);await open.focus();await open.press('Enter');await group.locator('textarea').waitFor();
    ok('keyboard Open Team reopens the canonical team with its draft',await group.count()===1&&await group.locator('textarea').inputValue()==='Keep the group draft');
    await p.evaluate(key=>pudlWindows.raise(key),key);
    ok('navigation preserves member draft, transcript and reading position',await win.locator('[data-view-id=prompt]').inputValue()===draft&&draft==='Keep the member draft'&&await win.locator('[data-view-id=logs]').textContent()===transcript&&Math.abs(await activity.evaluate(el=>el.scrollTop)-position)<2);
    ok('the Team body omits its duplicate aggregate',await win.locator('.team-aggregate').count()===0&&await win.locator('.team-stuck-chip .chat-state').count()===1);
    await p.evaluate(()=>__membership(true));await open.click();
    await p.waitForFunction(()=>[...document.querySelectorAll('.win[data-win-kind="bus-room"] .win-title')].some(el=>el.textContent.includes('Other Workspace Team')));
    ok('membership changes use the new canonical group identity',await p.locator('.win[data-win-kind="bus-room"]').count()===2);
    await p.evaluate(key=>pudlWindows.raise(key),key);await p.evaluate(()=>__membership(false));await open.waitFor({state:'hidden'});
    await p.locator('[data-menubar] .menubar-title').filter({hasText:/^Actions$/}).click();
    ok('ordinary chats have no misleading Open Team action',await p.locator('#menubar-panel .menu-action').filter({hasText:/^Open Team$/}).count()===0);
    await p.keyboard.press('Escape');
    ok('navigation sends no message and starts no provider turn',await p.evaluate(()=>!__navigationCalls.some(c=>c[0]==='send'||c[0]==='provider-turn')));
    await t.shot('team-navigation');
  } finally {await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
