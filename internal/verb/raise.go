package verb

import (
	"path/filepath"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Raise puts up the tier required at the column a card is standing in and
// hands the card back, in one act. It is what `dinah raise <ref> <tier>
// <reason...>` reaches, and it is the answer to an agent that has taken a card
// up and found the work beyond its own class: the assessment somebody made at
// the door was too low, and saying so is the system working rather than a
// failure of it.
//
// Three rules make the act what it is. The caller has to hold the card, since
// the finding is the holder's own and a bystander raising a tier is an
// ordinary assignment write rather than this. The reason is required, because
// a re-queuing with nothing behind it is indistinguishable from an agent
// avoiding work. And the resolved tier has to rank above what the card already
// requires here, so a raise can only ever go up; a deliberate downward or
// lateral correction is `set <ref> tier <value> --at <column>`, which is
// unrestricted and which this verb does not replace.
//
// The column is the card's own current column and never a flag. The card
// raises the tier for that stop, the one the holder is standing in, which is
// where the work it found too hard is waiting.
//
// The write and the hand-back are one act. The anchor carries both facts and
// is saved once, and the two journal lines are appended inside the same
// per-card lock, so no other write can land between them and the pair shares
// one timestamp. What a torn write can still leave behind is stated where the
// journal's own append is documented: an append tears at most its final line,
// so a crash between the two appends leaves the anchor already raised and
// released, with the tier_overridden line written and the released line
// absent. A reader meets a card whose journal does not say it was freed, which
// is a gap in the history rather than a half-applied write, and no state
// exists where the tier moved and the claim did not.
func (l *Library) Raise(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return l.FromError(req, err)
	}
	card := found.Card
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if card.Holder != req.Actor {
		return l.refuse(req, card, contract.NotHolder, card.Holder)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return l.refuse(req, card, contract.NoReason, "")
	}
	column := l.Bench.Column(card.Column)
	if column == nil {
		return l.refuse(req, card, contract.UnknownColumn, card.Column)
	}
	expr := strings.TrimSpace(req.Tier)
	absolute, against, refusal := l.Bench.ResolveTierWrite(column, expr)
	if refusal != nil {
		return l.refuseWith(req, card, refusal.Name, refusal.Detail, refusal.Extra)
	}
	if response := l.raisesTheRequirement(req, card, column, absolute); response != nil {
		return response
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	if l.Interleave != nil {
		l.Interleave()
	}
	// The write reloads the card under its own lock and edits the reloaded
	// value, for the reason SetCardTierAt gives: Save rewrites the whole
	// anchor from the frontmatter the caller holds, and a stale copy would
	// revert whatever landed after it was read.
	reloaded, err := l.Bench.LoadCardIn(filepath.Dir(card.Dir), card.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	ref := columnRef(column)
	was := reloaded.ColumnTierFor(l.Bench, ref)
	reloaded.SetColumnTier(l.Bench, ref, absolute)
	reloaded.State = contract.StateReady
	reloaded.Holder = ""
	reloaded.ClaimSince = ""
	reloaded.Expires = ""
	if err := reloaded.Save(); err != nil {
		return l.FromError(req, err)
	}
	raised := bench.Event{
		TS:          now,
		Event:       contract.EventTierOverridden,
		Actor:       req.Actor,
		Column:      column.ID,
		ColumnTitle: column.Title,
		From:        was,
		To:          absolute,
		Expr:        expr,
		Against:     against,
		Reason:      reason,
	}
	if err := bench.AppendEvent(reloaded.JournalPath(), raised); err != nil {
		return l.FromError(req, err)
	}
	freed := bench.Event{
		TS:    now,
		Event: contract.EventReleased,
		Actor: req.Actor,
	}
	if err := bench.AppendEvent(reloaded.JournalPath(), freed); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, reloaded)
	response.Detail = absolute
	return response
}

// raisesTheRequirement refuses a raise whose resolved tier does not rank above
// what the card already asks for at this column, and answers nil for every
// raise that does.
//
// The comparison is against the card's own requirement rather than against the
// column's tier default, because a relative expression is resolved against
// that default: a card already overridden above it can see "+1" come out at or
// below what it already requires, and comparing against the default would let
// that through as a raise that lowers.
//
// A requirement naming a tier the workbench no longer declares is skipped
// rather than treated as the lowest rank, which is the allowance claimableTier
// already makes for the identical fact. There is no rank to compare against
// and no honest refusal to make, dinah check reports the stale value, and the
// raise proceeds as though the card asked for nothing here.
func (l *Library) raisesTheRequirement(req *Request, card *bench.Card, column *bench.Column, absolute string) *Response {
	required := card.RequiredTier(l.Bench, column)
	if required == "" {
		return nil
	}
	current, declared := l.Bench.TierRank(required)
	if !declared {
		return nil
	}
	wanted, _ := l.Bench.TierRank(absolute)
	if wanted > current {
		return nil
	}
	return l.refuseWith(req, card, contract.TierNotHigher, absolute, map[string]string{
		"current":   required,
		"attempted": absolute,
	})
}
