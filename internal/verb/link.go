package verb

import (
	"path/filepath"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The two verbs in this file are the write side of a card's links. A link is
// a declaration rather than an entity: it carries no identity, no journal of
// its own and no directory, so neither verb reaches for the sub-entity shape
// the checklist verbs use. They follow SetCardTierAt instead, which writes a
// declared block of the card's own frontmatter under the card's own lock and
// reloads the card fresh before the write.
//
// Both are card-owned writes, so each touches exactly one file: the source
// card's anchor. Nothing is written to the card a link names, and nothing
// computes the reverse direction, which is what the format document's
// card-owned rule fixes. A caller who wants the fact recorded from both ends
// writes two links, one on each card, in whatever kind spellings suit them.
//
// Neither verb reads or touches the claim system. A link is a comment-shaped
// act rather than a claim-shaped one, so writing or removing one on a card
// somebody else holds succeeds exactly as leaving a comment on it does.
//
// The kind is a free word. There is no permitted set anywhere here, no
// canonical spelling, and no rewriting of what the caller typed. The format
// document settles that ("The kind is an open enum... A workbench writing
// something else is conforming"), and the operator's ruling that Dinah stay
// usable for a workbench with no code, no merge and no tests is the reason it
// has to stay that way: any set chosen here would be a set built around a
// software pipeline's habits, and somebody running a renovation invents their
// own word for how two pieces of work relate. The profile's CORE-LINK-4
// drafts the identical rule, on a maturity channel whose own text says nothing
// below it binds, so it is convergent rather than the source of the rule.

// Link records one kind-and-target entry on a card's own links block.
//
// It evaluates in the order every other write verb in this package fixes: the
// workbench designates an operator, the reference names a card, the request
// names an owner, the arguments are non-empty, and the target resolves. The
// target is refused at write time when it names no card either half of the
// collection carries, which is the format document's "checked the way every
// other frontmatter reference is" read at the moment a link is offered. A
// target that resolves now and is deleted later is check's dangling-link
// finding rather than this verb's business, and archiving a target changes
// nothing, because the identifier space spans both halves.
//
// A call repeating a pair the card already carries writes nothing and
// journals nothing, on SetCardTierAt's own precedent for a write that changes
// no value.
func (l *Library) Link(req *Request) *Response {
	found, kind, to, refused := l.linkArguments(req)
	if refused != nil {
		return refused
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(found.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	// Reloaded under the lock on SetCardTierAt's own reasoning: Save rewrites
	// the whole anchor from the frontmatter the caller holds, and a copy read
	// before the lock would revert whatever landed after it.
	reloaded, err := bench.LoadCard(filepath.Dir(found.Card.Dir), found.Card.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	if indexOfLink(reloaded.Links, kind, to) >= 0 {
		return l.ok(req, reloaded)
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

// Unlink removes the one entry whose kind and resolved target match, leaving
// every other entry the card carries in the order it already held them.
//
// Removal is a delete rather than an archive. The archive-and-restore
// distinction belongs to entities that carry an identity somebody can name
// afterwards, and a link carries none, so there would be nothing to address a
// restore at. The act is still recorded, on the card's own journal, by the
// unlinked event.
//
// The target resolves by the identical rule Link's does, so a caller removes a
// link by typing the reference they created it with or the bare identifier,
// interchangeably. A pair the card does not carry is refused rather than
// treated as already done, because a caller who names the wrong kind or the
// wrong card has made a mistake worth hearing about, and nothing was removed.
func (l *Library) Unlink(req *Request) *Response {
	found, kind, to, refused := l.linkArguments(req)
	if refused != nil {
		return refused
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(found.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	reloaded, err := bench.LoadCard(filepath.Dir(found.Card.Dir), found.Card.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	index := indexOfLink(reloaded.Links, kind, to)
	if index < 0 {
		return l.refuseWith(req, reloaded, contract.UnknownLink, strings.TrimSpace(req.LinkTo), map[string]string{
			"kind": kind,
		})
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

// linkArguments runs the preconditions both verbs share and answers the card
// they act on, the kind as the caller wrote it, and the target resolved to an
// identifier. The refusal it returns is the response to give back untouched;
// a nil refusal means the other three are good.
//
// The kind is trimmed of surrounding whitespace and otherwise passes through
// exactly as typed. Nothing checks it against a set, because there is no set.
func (l *Library) linkArguments(req *Request) (*bench.Resolved, string, string, *Response) {
	if l.Bench.Operator == "" {
		return nil, "", "", l.refuse(req, nil, contract.NoOperator, "")
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return nil, "", "", l.FromError(req, err)
	}
	if req.Actor == "" {
		return nil, "", "", l.refuse(req, found.Card, contract.NoOwner, "")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		return nil, "", "", l.refuse(req, found.Card, contract.Malformed, "kind")
	}
	raw := strings.TrimSpace(req.LinkTo)
	if raw == "" {
		return nil, "", "", l.refuse(req, found.Card, contract.Malformed, "to")
	}
	to, refusal := l.Bench.ResolveLinkTarget(raw)
	if refusal != nil {
		return nil, "", "", l.refuseWith(req, found.Card, refusal.Name, refusal.Detail, refusal.Extra)
	}
	return found, kind, to, nil
}

// indexOfLink answers where a card carries the given pair, and -1 when it
// carries no such pair. Both verbs address an entry by its own content rather
// than by position, which is how SetColumnTier addresses one of its own
// block's entries, so a card carrying two links to one target under different
// kinds has each of them addressable and neither of them ambiguous.
func indexOfLink(links []bench.Link, kind, to string) int {
	for i, link := range links {
		if link.Kind == kind && link.To == to {
			return i
		}
	}
	return -1
}
