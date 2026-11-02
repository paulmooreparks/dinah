/* HTML that Dinah did not write itself is made safe here before it is put
   on the page: an agent's message drawn from its Markdown, a document's
   preview (which keeps the HTML written in the document), and HTML pasted
   into Page view. The page's Content Security Policy (shell_server.go)
   already runs no inline script; this keeps the markup clean as well, so
   that nothing depends on the policy alone.

   The HTML must not run anything, and it must not change anything on the
   page outside the box it is drawn in, since an agent can write a document
   that is then previewed beside an approval card. So:

   - An element that holds code, a frame, a form or a stylesheet is left
     out with its content. A style element, in SVG or not, applies to the
     whole page, so none is kept.
   - Every event handler is removed, and so is every address whose scheme
     is not the web's (or, for a link, mail's), except a picture's data
     address. An address with no scheme, such as a place on the page or
     another document, is kept. An SVG animation that would set a link's
     address is left out, since it could set one with any scheme.
   - A style attribute is kept only on an SVG element, and only with the
     properties in STYLE_PROPERTIES, none of which positions an element or
     lets it paint outside its own box. An HTML element loses it.
   - The attributes that put an element in the top layer (popover), give it
     a page-wide key (accesskey) or take focus (autofocus) are removed.
   - An id or a name could replace one of Dinah's own elements by its id,
     or a global of the same name. Both are removed, unless the caller gives
     a scope, in which case every id is moved into that scope and every
     reference to an id follows (see fragment).

   The boxes the HTML is drawn in contain their paint as well (app.css,
   preview.css), so that a class name of Dinah's or PUDL's that positions
   an element still keeps it inside the box.

   The HTML is parsed into a template element, whose content runs no script
   and loads nothing until it is placed in the page. */
