# Combined org chart progress

The combined worktree is frontend/org-chart-combined, based on master db6f8d1131ceae0cefa6a3e8e08848f16d2b2584. Merge 0b453430 combines the accepted view-only chart at d90ad7ba with the cleanup. Its only conflict was app.css; both independent style sections are kept.

Engine's earlier query at 0a9016da is intentionally absent because it depends on an unfinished logical-model stage. Engine is porting the query onto the interim resolver and owns that host integration. The combined real-query check awaits its tested immutable tip. The chart has not received host acceptance.

The opt-in org-chart-hidden.cjs runner uses the real hidden desktop bridge, an invented home and only local network requests. It calls OrgChart without substituting a response, checks canonical rendering and group labels, renames a local group through the real host, samples every frame until the title changes, checks the actual generation and repeats the page at both widths in light and dark. A reload must retain the new local title. It records a program hash and results, and closes the hidden program and browser.

The org-chart-seed helper creates ten invented closed chat records, team relationships and two groups with the existing local bus API. It built successfully through the shared test lock, and generated %TEMP%/org-chart-host-seed-20261115. The runner passed node --check. These are preparation checks only; no hidden chart execution has occurred yet.

The next step is to merge Engine's standalone query tip, build the combined hidden program and run this harness, followed by the focused chart and cleanup browser checks. The next chart stage also retains the Team Lead's nonblocking lane-spacing improvement from message 8866. Import from Codex follows this combined handover; its page contract at 4948af3c is accepted, but its host remains subject to Release's resolver gates.
