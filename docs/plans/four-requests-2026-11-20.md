# Four requests of 20 November 2026

Paul asked on 20 November 2026 for four things: Google Gemini as an agent program, an assistant for the window itself that is reached from the tray, a first-time set-up that an agent guides, and judgements that set themselves up. This document says what each would be, what stands in its way, and what Paul has to decide. Nothing in it is built.

The four share one piece. The set-up guide, the judgement set-up and the window assistant are all a small agent that talks to the person and acts on the product through its applet actions. I propose to build that piece once, as the window assistant, and to have the other two be conversations it holds. The order below follows from that.

## 1. Judgements that set themselves up

Judgements are switched on from the first start, and they still do nothing, because no model is chosen. The status bar says "Off, no model chosen", and the person has to find Settings, pick a provider, and run a calibration before anything happens. That is the whole of the discoverability problem.

I propose this.

- At the first start with a default provider that can judge, the product chooses that provider and its default model as the judge, at once, with no question asked. The status bar then says "On".
- Calibration, which compares the provider's models on the kinds' own cases and picks the cheapest that passes, runs by itself in the background the first time the window is idle. Until it has finished, the default model judges.
- The first time a judgement visibly does something for the person, such as a suggested reply or a chat's title, a single notice says what did it and offers Judgement Settings. It appears once.
- The budget starts at a modest limit and not at "no limit", so that a person who never opens the page cannot be surprised by a cost. The limit is shown in the notice.

What it costs: judgement calls spend the person's allowance from the first day without their having asked. The budget and the one notice are the answer to that. For a provider billed per token through an API key the product should not turn itself on, and should ask instead.

Decisions: question 1 and question 2 below.

## 2. The window assistant

The assistant is a small chat that knows the window and can act on it. It is opened from the tray and from a key, takes a line of text, and has a few standing requests as buttons above the line: Show Only Active Windows, Tile the Active Windows, Show Everything About…, Close What Is Finished. Its answer is the action carried out, and one line that says what it did and offers Undo where the action can be undone.

It would be built on what exists.

- The product already publishes its windows and their commands as applet actions (`dinah.windows/list@1`, `dinah.windows/commands@1`), with settings, chats, documents and teams beside them. The assistant is an agent given those actions and no others: no shell, no files.
- It runs on the default provider and model, as one hidden chat that is kept warm, so a request does not wait for a session to start.
- Layout actions that do not exist yet are added as applet actions, since any agent should be able to ask for them: show or hide a set of windows, tile, bring a set to the front, save and restore an arrangement. A request such as "all windows related to" one ticket or project is the model's job: it reads the list of windows with their titles, workspaces and sources and chooses the set.
- Every action it takes is one the person could take by hand, so nothing it does needs a permission prompt. Anything that removes or sends something is outside its actions.

What it costs: one more warm session, and a turn of the default model for each request. Standing requests that need no judgement, such as Tile the Active Windows, are carried out directly with no model call.

The form of it needs a mockup before any code, by the standing rule. Decisions: questions 3 and 4.

## 3. A first-time set-up that an agent guides

The guide is the window assistant holding a set conversation at the first start, in place of a form. It asks what the person will use the product for, in two or three questions with choices, and from the answers it sets what Settings holds today: the default provider and model, the approvals mode, the window mode, the person's name, the workspaces to add, and which panes are open. It finishes by showing what it set, each line with a link to the setting.

Two things limit it.

- It needs a working provider before it can speak. The first step therefore cannot be the agent: the product has to find the installed agent programs, say which are signed in, and help with the one that is not, with no model at all. That part is an ordinary page, and it exists in part as Settings, Agents.
- It must be skippable in one press and repeatable later from Help. A person who skips it gets the defaults of today.

A product built on this one supplies its own questions and its own defaults for the guide through its profile, as it supplies its providers today.

Decisions: question 5.

## 4. Google Gemini

A fact has to be settled before a design. Dinah hosts an agent program that speaks the Agent Client Protocol, and Google's Gemini CLI does: `gemini --acp` is documented, with new and loaded sessions, approval modes, a model choice marked unstable, and hooks. But the Gemini CLI's own README says that free individual access and Google AI Pro and Ultra access "moved to Antigravity CLI on June 18, 2026", and that the Gemini CLI is now for Code Assist Standard and Enterprise. I have not read the Antigravity CLI's documentation, and I do not know which of the two Paul, or the people he means these products for, can sign in to.

There are three ways to have Gemini.

- **The Gemini CLI over the protocol.** It is the least work, since Devin is hosted the same way: a row in the provider registry, a module for its executable and sign-in, and a check of what its `initialize` offers. It serves only the people the CLI still serves.
- **The Antigravity CLI over the protocol**, if it speaks it. This is unknown until its documentation is read.
- **Gemini's API with Dinah's own agent loop**, as Ollama is done. It works for anyone with an API key and depends on no CLI's fate, and Dinah's tools, approvals and steering come with it. It is the most work, and it bills per token.

What the documentation leaves open for the CLI, to be read from the installed program before anything is promised: which sign-in methods it offers a host, whether it sends its thinking, whether it takes pictures, whether a session can be closed, and whether a turn can be steered.

Decisions: question 6.

## Order

1. Judgements that set themselves up. It is small, it needs no new piece, and it removes a fault that is in front of every new person.
2. The window assistant: the layout actions first, then the mockup, then the assistant.
3. The guided set-up, on the assistant.
4. Gemini, as soon as question 6 is answered. It depends on none of the others and can go beside them.

## Questions for Paul

1. May judgements choose the default provider and its default model by themselves at the first start, with a starting budget and one notice, when the provider is used through a sign-in and not through a key billed per token?
2. What should the starting budget be? I propose a tenth of what a day's ordinary use of the provider costs, as a number of tokens or units for each provider, which I would work out from the judgement records of the last two weeks and bring back before building.
3. Should the assistant's standing requests be the four named above, or others?
4. Should the assistant be reachable only from the tray and a key, or also have a place in the window, such as a line in the title bar's launcher?
5. Should the guided set-up start by itself at the first start, or be offered as a button on the start page with the form of today kept beside it?
6. Which Gemini do you mean, and which can you sign in to: the Gemini CLI with a Code Assist licence, the Antigravity CLI, or an API key?
