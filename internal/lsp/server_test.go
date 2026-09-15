package lsp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// refusalNameOf answers the name a refusal carries, so a test asserting which
// resolver refused a reference names the refusal rather than the error text.
func refusalNameOf(err error) string {
	var refusal *contract.Refusal
	if errors.As(err, &refusal) {
		return refusal.Name
	}
	return ""
}

// TestInitializeDeclaresExactlyTheSpecifiedCapabilities asserts dinah-515
// criterion 1: the capability object is the one section 2.1 draws and carries
// no other key, which is what keeps a diagnostic capability out of this card.
func TestInitializeDeclaresExactlyTheSpecifiedCapabilities(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	raw := h.send(methodInitialize, clientDeclaring(true))

	var read struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
		ServerInfo   map[string]string          `json:"serverInfo"`
	}
	if err := json.Unmarshal(raw, &read); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]string{
		"textDocumentSync":     `{"openClose":true,"change":1,"save":{"includeText":false}}`,
		"hoverProvider":        `true`,
		"documentLinkProvider": `{"resolveProvider":false}`,
		"definitionProvider":   `true`,
		"completionProvider":   `{"triggerCharacters":["/","-"," "],"resolveProvider":false}`,
		"inlayHintProvider":    `{"resolveProvider":false}`,
	}
	for key, expected := range want {
		got, carried := read.Capabilities[key]
		if !carried {
			t.Errorf("the capabilities carry no %s", key)
			continue
		}
		if string(got) != expected {
			t.Errorf("%s is %s, wanted %s", key, got, expected)
		}
	}
	if len(read.Capabilities) != len(want) {
		var extra []string
		for key := range read.Capabilities {
			if _, declared := want[key]; !declared {
				extra = append(extra, key)
			}
		}
		t.Errorf("the capabilities carry %d keys and the contract draws %d; the extra are %v", len(read.Capabilities), len(want), extra)
	}
	if _, published := read.Capabilities["diagnosticProvider"]; published {
		t.Error("the server declares a diagnostic provider, which dinah-264 owns and this card does not")
	}
	if read.ServerInfo["name"] != "dinah" {
		t.Errorf("the server names itself %q", read.ServerInfo["name"])
	}
}

// frontMatterFixture is a card anchor carrying one value at every declared
// front-matter reference position and a spread of keys at none of them,
// written by hand so the test knows exactly what it put there.
//
// The sequences are written one in block form and one in flow form, which is
// the pair criterion 2 names, and the flow case is what pins that the
// brackets and the commas fall outside every range.
func frontMatterFixture(f *fixture) (text string, declared int, quiet int) {
	columns := f.bench.Columns
	text = strings.Join([]string{
		"---",
		"title: A hand-written card",
		"column: " + columns[0].ID,
		"state: ready",
		"claim_holder: " + columns[0].ID,
		"block_kind: " + columns[1].ID,
		"workstreams: [" + f.workstream + "]",
		"links:",
		"  - kind: relates_to",
		"    to: " + f.other,
		"tier_at:",
		"  - column: " + columns[1].ID,
		"    tier: frontier",
		"field_values:",
		"  git.branch: " + columns[0].ID,
		"---",
		"",
	}, "\n")
	// Four declared positions stand in this fixture: column, workstreams[],
	// links[].to and tier_at[].column. The other four of the eight live in
	// anchors of other kinds, which criterion 25 sweeps.
	//
	// Four keys hold a column identifier at a position the format declares
	// not to be a reference: claim_holder, block_kind, a field_values member
	// and title, the last holding prose rather than an identifier.
	return text, 4, 4
}

