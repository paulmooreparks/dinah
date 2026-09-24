package verb

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// filledCard is a card carrying one comment, one checklist item and one
// attachment, so every mount the containment grammar gives a card holds a
// member and a walk that skipped one is visible in the total.
func filledCard(t *testing.T, h *harness) string {
	t.Helper()
	ref := h.ready("A card with content")
	h.comment(ref, "A thought.")
	h.item(ref, "d00000000001", "kind: decision\nstate: resolved\nordinal: 1\n", "A decision.")
	h.attach(ref, "note.txt", "some bytes")
	return ref
}

// TestCardViewPublishesChildCount asserts dinah-519 criteria/13's first half:
// the total rides on every response holding a card view, it is absent where
// the card holds nothing, and the two per-collection counts beside it are
// unchanged.
//
// Arming: dropping the ChildCount assignment from Library.view reddens the
// positive case by name.
func TestCardViewPublishesChildCount(t *testing.T) {
	h := newHarness(t)
	ref := filledCard(t, h)

	view, err := h.library.view(h.card(ref))
	if err != nil {
		t.Fatalf("view the filled card: %v", err)
	}
	if view.ChildCount != 3 {
		t.Errorf("the filled card publishes child_count %d, wanted 3", view.ChildCount)
	}
	if view.AttachmentCount != 1 {
		t.Errorf("the filled card publishes attachment_count %d, wanted 1", view.AttachmentCount)
	}
	if view.ChecklistCount != 1 {
		t.Errorf("the filled card publishes checklist_count %d, wanted 1", view.ChecklistCount)
	}
	payload := marshalled(t, view)
	if payload["child_count"] != float64(3) {
		t.Errorf("the payload carries child_count %v, wanted 3", payload["child_count"])
	}
	for _, key := range []string{"attachment_count", "checklist_count"} {
		if payload[key] != float64(1) {
			t.Errorf("the payload carries %s %v, wanted 1", key, payload[key])
		}
	}

	bare := h.ready("A card with nothing")
	empty, err := h.library.view(h.card(bare))
	if err != nil {
		t.Fatalf("view the empty card: %v", err)
	}
	if empty.ChildCount != 0 {
		t.Errorf("the empty card publishes child_count %d, wanted 0", empty.ChildCount)
	}
	payload = marshalled(t, empty)
	if _, carried := payload["child_count"]; carried {
		t.Errorf("the empty card's payload carries child_count, and omitempty should have left the key out: %v", payload)
	}
}

// marshalled renders a view through encoding/json and reads it back as a
// map, which is how a test asserts what the key does on the wire rather than
// what the struct holds.
func marshalled(t *testing.T, view *CardView) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal the view: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("read the payload back: %v", err)
	}
	return payload
}

// TestOneCardViewMakesOneListingPerMountPlusTheBlockingRead asserts dinah-519
// criteria/13's behavioural half: the two counts a card view used to take by
// listing two directories are read out of the one grammar walk instead, so
// publishing the total costs one listing per card view rather than three.
//
// The property is the number of directory listings, so the number is what is
// counted. The mount count comes from bench.Contains rather than being written
// down, and the one extra is TallyItems, which lists the checklist a second
// time because it opens each item's anchor to count both the blocking items
// and, since dinah-599, the items waiting on the operator. That is four at the
// grammar as it stands, and the unfolded implementation makes six.
//
// The test writes a package var in bench, so it declares itself non-parallel
// and puts the seam back.
//
// Arming: restoring Library.view's two bench.CountAttachments and
// bench.CountItems calls beside the walk reddens this test by name with six
// listings against four.
func TestOneCardViewMakesOneListingPerMountPlusTheBlockingRead(t *testing.T) {
	h := newHarness(t)
	ref := filledCard(t, h)
	card := h.card(ref)

	var listed []string
	bench.ListIDsObserver = func(collection string) { listed = append(listed, collection) }
	t.Cleanup(func() { bench.ListIDsObserver = nil })

	if _, err := h.library.view(card); err != nil {
		bench.ListIDsObserver = nil
		t.Fatalf("view the card: %v", err)
	}
	bench.ListIDsObserver = nil

	mounts := len(bench.Contains(bench.KindCard))
	if mounts == 0 {
		t.Fatal("the grammar gives a card no mount, so this count has nothing to compare against")
	}
	// One listing per mount, plus the one TallyItems makes over the
	// checklist it then opens item by item.
	want := mounts + 1
	if len(listed) != want {
		t.Errorf("one card view made %d collection listings, wanted %d (%d mounts plus the blocking-item read): %v",
			len(listed), want, mounts, listed)
	}
}

