/* Each chat keeps its mention selection and the shared slash/@ chooser here. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, element: $, productName, sessions, normalizedWorkspace, title, workspaceLabel, typed, GROUPS, templateRows, insertTemplate, skillRows, insertSkill, skillForm, closeForm } = adapter;

    // The "/word" or "@word" the caret is in, if the mark starts the word.
    function wordToken(mark) {
      var box = $('prompt'), v = box.value, caret = box.selectionStart;
      if (caret !== box.selectionEnd) return null;
      var start = caret;
      while (start > 0 && !/\s/.test(v[start - 1])) start--;
      var word = v.slice(start, caret);
      if (word.charAt(0) !== mark) return null;
      var end = caret;
      while (end < v.length && !/\s/.test(v[end])) end++;
      return { start: start, end: end, query: word.slice(1).toLowerCase() };
    }
    function slashToken() { return wordToken('/'); }

    /* ---- @ mentions -------------------------------------------------------
       "@" offers windows, bundled applets, open documents and the other open
       chats. A selected applet supplies its verified identity to the agent;
       documents supply resource links. References go only while their
       selected token remains in the message. */
    var mentions = [];      // { ins, att } chosen for the message being typed
    var appletMentions = [], appletCatalogState = 'loading', appletCatalogJob = null;
    function refreshMentions() {
      if (adapter.deleted() || $('slash-pop').hidden || pop.mode !== 'mention' || !pop.tok) return;
      var tok = wordToken('@');
      if (tok && tok.start === pop.tok.start) showMentions(tok, pop.mentionScope);
    }
    function loadAppletMentions() {
      if (appletCatalogJob) return appletCatalogJob;
      appletCatalogState = 'loading';
      appletCatalogJob = Promise.resolve().then(function () {
        if (!app().AppletCatalog) return [];
        return app().AppletCatalog();
      }).then(function (catalog) {
        appletMentions = (catalog || []).filter(function (m) { return m.actions && m.actions.length; });
        appletCatalogState = 'ready';
      }).catch(function () {
        appletMentions = [];
        appletCatalogState = 'failed';
      }).finally(function () {
        appletCatalogJob = null;
        if (adapter.deleted() || $('slash-pop').hidden || pop.mode !== 'mention' || !pop.tok) return;
        var tok = wordToken('@');
        if (tok && tok.start === pop.tok.start) showMentions(tok, pop.mentionScope);
      });
      return appletCatalogJob;
    }
    /* How well a mention matches what was typed: its name beginning with it,
       then its name containing it, then its ID, then only its description or
       its actions; -1 when nothing does. Better matches are listed first. */
    function mentionRank(q, name, id, rest) {
      if (!q) return 0;
      var n = name.toLowerCase();
      if (n.indexOf(q) === 0 || n.replace(/\s+/g, '_').indexOf(q) === 0) return 0;
      if (n.indexOf(q) >= 0 || n.replace(/\s+/g, '_').indexOf(q) >= 0) return 1;
      if (id.toLowerCase().indexOf(q) >= 0) return 2;
      return rest.toLowerCase().indexOf(q) >= 0 ? 3 : -1;
    }
    function mentionItems(q, scope) {
      var out = [], ranked = [];
      /* The tool windows, which a chat drives through their commands, with
         the applet of the same tool's data when it has one; that applet is
         then not listed again on its own. */
      var tools = window.dinahWorkbench ? window.dinahWorkbench.launchers().filter(function (t) { return /^[a-z][a-z0-9-]{0,63}$/.test(t.id); }) : [];
      var covered = {};
      tools.forEach(function (t) {
        var name = t.windowTitle || t.title, applet = t.applet && appletMentions.filter(function (m) { return m.id === t.applet; })[0];
        if (applet) covered[applet.id] = true;
        var rank = mentionRank(q, name, t.id, [t.description || '', applet ? applet.description : ''].join(' '));
        if (rank < 0) return;
        ranked.push({ rank: rank, item: { group: 'Windows', label: name, meta: t.description || 'A ' + productName() + ' window', ins: '@' + name.replace(/\s+/g, '_'), att: { kind: 'window', window: t.id, provider: applet ? applet.id : '', name: name } } });
      });
      appletMentions.forEach(function (m) {
        if (covered[m.id]) return;
        var token = '@' + m.title.replace(/\s+/g, '_');
        if (appletMentions.some(function (other) { return other.id !== m.id && other.title.replace(/\s+/g, '_') === token.slice(1); })) token += ':' + m.id;
        var rest = [m.description, token].concat(m.actions.map(function (action) { return action.title + ' ' + action.ref + ' ' + action.description; })).join(' ');
        var rank = mentionRank(q, m.title, m.id, rest);
        if (rank < 0) return;
        ranked.push({ rank: rank, item: { group: 'Applets', label: m.title, meta: m.description || m.actions.map(function (action) { return action.title; }).join(' · '), ins: token, att: { kind: 'applet', provider: m.id, name: m.title } } });
      });
      // Each group once: the group with the best match first, and in each
      // group the better matches first.
      var best = {};
      ranked.forEach(function (r) { var g = r.item.group; best[g] = best[g] === undefined ? r.rank : Math.min(best[g], r.rank); });
      ranked.sort(function (a, b) { var ga = a.item.group, gb = b.item.group; return best[ga] - best[gb] || (ga === gb ? 0 : ga === 'Windows' ? -1 : 1) || a.rank - b.rank; }).forEach(function (r) { out.push(r.item); });
      if (scope === 'applets') return out;
      // Documents open in the Editor, the one most recently in front first.
      ((window.dinahDocuments && window.dinahDocuments.open) ? window.dinahDocuments.open() : []).forEach(function (d) {
        if (q && d.title.toLowerCase().indexOf(q.replace(/_/g, ' ')) < 0) return;
        out.push({ group: 'Open Documents', label: d.title, meta: 'Open in the Editor', ins: '@' + d.title.replace(/\s+/g, '_'), att: { kind: 'draft', name: d.title, draftId: d.draftId } });
      });
      /* Open chats across workspaces, with this chat's workspace first and
         workspace labels to distinguish names. A
         chat whose window was restored with the layout counts as open; it is
         connected to its agent when the message is sent. */
      var asking = adapter.active() && sessions[adapter.active()], home = normalizedWorkspace(asking && asking.info.cwd || adapter.workspace());
      var chats = Object.keys(sessions).map(function (k) { return sessions[k]; }).filter(function (s) {
        return s.id !== adapter.active() && (s.info.open || (s.saved && s.view)) && !s.info.continuedBy && s.info.cwd;
      }).sort(function (a, b) { return Number(normalizedWorkspace(b.info.cwd) === home) - Number(normalizedWorkspace(a.info.cwd) === home) || String(b.info.updatedAt || '').localeCompare(String(a.info.updatedAt || '')); });
      chats.forEach(function (s) {
        var name = title(s), place = workspaceLabel(s.info.cwd), searchable = (name + ' ' + place).toLowerCase();
        if (q && searchable.indexOf(q.replace(/_/g, ' ')) < 0 && searchable.replace(/\s+/g, '_').indexOf(q) < 0) return;
        var token = '@' + name.replace(/\s+/g, '_');
        if (chats.some(function (o) { return o !== s && title(o).toLowerCase() === name.toLowerCase(); })) token += ':' + encodeURIComponent(s.id);
        out.push({ group: 'Open Chats', label: name, meta: place + (window.dinahChatState.read(s.info).window === false ? ' · No window' : ''), state: s.info, ins: token, att: { kind: 'chat', sessionId: s.id, name: name } });
      });
      return out.slice(0, 60);
    }

    var pop = { tok: null, items: [], sel: 0, mode: 'skill', mentionScope: 'all' };
    function showMentions(tok, scope) {
      var previous = pop.mode === 'mention' && pop.tok && pop.tok.start === tok.start ? pop.items[pop.sel] : null;
      var items = mentionItems(tok.query, scope);
      pop.mode = 'mention';
      pop.mentionScope = scope || 'all';
      pop.tok = tok;
      pop.items = items;
      var kept = previous ? items.findIndex(function (it) { return it.ins === previous.ins && it.att.kind === previous.att.kind && it.att.provider === previous.att.provider; }) : -1;
      pop.sel = Math.max(0, kept);
      if (!items.length) {
        pop.items = [];
        var host = $('slash-pop');
        var note = document.createElement('p');
        note.className = 'form-help menu-note';
        note.textContent = appletCatalogState === 'loading' ? 'Loading applets…'
          : appletCatalogState === 'failed' ? 'Applet actions could not be loaded. Choose Add > Windows and Applets to retry.'
          : scope === 'applets' ? 'No matching applets are available.'
          : 'Nothing matches. Search for a window, an applet, an open document or another chat.';
        host.replaceChildren(note);
        host.hidden = false;
        return;
      }
      drawPop();
    }
    function pickMention(it, tok) {
      var box = $('prompt'), v = box.value;
      box.value = v.slice(0, tok.start) + it.ins + ' ' + v.slice(tok.end).replace(/^\s/, '');
      var at = tok.start + it.ins.length + 1;
      box.setSelectionRange(at, at);
      mentions = mentions.filter(function (m) { return m.ins !== it.ins; }).concat([{ ins: it.ins, att: it.att }]);
      hidePop();
      typed();
      box.focus();
    }

    function showPop(tok) {
      pop.mode = 'skill';
      var q = tok.query;
      var list = adapter.skills().filter(function (s) { return !q || s.name.toLowerCase().indexOf(q) >= 0; });
      list.sort(function (a, b) {
        var ga = GROUPS.findIndex(function (g) { return g[0] === a.source; }), gb = GROUPS.findIndex(function (g) { return g[0] === b.source; });
        var pa = a.name.toLowerCase().indexOf(q) === 0 ? 0 : 1, pb = b.name.toLowerCase().indexOf(q) === 0 ? 0 : 1;
        return ga - gb || pa - pb || a.name.localeCompare(b.name);
      });
      var tpls = adapter.templates().filter(function (t) { return !q || t.name.toLowerCase().indexOf(q) >= 0; });
      if (!list.length && !tpls.length) { hidePop(); return; }
      pop.tok = tok;
      // Templates are told from skills by a prefix, since a template may
      // share a skill's name.
      var flat = tpls.map(function (t) { return 'tpl:' + t.name; });
      GROUPS.forEach(function (g) { list.forEach(function (s) { if (s.source === g[0]) flat.push(s.name); }); });
      var keep = pop.items[pop.sel];
      pop.items = flat;
      pop.sel = Math.max(0, flat.indexOf(keep));
      drawPop();
    }
    function drawPop() {
      var host = $('slash-pop');
      host.innerHTML = '';
      if (pop.mode === 'mention') {
        var group = '';
        pop.items.forEach(function (it, n) {
          if (it.group !== group) {
            group = it.group;
            var h = document.createElement('div');
            h.className = 'md-section-label';
            h.textContent = group;
            host.appendChild(h);
          }
          var r = document.createElement('div');
          r.className = 'md-row skill-row mention-row';
          r.setAttribute('role', 'option');
          if (n === pop.sel) r.setAttribute('aria-selected', 'true');
          r.innerHTML = '<a class="md-item" href="#"><span class="mention-name"></span><span class="md-meta"></span></a>';
          r.querySelector('.mention-name').textContent = it.label;
          if (it.state) r.firstChild.appendChild(window.dinahChatState.badge(it.state));
          r.querySelector('.md-meta').textContent = it.meta;
          r.firstChild.addEventListener('mousedown', function (e) { e.preventDefault(); });
          r.firstChild.addEventListener('click', function (e) { e.preventDefault(); pickMention(it, pop.tok); });
          host.appendChild(r);
        });
        // The windows are listed while the applets are still loading, or
        // could not be; that is said under them.
        if (appletCatalogState !== 'ready') {
          var note = document.createElement('p');
          note.className = 'form-help menu-note';
          note.textContent = appletCatalogState === 'loading' ? 'Loading applets…' : 'Applet actions could not be loaded. Choose Add > Windows and Applets to retry.';
          host.appendChild(note);
        }
      } else {
        templateRows(host, adapter.templates().filter(function (t) { return pop.items.indexOf('tpl:' + t.name) >= 0; }), function (name) { insertTemplate(name, pop.tok); }, pop.items[pop.sel]);
        skillRows(host, adapter.skills().filter(function (s) { return pop.items.indexOf(s.name) >= 0; }), function (name) { insertSkill(name, pop.tok); }, pop.items[pop.sel]);
      }
      var hint = document.createElement('div');
      hint.className = 'slash-hint';
      hint.textContent = pop.mode === 'mention'
        ? 'Enter or Tab adds · Up and Down choose · Esc closes'
        : 'Enter or Tab inserts · Up and Down choose · Esc closes · type \\/ for a plain slash';
      host.appendChild(hint);
      host.hidden = false;
      var cur = host.querySelector('[aria-selected="true"]');
      if (cur) cur.scrollIntoView({ block: 'nearest' });
    }
    function hidePop() { $('slash-pop').hidden = true; pop.tok = null; }
    $('prompt').addEventListener('input', function () {
      var tok = slashToken(), at = !tok && wordToken('@');
      if (tok) showPop(tok);
      else if (at) showMentions(at, !$('slash-pop').hidden && pop.mode === 'mention' && pop.tok && pop.tok.start === at.start ? pop.mentionScope : 'all');
      else hidePop();
    });
    $('prompt').addEventListener('blur', function () { setTimeout(function () { if (document.activeElement !== $('prompt')) hidePop(); }, 150); });
    // Registered on the box for the capturing phase, so it runs before the
    // box's other key handlers while the pop-up is open.
    $('prompt').addEventListener('keydown', function (e) {
      if (skillForm.isOpen() && e.key === 'Escape' && $('slash-pop').hidden) {
        e.preventDefault();
        e.stopImmediatePropagation();
        closeForm(true);
        return;
      }
      if ($('slash-pop').hidden || !pop.tok) return;
      var n = pop.items.length;
      if (!n && e.key !== 'Escape') return;
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        pop.sel = (pop.sel + (e.key === 'ArrowDown' ? 1 : n - 1)) % n;
        drawPop();
      } else if ((e.key === 'Enter' && !e.ctrlKey && !e.metaKey) || e.key === 'Tab') {
        var it = pop.items[pop.sel];
        if (pop.mode === 'mention') pickMention(it, pop.tok);
        else if (it.indexOf('tpl:') === 0) insertTemplate(it.slice(4), pop.tok);
        else insertSkill(it, pop.tok);
      } else if (e.key === 'Escape') {
        hidePop();
      } else {
        return;
      }
      e.preventDefault();
      e.stopImmediatePropagation();
    }, true);

    return { refreshMentions, loadAppletMentions, showMentions, wordToken, hidePop,
      selected: () => mentions, setSelected: value => { mentions = value; },
      catalogFailed: () => appletCatalogState === 'failed'
    };
  }

  window.dinahChatMentions = { create };
})();
