# Lean Dinah pilot

## Status and decision

The bounded pilot does not support adopting a lean replacement for Dinah development. All six corrected fixture arms passed their independent acceptance checks, and both lean arms reached the real candidate workbench's operator-owned Acceptance column. Against the primary direct-plus-review baseline, the lean arms reported 682,460 input tokens versus 649,787. Their uncached portions were 67,164 versus 93,371, and their output totals were 13,602 versus 12,426. Cache composition and run order moved in opposite directions, so the lower uncached figure does not establish a saving. The lean arms consumed more reported input and output overall.

The prototype is usable for another isolated run. The two Python fixtures are too small to establish quality equivalence for full Dinah feature work or reduced human coordination burden. They provide no reliable workflow or subscription-savings evidence. A production rollout or shared-core change needs a separate decision after a wider matched study.

## Candidate route

The committed workbench template has Ready, Working, Review, Acceptance, and Done columns. Acceptance belongs to the operator. The adapter creates a card in Ready, moves it to Working with the last observed revision as `basis`, and assigns the builder. It moves a completed builder result to Review, releases the builder, and assigns a separate reviewer. A successful external evaluation lets the reviewer release the card and request the move to Acceptance. The adapter stops there.

The installed `dinah.exe` created an isolated workbench from the template during the measured pilot. Both lean cards reached Ready, Working, Review, and Acceptance through Dinah's MCP surface. A lifecycle smoke also demonstrated that a claimed card cannot enter an awaiting-outside column. Releasing the reviewer before the guarded move allowed each card to reach Acceptance. An automated actor cannot leave Acceptance for Done.

Dinah owns each claim and transition. The adapter stops when the MCP response is a refusal. Mutating MCP calls carry the last card revision as `basis`. Instruction hashes, workbench revisions, card revisions, starting code state, and change cursors are polled before dispatch and before accepting a result. Those separate reads do not make instruction freshness atomic, so the prototype makes no such claim.

## Packets and recovery

A builder packet contains generated run and attempt identifiers, the task contract, permitted scope, acceptance description, applicable decisions, instruction hash, starting code revision, workbench revision, card revision, changes cursor, and evidence requirements. Additional context belongs in content-addressed references with a revision and byte count.

The current launcher uses `--ephemeral`, so it does not retain a session for continuation. A later builder round must receive an explicit recovery packet containing the prior result, review findings, current revisions, and the changed-file manifest. The pilot never claims that this reproduces hidden model state.

The reviewer packet is assembled separately. The reviewer may inspect the workspace beyond the builder report. The external acceptance evaluator stays outside both worker prompts and is identical across the three arms of a fixture.

## Launcher

The prototype checks the installed command's version and help before launching a worker. The measured run pinned `C:\Users\paul\.vscode\extensions\openai.chatgpt-26.930.31730-win32-x64\bin\windows-x86_64\codex.exe`, which reported version 0.160.0. Each measured process received the prompt on standard input and ran this argument shape:

```text
codex exec --ignore-user-config -c windows.sandbox="elevated" --sandbox workspace-write -m gpt-5.6-sol --json --ephemeral --skip-git-repo-check -C <fixture-workspace> -
```

The command array is passed directly to `subprocess.run`. Shell interpolation does not carry the prompt. The record includes the exact array, prompt hash, workspace, model, start and finish times, exit status, thread identifier, transcript path, and transcript hash. Ambient environment values and credentials are omitted. `--ignore-user-config` prevents the parent session's `gpt-6.1-sol` choice from changing the measured model.

