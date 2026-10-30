/* Each chat owns a skill form and assembles its command with skill-inputs. */
(function () {
  'use strict';

  function create(adapter) {
    const { element: $, viewPrefix, typed, send } = adapter;

    /* ---- A skill's parameters as a form ----------------------------------
       A skill's argument-hint (Devin's documented front-matter field) is read
       as the usual convention: <required>, [optional], quotes round a value
       with spaces, a|b for a choice, --flag and --flag <value>. Each
       parameter becomes a field. A last parameter written as words ("the
       change, in your words") is the prompt box itself. Send puts the command
       together; Esc or the cross goes back to typing it by hand. */
    var form = null;      // { skill, fields: [{ spec, el }], words }
    function parseHint(hint) {
      return dinahSkillInputs.parse(hint);
    }

    function openForm(skill) {
      var specs = parseHint(skill.hint);
      if (!specs.length) return false;
      form = { skill: skill, fields: [], words: null };
      $('skill-form-name').textContent = '/' + skill.name;
      $('skill-form-desc').textContent = (skill.description || '').trim();
      $('skill-form-desc').title = (skill.description || '').trim();
      var host = $('skill-form-fields');
      host.innerHTML = '';
      specs.forEach(function (spec, n) {
        if (spec.kind === 'words') {
          form.words = spec;
          $('prompt').placeholder = spec.label.charAt(0).toUpperCase() + spec.label.slice(1) + (spec.required ? '' : ' (optional)');
          return;
        }
        var field = dinahSkillInputs.field(spec, viewPrefix + '-skill-field-' + n);
        var wrap = field.wrap, el = field.el;
        if (spec.kind === 'text' && /title|name/i.test(spec.label)) wrap.classList.add('wide');
        host.appendChild(wrap);
        form.fields.push(field);
        if (spec.kind === 'text') el.placeholder = spec.label;
        el.addEventListener('input', function () { wrap.classList.remove('missing'); preview(); });
        el.addEventListener('change', preview);
      });
      $('skill-form').hidden = false;
      typed();
      preview();
      var first = form.fields.filter(function (f) { return !f.el.value; })[0];
      (first ? first.el : $('prompt')).focus();
      return true;
    }

    // The command the form stands for, or null with the missing fields marked.
    // With partial set, the command so far, complete or not.
    function assemble(mark, partial) {
      return dinahSkillInputs.assemble(form.skill.name, form.fields, $('prompt').value, form.words, mark, partial);
    }
    function preview() { if (form) $('skill-form-preview').textContent = assemble(false, true); }
    $('prompt').addEventListener('input', preview);

    function closeForm(keep) {
      if (!form) return;
      var cmd = keep ? assemble(false, true) : null;
      form = null;
      $('skill-form').hidden = true;
      adapter.refreshWorking();
      if (cmd) { $('prompt').value = cmd + ' '; typed(); }
      $('prompt').focus();
    }
    $('skill-form-close').addEventListener('click', function () { closeForm(true); });
    $('skill-form').addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeForm(true); }
      else if (e.key === 'Enter' && e.target.tagName !== 'SELECT') { e.preventDefault(); send('auto'); }
    });

    function clear() { form = null; $('skill-form').hidden = true; }
    return { openForm, closeForm, assemble, clear, isOpen: () => !!form, skill: () => form && form.skill };
  }

  window.dinahChatSkillForm = { create };
})();
