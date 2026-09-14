---
title: The MCP surface takes the workbench as a parameter
column: b69abf918c42
state: ready
severity: major
priority: next
tier: frontier
workstreams:
  - fdfdeaaff2dd
---
An agent connecting over MCP sees exactly one workbench. `mcp.Serve` takes one already-opened library and holds it for the life of the process, no tool declares a workbench argument, and the handshake text opens "You are working the workbench <title>". So somebody working three workbenches configures three registrations, each pointed at its own directory or carrying its own `DINAH_WORKBENCH`.

The operator has ruled that this changes: a CLI invocation stays scoped to one workbench, and the MCP surface becomes callable for many. D-1 records the ruling and where the mistake actually sits, which is not in the core profile. The profile's boundary row rules out cross-workbench views in the protocol, a statement about cards and references; the tool then read that boundary as an architectural constraint on a long-lived server, and a server surface is not a process invocation.

What a spec has to settle. Whether every tool takes the workbench as an argument, which touches twenty-six schemas, or the head carries a current workbench, which puts state into a protocol that holds none today. How an agent learns which workbenches it may address, given that `workbenches` is absent from the MCP surface and the exclusion comment that names six shell-only commands does not name it. And what bounds the reach, because the present single-workbench scope is the only thing stopping a head from walking into the operator's own boards; that bound has to move into the code rather than resting on instructions an agent may not read.

## Specification

`dinah mcp` becomes a server over many workbenches. The cli head does not change. Every cli command still resolves exactly one workbench through `bench.DiscoverSource`, and `--root` is a flag of `mcp` rather than a global one.

## The workbench arrives as an argument

`schemaFor` in `internal/mcp/tools.go` injects a third property beside `actor` and `basis`, named `workbench`, of type `string`, carrying the description at the new catalog key `schema.workbench.description` and no enum. The rule for injecting it is uniformity with one exception rather than a principle with one instance. Every tool gets the property, including `version`, which acts on no workbench and gets it because one injection site is cheaper to hold true than a list of the tools that need it. `workbenches` is the one tool held out, since a caller asking what the property may say cannot already know the answer. The property is never required. A client that has never heard of it keeps working.

The head consumes the property. It does not copy it onto `verb.Request`. `actor` and `basis` tell a library something; this one chooses which library answers, so it is read in `call` in `internal/mcp/mcp.go` and goes no further.

No tool sets a current workbench, and the head holds none. Two calls arriving in either order therefore mean the same thing, so a client may interleave work on two workbenches without a mode to keep track of, and a call that reads badly in a log reads the same way when it is replayed.

## Resolving a call's workbench

Every tool call is resolved in the order below, and the first unsatisfied step names the refusal that comes back. Declaring the order is what keeps a request that trips two of these steps from having two answers and no rule.

1. A call carrying no `workbench` property, or an empty one, acts on the default workbench. When the server has no default, the call is refused `dinah.no-workbench-found`, carrying the directory the server's startup discovery began from and the user base it fell back to.
2. The value is resolved to an absolute path with `filepath.Abs`, against the server process's working directory.
3. A resolved path that does not lie inside the root is refused `dinah.outside-root`, carrying the resolved path as the detail and the root as the named value `root`.
4. A path carrying no `workbench.md` is refused `dinah.no-workbench`, which is the name the override branch of `bench.DiscoverSource` already raises for that condition.
5. `bench.Open` runs, and its own refusals travel unchanged.
6. The verb's own ordered check list runs, unchanged.

Steps 1 and 4 reuse two names whose English was written for a person who typed `--workbench` at a shell, and that English never reaches a caller here. An MCP answer carries `outcome`, `verb`, `refusal`, `detail`, and `context`, and it carries no sentence. `composeRefusal` in `cmd/dinah/render.go` renders the English, and it serves the terminal alone. Reusing the names therefore puts no false sentence in front of an agent, and minting a third name for a condition the contract already names would give one condition two names for the sake of copy nobody on this surface reads. What does change is the translator's note on each of the two entries, which says today that `{detail}` is what the caller named with `--workbench` or `DINAH_WORKBENCH`. Both notes gain the second raise site, so a translator reading the entry knows the sentence is the terminal's alone.

