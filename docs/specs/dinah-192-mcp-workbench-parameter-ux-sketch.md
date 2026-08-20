# dinah-192 UX sketch: one MCP head, many workbenches

Two kinds of block appear below. A block headed **Today** came out of a binary built from
commit `f065620`, the current head of `main`, and it is quoted verbatim. A block headed
**Proposed** came out of a throwaway binary built from that same commit with this card's
changes sketched into it, and it is also quoted verbatim, so the schema shapes, the
handshake wording, and the help page below are what a renderer produced rather than what a
drawing guessed at. The throwaway build was thrown away; it is not part of this branch.

Every command below was run in PowerShell on Windows, and the paths are Windows paths for
that reason. Nothing here was run against the operator's own workbenches, and no block
below names a directory of his. The scratch tree sits at `C:\dinah-scratch\dinah-192-spec2`,
outside his profile, and `DINAH_HOME`, `TMP`, and `TEMP` all pointed inside it for every
run.

The scratch tree holds four workbenches, every one of them created by running `dinah init`
in a directory this card made. Two of them lie under the root the server below is started
with, at `benchroot\plans` and `benchroot\plans-archive`. The third lies outside that root
at `outside\out-of-reach`, and its `workbench.md` was then overwritten with a line that does
not parse, which is what section 6 uses to show that a refused call never opened it. The
fourth lies outside the root at `outside\good` and is left intact. None of the four holds
any card, which is why the listings below are short.

The two workbenches under the root are named so that the order the walk meets them differs
from the order their paths sort in. A directory listing gives `plans` before
`plans-archive`, and a byte-wise sort of the two workbench paths gives `plans-archive`
first, because the hyphen sorts ahead of the path separator. Section 3's listing is
therefore evidence about the ordering rule rather than about one arrangement that happened
to agree with it.

The blocks in this document are not read by any guard. `guardedDocuments` in
`cmd/dinah/guide_guard_test.go` reads the embedded guides and the quick start, and a
document under `docs/specs/` reaches none of those checks. Nothing here is replayed, so
treat it as a drawing that happens to be accurate on the day it was written rather than as
text the build holds.

## 1. What an agent sees today

Today. The handshake names one workbench and offers no way to reach another.

```text
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
```

```text
You are working the workbench plans.

The working agreement, which binds you rather than the tool:
1. Claim a card before producing work on it.
2. Do not hold a claim on a card you have stopped working.
3. Treat the workbench as the authority for where a card stands and who holds it.
4. Do not move a card out of an operator-owned state unless you are the operator.

Every response carries an affordances member naming what you may do next. A successful claim or move carries the instructions of the position in three separate layers and the moves the flow allows. Tokens are canonical on this surface and are never translated.

The operator of this workbench is paul. Read the embedded guides through resources/list and resources/read.
```

Today. The surface carries twenty-six tools, and every input schema carries the two
properties `schemaFor` injects beyond the command's own parameters.

```json
{
  "description": "take up a ready card",
  "inputSchema": {
    "properties": {
      "actor": {
        "description": "act as this owner",
        "type": "string"
      },
      "basis": {
        "description": "Dinah computes a basis for every request that changes something, and nothing here offers a way to write one, so leave this property alone.",
        "type": "string"
      },
      "card": {
        "description": "the card you are taking up",
        "type": "string"
      },
      "expires": {
        "description": "how long your claim holds before it goes stale, written as a number and a unit: 30m, 2h, 7d",
        "type": "string"
      }
    },
    "required": [
      "card"
    ],
    "type": "object"
  },
  "name": "claim"
}
```

## 2. The third injected property

Proposed. Every tool but `workbenches` gains a `workbench` property beside `actor` and
`basis`. The property is optional, so a client that never sends it keeps getting the
workbench the server started with.

