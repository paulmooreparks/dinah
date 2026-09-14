---
title: Token-lean output rendering for agent loops
column: b69abf918c42
state: ready
severity: minor
priority: soon
tier: frontier
workstreams:
  - fdfdeaaff2dd
---
The canonical JSON verb forms are the frozen machine contract, but they are not cheap to read in a driver loop that lists and claims cards all day. kanban-md ships a compact output format it measures at roughly seventy percent fewer tokens than JSON, aimed exactly at agent loops. Dinah should offer a compact rendering as an additional projection of the same verb definitions, so the contract stays untouched and nothing parses human output. Token economics are a first-class concern for the people running agents against a bench, and this is the cheapest lever available.

## Specification

## Contract check

CORE-JSON-1 through CORE-JSON-8 (`docs/spec/core-profile.md:589-603`) govern the interchange form of a workbench definition, the object `dinah init --from` reads and `dinah extract` writes. None of them names how a verb's answer is encoded. CORE-OUT-1 through CORE-OUT-6 (`docs/spec/core-profile.md:658-712`) govern what a verb's answer must report: one of four outcome tokens, exactly one refusal name on a refusal, and the order in which competing refusals are decided. None of them names an encoding either. A second machine projection carrying the same facts as the canonical JSON form breaks no statement in either list.

`docs/design/surfaces.md:34-47` states the architecture this card extends: one verb definition already generates four projections (CLI flags and help, the MCP tool schema, the HTTP route, and the human rendering), all read off one canonical JSON contract, and it says the CLI's machine form is under `--json`. A fifth projection, selected the same way the fourth one is, is consistent with that document's own shape. It also states plainly that "the MCP tools and HTTP routes are thin mappings of it", meaning the canonical JSON. Extending the compact projection to the MCP head would be changing what that sentence says, which is a design decision this card does not make (see Out of scope).

A grep across the repository for `--json` and `DINAH_FORMAT` outside test files and outside this board's own generated docs found no production code that parses a Dinah answer. The only non-test consumer that references `DINAH_FORMAT` is `scripts/capture_fixture.py`, which clears it before capturing a fixture rather than reading it. Nothing in this repository has to be taught about the new format.

## Scope

The compact projection is defined for three shapes, chosen because they carry a driver loop's traffic, per the measurement already on this card's timeline (`ls` at 3.9x the human form, `next` at 2.5x, `claim` at 10x, bytes as a proxy for tokens):

- `*verb.Response` (`internal/verb/library.go:209`), the shared envelope every mutating command already answers through. `render.go`'s `emit` is the call site for `claim`, `move`, `release`, `block`, `unblock` (all via `l.Do`), `add` (`l.Add`), `card set` (`l.SetCardField`), `comment` (`l.Comment`), `attach` (`l.Attach`), `archive` (`l.Archive`), `delete` (`l.Delete`), `rename` (`l.Rename`), `pull` (`l.Pull`), `join` and `leave` (both via `l.Do`), and `workbench set` (`l.SetWorkbench`), sixteen commands in all. `emitWorkstream` is the call site for the two remaining acts that produce this shape, `workstream new` (`l.NewWorkstream`) and `workstream set` (`l.SetWorkstream`). Between them, `emit` and `emitWorkstream` are the only two call sites that produce a `*verb.Response`, so defining its compact encoding once covers every one of those eighteen commands, not only `claim`.
- `*verb.Listing` (`internal/verb/read.go:131`), the answer `ls` gives.
- `[]verb.Offer` (`internal/verb/read.go:176`), the answer `next` gives.

Every other shape `emitJSON` is handed today (`Status`, `[]ColumnView`, `Matches`, `Tree`, `Detail`, `ChangeSet`, a workstream `Listing`/`Detail`, `Served`, `[]SettingView`, `Identity`, `WorkbenchFields`, `[]bench.Candidate`, `VersionReport`, `CheckReport`) keeps emitting canonical JSON when compact is requested. These are read-once or low-frequency answers in a driver loop, per the same measurement, and adding a compact rendering for each is separate design work this card does not commit to. A later card can add one without touching the format-selection machinery this one builds, because the fallback rule below is per-type, not a blanket switch.