// TestLibraryViewReachesNoPerCollectionCount asserts dinah-519 criteria/13's
// source half, which sits beside the count because a count alone admits an
// implementation reaching four listings some other way.
//
// The walk is transitive through every function and method declared in this
// package, because a guard over one body is defeated by moving the two
// listings into a helper Library.view calls: the helper passes the guard while
// the card view still makes six listings. The guard cannot simply assert that
// package verb never names the two helpers either, because it legitimately
// does elsewhere, in the root's status and in columnViews, neither of which
// Library.view reaches.
//
// Since dinah-599, the guard's final assertion moved from requiring
// CountBlockingItems to be called at least once to requiring TallyItems to be
// called exactly once, with the same non-foldability reason, and a new
// assertion requires CountBlockingItems to be called zero times: the card
// view's two counts, blocking and operator-pending, come from one walk of
// the checklist now rather than from a second call that opens the same
// anchors again.
//
// Arming: restoring the bench.CountAttachments and bench.CountItems calls in
// Library.view reddens this test by name, and moving them into a helper
// Library.view calls reddens it too. Restoring the old
// l.Bench.CountBlockingItems call in place of TallyItems reddens the last two
// assertions.
func TestLibraryViewReachesNoPerCollectionCount(t *testing.T) {
	graph := packageCallGraph(t)
	reachable := reachableFrom(graph, "view")
	if len(reachable) < 2 {
		t.Fatalf("the walk from Library.view reaches %d functions, so it read nothing", len(reachable))
	}
	if !reachable["view"] {
		t.Fatal("the walk does not reach Library.view itself, so this guard is looking at the wrong graph")
	}

	calls := map[string]int{}
	for name := range reachable {
		for _, called := range graph[name] {
			calls[called]++
		}
	}
	// The names are spelled without their package qualifier, because the
	// graph keys every call by the name after its last dot and because the
	// product's word is workbench: a literal carrying the package's own short
	// name trips the vocabulary guard in internal/profile. Nothing is lost,
	// since this package declares no function of any of these names itself.
	for _, forbidden := range []string{"CountAttachments", "CountItems"} {
		if calls[forbidden] > 0 {
			t.Errorf("%s is called %d times in what Library.view reaches, and the whole of the fold is that it is called none",
				forbidden, calls[forbidden])
		}
	}
	if calls["ChildCounts"] != 1 {
		t.Errorf("ChildCounts is called %d times in what Library.view reaches, wanted exactly once", calls["ChildCounts"])
	}
	if calls["TallyItems"] != 1 {
		t.Errorf("TallyItems is called %d times in what Library.view reaches, wanted exactly once: it is not foldable, it opens each item's anchor", calls["TallyItems"])
	}
	if calls["CountBlockingItems"] != 0 {
		t.Errorf("CountBlockingItems is called %d times in what Library.view reaches, wanted zero: the card view now counts blocking items from the same TallyItems walk it counts operator_pending from", calls["CountBlockingItems"])
	}
}

// packageCallGraph is what each function and method declared in this package
// calls, keyed by the declaration's own name with no receiver on it. A call
// site carries no type information the parser can read, so `l.declaredFieldValues`
// and `l.Bench.CountBlockingItems` are both reachable only by the name after
// the last dot, and keying the declarations the same way is what lets the two
// meet. Two declarations sharing a name merge, which widens what the walk
// reaches rather than narrowing it, so a forbidden call cannot hide behind the
// merge.
//
// A call through a package qualifier is recorded twice, qualified and bare,
// and the assertions read the bare form: one is enough here, because this
// package declares no function sharing a name with the helpers in question.
//
// The package's own non-test sources are parsed. A test helper calling one of
// the forbidden helpers is not the product reaching it, and it would redden a
// guard about the product for a reason that has nothing to do with the rule.
func packageCallGraph(t *testing.T) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package's sources: %v", err)
	}
	graph := map[string][]string{}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, source, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			graph[fn.Name.Name] = append(graph[fn.Name.Name], calledNames(fn.Body)...)
		}
	}
	if len(graph) == 0 {
		t.Fatal("the parse found no function in this package, so the graph is empty and this guard proves nothing")
	}
	return graph
}

