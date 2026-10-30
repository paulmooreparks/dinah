const {chromium}=require('playwright-core');
const fs=require('fs'),path=require('path');
(async()=>{
 const root=process.argv[2], entry=fs.readFileSync(path.join(root,'entry.txt'),'utf8');
 const histories=fs.readdirSync(path.join(root,'conversations')).filter(n=>n.endsWith('.json')).map(n=>{try{return JSON.parse(fs.readFileSync(path.join(root,'conversations',n),'utf8'))}catch{return null}}).filter(t=>t&&!t.deleted&&t.sessionId&&t.workspace&&t.messages?.length>200).sort((a,b)=>b.messages.length-a.messages.length).slice(0,4).map(t=>({id:t.sessionId,cwd:t.workspace,count:t.messages.length}));
 if(histories.length!==4)throw Error('Four saved histories required');
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});
 try{
 const auth=path.join(root,'browser-auth.json'); const context=await browser.newContext({viewport:{width:1920,height:1080},...(fs.existsSync(auth)?{storageState:auth}:{})});
 await context.route('**/bridge/app.js',async route=>{const r=await route.fetch();await route.fulfill({response:r,body:(await r.text())+`\n(()=>{const a=window.go.main.App;a.OpenSessionAt=()=>new Promise(()=>{});a.ResolveSessionContinuation=async id=>id;a.Sessions=async()=>[];a.History=async()=>[];a.ReadUIState=async()=>({version:1,values:{'dinah-workbench-layout':'?open=workspace,chat-browser,needs-you'}});a.SaveUIState=async()=>{};})();`});});
 let currentMode='baseline';
 await context.route('**/workbench.js',async route=>{const r=await route.fetch();let body=await r.text();if(['scan-only','combined'].includes(currentMode))body=body.replace('function consider(el) {',"function consider(el) { if(el.closest('[data-ui-scroll-ignore]'))return;").replace('function scan(scope) {',"function scan(scope) { if(scope.nodeType===1&&scope.closest('[data-ui-scroll-ignore]'))return;");await route.fulfill({response:r,body});});
 await context.route(url=>url.pathname==='/app.js',async route=>{const r=await route.fetch();let body=await r.text();if(['scan-only','combined'].includes(currentMode))body=body.replace("ol.className = 'activity run-log';","ol.className = 'activity run-log'; ol.dataset.uiScrollIgnore='';");await route.fulfill({response:r,body});});
 await context.route(url=>url.pathname==='/pudl/pudl-menu.js',async route=>{const response=await route.fetch();await route.fulfill({response,...(currentMode==='menu-only'?{body:fs.readFileSync(path.join(root,'pudl-menu-candidate.js'),'utf8')}:{})});});
 const page=await context.newPage(),errors=[];let nativeCalls={};page.on('requestfinished',request=>{if(new URL(request.url()).pathname!=='/bridge/call')return;const method=request.postDataJSON()?.method,t=request.timing(),duration=t.responseEnd-t.requestStart;if(!method||duration<0)return;const row=nativeCalls[method]||={count:0,totalMs:0,maxMs:0};row.count++;row.totalMs+=duration;row.maxMs=Math.max(row.maxMs,duration);});page.on('pageerror',e=>errors.push(e.message));
 await page.goto(fs.existsSync(path.join(root,'browser-auth.json'))?new URL('/index.html',entry).href:entry); await context.storageState({path:path.join(root,'browser-auth.json')});const base=new URL('/index.html',page.url()).href;
 const cdp=await context.newCDPSession(page);await cdp.send('Performance.enable');
 const metrics=async()=>Object.fromEntries((await cdp.send('Performance.getMetrics')).metrics.map(m=>[m.name,m.value]));
 const results=[];
 let traceTotals={}; const traceCollect=p=>p.value.forEach(e=>{if(e.ph==='X'&&['Paint','Layout','UpdateLayoutTree','PrePaint','CompositeLayers'].includes(e.name))traceTotals[e.name]=(traceTotals[e.name]||0)+(e.dur||0)/1000;}); cdp.on('Tracing.dataCollected',traceCollect);
 async function startProfile(){ if(!process.env.PROFILE)return; traceTotals={}; await cdp.send('Profiler.enable');await cdp.send('Profiler.start');await cdp.send('Tracing.start',{categories:'devtools.timeline',options:'record-as-much-as-possible'}); }
 async function stopProfile(){ if(!process.env.PROFILE)return null; const {profile}=await cdp.send('Profiler.stop');const parents=new Map();profile.nodes.forEach(n=>(n.children||[]).forEach(child=>parents.set(child,n.id)));const raw=new Map(profile.nodes.map(n=>[n.id,n]));const label=n=>(n.callFrame.functionName||'(anonymous)')+'@'+n.callFrame.url.split('/').pop().split('?')[0]+':'+n.callFrame.lineNumber;const self={};(profile.samples||[]).forEach((id,i)=>{const n=raw.get(id);let name=label(n);if(!n.callFrame.url&&parents.has(id))name+=' <- '+label(raw.get(parents.get(id)));self[name]=(self[name]||0)+(profile.timeDeltas[i]||0)/1000;});await cdp.send('Tracing.end');await new Promise(resolve=>cdp.once('Tracing.tracingComplete',resolve));return {timeline:traceTotals,topSelf:Object.entries(self).sort((a,b)=>b[1]-a[1]).slice(0,12)}; }

 for(const mode of (process.env.VARIANTS?process.env.VARIANTS.split(','):process.env.PROFILE?['baseline','offscreen']:['baseline','offscreen','baseline','offscreen'])){
  currentMode=mode; nativeCalls={};await page.goto(base);await page.waitForFunction(()=>window.dinahWorkbench&&window.dinahUIState).catch(()=>{throw Error('Application initialization: '+JSON.stringify(errors));});
  if(['offscreen','combined'].includes(mode))await page.addStyleTag({content:'.chat-instance .activity-row {content-visibility:auto;contain-intrinsic-block-size:auto 120px;}'});
  if(process.env.RIBBON){const toggle=page.getByRole('button',{name:'Show as ribbon',exact:true});if(await toggle.count())await toggle.click();await page.locator('[data-menubar] .menubar-title').filter({hasText:/^View$/}).click();}
  if(!await page.evaluate(()=>go.main.App.OpenSessionAt.toString().includes('new Promise')))throw Error('Provider isolation missing');
  await startProfile();const before=await metrics(),start=performance.now();
  console.log(JSON.stringify({phase:'opening',mode}));
  await page.evaluate(async histories=>{window.__benchKeys=[];for(let i=0;i<histories.length;i++){const h=histories[i],key=await dinahWorkbench.open({kind:'chat-session',workspace:h.cwd,path:h.id});__benchKeys.push(key);pudlWindows.snap(key,['top-left','top-right','bottom-left','bottom-right'][i]);}},histories);
  console.log(JSON.stringify({phase:'opened',mode}));
  await page.waitForFunction(()=>__benchKeys.length===4&&__benchKeys.every(k=>{const w=document.querySelector('[data-win="'+CSS.escape(k)+'"]');return w&&w.querySelectorAll('.activity-row').length>50&&!w.querySelector('[data-view-id=replay]:not([hidden])')&&[...w.querySelectorAll('[data-view-id=logs]>*')].some(log=>!log.hidden&&log.textContent.length>5000)}),null,{timeout:120000}).catch(async error=>{console.log(JSON.stringify(await page.evaluate(()=>__benchKeys.map(k=>{const w=document.querySelector('[data-win="'+CSS.escape(k)+'"]');return {rows:w?.querySelectorAll('.activity-row').length,logs:[...w.querySelectorAll('[data-view-id=logs]>*')].map(e=>({hidden:e.hidden,characters:e.textContent.length})),replayVisible:!!w.querySelector('[data-view-id=replay]:not([hidden])')};}))));throw error;});
  await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
  const restored=await metrics(),restoreWall=performance.now()-start,restoreProfile=await stopProfile();
  const details=await page.evaluate(()=>{const rows=[...document.querySelectorAll('.activity-row')],heights=rows.map(e=>e.getBoundingClientRect().height).filter(n=>n>0).sort((a,b)=>a-b);return {ignoredLogs:document.querySelectorAll('[data-ui-scroll-ignore]').length,rows:rows.length,medianRow:heights[Math.floor(heights.length/2)],nodes:document.querySelectorAll('*').length};});
  if(process.env.RIBBON){const toggle=page.getByRole('button',{name:'Show as ribbon',exact:true});if(await toggle.count())await toggle.click();await page.locator('[data-menubar] .menubar-title').filter({hasText:/^View$/}).click();}
  await startProfile();const resizeStart=await metrics(),wall=performance.now();
  for(let i=0;i<24;i++){await page.setViewportSize({width:1920-i*18,height:1080-i*8});await page.evaluate(()=>new Promise(r=>requestAnimationFrame(r)));}
  const resized=await metrics(),resizeProfile=await stopProfile();
  const delta=(a,b)=>Object.fromEntries(['LayoutDuration','RecalcStyleDuration','ScriptDuration','TaskDuration'].map(k=>[k,Math.round((b[k]-a[k])*1000)]));
  const result={mode,ribbon:!!process.env.RIBBON,nativeCalls,restoreProfile,resizeProfile,histories:histories.map(h=>h.count),...details,restoreWall:Math.round(restoreWall),restore:delta(before,restored),resizeWall:Math.round(performance.now()-wall),resize:delta(resizeStart,resized)};results.push(result);console.log(JSON.stringify(result));
  await page.setViewportSize({width:1920,height:1080});
 }
 fs.writeFileSync(path.join(root,process.env.MENU_COMPARE?'menu-results.json':process.env.VARIANTS?'candidate-results.json':process.env.PROFILE?'profile-results.json':'results.json'),JSON.stringify({results,errors},null,2));
 if(errors.length)throw Error(errors.join('\n'));
 }finally{await browser.close();}
})().catch(e=>{console.error(e.stack);process.exitCode=1;});
