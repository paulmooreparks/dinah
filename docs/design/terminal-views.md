# Views, the agenda, completion and a live board

This document is a design, proposed on 2026-09-24 and ruled on by the operator on 2026-09-25; section 9 records his rulings. When it was written nothing in it was built. It combines five ideas that came out of comparing Dinah with command-line and terminal tools: saved views from gh-dash, a ranked agenda from Taskwarrior, shell completion, a board that redraws from k9s, and a key footer from lazygit. The terminal mockups use cards from the development workbench as it stood that day, trimmed to fit.

## 1. The problem it answers

A person working a Dinah workbench asks the same four questions many times a day. What is waiting for me? What is in flight, and who holds it? What should I look at first? What can I do to this card? Each of these takes several commands today, and each command needs a card number or a column slug typed from memory.

`status` answers the first part of the second question with column counts. `tree --group-by column,state` lists every card under its column, which is a board read top to bottom. `query` filters, and `next` names the head of one column's queue. None of them lets a person name a question once and ask it again, none ranks work across the whole workbench, and nothing in the terminal completes a card number.

## 2. The design in one paragraph

The design adds one idea, the view, and two verbs, `view` and `completion`. A view is a named list of sections, and each section is a query the workbench already knows how to answer. A view also names its layout and its order. The board and the agenda become two views Dinah ships, not two new commands, because a board is a view laid out in columns and an agenda is a view sorted by urgency. `--watch` redraws any view when the workbench changes. Shell completion reads the same workbench to complete card numbers, columns, field values and view names. A terminal UI follows as its own phase, and section 8 describes what it would reuse.

Section 10 of the critical analysis found that surface growth drives the inflow of new cards. This design is shaped to add as little surface as it can. It adds two verbs and not five, and it adds one block to the workbench definition and no new query syntax beyond one placeholder.

## 3. Views

### 3.1 What a view is

A view has a name, a title, a layout, an order and one or more sections. Each section has a title and a query written in the existing query language. A view shows each section's cards in turn, and a card that matches two sections appears in both.

The query language has no `or` between terms, which is the reason a view has sections. A person who wants "everything waiting for me" wants the union of several different questions: cards at the operator's stations, cards carrying a question only he can answer, and cards blocked on him. Each of those is one query, and the view is their union with a heading over each part.

A query in a view may use one placeholder, `@me`, which stands for the caller's actor as `whoami` reports it. The placeholder is expanded before the query is parsed, so the query language itself does not change.

### 3.2 Where views are declared

Views can be declared in two places, and a view in the second replaces a view of the same name in the first.

1. The workbench definition, `workbench.md`, holds the views everyone working the workbench shares. They travel with the repository.
2. The user's own settings hold personal views and personal replacements for shared ones.

Dinah also ships three built-in views, `board`, `agenda` and `mine`, and a workbench or a user may replace any of them by name. Section 7 lists them.

A workbench's views are presentation and not part of the contract a second implementation must honour. They belong in a first-party layer, `dinah.views`, which another implementation may ignore. Section 9 records the operator's ruling confirming that placement.

### 3.3 The declaration

```yaml
views:
  waiting-on-me:
    title: Waiting on me
    layout: list
    order: urgency
    sections:
      - title: At my stations
        query: "column:operator-design-review,operator-code-review,acceptance"
      - title: Questions only I can answer
        query: "item_owner:operator item_state:pending"
      - title: Blocked
        query: "state:blocked"
  urgent-in-flight:
    title: Urgent work in flight
    layout: columns
    sections:
      - query: "priority:now,next column!=intake column!=done"
```

The second section of `waiting-on-me` names two fields the query language does not have today. Section 3.5 explains why the design needs them.

### 3.4 Reading a view

```console
$ dinah view waiting-on-me
Waiting on me                                          acting as paul, operator

At my stations (8)
  Card       Column        Pri   Sev    Title
  ---------  ------------  ----  -----  -----------------------------------------
  dinah-572  Acceptance    now   major  dinah setup connects a named harness to a …
  dinah-472  Acceptance    next  major  A checklist item's four states cannot say …
  dinah-593  Acceptance    next  major  A column declares standing checklist items…
  dinah-573  Acceptance    next  major  dinah prime answers, in one read, what an …
  dinah-597  Acceptance    next  minor  An unblock can say why the block was lifte…
  dinah-589  Acceptance    next  minor  Collection rows draw no icon, so a collect…
  dinah-583  Acceptance    soon  minor  attach accepts a workstream reference, rea…
  dinah-590  Acceptance    soon  minor  A level axis can be conditioned on another…

Questions only I can answer (1)
  Card       Column        Item         Title
  ---------  ------------  -----------  ------------------------------------------
  dinah-565  Design Queue  questions/1  Dinah's 1.0 scope is written down…

Blocked (0)
  Nothing matches.
```

