---
title: An English phrase reaches a translated sentence through the detail field
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A refusal's `detail` token is meant to name what the refusal was about, and several raise sites in `internal/bench` build it out of an English phrase instead, such as the slug and state wording that `bench.go` composes before handing it to `contract.RefuseWith`. The phrase is then spliced into whatever language the reader chose, so a Hindi sentence carries an English clause in the middle of it.

This is the same leak as the one dinah-102 closes on the `label` value, one field over. That card scopes its guard away from `detail` on purpose, because widening it would reach into a package that card does not otherwise touch, so the repair needs its own pass over the raise sites and its own widening of the guard.
