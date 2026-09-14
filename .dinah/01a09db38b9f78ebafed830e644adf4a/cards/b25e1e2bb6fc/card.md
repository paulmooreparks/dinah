---
title: A phrase in a translator note switches every glossary check off for the whole entry
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The glossary check reads an English message, sees a declared term in it, and requires the translation to carry the declared word. It skips any entry whose translator note contains the phrase "never translated". That phrase is written for a placeholder, meaning this value is a token and stays as it was written, but the skip applies to the entry rather than to the placeholder, so one sentence in a note turns the check off for everything else in the same message.

Forty-four of the 928 catalogue entries are currently exempt this way. Six of the forty-four trigger a declared term today and five of those six already carry the right word, so the live damage is one entry, and that one is a false alarm rather than a real miss: it is the tree command's group-by help, where the axis names are literal vocabulary the translations correctly leave in English. The exposure is the silence rather than today's damage. Any of the forty-four could acquire a real glossary error tomorrow and nothing would say so.

Found on dinah-461, where declaring a new term armed the check for free and three of four keys went red and named themselves while the fourth did not. Code review then established the number and proved the mechanism directly: reverting the translation on the exempt entry left the guard green, and removing the exempting phrase from that entry's English note turned it red, so the phrase is the only thing switching the check off.

THE TRAP, and it is why this is a card rather than a two-line fix. The obvious narrowing, scoping the skip to the placeholder it is written about, does not fix the one live entry, because there the exempting sentence and the triggering words sit in different sentences of the same note. Whoever takes this has to mark those axis names as machine vocabulary in the same change, or the tightened check lands red on day one, on a translation that is correct.

What the card should settle, none of it the operator's to rule on:

What the skip is actually for, and whether the note is the right place to express it. A translator note is prose written for a person, and a check reading prose for a control phrase is a guard whose behaviour changes when somebody rewords a sentence. If the exemption is real it may want to be declared rather than written.

How a value that stays as it was written is distinguished from a value that is translated, and whether the eight catalogues can say that per placeholder rather than per entry.

Whether the forty-four should be swept for glossary errors they are currently hiding, before or after the check is narrowed. Narrowing first turns unknown silence into a pile of reds all at once; sweeping first tells you how big the pile is.

Related: dinah-460 wrote the rule that no word may agree with a placeholder holding an untranslated token, and shipped a considered refusal to build a guard over German because it fires falsely in a language nobody here can adjudicate. dinah-461 shipped the second such refusal, over the reverse glossary direction, on evidence rather than on principle. Both are worth reading before deciding what to guard here.
