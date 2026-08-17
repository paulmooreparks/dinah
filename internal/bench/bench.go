package bench

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// The names the format fixes for anchors, collections and the user base.
const (
	WorkbenchAnchor  = "workbench.md"
	StateAnchor      = "state.md"
	CardAnchor       = "card.md"
	CommentAnchor    = "comment.md"
	AttachmentAnchor = "attachment.md"
	ItemAnchor       = "item.md"
	JournalName      = "journal.ndjson"
	PayloadDir       = "payload"
	UserBaseName     = ".dinah"
	ConfigName       = "config.md"
	InstructionsName = "instructions.md"
	IgnoreName       = ".gitignore"
)

// ignoreLocks is what a new bench's .gitignore carries. A bench inside a
// repository is versioned by that repository, and a lock is coordination-plane
// state on one machine, so committing one would ship a stale holder to
// everybody who clones. Nothing the tool reads is affected either way, since
// every listing walks the identifiers of a collection and no lock is ever a
// member of one; this is about what git picks up.
const ignoreLocks = "lock\n*.lock\n"

// The collection directory names.
const (
	StatesDir      = "states"
	CardsDir       = "cards"
	CommentsDir    = "comments"
	AttachmentsDir = "attachments"
	ChecklistDir   = "checklist"
	WorkstreamsDir = "workstreams"
	ArchiveDir     = "archive"
)

// StorageFormat is the storage format version this binary implements. A bench
// declaring a higher number is refused loudly, naming the version it wanted.
const StorageFormat = 1

// The profile version this binary conforms to. The two numbers are the
// conformance claim CORE-VER-1 requires, and no channel name joins them,
// which CORE-VER-2 forbids.
const (
	ProfileName  = "dinah-core"
	ProfileMajor = 1
	ProfileMinor = 0
)

// ProfileVersion is the conformance claim as the interchange form spells it.
var ProfileVersion = ProfileName + "/" + strconv.Itoa(ProfileMajor) + "." + strconv.Itoa(ProfileMinor)

// State is one station of the flow.
type State struct {
	// ID is the state's 12-hex identifier and the name of its directory.
	ID string
	// Title is what a person calls the state.
	Title string
	// Kind is one of intake, work and done.
	Kind string
	// OperatorOwned marks a state an owner other than the operator may not
	// move a card out of.
	OperatorOwned bool
	// Capacity is the state's declared limit, or zero for unlimited.
	Capacity int
	// Instructions is the state's own body, the last layer of the chain.
	Instructions string
	// Position is the state's zero-based index in the flow.
	Position int
	// FM is the anchor's header, kept so a write preserves unknown keys.
	FM *Frontmatter
}

// ErrAborted is what a test's step hook returns to stand for a process that
// died at that step. The act stops where it is, releasing nothing and
// unwinding nothing, so the tree is left in the state a crash leaves and the
// recovery path can be driven over it.
var ErrAborted = errors.New("the structural act was aborted")

// Hooks are the levers a test drives the protocol's own timing with. They are
// nil on every bench outside a test, exactly as the verb layer's interleaving
// hook is.
type Hooks struct {
	// AfterStep runs after each numbered step of the structural protocol
	// completes. Returning ErrAborted stands for a crash at that step, and
	// returning any other error stands for a live failure there.
	AfterStep func(step int) error
	// BeforeAnchorRead runs inside the state scan between the stat of one
	// card's lock and the read of that card's anchor, which is the gap a
	// wrongly ordered scan would let a whole critical section through.
	BeforeAnchorRead func(id string)
}

// Bench is an opened workbench: its definition, its states in flow order and
// the root directory everything below it hangs from.
type Bench struct {
	// Root is the bench directory, the one holding workbench.md.
	Root string
	// Title is the workbench's title.
	Title string
	// Slug is the short name a card reference carries ahead of its number.
	Slug string
	// Operator is the owner reserved acts belong to. An empty operator is
	// what CORE-OWNER-3 refuses every verb over.
	Operator string
	// Standing is the workbench body, the middle layer of the chain.
	Standing string
	// States are the flow in the order workbench.md declares.
	States []*State
	// Format is the declared storage format version.
	Format int
	// Profile is the declared conformance target.
	Profile string
	// FM is the anchor's header, kept so a write preserves unknown keys.
	FM *Frontmatter
	// Hooks are the test-only levers on the structural protocol's timing.
	Hooks *Hooks
}