`dinah view` with no name lists the views the caller can see, where each came from, and which one a name resolves to. It is the same shape `dinah setup --list` already uses for recipes.

```console
$ dinah view
  View                 Title                      Layout   From
  -------------------  -------------------------  -------  ---------
  agenda               What needs me first        list     built in
  board                Board                      columns  built in
  mine                 Held by me                 list     built in
  waiting-on-me        Waiting on me              list     workbench
  urgent-in-flight     Urgent work in flight      columns  workbench
  triage-sweep         Intake older than a week   list     user
```

`dinah --json view <name>` returns the sections and their cards in the shape `query` already returns, with the section titles added. The editor extension can then show a view as a tree without knowing how it was declared.

### 3.5 Two questions the query language cannot yet ask

The most useful view, "waiting on me", needs a card's pending items, and today's query fields describe the card and its journal but not its checklist. Two new fields would let a view ask about a card's items.

- `item_owner` would be the owner an unresolved item names, taking `holder` or `operator`.
- `item_state` would be the state of an item, taking the item lifecycle's own states, which after dinah-472 include `waived` and `withdrawn`.

As with the journal fields, both terms would have to be satisfied by one and the same item. `item_owner:operator item_state:pending` therefore finds a card carrying at least one pending item that the operator owns, and not a card with a pending item of anyone's and a separate operator item already settled. This is the only change the design asks of the query language, apart from `@me`.

## 4. The agenda: one ranked answer to "what first?"

### 4.1 Why it ranks

The query guide says severity and priority filter but do not rank, and that stays true of `query`. The agenda is the one surface that ranks, and it earns the exception by showing its arithmetic. Every score it prints can be taken apart into the terms that made it, which is what Taskwarrior's urgency does and what makes a ranking trustworthy rather than magical.

### 4.2 The terms

A card's urgency is a sum of terms. Each term's weight is declared in the workbench, and Dinah ships defaults so a workbench that declares nothing still gets a sensible order.

| Term | Applies when | Default weight |
|---|---|---|
| waits on you | the card stands at a station the caller owns | 10 |
| your question | per unresolved item the caller owns, up to three | 4 each |
| priority | the card's priority, by its rank in the declared list | 0, 2, 4, 6 |
| severity | the card's severity, by its rank in the declared list | 0, 1, 2, 4 |
| blocked | the card is blocked | 3 |
| blocks others | per card waiting on it through a link the workbench declares under `dinah.holds` | 2 each |
| age | per day in its current column, up to five days | 0.5 per day |
| stale claim | the claim has outlived its expiry | 3 |

Weights are declared under `dinah.urgency` in `workbench.md`, like this, and a term a workbench leaves out keeps its default. The two mapping terms are written with one member per line, and a comment goes on a line of its own, because the shared block reader reads a flow mapping and a trailing comment as text.

```yaml
dinah.urgency:
  waits-on-you: 10
  # priority weights, lowest first: later, soon, next, now
  priority: [0, 2, 4, 6]
  age:
    per-day: 0.5
    cap: 5
```

The agenda ranks only cards the caller can act on. For an agent it means the cards `next` would offer it, which already respects tier, route and claims. So an agent's agenda is `prime`'s ready list put in order, and `prime` could adopt that order later without changing what it offers. For the operator it means four arms: cards at his stations, except a station of the done kind, since a finished card there is work already done; cards carrying his items, in any column, the done kind included; every blocked card, because a block is how this workbench raises a question for him; and, on his ruling of 2026-09-25, the ready cards `next` would offer him.

### 4.3 Reading the agenda

```console
$ dinah view agenda
What needs me first                                    acting as paul, operator

  Rank  Card       Where         Urgency  Why
  ----  ---------  ------------  -------  ------------------------------------
     1  dinah-572  Acceptance       18.5  waits on you, now, major, 1 day here
     2  dinah-573  Acceptance       16.5  waits on you, next, major, 1 day here
     3  dinah-472  Acceptance       16.0  waits on you, next, major
     4  dinah-593  Acceptance       16.0  waits on you, next, major
     5  dinah-565  Design Queue     11.0  your question, next, major, 2 days
```

Ties break on age, oldest first, and then on card number, which is why dinah-472 stands above dinah-593.

