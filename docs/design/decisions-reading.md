# Decisions reading layout

The Decisions window gives Paul room to read a request before answering it. It adopts the reference's source-first hierarchy and grouped delivery warning while keeping the existing answer and receipt contract. The Architectural Principles remain at `C:\Users\paul\OneDrive\Documents\Architectural Principles.md`.

The window asks for 84rem through PUDL's documented `data-win-room` attribute. Automatic side docks temporarily become rails when necessary; pinned-open docks keep their chosen placement. PUDL restores automatic docks when Decisions loses the front or closes. This uses the existing window manager without changing saved dock preferences or vendor code. The contract is documented in PUDL's README under window room and implemented in the bundled 0.107.1 release.

The desktop view has a 22rem request list and a remaining reading pane, with a single-pane view below 42rem. The source chat heads each request, followed by its workspace, group and time. The numbered question text precedes its recommendation and answer. Delivery status and Check Delivery share a reserved region near the source, above the questions. A separate reserved review region prevents a changed-request warning from pushing the answer down. The window, list rows and reading pane keep their bounds when selection or arrivals change.

The page inherits PUDL's surface, surface-alt, text, text-muted, border and accent colors and its current font family. Source headings use the existing large type token; question text uses the base type with stronger weight and a comfortable line height. Prose stays within 76ch. The change adds no palette, animation or decorative card system.

As Recommended still fills a saved draft. Send Answers remains the only answer submission step. Partial answers, explicit destinations, dismissal, draft recovery and operation reconciliation remain unchanged. Broadcast retains its preview and confirmation steps. Verification keeps all ten existing Decisions suites and their behavioural assertions, and adds focused geometry and reading-order checks with invented requests in a headless browser. No installed program or real home folder is involved.
