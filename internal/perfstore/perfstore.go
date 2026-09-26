// Package perfstore writes a workbench of a stated size from a seed, so that a
// test can measure Dinah's reads against a store shaped like a real one rather
// than against the handful of cards every other fixture carries.
//
// The same seed and the same Shape give the same paths and the same bytes on
// every platform, because every choice the generator makes is a SHA-256
// derivation over the seed and nothing reads the clock or a random source. The
// package is imported by tests alone; TestPerfstoreIsNotInTheBinary holds that.
package perfstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// DefaultSeed is the seed CI generates with. A budget failure names it, so a
// local run with the same seed and shape reproduces the same bytes.
const DefaultSeed uint64 = 20260926

// Shape states what a generated workbench holds. Every count is exact, not a
// mean: Generate writes precisely this many of each, or refuses. It refuses a
// shape whose ItemComments is below Items minus PendingItems, because every
// settled item points at a comment of its own.
type Shape struct {
	Cards            int // live cards, numbered perf-1 .. perf-Cards
	ArchivedCards    int // cards under archive/cards, numbered after the live ones
	Workstreams      int
	Items            int // checklist items across live cards
	PendingItems     int // of Items, how many stay pending; the rest are settled
	ItemComments     int // comments below checklist items
	CardComments     int // comments on cards themselves
	Attachments      int // attachments on live cards, one payload file each
	JournalLines     int // total lines across live cards' journals
	LinkedCards      int // live cards carrying at least one link
	CardBodyBytes    int // mean card body length
	ItemBodyBytes    int // mean item body length
	CommentBodyBytes int // mean comment body length
	PayloadBytes     int // mean attachment payload length
	ColumnBodyBytes  int // mean column instructions length
}

// DevelopmentShape mirrors the Dinah development workbench as it stood on
// 2026-09-26: the store dinah-618's measurements were taken on. Each count
// read from that workbench is rounded up so the shape meets dinah-621's floor
// of 350 cards, 3,000 items and 4,700 comments.
func DevelopmentShape() Shape {
	return Shape{
		Cards:            360,
		ArchivedCards:    9,
		Workstreams:      24,
		Items:            3000,
		PendingItems:     90,
		ItemComments:     3500,
		CardComments:     1200,
		Attachments:      95,
		JournalLines:     16000,
		LinkedCards:      136,
		CardBodyBytes:    6000,
		ItemBodyBytes:    500,
		CommentBodyBytes: 1600,
		PayloadBytes:     45000,
		ColumnBodyBytes:  5000,
	}
}

// SmallShape is a shape small enough for the ordinary suite: every column
// occupied, every entity kind present, generated in well under a second.
//
// JournalLines is 1,000 rather than the 600 dinah-621's specification first
// named. Every column holding a card costs 91 moves before any card has a
// comment, and the 200 items, 300 comments and 6 attachments each carry a
// journal line of their own, so 600 is below what the shape's own entities
// need; this shape needs 886, and 1,000 keeps about the development
// workbench's ratio of journal lines to items.
func SmallShape() Shape {
	return Shape{
		Cards:            28,
		ArchivedCards:    2,
		Workstreams:      3,
		Items:            200,
		PendingItems:     20,
		ItemComments:     240,
		CardComments:     60,
		Attachments:      6,
		JournalLines:     1000,
		LinkedCards:      10,
		CardBodyBytes:    600,
		ItemBodyBytes:    50,
		CommentBodyBytes: 160,
		PayloadBytes:     4500,
		ColumnBodyBytes:  500,
	}
}

// Store describes a generated workbench.
type Store struct {
	Root   string // absolute path of the workbench directory, <dir>/.dinah/<id>
	Slug   string // "perf"
	Seed   uint64
	Shape  Shape
	Files  int    // regular files written, counted as they were written
	Digest string // Digest(Root) at the end of Generate
	// ProbeCard is the reference every per-card measurement reads: "perf-1".
	ProbeCard string
}

// The names the generated workbench carries. The operator is the one actor
// the workbench declares; the other two author comments and hold claims so a
// read meets more than one name.
const (
	workbenchSlug  = "perf"
	workbenchTitle = "Perf"
	operatorName   = "perf-operator"
	agentName      = "perf-agent"
	reviewerName   = "perf-reviewer"
	itemOwner      = "holder"
	blockKind      = "external_dep"
)

// epoch is the instant every generated timestamp is an offset from. Nothing in
// this package reads the clock, so this and the seed decide every stamp.
var epoch = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// columnSpec is one column of the fixed flow the generator writes, carrying
// the interchange members a column element is written with.
type columnSpec struct {
	Title         string `json:"title"`
	Kind          string `json:"kind"`
	Slug          string `json:"slug"`
	RejectTo      string `json:"reject_to,omitempty"` // the slug a rejection lands in
	OperatorOwned bool   `json:"operator_owned,omitempty"`
}

