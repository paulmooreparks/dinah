/* Session opening and reconnects share the page's existing records. The
   shell supplies window operations and the chat's rendering callbacks. */
(function () {
  'use strict';

  function create(adapter) {
    const { store, app, workspace, active, chatKey, isWindowOpen, closeWindow,
      onWindowsChange, offWindowsChange, openChatIds, chatWindowsFor,
      fail, toast, providerName, title, show, showChat, drawSaved,
      replayProgress, sessionNote, stateLine, enqueue, drainAgentEvents,
      rememberCatalog, drawAgentState, openingNote, stampReplay, drawAll,
      presentHistory, noWorkspaceYet, startLog, catchUpLog } = adapter;
    const sessions = store.records, S = store.get;
    const applySessionInfo = store.applyInfo, sessionProvider = store.provider;

    /* Opening a session from the list reopens it in Devin, which replays the
       whole conversation before it answers. Meanwhile the log is hidden and a
       line counts what has arrived; when Devin answers, the history is drawn
       once and the view is left at its end. If Devin cannot open the session,
       the reason and what to do stay in sight above the prompt, and whatever
       history arrived stays readable. */
    function openSession(id, reveal) {
      if (app().ResolveSessionContinuation) return app().ResolveSessionContinuation(id).then(function (head) {
        if (head && head !== id && reveal === false) {
          var old = S(id), oldKey = chatKey(id, old.info.cwd || workspace());
          return new Promise(function (resolve) {
            function redirect() {
              if (!isWindowOpen(oldKey)) return;
              offWindowsChange(redirect);
              openResolvedSession(head).then(function (s) { closeWindow(oldKey); resolve(s); });
            }
            onWindowsChange(redirect);
            redirect();
          });
        }
        return openResolvedSession(head || id, reveal);
      }).catch(function (error) { fail(error); return null; });
      return openResolvedSession(id, reveal);
    }
    /* A chat window restored with the layout shows Dinah's saved copy
       of the chat at once, and then reconnects to its agent behind it, one
       chat at a time, so that every chat in the layout is live again, and
       answers the bus, without the person touching it (23 October 2026).
       Each reconnect makes the agent replay the whole conversation, so doing
       them all at once made starting slow; the saved copy stays in sight
       while the agent replays, and the replay takes its place when it ends.
       Sending a message, the Skills or model menu, or Reconnect Now reconnect
       a chat at once. A chat with no complete saved copy, or one continued in
       another chat, reopens as before. */
    function restoreChat(s) {
      var id = s.id;
      return Promise.resolve(app().ResolveSessionContinuation ? app().ResolveSessionContinuation(id) : id).then(function (head) {
        if (sessions[id] !== s) return s;
        if ((head && head !== id) || !app().SavedConversation) return openSession(id, false);
        return Promise.resolve(app().SavedConversation(id)).then(function (saved) {
          /* A chat that is live in the engine has no replay to wait for: the
             saved copy says which log entry it holds, and the log supplies the
             turn in progress after it. A live chat without that number stays
             as it is. */
          var live = s.info.open && !!saved && !!saved.streamGen;
          if (sessions[id] !== s || s.loading || (s.info.open && !live)) return s;
          if (!saved || !saved.available) return s.info.open ? s : openSession(id, false);
          s.historyWarning = saved.warning || ''; s.historyComplete = saved.complete !== false; s.historyRecoverable = saved.recoverable !== undefined ? !!saved.recoverable : s.historyComplete;
          s.savedCursor = saved.streamGen ? { gen: saved.streamGen, through: saved.streamThrough || 0 } : null;
          if (live) startLog(id, saved.streamGen, saved.streamThrough);
          return drawSaved(s, saved.messages || []).then(function () {
            if (live) return catchUpLog(id).then(function () { return s; });
            connectBehind(s);
            return s;
          });
        });
      }).catch(function () { return sessions[id] === s ? openSession(id, false) : s; });
    }
    /* The chats restored with the layout reconnect one after another. One
       whose agent takes long to replay, or never replies, delays the next
       by BEHIND_PATIENCE at most, so a single chat cannot leave every other
       window waiting at the prompt to reconnect (28 October 2026); it goes
       on reconnecting while the next one starts. */
    var behindQueue = Promise.resolve(), BEHIND_PATIENCE = 60000;
    function connectBehind(s) {
      behindQueue = behindQueue.then(function () { return new Promise(function (r) { setTimeout(r, 250); }); }).then(function () {
        if (sessions[s.id] !== s || !s.saved || s.info.open || s.loading) return;
        return Promise.race([reconnectChat(s.id, false), new Promise(function (r) { setTimeout(r, BEHIND_PATIENCE); })]);
      }).catch(function () {});
    }
    /* reconnectChat is how a chat waiting at the prompt to reconnect, or a
       closed chat, is connected to its agent. Reconnect Now calls it with
       front, which brings the chat forward as choosing it does; the
       restored chats reconnecting by themselves, and a message on the bus
       that wakes a chat (chat:connect), call it without, which leaves the
       window where it is and keeps the saved copy in sight while the agent
       replays. A failure is reported in the chat's window above the prompt
       either way (sessionNote). */
    function reconnectChat(id, front) {
      if (front) return openSession(id);
      var s = S(id);
      // What the chat already shows stays in sight while the agent replays, and the replay takes its place.
      return openResolvedSession(id, false, true, !!(s.saved || s.log.firstChild));
    }
    function connect(m) {
      var s = S(m.sessionId);
      if (!s.info.cwd && m.cwd) s.info.cwd = m.cwd;
      if (s.info.open || s.loading || s.savedBuild) return;
      reconnectChat(s.id, false).then(function (r) {
        if (r || sessions[s.id] !== s || chatWindowsFor(s.id).length) return;
        var agent = providerName(sessionProvider(s));
        toast(agent + ' reported: ' + String(s.failed || 'the agent did not open it').replace(/^Error:\s*/, '') + ' The messages waiting for it keep waiting.', 'danger', { title: agent + ' could not open "' + title(s) + '".' });
      }).catch(function () {});
    }
    var reportedWindows = null;
    function reportChatWindows() {
      if (!app() || !app().ChatWindows) return;
      var ids = openChatIds();
      if (!ids) return;
      ids.sort();
      var now = JSON.stringify(ids);
      if (now === reportedWindows) return;
      reportedWindows = now;
      Promise.resolve(app().ChatWindows(ids)).catch(function () { reportedWindows = null; });
    }
    // The chat in front reconnects when its agent's live values are needed.
    function reconnectSaved() {
      var id = active(), s = id && sessions[id];
      if (s && s.saved && !s.info.open && !s.loading) openSession(id).catch(function () {});
    }
    /* behind reconnects the chat without making it the current chat, for a
       chat another chat is about to ask. quiet keeps a saved copy in sight
       while the agent replays into a log of its own, which takes the saved
       copy's place when the replay ends; if the reconnect fails, the saved
       copy stays. */
    function openResolvedSession(id, reveal, behind, quiet, drawnFirst) {
      var s = S(id);
      if (!behind) show(id);
      if (reveal !== false && !behind) showChat();
      if (s.savedBuild) return s.savedBuild.then(function () { return sessions[id] === s ? openResolvedSession(id, reveal, behind, quiet) : s; });
      if (s.loading) return s.loadingPromise || Promise.resolve(s);
      if (s.info.open) return Promise.resolve(s);
      /* A chat the page has not drawn yet shows its saved copy at once, and the agent's replay (or its
         resume) goes on behind it: the replay is drawn aside and takes the copy's place when it ends. */
      if (!drawnFirst && (s.view || !behind) && !s.saved && !s.log.firstChild && app().SavedConversation) {
        return Promise.resolve(app().SavedConversation(id)).catch(function () { return null; }).then(function (saved) {
          if (sessions[id] !== s) return s;
          if (s.info.open || s.loading || s.savedBuild || !saved || !saved.available) return openResolvedSession(id, reveal, behind, quiet, true);
          s.historyWarning = saved.warning || ''; s.historyComplete = saved.complete !== false; s.historyRecoverable = saved.recoverable !== undefined ? !!saved.recoverable : s.historyComplete;
          s.savedCursor = saved.streamGen ? { gen: saved.streamGen, through: saved.streamThrough || 0 } : null;
          return drawSaved(s, saved.messages || []).then(function () { return openResolvedSession(id, reveal, behind, true, true); });
        });
      }
      var fullHistory = !!s.loadFull;
      var kept = quiet && (s.saved || s.log.firstChild) ? { log: s.log, rows: s.rows, trimmed: s.trimmed } : null;
      if (kept) s.log = kept.log.cloneNode(false);
      s.keptLog = kept && kept.log;
      s.quiet = !!kept;
      s.saved = false;
      s.log.innerHTML = ''; s.rows = {}; s.current = null; s.trimmed = 0; s.arriving = {}; s.arrivingMore = 0; s.arrivingMoreLi = null;
      s.settling = true;
      s.loading = true; s.replayed = 0; s.failed = ''; s.unoffered = null;
      if (s.view) s.view.drawLoadingChip(); // the window's title bar says it is loading
      s.info.resumed = false; // omitted from an opening that replayed, so a stale true would stay
      s.log.hidden = true;
      if (!kept) {
        replayProgress(s);
        sessionNote();
        stateLine();
      }
      return s.loadingPromise = Promise.resolve(app().SessionContinuation ? app().SessionContinuation(id) : null).then(function (continuation) {
        var history = continuation && continuation.history || [], index = 0;
        if (!history.length) return;
        return new Promise(function (resolve, reject) {
          enqueue({}, null, function (deadline) {
            try {
              if (sessions[id] !== s || !s.view) { resolve(); return false; }
              var count = 0;
              while (index < history.length && count++ < 16 && (count === 1 || performance.now() < deadline)) {
                s.view.prependContinuation([history[index++]]);
              }
              if (index < history.length) return true;
              resolve();
            } catch (error) { reject(error); }
            return false;
          }, null, 'continued history');
        });
      }).then(function () {
        if (sessions[id] !== s) return s.info;
        // Open With… reopens the chat with the value the person chose in
        // place of its saved one (option_substitutions.go).
        var openWith = s.openWith; s.openWith = null;
        // The engine holds a message sent to the chat from here until its history is loaded (s.holding).
        s.holding = true;
        if (openWith) return app().OpenSessionWith(s.info.cwd || workspace(), id, openWith.option, openWith.value, openWith.always);
        // Load Full History: the agent replays the whole conversation, as a chat closed and opened again.
        if (s.loadFull) { s.loadFull = false; return app().LoadFullHistory(s.info.cwd || workspace(), id); }
        return app().OpenSessionAt(s.info.cwd || workspace(), id);
      }).then(async function (info) {
        s.holding = false;
        await drainAgentEvents();
        if (sessions[id] !== s) return null;
        // The engine opens the newest continuation of a chat that was continued and returns that chat's
        // information, with another id than the one asked for. Its window takes that id, which the layout
        // then saves, so that the next start asks for it directly.
        if (info && info.sessionId && info.sessionId !== id) return handOver(s, info);
        s.openingNotes = (info && info.notes) || [];
        applySessionInfo(s, info);
        rememberCatalog(s);
        if (s.view) s.view.loadSkills();
        drawAgentState();
        // Ordinary window openings use the saved copy's checkpoint for a new
        // or existing session. Only Load Full History deliberately draws a replay.
        s.checkpointDraw = !!(info && !fullHistory && (info.streamGen || info.resumed));
        if (s.checkpointDraw) await drawSnapshot(s, info, kept);
        return s;
      }).catch(async function (err) {
        s.holding = false;
        await drainAgentEvents();
        if (sessions[id] !== s) return null;
        s.failed = String(err);
        // A refusal for a saved value the agent no longer offers lists the
        // values it offers instead, for Open With… (sessionNote).
        var detail = err && err.detail;
        s.unoffered = detail && detail.kind === 'unoffered-option' && (detail.offered || []).length ? detail : null;
        if (!kept && app().SavedConversation) {
          try {
            var saved = await app().SavedConversation(id);
            if (sessions[id] === s && saved && saved.available) {
              s.loading = false;
              s.historyWarning = saved.warning || ''; s.historyComplete = saved.complete !== false; s.historyRecoverable = saved.recoverable !== undefined ? !!saved.recoverable : s.historyComplete;
              s.log.replaceChildren(); s.rows = {}; s.current = null;
              await drawSaved(s, saved.messages || []);
            }
          } catch (_) { /* Preserve the provider error when the local copy is unavailable. */ }
        }
        return null;
      }).then(function (r) {
        if (sessions[id] !== s) return null;
        s.loading = false; s.quiet = false;
        if (kept && !r) {
          // The reconnect failed: the saved copy stays, as it was.
          s.log = kept.log; s.keptLog = null; s.rows = kept.rows; s.trimmed = kept.trimmed; s.current = null; s.saved = true; s.settling = false;
          sessionNote();
          if (s.view) { s.view.stateLine(); s.view.sessionNote(); }
          return r;
        }
        // A line the opening returned, such as a stored substitution it made,
        // follows the chat's history.
        if (r) (s.openingNotes || []).forEach(function (text) { openingNote(s, text); });
        s.openingNotes = null;
        var checkpointDraw = r && s.checkpointDraw; s.checkpointDraw = false;
        if (!checkpointDraw) {
          stampReplay(s);
          drawAll(s, function () {
            if (sessions[id] !== s) return;
            presentHistory(s, kept);
          });
        } else catchUpLog(id);
        sessionNote();
        // The chat's own window says it is open now, in front or not.
        if (s.view) { s.view.drawLoadingChip(); s.view.stateLine(); s.view.sessionNote(); s.view.recoverSteering(); }
        return r;
      });
    }

    function handOver(s, info) {
      var head = S(info.sessionId), oldKey = chatKey(s.id, s.info.cwd || workspace());
      if (!head.info.cwd && info.cwd) head.info.cwd = info.cwd;
      applySessionInfo(head, info);
      rememberCatalog(head);
      drawAgentState();
      if (isWindowOpen(oldKey)) {
        window.dinahWorkbench.open({ kind: 'chat-session', workspace: head.info.cwd || workspace(), path: head.id });
        closeWindow(oldKey);
      }
      return s;
    }
    /* The journal follows the checkpoint belonging to the saved copy in sight.
       A running session can still carry resumed from an earlier opening, so
       that flag never decides which journal entries the page has already drawn. */
    function drawSnapshot(s, info, kept) {
      s.keptLog = null; s.saved = false;
      if (kept) {
        startLog(s.id, info.streamGen, s.savedCursor && s.savedCursor.gen === info.streamGen ? s.savedCursor.through : 0);
        s.log = kept.log; s.rows = kept.rows; s.trimmed = kept.trimmed; s.current = null; s.historyAvailable = true;
        // The copy in sight is presented as a replay that has taken its place is: the reader's place, or the end
        // when they were following it, is put back and the view goes on from there.
        presentHistory(s, kept);
        return Promise.resolve();
      }
      return Promise.resolve(app().SavedConversation ? app().SavedConversation(s.id) : null).then(function (saved) {
        if (sessions[s.id] !== s) return;
        s.savedCursor = saved && saved.streamGen ? { gen: saved.streamGen, through: saved.streamThrough || 0 } : null;
        startLog(s.id, info.streamGen, s.savedCursor && s.savedCursor.gen === info.streamGen ? s.savedCursor.through : 0);
        s.historyWarning = saved && saved.warning || ''; s.historyComplete = !saved || saved.complete !== false; s.historyRecoverable = saved && saved.recoverable !== undefined ? !!saved.recoverable : s.historyComplete;
        return drawSaved(s, saved && saved.messages || []).then(function () { s.saved = false; });
      });
    }
    /* Load Full History asks the agent for the whole conversation, which makes the chat a closed and
       reopened one: the page clears what it drew and draws the replay as it arrives. */
    function loadFullHistory(id) {
      var s = S(id);
      if (!s.info.open || s.loading || s.working) return Promise.resolve(s);
      // What is drawn stays in sight while the agent replays, and the replay takes its place when it ends.
      s.loadFull = true; s.info.open = false; s.info.resumed = false; s.saved = true;
      return openResolvedSession(id, false, false, true, true);
    }

    function newSession() {
      if (!workspace()) { noWorkspaceYet(); return Promise.reject('no workspace'); }
      return app().NewSessionAt(workspace()).then(function (info) {
        var s = S(info.sessionId);
        applySessionInfo(s, info);
        show(s.id);
        return showChat().then(function () { return s; });
      }).catch(function (err) { fail(err); throw err; });
    }


    return { openSession, restoreChat, reconnectChat, connect, reportChatWindows,
      reconnectSaved, openResolvedSession, newSession, loadFullHistory };
  }

  window.dinahSessionLifecycle = { create };
}());
