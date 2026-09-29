package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// TestAMemberReferenceOpensItsCardsFile drives dinah-637/criteria/19. In the
// card-unit layout a checklist item and a comment are lines of their card's
// journal and have no file of their own, so an item reference and a comment
// reference are annotated with the holding card's card.md as their file, the
// document link and the definition both.
//
// Arming: making memberFile answer the empty string leaves both references
// with no link, which the first assertion catches.
func TestAMemberReferenceOpensItsCardsFile(t *testing.T) {
	bench.EnableCardUnitForTest(t)
	f := build(t)
	if f.bench.Format != bench.CardUnitFormat {
		t.Fatalf("the fixture opened at format %d, wanted the card-unit format %d", f.bench.Format, bench.CardUnitFormat)
	}
	h, _ := f.serve(t)
	h.initialize(nil)

	want := fileURI(f.cardAnchor(f.card))
	rows := []string{f.item, f.comment}
	var lines []string
	for _, ref := range rows {
		lines = append(lines, "The member is "+ref+" here.")
	}
	doc := filepath.Join(filepath.Dir(f.bench.Root), "members.md")
	uri := h.openText(doc, strings.Join(lines, "\n")+"\n")
	links := decode[[]documentLink](t, h.send(methodDocumentLink, map[string]any{"textDocument": map[string]any{"uri": uri}}))
	byLine := map[int]documentLink{}
	for _, link := range links {
		byLine[link.Range.Start.Line] = link
	}
	for number, ref := range rows {
		link, linked := byLine[number]
		if !linked {
			t.Errorf("%s carries no document link", ref)
			continue
		}
		if link.Target != want {
			t.Errorf("%s links to %q, wanted its card's file %q", ref, link.Target, want)
		}
		location := decode[Location](t, h.send(methodDefinition, map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": number, "character": link.Range.Start.Character},
		}))
		if location.URI != want {
			t.Errorf("%s defines at %q, wanted its card's file %q", ref, location.URI, want)
		}
	}
}
