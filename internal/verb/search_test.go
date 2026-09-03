package verb

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// planted is the word the fixtures below plant, chosen because nothing in the
// fixture definition, in any title Instantiate writes, or in any reference the
// bench mints carries it. A search for it therefore answers with what the
// fixture put in and nothing else, so a hit nobody planted fails the test
// rather than hiding among hits somebody did.
const planted = "coelacanth"

// searchPlant names one place a fixture plants the phrase. The seven of them
// are the seven fields dinah-268 D-1 puts inside the scan's reach.
type searchPlant string

const (
	plantTitle      searchPlant = "card title"
	plantFraming    searchPlant = "card framing"
	plantComment    searchPlant = "card comment"
	plantAttachment searchPlant = "card attachment"
	plantBench      searchPlant = "workbench body"
	plantColumn     searchPlant = "column instructions"
	plantStream     searchPlant = "workstream notes"
)

// searchPlants is the whole set, in the order the ranking answers them.
var searchPlants = []searchPlant{
	plantTitle, plantFraming, plantComment, plantAttachment,
	plantBench, plantColumn, plantStream,
}

// wantedHit is the kind and the field a plant is expected to answer under.
var wantedHit = map[searchPlant][2]string{
	plantTitle:      {SearchKindCard, MatchedInTitle},
	plantFraming:    {SearchKindCard, MatchedInFraming},
	plantComment:    {SearchKindCard, MatchedInComment},
	plantAttachment: {SearchKindCard, MatchedInAttachment},
	plantBench:      {SearchKindWorkbench, MatchedInFraming},
	plantColumn:     {SearchKindColumn, MatchedInFraming},
	plantStream:     {SearchKindWorkstream, MatchedInFraming},
}

// plantAll builds the seven-way fixture with one plant left out, and answers
// the card it planted on. Passing the empty string leaves all seven in.
//
// Leaving one out is what arms every assertion built on this fixture: the
// hits the whole fixture answers with are only evidence that the scan reaches
// a field if removing that field's own text takes its hit away and leaves the
// other six standing.
func plantAll(h *harness, omit searchPlant) string {
	h.t.Helper()
	ref := h.add(title(omit == plantTitle))
	if omit != plantFraming {
		plantFramingOn(h, ref, "The framing mentions "+planted+" once.")
	}
	if omit != plantComment {
		h.comment(ref, "The comment mentions "+planted+" once.")
	}
	if omit != plantAttachment {
		h.attach(ref, "notes.txt", "The attachment mentions "+planted+" once.\n")
	}
	if omit != plantBench {
		appendTo(h, filepath.Join(h.root, bench.WorkbenchAnchor), "The workbench body mentions "+planted+".\n")
	}
	if omit != plantColumn {
		appendTo(h, h.library.Bench.ColumnAnchorPath(intake), "The column mentions "+planted+".\n")
	}
	id, _ := h.workstream("Fossil record")
	if omit != plantStream {
		appendTo(h, h.library.Bench.WorkstreamAnchorPath(id), "The workstream mentions "+planted+".\n")
	}
	h.reopen()
	return ref
}

// title is the fixture card's title, carrying the phrase unless the title is
// the plant being left out.
func title(omit bool) string {
	if omit {
		return "A card with no planted word in its title"
	}
	return "A card about the " + planted
}

// plantFramingOn writes framing prose onto a card, which no verb does: a
// card's body is written by a person editing the file, and every reader of it
// reads what that person wrote.
func plantFramingOn(h *harness, ref, body string) {
	h.t.Helper()
	appendTo(h, filepath.Join(h.library.Bench.CardsRoot(), h.cardID(ref), bench.CardAnchor), body+"\n")
}

// appendTo adds a line to the body of an anchor already on disk.
func appendTo(h *harness, path, text string) {
	h.t.Helper()
	source, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read %s: %v", path, err)
	}
	if err := bench.WriteText(path, source+text); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
	h.reopen()
}

// found runs a search and fails the test unless it answered.
func found(h *harness, req *Request) *SearchResults {
	h.t.Helper()
	req.Verb = "search"
	if req.Actor == "" {
		req.Actor = "alka"
	}
	results, err := h.library.Search(req)
	if err != nil {
		h.t.Fatalf("search %q: %v", req.SearchText, err)
	}
	return results
}

