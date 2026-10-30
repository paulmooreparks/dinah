# Proposal: How PUDL Presents a Diff

Dinah asks PUDL for a shared contract for showing the difference between two texts: the parts of a diff, the classes and attributes that mark them, the tokens they are drawn with, how lines are chosen from the keyboard, and how a diff renders in forced colours. This is a proposal for Paul to hand off; nothing in PUDL is changed by it.

## The Need

Dinah's Editor gained a diff view on 25 October 2026 (`cmd/dinah-desktop/frontend/diff-applet.js` and `diff-applet.css`), built on CodeMirror's merge view. One viewer serves the Git extension, reviews and agents' suggested edits: whoever opens a diff hands it the actions to show for the whole file, for a hunk and for chosen lines, and each press goes back to the opener with the lines it applies to. It draws inline and side by side, with line numbers, a sign column, a sticky strip above each hunk, tick boxes for choosing lines, and added and removed lines tinted with `positive` and `danger` at 14 percent.

PUDL said on 25 October that it has no diff component, hunk-selection interface or diff colours, that the 14 percent tints are Dinah's choice, and that it would like a proposal covering gutters, additions and deletions, unmatched regions, hunk headings, selection and forced-colour rendering. On 28 October it added that any code-review or document-compare view needs this, and that Git is only one of them. A web page reviewing a pull request, a document-compare view in any PUDL application, and an agent's proposed edit in a chat all draw the same parts, and today each would draw them differently.

## What Is Asked

PUDL would define a diff's parts and their markup, add a small set of derived tokens, and state the keyboard, accessibility and forced-colour rules, in a new section of chapter 8. A server-rendered, read-only diff would then be plain markup that `pudl.css` draws with no script. An editor-based diff such as Dinah's, which cannot put PUDL's markup inside CodeMirror's own, would map its classes onto the same tokens and meet the same rules. Neither needs PUDL to ship a diff engine or a script.

## The Parts

| Part | What it is |
|---|---|
| Diff | The whole comparison, in one of two layouts: inline, with removed lines above the lines that replace them, or side by side, with the before text at the start edge and the after text at the end edge |
| Line | One line of either text, of one of four kinds: unchanged, added, removed, or an unmatched region |
| Unmatched region | The space on one side of a side-by-side diff opposite lines that exist only on the other side, so that the two sides stay aligned |
| Gutter | The columns before a line's text: its number in the before text, its number in the after text (inline shows both, side by side shows one per side), its sign, and its choice box when lines can be chosen |
| Sign | The mark for an added or removed line, drawn as PUDL's `plus` or `minus` glyph in the line's colour |
| Changed words | The part of a changed line that differs from its counterpart, marked within the line |
| Hunk heading | A strip above each run of changes, with the range in unified-diff form, its place ("Hunk 2 of 5") and the hunk's actions |
| Folded lines | A band standing for a run of unchanged lines that are not shown, with a button to show them |
| Summary | The count of hunks, added lines and removed lines, as words in the diff's head |

## The Markup

This is a read-only diff as `pudl.css` would draw it, with no script.

```html
<section class="diff" data-diff-layout="inline" aria-label="Changes to internal/store/store.go">
  <p class="diff-summary">3 hunks, 12 lines added, 4 removed</p>
  <div class="diff-hunk" role="group" aria-labelledby="h2">
    <div class="diff-hunk-head" id="h2">
      <code>@@ -40,6 +40,8 @@</code> <span>Hunk 2 of 3</span>
      <span class="diff-hunk-actions"><button class="btn btn-sm" type="button">Stage Hunk</button></span>
    </div>
    <div class="diff-line" data-diff="same"><span class="diff-no">40</span><span class="diff-no">40</span><span class="diff-sign"></span><code class="diff-text">func open() {</code></div>
    <div class="diff-line" data-diff="del" role="deletion"><span class="diff-no">41</span><span class="diff-no"></span><span class="diff-sign"><span class="visually-hidden">Removed</span></span><code class="diff-text">  return <del class="diff-word">nil</del></code></div>
    <div class="diff-line" data-diff="add" role="insertion"><span class="diff-no"></span><span class="diff-no">41</span><span class="diff-sign"><span class="visually-hidden">Added</span></span><code class="diff-text">  return <ins class="diff-word">err</ins></code></div>
  </div>
  <div class="diff-fold"><button class="btn btn-sm" type="button">Show 42 unchanged lines</button></div>
</section>
```

