// Real hidden desktop bridge with an invented, preseeded local home.
// Usage: node org-chart-hidden.cjs <desktop.exe> <seed-home> <fresh-output> <presentation|engine> <refusing-provider.exe>
const fs = require('node:fs'), path = require('node:path');
const { spawn } = require('node:child_process');
const { chromium } = require('playwright-core');
const assert = require('node:assert/strict');
const [product, seed, out, profile = 'presentation', refuser] = process.argv.slice(2);
if (!product || !seed || !out || !refuser || fs.existsSync(out)) throw Error('Supply a program, an invented seed home and a fresh output folder.');
fs.mkdirSync(out,{recursive:true});
for (const name of ['home','profile','appdata','local','tmp']) fs.mkdirSync(path.join(out,name));
const home=path.join(out,'home'), entry=path.join(out,'entry.txt'), exe=path.join(out,'dinah-desktop.exe');
fs.cpSync(seed,home,{recursive:true}); fs.copyFileSync(product,exe);
const disabled = path.join(out,'provider-disabled.exe'); fs.copyFileSync(refuser,disabled);
const settingsPath=path.join(home,'settings.json'), settings=JSON.parse(fs.readFileSync(settingsPath,'utf8'));
settings.hostNotificationsOff=true;settings.providerExecutables={codex:disabled,devin:disabled,claude:disabled};
fs.writeFileSync(settingsPath,JSON.stringify(settings));
const env={SystemRoot:process.env.SystemRoot,WINDIR:process.env.WINDIR,COMSPEC:process.env.COMSPEC,
 PATH:path.join(process.env.SystemRoot,'System32'),USERPROFILE:path.join(out,'profile'),HOME:path.join(out,'profile'),
 APPDATA:path.join(out,'appdata'),LOCALAPPDATA:path.join(out,'local'),TEMP:path.join(out,'tmp'),TMP:path.join(out,'tmp'),
 CODEX_HOME:path.join(out,'profile','.codex'),CLAUDE_CONFIG_DIR:path.join(out,'profile','.claude'),OLLAMA_HOST:'http://127.0.0.1:1',DINAH_CHART_PROVIDER_REFUSAL:path.join(out,'provider-refusals.jsonl'),DINAH_HOME:home,DINAH_SHELL_ENTRY_FILE:entry};