// pairs is the kind and matched field of every hit, sorted, which is what the
// reach assertions compare.
func pairs(results *SearchResults) []string {
	var seen []string
	for _, hit := range results.Hits {
		seen = append(seen, hit.Kind+"/"+hit.MatchedIn)
	}
	sort.Strings(seen)
	return seen
}

// wantedPairs is the same shape built from the plants a fixture holds.
func wantedPairs(omit searchPlant) []string {
	var wanted []string
	for _, plant := range searchPlants {
		if plant == omit {
			continue
		}
		pair := wantedHit[plant]
		wanted = append(wanted, pair[0]+"/"+pair[1])
	}
	sort.Strings(wanted)
	return wanted
}

// TestASearchReachesEverySearchedField asserts dinah-268 AC-1: a phrase
// planted in a card's title, its framing, one comment and one text attachment,
// and in the workbench's own body, one column's instructions and one
// workstream's notes, comes back as seven hits naming exactly those seven
// places.
func TestASearchReachesEverySearchedField(t *testing.T) {
	h := newHarness(t)
	plantAll(h, "")
	results := found(h, &Request{SearchText: planted})
	if got, want := strings.Join(pairs(results), " "), strings.Join(wantedPairs(""), " "); got != want {
		t.Errorf("the search answered %q, wanted %q", got, want)
	}
	if results.Count != len(results.Hits) {
		t.Errorf("the count is %d and the array holds %d", results.Count, len(results.Hits))
	}
	if results.Text != planted {
		t.Errorf("the phrase came back as %q, wanted it echoed as %q", results.Text, planted)
	}
}

// TestRemovingOnePlantRemovesOneHit asserts dinah-268 AC-2, and is what arms
// the case above: each of the seven fields is emptied of the phrase in turn,
// and exactly that field's hit goes while the other six stay. A scan that
// never read one of those fields fails here rather than passing the case
// above on six hits and a coincidence.
func TestRemovingOnePlantRemovesOneHit(t *testing.T) {
	for _, omit := range searchPlants {
		t.Run(string(omit), func(t *testing.T) {
			h := newHarness(t)
			plantAll(h, omit)
			results := found(h, &Request{SearchText: planted})
			got := strings.Join(pairs(results), " ")
			if want := strings.Join(wantedPairs(omit), " "); got != want {
				t.Errorf("with the %s plant removed the search answered %q, wanted %q", omit, got, want)
			}
		})
	}
}

// TestATitleOutranksAComment asserts dinah-268 AC-3: one phrase in one card's
// title and in another card's comment ranks the title hit first, and scores it
// higher, because where the phrase matched outranks how well it matched.
//
// The title is long and the comment is the phrase and nothing else, so the
// comment covers its whole field and the title covers a fraction of one. Only
// the tier can put the title first, and a build that ranked on match quality
// alone fails here. A fixture with a short title would pass either way and
// would prove nothing about the ordering it names.
func TestATitleOutranksAComment(t *testing.T) {
	h := newHarness(t)
	h.add("A long card title that says " + planted + " somewhere along its length")
	second := h.add("A card with a comment")
	h.comment(second, planted)
	results := found(h, &Request{SearchText: planted})
	if len(results.Hits) != 2 {
		t.Fatalf("wanted the two planted hits, got %d: %+v", len(results.Hits), results.Hits)
	}
	if results.Hits[0].MatchedIn != MatchedInTitle {
		t.Errorf("the first hit matched in %s, wanted the title", results.Hits[0].MatchedIn)
	}
	if results.Hits[1].MatchedIn != MatchedInComment {
		t.Errorf("the second hit matched in %s, wanted the comment", results.Hits[1].MatchedIn)
	}
	if results.Hits[0].Score <= results.Hits[1].Score {
		t.Errorf("the title scored %g and the comment %g, wanted the title higher",
			results.Hits[0].Score, results.Hits[1].Score)
	}
}

