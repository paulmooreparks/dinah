package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// tieredHarness is newHarness with a tiers table and a tier axis written onto
// the workbench anchor, which is what turns the claim gate on. The table lists
// one model at each of three rungs, so a test says what a caller is by naming
// a rung and letting the workbench resolve it.
//
// It drives CORE-CAP-1 and CORE-CAP-2 alongside the cases below.
func tieredHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	anchor := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw("levels", []string{"levels:", "  tier: [minimal, workhorse, frontier]"})
	fm.SetRaw("tiers", []string{
		"tiers:",
		"  minimal:",
		"    meaning: mechanical edits",
		"    models:",
		"      - {provider: acme, model: minimal}",
		"  workhorse:",
		"    meaning: scoped implementation against a contract",
		"    models:",
		"      - {provider: acme, model: workhorse}",
		"  frontier:",
		"    meaning: novel design judgement",
		"    models:",
		"      - {provider: acme, model: frontier}",
	})
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
	return h
}

// runningAs is the request an agent at one rung makes, which is a provider and
// a model rather than a declaration of the rung itself. A rung of the empty
// string declares no model at all.
func runningAs(actor, tier string) *Request {
	req := &Request{Actor: actor}
	if tier != "" {
		req.Provider = "acme"
		req.Model = tier
	}
	return req
}

// claimAs claims one card as an owner running at one rung.
func claimAs(h *harness, actor, tier, ref string) *Response {
	h.t.Helper()
	req := runningAs(actor, tier)
	req.Verb = Claim
	req.Card = ref
	response := h.library.Do(req)
	h.reopen()
	return response
}

// requiring stands a card at the aftercare station and gives it a baseline
// requirement, which is the card every gate case below claims.
func requiring(h *harness, title, tier string) string {
	h.t.Helper()
	ref := h.ready(title)
	response := h.library.SetField(&Request{Verb: "set", Actor: "alka", Ref: ref, Field: bench.TierField, Value: tier})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("set the tier on %s: %s %s", ref, response.Outcome, response.Refusal)
	}
	h.reopen()
	return ref
}

// TestTheGateAdmitsAResolvedTierAndRefusesEachWayItCanFail drives CORE-CAP-3,
// CORE-CAP-4 and CORE-CAP-5 together with dinah-496's three refusal criteria.
// Each card below requires a capability at the column it stands in, which is
// what CORE-CAP-3 permits, and the four claims are what the other two govern.
//
// Four claims run against one workbench. A caller whose model the table lists
// at the requirement is admitted. One listed below it is refused under
// dinah.below-tier. One the table lists nowhere is refused under
// dinah.unlisted-model. One declaring no model at all is refused under
// dinah.undeclared-model. The identical calls then succeed on a card requiring
// nothing at its column, which is what keeps each refusal from being a refusal
// every value satisfies.
func TestTheGateAdmitsAResolvedTierAndRefusesEachWayItCanFail(t *testing.T) {
	h := tieredHarness(t)
	admitted := requiring(h, "admitted", "workhorse")
	below := requiring(h, "below", "frontier")
	unlisted := requiring(h, "unlisted", "frontier")
	undeclared := requiring(h, "undeclared", "frontier")
	open := h.ready("nobody has assessed this one")

	if got := claimAs(h, "brin", "workhorse", admitted); got.Outcome != contract.OutcomeOK {
		t.Errorf("a caller the table lists at the requirement was refused: %s %s", got.Outcome, got.Refusal)
	}
	refusals := map[string]string{
		below:      contract.BelowTier,
		unlisted:   contract.UnlistedModel,
		undeclared: contract.UndeclaredModel,
	}
	declared := map[string]string{below: "workhorse", unlisted: "unknown-model", undeclared: ""}
	for ref, want := range refusals {
		got := claimAs(h, "brin", declared[ref], ref)
		if got.Outcome != contract.OutcomeRefused {
			t.Errorf("%s: the claim answered %s, wanted refused", want, got.Outcome)
			continue
		}
		if got.Refusal != want {
			t.Errorf("the claim refused under %s, wanted %s", got.Refusal, want)
		}
	}
	// The same three callers on a card asking for nothing at its column.
	for _, tier := range []string{"workhorse", "unknown-model", ""} {
		h := tieredHarness(t)
		open = h.ready("nobody has assessed this one")
		if got := claimAs(h, "brin", tier, open); got.Outcome != contract.OutcomeOK {
			t.Errorf("a caller running %q was refused on a card asking for nothing: %s %s", tier, got.Outcome, got.Refusal)
		}
	}
}

