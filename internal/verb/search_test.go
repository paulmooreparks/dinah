package verb

import (
	"fmt"
	"math/rand"
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
// It is not dinah-268 AC-5, which asks for a title carrying a transposition
// and has a case of its own below. The two shapes are worth separate fixtures
// because they were not always both findable: the matcher this layer began as
// found a dropped letter and could never have found a swap, and this case is
// what proves the widening kept the shape it already had.
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
// The comment carries the transposition the criterion names, which is a shape
// layer 2 does now find when it sits in a title. So the fixture is armed: a
// build that let the fuzzy layer reach comments answers a hit here and fails,
// where a fixture carrying a shape no matcher finds would have passed whatever
// the layer reached.
func TestLayerTwoDoesNotReachAComment(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card with no planted word in its title")
	h.comment(ref, "Coelacnath")
	results := found(h, &Request{SearchText: planted})
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

// swapped is the phrase of TestLayerTwoFindsATitleWithTwoLettersSwapped and
// the fixtures built on it: the planted word with one adjacent pair of letters
// exchanged, which is the shape dinah-268 AC-5 names.
//
// Its length is deliberate and dinah-268 OQ-4 is what makes it so. Layer 2
// does not run at all for a phrase of fuzzyFloor runes or fewer, so a fixture
// built on a two- or three-letter word would answer nothing here whatever the
// matcher did, and the case would look like a broken matcher rather than a
// phrase below the floor. Ten runes sits well clear of that floor, and it is
// the length precedent already uses.
const swapped = "coelacnath"

// TestLayerTwoFindsATitleWithTwoLettersSwapped asserts dinah-268 AC-5: a title
// carrying the phrase with one adjacent pair of letters exchanged is answered,
// under the title, by layer 2, where no substring of any field carries the
// phrase at all.
//
// This is the case the matcher was widened for. A transposition leaves a
// string of the phrase's own length that is not the phrase, so containment
// finds it never and an ordered subsequence finds it never either, since a
// string is a subsequence of another of equal length only when the two are
// equal. An alignment distance prices the swap at one edit and finds it.
//
// The second row is what arms the first against the algorithm rather than
// against the layer. Its phrase is seven runes, so the budget is exactly one
// edit, and a matcher that priced an adjacent swap at two, which is what a
// plain Levenshtein distance does, answers nothing for it. The ten-rune row
// alone would go on passing against such a matcher, since a budget of two
// absorbs the swap either way.
func TestLayerTwoFindsATitleWithTwoLettersSwapped(t *testing.T) {
	for _, c := range []struct {
		what   string
		phrase string
		title  string
	}{
		{"a budget of two", planted, "Coelacnath"},
		{"a budget of one, where the swap must cost one edit", "narwhal", "Narhwal"},
	} {
		t.Run(c.what, func(t *testing.T) {
			h := newHarness(t)
			ref := h.add(c.title)
			results := found(h, &Request{SearchText: c.phrase})
			if len(results.Hits) != 1 {
				t.Fatalf("wanted the one fuzzy hit, got %+v", results.Hits)
			}
			if results.Hits[0].Ref != ref || results.Hits[0].MatchedIn != MatchedInTitle {
				t.Errorf("the hit is %s/%s, wanted the title of %s",
					results.Hits[0].Ref, results.Hits[0].MatchedIn, ref)
			}
		})
	}
}

// TestAFuzzyTitleHitScoresInsideItsBand asserts dinah-268 AC-18: the hit the
// case above answers scores inside [searchTiers-tierTitle+0.75,
// searchTiers-tierTitle+1), which is the band a quality of 1 - distance/n
// produces once the budget bounds the distance at a quarter of the phrase.
//
// The band's two ends are what the criterion is about. The floor keeps a
// fuzzy title hit above every score the next tier down can reach, and the
// ceiling keeps it below an exact hit on the same tier, so a mistyped match
// never crosses a tier boundary in either direction.
func TestAFuzzyTitleHitScoresInsideItsBand(t *testing.T) {
	h := newHarness(t)
	h.add("Coelacnath")
	results := found(h, &Request{SearchText: planted})
	if len(results.Hits) != 1 {
		t.Fatalf("wanted the one fuzzy hit, got %+v", results.Hits)
	}
	floor := float64(searchTiers-tierTitle) + 0.75
	ceiling := float64(searchTiers-tierTitle) + 1
	if score := results.Hits[0].Score; score < floor || score >= ceiling {
		t.Errorf("the fuzzy title hit scored %g, wanted it inside [%g, %g)", score, floor, ceiling)
	}
}

// TestTheAlignmentDistancePricesOneSwapAsOneEdit asserts what dinah-268 D-9
// chose the algorithm for, and it is what arms the fixtures above and below:
// every distance those fixtures rest on is stated here and checked against the
// matcher's own arithmetic rather than counted by hand in a comment.
//
// The first row is the whole reason the matcher is an alignment distance and
// not a plain Levenshtein one. A plain Levenshtein distance prices an adjacent
// swap at 2, a deletion and an insertion, so a build that dropped the
// transposition arm of alignmentDistance answers 2 here and fails.
func TestTheAlignmentDistancePricesOneSwapAsOneEdit(t *testing.T) {
	for _, c := range []struct {
		what     string
		from, to string
		want     int
	}{
		{"one adjacent swap", planted, swapped, 1},
		{"one adjacent swap in a shorter word", "narwhal", "narhwal", 1},
		{"one letter dropped", planted, "colacanth", 1},
		{"one letter changed", planted, "coelacantz", 1},
		{"two letters changed", planted, "xoelacantz", 2},
		{"three letters changed", planted, "xoelycantz", 3},
		{"the same word", planted, planted, 0},
		{"nothing in common", planted, "qz", 10},
	} {
		t.Run(c.what, func(t *testing.T) {
			got := alignmentDistance([]rune(c.from), []rune(c.to))
			if got != c.want {
				t.Errorf("%q to %q is %d edits, wanted %d", c.from, c.to, got, c.want)
			}
		})
	}
}

// TestLayerTwoStopsAtTheTypoBudget asserts dinah-268 AC-17: a title one edit
// further from the phrase than the budget allows is not answered, and the same
// title at exactly the budget is.
//
// Both halves are needed. The far title alone would pass against a matcher
// that had stopped working, and the near title alone would pass against one
// that forgave everything, so the pair is what pins the threshold to the
// budget rather than to any other number. The distances themselves are
// asserted in the case above.
func TestLayerTwoStopsAtTheTypoBudget(t *testing.T) {
	for _, c := range []struct {
		what  string
		title string
		want  int
	}{
		{"at the budget", "xoelacantz", 1},
		{"one edit past the budget", "xoelycantz", 0},
	} {
		t.Run(c.what, func(t *testing.T) {
			h := newHarness(t)
			h.add(c.title)
			results := found(h, &Request{SearchText: planted})
			if len(results.Hits) != c.want {
				t.Errorf("a title %s answered %d hits, wanted %d: %+v",
					c.what, len(results.Hits), c.want, results.Hits)
			}
		})
	}
}

// TestLayerTwoDoesNotFireBelowItsFloor asserts dinah-268 AC-19: a phrase of
// fuzzyFloor runes or fewer answers no fuzzy hit at all, however near a title
// sits to it.
//
// Each phrase here is one adjacent swap from its own card's title, which is
// the closest a title can sit without being the phrase, so the only thing
// separating the answers is the phrase's length. The four-rune row is what
// arms the other two: it is built the same way and it does answer, so a build
// with no floor at all fails the short rows while the long one goes on
// passing. The letters are nonsense on purpose, since a real word would risk
// a substring hit somewhere in the fixture's own prose and the count would
// stop meaning what it says here.
func TestLayerTwoDoesNotFireBelowItsFloor(t *testing.T) {
	for _, c := range []struct {
		what   string
		phrase string
		title  string
		want   int
	}{
		{"two runes", "qz", "Zq", 0},
		{"three runes", "qzx", "Qxz", 0},
		{"four runes, above the floor", "qzxv", "Qxzv", 1},
	} {
		t.Run(c.what, func(t *testing.T) {
			h := newHarness(t)
			h.add(c.title)
			results := found(h, &Request{SearchText: c.phrase})
			if len(results.Hits) != c.want {
				t.Errorf("a phrase of %s answered %d hits, wanted %d: %+v",
					c.what, len(results.Hits), c.want, results.Hits)
			}
		})
	}
}

// noiseVocabulary is the word list the corpus below is generated from: words
// this workbench's own card titles are written out of. It is the source of the
// corpus's realism, and it is chosen before the phrase rather than around it.
// The one word it deliberately excludes is the phrase's own correct spelling,
// so that every title the search answers besides the planted target is noise
// by construction rather than a real match somebody would have wanted.
var noiseVocabulary = []string{
	"workbench", "column", "card", "search", "agent", "operator", "attachment",
	"comment", "workstream", "journal", "refusal", "locale", "catalog", "guide",
	"profile", "contract", "verb", "pull", "claim", "sweep", "rename", "archive",
	"capacity", "station", "lane", "ready", "blocked", "level", "severity",
	"priority", "reference", "snippet", "tree", "status", "history", "identifier",
	"discovery", "container", "anchor", "definition", "frontmatter", "listing",
	"quick", "start", "answer", "question", "decision", "criterion", "fixture",
	"boundary", "phrase", "title", "body", "text", "field", "hit", "score",
}

// noiseLengths is how many words a generated title carries, drawn from at
// random. Half the list is one or two words, because a title of about the
// phrase's own length is the only kind that can collide with it at all and a
// corpus of long ones would measure nothing.
var noiseLengths = []int{1, 1, 1, 1, 2, 2, 2, 3, 3, 4, 5, 6, 8}

// noiseCorpus generates count distinct card titles from noiseVocabulary and a
// fixed seed, so the corpus is the same on every run and on every machine and
// nobody had to choose which titles went into it one at a time.
//
// Word counts come from noiseLengths, which leans hard on the short end. That
// lean matters more than it looks, and it leans the way it does deliberately.
// An alignment distance is never smaller than the difference between two
// lengths, so a corpus of long sentences could not collide with a ten-rune
// phrase whatever its words were, and a measurement taken against one would
// report a zero it had arranged for itself. Every title short enough to
// collide is a title that might, so weighting the corpus toward short ones can
// only raise the count this case reports, never lower it.
func noiseCorpus(count int) []string {
	random := rand.New(rand.NewSource(268))
	seen := map[string]bool{}
	var titles []string
	for len(titles) < count {
		words := make([]string, noiseLengths[random.Intn(len(noiseLengths))])
		for at := range words {
			words[at] = noiseVocabulary[random.Intn(len(noiseVocabulary))]
		}
		title := strings.Join(words, " ")
		if seen[title] {
			continue
		}
		seen[title] = true
		titles = append(titles, title)
	}
	return titles
}

// mistype exchanges the two letters either side of a word's midpoint, which is
// the shape of slip the widened matcher exists to catch. It answers the empty
// string for a word too short for layer 2 to look at, since a phrase at or
// below the floor is not a measurement of anything.
func mistype(word string) string {
	letters := []rune(word)
	if len(letters) <= fuzzyFloor+1 {
		return ""
	}
	at := len(letters) / 2
	letters[at-1], letters[at] = letters[at], letters[at-1]
	return string(letters)
}

// TestTheNoiseAgainstAGeneratedCorpus asserts dinah-268 AC-20, which is a
// measurement rather than a bound: it runs a mistyped phrase against a
// generated corpus of unrelated titles and reports how many of them the widened
// matcher answers by coincidence. The count is logged so the run itself states
// it, since the criterion asks for the number to be recorded rather than
// assumed.
//
// Three things make the number worth reading. The corpus is generated from a
// seed rather than picked, so nobody removed a title that would have collided.
// The nearest distance any corpus title reached is logged beside the count,
// which says how much room the budget had left rather than only that it was
// not exceeded. And one phrase is a thin measurement, so the case goes on to
// sweep every word in the corpus's own vocabulary, mistyped the same way,
// against every title, and reports what that whole sweep answered.
//
// The sweep is where the real figure is. It is what proves the zero above is a
// property of the matcher rather than of one lucky phrase, and it is what would
// catch a budget loose enough to answer a board's worth of titles for an
// ordinary mistyped word.
//
//	go test ./internal/verb/ -run TestTheNoiseAgainstAGeneratedCorpus -v
func TestTheNoiseAgainstAGeneratedCorpus(t *testing.T) {
	const corpusSize = 300
	h := newHarness(t)
	titles := noiseCorpus(corpusSize)
	budget := typoBudget(len([]rune(swapped)))
	nearest, candidates := len(swapped)+1, 0
	for _, title := range titles {
		distance := alignmentDistance([]rune(asciiFold(swapped)), []rune(asciiFold(title)))
		if distance < nearest {
			nearest = distance
		}
		// A title can only collide when its own length is within the budget
		// of the phrase's, so this is how many of the corpus ever had the
		// chance to.
		if difference := len([]rune(title)) - len([]rune(swapped)); difference <= budget && -difference <= budget {
			candidates++
		}
		h.add(title)
	}
	target := h.add("Coelacanth")
	results := found(h, &Request{SearchText: swapped})

	var noise []string
	sawTarget := false
	for _, hit := range results.Hits {
		if hit.Ref == target {
			sawTarget = true
			continue
		}
		noise = append(noise, hit.Ref+" "+hit.Title)
	}
	t.Logf("dinah-268 AC-20: a corpus of %d generated titles, %d of them within the %d-edit budget's own length window, answered %d unrelated hits for the phrase %q; the nearest unrelated title sat %d edits away",
		len(titles), candidates, budget, len(noise), swapped, nearest)
	for _, one := range noise {
		t.Logf("dinah-268 AC-20: unrelated hit: %s", one)
	}
	if !sawTarget {
		t.Errorf("the intended target was not answered at all, so the count above measures nothing")
	}

	// One phrase against one corpus is a thin measurement, so the same corpus
	// is swept with every mistyping the vocabulary itself yields. Each sweep
	// phrase is a real word with two of its letters exchanged, which is the
	// mistake this widening exists to catch, and every title the matcher
	// answers that is not that word itself is noise.
	intended, noisy, worst, worstPhrase := 0, 0, 0, ""
	for _, word := range noiseVocabulary {
		phrase := mistype(word)
		if phrase == "" {
			continue
		}
		here := 0
		for _, title := range titles {
			if _, ok := withinTypoBudget(phrase, title); !ok {
				continue
			}
			if asciiFold(title) == word {
				intended++
				continue
			}
			here++
			if here <= 3 {
				t.Logf("dinah-268 AC-20: %q answered the unrelated title %q", phrase, title)
			}
		}
		noisy += here
		if here > worst {
			worst, worstPhrase = here, phrase
		}
	}
	loudest := "no single phrase answered any"
	if worst > 0 {
		loudest = fmt.Sprintf("the loudest single phrase was %q with %d", worstPhrase, worst)
	}
	t.Logf("dinah-268 AC-20: sweeping %d mistyped vocabulary words across the same %d titles answered %d unrelated titles in total, %s, and found the word actually meant %d times",
		len(noiseVocabulary), len(titles), noisy, loudest, intended)
	if intended == 0 {
		t.Errorf("the sweep never found a word it was a mistyping of, so it measured nothing")
	}
	if candidates < 20 {
		t.Errorf("only %d of %d corpus titles were close enough in length to collide with the phrase, which is too few for its own count to mean anything",
			candidates, len(titles))
	}
	// The criterion asks for a count that is small relative to the corpus,
	// single digits, and says plainly that it is not a proof of zero. This is
	// that bound and nothing more: a build whose matcher went loose enough to
	// answer a tenth of a board fails here.
	if len(noise) > 9 {
		t.Errorf("the search answered %d unrelated titles out of %d, wanted single digits", len(noise), len(titles))
	}
}
