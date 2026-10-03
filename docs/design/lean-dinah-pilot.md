# Lean Dinah pilot

## Status and decision

The bounded pilot does not support adopting a lean replacement for Dinah development. In the final frozen campaign, all six fixture arms passed their independent acceptance checks, and both lean arms reached the real candidate workbench's operator-owned Acceptance column. Against the primary direct-plus-review baseline, the lean arms reported 664,097 input tokens versus 606,815. Their uncached portions were 59,937 versus 77,407, and their output totals were 9,690 versus 9,897. Cache composition and run order moved in opposite directions. These two fixtures do not establish a saving or a quality advantage.

The prototype is usable for another isolated run. The two Python fixtures are too small to establish quality equivalence for full Dinah feature work or reduced human coordination burden. They provide no reliable subscription-savings evidence. Keep the ledger and guarded-control prototype available for experiments, but do not adopt the reduced route broadly. A production rollout or shared-core change needs a separate decision after a wider matched study.

## Candidate route

The committed workbench template has Ready, Working, Review, Acceptance, and Done columns. Acceptance belongs to the operator. The adapter creates a card in Ready, moves it to Working with the last observed revision as `basis`, and assigns the builder. It moves a completed builder result to Review, releases the builder, and assigns a separate reviewer. A successful external evaluation lets the reviewer release the card and request the move to Acceptance. The adapter stops there.

The installed `dinah.exe` created an isolated workbench from the template during the measured pilot. Both lean cards reached Ready, Working, Review, and Acceptance through Dinah's MCP surface. A lifecycle smoke also demonstrated that a claimed card cannot enter an awaiting-outside column. Releasing the reviewer before the guarded move allowed each card to reach Acceptance. An automated actor cannot leave Acceptance for Done.

Dinah owns each claim and transition. The adapter stops when the MCP response is a refusal. Mutating MCP calls carry the last card revision as `basis`. Instruction hashes, workbench revisions, card revisions, starting code state, and change cursors are polled before dispatch and before accepting a result. Those separate reads do not make instruction freshness atomic, so the prototype makes no such claim.

## Packets and recovery

A builder packet contains generated run and attempt identifiers, the task contract, permitted scope, acceptance description, applicable decisions, instruction hash, starting code revision, workbench revision, card revision, changes cursor, and evidence requirements. Additional context belongs in content-addressed references with a revision and byte count.

The current launcher uses `--ephemeral`, so it does not retain a session for continuation. A later builder round must receive an explicit recovery packet containing the prior result, review findings, current revisions, and the changed-file manifest. The pilot never claims that this reproduces hidden model state.

The reviewer packet is assembled separately. The reviewer may inspect the workspace beyond the builder report but may write only `REVIEW.md`. Its first line records `VERDICT: PASS` or `VERDICT: REPAIR`. A repair verdict moves the card from Review back to Working and returns it to the builder identity with an explicit recovery packet containing the prior result, findings, current revisions, and changed-file manifest. Repaired work moves from Working to Review before a separately claimed reviewer checks it. The campaign allows one such recovery round. A failed reviewer process, a verdict bound to an older workspace, an exhausted repair, or an unknown verdict is incomplete rather than accepted. The external acceptance evaluator stays outside both worker prompts and is identical across the three arms of a fixture.

## Launcher

The prototype checks the installed command's version and help before launching a worker. The measured run pinned `C:\Users\paul\.vscode\extensions\openai.chatgpt-26.930.31730-win32-x64\bin\windows-x86_64\codex.exe`, which reported version 0.160.0. Each measured process received the prompt on standard input and ran this argument shape:

```text
codex exec --ignore-user-config -c windows.sandbox="elevated" --sandbox workspace-write -m gpt-5.6-sol --json --ephemeral --skip-git-repo-check -C <fixture-workspace> -
```

