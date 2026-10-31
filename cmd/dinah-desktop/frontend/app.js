/* The page shell wires applets to host services and starts the saved layout. */
(function () {
  'use strict';

  const $ = id => document.getElementById(id);
  const app = window.dinahHelpers.app;
  // The product's name as people see it (product.js).
  function productName() { return window.dinahProduct ? window.dinahProduct.name : ''; }

  const on = window.dinahHost.on;
  const events = window.dinahHelpers.events;
  events.route(function (name, data) {
    if (name.indexOf('agent:') === 0) queueAgentEvent(name, data);
    else dispatchEvent(name, data);
  });
  // A resync rereads state only after the agent events queued before it are applied.
  events.drainer(function () { return drainAgentEvents(); });
  var agentEvents = [], agentEventHead = 0, agentEventKey = {};
  /* A numbered chat event goes through the sequencer (chat-sequence.js), which
     applies each chat's events in order, once; everything else is delivered. */
  // The events that say the whole of some state are applied only if no newer
  // snapshot of it has been (state-order.js). agent:session is ordered where its
  // information is applied, since that has other readers than the event.
  const STATE_OWNER = { 'agent:queue': d => d.sessionId, 'agent:options': d => d.sessionId, 'agent:state': d => d.provider, 'agent:account': d => d.provider, 'settings:changed': () => '', 'update:state': () => '' };
  function dispatchEvent(name, data) {
    if (STATE_OWNER[name] && data && !window.dinahStateOrder.accept(name, STATE_OWNER[name](data), data)) return;
    if (data && data.gen !== undefined && data.n !== undefined && data.sessionId) sequence.receive(name, data);
    else deliverEvent(name, data);
  }
  function deliverEvent(name, data) {
    if (data && data.sessionId && ['agent:update', 'agent:updates', 'agent:file', 'agent:turn', 'agent:sent', 'agent:teamsent', 'agent:arriving'].includes(name)) chats.ensure(S(data.sessionId));
    events.dispatch(name, data);
  }
  function queueAgentEvent(name, data, done) {
    agentEvents.push({ name: name, data: data, done: done, index: 0 });
    window.dinahRender.enqueue(agentEventKey, null, function (deadline) {
      var count = 0;
      while (agentEventHead < agentEvents.length && count++ < 128 && (count === 1 || performance.now() < deadline)) {
        var event = agentEvents[agentEventHead], complete = true;
        try {
          if (event.done) event.done();
          else if (event.name === 'agent:updates') {
            var updates = event.data.updates || [], ns = event.data.ns || [];
            // Each update of a batch carries its own number in the chat's log.
            if (event.index < updates.length) {
              var one = { sessionId: event.data.sessionId, update: updates[event.index] };
              if (event.data.gen !== undefined) { one.gen = event.data.gen; one.n = ns[event.index]; }
              event.index++;
              dispatchEvent('agent:update', one);
            }
            complete = event.index >= updates.length;
          } else dispatchEvent(event.name, event.data);
        } catch (error) { setTimeout(function (e) { return function () { throw e; }; }(error), 0); }
        if (complete) { agentEvents[agentEventHead++] = null; }
      }
      if (agentEventHead < agentEvents.length) return true;
      agentEvents = []; agentEventHead = 0;
      return false;
    }, null, 'provider events');
  }
  function drainAgentEvents() {
    if (agentEventHead === agentEvents.length) return Promise.resolve();
    return new Promise(function (resolve) { queueAgentEvent('', null, resolve); });
  }
  /* An error is shown as a toast and kept under Notifications, always
     both, so an error the person missed can be found again. options may
     give the draft the error is about, which its card reopens, and a title
     for the card; noted says the caller has made the card itself. */
  function toast(msg, kind, options) {
    options = options || {};
    if (window.pudlToast) window.pudlToast(options.title ? options.title + ' ' + msg : msg, { kind: kind || 'info' });
    if (!options.noted) noteEntry(String(msg), kind || 'info', options);
    // What the page shows as an error or a warning is in the log too.
    var level = kind === 'danger' || kind === 'error' ? 'error' : kind === 'warn' || kind === 'warning' ? 'warning' : 'info';
    if (app() && app().Log) Promise.resolve().then(function () { return app().Log('ui', level, (options.title ? options.title + ' ' : '') + String(msg)); }).catch(function () {});
  }
  function fail(err) { toast(String(err), 'danger'); }
  const notifications = window.dinahNotifications.create({
    app: app, on: on, element: function (id) { return document.getElementById(id); },
    session: function (id) { return S(id); }, getSession: function (id) { return sessions[id]; },
    sessionProvider: function (s) { return sessionProvider(s); }, workspace: function () { return workspace; },
    title: session => title(session), openSession: function (id) { return openSession(id); }, row: (session, cls) => chats.ensure(session).row(session, cls),
    now: () => dinahRegional.format(new Date(), 'clock'), drawSessions: () => drawSessions(), toast: toast, fail: fail
  });
  const { notices, ask, noteEntry, needsCount, drawNoticeCounts,
    flushNotifications, restoreNotifications, waitingApproval, permissionCard,
    headsUp, showNotifications, nextNotification, clearNotifications } = notifications;

  /* Host services for applets kept in their own files, so that each uses the
     page's one question dialog, its toasts and its one subscription per Wails
     event rather than keeping its own. */
  /* Find and Open asks each registered source for its items as it draws,
     so an applet adds what it holds (the Editor its drafts) without this
     page knowing about it. A source returns {title, detail, glyph?, open}. */
  Object.assign(window.dinahHost, { ask: ask, toast: toast, fail: fail, on: on, copy: window.dinahChatText.copy, workspaceId: function () { return workspaces.currentId(); },
    /* A document that could not be saved or published: a toast, and a card
       under Notifications that reopens it. */
    documentFailed: function (draftId, title, message) { toast(message, 'danger', { draftId: draftId, title: title }); },
    /* The notifications, for another module to add its entries to. */
    notices: notices,
    /* A chat's title as the lists show it. */
    chatTitle: function (id) { return title(S(id)); },
    findSource: function (fn) { return finder.addSource(fn); },
    /* A workspace's name as the panes show it. */
    workspaceLabel: function (path) { return workspaceLabel(path); } });

  /* ---- Sessions ---------------------------------------------------------
     Each session the page knows has its log (an <ol> under #logs), its tool
     rows, the message being streamed, its options and whether it works. */
  const sessionStore = window.dinahSessionStore.create({
    createLog: function () {
      var ol = document.createElement('ol');
      ol.className = 'activity run-log';
      ol.dataset.uiScrollIgnore = '';
      ol.hidden = true;
      ol.setAttribute('aria-live', 'polite');
      document.getElementById('applet-storage').appendChild(ol);
      return ol;
    },
    subagentsChanged: function (earlierChanged) {
      if (earlierChanged) drawSessions();
      if (window.pudlCommands) window.pudlCommands.changed();
    }
  });
  const sessions = sessionStore.records;
  const S = sessionStore.get, sessionProvider = sessionStore.provider;
  let workspace = '';
  const chats = window.dinahChat.create({
    store: sessionStore, toast, fail, ask, productName, drainAgentEvents, noWorkspaceYet,
    workspace: () => workspace,
    title: session => chatBrowser.title(session), hasKnownTitle: id => chatBrowser.hasKnownTitle(id),
    drawSessions: () => chatBrowser.drawSessions(), refreshSessions: () => chatBrowser.refreshSessions(), listedSessions: () => chatBrowser.listed(),
    startRename: (...args) => chatBrowser.startRename(...args), openRemove: id => chatBrowser.openRemove(id),
    chatPin: id => chatBrowser.chatPin(id), sameWs: path => chatBrowser.sameWs(path),
    normalizedWorkspace: path => chatBrowser.normalizedWorkspace(path), when: time => chatBrowser.when(time),
    workspaceLabel: path => workspaces.workspaceLabel(path),
    drawDefaults: () => settings.drawDefaults(), drawPrefs: () => settings.drawPrefs(),
    scanProviders: () => settings.scanProviders(), sendKeyName: () => settings.sendKeyName(),
    readProviderPreferences: id => settings.readProviderPreferences(id),
    startLog: (id, gen, through) => sequence.startAt(id, gen, through), catchUpLog: id => sequence.catchUp(id),
    needsCount, waitingApproval, drawNoticeCounts, permissionCard, headsUp,
    providerStopped: notifications.providerStopped, permissions: notifications.permissions,
    setTitles: value => chatBrowser.setTitles(value), hiddenCount: () => chatBrowser.hiddenCount()
  });
  const { providerName, chatWorking, backgroundWork, earlierSubagents,
    show, showChat, openSession, startNewChat, restartAgent, frontProvider, signIn, setContext } = chats;
  // A window that names a chat by id, such as a judgement's record of where
  // its decision went, opens it through the same path as the chat list.
  window.dinahOpenChat = id => openSession(id);

  const chatBrowser = window.dinahChatBrowser.create({
    app, fail, toast, productName, sessionProvider, providerName, notices, sessions, S,
    show, showChat, openSession: id => openSession(id),
    workspaceLabel: path => workspaceLabel(path), chatWorking, needsCount, waitingApproval,
    backgroundWork, earlierSubagents, browserSection: (...args) => browserSection(...args),
    active: () => chats.active(), workspace: () => workspace, openPaths: () => workspaces.openPaths(),
    disposeChat: chats.disposeChat,
    showSavedChat: session => {
      show(session.id); showChat();
      const chat = chats.current();
      chat.sessionNote(); chat.stateLine();
    }
  });
  const { title, startRename, openRemove, refreshSessions, drawSessions } = chatBrowser;
  window.dinahChatState.connect({ fail: fail, changed: function () {
    drawSessions(); chats.refreshStates();
  } });

  const sequence = window.dinahChatSequence.create({
    app,
    deliver: deliverEvent,
    loading: id => !!(sessions[id] && sessions[id].loading),
    /* A chat the engine opened again: the page opens it again too, which draws
       its replay in a log cleared first, when a window shows it; otherwise it
       is marked closed and replays when it is next opened. */
    reset: id => {
      const s = sessions[id];
      if (!s) return;
      s.info.open = false;
      if (s.view) chats.reconnectChat(id, false).catch(() => {});
    },
    loss: id => {
      const s = sessions[id];
      if (!window.dinahHost.notices) return;
      window.dinahHost.notices.add({ id: 'lost-activity-' + id, type: 'warning', title: 'Part of "' + (s ? title(s) : id) + '" may be missing.',
        description: 'Some of what this chat did while the page was not receiving events could not be read back, so its text may have a gap.',
        dismissible: true, className: 'lost-activity' });
    }
  });
  sessionStore.onInfo(sequence.info);

  const settings = window.dinahSettings.create({
    ...chats.settings, setWorkspaceRecords: records => workspaces.setRecords(records)
  });
  const { openSettings, setWindowModeControl, drawWindowKindModes,
    setFileLimits, setOptionSubstitutions, scanProviders,
    setThinking, saveThinking, setNotifyNeeded, reportFocus, setSendKey,
    setPersonName, setDocumentViews, settingsStatus } = settings;

  const workspaces = window.dinahWorkspaces.create({
    app, ask, toast, fail, productName, drawSessions, setContext, show, refreshSessions,
    drawGoTo: () => finder.drawGoTo(),
    workspace: () => workspace,
    setWorkspace: path => { workspace = path; },
    setPackageSources: count => packages.setSources(count),
    refreshChatTitles: chats.refreshChatTitles,
    setDraftWorkspace: chats.setDraftWorkspace
  });
  const { workspaceLabel, browserSection, drawWorkspaces, removeWorkspace,
    exportWorkspace, importWorkspace, createWorkspace, openWorkspaceRename } = workspaces;

  const packages = window.dinahPackages.create({
    app, on, toast, fail, productName, workspaceLabel, providerName, frontProvider, restartAgent, notices,
    workspace: () => workspace,
    hasWorkingChat: () => Object.keys(sessions).some(id => sessions[id].working)
  });
  const { installPackages } = packages;

  const finder = window.dinahFind.create({
    fail, nextNotification, showChat,
    chatRows: () => chatBrowser.listed().map(info => ({
      title: title(sessions[info.sessionId]), detail: 'Chat · ' + workspaceLabel(info.cwd),
      open: () => openSession(info.sessionId)
    }))
  });
  const { openGoTo } = finder;

  window.dinahCommands.installHost({
    app, chats, chatBrowser, notices, productName, toast, fail, showChat, showNotifications,
    openGoTo, openWorkspaceRename, removeWorkspace, exportWorkspace, importWorkspace,
    installPackages, startNewChat, createWorkspace, clearNotifications, drawWorkspaces,
    refreshSessions, openSettings, startRename, openRemove, signIn, frontProvider,
    restartAgent, saveThinking, flushNotifications,
    workspaceCommands: workspaces.commandEnabled, chatCommands: chats.commandEnabled,
    thinking: settings.thinkingState
  });
  document.addEventListener('dinah:menu-error', event => fail(event.detail));
  document.addEventListener('click', event => {
    const control = event.target.closest('[data-theme-choice]');
    if (!control || control.disabled || control.getAttribute('aria-disabled') === 'true') return;
    window.pudlSetTheme(control.dataset.themeChoice); themeTicks();
  });

  function themeTicks() {
    var pref = window.pudlThemePreference ? window.pudlThemePreference() : null;
    document.querySelectorAll('[data-theme-choice]').forEach(function (b) {
      var checked = b.getAttribute('data-theme-choice') === pref;
      b.setAttribute('aria-checked', String(checked));
      if (b.getAttribute('role') === 'radio') b.tabIndex = checked ? 0 : -1;
    });
  }
  document.addEventListener('pudl:theme-change', themeTicks);
  document.addEventListener('pudl:theme-change', function (event) { if (window.dinahUIState) window.dinahUIState.set('pudl-theme', event.detail.preference); });
  document.addEventListener('dinah:ui-state-error', function (event) {
    settingsStatus('ui-settings-status', 'UI state could not be saved or restored: ' + event.detail, true);
    fail(event.detail);
  });
  on('dinah:ui-state-flush', function (request) {
    Promise.resolve().then(function () { return Promise.all([flushNotifications(), window.dinahUIState ? window.dinahUIState.flush() : null]); }).then(function () {
      return app().AckUIStateFlush(request.id, true);
    }).catch(function (error) {
      fail(error);
      Promise.resolve(app().AckUIStateFlush(request.id, false)).catch(fail);
    });
  });
  on('dinah:ui-state-flush-timeout', function (request) { fail(request.detail || 'The window stayed open because UI state could not be saved in time.'); });

  function ready() {
    if (!app()) { setTimeout(ready, 50); return; }
    themeTicks();
    // The saved layout and the app's state are read together; neither needs
    // the other.
    var stateRead = Promise.resolve().then(function () { return app().State(); });
    Promise.all([Promise.resolve(window.dinahUIState ? window.dinahUIState.initialize() : null), stateRead]).then(function (both) {
      if (window.dinahUIState && window.dinahUIState.initializationError()) fail(window.dinahUIState.initializationError());
      if (window.dinahWorkbench && window.dinahWorkbench.applySavedTitles) window.dinahWorkbench.applySavedTitles();
      return both[1];
    }).then(function (st) {
      // The page shows the product's name from here on (product.js).
      if (window.dinahProduct) window.dinahProduct.apply(st.product);
      var mode = st.newWindowMode === 'maximized' ? 'maximized' : 'floating';
      setWindowModeControl(mode);
      drawWindowKindModes(st.windowModes, st.windowModesShipped);
      setNotifyNeeded(st.notifyWhenNeeded !== false);
      reportFocus();
      setSendKey(st.promptSendKey || 'enter');
      setPersonName(st.personName || '', st.accountName || '', st.busHandle || '');
      setDocumentViews(st.documentViews);
      window.dinahProviderSettings.apply(st);
      setFileLimits(st.fileLimits || {});
      setOptionSubstitutions(st.optionSubstitutions || {});
      if (window.dinahUIState) {
        window.dinahUIState.setNewWindowMode(mode);
        var theme = window.dinahUIState.get('pudl-theme');
        if (theme && window.pudlSetTheme) window.pudlSetTheme(theme);
        window.dinahUIState.restoreLayout();
      }
      drawWorkspaces(st);
      chatBrowser.setTitles(st.titles);
      restoreNotifications().catch(fail);
      chatBrowser.setHiddenCount(st.hiddenCount);
      chats.restorePreferences(st);
      setThinking(!!st.showThinking);
      chats.setTemplates(st.templates || []);
      $('about-version').textContent = st.version || '';
      chats.restoreView();
      refreshSessions();
      scanProviders();
    }).catch(fail).then(function () { setStarted(); });
  }
  /* Until the layout and the app's state have been read, the status area
     says Dinah is starting, and commands that need the workspace say
     so rather than that there is no workspace. */
  var started = false;
  function setStarted() {
    started = true;
    var s = $('status-starting');
    if (s) s.hidden = true;
    document.body.removeAttribute('aria-busy');
  }
  function noWorkspaceYet() {
    toast(started ? 'Choose a workspace first.' : productName() + ' is still starting; try again in a moment.', 'warn');
  }
  chats.setupChatWindows();
  ready();
})();
