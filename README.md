# Yokoten

Yokoten is a reference implementation of a coordination protocol for board-based work: workbench definitions, states and transitions, per-state instruction serving, and the claim, move, release, and block actions an agent or a person takes on a unit of work. It exists as a single, minimal binary rather than a hosted service.

## Relationship to Andoneer

Andoneer is a separate, hosted, multi-seat implementation of the same protocol. Yokoten and Andoneer share no code. They are kept consistent through a shared conformance suite that exercises both against the same protocol contract, so a change to the protocol has to pass on both implementations before it counts as landed.

## Status

This repository currently contains a skeleton: a Go module, a placeholder binary that prints an identification line, and the build tooling and agent guardrails the project runs under. No protocol commands are implemented yet. That work starts with a later card.

## Name

"Yokoten" is a code name. It has not been through a trademark review and should be treated as provisional, not as a settled product name.

## Building

This is a Go module. From the repository root:

```
go build ./...
go vet ./...
go test ./...
```

The build produces a single binary at `cmd/yokoten`.

## License

Yokoten is licensed under Apache-2.0. See `LICENSE` for the full text.
