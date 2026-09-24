package verb

import (
	"dinah/internal/bench"
	"dinah/internal/contract"
)

// fileStandingItems mints, on one card, an instance of every standing entry
// of one column the card carries no live instance of, and answers the journal
// lines it wrote. It is the one minting path: the four arrivals call it (add,
// move, pull and a reshape carrying a card), and so does the operator's
// `dinah check --file-standing` repair, so the anchors and the lines a repair
// writes are byte for byte what an arrival would have written.
//
// The caller holds the card's lock and has already appended its own line, so
// the minted lines land after the line recording the arrival, at the same
// instant. For each entry in declaration order the anchor is written and then
// its line appended, so a crash between the two leaves an item with no line,
// which is the shape a crashed `dinah file` already leaves and which the next
// arrival does not duplicate, since the identity check reads the anchor.
//
// The actor is the arriving act's own, composed through the request as every
// other line's is. A column is not an actor and the journal refuses a line
// with none, so the honest record is that the mover's act caused the filing
// and the column's declaration decided what was filed, which is what the
// line's column, column_title and standing members say.
//
// Minting never refuses the arrival: every refusal the act runs has passed
// before this is reached, and the destination's own entry hold was read
// before the instances existed, so a first arrival is never held by the items
// it is about to receive.
func fileStandingItems(req *Request, b *bench.Bench, card *bench.Card, column *bench.Column, ts string) ([]bench.Event, error) {
	missing, err := b.MissingStandingItems(card, column)
	if err != nil {
		return nil, err
	}
	var written []bench.Event
	for _, entry := range missing {
		item, err := bench.AddStandingItem(card.Dir, column.ID, entry, ts)
		if err != nil {
			return written, err
		}
		ev := bench.Event{
			TS:          ts,
			Event:       contract.EventItemFiled,
			Actor:       req.Acting(),
			Item:        item.ID,
			Kind:        entry.Kind,
			Column:      column.ID,
			ColumnTitle: column.Title,
			Standing:    entry.Key,
		}
		if err := bench.AppendEvent(card.JournalPath(), ev); err != nil {
			return written, err
		}
		written = append(written, ev)
	}
	return written, nil
}

// mintStandingItems is fileStandingItems for an arrival the library itself
// carried out: it reads the destination off the library's own bench and stamps
// the request's actor, and it answers nothing, because an arrival reports the
// card rather than the items. It is the site add, move and pull share; the
// reshape carry reads a fresh bench of its own and calls the helper directly.
func (l *Library) mintStandingItems(req *Request, card *bench.Card, destination *bench.Column, ts string) error {
	_, err := fileStandingItems(req, l.Bench, card, destination, ts)
	return err
}

// StandingFiling is one instance the repair filed or would file: the card,
// the declaring column and the entry key, which is the same triple the
// check.standing-item-missing finding names.
type StandingFiling struct {
	// Card is the card's reference.
	Card string `json:"card"`
	// Column is the declaring column's reference.
	Column string `json:"column"`
	// Key is the standing entry's key.
	Key string `json:"key"`
}

// StandingRepair is what one `dinah check --file-standing` run answers: the
// instances it filed, or, on a preview, the instances a confirmed run would
// file. It is the two-phase shape the branch migration runs, and it reads the
// same way: Preview says nothing was written.
type StandingRepair struct {
	// Preview says the run wrote nothing because it carried no confirmation,
	// so Filed names what a confirmed run would mint.
	Preview bool `json:"preview,omitempty"`
	// Filed are the instances, in card order and then declaration order.
	Filed []StandingFiling `json:"filed,omitempty"`
}

// fileStanding is the repair behind `dinah check --file-standing`: every live
// card standing in a column whose declaration it carries no instance of
// receives one, through the same path an arrival uses, with the operator as
// actor. It is refused not-operator to anybody else, because it writes items
// the workbench then holds cards on. A blocked or claimed card is filed for
// as any other, since the declaration is a fact about the column rather than
// about who holds the card; the repair takes each card's lock in turn.
//
// Without confirmation it reports what it would file and writes nothing, and
// the preview reads the same store the confirmed run reads, so the two agree
// unless somebody moved a card between them.
func (l *Library) fileStanding(req *Request) (*StandingRepair, error) {
	if req.Actor != l.Bench.Operator {
		return nil, contract.Refuse(contract.NotOperator, req.Actor)
	}
	report := &StandingRepair{Preview: !req.Confirm}
	ids, err := bench.ListIDs(l.Bench.CardsRoot())
	if err != nil {
		return report, err
	}
	now := bench.Stamp(l.Now())
	for _, id := range ids {
		card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), id)
		if err != nil {
			continue
		}
		column := l.Bench.Column(card.Column)
		if column == nil || len(column.StandingItems) == 0 {
			continue
		}
		if !req.Confirm {
			missing, err := l.Bench.MissingStandingItems(card, column)
			if err != nil {
				return report, err
			}
			for _, entry := range missing {
				report.Filed = append(report.Filed, StandingFiling{Card: card.Ref(l.Bench.Slug), Column: column.Ref(), Key: entry.Key})
			}
			continue
		}
		lock, err := bench.Acquire(card.Dir, req.Actor, now)
		if err != nil {
			return report, err
		}
		written, err := fileStandingItems(req, l.Bench, card, column, now)
		lock.Release()
		for _, ev := range written {
			report.Filed = append(report.Filed, StandingFiling{Card: card.Ref(l.Bench.Slug), Column: column.Ref(), Key: ev.Standing})
		}
		if err != nil {
			return report, err
		}
	}
	return report, nil
}
