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

// SlugMandatoryMajor is the major number CORE-STATE-10, which requires every
// state to carry a slug, is published under. A workbench declaring an earlier
// major is not held to that statement, because a conformance claim names one
// major and is evaluated over the statements that revision publishes, so a
// state carrying no slug opens there. A workbench declaring this major or a
// later one is held to it and is refused.
//
// The gate is written against the major a workbench declares rather than
// against ProfileMajor, so the day a release raises this binary's own claim,
// no reader of a state has to be edited to make the requirement bite.
const SlugMandatoryMajor = 3

// State is one station of the flow.
type State struct {
	// ID is the state's 12-hex identifier and the name of its directory.
	ID string
	// Title is what a person calls the state.
	Title string
	// Slug is the short handle a person types instead of quoting the title.
	// It is empty on a state written before the field existed, which is what
	// the slug migration fills in.
	Slug string
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
	// BeforeOrdinalStamp runs before the ordinal migration writes an entity's
	// anchor, and an error it returns stands for that write failing. A test
	// reaches the migration's unwritable-entity path through this hook rather
	// than through file permissions, because what a permission bit forbids is
	// a question each operating system answers differently: the migration
	// writes a temporary beside the anchor and renames it over, and on POSIX
	// the right to replace a name belongs to the containing directory rather
	// than to the file being replaced, so a read-only anchor is overwritten
	// there and refused on Windows. The hook provokes one path on every
	// platform, and the divergence itself is covered separately by a test that
	// makes the directory unwritable and runs only where that means something.
	BeforeOrdinalStamp func(id string) error
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
	// StrandedStates is every identifier this workbench's own definition names
	// whose directory is not there at all: a state a retiring act moved or
	// removed without also editing the definition, or one a person removed by
	// hand. It plays no part in the flow; dinah check reports it and dinah
	// check --migrate-states removes it from the definition.
	StrandedStates []string
	// Passed is the workbench.md files the discovery walk found and did not
	// claim on its way to resolving this bench, each one a directory holding
	// somebody else's document rather than a Dinah workbench. It is set by
	// whichever caller resolved this bench through Discover, and stays nil
	// on a bench Open reads directly by path (extraction, the interchange
	// import path, every test that skips discovery).
	Passed []string
}

// Discover finds the bench to serve. An override, from the --workbench flag
// or from DINAH_WORKBENCH, is taken as given. Otherwise the search walks up
// from the starting directory the way git looks for a repository, and falls
// back to the user base.
//
// A base holding several workbenches never decides the search, and the walk
// climbs past it as it always has. It is remembered, though: a search that
// ends with nothing found reports the ambiguity it passed rather than telling
// the reader that no workbench exists, which is false wherever one was seen.
//
// nativeHome is the machine's own home directory, from NativeHome, and it is
// separate from home because home honours DINAH_WORKBENCH's neighbour
// DINAH_HOME and this one never does. The walk skips the .dinah of that one
// directory, leaving the real user base to the fallback below, which reads
// the relocated value. Pass an empty nativeHome to run the walk unbounded.
//
// The second return value is the workbench.md files the walk found and did
// not claim, each one a directory holding somebody else's document rather
// than a Dinah workbench, in the order the walk met them. It is nil on the
// override branch, since --workbench and DINAH_WORKBENCH test only file
// presence and never run the recognition test (see the card's decisions).
//
// Discover is DiscoverSource with no override rung named and no configured
// default to fall back to, kept at its own four-argument signature so every
// existing call site and test is untouched.
func Discover(start, override, home, nativeHome string) (string, []string, error) {
	root, _, passed, err := DiscoverSource(start, override, "", home, nativeHome, "")
	return root, passed, err
}

