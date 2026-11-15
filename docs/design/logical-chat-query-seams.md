# Logical-chat query seams

Release and Engine coordinate read-only consumers through the following pure queries. Engine may supply their interim bodies without importing the unfinished journal. Release replaces those bodies when the durable model is wired. These queries start no provider, discover no native history and open no chat.

```go
func logicalChatReferencesFromSettings(s Settings) []string

type LogicalChatMetadata struct {
    Title     string
    Workspace string
    Lifecycle string
}

func logicalChatMetadataFromSettings(s Settings, reference string) (LogicalChatMetadata, error)
```

The reference query returns sorted, unique local references from saved titles, bus places, transcript identity headers, removal records and explicit binding/transfer/history ownership. SessionProviders alone and native history inventory confer no admission. The eventual model also includes logical-only records, while excluding judgement-purpose records and finalized tombstones. Consumers resolve each reference with logicalChatMembersFromSettings and deduplicate by its ChatID. References remain opaque to consumers.

The metadata query resolves the reference through the same member query. It returns the canonical owner's saved title and workspace, plus lifecycle. Its error preserves ownership ambiguity; a consumer may show a repair row but cannot guess a head or recipient. Missing display metadata stays empty. The interim body uses the resolved head's saved title and captured transcript header. The eventual body uses the logical record, including records with no current native head. Both bodies return detached values.

The captured Settings may carry a nonserialized logicalEvidence map of transcript identity headers. The App query captures those headers once and attaches them to its detached Settings snapshot; clone preserves the detached map for that query. The pure helpers perform no file reads. This field alone imports no journal or conversion behavior. Engine may take this field and its capture/clone support without the unfinished Settings.logical field.

Lifecycle is active when there is no owned removal reservation. Every nonempty RemovedChats reservation blocks ordinary chart admission, regardless of its phase name; the metadata body reports reserved while one exists. The later model's lifecycle supplies additional holds. The chart also retains its existing member-level reservation check, including on partial resolution errors. A metadata lookup cannot release a hold.

Engine owns the App query, its pageMethods registration and captured-generation validation. A query uses one captured Settings/model/header view, checks the applicable generation before returning it and retries or reports stale data under its existing bounded policy. Metadata access does not replace membership Generation or cleanup's mutation recheck. The durable model handover must preserve cleanup reservations, retained member snapshots and the Codex descendant-ownership guard.