## What lies inside the root

A path lies inside the root when the root and the path name the same directory, or when the root names one of the path's ancestors. Both questions are settled by `os.Stat` on the two paths and `os.SameFile` rather than by comparing strings.

Every stat failure refuses. The containment test answers `dinah.outside-root` when the root will not stat, when the resolved path will not stat, and when any ancestor between them will not stat, for any reason at all, including a permission error, a disconnected volume, and a root deleted after the server started. The walk stops at the first ancestor it cannot stat. It does not skip the hole and climb on, because an ancestor chain with a gap in it cannot show that the root sits above the gap. A bound that fails open is worse than one nobody claimed.

Every stat the containment test makes goes through an unexported package-level `statPath` in `internal/bench`, which holds `os.Stat` and which a test replaces for the length of one test. Without that seam nothing can drive the failure the rule is written for. A test that creates the directories it compares never meets a failing stat at all, and the one failure it can produce without a seam, by removing a directory, is the not-exist case, which is the single case `samePath` already handles safely; a containment test built on `samePath` would pass a removed-root check and still admit every path the process cannot read. `readAnchorContent` in the same file is the precedent and its doc comment carries the reason, which is that how a platform reports an unreadable directory is not something this repository builds a check on. A test overrides the seam and restores it through `t.Cleanup`, the way `internal/bench/anchor_recognition_test.go` already overrides `readAnchorContent`.

`samePath` in `internal/bench/bench.go` is not reused, and the reason belongs in the spec rather than being left for the implementer to discover. `samePath` returns true when a stat fails for any reason but not-existing, and its doc comment says why: its only caller today is the upward walk, where true means stop climbing, so failing that way keeps a boundary the tool cannot check. On a containment test the same value means the opposite, since true means admitted, and reusing the function as it stands would admit every path the process cannot stat. The new code stats and compares on its own, and `samePath` gains one sentence naming the new function and saying that the two want opposite failure postures. A second parameterised helper is refused for the same reason: a flag deciding which way a safety check fails is the thing that has to be legible at the call site.

A string prefix test will not do. It fails whatever it normalises first. Windows hands out the 8.3 short form of a long directory name and macOS mounts case-insensitive volumes, so two spellings of one directory read as two.

## Opening a workbench per call

The head keeps a map from resolved workbench path to the `*verb.Library` opened for it, and reuses the entry for the life of the process. That is what the head does with its single library today, so a workbench whose anchor changes on disk after the server started reads as a snapshot on this surface both before this card and after it. `Serve` answers one request at a time, so the map needs no lock; a later card that answers concurrently adds one.

## What moves through the head

Four signatures widen and one gains a nil case, and naming them here saves the implementer a search.

`mcp.Serve` takes the root and the default library, where the default may be nil, and it owns the map of opened libraries. `dispatch`, `initializeResult`, and `call` each carry the root and that map alongside the library they take today. `workingAgreement` takes a library that may be nil and a root that never is, and it dereferences the library only inside the branch that has one.

The `tool` struct's `run` field stays `func(*verb.Library, *verb.Request) any`. `workbenches` acts on the root rather than on a library, so `call` answers it ahead of the table lookup that leads to `run`, which keeps the one tool that needs no library out of a signature widened for its sake alone. `workbenches` still holds a row in `tools` so that `toolList` renders it and the surface count reaches twenty-seven.

A refusal raised by step 1, step 3, or step 4 travels in the result rather than in a JSON-RPC error. `dispatch` turns an error out of `call` into an `rpcError` today, and the head's own comment above that type rules that out for a refusal, so `call` builds the refused response itself and marshals it through the same content wrapper a tool's payload goes through. What builds it needs saying too, because `FromError` and `refuse` are methods on `*verb.Library` and a server with no default has no library to call them on. `internal/verb` gains one exported function that composes a refused response from a request and a `*contract.Refusal` without a library, carrying the affordances `affordances` already returns where there is no card, and `FromError` delegates its refusal branch to it so the two cannot compose different shapes for one refusal.

