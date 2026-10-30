/* The engine owns execution, lifecycle and aggregate priority. Every page
   surface uses this one presentation of that state (plans/chat-states.md). */
(function () {
  'use strict';
  var mapping = {
    working: { word: 'Working', colour: 'accent', glyph: 'chat-working' },
    waiting: { word: 'Waiting', colour: 'warn', glyph: 'flag' },
    stuck: { word: 'Stuck', colour: 'danger', glyph: 'warning' },
    idle: { word: 'Idle', colour: '', glyph: 'ring' }
  };
  var chats = new Map(), buses = new Map(), groups = new Map();
  function read(value) {
    value = value || {};
    return chats.get(value.sessionId || value.chat || value.id) || buses.get(value.bus || value.id) || value.chatState || value;
  }
  function counts(value) {
    return value && (groups.get(value.name) || value.counts || value.team || value);
  }
  function apply(snapshot) {
    if (!snapshot || !window.dinahStateOrder.accept('agent:chatstate', '', snapshot)) return false;
    chats.clear(); buses.clear(); groups.clear();
    (snapshot.chats || []).forEach(function (entry) {
      if (entry.sessionId) chats.set(entry.sessionId, entry);
      if (entry.bus) buses.set(entry.bus, entry);
    });
    (snapshot.groups || []).forEach(function (entry) { groups.set(entry.name, entry.counts); });
    document.dispatchEvent(new CustomEvent('dinah:execution-state', { detail: snapshot }));
    return true;
  }
  function connect(options) {
    function changed(snapshot) { if (apply(snapshot) && options.changed) options.changed(snapshot); }
    function refresh() {
      var app = window.dinahHelpers.app();
      return app && app.ChatStates ? Promise.resolve(app.ChatStates()).then(changed).catch(options.fail) : Promise.resolve();
    }
    window.dinahHost.on('agent:chatstate', changed);
    window.dinahHost.onResync(refresh);
    return refresh();
  }
  function describe(value, options) {
    value = value || {}; options = options || {};
    var aggregate = Object.prototype.hasOwnProperty.call(value, 'top') || typeof value.unknown === 'number' || typeof value.closed === 'number' || !!(value.counts && !value.life && !value.chatState && !value.exec);
    value = aggregate ? counts(value) : read(value);
    var state = aggregate ? value.top : value.exec && value.exec.state;
    var life = value.life;
    if (!aggregate && value.elsewhere) return { text: value.elsewhere, kind: '', cls: 'chat-state chat-lifecycle', label: value.elsewhere, glyph: '' };
    if (!aggregate && (life === 'closed' || life === 'retired')) return { text: life === 'closed' ? 'Closed' : 'Retired', kind: '', cls: 'chat-state chat-lifecycle', label: life === 'closed' ? 'The session has ended' : 'The team member is retired', glyph: '' };
    var spec = mapping[state], why = !aggregate && value.exec && value.exec.why || '', words = spec ? spec.word : '';
    if (aggregate) {
      var totals = value.counts || value;
      var parts = ['stuck', 'waiting', 'working', 'idle', 'unknown', 'closed'].filter(function (name) { return totals[name] > 0; }).map(function (name) { return totals[name] + ' ' + (name === 'unknown' ? 'running elsewhere' : name); });
      words = parts.length ? parts.join(', ') : words;
      if (!spec && words) return { text: words, kind: '', cls: 'chat-state chat-lifecycle', label: words, glyph: '' };
      if (!totals.stuck && !totals.waiting && !totals.working && !totals.idle && totals.closed) words = spec.word + ' (' + words + ')';
    } else if (why && options.reason !== false && (state === 'waiting' || state === 'stuck')) words += ' (' + why + ')';
    if (!spec) return { text: 'State unavailable', kind: '', cls: 'chat-state chat-state-unknown', label: 'The engine has not supplied an execution state', glyph: '' };
    return { state: state, text: words, kind: spec.colour, glyph: spec.glyph, cls: 'chat-state chat-state-' + state, label: aggregate ? spec.word + ': ' + words : spec.word + (why ? ': ' + why : '') };
  }
  function badge(value, options) {
    var spec = describe(value, options), el = window.dinahBrowserList.badge(spec);
    el.style.setProperty('--chat-state-colour', 'var(--' + (spec.kind || 'text-muted') + ')');
    if (options && options.className) el.classList.add(options.className);
    return el;
  }
  window.dinahChatState = { describe: describe, badge: badge, read: read, counts: counts, apply: apply, connect: connect };
})();
