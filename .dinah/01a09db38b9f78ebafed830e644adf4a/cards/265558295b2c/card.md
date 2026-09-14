---
title: The renumbering stranded every gate that tests a major version
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
Several requirements switch themselves on by testing the contract's major version number, in the shape "this applies from major 3 onwards". The renumbering moved the contract from 3.0 into the 0.x range, so a workbench declaring the current revision reads as major 0 and escapes requirements the current contract genuinely imposes. The state-slug requirement is one confirmed instance, found while checking another card's spec against the landed contract, and every gate of that shape in the tree has the same problem. The cause is that a major number stopped being the axis a breaking change moves along, so any test written against it now asks a question that no longer means what it meant.
