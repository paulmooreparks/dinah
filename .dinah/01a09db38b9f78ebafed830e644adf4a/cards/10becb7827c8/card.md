---
title: bench.Finding carries no JSON struct tags, unlike every other wire struct in the codebase
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
internal/bench.Finding's fields (Path, Key, Detail) serialize as capitalised Go field names because the struct carries no json tags, while CheckReport.Findings itself is tagged `json:\"findings\"` and every other wire struct in the codebase carries a lowercase snake_case json tag. This forces every client to hard-code the capitalised spelling rather than reading a normalised key.

Flagged during dinah-330's spec (D-7) as out of scope for a VS Code-extension card: fixing the Go struct is a wire-format change that needs its own review, since it touches internal/bench/check.go and whatever already parses its current shape.
