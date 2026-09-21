package bench

import (
	"regexp"
	"sort"
	"strings"
)

// RoutesKey is the workbench frontmatter key carrying the declared routes, one
// nested block holding one entry per route name.
const RoutesKey = "routes"

// RouteField is the top-level key a card names its route under, and the name a
// reader types after `dinah set <card>`.
const RouteField = "route"

// routeName matches a route's own line inside the routes block: any key
// indented beneath the block's key at column one, up to its colon. It is
// deliberately wider than the route-name grammar. A name outside that grammar
// is kept and check reports it under check.route-name-malformed, which is what
// section 1.1 of the specification promises; a narrower pattern here would
// drop such a route without a trace, so neither a listing nor check could name
// it.
var routeName = regexp.MustCompile(`^\s+([^\s:#][^:#]*?):(.*)$`)

// routeEntry matches one dashed entry beneath a route name.
var routeEntry = regexp.MustCompile(`^\s*-\s*(.*)$`)

// readRoutes reads the routes block out of the raw frontmatter lines the way
// readLevels reads the levels block, rather than by introducing a YAML parser
// for one key.
//
// Both declared syntaxes are accepted and they mix freely across routes within
// one block, which is what the groups key beside it already admits: a flow
// sequence on the route's own line, and a block of dashed entries beneath it.
//
// Nothing is validated and nothing is dropped for being wrong. A name outside
// the slug grammar, a column the workbench does not declare, a column named
// twice and an order other than flow order are all reported by check, so the
// reader keeps what the file says and lets the checker speak. A duplicate is
// kept in particular, because dropping it here would take the finding with it.
//
// The second answer is the route names in declaration order, which is the
// order a listing prints them and a refusal names them back.
func readRoutes(fm *Frontmatter) (map[string][]string, []string) {
	routes := map[string][]string{}
	var order []string
	name := ""
	declare := func(route string) {
		if _, seen := routes[route]; seen {
			return
		}
		routes[route] = nil
		order = append(order, route)
	}
	for _, line := range fm.Raw(RoutesKey) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := routeEntry.FindStringSubmatch(line); m != nil {
			if name == "" {
				continue
			}
			if id := unquote(stripComment(m[1])); id != "" {
				routes[name] = append(routes[name], id)
			}
			continue
		}
		m := routeName.FindStringSubmatch(line)
		if m == nil {
			name = ""
			continue
		}
		name = m[1]
		declare(name)
		// A trailing comment is annotation for a person and is dropped
		// before the flow form is read, as it is on a columns entry.
		inline := unquote(stripComment(m[2]))
		if !strings.HasPrefix(inline, "[") || !strings.HasSuffix(inline, "]") {
			continue
		}
		for _, raw := range strings.Split(strings.Trim(inline, "[]"), ",") {
			if id := unquote(strings.TrimSpace(raw)); id != "" {
				routes[name] = append(routes[name], id)
			}
		}
	}
	return routes, order
}

// DeclaresRoute reports whether the workbench declares a route of this name.
func (b *Bench) DeclaresRoute(name string) bool {
	_, declared := b.Routes[name]
	return declared
}

// RouteOf returns the columns a card walks, in flow order: the live columns the
// card's declared route names, or every live column where the card names no
// route or names one this workbench does not declare.
//
// The answer is always ordered by each column's Position, whatever order the
// declaration listed them in, so the flow stays the single authority for order
// that CORE-STATE-6 makes it and no consumer can meet a route that doubles
// back. A declaration written out of flow order is reported by check rather
// than honoured.
//
// Sorting is also what keeps the route order-isomorphic to its image in the
// flow, which the loop-limit argument rests on: for any two columns a route
// carries, the route and the flow agree about which stands earlier. A later
// change honouring the declaration's own order would break that argument and
// would have to bring the regressive comparison back onto the route.
func (b *Bench) RouteOf(card *Card) []*Column {
	if route := b.DeclaredRouteOf(card); route != nil {
		return route
	}
	return b.Columns
}

// DeclaredRouteOf is RouteOf answering nil rather than the whole flow for a
// card walking the default route, which is what lets a caller tell a card that
// named a road from a card that named none. A card naming a route the workbench
// does not declare walks the default route, so it answers nil too.
func (b *Bench) DeclaredRouteOf(card *Card) []*Column {
	if card == nil || card.Route == "" {
		return nil
	}
	ids, declared := b.Routes[card.Route]
	if !declared {
		return nil
	}
	return RouteColumnsIn(ids, b.Columns)
}

// RouteColumnsIn resolves a route's declared identifiers against an explicit
// ordered column list, dropping an identifier that names no column of that list
// and keeping one column once however often the declaration names it.
//
// The answer is ordered by each column's place in the list it was resolved
// against, which is Position for the workbench's own flow and the placement
// comparison's own order for the flow a column-new placement would produce.
// Taking the order from the list rather than from the Position field is what
// lets the comparison run over a synthesized flow without every caller having
// to keep a cloned Position honest.
//
// The answer is never nil, so a declared route that resolves to no live column
// reads as the empty road it is rather than as a card naming no route at all.
// Such a card has no forward move and no pull destination, which is what
// check.route-empty exists to tell somebody about.
func RouteColumnsIn(ids []string, columns []*Column) []*Column {
	at := make(map[string]int, len(columns))
	for i, column := range columns {
		at[column.ID] = i
	}
	seen := make(map[string]bool, len(ids))
	carried := []*Column{}
	for _, id := range ids {
		index, live := at[id]
		if !live || seen[id] {
			continue
		}
		seen[id] = true
		carried = append(carried, columns[index])
	}
	sort.SliceStable(carried, func(i, j int) bool {
		return at[carried[i].ID] < at[carried[j].ID]
	})
	return carried
}

