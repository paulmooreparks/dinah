// View uses the Settings catalogue and transaction; Unknown retains its semantics.
const {launch,OUT,FRONTEND}=require('./lib/page');
const fs=require('fs'),path=require('path');
const choices=JSON.parse(fs.readFileSync(path.join(FRONTEND,'pudl/pudl-skins.json'),'utf8')).skins.filter(s=>s.admitted).map(({id,label})=>({id,label}));
(async()=>{
 const t=await launch({viewport:{width:1440,height:950},args:{choices},app:({choices})=>{
  let state={version:1,active:'mineral',skin:'standard',themes:[],overrides:{}},revision=0;
  window.__saveFailure=false;window.__skinSaves=[];
  const snapshot=()=>({state:structuredClone(state),revision:String(revision),skins:choices});
  window.__externalSkin=skin=>{state.skin=skin;revision++;__emit('appearance:changed',{revision:String(revision)});};
  const member={id:'bus-unknown',chat:'unknown',title:'Unknown Member',life:'open',window:true};
  const room={name:'appearance-followup',title:'Appearance Review',members:[member],counts:{unknown:1}};
  return {ReadAppearance:async()=>snapshot(),SaveAppearance:async(next,expected)=>{if(__saveFailure)throw Error('Appearance storage unavailable');if(expected!==String(revision))throw Error('Revision conflict');state=next;revision++;__skinSaves.push(next.skin);return snapshot();},
   SavedConversation:async()=>({available:false}),OpenSessionAt:async(cwd,sessionId)=>({sessionId,cwd,provider:'codex',title:'Unknown Chat',open:true,chatState:{life:'open',window:true}}),
   BusOverview:async()=>({inbox:[],rooms:[room],workspaces:[],links:[]}),BusResolveGroup:async()=>({state:'resolved',room}),BusGroupMessages:async()=>[],BusMarkGroupRead:async()=>{}};
 }});
 const {page:p,ok}=t,records=[];
 async function skinMenu(){await p.locator('.menubar-title:visible').filter({hasText:/^View$/}).click();const panel=p.locator('#menubar-panel');await panel.waitFor({state:'visible'});await panel.locator('[role^="menuitem"]').filter({hasText:/^Skin$/}).click();return panel;}
 async function choose(skin){const panel=await skinMenu();await panel.getByRole('menuitemradio',{name:skin.label,exact:true}).click();}
 try{
  await p.evaluate(()=>dinahAppearance.ready);
  const chat=await p.evaluate(()=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'unknown'}));
  const group=await p.evaluate(()=>dinahWorkbench.open({kind:'bus-room',path:'appearance-followup'}));
  const prompt=p.locator('[data-win="'+chat+'"] [data-view-id=prompt]');await prompt.fill('Retain this draft across menu skin changes.');
  await prompt.evaluate(el=>{window.__retainedPrompt=el;window.__retainedWindows=[...document.querySelectorAll('.dinah-applet')];});
  await p.evaluate(key=>pudlWindows.raise(key),chat);
  ok('View and Settings use the same catalogue labels',JSON.stringify(await p.locator('#host-skins > li > button').allTextContents())===JSON.stringify(choices.map(s=>s.label)));
  ok('unknown execution still has its accessible explanation',await p.evaluate(()=>{const d=dinahChatState.describe({life:'open'});return d.text==='Unknown'&&d.label==='The engine has not supplied an execution state'&&!d.state&&!d.glyph;}));
  for(const mode of ['light','dark'])for(const motion of ['reduce','no-preference'])for(const skin of choices){
   await p.emulateMedia({reducedMotion:motion});await p.evaluate(mode=>pudlSetTheme(mode),mode);await choose(skin);
   await p.waitForFunction(id=>dinahAppearance.snapshot().state.skin===id,skin.id);
   ok('menu selection persists '+[skin.id,mode,motion].join('/'),await p.evaluate(id=>pudlSkins.current()===id&&document.getElementById('appearance-skin').value===id,skin.id));
   const panel=await skinMenu();ok('the menu checks the saved skin',await panel.getByRole('menuitemradio',{name:skin.label,exact:true}).getAttribute('aria-checked')==='true');await p.keyboard.press('Escape');await p.keyboard.press('Escape');
   for(const [surface,key] of [['ordinary',chat],['group',group]]){
    await p.evaluate(key=>{pudlWindows.raise(key);pudlWindows.snap(key,'maximized');},key);
    const win=p.locator('[data-win="'+key+'"]');
    ok(surface+' Unknown remains visible '+skin.id+'/'+mode+'/'+motion,await win.locator('.chat-state-unknown').first().textContent()==='Unknown');
    if(motion==='reduce')await t.shot('followup-'+skin.id+'-'+mode+'-'+surface);
   }
   records.push({skin:skin.id,mode,motion});
  }
  ok('skin switching preserves mounted windows and the draft',await p.evaluate(()=>__retainedPrompt.isConnected&&__retainedPrompt.value==='Retain this draft across menu skin changes.'&&__retainedWindows.every(el=>el.isConnected)));
  await p.evaluate(()=>{__saveFailure=true;});await choose(choices.find(s=>s.id==='bevel'));
  await p.locator('.toast').filter({hasText:'Appearance storage unavailable'}).last().waitFor();
  ok('a menu save failure preserves the previous preference and draft',await p.evaluate(last=>dinahAppearance.snapshot().state.skin===last&&pudlSkins.current()===last&&__retainedPrompt.value.includes('Retain this draft'),choices[choices.length-1].id));
  await p.evaluate(()=>{__saveFailure=false;__externalSkin('later-skin');});await p.waitForFunction(()=>dinahAppearance.snapshot().state.skin==='later-skin');
  await p.locator('#host-skins button').filter({hasText:'later-skin (unavailable)'}).waitFor({state:'attached'});const unavailable=await skinMenu();fs.writeFileSync(path.join(OUT,'unavailable-menu.html'),await unavailable.innerHTML());const row=unavailable.getByRole('menuitemradio',{name:/later-skin \(unavailable\)/});
  ok('an unknown saved skin is checked and disabled with Standard fallback',await row.isDisabled()&&await row.getAttribute('aria-checked')==='true'&&await p.evaluate(()=>pudlSkins.current()==='standard'&&document.getElementById('appearance-skin').value==='later-skin'));
  await p.keyboard.press('Escape');await p.keyboard.press('Escape');
  await p.evaluate(()=>__externalSkin('bevel'));await p.waitForFunction(()=>pudlSkins.current()==='bevel');
  await p.evaluate(()=>{document.querySelector('link[data-pudl-skin="bevel"]').disabled=true;});await p.waitForFunction(()=>pudlSkins.current()==='standard');
  const failed=await skinMenu();ok('failed styles keep the persisted checked choice',await failed.getByRole('menuitemradio',{name:'Bevel',exact:true}).getAttribute('aria-checked')==='true'&&await p.evaluate(()=>dinahAppearance.snapshot().state.skin==='bevel'&&!!dinahAppearance.snapshot().skinNotice));
  await p.keyboard.press('Escape');await p.keyboard.press('Escape');
 }finally{fs.writeFileSync(path.join(OUT,'appearance-followup.json'),JSON.stringify(records,null,2));await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
