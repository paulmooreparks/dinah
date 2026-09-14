---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:47Z
ordinal: 1
note: "Verified. Test: editors/vscode/test/unit/manifest.test.ts, \"Delete Attachment is declared with a bare title and hidden from the Command Palette\". Command: npm --prefix editors/vscode run test:unit (540 tests, 540 passing).\n\nArmed twice, both plants leaving the tree compiling and the run executing 540 tests.\n1. Plant: package.nls.json's title becomes \"Dinah: Delete...\". Red assertion: assert.equal(titles.get(COMMAND_DELETE_ATTACHMENT), \"Delete...\") reporting + 'Dinah: Delete...' - 'Delete...'. Two existing whole-set guards reddened with it (the manifest honesty guard and both staleness guards), which is the expected blast radius for a manifest string.\n2. Plant: the commandPalette entry is deleted from package.json. Red assertion: assert.equal(entries.length, 1) reporting \"dinah.tree.deleteAttachment has 0 commandPalette entries, wanted 1\". The existing whole-set guard \"every row command is hidden from the Command Palette\" reddened with it.\n\nRestored with git checkout -- after each plant; the suite is green and git status --short is empty."
---
The manifest declares dinah.tree.deleteAttachment with the bare title "Delete..." and hides it from the Command Palette.