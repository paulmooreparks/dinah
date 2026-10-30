# Proposal: A Categorical Palette for PUDL

Dinah asks PUDL for a set of eight colours that tell things apart without saying anything about their state, as palette tokens in the specification beside the status and syntax colours. This is a proposal for Paul to hand off; nothing in PUDL is changed by it.

## The Need

PUDL's colours all have jobs. The accent marks the primary action, where the reader is and focus; `warn`, `danger` and `positive` are status; the ten syntax colours belong to highlighted code. Nothing is left for the common case where an interface only needs to show that two things are different, such as two lines on a chart, two tags, or two branches in a commit graph.

PUDL Development said on 25 October 2026 that the status tokens must not colour arbitrary lanes, because a red lane would read as a failing branch, and that PUDL has no categorical palette and one should be proposed. On 28 October it asked for this one first, as the most general of the three Dinah owes it, since charts, tags and history lanes all need colours that stay distinct from each other and pass contrast in both themes.

The uses I know of today are these.

- The Git extension's commit graph draws one lane per line of history. Until PUDL has a palette, its lanes use five colours of its own mixed from the accent and the text (`dinah-git`, `web/common.js`). Its first lane is the accent itself, and its third and fourth lanes are only 3.4 apart in the light theme and 2.6 in the dark one, on the distance scale defined under the evidence below, so the eye cannot follow them.
- Dinah renders Mermaid diagrams in chats and previews with Mermaid's own default theme, so a pie chart, a Gantt chart or a `gitGraph` arrives in colours that ignore the reader's theme. Mermaid takes eight branch colours (`git0` to `git7`) and twelve pie colours as theme variables, and the palette would give Dinah something principled to pass it.
- Group chats in Dinah could give each member chat a colour in the group window, to help the eye follow who said what.
- Any PUDL application with a chart, a tag colour, a calendar's categories or a board's swimlanes faces the same choice and today makes it alone.

## What Is Asked

Dinah asks for eight palette tokens, `category-1` to `category-8`, set in both `palette.light.tokens.json` and `palette.dark.tokens.json` like every other palette token, with a fixed order and rules a theme must keep when it sets them. The order matters because consumers assign slots in sequence, so the first few slots are the ones most often seen next to each other.

The rules I propose for chapter 3, in the same form as the syntax rule:

- Each category colour reaches 3:1 against `bg`, `surface` and `input-bg`, which WCAG 2.2 asks of a graphical object that conveys information.
- Each category colour stays clearly apart from `warn`, `danger` and `positive`, at a distance in OKLab of at least 12 (the scale is defined under the evidence below), and its hue sits at least 20 degrees from each of theirs, so that a category colour is never mistaken for a status.
- Each category colour stays at least 8 from `accent` in the same measure.
- Neighbouring slots stay at least 15 apart for normal vision and at least 8 apart under simulated protanopia and deuteranopia.
- A category colour is never the only way a thing is identified. A lane has its position and its drawn topology, a chart series has its legend or a direct label, and a tag has its words.

Category colours are meant for marks such as lines, dots, swatches, bars and the edge of a chip. Words beside a mark stay in `text` or `text-muted`, since several slots fall short of 4.5:1 against the surfaces and a coloured word reads like a link or a status.

## The Proposed Values

These values are a first draft for PUDL to adjust, found by search against PUDL's default palettes so that they pass every rule above. The hue families keep away from the hues of `danger`, `warn` and `positive` and lean on blue, teal, violet, magenta and olive, with lightness varied between neighbours. The nearest approach is the raspberry of slot 8, about 30 degrees of hue from `danger`, which the questions below return to.

| Token | Family | Light | Dark |
|---|---|---|---|
| `category-1` | Blue | `#4e5efe` | `#4368fd` |
| `category-2` | Teal | `#149d99` | `#07d5d1` |
| `category-3` | Plum | `#83296b` | `#f558d8` |
| `category-4` | Olive | `#769805` | `#68863e` |
| `category-5` | Steel blue | `#106e9a` | `#1280a5` |
| `category-6` | Purple | `#7400bb` | `#ab40df` |
| `category-7` | Lavender | `#8483ca` | `#a19efd` |
| `category-8` | Raspberry | `#ce1a6f` | `#dd1e78` |

