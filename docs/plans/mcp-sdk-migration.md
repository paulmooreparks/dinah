# Official MCP SDK migration

## Status and scope

Architecture approved this boundary on 29 October 2026 after reviewing proposal 474a958 against the pinned SDK's exported transport documentation and the old worker/close implementations. The baseline is fd5b5e0. Implementation may proceed in the separate reviewed cuts below. No production code or dependencies changed in this ruling. No installation or push is authorized.

The migration keeps github.com/modelcontextprotocol/go-sdk at v1.8.0. The current inventory corrects the earlier survey: Ollama still uses mark3labs/mcp-go v0.54.1. Removing that dependency requires migrating both production consumers and two test consumers.

| Path | Current responsibility | Proposed owner/API |
| --- | --- | --- |
| cmd/dinah-desktop/applet_mcp.go | Five-tool stdio helper and authenticated HTTP bridge client | Official Server, raw AddTool handlers and StdioTransport; bridge remains unchanged |
| internal/ollamaagent/applets.go | Cached stdio clients, tool discovery, approvals and calls | Official Client and ClientSession; existing cache and approval ownership remain |
| cmd/dinah-desktop/applet_mcp_live_test.go | Packaged hidden helper handshake | Official client plus raw protocol compatibility coverage |
| internal/codexagent/mcp_smoke_test.go | Opt-in HTTP elicitation fixture for installed Codex | Official Server, StreamableHTTPHandler and session elicitation |
| cmd/dinah-desktop/applet_mcp_test.go and applet_mcp_invoke_test.go | Direct HandleMessage tests | Connected in-memory or pipe sessions with equivalent assertions |

The bus server in internal/mcpserver and extension host in internal/extensions already use the official SDK. Their production transports stay unchanged. ACP, Codex provider JSON-RPC and the Ollama model loop are outside this migration.

## Helper compatibility contract

The helper retains its command-line arguments, absolute state-directory check, server identity/version, instructions, five tool names, descriptions, required fields and annotations. Initialization and tool listing work without a running host; each tool call resolves its own private context. Tool discovery must retain the current schema representation, including omitted fields, rather than acquire inferred defaults from typed registration.

Use the official raw Server.AddTool API, whose documented contract leaves argument decoding and validation to the handler. Use explicit schemas and compatibility decoding for current string/numeric getters. Preserve authorization-before-timeout-validation ordering, timeout default 300, integer range 1 through 86400 and existing error text. inputJSON remains an exact string, including large integer bytes. Successful calls return one text content item containing the bridge response. Authorization, validation and bridge failures remain tool results with IsError, distinct from protocol errors.

Architecture approved one characterized tools/list wire difference on 29 October 2026 (message 442). The official SDK may add `ttlMs: 0` and `cacheScope: "public"` to the result envelope. Its exported Cacheable documentation defines zero TTL as immediately stale; it does not prohibit storage. Public scope is acceptable for this helper's unauthenticated, fixed tool catalog and does not authorize caching private tool results or context-bearing data. No transport response rewriter is introduced. Retain the old-wire fixture and failure evidence, assert these two fields explicitly in the migrated wire test, and compare the remaining result and tool definitions without broad field suppression. Verify both explicit 2024-11-05 and normal negotiation paths. This exception covers these tools/list envelope fields only; other wire differences still require review.

Architecture approved a second difference on 29 October 2026 (message 454). For a call to an unknown tool, both helpers return error code -32602, but the official SDK writes its own message (`unknown tool "not-a-tool"` where the old helper wrote `tool 'not-a-tool' not found: tool not found`). Nothing in Dinah matches on that text, and an agent that sees it still learns the tool does not exist. The migrated wire test asserts the code and the correlated request ID and does not assert the message. The old text stays in the characterization fixture as evidence. This allowance applies only to errors the SDK generates itself; messages Dinah writes, such as authorization and validation failures, keep their exact text.

The SDK API reference documents raw registration and configurable framing through StdioTransport and IOTransport. These public APIs are the implementation boundary; SDK internals will not become a correctness dependency. See the [versioned official SDK reference](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk@v1.8.0/mcp).

The HTTP bridge retains loopback-only endpoint validation, bearer authentication, redirect refusal, disabled proxy use, its 25-second timeout, 1 MiB request cap and 24 MiB response cap. No OAuth or new network listener is introduced. Context cancellation terminates the request; it does not promise cancellation of an already durable invocation. applet_cancel retains that separate role. Token resolution, revocation, digest checks and human authorization remain host policy.