// TestFrontMatterIsRecognisedByPositionAndNeverByShape asserts dinah-515
// criterion 2: over the framed base protocol, one inlay hint stands at each
// declared position and none at any other key, including claim_holder and a
// field_values entry, each half reporting the size of the set it swept.
func TestFrontMatterIsRecognisedByPositionAndNeverByShape(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)

	text, declared, quiet := frontMatterFixture(f)
	path := filepath.Join(f.bench.CardsRoot(), f.card, bench.CardAnchor)
	uri := h.openText(path, text)
	hints := decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": uri}}))

	if len(hints) != declared {
		t.Fatalf("the fixture carries %d declared positions and the server drew %d hints: %+v", declared, len(hints), hints)
	}
	lines := strings.Split(text, "\n")
	shaped := 0
	for _, hint := range hints {
		key := strings.TrimSpace(strings.SplitN(strings.TrimSpace(lines[hint.Position.Line]), ":", 2)[0])
		switch strings.TrimPrefix(key, "- ") {
		case "column", "to":
		default:
			if !strings.HasPrefix(strings.TrimSpace(lines[hint.Position.Line]), "workstreams:") {
				t.Errorf("a hint stands on the line %q, which is no declared position", lines[hint.Position.Line])
			}
		}
		shaped++
	}
	if shaped != declared {
		t.Errorf("swept %d hints, wanted %d", shaped, declared)
	}

	// The quiet half: every key the fixture carries a column identifier at
	// which the format declares not to be a reference position.
	silent := 0
	for _, key := range []string{"claim_holder", "block_kind", "git.branch", "title"} {
		for number, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), key+":") {
				continue
			}
			for _, hint := range hints {
				if hint.Position.Line == number {
					t.Errorf("the key %s carries a hint, and it is no declared reference position", key)
				}
			}
			silent++
		}
	}
	if silent != quiet {
		t.Errorf("swept %d non-reference keys, wanted %d", silent, quiet)
	}

	// The flow sequence carries one hint per element, and the element's own
	// range excludes the brackets.
	for number, line := range lines {
		if !strings.HasPrefix(line, "workstreams:") {
			continue
		}
		found := 0
		for _, hint := range hints {
			if hint.Position.Line == number {
				found++
				if strings.ContainsAny(string(line[hint.Position.Character-1]), "[]") {
					t.Errorf("the flow element's range ends on a bracket: %q", line)
				}
			}
		}
		if found != 1 {
			t.Errorf("the flow sequence carries %d elements and drew %d hints", 1, found)
		}
	}
}

// TestAnUnresolvedFrontMatterValueIsReportedAndNeverDiagnosed asserts
// dinah-515 criterion 10: a value that resolves to nothing at a declared
// position carries the unresolved label, and the server publishes no
// diagnostic for it and declares no diagnostic provider.
func TestAnUnresolvedFrontMatterValueIsReportedAndNeverDiagnosed(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)

	text := "---\ntitle: A card\ncolumn: aaaaaaaaaaaa\n---\n"
	uri := h.openText(filepath.Join(f.bench.CardsRoot(), f.card, bench.CardAnchor), text)
	hints := decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": uri}}))
	if len(hints) != 1 {
		t.Fatalf("wanted one hint over the unresolved value, got %d", len(hints))
	}
	want := h.server.messages.T(keyLabelUnresolved)
	if hints[0].Label != want {
		t.Errorf("the unresolved value is labelled %q, wanted %q", hints[0].Label, want)
	}
	if sent := h.quiet("textDocument/publishDiagnostics"); sent != 0 {
		t.Errorf("the server published %d diagnostics, and it publishes none", sent)
	}

	// The accepting case beside the refusing one: a value that does resolve
	// carries its own label rather than the unresolved one, so this test
	// cannot pass against a server that labels everything unresolved.
	resolved := "---\ntitle: A card\ncolumn: " + f.bench.Columns[0].ID + "\n---\n"
	second := h.openText(filepath.Join(f.bench.CardsRoot(), f.other, bench.CardAnchor), resolved)
	found := decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": second}}))
	if len(found) != 1 || found[0].Label == want {
		t.Fatalf("the resolving value drew %d hints labelled %v, and it must not read as unresolved", len(found), found)
	}
}

