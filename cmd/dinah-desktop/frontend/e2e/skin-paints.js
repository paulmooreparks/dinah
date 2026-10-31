// Compare actual interactive glyph pixels with the same control without its label.
const {launch,OUT}=require('./lib/page'),png=require('./lib/png'),fs=require('fs'),path=require('path');
const luminance=c=>c.slice(0,3).reduce((sum,n,i)=>sum+(n/255<=.04045?n/255/12.92:((n/255+.055)/1.055)**2.4)*[.2126,.7152,.0722][i],0);
const ratio=(a,b)=>{const x=luminance(a),y=luminance(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05);};
function contrast(text,background,foreground){
 const a=png(text),b=png(background);if(a.width!==b.width||a.height!==b.height)throw Error('Control geometry changed between captures');
 let minimum=Infinity,samples=0;
 for(let y=0;y<a.height;y++)for(let x=0;x<a.width;x++){
  const visible=a.pixel(x,y),base=b.pixel(x,y),fg=foreground.slice(0,3).map((c,i)=>c*foreground[3]/255+base[i]*(1-foreground[3]/255));
  const d=fg.map((c,i)=>c-base[i]),delta=visible.map((c,i)=>c-base[i]),norm=d.reduce((s,c)=>s+c*c,0);if(norm<100)continue;
  const coverage=d.reduce((s,c,i)=>s+c*delta[i],0)/norm,error=Math.sqrt(d.reduce((s,c,i)=>s+(delta[i]-coverage*c)**2,0));
  if(coverage>=.9&&coverage<=1.05&&error<=8){samples++;minimum=Math.min(minimum,ratio(fg,base));}
 }
 return {minimum,samples};
}
(async()=>{
 const t=await launch({viewport:{width:1100,height:850}}),p=t.page,records=[];
 try{
  await p.evaluate(()=>dinahAppearance.ready);
  await p.evaluate(()=>{const style=document.createElement('style');style.textContent='#skin-render-probe{position:fixed;z-index:2147483647;left:100px;top:100px;width:500px;padding:32px;background:var(--surface)}#skin-render-probe *{transition:none!important;animation:none!important}#skin-render-probe .win{position:relative;width:400px;min-height:120px}';document.head.append(style);const host=document.createElement('div');host.id='skin-render-probe';document.body.append(host);});
  const themes=await p.evaluate(()=>Object.keys(dinahAppearance.builtins)),skins=await p.evaluate(()=>pudlSkins.available());
  for(const theme of themes)for(const mode of ['light','dark'])for(const skin of skins)for(const kind of ['button','primary','danger','title'])for(const state of kind==='title'?['rest']:['rest','hover','pressed','focus']){
   const name=[theme,mode,skin,kind,state].join('-');await p.mouse.move(10,10);
   await p.evaluate(async({theme,mode,skin,kind})=>{
    pudlSetTheme(mode);await new Promise(requestAnimationFrame);pudlSkins.select(skin);
    Object.entries(dinahAppearance.builtins[theme].palettes[mode]).forEach(([k,v])=>document.documentElement.style.setProperty('--'+k,v));
    const host=document.getElementById('skin-render-probe');host.replaceChildren();const win=document.createElement('section');win.className='win active';host.append(win);
    const c=document.createElement(kind==='title'?'header':'button');c.id='skin-render-control';c.className=kind==='title'?'win-head':kind==='primary'?'btn btn-primary':kind==='danger'?'btn btn-danger':'btn';
    const label=document.createElement('span');label.id='skin-render-label';label.className=kind==='title'?'win-title':'';label.textContent=kind==='title'?'Sample active window':'Sample';c.append(label);win.append(c);
   },{theme,mode,skin,kind});
   const control=p.locator('#skin-render-control');if(state==='hover'||state==='pressed')await control.hover();if(state==='pressed')await p.mouse.down();
   if(state==='focus'){await control.focus();await p.keyboard.press('Tab');await p.keyboard.press('Shift+Tab');}
   const data=await control.evaluate(el=>{const css=getComputedStyle(el),ctx=document.createElement('canvas').getContext('2d');ctx.fillStyle=css.color;ctx.fillRect(0,0,1,1);return {rgba:[...ctx.getImageData(0,0,1,1).data],image:css.backgroundImage,color:css.color,hover:el.matches(':hover'),pressed:el.matches(':active'),focus:el.matches(':focus-visible')};});
   const text=await control.screenshot();await p.locator('#skin-render-label').evaluate(el=>el.style.visibility='hidden');const background=await control.screenshot();if(state==='pressed')await p.mouse.up();
   const measured=contrast(text,background,data.rgba),record={name,theme,mode,skin,kind,state,...data,...measured};records.push(record);
   const bad=!measured.samples||measured.minimum<4.5;
   if(bad||theme==='mineral'){fs.writeFileSync(path.join(OUT,name+'-text.png'),text);fs.writeFileSync(path.join(OUT,name+'-background.png'),background);}
   t.ok(name+' has actual glyph contrast',!bad,JSON.stringify(record));
   t.ok(name+' uses the requested interaction state',state==='rest'||state==='hover'&&data.hover||state==='pressed'&&data.pressed||state==='focus'&&data.focus);
  }
 }finally{fs.writeFileSync(path.join(OUT,'skin-rendered-contrast.json'),JSON.stringify(records,null,2));await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
