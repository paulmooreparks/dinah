# The independent reader of the interchange form

This directory holds a reader of the interchange form that section 5.7 of the
published profile, `docs/spec/core-profile.md`, defines. It is written in
Python 3.12 against the standard library alone, and it was written from the
profile and from `BRIEF.md` in this directory, by an author who read nothing
else. That author ran as a separate `claude` process whose file tools were
confined to a directory holding those two files, so it never saw Dinah's code,
its tests, its fixtures or its design documents. Dinah's code and its
conformance suite are written from one reading of the profile, and this reader
is a second reading that can disagree with them.

## Who may change these files

`BRIEF.md`, `README.md` and `provenance.json` are maintained by the project in
the ordinary way. Every other file here changes only through a new starved run
that follows `BRIEF.md`, with the run's evidence attached to the card that runs
it. A change made by anybody who has read Dinah's code puts the shared reading
back into the one component that exists to be free of it, so a reviewer of any
card touching this directory checks for that evidence attachment.

`provenance.json` records the commit the author was given, the hashes of the
profile and the brief it read, the harness and model that ran it, and the
number of rounds the run took.

Two scripts support a starved run. `scripts/interchange_reader_prepare.py`
builds the starved directory from git at one commit and writes a manifest of
what it holds. `scripts/interchange_reader_audit.py` reads the transcripts the
run wrote and reports any tool call that named a tool other than the five file
tools or a path outside the starved directory.

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

The comparison scripts' tests need neither a Go build nor the reader:

```
python -m unittest discover -s scripts -p "test_interchange_reader_*.py" -v
```
