# Remote control proposal

This proposal responds to Paul's questions on 25 October 2026 about Claude sessions missing from Remote Control and remote access to all of Dinah. It records a recommendation for review. Remote access is not implemented or enabled by this document. Paul's cross-project Architectural Principles apply, including Dinah's existing exception for its browser workbench.

## Claude Remote Control

Dinah's Claude provider uses the pinned Claude Agent ACP adapter, which calls the Claude Agent SDK. Dinah currently provides no operation that registers its Claude sessions with Anthropic's Remote Control service. Sharing Claude Code authentication does not itself establish a remote connection.

Anthropic documents Remote Control entry points for the CLI, Claude Desktop and VS Code. An interactive CLI session can start with `claude --remote-control`; an existing interactive session can use `/remote-control`. These commands are documented for those products. They do not establish an adapter contract for enabling Remote Control on a running Dinah session. No supported operation for that integration was found in the pinned ACP adapter or the Remote Control documentation reviewed today. This is a finding about our integration, rather than a claim that Anthropic can never support SDK sessions.

An adapter enhancement would require a documented contract for connection setup, session identity, approvals and inbound messages. We should not run another writer against a session already owned by Dinah or adopt undocumented SDK flags to make it appear remotely. Claude's UI would still cover only Claude sessions.

## Recommended Dinah architecture

Dinah should provide a browser client connected to the same backend that owns the desktop's sessions. The remote client must observe and control those sessions without resuming duplicate provider processes. It should cover all providers, documents, files, approvals and terminals through the existing services.

The first version should run on Paul's existing PC and use private-network HTTPS access. Tailscale Serve is one candidate transport; it documents forwarding a local service to other devices in the same tailnet with HTTPS and network access rules. This needs no additional application server hosted by Paul. The PC must remain awake, and the initial implementation would require Dinah to remain running.

The current shell server is a foundation, rather than a ready remote endpoint. It binds to loopback, restricts Host and Origin, uses one process-wide cookie token, and exposes callable App methods through a reflected bridge. A reverse proxy alone would not supply the required remote-client identity, permissions or lifecycle behavior.

The remote boundary needs the following behavior.

- Each paired browser receives its own revocable session, with expiry, secure cookies and authorization checked for every operation. A pairing code is short-lived and exchanged once; normal navigation URLs contain no credentials.
- Remote resources expose explicit capabilities and available actions. The remote API follows the Architectural Principles instead of publishing every reflected App method. A client cannot obtain desktop shutdown, native dialogs or other host-only authority through an alternate route.
- Each browser keeps its own layout, focus, reading position and clipboard. Shared documents retain revision checks; concurrent writes cannot silently overwrite a newer version. Desktop and remote views share backend state without sharing transient presentation state.
- Reconnection restores a consistent snapshot and resumes sequenced events, or explicitly reports a gap and resynchronizes. The present event stream can drop a slow subscriber and has no replay cursor. Remote approvals must retain their identities and reject stale answers.
- Mutating submissions carry stable operation identities. Retrying after a lost connection cannot send a prompt, execute a terminal command or apply an edit twice. Existing steering and recovery semantics remain authoritative.
- Browser uploads, downloads and clipboard operations replace native file pickers and clipboard access. Native-only actions are absent from the remote representation. Extension views receive only the capabilities available to that client.
- Terminal viewing and input have explicit ownership. Closing a browser tab does not kill a shell. A reconnect must not silently replay keystrokes, and viewers need a defined output snapshot and resynchronization path.

## Delivery sequence

The first increment should provide paired browser access to workspace and chat lists, live output, prompt submission, steering, stopping and approvals. It should validate phone layouts and desktop/browser concurrency before extending to document editing and terminals. The full scope remains Dinah-wide control; these increments isolate the different consistency requirements.

A later increment can extract a headless host so provider sessions survive closing the desktop window. The existing architecture already describes this direction, but current native-runtime dependencies need an audit before claiming that the backend can run unattended.

If access from an ordinary browser without a private-network client becomes necessary, an authenticated relay can accept outbound connections from Dinah hosts. Its responsibilities, credential handling and transcript exposure need a separate design. Tela is a possible host, rather than a prerequisite for the private-network version. A relay does not keep agents running while the execution PC is asleep; that requires moving execution to an always-on host.

Verification must cover pairing and revocation, rejected Host/Origin requests, independent desktop and phone state, disconnects during sends and approvals, event loss, concurrent edits, and terminal interruption and reconnection. No network exposure or configuration change is authorized by this proposal alone.

## Sources checked

- [Anthropic Remote Control documentation](https://code.claude.com/docs/en/remote-control)
- [Anthropic Agent SDK overview](https://platform.claude.com/docs/en/agent-sdk/overview)
- [Tailscale Serve documentation](https://tailscale.com/docs/features/tailscale-serve)
- Dinah's `internal/claudeagent/claudeagent.go`, pinned adapter implementation, `cmd/dinah-desktop/shell_server.go` and `internal/bridge/bridge.go`