// flowDefinition is the full route of the development workbench as the
// interchange form writes its columns, with the titles, slugs, kinds,
// reject_to targets and operator_owned flags it declares. It is data in the
// shape internal/template keeps its templates in, so the generator declares
// the flow without answering any question about what a kind means; whether a
// column takes work up is asked of bench.Column.
const flowDefinition = `[
	{"title": "Intake", "kind": "intake", "slug": "intake"},
	{"title": "Triage", "kind": "work", "slug": "triage"},
	{"title": "Design Queue", "kind": "dinah.buffer", "slug": "design-queue"},
	{"title": "Spec", "kind": "work", "slug": "spec", "reject_to": "design-queue"},
	{"title": "Agent Design Review", "kind": "work", "slug": "agent-design-review", "reject_to": "spec"},
	{"title": "Operator Design Review", "kind": "work", "slug": "operator-design-review", "reject_to": "agent-design-review"},
	{"title": "Build Queue", "kind": "dinah.buffer", "slug": "build-queue"},
	{"title": "Implement", "kind": "work", "slug": "implement", "reject_to": "design-queue"},
	{"title": "Agent Code Review", "kind": "work", "slug": "agent-code-review", "reject_to": "implement"},
	{"title": "Operator Code Review", "kind": "work", "slug": "operator-code-review", "reject_to": "implement"},
	{"title": "Test", "kind": "work", "slug": "test", "reject_to": "implement"},
	{"title": "Merge", "kind": "work", "slug": "merge", "reject_to": "implement"},
	{"title": "Acceptance", "kind": "work", "slug": "acceptance", "reject_to": "implement", "operator_owned": true},
	{"title": "Done", "kind": "done", "slug": "done"}
]`

// flow is flowDefinition read once. The columns are fixed rather than a Shape
// parameter, so every generated store has the flow the development workbench
// has. The definition is a constant of this file, so a failure to read it is
// a defect in the source and every test in the package fails on it at once.
var flow = readFlow()

// readFlow parses flowDefinition.
func readFlow() []columnSpec {
	var columns []columnSpec
	if err := json.Unmarshal([]byte(flowDefinition), &columns); err != nil {
		panic(fmt.Sprintf("perfstore: the flow definition does not parse: %v", err))
	}
	return columns
}

// smallRoute names the columns of the development workbench's small route, by
// slug, in flow order.
var smallRoute = []string{
	"intake", "triage", "design-queue", "build-queue", "implement",
	"agent-code-review", "test", "merge", "acceptance", "done",
}

// The three level axes the development workbench declares, in its order.
var (
	severities = []string{"trivial", "minor", "major", "critical"}
	priorities = []string{"later", "soon", "next", "now"}
	tierNames  = []string{"minimal", "workhorse", "frontier", "apex"}
)

// linkKinds are the five link kinds the development workbench uses. None
// carries behaviour here, because the generated workbench declares no holds.
var linkKinds = []string{"blocks", "relates_to", "supersedes", "parked_behind", "spawned_from"}

// The item kinds the format declares, named once for the generator's tables.
const (
	kindCriterion = "acceptance_criterion"
	kindQuestion  = "open_question"
	kindDecision  = "decision"
)

// itemColumnSlug is the column each kind of generated item names, which is
// where the development workbench files it: a criterion names Merge, a
// decision names Implement, and a question names the operator's design
// station.
var itemColumnSlug = map[string]string{
	kindCriterion: "merge",
	kindDecision:  "implement",
	kindQuestion:  "operator-design-review",
}

// The number of live cards the generator claims and blocks. Both sit in work
// columns the operator does not own, which is where a claim can stand.
const (
	claimedCards = 3
	blockedCards = 2
)

// Generate writes a new workbench under dir and returns it. dir must exist
// and must not already hold a .dinah directory. The workbench is written at
// the StorageFormat and ProfileVersion of the binary the caller is linked
// into.
func Generate(dir string, seed uint64, shape Shape) (*Store, error) {
	if err := shape.validate(); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, fmt.Errorf("generate into %s: %w", absolute, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("generate into %s: not a directory", absolute)
	}
	container := filepath.Join(absolute, bench.UserBaseName)
	if bench.Exists(container) {
		return nil, fmt.Errorf("generate into %s: a %s directory is already there", absolute, bench.UserBaseName)
	}
	g := &generator{
		seed:     seed,
		shape:    shape,
		root:     filepath.Join(container, workbenchID(seed)),
		issued:   map[string]bool{},
		counters: map[string]int{},
	}
	if err := g.plan(); err != nil {
		return nil, err
	}
	if err := g.write(); err != nil {
		return nil, err
	}
	digest, files, err := Digest(g.root)
	if err != nil {
		return nil, err
	}
	store := &Store{
		Root:      g.root,
		Slug:      workbenchSlug,
		Seed:      seed,
		Shape:     shape,
		Files:     g.files,
		Digest:    digest,
		ProbeCard: workbenchSlug + "-1",
	}
	if files != g.files {
		return store, fmt.Errorf("generated %d files but the tree holds %d", g.files, files)
	}
	return store, nil
}

// Digest is the SHA-256, hex encoded, over every regular file under root in
// ascending order of slash-separated relative path, each contributing
// path, NUL, length as decimal, NUL, bytes. It also returns the file count.
func Digest(root string) (digest string, files int, err error) {
	var paths []string
	walk := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		return "", 0, err
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, relative := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return "", 0, err
		}
		hash.Write([]byte(relative))
		hash.Write([]byte{0})
		hash.Write([]byte(strconv.Itoa(len(data))))
		hash.Write([]byte{0})
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil)), len(paths), nil
}