`--explain` prints every term for every row, so a surprising rank can be checked rather than argued about.

```console
$ dinah view agenda --explain dinah-572
dinah-572  dinah setup connects a named harness to a workbench …
  waits on you     Acceptance is yours                 +10.0
  priority         now, rank 4 of 4                     +6.0
  severity         major, rank 3 of 4                   +2.0
  age              1 day in Acceptance, 0.5 per day     +0.5
  -------------------------------------------------------------
  urgency                                               18.5
```

The figure in the agenda's Urgency column and the total `--explain` prints come from one computation. A test should hold every row's column figure equal to that row's explained total, so the two can never drift apart.

## 5. Layouts, and the board

dinah-288 built this section, and its specification is the contract wherever the two differ. What follows describes what shipped.

### 5.1 Two layouts

A view is laid out as a `list`, which is a table per section as in section 3.4, or as `columns`, which is a board. The board is not a new command. `dinah view board` is the built-in view with the `columns` layout, and the operator ruled on 2026-09-25 against a `dinah board` command, so the views guide shows the one-line alias a person can give themselves instead.

### 5.2 How columns fit a terminal

The development workbench has fourteen columns, and Intake alone holds hundreds of cards, so the layout has to choose what to show. These are the rules.

- Every line draws in one column fewer than the window, so no line reaches the window's last column and nothing depends on how a terminal wraps there.
- A column appears only when it holds at least one card that matches the view. Empty stations take no width.
- A column the view marks as collapsed, by default the intake and done kinds, is drawn as a count on a line of its own under the heading, and never as a column.
- Each column gets an equal share of the width, with a minimum of twenty characters and a gutter of two. When the columns will not fit at that minimum, they wrap into further bands below the first, still in flow order, so the board never scrolls sideways. Below twenty characters the board draws one column as wide as the window allows and cuts everything in it to fit.
- Each column shows at most five cards and then "+N more". `--all` lifts the cap.
- A card takes two lines. The first carries its state glyph, its number, the operator mark where an item waits on the operator, then its holder or its block's kind, then its priority, dropping the priority first and then shortening and dropping the holder as the column narrows. The second carries its title, cut to the column's width. Severity is never drawn.
- Text is cut between whole characters, whole emoji sequences and whole flags, and ends in an ellipsis.

A board at 118 characters, drawn by the binary over a workbench shaped like the development one:

```console
$ dinah view board
Board                                                                                        acting as paul, operator
Collapsed: Intake 5

Triage (3)             Design Queue (7)       Agent Design Rev… (1)  Implement (2)          Acceptance ◆ (7)
─────────────────────  ─────────────────────  ─────────────────────  ─────────────────────  ─────────────────────
○ 6  later             ○ 9 ◆  next            ● 16  claude  now      ● 17  claude  now      ○ 19  next
  Jira-resolution wo…    Dinah's 1.0 scope …    A starved run upda…    A declared field m…    dinah setup connec…
○ 7  soon              ○ 10  later                                   ✕ 18  operator-ruling  ○ 20  next
  Rung three: the in…    The workbench live…                           Move this project …    A checklist item's…
○ 8  soon              ○ 11  later                                                          ○ 21  next
  The quick start te…    The compatibility …                                                  A column declares …
                       ○ 12  later                                                          ○ 22  next
                         A workbench declar…                                                  dinah prime answer…
                       ○ 13  later                                                          ○ 23  next
                         A long-lived serve…                                                  An unblock can say…
                       +2 more                                                              +2 more
```

### 5.3 Glyphs, colour and plain terminals

Status is shown by a glyph, and colour only ever reinforces it, so the board reads the same on a monochrome terminal and to a reader who cannot tell the colours apart.

| State | Glyph | Colour | Plain form |
|---|---|---|---|
| ready | ○ | default | `o` |
| active | ● | blue | `*` |
| blocked | ✕ | red | `x` |
| waits on the operator | ◆ in the column heading and after a card's number | yellow | `!` |

Rules are `─`, or `-` in the plain form, and a cut ends in `…`, or `...`. The colour is yellow because neither the Windows console's documented character attributes nor terminfo's eight portable `setaf` colours has an amber.

No documented interface says whether a console's font can draw a glyph, so the glyph set is chosen and never detected. It is the Unicode set unless `--plain` asks for the plain one for a run, or the setting `glyphs: plain` asks for it every time, on the operator's ruling of 2026-09-25. `NO_COLOR` turns colour off and changes nothing else. A file or a pipe receives the same glyph set as a terminal, in UTF-8, with no colour and no control sequence, laid out at `COLUMNS` or at 80.