// calledNames are the names one body calls, spelled the way the graph keys
// them. A selector is recorded twice, once qualified and once bare, because
// the qualifier is a package name for bench.ChildCounts and a receiver
// expression for l.Bench.CountBlockingItems, and the walk cannot tell the two
// apart without type information it does not have.
func calledNames(body *ast.BlockStmt) []string {
	var called []string
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			called = append(called, fn.Name)
		case *ast.SelectorExpr:
			called = append(called, fn.Sel.Name)
			if ident, ok := fn.X.(*ast.Ident); ok {
				called = append(called, ident.Name+"."+fn.Sel.Name)
			}
		}
		return true
	})
	return called
}

// reachableFrom is the transitive closure of a call graph from one name,
// including the name itself.
func reachableFrom(graph map[string][]string, start string) map[string]bool {
	reached := map[string]bool{}
	queue := []string{start}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if reached[name] {
			continue
		}
		reached[name] = true
		for _, called := range graph[name] {
			if _, declared := graph[called]; declared && !reached[called] {
				queue = append(queue, called)
			}
		}
	}
	return reached
}

// TestOperatorPendingCountsOnlyWhatAwaitsTheOperator drives dinah-599
// criteria/1. The five items on the first card are chosen so that every
// clause of bench.ItemAwaitsOperator has something to admit or refuse: a
// pending open question owned by the operator and a pending decision with no
// owner both count; a pending open question owned by holder, a resolved
// decision owned by the operator, and a pending acceptance criterion owned by
// the operator do not, on the owner, state and kind clauses respectively. The
// second card carries only the three that never count, so operator_pending
// is omitted rather than published as zero.
func TestOperatorPendingCountsOnlyWhatAwaitsTheOperator(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("five items, two for the operator")
	h.item(ref, "b00000000001", "kind: open_question\nstate: pending\nowner: operator\nordinal: 1\n", "Owned by the operator.")
	h.item(ref, "b00000000002", "kind: decision\nstate: pending\nordinal: 2\n", "No owner at all.")
	h.item(ref, "b00000000003", "kind: open_question\nstate: pending\nowner: holder\nordinal: 3\n", "Owned by the holder.")
	h.item(ref, "b00000000004", "kind: decision\nstate: resolved\nowner: operator\nordinal: 4\n", "Already settled.")
	h.item(ref, "b00000000005", "kind: acceptance_criterion\nstate: pending\nowner: operator\nordinal: 5\n", "A criterion, never his queue.")

	view, err := h.library.view(h.card(ref))
	if err != nil {
		t.Fatalf("view the five-item card: %v", err)
	}
	if view.OperatorPending != 2 {
		t.Errorf("OperatorPending is %d, wanted 2", view.OperatorPending)
	}
	payload := marshalled(t, view)
	if payload["operator_pending"] != float64(2) {
		t.Errorf("the payload carries operator_pending %v, wanted 2", payload["operator_pending"])
	}

	bare := h.ready("only the three that never count")
	h.item(bare, "b00000000006", "kind: open_question\nstate: pending\nowner: holder\nordinal: 1\n", "Owned by the holder.")
	h.item(bare, "b00000000007", "kind: decision\nstate: resolved\nowner: operator\nordinal: 2\n", "Already settled.")
	h.item(bare, "b00000000008", "kind: acceptance_criterion\nstate: pending\nowner: operator\nordinal: 3\n", "A criterion.")

	noneView, err := h.library.view(h.card(bare))
	if err != nil {
		t.Fatalf("view the three-item card: %v", err)
	}
	if noneView.OperatorPending != 0 {
		t.Errorf("OperatorPending is %d, wanted 0", noneView.OperatorPending)
	}
	nonePayload := marshalled(t, noneView)
	if _, carried := nonePayload["operator_pending"]; carried {
		t.Errorf("the payload carries operator_pending though nothing awaits the operator, and omitempty should have left the key out: %v", nonePayload)
	}
}