## The root

`dinah mcp` takes `--root <dir>`, declared as `{Name: "root", Flag: true, Value: "dir"}` in `params["mcp"]` in `internal/verb/definition.go`, so the syntax line, the generated help page, and the parser's accepted set all follow from one declaration. `DINAH_MCP_ROOT` carries the same value. The two resolve through `bench.Resolve` with `bench.SourceFlag` first and `bench.SourceEnvironment` second, which is the ladder `--workbench` and `DINAH_WORKBENCH` already climb. Both spellings ship because a client writing a registration can set process arguments, environment, or both, and the workbench pointer already ships in both spellings for the same reason.

Naming neither leaves the root at the directory of the workbench discovery resolved at startup. Only that workbench lies inside it, so a registration nobody edits serves one workbench and behaves as it does today. The root is resolved to an absolute path with `filepath.Abs` before anything is compared against it, and a relative root resolves against the server process's working directory, which is the same directory a call's relative `workbench` value resolves against.

## What happens at startup

1. A root was named and no directory sits at the resolved path. `dinah mcp` refuses `dinah.unknown-root`, carrying the resolved path, and the process exits 2 without serving.
2. Discovery runs through `session.open`, as it does today, and a refusal from it lands differently depending on where the pointer came from. When no root was named, the refusal is reported and the process exits, byte for byte as it does today. When a root was named and an explicit pointer raised the refusal, meaning `--workbench` or `DINAH_WORKBENCH` named a directory holding no `workbench.md`, `dinah mcp` reports that refusal, which is `dinah.no-workbench`, and exits 2. When a root was named and the refusal came from any other rung, meaning an ambiguous base the walk met, a stale directory in the user config, or a walk that reached the root of the filesystem, the server starts with no default workbench and serves, and one line goes to stderr from the new key `mcp.no-default`, in the shape `runInit` already uses when it cannot record an actor.

   The head can tell the two apart without asking discovery twice. `session` resolves `--workbench` and `DINAH_WORKBENCH` into one override value and its rung before `open` runs, so a non-empty override is the explicit pointer this rule turns on, and `bench.DiscoverSource` raises `dinah.no-workbench` from that override branch alone.
3. A root was named and discovery resolved a workbench outside it. This case splits on the same line case 2 splits on, and `bench.ResolveWorkbenchSource` already returns which rung answered. A default that came from `bench.SourceFlag` or `bench.SourceEnvironment` is a contradiction whoever wrote the registration wrote down, since `--root` and `--workbench` were both set and they disagree, so `dinah mcp` refuses `dinah.outside-root`, carrying that workbench's path and the root, and exits 2. A default that came from any other rung, meaning the ancestor walk or the user config, is nobody's instruction, so the server starts with no default workbench and serves, and one line goes to stderr from `mcp.no-default` naming the workbench that was dropped.

   The split is what keeps a working registration working on a second machine. Startup discovery climbs from the server process's working directory. An MCP client chooses that directory. A bounded head that exited 2 over a walk nobody asked for would start on one machine and refuse on another for a reason written down nowhere. A pointer somebody typed has no such dependency, and refusing it while somebody is still reading stderr is worth more than serving a head whose default answers every unqualified call with a refusal. The two cases treat the same pointer the same way, so an explicit pointer that names nothing and an explicit pointer that names a workbench in the wrong place both stop the server.
4. A root holding no workbench at all is not a refusal. The server starts, the `workbenches` tool answers with an empty array, and every call naming a workbench is refused by the order above. `runWorkbenches` already rules that a question about what is reachable is answered by zero rows as truthfully as by several, and this head answers it the same way.

Three branches exit: case 1, and the explicit-pointer branch of cases 2 and 3. Every other branch ends in a server that serves and has no default, so each of those answers an unqualified call with `dinah.no-workbench-found` and each answers `workbenches`. `beyondChecks["mcp"]` carries `dinah.unknown-root` first and `dinah.outside-root` second, which is startup case 1 followed by startup case 3's explicit branch. `dinah.no-workbench` is not in that list, because it belongs to workbench discovery rather than to `mcp` and no other command's check list carries it either.

