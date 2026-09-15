package verb

import (
	"path/filepath"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// SetCardTierAt writes one of a card's per-column tier overrides, and clears
// it when the request carries no value. It is what `dinah set <ref> tier
// <expr> --at <column>` reaches, where the same command without --at writes
// the card's own baseline through the generic field write SetField performs.
//
// It evaluates in the order that baseline write fixes: the
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
	// the reloaded value, for the reason writeField gives: Save rewrites the
	// whole anchor from the frontmatter the caller holds, and a stale copy
	// would revert whatever landed after it was read.
	reloaded, err := l.Bench.LoadCardIn(filepath.Dir(card.Dir), card.ID)
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
		Actor:   req.Acting(),
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

// claimableTier refuses a claim by a caller whose resolved tier is below what
// the card requires at the column it is being taken up in, and passes every
// other claim.
//
// The requirement comes from the card and from nowhere else: its own per-column
// override where one names this column, its own baseline otherwise, and nothing
// at all when it declares neither. The column's own tier default is never read
// here, however high it is set, because a column default informs and never
// refuses. A station cannot make itself selective by declaring one, and the
// floor this gate enforces protects the cards somebody has assessed rather than
// the columns they pass through.
//
// What the caller is is resolved rather than declared. The provider and the
// model the caller reported are matched against the workbench's own tiers
// table, and the rung that match sits at is what meets the floor. A caller that
// resolves to nothing cannot be shown to meet a floor and is refused wherever
// one applies. That is the strict reading on purpose: a gate admitting an
// absent declaration would be a refusal any value satisfies.
//
// Three things run in a fixed order, and the order is what makes the exemption
// reachable. The card's requirement comes first, so a card asking for nothing
// admits everybody without any of the rest being read. The workbench's table
// comes next, because a workbench that declares none has not asked for the
// gate. The operator's exemption comes after that and before either refusal,
// so the person at the terminal can still claim a tiered card while declaring
// nothing. Everybody else is admitted only by a resolved tier at or above the
// requirement.
//
// The comparison is a floor rather than a match. A claimant resolving to more
// than the card asks for is admitted, since somebody over-qualified taking work
// is waste rather than an error, and waste is not this gate's business.
func (l *Library) claimableTier(req *Request, card *bench.Card, column *bench.Column) *Response {
	// The comparison itself lives on the workbench, in TierAdmission, because
	// selection asks the same question of the same card at the same column and
	// the two answers have to agree. A card the offer shows is a card the gate
	// admits, and that holds by there being one comparison rather than by two
	// copies of it being kept in step.
	//
	// An admitted claim covers three cases the refusal never sees: the card
	// asks for nothing here, it asks for a tier this workbench does not
	// declare, which dinah check reports and no gate can honestly refuse, and
	// the caller's resolved tier clears the floor.
	resolved, _ := l.Bench.TierOf(req.Provider, req.Model, req.Server)
	admitted, required := l.Bench.TierAdmission(card, column, resolved)
	if admitted {
		return nil
	}
	// A workbench that declares no tiers block refuses no claim on tier
	// grounds, whatever its cards require and whatever any caller declares.
	// Without that rule, installing this build would lock every claim on every
	// existing workbench that declares a tier axis, because none of them has a
	// table yet and every caller would resolve to nothing. The rule is not a
	// grace period and does not expire: a table is how a workbench asks for
	// the gate, and a workbench that has not written one has not asked.
	// check.requirements-without-table is where such a workbench learns that
	// its requirements refuse nobody.
	if !l.Bench.DeclaresTierTable() {
		return nil
	}
	// The operator of the workbench is admitted at every tier whatever he
	// declares, and he is the only owner who is. The exemption opens no hole an
	// agent does not already have, because an agent that sets DINAH_ACTOR to
	// the operator's name already holds every operator-reserved act: the
	// comparison separates names rather than people.
	if req.Actor != "" && req.Actor == l.Bench.Operator {
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
	extra := map[string]string{
		"required":     required,
		"column":       ref,
		"satisfied_by": bench.RenderTierModels(l.Bench.SatisfyingModels(required)),
	}
	// The three refusals lead to three different repairs, which is why they are
	// three names rather than one. A caller that declared no model is a harness
	// nobody configured, and its sentence names the variable to set. A caller
	// the table lists nowhere is a model to add to the table or to switch away
	// from. A caller the table lists below the requirement is a model to switch
	// away from, and that one keeps the name the gate has always carried.
	declared := bench.TierModel{Provider: req.Provider, Model: req.Model, Server: req.Server}
	switch {
	case req.Provider == "" || req.Model == "":
		return l.refuseWith(req, card, contract.UndeclaredModel, required, extra)
	case resolved == "":
		extra["model"] = declared.Render()
		return l.refuseWith(req, card, contract.UnlistedModel, declared.Render(), extra)
	default:
		extra["model"] = declared.Render()
		return l.refuseWith(req, card, contract.BelowTier, required, extra)
	}
}
