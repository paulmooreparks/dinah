/* A chat's subagents (subagents.go): the workers its agent starts while it
   handles a prompt, as the ACP subagent RFD reports them. Three kinds of
   window show them. "subagents" is the tree of the subagents a chat started
   in this run, since Dinah started, and theirs, each with its state, what
   it was asked to do, how long it ran and Open. "subagent-archive" is the
   chat's Earlier Subagents, those its history brought back from earlier
   runs, newest first, which Paul decided on 28 October 2026 should be kept
   out of the tree and the Subagents menu. "subagent" shows one subagent's
   work as it happens: its messages, thoughts, plan and tool calls, drawn
   with the chat's own Markdown and row styles. A subagent takes no prompt
   from the person, so no window has a prompt box; the chat's Go side
   refuses any.

   The chat page (app.js) keeps each chat's subagents and gives them here
   through window.dinahChatSubagents. */
(function () {
  'use strict';
  const app = window.dinahHelpers.app;
  // The product's name as people see it (product.js).
  function productName() { return window.dinahProduct ? window.dinahProduct.name : ''; }
  function host() { return window.dinahHost; }
  function chats() { return window.dinahChatSubagents; }

  /* Each state is a glyph and a word, as PUDL draws status. Disconnected
     says that the outcome is unknown, so it must look like neither a
     failure nor a cancellation. glyph is the glyph the badge's class draws,
     and tone the status colour of a command for the state, so that a menu
     row shows the same glyph as the badge (app.js, the chat's Subagents
     menu). */
  var STATES = {
    running: { kind: 'accent', word: 'Running', glyph: 'diamond' },
    completed: { kind: 'positive', word: 'Completed', glyph: 'circle', tone: 'positive' },
    failed: { kind: 'danger', word: 'Failed', glyph: 'square', tone: 'danger' },
    cancelled: { kind: 'subagent-cancelled', word: 'Cancelled', glyph: 'stop' },
    disconnected: { kind: 'subagent-unknown', word: 'Disconnected', glyph: 'ring' }
  };
  function stateOf(sub) { return STATES[sub.state] || { kind: '', word: sub.state || 'Unknown', glyph: 'ring' }; }
  function stateTitle(sub) {
    if (sub.state === 'disconnected') return sub.local
      ? 'The chat\'s turn or its agent ended before the agent said how this subagent ended, so its outcome is unknown.'
      : 'The agent could no longer follow this subagent, so its outcome is unknown.';
    if (sub.state === 'running') return 'The subagent is working.';
    return 'The agent reported that the subagent ' + (STATES[sub.state] || { word: sub.state }).word.toLowerCase() + '.';
  }
  function badge(sub, el) {
    var st = stateOf(sub);
    el = el || document.createElement('span');
    el.className = 'badge subagent-state ' + st.kind;
    el.textContent = st.word;
    el.title = stateTitle(sub);
    el.dataset.state = sub.state || '';
    return el;
  }

  function duration(sec) { return window.dinahHelpers.duration(sec, 0); }
  /* How long a subagent ran, or has run so far. One replayed with its
     chat's history has no times. */
  function ran(sub) {
    if (sub.replayed || !sub.started) return 'Earlier';
    var start = Date.parse(sub.started), end = sub.ended ? Date.parse(sub.ended) : Date.now();
    if (!isFinite(start) || !isFinite(end)) return '';
    return duration(Math.round((end - start) / 1000));
  }

  /* The children of each parent, in the order they began. */
  function tree(list, rootId) {
    var known = {}, children = {};
    list.forEach(function (sub) { known[sub.id] = true; });
    list.forEach(function (sub) {
      var parent = known[sub.parentId] ? sub.parentId : rootId;
      (children[parent] = children[parent] || []).push(sub);
    });
    return children;
  }

  function workspaceOf(rootId, ws) { return ws || (chats() && chats().workspace(rootId)) || ''; }
  function openTree(rootId, ws) {
    return window.dinahWorkbench.open({ kind: 'subagents', workspace: workspaceOf(rootId, ws), path: rootId, title: 'Subagents' });
  }
  function openArchive(rootId, ws) {
    return window.dinahWorkbench.open({ kind: 'subagent-archive', workspace: workspaceOf(rootId, ws), path: rootId, title: 'Earlier Subagents' });
  }
  function openOne(rootId, id, ws, name) {
    return window.dinahWorkbench.open({ kind: 'subagent', workspace: workspaceOf(rootId, ws), path: JSON.stringify([rootId, id]), title: name || 'Subagent' });
  }
  function openButton(rootId, sub, ws) {
    var b = document.createElement('button');
    b.type = 'button'; b.className = 'btn btn-sm subagent-open'; b.textContent = 'Open';
    b.setAttribute('aria-label', 'Open the subagent ' + (sub.name || ''));
    b.addEventListener('click', function () { openOne(rootId, sub.id, ws, sub.name).catch(host().fail); });
    return b;
  }
  function stopButton(sub) {
    var b = document.createElement('button');
    b.type = 'button'; b.className = 'btn btn-sm btn-danger subagent-stop'; b.textContent = 'Stop';
    b.setAttribute('aria-label', 'Stop the subagent ' + (sub.name || ''));
    b.addEventListener('click', function () {
      b.disabled = true;
      Promise.resolve().then(function () { return app().CancelSubagent(sub.id); }).catch(function (err) { b.disabled = false; host().fail(err); });
    });
    return b;
  }

  /* ---- The tree of a chat's subagents ----------------------------------- */

  function initTree(root, opts, item) {
    var win = root.closest('[data-win]'), key = opts.key || (win && win.dataset.win);
    var rootId = item.path, ws = item.workspace, off = [], timer = 0, disposed = false;
    root.classList.add('subagents-applet');
    root.innerHTML = '<p class="doc-meta subagents-meta" data-sa-meta></p>' +
      '<div class="subagents-body" data-sa-body><ul class="subagent-tree" data-sa-tree aria-label="Subagents"></ul>' +
      '<div class="empty-state" data-sa-empty hidden><p class="empty-state-title" data-sa-empty-title></p>' +
      '<p class="empty-state-body">When the chat\'s agent hands work to a subagent, it appears here with what it was asked to do.</p></div>' +
      '<p class="subagents-foot" data-sa-foot hidden><button type="button" class="btn btn-sm" data-sa-earlier>Earlier Subagents…</button></p></div>';
    var $ = function (name) { return root.querySelector('[data-sa-' + name + ']'); };
    function earlier() { return chats() && chats().earlier ? chats().earlier(rootId) : []; }
    $('earlier').addEventListener('click', function () { openArchive(rootId, ws).catch(host().fail); });
    function node(sub, children) {
      var li = document.createElement('li');
      li.className = 'subagent-node';
      li.dataset.subagent = sub.id;
      var line = document.createElement('div');
      line.className = 'subagent-line';
      var words = document.createElement('span');
      words.className = 'subagent-words';
      var name = document.createElement('strong'); name.className = 'subagent-name'; name.textContent = sub.name || 'Subagent';
      var task = document.createElement('span'); task.className = 'subagent-task'; task.textContent = sub.task || '';
      task.title = sub.task || '';
      words.append(name, task);
      var time = document.createElement('span'); time.className = 'subagent-ran'; time.textContent = ran(sub);
      time._sub = sub;
      line.append(badge(sub), words, time);
      if (sub.canCancel && sub.state === 'running') line.appendChild(stopButton(sub));
      line.appendChild(openButton(rootId, sub, ws));
      li.appendChild(line);
      var kids = children[sub.id];
      if (kids && kids.length) {
        var ul = document.createElement('ul');
        ul.setAttribute('aria-label', 'Subagents of ' + (sub.name || 'the subagent'));
        kids.forEach(function (k) { ul.appendChild(node(k, children)); });
        li.appendChild(ul);
      }
      return li;
    }
    function draw() {
      if (disposed) return;
      var list = chats() ? chats().list(rootId) : [], children = tree(list, rootId);
      var running = list.filter(function (s) { return s.state === 'running'; }).length;
      var title = chats() ? chats().title(rootId) : 'the chat';
      if (!opts.embedded && key && window.pudlWindows) window.pudlWindows.retitle(key, 'Subagents', title);
      var before = earlier().length;
      $('meta').textContent = !list.length ? 'The subagents of "' + title + '" in this run.'
        : '"' + title + '" has started ' + (list.length === 1 ? '1 subagent' : list.length + ' subagents') + ' in this run' + (running ? ', and ' + running + ' ' + (running === 1 ? 'is' : 'are') + ' running.' : '.');
      $('tree').replaceChildren.apply($('tree'), (children[rootId] || []).map(function (sub) { return node(sub, children); }));
      $('empty').hidden = list.length > 0;
      $('empty-title').textContent = before ? 'No Subagents in This Run' : 'No Subagents Yet';
      // The archive of the chat's earlier subagents is offered only while
      // it has some.
      $('foot').hidden = !before;
      clearInterval(timer);
      timer = running ? setInterval(tick, 1000) : 0;
    }
    function tick() { root.querySelectorAll('.subagent-ran').forEach(function (el) { if (el._sub && el._sub.state === 'running') el.textContent = ran(el._sub); }); }
    off.push(host().on('agent:subagent', function (m) { if (m && m.sessionId === rootId) draw(); }));
    off.push(host().on('agent:session', function (info) { if (info && info.sessionId === rootId) draw(); }));
    draw();
    return {
      commands: function () {
        var out = [{ label: 'Open the Chat', run: function () { return chats().openChat(rootId); } }];
        if (earlier().length) out.push({ label: 'Earlier Subagents…', run: function () { return openArchive(rootId, ws); } });
        return out;
      },
      destroy: function () { disposed = true; clearInterval(timer); off.forEach(function (fn) { if (fn) fn(); }); }
    };
  }

  /* ---- A chat's earlier subagents --------------------------------------- */

  /* The archive of the subagents a chat's history brought back from earlier
     runs, newest first: each with its state, what it was asked to do on two
     lines at most, when it ran and Open, which opens its window read-only
     as Open in the tree does. A subagent of another lists the one it was
     under. When it ran is the time on its row in the chat (app.js,
     subagentRowTime), since the agent replays a subagent without its times;
     a row no longer in the page leaves it unknown. */
  function initArchive(root, opts, item) {
    var win = root.closest('[data-win]'), key = opts.key || (win && win.dataset.win);
    var rootId = item.path, ws = item.workspace, off = [], disposed = false;
    root.classList.add('subagents-applet', 'subagent-archive-applet');
    root.innerHTML = '<p class="doc-meta subagents-meta" data-sa-meta></p>' +
      '<div class="subagents-body" data-sa-body><ol class="subagent-tree subagent-archive" data-sa-list aria-label="Earlier Subagents"></ol>' +
      '<div class="empty-state" data-sa-empty hidden><p class="empty-state-title">No Earlier Subagents</p>' +
      '<p class="empty-state-body" data-sa-empty-body></p>' +
      '<div class="empty-state-actions"><button type="button" class="btn btn-sm" data-sa-open-chat>Open the Chat</button></div></div></div>';
    var $ = function (name) { return root.querySelector('[data-sa-' + name + ']'); };
    $('open-chat').addEventListener('click', function () { Promise.resolve(chats() && chats().openChat(rootId)).catch(host().fail); });
    function when(sub) {
      var t = document.createElement('time');
      t.className = 'subagent-ran';
      var at = sub.started || (chats() && chats().when ? chats().when(rootId, sub.id) : '');
      if (at && window.dinahRegional) window.dinahRegional.stamp(t, at, 'dayTime');
      else { t.textContent = 'Earlier Run'; t.title = 'The agent did not say when this subagent ran, and its row in the chat is no longer shown.'; }
      return t;
    }
    function draw() {
      if (disposed) return;
      var list = chats() && chats().earlier ? chats().earlier(rootId) : [], names = {};
      var title = chats() ? chats().title(rootId) : 'the chat';
      list.forEach(function (sub) { names[sub.id] = sub.name || 'Subagent'; });
      if (!opts.embedded && key && window.pudlWindows) window.pudlWindows.retitle(key, 'Earlier Subagents', title);
      $('meta').textContent = !list.length ? 'The subagents of "' + title + '" from earlier runs.'
        : '"' + title + '" has ' + (list.length === 1 ? '1 subagent' : list.length + ' subagents') + ' from earlier runs, newest first.';
      $('list').replaceChildren.apply($('list'), list.slice().reverse().map(function (sub) {
        var li = document.createElement('li');
        li.className = 'subagent-node';
        li.dataset.subagent = sub.id;
        var line = document.createElement('div');
        line.className = 'subagent-line';
        var words = document.createElement('span');
        words.className = 'subagent-words';
        var name = document.createElement('strong'); name.className = 'subagent-name'; name.textContent = sub.name || 'Subagent';
        if (names[sub.parentId]) {
          var under = document.createElement('span'); under.className = 'subagent-under'; under.textContent = ', under ' + names[sub.parentId];
          name.appendChild(under);
        }
        var task = document.createElement('span'); task.className = 'subagent-task'; task.textContent = sub.task || ''; task.title = sub.task || '';
        words.append(name, task);
        line.append(badge(sub), words, when(sub), openButton(rootId, sub, ws));
        li.appendChild(line);
        return li;
      }));
      $('empty').hidden = list.length > 0;
      $('empty-body').textContent = 'This chat has no subagents from earlier runs that ' + productName() + ' knows of. Its agent sends them when the chat is opened.';
    }
    off.push(host().on('agent:subagent', function (m) { if (m && m.sessionId === rootId) draw(); }));
    off.push(host().on('agent:session', function (info) { if (info && info.sessionId === rootId) draw(); }));
    function stamped(e) { if (e.detail && e.detail.sessionId === rootId) draw(); }
    document.addEventListener('dinah:chat-times', stamped);
    document.addEventListener('dinah:locale', draw);
    off.push(function () { document.removeEventListener('dinah:chat-times', stamped); document.removeEventListener('dinah:locale', draw); });
    draw();
    return {
      commands: function () { return [{ label: 'Show Subagents', run: function () { return openTree(rootId, ws); } }, { label: 'Open the Chat', run: function () { return chats().openChat(rootId); } }]; },
      destroy: function () { disposed = true; off.forEach(function (fn) { if (fn) fn(); }); }
    };
  }

  /* ---- One subagent's work ---------------------------------------------- */

  var ROWS_MAX = 600, ROWS_KEEP = 450, TOOL_TEXT_MAX = 32 * 1024;
  var TOOL_STATES = { pending: ['', 'waiting'], in_progress: ['accent running', 'running'], completed: ['positive', 'done'], failed: ['danger', 'failed'] };
  function plainText(content) {
    var parts = [];
    (content || []).forEach(function (c) {
      if (c.type === 'diff') parts.push('Changed ' + c.path);
      else if (c.type === 'content' && c.content) {
        if (c.content.type === 'text') parts.push(c.content.text);
        else if (c.content.type === 'resource' && c.content.resource && c.content.resource.text) parts.push(c.content.resource.text);
      } else if (c.type === 'terminal') parts.push('Terminal ' + c.terminalId);
    });
    // A command's colours and other terminal controls are left out here.
    return parts.join('\n').replace(/\x1b\[[0-9;?]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|[\x00-\x08\x0b-\x1f\x7f]/g, '');
  }

  function initOne(root, opts, item) {
    var win = root.closest('[data-win]'), key = opts.key || (win && win.dataset.win);
    var ids = [];
    try { ids = JSON.parse(item.path); } catch (err) { ids = []; }
    var rootId = ids[0] || '', id = ids[1] || '', ws = item.workspace;
    var off = [], disposed = false, loaded = false, seq = 0, pending = [], sub = null, prompt = '';
    var rows = {}, current = null, dirty = new Set(), renderKey = {};
    root.classList.add('subagent-applet');
    root.innerHTML = '<div class="subagent-head" data-sa-head>' +
        '<p class="subagent-head-line"><span data-sa-state></span><strong class="subagent-head-name" data-sa-name></strong><span class="subagent-head-ran" data-sa-ran></span></p>' +
        '<p class="subagent-head-task" data-sa-task></p>' +
        '<details class="subagent-prompt" data-sa-prompt hidden><summary>Prompt</summary><pre data-sa-prompt-text></pre></details>' +
        '<p class="form-help subagent-note" data-sa-note hidden></p></div>' +
      '<div class="subagent-body" data-sa-body><ol class="activity run-log subagent-log" data-sa-log aria-live="polite" aria-label="The subagent\'s work"></ol>' +
      '<div class="empty-state" data-sa-empty hidden><p class="empty-state-title" data-sa-empty-title></p><p class="empty-state-body" data-sa-empty-body></p></div></div>';
    var $ = function (name) { return root.querySelector('[data-sa-' + name + ']'); };
    var log = $('log'), body = $('body');

    function drawHead() {
      if (!sub) return;
      badge(sub, $('state'));
      $('name').textContent = sub.name || 'Subagent';
      $('task').textContent = sub.task || '';
      $('task').title = sub.task || '';
      $('ran').textContent = ran(sub);
      $('prompt').hidden = !prompt;
      $('prompt-text').textContent = prompt;
      var notes = [];
      if (sub.trimmed) notes.push('Only this subagent\'s latest work is kept, so its earliest steps are not shown.');
      if (sub.state === 'disconnected') notes.push(stateTitle(sub));
      $('note').hidden = !notes.length;
      $('note').textContent = notes.join(' ');
      if (!opts.embedded && key && window.pudlWindows) window.pudlWindows.retitle(key, sub.name || 'Subagent', chats() ? chats().title(rootId) : '');
      if (window.pudlCommands) window.pudlCommands.changed();
    }
    function empty(title, text) {
      $('empty').hidden = !title;
      $('empty-title').textContent = title || '';
      $('empty-body').textContent = text || '';
    }

    /* The rows are the chat's: a time, then a message, a thought, a plan,
       a tool call or a subagent of this one. */
    function row(cls) {
      var li = document.createElement('li');
      li.className = 'activity-row ' + cls;
      var t = document.createElement('time'); t.className = 'activity-time';
      if (loaded) dinahRegional.stamp(t, new Date(), 'clock');
      li.appendChild(t);
      log.appendChild(li);
      while (log.children.length > ROWS_MAX) {
        for (var n = log.children.length - ROWS_KEEP; n > 0; n--) {
          var gone = log.firstElementChild;
          Object.keys(rows).forEach(function (k) { if (rows[k].li === gone) delete rows[k]; });
          if (current && current.li === gone) current = null;
          gone.remove();
        }
      }
      empty('');
      return li;
    }
    function chunk(kind, text) {
      if (!current || current.kind !== kind) {
        var li = row(kind === 'thought' ? 'thought-row' : 'msg-row'), el;
        if (kind === 'thought') { el = document.createElement('span'); el.className = 'thought'; li.appendChild(el); }
        else {
          var div = document.createElement('div'); div.className = 'msg';
          div.innerHTML = '<div class="msg-head"><span class="msg-label"></span></div><div class="msg-text"></div>';
          div.querySelector('.msg-label').textContent = kind === 'you' ? 'Prompt' : (sub && sub.name) || 'Subagent';
          li.appendChild(div); el = div.querySelector('.msg-text');
        }
        current = { kind: kind, li: li, el: el, text: '' };
      }
      current.text += text;
      dirty.add(current);
    }
    function tool(u) {
      var r = rows[u.toolCallId];
      var t = plainText(u.content);
      /* As in the chat (app.js, toolShows), an update for a call not yet
         seen that gives no title, kind, output, command or failure draws no
         row, so the call that starts a subagent of this one leaves no empty
         row reading "waiting" under the subagent's row. */
      if (!r && !(u.title || u.kind || t || (u.rawInput && u.rawInput.command) || u.status === 'failed')) return;
      if (!r) {
        current = null;
        var li = row('tool-row');
        li.insertAdjacentHTML('beforeend', '<span><span class="tool-title"></span><span class="badge"></span></span><details class="tool-detail" hidden><summary>Details</summary><pre></pre></details>');
        r = rows[u.toolCallId] = { li: li, title: li.querySelector('.tool-title'), badge: li.querySelector('.badge'), detail: li.querySelector('.tool-detail'), pre: li.querySelector('pre'), name: '', text: '', status: 'pending', tool: true };
      }
      if (u.title) r.name = u.title;
      if (u.status) r.status = u.status;
      if (u.rawInput && u.rawInput.command) r.command = u.rawInput.command;
      if (!t && r.command && !r.text) t = r.command;
      if (t && !r.cut) {
        r.text = r.text ? r.text + '\n' + t : t;
        if (r.text.length > TOOL_TEXT_MAX) { r.text = r.text.slice(0, TOOL_TEXT_MAX) + '\n\n[The rest of this output is not shown.]'; r.cut = true; }
      }
      dirty.add(r);
    }
    function plan(entries) {
      var r = rows.__plan;
      if (!r) {
        var li = row('plan-row');
        li.insertAdjacentHTML('beforeend', '<span><strong>Plan</strong><ol class="plan"></ol></span>');
        r = rows.__plan = { li: li, list: li.querySelector('.plan'), plan: true };
      }
      r.entries = entries || [];
      dirty.add(r);
    }
    /* A subagent of this one: its row, kept up to date by the chat's list. */
    function child(u) {
      current = null;
      var li = row('subagent-row');
      li.dataset.subagent = u.subagentSessionId;
      var known = (chats() ? chats().all(rootId) : []).filter(function (s) { return s.id === u.subagentSessionId; })[0];
      drawChildRow(li, known || { id: u.subagentSessionId, name: u.name, task: u.task, state: 'running' });
    }
    function drawChildRow(li, s) {
      var body = li.querySelector('.subagent-row-body');
      if (!body) {
        body = document.createElement('span'); body.className = 'subagent-row-body';
        body.innerHTML = '<span class="subagent-row-head"><strong class="subagent-row-name"></strong><span class="subagent-row-state"></span></span><span class="subagent-row-task"></span>';
        li.append(body, openButton(rootId, s, ws));
      }
      body.querySelector('.subagent-row-name').textContent = 'Subagent ' + (s.name || '');
      body.querySelector('.subagent-row-task').textContent = s.task || '';
      body.querySelector('.subagent-row-task').title = s.task || '';
      var st = body.querySelector('.subagent-row-state');
      st.replaceChildren(badge(s));
    }
    function feed(u) {
      switch (u && u.sessionUpdate) {
        case 'agent_message_chunk': if (u.content && u.content.type === 'text') chunk('agent', u.content.text); break;
        case 'agent_thought_chunk': if (u.content && u.content.type === 'text') chunk('thought', u.content.text); break;
        case 'user_message_chunk': if (u.content && u.content.type === 'text') chunk('you', u.content.text); break;
        case 'tool_call': case 'tool_call_update': tool(u); break;
        case 'plan': plan(u.entries); break;
        case 'subagent_spawned': child(u); break;
      }
    }
    function draw(entry) {
      if (entry.tool) {
        var st = TOOL_STATES[entry.status] || ['', entry.status];
        entry.title.textContent = entry.name;
        entry.badge.className = 'badge ' + st[0];
        entry.badge.textContent = st[1];
        entry.pre.textContent = entry.text;
        entry.detail.hidden = !entry.text;
      } else if (entry.plan) {
        entry.list.replaceChildren.apply(entry.list, entry.entries.map(function (e) {
          var li = document.createElement('li');
          li.textContent = e.content + (e.status === 'completed' ? ' (done)' : e.status === 'in_progress' ? ' (now)' : '');
          return li;
        }));
      } else if (entry.kind === 'thought') entry.el.textContent = entry.text;
      else {
        // A subagent's message is drawn as the chat draws one, made safe
        // before it is shown (safe-html.js), with its math and diagrams.
        if (!window.dinahChatText) { entry.el.textContent = entry.text; return; }
        entry.el.replaceChildren(window.dinahChatText.nodes(entry.text, null, ws));
        if (window.dinahDiagrams) window.dinahChatText.drawDiagrams(entry.el);
      }
    }
    function atEnd() { return body.scrollTop + body.clientHeight >= body.scrollHeight - 24; }
    /* Whether the window follows the output. It is decided by the person's
       scrolling, never measured when output is drawn: a new row is added to
       the log as soon as it arrives, so by the time the log is drawn the end
       has already moved, and measuring then would stop following at every
       update. Scrolling back to the end follows again, and anything that
       grows the log later, such as a diagram or a picture, keeps it at the
       end while it follows. */
    var following = true;
    body.addEventListener('scroll', function () { following = atEnd(); }, { passive: true });
    var grew = new ResizeObserver(function () { if (following && !disposed) body.scrollTop = body.scrollHeight; });
    grew.observe(log);
    function flush(deadline) {
      if (disposed) return false;
      var it = dirty.values(), next, did = false;
      while (!(next = it.next()).done) {
        if (did && performance.now() >= deadline) { if (following) body.scrollTop = body.scrollHeight; return true; }
        dirty.delete(next.value);
        if (log.contains(next.value.li)) draw(next.value);
        did = true;
      }
      if (following) body.scrollTop = body.scrollHeight;
      return false;
    }
    function schedule() { if (!disposed) window.dinahRender.enqueue(renderKey, root, flush, null, 'subagent output'); }

    /* Links in the subagent's messages open as the chat's do, with paths
       read from the chat's folder. */
    if (window.dinahChatText) window.dinahChatText.openLinks(log, function () { return ws; });

    /* Live updates that arrive while the kept ones load wait for them, and
       those the kept ones already hold are passed over. */
    off.push(host().on('agent:subagent-update', function (m) {
      if (!m || m.subagentId !== id) return;
      if (!loaded) { pending.push(m); return; }
      if (m.seq <= seq) return;
      seq = m.seq;
      feed(m.update);
      schedule();
    }));
    off.push(host().on('agent:subagent', function (m) {
      if (!m || !m.subagent || m.sessionId !== rootId) return;
      if (m.subagent.id === id) { sub = Object.assign({}, sub, m.subagent); drawHead(); timer(); }
      log.querySelectorAll('[data-subagent="' + CSS.escape(m.subagent.id) + '"]').forEach(function (li) { drawChildRow(li, m.subagent); });
      if (!loaded) load();
    }));
    // A chat opened again replays its subagents; a window restored before
    // that finds this one once the chat has it.
    off.push(host().on('agent:session', function (info) { if (info && info.sessionId === rootId && !loaded) load(); }));

    var ticking = 0;
    function timer() {
      clearInterval(ticking);
      ticking = sub && sub.state === 'running' && !disposed ? setInterval(function () { $('ran').textContent = ran(sub); }, 1000) : 0;
    }
    var loading = null;
    function load() {
      if (loading || loaded || disposed) return loading;
      if (!app() || !app().SubagentUpdates) { empty('This Subagent Is Not Available', productName() + ' cannot read subagents here.'); return null; }
      loading = Promise.resolve().then(function () { return app().SubagentUpdates(id); }).then(function (rec) {
        if (disposed) return;
        sub = rec.subagent; prompt = rec.subagent.prompt || ''; seq = rec.seq || 0;
        log.replaceChildren(); rows = {}; current = null;
        (rec.updates || []).forEach(feed);
        loaded = true;
        pending.forEach(function (m) { if (m.seq > seq) { seq = m.seq; feed(m.update); } });
        pending = [];
        drawHead(); timer();
        if (!log.children.length) empty(sub.state === 'running' ? 'Nothing Yet' : 'Nothing Kept', sub.state === 'running' ? 'The subagent\'s work appears here as it happens.' : productName() + ' kept none of this subagent\'s work.');
        schedule();
        body.scrollTop = body.scrollHeight;
      }).catch(function (err) {
        if (disposed) return;
        // The next try reads everything kept, so what waited is not needed.
        pending = [];
        empty('This Subagent Is Not Known Yet', String(err && err.message || err).replace(/^Error:\s*/, '') + '.');
      }).finally(function () { loading = null; });
      return loading;
    }
    load();

    function canStop() { return !!sub && sub.canCancel && sub.state === 'running'; }
    var unregister = window.pudlCommands ? window.pudlCommands.register(root, function () {
      return [
        { id: 'stop-subagent', label: 'Stop the Subagent', glyph: 'stop', danger: true, hidden: !canStop(), run: function () { return app().CancelSubagent(id); } },
        { id: 'show-subagents', label: 'Show Subagents', run: function () { return openTree(rootId, ws); } },
        { id: 'open-chat', label: 'Open the Chat', run: function () { return chats().openChat(rootId); } }
      ];
    }) : function () {};
    off.push(unregister);
    return {
      commands: function () {
        var out = [{ label: 'Show Subagents', run: function () { return openTree(rootId, ws); } }, { label: 'Open the Chat', run: function () { return chats().openChat(rootId); } }];
        if (canStop()) out.unshift({ label: 'Stop the Subagent', danger: true, run: function () { return app().CancelSubagent(id).catch(host().fail); } });
        return out;
      },
      destroy: function () {
        disposed = true;
        clearInterval(ticking);
        grew.disconnect();
        window.dinahRender.cancel(renderKey);
        off.forEach(function (fn) { if (fn) fn(); });
      }
    };
  }

  window.dinahSubagents = { badge: badge, state: stateOf, tree: tree, ran: ran, openTree: openTree, openArchive: openArchive, open: openOne, openButton: openButton };
  window.dinahWorkbench.register('subagents', { applet: { id: 'dinah.chat', title: 'Chats', glyph: 'comments', workspace: true }, title: 'Subagents', glyph: 'subagents', init: initTree });
  window.dinahWorkbench.register('subagent-archive', { applet: { id: 'dinah.chat', title: 'Chats', glyph: 'comments', workspace: true }, title: 'Earlier Subagents', glyph: 'subagents', init: initArchive });
  window.dinahWorkbench.register('subagent', { applet: { id: 'dinah.chat', title: 'Chats', glyph: 'comments', workspace: true }, title: 'Subagent', glyph: 'subagents', init: initOne });
})();
