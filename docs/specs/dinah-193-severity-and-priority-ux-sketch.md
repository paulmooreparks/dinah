# dinah-193 UX sketch: setting a card's severity and priority

Every block below is headed **Proposed** and is drawn by hand against this card's spec.
None of it is quoted from a running binary, because none of these commands exists yet and
the spec stage does not build. Column widths and refusal wording follow the shapes the
shipped commands already print, taken from `dinah workbench`, `dinah workstream` and the
query's own value refusal, so the drawing is consistent with the product rather than
measured against it. Treat a width here as intent and not as a measurement.

The scratch workbench these transcripts run in is the Dinah development board itself. It
declares both level sets in `workbench.md`:

```yaml
levels:
  severity: [trivial, minor, major, critical]
  priority: [later, soon, next, now]
```

## 1. What you can do today

You cannot record either field from the command line. `dinah add` takes a title and a
state and nothing else. No command writes a field on a card that already exists, and
`rename` reaches attachments alone. The only way to record how bad an item is, or when it
should be picked up, is to open `card.md` in an editor and type the key by hand. Dinah
preserves what you typed, because unknown frontmatter keys pass through a read and a write
untouched, and no part of the tool reads it back.

## 2. Setting both fields when you file the card

Proposed. You give `dinah add` a level for either axis, or for both, alongside the state
flag it already takes.

```text
$ dinah add --severity major --priority now "Setting a card's severity and priority"
dinah-193
```

Proposed. You leave both flags out and the card carries neither field, which is what every
card Dinah files today carries.

```text
$ dinah add "a card nobody has classified yet"
dinah-199
```

## 3. Setting a field on a card that already exists

Proposed. You name the card, the field and the level. The command follows the grammar
`dinah workbench` and `dinah workstream` already use for their own fields, so the third
entity that carries writable fields is reached the same way as the other two.

```text
$ dinah card set dinah-199 severity minor
$ echo $?
0
```

Proposed. Writing the level that is already there succeeds and writes nothing, on the
terms `join` already succeeds when the card is already a member of the workstream.

```text
$ dinah card set dinah-199 severity minor
$ echo $?
0
```

## 4. Reading one back

Proposed. `dinah card get` prints the stored level and nothing else, the way
`dinah workbench get` prints one field's value.

```text
$ dinah card get dinah-199 severity
minor
```

Proposed. A card carrying no level for the axis prints an empty line, because absence is
legal rather than an error.

```text
$ dinah card get dinah-199 priority

```

## 5. Clearing a level you set in error

Proposed. You leave the value off and Dinah removes the key from the card. This is the
grammar `config set` already uses, and it is available here because a card without a level
is a legal card. `dinah workbench set` refuses an empty value instead, because a workbench
without a title cannot be opened.

```text
$ dinah card set dinah-199 severity
$ dinah card get dinah-199 severity

```

Proposed. Clearing runs neither level check, so a level left on a card by an earlier
declaration comes off even on a workbench that declares no set for the axis. Refusing here
would leave the one card that needs clearing as the one card that cannot be cleared.

```text
$ dinah card get fx-1 severity
urgent
$ dinah card set fx-1 severity
$ dinah card get fx-1 severity

```

## 6. The three ways a write is refused

Proposed. You name a level the workbench does not declare, and Dinah lists the ones it
does, so you can pick from the answer instead of going to look for it.

```text
$ dinah card set dinah-199 severity urgent
dinah.unknown-level this workbench declares no severity level called urgent. The
severity levels it declares are: trivial, minor, major, critical. Name one of those
instead.
```

Proposed. You name a level on a workbench that declares no set for that axis, and Dinah
says so in different words, because telling you the name is unknown would be false when
every name is. The sentence names the file where the declaration belongs.

```text
$ dinah card set fx-1 severity major
dinah.no-levels this workbench declares no severity levels, so there is none to set;
declare them under levels: in /home/paul/scratch/.dinah/9f2c1a04bb31/workbench.md
```

Proposed. You name a field a card does not record, and Dinah names the ones it does. The
sentence carries the field list rather than printing a table under it, because the set
belongs to the command that refused rather than to the tool as a whole.

```text
$ dinah card set dinah-199 urgency major
dinah.unknown-field Dinah has no field urgency. The fields a card records are: severity,
priority. Name one of those instead, or run `dinah help card` to see what the command
checks.
```

Proposed. The same two level refusals answer `dinah add`, so a flag and a later write are
refused in the same words.

```text
$ dinah add --priority urgent "a card filed with a level nobody declared"
dinah.unknown-level this workbench declares no priority level called urgent. The
priority levels it declares are: later, soon, next, now. Name one of those instead.
```

## 7. A level that is already on disk and is not declared

Proposed. Reading a card whose stored level the workbench no longer declares does not
refuse. Every read tolerates it, the write path is the only place a level is validated,
and `dinah check` reports it beside the other structural findings.

```text
$ dinah check
  Path                          Finding
  ----------------------------  --------------------------------------------
  cards/4f2c19ab77e0/card.md    a card names severity urgent, which this
                                workbench does not declare
```

## 8. The help page

Proposed. `dinah help card` prints the command's syntax line, its arguments and its
ordered refusal list, generated from the same parameter table and check list every other
help page is generated from.

```text
$ dinah help card
  card <get|set> <card> <field> [value]

  Read or write one of a card's own fields.

  Argument  Meaning
  --------  ------------------------------------------------------------
  get|set   Which of the two acts to run.
  card      The card to act on, by reference or by identifier.
  field     The field to read or write: severity or priority.
  value     The level to write. Leave it out to clear the field.

  Dinah refuses in this order.

  Order  Check                                          Refusal
  -----  ---------------------------------------------  -------------------
  1      the reference names a card of this workbench    unknown-card
  2      the field is one a card records                 dinah.unknown-field
  3      the workbench declares levels for that field    dinah.no-levels
  4      the value is a level that field declares        dinah.unknown-level
  5      the request names an owner                      no-owner
```

The list is the union across both acts, which is what every generated help page prints and
what `dinah help workstream` already prints for a command covering three. Rows 1 and 2
answer `get` and `set` alike, rows 3 and 4 run only where a value is present, and row 5
runs on every write including a clear. Section 5 of the spec carries that mapping, and the
check list in `checks.go` carries it in its own comment.

## 9. The alternative surface, for the ruling at Operator Design Review

The spec is written against `dinah card get|set`. One alternative was weighed and is worth
naming, because reversing the choice after it ships means keeping a spelling nobody uses.

**Two verbs of their own**, `dinah severity <card> [level]` and
`dinah priority <card> [level]`. Fewer words to type for the two fields that exist. It
spends two top-level command names on two fields, gives no home to a third card field, and
breaks the parallel with `workbench set` and `workstream set`, which are the two commands
that already write an entity's own fields.

**What differs between them**, concretely. The proposed form costs `dinah card set dinah-199
severity minor` where the alternative costs `dinah severity dinah-199 minor`, which is one
word and eight characters. The proposed form adds one command name and one MCP tool; the
alternative adds two of each. A later card field, such as a writable title, lands as a new
value in the proposed form's `field` vocabulary and as a third top-level verb in the
alternative's.