### 5.4 Watching

`--watch` works on any view. The view is drawn, `changes --wait` blocks until the workbench changes, and the view is drawn again in place. The watch also redraws when the window's size changes and when a claim the last frame drew expires. The frame is written one row at a time, each row placed at the start of its line and erased to its end, with a status line on the window's last row. No row is written with a line feed and none reaches past the window, so a frame never scrolls it. On Ctrl+C the last frame stays on the screen, the colour and the cursor are put back, and the prompt returns below it.

```console
$ dinah view board --watch
Board                                                                                        acting as paul, operator
Collapsed: Intake 5

…
updated 09:41:07 · dinah-598 moved Spec to Agent Design Review by claude · Ctrl+C stops
```

The status line names the change that caused the redraw, which answers the question a person watching a board actually has, namely what just happened.

On Windows the watch and the colour go through the classic console functions and never through a virtual-terminal sequence, on the operator's ruling of 2026-09-25, because nothing Microsoft documents says how a sequence cut across two `WriteConsole` calls is parsed, and the console writer can cut one. Six of the seven functions carry this notice on their Microsoft Learn pages, quoted as the pages carry it: "This document describes console platform functionality that is no longer a part of our ecosystem roadmap. We do not recommend using this content in new products, but we will continue to support existing usages for the indefinite future. Our preferred modern solution focuses on virtual terminal sequences for maximum compatibility in cross-platform scenarios." `GetConsoleScreenBufferInfo` carries none. The functions sit behind one package, `internal/screen`, so moving to virtual-terminal sequences later changes no interface.

Everywhere else the watch writes only what the terminal's own terminfo entry supplies, found through `TERM` and read by an in-tree reader of the compiled format, again on the operator's ruling of 2026-09-25, with no new dependency.

## 6. Shell completion

### 6.1 What it completes

Completion saves the most keystrokes of anything in this design, and it asks nothing of the contract. It completes:

- verbs and their flags, from the same table `dinah help` prints;
- card references, with the card's title shown beside each where the shell can show a description;
- column slugs, level names and workstream slugs;
- query fields, and after `field:` the values that field takes on this workbench;
- view names and recipe names.

```console
PS> dinah move dinah-59<Tab>
dinah-590  A level axis can be conditioned on another field's value…
dinah-593  A column declares standing checklist items…
dinah-594  A declared field may enumerate its legal values…
dinah-597  An unblock can say why the block was lifted…
dinah-598  A starved run updates the independent reader…

PS> dinah move dinah-594 <Tab>
agent-code-review   design-queue
```

The second completion offers only the columns the card may legally move to, read from the same affordances a move's response carries. A person is never offered a move the workbench would refuse.

### 6.2 How it works

`dinah completion powershell`, `bash`, `zsh` or `fish` prints a script for the named shell, which the user loads from their profile. The script calls back into Dinah for candidates, as `dinah completion --complete -- <words so far>`, so the candidates always come from the binary and workbench actually in use. No list is baked into the script, so the script never goes stale when a card is filed.

A completion runs on every Tab, so it has to be quick. It reads card titles from the card files and nothing else, filters by the typed prefix before it formats anything, and stops at two hundred candidates.

## 7. What Dinah ships

| View | Layout | Order | Sections |
|---|---|---|---|
| `board` | columns | column | every live card, the intake and done columns collapsed |
| `agenda` | list | urgency | cards the caller can act on |
| `mine` | list | column | `holder:@me`, split into active and blocked |

## 8. A terminal UI

A terminal UI is the phase after the board, and the design above is most of one. A terminal UI would add input to what `view --watch` already draws. The ideas it would take are these.

- **From k9s.** `:` jumps to a view, a column or a card, `/` filters the current pane with a query, and the heading always shows which workbench and which actor are in use.
- **From lazygit.** A footer lists the keys valid for the selected card, built from that card's affordances. The footer can only offer what the workbench would allow, so an illegal action is absent from the screen rather than refused after the fact. That is the same principle as a representation that omits illegal actions, and Dinah already computes the list.

```console
┌ Acceptance ◆ (11) ─────────────────────────┐┌ dinah-572 ───────────────────────────────────────────┐
│ ○ 572  dinah setup connects a named har…   ││ dinah setup connects a named harness to a workbench,  │
│ ○ 472  A checklist item's four states c…   ││ from a recipe anybody can add                         │
│ ○ 593  A column declares standing check…   ││ Acceptance · ready · priority now · severity major    │
│ ○ 573  dinah prime answers, in one read…   ││ Merged as 7ef4ccea. 28 criteria verified.             │
│ ○ 597  An unblock can say why the block…   ││ Pull request 303.                                     │
└────────────────────────────────────────────┘└──────────────────────────────────────────────────────┘
 enter show   a accept → Done   b send back → Implement   c comment   / filter   : jump   q quit
```

