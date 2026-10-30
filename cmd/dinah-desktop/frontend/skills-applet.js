(function () {
  'use strict';
  const app = window.dinahHelpers.app;
  // The provider with commands of its own, as its facts say, and its name for the skill lists' titles.
  function ownCommandsName() { var id = window.dinahOwnCommandsProvider && window.dinahOwnCommandsProvider(); return id ? window.dinahProviderName(id) : 'Agent'; }
  function ownCommandsTitle() { return ownCommandsName() + ' Commands'; }
  function host() { return window.dinahHost; }
  var catalog = { skills: [], problems: [] }, pending, listeners = new Set(), serial = 0;
  function refresh() {
    if (pending) return pending;
    if (!app() || !app().SkillCatalog) return Promise.resolve(catalog);
    pending = app().SkillCatalog().then(function (value) {
      var changed=JSON.stringify(catalog)!==JSON.stringify(value); catalog = value; if(changed)listeners.forEach(function (fn) { fn(catalog); }); return value;
    }).finally(function () { pending = null; });
    return pending;
  }
  function subscribe(fn) { listeners.add(fn); return function () { listeners.delete(fn); }; }
  function observe(root) { var timer=setInterval(function(){if(!document.hidden&&root.getClientRects().length)refresh().catch(function(){});},15000); return function(){clearInterval(timer);}; }
  function open(skill) { return dinahWorkbench.open({ kind: 'skill', path: skill.id, title: skill.name }); }
  const element = window.dinahHelpers.node;
  function button(text, run, cls) { var el = element('button', cls || 'btn btn-sm', text); el.type = 'button'; el.addEventListener('click', run); return el; }
  function message(err) { return window.dinahHelpers.message(err, 'capital'); }
  window.dinahSkills = { refresh:refresh, subscribe:subscribe, open:open, records:function () { return catalog.skills; }, forChat:function (workspace,provider) {
    return refresh().then(function (value) { return value.skills.filter(function (skill) { return (skill.scope !== 'workspace' || skill.workspace === workspace) && (skill.scope !== 'provider' || window.dinahProviderFact(provider, 'ownCommands')); }); });
  }};
  /* The Skills pane groups skills by the collection they come from, and
     each group says where that collection is stored. A skill whose name is in more than one collection
     says in how many, so copies that have drifted apart can be found. */
  function collection(skill) {
    if (skill.scope==='provider') return { key:'provider', title:ownCommandsTitle(), location:'Commands ' + ownCommandsName() + ' offers in a chat' };
    if (skill.scope==='mine') return { key:'mine', title:'My Skills', location:skill.location };
    return { key:'workspace:'+skill.workspaceId+':'+skill.location, title:skill.origin.split(' / ')[0], location:skill.location };
  }
  dinahWorkbench.register('skills-browser', { singleton:true, title:'Skills', glyph:'document', mode:'dock-left', init:function (root,opts) {
    root.classList.add('skills-browser'); var disposed=false, selected='', scope='all', query='', currentWorkspace='';
    var bar=dinahBrowserList.toolbar({
      search:{label:'Find a Skill',onInput:function (value) { query=value; draw(); opts.changed(state()); }},
      scope:{label:'Skill Scope',options:[['all','All Skills'],['mine','My Skills'],['current','This Workspace'],['workspace','All Workspace Skills'],['provider',ownCommandsTitle()]],onChange:function (value) { scope=value; draw(); opts.changed(state()); }},
      actions:[{label:'Refresh Skills',run:function () { refresh().catch(error); }}]
    });
    var filter=bar.search, select=bar.scope;
    var status=element('p','form-help skills-status'), groups=element('div','skills-results');
    root.append(bar.el,groups,status);
    function error(err) { if (!disposed) status.textContent=String(err); }
    function state() { return JSON.stringify({scope:scope,query:query,selected:selected}); }
    function draw() {
      if(disposed)return;
      groups.replaceChildren();
      var places={};
      catalog.skills.forEach(function (skill) { var c=collection(skill); (places[skill.name]=places[skill.name]||{})[c.key]=c.title; });
      var shown=catalog.skills.filter(function (skill) { return (scope==='all'||skill.scope===scope||(scope==='current'&&skill.scope==='workspace'&&skill.workspace===currentWorkspace)) && (skill.name+' '+skill.description+' '+skill.origin+' '+(skill.location||'')).toLowerCase().includes(query.toLowerCase()); });
      var order=[], byKey={};
      shown.forEach(function (skill) { var c=collection(skill); if(!byKey[c.key]){byKey[c.key]={c:c,rows:[]};order.push(c.key);} byKey[c.key].rows.push(skill); });
      order.sort(function (x,y) { function rank(k){return k==='mine'?0:k==='provider'?2:1;} return rank(x)-rank(y)||byKey[x].c.title.localeCompare(byKey[y].c.title); });
      order.forEach(function (key) {
        var c=byKey[key].c, sections=window.dinahBrowserSections, id='skills-'+key;
        var g=dinahBrowserList.group({id:id,title:c.title,count:byKey[key].rows.length,location:c.location,open:sections?sections.open(id):true,onToggle:function (open) { if(sections)sections.save(id,open); }});
        byKey[key].rows.forEach(function (skill) {
          var elsewhere=Object.keys(places[skill.name]).filter(function (k) { return k!==key; }).map(function (k) { return places[skill.name][k]; });
          var row=dinahBrowserList.row({
            title:skill.name, meta:[skill.description], clamp:true, tooltip:skill.description,
            badge:elsewhere.length?{chip:true,text:'In '+(elsewhere.length+1)+' Places',label:'Also in '+elsewhere.join(', ')}:null,
            current:selected===skill.id, data:{skillId:skill.id},
            onOpen:function () { selected=skill.id; opts.changed(state()); groups.querySelectorAll('a[data-skill-id]').forEach(function (a) { if(a.dataset.skillId===selected)a.setAttribute('aria-current','true');else a.removeAttribute('aria-current'); }); open(skill).catch(error); }
          });
          row.dataset.skillId=skill.id; g.body.append(row);
        });
        groups.append(g.el);
      });
      if(!shown.length)groups.append(dinahBrowserList.empty({title:query?'No Matching Skills':'No Skills Here',body:query?'No skill in this scope has those words in its name or description.':'A skill is a folder with a SKILL.md file, kept in a workspace or in your own folder of skills.'}));
      status.textContent=catalog.problems.length ? catalog.problems.join('\n') : '';
    }
    function setState(value) { try { var saved=JSON.parse(value||'{}'); scope=['all','mine','current','workspace','provider'].includes(saved.scope)?saved.scope:'all'; query=saved.query||''; selected=saved.selected||''; } catch (_) {} filter.value=query; select.value=scope; draw(); }
    function updateWorkspace() { app().State().then(function(value){if(!disposed){currentWorkspace=value.workspace;draw();}}).catch(error); }
    setState(opts.state); var stop=subscribe(draw), stopResources=dinahResources.subscribe(updateWorkspace), stopObserve=observe(root); updateWorkspace(); refresh().then(draw).catch(error);
    return {state:state,setState:setState,commands:function () { return [{label:'Refresh Skills',run:refresh}]; },destroy:function () { disposed=true; stop(); stopResources(); stopObserve(); }};
  }});

  /* ---- The parameter designer ------------------------------------------
     Chat reads a skill's argument-hint into a form (skill-inputs.js); the
     designer goes the other way. Each row is one parameter, and every change
     writes the hint that parse reads back into the same rows. A hint written
     by hand that would not come back unchanged leaves the table read-only. */
  var KINDS = [['text','Text'],['words','Free text, last'],['quoted','Quoted text'],['choice','Choice'],['flag','Flag'],['flag-value','Flag with a value']];
  function kindOf(spec) {
    if (spec.kind === 'bool') return 'flag';
    if (spec.flag) return 'flag-value';
    if (spec.quoted) return 'quoted';
    if (['choice','words'].includes(spec.kind)) return spec.kind;
    return 'text';
  }
  /* The spec a row describes, in the shape parse gives, so format writes it. */
  function specOf(row) {
    var name = (row.name || '').trim() || 'value', required = !!row.required;
    switch (row.kind) {
      case 'flag': return { kind: 'bool', flag: '--' + name.replace(/^-+/, ''), raw: '--' + name.replace(/^-+/, ''), required: false };
      case 'flag-value': return { kind: 'text', flag: '--' + name.replace(/^-+/, ''), raw: name.replace(/^-+/, ''), required: required };
      case 'quoted': return { kind: 'text', quoted: true, raw: name, required: required };
      case 'choice': return { kind: 'choice', raw: (row.choices || '').split(/[|,]/).map(function (c) { return c.trim(); }).filter(Boolean).join('|') || 'a|b', choices: [], required: required };
      case 'words': return { kind: 'words', raw: /\s/.test(name) ? name : name + ' text', required: required };
      default: return { kind: 'text', raw: name.replace(/\s+/g, '-'), required: required };
    }
  }
  function rowOf(spec) {
    var kind = kindOf(spec), name = spec.raw || spec.key || '';
    if (kind === 'flag' || kind === 'flag-value') name = (spec.flag || name).replace(/^-+/, '');
    return { kind: kind, name: name, required: !!spec.required, choices: kind === 'choice' ? (spec.choices || []).join(' | ') : '' };
  }
  /* Undo and Redo, which PUDL has no glyphs for, drawn in PUDL's stroke;
     the rest are PUDL's. */
  var DESIGNER_GLYPHS = {
    undo: '<path d="M5.5 4.5L2.5 7.5l3 3" fill="none" stroke="black" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/><path d="M2.5 7.5h6.5a4 4 0 0 1 0 8H7" fill="none" stroke="black" stroke-width="1.6" stroke-linecap="round"/>',
    redo: '<path d="M10.5 4.5l3 3-3 3" fill="none" stroke="black" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/><path d="M13.5 7.5H7a4 4 0 0 0 0 8h2" fill="none" stroke="black" stroke-width="1.6" stroke-linecap="round"/>'
  };
  function designerGlyph(name) {
    var span = document.createElement('span'); span.className = 'glyph'; span.setAttribute('aria-hidden', 'true');
    if (DESIGNER_GLYPHS[name]) {
      var svg = "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'>" + DESIGNER_GLYPHS[name].replace(/"/g, "'") + '</svg>';
      span.style.setProperty('--glyph', 'url("data:image/svg+xml,' + svg.replace(/#/g, '%23') + '")');
    } else span.style.setProperty('--glyph', 'var(--glyph-' + name + ')');
    return span;
  }

  /* The parameter designer: a toolbar (Undo, Redo, Add, Move Up, Move
     Down, Remove, Copy the Command), the rows, and a preview of the form
     Chat asks. Every change is one argument-hint, written through onChange;
     the command line it makes is drawn into commandEl, which the window
     keeps in its head. The toolbar's row commands act on the row that has
     focus or was last pressed. Undo and Redo step through the hints the
     designer wrote; a change made in the text is one step too, so Undo
     after it puts the earlier hint back. */
  function designer(prefix, hint, onChange, commandEl) {
    var el = element('div', 'skill-designer'), note = element('p', 'form-help'), table = element('div', 'skill-parameters-table'), preview = element('fieldset', 'skill-form-preview');
    var bar = element('div', 'toolbar editor-toolbar skill-designer-toolbar'); bar.setAttribute('role', 'toolbar'); bar.setAttribute('aria-label', 'Parameters');
    table.setAttribute('role', 'list'); table.setAttribute('aria-label', 'Parameters');
    preview.disabled = true;
    var legend = element('legend', '', 'As Chat asks for them'); preview.append(legend);
    var rows = dinahSkillInputs.parse(hint || '').map(rowOf), editable = !hint || dinahSkillInputs.roundTrips(hint), current = hint || '', undo = [], redo = [], selected = -1;
    function text() { return dinahSkillInputs.format(rows.map(specOf)); }
    /* A field that commits the value it already had (a blur after a change
       already taken) is no step and no write. */
    function write() { var next = text(); if (next === current) { draw(); return; } undo.push(current); redo.length = 0; current = next; draw(); onChange(current); }
    function take(h) { rows = dinahSkillInputs.parse(h || '').map(rowOf); editable = !h || dinahSkillInputs.roundTrips(h); current = h || ''; }
    var tools = {
      undo: { label: 'Undo', glyph: 'undo', can: function () { return editable && undo.length > 0; }, run: function () { redo.push(current); take(undo.pop()); draw(); onChange(current); } },
      redo: { label: 'Redo', glyph: 'redo', can: function () { return editable && redo.length > 0; }, run: function () { undo.push(current); take(redo.pop()); draw(); onChange(current); } },
      add: { label: 'Add a Parameter', glyph: 'plus', can: function () { return editable; }, run: function () { rows.push({ kind: 'text', name: 'value', required: true, choices: '' }); selected = rows.length - 1; write(); } },
      up: { label: 'Move Up', glyph: 'arrow-up', can: function () { return editable && selected > 0; }, run: function () { rows.splice(selected - 1, 0, rows.splice(selected, 1)[0]); selected--; write(); } },
      down: { label: 'Move Down', glyph: 'arrow-down', can: function () { return editable && selected >= 0 && selected < rows.length - 1; }, run: function () { rows.splice(selected + 1, 0, rows.splice(selected, 1)[0]); selected++; write(); } },
      remove: { label: 'Remove', glyph: 'close', can: function () { return editable && selected >= 0; }, run: function () { rows.splice(selected, 1); if (selected >= rows.length) selected = rows.length - 1; write(); } },
      copy: { label: 'Copy the Command', glyph: 'copy', can: function () { return true; }, run: function () { if (navigator.clipboard) navigator.clipboard.writeText(commandEl.textContent).catch(function () {}); } }
    };
    var buttons = {};
    [['history', ['undo', 'redo']], ['rows', ['add', 'up', 'down', 'remove']], ['command', ['copy']]].forEach(function (g) {
      var box = element('div', 'toolbar-group'); box.setAttribute('role', 'group'); box.setAttribute('aria-label', g[0] === 'history' ? 'History' : g[0] === 'rows' ? 'Rows' : 'Command');
      g[1].forEach(function (id) {
        var b = element('button', 'icon-btn'); b.type = 'button'; b.dataset.command = id; b.setAttribute('aria-label', tools[id].label); b.title = tools[id].label; b.append(designerGlyph(tools[id].glyph));
        b.addEventListener('mousedown', function (e) { e.preventDefault(); });
        b.addEventListener('click', function () { if (tools[id].can()) tools[id].run(); });
        buttons[id] = b; box.append(b);
      });
      bar.append(box);
    });
    /* PUDL's toolbar script makes the toolbar one tab stop, with the arrow
       keys along it, as it does the Editor's. */
    /* Ctrl+Z and Ctrl+Y (or Ctrl+Shift+Z) anywhere in the designer are its
       Undo and Redo, except while a field holds typing not yet committed,
       which the browser's own undo takes back. */
    el.addEventListener('keydown', function (e) {
      if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
      var k = e.key.toLowerCase(), id = k === 'z' && !e.shiftKey ? 'undo' : (k === 'y' || (k === 'z' && e.shiftKey)) ? 'redo' : null;
      if (!id) return;
      var t = e.target;
      if (t && /^(INPUT|TEXTAREA)$/.test(t.tagName) && t.type !== 'checkbox' && t.value !== t.defaultValue) return;
      e.preventDefault(); e.stopPropagation();
      if (tools[id].can()) tools[id].run();
    });
    function refresh() {
      Object.keys(buttons).forEach(function (k) { buttons[k].disabled = !tools[k].can(); });
    }
    function select(i) { selected = i; table.querySelectorAll('.skill-param-row').forEach(function (r, j) { r.classList.toggle('selected', j === i); if (j === i) r.setAttribute('aria-current', 'true'); else r.removeAttribute('aria-current'); }); refresh(); }
    table.addEventListener('focusin', function (e) { var r = e.target.closest('.skill-param-row'); if (r) select(Array.prototype.indexOf.call(table.children, r)); });
    table.addEventListener('pointerdown', function (e) { var r = e.target.closest('.skill-param-row'); if (r) select(Array.prototype.indexOf.call(table.children, r)); });
    function draw() {
      /* The rows are rebuilt; focus goes back to the same field of the
         selected row, so a command from the keyboard does not lose it. */
      var active = document.activeElement, focused = table.contains(active) ? active.getAttribute('aria-label') : null;
      table.replaceChildren();
      rows.forEach(function (row, i) {
        var tr = element('div', 'skill-param-row'), id = prefix + '-p' + i; tr.setAttribute('role', 'listitem');
        var name = element('input', 'form-input'); name.value = name.defaultValue = row.name; name.id = id + '-name'; name.placeholder = 'Name'; name.setAttribute('aria-label', 'Parameter name'); name.disabled = !editable;
        name.addEventListener('change', function () { row.name = name.value; write(); });
        var kind = element('select', 'form-select'); KINDS.forEach(function (k) { kind.append(new Option(k[1], k[0])); }); kind.value = row.kind; kind.setAttribute('aria-label', 'Parameter kind'); kind.disabled = !editable;
        kind.addEventListener('change', function () { row.kind = kind.value; if (row.kind === 'flag') row.required = false; write(); });
        var required = element('input', ''); required.type = 'checkbox'; required.checked = row.required; required.setAttribute('aria-label', 'Required'); required.disabled = !editable || row.kind === 'flag';
        required.addEventListener('change', function () { row.required = required.checked; write(); });
        var requiredLabel = element('label', 'skill-param-required'); requiredLabel.append(required, document.createTextNode(' Required'));
        var choices = element('input', 'form-input'); choices.value = choices.defaultValue = row.choices; choices.placeholder = 'a | b | c'; choices.setAttribute('aria-label', 'Choices'); choices.disabled = !editable || row.kind !== 'choice'; choices.hidden = row.kind !== 'choice';
        choices.addEventListener('change', function () { row.choices = choices.value; write(); });
        tr.append(name, kind, requiredLabel, choices);
        table.append(tr);
      });
      if (selected >= rows.length) selected = rows.length - 1;
      select(selected);
      if (focused && selected >= 0) { var again = table.children[selected].querySelector('[aria-label="' + focused + '"]'); if (again && !again.disabled && !again.hidden) again.focus({ preventScroll: true }); }
      preview.querySelectorAll('.skill-field').forEach(function (f) { f.remove(); });
      var specs = dinahSkillInputs.parse(text());
      specs.forEach(function (spec, i) { preview.append(dinahSkillInputs.field(spec, prefix + '-preview-' + i).wrap); });
      commandEl.textContent = '/' + (prefix.split(':')[1] || 'skill') + (rows.length ? ' ' + text() : '');
      note.textContent = editable ? (rows.length ? '' : 'The skill takes no parameters. Add one, and Chat will ask for it.') : 'The argument-hint in the text is written in a form the designer cannot rewrite without losing something; change it in the text.';
    }
    draw();
    el.append(note, table, preview);
    return { el: el, bar: bar, specs: function () { return rows.map(specOf); }, setHint: function (h) { if ((h || '') === current) return; undo.push(current); redo.length = 0; take(h); draw(); } };
  }

  // The Skill window's keys, for Help › Keyboard Shortcuts.
  if (window.dinahCommands && window.dinahCommands.addHelp) {
    window.dinahCommands.addHelp('Skills', [
      { where: 'Form Designer', label: 'Undo or Redo', key: 'Ctrl+Z / Ctrl+Y or Ctrl+Shift+Z' },
      { where: 'Tabs', label: 'Next or Previous Tab', key: 'Left / Right, Home / End' },
      { where: 'Run or Add to Chat', label: 'Run or Add', key: 'Enter, or Ctrl+Enter in a box' }]);
  }

  /* ---- The Skill window ------------------------------------------------ */
  dinahWorkbench.register('skill', { title:'Skill',glyph:'document',init:function (root,opts,item) {
    var disposed=false, pack, viewer, editor, draft, fields, design, formRevision='', filesViewer, filesLoading=false, version=0, activeTab='edit', launchDialog, fieldsTimer=0;
    root.classList.add('skill-window');
    var toolbar=element('div','skills-toolbar'), status=element('p','form-help'), title=element('h2',''), origin=element('p','form-help'), tabs=element('div','tablist'), body=element('div','skill-window-body'); tabs.setAttribute('role','tablist'); status.setAttribute('role','status');
    /* The command line the parameters make, in the head under the
       description, drawn live by the designer (Paul, 14 October 2026). */
    var description=element('p','skill-description'), commandLine=element('pre','code skill-command-preview');
    var panels={}, tabButtons={}, prefix='skill-'+(++serial);
    function tab(name) { if(!['edit','parameters','files'].includes(name)||(name==='parameters'&&tabButtons.parameters.hidden))name='edit'; activeTab=name; Object.keys(panels).forEach(function (key) { panels[key].hidden=key!==name; tabButtons[key].setAttribute('aria-selected',String(key===name)); tabButtons[key].tabIndex=key===name?0:-1; }); opts.changed(name);if(name==='files')loadFiles(); }
    [['edit','Edit'],['parameters','Parameters'],['files','Files']].forEach(function (entry) {
      var control=button(entry[1],function () { tab(entry[0]); }); control.className=''; control.setAttribute('role','tab'); control.id=prefix+'-tab-'+entry[0]; control.setAttribute('aria-controls',prefix+'-'+entry[0]);
      var panel=element('section','skill-panel'); panel.id=prefix+'-'+entry[0]; panel.setAttribute('role','tabpanel'); panel.setAttribute('aria-labelledby',control.id); panels[entry[0]]=panel; tabButtons[entry[0]]=control; tabs.append(control); body.append(panel);
    });
    /* The text has the Edit tab to itself; the form over the front matter
       is the Parameters tab (Paul, 14 October 2026: the form took the
       window at HD size). Both edit one draft. */
    var form=element('div','skill-form'), editorHost=element('div','skill-editor-host'), paramsBar=element('div','editor-toolbar-host skill-designer-host');
    panels.edit.append(editorHost); panels.parameters.classList.add('skill-parameters-panel'); panels.parameters.append(paramsBar, form);
    /* A nested disclosure card, closed, for the fields the form does not
       show by name. */
    function fold(title, cls) { var card=element('section','card disclosure '+cls), head=element('div','card-head'), toggle=element('button','card-toggle'), body=element('div','card-body'); toggle.type='button'; toggle.setAttribute('aria-expanded','false'); head.append(toggle, element('h3','card-title',title)); card.append(head, body); return {card:card, body:body}; }
    /* The embedded Editor is made on the host element itself, so the event
       goes to that element. */
    var check=button('Check',function(){ if(editor)editorHost.dispatchEvent(new CustomEvent('dinah:editor-check')); });
    var use=button('Use in Chat…',function(){launch(false);}), run=button('Run…',function(){launch(true);}), copy=button('Copy to My Skills',copyToMine), rename=button('Rename…',renameDialog), reload=button('Refresh',load);
    use.disabled=run.disabled=check.disabled=copy.disabled=rename.disabled=true;
    tabs.addEventListener('keydown',function(event){var names=Object.keys(tabButtons), index=names.indexOf(activeTab); if(event.key==='ArrowRight')index=(index+1)%names.length;else if(event.key==='ArrowLeft')index=(index+names.length-1)%names.length;else if(event.key==='Home')index=0;else if(event.key==='End')index=names.length-1;else return;event.preventDefault();tab(names[index]);tabButtons[names[index]].focus();});
    toolbar.append(check,run,use,copy,rename,reload); root.append(toolbar,title,description,commandLine,origin,tabs,body,status);
    async function loadFiles() {
      if (!pack || filesViewer || filesLoading || !pack.skill.ref) return;
      filesLoading=true; var request=version;
      panels.files.textContent='Loading Files…';
      try {
        var ref=Object.assign({},pack.skill.ref,{path:pack.skill.ref.path.replace(/\/SKILL\.md$/,'')});
        var resource=await app().ResolveWorkspaceResourceMount(ref);
        if(disposed||request!==version)return;
        panels.files.replaceChildren();
        filesViewer=dinahWorkbench.embed({kind:'files',workspace:ref.workspaceId,path:JSON.stringify({path:resource.path,ref:ref})},panels.files,{listOnly:true,rooted:true});
      } catch(err) { if(!disposed&&request===version)panels.files.textContent=String(err); }
      finally { if(request===version)filesLoading=false; }
    }
    function error(err) { if(!disposed)status.textContent=message(err); }
    function say(text) { if(!disposed)status.textContent=text||''; }

    /* The form over the front matter. Text and form edit one draft: a
       change here is one line of the draft, and a change in the text redraws
       the form after a pause, through the draft's events. */
    function setField(key, value) {
      if(!draft)return Promise.resolve();
      return app().SetSkillField(draft.id, formRevision, key, value).then(function (v) { formRevision=v.revision; say(''); }).catch(function (err) { error(err); return loadFields(); });
    }
    function drawForm() {
      /* The designer is kept across redraws, so focus in it is put back
         after the form around it is rebuilt. */
      var active=document.activeElement, keep=design&&(design.el.contains(active)||design.bar.contains(active))?active:null;
      form.replaceChildren(); paramsBar.replaceChildren();
      if(!fields)return;
      if(!fields.present){ form.append(element('p','form-help','The skill has no front matter yet. Set a description, and the block is added to the text.')); }
      var known={};
      function row(label, control, help) { var wrap=element('div','skill-field'), l=element('label','',label); l.htmlFor=control.id; wrap.append(l, control); if(help){var h=element('p','form-help',help); wrap.append(h);} form.append(wrap); }
      var nameField=element('input','form-input'); nameField.id=prefix+'-name'; nameField.value=fields.folder; nameField.readOnly=true;
      row('Name', nameField, 'The name is the skill’s folder, which /'+fields.folder+' invokes; Rename… changes both.');
      var desc=(fields.fields||[]).find(function (f) { return f.key==='description'; }); known.description=true;
      if(desc&&!desc.scalar){ var p=element('p','form-help','The description is written over several lines in the text; change it there.'); var dl=element('div','skill-field'); dl.append(element('label','','Description'),p); form.append(dl); }
      else { var ta=element('textarea','form-textarea'); ta.id=prefix+'-description'; ta.rows=2; ta.value=desc?desc.value:''; ta.setAttribute('aria-label','Description'); ta.addEventListener('change',function(){ setField('description', ta.value.replace(/\s*\n\s*/g,' ').trim()); }); row('Description', ta, 'Shown beside the command in Chat; what the skill does and when to use it.'); }
      var hint=(fields.fields||[]).find(function (f) { return f.key==='argument-hint'; }); known['argument-hint']=true;
      var params=element('div','skill-field'); params.append(element('label','','Parameters'));
      if(hint&&!hint.scalar){ params.append(element('p','form-help','The argument-hint is written over several lines in the text; change it there.')); }
      else { if(design&&design.setHint&&design.folder===fields.folder){ design.setHint(hint?hint.value:''); } else { design=designer(prefix+':'+fields.folder, hint?hint.value:'', function (h) { setField('argument-hint', h); }, commandLine); design.folder=fields.folder; } paramsBar.append(design.bar); params.append(design.el); }
      form.append(params);
      known.name=true;
      var others=(fields.fields||[]).filter(function (f) { return !known[f.key]; });
      if(others.length){
        var det=fold('Other Fields ('+others.length+')','skill-other-fields');
        others.forEach(function (f) {
          if(f.scalar){ var inp=element('input','form-input'); inp.id=prefix+'-f-'+f.key; inp.value=f.value; inp.addEventListener('change',function(){ setField(f.key, inp.value); }); var w=element('div','skill-field'), l=element('label','',f.key); l.htmlFor=inp.id; w.append(l,inp); det.body.append(w); }
          else { var w2=element('div','skill-field'); w2.append(element('label','',f.key), element('p','form-help','Written over several lines in the text; change it there.')); det.body.append(w2); }
        });
        form.append(det.card);
      }
      if(keep&&keep.isConnected)keep.focus({preventScroll:true});
    }
    function loadFields() {
      if(!draft)return Promise.resolve();
      return app().SkillFields(draft.id).then(function (f) { if(disposed)return; fields=f; formRevision=f.revision; drawForm(); }).catch(error);
    }
    function draftChanged(e) {
      if(!draft||!e||e.id!==draft.id)return;
      if(e.kind==='discarded'){ draft=null; design=null; if(editor){editor.destroy();editor=null;} editorHost.replaceChildren(); load(); return; }
      if(e.revision===formRevision)return;
      clearTimeout(fieldsTimer); fieldsTimer=setTimeout(loadFields, 300);
    }
    var stopDrafts = host()&&host().on ? host().on('draft:changed', draftChanged) : function(){};
    var stopResync = host()&&host().onResync ? host().onResync(loadFields) : function(){};

    async function copyToMine() {
      copy.disabled=true;
      try { var copied=await app().CopySkillToMine(item.path); await refresh(); say('Copied to My Skills as '+copied.name+'.'); await open(copied); }
      catch(err){ error(err); } finally { copy.disabled=!pack||!pack.skill.ref; }
    }
    function renameDialog() {
      if(!pack||!pack.skill.ref)return;
      var dialog=element('dialog','dialog skill-launch'), heading=element('h2','dialog-title','Rename the Skill'), input=element('input','form-input'), note=element('p','form-help'), actions=element('div','dialog-actions');
      heading.id=prefix+'-rename-title'; dialog.setAttribute('aria-labelledby',heading.id); input.id=prefix+'-rename'; input.value=pack.skill.name; input.setAttribute('aria-label','New name');
      note.textContent='The name is the folder’s name and the command: /'+pack.skill.name+' becomes /<name>. Lowercase letters, digits and hyphens suit every agent.';
      var cancel=button('Cancel',function(){dialog.close();}), ok=button('Rename',async function(){
        ok.disabled=true;
        try { var renamed=await app().RenameSkill(item.path, input.value); dialog.close(); await refresh(); var key=root.closest('[data-win]').dataset.win; await open(renamed); if(window.pudlWindows)pudlWindows.close(key); }
        catch(err){ note.textContent=message(err); ok.disabled=false; }
      },'btn btn-sm btn-primary');
      input.addEventListener('keydown',function(e){ if(e.key==='Enter'){e.preventDefault();ok.click();} });
      actions.append(cancel,ok); dialog.append(heading,input,note,actions); root.append(dialog); dialog.addEventListener('close',function(){dialog.remove();}); dialog.showModal(); input.select();
    }

    async function launch(execute) {
      if(use.disabled||!pack||execute&&run.disabled)return;
      if(launchDialog){launchDialog.focus();return;}
      var selected=pack.skill, fieldsList=[], wordsField, launching=false;
      var choices=window.dinahChatSkills ? dinahChatSkills.targets().filter(function (target) { return (!pack.skill.workspace||target.workspace===pack.skill.workspace) && (pack.skill.scope!=='provider'||window.dinahProviderFact(target.provider,'ownCommands')); }) : [];
      var dialog=element('dialog','dialog skill-launch'), heading=element('h2','dialog-title',execute?'Run Skill':'Use in Chat'), select=element('select','form-select'), note=element('p','form-help');
      launchDialog=dialog; heading.id=prefix+'-launch-title'; dialog.setAttribute('aria-labelledby',heading.id); note.setAttribute('role','status');
      select.setAttribute('aria-label','Chat'); choices.forEach(function (target,index) { select.append(new Option(target.title,String(index))); });
      var parameters=element('div','skill-parameters'), preview=element('pre','code');
      dinahSkillInputs.parse(selected.hint).forEach(function(spec,index){var field=dinahSkillInputs.field(spec,prefix+'-launch-'+index);parameters.append(field.wrap);if(spec.kind==='words')wordsField=field;else fieldsList.push(field);});
      function command(mark){var text=dinahSkillInputs.assemble(selected.name,fieldsList,wordsField?wordsField.el.value:'',wordsField&&wordsField.spec,mark,!mark);if(mark&&wordsField){var invalid=wordsField.spec.required&&!wordsField.el.value.trim();wordsField.wrap.classList.toggle('missing',invalid);wordsField.el.setAttribute('aria-invalid',String(!!invalid));}return text;}
      function update(){preview.textContent=command(false);}parameters.addEventListener('input',update);parameters.addEventListener('change',update);update();
      var workspace=element('select','form-select'), provider=element('select','form-select'), model=element('select','form-select'), providers=[];
      workspace.setAttribute('aria-label','Workspace');provider.setAttribute('aria-label','Provider');model.setAttribute('aria-label','Model');
      function models(){model.replaceChildren(new Option('Provider Default',''));var info=providers.find(function(p){return p.id===provider.value;});function add(values){(values||[]).forEach(function(value){if(value.options)add(value.options);else model.append(new Option(value.name,value.value));});}(info&&info.catalog||[]).filter(function(option){return option.category==='model';}).forEach(function(option){add(option.options);});}provider.addEventListener('change',models);models();
      if(execute&&app().AgentProviders)app().AgentProviders().then(function(value){if(dialog.isConnected){providers=value;provider.replaceChildren.apply(provider,providers.filter(function(p){return p.facts&&p.facts.takesSkills;}).map(function(p){return new Option(p.name,p.id);}));models();}}).catch(function(){});
      function labelled(text,control){var label=element('label','skill-field',text);label.append(control);return label;}
      var actions=element('div','dialog-actions'); var put=button('Add to Chat',async function () {
        var text=command(true);if(text===null){note.textContent='Fill the required parameters.';var missing=parameters.querySelector('[aria-invalid=true]');if(missing)missing.focus();return;}
        put.disabled=true;
        launching=true;cancel.disabled=true;
        try {
          if(execute){var result=await app().StartSkill(selected.id,selected.revision,workspace.value,provider.value,model.value.trim(),text);dialog.close();await dinahWorkbench.open({kind:'chat-session',workspace:result.workspace,path:result.sessionId});}
          else {var target=choices[Number(select.value)]; var prepared=await app().PrepareSkill(selected.id,selected.revision,target.workspace,text); await target.put(prepared); dialog.close();}
          status.textContent='';
        } catch(err) { note.textContent=String(err); put.disabled=false; }finally{launching=false;cancel.disabled=false;}
      });
      put.textContent=execute?'Run':'Add to Chat';put.classList.add('btn-primary');put.disabled=execute||!choices.length;
      dialog.addEventListener('keydown',function(event){if(event.key==='Enter'&&(event.target.tagName==='INPUT'||event.target.tagName==='TEXTAREA'&&event.ctrlKey)){event.preventDefault();if(!put.disabled)put.click();}});
      note.textContent=execute?'The skill runs in a new chat, which opens when it starts.':choices.length?'The prepared instructions will be added to the composer for review.':'Open a chat in the appropriate workspace first.';
      var cancel=button('Cancel',function () { dialog.close(); });dialog.addEventListener('cancel',function(event){if(launching)event.preventDefault();});actions.append(cancel,put); dialog.append(heading,element('p','',selected.name),parameters,preview);
      if(execute)dialog.append(labelled('Workspace',workspace),labelled('Provider',provider),labelled('Model (optional)',model));else dialog.append(labelled('Chat',select));
      dialog.append(note,actions); root.append(dialog); dialog.addEventListener('close',function () { launchDialog=null;dialog.remove(); }); dialog.showModal();
      if(execute)try{var state=await app().State();if(!dialog.isConnected)return;(state.workspaceRecords||[]).filter(function(record){return !selected.workspaceId||record.id===selected.workspaceId;}).forEach(function(record){var path=record.executionPath||(record.sources||[]).find(function(s){return s.id===record.primarySourceId;})?.path;if(path)workspace.append(new Option(record.name,path));});var chosen=selected.workspace||state.workspace;if(Array.from(workspace.options).some(function(option){return option.value===chosen;}))workspace.value=chosen;provider.value=Array.from(provider.options).some(function(o){return o.value===state.defaultProvider;})?state.defaultProvider:(provider.options[0]?provider.options[0].value:'');models();put.disabled=!workspace.options.length;if(put.disabled)note.textContent='Choose a registered workspace before running a skill.';}catch(err){note.textContent=String(err);}
    }
    async function load() {
      var request=++version; use.disabled=run.disabled=check.disabled=copy.disabled=rename.disabled=true; say('Loading Skill…');
      try {
        var next=await app().ReadSkill(item.path); if(disposed||request!==version)return; pack=next;
        title.textContent=pack.skill.name; description.textContent=pack.skill.description;
        if(!design)commandLine.textContent='/'+pack.skill.name+(pack.skill.hint?' '+pack.skill.hint:'');
        origin.textContent=pack.skill.origin+(pack.skill.ref?'':' · Read Only');
        tabButtons.edit.textContent=pack.skill.ref?'Edit':'Instructions';
        tabButtons.parameters.hidden=!pack.skill.ref; if(!pack.skill.ref&&activeTab==='parameters')tab('edit');
        if(window.pudlWindows&&!opts.embedded)pudlWindows.retitle(root.closest('[data-win]').dataset.win,pack.skill.name);
        if(pack.skill.ref){
          /* The editor stays open on its draft across a refresh; it is made
             once, and the form follows the draft's events. */
          if(!draft){
            draft=await app().OpenSkillDraft(item.path); if(disposed||request!==version)return;
            if(viewer){viewer.destroy();viewer=null;}
            editorHost.replaceChildren();
            editor=dinahWorkbench.embed({kind:'editor',workspace:draft.origin.workspaceId||'',path:draft.id,title:pack.skill.name},editorHost,{state:'split',changed:function(){}});
            await loadFields();
          }
        } else {
          if(editor){editor.destroy();editor=null;} draft=null; design=null; form.replaceChildren(); editorHost.replaceChildren();
          editorHost.append(element('p','',pack.skill.description||'This provider command has no retrievable instruction package.'));
        }
        if(filesViewer)filesViewer.destroy(); filesViewer=null; filesLoading=false; panels.files.replaceChildren();
        if(!pack.skill.ref)panels.files.append(element('p','form-help','This command has no accessible supporting files.'));
        else if(activeTab==='files')loadFiles();
        use.disabled=false;run.disabled=!pack.skill.ref;run.hidden=!pack.skill.ref; check.disabled=copy.disabled=rename.disabled=!pack.skill.ref; check.hidden=copy.hidden=rename.hidden=!pack.skill.ref; say('');
      } catch(err) { error(err); }
    }
    var stop=subscribe(function () { if(!pack)return; var current=catalog.skills.find(function (s) {return s.id===item.path;}); if(!current) {use.disabled=run.disabled=true;say('This skill is no longer available from its original collection.');} else {origin.textContent=current.origin+(current.ref?'':' · Read Only'); if(current.revision!==pack.skill.revision){use.disabled=run.disabled=true;say('This skill changed. Refresh to use or run its current instructions.');}} });
    var stopObserve=observe(root); tab(['edit','parameters','files'].includes(opts.state)?opts.state:'edit'); load();
    return {state:function(){return activeTab;},setState:tab,commands:function(){return [{label:'Refresh Skill',run:load},{label:'Check',disabled:check.disabled,run:function(){check.click();}},{label:'Run…',disabled:run.disabled,run:function(){launch(true);}},{label:'Use in Chat…',disabled:use.disabled,run:function(){launch(false);}},{label:'Copy to My Skills',disabled:copy.disabled,run:copyToMine},{label:'Rename…',disabled:rename.disabled,run:renameDialog}];},destroy:function(){disposed=true;stop();stopObserve();stopDrafts();stopResync();clearTimeout(fieldsTimer);if(launchDialog)launchDialog.close();if(viewer)viewer.destroy();if(editor)editor.destroy();if(filesViewer)filesViewer.destroy();}};
  }});
  if(window.dinahResources)dinahResources.subscribe(function(){refresh().catch(function(){});});
  window.dinahHost.on('agent:commands',function(){refresh().catch(function(){});});
  window.dinahHost.onResync(refresh);
})();