// TestTheFourStandardHandlersAnswerContent asserts dinah-515 criterion 15:
// each of hover, documentLink, definition and completion answers a value
// rather than null, pinned per handler and each assertion naming what it
// expects.
func TestTheFourStandardHandlersAnswerContent(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)

	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	doc := filepath.Join(filepath.Dir(f.bench.Root), "notes.md")
	uri := h.openText(doc, "The work is on "+card.Ref(f.bench.Slug)+" today.\n")
	at := map[string]any{"uri": uri}
	where := map[string]any{"textDocument": at, "position": map[string]any{"line": 0, "character": 16}}

	hover := decode[hoverResult](t, h.send(methodHover, where))
	if !strings.Contains(hover.Contents.Value, card.Title) {
		t.Errorf("the hover answered %q, which does not carry the card's title %q", hover.Contents.Value, card.Title)
	}

	links := decode[[]documentLink](t, h.send(methodDocumentLink, map[string]any{"textDocument": at}))
	if len(links) != 1 {
		t.Fatalf("the document carries one reference and answered %d links", len(links))
	}
	wanted := fileURI(f.cardAnchor(f.card))
	if links[0].Target != wanted {
		t.Errorf("the link targets %q, wanted %q", links[0].Target, wanted)
	}

	location := decode[Location](t, h.send(methodDefinition, where))
	if location.URI != wanted {
		t.Errorf("the definition answers %q, wanted %q", location.URI, wanted)
	}

	anchor := filepath.Join(f.bench.CardsRoot(), f.card, bench.CardAnchor)
	column := f.bench.Columns[0]
	text := "---\ntitle: A card\ncolumn: \n---\n"
	cardURI := h.openText(anchor, text)
	list := decode[completionList](t, h.send(methodCompletion, map[string]any{
		"textDocument": map[string]any{"uri": cardURI},
		"position":     map[string]any{"line": 2, "character": 8},
	}))
	if len(list.Items) == 0 {
		t.Fatal("completion inside column: answered no items")
	}
	found := false
	for _, item := range list.Items {
		if item.InsertText == column.ID && item.Label == column.Title {
			found = true
		}
	}
	if !found {
		t.Errorf("no candidate inserts %s under the label %q; got %+v", column.ID, column.Title, list.Items)
	}
}

// TestALinkAndADefinitionAnswerTheSameFile asserts dinah-515 criterion 26:
// for every reference kind, the document link and the definition name one
// file, and the two kinds where a naive rule would have diverged carry their
// own expected value rather than equality alone.
func TestALinkAndADefinitionAnswerTheSameFile(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)

	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ref := card.Ref(f.bench.Slug)
	rows := []struct {
		name  string
		write string
	}{
		{"card", ref},
		{"column", f.bench.Columns[0].ID},
		{"workstream", "workstream/" + f.workstreamSlug},
		{"item", ref + "/questions/1"},
		{"attachment", ref + "/attachments/1"},
		{"comment", ref + "/comments/1"},
		{"collection", ref + "/questions"},
		{"journal", ref + "/journal"},
		{"payload", ref + "/attachments/1/payload"},
	}
	var lines []string
	for _, row := range rows {
		lines = append(lines, "The "+row.name+" is "+row.write+" here.")
	}
	doc := filepath.Join(filepath.Dir(f.bench.Root), "kinds.md")
	uri := h.openText(doc, strings.Join(lines, "\n")+"\n")
	links := decode[[]documentLink](t, h.send(methodDocumentLink, map[string]any{"textDocument": map[string]any{"uri": uri}}))
	byLine := map[int]documentLink{}
	for _, link := range links {
		byLine[link.Range.Start.Line] = link
	}

	compared := 0
	for number, row := range rows {
		link, linked := byLine[number]
		if !linked {
			t.Errorf("%s carries no document link", row.name)
			continue
		}
		location := decode[Location](t, h.send(methodDefinition, map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": number, "character": link.Range.Start.Character},
		}))
		if location.URI != link.Target {
			t.Errorf("%s links to %q and defines at %q", row.name, link.Target, location.URI)
		}
		compared++
		switch row.name {
		case "workstream":
			want := fileURI(filepath.Join(f.bench.WorkstreamsRoot(), f.workstream, bench.WorkstreamAnchor))
			if link.Target != want {
				t.Errorf("a workstream reference opens %q, wanted the anchor %q", link.Target, want)
			}
			directory, err := f.bench.ResolvePath("workstream/" + f.workstreamSlug)
			if err != nil {
				t.Fatalf("resolve the workstream path: %v", err)
			}
			if link.Target == fileURI(directory) {
				t.Error("a workstream reference opens the directory Bench.ResolvePath answers, which is not a file an editor can open")
			}
		case "attachment":
			attachments, err := bench.Attachments(filepath.Join(f.bench.CardsRoot(), f.card))
			if err != nil || len(attachments) == 0 {
				t.Fatalf("attachments: %v", err)
			}
			if link.Target != fileURI(attachments[0].Path) {
				t.Errorf("an attachment reference opens %q, wanted the payload %q", link.Target, attachments[0].Path)
			}
			if strings.HasSuffix(link.Target, bench.AttachmentAnchor) {
				t.Error("an attachment reference opens the attachment.md anchor rather than its payload")
			}
		case "journal":
			if !strings.HasSuffix(link.Target, "journal.ndjson") {
				t.Errorf("a card's journal opens %q, wanted journal.ndjson", link.Target)
			}
		case "payload":
			if !strings.Contains(link.Target, "/payload/") {
				t.Errorf("an attachment's payload opens %q, wanted the payload file", link.Target)
			}
		}
	}
	if compared != len(rows) {
		t.Errorf("compared %d pairs, wanted %d", compared, len(rows))
	}
}