// validate refuses a shape Generate cannot write exactly.
func (s Shape) validate() error {
	counts := []struct {
		name  string
		value int
	}{
		{"Cards", s.Cards}, {"ArchivedCards", s.ArchivedCards}, {"Workstreams", s.Workstreams},
		{"Items", s.Items}, {"PendingItems", s.PendingItems}, {"ItemComments", s.ItemComments},
		{"CardComments", s.CardComments}, {"Attachments", s.Attachments}, {"JournalLines", s.JournalLines},
		{"LinkedCards", s.LinkedCards}, {"CardBodyBytes", s.CardBodyBytes}, {"ItemBodyBytes", s.ItemBodyBytes},
		{"CommentBodyBytes", s.CommentBodyBytes}, {"PayloadBytes", s.PayloadBytes},
		{"ColumnBodyBytes", s.ColumnBodyBytes},
	}
	for _, count := range counts {
		if count.value < 0 {
			return fmt.Errorf("shape %s is %d, below zero", count.name, count.value)
		}
	}
	if s.Cards < len(flow) {
		return fmt.Errorf("shape Cards is %d, fewer than the %d columns every one of which holds a card", s.Cards, len(flow))
	}
	if s.PendingItems > s.Items {
		return fmt.Errorf("shape PendingItems is %d, more than the %d Items", s.PendingItems, s.Items)
	}
	settled := s.Items - s.PendingItems
	if s.ItemComments < settled {
		return fmt.Errorf("shape ItemComments is %d, fewer than the %d settled items each needing a comment of its own", s.ItemComments, settled)
	}
	if s.ItemComments > 0 && s.Items == 0 {
		return fmt.Errorf("shape ItemComments is %d with no Items to hold them", s.ItemComments)
	}
	if s.LinkedCards > s.Cards {
		return fmt.Errorf("shape LinkedCards is %d, more than the %d Cards", s.LinkedCards, s.Cards)
	}
	if s.CommentBodyBytes == 0 && s.ItemComments+s.CardComments > 0 {
		return fmt.Errorf("shape CommentBodyBytes is 0, and an empty comment is a finding dinah check reports")
	}
	return nil
}

// generator holds one generation's plan and its running counters. Nothing is
// kept at package level, so two generations into two directories can run in
// parallel.
type generator struct {
	seed   uint64
	shape  Shape
	root   string
	issued map[string]bool
	// counters holds the next index for each draw label whose index is a
	// running count rather than an entity's own position.
	counters map[string]int
	files    int

	columnIDs   []string
	workstreams []workstreamPlan
	cards       []*cardPlan
	archived    []*cardPlan
}

// workstreamPlan is one workstream the generator writes.
type workstreamPlan struct {
	id    string
	title string
	slug  string
}

// cardPlan is one card the generator writes, settled before any file is.
type cardPlan struct {
	number      int
	id          string
	column      int // position in flow
	state       string
	items       []*itemPlan
	comments    int
	attachments int
	links       []bench.Link
	workstream  string // identifier, empty for none
	fillerPairs int    // claimed-and-released pairs making up the journal's share
	leadClaim   bool   // a blocked card that was claimed before it was blocked
	archivedAt  string // the stamp an archived card's archived event carries
}

// itemPlan is one checklist item the generator writes.
type itemPlan struct {
	id       string
	kind     string
	state    string
	comments int // every comment below the item, the designated answer included
}

// draw is the one source of every choice: SHA-256 over the seed as eight
// big-endian bytes, the label, a NUL, and the index.
func (g *generator) draw(label, index string) [sha256.Size]byte {
	var seed [8]byte
	binary.BigEndian.PutUint64(seed[:], g.seed)
	message := make([]byte, 0, len(seed)+len(label)+1+len(index))
	message = append(message, seed[:]...)
	message = append(message, label...)
	message = append(message, 0)
	message = append(message, index...)
	return sha256.Sum256(message)
}

// number draws a value in [0, n) for one labelled position.
func (g *generator) number(label string, index, n int) int {
	sum := g.draw(label, strconv.Itoa(index))
	value := binary.BigEndian.Uint64(sum[:8])
	return int(value % uint64(n))
}

// next answers the next running index for a label.
func (g *generator) next(label string) int {
	index := g.counters[label]
	g.counters[label] = index + 1
	return index
}

// id derives the next 12-hex identifier under a label. A collision with one
// already issued in this generation is redrawn with the index suffixed /1,
// /2 and so on, so the answer stays a function of the seed.
func (g *generator) id(label string) string {
	index := strconv.Itoa(g.next(label))
	key := index
	for attempt := 1; ; attempt++ {
		sum := g.draw(label, key)
		id := hex.EncodeToString(sum[:6])
		if !g.issued[id] {
			g.issued[id] = true
			return id
		}
		key = index + "/" + strconv.Itoa(attempt)
	}
}

// workbenchID lays out a UUID version 7 the way bench.NewWorkbenchID
// documents, with the timestamp field fixed at the epoch and both random
// fields taken from SHA-256 over the seed and the label workbench-id.
func workbenchID(seed uint64) string {
	var seedBytes [8]byte
	binary.BigEndian.PutUint64(seedBytes[:], seed)
	sum := sha256.Sum256(append(seedBytes[:], "workbench-id"...))
	var raw [16]byte
	milliseconds := epoch.UnixMilli()
	for i := 0; i < 6; i++ {
		raw[i] = byte(milliseconds >> (40 - 8*i))
	}
	copy(raw[6:], sum[:10])
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	return hex.EncodeToString(raw[:])
}

// text draws prose of about mean bytes, between half and one and a half times
// the mean, from the word list. With paragraphs set it breaks the prose into
// paragraphs of 48 words; without, it is one line. A positive mean always
// gives at least one word.
func (g *generator) text(label string, index, mean int, paragraphs bool) string {
	if mean <= 0 {
		return ""
	}
	target := mean/2 + g.number(label, index, mean+1)
	var out strings.Builder
	written := 0
	for {
		sum := g.draw("text-word", strconv.Itoa(g.next("text-word")))
		for _, b := range sum {
			if written > 0 {
				separator := " "
				if paragraphs && written%48 == 0 {
					separator = "\n\n"
				}
				out.WriteString(separator)
			}
			out.WriteString(words[b])
			written++
			if out.Len() >= target {
				return out.String()
			}
		}
	}
}

