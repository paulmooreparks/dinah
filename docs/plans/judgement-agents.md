# Judgement Agents

Paul asked on 30 October 2026 (messages 795, 800 and 801) whether Dinah could keep a small agent running in the background to make judgement calls, configurable by the person and documented. He asked for this document, and for it to cover growing and shrinking the number of these agents, to explore where they would be useful, and to show the person what they cost and what they decided through a graph that is always in view or easy to reach. His reasoning is that Dinah presumes agents are always available, local or cloud, so it may as well use them in place of the heuristics, hard-coded values, magic numbers and lookup tables that classic software is full of. He also suggested giving the agent minimal context and a short orienting system prompt. If it works in Dinah, his other projects can take it too.

This is a proposal for Paul's decision. Nothing here is built.

## What a Judgement Agent Is

A judgement agent is a model that Dinah asks a small, self-contained question and whose answer Dinah uses in place of a rule. It is not a chat. It has no history, no tools and no workspace, and it never talks to the person. Each call carries a short system prompt that tells it what Dinah is and what form to answer in, and a small input whose shape is fixed for that kind of call. Engine proposed, and I agree, that every kind of call declares its input as a named structure. For a chat's title the structure is the first line of the person's words and nothing else. The list of structures is therefore the complete list of what a judgement agent may ever see, and it can be reviewed like any other part of the design.

Minimal context is the right default for more reasons than speed. A call that stands alone can be repeated, compared and shown to the person exactly as it was made. A small model's time goes mostly on reading its input, so a short input is a fast call. And a judge cannot leak what it was never given. This week's title leak, where whole prompts with their private applet contexts became chat titles, would not be possible for a judge that only ever sees one line.

## The Rules It Is Held To

1. **Every call has a deadline and a rule behind it.** The rule that Dinah uses today stays in the code as the fallback. When the judge is late, absent, turned off, or answers in the wrong form, the rule's answer is used, and the record says so. A slow or missing model can lower the quality of a decision but can never stall or break the program.
2. **It judges and does not establish facts.** A judge's answer is a reading or a matter of taste, such as which of three turns matter least. It is never a fact about an external system. The cloud-model question earlier tonight is the example. A model's guess about which models run in Ollama's cloud is no more documented than the name, so it could not have settled that question, and the rule against relying on undocumented behaviour applies to judges as it does to code.
3. **It never authorizes anything.** Permissions, applet consent, file access, approvals and anything else that grants authority stay with the person and with fixed rules. A judge's output can choose among options that are all already allowed; it cannot widen what is allowed.
4. **Measured thresholds stay measured.** The 80% note and the 90% hold in the Ollama context window, the documented file limits and the documented picture sizes are measurements or published limits, not judgement, and they stay as they are. A judge may decide what to do once a threshold is crossed, such as which turns to set aside, but not where the threshold is.
5. **Every answer is checked before it is used.** The answer must parse as the declared form and must satisfy the rule's own constraints. A set-aside answer that includes the current turn, for example, is refused and the rule's answer is used. This matters, because one of the small models measured below did exactly that.
6. **Every decision is visible.** The record of each call keeps its input structure, its answer, the deadline, how long it took, its cost, and whether the judge's answer or the rule's was used. The person can see the record, and can correct a decision where correcting it makes sense.

## How Fast and How Good

I measured this on Paul's machine (an RTX 3070 Ti with 8 GB) on 30 October, with thinking off, a 60-token answer cap and a two-line system prompt, three runs of each. The two tasks were choosing which members of a group should answer a message with no mentions, and choosing three turns to set aside from a list of eight short summaries.

