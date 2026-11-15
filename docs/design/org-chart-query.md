# Read-only organisation chart query

The page calls `App.OrgChart() (OrgChartView, error)`. Engine owns this query and its pageMethods entry. Front End owns the addressable page and its presentation. The query uses local state and never starts a provider, discovers native history or opens a chat.

```typescript
type OrgChartView = {
  generation: string;
  generatedAt: string;
  chats: OrgChartChat[];
  teams: OrgChartTeam[];
  groups: OrgChartGroup[];
};
type OrgChartChat = {
  chatId: string;
  head: string;
  title: string;
  workspace: string;
  workspaceName: string;
  provider: string;
  providerName: string;
  state: ChatState;
  resolution: "resolved" | "repair";
  reason: string;
};
type OrgChartTeam = {
  id: string;
  title: string;
  leadId: string;
  memberIds: string[];
  resolution: "resolved" | "repair";
  reason: string;
};
type OrgChartGroup = {
  id: string;
  title: string;
  ownerId: string;
  memberIds: string[];
  resolution: "resolved" | "repair";
  reason: string;
};
```

The existing `ChatState` shape supplies life, window and execution state. All arrays are present, including empty arrays. Times use UTC RFC3339. An absent owner or lead has an empty ID. IDs in team and group edges refer only to returned canonical chat IDs. The person is excluded from member edges. An ordinary group may have no chat owner.

The host captures Settings once and uses `logicalChatMembersFromSettings` and one alias map throughout the query. ChatID supplies node identity; Head is a current session reference and cannot supply node identity. Separate recipients remain separate nodes. Every removal reservation excludes its chat until restoration has completed, including interrupted-operation phases. Unresolved ownership produces a repair node with an empty Head and a reason. It never produces guessed edges; the affected team or group has repair resolution and a reason. The page offers no management actions.

Generation identifies the captured logical model and the local team, group and chat-state observations represented by the result. It changes when any represented fact changes. The query does not claim a transaction across the settings store and bus database. It verifies the model generation before returning and refuses an inconsistent snapshot rather than returning edges against a different model. The page refreshes on existing chat-state, bus, removal and restoration events. A generation change alone does not require layout; the page compares canonical nodes and edges and lays out again only when structure changes.

This contract was agreed for the read-only stage on 15 November 2026. The standalone implementation uses the interim member resolver and the pure inventory and metadata helpers specified in logical-chat-query-seams.md. Captured transcript headers travel in private snapshot metadata and are never serialized into Settings. The query validates the captured ownership and header stamps before returning. Release's durable model can replace those helper bodies without changing this page contract.


Release confirmed `logicalChatReferencesFromSettings(s Settings) []string` as the inventory seam. It returns sorted local logical ChatIDs from the captured snapshot, including logical-only, repair and lifecycle-held records. It excludes judgement records, finalized tombstones and provider-history-only entries. The query resolves those references through the pure member resolver and then applies its removal and repair rules. Both functions share the importer's local-evidence rule.
