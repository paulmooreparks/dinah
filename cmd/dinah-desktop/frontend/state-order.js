/* The order of state snapshots. The engine puts a generation and a version
   (stateGen, stateVer) on each event or call result that says the whole of some
   state, taken from the owner's counter in the same critical section that read
   the value: a session's information, queue and options, a provider's state and
   account, the window's settings and the update state. Events are emitted after
   the owner's lock is released, so two of them can arrive in the opposite order
   to the one the changes happened in. The page applies a snapshot only if its
   version is higher than the last one it applied for the same kind and owner,
   and a different generation always wins, since it is a new owner: a chat opened
   again, or an engine that started again (docs/plans/state-versions.md). A
   snapshot with no version is applied as before, which is what a closed chat's
   saved information is. A reread after a resync goes through the same rule as an
   event does. */
(function () {
  'use strict';

  var records = new Map();

  function key(name, owner) { return name + '\u0000' + (owner || ''); }
  function versioned(data) { return !!data && typeof data.stateVer === 'number' && data.stateGen !== undefined; }

  /* True when the snapshot is not newer than the one already applied (or, for
     a snapshot that may be applied again, older than it). Nothing is recorded;
     a caller that only looks, such as a list being merged, uses it. */
  function stale(name, owner, data, again) {
    if (!versioned(data)) return false;
    var held = records.get(key(name, owner));
    return !!held && held.gen === data.stateGen && (again ? data.stateVer < held.ver : data.stateVer <= held.ver);
  }

  /* True when the snapshot should be applied, and then it is the one held. An
     event is applied once. A read passes again, since several windows each read
     the same state and apply it to their own copy; the same snapshot is then
     applied by each, and an older one by none. */
  function accept(name, owner, data, again) {
    if (!versioned(data)) return true;
    if (stale(name, owner, data, again)) return false;
    records.set(key(name, owner), { gen: data.stateGen, ver: data.stateVer });
    return true;
  }

  /* A session that is gone from the page leaves nothing behind. */
  function forget(owner) {
    var suffix = '\u0000' + (owner || '');
    Array.from(records.keys()).forEach(function (k) { if (k.slice(-suffix.length) === suffix) records.delete(k); });
  }

  window.dinahStateOrder = { accept: accept, stale: stale, forget: forget, held: function (name, owner) { return records.get(key(name, owner)) || null; } };
})();
