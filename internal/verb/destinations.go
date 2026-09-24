package verb

// MoveDestinations reports the rows of the card's legal moves that a move
// request would pass every check on, for the request's actor and override.
// It takes no lock, writes nothing, lapses no claim on disk and witnesses no
// divergence. Wherever Do would refuse every move, it answers nothing.
//
// It runs Do's own rows in Do's own order up to the card's lock: the
// workbench has an operator, the declared harness is well formed, the card
// resolves, and the request names an owner. Those are the rows Do runs before
// canMove that depend on neither the card's state nor the destination, and
// each of them refuses every move alike, so any of them firing answers
// nothing. A claim that has lapsed is cleared on the card in memory, as lapse
// would clear it on disk, and then each row is put to canMove itself, so the
// offered set cannot drift from the checks a real move makes.
//
// A capacity question is asked of every row at once. When any destination
// declares a capacity, the live cards' headers are read once and counted per
// column, and every row's canMove reads that count rather than reading every
// card again for each destination.
func (l *Library) MoveDestinations(req *Request) ([]LegalMove, error) {
	if l.Bench.Operator == "" {
		return nil, nil
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return nil, nil
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return nil, err
	}
	if req.Actor == "" {
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