const log=fs.openSync(path.join(out,'desktop.log'),'w'), child=spawn(exe,[],{env,cwd:out,windowsHide:true,stdio:['ignore',log,log]});
const evidence={programSha256:require('node:crypto').createHash('sha256').update(fs.readFileSync(exe)).digest('hex'),checks:[],snapshots:[],pageErrors:[],failedResources:[]};let browser,page;
function check(name,condition){assert.ok(condition,name);evidence.checks.push(name);}
async function until(fn,label){const end=Date.now()+60000;while(Date.now()<end){const value=await fn();if(value)return value;await new Promise(r=>setTimeout(r,100));}throw Error('Timed out: '+label);}
const call=(name,...args)=>page.evaluate(({name,args})=>go.main.App[name](...args),{name,args});
(async()=>{try{
 const url=await until(()=>fs.existsSync(entry)&&fs.readFileSync(entry,'utf8').trim(),'hidden entry');
 browser=await chromium.launch({headless:true,channel:'msedge'});page=await browser.newPage({viewport:{width:1440,height:1000}});
 page.on('pageerror',error=>evidence.pageErrors.push(error.message));
 page.on('response',response=>{if(response.status()>=400)evidence.failedResources.push({path:new URL(response.url()).pathname,status:response.status()});});
 await page.route('**/*',route=>['127.0.0.1','localhost','[::1]'].includes(new URL(route.request().url()).hostname)?route.continue():route.abort());
 await page.goto(url);await page.waitForFunction(()=>window.dinahUIState&&window.dinahWorkbench);await page.evaluate(()=>dinahUIState.ready);
 evidence.scripts=await page.evaluate(()=>Array.from(document.scripts).map(s=>s.src?new URL(s.src).pathname:'inline'));
 evidence.managementLoaded=await page.evaluate(()=>!!window.dinahOrgChartManagement);
 evidence.documentState=await page.evaluate(()=>document.readyState);
 evidence.resources=await page.evaluate(()=>performance.getEntriesByType('resource').map(r=>new URL(r.name).pathname));
 await page.waitForFunction(()=>window.dinahOrgChartManagement&&window.dinahCommands.rows('window').some(row=>row.label==='Org Chart'),null,{timeout:10000});
 evidence.managementReady=await page.evaluate(()=>!!window.dinahOrgChartManagement);
 const initial=await call('OrgChart');evidence.snapshots.push(initial);
 if(profile==='engine') {
  check('the switched chain and separate fork have exactly two canonical nodes',JSON.stringify(initial.chats.map(c=>c.chatId).sort())===JSON.stringify(['codex:fork','codex:old']));
  check('the predecessor retains its current head and the reserved chat stays excluded',initial.chats.find(c=>c.chatId==='codex:old').head==='claude:head'&&!initial.chats.some(c=>c.chatId==='codex:bin'));
  check('the real closed-state query includes the team and group',initial.teams.length===1&&initial.groups.length>=1&&initial.chats.every(c=>c.state.life==='closed'));
 } else check('the real query returns at least two teams, two groups and six invented chats',initial.teams.length>=2&&initial.groups.length>=2&&initial.chats.length>=6);
 const expected=initial.chats.map(c=>c.chatId).sort();
 check('query identities are canonical and unique',new Set(initial.chats.map(c=>c.chatId)).size===initial.chats.length);
 const key=await page.evaluate(()=>dinahWorkbench.open('org-chart'));
 await page.evaluate(key=>pudlWindows.snap(key,'maximized'),key);
 const chart=page.locator('[data-win="'+key+'"] .org-chart');await chart.locator('[data-org-chat]').first().waitFor();
 check('the live page renders the actual query chat identities once',JSON.stringify((await chart.locator('[data-org-chat]').evaluateAll(rows=>rows.map(r=>r.dataset.orgChat))).sort())===JSON.stringify(expected));
 check('the live page labels every actual group',await chart.locator('[data-org-group]').evaluateAll((rows,groups)=>groups.every(g=>rows.some(row=>row.dataset.orgGroup===g.id&&row.querySelector('strong').textContent===g.title)),initial.groups));
 const target=initial.chats.find(c=>c.resolution==='resolved'), changed=target.title+' Revised';
 const frames=await chart.evaluate(async(root,{id,head,title})=>{
  const svg=root.querySelector('svg'), cards=Array.from(root.querySelectorAll('[data-org-chat]'));
  const measure=()=>cards.flatMap(el=>{const r=el.getBoundingClientRect();return[r.x,r.y,r.width,r.height];});
  const baseline=measure(),samples=[];let finished=false;
  const rename=go.main.App.RenameSession(head,title).finally(()=>{finished=true;});
  for(let n=0;n<120;n++){samples.push(measure());await new Promise(requestAnimationFrame);if(finished&&root.querySelector('[data-org-chat="'+CSS.escape(id)+'"] strong')?.textContent===title)break;}
  await rename;return {baseline,samples,updated:root.querySelector('[data-org-chat="'+CSS.escape(id)+'"] strong')?.textContent===title,sameSVG:svg===root.querySelector('svg'),sameCards:cards.every(el=>el===root.querySelector('[data-org-chat="'+CSS.escape(el.dataset.orgChat)+'"]'))};
 },{id:target.chatId,head:target.head,title:changed});
 check('a real local title change preserves every observed chat box and the drawing',frames.updated&&frames.sameSVG&&frames.sameCards&&frames.samples.every(row=>JSON.stringify(row)===JSON.stringify(frames.baseline)));evidence.frames=frames;
 await chart.locator('[data-org-chat="'+target.chatId+'"] strong').filter({hasText:changed}).waitFor();
 const next=await call('OrgChart');evidence.snapshots.push(next);check('the real query generation changes with local chat metadata',next.generation!==initial.generation);
 for(const width of [1440,390])for(const theme of ['light','dark']){
  await page.setViewportSize({width,height:1000});await page.evaluate(theme=>document.documentElement.dataset.theme=theme,theme);
  await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
  check(theme+' chart fits the real reading surface at '+width,await chart.evaluate(root=>root.scrollWidth<=root.clientWidth+1&&root.querySelector('.org-chart-scroll').scrollWidth<=root.querySelector('.org-chart-scroll').clientWidth+1));
  check(theme+' chat boxes keep readable width and complete status badges at '+width,await chart.locator('[data-org-chat]').evaluateAll(rows=>rows.every(row=>{const r=row.getBoundingClientRect(),badge=row.querySelector('.badge');return r.width>=120&&(!badge||badge.getBoundingClientRect().right<=r.right-2);} )));
  await page.screenshot({path:path.join(out,'org-chart-host-'+theme+'-'+width+'.png')});
 }
 await page.reload();await page.waitForFunction(()=>window.dinahWorkbench&&window.dinahUIState);await page.evaluate(()=>dinahUIState.ready);
 await page.evaluate(()=>dinahWorkbench.open('org-chart'));
 await page.locator('.org-chart [data-org-chat="'+target.chatId+'"] strong').filter({hasText:changed}).waitFor();
 check('a page reload retains the real local chat title',true);check('the hidden page reports no errors',evidence.pageErrors.length===0);
 if(profile==='actions'){
  const panel=page.locator('.org-chart .org-action');
  async function action(chatId,kind){
   const offered=await call('OrgChartActions',chatId), descriptor=offered.actions.find(a=>a.kind===kind);
   check(kind+' is offered by the real host',!!descriptor);
   const state=new URL(descriptor.href,'http://localhost').searchParams.get('view.org-chart');
   await page.evaluate(state=>dinahWorkbench.setState('org-chart',state),state);
   await panel.getByRole('heading',{name:descriptor.label,exact:true}).waitFor();return descriptor;
  }
  async function submit(label){await panel.getByRole('button',{name:label,exact:true}).click();await panel.locator('.org-action-result[data-status]').waitFor();check(label+' succeeds in the real host',await panel.locator('.org-action-result').getAttribute('data-status')==='succeeded');}
  const before=await call('OrgChart'), titleOf=id=>before.chats.find(c=>c.chatId===id).title;
  await action('','create_group');await panel.getByLabel('Name',{exact:true}).fill('Hidden Circle');
  for(const id of ['codex:chart-Oren','codex:chart-Uma'])await panel.getByRole('checkbox',{name:titleOf(id),exact:true}).check();
  await submit('Create Group');let live=await call('OrgChart'), circle=live.groups.find(g=>g.title==='Hidden Circle');check('the real ordinary group has both selected canonical members',!!circle&&circle.memberIds.includes('codex:chart-Oren')&&circle.memberIds.includes('codex:chart-Uma'));
  await action('codex:chart-Oren','make_lead');await panel.getByLabel('Ordinary Group',{exact:true}).selectOption(circle.id);await submit('Make Lead');
  live=await call('OrgChart');check('Make Lead converts that existing group into a reporting line',live.teams.some(t=>t.leadId==='codex:chart-Oren'&&t.memberIds.includes('codex:chart-Uma')));
  await action('','create_team');await panel.getByLabel('Name',{exact:true}).fill('Hidden Team');await panel.getByLabel('Lead',{exact:true}).selectOption('codex:chart-Bo');
  await panel.getByRole('checkbox',{name:titleOf('codex:chart-Lena'),exact:true}).check();await submit('Create Team');
  live=await call('OrgChart');check('Create Team applies the new nested reporting line',live.teams.some(t=>t.leadId==='codex:chart-Bo'&&t.memberIds.includes('codex:chart-Lena')));
  await action('codex:chart-Cal','assign_team');await panel.getByLabel('Team Lead',{exact:true}).selectOption('codex:chart-Bo');await submit('Assign to a Team');
  live=await call('OrgChart');check('assignment moves the chat to its chosen lead',live.teams.some(t=>t.leadId==='codex:chart-Bo'&&t.memberIds.includes('codex:chart-Cal'))&&!live.teams.some(t=>t.leadId==='codex:chart-Nina'&&t.memberIds.includes('codex:chart-Cal')));
  await action('codex:chart-Cal','delete_chat');check('real deletion confirmation mentions retained local recovery', (await panel.locator('.org-action-confirmation').innerText()).includes('30 days'));await submit('Delete Chat');
  live=await call('OrgChart');check('deletion excludes the chat and retains it in the real Recycle Bin',!live.chats.some(c=>c.chatId==='codex:chart-Cal')&&(await call('ListChatCleanup')).removed.some(c=>c.sessionId==='codex:chart-Cal'));
  const refusalPath=path.join(out,'provider-refusals.jsonl'), priorAttempts=fs.existsSync(refusalPath)?fs.readFileSync(refusalPath,'utf8').trim().split('\n').length:0;
  check('relation changes and binning attempted no native provider',priorAttempts===0);
  const start=await action('','start_chat');await panel.getByLabel('Workspace',{exact:true}).selectOption(start.workspaces[0].id);await panel.getByLabel('Agent Program',{exact:true}).selectOption('codex');
  await panel.getByRole('button',{name:'Start a Chat',exact:true}).click();await panel.locator('.org-action-result[data-status="failed"]').waitFor();
  check('only explicit Start a Chat reaches the refusing stand-in',fs.existsSync(refusalPath)&&fs.readFileSync(refusalPath,'utf8').trim().length>0);
  await page.screenshot({path:path.join(out,'org-chart-actions-host-result.png')});
  live=await call('OrgChart');await panel.getByRole('button',{name:'Back to Chart',exact:true}).click();
  await page.waitForFunction(generation=>document.querySelector('.org-chart-canvas').dataset.generation===generation,live.generation);
  evidence.returnedStructure={expected:live.chats.map(c=>c.chatId),visible:await chart.locator('[data-org-chat]').evaluateAll(rows=>rows.map(r=>r.dataset.orgChat)),status:await chart.locator('.org-chart-status').innerText()};
  evidence.actionSnapshot=live;await page.screenshot({path:path.join(out,'org-chart-actions-host-structure.png')});
  check('returning from management renders the changed real structure',await chart.locator('[data-org-chat]').count()===live.chats.length);
  check('the management journey reports no page errors',evidence.pageErrors.length===0);
  await page.screenshot({path:path.join(out,'org-chart-actions-host-structure.png')});evidence.actionSnapshot=live;
 }

 console.log('PASS real OrgChart query, canonical rendering, local refresh, fixed frames, reload and four viewports');
}finally{
 const closing = browser ? browser.close() : Promise.resolve();
 const ended = child.exitCode===null ? new Promise(resolve=>{child.once('exit',resolve);child.kill();}) : Promise.resolve();
 await Promise.allSettled([closing,ended]);fs.closeSync(log);
 fs.writeFileSync(path.join(out,'results.json'),JSON.stringify(evidence,null,2));
}})().catch(error=>{console.error(error);process.exitCode=1;});
