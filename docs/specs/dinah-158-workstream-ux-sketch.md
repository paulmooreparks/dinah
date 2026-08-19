# dinah-158: what a workstream looks like from a terminal

Every transcript below is drawn from the spec rather than produced by a
prototype, because nothing in the tool creates a workstream yet. The tables are
laid out to the geometry the tool's own table renderer produces: two spaces of
indent, each column padded to its widest cell or its heading, two spaces between
columns, and the last column padded to nothing.

The fixture is a workbench whose slug is `ws` and whose title is `Widgets`. It
holds six cards and two workstreams. `portfolio` holds three cards, `redesign`
holds one, and one card carries an identifier that names no workstream at all.

## 1. `dinah workstream`, with no argument

You get every live workstream of this workbench.

```text
  Slug       Name              Status  Cards
  ---------  ----------------  ------  -----
  portfolio  Portfolio work    active  3
  redesign   Console redesign  paused  1
```

Cards counts the live cards that list the workstream. A workbench holding no
workstream prints one sentence instead of an empty table.

```text
this workbench carries no workstream
```

## 2. `dinah workstream new Portfolio work`

Dinah creates the workstream, derives its slug from the title, and prints what
it made.

```text
portfolio  Portfolio work  [active]
```

The line reads the way `dinah add` reads for a card. The title takes every
remaining word, so you may quote it and you need not.

## 3. `dinah workstream get portfolio`

You get the workstream's own fields, then the cards that belong to it.

```text
  Field   Value
  ------  --------------
  slug    portfolio
  id      0f1e2d3c4b5a
  title   Portfolio work
  status  active
  cards   3

  Card  Title                   State
  ----  ----------------------  ------
  ws-1  Draw the guides         Doing
  ws-4  Translate the headings  Intake
  ws-5  Count what is hidden    Intake
```

Naming a field prints that field's value alone, with no heading and no padding,
which is what `dinah workbench get title` already does.

```text
$ dinah workstream get portfolio status
active
```

## 4. `dinah workstream set portfolio status paused`

```text
portfolio  Portfolio work  [paused]
```

Dinah writes one field per call. The fields you may write are the title, the
status, and the slug. A slug change needs `--yes`, because every reference
somebody has already written down names the old one.

```text
$ dinah workstream set portfolio slug folio
dinah.unconfirmed this command needs --yes
```

## 5. `dinah join ws-2 portfolio` and `dinah leave ws-2 portfolio`

Membership belongs to the card, so the card is the subject and the two verbs sit
with the other card verbs.

```text
$ dinah join ws-2 portfolio
ws-2  Name the depth levels  [Doing / active]  portfolio

$ dinah leave ws-2 portfolio
ws-2  Name the depth levels  [Doing / active]
```

The trailing field lists the workstreams the card now belongs to, and it is
absent on a card that belongs to none. Joining a workstream the card already
belongs to succeeds and changes nothing, and so does leaving one it never
belonged to.

Naming a workstream that does not exist refuses, and the refusal lists what you
could have meant.

```text
$ dinah join ws-2 portfollio
dinah.unknown-workstream this workbench carries no workstream portfollio; run `dinah workstream` to see the workstreams this workbench carries
```

## 6. `dinah show ws-1`, where membership becomes visible

```text
ws-1  Draw the guides  [Doing / active]  portfolio, redesign
```

## 7. The generic entity commands

`archive`, `delete`, `attach`, `path`, and `edit` take an entity reference, and a
workstream names its kind in that reference. Nothing else in that grammar names
its kind, so this is the one asymmetry the spec introduces, and it exists so a
workstream and a state may share a name without either one shadowing the other.

```text
$ dinah archive workstream/redesign
archived workstream redesign

$ dinah attach workstream/portfolio ./roadmap.pdf
attached roadmap.pdf to workstream portfolio

$ dinah path workstream/portfolio
C:\work\widgets\.dinah\3f8a1c2d4e5b\workstreams\0f1e2d3c4b5a
```

Archiving a workstream is allowed while cards still belong to it, because a
finished effort is the ordinary thing to archive. Deleting one is refused while
any card still lists it.

```text
$ dinah delete workstream/portfolio --yes
dinah.referenced cards still belong to workstream portfolio
```

## 8. `dinah check`, and the identifiers that name nothing

A workbench written before this card carries membership lists whose identifiers
resolve to no workstream. Dinah reports each one and changes nothing.

```text
  a card belongs to workstream c3d4e5f60718, which resolves in neither half of the collection (C:\work\widgets\.dinah\3f8a1c2d4e5b\cards\9b2c17ff40de\card.md)
1 defect.
```

`dinah check --migrate-workstreams` adopts them. Dinah creates a real workstream
at each identifier the live cards name, keeping the identifier, so no card file
is touched and every reference already written down still resolves.

```text
Adopted 1 workstream.
  a workstream carries no slug, so it is reachable only by its identifier (C:\work\widgets\.dinah\3f8a1c2d4e5b\workstreams\c3d4e5f60718\workstream.md)
1 defect.
```

The adopted workstream carries no title, because there is no title to recover.
Give it one with `dinah workstream set c3d4e5f60718 title Old work`, then run
`dinah check --migrate-slugs` to derive its slug.

## 9. `dinah --json workstream`

```json
{
  "workstreams": [
    {
      "id": "0f1e2d3c4b5a",
      "ref": "portfolio",
      "slug": "portfolio",
      "title": "Portfolio work",
      "status": "active",
      "cards": 3
    }
  ]
}
```

`dinah --json workstream get portfolio` carries the one workstream, its notes,
and its path, in the shape `dinah --json show` already uses for a card.

```json
{
  "workstream": {
    "id": "0f1e2d3c4b5a",
    "ref": "portfolio",
    "slug": "portfolio",
    "title": "Portfolio work",
    "status": "active",
    "cards": 3
  },
  "body": "The long-form notes.\n",
  "path": "C:\\work\\widgets\\.dinah\\3f8a1c2d4e5b\\workstreams\\0f1e2d3c4b5a"
}
```

Every card the machine surface carries gains its memberships.

```json
{
  "card": {
    "id": "9b2c17ff40de",
    "ref": "ws-1",
    "title": "Draw the guides",
    "state": "4d1e77a0b3c9",
    "state_title": "Doing",
    "substate": "active",
    "holder": "paul",
    "workstreams": ["0f1e2d3c4b5a", "b7c8d9e0f102"],
    "revision": "sha256:..."
  }
}
```

The list carries identifiers rather than slugs, because it is the card's stored
frontmatter reported as it stands.

## 10. `dinah help workstream`

```text
Usage: dinah workstream [new|get|set] [workstream|title] [field] [value] [--yes]

  read this workbench's workstreams, create one, or write one's fields

Arguments:
  action            new, get, or set; omit it to list every workstream
  workstream|title  the workstream to read or write, or the title of a new one
  field             slug, title, or status
  value             what to write
  --yes             confirm a slug change

Refusals:
  dinah.unknown-workstream  no workstream of this workbench answers to that name
  dinah.unconfirmed         a slug change needs --yes
  dinah.usage               the action is not one of new, get, and set
  no-operator               this workbench designates no operator
  no-owner                  this action carries no owner

Exit codes:
  0  the command did what you asked
  2  the command refused
```
