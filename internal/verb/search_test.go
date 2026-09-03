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
		{"a ten-rune phrase", planted, "Coelacnath"},
		{"a seven-rune phrase, where the swap must cost one edit", "narwhal", "Narhwal"},
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
//
// The two titles moved one edit nearer the phrase when D-11 replaced the
// proportional budget with the square-root one. The phrase is ten runes, where
// the first version allowed two edits and this one allows the floor of one, so
// the pair that sits either side of the threshold is now a distance of one
// against a distance of two rather than two against three. Anything else here
// would be measuring the old number.
func TestLayerTwoStopsAtTheTypoBudget(t *testing.T) {
	if budget := typoBudget(len([]rune(planted))); budget != 1 {
		t.Fatalf("the fixture below is built for a budget of 1 at %d runes, but the budget is %d",
			len([]rune(planted)), budget)
	}
	for _, c := range []struct {
		what  string
		title string
		want  int
	}{
		{"at the budget", "coelacantz", 1},
		{"one edit past the budget", "xoelacantz", 0},
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

// mistype exchanges an adjacent pair of letters near a word's midpoint, which
// is the shape of slip the widened matcher exists to catch. It answers the
// empty string for a word too short for layer 2 to look at, and that is
// exactly the words at or below fuzzyFloor, since withinTypoBudget refuses a
// phrase whose length is at or below the floor and admits every longer one. A
// four-rune word is therefore swept rather than skipped, and it is the one
// this sweep can least afford to miss: a budget of one forgiven edit against
// four runes is the largest ratio of tolerance to phrase the design ever
// allows, so it is the loudest regime the matcher has.
//
// The pair it exchanges is the one nearest the midpoint whose two letters
// differ, and searching outward for that pair is not fussiness. Exchanging a
// doubled letter with itself answers the word back unchanged, so the phrase
// would be the word rather than a mistyping of it, layer 1 would answer it as
// a substring, and the sweep would count a measurement of layer 2 it never
// took. The word "comment" in the vocabulary below is exactly that case, and
// the plain midpoint exchange was silently returning it whole.
func mistype(word string) string {
	letters := []rune(word)
	if len(letters) <= fuzzyFloor {
		return ""
	}
	middle := len(letters) / 2
	for step := 0; step < len(letters); step++ {
		for _, at := range [2]int{middle - step, middle + step} {
			if at < 1 || at >= len(letters) || letters[at-1] == letters[at] {
				continue
			}
			letters[at-1], letters[at] = letters[at], letters[at-1]
			return string(letters)
		}
	}
	return ""
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
	// Which vocabulary words the corpus put on a card of their own, since a
	// mistyping can only find the word it was a mistyping of where a title
	// carrying that word alone exists. This is what makes the liveness figure
	// below readable rather than a bare number to be compared against the
	// vocabulary's size and found short.
	standalone := make(map[string]bool, len(titles))
	for _, title := range titles {
		standalone[asciiFold(title)] = true
	}

	intended, noisy, worst, worstPhrase := 0, 0, 0, ""
	// swept is how many phrases the loop actually built, which is not the size
	// of the vocabulary: a word at or below the floor yields no phrase. The log
	// below reports this rather than len(noiseVocabulary), because the whole
	// product of this criterion is an honest count and a sweep that reports the
	// length of what it ranged over can overstate what it did.
	swept, reachable := 0, 0
	var unreachable []string
	for _, word := range noiseVocabulary {
		phrase := mistype(word)
		if phrase == "" {
			continue
		}
		swept++
		// A phrase equal to its own word is not a mistyping of it, and layer
		// 1 would answer it as a substring, so such a phrase measures nothing
		// about layer 2 while still being counted as swept.
		if phrase == word {
			t.Errorf("the phrase built from %q is the word itself, so it measures nothing about layer 2", word)
		}
		if standalone[word] {
			reachable++
		} else {
			unreachable = append(unreachable, word)
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
	t.Logf("dinah-268 AC-20: sweeping %d mistyped phrases, built from a vocabulary of %d words of which %d are too short for layer 2 to look at, across the same %d titles answered %d unrelated titles in total, %s, and found the word actually meant %d times",
		swept, len(noiseVocabulary), len(noiseVocabulary)-swept, len(titles), noisy, loudest, intended)
	t.Logf("dinah-268 AC-20: of those %d phrases, %d are mistypings of a word the corpus gave a title of its own and %d are not, so the second group had nothing to find: %s",
		swept, reachable, len(unreachable), strings.Join(unreachable, ", "))
	if intended == 0 {
		t.Errorf("the sweep never found a word it was a mistyping of, so it measured nothing")
	}
	// The liveness figure and the corpus have to agree exactly. Every word the
	// corpus titled on its own must be found by its own mistyping, and no word
	// it did not title can be, so a shortfall here is the sweep quietly missing
	// phrases rather than the corpus happening not to carry the word.
	if intended != reachable {
		t.Errorf("the sweep found the word actually meant %d times, but %d of the %d swept phrases are mistypings of a word the corpus gave a title of its own; every one of those must be found",
			intended, reachable, swept)
	}
	// The floor is the only reason a vocabulary word yields no phrase, so the
	// arithmetic between the vocabulary and the sweep has to close against the
	// floor itself rather than against a number written down here.
	short := 0
	for _, word := range noiseVocabulary {
		if len([]rune(word)) <= fuzzyFloor {
			short++
		}
	}
	// This, not the intended/reachable check above, is the assertion that
	// caught the guard that dropped every four-rune word: under that guard,
	// reachable dropped in step with swept and intended stayed equal to
	// reachable, so that check stayed green. Only the count here disagreed
	// with the vocabulary's own arithmetic against the floor.
	if swept != len(noiseVocabulary)-short {
		t.Errorf("the sweep built %d phrases from %d vocabulary words of which %d sit at or below the floor of %d, so it should have built %d",
			swept, len(noiseVocabulary), short, fuzzyFloor, len(noiseVocabulary)-short)
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
	// The criterion's ceiling is over the whole sweep and not only over the one
	// phrase above, and a bound that reads one phrase while logging the sweep
	// is the shape of check this card came back from Merge for. This is that
	// ceiling: fewer than ten unrelated titles across every phrase the
	// vocabulary yielded.
	if noisy > 9 {
		t.Errorf("the sweep of %d phrases answered %d unrelated titles across %d, wanted fewer than 10",
			swept, noisy, len(titles))
	}
}

// firstVersionTypoBudget is the budget dinah-268 shipped before D-11 was
// revised: max(1, n/4), growing in direct proportion to the phrase. It is
// written out here rather than referred to, because the code no longer carries
// it and a property this card asserts about it cannot rest on a formula that
// exists only in a card note.
func firstVersionTypoBudget(length int) int {
	if scaled := length / 4; scaled > 1 {
		return scaled
	}
	return 1
}

// TestTheRevisedBudgetIsNeverLooserThanTheFirstVersion asserts dinah-268 AC-21
// over the whole range rather than at a handful of lengths. A revision that
// was looser anywhere would mean a title correctly rejected before this card
// now qualifies, which would quietly reopen every fixture verified against the
// first version's numbers.
//
// The named boundaries go with it. The range check alone would pass against a
// budget that answered one at every length, which is tighter everywhere and
// also useless, so the boundaries pin the curve to the one D-11 chose: one
// through 63 runes, two at 64, still two at 143, three at 144.
func TestTheRevisedBudgetIsNeverLooserThanTheFirstVersion(t *testing.T) {
	for length := 4; length <= 400; length++ {
		revised, first := typoBudget(length), firstVersionTypoBudget(length)
		if revised > first {
			t.Fatalf("at %d runes the revised budget forgives %d edits and the first version forgave %d, so the revision is looser",
				length, revised, first)
		}
	}
	for _, c := range []struct {
		length, want int
	}{
		{4, 1}, {10, 1}, {63, 1}, {64, 2}, {143, 2}, {144, 3}, {255, 3}, {256, 4},
	} {
		if got := typoBudget(c.length); got != c.want {
			t.Errorf("the budget at %d runes is %d, wanted %d", c.length, got, c.want)
		}
	}
}

// TestTheIntegerSquareRootIsExactAtEveryBoundary arms the case above against
// the one way the budget could drift without any formula changing. A perfect
// square is where a floating-point root can answer a hair under the integer
// and floor to one less, and it is exactly where the budget steps, so the
// boundaries the criterion names are the values most exposed to it.
func TestTheIntegerSquareRootIsExactAtEveryBoundary(t *testing.T) {
	for root := 0; root <= 64; root++ {
		square := root * root
		if got := integerSquareRoot(square); got != root {
			t.Errorf("the root of %d is %d, wanted %d", square, got, root)
		}
		if square > 0 {
			if got := integerSquareRoot(square - 1); got != root-1 {
				t.Errorf("the root of %d is %d, wanted %d", square-1, got, root-1)
			}
		}
		if got := integerSquareRoot(square + root); got != root {
			t.Errorf("the root of %d is %d, wanted %d", square+root, got, root)
		}
	}
}

// noiseFillerPath is the checked-in word list dinah-268 AC-20's templated
// corpus draws from. The criterion names the path itself, so that two builds
// draw the same corpus and no build can pick fillers that dodge a collision.
const noiseFillerPath = "testdata/noise_fillers.txt"

// noiseFillerBands reads that list into its length bands. The file is one
// lowercase-ASCII word per line, and a blank line ends a band, which is the
// whole of its format: the order of the words inside a band is the chain the
// criterion asks for, and it is the order the corpus draws in.
func noiseFillerBands(t *testing.T) [][]string {
	t.Helper()
	payload, err := os.ReadFile(noiseFillerPath)
	if err != nil {
		t.Fatalf("read %s: %v", noiseFillerPath, err)
	}
	var bands [][]string
	var band []string
	for _, line := range strings.Split(strings.ReplaceAll(string(payload), "\r\n", "\n"), "\n") {
		if line == "" {
			if len(band) > 0 {
				bands = append(bands, band)
				band = nil
			}
			continue
		}
		band = append(band, line)
	}
	if len(band) > 0 {
		bands = append(bands, band)
	}
	if len(bands) == 0 {
		t.Fatalf("%s carries no bands", noiseFillerPath)
	}
	return bands
}

// TestTheFillerListIsTheChainItClaims asserts that the file on disk is the
// thing dinah-268 AC-20 describes, rather than a list somebody edited later
// into something the corpus below would quietly accept.
//
// Each property here is one clause of the criterion. The words are ordinary
// lowercase ASCII. Each band is at least forty words, which is the size the
// criterion sets for every band the templates draw from. Every word in a band
// sits within one rune of every other, which is what lets any twenty of them
// share a template. And consecutive words are near-miss partners, an alignment
// distance of two or fewer, which is the property the whole measurement rests
// on: a corpus of same-length words that share no letters would report a clean
// number without ever exercising the regime the flooding came from.
func TestTheFillerListIsTheChainItClaims(t *testing.T) {
	seen := map[string]bool{}
	for number, band := range noiseFillerBands(t) {
		if len(band) < 40 {
			t.Errorf("band %d carries %d words, wanted at least 40", number, len(band))
		}
		shortest, longest := len([]rune(band[0])), len([]rune(band[0]))
		for _, word := range band {
			if seen[word] {
				t.Errorf("%q appears twice in %s", word, noiseFillerPath)
			}
			seen[word] = true
			for _, letter := range word {
				if letter < 'a' || letter > 'z' {
					t.Errorf("%q is not an ordinary lowercase-ASCII word", word)
					break
				}
			}
			if length := len([]rune(word)); length < shortest {
				shortest = length
			} else if length > longest {
				longest = length
			}
		}
		if longest-shortest > 1 {
			t.Errorf("band %d runs from %d runes to %d, so two of its words cannot share a template",
				number, shortest, longest)
		}
		for at := 1; at < len(band); at++ {
			distance := alignmentDistance([]rune(band[at-1]), []rune(band[at]))
			if distance > 2 {
				t.Errorf("band %d has %q and %q consecutive at a distance of %d, wanted 2 or fewer",
					number, band[at-1], band[at], distance)
			}
		}
	}
}

// noiseTemplates are the ten long card titles dinah-268 AC-20's templated
// corpus is built from. Each carries one %s where its filler goes, and each is
// long enough that a filled title clears the ninety runes the criterion sets.
// Sibling titles from one template differ in that one word and nowhere else,
// which is the shape a real board full of near-identical titles takes and the
// shape the first version's budget flooded on.
var noiseTemplates = []string{
	"the workbench refuses a card whose column names no ready zone and the operator is left %s at the station",
	"a column that declares no instructions of its own leaves the agent %s over the card it has just pulled",
	"the journal records every refusal in the order it happened so that nobody is left %s about what a verb did",
	"an attachment past the cap is read up to the cap and no further, which stops the scan %s at the boundary",
	"the contract names the states a card may stand in and the profile keeps the tree from %s past its anchor",
	"a workstream gathers the cards a reader wants beside each other without %s the columns they stand in",
	"the guide explains the pull and the claim in one place so that a first reader is not %s between the two",
	"a reference minted for a card outlives every rename it meets, so a link written today is not %s next year",
	"the catalog falls back to english message by message, which leaves a half translated build %s not broken",
	"the discovery walk climbs to the drive root and stops there instead of %s into a directory nobody named",
}

// noiseTemplatedCorpus builds the templated corpus: one title per template and
// filler, drawn in file order.
//
// The draw is the part worth reading. Each template takes twenty consecutive
// words from one band, starting at an offset the template's own index fixes,
// so the corpus is the same on every machine and nobody chose which words went
// into it. Consecutive in file order is consecutive in the chain, which is
// what carries the near-miss property from the file into the corpus: drawing
// the same words in any other order, alphabetical order among them, would
// scatter the chain and leave a template of words that merely share a length.
func noiseTemplatedCorpus(t *testing.T) (titles []string, fillers []string) {
	t.Helper()
	bands := noiseFillerBands(t)
	for number, template := range noiseTemplates {
		band := bands[number%len(bands)]
		start := (number / len(bands)) * 4
		for at := 0; at < fillersPerTemplate; at++ {
			filler := band[(start+at)%len(band)]
			titles = append(titles, fmt.Sprintf(template, filler))
			fillers = append(fillers, filler)
		}
	}
	return titles, fillers
}

// fillersPerTemplate is how many sibling titles each template contributes, and
// twenty is the number AC-20 sets.
const fillersPerTemplate = 20

// TestTheNoiseAgainstATemplatedCorpus asserts dinah-268 AC-20's long-phrase
// half, which is the regime the operator's 2026-09-03 ruling was about: two
// hundred titles of ninety-odd runes, ten templates of twenty siblings each,
// where a sibling differs from its neighbours in one word and nowhere else.
//
// The sweep is one mistyped phrase per title, each built by exchanging an
// adjacent pair of letters inside that title's own filler, and every phrase
// goes through the library's own Search rather than through the matcher
// underneath it. Three gates read what comes back, and the criterion needs all
// three, because any two of them can be satisfied without the matcher being
// any better.
//
// Completeness is the one that cannot be satisfied by answering with less. For
// each phrase the case works out for itself which titles lie within the
// shipped budget, calling alignmentDistance and typoBudget directly rather
// than asking Search what it found, and every title in that set must appear
// among the Hits the search answered with. A Search that caps or thins its
// Hits array drops titles this set still names, and the corpus is measured to
// carry a phrase with ten titles inside the budget's reach, so no fixed cap
// below that survives here. The set cannot be satisfied by answering with more
// either, since it is computed in the test and nothing the implementation does
// can move it.
//
// Ranking is second. The title a phrase was a mistyping of must be present,
// and its score must be strictly greater than every other hit's, so nothing
// ties it for first place. The construction guarantees that only for the
// chain-adjacent sibling each phrase is deliberately given, whose distance
// from the phrase is greater than the intended title's distance of one; a
// coincidental collision from some other template is not ruled out and fails
// here if it happens.
//
// Reduction is third. The same sweep is counted again under the budget this
// card replaced, max(1, n/4), written out in the test rather than reached
// through the shipped code, and the shipped budget's own unrelated-title total
// must be no more than a third of it. Both counts are taken in this run, so a
// change to the corpus or the filler list moves them together and the ratio is
// recomputed rather than compared against a figure written down once.
//
// What this costs, stated before it runs: 200 phrases against 200 titles is
// 40,000 alignment distances measured directly, and 200 searches through the
// library over a workbench of 200 cards.
//
//	go test ./internal/verb/ -run TestTheNoiseAgainstATemplatedCorpus -v
func TestTheNoiseAgainstATemplatedCorpus(t *testing.T) {
	titles, fillers := noiseTemplatedCorpus(t)
	if len(titles) < 200 {
		t.Fatalf("the corpus carries %d titles, wanted at least 200", len(titles))
	}
	// The titles have to be distinct, because every gate below attributes a
	// hit to a title by the string itself. Two equal titles would make one
	// phrase's own title indistinguishable from an unrelated one.
	distinct := map[string]int{}
	for at, title := range titles {
		if length := len([]rune(title)); length < 90 {
			t.Fatalf("title %d is %d runes, wanted at least 90: %q", at, length, title)
		}
		if first, ok := distinct[title]; ok {
			t.Fatalf("titles %d and %d are the same string, so no hit can be attributed to either: %q", first, at, title)
		}
		distinct[title] = at
	}
	// Every filler must have a near-miss partner on its own template, which is
	// the property that makes this corpus adversarial rather than comfortable.
	// It is asserted here against the corpus as drawn, not only against the
	// file, because the draw is what decides which words meet on a template.
	for start := 0; start < len(fillers); start += fillersPerTemplate {
		for at := start; at < start+fillersPerTemplate; at++ {
			nearest := len(fillers[at]) + 1
			for other := start; other < start+fillersPerTemplate; other++ {
				if other == at || fillers[other] == fillers[at] {
					continue
				}
				if distance := alignmentDistance([]rune(fillers[at]), []rune(fillers[other])); distance < nearest {
					nearest = distance
				}
			}
			if nearest > 2 {
				t.Errorf("the filler %q sits %d edits from its nearest sibling on its own template, wanted 2 or fewer",
					fillers[at], nearest)
			}
		}
	}

	phrases := make([]string, len(titles))
	for at := range titles {
		mistyped := mistype(fillers[at])
		if mistyped == "" || mistyped == fillers[at] {
			t.Fatalf("the filler %q yielded no mistyping, so title %d measures nothing", fillers[at], at)
		}
		phrases[at] = strings.Replace(titles[at], fillers[at], mistyped, 1)
		if phrases[at] == titles[at] {
			t.Fatalf("the phrase for title %d is the title itself", at)
		}
	}

	// One pass over the 40,000 pairs answers two questions at once: which
	// titles each phrase genuinely reaches under the shipped budget, which is
	// the floor completeness holds the Hits array to, and how many unrelated
	// titles the retired budget would have answered, which is the reference
	// reduction measures against. Neither number comes from Search.
	folded := make([][]rune, len(titles))
	for at, title := range titles {
		folded[at] = []rune(asciiFold(title))
	}
	inRange := make([]map[string]bool, len(phrases))
	oldNoise, largest, largestAt := 0, 0, 0
	for at, phrase := range phrases {
		wanted := []rune(asciiFold(phrase))
		length := len(wanted)
		if length <= fuzzyFloor {
			t.Fatalf("phrase %d is %d runes, at or below layer 2's floor of %d, so it measures nothing", at, length, fuzzyFloor)
		}
		shipped, first := typoBudget(length), firstVersionTypoBudget(length)
		reach := map[string]bool{}
		for other := range titles {
			distance := alignmentDistance(wanted, folded[other])
			if distance <= shipped {
				reach[titles[other]] = true
			}
			if other != at && distance <= first {
				oldNoise++
			}
		}
		inRange[at] = reach
		if len(reach) > largest {
			largest, largestAt = len(reach), at
		}
	}

	h := newHarness(t)
	for _, title := range titles {
		h.add(title)
	}

	missing, unranked, tied, shippedNoise := 0, 0, 0, 0
	for at, phrase := range phrases {
		results := found(h, &Request{SearchText: phrase})
		answered := make(map[string]bool, len(results.Hits))
		intended, present := 0.0, false
		for _, hit := range results.Hits {
			if hit.Kind != SearchKindCard || hit.MatchedIn != MatchedInTitle {
				continue
			}
			answered[hit.Title] = true
			if hit.Title == titles[at] {
				intended, present = hit.Score, true
			}
		}
		if !present {
			unranked++
			if unranked <= 3 {
				t.Errorf("phrase %d answered no title hit for the title it was a mistyping of: %q", at, titles[at])
			}
		} else {
			for _, hit := range results.Hits {
				if hit.Kind == SearchKindCard && hit.MatchedIn == MatchedInTitle && hit.Title == titles[at] {
					continue
				}
				if hit.Score >= intended {
					tied++
					if tied <= 3 {
						t.Errorf("phrase %d ranks the hit %q (%s in %s, score %g) at or above its own title (score %g)",
							at, hit.Title, hit.Kind, hit.MatchedIn, hit.Score, intended)
					}
				}
			}
		}
		// Completeness. The independently computed set is the floor, so a
		// title inside the budget and absent from the Hits array is a thinned
		// view of what the matcher found, whatever the other two gates report.
		for title := range inRange[at] {
			if !answered[title] {
				missing++
				if missing <= 3 {
					t.Errorf("phrase %d reaches %q within the shipped budget, but the search did not answer it, so its Hits array is a capped or filtered view of what the matcher found",
						at, title)
				}
			}
		}
		for title := range answered {
			if title != titles[at] {
				shippedNoise++
			}
		}
	}

	t.Logf("dinah-268 AC-20: %d templated titles of %d runes swept with %d mistyped phrases through the library's own Search, at a shipped budget of %d edits. Completeness: the widest independently computed within-budget set holds %d titles (phrase %d), and %d titles across the sweep were inside a budget and absent from the Hits. Ranking: %d phrases lost their own title and %d hits tied with or beat one. Reduction: %d unrelated title hits under the shipped budget against %d under the retired max(1, n/4), a ratio of %.4f against a ceiling of one third.",
		len(titles), len([]rune(titles[0])), len(phrases), typoBudget(len([]rune(phrases[0]))),
		largest, largestAt, missing, unranked, tied, shippedNoise, oldNoise,
		float64(shippedNoise)/float64(oldNoise))

	// The corpus has to stay adversarial enough for completeness to bite. A
	// fixture whose widest within-budget set fell to a handful would let a
	// Search capping its Hits array at ten pass this case, which is exactly
	// the pass the criterion exists to stop.
	if largest < 10 {
		t.Errorf("the widest within-budget set holds only %d titles, so a Search capping its Hits array at ten would still pass completeness; the corpus needs a phrase reaching at least 10",
			largest)
	}
	if missing > 0 {
		t.Errorf("%d titles across the sweep lie inside the shipped budget and are absent from the Hits the search answered", missing)
	}
	if unranked > 0 {
		t.Errorf("%d of %d phrases no longer find the title they were a mistyping of, so the transposition-catching behaviour is lost",
			unranked, len(phrases))
	}
	if tied > 0 {
		t.Errorf("%d hits across the sweep tie with or outrank the phrase's own title, so the intended card does not stand alone at the top", tied)
	}
	if oldNoise == 0 {
		t.Fatalf("the retired budget answered no unrelated title at all, so the reduction gate has no reference left to measure against")
	}
	if 3*shippedNoise > oldNoise {
		t.Errorf("the shipped budget answered %d unrelated titles against the retired budget's %d over the same corpus, wanted no more than a third of it (%d)",
			shippedNoise, oldNoise, oldNoise/3)
	}
}
