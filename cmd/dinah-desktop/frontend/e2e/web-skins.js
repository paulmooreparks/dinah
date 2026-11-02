// Exercise the real server-rendered pages and embedded assets in an isolated home.
const {launch,OUT,FRONTEND}=require('./lib/page'),{spawn,execFileSync}=require('child_process'),readline=require('readline'),fs=require('fs'),path=require('path'),os=require('os');
(async()=>{
 const root=path.resolve(FRONTEND,'../../..'),bin=path.join(fs.mkdtempSync(path.join(os.tmpdir(),'dinah-web-skin-bin-')),'web-skin-host.exe');
 execFileSync('go',['build','-o',bin,'./internal/web/skinfixture'],{cwd:root,env:{...process.env,CGO_ENABLED:'0'},windowsHide:true});
 if(!fs.readFileSync(path.join(FRONTEND,'skin-assets.js')).equals(fs.readFileSync(path.join(root,'internal/web/static/skin-assets.js'))))throw Error('Host stylesheet trackers differ.');
 const host=spawn(bin,[],{cwd:root,windowsHide:true,stdio:['pipe','pipe','pipe']}),lines=readline.createInterface({input:host.stdout})[Symbol.asyncIterator]();let stderr='';host.stderr.on('data',b=>{stderr+=b;});
 const info=JSON.parse((await lines.next()).value),t=await launch({goto:false}),context=await t.context.browser().newContext({viewport:{width:1360,height:900}}),p=await context.newPage(),records=[],errors=[];
 const set=async(skin,mode)=>{host.stdin.write(JSON.stringify({skin,mode})+'\n');const line=await lines.next();if(line.done)throw Error('Isolated host stopped: '+stderr);return JSON.parse(line.value);};
 p.on('pageerror',e=>errors.push(String(e)));
 await context.addInitScript(()=>{localStorage.setItem('pudl-skin','gel');localStorage.setItem('pudl-theme','system');window.__webSkinFrame=null;function sample(){if(document.body&&window.pudlSkins){const b=document.createElement('button');b.className='btn';document.body.append(b);__webSkinFrame={skin:pudlSkins.current(),preference:pudlSkins.preference(),mode:document.documentElement.dataset.theme,radius:getComputedStyle(b).borderRadius};b.remove();}else requestAnimationFrame(sample);}requestAnimationFrame(sample);});
 try{
  await p.goto(info.signin);await p.waitForURL(info.url+'/');t.ok('the isolated host signs in through its real HttpOnly cookie',(await context.cookies()).some(c=>c.httpOnly));
  const skins=['standard','bevel','gel','glaze'];
  for(const skin of skins)for(const mode of ['light','dark'])for(const motion of ['reduce','no-preference']){
   await set(skin,mode);const pref=fs.readFileSync(path.join(info.home,'appearance.json'),'utf8'),ui=fs.readFileSync(path.join(info.home,'ui-state.json'),'utf8');await p.emulateMedia({reducedMotion:motion});
   for(const width of [1360,560]){
    await p.setViewportSize({width,height:900});
    for(const [label,url]of [['desktop','/'],['inbox','/inbox'],['groups','/rooms'],['conversation','/rooms/'+encodeURIComponent(info.room)]]){
     const response=await p.goto(info.url+url);await p.waitForFunction(()=>!!__webSkinFrame);const first=await p.evaluate(()=>__webSkinFrame);
     t.ok([skin,mode,motion,width,label].join('/')+' draws the server-owned preference',response.ok()&&first.skin===skin&&first.preference===skin&&first.mode===mode,JSON.stringify(first));
     records.push({skin,mode,motion,width,page:label,first});
     if(motion==='no-preference')await p.screenshot({path:path.join(OUT,['web',skin,mode,width,label].join('-')+'.png')});
    }
   }
   t.ok('web navigation preserves the appearance and mode files',fs.readFileSync(path.join(info.home,'appearance.json'),'utf8')===pref&&fs.readFileSync(path.join(info.home,'ui-state.json'),'utf8')===ui);
  }
  const signedOut=await t.context.browser().newContext(),outPage=await signedOut.newPage();
  try{for(const skin of skins)for(const mode of ['light','dark']){await set(skin,mode);await outPage.goto(info.url+'/inbox');await outPage.waitForFunction(()=>window.pudlSkins);t.ok('signed-out '+skin+'/'+mode+' retains the server appearance',await outPage.evaluate(({skin,mode})=>pudlSkins.current()===skin&&document.documentElement.dataset.theme===mode,{skin,mode}));}}finally{await signedOut.close();}
  await set('a-later-skin','light');await p.goto(info.url+'/inbox');await p.waitForFunction(()=>!!__webSkinFrame);
  t.ok('an unknown web skin renders Standard without rewriting its preference',await p.evaluate(()=>pudlSkins.current()==='standard')&&JSON.parse(fs.readFileSync(path.join(info.home,'appearance.json'),'utf8')).skin==='a-later-skin');
  await set('bevel','light');await context.route(info.url+'/static/pudl/pudl-skin-bevel.css',r=>r.fulfill({status:404,body:'',contentType:'text/css'}));await p.goto(info.url+'/inbox');await p.waitForFunction(()=>!!__webSkinFrame);
  const failed=await p.evaluate(()=>__webSkinFrame);t.ok('a failed web stylesheet reports Standard before the first drawn frame',failed.skin==='standard'&&failed.preference==='bevel',JSON.stringify(failed));
  t.ok('failed web assets keep their saved preference and explain the fallback',await p.locator('#skin-unavailable-notice').isVisible()&&JSON.parse(fs.readFileSync(path.join(info.home,'appearance.json'),'utf8')).skin==='bevel');
  // A remembered address is followed only when it is a query, so a saved
  // value that names another site leaves the page where it is (FE-07).
  await p.goto(info.url+'/');await p.evaluate(()=>localStorage.setItem('dinah.desktop','//example.invalid/x'));await p.goto(info.url+'/');await p.waitForTimeout(500);
  t.ok('a remembered address that is not a query is not followed',new URL(p.url()).origin===new URL(info.url).origin,p.url());
  t.ok('real server pages have no page errors',!errors.length,JSON.stringify(errors));
 }finally{
  fs.writeFileSync(path.join(OUT,'web-skin-records.json'),JSON.stringify({url:info.url,home:info.home,room:info.room,records,errors,stderr},null,2));await context.close();await t.close();const ended=new Promise(resolve=>host.once('exit',resolve));host.stdin.end(JSON.stringify({skin:'quit'})+'\n');await ended;
 }
})().catch(error=>{console.error(error);process.exitCode=1;});
