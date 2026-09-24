package bench

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// routeFixture writes a workbench carrying a four-column flow and whatever
// routes block the case hands it, and opens it. The flow is intake, a station,
// a second station and a done column, which is short enough to read at a glance
// and long enough for a route to drop something from the middle.
//
// The routes block is written into the anchor as raw lines rather than passed
// through the interchange reader, because several cases below declare routes
// the reader would carry unchanged anyway and one declares a name the renderer
// refuses to write, and every case has to reach Open with exactly the text it
// names.
func routeFixture(t *testing.T, routes []string) *Bench {
	t.Helper()
	root := containedPath(filepath.Join(t.TempDir(), "workbench"))
	definition := `{
  "profile": "dinah-core/0.7",
  "title": "Routes",
  "columns": [
    { "id": "f00000000001", "title": "Intake", "kind": "intake" },
    { "id": "f00000000002", "title": "Doing", "kind": "work" },
    { "id": "f00000000003", "title": "Review", "kind": "work", "reject_to": "f00000000002" },
    { "id": "f00000000004", "title": "Done", "kind": "done" }
  ]
}`
	read, err := ReadDefinition([]byte(definition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := Instantiate(root, "rf", "alka", read); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if routes != nil {
		anchor := filepath.Join(root, WorkbenchAnchor)
		text, err := ReadText(anchor)
		if err != nil {
			t.Fatalf("read the anchor: %v", err)
		}
		fm, body := ParseAnchor(text)
		fm.SetRaw(RoutesKey, append([]string{RoutesKey + ":"}, routes...))
		if err := WriteText(anchor, fm.Render(body)); err != nil {
			t.Fatalf("write the anchor: %v", err)
		}
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return opened
}

// routeFindings is every finding check reports whose key names a route or a
// card's road, which is what the cases below are about, so a finding some other
// rule reports over the same fixture cannot pass for one of them.
func routeFindings(t *testing.T, b *Bench) []Finding {
	t.Helper()
	all, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	var kept []Finding
	for _, finding := range all {
		if strings.Contains(finding.Key, "route") {
			kept = append(kept, finding)
		}
	}
	return kept
}

// TestEveryRouteFindingHasAFixtureThatProducesItAndOneThatDoesNot is
// dinah-542/criteria/11 and dinah-542/criteria/20. Each of the eight findings
// check reports over a declaration is reached by a routes block that produces
// it and missed by one that does not, and the workbench opens in every case,
// because a route declaration is never refused on read.
//
// The sweep states its own size so a table that lost a row reads as a failure
// rather than as a smaller pass.
func TestEveryRouteFindingHasAFixtureThatProducesItAndOneThatDoesNot(t *testing.T) {
	clean := []string{"  good:", "    - f00000000001", "    - f00000000002", "    - f00000000004"}
	cases := []struct {
		key      string
		produces []string
		detail   string
	}{
		{FindingRouteNameMalformed, []string{"  Not_A_Slug:", "    - f00000000001", "    - f00000000004"}, "Not_A_Slug"},
		{FindingRouteEmpty, []string{"  nothing:"}, "nothing"},
		{FindingRouteUnknownColumn, []string{"  stale:", "    - f00000000001", "    - f0000000dead", "    - f00000000004"}, "stale f0000000dead"},
		{FindingRouteDuplicateColumn, []string{"  twice:", "    - f00000000001", "    - f00000000002", "    - f00000000002", "    - f00000000004"}, "twice doing"},
		{FindingRouteOutOfOrder, []string{"  backward:", "    - f00000000001", "    - f00000000004", "    - f00000000002"}, "backward"},
		{FindingRouteMissingFirstColumn, []string{"  doorless:", "    - f00000000002", "    - f00000000004"}, "doorless intake"},
		{FindingRouteMissingTerminal, []string{"  endless:", "    - f00000000001", "    - f00000000002"}, "endless doing"},
		// Review is carried and rejects to Doing, which this route drops.
		{FindingRouteRejectTargetOffRoute, []string{"  lossy:", "    - f00000000001", "    - f00000000003", "    - f00000000004"}, "lossy review doing"},
	}
	if len(cases) != 8 {
		t.Fatalf("the table carries %d cases and check declares eight findings over a declaration", len(cases))
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			produced := routeFindings(t, routeFixture(t, c.produces))
			found := false
			for _, finding := range produced {
				if finding.Key == c.key && finding.Detail == c.detail {
					found = true
				}
			}
			if !found {
				t.Errorf("the fixture produced %+v, and none of it is %s naming %q", produced, c.key, c.detail)
			}
			for _, finding := range routeFindings(t, routeFixture(t, clean)) {
				if finding.Key == c.key {
					t.Errorf("a clean route produced %s: %+v", c.key, finding)
				}
			}
		})
	}

	// A column declaring no reject_to is passed over, which the clean route
	// above already shows, and a route whose every column rejects onto the
	// route reports nothing: Review rejects to Doing and this route carries
	// both.
	carrying := routeFindings(t, routeFixture(t, []string{"  whole:", "    - f00000000001", "    - f00000000002", "    - f00000000003", "    - f00000000004"}))
	for _, finding := range carrying {
		if finding.Key == FindingRouteRejectTargetOffRoute {
			t.Errorf("a route carrying every reject target reported %+v", finding)
		}
	}
}

// TestEveryCardRouteFindingHasAFixtureThatProducesItAndOneThatDoesNot is the
// card half of dinah-542/criteria/11. A card naming an undeclared route opens,
// walks the whole flow, and is reported; a card standing off its road is
// reported; and a pending item naming a column the road drops is reported.
// Each has an accepting fixture beside it.
func TestEveryCardRouteFindingHasAFixtureThatProducesItAndOneThatDoesNot(t *testing.T) {
	routes := []string{"  short:", "    - f00000000001", "    - f00000000003", "    - f00000000004"}

	t.Run(FindingCardUnknownRoute, func(t *testing.T) {
		b := routeFixture(t, routes)
		card := plantRoutedCard(t, b, "c00000000001", 1, "f00000000001", "nonesuch")
		if !hasRouteFinding(t, b, FindingCardUnknownRoute, "rf-1 nonesuch") {
			t.Error("a card naming an undeclared route is not reported")
		}
		if got := len(b.RouteOf(card)); got != len(b.Columns) {
			t.Errorf("the card naming an undeclared route walks %d columns, wanted the whole flow of %d", got, len(b.Columns))
		}
		other := routeFixture(t, routes)
		plantRoutedCard(t, other, "c00000000001", 1, "f00000000001", "short")
		if hasRouteFinding(t, other, FindingCardUnknownRoute, "") {
			t.Error("a card naming a declared route is reported")
		}
	})

	t.Run(FindingCardOffRoute, func(t *testing.T) {
		b := routeFixture(t, routes)
		plantRoutedCard(t, b, "c00000000001", 1, "f00000000002", "short")
		if !hasRouteFinding(t, b, FindingCardOffRoute, "rf-1 doing") {
			t.Error("a card standing at a column its road drops is not reported")
		}
		other := routeFixture(t, routes)
		plantRoutedCard(t, other, "c00000000001", 1, "f00000000003", "short")
		if hasRouteFinding(t, other, FindingCardOffRoute, "") {
			t.Error("a card standing on its road is reported")
		}
	})

	t.Run(FindingCardRouteSkipsOperatorColumn, func(t *testing.T) {
		// Review is reserved to the operator here, and short drops it. A card
		// at Intake has not passed Review, so a hand-written route key
		// carrying it around the column is reported; the same card standing
		// at Done, past the column, is not.
		reserve := func(b *Bench) *Bench {
			anchor := b.ColumnAnchorPath("f00000000003")
			text, err := ReadText(anchor)
			if err != nil {
				t.Fatalf("read review: %v", err)
			}
			fm, body := ParseAnchor(text)
			fm.Set("operator_owned", "true")
			if err := WriteText(anchor, fm.Render(body)); err != nil {
				t.Fatalf("write review: %v", err)
			}
			opened, err := Open(b.Root)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			return opened
		}
		skipping := []string{"  short:", "    - f00000000001", "    - f00000000002", "    - f00000000004"}
		b := reserve(routeFixture(t, skipping))
		plantRoutedCard(t, b, "c00000000001", 1, "f00000000001", "short")
		if !hasRouteFinding(t, b, FindingCardRouteSkipsOperatorColumn, "rf-1 review") {
			t.Error("a card whose route carries it around a reserved column is not reported")
		}
		other := reserve(routeFixture(t, skipping))
		plantRoutedCard(t, other, "c00000000001", 1, "f00000000004", "short")
		if hasRouteFinding(t, other, FindingCardRouteSkipsOperatorColumn, "") {
			t.Error("a card already past the reserved column is reported")
		}
	})

	t.Run(FindingItemOffRoute, func(t *testing.T) {
		b := routeFixture(t, routes)
		plantRoutedCard(t, b, "c00000000001", 1, "f00000000001", "short")
		plantItemColumn(t, b.Root, "c00000000001", "d00000000001", "f00000000002")
		if !hasRouteFinding(t, b, FindingItemOffRoute, "rf-1 d00000000001 doing") {
			t.Error("a pending item naming a column the road drops is not reported")
		}
		other := routeFixture(t, routes)
		plantRoutedCard(t, other, "c00000000001", 1, "f00000000001", "short")
		plantItemColumn(t, other.Root, "c00000000001", "d00000000001", "f00000000003")
		if hasRouteFinding(t, other, FindingItemOffRoute, "") {
			t.Error("an item naming a column the road carries is reported")
		}
	})
}

// plantRoutedCard writes one card standing at a column and naming a route,
// registers its number, and returns it as the reopened bench reads it.
func plantRoutedCard(t *testing.T, b *Bench, id string, number int, column, route string) *Card {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set(TitleField, "A routed card")
	fm.Set("column", column)
	fm.Set("state", contract.StateReady)
	fm.Set(RouteField, route)
	write(t, filepath.Join(b.Root, CardsDir, id, CardAnchor), fm.Render("Framing.\n"))
	write(t, filepath.Join(b.Root, CardsDir, id, JournalName), "")
	// A fresh workbench carries no registry until its first filing, so the
	// line is written rather than appended where the file is not there yet.
	registry := filepath.Join(b.Root, CardNumbersName)
	existing, _ := ReadText(registry)
	write(t, registry, existing+strconv.Itoa(number)+" "+id+"\n")
	b.ReloadNumbers()
	card, err := b.LoadCardIn(b.CardsRoot(), id)
	if err != nil {
		t.Fatalf("load the planted card: %v", err)
	}
	return card
}

// hasRouteFinding reports whether check reports one key, naming one detail
// where the case names one, over a workbench.
func hasRouteFinding(t *testing.T, b *Bench, key, detail string) bool {
	t.Helper()
	for _, finding := range routeFindings(t, b) {
		if finding.Key == key && (detail == "" || finding.Detail == detail) {
			return true
		}
	}
	return false
}

// TestARouteResolvesInFlowOrder is dinah-542/criteria/12. A route declared out
// of flow order resolves to its columns in flow order, so no consumer meets a
// road that doubles back, and it resolves identically to the same route written
// in order.
func TestARouteResolvesInFlowOrder(t *testing.T) {
	backward := routeFixture(t, []string{"  r:", "    - f00000000004", "    - f00000000003", "    - f00000000001"})
	forward := routeFixture(t, []string{"  r:", "    - f00000000001", "    - f00000000003", "    - f00000000004"})
	card := &Card{Route: "r", Column: "f00000000001"}
	b, f := backward.RouteOf(card), forward.RouteOf(card)
	if len(b) != 3 || len(f) != 3 {
		t.Fatalf("the two routes resolved to %d and %d columns, wanted three each", len(b), len(f))
	}
	for at := range b {
		if b[at].ID != f[at].ID {
			t.Fatalf("the route written backward resolves to %v and written forward to %v", idsOf(b), idsOf(f))
		}
		if at > 0 && b[at].Position <= b[at-1].Position {
			t.Fatalf("the resolved route %v doubles back", idsOf(b))
		}
	}
	if got := RouteForwardOf(b, b[0]); got == nil || got.ID != "f00000000003" {
		t.Errorf("the forward move from intake along the backward-written route is %v, wanted review", got)
	}
}

// idsOf names a run of columns for a failure message.
func idsOf(columns []*Column) []string {
	ids := make([]string, 0, len(columns))
	for _, column := range columns {
		ids = append(ids, column.ID)
	}
	return ids
}

// TestRetiringAColumnDropsItFromEveryRoute is dinah-542/criteria/8. The write
// that takes a column out of the workbench's list takes it out of every route in
// the same write, and restoring the column returns it to the list and to no
// route.
func TestRetiringAColumnDropsItFromEveryRoute(t *testing.T) {
	b := routeFixture(t, []string{
		"  one:", "    - f00000000001", "    - f00000000002", "    - f00000000004",
		"  two:", "    - f00000000001", "    - f00000000002", "    - f00000000003", "    - f00000000004",
	})
	if err := b.RemoveColumnID("f00000000002"); err != nil {
		t.Fatalf("retire: %v", err)
	}
	reopened, err := Open(b.Root)
	if err == nil {
		for _, name := range reopened.RouteNames {
			for _, id := range reopened.Routes[name] {
				if id == "f00000000002" {
					t.Errorf("the route %s still names the retired column", name)
				}
			}
		}
		if len(reopened.Routes["one"]) != 2 || len(reopened.Routes["two"]) != 3 {
			t.Errorf("the routes read back %v, wanted each to lose exactly the retired column", reopened.Routes)
		}
	} else {
		// The column's directory is still here, since this case drives the
		// definition write alone; the anchor is read straight off the disk.
		t.Logf("reopen: %v", err)
	}
	text, err := ReadText(filepath.Join(b.Root, WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	fm, _ := ParseAnchor(text)
	if strings.Contains(strings.Join(fm.Raw(RoutesKey), "\n"), "f00000000002") {
		t.Errorf("the anchor's routes block still names the retired column:\n%s", strings.Join(fm.Raw(RoutesKey), "\n"))
	}
	for _, id := range fm.Seq("columns") {
		if id == "f00000000002" {
			t.Error("the anchor's columns list still names the retired column")
		}
	}

	if err := b.AddColumnID("f00000000002"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	text, err = ReadText(filepath.Join(b.Root, WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	fm, _ = ParseAnchor(text)
	if strings.Contains(strings.Join(fm.Raw(RoutesKey), "\n"), "f00000000002") {
		t.Error("restoring the column put it back into a route, and a route is a choice somebody made")
	}
	listed := false
	for _, id := range fm.Seq("columns") {
		if id == "f00000000002" {
			listed = true
		}
	}
	if !listed {
		t.Error("restoring the column did not return it to the columns list")
	}
}

// TestRoutesTravelThroughInterchangeUntouched is dinah-542/criteria/9. A
// workbench declaring routes exports them as a member of the object, importing
// that object back and exporting again gives the same bytes, and a reader that
// knows nothing of routes preserves both the workbench's member and a card's
// key, by the generic pass the interchange already runs for every member it does
// not recognise.
func TestRoutesTravelThroughInterchangeUntouched(t *testing.T) {
	b := routeFixture(t, []string{"  short:", "    - f00000000001", "    - f00000000003", "    - f00000000004"})
	first, err := b.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(first, &object); err != nil {
		t.Fatalf("the export is not an object: %v", err)
	}
	var routes map[string][]string
	if err := json.Unmarshal(object[RoutesKey], &routes); err != nil {
		t.Fatalf("the routes member does not read as an object of arrays: %s", object[RoutesKey])
	}
	if strings.Join(routes["short"], ",") != "f00000000001,f00000000003,f00000000004" {
		t.Errorf("the routes member reads %v", routes)
	}

	// A reader that has never heard of routes reads the object as JSON and
	// writes it back untouched, which is all CORE-JSON-7 asks of it.
	var untouched any
	if err := json.Unmarshal(first, &untouched); err != nil {
		t.Fatalf("reread: %v", err)
	}
	carried, err := json.Marshal(untouched)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	read, err := ReadDefinition(carried)
	if err != nil {
		t.Fatalf("read the carried object: %v", err)
	}
	root := containedPath(filepath.Join(t.TempDir(), "clone"))
	if err := Instantiate(root, "rf", "alka", read); err != nil {
		t.Fatalf("instantiate the clone: %v", err)
	}
	clone, err := Open(root)
	if err != nil {
		t.Fatalf("open the clone: %v", err)
	}
	second, err := clone.Export()
	if err != nil {
		t.Fatalf("export the clone: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("the second export differs from the first:\n%s\n---\n%s", first, second)
	}
	if strings.Join(clone.Routes["short"], ",") != "f00000000001,f00000000003,f00000000004" {
		t.Errorf("the clone reads the route back as %v", clone.Routes)
	}

	// A card's route key rides in the frontmatter as raw lines a write of its
	// neighbours puts back, which is CORE-CARD-9.
	card := plantRoutedCard(t, b, "c00000000001", 1, "f00000000001", "short")
	card.Severity = "minor"
	if err := card.Save(); err != nil {
		t.Fatalf("save the card: %v", err)
	}
	reloaded, err := b.LoadCardIn(b.CardsRoot(), card.ID)
	if err != nil {
		t.Fatalf("reload the card: %v", err)
	}
	if reloaded.Route != "short" {
		t.Errorf("a write of the card's severity dropped its route, which now reads %q", reloaded.Route)
	}
}

// TestTheStorageFormatDoesNotMoveForRoutes is dinah-542/criteria/16. Routes
// are additive and optional, so declaring them moves the storage number no
// further, and a workbench declaring routes opens on a build stamped with the
// current number with no migration, repair or finding about its format.
//
// The number it names is the one dinah-590 moved it to for a reason of its
// own, which is a declaration ceasing to apply to a card. That is what the
// pin below reads: this case asserts that routes moved it no further, so the
// pin travels with every later move rather than asserting a number routes
// never had anything to do with.
func TestTheStorageFormatDoesNotMoveForRoutes(t *testing.T) {
	if StorageFormat != AppliesWhenFormat {
		t.Fatalf("the storage format is %d, and routes are additive and optional, so it does not move past the %d dinah-590 left it at", StorageFormat, AppliesWhenFormat)
	}
	// The route drops Review, whose reject_to names Doing, and carries Doing,
	// which declares none, so no finding of any kind is owed.
	b := routeFixture(t, []string{"  short:", "    - f00000000001", "    - f00000000002", "    - f00000000004"})
	if b.Format != StorageFormat {
		t.Errorf("the workbench declares format %d on a build stamped %d", b.Format, StorageFormat)
	}
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("a clean workbench declaring routes reports %+v", findings)
	}
}

// TestARouteDeclarationReadsTheWayItIsWritten covers three findings code review
// raised on dinah-542's reader and writer. A trailing comment on a flow-form
// route is annotation and not part of the list. A route name outside the
// grammar is kept and reported rather than dropped without a trace. And a
// retirement removes the retired column from the routes and leaves every
// other line of the block, comments included, exactly as it was written.
func TestARouteDeclarationReadsTheWayItIsWritten(t *testing.T) {
	t.Run("a trailing comment on a flow-form route", func(t *testing.T) {
		b := routeFixture(t, []string{"  commented: [f00000000001, f00000000002, f00000000004]   # annotated"})
		if got := strings.Join(b.Routes["commented"], ","); got != "f00000000001,f00000000002,f00000000004" {
			t.Errorf("the route reads %q, and a trailing comment is not part of it", got)
		}
		if hasRouteFinding(t, b, FindingRouteEmpty, "") {
			t.Error("check calls the commented route empty")
		}
	})

	t.Run("a name outside the grammar is reported rather than dropped", func(t *testing.T) {
		b := routeFixture(t, []string{"  my route:", "    - f00000000001", "    - f00000000004"})
		if !b.DeclaresRoute("my route") {
			t.Fatalf("the reader dropped the route; it read %v", b.RouteNames)
		}
		if !hasRouteFinding(t, b, FindingRouteNameMalformed, "my route") {
			t.Error("check does not report the name outside the grammar")
		}
	})

	t.Run("a retirement leaves the rest of the block as written", func(t *testing.T) {
		b := routeFixture(t, []string{
			"  dashed:",
			"    - f00000000001   # Intake",
			"    - f00000000002   # Doing",
			"    - f00000000004   # Done",
			"  flow: [f00000000001, f00000000002, f00000000004]   # kept",
			"  untouched:",
			"    - f00000000001   # Intake",
			"    - f00000000004   # Done",
		})
		if err := b.RemoveColumnID("f00000000002"); err != nil {
			t.Fatalf("retire: %v", err)
		}
		text, err := ReadText(filepath.Join(b.Root, WorkbenchAnchor))
		if err != nil {
			t.Fatalf("read the anchor: %v", err)
		}
		fm, _ := ParseAnchor(text)
		got := strings.Join(fm.Raw(RoutesKey), "\n")
		want := strings.Join([]string{
			"routes:",
			"  dashed:",
			"    - f00000000001   # Intake",
			"    - f00000000004   # Done",
			"  flow: [f00000000001, f00000000004]   # kept",
			"  untouched:",
			"    - f00000000001   # Intake",
			"    - f00000000004   # Done",
		}, "\n")
		if got != want {
			t.Errorf("the routes block after retirement reads\n%s\nwanted\n%s", got, want)
		}
	})
}
