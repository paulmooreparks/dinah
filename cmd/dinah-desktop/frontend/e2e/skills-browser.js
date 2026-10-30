// The Skills browser and a skill's window: grouping by collection, the
// Edit, Parameters and Files tabs over SKILL.md as a draft, the designer
// and its Undo and Redo, Check, launching into a chat, Copy to My Skills,
// Rename… and what restores after a reload.
const { launch } = require('./lib/page');
const { press } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    state: {workspace:'C:/Alpha',workspaceId:'alpha',workspaceRecords:[{id:'alpha',name:'Alpha',executionPath:'C:/Alpha',sources:[]}],recent:[],titles:{},templates:[]},
    app: () => {
      const resource = p => ({path:p,title:p.split('/').pop()||'/',directory:!p.endsWith('.md'),kind:p.endsWith('.md')?'file':'directory',actions:p.endsWith('.md')?['preview','read']:['list'],ref:p.startsWith('/home/dinah/')?{workspaceId:'',sourceId:'dinah-home',path:p.slice('/home/dinah/'.length)}:null});
      const paths=['/','/home','/home/dinah','/home/dinah/skills','/home/dinah/skills/review','/home/dinah/skills/review/SKILL.md','/home/dinah/skills/review/references','/home/dinah/skills/review/references/checklist.md'];
      // SKILL.md as a draft, the way the documents service keeps one: the
      // form changes one front-matter line, the Editor the whole text.
      let n=0; const rev=()=>'r'+(++n);
      const drafts={};
      const skillText={personal:"---\nname: review\ndescription: Personal review instructions\nargument-hint: '<TITLE>'\nmodel: sonnet\n---\n\n# Instructions\n\nRead references/checklist.md.\n", workspace:"---\nname: review\ndescription: Workspace review instructions\nargument-hint: '<TITLE>'\n---\n\n# Instructions\n"};
      const view=d=>Object.assign({},d,{spans:{},suggestions:[],changed:d.text!==d.baseline.text,writeBack:'save',actions:['update','discard','save-as'].concat(d.text!==d.baseline.text?['save']:[])});
      const get=id=>{const d=drafts[id];if(!d)throw new Error('the draft no longer exists');return d;};
      const frontMatter=text=>{const m=/^---\n([\s\S]*?)\n---\n/.exec(text);if(!m)return {present:false,fields:[]};const fields=[];m[1].split('\n').forEach((line,i)=>{const kv=/^([\w-]+):\s*(.*)$/.exec(line);if(kv)fields.push({key:kv[1],value:kv[2].replace(/^'(.*)'$/,'$1').replace(/^"(.*)"$/,'$1'),line:i+2,scalar:true});});return {present:true,fields};};
      const problemsOf=text=>{const out=[];if(/missing\.md/.test(text))out.push({line:9,severity:'warning',message:'references/missing.md is not in the package, though its folder references is',rule:'skill'});if(!/description:/.test(text))out.push({line:1,severity:'error',message:'the front matter has no description',rule:'skill'});return out;};
      window.__change=(id,text)=>{const d=get(id);d.text=text;d.revision=rev();window.__emit('draft:changed',{id,revision:d.revision,kind:'updated'});};
      window.__draft=id=>drafts[id];
      window.__draftsAll=()=>drafts;
      return {
        OpenSessionAt:async(ws,id)=>({sessionId:id,cwd:ws,title:'Skill: review',open:true,working:true,queued:[]}),
        SkillCatalog:async()=>({skills:JSON.parse(JSON.stringify(__skills)),problems:[]}),
        ReadSkill:async id=>{const skill=__skills.find(s=>s.id===id);if(!skill)throw Error('Unavailable');return {skill:JSON.parse(JSON.stringify(skill)),text:skillText[id]||'# Instructions'};},
        OpenSkillDraft:async id=>{const skill=__skills.find(s=>s.id===id);if(!skill||!skill.ref)throw Error('no package');let d=Object.values(drafts).find(x=>x.origin.path===skill.ref.path&&x.origin.sourceId===skill.ref.sourceId);if(!d){const text=skillText[id];const did='d'+id.padEnd(31,'0');d=drafts[did]={id:did,origin:{kind:'local-folder',workspaceId:skill.ref.workspaceId,sourceId:skill.ref.sourceId,path:skill.ref.path},baseline:{text,revision:'b1',title:'SKILL.md'},title:'SKILL.md',text,revision:rev(),documentType:''};}return view(d);},
        GetDraft:async id=>view(get(id)),
        UpdateDraft:async(id,expected,text)=>{const d=get(id);if(d.revision!==expected)throw new Error('the draft was changed in another window since this one read it');d.text=text;d.revision=rev();window.__emit('draft:changed',{id,revision:d.revision,kind:'updated'});return view(d);},
        SaveDraft:async(id,expected)=>{const d=get(id);__calls.push(['save',id]);d.baseline={text:d.text,revision:rev(),title:d.title};window.__emit('draft:changed',{id,revision:d.revision,kind:'saved'});return Object.assign(view(d),{notes:[]});},
        DiscardDraft:async id=>{delete drafts[id];window.__emit('draft:changed',{id,revision:'',kind:'discarded'});},
        PreviewDraft:async id=>{const d=get(id);return {revision:d.revision,blocks:[{line:1,html:'<p>'+d.text.split('\n').pop()+'</p>'}],problems:problemsOf(d.text)};},
        // Visual view, as the whole text in one block (page-view.js covers it).
        PageDocumentOf:async(id,text)=>({type:'doc',attrs:{front:'',tail:''},content:[{type:'source_block',attrs:{uid:1,gap:'',src:text,kind:'text',html:'<p>Text</p>'}}]}),
        PageDocumentMarkdown:async doc=>doc.content.map(b=>(b.attrs.gap||'')+b.attrs.src).join(''),
        SkillFields:async id=>{const d=get(id);const fm=frontMatter(d.text);return {revision:d.revision,folder:'review',present:fm.present,fields:fm.fields,problems:problemsOf(d.text)};},
        SetSkillField:async(id,expected,key,value)=>{__calls.push(['set',key,value]);const d=get(id);if(d.revision!==expected)throw new Error('the draft changed; read it again');const m=/^---\n([\s\S]*?)\n---\n/.exec(d.text);const lines=m[1].split('\n');const i=lines.findIndex(l=>l.startsWith(key+':'));const v=/[\[<]/.test(value)?"'"+value+"'":value;if(i>=0)lines[i]=key+': '+v;else lines.push(key+': '+v);d.text='---\n'+lines.join('\n')+'\n---\n'+d.text.slice(m[0].length);d.revision=rev();window.__emit('draft:changed',{id,revision:d.revision,kind:'updated'});return view(d);},
        CheckSkillDraft:async id=>problemsOf(get(id).text),
        CopySkillToMine:async id=>{__calls.push(['copy',id]);const c=Object.assign({},__skills.find(s=>s.id===id),{id:'copied',scope:'mine',origin:'My Skills',workspace:'',workspaceId:'',ref:{workspaceId:'',sourceId:'dinah-home',path:'skills/review/SKILL.md'}});__skills.push(c);skillText.copied=skillText[id];return c;},
        RenameSkill:async(id,name)=>{__calls.push(['rename',id,name]);const s=__skills.find(x=>x.id===id);s.name=name;s.id='renamed';skillText.renamed=skillText[id];return s;},
        ListDrafts:async()=>Object.values(drafts).map(view), HomeMountPath:async()=>'/home/dinah',
        PrepareSkill:async(id,rev,ws,command)=>{__calls.push(['prepare',id,rev,ws,command]);return 'Prepared '+id+' '+command;},
        StartSkill:async(...args)=>{__calls.push(['run',...args]);return {workspace:'C:/Alpha',sessionId:'run-chat'};},
        AgentProviders:async()=>[window.__provider('devin'),Object.assign(window.__provider('codex'),{catalog:[{id:'model',category:'model',type:'select',options:[{value:'model-test',name:'Test Model'}]}]})],
        PreviewWorkspaceResource:async ref=>({resource:{ref,title:'SKILL.md',kind:'file',revision:'one',actions:['read','preview']},blocks:[{line:1,html:'<h1>Review Instructions</h1><p>Read the supporting evidence.</p>'}],problems:[]}),
        ResolveWorkspaceResourceMount:async ref=>resource('/home/dinah/'+ref.path),
        StatWorkspaceMount:async p=>resource(p),
        StatWorkspaceMountExpected:async p=>resource(p),
        ListWorkspaceMounts:async p=>({entries:paths.filter(v=>v!==p&&v.slice(0,v.lastIndexOf('/'))===(p==='/'?'':p)).map(resource),nextStart:-1}),
        ListWorkspaceMountsExpected:async p=>go.main.App.ListWorkspaceMounts(p),
        Send:async()=>__calls.push(['send'])
      };
    },
    init: () => {
      window.__calls = [];
      window.__skills = [
        {id:'personal',name:'review',description:'Personal review instructions',hint:'<TITLE>',source:'mine',scope:'mine',origin:'My Skills',location:'C:/Users/me/.dinah/home/skills',workspace:'',revision:'one',ref:{workspaceId:'',sourceId:'dinah-home',path:'skills/review/SKILL.md'}},
        {id:'workspace',name:'review',description:'Workspace review instructions',hint:'<TITLE>',source:'workspace',scope:'workspace',origin:'Alpha / Skills',location:'C:/Alpha/.devin/skills',workspace:'C:/Alpha',workspaceId:'alpha',revision:'one',ref:{workspaceId:'alpha',sourceId:'skills',path:'review/SKILL.md'}},
        {id:'other',name:'review',description:'Other workspace instructions',source:'workspace',scope:'workspace',origin:'Beta / Skills',location:'C:/Beta/.devin/skills',workspace:'C:/Beta',workspaceId:'beta',revision:'one',ref:{workspaceId:'beta',sourceId:'skills',path:'review/SKILL.md'}},
        {id:'command',name:'compact',description:'Provider command',source:'devin',scope:'provider',origin:'Devin Commands',workspace:'',revision:''}
      ];
    },
    ready: ()=>window.dinahWorkbench && window.dinahChatSkills,
  });
  const { page, ok, errors } = t;
  try {
    await page.evaluate(()=>dinahWorkbench.open('skills-browser'));
    const catalog = page.locator('[data-win-kind="skills-browser"]');
    await catalog.locator('.browser-row[data-skill-id=personal]').waitFor();
    ok('duplicate names retain separate rows',await catalog.locator('.browser-row').count()===4);
    ok('skills are grouped by the collection they come from, mine first and commands last',await catalog.locator('.browser-group-title').allTextContents().then(v=>v.join(',')==='My Skills,Alpha,Beta,Devin Commands'));
    ok('each group says where its collection is stored',await catalog.locator('.browser-group').filter({hasText:'Alpha'}).locator('.browser-group-where').textContent()==='C:/Alpha/.devin/skills');
    ok('a skill in several collections says in how many',await catalog.locator('.browser-row[data-skill-id=personal] .browser-row-badge').textContent()==='In 3 Places' && await catalog.locator('.browser-row[data-skill-id=command] .browser-row-badge').count()===0);
    await catalog.getByLabel('Skill Scope').selectOption('current');
    await page.waitForFunction(()=>document.querySelectorAll('[data-win-kind="skills-browser"] .browser-row').length===1);
    ok('current workspace filter excludes other collections',await catalog.locator('.browser-row[data-skill-id=workspace]').count()===1);
    await catalog.getByLabel('Skill Scope').selectOption('all');
    await catalog.locator('.browser-row[data-skill-id=personal] a').click();
    ok('the opened skill is the current row',await catalog.locator('.browser-row[data-skill-id=personal] a').getAttribute('aria-current')==='true');
    const skill = page.locator('[data-win-kind=skill]');
    await skill.locator('.editor-applet .cm-content').waitFor();
    ok('the Edit tab embeds the Editor on SKILL.md as a draft',await skill.locator('[data-embedded-kind="editor"]').count()===1 && (await skill.locator('.editor-applet .cm-content').textContent()).includes('Personal review instructions'));
    await skill.locator('.editor-applet [data-command-choice="view"] [data-command]').nth(3).waitFor({state:'attached'});
    ok('the Editor offers the same four views as a document',await skill.locator('.editor-applet [data-command-choice="view"] [data-command]').allTextContents().then(v=>v.join(',')==='Visual,Text,Split,Preview'));
    ok('description appears below the skill name',await skill.locator('.skill-description').textContent()==='Personal review instructions');
    ok('the Edit tab holds the text alone, with no expander',await skill.locator('#skill-1-edit .skill-form, #skill-1-edit details, #skill-1-edit .card.disclosure').count()===0);
    // The form over the front matter is the Parameters tab, with the designer.
    const goTab=name=>skill.getByRole('tab',{name,exact:true}).click();
    await goTab('Parameters');
    await skill.locator('.skill-form .skill-param-row').first().waitFor();
    ok('the description field shows its whole text',await skill.locator('#skill-1-description').evaluate(el=>el.scrollHeight<=el.clientHeight+1));
    ok('the form shows the description and the parameters the hint gives',await skill.locator('#skill-1-description').inputValue()==='Personal review instructions' && await skill.locator('.skill-param-row').count()===1 && await skill.locator('.skill-param-row input[aria-label="Parameter name"]').inputValue()==='TITLE');
    ok('other fields are listed under Other Fields, in a PUDL disclosure card',(await skill.locator('.skill-other-fields.card.disclosure .card-title').textContent())==='Other Fields (1)' && await skill.locator('[data-win-kind=skill] details').count()===0);
    await skill.locator('#skill-1-description').fill('Reviews the notes carefully');
    await skill.locator('#skill-1-description').dispatchEvent('change');
    await page.waitForFunction(()=>__calls.some(c=>c[0]==='set'&&c[1]==='description'));
    await goTab('Edit');
    await page.waitForFunction(()=>document.querySelector('[data-win-kind=skill] .editor-applet .cm-content').textContent.includes('Reviews the notes carefully'));
    ok('a form change is one front-matter line in the text',await page.evaluate(()=>{const d=Object.values(window.__draft?{}:{}).length===0;const text=document.querySelector('[data-win-kind=skill] .editor-applet .cm-content').textContent;return text.includes('description: Reviews the notes carefully')&&text.includes("argument-hint: '<TITLE>'")&&text.includes('model: sonnet')&&__calls.some(c=>c[0]==='set'&&c[1]==='description');}));
    await goTab('Parameters');
    await skill.getByRole('button',{name:'Add a Parameter',exact:true}).click();
    await skill.locator('.skill-param-row').nth(1).waitFor();
    await skill.locator('.skill-param-row').nth(1).locator('select').selectOption('choice');
    await skill.locator('.skill-param-row').nth(1).locator('input[aria-label="Choices"]').fill('short | long');
    await skill.locator('.skill-param-row').nth(1).locator('input[aria-label="Choices"]').dispatchEvent('change');
    await page.waitForFunction(()=>__calls.some(c=>c[0]==='set'&&c[1]==='argument-hint'&&c[2]==='<TITLE> <short|long>'));
    await goTab('Edit');
    await page.waitForFunction(()=>document.querySelector('[data-win-kind=skill] .editor-applet .cm-content').textContent.includes('<short|long>'));
    ok('the designer writes the argument-hint Chat reads, and previews its form',await page.evaluate(()=>{const win=document.querySelector('[data-win-kind=skill]');return win.querySelector('.editor-applet .cm-content').textContent.includes("argument-hint: '<TITLE> <short|long>'")&&win.querySelectorAll('.skill-form-preview select option').length===2&&win.querySelector('.skill-command-preview').textContent==='/review <TITLE> <short|long>';}));
    await goTab('Parameters');
    await t.shot('skills-parameters');
    ok('the designer\'s toolbar is at the top of the Parameters tab, in the Visual editor\'s strip',await skill.locator('#skill-1-parameters > .editor-toolbar-host:first-child > .editor-toolbar [aria-label="Undo"]').count()===1);
    ok('the command line stands in the window head, under the description',(await skill.locator('.skill-window > .skill-command-preview').textContent())==='/review <TITLE> <short|long>' && await skill.locator('.skill-window > .skill-command-preview').evaluate(el=>el.previousElementSibling.classList.contains('skill-description')));
    await skill.locator('.skill-param-row').nth(1).locator('input[aria-label="Required"]').uncheck();
    await page.waitForFunction(()=>__calls.some(c=>c[0]==='set'&&c[1]==='argument-hint'&&c[2]==='<TITLE> [short|long]'));
    ok('an optional parameter is written in brackets',true);
    // Undo and Redo on the designer's toolbar step through the hints it wrote.
    const sets=()=>page.evaluate(()=>__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint').map(c=>c[2]));
    await skill.getByRole('button',{name:'Undo',exact:true}).click();
    await page.waitForFunction(()=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s[s.length-1][2]==='<TITLE> <short|long>';});
    ok('Undo puts the earlier hint back',(await skill.locator('.skill-window > .skill-command-preview').textContent())==='/review <TITLE> <short|long>');
    await skill.getByRole('button',{name:'Redo',exact:true}).click();
    await page.waitForFunction(()=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s[s.length-1][2]==='<TITLE> [short|long]';});
    ok('Redo puts it forward again',(await skill.locator('.skill-param-row').nth(1).locator('input[aria-label="Required"]').isChecked())===false);
    // The toolbar's row commands act on the selected row.
    await skill.locator('.skill-param-row').nth(1).locator('select').focus();
    await skill.getByRole('button',{name:'Move Up',exact:true}).click();
    await page.waitForFunction(()=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s[s.length-1][2]==='[short|long] <TITLE>';});
    ok('Move Up moves the selected row, which stays selected',await skill.locator('.skill-param-row').first().evaluate(el=>el.classList.contains('selected')));
    await skill.getByRole('button',{name:'Remove',exact:true}).click();
    await page.waitForFunction(()=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s[s.length-1][2]==='<TITLE>';});
    await skill.getByRole('button',{name:'Undo',exact:true}).click();
    await page.waitForFunction(()=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s[s.length-1][2]==='[short|long] <TITLE>';});
    await skill.getByRole('button',{name:'Undo',exact:true}).click();
    await skill.getByRole('button',{name:'Undo',exact:true}).click();
    await page.waitForFunction(()=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s[s.length-1][2]==='<TITLE> <short|long>';});
    ok('Remove and Undo twice more return to the two parameters',await skill.locator('.skill-param-row').count()===2);
    await skill.locator('.skill-param-row').nth(1).locator('select').focus();
    const setCount=await page.evaluate(()=>__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint').length);
    await page.keyboard.press('Control+z');
    ok('after a keyboard command, focus stays on the same field of the selected row',await page.evaluate(()=>document.activeElement.closest('.skill-param-row')!==null&&document.activeElement.tagName==='SELECT'));
    await page.waitForFunction(n=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s.length===n+1&&s[s.length-1][2]!=='<TITLE> <short|long>';},setCount);
    await page.keyboard.press('Control+y');
    await page.waitForFunction(()=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s[s.length-1][2]==='<TITLE> <short|long>';});
    ok('Ctrl+Z and Ctrl+Y in the designer are its Undo and Redo',await skill.locator('.skill-param-row').count()===2);
    await skill.locator('.skill-param-row').nth(1).locator('input[aria-label="Required"]').uncheck();
    await page.waitForFunction(()=>{const s=__calls.filter(c=>c[0]==='set'&&c[1]==='argument-hint');return s[s.length-1][2]==='<TITLE> [short|long]';});
    // A change in the text redraws the form.
    await page.evaluate(()=>{const d=Object.values(__draftsAll())[0];window.__change(d.id,d.text.replace("argument-hint: '<TITLE> [short|long]'","argument-hint: '<NAME> \"<Request Name>\"'"));});
    await page.waitForFunction(()=>document.querySelectorAll('[data-win-kind=skill] .skill-param-row').length===2&&document.querySelector('[data-win-kind=skill] .skill-param-row input[aria-label="Parameter name"]').value==='NAME');
    ok('typing in the text redraws the designer',await skill.locator('.skill-param-row').nth(1).locator('select').inputValue()==='quoted');
    await page.evaluate(()=>{const d=Object.values(__draftsAll())[0];window.__change(d.id,d.text.replace('references/checklist.md','references/missing.md'));});
    await skill.getByRole('button',{name:'Check',exact:true}).click();
    await page.waitForFunction(()=>{const b=document.querySelector('[data-win-kind=skill] .editor-applet [data-ed-problems-btn]'),l=document.querySelector('[data-win-kind=skill] .editor-applet [data-ed-problems]');return b&&!b.hidden&&b.textContent==='1 Problem'&&l&&!l.hidden&&l.textContent.includes('not in the package');});
    ok('Check shows the skill problems in the Editor’s problems pane',true);
    await skill.getByRole('tab',{name:'Files',exact:true}).click();
    const files=skill.locator('[data-embedded-kind=files]');
    await files.locator('tr[data-name="SKILL.md"]').waitFor();
    ok('Files tab embeds file list without tree or preview',await files.locator('[data-fb=tree-pane]').count()===0&&await files.locator('.fm-preview').isHidden());
    ok('rooted Files shows toolbar and disables Up at the package root',await files.locator('.fm-toolbar').count()===1&&await files.locator('[data-command="up"]:disabled').waitFor({state:'attached',timeout:3000}).then(()=>true,()=>false));
    await files.locator('tr[data-name=references]').click();
    await files.locator('tr[data-name=references]').press('Backspace');
    ok('rooted Files stays at the package on Backspace',await files.locator('tr[data-name="SKILL.md"]').count()===1);
    await files.locator('tr[data-name=references]').dblclick();
    await files.locator('tr[data-name="checklist.md"]').waitFor();
    ok('subfolder opens within the embedded list with capped breadcrumbs',await page.locator('[data-win-kind=files]').count()===0&&await files.locator('[data-fb=crumbs] li').count()===2);
    await press(page,files,'home');
    await files.locator('tr[data-name="SKILL.md"]').waitFor();
    ok('Home returns to the package root',await files.locator('[data-command="up"]:disabled').waitFor({state:'attached',timeout:3000}).then(()=>true,()=>false));
    await files.locator('tr[data-name=references]').dblclick();
    await files.locator('tr[data-name="checklist.md"]').waitFor();
    await press(page,files,'up');
    await files.locator('tr[data-name="SKILL.md"]').waitFor();
    ok('programmatic paths cannot escape a rooted Files control',await page.evaluate(async()=>{
      const root=document.createElement('div');document.body.append(root);
      const base='/home/dinah/skills/review';
      const instance=dinahFiles.init(root,{rooted:true,listOnly:true,toolbar:false},{kind:'files',path:base});
      await instance.ready;
      let passed=!root.querySelector('.fm-toolbar');
      for(const path of ['/home',base+'-other',base+'/../other']) {await instance.setState('path='+encodeURIComponent(path));passed=passed&&instance.state()==='path='+encodeURIComponent(base);}
      await instance.setState('path='+encodeURIComponent(base+'/references'));
      passed=passed&&root.querySelector('tr[data-name="checklist.md"]')!==null;
      instance.destroy();root.remove();return passed;
    }));
    await t.shot('skills-files');
    await skill.getByRole('tab',{name:'Edit',exact:true}).click();
    ok('the toolbar stays in place while the list scrolls',await catalog.locator('.skills-results').evaluate(el=>{el.style.maxHeight='90px';el.scrollTop=80;var bar=el.parentElement.querySelector('.browser-toolbar');var still=el.scrollTop>0&&bar.getBoundingClientRect().bottom<=el.getBoundingClientRect().top+1;el.style.maxHeight='';el.scrollTop=0;return still;}));
    ok('an editable skill has a Parameters tab',await skill.getByRole('tab',{name:'Parameters',exact:true}).count()===1);
    await skill.getByRole('button',{name:'Use in Chat…',exact:true}).click();
    await page.getByRole('button',{name:'Add to Chat',exact:true}).click();
    ok('required fields stay in the launch dialog',await page.locator('dialog[open] .skill-field.missing').count()===1 && await skill.getByRole('tab',{name:'Edit',exact:true}).getAttribute('aria-selected')==='true');
    await page.locator('dialog[open] .skill-parameters input').fill('Review this');
    await page.getByRole('button',{name:'Add to Chat',exact:true}).click();
    await page.waitForFunction(()=>document.querySelector('[data-view-id=prompt]').value.includes('Prepared personal'));
    ok('selected package and parameters reach preparation',await page.evaluate(()=>__calls.some(c=>c[0]==='prepare'&&c[1]==='personal'&&c[4]==='/review "Review this"')));
    ok('adding instructions never sends a message',await page.evaluate(()=>!__calls.some(c=>c[0]==='send')));
    await page.evaluate(()=>dinahSkills.open(dinahSkills.records().find(s=>s.id==='personal')));
    await skill.getByRole('tab',{name:'Edit',exact:true}).click();
    await page.evaluate(()=>{document.documentElement.setAttribute('data-theme','dark');});
    await t.shot('skills-browser-dark');
    await page.evaluate(()=>{document.documentElement.setAttribute('data-theme','light');});
    await t.shot('skills-browser-light');
    await skill.getByRole('button',{name:'Run…',exact:true}).click();
    await page.locator('dialog[open] .skill-parameters input').fill('Run this');
    await page.locator('dialog[open]').getByLabel('Model',{exact:true}).selectOption('model-test');
    await t.shot('skills-launch');
    await page.evaluate(()=>document.documentElement.setAttribute('data-theme','dark'));
    await t.shot('skills-launch-dark');
    await page.getByRole('dialog').getByRole('button',{name:'Run',exact:true}).click();
    await page.locator('[data-win-kind=chat-session]').waitFor();
    ok('Run submits parameters and execution context independently',await page.evaluate(()=>__calls.some(c=>c[0]==='run'&&c[1]==='personal'&&c[3]==='C:/Alpha'&&c[4]==='codex'&&c[5]==='model-test'&&c[6]==='/review "Run this"')&&!__calls.some(c=>c[0]==='send')));
    ok('Run opens the skill\'s chat in its own window',await page.locator('[data-win-kind=chat-session]').count()===1);
    await page.evaluate(()=>dinahSkills.open(dinahSkills.records().find(s=>s.id==='personal')));    // Copy to My Skills makes a package of the person's own and opens it; Rename… renames the folder and the name.
    await skill.getByRole('button',{name:'Copy to My Skills',exact:true}).click();
    await page.waitForFunction(()=>document.querySelectorAll('[data-win-kind=skill]').length===2||__calls.some(c=>c[0]==='copy'));
    ok('Copy to My Skills copies the package',await page.evaluate(()=>__calls.some(c=>c[0]==='copy'&&c[1]==='personal')));
    const copied=page.locator('[data-win-kind=skill]').last();
    await copied.locator('.editor-applet .cm-content').waitFor();
    await copied.getByRole('button',{name:'Rename…',exact:true}).click();
    await page.locator('dialog[open] input[aria-label="New name"]').fill('notes-review');
    await page.locator('dialog[open]').getByRole('button',{name:'Rename',exact:true}).click();
    await page.waitForFunction(()=>__calls.some(c=>c[0]==='rename'));
    ok('Rename… renames the package',await page.evaluate(()=>__calls.some(c=>c[0]==='rename'&&c[1]==='copied'&&c[2]==='notes-review')));
    await page.waitForFunction(()=>[...document.querySelectorAll('[data-win-kind=skill] h2')].some(h=>h.textContent==='notes-review'));
    ok('the renamed skill opens under its new name and the old window closes',await page.locator('[data-win-kind=skill]').count()===2);
    // The later steps are about the personal skill alone.
    await page.evaluate(()=>{document.querySelectorAll('[data-win-kind=skill]').forEach(w=>{if(w.querySelector('h2').textContent!=='review')pudlWindows.close(w.dataset.win);});});
    await page.waitForFunction(()=>document.querySelectorAll('[data-win-kind=skill]').length===1);
    await page.evaluate(async()=>{__skills.find(s=>s.id==='personal').revision='two';await dinahSkills.refresh();});
    ok('changed instructions require a refresh',await skill.getByRole('button',{name:'Use in Chat…',exact:true}).isDisabled());
    await skill.getByRole('button',{name:'Refresh',exact:true}).first().click();
    await page.waitForFunction(()=>!document.querySelector('[data-win-kind=skill] .skills-toolbar button').disabled);
    await page.evaluate(async()=>{__skills=__skills.filter(s=>s.id!=='personal');await dinahSkills.refresh();});
    ok('removed source cannot be used',await skill.getByRole('button',{name:'Use in Chat…',exact:true}).isDisabled());
    await page.evaluate(()=>dinahWorkbench.open('skills-browser'));
    await catalog.getByLabel('Skill Scope').selectOption('current');
    await catalog.getByLabel('Find a Skill').fill('review');
    await skill.getByRole('tab',{name:'Files',exact:true}).click();
    await page.reload();
    await catalog.locator('.browser-row[data-skill-id=workspace]').waitFor();
    ok('browser scope and search restore',await catalog.getByLabel('Skill Scope').inputValue()==='current'&&await catalog.getByLabel('Find a Skill').inputValue()==='review');
    ok('Skill window tab restores',await skill.getByRole('tab',{name:'Files',exact:true}).getAttribute('aria-selected')==='true');
    if(errors.length)console.log(errors.join('\n'));
  } finally { await t.close(); }
})().catch(err=>{console.error(err);process.exitCode=1;});
