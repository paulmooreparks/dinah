package lsp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/bench"
)

// hover answers the three-block markdown of section 2.5 over the exact range
// of the reference, and nothing at all where no reference stands.
func (s *Server) hover(raw json.RawMessage) any {
	var params positionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	at := s.annotationAt(params.TextDocument.URI, params.Position)
	if at == nil {
		return nil
	}
	span := at.Slot.Range
	return hoverResult{Contents: markupContent{Kind: "markdown", Value: at.Tooltip}, Range: &span}
}

// documentLinks answers one link per resolved reference in the document,
// standard and prose alike.
//
// A link's target is the file the definition rule answers for the same
// reference, kind by kind, because both read the one File member an
// annotation carries. There is no second per-kind rule here to be kept equal
// to that one, so no reference can open one file on a click and a different
// one on a go-to-definition.
func (s *Server) documentLinks(raw json.RawMessage) any {
	var params documentParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return []documentLink{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	links := []documentLink{}
	doc, ok := s.docs[params.TextDocument.URI]
	if !ok {
		return links
	}
	for _, at := range doc.model {
		if at.File == "" {
			continue
		}
		links = append(links, documentLink{Range: at.Slot.Range, Target: fileURI(at.File)})
	}
	return links
}

// definition answers the file a reference names, at its head, as a Location
// rather than a LocationLink.
func (s *Server) definition(raw json.RawMessage) any {
	var params positionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	at := s.annotationAt(params.TextDocument.URI, params.Position)
	if at == nil || at.File == "" {
		return nil
	}
	return Location{URI: fileURI(at.File), Range: Range{}}
}

// inlayHints answers the standard inline label for each annotation that draws
// one, positioned after the value it describes.
func (s *Server) inlayHints(raw json.RawMessage) any {
	var params documentParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return []inlayHint{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	hints := []inlayHint{}
	for _, at := range s.inlineOf(params.TextDocument.URI) {
		hints = append(hints, inlayHint{Position: at.Slot.Range.End, Label: at.Label, PaddingLeft: true})
	}
	return hints
}

// annotations answers the namespaced request a client draws its own
// decoration from. The label and the tooltip are for a person and are
// translated; the target and the fields are for the machine and are canonical
// tokens, never translated.
func (s *Server) annotations(raw json.RawMessage) any {
	var params documentParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return annotationsResult{Annotations: []wireAnnotation{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	wire := []wireAnnotation{}
	for _, at := range s.inlineOf(params.TextDocument.URI) {
		fields := at.Fields
		if fields == nil {
			fields = map[string]string{}
		}
		wire = append(wire, wireAnnotation{
			Range:   at.Slot.Range,
			Label:   at.Label,
			Tooltip: at.Tooltip,
			Target:  at.Target,
			Fields:  fields,
			Link:    fileURI(at.File),
		})
	}
	return annotationsResult{Annotations: wire}
}

// inlineOf lists the annotations of one document that draw an inline
// annotation, which is the five kinds of section 4.4 and an unresolved
// front-matter value. The caller holds the lock.
func (s *Server) inlineOf(uri string) []annotation {
	doc, ok := s.docs[uri]
	if !ok {
		return nil
	}
	var inline []annotation
	for _, at := range doc.model {
		if at.Inline {
			inline = append(inline, at)
		}
	}
	return inline
}

// annotationAt finds the annotation whose range covers a position. The caller
// holds the lock.
func (s *Server) annotationAt(uri string, at Position) *annotation {
	doc, ok := s.docs[uri]
	if !ok {
		return nil
	}
	for i := range doc.model {
		if covers(doc.model[i].Slot.Range, at) {
			return &doc.model[i]
		}
	}
	return nil
}

// covers reports whether a range holds a position, the end being included so
// that a cursor sitting just past the last character of a reference is still
// on it.
func covers(span Range, at Position) bool {
	if at.Line != span.Start.Line || at.Line != span.End.Line {
		return false
	}
	return at.Character >= span.Start.Character && at.Character <= span.End.Character
}

// completion fires at each of the eight front-matter reference positions the
// format declares, at no other position, and never in prose.
//
// The set is the declared positions themselves rather than a count of them,
// so it is stated in one place and moves with the table: what decides is the
// kind the scanner read off the position, and a position outside the table
// yields no slot for this to find.
func (s *Server) completion(raw json.RawMessage) any {
	var params positionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return completionList{Items: []completionItem{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	empty := completionList{Items: []completionItem{}}
	doc, ok := s.docs[params.TextDocument.URI]
	if !ok || s.bench == nil {
		return empty
	}
	for _, at := range doc.slots {
		if at.Kind == slotProse || !covers(at.Range, params.Position) {
			continue
		}
		switch at.Kind {
		case slotColumn:
			return completionList{Items: s.columnCandidates(at.Text)}
		case slotWorkstream:
			return completionList{Items: s.workstreamCandidates(at.Text)}
		case slotRoute:
			return completionList{Items: s.routeCandidates(at.Text)}
		case slotCard:
			items, cut := s.cardCandidates(at.Text)
			return completionList{IsIncomplete: cut, Items: items}
		}
	}
	return empty
}

// columnCandidates lists the columns of the flow, in flow order, so a
// completion list reads the way the board does.
func (s *Server) columnCandidates(typed string) []completionItem {
	width := len(strconv.Itoa(len(s.bench.Columns)))
	items := []completionItem{}
	for _, column := range s.bench.Columns {
		item := completionItem{
			Label:      column.Title,
			InsertText: column.ID,
			Detail:     s.render(keyCompletionColumn, "slug", column.Ref(), "position", strconv.Itoa(column.Position+1)),
			SortText:   fmt.Sprintf("%0*d", width, column.Position),
		}
		item.FilterText = filterTextOf(item, column.Slug)
		if matches(item, typed) {
			items = append(items, item)
		}
	}
	return items
}

// routeCandidates lists the routes the workbench declares, in declaration
// order, which is the order the declaration reads in and the order a listing
// prints them. The detail is the route's own column count, which is the one
// fact that tells a reader how short the road is without opening the block.
func (s *Server) routeCandidates(typed string) []completionItem {
	width := len(strconv.Itoa(len(s.bench.RouteNames)))
	items := []completionItem{}
	for at, name := range s.bench.RouteNames {
		carried := bench.RouteColumnsIn(s.bench.Routes[name], s.bench.Columns)
		item := completionItem{
			Label:      name,
			InsertText: name,
			Detail:     s.render(keyCompletionRoute, "columns", strconv.Itoa(len(carried))),
			SortText:   fmt.Sprintf("%0*d", width, at),
		}
		item.FilterText = filterTextOf(item, name)
		if matches(item, typed) {
			items = append(items, item)
		}
	}
	return items
}

// workstreamCandidates lists the live workstreams, ordered by title.
func (s *Server) workstreamCandidates(typed string) []completionItem {
	workstreams, err := s.bench.Workstreams()
	if err != nil {
		return []completionItem{}
	}
	items := []completionItem{}
	for _, workstream := range workstreams {
		item := completionItem{
			Label:      workstream.Title,
			InsertText: workstream.ID,
			Detail:     s.render(keyCompletionWorkstream, "slug", workstream.Ref()),
			SortText:   workstream.Title,
		}
		item.FilterText = filterTextOf(item, workstream.Slug)
		if matches(item, typed) {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].SortText < items[j].SortText })
	return items
}

// cardCandidates lists the live cards, newest first, capped and honest about
// the cap. A workbench with thousands of cards must not send them all, and a
// list cut short says so rather than pretending it is whole.
func (s *Server) cardCandidates(typed string) ([]completionItem, bool) {
	cards, err := s.bench.Cards()
	if err != nil {
		return []completionItem{}, false
	}
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].Number > cards[j].Number })
	items := []completionItem{}
	for _, card := range cards {
		ref := card.Ref(s.bench.Slug)
		columnTitle := ""
		if column := s.bench.Column(card.Column); column != nil {
			columnTitle = column.Title
		}
		item := completionItem{
			Label:      ref + " " + card.Title,
			InsertText: card.ID,
			Detail:     s.render(keyCompletionCard, "column", columnTitle, "state", card.State),
			SortText:   fmt.Sprintf("%09d", descending(card.Number)),
		}
		item.FilterText = filterTextOf(item, ref)
		if !matches(item, typed) {
			continue
		}
		if len(items) == completionCap {
			return items, true
		}
		items = append(items, item)
	}
	return items, false
}

// descending turns a card number into a sort key that puts the newest first,
// which is the order a reader wants when linking a card they just filed.
func descending(number int) int {
	const ceiling = 999999999
	if number > ceiling {
		return 0
	}
	return ceiling - number
}

// filterTextOf composes what a client narrows on, so typing the identifier,
// the slug or a word of the title all reach the same candidate.
func filterTextOf(item completionItem, slug string) string {
	return strings.TrimSpace(item.Label + " " + item.InsertText + " " + slug)
}

// matches narrows a candidate against what has been typed at the position,
// without regard to ASCII case. An empty value matches everything, which is
// the ordinary case: a reader opens completion before typing anything.
func matches(item completionItem, typed string) bool {
	want := strings.ToLower(strings.TrimSpace(typed))
	if want == "" {
		return true
	}
	return strings.Contains(strings.ToLower(item.FilterText), want)
}

// cardsOf is the count of live cards, which the completion cap is measured
// against. It is here rather than inline so a test naming the size of the set
// it swept reads the same number the implementation does.
func cardsOf(b *bench.Bench) int {
	cards, err := b.Cards()
	if err != nil {
		return 0
	}
	return len(cards)
}
