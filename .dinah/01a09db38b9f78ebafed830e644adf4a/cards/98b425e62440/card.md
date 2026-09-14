---
title: The language flag is honoured only when it precedes the word that fails to parse
column: b69abf918c42
state: ready
severity: minor
priority: soon
tier: workhorse
workstreams:
  - fdfdeaaff2dd
links:
  - kind: relates_to
    to: f8193ba6b038
---
Dinah resolves the display language from four rungs, in order: the `--lang` flag, `DINAH_LANG`, the user's config, and the OS locale. A refusal raised while the arguments are still being read renders through the same localised renderer every other refusal uses, because `run` builds the session and its renderer before it tests the parse error. Three of the four rungs are therefore honoured no matter where the parse stopped.

The flag rung is not. A `--lang` written after the word that fails to parse is never seen, because the scan stopped before reaching it, so `dinah --lang de --nosuchflag` answers in German and `dinah --nosuchflag --lang de` answers in English. The same invocation renders in two languages depending on where the caller typed the flag, and both orderings are equally natural to type.

The behaviour is deliberate and is explained in a comment on the branch that reports the parse error. Nothing asserts it. It is a convention rather than a contract, so a later rewrite of the argument scan could reverse it in either direction without failing a test.

The card decides one of two things. Either the scan pre-reads `--lang` before composing a parse refusal, making the flag order-independent like the other three rungs, or the current behaviour becomes a contract with a test behind it, on the argument that a scan which stopped cannot honestly claim to have read what came after it.

This card was filed as "A refusal raised while reading the arguments is always in English" and that is no longer true; the language ladder was moved ahead of the parse-error report at some point after filing. The measured evidence for what remains, and the rejected collision with dinah-31, are in the triage comment.

## Specification

## The decision

The scan gains a pre-read of `--lang`, and the flag becomes order-independent, the same way the other three rungs already are. This half of the fork is unchanged from the previous pass. `docs/design/format.md:1129-1130` documents the ladder as first-hit-wins with no mention of scan position, and pinning today's position-dependent behaviour with a test would put on record that `dinah --lang de --nosuchflag` and `dinah --nosuchflag --lang de` answer in different languages for the same mistake. This spec still declines to do that.

Review found the shape of the pre-scan unsound. As specified, it matched the bare token `--lang` wherever it sat in argv, with no notion that the word might be sitting in the value slot of a different flag. When a caller writes some other valued flag followed by `--lang de`, for example `dinah move card1 --card --lang de`, the real parse sets that flag's value to the literal string `--lang`, `parsed.value("lang")` is empty, and no `--lang` was given at all; the previous `scanLangFlag` read the same argv and returned `"de"`, which would have rendered the eventual refusal in German for an invocation that never asked for German. The flag's identity does not matter here; any valued flag opens the same trap. That broke the spec's own claim that the scan and `parseArgs` agree on every successful parse, and it meant the fix could change the rendering language of an invocation that has nothing to do with `--lang`.

## Reading argv once

A valued flag's value slot is not a special case bolted onto the scan; it is `parseArgs` doing exactly what it always does, which is read `valuedFlags` (`cmd/dinah/args.go:27`, declared once from every command's parameter table by `declaredFlags`) and consume the next word as that flag's value. `scanLangFlag` cannot get this right by matching the word `--lang` against the input in isolation, because whether a given occurrence of `--lang` is a flag at all depends on what came immediately before it, and that is the one piece of information a pattern match never has. It has to be the same decision, made once.

So `parseArgs`'s own inner loop is split out into `walkFlags`, a function both callers run. It carries every rule the loop already applies, the POSIX `--` marker, the `askedFor` spellings, and which flags are valued, and it is the only place that decides whether a word is a flag's own value. Neither caller repeats that decision; each supplies what differs.

`cmd/dinah/args.go` gains:

