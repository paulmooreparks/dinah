# Closed chat title events

The real hidden receiving check found stale names after an external RenameSession call. The browser fixture had emitted agent:titles itself and concealed the missing host event. This standalone correction belongs to the immediate inventory fix requested in message 9267.

RenameSession now emits agent:titles with its saved title-map snapshot after renameSession succeeds. A failed save returns before the event or bus title publication. Closed chats without a runtime or bus recipient therefore notify the page too. The skill creation path no longer emits a duplicate event after calling RenameSession. Existing bus title publication remains in place after successful saves.

The regression test calls the real App method for a closed chat without a recipient, checks the persisted title map during each event, and covers both setting and clearing an override. It asserts that no provider call occurs and that the first event snapshot remains intact after the second rename. A failed-write test places a directory at the isolated settings-file path and verifies that no title event or in-memory title is published.

The first run caught a fixture comparison error when clearing the final override: omitted empty maps reload as nil. The test now compares map entries with maps.Equal, retaining the requirement that persisted content matches the event. No production behavior was changed to satisfy that correction.

The selected closed-chat event, failed-write, concurrent naming and bus title tests passed with race detection and all vet analyzers in 5.801 seconds, exit zero. The existing linker manifest warning remains. The command ran through the shared FIFO runner:

```text
go test -tags desktop -race -vet=all ./cmd/dinah-desktop -run Test(ClosedChatRename|FailedClosedChatRename|ChatsNamedSideBySide|AChatsTitle|Closed.*Title) -count=1 -timeout=60s
```

The scope is a host event correction; Front End owns the central title listener and the combined hidden three-surface check on the candidate. These tests used isolated homes and invented connections. No browser or desktop was launched, no native account was contacted, and no push or installation occurred. No installed-product result is claimed here.
