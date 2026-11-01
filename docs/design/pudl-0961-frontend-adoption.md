# PUDL 0.96.1 adoption, the page's half

Dinah takes PUDL 0.96.1 from its published archive (https://github.com/paulmooreparks/pudl/releases/tag/v0.96.1, commit 6ac2906930c0232f464ccdd689cc8881d598530c). Front End checked the archive's SHA-256, d6868e709dd21fa62d183ca93fc773dac7812c3c75df459edff3acd1bdc11611, against the published checksum, and Release verified it independently (bus 2874). Engine's backend half is in pudl-0961-backend-adoption.md, and this branch sits on Engine's commit 89d3d37. Team Lead split the work in bus 2883: this candidate carries the eight skins, the static state glyphs and title suppression, and the three companion palettes follow separately.

## The bundled copies

Both bundled folders, cmd/dinah-desktop/frontend/pudl and internal/web/static/pudl, are replaced whole by the archive's dist, as PUDL.md describes, and their VERSION files say 0.96.1. The two folders are byte-identical, and every file in them is the archive's own except VERSION. The skin catalogue in the dist admits Standard, Bevel, Gel, Glaze, Matte, Ink, Alloy and Glass, and Engine's catalogue reader takes it unchanged.

## Static state glyphs

Paul asked for each chat state to keep one still glyph under every motion preference. A working chat now shows PUDL's play glyph in the accent colour, a waiting chat its pause glyph in the warning colour, an idle chat its square in the muted text colour, and a stuck chat its warning triangle in the danger colour. The spinning glyph and its animation are gone from chat state marks. The provider-starting indicator in the status bar is not a chat state and keeps its own glyph.

## Title suppression

Paul asked for the redundant glyph to the left of the chat's status pill in the title bar to go. A chat window now sets data-win-state-title="none", which PUDL documents (WINDOW-STATUS.md) for a host that supplies its own title status chip: PUDL leaves out its generated title mark and its execution row in Window status, and keeps the host's chips. The chat's own chip, which used to hide itself while PUDL showed the execution state, now always shows the chat's state, so the title has one pill with its glyph and word. The taskbar, docks and stacks keep reading data-win-state as before.

## The companion palettes

Bevel, Gel and Glaze are not admitted in this candidate, and remain outstanding in Paul's requested batch. Built as Dinah theme documents from the generated pudl-theme-*.css, which is PUDL's base with each palette's overrides, each maps onto Dinah's 40 tokens exactly. Dinah's validator then rejects them: warn falls below 4.5:1 against the backgrounds in all six forms, danger in five, the raw border falls below the 3:1 input check in all six, and Glaze light's derived warning badge text falls below 4.5:1. Admitting them waits on a rendered inventory of where Dinah uses warn, danger, on-warn and input boundaries, and on a backward-compatible role contract that Team Lead ratifies. The measurement script and its results are kept in .diagnostics/pudl-0961-palettes in the main checkout.

## Verification

The browser suites that cover chat state, the title chip and the taskbar are to run in Front End's next slot: chat-states, chat-state-contract, chat-windows, chat-stop, chat-saved-restore, status-popup, taskbar-state-stability and workspace-identity, then the full suite. None has run on this candidate yet.
