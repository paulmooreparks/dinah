/* The order of a chat's stream events. The engine numbers each chat's
   agent:update, agent:turn, agent:steer and agent:sent events (n) within the
   generation (gen) of the chat's open conversation, and keeps them in a log the
   page can read (ChatEvents). The page applies each chat's events in number
   order, once: a repeat is ignored, a number ahead of the next one waits while
   the missing entries are read, and an entry the log no longer has is reported
   as a loss. A new generation means the chat was closed and opened again, or
   the engine started again, so the page opens the chat again and ignores what
   is still queued from the generation it replaced. After a resync every chat
   asks the log for what followed its last number
   (docs/plans/event-streams.md). */
(function () {
  'use strict';

  function create(adapter) {
    var chats = new Map();

    function state(id) {
      var st = chats.get(id);
      if (!st) { st = { gen: undefined, last: 0, held: [], filling: false, reading: null, again: false, retried: false, dropped: new Set() }; chats.set(id, st); }
      return st;
    }
    function withMeta(id, gen, entry) {
      var data = Object.assign({}, entry.data || {}, { sessionId: id, gen: gen, n: entry.n });
      return data;
    }
    function hold(st, name, data) {
      if (st.held.some(function (h) { return h.data.n === data.n; })) return;
      st.held.push({ name: name, data: data });
      st.held.sort(function (a, b) { return a.data.n - b.data.n; });
    }
    function apply(st, name, data) {
      st.last = data.n;
      adapter.deliver(name, data);
    }

    /* The chat was closed and opened again, or the engine started again.
       What the page drew from the old generation is replaced by the reopened
       chat's replay, and anything still queued from the old one is dropped. */
    function reset(id, st, view) {
      if (st.gen !== undefined) st.dropped.add(st.gen);
      st.gen = undefined; st.last = 0; st.held = []; st.filling = false; st.reading = null; st.again = false; st.retried = false;
      adapter.reset(id);
      // The engine has the chat open under a generation the page has not seen, so opening
      // it again replays nothing: the transcript comes from the start of that log.
      if (view && view.gen && view.head > 0) { st.gen = view.gen; return read(id, st); }
      return Promise.resolve();
    }

    function receive(name, data) {
      if (!data || !data.sessionId || data.gen === undefined || data.n === undefined) { adapter.deliver(name, data); return; }
      var id = data.sessionId, st = state(id);
      if (st.dropped.has(data.gen)) return;
      if (st.gen === undefined) { st.gen = data.gen; st.last = data.n - 1; }
      else if (st.gen !== data.gen) {
        if (adapter.loading(id)) { st.gen = data.gen; st.last = data.n - 1; st.held = []; st.filling = false; st.reading = null; st.again = false; st.retried = false; }
        else { reset(id, st, null); st.gen = data.gen; }
      }
      if (data.n <= st.last) return;
      if (st.filling) { hold(st, name, data); return; }
      if (data.n === st.last + 1) { apply(st, name, data); return; }
      hold(st, name, data);
      read(id, st);
    }

    // Applies what the log returned, then the held events that follow it.
    function take(id, st, view) {
      if (view.truncated) adapter.loss(id);
      (view.events || []).forEach(function (entry) {
        if (entry.n > st.last) apply(st, entry.name, withMeta(id, st.gen, entry));
      });
    }
    function drain(st) {
      st.held = st.held.filter(function (h) { return h.data.n > st.last; });
      while (st.held.length && st.held[0].data.n === st.last + 1) {
        var h = st.held.shift();
        apply(st, h.name, h.data);
      }
    }
    // An entry between the log's answer and the first held event that nobody can
    // give back (the log dropped it) is reported once, and the held events go on.
    function skipGap(id, st) {
      if (!st.held.length) return;
      adapter.loss(id);
      st.last = st.held[0].data.n - 1;
      drain(st);
    }

    /* The one way a chat reads its log, for a gap and for a resync alike. One
       read runs at a time. A request made while one is running (a resync, or a
       gap that opened behind it) is remembered, and the log is read once more
       when the running read ends, since that read answered as of before the
       request. Events held behind a read that still leave a gap get one more
       read; only a gap that survives it is reported as lost. A failed read marks
       the loss and lets the held events through. */
    function read(id, st) {
      if (st.reading) { st.again = true; return st.reading; }
      var gen = st.gen;
      st.filling = true;
      st.reading = Promise.resolve().then(function () { return adapter.app().ChatEvents(id, gen, st.last); }).then(function (view) {
        if (st.gen !== gen) return;
        if (view.reset) return reset(id, st, view);
        take(id, st, view);
        drain(st);
        if (!st.held.length) st.retried = false;
        else if (!st.retried) { st.retried = true; st.again = true; }
        else { st.retried = false; skipGap(id, st); }
      }, function () {
        if (st.gen !== gen) return;
        st.retried = false;
        if (st.held.length) skipGap(id, st); else adapter.loss(id);
      }).then(function () {
        if (st.gen !== gen) return;
        st.reading = null;
        if (st.again) { st.again = false; return read(id, st); }
        st.filling = false;
      });
      return st.reading;
    }

    /* A chat the page has no number for yet starts from the head its Info
       reports. Info arrives with every agent:session event too, so a number
       the page already holds is never moved by it: that would skip updates
       between the two. A chat being opened takes its start from its replay. */
    function info(id, value) {
      if (!value || !value.streamGen) return;
      var st = state(id);
      if (st.gen !== undefined || st.dropped.has(value.streamGen) || adapter.loading(id)) return;
      st.gen = value.streamGen; st.last = value.streamHead || 0;
    }

    /* A chat drawn from its saved copy while it is live in the engine. The copy
       holds every log entry up to a number (SavedConversation: streamGen,
       streamThrough), so the chat takes that as its position, and what arrives
       while the copy is drawn waits. catchUp reads what follows the number,
       which is the turn so far, once the copy is drawn. A chat with no
       generation to start from is left as it is. */
    function startAt(id, gen, through) {
      if (!gen) return false;
      var st = state(id);
      if (st.dropped.has(gen)) return false;
      st.gen = gen; st.last = through || 0; st.held = []; st.again = false; st.retried = false;
      if (!st.reading) st.filling = true;
      return true;
    }
    function catchUp(id) {
      var st = chats.get(id);
      if (!st || st.gen === undefined) return Promise.resolve();
      return read(id, st);
    }

    /* After a resync each chat asks for what followed its last number. */
    function resync() {
      var work = [];
      chats.forEach(function (st, id) {
        if (st.gen !== undefined) work.push(read(id, st));
      });
      return Promise.all(work);
    }
    if (window.dinahHost && window.dinahHost.onResync) window.dinahHost.onResync(resync);

    return { receive: receive, info: info, startAt: startAt, catchUp: catchUp, forget: function (id) { chats.delete(id); }, state: function (id) { var st = chats.get(id); return st ? { gen: st.gen, last: st.last } : null; } };
  }

  window.dinahChatSequence = { create: create };
})();
