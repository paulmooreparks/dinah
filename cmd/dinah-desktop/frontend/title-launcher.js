/* Dinah owns catalog destinations and saved preferences; PUDL owns presentation. */
(function () {
  'use strict';
  var root = document.getElementById('dinah-launcher'), started = false, restoring = false;
  var store = 'dinah-title-launcher', actions = new Map();
  function draw() {
    var list = root.querySelector('[data-start-items]');
    actions.clear(); list.replaceChildren();
    function add(id, title, category, glyph, run, disabled) {
      actions.set(id, run);
      var li = document.createElement('li'); li.className = 'md-row'; li.dataset.startItem = ''; li.dataset.startCategory = category;
      var button = document.createElement('button'); button.type = 'button'; button.className = 'menu-action md-item start-item'; button.dataset.launcherAction = id; button.disabled = !!disabled;
      var icon = document.createElement('span'); icon.className = 'glyph'; icon.style.setProperty('--glyph', 'var(--glyph-' + (glyph || 'window') + ')'); icon.setAttribute('aria-hidden', 'true');
      var name = document.createElement('span'); name.className = 'start-name'; name.textContent = title;
      button.append(icon, name); li.append(button);
      var pin = window.dinahTaskbarPins && dinahTaskbarPins.reference(id);
      if (pin) {
        li.dataset.winDestinationProvider = pin.provider; li.dataset.winPinTarget = pin.id;
        var more = document.createElement('button'); more.type = 'button'; more.dataset.rowActions = ''; more.setAttribute('aria-label','Actions for ' + title); li.append(more);
      }
      list.append(li);
    }
    [
      ['new-chat','chat','comments'], ['chat-browser','chat','comments'],
      ['skills-browser','tools','document'], ['in-progress','documents','pencil'], ['needs-you','tools','warning','Notifications'],
      ['show-workspace','workspace','folder'], ['new-workspace','workspace','folder'],
      ['new-empty-workspace','workspace','folder'], ['import-workspace','workspace','upload'],
      ['open-document','documents','open','Open Document…'],
      ['install-packages-file','tools','download'], ['find-tool','tools','search'],
      ['keyboard-shortcuts','help','info'], ['about','help','info']
    ].forEach(function (entry) {
      var cmd = dinahCommands.describe(entry[0],entry[3]); add(entry[0],cmd.label,entry[1],entry[2],cmd.run,cmd.disabled);
      list.lastElementChild.querySelector('button').dataset.launcherCommand = entry[0];
    });
    dinahWorkbench.launchers().filter(function (tool) { return tool.id !== 'terminal'; }).forEach(function (tool) { add('tool:'+tool.id,tool.title,tool.category || (tool.id === 'files' ? 'workspace' : 'tools'),tool.glyph,tool.run); });
    var terminal = dinahWorkbench.launchers().find(function (tool) { return tool.id === 'terminal'; });
    add('terminal:default','Default Shell','terminals','terminal',terminal ? terminal.run : function () {},!terminal);
    if (window.dinahShellTerminal) {
      dinahShellTerminal.shells().forEach(function (shell) {
        add('terminal:'+shell.id,shell.name,'terminals','terminal',function () {
          return dinahResources.currentPath().then(function (path) { return dinahResources.openTerminal(path,null,shell.id); });
        });
      });
      add('terminal:refresh','Refresh Shells','terminals','reload',dinahShellTerminal.refresh);
    }
    // The entries are in alphabetical order, whichever category is chosen (Paul, 9 November 2026).
    Array.from(list.children).sort(function (a, b) { return a.querySelector('.start-name').textContent.localeCompare(b.querySelector('.start-name').textContent, undefined, { sensitivity: 'base' }); }).forEach(function (li) { list.append(li); });
    if (started) pudlStart.refresh(root);
  }
  function save() {
    if (!started || restoring) return;
    var state = pudlLauncher.state(root), filter = pudlStart.state(root);
    dinahUIState.set(store, Object.assign({}, dinahUIState.get(store), { presentation: state.presentation, width:state.width, height:state.height }));
    var url = new URL(location.href);
    [['launcher',state.presentation],['launcher-query',filter.query],['launcher-category',filter.category]].forEach(function (entry) { if(entry[1]) url.searchParams.set(entry[0],entry[1]); else url.searchParams.delete(entry[0]); });
    history.replaceState(history.state,'',url);
    dinahUIState.set('dinah-workbench-layout',location.search);
    root.querySelectorAll('[data-start-category-key]').forEach(function (link) { var u=new URL(location.href);u.searchParams.set('launcher-category',link.dataset.startCategoryKey);link.href=u.pathname+u.search; });
  }
  function restore() {
    if (!started) return;
    restoring = true;
    var params = new URLSearchParams(location.search), saved = dinahUIState.get(store) || {};
    pudlStart.setState(root,{query:params.get('launcher-query')||'',category:params.get('launcher-category')||''});
    pudlLauncher.present(root,params.get('launcher')||saved.presentation||'closed',{focus:false});
    restoring = false;
  }
  root.addEventListener('click',function (event) {
    var button=event.target.closest('[data-launcher-action]'); if(!button||button.disabled) return;
    var run=actions.get(button.dataset.launcherAction); if(!run) return;
    event.preventDefault(); pudlLauncher.activated(root,button);
    Promise.resolve().then(run).then(function(){if(matchMedia('(max-width: 640px)').matches&&pudlLauncher.state(root).pinned)pudlLauncher.present(root,'collapsed',{focus:false});}).catch(function(error){document.dispatchEvent(new CustomEvent('dinah:menu-error',{detail:error}));});
  });
  root.addEventListener('keydown', function (event) {
    if (event.key !== 'Escape' || event.defaultPrevented || !started || !pudlLauncher.state(root).pinned) return;
    if (root.querySelector('[popover]:popover-open')) return;
    event.preventDefault(); pudlLauncher.close(root);
  });
  root.addEventListener('pudl:start-change',save); root.addEventListener('pudl:launcher-change',save);
  document.addEventListener('dinah:launchers-change',draw);
  document.addEventListener('dinah:commands-change',function () {
    root.querySelectorAll('[data-launcher-command]').forEach(function (button) { button.disabled = dinahCommands.describe(button.dataset.launcherCommand).disabled; });
  });
  window.addEventListener('popstate',restore);
  document.addEventListener('pudl:split', function (event) {
    var layout = event.target.parentElement;
    if (layout !== document.getElementById('launcher-workspace') && layout !== root.querySelector('.start-content')) return;
    var size = event.detail && event.detail.size; if (!Number.isFinite(size) || size <= 0) return;
    var saved = Object.assign({}, dinahUIState.get(store));
    saved[layout.id === 'launcher-workspace' ? 'paneWidth' : 'categoryWidth'] = size;
    dinahUIState.set(store, saved);
  });
  function init() {
    if (started) return;
    draw(); var saved=dinahUIState.get(store)||{};
    if (saved.paneWidth) document.getElementById('launcher-workspace').style.setProperty('--split-a', saved.paneWidth + 'px');
    if (saved.categoryWidth) root.querySelector('.start-content').style.setProperty('--split-a', saved.categoryWidth + 'px');
    pudlStart.enhance(root);
    started=pudlLauncher.enhance(root,{pane:document.getElementById('launcher-slot'),layout:document.getElementById('launcher-workspace'),invoker:function(){return pudlMenubar.launcherButton();},anchor:function(){return pudlMenubar.launcherButton({visible:true});},presentation:'closed',size:{width:saved.width||720,height:saved.height||600}});
    restore();
  }
  if(document.documentElement.dataset.uiStateReady==='true')init();else document.addEventListener('dinah:ui-state-restored',init,{once:true});
})();
