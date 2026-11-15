# Recorded fresh-session allocation

Release supplies this standalone lifecycle entry for Engine's approved task host. It is based on master ff236bd1f514a3739844235cd493478ecc0b12a9 and imports no logical journal, conversion or resolver implementation. Engine owns task intent, receipts, admission and final binding. Release owns the lifecycle and native adapter callback wiring. The existing ConfigureServers hook remains unchanged.

```go
func (l *SessionLifecycle) NewRecorded(intent AllocationIntent, callbacks AllocationCallbacks) (Info, error)

type AllocationIntent struct {
    Source, Workspace, Provider, Recipient, Kind, Revision string
}

type AllocationCallbacks struct {
    Dispatched func() error
    Received func(string) error
    Finished func(string, error) error
}
```

Source is an opaque durable operation reference and may identify a task operation. The lifecycle does not resolve it as a native chat or join it to an ownership chain. Kind identifies the operation purpose, such as task_binding. Revision carries the caller's reviewed revision when applicable. Workspace and Provider identify the intended allocation. This entry always creates a fresh recipient; a nonempty Recipient is refused before provider work. It accepts no implicit adoption or automatic retry.

Engine saves its intent before calling NewRecorded. All three callbacks are mandatory and run synchronously outside the shared runtime mutex. Missing callbacks, source or kind refuse before any native request. This entry grants no task execution authority; Engine checks the current person-confirmed review or invocation grants before admission.

Dispatched must persist the outstanding dispatch obligation before session/new can be sent. Received persists the exact nonempty provider-session reference at the first documented success boundary. ACP reports it directly after session/new; Codex reports thread/start success before injection, channel binding, option restoration or title work. Persistence failure stops later preparation, but Info.ID and acp.SessionAllocationError still retain the received identity. A received identity alone never proves setup or task binding succeeded.

Finished runs once for this call and reports the known identity, if any, with the lifecycle setup outcome. Its own persistence failure is returned while preserving that identity. Engine must also retain the final method error, including failures in its subsequent title/options/binding work. A generic protocol or transport error after dispatch is unknown unless a documented adapter response establishes noncreation. Unknown results remain non-replayable. No native delete is used as rollback.

The callback closures use Engine's internal receipt/cleanup persistence path so shutdown admission closure cannot suppress an already owed receipt. The lifecycle calls them even when native allocation or later setup returns cancellation; it supplies no canceled admission context to the closures. This does not authorize fresh execution during shutdown or Rebind. Engine's startup/recovery path reconciles its durable intent and receipts without allocating again.

The optional general BeginAllocation observer remains available for future model coupling. NewRecorded composes that observer with the task callbacks; one observer's receipt failure cannot skip the other observer's receipt. Both dispatch obligations must succeed before a request is sent. Finished callbacks observe lifecycle setup, while each owner's later operation commit remains separate. The standalone implementation enables no model observer.

Verification uses invented homes, fake provider connections and in-memory Codex transports. The lifecycle fixtures cover pre-dispatch refusal, unknown transport outcome, channel failure, stale provider, option failures, receipt failure and final persistence failure. The real App bridge fixture checks the same public entry with a task-operation source and no existing native chat. Codex stand-ins prove receipt persistence precedes injection and that post-start errors retain the ID. Applicable package race and vet results are recorded separately before handover. No desktop instance or browser is required.