// title draws a one-line title of three to seven words, capitalised.
func (g *generator) title(label string, index int) string {
	count := 3 + g.number(label, index, 5)
	sum := g.draw(label+"-word", strconv.Itoa(index))
	parts := make([]string, count)
	for i := range parts {
		parts[i] = words[sum[i]]
	}
	parts[0] = strings.ToUpper(parts[0][:1]) + parts[0][1:]
	return strings.Join(parts, " ")
}

// plan settles every identifier, count, placement and state before a file is
// written, so the journal line count can be checked against the shape first.
func (g *generator) plan() error {
	for range flow {
		g.columnIDs = append(g.columnIDs, g.id("column-id"))
	}
	for w := 0; w < g.shape.Workstreams; w++ {
		title := "Stream " + strconv.Itoa(w+1) + " " + g.title("workstream-title", w)
		plan := workstreamPlan{id: g.id("workstream-id"), title: title, slug: bench.SlugifyDashed(title)}
		g.workstreams = append(g.workstreams, plan)
	}
	columns := g.placeCards()
	for i := 0; i < g.shape.Cards; i++ {
		card := &cardPlan{number: i + 1, id: g.id("card-id"), column: columns[i], state: contract.StateReady}
		if len(g.workstreams) > 0 {
			card.workstream = g.workstreams[g.number("card-workstream", i, len(g.workstreams))].id
		}
		g.cards = append(g.cards, card)
	}
	for a := 0; a < g.shape.ArchivedCards; a++ {
		card := &cardPlan{
			number: g.shape.Cards + a + 1,
			id:     g.id("card-id"),
			column: len(flow) - 1,
			state:  contract.StateReady,
		}
		g.archived = append(g.archived, card)
	}
	g.planClaims()
	g.planItems()
	g.planComments()
	g.planAttachments()
	g.planLinks()
	return g.planJournals()
}

// placeCards answers the column position of every live card. Every column
// holds one card first; the rest follow the development workbench, about two
// thirds in Intake, a fifth in Done, a sixteenth in Acceptance and the
// remainder spread over the columns between. perf-1 is placed in the most
// populated column.
func (g *generator) placeCards() []int {
	last := len(flow) - 1
	counts := make([]int, len(flow))
	for position := range counts {
		counts[position] = 1
	}
	rest := g.shape.Cards - len(flow)
	intake := rest * 2 / 3
	done := rest / 5
	acceptance := rest / 16
	between := rest - intake - done - acceptance
	counts[0] += intake
	counts[last] += done
	counts[last-1] += acceptance
	for k := 0; k < between; k++ {
		counts[1+k%(last-2)]++
	}
	var slots []int
	for position, count := range counts {
		for k := 0; k < count; k++ {
			slots = append(slots, position)
		}
	}
	for i := len(slots) - 1; i > 0; i-- {
		j := g.number("card-column", i, i+1)
		slots[i], slots[j] = slots[j], slots[i]
	}
	most := 0
	for position, count := range counts {
		if count > counts[most] {
			most = position
		}
	}
	for j, position := range slots {
		if position == most {
			slots[0], slots[j] = slots[j], slots[0]
			break
		}
	}
	return slots
}

// planClaims marks the first live cards after perf-1 standing in a column that
// takes work up and that the operator does not own: three claimed, then two
// blocked.
func (g *generator) planClaims() {
	claimed, blocked := 0, 0
	for _, card := range g.cards[1:] {
		spec := flow[card.column]
		column := &bench.Column{Kind: spec.Kind}
		if !column.TakesWorkUp() || spec.OperatorOwned {
			continue
		}
		switch {
		case claimed < claimedCards:
			card.state = contract.StateActive
			claimed++
		case blocked < blockedCards:
			card.state = contract.StateBlocked
			blocked++
		}
	}
}

// spread divides total over n holders by drawn weights of one to four, the
// first holder always taking the heaviest weight, so it carries at least the
// mean. What rounding leaves over goes round the holders from the first.
func (g *generator) spread(label string, total, n int) []int {
	counts := make([]int, n)
	if n == 0 || total == 0 {
		return counts
	}
	weights := make([]int, n)
	sum := 0
	for i := range weights {
		weights[i] = 1 + g.number(label, i, 4)
		if i == 0 {
			weights[i] = 4
		}
		sum += weights[i]
	}
	assigned := 0
	for i := range counts {
		counts[i] = total * weights[i] / sum
		assigned += counts[i]
	}
	for k := 0; assigned < total; k++ {
		counts[k%n]++
		assigned++
	}
	return counts
}

// planItems deals the items over the live cards and settles each one's kind
// and state. Pending items are spread evenly through generation order.
func (g *generator) planItems() {
	perCard := g.spread("item-weight", g.shape.Items, len(g.cards))
	total, pending := g.shape.Items, g.shape.PendingItems
	index := 0
	for c, card := range g.cards {
		for k := 0; k < perCard[c]; k++ {
			item := &itemPlan{id: g.id("item-id"), kind: g.itemKind(index)}
			isPending := (index+1)*pending/total > index*pending/total
			switch {
			case isPending:
				item.state = bench.ItemPending
			case item.kind != kindCriterion:
				item.state = bench.ItemResolved
			case g.number("item-verdict", index, 10) == 0:
				item.state = bench.ItemFailed
			default:
				item.state = bench.ItemVerified
			}
			if item.state != bench.ItemPending {
				item.comments = 1
			}
			card.items = append(card.items, item)
			index++
		}
	}
}

// itemKind draws one item's kind in the development workbench's proportions:
// acceptance criteria 53%, decisions 40%, open questions 7%.
func (g *generator) itemKind(index int) string {
	roll := g.number("item-kind", index, 100)
	switch {
	case roll < 53:
		return kindCriterion
	case roll < 93:
		return kindDecision
	}
	return kindQuestion
}

