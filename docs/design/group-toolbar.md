# Group controls and transcript boundary

The scoped successor preserves accepted frontend 3c1e68a and follows Paul's final requirements recorded in bus 2449 and 2451. Members, Add Chats, Turn Budget and Jump to Where You Stopped share one toolbar in that order. The member list remains below the controls. The duplicate aggregate in the body is removed, including its rendering and styles; title-bar aggregate status and individual member status remain live and accessible. The earlier aggregate-sizing requirement was withdrawn and is excluded.

## Structure and ownership

The group header contains one ordinary PUDL toolbar and its member list. The toolbar uses the documented toolbar/group markup and keyboard enhancement, with natural wrapping instead of a second folding Group control. Its buttons retain their existing handlers. Members preserves its expanded state and controlled-list relationship.

The shared transcript accepts an optional host-supplied reading-position button. Groups supply that button in their toolbar. The transcript still owns the button's visibility, activation and disposal, so unseen counts and navigation use the same handlers as before. Chat and thread callers omit the option and retain their existing control placement. A group no longer creates a separate transcript-unread-controls row.

A semantic horizontal rule follows the complete controls/member section and precedes the transcript. It uses the shared border token in every skin. Expanded members keep a separate divider between controls and the list; collapsing members removes that interior divider. No skin-specific override or PUDL asset change is involved.

Controls wrap within narrow space, with emergency word wrapping for enlarged labels. The member area retains its existing bounded-height and internal-scroll policy. The toolbar remains outside the transcript's scroll region, and navigation targets and following state remain transcript-owned.

## Verification

Evidence remains under .diagnostics/group-controls-20261101. The matrix covers every admitted skin, light/dark, both motion preferences, normal/narrow widths, expanded/collapsed members and visible/hidden reading controls. It checks ordered control ownership, absence of duplicate body status, retained accessible title/member status, separator geometry/paint and horizontal overflow. Visible jumps use ordinary pointer activation or the toolbar's keyboard End/Enter path. All 128 matrix cases passed in the corrected run, including the toolbar End/Enter path and ordinary pointer clicks. Sixty-four screenshots were retained; representative Gel light/dark, Glaze narrow and Bevel dark images were inspected. Screenshots and the final candidate record preserve outcomes and exact provenance.

Existing group reading, budget, bus interaction and member-stability suites also exercise the affected controls and live status updates. Earlier diagnostic runs retain an obsolete folding-row wait, an at-end keyboard fixture mistake and a genuine large-text narrow overflow corrected before acceptance. No failure is erased or attributed to load.

Quality independently reviews the immutable successor before combined release gating. Accepted native resume work remains separate. The newly reported message-boundary corruption starts only after this candidate is frozen and must not change these reviewable bytes. No native chat reopen, transcript rewrite, install, push or app restart is performed here.
