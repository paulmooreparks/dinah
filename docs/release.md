# Releases, channels and promotion

Dinah publishes on three channels, and each of them has its own line of tags,
its own manifest on the rolling `channels` release, and its own counter. The
install scripts read `DINAH_CHANNEL` and fetch the manifest it names, so
choosing a channel is the only thing a person installing Dinah has to decide.

| Channel | Tag shape | What the number counts |
|---|---|---|
| dev | `v{VERSION}.{N}-dev` | builds, one per qualifying push to the trunk |
| beta | `v{VERSION}.{M}-beta` | beta releases within the minor, from zero |
| stable | `v{VERSION}.{K}` | stable releases within the minor, from zero |

`VERSION`, at the repository root, holds `major.minor` and nothing else. It is
bumped by hand, and every counter above is taken within one `major.minor` line.
That is what makes the build counter reset when the minor increments and when
the major increments, and it is why the first beta and the first stable of a
line are `vX.Y.0-beta` and `vX.Y.0` rather than gaps where a zeroth release
should have been. The minor itself resets when the major increments, which is a
convention the person editing `VERSION` keeps rather than something a workflow
computes.

A number means something within its channel and nothing across channels. A dev
number in the hundreds and a stable number in the single digits are counting
different things, so nothing compares one against the other, and no code in
this repository does.

## Why a beta is not a dev build under a second name

A promotion is a choice. The operator decides which of the cards that have
merged since the minor opened make the cut, and a beta assembled from that
subset is a tree nobody has built before. Holding back one card while shipping
the rest is the whole point, and it is also why the numbers cannot correspond:
`v0.1.0-beta` is not `v0.1.125-dev` renamed, it is a different tree.

Because the tree is new, it is checked as a new tree. The promotion workflow
runs the same Go build, vet, test and gofmt passes against what it assembled
rather than inheriting the green result of any dev build the commits came from.
Two branches that each passed their own suite can still fail together, and a
release is the worst place to meet that for the first time.

## Cutting a beta

Dispatch `.github/workflows/promote.yml` with `channel: beta` and three inputs.

`cards` lists the card human-ids the cut carries, comma-separated. Each one has
to resolve to a commit on the trunk, inside the current minor, that the release
workflow tagged. A card that resolves to nothing fails the whole run before any
tree is assembled, and the run names it. The pipeline cannot tell a mistyped
identifier from a card whose merge has not landed yet, so it refuses both the
same way rather than quietly promoting less than what was asked for.

`links` lists the incoming `blocks` and `parked_behind` links of the cards in
the cut, written as `dependent>predecessor` pairs, or the single word `none`
when there are none. Every predecessor must be in the same cut or already
carried by an earlier beta of this minor, because holding back the work another
card was built on ships a tree that has never worked. The check refuses the run
and names every violated pair.

The word `none` is required rather than assumed. An empty input is refused,
since a dependency check that switches itself off when somebody forgets an
input is absent exactly when it is needed.

What happens then, in order:

1. The cards resolve against the tagged commits of the current minor.
2. The dependency pairs are checked.
3. The commits are cherry-picked, in the trunk's own order, onto the tip of
   this minor's latest beta tag, or onto the minor's start commit when no beta
   exists yet. Every pick uses `git cherry-pick -x`, so the provenance trailer
   records which trunk commit it came from, which is how the next cut knows
   what this lineage already carries.
4. The assembled tree is checked on Linux, Windows and macOS.
5. Only then is anything published: the tag is pushed, the six binaries are
   built from the assembled tree, and `beta.json` is written on the `channels`
   release.

A cherry-pick that does not apply cleanly stops the run where it stands. The
sequence is aborted, no check runs, no tag is pushed and no manifest is
written, and the failure names the conflicting commit and the card it belongs
to. Resolving a conflict is the operator's call rather than a pipeline's, so
the answer is to rebase that card's work onto the beta base or to drop it from
`cards`, and then dispatch the cut again. Nothing needs cleaning up first,
because the assembly happened in the job's own checkout and the runner discards
it.

## Promoting a beta to stable

Dispatch the same workflow with `channel: stable` and `beta_tag` naming the
beta to promote. No card selection happens here. The stable tree is the beta
tree, and re-running the selection would make it something other than the beta
anybody tested. The checks run again all the same, against that exact tree,
because a green result belongs to the run that produced it.

## Two tag shapes, permanently

Tags published before dinah-361 carry the build counter in the pre-release
label with the patch pinned at zero, as in `v0.1.0-dev.73`. Tags from that card
forward carry it in the patch position, as in `v0.1.74-dev`. The discontinuity
sits between those two, and roughly seventy tags are on the older side of it.

The old tags stay exactly as they are. Rewriting a published tag breaks
anything that ever resolved it, including a manifest, an install, and any link
somebody saved, and it asserts a build number that build never carried. So the
tag list carries both shapes for as long as this repository keeps its history,
and the counter is computed across both of them: the next dev tag after
`v0.1.73-dev` or `v0.1.0-dev.73` is `v0.1.74-dev` either way.

Anything that reads the tag list to find the highest number has to read both
shapes or the numbering silently restarts at one. `internal/release` is where
that rule lives, `internal/release/tags_test.go` drives it against a fixture
list mixing the two, and `.github/workflows/release.yml` calls it rather than
matching tag strings itself.

## The VS Code extension's numbers are its own

`editors/vscode` is versioned separately and deliberately. Its published
version is committed in its own `package.json` and bumped on its own cadence,
and nothing derives it from a dinah release tag. An extension archive built
locally or in CI carries a version on the `0.0.x` line, numbered by the CI run
number or the checkout's commit count, so that an archive somebody installs by
hand always sorts below a published release and no two of them collide.

What tells a reader whether an installed extension and an installed binary
belong together is the profile revision, the `profile` field
`dinah --json version` publishes, which the extension already checks on every
binary it considers. `editors/vscode/README.md` says so where a marketplace
reader will find it.