dinah-124 (internal identifiers shown only on request) and dinah-128 (`guide` ignoring a request for the machine form) are ruled separate on this card's own timeline and are not touched here. In particular, the compact `card` record below carries every field the canonical `CardView` carries, including `id`; this card does not omit or gate any field, because doing so is dinah-124's question, not this one's.

## Format selection

`session.json bool` (`cmd/dinah/main.go:34`) becomes `session.format outputFormat`, a defined string type with three values:

```go
type outputFormat string

const (
    formatHuman   outputFormat = ""
    formatJSON    outputFormat = "json"
    formatCompact outputFormat = "compact"
)
```

`formatHuman` is the zero value, so a `session` nobody sets a format on renders for a person exactly as today.

Resolution reuses the flag-over-environment ladder `bench.Resolve` already provides (`internal/bench/config.go:140`), the same primitive `benchFlag` and `s.actor` are resolved through, rather than a new one:

1. If `--json` is present (a marker flag, unchanged) and `--format` is present with a value other than `json`, refuse `contract.Usage` with detail `"--json conflicts with --format "` followed by the `--format` value actually given (e.g. `"--json conflicts with --format compact"`). `--json` together with `--format json` is redundant and not a conflict.
2. Otherwise, resolve with `bench.Resolve(bench.Layer{Source: bench.SourceFlag, Value: flagValue}, bench.Layer{Source: bench.SourceEnvironment, Value: os.Getenv("DINAH_FORMAT")})`, where `flagValue` is `"json"` when `--json` is present, otherwise the literal value of `--format` when present, otherwise empty.
3. A resolved value of `""` (nothing set at either rung) is `formatHuman`. A resolved value of `"json"` is `formatJSON`. A resolved value of `"compact"` is `formatCompact`. Any other resolved value refuses `contract.UnknownFormat` with detail equal to that value, at the rung it was resolved from (flag or environment); the refusal fires whether the bad value arrived via `--format` or via `DINAH_FORMAT`, since a script setting the environment variable deserves the same refusal a script passing the flag gets.

This replaces today's silent behavior, where `DINAH_FORMAT` set to anything other than `json` falls through to the human rendering unremarked. The repository grep in Contract check found no test and no consumer that relies on that silence, so nothing this card touches is exercising it on purpose. Record this as a decision:

**Decision: an unrecognized format name refuses with a dedicated name, rather than falling back to human rendering or reaching for `contract.Usage`.** A caller who mistypes `DINAH_FORMAT=cmopact` and gets prose back where it expected structure has a worse failure than a caller who gets a refusal naming the mistake, so the silent fallback goes.

`contract.Usage` is the wrong precedent for the refusal itself, though it stays correct for step 1 above (a genuine usage error: two flags naming conflicting values). `cmd/dinah/commands.go:815` (`if looksLikeMistypedFlag(key) { return s.fail(contract.Usage, key) }`) is the case of a flag typed where `config get`'s key argument was expected, a structural mistake in the shape of the command line. The refusal for a key that parses fine but names nothing `config` recognizes sits five lines later, `cmd/dinah/commands.go:820-821` (`if !bench.KnownConfigKey(key) { return s.fail(contract.UnknownKey, key) }`), and it already carries a dedicated name, `contract.UnknownKey`, not `contract.Usage`.

That dedicated-name shape is the codebase's actual convention for a value outside a closed set, and it holds across every other example: a guide topic gets `contract.UnknownGuide` (`internal/guide/guide.go:71`), a tree depth gets `contract.UnknownDepth` (`internal/verb/tree.go:273`), a workstream reference gets `contract.UnknownWorkstream`, a level name gets `contract.UnknownLevel`. `contract.UnknownValue` reads as the generic member of that family, and it is the nearest existing name to reach for, but its own doc comment scopes it to one thing: "a query giving a closed-vocabulary field a value that vocabulary does not hold," kept distinct from an empty result "because an empty result is also the honest answer to a query that is exactly right, and a reader cannot tell a typo from a fact" (`internal/contract/contract.go:138-142`). That reasoning is about querying specifically: a query can legitimately match nothing, so a wrong field value needs a name distinct from zero matches. `--format` and `DINAH_FORMAT` are not a query; there is no result set for an unrecognized format to be confused with, so the fact `UnknownValue`'s comment exists to state does not apply here, and attaching the name would leave a refusal whose only rationale describes a different feature.