// DiscoverSource is Discover with two things added: the rung that answered,
// named alongside the root, and a configured default consulted as the last
// rung before a refusal.
//
// override is the value --workbench or DINAH_WORKBENCH already resolved to,
// and overrideSource names which of the two produced it (SourceFlag or
// SourceEnvironment); it is trusted without further scrutiny, the same plain
// Exists stat the override branch has always run, because an explicit
// pointer names an exact directory rather than a directory the climb merely
// stumbled onto. configured is the workbench setting's stored value, tried
// only once the walk has resolved to nothing at all: a sole workbench or an
// ambiguous base each still decide the search on their own, and the
// configured default never breaks a tie an ambiguous base already raised.
//
// A configured path that no longer carries a workbench.md refuses by its own
// name, dinah.no-configured-workbench, naming the path, rather than falling
// through to dinah.no-workbench-found: falling through would silently run
// the command against no workbench at all, or against whatever the caller's
// own working directory happens to be.
func DiscoverSource(start, override, overrideSource, home, nativeHome, configured string) (string, string, []string, error) {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", "", nil, err
		}
		if !Exists(filepath.Join(abs, WorkbenchAnchor)) {
			return "", "", nil, contract.Refuse(contract.NoWorkbench, abs)
		}
		return abs, overrideSource, nil, nil
	}
	search, err := walk(start, home, nativeHome)
	if err != nil {
		return "", "", nil, err
	}
	if search.sole != "" {
		return search.sole, SourceSearch, search.passed, nil
	}
	if search.base != "" {
		extra := map[string]string{"base": search.base}
		return "", "", nil, contract.RefuseWith(contract.AmbiguousWorkbench, describeCandidates(search.candidates), extra)
	}
	if configured != "" {
		abs, err := filepath.Abs(configured)
		if err != nil {
			return "", "", nil, err
		}
		if !Exists(filepath.Join(abs, WorkbenchAnchor)) {
			return "", "", nil, contract.Refuse(contract.NoConfiguredWorkbench, abs)
		}
		return abs, SourceConfig, search.passed, nil
	}
	return "", "", nil, contract.RefuseWith(contract.NoWorkbenchFound, start, map[string]string{"home": search.userBase})
}

// Reachable reports what Discover's walk finds right now, without turning
// ambiguity or emptiness into a refusal: the sole workbench Discover would
// resolve to, the candidates of the first ambiguous base it would meet, or no
// rows at all. It shares that walk rather than repeating it, so it stops
// exactly where Discover stops and inherits any later change to how far the
// search climbs.
//
// The one error it returns is the one an explicit override raises. A
// --workbench naming a directory that holds no workbench is a caller's
// mistake, and a listing that softened it into an empty result would hide the
// typo.
func Reachable(start, override, home, nativeHome string) ([]Candidate, error) {
	if override != "" {
		root, _, err := Discover(start, override, home, nativeHome)
		if err != nil {
			return nil, err
		}
		return []Candidate{describe(root)}, nil
	}
	search, err := walk(start, home, nativeHome)
	if err != nil {
		return nil, err
	}
	if search.sole != "" {
		return []Candidate{describe(search.sole)}, nil
	}
	return describeAll(search.candidates), nil
}

// search is what one discovery walk found: the workbench it resolved to, or
// else the first ambiguous base it passed with that base's candidates. The
// user base is carried out too, because the refusal raised over an exhausted
// search names where the fallback looked.
type search struct {
	// sole is the one workbench the walk resolved to, empty when it resolved
	// to none.
	sole string
	// base is the first ambiguous base the walk met, empty when it met none.
	base string
	// candidates are the workbench directories that base holds.
	candidates []string
	// userBase is the fallback base, empty when no home was given.
	userBase string
	// passed is the workbench.md files met and not claimed along the way,
	// accumulated across every rung of the climb and the fallback base, in
	// the order the walk met them.
	passed []string
}