```go
// walkFlags is the walk parseArgs and scanLangFlag both run over argv. It
// recognizes the POSIX "--" marker, every askedFor spelling, and every flag
// in known, and for a valued flag it decides whether the following word is
// that flag's own value, all without knowing which command the caller named:
// valuedFlags and markerFlags are declared once across every command's
// parameter table (declaredFlags, above), so the walk is the same walk
// whatever command argv turns out to name. Splitting it out is what makes
// scanLangFlag agree with parseArgs on which word belongs to which flag,
// rather than a second reading of argv reaching its own answer.
//
// onPositional is called for every word the walk does not place as a flag:
// every word once the marker has been seen, and every word before it that
// carries no "--" prefix (or is the bare "-").
//
// visit is called once per recognized flag occurrence, in argv order: name
// is the flag's own name (or the value askedFor maps a spelling to), value
// is what a valued flag's occurrence carried (empty for a marker or an
// askedFor spelling), and complete is false only for a valued flag whose
// name was the last word in argv, with no word left to serve as its value.
// tokens is the literal argv word or words this occurrence consumed, in the
// order the caller wrote them.
//
// onUnknown is called once for each "--name" word the walk cannot place
// (any inline "=value" already split off, the same word contract.RefuseWith
// names today). Its return says whether the walk stops there. parseArgs
// stops, since an unrecognized flag is refused before anything past it is
// read. scanLangFlag does not, since reading past the word that fails to
// parse is the reason it exists. A word onUnknown lets pass is left as one
// consumed word: the walk was never told this name takes a value, so it has
// no ground to claim the next word as that value either, and the next word
// is read on its own terms by the following iteration.
func walkFlags(
	argv []string,
	valued, known map[string]bool,
	onPositional func(word string),
	visit func(name, value string, complete bool, tokens []string),
	onUnknown func(word string) bool,
) {
	markerSeen := false
	for i := 0; i < len(argv); i++ {
		word := argv[i]
		if markerSeen {
			onPositional(word)
			continue
		}
		if word == "--" {
			markerSeen = true
			continue
		}
		if asked, ok := askedFor[word]; ok {
			visit(asked, "", true, nil)
			continue
		}
		if word == "-" || !strings.HasPrefix(word, "--") {
			onPositional(word)
			continue
		}
		name, inline, joined := strings.Cut(strings.TrimPrefix(word, "--"), "=")
		if !known[name] {
			if onUnknown(word) {
				return
			}
			continue
		}
		if !valued[name] {
			visit(name, "", true, []string{word})
			continue
		}
		if joined {
			visit(name, inline, true, []string{word})
			continue
		}
		if i+1 >= len(argv) {
			visit(name, "", false, []string{word})
			continue
		}
		i++
		visit(name, argv[i], true, []string{word, argv[i]})
	}
}
```

`parseArgs` keeps its own signature and its own error, and becomes a thin caller of `walkFlags` that reassembles exactly what it built before:

```go
func parseArgs(argv []string, valued map[string]bool) (*arguments, error) {
	parsed := &arguments{flags: map[string]string{}}
	known := map[string]bool{}
	for _, flag := range append(append([]string{}, valuedFlags...), markerFlags...) {
		known[flag] = true
	}
	var refusal *contract.Refusal
	walkFlags(argv, valued, known,
		func(word string) {
			parsed.positional = append(parsed.positional, word)
		},
		func(name, value string, complete bool, tokens []string) {
			session := sessionFlagNames[name]
			if !complete {
				if session {
					refusal = contract.RefuseWith(contract.Usage, tokens[0], map[string]string{"dashHint": "1"})
					return
				}
				parsed.domainCaptures = append(parsed.domainCaptures, domainCapture{
					name: name, tokens: tokens, posAt: len(parsed.positional), complete: false,
				})
				return
			}
			parsed.flags[name] = value
			if !session {
				parsed.domainCaptures = append(parsed.domainCaptures, domainCapture{
					name: name, value: value, tokens: tokens, posAt: len(parsed.positional), complete: true,
				})
			}
		},
		func(word string) bool {
			refusal = contract.RefuseWith(contract.Usage, word, map[string]string{"dashHint": "1"})
			return true
		},
	)
	if refusal != nil {
		return parsed, refusal
	}
	return parsed, nil
}
```

