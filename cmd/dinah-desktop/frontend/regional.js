/* Dates and times are written as this computer's regional format writes
   them (Paul, 26 October 2026). The host reads the format from the
   operating system (internal/regional) and HostLocale returns it as a
   language tag, such as en-SG-u-hc-h23, which every date and time in the
   page is formatted with through Intl.DateTimeFormat. The page's own lang
   stays English, as its text is. Until the tag arrives, or when the host
   cannot read one, the page's default locale is used.

   Scripts format with dinahRegional.format(date, kind), where kind is one
   of the forms below. dinahRegional.stamp(timeElement, date, kind) fills a
   time element, with the full date and time as its title, and marks it so
   that it is written again if the tag arrives after it was drawn. Text
   written some other way is drawn again on the dinah:locale event. */
(function () {
  'use strict';
  var FORMS = {
    // The time of day, to the minute, as the rows of a chat show it.
    clock: { hour: '2-digit', minute: '2-digit' },
    // The time of day, to the second.
    time: { hour: '2-digit', minute: '2-digit', second: '2-digit' },
    // The day and month with the time, as lists show when something happened.
    dayTime: { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' },
    // The date alone.
    date: { day: 'numeric', month: 'short', year: 'numeric' },
    // The full numeric date and time, as a title or a sentence gives it.
    dateTime: { year: 'numeric', month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' },
    // The numeric date and time with fixed-width fields, for columns of text.
    stamp: { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' },
    // The long date and time, with the day of the week and the time zone.
    full: { dateStyle: 'full', timeStyle: 'long' }
  };
  var tag = '';
  var formatters = {};

  // A tag Intl does not accept is not used; supportedLocalesOf throws a
  // RangeError for a malformed tag.
  function usable(v) {
    if (!v || typeof v !== 'string') return false;
    try { return Intl.DateTimeFormat.supportedLocalesOf([v]).length > 0; } catch (e) { return false; }
  }

  function formatter(kind) {
    var f = formatters[kind];
    if (!f) f = formatters[kind] = new Intl.DateTimeFormat(tag || undefined, FORMS[kind] || FORMS.dateTime);
    return f;
  }

  function toDate(v) { return v instanceof Date ? v : new Date(v); }

  function format(value, kind) {
    var d = toDate(value);
    return isNaN(d) ? '' : formatter(kind || 'dateTime').format(d);
  }

  function stamp(t, value, kind) {
    var d = toDate(value);
    if (isNaN(d)) { t.textContent = ''; t.removeAttribute('datetime'); t.removeAttribute('title'); t.removeAttribute('data-regional'); return; }
    kind = kind || 'clock';
    t.dateTime = d.toISOString();
    t.textContent = format(d, kind);
    t.title = format(d, 'dateTime');
    t.setAttribute('data-regional', kind);
  }

  // Writes again every time element stamped here, from its datetime.
  function refresh(scope) {
    (scope || document).querySelectorAll('time[data-regional][datetime]').forEach(function (t) {
      stamp(t, t.getAttribute('datetime'), t.getAttribute('data-regional'));
    });
  }

  function use(v) {
    v = usable(v) ? v : '';
    if (v === tag) return;
    tag = v;
    formatters = {};
    refresh(document);
    document.dispatchEvent(new CustomEvent('dinah:locale', { detail: { locale: tag } }));
  }

  var ready = Promise.resolve().then(function () {
    var app = window.dinahHelpers.app();
    return app && app.HostLocale ? app.HostLocale() : '';
  }).then(use, function () {});

  window.dinahRegional = {
    // The tag in use, or '' while the page's default locale is used.
    get locale() { return tag; },
    ready: ready,
    format: format,
    stamp: stamp,
    refresh: refresh
  };
})();
