---
title: The guides teach a command that does not exist on Windows
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Two guides tell a reader to run `cut -d' ' -f1` to lift the name out of an error message, and neither names a shell. That command is POSIX: it is absent from PowerShell and from cmd, so the instruction fails for a reader on Windows, which is a platform Dinah builds and releases for. `verbs.md` has shipped the sentence for some time and `first-session.md` repeats it, so the guides now teach it twice. The question is not only how to spell the advice portably; it is whether a guide that gives a shell instruction should say which shell it assumes, which is a convention decision affecting every document the product ships. Two Agent Code Review passes on dinah-164 flagged this and neither tagged it, on the correct grounds that the sentence predates that card.
