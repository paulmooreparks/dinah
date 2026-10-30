/* Shared argument guidance, controls and command assembly for skill callers. */
(function () {
  'use strict';
  function parse(hint) {
    var out = [], i = 0, s = hint || '';
    function placeholder(text, optional, quoted) {
      var name = text.trim(), f = { label: name, key: name.toLowerCase(), raw: name, required: !optional, quoted: !!quoted };
      if (name.includes('|')) { f.kind = 'choice'; f.choices = name.split('|'); f.label = 'Choice'; }
      else if (/\s/.test(name) && !quoted) f.kind = 'words';
      else f.kind = 'text';
      return f;
    }
    function inner(text, optional) {
      text = text.trim(); var m;
      if ((m = /^(--[\w-]+)\s+<([^>]+)>$/.exec(text))) { var f = placeholder(m[2], optional); f.flag = m[1]; f.label = m[1].slice(2) + ' (' + m[2] + ')'; return f; }
      if ((m = /^(--[\w-]+)$/.exec(text))) return { label: m[1].slice(2), key: m[1], raw: m[1], flag: m[1], kind: 'bool', required: false };
      if ((m = /^"<([^>]+)>"$/.exec(text))) return placeholder(m[1], optional, true);
      if ((m = /^<([^>]+)>$/.exec(text))) return placeholder(m[1], optional);
      return placeholder(text, optional);
    }
    while (i < s.length) {
      var c = s[i], end;
      if (/\s/.test(c)) { i++; continue; }
      if (c === '[') { end = s.indexOf(']', i); if (end < 0) break; out.push(inner(s.slice(i + 1, end), true)); i = end + 1; }
      else if (c === '"' && s[i + 1] === '<') { end = s.indexOf('>"', i); if (end < 0) break; out.push(inner(s.slice(i, end + 2), false)); i = end + 2; }
      else if (c === '<') { end = s.indexOf('>', i); if (end < 0) break; out.push(inner(s.slice(i, end + 1), false)); i = end + 1; }
      else { end = i; while (end < s.length && !/\s/.test(s[end])) end++; out.push(placeholder(s.slice(i, end), false)); i = end; }
    }
    out.forEach(function (f, n) { if (f.kind === 'words' && n !== out.length - 1) f.kind = 'text'; });
    return out;
  }
  function field(spec, id) {
    var wrap = document.createElement('div'); wrap.className = 'skill-field' + (spec.kind === 'bool' ? ' check-field' : '');
    var label = document.createElement('label'), el;
    label.htmlFor = id; label.textContent = spec.label + (spec.required ? '' : ' (optional)');
    if (spec.kind === 'bool') {
      el = document.createElement('input'); el.type = 'checkbox'; label.className = 'check'; label.prepend(el); wrap.append(label);
    } else {
      wrap.append(label);
      var select = spec.kind === 'choice';
      el = document.createElement(select ? 'select' : spec.kind === 'words' ? 'textarea' : 'input');
      el.className = select ? 'form-select' : 'form-input';
      if (select) { var holder = document.createElement('span'); holder.className = 'form-select-wrap'; holder.append(el); wrap.append(holder); }
      else wrap.append(el);
      if (spec.kind === 'choice') {
        if (!spec.required) el.append(new Option('(let the skill ask)', ''));
        spec.choices.forEach(function (choice) { el.append(new Option(choice, choice)); });
      }
    }
    el.id = id;
    if(spec.required)el.setAttribute('aria-required','true');
    return { spec: spec, el: el, wrap: wrap };
  }
  function assemble(name, fields, words, wordsSpec, mark, partial) {
    var parts = ['/' + name], missing = false;
    fields.forEach(function (f) {
      var s = f.spec, value = s.kind === 'bool' ? (f.el.checked ? 'on' : '') : f.el.value.trim();
      if (mark) { f.wrap.classList.toggle('missing', s.required && !value); f.el.setAttribute('aria-invalid',String(!!(s.required && !value))); }
      if (!value) { if (s.required) missing = true; return; }
      if (s.kind === 'bool') { parts.push(s.flag); return; }
      var val = s.quoted || /\s/.test(value) ? '"' + value.replace(/"/g, '\\"') + '"' : value;
      parts.push(s.flag ? s.flag + ' ' + val : val);
    });
    words = (words || '').trim();
    if (wordsSpec && wordsSpec.required && !words) missing = true;
    return missing && !partial ? null : parts.join(' ') + (words ? ' ' + words : '');
  }
  /* The other direction, for the Skill window's parameter designer: the
     argument guidance that parse reads back into these specs. A spec's
     raw text is what the person wrote or chose; the kind follows from it, as
     parse decides. */
  function format(specs) {
    return (specs || []).map(function (s) {
      if (s.kind === 'bool') return s.flag || s.raw;
      var name = s.raw || s.key, inner = s.quoted ? '"<' + name + '>"' : '<' + name + '>';
      if (s.flag) inner = s.flag + ' ' + inner;
      return s.required ? inner : '[' + inner.replace(/^<(.*)>$/, '$1') + ']';
    }).join(' ');
  }
  /* Whether a hint is one the designer can rewrite without losing anything:
     the specs it reads come back unchanged from the text the designer writes. */
  function roundTrips(hint) {
    var a = parse(hint), b = parse(format(a));
    var strip = function (f) { return JSON.stringify({ k: f.kind, r: f.raw, q: !!f.quoted, o: !!f.required, f: f.flag || '', c: f.choices || [] }); };
    return a.length === b.length && a.every(function (f, i) { return strip(f) === strip(b[i]); });
  }
  window.dinahSkillInputs = { parse: parse, field: field, assemble: assemble, format: format, roundTrips: roundTrips };
})();