The dark values are a selection in their own right rather than the light ones inverted. Each keeps its hue family, and its lightness is set for the dark surfaces.

## The Evidence

Contrast is the WCAG ratio PUDL uses in chapter 6. `input-bg` is computed from `derived.json`, which gives `#fefefe` in light (about `#ffffff`) and `#282c30` in dark. Distance is the Euclidean distance between two colours in OKLab, multiplied by 100. As a sense of scale, PUDL's own `warn` and `danger` are 14.7 apart in the light theme and 12.5 apart in the dark one, and `positive` and `accent` are 17.9 and 15.9 apart. Colour-vision simulation uses the Machado, Oliveira and Fernandes model of 2009 at full severity.

In the light theme:

| Slot | `bg` | `surface` | `surface-alt` | `input-bg` | Nearest status | From `accent` |
|---|---|---|---|---|---|---|
| 1 | 4.38 | 4.66 | 3.89 | 4.82 | `positive`, 30 | 20 |
| 2 | 3.02 | 3.22 | 2.68 | 3.33 | `positive`, 12 | 22 |
| 3 | 7.65 | 8.14 | 6.79 | 8.42 | `danger`, 15 | 16 |
| 4 | 3.05 | 3.24 | 2.70 | 3.35 | `positive`, 13 | 30 |
| 5 | 5.13 | 5.46 | 4.55 | 5.65 | `positive`, 14 | 8 |
| 6 | 7.76 | 8.25 | 6.88 | 8.54 | `danger`, 29 | 19 |
| 7 | 3.14 | 3.34 | 2.78 | 3.45 | `positive`, 22 | 21 |
| 8 | 4.77 | 5.07 | 4.23 | 5.25 | `danger`, 12 | 28 |

In the dark theme:

| Slot | `bg` | `surface` | `surface-alt` | `input-bg` | Nearest status | From `accent` |
|---|---|---|---|---|---|---|
| 1 | 4.07 | 3.73 | 3.31 | 3.11 | `positive`, 31 | 17 |
| 2 | 10.06 | 9.22 | 8.18 | 7.67 | `positive`, 12 | 16 |
| 3 | 6.39 | 5.86 | 5.19 | 4.87 | `danger`, 19 | 25 |
| 4 | 4.45 | 4.08 | 3.62 | 3.39 | `positive`, 13 | 20 |
| 5 | 4.09 | 3.75 | 3.33 | 3.12 | `positive`, 19 | 12 |
| 6 | 3.98 | 3.65 | 3.24 | 3.03 | `danger`, 26 | 22 |
| 7 | 7.75 | 7.11 | 6.31 | 5.91 | `positive`, 23 | 10 |
| 8 | 3.98 | 3.65 | 3.24 | 3.03 | `danger`, 13 | 28 |

Between the slots themselves:

| Measure | Light | Dark |
|---|---|---|
| Neighbours, normal vision, worst pair | 22.6 (slots 5 and 6) | 16.2 (slots 4 and 5) |
| Neighbours, protanopia or deuteranopia, worst pair | 10.1 (slots 5 and 6) | 8.3 (slots 2 and 3) |
| Any two of the first four, normal vision | 15.1 | 24.6 |
| Any two of the first four, simulated | 14.3 | 8.0 |
| Any two of all eight, normal vision | 14.5 (slots 2 and 5) | 15.3 (slots 3 and 8) |
| Any two of all eight, simulated | 7.3 | 5.6 |

Three things in these tables need PUDL's judgement.

The light theme's three lightest slots, 2, 4 and 7, fall below 3:1 on `surface-alt`, at 2.68 to 2.78. Requiring `surface-alt` as well removes every teal and olive that stays apart from `positive` and `accent`, because in the light theme those hues must be dark to pass and then sit too close to the status green and the accent blue. I chose to leave `surface-alt` out of the rule. A mark drawn on `surface-alt` would then need a 1px outline in `text-muted`, or the application keeps its marks off that surface.