// TestAWorkbenchWithNoTableRefusesNobodyOnTierGrounds drives dinah-496's
// no-table criterion. The workbench declares a tier axis and its card carries a
// requirement, and a caller declaring nothing claims it and succeeds, because a
// table is how a workbench asks for the gate and this one has not asked.
func TestAWorkbenchWithNoTableRefusesNobodyOnTierGrounds(t *testing.T) {
	h := tieredHarness(t)
	anchor := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.Delete("tiers")
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
	if h.library.Bench.DeclaresTierTable() {
		t.Fatal("the fixture still declares a table, so this case is not the workbench it names")
	}
	ref := requiring(h, "assessed", "frontier")
	if got := claimAs(h, "brin", "", ref); got.Outcome != contract.OutcomeOK {
		t.Errorf("a workbench declaring no table refused a claim on tier grounds: %s %s", got.Outcome, got.Refusal)
	}
}

// TestTheOperatorIsTheOneExemptionAndItIsTheName drives dinah-496's exemption
// criterion. The workbench's own operator claims a card requiring the top rung
// while declaring nothing and succeeds; a caller of any other name making the
// identical call is refused; and the operator declaring a model the table lists
// below the requirement is admitted too, so the exemption is the name rather
// than the declaration.
func TestTheOperatorIsTheOneExemptionAndItIsTheName(t *testing.T) {
	h := tieredHarness(t)
	if h.library.Bench.Operator != "alka" {
		t.Fatalf("the fixture's operator is %q, and this case is written for alka", h.library.Bench.Operator)
	}
	bare := requiring(h, "for the operator", "frontier")
	if got := claimAs(h, "alka", "", bare); got.Outcome != contract.OutcomeOK {
		t.Errorf("the operator declaring nothing was refused: %s %s", got.Outcome, got.Refusal)
	}

	other := requiring(h, "for anybody else", "frontier")
	refused := claimAs(h, "brin", "", other)
	if refused.Outcome != contract.OutcomeRefused || refused.Refusal != contract.UndeclaredModel {
		t.Errorf("a caller who is not the operator answered %s %s, wanted refused under %s",
			refused.Outcome, refused.Refusal, contract.UndeclaredModel)
	}

	lower := requiring(h, "the operator running low", "frontier")
	if got := claimAs(h, "alka", "minimal", lower); got.Outcome != contract.OutcomeOK {
		t.Errorf("the operator running a model listed below the requirement was refused: %s %s", got.Outcome, got.Refusal)
	}
}

