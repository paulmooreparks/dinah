package bench

import (
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// Workstream is one named grouping of cards inside a workbench: an entity of
// its own carrying a title, a slug, a status and the long-form notes a person
// writes about the effort.
//
// It records nothing about which cards belong to it. Membership is card-owned,
// a workstreams list in card frontmatter, so every count and every listing of
// members here is derived by walking the live cards.
type Workstream struct {
	// ID is the workstream's 12-hex identifier and the name of its directory.
	ID string
	// Dir is the workstream's directory, in whichever half of the collection
	// it currently sits.
	Dir string
	// Title is what a person calls the workstream. It is empty on a
	// workstream the adoption repair created, because there was no title to
	// recover.
	Title string
	// Slug is the short handle a person types instead of the identifier. It
	// is empty on a workstream carrying none, which is what the slug
	// migration fills in.
	Slug string
	// Status is an open value: Dinah writes active at creation and accepts
	// any non-empty value afterwards, because nothing in the tool refuses,
	// orders, counts or routes anything on it.
	Status string
	// Ordinal is the creation ordinal, counting the workstream's arrival
	// within this workbench.
	Ordinal int
	// Notes are the workstream's own body, which carries no imposed shape.
	Notes string
	// FM is the anchor's header, kept so a write preserves unknown keys.
	FM *Frontmatter
}

// WorkstreamFields are the fields of a workstream a person wrote and may
// rewrite. The ordinal beside them is not a person's to type.
var WorkstreamFields = []string{"title", "slug", "status"}

// KnownWorkstreamField reports whether a name is one of the fields a
// workstream records, which is what both a read of one and a write to one ask
// first.
func KnownWorkstreamField(name string) bool {
	for _, known := range WorkstreamFields {
		if known == name {
			return true
		}
	}
	return false
}

// StatusActive is the status Dinah writes when it creates a workstream, so the
// field is never absent on one the tool made.
const StatusActive = "active"

// WorkstreamsRoot is the live workstreams collection.
func (b *Bench) WorkstreamsRoot() string {
	return filepath.Join(b.Root, WorkstreamsDir)
}

// ArchivedWorkstreamsRoot is the archived half of the workstreams collection.
// Resolution spans both halves, while listings and counts see the live half
// alone, which is the split the cards collection already makes.
func (b *Bench) ArchivedWorkstreamsRoot() string {
	return filepath.Join(b.Root, ArchiveDir, WorkstreamsDir)
}

// WorkstreamAnchorPath is the anchor of one live workstream.
func (b *Bench) WorkstreamAnchorPath(id string) string {
	return filepath.Join(b.WorkstreamsRoot(), id, WorkstreamAnchor)
}

// AnchorPath is the workstream's anchor file.
func (w *Workstream) AnchorPath() string {
	return filepath.Join(w.Dir, WorkstreamAnchor)
}

// JournalPath is the workstream's journal, which appears at creation and
// records the workstream's own arc.
func (w *Workstream) JournalPath() string {
	return filepath.Join(w.Dir, JournalName)
}

// Ref is what a person types to reach this workstream: its own slug when it
// carries one, its identifier otherwise. Mirrors State.Ref's fallback, so a
// workstream carrying no slug still gives a caller something to type.
func (w *Workstream) Ref() string {
	if w.Slug != "" {
		return w.Slug
	}
	return w.ID
}

// Field reads one of the workstream's own fields by name, and answers the
// empty string for a name outside the set. The caller refuses over the name;
// this reports what is stored under it.
func (w *Workstream) Field(name string) string {
	switch name {
	case "title":
		return w.Title
	case "slug":
		return w.Slug
	case "status":
		return w.Status
	}
	return ""
}

// SetField writes one of the workstream's own fields in memory, ready for
// Save. A name outside the set writes nothing, since the caller has already
// refused over it.
func (w *Workstream) SetField(name, value string) {
	switch name {
	case "title":
		w.Title = value
	case "slug":
		w.Slug = value
	case "status":
		w.Status = value
	}
}

// LoadWorkstream reads one workstream from a workstreams collection.
//
// An anchor carrying no parseable frontmatter loads with empty fields rather
// than failing, which is the state the adoption repair leaves a workstream in
// and the state a hand-written directory may already be in. The slug findings
// name it, so nothing here has to refuse over it.
func LoadWorkstream(collection, id string) (*Workstream, error) {
	dir := filepath.Join(collection, id)
	text, err := ReadText(filepath.Join(dir, WorkstreamAnchor))
	if err != nil {
		return nil, contract.Refuse(contract.UnknownWorkstream, id)
	}
	fm, body := ParseAnchor(text)
	workstream := &Workstream{
		ID:      id,
		Dir:     dir,
		Title:   fm.Value("title"),
		Slug:    fm.Value(SlugField),
		Status:  fm.Value("status"),
		Ordinal: OrdinalOf(fm),
		Notes:   body,
		FM:      fm,
	}
	return workstream, nil
}

// Save writes the workstream anchor back, preserving every key it does not
// itself set and every key's position.
func (w *Workstream) Save() error {
	w.FM.Set("title", w.Title)
	if w.Slug != "" {
		w.FM.SetAfter(SlugField, w.Slug, "title")
	}
	if w.Status != "" {
		w.FM.Set("status", w.Status)
	}
	if w.Ordinal > 0 {
		w.FM.Set(OrdinalField, strconv.Itoa(w.Ordinal))
	}
	return WriteText(w.AnchorPath(), w.FM.Render(w.Notes))
}

// workstreamsIn reads every workstream of one half of the collection, in
// creation order.
//
// The order is the ordinal's rather than the directory listing's, because a
// hex identifier is random and a listing ordered by one is in no order
// anybody wrote in. A directory whose name is not a 12-hex identifier is
// invisible here, because ListIDs drops it, and a directory carrying no
// anchor is skipped rather than refused, which is what leaves the listing
// able to name the bad directory instead of disappearing over it.
func workstreamsIn(collection string) []*Workstream {
	var workstreams []*Workstream
	for _, id := range SortByOrdinal(collection, WorkstreamAnchor, ListIDs(collection)) {
		workstream, err := LoadWorkstream(collection, id)
		if err != nil {
			continue
		}
		workstreams = append(workstreams, workstream)
	}
	return workstreams
}

// Workstreams reads the live workstreams of the bench, in creation order.
func (b *Bench) Workstreams() []*Workstream {
	return workstreamsIn(b.WorkstreamsRoot())
}

// Workstream returns the live or archived workstream carrying an identifier,
// or nil when the bench holds none. Resolution spans both halves, so a card
// belonging to an archived workstream is not a dangler.
func (b *Bench) Workstream(id string) *Workstream {
	if !IsID(id) {
		return nil
	}
	for _, collection := range []string{b.WorkstreamsRoot(), b.ArchivedWorkstreamsRoot()} {
		if !Exists(filepath.Join(collection, id, WorkstreamAnchor)) {
			continue
		}
		workstream, err := LoadWorkstream(collection, id)
		if err != nil {
			continue
		}
		return workstream
	}
	return nil
}

// WorkstreamByRef returns the workstream a bare reference names, accepting the
// identifier first and the slug second, both halves of the collection at each
// pass. It never resolves a title.
//
// The order is load-bearing rather than tidy. ValidStateSlug admits a slug of
// twelve characters drawn from the letters a to f, so one workstream's slug
// can be another workstream's identifier, and the identifier wins. Dropping
// the title pass StateByRef makes keeps one rule on every surface: no
// workstream is ever named by its title, so `dinah workstream new Portfolio`
// and `dinah workstream get Portfolio` cannot read the same word two ways.
func (b *Bench) WorkstreamByRef(ref string) *Workstream {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if found := b.Workstream(ref); found != nil {
		return found
	}
	want := asciiLower(ref)
	for _, collection := range []string{b.WorkstreamsRoot(), b.ArchivedWorkstreamsRoot()} {
		for _, workstream := range workstreamsIn(collection) {
			if workstream.Slug != "" && asciiLower(workstream.Slug) == want {
				return workstream
			}
		}
	}
	return nil
}

// HasWorkstream reports whether an identifier names a workstream in either
// half of the collection, which is the resolution scope a card's membership
// is checked against.
func (b *Bench) HasWorkstream(id string) bool {
	return b.Workstream(id) != nil
}

// WorkstreamCounts reports how many live cards belong to each workstream,
// keyed by identifier. Membership is card-owned, so the count is derived by
// walking the cards the way the Cards column of dinah states already is.
func (b *Bench) WorkstreamCounts() (map[string]int, error) {
	cards, err := b.Cards()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, card := range cards {
		for _, id := range card.Workstreams {
			counts[id]++
		}
	}
	return counts, nil
}

// WorkstreamReferenced reports whether live cards still belong to a
// workstream, which is what refuses its deletion. A nil answer means nothing
// does; anything else is the refusal to report, naming the workstream by what
// a person typed rather than by the identifier behind it.
//
// The population is the live half of the cards collection alone. Check walks
// the live half and the dangling-link finding draws its own population the
// same way, so this follows the tool rather than widening one finding's reach
// on its own. Deleting a workstream that only archived cards list therefore
// leaves each of those cards carrying an identifier that resolves to nothing,
// which is a record of what the card belonged to on the day it was archived
// rather than a live reference.
func (b *Bench) WorkstreamReferenced(id, ref string) error {
	for _, cardID := range ListIDs(b.CardsRoot()) {
		card, err := LoadCard(b.CardsRoot(), cardID)
		if err != nil {
			continue
		}
		for _, joined := range card.Workstreams {
			if joined == id {
				return contract.Refuse(contract.Referenced, ref)
			}
		}
	}
	return nil
}

// NewWorkstream creates a workstream from a title: the directory, the anchor
// carrying the four fields, and nothing else. The journal's opening event is
// the caller's to append, the way AddComment leaves the card's event to the
// verb that wrote the comment.
//
// The slug is derived from the whole title through SlugifyDashed rather than
// through Slugify. Slugify strips exactly the trailing dash and digits a
// workstream slug is allowed to carry, so a workstream titled "Sprint 2" would
// be born sprint2 and could never be born sprint-2, and the collision
// resolver's own -2 suffix would be a slug no creation could write. A title
// yielding no usable slug leaves the field empty, which the slug findings name
// and the slug migration repairs.
//
// The caller holds the workbench's lock, which is what makes the ordinal scan
// and the collision scan race-free.
func (b *Bench) NewWorkstream(title string) (*Workstream, error) {
	collection := b.WorkstreamsRoot()
	id, err := ClaimID(collection, b.HasWorkstream)
	if err != nil {
		return nil, err
	}
	taken := map[string]bool{}
	for _, existing := range b.Workstreams() {
		if existing.Slug != "" {
			taken[existing.Slug] = true
		}
	}
	workstream := &Workstream{
		ID:      id,
		Dir:     filepath.Join(collection, id),
		Title:   title,
		Slug:    FreeSlug(SlugifyDashed(title), taken),
		Status:  StatusActive,
		Ordinal: nextOrdinal(collection, WorkstreamAnchor),
		FM:      NewFrontmatter(),
	}
	if err := workstream.Save(); err != nil {
		return nil, err
	}
	return workstream, nil
}

// AdoptWorkstream creates a workstream at an identifier a card already names,
// so that a membership written before the tool could read a workstream
// resolves without any card file being touched.
//
// It carries no title, because there is no title to recover, and no slug,
// because a slug is derived from a title. The workstream reports as
// slug-missing until somebody names it and runs the slug migration.
func (b *Bench) AdoptWorkstream(id string) (*Workstream, error) {
	collection := b.WorkstreamsRoot()
	dir := filepath.Join(collection, id)
	workstream := &Workstream{
		ID:      id,
		Dir:     dir,
		Status:  StatusActive,
		Ordinal: nextOrdinal(collection, WorkstreamAnchor),
		FM:      NewFrontmatter(),
	}
	if err := workstream.Save(); err != nil {
		return nil, err
	}
	return workstream, nil
}

// DanglingWorkstreams reports every identifier the live cards list that names
// no workstream in either half of the collection, in the order the cards were
// read, without repeating one two cards share.
func (b *Bench) DanglingWorkstreams() ([]string, error) {
	cards, err := b.Cards()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var dangling []string
	for _, card := range cards {
		for _, id := range card.Workstreams {
			if seen[id] || b.HasWorkstream(id) {
				continue
			}
			seen[id] = true
			dangling = append(dangling, id)
		}
	}
	return dangling, nil
}

// checkWorkstreams applies the collection's own invariants: a directory
// carrying no anchor is reported, every workstream carries a slug, each one
// conforms to the grammar, and no two workstreams share one.
//
// A directory whose name is not a 12-hex identifier is invisible to this walk,
// because ListIDs drops it, which is how the cards collection already treats
// one. A directory carrying no anchor is reported and skipped rather than
// refused, unlike a card directory carrying none: Dinah wrote every card
// directory, so a missing card anchor means a torn act, while the format
// invites a person to write a workstream directory by hand, and refusing the
// listing over one bad directory would take away the command that names it.
func (b *Bench) checkWorkstreams() []Finding {
	var findings []Finding
	seen := map[string]bool{}
	for _, id := range ListIDs(b.WorkstreamsRoot()) {
		dir := filepath.Join(b.WorkstreamsRoot(), id)
		if !Exists(filepath.Join(dir, WorkstreamAnchor)) {
			findings = append(findings, Finding{Path: dir, Key: FindingMissingAnchor, Detail: id})
			continue
		}
		workstream, err := LoadWorkstream(b.WorkstreamsRoot(), id)
		if err != nil {
			findings = append(findings, Finding{Path: dir, Key: FindingMissingAnchor, Detail: id})
			continue
		}
		path := b.WorkstreamAnchorPath(id)
		switch {
		case workstream.Slug == "":
			findings = append(findings, Finding{Path: path, Key: FindingWorkstreamSlugMissing, Detail: id})
		case !ValidStateSlug(workstream.Slug):
			findings = append(findings, Finding{Path: path, Key: FindingWorkstreamSlugMalformed, Detail: id})
		case seen[workstream.Slug]:
			findings = append(findings, Finding{Path: path, Key: FindingWorkstreamSlugDuplicate, Detail: id})
		default:
			seen[workstream.Slug] = true
		}
	}
	return findings
}

// WorkstreamSlugAssignment is one workstream the slug migration repaired,
// named together with the slug it was given.
//
// It stands beside SlugAssignment rather than reusing it because the member
// naming the entity is part of the machine surface, and reporting a workstream
// under a member called state would tell a caller it repaired something else.
type WorkstreamSlugAssignment struct {
	// Workstream is the identifier of the workstream repaired.
	Workstream string `json:"workstream"`
	// Title is that workstream's title, which is what the slug was derived
	// from.
	Title string `json:"title"`
	// Slug is the slug written to the workstream's anchor.
	Slug string `json:"slug"`
}

// BackfillWorkstreamSlugs derives a slug for every live workstream carrying
// none and writes it to that workstream's anchor, reporting what it assigned
// and returning a finding for every workstream it could not repair.
//
// It follows BackfillStateSlugs in every respect that matters: the walk takes
// the workstreams in creation order so the answers are the same on every
// machine, a slug already on disk is left exactly as it stands including a
// malformed or duplicated one, and a workstream this run cannot write is
// reported and stepped over rather than costing the operator the account of
// the ones already repaired.
func (b *Bench) BackfillWorkstreamSlugs() ([]WorkstreamSlugAssignment, []Finding) {
	workstreams := b.Workstreams()
	taken := map[string]bool{}
	for _, workstream := range workstreams {
		if workstream.Slug != "" {
			taken[workstream.Slug] = true
		}
	}
	assigned := []WorkstreamSlugAssignment{}
	var findings []Finding
	for _, workstream := range workstreams {
		if workstream.Slug != "" {
			continue
		}
		path := b.WorkstreamAnchorPath(workstream.ID)
		derived := SlugifyDashed(workstream.Title)
		if derived == "" {
			findings = append(findings, Finding{Path: path, Key: FindingWorkstreamSlugUnderivable, Detail: workstream.ID})
			continue
		}
		candidate := FreeSlug(derived, taken)
		if err := stampSlug(path, candidate); err != nil {
			findings = append(findings, Finding{Path: path, Key: FindingWorkstreamSlugUnwritable, Detail: workstream.ID})
			continue
		}
		taken[candidate] = true
		workstream.Slug = candidate
		assigned = append(assigned, WorkstreamSlugAssignment{Workstream: workstream.ID, Title: workstream.Title, Slug: candidate})
	}
	return assigned, findings
}