// TestAReferenceMatchOutranksEverything asserts dinah-268 AC-4: a phrase that
// is a card's own human reference answers that card first, under the reference
// field, even where another card carries the same literal string in its own
// title and framing and therefore scores at a lower tier.
func TestAReferenceMatchOutranksEverything(t *testing.T) {
	h := newHarness(t)
	named := h.add("The card being named")
	other := h.add("A card naming " + named + " in its title")
	plantFramingOn(h, other, "The framing names "+named+" too.")
	results := found(h, &Request{SearchText: named})
	if len(results.Hits) < 3 {
		t.Fatalf("wanted the reference hit and the two on the other card, got %+v", results.Hits)
	}
	first := results.Hits[0]
	if first.MatchedIn != MatchedInReference || first.Ref != named {
		t.Errorf("the first hit is %s on %s, wanted the reference hit on %s", first.MatchedIn, first.Ref, named)
	}
	for _, hit := range results.Hits[1:] {
		if hit.Score >= first.Score {
			t.Errorf("%s/%s scored %g and the reference hit scored %g, wanted the reference first",
				hit.Ref, hit.MatchedIn, hit.Score, first.Score)
		}
	}
}

// TestLayerTwoFindsATitleThePhraseIsMissingALetterOf asserts what layer 2 is
// for: a phrase whose letters all occur in a title, in order, is answered even
// where no substring of the title carries it, which is the shape a caller
// typing from memory produces.
//
// It is not dinah-268 AC-5, and cannot be. AC-5 asks for a title carrying a
// transposition of the phrase, and a transposition is the one shape a
// subsequence matcher can never find: swapping two adjacent characters leaves
// a string of the phrase's own length that is not the phrase, and a string is
// a subsequence of another string of equal length only when the two are
// equal. That criterion is left pending with the contradiction recorded on the
// card rather than met by a fixture built to look as though it were.
func TestLayerTwoFindsATitleThePhraseIsMissingALetterOf(t *testing.T) {
	h := newHarness(t)
	ref := h.add("Coelacanth")
	results := found(h, &Request{SearchText: "colacanth"})
	if len(results.Hits) != 1 {
		t.Fatalf("wanted the one fuzzy hit, got %+v", results.Hits)
	}
	if results.Hits[0].Ref != ref || results.Hits[0].MatchedIn != MatchedInTitle {
		t.Errorf("the hit is %s/%s, wanted the title of %s", results.Hits[0].Ref, results.Hits[0].MatchedIn, ref)
	}
	if quality := results.Hits[0].Score - float64(searchTiers-tierTitle); quality <= 0 || quality > 1 {
		t.Errorf("the quality behind the score is %g, wanted a value inside (0, 1]", quality)
	}
}

// TestLayerTwoDoesNotReachAComment asserts dinah-268 AC-6: the fuzzy layer is
// scoped to titles, so a comment carrying the phrase only as a subsequence
// answers nothing.
//
// The criterion's own fixture says the comment carries a transposition. A
// transposition is found by no matcher this card ships, so a fixture built on
// one would pass whether or not layer 2 reached comments, and would prove
// nothing. The fixture here plants the shape layer 2 does find, so a build
// that let the fuzzy layer reach comments fails this test by answering a hit.
func TestLayerTwoDoesNotReachAComment(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card with no planted word in its title")
	h.comment(ref, "Coelacanth")
	results := found(h, &Request{SearchText: "colacanth"})
	if len(results.Hits) != 0 {
		t.Errorf("wanted no hit, got %+v", results.Hits)
	}
}

// TestAnAttachmentIsSearchedUpToTheCapAndNoFurther asserts dinah-268 AC-7: a
// payload of 65537 bytes carrying the phrase at byte 65536 answers nothing,
// and the same payload with the phrase one byte earlier answers a hit.
//
// The phrase is one byte long because the criterion's own arithmetic requires
// it: a 65537-byte payload leaves exactly one byte past the 65536-byte cap, so
// no longer phrase can sit wholly beyond it in a file of that size.
func TestAnAttachmentIsSearchedUpToTheCapAndNoFurther(t *testing.T) {
	// The two offsets and the payload's length are written out rather than
	// derived from the cap the code declares. A fixture built from that
	// constant moves whenever the constant does, so it would go on passing
	// against a build that read any number of bytes at all.
	const (
		mark  = "~"
		bytes = 65537
	)
	for _, c := range []struct {
		name string
		at   int
		want int
	}{
		{name: "past the cap", at: 65536, want: 0},
		{name: "inside the cap", at: 65535, want: 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			ref := h.add("A card carrying a long attachment")
			payload := []byte(strings.Repeat("a", bytes))
			payload[c.at] = mark[0]
			h.attach(ref, "notes.txt", string(payload))
			results := found(h, &Request{SearchText: mark})
			if len(results.Hits) != c.want {
				t.Errorf("with the mark at byte %d the search answered %d hits, wanted %d", c.at, len(results.Hits), c.want)
			}
		})
	}
}

