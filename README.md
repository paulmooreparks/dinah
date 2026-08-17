# Dinah

<img src="logo/dinah-lantern.svg" align="right" width="256" alt="The Dinah lantern">

[![CI](https://img.shields.io/github/actions/workflow/status/paulmooreparks/dinah/ci.yml?branch=main&label=CI)](https://github.com/paulmooreparks/dinah/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/paulmooreparks/dinah)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/paulmooreparks/dinah)](go.mod)
[![Latest release](https://img.shields.io/github/v/release/paulmooreparks/dinah?include_prereleases&sort=semver&label=latest)](https://github.com/paulmooreparks/dinah/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/paulmooreparks/dinah/total)](https://github.com/paulmooreparks/dinah/releases)

Dinah keeps work moving. Your tasks live as cards on a board made of plain files on your file system. Each card waits at a station, and that station's instructions tell whoever picks up the card exactly what to do there, and whoever picks it up may be you, a teammate, or an AI agent. Work is pulled by whoever is ready rather than assigned by whoever is busy, handoffs pass through review gates, and every move lands on a permanent record.

There is no server to run and no account to create. A workbench is a folder of Markdown you can read, edit, search, and commit like any other part of your project, and the whole tool is one small binary. Agents drive it as naturally as people do, because Dinah was designed for work where AI colleagues take on real tasks and need to know what each one expects without being told every time.

Under the hood, Dinah is a reference implementation of a shared coordination contract for board-based work: workbench definitions, states and transitions, per-state instruction serving, and the claim, move, release, and block actions a worker takes on a unit of work.

<br clear="right">

## Start here

[The quick start](docs/quick-start.md) walks one bench from an empty directory through every command the binary offers, with the real output of each step alongside it. Reading it takes one sitting, and it is the shortest route to using the tool for real work.

## Name

Dinah is a recursive acronym: **Dinah Is Not A Harness.**

Dinah coordinates work. It knows which card is where, who holds it, what the instructions at each station say, and which moves are legal. Running agents, calling models, and managing tools are deliberately left to the harness, and any harness can drive a Dinah workbench.

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
