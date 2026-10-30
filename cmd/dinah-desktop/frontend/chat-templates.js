/* A chat's template chooser and editing form use the shared template list. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, element: $, hidePop, typed, skillsChanged, toast, fail } = adapter;

    /* ---- Personal templates -----------------------------------------------
       Prompts the person keeps under a name, in the settings file. They come
       first in the Skills menu and in the "/" pop-up, where typing filters
       them by name; choosing one puts its text in the box. */
    function templateRows(host, list, onPick, selected) {
      if (!list.length) return;
      var h = document.createElement('div');
      h.className = 'md-section-label';
      h.textContent = 'My Templates';
      host.appendChild(h);
      list.forEach(function (t) {
        var r = document.createElement('div');
        r.className = 'md-row skill-row template-row';
        r.setAttribute('role', 'option');
        r.innerHTML = '<a class="md-item" href="#"><span class="template-name"></span><span class="md-meta"></span></a>';
        r.querySelector('.template-name').textContent = t.name;
        r.querySelector('.md-meta').textContent = t.text.split('\n')[0];
        if (selected === 'tpl:' + t.name) r.setAttribute('aria-selected', 'true');
        r.firstChild.addEventListener('mousedown', function (e) { e.preventDefault(); });
        r.firstChild.addEventListener('click', function (e) { e.preventDefault(); onPick(t.name); });
        host.appendChild(r);
      });
    }
    // Puts a template's text in the box: in place of the "/word" being typed,
    // or at the caret.
    function insertTemplate(name, tok) {
      var t = adapter.templates().filter(function (x) { return x.name === name; })[0];
      if (!t) return;
      var box = $('prompt'), v = box.value;
      var start = tok ? tok.start : box.selectionStart, end = tok ? tok.end : box.selectionEnd;
      box.value = v.slice(0, start) + t.text + v.slice(end);
      box.setSelectionRange(start + t.text.length, start + t.text.length);
      hidePop();
      typed();
      box.focus();
    }
    function setTemplates(l) { adapter.setTemplates(l || []); skillsChanged(); drawTemplates(); }

    var editing = null;   // the name of the template in the form, if it is an existing one
    function drawTemplates() {
      var host = $('templates-list');
      host.innerHTML = '';
      $('templates-empty').hidden = adapter.templates().length > 0;
      host.hidden = !adapter.templates().length;
      adapter.templates().forEach(function (t) {
        var li = document.createElement('li');
        li.className = 'template-item';
        li.innerHTML = '<div class="template-head"><strong class="template-name"></strong><span class="hstack gap-2">' +
          '<button type="button" class="btn btn-sm" data-t="use">Use</button>' +
          '<button type="button" class="btn btn-sm" data-t="edit">Edit</button>' +
          '<button type="button" class="btn btn-sm btn-danger" data-t="delete">Delete</button></span></div><p class="history-text"></p>';
        li.querySelector('.template-name').textContent = t.name;
        li.querySelector('.history-text').textContent = t.text;
        li.querySelector('[data-t="use"]').addEventListener('click', function () { $('templates').close(); insertTemplate(t.name, null); });
        li.querySelector('[data-t="edit"]').addEventListener('click', function () { templateForm(t.name, t.text); });
        var del = li.querySelector('[data-t="delete"]');
        // A first press asks again, in the button itself.
        del.addEventListener('click', function () {
          if (del.dataset.armed) { app().DeleteTemplate(t.name).then(setTemplates).catch(fail); return; }
          del.dataset.armed = '1';
          del.textContent = 'Delete It?';
        });
        host.appendChild(li);
      });
    }
    function templateForm(name, text) {
      editing = name || null;
      $('template-form-title').textContent = name ? 'Change ' + name : 'New Template';
      $('template-name').value = name || '';
      $('template-text').value = text || '';
      (name ? $('template-text') : $('template-name')).focus();
    }
    function openTemplates(text) {
      drawTemplates();
      if (!$('templates').open) $('templates').showModal();
      templateForm(null, text ? text.trim() : '');
    }
    $('template-form').addEventListener('submit', function (e) {
      e.preventDefault();
      var name = $('template-name').value.trim(), text = $('template-text').value;
      // A renamed template replaces the old one, once the new one is saved.
      var old = editing && editing.toLowerCase() !== name.toLowerCase() ? editing : null;
      var go = app().SaveTemplate(name, text).then(function (l) { return old ? app().DeleteTemplate(old) : l; });
      go.then(function (l) { setTemplates(l); templateForm(null, ''); toast('Saved the template "' + name + '".', 'positive'); }).catch(fail);
    });
    $('template-new').addEventListener('click', function () { templateForm(null, ''); });

    return { templateRows, insertTemplate, openTemplates };
  }

  window.dinahChatTemplates = { create };
})();
