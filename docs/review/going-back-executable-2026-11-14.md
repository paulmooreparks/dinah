# Retaining the executable for recovery

The work is on engine/going-back-exe, based on master 9386987a. The earlier data-restoration trial used 4fe5a52c; this follow-up identifies how to locate the executable that the person is still running.

The shared installer is internal/programs.Place, called by the command program's toolchain installer. It previously tried to remove every destination.old-* file before it even read the source. It depended on deletion failing for a running executable and also removed the newest inactive fallback. The correction removes automatic pruning entirely. Every set-aside program is retained, so neither running-process visibility nor a snapshot-to-deletion race determines whether it survives. Disk use grows with installations; no automatic pruning policy is introduced here.

The old naming pattern was destination.old-<UnixNano>. The corrected code reserves a unique name with CreateTemp using destination.old-<UnixNano>-<unique-suffix> before renaming the old installed program. It replaces only its own empty reservation, preventing a repeated clock value from replacing an earlier fallback. The existing failure path still puts the previous installed program back when final placement fails.

Only names and sizes in the real bin folder were listed. Eight desktop .old-* names were present at the observation checkpoint. No file contents, versions or process paths were inspected there, and no real executable was copied, started or changed. A name or size cannot identify the running window's build. The updated recovery note tells the person to use the running-program status path and PID, with a read-only process query when that page is unavailable, then keep a normal-named executable copy outside bin before closing. This also protects that copy from older installer processes whose cleanup code has not been updated.

## Verification

The complete touched Go packages passed under FIFO:

```powershell
scripts/go-gate.ps1 ./internal/programs/... ./cmd/dinah/... -count=1
```

The programs package passed in 0.964 seconds and the command package in 1.137 seconds. The tests preserve multiple existing copies through repeated installs, install from a set-aside source, a missing source and a failed final placement.

A Windows-only test copies its own Go test executable into a temporary folder and starts it with CREATE_NO_WINDOW and HideWindow. A pipe handshake establishes that the child is running. Two replacements leave the running image byte-identical; closing only its input pipe ends it normally, and a third replacement still retains the former image. Cleanup targets only that fixture child. No native desktop window, input automation or real product process is involved.

No full browser suite, push, merge or installation ran. The live window's exact executable remains for the person to identify from its running-program report. An unidentified build earlier than the old source used by the previous trial is outside that trial's proof.
