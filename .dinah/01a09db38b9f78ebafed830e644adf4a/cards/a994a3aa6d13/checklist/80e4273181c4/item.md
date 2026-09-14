---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:49Z
ordinal: 11
note: PASS. `dinah mcp --root <nonexistent>` exits 2, writes `dinah.unknown-root` as the first whitespace-delimited token on stderr, and puts zero bytes on stdout, so it served nothing.
---
`dinah mcp --root <a directory that does not exist>` writes `dinah.unknown-root` as the first whitespace-delimited token on stderr, exits 2, and serves nothing.