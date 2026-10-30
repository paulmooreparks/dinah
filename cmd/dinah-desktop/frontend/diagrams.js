/* Shared, source-preserving diagram rendering for chats and previews. */
(function () {
  'use strict';
  var mermaidReady, serial = 0, cache = new Map();
  var svgNS = 'http://www.w3.org/2000/svg', htmlNS = 'http://www.w3.org/1999/xhtml';
  var svgTags = new Set('svg g defs path text tspan textPath rect circle ellipse line polyline polygon marker pattern mask clipPath linearGradient radialGradient stop title desc style foreignObject use image filter feGaussianBlur feOffset feBlend feColorMatrix feComposite feFlood feMerge feMergeNode'.split(' '));
  var htmlTags = new Set('div span p b strong i em br sub sup ul ol li table tbody tr td th code pre'.split(' '));
  var cssProperties = new Set(('fill fill-opacity fill-rule stroke stroke-width stroke-opacity stroke-dasharray stroke-dashoffset stroke-linecap stroke-linejoin stroke-miterlimit opacity color background-color font font-family font-size font-weight font-style font-variant text-anchor text-decoration text-transform dominant-baseline alignment-baseline baseline-shift letter-spacing word-spacing line-height white-space word-break overflow overflow-wrap display visibility width height min-width min-height max-width max-height text-align vertical-align padding padding-top padding-right padding-bottom padding-left margin margin-top margin-right margin-bottom margin-left border border-width border-style border-color border-radius box-sizing filter clip-path mask marker-start marker-mid marker-end').split(' '));
  function safeValue(value) {
    if (/[\\@<>]|\/\*|expression\s*\(/i.test(value)) return false;
    var functions = value.replace(/url\(\s*["']?#[\w:.-]+["']?\s*\)/gi, '').match(/[\w-]+\s*\(/g) || [];
    return functions.every(function (name) { return /^(rgb|rgba|hsl|hsla|lab|lch|oklab|oklch|color|calc|min|max|clamp|drop-shadow|blur|brightness|contrast|grayscale|hue-rotate|invert|opacity|saturate|sepia|matrix|matrix3d|translate|translateX|translateY|scale|scaleX|scaleY|rotate|skewX|skewY)\s*\($/i.test(name); });
  }
  // A font the SVG brings inside itself, as d2 does with the fonts it measured
  // its labels in. Only a font in a data URL is kept, so nothing is fetched.
  var embeddedFont = /^url\(\s*"?data:(font\/(woff2?|ttf|otf|sfnt)|application\/(font-woff2?|x-font-woff|font-sfnt|x-font-ttf|x-font-opentype));base64,[A-Za-z0-9+\/=]+"?\s*\)(\s*format\(\s*"?[\w-]+"?\s*\))?$/;
  function familyName(value) { return value.trim().replace(/^(["'])(.*)\1$/, '$2'); }
  // An embedded font is given a name of the drawing's own, and the drawing's
  // rules use that name, so a font an SVG brings never replaces one the page
  // uses under the same name.
  function renameFamilies(value, fonts) {
    return value.split(',').map(function (part) { var own = fonts[familyName(part)]; return own ? '"' + own + '"' : part.trim(); }).join(', ');
  }
  function declarations(style, fonts) {
    var output = [];
    Array.from(style).forEach(function (property) {
      var value = style.getPropertyValue(property);
      if (property === 'font-family' && fonts) value = renameFamilies(value, fonts);
      if (cssProperties.has(property) && safeValue(value)) output.push(property + ':' + value + (style.getPropertyPriority(property) ? ' !important' : ''));
    });
    return output.join(';');
  }
  function parsed(text) {
    // Constructed stylesheets are parsed without attaching them to the page.
    var sheet = new CSSStyleSheet(); sheet.replaceSync(text);
    return Array.from(sheet.cssRules);
  }
  function fontFace(rule, fonts) {
    var own = fonts[familyName(rule.style.getPropertyValue('font-family'))], src = rule.style.getPropertyValue('src').trim();
    if (!own || !embeddedFont.test(src)) return '';
    var face = ['font-family:"' + own + '"', 'src:' + src];
    ['font-weight', 'font-style'].forEach(function (property) { var value = rule.style.getPropertyValue(property); if (value && /^[\w\s-]+$/.test(value)) face.push(property + ':' + value); });
    return '@font-face{' + face.join(';') + '}';
  }
  function stylesheet(text, id, fonts) {
    return parsed(text).map(function (rule) {
      if (rule.type === CSSRule.FONT_FACE_RULE) return fontFace(rule, fonts);
      if (rule.type !== CSSRule.STYLE_RULE || /[\\@<>]/.test(rule.selectorText)) return '';
      var body = declarations(rule.style, fonts), scope = '#' + CSS.escape(id);
      return body ? scope + ':is(' + rule.selectorText + '),' + scope + ' :is(' + rule.selectorText + '){' + body + '}' : '';
    }).filter(Boolean).join('\n');
  }
  // The embedded fonts of every style element in the drawing, each with the
  // name it is given.
  function embeddedFonts(svg) {
    var fonts = {}, count = 0;
    svg.querySelectorAll('style').forEach(function (element) {
      parsed(element.textContent).forEach(function (rule) {
        if (rule.type !== CSSRule.FONT_FACE_RULE || !embeddedFont.test(rule.style.getPropertyValue('src').trim())) return;
        var name = familyName(rule.style.getPropertyValue('font-family'));
        if (name && !fonts[name]) fonts[name] = svg.id + '-font-' + (++count);
      });
    });
    return fonts;
  }
  function sanitize(svgText) {
    var parsed = new DOMParser().parseFromString(svgText, 'image/svg+xml'), svg = parsed.documentElement;
    if (parsed.querySelector('parsererror') || svg.localName !== 'svg' || svg.namespaceURI !== svgNS) throw new Error('The renderer returned an invalid SVG.');
    if (!svg.id) svg.id = 'dinah-diagram-root-' + (++serial);
    var fonts = embeddedFonts(svg);
    [svg].concat(Array.from(svg.querySelectorAll('*'))).forEach(function (element) {
      if (!svg.contains(element) && element !== svg) return;
      var allowed = element.namespaceURI === svgNS ? svgTags.has(element.localName) : element.namespaceURI === htmlNS && htmlTags.has(element.localName) && !!element.closest('foreignObject');
      if (!allowed) { element.remove(); return; }
      element.removeAttribute('data-dinah-svg');
      Array.from(element.attributes).forEach(function (attribute) {
        var name = attribute.localName.toLowerCase(), value = attribute.value;
        if (name.startsWith('on') || ['src','srcset','poster','formaction','action','base'].includes(name) || name === 'href' && !/^#[\w:.-]+$/.test(value) || name !== 'style' && !safeValue(value)) element.removeAttributeNode(attribute);
      });
      if (element.hasAttribute('style')) {
        var style = document.createElement('span').style; style.cssText = element.getAttribute('style');
        var cleaned = declarations(style, fonts); if (cleaned) element.setAttribute('style', cleaned); else element.removeAttribute('style');
      }
      if (element.localName === 'style') element.textContent = stylesheet(element.textContent, svg.id, fonts);
    });
    return new XMLSerializer().serializeToString(svg);
  }
  function mermaid() {
    if (mermaidReady) return mermaidReady;
    mermaidReady = new Promise(function (resolve, reject) {
      var script = document.createElement('script'); script.src = 'vendor/mermaid.min.js';
      script.onload = function () {
        window.mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'default', suppressErrorRendering: true });
        resolve(window.mermaid);
      };
      script.onerror = function () { mermaidReady = null; script.remove(); reject(new Error('The Mermaid library could not be loaded.')); };
      document.head.appendChild(script);
    });
    return mermaidReady;
  }
  function render(language, source, retry) {
    var key = language + '\n' + source, pending = cache.get(key);
    if (pending && !(retry && pending.failed)) return pending;
    pending = Promise.resolve().then(function () {
      if (language === 'd2') return window.dinahHelpers.app().RenderD2(source);
      if (language === 'mermaid') return mermaid().then(function (m) { return m.render('dinah-diagram-' + (++serial), source); }).then(function (result) { return result.svg; });
      throw new Error('Unsupported diagram language.');
    }).then(function (svg) { return { svg: sanitize(svg) }; }, function (error) { return { error: String(error && error.message || error) }; }).catch(function (error) { return { error: String(error && error.message || error) }; });
    pending = pending.then(function (result) { pending.failed = result.error !== undefined; return result; });
    cache.set(key, pending);
    if (cache.size > 128) cache.delete(cache.keys().next().value);
    return pending;
  }
  function place(target, language, source, result) {
    var figure = document.createElement('figure'); figure.className = 'msg-diagram';
    var pre = document.createElement('pre'), code = document.createElement('code'); code.textContent = source; pre.appendChild(code);
    if (result.svg !== undefined) {
      var drawing = document.createElement('div'); drawing.className = 'msg-diagram-svg'; drawing.innerHTML = result.svg;
      var details = document.createElement('details'), summary = document.createElement('summary');
      summary.textContent = 'The ' + (language === 'd2' ? 'd2' : 'Mermaid') + ' source'; details.append(summary, pre); figure.append(drawing, details);
      window.dinahWorkbench.namespaceSVG(figure);
      var frame = document.createElement('div'), expand = document.createElement('button');
      frame.className = 'media-frame'; expand.type = 'button'; expand.className = 'icon-btn media-expand';
      expand.innerHTML = '<span class="glyph" style="--glyph:var(--glyph-search)" aria-hidden="true"></span>';
      expand.setAttribute('aria-label', 'Expand ' + (language === 'd2' ? 'D2' : 'Mermaid') + ' Diagram');
      expand.title = expand.getAttribute('aria-label');
      expand.addEventListener('click', function () { window.dinahMediaViewer.open(drawing.querySelector('svg'), expand, (language === 'd2' ? 'D2' : 'Mermaid') + ' Diagram'); });
      drawing.replaceWith(frame); frame.append(drawing, expand);
    } else {
      var error = document.createElement('p'); error.className = 'msg-diagram-error';
      error.textContent = 'The ' + (language === 'd2' ? 'd2' : 'Mermaid') + ' diagram could not be drawn: ' + result.error;
      figure.append(error, pre);
    }
    target.replaceWith(figure); return figure;
  }
  function draw(target, language, source, isCurrent, changed, retry) {
    return render(language, source, retry).then(function (result) {
      if (!target.isConnected || isCurrent && !isCurrent()) return;
      place(target, language, source, result); if (changed) changed(result);
    });
  }
  window.dinahDiagrams = { draw: draw, sanitize: sanitize };
})();
