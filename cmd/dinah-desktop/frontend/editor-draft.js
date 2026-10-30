/* Keeping the draft of an Editor window written as the person types (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var api = C.api, productName = C.productName, host = C.host, message = C.message;

  window.dinahEditorParts.draft = function (ed) {
    /* ---- Keeping the draft --------------------------------------------- */

    /* Typing is written to the draft after a short pause. A write names the
       revision it started from, so a change made elsewhere in between is
       never overwritten without the person's choice. */
    function schedule() {
      ed.pending = true;
      ed.drawHead();
      clearTimeout(ed.writeTimer);
      ed.writeTimer = setTimeout(flush, 600);
    }
    function flush() {
      clearTimeout(ed.writeTimer);
      // What was typed on the page is put into the text first.
      if (ed.pageView && ed.pageView.pending()) return ed.pageView.sync().then(function () { return ed.pageView.pending() ? undefined : flush(); });
      if (ed.writing) return ed.writing.then(flush);
      if (!ed.pending || !ed.draft || !ed.view || ed.outside || ed.disposed) return Promise.resolve();
      var text = ed.view.state.doc.toString();
      ed.pending = false;
      if (text === ed.draft.text) { ed.drawHead(); return Promise.resolve(); }
      ed.writing = api().UpdateDraft(ed.id, ed.draft.revision, text).then(function (v) {
        ed.draft = v;
        ed.schedulePreview();
        ed.scheduleAutoSave();
      }).catch(function (err) {
        ed.pending = true;
        return api().GetDraft(ed.id).then(function (latest) {
          if (latest.revision !== ed.draft.revision) showOutside(latest);
          else ed.say(message(err), true);
        }, function () { ed.say(message(err), true); });
      }).finally(function () {
        ed.writing = null;
        ed.drawHead();
        if (ed.pending && !ed.outside && !ed.disposed) { clearTimeout(ed.writeTimer); ed.writeTimer = setTimeout(flush, 600); }
      });
      return ed.writing;
    }

    /* Replaces the editor's text with as small a change as covers the
       difference, so the cursor and the scroll position stay where they were
       when the change is elsewhere, and keeps it out of the undo history. */
    function replaceText(text) {
      var old = ed.view.state.doc.toString();
      if (old === text) return;
      var start = 0, oldEnd = old.length, newEnd = text.length;
      while (start < oldEnd && start < newEnd && old.charCodeAt(start) === text.charCodeAt(start)) start++;
      while (oldEnd > start && newEnd > start && old.charCodeAt(oldEnd - 1) === text.charCodeAt(newEnd - 1)) { oldEnd--; newEnd--; }
      ed.applying = true;
      try {
        ed.view.dispatch({ changes: { from: start, to: oldEnd, insert: text.slice(start, newEnd) }, annotations: ed.CM.Transaction.addToHistory.of(false) });
      } finally { ed.applying = false; }
    }

    /* Another part of Dinah (an agent) changed the draft while this
       window held changes it had not yet written. The person chooses which
       text the draft keeps. */
    function showOutside(latest) {
      ed.outside = latest;
      ed.$('notice-body').textContent = 'This draft was changed elsewhere in ' + productName() + ', by an agent, while you had changes it had not yet kept. Keep Mine replaces that change with your text; Take Theirs replaces your latest changes with it.';
      ed.$('notice').hidden = false;
      ed.drawHead();
    }
    function settleOutside(keepMine) {
      if (!ed.outside) return;
      var latest = ed.outside;
      ed.outside = null;
      ed.$('notice').hidden = true;
      if (keepMine) {
        ed.draft = Object.assign({}, ed.draft, { revision: latest.revision, text: latest.text });
        ed.pending = true;
        flush();
      } else {
        ed.draft = latest;
        ed.pending = false;
        replaceText(latest.text);
        ed.schedulePreview(0);
      }
      ed.drawHead();
    }
    ed.$('mine').addEventListener('click', function () { settleOutside(true); });
    ed.$('theirs').addEventListener('click', function () { settleOutside(false); });

    function draftChanged(e) {
      if (!e || e.id !== ed.id || ed.disposed || !ed.draft) return;
      if (ed.writing) { ed.writing.then(function () { draftChanged(e); }); return; }
      if (e.kind === 'discarded') {
        if (!ed.discarding) host().toast('The draft of "' + ed.draft.title + '" was discarded elsewhere, so its window has closed.', 'warn');
        ed.allowClose = true;
        window.pudlWindows.close(ed.key);
        return;
      }
      /* A save, a new suggestion or a rename leaves the text, and so the
         revision, as it was, so those are always read again. */
      if (e.kind !== 'saved' && e.kind !== 'suggested' && e.kind !== 'renamed' && e.revision === ed.draft.revision) return;
      api().GetDraft(ed.id).then(function (latest) {
        if (ed.disposed) return;
        var local = ed.view.state.doc.toString();
        /* The event may come from this window's own write, and may arrive
           before the write's reply, so the text decides, not the order. */
        if (latest.revision === ed.draft.revision || latest.text === local) {
          /* Suggestions are marked in the text and on the page, which
             Preview shows neither of, so their arrival there switches the
             window to Split; Visual marks them where it is. */
          var more = (latest.suggestions || []).length > (ed.draft.suggestions || []).length;
          ed.draft = latest;
          ed.drawHead();
          ed.scheduleAutoSave();
          if (more && ed.mode === 'preview') { ed.setMode('split'); ed.say('Suggestions arrived, so the text is shown beside the page.'); }
          else if (more && ed.pageWanted()) ed.say('Suggestions arrived; each is marked on the page with Accept and Reject.');
          return;
        }
        if (ed.writing) { ed.writing.then(function () { draftChanged(e); }); return; }
        if (!ed.pending && local === ed.draft.text) {
          ed.draft = latest;
          replaceText(latest.text);
          ed.schedulePreview();
          ed.drawHead();
          ed.scheduleAutoSave();
          ed.say('The draft was changed elsewhere, and shows that change.');
          return;
        }
        showOutside(latest);
      }).catch(function () {});
    }
    var unsubscribe = host().on('draft:changed', draftChanged);
    // After a resync the draft is compared with the engine's at once, as the watcher does every few seconds.
    var unresync = host().onResync(function () { return ed.draft && !ed.disposed ? checkStamp() : null; });

    /* Another program, such as the command line, changes the drafts
       without this app's events, so the window compares the draft it
       shows with the stored one every few seconds while it can be seen: at
       each check while it is the window in front, at every fourth while it
       is behind another, and not at all while it is minimised. The check
       asks for the draft's stamp, a few bytes, and reads the draft only when
       the stamp has changed. */
    function fingerprint(v) {
      return v.revision + '|' + ((v.baseline && v.baseline.revision) || '') + '|' + (v.suggestions || []).map(function (s) { return s.id + (s.stale ? '!' : ''); }).join(',');
    }
    var lastStamp = '';
    function checkStamp() {
      if (!api().DraftStamp) return api().GetDraft(ed.id).then(function (latest) {
        if (!ed.disposed && ed.draft && fingerprint(latest) !== fingerprint(ed.draft)) draftChanged({ id: ed.id, kind: 'suggested', revision: latest.revision });
      });
      return Promise.resolve(api().DraftStamp(ed.id)).then(function (stamp) {
        // The first check compares the whole draft, as nothing is known to
        // compare the stamp with.
        var changed = !lastStamp || stamp !== lastStamp;
        lastStamp = stamp;
        if (changed && !ed.disposed && ed.draft) return api().GetDraft(ed.id).then(function (latest) {
          if (!ed.disposed && ed.draft && fingerprint(latest) !== fingerprint(ed.draft)) draftChanged({ id: ed.id, kind: 'suggested', revision: latest.revision });
        });
      });
    }
    function windowShown() {
      var st = !ed.opts.embedded && window.pudlWindows && window.pudlWindows.state();
      if (!st) return 'front';
      if (st.min && st.min[ed.key]) return 'hidden';
      return st.top === ed.key ? 'front' : 'behind';
    }
    var watchTimer = setInterval(function () {
      if (ed.disposed || !ed.draft || ed.writing || document.hidden) return;
      var shown = windowShown();
      if (shown === 'hidden') return;
      if (shown === 'behind' && (++behindTicks % 4) !== 0) return;
      ed.checkMoved(false);
      // A slow check is not overlapped by the next, which would read the
      // draft a second time for the same change.
      if (checking) return;
      checking = true;
      Promise.resolve().then(checkStamp).catch(function () {}).then(function () { checking = false; });
    }, 4000), behindTicks = 0, checking = false;

    return {
      schedule: schedule, flush: flush, replaceText: replaceText,
      teardown: function () { unsubscribe(); unresync(); clearInterval(watchTimer); }
    };
  };
})();