// Discover finds the bench to serve. An override, from the --bench flag or
// from DINAH_BENCH, is taken as given. Otherwise the search walks up from the
// starting directory the way git looks for a repository, and falls back to
// the user base.
//
// A base holding several workbenches never decides the search, and the walk
// climbs past it as it always has. It is remembered, though: a search that
// ends with nothing found reports the ambiguity it passed rather than telling
// the reader that no workbench exists, which is false wherever one was seen.
func Discover(start, override, home string) (string, error) {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		if !Exists(filepath.Join(abs, WorkbenchAnchor)) {
			return "", contract.Refuse(contract.NoBench, abs)
		}
		return abs, nil
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	// The first ambiguity the search meets, closest to the starting
	// directory, which is the one a reader is likeliest to have meant.
	firstBase := ""
	var firstCandidates []string
	for {
		found, ambiguous := benchIn(dir)
		if found != "" {
			return found, nil
		}
		if len(ambiguous) > 0 && firstBase == "" {
			firstBase, firstCandidates = filepath.Join(dir, UserBaseName), ambiguous
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	userBase := ""
	if home != "" {
		userBase = filepath.Join(home, UserBaseName)
		found, ambiguous := soleBench(userBase)
		if found != "" {
			return found, nil
		}
		if len(ambiguous) > 0 && firstBase == "" {
			firstBase, firstCandidates = userBase, ambiguous
		}
	}
	if firstBase != "" {
		extra := map[string]string{"base": firstBase}
		return "", contract.RefuseWith(contract.AmbiguousBench, describeCandidates(firstCandidates), extra)
	}
	return "", contract.RefuseWith(contract.NoBenchFound, start, map[string]string{"home": userBase})
}

// benchIn reports the bench a single directory offers, either as the bench
// itself or as the sole bench of a .dinah directory inside it. The second
// value carries the candidates when that .dinah holds several, which is what
// tells a caller apart from a base holding nothing at all.
func benchIn(dir string) (found string, ambiguous []string) {
	if Exists(filepath.Join(dir, WorkbenchAnchor)) {
		return dir, nil
	}
	return soleBench(filepath.Join(dir, UserBaseName))
}

// soleBench returns the one bench a base directory holds. A base holding
// several is ambiguous, so it returns no bench and the candidates instead,
// and the walk continues rather than picking one.
func soleBench(base string) (found string, ambiguous []string) {
	var candidates []string
	for _, id := range ListIDs(base) {
		candidate := filepath.Join(base, id)
		if !Exists(filepath.Join(candidate, WorkbenchAnchor)) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return "", candidates
	}
	return "", nil
}

// describeCandidates names each workbench of an ambiguous base by its title
// and the directory it sits in, joined into the list the refusal prints. A
// candidate whose anchor will not read, or that declares no title, is named
// by its path alone, so one unreadable workbench does not hide the others.
func describeCandidates(candidates []string) string {
	described := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		title := ""
		if text, err := ReadText(filepath.Join(candidate, WorkbenchAnchor)); err == nil {
			fm, _ := ParseAnchor(text)
			title = fm.Value("title")
		}
		if title == "" {
			described = append(described, candidate)
			continue
		}
		described = append(described, title+" ("+candidate+")")
	}
	return strings.Join(described, "; ")
}

// Open reads a bench definition and the states it declares.
//
// The refusals here are the profile's own: a definition missing a title, a
// state list or a profile declaration is malformed, a definition declaring a
// major number this binary does not implement is unsupported-version, and a
// bench whose storage format is newer than this binary knows is refused with
// the version it wanted named. Each malformed refusal carries the file it was
// raised over, so a reader knows which workbench to repair.
func Open(root string) (*Bench, error) {
	anchor := map[string]string{"path": filepath.Join(root, WorkbenchAnchor)}
	text, err := ReadText(filepath.Join(root, WorkbenchAnchor))
	if err != nil {
		return nil, contract.RefuseWith(contract.Malformed, WorkbenchAnchor, anchor)
	}
	fm, body := ParseAnchor(text)
	b := &Bench{
		Root:     root,
		Title:    fm.Value("title"),
		Slug:     fm.Value("slug"),
		Operator: fm.Value("operator"),
		Standing: body,
		Profile:  fm.Value("profile"),
		FM:       fm,
	}
	if b.Title == "" {
		return nil, contract.RefuseWith(contract.Malformed, "title", anchor)
	}
	if b.Profile == "" {
		return nil, contract.RefuseWith(contract.Malformed, "profile", anchor)
	}
	major, minor, ok := splitProfile(b.Profile)
	if !ok {
		return nil, contract.RefuseWith(contract.Malformed, "profile", anchor)
	}
	if major != ProfileMajor || minor > ProfileMinor {
		return nil, contract.Refuse(contract.UnsupportedVer, b.Profile)
	}
	b.Format = StorageFormat
	if declared := fm.Value("format"); declared != "" {
		n, err := strconv.Atoi(declared)
		if err != nil {
			return nil, contract.RefuseWith(contract.Malformed, "format", anchor)
		}
		if n > StorageFormat {
			return nil, contract.Refuse(contract.UnsupportedVer, "format "+declared)
		}
		b.Format = n
	}
	ids := fm.Seq("states")
	if len(ids) == 0 {
		return nil, contract.RefuseWith(contract.Malformed, "states", anchor)
	}
	seen := map[string]bool{}
	for position, id := range ids {
		if seen[id] {
			return nil, contract.RefuseWith(contract.Malformed, "states", anchor)
		}
		seen[id] = true
		state, err := readState(root, id, position)
		if err != nil {
			return nil, err
		}
		b.States = append(b.States, state)
	}
	return b, nil
}

// splitProfile reads a conformance target of the form dinah-core/1.0 into its
// major and minor numbers.
func splitProfile(declared string) (int, int, bool) {
	name, version, ok := strings.Cut(declared, "/")
	if !ok || name != ProfileName {
		return 0, 0, false
	}
	majorText, minorText, ok := strings.Cut(version, ".")
	if !ok {
		return 0, 0, false
	}
	major, err := strconv.Atoi(majorText)
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(minorText)
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// readState reads one state anchor and applies the profile's own checks on a
// state: a duplicate identifier, an absent title and a kind outside the three
// are each malformed.
func readState(root, id string, position int) (*State, error) {
	path := filepath.Join(root, StatesDir, id, StateAnchor)
	anchor := map[string]string{"path": path}
	text, err := ReadText(path)
	if err != nil {
		return nil, contract.RefuseWith(contract.Malformed, "state "+id, anchor)
	}
	fm, body := ParseAnchor(text)
	state := &State{
		ID:            id,
		Title:         fm.Value("title"),
		Kind:          fm.Value("kind"),
		OperatorOwned: fm.Value("operator_owned") == "true",
		Instructions:  body,
		Position:      position,
		FM:            fm,
	}
	if state.Title == "" {
		return nil, contract.RefuseWith(contract.Malformed, "state "+id, anchor)
	}
	switch state.Kind {
	case contract.KindIntake, contract.KindWork, contract.KindDone:
	default:
		return nil, contract.RefuseWith(contract.Malformed, "state "+id, anchor)
	}
	if limit := fm.Value("wip_limit"); limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return nil, contract.RefuseWith(contract.Malformed, "state "+id, anchor)
		}
		state.Capacity = n
	}
	return state, nil
}

