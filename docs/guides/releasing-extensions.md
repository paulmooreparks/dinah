# Signing and Releasing a Dinah Extension

Paul asked on 28 October 2026 how to sign a release of the Git extension, believing he had no signing key yet. This guide records what is already in place on his computer and the steps for a release. It applies to any extension Paul publishes, with the Git extension as the example.

## Your Signing Key Already Exists

A signing key was made on 25 October 2026, when the Git extension was first packaged, and it is at `C:\Users\paul\.dinah\extension-keys\paul-parks.private-key`. It is an Ed25519 private key written as one line of base64 by the SDK's `keygen` command. On 28 October I checked that its public half is the key Dinah pinned when it installed the Git extension (`JSsXyoyWDux66kUFFLVdLRtGs0cRqZ/emP0n8sHaDKY=`), so a package signed with this file installs as an update of the one you have.

Dinah pins the first key that signs an extension and refuses any later version signed with another key, so that nobody else can push an update to an extension you trust. That rule makes this file irreplaceable. If it is lost, no update to the Git extension can be installed over the current one; every installation would have to remove the extension and install it again, accepting a new key.

### Protect it

1. **Back it up now.** Store a copy somewhere that is not this computer and is encrypted, such as a secure note in your password manager (the file is 88 characters of text, so it can be pasted). A copy in OneDrive or any synced folder is not encrypted at rest under your control and is the wrong place.
2. **Restrict who can read it.** On 28 October the file could be read by the Windows group `BARN\CodexSandboxUsers` as well as by you, SYSTEM and Administrators, because it inherits its folder's permissions. Anything running under that group could copy the key and sign packages as you. To leave only yourself and SYSTEM, run this in PowerShell:

   ```powershell
   $k = "$HOME\.dinah\extension-keys\paul-parks.private-key"
   icacls $k /inheritance:r /grant:r "${env:USERNAME}:(R,W)" "SYSTEM:(F)"
   icacls $k
   ```

   The second command shows the result; it should list only your account and SYSTEM. If a tool you run under that sandbox group ever needs to sign a package, give it the key for that one run instead of leaving the file readable.
3. **Never put it in a repository or a package.** The build script reads it from where it is; nothing copies it.

### If you ever need a new key

Only make a new key for an extension that has never been released, or when you accept that every installation must remove and reinstall the extension. The command is:

```powershell
go run github.com/paulmooreparks/dinah-extension-go/cmd/dinah-extension keygen "$HOME\.dinah\extension-keys\NAME.private-key"
```

It refuses to overwrite an existing file.

## Releasing the Git Extension

The Git extension's repository is `C:\Users\paul\source\repos\dinah-git`, and it is public on GitHub. The release prepared on 28 October is on its local branch `pudl-0.93`, which moves it to PUDL 0.93.0's glyphs and category colours. Dinah must already be on PUDL 0.93.0 before you install it, and it is from 28 October.

1. **Merge the work.** In `dinah-git`, merge `pudl-0.93` into `main`, or ask Dinah Development to.
2. **Set the version.** The version is the constant `Version` in `contract.go` (0.2.0 now), and one test (`git_test.go`) checks it. Raise both, to 0.3.0 for this release.
3. **Write the release notes** at the top of `dist/release-notes.md`.
4. **Test.** Run `go test ./...`, `go vet ./...` and the browser suites under `web/e2e`.
5. **Build and sign.** From the repository's folder:

   ```powershell
   pwsh tools/build.ps1 -SigningKey "$HOME\.dinah\extension-keys\paul-parks.private-key"
   ```

   The script builds the backends for Windows, Linux and macOS, copies the pages, and signs the folder `dist/extension`.
6. **Package it.**

   ```powershell
   go run github.com/paulmooreparks/dinah-extension-go/cmd/dinah-extension zip ./dist/extension ./dist/dinah-git-0.3.0-amd64.zip
   ```

7. **Install it in Dinah.**

   ```powershell
   dinah ext install ./dist/dinah-git-0.3.0-amd64.zip
   ```

   Because the ID and the key match what is installed, Dinah stages the new version and switches to it; a running Git window pauses while it does.
8. **Commit and push** the version change and release notes to `dinah-git`, and tag the commit `v0.3.0`. Publishing the zip as a GitHub release is optional; Dinah installs from the file.

Steps 5 and 7 use the key and change your installed Dinah, so they are yours to run, or to ask a chat to run for you.

For 0.3.0, steps 1 to 4 were done on 28 October 2026: `pudl-0.93` is merged into `main`, the version is 0.3.0 (commit `b4ed0d7`, not pushed), the release notes are written, and the Go tests, vet and both browser suites pass. The release notes file is in the ignored `dist` folder, so it is not committed; it travels with the package. Steps 5 to 8 remain.
