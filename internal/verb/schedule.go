package verb

import (
	"dinah/internal/bench"
	"dinah/internal/contract"
)

// scheduleOf is every schedule condition that holds for a card today, in the
// precedence order contract.ScheduleConditions fixes, and nil where none
// does. Nothing about a condition is stored: each is computed here from the
// card's own three dates, the workbench's soon window, today, and, for the
// two conditions start_by drives, whether the card has started.
//
// A card that has finished holds no condition at all, whatever its dates say.
// A stored date that does not parse is read as absent, so it contributes no
// condition. The card's journal is read only where the card carries a
// parseable start_by and is not active, because that is the one question the
// journal answers here.
func (l *Library) scheduleOf(card *bench.Card, today bench.Date) ([]string, error) {
	if l.hasFinished(card) {
		return nil, nil
	}
	settings, _ := l.Bench.Schedule()
	horizon := today.AddDays(settings.SoonDays)
	soon := func(date bench.Date) bool {
		return !date.Before(today) && !date.After(horizon)
	}
	var held []string
	due, carriesDue := card.ScheduleDate(bench.DueField)
	if carriesDue && due.Before(today) {
		held = append(held, contract.ScheduleOverdue)
	}
	startBy, carriesStartBy := card.ScheduleDate(bench.StartByField)
	late, startSoon := false, false
	if carriesStartBy && (startBy.Before(today) || soon(startBy)) {
		started, err := l.hasStarted(card)
		if err != nil {
			return nil, err
		}
		late = !started && startBy.Before(today)
		startSoon = !started && soon(startBy)
	}
	if late {
		held = append(held, contract.ScheduleLateStart)
	}
	if carriesDue && soon(due) {
		held = append(held, contract.ScheduleDueSoon)
	}
	if startSoon {
		held = append(held, contract.ScheduleStartSoon)
	}
	if startAfter, ok := card.ScheduleDate(bench.StartAfterField); ok && startAfter.After(today) {
		held = append(held, contract.ScheduleNotYet)
	}
	return held, nil
}

// hasStarted reports whether a card has been taken up: it is active now, or
// its journal records at least one claim. A pull records a claim, so a pulled
// card has started. The claim is the core's own act of taking work up, which
// is why it is the start rather than a move out of any particular column, and
// a claim is append-only history, so a card that has started never stops
// having started.
//
// It takes any card rather than the one being scheduled, so a later rule
// keyed to another card starting asks the question the conditions ask.
func (l *Library) hasStarted(card *bench.Card) (bool, error) {
	if card.State == contract.StateActive {
		return true, nil
	}
	events, _, err := bench.ReadJournal(card.JournalPath())
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if event.Event == contract.EventClaimed {
			return true, nil
		}
	}
	return false, nil
}

// hasFinished reports whether a card stands in a column of kind done, which
// is what finishing means here. It takes any card, on hasStarted's terms.
func (l *Library) hasFinished(card *bench.Card) bool {
	column := l.Bench.Column(card.Column)
	return column != nil && column.Terminal()
}

// startHold reports whether a ready card may not be selected yet, and the
// date from which it may be, as YYYY-MM-DD. At dinah-605 the one ground is the
// card's own start_after later than today, and every hold carries its date.
//
// It is a function value, on the pattern of landingFor, so that the scans in
// headOfReadyFor ask one question and read neither today nor any date
// themselves. A later ground, such as a hold carried through a link to
// another card, becomes a further test inside startHoldFor, which runs on the
// Library and can read another card and its journal, and none of the scans
// changes shape to take it. A ground carrying no date has nothing to put in
// the second answer, and the fold in headOfReadyFor reads an empty date as no
// hold having carried one, so a ground of that kind changes that fold as well.
type startHold func(*bench.Card) (held bool, from string)

// startHoldFor is the start hold every selection on this call applies. Each
// caller builds it once, from today as Bench.Today reads it, and passes it to
// every scan it runs, so one call reads the clock once.
func (l *Library) startHoldFor(today bench.Date) startHold {
	return func(card *bench.Card) (bool, string) {
		startAfter, ok := card.ScheduleDate(bench.StartAfterField)
		if !ok || !startAfter.After(today) {
			return false, ""
		}
		return true, startAfter.String()
	}
}

// selectionHold is the start hold for a selection starting now.
func (l *Library) selectionHold() startHold {
	return l.startHoldFor(l.Bench.Today(l.Now()))
}

// earlierDate folds two dates written YYYY-MM-DD into the earlier of them,
// either of which may be empty for no date. The text compares in date order
// because the layout is fixed width and most significant first.
func earlierDate(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case b < a:
		return b
	}
	return a
}

// scheduleOrderWarning sets the order warning on a successful write that
// leaves the card's dates out of order, naming the first violated pair. A
// write is never refused over the order, because entering dates one at a
// time passes through inconsistent states.
func scheduleOrderWarning(response *Response, card *bench.Card) {
	if response == nil || card == nil || response.Outcome != contract.OutcomeOK {
		return
	}
	violations := card.ScheduleOrderViolations()
	if len(violations) == 0 {
		return
	}
	first := violations[0]
	response.Warning = "warn.schedule-order"
	response.WarningDetail = first.First + " " + first.FirstDate + " is after " + first.Second + " " + first.SecondDate
}
