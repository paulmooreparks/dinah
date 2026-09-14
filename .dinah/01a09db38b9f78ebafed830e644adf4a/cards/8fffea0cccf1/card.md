---
title: The Go style standard document says the mechanical floor is unscoped gofmt -l ., but the repository runs gofmt -l cmd internal
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The workbench document "Go style standard" (id 49) lists the mechanical floor as three commands, including `gofmt -l .`. `.github/actions/gofmt-check/action.yml` runs `gofmt -l cmd internal` instead, and its own comment explains why: `gofmt -l .` would descend into `editors/vscode/node_modules`, where an npm dependency can vendor unformatted third-party Go source. That scoping predates dinah-274 and is correct; the document is simply describing a command that stopped being true.

Found during dinah-274's review: the document is wrong, not the command. The fix is to correct the document's mechanical-floor section to say `gofmt -l cmd internal` and say briefly why it is scoped, rather than to change CI.
