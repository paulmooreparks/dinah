/* Saving, automatic saving and the resource that moved on, for an Editor window (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var api = C.api, productName = C.productName, host = C.host, message = C.message, originRef = C.originRef;

  window.dinahEditorParts.saving = function (ed) {
    /* ---- Saving -------------------------------------------------------- */

    function notes(list) {
      (list || []).forEach(function (note) { host().toast(note, 'warn'); });
    }
    function save() {
      if (!ed.draft || ed.busy || ed.outside) return;
      if (ed.isNew()) { saveAs(); return; }
      if (!ed.draft.writeBack) return;
      // The window is busy from the press until the write is done, so a
      // second press is ignored.
      ed.busy = true;
      ed.drawHead();
      ed.say('Saving…');
      ed.flush().then(function () {
        if (ed.outside) throw new Error('Settle the change made elsewhere first.');
        return api().SaveDraft(ed.id, ed.draft.revision);
      }).then(function (saved) {
        took(saved);
        autoStopped = '';
        ed.$('moved').hidden = true;
        ed.say('Saved.');
        host().toast('Saved "' + ed.draft.title + '".', 'positive');
        notes(saved.notes);
      }).catch(refused)
        .finally(function () { ed.busy = false; ed.drawHead(); });
    }

    /* A saved draft becomes the window's; a save that changed the text
       makes the window show it, unless the person has typed since. */
    function took(saved) {
      var before = ed.view ? ed.view.state.doc.toString() : null;
      ed.draft = saved;
      if (ed.view && !ed.pending && saved.text != null && saved.text !== before) { ed.replaceText(saved.text); ed.schedulePreview(); }
    }

    /* A document kept in a folder saves itself a moment after each pause in
       typing; Save still saves at once. When the file was changed
       by something else, or a save fails, automatic saving of this document
       stops, says why, and starts again after a Save that succeeds. */
    var autoTimer = 0, autoStopped = '';
    function autoSaves() { return !!ed.draft && !ed.isNew() && ed.draft.writeBack === 'save' && !autoStopped && !ed.outside && !ed.disposed; }
    function scheduleAutoSave() {
      clearTimeout(autoTimer);
      if (autoSaves() && ed.draft.changed) autoTimer = setTimeout(autoSave, 1500);
    }
    function autoSave() {
      if (!autoSaves() || !ed.draft.changed) return;
      if (ed.busy || ed.pending || ed.writing) { scheduleAutoSave(); return; }
      ed.busy = true;
      Promise.resolve().then(function () { return api().SaveDraft(ed.id, ed.draft.revision); }).then(function (saved) {
        took(saved);
        ed.$('moved').hidden = true;
        ed.say('Saved.');
        notes(saved.notes);
      }).catch(function (err) {
        autoStopped = message(err);
        ed.say('Automatic saving stopped: ' + autoStopped + ' Save once the document is settled to start it again.', true);
        // The banner, when the file moved on, says what changed; the status
        // line keeps saying that saving stopped.
        checkMoved(false);
      }).finally(function () { ed.busy = false; ed.drawHead(); });
    }

    /* ---- The resource moved on ------------------------------------------ */

    /* A refused save is most often a change made to the document elsewhere
       since the draft was opened. Whether it was is asked of the draft
       service, not read from the error's wording. */
    var comparison = null, movedAt = '';
    function refused(err) {
      ed.say(message(err), true, ed.draft ? 'Saving "' + (ed.draft.title || 'Untitled') + '" failed.' : '');
      checkMoved(true);
    }
    /* Whether the file moved on is asked of the draft service. It is asked
       after a refused save, when the window opens, and in the background
       while the window is open, so a change made elsewhere is seen while it
       is small rather than at the end. */
    var movedRequest = 0;
    function checkMoved(afterRefusal) {
      if (!ed.draft || ed.isNew() || !ed.draft.writeBack || ed.disposed) return;
      var generation = ++movedRequest;
      Promise.resolve().then(function () { return api().CompareDraft(ed.id); }).then(function (c) {
        if (ed.disposed || generation !== movedRequest) return;
        if (!c.moved) {
          if (!afterRefusal && !ed.$('moved').hidden) { ed.$('moved').hidden = true; comparison = null; }
          return;
        }
        var fresh = c.revision !== movedAt;
        comparison = c;
        movedAt = c.revision;
        if (!afterRefusal && !fresh && !ed.$('moved').hidden) return;
        var n = (c.conflicts || []).length;
        ed.$('moved-body').textContent = '"' + ed.draft.title + '" was changed on disk since you opened it' +
          (afterRefusal ? ', so your changes were not saved. ' : '. ') +
          (n ? 'You both changed ' + (n === 1 ? 'one part' : n + ' parts') + ' of it; Compare shows ' + (n === 1 ? 'it' : 'them') + ', and once you have changed your draft to settle ' + (n === 1 ? 'it' : 'them') + ', their other changes can be brought in.' :
            'Your changes and theirs do not touch, so theirs can be brought into your draft' + (afterRefusal ? ' before you try again.' : ' now.'));
        ed.$('bring').hidden = n > 0;
        ed.$('moved').hidden = false;
        if (afterRefusal) ed.say('');
      }).catch(function () { /* The next check tries again. */ });
    }
    function bringIn() {
      if (ed.busy || !ed.draft) return;
      ed.busy = true;
      ed.drawHead();
      ed.flush().then(function () {
        if (ed.outside) throw new Error('Settle the change made elsewhere in ' + productName() + ' first.');
        return api().MergeIntoDraft(ed.id, ed.draft.revision);
      }).then(function (v) {
        ed.draft = v;
        ed.replaceText(v.text);
        ed.schedulePreview(0);
        ed.$('moved').hidden = true;
        comparison = null;
        ed.say('Their changes are in your draft. Save when it is ready.');
      }).catch(function (err) { refused(err); })
        .finally(function () { ed.busy = false; ed.drawHead(); });
    }
    ed.$('bring').addEventListener('click', bringIn);

    function diffBlock(text) {
      var pre = document.createElement('pre');
      pre.className = 'editor-diff';
      String(text || '').split('\n').forEach(function (line) {
        var span = document.createElement('span');
        span.textContent = line + '\n';
        if (/^\+(?!\+\+)/.test(line)) span.className = 'add';
        else if (/^-(?!--)/.test(line)) span.className = 'del';
        else if (/^@@/.test(line)) span.className = 'hunk';
        pre.appendChild(span);
      });
      return pre;
    }
    function compare() {
      if (!comparison) return;
      var c = comparison, dialog = document.createElement('dialog');
      dialog.className = 'dialog editor-dialog editor-compare';
      dialog.setAttribute('aria-labelledby', ed.key + '-compare-title');
      dialog.innerHTML = '<h2 class="dialog-title" id="' + ed.key + '-compare-title">Changes Since You Opened It</h2><div class="dialog-body"></div>' +
        '<div class="dialog-actions"><button type="button" class="btn" data-no>Close</button><button type="button" class="btn btn-primary" data-yes>Bring In Their Changes</button></div>';
      var body = dialog.querySelector('.dialog-body');
      function section(title, node) {
        var h = document.createElement('h3');
        h.textContent = title;
        body.append(h, node);
      }
      (c.conflicts || []).forEach(function (k, i) {
        var box = document.createElement('div');
        box.className = 'editor-conflict';
        [['Yours', k.mine], ['Theirs', k.theirs]].forEach(function (side) {
          var figure = document.createElement('figure'), caption = document.createElement('figcaption'), pre = document.createElement('pre');
          caption.textContent = side[0];
          pre.className = 'editor-diff';
          pre.textContent = side[1] || '(removed)';
          figure.append(caption, pre);
          box.appendChild(figure);
        });
        section('Both Changed, From Line ' + k.line + (c.conflicts.length > 1 ? ' (' + (i + 1) + ' of ' + c.conflicts.length + ')' : ''), box);
      });
      section('Changed Elsewhere', diffBlock(c.theirs || 'Nothing.'));
      section('Changed in Your Draft', diffBlock(c.mine || 'Nothing.'));
      var yes = dialog.querySelector('[data-yes]');
      yes.disabled = (c.conflicts || []).length > 0;
      yes.title = yes.disabled ? 'Change your draft to settle the parts you both changed first' : '';
      ed.root.appendChild(dialog);
      function close() { dialog.close(); dialog.remove(); }
      dialog.addEventListener('close', function () { if (dialog.isConnected) dialog.remove(); });
      dialog.querySelector('[data-no]').addEventListener('click', close);
      yes.addEventListener('click', function () { close(); bringIn(); });
      dialog.showModal();
    }
    ed.$('compare').addEventListener('click', compare);

    function slug(title) {
      return String(title || 'document').trim().replace(/[\\/:*?"<>|]+/g, ' ').replace(/\s+/g, ' ').trim() || 'document';
    }
    /* The name Save As suggests: a copy's name beside an opened file, or the
       title of a new document, given an extension when it has none. */
    function suggestedName() {
      var current = originRef(ed.draft);
      if (current && ed.draft.origin.kind === 'local-folder') return current.path.slice(current.path.lastIndexOf('/') + 1).replace(/(\.[^.]+)?$/, ' copy$1');
      var name = slug(ed.draft.title);
      return /\.[A-Za-z0-9]{1,8}$/.test(name) ? name : name + '.md';
    }
    /* Save As starts in the folder of the document it was opened from, and
       for a new document at the workspace itself, which takes a file into
       its main folder. */
    function saveAsStart() {
      var current = originRef(ed.draft);
      if (!current || !api().ResolveWorkspaceResourceMount) return dinahResources.currentPath();
      return Promise.resolve(api().ResolveWorkspaceResourceMount(current)).then(function (item) {
        return item.path.slice(0, item.path.lastIndexOf('/')) || '/';
      }).catch(function () { return dinahResources.currentPath(); });
    }
    // What a folder takes, as Save As asks it.
    var saveKind = dinahResources.documentHomes();
    function settled() {
      return ed.flush().then(function () { if (ed.outside) throw new Error('Settle the change made elsewhere first.'); });
    }
    /* To Your Computer saves a copy, which the draft does not follow; a file
       that is there already is replaced only when the person agrees. */
    function saveToComputer(name) {
      var ws = ed.draft.origin.workspaceId || host().workspaceId();
      return settled().then(function () { return api().ChooseComputerSavePath(ws, name); }).then(function (path) {
        if (!path) return false;
        return Promise.resolve(api().SaveDraftToComputer(ed.id, path, false)).catch(function (err) {
          if (!/already exists/.test(message(err))) throw err;
          return host().ask('Replace the File?', path + ' already exists. Replace it with this document?', 'Replace', true).then(function (yes) {
            return yes ? api().SaveDraftToComputer(ed.id, path, true) : Promise.reject(new Error('Nothing was saved, so choose another name.'));
          });
        }).then(function () {
          ed.say('Saved a copy.');
          host().toast('Saved a copy to ' + path + '.', 'positive');
          return true;
        });
      });
    }
    /* A judgement agent suggests a name for a new document while Save As is
       open (judgement-agents.md, document-name): the box holds untitled until
       the answer comes, then the suggestion, unless the person typed
       meanwhile. Nothing about the dialog waits on it. Saving under another
       name is recorded with the call and never changes what was saved. */
    var nameSuggestion = null;
    function suggestName() {
      if (!ed.isNew() || !api().SuggestDocumentName) return null;
      return Promise.resolve().then(function () { return api().SuggestDocumentName(ed.id); }).then(function (r) {
        if (!r || !r.name || !String(r.name).trim()) return null;
        var name = String(r.name).trim();
        nameSuggestion = { name: /\.[A-Za-z0-9]{1,8}$/.test(name) ? name : name + '.md', call: r.call || 0 };
        return nameSuggestion.name;
      }).catch(function () { return null; });
    }
    function recordNameChoice(chosen) {
      if (nameSuggestion && nameSuggestion.call && chosen !== nameSuggestion.name) Promise.resolve(api().CorrectJudgement(nameSuggestion.call, chosen)).catch(function () {});
    }
    function saveAs() {
      if (!ed.draft || ed.busy || ed.outside) return;
      ed.busy = true;
      ed.drawHead();
      nameSuggestion = null;
      Promise.resolve(saveAsStart()).then(function (start) {
        return dinahFilePicker.pick({ mode: 'save', title: 'Save As', start: start, name: suggestedName(), saveKind: saveKind, suggest: suggestName(),
          commit: function (choice) {
            if (choice.computer) return saveToComputer(choice.computer).then(function (savedIt) {
              if (savedIt) recordNameChoice(choice.computer);
              return savedIt;
            });
            var ref = choice.dir.ref;
            var target = { workspaceId: ref.workspaceId, sourceId: ref.sourceId, path: (ref.path && ref.path !== '.' ? ref.path + '/' : '') + choice.name };
            return settled().then(function () { return api().SaveDraftAs(ed.id, ed.draft.revision, target); }).then(function (saved) {
              took(saved);
              autoStopped = '';
              ed.say('Saved.');
              host().toast('Saved "' + choice.name + '".', 'positive');
              notes(saved.notes);
              recordNameChoice(choice.name);
            });
          } });
      }).catch(function (err) { ed.say(message(err), true); }).finally(function () { ed.busy = false; ed.drawHead(); });
    }

    /* Rename… gives a file a new name in its folder at once, and the draft
       and its changes go with it. A document kept anywhere else has no file
       name to change. */
    function canRename() { return !!ed.draft && !ed.busy && !ed.outside && ed.draft.origin.kind === 'local-folder'; }
    function rename() {
      if (!canRename()) return;
      return settled().then(function () { return dinahResources.renameFile({ ref: originRef(ed.draft) }); }).then(function (renamed) {
        var mine = renamed && (renamed.drafts || []).filter(function (v) { return v.id === ed.id; })[0];
        if (!mine || ed.disposed) return;
        ed.draft = mine;
        ed.drawHead();
        ed.say('The file is renamed.');
      }).catch(function (err) { ed.say(message(err), true); });
    }

    function discard() {
      if (!ed.draft) return;
      host().ask('Discard This Draft?', 'Your changes to "' + ed.draft.title + '" that are not saved will be lost. ' + (ed.isNew() ? 'The document has not been saved anywhere.' : 'The document itself is not changed.'), 'Discard Draft', true).then(function (go) {
        if (!go || ed.disposed) return;
        clearTimeout(ed.writeTimer);
        ed.pending = false;
        ed.discarding = true;
        return (ed.writing || Promise.resolve()).then(function () { return api().DiscardDraft(ed.id, ed.draft.revision); }).then(function () {
          ed.allowClose = true;
          window.pudlWindows.close(ed.key);
        });
      }).catch(function (err) { ed.discarding = false; ed.say(message(err), true); });
    }

    return {
      save: save, scheduleAutoSave: scheduleAutoSave, checkMoved: checkMoved, saveAs: saveAs, canRename: canRename, rename: rename, discard: discard,
      teardown: function () { clearTimeout(autoTimer); }
    };
  };
})();