One behaviour is worth naming explicitly. `walkFlags`'s `visit` callback has no way to stop the walk early the way `onUnknown` does, and `parseArgs` does not need one. Its only early-stop case under the current code is an incomplete session flag, and `i+1 >= len(argv)` is true only on the very last word of argv, so the surrounding `for` loop has nothing left to iterate over anyway; setting `refusal` there and letting the loop end on its own reaches the same result as returning from inside it.

`scanLangFlag` becomes a second, narrower caller:

```go
// scanLangFlag finds the value the caller gave --lang, walking the whole
// argument list through walkFlags rather than stopping at the first word
// parseArgs cannot place. DINAH_LANG, the user config and the OS locale are
// not attached to argv at all, so nothing about them depends on where a
// word sits; --lang was the one rung a scan that stops early could still
// silence. dinah-97 is the record of that: the same invocation answered in
// two languages depending on where the flag was typed.
//
// Sharing walkFlags with parseArgs, rather than a second pattern match
// against the literal word "--lang", is what keeps this scan honest about a
// word that belongs to somebody else. When a caller writes a valued flag and
// then "--lang de", the value slot that flag opens takes the literal text
// "--lang", and the caller is left with no --lang at all. walkFlags already
// knows which names take a value and consumes the word after one wherever in
// argv it falls, including after the word a failed parse stopped at, so this
// scan places that word where parseArgs places it.
//
// No flag name stands in for "a valued flag" above, because a name spelled in
// a comment goes stale in silence the day the flag tables are renamed.
// TestScanLangFlagReadsOnlyALangThatIsAFlag builds the invocation instead, out
// of a valued flag it reads from valuedFlags.
//
// The scan stops at the same POSIX "--" marker parseArgs does. A --lang
// with no following word is incomplete and is not reported; the ladder
// falls through to its next rung exactly as it does when --lang is absent
// altogether. An incomplete flag can only be the last word in argv, so
// there is nothing after it the scan could have missed either way. An
// unrecognized "--word" does not stop the scan, since reading past it is
// the whole point, and walkFlags does not treat the word that follows it as
// anyone's value, since nothing declared the unrecognized name as taking
// one. The last complete --lang the scan finds wins, matching parseArgs's
// own last-value-wins rule for a repeated flag.
func scanLangFlag(argv []string) string {
	valued := map[string]bool{}
	known := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
		known[flag] = true
	}
	for _, flag := range markerFlags {
		known[flag] = true
	}
	value := ""
	walkFlags(argv, valued, known,
		func(string) {},
		func(name, v string, complete bool, tokens []string) {
			if complete && name == "lang" {
				value = v
			}
		},
		func(string) bool { return false },
	)
	return value
}
```

`run` (`main.go:102`) changes the same one line as before, from:

```go
r: msg.For(bench.ResolveLang(parsed.value("lang"), cfg)),
```

to:

```go
r: msg.For(bench.ResolveLang(scanLangFlag(argv), cfg)),
```

`parsed.value("lang")` is not touched anywhere else and stays as it is. `commands.go:797` reads it inside `runConfig`, which only runs once a command has already been dispatched, so `parsed.value("lang")` and `scanLangFlag(argv)` agree there by construction, exactly as before.

## Why the two callers now agree everywhere, not just on the cases already tested

Both `parseArgs` and `scanLangFlag` resolve the same question, "which word is which flag's value", by running the same `walkFlags`, fed the same `valuedFlags`/`markerFlags`. They cannot read one occurrence of `--lang` two different ways, because there is only the one reading. `walkFlags` decides it once, and each caller's `visit` callback only records what `walkFlags` already decided. This closes the value-slot case review found. A `--lang` sitting where any valued flag expects its value is consumed by that flag's occurrence inside `walkFlags` itself, before `scanLangFlag`'s own `visit` callback ever sees a `name == "lang"` to record.

