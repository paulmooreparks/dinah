# Chat Markdown

Dinah draws every message's Markdown with markdown-it and its TeX math with KaTeX. Both are vendored under `cmd/dinah-desktop/frontend/vendor/` and served by Dinah itself. Paul approved the change on 28 October 2026, after the hand-written parser it replaced had shown its faults: nested lists came out flat with the wrong numbers, a `---` rule showed as three dashes, and TeX such as `$\rightarrow$` showed as source. That parser also had no block quotes, no list start numbers, headings only to four hashes, and inline formatting done by regular expressions, which missed strikethrough and bare addresses and got emphasis wrong at the edges.

## What draws a message

The renderer is in `chat-text.js`, and every window that shows messages uses it: a chat (`app.js`), a group chat and a thread (`bus-applets.js`), and a subagent's window (`subagents.js`). `markdown(src)` returns markdown-it's HTML. `nodes(text, decorate)` makes that HTML safe with `safe-html.js`, draws its math if KaTeX is there, lets the window mark more (a group chat marks its mentions), and returns the blocks as nodes without the line breaks markdown-it writes between them. `draw(m)` draws a message as it streams in, and `drawDiagrams` turns a closed d2 or Mermaid fence into its diagram.

Copy and Quote read the message's Markdown as written, never the drawn HTML, so neither changes. Regional times are drawn outside the message text and are not affected. The long-message foot is decided from the text alone and is not affected either.

## The vendored versions

| Library | Version | Files | Integrity of the npm tarball |
| --- | --- | --- | --- |
| markdown-it | 15.0.3 | `vendor/markdown-it-15.0.3/markdown-it.umd.min.js` | `sha512-+Az7Ojz3iRGu8KYMvJ6xVhvrxP3eE+vEtJX75Ow88XBpcQNV/0lGX5ZynIIBftREo4DC2jsveAMDaeltmm8QvQ==` |
| KaTeX | 0.19.1 | `vendor/katex-0.19.1/katex.min.js`, `katex.min.css`, `fonts/` | `sha512-jG8uKntjSEcm2EpQOh0XturpPuuC9r3Sq5HLHdsFdWZT27E672SF19QEi+of0/YeOy15lDgRhKvr5614uRrEfA==` |

Each folder has the package's `LICENSE` as `LICENSE.txt` and a `NOTICE.txt` saying where the files came from. The tarballs came from the npm registry, and each one's SHA-512 matched the registry's integrity value before it was unpacked. Release 15 of markdown-it moved its browser build from `dist/markdown-it.min.js` to `dist/browser/markdown-it.umd.min.js`; the bundle includes linkify-it 6.1, which markdown-it uses for addresses in the text. `index.html` loads markdown-it before `chat-text.js`. KaTeX is not in `index.html`, since it is loaded only when needed (see below).

## How markdown-it is set up

markdown-it runs its default preset, which is CommonMark with GitHub's tables and strikethrough, with `html: false`, `linkify: true` and `typographer: false`. HTML written in a message is shown as text. Two further settings keep the incremental draw honest and the links useful:

- Link reference definitions are disabled, because a definition changes links written before it, so a block already drawn would draw differently once the definition arrived. A definition is shown as the text it is.
- linkify-it makes links only of addresses written with their scheme. Fuzzy links, fuzzy email and IP addresses are off, and the `ftp:`, `mailto:` and `//` schemes are removed, so `README.md` is not taken for a web site. A `file:` scheme is added with a validator that takes the address up to its last character that is not punctuation, as the old chip pattern did.

`md.validateLink` accepts `http`, `https` and `file` addresses, Windows paths such as `C:\x`, and paths with no scheme. It refuses every other scheme, a place on the page (`#x`) and an address beginning with `//`, and markdown-it then shows the Markdown as text. A `javascript:` link is therefore never a link.

## What Dinah adds

Everything Dinah adds is a markdown-it rule on the same parse, or a pass over the drawn nodes. There is no second parser.

