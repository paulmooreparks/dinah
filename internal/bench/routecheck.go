package bench

import (
	"path/filepath"
	"regexp"
)

// validRouteName matches ColumnSlugPattern, which is the grammar a route name
// answers to. The name is typed on a command line and read back out of a
// refusal, which is what a slug grammar is for.
var validRouteName = regexp.MustCompile(ColumnSlugPattern)

// checkRoutes applies the route rules to the workbench's own declaration. Every
// rule is a finding rather than a refusal on read, so a workbench whose routes
// are nonsense opens and is reported.
//
// The findings are reported per route in declaration order, and within a route
// in the order the table of section 8 states them, so two runs over one
// workbench report the same rows in the same order.
func (b *Bench) checkRoutes() []Finding {
	if len(b.RouteNames) == 0 {
		return nil
	}
	anchor := filepath.Join(b.Root, WorkbenchAnchor)
	var findings []Finding
	for _, name := range b.RouteNames {
		findings = append(findings, b.checkRoute(anchor, name)...)
	}
	return findings
}

// checkRoute applies every rule to one declared route.
func (b *Bench) checkRoute(anchor, name string) []Finding {
	var findings []Finding
	report := func(key, detail string) {
		findings = append(findings, Finding{Path: anchor, Key: key, Detail: detail})
	}
	if !validRouteName.MatchString(name) {
		report(FindingRouteNameMalformed, name)
	}
	ids := b.Routes[name]
	seen := map[string]bool{}
	var declared []*Column
	for _, id := range ids {
		column := b.Column(id)
		if column == nil {
			report(FindingRouteUnknownColumn, name+" "+id)
			continue
		}
		if seen[id] {
			report(FindingRouteDuplicateColumn, name+" "+column.Ref())
			continue
		}
		seen[id] = true
		declared = append(declared, column)
	}
	if len(declared) == 0 {
		report(FindingRouteEmpty, name)
		return findings
	}
	// The declaration's own order is compared against flow order over the
	// columns that resolve, because an identifier naming nothing has no place
	// in the flow to be out of order against and has already been reported.
	for i := 1; i < len(declared); i++ {
		if declared[i].Position < declared[i-1].Position {
			report(FindingRouteOutOfOrder, name)
			break
		}
	}
	route := RouteColumnsIn(ids, b.Columns)
	if len(b.Columns) > 0 && RouteIndexOf(route, b.Columns[0]) < 0 {
		report(FindingRouteMissingFirstColumn, name+" "+b.Columns[0].Ref())
	}
	if last := route[len(route)-1]; !last.Terminal() {
		report(FindingRouteMissingTerminal, name+" "+last.Ref())
	}
	for _, column := range route {
		if column.RejectTo == "" {
			continue
		}
		target := b.ColumnByRef(column.RejectTo)
		// A target naming no column at all is checkRejectTargets' finding
		// rather than this one's, and reporting it twice would tell a reader
		// to make two repairs where there is one.
		if target == nil || RouteIndexOf(route, target) >= 0 {
			continue
		}
		report(FindingRouteRejectTargetOffRoute, name+" "+column.Ref()+" "+target.Ref())
	}
	return findings
}

// checkCardRoute reports what one card's route key says that this workbench
// cannot act on: a name it does not declare, and a card standing at a column
// its own route does not carry.
//
// It mirrors checkTierOverrides rather than refusing on read, for the reason
// that function gives: a reshape can retire a column long after somebody wrote
// the value, so a reader has to tolerate a reference that no longer resolves.
func (b *Bench) checkCardRoute(card *Card) []Finding {
	if card.Route == "" {
		return nil
	}
	anchor := card.AnchorPath()
	if !b.DeclaresRoute(card.Route) {
		return []Finding{{Path: anchor, Key: FindingCardUnknownRoute, Detail: card.Ref(b.Slug) + " " + card.Route}}
	}
	route := b.DeclaredRouteOf(card)
	column := b.Column(card.Column)
	// A card standing at a column the workbench does not declare at all is
	// checkCard's own unknown-column finding, and it is not off its route as
	// well: there is no position left to ask the question about.
	if column == nil || RouteIndexOf(route, column) >= 0 {
		return nil
	}
	return []Finding{{Path: anchor, Key: FindingCardOffRoute, Detail: card.Ref(b.Slug) + " " + column.Ref()}}
}