(function () {
  'use strict';
  var DROPPED = 'script,style,iframe,frame,frameset,object,embed,applet,noscript,template,link,meta,base,form,portal';
  var SVG = 'http://www.w3.org/2000/svg';

  // The CSS properties an SVG element's style, or a diagram's stylesheet
  // (diagrams.js), may set. None of them positions an element, stacks it
  // or lets it overflow its box.
  var STYLE_PROPERTIES = new Set(('fill fill-opacity fill-rule stroke stroke-width stroke-opacity stroke-dasharray stroke-dashoffset stroke-linecap stroke-linejoin stroke-miterlimit opacity color background-color font font-family font-size font-weight font-style font-variant text-anchor text-decoration text-transform dominant-baseline alignment-baseline baseline-shift letter-spacing word-spacing line-height white-space word-break overflow-wrap display visibility width height min-width min-height max-width max-height text-align vertical-align padding padding-top padding-right padding-bottom padding-left margin margin-top margin-right margin-bottom margin-left border border-width border-style border-color border-radius box-sizing filter clip-path mask marker-start marker-mid marker-end').split(' '));
  // Whether a CSS value is plain: no escapes, comments, at-rules or markup,
  // no address other than a reference to an element of the same drawing,
  // and no function that is not a colour, a length or a transform.
  function safeValue(value) {
    if (/[\\@<>]|\/\*|expression\s*\(/i.test(value)) return false;
    var functions = value.replace(/url\(\s*["']?#[\w:.-]+["']?\s*\)/gi, '').match(/[\w-]+\s*\(/g) || [];
    return functions.every(function (name) { return /^(rgb|rgba|hsl|hsla|lab|lch|oklab|oklch|color|calc|min|max|clamp|drop-shadow|blur|brightness|contrast|grayscale|hue-rotate|invert|opacity|saturate|sepia|matrix|matrix3d|translate|translateX|translateY|scale|scaleX|scaleY|rotate|skewX|skewY)\s*\($/i.test(name); });
  }
  // A CSS declaration block, given as a CSSStyleDeclaration or as text,
  // with only the properties and values above, as text. value(property,
  // value) may change a value first, as diagrams.js does with font names.
  function declarations(style, value) {
    if (typeof style === 'string') { var text = style; style = document.createElement('span').style; style.cssText = text; }
    var output = [];
    Array.prototype.forEach.call(style, function (property) {
      var v = style.getPropertyValue(property);
      if (value) v = value(property, v);
      if (STYLE_PROPERTIES.has(property) && safeValue(v)) output.push(property + ':' + v + (style.getPropertyPriority(property) ? ' !important' : ''));
    });
    return output.join(';');
  }

  // Whether an attribute may stay, by its name and value. An id, a name and
  // a style attribute are decided by fragment.
  function attr(name, value) {
    name = String(name).toLowerCase();
    if (/^on/.test(name) || /^popover/.test(name) || name === 'srcdoc' || name === 'formaction' || name === 'action' || name === 'accesskey' || name === 'autofocus') return false;
    if (name !== 'href' && name !== 'src' && name !== 'xlink:href') return true;
    // A browser ignores tabs, line breaks and spaces in an address, so
    // "java\tscript:" is read as the scheme it would be followed as.
    var v = String(value).replace(/[\u0000- ]/g, ''), m = /^([^:/?#]+):/.exec(v), scheme = m ? m[1].toLowerCase() : '';
    return !scheme || scheme === 'http' || scheme === 'https' || (scheme === 'mailto' && name === 'href') || (name === 'src' && /^data:image\//i.test(v));
  }
  // Whether an element is an SVG animation of a link's address.
  function animatesAddress(el) {
    return (el.localName === 'animate' || el.localName === 'set') && /^(xlink:)?href$/i.test(el.getAttribute('attributeName') || '');
  }
  // Every reference to an id in an attribute's value, moved by to(id):
  // "#id" in an address, "url(#id)" in a value, and the ids listed in an
  // ARIA relation.
  var URL_REFERENCE = /url\(\s*(["']?)#([^"')\s]+)\1\s*\)/g;
  function moveReferences(name, value, to) {
    if (/^aria-(labelledby|describedby|controls|owns|activedescendant|details|errormessage|flowto)$/.test(name)) return value.split(/\s+/).map(function (id) { return id && to(id); }).join(' ');
    if (name === 'href' || name === 'xlink:href') {
      if (value.charAt(0) !== '#' || value.length < 2) return value;
      var target;
      try { target = decodeURIComponent(value.slice(1)); } catch (_) { return value; }
      return '#' + encodeURIComponent(to(target));
    }
    return value.replace(URL_REFERENCE, function (whole, quote, id) { return 'url(' + quote + '#' + to(id) + quote + ')'; });
  }
  /* The HTML as a document fragment, made safe. options.scope, if given,
     keeps the ids: each becomes scope + "-" + the id, and every reference
     to an id follows by the same rule, so a preview drawn in several parts
     keeps its heading links between them, and an SVG its gradients. The
     scope must not be used by anything else on the page. */
  function fragment(html, options) {
    var scope = options && options.scope;
    function to(id) { return scope + '-' + id; }
    var t = document.createElement('template');
    t.innerHTML = html == null ? '' : String(html);
    t.content.querySelectorAll(DROPPED).forEach(function (el) { el.remove(); });
    t.content.querySelectorAll('*').forEach(function (el) {
      if (animatesAddress(el)) { el.remove(); return; }
      Array.prototype.slice.call(el.attributes).forEach(function (a) {
        var name = a.name.toLowerCase();
        if (name === 'id') { if (scope && a.value) el.id = to(a.value); else el.removeAttribute(a.name); }
        else if (name === 'name') el.removeAttribute(a.name);
        else if (name === 'style') {
          var kept = el.namespaceURI === SVG ? declarations(a.value) : '';
          if (kept && scope) kept = moveReferences(name, kept, to);
          if (kept) el.setAttribute(a.name, kept); else el.removeAttribute(a.name);
        } else if (!attr(a.name, a.value)) el.removeAttribute(a.name);
        else if (scope) {
          var moved = moveReferences(name, a.value, to);
          if (moved !== a.value) el.setAttribute(a.name, moved);
        }
      });
    });
    return t.content;
  }
  /* Text as HTML that shows it as it is: the five characters that markup
     reads, &, <, >, " and ', become their references, so the result is
     safe in an element's content and in a quoted attribute alike. This is
     the page's one escaper (frontend_policy_test.go). */
  var REFERENCES = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
  function text(value) { return String(value == null ? '' : value).replace(/[&<>"']/g, function (c) { return REFERENCES[c]; }); }
  // The HTML as a string, made safe.
  function clean(html, options) {
    var box = document.createElement('div');
    box.appendChild(fragment(html, options));
    return box.innerHTML;
  }

  window.dinahHTML = Object.freeze({ attr: attr, fragment: fragment, clean: clean, text: text, declarations: declarations, safeValue: safeValue });
})();
