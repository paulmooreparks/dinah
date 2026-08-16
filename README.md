# Dinah

<img src="logo/dinah-lantern.svg" align="right" width="256" alt="The Dinah lantern">

[![CI](https://img.shields.io/github/actions/workflow/status/paulmooreparks/dinah/ci.yml?branch=main&label=CI)](https://github.com/paulmooreparks/dinah/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/paulmooreparks/dinah)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/paulmooreparks/dinah)](go.mod)
[![Latest release](https://img.shields.io/github/v/release/paulmooreparks/dinah?include_prereleases&sort=semver&label=latest)](https://github.com/paulmooreparks/dinah/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/paulmooreparks/dinah/total)](https://github.com/paulmooreparks/dinah/releases)

Dinah is a reference implementation of a shared coordination contract for board-based work: workbench definitions, states and transitions, per-state instruction serving, and the claim, move, release, and block actions an agent or a person takes on a unit of work. It exists as a single, minimal binary rather than a hosted service.

<br clear="right">

## Name

Dinah is a recursive acronym: **Dinah Is Not A Harness.**

The expansion is the scope statement. Dinah coordinates work: it knows which card is where, who holds it, what the instructions at each station say, and what moves are legal. It does not run agents, call models, or manage anyone's tools; the harness around the agents is deliberately somebody else's job, and any harness can drive a Dinah workbench.

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
