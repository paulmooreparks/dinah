# The independent reader of the interchange form

This directory holds a reader of the interchange form that section 5.7 of
the published profile, `docs/spec/core-profile.md`, defines. It is written in
Python 3.12 against the standard library alone. Its author was a separate
`claude` process whose file tools were confined to a directory holding the
profile and a brief, so the author never saw Dinah's code, its tests, its
fixtures or its design documents. Dinah's code and its conformance suite are
written from one reading of the profile, and this reader is a second reading
that can disagree with them.

## Who may change these files

`BRIEF.md`, `UPDATE-BRIEF.md`, `README.md` and `provenance.json` are the
project's, and they are maintained in the ordinary way. Every other file here
changes only through a starved run, with the run's evidence attached to the
card that runs it. A change made by anybody who has read Dinah's code puts the
shared reading back into the one component that exists to be free of it, so a
reviewer of any card touching this directory checks for that evidence
attachment.

`provenance.json` describes the latest run in its top-level members and
carries every earlier run under `history`, oldest first. `card` names the card
that ran it and `mode` says whether the run was a construction or an update.
`commit` is the revision the author was given and `profile_sha256` is the hash
of the profile it read at that revision. `base_commit`, on an update, is the
revision the reader had last been written from. `brief` names the brief file
the author followed and `brief_sha256` hashes it. `harness` and `model` record
what ran the author, `rounds` how many sessions the run took, and `date` the
day the output was copied in.

## Two kinds of starved run

A construction run writes the whole reader from nothing. The author's
directory holds the profile, `BRIEF.md` and an empty `out/`, and the author
classifies every statement, writes the reader, its readings, its fixtures and
its tests. A reader that has to be rebuilt needs a construction run, and
nothing else does.

An update run changes the reader the project already has. The author's
directory holds the revised profile, a diff of the profile since the revision
the reader was last written from, `BRIEF.md`, `UPDATE-BRIEF.md`, and an `out/`
seeded with every file the author owns. `UPDATE-BRIEF.md` keeps `BRIEF.md` as
the contract the reader has to meet and asks the author to change only what
the diff requires.

A card that changes the profile needs an update run when the
`independent-reader` job goes red on its branch for a reason the reader's own
files explain, which is an added, retired or reworded `CORE-` statement or a
changed version identity. A card whose profile change leaves the job green
runs nothing. The diff an update run receives is taken from the revision
`provenance.json` records rather than from the previous commit, so every
change since the last run reaches the next one whether or not the cards in
between ran anything.

The prepare script finds that base revision by the hash of the profile
`provenance.json` records, not by the commit it names alone. A run's commit
can be a branch commit that a squash merge leaves unreachable from the trunk,
while a commit carrying the same profile is on the trunk, and the hash is
what identifies the profile the reader was written from.

## How a run is launched

Three scripts support a starved run, and each records what it did so a
reviewer can check it. `scripts/interchange_reader_prepare.py` builds the
author's directory from git at one commit, seeds `out/` for an update, and
writes a manifest naming the commit, the base, the statements that changed
and the hash of every file it wrote. `scripts/interchange_reader_launch.py`
runs the smoke check that shows the confinement holds, then runs each round
with `--tools Read,Write,Edit,Glob,Grep` fixed in the script, and it writes
the argument vector of every session to the evidence directory before running
it. `scripts/interchange_reader_audit.py` reads the transcripts a run wrote
and reports any tool call that named a tool outside the five or a path
outside the author's directory, and any tool roster a transcript records that
is wider than the five.

A flag passed by hand can be left off, so the flag that confines the roster
lives in the launcher and the command each round ran is recorded in its
evidence, where a reviewer reads it. The roster the audit reads out of a
transcript is evidence beside that record and not a substitute for it.

The evidence of a run, which is the manifest, the diff, the prompts, the
command records, the transcripts, the audit reports and the final `out/`, is
zipped and attached to the card that ran it.

## Known limits

The confinement has one known gap, which the operator accepted as a limit at
dinah-548/questions/14. Under `--restricted`, the harness lets the author
write and then read a file in its own session scratchpad, a directory the
harness creates under the user's temporary directory, although the flag's
help says only that it confines the file tools to the working directories. A
probe under the same flags was refused another session's scratchpad and the
directory above it. The audit reports any such call as outside the root, and
a reviewer of any run reads such a finding and confirms it names a path the
author itself wrote.

A model whose training data postdates 2026-08-14 may have seen this
repository, and nothing about the confinement can rule that out. That is why
`provenance.json` records the model of every run, so a reader of the lineage
can weigh the independence of each one.

## How CI runs it

The `independent-reader` job in `.github/workflows/ci.yml` runs the reader's
own tests, then `scripts/interchange_reader_compare.py`, which asks the reader
and Dinah the same questions. The reader judges the export Dinah writes for
every compatibility fixture. Dinah is handed every fixture in `fixtures/`, and
its verdict is set beside the one this reader's author expected. Both sides
judge thirteen broken variants of one real export. A leak scan also looks for
any member name Dinah writes and the profile never names, quoted in a file the
author wrote.

Any disagreement fails the job until a person has ruled which side is wrong.
The rulings live in `conformance/interchange-reader-rulings.json`, one entry
per disagreement key, each naming its disposition, the card that acts on it,
the open question the operator resolved, and who ruled. A ruling whose
disagreement has gone away also fails the job, so the card that fixes a defect
removes its ruling. The job publishes the comparison's report to its summary
and uploads it as the artifact `independent-reader-report`.

## Running it locally

From the repository root, the reader runs as:

```
python conformance/interchange-reader/reader.py --profile docs/spec/core-profile.md check <file>...
python conformance/interchange-reader/reader.py --profile docs/spec/core-profile.md pair <sent> <returned>
python conformance/interchange-reader/reader.py --profile docs/spec/core-profile.md scope
```

Its own tests read the profile's path from `INTERCHANGE_READER_PROFILE`:

```
INTERCHANGE_READER_PROFILE=docs/spec/core-profile.md python -m unittest discover -s conformance/interchange-reader/tests -t conformance/interchange-reader -v
```

The support scripts' tests need neither a Go build nor the reader:

```
python -m unittest discover -s scripts -p "test_interchange_reader_*.py" -v
```