## Discovery

A twenty-seventh tool joins the surface. It is named `workbenches`, it is bound to the `workbenches` command, and that command's parameter list stays empty.

The exclusion comment above `tools` in `internal/mcp/tools.go` is amended rather than quietly contradicted. The rule stands: a command that exists only because a shell and a filesystem exist gets no tool. `workbenches` sat inside that rule while the head served one workbench, and it sits outside it now, because the workbench argument creates an address space and an address space an agent cannot enumerate is one it cannot use. The comment says that, so the next reader meets a rule with its exception in it rather than a list that no longer matches the table.

The `tool` struct gains a `summaryKey` field, empty on every other tool, and `toolList` prefers it over `cmd.<command>.summary`. `workbenches` sets it to the new key `tool.workbenches.summary`, reading "the workbenches this server may serve", because the command's own summary, "the workbenches reachable from here", is false of a head bounded by a root.

The tool answers `{"workbenches": [...], "affordances": [...]}`, where each row is a `bench.Candidate` carrying `title`, `slug`, and `path`, and `slug` keeps its `omitempty`.

Rows come back in ascending order by their `path` member, sorted after the walk rather than left in the order the walk met them. The two orders differ in ordinary trees: a walk reading each directory's entries in name order meets `plans` before `plans-archive`, while the two workbench paths sort the other way round, because the hyphen sorts ahead of the path separator. A reader may rely on ascending path order, and two calls in one session answer alike as a consequence of that rather than as the whole of the promise.

Enumeration walks downward from the root. The walk enters each directory under the root and reads that directory's `.dinah` container the way `benchIn` already reads one, it descends into no directory whose name begins with a dot, which keeps it out of `.dinah` and `.git` alike, and it follows no symlink. A directory it cannot read is skipped rather than failing the call. Where the root is the startup default, the walk finds the one workbench and the answer is that single row.

The answer names no workbench outside the root. It opens no workbench either, so no card, no operator name, no state list, and no instruction text travels in it.

## The handshake

`workingAgreement` takes the default library, which may be nil, and the root. With a default it keeps its opening line word for word and gains one paragraph from the new key `mcp.reach`, which takes `root` and `title`, and it keeps the operator sentence. Without a default it drops the opening line and the operator sentence, since neither has a subject any more, and prints one paragraph from the new key `mcp.reach.nodefault`, which takes `root`. Draft English for both keys sits in section 4 of the UX sketch on this card's branch.

## Refusal names and catalog copy

`dinah.unknown-root` and `dinah.outside-root` are declared once in `internal/contract/contract.go` beside their siblings, added to `contract.Introduced`, and spelled from those constants at every site. Each also needs an entry in `contract.Shapes` in `internal/contract/shape.go`, because `checkOneRefusalIsOneDeclaration` in `internal/profile/guards_test.go` fails over a refusal name no shape declares, and `checkEveryShapeSaysWhatToDoNext` requires the `.next` fragment each one carries. `dinah.outside-root` declares `root` in its `Values`, which is the named value its sentence and its `context` member both use. Their English text is in section 7 of the UX sketch, drafted as a pair so that a reader moving from one to the other gets the same class of guidance.

New keys: `schema.workbench.description`, `tool.workbenches.summary`, `param.mcp.root.summary`, `check.mcp.1`, `check.mcp.2`, `mcp.reach`, `mcp.reach.nodefault`, `mcp.no-default`, `refusal.dinah.unknown-root`, `refusal.dinah.unknown-root.next`, `refusal.dinah.outside-root`, and `refusal.dinah.outside-root.next`. Changed keys: `cmd.mcp.summary`, which becomes "serve workbenches over MCP on stdio", and `help.environment`, which gains `DINAH_MCP_ROOT`. The translator's note on `refusal.dinah.no-workbench` and on `refusal.dinah.no-workbench-found` gains the second raise site, which changes no rendered text. Every locale file under `internal/msg/locales/` carries every one of these entries.

`beyondChecks["mcp"]` in `internal/verb/checks.go` gains two rows in the order above, so `dinah help mcp` generates its refusal table from the declaration the behaviour follows.

