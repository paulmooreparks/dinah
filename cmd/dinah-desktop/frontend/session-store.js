/* The page's session records retain their identity across snapshots and
   live updates. The shell supplies log creation and metadata notifications. */
(function () {
  'use strict';

  function provider(session) {
    // The provider is the one the engine reports for the chat; a chat it has not reported on has none yet.
    return session && session.info.provider || '';
  }

  function create(adapter) {
    const records = {};
    function get(id) {
      let session = records[id];
      if (!session) {
        session = records[id] = {
          id: id, info: { sessionId: id, title: '', open: false },
          log: adapter.createLog(), rows: {}, current: null, options: [], working: false, queued: []
        };
      }
      return session;
    }
    // Others that need to know of every Info applied, such as the chat sequencer.
    var infoListeners = [];
    function onInfo(fn) { infoListeners.push(fn); }
    function applyInfo(session, info) {
      // A snapshot older than the one already applied is ignored (state-order.js).
      if (!window.dinahStateOrder.accept('agent:session', session.id, info)) return;
      session.info = Object.assign({}, session.info, info);
      // A versioned snapshot is the whole of the chat's information, so what it leaves out is gone: a hold that
      // has ended, a choice that was forgotten.
      if (info.stateVer !== undefined) { if (!info.hold) session.info.hold = null; if (!info.choice) session.info.choice = ''; if (!info.switch) delete session.info.switch; if (!info.lead) session.info.lead = null; }
      if (info.stateVer !== undefined) { if (!info.chatState) delete session.info.chatState; if (!info.team) delete session.info.team; }
      infoListeners.forEach(fn => fn(session.id, info));
      document.dispatchEvent(new CustomEvent('dinah:session-info', { detail: { id: session.id } }));
      if (typeof info.working === 'boolean') session.working = info.working;
      if (session.working === false) session.stopping = false;
      if (Array.isArray(info.queued)) session.queued = info.queued;
      if (Array.isArray(info.background)) session.background = info.background;
      if (Array.isArray(info.subagents)) {
        const earlier = (session.subagents || []).filter(sub => !sub.thisRun).length;
        session.subagents = info.subagents;
        adapter.subagentsChanged(session.subagents.filter(sub => !sub.thisRun).length !== earlier);
      }
    }
    return { records, get, applyInfo, onInfo, provider };
  }

  window.dinahSessionStore = { create };
}());