// TestTheFallThroughIsPinnedRatherThanItsOutcome asserts the half of dinah-515
// criterion 17 that the outcome alone would not catch: a card's journal and an
// attachment's payload are each refused by Bench.ResolveReference and answered
// by Bench.ResolvePath, so the server's second resolver is what produces their
// answer and the set of files stays the library's.
func TestTheFallThroughIsPinnedRatherThanItsOutcome(t *testing.T) {
	f := build(t)
	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ref := card.Ref(f.bench.Slug)
	for _, spelling := range []string{ref + "/journal", ref + "/attachments/1/payload"} {
		entity, collection, err := f.bench.ResolveReference(spelling)
		if err == nil {
			t.Fatalf("%s resolved to an entity or a collection: %v %v", spelling, entity, collection)
		}
		if name := refusalNameOf(err); name != "dinah.unknown-path" {
			t.Errorf("%s was refused %s, wanted dinah.unknown-path", spelling, name)
		}
		path, err := f.bench.ResolvePath(spelling)
		if err != nil {
			t.Errorf("%s was refused by the path resolver too: %v", spelling, err)
			continue
		}
		if path == "" {
			t.Errorf("%s answered an empty path", spelling)
		}
	}
	// The accepting case beside the refusing one: two spellings the entity
	// resolver does answer, which therefore never reach the fall-through.
	for _, spelling := range []string{ref + "/comments/1", ref + "/questions"} {
		if _, _, err := f.bench.ResolveReference(spelling); err != nil {
			t.Errorf("%s was refused by the entity resolver: %v", spelling, err)
		}
	}
}

