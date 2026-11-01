# Transcript navigation release correction

The composer release stopped on exact merged 9e3494d140f35f901b9f9dd65a2eac5089be93b4 after its full browser gate reported chat-scroll-place, group-controls, group-last-read and pin failures. This branch preserves that merge and investigates those failures independently of the row-layout/team-navigation successor and unpublished PUDL work. Original output is retained under the release directory and copied with a hash into .diagnostics/composer-gate-20261101.

## Controlled observations

One controlled released-50a49b run reproduced reading-place, unread-navigation and pin failures in 75 seconds. One exact-9e3494d run reproduced all four in 50 seconds. Both used a diagnostic preload without product or assertion edits. The preload records synthetic fixture key/focus/scroll events, follow-state changes and geometry; those layout reads can perturb timing. Neither result is an uninstrumented full-gate result or a claim that the composer change introduced the defect.

The candidate toolbar trace records Ctrl+Home clearing following at scrollTop 21131, followed by a scroll event at the unchanged end and following becoming true again. Native movement subsequently reaches 21081, then tailing restores 21131. Waiting for scrollTop zero therefore times out. The chat reading fixture also places a target as soon as following becomes false, while the default keyboard movement can still be pending; the retained streaming samples show that target moving far below the viewport afterward.

The pin trace records pin-go beginning a smooth scroll from 1550 to 1548 and 1540. Following becomes true within those first near-end pixels and tailing restores 1550, leaving the original message above the viewport. This is navigation to the message, not dismissal of the pin.

The unread trace records Ctrl+End starting at 5450 with viewport 223 and extent 7634. As unread controls disappear, the viewport and extent change. The native destination is then short of the current end and following remains false. These observations identify concrete navigation/follow transitions, without treating the four original outputs as proof of one shared cause.

The first preload did not survive a goto:false fixture's navigation. A targeted diagnostic attempt to correct that then observed documentElement too early and produced observer errors. That attempt is invalid as a product page-error result and is retained separately. The corrected preload observes the document and survives navigation. No fixture deadlines or assertions were changed.

## Correction and ownership

The transcript owns line, page and endpoint scroll commands when its log itself has focus. Home releases the reading anchor, clears following and moves to the beginning in the same operation. End uses the existing latest operation, which follows the current extent after unread controls change layout. Arrow keys move by the computed line height; PageUp/PageDown and Space move by the viewport, with Shift+Space moving backward. Ctrl/Meta+Home/End use the same endpoints. The design no longer relies on a default keyboard animation finishing before another transcript operation.

Already-handled keys and keys targeting descendants remain with their owner. This includes select, input, textarea, empty/plaintext-only contenteditables and nested buttons/links. Shift selection shortcuts keep their browser default. The keydown listener is non-passive because these explicit log commands cancel the default action; wheel/touch/pointer listeners retain their passive behavior.

Pin-go uses the existing transcript item-navigation operation. It makes the owning message visible without starting a separate smooth scroll that can be captured by near-end following. Existing item navigation also releases its target when the reader navigates again. This correction changes no send, provider, recipient or persistence behavior.

## Verification boundary

The four original suites passed in 55 seconds after the navigation correction, with unchanged assertions and deadlines. Quality then identified the nested-control boundary before the candidate was frozen. A dedicated actual-keyboard suite covers focused log Home/End, changed extent, nested controls, already-handled keys and selection shortcuts. The final affected-consumer gate and immutable evidence record must pass before handoff. These fixtures do not establish the cause of the separate live typing/window-switch latency, native profiler readiness or installed acceptance.

After the keyboard ownership guard was added, an affected 17-suite run passed sixteen suites but group-last-read failed during a later back operation. Its targeted trace shows Home prevented the default, reached scrollTop zero and kept following false, then the view moved through 37, 68, 91 and 103 pixels without another navigation key. The earlier PageDown remained a separate default keyboard operation. This failure justified extending explicit ownership to the log's line/page commands instead of treating Home/End alone as sufficient. A rapid PageDown/End/Home regression samples twelve subsequent observations and requires zero throughout. That regression and group-last-read passed in four seconds after the extension. Earlier failed outputs remain separate and are not erased by that pass.

The final uninstrumented affected gate passed all seventeen suites in 81 seconds with two workers and empty failures.txt. It included the original four, transcript-keyboard, chat-resumed-follow, chat-tail, chat-stream-tail, chat-streaming, chat-stream-markdown, group-chat-composer, teams-traffic, teams-tail, chat-unread, chat-windows, chat-scrollback and composer-stability. The group transcript screenshot was inspected. Independent review and full merged gating remain required. A coordinated diagnostic launch was subsequently authorized in bus 2562; no heavy gates may overlap that live capture.