// planComments deals the item comments left after every settled item has its
// answer, and the card comments.
func (g *generator) planComments() {
	var items []*itemPlan
	settled := 0
	for _, card := range g.cards {
		for _, item := range card.items {
			items = append(items, item)
			settled += item.comments
		}
	}
	extra := g.spread("item-comment-weight", g.shape.ItemComments-settled, len(items))
	for i, item := range items {
		item.comments += extra[i]
	}
	perCard := g.spread("card-comment-weight", g.shape.CardComments, len(g.cards))
	for c, card := range g.cards {
		card.comments = perCard[c]
	}
}

// planAttachments gives perf-1 one attachment and deals the rest.
func (g *generator) planAttachments() {
	if g.shape.Attachments == 0 {
		return
	}
	g.cards[0].attachments = 1
	perCard := g.spread("attachment-weight", g.shape.Attachments-1, len(g.cards))
	for c, card := range g.cards {
		card.attachments += perCard[c]
	}
}

// planLinks gives perf-1 and LinkedCards-1 other cards, evenly spaced, one to
// three links each to other live cards, perf-1 taking three.
func (g *generator) planLinks() {
	if g.shape.LinkedCards == 0 {
		return
	}
	n := len(g.cards)
	linked := []int{0}
	for k := 1; k < g.shape.LinkedCards; k++ {
		linked = append(linked, 1+(k-1)*(n-1)/(g.shape.LinkedCards-1))
	}
	for _, c := range linked {
		count := 1 + g.number("link-count", c, 3)
		if c == 0 {
			count = 3
		}
		if count > n-1 {
			count = n - 1
		}
		taken := map[int]bool{c: true}
		for len(g.cards[c].links) < count {
			target := g.number("link-target", g.next("link-target"), n)
			if taken[target] {
				continue
			}
			taken[target] = true
			kind := linkKinds[g.number("link-kind", g.next("link-kind"), len(linkKinds))]
			g.cards[c].links = append(g.cards[c].links, bench.Link{Kind: kind, To: g.cards[target].id})
		}
	}
}

// requiredLines counts the journal lines a live card's own content demands:
// its creation, a move per column it passed, its workstream join, one line per
// item filed, comment written and item settled, one per attachment and link,
// and the claim or block it ends in.
func requiredLines(card *cardPlan) int {
	lines := 1 + card.column + card.comments + card.attachments + len(card.links)
	if card.workstream != "" {
		lines++
	}
	for _, item := range card.items {
		lines += 1 + item.comments
		if item.state != bench.ItemPending {
			lines++
		}
	}
	if card.state != contract.StateReady {
		lines++
	}
	return lines
}

// planJournals makes the live journals come to JournalLines exactly. What the
// cards' own content leaves is made up of claimed-and-released pairs on the
// cards past Intake, and an odd line is a claim ahead of the first block.
func (g *generator) planJournals() error {
	required := 0
	for _, card := range g.cards {
		required += requiredLines(card)
	}
	if g.shape.JournalLines < required {
		return fmt.Errorf("shape JournalLines is %d, fewer than the %d lines the shape's cards, items, comments, attachments and links need", g.shape.JournalLines, required)
	}
	remainder := g.shape.JournalLines - required
	var moved []*cardPlan
	for _, card := range g.cards {
		if card.column > 0 {
			moved = append(moved, card)
		}
	}
	for k := 0; k < remainder/2; k++ {
		moved[k%len(moved)].fillerPairs++
	}
	if remainder%2 == 0 {
		return nil
	}
	for _, card := range g.cards {
		if card.state == contract.StateBlocked {
			card.leadClaim = true
			return nil
		}
	}
	return fmt.Errorf("no blocked card to carry the odd journal line")
}

// write lays the planned workbench down on disk.
func (g *generator) write() error {
	if err := g.writeSkeleton(); err != nil {
		return err
	}
	for w := range g.workstreams {
		if err := g.writeWorkstream(w); err != nil {
			return err
		}
	}
	for i, card := range g.cards {
		if err := g.writeCard(i, card, false); err != nil {
			return err
		}
	}
	for a, card := range g.archived {
		if err := g.writeCard(len(g.cards)+a, card, true); err != nil {
			return err
		}
	}
	var lines []string
	for _, card := range append(append([]*cardPlan{}, g.cards...), g.archived...) {
		lines = append(lines, strconv.Itoa(card.number)+" "+card.id)
	}
	if err := bench.WriteNumberLines(filepath.Join(g.root, bench.CardNumbersName), lines); err != nil {
		return err
	}
	g.files++
	return g.archive()
}

// archive carries each card planned as archived out of the live half through
// bench's own structural act, the one dinah archive runs, recording the
// archived event in the card's journal as the act's record. The card keeps its
// registry line, as an archived card does.
func (g *generator) archive() error {
	if len(g.archived) == 0 {
		return nil
	}
	b, err := bench.Open(g.root)
	if err != nil {
		return err
	}
	for _, plan := range g.archived {
		dir := filepath.Join(b.CardsRoot(), plan.id)
		journalPath := filepath.Join(dir, bench.JournalName)
		archived := bench.Event{
			TS:    plan.archivedAt,
			Event: contract.EventArchived,
			Actor: bench.NamedActor(operatorName),
			Note:  plan.id,
		}
		act := &bench.StructuralAct{
			Dir:     dir,
			LockDir: dir,
			Op:      bench.OpArchive,
			Actor:   operatorName,
			Now:     plan.archivedAt,
			Record:  func() error { return bench.AppendEvent(journalPath, archived) },
		}
		if err := b.Run(act); err != nil {
			return err
		}
	}
	return nil
}

