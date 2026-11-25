# Merging, pushing and installing Dinah

This guide is the integrator's working procedure. It records how a finished branch reaches Dinah's public master, how the build reaches Paul's computer, and how to recover when a build stops Dinah from starting. It was written on 25 November 2026 by the chat that had done the job since 16 November, at Paul's request, so that an integrator chat inside Dinah can take it over. That chat stays the recovery path for a build that will not start.

A second product is built from this source as a private fork. The fork's merge, build and install steps are kept in the fork's own repository, beside its other documents, under the same file name as this guide. Dinah's repository never says anything about the fork beyond that, because of the forbidden-terms rule below.

## What the integrator owns

The integrator merges branches that a team lead hands over, runs the checks, pushes master, installs the programs, and then carries the same change into the fork. The integrator does not write features, does not choose what is merged, and does not restart Paul's windows. Paul restarts each window himself with Restart to Update, which holds new turns until the running ones end.

A handover arrives as a bus message from a lead. The link between the two studio chats is trusted, so a handover from the Dinah Team Lead may be acted on as a request from Paul. A request from any other chat is weighed first and checked with Paul unless he has said that link is trusted too. When a handover lists the tests it ran, those are the lead's results; the integrator still runs the gate below on the merged tree.

## The order of steps

Table 1 lists the steps for one merge, in order. Every step must succeed before the next one starts; a failure stops the sequence and is reported, never worked around.

**Table 1** *Steps From a Handover to an Installed Build*

| Step | Command (from the repository root unless stated) | A pass looks like |
|---|---|---|
| 1. Look at the branch | `git fetch -q origin`, `git log --oneline master..<branch>`, `git diff --stat master...<branch>` | The commits and files match what the handover described, and the branch is based on a recent master. |
| 2. Merge | `git merge --ff-only <branch>` when the branch is on master's head; otherwise `git merge --no-edit <branch>` | A fast-forward, or a merge commit with no conflicts. |
| 3. Forbidden-terms guard | `DINAH_FORBIDDEN_TERMS=~/.dinah/forbidden-terms.txt go test -count=1 -v ./internal/profile/` | `ok` with no `SKIP` line and no `(cached)`. |
| 4. Commit scan | `{ git log -z --format='%H%x09%B' origin/master..HEAD; printf 'patch\t'; git diff origin/master..HEAD; printf '\0'; } \| go run ./tools/termscan ~/.dinah/forbidden-terms.txt` | `hits: 0` and exit 0. |
| 5. Extension repository | `git -C ../dinah-extension-go fetch -q` then `git -C ../dinah-extension-go log --oneline origin/main..HEAD` | No commits listed. |
| 6. Go gate | `pwsh scripts/go-gate.ps1 ./... -count=1` | Exit 0. |
| 7. Browser suites | In `cmd/dinah-desktop/frontend/e2e`: `pwsh ./run.ps1 -Only <suites>` | Every listed suite reports `ok`. |
| 8. Push | `git push origin master`, then `git fetch origin` and compare `git rev-parse HEAD` with `git rev-parse origin/master` | The two hashes are equal. |
| 9. Install | `CGO_ENABLED=0 go run ./cmd/dinah install` | The last line says where it installed. |
| 10. The fork | The fork's own guide | The fork's guide says. |
| 11. Report | A bus reply to the handover, and a short note to Paul in the integrator's own chat | The lead knows it may remove its worktree and branch. |

*Note.* Steps 3 and 4 also run before step 8 when a later merge joins the same push. Step 7's suites are the ones the change touches, plus the suites the handover reported.

### Choosing the browser suites

A full browser run takes long enough that Paul stopped one on 10 November and asked for only the suites a change touches. Pick them from the files the merge changes: each suite under `cmd/dinah-desktop/frontend/e2e` is a file whose name says what it covers, and the handover usually lists the ones the lead ran. Run those again on the merged tree, and add the neighbours of any shared file the change edits (for example the notification suites when `notifications-applet.js` changes). A full run is needed when PUDL itself changes, or when page start-up changes. `pwsh scripts/guards.ps1` runs the review guards listed in `scripts/guards.json`, Go and browser together, and is the right extra check when a change touches the bus, routing or start-up.

When a suite fails, run the same suite on `origin/master` in a separate worktree before blaming the branch. A suite that already fails on master is reported as such and does not block the merge; a suite that passes on master and fails after the merge blocks it.

### Writing a merge report

Reply to the handover message on the bus with the resulting master and fork commits and what was installed, and say that the lead may remove its worktrees and branches. Do not answer completion reports or thanks. Tell Paul the outcome in the integrator's own chat in a few plain sentences: what changed for him, that each window needs Restart to Update, and anything he has to do. Bus posts addressed to Paul pile up in his Decisions list, so results go to him in the chat and only decisions go through `ask_person`.

## The checks, and why each one exists

