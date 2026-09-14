---
title: The internal register says workbench too
column: 5ea2db0272fc
state: ready
severity: minor
priority: later
---
The user-facing sweep on dinah-51 deliberately stopped at the binary's own prose, leaving Go comments and test-case names saying bench across roughly forty files. Nothing a person using dinah ever reads is affected, so the change was cut rather than dropped, and the implementer named it as a follow-on worth having. The card moves the internal register for readers of the source, and it stays a comment-and-name change: Go identifiers, package paths, and the internal/bench package name are out of scope, since those are the token spelling the product keeps.
