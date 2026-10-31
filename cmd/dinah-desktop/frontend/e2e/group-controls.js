// Group reading controls share one wrapping toolbar above the member area.
const {launch,OUT,FRONTEND}=require('./lib/page');
const fs=require('fs'),path=require('path');
const choices=JSON.parse(fs.readFileSync(path.join(FRONTEND,'pudl/pudl-skins.json'),'utf8')).skins.filter(s=>s.admitted).map(({id,label})=>({id,label}));
(async()=>{
 const t=await launch({viewport:{width:1440,height:950},args:{choices},app:({choices})=>{
  const member={id:'member',chat:'docs',title:'Docs Pass',workspace:'docs',life:'open',window:true,exec:{state:'idle',why:'No work in progress'}};
  let next=0,state={version:1,active:'mineral',skin:'standard',themes:[],overrides:{}},revision=0;
  const message=()=>({id:++next,room:'controls',from:'Docs Pass',text:'Message '+next+'.\n\n'+Array.from({length:12},(_,n)=>'Paragraph '+n+' in this report.').join('\n\n'),createdAt:'2026-11-01T00:00:00Z',kind:'message'});
  const messages=Array.from({length:12},message);
  const room={name:'controls',title:'Group controls',members:[member],counts:{top:'idle',idle:1}};
  window.__appendReports=count=>{while(count--)messages.push(message());__emit('bus:changed',{});};
  const snapshot=()=>({state:structuredClone(state),revision:String(revision),skins:choices});
  return {State:async()=>({workspace:'C:/Fixture',recent:['C:/Fixture']}),AgentProviders:async()=>[],Sessions:async()=>[],SessionsAt:async()=>[],
   ReadUIState:async()=>({version:1,values:{'dinah-workbench-layout':'?open=workspace'}}),SaveUIState:async()=>{},
   ReadAppearance:async()=>snapshot(),SaveAppearance:async value=>{state=value;revision++;return snapshot();},
   BusOverview:async()=>({inbox:[],rooms:[room],workspaces:[],links:[]}),BusResolveGroup:async()=>({state:'resolved',room}),BusGroupMessages:async()=>messages.slice(),BusMarkGroupRead:async()=>{}};
 }});
 const {page:p,ok}=t,records=[];
 try{
  await p.evaluate(()=>dinahAppearance.ready);const key=await p.evaluate(()=>dinahWorkbench.open({kind:'bus-room',path:'controls'}));const win=p.locator('[data-win="'+key+'"]'),log=win.locator('.bus-log'),jump=win.getByRole('button',{name:'Jump to Where You Stopped',exact:true});
  await win.locator('[data-bus-message="12"]').waitFor({state:'attached'});
  const end=()=>p.waitForFunction(key=>{const log=document.querySelector('[data-win="'+key+'"] .bus-log');return log.dataset.following==='true'&&log.scrollHeight-log.scrollTop-log.clientHeight<24;},key);
  const geometry=()=>win.evaluate(win=>{
   const rect=el=>{const r=el.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom};};
   const shell=win.querySelector('.group-member-shell'),toolbar=win.querySelector('.group-toolbar'),separator=win.querySelector('.group-transcript-separator'),log=win.querySelector('.bus-log');
   const css=getComputedStyle(separator);
   return {shell:rect(shell),toolbar:rect(toolbar),separator:rect(separator),log:rect(log),controls:[...toolbar.querySelectorAll('button')].map(el=>({text:el.textContent,hidden:el.hidden,rect:rect(el)})),separatorPaint:{style:css.borderBlockStartStyle,width:parseFloat(css.borderBlockStartWidth),color:css.borderBlockStartColor},extraRow:!!win.querySelector('.transcript-unread-controls'),bodyAggregate:!!win.querySelector('.group-aggregate'),titleStatus:win.querySelector('.group-state-chip .chat-state')?.getAttribute('aria-label')};
  });
  for(const skin of choices)for(const mode of ['light','dark'])for(const motion of ['reduce','no-preference'])for(const width of [1440,560]){
   await p.setViewportSize({width,height:950});await p.emulateMedia({reducedMotion:motion});await p.evaluate(({id,mode,key})=>{pudlSetTheme(mode);pudlWindows.raise(key);pudlWindows.snap(key,'maximized');return dinahAppearance.selectSkin(id);},{id:skin.id,mode,key});
   for(const collapsed of [false,true]){
    const members=win.getByRole('button',{name:/^Members/});if((await members.getAttribute('aria-expanded')==='false')!==collapsed)await members.click();
    const latest=win.getByRole('button',{name:'Jump to the latest',exact:true});if(await latest.isVisible())await latest.click();await end();
    for(const shown of [false,true]){
     if(shown){await log.focus();await p.keyboard.press('Control+Home');await p.waitForFunction(key=>document.querySelector('[data-win="'+key+'"] .bus-log').scrollTop<2,key);const count=await win.locator('.bus-message').count();await p.evaluate(()=>__appendReports(4));await p.waitForFunction(({key,count})=>document.querySelectorAll('[data-win="'+key+'"] .bus-message').length===count+4,{key,count});await jump.waitFor();}
     const data=await geometry(),label=[skin.id,mode,motion,width,collapsed?'collapsed':'expanded',shown?'shown':'hidden'].join('-');records.push({label,...data});
     ok(label+' has exactly the four ordered controls',data.controls.length===4&&/^Members/.test(data.controls[0].text)&&data.controls[1].text==='Add Chats…'&&data.controls[2].text==='Turn Budget…'&&data.controls[3].text==='Jump to Where You Stopped'&&data.controls[3].hidden!==shown&&!data.extraRow);
     ok(label+' keeps title status without a duplicate body pill',!data.bodyAggregate&&data.titleStatus?.includes('Idle')&&await win.locator('[data-group-member="member"] .member-state').getAttribute('aria-label')!==null);
     ok(label+' separates the complete controls/member area from the transcript',data.separator.y>=data.shell.bottom-1&&data.log.y>=data.separator.bottom-1&&data.separatorPaint.style==='solid'&&data.separatorPaint.width>=1&&data.separatorPaint.color!=='rgba(0, 0, 0, 0)');
     ok(label+' wraps controls without horizontal overflow',await win.locator('.group-toolbar').evaluate(el=>el.scrollWidth<=el.clientWidth+1));
     if(motion==='reduce')await t.shot('group-controls-'+label);
     if(shown){const before=await log.evaluate(el=>el.scrollTop);if(collapsed){await members.focus();await p.keyboard.press('End');ok(label+' keyboard reaches the reading control',await jump.evaluate(el=>document.activeElement===el));await p.keyboard.press('Enter');}else await jump.click();await p.waitForFunction(({key,before})=>document.querySelector('[data-win="'+key+'"] .bus-log').scrollTop>before+1,{key,before});ok(label+' activates reading navigation by '+(collapsed?'keyboard':'ordinary click'),true);}
    }
   }
  }
 }catch(error){await t.shot('group-controls-failure');throw error;}finally{fs.writeFileSync(path.join(OUT,'group-controls-matrix.json'),JSON.stringify(records,null,2));await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
