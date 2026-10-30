# Nothing Moves When a Chat's State Changes

Paul reported on the evening of 30 October 2026 that Dinah's interface is inconsistent and jarring in use. Two examples came in quick succession. A group's member chips moved their menu buttons whenever a member's state word changed length, so the buttons could not be clicked while chats were working. A chat window's message box changed its height and width between Working and Idle, because the buttons beside it and the hint inside it differ between the two. Both passed review, because every review and every suite so far has looked at the interface still, one state at a time, and neither kind of check can see movement.

## The Rule

Nothing on screen changes its size or position because of a live change in state: a turn starting or ending, a chat waiting or becoming stuck, a message streaming in, a member joining or leaving the working set, a count going up or down. A control that the person might be reaching for stays where it is.

- A control that exists in one state and not in another keeps its place reserved in both, or sits where its appearing moves nothing else, such as at the end of a row whose other items are fixed.
- A label whose text changes with state either has a fixed width that fits its longest form, or is replaced by a glyph in a fixed slot with the words as its accessible name and tooltip (see [chat-states.md](chat-states.md)).
- A text box keeps one height and width in every state, and its hint text does not change with state.
- Content the person is reading, such as a transcript, may grow, but it never moves the controls around it.

Changes the person causes, such as resizing a window, opening a menu or typing more lines into the message box, may change layout as they do now.

## How It Is Checked

A suite drives each surface through the transitions it can make, including working to idle, idle to waiting, waiting to stuck and back, streaming, members changing state, and counts changing. It records the bounding box of every interactive control before and after each change, and fails if any control moved or changed size. The surfaces are the chat window (title bar, status line, toolbar and message box), the group window (member chips, toolbar and message box), the team strip, the chat lists, the pickers and the taskbar.

Dinah UI also reviews live behaviour as well as still screenshots. For each surface it watches the transitions happen, in light and dark and at a narrow width, and lists everything that moves, flickers or jumps, however small.

The composer suite waits for the requested `data-win-state` before sampling each transition. A queued update need not draw before the next animation callback. The trace records its wait duration and pending frames, then applies the same strict geometry and action checks from state application onward. Late-capability checks observe a fresh Queue result followed by a fresh Steer result before resolving an older answer.

## Order

Front End fixes the message box first, since it is what Paul types into: one geometry for Working and Idle, with the send, queue and steer actions arranged so that none of them appears or disappears in a way that resizes the box, and one hint text. Then comes the stability suite. Its first run will list the other movements, and each is fixed under the rule above. Dinah UI's live review runs beside it and adds what a suite cannot judge, such as flicker and distracting motion.

## The First Audit

Dinah UI drove 64 state changes on master 497afbe on 30 October, in light and dark at 1440 and 700 pixels wide, sampling every interactive control on every frame (message 1619). Every surface it covered moved. The message box grew from 63 to 85.5 pixels and halved its Send button. Group member menus jumped sideways by up to 298 pixels, and an expanded member area grew from 66 to 176 pixels, moving everything below it. A chat's own status reason wrapped and pushed the team strip down. The title bar's buttons jumped 131 pixels for one frame while its status chips folded. Chat list rows grew by 27 pixels as reasons wrapped. The taskbar's running dot widened its button and moved every button after it. A long reason enlarged a mention candidate.

Two decisions follow from it, beyond the rule above.

- **A reason takes one line.** Wherever a state's reason appears (a chat's status line, a team member row, a chat list row, a picker or mention candidate), it is a single line of fixed height, cut with an ellipsis when it is too long, with the whole reason as the tooltip and accessible name. Aggregate counts in a list row are one line in the same way. Rows therefore keep one height whatever their state.
- **Folding decides once.** A title bar or toolbar that folds its status into a menu decides from widths that do not depend on the state's current text, such as the reserved width of each mark, so it cannot fold and unfold from one frame to the next.

Dinah UI also found the glyphs too small to carry the state alone: Idle's ring read as a speck, and Working's arc was hard to tell from it. Each mark keeps a slot of at least 16 CSS pixels, with Idle a visibly closed hollow ring and Working a clearly open arc, reviewed with animation stopped as well as running.

Group member chips keep their menu buttons and their identity through execution updates. Execution appears as the shared glyph in a fixed slot. Lifecycle words use that same reserved slot, with one line, ellipsis and complete details in the tooltip and accessible name. Window presence appears in those details and the member's menu. The member area keeps its aggregate counts and turn budget to one line each; their complete text remains available. State changes neither rebuild an open member menu nor move its anchor, and the menu's state explanation updates in place.

