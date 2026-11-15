# Dinah fresh-start release check

This check uses master fa3fc79ae729e3ba4d6b1a869f4ba821708c6a70. It builds `cmd/dinah-desktop` with `desktop,production` and `-H windowsgui`, and builds `cmd/dinah`. Both binaries go into a temporary folder. The version remains the source's development version; this is a master-build check, not a numbered Dinah release.

The driver is `cmd/dinah-desktop/frontend/e2e/release-dinah-hidden.cjs`. It copies only the two built programs into an empty temporary home. Environment variables redirect product data and provider settings. The desktop uses `DINAH_SHELL_ENTRY_FILE` and headless Chromium. Browser requests outside loopback are blocked.

The corrected driver passed with exit zero using the private terms file. Its receipt is `%TEMP%/dinah-release-fresh-Si0hBM/result.json`, beside `first-start.png`. The profile guard passed with DINAH_FORBIDDEN_TERMS set and `go test ./internal/profile -count=1` (13.920 seconds). The branch commit was rewritten to remove literal private terms.

| Item | Result |
| --- | --- |
| Both Dinah programs start | Pass. The unstamped command reports `dinah 0.1.0`; the desktop reports `dinah-desktop 0.0.0-dev`. |
| Fresh hidden desktop | Pass |
| One What's New card | Pass |
| Decisions button | Pass |
| Working with Chats button | Pass |
| Background Chats button | Pass |
| Set-aside Programs button | Pass |
| Dismissal survives process restart | Pass |
| Private product content absent | Pass with patterns read from DINAH_FORBIDDEN_TERMS at runtime. The check never prints a term and explicitly skips the assertion when that variable is unset. The feature card has no Updates button, the Settings Updates section is hidden and the product service registry has no updates service. |
| Page errors | None in either process run |

Two initial harness runs reached shell APIs before initialization finished. They failed with an unknown applet and an unavailable window manager. The driver now waits for the registered Settings sections, feature-card module and window manager. The original failures remain in `%TEMP%/dinah-release-fresh-MhfDB7/result.json` and `%TEMP%/dinah-release-fresh-9f4m0o/result.json`. No product change was necessary.

The builds are in `%TEMP%/quality-dinah-release-20261115`. Their SHA-256 hashes are `44b9cb5eedec52b5c90bf4c2231106577aa82f993121f17bc590845db0ce8593` for the command and `9f18f50bfd39431314a1a3df95be68b25314bdcb450095af6d962852c65e9738` for the desktop. The checkout contains only these two program commands. No assertion requires their unstamped version defaults to match.

Builds and the driver run through `scripts/with-test-lock.ps1`. Nothing is installed in a real home.