// TestTheInlineAnnotationSetIsTheFiveKinds asserts dinah-515 criterion 17: a
// document carrying a reference of every kind yields exactly five inlay
// hints, on the five the contract enumerates, and the four further forms
// answer hover and a link while yielding no hint and no annotations entry.
func TestTheInlineAnnotationSetIsTheFiveKinds(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)
	h.notify(methodDidChangeConfiguration, map[string]any{"settings": map[string]any{"dinah.lsp": map[string]any{"annotateProse": true}}})
	h.settle()

	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ref := card.Ref(f.bench.Slug)
	inline := []string{ref, ref + "/questions/1", f.bench.Columns[0].ID, "workstream/" + f.workstreamSlug, ref + "/attachments/1"}
	further := []string{ref + "/comments/1", ref + "/questions", ref + "/journal", ref + "/attachments/1/payload"}
	var lines []string
	for _, write := range append(append([]string{}, inline...), further...) {
		lines = append(lines, "Here stands "+write+" alone.")
	}
	doc := filepath.Join(filepath.Dir(f.bench.Root), "set.md")
	uri := h.openText(doc, strings.Join(lines, "\n")+"\n")

	hints := decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": uri}}))
	if len(hints) != len(inline) {
		t.Fatalf("the fixture carries %d annotated kinds and %d further forms, and the server drew %d hints: %+v",
			len(inline), len(further), len(hints), hints)
	}
	for _, hint := range hints {
		if hint.Position.Line >= len(inline) {
			t.Errorf("a hint stands on line %d, which carries one of the four further forms", hint.Position.Line)
		}
	}

	answer := decode[annotationsResult](t, h.send(methodAnnotations, map[string]any{"textDocument": map[string]any{"uri": uri}}))
	if len(answer.Annotations) != len(inline) {
		t.Errorf("dinah/annotations carried %d entries, wanted %d", len(answer.Annotations), len(inline))
	}

	swept := 0
	for offset, write := range further {
		line := len(inline) + offset
		at := map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": line, "character": 12},
		}
		hover := decode[hoverResult](t, h.send(methodHover, at))
		if hover.Contents.Value == "" {
			t.Errorf("%s answered no hover", write)
		}
		found := 0
		for _, link := range decode[[]documentLink](t, h.send(methodDocumentLink, map[string]any{"textDocument": map[string]any{"uri": uri}})) {
			if link.Range.Start.Line == line {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%s answered %d document links, wanted one", write, found)
		}
		swept++
	}
	if swept != len(further) {
		t.Errorf("swept %d further forms, wanted %d", swept, len(further))
	}
}

// TestAnEmptyCollectionAnswersHoverAndNeitherLinkNorDefinition asserts the
// last row of dinah-515 criterion 26: a collection holding no member has no
// first member's anchor to open, so it answers hover and nothing else.
func TestAnEmptyCollectionAnswersHoverAndNeitherLinkNorDefinition(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)

	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.other)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// The second card carries no checklist at all, so its questions
	// collection holds no member.
	empty := card.Ref(f.bench.Slug) + "/questions"
	doc := filepath.Join(filepath.Dir(f.bench.Root), "empty.md")
	uri := h.openText(doc, "Nothing stands at "+empty+" yet.\n")

	at := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": 0, "character": 20},
	}
	if hover := decode[hoverResult](t, h.send(methodHover, at)); hover.Contents.Value == "" {
		t.Error("an empty collection answered no hover")
	}
	if links := decode[[]documentLink](t, h.send(methodDocumentLink, map[string]any{"textDocument": map[string]any{"uri": uri}})); len(links) != 0 {
		t.Errorf("an empty collection answered %d document links, wanted none", len(links))
	}
	if string(h.send(methodDefinition, at)) != "null" {
		t.Error("an empty collection answered a definition, and it has no member's anchor to open")
	}
}