// TestEachTierRefusalCarriesTheFactsARepairNeeds drives dinah-496's refusal
// context criterion. Each of the three names carries required and satisfied_by
// as strings, the entries render as provider/model or provider/model@server,
// they are joined by a comma and a space, and the two refusals raised by a
// caller that did declare a model carry that declaration under model where the
// third carries no model key at all.
func TestEachTierRefusalCarriesTheFactsARepairNeeds(t *testing.T) {
	h := tieredHarness(t)
	cases := []struct {
		name  string
		tier  string
		model string
	}{
		{contract.BelowTier, "workhorse", "acme/workhorse"},
		{contract.UnlistedModel, "unknown-model", "acme/unknown-model"},
		{contract.UndeclaredModel, "", ""},
	}
	for _, want := range cases {
		ref := requiring(h, "for "+want.name, "frontier")
		got := claimAs(h, "brin", want.tier, ref)
		if got.Outcome != contract.OutcomeRefused || got.Refusal != want.name {
			t.Fatalf("the claim answered %s %s, wanted refused under %s", got.Outcome, got.Refusal, want.name)
		}
		if got.Context["required"] != "frontier" {
			t.Errorf("%s carries required %q, wanted frontier", want.name, got.Context["required"])
		}
		if got.Context["satisfied_by"] != "acme/frontier" {
			t.Errorf("%s carries satisfied_by %q, wanted acme/frontier", want.name, got.Context["satisfied_by"])
		}
		if got.Context["column"] == "" {
			t.Errorf("%s names no column", want.name)
		}
		if got.Context["model"] != want.model {
			t.Errorf("%s carries model %q, wanted %q", want.name, got.Context["model"], want.model)
		}
	}

	// The rendering carries the server where the entry declares one, and the
	// list is joined by a comma and a space in the same order the offer uses.
	models := []bench.TierModel{
		{Provider: "anthropic", Model: "claude-sonnet-5"},
		{Provider: "ollama", Model: "qwen3:235b", Server: "ollama.com"},
	}
	if got := bench.RenderTierModels(models); got != "anthropic/claude-sonnet-5, ollama/qwen3:235b@ollama.com" {
		t.Errorf("the entries render as %q", got)
	}
}

// TestAnOfferAboveTheTierNamesTheRequirementAndWhatWouldSatisfyIt drives
// dinah-496's offer criterion. An offer withholding work above the caller's
// resolved tier carries required_tier and satisfied_by, with satisfied_by
// listing every table entry at or above that tier in levels.tier order from the
// lowest satisfying rung upward, and an offer withholding nothing carries
// neither member.
func TestAnOfferAboveTheTierNamesTheRequirementAndWhatWouldSatisfyIt(t *testing.T) {
	h := tieredHarness(t)
	requiring(h, "assessed", "workhorse")
	offers, err := h.library.Next(runningWithVerb("brin", "minimal"))
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	withheld := offerAt(t, offers, aftercare)
	if !withheld.AboveTier {
		t.Fatalf("the offer does not withhold the card: %+v", withheld)
	}
	if withheld.RequiredTier != "workhorse" {
		t.Errorf("the offer names %q as the requirement, wanted workhorse", withheld.RequiredTier)
	}
	rendered := make([]string, 0, len(withheld.SatisfiedBy))
	for _, model := range withheld.SatisfiedBy {
		rendered = append(rendered, model.Provider+"/"+model.Model)
	}
	if strings.Join(rendered, ", ") != "acme/workhorse, acme/frontier" {
		t.Errorf("satisfied_by is %v, wanted the cheapest satisfying rung first", rendered)
	}

	// An offer that withholds nothing carries neither member.
	admitted, err := h.library.Next(runningWithVerb("brin", "frontier"))
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	taken := offerAt(t, admitted, aftercare)
	if taken.Card == nil {
		t.Fatalf("the admitted caller was offered nothing: %+v", taken)
	}
	if taken.RequiredTier != "" || len(taken.SatisfiedBy) != 0 {
		t.Errorf("an offer withholding nothing explains a withholding: %+v", taken)
	}
}

// runningWithVerb is runningAs with the verb a read names.
func runningWithVerb(actor, tier string) *Request {
	req := runningAs(actor, tier)
	req.Verb = "next"
	return req
}

// offerAt picks one column's offer out of an answer and fails when the column
// is absent, so a silent miss cannot pass as an empty offer.
func offerAt(t *testing.T, offers []Offer, column string) Offer {
	t.Helper()
	for _, offer := range offers {
		if offer.Column == column {
			return offer
		}
	}
	t.Fatalf("the answer carries no offer for %s: %+v", column, offers)
	return Offer{}
}

