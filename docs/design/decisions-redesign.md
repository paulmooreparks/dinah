# Decisions Redesign

Paul approved this layout and behaviour on 16 November 2026. This note records the design for the Decisions window, the page that gathers the questions agents ask Paul. It answers the three complaints Paul made on 15 November 2026 and lists the other problems I found when I ran the current page. The implementation follows the approved mockup, including his answers below.

The mockup is `docs/mockups/decisions/decisions-redesign.html`. It loads the shipped PUDL, brand, dinah and app stylesheets, the palette the real window applies (captured from it into `clear-chime-tokens.css`), and the proposed layout in `decisions-redesign.css`, which is what would replace `decisions-applet.css`. Opening the page without a state lists every state with a light and a dark link. Each state is only a starting point, because the page works from there. You can choose a request, answer it, send, dismiss through the confirmation dialog, filter the list, ask for more requests, and open the record of a finished request. A finished request leaves the list and the next one opens. A small Try menu in the title bar belongs to the mockup alone. It makes happen the things you cannot do from Decisions: a new request arrives, a task asks a question or is cancelled, the open request changes, or a send is not confirmed. It also switches between light and dark. `explore/mockup-walkthrough.js` runs through all of this headless and checks 69 results. The pictures in `pictures/` show each state at 1440 and 390 pixels wide in both themes. The original current-page captures and overview images remain private because their demo labels are excluded from public evidence. The public mockup and matching walkthrough use neutral invented examples.

## How the current page was examined

I ran the shipped page headless through the e2e fixture with invented requests: none, one, five of mixed kinds, a five-question request, one already partly answered, a plain group message with a 24-line quote, and one whose asking chat had been retired. I read, typed, chose As Recommended, sent, dismissed, filtered and asked for earlier requests, and recorded Send's state on every animation frame while typing. The scripts are in `docs/mockups/decisions/explore/`.

## Paul's three complaints

### The page is ugly

The current page is a toolbar, a status band, a list column and a reading column centred in whatever width is left. With no requests the list column is an empty panel beside the words "Choose a request to read and answer." With requests, the reading column is 720 pixels wide in a 1008-pixel pane, so a third of the pane is margin. Each request opens with a grey delivery box that permanently holds a disabled Check Send Result button, then a blank band kept for a message that rarely appears, then a fixed 18rem box of earlier group messages that scrolls inside the page and is mostly empty. Send Answers sits below the fold on every request at 1440 by 1000, so the first thing a reader sees is never the action. Several controls are greyed with no reason: Earlier Requests, Check Send Result, and Send itself.

The redesign uses PUDL's master-detail layout, which the current page reinvents by hand.

- **With nothing waiting,** the whole window is one PUDL empty state that says nothing is waiting and what will appear. There is no list pane beside it.
- **The list** is a PUDL row list with the filter field in its head and a count badge beside the heading. Each row shows the first open question, who asked and where, and a status badge with its glyph (questions waiting, a message, partly answered, or cannot be delivered). Rows have one fixed height.
- **The request** has a fixed head (who asked, in which group, when), one scrolling middle and a fixed foot. Between them, the questions scroll on the left, and About This Request is a pane on the right with its own scrolling: who asked, where the reply goes and what was said before in the group. On a phone the pane folds away as PUDL's master-detail does, one pane at a time: the head has an About This Request button, and the pane has Back to the Request.
- **The foot** always shows Send and a status line that says what Send will do or why it cannot yet. It has one height in every state, so nothing jumps.
- **Problems are said where they arise.** An unconfirmed send, a changed request and a draft that could not be saved each appear in the foot's status line, with glyph and colour, and Check Send Result or I Have Read the Change appears only while it is needed. A request that can no longer be delivered opens with a PUDL warning notice.

### Send flashes while typing

The cause is in the page. Every keystroke saves the draft, and Send is disabled for as long as a save is in flight. The status text also switches between "Your draft is being saved." and "Your draft is saved." on each key. With a 30 millisecond save, a fair stand-in for the round trip to Go, typing one 47-character answer changed Send's state on alternate frames:

| Page | Characters typed | Times Send changed state |
|---|---|---|
| Current page | 47 | 93 |
| Mockup | 47 | 1 |

With an instant save the current page shows no flash at all, which is why the existing suites never caught it.

In the redesign Send depends only on whether there is something to send. A draft save is never shown and never disables anything. Pressing Send while a save is still pending already waits for it, because sending writes the draft first. Send therefore changes once, when the first answer appears, and its label and width never change. In the mockup the head, the scrolling middle, the foot, the status line and Send kept one position and size on every frame while typing, at both widths.

### The last item stays after it is done

