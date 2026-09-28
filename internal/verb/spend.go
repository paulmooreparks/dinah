package verb

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// SpendFigures are the figures a spend line carries, each present only where
// the provider reported it, on the terms bench.Event carries them.
type SpendFigures struct {
	Input  *float64 `json:"input,omitempty"`
	Output *float64 `json:"output,omitempty"`
	Cached *float64 `json:"cached,omitempty"`
	Total  *float64 `json:"total,omitempty"`
}

// SpendRecord is one spend line as a read reports it. Provider and Model are
// the consumer's: the line's own consumer where it names one, and the
// recorder's otherwise, so a reader summing by model never has to know which
// of the two the line carried.
type SpendRecord struct {
	// TS is when the line was written.
	TS string `json:"ts"`
	// Actor is who wrote the line.
	Actor string `json:"actor"`
	// Provider and Model are the consumer, resolved as the type comment says.
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	// Unit is what the figures are counted in.
	Unit string `json:"unit"`
	SpendFigures
	// Unreported marks a line whose harness reported no figure.
	Unreported bool `json:"unreported,omitempty"`
	// Round is the pass of the station the line belongs to, absent where the
	// caller did not say.
	Round int `json:"round,omitempty"`
	// Column and ColumnTitle are the column the work was performed in, as of
	// the write.
	Column      string `json:"column,omitempty"`
	ColumnTitle string `json:"column_title,omitempty"`
	// Note is the remark the caller kept beside the figures.
	Note string `json:"note,omitempty"`
}