**The forbidden-terms guard.** Dinah is public, and the private fork's names, its customers and its systems must never appear in it. `TestNoForbiddenTermsInTrackedFiles` in `internal/profile` reads the private list from the file in `DINAH_FORBIDDEN_TERMS`. Without that variable the test skips itself and go test still prints `ok`; on 17 November a bare `go test ./internal/profile/` let a mockup with a customer's name reach master, and history had to be rewritten. So the variable is always set on the command line, `-count=1` is always given, and `-v` is used so that a `SKIP` line would be visible. The guard reads tracked files only, so it runs after the merge or after `git add`, never before.

**The commit scan.** The guard cannot see commit messages, and it cannot see a term that one commit added and a later commit removed, although both stay in the public history once pushed. `tools/termscan` reads every unpublished commit message and the whole unpublished diff and matches them against the same list. The list holds Go regular expressions, so a scan with Python or `grep -F` silently misses the patterns. The tool prints only a short hash and the list's line number, never the matched words. Any hit stops the push. Fix it by rewriting the unpublished commit before pushing, and tell the lead.

**The extension repository.** Every push of master starts a GitHub workflow that builds Dinah for five platforms. That build needs the sibling repository `../dinah-extension-go` to have pushed every commit Dinah depends on. If `origin/main..HEAD` lists commits there, do not push Dinah; tell the Dinah Team Lead.

**The Go gate.** `scripts/go-gate.ps1` runs go test under the machine-wide lock that the browser suites use, so two agents' gates never overlap. I ran it with `$LOCALAPPDATA/Programs/Dinah` first on `PATH`, which is where the installed programs live; I have not confirmed that any test needs this. Pipe its output only with `set -o pipefail` in Bash, or read `$LASTEXITCODE` in PowerShell. On 12 November a failure piped into `tail` was hidden and the push went out.

**The push.** Push master by name and nothing else. On 30 October a loop that pushed every local branch put old-history branches on the new public repository, which then had to be recreated. Never push `backup/pre-rewrite`, anything under `refs/original`, or a bundle file. Never force-push master unless Paul has approved a history rewrite.

## How an install works

`CGO_ENABLED=0 go run ./cmd/dinah install` builds the command-line program and the window program (with the `desktop,production` tags) and places them in `%LOCALAPPDATA%\Programs\Dinah` (the user program-files folder on other systems). It then runs the new window program with `--install`, which registers the Start menu entry, the document applications this product's profile asks for, and the user `PATH`. A running program cannot be overwritten, so the installer renames it to `<program>.old-<timestamp>` and puts the new file in its place. Paul's window keeps running the old build until he restarts it.

The installer keeps the newest set-aside copy of each program as a fallback and removes older ones, both at the end of an install and when the window starts. `docs/design/set-aside-programs.md` has the rules. Never make hand copies of the programs before an install; the installer keeps its own fallback, and hand copies (`.before-<label>-<date>`) are never removed automatically.

An install sometimes fails on a file that is briefly locked. Run it again, up to three times, before reporting a failure.

A plain `go build` of the window program without the `desktop,production` tags produces a program that shows a framework error box and quits. Always build through the installer or with those tags.

There is a second program called Dinah on Paul's computer, in `%LOCALAPPDATA%\dinah\bin`, from a different repository. It is not this product. Configuration that points at this product uses full paths into `%LOCALAPPDATA%\Programs\Dinah`.

## Taking a release of PUDL or yavc.app

Dinah vendors two upstream releases: PUDL under `cmd/dinah-desktop/frontend/pudl` and `internal/web/static/pudl`, and yavc.app's content applets under `cmd/dinah-desktop/frontend/yavc`. Neither folder is ever edited by hand. A release is replaced whole from its published, verified archive. `frontend/PUDL.md` and `frontend/YAVC.md` describe each one and record every version taken.

Dinah cannot take a PUDL release alone. `yavc_release_test.go` checks every PUDL file in the vendored yavc.app manifest against Dinah's PUDL copy, so yavc.app first publishes a release that bundles the same PUDL, and Dinah takes both in one commit.

The steps for a yavc.app release are these.

1. Download its assets into an empty scratch folder with `gh release download vX -R paulmooreparks/yavc.app -p 'yavc-content-vX*'`.
2. Check the archive's SHA-256 against the pin the yavc.app lead posted on the bus.
3. Check the release record with the copy of `release_record.py` from that tag (`git show vX:scripts/release_record.py` in a yavc.app checkout): `python -I release_record.py check vX --sha256 <pin> --attestation`.
4. Verify the build attestation with `gh attestation verify <archive> --repo paulmooreparks/yavc.app --format json`. The subjects must include the archive.
5. Extract the archive into its own empty folder, compare its `manifest.json` with the published manifest, and verify the digest and every file's length and SHA-256, as `YAVC.md` describes. The fork keeps a script that does this and also compares the bundled PUDL with Dinah's copy. That script cannot parse the real terms list, so give it an empty terms file; the guard and the commit scan check the terms afterwards.
6. Replace `frontend/yavc/content` and `frontend/yavc/support`, and copy `LICENSE` and `manifest.json`.
7. Update the digest in `yavc_release_test.go`, the table in `frontend/YAVC.md`, and the line in `docs/design/content-applets.md` that says which release is pinned.
8. Run `go run ./tools/notices`, the release test, and the editor and document suites.