// walk runs the ancestor search and the fallback to the user base, and reports
// what it found without deciding what to do about it. It is the one
// implementation of how far discovery reaches, shared by the refusal Discover
// raises and the listing Reachable returns.
//
// The ancestor half of the search examines the .dinah of every directory it
// climbs through except the machine's native home, whose .dinah belongs to
// the fallback half alone. Relocating the user base therefore relocates it
// for a working directory nested under the real home too, which is what a
// caller who set the variable asked for.
//
// A climb that reaches the native home consults the user base there, at the
// rung where the walk used to read that directory's own .dinah, rather than
// after the climb. That keeps the precedence the user base has always had
// over the few directories above a person's home, so the moved .dinah check
// changes which base the search reads without changing which base wins. A
// climb that never reaches the native home runs the fallback after the walk,
// as before.
func walk(start, home, nativeHome string) (search, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return search{}, err
	}
	boundary := ""
	if nativeHome != "" {
		boundary = filepath.Clean(nativeHome)
	}
	// The first ambiguity the search meets, closest to the starting
	// directory, which is the one a reader is likeliest to have meant.
	result := search{}
	consulted := false
	for {
		atNativeHome := samePath(dir, boundary)
		found, ambiguous, passed, err := benchIn(dir, atNativeHome)
		if err != nil {
			return search{}, err
		}
		result.passed = append(result.passed, passed...)
		if found != "" {
			result.sole = found
			return result, nil
		}
		if len(ambiguous) > 0 && result.base == "" {
			result.base, result.candidates = filepath.Join(dir, UserBaseName), ambiguous
		}
		if atNativeHome && !consulted {
			consulted = true
			resolved, err := result.fallbackTo(home)
			if err != nil {
				return search{}, err
			}
			if resolved {
				return result, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if !consulted {
		if _, err := result.fallbackTo(home); err != nil {
			return search{}, err
		}
	}
	return result, nil
}

// fallbackTo consults the user base under home and reports whether it
// resolved the search. An ambiguous base is remembered the way the walk
// remembers one, and a caller that gave no home has no fallback to run.
func (s *search) fallbackTo(home string) (bool, error) {
	if home == "" {
		return false, nil
	}
	s.userBase = filepath.Join(home, UserBaseName)
	found, ambiguous, passed, err := soleBench(s.userBase)
	if err != nil {
		return false, err
	}
	s.passed = append(s.passed, passed...)
	if found != "" {
		s.sole = found
		return true, nil
	}
	if len(ambiguous) > 0 && s.base == "" {
		s.base, s.candidates = s.userBase, ambiguous
	}
	return false, nil
}

// benchIn reports the bench a single directory offers, either as the bench
// itself or as the sole bench of a .dinah directory inside it. The second
// value carries the candidates when that .dinah holds several, which is what
// tells a caller apart from a base holding nothing at all. The third value
// carries the workbench.md files met at this rung and not claimed. The error
// is a workbench.md that exists and could not be read.
//
// skipBase drops the .dinah half, and the walk sets it at the one directory
// that is the machine's native home. The anchor half still runs there, so a
// repository checked out at the home directory is found exactly as before.
func benchIn(dir string, skipBase bool) (found string, ambiguous, passed []string, err error) {
	anchorPath := filepath.Join(dir, WorkbenchAnchor)
	state, err := readAnchor(anchorPath)
	if err != nil {
		return "", nil, nil, contract.Refuse(contract.UnreadableBench, anchorPath)
	}
	if state == anchorOurs {
		return dir, nil, nil, nil
	}
	if state == anchorForeign {
		passed = append(passed, anchorPath)
	}
	if skipBase {
		return "", nil, passed, nil
	}
	baseFound, baseAmbiguous, basePassed, err := soleBench(filepath.Join(dir, UserBaseName))
	if err != nil {
		return "", nil, nil, err
	}
	return baseFound, baseAmbiguous, append(passed, basePassed...), nil
}

// samePath reports whether two directory paths name the same directory. It
// asks the filesystem, the way sameDirs in cmd/dinah's tests does, because one
// directory answers to several spellings: Windows hands out the short 8.3 form
// of any user name holding a space and compares paths without regard to case,
// and macOS mounts a case-insensitive volume by default. Comparing the two
// strings misses every one of those spellings, and a boundary that misses
// reads the real user base without saying so.
//
// An empty path names no directory and matches nothing. Where the filesystem
// cannot answer, the two paths are reported as the same directory, so that a
// boundary the tool cannot check still bounds the walk; the other direction
// silently resumes reading the home it was asked to leave alone. A boundary
// that is not there at all is the one exception, since a directory that does
// not exist holds no user base for the walk to reach.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	boundary, err := os.Stat(b)
	if err != nil {
		return !errors.Is(err, os.ErrNotExist)
	}
	here, err := os.Stat(a)
	if err != nil {
		return true
	}
	return os.SameFile(here, boundary)
}

// soleBench returns the one bench a base directory holds. A base holding
// several is ambiguous, so it returns no bench and the candidates instead,
// and the walk continues rather than picking one. The third value carries
// the workbench.md files met and not claimed, each one a container id
// directory holding somebody else's document. The error is a workbench.md
// that exists and could not be read.
func soleBench(base string) (found string, ambiguous, passed []string, err error) {
	var candidates []string
	for _, id := range ListIDs(base) {
		candidate := filepath.Join(base, id)
		anchorPath := filepath.Join(candidate, WorkbenchAnchor)
		state, rerr := readAnchor(anchorPath)
		if rerr != nil {
			return "", nil, nil, contract.Refuse(contract.UnreadableBench, anchorPath)
		}
		if state == anchorForeign {
			passed = append(passed, anchorPath)
			continue
		}
		if state != anchorOurs {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 1 {
		return candidates[0], nil, passed, nil
	}
	if len(candidates) > 1 {
		return "", candidates, passed, nil
	}
	return "", nil, passed, nil
}

// anchorState is what a workbench.md holds at one rung of the search, without
// validating what it holds.
type anchorState int

const (
	// anchorAbsent means no file sits there at all.
	anchorAbsent anchorState = iota
	// anchorForeign means a file sits there and carries none of the keys
	// that claim it as a Dinah workbench.
	anchorForeign
	// anchorOurs means a file sits there and carries at least one of those
	// keys, whatever else it does or does not carry.
	anchorOurs
)

// readAnchorContent is ReadText by default; a test overrides it to force the
// unreadable-file path without depending on how a permission bit behaves on
// the platform running the test, mirroring the reason Hooks.BeforeOrdinalStamp
// exists in this same file.
var readAnchorContent = ReadText

// readAnchor reports what a workbench.md holds, without validating it. The
// stat runs unconditionally, matching what every rung of the climb already
// paid; the read runs only when the stat found a file there, which stays
// rare. A non-nil error means the file exists and could not be read, a
// different question from whether it is recognized, and the caller checks
// the error first.
func readAnchor(path string) (anchorState, error) {
	if !Exists(path) {
		return anchorAbsent, nil
	}
	text, err := readAnchorContent(path)
	if err != nil {
		return anchorAbsent, err
	}
	fm, _ := ParseAnchor(text)
	if fm.Recognized() {
		return anchorOurs, nil
	}
	return anchorForeign, nil
}

// AnchorRecognized reports whether the workbench.md at path carries Dinah's
// claim to its directory (Frontmatter.Recognized), the same test benchIn and
// soleBench apply at every rung of the discovery walk. It answers false for
// a path with nothing there and false for a file that exists but is
// somebody else's document; a non-nil error means a file exists and could
// not be read, and the caller decides how to refuse over that.
func AnchorRecognized(path string) (bool, error) {
	state, err := readAnchor(path)
	if err != nil {
		return false, err
	}
	return state == anchorOurs, nil
}

// Candidate is one workbench a listing reports: enough of its identity to
// recognise it, and the path that selects it. The members stop where reading
// the anchor stops, so a listing never opens a workbench to describe it.
type Candidate struct {
	// Title is the workbench's title, empty when its anchor declares none or
	// will not read.
	Title string `json:"title"`
	// Slug is the short name a card reference carries ahead of its number,
	// empty on a workbench written before the field or one whose anchor will
	// not read. The key is absent rather than an empty string standing in
	// for the workbench having none, matching the convention StateView.Slug
	// already carries.
	Slug string `json:"slug,omitempty"`
	// Path is the workbench directory, which is what --workbench takes.
	Path string `json:"path"`
}

// describe reads one workbench's identity off its anchor without opening it.
// An anchor that will not read leaves the title and the slug empty rather than
// failing, so one unreadable workbench does not hide the others.
func describe(root string) Candidate {
	found := Candidate{Path: root}
	text, err := ReadText(filepath.Join(root, WorkbenchAnchor))
	if err != nil {
		return found
	}
	fm, _ := ParseAnchor(text)
	found.Title = fm.Value("title")
	found.Slug = fm.Value("slug")
	return found
}

// describeAll describes each of a base's candidates in the order the base
// offered them. The slice is never nil, so an empty listing marshals to an
// empty array rather than to null.
func describeAll(candidates []string) []Candidate {
	described := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		described = append(described, describe(candidate))
	}
	return described
}

// describeCandidates names each workbench of an ambiguous base by its title
// and the directory it sits in, joined into the list the refusal prints. A
// candidate that declares no title is named by its path alone.
//
// The refusal and the listing read the same descriptions in the same order,
// so the two surfaces cannot disagree about which workbenches an ambiguity
// holds.
func describeCandidates(candidates []string) string {
	described := make([]string, 0, len(candidates))
	for _, candidate := range describeAll(candidates) {
		if candidate.Title == "" {
			described = append(described, candidate.Path)
			continue
		}
		described = append(described, candidate.Title+" ("+candidate.Path+")")
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
	seenSlug := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, contract.RefuseWith(contract.Malformed, "states", anchor)
		}
		seen[id] = true
		stateDir := filepath.Join(root, StatesDir, id)
		if !Exists(stateDir) {
			b.StrandedStates = append(b.StrandedStates, id)
			continue
		}
		state, err := readState(root, id, len(b.States))
		if err != nil {
			return nil, err
		}
		if err := admitSlug(state, major, seenSlug, map[string]string{"path": b.StateAnchorPath(id)}); err != nil {
			return nil, err
		}
		b.States = append(b.States, state)
	}
	return b, nil
}

// admitSlug applies the slug rules to one state as the bench is opened, given
// the major the workbench declares, the slugs its earlier states took, and the
// anchor the state was read from.
//
// Every rule here turns on the declared major, an absent slug and a stored one
// alike, because CORE-STATE-10 arrives with SlugMandatoryMajor and a workbench
// claiming an earlier revision is not evaluated against it. Below that major
// the reader carries a malformed or duplicated slug through, and `dinah check`
// names it for a person to repair. Refusing it here would take the workbench
// away from the command that reports the defect and from the migration that
// repairs the states around it, since both have to open the workbench before
// they can do anything at all, and one hand-typed slug would then close the
// workbench with no way back in.
//
// Each refusal names the state and the file it was raised over, so a reader
// past that major knows which anchor to open.
func admitSlug(state *State, major int, seen map[string]bool, anchor map[string]string) error {
	if major < SlugMandatoryMajor {
		return nil
	}
	if state.Slug == "" {
		return contract.RefuseWith(contract.Malformed, "slug of state "+state.ID, anchor)
	}
	if !ValidStateSlug(state.Slug) || seen[state.Slug] {
		return contract.RefuseWith(contract.Malformed, "slug "+state.Slug+" of state "+state.ID, anchor)
	}
	seen[state.Slug] = true
	return nil
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
		Slug:          fm.Value("slug"),
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

// StateByRef returns the state a reference names, accepting the identifier,
// then the slug, then the title, the last two compared without regard to ASCII
// case, which is what makes a state nameable on a command line at all.
//
// The slug is tried ahead of the title so a reference matching one state's
// slug and another state's title resolves to the state whose slug it is. No
// two states of an open bench carry the same slug, so the slug pass itself is
// never ambiguous; the title pass keeps the first-match-in-order behaviour it
// has always had.
func (b *Bench) StateByRef(ref string) *State {
	if state := b.State(ref); state != nil {
		return state
	}
	want := asciiLower(strings.TrimSpace(ref))
	for _, state := range b.States {
		if state.Slug != "" && asciiLower(state.Slug) == want {
			return state
		}
	}
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

// NativeHome returns the machine's own home directory and ignores DINAH_HOME,
// which is what separates it from Home. Discovery bounds its ancestor walk
// here, so that a caller who relocated the user base is not handed the real
// one by a working directory that happens to sit under it.
//
// A machine whose home will not resolve reports no directory at all, which is
// the value the walk reads as unbounded, so the search reaches as far as it
// always did. Home answers "." in that case and this one does not, because the
// boundary is compared by asking the filesystem which directory a path names,
// and "." names the directory the tool was run from.
func NativeHome() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return dir
}