## Transport differences and ruling

Source inspection found three differences that prevent describing the whole change as mechanical.

1. The old helper configures four workers and a 32-item queue, but a full queue executes another tool call synchronously on the input reader. That can permit a fifth active call and stops reading cancellation/control messages until it returns. I propose four active calls and 32 waiting calls with an explicit overload error after those slots fill. Cancellation can remove waiting calls, and protocol control messages remain readable. This changes overload behavior and requires approval plus a specified error code/text before implementation. Admission must use public handler/middleware APIs with owned synchronization; no transport-internal assumptions are allowed.
2. Both old stdio readers accept lines without a configured size limit. The official default is 16 MiB, smaller than a valid bridge response, and CommandTransport exposes no frame-size setting. I propose StdioTransport and IOTransport with MaxLineLength set to -1, preserving the current MCP framing behavior while leaving bridge limits intact. The Ollama client therefore needs a small subprocess owner around IOTransport rather than the default CommandTransport. Tests must include a response larger than 16 MiB and within the existing bridge limit.
3. The old client closes stdin, waits two seconds, attempts termination, waits three seconds, kills if necessary and waits another three seconds. Official CommandTransport has different timing and exposes only one duration. The proposed subprocess owner preserves the existing phases and one Wait owner using documented os/exec operations. On Windows it must handle unsupported signals explicitly and retain the existing fallback delay; it must not assume SIGTERM support. Close remains idempotent and releases pending calls. No new global shutdown guarantee is claimed.

If the public SDK interfaces cannot implement the approved admission or close policy without moving an effect or changing lock scope, implementation stops at that concrete conflict for review. A new home-grown JSON-RPC implementation is outside the proposal.

Architecture approves the following concrete contracts. The helper admits at most four executing tool calls and 32 waiting calls. A further call receives JSON-RPC server error code -32000 with message `applet helper overloaded; try again later`, correlated to the original request ID. This is an intentional overload behavior change, separate from existing tool-result errors. Rejection performs no bridge call and adds no automatic retry. Queue cancellation releases its slot and must not subsequently invoke the bridge. Control traffic remains outside this admission queue. Tests must establish these properties through connected public transports, including cancellation while all execution slots are blocked.

Both migrated stdio endpoints set MaxLineLength to -1, as documented by the pinned SDK. The existing HTTP bridge caps remain unchanged. The subprocess owner uses documented platform operations and one Wait caller. Windows takes an explicit unsupported-graceful-signal path while preserving the existing delay before Kill; no SIGTERM behavior is assumed. Pending calls must unblock independently of process exit. A final close timeout remains an error, and the sole Wait owner remains responsible for eventual process collection. Returning from Close does not establish that the process exited. Preserve pipe cleanup and partial-startup cleanup, and test concurrent/repeated Close and EOF races.

## Ollama session and call contract

The existing Conn cache remains the sole cache. Its key still includes server name, command and arguments; changing the current omission of environment values is separate work. Preserve name validation, stdio-only selection, inherited environment plus declared overrides, hidden Windows execution and serialized startup under the existing server mutex.

The subprocess lifetime belongs to the cached server, independent of a single turn context. Preserve startup before the shared 20-second initialization/listing deadline, close partial startup on either failure, and publish only after initialization and listing succeed. Preserve application error prefixes for start, initialization and listing failures. Use one ListTools response as today; introducing pagination aggregation would be a separate behavior change. Cancellation of one call must not kill a shared helper or disrupt another call. Session close must unblock pending calls and reap the process once.

Preserve tool names in server__tool form, schemas, readOnly=true approval bypass, allow/all and chat approval keys, and refusal behavior. Calls still concatenate text content with newlines, trim trailing newlines, ignore non-text content and derive success from IsError. Do not reinterpret structured output or raw input schemas. SDK-generated protocol diagnostics can differ; compatibility tests distinguish application text that must remain stable from SDK diagnostic wording. Any externally observed protocol error-code difference is reported before accepting the cut.

## Cut 2 proposal: the Ollama client and its subprocess owner

Engine proposed this design on 29 October 2026 for Architecture's review. No code for it has been written.

### What the documentation rules out

Three documented rules shape the design, and each removes a question instead of answering it by measurement.

