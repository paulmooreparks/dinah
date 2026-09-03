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
	} else if quality, ok := subsequenceIn(phrase, card.Title); ok {
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

// subsequenceIn is layer 2: every rune of the phrase occurs in the field, in
// order, compared without regard to case, which finds a title typed with a
// letter left out. It runs only where layer 1 found no substring in that same
// field, and only against a card's title, because scoring a scattered
// subsequence against a paragraph of prose produces matches nobody meant.
//
// The quality it answers with is bounded to (0, 1] by three factors, each in
// that range and each 1 exactly when the match is perfect on its own terms:
// how densely the matched runes sit together, how near the start of the field
// the match begins, and how much of the field it covers. A phrase that is the
// whole of a short title scores 1, and a scattered late subsequence in a long
// one scores near 0.
func subsequenceIn(phrase, field string) (float64, bool) {
	if phrase == "" || field == "" {
		return 0, false
	}
	wanted := []rune(asciiFold(phrase))
	haystack := []rune(asciiFold(field))
	first, last, matched := -1, -1, 0
	for at, r := range haystack {
		if matched == len(wanted) {
			break
		}
		if r != wanted[matched] {
			continue
		}
		if first < 0 {
			first = at
		}
		last = at
		matched++
	}
	if matched < len(wanted) {
		return 0, false
	}
	span := last - first + 1
	density := float64(matched) / float64(span)
	earliness := 1 - float64(first)/float64(len(haystack))
	covered := float64(matched) / float64(len(haystack))
	quality := density * earliness * covered
	if quality <= 0 {
		return 0, false
	}
	return quality, true
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
