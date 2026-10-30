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

## Order

Front End fixes the message box first, since it is what Paul types into: one geometry for Working and Idle, with the send, queue and steer actions arranged so that none of them appears or disappears in a way that resizes the box, and one hint text. Then comes the stability suite. Its first run will list the other movements, and each is fixed under the rule above. Dinah UI's live review runs beside it and adds what a suite cannot judge, such as flicker and distracting motion.

## The First Audit

Dinah UI drove 64 state changes on master 497afbe on 30 October, in light and dark at 1440 and 700 pixels wide, sampling every interactive control on every frame (message 1619). Every surface it covered moved. The message box grew from 63 to 85.5 pixels and halved its Send button. Group member menus jumped sideways by up to 298 pixels, and an expanded member area grew from 66 to 176 pixels, moving everything below it. A chat's own status reason wrapped and pushed the team strip down. The title bar's buttons jumped 131 pixels for one frame while its status chips folded. Chat list rows grew by 27 pixels as reasons wrapped. The taskbar's running dot widened its button and moved every button after it. A long reason enlarged a mention candidate.

Two decisions follow from it, beyond the rule above.

- **A reason takes one line.** Wherever a state's reason appears (a chat's status line, a team member row, a chat list row, a picker or mention candidate), it is a single line of fixed height, cut with an ellipsis when it is too long, with the whole reason as the tooltip and accessible name. Aggregate counts in a list row are one line in the same way. Rows therefore keep one height whatever their state.
- **Folding decides once.** A title bar or toolbar that folds its status into a menu decides from widths that do not depend on the state's current text, such as the reserved width of each mark, so it cannot fold and unfold from one frame to the next.

Dinah UI also found the glyphs too small to carry the state alone: Idle's ring read as a speck, and Working's arc was hard to tell from it. Each mark keeps a slot of at least 16 CSS pixels, with Idle a visibly closed hollow ring and Working a clearly open arc, reviewed with animation stopped as well as running.

## Transcript Navigation and Measured Rows

A jump to the latest releases the saved reading place before following the end. Once a bounded chat log has been read back, its rows keep their measured layout when following resumes. Re-enabling estimated heights during a jump changes the scroll extent underneath its destination. The chat log already trims its retained rows, so this preservation remains bounded.

The jump also updates the pinned prompt before its final scroll position is set. The pin and unread controls can change the transcript viewport or extent. The final position is calculated after those changes in the same operation. Deferring that correction to the next animation frame leaves a visible gap for one frame.

While reading back, DOM mutations restore the held row during mutation delivery. Resize observation also restores it when an image or font changes its size later. The regression suites sample the readable row and the jump destination on every animation frame. They reject a transient displacement even when the final position recovers.

## A Control That Exists in One State

An action that cannot be taken is absent rather than shown disabled, and its place is still kept. When a chat is idle, Stop keeps its width and height in the toolbar but is invisible, cannot be focused or clicked, and is not in the accessibility tree; when a turn starts it appears in that same place (Architecture and Dinah UI, 31 October, messages 1732 and 1733). Toolbar folding counts the reserved width in both states, so hiding the control never changes what folds and never leaves an empty item in the overflow menu. If Stop has focus when the turn ends, focus returns to the message box. Every other control that exists in only one state is treated the same way.
