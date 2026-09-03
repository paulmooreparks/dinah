package verb

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The four kinds of entity a hit names.
const (
	SearchKindCard       = "card"
	SearchKindWorkbench  = "workbench"
	SearchKindColumn     = "column"
	SearchKindWorkstream = "workstream"
)

// The five fields a hit reports the phrase as having matched in.
const (
	MatchedInReference  = "reference"
	MatchedInTitle      = "title"
	MatchedInFraming    = "framing"
	MatchedInComment    = "comment"
	MatchedInAttachment = "attachment"
)

// The five tiers the ranking orders hits by, tier 0 ranking highest. The
// score a caller receives is (searchTiers - tier) + quality, and quality is
// bounded to (0, 1], so every tier-n hit outranks every tier-(n+1) hit
// whatever their qualities are and quality only breaks ties inside one tier.
const (
	tierReference  = 0
	tierTitle      = 1
	tierFraming    = 2
	tierComment    = 3
	tierAttachment = 4
	// searchTiers is the constant the score subtracts a tier from, which is
	// the last tier's own index. A tier-4 hit therefore scores in (0, 1] and
	// nothing a search answers ever scores at or below zero.
	searchTiers = tierAttachment
)

// fuzzyFloor is the phrase length at or below which layer 2 does not run at
// all, counted in runes after case-folding. The budget below already floors at
// one edit, and one edit of tolerance against a two- or three-rune phrase
// matches nearly anything typed near it, which is noise this search has no
// reason to pay for.
const fuzzyFloor = 3

// attachmentCap is how much of an attachment's payload a search reads and
// matches against. The scan is bounded rather than partial and silent: a hit
// inside the prefix is reported like any other, and nothing is reported for a
// match that would only exist past it.
const attachmentCap = 65536

// attachmentSniff is how many bytes of an extensionless attachment are read to
// decide whether it carries text. A NUL byte among them says it does not.
// The heuristic is this tool's own deterministic rule, spelled out here rather
// than delegated to any library's content-type detection, so nothing about
// what a search reads rests on behaviour somebody else is free to change.
const attachmentSniff = 512

// snippetCap is the widest excerpt a hit carries, in bytes.
const snippetCap = 200

// textExtensions are the filename suffixes that qualify an attachment for the
// scan outright, compared case-insensitively.
var textExtensions = []string{".md", ".markdown", ".txt"}

// SearchResults is what dinah search answers: every hit the phrase found,
// ranked, in one workbench.
type SearchResults struct {
	// Text is the search phrase, echoed byte for byte, the way Matches echoes
	// Query.
	Text string `json:"text"`
	// Filter is the --query narrowing string, when one was given.
	Filter string `json:"filter,omitempty"`
	// Archived is whether this run's scan included the archived half.
	Archived bool `json:"archived"`
	// Hits are the matches, ranked highest score first.
	Hits []SearchHit `json:"hits"`
	// Count is len(Hits), so a caller need not measure the array.
	Count int `json:"count"`
}

// SearchHit is one matched entity: enough of its identity to act on, plus
// where and how well the phrase matched.
type SearchHit struct {
	// Kind is "card", "workbench", "column" or "workstream".
	Kind string `json:"kind"`
	// ID is the entity's identifier: a card's, a column's or a workstream's
	// 12-hex identifier, or the workbench's own.
	ID string `json:"id"`
	// Ref is the human reference, when the kind has one.
	Ref string `json:"ref,omitempty"`
	// Title is the entity's title.
	Title string `json:"title,omitempty"`
	// Column and ColumnTitle are set on a card hit only.
	Column      string `json:"column,omitempty"`
	ColumnTitle string `json:"column_title,omitempty"`
	// MatchedIn names the field the phrase matched: "reference", "title",
	// "framing", "comment" or "attachment".
	MatchedIn string `json:"matched_in"`
	// Snippet is a short excerpt of the matched field around the match, so a
	// caller sees why this hit surfaced without a second read.
	Snippet string `json:"snippet"`
	// Score orders Hits. Higher ranks first, and the score is bounded so that
	// where the phrase matched always outranks how well it matched.
	Score float64 `json:"score"`
	// Archived is true on a card hit drawn from the archived half.
	Archived bool `json:"archived,omitempty"`
	// tier and arrival order hits that score alike, and are unexported
	// because neither is part of what a caller reads: tier is already
	// recoverable from the score, and arrival is the position the entity was
	// walked in.
	tier    int
	arrival int
}

