---
title: the vocabulary migration cannot see a workbench in the directory you run it from
column: b69abf918c42
state: ready
severity: critical
priority: next
tier: workhorse
workstreams:
  - 4fd7a9f0b8ff
---
`dinah check --migrate-vocabulary` reports zero workbenches and writes nothing when it is run from the directory whose own `.dinah` holds the board. The next command the operator types then refuses the same board with `dinah.needs-vocabulary-migration`, which is the refusal telling him to run the command that just told him there was nothing to do.

Reproduced against the installed build, dinah-core/0.7:

```
mkdir board && cd board
dinah init . --slug repro --operator paul     # writes board/.dinah/3e112ba569bb
dinah check --migrate-vocabulary              # from board:   0 workbenches, nothing listed
cd .. && dinah check --migrate-vocabulary     # from parent:  1 workbench, listed and classified
```

The second run finds the same board the first run could not see, which is the whole defect in two commands.

## Cause

The walk probes the root and its children with different questions. `vocabularyCandidates` in `internal/verb/vocabulary.go` asks `bench.RecognizedAt(root)`, which is `benchIn(root, true)`, and that `true` is `skipBase`: it tests whether the root directory is itself a workbench directory and returns before it ever looks inside the root's `.dinah`. The children are probed by `walkFor` in `internal/bench/bench.go` with `benchIn(full, false)`, which does look inside each child's `.dinah`. `walkFor` also skips every entry whose name begins with a dot, so the root's own `.dinah` is not reached by descent either.

A board at `<root>/.dinah/<hex>` is therefore invisible from both directions, and that is the layout `dinah init` writes. The walk covers the case where the root is a bare workbench directory and the case where a child holds one, and misses the ordinary one in between.

## Reach

The operator's AOY boards are the live instance. `concepts/GAP007 - Aoyama-Couponing-System-Integration/.dinah/db2711207cbf` migrated only once the command was run from somewhere that treated it as a child. `concepts/.dinah/7c0e5a91d3b2` still declares `dinah-core/0.4` and still carries a `states/` directory, so it is stranded in exactly the same way and will refuse on the next open.

Severity is critical because the command's whole job is to be the one repair for a board the tool otherwise refuses to open, and it reports success while doing nothing. A migration that silently covers no boards is worse than one that refuses, since the operator has been told there is nothing to carry forward.

## Why no test caught it

`buildTreeFixture` in `cmd/dinah/vocabulary_test.go` plants each workbench by running `dinah init` into a scratch directory and then renaming the hex directory out of its `.dinah` container onto the tree. Its own comment says `dinah init` always writes into a `.dinah` base and the fixture wants a different shape. Every workbench in the tree test therefore sits bare on disk, which is a layout the tool never produces, and the layout it does produce is the one shape the suite never walks.

## Suspected second instance, not reproduced

`answerWorkbenches` in `internal/mcp/mcp.go` calls `bench.Enumerate(root)` with no root probe at all. Since `walkFor` skips dotted entries, a `DINAH_MCP_ROOT` pointing at a directory whose own `.dinah` holds the boards would list nothing, for the same reason and with no second question asked. This is read off the code rather than provoked, so it needs confirming before it is fixed or dismissed.

## Specification

## Evidence gathered before writing this contract

Two facts below were produced by running a command, not by reading, per this column's rule.

