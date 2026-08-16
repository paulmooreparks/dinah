# Dinah

Dinah is a reference implementation of a shared coordination contract for board-based work: workbench definitions, states and transitions, per-state instruction serving, and the claim, move, release, and block actions an agent or a person takes on a unit of work. It exists as a single, minimal binary rather than a hosted service.

## Name

Dinah is a recursive acrostic: **Dinah Is Not A Harness.**

The expansion is the scope statement. Dinah coordinates work: it knows which card is where, who holds it, what the instructions at each station say, and what moves are legal. It does not run agents, call models, or manage anyone's tools; the harness around the agents is deliberately somebody else's job, and any harness can drive a Dinah workbench.

The name has not been through a trademark review and should be treated as provisional until it has.

## Relationship to Andoneer

Andoneer is a separate, hosted, multi-seat implementation of the same contract. Dinah and Andoneer share no code. They are kept consistent through a shared conformance suite that exercises both against the same contract, so a change to the contract has to pass on both implementations before it counts as landed.

The project was developed under the code name yokoten, the Toyota Production System term for deploying a practice proven in one place horizontally to everywhere it applies. The name moved on, and the practice stayed. A workbench definition is a working method made portable, and every shipped template began as a live board that already worked somewhere.

## Status

This repository currently contains a skeleton: a Go module, a placeholder binary that prints an identification line, and the build tooling and agent guardrails the project runs under. The on-disk format and the surface architecture are designed (see `docs/design/`); no contract verbs are implemented yet.

## Building

This is a Go module. From the repository root:

```
go build -o dinah ./cmd/dinah
go vet ./...
go test ./...
```

The first command produces a single binary named `dinah` at the repository root; naming it explicitly with `-o` avoids depending on Go's default output-file rule for a module with a single command package.

## License

Dinah is licensed under Apache-2.0. See `LICENSE` for the full text.