Both check sentences fit the column the page gives them. `check.mcp.2` reads "the workbench you name lies under the root", which is 42 display columns, and 51 is the widest a check sentence can be before the renderer takes its refusal name off the row and gives the sentence a line of its own. Past 51 the page also runs over 80 columns, because the checks table is the one table in the tool that does not wrap its tail. The ceiling is measured rather than derived. The page renders inside 80 columns with a 51-column sentence and over it with a 52-column one. Every translation of `check.mcp.1` and `check.mcp.2` holds the same ceiling, and the translator's note on both keys says so.

## The ratified surface

`cmd/dinah/testdata/help.txt` changes on two lines. The `mcp` entry becomes `mcp [--root <dir>]` with the new summary beside it, and the environment line gains `DINAH_MCP_ROOT`. The list of valued flags asserted in `cmd/dinah/main_test.go` gains `root`.

## Tests the change moves

`TestToolSurfaceIsTheProjection` counts twenty-six tools and counts twenty-seven after this card, with its absent list unchanged.

`TestEverySchemaPropertyIsDescribedAndNoneCarriesAnEnum` counts a property as injected only when its name is `actor` or `basis`, and asserts that the count is twice the number of tools. That assertion holds untouched after this card, because twenty-seven tools still carry two such properties each, and the new `workbench` property would be absorbed into the described count without ever being counted. The guard therefore learns the third name: it counts `workbench` alongside the other two, and it asserts that the injected count is `3*len(listed.Tools) - 1`, the subtraction being the one tool that carries no `workbench`. Deleting the injection then turns the guard red instead of leaving it green.

`DINAH_MCP_ROOT` is a new ambient input, so the change that teaches the product to read it also closes it out of every test that never asked to see it. `TestMain` in `cmd/dinah` clears it for the whole package beside `COLUMNS`, and `newBench` clears it for each fixture beside `DINAH_WORKBENCH`. A developer with the variable exported then runs what CI runs, and the one criterion that cares about the variable sets it for itself.

`TestInitializeCarriesTheWorkingAgreement` reads the instructions text. `TestHelpBlockIsTheRatifiedSurface` reads the fixture that changes.

## What this does not touch

The core profile is not reopened, and a later reader should not have to work that out. Section 5.1 scopes the model to one workbench, and the section 10 boundary table rules out views across several workbenches with the reopen condition that two tools need to agree how a card in one workbench refers to a card in another. Nothing here meets that condition. Every tool call still answers about exactly one workbench, the `workbenches` listing names workbenches rather than relating cards, and no response carries a reference from a card in one workbench to anything in another. A head serving several workbenches is a property of a long-lived server process rather than a rule of the protocol it speaks.

Out of scope on this card: the cli head's own workbench resolution, any response that spans two workbenches, concurrent request handling, and a `--root` on any command other than `mcp`.

The UX sketch for this card is `docs/specs/dinah-192-mcp-workbench-parameter-ux-sketch.md` on the branch `dinah-192-the-mcp-surface-takes-the-workbench-as-a-parameter`.

## Working notes

## Why the argument rather than a current workbench

A current workbench would have cost less at the schema and more everywhere else. MCP as this head implements it carries no session state at all. `Serve` reads a line, `dispatch` answers it, and nothing survives between the two beyond the library the process was started with. A `use_workbench` tool would introduce the first piece of state the protocol holds. Every call after it would then mean something different depending on a call the reader of the transcript may not have in front of them, and a client running two agents against one registration would have to serialise them around a mode neither can see. The refusal an agent meets when it gets that wrong says the card is unknown, which is true of the workbench it was pointed at and useless as a diagnosis.

The argument costs one injection site. `schemaFor` already adds two properties that are no command's parameter. The third is therefore a change in one function rather than across twenty-six schemas, which is the fact triage handed to this station and which the throwaway build confirmed.

## Why a downward walk rather than a configured list