## Chat List and Team Rows

Chat-list and team-strip rows reserve one line for reasons and metadata, including an empty reason. Execution glyphs and lifecycle words share a reserved state slot; full state and reason text remains in the accessible name and tooltip. List attention badges and group counts reserve their capped `99+` width.

The chat lists retain their row links and group headers while execution changes. They repaint state and group counts in place while the pointer is over the list, focus is inside it, or a row menu is open. Pending regrouping applies once all those interactions end. Explicit scope and filter changes take effect immediately. Reordering preserves scroll positions and restores any retained focus without scrolling. A mounted team also repaints when the execution snapshot changes, so the order of team and execution events cannot leave its badges one state behind.

## Transcript Navigation and Measured Rows

A retained reading place belongs to the actual `scrollTop` captured with it, or read after its last correction. If that position changes outside the hold, the hold expires and the next controlled mutation captures the current entry. Delayed observer callbacks cannot restore a reading place from before navigation. This uses the documented [scrollTop coordinate](https://drafts.csswg.org/cssom-view/#dom-element-scrolltop) without depending on [mutation observer delivery](https://dom.spec.whatwg.org/#mutation-observers) relative to navigation.

Rows marked `data-scroll-anchor-skip`, including the earlier-entry marker, cannot anchor the reading place. Trimming replaces that marker, so capture prefers the first real entry in view. The regression exercises stale callbacks, stale hold reuse and marker resizing deterministically. Its trim trace waits for the earlier-entry count to advance and the last delivered entry to render, then checks every frame. Subpixel corrections remain recorded separately from the existing 12 px large-jump bound.

A jump to the latest releases the saved reading place before following the end. Once a bounded chat log has been read back, its rows keep their measured layout when following resumes. Re-enabling estimated heights during a jump changes the scroll extent underneath its destination. The chat log already trims its retained rows, so this preservation remains bounded.

The jump also updates the pinned prompt before its final scroll position is set. The pin and unread controls can change the transcript viewport or extent. The final position is calculated after those changes in the same operation. Deferring that correction to the next animation frame leaves a visible gap for one frame.

While reading back, DOM mutations restore the held row during mutation delivery. Resize observation also restores it when an image or font changes its size later. The regression suites sample the readable row and the jump destination on every animation frame. They reject a transient displacement even when the final position recovers.

## A Control That Exists in One State

An action that cannot be taken is absent rather than shown disabled, and its place is still kept. When a chat is idle, Stop keeps its width and height in the toolbar but is invisible, cannot be focused or clicked, and is not in the accessibility tree; when a turn starts it appears in that same place (Architecture and Dinah UI, 31 October, messages 1732 and 1733). Toolbar folding counts the reserved width in both states, so hiding the control never changes what folds and never leaves an empty item in the overflow menu. If Stop has focus when the turn ends, focus returns to the message box. Every other control that exists in only one state is treated the same way.

## Paul's Priority

Paul decided on 31 October to accept a brief Idle flicker when a stop is blocked to deliver a message, since removing it would rest on an order of events that neither the adapter nor Claude Code documents, and asked that the work now be about getting Dinah stable. Until the movements found by the first audit are fixed and the stability suite passes for every surface, no new feature is started. Skins wait behind that work, and so do new requests that are not defects.

## No Heading in a Chat Window

Paul asked on 31 October for a chat window to lose the large title at the top of its body and the status line beneath it, because both repeat what the window's title bar already shows and they take vertical space that the group window, which has neither, gives to the conversation. The chat window's body therefore starts with the team strip, when the chat leads a team, or else with the transcript. The title bar keeps the chat's name and its state, and the state's reason and the chat's workspace are in the title bar's tooltip and in its folded status menu. Rename and Delete, which sat beside the title, remain in the chat's Actions menu, where they already are. The explanation a waiting chat needs, such as an approval, is in the request card in the transcript, so nothing is lost with the status line. This replaces the earlier rule in [chat-states.md](chat-states.md) that kept the visible state word in a chat's own status line; the title bar's state chip now carries it, with its width reserved so that a change of state never moves the window's buttons.

## The Composer's Reserved Actions

Send and Queue share one fixed-width control. Steer keeps its measured space beside that control while idle, but is invisible, inert and absent from the accessibility tree. Stop has a fixed-size empty slot beside the folding toolbar, so its absence does not change the width the toolbar receives. The hidden Stop command is also absent from Actions and folded menus. A turn ending while Stop or Steer has focus returns focus to the message box. The message box uses the same hint in both states.

Stop's slot takes the intrinsic width of its longest actual label, including Stopping, its glyph and its padding. It keeps that width while idle. The composer reserves PUDL's reported minimum folded toolbar width before sharing the row with Stop, so a narrow window wraps Stop to another row instead of clipping More. That arrangement follows the available width and text size throughout the turn. The regression includes 320-pixel windows at both text sizes, checks controls against every clipping ancestor, hit-tests them before scrolling and opens the presented commands with a direct pointer click. Its wide fixtures fit between the docked panes so those panes cannot cover the controls under test.

Removing the body heading moves renaming from that heading to a Rename Chat form opened by Actions or F2. The title bar keeps the name throughout editing. State reasons and workspace remain in the state chip's tooltip and accessible name, which the folded status control also uses.

The composer suite checks every sampled frame in light and dark at 1440 and 700 pixels, with normal and doubled text. It includes window buttons, checks hidden-action interaction separately from geometry, and exercises provider-aware defaults, capability changes, late replies and once-per-change Queue logging. The accepted adapter Idle flicker remains outside this frontend correction; no timed hold conceals a real end.

## The Message Box at Narrow Widths

The first build of a fixed message box (31 October) reserved Stop's slot at the width of its own label, so in a narrow window with large text the column of buttons beside the text box was wider than the text box itself. The column of buttons is as wide as its widest button and no wider, and the text box takes the rest of the row. When the window is too narrow for the two side by side, the buttons sit below the text box. That choice follows the window's width, which the person controls, and never the chat's state, so the rule above still holds.

## Native Window State and Open Status Menus

Status-bar count badges reserve their widest displayed form through zero. Notifications and Judgements show at most 99+ while their tooltips and accessible names retain the full counts. Invisible, inert measurement spans use the same badge styling and are excluded from accessibility. The Agent badge reserves its longest state word, starting, including its glyph. The regression samples every status control through zero/one, 9/10 and 99/100 count boundaries and all provider states. When reserved tray widths, More and the minimum front button cannot fit, Dinah offers one Status control in the tray. Its reserved glyph shows the most urgent folded status, and its reserved badge totals the unseen counts. The accessible summary retains every item, state and count. Each fixed row opens the original item's panel. The layout decision runs on resize and reserved-font-size changes, with constant measurement labels; execution state only repaints the contents. Dismiss All belongs beside the Judgements summary, separate from the warning rows.

Dinah supplies PUDL's execution attributes from the engine's state and gives the mark the short state word as its label and the workspace and reason as its detail. Background work and subagent counts remain separate statuses. Draft, closed and unknown lifecycle words occupy a reserved host slot without a second execution glyph. The slot stays in the layout when the native execution mark is present.

The open Window status panel reserves two complete rendered rows and their gap, using the current line height and enough room for each glyph. Extra rows scroll inside that capacity, with Dismiss fixed below. Rows stay on one line and keep their complete text accessible when ellipsized. Changes to lifecycle words or the number of status rows therefore leave Dismiss in place. The taskbar stability suite keeps both the stack popup and folded status panel open through all four execution states, unknown and closed states. It checks every sampled frame, including the state actually painted, control appearance and disappearance, focus retention and the absence of the legacy running attribute.

The six-row popup check covers reach, vertical clipping and Dismiss geometry at doubled text. PUDL 0.95.2 supplies the named, focusable status region. Ordinary checks now require keyboard scrolling to the sixth row, Tab reaching Dismiss and focus returning to the opener. Dinah does not modify PUDL's generated popup DOM to supply them. A draft chat's neutral chip shows its destination workspace; its accessible name and tooltip retain the complete context without repeating New Chat visibly.

PUDL 0.95.2 owns pointer input and the two-row viewport, selected by data-win-status-rows on the chat window. Dinah's interim pointer and list-sizing overrides are removed; its single-line text truncation remains. The regression uses native scrollbars and actual wheel input, checks that the split and transcripts behind it stay put, then clicks Dismiss with a fixture control directly underneath and checks that the underlying action did not run. These interaction checks remain in the suite.

Actions comes from flags on the chat's command definitions. The menu suite checks every visible flagged command, invokes Drop the Queued Messages, and checks that queue changes add and remove it while Actions remains open. PUDL's menu freshness replaces the old forced refresh after command redraws and retitling.
