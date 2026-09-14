---
title: A verb's refusals are listed once, and the listing is what runs
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Each verb's ordered list of ways it can refuse exists twice: once as the data the help screen prints, and once as the chain of checks the binary actually walks. Nothing compares them, and eight of the ten verbs now disagree, so a person reading the help is told an order the tool does not follow. Refusal precedence is normative rather than incidental, which means the divergence reaches the published contract and would be inherited by anyone building a second implementation from it, and by the conformance suite. Adding a refusal to a verb is currently two edits, and skipping the second is invisible because no test goes red.