// workbenchPackage is the name the workbench package is imported under, which
// the walks below compare a selector's qualifier against. It is a constant so
// that the spelling appears once, beside the sentence that says what it is.
const workbenchPackage = "bench" // retired spelling, named deliberately

// TestNoEventIsBuiltWithAnActorComposedAnywhereElse drives dinah-496's
// composition criterion. A static walk over internal/verb and internal/bench
// outside tests asserts three things: every bench.Event composite literal names
// Actor, every such literal takes it from Request.Acting or bench.NamedActor,
// and no assignment to an Actor field happens outside those two composers.
//
// The walk resolves an element's type from the composite literal enclosing it,
// because an element of a slice, array or map of events carries no type node of
// its own. The first draft of this guard read each literal's own Type node
// alone, and Agent Code Review defeated it by replacing one event with a
// one-element slice literal composing the actor by hand: the build was clean,
// the guard passed, and the population fell by one unnoticed.
//
// The count is pinned at the number the walk finds rather than at a floor
// under it, which is the other half of the same defect. A floor three quarters
// below the population is not the assertion this workbench's discipline asks
// for when a check sweeps a set, because the set can shrink to the floor
// without a word. Moving a construction site is therefore an edit to this
// number in the same commit, which is the point.
func TestNoEventIsBuiltWithAnActorComposedAnywhereElse(t *testing.T) {
	// constructionSites is how many bench.Event literals the two packages carry
	// between them. It is exact: a literal added or removed fails here until
	// somebody writes the new number down, which is what stops the population
	// shrinking quietly.
	const constructionSites = 52
	// actorAssignments is how many assignments to an Actor field the two
	// packages carry, which is none: every event names its actor in the literal
	// that builds it.
	const actorAssignments = 0

	literals, assignments := 0, 0
	for _, dir := range []string{filepath.Join("..", "verb"), filepath.Join("..", workbenchPackage)} {
		fileset := token.NewFileSet()
		packages, err := parser.ParseDir(fileset, dir, func(info os.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", dir, err)
		}
		for _, pkg := range packages {
			for path, file := range pkg.Files {
				ast.Inspect(file, func(node ast.Node) bool {
					switch found := node.(type) {
					case *ast.CompositeLit:
						for _, built := range eventLiteralsIn(found) {
							literals++
							actor, named := actorElement(built)
							if !named {
								t.Errorf("%s:%d builds an event naming no actor", path, fileset.Position(built.Pos()).Line)
								continue
							}
							if !composedByAComposer(actor) {
								t.Errorf("%s:%d composes an actor outside the two composers this card names",
									path, fileset.Position(built.Pos()).Line)
							}
						}
					case *ast.AssignStmt:
						for _, target := range found.Lhs {
							selector, ok := target.(*ast.SelectorExpr)
							if !ok || selector.Sel.Name != "Actor" {
								continue
							}
							assignments++
							if len(found.Rhs) != 1 || !composedByAComposer(found.Rhs[0]) {
								t.Errorf("%s:%d assigns an actor composed outside the two composers",
									path, fileset.Position(found.Pos()).Line)
							}
						}
					}
					return true
				})
			}
		}
	}
	if literals != constructionSites {
		t.Errorf("the walk found %d event literals and this guard is written against %d; a construction site moved, so read the diff and write the new number here",
			literals, constructionSites)
	}
	if assignments != actorAssignments {
		t.Errorf("the walk found %d assignments to an Actor field and this guard is written against %d",
			assignments, actorAssignments)
	}
}

// eventLiteralsIn answers the event literals one composite literal is: itself,
// where its own type names an event, and each of its elements whose type the
// enclosing literal supplies.
//
// An element of a slice, array or map literal may name no type at all, because
// Go takes it from the composite it stands in. A walk reading each literal's
// own Type node therefore steps straight over such an element, which is how a
// slice of events built by hand walks past this guard. A map's element is the
// value rather than the key, and a key that is itself a literal is left alone,
// since an event is never a map key here and reading one as an element would
// invent a construction site.
//
// Only one level is unwrapped, because a literal nested two deep carries its
// own type wherever this code base builds one, and a guard that walked deeper
// would count a literal twice.
func eventLiteralsIn(literal *ast.CompositeLit) []*ast.CompositeLit {
	if namesEventType(literal.Type) {
		return []*ast.CompositeLit{literal}
	}
	if !namesEventCollection(literal.Type) {
		return nil
	}
	var built []*ast.CompositeLit
	for _, element := range literal.Elts {
		candidate := element
		if pair, keyed := element.(*ast.KeyValueExpr); keyed {
			candidate = pair.Value
		}
		if inner, ok := candidate.(*ast.CompositeLit); ok && inner.Type == nil {
			built = append(built, inner)
		}
	}
	return built
}

// namesEventCollection reports whether a composite literal's type is a slice,
// an array or a map of events, which are the three shapes that supply their
// elements' type rather than making each element carry one.
func namesEventCollection(node ast.Expr) bool {
	switch named := node.(type) {
	case *ast.ArrayType:
		return namesEventType(named.Elt)
	case *ast.MapType:
		return namesEventType(named.Value)
	}
	return false
}

// composedByAComposer reports whether an expression is a call to one of the two
// functions this card allows an actor to be composed in.
func composedByAComposer(node ast.Expr) bool {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch called := call.Fun.(type) {
	case *ast.Ident:
		return called.Name == "NamedActor"
	case *ast.SelectorExpr:
		if called.Sel.Name == "Acting" {
			return true
		}
		pkg, ok := called.X.(*ast.Ident)
		return ok && pkg.Name == workbenchPackage && called.Sel.Name == "NamedActor"
	}
	return false
}

// namesEventType reports whether a composite literal's type is bench.Event or,
// inside internal/bench, Event.
func namesEventType(node ast.Expr) bool {
	switch named := node.(type) {
	case *ast.Ident:
		return named.Name == "Event"
	case *ast.SelectorExpr:
		pkg, ok := named.X.(*ast.Ident)
		return ok && pkg.Name == workbenchPackage && named.Sel.Name == "Event"
	}
	return false
}

// actorElement is the value a literal gives its Actor field, and false where
// the literal names no Actor at all.
func actorElement(literal *ast.CompositeLit) (ast.Expr, bool) {
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok || key.Name != "Actor" {
			continue
		}
		return pair.Value, true
	}
	return nil, false
}