Discovery today climbs. Nothing in the tool walks downward, so bounding the reach by a root and then enumerating what lies under it is genuinely new code rather than a reuse of `walk`. The alternative considered was a list of workbenches written into the registration, which needs a configuration surface that does not exist and which D-2's own note already ruled out as a second card's worth of work. The walk it is, with the cost paid only by the `workbenches` tool, because resolving a named workbench needs a containment test and a stat rather than a search.

## Why the startup rule splits on where the default came from

Three of the four startup cases end in a head whose unqualified calls all answer with a refusal. So the question is not whether that head is useful. It is whether anybody asked for the situation it is in. A root that holds no workbench serves, and a walk that landed outside the root serves. A pointer somebody wrote into the registration is different in kind. `--root` and `--workbench` disagreeing, or `--workbench` naming a directory that holds no workbench at all, is a contradiction in a file, and refusing it while whoever wrote that file is still reading stderr is worth more than serving.

The rung is what separates the two, and nothing else can. Startup discovery climbs from the server process's working directory, and an MCP client chooses that directory. A head that exited over a walk would start on one machine and refuse on another with no explanation anywhere. The split costs one comparison. `bench.ResolveWorkbenchSource` returns the rung that answered, and `session` holds the resolved override and its rung before discovery runs at all.

Cases 2 and 3 split on the same line for the same reason. Treating them differently would say that a pointer naming a real workbench in the wrong place is somebody's instruction while a pointer naming nothing at all is not, and both are the same mistake in the same file.

## Why the two reused refusal names keep their sentences

`dinah.no-workbench` names `--workbench` in its sentence and `dinah.no-workbench-found` tells its reader to run `dinah init` here, and neither sentence travels over MCP. `verb.Response` carries the refusal name, the detail, and the named context, and `composeRefusal` in the cli head is the only site that renders a sentence. The JSON-RPC error branch carries no sentence either, since `Refusal.Error()` formats as a name and a detail. A binary built from the trunk confirms it. D-11 declines the finding on that ground and keeps the two smaller repairs it was right about, which are the translator's notes and the stderr report that startup case 2 prints before serving.

## Why the check sentence is short rather than declared

A generated help page can carry a sentence wider than the column it is rendered into, and the renderer's answer is to give that sentence a line of its own and drop its neighbour to the next line. `dinah help query` ships three rows in that shape today, so the layout is the tool's own behaviour rather than a fault, and declaring it in the spec would have been a defensible answer.

Two things decide it the other way. A sentence at or under 52 display columns keeps its refusal name beside it, and one of 53 or more pushes the name onto a line of its own. The long draft was also hard to read on its own terms, since "the workbench --workbench or DINAH_WORKBENCH names lies under the root" is a garden path and naming both spellings is what forced the length. After the startup rule split on the rung, the check applies exactly when somebody named a workbench, so "the workbench you name lies under the root" says the whole of the rule in 42 columns and leaves ten columns for a translation to spend.

Two figures in this section were wrong until the 2026-08-26 retest measured them, and both are corrected above. The ceiling is 52 rather than 51. And an overlong sentence does not push the page past 80 columns at the ceiling, as this section previously claimed: the page stays inside 80 until the sentence itself reaches 72 columns, which is the first width producing an 81-column line. Both figures were established by rebuilding the binary at each width and reading the rendered page, twice over by different agents. The correction is recorded here rather than silently applied, because a card whose retest turned on measuring these numbers should not go on carrying the versions the measurement disproved.

## Alternatives left on the table

Serving whatever the caller names was rejected by D-2. Naming the root with a global flag rather than one on `mcp` was rejected because no other command has any use for it and a global flag is a promise to every command. Reusing `dinah.no-workbench-found` for a root that does not exist was rejected because its sentence describes an upward walk from a starting directory, which is not what happened, and the contract already separates `dinah.no-workbench` from `dinah.no-workbench-found` on exactly that ground. Reusing `samePath` for the containment test was rejected by D-10, because it returns true when a stat fails and true means admitted here. Declaring the two-line refusal row rather than shortening the sentence was rejected by D-12, because the sentence fits once it stops naming both spellings of the pointer.

## Branch

dinah-192-the-mcp-surface-takes-the-workbench-as-a-parameter