The footer in that mockup shows the review walk suggested earlier: a person working through Acceptance one card at a time, accepting or sending each back. It is the most repetitive thing the operator does at a terminal.

The surfaces document and the workbench's own standing text disagree on where a terminal UI would live. The surfaces document says every head lives in the one binary, as `dinah mcp` and the planned `dinah ui` do. The workbench says the command-line tool stays tiny with no board of its own. The operator's second ruling in section 9 settles it in favour of the one binary.

## 9. Decisions

The operator ruled on all four decisions on 2026-09-25, and on a fifth that followed from the fourth.

1. **Views are declared in a first-party layer.** They live in a `dinah.views` layer in the workbench definition and in the user's own settings, not in the core contract, because views are presentation and another implementation of the contract should not be obliged to render them.
2. **A board does not break the "no board interface" rule.** The operator said the rule had been read too strictly, and that he is now more willing to have a board in the tool. `view` with the `columns` layout goes ahead.
3. **The agenda's weights are declared per workbench,** with the defaults of section 4.2.
4. **A terminal UI comes sooner rather than later.** The operator values a live view that updates highly, so the terminal UI is planned as the phase after the board and `--watch`, not left until somebody else drives a workbench. Section 8's question about where it lives is settled by the second ruling: the rule no longer keeps an interactive board out of the tool, so the terminal UI can be one more head in the one binary, as the surfaces document already describes for `dinah mcp` and `dinah ui`. Its own design settles the details.
5. **The terminal UI is built on Bubble Tea.** On the same day the operator chose Bubble Tea, with its Bubbles components and Lip Gloss, over tcell, tview and custom code. The comparison behind the choice was measured. Stripped minimal builds came to 1.85 MB for the baseline of `golang.org/x/term`, 2.32 MB for tcell with 17 modules, 3.18 MB for tview with 19, and 3.22 MB for Bubble Tea with 36. Dinah's own binary is about 9.7 MB, so download size did not decide it. Reading the libraries' source showed that all three drive the Windows console with escape sequences. The choice reverses two rulings made earlier that day, on dinah-288, but only for the interactive head: the board drawn through the classic console functions and the in-tree terminfo reader. `dinah view board` and `--watch` keep both. Bubble Tea was preferred because it handles keyboard input across terminals, can be tested without a real console, and supplies text input, lists and a help component that draws key bindings as a footer. It also renders strings, so the existing board drawing can be reused. The costs accepted are a 36-module supply chain and a major-version transition that was under way at the time. The terminal UI never binds a bare Esc key, so the library's guess at Esc timing never matters. dinah-603 carries the ruling.

Shell completion is built at the same time as views, as section 10 allows.

## 10. Phasing

| Phase | Delivers | New surface | Depends on |
|---|---|---|---|
| 1 | views, `@me`, `item_owner`, `item_state`, `dinah view`, the `list` layout | one verb, two query fields, one block | nothing |
| 2 | the agenda: urgency terms, `--explain`, the `urgency` block | one block | phase 1 |
| 3 | shell completion | one verb | nothing, and it can run beside phase 1 |
| 4 | the `columns` layout and `--watch` | three flags on `view`, the `glyphs` setting and the `dinah.watch-unavailable` refusal | phase 1 |
| 5 | a terminal UI | one verb, `dinah tui`, and the `dinah.tui-unavailable` refusal | phase 4 |

Phases 1 and 3 are independent and can run at the same time. The phases together add three verbs. Every new verb and message still costs eight catalogs, a help entry and quick start transcripts, which are the shared generated files the critical analysis found taxing every merge. The design keeps the number of verbs down partly for that reason.

## 11. What this design deliberately leaves out

- **No `or` or grouping in the query language.** Sections give a view unions without widening a language that `query` users and agents both rely on.
- **No ranking inside `query`.** Only the agenda ranks, and it shows its arithmetic.
- **No interactive input before the terminal UI.** Everything in phases 1 to 4 prints and exits, or prints and waits for the workbench to change, and input arrives only with phase 5.
- **No persistence of a person's position in a view.** The editor extension already remembers what was selected, and a terminal view that remembered would be a second place for that state to live.
