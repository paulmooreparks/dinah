package main

import (
	"strings"
	"testing"

	"dinah/internal/msg"
)

// routedCLIDefinition is a four-column flow with one short route through it,
// which is the least a command-level test of routes needs: a road that drops a
// station, and a station for it to drop.
const routedCLIDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Routed",
  "routes": { "short": ["a20000000001", "a20000000003", "a20000000004"] },
  "columns": [
    { "id": "a20000000001", "title": "Intake", "kind": "intake" },
    { "id": "a20000000002", "title": "Review", "kind": "work", "operator_owned": true },
    { "id": "a20000000003", "title": "Doing", "kind": "work" },
    { "id": "a20000000004", "title": "Done", "kind": "done" }
  ]
}`

// plainCLIDefinition is the same flow declaring no route at all.
const plainCLIDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Plain",
  "columns": [
    { "id": "a30000000001", "title": "Intake", "kind": "intake" },
    { "id": "a30000000002", "title": "Doing", "kind": "work" },
    { "id": "a30000000003", "title": "Done", "kind": "done" }
  ]
}`

// TestTheRouteReachesEveryCommandSurface is the command-level half of
// dinah-542/criteria/14. The routes listing names what each road skips and
// marks the operator's column, a card's line carries a route only where the
// card walks one, the query selects the carrying and the default-route cards,
// and the tree nests along the route axis.
//
// Every surface is read through the terminal head, because that is the one a
// person meets and the one whose rendering the library tests never reach.
func TestTheRouteReachesEveryCommandSurface(t *testing.T) {
	root := newBenchFromDefinition(t, routedCLIDefinition)
	en := msg.For(msg.Base)

	listed := runCLI(t, root, "list", "routes")
	if listed.code != 0 {
		t.Fatalf("list routes: %d %s", listed.code, listed.errw)
	}
	for _, want := range []string{
		en.T("column.routes.route"), en.T("column.routes.skips"), "short",
		en.T("routes.skip.operator-owned", "column", "review"),
	} {
		if !strings.Contains(listed.out, want) {
			t.Errorf("the routes listing does not carry %q:\n%s", want, listed.out)
		}
	}

	routed := runCLI(t, root, "add", "On the short road", "--route", "short")
	if routed.code != 0 {
		t.Fatalf("add --route: %d %s", routed.code, routed.errw)
	}
	plain := runCLI(t, root, "add", "On the whole flow")
	if plain.code != 0 {
		t.Fatalf("add: %d %s", plain.code, plain.errw)
	}
	line := en.T("card.route", "route", "short")
	if !strings.Contains(routed.out, line) {
		t.Errorf("the routed card's line does not carry %q:\n%s", line, routed.out)
	}
	if strings.Contains(plain.out, strings.TrimSpace(en.T("card.route", "route", ""))) {
		t.Errorf("a card walking the whole flow draws a route line:\n%s", plain.out)
	}
	if shown := runCLI(t, root, "show", "fx-1", "--fields", "card"); !strings.Contains(shown.out, line) {
		t.Errorf("show does not print the route line for the routed card:\n%s", shown.out)
	}

	carrying := runCLI(t, root, "query", "route:short")
	if carrying.code != 0 || !strings.Contains(carrying.out, "fx-1") || strings.Contains(carrying.out, "fx-2") {
		t.Errorf("route:short selected the wrong cards: %d\n%s%s", carrying.code, carrying.out, carrying.errw)
	}
	absent := runCLI(t, root, "query", `route:""`)
	if absent.code != 0 || !strings.Contains(absent.out, "fx-2") || strings.Contains(absent.out, "fx-1") {
		t.Errorf(`route:"" selected the wrong cards: %d\n%s%s`, absent.code, absent.out, absent.errw)
	}

	tree := runCLI(t, root, "tree", "--group-by", "route")
	if tree.code != 0 {
		t.Fatalf("tree --group-by route: %d %s", tree.code, tree.errw)
	}
	if !strings.Contains(tree.out, "short") {
		t.Errorf("the tree grouped by route does not name the route:\n%s", tree.out)
	}

	refused := runCLI(t, root, "set", "fx-2", "route", "nonesuch")
	if refused.code != 2 || !strings.Contains(refused.errw+refused.out, "dinah.unknown-route") {
		t.Errorf("an undeclared route was not refused by name: %d\n%s%s", refused.code, refused.out, refused.errw)
	}
}

// TestAWorkbenchDeclaringNoRouteListsNone asserts the empty listing, which says
// what having no route means rather than printing an empty table.
func TestAWorkbenchDeclaringNoRouteListsNone(t *testing.T) {
	root := newBenchFromDefinition(t, plainCLIDefinition)
	got := runCLI(t, root, "list", "routes")
	if got.code != 0 {
		t.Fatalf("list routes: %d %s", got.code, got.errw)
	}
	if want := msg.For(msg.Base).T("routes.empty"); !strings.Contains(got.out, want) {
		t.Errorf("an empty routes listing printed %q, wanted %q", got.out, want)
	}
	// A route named on a workbench declaring none is refused with the sentence
	// that says so, rather than with an empty list of names.
	refused := runCLI(t, root, "add", "A filing", "--route", "short")
	if refused.code != 2 || strings.Contains(refused.errw+refused.out, ": .") {
		t.Errorf("a route named on a workbench declaring none answered %d:\n%s%s", refused.code, refused.out, refused.errw)
	}
}
