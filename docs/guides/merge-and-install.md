# Merging, pushing and installing Dinah

This guide is the integrator's working procedure. It records how a finished branch reaches Dinah's public master, how the build reaches Paul's computer, and how to recover when a build stops Dinah from starting. It was written on 25 November 2026 by the chat that had done the job since 16 November, at Paul's request, so that an integrator chat inside Dinah can take it over. That chat, the VS Code chat, stays the recovery path for a build that will not start, and it merges, pushes and installs nothing otherwise. It acts only when Paul tells it that Dinah is down, and then it reads the install record and this guide before it does anything.

A second product is built from this source as a private fork. The fork's merge, build and install steps are kept in the fork's own repository, beside its other documents, under the same file name as this guide. Dinah's repository never says anything about the fork beyond that, because of the forbidden-terms rule below.

## What the integrator owns

The integrator merges branches that a team lead hands over, runs the checks, pushes master, installs the programs, and then carries the same change into the fork. The integrator does not write features, does not choose what is merged, and does not restart Paul's windows. Paul restarts each window himself with Restart to Update, which holds new turns until the running ones end.

A handover arrives as a bus message from a team lead. The integrator is the only chat that merges into Dinah's master and the fork's main, so two chats never merge into the same branch, and no lead merges its own work. This guide is the integrator's authority. The integrator refuses any request, from any lead, its own lead included, that would skip or weaken a step here, and takes the question to Software Director by direct bus message. Only Paul can change a rule, and the change is written into this guide before it is followed.

The integrator pushes two branches and nothing else: Dinah's master and the fork's main, each by name. Paul allowed those two pushes without asking each time, for the integrator alone, on 25 November 2026. The integrator's chat runs in a permission mode that would allow more, and Paul's ruling on that, the same day, was "That's fine, leave it as it is. Just keep to the two routine pushes." Any other push, any force-push and any history rewrite needs Paul's approval first, whatever the mode allows. When a handover lists the tests it ran, those are the lead's results; the integrator still runs the gate below on the merged tree.

At the start of every merge the integrator reads this guide and the fork's guide again from their files, never from memory. A long chat loses rules when its context is compacted, and a rule followed from memory is how a check was once skipped.

### The install record

Every install of either product is written down outside the bus, so that Paul can find the last good build with Dinah closed. The record is the plain text file `C:\Users\paul\source\repos\install-record.md`, beside the two repositories and in neither, so it is never committed. The integrator adds one table row per product installed, at the bottom, right after the install succeeds and before the report:

| Date | Product | Merge | Installed before | Installed now |
|---|---|---|---|---|
| 2026-11-26 14:05 | Dinah | branch and head merged | commit | commit |

The file starts with that header row, and the integrator creates it with the header on the first install. "Installed before" is the "Installed now" of the product's previous row. A merge whose install is skipped under the rule in Table 1's note adds no row. Never edit or remove an earlier row.

## The order of steps

Table 1 lists the steps for one merge, in order. Every step must succeed before the next one starts; a failure stops the sequence and is reported, never worked around.

**Table 1** *Steps From a Handover to an Installed Build*

