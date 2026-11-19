# Judgements popup stability

The Judgements popup retains the size it opened with until it closes. Dismissing a warning removes its row and lets following rows move upward inside the existing box. The condition line and Dismiss All remain above the scrolling content. When the last warning is dismissed, the content shows “No warnings.” and Dismiss All remains visible but disabled. The next opening measures the current content again.

Dinah calls the documented `pudlMenu.place(panel)` function during the native opening `toggle` event, then records the panel width, panel height and condition header height. These measurements belong to this opening only and are cleared during the next opening's `beforetoggle` event. Dinah does not modify PUDL or depend on its private placement classes, mutation observers or listener ordering. The public placement function is documented in PUDL 0.95.2, the bundled release. PUDL continues to handle placement and viewport constraints. A viewport resize may constrain the panel to the available space.

The warning rows, daily summary, calibration/budget details and navigation links share a scrolling body. An asynchronous daily-summary response or a new warning can change that body's contents without resizing the panel or header. The condition text remains readable through its own overflow area if it grows during the opening. The daily summary appears below the warnings so that its later arrival cannot move the dismissal controls.

After keyboard dismissal, focus moves to the warning now occupying the removed row's index, or to the previous warning if the removed row was last. When no warnings remain, focus moves to Open Judgements. Dismiss All also transfers focus there after clearing the list. Scroll anchoring is disabled within the body so removal does not preserve an obsolete row position.

## Verification

The warning regression samples the panel, condition header, condition text and Dismiss All rectangles on every animation frame while three warnings are dismissed individually. It also exercises new warnings, the 10-to-9 badge boundary, Dismiss All, the empty state and reopening with smaller content. Keyboard focus is checked after each individual dismissal. The variants cover light and dark themes, three viewport widths and normal/doubled text.

The four focused warning/widget/budget/status suites passed. A further warning run added the next-control position check and passed all twelve variants, recording 430 sampled frames with zero geometry differences. All 158 browser suites then passed in 200 seconds with eight workers. The full run recorded 496 popup frames across the twelve warning variants with zero geometry differences; failures.txt is empty. JavaScript syntax and whitespace checks were clean. The recurring chat restoration/click failures and Paul's original group incident remain separate open investigations.

## The flyout as a PUDL popover, 19 November 2026

Paul asked on 19 November 2026 for a redesign that follows PUDL and approved the mockup (`docs/mockups/judgements-flyout/index.html`) the same day, with room for two warnings. What is described above is replaced by this section.

The flyout is PUDL's popover as PUDL specifies it. Its head has the title and Close. Under it the state is a glyph, a colour and one word (On, Warning, Paused, Off, Reading), which are the status bar item's own, and one line says why when the state is not simply on. Five facts are rows of a key-value table: the judge, the day's calls, the latest call, the budget and the pool. The warnings are PUDL notices under their own heading, with Dismiss All beside the heading. The two ways out, Judgement Settings and Open Judgements, are buttons in the popover's foot.

The flyout has one size for as long as it is open, and it gets that from its stylesheet. Every row is in the page from the start (`index.html`), and a value that is still being read says so in its row. The line that says why has room for two lines whether it has words or not. A value that does not fit its row is cut and its whole text is its tooltip. The warnings' heading keeps to one line. Only the warnings scroll, in a region with room for two. So nothing is measured when the flyout opens, the script that kept its opening size is gone, and a flyout opened with no warnings is the size of one opened with three.

The engine's state for the flyout gained the chosen judge's provider and model. The day's count comes from that state, so the flyout no longer asks for a summary after it opens.

After keyboard dismissal, focus moves to the warning now in the removed one's place, or the one before it, or to Open Judgements when none is left. Dismiss All moves it to Open Judgements.

The warning suite samples the flyout, its head, its state line, Dismiss All and its foot on every animation frame while warnings are dismissed and arrive, in light and dark, at three widths and at normal and doubled text, and finds no difference.