// TestAnAttachmentNamedForAPictureIsNotSearched asserts dinah-268 AC-8: bytes
// that happen to spell the phrase inside a file named for a format this tool
// does not read as text are not an occurrence of it.
func TestAnAttachmentNamedForAPictureIsNotSearched(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card carrying a picture")
	h.attach(ref, "diagram.png", "\x89PNG\r\n\x1a\n"+planted+"\n")
	if results := found(h, &Request{SearchText: planted}); len(results.Hits) != 0 {
		t.Errorf("wanted no hit against the picture, got %+v", results.Hits)
	}
}

// TestTheFilterNarrowsTheCardsSearched asserts dinah-268 AC-9: the query
// string narrows which cards the phrase is run over, and its absence leaves
// every card in.
func TestTheFilterNarrowsTheCardsSearched(t *testing.T) {
	h := newHarness(t)
	here := h.add("A " + planted + " here")
	there := h.add("A " + planted + " there")
	h.at(there, doing)
	if results := found(h, &Request{SearchText: planted}); len(results.Hits) != 2 {
		t.Fatalf("without a filter the search answered %d hits, wanted both cards", len(results.Hits))
	}
	results := found(h, &Request{SearchText: planted, Query: "column:" + intake})
	if len(results.Hits) != 1 || results.Hits[0].Ref != here {
		t.Errorf("the filtered search answered %+v, wanted the one card at intake", results.Hits)
	}
	if results.Filter != "column:"+intake {
		t.Errorf("the filter came back as %q, wanted it echoed", results.Filter)
	}
}

// TestAMalformedFilterIsRefusedTheWayQueryRefusesIt asserts dinah-268 AC-10:
// the filter is the query language, reached from a second verb, so a field
// name outside its twelve is refused with the same name.
func TestAMalformedFilterIsRefusedTheWayQueryRefusesIt(t *testing.T) {
	h := newHarness(t)
	h.add("A card")
	_, err := h.library.Search(&Request{Verb: "search", Actor: "alka", SearchText: planted, Query: "bogus:1"})
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.UnknownField {
		t.Errorf("search refused with %s", refusal.Name)
	}
	if want := h.refuse("bogus:1"); want.Name != refusal.Name {
		t.Errorf("query refuses the same text with %s and search with %s", want.Name, refusal.Name)
	}
}

// TestTheFilterLeavesTheWorkbenchHitAlone asserts dinah-268 AC-11: the twelve
// field names mean nothing for a workbench, a column or a workstream, so a
// filter narrows the cards and leaves those three where they are.
func TestTheFilterLeavesTheWorkbenchHitAlone(t *testing.T) {
	h := newHarness(t)
	appendTo(h, filepath.Join(h.root, bench.WorkbenchAnchor), "The workbench body mentions "+planted+".\n")
	elsewhere := h.add("A " + planted + " on a card")
	h.at(elsewhere, doing)
	results := found(h, &Request{SearchText: planted, Query: "column:" + intake})
	if len(results.Hits) != 1 {
		t.Fatalf("wanted the workbench hit alone, got %+v", results.Hits)
	}
	if results.Hits[0].Kind != SearchKindWorkbench {
		t.Errorf("the surviving hit is a %s, wanted the workbench", results.Hits[0].Kind)
	}
}