// Search reports every place a phrase occurs in one workbench, ranked.
//
// The scan is linear over the workbench's own entities and reads no index,
// which is the operator's own ruling on this card: at a few hundred cards an
// index is a cache that has to agree with a scan, and the scan is what it
// would have to agree with. A workbench for which this proves too slow is a
// measurement rather than a hypothesis, and the card that adds an index is
// the one that carries it.
func (l *Library) Search(req *Request) (*SearchResults, error) {
	phrase := req.SearchText
	if strings.TrimSpace(phrase) == "" {
		return nil, contract.Refuse(contract.EmptySearch, "")
	}
	results := &SearchResults{
		Text:     phrase,
		Filter:   req.Query,
		Archived: req.Archived,
		Hits:     []SearchHit{},
	}
	// The filter narrows the cards the phrase is run over and has no bearing
	// on the workbench, its columns or its workstreams, since not one of the
	// twelve field names means anything for those three kinds. A card outside
	// the filter's own set contributes no hit at any tier.
	var narrowed map[string]bool
	if strings.TrimSpace(req.Query) != "" {
		matched, _, err := l.selection(req.Query, req.Actor)
		if err != nil {
			return nil, err
		}
		narrowed = make(map[string]bool, len(matched))
		for _, card := range matched {
			narrowed[card.ID] = true
		}
	}
	live, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	for _, card := range live {
		l.searchCard(results, card, phrase, narrowed, false)
	}
	if req.Archived {
		root := l.Bench.ArchivedCardsRoot()
		for _, id := range bench.ListIDs(root) {
			card, err := bench.LoadCard(root, id)
			if err != nil {
				continue
			}
			l.searchCard(results, card, phrase, narrowed, true)
		}
	}
	l.searchBench(results, phrase)
	rankHits(results.Hits)
	results.Count = len(results.Hits)
	return results, nil
}

// searchCard adds every hit one card carries, in tier order: its reference,
// its title, its framing prose, each of its comments, and each of its
// qualifying attachments.
func (l *Library) searchCard(results *SearchResults, card *bench.Card, phrase string, narrowed map[string]bool, archived bool) {
	if narrowed != nil && !narrowed[card.ID] {
		return
	}
	ref := card.Ref(l.Bench.Slug)
	hit := SearchHit{
		Kind:     SearchKindCard,
		ID:       card.ID,
		Ref:      ref,
		Title:    card.Title,
		Column:   card.Column,
		Archived: archived,
	}
	if column := l.Bench.Column(card.Column); column != nil {
		hit.ColumnTitle = column.Title
	}
	if asciiFold(phrase) == asciiFold(ref) {
		results.add(hit, tierReference, MatchedInReference, ref, 0, len(ref))
	}
	if at, length, ok := substringIn(phrase, card.Title); ok {
		results.add(hit, tierTitle, MatchedInTitle, card.Title, at, length)
	} else if quality, ok := withinTypoBudget(phrase, card.Title); ok {
		results.addScored(hit, tierTitle, MatchedInTitle, snippetOf(card.Title, 0, len(card.Title)), quality)
	}
	if at, length, ok := substringIn(phrase, card.Body); ok {
		results.add(hit, tierFraming, MatchedInFraming, card.Body, at, length)
	}
	comments, err := bench.Comments(card.Dir)
	if err == nil {
		for _, comment := range comments {
			if at, length, ok := substringIn(phrase, comment.Body); ok {
				results.add(hit, tierComment, MatchedInComment, comment.Body, at, length)
			}
		}
	}
	attachments, err := bench.Attachments(card.Dir)
	if err == nil {
		for _, attachment := range attachments {
			text, readable := attachmentText(attachment)
			if !readable {
				continue
			}
			if at, length, ok := substringIn(phrase, text); ok {
				results.add(hit, tierAttachment, MatchedInAttachment, text, at, length)
			}
		}
	}
}

// searchBench adds the hits the workbench itself, its columns and its
// workstreams carry. All three are framing prose, so all three land at the one
// tier framing sits at, and no filter narrows any of them.
func (l *Library) searchBench(results *SearchResults, phrase string) {
	for _, column := range l.Bench.Columns {
		if at, length, ok := substringIn(phrase, column.Instructions); ok {
			results.add(SearchHit{
				Kind:  SearchKindColumn,
				ID:    column.ID,
				Ref:   column.Ref(),
				Title: column.Title,
			}, tierFraming, MatchedInFraming, column.Instructions, at, length)
		}
	}
	for _, workstream := range l.Bench.Workstreams() {
		if at, length, ok := substringIn(phrase, workstream.Notes); ok {
			results.add(SearchHit{
				Kind:  SearchKindWorkstream,
				ID:    workstream.ID,
				Ref:   workstream.Ref(),
				Title: workstream.Title,
			}, tierFraming, MatchedInFraming, workstream.Notes, at, length)
		}
	}
	// The workbench is added last within its own tier, since there is only
	// ever one of it and nothing it could be ordered against.
	if at, length, ok := substringIn(phrase, l.Bench.Standing); ok {
		results.add(SearchHit{
			Kind:  SearchKindWorkbench,
			ID:    l.Bench.ID,
			Ref:   l.Bench.Slug,
			Title: l.Bench.Title,
		}, tierFraming, MatchedInFraming, l.Bench.Standing, at, length)
	}
}