| Model | First call | Later calls | Routing answer | Set-aside answer |
|---|---|---|---|---|
| nemotron-3-nano:4b (local) | 11 to 24 s | 0.5 to 1.0 s | wrong (another project's lead, twice) | varied, once included the current turn |
| qwen3.5:4b (local) | 10 to 19 s | 0.5 to 1.4 s | alternated between Engine and Front End | varied run to run |
| llama3.1:8b (local) | 14 to 24 s | 0.8 to 3.0 s | every member, every time | mostly sound, inconsistent form |
| deepseek-v4.1-flash:cloud | 0.7 to 1.3 s | 0.7 to 2.1 s | Engine and Front End (sound) | consistent (6, 3 and one early turn) |
| gpt-oss:120b-cloud | 0.6 to 1.2 s | 0.6 s | empty | empty |

Several things follow. The first call to a local model is slow because Ollama loads it into memory, so a local judge has to be kept loaded, which costs memory the person's own chats may want. Once loaded, a 4B model answers in under a second and a half, which is fast enough for anything that runs beside the work and too slow for anything on the typing, drawing or sending path. The small local models wrote acceptable titles but routed badly and inconsistently, so a 4B local model is good enough for titles and not for decisions that send work to someone. The fast cloud model was quick and sensible on both tasks. gpt-oss returned nothing, because it is a reasoning model that spent its whole token cap thinking; a reasoning model is the wrong choice for this unless it is allowed to think, which costs time. The variation between runs means a judgement should be made once and recorded, rather than asked again each time it is needed.

## Growing and Shrinking

A judge is a pool, with a smallest and a largest size that the person sets, and an off switch. What an instance is depends on the provider. For Ollama it is a slot in the server's parallel requests, since Ollama serves several requests from one loaded copy of a model, and the number of slots is the server's own setting (`OLLAMA_NUM_PARALLEL`). For an agent provider reached through ACP, such as Claude or Codex, an instance is a session in its own process, and Engine measured those at about 270 MB each earlier this week.

The pool grows when calls wait long enough that they would miss their deadlines, by one instance at a time up to the largest size, and it shrinks after an instance has been idle for a while, down to the smallest. Calls are queued in order with their deadlines, and a call whose deadline has passed while it waited is answered by its rule without being sent. That keeps the pool from ever working on an answer nobody can use. The figures for "waiting too long" and "idle for a while" are themselves the kind of number this design is trying to avoid, so I propose that they be stated as fractions of the calls' own deadlines rather than as fixed times, and shown in the view below so the person can see them working.

## What the Person Sees

A widget in Dinah's tray is always in view (Paul's answer to the third question below). It shows whether the judges are on, how many calls they made today and what share fell back to the rule, and it turns to a warning when the fallback share rises or a judge stops answering. Pressing it opens the Judgements page, which has its own address like every other page, in keeping with the architectural principles. The page has a graph over time of calls, fallbacks, time taken and cost, by kind of call and by model. Below the graph is the list of decisions, each with its input, its answer, whether it was used, and where the decision went into effect, such as the chat whose title it set. Where a decision can be corrected, such as a title or a routing, the list offers the correction, and corrections are kept so that the person can see how often the judge was overruled. Settings has the judge's provider and model, the pool's sizes, the off switch, and for each kind of call whether it is on.

## Where It Could Be Used in Dinah

These are the places I found where Dinah uses a rule, a cut-off or a fixed recipe for something that is really a matter of judgement. They are in rough order of how much I think a judge would help.