After a send that closes the request, or a dismissal, the current page keeps drawing the closed request from its saved copy. The reader sees "Your reply was sent." or "Message dismissed. No reply was sent." above the old question, a line saying the request is not in the list of open requests, and every control disabled.

In the redesign a finished request leaves the list as soon as the engine confirms it, and the next open request opens in its place. The foot's status line confirms what happened ("Your answers were sent to Release Engineer in Release 2.0.2.") with a Show Answers link to the existing read-only view of that request. When the finished request was the last one, the empty state appears with the same confirmation under it. A partial answer keeps the request open; the answered questions show their answer and time with a positive badge, and the rest stay answerable.

## Other changes and why

- **Answering is one set of choices.** Today a question has a text box, an As Recommended button that copies the recommendation into the box, and a separate menu of suggested answers that also writes into the box, so the reader cannot tell afterwards whether the text was edited. The redesign shows the recommendation as flat text and offers sunken radio choices: As recommended, each suggested answer, and My own answer with its text area always visible. Typing in the text area chooses My own answer, so nothing appears or disappears while answering.
- **A plain message has one label.** Today a group message shows "Your Reply" twice, once as the heading over the quoted message and once over the input. The redesign heads the quote "Message", says who wrote it, and keeps "Your Reply" for the input.
- **A long message is quoted once and folds in place.** The redesign folds a quote after twelve lines behind a Show the Whole Message button that opens it in the same scroll, instead of a box that scrolls inside the scrolling page.
- **A request that cannot be delivered keeps every control.** Today a request whose asking chat was retired still has an enabled answer box and As Recommended button, while Send can never be pressed. The redesign shows the reason in a warning notice and the questions read-only. Its foot has the same Dismiss Request… and Send Answers as every other request, with Send disabled and the status line saying the answer cannot be delivered.
- **The destination is said in words.** "Send to Release 2.0.2, replying to Release Engineer. Reply to message 101." becomes "Reply goes to Release 2.0.2, as a reply to Release Engineer" in the summary, without the internal message number.
- **Earlier Requests is replaced by the list's own foot.** The engine returns open requests newest first, 100 at a time, so the greyed Earlier Requests button can only work past 100 open requests. The redesign ends the list with "That is every open request." and shows a Show More Requests button there only when more exist.
- **The status band goes.** "5 awaiting an answer" becomes the badge beside the list heading. Load errors become a PUDL notice in the list pane.
- **Refresh leaves the toolbar.** The list already follows the bus as requests arrive and close, and Refresh Requests stays in the window's menu. With the filter in the list head, the toolbar is no longer needed.
- **Wording.** "1 questions waiting" becomes "1 question", and the list says which questions are still open on a partly answered request.

## Questions a task owns

Paul wants every question for him gathered in Decisions, including the ones a running task asks. Front End's presentation contract (`docs/design/task-question-presentation.md` on `frontend/task-question-ui`) and Engine's confirmation of it define what such a question carries. It has a service name, a task title, a workspace, the question, the service's own options, and either a link to the task's page or the reason there is none. It has no chat, group or recommendation.

The redesign gives it the same list row, head and foot as any other request, so it needs no layout of its own. The row shows the question, the service and task title, and a plain Task badge. The head shows the task title where a chat's name would be, says which service asked, and has Open Task in place of Open Chat. The service's options are radio choices with no text area, because the answer path accepts only an option. The About pane shows the service, the task with its page link, the workspace and when it was asked, and has no earlier conversation. When the task has no page, the head says why and that the question can still be answered. The foot has Dismiss Question… and Send Answer, like every entry. When the task ends it first, the question leaves the list like any finished request, and the foot's status line says the task was cancelled and no answer was sent. The mocked states are "task", "task-no-page" and "task-ended".

## Typed questions from a task

A task can also ask for words or for several choices, under Engine's contract for typed task questions (`docs/design/typed-task-questions.md` on `engine/typed-task-questions`). A text question has a limit the service sets, from 1 to 16,384 characters, and may require an answer or allow an empty one. A multiple-selection question offers up to 16 options with a minimum and a maximum number to choose. Dinah adds no bounds of its own.

A text question is answered in a text area under the question, labelled Your Answer. A multiple-selection question is a set of check boxes. Under either is one line that is always there, so nothing moves while answering. For text it says how many characters are used of the limit, counting characters as the host counts them, so an emoji counts as one and the text area has no `maxlength`, which would count differently. For choices it says how many to choose and how many are chosen. Over the limit, the line takes the danger colour and a warning glyph and says by how much, and Send waits, with the foot's status line saying why. Nothing is cut off or refused while typing, so the person can shorten the answer or swap one choice for another. The record of a sent answer shows the text as written, or the choices in the order the service offered them.