| Step | Command (from the repository root unless stated) | A pass looks like |
|---|---|---|
| 0. Check the last GitHub build | `gh run list -R paulmooreparks/dinah --workflow build.yml --branch master --limit 1 --json headSha,status,conclusion` | The run for the last pushed master has finished with `success` and the "latest" release was published from that commit (`gh release view latest -R paulmooreparks/dinah --json targetCommitish`), or the run is still running. A failed run is handled by the rule under "A failure on GitHub" below before the next merge starts. The merge itself may go ahead unless the failure is in the area the handover changes. |
| 1. Look at the branch | `git fetch -q origin`, `git log --oneline master..<branch>`, `git diff --stat master...<branch>` | The commits and files match what the handover described, and the branch is based on a recent master. |
| 2. Merge | `git merge --ff-only <branch>` when the branch is on master's head; otherwise `git merge --no-edit <branch>` | A fast-forward, or a merge commit with no conflicts. |
| 3. Forbidden-terms guard | `DINAH_FORBIDDEN_TERMS=~/.dinah/forbidden-terms.txt go test -count=1 -v ./internal/profile/` | `ok` with no `SKIP` line and no `(cached)`. |
| 4. Commit scan | `{ git log -z --format='%H%x09%B' origin/master..HEAD; printf 'patch\t'; git diff origin/master..HEAD; printf '\0'; } \| go run ./tools/termscan ~/.dinah/forbidden-terms.txt` | `hits: 0` and exit 0. |
| 5. Extension repository | `git -C ../dinah-extension-go fetch -q` then `git -C ../dinah-extension-go log --oneline origin/main..HEAD` | No commits listed. |
| 6. Go gate | `pwsh scripts/go-gate.ps1 ./... -count=1` | Exit 0. |
| 7. Browser suites | In `cmd/dinah-desktop/frontend/e2e`: `pwsh ./run.ps1 -Only <suites>` | Every listed suite reports `ok`. |
| 8. Push | `git push origin master`, then `git fetch origin` and compare `git rev-parse HEAD` with `git rev-parse origin/master` | The two hashes are equal. |
| 9. Install | `CGO_ENABLED=0 go run ./cmd/dinah install`, then add a row to the install record | The last line says where it installed, and the record's new row gives the commit installed before and now. |
| 10. The fork | The fork's own guide | The fork's guide says. |
| 11. Report | The full report sent directly to Software Director, a reply to the handover, a post in #Dinah Desktop Team, and a short note to Paul in the integrator's own chat | The report gives every guard result, and the lead knows it may remove its worktree and branch. |

*Note.* Steps 3 and 4 also run before step 8 when a later merge joins the same push. Step 7's suites are the ones the change touches, plus the suites the handover reported. Step 9 runs after every merge. The one exception is a merge in which every changed file is under `docs/`, since no build reads that folder; the report then says the install was skipped and why. When in doubt, install. A merge in which every changed file is under `docs/` also skips steps 6 and 7, the Go gate and the browser suites, while steps 3 and 4, the guard and the commit scan, still run (Paul, 26 November 2026, Table 7 of docs/design/agent-organisation-economy.md).

When any step leaves a tracked change that this guide does not cover (a file regenerated, reformatted or rewritten by a tool), the integrator stops before committing anything and reports it to the lead and to Software Director. Check `git status` after every step that runs a tool. An install is recorded only once the tree it was built from is committed, so the commit in the install record is always the commit that was built.

### Choosing the browser suites

A full browser run takes long enough that Paul stopped one on 10 November and asked for only the suites a change touches. Pick them from the files the merge changes: each suite under `cmd/dinah-desktop/frontend/e2e` is a file whose name says what it covers, and the handover usually lists the ones the lead ran. Run those again on the merged tree, and add the neighbours of any shared file the change edits (for example the notification suites when `notifications-applet.js` changes). A full run is needed when PUDL itself changes, or when page start-up changes. `pwsh scripts/guards.ps1` runs the review guards listed in `scripts/guards.json`, Go and browser together, and is the right extra check when a change touches the bus, routing or start-up.

A change that adds or changes a menu row, a registered command or a keyboard shortcut runs the `keyboard` and `host-menus-order` suites before it is handed over, and the handover lists them among the suites run. These suites check that no menu letter or key is refused or taken twice, and a new row can break a row elsewhere in the same menu. On 28 November a new View row took Color Mode's letter; the lead's focused run had left out the keyboard suite, and the integrator's full run caught it before the push. The integrator sends back a handover for such a change that does not list both suites. Paul approved this rule on 28 November 2026: "Yes, I approve the new rule for the Integrator's guide. Go ahead and merge it."

When a suite fails, run the same suite on `origin/master` in a separate worktree before blaming the branch. A suite that already fails on master is reported as such and does not block the merge; a suite that passes on master and fails after the merge blocks it.

### Writing a merge report

