---
title: The output check measures a joined emoji sequence four columns wider than the table does
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The table pads a field to the width `displayWidth` reports for the whole string, and `columnarFields` in the output check walks a line one rune at a time and adds up `displayWidth` of each rune on its own. The two agree on Latin, on Devanagari, and on CJK, and they disagree on a sequence joined by zero-width joiners: the family emoji the sweep files a card under measures nine columns whole and thirteen summed rune by rune. Nothing fails today because that title only ever lands in a last column, where no field follows it for the disagreement to displace. A field carrying such a sequence anywhere else makes the output check report a misalignment that is not there, or hide one that is.