The same questions are cards in Notifications, with the same controls and the same draft, so an answer started in one place is there in the other. Answering in either settles the question in both, and it leaves Decisions like any finished request. The mockup shows Notifications as a small panel beside the window. The mocked states are "task-text", "task-text-over", "task-multiple", "task-multiple-over" and "notifications".

## What the redesign keeps

Every state still has its address: the open request, the filter and the read-only view of a closed request stay in the window's state, and on a phone the list and the request are separate addresses as PUDL's master-detail requires. The engine's contract is unchanged. Drafts, operation identities, reconciliation after a lost receipt, stale-revision rejection and the dismissal confirmation all work as they do now. The redesign changes what the page shows, not what it sends.

## Limits of this mockup

- The engine has no request that expires. The closest state is a request whose asking chat was retired or whose group is gone, which the engine marks as dismissible with a reason, and that is the state shown as "cannot be delivered".
- Broadcast to Active and Broadcast to All share this window and are not mocked. They would take the same fixed head and foot, with Preview Recipients and Confirm Broadcast in the foot.
- The mockup shows the window maximized without Dinah's menu bar, docks and taskbar around it.
- The current page's typing measurements used a simulated 30 millisecond save. I did not measure the real bridge's save time.

## Every entry's commands, Dismiss and withdrawal

Paul asked on 16 November 2026 for a right-click on each item with the relevant commands, among them a Dismiss for anything he has dealt with elsewhere or can no longer answer. He asked for the same dismissal control on the open request itself, and said he did not understand why some entries had that button and others did not. Engine's settlement contract (`docs/design/request-settlement.md` on `engine/request-settlement-contract`) defines Dismiss and Withdraw on the host.

Every row of the list now has an actions button, always shown and a Tab stop, beside the row. It opens the same menu as a right-click on the row, Shift+F10 or the Menu key. The menu is the same for every entry: Open, Open Chat, Open Group, Open Task, and Dismiss Request…, Dismiss Message… or Dismiss Question…. A command that cannot apply is disabled with its reason under it, and never left out. A task question's Open Chat says a task question comes from a service, not a chat. A direct request's Open Group says it was asked directly. A retired chat's Open Chat says the chat was retired, and a task without a page gives the service's reason.

The open entry's foot has the same two buttons for every entry: Dismiss and Send. Dismiss asks first, in a PUDL dialog that says no answer is sent and who is told: the asking chat, so it does not wait, or for a task question the service, which decides what happens next. A request whose chat was retired says nobody can be told. Dismissing a partly answered request keeps the answers already sent. Send is disabled while there is nothing to send or the answer cannot be delivered, and the status line says why.

A chat that asked can withdraw its own request, for instance when Paul settles the matter with it some other way. The request leaves the list as a finished one does, the status line says which chat withdrew it, and its record says that the chat withdrew it and when. The Try menu stands in for the chat with The Asking Chat Withdraws the Open Request. Only the asking chat can withdraw, so the person's menu offers no Withdraw. Cancellation of a task is still said as cancellation, never as a withdrawal.

The mocked states are "row-menu", "dismiss-task", "withdrawn" and "about-phone".

## Paul's answers

Paul answered the four questions on 16 November 2026, and said yes to each.

1. The list runs oldest first, so a new request joins the bottom and nothing above the one being read moves.
2. After a request is finished, the next one opens by itself.
3. As Recommended and the suggested-answer menu become one set of radio choices.
4. Refresh leaves the toolbar and stays in the window's menu.

He also asked for the changes described above: the menus on every item, Dismiss everywhere with the same controls on every entry, a chat able to withdraw its own request, and About This Request in a pane on the right.

## Product integration

The product keeps each request's draft and the existing operation receipt protocol. Saving an edit does not change Send's enabled state or the footer's position; submitting still flushes the latest draft before contacting the host. The selected radio mode is saved with its text, so an own answer that happens to equal a suggestion remains an own answer.

About switches to its own address when the reading pane is narrow, including when pinned docks leave little room inside a wide desktop. The list follows PUDL's documented 640-pixel master-detail policy. Requests remain ordered by their recorded creation time, with deterministic identity tie breakers.

Text and multiple-selection task controls are shared with Notifications, including unsent drafts, validation and the once-only answer gate. Text uses Unicode scalar counts and retains input beyond the limit for correction. Multiple answers use the host's option order. Task metadata and destinations come from the host. Early task events wait for saved UI state before restoring a draft.

Dismiss remains visible and disabled with an explanation until Engine's universal settlement host is received. The page exposes no person-side Withdraw. The current task end event cannot distinguish an answer from cancellation, so the product reports that a question is no longer waiting. Cancellation-specific wording and durable disposition records will bind to the settlement host's authoritative event and passive read when they land.