// definitionColumn is one element of the interchange columns array, in the
// order its members are written.
type definitionColumn struct {
	ID string `json:"id"`
	columnSpec
	Instructions string `json:"instructions,omitempty"`
}

// definitionLevels is the levels member, whose axes keep the development
// workbench's order because a struct marshals in field order.
type definitionLevels struct {
	Severity []string `json:"severity"`
	Priority []string `json:"priority"`
	Tier     []string `json:"tier"`
}

// definitionModel is one model a tier lists.
type definitionModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// definitionTier is one entry of the tiers member.
type definitionTier struct {
	Meaning string            `json:"meaning"`
	Models  []definitionModel `json:"models"`
}

// definitionTiers is the tiers member, in the order the levels declare them.
type definitionTiers struct {
	Minimal   definitionTier `json:"minimal"`
	Workhorse definitionTier `json:"workhorse"`
	Frontier  definitionTier `json:"frontier"`
	Apex      definitionTier `json:"apex"`
}

// definitionRoutes is the routes member.
type definitionRoutes struct {
	Small []string `json:"small"`
}

// definition is the interchange object the skeleton is instantiated from.
type definition struct {
	Profile string             `json:"profile"`
	Title   string             `json:"title"`
	Levels  definitionLevels   `json:"levels"`
	Tiers   definitionTiers    `json:"tiers"`
	Routes  definitionRoutes   `json:"routes"`
	Columns []definitionColumn `json:"columns"`
}

// writeSkeleton writes workbench.md, the two dot files and every column
// anchor through bench.Instantiate, from a definition carrying every column's
// identifier so nothing is minted.
func (g *generator) writeSkeleton() error {
	bySlug := map[string]string{}
	for position, spec := range flow {
		bySlug[spec.Slug] = g.columnIDs[position]
	}
	var small []string
	for _, slug := range smallRoute {
		small = append(small, bySlug[slug])
	}
	object := definition{
		Profile: bench.ProfileVersion,
		Title:   workbenchTitle,
		Levels:  definitionLevels{Severity: severities, Priority: priorities, Tier: tierNames},
		Tiers:   perfTiers(),
		Routes:  definitionRoutes{Small: small},
	}
	for position, spec := range flow {
		column := definitionColumn{
			ID:           g.columnIDs[position],
			columnSpec:   spec,
			Instructions: g.text("column-body", position, g.shape.ColumnBodyBytes, true),
		}
		object.Columns = append(object.Columns, column)
	}
	data, err := json.Marshal(object)
	if err != nil {
		return err
	}
	parsed, err := bench.ReadDefinition(data)
	if err != nil {
		return err
	}
	if err := bench.Instantiate(g.root, workbenchSlug, operatorName, parsed); err != nil {
		return err
	}
	// Instantiate writes the anchor, .gitignore, .gitattributes and one anchor
	// per column. Generate compares this count against the tree, so a change
	// to what Instantiate writes is reported rather than absorbed.
	g.files += 3 + len(flow)
	return nil
}

// perfTiers is the development workbench's tier table.
func perfTiers() definitionTiers {
	anthropic := func(model string) definitionModel { return definitionModel{Provider: "anthropic", Model: model} }
	openai := func(model string) definitionModel { return definitionModel{Provider: "openai", Model: model} }
	return definitionTiers{
		Minimal: definitionTier{
			Meaning: "mechanical work: triage, moves, bookkeeping",
			Models:  []definitionModel{anthropic("claude-haiku-4-5-20251001"), openai("gpt-5.6-luna")},
		},
		Workhorse: definitionTier{
			Meaning: "the rung most agents work at: implement, review, test",
			Models:  []definitionModel{anthropic("claude-sonnet-5"), openai("gpt-5.6-terra")},
		},
		Frontier: definitionTier{
			Meaning: "design judgement, and the reviews that catch what reviews miss",
			Models:  []definitionModel{anthropic("claude-opus-5-5"), openai("gpt-5.6-sol")},
		},
		Apex: definitionTier{
			Meaning: "irreversible design calls, and the final review before one hardens",
			Models:  []definitionModel{anthropic("claude-fable-5-1"), openai("gpt-6-astra")},
		},
	}
}

// writeWorkstream writes one workstream anchor and its journal, which holds
// the one created event a new workstream's journal holds.
func (g *generator) writeWorkstream(w int) error {
	plan := g.workstreams[w]
	dir := filepath.Join(g.root, bench.WorkstreamsDir, plan.id)
	workstream := &bench.Workstream{
		ID:      plan.id,
		Dir:     dir,
		Title:   plan.title,
		Slug:    plan.slug,
		Status:  bench.StatusActive,
		Ordinal: w + 1,
		FM:      bench.NewFrontmatter(),
	}
	if err := workstream.Save(); err != nil {
		return err
	}
	created := bench.Event{
		TS:    bench.Stamp(epoch.Add(time.Duration(w+1) * time.Minute)),
		Event: contract.EventCreated,
		Actor: bench.NamedActor(operatorName),
		Title: plan.title,
	}
	journal := &journal{}
	if err := journal.add(created); err != nil {
		return err
	}
	if err := journal.save(workstream.JournalPath()); err != nil {
		return err
	}
	g.files += 2
	return nil
}

// journal collects one entity's events and writes them in one write, which is
// why bench.AppendEvent and its fsync per line are not used: every text the
// generator puts in an event comes from the word list, so the normalisation
// AppendEvent applies would change nothing.
type journal struct {
	buffer bytes.Buffer
}

// add appends one event as a line.
func (j *journal) add(ev bench.Event) error {
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	j.buffer.Write(line)
	j.buffer.WriteByte('\n')
	return nil
}

// save writes the collected lines to path.
func (j *journal) save(path string) error {
	return os.WriteFile(path, j.buffer.Bytes(), 0o644)
}