`data-diff` is `same`, `add`, `del` or `pad`, the last for an unmatched region, which is hidden from assistive technology. `insertion` and `deletion` are ARIA 1.2 roles, and `ins` and `del` are HTML's own elements for changed text, so the kind of a line or a word is stated in the markup as well as drawn. Where lines can be chosen, the gutter gains a `span.diff-choose` holding a checkbox, and a chosen line has `aria-checked="true"` on that box. An editor-based diff marks the same parts with its own classes and meets the same rules, as the section on Dinah below shows.

## The Tokens

I propose four derived tokens, with their own names so a theme can point them elsewhere, and defaults taken from the status colours.

| Token | Default formula | Use |
|---|---|---|
| `diff-add` | `"positive"` | The sign of an added line, and the underline of added words |
| `diff-del` | `"danger"` | The sign of a removed line, and the underline of removed words |
| `diff-add-bg` | `{ "fade": "diff-add", "by": 0.10 }` | The fill of an added line and its gutter |
| `diff-del-bg` | `{ "fade": "diff-del", "by": 0.10 }` | The fill of a removed line and its gutter |

The hunk heading takes `surface-alt` with `border` above and below and its range in `mono` and `accent`, as Dinah's does. An unmatched region is hatched at 45 degrees with `border` at half strength, and folded lines are a band of `surface-alt` in `text-muted`. Neither needs a token of its own.

The amount is 10 percent because of the syntax rule in chapter 3, which says each syntax colour reaches 4.5:1 on `input-bg`. A diff's text is code on an editor's surface, and the tint lowers that contrast. I measured the worst syntax colour on each tint over `input-bg`:

| Tint | Light, added | Light, removed | Dark, added | Dark, removed |
|---|---|---|---|---|
| 14 percent (Dinah today) | 5.31 | 5.05 | 4.32 | 4.68 |
| 12 percent | 5.44 | 5.24 | 4.50 | 4.80 |
| 10 percent | 5.60 | 5.41 | 4.62 | 4.92 |

The worst colour in every case is `syntax-comment`. Dinah's 14 percent breaks the rule for comments on added lines in the dark theme, 12 percent passes with nothing to spare, and 10 percent passes with some margin. At 10 percent the signs reach 4.43 (added) and 5.58 (removed) against their tints in the light theme and 4.63 and 3.60 in the dark one, which clears the 3:1 that chapter 6 asks of anything that conveys a state.

The tint itself is only 1.12 to 1.19 against the untinted surface, which is far below 3:1. That is acceptable only because the sign states the line's kind, so the rule must be that the sign column is always drawn, in both layouts. Changed words are marked by a 2px underline in `diff-add` or `diff-del` at full strength. Dinah's 45 percent underline is fainter than it needs to be, and a filled background at that strength would take every syntax colour below 4.5:1 (2.4 to 3.4 in my measurements).

## Choosing Lines and the Keyboard

- Next Change and Previous Change move the caret to the first line of the next or previous hunk, scroll it into view below its sticky heading, and announce the hunk's heading and its counts in a polite live region. Dinah binds them to F7 and Shift+F7 and puts them on the diff's toolbar.
- A hunk heading's actions are ordinary buttons in the Tab order, each with an accessible name that includes the hunk's number, such as "Stage hunk 2".
- The choice boxes form one stop in the Tab order, as chapter 6 asks of any set of items. Up and Down move between the boxes of changed lines, Space toggles one, and Shift with Up or Down extends the choice. A pointer may also Shift-click to choose a run.
- A chosen line is marked by its ticked box and by a 3px edge in `accent` on its start side, as a current tree row is, so the choice does not rest on the box alone.
- Actions on chosen lines sit at the diff's foot, with the count in their words ("Stage 3 Lines"), and are disabled until a line is chosen.
- Selecting and copying text takes the text alone. Gutters, signs, hunk headings and folded bands are not selectable. In a side-by-side diff a selection stays within one side.

