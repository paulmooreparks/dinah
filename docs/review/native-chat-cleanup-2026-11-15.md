# Native listing rows in chat cleanup

Engine owns this standalone correction on engine/native-cleanup, based on master ff236bd1. It implements Team Lead's message 9229. Front End owns the combined panel, chart and cleanup agreement check and receives this host stage separately from the passive-startup work.

## Change

SessionLifecycle.List now calls the host's RecordListedSessions before returning each native listing page. The host validates the provider/workspace scope and durably records native identities and safe provider titles in Settings.ListedChats. It does not create a transcript, open a chat or request another provider. Provider titles remain separate from the person's SessionTitles override. Empty or private-context titles do not erase a retained safe title. Conflicting identity scopes refuse rather than changing the recorded owner.

ListChatCleanup includes these recorded native rows with their provider, workspace and title, using the existing member resolver and eligibility checks. Native metadata participates in the generation recheck before removal. The archive retains only the selected rows' metadata. Binning uses the existing reservation and retention operation; a later native listing cannot change that reserved record or make it appear in SessionsAt. Restore uses the existing retained snapshot. Final deletion uses the existing capability-checked session/delete path and retains unsupported or failed provider-copy outcomes. Permanent erasure removes recorded metadata and redacts it from migration backups.

This stage records observations only. A partial listing never proves absence, and this implementation marks no row missing. It adds no Codex import authority, ownership edge or logical journal dependency. The later shared-inventory contract governs the passive population and refresh completeness work.

## Verification

The focused native-row tests passed on invented Devin and Claude connections through the real App SessionsAt, ListChatCleanup, RemoveChat, RestoreRemovedChat and DeleteRemovedChat entry points. The six provider/outcome combinations cover supported, unsupported and failed documented deletion. They check unknown message count, settings reload persistence, native-only identity, provider title preservation, absence while binned, retained metadata during a late listing, restore, final tombstones and removal of cached titles. A separate test covers title overrides, cross-program scope refusal and private-context title rejection.

The focused non-race test run with all vet analyzers passed in 4.162 seconds. The broader selected cleanup, native-list and session-lifecycle race run with all vet analyzers passed: desktop 44.661 seconds and sessionhost 1.161 seconds, exit zero. The existing linker warning about multiple non-default manifests remains. Commands used the shared FIFO runner:

```text
go test -tags desktop -vet=all ./cmd/dinah-desktop -run ^TestNativeList -count=1 -timeout=60s
go test -tags desktop -race -vet=all ./cmd/dinah-desktop ./internal/sessionhost -run Test(NativeList|Cleanup|SessionLifecycle) -count=1 -timeout=90s
```

These are host tests with invented connections and isolated homes. No real native deletion, desktop/browser launch, installed-build comparison, push or installation occurred. Front End must combine this stage and prove identity/title/state agreement across all three surfaces on the candidate, including native-row bin, restore and deletion, before the combined fix ships. Engine retains host corrections until that receiving result.
