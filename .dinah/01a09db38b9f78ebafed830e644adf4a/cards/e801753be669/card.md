---
title: Nothing adds or removes a card link, though the tool shows them
column: b69abf918c42
state: ready
severity: minor
priority: next
tier: workhorse
workstreams:
  - b3f924406e4c
  - de90dc7a5ac4
links:
  - kind: parked_behind
    to: 5ef07a3b83a3
---
The command line shows a card's links and nothing creates or deletes one, so a link is reachable only by hand-editing files. This is the half of dinah-206 that triage found did not belong with the other, and it is split out here on that finding.

Why the split. dinah-206 bundled checklist items and card links as one question about how a card's sub-entities are written, which reads well and is wrong in practice. A checklist item carries contract behaviour: a claim on a card holding an unanswered question or decision is refused. A link carries none, and dinah-32 already ruled that a tool must never refuse a verb on account of a link and must never add one as a side effect. So one half is the blocker for moving this development onto Dinah and the other is a small addition nothing waits on. Carrying them together would have taken the passenger's problems into a card the operator most needs to be right.

What makes this the smaller half. The hard question a write verb usually has to settle, what it refuses and what refuses because of it, is already answered for links by dinah-32's ruling: nothing refuses because of a link. So this card mints two verbs over an existing shape rather than deciding a contract.

What it still has to settle. What a link is between, and whether the verbs name a kind at all, since the format's own vocabulary for link kinds is the thing a caller will have to type. Whether removing a link is a delete or an archive, given how the format treats every other disappearance. And whether a link is written from either end or only one, because a link between two cards has two directions and a caller standing at one of them will expect to say so.

Read dinah-32's ruling before specifying, since its two prohibitions are the reason this card is small, and read dinah-206's own comments for what triage found when it separated the two.

## Specification

## What already exists (verified against origin/main, b825059)

The read side is already built and does not change. `bench.Card.Links` (`internal/bench/card.go:83-84`) is a slice of `Link{Kind, To string}`, read by `readLinks` (`card.go:204-235`) from a `links:` sequence in the card's own frontmatter, one mapping of `kind`/`to` per dashed entry. `dinah show` already renders it as `LinkView{Kind, To, Ref}` (`internal/verb/read.go:660-670,811-813`), where `Ref` resolves `To` to a human reference when one still applies. `dinah check` already reports a link whose target resolves nowhere, under `check.dangling-link` (`FindingDanglingLink`, `internal/bench/check.go:32,537-542`), checked against `b.HasIdentifier`, which spans both the live and archived halves of the cards collection (`bench.go:2047-2051`).

What is genuinely missing, confirmed by `grep -rln "func.*Link" --include=*.go .` (whole tree) returning only `card.go` (the reader) and `read.go` (the view): nothing writes a `links:` entry and nothing removes one. `Card.Save()` (`card.go:398-433`) writes every field the tool owns back to the anchor and does not touch `Links` at all, confirming the field is read-only in memory today. This card adds that write side: two verbs, `link` and `unlink`.

## The governing text, read fresh rather than assumed

`docs/spec/core-profile.md:684-694` (CORE-LINK-1 through CORE-LINK-6) is already merged into trunk, dinah-32 having landed before this pass. It states, in full: a card MAY carry links; every link MUST carry a kind and the identifier of the card it names; a tool MUST refuse a link naming a card its workbench does not carry, under `unknown-card`; a tool MUST NOT restrict a link's kind to a closed set; a tool MUST NOT report any of this profile's own refusal names for a claim, move, release, block or unblock refused on account of a link; a tool MUST NOT add a link to a card as a consequence of a link another card carries (no mirroring). `internal/profile/conformance_test.go:30-35` currently lists all six as `outOfReach`, each for the same reason: "v0 ships no link write-sugar." Once this card ships a write surface, that table entry is wrong on its own terms and has to come out, replaced by a real conformance test per statement, which the file's own header rule enforces: "A statement absent from both this table and the tests fails the run."