1. The os/exec documentation says that `Cmd.Wait` closes a `StdoutPipe` after the command exits, and that it is incorrect to call Wait before all reads from that pipe have completed. The old client called Wait while its reader might still be reading. The owner therefore creates both pipes with `os.Pipe` and gives the child's ends to `cmd.Stdin` and `cmd.Stdout` as `*os.File` values, so Wait never closes a pipe the SDK is reading. The parent closes its copies of the child's ends after Start. One goroutine calls Wait immediately and records the result, and it is the only caller of Wait.
2. The SDK documents that cancelling the context of a `ClientSession` method terminates that RPC and sends `notifications/cancelled`. It also documents that once the server closes the connection, calls return an error that wraps `ErrConnectionClosed`. `ClientSession.Close` is documented as a graceful close that waits for ongoing requests, which is not a promise to release pending calls. Pending calls are therefore released by context. Each cached server has a lifetime context, every tool call runs under a context that ends when either the turn's context or the server's lifetime context ends, and Close cancels the lifetime context before anything else.
3. `Client.Connect` documents no limit on how long it keeps its context, so the 20-second deadline is not passed to it as a context deadline. Connect receives the server's lifetime context and runs in its own goroutine. If initialization and listing have not finished within 20 seconds, the owner closes the server, which ends Connect. `ListTools` then runs under the same 20-second budget, as today.

### The owner

A new file, `internal/ollamaagent/process.go`, holds an `appletProcess` type with three operations.

- **Start** builds the command with `exec.Command`, not `CommandContext`, because the process belongs to the cached server and not to a turn. It sets the environment to `cmd.Environ()` plus the declared overrides and hides the window. It then makes the two pipes and starts the command. A start failure closes every pipe end it made.
- **Session** connects an official client named `dinah-ollama` over `IOTransport` with `MaxLineLength: -1`, as described above.
- **Close** is idempotent and safe to call concurrently. The first call does the work and later calls return its result. In order, it cancels the lifetime context, calls `ClientSession.Close` (which closes both pipe ends the session holds), and then waits through the old phases. Those phases are two seconds for the process to exit, then a graceful signal and three seconds, then `Process.Kill` and three seconds. If the last wait times out, Close returns an error and the Wait goroutine keeps running so that it still collects the process. Returning from Close does not mean the process has exited.

The graceful signal is behind build tags. On Unix it sends SIGTERM. On Windows it sends nothing and goes straight to the three-second wait, because the os package documents only `Kill` as supported there. The old client called `Signal(SIGTERM)`, got an error back on Windows and then waited, so the Windows timing does not change. The phase durations are fields set from three constants, so tests can shorten them without changing production.

Partial startup follows the same path. If Start succeeds and Connect or ListTools fails or times out, the owner runs Close before returning the existing error prefix.

### What stays the same

The cache, its key, the per-server mutex and its scope do not change. Close still runs under the server's mutex, as `appletServer.close` does today. The tool list, the read-only bypass, the approval keys, the result text and the IsError mapping keep the contract in the section above. An absent read-only annotation still means false.

### Differences the person or a model could see

1. The client now negotiates the official SDK's protocol version instead of pinning the old library's latest version. The only server Dinah gives an Ollama chat is its own helper, which already runs on the official SDK.
2. Tool schemas sent to Ollama are re-serialized from the SDK's value, so key order can differ from today's while the content stays the same. Tests compare schemas as JSON values.
3. Transport error text from a failed call comes from the SDK. This is SDK diagnostic wording, and the existing ruling already allows it to differ.

### A question for review

The old client opened a stderr pipe for the child and never read from it, so a child that wrote enough to stderr would block once the pipe filled. I propose to leave `cmd.Stderr` nil, which the os/exec documentation defines as the null device. Nothing in Dinah reads that output today. The alternative is to keep a pipe and drain it into the application log. That keeps information but adds a goroutine and a policy on log volume, which I would rather leave out of a migration.

### Tests

The tests run the test binary itself as the child process, selected by an environment variable, so they need no build step and no model credentials. In that mode the binary serves a small official-SDK server over stdio with a read-only tool, a tool that changes something, a slow tool that blocks until it is cancelled, a tool that returns more than 16 MiB, an IsError tool and a tool with mixed text and non-text content. Other modes simulate a child that exits before initialization, one that initializes but fails tools/list, one that never answers initialization, one that exits as soon as stdin closes, one that ignores stdin closing but exits on SIGTERM (Unix only), and one that ignores everything until it is killed.