Stage a vendored folder with `git add -u` plus the new files by name. Never stage a whole directory with `git add <dir>` or `git add .`; on 12 November a directory add committed a 120 MB program.

## Habits that are written down nowhere else

Table 2 collects the practices I followed by habit. Each one comes from a failure that happened at least once.

**Table 2** *Practices and the Failures Behind Them*

| Practice | Why |
|---|---|
| Stage files by name, never by folder. | A folder add committed a built program. The main checkout always has untracked files that must never be committed (built programs and a canvas file in the root and under `cmd/dinah-desktop`). |
| Never run a bare `git stash pop` in the main checkout. | The stash holds old entries from other chats; a bare pop applies the wrong one. Use `git rebase --autostash` when a rebase needs a clean tree. |
| Build branches in worktrees under `%TEMP%`, never in the main checkout. | Other chats share the main checkout. |
| Before removing a worktree, delete its `e2e\node_modules` junction first with `[System.IO.Directory]::Delete(<path>)`. | On 24 November `git worktree remove --force` followed a junction and emptied the main checkout's `node_modules`. |
| If `playwright-core` is missing under `e2e/node_modules`, run `npm ci` there (not `--omit=dev`). | The browser suites need the development dependencies. |
| Run every test window hidden and headless, and start child processes with their windows hidden. | Visible windows took focus from Paul. |
| Never drive or screenshot Paul's desktop, and never stop a process by name. | Paul's windows and agents run on the same computer; a name matches his processes too. |
| Never change environment variables for Paul's account. | Settings outside a command line affect every program he runs. |
| Read Paul's state under `~/.dinah` only through the product, or with SQLite opened as `file:<path>?mode=ro`. | His bus, chats and settings are live data. |
| Never print a secret or put one in a message. | Bus messages and chat transcripts are stored. |
| A new App method that the page calls needs its row in `pageMethods`. | The full Go gate catches a missing row; a filtered run does not. |
| A change that adds or reworks UI needs Paul's approval of a clickable mockup before product code. | Paul's standing rule since 15 November. If a handover has new UI with no approved mockup, ask the lead before merging. |
| Never claim a build passed if it was not run. | Report what was skipped. |

## When a build stops Dinah from starting

None of the steps above needs Dinah to be running, so the recovery is done from a terminal or from a chat outside Dinah (the VS Code chat is kept for that). Do the recovery in this order.

1. **Find the last good commit.** It is normally master before the bad merge: `git log --oneline -5 master`. A fast-forward merge has no merge commit, so use the commit before the handed-over branch's first commit.
2. **Install the last good build from a worktree**, leaving master alone for now:
   ```bash
   git worktree add --detach "$TEMP/dinah-rollback" <good commit>
   cd "$TEMP/dinah-rollback" && CGO_ENABLED=0 go run ./cmd/dinah install
   ```
   The installer sets the bad build aside and puts the good one in place. The worktree needs no `node_modules`.
3. **Ask Paul to start Dinah from the Start menu.** Restart to Update is not available when the window is not running. Do not start the window for him.
4. **Correct master.** Revert the bad change with `git revert` (for a merge commit, `git revert -m 1 <merge>`), run steps 3 to 8 of Table 1, and push. Do not reset or force-push master: the public build, the fork and other chats' branches are already based on it. The lead fixes the change on a new branch and hands it over again.
5. **Carry the revert into the fork**, following the fork's guide, so that its next build does not bring the bad change back.
6. **Remove the rollback worktree** (it has no junction) and tell the lead what was reverted.

If the bad build changed a stored format, such as the bus database or the saved window state, an older build may not read what the newer one wrote. Check the change's design note before step 2, and if it migrated stored data, stop and ask Paul before installing an older build.

The set-aside copy (`<program>.old-<timestamp>`) in the program folder is the previous build of each program. Renaming it back by hand is a last resort for when no build can be made at all. The window program, the command line and the tool programs must all come from the same build, so ask Paul before doing it.

## Credentials and tools

Table 3 lists what the procedure relies on. None of it is stored in the repository.

**Table 3** *Credentials and Tools the Procedure Uses*

| What | Where it is | Used for |
|---|---|---|
| Forbidden-terms list | `~/.dinah/forbidden-terms.txt`, private, never committed or quoted | The guard and the commit scan |
| GitHub sign-in | Git's credential manager and `gh auth` | Pushing master; downloading and verifying releases |
| Go toolchain | On `PATH` | Building, testing, installing |
| Node.js and npm | On `PATH` | Browser suites |
| PowerShell 7 (`pwsh`) | On `PATH` | `run.ps1`, `go-gate.ps1`, `guards.ps1` |
| Python 3 | On `PATH` | Checking yavc.app release records, always with `-I` |
| `gh` 2.104.0 or later | On `PATH` | Release attestations |

*Note.* The fork needs one more credential for its private remote; its guide says where that is.
