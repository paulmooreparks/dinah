/* HTML that Dinah did not write itself is made safe here before it is put
   on the page: an agent's message drawn from its Markdown, a document's
   preview (which keeps the HTML written in the document), and HTML pasted
   into Page view. The page's Content Security Policy (shell_server.go)
   already runs no inline script; this keeps the markup clean as well, so
   that nothing depends on the policy alone.

   An element that holds code, a frame or a form is left out with its
   content. Every event handler is removed, and so is every address whose
   scheme is not the web's (or, for a link, mail's), except a picture's data
   address. An address with no scheme, such as a place on the page or
   another document, is kept. An SVG animation that would set a link's
   address is left out, since it could set one with any scheme. The HTML is
   parsed into a template element, whose content runs no script and loads
   nothing until it is placed in the page. */
(function () {
  'use strict';
  var DROPPED = 'script,style:not(svg style),iframe,frame,frameset,object,embed,applet,noscript,template,link,meta,base,form,portal';

  // Whether an attribute may stay, by its name and value.
  function attr(name, value) {
    name = String(name).toLowerCase();
    if (/^on/.test(name) || name === 'srcdoc' || name === 'formaction' || name === 'action') return false;
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
  // The HTML as a document fragment, made safe.
  function fragment(html) {
    var t = document.createElement('template');
    t.innerHTML = html == null ? '' : String(html);
    t.content.querySelectorAll(DROPPED).forEach(function (el) { el.remove(); });
    t.content.querySelectorAll('*').forEach(function (el) {
      if (animatesAddress(el)) { el.remove(); return; }
      Array.prototype.slice.call(el.attributes).forEach(function (a) { if (!attr(a.name, a.value)) el.removeAttribute(a.name); });
    });
    return t.content;
  }
  // The HTML as a string, made safe.
  function clean(html) {
    var box = document.createElement('div');
    box.appendChild(fragment(html));
    return box.innerHTML;
  }

  window.dinahHTML = Object.freeze({ attr: attr, fragment: fragment, clean: clean });
})();