// add files one substring hit, computing its quality as the share of the field
// the phrase covers.
func (r *SearchResults) add(hit SearchHit, tier int, matchedIn, field string, at, length int) {
	r.addScored(hit, tier, matchedIn, snippetOf(field, at, length), coverage(length, len(field)))
}

// addScored files one hit whose quality the caller has already worked out,
// which is the layer-2 path.
func (r *SearchResults) addScored(hit SearchHit, tier int, matchedIn, snippet string, quality float64) {
	hit.MatchedIn = matchedIn
	hit.Snippet = snippet
	hit.Score = float64(searchTiers-tier) + quality
	hit.tier = tier
	hit.arrival = len(r.Hits)
	r.Hits = append(r.Hits, hit)
}

// coverage is the share of a field one match covers, bounded to (0, 1]. A
// match as long as the whole field scores 1.
func coverage(length, field int) float64 {
	if field <= 0 || length <= 0 {
		return 1
	}
	if length >= field {
		return 1
	}
	return float64(length) / float64(field)
}

// rankHits orders hits by score, highest first, and breaks a tie by the order
// the entities were walked in, which is ascending identifier for cards and
// declaration order for columns.
func rankHits(hits []SearchHit) {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].arrival < hits[j].arrival
	})
}

// substringIn reports where a phrase occurs in a field, compared without
// regard to case, and how many bytes of the field the match covers. The
// length is the phrase's own, since a case-insensitive comparison over ASCII
// letters changes no byte count.
func substringIn(phrase, field string) (at, length int, ok bool) {
	if phrase == "" || field == "" {
		return 0, 0, false
	}
	at = strings.Index(asciiFold(field), asciiFold(phrase))
	if at < 0 {
		return 0, 0, false
	}
	return at, len(phrase), true
}

// withinTypoBudget is layer 2: a title the phrase was very nearly typed as,
// found by edit distance rather than by containment, which is what catches a
// caller who swapped two letters or dropped one. It runs only where layer 1
// found no substring in that same title, and only against a card's title,
// because measuring a short phrase against a paragraph of prose produces
// matches nobody meant.
//
// The distance is restricted Damerau-Levenshtein, also called optimal string
// alignment, so one swap of adjacent letters costs one edit rather than the
// two a plain Levenshtein distance would charge it. Phrase and title are
// compared whole, case-folded, and a title qualifies when the distance sits
// inside typoBudget of the phrase's own length.
//
// The quality it answers with is 1 - distance/n, on the same coverage-ratio
// scale layer 1's own quality uses. Since a qualifying distance is at least 1
// and at most n/4, that quality lands in [0.75, 1): high enough to sit beside
// a substring hit on the same title tier, and bounded below 1 so an exact hit
// always wins. A distance of 0 cannot arrive here, because two equal strings
// are a substring match and layer 1 already answered them.
func withinTypoBudget(phrase, title string) (float64, bool) {
	wanted := []rune(asciiFold(phrase))
	haystack := []rune(asciiFold(title))
	length := len(wanted)
	if length <= fuzzyFloor || len(haystack) == 0 {
		return 0, false
	}
	distance := alignmentDistance(wanted, haystack)
	if distance > typoBudget(length) {
		return 0, false
	}
	return 1 - float64(distance)/float64(length), true
}

// typoBudget is how many edits layer 2 forgives in a phrase of n runes. It
// scales with the phrase rather than with the title, because it is the phrase
// a caller might have mistyped, and it floors at one so even a short phrase
// catches a single slip.
//
// Nothing else has to reject a title of wildly different length from the
// phrase. An alignment distance is never smaller than the difference between
// the two lengths, so a title much longer or shorter than the phrase fails
// this budget on that ground alone.
func typoBudget(length int) int {
	if scaled := length / 4; scaled > 1 {
		return scaled
	}
	return 1
}

