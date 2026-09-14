---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:33Z
ordinal: 20
note: "internal/msg/locales/en.json carries refusal.dinah.unknown-link (\"this card carries no {kind} link to {detail}\") and refusal.dinah.unknown-link.next, each with a context sentence. `go test ./internal/msg` passes, which is TestEveryKeyCarriesAContext. The shape is declared in internal/contract/shape.go with Values: []string{\"kind\"}, which internal/profile's own guard requires so a declared value is one an entry actually fills."
---
`dinah.unknown-link` carries an `en.json` entry with both the `{name}` text and a context sentence, satisfied by `TestEveryKeyCarriesAContext`.