Send every full merge report directly to Software Director by bus message, so that the record reaches Software Director from the chat that did the work. Post the same report in #Dinah Desktop Team, the group of the integrator's own team. The integrator cannot post in #Software Team Leads, because the bus does not let a member of one lead's team join that group, so the Dinah Team Lead copies a summary of each report there for the other leads. The report gives the branch and head merged, the resulting master and fork commits, the result of each step of Table 1 and of the fork's guide (with the guard's and the scan's exact results, and any suite that was skipped or already failing on master), and, for each product, the commit that was installed before this merge and the one installed now, as written to the install record. When the install was skipped under the rule in Table 1's note, the report says so, says why, and repeats the commit still installed.

Then reply to the handover message, which reaches the lead who handed the work over, with the same commits, and say that the lead may remove its worktrees and branches. Do not answer completion reports or thanks. Tell Paul the outcome in the integrator's own chat in a few plain sentences: what changed for him, that each window needs Restart to Update, and anything he has to do. Bus posts addressed to Paul pile up in his Decisions list, so results go to him in the chat and only decisions go through `ask_person`.

## The checks, and why each one exists

**The forbidden-terms guard.** Dinah is public, and the private fork's names, its customers and its systems must never appear in it. `TestNoForbiddenTermsInTrackedFiles` in `internal/profile` reads the private list from the file in `DINAH_FORBIDDEN_TERMS`. Without that variable the test skips itself and go test still prints `ok`; on 17 November a bare `go test ./internal/profile/` let a mockup with a customer's name reach master, and history had to be rewritten. So the variable is always set on the command line, `-count=1` is always given, and `-v` is used so that a `SKIP` line would be visible. The guard reads tracked files only, so it runs after the merge or after `git add`, never before.

**The commit scan.** The guard cannot see commit messages, and it cannot see a term that one commit added and a later commit removed, although both stay in the public history once pushed. `tools/termscan` reads every unpublished commit message and the whole unpublished diff and matches them against the same list. The list holds Go regular expressions, so a scan with Python or `grep -F` silently misses the patterns. The tool prints only a short hash and the list's line number, never the matched words. Any hit stops the push. Fix it by rewriting the unpublished commit before pushing, and tell the lead.

**The extension repository.** Every push of master starts a GitHub workflow that builds Dinah for five platforms. That build needs the sibling repository `../dinah-extension-go` to have pushed every commit Dinah depends on. If `origin/main..HEAD` lists commits there, do not push Dinah; tell the Dinah Team Lead.

**The Go gate.** `scripts/go-gate.ps1` runs go test under the machine-wide lock that the browser suites use, so two agents' gates never overlap. Pipe its output only with `set -o pipefail` in Bash, or read `$LASTEXITCODE` in PowerShell. On 12 November a failure piped into `tail` was hidden and the push went out.

**A failure outside the merge.** A test sometimes times out when another chat's work is loading the machine; it happened three times on 26 November, each in a package the merge did not touch. When a Go gate or browser suite failure is in a package or suite the merge does not change, the integrator may rerun once, and only on these terms:
1. Report the first failure in full in the merge report: the test, its package and its output.
2. Run the failing test alone (`go test -count=1 -run '^<Test>$' ./<package>`, or the one suite with `run.ps1 -Only`). Rerun the whole gate only if it passes alone.
3. Stop and ask the lead and Software Director, without merging, if the same test fails a second time, if the test also fails alone, or if any failure is in a package or suite the merge changes.
4. Name the test to Dinah Quality, which owns tests that fail at random, so it is fixed and does not become a known failure everyone passes over.

A merge changes a package when it changes any file in that package's folder, or a file the package embeds.

**A fix Paul is waiting for.** Paul set this rule on 26 November 2026: "If it gets hung on one or two tests, I want to know what they are so that I can make a judgment call." When one or two failing checks (a local gate, a GitHub run, a release check or a review) hold a fix he is waiting for, and the failure will not be fixed within about 15 minutes, the integrator tells the Dinah Team Lead at once, before a second attempt at the same run, and the lead tells Software Director. One short message covers the points in Table 2.

**Table 2** *Contents of a Report on a Held Fix*

| Point | What to say |
| --- | --- |
| The fix | What Paul gets when it lands. |
| Each failing check | What it tests, in plain words. |
| Where | Where it fails and where it passes (master, the branch, alone, on GitHub). |
| Relevance | Whether the failure bears on the change. |
| Risk | What could go wrong if the fix goes ahead. |
| Options | Go ahead with the failure tracked, wait (with an estimate), or split the change. |
| Recommendation | Which option, and why. |

