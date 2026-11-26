# Go 1.26.11 verification

The module now requires Go 1.26.11. Every setup-go step in the build workflow already reads `go.mod`, so no workflow edit is required. The generated third-party notices also identify the new runtime version.

The go command selected its downloaded toolchain from its module cache. The existing Go installation, user environment and PATH were left unchanged. Toolchain selection follows the [documented Go mechanism](https://go.dev/doc/toolchain).

The Windows amd64 and arm64 runs of `scripts/vulnerability-gate.ps1` passed for both `dinah-desktop` and `dinah`, reporting no called vulnerabilities and an analyzed toolchain of `go1.26.11`. Their reports remain in `%TEMP%/dinah-go12611-vuln` and `%TEMP%/dinah-go12611-vuln-arm64`. Native Linux and macOS scans remain the CI runners' responsibility.

The first full Go gate passed every package except `tools/notices`, which detected the stale generated runtime version. Regenerating the notices corrected that check, and its focused test passed. The original gate log is retained in `%TEMP%/dinah-go12611-gate.log`.

The fresh full gate, `pwsh scripts/go-gate.ps1 ./... -count=1`, then passed with Go 1.26.11 and exit zero. Its complete log remains in `%TEMP%/dinah-go12611-final-gate.log`.

The sibling extension module was scanned separately on Windows amd64 with Go 1.26.11. Its symbol scan reported no vulnerabilities. Its JSON report additionally identified GO-2026-5024 in `golang.org/x/sys` v0.42.0 at module and package level, with no called symbol. Dinah resolves v0.49.0. The extension module was not changed. Reports remain in `%TEMP%/dinah-extension-go12611-vuln.json` and `%TEMP%/dinah-extension-go12611-vuln-text.log`.

This branch contains no Wails upgrade, product UI change, push or product installation.