The command array is passed directly to `subprocess.run`. Shell interpolation does not carry the prompt. Future runs use an explicit allowlist of Windows process and path variables. The invocation identity and receipt record the allowed names and their non-secret values; credentials and unrelated ambient variables do not reach the subprocess. `--ignore-user-config` prevents the parent session's model choice from changing the measured model.

The measured8 campaign ran on launcher commit `396c4b1b`. Its ten processes inherited one unchanged parent environment, and those values were not recorded. All arms remain matched on that observed setup, but the campaign does not prove clean-allowlist portability or exact environment reproduction. The allowlist repair landed after the campaign and has deterministic coverage only. No measured8 receipt has been reconstructed or relabeled as contemporaneous evidence from the repaired launcher.

The [Windows sandbox documentation](https://learn.chatgpt.com/docs/windows/windows-sandbox) requires a native sandbox mode. `--ignore-user-config` omitted the existing `[windows] sandbox = "elevated"` setting, so the first write attempts ran read-only even though the argument array requested `workspace-write`. The explicit override fixed that mismatch without weakening confinement. A one-file probe created `ready.txt` before the corrected campaign started. The launcher makes no hard read-isolation claim.

## Usage ledger

Every process has an adapter run identifier. The ledger accepts usage only when a successful process has one terminal `turn.completed` event with integer `input_tokens`, `cached_input_tokens`, and `output_tokens`. It preserves `reasoning_output_tokens` and `cache_write_input_tokens` when reported. Cached input remains a subset category and is never added to input.

An unsuccessful process, truncated JSON Lines stream, missing terminal event, or conflicting terminal event remains incomplete. Unknown usage stays unknown rather than becoming zero. Re-importing the same run and identity is idempotent. Reusing a run identifier with a different command, prompt, transcript, state, or usage is refused.

The report separates setup and development activity from measured task execution. The earlier readiness probe reported 19,339 input tokens, 6,912 cached input tokens, and 5 output tokens. That fixed harness overhead is setup evidence, not Dinah coordination cost. Development-session usage was not measured and remains unknown.

## Evidence receipts

A receipt binds the repository revision, dirty-state manifest hash, task hash, acceptance-input hash, command array, relevant configuration hash, exit status, and output hash. A changed output or changed merge candidate invalidates the receipt. An empty automated-check response remains unknown.

The tests exercise matching and stale receipts, changed output, changed acceptance input, changed merge candidates, empty check results, changed instructions, late task changes, stale card revisions, operator-only completion, duplicate usage delivery, conflicting duplicate delivery, missing usage, subprocess environment filtering, and accepted counterparts. A real isolated Dinah integration changes a column instruction and posts a late card comment between packet creation and the next gate; each stale packet is refused, while a refreshed packet proceeds. The same integration moves Review to Working for repair, supplies a changed-file manifest, moves the repair back to Review, and rejects a stale PASS after a failed process or changed workspace.

## Fixtures and comparison method

Fixture manifests name the task file, workspace seed, external acceptance command, and arm order. The orchestration contains no fixture-specific branch. The current fixtures cover idempotent usage accounting and dependency readiness scheduling, both drawn from Dinah behavior.

Each fixture defines these arms:

1. Direct prompting with builder verification.
2. Direct prompting followed by an independent reviewer.
3. The lean candidate with builder, reviewer, and Dinah lifecycle.

The usage-ledger fixture orders the arms direct, direct plus review, then lean. The dependency fixture reverses the endpoints and runs lean, direct plus review, then direct. This counterbalancing reduces one simple order effect. It cannot remove provider variation or cache effects.

## Measured results

Every final frozen arm passed the same external acceptance contract. Reasoning output is a subset of output in the provider event and is shown separately rather than added again.

| Fixture | Arm | Reported input | Cached subset | Derived uncached | Output | Reasoning subset | Worker seconds | Accepted |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Usage ledger | Direct | 128,614 | 94,976 | 33,638 | 2,585 | 618 | 83.8 | yes |
| Usage ledger | Direct plus review | 293,593 | 267,136 | 26,457 | 4,823 | 906 | 160.3 | yes |
| Usage ledger | Lean | 298,873 | 265,984 | 32,889 | 4,604 | 881 | 164.7 | yes |
| Dependency readiness | Direct | 174,842 | 140,800 | 34,042 | 3,244 | 699 | 111.8 | yes |
| Dependency readiness | Direct plus review | 313,222 | 262,272 | 50,950 | 5,074 | 1,015 | 155.7 | yes |
| Dependency readiness | Lean | 365,224 | 338,176 | 27,048 | 5,086 | 865 | 127.4 | yes |

The two primary-baseline arms pooled as follows:

| Arm | Reported input | Cached subset | Derived uncached | Output | Reasoning subset |
|---|---:|---:|---:|---:|---:|
| Direct | 303,456 | 235,776 | 67,680 | 5,829 | 1,317 |
| Direct plus review | 606,815 | 529,408 | 77,407 | 9,897 | 1,921 |
| Lean | 664,097 | 604,160 | 59,937 | 9,690 | 1,746 |

Lean input plus output was 9.25 percent above the reviewed baseline and 117.85 percent above direct prompting. Its uncached input was 22.57 percent below the reviewed baseline and 11.44 percent below direct prompting. The usage-ledger fixture put lean slightly above the reviewed baseline for input and slightly below it for output. The dependency fixture put lean above the reviewed baseline for both. Counterbalanced order reduces one simple ordering effect, but it does not remove cache warmth, provider variation, or the lack of repeated runs. The report does not convert these values to money or subscription capacity.

The independent reviewers returned a fresh PASS after successful processes in all four final review arms, so no recovery round ran during the frozen campaign. The three later repairs do not alter its happy-path results. The deterministic integration separately exercises the corrected recovery route and verdict refusal. No fixture carried an intentionally seeded defect, so the result does not establish a quality advantage.

The first two campaign starts used the older Codex 0.122.0 found on `PATH`. They failed read-only and remain under `measured2`; one reported 120,499 input, 98,432 cached input, 1,375 output, and 135 reasoning-output tokens, while the interrupted attempt has unknown usage. The pinned 0.160.0 probe without the native sandbox override also failed and reported 38,830 input, 19,200 cached input, and 87 output tokens. The successful elevated-mode write probe reported 59,289 input, 45,952 cached input, 226 output, and 47 reasoning-output tokens. The original read-only readiness probe reported 19,339 input, 6,912 cached input, and 5 output tokens. These setup and failed-attempt costs are separate from the matched task table.

The earlier campaigns are exploratory and invalid as matched comparisons. They used different role instructions across arms, placeholder authority revisions, and incomplete receipt wiring. Three evaluator-contract mismatches were also found during development: module registration, an unsupported storage-inspection assumption, and totals field names absent from the task contract. The post-hoc corrected acceptance of the original direct artifact is retained under `corrected-acceptance` with its source transcript and artifact hashes; its receipt explicitly says `post-hoc-reconstruction` and does not rescue the earlier comparison.

Before the frozen run, both fixture evaluators were audited assertion by assertion against explicit contracts. Each evaluator accepts a small conforming reference candidate and rejects a meaningful broken candidate. The dependency contract and evaluator cover a one-shot iterable. The usage contract names the class, methods, row exposure, token fields, totals shape, duplicate behavior, and unknown handling.

The durable frozen evidence is under `C:\dinah-scratch\lean-dinah-pilot\measured8`. Every worker, reviewer, evaluator, authority gate, and transition is represented there by a raw stream or hash-linked receipt. Earlier directories `measured`, `measured2`, `measured3`, and `measured4` retain the exploratory setup and first campaign. `measured5` retains one completed invalid direct run and an interrupted direct-plus-review launch. `measured6` and `measured7` retain interrupted launches made while the contract and recovery defects were being corrected. Their incomplete process usage is unknown where no terminal event exists. Every failed integration directory remains preserved. Development-session usage was not recorded and remains unknown.

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