*Note.* The rule changes none of the following. The one-retry rules above still apply. Only Paul can let a fix through when the failing test is one the change touches. Signing, attestation, the forbidden-terms guard and the commit scan are never skipped. A check left failing gets an owner and is fixed afterwards.

**A failure on GitHub.** Paul chose this rule on 26 November 2026: "Option b, retry once. Go with your recommendation." A failed test job in the GitHub build of master may be re-run once on the same commit, as follows.
1. Report the first failure in full to the lead who owns the area and to Software Director: the run, the job, the test and its output from `gh run view <id> --log-failed`. Name the test to Dinah Quality.
2. If every failing test is outside what the pushed change touched, re-run once with `gh run rerun <id> -R paulmooreparks/dinah --failed`. That re-runs the failed jobs and the jobs that depend on them, on the same commit, with no push. Push nothing to master while it runs, since a push cancels it.
3. A retry is never used to pass a failure in a test the change itself touched. Such a failure blocks, and goes back to the lead.
4. Publishing happens only if the re-run passes; the build and publish jobs then run as usual. If the same test fails a second time, that blocks: no further re-run, and the lead and Software Director are told.
5. After the run, confirm that "latest" was published from that commit, and say so in the merge report.
The workflow itself does not retry; this is a manual step, so every retry is visible in a report.

**The push.** Push master by name and nothing else. On 30 October a loop that pushed every local branch put old-history branches on the new public repository, which then had to be recreated. Never push `backup/pre-rewrite`, anything under `refs/original`, or a bundle file. Never force-push master unless Paul has approved a history rewrite.

## How an install works

`CGO_ENABLED=0 go run ./cmd/dinah install` builds the command-line program and the window program (with the `desktop,production` tags) and places them in `%LOCALAPPDATA%\Programs\Dinah` (the user program-files folder on other systems). It then runs the new window program with `--install`, which registers the Start menu entry, the document applications this product's profile asks for, and the user `PATH`. A running program cannot be overwritten, so the installer renames it to `<program>.old-<timestamp>` and puts the new file in its place. Paul's window keeps running the old build until he restarts it.

The installer keeps the newest set-aside copy of each program and removes older ones, both at the end of an install and when the window starts. `docs/design/set-aside-programs.md` has the rules. No document describes putting a set-aside copy back into use, so the recovery below never does that. Never make hand copies of the programs before an install; the installer keeps its own fallback, and hand copies (`.before-<label>-<date>`) are never removed automatically.

The integrator runs inside the Dinah window it installs over. That is the documented path: the window and the tool programs the integrator itself uses keep running the old build, and the window offers Restart to Update once the new one is in place. The integrator finishes its report before it asks Paul to restart, because Restart to Update waits for the running turn and then stops every chat, the integrator included. It never restarts the window itself.

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

Table 3 collects the practices I followed by habit. Each one comes from a failure that happened at least once.

**Table 3** *Practices and the Failures Behind Them*

