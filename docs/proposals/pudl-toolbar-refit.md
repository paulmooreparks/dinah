# Proposal: Fold a Toolbar Again When Its Menu Closes

Dinah asks PUDL to fold a folding toolbar again once a menu of that toolbar closes, when what the toolbar holds changed while the menu was open. This was a proposal for Paul to hand off. PUDL 0.93.1 does what it asks and adds the README sentence it asks for, and Dinah, now on 0.93.1, has dropped the workaround described at the end.

## What Happens

On 28 October 2026 Paul had two chat windows of the same width whose composer toolbars were folded differently. One had folded its Agent and Message groups into menus while the other showed them open, and restoring and snapping the folded window unfolded its toolbar. The folding had been worked out for content the toolbar no longer held, and it stayed that way until the window's width changed.

PUDL 0.93.0 folds a toolbar again in two cases. Its `ResizeObserver` on the toolbar refolds it when the toolbar's width changes. Its `MutationObserver` on the toolbar, which watches the child list, the text and the `disabled`, `aria-disabled` and `hidden` attributes of everything in it, marks the toolbar as changed and folds it once the change is done. Both lead to `fit(bar)`, which clears the changed mark and records the width before it calls `least(bar)` and `foldTo(bar)`.

Both `least` and `foldTo` return without doing anything while any `.menu-panel` in the toolbar is open, which is right, since folding the group whose menu is open would take the menu's button away from under it. The trouble is that nothing folds the toolbar again when the menu closes. The changed mark is already cleared and the width has not changed, so the next call to `fit` finds nothing to do, and the folding stays as it was until the width changes.

In Dinah this happens whenever the composer's toolbar changes while one of its menus is open. A turn starts or ends, which shows or hides the Stop button; a subagent starts, which shows the Subagents button or raises its count; or a saved chat reconnects, which Dinah does as one of the toolbar's menus opens and which redraws the provider, the mode and the model. Opening a menu also redraws that menu's rows inside the toolbar, which the observer sees, so `fit` is called while the menu is open even when nothing else changed.

## Reproduction

The composer-fold browser suite in Dinah (`cmd/dinah-desktop/frontend/e2e/composer-fold.js`) has the reproduction as its last two checks. Without Dinah's workaround they fail as follows.

1. A chat window is made as narrow as it can be while its toolbar still holds every group open with no turn running (700px in the stubbed page).
2. The Model menu in the toolbar is opened, a turn starts so that the Stop button shows, and the menu is closed. The toolbar is left with every group at stage 1 and its content wider than its row, so the Stop button runs past the end.
3. The width is nudged so that PUDL folds the toolbar for the running turn (Agent and Message at stage 2). The Model menu is opened again, the turn ends, and the menu is closed. The toolbar is left folded at stage 2 although every group now fits open.

A page with no Dinah in it shows the same thing. Make a folding toolbar with a menu button in it at a width where everything fits, open the menu, append a button to one of the groups so the row overflows, close the menu, and read `data-toolbar-stage` on the groups. They stay at 1 until the toolbar's width changes or `pudlToolbar.fit(bar)` is called.

## What Is Asked

When `fit` finds a menu of the toolbar open, PUDL should remember that it has folding left to do and do it when that menu closes. One way is to leave the changed mark set when `least` or `foldTo` skip because a menu is open, and to listen in the capture phase on the toolbar for `toggle` events whose `newState` is `closed` from any `.menu-panel` in it, calling `fit` from there. Toggle events do not bubble, so capture is needed, and a toggle event comes after the popover has closed, so the menu no longer counts as open by then.

It would also help if the README's section on folding toolbars said that a toolbar does not fold while one of its menus is open, and that it folds once the menu closes.

## Changes PUDL Does Not See

Two kinds of change in a toolbar that affect its width are not seen by PUDL's observer, and the README does not say which changes it watches. A class added to a control, such as one that widens its padding, and a change to a control's `style` attribute both leave the folding as it was. Dinah makes neither change in its toolbars, so this proposal does not ask for them to be watched. A sentence in the README listing the changes that refold a toolbar, and saying that a page making any other change calls `pudlToolbar.fit(bar)`, would settle it.

## How Dinah Worked Round It

Until it moved to PUDL 0.93.1, Dinah worked round the problem as follows; the composer-fold suite's two checks now pass on PUDL alone. As each menu of the composer's toolbar closes, Dinah calls `pudlToolbar.fit(bar)`, which PUDL's contract (`docs/CONTRACT.md`) describes as "fold a toolbar now". The workaround rests only on that documented call and on the HTML standard's rule that a popover's `toggle` event is fired after the popover has closed. It does not depend on PUDL skipping the fold while a menu is open. Once PUDL folds again on its own when a menu closes, Dinah's listener can go, and the composer-fold suite will show whether it went cleanly.