```json
{
  "description": "take up a ready card",
  "inputSchema": {
    "properties": {
      "actor": {
        "description": "act as this owner",
        "type": "string"
      },
      "basis": {
        "description": "Dinah computes a basis for every request that changes something, and nothing here offers a way to write one, so leave this property alone.",
        "type": "string"
      },
      "card": {
        "description": "the card you are taking up",
        "type": "string"
      },
      "expires": {
        "description": "how long your claim holds before it goes stale, written as a number and a unit: 30m, 2h, 7d",
        "type": "string"
      },
      "workbench": {
        "description": "which workbench this call acts on, named by the path the workbenches tool lists; the workbench this server started with when you name none",
        "type": "string"
      }
    },
    "required": [
      "card"
    ],
    "type": "object"
  },
  "name": "claim"
}
```

Proposed. A tool taking no parameters of its own shows the same three.

```json
{
  "description": "where this workbench stands, and what you hold",
  "inputSchema": {
    "properties": {
      "actor": {
        "description": "act as this owner",
        "type": "string"
      },
      "basis": {
        "description": "Dinah computes a basis for every request that changes something, and nothing here offers a way to write one, so leave this property alone.",
        "type": "string"
      },
      "workbench": {
        "description": "which workbench this call acts on, named by the path the workbenches tool lists; the workbench this server started with when you name none",
        "type": "string"
      }
    },
    "type": "object"
  },
  "name": "status"
}
```

The rule behind that injection is uniformity with one exception rather than a principle with
one instance. `version` acts on no workbench and gets the property anyway, because
`schemaFor` injects it everywhere and one injection site is cheaper to hold true than a list
of tools that need it. `workbenches` is the one tool held out, and it is held out because a
caller answering the question of what the property may say cannot already know the answer.

## 3. The discovery tool

Proposed. The surface grows to twenty-seven tools. The new one is `workbenches`, and it is
the only tool that carries no `workbench` property.

```json
{
  "description": "the workbenches this server may serve",
  "inputSchema": {
    "properties": {
      "actor": {
        "description": "act as this owner",
        "type": "string"
      },
      "basis": {
        "description": "Dinah computes a basis for every request that changes something, and nothing here offers a way to write one, so leave this property alone.",
        "type": "string"
      }
    },
    "type": "object"
  },
  "name": "workbenches"
}
```

Proposed. Calling it answers with the same three fields the terminal listing prints, wrapped
the way every other read is wrapped, and ordered by path.

```json
{
  "affordances": [
    "status",
    "states",
    "list_cards",
    "next_card"
  ],
  "workbenches": [
    {
      "title": "plans-archive",
      "slug": "pa",
      "path": "C:\\dinah-scratch\\dinah-192-spec2\\benchroot\\plans-archive\\.dinah\\793d2e263ee6"
    },
    {
      "title": "plans",
      "slug": "pl",
      "path": "C:\\dinah-scratch\\dinah-192-spec2\\benchroot\\plans\\.dinah\\5164831b5ee4"
    }
  ]
}
```

`plans-archive` comes back first although the walk met `plans` first, which is the ordering
rule doing visible work. An implementation that emitted rows in walk order would answer with
these two the other way round.

Today. The terminal listing answers a different question with the same row shape. It climbs
from where it is run rather than descending from a root, so run inside `plans` it finds that
one workbench and does not find `plans-archive` at all.

```text
$ dinah workbenches
  Workbench  Slug  Path
  ---------  ----  -------------------------------------------------------------
  plans      pl    C:\dinah-scratch\dinah-192-spec2\benchroot\plans\.dinah\5164831b5ee4
```

## 4. The handshake

Proposed. A server that discovered a workbench at startup keeps today's opening line word
for word and gains a paragraph naming its reach.

```text
You are working the workbench plans.

This server also reaches the other workbenches under C:\dinah-scratch\dinah-192-spec2\benchroot. Every tool takes an optional workbench property, and a call that names none acts on plans. Call the workbenches tool to see what you may name.

The working agreement, which binds you rather than the tool:
1. Claim a card before producing work on it.
2. Do not hold a claim on a card you have stopped working.
3. Treat the workbench as the authority for where a card stands and who holds it.
4. Do not move a card out of an operator-owned state unless you are the operator.

Every response carries an affordances member naming what you may do next. A successful claim or move carries the instructions of the position in three separate layers and the moves the flow allows. Tokens are canonical on this surface and are never translated.

The operator of this workbench is paul. Read the embedded guides through resources/list and resources/read.
```

