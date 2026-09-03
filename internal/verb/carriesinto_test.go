package verb

import (
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// flowOf builds a flow of columns from a compact description, so a case below
// can state the shape it turns on in one line. Each entry is a kind, optionally
// followed by `+outside` for a column waiting on somebody outside and `+operator`
// for an operator-owned one.
func flowOf(descriptions ...string) []*bench.Column {
	columns := make([]*bench.Column, 0, len(descriptions))
	for at, description := range descriptions {
		fields := strings.Split(description, "+")
		column := &bench.Column{
			ID:       string(rune('a' + at)),
			Title:    fields[0],
			Kind:     fields[0],
			Position: at,
		}
		for _, marker := range fields[1:] {
			switch marker {
			case "outside":
				column.AwaitingOutside = true
			case "operator":
				column.OperatorOwned = true
			}
		}
		columns = append(columns, column)
	}
	return columns
}

// carriesIntoCase is one flow the walk is tested over: the shape, the column a
// card stands at, and the index of the column a pull would carry it into, or a
// negative index where no pull could carry it anywhere.
type carriesIntoCase struct {
	name  string
	flow  []*bench.Column
	from  int
	wants int
}

// carriesIntoCases is the table the walk is tested over. It stands outside the
// test that grew it because dinah-280 published the walk's answer as a field,
// and the test holding the field against the function asks these same shapes.
// A second list kept in step by hand is the drift both tests exist to prevent.
func carriesIntoCases() []carriesIntoCase {
	return []carriesIntoCase{
		{
			name:  "a station's card goes to the station beyond it",
			flow:  flowOf(contract.KindWork, contract.KindWork),
			from:  0,
			wants: 1,
		},
		{
			name:  "a buffer's card is carried to the station beyond",
			flow:  flowOf(contract.KindBuffer, contract.KindWork),
			from:  0,
			wants: 1,
		},
		{
			name:  "a run of queue columns is looked through",
			flow:  flowOf(contract.KindIntake, contract.KindBuffer, contract.KindBuffer, contract.KindWork),
			from:  0,
			wants: 3,
		},
		{
			name:  "a done column carries nothing",
			flow:  flowOf(contract.KindDone, contract.KindWork),
			from:  0,
			wants: -1,
		},
		{
			name:  "a column waiting on somebody outside carries nothing",
			flow:  flowOf(contract.KindWork+"+outside", contract.KindWork),
			from:  0,
			wants: -1,
		},
		{
			name:  "the run reaches the end of the flow",
			flow:  flowOf(contract.KindWork, contract.KindBuffer),
			from:  1,
			wants: -1,
		},
		{
			name:  "the run meets a done column",
			flow:  flowOf(contract.KindBuffer, contract.KindDone),
			from:  0,
			wants: -1,
		},
		{
			name:  "the run meets an operator-owned queue",
			flow:  flowOf(contract.KindBuffer, contract.KindBuffer+"+operator", contract.KindWork),
			from:  0,
			wants: -1,
		},
	}
}

// TestCarriesIntoAnswersWhereAPullWouldPutACard is dinah-273 AC-32. The walk is
// the one place the reach of a pull through a flow is written, so the table
// above is where the five cases that end it without an answer are held.
func TestCarriesIntoAnswersWhereAPullWouldPutACard(t *testing.T) {
	for _, c := range carriesIntoCases() {
		t.Run(c.name, func(t *testing.T) {
			got := carriesInto(c.flow[c.from], c.flow)
			if c.wants < 0 {
				if got != nil {
					t.Fatalf("wanted no answer, got %s", got.Title)
				}
				return
			}
			if got != c.flow[c.wants] {
				t.Fatalf("wanted the column at %d, got %+v", c.wants, got)
			}
		})
	}

	t.Run("an operator-owned queue in the run stops the walk for the operator too", func(t *testing.T) {
		// carriesInto reads no caller at all, so the operator meets the same
		// answer as anybody else. The asymmetry the filter below relies on is
		// that the walk never tests the column it starts from.
		flow := flowOf(contract.KindBuffer, contract.KindBuffer+"+operator", contract.KindWork)
		if got := carriesInto(flow[0], flow); got != nil {
			t.Fatalf("the walk ran past an operator-owned queue and answered %s", got.Title)
		}
		if got := carriesInto(flow[1], flow); got != flow[2] {
			t.Fatalf("a card standing at the operator-owned queue is carried on, got %+v", got)
		}
	})
}

// TestTheColumnViewPublishesCarriesIntosOwnAnswer is dinah-280 AC-2 and AC-4.
// It asserts no expectation of its own. For every flow shape the walk is
// tested over, and for every column of each, the published field is held
// against what carriesInto answers for that same column, so the field cannot
// come to read differently from the pull it describes.
func TestTheColumnViewPublishesCarriesIntosOwnAnswer(t *testing.T) {
	for _, c := range carriesIntoCases() {
		t.Run(c.name, func(t *testing.T) {
			flow := c.flow
			library := &Library{Bench: &bench.Bench{Root: t.TempDir(), Columns: flow}}
			views := library.columnViews(nil)
			if len(views) != len(flow) {
				t.Fatalf("the flow has %d columns and the listing carries %d", len(flow), len(views))
			}
			for at, column := range flow {
				want := ""
				if destination := carriesInto(column, flow); destination != nil {
					want = columnRef(destination)
				}
				if views[at].PullDestination != want {
					t.Errorf("%s publishes pull_destination %q and carriesInto answers %q",
						column.ID, views[at].PullDestination, want)
				}
			}
		})
	}

	// The loop above holds the field against the function whatever the
	// function says, so a shape that answers nothing everywhere would satisfy
	// it. Two stations standing together is the branch a buffered flow never
	// reaches, so it is named here with the answer written out.
	t.Run("a station carries a card into the station beyond it", func(t *testing.T) {
		flow := flowOf(contract.KindWork, contract.KindWork)
		library := &Library{Bench: &bench.Bench{Root: t.TempDir(), Columns: flow}}
		views := library.columnViews(nil)
		if views[0].PullDestination != flow[1].ID {
			t.Errorf("the first station publishes pull_destination %q, wanted %q", views[0].PullDestination, flow[1].ID)
		}
		if views[1].PullDestination != "" {
			t.Errorf("the last station publishes pull_destination %q and nothing stands beyond it", views[1].PullDestination)
		}
	})
}

// TestPullSourcesFiltersTheWalkRatherThanRepeatingIt is dinah-273 AC-38. Every
// column carriesInto answers with the destination is a source, in nearest-first
// order, and the operator-owned queue standing immediately upstream is among
// them, which is the asymmetry a hand-written backward walk would lose.
func TestPullSourcesFiltersTheWalkRatherThanRepeatingIt(t *testing.T) {
	flow := flowOf(contract.KindIntake, contract.KindBuffer+"+operator", contract.KindBuffer, contract.KindWork)
	for _, destination := range flow {
		var wanted []*bench.Column
		for at := len(flow) - 1; at >= 0; at-- {
			if carriesInto(flow[at], flow) == destination {
				wanted = append(wanted, flow[at])
			}
		}
		got := pullSources(destination, flow)
		if len(got) != len(wanted) {
			t.Fatalf("the sources of %s are %v and the filter answered %v", destination.ID, refsOf(wanted), refsOf(got))
		}
		for at := range got {
			if got[at] != wanted[at] {
				t.Fatalf("the sources of %s are %v in nearest-first order and the filter answered %v",
					destination.ID, refsOf(wanted), refsOf(got))
			}
		}
	}

	// The assertion above holds by construction unless the operator-owned
	// buffer really is a source of the station, so that one case is named.
	station := flow[3]
	found := false
	for _, source := range pullSources(station, flow) {
		if source == flow[1] {
			found = true
		}
	}
	if !found {
		t.Error("the operator-owned buffer standing immediately upstream is not among the station's sources")
	}
}

// refsOf names a run of columns for a failure message.
func refsOf(columns []*bench.Column) []string {
	var names []string
	for _, column := range columns {
		names = append(names, column.ID)
	}
	return names
}