The [Windows sandbox documentation](https://learn.chatgpt.com/docs/windows/windows-sandbox) requires a native sandbox mode. `--ignore-user-config` omitted the existing `[windows] sandbox = "elevated"` setting, so the first write attempts ran read-only even though the argument array requested `workspace-write`. The explicit override fixed that mismatch without weakening confinement. A one-file probe created `ready.txt` before the corrected campaign started. The launcher makes no hard read-isolation claim.

## Usage ledger

Every process has an adapter run identifier. The ledger accepts usage only when a successful process has one terminal `turn.completed` event with integer `input_tokens`, `cached_input_tokens`, and `output_tokens`. It preserves `reasoning_output_tokens` and `cache_write_input_tokens` when reported. Cached input remains a subset category and is never added to input.

An unsuccessful process, truncated JSON Lines stream, missing terminal event, or conflicting terminal event remains incomplete. Unknown usage stays unknown rather than becoming zero. Re-importing the same run and identity is idempotent. Reusing a run identifier with a different command, prompt, transcript, state, or usage is refused.

The report separates setup and development activity from measured task execution. The earlier readiness probe reported 19,339 input tokens, 6,912 cached input tokens, and 5 output tokens. That fixed harness overhead is setup evidence, not Dinah coordination cost. Development-session usage was not measured and remains unknown.

## Evidence receipts

A receipt binds the repository revision, dirty-state manifest hash, task hash, acceptance-input hash, command array, relevant configuration hash, exit status, and output hash. A changed output or changed merge candidate invalidates the receipt. An empty automated-check response remains unknown.

The tests exercise matching and stale receipts, changed instructions, late task-affecting decisions, stale card revisions, operator-only completion, duplicate usage delivery, conflicting duplicate delivery, missing usage, and accepted counterparts. The unknown-usage test was armed by replacing a failed process's missing usage with three zero categories. The red run reported that a `Usage` value was not `None`; restoration returned the suite to green.

## Fixtures and comparison method

Fixture manifests name the task file, workspace seed, external acceptance command, and arm order. The orchestration contains no fixture-specific branch. The current fixtures cover idempotent usage accounting and dependency readiness scheduling, both drawn from Dinah behavior.

Each fixture defines these arms:

1. Direct prompting with builder verification.
2. Direct prompting followed by an independent reviewer.
3. The lean candidate with builder, reviewer, and Dinah lifecycle.

The usage-ledger fixture orders the arms direct, direct plus review, then lean. The dependency fixture reverses the endpoints and runs lean, direct plus review, then direct. This counterbalancing reduces one simple order effect. It cannot remove provider variation or cache effects.

## Measured results

Every corrected arm passed the same external acceptance contract. Reasoning output is a subset of output in the provider event and is shown separately rather than added again.

| Fixture | Arm | Reported input | Cached subset | Derived uncached | Output | Reasoning subset | Worker seconds | Accepted |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Usage ledger | Direct | 149,689 | 124,672 | 25,017 | 3,296 | 467 | 101.4 | yes |
| Usage ledger | Direct plus review | 368,812 | 326,272 | 42,540 | 8,431 | 2,541 | 260.3 | yes |
| Usage ledger | Lean | 264,723 | 233,472 | 31,251 | 6,547 | 1,979 | 197.9 | yes |
| Dependency readiness | Direct | 153,164 | 117,376 | 35,788 | 2,507 | 432 | 81.6 | yes |
| Dependency readiness | Direct plus review | 280,975 | 230,144 | 50,831 | 3,995 | 430 | 144.0 | yes |
| Dependency readiness | Lean | 417,737 | 381,824 | 35,913 | 7,055 | 869 | 233.2 | yes |

The two primary-baseline arms pooled as follows:

| Arm | Reported input | Cached subset | Derived uncached | Output | Reasoning subset |
|---|---:|---:|---:|---:|---:|
| Direct plus review | 649,787 | 556,416 | 93,371 | 12,426 | 2,971 |
| Lean | 682,460 | 615,296 | 67,164 | 13,602 | 2,848 |

The usage-ledger fixture favored lean on every reported consumption category except cached share. The dependency fixture reversed the reported-input and output result. Counterbalanced order reduces one simple ordering effect, but it does not remove cache warmth, provider variation, or the lack of repeated runs. A median over two fixtures would only restate those two values, so the report does not treat it as an estimate.

The independent reviewers found no defect in either usage-ledger result or the dependency direct-plus-review result. The lean dependency reviewer found that one-pass dependency iterables were exhausted during cycle detection and repaired the implementation before acceptance. No fixture carried an intentionally seeded defect, so that one observation does not establish a quality advantage.

The first two campaign starts used the older Codex 0.122.0 found on `PATH`. They failed read-only and remain under `measured2`; one reported 120,499 input, 98,432 cached input, 1,375 output, and 135 reasoning-output tokens, while the interrupted attempt has unknown usage. The pinned 0.160.0 probe without the native sandbox override also failed and reported 38,830 input, 19,200 cached input, and 87 output tokens. The successful elevated-mode write probe reported 59,289 input, 45,952 cached input, 226 output, and 47 reasoning-output tokens. The original read-only readiness probe reported 19,339 input, 6,912 cached input, and 5 output tokens. These setup and failed-attempt costs are separate from the matched task table.

The external evaluator initially had two plumbing errors. One import path did not register the candidate module before executing it, and one assertion assumed a storage-inspection property the contract did not require. Both evaluator failures are retained. The corrected evaluator was then run against the same completed artifacts, with no worker rerun or feedback, and accepted them. The corrected usage evaluator hash is `2dd8b9d2a12a0f6e092c2959e222cb40c18ebb12863c59a26fef118b48d198da`; the dependency evaluator hash is `a5648e8bceae6f7bea85758bd0445fedb59481ec324e5eaa7c2075fc6fbf55a2`.

Raw evidence is under `C:\dinah-scratch\lean-dinah-pilot\measured3` for the retained direct arm and `C:\dinah-scratch\lean-dinah-pilot\measured4` for the remaining arms. Earlier failures remain under `measured` and `measured2`. Lifecycle smoke workbenches and readiness probes remain beside those directories for review.

## Reproduction

Run the prototype tests from the repository worktree:

```text
python -m unittest scripts.lean_dinah.test_prototype -v
```

Run the matched pilot only after a write-readiness probe succeeds with the same launcher flags:

```text
python scripts/lean_dinah/prototype.py preflight --root C:/dinah-scratch/lean-dinah-pilot/<preflight> --model gpt-5.6-sol --codex C:/Users/paul/.vscode/extensions/openai.chatgpt-26.930.31730-win32-x64/bin/windows-x86_64/codex.exe

python scripts/lean_dinah/prototype.py pilot --root C:/dinah-scratch/lean-dinah-pilot/<run> --model gpt-5.6-sol --codex C:/Users/paul/.vscode/extensions/openai.chatgpt-26.930.31730-win32-x64/bin/windows-x86_64/codex.exe --dinah C:/Users/paul/bin/dinah.exe --fixture usage_ledger --fixture dependency_ready
```

The preflight has a three-minute timeout and refuses unless the worker creates `ready.txt` with the required bytes. Use a new run directory. The prototype refuses an existing fixture workspace so a rerun cannot overwrite evidence.

## Limits

This pilot is an internal experiment. It changes no shared storage, live route, public command, scheduling behavior, merge automation, or pull request automation. Two bounded fixtures could expose a large regression, but a clean result would still be too small to establish quality equivalence across large Dinah changes. Any production change needs completed matched arms, independent evaluation, and a separate operator decision.