The case review asked to have named explicitly, a `--lang` in the value slot of a flag that itself comes after the word that fails to parse, works the same way and needs no special handling. `dinah --nosuchflag --card --lang de` (any valued flag stands in for `--card` here) walks past `--nosuchflag` (the unknown-flag case, not stopped), then reaches `--card`, which is valued and known, and consumes `--lang` as its own value exactly as it would if `--card` had been the first word on the line. `scanLangFlag` returns empty, matching what `parsed.value("lang")` would have returned had parsing not already failed on `--nosuchflag` first.

That equivalence covers only the walk itself, and one further fact is what makes it survive to `parsed.flags` rather than getting overwritten afterward. After `parseArgs` returns, `resolveOpenTailFlags` (`args.go:377`) corrects `parsed.flags` for a domain flag whose capture falls inside an open-tail command's free-text zone. When the capture does not form a genuinely trailing run of the command's own flags, the function deletes the flag's entry from `parsed.flags` and splices its tokens back into positional text (`args.go:451-471`). `scanLangFlag` has no equivalent correction pass and could not have one, since it is never told which command was named or where that command's free-text zone begins. The reason `resolveOpenTailFlags` never reaches `--lang` is not that it was taught to skip it; it is that `--lang` can never appear in `parsed.domainCaptures` at all. `parseArgs`'s `visit` callback appends a capture only when the flag is not a session flag (`args.go:218`, `227`, `251`), and `sessionFlagNames["lang"]` is `true` unconditionally, for every command (`args.go:78-79`). `resolveOpenTailFlags` reads and rewrites `parsed.flags` exclusively through that capture list, so a flag that never enters the list is a flag the function has no path to touch, on any command, free-text zone or not.

Sharing `walkFlags` is what makes the two callers agree on which word belongs to which flag in the first place. The session/domainCapture partition is the separate, load-bearing fact that keeps `parseArgs`'s own later correction pass from moving that answer again after `walkFlags` has already spoken. Neither fact stands in for the other. `walkFlags` alone would still let `resolveOpenTailFlags` rewrite a captured `--lang` on some future command, and the session/domainCapture partition alone, without a shared walk, would still let `scanLangFlag` mis-scan a valued flag's value slot as its own. A change that folded session flags into `domainCaptures`, to let `dinah-100`'s free-text handling reach them too, would reopen exactly this gap, and nothing about `walkFlags` itself would notice, since `walkFlags` never sees `domainCaptures` at all.

A test for this invariant is cheap, so this spec adds one rather than leaving it as a fact a future reader has to re-derive from `args.go` by hand. `parseArgs` already has a table-driven test in `args_test.go`; asserting that `parsed.domainCaptures` carries no entry for any name in `sessionFlagNames`, run once for each of the five session flags that can reach the valued/marker branch at all (`workbench`, `json`, `quiet`, `lang`, `actor`; `help` and `version` resolve through `askedFor` before that branch and never reach it either way), costs one new test function and no new production code. It is the one place a later change could silently reopen the gap above, and the test fails the moment that happens rather than waiting for the next review to notice. AC-9, below.

## The sweep, run again

D-2's note and the previous pass named three flags read from the partial parse ahead of the parse-error test: `workbench`, `json`, `actor`. Tracing `run` (`main.go:82` onward) again, a fourth is also read there and was missing from that list: `quiet`, read at `parsed.has("quiet")` when the session is built. I checked this rather than assuming it. `s.quiet` is read in exactly one place in the whole tree, `render.go:39`, inside the success-path branch that suppresses the served instructions on a claim or a move, and `reportError` (`main.go:272`), the function a parse-time refusal renders through, never reaches that branch. `quiet` is therefore inert on the parse-error path, and the sweep is now complete rather than three-quarters complete: `workbench` is read before the check but never consulted composing a parse-time refusal; `json` is forced false on that path regardless of what `--json` resolved to; `actor` is only resolved once parsing has already succeeded; `quiet` is read but never consulted composing a parse-time refusal either. `--lang` remains the only flag whose value both (a) is read before the parse-error check and (b) changes what the refusal report says.

## The malformed-`--lang` catalog, complete

A `--lang` can fail to name a working language in four ways rather than the three the previous pass listed:

