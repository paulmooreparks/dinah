# A suggested reply in the prompt box

Status: Paul approved the mockup (`docs/mockups/reply-suggestion/index.html`) on 19 November 2026, and steps 1 and 2 below were built that day. Step 3, the measurement, is not done. The kind is off until it is turned on in Settings, Judgements.

Paul asked on 19 November 2026 for a feature he has seen in Claude Code. A judgement agent reads the most recent message of a chat, or more if that works better, and proposes his reply in the prompt box. The proposal looks like the box's placeholder. Tab makes it the box's text, with the caret at its end.

## What is already there

The judgement agents are built (`docs/plans/judgement-agents.md`, `internal/judge`). A new kind of judgement is declared in one place with its prompt, the shape of its input, the form of its answer, a deadline and a fallback rule, and it then has a switch under Settings, Judgements, a record of every call, a weekly budget and a calibration. A chat's title is made this way today.

The prompt box has no suggestion of any kind, and nothing is drawn over it. Tab in the box moves to the next control, except while the list of skills or of mentions is open, where it picks from the list.

Go can read the end of a chat's saved conversation without the page, and already does so at the end of a turn to copy a team member's answer to its lead.

## The design

### What the person sees

The mockup shows all of it and can be used.

- A suggestion appears over the empty prompt box in the placeholder's colour, a moment after a turn ends. Nothing else moves or changes when it arrives.
- Tab makes it the box's text with the caret at its end. Nothing is sent.
- Esc drops it. Typing hides it, and emptying the box before the next turn brings it back. A new turn drops it, since it is out of date.
- A suggestion longer than the box's two lines is cut to them, so the box does not grow when it arrives. Tab gives all of it.
- When the judge proposes nothing, the box is as it always is.
- A small Tab key cap in the box's corner says which key takes it.

### Rules it keeps

1. **The suggestion is never the box's text until the person takes it.** The box's text is saved as a draft, goes into the prompt history, enables Send and is read by the check for steering. A suggestion is drawn over the box and held apart from all of that.
2. **Nothing moves when it arrives.** It lies over the box and takes no room. It is not set as the placeholder, because the box sizes itself to its placeholder when empty and would jump taller.
3. **It never sends.** Only Tab takes it. Enter over an empty box does nothing, as today.
4. **A judge never interrupts**, which is the seventh rule of the judgement agents. The suggestion shows only in a box the person is already looking at.
5. **It is off the path of sending and of typing.** The call is made after the turn has ended, in its own task, and is never waited for.

### When a suggestion is made

A suggestion is made when a turn ends in a chat that has a window, and the chat is then waiting for the person: no queued message, no turn starting straight after, no approval card open. A chat with no window gets its suggestion when its window opens, if its last turn has none yet.

No suggestion is made for a chat that is a member of a team under a lead, since its turns are answered by the lead and not by the person, and none in a group chat's window in this first version.

### What the judge is given

The first version gives it the person's last message and the agent's last message, each cut to a fixed length from its end, with applet contexts taken out as every judgement's input is. Whether more of the conversation gives better suggestions is a measurement to make and not a guess: the kind has a calibration set, and each suggestion's record says whether it was taken, dropped or ignored, so the rate at which suggestions are taken can be compared before and after the input changes.

The answer is a short text of a few lines at most. An empty answer means "propose nothing", and that is also the fallback rule, so a judge that is late, off, over its budget or wrong in form shows nothing.

### What it costs

One call is made for each turn that ends in a chat with a window. With the default judge, a local model in Ollama, a call costs nothing and takes about a second. With Claude Code or Codex as the judge, a call was measured at about 41,000 and 27,000 tokens, nearly all of it the agent's own instructions to itself, so a suggestion at every turn would be costly there. The kind is therefore off until it is turned on, its description says what it costs on a hosted judge, and the weekly budget applies to it as to every kind.

Every call is kept in the judgements' record for 90 days with its input, which is the end of the conversation. That is the same record a chat's title has, and the Judgements page shows it.

## Alternatives

- **The suggestion set as the placeholder.** It is the smallest change and looks exactly like the placeholder. The box would grow when a long one arrived, and a skill's form already uses the placeholder for its own words.
- **Enter also sends the suggestion**, as Claude Code has it. One stray key would send a message the person did not write.
- **A suggestion only when the person asks**, by a key in the empty box. It costs nothing until asked for, and it loses the point of the feature, which is that the reply is already there.
- **Two or three short replies in a row above the box.** It gives a choice and costs a row of the chat.
- **The chat's own agent proposes the reply** at the end of its turn, with the whole conversation in mind. It would be the best informed. It would make every turn longer and dearer, it would need each agent program's cooperation, and the agent would be proposing the answer to its own question.

## What was built, 19 November 2026

Paul's answers to the three questions: the mockup is right, with the Tab key cap; a suggestion is made for every chat with a window; group chats and team rooms are left out.

- **The kind** is `reply-suggestion` (`cmd/dinah-desktop/judge_reply.go`). The judge is given the person's last message, 600 characters from its start, and the agent's answer, 2,000 characters from its end. It answers with JSON holding one field, the reply, and an empty reply proposes nothing, which is also the rule. A reply over 400 characters or 6 lines is refused. The deadline is 10 seconds. It has three calibration cases.
- **The page asks, and Go answers.** The page knows which chats have a window and whether one waits for the person, so it asks (`SuggestReply`) when a turn ends of itself in a chat with a window, and once when a window shows a chat that is idle. A turn the person stopped or steered, or one that ended in an error, asks for nothing. A chat that reports to a lead asks for nothing.
- **One call for a turn.** Go looks in the judgements' record first. A suggestion already made for the conversation as it stands is given again from the record, so a window that is closed and opened, or a program that is restarted, makes no second call. While the kind is off, or no judge is set, nothing is asked and nothing is recorded.
- **What became of it** is kept beside the call's record (`internal/judge/outcomes.go`): taken (Tab), dropped (Esc), or passed over, which is recorded when the person's next message starts a turn. The first outcome stands. A suggestion that was taken or dropped is not offered again for the same conversation.
- **On the page** (`chat-applet.js`, "A suggested reply") the suggestion is kept on the session and drawn over the empty box. It lies in the box's cell of the composer's row and is taken out of the flow, so the box is no taller for it, in a wide window and in a narrow one where Send is on a row of its own.

The browser suite `e2e/reply-suggestion.js` and the Go tests in `judge_reply_test.go` cover it.

Not built: a view of the outcomes. They can be counted (`judge.Store.Outcomes`), and the Judgements page does not show them yet.

## Steps

1. The judgement kind, with its calibration set, behind its switch. Go asks for a suggestion at the end of a turn and keeps the newest one for each chat.
2. The page: the suggestion over the box, Tab and Esc, and the record of whether each was taken.
3. A measurement of how often suggestions are taken, with the judge given two messages and then given more, to settle what the judge is given.

## Questions for Paul

All three were answered on 19 November 2026, as recorded above.
