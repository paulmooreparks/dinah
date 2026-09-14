---
title: The German help text drops a clause the English and Hindi both carry
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The German catalog's entry for the unsupported-version refusal omits the clause saying it is the workbench that declares the version. The English carries it and the Hindi carries it, so the German alone tells the reader less about where the version being judged comes from.

It says nothing false. It simply drops a fact the other two supply, which matters because that clause is what tells a reader the number under test is theirs rather than the tool's, and confusing those two is the whole subject of the card that found this.

The gap predates dinah-123 and was left alone there deliberately. That card had corrected the German for a different reason, moving a noun to fix a false claim, and its file list was pinned by an acceptance criterion. Restoring the missing clause means rewriting the sentence rather than substituting a word, which would have put an untracked second change into a diff the criterion held to an exact set of files.

What is wanted is the clause restored in German, written in German rather than translated word for word from the English, with the catalog's staleness fingerprint recomputed and a translation decision recorded the way this repository does it. Read the German entry's siblings first, since the surrounding messages establish how this catalog already words the idea, and a phrasing that fights them is worse than the omission.

Check the other six catalogs against the English for the same kind of quiet omission while you are there. Establish that by reading each entry against the English rather than by comparing lengths, and expect the count to be higher than this card suggests, because every count of affected sites this board has produced recently has been short, and the misses have tended to land in the catalogs carrying real translations rather than English placeholders.