// harnessCheckedElsewhere names the functions of this package that build an
// event and do not run the malformed-harness refusal themselves, each with the
// entry point that runs it before they are reached.
//
// The table exists so that the guard below can be a rule over the whole package
// rather than a list of the functions somebody remembered. A function added
// later that builds an event and neither checks nor appears here fails the
// guard, which is what stops the refusal reaching three write paths and missing
// twenty, the shape Agent Code Review found in round one.
var harnessCheckedElsewhere = map[string]string{
	"removalRecord":         "composes the deletion's own record for Delete, which runs the refusal before it resolves anything",
	"Comment":               "runs canComment before it takes the lock, and canComment runs the refusal as its second row, which is how OfferActs asks the same rows",
	"fieldEvent":            "composes the write's own record for writeField, which runs the refusal at its head",
	"lapse":                 "composes from the lapsed holder's name through the composer that declares nothing, so no declared harness reaches the line and a read that lapses a claim is never refused",
	"claim":                 "one of the seven verb bodies Library.Do dispatches to under the card's lock, and Do runs the refusal at its head",
	"move":                  "the same",
	"release":               "the same",
	"block":                 "the same",
	"unblock":               "the same",
	"join":                  "the same",
	"leave":                 "the same",
	"grant":                 "the same",
	"revoke":                "the same",
	"pull":                  "the body Library.Pull runs under the lock, and Pull runs the refusal at its head",
	"closeItem":             "one of the three terminal checklist bodies withItem runs, and withItem runs the refusal at its head",
	"Waive":                 "the same, for the body that lands an item at waived",
	"Withdraw":              "the same, for the body that lands an item at withdrawn",
	"mintDesignation":       "the same, for the comment the --text form of a terminal verb mints inside closeItem",
	"Cite":                  "the same, for the citation body",
	"Reopen":                "the same, for the reopening body",
	"adoptWorkstreams":      "a repair Check reaches, and Check runs the refusal for every request carrying a repair marker",
	"migrateDesignations":   "the same, and the one line it writes is written by the converting form alone, which is a repair; the rehearsal writes nothing and is deliberately not one",
	"writeAddedColumns":     "a step of Reshape, which runs the refusal at its head",
	"carryOneCard":          "the same",
	"archiveRetiredColumns": "the same",
	"rewriteKeptColumns":    "the same",
	"writeAddedAttachments": "the attached lines writeAddedColumns journals for an added column's attachments, reached only from that step of Reshape",
	"fileStandingItems":     "the item_filed lines an arrival mints for a column's standing items, reached from add, move and pull, which each run the refusal at their head, from the carry step of Reshape, and from the file-standing repair Check reaches",
	"withdrawInstancesOf":   "a step of Reshape, which runs the refusal at its head",
}