**core-profile.md's own version line matters here, and this spec reads it rather than assuming what it says.** `docs/spec/core-profile.md:3` states "Version identity: `dinah-core 0.12`, maturity channel `dev`," and section 2.1 (`core-profile.md:74-90`) defines that channel: "The text moves freely. Statements are added, reworded and withdrawn without notice, and nothing below binds." CORE-LINK-1 through CORE-LINK-6 are text on this revision, so none of the six binds a tool yet, which is exactly what `conformance_test.go`'s `outOfReach` table already says about them in its own words. This spec cites the six statements below as the drafted, convergent design direction they are, not as an obligation already in force; where a decision needs grounding beyond "the profile agrees," it is grounded in `docs/design/format.md` instead, which opens by declaring "Decisions recorded here are settled unless reopened" (`format.md:6`) and is not on a maturity channel that withholds that force.

`docs/design/format.md`'s "Card-to-card links" section (already landed, no diff needed here) says much of the same thing in prose and adds two claims the boundary-table statements do not, both of which this card must honour rather than re-litigate: a link is a declaration, not an entity, so it carries no identity, no journal of its own and no directory; and it is card-owned, so "the card carrying the link is the only file that changes when the link is added or removed."

**On first read these two documents looked like they might disagree.** Core-profile.md's CORE-LINK-3 describes a write-time refusal; format.md's own prose about the target id says only "the id a link names is checked the way every other frontmatter reference is: check reports a `to:` that resolves to no card," which reads, out of context, as though checking happens only after the fact. Reading both closed the gap rather than opening it: they answer different questions. format.md's own "checked the way every other frontmatter reference is" already covers the moment a link is offered to the tool: a frontmatter reference is checked before it is written, so a link naming an absent card refuses at write time on that reading alone. CORE-LINK-3 states the same write-time refusal as drafted, convergent dev-channel text, cited here alongside format.md's reading rather than as the source of the obligation. `check`'s dangling-link finding governs a link that was valid when written and stopped being valid later, because its target was deleted (archiving is not deletion; the id space format.md defines spans both halves, so an archived target stays resolvable). Both are true at once: `link` refuses `unknown-card` at write time, and `check.dangling-link` stays exactly as it is today for the case that arises afterward. Nothing in `check.go` changes.

## Scope note, for the record

dinah-449's own tracking description characterizes this card as "two small verbs over a documented shape, not a new contract decision," on the reasoning that dinah-32 already answers the hard question a write verb usually has to settle (nothing refuses because of a link). That holds for refusal behavior, but this card still had to settle directionality (D-2), delete-vs-archive (D-3), authority level (D-4), the write-time-refuses/check-time-catches-later reconciliation between core-profile.md and format.md (D-5), cross-half target resolution (D-6), idempotency (D-7), and self-link/duplicate-kind legality (D-8). Each is well-grounded in the existing text, but eight resolved decisions is real design surface. dinah-449's "small" framing overstates how small this card turned out to be, and the record should say so rather than let the description stand uncorrected.

## The one question this card owns: what the kind vocabulary is

format.md's "Card-to-card links" section already carries a registry-flavoured answer: "The kind is an open enum by the rule that settles the question, since no contract behavior hangs on its members. [...] carries `duplicates` and `relates` as suggested spellings." That prose is itself the settled ground, under format.md's own "Decisions recorded here are settled unless reopened" rule, and this decision has not been reopened. core-profile.md states the identical rule as CORE-LINK-4, a MUST NOT against restricting a link's kind to a closed set, but as read above that revision sits on maturity channel `dev`, where nothing yet binds; `conformance_test.go`'s `outOfReach` table lists CORE-LINK-4 among the six statements v0 does not implement, which is the same fact stated from the test side. CORE-LINK-4 is drafted and convergent with format.md's already-settled answer. It is cited here as that, not as the reason the tool must comply.

