// Composer applet references use the same picker from Add and @.
const { launch } = require('./lib/page');
const { openMenu } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: {width:1280,height:900},
    app: () => {
      const workspace = 'C:/Synthetic', info = id => ({sessionId:id,title:id,cwd:workspace,open:true,working:false,queued:[]});
      // Sketch Pad is the applet of a tool window the suite registers; the
      // others are callable applets with no window of their own.
      const catalog = [
        {id:'example.sketch',title:'Sketch Pad',description:'Draw and describe sketches.',actions:[{ref:'example.sketch/draw@1',title:'Draw Sketch',description:'Create an SVG.'}]},
        {id:'example.checklist',title:'Review Checklist',description:'Ask a person to review a checklist.',actions:[{ref:'example.checklist/request@1',title:'Request Review',description:'Wait for a human response.'}]},
        ...['Workspace','Chat','Notifications','Settings'].map(title=>({id:'dinah.'+title.toLowerCase(),title,description:'Built-in applet interfaces.',actions:[{ref:'dinah.'+title.toLowerCase()+'/read@1',title:'Read '+title,description:'Use the built-in applet.'}]})),
        {id:'example.view',title:'Display Only',description:'No callable actions.',actions:[]}
      ];
      return {
        State:async()=>({workspace,recent:[workspace],devin:'devin',approvals:'devin'}),
        AgentProviders:async()=>[],
        Sessions:async()=>['First','Second'].map(id=>({...info(id),open:false})),
        OpenSessionAt:async(ws,id)=>info(id),
        Send:async(id,text,doc,atts)=>{__calls.push({id,text,atts});},
        PickAttachments:async()=>window.__pickedFiles || [],
        AppletInvocationsAt:async()=>[],
        InvokeAppletAt:async()=>{__invocations++;throw new Error('Selection must not invoke an action');},
        AppletCatalog:async()=>{
          __catalogCalls++;
          if(__catalogMode==='hold')await new Promise(resolve=>{window.__releaseCatalog=resolve;});
          if(__catalogMode==='failed')throw new Error('Catalog unavailable');
          return catalog;
        }
      };
    },
    init: () => { window.__calls = []; window.__invocations = 0; window.__catalogMode = 'ready'; window.__catalogCalls = 0; },
    ready: 'workbench',
  });
  const { page, ok } = t;
  try {
    await page.evaluate(()=>dinahWorkbench.registerLauncher('sketch',{title:'Sketch Pad',description:'Draw sketches for the chat.',applet:'example.sketch',run(){}}));
    const front = () => page.locator('.win.active[data-win-kind="chat-session"]');
    const prompt = () => front().locator('[data-view-id="prompt"]');
    const picker = () => front().locator('[data-view-id="slash-pop"]');
    const open = async id => {
      await page.evaluate(()=>dinahWorkbench.open('workspace'));
      await page.locator('#workspace-chats a[data-session="'+id+'"]').click();
      await front().locator('[data-view-id="session-title"]').getByText(id,{exact:true}).waitFor();
    };
    // The Add menu, opened from its control or from the menu its group
    // folded into, and one of its rows.
    const addMenu = () => openMenu(page, front(), 'add');
    const addRow = async words => (await addMenu()).locator('.menu-action', {hasText: words}).first();
    const addApplets = async () => { await (await addRow('Applets…')).click(); };
    const choose = async query => {
      await prompt().fill(query);
      await picker().locator('.mention-row').first().waitFor();
      await prompt().press('Enter');
    };
    const send = async () => {
      const count = await page.evaluate(()=>__calls.length);
      await prompt().press('Control+Enter');
      await page.waitForFunction(count=>__calls.length===count+1,count);
      return page.evaluate(()=>__calls.at(-1));
    };
    await page.evaluate(()=>{__catalogMode='hold';});
    await open('First');
    let applets = await addRow('Applets…');
    ok('Add includes Applets',await applets.isVisible());
    await t.shot('chat-add-applets-wide');
    await applets.click();
    ok('picker explains pending catalog load',(await picker().textContent()).includes('Loading applets'));
    await page.waitForFunction(()=>typeof __releaseCatalog==='function');
    await page.evaluate(()=>{__catalogMode='ready';__releaseCatalog();});
    await picker().getByText('Sketch Pad',{exact:true}).waitFor();
    const groups = await picker().evaluate(el=>{const out={};let g='';el.querySelectorAll('.md-section-label, .mention-name').forEach(n=>{if(n.classList.contains('md-section-label'))g=n.textContent;else(out[g]||=[]).push(n.textContent);});return out;});
    ok('Add lists the windows, with the applet of a window\'s own tool folded into it, then the other callable applets',(groups.Windows||[]).includes('Sketch Pad')&&groups.Windows.includes('Editor')&&
      (groups.Applets||[]).join('|')==='Review Checklist|Workspace|Chat|Notifications|Settings');
    await t.shot('chat-applet-picker-wide');
    for (let i = 0; i < 30 && (await picker().locator('[aria-selected="true"] .mention-name').textContent()) !== 'Sketch Pad'; i++) await prompt().press('ArrowDown');
    ok('selected Add row has accent fill and contrasting text', await picker().locator('[aria-selected="true"]').evaluate(el => { const probe = document.createElement('span'); probe.style.color = 'var(--on-accent)'; probe.style.backgroundColor = 'var(--accent)'; el.append(probe); const expected = getComputedStyle(probe), actual = getComputedStyle(el), text = getComputedStyle(el.querySelector('.md-meta')); const result = actual.backgroundColor === expected.backgroundColor && text.color === expected.color; probe.remove(); return result; }));
    await prompt().press('Enter');
    ok('keyboard selection inserts a readable applet token',await prompt().inputValue()==='@Sketch_Pad ');
    ok('selecting an applet does not invoke it or send a message',await page.evaluate(()=>__invocations===0&&__calls.length===0));
    await prompt().fill('@Sketch_Pad draw a floor plan for Synthetic Example.');
    let sent = await send();
    ok('Send supplies the selected window and its tool\'s applet',sent.id==='First'&&sent.atts.length===1&&sent.atts[0].kind==='window'&&sent.atts[0].window==='sketch'&&sent.atts[0].provider==='example.sketch');
    await prompt().fill('@Sketch_Pad this token was not selected.');
    sent = await send();
    ok('sending clears the previous selection',sent.atts.length===0);
    await prompt().fill('@review');
    await picker().getByText('Review Checklist',{exact:true}).waitFor();
    await prompt().press('Tab');
    ok('@ searches applets and Tab selects',await prompt().inputValue()==='@Review_Checklist ');
    await prompt().fill('The applet token was removed.');
    sent = await send();
    ok('removing the token removes its reference',sent.atts.length===0);
    await choose('@draw');
    ok('action titles also find their applet',await prompt().inputValue()==='@Sketch_Pad ');
    await prompt().fill('@Sketch_Padding is a different word.');
    sent = await send();
    ok('extending a token does not retain the applet reference',sent.atts.length===0);
    await choose('@sketch');
    await prompt().fill('@Sketch_Pad, draw a floor plan.');
    sent = await send();
    ok('ordinary punctuation preserves a selected reference',sent.atts.length===1&&sent.atts[0].provider==='example.sketch');
    await choose('@review');
    await prompt().press('End');await prompt().pressSequentially('@sketch');
    await picker().getByText('Sketch Pad',{exact:true}).waitFor();await prompt().press('Enter');
    sent = await send();
    ok('a message can refer to multiple applets',sent.atts.map(a=>a.provider).sort().join('|')==='example.checklist|example.sketch');
    await choose('@sketch');
    await prompt().press('End');await prompt().pressSequentially('@sketch');
    await picker().getByText('Sketch Pad',{exact:true}).waitFor();await prompt().press('Enter');
    sent = await send();
    ok('repeated selection attaches a provider once',sent.atts.length===1);
    await choose('@sketch');
    await page.evaluate(()=>{__catalogMode='failed';});
    await open('Second');
    await addApplets();
    await picker().getByText('Applet actions could not be loaded. Choose Add > Windows and Applets to retry.').waitFor();
    ok('catalog failure has a visible retry path',await picker().isVisible());
    await prompt().press('Escape');
    await page.evaluate(()=>{__catalogMode='ready';});
    await addApplets();
    await picker().getByText('Sketch Pad',{exact:true}).waitFor();
    ok('Add Applets retries failed discovery',await picker().getByText('Settings',{exact:true}).count()===1&&!(await picker().textContent()).includes('could not be loaded'));
    await prompt().press('Escape');
    await prompt().fill('@Sketch_Pad was selected in another chat.');
    sent = await send();
    ok('applet selections remain in their owning chat',sent.id==='Second'&&sent.atts.length===0);
    await open('First');
    ok('returning to a chat retains its selected draft',await prompt().inputValue()==='@Sketch_Pad ');
    sent = await send();
    ok('the retained draft sends its applet reference',sent.id==='First'&&sent.atts.length===1);
    await prompt().fill('@');
    await picker().getByText('Second',{exact:true}).waitFor();
    ok('plain @ includes applets alongside the open chats',await picker().getByText('Sketch Pad',{exact:true}).isVisible());
    await prompt().press('Escape');
    await prompt().fill('');
    await addApplets();
    await prompt().pressSequentially('review');
    ok('typing in Add Applets keeps the applet filter, the name that matches first',(await picker().locator('.mention-name').first().textContent())==='Review Checklist'&&!(await picker().textContent()).includes('Second'));
    await prompt().press('Escape');await prompt().fill('');
    await page.evaluate(()=>{window.__pickedFiles=[{kind:'file',name:'selected.txt',mimeType:'text/plain',data:btoa('Selected file'),size:13}];});
    await (await addRow('Files From This Computer…')).click();
    await front().locator('.attachment-name').getByText('selected.txt',{exact:true}).waitFor();
    await prompt().fill('Read the selected file.');
    sent = await send();
    ok('native file picker feeds existing attachment submission',sent.atts.length===1&&sent.atts[0].kind==='file'&&sent.atts[0].name==='selected.txt'&&sent.atts[0].data===Buffer.from('Selected file').toString('base64'));
    await page.evaluate(()=>{window.__pickedFiles=[];});
    await (await addRow('Files From This Computer…')).click();
    await prompt().fill('No file selected.');
    sent = await send();
    ok('canceling native file picker adds nothing',sent.atts.length===0);
    for (const name of ['Workspace','Chat','Notifications','Settings']) {
      await choose('@'+name);
      ok('@'+name+' inserts the built-in token',await prompt().inputValue()==='@'+name+' ');
      sent = await send();
      ok('@'+name+' delegates the registered provider',sent.atts.length===1&&sent.atts[0].provider==='dinah.'+name.toLowerCase());
    }
    await prompt().fill('@file');
    await picker().locator('.mention-row').first().waitFor();
    ok('@file finds Files first, by its name rather than words in other descriptions',(await picker().locator('.mention-name').first().textContent())==='Files');
    await prompt().press('Enter');
    sent = await send();
    ok('@Files refers to the Files window (this catalog has no applet for its data)',sent.atts.length===1&&sent.atts[0].kind==='window'&&sent.atts[0].window==='files'&&sent.atts[0].provider==='');
    // The other open chats of the workspace, which this chat's agent can ask.
    await prompt().fill('@sec');
    await picker().getByText('Second',{exact:true}).waitFor();
    const chatGroups = await picker().evaluate(el=>{const out={};let g='';el.querySelectorAll('.md-section-label, .mention-name').forEach(n=>{if(n.classList.contains('md-section-label'))g=n.textContent;else(out[g]||=[]).push(n.textContent);});return out;});
    ok('@ lists the other open chats of the workspace, without the chat itself',(chatGroups['Open Chats']||[]).join('|')==='Second');
    await prompt().press('Escape');
    await page.evaluate(()=>__emit('agent:session',{sessionId:'Seconds Elsewhere',title:'Seconds Elsewhere',cwd:'C:/Another Workspace',open:true,working:false,queued:[]}));
    await prompt().fill('@sec');
    await picker().getByText('Second',{exact:true}).waitFor();
    await picker().getByText('Seconds Elsewhere',{exact:true}).waitFor();
    ok('open chats across workspaces show their workspace labels',(await picker().innerText()).includes('Another Workspace'));
    await prompt().press('Enter');
    ok('choosing a chat inserts its token',await prompt().inputValue()==='@Second ');
    await prompt().pressSequentially('which file did you change?');
    sent = await send();
    ok('Send supplies the chat mentioned',sent.atts.length===1&&sent.atts[0].kind==='chat'&&sent.atts[0].sessionId==='Second');
    await prompt().fill('@Seconds_Elsewhere');
    await picker().getByText('Seconds Elsewhere',{exact:true}).waitFor();
    await prompt().press('Enter');
    await prompt().pressSequentially('share the design');
    sent = await send();
    ok('Send preserves the cross-workspace destination',sent.atts.length===1&&sent.atts[0].sessionId==='Seconds Elsewhere');
    await page.evaluate(()=> {
      for (const [id,cwd] of [['codex:01-same-one','C:/Another Workspace'],['codex:01-same-two','C:/Third Workspace']])
        __emit('agent:session',{sessionId:id,title:'Shared Chat',cwd,open:true,working:false,queued:[]});
    });
    await prompt().fill('@Shared_Chat');
    await picker().locator('.mention-row').filter({hasText:'Third Workspace'}).click();
    ok('duplicate titles insert a token with the complete destination ID',(await prompt().inputValue()).includes('codex%3A01-same-two'));
    await prompt().pressSequentially('share your change');
    sent = await send();
    ok('workspace-qualified selection reaches the chosen duplicate',sent.atts.length===1&&sent.atts[0].sessionId==='codex:01-same-two');
    await prompt().fill('@Secondary is another word.');
    sent = await send();
    ok('a longer word does not keep the chat reference',sent.atts.length===0);
    await page.evaluate(()=>__emit('agent:sent',{sessionId:'First',text:'Another chat asks: which file?',kind:'asked',label:'Asked by Second'}));
    await front().getByText('Asked by Second',{exact:true}).waitFor();
    await front().getByText('Another chat asks: which file?',{exact:true}).waitFor();
    ok('a question from another chat is shown with who asked it',(await front().textContent()).includes('Another chat asks: which file?'));
    await page.setViewportSize({width:390,height:844});
    applets = await addRow('Applets…');
    await t.shot('chat-add-applets-narrow');
    const fits = async locator => locator.evaluate(el=>{const r=el.getBoundingClientRect();return r.width>0&&r.left>=0&&r.right<=innerWidth+1&&el.scrollWidth<=el.clientWidth+1;});
    ok('Add menu fits a narrow viewport',await fits(page.locator('.menu-panel:popover-open')));
    await applets.click();
    await picker().getByText('Sketch Pad',{exact:true}).waitFor();
    await t.shot('chat-applet-picker-narrow');
    ok('applet picker fits a narrow viewport',await fits(picker()));
  } finally { await t.close(); }
})();