// RouteIndexOf returns the index of a column within a route, and -1 where the
// route does not carry it.
//
// It searches by identifier. Column.Position is a dense index over the flow and
// a route is a subsequence of the flow, so indexing a route slice by Position
// reads the wrong column wherever the route has dropped one, and that is the
// commonest way an implementation of routes goes wrong.
//
// No comparison of two columns is ever built out of this function's answer. A
// -1 is not a position, and a caller that compared it would read an off-route
// column as standing before everything.
func RouteIndexOf(route []*Column, column *Column) int {
	if column == nil {
		return -1
	}
	for i, carried := range route {
		if carried.ID == column.ID {
			return i
		}
	}
	return -1
}

// RoutePositionOf returns the index within a card's route from which its
// forward move is taken: the index of the column it stands in where the route
// carries that column, and otherwise the index of the first route column
// standing strictly after it in the flow, which is where the card rejoins its
// route. It returns len(route) where no route column stands after the card at
// all.
//
// It answers a forward move and nothing else. No comparison of two columns is
// built from it: two off-route columns between one pair of route columns share
// a rejoin index, so a move between them would read as no move.
func RoutePositionOf(route []*Column, column *Column) int {
	if at := RouteIndexOf(route, column); at >= 0 {
		return at
	}
	if column == nil {
		return len(route)
	}
	for i, carried := range route {
		if carried.Position > column.Position {
			return i
		}
	}
	return len(route)
}

// RouteForwardOf returns the column a card's route puts ahead of the column the
// card stands at, and nil where the route has none.
//
// The two branches are the two readings RoutePositionOf carries. Where the
// route carries the column, the forward move is the route's next column after
// it. Where it does not, RoutePositionOf has already answered the rejoin index,
// and the rejoin column itself is the forward move, because that is the first
// route column standing after the card in the flow's own order.
func RouteForwardOf(route []*Column, column *Column) *Column {
	at := RoutePositionOf(route, column)
	if RouteIndexOf(route, column) >= 0 {
		at++
	}
	if at < 0 || at >= len(route) {
		return nil
	}
	return route[at]
}

// RouteSkips returns the live columns a route does not carry, in flow order,
// which is what a listing names beside the route and the one question an
// operator asks of a road somebody drew.
func (b *Bench) RouteSkips(name string) []*Column {
	ids, declared := b.Routes[name]
	if !declared {
		return nil
	}
	carried := map[string]bool{}
	for _, column := range RouteColumnsIn(ids, b.Columns) {
		carried[column.ID] = true
	}
	var skipped []*Column
	for _, column := range b.Columns {
		if !carried[column.ID] {
			skipped = append(skipped, column)
		}
	}
	return skipped
}

// RemoveColumnIDFromRoutes drops one identifier from every declared route,
// rewriting the block the reader above parses, and reports whether any route
// carried it. It writes no file: the retirement's own write to the anchor
// carries the columns list and the routes together, which is what keeps the
// anchor from ever naming in a route a column whose directory is gone.
func (b *Bench) RemoveColumnIDFromRoutes(id string) bool {
	return b.removeFromRoutes(map[string]bool{id: true})
}

// removeFromRoutes is the body RemoveColumnIDFromRoutes and the stranded-column
// repair share, dropping every named identifier from every route in one pass
// and rewriting the block once.
func (b *Bench) removeFromRoutes(drop map[string]bool) bool {
	if len(b.Routes) == 0 || !b.FM.Has(RoutesKey) {
		return false
	}
	changed := false
	for _, name := range b.RouteNames {
		kept := make([]string, 0, len(b.Routes[name]))
		for _, existing := range b.Routes[name] {
			if drop[existing] {
				changed = true
				continue
			}
			kept = append(kept, existing)
		}
		b.Routes[name] = kept
	}
	if !changed {
		return false
	}
	b.FM.SetRaw(RoutesKey, dropFromRouteLines(b.FM.Raw(RoutesKey), drop))
	return true
}

// dropFromRouteLines removes identifiers from the routes block's own lines and
// touches nothing else, so a route that lost no entry reads exactly as it was
// written and every trailing comment a person put on an entry survives. A
// dashed entry naming a dropped identifier loses its line; a route written in
// flow form keeps its line, its name and its trailing comment, and loses the
// identifier from inside the brackets.
func dropFromRouteLines(lines []string, drop map[string]bool) []string {
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if m := routeEntry.FindStringSubmatch(line); m != nil {
			if drop[unquote(stripComment(m[1]))] {
				continue
			}
			kept = append(kept, line)
			continue
		}
		open, shut := strings.Index(line, "["), strings.LastIndex(line, "]")
		if routeName.MatchString(line) && open >= 0 && shut > open {
			var ids []string
			for _, raw := range strings.Split(line[open+1:shut], ",") {
				if id := strings.TrimSpace(raw); id != "" && !drop[unquote(id)] {
					ids = append(ids, id)
				}
			}
			line = line[:open+1] + strings.Join(ids, ", ") + line[shut:]
		}
		kept = append(kept, line)
	}
	return kept
}
