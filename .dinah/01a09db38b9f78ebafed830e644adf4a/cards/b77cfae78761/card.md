---
title: nothing in CI runs the git guard's test suite, so its 1146 cases gate no change
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
tier: workhorse
---
`scripts/hooks/test-deny-destructive-git.py` holds 1146 cases covering the destructive-git guard, and no workflow runs it. `ci.yml` builds, vets and tests the Go tree, formats it, and builds and tests the VS Code extension. Neither it nor `release.yml` invokes that suite or any other Python under `scripts/hooks/`.

So a change to the guard is gated by nothing. The six checks a pull request touching only those files reports green are the Go and extension jobs, which never read the file that changed. A reviewer scanning the checks sees the same six green ticks a Go change earns and has no signal at all about the change in front of them.

## How this surfaced

Found during code review of dinah-291, which changes the guard and its suite and nothing else. The implementer reported the suite passing at 1146 cases, which was true and was a Windows-local figure. The reviewer then found a new case that cannot pass on Linux or macOS at all, because a helper that rewrites a Windows path into its Git Bash spelling does nothing off Windows, leaving two cases building one command and demanding opposite answers of it. Nothing would have caught that before merge, and nothing would have caught it after.

The defect is dinah-291's to repair. The reason it could reach review unnoticed is this card's.

## What the fix has to settle

**Which platforms the suite must run on.** The guard's subject is Windows path spelling and Windows drive letters, and the operator works on Windows, so a Windows job is the one that matters most. The dinah-291 defect is the opposite case, a suite that passes on Windows and cannot pass elsewhere, so a single-platform job would have missed it. The extension job already runs on two platforms for path-handling reasons the repository has written down, which is the nearest precedent.

**What counts as passing.** The suite reports a count. A run that executed nothing must not report success, which is a hazard this repository has already met and written up: the extension's unit layer once matched no files and went green until somebody noticed. Whatever job runs this suite reads the count back rather than trusting the exit code.

**Whether the release gate counts it.** `release.yml` waits on a fixed number of check runs, and that number has been wrong once before when a job was added without updating every place it appears. A new job means that count changes.

**Whether the guard's own tests can express a platform.** The suite mixes cases that are Windows-only with cases that are universal, and the dinah-291 defect is exactly a case in the wrong group. Whether the suite can say which it is, and fail rather than silently pass when a case is mis-grouped, is worth deciding here rather than leaving to whoever writes the next case.

## Related

dinah-291 repairs the guard's path classifier and is where this was found. dinah-325 covers the guard's fail-open behaviour. dinah-327 covers a false positive on branch names. All four are the same file, and this is the one that would have caught the others earlier.