**Fact 1: the MCP path carries the identical defect, reproduced rather than merely suspected.** I built the binary from this worktree (`go build ./cmd/dinah`), ran `dinah init . --slug repro --operator paul` in a scratch directory (writing `board/.dinah/d1863909c10b`, the same layout the card's own repro shows), then ran, from a temporary `internal/bench/zz_probe_test.go` (written, run, deleted; never committed, confirmed by `git status --short` returning nothing):

```
PROBE enumerate(C:\dinah-scratch\dinah-312-spec\probe\board) -> listed=[]bench.Candidate{} err=<nil>
PROBE RecognizedAt(C:\dinah-scratch\dinah-312-spec\probe\board) -> found="" err=<nil>
```

`bench.Enumerate(root)` is exactly what `answerWorkbenches` in `internal/mcp/mcp.go:284` calls with the `DINAH_MCP_ROOT` value, so this settles the card's "suspected second instance": it is a real second instance of the same root-omission, not a separate defect and not merely a suspicion. It closes under the same fix (below), so no second card is needed.

**Fact 2: `RecognizedAt` has exactly one caller in the whole tree.** `grep -rn "RecognizedAt(" --include=*.go .` from the repository root returns two lines: the definition at `internal/bench/vocabulary.go:137` and its one call site at `internal/verb/vocabulary.go:88`. This licenses changing or removing it without a compatibility shim.

## Root cause, precisely

`internal/bench/bench.go:595-627`, `enumerate(root string)`, calls `walkFor(root, &collected, seen)` directly. `walkFor` iterates `root`'s **entries** and tests each child with `benchIn(full, false)` (bench.go:614-657); it never calls `benchIn` on `root` itself, and it skips every entry whose name begins with `.` (bench.go 630-632), which is what makes it skip `root/.dinah` were it ever to consider `root` itself as an entry. So `bench.Enumerate` finds a workbench two rungs deep (`root/customer/project/.dinah/<hex>`) and a bare workbench at `root/customer/project` directly, but never a workbench inside `root`'s own `.dinah`, and never a bare workbench.md sitting directly at `root`.

`internal/bench/vocabulary.go:132-140`, `RecognizedAt(root)`, calls `benchIn(root, true)`. The second argument is `skipBase`; set true, `benchIn` (bench.go:534-552) returns after testing only whether `root` itself is a bare workbench directory (`root/workbench.md` recognized) and never opens `root/.dinah` at all (bench.go 546-548, `if skipBase { return "", nil, passed, nil }`).

`internal/verb/vocabulary.go:78-93`, `vocabularyCandidates(root)`, calls both: `bench.RecognizedAt(root)` for "the root itself", then `bench.Enumerate(root)` for "at or beneath the root". Neither call ever opens `root/.dinah`, so a board at `<root>/.dinah/<hex>`, which is what `dinah init` always writes (`internal/verb/beyond.go:812-820`, `Init` always creates `written := filepath.Join(container, id)` where `container := filepath.Join(root, bench.UserBaseName)`), is invisible to both halves of the union.

## The fix: one change, at the point every caller shares

`bench.Enumerate` gains the root probe `benchIn` already runs for every other directory it visits, so every caller of `Enumerate`, present and future, inherits the fix instead of carrying its own copy of it. This is the generalized form of the child-directory question, not a new question: `benchIn(root, false)` is exactly the call `walkFor` already makes for every child.

### 1. `internal/bench/bench.go`, `enumerate`

Replace the body of `enumerate` (currently `bench.go:595-613`) with a version that probes `root` itself, with `skipBase=false`, before descending into its children, and folds the result into `collected` the same way `walkFor` already folds a child's result in (bench.go 644-656: a `found` value is described and appended; every `ambiguous` candidate is described and appended too, never just the first, never dropped):

```go
func enumerate(root string) ([]Candidate, error) {
	info, err := statPath(root)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownRoot, root)
	}
	if !info.IsDir() {
		return nil, contract.Refuse(contract.UnknownRoot, root)
	}
	var collected []Candidate
	seen := map[string]bool{}
	found, ambiguous, _, err := benchIn(root, false)
	if err != nil {
		return nil, err
	}
	if found != "" {
		seen[found] = true
		collected = append(collected, describe(found))
	}
	for _, candidate := range ambiguous {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		collected = append(collected, describe(candidate))
	}
	if err := walkFor(root, &collected, seen); err != nil {
		return nil, err
	}
	if collected == nil {
		collected = []Candidate{}
	}
	return collected, nil
}
```

The third return of `benchIn` (`passed`, the foreign anchors met and not claimed) is discarded with `_`, matching what `walkFor` already does with the same value at bench.go:648 (`_ = passed`); `Enumerate`'s contract carries no field for it today and this card does not add one.

This is the whole fix. `Enumerate`'s exported wrapper (bench.go:580-593) and its process-lifetime cache are untouched; a root string is still the cache key, and probing the root before the walk changes what gets cached, not whether caching is correct.

**Ambiguity at the root is not a new case.** When `root`'s own `.dinah` holds more than one recognized workbench, `benchIn(root, false)` returns `ambiguous` non-empty and `found` empty (`soleBench`, bench.go:760-784, returns no `found` once `len(candidates) > 1`). The loop above adds every one of them as its own `Candidate`, exactly as the existing loop at bench.go:657-663 already does for an ambiguous child. Nothing here silently picks one, and nothing here is a new report shape: a tree walk that meets an ambiguous `.dinah` two levels down already surfaces every candidate in it today, and the root is no longer a special case that fails to.

### 2. `internal/bench/vocabulary.go`

Delete `RecognizedAt` (lines 132-140 and its doc comment) entirely. It has exactly one caller (Fact 2 above), and that caller's need — "does the root itself offer a workbench, container included" — is now answered by `Enumerate` alone.

### 3. `internal/verb/vocabulary.go`

Replace `vocabularyCandidates` (lines 75-93) with a single call to `bench.Enumerate`:

```go
// vocabularyCandidates answers every directory at or beneath the root that
// offers a workbench, including the root itself, with no directory named
// twice. bench.Enumerate is the one place that question is answered; a
// second, independent root check here was what let the root's own .dinah go
// unchecked while every other directory's did not (dinah-312).
func vocabularyCandidates(root string) ([]string, error) {
	listed, err := bench.Enumerate(root)
	if err != nil {
		return nil, err
	}
	candidates := make([]string, 0, len(listed))
	for _, candidate := range listed {
		candidates = append(candidates, candidate.Path)
	}
	return candidates, nil
}
```

Update `MigrateVocabularyTree`'s doc comment (lines 39-48), which currently explains why the walk is "deliberately two calls rather than one": that reasoning is retired along with the second call. Replace it with a comment stating plainly that `bench.Enumerate` alone now answers the root and everything beneath it, and that this card (dinah-312) is why the old two-call shape existed and why it was wrong: the two calls disagreed about whether `root`'s own `.dinah` counted, and a board living there was invisible to both.

No other line in `MigrateVocabularyTree` or `migrateOneVocabulary` changes. `TreeVocabularyReport`'s shape (`Migrated`, `AlreadyCurrent`, `Unsupported`, `Malformed`, `Failed`) is unchanged: two workbenches found ambiguously at the root are two separate candidates by the time `migrateOneVocabulary` sees them, and each is classified into whichever bucket its own declared revision earns, independently of the other. No `Ambiguous` bucket is added, because none is needed: this is the same as any other pair of workbenches found side by side in the tree, and the report already has no special case for that.

## Why the fixture never caught this: the shape it plants, and the shape that stays legitimate

`cmd/dinah/vocabulary_test.go:107-179`, `buildTreeFixture`, plants every one of its four workbenches (`atRoot`, `nested`, `sibling`, `current`) the same way: `dinah init` into a scratch directory, which writes `scratch/.dinah/<hex>`, then `os.Rename(anchor, where)` moves the hex directory itself out to `where`, so `where` (not `where/.dinah/<hex>`) becomes the anchor directory. `atRoot`'s own doc comment says why: "bench.Enumerate tests a root's children and never the root, so a walk relying on it alone loses exactly this one" — true of the old code, and the fixture built around that gap rather than around the layout the tool actually writes. Every planted workbench in the current fixture is bare (no `.dinah` ancestor anywhere on its path), which is a shape `dinah init` never produces (`internal/verb/beyond.go:812`, `Init` always creates its anchor inside a `.dinah` container) and the one shape this defect hides in is a workbench still inside a `.dinah`, which the fixture never plants.

**The bare shape is not invented by the fixture and stays a real, separately-supported case.** `benchIn` (bench.go:534-548) tests `dir/workbench.md` directly, unconditionally, before it ever looks at `dir/.dinah`, and this is what the ordinary discovery climb (`walk`, bench.go:447-495) relies on at the machine's native home boundary ("a repository checked out at the home directory is found exactly as before", bench.go:344), and what the `--workbench`/`DINAH_WORKBENCH` override branch relies on when it is pointed at an exact anchor directory (`DiscoverSource`, bench.go:353 and 374, tests `Exists(filepath.Join(abs, WorkbenchAnchor))` directly). Neither of those code paths is touched by this card, and neither should lose test coverage as a side effect of fixing the root case. So: **both shapes must keep working, and the reworked fixture proves both, deliberately, rather than proving one by accident.**

### The fixture rework

Change `buildTreeFixture` so `atRoot` is planted in the real, `.dinah`-container shape at the walk's root — that is, stop renaming its anchor out of `.dinah`, and instead let `plant` (or a sibling helper) leave `root/.dinah/<hex>` in place and return `root` as the directory the walk is pointed at. This is the shape the reproduction in the card's description shows and the one shape the whole card is about; every acceptance criterion below that touches `atRoot` depends on this change, and none of them can pass against the current fixture, which never plants this shape at all.

Change `nested` to the same container shape, two levels down (`root/customer/project/.dinah/<hex>`), since that is the ordinary shape a `dinah init` run at that depth actually produces and the shape this migration will meet on a real machine far more often than a bare one.

Keep `sibling` planted bare, exactly as today, and say so in the fixture's own comment: `sibling` is deliberately the layout no command produces, kept to prove the discovery-override and native-home recognition path (`benchIn`'s unconditional anchor check) still works once the root gets its own probe. A fixture that quietly stopped planting any bare workbench would let that path rot unnoticed the same way the root case did.

