---
title: Channel-based releases and self-update for the CLI
column: 5ea2db0272fc
state: ready
severity: minor
priority: later
workstreams:
  - fdfdeaaff2dd
---
Tela established a release shape worth borrowing when yokoten starts shipping binaries: modified semver with a build counter promoted through dev, beta, and stable channels, plus an automated update system where the CLI pulls its own updates directly from GitHub. Users select a channel, including a local channel for builds of their own. None of this machinery is needed for the first cut; the card exists so the roadmap carries it and so early release decisions do not accidentally foreclose it. The profile document's versioning was deliberately ruled separately and does not depend on this.
