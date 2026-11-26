# Shortcut identity and taskbar wait corrections

The shortcut creation test passed in GitHub run 45779945537 after the COM thread fix. Its new retarget test then returned changed=false without an error. The production retarget operation compared only cleaned path strings. Windows Script Host can return a long path for an 8.3 input, so the comparison could reject the same executable. The earlier readback test already accounted for this distinction.

Retarget now accepts identical cleaned spellings or the same existing file according to os.Stat and os.SameFile. The spelling check remains first so an old executable that has already moved can still be recognized. A different file and an unavailable differently spelled path remain unmatched. No save operation changed. The runner's precise returned path was not printed in its failure, so path normalization is the source-supported diagnosis and the next runner result remains the confirmation.

The existing shortcut retarget round trip remains unchanged. A separate identity control uses a real hard link and checks matching aliases, different files, and identical versus different missing paths. A preliminary COM round trip using a hard-link alias passed even with the old comparison, so it was not retained as regression evidence. The identity control tests the matching decision directly without assuming a particular shell spelling.

The taskbar suite selected every stack button and called strict locator.waitFor for detachment. The failed log reports four matching elements at the wait. The selector covers all stack groups, so multiple matches do not show duplicate buttons. The wait now polls the collection count until it is zero, preserving the required absence of all stacks before checking separate buttons. The subsequent narrow-layout overflow assertion is unchanged. This changes only the test, with no product page or visible interface change.

Go documents os.SameFile as an identity check using Windows file information. Playwright documents strict single-element operations and recommends waiting for observable conditions through page.waitForFunction.

- https://pkg.go.dev/os#SameFile
- https://playwright.dev/docs/locators#strictness
- https://playwright.dev/docs/api/class-page#page-wait-for-function

## Verification

Go 1.26.11 ran both shortcut round trips and the identity control twenty times, passing in 2.308 seconds. The headless taskbar-stacks suite passed in nine seconds through run.ps1. Its temporary node_modules junction was removed with System.IO.Directory.Delete before handoff. Logs remain in the system temporary folder as dinah-shortcut-alias-final.log, dinah-shortcut-taskbar-race.log, dinah-taskbar-stacks-ci-fix.log and dinah-shortcut-taskbar-full-go.log. Screenshots and suite results remain in the dinah-taskbar-stacks-ci-fix temporary folder.
The focused race run passed ten repetitions in 2.851 seconds with no race report. Its linker emitted a multiple non-default manifests warning, but the tests executed and returned zero.

The full Go gate passed with exit zero and 73 packages reporting ok. The staged profile guard passed with no SKIP. No push, merge or installation was performed in this worktree.