That settles the shape of the answer before this card has to invent one: `link`'s `kind` argument takes any non-empty string and refuses nothing on its spelling. It does not declare a `Vocabulary` the way `file`'s `kind` argument does (`internal/verb/definition.go:314`, which is closed because CORE-CARD's item-kind set is a closed contract enum). The two look like siblings and are opposite cases: an item's kind is enforced, so the profile closes it; a link's kind is never enforced, so format.md and the drafted profile text agree it should stay open.

The open vocabulary is also the direct answer to the operator's 2026-09-09 ruling that Dinah must stay usable for a workbench with no code, no merge and no tests. Andoneer's own link kinds (`blocks`, `relates_to`, `supersedes`, `parked_behind`) are not adopted here, not because they are wrong, but because adopting any fixed set at all, Andoneer's or a fresh one, would be the mistake: a person running a kitchen renovation who wants to say "the tile order duplicates the one I already placed," or "the electrical inspection blocks the drywall," or "this punch-list item came out of that walkthrough," is typing a word that means something to them, and the tool's job is to hold what they typed, not to make them pick from a list built for a software pipeline's habits. A workbench that wants Andoneer's own four spellings gets them by writing them, the same as any other kind; nothing here maps one vocabulary onto another, because there is no closed vocabulary on either side of the tool. Recorded as D-1 below.

## Verbs

Two new library methods, following `File`/`Cite` (`internal/verb/checklist.go:40-121`) and `SetCardTierAt` (`internal/verb/tier.go:32-96`) exactly, because a link is card-owned data written under the card's own lock and reloaded fresh before the write, the same shape `tier_at` already uses, rather than a sub-entity with its own directory and lock, the shape checklist items use.

```
dinah link <card> <kind> <to>
dinah unlink <card> <kind> <to>
```

Both are three required positional arguments, `bounded: 3`, following `cite`'s table shape (`definition.go:319-323`, `commands.go:57`). New `Request` fields are unnecessary: `Card` (existing, `Field: "Card"`), `Kind` (existing, reused exactly as `file` reuses it), and one new field, `LinkTo string`, for the third argument (`CiteTarget` is not reused, since a link's target is a card reference and a citation's target is not, and giving them separate fields keeps a future change to either from touching the other).

```go
"link": {
    {Name: "card", Required: true, Shared: "card", Field: "Card"},
    {Name: "kind", Required: true, Field: "Kind"},
    {Name: "to", Required: true, Field: "LinkTo"},
},
"unlink": {
    {Name: "card", Required: true, Shared: "card", Field: "Card"},
    {Name: "kind", Required: true, Field: "Kind"},
    {Name: "to", Required: true, Field: "LinkTo"},
},
```

Note `kind` here declares no `Vocabulary`, unlike `file`'s. That absence is the load-bearing detail of this whole card, settled by format.md's already-binding prose and the operator's non-software-workbench ruling, and convergent with CORE-LINK-4's drafted text: do not add a `Vocabulary` here later without reopening those grounds.

Wiring: one entry each in `cmd/dinah/commands.go`'s command table (`group: groupWork, run: runLink / runUnlink, bounded: 3`), one dispatch function each following `runCite`'s shape, and one roster entry each in `internal/mcp/tools.go` (`{name: "link_card", command: "link", run: ...}`, `{name: "unlink_card", command: "unlink", ...}`), matching the three-point wiring every other verb in this file already shows. `TestEveryLibraryCommandIsDispatchedOrExempted` and its MCP twin fail the build, not a review, if either point is missed.

### `resolveLinkTarget`: how `to` is read, shared by both verbs

A caller types whatever they'd type to reference the target card anywhere else: the 12-hex identifier or the dash-slug-number human reference. The stored value is always the resolved identifier, because format.md's own example (`links: - kind: duplicates \n to: 4f2c19ab77e0`) shows an id in that slot, not a reference. CORE-LINK-2 states the same rule ("the identifier of the card it names") as drafted, convergent dev-channel text, cited here alongside format.md's example rather than as its source.

```go
// resolveLinkTarget resolves what a caller typed for a link's target into the
// 12-hex identifier the anchor stores, checking both halves of the collection
// the way HasIdentifier and the dangling-link check already do, since a link
// may legally name a card that is now archived.
func (b *Bench) resolveLinkTarget(raw string) (string, *contract.Refusal) {
	raw = strings.TrimSpace(raw)
	if bench.IsID(raw) {
		if !b.HasIdentifier(raw) {
			return "", &contract.Refusal{Name: contract.UnknownCard, Detail: raw}
		}
		return raw, nil
	}
	if found, err := b.ResolveCard(raw); err == nil {
		return found.Card.ID, nil
	}
	if found, err := b.ResolveArchivedCard(raw); err == nil {
		return found.Card.ID, nil
	}
	return "", &contract.Refusal{Name: contract.UnknownCard, Detail: raw}
}
```

This is the one place the two halves of the collection are tried in sequence for a reference (not just an id) at write time; nothing existing needed this before, because every other write verb resolves a reference that must be live (you cannot comment on, attach to, or claim an archived card). A link is the first write-time reference allowed to name either half, which is exactly the case format.md's own text calls out ("the id space spans the live and archived halves"), so a helper is worth minting rather than inlining twice.

### `link`

```go
func (l *Library) Link(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, found.Card, contract.NoOwner, "")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		return l.refuse(req, found.Card, contract.Malformed, "kind")
	}
	rawTo := strings.TrimSpace(req.LinkTo)
	if rawTo == "" {
		return l.refuse(req, found.Card, contract.Malformed, "to")
	}
	to, refusal := l.Bench.resolveLinkTarget(rawTo)
	if refusal != nil {
		return l.refuseWith(req, found.Card, refusal.Name, refusal.Detail, nil)
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(found.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	// Reloaded under the lock, on SetCardTierAt's own reasoning: Save
	// rewrites the whole anchor from the frontmatter the caller holds, and a
	// copy read before the lock would revert whatever landed after it.
	reloaded, err := bench.LoadCard(l.Bench.CardsRoot(), found.Card.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	for _, existing := range reloaded.Links {
		if existing.Kind == kind && existing.To == to {
			// Idempotent: the pair is already there, so nothing is written
			// and nothing is journalled, on SetCardTierAt's own "was ==
			// absolute" precedent for a write that changes nothing.
			return l.ok(req, reloaded)
		}
	}
	reloaded.Links = append(reloaded.Links, bench.Link{Kind: kind, To: to})
	if err := reloaded.Save(); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{TS: now, Event: contract.EventLinked, Actor: req.Actor, Kind: kind, To: to}
	if err := bench.AppendEvent(reloaded.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	return l.ok(req, reloaded)
}
```

### `unlink`

```go
func (l *Library) Unlink(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, found.Card, contract.NoOwner, "")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		return l.refuse(req, found.Card, contract.Malformed, "kind")
	}
	rawTo := strings.TrimSpace(req.LinkTo)
	if rawTo == "" {
		return l.refuse(req, found.Card, contract.Malformed, "to")
	}
	// unlink resolves `to` by the identical rule link does, so a caller may
	// remove a link by typing the same reference they created it with, or
	// the bare identifier, interchangeably.
	to, refusal := l.Bench.resolveLinkTarget(rawTo)
	if refusal != nil {
		return l.refuseWith(req, found.Card, refusal.Name, refusal.Detail, nil)
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(found.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	reloaded, err := bench.LoadCard(l.Bench.CardsRoot(), found.Card.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	index := -1
	for i, existing := range reloaded.Links {
		if existing.Kind == kind && existing.To == to {
			index = i
			break
		}
	}
	if index == -1 {
		return l.refuseWith(req, reloaded, contract.UnknownLink, rawTo, map[string]string{"kind": kind, "to": to})
	}
	reloaded.Links = append(reloaded.Links[:index], reloaded.Links[index+1:]...)
	if err := reloaded.Save(); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{TS: now, Event: contract.EventUnlinked, Actor: req.Actor, Kind: kind, To: to}
	if err := bench.AppendEvent(reloaded.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	return l.ok(req, reloaded)
}
```

`unlink` removes exactly the one entry whose kind and resolved target match, leaving the rest in their original order; a card is free to carry two links to the same target under different kinds, or a self-link (a card naming itself), and removal is scoped to the exact pair for the same reason `SetColumnTier`'s removal is scoped to one column: a block-shaped store where each entry is addressed by its own content rather than by position.

## `Card.Save` gains one more block

`Save()` (`card.go:398-433`) needs a fourth declared-block clause beside the `tier_at` one it already has:

```go
if len(c.Links) == 0 {
    c.FM.Delete("links")
} else {
    c.FM.SetRaw("links", renderLinks(c.Links))
}
```

`renderLinks` mirrors `renderColumnTiers` (`card.go:276-284`) exactly: one `- kind: <kind>` / `  to: <to>` pair per entry, in the order the slice holds them, through `quote()` for the same reasons `renderColumnTiers` already quotes.

## Directionality: a link is one-way, and the tool never computes the other end

The design question the card poses ("a card that blocks another and a card blocked by it are the same fact seen twice") is already answered by format.md's own "card-owned" language, convergent with CORE-LINK-6's drafted, dev-channel text, and this card does not reopen it: a link is recorded on the card that carries it, `link`/`unlink` touch only that one card's file (AC-9 below), and nothing renders, infers or writes a reverse entry on the named card. `dinah show` on the target card shows nothing about being named by another card's link, exactly as it shows nothing today; that is not a gap this card leaves, it is the shape format.md's own "card-owned" prose independently settles, convergent with what CORE-LINK-6 drafts. A caller who wants both cards to carry the fact writes two `link` calls, one on each card, in whatever kind spellings suit them (`blocks` on one, `blocked_by` on the other, or the same kind both ways if the relation reads the same from either end, such as `relates`). This is recorded as D-2.

## Compatibility

No profile revision changes. Both the frontmatter shape and CORE-LINK-1 through CORE-LINK-6 are already declared at the profile's current revision; this card is implementation catching up to a standing declaration, exactly as dinah-206's compatibility section reasoned for checklist items. `internal/bench/testdata/compat/dinah-core-{0.4,0.5,0.6,0.7,0.9,0.12,1.0,1.0-pre-slug}` carry no `links:` block in any fixture card (confirmed: `grep -rln "^links:" internal/bench/testdata/compat` over the whole compat tree returns nothing); reading one continues to answer an empty `Links` slice as it does today. One new compat-suite case is added: open each existing fixture workbench through this build, `link` a card in it, `unlink` the same pair, and confirm the resulting anchor is byte-identical to the pre-link anchor except for the round-tripped `links:` presence and absence, proving neither verb needs a card re-migrated to act on it.

## Journal, message catalog and conformance bookkeeping (for the implementer)

- Two new journal event names, `EventLinked = "linked"` and `EventUnlinked = "unlinked"`, join the closed block beside `EventItemFiled` etc. in `internal/contract/contract.go` (not under `LayerPrefix`: a link is format-declared, exactly as the comment beside the checklist events already reasons), each appended to the `Events` list.
- One new refusal name, `dinah.unknown-link` (`UnknownLink = LayerPrefix + "unknown-link"`), for `unlink` naming a pair the card does not carry. This one does carry `LayerPrefix`, unlike the events: removal is Dinah's own invention, since format.md and core-profile.md say nothing about an unlink verb or its refusals, so it is not a declared-shape name the way `unknown-card` and `malformed` are.
- `en.json` gains an entry for `dinah.unknown-link` (the `{name}` text plus a context sentence), which `TestEveryKeyCarriesAContext` requires; the other locales fall back safely per `TestMissingKeysFallBackPerKey`, and per the workbench's translation-staleness contract (document id 51) the implementer should read that contract before deciding whether to leave them stale on purpose.
- `internal/profile/conformance_test.go`'s `outOfReach` table loses its six `CORE-LINK-*` rows (`conformance_test.go:30-35`), and the package gains one test per statement (`TestCoreLink1` through `TestCoreLink6`, or one test covering the row's own written property, matching the shape neighbouring `TestCore*` functions already use for a MAY/MUST/MUST NOT statement), each driven by the property the row's own table entry names (`format.md:1457-1462`), not by a hand-picked example.