// State returns the state carrying an identifier, or nil when the bench
// declares none.
func (b *Bench) State(id string) *State {
	for _, state := range b.States {
		if state.ID == id {
			return state
		}
	}
	return nil
}

// StateByRef returns the state a reference names, accepting the identifier or
// the title compared without regard to ASCII case, which is what makes a
// state nameable on a command line at all.
func (b *Bench) StateByRef(ref string) *State {
	if state := b.State(ref); state != nil {
		return state
	}
	want := asciiLower(strings.TrimSpace(ref))
	for _, state := range b.States {
		if asciiLower(state.Title) == want {
			return state
		}
	}
	return nil
}

// asciiLower lowercases using ASCII rules alone. The locale-aware form is a
// correctness bug the format's encoding section names: an uppercase I
// lowercased under a Turkish locale is not i, and a bench must parse
// identically in Istanbul and in Iowa.
func asciiLower(s string) string {
	out := []byte(s)
	for i := 0; i < len(out); i++ {
		if out[i] >= 'A' && out[i] <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}

// CardsRoot is the live cards collection.
func (b *Bench) CardsRoot() string {
	return filepath.Join(b.Root, CardsDir)
}

// ArchivedCardsRoot is the archived half of the cards collection. Identifier
// uniqueness spans both halves, while listings, pull and capacity counts see
// the live half alone.
func (b *Bench) ArchivedCardsRoot() string {
	return filepath.Join(b.Root, ArchiveDir, CardsDir)
}

// JournalPath is the bench's own journal, which appears on the first
// bench-scoped act rather than at creation.
func (b *Bench) JournalPath() string {
	return filepath.Join(b.Root, JournalName)
}

// Cards reads every live card of the bench, in ascending identifier order.
func (b *Bench) Cards() ([]*Card, error) {
	var cards []*Card
	for _, id := range ListIDs(b.CardsRoot()) {
		card, err := LoadCard(b.CardsRoot(), id)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// HasIdentifier reports whether an identifier is already in use in either
// half of the cards collection, which is the uniqueness scope the archive
// section fixes.
func (b *Bench) HasIdentifier(id string) bool {
	if Exists(filepath.Join(b.CardsRoot(), id)) {
		return true
	}
	return Exists(filepath.Join(b.ArchivedCardsRoot(), id))
}

// NextNumber returns the number a newly filed card carries: one past the
// highest in use across both halves of the collection. Numbers are the
// durable half of a card reference, so a number is never reused.
func (b *Bench) NextNumber() int {
	highest := 0
	for _, root := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		for _, id := range ListIDs(root) {
			card, err := LoadCard(root, id)
			if err != nil {
				continue
			}
			if card.Number > highest {
				highest = card.Number
			}
		}
	}
	return highest + 1
}

// Save writes the workbench anchor back, preserving every key it does not
// itself set.
func (b *Bench) Save() error {
	b.FM.Set("title", b.Title)
	if b.Slug != "" {
		b.FM.Set("slug", b.Slug)
	}
	if b.Operator != "" {
		b.FM.Set("operator", b.Operator)
	}
	return WriteText(filepath.Join(b.Root, WorkbenchAnchor), b.FM.Render(b.Standing))
}

// UserBase returns the user base directory, which is where the user's own
// config, the user-global instruction layer and benches belonging to no
// repository live.
func UserBase(home string) string {
	return filepath.Join(home, UserBaseName)
}

// Home returns the user's home directory, preferring the environment so a
// test can point the whole user base somewhere disposable.
func Home() string {
	if dir := os.Getenv("DINAH_HOME"); dir != "" {
		return dir
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return dir
}
