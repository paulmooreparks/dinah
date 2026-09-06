package verb

import (
	"path/filepath"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// SetCardTierAt writes one of a card's per-column tier overrides, and clears
// it when the request carries no value. It is what `dinah card set <ref> tier
// <expr> --at <column>` reaches, where the same command without --at writes
// the card's own baseline through SetCardField.
//
// It evaluates in the order SetCardField fixes for the baseline write: the
// workbench designates an operator, the reference names a card, the column the
// override is written for resolves, the expression resolves against the
// workbench's declared tier set, and the request names an owner.
//
// The column is refused at write time when it resolves to nothing, under the
// name `column new --before` already uses, because a write can catch a typo
// before it lands. A read tolerates what a write refuses: an override whose
// column is retired later resolves to nothing and dinah check reports it,
// since nothing about a card's own file can stop a reshape retiring a column
// somebody else declared.
//
// What lands on the anchor is always the absolute member name. What a person
// typed, and the column default a relative expression was measured against,
// land on the journal, which is what "recording what it resolved to and what
// it resolved against" means once storage is unambiguous.
func (l *Library) SetCardTierAt(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return l.FromError(req, err)
	}
	card := found.Card
	ref := strings.TrimSpace(req.At)
	column := l.Bench.ColumnByRef(ref)
	if column == nil {
		return l.refuse(req, card, contract.UnknownColumn, ref)
	}
	expr := strings.TrimSpace(req.Value)
	absolute, against := "", ""
	if expr != "" {
		var refusal *contract.Refusal
		absolute, against, refusal = l.Bench.ResolveTierWrite(column, expr)
		if refusal != nil {
			return l.refuseWith(req, card, refusal.Name, refusal.Detail, refusal.Extra)
		}
	}
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
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
	// The write reloads the card under its own lock and sets the one field on
	// the reloaded value, for the reason SetCardField gives: Save rewrites the
	// whole anchor from the frontmatter the caller holds, and a stale copy
	// would revert whatever landed after it was read.
	reloaded, err := bench.LoadCard(filepath.Dir(card.Dir), card.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	was := reloaded.ColumnTierFor(l.Bench, ref)
	if was == absolute {
		response := l.ok(req, nil)
		response.Detail = absolute
		return response
	}
	reloaded.SetColumnTier(l.Bench, ref, absolute)
	if err := reloaded.Save(); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:      now,
		Event:   contract.EventTierOverridden,
		Actor:   req.Actor,
		Column:  column.ID,
		From:    was,
		To:      absolute,
		Expr:    expr,
		Against: against,
	}
	if err := bench.AppendEvent(reloaded.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	response.Detail = absolute
	return response
}

// claimableTier refuses a claim declaring a tier below what the card requires
// at the column it is being taken up in, and passes every other claim.
//
// The requirement comes from the card and from nowhere else: its own per-column
// override where one names this column, its own baseline otherwise, and nothing
// at all when it declares neither. The column's own tier default is never read
// here, however high it is set, because a column default informs and never
// refuses. A station cannot make itself selective by declaring one, and the
// floor this gate enforces protects the cards somebody has assessed rather than
// the columns they pass through.
//
// A claim declaring nothing, or declaring a name the workbench does not carry,
// cannot be shown to meet a floor and is refused wherever one applies. That is
// the strict reading on purpose: a gate admitting an absent declaration would
// be a refusal any value satisfies, which is a refusal in appearance alone. It
// costs nothing where no card declares a requirement, because the gate has
// already passed by then.
//
// The comparison is a floor rather than a match. A claimant declaring more than
// the card asks for is admitted, since somebody over-qualified taking work is
// waste rather than an error, and waste is not this gate's business.
func (l *Library) claimableTier(req *Request, card *bench.Card, column *bench.Column) *Response {
	required := card.RequiredTier(l.Bench, column)
	if required == "" {
		return nil
	}
	floor, declared := l.Bench.TierRank(required)
	if !declared {
		// The card names a tier this workbench does not declare, so there is
		// no rank to compare against and no honest refusal to make. dinah
		// check reports the card, and the claim is left alone.
		return nil
	}
	if rank, ok := l.Bench.TierRank(req.Tier); ok && rank >= floor {
		return nil
	}
	// The column can be nil here, and the refusal still has to name one. A
	// card standing at a column the workbench no longer declares reaches this
	// gate with nothing resolved, exactly as operatorReservesClaim's comment
	// says it reaches that one, and dereferencing it would answer a claim
	// with a panic where a refusal is owed. So the name degrades to the
	// identifier the card itself carries, which is what dinah check prints
	// under check.unknown-column and what an operator names on the reshape
	// that repairs it. ResolveTierWrite makes the same allowance for the same
	// argument on the write side.
	ref := card.Column
	if column != nil {
		ref = columnRef(column)
	}
	return l.refuseWith(req, card, contract.BelowTier, required, map[string]string{
		"required": required,
		"column":   ref,
	})
}
