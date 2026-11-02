/* Text that holds the control sequences a terminal reads (ECMA-48), in one
   place for every page that shows it (FE-16): a command's output in a chat
   or a subagent's row, and the Terminal's own screen. Each pattern below is
   the only copy; the frontend policy test finds a regular expression for an
   escape sequence anywhere else.

     strip(text)  every escape sequence removed, other characters kept; the
                  Terminal's piped and saved text.
     plain(text)  strip, and every control character but tab and new line
                  dropped; a subagent's tool output.
     html(text)   plain, with the colour and weight sequences (SGR) drawn as
                  styled spans and the text escaped; a chat's tool output.
     safe(text)   for the Terminal's screen: escape shown as a visible
                  symbol, invisible marks as their code points, other
                  control characters dropped.
     inert(text)  for the Terminal's screen: as safe, but the SGR colour and
                  emphasis sequences the Terminal uses pass. */
(function () {
  'use strict';

  /* One control sequence: a CSI sequence (ESC [ parameters final), an OSC
     string ended by BEL or ST, or any other ESC with the character after it.
     The CSI parameters and final byte are captured for html. */
  var SEQUENCE = /\x1b\[([0-9;?]*)([@-~])|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-_]?/g;
  var CONTROLS = /[\x00-\x08\x0b-\x1f\x7f]/g;

  function strip(text) { return String(text == null ? '' : text).replace(SEQUENCE, ''); }
  function plain(text) { return strip(text).replace(CONTROLS, ''); }

  /* PowerShell 7 colours a table's header with ESC[32m, for one. The chat
     draws the colour and weight sequences as styled text and leaves the rest
     out (other CSI sequences, OSC titles and links, stray ESC and other C0
     controls but tab and new line), so the reader sees the output as a
     terminal would show it. */
  var ANSI_FG = [30, 31, 32, 33, 34, 35, 36, 37, 90, 91, 92, 93, 94, 95, 96, 97];
  var ANY = new RegExp(SEQUENCE.source + '|' + CONTROLS.source, 'g');
  function html(value) {
    var text = String(value == null ? '' : value);
    var out = '', open = false, style = { bold: false, dim: false, italic: false, under: false, fg: null, bg: null };
    var esc = window.dinahHTML.text;
    function apply(params) {
      var p = params.length ? params.split(';').map(function (n) { return n === '' ? 0 : Number(n); }) : [0];
      for (var i = 0; i < p.length; i++) {
        var n = p[i];
        if (n === 0) style = { bold: false, dim: false, italic: false, under: false, fg: null, bg: null };
        else if (n === 1) style.bold = true;
        else if (n === 2) style.dim = true;
        else if (n === 3) style.italic = true;
        else if (n === 4) style.under = true;
        else if (n === 22) { style.bold = false; style.dim = false; }
        else if (n === 23) style.italic = false;
        else if (n === 24) style.under = false;
        else if (n === 39) style.fg = null;
        else if (n === 49) style.bg = null;
        else if (ANSI_FG.indexOf(n) >= 0) style.fg = ANSI_FG.indexOf(n);
        else if (ANSI_FG.indexOf(n - 10) >= 0) style.bg = ANSI_FG.indexOf(n - 10);
        // 256-colour and true-colour forms take their own parameters; the
        // chat keeps to the sixteen colours and skips them.
        else if (n === 38 || n === 48) i += p[i + 1] === 5 ? 2 : p[i + 1] === 2 ? 4 : 1;
      }
    }
    function span() {
      var cls = [];
      if (style.bold) cls.push('ansi-b');
      if (style.dim) cls.push('ansi-d');
      if (style.italic) cls.push('ansi-i');
      if (style.under) cls.push('ansi-u');
      if (style.fg !== null) cls.push('ansi-fg' + style.fg);
      if (style.bg !== null) cls.push('ansi-bg' + style.bg);
      if (open) out += '</span>';
      open = cls.length > 0;
      if (open) out += '<span class="' + cls.join(' ') + '">';
    }
    var re = new RegExp(ANY.source, 'g'), last = 0, m;
    while ((m = re.exec(text))) {
      out += esc(text.slice(last, m.index));
      last = re.lastIndex;
      if (m[2] === 'm' && m[1].indexOf('?') < 0) { apply(m[1]); span(); }
    }
    out += esc(text.slice(last));
    if (open) out += '</span>';
    return out;
  }

  /* Marks that take no room on the screen but change what the reader sees:
     zero-width characters, the left-to-right and right-to-left marks, line
     and paragraph separators, the byte order mark, and the directional
     embeddings, overrides and isolates. Each is shown as its code point,
     such as <U+200B>, so text cannot hide a character or reorder itself. */
  var MARKS = /[\u200b-\u200f\u2028\u2029\u202a-\u202e\u2066-\u2069\ufeff]/g;
  function marks(s) { return s.replace(MARKS, function (c) { return '<U+' + c.charCodeAt(0).toString(16).toUpperCase().padStart(4, '0') + '>'; }); }
  /* Text that comes from a document or a person, which must not reach the
     Terminal's screen as control sequences: escape becomes a visible symbol,
     invisible marks are shown as their code points, and other control
     characters are dropped. */
  function safe(value) {
    return marks(String(value == null ? '' : value).replace(/\r\n?/g, '\n').replace(/\x1b/g, '\u241b')
      .replace(/[\x00-\x08\x0b-\x1f\x7f-\x9f]/g, ''));
  }
  /* Text on its way to the Terminal's screen as its output, and the saved
     transcript when it is shown again. Only the colour and emphasis
     sequences of Select Graphic Rendition that the Terminal and scripts use
     pass: reset, bold, dim, italic, underline, inverse, their resets, and the
     sixteen colours. Any other escape is shown as the visible symbol safe()
     uses, invisible marks are shown as their code points, and every other
     control character is dropped apart from line breaks and tabs. This is
     the last barrier: whichever path text took, it cannot move the cursor,
     rewrite an earlier line, set the title or the clipboard, or hide
     itself. */
  var SGR = /^\x1b\[([\d;]{0,48})m/, STYLE = /^(0|1|2|3|4|7|22|23|24|27|3[0-7]|39|4[0-7]|49|9[0-7]|10[0-7])$/;
  function inert(value) {
    var s = String(value == null ? '' : value).replace(/\r\n?/g, '\n'), out = '', i = 0;
    while (i < s.length) {
      var at = s.indexOf('\x1b', i);
      if (at < 0) { out += s.slice(i); break; }
      out += s.slice(i, at);
      var m = SGR.exec(s.slice(at, at + 52));
      if (m && m[1].split(';').every(function (p) { return STYLE.test(String(+p)); })) { out += m[0]; i = at + m[0].length; }
      else { out += '\u241b'; i = at + 1; }
    }
    return marks(out.replace(/[\x00-\x08\x0b-\x1a\x1c-\x1f\x7f-\x9f]/g, ''));
  }

  window.dinahTerminalText = { strip: strip, plain: plain, html: html, safe: safe, inert: inert };
})();