## Out of scope, restated so it isn't re-litigated

- Any change to `dinah show`'s link rendering, `LinkView`, or `check.dangling-link`: all three already exist and this card does not touch them.
- A closed or mapped link-kind vocabulary of any shape, Andoneer's or a fresh one: ruled out by format.md's settled prose, the operator's non-software-workbench ruling, and D-1.
- Computing, storing or rendering a link's reverse direction: ruled out by format.md's "card-owned" prose (convergent with CORE-LINK-6's drafted, dev-channel text) and D-2.
- Any refusal of a claim, move, release, block or unblock on account of a link: ruled out by format.md's own "no verb refuses because of one" prose (convergent with CORE-LINK-5's drafted, dev-channel text), unchanged by this card.
- A registry file or completion mechanism for suggested kind spellings: format.md's own "token registry" section describes this as a future artifact feeding four consumers; nothing in this codebase builds it today (confirmed: no `registry.json` and no `TokenRegistry` type anywhere in `internal` or `docs`), and this card does not start it. `dinah link --help` names `duplicates` and `relates` as examples in prose, not as an enforced or completed set.

## Decisions recorded

- **D-1 (kind stays fully open; no mapping onto Andoneer's or any other closed vocabulary is built).** format.md's own prose settles this independently ("The kind is an open enum... A workbench writing something else is conforming"), under format.md's own "Decisions recorded here are settled unless reopened" rule, and the operator's 2026-09-09 ruling that Dinah must stay usable for a workbench with no code, no merge and no tests backs it further. core-profile.md states the same rule as CORE-LINK-4, but that revision sits on maturity channel `dev`, where section 2.1 says nothing below it binds yet, and `conformance_test.go` still lists CORE-LINK-4 as `outOfReach` for the same reason. CORE-LINK-4 is cited here as drafted and convergent with the settled answer, not as the source of the obligation; not an operator call, since the operator's own ruling is already what grounds it.
- **D-2 (a link is one-way; the tool never writes or infers the reverse).** format.md's "card-owned" language settles this in force today; CORE-LINK-6 (no mirroring) states the same rule as drafted, convergent text. A caller wanting the pair writes two links. Not an operator call.
- **D-3 (removing a link is a delete, not an archive).** A link carries no identity and no journal of its own (format.md), so the entity-level archive/delete distinction that exists for cards, comments and attachments has nothing to attach to here; `unlink` removes the frontmatter entry outright, and the removal is still recorded, on the card's own journal, by `EventUnlinked`.
- **D-4 (`unlink` needs no more authority than `link`).** Both are comment-shaped writes under the card's own lock, exactly as `Comment`, `Attach` and the checklist verbs already are; nothing in format.md asks for a special permission tier over removing a link. CORE-LINK-5 (no refusal keyed to a link's presence) states the same rule as drafted, dev-channel text, cited here as convergent with format.md's design rather than as a rule already in force.
- **D-5 (a link naming an absent card refuses at write time; a link naming a later-archived card stays valid).** format.md's own prose for the first half ("the id a link names is checked the way every other frontmatter reference is") plus "the id space spans the live and archived halves" and the untouched `check.dangling-link` finding for the second; CORE-LINK-3 states the write-time half as drafted, convergent text.
- **D-6 (`to` resolves against both halves of the collection and stores the resolved identifier, never the caller's spelling).** format.md's own YAML example stores an id; `resolveLinkTarget` above is the mechanism.
- **D-7 (`link` is idempotent on an identical existing pair).** No duplicate entry, no duplicate journal event, mirroring `SetCardTierAt`'s own "nothing changed, nothing written" precedent.
- **D-8 (a self-link, and two links to the same target under different kinds, are both legal).** Nothing in format.md or the drafted CORE-LINK text gives grounds to refuse either, and inventing a refusal the contract does not ask for would itself be the kind of unrequested enforcement format.md's no-consultation design rules out. CORE-LINK-1 through CORE-LINK-6 draft the same absence of grounds in the profile's own boundary table, on the profile's dev channel, where nothing yet binds; cited here as convergent drafted text, not as a ruling already in force.

No item on this card is filed as an operator-owned open question. The one question the description hands to this card, whether Dinah should offer a closed link-kind set at all, is already closed by format.md's settled prose and the operator's own 2026-09-09 ruling, so there is nothing left for the operator to rule on beyond the ordinary review of the CLI surface itself, which Operator Design Review already does for every new verb.

## Command transcript, for Operator Design Review

```
$ dinah link dinah-436 spawned_from dinah-206
ok: dinah-436 now carries a link (spawned_from -> dinah-206)

$ dinah link dinah-436 spawned_from dinah-206
ok: dinah-436 already carries that link (spawned_from -> dinah-206)

$ dinah link dinah-436 blocks 000000000000
refused: unknown-card (000000000000)

$ dinah show dinah-436 --fields links
links:
  - kind: spawned_from
    to: dinah-206

$ dinah unlink dinah-436 spawned_from dinah-206
ok: dinah-436 no longer carries that link (spawned_from -> dinah-206)

$ dinah unlink dinah-436 spawned_from dinah-206
refused: dinah.unknown-link (dinah-206): dinah-436 carries no spawned_from link to dinah-206
```

The one-line criterion this transcript asks the operator to approve: two new verbs, `link` and `unlink`, each taking a card, a kind (any word, never a closed list) and a target card, writing or removing one entry on the source card's own file and nothing else.

## Branch

dinah-436-nothing-adds-or-removes-a-card-link-though-the-tool-shows-them