The tests cover these cases:

- Concurrent first calls start one process.
- A failed initialization, a failed listing or a 20-second timeout cleans up the process and is never published to the cache.
- The tool list, schemas, read-only bypass and approval keys match the existing contract.
- Text joining, the IsError mapping and a result over 16 MiB come through intact.
- Cancelling one call leaves another call on the same server running.
- Close releases a pending call while the child ignores every signal.
- Each shutdown phase is reached in order, using the shortened durations.
- Close called repeatedly or concurrently returns one result, including when it races the child's own exit, and the process is collected exactly once.

The full gates run as for cut 1. Nothing is installed or pushed.

### Ruling

Architecture approved this design on 29 October 2026 (message 466), with the following additions.

- **The owner closes its own pipe ends.** The SDK documents that closing a connection unblocks its reads, but `IOTransport` does not document that it closes the Reader and Writer it was given. Close therefore closes the parent's write end itself, which is the stdin close that starts the first shutdown phase, and its read end after the session closes, ignoring an already-closed error. Neither the first phase's EOF nor the end of the SDK's read loop depends on what the SDK does with the files. Closing the read end also ends the read when a grandchild has inherited the child's stdout and keeps it open.
- **Connect's goroutine is joined.** When the 20-second deadline closes the server, the start path waits for the Connect goroutine to return before it reports the error, so no goroutine outlives a failed start or writes to the owner afterwards.
- **Stderr stays nil.** The os/exec documentation defines that as the null device. It removes the old risk of a full pipe blocking the child, and nothing in Dinah read that output. Draining it into the log is a separate decision that a migration should not take.
- The three visible differences (negotiated protocol version, schema key order, SDK wording on transport errors) are accepted, with schemas compared as JSON values.

## Protocol and test coverage

Characterization tests first capture initialization metadata, capabilities, schemas and representative result/error envelopes from the current helper. Retain explicit 2024-11-05 initialization coverage from the packaged test as well as normal SDK negotiation. Verify request-ID correlation, unknown tools, malformed arguments, malformed JSON, cancellation, EOF and clean close using actual connected transports. Direct handler tests alone cannot establish wire compatibility.

Deterministic tests will block and explicitly release bridge calls to verify admitted concurrency, queued cancellation and overload behavior under the approved policy. They will cover authentication failure, context revocation, exact inputJSON numbers, timeout boundaries, HTTP errors and large artifact responses. Logs and failure output must not disclose contexts or bearer tokens.

Ollama tests will cover concurrent startup reuse, partial initialization/list failure cleanup, environment and hidden-command construction, tool-schema copying, read-only approvals, text/non-text/error results, per-call cancellation while another call remains active, EOF and repeated close. Child-process fixtures will verify graceful and forced shutdown ownership without real model credentials.

The packaged live-tagged handshake builds and runs a temporary production helper without installing it. Preserve its unauthenticated failure and authenticated success assertions. The separate real-agent artifact test and installed-Codex elicitation smoke test remain explicitly opt-in; unavailable execution is recorded as skipped, never folded into a claim that all live tests passed. The elicitation fixture must retain accept/decline/cancel coverage and its current request schema.

Every production cut runs focused tests, the full Go suite, vet, live-tag compilation/vet, production build and the full browser gate. Existing digest tests must cover any new production helper/client source that participates in action implementation. The final page audit retains all 271 exported App signatures and unchanged page_api.go. Preserve historical failure evidence and the unavailable race-detector result; this migration does not resolve them.

## Review sequence and dependency removal

1. The first commit migrates the helper and its connected/packaged test harness after characterizing the old wire contract and resolving the overload ruling. It includes framing and cancellation tests. mark3labs remains for the Ollama client and Codex fixture.
2. The second commit migrates Ollama with the narrowly scoped subprocess owner and lifecycle tests. It preserves the existing cache and approval policy. It returns for separate review with full gates.
3. The third commit migrates the opt-in Codex elicitation fixture and removes the old module. Search all source and build-tagged tests for remaining imports, run go mod tidy, and inspect every go.mod/go.sum deletion. Remove only mark3labs and dependencies that become unreachable. Do not upgrade the official SDK or unrelated modules. Re-run full gates and report live test execution separately.

The cleanup plan will mark step 8 complete only after all consumers have migrated, dependency removal has been reviewed and combined-tree gates pass. This proposal leaves step 8 open.
