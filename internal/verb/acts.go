package verb

import "slices"

// OfferedActs is what may be offered for one card to the owner a request
// names: each act a real act would pass every check it runs before writing.
type OfferedActs struct {
	Claim   bool
	Release bool
	Comment bool
	// Moves are the destinations MoveDestinations answers, in flow order.
	Moves []LegalMove
}

// OfferActs answers the acts the request's owner may take on the card the
// request names. It takes no lock and writes nothing.
//
// Every answer comes from the function the act itself calls, so no row is
// written twice: canComment for the comment, admit for the rows Do runs before
// the card's lock, canClaim and canRelease for those two acts, and
// destinationsFor, through canRoute and canLand, for each move. The card's
// affordance list is read only as the upper bound on claim, release and move,
// because it is a list by state: an active card lists release whoever holds
// it, and a ready card lists claim whatever the caller's tier.
//
// A lapsed claim is cleared on the card in memory, as MoveDestinations clears
// it, so the offer answers what the act would find after the act's own lapse.
// Three refusals of a real act are left unchecked here, as they are by
// MoveDestinations: a read or write that fails, a lock another process holds,
// and a stale basis, each of which the head answers when the act meets it.
func (l *Library) OfferActs(req *Request) (*OfferedActs, error) {
	offered := &OfferedActs{}
	commentReq := *req
	commentReq.Verb = "comment"
	if _, refused := l.canComment(&commentReq); refused == nil {
		offered.Comment = true
	}
	found, refused := l.admit(req)
	if refused != nil {
		return offered, nil
	}
	card := found.Card
	if card.Lapsed(l.Now()) {
		clearLapsedClaim(card)
	}
	allowed := l.affordances(card)
	if slices.Contains(allowed, Claim) {
		claimReq := *req
		claimReq.Verb = Claim
		offered.Claim = l.canClaim(&claimReq, card) == nil
	}
	if slices.Contains(allowed, Release) {
		releaseReq := *req
		releaseReq.Verb = Release
		offered.Release = l.canRelease(&releaseReq, card) == nil
	}
	if slices.Contains(allowed, Move) {
		moves, err := l.destinationsFor(req, card)
		if err != nil {
			return nil, err
		}
		offered.Moves = moves
	}
	return offered, nil
}
