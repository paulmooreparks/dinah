---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:47Z
ordinal: 5
note: "Compares the parsed package.json's contributes arrays against the constants imported from src/identity.ts at test time. Each half can fail on its own: a wrong position, a wrong title, a missing palette suppression, a missing or duplicated menu entry, a wrong clause, or a wrong group. The entry-count assertions are what stop a second menu entry being added later and the test still passing on the first one. The existing menu and palette assertions in test/unit/manifest.test.ts are the shape to follow; they are named by their test function rather than by a line, because this card's own diff moves the lines in that file."
---
The manifest declares `dinah.tree.archiveCard` at the position `TREE_COMMANDS` puts it, with title `Dinah: Archive Card`; the id is in `ROW_COMMANDS` and absent from `GLOBAL_COMMANDS`; it carries exactly one `commandPalette` entry whose `when` is the string `false`; and it carries exactly one `view/item/context` entry whose `when` is `view == dinah.workbenchView && viewItem =~ /^dinah\.card\./` and whose `group` is `9_destructive@1`.