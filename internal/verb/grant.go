package verb

import (
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The two verb names the criterion-retirement grant is given and taken back
// by. They sit beside Join and Leave in checks.go's own block of
// beyond-contract names, and they run inside Do's transaction for Join's
// reason: each writes the card's frontmatter, which is a write to the card
// like any other.
const (
	GrantPermission  = "grant"
	RevokePermission = "revoke"
)

// CriterionRetirement is the one permission name a grant carries today. It is
// an argument with a declared vocabulary rather than a verb of its own, so a
// second kind of grant can join later without renaming anything a person
// types.
const CriterionRetirement = "criterion-retirement"

// grant gives a card a standing criterion-retirement authorization, which lets
// an actor who is not the workbench operator withdraw an acceptance criterion
// of that card.
//
// The operator asked for it so that narrowing a card costs him one act rather
// than one act per criterion. On one card of this project's own workbench,
// nine acceptance criteria and four decisions stopped applying the day he
// narrowed the card, and reserving every retirement to himself would have
// brought him all thirteen one at a time.
//
// The grant is stored on one card, it carries the identifier of the column the
// card stood in when it was given, and the card's next move spends it. There
// is no workbench-level form and no form naming a set of cards, and Withdraw
// reads the grant off the item's own card, so a grant cannot reach a second
// card by any route.
//
// Granting a card that already carries a grant succeeds and rebinds it to the
// card's current column. Re-granting after a move is the intended flow, and
// refusing it would put a turnstile in front of the mechanism built to remove
// one.
func (l *Library) grant(req *Request, card *bench.Card) *Response {
	if refused := l.admitGrantVerb(req, card); refused != nil {
		return refused
	}
	card.RetirementGrant = card.Column
	ev := bench.Event{
		TS:    bench.Stamp(l.Now()),
		Event: contract.EventRetirementGranted,
		Actor: req.Acting(),
		To:    card.Column,
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	return response
}

// revoke takes back a standing criterion-retirement authorization.
//
// A card carrying no grant is refused rather than answered quietly, which is
// where this parts company with leave. Leaving a workstream a card never
// joined succeeds because the caller's intent is satisfied either way; a
// revoke naming a card with nothing standing is an operator who believes a
// permission exists, and telling him it does not is the answer he needs.
func (l *Library) revoke(req *Request, card *bench.Card) *Response {
	if refused := l.admitGrantVerb(req, card); refused != nil {
		return refused
	}
	if card.RetirementGrant == "" {
		return l.refuse(req, card, contract.NoGrant, card.Ref(l.Bench.Slug))
	}
	card.RetirementGrant = ""
	ev := bench.Event{
		TS:    bench.Stamp(l.Now()),
		Event: contract.EventRetirementRevoked,
		Actor: req.Acting(),
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	return response
}

// admitGrantVerb is the guard the two grant verbs share: the act is the
// workbench operator's alone, and the permission name has to be one the
// vocabulary declares.
//
// The operator check reads the actor alone and reads nothing about the card.
// A grant is a permission to retire an acceptance criterion, which is the
// authority this whole family of rules reserves to him, so an agent that
// could grant one to its own card would hold every permission the grant
// carries.
func (l *Library) admitGrantVerb(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if req.Actor != l.Bench.Operator {
		return l.refuse(req, card, contract.NotOperator, req.Actor)
	}
	permission := strings.TrimSpace(req.Permission)
	if permission == "" {
		return l.refuse(req, card, contract.Malformed, "permission")
	}
	if permission != CriterionRetirement {
		return l.refuseWith(req, card, contract.UnknownValue, permission, map[string]string{
			"term":  "permission",
			"field": "permission",
			"legal": CriterionRetirement,
		})
	}
	return nil
}
