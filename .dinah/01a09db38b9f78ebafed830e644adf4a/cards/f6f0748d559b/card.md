---
title: The extension registers Dinah's MCP server with the editor's agent mode
column: b69abf918c42
state: ready
severity: minor
priority: later
tier: workhorse
workstreams:
  - 58f3e3eb621a
links:
  - kind: spawned_from
    to: f67d6e37434c
---
Split out of dinah-271, and held back from the other five because it touches the product's own boundary rather than a technical question.

The extension registers the tool's MCP server with the editor's agent mode for the workbench it found, so a reader's own agent session can reach their workbench without them wiring it up by hand.

**The operator ruled yes on 2026-09-11.** The hold below is lifted and this card may be specced. The rest of this description is the framing that was put to him, kept because it is the reasoning the spec should honour rather than re-derive.

The boundary. The extension's stated refusal is that it is a viewer rather than a harness, and that no button in it runs an agent. dinah-271 argued registering a server is not running an agent, and triage agreed after reading the code: registering publishes a server address into the editor's configuration, and whether anything calls it is decided by the reader's own agent session. The argument holds technically.

It was still a positioning call, because registering the server makes the extension visibly offer an agent capability whatever the mechanism underneath. The question put to the operator was whether the extension registers the MCP server for a discovered workbench, given that registering publishes an address the reader's own agent may or may not use, and given that the extension still runs nothing itself. He answered yes.

What the ruling does not license. The refusal it sits beside is unchanged: the extension remains a viewer, it runs no agent itself, and nothing in this card adds a control that starts one. A spec that grows a button running an agent has gone past what was approved.

## Specification

Written against trunk `65a80ad8b3b525e62af2262ad6d3203876ea6c7a`, read in the worktree `C:/dinah-scratch/dinah-424-spec4`. The acceptance criteria on the card are the contract that gets verified; this prose says what the code does and what the card changes, and where the two ever disagree the criteria govern.

## What ships