The refusal is `contract.UnknownFormat`, a new name Implement adds to `internal/contract/contract.go`, in the `LayerPrefix` block alongside `UnknownKey` and `UnknownGuide`, with a doc comment of its own (naming what closed set the value fell outside of, in the style the neighboring constants already use) and an entry in the `Introduced` slice.

`--format <name>` is added as a new global flag, valued, alongside `--json` in `globalFlags` (`cmd/dinah/help.go:63`), and `"format"` is added to `sessionFlagNames` (`cmd/dinah/args.go:77`) so it is read at session-build time before any command runs, the same as `json`. `--json` is not deprecated and is not rewritten to expand to `--format json` internally; it stays its own marker flag with its own entry in `globalFlags`, so a script that has always written `--json` keeps working with no change to argument parsing, help text order, or exit behavior.

On a parse failure (`cmd/dinah/main.go:118-119`, where `s.json = false` today), the replacement is `s.format = formatHuman`, for the same stated reason: the flags that would have carried a format are the ones the parse never reached.

## The compact grammar

The compact form is UTF-8 text, line-oriented. Splitting the whole payload into physical lines is always done first, on the literal byte `0x0A`, before any field is read from a line; no field's content is ever allowed to contain a raw `0x0A`, because every string value is escaped before it is written (below), so this split is never ambiguous.