1. **Chat titles.** A title is the first line of the person's words cut at 120 characters. A judge would write a short title from that one line. A small local model is good enough for this.
2. **Which group members a message is for.** A message with no mentions from another chat wakes nobody, and one from Paul wakes everyone. A judge could suggest the members a message is really for, shown to the sender as a suggestion and never used to keep a message from a member it was addressed to. The measurements say this needs a better model than a small local one.
3. **The summary in a compaction or a provider handoff.** Today it is a fixed recipe (a deterministic summary plus the most recent turns, up to the provider's budget). A judge could write the summary, with the recipe as the fallback. This call needs more input than the others, so its input structure is the one that most needs review.
4. **Which turns to set aside when a window fills.** Today the oldest whole turns go first. A judge could choose the turns that matter least to the current work from one-line summaries of each turn, with the current turn always kept and oldest-first as the fallback. Its answer would be checked against the measured costs so that enough room is made.
5. **Which chats to keep warm.** Dinah reopens the four most recent group chats in the background at start, and closes a background chat after thirty idle minutes. A judge could choose from the recent activity which chats are likely to be needed.
6. **Grouping and wording of notifications.** A judge could group related notifications and write the line that says what happened.
7. **What a failure means.** When an agent's process ends or a call fails, Dinah shows the error. A judge could add one plain sentence about what the person can do, beside the error itself and never in its place.

## Paul's Ideas for Use Cases

Here are some places I could see this being used. Let me know if these are workable.

- Ordering of buttons on the task bar. I often wish Dinah could just figure out what buttons I'm likely to work with and which ones are likely inconsequential and order them accordingly. Putting new windows at the right puts them into overflow where they're difficult to find, and putting them at the left pushes stuff at the right into overflow. Some judgment here might be useful, but it might happen with some lag. 
- Command suggestions. Reading the users' input, at least as periodic samples since real-time is likely unworkable, and offering the next likely command. In order to build a good profile we'd probably need to build context based on the user's actions, and that might be prohibitive.
- Suggesting the right model or class of model for a given task or category of task.
- Suggesting the name of an untitled document.
- Code completion.
- Cache evictions.

Anything like the above is what I'm going for. It's like a good surgeon's assistant, being able to read the surgeon's mind and hand over the right instrument at just the right time because of having worked together and worked in the domain for so long.

What I \*don't\* want to do is reinvent Clippy. "It looks like you're designing an AI harness..."

## Managing and Configuring

Scaling horizontally is the obvious play. Having at least one agent spun up is a given, but as demand increase we need to add and prune them judiciously. Should we consider scaling vertically, introducing more powerful models or a bit more context or a bit more thinking?

What about the model's temperature? Do we want more predictable judgment than a typical model?

The user may want to configure the token budget available to these agents over a given period of time (say, a week). That's probably a better scale than "No more than \*X\* agents at any given time".

## Architecture's Reading of Paul's Ideas

Paul's test, the surgeon's assistant who hands over the right instrument and never says "It looks like you're designing an AI harness", gives this design a seventh rule. **A judge never interrupts.** Its work appears only where the person is already looking or already choosing: the order of things in a list, the first entry in a picker, a suggested name in a box that is already open. It never opens anything, never pops up, and never speaks first. Everything below is held to that.

- **Ordering the taskbar's buttons** is workable and a good fit, because a little lag does no harm. The input is the list of windows with their kinds, titles, and when and how often each was focused, and the rule behind it is the order they have today. The danger is a button that moves while the person reaches for it, so the order changes only when a window opens or closes or after the bar has been left alone, the focused window and pinned ones never move, and the buttons the judge thinks least likely are the ones that go into overflow.
- **Command suggestions** are workable in one form. Reading the person's typing in samples would be both slow and the Clippy shape. When the person opens the command or skill picker, a judge can put the likely commands first, from a short record of what was run recently and where. That record is counts and names of commands, not the person's words, which keeps the context small and keeps it out of what the judge is allowed to see.
- **Suggesting a model for a task** is workable and useful, on one condition from the second rule. What each model can do (pictures, context length, price, thinking) stays a fact from the provider table and from what the providers report. The judge only chooses among models those facts allow, and its choice is the first entry in the model picker when a chat starts or is handed off, not a change made for the person.
- **Naming an untitled document** is the same call as a chat's title and should be built with it.
- **Code completion** is not a fit for this pool. It sits on the typing path, where even the 0.5 s measured here is too slow, and it is a field with its own specialised models and protocols. If Dinah wants completion, it is a separate feature, and I would leave it out of this design.
- **Cache evictions** are workable only where an eviction is a preference rather than a correctness question. Choosing which closed chats keep their history in memory, or which old attachments to offer for removal, is preference, and least-recently-used stays the fallback. The caches that correctness rests on, such as the event ring that a resync reads, keep fixed rules, because a wrong guess there loses events.

## Managing and Configuring: Architecture's Reading

**Scaling up as well as out.** Yes, and the record makes it measurable instead of guessed. Each kind of call starts on the pool's model. When an answer fails its check, the call may be asked once more of a larger model if the deadline allows, and the record keeps both answers. When the record shows a kind of call falling back or being corrected often, the widget says so and offers to move that kind to a larger model or to allow it some thinking, and the person decides.

**Temperature.** Yes, low. A judgement should come out the same way twice, and the runs measured above varied at the default temperature. Ollama documents `temperature` and `seed` as request options, so for Ollama a judge runs at temperature 0 with a fixed seed. Agents reached through ACP document no temperature setting, so a judge on such a provider runs at that agent's own setting, and the record notes it.

**A budget over time instead of a cap on agents.** This is the better scale, and it matches the Claude Max pacing in Andoneer's requirements. The person sets a budget for a period, a week by default, for each provider the judges use. Dinah spends it evenly across the period, so the budget is never spent by Tuesday, and when it is spent every call falls back to its rule until the next period. The widget shows what is left. The pool's size then follows from demand, within the budget and within the memory the machine can spare, and needs no fixed number of its own.

**Choosing the model without a hard-coded name.** Paul's answer below rules out naming a default model, and rightly, because the models a person has depend on their providers and plans and change over time. Dinah can choose by measurement instead of by guess. At setup, and again when the models the person has change, Dinah lists the models available from what the providers report, with their facts. The first model found is asked which few of them are likely to suit short judgement calls, which is the bootstrap Paul suggested. Those candidates then run a small fixed set of calibration judgements whose right answers are known, and the one that answers correctly and fastest becomes the judge. The calibration set is part of the design and can be reviewed like the input structures. The person can choose a model before any of this runs, and can override the result at any time, and either choice stands until they change it.

**The widget.** It goes in Dinah's own tray at the right of the bar, as Paul said, with a glyph and a colour for its state, in the PUDL manner. Clicking it opens a popup with the day's summary and the budget. The popup is a summary only. The graph and the list of decisions are a page with its own address, and the configuration is a section of Settings, both reached from the popup, so that no state lives only in a dialog.

## Questions for Paul

1. Which model should be the default judge? I recommend a fast cloud model such as deepseek-v4.1-flash:cloud for everything, because it was quick and sound on both tasks, with a small local model as an option the person can choose for titles alone when they want nothing to leave the machine. Paul's answer: It depends on the provider(s) that the user has installed and the models available in the plan the user has. Perversely, this is exactly the kind of question I'd want to hand off to an agent, so it's a chicken-and-egg thing almost. Anyway, if we can know the correct model for a given provider (and that will change over time), then we use that. If more than one model is in play, maybe that's part of the setup. Maybe we ask the first model we find in the provider and bootstrap. However we do it, the user should be able to override a guess that Dinah made with a later configuration change or pre-empt that guess with an initial configuration.
2. Which uses should be built first? I recommend titles, then the compaction summary, then the set-aside choice, because each has a clear rule to fall back to and a decision the person can see and correct. Group routing comes later, once the record shows how often the judge is right. Paul's answer: Yes. Consider some of my ideas above as well.
3. Is the status bar the right place for the indicator, or would you rather it sat beside the usage meter? Paul's answer: Maybe it's a widget in the tray, and it can give some passive indication via glyph and colour, more details in a popup on click, and even more details and configuration via a settings dialog available from the popup.

## Further Questions for Paul

4. Is the calibration approach to choosing the judge's model acceptable: the first model found suggests candidates, and a fixed set of known judgements decides between them by correctness and speed?
5. Should code completion stay out of this design, as a separate feature if it is wanted at all?
6. What unit should the weekly budget be in? I recommend tokens for each provider, because a cost in money is known only for providers that report prices, and the widget can show money beside the tokens where it is known.
7. Should I send this document to the other projects' leads now for their lists of uses, or after the first uses are built in Dinah?

## Paul's Decisions

Paul answered the further questions on 30 October 2026 (message 817). The calibration approach to choosing the judge's model is accepted. Code completion stays out of this design. The budget is set for each provider in that provider's own unit, tokens for most and ACUs for Devin, so the unit is a declared fact in the provider table, with money shown beside it where the provider reports prices. The idea is proven in Dinah first, and the other projects' leads get the document once it is.

The first uses to build are the titles of chats and untitled documents, then the summary in a compaction, then the choice of turns to set aside, as recommended. Paul's other ideas (taskbar order, command suggestions, a suggested model) follow once those have a record, along with the suggested mentions in the group composer from [group-attention.md](group-attention.md).

## Build Order

Each step comes back for review with tests before the next begins.

1. **The call and its record (Engine).** A kind of call is declared with its name, its input structure, its answer's form, its check, its deadline and its rule. One function makes a call: it sends the input to the judge, waits until the deadline, checks the answer, and uses the rule's answer when anything fails. Every call is recorded, with its input, its answer, the time it took, its cost in the provider's unit, and whether the judge's answer was used. The judge runs on Ollama first, at temperature 0 with a fixed seed, because Ollama documents both. Tests cover a late judge, a missing one, a wrong form, an answer that fails its check, and a record for each.
2. **Titles (Engine and Front End).** Chat titles and the names of untitled documents become the first kind of call. The rule is today's cleaned first line, so nothing gets worse when the judge is off. The page shows a title the judge made, and offers to correct it, and a correction is kept in the record.
3. **The widget and the Judgements page (Front End).** The tray widget shows on, off or warning with a glyph and a colour, and its popup shows the day's calls, the share that fell back and the budget. The Judgements page has its own address and shows the graph and the list of decisions, and Settings gets a Judgements section.
4. **The budget and the pool (Engine).** A budget for each provider in its own unit, spent evenly across the week, with every call falling back to its rule once the budget is spent. The pool grows and shrinks within the person's smallest and largest sizes, by the fractions of the calls' deadlines described above.
5. **Calibration (Engine).** The fixed set of known judgements, the bootstrap question to the first model found, and the choice of the judge by correctness and speed, run at setup and when the person's models change. The person's own choice always overrides it.
6. **The compaction summary, then the set-aside choice,** each as a kind of call on the same foundation.

## Step 1's Shape

Engine proposed the shape of step 1 on 30 October (message 851), and Architecture approved it with additions (message 852). The work is a package of its own, internal/judge, with no dependency on the window, the bus or any chat. A kind of call is declared with a name, a short system prompt, an input structure whose fields are documented, an answer form, a check, a deadline and a rule, and one registry lists every kind with its input fields, so everything a judge may see can be printed and reviewed. One function makes a call. It never returns an error and never runs past its deadline, and it answers with the rule on any failure, including a deadline that passes while the call waits in the queue. The first judge is Ollama with thinking off, temperature 0, a fixed seed and an answer cap, behind an interface that other providers can fill later.

Each call is recorded in its own SQLite file with its kind and the kind's version (a hash of its prompt, input structure and answer form), the input the judge saw, the judge's raw answer, the rule's answer, which of the two was used and why, the judge's provider, model, temperature and seed, the time taken and the deadline, the cost in the provider's unit, where the decision took effect, and any correction by the person. The applet-context removal runs over every input before it is sent and before it is stored. Records are kept for 90 days by default, a setting the person can change. The widget and the page read the same file.

## Step 1 Built

Step 1 was built on 30 October 2026 in `internal/judge`, as Architecture and Front End reviewed it (messages 851, 852 and 854). A kind is declared with `judge.Declare` and called with `judge.Call`, and `judge.Kinds` lists every kind with its input fields, its rule in plain words and whether it can be corrected. The record is a row in `judgements.db`, and the store reads it as a page of records filtered by time, kind, whether the judge or the rule was used and why, model and what the decision was applied to, and as a summary with the calls, fallbacks, average and longest time, disagreements with the rule, corrections and cost by kind, model and unit, along with the latest call and the current run of fallbacks. Nothing calls it yet. The judge is not yet wired to a setting, so Dinah has no switch or model choice until step 2 needs one.

## Step 3, the Engine Side Built

The methods the widget, the page and Settings read and write were built on 30 October 2026, with the names agreed with Front End and Architecture (messages 861 to 863): `JudgementState`, `JudgementSummary`, `JudgementRecords`, `JudgementKinds`, `JudgementSettings` and `JudgementModels` as queries, and `SetJudgementSettings` and `CorrectJudgement` as work. The warning threshold is a setting, 3 by default, and the settings are saved against a revision. The event `judgements:changed` is numbered like the window's other state, and a save of the settings sends it too. Nothing calls a judge yet, so the page shows its empty state until titles land in step 2.

## Titles and Document Names

Engine's plan for step 2 was approved on 30 October (messages 871 and 873). A chat's title is the kind chat-title. Its input is the cleaned first line of the person's first message, at most 120 characters, and its answer is plain text of at most 60 characters with no Dinah marker or applet context in it. The deadline is 3 seconds, and the rule is today's cleaned first line. It runs once per chat, off the sending path, only when the person has not named the chat, and never for old chats or for a first message that is only files. The judged title is kept beside the rule's title and is shown until the provider gives a title of its own, and the person's own name always wins. A correction renames the chat. An untitled document's name is the second kind, document-name, offered as the suggested name in the Save As box from the document's first non-empty line, with "Untitled" as the rule.

## Step 2, Chat Titles Built

The chat-title kind was built on 30 October 2026 as Architecture approved it (messages 871 and 873). It runs when the person's first message in a chat starts its turn, off the sending path, and keeps a judged title beside the first line it was written from. Documents are the second kind of this step, built the same day: the Save As dialog asks `SuggestDocumentName` when it opens and fills its name box if the person has not typed, from the document's first non-empty line, with "Untitled" as the rule.

## Step 4: the Budget and the Pool

Step 2 was finished on 30 October with document names (message 887), offered through SuggestDocumentName when the Save As box opens. This section is the build order for step 4.

**The budget.** Each provider the judges use has a budget for a period, set in Settings › Judgements, in that provider's own unit. The unit is a declared fact in the provider table (tokens for most, ACUs for Devin), so the budget code asks the row and never compares a provider's name. A budget may be a number or none, as the file and round limits are, and none is the default, so nothing changes until the person sets one. The period is a week by default, and the person can set the day and hour it begins, so it can line up with a plan's own reset. The budget is spent evenly. At any moment the allowance is the budget times the share of the period that has passed, and a call is sent only while what has been spent is below the allowance, so an unused allowance carries forward within the period and the budget is never gone by its second day. A call's cost is known only after it returns, so one call may take the total slightly past the allowance, and the next calls wait for the allowance to catch up. A call refused for the budget is answered by its rule and recorded with the reason "budget". When a period ends, the spending starts again from zero.

**The pool.** Dinah's own limit on judge calls in flight at once, between a smallest and a largest number the person sets, one and four by default. For Ollama this is Dinah's side only, because the number of requests the server serves at once is the server's own setting (OLLAMA_NUM_PARALLEL), which Dinah does not change. The pool grows by one, up to the largest, when a call has waited in the queue for half of its deadline, and it shrinks by one, down to the smallest, after it has been idle for ten times the longest deadline of any kind. Those two fractions are written once in the code, named, and shown on the Judgements page beside the pool's size, so the person can see the pool working and why it changed.

**What the person sees.** The widget's popup and the page show the budget's period, what has been spent, the allowance at this moment and what is left, by provider and in its unit, with money beside it where the provider reports prices. The widget turns to a warning when calls are being answered by their rule because of the budget, and says so.

**Tests.** A budget of none never refuses. A call when spending is under the allowance is sent, and one over it is answered by the rule with the reason "budget". The allowance grows with the period and starts again at its end, with a clock the test controls. The unit comes from the provider's row. The pool grows when a call waits half its deadline and shrinks after the idle time, and never leaves its bounds. Each guard is broken on purpose once to see its test fail, and the full suite runs before the report.

## Step 4, the Engine Side Built

The budget and the pool were built on 30 October 2026 as Architecture approved them (messages 889 and 890), with one change to the plan: the pool's largest size has no ceiling and four is only its default. The settings are `budgets` (a row for each provider that can judge, with its unit read from the provider table and a limit that is a number or null), `period` (the weekday and hour it begins) and `pool` (smallest and largest). The state has `budget` and `pool`, and the two fractions the pool grows and shrinks by are sent with it. Money beside the amount is not built, because no provider that reports prices can judge yet.

## Step 5: Calibration

Engine proposed calibration's shape on 30 October (message 898), and Architecture approved it with one change and one addition (message 899). Calibration lists the models the provider reports, asks one of them which few suit short judgement calls (a declared kind, judge-candidates, whose answer may only use names from that list and whose rule is every name), warms each candidate once, and runs it through a small fixed set of calls with known right answers at each kind's real deadline. A model is chosen only if it answers every case correctly, and the fastest by median time wins. When none passes, no model is chosen and every call keeps its rule. Calibration calls are recorded, pooled and budgeted like any other. It runs by itself when no model is chosen and the person has not chosen one, once for each set of installed models, and when the person asks. The person's own choice is never replaced, and the settings say whether the person or calibration chose the model.

An automatic run never loads a model into memory on its own. It considers only cloud models, by the naming convention Paul accepted, and local models Ollama already has loaded (/api/ps), including the model it asks the bootstrap question. Loading each large local model in turn would push the person's own models out of memory and slow their work. A run the person starts may try any installed model, and its report says which ones it loaded. The fixed set grows with the kinds, and a test requires every declared kind to have at least one calibration case.

## Step 5, Calibration Built

Calibration was built on 30 October 2026 as Architecture approved it (messages 898 and 899), with the change that an automatic run never loads a model into memory: it considers only cloud models and models the provider already has loaded, and a run the person starts may try any. The fixed set is in `cmd/dinah-desktop/judge_calibration.go` and `internal/judge/calibration.go`, and the page can read it with `JudgementCalibration`. When routing arrives, its cases need right answers of their own, since that is where the small models failed in the measurements above.

## Step 6, the Compaction Summary Built

The compaction summary was built on 30 October 2026 as Architecture approved it (messages 902 and 903), starting off until the person turns it on. A kind now declares its input cap and whether it starts off, and the registry and the Judgements page show both beside the deadline. The calibration set has a case for it: eight lines of a chat whose early decision, to build the exporter as its own module called exportd, the summary must keep. The set-aside choice, the second half of the step, was built the same day and also starts off (message 908). The adapter validates the turns a judge chooses against the costs it recorded, so the validation stays where the record is. Its calibration case is seven turns, one of which records a decision to keep the API backwards compatible; an answer that sets that turn aside fails.

## Step 6: the Kinds That See a Chat

The handoff summary (handoff-summary, messages 902 to 904) and the set-aside choice (set-aside-choice, messages 906 and 907) were approved on 30 October. Each is shown one line from each of many of a chat's messages or turns, and never the newest ones, files or tool output, within an input cap of 12,000 bytes declared with the kind and shown beside its deadline of 10 seconds. The handoff summary replaces only the digest of older messages, within the quarter of the provider's handoff budget that the digest already takes, and the summary saved beside a chat stays the rule's, so a switch never needs a model. The set-aside choice picks which turns go instead of the oldest first. The Ollama adapter checks the choice again against its own recorded costs and refuses one that would set aside the latest turn, a turn whose cost is unknown, or too little, and the chat's note says which turns were set aside.

Paul decided on 30 October that both kinds start turned off, each with its own switch, so nothing of a chat's content goes to a judge until he turns it on.

## Judges on Every Provider

Paul pointed out on 30 October (message 929) that judgements support only Ollama, while most people will have only Claude Code or Codex. Ollama came first because it offers a plain chat request with no agent around it and documents temperature and a seed. The judge was put behind an interface for other providers, but none had been added. Paul decided (message 931) that this comes right after the workspace and switching round.

A judge on a provider reached through ACP, such as Claude Code or Codex, makes each call in a fresh session, so each judgement still sees only its declared input and no history. The session starts in an empty temporary folder with none of Dinah's tool servers attached, and Dinah, as the client, refuses every permission request the agent makes, so a judge can answer but cannot act. Its cost is the usage the agent reports, counted against that provider's budget in the provider's own unit. Such a judge is slower than Ollama because a session starts for each call, and neither agent documents a temperature setting, so the record shows the time each call took and calibration measures the speed against each kind's deadline. The judge's provider becomes a choice in Settings › Judgements among the providers the person has installed, and calibration runs over the chosen provider's models, with nothing hard-coded.


### Built So Far

Judges on Claude Code and Codex are built as designed above, with two changes Architecture made on 30 October (message 1002). The cost of a call is the agent's own usage report and is never estimated: an agent that reports none gives a call whose cost is unknown, kept as such, and the budget row for that provider says how many calls it could not count. The report is how full the session's context is, which includes the agent's own instructions, so a call on a hosted agent costs far more than the same call on Ollama; the figure from a real call is to be measured and given to Paul before he chooses a hosted judge.

The page has `JudgementProviders` for the choice of provider (whether each can judge now, and why not in words, with its unit and a note when its calls use the plan), `unmeasured` on each budget row, and the models of a hosted provider only when it is the chosen judge. Ollama remains the default judge.

Engine built judges on Claude Code and Codex on 30 October (message 1030) and measured a real chat-title call on each: about 41,000 tokens on Claude Code and about 27,000 on Codex, against about 15 on Ollama, nearly all of it each agent's own instructions to itself. Cost is the usage the agent reports, and an agent that reports none leaves the call's cost unknown and the provider unable to hold a budget, with nothing estimated. Paul decided the same day (message 1037) that a hosted provider is never calibrated unless he starts the run, and that choosing a Claude Code or Codex model as the judge opens a short confirmation giving the measured cost and whether a budget is set, with Cancel first, while choosing a local model asks nothing.