| Practice | Why |
|---|---|
| Stage files by name, never by folder. | A folder add committed a built program. The main checkout always has untracked files that must never be committed (built programs and a canvas file in the root and under `cmd/dinah-desktop`). |
| Never run a bare `git stash pop` in the main checkout. | The stash holds old entries from other chats; a bare pop applies the wrong one. Use `git rebase --autostash` when a rebase needs a clean tree. |
| Build branches in worktrees under `%TEMP%`, never in the main checkout. A worktree that runs `go run ./cmd/dinah install` or builds `./cmd/dinah` goes beside the main checkout instead, in `C:\Users\paul\source\repos`. | Other chats share the main checkout. `go.mod` replaces the extension module with the folder `../dinah-extension-go`, so a build under `%TEMP%` stops with "replacement directory ../dinah-extension-go does not exist". |
| Before removing a worktree, delete its `e2e\node_modules` junction first with `[System.IO.Directory]::Delete(<path>)`. | On 24 November `git worktree remove --force` followed a junction and emptied the main checkout's `node_modules`. |
| If `playwright-core` is missing under `e2e/node_modules`, run `npm ci` there (not `--omit=dev`). | The browser suites need the development dependencies. |
| Run every test window hidden and headless, and start child processes with their windows hidden. | Visible windows took focus from Paul. |
| Never drive or screenshot Paul's desktop, and never stop a process by name. | Paul's windows and agents run on the same computer; a name matches his processes too. |
| Never change environment variables for Paul's account. | Settings outside a command line affect every program he runs. |
| Read Paul's state under `~/.dinah` only through the product, or with SQLite opened as `file:<path>?mode=ro`. | His bus, chats and settings are live data. |
| Never print a secret or put one in a message. | Bus messages and chat transcripts are stored. |
| A new App method that the page calls needs its row in `pageMethods`. | The full Go gate catches a missing row; a filtered run does not. |
| A change that adds or reworks UI needs Paul's approval of a clickable mockup before product code. | Paul's standing rule since 15 November. If a handover has new UI with no approved mockup, ask the lead before merging. A stock PUDL component adopted unchanged needs no separate mockup (Paul, 26 November 2026, Table 8 of docs/design/agent-organisation-economy.md). |
| Never claim a build passed if it was not run. | Report what was skipped. |

## When a build stops Dinah from starting

This section is written so that Paul can follow it himself with Dinah closed, or have the VS Code chat follow it, since that chat runs outside Dinah and is kept for this. None of the steps needs Dinah to be running. The way back is to install the last good commit again with Dinah's own install command, which is the documented installer. The install in step 2 was run on 25 November 2026 against scratch program and home folders: installing an older commit over a newer one made the older build the active program, and the newer one was set aside.

1. **Find the last good commit.** Open `C:\Users\paul\source\repos\install-record.md` in File Explorer or any text editor. In the last row for Dinah, the "Installed before" column is the last good commit. Master before the bad merge is not a safe substitute, because not every merge is installed. If the record is missing, stop and ask Paul which build last worked.
2. **Install the last good build from a worktree beside the repository**, leaving master alone for now. In Git Bash:
   ```bash
   cd /c/Users/paul/source/repos/studio
   git worktree add --detach ../dinah-rollback <good commit>
   cd ../dinah-rollback && CGO_ENABLED=0 go run ./cmd/dinah install
   ```
   The worktree must sit beside `studio`, not under `%TEMP%`, because the build needs the folder `../dinah-extension-go`. It needs no `node_modules`. The last line of the install says where it installed. The installer sets the bad build aside and puts the good one in place, and a window that is still running keeps the bad build until it is closed. Add a row to the install record for this install too, with "recovery" in the Merge column.
3. **Start Dinah from the Start menu.** If the bad window is still open, close it first. Restart to Update is not available when the window is not running. An agent never starts the window for Paul.
4. **Correct master.** Revert the bad change with `git revert` (for a merge commit, `git revert -m 1 <merge>`), run steps 3 to 8 of Table 1, and push. Do not reset or force-push master: the public build, the fork and other chats' branches are already based on it. The lead fixes the change on a new branch and hands it over again.
5. **Carry the revert into the fork**, following the fork's guide, so that its next build does not bring the bad change back.
6. **Remove the rollback worktree** with `git worktree remove ../dinah-rollback` from `studio` (it has no junction), and tell the lead what was reverted.

Steps 4 to 6 are the integrator's, or the VS Code chat's while Dinah cannot start. Paul needs only steps 1 to 3 to have a working Dinah again.

If the bad build changed a stored format, such as the bus database or the saved window state, an older build may not read what the newer one wrote. Check the change's design note before step 2, and if it migrated stored data, stop and ask Paul before installing an older build.

The set-aside copy (`<program>.old-<timestamp>`) in the program folder is the previous build of each program, but no document describes putting it back into use, and the window program, the command line and the tool programs must all come from one build. Do not rename it back. If no build can be made at all, stop and ask Paul.

## Credentials and tools

Table 4 lists what the procedure relies on. None of it is stored in the repository.

**Table 4** *Credentials and Tools the Procedure Uses*

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
