---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:52Z
ordinal: 60
note: "Once `showInfo` and `showWarning` are collected, something has to become of the text. Three answers were on the table.\n\nDiscard it, as the collected `showError` is discarded. Rejected, because unlike an error it is not recorded anywhere else: a check that found defects on three workbenches would report \"Dinah finished all 3 selected rows\" and the reader would never learn there were defects. That is the \"reports success because it never looked\" failure wearing a different hat.\n\nShow it after the loop. Rejected, because it is the duplicate-toast defect with the toasts moved later.\n\nWhat ships: the collected text is written to the channel, one line each in call order, and `summaryFor` gains case 8, `dialog.bulk.allSucceededNotes`, which names how many notes were written. The summary's level is the highest level among the notes, so three workbench checks that found defects escalate to a warning exactly as one check does today, and four empty pulls stay informational. The warning form carries the `dialog.openOutput.label` action, so the channel is one click away.\n\nAttribution is not lost by dropping to one line per note: every note-producing sentence in `src/` is already filled from its own row's label, which §3a derives by sweep rather than by memory. That is why the notes need no `<ref>: ` prefix and why draining per row would buy nothing.\n\nThe partial case takes no notes clause, because `dialog.bulk.partial` already points at the channel the notes were written to. AC-33 drives both levels."
---
A collected informational or warning message becomes a channel note and changes what the summary says, rather than being discarded or shown.