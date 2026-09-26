package verb

import "dinah/internal/bench"

// MoveDestinations reports the rows of the card's legal moves that a move
// request would pass every check on, for the request's actor and override.
// It takes no lock, writes nothing, lapses no claim on disk and witnesses no
// divergence. Wherever Do would refuse every move, it answers nothing.
//
// It calls the two functions a real move is refused by. admit runs the rows
// Do runs before the card's lock, and any of them refusing refuses every
// destination alike, so it answers nothing. canMove runs every row the move
// itself runs, once for each destination. Between the two, Do lapses an
// expired claim, which this clears on the card in memory rather than on disk,
// and witnesses a divergence, which refuses nothing. Three refusals of a real
// move are left unchecked here: a read or write that fails, a lock on the
// card that another process holds when the move tries to take it, and a
// stale basis, which a completion never carries. Apart from those, a move is
// refused today on no row this filter does not run.
//
// Nothing enforces that for a row added later. A refusal written into Do,
// evaluate or move, or into anything they call other than admit and canMove,
// reaches the real move and not this filter, and no test fails because of
// where it was written. Two things protect the filter. The rows live in the
// two functions it shares with the move, so the ordinary place to add a row
// is already shared. And TestAMoveOffersOnlyTheDestinationThatPasses and
// TestTheMoveFilterRefusesExactlyWhereTheMoveRefuses, in cmd/dinah, compare
// the offered set with a real move on a fresh copy of the workbench, row by
// row, but only in the states their fixtures build. A new row that refuses
// in some other state is caught only if somebody adds a fixture for it.
//
// A capacity question is asked of every row at once. When any destination
// declares a capacity, the live cards' headers are read once and counted per
// column, and every row's canMove reads that count rather than reading every
// card again for each destination. The count parts company with the real
// move's in one case, and the specification prescribes it. A card whose
// header will not read is skipped here, while Bench.Cards fails the whole
// move on it, so on a damaged workbench a destination with a capacity can be
// offered that the move then refuses with an error.
func (l *Library) MoveDestinations(req *Request) ([]LegalMove, error) {
	found, refused := l.admit(req)
	if refused != nil {
		return nil, nil
	}
	card := found.Card
	if card.Lapsed(l.Now()) {
		clearLapsedClaim(card)
	}
	return l.destinationsFor(req, card)
}

// destinationsFor is the per-row half of MoveDestinations: each of the card's
// legal moves that canMove, which runs canRoute and then canLand, passes for
// the request's owner, in the flow's order. The caller has already run admit
// and cleared a lapsed claim in memory. MoveDestinations and OfferActs both
// call it, so the completion of a move and the offer of the terminal head
// answer one set.
func (l *Library) destinationsFor(req *Request, card *bench.Card) ([]LegalMove, error) {
	moves := l.legalMoves(card)
	asking := *req
	if l.anyCapacity(moves) {
		occupancy, err := l.occupancy()
		if err != nil {
			return nil, err
		}
		asking.occupancy = occupancy
	}
	var accepted []LegalMove
	for _, move := range moves {
		row := asking
		row.Verb = Move
		row.Column = move.Column
		_, _, _, refusal, err := l.canMove(&row, card)
		if err != nil || refusal != nil {
			continue
		}
		accepted = append(accepted, move)
	}
	return accepted, nil
}

// anyCapacity reports whether any of the rows names a column that declares a
// capacity, which is the one case where a move has to count the cards.
func (l *Library) anyCapacity(moves []LegalMove) bool {
	for _, move := range moves {
		column := l.Bench.Column(move.Column)
		if column != nil && column.Capacity > 0 {
			return true
		}
	}
	return false
}

// occupancy counts the live cards standing in each column, from their headers
// alone, keyed by column identifier.
func (l *Library) occupancy() (map[string]int, error) {
	headers, err := l.Bench.LiveCardHeaders()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, header := range headers {
		counts[header.Column]++
	}
	return counts, nil
}