// TestEveryFunctionThatWritesAnEventRunsTheHarnessRefusal asserts that the
// malformed-harness refusal reaches every act that writes a journal line rather
// than the handful of verbs somebody edited.
//
// Round one of Agent Code Review set an illegal DINAH_HARNESS and watched
// comment, link and file each succeed and each write the illegal name into the
// journal, with about twenty other verbs in the same hole, because the refusal
// was called from three places. dinah-497 forms a layer name out of this value,
// so a journal carrying one the grammar refuses is the thing the grammar exists
// to prevent.
//
// The rule is over the function rather than over the verb: a function of this
// package that builds a bench.Event or a bench.StructuralAct either runs the
// refusal itself or is named in the table above with the entry that runs it.
func TestEveryFunctionThatWritesAnEventRunsTheHarnessRefusal(t *testing.T) {
	fileset := token.NewFileSet()
	packages, err := parser.ParseDir(fileset, ".", func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse the package: %v", err)
	}
	writers, checked := map[string]bool{}, map[string]bool{}
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok {
					continue
				}
				ast.Inspect(function, func(node ast.Node) bool {
					switch found := node.(type) {
					case *ast.CompositeLit:
						if len(eventLiteralsIn(found)) > 0 || namesStructuralAct(found.Type) {
							writers[function.Name.Name] = true
						}
					case *ast.CallExpr:
						if selector, ok := found.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "malformedHarness" {
							checked[function.Name.Name] = true
						}
					}
					return true
				})
			}
		}
	}
	if len(writers) == 0 {
		t.Fatal("the walk found no function that writes an event, so it is asserting nothing")
	}
	if len(writers) < 20 {
		t.Errorf("the walk found %d functions that write an event, and this package carries about two dozen", len(writers))
	}
	for name := range writers {
		if checked[name] {
			continue
		}
		if _, exempt := harnessCheckedElsewhere[name]; exempt {
			continue
		}
		t.Errorf("%s builds an event and neither runs the malformed-harness refusal nor is named in harnessCheckedElsewhere with the entry that does", name)
	}
	for name := range harnessCheckedElsewhere {
		if !writers[name] {
			t.Errorf("harnessCheckedElsewhere names %s, and no function of that name builds an event any more; remove the stale row", name)
		}
	}
}

// namesStructuralAct reports whether a composite literal's type is
// bench.StructuralAct, which archive, restore and delete write their own
// journal line through.
func namesStructuralAct(node ast.Expr) bool {
	selector, ok := node.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == workbenchPackage && selector.Sel.Name == "StructuralAct"
}
