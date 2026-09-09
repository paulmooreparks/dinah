package verb

import (
	"path/filepath"
	"testing"

	"dinah/internal/bench"
)

// TestAWorkbenchHitIsAddressedAsTheWorkbench asserts dinah-454 AC-7: a
// workbench that matches a search prints the spelling that reaches it.
//
// The second assertion is what keeps the first from becoming a tautology
// later: it records why the slug was wrong rather than only that the value
// changed, and it goes red the day a bare slug starts resolving.
func TestAWorkbenchHitIsAddressedAsTheWorkbench(t *testing.T) {
	h := newHarness(t)
	anchor := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	if err := bench.WriteText(anchor, text+"\nhaystack prose\n"); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()

	results, err := h.library.Search(&Request{Verb: "search", Actor: "alka", SearchText: "haystack"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var hit *SearchHit
	for i, candidate := range results.Hits {
		if candidate.Kind == SearchKindWorkbench {
			hit = &results.Hits[i]
		}
	}
	if hit == nil {
		t.Fatalf("the search drew no workbench hit, so the fixture never reached the row this guards: %+v", results.Hits)
	}
	if hit.Ref != bench.WorkbenchRef {
		t.Errorf("the workbench hit is addressed %q, wanted %q, which is the form path, show and edit accept", hit.Ref, bench.WorkbenchRef)
	}
	want, err := filepath.Abs(anchor)
	if err != nil {
		t.Fatalf("resolve the anchor's own path: %v", err)
	}
	if path, err := h.library.Bench.ResolvePath(hit.Ref); err != nil {
		t.Errorf("the workbench hit is addressed %q, which resolves to nothing: %v", hit.Ref, err)
	} else if path != want {
		t.Errorf("the workbench hit is addressed %q, which resolves to %q rather than the workbench anchor at %q", hit.Ref, path, want)
	}
	if _, err := h.library.Bench.ResolvePath(h.library.Bench.Slug); err == nil {
		t.Errorf("the workbench's bare slug %q resolves, so this guard no longer records why the slug was the wrong spelling", h.library.Bench.Slug)
	}
}
