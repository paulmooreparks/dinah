/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Lifted from Dinah Desktop (cmd/dinah-desktop/frontend/safe-html.js, Apache-2.0). */
/* HTML that Dinah did not write itself is made safe here before it is put
   on the page: an agent's message drawn from its Markdown, a document's
   preview (which keeps the HTML written in the document), and HTML pasted
   into Page view. The site's Content Security Policy (internal/web) runs
   no inline script either; this keeps the markup clean as well, so that
   nothing depends on the policy alone.

   An element that holds code, a stylesheet, a frame or a form is left out
   with its content, except a hosted video's player frame as the video
   shortcode writes it, from the players it knows. A <style> anywhere,
   inside an SVG too, styles the whole page, the site's own dialogs
   included, so none is kept (security review, 2026-11-03). Every event
   handler is removed, and so is every address whose scheme is not the
   web's (or, for a link, mail's), except a picture's data address. An
   address with no scheme, such as a place on the page or another
   document, is kept. A style attribute keeps only properties that change
   how its own element looks, never where it sits on the page or what it
   loads. An SVG animation that would set a link's address is left out,
   since it could set one with any scheme. A document may embed one of the
   site's games and tools, never the site's own applications, which hold
   the person's files and settings. The HTML is parsed into a template
   element, whose content runs no script and loads nothing until it is
   placed in the page. */
(function () {
  'use strict';
  var DROPPED = 'script,style,iframe,frame,frameset,object,embed,applet,noscript,template,link,meta,base,form,portal';
  /* The site's own applications (apps/ in the repository), which a
     document may not embed; scripts/check.mjs keeps this list complete. */
  var NOT_EMBEDDED = ['diagrams', 'docs', 'editor', 'files', 'settings', 'slides', 'terminal'];
  /* What a style attribute may set: how the element itself looks. Never
     position, inset, z-index or transforms, with which a document could
     lay a false control over the page's own, and never an address. */
  var STYLE_OK = /^(--glyph|color|background-color|font-(style|weight|size|family|variant)|text-(align|decoration|transform|indent)|letter-spacing|word-spacing|line-height|vertical-align|white-space|(max-|min-)?(width|height)|margin(-[a-z]+)?|padding(-[a-z]+)?|border(-[a-z]+)*|list-style-type|display|opacity|columns?(-[a-z]+)*|gap|aspect-ratio|object-fit|float|clear)$/;
  function safeStyle(value) {
    var probe = document.createElement('div');
    probe.setAttribute('style', value);
    var out = [];
    for (var i = 0; i < probe.style.length; i++) {
      var name = probe.style[i], v = probe.style.getPropertyValue(name);
      if (!STYLE_OK.test(name) || /url\(|image-set|expression|attr\(/i.test(v)) continue;
      out.push(name + ': ' + v + (probe.style.getPropertyPriority(name) ? ' !important' : ''));
    }
    return out.join('; ');
  }
  /* The one frame kept: a hosted video's player, which the video shortcode
     writes (internal/shortcodes), from the two players it knows, and
     nothing else in it but what the shortcode wrote. */
  var PLAYERS = /^https:\/\/(www\.youtube-nocookie\.com\/embed\/[A-Za-z0-9_-]{6,20}|player\.vimeo\.com\/video\/[0-9]{5,16}\?dnt=1)$/;
  function keptFrame(el) {
    if (el.localName !== 'iframe' || !el.parentNode || !el.parentNode.classList || !el.parentNode.classList.contains('sc-video')) return false;
    if (!PLAYERS.test(el.getAttribute('src') || '')) return false;
    Array.prototype.slice.call(el.attributes).forEach(function (a) {
      if (['src', 'title', 'loading', 'allow', 'referrerpolicy', 'allowfullscreen'].indexOf(a.name.toLowerCase()) < 0) el.removeAttribute(a.name);
    });
    el.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-presentation allow-popups');
    return true;
  }

  // Whether an attribute may stay, by its name and value.
  function attr(name, value) {
    name = String(name).toLowerCase();
    if (/^on/.test(name) || name === 'srcdoc' || name === 'formaction' || name === 'action' || name === 'ping' || name === 'srcset' || name === 'imagesrcset' || name === 'data-host-suggestion' || name === 'data-suggestion-action') return false;
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
  /* A diagram's own stylesheet, which Mermaid writes inside its SVG with
     every rule under the SVG's id: kept with only the rules whose every
     selector starts with that id, so none reaches outside the diagram. */
  function keptDiagramStyle(el) {
    var svg = el.localName === 'style' && el.parentNode && el.parentNode.localName === 'svg' ? el.parentNode : null;
    var id = svg && svg.getAttribute('id');
    if (!id || !/^[A-Za-z][A-Za-z0-9_-]*$/.test(id)) return false;
    var kept = [];
    try {
      var sheet = new CSSStyleSheet();
      sheet.replaceSync(el.textContent || '');
      Array.prototype.forEach.call(sheet.cssRules, function (r) {
        if (r.type === CSSRule.KEYFRAMES_RULE) { kept.push(r.cssText); return; }
        if (r.type !== CSSRule.STYLE_RULE) return;
        var own = r.selectorText.split(',').every(function (s) { s = s.trim(); return s === '#' + id || s.indexOf('#' + id + ' ') === 0 || s.indexOf('#' + id + '.') === 0 || s.indexOf('#' + id + ':') === 0 || s.indexOf('#' + id + '>') === 0; });
        if (own && !/url\(|image-set|@import/i.test(r.cssText)) kept.push(r.cssText);
      });
    } catch (err) { return false; }
    el.textContent = kept.join('\n');
    return true;
  }
  /* The HTML as a document fragment, made safe. opts.diagram keeps a
     diagram's own scoped stylesheet (keptDiagramStyle). */
  function fragment(html, opts) {
    var t = document.createElement('template');
    t.innerHTML = html == null ? '' : String(html);
    t.content.querySelectorAll(DROPPED).forEach(function (el) { if (!keptFrame(el) && !(opts && opts.diagram && keptDiagramStyle(el))) el.remove(); });
    t.content.querySelectorAll('*').forEach(function (el) {
      if (animatesAddress(el)) { el.remove(); return; }
      Array.prototype.slice.call(el.attributes).forEach(function (a) { if (!attr(a.name, a.value)) el.removeAttribute(a.name); });
      // Document-relative images are inert until the owning host resolves
      // them. Their authored reference stays separate from the display URL.
      if(el.localName==='img'&&el.hasAttribute('src')){
        var source=el.getAttribute('src');
        if(source&&!/^(?:[a-z][a-z0-9+.-]*:|\/)/i.test(source)){
          el.setAttribute('data-attachment',source);el.removeAttribute('src');
        }
      }
      if (el.hasAttribute('style')) {
        var kept = safeStyle(el.getAttribute('style'));
        if (kept) el.setAttribute('style', kept); else el.removeAttribute('style');
      }
      if (el.hasAttribute('data-applet') && NOT_EMBEDDED.indexOf(String(el.getAttribute('data-applet')).toLowerCase()) >= 0) {
        Array.prototype.slice.call(el.attributes).forEach(function (a) { if (/^data-applet/i.test(a.name)) el.removeAttribute(a.name); });
      }
    });
    return t.content;
  }
  // The HTML as a string, made safe.
  function clean(html) {
    var box = document.createElement('div');
    box.appendChild(fragment(html));
    return box.innerHTML;
  }

  window.yavcHTML = Object.freeze({ attr: attr, fragment: fragment, clean: clean, notEmbedded: NOT_EMBEDDED.slice() });
})();