Each physical line is one **record**: a sequence of fields separated by the literal byte `|` (`0x7C`), where a `|` or a `\` occurring inside a field's own content has been escaped. Splitting a line into fields is done left to right, tracking whether the previous byte was an unescaped `\`; a `|` reached while not in that state ends a field, and a `|` reached while in that state is data. The **kind** is always the first field of every record and names which of the record shapes below it is.

**Escaping.** Before a string value is written into a field, every occurrence of `\` becomes `\\`, then every occurrence of `|` becomes `\|`, then every literal LF (`0x0A`) becomes the two bytes `\n`, then every literal CR (`0x0D`) becomes the two bytes `\r`. Decoding reverses this in one left-to-right pass: `\\` to `\`, `\|` to `|`, `\n` to LF, `\r` to CR. A `\` followed by any other byte, or a bare trailing `\` at end of field, is malformed input and never occurs in anything Dinah emits; Implement adds an internal (test-only, unexported) decoder under `cmd/dinah` used by the test suite to verify round-trips, and that decoder treats such a sequence as a hard parse error rather than guessing. This decoder is not a shipped public API; nothing in this card promises a stable Go package for consuming the compact form, only the wire grammar itself.

**Payload framing.** Every compact payload begins with a version record, `fmt|compact|1`, on its own line, before anything else. Nothing before this card's format has a version to compare against, so `1` is the first value; a future incompatible change to the grammar below increments it, and a caller checks it before assuming the field order this card specifies. The payload ends with exactly one trailing `\n` after the last record, mirroring `emitJSON`'s own `string(data)+"\n"` (`cmd/dinah/render.go:75`).

**Record kinds and their fields, kind first:**

| kind | fields after kind, in order | repeats |
|---|---|---|
| `fmt` | `compact`, version (`1`) | once, first line |
| `rsp` | outcome, verb, refusal, detail, basis, warning, warning_detail, message | 0 or 1, envelope for a `*verb.Response` or a pre-verb refusal |
| `card` | id, ref, title, column, column_title, state, severity, priority, holder, claim_since, expires, block_reason, block_kind, revision, *then* 0 or more trailing fields, one per workstream identifier | 0 or 1 directly under `rsp`; 0 or more directly under `lst`; 0 or 1 directly under an `off` |
| `wstream` | id, ref, slug, title, status, cards | 0 or 1, only when `Response.Workstream != nil` |
| `instr` | global, standing, column | 0 or 1, only when `Response.Instructions != nil` |
| `move` | column, ref, title, direction, reject | 0 or more, one per `Response.LegalMoves` entry, original order preserved |
| `ctx` | key, value | 0 or more, one per `Response.Context` or `refusalReport.Context` entry, sorted ascending by key the same way `flattenMessageValues` (`cmd/dinah/render.go:115`) already sorts a message's named values, for the same reason: one map produces one field order however the runtime walks it |
| `msgval` | key, value | 0 or more, one per `Response.MessageValues` entry, sorted ascending by key, same reasoning as `ctx` |
| `wb` | title, slug, path | 0 or more, one per `bench.Candidate`, only on a `dinah.ambiguous-workbench` refusal, in the order `bench.Reachable` returns them |
| `aff` | 0 or more trailing fields, one per affordance token | always exactly 1, even when there are zero affordances, so a parser can rely on it marking the end of an `rsp` record's block |
| `lst` | column | always exactly 1 for an `ls` answer, first line after `fmt` |
| `off` | column, title, awaiting_outside, no_taker, taken_by_pull (each `1` or empty) | once per `Offer` in the `[]Offer` answer, in slice order |

**Canonical order for a `*verb.Response` payload (covers `claim`, `move`, `release`, `block`, `unblock`, `add`, `card set`, `comment`, `attach`, `archive`, `delete`, `rename`, `pull`, `join`, `leave`, `workbench set`, `workstream new` and `workstream set`) or a pre-verb refusal:** `fmt`, `rsp`, `card` (if present), `wstream` (if present), `instr` (if present), `move`* (if any), `ctx`* (if any), `msgval`* (if any), `wb`* (if present, refusal path only), `aff`.

**Canonical order for `ls`:** `fmt`, `lst`, then `card`* (zero or more, in the same order `Listing.Cards` already carries, which is CORE-QUEUE-3's fixed order).

**Canonical order for `next`:** `fmt`, then for each `Offer` in order: `off`, then `card` if that offer carries one, with no `card` line at all when it does not (an `Offer.AwaitingOutside` offer or an empty column).

A pre-verb refusal (`refusalReport`, `cmd/dinah/main.go:243`) uses the `rsp` record with `verb`, `basis`, `warning`, `warning_detail` and `message` all empty, `outcome` always `refused`, followed by `ctx`* and, for `dinah.ambiguous-workbench` only, `wb`*, then `aff` with zero trailing fields (a pre-verb refusal names no affordances). This keeps the property `refusalReport`'s own doc comment already claims for JSON ("a caller parsing --json reads one shape whichever layer said no") true for compact as well.

**Equivalence.** For every field the canonical JSON carries, the compact record carries the identical string, byte for byte, decoded. No field is dropped, renamed, truncated, or summarized. Where JSON's `omitempty` drops an empty string or a nil pointer, compact represents the same absence as an empty field (for a scalar) or the absence of the optional record (for `card`, `wstream`, `instr`, `wb`). This is the same non-distinction the Go structs themselves already make (an omitted JSON key and an explicit empty string decode to the same Go zero value), so no information is lost in either direction that JSON did not already lose.

**Where this table was wrong.** The `reject`, `no_taker` and `taken_by_pull` fields and the whole `wstream` record were missing from the table above when this spec first reached Implement. Implement carried them regardless and recorded the departure as D-7.

The two halves of this section contradicted each other. Equivalence is unconditional and says no field is dropped, and the table then dropped `Offer.NoTaker`, `Offer.TakenByPull`, `LegalMove.Reject` and `Response.Workstream`. AC-3, AC-4 and AC-5 each demand identical field values, so the shorter table could not be satisfied against the criteria either. Equivalence binds. The table enumerates what that clause requires, so an enumeration falling short of the clause is a defect in the enumeration, and the rows above now carry what the clause already asked for.

The `Response.Workstream` omission was the costly one. Without that record `workstream new` and `workstream set` would have answered with an envelope naming no subject at all, on two of the eighteen commands this spec lists by hand as covered.

## Dispatch, and what changes at each call site

`emitJSON` (`cmd/dinah/render.go:68`) is renamed to `emitCanonical` and is otherwise untouched: it marshals canonical JSON exactly as it does today, byte for byte, and every existing caller that keeps calling it is unaffected by this card.

A new `emitMachine(value any) int` replaces every one of the roughly twenty call sites currently reading `if s.json { return s.emitJSON(x) }`, changing them to `if s.format != formatHuman { return s.emitMachine(x) }`:

```go
func (s *session) emitMachine(value any) int {
    if s.format == formatCompact {
        if data, ok := compactEncode(value); ok {
            io.WriteString(s.out, data)
            return exitCodeFor(value)
        }
    }
    return s.emitCanonical(value)
}
```

`compactEncode` is a type switch over `*verb.Response`, `refusalReport`, `*verb.Listing`, and `[]verb.Offer`; every other type returns `ok == false`, and `emitMachine` falls through to `emitCanonical`, which is byte-identical to today's `--json` output whatever `s.format` says. `exitCodeFor` extracts the same exit code `emitJSON`'s callers compute today (`contract.ExitCode(response.Outcome)` for a `*verb.Response`, `0` for everything else, matching each call site's existing return value); this is a straight carry of logic already in each call site rather than new behavior. `render.go`'s `emit` and `emitWorkstream`, and `main.go`'s `reportError`, call `s.emitMachine` in place of their own `s.json` branch and direct `s.emitJSON` call, so the `*verb.Response` and `refusalReport` compact paths are reached from exactly the same call sites that reach canonical JSON today. `s.quiet` continues to have no effect on any machine form: today's comment on `session.quiet` already says the machine forms always carry served instructions, and compact is a machine form.

## Help text and catalogs

`globalFlags` (`cmd/dinah/help.go:63`) gains `{name: "format", usage: "--format <name>", value: "name"}`, placed immediately after the `json` row so the two related flags sit together. The catalog key `flag.json.summary` (`internal/msg/locales/en.json:1112`) is unchanged; a new key `flag.format.summary` is added with English text `"Select json or compact for the machine form"`. The `help.environment` key (`internal/msg/locales/en.json:1136`) changes from `DINAH_FORMAT=json` to `DINAH_FORMAT=json|compact`, and both changes are made in every locale file under `internal/msg/locales/`, not only `en.json`; this repository's message-catalog coverage test already fails a locale that is missing a key another locale carries, so the new key is exercised by existing infrastructure rather than a new test class.

## Out of scope

- **The MCP head.** `internal/mcp/mcp.go`'s `call` function encodes every tool's answer with `json.MarshalIndent` directly (`internal/mcp/mcp.go:266-273`) and does not go through `session` or `s.format` at all. `docs/design/surfaces.md` states the MCP surface is a thin mapping of the canonical JSON. Giving an MCP tool call its own `format` argument, and deciding whether an LLM client actually benefits from the compact wire form the way a shell-scripted driver loop does (a tool result already arrives inside a JSON-RPC envelope, which changes the arithmetic), is a distinct question this card does not answer. A later card can extend `compactEncode` to an MCP-side caller without changing anything this card defines, since the encoder is a pure function of the same Go values the MCP head already holds.
- **A persisted config layer for format.** `--json` has no `dinah config set` equivalent today; `--format`/`DINAH_FORMAT` get the same two-rung treatment (flag over environment) and no third rung, matching the existing precedent rather than inventing a new one for this card alone.
- **Compact renderings for any shape besides `*verb.Response`, `Listing` and `[]Offer`.** See Scope above.
- **dinah-124 and dinah-128.** Neither is touched, absorbed, or blocked on by this card.

## Verification of the token saving

The byte-ratio table on this card's timeline is a proxy; bytes and tokens diverge in exactly the direction that matters here, because JSON's structural characters (`{`, `}`, `"`, `:`, `,`) are typically single tokens under a byte-pair-encoding tokenizer, so a byte saving from removing them overstates the token saving. AC-9 below states the exact measurement Test runs to replace the proxy with a real number before this card is accepted as delivering what it claims.

## Branch

dinah-31-token-lean-output-rendering-for-agent-loops
