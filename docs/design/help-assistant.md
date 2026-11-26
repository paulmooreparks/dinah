# A help assistant in the window Assistant

Paul asked on 26 November 2026, through Software Director, for a helper that a person can consult for advice on how to use a feature or solve a problem, which he described as an AI-enabled Clippy without the goofy animation, and suggested it could extend the Assistant. Software Director proposed eight points for it. This note checks those points against the code as it stands on master at 539440eb, says what would have to be added, and asks Paul what only he can decide. Nothing in it is built, and the form of any screen it describes needs his approved mockup first.

## 1. What the Assistant does today

The window Assistant shipped on 20 November 2026 (a9c64d9f, 338bf993, 11a1479d). Its plan is section 2 of `docs/plans/four-requests-2026-11-20.md`, and its approved mockup is `docs/mockups/window-assistant`.

It opens as a popover from the Assistant item in the status bar, from Ctrl+Space, and from Window, Assistant (`cmd/dinah-desktop/frontend/index.html`, the `assistant-menu` popover; `frontend/assistant.js`). The popover has a line to type in with Ask, the person's list of tasks with Edit Tasks…, one paragraph that says what a task would do or what a request did, and Run and Undo in its foot.

Each request to the Assistant is answered by a judgement call. `cmd/dinah-desktop/judge_assistant.go` declares a judge kind, `window-assistant`, through `internal/judge`. The plan proposed a warm hidden chat on the default provider, and the build took the cheaper route instead: every request is one self-contained call with no history and no tools, made on whatever provider and model the person chose on the Judgements page, which calibration narrows to the cheapest model that passes the kind's cases. The call has a deadline of 12 seconds and an input cap of 24,000 characters. When there is no judge, when the weekly budget for that provider refuses the call, or when the answer is late or fails its check, a rule written in Go reads the request by its words and answers instead, so the Assistant works with judgements off.

The page sends the request (at most 600 characters) and up to 60 windows, each with its key, kind, title, workspace and state, and never their contents. The engine adds the chats with their provider, team and state, the team names and the providers (`judge_assistant_chats.go`). The answer is one of eleven actions (gather, tile, working, tidy, all, side, front, say, switch, stop, tell) with the windows or chats it applies to and one sentence for the person. The kind's check refuses any answer that refers to a window, chat, provider or team the call was not given, which is the mechanism point 2 below relies on.

The page carries the answer out with the window layer's own operations and the chat window's own switch (`assistant.js`, `chat-applet.js`). Every action is one the person could take by hand, so there is no permission prompt. A typed request runs at once on Ask; a task first says what it would do and waits for Run, and a task that acts on chats lists them with ticks. Undo restores the windows, except after a stopped turn or a sent message, which the result says cannot be undone.

Each call is recorded in the judgement store, appears on the Judgements page, and is counted against the provider's weekly budget (`internal/judge/budget.go`). After the bare-session change of 20 November (8cc9a14d), a judgement call costs about 900 tokens of overhead through Claude, about 9,600 through Codex, and a few hundred through Ollama, plus its input.

## 2. How much of the eight points exists

The Assistant already supplies the call, the grounding check, the budget and the action layer, and what is missing is mostly the help itself (Table 1).

**Table 1** *The Eight Points Against What Exists*