// alignmentDistance is the restricted Damerau-Levenshtein distance between two
// rune slices: an insertion, a deletion, a substitution and a transposition of
// two adjacent runes each cost one edit, under the restriction that no pair of
// runes is edited twice. That restriction is what makes it cheap, and it costs
// this search nothing, since the case it exists to price at one edit is a
// caller swapping one pair of letters.
//
// Three rows are carried rather than the whole table, because a transposition
// reads the row before the previous one and nothing reads further back.
func alignmentDistance(from, to []rune) int {
	if len(from) == 0 {
		return len(to)
	}
	if len(to) == 0 {
		return len(from)
	}
	before := make([]int, len(to)+1)
	previous := make([]int, len(to)+1)
	current := make([]int, len(to)+1)
	for at := range previous {
		previous[at] = at
	}
	for row := 1; row <= len(from); row++ {
		current[0] = row
		for column := 1; column <= len(to); column++ {
			substitution := 1
			if from[row-1] == to[column-1] {
				substitution = 0
			}
			least := previous[column] + 1
			if insertion := current[column-1] + 1; insertion < least {
				least = insertion
			}
			if replaced := previous[column-1] + substitution; replaced < least {
				least = replaced
			}
			if row > 1 && column > 1 && from[row-1] == to[column-2] && from[row-2] == to[column-1] {
				if swapped := before[column-2] + 1; swapped < least {
					least = swapped
				}
			}
			current[column] = least
		}
		before, previous, current = previous, current, before
	}
	return previous[len(to)]
}

// asciiFold lowercases the ASCII letters of a string and leaves every other
// byte alone, which is the comparison every match here is made under.
func asciiFold(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, text)
}

// snippetOf is the excerpt a hit carries: the matched text with as much of
// what surrounds it as the cap allows, centred on the match where the field is
// wider than the cap. An elision is marked with an ellipsis, and a field that
// fits under the cap whole is carried whole, so a reader can tell an excerpt
// from an entire field by reading it.
func snippetOf(field string, at, length int) string {
	if len(field) <= snippetCap {
		return flatten(field)
	}
	start := at
	if length < snippetCap {
		start = at - (snippetCap-length)/2
	}
	if start+snippetCap > len(field) {
		start = len(field) - snippetCap
	}
	if start < 0 {
		start = 0
	}
	end := start + snippetCap
	if end > len(field) {
		end = len(field)
	}
	start = runeBoundaryAt(field, start, 1)
	end = runeBoundaryAt(field, end, -1)
	excerpt := flatten(field[start:end])
	if start > 0 {
		excerpt = "..." + excerpt
	}
	if end < len(field) {
		excerpt += "..."
	}
	return excerpt
}

// flatten folds every run of whitespace in a field into one space, so a
// snippet drawn from a paragraph stays on the one line a table row gives it.
func flatten(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// runeBoundaryAt walks an index in the given direction until it sits on a rune
// boundary, so an excerpt never cuts a character in half.
func runeBoundaryAt(text string, at, step int) int {
	for at > 0 && at < len(text) && !utf8.RuneStart(text[at]) {
		at += step
	}
	if at < 0 {
		return 0
	}
	if at > len(text) {
		return len(text)
	}
	return at
}

// attachmentText reads as much of an attachment's payload as the cap allows,
// and reports whether the attachment qualifies for the scan at all.
//
// It qualifies on its filename where the filename says so, and otherwise only
// where it carries no extension and the first bytes of its payload hold no NUL
// byte. An attachment named for a format this tool does not read as text is
// not sniffed: bytes that happen to spell a phrase inside a picture are not an
// occurrence of it.
func attachmentText(attachment *bench.Attachment) (string, bool) {
	if attachment.Path == "" {
		return "", false
	}
	extension := strings.ToLower(filepath.Ext(attachment.Filename))
	named := false
	for _, allowed := range textExtensions {
		if extension == allowed {
			named = true
			break
		}
	}
	if !named && extension != "" {
		return "", false
	}
	file, err := os.Open(attachment.Path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	payload := make([]byte, attachmentCap)
	read, err := io.ReadFull(file, payload)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", false
	}
	payload = payload[:read]
	if !named && !looksLikeText(payload) {
		return "", false
	}
	return string(payload), true
}

// looksLikeText reports whether the sniffed head of a payload holds no NUL
// byte, which is this tool's own rule for an attachment carrying no extension.
func looksLikeText(payload []byte) bool {
	head := payload
	if len(head) > attachmentSniff {
		head = head[:attachmentSniff]
	}
	for _, b := range head {
		if b == 0 {
			return false
		}
	}
	return true
}