1. **Incomplete** (`dinah --nosuchflag --lang`, nothing after it): treated as absent, falls through to the next rung. AC-3.
2. **Doubled** (`dinah --lang de --nosuchflag --lang hi`): the last complete occurrence wins. AC-4.
3. **Past the `--` marker** (`dinah add -- --lang de`): both words become literal text, and `scanLangFlag` never reads past the marker. AC-5.
4. **An unknown tag** (`dinah --lang xx --nosuchflag`, where no catalog answers to `xx`): `bench.ResolveLang` (`internal/bench/config.go:224`) returns the tag exactly as given, and `msg.For` (`internal/msg/msg.go:183`) walks it from full tag, to base language, to English, rendering in English when nothing along that walk matches. This is unchanged by this card. The walk runs identically whether the tag reached `msg.For` via `parsed.value("lang")` or via `scanLangFlag(argv)`, so a bogus tag behaves the same before and after the fix and needs no new handling.

## Acceptance criteria

AC-1 through AC-6, filed on this card, are unchanged: the position-independence regression both ways, the pre-existing Hindi-before-the-flag case staying green, an incomplete trailing `--lang` falling through silently, last-occurrence-wins for a repeated flag, the POSIX `--` marker being respected, and the whole-tree grep confirming exactly one remaining `value("lang")` call site. Three further criteria close the blocker that review found across both cycles:

- **AC-7.** A table-driven unit test in `cmd/dinah/args_test.go`, alongside `TestParseArgsHonorsTheEndOfOptionsMarker`, asserting `scanLangFlag(argv)` directly (no CLI dispatch, no fixture bench) against a table including at minimum: a `--lang` standing in a valued flag's value slot, wanting `""` (that `--lang` is the other flag's value, not a language choice); the same valued flag given a normal value with an ordinary `--lang` after it, wanting that language (an unambiguous `--lang` still reads correctly once a filled value slot precedes it); and the value-slot case again against a second, different valued flag, showing the fix is general rather than special-cased to one flag. The example flags are read out of `valuedFlags` at test time by `exampleValuedFlags` rather than spelled in the table, so a rename cannot leave a case naming a word the parser no longer knows, which is a case that still compiles and still passes while the scenario it exercises has stopped happening. This is the direct regression test for the blocker and fails against the previous `scanLangFlag`, which pattern-matched `--lang` blind to position.
- **AC-8.** A `--lang` sitting in a valued flag's value slot after the word that already fails to parse (`dinah --nosuchflag --SOMEVALUEDFLAG --lang de`) exits 2 and renders in English; `scanLangFlag(argv)` for that exact argv is asserted to return `""` in the same unit test as AC-7, showing the value-slot rule holds on the far side of the failing word too. The valued flag is read out of `valuedFlags` at test time rather than named. Verified with `t.Setenv("DINAH_LANG", "")` and no configured `lang`.
- **AC-9.** A unit test in `cmd/dinah/args_test.go` asserts that `parseArgs` never records a `domainCapture` for any name in `sessionFlagNames`: run once for each of `workbench`, `json`, `quiet`, `lang` and `actor` with a minimal argv that gives the flag a value or marks it present (e.g. `{"--lang", "de"}`, `{"--json"}`), asserting `len(parsed.domainCaptures) == 0` after each call. This is the direct guard for the invariant "Why the two callers now agree everywhere" names: were a future change to fold a session flag into `domainCaptures`, this test fails immediately, rather than leaving `resolveOpenTailFlags` free to rewrite that flag's value on some open-tail command with nothing on record to catch it.

## Out of scope

Unchanged from the previous pass: `docs/design/format.md`'s display-language section already documents the ladder as flag-over-environment-over-config-over-locale with no mention of scan position, and needs no wording change here. `runConfig`'s use of `parsed.value("lang")` (`commands.go:797`) only runs post-parse and already agrees with `scanLangFlag` there. dinah-31's `contract.UnknownFormat` refusal is confirmed on this card's own timeline to be raised on a parse that succeeded, so it is unaffected by anything here.

## Branch

dinah-97-the-language-flag-is-honoured-only-when-it-precedes-the-word-that-fails-to-parse
