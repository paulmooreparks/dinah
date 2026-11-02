# Protected host-file identity

## Cause and scope

Quality reproduced the protected-path bypass on released dbd849c. The lexical containment check treats Windows namespace prefixes as another volume. It therefore fails open for aliases of the same protected file. The ordinary write path also follows directory links through os.WriteFile after checking only the original spelling. This correction is isolated from the unshipped Agent File Access policy and starts at released 60d7f257.

## Boundary and operation contract

Host-mediated agent paths on Windows must use an absolute drive-letter volume. The documented extended drive form is normalized to its drive-letter spelling. Device paths, extended UNC, ordinary UNC and every other volume form are refused before filesystem access. Other platforms require an absolute local pathname. This is the explicit product boundary ratified in message 3431.

Lexical checks remain an early refusal for nonexistent protected descendants. They do not grant access. The final read and write checks compare os.FileInfo identities with os.SameFile against protected root identities and the identities of every directory opened through held parent handles. A case alias or junction therefore cannot obtain permission by changing the path spelling. A registered managed execution directory remains the only state-directory exception; its identity must occur in the opened chain. Provider approval roots retain their independent denial.

Writes resolve existing parents before opening, walk directory handles one component at a time, compare identities before and after each open, and create missing directories only relative to the checked parent. An existing target is opened without truncation, checked for identity and regular-file type, then truncated through that handle. A missing target is created exclusively through the checked parent; a competing creation fails rather than following its target. Managed writes continue refusing symbolic-link redirects. Creation modes retain prior behavior: ordinary directories use 0755 and files use 0644, subject to umask; managed execution directories use 0700 and files use 0600. Existing file modes remain unchanged.

These guarantees use documented [Go Root](https://pkg.go.dev/os#Root), [SameFile](https://pkg.go.dev/os#SameFile) and [Windows namespaces](https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file). They do not sandbox native providers or claim protection against a local same-user process relocating an already opened directory. The namespace boundary and identity checks apply to the expanded deny policy when this correction is merged into it.

## Verification

Focused guards cover read and write namespace forms, a junction into protected state, case aliases, permitted ordinary writes and the managed-execution exception. Windows tests use temporary files only. macOS case behavior must run on a macOS filesystem; a Windows result does not establish that platform result. Quality independently reads the change before direct Release handoff. No live desktop or credentials are involved.
