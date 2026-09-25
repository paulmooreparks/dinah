package verb

import (
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// scheduleOf is every schedule condition that holds for a card on the
// request's day, in the precedence order contract.ScheduleConditions fixes,
// and nil where none does. Nothing about a condition is stored: each is
// computed here from the card's own three dates, the workbench's soon window,
// today, whether the card has started for the two conditions start_by
// drives, and the request's hold index for waiting.
//
// A card that has finished holds no condition at all, whatever its dates say.
// A stored date that does not parse is read as absent, so it contributes no
// condition.
func (l *Library) scheduleOf(card *bench.Card, day *requestDay) ([]string, error) {
	if l.hasFinished(card) {
		return nil, nil
	}
	today := day.date
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
		started := l.hasStarted(card, day.now)
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
	holds, err := l.holdsAt(day)
	if err != nil {
		return nil, err
	}
	if len(holds.holdsOf(l.Bench, card, today)) > 0 {
		held = append(held, contract.ScheduleWaiting)
	}
	return held, nil
}

// hasStarted reports whether a card has started, which is a position in the
// flow rather than an event in its history: it stands at or beyond the
// workbench's commitment column, or somebody holds it now. It answers
// Bench.Started, the one definition the holds read as well, and reads no
// journal. A claim taken before the commitment column starts the card only
// while it is held, so a card released there, or whose claim has expired at
// now, has not started, and a card moved back before the commitment column
// has not started either.
func (l *Library) hasStarted(card *bench.Card, now time.Time) bool {
	return l.Bench.Started(card, now)
}

// hasFinished reports whether a card stands in a column of kind done, which
// is what finishing means for the schedule conditions.
func (l *Library) hasFinished(card *bench.Card) bool {
	column := l.Bench.Column(card.Column)
	return column != nil && column.Terminal()
}

// startHold reports whether a ready card may not be selected yet, and why.
// It carries two grounds. The card's own start_after later than today holds
// it wherever it stands, and releases it on that date. A link the workbench
// declares under dinah.holds holds a card that has not started while the
// card it waits on has not reached the rule's event, and for the rule's lag
// after it did; the first waits on a card and no date releases it, and the
// second is released by time on the day the lag runs out.
//
// It is a function value, on the pattern of landingFor, so that the scans in
// headOfReadyFor ask one question and read neither today, any date nor any
// link themselves.
type startHold func(*bench.Card) holdAnswer

// startHoldFor is the start hold every selection on this call applies. Each
// caller builds it once, from today as Bench.Today reads it and from the
// request's hold index, and passes it to every scan it runs, so one call
// reads the clock once and one graph.
func (l *Library) startHoldFor(today bench.Date, holds *holdIndex) startHold {
	return func(card *bench.Card) holdAnswer {
		answer := holdAnswer{}
		if startAfter, ok := card.ScheduleDate(bench.StartAfterField); ok && startAfter.After(today) {
			answer.Held, answer.From = true, startAfter.String()
		}
		for _, ground := range holds.holdsOf(l.Bench, card, today) {
			answer.Held = true
			if ground.awaiting() {
				answer.Waiting = append(answer.Waiting, ground)
				continue
			}
			if ground.Until > answer.From {
				answer.From = ground.Until
			}
		}
		if len(answer.Waiting) > 0 {
			answer.From = ""
		}
		return answer
	}
}

// selectionHold is the start hold for a selection made on req, on the day
// and the hold index every other schedule reading of that request uses.
func (l *Library) selectionHold(req *Request) (startHold, error) {
	holds, err := l.holdsOn(req)
	if err != nil {
		return nil, err
	}
	return l.startHoldFor(l.today(req), holds), nil
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
