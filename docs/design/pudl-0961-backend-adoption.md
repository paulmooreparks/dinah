# PUDL 0.96.1 backend adoption preflight

Lead 2754 authorizes a UI/PUDL-only release from 382305c9ccfb1546b9a04dd9c4b07c4fbdd9eefe. Engine prepared fix/pudl-0961-backend in the pudl-adoption worktree from that exact revision. Unreleased startup, shutdown and notification work is excluded. No live restart is authorized.

Release 2760 confirms that corrected 0.96.1 is not yet published. Version 0.96.0 remains adoption-held. Engine has copied no candidate assets and changed no admission metadata. Release owns immutable publication verification and Front End owns both asset/catalogue copies; Engine must use the same verified artifact set.

## Existing backend behavior

internal/skins/skins.go parses catalogue version 1, accepts valid lower-case IDs of up to 32 characters, requires each admitted non-standard stylesheet to be a local CSS filename with a SHA-256 digest, and validates the actual embedded bytes independently in each host. Choices and server-side stylesheet links derive from the catalogue. No four-skin whitelist exists. Missing saved skin IDs retain their preference and render Standard.

Desktop appearance reads built-in theme documents from the shared frontend/theme-builtins.js data. Its Go validation and tests enumerate that data rather than duplicating a theme-ID list. Front End should supply any new published palette mapping through the existing appearance transaction and preserve custom documents and overrides. Theme admission cannot bypass the existing palette/category validation.

The bus web host currently renders the saved skin plus light/dark/system mode. It does not render the active named desktop palette. Therefore adding palette documents to desktop data alone cannot establish web palette adoption. The final published palette selection/persistence contract must determine the required backend representation and server-side rendering. A palette must not be inserted into the mode-only reader or silently omitted from web validation.

## Remaining integration work

After Release verifies corrected publication, compare the actual catalogue and spec against the parser, including any new palette catalogue and selection axis. Coordinate the web palette representation with Front End before changing storage or markup. Extend only the backend capability required by that published contract; do not invent proposal names, widen admission silently or fork published assets.

Integrate Front End's exact verified asset/data commit into this isolated backend line for the existing skins, appearance and web package checks. Verify independent embedded hashes in desktop and web, every admitted ID, retained unknown preferences, appearance revisions/custom themes, server-side first paint and served assets. Quality owns independent rendered review; Release owns complete merged gates, installation and master-only publication.

This preflight contains no runtime change or completed gate claim. Source work and asset adoption remain ready to proceed when the corrected immutable artifacts arrive. The publication is the current external dependency.