// TestTheArchiveIsSearchedOnlyWhenAsked asserts dinah-268 AC-12: the archived
// half is left out on the format's own read-on-demand rule, and a hit drawn
// from it says so.
func TestTheArchiveIsSearchedOnlyWhenAsked(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A " + planted + " filed long ago")
	h.archive(ref)
	if results := found(h, &Request{SearchText: planted}); len(results.Hits) != 0 {
		t.Fatalf("the live scan answered %+v, wanted nothing", results.Hits)
	}
	results := found(h, &Request{SearchText: planted, Archived: true})
	if len(results.Hits) != 1 {
		t.Fatalf("the archived scan answered %d hits, wanted the one archived card", len(results.Hits))
	}
	if !results.Hits[0].Archived {
		t.Errorf("the hit does not say it came from the archive: %+v", results.Hits[0])
	}
	if !results.Archived {
		t.Error("the answer does not record that the scan included the archive")
	}
}

// TestAnEmptyPhraseIsRefused asserts dinah-268 AC-13: a search with nothing to
// search for is a caller's mistake rather than a request for the whole
// workbench.
func TestAnEmptyPhraseIsRefused(t *testing.T) {
	h := newHarness(t)
	h.add("A card")
	for _, text := range []string{"", "   "} {
		_, err := h.library.Search(&Request{Verb: "search", Actor: "alka", SearchText: text})
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("%q: wanted a refusal, got %v", text, err)
		}
		if refusal.Name != contract.EmptySearch {
			t.Errorf("%q: refused with %s, wanted %s", text, refusal.Name, contract.EmptySearch)
		}
	}
}

// TestASearchForestAnswersPerWorkbench asserts the grouping half of dinah-268
// AC-14 at the library: a walk answers one member per workbench beneath the
// root, each carrying that workbench's own hits, and it refuses an empty
// phrase once rather than in every member.
func TestASearchForestAnswersPerWorkbench(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one", "two"} {
		h := newHarness(t)
		h.add("A " + planted + " in " + name)
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := copyTree(filepath.Join(h.root), filepath.Join(root, name, bench.UserBaseName, harnessWorkbenchID)); err != nil {
			t.Fatalf("copy: %v", err)
		}
	}
	answer, err := SearchForest(root, "", &Request{Verb: "search", Actor: "alka", SearchText: planted}, 0)
	if err != nil {
		t.Fatalf("the walk: %v", err)
	}
	if len(answer.Workbenches) != 2 {
		t.Fatalf("the walk answered for %d workbenches, wanted two", len(answer.Workbenches))
	}
	for _, member := range answer.Workbenches {
		if member.Results == nil || len(member.Results.Hits) != 1 {
			t.Errorf("%s answered %+v, wanted its own one hit", member.Path, member.Results)
		}
	}
	if _, err := SearchForest(root, "", &Request{Verb: "search", Actor: "alka", SearchText: ""}, 0); err == nil {
		t.Error("the walk accepted an empty phrase")
	}
}

// copyTree copies a directory tree, which is how the walk above gets two
// workbenches carrying the same fixture under one root.
func copyTree(from, to string) error {
	return filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
	})
}

// TestASnippetIsBoundedAndMarksWhatItLeftOut asserts what a caller reads
// beside a hit: a field that fits comes back whole, and one that does not
// comes back centred on the match, inside the cap, marked at each end it cut.
func TestASnippetIsBoundedAndMarksWhatItLeftOut(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card with a long framing")
	filler := strings.Repeat("filler ", 200)
	plantFramingOn(h, ref, filler+planted+" "+filler)
	results := found(h, &Request{SearchText: planted})
	if len(results.Hits) != 1 {
		t.Fatalf("wanted the one framing hit, got %+v", results.Hits)
	}
	snippet := results.Hits[0].Snippet
	if !strings.Contains(snippet, planted) {
		t.Errorf("the snippet does not carry the matched text: %q", snippet)
	}
	if len(snippet) > snippetCap+len("...")*2 {
		t.Errorf("the snippet is %d bytes, past the cap of %d and its two markers", len(snippet), snippetCap)
	}
	if !strings.HasPrefix(snippet, "...") || !strings.HasSuffix(snippet, "...") {
		t.Errorf("the snippet cut both ends and marked neither: %q", snippet)
	}
	short := found(h, &Request{SearchText: "long framing"})
	if len(short.Hits) != 1 || strings.Contains(short.Hits[0].Snippet, "...") {
		t.Errorf("a field inside the cap came back marked as cut: %+v", short.Hits)
	}
}
