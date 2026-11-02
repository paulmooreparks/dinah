# TM-01 provider read and reverse-request boundary

## Expanded policy under Paul's decision 3

Messages 3337, 3357 and 3359 authorize the following follow-up after BL-04 and TM-03. Reads remain unconfined except for explicit deny rules. The current TM-01 behavior below is the installed foundation; this expanded policy is being designed and has not shipped.

The host derives the requesting chat, provider connection and current workspace from its registered session. It resolves current registered local source roots and evaluates requested paths after symlink resolution, using the existing identity-safe open. A page-supplied workspace selection affects only the configuration preview and never grants authority to an agent read.

Rules have stable IDs and one kind: protected, default or custom. Protected rules include the host's state and approval files, retaining only the requesting workspace's managed execution exception. Other workspaces' data outside the requesting workspace's registered roots receives a default denial. Known-sensitive defaults cover SSH key directories and agent endpoints; cloud and Kubernetes credentials; package-manager, container-registry and Git credential files; browser profiles; and password-manager or operating-system credential stores. The implementation must cite the relevant vendor or platform documentation for each resolver and test Windows, macOS and Linux fixtures without inspecting the person's actual credential files. The UI retains an absent default with its example path and reason.

Default path templates and reasons are read-only. The person may enable or disable an optional default, with confirmation before disabling; that transition is audited with its time. Protected rules have no disable action. Custom entries are absolute file paths or directory subtrees, not arbitrary regular expressions. The person may add and remove them, with confirmation before removal. The service resolves existing ancestors for a new leaf and rejects relative, malformed or unresolvable input with a useful error. Prefix matching uses path components and the platform's documented path comparison rules rather than textual starts-with tests.

Every policy-denied read is audited with the matching rule. An allowed read outside the requesting workspace's registered roots is audited with time, stable chat ID, readable title snapshot, workspace, canonical path and bytes returned. Audit records contain no file contents, credentials or applet context. Refusals do not create Notifications. The chat receives a passive unread-refusal count in its existing status detail and tooltip, without changing its execution state. Opening the log acknowledges the displayed chat through a returned audit cursor, preserving later refusals.

The store will keep policy, acknowledgement cursors and rotating audit generations in one protected SQLite database so a policy change and its audit record commit together. Existing rotlog is best-effort and may buffer or drop records; that contract cannot establish an audit prerequisite. This uses the existing SQLite dependency rather than introducing another log-file implementation. Four generations retain at most 1 MiB of serialized audit payload each. Rotation deletes the oldest generation transactionally, while IDs continue increasing. Policy rows and cursors survive rotation.