- **Links become chips.** The `link_open` rule writes `a.smart-chip` with `href="#"` and the address in `data-chip-url` (web), `data-chip-file` (a `file:` address) or `data-chip-local` (a path, read from the writer's workspace). A local path is turned back from markdown-it's encoded form with `md.normalizeLinkText`, so `<docs/my note.md>` reaches Go with its space. Addresses that linkify found are drawn by the same rule, which replaces the old `chipify` pass. A link to a file or a folder opens where Dinah opens that kind of file, which Go decides, as described under Links to documents below.
- **Pictures become chips.** The `image` rule draws `![alt](src)` as a chip with its description, so a message never loads anything from the web.
- **Tables are wrapped.** `table_open` and `table_close` put the table in `div.msg-table`, which scrolls sideways.
- **Fences say their language and whether they are closed.** The `fence` rule writes `pre` with `data-lang` (the first word of the info string, in lower case) and `data-closed="1"` once the closing fence has arrived, which `drawDiagrams` needs before it draws a d2 or Mermaid block. The closure is read from the token's documented `map` and `content`: a closed fence's lines are its opening line, its content and the closing line, and an open one's are its opening line and its content alone.
- **Math is marked for KaTeX** by an inline rule and a block rule, described next.

Code that reads what the renderer emits was checked: `drawDiagrams` and `subagents.js` read `pre[data-lang][data-closed]`, `openChip` reads the chip attributes (the subagent window now uses it too), the group chat's `markMentions` skips `pre`, `code`, `a` and now `.msg-math`, and `app.js` finds the first visible `p`, `pre` or `li` in `.msg-text`. All of them work unchanged on markdown-it's output.

## Links to documents

A path in a message becomes a link that opens the file or folder inside Dinah, by a pass over the drawn nodes (`linkPaths` in `chat-text.js`) rather than a markdown-it rule. The pass runs in `nodes(text, decorate, cwd)` on each block after the window's own marking, where `cwd` is the folder of the workspace that wrote the message; a window gives it as `m.cwd`, a string or a function. It gives a Markdown link to a path and a `file:` address the tooltip and glyph of where they open, and it wraps a path in inline code or in running text in `a.smart-chip` with the path in `data-chip-path`, when Go says a file or folder is there. It never looks inside `pre`, `a`, `.msg-math` or a mention. Go is asked in batches a moment after a draw, and the answers are kept for ten seconds, so a block drawn while they are on their way is linked when they arrive. The link adds an empty glyph and wraps the text without changing it, so the streamed message still ends as the whole message draws, and Copy and Quote are unaffected. `openChip` sends every reference to a file to `openDocument`, which asks Go where it opens and opens the Editor, the media viewer, Files or the system's program. docs/plans/document-links.md records the whole design, including the order in which Go decides and what each link shows.

## The math rule

Math follows the dollar rule that GitHub and Pandoc use.

- `$...$` is inline math when the character after the opening dollar is not a space, the character before the closing dollar is not a space, and no digit follows the closing dollar. The closing dollar is the next unescaped one, so when that dollar breaks the rule there is no math at all. "costs $5 or $10" and "$PATH" stay text.
- `$$...$$` in a line is display math in that line.
- A block of display math is a line that is `$$...$$`, or a line that is `$$` alone, the TeX, and a line ending in `$$`. A line that begins with `$$` and goes on with more than math is a paragraph, and the inline rule finds its math. A block whose closing line has not arrived runs to the end of the text, or of the list item or quotation it is in, as a fence does, and it is shown as its source and not drawn.
- `\$` is a dollar sign.
- Code is never math. markdown-it reads a code span or a code block before the math rule could see its dollars, and the inline rule refuses math whose TeX would contain a backtick, so a code span that starts inside a pair of dollars stays code.

I wrote the two rules (`mathInline` and `mathBlock` in `chat-text.js`) rather than vendoring a plugin, so that the rule is the one agreed and nothing besides markdown-it and KaTeX is vendored; I did not survey plugins against it. The rules use only markdown-it's documented rule interfaces: `md.inline.ruler.before('escape', ...)`, `md.block.ruler.before('fence', ...)` with the `alt` list that lets the block interrupt a paragraph, a quotation or a list, and the typed fields of `StateInline`, `StateBlock` and `Token`. markdown-it tries inline rules at the characters its text rule stops at, and the dollar is one of those, kept for extensions. The streaming suite would fail at once if a later release stopped doing so.

Math is drawn as its source in an element with `class="msg-math"`, `data-math="inline"` or `"display"`, and the TeX in `data-tex`. `typeset(root)` replaces that source with KaTeX's drawing and marks the element `data-typeset`. KaTeX is called with `throwOnError: false`, `trust: false`, `strict: 'ignore'` and `output: 'htmlAndMathml'`. With no trust, `\href`, `\url` and `\includegraphics` make no link and load nothing, and KaTeX shows them, and any TeX it cannot read, as source in its error colour. That colour is the theme's `--danger`, turned into the hex colour KaTeX's documented option takes by reading it back from a canvas's `fillStyle`, so the error adds no colour of its own.

KaTeX loads lazily. The first time `typeset` finds math, it adds KaTeX's stylesheet and script to the page and keeps the math elements it found; when the script arrives it draws them, whether or not their message is on screen. A chat without math never loads KaTeX. Math drawn before the script arrives shows its source until then.

## What safe-html.js allows

Nothing was added. `safe-html.js` is a deny-list rather than an allow-list. It removes scripts, style sheets outside SVG, frames, embedded objects, forms and the like, every event handler, and every address with a scheme other than the web's (and mail's for a link); it keeps everything else. KaTeX's output is spans with classes and inline styles, an SVG for stretchy symbols and roots, and MathML (`math`, `semantics`, `mrow`, `mi`, `mo`, `mn`, `annotation` and their kin), none of which it removes. The streaming suite checks that a sample covering fractions, roots, arrows, a matrix, colour and text comes out of `dinahHTML.clean` exactly as KaTeX wrote it.