Proposed. A server started with a root but standing in no workbench of its own drops the
opening line and the operator line, since neither has a subject any more. The block below
came from a server launched inside `outside\good`, which is a workbench that lies outside
the root, so this is also what startup case 3 of the spec produces.

```text
This server serves the workbenches under C:\dinah-scratch\dinah-192-spec2\benchroot, and it started in none of them. Every tool takes a workbench property, and a call that names none is refused with dinah.no-workbench-found. Call the workbenches tool to see what you may name.

The working agreement, which binds you rather than the tool:
1. Claim a card before producing work on it.
2. Do not hold a claim on a card you have stopped working.
3. Treat the workbench as the authority for where a card stands and who holds it.
4. Do not move a card out of an operator-owned state unless you are the operator.

Every response carries an affordances member naming what you may do next. A successful claim or move carries the instructions of the position in three separate layers and the moves the flow allows. Tokens are canonical on this surface and are never translated.

Read the embedded guides through resources/list and resources/read.
```

## 5. The startup surface

Proposed. `dinah mcp` gains one flag, and its page is generated from the same parameter list
and the same check list every other page is generated from.

```text
$ dinah help mcp
mcp [--root <dir>]

serve workbenches over MCP on stdio

What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  [--root <dir>]   the directory whose workbenches this server may serve; the
                   workbench Dinah discovers at startup when you name none

What can go wrong, in the order each is checked:
  Order  What can go wrong                     Refusal
  -----  ------------------------------------  ------------------
  1      --root names a directory that exists  dinah.unknown-root
  2      the workbench --workbench or DINAH_WORKBENCH names lies under the root
                                               dinah.outside-root

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
```

Row 2 of the refusal table prints on two lines, and that is the renderer obeying its own
rule rather than a fault. `chooseWidths` in `cmd/dinah/table.go` leaves a field the window
cannot hold out of the measurement, so the middle column measures at 36 columns, which is
row 1's exact width, and the row renderer gives the oversized field the rest of its line and
starts `dinah.outside-root` at column 47 beneath it. The spec keeps the check sentence at
its full length rather than cutting it to fit, because the same sentence is longer again in
seven of the eight locales and shortening the English would buy nothing for the other seven.

Proposed. The ratified help block gains the flag on its `mcp` line and the variable on its
environment line, and nothing else on the page moves.

```text
SERVE
  mcp [--root <dir>]                     serve workbenches over MCP on stdio

Global flags:
  Option             What it does
  -----------------  -----------------------------------------------------------
  --workbench <dir>  use this workbench instead of the one discovered from here
  --json             emit the canonical machine form
  --quiet            suppress served instructions on claim and move
  --lang <tag>       render in this language; run `dinah version --catalogs` for the tags
  --actor <name>     act as this owner

Environment: DINAH_WORKBENCH, DINAH_MCP_ROOT, DINAH_HOME, DINAH_FORMAT=json, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, VISUAL, EDITOR
```

## 6. Naming a workbench, and being refused one

The three answers below carry a `states` member listing every state of the workbench, and
that member is cut from the first two for length. Everything else in the three blocks is
verbatim.

Proposed. A call omitting the property acts on the workbench the server started with.

```json
{
  "affordances": [
    "status",
    "states",
    "list_cards",
    "next_card"
  ],
  "status": {
    "workbench": "plans",
    "root": "C:\\dinah-scratch\\dinah-192-spec2\\benchroot\\plans\\.dinah\\5164831b5ee4",
    "is_operator": false,
    "operator": "paul",
    "profile": "dinah-core/0.4",
    "holding": [],
    "blocked": []
  }
}
```

Proposed. A call naming the other workbench under the root acts on that one instead.

```json
{
  "affordances": [
    "status",
    "states",
    "list_cards",
    "next_card"
  ],
  "status": {
    "workbench": "plans-archive",
    "root": "C:\\dinah-scratch\\dinah-192-spec2\\benchroot\\plans-archive\\.dinah\\793d2e263ee6",
    "is_operator": false,
    "operator": "paul",
    "profile": "dinah-core/0.4",
    "holding": [],
    "blocked": []
  }
}
```