// TestCardCompletionIsCappedAndSaysSo asserts dinah-515 criterion 18: against
// a workbench carrying more than the cap, completion answers exactly the cap
// and reports the list incomplete; against a smaller one it answers all of
// them and reports it whole. Both halves name the number the fixture held.
func TestCardCompletionIsCappedAndSaysSo(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)

	anchor := filepath.Join(f.bench.CardsRoot(), f.card, bench.CardAnchor)
	text := "---\ntitle: A card\nlinks:\n  - kind: relates_to\n    to: \n---\n"
	ask := func() completionList {
		uri := h.openText(anchor, text)
		return decode[completionList](t, h.send(methodCompletion, map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": 4, "character": 8},
		}))
	}

	small := ask()
	held := cardsOf(f.bench)
	if held >= completionCap {
		t.Fatalf("the fixture already holds %d cards, so the whole-list half proves nothing", held)
	}
	if small.IsIncomplete {
		t.Errorf("a workbench of %d cards answered an incomplete list", held)
	}
	if len(small.Items) != held {
		t.Errorf("a workbench of %d cards answered %d candidates", held, len(small.Items))
	}

	for len(mustCards(t, f)) <= completionCap {
		f.add(t, "Filler")
	}
	grown := len(mustCards(t, f))
	// The server holds its own workbench, so it has to be told the fixture
	// moved before it can answer over the larger one.
	h.server.mu.Lock()
	h.server.openAt(f.root)
	h.server.mu.Unlock()

	large := ask()
	if !large.IsIncomplete {
		t.Errorf("a workbench of %d cards answered a complete list", grown)
	}
	if len(large.Items) != completionCap {
		t.Errorf("a workbench of %d cards answered %d candidates, wanted the cap of %d", grown, len(large.Items), completionCap)
	}
}

// mustCards lists the fixture's live cards, failing the test rather than
// answering a count nothing read.
func mustCards(t *testing.T, f *fixture) []string {
	t.Helper()
	f.reopen(t)
	cards, err := f.bench.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	ids := make([]string, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.ID)
	}
	return ids
}

// TestTheScopeRootBoundsWhatIsAnnotated asserts dinah-515 criterion 19: a
// document under the scope root is annotated and a byte-identical one outside
// it is not, and front-matter handling is narrower still.
func TestTheScopeRootBoundsWhatIsAnnotated(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)
	h.notify(methodDidChangeConfiguration, map[string]any{"settings": map[string]any{"dinah.lsp": map[string]any{"annotateProse": true}}})
	h.settle()

	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	text := "The work is on " + card.Ref(f.bench.Slug) + " today.\n"

	inside := h.openText(filepath.Join(f.bench.Root, "notes.md"), text)
	if hints := decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": inside}})); len(hints) != 1 {
		t.Fatalf("a document under the scope root drew %d hints, wanted one", len(hints))
	}

	outside := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(outside, []byte(text), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	outsideURI := h.openText(outside, text)
	if hints := decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": outsideURI}})); len(hints) != 0 {
		t.Errorf("a byte-identical document outside the scope root drew %d hints, wanted none", len(hints))
	}
	if links := decode[[]documentLink](t, h.send(methodDocumentLink, map[string]any{"textDocument": map[string]any{"uri": outsideURI}})); len(links) != 0 {
		t.Errorf("a document outside the scope root answered %d links, wanted none", len(links))
	}

	// The narrower front-matter handling: card.md under the workbench root is
	// read as a schema, and a file of the same name outside it is prose.
	//
	// The column is named by its title rather than by its identifier, and
	// that is what makes the pair a discriminator. A title is recognised at a
	// declared front-matter position, Bench.ColumnByRef accepting it, and it
	// is no prose candidate at all, so the second file drawing nothing is
	// evidence that its front matter was read as prose rather than evidence
	// that the file was ignored. Naming the column by its identifier would
	// prove neither thing, an identifier being a prose candidate in its own
	// right.
	title := f.bench.Columns[0].Title
	if title == "" || title == f.bench.Columns[0].ID {
		t.Fatalf("the fixture's first column is titled %q, which this half cannot tell from an identifier", title)
	}
	schema := "---\ntitle: A card\ncolumn: " + title + "\n---\n"
	anchorURI := h.openText(filepath.Join(f.bench.CardsRoot(), f.card, bench.CardAnchor), schema)
	if hints := decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": anchorURI}})); len(hints) != 1 {
		t.Errorf("card.md under the workbench root drew %d hints over its column position, wanted one", len(hints))
	}
	proseURI := h.openText(filepath.Join(scopeRootOf(f.bench), bench.CardAnchor), schema)
	if hints := decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": proseURI}})); len(hints) != 0 {
		t.Errorf("a card.md outside the workbench's own root drew %d hints (%+v) over %q, and it is prose throughout", len(hints), hints, schema)
	}
}