| Point | State | What exists |
|---|---|---|
| 1. Consulted, never interrupting; a quiet sign after a failure | Partly | The Assistant opens only when asked. Errors already become cards under Notifications (`docs/design/notifications.md`), and a failed menu command raises `dinah:menu-error`. Nothing links an error to the Assistant. |
| 2. Answers from shipped help and the command list; says when help does not cover it | Partly | The command registry (`frontend/commands.js`, `window.dinahCommands`) holds about 100 commands with id, menu, label, shortcut, enabled state and the reason a disabled one cannot run, and `searchable()` lists them. Shipped prose help is one page, Working with Chats, of about 4 KB, plus the keyboard rows added with `addHelp`. The judge check that refuses unknown ids exists and can be copied. |
| 3. Knows the window in front, the selection and the last error | Partly | `assistant.js` already reads the windows in z-order with kind, title and state. The notices store can list error entries. There is no general way to read what is selected. |
| 4. Explains and points: highlight a control, offer a button that runs the command | Partly | `dinahCommands.run(id)` runs a command only when it is enabled and no modal dialog is open; `controlsFor(id)` returns the controls on the page that run it. There is no highlight, and a command that lives only in a closed menu has no control to point at. |
| 5. Cheap: small model, fresh short conversation | Covered | A judge call is one-shot, on the calibrated cheapest model, under a weekly budget, with a rule behind it. |
| 6. Sends only the question, the names of what is on screen and the help text | Covered in principle | Every judge kind declares the structure of its only input (`docs/plans/judgement-agents.md`), and the Assistant sends titles and states and no content. |
| 7. Dinah ships neutral help; a product adds its own through an extension point | Partly | A product's page scripts load from `product-applets.json` and can already call `dinahCommands.register` and `addHelp` (`docs/design/product-applets.md`). There is no way to add prose help sections. |
| 8. A product's tutorial by feature, with command ids, can feed the helper | Not | No help format with sections and command ids exists yet. |

*Note.* Command count from `register({ id:` calls in `cmd/dinah-desktop/frontend` on 26 November 2026.

## 3. What would have to be added

The parts needed are listed in Table 2, in the order they would be built.

**Table 2** *Parts to Add*

| Part | What it is |
|---|---|
| Help section format and registry | A section is an id, a title, a few paragraphs and the command ids it is about. `dinahHelp.add(section)` on the page, called by Dinah's own help file and by a product's scripts; Help, Working with Chats is drawn from the same sections. |
| Dinah's own help sections | The writing: one section per feature of Dinah, in the neutral terms Dinah already uses. |
| Accuracy test | A test that every command id in a section exists in the registry, run with the Go gate, beside `help_accuracy_test.go`. |
| Section choice by rule | A word match of the question against section titles, text and command labels that picks the few sections sent to the judge. It is also the whole answer when there is no judge: the matched sections, quoted. |
| Help judge kind | A second kind, `help-answer`, taking the question, the front window's kind and title, the last error's text, the chosen sections and their commands. Its answer is a short reply, the section ids it used, at most two command ids, or "not covered". Its check refuses any section or command id it was not given. |
| Answer view in the Assistant | Room in the popover for a reply with its sources, Show Me and Do It. A question is told apart from a window request by a toggle or a leading question form, which the mockup has to settle. |
| Show Me | An outline drawn over the control `controlsFor(id)` returns, or the menu opened at the command's row when it has no control on the page. It must not move or resize anything. |
| Do It | A button that calls `dinahCommands.run(id)`, disabled with the command's own reason when it cannot run. |
| Ask About This | An action on error cards under Notifications that opens the Assistant with the error as the question's context. |

## 4. What I would change in the eight points

**Point 1.** I would drop the separate quiet sign and put Ask About This on the error card that Notifications already shows. A new sign would be a second notice of one failure, and a mark that appears on a failure would move something on the screen, which the standing rule against live movement forbids. The card is already where the person goes to find what went wrong.

**Point 2.** I agree with the aim, and the guarantee has to be enforced by code that checks the answer, since an instruction to the model can be ignored. The Assistant's check already refuses an answer that refers to a window it was not given, and the help kind would refuse any command or section id outside the call's input in the same way, falling back to the quoted sections. The larger problem is that the help barely exists: with one 4 KB page, nearly every question would honestly be "not covered". The helper is worth building only after Dinah's help is written, so I would make the writing the first part of the work and not an afterthought.

**Point 3.** I would keep the front window and the last error and leave out the selection. No general selection API exists, and a selection is usually content (a passage of a document, a chat message), which point 6 keeps away from the provider. The person can paste it into the question when it matters.

