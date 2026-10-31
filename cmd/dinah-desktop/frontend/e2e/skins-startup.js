// Static skin sheets and the host cache must agree on the first drawn frame.
const {launch,OUT}=require('./lib/page');const fs=require('fs'),path=require('path');
(async()=>{const evidence=[];
 for(const skin of ['standard','bevel','gel','glaze','unknown-skin','broken-bevel'])for(const mode of ['light','dark']){
  const desired=skin==='broken-bevel'?'bevel':skin;
  const t=await launch({goto:false,args:{skin:desired,mode},app:({skin,mode})=>{window.__readAppearanceGate=new Promise(resolve=>{window.__releaseAppearance=resolve;});return {ReadUIState:async()=>{await __readAppearanceGate;return {version:1,values:{'pudl-theme':mode}};},ReadAppearance:async()=>{await __readAppearanceGate;return {state:{version:1,active:'mineral',skin,themes:[],overrides:{}},revision:'0',skins:[{id:'standard',label:'Standard'},{id:'bevel',label:'Bevel'},{id:'gel',label:'Gel'},{id:'glaze',label:'Glaze'}]};}};},init:({skin,mode})=>{
   if(window!==top)return;localStorage.setItem('dinah-appearance-startup-v1',JSON.stringify({skin,mode}));localStorage.setItem('pudl-skin','glaze');localStorage.setItem('pudl-theme',mode==='light'?'dark':'light');
   window.__firstSkinFrame=null;function sample(){if(document.body&&window.pudlSkins){const probe=document.createElement('button');probe.className='btn';document.body.append(probe);__firstSkinFrame={skin:document.documentElement.dataset.skin,preference:pudlSkins.preference(),mode:document.documentElement.dataset.theme,radius:getComputedStyle(probe).borderRadius};probe.remove();}else requestAnimationFrame(sample);}requestAnimationFrame(sample);
  }});
  try{
   if(skin==='broken-bevel')await t.context.route('http://dinah.test/pudl/pudl-skin-bevel.css',route=>route.fulfill({status:404,body:''}));
   await t.page.goto('http://dinah.test/index.html');await t.page.waitForFunction(()=>!!__firstSkinFrame);
   const expected=['unknown-skin','broken-bevel'].includes(skin)?'standard':skin;
   const first=await t.page.evaluate(()=>__firstSkinFrame);
   t.ok(skin+'/'+mode+' first drawn frame uses the cached available skin and mode',first.skin===expected&&first.preference===desired&&first.mode===mode,JSON.stringify(first));
   const stable=await t.page.evaluate(()=>{const b=document.createElement('button');b.className='btn';document.body.append(b);const value=getComputedStyle(b).borderRadius;b.remove();return value;});
   t.ok(skin+'/'+mode+' has no later shape correction',first.radius===stable);
   await t.page.evaluate(()=>__releaseAppearance());await t.page.evaluate(()=>dinahAppearance.ready);
   if(expected==='standard'&&desired!=='standard')t.ok(skin+' keeps its unavailable preference with a notice',await t.page.evaluate(desired=>dinahAppearance.snapshot().state.skin===desired&&dinahAppearance.snapshot().skinNotice.length>0,desired));
   await t.page.evaluate(()=>{dispatchEvent(new StorageEvent('storage',{key:'pudl-skin',newValue:'gel'}));dispatchEvent(new StorageEvent('storage',{key:'pudl-theme',newValue:'system'}));});
   t.ok('PUDL browser keys cannot override host-owned preferences',await t.page.evaluate(({skin,mode})=>pudlSkins.preference()===skin&&pudlThemePreference()===mode,{skin:desired,mode}));
   evidence.push({skin,mode,first,stable});
  }finally{await t.close();}
 }
 fs.writeFileSync(path.join(OUT,'skin-first-frames.json'),JSON.stringify(evidence,null,2));
})().catch(error=>{console.error(error);process.exitCode=1;});
