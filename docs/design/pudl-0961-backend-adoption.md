# PUDL 0.96.1 backend adoption preflight

Lead 2754 authorizes a UI/PUDL-only release from 382305c9ccfb1546b9a04dd9c4b07c4fbdd9eefe. Engine prepared fix/pudl-0961-backend in the pudl-adoption worktree from that exact revision. Unreleased startup, shutdown and notification work is excluded. No live restart is authorized.

Release 2760 confirms that corrected 0.96.1 is not yet published. Version 0.96.0 remains adoption-held. Engine has copied no candidate assets and changed no admission metadata. Release owns immutable publication verification and Front End owns both asset/catalogue copies; Engine must use the same verified artifact set.

## Existing backend behavior

internal/skins/skins.go parses catalogue version 1, accepts valid lower-case IDs of up to 32 characters, requires each admitted non-standard stylesheet to be a local CSS filename with a SHA-256 digest, and validates the actual embedded bytes independently in each host. Choices and server-side stylesheet links derive from the catalogue. No four-skin whitelist exists. Missing saved skin IDs retain their preference and render Standard.

Desktop appearance reads built-in theme documents from the shared frontend/theme-data.js data. Its Go validation and tests enumerate that data rather than duplicating a theme-ID list. Front End should supply any new published palette mapping through the existing appearance transaction and preserve custom documents and overrides. Theme admission cannot bypass the existing palette/category validation.

The bus web host currently renders the saved skin plus light/dark/system mode. It does not render the active named desktop palette. Therefore adding palette documents to desktop data alone cannot establish web palette adoption. The final published palette selection/persistence contract must determine the required backend representation and server-side rendering. A palette must not be inserted into the mode-only reader or silently omitted from web validation.

## Remaining integration work

After Release verifies corrected publication, compare the actual catalogue and spec against the parser, including any new palette catalogue and selection axis. Coordinate the web palette representation with Front End before changing storage or markup. Extend only the backend capability required by that published contract; do not invent proposal names, widen admission silently or fork published assets.

Integrate Front End's exact verified asset/data commit into this isolated backend line for the existing skins, appearance and web package checks. Verify independent embedded hashes in desktop and web, every admitted ID, retained unknown preferences, appearance revisions/custom themes, server-side first paint and served assets. Quality owns independent rendered review; Release owns complete merged gates, installation and master-only publication.

This preflight contains no runtime change or completed gate claim. Source work and asset adoption remain ready to proceed when the corrected immutable artifacts arrive. The publication is the current external dependency.

## Shared appearance preparation

Engine and Front End agreed in 2787 through 2789 to preserve the existing appearance.active, themes and overrides format. Engine adds internal/web/static/theme-data.js and a byte-identity test against the desktop copy; Front End subsequently owns both copies in one adoption commit. No new PUDL palette names or selection axis is assumed before publication.

The backend preparation extracts desktop appearance decoding, validation and category migration into internal/appearance. Desktop keeps its existing function entry points and test coverage. Web uses the same decoder before serving /appearance.css after host styles. The stylesheet contains both palette modes, follows PUDL data-theme changes, and supplies a no-script preference/system fallback. Font families are CSS-string escaped. Category overrides apply only outside forced colors. Custom content typography and diff tints follow desktop behavior.

The authenticated stylesheet reads saved preferences without writing. Malformed or unreadable libraries render the profile default with a machine-readable fallback header. Signed-out pages receive default appearance CSS without exposing the saved library. These fallbacks preserve the original file. The endpoint uses no-store and nosniff and needs no CSP relaxation.

Prepared verification covers all embedded built-ins, custom palettes and font overrides, mode independence, unchanged preferences, invalid-library fallback, signed-out CSS, safe font quoting and data-copy identity. Existing desktop appearance/category tests must still pass after extraction. No test gate has run on this preparation; the shared runner remains reserved by other work. Corrected immutable PUDL publication remains required for final adoption and independent rendered verification.

## Correction after independent compile failure

Quality message 2829 rejected 46d2db0 before tests could execute. The new web custom-font fixture assigned a field directly inside a map value, which Go forbids. The extraction also omitted the desktop compatibility wrapper for themePaletteCategoryErrors, still called by the unchanged category test. The correction assigns the font through a local value and restores that wrapper through the exported shared PaletteCategoryErrors function. Assertions, validation policy and deadlines are unchanged.

The corrected source passes gofmt and git diff --check. Package tests and vet remain pending in Quality's serialized slot after Front End's UI correction. No acceptance, published-asset adoption or release gate is claimed.

## Published contract review

PUDL supplied 0.96.1 in message 2867 at immutable commit 6ac2906930c0232f464ccdd689cc8881d598530c, archive SHA-256 d6868e709dd21fa62d183ca93fc773dac7812c3c75df459edff3acd1bdc11611. Release independently owns public provenance verification. The catalogue retains version 1 with eight admitted entries and seven optional stylesheets, compatible with the existing parser. Engine changes no catalogue schema or admission policy.

The three published companion themes contain partial authored overrides. Their generated dist stylesheets contain the PUDL base plus those overrides. Host mapping must preserve that distinction and keep named palette selection independent of light/dark/system mode. Published docs/THEMES.md permits host overrides afterwards. Dinah's validation policy remains intact, so an unchanged published palette that fails it needs an explicit host adaptation or a reported admission gap.

A bounded source calculation against the immutable generated CSS found warning/danger contrast pairs below Dinah's 4.5 threshold. Examples include Bevel light warning on background at 3.390 and dark danger on surface-alt at 3.092; Gel light warning on background at 3.945 and dark danger on surface-alt at 3.043; Glaze light warning on background at 3.608 and dark danger on surface-alt at 3.039. This is a partial precheck, not a full host validation result. Front End owns the palette mapping and must run the unchanged complete validation on it.

Quality accepted exact f77450a as the backend base in 2854 after skins/web, thirteen desktop tests and vet passed. The follow-up tests now enumerate both forms of every embedded theme without a hard-coded count and assert that switching among all built-ins preserves a custom document and content overrides and leaves that custom theme selectable. These follow-up changes have not run in a gate yet. The existing accepted base and original compile-failure evidence remain preserved.

PUDL's original tag WebKit result was 113/115; the unchanged rerun passed 115/115, and both initiating causes remain unexplained. Its release owner accepted that documented risk. Dinah's consumer acceptance remains separate.

## Authorized split for the immediate release

Lead message 2883 authorizes adoption of all eight skins, static execution states and title-only suppression while preserving the currently admitted palettes and custom settings. Bevel, Gel and Glaze companion palettes remain outstanding in Paul's requested batch. Their asset presence does not admit their selection. No persisted role-format change, theme-ID exemption, blanket threshold reduction or published-token rewrite is authorized by this split.

The rendered-use inventory must cover warning and danger text, graphics, on-warn and input identification before one complete backward-compatible contract is ratified. The current validator separately checks raw border and the actual derived input-border mix. PUDL 0.96.1 defines the latter as 55 percent text plus 45 percent surface; inventory must distinguish decorative raw border uses from required control identification. Engine will implement no role-policy change before ratification.

Release message 2874 independently verifies the archive and provides the shared expanded source at C:/Users/paul/AppData/Local/Temp/dinah-pudl-0961-verified/expanded/pudl-0.96.1. Engine's source-only comparison of Front End's working copies against that source is retained under .diagnostics/split-asset-source-check.json. It is an observation of mutable preparation and does not replace exact-candidate verification or rendered review.