**Point 4.** I agree, with one limit: Do It appears only for a command that takes no further input and is not marked `danger`. A dangerous command gets Show Me, so the person runs it from its own place with its own confirmation. Nothing runs on Ask; Do It is always a separate press.

**Point 5.** The helper should be a judge call like the Assistant, which means there is no conversation at all, short or otherwise. A follow-up question would carry the previous question and answer as part of its input and nothing more. It spends the same judgement budget, so the person controls its cost where they already control the rest.

**Point 6.** I agree and would add one item: the text of the last error goes too, since it is the subject of the question. Error texts can contain a file path or a chat's title, and the Ask About This button makes that choice visible to the person.

**Point 7.** I agree. The extension point is `dinahHelp.add`, called from the scripts a product already lists in `product-applets.json`, with no new Go and no profile field. Dinah's sections stay neutral, and a product's sections can mention only the product's commands and Dinah's.

**Point 8.** I agree, and I would make Dinah's own help use the same format, so a product's tutorial and Dinah's help are one kind of thing checked by one test.

I would also add a ninth point. The helper and the window Assistant should be one popover with one key, since Paul asked for an extension of the Assistant, and the Help menu would gain Ask for Help… that opens the same popover in its question form.

## 5. Rough cost

Most of the work is small or medium; the one large part is writing the help (Table 3).

**Table 3** *Size of Each Part*

| Part | Size |
|---|---|
| Help section format, registry and Working with Chats drawn from it | Small |
| Dinah's own help sections, about 20 to begin with | Large |
| Accuracy test for command ids | Small |
| Section choice by rule, and the rule's answer | Small |
| Help judge kind with its check and calibration cases | Medium |
| Answer view in the Assistant popover | Medium |
| Show Me, including opening a menu at a row | Medium |
| Do It | Small |
| Ask About This on error cards | Small |

*Note.* Sizes are the author's estimate. Small is under a day of one chat's work, medium a few days, large a week or more, most of it writing and review.

Each question would send the bare overhead, the question, a few lines of context and about three sections of help, roughly 1,500 to 3,000 tokens of input, and receive a reply of under 200 tokens. Through Claude that is about 2,500 to 4,000 tokens a question; through Codex about 11,000 to 13,000, since its bare session is larger; through Ollama a few thousand with no charge. Sending the whole help each time would stop being cheap once the help passes about 20 KB, which is why the rule chooses sections first. With judgements off, a question costs nothing and the person gets the matched sections quoted.

## 6. What Paul would need to see as a mockup

The mockup would be a clickable PUDL page with these screens and states:

- The Assistant popover in its question form, reached from Ctrl+Space and from Help, Ask for Help…, and how a question is told apart from a window request.
- An answer with its reply, the sections it came from, Show Me and Do It.
- Show Me on a control in a window, and Show Me on a row of a closed menu, with nothing on the screen moving.
- Do It disabled, with the command's reason beside it.
- The "not covered" answer, which says so plainly and offers the nearest sections.
- The answer when judgements are off, showing the matched sections quoted.
- An error card under Notifications with Ask About This, and the answer it leads to.
- A follow-up question after an answer.

## 7. Questions for Paul

1. Should help live in the existing Assistant popover, with Help, Ask for Help… opening it in question form? I recommend one popover and one key, as an extension of what exists.
2. Should the writing of Dinah's help come first, with the helper built only once about 20 sections exist? I recommend yes, since without the help nearly every answer would be "not covered".
3. Should the sign after a failure be only Ask About This on the existing error card? I recommend yes, so that nothing new appears or moves when something fails.
4. Should help questions spend the same judgement budget as the other kinds, and fall back to quoted sections with judgements off? I recommend both.
5. Should a follow-up question carry the previous question and answer, and nothing earlier? I recommend one previous exchange only.
6. Should Do It be offered only for commands that need no further input and are not marked dangerous, with Show Me alone for the rest? I recommend yes.
7. Should the first version leave out what is selected? I recommend leaving it out, since a selection is usually content and the person can paste it into the question.
