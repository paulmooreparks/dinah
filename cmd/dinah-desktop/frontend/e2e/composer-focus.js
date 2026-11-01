const {launch,OUT}=require('./lib/page');
const {choose}=require('./lib/commands');
const fs=require('fs'),path=require('path');
(async()=>{
 const choices=JSON.parse(fs.readFileSync(path.join(__dirname,'../pudl/pudl-skins.json'),'utf8')).skins.filter(s=>s.admitted).map(({id,label})=>({id,label}));
 const t=await launch({viewport:{width:1360,height:960},args:{choices},app:({choices})=>{
  let state={version:1,active:'mineral',skin:'standard',themes:[],overrides:{}},revision=0;
  const snapshot=()=>({state:JSON.parse(JSON.stringify(state)),revision:String(revision),skins:choices});
  return {ReadAppearance:async()=>snapshot(),SaveAppearance:async next=>{state=next;revision++;return snapshot();},
   OpenSessionAt:async(cwd,sessionId)=>({sessionId,cwd,provider:'codex',title:'Composer Focus',open:true}),
   SavedConversation:async()=>({available:false}),PickAttachments:async()=>[{kind:'file',name:'focus.txt',mimeType:'text/plain',data:btoa('Focus fixture'),size:13}]};
 }});
 const {page:p,ok}=t,records=[];
 const referenceRoot=process.env.DINAH_REFERENCE_FRONTEND;
 if(referenceRoot)await t.context.route('http://gkc.test/**',route=>route.fulfill({path:path.join(referenceRoot,new URL(route.request().url()).pathname),headers:{"Access-Control-Allow-Origin":"*"}}));
 async function reference(skin,mode){
  if(!referenceRoot)return null;
  return p.evaluate(async({skin,mode})=>{
   const frame=document.createElement('iframe');frame.style.cssText='position:fixed;left:-16000px;top:0;width:800px;height:350px';
   const loaded=new Promise(resolve=>frame.onload=resolve);
   frame.srcdoc='<!doctype html><html data-skin="'+skin+'" data-theme="'+mode+'"><head>'+['pudl/pudl.css','pudl/pudl-windows.css','pudl/pudl-skin-'+skin+'.css','brand.css','app.css'].filter(x=>!x.includes('skin-standard')).map(file=>'<link rel="stylesheet" href="http://gkc.test/'+file+'">').join('')+'</head><body><form class="composer"><div class="composer-row"><textarea class="form-textarea composer-input" rows="2">Matching focus</textarea></div></form></body></html>';
   document.body.append(frame);await loaded;
   const doc=frame.contentDocument;doc.documentElement.style.cssText=document.documentElement.style.cssText;await doc.fonts.ready;
   const el=doc.querySelector('textarea');el.focus();const css=frame.contentWindow.getComputedStyle(el);
   const result={boxShadow:css.boxShadow,borderRadius:css.borderRadius,outline:css.outline};frame.remove();return result;
  },{skin,mode});
 }
 try{
  await p.evaluate(()=>dinahAppearance.ready);
  const key=await p.evaluate(()=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'codex:focus'}));
  const win=p.locator('[data-win="'+key+'"]'),prompt=win.locator('[data-view-id="prompt"]');
  await p.evaluate(key=>pudlWindows.snap(key,'maximized'),key);await prompt.fill('Keep this draft and its keyboard focus.');
  for(const skin of ['gel'])for(const mode of ['light','dark'])for(const attached of [false,true]){
   await p.evaluate(({skin,mode})=>{pudlSetTheme(mode);return dinahAppearance.selectSkin(skin);},{skin,mode});
   if(attached){await choose(p,win,'add','Files From This Computer…');await win.getByText('focus.txt',{exact:true}).waitFor();}
   if(!records.length){
    await prompt.focus();
    const baseline=await prompt.evaluate(el=>{const scroll=el.closest('.composer-scroll');scroll.style.padding='0';scroll.style.margin='0';const a=el.getBoundingClientRect(),b=scroll.getBoundingClientRect();return {bottom:b.bottom-a.bottom,left:a.left-b.left};});
    ok('the original scroll edge clips the outer focus ring',baseline.bottom<3||baseline.left<3,JSON.stringify(baseline));
    await t.shot('composer-focus-original-clip');
    await prompt.evaluate(el=>{const scroll=el.closest('.composer-scroll');scroll.style.removeProperty('padding');scroll.style.removeProperty('margin');});
   }
   const referenceStyle=await reference(skin,mode);
   const before=await prompt.evaluate(el=>({input:el.getBoundingClientRect().toJSON(),composer:el.closest('.composer').getBoundingClientRect().toJSON()}));
   await prompt.focus();
   const result=await prompt.evaluate(el=>{
    const r=el.getBoundingClientRect(),css=getComputedStyle(el),ring=parseFloat(css.getPropertyValue('--focus-ring-width'))||3,clips=[];
    for(let a=el.parentElement;a;a=a.parentElement){const style=getComputedStyle(a),b=a.getBoundingClientRect();
     const clearance={left:r.left-b.left-a.clientLeft,right:b.left+a.clientLeft+a.clientWidth-r.right,top:r.top-b.top-a.clientTop,bottom:b.top+a.clientTop+a.clientHeight-r.bottom};
     if((/auto|scroll|hidden|clip/.test(style.overflowX)&&(clearance.left<ring-.1||clearance.right<ring-.1))||(/auto|scroll|hidden|clip/.test(style.overflowY)&&(clearance.top<ring-.1||clearance.bottom<ring-.1)))clips.push({className:a.className,clearance});
    }
    return {input:r.toJSON(),composer:el.closest('.composer').getBoundingClientRect().toJSON(),clips,ring,style:{boxShadow:css.boxShadow,borderRadius:css.borderRadius,outline:css.outline},resize:css.resize,focused:document.activeElement===el,value:el.value};
   });
   records.push({skin,mode,attached,referenceStyle,...result});
   if(referenceStyle)ok(skin+'/'+mode+' matches the reference application focus styling at the same scale',JSON.stringify(referenceStyle)===JSON.stringify(result.style),JSON.stringify({referenceStyle,actual:result.style}));
   ok(skin+'/'+mode+'/'+attached+' shows every edge of the focus ring',!result.clips.length,JSON.stringify(result.clips));
   ok(skin+'/'+mode+'/'+attached+' focus preserves composer geometry and draft',JSON.stringify(before)===JSON.stringify({input:result.input,composer:result.composer})&&result.focused&&result.value==='Keep this draft and its keyboard focus.');
   await t.shot('composer-focus-'+skin+'-'+mode+'-'+(attached?'attached':'empty'));
   if(attached)await win.getByRole('button',{name:'Remove focus.txt',exact:true}).click();
  }
  const before=await prompt.evaluate(el=>el.getBoundingClientRect().height);await prompt.fill(Array(12).fill('The textarea still grows with content.').join('\n'));
  ok('the composer retains content-driven resizing',await prompt.evaluate(el=>el.getBoundingClientRect().height)>before);
  await p.keyboard.press('End');await p.keyboard.type(' Continued.');
  ok('keyboard editing still works',await prompt.inputValue().then(value=>value.includes(' Continued.')));
 }finally{fs.mkdirSync(OUT,{recursive:true});fs.writeFileSync(path.join(OUT,'composer-focus.json'),JSON.stringify(records,null,2));await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