// SpendTotal is the sum over the records that share a column, a consumer and
// a unit. A figure is summed across the records that carry it and is absent
// where none did, so a total never presents an absent figure as a zero.
// Records counts every line in the group and Unreported the lines among
// them that carried no figure, so a reader sees what the sum leaves out.
type SpendTotal struct {
	Column      string `json:"column,omitempty"`
	ColumnTitle string `json:"column_title,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	Unit        string `json:"unit"`
	SpendFigures
	Records    int `json:"records"`
	Unreported int `json:"unreported,omitempty"`
}

// SpendReport is what spend answers when it is asked rather than told: one
// card's lines and their totals, or the whole workbench's totals when the
// call named no card. Card is the card's reference, empty on the workbench
// report, and Records is empty there too, because the workbench's question is
// what each column costs rather than what every line said.
type SpendReport struct {
	Card    string        `json:"card,omitempty"`
	Records []SpendRecord `json:"records,omitempty"`
	Totals  []SpendTotal  `json:"totals,omitempty"`
}

// Recording reports whether a spend request records rather than reports: it
// names a unit, a figure, the unreported marker, or any of the members only a
// record takes. A call carrying one of those and no unit is still a record,
// so that Spend refuses it over the missing unit rather than the report
// answering as though the figures had not been typed.
func (req *Request) Recording() bool {
	return strings.TrimSpace(req.Unit) != "" || req.Unreported ||
		strings.TrimSpace(req.Input) != "" || strings.TrimSpace(req.Output) != "" ||
		strings.TrimSpace(req.Cached) != "" || strings.TrimSpace(req.Total) != "" ||
		strings.TrimSpace(req.By) != "" || strings.TrimSpace(req.Round) != "" ||
		strings.TrimSpace(req.Column) != "" || strings.TrimSpace(req.Note) != ""
}

// SpendCall is spend's whole surface behind one name, which is what a head
// that dispatches by command name needs: a record where the request is one,
// and the report otherwise. A refused read comes back as a Response so that
// every head reports it the way it reports any refusal.
func (l *Library) SpendCall(req *Request) any {
	if req.Recording() {
		return l.Spend(req)
	}
	report, err := l.SpendReport(req)
	if err != nil {
		return l.FromError(req, err)
	}
	return report
}

// canSpend runs Spend's rows ahead of what the person typed: the card
// resolves, the request names an owner and the declared harness is well
// formed. Spend and OfferActs both call it.
//
// The reference goes to the general resolver rather than to ResolveCard, on
// the terms canComment sends its own there: a reference naming a collection
// is then refused as the collection it is, the way every other card-taking
// verb refuses one, and a reference reaching anything but a card is refused
// as the card that does not exist. A blank reference is refused ahead of
// the resolver, which would otherwise read it as the workbench.
func (l *Library) canSpend(req *Request) (*bench.Card, *Response) {
	if strings.TrimSpace(req.Card) == "" {
		return nil, l.refuse(req, nil, contract.UnknownCard, "")
	}
	entity, err := l.Bench.ResolveEntity(req.Card)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	if entity.Kind != bench.KindCard || entity.Card == nil {
		return nil, l.refuse(req, entity.Card, contract.UnknownCard, req.Card)
	}
	if req.Actor == "" {
		return nil, l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if refused := l.malformedHarness(req, entity.Card); refused != nil {
		return nil, refused
	}
	return entity.Card, nil
}

// Spend appends one spend line to a card's journal. It is a record and never
// a gate: nothing reads the sum to refuse a claim or a move, because Dinah
// records what was spent and whoever runs the agents decides what to do
// about it.
//
// The unit is one lowercase word and every figure a number that is not
// negative, each refused malformed by its own name. A call carrying no figure
// says so with the unreported marker, and a call carrying both is
// contradictory, so each of those is refused malformed as well: the first
// over figure, the second over the marker. The consumer is written only
// where the call named one, so a line records who wrote it and, apart from
// that, who consumed it.
func (l *Library) Spend(req *Request) *Response {
	card, refused := l.canSpend(req)
	if refused != nil {
		return refused
	}
	ev, refused := l.spendEvent(req, card)
	if refused != nil {
		return refused
	}
	now := bench.Stamp(l.Now())
	lock, err := l.Bench.Acquire(card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	ev.TS = now
	if err := bench.AppendEvent(card.JournalPath(), *ev); err != nil {
		return l.FromError(req, err)
	}
	return l.ok(req, card)
}

// spendUnitLegal reports whether a unit is one lowercase word: a letter, then
// letters, digits, hyphens and underscores. It is the declared-field key
// grammar's segment rule, because a unit is a name a reader compares and
// sums by, and a name that varies in case or carries a space would split one
// unit into several.
func spendUnitLegal(unit string) bool {
	if unit == "" {
		return false
	}
	for i, r := range unit {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && (r >= '0' && r <= '9' || r == '-' || r == '_'):
		default:
			return false
		}
	}
	return true
}

// spendFigure parses one figure as the caller wrote it: absent where the
// caller wrote nothing, and otherwise a finite number that is not negative.
// The second answer is false where the text will not do.
func spendFigure(written string) (*float64, bool) {
	written = strings.TrimSpace(written)
	if written == "" {
		return nil, true
	}
	value, err := strconv.ParseFloat(written, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return nil, false
	}
	return &value, true
}

// spendEvent composes the line a spend request writes, or the refusal the
// request earns. The column is the card's own unless the call named one, and
// its title is captured as of the write the way a move captures its states,
// so the line reads under the column's name of the day whatever it is called
// later.
func (l *Library) spendEvent(req *Request, card *bench.Card) (*bench.Event, *Response) {
	unit := strings.TrimSpace(req.Unit)
	if !spendUnitLegal(unit) {
		return nil, l.refuse(req, card, contract.Malformed, "unit")
	}
	ev := &bench.Event{Actor: req.Acting(), Event: contract.EventSpend, Unit: unit, Unreported: req.Unreported}
	var ok bool
	figures := 0
	for _, figure := range []struct {
		flag    string
		written string
		into    **float64
	}{
		{"--input", req.Input, &ev.Input},
		{"--output", req.Output, &ev.Output},
		{"--cached", req.Cached, &ev.Cached},
		{"--total", req.Total, &ev.Total},
	} {
		if *figure.into, ok = spendFigure(figure.written); !ok {
			return nil, l.refuse(req, card, contract.Malformed, figure.flag)
		}
		if *figure.into != nil {
			figures++
		}
	}
	if figures == 0 && !req.Unreported {
		return nil, l.refuse(req, card, contract.Malformed, "figure")
	}
	if figures > 0 && req.Unreported {
		return nil, l.refuse(req, card, contract.Malformed, "--unreported")
	}
	if by := strings.TrimSpace(req.By); by != "" {
		provider, model, cut := strings.Cut(by, "/")
		provider, model = strings.TrimSpace(provider), strings.TrimSpace(model)
		if !cut || provider == "" || model == "" {
			return nil, l.refuse(req, card, contract.Malformed, "--by")
		}
		ev.ConsumerProvider, ev.ConsumerModel = provider, model
	}
	if written := strings.TrimSpace(req.Round); written != "" {
		round, err := strconv.Atoi(written)
		if err != nil || round < 1 {
			return nil, l.refuse(req, card, contract.Malformed, "--round")
		}
		ev.Round = round
	}
	column := l.Bench.Column(card.Column)
	if named := strings.TrimSpace(req.Column); named != "" {
		if column = l.Bench.ColumnByRef(named); column == nil {
			return nil, l.refuse(req, card, contract.UnknownColumn, named)
		}
	}
	if column != nil {
		ev.Column, ev.ColumnTitle = column.ID, column.Title
	} else {
		ev.Column = card.Column
	}
	ev.Note = strings.TrimSpace(req.Note)
	return ev, nil
}

// SpendReport answers what was spent: on one card, its lines and their totals,
// and on the workbench, when the request names no card, the totals of every
// live card. The archive is out of it, on the terms Bench.Cards leaves the
// archive out of every walk.
func (l *Library) SpendReport(req *Request) (*SpendReport, error) {
	if strings.TrimSpace(req.Card) == "" {
		cards, err := l.Bench.Cards()
		if err != nil {
			return nil, err
		}
		var all []SpendRecord
		for _, card := range cards {
			records, err := l.spendRecords(card)
			if err != nil {
				return nil, err
			}
			all = append(all, records...)
		}
		return &SpendReport{Totals: l.spendTotals(all)}, nil
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return nil, err
	}
	records, err := l.spendRecords(found.Card)
	if err != nil {
		return nil, err
	}
	return &SpendReport{Card: found.Card.Ref(l.Bench.Slug), Records: records, Totals: l.spendTotals(records)}, nil
}

// spendRecords reads a card's spend lines off its journal, in the order they
// were written, resolving each line's consumer as SpendRecord says.
func (l *Library) spendRecords(card *bench.Card) ([]SpendRecord, error) {
	events, _, err := l.Bench.ReadJournal(card.JournalPath())
	if err != nil {
		return nil, err
	}
	var records []SpendRecord
	for _, ev := range events {
		if ev.Event != contract.EventSpend {
			continue
		}
		record := SpendRecord{
			TS:          ev.TS,
			Actor:       ev.Actor.Name,
			Provider:    ev.Actor.Provider,
			Model:       ev.Actor.Model,
			Unit:        ev.Unit,
			Unreported:  ev.Unreported,
			Round:       ev.Round,
			Column:      ev.Column,
			ColumnTitle: ev.ColumnTitle,
			Note:        ev.Note,
		}
		record.SpendFigures = SpendFigures{Input: ev.Input, Output: ev.Output, Cached: ev.Cached, Total: ev.Total}
		if ev.ConsumerProvider != "" || ev.ConsumerModel != "" {
			record.Provider, record.Model = ev.ConsumerProvider, ev.ConsumerModel
		}
		records = append(records, record)
	}
	return records, nil
}

// spendTotals sums records by column, consumer and unit. The groups come
// back in the workbench's column order, then by provider, model and unit, so
// the workbench report reads down the flow the way the board is drawn, and a
// line whose column the workbench no longer declares sorts after every column
// it does.
func (l *Library) spendTotals(records []SpendRecord) []SpendTotal {
	type key struct{ column, provider, model, unit string }
	index := map[key]int{}
	var totals []SpendTotal
	for _, record := range records {
		k := key{record.Column, record.Provider, record.Model, record.Unit}
		at, seen := index[k]
		if !seen {
			at = len(totals)
			index[k] = at
			totals = append(totals, SpendTotal{
				Column: record.Column, ColumnTitle: record.ColumnTitle,
				Provider: record.Provider, Model: record.Model, Unit: record.Unit,
			})
		}
		total := &totals[at]
		total.Records++
		if record.Unreported {
			total.Unreported++
		}
		addFigure(&total.Input, record.Input)
		addFigure(&total.Output, record.Output)
		addFigure(&total.Cached, record.Cached)
		addFigure(&total.Total, record.Total)
	}
	position := map[string]int{}
	for i, column := range l.Bench.Columns {
		position[column.ID] = i
	}
	rank := func(column string) int {
		if at, declared := position[column]; declared {
			return at
		}
		return len(l.Bench.Columns)
	}
	sort.SliceStable(totals, func(i, j int) bool {
		a, b := totals[i], totals[j]
		if rank(a.Column) != rank(b.Column) {
			return rank(a.Column) < rank(b.Column)
		}
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		return a.Unit < b.Unit
	})
	return totals
}

// addFigure adds a record's figure into a running sum that is absent until
// the first record carrying the figure arrives.
func addFigure(sum **float64, figure *float64) {
	if figure == nil {
		return
	}
	if *sum == nil {
		value := *figure
		*sum = &value
		return
	}
	**sum += *figure
}