`current` (the already-current-vocabulary workbench) may stay bare or move to the container shape; it is not exercising either question this card is about, so its shape is not part of this card's contract either way.

### A new, dedicated ambiguous-root fixture

`buildTreeFixture` grows no further; a second, small, single-purpose fixture proves decision D-2 below on its own, because loading a fifth concern onto one fixture is how a future reader loses track of which planted workbench is proving what. Build it as its own helper, in `internal/bench` (unit level, the layer that owns `enumerate`) rather than in `cmd/dinah`: a root directory whose `.dinah` holds two workbenches, both instantiated with `bench.Instantiate` directly into `root/.dinah/<id-a>` and `root/.dinah/<id-b>` (skipping `Init`'s container-claim machinery is fine here since the test names both ids itself and needs no `ClaimID` collision avoidance). Call `enumerate(root)` (or `Enumerate`, either is fine since the cache key is fresh per `t.TempDir()`) and assert the returned paths are exactly the two anchor directories, as a set, not as an ordered pair (`os.ReadDir`, which `soleBench` walks through `ListIDs`, sorts by name, so the order is in fact deterministic, but the property under test is "neither is dropped and neither is silently chosen," not "which one is listed first").

## Acceptance criteria

Each is written as one executable statement. Where a criterion asserts something a hand-written list could quietly stop maintaining, it says so.

**AC-1.** A unit test in `internal/bench` builds a directory whose own `.dinah` holds exactly one workbench (via `bench.Instantiate` into `<root>/.dinah/<id>`, mirroring what `Init` produces) and asserts `enumerate(root)` (or `Enumerate(root)`) returns exactly one `Candidate` whose `Path` is that anchor directory.
*Fails today*: the probe run above already shows it failing (`listed=[]bench.Candidate{}`).
*Mutation that must turn it red*: reverting `enumerate` to its current body (dropping the `benchIn(root, false)` probe added above).
*False-failure check*: the test asserts on the returned `Path`, not merely on `len(listed) > 0`, so a future change that returns one candidate for the wrong reason (e.g. a stray descendant match) does not pass it by accident.

**AC-2.** A unit test in `internal/bench` builds a directory whose own `.dinah` holds two recognized workbenches and asserts `enumerate(root)` returns exactly two candidates, whose paths are the two anchor directories, compared as a set.
*Mutation that must turn it red*: changing the ambiguity loop in `enumerate` to `if len(ambiguous) > 0 { collected = append(collected, describe(ambiguous[0])) }` (silently picking one) or to skip the loop entirely (silently dropping both). Either mutation is a plausible-looking simplification of the fix above, which is why this criterion exists as its own case rather than folding into AC-1.

**AC-3.** A CLI-level test in `cmd/dinah` reproduces the exact steps in the card's description — `dinah init . --slug repro --operator paul` in a fresh directory, then `dinah check --migrate-vocabulary --workbench <that directory>` (or run from inside it, matching the repro's `cd board`) against a workbench whose declared revision sits inside the pre-vocabulary window — and asserts the report lists that workbench under `Migrated` (preview) or actually migrates it (apply), rather than reporting zero workbenches. The test unwinds the freshly-initialized workbench to a pre-vocabulary revision first (`unwind`, `cmd/dinah/vocabulary_test.go:27-45`, already does exactly this for the existing tree fixture) so the report has something to classify.
*Mutation that must turn it red*: the same revert as AC-1, since this is the end-to-end path the card's description reports as broken.
*What it proves beyond AC-1*: that the fix reaches the actual command, not just the package-level function; a change that fixed `enumerate` but left some other layer between it and `dinah check` unwired would pass AC-1 and fail this one.

