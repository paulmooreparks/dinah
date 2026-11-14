/* The shortcodes a build adds, as services for the applets. The window's
   page gives them to every content applet (content-host.js), and a slide
   show's page to its show (show-host.js).

   A build may add shortcodes to the Markdown dialect beyond the engine's
   own (docs/design/shortcodes.md). Two services give them to the applets.
   contentWorkspace.shortcodes(context) is the applets' documented call
   for a host's extra shortcodes, and returns { shortcodes: [{ name,
   title, description, kind, params, example, prefix, managed, literal,
   colorAttribute, colorClassPrefix, cellColor }] }
   for completion, hover and the Insert menu. One inline entry may have
   cellColor, when it is the shortcode the product keeps a table cell's
   colour with in a Markdown table. An entry has a prefix when
   the shortcode may be written with one character in place of its name
   and colon, is managed when the product keeps it with a tool of its
   own, and is literal when it is a container whose body is markup and
   not Markdown. A literal entry has a colorAttribute when the product
   keeps a table cell's colour in that data attribute beside the cell's
   style, and a colorClassPrefix when the product also marks a coloured
   cell with a class whose name begins so, which the applets remove when
   the colour is changed or removed. contentWorkspace.shortcodeHost draws and changes an occurrence:
   render({ name, args, source, kind }) returns { html }, where kind is
   how the applet read the occurrence and the HTML of a container or a
   span has one <slot name="content">
   </slot> for the body the applet draws itself; form(source) returns the
   form an occurrence is changed through, or null when it is changed as
   text; build(kind, values) returns { source }, or { error: { field,
   message } } for a value the shortcode cannot have. The catalog is read
   once and kept, since it is the build's. */
(function () {
  'use strict';
  function app() { return window.go && window.go.main && window.go.main.App; }
  var shortcodeCatalog = null;
  function shortcodeList() {
    if (!shortcodeCatalog) {
      shortcodeCatalog = Promise.resolve().then(function () { return app().ShortcodeCatalog ? app().ShortcodeCatalog() : []; })
        .then(function (list) {
          return (Array.isArray(list) ? list : []).map(function (d) {
            var entry = { name: String(d.name || ''), title: String(d.title || d.name || ''), description: String(d.description || ''), kind: String(d.kind || ''), example: String(d.example || ''),
              params: (Array.isArray(d.params) ? d.params : []).map(function (p) { return { name: String(p.name || ''), description: String(p.description || ''), required: p.required === true }; }) };
            if (typeof d.prefix === 'string' && d.prefix.length === 1) entry.prefix = d.prefix;
            if (d.managed === true) entry.managed = true;
            if (d.literal === true) entry.literal = true;
            if (d.literal === true && typeof d.colorAttribute === 'string' && d.colorAttribute) entry.colorAttribute = d.colorAttribute;
            if (d.literal === true && typeof d.colorClassPrefix === 'string' && d.colorClassPrefix) entry.colorClassPrefix = d.colorClassPrefix;
            if (d.cellColor === true && d.kind === 'inline') entry.cellColor = true;
            return entry;
          });
        })
        .catch(function () { shortcodeCatalog = null; return []; });
    }
    return shortcodeCatalog;
  }
  function shortcodes() { return shortcodeList().then(function (list) { return { shortcodes: list }; }); }
  var shortcodeHost = {
    version: 1,
    render: function (request) {
      var r = request || {};
      return Promise.resolve(app().RenderShortcode({ name: String(r.name || ''), args: String(r.args || ''), source: String(r.source || ''), content: r.content == null ? '' : String(r.content), kind: r.kind == null ? '' : String(r.kind) }))
        .then(function (drawn) {
          /* The engine answers { html, flow, print }; flow says a block's drawing
             is flow content the applet may break across pages. */
          var html = drawn && typeof drawn.html === 'string' ? drawn.html : '';
          var result = { html: html };
          if (drawn && drawn.flow === true) result.flow = true;
          /* print: "none" keeps a drawing off paper; without it the applet prints the drawing as the screen shows it. */
          if (drawn && drawn.print === 'none') result.print = 'none';
          return result;
        });
    },
    form: function (source) {
      return Promise.resolve(app().ShortcodeForm(String(source || ''))).then(function (form) { return form && form.kind ? form : null; });
    },
    build: function (kind, values) {
      var v = {};
      Object.keys(values || {}).forEach(function (name) { v[name] = values[name] == null ? '' : String(values[name]); });
      return Promise.resolve(app().BuildShortcode(String(kind || ''), v)).then(function (built) {
        if (built && built.error) return { error: { field: built.error.field || '', message: built.error.message || 'The shortcode could not be written.' } };
        return { source: built && built.source || '' };
      });
    }
  };
  window.dinahShortcodeServices = { shortcodes: shortcodes, shortcodeHost: shortcodeHost };
})();
