# TM-01 provider read and reverse-request boundary

The lead authorized urgent delivery after Quality independently reproduced a negative-limit process panic. The ledger remains open until the green baseline, but installation does not wait for it. Messages 3307 and 3308 narrow the security contract and supersede the initial source-root confinement proposal.

A read must name a live registered chat or active attributed subagent on its requesting connection generation. The read is attributed to that chat. ACP multiplexes sessions within a provider process; a model caller within that same process has no separate authenticated identity. Host validation rejects unknown, closed, cross-provider and retired-generation callers.

Reads retain current access to sibling repositories, dotfiles and SDK paths. Source-root confinement is a separate product decision. Protected paths use the same agents.ProtectedPath policy and managed-execution exceptions as writes. Canonical symlink targets are checked against that policy; confined directory handles and file-identity checks prevent path replacement from silently redirecting the open. This does not sandbox native providers or prevent local same-user hard-link manipulation.

MaxProviderText is a documented 1 MiB response-content bound. Reads stream selected lines rather than loading an entire large file before slicing. An oversized response fails clearly and asks for line and limit; it never silently truncates. Invalid UTF-8 is replaced with U+FFFD, and the returned text includes an explicit replacement note.

Line and limit must be positive when present. A line beyond the end is an error. Large positive limits clamp to the available remainder without arithmetic overflow. Both the request boundary and SliceLines validate ranges. The existing Ollama caller handles SliceLines errors.

Each reverse-request reply contains handler and result-encoding panics. It returns a generic internal error with the original request ID and leaves the connection serving other requests. Panic payloads are not echoed to the peer or diagnostics.

Tests use isolated temporary files and fake protocol peers. They cover unknown/cross-provider callers, stale connections, permitted outside-workspace reads, protected paths and their symlink aliases, oversized responses and bounded large-file sections, invalid UTF-8, numeric edge cases, fuzzed range/parameter decoding and a panicking request followed by successful requests on the same connection. No live provider, credential, terminal or desktop is touched.

The implementation uses the documented os.Root confinement and directory-handle behavior, os.SameFile identity comparisons and filepath.EvalSymlinks resolution. It does not infer filesystem guarantees from probes. The supported Windows host executed both protected and permitted symlink cases without skipping them.

Focused verification passed the provider-read, range, request-panic, session-ownership and subagent-attribution tests. Both fuzz seed sets passed. Five-second fuzz runs with one worker passed 140,721 SliceLines executions and 28,517 reverse-parameter executions. The focused package run also passed under the shared runner lock across agents, ACP, sessionhost, Ollama and desktop; Ollama compiled with no matching tests. Isolated checks used the runner policy's single-test exception. The profile package's forbidden-term, product-name and provider-name guards must pass on the committed handoff tip. Quality independently reviews the diff before Release installs it. The review ledger stays open until the repaired baseline is green.
