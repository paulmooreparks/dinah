// Member updates retain the presented controls and their geometry on every frame.
const { launch } = require('./lib/page');
const fs = require('fs');
const path = require('path');
(async () => {
 const t = await launch({viewport:{width:1440,height:1000},app:()=>{
  let version=1, used=0;
  const cwd='C:/Fixture', ids=['alpha','beta','gamma','delta','epsilon','zeta','eta'];
  let state={life:'open',window:true,exec:{state:'idle',why:'No work in progress'}};
  const chats=()=>ids.map(id=>({sessionId:id,title:'Member '+id,provider:'codex',cwd,open:state.life==='open',working:state.exec?.state==='working',chatState:structuredClone(state)}));
  const counts=()=>({top:state.exec?.state||'idle',working:0,waiting:0,stuck:0,idle:0,closed:0,unknown:0,[state.exec?.state||(state.elsewhere?'unknown':'closed')]:ids.length});
  const room=()=>({name:'stable',title:'Member stability',budget:20,used,members:[...chats().map(c=>({id:'bus-'+c.sessionId,bus:'bus-'+c.sessionId,chat:c.sessionId,title:c.title,workspace:'Fixture',...c.chatState})),{id:'paul',title:'Paul',state:'person'}],counts:counts()});
  const snapshot=()=>({stateGen:'member-stability',stateVer:version,chats:chats().map(c=>({sessionId:c.sessionId,bus:'bus-'+c.sessionId,...c.chatState})),groups:[{name:'stable',counts:counts()}]});
  window.__memberStep=(kind)=>{
   version++;
   state=kind==='closed'||kind==='retired'?{life:kind,window:false}:kind==='elsewhere'?{life:'open',window:false,elsewhere:'Running in another Dinah window'}:kind==='unknown'?{life:'open',window:true}:{life:'open',window:kind!=='background',exec:{state:kind==='background'?'idle':kind,why:(kind==='waiting'||kind==='stuck'?'A long reason that must stay available without changing any chip width. '.repeat(6):'Turn state changed')}};
   __emit('agent:chatstate',structuredClone(snapshot())); __emit('bus:changed',{});
  };
  window.__memberBudget=(n)=>{used=20-n;__emit('bus:changed',{});};
  return {State:async()=>({workspace:cwd,workspaces:[cwd],recent:[cwd]}),Sessions:async()=>chats(),SessionsAt:async()=>chats(),ChatStates:async()=>snapshot(),ChatLead:async()=>null,Team:async()=>null,BusOverview:async()=>({rooms:[room()],inbox:[],unread:0,workspaces:[],links:[]}),BusGroupMessages:async()=>[{id:1,room:'stable',from:'Member alpha',text:'The transcript stays below the member area.',createdAt:'2026-10-31T00:00:00Z'}],BusMarkGroupRead:async()=>{},ProviderModels:async()=>[],ProviderModelsAt:async()=>[]};
 }});
 const p=t.page, evidence=[];
 try {
  const key=await p.evaluate(()=>dinahWorkbench.open({kind:'bus-room',path:'stable'}));
  const win=p.locator('[data-win="'+key+'"]');
  await win.locator('[data-group-member="bus-alpha"] button').waitFor();
  await win.locator('[data-win-action="maximize"]').click();
  const tokens=await p.evaluate(()=>['2xs','xs','sm','md','base','lg','xl','2xl','3xl'].map(s=>['--text-'+s,getComputedStyle(document.documentElement).getPropertyValue('--text-'+s)]));
  for(const theme of ['light','dark']) for(const width of [1440,700,390]) for(const scale of [1,2]) {
   await p.setViewportSize({width,height:1000});
   await p.evaluate(({theme,tokens,scale})=>{pudlSetTheme(theme);tokens.forEach(([name,value])=>document.documentElement.style.setProperty(name,parseFloat(value)*scale+'px'));__memberStep('idle');__memberBudget(9);},{theme,tokens,scale});
   await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(r)))));
   const button=win.locator('[data-group-member="bus-alpha"] button');
   await button.focus();
   const trace=await p.evaluate(async(key)=>{
    const root=document.querySelector('[data-win="'+key+'"]'), first=root.querySelector('[data-group-member="bus-alpha"] button');
    const buttons=[...root.querySelectorAll('.member-menu-btn')];
    function sample(){return [...root.querySelectorAll('.group-member-shell,.member-chip,.member-menu-btn,.group-composer,.bus-log,.group-aggregate')].map((el,i)=>{const r=el.getBoundingClientRect();return {key:el.dataset.groupMember||el.id||el.className+'|'+i,x:r.x,y:r.y,w:r.width,h:r.height};});}
    const before=sample(), samples=[], failures=[];
    for(const state of ['working','waiting','stuck','idle','closed','retired','elsewhere','unknown','background']) {
     __memberStep(state);
     for(let frame=0;frame<8;frame++) {
      await new Promise(requestAnimationFrame);const now=sample();samples.push({state,frame,rectangles:now});
      now.forEach((r,i)=>{const b=before[i];if(!b||r.key!==b.key||['x','y','w','h'].some(k=>Math.abs(r[k]-b[k])>.02))failures.push({state,frame,before:b,after:r});});
      if(document.activeElement!==first||buttons.some(b=>!b.isConnected))failures.push({state,frame,focus:'member controls were replaced or focus was lost'});
     }
     const mark=root.querySelector('[data-group-member="bus-alpha"] .member-state'), expected=['closed','retired','elsewhere'].includes(state)?'chat-lifecycle':'chat-state-'+(state==='background'?'idle':state);
     if(!mark.classList.contains(expected))failures.push({state,painted:mark.className,expected});
     if(state==='background'&&!mark.title.includes('No window'))failures.push({state,detail:mark.title});
     const glyph=mark.querySelector('.glyph');if(glyph&&glyph.getBoundingClientRect().width<16)failures.push({state,glyphWidth:glyph.getBoundingClientRect().width});
     if(!glyph&&mark.scrollHeight>mark.clientHeight+1)failures.push({state,clippedLifecycle:true});
     const aggregate=getComputedStyle(root.querySelector('.group-aggregate .chat-state'),'::before');
     if(aggregate.display!=='inline-block'||parseFloat(aggregate.width)<16||aggregate.maskImage==='none')failures.push({state,aggregateGlyph:{display:aggregate.display,width:aggregate.width,mask:aggregate.maskImage}});
    }
    for(const remaining of [10,1,0,9]) {
     __memberBudget(remaining);
     for(let frame=0;frame<8;frame++) {await new Promise(requestAnimationFrame);const now=sample();samples.push({remaining,frame,rectangles:now});now.forEach((r,i)=>{const b=before[i];if(!b||['x','y','w','h'].some(k=>Math.abs(r[k]-b[k])>.02))failures.push({remaining,frame,before:b,after:r});});}
    }
    return {before,samples,failures};
   },key);
   t.ok('member controls and conversation stay fixed on every frame '+theme+'/'+width+'/'+scale,trace.failures.length===0,JSON.stringify(trace.failures.slice(0,2)));
   await button.click();
   const menu=win.locator('.group-member-menu');await menu.waitFor();
   const action=menu.getByRole('menuitem',{name:'Open Chat',exact:true});await action.focus();
   const menuFrames=await menu.evaluate(async(el)=>{const actions=[...el.querySelectorAll('button')],before=actions.map(b=>b.getBoundingClientRect().toJSON()),samples=[],failures=[];__memberStep('stuck');for(let frame=0;frame<8;frame++){await new Promise(requestAnimationFrame);samples.push(actions.map(b=>b.getBoundingClientRect().toJSON()));actions.forEach((b,i)=>{const r=b.getBoundingClientRect();if(['x','y','width','height'].some(k=>Math.abs(r[k]-before[i][k])>.02))failures.push({frame,before:before[i],after:r.toJSON()});});}return {before,samples,failures};});
   t.ok('an open member menu keeps its action geometry '+theme+'/'+width+'/'+scale,menuFrames.failures.length===0,JSON.stringify(menuFrames.failures.slice(0,2)));
   await menu.locator('.member-menu-state').filter({hasText:'A long reason'}).waitFor();
   t.ok('an open member menu keeps focus and reads fresh state '+theme+'/'+width+'/'+scale,await action.evaluate(el=>document.activeElement===el));
   await p.mouse.move(1,1);await t.shot('member-menu-'+theme+'-'+width+'-'+scale);
   await p.keyboard.press('Escape');
   t.ok('Escape returns to the retained member button '+theme+'/'+width+'/'+scale,await button.evaluate(el=>document.activeElement===el));
   await p.mouse.move(1,1);
   await win.locator('.group-composer textarea').focus();
   await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
   await t.shot('member-stability-'+theme+'-'+width+'-'+scale);
   evidence.push({theme,width,scale,...trace,menu:menuFrames});
  }
  fs.writeFileSync(path.join(process.argv[3],'member-stability.json'),JSON.stringify(evidence,null,2));
 } finally {await t.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
