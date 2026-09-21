package verb

import (
	"sort"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// admitRouteWrite runs the two rows a route write carries beyond the guard.
// Both read the card rather than the value, which is why neither is a guard and
// why both run after the field router's own list.
//
// It does not refuse a card standing at a column the new route omits. Refusing
// would make a route change impossible at the moment somebody most wants one,
// and a card standing off its route is a state the forward move and dinah check
// both already answer for.
func (l *Library) admitRouteWrite(req *Request, entity *bench.EntityRef, route string) *Response {
	card := entity.Card
	if card == nil {
		return nil
	}
	// Pending is the state that matters, and the reading is the workbench's
	// own, so the refusal here and the finding dinah check reports for the
	// same state cannot come to mean two different things.
	item, column, err := l.Bench.StrandedItemOf(card, route)
	if err != nil {
		return l.FromError(req, err)
	}
	if item != nil {
		// The item is named by the reference a person types to reach it,
		// which is what the next step's command needs to resolve.
		ref, err := l.itemCanonicalRef(card, item.ID)
		if err != nil {
			return l.FromError(req, err)
		}
		return l.refuseWith(req, card, contract.RouteStrandsItem, route, map[string]string{
			"item":   ref,
			"column": column.Ref(),
		})
	}
	if skipped := l.Bench.RouteSkipsOperatorColumn(card, route); skipped != nil {
		return l.refuseWith(req, card, contract.RouteSkipsOperatorColumn, route, map[string]string{
			"column": skipped.Ref(),
		})
	}
	return nil
}

// admitItemColumnRoute refuses moving an item onto a column the card's own
// route does not carry, which is the write half of the same rule File runs when
// the item is first filed.
//
// An item naming a column the card never reaches is a hold that never fires,
// and the workbench's own instructions call that the commonest way a stop
// somebody meant to create silently fails to exist.
func (l *Library) admitItemColumnRoute(req *Request, entity *bench.EntityRef, value string) *Response {
	if entity.Kind != bench.KindItem || entity.Card == nil {
		return nil
	}
	named := l.Bench.ColumnByRef(value)
	if l.Bench.RouteCarries(entity.Card, named) {
		return nil
	}
	return l.refuseWith(req, entity.Card, contract.ItemOffRoute, named.Ref(), map[string]string{
		"route": entity.Card.Route,
	})
}

// RouteView is one declared route as a listing reports it: the name, how many
// live columns it carries, and the live columns it omits.
type RouteView struct {
	// Name is the route's own name, as the declaration spells it.
	Name string `json:"name"`
	// Columns is how many live columns the route carries.
	Columns int `json:"columns"`
	// Skips are the live columns the route omits, in flow order, each as the
	// reference a person types to reach it.
	Skips []string `json:"skips,omitempty"`
	// OperatorOwnedSkips are the members of Skips the workbench reserves to
	// its operator, in the same order. They are named apart because that is
	// the one question an operator asks of a road somebody drew, and a reader
	// that had to cross-reference the column listing to answer it would be
	// deriving what this member states.
	OperatorOwnedSkips []string `json:"operator_owned_skips,omitempty"`
}

// RouteListing is what the roster word routes answers.
type RouteListing struct {
	Routes []RouteView `json:"routes"`
}

// Routes reports the workbench's declared routes in declaration order, which is
// the order the declaration reads in and the order a refusal names them back.
// A workbench declaring none answers the empty listing every other roster
// answers with.
func (l *Library) Routes() *RouteListing {
	listing := &RouteListing{Routes: []RouteView{}}
	for _, name := range l.Bench.RouteNames {
		view := RouteView{Name: name, Columns: len(bench.RouteColumnsIn(l.Bench.Routes[name], l.Bench.Columns))}
		for _, column := range l.Bench.RouteSkips(name) {
			view.Skips = append(view.Skips, columnRef(column))
			if column.OperatorOwned {
				view.OperatorOwnedSkips = append(view.OperatorOwnedSkips, columnRef(column))
			}
		}
		listing.Routes = append(listing.Routes, view)
	}
	return listing
}

// routeRoster is every value a query term on the route field may carry: the
// workbench's declared names in declaration order, followed by any route name a
// live card actually carries that the workbench does not declare, sorted among
// itself.
//
// The drifted tail is there for the reason the level roster carries one: dinah
// check reports a card naming a route nobody declares, and a query that could
// not find that card would make the finding unactionable from the command
// people filter with.
func routeRoster(b *bench.Bench, cards []*bench.Card) []string {
	seen := map[string]bool{}
	roster := make([]string, 0, len(b.RouteNames))
	for _, name := range b.RouteNames {
		seen[name] = true
		roster = append(roster, name)
	}
	var drift []string
	for _, card := range cards {
		if card.Route == "" || seen[card.Route] {
			continue
		}
		seen[card.Route] = true
		drift = append(drift, card.Route)
	}
	sort.Strings(drift)
	return append(roster, drift...)
}

// unknownRoute raises the refusal a caller naming a route the workbench does
// not declare meets, wherever it is raised from. The roster rides as a value
// read off the workbench rather than out of a sentence, so a route declared
// later reaches the message with nobody editing a catalog.
func (l *Library) unknownRoute(req *Request, card *bench.Card, name string) *Response {
	return l.refuseWith(req, card, contract.UnknownRoute, name, map[string]string{
		"routes": strings.Join(l.Bench.RouteNames, ", "),
	})
}
