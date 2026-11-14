const fs=require('fs'),path=require('path'),png=require('./png');
const {OUT}=require('./page');
const skins=require('../../pudl/pudl-skins.json').skins.filter(s=>s.admitted).map(s=>s.id);
const lum=c=>c.slice(0,3).reduce((s,n,i)=>s+(n/255<=.04045?n/255/12.92:((n/255+.055)/1.055)**2.4)*[.2126,.7152,.0722][i],0);
const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05);};
async function audit(p,selector){
 const nodes=await p.locator(selector).evaluate(root=>{
  const canvas=document.createElement('canvas'),ctx=canvas.getContext('2d');
  function color(c){ctx.clearRect(0,0,1,1);ctx.fillStyle=c;ctx.fillRect(0,0,1,1);return [...ctx.getImageData(0,0,1,1).data];}
  function visible(el){return el.checkVisibility({checkOpacity:true,checkVisibilityCSS:true})&&!el.closest('[hidden],:disabled,[aria-disabled="true"]');}
  function clipped(el,r){let a={x:Math.max(0,r.x),y:Math.max(0,r.y),right:Math.min(innerWidth,r.right),bottom:Math.min(innerHeight,r.bottom)};for(let at=el;at;at=at.parentElement){const c=getComputedStyle(at),b=at.getBoundingClientRect();if(/hidden|clip|auto|scroll/.test(c.overflowX)){a.x=Math.max(a.x,b.x);a.right=Math.min(a.right,b.right);}if(/hidden|clip|auto|scroll/.test(c.overflowY)){a.y=Math.max(a.y,b.y);a.bottom=Math.min(a.bottom,b.bottom);}}return a;}
  const result=[],walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);let text;
  while((text=walker.nextNode())){const el=text.parentElement;if(!text.textContent.trim()||!visible(el))continue;const css=getComputedStyle(el),r=document.createRange();r.selectNodeContents(text);for(const box of r.getClientRects()){const b=clipped(el,box);if(b.right-b.x<2||b.bottom-b.y<2)continue;result.push({...b,text:text.textContent.trim().slice(0,90),fg:color(css.color),threshold:parseFloat(css.fontSize)>=24||parseFloat(css.fontSize)>=18.66&&+css.fontWeight>=700?3:4.5});}}
  for(const el of root.querySelectorAll('input:not([type=checkbox]):not([type=radio]),textarea')){if(!visible(el))continue;const css=getComputedStyle(el),b=clipped(el,el.getBoundingClientRect());result.push({...b,text:el.value||el.placeholder,fg:color(el.value?css.color:getComputedStyle(el,'::placeholder').color),threshold:4.5});}
  return result;
 });
 const capture=await p.screenshot(),actual=png(capture);
 const hiding=await p.addStyleTag({content:selector+' *, '+selector+' { -webkit-text-fill-color:transparent!important;text-shadow:none!important; }'});
 const blank=png(await p.screenshot());await hiding.evaluate(el=>el.remove());
 const bad=[],unmeasured=[];let measured=0,minimum=Infinity;
 for(const n of nodes){let samples=0,min=Infinity;for(let y=Math.max(0,Math.ceil(n.y));y<Math.min(actual.height,Math.floor(n.bottom));y++)for(let x=Math.max(0,Math.ceil(n.x));x<Math.min(actual.width,Math.floor(n.right));x++){
  const a=actual.pixel(x,y),b=blank.pixel(x,y),fg=n.fg.slice(0,3).map((v,i)=>v*n.fg[3]/255+b[i]*(1-n.fg[3]/255));const d=fg.map((v,i)=>v-b[i]),delta=a.map((v,i)=>v-b[i]),norm=d.reduce((s,v)=>s+v*v,0);if(norm<100)continue;const coverage=d.reduce((s,v,i)=>s+v*delta[i],0)/norm,error=Math.sqrt(d.reduce((s,v,i)=>s+(delta[i]-coverage*v)**2,0));if(coverage>=.9&&coverage<=1.05&&error<8){samples++;min=Math.min(min,ratio(fg,b));}}
  if(samples){measured++;minimum=Math.min(minimum,min);if(min<n.threshold)bad.push({text:n.text,ratio:min,required:n.threshold});}else unmeasured.push(n.text);
 }
 return {measured,minimum,bad,unmeasured,capture};
}
async function focus(p,selector,capture){
 const choices=p.locator(selector).locator('button:not(:disabled):not([aria-disabled=true]),a[href],textarea:not(:disabled),input:not(:disabled)');let target;
 for(let i=0;i<await choices.count();i++){const el=choices.nth(i),b=await el.boundingBox();if(b&&await el.evaluate(e=>{const r=e.getBoundingClientRect(),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return hit===e||e.contains(hit);})&&b.x>=4&&b.y>=4&&b.x+b.width<=p.viewportSize().width-4&&b.y+b.height<=p.viewportSize().height-4){target=el;break;}}
 if(!target)return {skipped:'No fully visible control in this slice.',capture};
 const before=png(capture);await p.keyboard.press('Tab');await target.focus();
 const state=await target.evaluate(el=>{const c=getComputedStyle(el);return {visible:el.matches(':focus-visible'),outline:c.outline,shadow:c.boxShadow};});const box=await target.boundingBox(),focused=await p.screenshot(),after=png(focused);let pixels=0,maximum=1;
 for(let y=Math.max(0,Math.floor(box.y-5));y<Math.min(after.height,Math.ceil(box.y+box.height+5));y++)for(let x=Math.max(0,Math.floor(box.x-5));x<Math.min(after.width,Math.ceil(box.x+box.width+5));x++){if(x>box.x+3&&x<box.x+box.width-3&&y>box.y+3&&y<box.y+box.height-3)continue;const r=ratio(before.pixel(x,y),after.pixel(x,y));maximum=Math.max(maximum,r);if(r>=3)pixels++;}
 return {...state,pixels,maximum,capture:focused};
}
async function matrix(t,label,selector,prepare=async()=>{}){
 const p=t.page;await p.emulateMedia({reducedMotion:'reduce'});const results=[];
 for(const width of [1440,390])for(const mode of ['light','dark'])for(const skin of process.env.DINAH_TODAY_PILOT ? skins.slice(0,1) : skins){
  await p.setViewportSize({width,height:width===390?844:1000});await p.evaluate(({mode,skin})=>{pudlSetTheme(mode);pudlSkins.select(skin);},{mode,skin});
  await prepare();await p.evaluate(()=>document.fonts.ready);await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));await p.mouse.move(0,0);await p.evaluate(()=>{if(document.activeElement instanceof HTMLElement)document.activeElement.blur();});
  const name=[label,skin,mode,width].join('-'),text=await audit(p,selector),ring=await focus(p,selector,text.capture);const screenshot=ring.capture;delete text.capture;delete ring.capture;
  const geometry=await p.locator(selector).evaluate(root=>{const controls=[...root.querySelectorAll('button,input,select,textarea')].filter(el=>el.checkVisibility()&&el.getBoundingClientRect().width);const clipped=controls.filter(el=>el.tagName==='BUTTON'&&el.scrollWidth>el.clientWidth+2&&getComputedStyle(el).overflowX!=='auto').map(el=>el.textContent.trim().slice(0,60));const overlap=[];for(let i=0;i<controls.length;i++)for(let j=i+1;j<controls.length;j++){const a=controls[i],b=controls[j];if(a.contains(b)||b.contains(a))continue;const x=a.getBoundingClientRect(),y=b.getBoundingClientRect();if(Math.min(x.right,y.right)-Math.max(x.x,y.x)>2&&Math.min(x.bottom,y.bottom)-Math.max(x.y,y.y)>2)overlap.push([a.id||a.textContent.trim().slice(0,30),b.id||b.textContent.trim().slice(0,30)]);}return {clipped,overlap};});
  const symbols=await p.locator(selector).evaluate(root=>[...root.querySelectorAll('.updates-status,#set-aside-status,.decisions-receipt,.decisions-status')].filter(el=>el.checkVisibility()).map(el=>{const c=getComputedStyle(el,'::before');return {id:el.id||el.className,mask:c.maskImage,color:c.backgroundColor};}));
  t.ok(name+' status symbols use a glyph and colour',symbols.every(s=>s.mask!=='none'&&s.color!=='rgba(0, 0, 0, 0)'),JSON.stringify(symbols));
  const record={name,...text,focus:ring,geometry,symbols};results.push(record);
  t.ok(name+' text meets PUDL contrast',text.measured>0&&!text.bad.length,JSON.stringify(text.bad));
  t.ok(name+' focus is visible',ring.skipped||ring.visible&&ring.pixels>=10,JSON.stringify(ring));
  t.ok(name+' controls fit',!geometry.clipped.length&&!geometry.overlap.length,JSON.stringify(geometry));
  fs.writeFileSync(path.join(OUT,name+'.png'),screenshot);
 }
 fs.writeFileSync(path.join(OUT,label+'-measurements.json'),JSON.stringify(results,null,2));return results;
}
module.exports={matrix,skins};