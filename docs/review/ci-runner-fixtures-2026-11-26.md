# CI runner fixture repairs

Build run [45751947224](https://github.com/paulmooreparks/dinah/actions/runs/45751947224), for source `45479ed0`, exposes two machine-dependent test assumptions. The downloaded failed log remains in `%TEMP%/dinah-ci-45751947224-failed.log`. At investigation time the latest master run, `45754206687` for `eefc0d43`, was still running and did not provide completed logs. This branch starts at that same master source.

## Shortcut target identity

`TestAShortcutsTargetIsReadBack` compared path strings. The runner created its temporary file below `C:\Users\RUNNER~1`, and Windows returned the target below `C:\Users\runneradmin`. The assertion therefore rejected two names for the intended file.

The test still writes and reads a real shortcut in its own temporary directory. It now stats the returned target and intended program and requires `os.SameFile`, so an unreadable or different target fails. It no longer requires a particular path spelling. Windows documents [short and long file names](https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#short-vs-long-names), and Go documents [SameFile](https://pkg.go.dev/os#SameFile). No assumption that a volume creates short aliases is needed.

## Empty-chat restart prerequisite

The Devin peer in `TestEmptyChatHostRestartBeforeFirstTurn` simulates the provider and never executes a hook. Actual chat creation nevertheless validates the command location before configuring the managed integration. The test inherited an installed command from PATH on development machines and failed on the runner.

The Devin subtest now supplies a regular disposable command file beside its simulated window and empties PATH. It restores the executable-location hook at cleanup. The original creation, provider persistence, reconstructed App, list, title and load assertions remain intact. The command is never executed. An unmodified-fixture run with PATH restricted to Windows system tools reproduced the runner's missing-command failure; its log remains in `%TEMP%/dinah-ci-restart-before.log`.

## Verification

With Go 1.26.11, the shortcut test, all four empty-chat provider cases, command-location positive and negative controls, and both continued-chat archive cases passed three times. The focused command used `-count=3 -v`; its package result was 32.015 seconds and exit zero. The complete output remains in `%TEMP%/dinah-ci-runner-after.log`. No capability was skipped. An initial compilation attempt exposed an unused import while the edits were being prepared; the import was removed before this complete run.

Only tests and this record change. No product UI, shortcut-removal policy or command discovery changes. Browser checks do not apply to these changes. The full merged-tree gate and GitHub build remain receiving checks; a local focused pass does not establish that the remote build is green.
