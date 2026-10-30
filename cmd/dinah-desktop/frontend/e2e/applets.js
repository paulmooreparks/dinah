// Bundled SDK, durable human requests, applet results and workspace contributions.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: {width:1440,height:1000},
    app: () => {
      const copy = v => JSON.parse(JSON.stringify(v));
      const list = () => JSON.parse(localStorage.getItem('fixture-applet-invocations') || '[]');
      const save = values => localStorage.setItem('fixture-applet-invocations',JSON.stringify(values));
      // The suite's own name for firing an event, kept from before the fixture.
      window.__events = (name,value) => window.__emit(name,value);
      window.__answers = []; window.__rejections = []; window.__exports = [];
      window.__postNotification = () => {const values=list();values.push({id:'notice-1',workspace:'C:/Alpha',ownerKind:'chat',ownerId:'Chat A',action:'dinah.notifications/post@1',status:'done',version:1,outputJSON:JSON.stringify({title:'Review the Release Notes',body:'The notes are saved and ready for review.'})});save(values);__events('applet:invocation',{});};
      // A human request an applet raised from a chat, waiting in Notifications.
      window.__seedRequest = (id,workspace,title,items) => {const values=list();values.push({id,workspace,ownerKind:'chat',ownerId:'Chat A',action:'example.checklist/request@1',status:'waiting',version:1,deadline:new Date(Date.now()+3600000).toISOString(),outputJSON:JSON.stringify({title,items}),request:{title,body:'',fields:items.map((label,i)=>({name:'item_'+(i+1),label,type:'boolean',required:true})).concat([{name:'notes',label:'Notes',type:'string'}])}});save(values);__events('applet:invocation',{});};
      window.__bumpVersion = id => {const values=list();values.find(v=>v.id===id).version++;save(values);};
      window.__seedChatArtifact = () => {const values=list();values.push({id:'chat-image',workspace:'C:/Alpha',ownerKind:'chat',ownerId:'Chat A',action:'example.images/draw@1',status:'done',outputJSON:'{}',artifacts:[{id:'sample-image',name:'sample.svg',mime:'image/svg+xml'}]},{id:'chat-other',workspace:'C:/Alpha',ownerKind:'chat',ownerId:'Chat B',action:'private.other/action@1',status:'done',outputJSON:'{}'});save(values);};
      const session = id => ({sessionId:id,title:id,cwd:'C:/Alpha',open:false,working:false,queued:[]});
      const svg='<svg xmlns="http://www.w3.org/2000/svg" width="200" height="80"><text x="10" y="30">Sample</text></svg>';
      const requests=async()=>{const result=list().filter(v=>v.status==='waiting');if(window.__holdRequests)await new Promise(resolve=>{window.__releaseRequests=resolve;});return result;};
      const results=async()=>list().filter(v=>!['waiting','running'].includes(v.status)&&!v.resultDismissed);
      return {
        // The fixture defines AppletAttention, which the SDK prefers to
        // AppletRequests and AppletResults when it exists, so it answers
        // from this suite's two lists as that pair did.
        AppletAttention:async()=>{const [r,s]=await Promise.all([requests(),results()]);return {requests:r,results:s};},
        AppletRequests:requests,AppletResults:results,
        State:async()=>({workspace:'C:/Alpha',recent:['C:/Alpha','C:/Beta'],approvals:'devin',devin:'devin',catalog:[],defaults:{},pinned:[],titles:{}}),
        AgentProviders:async()=>[],
        Sessions:async()=>['Chat A','Chat B'].map(session),OpenSessionAt:async(ws,id)=>({...session(id),open:true}),
        AppletInvocationsAt:async workspace=>list().filter(v=>v.workspace===workspace),
        DismissAppletResultAt:async(workspace,id)=>{const values=list(),value=values.find(v=>v.workspace===workspace&&v.id===id);if(!value||['waiting','running'].includes(value.status))throw new Error('Result unavailable');value.resultDismissed=true;save(values);},
        AppletInvocationAt:async(workspace,id)=>{const value=list().find(v=>v.id===id&&v.workspace===workspace);if(!value)throw new Error('Foreign invocation');return value;},
        AnswerAppletRequest:async(workspace,id,version,responseJSON)=>{
          const values=list(),value=values.find(v=>v.id===id&&v.workspace===workspace);
          if(!value||value.status!=='waiting'||value.version!==version){__rejections.push(id);throw new Error('The request changed. Refresh before answering.');}
          const response=JSON.parse(responseJSON);
          if(value.request.fields.some(f=>f.required&&!(f.name in response)))throw new Error('Missing answer');
          __answers.push({id,workspace,version,response});value.status='done';value.version++;value.outputJSON=JSON.stringify({...JSON.parse(value.outputJSON),response});delete value.request;save(values);return copy(value);
        },
        CancelAppletInvocationAt:async(workspace,id)=>{const values=list(),value=values.find(v=>v.id===id&&v.workspace===workspace);if(!value)throw new Error('Foreign invocation');value.status='cancelled';value.version++;delete value.request;save(values);},
        AppletArtifactAt:async(workspace,id)=>{if(workspace!=='C:/Alpha'||id!=='sample-image')throw new Error('Foreign artifact');return{id,name:'sample.svg',mime:'image/svg+xml',size:svg.length,sha256:'a'.repeat(64),dataBase64:btoa(svg)};},
        SaveAppletArtifactAt:async(workspace,id)=>{__exports.push({workspace,id});return 'C:/Test/sample.svg';}
      };
    },
    ready: 'workbench',
  });
  const { page, ok } = t;
  try {
    ok('the Workspace pane has no applet sections',await page.locator('#workspace-applets').evaluate(root=>root.children.length===0));
    await page.evaluate(()=>__seedRequest('request-1','C:/Alpha','Release Readiness',['Tests pass','Installation checked']));
    await page.evaluate(()=>dinahWorkbench.open('needs-you'));
    const question=()=>page.locator('#applet-question-request-1');await question().waitFor({state:'visible'});
    ok('a human request shows in Notifications',await question().getByText('Release Readiness',{exact:true}).isVisible());
    ok('human request form fits its docked pane',await question().evaluate(root=>root.scrollWidth<=root.clientWidth+1));
    await t.shot('applet-needs-you-wide');
    await page.evaluate(()=>__bumpVersion('request-1'));await question().getByRole('button',{name:'Complete Request'}).click();
    await page.waitForFunction(()=>__rejections.length===1);
    ok('stale request version remains pending and retryable',await question().getByRole('button',{name:'Complete Request'}).isEnabled()&&await page.evaluate(()=>__answers.length===0));
    await page.evaluate(()=>dinahAppletSDK.refreshRequests());
    await question().getByLabel('Tests pass').check();await question().getByLabel('Notes').fill('Installation needs another check.');
    await question().getByRole('button',{name:'Complete Request'}).click();await question().waitFor({state:'detached'});
    ok('unchecked required boolean is submitted as false',await page.evaluate(()=>__answers[0].response.item_1===true&&__answers[0].response.item_2===false));
    ok('human response appears in the completed invocation',await page.evaluate(async()=>JSON.parse((await go.main.App.AppletInvocationAt('C:/Alpha','request-1')).outputJSON).response.notes==='Installation needs another check.'));
    await page.locator('#applet-result-request-1').waitFor({state:'visible'});
    ok('completed response moves to a result notice',await page.locator('#applet-result-request-1').textContent().then(text=>text.includes('Installation needs another check.')));
    ok('duplicate completion is rejected',await page.evaluate(async()=>{try{await go.main.App.AnswerAppletRequest('C:/Alpha','request-1',2,'{}');return false;}catch(_){return true;}}));
    await page.evaluate(()=>__seedRequest('request-2','C:/Beta','Beta Review',['Review Beta']));
    await page.reload();await page.evaluate(()=>dinahWorkbench.open('needs-you'));await page.locator('#applet-question-request-2').waitFor({state:'visible'});
    ok('pending requests reappear after a reload',await page.locator('#applet-question-request-2').getByText('Beta Review',{exact:true}).isVisible());
    ok('completed result persists beside another workspace request',await page.locator('#applet-result-request-1').isVisible());
    await page.locator('#applet-result-request-1').getByRole('button',{name:'Dismiss',exact:true}).click();
    await page.locator('#applet-result-request-1').waitFor({state:'detached'});
    ok('dismissal preserves the saved output',await page.evaluate(async()=>!!(await go.main.App.AppletInvocationAt('C:/Alpha','request-1')).outputJSON));
    await page.evaluate(()=>{__holdRequests=true;window.__staleRefresh=dinahAppletSDK.refreshRequests();});await page.waitForFunction(()=>typeof __releaseRequests==='function');await page.evaluate(()=>{__holdRequests=false;});
    await page.locator('#applet-question-request-2').getByRole('button',{name:'Cancel Request'}).click();await page.locator('#applet-question-request-2').waitFor({state:'detached'});
    await page.evaluate(()=>{__releaseRequests();return __staleRefresh;});
    ok('older request refresh cannot resurrect a cancelled request',await page.locator('#applet-question-request-2').count()===0);
    ok('cancelling removes the request from Notifications',await page.evaluate(async()=>(await go.main.App.AppletInvocationAt('C:/Beta','request-2')).status==='cancelled'));
    await page.evaluate(()=>dinahWorkbench.open({kind:'applet-artifact',workspace:'C:/Alpha',path:'sample-image'}));
    const artifact=page.locator('.win.active[data-win-kind="applet-artifact"]');await artifact.getByRole('img',{name:'sample.svg'}).waitFor();
    ok('artifact viewer renders the scoped image',await artifact.locator('img').evaluate(img=>img.complete&&img.naturalWidth>0));
    await artifact.getByRole('button',{name:'Save SVG…'}).click();await page.waitForFunction(()=>__exports.length===1);
    ok('artifact export retains workspace ownership',await page.evaluate(()=>__exports[0].workspace==='C:/Alpha'&&__exports[0].id==='sample-image'));
    await t.shot('applet-artifact-wide');
    await page.evaluate(()=>dinahWorkbench.open('workspace'));
    await page.evaluate(()=>__seedChatArtifact());
    await page.evaluate(()=>{dinahAppletSDK.setWorkspace('C:/Beta');__events('applet:invocation',{});});
    await page.evaluate(()=>dinahWorkbench.open('needs-you'));
    await page.locator('#applet-result-chat-image').getByRole('button',{name:'Open sample.svg'}).click();
    ok('Notifications opens a result from its source workspace',await page.locator('.win.active[data-win-kind="applet-artifact"]').getByRole('img',{name:'sample.svg'}).isVisible());
    await page.evaluate(()=>dinahWorkbench.open('needs-you'));
    await t.shot('applet-results-wide');
    await page.setViewportSize({width:480,height:780});
    await page.evaluate(()=>dinahWorkbench.open('needs-you'));
    await page.locator('#applet-result-chat-image').waitFor({state:'visible'});
    await t.shot('applet-results-narrow');
    ok('result notice fits a narrow pane',await page.locator('#applet-result-chat-image').evaluate(root=>root.scrollWidth<=root.clientWidth+1));
    await page.setViewportSize({width:1440,height:1000});
    await page.reload();await page.evaluate(()=>dinahWorkbench.open('needs-you'));await page.locator('#applet-result-chat-image').waitFor({state:'visible'});
    ok('dismissed result stays dismissed after reload',await page.locator('#applet-result-request-1').count()===0);
    await page.evaluate(()=>__postNotification());
    await page.locator('#applet-result-notice-1').getByText('Review the Release Notes',{exact:true}).waitFor({state:'visible'});
    ok('posted notification displays its title and message',await page.locator('#applet-result-notice-1').getByText('The notes are saved and ready for review.',{exact:true}).isVisible());
    ok('a posted notification shows as a toast with its title and message',await page.locator('.toast').filter({hasText:'Review the Release Notes The notes are saved and ready for review.'}).count()>0);
    await page.locator('#applet-result-notice-1').getByRole('button',{name:'Dismiss',exact:true}).click();
    await page.locator('#applet-result-notice-1').waitFor({state:'detached'});
    await page.evaluate(()=>dinahWorkbench.open('workspace'));
    await page.locator('#workspace-chats a[data-session="Chat A"]').click();
    const chat=page.locator('.win.active[data-win-kind="chat-session"]');
    ok('Chat does not duplicate Applet Results below its composer',await chat.locator('.chat-applet-results').count()===0);
    for (const size of [{width:1440,height:1000},{width:480,height:780}]) {
      await page.setViewportSize(size);
      await chat.locator('[data-view-id="composer"]').waitFor({state:'visible'});
      ok('Chat composer fits at '+size.width,await chat.evaluate(win=>{
        const pane=win.querySelector('.chat-pane').getBoundingClientRect();
        const composer=win.querySelector('[data-view-id="composer"]').getBoundingClientRect();
        return composer.height>0&&composer.bottom<=pane.bottom+1&&composer.bottom<=innerHeight&&composer.top>=pane.top;
      }));
      await t.shot('chat-composer-'+size.width);
    }
    await page.setViewportSize({width:1440,height:1000});
    await page.evaluate(()=>dinahWorkbench.open('workspace'));
    ok('workspace groups share the Sources tree toggle and one heading style',await page.evaluate(()=>{
      const heads=[...document.querySelectorAll('#sidebar > .browser-group:not([hidden]) > .browser-group-head, #sidebar > .ws-frozen > .browser-group:not([hidden]) > .browser-group-head')];
      const reference=getComputedStyle(heads[0].querySelector('.browser-group-title'));
      return heads.length>=2&&heads.every(head=>{
        const toggle=head.querySelector('.browser-group-toggle'),title=getComputedStyle(head.querySelector('.browser-group-title')),box=toggle.getBoundingClientRect();
        return toggle.classList.contains('tree-toggle')&&Math.abs(box.width-16)<1&&Math.abs(box.height-16)<1&&title.fontSize===reference.fontSize&&title.fontFamily===reference.fontFamily;
      });
    }));
    await t.shot('workspace-groups-wide');
  } finally {await t.close();}
})().catch(err=>{console.error(err);process.exitCode=1;});