// clock issues one card's timestamps, each strictly later than the last.
type clock struct {
	g  *generator
	at time.Time
}

// tick advances the clock by a drawn one to three hundred seconds and
// answers the new moment as the format stamps it.
func (c *clock) tick() string {
	gap := 1 + c.g.number("event-gap", c.g.next("event-gap"), 300)
	c.at = c.at.Add(time.Duration(gap) * time.Second)
	return bench.Stamp(c.at)
}

// writeCard writes one live card: its items and their comments, its own
// comments, its attachments, its journal, and its anchor, which records the
// claim or block the journal ends in. A card planned as archived is written
// live with the stamp of its archived event settled, and archive moves it.
func (g *generator) writeCard(index int, plan *cardPlan, archived bool) error {
	dir := filepath.Join(g.root, bench.CardsDir, plan.id)
	start := epoch.Add(24*time.Hour + time.Duration(index)*8*time.Hour)
	start = start.Add(time.Duration(g.number("card-start", index, 3600)) * time.Second)
	clk := &clock{g: g, at: start}
	title := g.title("card-title", index)
	events := &journal{}
	created := bench.Event{
		TS:      clk.tick(),
		Event:   contract.EventCreated,
		Actor:   bench.NamedActor(operatorName),
		Title:   title,
		To:      g.columnIDs[0],
		ToTitle: flow[0].Title,
	}
	if err := events.add(created); err != nil {
		return err
	}
	for position := 1; position <= plan.column; position++ {
		moved := bench.Event{
			TS:        clk.tick(),
			Event:     contract.EventMoved,
			Actor:     bench.NamedActor(agentName),
			From:      g.columnIDs[position-1],
			FromTitle: flow[position-1].Title,
			To:        g.columnIDs[position],
			ToTitle:   flow[position].Title,
		}
		if err := events.add(moved); err != nil {
			return err
		}
	}
	if plan.workstream != "" {
		joined := bench.Event{
			TS:         clk.tick(),
			Event:      contract.EventWorkstreamJoined,
			Actor:      bench.NamedActor(operatorName),
			Workstream: plan.workstream,
		}
		if err := events.add(joined); err != nil {
			return err
		}
	}
	for ordinal, item := range plan.items {
		if err := g.writeItem(dir, ordinal+1, item, clk, events); err != nil {
			return err
		}
	}
	for ordinal := 1; ordinal <= plan.comments; ordinal++ {
		commentID, err := g.writeComment(dir, ordinal, clk)
		if err != nil {
			return err
		}
		commented := bench.Event{
			TS:      bench.Stamp(clk.at),
			Event:   contract.EventCommented,
			Actor:   bench.NamedActor(g.author(commentID)),
			Comment: commentID,
		}
		if err := events.add(commented); err != nil {
			return err
		}
	}
	for ordinal := 1; ordinal <= plan.attachments; ordinal++ {
		if err := g.writeAttachment(dir, plan.number, ordinal, clk, events); err != nil {
			return err
		}
	}
	for _, link := range plan.links {
		linkedEvent := bench.Event{
			TS:    clk.tick(),
			Event: contract.EventLinked,
			Actor: bench.NamedActor(agentName),
			Kind:  link.Kind,
			To:    link.To,
		}
		if err := events.add(linkedEvent); err != nil {
			return err
		}
	}
	for pair := 0; pair < plan.fillerPairs; pair++ {
		for _, name := range []string{contract.EventClaimed, contract.EventReleased} {
			filler := bench.Event{TS: clk.tick(), Event: name, Actor: bench.NamedActor(agentName)}
			if err := events.add(filler); err != nil {
				return err
			}
		}
	}
	card := &bench.Card{
		ID:       plan.id,
		Dir:      dir,
		Title:    title,
		Column:   g.columnIDs[plan.column],
		State:    plan.state,
		Severity: severities[g.number("card-severity", index, len(severities))],
		Priority: priorities[g.number("card-priority", index, len(priorities))],
		Tier:     tierNames[g.number("card-tier", index, len(tierNames))],
		Links:    plan.links,
		Body:     g.text("card-body", index, g.shape.CardBodyBytes, true),
		FM:       bench.NewFrontmatter(),
	}
	if plan.workstream != "" {
		card.Workstreams = []string{plan.workstream}
	}
	if err := g.finishCard(card, plan, clk, events, archived); err != nil {
		return err
	}
	if err := card.Save(); err != nil {
		return err
	}
	if err := events.save(card.JournalPath()); err != nil {
		return err
	}
	g.files += 2
	return nil
}

// finishCard writes the event a live card's journal ends in and records on the
// anchor what that event left standing. For a card about to be archived it
// settles the archived event's stamp instead, which archive records.
func (g *generator) finishCard(card *bench.Card, plan *cardPlan, clk *clock, events *journal, archived bool) error {
	if archived {
		plan.archivedAt = clk.tick()
		return nil
	}
	if plan.leadClaim {
		claimed := bench.Event{TS: clk.tick(), Event: contract.EventClaimed, Actor: bench.NamedActor(agentName)}
		if err := events.add(claimed); err != nil {
			return err
		}
	}
	switch plan.state {
	case contract.StateActive:
		claimed := bench.Event{TS: clk.tick(), Event: contract.EventClaimed, Actor: bench.NamedActor(agentName)}
		card.Holder = agentName
		card.ClaimSince = claimed.TS
		return events.add(claimed)
	case contract.StateBlocked:
		reason := g.text("block-reason", plan.number, 60, false)
		blocked := bench.Event{
			TS:     clk.tick(),
			Event:  contract.EventBlocked,
			Actor:  bench.NamedActor(agentName),
			Reason: reason,
			Kind:   blockKind,
		}
		card.BlockReason = reason
		card.BlockKind = blockKind
		card.BlockSince = blocked.TS
		return events.add(blocked)
	}
	return nil
}

