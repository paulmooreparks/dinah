// Native catalogue/preferences drive all skin changes; mounted work survives.
const {launch,OUT,FRONTEND}=require('./lib/page');
const fs=require('fs'),path=require('path'),crypto=require('crypto');
const catalogue=JSON.parse(fs.readFileSync(path.join(FRONTEND,'pudl/pudl-skins.json'),'utf8'));
const admitted=catalogue.skins.filter(s=>s.admitted),choices=admitted.map(({id,label})=>({id,label}));
(async()=>{
 const t=await launch({viewport:{width:1600,height:1000},args:{choices},app:({choices})=>{
  let state={version:1,active:'mineral',skin:'standard',themes:[],overrides:{}},revision=0;
  window.__saveSkinCalls=[];window.__failSkinSave=false;window.__failSkinRead=false;
  const snapshot=()=>({state:JSON.parse(JSON.stringify(state)),revision:String(revision),skins:choices,effective:choices.some(s=>s.id===state.skin)?state.skin:'standard'});
  window.__externalSkin=skin=>{state.skin=skin;revision++;__emit('appearance:changed',{revision:String(revision),skin});};
  window.__invalidStoredCustom=()=>{const document=JSON.parse(JSON.stringify(dinahThemeBuiltins.solarized));document.name='Existing custom';document.palettes.dark['on-accent']='#002b36';state.active='existing-custom';state.themes=[{id:'existing-custom',document}];revision++;__emit('appearance:changed',{revision:String(revision),skin:state.skin});};
  const room={name:'skin-room',title:'Skin Group',members:[{id:'paul',title:'Paul',state:'person'}],counts:{top:'idle',idle:0}};
  return {ReadAppearance:async()=>{if(__failSkinRead)throw Error('Appearance read unavailable');return snapshot();},SaveAppearance:async(next,expected)=>{if(__failSkinSave)throw Error('Skin storage unavailable');if(expected!==String(revision))throw Error('Revision conflict');__saveSkinCalls.push(next.skin);state=next;revision++;return snapshot();},
   SavedConversation:async()=>({available:false}),OpenSessionAt:async(cwd,sessionId)=>({sessionId,cwd,provider:'codex',title:'Skin Chat',open:true}),
   BusOverview:async()=>({inbox:[],rooms:[room],workspaces:[],links:[]}),BusResolveGroup:async()=>({state:'resolved',room}),BusGroupMessages:async()=>[],BusMarkGroupRead:async()=>{}};
 }});
 const {page:p,ok}=t,requests=[];t.context.on('request',r=>requests.push(r.url()));
 try{
  await p.evaluate(()=>dinahAppearance.ready);
  ok('the admitted catalogue is bundled with its verified stylesheet bytes',admitted.every(s=>!s.stylesheet||crypto.createHash('sha256').update(fs.readFileSync(path.join(FRONTEND,'pudl',s.stylesheet))).digest('hex')===s.sha256));
  ok('static declared skin links match the admitted catalogue',JSON.stringify(await p.locator('link[data-pudl-skin]').evaluateAll(els=>els.map(e=>e.dataset.pudlSkin).sort()))===JSON.stringify(admitted.filter(s=>s.stylesheet).map(s=>s.id).sort()));
  const chat=await p.evaluate(()=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'codex:skins'}));
  const prompt=p.locator('[data-win="'+chat+'"] [data-view-id=prompt]');await prompt.fill('Keep the skin-switch draft.');
  await p.evaluate(()=>dinahWorkbench.open('settings'));await p.evaluate(()=>pudlWindows.snap('settings','maximized'));
  ok('Settings offers catalogue labels without a private skin list',JSON.stringify(await p.locator('#appearance-skin option').allTextContents())===JSON.stringify(choices.map(s=>s.label)));
  await p.evaluate(()=>{window.__skinNodes=[...document.querySelectorAll('.dinah-applet')];});
  await prompt.evaluate(el=>{window.__skinPrompt=el;});
  const snapshots=[];
  for(const motion of ['reduce','no-preference'])for(const mode of ['light','dark']){
   await p.emulateMedia({reducedMotion:motion});await p.evaluate(mode=>pudlSetTheme(mode),mode);
   for(const skin of choices){
    await p.locator('#appearance-skin').selectOption(skin.id);await p.waitForFunction(id=>dinahAppearance.snapshot().state.skin===id&&pudlSkins.current()===id,skin.id);
    ok(skin.id+'/'+mode+'/'+motion+' is effective',await p.evaluate(id=>document.documentElement.dataset.skin===id,skin.id));
    snapshots.push({skin:skin.id,mode,motion,paint:await p.locator('#appearance-edit').evaluate(el=>({radius:getComputedStyle(el).borderRadius,background:getComputedStyle(el).backgroundImage}))});
   }
  }
  await p.evaluate(key=>pudlWindows.raise(key),chat);await prompt.focus();await prompt.evaluate(el=>el.setSelectionRange(2,9));
  const before=await p.evaluate(()=>({url:location.href,rectangles:[...document.querySelectorAll('.win[data-win]')].map(el=>[el.dataset.win,el.getBoundingClientRect().toJSON()])})),count=requests.length;
  await p.evaluate(()=>dinahAppearance.selectSkin('gel'));
  const preserved=await p.evaluate(()=>({nodes:__skinNodes.every(el=>el.isConnected),prompt:__skinPrompt.isConnected&&__skinPrompt.value==='Keep the skin-switch draft.',focus:document.activeElement===__skinPrompt,selection:[__skinPrompt.selectionStart,__skinPrompt.selectionEnd],url:location.href,rectangles:[...document.querySelectorAll('.win[data-win]')].map(el=>[el.dataset.win,el.getBoundingClientRect().toJSON()])}));
  ok('skin switching preserves mounted applets, draft, focus, selection and URL',preserved.nodes&&preserved.prompt&&preserved.focus&&preserved.selection.join(',')==='2,9'&&preserved.url===before.url,JSON.stringify(preserved));
  ok('skin switching preserves window positions and dimensions',JSON.stringify(preserved.rectangles)===JSON.stringify(before.rectangles));
  ok('skin switching makes no asset/navigation requests',requests.length===count,JSON.stringify(requests.slice(count)));
  await p.evaluate(()=>{__failSkinSave=true;});let failed=false;try{await p.evaluate(()=>dinahAppearance.selectSkin('bevel'));}catch(_){failed=true;}
  ok('a failed skin save retains the prior preference and rendered skin',failed&&await p.evaluate(()=>dinahAppearance.snapshot().state.skin==='gel'&&pudlSkins.current()==='gel'));
  await p.evaluate(()=>{__failSkinSave=false;__externalSkin('unavailable-skin');});await p.waitForFunction(()=>dinahAppearance.snapshot().state.skin==='unavailable-skin');
  ok('a missing skin remains selected while Standard and an explanation are shown',await p.evaluate(()=>pudlSkins.current()==='standard'&&dinahAppearance.snapshot().skinNotice.includes('choice is kept')));
  await p.evaluate(()=>__externalSkin('bevel'));await p.waitForFunction(()=>pudlSkins.current()==='bevel');
  ok('an appearance event reloads the authoritative preference',await p.evaluate(()=>dinahAppearance.snapshot().state.skin==='bevel'));
  await p.evaluate(()=>{__failSkinRead=true;});let readFailed=false;try{await p.evaluate(()=>dinahAppearance.reload());}catch(_){readFailed=true;}
  ok('failed preference refresh preserves the prior skin and mounted draft',readFailed&&await p.evaluate(()=>dinahAppearance.snapshot().state.skin==='bevel'&&pudlSkins.current()==='bevel'&&__skinPrompt.isConnected&&__skinPrompt.value==='Keep the skin-switch draft.'));
  await p.evaluate(()=>{__failSkinRead=false;return dinahAppearance.reload();});
  await p.evaluate(()=>{document.querySelector('link[data-pudl-skin="bevel"]').disabled=true;});
  await p.waitForFunction(()=>pudlSkins.current()==='standard'&&!!dinahAppearance.snapshot().skinNotice);
  ok('disabling the selected stylesheet preserves its preference with visible fallback',await p.evaluate(()=>dinahAppearance.snapshot().state.skin==='bevel'&&document.getElementById('appearance-skin-status').textContent.includes('choice is kept')));
  await p.evaluate(()=>{document.querySelector('link[data-pudl-skin="bevel"]').disabled=false;});await p.waitForFunction(()=>pudlSkins.current()==='bevel'&&!dinahAppearance.snapshot().skinNotice);
  await p.evaluate(()=>{const link=document.querySelector('link[data-pudl-skin="bevel"]');link.sheet.disabled=true;pudlSkins.refresh();});
  await p.waitForFunction(()=>pudlSkins.current()==='standard'&&!!dinahAppearance.snapshot().skinNotice);
  await p.evaluate(()=>{const link=document.querySelector('link[data-pudl-skin="bevel"]');link.sheet.disabled=false;pudlSkins.refresh();});await p.waitForFunction(()=>pudlSkins.current()==='bevel');
  await p.evaluate(()=>dinahWorkbench.open('theme-editor'));
  await p.locator('iframe[title="light theme preview"]').waitFor();await p.waitForFunction(()=>[...document.querySelectorAll('.theme-previews iframe')].every(f=>f.contentDocument?.documentElement.dataset.skin==='bevel'));
  const frames=await p.locator('.theme-previews iframe').elementHandles();
  const fontInput=p.locator('#appearance-fonts input').first();await fontInput.fill('Georgia');await fontInput.dispatchEvent('change');
  await fontInput.evaluate(el=>{window.__skinFontInput=el;});
  await p.evaluate(()=>dinahAppearance.selectSkin('glaze'));await p.waitForFunction(()=>[...document.querySelectorAll('.theme-previews iframe')].every(f=>f.contentDocument?.documentElement.dataset.skin==='glaze'));
  ok('skin selection preserves unsaved font fields',await p.evaluate(()=>__skinFontInput.isConnected&&__skinFontInput.value==='Georgia'));
  ok('open theme previews follow the skin without replacing their frames',await Promise.all(frames.map(f=>f.evaluate(el=>el.isConnected))).then(values=>values.every(Boolean)));
  const contrast=await t.deadline('all built-in/custom skin contrast checks',p.evaluate(async()=>{const result={},records={};for(const [id,theme]of Object.entries(dinahAppearance.builtins)){records[id]=[];result[id]=await dinahSkinContrast.check(theme,records[id]);}const custom=dinahAppearance.document(dinahAppearance.snapshot().state.active);custom.name='Custom skin audit';records.custom=[];result.custom=await dinahSkinContrast.check(custom,records.custom);window.__skinPaintRecords=records;return result;}),60000);
  ok('all built-in themes and a custom theme pass every admitted skin',Object.values(contrast).every(v=>!v.length),JSON.stringify(contrast));
  fs.writeFileSync(path.join(OUT,'skin-contrast.json'),JSON.stringify(contrast,null,2));
  fs.writeFileSync(path.join(OUT,'skin-paint-records.json'),JSON.stringify(await p.evaluate(()=>__skinPaintRecords),null,2));
  for(const skin of choices)for(const mode of ['light','dark'])for(const width of [1600,560]){
   await p.setViewportSize({width,height:1000});await p.evaluate(mode=>pudlSetTheme(mode),mode);await p.evaluate(id=>dinahAppearance.selectSkin(id),skin.id);
   for(const surface of ['main','chat','group','settings','theme-editor']){
    if(surface==='chat')await p.evaluate(key=>pudlWindows.raise(key),chat);
    if(surface==='group')await p.evaluate(()=>dinahWorkbench.open({kind:'bus-room',path:'skin-room'}));
    if(surface==='settings'||surface==='theme-editor')await p.evaluate(kind=>dinahWorkbench.open(kind),surface);
    await t.shot('skin-'+skin.id+'-'+mode+'-'+width+'-'+surface);
   }
  }
  fs.writeFileSync(path.join(OUT,'skin-switching.json'),JSON.stringify(snapshots,null,2));
  await p.evaluate(()=>__invalidStoredCustom());await p.waitForFunction(()=>dinahAppearance.snapshot().state.active==='existing-custom'&&dinahAppearance.snapshot().skinContrastNotice.includes('4.5:1'));
  const saves=await p.evaluate(()=>__saveSkinCalls.length);let contrastFailed=false;try{await p.evaluate(()=>dinahAppearance.selectSkin('standard'));}catch(_){contrastFailed=true;}
  ok('a skin-invalid stored custom theme is explained and remains unchanged on failure',contrastFailed&&await p.evaluate(saves=>__saveSkinCalls.length===saves&&dinahAppearance.snapshot().state.skin==='glaze'&&dinahAppearance.document('existing-custom').palettes.dark['on-accent']==='#002b36'&&__skinPrompt.value==='Keep the skin-switch draft.',saves));
  await p.evaluate(()=>{const document=dinahAppearance.document('existing-custom');document.palettes.dark['on-accent']=dinahAppearance.builtins.solarized.palettes.dark['on-accent'];return dinahAppearance.save(document,'existing-custom',true);});
  ok('the user can explicitly correct the stored custom theme',await p.evaluate(()=>!dinahAppearance.snapshot().skinContrastNotice));
 }finally{await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
