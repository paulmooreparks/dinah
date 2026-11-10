# Proposal: How People Get New Claude Models

Dinah Development wrote this proposal on 10 November 2026 for Paul's decision. Nothing in it is built except the move of the pinned adapter to 0.101.0, which Paul asked for the same day.

## Why Fable 5.2 Was Missing

Dinah shows the Claude models that the Claude adapter reports, and the adapter reports the list it gets from the copy of Claude Code that npm installs with it. Dinah had pinned adapter 0.87.0, whose Claude Code is 2.1.287, a release from before Fable 5.2 existed. Paul's own Claude Code had updated itself to 2.1.317 and offered the model, so his terminal had it and Dinah did not.

A new model therefore reaches a person in four steps today. Anthropic releases a Claude Code that has the model. The adapter's maintainers release an adapter that depends on it, usually within a day. Somebody moves Dinah's pin and releases Dinah. The person installs the Dinah update and then presses Install for the adapter again in Settings › Agents, because the update leaves Claude marked Not Set Up until they do. The pin rule adds a wait of two weeks before the third step can begin.

## What Moving the Pin Showed

I compared what 0.87.0 and 0.101.0 report at the start of a session, and ran the live test (a real turn through an applet, then a reopened chat) on both. The live test's checks pass on 0.101.0, and Fable 5.2 is among its models. Three things that a person can see changed between the two releases, which are five weeks apart.

1. A new chat with no saved permission mode starts in Auto, where Claude decides what needs permission. It started in Manual before. The adapter chooses Auto when the person's Claude Code settings give no default mode, as an interactive Claude Code session now does. A person who has saved a default mode in Dinah is unaffected, and Paul has.
2. The session has a new option, Ultracode, which Dinah shows with the others.
3. The value `haiku` now means Haiku 5.5, and Haiku 4.5 has a value of its own.

The first change is why I do not propose following the newest adapter automatically. A release five days old changed how much a new chat may do before it asks, and nothing told the client.

## Routes

**The person's own Claude Code.** The adapter documents an environment variable, `CLAUDE_CODE_EXECUTABLE`, for running a Claude Code the person already has. Claude Code updates itself, so the model list would always be current with no release of anything. I tried it once to understand it. Adapter 0.87.0 with Claude Code 2.1.317 did offer Fable 5.2, and the session took 30 seconds to start where it normally takes about one, with the adapter reporting that Claude Code had not answered it in time. Each adapter release is built for one Claude Code release and nothing documents that another one works with it, so I do not recommend this route.

**The newest adapter, automatically.** Dinah would install whatever npm marks as latest. This is the fastest route. It also runs code that nobody has tried with Dinah, on every machine at once, a few hours after it is published. The permission change above would have reached everyone that way.

**The pin, moved by a Dinah release.** Dinah keeps one tested adapter and a Dinah release moves it, which is today's design. It is safe and slow. Each new model costs a Dinah release and, for a product built on Dinah, a merge and a release of that product as well.

**A newer adapter the person chooses.** Settings › Agents › Claude would show the tested release and the newest one on npm with its date. A person who wants a model today presses a button to use the newer one, and a second button returns them to the tested one. An adapter nobody has tried with Dinah can misbehave, and the way back is that second button.

**A published file that says which adapter is current.** Dinah would read a small file from its own release site that says which adapter suits each Dinah version, and install that one. A new model would then need a test and a one-line change, with no Dinah build. The costs are a request to the release site at every start, a file that can tell every installed Dinah what to install, and a table of which adapter suits which Dinah to keep correct.

## Recommendation

I recommend three changes, in this order.

1. Dinah installs the pinned adapter by itself after an update that moved the pin, and runs the previous one until the new one is in place. Today the update leaves Claude marked Not Set Up until the person presses Install.
2. The person can choose a newer adapter in Settings › Agents › Claude, as described above. Dinah offers only the release that npm marks as latest and leaves the preview channel alone, and a chat's problem report includes the adapter's version. This is the change that gives a person a model on the day the adapter has it.
3. The pin moves when a release is one week old and has passed the live test, and a new model is a reason to move it as soon as both are true. Two weeks was chosen so that other people find a bad npm release first. One week keeps most of that protection, and the person who cannot wait has the button.

I would leave the published file until the pace of Dinah releases proves to be the obstacle. With the button, the people who need a model today have it, and everyone else gets it with the next Dinah release.

Dinah should also set a new Claude chat's permission mode itself when the person has saved none, so that an adapter release cannot change it again. Which mode that should be is the second question below.

## Questions for Paul

1. Do you approve the three changes in the recommendation?
2. Should a new Claude chat with no saved mode start in Manual, as it did before 0.101.0, or in Auto, as Claude Code itself now does? I recommend Manual.
3. Is one week acceptable as the age of a pinned release?