// author draws who wrote a comment from its identifier, so the three names
// the workbench knows all appear.
func (g *generator) author(commentID string) string {
	authors := []string{operatorName, agentName, reviewerName}
	return authors[int(commentID[0])%len(authors)]
}

// writeComment writes one comment below holderDir with the next moment on the
// clock and answers its identifier. The caller journals it.
func (g *generator) writeComment(holderDir string, ordinal int, clk *clock) (string, error) {
	id := g.id("comment-id")
	dir := filepath.Join(holderDir, bench.CommentsDir, id)
	fm := bench.NewFrontmatter()
	fm.Set("ts", clk.tick())
	fm.Set("author", g.author(id))
	fm.Set(bench.OrdinalField, strconv.Itoa(ordinal))
	body := g.text("comment-body", g.next("comment-body"), g.shape.CommentBodyBytes, true)
	if err := bench.WriteCommentAnchor(dir, fm, body); err != nil {
		return "", err
	}
	g.files++
	return id, nil
}

// writeItem writes one checklist item with its comments and journals them. A
// settled item carries its state and a resolution naming its first comment,
// in the key order internal/verb/checklist.go leaves: the state rewritten in
// place and the resolution appended.
func (g *generator) writeItem(cardDir string, ordinal int, item *itemPlan, clk *clock, events *journal) error {
	dir := filepath.Join(cardDir, bench.ChecklistDir, item.id)
	filedAt := clk.tick()
	filed := bench.Event{
		TS:    filedAt,
		Event: contract.EventItemFiled,
		Actor: bench.NamedActor(agentName),
		Item:  item.id,
		Kind:  item.kind,
	}
	if err := events.add(filed); err != nil {
		return err
	}
	var commentIDs []string
	for k := 1; k <= item.comments; k++ {
		commentID, err := g.writeComment(dir, k, clk)
		if err != nil {
			return err
		}
		commentIDs = append(commentIDs, commentID)
		commented := bench.Event{
			TS:      bench.Stamp(clk.at),
			Event:   contract.EventCommented,
			Actor:   bench.NamedActor(g.author(commentID)),
			Item:    item.id,
			Comment: commentID,
		}
		if err := events.add(commented); err != nil {
			return err
		}
		if k == 1 && item.state != bench.ItemPending {
			settled := bench.Event{
				TS:    clk.tick(),
				Event: settleEvent(item.state),
				Actor: bench.NamedActor(g.author(commentID)),
				Item:  item.id,
				From:  bench.ItemPending,
				To:    item.state,
			}
			if err := events.add(settled); err != nil {
				return err
			}
		}
	}
	fm := bench.NewFrontmatter()
	fm.Set(bench.ItemKindField, item.kind)
	fm.Set(bench.ItemStateField, item.state)
	fm.Set(bench.ItemColumnField, g.columnBySlug(itemColumnSlug[item.kind]))
	fm.Set(bench.ItemOwnerField, itemOwner)
	fm.Set("ts", filedAt)
	fm.Set(bench.OrdinalField, strconv.Itoa(ordinal))
	if item.state != bench.ItemPending {
		fm.Set(bench.ItemResolutionField, commentIDs[0])
	}
	body := g.text("item-body", g.next("item-body"), g.shape.ItemBodyBytes, true)
	if err := bench.WriteItemAnchor(dir, fm, body); err != nil {
		return err
	}
	g.files++
	return nil
}

// settleEvent names the journal event that lands an item in a settled state.
func settleEvent(state string) string {
	switch state {
	case bench.ItemVerified:
		return contract.EventItemVerified
	case bench.ItemFailed:
		return contract.EventItemFailed
	}
	return contract.EventItemResolved
}

// columnBySlug answers a flow column's identifier.
func (g *generator) columnBySlug(slug string) string {
	for position, spec := range flow {
		if spec.Slug == slug {
			return g.columnIDs[position]
		}
	}
	return ""
}

// writeAttachment writes one attachment of a card with the anchor keys
// bench.AddAttachmentBytes writes, in its order, and the payload byte for
// byte, then journals it.
func (g *generator) writeAttachment(cardDir string, number, ordinal int, clk *clock, events *journal) error {
	id := g.id("attachment-id")
	dir := filepath.Join(cardDir, bench.AttachmentsDir, id)
	index := g.next("attachment")
	filename := workbenchSlug + "-" + strconv.Itoa(number) + "-" + words[g.number("attachment-name", index, len(words))] + ".md"
	fm := bench.NewFrontmatter()
	fm.Set("filename", filename)
	fm.Set("description", g.text("attachment-description", index, 80, false))
	fm.Set("provenance", agentName)
	fm.Set(bench.OrdinalField, strconv.Itoa(ordinal))
	if err := bench.WriteText(filepath.Join(dir, bench.AttachmentAnchor), fm.Render("")); err != nil {
		return err
	}
	payload := filepath.Join(dir, bench.PayloadDir, filename)
	if err := os.MkdirAll(filepath.Dir(payload), 0o755); err != nil {
		return err
	}
	content := g.text("attachment-payload", index, g.shape.PayloadBytes, true) + "\n"
	if err := os.WriteFile(payload, []byte(content), 0o644); err != nil {
		return err
	}
	g.files += 2
	attached := bench.Event{
		TS:         clk.tick(),
		Event:      contract.EventAttached,
		Actor:      bench.NamedActor(agentName),
		Attachment: id,
		Filename:   filename,
	}
	return events.add(attached)
}
