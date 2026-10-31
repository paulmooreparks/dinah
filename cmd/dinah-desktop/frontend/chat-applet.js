/* Chat controllers own their views, provider preferences and window lifecycle.
   The shell supplies store access and actions for the surrounding applets. */
(function () {
  'use strict';

  function create(adapter) {
    const { toast, fail, ask, productName, drainAgentEvents, title, drawSessions, refreshSessions, startRename, openRemove, chatPin, sameWs, normalizedWorkspace, when, workspaceLabel, drawDefaults, drawPrefs, scanProviders, sendKeyName, needsCount, waitingApproval, drawNoticeCounts, permissionCard, headsUp, providerStopped, permissions, setTitles, hiddenCount, noWorkspaceYet } = adapter;
    const { app, num, money, duration, contentPct, events } = window.dinahHelpers;
    const on = window.dinahHost.on;
    const $ = id => document.getElementById(id);
    const sessionStore = adapter.store;
    const sessions = sessionStore.records;
    const S = sessionStore.get, applySessionInfo = sessionStore.applyInfo, sessionProvider = sessionStore.provider;

    var active = null;      // the id of the session in front
    var providers = [], defaultProvider = '', providerCatalogs = {}, providerPins = {}, providerDefaults = {};
    // Each provider's limit on the files in one message, with its default and
    // source, by provider (file_limits.go); State gives them.
    var fileLimits = {}, optionSubstitutions = {};
    /* What the engine declares about a provider (its facts: prices, usage, skills, removal, executable,
       sign-in), from its entry in the list and kept current by providers:facts. Nothing is assumed of a
       provider the list does not have. */
    function providerFact(id, name) {
      var found = providers.filter(function (p) { return p.id === id; })[0];
      return found && found.facts ? found.facts[name] : undefined;
    }
    // Each chat window keeps its own skill groups; they follow the providers' facts when those change.
    var skillGroupUpdates = new Set();
    function refreshSkillGroups() { skillGroupUpdates.forEach(function (update) { update(); }); }
    window.dinahProviderFact = providerFact;
    window.dinahProviderName = providerName;
    window.dinahOwnCommandsProvider = function () { var own = providers.filter(function (p) { return p.facts && p.facts.ownCommands; })[0]; return own ? own.id : ''; };
    // A provider's name is the one the engine's list gives it; one that is not in the list has no name of its own.
    function providerName(id) {
      var found = providers.filter(function (p) { return p.id === id; })[0];
      return found && found.name || 'Agent';
    }
    /* The providers to choose from, with why one cannot be chosen: the chat's
       own is always among them. */
    function providerChoices(chosen) {
      var list = providers;
      if (!list.some(function (p) { return p.id === chosen; })) list = list.concat([{ id: chosen, installed: false, statusWords: 'Not Offered' }]);
      return list.map(function (p) {
        // The words that say what is wrong come from the engine's list, made from the provider's status code.
        return { id: p.id, name: p.name || providerName(p.id), problem: p.installed ? '' : p.statusWords || 'Not Ready' };
      });
    }
    function providerOptions(select, chosen) {
      select.replaceChildren();
      providerChoices(chosen).forEach(function (p) {
        var option = new Option(p.name + (p.problem ? ' (' + p.problem + ')' : ''), p.id);
        option.disabled = !!p.problem;
        select.add(option);
      });
      select.value = chosen;
    }
    function rememberCatalog(s) {
      // Replayed options can arrive before the session's provider metadata.
      if (!s.options.length || !s.info.provider) return;
      var id = sessionProvider(s);
      providerCatalogs[id] = s.options;
      if (id === defaultProvider) catalog = s.options;
    }

    /* Subagents (subagents.go, subagents.js): the workers the chat's agent
       starts while it handles a prompt, each a session of its own whose work
       is not the chat's conversation. The chat keeps the list, every
       subagent with its parent, so that the tree and the rows in the chat
       stay current. Paul decided on 28 October 2026 that the Subagents menu,
       the tree and the counts show only this run's subagents, those Dinah saw
       start since it started, and that the earlier ones, which a chat's
       history brings back when it opens, are kept in an archive: the Earlier
       Subagents window, reached from the chat's entry in the Chats pane and
       from the foot of the Subagents menu and the tree. */
    function currentSubagents(s) { return s ? (s.subagents || []).filter(function (x) { return x.thisRun; }) : []; }
    function earlierSubagents(s) { return s ? (s.subagents || []).filter(function (x) { return !x.thisRun; }) : []; }
    function runningSubagents(s) { return currentSubagents(s).filter(function (x) { return x.state === 'running'; }).length; }
    function subagentOf(s, id) { return s ? (s.subagents || []).filter(function (x) { return x.id === id; })[0] || null : null; }
    /* When a subagent ran, as near as the chat can say: the time on its row in
       the chat, which for a replayed chat is the time of the message before it
       (stampReplay). A row no longer in the page has none. */
    function subagentRowTime(s, id) {
      var at = '';
      [s.log, s.keptLog].forEach(function (log) {
        var t = !at && log && log.querySelector('li[data-subagent="' + CSS.escape(id) + '"] > time.activity-time[datetime]');
        if (t) at = t.getAttribute('datetime');
      });
      return at;
    }
    /* A subagent's row in its parent's log: its name, what it was asked to
       do, its state as a glyph and a word, and Open, which shows its work. */
    function drawSubagentRow(li, s, sub) {
      var body = li.querySelector('.subagent-row-body');
      if (!body) {
        body = document.createElement('span');
        body.className = 'subagent-row-body';
        body.innerHTML = '<span class="subagent-row-head"><strong class="subagent-row-name"></strong><span class="subagent-row-state"></span></span><span class="subagent-row-task"></span>';
        li.appendChild(body);
        if (window.dinahSubagents) li.appendChild(window.dinahSubagents.openButton(s.id, sub, s.info.cwd || adapter.workspace()));
      }
      body.querySelector('.subagent-row-name').textContent = 'Subagent ' + (sub.name || '');
      // The task is drawn on two lines at most (app.css); the whole of it is
      // still the element's text, and its tooltip.
      body.querySelector('.subagent-row-task').textContent = sub.task || '';
      body.querySelector('.subagent-row-task').title = sub.task || '';
      if (window.dinahSubagents) body.querySelector('.subagent-row-state').replaceChildren(window.dinahSubagents.badge(sub));
    }
    on('agent:subagent', function (m) {
      var s = m && m.subagent && sessions[m.sessionId];
      if (!s) return;
      var list = (s.subagents || []).slice(), at = list.findIndex(function (x) { return x.id === m.subagent.id; });
      if (at >= 0) list[at] = m.subagent; else list.push(m.subagent);
      s.subagents = list;
      // A chat's entry in the Chats pane offers its earlier subagents.
      if (at < 0 && !m.subagent.thisRun) drawSessions();
      [s.log, s.keptLog].forEach(function (log) {
        if (log) log.querySelectorAll('[data-subagent="' + CSS.escape(m.subagent.id) + '"]').forEach(function (li) { drawSubagentRow(li, s, m.subagent); });
      });
      // A replay announces many at once; the chat is drawn when it ends.
      if (s.view && !s.loading) s.view.refresh();
      // The chat's Subagents menu button counts them (chatCommandTable).
      if (window.pudlCommands) window.pudlCommands.changed();
    });
    /* A menu row's second line is drawn on two lines at most (app.css), as a
       subagent's task in the Subagents menu can be a paragraph long. A line
       that is cut gets its whole text as its tooltip when the menu opens; the
       text itself stays whole, so a screen reader reads all of it. Toggle
       events do not bubble, so the page listens as they pass. */
    document.addEventListener('toggle', function (e) {
      if (e.newState !== 'open' || !e.target.querySelectorAll) return;
      e.target.querySelectorAll('.menu-row-detail, .menubar-detail').forEach(function (d) {
        if (!d.title && d.scrollHeight > d.clientHeight + 1) d.title = d.textContent;
      });
    }, true);
    window.dinahChatSubagents = {
      // This run's subagents, which the tree shows; the earlier ones, which
      // the archive shows; and all of them, for a subagent's window.
      list: function (id) { return currentSubagents(sessions[id]); },
      earlier: function (id) { return earlierSubagents(sessions[id]); },
      all: function (id) { return (sessions[id] && sessions[id].subagents || []).slice(); },
      when: function (id, subId) { return sessions[id] ? subagentRowTime(sessions[id], subId) : ''; },
      title: function (id) { return sessions[id] ? title(sessions[id]) : 'Chat'; },
      workspace: function (id) { return sessions[id] && sessions[id].info.cwd || adapter.workspace(); },
      openChat: function (id) { return openSession(id); }
    };
    /* Background work (background_tasks.go): what the agent runs on after its
       turn, such as a backgrounded command or a background subagent. A turn
       the agent starts of its own, to answer such work finishing, is reported
       as a task of type "turn" and shows as the chat working. */
    function busyOnItsOwn(s) { return !!s && (s.background || []).some(function (t) { return t.type === 'turn'; }); }
    function backgroundWork(s) { return s ? (s.background || []).filter(function (t) { return t.type !== 'turn'; }) : []; }
    function chatWorking(s) { return !!s && (s.working || busyOnItsOwn(s)); }

    const sessionLifecycle = window.dinahSessionLifecycle.create({
      store: sessionStore, app: app,
      workspace: function () { return adapter.workspace(); }, active: function () { return active; },
      chatKey: chatKey,
      isWindowOpen: function (key) { return window.pudlWindows.state().open.includes(key); },
      closeWindow: function (key) { window.pudlWindows.close(key); },
      onWindowsChange: function (fn) { document.addEventListener('pudl:windows-change', fn); },
      offWindowsChange: function (fn) { document.removeEventListener('pudl:windows-change', fn); },
      openChatIds: function () {
        if (!window.pudlWindows || !window.dinahWorkbench) return null;
        var ids = [];
        window.pudlWindows.state().open.forEach(function (key) {
          var item = window.dinahWorkbench.item(key);
          if (item && item.kind === 'chat-session' && item.path && ids.indexOf(item.path) < 0) ids.push(item.path);
        });
        return ids;
      },
      chatWindowsFor: chatWindowsFor, fail: fail, toast: toast, providerName: providerName, title: title,
      show: show, showChat: showChat, drawSaved: drawSaved, replayProgress: replayProgress,
      sessionNote: (...args) => currentChat()?.sessionNote(...args), stateLine: (...args) => currentChat()?.stateLine(...args),
      enqueue: function () { return window.dinahRender.enqueue.apply(window.dinahRender, arguments); },
      drainAgentEvents: drainAgentEvents, rememberCatalog: rememberCatalog, drawAgentState: drawAgentState,
      openingNote: openingNote, stampReplay: stampReplay, drawAll: drawAll, presentHistory: presentHistory,
      noWorkspaceYet: noWorkspaceYet,
      startLog: function (id, gen, through) { return adapter.startLog ? adapter.startLog(id, gen, through) : false; },
      catchUpLog: function (id) { return adapter.catchUpLog ? adapter.catchUpLog(id) : Promise.resolve(); }
    });
    const { openSession, restoreChat, reconnectChat, reportChatWindows,
      reconnectSaved, openResolvedSession, newSession, loadFullHistory } = sessionLifecycle;
    on('chat:connect', sessionLifecycle.connect);
    document.addEventListener('pudl:windows-change', reportChatWindows);
    document.addEventListener('dinah:ui-state-restored', function () { setTimeout(reportChatWindows, 0); });
    // A layout with no windows to restore changes no window, so the page
    // also says so once it has loaded.
    window.addEventListener('load', function () { setTimeout(reportChatWindows, 2000); });
    /* The saved messages are drawn as the agent's replay would draw them,
       through the same handlers, as a person's words, the agent's words and
       a tool's title and state. */
    /* A saved copy draws only its latest messages: the page keeps no more
       rows than that anyway (trimLog), and building thousands of rows only to
       drop them made restoring a long chat slow. The row at the top counts
       the messages not drawn and opens the chat's log, which has them all. */
    var SAVED_DRAW_MESSAGES = 400;
    function drawSaved(s, messages) {
      s.log.innerHTML = ''; s.log.hidden = true; s.settling = true; s.rows = {}; s.current = null; s.arriving = {}; s.arrivingMore = 0; s.arrivingMoreLi = null;
      // Drawn as a replay is, so that the rows take their times from the
      // session's journal (stampReplay) rather than the time of drawing.
      s.loading = true; s.replayed = 0; s.drawingSaved = true; // what is drawn here is Dinah's copy, not a replay to count
      var index = Math.max(0, messages.length - SAVED_DRAW_MESSAGES), job = {}, limit = messages.length;
      s.trimmed = index;
      ensureChat(s).trimMarker(s);
      var build = new Promise(function (resolve, reject) {
        window.dinahRender.enqueue(job, null, function (deadline) {
          try {
            if (sessions[s.id] !== s) { resolve(); return false; }
            var count = 0;
            while (index < limit && count++ < 16 && (count === 1 || performance.now() < deadline)) {
              var i = index++, m = messages[i];
              var u = null;
              if (m.role === 'you') u = { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: m.text } };
              else if (m.role === 'agent') u = { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: m.text } };
              // A message the bus delivered is drawn as the replay draws it
              // (delivered.go): Dinah's own update, not the agent's.
              else if (m.role === 'delivered') u = { sessionUpdate: 'dinah_delivered', message: m };
              else if (m.role === 'sent') u = { sessionUpdate: 'dinah_team_sent', message: m };
              // A line Dinah said keeps its kind (a warning is drawn as one); a turn's end note has none and is drawn plain.
              else if (m.role === 'status' && m.kind) u = { sessionUpdate: 'dinah_status', text: m.text, kind: m.kind };
              else if (m.role === 'tool') {
                var lines = (m.text || '').slice(0, 4096).split('\n', 2), st = (lines[1] || '').match(/^Status: (\w+)/);
                u = { sessionUpdate: 'tool_call', toolCallId: 'saved-' + i, title: lines[0], status: st ? st[1] : undefined };
              }
              if (u) { events.dispatch('agent:update', { sessionId: s.id, update: u }); if (m.images && s.current) ensureChat(s).restoreMessageImages(s.current, m.images); }
              else if (m.text) ensureChat(s).prependContinuation([m]);
            }
            if (index < limit) return true;
            resolve();
          } catch (error) { reject(error); }
          return false;
        }, null, 'saved history');
      });
      s.savedBuild = build.then(function () {
        if (sessions[s.id] !== s) return;
        s.loading = false;
        s.saved = true; s.historyAvailable = true;
        stampReplay(s);
        return new Promise(function (resolve) {
          drawAll(s, function () {
            if (sessions[s.id] === s) { presentHistory(s); currentChat()?.sessionNote(); }
            resolve();
          });
        });
      }).finally(function () { s.loading = false; s.drawingSaved = false; s.savedBuild = null; if (s.view) s.view.drawLoadingChip(); });
      return s.savedBuild;
    }
    function presentHistory(s, kept) {
      var chat = ensureChat(s), position = kept && chat.historyPosition();
      chat.prepareHistoryLayout(s.log, position);
      s.log.style.visibility = 'hidden';
      if (kept && kept.log.parentNode) kept.log.replaceWith(s.log);
      s.keptLog = null;
      s.log.hidden = false;
      if (s.id === active) show(s.id);
      chat.replayDone(s);
      if (position && !position.following) chat.historyPosition(position); else chat.scroll(true);
      chat.scrollReady();
      s.settling = false;
      s.log.style.visibility = '';
    }
    /* An option's value as the page writes it, and for the person to read:
       its name, with the value beside it when the two differ (as
       option_substitutions.go writes it). */
    function valueText(v) { return typeof v === 'string' ? v : JSON.stringify(v); }
    function valueWords(name, v) {
      var text = valueText(v);
      return !name ? text : name.toLowerCase() === text.toLowerCase() ? name : name + ' (' + text + ')';
    }
    /* A line in the chat that says what Dinah did or what is the matter, in the words it was sent with. */
    function statusRow(s, text, kind) {
      var li = row(s, 'turn-row status-row' + (kind === 'warning' ? ' status-warning' : ''));
      var words = document.createElement('span'); words.className = 'activity-muted'; words.textContent = text;
      li.appendChild(words);
    }
    /* openingNote is a muted line in a chat about how it was opened. */
    function openingNote(s, text) {
      var li = row(s, 'turn-row opening-note');
      var words = document.createElement('span'); words.className = 'activity-muted';
      words.textContent = text;
      li.appendChild(words);
    }

    /* The times of a replayed history. Dinah's journal of the session
       holds, in order, when each of the person's messages was sent and when
       each of the agent's began; they are put on the replayed messages in the
       same order, and every row after a message takes that message's time. A
       session Dinah never saw has no journal and shows no times. When
       the journal holds fewer entries of a kind than were replayed, as for a
       session begun in Devin Desktop and carried on here, the times go on the
       latest messages, which are the ones Dinah saw. */
    // A chat that closes takes the cards of the messages that were arriving with it.
    function clearArriving(s) {
      Object.keys(s.arriving || {}).forEach(function (id) { s.arriving[id].remove(); });
      if (s.arrivingMoreLi) s.arrivingMoreLi.remove();
      s.arriving = {}; s.arrivingMore = 0; s.arrivingMoreLi = null;
    }
    function applySessionEvent(info) {
      var s = S(info.sessionId);
      var previousProvider = s.info.provider;
      if (info.open === false) clearArriving(s);
      applySessionInfo(s, info);
      rememberCatalog(s);
      if (s.view) {
        s.view.refresh();
        if (previousProvider !== s.info.provider) s.view.loadSkills();
      }
    }
    function sessionsDrawn() {
      drawAgentState();
      drawSessions();
      chatControllers.forEach(function (chat) { chat.refreshMentions(); });
    }
    on('agent:session', function (info) {
      applySessionEvent(info);
      sessionsDrawn();
    });
    /* After a resync the sessions are read again and each one is applied as an
       agent:session event would apply it, so a chat's working state, queue,
       background tasks and subagents catch up; the options, commands,
       accounts and prices follow. The page's own copy of a chat's text is not
       repaired here. */
    window.dinahHost.onResync(function () {
      return Promise.resolve(refreshSessions()).then(function () {
        adapter.listedSessions().forEach(applySessionEvent);
        sessionsDrawn();
        var again = [];
        // Each open chat's options, the titles, the accounts and the Devin prices, as their events carry them.
        Object.keys(sessions).forEach(function (id) {
          if (!sessions[id].info.open || !app().SessionOptions) return;
          again.push(Promise.resolve(app().SessionOptions(id)).then(function (options) { sessionOptions({ sessionId: id, options: options }); }));
        });
        again.push(loadModelPrices());
        return Promise.all(again);
      });
    });
    function sessionOptions(m) {
      var s = S(m.sessionId);
      s.options = m.options || [];
      rememberCatalog(s);
      if (s.view) s.view.renderComposerOptions();
      if (m.error) fail(m.error);
    }
    on('agent:options', sessionOptions);

    /* ---- Devin's process -------------------------------------------------- */
    var BADGE = { starting: ['accent running', 'starting'], ready: ['positive', 'ready'], stopped: ['', 'off'], failed: ['danger', 'failed'] };
    /* ---- Restarting Devin ---------------------------------------------------
       A restart stops Devin and starts it again, so that it reads its skills,
       settings and MCP servers afresh, as a skill asks after installing them.
       Every session ends with the old process, so a turn that is running
       stops; the session in front is opened again, which replays it. The
       Devin item in the status area opens the menu that offers it. */
    var restarting = false;
    var providerStates = {};
    function frontChat() {
      var key = window.pudlWindows && window.pudlWindows.state().top;
      var item = key && window.dinahWorkbench.item(key);
      if (key === 'chat') return chatControllers.get(key);
      // A restored window can use a workspace identity or an older folder
      // spelling. The session owns its controller independently of that key.
      if (item && item.kind === 'chat-session') {
        var s = sessions[item.path];
        return s && s.view || chatControllers.get(key);
      }
      return null;
    }
    document.addEventListener('paste', function (event) {
      var chat = frontChat();
      if (chat && !event.defaultPrevented) chat.paste(event);
    });
    function frontProvider() { var chat = frontChat(); return chat ? chat.provider() : defaultProvider; }
    function drawAgentState() {
      var id = frontProvider(), name = providerName(id), st = providerStates[id] || { state: 'stopped', detail: name + ' is not started' };
      var b = $('status-agent-badge'), v = BADGE[st.state] || ['', st.state];
      b.className = 'badge ' + v[0]; b.textContent = v[1];
      $('status-agent').querySelector('.status-words').textContent = name;
      $('status-agent').dataset.statusState = st.state === 'failed' ? 'stuck' : st.state === 'starting' ? 'working' : 'idle';
      $('status-agent').dataset.statusText = v[1];
      $('status-agent').dataset.statusGlyph = st.state === 'failed' ? 'warning' : st.state === 'starting' ? 'chat-working' : st.state === 'ready' ? 'tick' : 'ring';
      $('status-agent').dataset.statusColour = st.state === 'failed' ? 'danger' : st.state === 'starting' ? 'accent' : st.state === 'ready' ? 'positive' : 'text-muted';
      $('status-agent').title = name + ': ' + st.detail;
      $('status-agent').setAttribute('aria-label', name + ': ' + st.detail);
      $('agent-state').textContent = st.detail + '.';
      $('agent-menu').setAttribute('aria-label', name);
      $('agent-menu').querySelector('[data-cmd="sign-in"] .md-meta').textContent = 'Opens ' + name + '\'s sign-in page in your browser';
      document.querySelectorAll('[data-cmd="restart"]').forEach(function (b) { (b.querySelector('span') || b).textContent = 'Restart ' + name; });
      document.querySelectorAll('[data-cmd="sign-in"]').forEach(function (b) { (b.querySelector('span') || b).textContent = 'Sign In to ' + name + '…'; });
    }
    function restartAgent() {
      if (restarting) return;
      if ($('agent-menu').matches(':popover-open')) $('agent-menu').hidePopover();
      restarting = true;
      var focusedChat = frontChat(), provider = frontProvider();
      var front = focusedChat ? active : null;
      Object.keys(sessions).forEach(function (k) { if (sessionProvider(sessions[k]) === provider) { sessions[k].info.open = false; sessions[k].working = false; } });
      show(front);
      (app().AgentRestart ? app().AgentRestart(provider) : app().Restart()).then(function () {
        restarting = false;
        toast(providerName(provider) + ' restarted.', 'positive');
        currentChat()?.loadSkills();
        if (front) return openSession(front);
        refreshSessions();
      }).catch(function (err) { restarting = false; fail(err); });
    }
    /* The agent menu says which agent new chats use and changes it, as
       Settings › Agents does, so the default is set where it is shown. */
    function drawDefaultAgentRows() {
      var host = $('default-agent-rows');
      host.replaceChildren();
      Array.from($('default-provider').options).forEach(function (option) {
        var row = document.createElement('button');
        row.type = 'button'; row.className = 'menu-action'; row.setAttribute('role', 'menuitemradio');
        var chosen = option.value === defaultProvider;
        row.setAttribute('aria-checked', String(chosen));
        // PUDL's check glyph marks the agent in use, and keeps the names aligned.
        var mark = document.createElement('span');
        mark.className = 'glyph'; mark.setAttribute('aria-hidden', 'true');
        mark.style.setProperty('--glyph', 'var(--glyph-check)');
        mark.style.visibility = chosen ? 'visible' : 'hidden';
        row.append(mark, document.createTextNode(' ' + option.textContent));
        row.disabled = option.disabled || $('default-provider').disabled;
        row.addEventListener('click', function () {
          $('agent-menu').hidePopover();
          if (option.value === defaultProvider) return;
          $('default-provider').value = option.value;
          $('default-provider').dispatchEvent(new Event('change'));
        });
        host.appendChild(row);
      });
    }
    /* The Usage… row of the agent menu is for the agent the menu is for: its
       account usage as its usage panel reports it, each window's use in a few
       words, with the warning glyph from 90% on. The row opens the agent's
       usage page. */
    var menuUsageRequest = 0;
    function drawMenuUsage(provider) {
      var note = $('agent-menu-usage'), request = ++menuUsageRequest;
      showMenuUsage('Reading…', false);
      // The chat's usage panel has helpers like these, in a scope of its own.
      var isNum = function (v) { return typeof v === 'number' && Number.isFinite(v); };
      var fmt = function (n) { return Number(n || 0).toLocaleString('en-US'); };
      var windowWords = function (w) {
        var m = w.windowDurationMins;
        return isNum(m) && m > 0 ? (m % 1440 === 0 ? fmt(m / 1440) + ' day' : m % 60 === 0 ? fmt(m / 60) + ' hour' : fmt(m) + ' minute') + ' window' : 'reported window';
      };
      Promise.resolve().then(function () { return app().ProviderUsage ? app().ProviderUsage(provider) : null; }).then(function (info) {
        if (request !== menuUsageRequest) return;
        info = info || { status: 'unavailable' };
        var words = [], warn = false;
        (info.buckets || []).forEach(function (bucket) {
          [bucket.primary, bucket.secondary].forEach(function (w) {
            if (!w || !isNum(w.usedPercent)) return;
            words.push(fmt(w.usedPercent) + '% of the ' + windowWords(w) + ' used');
            if (w.usedPercent >= 90) warn = true;
          });
        });
        var text = words.length ? providerName(provider) + ': ' + words.join('; ') : info.detail || (info.status === 'available' ? 'No quota windows reported.' : 'Account usage is unavailable.');
        showMenuUsage(text, warn);
      }).catch(function (err) { if (request === menuUsageRequest) note.textContent = 'The usage could not be read: ' + String(err); });
    }
    /* A window near its limit is marked with the warning glyph and colour, as
       PUDL marks a status. */
    function showMenuUsage(text, warn) {
      var el = $('agent-menu-usage');
      el.replaceChildren();
      el.classList.toggle('menu-usage-warn', !!warn);
      if (warn) {
        var g = document.createElement('span');
        g.className = 'glyph'; g.setAttribute('aria-hidden', 'true');
        g.style.setProperty('--glyph', 'var(--glyph-warning)');
        el.append(g, document.createTextNode(' '));
      }
      el.append(document.createTextNode(text));
    }
    $('status-agent').addEventListener('click', function () {
      drawAgentState();
      drawDefaultAgentRows();
      var provider = frontProvider();
      drawMenuUsage(provider);
      var n = Object.keys(sessions).filter(function (k) { return sessions[k].working && sessionProvider(sessions[k]) === provider; }).length;
      $('restart-meta').textContent = n
        ? 'Stops ' + (n === 1 ? 'the turn that is running' : 'the ' + n + ' turns that are running') + ', reads the skills, settings and MCP servers again, and reopens this session'
        : 'Reads the skills, settings and MCP servers again, and reopens this session';
    });

    // quiet: a state read again after a resync is not reported as a failure anew.
    function providerStateChanged(st, quiet) {
      providerStates[st.provider || ''] = st;
      drawAgentState();
      if (st.state === 'ready') loadModelPrices(st.provider);
      if (st.state === 'failed' && !quiet) fail(st.detail);
      if (st.state === 'stopped') {
        providerStopped(st.provider || '');
      }
    }
    on('agent:state', function (st) { providerStateChanged(st, false); });
    window.dinahHost.onResync(function () {
      if (!app().ProviderStates) return null;
      return Promise.resolve(app().ProviderStates()).then(function (list) { (list || []).forEach(function (st) { if (window.dinahStateOrder.accept("agent:state", st.provider, st, true)) providerStateChanged(st, true); }); });
    });


    var catalog = [], defaults = {}, pinned = [], templates = [], context = null;
    var MODES = {
      'accept-edits': ['Work', 'Reads, writes files and runs tools'],
      'ask': ['Ask', 'Answers questions and changes nothing'],
      'plan': ['Plan', 'Proposes a plan and waits for your go-ahead']
    };
    function show(id) {
      active = id;
      var chat = ensureChat(id ? S(id) : null);
      markCurrentChat();
      chat.refresh();
      drawSessions();
    }
    function row(s, cls) { return ensureChat(s).row(s, cls); }
    function drawAll(s, ready) { return ensureChat(s).drawAll(s, ready); }
    function stampReplay(s) { return ensureChat(s).stampReplay(s); }
    function replayProgress(s) { return ensureChat(s).replayProgress(s); }
    function signIn() {
      var chat = frontChat();
      if (chat) return chat.signIn();
      var provider = frontProvider();
      if ($('agent-menu').matches(':popover-open')) $('agent-menu').hidePopover();
      return Promise.resolve().then(function () { return app().AgentSignIn ? app().AgentSignIn(provider) : app().SignIn(); }).then(function () {
        toast('Sign-in was requested. Test Connection in Settings checks whether it completed.', 'positive');
      }).catch(fail);
    }
    function now() { return dinahRegional.format(new Date(), 'clock'); }


    // Messages are drawn and copied by the renderer every window shares
    // (chat-text.js).
    function copyText(text, said) { window.dinahChatText.copy(text, said); }

    function optValues(o) {
      var out = [];
      (o.options || []).forEach(function (v) {
        if (v.options) v.options.forEach(function (g) { out.push({ value: g.value, name: g.name, description: g.description }); });
        else out.push({ value: v.value, name: v.name, description: v.description });
      });
      if (o.category === 'mode') out = out.map(function (v) { var m = MODES[v.value]; return m ? { value: v.value, name: m[0], description: m[1] } : v; });
      return out;
    }
    function byCat(opts, cat) { return (opts || []).filter(function (o) { return o.category === cat; })[0]; }
    function valueName(o, value) {
      var v = optValues(o).filter(function (x) { return x.value === value; })[0];
      return v ? v.name : String(value);
    }

    /* The options the bar over the prompt shows. For an open session, its
       own. With no session in front, what the next session will start with:
       Devin's list from the last session seen, with the defaults and any
       choice made here since; such a choice is applied when the session
       starts. For a past session that is not open, none: nothing can be
       changed until it is. */

    function modelRows(host, o, opts) {
      var rowPins = opts.pinned || pinned, pinModel = opts.pin || togglePin;
      host.innerHTML = '';
      var f = (opts.filter || '').trim().toLowerCase();
      var all = optValues(o).filter(function (v) { return !f || (v.name + ' ' + v.value).toLowerCase().indexOf(f) >= 0; });
      var pins = all.filter(function (v) { return rowPins.indexOf(v.value) >= 0; });
      var rest = all.filter(function (v) { return rowPins.indexOf(v.value) < 0; });
      function section(name, vals) {
        if (!vals.length) return;
        var h = document.createElement('div');
        h.className = 'md-section-label';
        h.textContent = name;
        host.appendChild(h);
        vals.forEach(function (v) {
          var r = document.createElement('div');
          r.className = 'md-row model-row';
          var fav = rowPins.indexOf(v.value) >= 0;
          if (opts.choose) {
            r.innerHTML = '<a class="md-item" href="#" role="option"></a><button type="button" class="icon-btn model-pin"><span class="glyph" style="--glyph: var(--glyph-pin)" aria-hidden="true"></span></button>';
            var a = r.firstChild, pin = r.lastChild;
            modelLabel(a, v.name, modelPrice(opts.provider, v.value));
            if (v.description) a.title = v.description;
            if (v.value === opts.current) { a.setAttribute('aria-current', 'true'); a.setAttribute('aria-selected', 'true'); }
            a.setAttribute('aria-disabled', String(!!opts.disabled));
            if (opts.disabled) a.tabIndex = -1;
            a.addEventListener('click', function (e) { e.preventDefault(); if (!opts.disabled) opts.choose(v.value); });
            pin.setAttribute('aria-pressed', String(fav));
            pin.setAttribute('aria-label', (fav ? 'Remove ' : 'Add ') + v.name + (fav ? ' from' : ' to') + ' the favorites');
            pin.title = fav ? 'A favorite. Press to remove it.' : 'Make it a favorite';
            pin.disabled = !!opts.disabled;
            pin.addEventListener('click', function () { pinModel(v.value, !fav).then(opts.redraw); });
          } else {
            r.innerHTML = '<label class="check fav-check"><input type="checkbox"> <span></span></label>';
            var box = r.querySelector('input');
            box.checked = fav;
            r.querySelector('span').textContent = v.name;
            box.disabled = !!opts.disabled;
            box.addEventListener('change', function () { pinModel(v.value, box.checked).then(opts.redraw); });
          }
          host.appendChild(r);
        });
      }
      section('Favorites', pins);
      section(pins.length ? 'All Models' : 'Models', rest);
      if (!all.length) host.innerHTML = '<p class="form-help menu-note">No model matches the filter.</p>';
      var cur = host.querySelector('[aria-current]');
      if (cur && !f) cur.scrollIntoView({ block: 'nearest' });
    }
    function togglePin(value, on) {
      return app().SetPinned(value, on).then(function (p) { pinned = p || []; drawDefaults(); });
    }

    /* Devin's model prices, by the model's value, as its command line lists
       them (DESIGN.md, Devin Model Prices). They arrive in the background and
       may never arrive; a model with no price is drawn as it always was. */
    var modelPrices = {}; // by provider, for each that has prices
    function modelPrice(provider, value) {
      var prices = modelPrices[provider];
      if (!prices || !Object.prototype.hasOwnProperty.call(prices, value)) return null;
      var p = prices[value];
      return p && typeof p.costSummary === 'string' && p.costSummary ? p : null;
    }
    // A context window as a reader says it: 262K, 1M.
    function contextWords(n) {
      if (!(typeof n === 'number' && n > 0 && isFinite(n))) return '';
      if (n >= 1e6) return (Math.round(n / 1e5) / 10) + 'M';
      if (n >= 1e3) return Math.round(n / 1e3) + 'K';
      return String(n);
    }
    // The price, the tier and the context window, as one quiet line.
    function priceWords(p) {
      var words = [p.costSummary];
      if (typeof p.costTier === 'string' && p.costTier) words.push(p.costTier);
      if (contextWords(p.contextTokens)) words.push(contextWords(p.contextTokens) + ' context');
      return words.join(' · ');
    }
    /* A model row's words: the name alone, as before prices existed, or the
       name over its price in PUDL's quiet metadata line. */
    function modelLabel(a, name, price) {
      if (!price) { a.textContent = name; return; }
      a.textContent = '';
      var words = document.createElement('span'); words.textContent = name;
      var meta = document.createElement('span'); meta.className = 'md-meta model-price'; meta.textContent = priceWords(price);
      a.append(words, meta);
    }
    function setModelPrices(provider, prices) {
      modelPrices[provider] = prices && typeof prices === 'object' ? prices : {};
      chatControllers.forEach(function (chat) { chat.pricesChanged(); });
      if ($('model-prefs').open) drawPrefs();
    }
    // Prices are read for a provider whose facts say its models have them, or for each such provider when none is named.
    function loadModelPrices(provider) {
      if (!app().ModelPrices) return Promise.resolve();
      var ids = provider ? [provider] : providers.map(function (p) { return p.id; });
      return Promise.all(ids.filter(function (id) { return providerFact(id, 'prices'); }).map(function (id) {
        return Promise.resolve().then(function () { return app().ModelPrices(id); }).then(function (prices) { setModelPrices(id, prices); }).catch(function () {});
      }));
    }
    on('models:prices', function (m) { if (m && providerFact(m.provider, 'prices')) setModelPrices(m.provider, m.prices); });
    // The engine tells the page the facts again when a reported fact replaces a declared one and when the default changes.
    on('providers:facts', function (list) {
      (list || []).forEach(function (f) {
        providers.forEach(function (p) { if (p.id === f.id) { p.facts = f.facts; p['default'] = !!f['default']; } });
        if (f['default']) defaultProvider = f.id;
      });
      refreshSkillGroups();
      loadModelPrices();
    });


    /* A chat controller keeps its view while its window is closed, so streamed
       events and drafts still belong to the session when it is reopened. */
    var chatControllers = new Map(), chatSerial = 0;
    var hostOn = on;
    function chatKey(id, ws) { return id ? window.dinahWorkbench.key({ kind: 'chat-session', workspace: ws, path: id }) : 'chat'; }
    function currentChat() {
      var s = active && sessions[active];
      return s ? ensureChat(s) : chatControllers.get('chat');
    }
    window.dinahChatSkills = { targets: function () {
      return Array.from(chatControllers.values()).map(function (chat) { return chat.skillTarget(); }).filter(Boolean);
    } };
    dinahSkills.subscribe(function (catalog) { chatControllers.forEach(function (chat) { chat.applySkillCatalog(catalog); }); });
    function ensureChat(s) {
      /* A session has one chat, found by the session rather than by a key
         made from its folder: the agent may report the folder spelled
         otherwise (case, separators) than the window it was started in, and
         a second chat for the session would take its log and draw every
         update a second time. */
      if (s && s.view && Array.from(chatControllers.values()).indexOf(s.view) >= 0) return s.view;
      var item = s ? { kind: 'chat-session', workspace: s.info.cwd || adapter.workspace(), path: s.id } : { kind: 'chat', workspace: adapter.workspace() };
      var key = s ? chatKey(s.id, item.workspace) : 'chat';
      if (!chatControllers.has(key)) chatControllers.set(key, makeChat(item, key));
      return chatControllers.get(key);
    }
    /* A bus sender as a heading writes it: the person's name for the person's
       bus handle (person_name.go), anyone else as the bus has them. Shared by
       every chat, so it lives outside the chat controller. */
    function speakerName(from) {
      var person = window.dinahPerson || {};
      if (!from || from !== person.handle) return from || '';
      return person.chosen || person.account || 'you';
    }
    /* A chat that sent a message is shown by its title as it is now, which the
       bus's chats give by the chat's session on the bus (bus-applets.js), so a
       renamed chat's cards follow it. A chat no longer open keeps the title it
       had when the message came. */
    function deliveredSender(d) {
      var chat = d.fromSession && window.dinahBus && window.dinahBus.chat(d.fromSession);
      if (chat) return chat.title + (chat.workspace ? ' (workspace ' + chat.workspace + ')' : '');
      return speakerName(d.from) + (d.workspace ? ' (workspace ' + d.workspace + ')' : '');
    }
    function deliveredHeading(d) {
      d = d || {};
      var from = deliveredSender(d);
      if (d.kind === 'result') return d.from ? 'Reply from ' + from : 'Completion report from ' + productName();
      if (d.kind === 'release') return 'Release notice from ' + from;
      if (d.kind === 'verdict') return 'Verdict from ' + from;
      return 'Message from ' + from;
    }
    // A changed name, the person's or a chat's, rewrites the headings of the
    // cards already drawn.
    function redrawDeliveredHeadings() {
      document.querySelectorAll('li.delivered-row').forEach(function (li) {
        var el = li.querySelector('.delivered-heading');
        if (el && li._delivered) el.textContent = deliveredHeading(li._delivered);
      });
    }
    document.addEventListener('dinah:person-name', redrawDeliveredHeadings);
    document.addEventListener('dinah:bus-chats', redrawDeliveredHeadings);
    function makeChat(item, initialKey) {
      var root = document.createElement('div');
      root.className = 'chat-instance';
      document.getElementById('applet-storage').appendChild(root);
      var viewPrefix = initialKey + '-view' + (++chatSerial);
      var view = window.dinahWorkbench.mount(root, 'chat-editor', viewPrefix);
      var $ = function (id) { return view.id(id) || document.getElementById(id); };
      /* The composer's toolbar folds. It is marked
         here, in this chat's copy, because PUDL sets a folding toolbar up when
         it first enters the page, which a copy of the chat-editor template does
         when it is mounted. */
      view.query('.composer-toolbar').setAttribute('data-toolbar-fold', '');
      var composerLayout = window.dinahComposer.layout($('composer'));
      var active = item.kind === 'chat-session' ? item.path : null;
      var workspace = item.workspace || '', key = initialKey, appletOptions = null, creating = null, switchingProvider = false;
      var chosenProvider = defaultProvider;
      function chatProvider() { return active ? sessionProvider(S(active)) : chosenProvider; }
      function agentName() { return providerName(chatProvider()); }
      function chatPins() { return chatProvider() === defaultProvider ? pinned : providerPins[chatProvider()] || []; }
      var listeners = [], deleted = false, cancelSwitchDialog = null, localSwitch = null, switchEpoch = 0;
      var teamPanel = window.dinahTeams.mount({ root: root, keepPlace: keepChatPlace,
        title: function () { return active ? title(sessions[active]) : 'New Chat'; },
        openChat: function (id) { return openSession(id); },
        changed: function () { if (window.pudlCommands) window.pudlCommands.changed(); }
      });
      var on = function (name, fn) {
        var stop = hostOn(name, function (data) {
          if (deleted || (data && data.sessionId && data.sessionId !== active)) return;
          fn(data);
        });
        listeners.push(stop);
        return stop;
      };
      function providerSwitchRow(s, text) {
        var li = row(s, 'turn-row');
        var words = document.createElement('span'); words.className = 'activity-muted';
        window.dinahBoundedText.draw(words, text || '', function (value) { words.textContent = value; });
        li.appendChild(words);
      }
      function prependContinuation(history) {
        var s = active && sessions[active];
        if (!s) return;
        history.forEach(function (entry) {
          var restored = null;
          if (entry.role === 'provider_switch') providerSwitchRow(s, entry.text);
          else if (entry.role === 'user' || entry.role === 'you') restored = message(s, 'you', entry.text, 'You');
          else if (entry.role === 'assistant' || entry.role === 'agent') restored = message(s, 'agent', entry.text, providerName(entry.provider));
          else if (entry.role === 'delivered') restored = deliveredCard(s, entry);
          else if (entry.role === 'sent') restored = teamSentCard(s, entry);
          else providerSwitchRow(s, entry.text);
          if (restored) dirty.add(restored);
          s.current = null;
        });
      }
      /* Waiting out a turn the person stopped from the confirmation: the
         wait ends with the turn or, at thirty seconds, with the wait itself,
         and the engine says whichever it then is. */
      function waitTurnEnd(id) {
        return new Promise(function (resolve) {
          // The drainer can replay a queued event while the handler registers,
          // so the release is taken only ever once, and only if it came back.
          var off = null, timer = null, done = false;
          function finish() { if (done) return; done = true; if (timer) clearTimeout(timer); if (typeof off === 'function') off(); resolve(); }
          off = on('agent:turn', function (t) { if (t.sessionId === id) finish(); });
          timer = setTimeout(finish, 30000);
        });
      }
      // Closing a confirmation ends ownership of its outstanding facts request.
      function confirmSwitchFacts(load, initial) {
        return new Promise(function (resolve) {
          var dlg = document.createElement('dialog'); dlg.className = 'dialog switch-confirm';
          var form = document.createElement('form'); form.method = 'dialog';
          var title = document.createElement('h2'); title.className = 'dialog-title';
          title.textContent = initial && initial.title || 'Continue this chat?';
          var body = document.createElement('div'); body.className = 'dialog-body vstack gap-3';
          var actions = document.createElement('div'); actions.className = 'dialog-actions';
          var cancel = document.createElement('button'); cancel.type = 'button'; cancel.className = 'btn'; cancel.textContent = 'Cancel';
          var go = document.createElement('button'); go.type = 'submit'; go.className = 'btn btn-primary'; go.textContent = 'Continue'; go.disabled = true;
          var done = false, current = null, decision = 'continue';
          function finish(answer) {
            if (done) return;
            done = true;
            if (cancelSwitchDialog === closeDialog) cancelSwitchDialog = null;
            if (dlg.open) dlg.close();
            dlg.remove();
            resolve({ answer: answer, preview: current });
          }
          function closeDialog() { finish('cancel'); }
          cancelSwitchDialog = closeDialog;
          cancel.addEventListener('click', closeDialog);
          dlg.addEventListener('cancel', function (event) { event.preventDefault(); closeDialog(); });
          dlg.addEventListener('close', closeDialog);
          form.addEventListener('submit', function (event) { event.preventDefault(); if (!go.disabled && current) finish(decision); });
          function para(words, cls) { var p = document.createElement('p'); p.className = cls || 'form-help'; p.textContent = words; body.appendChild(p); return p; }
          para('The conversation continues as a new chat, with older messages summarised when needed and recent turns carried in full. This chat stays linked to it.');
          var loading = para('Checking what will change…', 'form-help switch-facts-loading'); loading.setAttribute('role', 'status');
          actions.append(cancel, go); form.append(title, body, actions); dlg.appendChild(form);
          document.body.appendChild(dlg); dlg.showModal();
          Promise.resolve().then(function () { if (!done && !deleted) return typeof load === 'function' ? load() : load; }).then(function (preview) {
            if (done || deleted) return;
            current = preview;
            var facts = preview.facts || {}, h = facts.handoff, running = facts.running || {};
            var provider = facts.kind !== 'workspace', to = facts.to || {}, from = facts.from || {};
            var toName = provider ? to.providerName : (to.workspaceName || to.workspace);
            if (toName) title.textContent = provider ? 'Continue with ' + toName + '?' : 'Continue in ' + toName + '?';
            body.replaceChildren();
            if (!provider) para('The chat continues as a new chat in ' + toName + '; its files, skills and applet actions are that workspace’s, and this one stays where it is, linked to it.');
            else para('The chat continues as a new chat with ' + (toName || initial && initial.providerName || 'the chosen provider') + (to.model ? ' (' + to.model + ')' : '') + ', and this one stays as it is, linked to it.');
            if (!h) para('The new chat carries the conversation, summarising older messages when needed and keeping recent turns in full. The handoff is prepared after you continue.');
            else if (!h.messages) para('The new chat starts with no earlier conversation.');
            else if (h.summarised) para('The new chat starts from ' + (h.judged ? 'a summary the judge wrote' : 'a summary the rule wrote') + ' of ' + num(h.summarised) + ' earlier messages, plus the ' + num(h.whole) + ' recent in full (' + num(h.chars) + ' characters against the ' + num(h.budget) + ' the handoff may take).');
            else para('The new chat starts from the whole conversation in full: ' + num(h.messages) + ' messages (' + num(h.chars) + ' characters against the ' + num(h.budget) + ' budget).');
            if (facts.notCarried && facts.notCarried.length) {
              para(provider ? 'These stay with ' + (from.providerName || 'the current provider') + ':' : 'Settings that grant authority do not carry over and are set again there if wanted:');
              var list = document.createElement('ul'); list.className = 'form-help';
              facts.notCarried.forEach(function (option) { var li = document.createElement('li'); li.textContent = provider ? option.name + ': ' + option.value : option.name + ' (' + option.value + ')'; list.appendChild(li); });
              body.appendChild(list);
            }
            (preview.warnings || []).forEach(function (warning) { para('⚠ ' + warning, 'form-help switch-warning'); });
            if (preview.blocked) para(preview.blocked, 'form-help switch-blocked');
            decision = preview.blocked && running.turn && running.canStop ? 'stop' : 'continue';
            go.textContent = decision === 'stop' ? 'Stop the Turn and Continue' : 'Continue';
            go.disabled = decision !== 'stop' && (!preview.revision || !!preview.blocked);
          }, function (error) {
            if (done || deleted) return;
            body.replaceChildren(); para(String(error), 'form-help switch-facts-error');
          });
        });
      }
      function paintSwitchPreparation(before, kind, target) {
        localSwitch = { sessionId: before.id, kind: kind, to: target, stage: 'preparing', label: 'Preparing the conversation…' };
        drawSwitchState(before);
        // The second callback permits a progress frame to paint before the RPC.
        return new Promise(function (resolve) { requestAnimationFrame(function () { requestAnimationFrame(resolve); }); });
      }
      async function switchProvider(provider, recovery) {
        var before = active && sessions[active];
        recovery = !!(recovery || (before && before.failed));
        if (!before || switchingProvider || before.working || before.loading || (before.queued || []).length || (provider === chatProvider() && !recovery)) { renderComposerOptions(); return; }
        switchingProvider = true; var epoch = ++switchEpoch; renderComposerOptions(); typed();
        try {
          var review = await confirmSwitchFacts(function () {
            return recovery ? app().PreviewSessionRecovery(before.id, provider) : app().PreviewSessionContinuation(before.id, provider);
          }, { title: recovery ? 'Continue from Saved History?' : 'Continue with ' + providerName(provider) + '?', providerName: providerName(provider) });
          if (review.answer === 'cancel' || deleted || epoch !== switchEpoch || active !== before.id) return;
          var preview = review.preview;
          if (review.answer === 'stop') {
            await app().Cancel(before.id);
            await waitTurnEnd(before.id);
            if (deleted || epoch !== switchEpoch || active !== before.id) return;
            review = await confirmSwitchFacts(function () { return app().PreviewSessionContinuation(before.id, provider); }, { title: 'Continue with ' + providerName(provider) + '?' });
            if (review.answer !== 'continue' || deleted || epoch !== switchEpoch || active !== before.id) return;
            preview = review.preview;
          }
          await paintSwitchPreparation(before, 'provider', provider);
          if (deleted || epoch !== switchEpoch || active !== before.id) return;
          var result = await (recovery ? app().RecoverSession(before.id, provider, preview.revision) : app().ContinueSessionWithProvider(before.id, provider, preview.revision));
          if (deleted || epoch !== switchEpoch || active !== before.id) return;
          await applyContinuation(before, {
            result: result,
            provider: provider,
            recovery: recovery,
            note: recovery ? 'Continued from saved history with ' + providerName(provider) + '.' : 'Switched from ' + providerName(sessionProvider(before)) + ' to ' + providerName(provider) + '.'
          });
        } catch (error) {
          if (!deleted && epoch === switchEpoch && active === before.id) {
            localSwitch = { sessionId: before.id, kind: 'provider', to: provider, failed: String(error) };
            drawSwitchState(before); fail(error);
          }
        }
        finally {
          if (localSwitch && !localSwitch.failed) localSwitch = null;
          switchingProvider = false;
          if (!deleted) { drawSwitchState(active && sessions[active]); renderComposerOptions(); typed(); }
        }
      }
      /* A continuation's answer is applied the same way for both kinds of
         switch: the old chat links to the new and stays as it was; the window
         in place takes the new chat. */
      async function applyContinuation(before, move) {
          var result = move.result, provider = move.provider;
          var recovery = !!move.recovery;
          if (move.workspace) workspace = move.workspace;
          var info = result.session, next = S(info.sessionId), oldKey = key;
          // Early provider events can create a view before the RPC returns.
          // Transfer its pending rendering and unsubscribe it before this
          // controller takes ownership of the continued session.
          if (next.view && next.view !== controller) {
            chatControllers.forEach(function (c, k) { if (c === next.view) chatControllers.delete(k); });
            next.view.transferRendering(controller);
            next.view.dispose();
          }
          var earlyRows = Array.from(next.log.childNodes);
          next.info = Object.assign({}, next.info, info);
          next.log.replaceChildren();
          while (before.log.firstChild) next.log.appendChild(before.log.firstChild);
          [dirty, dirtyTools, dirtyPlans].forEach(function (set) { set.forEach(function (entry) { if (entry.s === before) entry.s = next; }); });
          dirtyGroups.forEach(function (group) { if (group._session === before) group._session = next; });
          scheduleDraw();
          before.view = null;
          before.info.continuedBy = next.id;
          active = next.id; chosenProvider = info.provider || provider; pending = {};
          item = { kind: 'chat-session', workspace: workspace, path: next.id };
          key = chatKey(next.id, workspace);
          chatControllers.delete(oldKey); chatControllers.set(key, controller); next.view = controller;
          if (root.closest('[data-win]')) window.dinahWorkbench.rekey(oldKey, item);
          $('logs').replaceChildren(next.log); next.log.hidden = false;
          providerSwitchRow(next, move.note);
          earlyRows.forEach(function (row) { next.log.appendChild(row); });
          selectChat(next.id); controller.refresh(); loadSkills();
          refreshSessions();
      }
      /* Continue in Another Workspace (docs/plans/chat-workspaces-and-
         switching.md, item 2): a new chat with the same provider in the chosen
         workspace, carrying the summary and the recent turns; this one stays
         as it was, linked. It is not an edit of the old chat's folder. */
      function continueInWorkspace() {
        var s = active && sessions[active];
        if (!s || switchingProvider || !app().PreviewSessionWorkspaceMove) return;
        return Promise.resolve().then(function () { return app().State(); }).then(function (state) {
          var records = (state && state.workspaceRecords) || [];
          var home = s.info.cwd || workspace;
          var choices = records.filter(function (r) { return (r.executionPath || '') !== home; });
          if (!choices.length) { toast('There is one workspace.', 'warn'); return; }
          return askWorkspaceChoice(choices, home, {
            title: 'Continue in Another Workspace', yes: 'Continue', selectId: 'continue-workspace-select',
            help: 'The chat continues as a new chat in the chosen workspace, carrying a summary and the recent turns. This chat stays as it is, linked to the new one.'
          }).then(function (record) {
            if (!record) return;
            return moveToWorkspace(s.id, record);
          });
        }).catch(fail);
      }
      async function moveToWorkspace(id, record) {
        if (switchingProvider) return;
        switchingProvider = true; var epoch = ++switchEpoch; renderComposerOptions(); typed();
        try {
          var folder = record.executionPath;
          var before = sessions[id];
          var review = await confirmSwitchFacts(function () { return app().PreviewSessionWorkspaceMove(id, folder); }, { title: 'Continue in ' + (record.name || folder) + '?' });
          if (review.answer === 'cancel' || deleted || epoch !== switchEpoch || active !== id) return;
          if (review.answer === 'stop') {
            await app().Cancel(id);
            await waitTurnEnd(id);
            if (deleted || epoch !== switchEpoch || active !== id) return;
            review = await confirmSwitchFacts(function () { return app().PreviewSessionWorkspaceMove(id, folder); }, { title: 'Continue in ' + (record.name || folder) + '?' });
            if (review.answer !== 'continue' || deleted || epoch !== switchEpoch || active !== id) return;
          }
          var preview = review.preview;
          await paintSwitchPreparation(before, 'workspace', folder);
          if (deleted || epoch !== switchEpoch || active !== id) return;
          var result = await app().ContinueSessionInWorkspace(id, folder, preview.revision);
          if (deleted || epoch !== switchEpoch || active !== id) return;
          await applyContinuation(before, {
            result: result,
            provider: sessionProvider(before),
            workspace: folder,
            note: 'Continued in ' + (record.name || folder) + '.'
          });
          if (result.optionsNotCarried && result.optionsNotCarried.length) toast('The provider did not take: ' + result.optionsNotCarried.join(', ') + '.', 'warn');
        } catch (error) {
          if (!deleted && epoch === switchEpoch && active === id) {
            localSwitch = { sessionId: id, kind: 'workspace', to: folder, failed: String(error) };
            drawSwitchState(before); fail(error);
          }
        }
        finally {
          if (localSwitch && !localSwitch.failed) localSwitch = null;
          switchingProvider = false;
          if (!deleted) { drawSwitchState(active && sessions[active]); renderComposerOptions(); typed(); }
        }
      }
      function newSession() {
        if (creating) return creating;
        if (!workspace) return Promise.reject('Choose a workspace first.');
        var chosen = pending;
        pending = {};
        creating = (app().NewSessionWithProviderAt ? app().NewSessionWithProviderAt(workspace, chosenProvider) : app().NewSessionAt(workspace)).then(function (info) {
          var s = S(info.sessionId);
          applySessionInfo(s, info);
          rememberCatalog(s);
          /* The chat claims its session before anything else is awaited. The
             agent's first updates can arrive while the chosen options are
             applied, or even before this reply, and a chat made for them in
             the meantime would draw every update a second time, so it is
             closed here and this one takes the session. */
          if (s.view && s.view !== controller) {
            chatControllers.forEach(function (c, k) { if (c === s.view) chatControllers.delete(k); });
            s.view.transferRendering(controller);
            s.view.dispose();
          }
          var old = key;
          active = s.id;
          item = { kind: 'chat-session', workspace: workspace, path: s.id };
          key = chatKey(s.id, workspace);
          chatControllers.delete(old);
          chatControllers.set(key, controller);
          // Rekeying the window asks for the current chat at once, so the
          // session is this chat's before it.
          s.view = controller;
          $('logs').appendChild(s.log); s.log.hidden = false;
          if (root.closest('[data-win]')) window.dinahWorkbench.rekey(old, item);
          return Object.keys(chosen).reduce(function (promise, id) {
            return promise.then(function () { return app().SetSessionOption(info.sessionId, id, chosen[id]).then(function (options) { s.options = options || s.options; }); });
          }, Promise.resolve()).then(function () {
            selectChat(s.id);
            controller.refresh();
            return s;
          });
        }).catch(function (err) { pending = chosen; fail(err); throw err; }).finally(function () { creating = null; renderComposerOptions(); });
        return creating;
      }
    function show(id) {
      active = id;
      if (id) { var log = S(id).keptLog || S(id).log; $('logs').appendChild(log); log.hidden = false; }
      var s = id ? sessions[id] : null;
      if (s) window.dinahFailures.sync(s);
      // A chat reconnecting quietly keeps its saved copy in sight (openResolvedSession).
      if (s && s.loading) s.log.hidden = true;
      $('replay').hidden = !(s && s.loading && !s.quiet && !s.drawingSaved);
      if (s && s.loading && !s.quiet && !s.drawingSaved) replayProgress(s);
      $('log-empty').hidden = !!(s && (s.log.children.length || s.loading));
      $('log-empty').textContent = s ? 'Nothing has happened yet in this chat.' : 'Start a new chat, or open one from the list. Your first message starts a new chat with the provider and model shown below.';
      sessionNote();
      menuStates();
      requestAnimationFrame(pinCheck);
      drawNoticeCounts();
      renderComposerOptions();
      setWorking(!!(s && s.working));
      drawQueue();
      drawUsage();
      stateLine();
      drawApprovals();
    }

    function drawApprovals() {
      var host = $('chat-approvals');
      host.replaceChildren();
      permissions(active).forEach(function (entry) {
        host.appendChild(permissionCard(entry, true));
      });
      host.hidden = !host.children.length;
    }
    /* Stop asks the engine to end the turn, which can take a while when the agent
       is in the middle of a tool call, so the chat says it was asked for, and
       a failure to ask is reported rather than dropped. The turn's end clears it. */
    function stopping() { var s = active && sessions[active]; return !!(s && s.stopping && chatWorking(s)); }
    /* Stop can be pressed again while the chat says Stopping: an agent that does not
       end its turn at the first request is asked again, and a request repeated does no
       harm. Every way of stopping, the control, Escape and the Stop command, comes here. */
    function stopTurn() {
      var s = active && sessions[active];
      if (!s) return;
      var again = s.stopping;
      s.stopping = true;
      stateLine(); renderComposerOptions(); controller.refresh();
      Promise.resolve().then(function () { return app().Cancel(s.id); }).catch(function (err) {
        if (!again) s.stopping = false;
        stateLine(); renderComposerOptions(); controller.refresh(); fail(err);
      });
    }
    var stateDetail = '', stateContext = '';
    function syncWindowExecution(win, s) {
      var execution = s && window.dinahChatState.describe(s.info, { reason: false }).state;
      win.dataset.winStatusRows = '2';
      if (execution) {
        win.dataset.winState = execution;
        win.dataset.winStateLabel = window.dinahChatState.describe(s.info, { reason: false }).text;
        win.dataset.winStateDetail = stateContext;
      }
      else { win.removeAttribute('data-win-state'); win.removeAttribute('data-win-state-label'); win.removeAttribute('data-win-state-detail'); }
      return execution;
    }
    function drawWindowLifecycle(chip, s, execution) {
      chip.setAttribute('role', 'img'); chip.replaceChildren(); chip.hidden = false;
      var concealed = !!execution || (!s && !workspace);
      chip.style.visibility = concealed ? 'hidden' : '';
      chip.setAttribute('aria-hidden', String(concealed));
      chip.title = stateDetail; chip.setAttribute('aria-label', stateDetail);
      if (concealed) return;
      var mark = s ? window.dinahChatState.badge(s.info, { reason: false }) : document.createElement('span');
      if (!s) { mark.className = 'chat-state chat-lifecycle'; mark.textContent = workspaceLabel(workspace); }
      mark.title = stateDetail; mark.setAttribute('aria-label', stateDetail); chip.appendChild(mark);
      // PUDL's folded status lists the full host-owned lifecycle text.
      var context = document.createElement('span'); context.className = 'visually-hidden chat-state-context'; context.setAttribute('aria-hidden', 'true');
      context.textContent = stateContext ? ' · ' + stateContext : ''; chip.appendChild(context);
    }
    function stateLine() {
      var s = active && sessions[active];
      var state = s && window.dinahChatState.read(s.info);
      var t = !s ? 'Start a new chat, or open one from the list.'
        : s.loading ? 'Reopening the session and its history…'
        : s.failed ? agentName() + ' could not open this session.'
        : s.stopping ? 'Stop was asked for. ' + agentName() + ' is finishing what it is doing.'
        : waitingApproval(s.id) ? 'Waiting for your approval.'
        : s.info.switch && s.info.switch.stage ? 'Switching to ' + switchTargetName(s.info.switch) + '. ' + (s.info.switch.label || '')
        : state.exec && state.exec.why ? state.exec.why
        : state.life === 'closed' || state.life === 'retired' ? 'This session has ended. Send a message to carry it on.'
        : '';
      // The agent waits on its background work rather than on the person
      // (Paul, 26 October 2026); the count says how much is running.
      var work = backgroundWork(s);
      if (work.length) t += ' ' + (work.length === 1 ? '1 background task is' : work.length + ' background tasks are') + ' running.';
      var helpers = runningSubagents(s);
      if (helpers) t += ' ' + (helpers === 1 ? '1 subagent is' : helpers + ' subagents are') + ' running.';
      var stateWindow = root.closest('.win'), stateChip = stateWindow && stateWindow.querySelector('.chat-working-chip');
      var mark = s ? window.dinahChatState.describe(s.info, { reason: false }).text : 'New Chat';
      var destination = workspace ? workspaceLabel(workspace) : '';
      stateDetail = [mark, destination, t].filter(Boolean).join(' · ');
      stateContext = [s ? destination : 'New Chat', t].filter(Boolean).join(' · ');
      var execution = stateWindow && syncWindowExecution(stateWindow, s);
      if (stateChip) drawWindowLifecycle(stateChip, s, execution);
      drawBackground(s, work);
      drawSwitchState(s);
      teamPanel.update(s);
    }
      /* While a switch runs, the box and the pickers rest: where the box
         was, an indeterminate bar and the stage as the engine names it. A
         failure says why in the engine's words there, until dismissed; the
         chat comes back unchanged on its old provider or workspace
         (docs/plans/chat-workspaces-and-switching.md, While It Runs). */
      // The end a switch goes to, as the page names it from the record: a
      // provider by its display name, a workspace by its name or its folder.
      function switchTargetName(sw) {
        if (!sw || !sw.to) return '';
        if (sw.kind === 'provider') return providerName(sw.to);
        return workspaceLabel(sw.to) === sw.to ? sw.to : workspaceLabel(sw.to);
      }
      function drawSwitchState(s) {
        var prog = $('switch-progress');
        if (!prog) return;
        var composer = $('composer');
        var sw = s && s.info && s.info.switch;
        if (localSwitch && s && localSwitch.sessionId === s.id && (localSwitch.failed || !sw || !sw.stage)) sw = localSwitch;
        var running = !!(sw && sw.stage);
        var failed = !!(sw && sw.failed);
        prog.hidden = !running && !failed;
        composer.hidden = running;
        var row = prog.querySelector('.switch-running');
        var stageEl = prog.querySelector('.switch-stage');
        var notice = prog.querySelector('.switch-failure');
        var failTitle = prog.querySelector('.switch-failure-title');
        var failEl = prog.querySelector('.switch-failed');
        row.hidden = !running;
        if (running) stageEl.textContent = sw.label || '';
        notice.hidden = !failed;
        if (failed) {
          failTitle.textContent = 'The switch to ' + switchTargetName(sw) + ' failed';
          failEl.textContent = sw.failed;
        }
      }
    /* The chat's background tasks under its state line, each with what it is
       doing and a Stop button when the agent allows it. */
    function drawBackground(s, work) {
      var list = $('background-tasks');
      if (!list) return;
      list.hidden = !work.length;
      list.replaceChildren.apply(list, work.map(function (t) {
        var li = document.createElement('li'); li.className = 'background-task';
        var mark = document.createElement('span'); mark.className = 'glyph background-task-mark'; mark.setAttribute('aria-hidden', 'true');
        mark.style.setProperty('--glyph', 'var(--glyph-reload)');
        var words = document.createElement('span'); words.className = 'background-task-words';
        var name = document.createElement('strong'); name.textContent = t.name || 'Background task';
        words.appendChild(name);
        var more = t.summary || t.description;
        if (more) { var detail = document.createElement('span'); detail.className = 'background-task-detail'; detail.textContent = ' · ' + more; words.appendChild(detail); }
        li.append(mark, words);
        if (t.canStop) {
          var stop = document.createElement('button'); stop.type = 'button'; stop.className = 'btn btn-sm'; stop.textContent = 'Stop';
          stop.setAttribute('aria-label', 'Stop ' + (t.name || 'the background task'));
          stop.addEventListener('click', function () {
            stop.disabled = true;
            Promise.resolve().then(function () { return app().StopBackgroundTask(s.id, t.id); }).catch(function (err) { stop.disabled = false; fail(err); });
          });
          li.appendChild(stop);
        }
        return li;
      }));
    }


    function stampReplay(s) {
      Promise.resolve().then(function () { return app().SessionTimes(s.id); }).then(function (entries) {
        entries = entries || [];
        var rows = Array.prototype.slice.call(s.log.children);
        var msgs = { you: [], agent: [] };
        rows.forEach(function (li) { var m = li._msg; if (m && (m.kind === 'you' || m.kind === 'agent')) msgs[m.kind].push(li); });
        var times = new Map();
        ['you', 'agent'].forEach(function (kind) {
          var list = entries.filter(function (e) { return e.kind === kind; });
          var lis = msgs[kind], skip = Math.max(0, lis.length - list.length), from = Math.max(0, list.length - lis.length);
          lis.forEach(function (li, n) { if (n >= skip) times.set(li, list[from + n - skip].at); });
        });
        var last = null;
        rows.forEach(function (li) {
          // A delivered message keeps the time it was sent (deliveredCard).
          if (li._delivered) return;
          var at = times.get(li);
          if (at) last = at;
          else if (!li._msg) at = last;
          var t = li.querySelector(':scope > time.activity-time');
          if (!t) return;
          dinahRegional.stamp(t, at || '', 'clock');
        });
        // The Earlier Subagents window reads when each subagent ran from its
        // row (subagents.js).
        document.dispatchEvent(new CustomEvent('dinah:chat-times', { detail: { sessionId: s.id } }));
      }).catch(function () {});
    }

    function replayProgress(s) {
      if (s.id !== active || s.quiet || s.drawingSaved) return;
      var p = $('replay');
      p.hidden = false;
      var waiting = waitingCount(s);
      // The count is of what the agent has sent; until it sends something, the chat is only opening, which
      // is also all a chat reopened with session/resume ever does.
      p.textContent = (s.replayed ? 'Reopening the session. ' + agentName() + ' is sending its history: ' + s.replayed + ' parts so far.' : 'Reopening the session…')
        + (waiting ? ' ' + (waiting === 1 ? 'One message is' : waiting + ' messages are') + ' waiting for ' + agentName() + ' and will show when it is done.' : '');
      $('log-empty').hidden = true;
    }
    /* The replay is drawn, whether the agent sent it or it came from Dinah's
       saved copy: the progress line goes and the chat's log shows. */
    function replayDone(s) {
      if (s.id !== active) return;
      $('replay').hidden = true;
      s.log.hidden = false;
      $('log-empty').hidden = !!s.log.children.length;
    }

    /* The note over the prompt for a session that could not be opened. */
    /* A send held because the context is nearly full (the chat's information has a hold): the courses the engine
       offers, with setting the oldest turns aside marked as the default. Picking one runs it, releases the held
       message and is remembered for the chat; the card is only for a chat with no choice yet. */
    function drawHold(s) {
      var hold = s && s.info.hold && s.info.hold.kind === 'context' ? s.info.hold : null;
      $('context-hold').hidden = !hold;
      if (!hold) { $('context-hold-status').textContent = ''; return; }
      $('context-hold-body').textContent = num(hold.used) + ' of ' + num(hold.size) + ' tokens are in use. Your next message waits until you choose. The choice is remembered for this chat, and Ask Me Again About a Full Context, in the chat\'s Actions menu, changes it.';
      $('hold-set-aside').hidden = (hold.actions || []).indexOf('set-aside') < 0;
      $('hold-compact').hidden = (hold.actions || []).indexOf('compact') < 0;
    }
    function resolveHold(action) {
      var s = active && sessions[active];
      if (!s || !app().ResolveHold) return;
      var id = s.id, buttons = ['hold-set-aside', 'hold-compact', 'hold-new'].map($);
      buttons.forEach(function (b) { b.disabled = true; });
      $('context-hold-status').textContent = 'Working…';
      Promise.resolve().then(function () { return app().ResolveHold(id, action); }).then(function (info) {
        $('context-hold-status').textContent = '';
        // A compaction moves the held message to a new chat, which is opened beside this one.
        if (info && info.sessionId && info.sessionId !== id) window.dinahWorkbench.open({ kind: 'chat-session', workspace: workspace, path: info.sessionId });
      }, function (error) {
        $('context-hold-status').textContent = 'That could not be done: ' + String(error).replace(/^Error:\s*/, '');
      }).finally(function () { buttons.forEach(function (b) { b.disabled = false; }); });
    }
    function sessionNote() {
      var s = active && sessions[active];
      drawHold(s);
      var saved = $('saved-note');
      saved.hidden = !(s && s.saved && !s.info.open && !s.loading && !s.failed);
      if (!saved.hidden) $('saved-note-body').textContent = agentName() + ' reconnects by itself in a moment, or at once when you send a message, and then shows its thoughts and the details of its tools too.';
      var note = $('session-note');
      note.hidden = !(s && s.failed && !s.loading);
      if (note.hidden) { openWithShown = false; return; }
      $('session-note-title').textContent = agentName() + ' could not open this session.';
      $('session-note-body').textContent = agentName() + ' reported: ' + s.failed.replace(/^Error:\s*/, '') + (s.unoffered ? ' Open With lets you choose one of the values ' + agentName() + ' offers now.' : '') + (s.historyAvailable ? ' Your saved history is shown above. You can save it or continue it in a fresh chat.' : ' Try again to reconnect. No saved history is available here.') + (s.historyWarning ? ' ' + s.historyWarning : '');
      $('session-recover').disabled = !s.historyAvailable || s.historyRecoverable === false;
      $('session-save-history').disabled = !s.historyAvailable;
      drawOpenWith(s);
    }
    /* Open With… is offered when the chat was refused because the agent no
       longer offers its saved value of an option (option_substitutions.go).
       It lists the values the agent offers, a value whose name is the saved
       value's name or the saved value itself first, marked Suggested but
       never chosen for the person. Choosing one reopens the chat with it, and
       the chat keeps it as its own. With Always ticked, Dinah also stores the
       choice, and later reopenings of this provider's chats make it without
       asking while the saved value is not offered; Settings › Agents lists it
       with Remove. */
    var openWithShown = false;
    function drawOpenWith(s) {
      var u = s.unoffered, button = $('session-open-with'), panel = $('session-open-with-panel');
      button.hidden = !u;
      if (!u) openWithShown = false;
      panel.hidden = !openWithShown;
      button.setAttribute('aria-expanded', openWithShown ? 'true' : 'false');
      if (!u) return;
      var saved = valueWords(u.savedName, u.saved), option = (u.optionName || u.option).toLowerCase();
      $('session-open-with-help').textContent = 'The saved ' + option + ', ' + saved + ', is not offered. Choose what to open this chat with instead; the chat keeps your choice.';
      $('session-open-with-always-label').textContent = "Always Use This When '" + (u.savedName || valueText(u.saved)) + "' Isn't Offered";
      var list = $('session-open-with-list');
      list.replaceChildren.apply(list, u.offered.map(function (v) {
        var li = document.createElement('li'); li.className = 'hstack gap-3 align-center';
        var choose = document.createElement('button'); choose.type = 'button'; choose.className = 'btn btn-sm' + (v.suggested ? ' btn-primary' : '');
        choose.textContent = valueWords(v.name, v.value);
        choose.dataset.value = valueText(v.value);
        choose.addEventListener('click', function () {
          if (!active || sessions[active] !== s) return;
          s.openWith = { option: u.option, value: v.value, always: $('session-open-with-always').checked };
          s.failed = ''; s.unoffered = null; openWithShown = false;
          $('session-open-with-always').checked = false;
          openSession(active);
        });
        li.appendChild(choose);
        if (v.suggested) {
          var mark = document.createElement('span'); mark.className = 'form-help'; mark.textContent = 'Suggested, because its name matches the saved ' + option + '.';
          li.appendChild(mark);
        }
        return li;
      }));
    }
    $('session-open-with').addEventListener('click', function () {
      var s = active && sessions[active];
      if (!s || !s.unoffered) return;
      openWithShown = !openWithShown;
      drawOpenWith(s);
    });
    $('session-open-with-cancel').addEventListener('click', function () {
      var s = active && sessions[active];
      openWithShown = false;
      $('session-open-with-always').checked = false;
      if (s) drawOpenWith(s);
    });
    $('hold-set-aside').addEventListener('click', function () { resolveHold('set-aside'); });
    $('hold-compact').addEventListener('click', function () { resolveHold('compact'); });
    $('hold-new').addEventListener('click', function () { startNewChat(); });
    $('saved-reconnect').addEventListener('click', function () { if (active) reconnectChat(active, true).catch(function () {}); });
    $('session-retry').addEventListener('click', function () {
      if (active) { sessions[active].failed = ''; openSession(active); }
    });
    $('session-new').addEventListener('click', function () { startNewChat().catch(function () {}); });
    $('session-recover').addEventListener('click', function () { switchProvider(chatProvider(), true); });
    /* The failure of a switch is dismissed where it is said (switch.go). */
    (function () { var d = $('switch-progress').querySelector('.switch-dismiss'); d.addEventListener('click', function () {
      var sn = active && sessions[active];
      localSwitch = null; drawSwitchState(sn);
      if (sn && app().DismissSwitchFailure) Promise.resolve(app().DismissSwitchFailure(sn.id)).catch(fail);
    }); })();
    $('session-save-history').addEventListener('click', function () {
      var id = active;
      Promise.resolve(app().SavedConversation(id)).then(function (saved) {
        if (!saved || !saved.available) throw new Error('No saved history is available for this chat.');
        var lines = ['# ' + title(S(id)), '', saved.warning || 'Saved conversation history', ''];
        // A delivered message is its sender's, never the person's.
        (saved.messages || []).forEach(function (m) { lines.push('## ' + (m.role === 'you' ? 'You' : m.role === 'agent' ? providerName(m.provider) : m.role === 'delivered' ? deliveredHeading(m.delivered) + deliveredWhen(m.delivered) : m.role), '', m.text || '', ''); });
        return app().SaveRun(lines.join('\n'));
      }).then(function (path) { if (path) toast('Saved history to ' + path + '.'); }).catch(fail);
    });


    /* ---- The run log ------------------------------------------------------ */
    function now() { return dinahRegional.format(new Date(), 'clock'); }

    function row(s, cls) {
      var li = document.createElement('li');
      li.className = 'activity-row' + (cls ? ' ' + cls : '');
      var t = document.createElement('time');
      t.className = 'activity-time';
      // A row of a replayed history gets its time from the session's journal
      // when the replay ends; the time it is drawn at is not when it was said.
      if (!s.loading) dinahRegional.stamp(t, new Date(), 'clock');
      li.appendChild(t);
      s.log.appendChild(li);
      trimLog(s);
      if (s.loading && !s.drawingSaved) { s.replayed = (s.replayed || 0) + 1; replayProgress(s); }
      else if (s.id === active) { $('log-empty').hidden = true; scheduleDraw(); }
      return li;
    }

    /* A chat keeps its latest rows on the page and lets the oldest go, so that
       a long chat, or many of them, do not fill the window's memory. Nothing
       is lost: the chat's log on disk keeps every prompt, thought, tool call
       and reply, and a row at the top of the chat
       says how much left the page and opens the log. A fold of tool calls is
       one row. */
    var CHAT_MAX_ROWS = 600, CHAT_KEEP_ROWS = 450;
    function trimLog(s) {
      var rows = s.log.children;
      if (rows.length <= CHAT_MAX_ROWS) return;
      /* Letting rows go shrinks what is above the reader; while the reader
         has scrolled back, the view is held to the row they are at
         (chat-text.js, keepPlace), rather than left to the browser's
         anchoring, which can give it up (Paul, 30 October 2026). */
      keepChatPlace(function () {
        var marker = trimmedRow(s);
        var li = marker ? marker.nextElementSibling : s.log.firstElementChild;
        var drop = rows.length - CHAT_KEEP_ROWS;
        while (drop-- > 0 && li) {
          var next = li.nextElementSibling;
          forgetRow(s, li);
          li.remove();
          s.trimmed = (s.trimmed || 0) + 1;
          li = next;
        }
        trimMarker(s);
      });
    }
    function trimmedRow(s) {
      var first = s.log.firstElementChild;
      return first && first.classList.contains('trimmed-row') ? first : null;
    }
    /* The row at the top of a chat that says how many entries have left the
       page, or are not drawn, and opens the chat's log. */
    function trimMarker(s) {
      if (!s.trimmed) return;
      var marker = trimmedRow(s);
      if (!marker) {
        marker = document.createElement('li');
        marker.className = 'activity-row trimmed-row';
        marker.setAttribute('data-scroll-anchor-skip', '');
        marker.innerHTML = '<time class="activity-time"></time><span class="activity-muted trimmed-words"></span>';
        var open = document.createElement('button');
        open.type = 'button'; open.className = 'btn btn-sm'; open.textContent = 'Show the Chat Log';
        open.addEventListener('click', function () {
          Promise.resolve().then(function () { return app().ShowChatLog(s.id); }).catch(function (err) { toast(String(err && err.message || err), 'warn'); });
        });
        marker.appendChild(open);
        s.log.insertBefore(marker, s.log.firstElementChild);
      }
      marker.querySelector('.trimmed-words').textContent = s.trimmed + (s.trimmed === 1 ? ' earlier entry of this chat is' : ' earlier entries of this chat are') +
        ' no longer shown here. The chat\'s log keeps all of them.';
    }
    function forgetRow(s, li) {
      var all = [li].concat(Array.prototype.slice.call(li.querySelectorAll('li')));
      all.forEach(function (x) {
        if (x._tool) Object.keys(s.rows).forEach(function (k) { if (s.rows[k] === x._tool) delete s.rows[k]; });
        if (s.current && s.current.li === x) s.current = null;
        dirty.forEach(function (m) { if (m.li === x) dirty.delete(m); });
      });
    }

    /* What a message's buttons do. Edit puts a message of the person's back in
       the box, with its document, to change and send again; the new message
       goes at the end, since the protocol cannot yet rewind a session. Quote
       puts Devin's words in the box as a quotation. */
    function boxEnd() { var box = $('prompt'); box.focus(); box.setSelectionRange(box.value.length, box.value.length); typed(); }
    function messageAction(act, m) {
      var box = $('prompt');
      if (act === 'edit') {
        var t = m.raw != null ? m.raw : m.text.replace(/\n\n\*(Document|Attached): [^\n]*\*$/g, '').replace(/\n\n\*(Document|Attached): [^\n]*\*$/g, '');
        if (skillForm.isOpen()) closeForm(false);
        box.value = t;
        if (m.doc) setContext(m.doc);
        boxEnd();
      } else if (act === 'quote') {
        window.dinahChatText.quoteInto(box, m.text);
        typed();
      }
    }

    /* The last message the person sent stays pinned at the top of the run
       once it has scrolled out of sight: its first lines, with buttons to see
       it whole, copy it, or go back to it. */
    function pinCheck() {
      var s = active && sessions[active], pin = $('pin');
      var m = s && s.lastYou;
      if (!m || !m.li.isConnected || s.loading) { pin.hidden = true; return; }
      var body = $('activity'), top = body.getBoundingClientRect().top;
      var gone = m.li.getBoundingClientRect().bottom < top + 4;
      if (gone && pin.dataset.text !== m.text) {
        pin.dataset.text = m.text;
        $('pin-text').textContent = m.text.trim();
        pin.classList.remove('open');
        $('pin-expand').setAttribute('aria-expanded', 'false');
      }
      pin.hidden = !gone;
      if (gone) $('pin-expand').hidden = $('pin-text').scrollHeight <= $('pin-text').clientHeight + 2 && !pin.classList.contains('open');
    }
    $('activity').addEventListener('scroll', pinCheck, { passive: true });
    $('pin-expand').addEventListener('click', function () {
      var open = !$('pin').classList.contains('open');
      $('pin').classList.toggle('open', open);
      $('pin-expand').setAttribute('aria-expanded', String(open));
      $('pin-expand').title = open ? 'Show less' : 'Show the whole message';
    });
    $('pin-copy').addEventListener('click', function () { copyText($('pin').dataset.text.trim(), 'Your message is copied.'); });
    $('pin-go').addEventListener('click', function () {
      var s = active && sessions[active];
      if (s && s.lastYou) {
        navigation.pause();
        s.lastYou.li.scrollIntoView({ block: 'start', behavior: 'smooth' });
      }
    });

    /* The run sticks to its end while the reader is at it, however much is
       added at once; force takes it there whatever. Whether the reader is at
       the end is decided by their own scrolling, not measured after new
       content arrives, since a long message would put the end out of reach of
       any threshold. Scrolling back into the history stops the following and
       shows Jump to the Latest; scrolling to the end, or the button, starts it
       again. */
    /* A scroll event is delivered a frame after the scroll, by which time more
       text may have arrived, so the event of the run's own move to the end is
       recognised by the position it left, and only a position the run did not
       set is the reader's. */
    var following = true;
    var navigation = window.dinahTranscript.mount({ root: root, log: $('activity'), content: $('logs'), jump: $('jump-end'),
      ready: function () { var s = active && sessions[active]; return !s || !s.loading && !s.settling; },
      canTail: function () { var s = active && sessions[active]; return !deleted && (!s || !s.loading && !s.settling); },
      followingChanged: function (value) { following = value; syncAnchor(); },
      scrolled: pinCheck,
      entries: function () {
        var s = active && sessions[active]; if (!s) return [];
        return Array.from(s.log.children).filter(function (row) { return !!row._msg; }).map(function (row) {
          var element = row;
          if (row.classList.contains('team-traffic-original')) {
            var previous = row.previousElementSibling;
            while (previous && previous.classList.contains('team-traffic-original')) previous = previous.previousElementSibling;
            if (previous && previous.classList.contains('team-traffic-fold')) element = previous;
          }
          return { identity: row, element: element, eligible: row._msg.kind !== 'you' && row._msg.label !== 'You' };
        });
      }
    });
    /* While the run follows its end the log does not let the browser anchor its view (CSS overflow-anchor,
       from the Scroll Anchoring specification), so the browser never moves the view as content above it
       changes size; the run keeps the end in view itself. A move away from the end is then the reader's,
       or a saved position being put back, and following stops. A reader who has scrolled away keeps the
       browser's anchoring, which holds their place while content above them changes or is trimmed. */
    /* While the browser's anchoring helps nobody once the page holds the
       place itself, it is off whether the run follows or the reader reads
       back; the page's own capture never gives the reader away (Paul, 30
       October 2026). And read-back keeps every row's real height under it,
       so a row the browser never painted yet estimates none of them away:
       the same preservation a reopened chat asks for holds while its reader
       is back (Dinah UI's suggestion 2, message 1070). */
    // The chat's following state stays authoritative while new rows arrive.
    // The strip and traffic folds share the same navigation choice.
    function keepChatPlace(run, deferTail) { return navigation.keep(run, deferTail); }
    function syncAnchor() {
      $('activity').style.overflowAnchor = 'none';
      $('activity').toggleAttribute('data-preserve-scroll-layout', !following);
      // Once read back, this bounded log keeps its measured rows. Returning
      // to estimates during a jump changes the extent underneath its target.
      var s = active && sessions[active];
      if (!following && s) s.log.setAttribute('data-preserve-scroll-layout', '');
    }
    var appearanceAnchor = null;
    function captureAppearance() {
      var body = $('activity');
      if (deleted || !body.clientHeight) { appearanceAnchor = null; return; }
      var top = body.getBoundingClientRect().top;
      var first = Array.from(body.querySelectorAll('.msg-text p, .msg-text pre, .msg-text li')).find(function (row) { return row.getBoundingClientRect().height && row.getBoundingClientRect().bottom > top; });
      if (!first) first = Array.from(body.querySelectorAll('.activity-row')).find(function (row) { return row.getBoundingClientRect().height && row.getBoundingClientRect().bottom > top; });
      appearanceAnchor = { row: first, offset: first ? first.getBoundingClientRect().top - top : 0, following: following };
    }
    function restoreAppearance() {
      if (deleted) return;
      var body = $('activity'), anchor = appearanceAnchor;
      if (!body.clientHeight) return;
      if (anchor && !anchor.following && anchor.row && anchor.row.isConnected) {
        navigation.position({ following: false, top: body.scrollTop + anchor.row.getBoundingClientRect().top - body.getBoundingClientRect().top - anchor.offset });
      } else if (following) queueTail();
      scheduleDraw();
    }
    function cancelAppearanceAnchor() { appearanceAnchor = null; }
    document.addEventListener('dinah:appearance-before', captureAppearance);
    document.addEventListener('dinah:appearance-change', restoreAppearance);
    ['wheel', 'touchstart', 'keydown'].forEach(function (name) { $('activity').addEventListener(name, cancelAppearanceAnchor, { passive: true }); });

    function scroll(force) { navigation.scroll(force); }
    function queueTail() { navigation.queueTail(); }


    /* A message is the shared card (chat-text.js): the person's offers Edit
       and the agent's Quote, and both offer Copy. */
    function message(s, who, text, label) {
      var li = row(s, 'msg-row');
      var m = window.dinahChatText.card({ you: who === 'you', text: text, label: label || (who === 'you' ? 'You' : providerName(sessionProvider(s))), act: messageAction });
      li.appendChild(m.div);
      m.kind = who; m.li = li; m.s = s; m.cwd = function () { return chatFolder(s); };
      dirty.add(m);
      if (!s.loading) scheduleDraw();
      li._msg = m;
      if (who === 'you') { s.lastYou = m; if (s.id === active) pinCheck(); }
      return m;
    }

    /* A message the bus delivered into the chat (a reply to its question,
       another agent's message, Paul's, a release notice or a verdict) is a
       card of its own, not one of the person's messages (delivered.go). Its
       head says what it is and who sent it, a result's status as a glyph and
       a word; its body is the message as Markdown, folded behind PUDL's
       disclosure when it is long. The row's time is when it was sent. A live
       delivery, the agent's replay and the saved copy all draw it here. */
    var DELIVERED_FOLD_CHARS = 1200, DELIVERED_FOLD_LINES = 16;
    var DELIVERED_STATUS = { done: 'positive', failed: 'danger', cancelled: 'warn', timeout: 'warn' };
    function deliveredWhen(d) {
      var at = d && d.time ? new Date(d.time) : null;
      return at && !isNaN(at) ? ', sent ' + dinahRegional.format(at, 'dateTime') : '';
    }
    function deliveredCard(s, entry) {
      var d = entry.delivered || {}, text = entry.text || '';
      s.current = null;
      var li = row(s, 'delivered-row');
      // The card of the same message that said it was arriving gives its place to this one.
      var waiting = d.id != null && s.arriving && s.arriving[d.id];
      if (waiting) { waiting.replaceWith(li); delete s.arriving[d.id]; tidyArriving(s); }
      var at = d.time ? new Date(d.time) : null, t = li.querySelector('.activity-time');
      if (at && !isNaN(at)) dinahRegional.stamp(t, at, 'clock');
      var long = text.length > DELIVERED_FOLD_CHARS || text.split('\n').length > DELIVERED_FOLD_LINES;
      var card = document.createElement('section');
      card.className = 'card disclosure delivered-card';
      card.dataset.kind = d.kind || 'text';
      card.innerHTML = '<div class="card-head"><button type="button" class="card-toggle" aria-expanded="' + !long + '"></button>' +
        '<span class="card-title delivered-heading"></span><span class="card-summary delivered-summary"></span></div><div class="card-body msg-text delivered-text"></div>';
      var heading = deliveredHeading(d);
      card.querySelector('.delivered-heading').textContent = heading;
      if (d.status) {
        var badge = document.createElement('span');
        badge.className = 'badge ' + (DELIVERED_STATUS[d.status] || '');
        badge.textContent = d.status;
        card.querySelector('.card-title').after(badge);
      }
      var first = text.trim().split('\n').find(function (l) { return l.trim(); }) || '';
      card.querySelector('.delivered-summary').textContent = first.length > 160 ? first.slice(0, 160) + '…' : first;
      li.appendChild(card);
      var m = { kind: 'delivered', li: li, text: text, el: card.querySelector('.delivered-text'), label: heading, s: s, cwd: function () { return chatFolder(s); } };
      li._msg = m; li._delivered = d;
      dirty.add(m);
      if (!s.loading) scheduleDraw();
      return m;
    }

    function teamSentCard(s, entry) {
      var m = deliveredCard(s, entry), d = entry.delivered || {};
      m.label = 'Sent to ' + (d.roomTitle || 'the team chat');
      m.li.querySelector('.delivered-heading').textContent = m.label;
      m.li._teamTraffic = d;
      return m;
    }

    /* A message waiting for a chat that is still restoring or replaying is shown at
       once as arriving: a card at the end of the transcript, where the delivered
       message will land and replace it. It is display, not part of the
       conversation, so it says Waiting in words and a badge and has a dashed edge
       (arriving.go announces each message once; only the first 400 characters
       come, and a count of the rest). While the replay hides the transcript, the
       progress line says how many are waiting. */
    function arrivingFrom(m) {
      return speakerName(m.from) + (m.workspace ? ' (workspace ' + m.workspace + ')' : '') + (m.group ? ', in the group ' + m.group : '');
    }
    function waitingCount(s) { return Object.keys(s.arriving || {}).length + (s.arrivingMore || 0); }
    function arrivingCard(s, msg) {
      s.arriving = s.arriving || {};
      if (s.arriving[msg.id]) return;
      var li = document.createElement('li');
      li.className = 'activity-row arriving-row'; li.dataset.arriving = String(msg.id);
      var card = document.createElement('section');
      card.className = 'card arriving-card';
      card.innerHTML = '<div class="card-head"><span class="badge warn">Waiting</span><span class="card-title arriving-heading"></span></div><div class="card-body msg-text arriving-text"></div>';
      card.querySelector('.arriving-heading').textContent = 'Arriving: message from ' + arrivingFrom(msg);
      card.querySelector('.arriving-text').textContent = msg.text || '';
      li.appendChild(card);
      s.log.appendChild(li);
      s.arriving[msg.id] = li;
    }
    // The line after the cards that counts the waiting messages not described.
    function arrivingMore(s) {
      if (s.arrivingMoreLi) { s.arrivingMoreLi.remove(); s.arrivingMoreLi = null; }
      if (!s.arrivingMore) return;
      var li = document.createElement('li');
      li.className = 'activity-row arriving-row arriving-more';
      var p = document.createElement('p'); p.className = 'activity-muted';
      p.textContent = s.arrivingMore === 1 ? 'One more message is waiting.' : s.arrivingMore + ' more messages are waiting.';
      li.appendChild(p); s.log.appendChild(li); s.arrivingMoreLi = li;
    }
    function tidyArriving(s) {
      if (!Object.keys(s.arriving || {}).length && s.arrivingMoreLi) { s.arrivingMoreLi.remove(); s.arrivingMoreLi = null; s.arrivingMore = 0; }
      if (s.loading) replayProgress(s);
    }
    on('agent:arriving', function (m) {
      var s = S(m.sessionId);
      (m.messages || []).forEach(function (msg) { arrivingCard(s, msg); });
      s.arrivingMore = m.more || 0;
      arrivingMore(s);
      if (s.loading) replayProgress(s);
      else if (s.id === active) { $('log-empty').hidden = true; scroll(); }
    });

    /* A message grows chunk by chunk. Drawing it again on every chunk costs
       time in proportion to its length, which made replaying a long session
       slow, so a message is drawn at most once a frame while live, and not at
       all while a session's history is being replayed. When replay ends,
       its dirty entries enter the shared frame queue. */
    var dirty = new Set(), dirtyTools = new Set(), dirtyGroups = new Set(), dirtyPlans = new Set(), renderKey = {}, renderReady = [];
    function scheduleDraw() {
      /* The frame's work changes row heights under a scrolled-back reader as
         surely as a trim does; the page holds the place (Paul, 30 October
         2026), except following at the end, where nothing is held. */
      if (!deleted) window.dinahRender.enqueue(renderKey, root, function (deadline) {
        return keepChatPlace(function () { return flush(deadline); }, true);
      }, function () {
        if (active && sessions[active] && !sessions[active].loading) scroll();
      }, 'chat output');
    }
    /* A thought is plain text. Any other message is drawn by the shared
       renderer (chat-text.js), which draws only the block still arriving
       while a message streams in and shows the foot once it is long. */
    function draw(m) {
      if (m.kind === 'thought') window.dinahBoundedText.draw(m.el, m.text, function (text) { m.el.textContent = text; });
      else window.dinahChatText.draw(m);
      drawMessageImages(m);
    }

    function drawMessageImages(m) {
      if (!m.images || !m.images.length) return;
      var old = m.el.querySelector('.sent-images'); if (old) old.remove();
      var shown = m.images.filter(function (image) { return /^image\/(png|jpeg|gif|webp|bmp)$/i.test(image.mimeType || '') && image.data; });
      m.el.append(window.dinahChatText.gallery(shown.map(function (image) { return { name: image.name, src: 'data:' + image.mimeType + ';base64,' + image.data }; })));
      if (window.dinahMediaViewer) dinahMediaViewer.enhance(m.el);
    }

    /* A path in a message is read from the chat's own workspace, both when
       the message is drawn and its paths are linked, and when a link is
       pressed. */
    function chatFolder(s) { return s && s.info && s.info.cwd || workspace; }
    window.dinahChatText.openLinks($('logs'), function () { return chatFolder(active && sessions[active]); });
    function flush(deadline) {
      var queues = [dirty, dirtyTools, dirtyGroups, dirtyPlans], didWork = false;
      for (var i = 0; i < queues.length; i++) {
        var entries = queues[i].values(), next;
        while (!(next = entries.next()).done) {
          var entry = next.value, s = entry.s || entry._session;
          var kept = s.keptLog && s.keptLog.contains(entry.li || entry);
          if (!s.log.contains(entry.li || entry) && !kept) { queues[i].delete(entry); continue; }
          if (s.loading && !kept) continue;
          if (didWork && performance.now() >= deadline) return true;
          queues[i].delete(entry);
          if (i === 0) draw(entry);
          else if (i === 1) drawTool(entry);
          else if (i === 2) summarize(entry);
          else drawPlan(entry);
          didWork = true;
        }
      }
      var ready = renderReady; renderReady = [];
      ready.forEach(function (fn) { fn(); });
      return false;
    }
    function drawTool(r) {
      var st = STATUS[r.status] || ['', r.status];
      r.title.textContent = r.name;
      r.badge.className = 'badge ' + st[0];
      r.badge.textContent = st[1];
      if (r.drawnText !== r.text) {
        r.pre.innerHTML = ansiHTML(r.text);
        r.detail.hidden = !r.text;
        r.drawnText = r.text;
      }
    }
    function appendChunk(s, kind, text) {
      if (!s.current || s.current.kind !== kind) {
        if (kind === 'agent') s.current = message(s, 'agent', '');
        else if (kind === 'you') s.current = message(s, 'you', '');
        else {
          var li = row(s, 'thought-row');
          var span = document.createElement('span');
          span.className = 'thought';
          li.appendChild(span);
          s.current = { kind: 'thought', li: li, text: '', el: span };
          li._thought = s.current;
        }
        s.current.s = s;
      }
      s.current.text += text;
      dirty.add(s.current);
      if (s.loading) { if (!s.drawingSaved) { s.replayed = (s.replayed || 0) + 1; replayProgress(s); } return; }
      scheduleDraw();
    }
    function drawAll(s, ready) { if (ready) renderReady.push(ready); scheduleDraw(); }

    var STATUS = {
      pending: ['', 'waiting'],
      in_progress: ['accent running', 'running'],
      completed: ['positive', 'done'],
      failed: ['danger', 'failed']
    };

    function contentText(content) {
      var parts = [];
      (content || []).forEach(function (c) {
        if (c.type === 'diff') parts.push('Changed ' + c.path);
        else if (c.type === 'content' && c.content) {
          if (c.content.type === 'text') parts.push(c.content.text);
          else if (c.content.type === 'resource' && c.content.resource && c.content.resource.text) parts.push(c.content.resource.text);
        } else if (c.type === 'terminal') parts.push('Terminal ' + c.terminalId);
      });
      return parts.join('\n');
    }

    /* A command's output may hold the control sequences a terminal reads
       (ECMA-48): PowerShell 7 colours a table's header with ESC[32m, for one.
       The chat draws the colour and weight sequences (SGR, ESC[...m) as styled
       text and leaves the rest out (other CSI sequences, OSC titles and
       links, stray ESC and other C0 controls but tab and new line), so the
       reader sees the output as a terminal would show it. */
    var ANSI_FG = [30, 31, 32, 33, 34, 35, 36, 37, 90, 91, 92, 93, 94, 95, 96, 97];
    function ansiHTML(text) {
      var out = '', open = false, style = { bold: false, dim: false, italic: false, under: false, fg: null, bg: null };
      function esc(s) { return s.replace(/[&<>]/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]; }); }
      function apply(params) {
        var p = params.length ? params.split(';').map(function (n) { return n === '' ? 0 : Number(n); }) : [0];
        for (var i = 0; i < p.length; i++) {
          var n = p[i];
          if (n === 0) style = { bold: false, dim: false, italic: false, under: false, fg: null, bg: null };
          else if (n === 1) style.bold = true;
          else if (n === 2) style.dim = true;
          else if (n === 3) style.italic = true;
          else if (n === 4) style.under = true;
          else if (n === 22) { style.bold = false; style.dim = false; }
          else if (n === 23) style.italic = false;
          else if (n === 24) style.under = false;
          else if (n === 39) style.fg = null;
          else if (n === 49) style.bg = null;
          else if (ANSI_FG.indexOf(n) >= 0) style.fg = ANSI_FG.indexOf(n);
          else if (ANSI_FG.indexOf(n - 10) >= 0) style.bg = ANSI_FG.indexOf(n - 10);
          // 256-colour and true-colour forms take their own parameters; the
          // chat keeps to the sixteen colours and skips them.
          else if (n === 38 || n === 48) i += p[i + 1] === 5 ? 2 : p[i + 1] === 2 ? 4 : 1;
        }
      }
      function span() {
        var cls = [];
        if (style.bold) cls.push('ansi-b');
        if (style.dim) cls.push('ansi-d');
        if (style.italic) cls.push('ansi-i');
        if (style.under) cls.push('ansi-u');
        if (style.fg !== null) cls.push('ansi-fg' + style.fg);
        if (style.bg !== null) cls.push('ansi-bg' + style.bg);
        if (open) out += '</span>';
        open = cls.length > 0;
        if (open) out += '<span class="' + cls.join(' ') + '">';
      }
      var re = /\x1b\[([0-9;?]*)([@-~])|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-_]?|[\x00-\x08\x0b-\x1f\x7f]/g, last = 0, m;
      while ((m = re.exec(text))) {
        out += esc(text.slice(last, m.index));
        last = re.lastIndex;
        if (m[2] === 'm' && m[1].indexOf('?') < 0) { apply(m[1]); span(); }
      }
      out += esc(text.slice(last));
      if (open) out += '</span>';
      return out;
    }

    var TOOL_TEXT_MAX = 32 * 1024;
    /* Whether an update gives a tool call anything a row could show: a title,
       a kind, output, a command, or a failure. An update for a call the chat
       has not seen that gives none of these draws no row until one that does
       arrives. The Claude adapter keeps the Agent tool call that starts a
       subagent out of the transcript, since the subagent's own row stands for
       it, but an update for that call with nothing in it still reached the
       chat and drew an empty row reading "waiting" under each subagent's row,
       which never changed (28 October 2026). */
    function toolShows(u, text) {
      return !!(u.title || u.kind || text || (u.rawInput && u.rawInput.command) || u.status === 'failed');
    }
    function toolRow(s, u) {
      var r = s.rows[u.toolCallId];
      if (!r && !toolShows(u, contentText(u.content))) return null;
      var groupChanged = !r;
      if (!r) {
        s.current = null;
        var li = row(s, 'tool-row');
        li.innerHTML += '<span><span class="tool-title"></span><span class="badge"></span></span><details class="tool-detail" hidden><summary>Details</summary><pre></pre></details>';
        r = s.rows[u.toolCallId] = { s: s, li: li, title: li.querySelector('.tool-title'), name: '', badge: li.querySelector('.badge'), detail: li.querySelector('.tool-detail'), pre: li.querySelector('pre'), text: '', kind: '', status: 'pending' };
        li._tool = r;
        group(s, li);
      }
      if (u.kind && u.kind !== r.kind) { r.kind = u.kind; groupChanged = true; }
      if (u.title && u.title !== r.name) { r.name = u.title; groupChanged = true; }
      if (u.status && u.status !== r.status) {
        r.status = u.status;
        groupChanged = true;
        // A step that fails in a live turn is told when the turn ends; one
        // replayed from history is not news, and one the person rejected is
        // already known to them.
        if (u.status === 'failed' && !s.loading && !r.rejected && !r.told) {
          r.told = true;
          (s.failures = s.failures || []).push(r);
        }
      }
      if (groupChanged) dirtyTools.add(r);
      if (r.li._group && groupChanged) dirtyGroups.add(r.li._group);
      if (u.rawInput && u.rawInput.command) r.command = u.rawInput.command;
      var t = contentText(u.content);
      if (!t && r.command && !r.text) t = r.command;
      if (t && !r.cut) {
        r.text = r.text ? r.text + '\n' + t : t;
        // A tool's output on the page is kept to its first part; the chat's
        // log has the rest.
        if (r.text.length > TOOL_TEXT_MAX) {
          r.text = r.text.slice(0, TOOL_TEXT_MAX) + '\n\n[The rest of this output is in the chat\'s log.]';
          r.cut = true;
        }
        dirtyTools.add(r);
      }
      // A replayed history is drawn once the replay ends; the log is hidden
      // until then, and keeping its end in view row by row would lay the
      // page out for every row.
      if (!s.loading && (dirtyTools.size || dirtyGroups.size || groupChanged)) scheduleDraw();
      return r;
    }

    /* A run of tool calls with nothing between them folds into one line that
       counts them by kind ("Read 6 files · Ran 2 commands · 1 failed") and,
       while the run is live, names the step in hand. Choosing the line shows
       the calls. The kinds are the protocol's tool kinds
       (https://agentclientprotocol.com/protocol/tool-calls). */
    var KIND_WORDS = {
      read: ['Read 1 file', 'Read {n} files'], edit: ['Edited 1 file', 'Edited {n} files'], 'delete': ['Deleted 1 file', 'Deleted {n} files'],
      move: ['Moved 1 file', 'Moved {n} files'], search: ['Searched once', 'Searched {n} times'], execute: ['Ran 1 command', 'Ran {n} commands'],
      think: ['Thought once', 'Thought {n} times'], fetch: ['Fetched 1 page', 'Fetched {n} pages'], other: ['1 other step', '{n} other steps']
    };
    function group(s, li) {
      var prev = li.previousElementSibling;
      if (prev && prev.classList.contains('tool-group')) {
        prev._list.appendChild(li);
        li._group = prev;
        return;
      }
      if (!prev || !prev.classList.contains('tool-row')) return;
      /* The folded run is a PUDL disclosure card (pudl-disclose.js): a raised
         toggle at the start of a flat head that counts the calls, and the calls
         themselves in the body while it is open. */
      var g = document.createElement('li');
      g.className = 'activity-row tool-group';
      g.innerHTML = '<time class="activity-time"></time><div class="card disclosure tool-group-body"><div class="card-head">' +
        '<button type="button" class="card-toggle" aria-expanded="false" aria-label="Show the steps"></button>' +
        '<span class="card-title tool-group-words"></span><span class="card-summary tool-group-now"></span></div>' +
        '<ol class="card-body tool-group-list"></ol></div>';
      g.querySelector('time').textContent = prev.querySelector('.activity-time').textContent;
      g._list = g.querySelector('.tool-group-list');
      g._session = s;
      g._open = function () { if (window.pudlDisclose) window.pudlDisclose.set(g.querySelector('.card'), true); };
      s.log.insertBefore(g, prev);
      g._list.appendChild(prev);
      g._list.appendChild(li);
      prev._group = g;
      li._group = g;
    }
    function summarize(g) {
      var counts = {}, order = [], failed = 0, live = null;
      Array.prototype.forEach.call(g._list.children, function (li) {
        var r = li._tool;
        if (!r) return;
        var k = KIND_WORDS[r.kind] ? r.kind : 'other';
        if (!counts[k]) { counts[k] = 0; order.push(k); }
        counts[k]++;
        if (r.status === 'failed') failed++;
        if (r.status === 'in_progress' || r.status === 'pending') live = r;
      });
      var words = order.map(function (k) { var w = KIND_WORDS[k]; return counts[k] === 1 ? w[0] : w[1].replace('{n}', counts[k]); });
      var el = g.querySelector('.tool-group-words');
      el.textContent = words.join(' · ');
      if (failed) {
        var f = document.createElement('span');
        f.className = 'tool-group-failed';
        f.textContent = ' · ' + failed + ' failed';
        el.appendChild(f);
      }
      // The step in hand is the card's summary, shown while the card is closed.
      g.querySelector('.tool-group-now').textContent = live ? 'now: ' + live.name : '';
      g.classList.toggle('has-failed', failed > 0);
    }

    function planRow(s, entries) {
      var r = s.rows.__plan;
      if (!r) {
        var li = row(s, 'plan-row');
        li.innerHTML += '<span><strong>Plan</strong><ol class="plan"></ol></span>';
        r = s.rows.__plan = { s: s, li: li, list: li.querySelector('.plan') };
      }
      r.entries = entries || [];
      dirtyPlans.add(r);
      if (!s.loading) scheduleDraw();
    }
    /* The agent handed work to a subagent: a row where it happened, kept
       current as the subagent runs and ends (drawSubagentRow). */
    function subagentRow(s, u) {
      if (!u.subagentSessionId) return;
      s.current = null;
      var li = row(s, 'subagent-row');
      li.dataset.subagent = u.subagentSessionId;
      drawSubagentRow(li, s, subagentOf(s, u.subagentSessionId) || { id: u.subagentSessionId, name: u.name, task: u.task, state: 'running' });
    }
    function drawPlan(r) {
      r.list.innerHTML = '';
      r.entries.forEach(function (e) {
        var item = document.createElement('li');
        var st = e.status === 'completed' ? 'done' : e.status === 'in_progress' ? 'now' : '';
        item.textContent = e.content + (st ? ' (' + st + ')' : '');
        r.list.appendChild(item);
      });
    }

    /* A replayed history arrives in batches, each update handled as if it
       had come alone (session.go, flushReplay). */
    on('agent:updates', function (m) {
      (m.updates || []).forEach(function (u) {
        var one = { sessionId: m.sessionId, update: u };
        events.dispatch('agent:update', one);
      });
    });
    on('agent:update', function (m) {
      var s = S(m.sessionId), u = m.update || {};
      switch (u.sessionUpdate) {
        case 'agent_message_chunk': if (u.content && u.content.type === 'text') appendChunk(s, 'agent', u.content.text); break;
        case 'agent_thought_chunk': if (u.content && u.content.type === 'text') appendChunk(s, 'thought', u.content.text); break;
        // A user message arrives only when Devin replays a session's history.
        case 'user_message_chunk':
          if (u.content && u.content.type === 'text') appendChunk(s, 'you', u.content.text);
          else if (u.content && u.content.type === 'image') { if (!s.current || s.current.kind !== 'you') s.current = message(s, 'you', ''); (s.current.images || (s.current.images = [])).push(u.content); dirty.add(s.current); if (!s.loading) scheduleDraw(); }
          break;
        case 'tool_call':
        case 'tool_call_update': toolRow(s, u); break;
        // A message the bus delivered, from a replay Dinah recognised or from
        // the saved copy (delivered.go).
        case 'dinah_delivered': if (u.message) deliveredCard(s, u.message); break;
        case 'dinah_team_sent': if (u.message) teamSentCard(s, u.message); break;
        // What Dinah did that the chat should say: turns set aside, a window nearly full (dinah_status, numbered like
        // any other update, so it is drawn once whenever the page gets it).
        case 'dinah_status': if (u.text) statusRow(s, u.text, u.kind); break;
        case 'plan': planRow(s, u.entries); break;
        case 'subagent_spawned': subagentRow(s, u); break;
        case 'usage_update':
          s.usage = { used: u.used, size: u.size, sizeKind: u.sizeKind || '', windowKnown: u.windowKnown, cost: u.cost || null };
          if (s.id === active) window.dinahRender.enqueue(drawUsage, root, drawUsage);
          break;
      }
    });

    /* ---- Context and cost -------------------------------------------------
       Devin reports how much of the model's context a session uses, and may
       report what it has cost so far (ACP's usage_update, which is stable).
       The page shows the context as a small meter over the prompt, and each
       turn's length and cost on the line that ends it. The protocol's
       per-turn token counts are still a draft, so they are not used. */
    /* The price of the chat's model, under Usage, when Devin's command line
       lists one; the section is hidden otherwise. */
    function drawModelPriceUsage() {
      var model = byCat(barOptions(), 'model');
      var price = model && typeof model.currentValue === 'string' ? modelPrice(chatProvider(), model.currentValue) : null;
      var host = $('model-price');
      host.replaceChildren();
      host.hidden = $('model-price-title').hidden = !price;
      if (!price) return;
      [['Model', valueName(model, model.currentValue)], ['Price', price.costSummary]].forEach(function (pair) {
        var p = document.createElement('p');
        p.className = 'usage-line';
        p.innerHTML = '<span class="usage-label"></span><span class="usage-value"></span>';
        p.firstChild.textContent = pair[0];
        p.lastChild.textContent = pair[1];
        host.appendChild(p);
      });
      var more = [];
      if (typeof price.costTier === 'string' && price.costTier) more.push(price.costTier);
      if (contextWords(price.contextTokens)) more.push(contextWords(price.contextTokens) + ' context');
      var note = document.createElement('p'); note.className = 'usage-note';
      note.textContent = (more.length ? more.join(' · ') + '. ' : '') + 'Prices come from ' + providerName(chatProvider()) + '\'s command line.';
      host.appendChild(note);
    }
    function drawUsage() {
      var s = active && sessions[active], u = s && s.usage;
      $('usage-button').hidden = false;
      drawModelPriceUsage();
      var host = $('usage-detail');
      host.innerHTML = '';
      function line(label, value) {
        var p = document.createElement('p');
        p.className = 'usage-line';
        p.innerHTML = '<span class="usage-label"></span><span class="usage-value"></span>';
        p.firstChild.textContent = label;
        p.lastChild.textContent = value;
        host.appendChild(p);
      }
      if (!u || !finite(u.used) || !finite(u.size) || !(u.size > 0)) {
        $('usage-fill').style.width = '0%';
        $('usage-button').classList.remove('usage-high');
        $('usage-words').textContent = 'Usage';
        $('usage-button').title = 'Context has not been reported. Open account usage for ' + agentName() + '.';
        line('Context', 'Nothing reported yet'); return;
      }
      /* The engine says whether the window is known (windowKnown): a size the person chose is, and so is the model's
         own for a model whose default is its maximum. A size that is only the model's limit is not the length the
         server runs with, which the provider gives no way to read, and is said as that. Over is for a known window.
         Only an explicit false is unknown: Codex and Claude report a window they know and send no field. */
      var known = u.windowKnown !== false;
      var pct = contentPct(u), over = known && u.used > u.size, modelLimit = !known;
      var of = modelLimit ? 'the model\'s limit of ' + num(u.size) + ' tokens' : (u.sizeKind === 'model' ? 'the model\'s context window of ' : 'the context length of ') + num(u.size) + ' tokens';
      $('usage-fill').style.width = Math.min(100, pct) + '%';
      $('usage-button').classList.toggle('usage-high', pct >= 80);
      $('usage-words').textContent = over ? 'Over' : pct + '%';
      $('usage-button').title = (over ? 'The last request, ' + num(u.used) + ' tokens, was over ' + of : 'The session uses ' + pct + '% of ' + of) + (modelLimit ? '; the server\'s real size may be lower' : '') + '. Press for more.';
      line('Context', over ? num(u.used) + ' tokens, over the context length of ' + num(u.size) : num(u.used) + ' of ' + num(u.size) + ' tokens (' + pct + '%)');
      if (modelLimit) {
        var limitNote = document.createElement('p'); limitNote.className = 'usage-note';
        limitNote.textContent = 'The size is the model\'s own limit. The length the server runs with may be lower, and ' + providerName(chatProvider()) + ' does not say what it is.';
        host.appendChild(limitNote);
      }
      if (s.lastTurn != null) line('Last turn', duration(s.lastTurn));
      if (u.cost) {
        // A turn a subscription pays for costs nothing more; the agent's
        // figure is then what it would have cost at API prices.
        var plan = subscriptionOf(s);
        if (plan) {
          line('At API prices', money(u.cost));
          var note = document.createElement('p'); note.className = 'usage-note';
          note.textContent = 'Covered by your ' + plan + ' plan, so this chat has cost you nothing beyond it. The figure is what the same work would cost with an API key.';
          host.appendChild(note);
        } else line('Cost so far', money(u.cost));
      }
    }
    /* The plan a provider's account is on, when a subscription pays for its
       turns (provider_account.go), or ''. */
    var accounts = {};
    function subscriptionOf(s) {
      var acct = accounts[sessionProvider(s)];
      return acct && acct.subscription ? acct.label || providerName(sessionProvider(s)) : '';
    }
    function accountChanged(acct) {
      if (!acct || !acct.provider) return;
      accounts[acct.provider] = acct;
      if (active && sessions[active]) drawUsage();
    }
    on('agent:account', accountChanged);
    // Each chat window reads the accounts again after a resync, as it reads them at start.
    listeners.push(window.dinahHost.onResync(function () {
      if (deleted || !app().ProviderAccounts) return null;
      return Promise.resolve(app().ProviderAccounts()).then(function (list) { (list || []).forEach(function (acct) { if (window.dinahStateOrder.accept("agent:account", acct.provider, acct, true)) accountChanged(acct); }); });
    }));
    if (app() && app().ProviderAccounts) Promise.resolve(app().ProviderAccounts()).then(function (list) { (list || []).forEach(function (acct) { if (window.dinahStateOrder.accept("agent:account", acct.provider, acct, true)) accounts[acct.provider] = acct; }); }).catch(function () {});

    var usageRequest = 0;
    function usageLine(host, label, value) {
      var line = document.createElement('p'); line.className = 'usage-line';
      var name = document.createElement('span'); name.className = 'usage-label'; name.textContent = label;
      var data = document.createElement('span'); data.className = 'usage-value'; data.textContent = value;
      line.append(name, data); host.appendChild(line);
    }
    function finite(value) { return typeof value === 'number' && Number.isFinite(value); }
    function windowName(name, window) {
      var mins = window.windowDurationMins;
      return finite(mins) && mins > 0 ? (mins % 1440 === 0 ? num(mins / 1440) + ' Day' : mins % 60 === 0 ? num(mins / 60) + ' Hour' : num(mins) + ' Minute') + ' Window' : name + ' Window (Duration Not Reported)';
    }
    function quotaWindow(host, name, window) {
      if (!window) return;
      usageLine(host, windowName(name, window), finite(window.usedPercent) ? num(window.usedPercent) + '% used' : 'Usage not reported');
      var reset = finite(window.resetsAt) ? new Date(window.resetsAt * 1000) : null;
      usageLine(host, 'Resets', reset && !isNaN(reset.getTime()) ? dinahRegional.format(reset, 'dateTime') : 'Not reported');
    }
    function drawAccountUsage(info, provider) {
      var host = $('account-usage'); host.replaceChildren();
      $('account-usage-title').textContent = providerName(provider) + ' Account Usage';
      $('account-usage-link').hidden = !info.url;
      $('account-usage-link').querySelector('a').textContent = info.linkLabel || 'Open ' + providerName(provider) + ' Usage Page';
      var detail = document.createElement('p'); detail.className = 'form-help';
      detail.textContent = info.detail || (info.status === 'available' ? 'Reported by ' + providerName(provider) + '.' : 'Account usage is unavailable.');
      host.appendChild(detail);
      if (info.accountType) usageLine(host, 'Account', info.accountLabel || info.accountType);
      if (info.planType) usageLine(host, 'Plan', info.planType);
      (info.buckets || []).forEach(function (bucket) {
        var section = document.createElement('section'); section.className = 'usage-bucket';
        var title = document.createElement('h4'); title.textContent = bucket.name || bucket.id || 'Reported Limit'; section.appendChild(title);
        if (bucket.planType && bucket.planType !== info.planType) usageLine(section, 'Plan', bucket.planType);
        quotaWindow(section, 'Primary', bucket.primary); quotaWindow(section, 'Secondary', bucket.secondary);
        if (!bucket.primary && !bucket.secondary) usageLine(section, 'Windows', 'Not reported');
        host.appendChild(section);
      });
      if (info.status === 'available' && !(info.buckets || []).length) usageLine(host, 'Quota Windows', 'Not reported');
      if (finite(info.checkedAt)) {
        var checked = new Date(info.checkedAt * 1000);
        if (!isNaN(checked.getTime())) usageLine(host, 'Checked', dinahRegional.format(checked, 'time'));
      }
    }
    function loadAccountUsage() {
      var provider = chatProvider(), request = ++usageRequest;
      $('usage-menu').dataset.usageProvider = provider;
      $('account-usage-title').textContent = providerName(provider) + ' Account Usage';
      $('account-usage').textContent = 'Loading ' + providerName(provider) + ' account usage…';
      $('account-usage-link').hidden = true;
      Promise.resolve().then(function () {
        if (app().ProviderUsage) return app().ProviderUsage(provider);
        return { status: 'unavailable', detail: 'Account usage is unavailable.' };
      }).then(function (info) {
        if (deleted || request !== usageRequest || provider !== chatProvider() || !$('usage-menu').matches(':popover-open')) return;
        drawAccountUsage(info || { status: 'unavailable' }, provider);
      }).catch(function (error) {
        if (deleted || request !== usageRequest || provider !== chatProvider() || !$('usage-menu').matches(':popover-open')) return;
        drawAccountUsage({ status: 'unavailable', detail: 'Account usage could not be loaded. ' + String(error) }, provider);
      });
    }
    $('usage-menu').addEventListener('beforetoggle', function (event) {
      if (event.newState === 'open') {
        drawUsage();
        loadModelPrices(chatProvider());
        $('account-usage-title').textContent = agentName() + ' Account Usage';
        $('account-usage').textContent = 'Loading ' + agentName() + ' account usage…';
        $('account-usage-link').hidden = true;
      }
      else usageRequest++;
    });
    $('usage-menu').addEventListener('toggle', function (event) { if (event.newState === 'open') loadAccountUsage(); });
    $('account-usage-link').querySelector('a').addEventListener('click', function (event) {
      event.preventDefault(); event.stopPropagation();
      var provider = chatProvider();
      $('usage-menu').hidePopover();
      Promise.resolve().then(function () { return app().OpenProviderUsage ? app().OpenProviderUsage(provider) : Promise.reject('No usage page is available for this provider.'); }).catch(fail);
    });

    on('agent:file', function (f) {
      var s = S(f.sessionId);
      s.current = null;
      var r = row(s, 'file-row');
      r.insertAdjacentHTML('beforeend', '<span></span>');
      r.lastChild.textContent = (f.subagent ? 'The subagent "' + (f.subagent.name || 'Subagent') + '" wrote ' : 'Wrote ') + f.path + ' (' + f.bytes + ' characters)';
    });

    /* A turn that stopped at the round limit was told in the chat by the
       adapter ("The turn stopped at the limit of N rounds… that you set in
       Settings. Send a message to continue."), so its row says no more and
       offers only Continue, which sends a plain message through the chat's
       own send path: queueing, steering and the working lock all behave as
       they do for anything typed. */
    function roundLimitStopped(r, name) {
      var go = document.createElement('button');
      go.type = 'button'; go.className = 'btn btn-sm turn-continue'; go.textContent = 'Continue';
      go.title = 'Ask ' + name + ' to keep going from where it stopped';
      go.addEventListener('click', function () {
        if (teamPanel.isMember()) { teamPanel.openTeam().catch(fail); return; }
        var box = $('prompt');
        if (!box) return;
        box.value = 'Keep going from where you stopped.';
        box.focus();
        send('auto');
      });
      r.appendChild(go);
    }

    on('agent:turn', function (t) {
      var s = S(t.sessionId);
      s.current = null;
      s.stopping = false;
      var r = row(s, 'turn-row');
      r.insertAdjacentHTML('beforeend', '<span class="activity-muted"></span>');
      var name = providerName(sessionProvider(s));
      var words = { end_turn: name + ' finished its turn.', cancelled: 'You stopped the turn.', steered: 'You stopped the step to steer ' + name + '.', max_tokens: name + ' stopped at its output limit.', max_turn_requests: name + ' stopped at its limit of steps for one turn.', refusal: name + ' declined to go on.' };
      r.lastChild.textContent = t.error ? 'The turn ended with an error: ' + t.error : (words[t.stopReason] || 'The turn ended (' + t.stopReason + ').');
      // The turn's length, the context in use after it, and its cost when
      // Devin reports a cost.
      var meta = [];
      if (t.seconds != null) { meta.push(duration(t.seconds)); s.lastTurn = t.seconds; }
      if (s.usage) {
        meta.push(s.usage.used > s.usage.size ? 'context over its length' : 'context ' + contentPct(s.usage) + '% full');
        if (s.usage.cost) {
          var spent = s.usage.cost.amount - (s.costMark || 0);
          s.costMark = s.usage.cost.amount;
          // A subscription's turn shows no price; Usage has the API-price figure.
          if (spent > 0 && !subscriptionOf(s)) meta.push(money({ amount: spent, currency: s.usage.cost.currency }));
        }
      }
      if (meta.length) {
        r.insertAdjacentHTML('beforeend', '<span class="turn-meta"></span>');
        r.lastChild.textContent = meta.join(' · ');
      }
      // A turn that stopped at the round limit says whose limit, and offers to go on.
      if (!t.error && t.stopReason === 'max_turn_requests') roundLimitStopped(r, name);
      if (s.id === active) drawUsage();
      // A turn the person stopped or steered ends on purpose; its failures are
      // told all the same, its stopping is not.
      var failures = s.failures || [];
      s.failures = [];
      if (!s.loading) headsUp(s, failures, t.error || '');
    });

    /* ---- Signing in to Devin -------------------------------------------------
       When Devin refuses a request until the person signs in, a notice over
       the prompt offers Sign In to Devin. Devin then opens the browser on the
       organisation's sign-in page and the app waits for it to report back.
       The provider status menu also offers sign-in. */
    var signing = false;
    function signInNote(state, text) {
      var note = $('signin-note'), go = $('signin-go');
      note.hidden = state === 'hidden';
      if (note.hidden) return;
      $('signin-title').textContent = state === 'signing-in' ? 'Signing in to ' + agentName() + '…' : state === 'failed' ? 'The sign-in did not complete.' : agentName() + ' needs you to sign in.';
      $('signin-body').textContent = text;
      go.disabled = state === 'signing-in';
      go.textContent = state === 'signing-in' ? 'Waiting for the Browser…' : state === 'failed' ? 'Try Again' : 'Sign In to ' + agentName();
    }
    function signIn() {
      if ($('agent-menu').matches(':popover-open')) $('agent-menu').hidePopover();
      if (signing) return;
      signing = true;
      signInNote('signing-in', agentName() + ' is opening your browser on the sign-in page. Finish signing in there, then come back here.');
      (app().AgentSignIn ? app().AgentSignIn(chatProvider()) : app().SignIn()).then(function () {
        signing = false;
        signInNote('hidden');
        toast('Sign-in was requested. Test Connection in Settings checks whether it completed.', 'positive');
        refreshSessions();
      }).catch(function (err) {
        signing = false;
        signInNote('failed', String(err));
      });
    }
    $('signin-go').addEventListener('click', signIn);
    on('agent:auth', function (m) {
      if (m.provider && m.provider !== chatProvider()) return;
      if (m.state === 'signed-in') { signInNote('hidden'); scanProviders(); return; }
      if (m.state === 'failed') { signInNote('failed', m.detail || 'Sign-in did not complete.'); return; }
      if (m.state === 'required' && !signing) {
        signInNote('required', agentName() + ' requires sign-in before continuing. Sign in through the provider\'s browser page.' + (m.method ? ' Available method: ' + m.method + '.' : ''));
      }
    });

    on('agent:teamsent', function (m) { if (m.message) teamSentCard(S(m.sessionId), { delivered: m.message, text: m.text || '' }); });
    on('agent:sent', function (m) {
      var s = S(m.sessionId);
      s.current = null;
      if (m.kind === 'queued') message(s, 'you', m.text, 'You, sent when ' + agentName() + ' finished');
      if (m.kind === 'asked') message(s, 'you', m.text, m.label || 'Asked by another chat');
      if (m.kind === 'skill') message(s, 'you', m.text, m.label);
      // Messages the bus delivered are a card each, from their senders; an
      // event without them, from before they were kept, is drawn as before.
      if (m.kind === 'bus' && m.delivered && m.delivered.length) m.delivered.forEach(function (entry) { deliveredCard(s, entry); });
      else if (m.kind === 'bus') message(s, 'you', m.text, m.label || 'Messages from the bus');
    });
    /* A steer waiting for the agent's hooks shows as pending on its row, and
       as delivered once a hook hands it over, or as sent when the turn ended
       before any did and it went as the next message (steers.go). */
    var STEER_LABELS = { rejected: 'You, steering (not sent)', pending: 'You, steering (waiting for the agent\'s next step)', delivered: 'You, steering (delivered)', accepted: 'You, steering (accepted by the agent)', failed: 'You, steering (delivery unconfirmed)', queued: 'You, steering (queued for the next turn)', sent: 'You, steering (sent after the turn ended)' };
    on('agent:steer', function (e) {
      var s = S(e.sessionId);
      s.steerRows = s.steerRows || {};
      var m = s.steerRows[e.steer];
      if (!m && e.state === 'pending' && s.unreportedSteers && s.unreportedSteers.length) m = s.steerRows[e.steer] = s.unreportedSteers.shift();
      if (!m) return;
      var label = m.li.querySelector('.msg-label');
      label.textContent = STEER_LABELS[e.state] || label.textContent;
      m.deliveryUnconfirmed = e.state === "failed";
      if (m.deliveryUnconfirmed && m.restorePayload) recoveryButton(m, m.restorePayload);
      m.li.dataset.steer = e.state;
      if (e.state !== 'pending' && e.state !== 'queued') delete s.steerRows[e.steer];
    });
    function titlesChanged(t) { setTitles(t); drawSessions(); controller.refresh(); }
    on('agent:titles', titlesChanged);
    listeners.push(window.dinahHost.onResync(function () {
      if (deleted || !app().State) return null;
      return Promise.resolve(app().State()).then(function (st) { titlesChanged(st.titles); });
    }));

    on('agent:queue', function (q) {
      var s = S(q.sessionId);
      s.queued = q.queued || [];
      if (q.sessionId === active) drawQueue();
    });

    function drawQueue() {
      var s = active && sessions[active], n = s ? s.queued.length : 0;
      $('queue').hidden = n === 0;
      if (n) $('queue-words').textContent = n === 1 ? 'One message waits to be sent when ' + agentName() + ' finishes: "' + s.queued[0] + '"' : n + ' messages wait to be sent when ' + agentName() + ' finishes.';
      renderComposerOptions();
    }


    /* ---- The composer -----------------------------------------------------
       Idle: Send. During active work, keyboard sends steer when the provider
       and model support it, and queue otherwise. The visible Queue and Steer
       controls always choose their named action. Steer adds guidance without cancelling active work.
       Provider adapters or hooks deliver it at their next supported boundary.
       Stop and Steer remains the explicit interruption command. Stop sits
       at the end of the bar. The typing buttons stay off while the box is empty. */
    var working = false, primaryRequest = 0, lastQueueDecision = '';
    function indicatePrimaryAction() {
      var request = ++primaryRequest, id = active;
      function record(can, failure, superseded) {
        if (!working || active !== id || !$('send').classList.contains('btn-primary')) return;
        var signature = JSON.stringify([id, can, failure, superseded]);
        if (signature === lastQueueDecision) return;
        lastQueueDecision = signature;
        if (!app().LogQueueDefault) return;
        try { Promise.resolve(app().LogQueueDefault(id, can, failure, superseded)).catch(function () {}); } catch (error) {}
      }
      function paint(steer, failure) {
        var superseded = request !== primaryRequest || active !== id;
        if (superseded) { record(steer, failure || '', true); return; }
        $('send').classList.toggle('btn-primary', !steer);
        $('steer-send').classList.toggle('btn-primary', steer);
        if (steer || !working) lastQueueDecision = '';
        else record(steer, failure || '', false);
      }
      if (!working || !id || !app().CanSteer) { paint(false); return; }
      Promise.resolve().then(function () { return app().CanSteer(id, collectAttachments($('prompt').value)); }).then(function (can) { paint(!!can); }, function (error) { paint(false, String(error)); });
    }
    function setWorking(w) {
      if (!w && root.contains(document.activeElement) && document.activeElement.closest('[data-command="stop-turn"], [data-view-id="steer-send"]')) $('prompt').focus({ preventScroll: true });
      working = w;
      $('steer-send').style.visibility = w ? '' : 'hidden';
      $('steer-send').setAttribute('aria-hidden', String(!w));
      $('steer-send').toggleAttribute('inert', !w);
      var stopControl = root.querySelector('.composer-stop');
      stopControl.setAttribute('aria-hidden', String(!w));
      stopControl.toggleAttribute('inert', !w);
      $('send').querySelector('.composer-send-label').textContent = w ? 'Queue' : 'Send';
      $('send').title = w ? 'Queue explicitly. ' + sendKeyName() + ' steers when supported, otherwise queues.' : 'Send (' + sendKeyName() + ')';
      $('prompt').placeholder = 'Say what you want the agent to do, or type / for a skill.';
      typed();
      renderComposerOptions();
    }
    // The box grows with its text as every window's box does (composer.js).
    function typed() {
      window.dinahComposer.fit($('prompt'));
      var empty = !$('prompt').value.trim() && !skillForm.isOpen() && !(typeof attachments !== 'undefined' && attachments.length);
      $('send').disabled = empty || switchingProvider;
      $('steer-send').disabled = !working || empty || switchingProvider;
      indicatePrimaryAction();
      saveDraft();
    }
    $('prompt').addEventListener('input', typed);

    var context = null;     // the document picked in the sidebar
    function ensureSession() {
      var s = active && sessions[active];
      if (s && s.info.open) return Promise.resolve(s);
      // A chat that is opening is in the engine as soon as it is announced, and the engine holds a message sent
      // to it while it loads; the page waits only for that announcement, not for the history.
      if (s && s.holding) return new Promise(function (resolve) {
        (function wait() { if (s.info.open || !s.holding) resolve(s); else setTimeout(wait, 50); })();
      });
      if (s && !s.info.open) return openSession(active);
      return newSession();
    }

    function restoreSubmission(keep) {
      if (keep.document && context && context !== keep.document) {
        toast('Clear the current document before restoring this submission.', 'warn'); return false;
      }
      var box = $('prompt');
      box.value += (box.value.trim() ? '\n\n' : '') + keep.text;
      attachments = attachments.concat(keep.files || []);
      mentionPicker.setSelected(mentionPicker.selected().concat(keep.mentions || []));
      if (keep.document) setContext(keep.document);
      drawAttachments(); typed(); box.focus(); return true;
    }
    function recoveryButton(m, keep) {
      if (m.recoveryAttached) return;
      m.recoveryAttached = true;
      var button = document.createElement('button');
      button.type = 'button'; button.className = 'btn btn-sm';
      button.textContent = 'Copy to Draft';
      button.addEventListener('click', function () { restoreSubmission(keep); });
      m.li.appendChild(button);
    }
    async function recoverSteering() {
      if (!active || !app().RecoverSteering) return;
      var s = S(active);
      if (s.working || s.loading) return;
      try {
        var records = await app().RecoverSteering(s.id);
        for (var record of records || []) {
          if (s.log.querySelector('[data-steering-recovery="' + record.id + '"]')) continue;
          var payload = JSON.parse(record.payload);
          var m = message(s, 'you', payload.text, record.state === 'queued' ? 'You, recovered follow-up (not resent)' : 'You, recovered steering (delivery unconfirmed; not resent)');
          m.li.dataset.steeringRecovery = record.id;
          recoveryButton(m, { text: payload.text, files: payload.attachments || [], document: payload.document });
          var dismiss = document.createElement('button');
          dismiss.type = 'button'; dismiss.className = 'btn btn-sm'; dismiss.textContent = 'Dismiss';
          (function (row, id, button) {
            button.addEventListener('click', function () {
              app().DismissSteering(s.id, id).then(function () { row.remove(); }).catch(fail);
            });
          })(m.li, record.id, dismiss);
          m.li.appendChild(dismiss);
        }
      } catch (err) { fail(err); }
    }
    /* While the chat's history loads behind the saved copy, the window says so in its title bar, since its
       controls are those of the chat as it was and change when the load ends. It is drawn on its own, since
       drawing the whole window again also moves the logs. */
    function drawLoadingChip() {
      var win = root.closest('[data-win]'), group = win && win.querySelector('.win-head > .win-statuses');
      if (!group) return;
      var s = active && sessions[active], el = group.querySelector('.chat-loading-chip');
      if (!el) { el = document.createElement('span'); el.className = 'win-status chat-loading-chip'; group.appendChild(el); }
      if (!el.firstChild) { el.innerHTML = '<span class="spinner" aria-hidden="true"></span><span>Loading</span>'; el.setAttribute('role', 'status'); }
      el.title = 'The rest of this chat is still loading in the background. Its controls may change when it finishes.';
      el.hidden = !(s && s.loading);
    }
    function send(how) {
      if (teamPanel.isMember()) return teamPanel.openTeam().catch(fail);
      if (switchingProvider) return;
      var pickedSkill = skillForm.skill() || selectedSkill;
      var text = $('prompt').value.trim();
      if (skillForm.isOpen()) {
        text = assemble(true);
        if (!text) { toast('Fill in the fields marked in red, or press Esc to type the command by hand.', 'warn'); return; }
        skillForm.clear();
        setWorking(working);
      }
      var atts = collectAttachments(text);
      if (!text && !atts.length) return;
      if (window.dinahUIState) window.dinahUIState.releaseScroll($('activity'));
      var doc = context;
      var names = atts.map(function (a) { return a.name; });
      var shown = text + (doc ? '\n\n*Document: ' + doc.title + (doc.selection ? ', the selected passage' : '') + '*' : '') + (names.length ? '\n\n*Attached: ' + names.join(', ') + '*' : '');
      var keep = { text: text, files: attachments.slice(), mentions: mentionPicker.selected().slice(), document: doc };
      var submittedRow = null, submittedSession = null;
      $('prompt').value = '';
      attachments = [];
      mentionPicker.setSelected([]);
      drawAttachments();
      typed();
      recall.push(text);
      ensureSession().then(async function (s) {
        await drainAgentEvents();
        submittedSession = s;
        if (!s) { $('prompt').value = keep.text; attachments = keep.files; mentionPicker.setSelected(keep.mentions); drawAttachments(); typed(); return; }
        if (pickedSkill && pickedSkill.id && text.split(/\s/,1)[0] === '/' + pickedSkill.name && app().PrepareSkill) {
          try {
            text = await app().PrepareSkill(pickedSkill.id, pickedSkill.revision || '', s.info.cwd || workspace, text);
          } catch (err) {
            $('prompt').value = keep.text; attachments = keep.files; mentionPicker.setSelected(keep.mentions);
            drawAttachments(); typed(); throw err;
          }
        }
        selectedSkill = null;
        /* A chat mentioned for asking whose window was restored with the
           layout is not connected to its agent yet; it is connected now, as
           Reconnect Now would, so that the agent can ask it. */
        for (const at of atts) {
          var target = at.kind === 'chat' && sessions[at.sessionId];
          if (!target || target.info.open) continue;
          if (!(await openResolvedSession(at.sessionId, false, true))) {
            $('prompt').value = keep.text; attachments = keep.files; mentionPicker.setSelected(keep.mentions);
            drawAttachments(); typed();
            throw new Error('"' + at.name + '" could not be reconnected to its agent, so it cannot be asked: ' + (target.failed || 'the agent did not open it'));
          }
        }
        if (how === 'auto' && s.working) {
          var canSteer = false;
          try { canSteer = !!(app().CanSteer && await app().CanSteer(s.id, atts)); } catch (_) {}
          how = canSteer ? 'steer' : 'queue';
        }
        s.current = null;
        var call, m;
        // A chat still loading its history holds the message in the engine's queue, which shows it as waiting
        // and sends it when the load ends; the page draws it then, where it belongs after the history.
        if (s.holding) call = app().Send(s.id, text, doc, atts);
        else if (!s.working) { m = message(s, 'you', shown); call = app().Send(s.id, text, doc, atts); }
        else if (how === 'queue') call = app().Queue(s.id, text, doc, atts);
        else if (how === 'stop-steer') { m = message(s, 'you', shown, 'You, stopping the step to steer'); call = app().StopAndSteer(s.id, text, doc, atts); }
        else {
          m = message(s, 'you', shown, 'You, steering');
          m.restorePayload = keep;
          // A steer the agent's hooks will hand over is reported as pending,
          // then delivered (agent:steer); the oldest unreported one is its row.
          (s.unreportedSteers = s.unreportedSteers || []).push(m);
          call = app().Steer(s.id, text, doc, atts);
        }
        if (m) { m.raw = text; m.doc = doc; m.images = atts.filter(function (at) { return /^image\/(png|jpeg|gif|webp|bmp)$/i.test(at.mimeType || '') && at.data; }); dirty.add(m); scheduleDraw(); scroll(true); }
        submittedRow = m;
        return call;
      }).catch(async function (err) {
        await drainAgentEvents();
        if (submittedSession && submittedRow && !submittedRow.deliveryUnconfirmed) {
          submittedSession.unreportedSteers = (submittedSession.unreportedSteers || []).filter(function (row) { return row !== submittedRow; });
        }
        if (submittedRow && submittedRow.deliveryUnconfirmed) {
          recoveryButton(submittedRow, keep);
        } else {
          var restored = !$('prompt').value.trim() && !attachments.length && !mentionPicker.selected().length && restoreSubmission(keep);
          if (restored) {
            if (submittedRow) submittedRow.li.querySelector('.msg-label').textContent = 'You (not sent; restored to draft)';
          } else {
            var retained = submittedRow;
            if (!retained && submittedSession) retained = message(submittedSession, 'you', keep.text, 'You (not sent)');
            if (!retained) {
              var note = document.createElement('div'); note.textContent = 'Unsent message: ' + keep.text;
              $('activity').appendChild(note); retained = { li: note };
            }
            recoveryButton(retained, keep);
          }
        }
        fail(err);
      });
    }
    /* ---- Skills -----------------------------------------------------------
       The skills menu lists the workspace's skills first (read from its
       .devin/skills folder, so it works before Devin starts), then Devin's own
       commands and the skills Devin imports from other tools, as Devin last
       offered them. The same list pops up over the prompt when a "/" starts a
       word there; "\/" or a slash inside a word stays plain text. */
    var skills = [], selectedSkill = null;
    var GROUPS = [['mine', 'My Skills'], ['workspace', 'Workspace Skills'], ['personal', 'My Agent Skills'], ['', 'Agent Commands'], ['imported', 'From Other Tools']];
    /* The group of the commands an agent offers is the one of the provider whose facts say it has commands of its
       own, under that provider's name; the skills it lists have that provider's id as their source. */
    function updateSkillGroups() {
      var own = providers.filter(function (p) { return p.facts && p.facts.ownCommands; })[0];
      GROUPS[3][0] = own ? own.id : ''; GROUPS[3][1] = own && own.name ? own.name : 'Agent Commands';
    }
    skillGroupUpdates.add(updateSkillGroups);
    listeners.push(function () { skillGroupUpdates.delete(updateSkillGroups); });
    updateSkillGroups();
    function loadSkills() {
      var requestedProvider = chatProvider(), requestedWorkspace = workspace;
      return (app().SkillCatalog ? dinahSkills.forChat(workspace, requestedProvider) : app().SkillsForProviderAt ? app().SkillsForProviderAt(workspace, requestedProvider) : app().SkillsAt(workspace)).then(function (l) {
        if (chatProvider() !== requestedProvider || workspace !== requestedWorkspace) return;
        skills = l || []; skillsChanged();
      }).catch(function () {});
    }
    on('agent:commands', function () { loadSkills(); });
    listeners.push(window.dinahHost.onResync(function () { return deleted ? null : loadSkills(); }));

    function skillRows(host, list, onPick, selected) {
      GROUPS.forEach(function (g) {
        var mine = list.filter(function (s) { return s.source === g[0]; });
        if (!mine.length) return;
        var h = document.createElement('div');
        h.className = 'md-section-label';
        h.textContent = g[1];
        host.appendChild(h);
        mine.forEach(function (s) {
          var r = document.createElement('div');
          r.className = 'md-row skill-row';
          r.setAttribute('role', 'option');
          r.innerHTML = '<a class="md-item" href="#"><span class="skill-name"></span><span class="md-meta"></span></a>';
          r.querySelector('.skill-name').textContent = '/' + s.name + (s.hint ? ' ' + s.hint : '');
          r.querySelector('.md-meta').textContent = (s.description || '').trim() + (s.origin ? ' · ' + s.origin : '');
          r.dataset.skill = s.name;
          if (selected === s.name) r.setAttribute('aria-selected', 'true');
          r.firstChild.addEventListener('mousedown', function (e) { e.preventDefault(); });
          r.firstChild.addEventListener('click', function (e) { e.preventDefault(); onPick(s.id || s.name); });
          host.appendChild(r);
        });
      });
    }

    /* The Skills menu's rows, as the toolbar's Skills command gives them: the
       templates and the skills under their groups' headings, each with its
       description as the row's second line, then the templates'
       own commands. The "/" pop-up over the box offers the same list,
       narrowed as the person types. */
    function skillItems() {
      var out = [];
      if (templates.length) out.push({ heading: 'My Templates' });
      templates.forEach(function (t) {
        out.push({ id: 'template-' + t.name, label: t.name, detail: t.text.split('\n')[0], run: function () { insertTemplate(t.name, null); } });
      });
      if (!skills.length) out.push({ id: 'skills-none', label: 'No Skills Yet', disabled: true, reason: 'No skills were found in this workspace.' });
      GROUPS.forEach(function (g) {
        var mine = skills.filter(function (s) { return s.source === g[0]; });
        if (!mine.length) return;
        out.push({ heading: g[1] });
        mine.forEach(function (s) {
          var about = (s.description || '').trim() + (s.origin ? ' · ' + s.origin : '');
          out.push({ id: 'skill-' + (s.id || s.name), label: '/' + s.name + (s.hint ? ' ' + s.hint : ''), detail: about || undefined, run: function () { insertSkill(s.id || s.name, null); } });
        });
      });
      out.push('-',
        { id: 'save-template', label: 'Save the Prompt as a Template…', run: function () { openTemplates($('prompt').value); } },
        'my-templates');
      return out;
    }
    function skillsChanged() { if (window.pudlCommands) window.pudlCommands.changed(); }

    const templateEditor = window.dinahChatTemplates.create({
      app, element: $, hidePop: () => hidePop(), typed, skillsChanged, toast, fail,
      templates: () => templates, setTemplates: value => { templates = value; }
    });
    const { templateRows, insertTemplate, openTemplates } = templateEditor;

    const skillForm = window.dinahChatSkillForm.create({
      element: $, viewPrefix, typed, send,
      refreshWorking: () => setWorking(working)
    });
    const { openForm, closeForm, assemble } = skillForm;

    // Puts "/name " in the box: in place of the "/word" being typed, or at
    // the start of what is there. A skill with an argument-hint opens its form
    // instead, when the box has nothing else in it.
    function insertSkill(name, tok) {
      var sk = skills.find(function (s) { return s.id === name; }) || skills.find(function (s) { return s.name === name; });
      selectedSkill = sk || null;
      name = sk ? sk.name : name;
      var box = $('prompt'), v = box.value, ins = '/' + name + ' ';
      var rest = tok ? (v.slice(0, tok.start) + v.slice(tok.end)).trim() : v.replace(/^\/\S*\s*/, '').trim();
      if (sk && sk.hint && !rest) {
        hidePop();
        box.value = '';
        typed();
        if (openForm(sk)) return;
      }
      if (tok) {
        box.value = v.slice(0, tok.start) + ins + v.slice(tok.end);
        box.setSelectionRange(tok.start + ins.length, tok.start + ins.length);
      } else {
        var rest = v.replace(/^\/\S*\s*/, '');
        box.value = ins + rest;
        box.setSelectionRange(box.value.length, box.value.length);
      }
      hidePop();
      typed();
      box.focus();
    }

    const mentionPicker = window.dinahChatMentions.create({
      app, element: $, productName, sessions, normalizedWorkspace, title, workspaceLabel,
      typed, GROUPS, templateRows, insertTemplate, skillRows, insertSkill, skillForm, closeForm,
      active: () => active, workspace: () => workspace, deleted: () => deleted,
      skills: () => skills, templates: () => templates
    });
    const { refreshMentions, loadAppletMentions, showMentions, wordToken, hidePop } = mentionPicker;

    /* ---- Paste and drop ---------------------------------------------------
       A screenshot pasted into the box, or files dropped on the chat, go with
       the next message. The Go side sends an image as an image, a text file
       embedded, and anything else saved into the workspace and linked, by
       what Devin says it takes. */
    var attachments = [];   // { kind: 'file', name, mimeType, data, size, thumb } or { kind: 'resource', name, ref }
    /* The chips are the shared box's (composer.js, attachments), which a
       group chat's box uses too. Drawing them saves the draft. */
    var chips = window.dinahComposer.attachments({
      host: $('attachments'),
      get: function () { return attachments; },
      set: function (list) { attachments = list; },
      changed: function () { typed(); },
      drawn: function () { saveDraft(); },
      alive: function () { return !deleted; },
      toast: function (text) { toast(text, 'warn'); },
      // The chat's provider's limit on the files in one message.
      limit: function () { var l = fileLimits[chatProvider()]; return l && l.limit > 0 ? { max: l.limit, reason: l.reason } : null; },
      // The engine says whether the chat's model can read pictures, from what the provider documents (ChatImageInput:
      // yes, no or unknown). Only a model that cannot is warned about, and no provider or model is named.
      note: function (list) {
        var s = active && sessions[active];
        if (!s || s.imageInput !== 'no' || !list.some(function (a) { return a.thumb; })) return '';
        return { text: 'This chat\'s model can\'t read pictures. Choose one that can, or remove the picture.', warn: true };
      }
    });
    // Asked when a picture is attached and when the chat's information changes, since the model can change.
    function askImageInput() {
      var s = active && sessions[active];
      if (!s || !app().ChatImageInput || !attachments.some(function (a) { return a.thumb; })) return;
      var id = s.id;
      Promise.resolve(app().ChatImageInput(id)).then(function (answer) {
        if (sessions[id] !== s || s.imageInput === answer) return;
        s.imageInput = answer; chips.draw();
      }).catch(function () {});
    }
    function addFiles(list) { chips.add(list); askImageInput(); }
    function drawAttachments() { chips.draw(); }
    function paste(e) {
      var target = e.target && e.target.closest ? e.target : null;
      var win = root.closest('[data-win]'), owner = target && target.closest('[data-win]');
      var dialog = target && target.closest('dialog, [role="dialog"]');
      if (!win || (owner && owner !== win) || (dialog && dialog !== win)) return;
      var field = target && target.closest('input, textarea, [contenteditable="true"]');
      if (field && field !== $('prompt')) return;
      var cd = e.clipboardData;
      if (!cd) return;
      var files = chips.clipboardFiles(cd);
      if (!files.length) return;
      // Keep text's native insertion in the composer when the clipboard has
      // both text and a picture. The picture must not be silently discarded.
      if (field !== $('prompt') || !cd.getData('text/plain')) e.preventDefault();
      addFiles(files);
    }
    chips.dropOn(root.querySelector('.chat-pane'), function () { $('prompt').focus(); });
    // The Add button: files from the computer through the system's file
    // dialog, or anything the @ pop-up offers, which it opens in the box.
    $('file-pick').addEventListener('change', function () {
      addFiles($('file-pick').files);
      $('file-pick').value = '';
      $('prompt').focus();
    });
    function pickComputerFiles() {
      return chips.pick($('file-pick')).then(function () { if (!deleted) $('prompt').focus(); });
    }
    /* Add › Browse the Workspace…: the Open dialog, from which anything in
       Dinah's workspaces goes with the message by its place there,
       and From Your Computer… adds files as Files From This Computer… does. */
    function browseToAttach() {
      dinahFilePicker.pick({ mode: 'open', any: true, title: 'Attach', okLabel: 'Attach',
        commit: function (choice) {
          if (choice.computer) return pickComputerFiles().then(function () { return true; });
          if (choice.node && choice.node.ref) {
            var ref = choice.node.ref;
            if (!attachments.some(function (a) { return a.kind === 'resource' && a.ref.sourceId === ref.sourceId && a.ref.path === ref.path && a.ref.workspaceId === ref.workspaceId; })) {
              attachments.push({ kind: 'resource', name: choice.node.title || choice.node.name || ref.path, ref: ref });
            }
            drawAttachments();
            typed();
            $('prompt').focus();
            return true;
          }
          throw new Error('Choose a document or a file in the list, or From Your Computer… for a file on this computer.');
        }
      }).catch(fail);
    }
    /* Add › Search… and Add › Applets…: an "@" typed at the caret, with the
       list it offers. */
    function addMention(what) {
      var box = $('prompt'), v = box.value, at = box.selectionStart;
      var gap = at > 0 && !/\s/.test(v[at - 1]) ? ' ' : '';
      box.value = v.slice(0, at) + gap + '@' + v.slice(box.selectionEnd);
      var c = at + gap.length + 1;
      box.focus();
      box.setSelectionRange(c, c);
      typed();
      if (what === 'applets') {
        if (mentionPicker.catalogFailed()) loadAppletMentions();
        showMentions(wordToken('@'), 'applets');
      } else showMentions(wordToken('@'), 'all');
    }
    // The Add menu's rows, with what each adds as their second line.
    function addItems() {
      return [
        // At the limit on the files in one message the command is disabled,
        // with the reason, rather than refusing the files afterwards.
        { id: 'add-files', label: 'Files From This Computer…', detail: 'Pictures, emails, text and any other file', disabled: chips.full(), reason: chips.fullReason(), run: function () { return pickComputerFiles().catch(fail); } },
        { id: 'add-browse', label: 'Browse the Workspace…', detail: 'Choose a document from the workspace\'s folders', run: browseToAttach },
        { id: 'add-mention', label: 'Mention an Open Document or Chat…', detail: 'An open document or another chat by name (type @ in the box)', run: function () { addMention('all'); } },
        { id: 'add-applets', label: 'Windows and Applets…', detail: 'Choose a window for the agent to drive, or a bundled applet, for this message (type @ in the box)', run: function () { addMention('applets'); } }
      ];
    }

    // A file dropped anywhere else in the window is not opened as a page.
    ['dragover', 'drop'].forEach(function (t) { window.addEventListener(t, function (e) { if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length) e.preventDefault(); }); });

    // What goes with the message: the files, and the mentions still named in
    // its text.
    function collectAttachments(text) {
      return attachments.map(function (a) {
        return a.kind === 'resource' ? { kind: 'resource', name: a.name, ref: a.ref } : { kind: 'file', name: a.name, mimeType: a.mimeType, data: a.data };
      })
        .concat(mentionPicker.selected().filter(function (m) {
          if (m.att.kind !== 'applet' && m.att.kind !== 'window' && m.att.kind !== 'chat') return text.indexOf(m.ins) >= 0;
          var token = m.ins.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
          return new RegExp('(^|\\s)' + token + '(?=$|[\\s.,!?;:)\\]}])').test(text);
        }).map(function (m) { return m.att; }));
    }

    $('composer').addEventListener('submit', function (e) { e.preventDefault(); send(e.submitter && e.submitter === $('send') ? 'queue' : 'auto'); });
    $('steer-send').addEventListener('click', function () { send('steer'); });

    /* Prompt history. Up in the box, with the caret on its first line, brings
       back this chat's previous prompt; Down, from the last line, goes forward
       again, back to what was being typed. A line that wraps counts as the
       lines it shows. The history outlasts a restart, and a chat continued
       from another keeps the earlier chat's prompts. A new chat starts with
       none. The keys are the shared box's (composer.js); a chat's entries
       are the texts of its prompts. */
    var recall = window.dinahComposer.history(function () { return $('prompt').value; }, function (text) { $('prompt').value = text; typed(); });

    /* The chat's unsent draft is kept across a restart (composer.js,
       keeper), by the chat's session, or as "new" for a chat that has none
       yet. The draft is the box's text, or the text Up set aside while an
       earlier prompt is in the box, with the mentions chosen for it and the
       workspace documents attached to it. Files attached from the computer
       or pasted are not kept: their contents, up to 25 MB each, are held only
       in memory, and the saved UI state is bounded at 32 MiB in all. Sending
       empties the box, which drops the draft. */
    var draftKeeper = null;
    function draftKey() { return 'chat:' + (active || 'new'); }
    function saveDraft() {
      // A chat closed for good, such as an early view of a session that
      // another chat took, saves nothing over the chat that has the session.
      if (!draftKeeper || deleted) return;
      draftKeeper.rekey(draftKey());
      var text = recall.draft($('prompt').value) || '';
      var resources = attachments.filter(function (a) { return a.kind === 'resource'; }).map(function (a) { return { kind: 'resource', name: a.name, ref: a.ref }; });
      var used = mentionPicker.selected().filter(function (m) { return text.indexOf(m.ins) >= 0; });
      draftKeeper.save(text.trim() || resources.length ? { text: text, mentions: used, resources: resources } : null);
    }
    // A draft saved before a restart comes back into an empty box.
    function restoreDraft(value) {
      if ($('prompt').value || attachments.length) { saveDraft(); return; }
      $('prompt').value = String(value.text || '');
      mentionPicker.setSelected(Array.isArray(value.mentions) ? value.mentions : []);
      attachments = Array.isArray(value.resources) ? value.resources : [];
      drawAttachments(); typed();
    }
    function loadRecall() {
      var id = active;
      if (!id || !app().ChatHistory) return Promise.resolve(recall.entries);
      return app().ChatHistory(id).then(function (texts) {
        if (active === id) recall.set(texts);
        return recall.entries;
      });
    }
    // The prompt's keys, for Help › Keyboard Shortcuts.
    if (window.dinahCommands && window.dinahCommands.addHelp) {
      window.dinahCommands.addHelp('Chat', [
        { label: 'Send', key: 'Ctrl+Enter' },
        { label: 'Send, When Settings › Chat Says Enter Sends', key: 'Enter' },
        { label: 'A New Line, When Enter Sends', key: 'Shift+Enter' },
        { label: 'Steer the Turn Under Way', key: 'Ctrl+Shift+Enter' },
        { label: 'Stop the Turn Under Way', key: 'Esc' },
        { label: 'An Earlier or Later Prompt, From the First or Last Line', key: 'Up / Down' },
        { label: 'Rename Chat…', key: 'F2' },
        { where: 'Rename Box', label: 'Keep or Drop the New Name', key: 'Enter / Esc' },
        { where: 'Skills and @ List', label: 'Choose', key: 'Up / Down, then Enter or Tab' },
        { where: 'Skills and @ List', label: 'Close the List', key: 'Esc' },
        { where: 'Skill Form', label: 'Close the Form', key: 'Esc' }]);
    }
    // Ctrl+Shift+Enter steers while a turn runs.
    window.dinahComposer.bind($('prompt'), { history: recall, send: function (e) { send(e.shiftKey && working ? 'steer' : 'auto'); } });
    $('prompt').addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && working) { e.preventDefault(); stopTurn(); }
    });

    /* ---- Agent options: mode and model ------------------------------------
       Devin offers them per session. The mode is named for people who are not
       writing code; any mode Devin adds keeps Devin's own name. */
    var MODES = {
      'accept-edits': ['Work', 'Reads, writes files and runs tools'],
      'ask': ['Ask', 'Answers questions and changes nothing'],
      'plan': ['Plan', 'Proposes a plan and waits for your go-ahead']
    };

    function optValues(o) {
      var out = [];
      (o.options || []).forEach(function (v) {
        if (v.options) v.options.forEach(function (g) { out.push({ value: g.value, name: g.name, description: g.description }); });
        else out.push({ value: v.value, name: v.name, description: v.description });
      });
      if (o.category === 'mode') out = out.map(function (v) { var m = MODES[v.value]; return m ? { value: v.value, name: m[0], description: m[1] } : v; });
      return out;
    }
    function byCat(opts, cat) { return (opts || []).filter(function (o) { return o.category === cat; })[0]; }
    function valueName(o, value) {
      var v = optValues(o).filter(function (x) { return x.value === value; })[0];
      return v ? v.name : String(value);
    }

    /* The options the bar over the prompt shows. For an open session, its
       own. With no session in front, what the next session will start with:
       Devin's list from the last session seen, with the defaults and any
       choice made here since; such a choice is applied when the session
       starts. For a past session that is not open, none: nothing can be
       changed until it is. */
    var pending = {};
    function barOptions() {
      var s = active && sessions[active];
      if (s) return s.info.open ? s.options : [];
      return (providerCatalogs[chosenProvider] || (chosenProvider === defaultProvider ? catalog : [])).map(function (o) {
        var v = pending[o.id] !== undefined ? pending[o.id] : chosenProvider === defaultProvider ? defaults[o.id] : (providerDefaults[chosenProvider] || {})[o.id];
        return v !== undefined ? Object.assign({}, o, { currentValue: v }) : o;
      });
    }

    /* The provider, the mode and the model are drawn by the toolbar from the
       chat's commands (providerCommand, modeCommand, modelCommand); a change
       to what they read is drawn again here. */
    function renderComposerOptions() {
      if (window.pudlCommands) window.pudlCommands.changed();
      if ($('usage-menu').matches(':popover-open') && $('usage-menu').dataset.usageProvider !== chatProvider()) loadAccountUsage();
      menuStates();
    }

    // Why the provider cannot be changed now, or null when it can.
    function providerLocked() {
      var session = active && sessions[active];
      if (creating || switchingProvider) return 'The chat is being set up.';
      if (session && (session.working || session.loading || (session.queued || []).length)) return 'Not while the agent is working or messages wait.';
      return null;
    }
    function chooseProviderId(id) {
      if (providerLocked() || id === chatProvider()) return;
      if (active) { switchProvider(id); return; }
      chosenProvider = id;
      pending = {};
      renderComposerOptions();
      loadSkills();
      drawAgentState();
      // A provider whose catalog the page has not been given (only the default's is) is read
      // from what the engine has cached, so its Model control shows before the chat starts.
      if (!providerCatalogs[id]) {
        Promise.resolve(adapter.readProviderPreferences(id)).then(function () {
          if (!active && chosenProvider === id) renderComposerOptions();
        }).catch(function () {});
      }
    }
    /* A saved chat not yet reopened is reopened as one of the toolbar's
       menus opens, so the model and the skills it lists are its own.
       Toggle events do not bubble, so the toolbar listens as they pass. */
    $('composer').querySelector('.composer-toolbar').addEventListener('beforetoggle', function (e) {
      if (e.newState === 'open' && e.target.classList.contains('menu-panel')) reconnectSaved();
    }, true);
    /* The provider, the mode and the model each have a glyph, so that a
       narrowing toolbar first shows them as glyphs alone, as it does the
       other controls, before it folds the Agent group into a menu. The one in
       effect is the control's detail as well as its words, so the tooltip and
       the accessible name still say it when only the glyph shows. PUDL has no
       glyph for a provider, a mode or a model as such: app is the program
       that answers the chat, shield is what it may do without asking, and
       tag is the name of the model in use. */
    function providerCommand() {
      var current = providerName(chatProvider());
      return { id: 'provider', label: 'Provider', glyph: 'app', current: current, detail: current,
        disabled: !!providerLocked(), reason: providerLocked(),
        items: providerChoices(chatProvider()).map(function (p) {
          return { id: 'provider-' + p.id, label: p.name, choice: 'chat-provider', value: p.id, checked: p.id === chatProvider(),
            disabled: !!p.problem, reason: p.problem, run: function () { chooseProviderId(p.id); } };
        }) };
    }
    function modeCommand() {
      var mode = byCat(barOptions(), 'mode'), current = mode ? valueName(mode, mode.currentValue) : '';
      return { id: 'mode', label: 'Mode', glyph: 'shield', hidden: !mode, current: current, detail: current || undefined,
        items: mode ? optValues(mode).map(function (v) {
          return { id: 'mode-' + v.value, label: v.name, detail: v.description || undefined, choice: 'chat-mode', value: v.value, checked: v.value === mode.currentValue,
            run: function () { return setSessionOption(mode.id, v.value); } };
        }) : [] };
    }
    /* The model: the favorites, and the one in effect when it is not among
       them, then All Models…. */
    function modelCommand() {
      var model = byCat(barOptions(), 'model');
      function row(v, words) {
        var price = modelPrice(chatProvider(), v.value);
        return { id: 'model-' + v.value, label: words, choice: 'chat-model', value: v.value, checked: v.value === model.currentValue,
          detail: price ? price.costSummary : undefined,
          run: function () { return setSessionOption(model.id, v.value); } };
      }
      var items = [];
      if (model) {
        var favs = optValues(model).filter(function (v) { return chatPins().indexOf(v.value) >= 0; });
        if (model.currentValue && chatPins().indexOf(model.currentValue) < 0) {
          items.push(row({ value: model.currentValue }, valueName(model, model.currentValue) + (active ? ' (This Session)' : ' (the Default)')));
        }
        items.push({ heading: 'Favorite Models' });
        if (!favs.length) items.push({ id: 'model-no-favorites', label: 'No Favorites Yet', disabled: true, reason: 'Pick them in Settings, or pin them under All Models.' });
        favs.forEach(function (v) { items.push(row(v, v.name)); });
        items.push('-', { id: 'all-models', label: 'All Models…', run: openModels });
      }
      var current = model ? valueName(model, model.currentValue) : '';
      return { id: 'model', label: 'Model', glyph: 'tag', hidden: !model,
        current: current, detail: current || undefined, items: items };
    }

    /* Menu entries that can do nothing at the moment are greyed out. */
    function commandEnabled(id) {
      const session = active && sessions[active];
      if (teamPanel.isMember() && ['send', 'steer', 'stop-and-steer', 'add', 'skills', 'prompt-history', 'my-templates'].includes(id)) return false;
      if (id === 'all-models') return !!byCat(barOptions(), 'model');
      if (id === 'cancel') return !!session?.working;
      if (id === 'save-run') return !!session?.log.children.length;
      if (id === 'rename-session' || id === 'remove-session') return !!session;
      if (id === 'unhide-sessions') return hiddenCount() > 0;
      return true;
    }
    function menuStates() {
      if (currentChat() === controller) window.dinahCommands.changed();
    }

    on('agent:options', function (m) {
      var s = S(m.sessionId);
      s.options = m.options || [];
      rememberCatalog(s);
      if (m.sessionId === active) renderComposerOptions();
      if ($('models').open) drawModelsDialog();
    });

    function setSessionOption(id, value) {
      if (!active) {
        pending[id] = value;
        renderComposerOptions();
        if ($('models').open) drawModelsDialog();
        return Promise.resolve();
      }
      return app().SetSessionOption(active, id, value).then(function (opts) {
        sessions[active].options = opts || sessions[active].options;
        renderComposerOptions();
        if ($('models').open) drawModelsDialog();
      }).catch(fail);
    }

    /* The full list of models, for this session. */
    function modelRows(host, o, opts) {
      var pinned = chatPins();
      host.innerHTML = '';
      var f = (opts.filter || '').trim().toLowerCase();
      var all = optValues(o).filter(function (v) { return !f || (v.name + ' ' + v.value).toLowerCase().indexOf(f) >= 0; });
      var pins = all.filter(function (v) { return pinned.indexOf(v.value) >= 0; });
      var rest = all.filter(function (v) { return pinned.indexOf(v.value) < 0; });
      function section(name, vals) {
        if (!vals.length) return;
        var h = document.createElement('div');
        h.className = 'md-section-label';
        h.textContent = name;
        host.appendChild(h);
        vals.forEach(function (v) {
          var r = document.createElement('div');
          r.className = 'md-row model-row';
          var fav = pinned.indexOf(v.value) >= 0;
          if (opts.choose) {
            r.innerHTML = '<a class="md-item" href="#" role="option"></a><button type="button" class="icon-btn model-pin"><span class="glyph" style="--glyph: var(--glyph-pin)" aria-hidden="true"></span></button>';
            var a = r.firstChild, pin = r.lastChild;
            modelLabel(a, v.name, modelPrice(chatProvider(), v.value));
            if (v.description) a.title = v.description;
            if (v.value === opts.current) { a.setAttribute('aria-current', 'true'); a.setAttribute('aria-selected', 'true'); }
            a.addEventListener('click', function (e) { e.preventDefault(); opts.choose(v.value); });
            pin.setAttribute('aria-pressed', String(fav));
            pin.setAttribute('aria-label', (fav ? 'Remove ' : 'Add ') + v.name + (fav ? ' from' : ' to') + ' the favorites');
            pin.title = fav ? 'A favorite. Press to remove it.' : 'Make it a favorite';
            pin.addEventListener('click', function () { togglePin(v.value, !fav).then(opts.redraw); });
          } else {
            r.innerHTML = '<label class="check fav-check"><input type="checkbox"> <span></span></label>';
            var box = r.querySelector('input');
            box.checked = fav;
            r.querySelector('span').textContent = v.name;
            box.addEventListener('change', function () { togglePin(v.value, box.checked).then(opts.redraw); });
          }
          host.appendChild(r);
        });
      }
      section('Favorites', pins);
      section(pins.length ? 'All Models' : 'Models', rest);
      if (!all.length) host.innerHTML = '<p class="form-help menu-note">No model matches the filter.</p>';
      var cur = host.querySelector('[aria-current]');
      if (cur && !f) cur.scrollIntoView({ block: 'nearest' });
    }
    function togglePin(value, on) {
      var id = chatProvider();
      return (app().SetPinnedForProvider ? app().SetPinnedForProvider(id, value, on) : app().SetPinned(value, on)).then(function (p) {
        providerPins[id] = p || [];
        if (id === defaultProvider) pinned = p || [];
        drawDefaults();
      });
    }

    function drawModelsDialog() {
      var model = byCat(barOptions(), 'model');
      $('models-help').textContent = active
        ? 'Every model ' + agentName() + ' offers. The choice applies to this session; the pin makes a model a favorite.'
        : 'Every model ' + agentName() + ' offers. The choice applies to the session your next message starts; the pin makes a model a favorite.';
      if (!model) { $('models-list').innerHTML = '<p class="form-help menu-note">Available models are not known yet. Test Connection in Settings.</p>'; return; }
      modelRows($('models-list'), model, {
        filter: $('models-filter').value, current: model.currentValue,
        choose: function (v) { setSessionOption(model.id, v).then(function () { $('models').close(); }); },
        redraw: drawModelsDialog
      });
    }
    function openModels() {
      if (!byCat(barOptions(), 'model')) return;
      $('models-filter').value = '';
      drawModelsDialog();
      if (!$('models').open) $('models').showModal();
      $('models-filter').focus();
      // The list is drawn at once; Devin's prices, when read now, draw it
      // again as they arrive.
      loadModelPrices(chatProvider());
    }
    function pricesChanged() {
      renderComposerOptions();
      if ($('models').open && currentChat() === controller) drawModelsDialog();
      if ($('usage-menu').matches(':popover-open') && currentChat() === controller) drawUsage();
    }
    $('models-filter').addEventListener('input', drawModelsDialog);


    /* ---- History and saving ----------------------------------------------- */
    var historyEntries = [];
    function drawHistory() {
      var list = $('history-list'), f = $('history-filter').value.trim().toLowerCase();
      list.innerHTML = '';
      var kinds = { send: '', steer: 'steering', queue: 'queued' };
      var shown = 0;
      for (var i = historyEntries.length - 1; i >= 0; i--) {
        var e = historyEntries[i];
        if (f && e.text.toLowerCase().indexOf(f) < 0) continue;
        var li = document.createElement('li');
        li.innerHTML = '<button type="button" class="history-item"><span class="history-meta"></span><span class="history-text"></span></button>';
        li.querySelector('.history-meta').textContent = when(e.time) + (kinds[e.kind] ? ', ' + kinds[e.kind] : '') + (e.doc ? ', ' + e.doc.title : '');
        li.querySelector('.history-text').textContent = e.text;
        (function (entry) {
          li.firstChild.addEventListener('click', function () {
            $('prompt').value = entry.text;
            if (entry.doc) setContext(entry.doc);
            typed();
            $('history').close();
            $('prompt').focus();
          });
        })(e);
        list.appendChild(li);
        if (++shown >= 300) break;
      }
      $('history-empty').hidden = shown > 0;
    }
    function openHistory() {
      app().History().then(function (h) {
        historyEntries = h || [];
        $('history-filter').value = '';
        drawHistory();
        if (!$('history').open) $('history').showModal();
        $('history-filter').focus();
      });
    }
    $('history-filter').addEventListener('input', drawHistory);
    $('history-save').addEventListener('click', function () {
      app().SaveHistory().then(function (p) { if (p) toast('Saved the history to ' + p + '.', 'positive'); }).catch(fail);
    });
    $('history-clear').addEventListener('click', function () {
      ask('Forget the Prompt History?', 'Every prompt in the history is forgotten. This cannot be undone.', 'Forget Them', true).then(function (go) {
        if (go) app().ClearHistory().then(function () { recall.clear(); historyEntries = []; drawHistory(); });
      });
    });

    /* Save the Session writes the session in front as Markdown: the messages
       in full, and a line for each tool call, file and turn. */
    function runMarkdown(s) {
      var out = ['# ' + title(s), '', productName() + ' session, saved ' + dinahRegional.format(new Date(), 'dateTime') + '.', ''];
      s.log.querySelectorAll('li.activity-row:not(.tool-group)').forEach(function (li) {
        var time = (li.querySelector('.activity-time') || {}).textContent || '';
        if (li._msg) {
          out.push('## ' + li._msg.label + ', ' + time, '', li._msg.text.trim(), '');
        } else if (li.classList.contains('thought-row')) {
          out.push('*' + time + ', thinking:* ' + (li._thought ? li._thought.text : li.textContent.replace(time, '').trim()), '');
        } else if (li._tool) {
          var t = li._tool;
          out.push('- ' + time + ' ' + t.name + ' (' + (STATUS[t.status] || ['', t.status])[1] + ')');
          if (t.text) out.push('', '  ```text', t.text.split('\n').map(function (l) { return '  ' + l; }).join('\n'), '  ```', '');
        } else {
          out.push('- ' + time + ' ' + li.textContent.replace(time, '').trim());
        }
      });
      return out.join('\n') + '\n';
    }
    function saveRun() {
      var s = active && sessions[active];
      if (!s || !s.log.children.length) { toast('The session is empty.'); return; }
      app().SaveRun(runMarkdown(s)).then(function (p) { if (p) toast('Saved the session to ' + p + '.', 'positive'); }).catch(fail);
    }


    function setContext(doc) {
      context = doc;
      $('context').hidden = !doc;
      if (doc) $('context-chip').textContent = 'Document: ' + doc.title + (doc.selection ? ', selection' : '');
      $('context-clear').title = $('context-clear').ariaLabel = 'Clear the document';
    }

      $('context-clear').addEventListener('click', function () {
        var was = context;
        setContext(null);
        // A cleared document stays cleared until its window comes forward again.
        if (was && was.draftId) document.dispatchEvent(new CustomEvent('dinah:context-cleared', { detail: { draftId: was.draftId } }));
      });
      /* The chat's commands, declared once for PUDL's commands
         (pudl-commands.js): the window's menu names them by id, and an agent
         lists and runs them as pressing them would. The composer's own
         (Send, Steer, Drop the Queue, Clear the Document) read their state
         from the composer's controls, which keep it. */
      var unregisterCommands = null;
      /* Subagents is a menu button in the composer's Turn group and a submenu
         of the window's Actions menu, there only while the chat has
         subagents. Its count and its rows are this run's subagents, those the
         tree shows, running or ended. Its menu lists each one in the tree's
         order, a subagent's own after it, with its state as a glyph and a
         word, and choosing one opens its window as the row's Open in the chat
         does. Show All Subagents opens the tree, and Earlier Subagents…, at
         the foot and only while the chat has subagents from earlier runs,
         opens the archive of those. */
      function chatSubagents() { return active ? currentSubagents(sessions[active]) : []; }
      function chatEarlierSubagents() { return active ? earlierSubagents(sessions[active]) : []; }
      function subagentsCommand() {
        var list = chatSubagents(), earlier = chatEarlierSubagents().length, running = runningSubagents(sessions[active]);
        return { id: 'subagents', label: 'Subagents', glyph: 'subagents', badge: list.length ? String(list.length) : null, hidden: !(list.length || earlier) || !window.dinahSubagents,
          detail: list.length ? (list.length === 1 ? '1 subagent' : list.length + ' subagents') + (running ? ', ' + running + ' running' : '')
            : earlier === 1 ? '1 from an earlier run' : earlier + ' from earlier runs',
          items: function () {
            var rows = subagentItems();
            if (rows.length) rows = rows.concat(['-', 'show-subagents']);
            if (chatEarlierSubagents().length) rows = rows.concat(rows.length ? ['-', 'earlier-subagents'] : ['earlier-subagents']);
            return rows;
          } };
      }
      function subagentItems() {
        var id = active, list = chatSubagents(), ws = id && sessions[id].info.cwd || workspace;
        if (!id || !window.dinahSubagents) return [];
        var children = window.dinahSubagents.tree(list, id), names = {}, out = [];
        list.forEach(function (sub) { names[sub.id] = sub.name || 'Subagent'; });
        (function walk(parent) {
          (children[parent] || []).forEach(function (sub) {
            var st = window.dinahSubagents.state(sub), under = parent !== id ? ', under ' + names[parent] : '';
            out.push({ id: 'open-subagent:' + sub.id, label: sub.name || 'Subagent', glyph: st.glyph, tone: st.tone || null,
              detail: st.word + under + (sub.task ? ': ' + sub.task : ''),
              run: function () { return window.dinahSubagents.open(id, sub.id, ws, sub.name).catch(fail); } });
            out[out.length - 1].rank = sub.state === 'running' ? 0 : 1;
            walk(sub.id);
          });
        })(id);
        /* Paul asked on 28 October 2026 for the running subagents at the top,
           since they are the ones he is most likely to open. The menu lists
           those first, then those that ended in this run, each in the tree's
           order; a nested one still says which subagent it is under. Those
           from earlier runs are in the archive. */
        return out.map(function (row, i) { return { row: row, i: i }; })
          .sort(function (a, b) { return (a.row.rank - b.row.rank) || (a.i - b.i); })
          .map(function (x) { delete x.row.rank; return x.row; });
      }
      function chatCommandTable() {
        var pin = active && sameWs(workspace) && window.dinahPins && window.dinahPins.command(chatPin(active));
        return [
          { id: 'new-chat', actions: true, label: 'New Chat', run: startNewChat, disabled: !workspace, reason: workspace ? null : 'Open a workspace first.' },
          { id: 'rename-chat', actions: true, label: 'Rename Chat…', run: function () { if (active) startRename(active, 'head'); }, disabled: !active, shortcut: 'F2' },
          // A continuation into another workspace (switch.go): confirmed first,
          // visibly in progress while it runs, unchanged where it fails.
          { id: 'continue-workspace', actions: true, label: 'Continue in Another Workspace…', detail: 'A new chat with the same provider in the workspace you choose; this one stays as it is', disabled: !active || !app().ContinueSessionInWorkspace, reason: 'The chat starts with its first message.', run: continueInWorkspace },
          // A chat of the open workspace can be pinned to it.
          pin ? Object.assign({}, pin, { id: 'pin-chat', actions: true }) : { id: 'pin-chat', actions: true, label: 'Pin to Workspace', run: function () {}, disabled: true, reason: 'Only a chat of the open workspace can be pinned to it.' },
          // Group chats (bus-applets.js): this chat and others the person
          // chooses, or this chat put in a group the person already has.
          { id: 'start-group', actions: true, label: 'Start a Group Chat…', detail: 'With this chat and others you choose', disabled: !active || !window.dinahBus,
            reason: 'The chat starts with its first message.', run: function () { if (active) return window.dinahBus.startGroup(active); } },
          { id: 'add-to-group', actions: true, label: 'Add to a Group Chat…', detail: 'Put this chat in a group you already have', disabled: !active || !window.dinahBus,
            reason: 'The chat starts with its first message.', items: function () { return active && window.dinahBus ? window.dinahBus.addItems(active) : []; } },
          { id: 'remove-chat', actions: true, label: providerFact(chatProvider(), 'removal') === 'hides' ? 'Hide Chat…' : 'Remove Chat…', run: function () { if (active) openRemove(active); }, disabled: !active, danger: providerFact(chatProvider(), 'removal') !== 'hides' },
          Object.assign(providerCommand(), { actions: true }), Object.assign(modeCommand(), { actions: true }), modelCommand(),
          { id: 'choose-model', actions: true, label: 'Choose the Model…', run: openModels, disabled: !byCat(barOptions(), 'model') },
          { id: 'add', label: 'Add', glyph: 'clip', items: addItems },
          { id: 'skills', label: 'Skills', glyph: 'slash', items: skillItems },
          { id: 'prompt-history', actions: true, label: 'Prompt History…', run: openHistory },
          { id: 'my-templates', actions: true, label: 'My Templates…', run: function () { openTemplates(null); } },
          { id: 'export-chat', actions: true, label: 'Export Chat…', run: saveRun, disabled: !active },
          { id: 'failure-summaries', actions: true, label: 'Show Failed Command Summaries', checked: !!active && window.dinahFailures.enabled(active), disabled: !active,
            run: function () { if (active) window.dinahFailures.setEnabled(sessions[active], !window.dinahFailures.enabled(active)); } },
          // The chat's log on disk: every prompt, thought, tool call and reply,
          // including what has left the window.
          { id: 'show-chat-log', actions: true, label: 'Show the Chat Log', run: function () { var id = active; Promise.resolve().then(function () { return app().ShowChatLog(id); }).catch(fail); }, disabled: !active },
          // The chat's subagents, and the tree of them (subagents.js), are
          // absent until the agent starts one.
          Object.assign(subagentsCommand(), { actions: true }),
          { id: 'show-subagents', label: 'Show All Subagents', run: function () { if (active && window.dinahSubagents) return window.dinahSubagents.openTree(active, sessions[active].info.cwd || workspace).catch(fail); },
            hidden: !chatSubagents().length },
          { id: 'earlier-subagents', label: 'Earlier Subagents…',
            run: function () { if (active && window.dinahSubagents) return window.dinahSubagents.openArchive(active, sessions[active].info.cwd || workspace).catch(fail); },
            hidden: !chatEarlierSubagents().length || !window.dinahSubagents },
          // A chat reopened without the agent replaying its history can ask for the whole of it.
          // A chat that remembered what to do when its context fills can be asked again.
          { id: 'forget-context-choice', actions: true, label: 'Ask Me Again About a Full Context', glyph: 'reload', run: function () { if (active) return Promise.resolve(app().ForgetContextChoice(active)).catch(fail); },
            hidden: !(active && sessions[active].info.choice) },
          { id: 'load-full-history', actions: true, label: 'Load Full History', glyph: 'reload', run: function () { if (active) return loadFullHistory(active).catch(fail); },
            hidden: !(active && sessions[active].info.resumed && sessions[active].info.open), disabled: working, reason: working ? 'The chat is working. Load Full History is available when it has finished.' : null },
          // Stop keeps a separate slot beside the folding toolbar while idle.
          // While the chat says Stopping, the control stays pressable: pressing it asks again.
          { id: 'stop-turn', actions: true, label: stopping() ? 'Stopping…' : 'Stop', glyph: 'square', danger: true, run: stopTurn, hidden: !working,
            detail: stopping() ? 'Stop was asked for. Press again to ask again.' : undefined },
          { id: 'send', label: working ? 'Queue' : 'Send', disabled: $('send').disabled, reason: $('send').disabled ? 'The message is empty.' : null, run: function () { send('queue'); } },
          { id: 'steer', label: 'Steer', hidden: !working, disabled: $('steer-send').disabled, run: function () { send('steer'); } },
          // Steer gives a Claude chat the message at its next step; this stops
          // the step first, for when the agent should drop what it is doing.
          { id: 'stop-and-steer', label: 'Stop and Steer', hidden: !working, disabled: $('steer-send').disabled, run: function () { send('stop-steer'); } },
          { id: 'drop-queue', actions: true, label: 'Drop the Queued Messages', glyph: 'close', hidden: $('queue').hidden, run: function () { if (active) app().Unqueue(active); } },
          { id: 'clear-document', label: 'Clear the Document', hidden: $('context').hidden, run: function () { $('context-clear').click(); } }
        ].map(function (command) {
          if (teamPanel.isMember() && ['send', 'steer', 'stop-and-steer', 'add', 'skills', 'prompt-history', 'my-templates', 'clear-document'].indexOf(command.id) >= 0) return Object.assign({}, command, { hidden: true });
          return command;
        }).concat(teamPanel.commands().map(function (command) { return Object.assign({}, command, { actions: true }); }));
      }
      var controller = {
        commandEnabled,
        restoreMessageImages: function (message, images) {
          message.images = images;
          dirty.add(message);
        },
        // A provider's limit on the files in one message changed in Settings.
        redrawAttachments: function () { drawAttachments(); },
        applySkillCatalog: function (catalog) {
          if (!app().SkillCatalog) return;
          skills=catalog.skills.filter(function (skill) {return (skill.scope!=='workspace'||skill.workspace===workspace)&&(skill.scope!=='provider'||providerFact(chatProvider(),'ownCommands'));}); skillsChanged();
        },
        skillTarget: function () { if (teamPanel.isMember()) return null; var targetWorkspace=workspace, targetProvider=chatProvider(); return {title: active ? title(sessions[active]) : 'New Chat', workspace: targetWorkspace, provider: targetProvider, put: function (text) {
          if(workspace!==targetWorkspace||chatProvider()!==targetProvider)return Promise.reject(new Error('The chat changed workspace or provider. Choose it again.'));
          return dinahWorkbench.open(active ? {kind:'chat-session',workspace:workspace,path:active} : 'chat').then(function () {
            closeForm(true); var box=$('prompt'); box.value += (box.value.trim() ? '\n\n' : '') + text; typed(); box.focus();
          });
        }}; },
        id: view.id, recoverSteering: recoverSteering, row: row, drawAll: drawAll, stampReplay: stampReplay, trimMarker: trimMarker, replayProgress: replayProgress, replayDone: replayDone, scroll: scroll,
        prepareHistoryLayout: function (log, position) { navigation.resetMessages(); var saved = window.dinahUIState && dinahUIState.get('dinah-scroll:' + key + ':activity'); log.toggleAttribute('data-preserve-scroll-layout', !!(position ? !position.following : saved && !saved.end)); },
        historyPosition: function (position) { return navigation.position(position); },
        scrollReady: function () { if (window.dinahUIState) window.dinahUIState.scrollReady($('activity')); },
        sessionNote: sessionNote, stateLine: stateLine, setWorking: setWorking, drawQueue: drawQueue, drawApprovals: drawApprovals,
        renderComposerOptions: renderComposerOptions, menuStates: menuStates, loadSkills: loadSkills,
        provider: chatProvider, paste: paste,
        acceptRendering: function (messages, tools, groups, plans, ready) {
          messages.forEach(function (m) { dirty.add(m); });
          tools.forEach(function (r) { dirtyTools.add(r); });
          groups.forEach(function (g) { dirtyGroups.add(g); });
          plans.forEach(function (r) { dirtyPlans.add(r); });
          renderReady = renderReady.concat(ready);
          scheduleDraw();
        },
        transferRendering: function (next) { next.acceptRendering(dirty, dirtyTools, dirtyGroups, dirtyPlans, renderReady); },
        prependContinuation: prependContinuation, refreshMentions: refreshMentions,
        updateDefaultProvider: function () { if (!active) { chosenProvider = defaultProvider; pending = {}; renderComposerOptions(); loadSkills(); } drawAgentState(); },
        loadRecall: loadRecall, forgetDraft: function () { if (draftKeeper) { draftKeeper.rekey(draftKey()); draftKeeper.forget(); } }, signIn: signIn, openModels: openModels, pricesChanged: pricesChanged, openHistory: openHistory,
        openTemplates: openTemplates, saveRun: saveRun, typed: typed, setContext: setContext,
        stopTurn: stopTurn, drawLoadingChip: drawLoadingChip,
        refresh: function () {
          show(active);
          drawSwitchState(active && sessions[active]);
          askImageInput(); // the note beside a picture depends on what the engine says of the chat's model
          var win = root.closest('[data-win]');
          if (win && window.pudlWindows) {
            var s = active && sessions[active], named = s ? title(s) : 'New Chat';
            var titleEl = win.querySelector('.win-title'), was = titleEl && titleEl.textContent.trim();
            // A retitle redraws the docks and the window rows, so the title
            // is set only when it changed.
            if (was !== named) window.pudlWindows.retitle(key, named);
            var work = backgroundWork(s), helpers = runningSubagents(s);
            var execution = syncWindowExecution(win, s);
            // PUDL owns execution marks. Host statuses retain lifecycle,
            // background work and subagent details (WINDOW-STATUS.md).
            // The group is in the window's markup from the start
            // (workbench.js, statuses); PUDL draws the chips, folds them into
            // its Window status control when the bar is narrow, and keeps its
            // own button in the group, so only the chat's spans are changed.
            var group = win.querySelector('.win-head > .win-statuses');
            if (group) {
              var chip = function (cls, strong) {
                var el = group.querySelector('.' + cls);
                if (!el) { el = document.createElement('span'); el.className = 'win-status ' + cls + (strong ? ' strong' : ''); group.appendChild(el); }
                return el;
              };
              drawWindowLifecycle(chip('chat-working-chip'), s, execution);
              drawLoadingChip();
              var backgroundChip = chip('chat-background-chip');
              backgroundChip.hidden = !work.length;
              backgroundChip.textContent = 'Background ' + work.length;
              backgroundChip.title = work.length === 1 ? '1 background task is running' : work.length + ' background tasks are running';
              // The chip only reports how many subagents run. PUDL's status
              // chips hold no controls (WINDOW-STATUS.md), so the action is
              // the Subagents menu button in the composer's toolbar.
              var subagentsChip = chip('chat-subagents-chip');
              subagentsChip.hidden = !helpers;
              subagentsChip.textContent = 'Subagents ' + helpers;
              subagentsChip.title = helpers === 1 ? '1 subagent is running' : helpers + ' subagents are running';
            }
            win.dataset.winAttention = String(active ? needsCount(active) : 0);
          }
        },
        setWorkspace: function (ws) {
          if (active) return;
          workspace = ws;
          loadSkills();
          stateLine();
        },
        attach: function (host, options) {
          appletOptions = options;
          host.appendChild(root);
          // The chat's commands belong to the window it is shown in.
          if (unregisterCommands) unregisterCommands();
          unregisterCommands = window.pudlCommands ? window.pudlCommands.register(host, chatCommandTable) : null;
          markCurrentChat();
          controller.refresh();
        },
        focus: function () { if (teamPanel.isMember()) teamPanel.focus(); else $('prompt').focus(); },
        // The window's Actions menu, by the ids of the chat's commands.
        commands: function () { return chatCommandTable().filter(function (command) { return command.actions; }).map(function (command) { return command.id; }); },
        destroy: function () {
          switchEpoch++;
          if (cancelSwitchDialog) cancelSwitchDialog();
          if (unregisterCommands) { unregisterCommands(); unregisterCommands = null; }
          if (deleted) root.remove(); else document.getElementById('applet-storage').appendChild(root);
          appletOptions = null;
        },
        dispose: function () { switchEpoch++; if (cancelSwitchDialog) cancelSwitchDialog(); composerLayout.destroy(); teamPanel.destroy(); navigation.destroy(); deleted = true; document.removeEventListener('dinah:appearance-before', captureAppearance); document.removeEventListener('dinah:appearance-change', restoreAppearance); ['wheel', 'touchstart', 'keydown'].forEach(function (name) { $('activity').removeEventListener(name, cancelAppearanceAnchor); }); window.dinahRender.cancel(renderKey); window.dinahRender.cancel(drawUsage); dirty.clear(); dirtyTools.clear(); dirtyGroups.clear(); dirtyPlans.clear(); renderReady = []; if (unregisterCommands) { unregisterCommands(); unregisterCommands = null; } listeners.forEach(function (stop) { if (stop) stop(); }); root.remove(); }
      };
      chatControllers.set(key, controller);
      if (active) { var s = S(active); s.view = controller; $('logs').appendChild(s.log); }
      draftKeeper = window.dinahComposer.keeper(draftKey(), restoreDraft);
      loadSkills(); loadRecall(); loadAppletMentions();
      show(active);
      return controller;
    }
    function markCurrentChat() {
      document.querySelectorAll('.win[data-win-kind="chat"], .win[data-win-kind="chat-session"]').forEach(function (win) {
        var item = window.dinahWorkbench.item(win.dataset.win);
        win.toggleAttribute('data-chat-current', active ? !!item && item.path === active : win.dataset.win === 'chat');
      });
    }
    function selectChat(id) { active = id; markCurrentChat(); currentChat()?.menuStates(); drawSessions(); drawAgentState(); }
    /* New Chat asks which workspace the chat belongs to (docs/plans/
       chat-workspaces-and-switching.md): the one in front is the default, and
       nothing is asked when only one is configured. The answer switches the
       workspace in front, since a chat is made in the window's workspace and
       the page follows a switch the way the pane's own choice is followed. */
    /* Choosing a workspace: New Chat at its start, and a chat continued into
       another one (docs/plans/chat-workspaces-and-switching.md). o: title,
       help, yes. Answers the record, or null when cancelled. */
    function askWorkspaceChoice(records, currentPath, o) {
      return new Promise(function (resolve) {
        var dlg = document.createElement('dialog');
        dlg.className = 'dialog dinah-new-chat-workspace';
        var form = document.createElement('form');
        form.method = 'dialog';
        var title = document.createElement('h2'); title.className = 'dialog-title'; title.textContent = o.title;
        var body = document.createElement('div'); body.className = 'dialog-body';
        var group = document.createElement('div'); group.className = 'form-group';
        var label = document.createElement('label'); label.className = 'form-label'; label.textContent = 'Workspace';
        var wrap = document.createElement('span'); wrap.className = 'form-select-wrap';
        var select = document.createElement('select'); select.className = 'form-select'; select.id = o.selectId || 'new-chat-workspace';
        label.htmlFor = select.id;
        records.forEach(function (record, i) {
          var name = record.name || record.executionPath || record.id || '';
          var path = record.executionPath || '';
          select.appendChild(new Option(name + (path ? ' (' + path + ')' : ''), String(i), false, (record.executionPath || '') === currentPath));
        });
        var help = document.createElement('p'); help.className = 'form-help';
        help.textContent = o.help;
        group.append(label, wrap);
        wrap.appendChild(select);
        body.append(group, help);
        var actions = document.createElement('div'); actions.className = 'dialog-actions';
        var cancel = document.createElement('button'); cancel.type = 'button'; cancel.className = 'btn'; cancel.textContent = 'Cancel';
        var yes = document.createElement('button'); yes.type = 'submit'; yes.className = 'btn btn-primary'; yes.textContent = o.yes || 'Continue';
        actions.append(cancel, yes);
        form.append(title, body, actions);
        dlg.appendChild(form);
        var done = false;
        function finish(record) { if (done) return; done = true; if (dlg.open) dlg.close(); dlg.remove(); resolve(record); }
        dlg.addEventListener('close', function () { finish(null); });
        cancel.addEventListener('click', function () { finish(null); });
        form.addEventListener('submit', function (event) { event.preventDefault(); finish(records[Number(select.value)] || null); });
        document.body.appendChild(dlg);
        dlg.showModal();
        select.focus();
      });
    }
    function askNewChatWorkspace(records, currentPath) {
      return askWorkspaceChoice(records, currentPath, {
        title: 'New Chat', yes: 'New Chat', selectId: 'new-chat-workspace',
        help: 'The chat belongs to that workspace: its agent works in its folder, and its skills, files and permissions are the workspace’s own.'
      });
    }
    function startNewChat() {
      if (!adapter.workspace()) { noWorkspaceYet(); return Promise.resolve(); }
      return Promise.resolve().then(function () { return app().State(); }).then(function (state) {
        var records = (state && state.workspaceRecords) || [];
        if (records.length < 2) return false;
        return askNewChatWorkspace(records, adapter.workspace());
      }).then(function (record) {
        if (record === null) return false; // cancelled
        var start = function () { show(null); return showChat(); };
        if (!record || !record.executionPath || (record.executionPath || '') === adapter.workspace()) return start();
        /* The pane stays where the person has it: the draft chat belongs to
           the chosen workspace, whose folder its session takes when it starts
           (NewSessionAt, or NewSessionWithProviderAt with the provider then
           chosen), and nothing else moves (Paul's stacking decision). If the
           call fails at then, no chat is made. */
        return start().then(function () {
          setDraftWorkspace(record.executionPath);
        });
      }).catch(fail);
    }
    function setupChatWindows() {
      function init(root, opts, item) {
        var s = item.kind === 'chat-session' ? S(item.path) : null;
        // A chat is drawn in one window. A second window for it, such as one
        // a layout kept from opening the chat under another workspace, closes
        // and the first is brought forward; left open it would stay blank.
        var own = root.closest('[data-win]') && root.closest('[data-win]').dataset.win;
        var other = s && chatWindowsFor(s.id).filter(function (key) { return key !== own && window.pudlApplets.started && window.pudlApplets.started(key); })[0];
        if (other) {
          setTimeout(function () { window.pudlWindows.close(own); window.pudlWindows.raise(other); }, 0);
          return { ready: Promise.resolve(), destroy: function () {} };
        }
        if (s && !s.info.cwd) s.info.cwd = item.workspace;
        var controller = ensureChat(s);
        controller.attach(root, opts);
        /* The window is ready once the chat is attached and drawn. Restoring
           the chat, which may reconnect to the agent and replay its history,
           goes on behind its own progress line; it must not hold up the
           windows PUDL starts after the ones in sight. */
        if (s && (!s.info.open || (!s.saved && !s.loading && !s.log.firstChild))) Promise.resolve().then(function () { return restoreChat(s); }).catch(function () {});
        controller.ready = Promise.resolve();
        return controller;
      }
      // New Chat starts its chat in the workspace chosen in the Workspace
      // pane (setDraftWorkspace), so its window is that workspace's.
      window.dinahWorkbench.register('chat', { applet: { id: 'dinah.chat', title: 'Chats', glyph: 'comments', workspace: true }, singleton: true, title: 'New Chat', glyph: 'comments', statuses: true, init: init, workspaceOf: function () { return window.dinahHost.workspaceId(); } });
      // A chat restored behind other windows starts when it is seen, or in idle
      // time after the windows in sight.
      window.dinahWorkbench.register('chat-session', { applet: { id: 'dinah.chat', title: 'Chats', glyph: 'comments', workspace: true }, title: 'Chat', glyph: 'comments', start: 'idle', statuses: true, init: init });
      document.addEventListener('pudl:windows-change', function () {
        var key = window.pudlWindows.state().top, item = window.dinahWorkbench.item(key);
        if (key === 'chat') selectChat(null);
        else if (item && item.kind === 'chat-session') selectChat(item.path);
        else drawAgentState();
      });
    }

    function setContext(doc) {
      context = doc;
      var chat = currentChat();
      if (chat) chat.setContext(doc);
    }

    /* ---- Document windows ------------------------------------------------ */
    /* The Editor in front tells the chat about its document, which the chat
       sends with its next messages until the person clears it. */
    var clearedDraft = '';
    document.addEventListener('dinah:context-cleared', function (e) { clearedDraft = e.detail.draftId; });
    document.addEventListener('dinah:item-context', function (e) {
      var doc = e.detail;
      if (doc && doc.draftId) {
        if (doc.draftId === clearedDraft && !doc.raised) return;
        clearedDraft = '';
      }
      setContext(doc);
    });
    document.addEventListener('pudl:window-close', function (e) {
      var item = window.dinahWorkbench.item(e.detail.key);
      if (!item || !context) return;
      if (item.kind === 'editor' ? context.draftId === item.path : item.workspace === context.workspace && item.path === context.path) setContext(null);
    });
    function showChat() {
      var s = active && sessions[active];
      // A chat already in a window is shown there, whichever workspace its
      // window was opened under; a chat has one window.
      var existing = s ? chatWindowsFor(s.id)[0] : null;
      if (existing) { window.pudlWindows.raise(existing); return Promise.resolve().then(function () { currentChat().focus(); }); }
      var item = s ? { kind: 'chat-session', workspace: s.info.cwd || adapter.workspace(), path: s.id } : 'chat';
      return window.dinahWorkbench.open(item).then(function () { currentChat().focus(); });
    }
    // The open windows that show a chat.
    function chatWindowsFor(id) {
      if (!window.pudlWindows) return [];
      return window.pudlWindows.state().open.filter(function (key) {
        var item = window.dinahWorkbench.item(key);
        return item && item.kind === 'chat-session' && item.path === id;
      });
    }


    function disposeChat(session) {
      if (!session || !session.view) return;
      const key = chatKey(session.id, session.info.cwd);
      window.pudlWindows.close(key);
      session.view.dispose();
      chatControllers.delete(key);
    }
    function setDraftWorkspace(path) {
      const chat = chatControllers.get('chat');
      if (chat) chat.setWorkspace(path);
    }
    function restorePreferences(state) {
      catalog = state.catalog || [];
      defaultProvider = state.defaultProvider || '';
      drawAgentState();
      providerCatalogs[defaultProvider] = catalog;
      defaults = state.defaults || {};
      pinned = state.pinned || [];
      providerDefaults[defaultProvider] = defaults;
      providerPins[defaultProvider] = pinned;
    }
    function restoreView() {
      const frontItem = window.pudlWindows && window.dinahWorkbench.item(window.pudlWindows.state().top);
      show(frontItem && frontItem.kind === 'chat-session' ? frontItem.path : null);
      setDraftWorkspace(adapter.workspace());
      chatControllers.forEach(chat => chat.updateDefaultProvider());
      currentChat()?.loadRecall();
      currentChat()?.loadSkills();
    }
    return {
      commandEnabled: id => !!currentChat()?.commandEnabled(id),
      ensure: ensureChat, current: currentChat, active: () => active,
      providerName, frontProvider, defaultProvider: () => defaultProvider,
      chatWorking, backgroundWork, earlierSubagents,
      show, showChat, openSession, reconnectChat, startNewChat, restartAgent, signIn,
      setContext, disposeChat, setDraftWorkspace, restorePreferences, restoreView, setupChatWindows,
      setTemplates: value => { templates = value; },
      refreshChatTitles: () => chatControllers.forEach(chat => chat.stateLine()),
      refreshStates: () => chatControllers.forEach(chat => chat.refresh()),
      settings: {
        app, on, toast, fail, ask, productName, providerName, providerOptions,
        optValues, byCat, valueWords, modelRows, loadModelPrices, drawAgentState,
        catalog: () => catalog, defaults: () => defaults, pinned: () => pinned,
        providers: () => providers, defaultProvider: () => defaultProvider,
        providerCatalogs: () => providerCatalogs, providerDefaults: () => providerDefaults,
        providerPins: () => providerPins, fileLimits: () => fileLimits,
        optionSubstitutions: () => optionSubstitutions,
        setDefaults: value => { defaults = value; },
        setCatalog: value => { catalog = value; },
        setProviders: value => { providers = value; refreshSkillGroups(); },
        replaceProvider: result => { providers = providers.map(entry => entry.id === result.id ? result : entry); refreshSkillGroups(); },
        setProviderCatalog: (id, value) => { providerCatalogs[id] = value; },
        setProviderDefaults: (id, value) => { providerDefaults[id] = value; },
        setProviderPins: (id, value) => { providerPins[id] = value; },
        setFileLimits: value => { fileLimits = value; },
        setOptionSubstitutions: value => { optionSubstitutions = value; },
        selectPreferences: id => {
          catalog = providerCatalogs[id]; defaults = providerDefaults[id]; pinned = providerPins[id];
        },
        applyDefaultProvider: state => {
          defaultProvider = state.defaultProvider;
          catalog = state.catalog || []; defaults = state.defaults || {}; pinned = state.pinned || [];
          providerCatalogs[defaultProvider] = catalog; providerPins[defaultProvider] = pinned; providerDefaults[defaultProvider] = defaults;
        },
        refreshComposerOptions: () => chatControllers.forEach(chat => chat.renderComposerOptions()),
        redrawAttachments: () => chatControllers.forEach(chat => { if (chat.redrawAttachments) chat.redrawAttachments(); }),
        updateDefaultProvider: () => chatControllers.forEach(chat => chat.updateDefaultProvider())
      }
    };

  }

  window.dinahChat = { create };
})();