// checkItemRoutes reports every pending checklist item of a card naming a
// column the card's route does not carry, which is a hold that would never
// fire. Pending is the state that matters: a resolved, verified or failed item
// holds nothing on entry and has nothing to strand.
//
// An item naming no column names nothing to be off the route, and a card on the
// default route walks every column, so neither reaches the report.
func (b *Bench) checkItemRoutes(card *Card) ([]Finding, error) {
	route := b.DeclaredRouteOf(card)
	if route == nil {
		return nil, nil
	}
	pending, err := itemsWhere(card.Dir, func(item *Item) bool {
		return item.Column != "" && item.State == ItemPending
	})
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, item := range pending {
		named := b.ColumnByRef(item.Column)
		// A column value that resolves to nothing is checkItemColumns'
		// finding, whose repair is a different one.
		if named == nil || RouteIndexOf(route, named) >= 0 {
			continue
		}
		findings = append(findings, Finding{
			Path:   filepath.Join(item.Dir, ItemAnchor),
			Key:    FindingItemOffRoute,
			Detail: card.Ref(b.Slug) + " " + item.ID + " " + named.Ref(),
		})
	}
	return findings, nil
}

// StrandedItemOf returns the first pending item of a card naming a column the
// named route does not carry, in identifier order, which is the convention the
// profile's own unresolved-item refusal already follows, together with the
// column that item names. It answers nil for a card stranding none.
//
// It is the write-side reading of check.item-off-route and sits beside it so
// that the refusal and the finding cannot come to mean two different things. An
// item naming a column the workbench does not declare at all is passed over
// here as it is there, because that is a misfiling with a repair of its own.
func (b *Bench) StrandedItemOf(card *Card, route string) (*Item, *Column, error) {
	ids, declared := b.Routes[route]
	if !declared {
		return nil, nil, nil
	}
	carried := RouteColumnsIn(ids, b.Columns)
	pending, err := itemsWhere(card.Dir, func(item *Item) bool {
		return item.Column != "" && item.State == ItemPending
	})
	if err != nil {
		return nil, nil, err
	}
	for _, item := range pending {
		named := b.ColumnByRef(item.Column)
		if named == nil || RouteIndexOf(carried, named) >= 0 {
			continue
		}
		return item, named, nil
	}
	return nil, nil, nil
}

// RouteCarries reports whether a card's own route carries a column, which is
// the one question the three route refusals and the two card findings all ask.
// A card on the default route walks every column the workbench declares, so it
// carries every one of them.
func (b *Bench) RouteCarries(card *Card, column *Column) bool {
	route := b.DeclaredRouteOf(card)
	if route == nil {
		return column != nil
	}
	return RouteIndexOf(route, column) >= 0
}

// RouteSkipsOperatorColumn returns the first operator-owned column a named
// route omits that stands at or after a card's current column in the flow
// order, and nil where it omits none.
//
// The rule is evaluated against the card's current position rather than against
// the whole flow, so a card that has already passed a station is not held back
// by it. A workbench declaring no operator-owned column is unaffected, and so
// is a route carrying every one of them ahead of the card.
func (b *Bench) RouteSkipsOperatorColumn(card *Card, name string) *Column {
	ids, declared := b.Routes[name]
	if !declared {
		return nil
	}
	route := RouteColumnsIn(ids, b.Columns)
	standing := b.Column(card.Column)
	for _, column := range b.Columns {
		if !column.OperatorOwned {
			continue
		}
		if standing != nil && column.Position < standing.Position {
			continue
		}
		if RouteIndexOf(route, column) < 0 {
			return column
		}
	}
	return nil
}

// RouteNamesFor is the roster a refusal lists back to a caller who named a
// route this workbench does not declare: the declared names in declaration
// order, which is the order a listing prints them.
func (b *Bench) RouteNamesFor() []string {
	return append([]string(nil), b.RouteNames...)
}
