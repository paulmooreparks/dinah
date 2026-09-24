# Brief for updating the interchange reader

You are updating a reader of the interchange form that section 5.7 of
`core-profile.md` defines. The reader already exists and sits under `out/`
in your working directory. It was written from an earlier revision of
`core-profile.md` by following `BRIEF.md`, which sits beside this brief and
is still the contract the reader has to meet. `core-profile.md` has since
been revised, and `core-profile.diff` shows every line that changed between
the revision the reader was written from and the revision now in your
working directory. Your job is to bring the reader up to the revised
profile by changing as little as the diff requires.

## What you may read

You may read `core-profile.md`, `core-profile.diff`, `BRIEF.md`, this
brief, and the files under `out/`, and nothing else. This section replaces
the section of the same name in `BRIEF.md`, and every other section of
`BRIEF.md` binds you as written. You may rely on what you already know of
RFC 8259, RFC 2119 as amended by RFC 8174, Unicode and UTF-8, and the
Python 3.12 standard library.

Your file tools are confined to your working directory and you have no
shell. Do not try to reach anything outside the directory. No
implementation of this profile is available to you, and you must not look
for one or reason from what you believe one does. The profile is the only
authority. Where it is silent, say so in `out/READINGS.md` rather than
filling the silence from anywhere else.

## How to read the diff

`core-profile.diff` is a unified diff of `core-profile.md` with three lines
of context around each change. A line beginning with a minus sign was in
the earlier revision, a line beginning with a plus sign is in the revision
you have, and every normative statement occupies one line in the shape
section 3.2 of the profile gives. So a statement line that appears only
with a plus sign is a statement the profile added, one that appears only
with a minus sign is a statement the profile retired, and an identifier
that appears on a minus line and on a plus line is a statement the profile
reworded. Read every hunk, including a hunk that changes no statement line,
because prose around a statement can change what it means, and the version
identity in the opening lines is itself a line the diff may carry.

## What you change

Change files under `out/` only, and change each one only where the diff
gives a reason. Where the diff gives no reason to change a file, leave the
file byte for byte as you found it. The reader you were given passes its
own tests against the earlier revision, and every one of those tests must
still pass against the revised profile when you finish.

- `out/scope.json`: classify every statement the diff added, under the
  rules `BRIEF.md` gives, and give the reason. Remove the entry of every
  statement the diff retired. Reconsider the classification of every
  statement the diff reworded, and change it only where the rewording
  changes what a reader of the interchange form can observe. Set the
  `profile` member to the version identity the revised profile states.
  Leave every other entry as it is.
- `out/reader.py` and any module beside it: judge each added or reworded
  statement you classified as `export` or `pair`, and stop judging each
  retired one. Do not restructure code the diff gives you no reason to
  touch.
- `out/READINGS.md`: append one section for each place the revised text
  admitted more than one reading, numbered on from the last `R-<n>` the
  file carries, in the shape `BRIEF.md` gives. Where the diff invalidates a
  reading already recorded, revise that section in place and add one final
  line to it, `Revised: <one sentence saying what changed and why>`, so
  that a reader of the file can tell which readings this update touched.
  Do not renumber, reorder or reword any other section.
- `out/fixtures/` and `out/fixtures/expectations.json`: add fixtures for
  each added or reworded statement you classified as `export`, under the
  rules `BRIEF.md` gives for fixtures, and remove any fixture whose only
  statements were retired. Where the version identity changed, revise the
  fixtures and expectations that carry it, and only those. Every file under
  `out/fixtures/` other than `expectations.json` still appears in
  `expectations.json` exactly once.
- `out/tests/test_reader.py`: extend the tests to cover every fixture you
  added, and keep every case `BRIEF.md` requires. The tests are run for you
  as `BRIEF.md` says, and their output is sent back to you. You cannot run
  them yourself, so write them to be read.

## Rules the files must follow

The rules `BRIEF.md` gives under the heading of the same name apply to
every file you write or change.

## When you finish

End with a message that lists: every file under `out/` you changed and
every file you added, each with one sentence saying why; the identifiers
you treated as added, as retired and as reworded; the number of readings
you appended and the number you revised; the count of statements
`out/scope.json` now classifies under each scope; and every file you read
during the session. If the diff gave you no reason to change any file, say
so, list every hunk you read, and change nothing.