Eight colours cannot all be told apart from every other under simulated colour blindness, and no ordering fixes that, so the order is tuned for neighbours. Where any mark may sit beside any other, as in a scatter plot or a set of tags, the first four slots are the safe set.

The dark values are lighter than many charting guides advise for dark surfaces, between 0.56 and 0.79 in OKLCH lightness. PUDL's own dark `accent`, `warn` and syntax colours sit in the same range, and on the dark `input-bg` a colour needs a lightness of about 0.56 or more before it reaches 3:1.

## Past Eight

PUDL should not generate a ninth colour. What happens past eight depends on whether something other than colour already identifies each item, so I propose two rules.

- Where position or words identify each item, as with lanes in a graph, tags with their words and a group chat's members with their titles, slot nine takes slot one's colour again and the sequence repeats. The repetition is harmless because colour was only ever an aid there.
- Where colour is the main way to match an item to its legend, as in a chart, the application shows the first seven and folds the rest into one "Other" in `text-muted`, or splits the chart into several.

## Dark Theme and Forced Colours

The dark theme sets its own eight values, chosen against the dark surfaces, with the same order and the same hue families as the light ones, so that a slot means the same family in both themes and an application never maps slots by theme.

In forced colours the system replaces the page's colours, and PUDL should say exactly what the category colours become instead of leaving it to each browser. I propose that `pudl.css` sets every `--category-N` to `CanvasText` inside `@media (forced-colors: active)`. Every mark then draws in the reader's text colour, and identity rests on what the rules above already require: position, labels, legends and words. An application that wants marks told apart in forced colours as well can vary the dash or the marker shape, which is the application's own business.

## What Stays With the Application

- The application decides which item gets which slot. It assigns slots in a stable order by identity, so a filter that hides one series does not repaint the others, and a branch keeps its colour while the graph scrolls.
- The application mixes its own fills, tints and widths from the tokens, such as a tag's swatch, a translucent chart area or a chosen lane's emphasis. PUDL might later add derived fills if several applications want the same ones.
- The application chooses any dash patterns and marker shapes it uses.
- The application maps the slots onto a third-party renderer such as Mermaid.

## Questions for PUDL

1. Is `category-1` to `category-8` the right name? Chart libraries tend to say "series", which suits charts and suits tags and lanes less well.
2. Should `surface-alt` be in the contrast rule, at the cost of the teal and olive slots and with them two of the eight hue families?
3. Is a status distance of 12 enough? The closest pairs are slot 8 to `danger` and slot 2 to `positive`, 12 or 13 apart in each theme and 27 to 37 degrees of hue away. Raising the floor to 15 leaves too few hues in the light theme for eight slots.
4. Should a theme that changes `bg`, `surface` or the lighting be required to set all eight? I measured the default values against Dinah's eleven built-in themes, and every one of them pushes at least one slot below 3:1 in at least one form: slot 2 on the light surfaces (2.27 on Tokyo Night to 2.85 on GitHub) and slot 6 on the dark input background (2.15 on Nord to 2.94 on Graphite). Only Tokyo Night's and GitHub's dark forms pass. A silent fallback to the defaults would therefore break the contrast rule in most themes, so I recommend that the rule require a theme to set them, or that PUDL publish a derivation that fits each slot's hue to the theme's surfaces.
5. Should the palette also be offered as a script function that returns slot N for an item, so every application assigns by the same rule? I lean against it, since assignment is application state.

## How Dinah Would Adopt It

The Git extension's lane graph would replace its five mixed colours with `var(--category-N)`, lane by lane in order, and would keep drawing the topology in full. Dinah's theme files would gain eight values for every built-in theme, fitted to each theme's own surfaces since the defaults fail most of them, and its theme check in `appearance.js`, which already tests the status and syntax colours against all four backgrounds, would test the category colours against the rules above. The diagram renderer would pass the eight slots to Mermaid as `git0` to `git7` and as the first eight pie colours, so diagrams follow the reader's theme. Group chats would mark each member chat with a slot's colour beside its title, in the order the members were added.