Proposed. A call naming a workbench outside the root is refused, and the refusal names the
root it was measured against.

```json
{
  "outcome": "refused",
  "verb": "status",
  "refusal": "dinah.outside-root",
  "detail": "C:\\dinah-scratch\\dinah-192-spec2\\outside\\out-of-reach\\.dinah\\7fad035688b1",
  "affordances": [
    "status",
    "states",
    "list_cards",
    "next_card"
  ],
  "context": {
    "root": "C:\\dinah-scratch\\dinah-192-spec2\\benchroot"
  }
}
```

That refusal names a scratch workbench this card created, and it is the one whose
`workbench.md` was overwritten with a line that does not parse. The next block is what the
same binary answers when the same directory is opened, so the two together are the evidence
that the bound ran ahead of the open rather than behind it. A head that checked the bound
after opening would have answered `malformed` instead.

```text
$ dinah --workbench C:\dinah-scratch\dinah-192-spec2\outside\out-of-reach\.dinah\7fad035688b1 status
malformed title is missing, empty, or will not parse, in C:\dinah-scratch\dinah-192-spec2\outside\out-of-reach\.dinah\7fad035688b1\workbench.md; hand-edit the file and add it, then run `dinah check` to confirm
```

The `affordances` member of the refusal carries the head's own read affordances rather than
the library's verb names, because no library was opened to ask.

An MCP refusal carries the name, the detail, and the named values, and it carries no
sentence. The English a refusal name owns is rendered by the cli head in `composeRefusal`
and reaches a person at a terminal, so nothing in the block above is translated or
translatable, and a client that wants a sentence looks the name up itself.

## 7. The catalog copy the two new refusals need

The sentences below are proposed text rather than captured output, because a refusal name
minted by this card has no catalog entry to print yet. Both are drafted from one
understanding of what the reader needs, so each names the path, names the root, and gives a
next step. Both are read at a terminal by whoever launched the server, which is the only
place either one renders.

| Key | English text |
|---|---|
| `refusal.dinah.unknown-root` | `{detail} was named as the root for dinah mcp, and no directory sits there` |
| `refusal.dinah.unknown-root.next` | `; point --root or DINAH_MCP_ROOT at a directory that exists` |
| `refusal.dinah.outside-root` | `{detail} lies outside {root}, which is the whole of what this server may serve` |
| `refusal.dinah.outside-root.next` | `; run the workbenches tool to see what it may serve, or restart it with a root that holds this one` |

The two check sentences the generated help page prints are proposed text as well, and they
are drawn in section 5 rather than here because the page is where a reader meets them.

| Key | English text |
|---|---|
| `check.mcp.1` | `--root names a directory that exists` |
| `check.mcp.2` | `the workbench --workbench or DINAH_WORKBENCH names lies under the root` |

## 8. What the operator is being asked to accept

Each item below is hard to change once a client has been registered against it, which is why
it is drawn here rather than settled at Implement.

The property is named `workbench`, it is optional on every tool but `workbenches`, and its
value is a path. Section 2 shows the schema and section 6 shows a call that uses it. The
spec leaves one question open beside this: whether the property should also accept a
workbench's slug, which is shorter for an agent to carry and ambiguous when two workbenches
under one root share it.

The discovery tool is named `workbenches` and returns title, slug, and path, ordered by
path. Section 3 shows it, and shows the ordering changing the answer. It never names a
workbench outside the root and never opens one to describe it, so no card, no operator name,
and no state list travels in that answer.

The startup surface is `--root <dir>` on `dinah mcp` plus `DINAH_MCP_ROOT`, with the flag
winning. Section 5 shows the page and the two lines of the ratified block that move, and it
shows the second refusal row printing on two lines.

The handshake gains a paragraph and loses its opening line when there is no default
workbench. Section 4 shows both forms.

The English of the two new refusals and the two new help-page checks is section 7's four
plus two rows. Approving them fixes the sentences the eight locale files are translated
from.
