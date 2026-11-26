# Shortcut COM thread lifetime

The Windows shortcut helpers initialized a single-threaded COM apartment without locking the calling goroutine to its OS thread. Go could therefore move the goroutine between initialization, interface calls, release and uninitialization. All three helpers now lock the OS thread before COM initialization and unlock it after their deferred releases and CoUninitialize have finished. Existing S_FALSE handling remains balanced, and incompatible apartment errors still propagate.

The failing run 45772407808 at baa39703 reports document_register_windows_test.go line 152. That line checks writeShortcut, before shortcutTarget is called. The generic automation exception does not identify the failed COM operation or prove a particular scheduler migration occurred. The source violates the documented COM contract independently of that uncertainty. The GitHub runner result after integration is still required to confirm this was its remaining cause. No CI skip, retry or weaker target comparison was added.

Microsoft requires COM initialization on each calling thread, STA object access on its apartment thread, and a matching CoUninitialize for every successful initialization including S_FALSE. Go documents LockOSThread for OS services that depend on per-thread state.

- https://learn.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-coinitializeex
- https://learn.microsoft.com/en-us/windows/win32/learnwin32/initializing-the-com-library
- https://pkg.go.dev/runtime#LockOSThread

The original file-identity round trip remains unchanged. A new test writes a shortcut, retargets it to another real temporary file, reads that target through the production helper, compares file identity, and verifies a subsequent retarget from the old target leaves it alone. These tests use temporary files and do not touch the Start menu or launch any program. No visible interface changed.

## Verification

All checks use Go 1.26.11 on Windows. The original round trip passed 30 repetitions in 1.384 seconds. Both shortcut tests passed 30 repetitions in 3.037 seconds. Logs are retained under the operating system temporary folder as dinah-shortcut-com-focused.log, dinah-shortcut-com-race.log and dinah-shortcut-com-full-go.log. The original failed job is retained as dinah-ci-45772407808-failed.log.
The focused race run passed ten repetitions in 2.483 seconds. Its linker printed a multiple non-default manifests warning, but the test process ran and exited zero with no race report.

The full Go gate passed with exit zero. The staged profile guard passed in 22.822 seconds with no SKIP. Browser tests were not run because the patch changes no browser or visible interface.