## Accessibility

- Each side of a side-by-side diff is a region labelled "Before" or "After" with the file's name, and an inline diff has one label for the whole.
- Every changed line exposes its kind, by role and by a visually hidden word, so a screen reader hears "Removed" or "Added" before the text and does not have to infer it from a colour it cannot see.
- A screen reader hears one line number per line. In an inline diff it is the number from the side the line belongs to.
- The summary is text in the head, and it changes when the texts do.
- In a right-to-left page the gutters and the two sides mirror as every other component does. The code itself keeps its own direction.

## Forced Colours

In forced colours the tints disappear, and PUDL should state what replaces them so every implementation draws the same thing.

- The signs keep painting, as chapter 6 requires of every glyph, in `CanvasText`.
- An added line gains a 2px solid rule on its start edge, and a removed line a 2px dashed rule, both in `CanvasText`, so the kinds can be scanned down the edge.
- Added words are underlined and removed words are struck through, in `CanvasText`, since text decoration survives the system's colours where a fill does not.
- An unmatched region keeps its hatch, drawn in `GrayText`.
- A chosen line takes `Highlight` for its edge, as every selected state does in chapter 6.
- Hunk headings and the folded band keep their borders, and their buttons keep theirs.

All of these use system colour keywords and properties that CSS Color 4 and the forced-colours media query define, so nothing rests on how one browser happens to treat an SVG fill or a background in that mode.

## What Stays With the Application

- The application computes the diff and decides where its hunks begin and end. Dinah's hunks are CodeMirror's chunks, which differ from Git's, and the Git extension maps between them by line number.
- The application decides which actions a file, a hunk and chosen lines offer, what they do, and whether they ask for confirmation.
- The application highlights the syntax, following chapter 8's code roles as it already does.
- The application chooses the layout a reader starts in and remembers the last one chosen.
- Three-way views for resolving conflicts stay with the application for now. They need more parts than this proposal covers and can follow once two-way diffs are settled.

## Questions for PUDL

1. Should added and removed draw from `positive` and `danger` at all? Red and green for removal and addition is what every diff tool has taught people, but PUDL reserves those colours for status. Separate tokens with status defaults, as above, keep the convention and let a theme re-point them, for example to blue and orange for readers with red-green colour blindness, as some code hosts offer.
2. Is 10 percent right, or should the rule be stated as "the strongest tint at which every syntax colour still reaches 4.5:1 on `input-bg`", so that a theme with different syntax colours gets its own amount?
3. Should PUDL ship the read-only `.diff` markup and its CSS, or only the contract and tokens for applications to meet? I would like both, since a server-rendered review page needs the markup and an editor-based view needs only the tokens and rules.
4. Is the sign column's `plus` and `minus` the right pair of glyphs? Their documented meanings are "add one" and "take one away", which are actions, and here they mark a state.
5. Should the choice boxes use PUDL's checkbox at a smaller size, or a glyph that toggles? Dinah draws native checkboxes at 12px, which is below the 24px minimum that chapter 6 cites. WCAG 2.2 allows a smaller target only when enough space surrounds it or when a full-size control elsewhere does the same thing, and I am not sure either is true of a column of boxes one text line apart.

## How Dinah Would Adopt It

Dinah's diff view already has most of the parts. Adopting the contract would mean these changes:

- The tints would move from 14 to 10 percent through `--diff-add-bg` and `--diff-del-bg`, and the word underline would take `--diff-add` and `--diff-del` at full strength.
- A forced-colours block in `diff-applet.css` would add the edge rules, the text decorations and the hatch as listed above.
- The sign column would draw the `plus` and `minus` glyphs. It now writes "+" and "-" as characters, which chapter 5's rule against font characters does not allow.
- Changed lines would expose their kind to assistive technology. CodeMirror's merge view marks them only with classes, so Dinah would add the words through line decorations.
- The tick boxes would become one stop in the Tab order with arrow keys, and a chosen line would gain its accent edge.
- The Git extension and the review and suggestion callers would need no change, since the diff's actions and the specification it is opened with stay as they are.