The database uses rollback journaling, FULL auto-vacuum enabled before schema creation, and a verified 16 MiB main-file page limit. The service verifies the configured pragmas instead of assuming an unknown pragma succeeded. It bounds rule counts and record sizes, serializes its operations and closes query rows promptly. Tests must verify physical file bounds, journal cleanup, logical rotation, stable pagination and failure recovery. SQLite documents [single-database atomic commit](https://www.sqlite.org/atomiccommit.html), [auto-vacuum](https://www.sqlite.org/pragma.html#pragma_auto_vacuum) and [maximum page count](https://www.sqlite.org/pragma.html#pragma_max_page_count). No read requiring an audit record returns content when that record cannot commit. No policy mutation succeeds without its audit record.

### Page and activity contract

The proposed page calls are fixed here before implementation. Front End owns agent-file-access.md and its UI; Engine owns this policy and storage contract.

| Call | Result and behavior |
| --- | --- |
| FileAccessPolicy() | Returns version and one rules list. Each rule has id, kind (protected/default/custom), pattern, reason, enabled, resolvedPaths, examples keyed by platform and actions. Protected actions are empty; defaults carry enable or disable; custom entries carry remove. |
| CheckFileAccessPath(path, workspaceID) | Returns valid, error, resolvedPath, matchedRuleIDs and insideWorkspace. The workspace is resolved from registered settings. It reads metadata only and returns no file contents. |
| AddFileAccessEntry(path, version) | Atomically adds a custom rule and its policy-change audit record, then returns the policy. A stale version fails without changes. |
| RemoveFileAccessEntry(id, version) | Removes only a custom rule, with the same version and audit requirements. |
| SetFileAccessDefault(id, enabled, version) | Changes only an optional default. Protected rules are rejected even if called directly. An unchanged value makes no duplicate audit entry. |
| FileAccessLog(filter) | Returns rows, nextBefore, highWater and retentionStart. The filter contains chat, since (RFC 3339), outcome, before and limit. Limit is bounded to 200, with a default of 50. Rows are newest first and use stable monotonically increasing IDs. |
| AcknowledgeFileAccessLog(chat, through) | Advances only that chat's acknowledgement through the supplied displayed cursor and returns its remaining refused count. It changes no audit rows or deliveries and preserves newer refusals. |

Each log row has id, at, chat, chatTitle, workspace, path, bytes, outcome, ruleID and reason. Outcomes are allowed, denied, error and policy-change. A denied row has zero returned bytes. Policy-change rows also identify the changed rule and its new enabled state or removal. An expired pagination cursor returns a clear retention boundary rather than silently changing the selected range.

Events file-access:read and file-access:policy are emitted only after commit; the latter carries the new version. Acknowledgement and rotation update the cached passive counts before the normal authoritative chat-state event is emitted. The common chat projection exposes refusedReads on every execution state and includes the count in its detail without changing precedence. Policy errors or audit failures return a clear operation error; they never fabricate an allowed read or silently omit its record.

The frontend asks for confirmation, draws only returned actions and acknowledges after successful log display. URLs retain the section, selected chat, time range and outcome. Guards cover every platform's resolved defaults, protected-action rejection, optional-default persistence and audit, custom changes, stale versions, symlink and case variants, allowed/denied audit records, rotation and per-chat cursor acknowledgement. No decision remains open for Paul in this contract; implementation research and independent security review remain required.

## Installed TM-01 contract

The lead authorized urgent delivery after Quality independently reproduced a negative-limit process panic. The ledger remains open until the green baseline, but installation does not wait for it. Messages 3307 and 3308 narrow the security contract and supersede the initial source-root confinement proposal.

A read must name a live registered chat or active attributed subagent on its requesting connection generation. The read is attributed to that chat. ACP multiplexes sessions within a provider process; a model caller within that same process has no separate authenticated identity. Host validation rejects unknown, closed, cross-provider and retired-generation callers.

Reads retain current access to sibling repositories, dotfiles and SDK paths. Source-root confinement is a separate product decision. Protected paths use the same agents.ProtectedPath policy and managed-execution exceptions as writes. Canonical symlink targets are checked against that policy; confined directory handles and file-identity checks prevent path replacement from silently redirecting the open. This does not sandbox native providers or prevent local same-user hard-link manipulation.

MaxProviderText is a documented 1 MiB response-content bound. Reads stream selected lines rather than loading an entire large file before slicing. An oversized response fails clearly and asks for line and limit; it never silently truncates. Invalid UTF-8 is replaced with U+FFFD, and the returned text includes an explicit replacement note.

Line and limit must be positive when present. A line beyond the end now returns an error; previously it returned empty text. Large positive limits clamp to the available remainder without arithmetic overflow. Both the request boundary and SliceLines validate ranges. The existing Ollama caller handles SliceLines errors. Ollama's separate whole-file read before slicing remains a TM-02 resource finding.

Each reverse-request reply contains handler and result-encoding panics. It returns a generic internal error with the original request ID and leaves the connection serving other requests. Panic payloads are not echoed to the peer or diagnostics.

Notification dispatch also contains handler panics. The reader drops the faulting notification and continues serving the connection in order, logging one generic fault line per connection without the method, parameters or panic text. This containment does not roll back handler effects that occurred before a panic.

Quality's independent diff read in message 3322 accepted the file-read and request containment changes and required the notification guard before handoff; the lead ratified that addition in message 3323. TestNotificationPanicKeepsConnectionServingChats passes with two faulting notifications, subsequent requests and notifications for two chats, and a single generic log. No additional independent review is required for this ratified addition before direct Release handoff.

Tests use isolated temporary files and fake protocol peers. They cover unknown/cross-provider callers, stale connections, permitted outside-workspace reads, protected paths and their symlink aliases, oversized responses and bounded large-file sections, invalid UTF-8, numeric edge cases, fuzzed range/parameter decoding and a panicking request followed by successful requests on the same connection. No live provider, credential, terminal or desktop is touched.

The implementation uses the documented os.Root confinement and directory-handle behavior, os.SameFile identity comparisons and filepath.EvalSymlinks resolution. It does not infer filesystem guarantees from probes. The supported Windows host executed both protected and permitted symlink cases without skipping them.

Focused verification passed the provider-read, range, request-panic, session-ownership and subagent-attribution tests. Both fuzz seed sets passed. Five-second fuzz runs with one worker passed 140,721 SliceLines executions and 28,517 reverse-parameter executions. The focused package run also passed under the shared runner lock across agents, ACP, sessionhost, Ollama and desktop; Ollama compiled with no matching tests. Isolated checks used the runner policy's single-test exception. The profile package's forbidden-term, product-name and provider-name guards must pass on the committed handoff tip. Quality independently reviews the diff before Release installs it. The review ledger stays open until the repaired baseline is green.
