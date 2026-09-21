# Brief for the author of the interchange reader

You are writing a reader of the interchange form that section 5.7 of
`core-profile.md` defines. `core-profile.md` is a published profile for tools
that coordinate work on a workbench. The file sits in your working directory
beside this brief.

## What you may read

You may read `core-profile.md` and this brief, and nothing else. You may rely
on what you already know of RFC 8259, RFC 2119 as amended by RFC 8174, Unicode
and UTF-8, and the Python 3.12 standard library.

Your file tools are confined to your working directory and you have no shell.
Do not try to reach anything outside the directory. No implementation of this
profile is available to you, and you must not look for one or reason from what
you believe one does. The profile is the only authority. Where it is silent,
say so in `READINGS.md` rather than filling the silence from anywhere else.

## What you write

Write every file under `out/`, which becomes one directory of a repository
after you finish. Nothing you write anywhere else is kept.

- `out/reader.py`: the reader. You may add further `.py` modules directly
  under `out/` and import them from `reader.py`.
- `out/scope.json`: your classification of every statement of the profile
  whose identifier begins with `CORE-`.
- `out/READINGS.md`: every place where the profile admitted more than one
  reading, and the reading you chose.
- `out/fixtures/`: interchange objects you wrote from the profile, one per
  file, each named `accept-<name>.json` or `refuse-<name>.json`, where
  `<name>` is lowercase letters, digits and hyphens.
- `out/fixtures/expectations.json`: the verdict you expect for each fixture.
- `out/tests/__init__.py`, empty, and `out/tests/test_reader.py`: the
  reader's own unit tests.

## The command line

The reader runs under Python 3.12, imports nothing outside the standard
library and the modules you write under `out/`, and never reaches the
network. It takes the path of the profile as a required option and reads
the profile at run time. It extracts the statements with the expression
section 3.2 gives. It finds its own `scope.json` beside `reader.py`.

    python reader.py --profile <path> check <file>...
    python reader.py --profile <path> pair <sent> <returned>
    python reader.py --profile <path> scope

`check` judges each file as an interchange object. `pair` judges two files
together. `<sent>` is an interchange object that was handed to a tool, and
`<returned>` is the interchange form that tool wrote after reading it.
`scope` compares `scope.json` with the statements the profile publishes.

Every command writes one JSON document to standard output, encoded as UTF-8,
and nothing else to standard output. Diagnostics go to standard error.

Exit codes:

- 0: the command ran. A refused object is still exit 0, because the verdict
  is in the output.
- 2: the command line was not understood.
- 3: the profile could not be read, or yielded no statements.
- 4: from `scope` only, some statement whose identifier begins with
  `CORE-JSON-` is not classified in `scope.json`.

## The output

Every document carries a `reader` member:

    { "name": "interchange-reader",
      "profile": "<the version identity the profile states in its opening lines>",
      "profile_sha256": "<SHA-256 of the profile file's bytes, lowercase hex>" }

`check` writes `{ "reader": {...}, "results": [ <one result per file, in the
order given> ] }`, where a result is:

    { "file": "<the path as given>",
      "verdict": "accept" or "refuse",
      "refusal": "<a refusal name section 6.1 declares>" or null,
      "statements": [ { "id": "<identifier>",
                        "result": "pass", "fail" or "not-applicable",
                        "detail": "<one sentence>" } ] }

A result carries one entry in `statements` for every statement `scope.json`
classifies as `export`, in the order the profile publishes them. `verdict` is
`refuse` when you judge that a tool conforming to the profile is required to
refuse the object, and a `refuse` verdict carries at least one statement whose
`result` is `fail`. `refusal` is the name you judge such a tool would report,
or null where the profile fixes none. A file whose bytes are not valid UTF-8,
or that does not parse as JSON, is still a result, never a crash.

`pair` writes `{ "reader": {...}, "result": { "sent": "<path>", "returned":
"<path>", "statements": [ ... ] } }`, with one entry for every statement
`scope.json` classifies as `pair`, shaped as in `check`.

`scope` writes:

    { "reader": {...},
      "classified": { "export": <n>, "pair": <n>, "none": <n> },
      "unclassified": [ "<identifier>", ... ],
      "stale": [ "<identifier>", ... ] }

`unclassified` lists CORE statements the profile publishes that `scope.json`
does not classify. `stale` lists identifiers `scope.json` classifies that the
profile no longer publishes.

## scope.json

    { "profile": "<version identity>",
      "statements": {
        "<identifier>": { "scope": "export", "pair" or "none",
                          "reason": "<one sentence>" } } }

Classify every statement whose identifier begins with `CORE-`. Use `export`
when the statement can be judged from one interchange object alone, `pair`
when judging it needs an object and what a tool wrote back after reading it,
and `none` when the statement says nothing a reader of the interchange form
can observe. Which statements govern the interchange form is itself part of
your reading, so decide it from the profile and give the reason.

## READINGS.md

Write one section per reading, in this shape:

    ## R-<n>: <a short title>

    Statements: <identifiers>

    Profile text: <the lines you are reading, quoted exactly, with their section number>

    Readings considered: <each reading, as a sentence>

    Chosen: <the reading the reader implements>

    Why: <the reason, from the profile>

Record a reading wherever you had to choose, including where the profile says
nothing about something the reader has to decide. Do not resolve an ambiguity
silently in code.

## Fixtures and your tests

Write fixtures from the profile's own material: the example in section 5.7,
the workbench of section 10.1, and each statement you classified as `export`,
with at least one object that satisfies it and, for a MUST or MUST NOT, at
least one that breaks it and nothing else. `expectations.json` is:

    { "fixtures": [
        { "file": "<file name>",
          "expect": "accept" or "refuse",
          "refusal": "<refusal name>" or null,
          "statements": [ "<identifier>", ... ],
          "source": "<where in the profile this fixture comes from>" } ] }

Every file under `out/fixtures/` other than `expectations.json` appears in
it exactly once.

`out/tests/test_reader.py` uses `unittest`. It reads the profile path from
the environment variable `INTERCHANGE_READER_PROFILE` and fails, rather than
skipping, when that variable is unset. It covers every fixture against its
expectation, byte-level cases built inside the tests (bytes that are not
UTF-8, a byte order mark, text that is not JSON), the `pair` command, the
`scope` command, and each exit code. The tests are run for you with

    python -m unittest discover -s out/tests -t out -v

from your working directory, and their output is sent back to you. You
cannot run them yourself, so write them to be read.

## Rules the files must follow

- Every file is UTF-8 with LF line endings.
- In any `.md`, `.json` or `.txt` file, the letters b, e, n, c, h in that
  order, in any case, appear only inside the word "workbench".
- Prose in `READINGS.md` is complete sentences, with no em-dash or en-dash
  characters and no curly quotation marks.

## When you finish

End with a message that lists every file you wrote under `out/`, the number
of readings in `READINGS.md`, the count of statements you classified under
each scope, and every file you read during the session.