The page's Content Security Policy did not change. KaTeX's script is served from the page's own origin (`script-src 'self'`), its stylesheet and inline styles fall under `style-src 'self' 'unsafe-inline'`, and its fonts under `font-src 'self'`. The streaming suite runs KaTeX under that policy and sees no violation.

## The settle point

A streaming message is drawn incrementally: the text up to the last settled point is drawn once and kept, and only the rest is drawn again on each chunk (docs/design/chat-rendering.md). The settled point now comes from markdown-it's own block structure. `settledLength(text)` parses the text's complete lines (a line still arriving can change what that line is) and looks at the top-level block tokens with a source map. Every top-level block except the last is settled, since a later line can change only the block that is still open. The last block is settled too when nothing can be added to it:

- a heading or a thematic break;
- a fence or a block of math whose closing line has arrived;
- a paragraph, a quotation or a table that a blank line has ended.

A list or an indented code block can continue after blank lines, so while it is last it is never settled. The settled point is the start of the last block's first line, or the end of the complete lines when the last block is settled. Because link reference definitions are off and each top-level block parses the same whatever follows it, the text before the point draws the same alone as within the whole, and the text after it draws as if it began the message. `drawMarkdown` looks for the next settled point only in the text after the kept part, so the work for a chunk does not grow with the message.

## Styles

PUDL's `pudl.css` has heading sizes for `h1` to `h4` and nothing for block quotes or rules, so `app.css` adds rules for `.msg-text`. Headings use PUDL's type scale a step or more below a page's (`--text-xl`, `--text-lg`, then `--text-base`), and levels five and six take PUDL's heading weight, with six in `--text-muted`. A block quote and a rule are drawn as a quotation in a card already is (`dinah.css`), with `--border`. Nested lists have a narrower indent, and display math scrolls sideways. No colour was added.

## Tests

`e2e/chat-stream-markdown.js` serves the page under the Content Security Policy read from `shell_server.go` and fails on any violation. It streams a message five characters at a time and compares it with a whole render after every chunk; the message has nested ordered and unordered lists with a start number, a loose list, a thematic break, a block quote with a list in it, inline and display math, "costs $5 or $10", "$PATH", math in a code span and in a fence, and a six-hash heading. It checks what was drawn, that KaTeX's fonts load, that KaTeX is not loaded for a message without math, that safe-html keeps KaTeX's output, that `\href{javascript:...}`, `\url` and `\includegraphics` make no link and fetch nothing, that bad TeX shows as source, and that a half-streamed Mermaid fence is not drawn until it closes. The earlier checks of kept nodes and of bounded work per chunk still run.
