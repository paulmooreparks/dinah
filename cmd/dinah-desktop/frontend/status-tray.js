/* Dinah owns the items in PUDL's tray. A narrow bar offers the same
   controls through one Status item; execution changes never choose its layout. */
(function () {
  'use strict';
  document.addEventListener('DOMContentLoaded', function () {
    var bar = document.querySelector('footer.win-bar'), area = bar.querySelector('.status-area');
    var opener = document.getElementById('status-folded'), panel = document.getElementById('status-tray-menu');
    var names = { 'status-workspace': 'Workspace', 'status-needs': 'Notifications', 'status-agent': 'Agent', 'status-judgements': 'Judgements', 'status-update': 'Update' };
    var sources = Object.keys(names).map(function (id) { return document.getElementById(id); });
    var rows = new Map(), folded = false;
    function node(tag, cls, text) { var el = document.createElement(tag); if (cls) el.className = cls; if (text != null) el.textContent = text; return el; }
    function setText(el, text) { if (el.textContent !== text) el.textContent = text; }
    function setAttr(el, name, value) { if (el.getAttribute(name) !== value) el.setAttribute(name, value); }
    function describe(source) {
      var text = source.dataset.statusText;
      if (source.id === 'status-workspace') text = document.getElementById('status-workspace-name').textContent;
      if (source.id === 'status-update') text = 'Ready to install';
      return { word: names[source.id], state: source.dataset.statusState || 'idle', text: text || '', count: Number(source.dataset.statusCount) || 0,
        glyph: source.dataset.statusGlyph || (source.id === 'status-workspace' ? 'folder' : source.id === 'status-update' ? 'reload' : 'ring'),
        colour: source.dataset.statusColour || 'text-muted', label: names[source.id] + ': ' + (text || 'Available') + (source.hasAttribute('data-status-count') ? ', ' + (Number(source.dataset.statusCount) || 0) + ' unseen' : '') + '. ' + (source.getAttribute('aria-label') || source.title || ''), available: !source.hidden };
    }
    sources.forEach(function (source) {
      source.classList.add('status-tray-item');
      var li = node('li'), row = node('button', 'btn status-tray-row'); row.type = 'button';
      row.id = 'folded-' + source.id; row.dataset.statusFor = source.id;
      row.setAttribute('popovertarget', source.getAttribute('popovertarget'));
      var glyph = node('span', 'glyph'); glyph.setAttribute('aria-hidden', 'true');
      var words = node('span', 'status-tray-words'), count = node('span', 'status-row-count'); count.setAttribute('aria-hidden', 'true');
      var value = node('span', 'badge warn'), reserve = node('span', 'badge warn status-badge-reserve', '99+'); reserve.setAttribute('inert', '');
      count.append(value, reserve); row.append(glyph, words, count); li.appendChild(row); panel.querySelector('ul').appendChild(li);
      var target = document.getElementById(source.getAttribute('popovertarget'));
      row.addEventListener('click', function () {
        // Run the original button's preparation without invoking its hidden
        // popover anchor. This row's native invoker opens that same panel.
        source.addEventListener('click', function (event) { event.preventDefault(); }, { once: true });
        source.click(); target.dataset.menuAnchor = row.id;
      });
      target.addEventListener('toggle', function (event) {
        if (event.newState !== 'closed') return;
        var active = document.activeElement;
        if (target.dataset.menuAnchor === row.id && panel.matches(':popover-open') && (!active || active === document.body || target.contains(active))) row.focus({ preventScroll: true });
        target.dataset.menuAnchor = source.id;
      });
      source.addEventListener('click', function () { if (!folded) target.dataset.menuAnchor = source.id; });
      rows.set(source, { li: li, row: row, glyph: glyph, words: words, value: value });
    });
    function paint() {
      var urgent = null, total = 0, labels = [], rank = { stuck: 0, waiting: 1, working: 2, idle: 3 };
      sources.forEach(function (source) {
        var spec = describe(source), parts = rows.get(source);
        parts.li.hidden = !spec.available;
        if (spec.available) {
          total += spec.count; labels.push(spec.label);
          if (!urgent || rank[spec.state] < rank[urgent.state]) urgent = spec;
        }
        setText(parts.words, spec.word + (spec.text ? ' · ' + spec.text : ''));
        setAttr(parts.row, 'aria-label', spec.label); setAttr(parts.row, 'title', spec.label);
        parts.glyph.style.setProperty('--glyph', 'var(--glyph-' + spec.glyph + ')'); parts.glyph.style.color = 'var(--' + spec.colour + ')';
        setText(parts.value, spec.count > 99 ? '99+' : String(spec.count || '')); parts.value.hidden = !spec.count;
      });
      var starting = document.getElementById('status-starting');
      if (!starting.hidden) { labels.unshift('Dinah is starting.'); if (!urgent || rank[urgent.state] > rank.working) urgent = { state: 'working', glyph: 'chat-working', colour: 'accent' }; }
      var badge = opener.querySelector('[data-status-badge]'), glyph = opener.querySelector('.glyph');
      setText(badge, total > 99 ? '99+' : String(total || '')); badge.hidden = !total;
      glyph.style.setProperty('--glyph', 'var(--glyph-' + (urgent ? urgent.glyph : 'ring') + ')'); glyph.style.color = 'var(--' + (urgent ? urgent.colour : 'text-muted') + ')';
      setAttr(opener, 'data-status-state', urgent ? urgent.state : 'idle'); setAttr(opener, 'data-status-count', String(total));
      setAttr(opener, 'title', 'Status: ' + total + ' unseen. ' + labels.join(' ')); setAttr(opener, 'aria-label', 'Status: ' + total + ' unseen. ' + labels.join(' '));
    }
    // The probe has constant labels and the widest badge forms. PUDL's
    // compact tray hides words; no live state supplies a measured width.
    var probe = node('div', 'status-tray-measure'); probe.setAttribute('data-win-bar-compact', ''); probe.setAttribute('aria-hidden', 'true'); probe.setAttribute('inert', '');
    var front = node('span', 'win-tab'), more = node('span', 'win-tab win-bar-more', 'More'), tray = node('div', 'status-tray-measure-items');
    front.append(node('span', 'glyph win-item-glyph'), node('span', 'win-item-title'));
    sources.forEach(function (source) {
      var copy = source.cloneNode(true); copy.hidden = false; copy.classList.remove('status-tray-item');
      copy.removeAttribute('id'); copy.querySelectorAll('[id]').forEach(function (el) { el.removeAttribute('id'); });
      if (copy.querySelector('.status-badge-reserve')) copy.querySelector('[data-status-badge]').hidden = true;
      Array.from(copy.children).forEach(function (el) { if (!el.matches('.glyph, .badge, img, svg, .status-picture')) el.hidden = true; });
      tray.appendChild(copy);
    });
    probe.append(front, more, tray); bar.parentElement.appendChild(probe);
    function fit() {
      var barStyle = getComputedStyle(bar), areaStyle = getComputedStyle(area);
      ['paddingLeft', 'paddingRight', 'borderLeftWidth', 'borderRightWidth', 'borderLeftStyle', 'borderRightStyle', 'columnGap'].forEach(function (name) { probe.style[name] = barStyle[name]; tray.style[name] = areaStyle[name]; });
      var staticControls = Array.from(bar.children).filter(function (el) { return !el.matches('.win-dock, .status-area, .win-bar-more'); });
      var staticWidth = staticControls.reduce(function (width, el) { var css = getComputedStyle(el); return width + el.getBoundingClientRect().width + parseFloat(css.marginLeft) + parseFloat(css.marginRight); }, 0);
      var css = getComputedStyle(more), maximumMore = parseFloat(css.maxWidth);
      var dock = bar.querySelector('.win-dock'), dockStyle = getComputedStyle(dock);
      var budget = probe.getBoundingClientRect().width + staticWidth + staticControls.length * parseFloat(barStyle.columnGap) + (isFinite(maximumMore) ? maximumMore - more.getBoundingClientRect().width : 0)
        + parseFloat(dockStyle.paddingLeft) + parseFloat(dockStyle.paddingRight);
      var next = bar.getBoundingClientRect().width < Math.ceil(budget);
      if (next !== folded) {
        var active = document.activeElement, returnFocus = active === opener || panel.contains(active) || sources.includes(active);
        sources.forEach(function (source) { var target = document.getElementById(source.getAttribute('popovertarget')); if (target.matches(':popover-open')) { returnFocus = returnFocus || target.contains(active); target.hidePopover(); } });
        if (panel.matches(':popover-open')) panel.hidePopover();
        folded = next;
        area.toggleAttribute('data-status-folded', folded); opener.hidden = !folded;
        if (returnFocus) (folded ? opener : sources.find(function (source) { return !source.hidden; })).focus();
      }
      area.dataset.statusReservedWidth = String(Math.ceil(budget));
    }
    var observer = new MutationObserver(function (changes) {
      if (changes.some(function (change) { return change.target.id === 'status-starting' || sources.some(function (source) { return source === change.target || source.contains(change.target); }); })) paint();
    });
    observer.observe(area, { subtree: true, childList: true, characterData: true, attributes: true,
      attributeFilter: ['hidden', 'title', 'aria-label', 'data-status-state', 'data-status-text', 'data-status-glyph', 'data-status-colour', 'data-status-count'] });
    window.addEventListener('resize', fit);
    var sizes = new ResizeObserver(fit); sizes.observe(probe);
    paint(); fit();
  });
})();
