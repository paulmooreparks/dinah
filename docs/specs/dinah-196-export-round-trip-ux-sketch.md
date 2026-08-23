# dinah-196 UX sketch: what `dinah export` prints, and what comes back

Every block headed **Today** is what this build prints, derived by reading
`Bench.Export` and `Frontmatter.Value` rather than by running the binary, because the spec
stage does not build. Every block headed **Proposed** is drawn by hand against this card's
spec. Indentation of the JSON follows `json.MarshalIndent(object, "", "  ")`, which is what
`Export` already calls, so an array member takes one line per element. Treat the drawings
as intent.

The workbench these transcripts run in is a throwaway board that declares both level sets
and a display overlay. Its `workbench.md` frontmatter reads:

```yaml
---
format: 1
profile: dinah-core/1.0
title: Sample workbench
slug: sample
operator: sam
levels:
  severity: [trivial, minor, major, critical]
  priority: [later, soon, next, now]
groups:
  DESIGN: [004acda2c28a, 7b5cfe51cb3b]
  BUILD: [68091798ab26]
states:
  - 004acda2c28a   # Intake
  - 7b5cfe51cb3b   # Doing
  - 68091798ab26   # Done
---
Every card on this workbench ends with a line in the changelog.
```

## 1. What `dinah export` prints today

Today. Dinah prints no `levels` member at all, and it prints `groups` as an empty string.
You declared four severities, four priorities, and two folders of states, and none of that
survives the print.

```text
$ dinah export
{
  "groups": "",
  "instructions": "Every card on this workbench ends with a line in the changelog.\n",
  "profile": "dinah-core/1.0",
  "states": [
    {
      "id": "004acda2c28a",
      "kind": "intake",
      "slug": "intake",
      "title": "Intake"
    },
    {
      "id": "7b5cfe51cb3b",
      "kind": "work",
      "slug": "doing",
      "title": "Doing"
    },
    {
      "id": "68091798ab26",
      "kind": "done",
      "slug": "done",
      "title": "Done"
    }
  ],
  "title": "Sample workbench"
}
[exit 0]
```

Today. The same loss reaches `dinah init --from` when you point it at a workbench
directory, because that path exports the source workbench and reads the result back. The
clone opens, and it declares neither the level sets nor the folders.

```text
$ dinah init --from ../sample --slug clone --operator sam
clone
$ head -8 .dinah/*/workbench.md
---
format: 1
profile: dinah-core/1.0
title: Sample workbench
slug: clone
operator: sam
groups: "\"\""
states:
```

## 2. What `dinah export` prints under this card

Proposed. Dinah reads the shape of each frontmatter value rather than only the text after
the colon, so a nested mapping prints as a JSON object and a sequence prints as a JSON
array. Nothing else about the print changes.

```text
$ dinah export
{
  "groups": {
    "DESIGN": [
      "004acda2c28a",
      "7b5cfe51cb3b"
    ],
    "BUILD": [
      "68091798ab26"
    ]
  },
  "instructions": "Every card on this workbench ends with a line in the changelog.\n",
  "levels": {
    "severity": [
      "trivial",
      "minor",
      "major",
      "critical"
    ],
    "priority": [
      "later",
      "soon",
      "next",
      "now"
    ]
  },
  "profile": "dinah-core/1.0",
  "states": [
    {
      "id": "004acda2c28a",
      "kind": "intake",
      "slug": "intake",
      "title": "Intake"
    },
    {
      "id": "7b5cfe51cb3b",
      "kind": "work",
      "slug": "doing",
      "title": "Doing"
    },
    {
      "id": "68091798ab26",
      "kind": "done",
      "slug": "done",
      "title": "Done"
    }
  ],
  "title": "Sample workbench"
}
[exit 0]
```

Dinah sorts the members of the top-level object, because `encoding/json` sorts the keys of
a map, so `groups` prints before `levels` whatever order `workbench.md` declares them in.
Inside `groups`, `DESIGN` stays ahead of `BUILD`, because Dinah builds a nested mapping in
the order the file declares it. The order of the folders is the order you want them drawn,
and alphabetising it would be a change you did not ask for.

## 3. What comes back when you import that

Proposed. A clone made from the printed definition, or from the source directory, declares
both level sets and both folders. The frontmatter is not a byte-for-byte copy of the
original. The comments beside the state ids are gone, and the keys sit in the order Dinah
writes an anchor in rather than the order you typed.

```text
$ dinah init --from ../sample --slug clone --operator sam
clone
$ cat .dinah/*/workbench.md
---
format: 1
profile: dinah-core/1.0
title: Sample workbench
slug: clone
operator: sam
states:
  - 004acda2c28a
  - 7b5cfe51cb3b
  - 68091798ab26
levels:
  severity: [trivial, minor, major, critical]
  priority: [later, soon, next, now]
groups:
  DESIGN: [004acda2c28a, 7b5cfe51cb3b]
  BUILD: [68091798ab26]
---
Every card on this workbench ends with a line in the changelog.
```

The state identifiers are the source's own, which is what `Instantiate` has always done, so
the ids the `groups` folders name still resolve in the clone.

## 4. A declaration that carries hints

The format lets a level entry be a bare name or a name mapping to a one-line hint, and both
forms appear on real boards. A second throwaway workbench declares severity in the hint
form and declares no priority set:

```yaml
levels:
  severity:
    - trivial
    - minor
    - major: A person's work is wrong or blocked; fix before new work.
    - critical: Data loss or money; drop everything.
```

Proposed. Dinah prints a hinted entry as a JSON object of one member, and a bare entry as a
string, so one axis can carry both.

```text
$ dinah export
{
  "levels": {
    "severity": [
      "trivial",
      "minor",
      {
        "major": "A person's work is wrong or blocked; fix before new work."
      },
      {
        "critical": "Data loss or money; drop everything."
      }
    ]
  },
```

Proposed. Importing that writes the dashed form back, because an axis carrying a hint
cannot be written on one line. The hints survive, and so does the rank each entry takes
from its position.

```text
$ cat .dinah/*/workbench.md
---
format: 1
profile: dinah-core/1.0
title: Hinted workbench
slug: clone
operator: sam
states:
  - 004acda2c28a
levels:
  severity:
    - trivial
    - minor
    - major: A person's work is wrong or blocked; fix before new work.
    - critical: Data loss or money; drop everything.
---
```

An axis whose entries are all bare names still prints on one line, as section 3 draws it.
Dinah picks the dashed form for one axis and the one-line form for another within the same
block, which is what the format already allows an author to do.

## 5. Nothing is added to the surface

No new command appears, no flag appears, and no refusal appears. `dinah export` takes the
arguments it takes today and exits 0 where it exits 0 today. The change is in what the
printed object holds, and the operator's decision is whether the JSON shapes in sections 2
and 4 are the right ones to publish, because another tool reading a Dinah workbench will
read those shapes.