The extension publishes one MCP server definition per workbench it has already resolved, through VS Code's own provider API. Each definition starts `<the absolute path the binary reported> --workbench <that workbench's root> mcp`, and the reader's agent session then reaches that workbench without the reader editing any configuration file.

Publishing an absolute path means the binary has to say where it is, which it cannot do today. So `dinah --json version` gains one field, `executable`, and the extension publishes what that field carries. D-9 explains why a bare name will not do, D-10 explains why that CLI change belongs on this card, and D-11 settles what happens when a binary cannot answer.

Nothing about the extension's standing refusal changes. No command starts an agent, no row gains an act, and the extension spawns no server itself. It answers a question the editor asks, and the editor decides whether to start anything. VS Code asks the reader to trust a server before it starts one for the first time, so publishing a definition cannot silently run a process. Two published pages carry that, `code.visualstudio.com/docs/agents/concepts/trust-and-safety` ("VS Code prompts you to trust each MCP server before it runs, and re-prompts after configuration changes") and `code.visualstudio.com/docs/agent-customization/mcp-servers` ("VS Code shows a dialog to confirm that you trust the server when you start a server for the first time", and "If you don't trust the MCP server, it will not be started"). The second page carries one caveat, that a server started directly from the `mcp.json` file prompts for no trust, and it does not reach this card, because nothing here writes an `mcp.json` for a reader to start a server from.

## The documented route, and the exact reach of each citation

`vscode.lm.registerMcpServerDefinitionProvider(id, provider)` is stable API. It first appears in the stable `index.d.ts` at `@types/vscode@1.101.0`, which is determined rather than remembered:

```
for v in 1.99.0 1.100.0 1.101.0 1.102.0; do
  n=$(curl -s https://unpkg.com/@types/vscode@$v/index.d.ts | grep -c 'registerMcpServerDefinitionProvider')
  echo "$v: $n"
done
# 1.99.0: 0   1.100.0: 0   1.101.0: 3   1.102.0: 3
```

Its own doc comment fixes what this card rests on. It "allows MCP servers to be dynamically provided to the editor in addition to those the user creates in their configuration files"; "Before calling this method, extensions must register the `contributes.mcpServerDefinitionProviders` extension point with the corresponding id"; extensions "should call `registerMcpServerDefinitionProvider` during activation" so the editor can present its refresh action; and it returns "A disposable that unregisters the provider when disposed."

`McpStdioServerDefinition` is documented as "an MCP server available by running a local process and operating on its stdin and stdout streams. The process will be spawned as a child process of the extension host and by default will not run in a shell environment." Its `version` field is documented as "Optional version identification for the server. If this changes, the editor will indicate that tools have changed and prompt to refresh them."

Build against the shipped `.d.ts` rather than against the documentation page's sample. The page at `code.visualstudio.com/api/extension-guides/ai/mcp` shows `new vscode.McpStdioServerDefinition({ label, command, args, cwd, env, version })`, an object-literal form. The `.d.ts` installed here declares a positional constructor, `constructor(label: string, command: string, args?: string[], env?: Record<string, string | number | null>, version?: string)`, with `cwd` a settable property rather than a constructor argument. The `.d.ts` is what the compiler enforces and is what this spec is written against. Do not cite `code.visualstudio.com/docs/agents/guides/mcp-developer-guide` for the sample or for the trust behaviour: it answers 301 into `/api/extension-guides/ai/ai-extensibility-overview`, which discusses neither. Every URL this spec does cite was fetched on 2026-09-11 and returned 200.

## Why a path is published and a name is not

Publishing the bare name `dinah` and leaving the editor to resolve it is not available, and D-9 records the reading in full. The `command` field's own doc comment says nothing about a system path, and its one hint points at handing over an already-resolved path. The class comment removes the shell without naming what replaces it. The sentence about the system path lives on the page documenting the `mcp.json` file a reader types by hand, which is a different surface, so bridging the two is an argument from silence and the no-undocumented-behaviour rule refuses it.

Publishing a full path dissolves the question instead of answering it. The MCP configuration reference requires a `command` that is "available on your system path or contain its full path", and a full path satisfies that requirement under either reading of who it binds, so nothing has to be inferred about how a name would be resolved.

The binary is the only party that knows where it is. `binary.ts` resolves either the `dinah.path` setting or the bare name and deliberately implements no PATH search of its own, because a search written there would be a second implementation of something the operating system already does and would get the Windows `PATHEXT` rules subtly wrong (`editors/vscode/src/binary.ts:12-15`). That reasoning still holds and this card does not touch it. What the extension already does, though, is spawn the binary successfully to run `--json version` before it publishes anything, so the running process can report its own location.

### The CLI half

`internal/verb/read.go` gains a field on `VersionReport` and a seam beside it:

```go
// Executable is where this binary is, which a client that must hand a
// third party a runnable command needs and cannot work out for itself.
// Absent when the operating system would not say.
Executable string `json:"executable,omitempty"`
```

```go
// executablePath is os.Executable behind a seam, so the failure arm is
// reachable from a test. bench.statPath is the same shape.
var executablePath = os.Executable
```

`Version` calls it and leaves the field empty when it errors, which `omitempty` turns into an absent key. Go documents `os.Executable` as returning "an absolute path unless an error occurred", and that sentence is the whole guarantee this card takes from it. Go documents two caveats beside it and this card acts on neither, for the reasons D-11 now gives in full: the symlink caveat, where the answer may be the link or its target and either is a runnable absolute path, and the staleness caveat, where the path "is still pointing to the correct executable" is not promised and a reader who replaces their binary between activation and a spawn gets a spawn failure from the editor that nothing in the extension notices.

Two assertions cover the field and they prove different things, so AC-14 keeps them apart. The in-process arm runs under `runCLI` at `cmd/dinah/main_test.go:198-223`, which calls `run(argv, in, out, errw)` inside the test process and spawns nothing, so `os.Executable` there reports the Go test binary's own path. That arm proves the field is populated, that it survives marshalling, and that it is absent when the seam fails. It proves nothing about what a shipped `dinah` reports, and the test's own comment says so rather than leaving a reader to infer it. The arm that proves a shipped binary reports where it is builds one. `editors/vscode/test/support/fixtures.ts:40-50` exposes `buildBinary(repoRoot)`, which runs `go build -o <temp>/dinah ./cmd/dinah` under a temporary root outside the checkout, and it is the only harness in the tree that builds and spawns a real `dinah`. A new unit file, `editors/vscode/test/unit/versionExecutable-live.test.ts`, builds this commit's binary, runs `--json version` against it through `runDinah(nodeSpawner, root.binary, ["version"], options(root))`, and asserts the reported `executable` equals the path `buildBinary` returned. It follows `verbCatalog-live.test.ts` exactly: the build is paid once, `after()` removes the temp root, and the spawn runs with `cwd` at the temp root and `fixtureEnv(root)` for its environment, so neither the discovery walk nor any write can reach the operator's own workbenches. `test/unit/layers.test.ts:86-93` refuses a unit file that starts a process unless `MAY_START_A_PROCESS` names it, so this file is added there with its reason, which is that no other harness in the tree can hold a shipped binary to reporting its own location.

That file carries one more assertion, which AC-17 names. The same payload the built binary returned goes through `classifyVersion` and so through the real `readReport`, and the decoded `version.executable` is asserted to equal the same built path. AC-14's second arm reads the raw JSON and AC-16 reads a payload the test wrote, so without this assertion nothing joins a real CLI to the real decode, and either half could stop carrying the field with both of the others still green.

Canonicalise both paths with `realpathSync.native()` before comparing them, and fold nothing else. Node's `fs` page documents that call as synchronous `realpath(3)`, and POSIX documents `realpath()` as producing an absolute pathname that names the same file with no `.`, no `..` and no symbolic link left in it, so the comparison rests on one documented normalisation applied identically to both operands rather than on two tools spelling one directory alike. Two spellings that no lesser transformation reconciles are in reach here. The extension's Windows CI leg runs on `windows-latest`, where `os.tmpdir()` reads `TEMP` and the runner sets it to a short-name path of the form `C:\Users\RUNNER~1\AppData\Local\Temp`, so `go build -o` is handed a short name while no page fixes what `os.Executable` reports back; and on a POSIX host whose temporary directory has a symlinked component, the kernel's answer carries the resolved spelling and `mkdtempSync` returns the unresolved one. `resolve()` normalises separators and relative segments and resolves neither a symlink nor a short name, and a case fold reaches neither either, so working out which way each case actually falls is a measurement, which the no-undocumented-behaviour rule refuses as a correctness argument. Canonicalising cannot produce a false pass, because AC-14's third plant compares against a path other than the one `buildBinary` returned and requires a red run. A difference that survives canonicalisation on both sides is a real failure, and if the Windows leg ever produces one that is a finding to report rather than a reason to widen the comparison.

This is the unit layer's second `go build`. `verbCatalog-live.test.ts` already builds one, each live file owns its own fixture root, and the new file builds its own binary into its own temporary directory, so both CI legs pay two builds rather than one. AC-14's second arm and AC-17 share this file's single build, so the join across the seam costs nothing beyond it. The `MAY_START_A_PROCESS` entry records that cost beside the reason.

The reach of that change was measured over the whole tree rather than assumed, and D-10 records the commands and their output. Nothing in the repository calls `os.Executable` today, `VersionReport` has five call sites and no more, and no test pins the report's key set, so the field is additive. The MCP head serves the same value through its `version` tool and gains the field for free. The human rendering does not change, no message key is minted, and `render.go` is not edited.

### The extension half

The value does not reach the extension by being added to a type. `readReport` at `editors/vscode/src/version.ts:93-107` is the only decode of `--json version` in the extension, and it builds its result field by field, `return { tool: report.tool, profile: report.profile, format: report.format }`, dropping every key it does not name. The whole path is `activate` calling `classifyVersion(await runDinah(...))` at `extension.ts:363-368`, `classifyVersion` calling `readReport`, and `toState` in `binary.ts` carrying the object out as `binary.version`. So `readReport` gains the field, accepting a string, treating a value of any other type as absent rather than refusing the whole report, and only then does `binary.version.executable` carry anything. AC-16 is the criterion that makes that true, and it asserts against a parsed payload rather than against a hand-built `VersionReport`, because a hand-built one passes over a decoder that drops the field. A spec that assumed the decoded report already carried the value reached review twice on the strength of exactly that.

Two comments in that module go stale in the same diff and are part of the change. The module header at `version.ts:3-4` says `dinah --json version` "reports three fields with three jobs, and this gate reads two of them": the count becomes four and the clause about the gate reading two of them stays true, because `executable` is not a gate field and nothing compares it. `readReport`'s own doc comment, "Reads the three fields off a parsed `--json version` payload", moves with it.

The decision about what to publish is a pure function in `editors/vscode/src/mcpServers.ts` and not a branch in the callback. Putting the ladder inside the inline `provideMcpServerDefinitions` arrow function in `extension.ts` would leave nothing on this card able to exercise it. `test/unit/layers.test.ts:161-191` allows `extension.ts` alone to import `vscode` as a value, its own comment records that no unit test imports `extension.ts`, a unit test that did would fail at load because `vscode` does not resolve outside an editor host, and this card's out-of-scope list rules the integration suite out, so the refusal the whole card rests on would reach Test unproven. `mcpServers.ts` imports `BinaryState` from `./api` as a type, which the layer rule permits and `status.ts` already does, so it takes the whole binary state and decides; `extension.ts` keeps the setting read, the emitter, the mapping to vscode values and the registration call, and nothing else. That is the shape `status.ts`, `diagnostics.ts` and `binary.ts` already use.

## Files

- `internal/verb/read.go`. The `Executable` field, the `executablePath` seam, and one line in `Version`.
- `editors/vscode/src/mcpServers.ts`, new, pure. Holds the publish-or-refuse decision, composes the published set, and answers whether two plan sets differ. It imports no vscode value, because `test/unit/layers.test.ts:161-191` allows `extension.ts` alone to do that, and the sibling test at `193-196` keeps that rule non-vacuous; it imports `BinaryState` from `./api` as a type, which that rule permits and `status.ts` already does. It returns plain data and `extension.ts` builds the `vscode.McpStdioServerDefinition` values from it.
- `editors/vscode/src/api.ts`. One optional field on `VersionReport`.
- `editors/vscode/src/version.ts`. `readReport` carries the new field through, and the module header and `readReport`'s doc comment move with it. Without this the field is undefined for every binary and the feature publishes nothing; AC-16 is the criterion.
- `editors/vscode/src/tree.ts`. One new accessor, `mcpTargets()`.
- `editors/vscode/src/identity.ts`. Two new constants.
- `editors/vscode/src/extension.ts`. The provider registration and its wiring.
- `editors/vscode/package.json`. The contribution point, the new setting, `engines.vscode` and `@types/vscode`.
- `editors/vscode/package.nls.json` and its seven siblings. Two new keys each.
- `editors/vscode/src/locales/flags.json`. Skeleton and source entries for those two keys.
- `editors/vscode/README.md`. One paragraph under "What it gives you" and one sentence under "Requirements".
- `editors/vscode/test/unit/layers.test.ts`. One entry in `MAY_START_A_PROCESS`, with its reason, for the live version test below, and the declaration's own doc comment at `layers.test.ts:74-85`, which goes with it. That comment says "Both of these check a command-line contract" above three entries, and says the processes "neither needs a Go toolchain and neither builds anything, so between them they cost the layer under two seconds". `verbCatalog-live.test.ts` already denies both clauses, and this card adds a fourth entry that also builds a binary. The replacement says the exemptions are of two kinds, the two that drive a script and the two that build and run this commit's `dinah`, and gives what each kind costs the layer, which is now two `go build` runs.
- Tests: `editors/vscode/test/unit/mcpServers.test.ts` (new), `editors/vscode/test/unit/versionExecutable-live.test.ts` (new, builds and spawns a real `dinah`), `editors/vscode/test/unit/version.test.ts`, `manifest.test.ts`, `l10n.test.ts`, `tree.test.ts`, and a Go test in `cmd/dinah` for the version field.

`version.test.ts:15`'s `reported(tool, profile, format)` helper builds the `CliOutcome` the decode is fed. It gains an optional fourth parameter carrying the payload's `executable` as `unknown`, absent by default, so every existing row is unchanged and the type-rejection case can be expressed.

No file in `src/locales/*.json` gains a key, because nothing published at run time carries an authored English sentence. D-4 settles that.

## The published set

`DinahTreeProvider.mcpTargets()` is the source, and it is new because `rootsFor` cannot serve this. `rootsFor` hands back the title `holdingReportOf` already substituted, which is `UNTITLED_WORKBENCH`, spelled `"Dinah"` at `tree.ts:701`, for a workbench with no title of its own. A label built on that reads `Dinah: Dinah`, and worse, it is indistinguishable from a workbench genuinely titled `Dinah`.

```ts
/** One workbench this window has resolved, as the MCP provider needs it. */
export interface McpTarget {
  /** The workbench's own absolute root, as `dinah --json status` reported it. */
  readonly root: string;
  /** The workbench's own title, empty when it has none. Never substituted. */
  readonly title: string;
}

mcpTargets(): readonly McpTarget[];
```

It walks every folder's rows exactly as `rootsFor` does, keeps a row only where `holdingReportOf` would call it `answered`, and carries `data.title` through unsubstituted. It folds nothing and drops nothing, so two workspace folders that both resolve to one workbench yield two targets here and `planMcpServers` is what reduces them to one plan. Order is workspace-folder order, then row order.

Deduplication does not happen in this method. It belongs to `planMcpServers`, which already takes the case-sensitivity flag and which is the function AC-4's assertion calls, so the rule is asserted against the code that performs it rather than against a fold hidden behind `DinahTreeProvider.rootKey`, which is private at `tree.ts:1691` and reads the provider's own `deps.caseInsensitive` rather than an argument.

The kept-row predicate is the whole of `holdingReportOf`'s condition rather than a summary of it: `row.rowKind` is not `deadEnd`, the row carries `data`, `data.path` is non-empty, and `data.fetchedAt` is set. The rowKind clause comes first in that function, at `tree.ts:1421`, and returns before `data` is looked at, so a predicate written without it is a narrower thing than the function it restates. Nothing tells the two apart today, because `deadEndRow` at `tree.ts:2013` builds no `data`. The clause is written down so that they cannot drift apart later.

A folder in `candidates` mode publishes nothing until the reader expands a candidate, because until then no row carries `data` and this window has not established which workbench the folder holds. A folder in `forest` mode publishes one definition per member it has resolved. A folder in `dead-end` mode publishes nothing.

## Composing a definition

`src/mcpServers.ts`:

```ts
/** A published server, as plain data extension.ts turns into a vscode value. */
export interface McpServerPlan {
  readonly label: string;
  readonly command: string;
  readonly args: readonly string[];
  readonly cwd: string;
  readonly version: string;
}

export function planMcpServers(
  targets: readonly McpTarget[],
  executable: string,
  toolVersion: string,
  caseInsensitive: boolean,
): readonly McpServerPlan[];

/**
 * Every plan this window publishes, which is often none.
 *
 * This is the publish-or-refuse decision, and it lives here rather than in the
 * callback because no unit test can reach extension.ts.
 */
export function publishedMcpServers(
  binary: BinaryState,
  registerMcpServer: boolean,
  targets: readonly McpTarget[],
  caseInsensitive: boolean,
): readonly McpServerPlan[];

/** Whether two plan sets differ by value, which is what fires the change event. */
export function mcpPlansDiffer(
  previous: readonly McpServerPlan[],
  next: readonly McpServerPlan[],
): boolean;
```

`publishedMcpServers` returns an empty array on each of four rungs, in this order, and reaches `planMcpServers` only past all four:

1. `registerMcpServer` is false. The reader's own switch is read first, so turning it off costs nothing else.
2. `binary.state !== "ok"`. No binary resolved, so nothing knows where one is.
3. `binary.version.executable` is absent or empty. An older binary that predates the CLI half reports no such field and is indistinguishable from one whose `os.Executable` failed, and D-11 settles that both publish nothing.
4. `isAbsolute(executable)` from `node:path` is false. No documented path produces a relative answer, so this rung guards a future regression rather than a case reachable today, and it refuses rather than falling back to the bare name.

`planMcpServers` publishes one plan per distinct root. Two targets collapse to one when `key(a) === key(b)`, where `key(root)` replaces every `\` with `/` and, when `caseInsensitive` is true, lowercases the result. Nothing else is normalised: no relative segment is resolved, no symbolic link is followed and no trailing separator is trimmed, because every root here is a path `dinah --json status` reported for a workbench it had already resolved, and both sides of any comparison come from that one source. The comparison is over the whole folded string, so `C:/w` and `C:/w-old` remain two plans. First occurrence wins, and order is otherwise the order the targets arrived in. That fold restates the rule `DinahTreeProvider.rootKey` applies for its own purposes rather than sharing it, because `rootKey` is private and reads the provider's flag rather than an argument.

`mcpPlansDiffer` compares length first, then every field of every plan in order, with `args` compared element by element. Object identity decides nothing, because the set is recomputed from fresh objects on every checkpoint and an identity comparison would report a change every time.

- `command` is `executable`, the absolute path the binary reported, verbatim. The function composes no path of its own and never substitutes a name. Its only caller is `publishedMcpServers` in the same module, which reaches it only past a rung that refuses anything but an absolute path, and AC-15 pins that.
- `args` is `["--workbench", target.root, "mcp"]`, in that order. `--workbench` is a global flag and precedes the command word, which is the argv `cmd/dinah/mcp_startup_test.go:46` and `:51` already exercise.
- `cwd` is `target.root`. This is belt and braces rather than load-bearing, since `--workbench` already pins the workbench and `cwd` only keeps a relative path the reader might later type resolving somewhere sensible.
- `env` is empty. The extension imposes nothing on a spawned `dinah`, which is the operator's 2026-09-06 ruling as `extension.ts:352-361` records it for `DINAH_LANG`. No `DINAH_ACTOR`, no `DINAH_WORKBENCH`, no `DINAH_MCP_ROOT`.
- `version` is `binary.version.tool`, the binary's own release number on `resolveBinary`'s `ok` arm (`api.ts:11-12`, `binary.ts:38-45`). D-5 settles why that field and not `profile`, and why this is not the comparison `api.ts:11` forbids.

### Why `--workbench` and not `--root`

`runMCP` at `cmd/dinah/commands.go:1642-1697` resolves `--root`/`DINAH_MCP_ROOT` into `s.mcpRoot`, which bounds which workbenches may be named, and separately opens the session's own library through the workbench ladder, which becomes the default answering a call that names none. `cmd/dinah/main.go:131-134` puts `SourceFlag` above `SourceEnvironment`, so `--workbench` outranks a `DINAH_WORKBENCH` the extension host happens to carry. `TestMCPKeepsDiscoveredDefaultButStopsNarrowingWithNoRoot` in `cmd/dinah/mcp_startup_test.go` pins that a server with a default but no root still answers a call naming a different workbench by absolute path. Passing no `--root` is therefore deliberate. The reader's agent gets their workbench for free and is fenced out of nothing, where `--root` would silently narrow what their agent can reach.

### Labels

The label is the only handle this card gives the reader, and the `.d.ts` describes it only as "The human-readable name of the server". Where the editor draws it is not documented anywhere I could find, and nothing below depends on knowing.

- `Dinah: <title>` where `target.title` is non-empty after trim.
- `Dinah: <basename of root>` where it is empty.
- Where two plans would carry the same label, every plan carrying it gains ` (<root>)`, using the root exactly as `dinah` reported it. Not only the second one. A reader cannot tell which of two same-titled workbenches got the bare label.
- The comparison for that collision check uses the same `key(root)` fold the deduplication above uses, so it folds case where `caseInsensitive` is set and does not otherwise.

## Registration in extension.ts

`identity.ts` gains:

```ts
/** The id the manifest and the registration call must both spell. */
export const MCP_PROVIDER_ID = "dinah.workbenches";

/** The settings key a reader turns MCP registration off with. */
export const SETTING_REGISTER_MCP = "dinah.registerMcpServer";
```

`package.json` gains, beside `configuration`:

```json
"mcpServerDefinitionProviders": [
  { "id": "dinah.workbenches", "label": "%manifest.mcp.provider.label%" }
]
```

The id is dot-namespaced to match every other contribution id this extension declares, which is the shape `dinah-423 D-3` settled for the walkthrough ids. The DIN-HIN dash convention governs the CLI's package and command names and governs nothing in this manifest, where `dinah.workbenchView` and `dinah.firstSession` are already the established spelling.

In `activate()`, after the tree provider exists, unconditionally:

```ts
const mcpChanged = new vscode.EventEmitter<void>();
context.subscriptions.push(mcpChanged);

let published: readonly McpServerPlan[] = [];
const currentPlans = (): readonly McpServerPlan[] =>
  publishedMcpServers(
    binary,
    settingOf<boolean>(SETTING_REGISTER_MCP, true),
    provider.mcpTargets(),
    process.platform === "win32",
  );

context.subscriptions.push(
  vscode.lm.registerMcpServerDefinitionProvider(MCP_PROVIDER_ID, {
    onDidChangeMcpServerDefinitions: mcpChanged.event,
    provideMcpServerDefinitions: () => {
      published = currentPlans();
      return published.map(toDefinition);
    },
  }),
);
```

Every rung of the ladder is inside `publishedMcpServers`, so there is nothing here to get wrong and nothing here a unit test would have to reach. `binary` is passed whole rather than unpacked, and the three arguments beside it are values rather than decisions: `settingOf` is the same read the rest of `activate` uses, and `process.platform === "win32"` is the same `caseInsensitive` expression `activate` already computes at `extension.ts:371`. AC-15's second plant is what holds the decision here: move the absoluteness check back up into this callback and delete it from the pure function, and the third arm goes red.

`toDefinition` is the one impure step. It constructs `new vscode.McpStdioServerDefinition(plan.label, plan.command, [...plan.args], {}, plan.version)` and then assigns `definition.cwd = vscode.Uri.file(plan.cwd)`, because the installed `.d.ts` takes `cwd` as a property rather than as a constructor argument.

Neither that construction nor the `registerMcpServerDefinitionProvider` call above it carries a type assertion, a `@ts-expect-error` or a `@ts-ignore`. A clean `npm run compile` does not establish that, because a cast is precisely what makes a call against types that do not declare it compile, and nothing in this extension's `eslint.config.mjs` bans a type assertion. So AC-10 asserts it separately, with a sweep over the text of `extension.ts`, and the compile step is left proving only what it can prove.

`resolveMcpServerDefinition` is not implemented. It is optional, it is where an extension "may take any actions which may require user interaction, such as authentication", and this server needs none. Implementing it to do nothing would add the one hook that could later grow into the harness behaviour the card forbids.

### Keeping it in step

Two things fire `mcpChanged`:

1. The checkpoint refresh hook at `extension.ts:711-720`, after `provider.refresh(folder)` has run, and only when the plan set has changed by value: `const next = currentPlans(); if (mcpPlansDiffer(published, next)) { published = next; mcpChanged.fire(); }`. The comparison is the pure `mcpPlansDiffer`, so the rule that decides how often a reader is prompted is asserted in `mcpServers.test.ts` rather than sitting in the one module no test reads. Firing on every checkpoint would prompt the reader to refresh their tools every poll interval, which the `version` field's documented behaviour makes a real cost rather than a tidiness point.
2. `vscode.workspace.onDidChangeConfiguration`, filtered with `event.affectsConfiguration(SETTING_REGISTER_MCP)`, which fires unconditionally because the event says the reader changed that key. Turning the setting off therefore takes effect without a window reload.

The set is otherwise recomputed lazily, inside `provideMcpServerDefinitions`, whenever the editor asks.

### Three limitations that are inherited rather than introduced

The first limitation is the binary. `resolveBinary` is called once, at `extension.ts:363`, and nowhere else, so a reader who installs the binary after the window opened gets nothing from this feature until the window reloads. They get nothing from the tree, the status bar or the diagnostics either, and one reload fixes all four. D-14 records why this card does not change that.

The second is workspace folders added or removed after activation. `src/` carries no `onDidChangeWorkspaceFolders` subscriber at all, which `grep -rn 'onDidChangeWorkspaceFolders' editors/vscode/src` answers with nothing, so the tree does not track folder changes either. A folder opened after activation reaches the published set when the window reloads.

The third is activation itself. No activation event is added, because `manifest.test.ts:214-222` asserts the list is exactly `workspaceContains:**/workbench.md` and `onView:dinah.workbenchView`, and that assertion stands unchanged. A reader whose workbench sits above their workspace folder, reached by the ancestor walk rather than by a `workbench.md` inside the folder, does not activate on `workspaceContains` and gets no registration until something else activates the extension, such as opening the Dinah view. Widening activation to `onStartupFinished` is forbidden by the same test.

## Declining it

`dinah.registerMcpServer`, boolean, default `true`, scope `window`.

The default is `true` because the operator ruled that the extension registers. The scope is `window` rather than `resource` because `provideMcpServerDefinitions` answers once for the whole window with one flat array, and the published set is deduplicated across folders, so a root that two folders both reach would need a reconciliation between two folder-scoped answers and there is no defensible one. `dinah.path` is already `machine-overridable` rather than `resource` for the same kind of reason, that the setting's subject is not the folder.

## Removal

Nothing is written to the reader's disk, so removal is not a file operation. The provider registration is a `Disposable` pushed onto `context.subscriptions`, which the editor disposes when the extension deactivates, and its own doc comment says disposing it "unregisters the provider". Turning the setting off empties the published array. Disabling or uninstalling the extension removes the provider entirely. The reader can also disable an individual published server from the editor's own MCP server list.

### Coexistence with a server the reader registered themselves

The API is documented to provide servers "in addition to those the user creates in their configuration files", and no documented API lets an extension read `.vscode/mcp.json`. So the extension does not look, does not deduplicate against it, and does not warn. A reader who has registered `dinah mcp` by hand sees two entries, told apart by the `Dinah: ` label, and turns one of them off. Reaching into a reader's own configuration file through an undocumented read, in order to suppress something they chose to write, is worse than showing them both.

## Reader-visible text

Two new keys, both in the manifest namespace, both carried by all eight `package.nls*.json` files:

- `manifest.mcp.provider.label`, the provider's human-readable name.
- `manifest.configuration.dinah.registerMcpServer.markdownDescription`, the setting's description.

Both must say what they say without tripping the guard at `manifest.test.ts:875`, which refuses a manifest string claiming the extension itself carries or has a `dinah`. On `65a80ad` that guard covers one of the two and not the other. Its collector at `manifest.test.ts:887-908` reads the manifest description, both description forms of every contributed setting, and every welcome block, so the setting's `markdownDescription` is inside the corpus and the provider's `label` is outside it. The arithmetic that counts the corpus a second time is elsewhere, at `914-923`, with the literal it is checked against at `925`. Those are three separate edits, and a change that moves the count without extending the collection is what the two counts exist to catch. D-13 settles the repair, which is to extend the collector to `contributes.mcpServerDefinitionProviders` on its derived half and its literal half together. `resolveNls` at `manifest.test.ts:84-105` walks the whole manifest, so the collector receives the English rather than the `%key%`.

That same guard performs no README check and must not be credited with one. It never opens `README.md`, so the README's own sentences are guarded by a new check beside it, reusing the `claimsToCarryDinah` helper at `manifest.test.ts:512` so that one definition of the refusal serves both surfaces. AC-13 carries the criterion and the plant.

The rest of the localisation contract, read off the code rather than restated from memory:

- `l10n.test.ts:122-137` holds every `package.nls.<tag>.json` to exactly `package.nls.json`'s key set, so all eight files gain both keys or the suite fails.
- `l10n.test.ts:167-180` requires `flags.skeleton[tag]` to be empty for `de` and `hi` and to equal the complete key list for `af`, `cs`, `es`, `fil` and `id`. German and Hindi must therefore be genuinely translated, since a skeleton is not available there.
- `l10n-staleness.test.ts:90-113` requires `flags.source["de"]` and `flags.source["hi"]` to carry `fingerprint(<the English text>)` for every non-skeleton manifest key, using the FNV-1a 64-bit function in `src/fingerprint.ts`.
- `l10n.test.ts:275` holds a translated entry to rendering each declared glossary term with a declared word. `flags.json`'s glossary declares `state`, `the root`, `owner`, `level`, `column`, `collection` and `the archive`. "The root" is the one an English sentence about a workbench root is likely to reach for, so a German rendering of it uses `Wurzelverzeichnis` and a Hindi one uses `रूट`.

Counted concretely, and each count is the floor a criterion asserts: 8 manifest files each gain 2 keys, `flags.skeleton` gains 2 entries in each of 5 skeleton tags, and `flags.source` gains 2 entries in each of 2 translated tags.

## What the version floor costs

`engines.vscode` moves from `^1.90.0` to `^1.101.0` and `@types/vscode` moves with it, because the API does not exist below that and shipping a manifest contribution point an older editor cannot honour would rest on behaviour documented nowhere. The pair is asserted against itself rather than against two literals, so the floor cannot half-move, and `npm run compile` against the installed types is what shows the API is actually declared there. That is the technical half, and D-7 carries the other half, which the operator should read before he accepts this at his station.

Every reader on VS Code 1.90 or later can install this extension today and receives each new version as it is published. Raising the floor ends that for everyone between 1.90 and 1.100. Nothing breaks for them and they keep the copy they have, but the Marketplace stops offering them new versions, and that means every future version rather than only this one, so a fix to the tree or the status bar that has nothing to do with MCP will not reach them either. Nothing tells them; an extension simply stops updating. Most of them can update VS Code and will. The reader this genuinely strands is the one who cannot, on a managed installation somebody else pins or on a remote host whose server version is not theirs to choose, and nobody has counted how many of those there are because this project collects no telemetry that could. The alternative that would avoid the cost is publishing two builds from one source, and D-7 records it as rejected for being out of proportion to a single-seat companion extension.

## Constraints and out of scope

- No command, no menu item, no row act, no status-bar affordance. Nothing in this card starts an agent or offers to.
- `.vscode/mcp.json`, the user-profile `mcp.json` and `settings.json` are not written, read or watched.
- The CLI's human rendering of `dinah version` is unchanged, no CLI message key is minted, and `render.go` is untouched. That also keeps this card clear of the fixtures that key on line numbers in `render.go` and `docs/quick-start.md`.
- `bench.ProfileMinor` does not move and the core profile's changelog gains no entry, because no statement of the profile changed. `MINIMUM_PROFILE` in the extension does not move either. D-12 records both, with the search that establishes the first.
- `docs/design/surfaces.md` is not edited. Its "The VS Code extension" section describes the client-surface ladder (LSP and verbs, the sidebar tree, the in-binary TUI, the webview), and MCP registration is not a rung of that ladder.
- The VS Code integration suite is not extended and is not run. It starts a real editor host, and nothing in it can settle what the editor does with a registration.

## What each load-bearing check actually touches

Six review rounds across this card and its sibling have all found one defect. A check's sentence names one thing and its assertion touches another. The README guard never opened the README. The absolute-path assertion read the Go test binary's own path. The ladder sat in a module no unit test can load. So this section answers, for each check carrying a load-bearing claim, what object the assertion compares at the moment it runs, rather than which file or module the check lives in. Naming the file is one layer short, and it is what let four wrong answers through the last review. Where the answer is weaker than the criterion reads, that is written here.

- **AC-1** compares two strings. The id half compares `contributes.mcpServerDefinitionProviders[0].id`, read from `package.json`, against `MCP_PROVIDER_ID` imported from the compiled `identity.ts`. The label half reads its operand from a fresh `JSON.parse` of `package.json`, because the file's `manifest` binding at `manifest.test.ts:105` is `resolveNls`-resolved and `resolveNls` at `84-103` has already replaced a lone `%key%` with its English. An assertion reading that binding sees `Dinah workbenches` whether the manifest carries the key or a hard-coded sentence, so the raw parse is what makes the second plant able to go red.
- **AC-2** compares `Object.keys(contributes.configuration.properties)` against a literal list with `deepEqual`, and the new property's `scope`, `type` and `default` against literals. Its corpus half compares one number three ways: the collected array's length, an expression derived from the manifest, and the literal `13` at `manifest.test.ts:925`. The collector is at `887-908` and the derived arithmetic at `914-923`, so they are two edits rather than one, and a change reaching the arithmetic alone would move the count without extending the collection.
- **AC-3** compares the fields of one returned `McpServerPlan` against literals, and its `command` against the very string the test passed as `executable`. That is not a value compared with itself, because the function under test is free to compose a different one, and the plant that rewrites `command` to `PATH_NAME` is what proves the assertion can see the difference.
- **AC-4** compares the length of `planMcpServers`'s returned array across three fixture pairs under both settings of `caseInsensitive`. The object that folds is `planMcpServers`'s own dedup key, which is why this spec puts the fold there rather than in `mcpTargets`: a fold performed in the tree provider would leave this criterion asserting a property of code it never calls.
- **AC-5** compares composed `label` strings against literals. No path, no file and no process.
- **AC-6** compares the roots in `mcpTargets()`'s returned array against the fixture's own rows, and its length against a number the test computes from the fixture. `holdingReportOf` is a plain `function` at `tree.ts:1421` and is not exported, so the expected side is the test's own restatement of that predicate and no assertion here touches the function the criterion names. What the count buys is anti-vacuity rather than agreement with `holdingReportOf`, and that is the whole of its job.
- **AC-7** compares regex matches against the text of every `.ts` file under `editors/vscode/src/`, read from disk in the test process, and compares the number of files read against zero.
- **AC-8** compares key sets between the eight `package.nls*.json` files, compares `flags.skeleton` and `flags.source` entries against lists derived from `SUPPORTED_TAGS`, and compares each stored fingerprint against one recomputed from the English of the day.
- **AC-9's first half** compares regex matches against the text of `editors/vscode/src/mcpServers.ts`, and that file's length against zero. It does not run the composition under eight localisers. `planMcpServers` takes no localiser, no module under `src/` holds one at module level, and a loop calling one function eight times with identical arguments compares eight identical results and cannot fail, so the claim is held structurally instead.
- **AC-9's second half** is not an assertion in any test. Test reads `git diff --unified=0 <merge base>..HEAD` over `identity.ts` and `package.json` and confirms it adds no line inside `TREE_COMMANDS` and no entry under `contributes.commands`. The object examined is this card's own diff, it is read by whoever verifies the card rather than by a runner, and nothing re-checks it afterwards. The steady-state property is held by the two existing roster guards and needs nothing from this card.
- **AC-10's compile half** is a build step rather than an assertion. `npm run compile` type-checks `src/` against the installed `@types/vscode` and its evidence is an exit status. **AC-10's version half** compares the `engines.vscode` string against the `@types/vscode` devDependency string, both read from `package.json`. **AC-10's sweep half** compares regex matches against the text of `extension.ts`. The sweep is the only one of the three that can see a cast, because a cast is what makes a bad call compile.
- **AC-11** is performed rather than asserted. A stage runs each plant, watches a suite go red, reads the test count that red run reported, restores the file and watches it go green, and records the output in a card comment. Nothing in the repository checks that any of that happened.
- **AC-12** compares arrays returned by `publishedMcpServers` against emptiness and non-emptiness for one fixture under both settings of the reader's switch, and compares a count of how many times `mcpPlansDiffer` answered yes against `0` over an unchanged fixture and `1` over one that gained a root. The `BinaryState` on every arm is constructed by the test, so nothing here reaches a binary or a decode.
- **AC-13** compares `claimsToCarryDinah`'s return over the text of `README.md` against the empty array, and compares the key names in `contributes.configuration.properties` against occurrences of those names in the README text. The guard at `manifest.test.ts:875` opens no README at all, which is why this is a new check beside it rather than a claim laid on that one.
- **AC-14's first arm** compares the `executable` value marshalled by `run()` inside the Go test process against `filepath.IsAbs` and against an `os.Stat` of it, and on the seam arm compares the marshalled key set against one lacking the key. The process observed is the Go test binary rather than a shipped `dinah`, and the test's own comment says so.
- **AC-14's second arm** compares `realpathSync.native()` of the `executable` a spawned, freshly built `dinah` reported against `realpathSync.native()` of the path `buildBinary` handed to `go build -o`. That child process is the only shipped-shaped binary anything on this card observes. Both operands go through the same documented canonicalisation, so what is compared is whether two spellings name one file rather than whether two tools spell one directory alike.
- **AC-15** compares four arrays returned by `publishedMcpServers` against emptiness and, on the publishing arm, against a one-element array whose `command` is the absolute path the test put on the constructed `BinaryState`. Its two sweeps compare regex matches against the text of every `.ts` file under `src/`, each comparing the number of files read against zero. The four arms prove the decision is in the pure function; the `isAbsolute` sweep is what adds that no other file under `src/` performs it, which is the half that catches a copy left in `extension.ts` beside the original rather than moved there. Neither half can catch a bare name composed character by character, which the criterion says of itself.
- **AC-16** compares the `executable` on the `VersionReport` that `classifyVersion` returned against the value in a payload the test itself wrote, in three cases: a string, a value of another type, and absence. It proves the decode carries what it is given and proves nothing about what the CLI sends.
- **AC-17** compares `realpathSync.native()` of the `executable` on the `VersionReport` that `classifyVersion` returned, decoded from the payload a built binary actually printed, against `realpathSync.native()` of the path `buildBinary` returned. It is the only assertion on the card whose two operands come from opposite sides of the CLI-to-extension seam, which is why AC-14's second arm and AC-16 can both stay green while the feature publishes nothing.

Two comparisons on this card put a path one tool produced beside a path another produced, and they are AC-14's second arm and AC-17. Both canonicalise each operand with `realpathSync.native()` first, for the reason "The CLI half" gives. Every other path handling here compares two strings from one source: `planMcpServers`'s dedup folds two roots that `dinah --json status` reported, and the sweeps in AC-7 and AC-15 compare a walked file path against a literal file name through one `relative()` call, which is the shape `layers.test.ts:161-191` already uses. That inventory was produced by a sweep over the card's whole stored text rather than by reading the sections that looked relevant.

What nothing on this card touches is the editor. The registration call, `toDefinition`, the `McpStdioServerDefinition` construction and the `cwd` assignment are exercised by no test here, because the integration suite is out of scope and the unit layer cannot load `extension.ts`. Three things bound the risk and none of them is a run against an editor. The code there is a mapping with no branch of its own. AC-10's sweep requires that no type assertion, `@ts-expect-error` or `@ts-ignore` reach either API call site, which the compile step cannot establish, since a cast is what makes a bad call compile. AC-1 holds the manifest's provider id to the constant the registration call passes. A mistake in that mapping ships. Widening this means running a real editor host, which the out-of-scope list refuses because nothing in that suite can settle what the editor does with a registration.

## What this spec did not run

The Spec column does not run the product's tests, so no arming pass was performed here and no plant was watched go red. Every criterion carries the plant that arms it and the text a red run must print, and AC-11 makes performing each of them, recording its red output and its test count, an obligation of the stages that build and verify this card. That obligation names no criterion range, so a criterion filed later cannot fall outside an obligation nobody noticed had shrunk, which is what a range written once does the moment a criterion lands past its end. The `dinah-461` pattern applies, where a spec's arming recipe turned out to be vacuous and the implementer found it by performing the recipe rather than by reading it, and this card has already met that trap. An earlier AC-13 named a guard that never opens the README, so its plant could not have gone red, and only performing it showed that.

What this spec did do is read the tree, in the worktree named at the top. Every claim it makes about `version.ts`, `binary.ts`, `api.ts`, `status.ts`, `tree.ts`, `extension.ts`, `cli.ts`, `test/unit/layers.test.ts`, `test/unit/manifest.test.ts`, `test/unit/version.test.ts`, `test/unit/verbCatalog-live.test.ts` and `test/support/fixtures.ts` was checked against the file on `65a80ad` rather than against an earlier draft of this prose, and every line number in it resolves there. The inventory of path comparisons in the section above was produced by a sweep over the card's whole stored text, criteria and notes included, rather than by reading the passages that looked relevant. No URL was fetched for this draft and no editor was probed, and the fetches recorded above stand. Nothing in this spec rests on a measurement of a running editor, and nothing in it rests on a measurement of a running anything.

## Branch

dinah-424-the-extension-registers-dinahs-mcp-server-with-the-editors-agent-mode
