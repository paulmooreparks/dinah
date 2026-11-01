# Chat row space and team navigation

Paul requested shared row correction in bus 2528, duplicate body aggregate removal in 2532 and direct team navigation in 2536. The candidate is on fix/ui-successor-20261101, which starts from released 382305c and so already includes the composer and navigation release.

## Source findings

Chats and the Workspaces sidebar share chat-browser.js. Every row created an empty team-state slot after its execution badge, and chat-state.css assigned that slot a clamped width even without an aggregate. A hidden question-count badge also reserved the width of 99+ before the execution badge. The execution badge itself used a fixed compact slot. These reservations reduced the title width independently of visible content.

Paul's rule that nothing moves on a live state change decides the shape (Team Lead, bus 2767). Every compact state mark is now one fixed square at the right of the row, holding a glyph alone, with its words in its accessible label. The four live states already had glyphs. A compact mark for a state with no glyph of its own gets one in its own muted colour: minus for closed or retired, window for a chat open elsewhere, and empty for a state not yet known. The square is sized in em, so it follows each skin's type, and the title takes all the rest of the row. The count slot keeps its hidden 99+ reserve, so a count arriving or going never moves the title. The team slot appears only on rows that have a team aggregate, at the same square size, so ordinary rows no longer carry an empty column. This reclaims the wide trailing reserve that the old clamp of up to 9ch gave every mark. Real row action buttons and populated team aggregate metadata remain available. A list is a separate navigation surface whose title does not supply each listed team's state, so its real aggregate metadata is retained.

The all-consumer inventory found one remaining duplicate body aggregate in teams.js. It was inserted after the Team disclosure and stretched by teams.css. That body aggregate and its styles are removed. Group windows already omit their body aggregate. Team/group title aggregates and individual member statuses, reasons, disclosures and live updates remain intact. This change does not adopt the unpublished PUDL title-glyph suppression API.

## Direct team navigation

The Team section provides an Open Team button next to its disclosure when the current chat has membership. The Actions menu uses the same navigateTeam function. It resolves the current membership's group identity through the existing groupLink/openGroup path, which reveals an existing canonical window or opens the correct closed group. Neither command guesses a title, changes the member draft or inserts a mention. Existing member links that intentionally add a member mention retain their behavior.

Membership refreshes update button/menu availability. Ordinary chats have no Open Team command. A member without its own subordinate team can still open its parent team without an empty disclosure. The section wraps naturally on narrow widths and leaves the bottom composer untouched.

## Verification contract

Rendered checks are queued behind Release's composer gate. They must cover both list surfaces, admitted skins and modes, normal/narrow widths, long titles and lifecycle labels, actual row actions, live updates, focus/selection, populated versus empty metadata and non-overlap. The original list-state-stability geometry assertions are kept unchanged: title, mark and count positions hold through every state and count change. list-row-layout checks the square mark at the trailing edge and that title and mark hold the first state's geometry through every state, across admitted skins, light and dark, and 1440 and 560 widths.

Navigation coverage must exercise button and Actions activation, pointer and keyboard, renamed groups, changed/removed membership, existing and closed targets, cross-workspace identities, draft/transcript/reading-position preservation and absence of sends/provider turns. Body aggregate removal must retain title and individual member state in every affected consumer. No rendered or installed acceptance is claimed before those checks run.

## Last Read consolidation

Paul extended this successor in bus 2605. Regular and team-member chats must place their reading-position action in the existing top controls area alongside the applicable Team and Open Team controls, without a standalone action row. Group chats keep that action in their consolidated group toolbar. Both surfaces show Last Read and the tooltip Go to where you stopped reading.

The source inventory identifies transcript.js as the creator of the regular chat's standalone transcript-unread-controls row. Group windows supply their own stopped button through the same mount contract. The correction must retain that shared target and visibility logic while letting the regular host supply its toolbar button too. It must not move or remove pinned-question content. Toolbar availability must remain correct when team membership or unread state changes, including an ordinary chat with no team controls.

The affected regression matrix must assert the new label and tooltip on both hosts, absence of the standalone controls row, unchanged target/availability, keyboard activation, narrow wrapping, preserved drafts and reading/follow state. The group controls and last-read fixtures currently select the old label and will need matching selector updates alongside behavior checks. The frozen composer/navigation candidate remains unchanged. Startup shutdown probes retain priority in the existing shared slot queue; this extension has no rendered acceptance yet.

The implementation gives the transcript's mount an optional stoppedChanged callback, which it calls whenever it offers or withdraws the control. The regular chat builds its Last Read button and passes it both to the transcript and to the team bar in teams.js. The team bar is therefore the chat's top bar: it shows whenever Team, Open Team or Last Read is offered, so an ordinary chat shows the bar only while there is something unread. Last Read sits at the bar's end. The group window's existing toolbar button is relabelled Last Read with the same tooltip. The new chat-last-read suite checks the label, tooltip, absence of the standalone row, an unmoved composer, and keyboard and pointer activation. The group suites use the new label. Showing the bar for an ordinary chat moves the transcript down by the bar's height, as the standalone row did before; the composer does not move.