**AC-4.** `TestTheVocabularyMigrationWalksTheWholeTree` (and any other existing test built on `buildTreeFixture`) passes against the reworked fixture without weakening any existing assertion: `atRoot` and `nested` are found via their new `.dinah`-container paths, `sibling` is still found via its unchanged bare path, and `current` is still reported as needing nothing.
*Mutation that must turn it red*: breaking `walkFor`'s existing per-child recognition (e.g. passing `skipBase=true` into the `benchIn(full, false)` call at bench.go:645) — a regression this criterion exists to catch precisely because the fixture rework touches the same file the fix touches.

**AC-5.** A test in `internal/mcp/mcp_test.go` builds a workbench via `verb.Init` (which writes the real `.dinah` container, unlike `newLibrary`'s existing `bench.Instantiate` call, which writes bare) and asserts a `workbenches` tool call served with `askUnderRoot(t, <the directory whose .dinah holds it>, library, ...)` lists that workbench in its `workbenches` array.
*Fails today*: Fact 1's probe already demonstrates the underlying call returns empty for this exact layout; this criterion is the same fact exercised through the protocol surface `DINAH_MCP_ROOT` actually reaches (`answerWorkbenches`, `internal/mcp/mcp.go:284`).
*Mutation that must turn it red*: same revert as AC-1.

**AC-6.** `grep -rn "RecognizedAt(" --include=*.go .` run from the repository root returns no matches. This is the completeness claim ("the old function is gone, not just unused") produced by the command itself rather than by a maintained list of call sites, per this column's rule.
*Mutation that must turn it red*: leaving `RecognizedAt` defined (even if uncalled) after the rework, which is the halfway state a reviewer skimming only `vocabulary.go`'s new body could miss.

**AC-7.** The new ambiguous-root fixture (its own `internal/bench` test, see above) is a distinct test function from AC-1's and AC-2's, and its own doc comment says which of the four questions this card raises it answers (decision D-2), so a future reader does not have to reconstruct that from the diff.
*This criterion is process rather than behavior*, and is verified by review reading the test file rather than by a run; it is listed because the column instructions treat a criterion that cannot be verified automatically as still worth stating, with what a human checks named plainly.

## Decisions

**D-1 (resolved).** The fix lives in `bench.Enumerate`, not duplicated in `vocabularyCandidates` and again wherever a future caller needs the same root-inclusive listing. Reasoning: `RecognizedAt` already was the duplicated, independently-wrong copy of the question `benchIn` answers correctly for every other directory; the card's own "suspected second instance" section is the proof that letting each caller carry its own root probe is exactly the shape that produces a second, un-reproduced instance of one defect. Fixing the shared function is the one-command-covers-the-set form this column's instructions ask for.

**D-2 (resolved).** A root whose own `.dinah` holds more than one workbench is reported the same way an ambiguous descendant `.dinah` already is: every candidate listed, none silently chosen, no new field on any report. Reasoning: this is not a new question; `benchIn` already answers it for children, `soleBench` already carries the ambiguous list, and `walkFor` already folds it into `Enumerate`'s result without a dedicated bucket. Inventing a special case for the root when one is not asked for at any other rung of the tree would be the same class of duplication D-1 rejects, applied to the report shape instead of to the probe.

**D-3 (resolved).** The bare-workbench-directly-at-a-directory layout (no `.dinah` anywhere on its path) is a real, separately-exercised capability — the discovery override and the native-home boundary rung both depend on `benchIn`'s unconditional anchor check — and stays covered in the reworked fixture via `sibling`, deliberately labeled as the layout no command produces. It is not converted to the container shape alongside `atRoot` and `nested`, because doing so would silently drop the one thing in the existing fixture that was accidentally still worth keeping.

No open questions. Every gap the card's description raised is settled above by reading, by the one probe run permitted in this column, or by a whole-tree grep; none of it needs an operator ruling, since none of it commits the product to anything a person outside this workbench would need to weigh in on.

## Branch

dinah-312-the-vocabulary-migration-cannot-see-a-workbench-in-the-directory-you-run-it-from
