package verb

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
// and witnesses a divergence, which refuses nothing. Leaving aside a read or
// write that fails and a stale basis, which a completion never carries, a
// move is refused on no row this filter does not run.
// TestNoMoveRefusalIsRaisedOutsideTheSharedChecks keeps it that way, by
// failing when Do or move raises a refusal anywhere but through those two.
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
	moves := l.legalMoves(card)
	if card.Lapsed(l.Now()) {
		clearLapsedClaim(card)
	}
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
