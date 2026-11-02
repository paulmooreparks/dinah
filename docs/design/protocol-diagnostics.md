# Protocol diagnostic redaction

Malformed provider output previously bypassed the connection trace redactors when conn.go formatted its first malformed line into the application Logf sink. Headers-only tracing omitted that payload from the protocol trace but the application log still persisted it under All or Debug. A replayed capability could therefore reach disk despite the registered redaction policy. This is TM-06.

Connection diagnostics now use a connection method that formats the message and applies the same registered redactors and exact-secret replacements as protocol and stderr traces before calling the application logging sink. Both existing diagnostic call sites use it. The malformed-line note also omits the payload entirely, including before a capability is registered. It retains the provider label and the once-per-connection warning. Full tracing may still retain the redacted malformed line; headers-only tracing omits it. No provider message or history is changed.

The disk guard runs with Off, Problems, All and Debug application logging and with full and headers-only protocol tracing. It registers a synthetic current secret and a synthetic replay redactor, sends both in a malformed line, and checks both the protocol file and flushed application log. The provider label also contains both canaries, proving that the diagnostic sink applies registered redactors even when the malformed payload is omitted. A following valid notification must still be served. The guard uses temporary files and pipes without a real provider, credentials or desktop interaction.

The affected ACP trace, malformed-line and desktop persisted-log selection passed under go-gate in ACP (0.509 seconds) and desktop (0.471 seconds). Independent Quality review and exact-tip profile guards are required before handoff.

The static guard parses production ACP source and rejects any reference to package Logf except its declaration and the single call inside Conn.logf. It passed after Quality suggested the guard during its accepted read, keeping future diagnostics on the redacting sink.
