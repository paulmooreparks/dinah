package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/bench/compattest"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// compatDir is where the compatibility fixtures and the population sequence
// live, named from this package.
const compatDir = "../../internal/bench/testdata/compat"

// populateName is the file holding the population sequence. It is the only
// source of that sequence: the capture procedure replays it through a built
// binary and the shape comparison replays it through this head, so the two
// trees differ only where the sequence cannot reach.
const populateName = "populate.txt"

// populateInputs are the files a populate.txt line names as an argument. They
// sit beside populate.txt and are copied next to the workbench before a
// replay, because a file argument names a file rather than a path.
var populateInputs = []string{
	"definition.json",
	"payload-one.txt",
	"payload-two.txt",
	"payload-three.txt",
}

// identifier matches a 12-hex identifier, which a path template replaces with
// a placeholder so a fixture and a fresh workbench can be compared at all.
var identifier = regexp.MustCompile(`^[0-9a-f]{12}$`)

// wantedTemplates is every path template the population sequence writes,
// relative to the anchor directory, with identifiers and payload filenames
// replaced.
var wantedTemplates = []string{
	".gitignore",
	"workbench.md",
	"journal.ndjson",
	"states/<id>/state.md",
	"archive/states/<id>/state.md",
	"cards/<id>/card.md",
	"cards/<id>/journal.ndjson",
	"cards/<id>/comments/<id>/comment.md",
	"cards/<id>/attachments/<id>/attachment.md",
	"cards/<id>/attachments/<id>/payload/<file>",
	"cards/<id>/archive/comments/<id>/comment.md",
	"cards/<id>/archive/attachments/<id>/attachment.md",
	"cards/<id>/archive/attachments/<id>/payload/<file>",
	"archive/cards/<id>/card.md",
	"archive/cards/<id>/journal.ndjson",
	"workstreams/<id>/workstream.md",
	"workstreams/<id>/journal.ndjson",
	"archive/workstreams/<id>/workstream.md",
	"archive/workstreams/<id>/journal.ndjson",
}

// wantedKeys is the union of top-level frontmatter keys the sequence writes,
// per file kind. Nested members sit outside the comparison, meaning the
// entries under the anchor's states sequence and under a card's links and
// workstreams, and the head writes none of them as mappings today.
var wantedKeys = map[string][]string{
	"workbench.md":                                      {"format", "profile", "title", "slug", "operator", "states", "levels"},
	"states/<id>/state.md":                              {"title", "slug", "kind", "operator_owned", "wip_limit"},
	"archive/states/<id>/state.md":                      {"title", "slug", "kind", "operator_owned", "wip_limit"},
	"cards/<id>/card.md":                                {"title", "number", "state", "substate", "severity", "priority", "claim_holder", "claim_since", "claim_expires", "block_reason", "block_kind", "block_since", "workstreams"},
	"archive/cards/<id>/card.md":                        {"title", "number", "state", "substate"},
	"cards/<id>/comments/<id>/comment.md":               {"ts", "author", "ordinal"},
	"cards/<id>/archive/comments/<id>/comment.md":       {"ts", "author", "ordinal"},
	"cards/<id>/attachments/<id>/attachment.md":         {"filename", "description", "provenance", "ordinal"},
	"cards/<id>/archive/attachments/<id>/attachment.md": {"filename", "description", "provenance", "ordinal"},
	"workstreams/<id>/workstream.md":                    {"title", "slug", "status", "ordinal"},
	"archive/workstreams/<id>/workstream.md":            {"title", "slug", "status", "ordinal"},
}

// wantedEvents is the union of member names the sequence writes, per journal
// event name. Three members are optional on an event that is otherwise
// sampled: kind on blocked, expires on claimed, and override on moved.
var wantedEvents = map[string][]string{
	contract.EventCreated:            {"ts", "event", "actor", "title", "to", "to_title"},
	contract.EventClaimed:            {"ts", "event", "actor", "expires"},
	contract.EventMoved:              {"ts", "event", "actor", "from", "from_title", "to", "to_title", "override"},
	contract.EventReleased:           {"ts", "event", "actor"},
	contract.EventBlocked:            {"ts", "event", "actor", "reason", "kind"},
	contract.EventUnblocked:          {"ts", "event", "actor"},
	contract.EventExpired:            {"ts", "event", "actor", "expires"},
	contract.EventCommented:          {"ts", "event", "actor", "comment"},
	contract.EventAttached:           {"ts", "event", "actor", "attachment", "filename"},
	contract.EventAttachmentReplaced: {"ts", "event", "actor", "attachment", "filename"},
	contract.EventAttachmentRemoved:  {"ts", "event", "actor", "attachment", "filename", "note"},
	contract.EventAttachmentRenamed:  {"ts", "event", "actor", "attachment", "filename", "from"},
	contract.EventArchived:           {"ts", "event", "actor", "note"},
	contract.EventDeleted:            {"ts", "event", "actor", "title", "note"},
	contract.EventWorkbenchUpdated:   {"ts", "event", "actor", "field", "from", "to"},
	contract.EventWorkstreamUpdated:  {"ts", "event", "actor", "field", "from", "to"},
	contract.EventCardUpdated:        {"ts", "event", "actor", "field", "from", "to"},
	contract.EventWorkstreamJoined:   {"ts", "event", "actor", "workstream"},
	contract.EventWorkstreamLeft:     {"ts", "event", "actor", "workstream"},
}

// unwrittenEvents are the event names internal/contract declares that this
// tool never writes, each with the reason it never does. The coverage alarm
// holds the population sequence to the difference between this table and the
// declared set, so adding an event constant turns the build red in the commit
// that adds it.
var unwrittenEvents = map[string]string{
	contract.EventRestored:         "archive has no inverse verb in the command surface, so nothing restores an entity",
	contract.EventManualCorrection: "a person writes this line into a journal by hand and no command produces it",
}

// shape is what a fixture and a freshly populated workbench are compared on.
// None of the three sets reads a value the tool generates.
type shape struct {
	// templates are the path templates, relative to the anchor directory.
	templates map[string]bool
	// keys are the top-level frontmatter keys, per path template.
	keys map[string]map[string]bool
	// members are the journal member names, per event name.
	members map[string]map[string]bool
}

// TestReplayingThePopulationSequenceReachesEveryShapeItNames asserts that the
// sequence still writes everything the spec's three sets name. A line that
// stops working, or a command that stops writing a key, is caught here rather
// than silently narrowing what every later capture samples.
func TestReplayingThePopulationSequenceReachesEveryShapeItNames(t *testing.T) {
	fresh := readShape(t, replayPopulation(t))
	for _, template := range wantedTemplates {
		if !fresh.templates[template] {
			t.Errorf("replaying %s wrote no %s", populateName, template)
		}
	}
	for template, keys := range wantedKeys {
		for _, key := range keys {
			if !fresh.keys[template][key] {
				t.Errorf("replaying %s wrote no %s key in %s", populateName, key, template)
			}
		}
	}
	for event, members := range wantedEvents {
		if fresh.members[event] == nil {
			t.Errorf("replaying %s wrote no %s event", populateName, event)
			continue
		}
		for _, member := range members {
			if !fresh.members[event][member] {
				t.Errorf("replaying %s wrote no %s member on the %s event", populateName, member, event)
			}
		}
	}
}

// TestTheSampleFixtureCarriesEveryShapeThisBuildWrites is the sample alarm. It
// creates a workbench with the build under test, replays the same sequence
// against it, and asserts the sample fixture contains that tree's shape. The
// containment runs in one direction on purpose, so a fixture carrying a key an
// older build wrote and this one no longer does still passes.
//
// The comparison is against the one fixture the manifest marks rather than
// against the union of every fixture declaring the revision, because a union
// lets a second fixture cover the first one's gaps.
//
// What this test does not prove (spec section 6.5): the containment shows the
// sample fixture holds every shape this build's own replay writes, and that
// is completeness within the sequence populate.txt drives, nothing more. A
// shape the sequence never exercises is absent from both sides and this test
// stays silent about it. It says nothing about where the sample fixture's
// bytes came from; provenance rests on the capture commit named in the
// fixture's manifest row and on a reader replaying it, not on any assertion
// here. wantedKeys above tells a reader which top-level keys the sequence
// samples, and it does not say that Instantiate and writeStateFromMember copy
// an unrecognised definition or state member straight into the anchor they
// write, so the workbench.md and state.md key sets are open in a way this
// comparison cannot see (OQ-7).
func TestTheSampleFixtureCarriesEveryShapeThisBuildWrites(t *testing.T) {
	sample := readShape(t, sampleFixture(t))
	fresh := readShape(t, replayPopulation(t))
	for template := range fresh.templates {
		if !sample.templates[template] {
			t.Errorf("the sample fixture carries no %s, which this build writes", template)
		}
	}
	for template, keys := range fresh.keys {
		for key := range keys {
			if !sample.keys[template][key] {
				t.Errorf("the sample fixture's %s carries no %s key, which this build writes", template, key)
			}
		}
	}
	for event, members := range fresh.members {
		for member := range members {
			if !sample.members[event][member] {
				t.Errorf("the sample fixture carries no %s member on the %s event, which this build writes", member, event)
			}
		}
	}
}

// TestTheSampleFixtureCarriesEveryJournalEventTheContractDeclares is the
// coverage alarm. The sample alarm above cannot notice a new event, because a
// shape the sequence never exercises is absent from the fresh tree as well as
// from the fixture, so both sides of the containment stay silent.
func TestTheSampleFixtureCarriesEveryJournalEventTheContractDeclares(t *testing.T) {
	sample := readShape(t, sampleFixture(t))
	for _, event := range declaredEvents(t) {
		if sample.members[event] != nil {
			continue
		}
		if _, exempt := unwrittenEvents[event]; exempt {
			continue
		}
		t.Errorf("internal/contract declares the %s event, the sample fixture carries no line of it, and unwrittenEvents says nothing about why. Extend %s until the event lands in a capture, or add it to unwrittenEvents with the reason nothing writes it", event, populateName)
	}
}

// cardJournalTemplates are the two path templates a card's own journal takes,
// live and archived. The query reads a card's journal and no other, so these
// are the journals whose event names have to be nameable.
var cardJournalTemplates = map[string]bool{
	"cards/<id>/journal.ndjson":         true,
	"archive/cards/<id>/journal.ndjson": true,
}

// TestEveryEventACardJournalCarriesIsOneAQueryCanName pairs the two lists that
// have to agree: the event names a card's journal carries, and contract.Events,
// which query hands the event field as its closed vocabulary. An event on a
// card journal that the vocabulary omits cannot be asked for, and the refusal a
// person meets names it as an unknown value rather than as an omission.
//
// The pairing runs on the sample fixture rather than on a hand-written list,
// and the coverage alarm above carries the other half. That alarm refuses a
// declared event the fixture carries no line of, so a new event has to reach
// the fixture, and once it reaches the fixture this test decides whether it
// reached the vocabulary too. What the two together catch is an event added to
// the tool and never exercised by the population sequence.
//
// The pair does not close every route, and the routes it leaves open are worth
// naming so the next reader does not read a closure that is not there. An
// event listed in unwrittenEvents is exempt from the alarm, which is a human
// step carrying a written reason rather than a silent gap. An event that lands
// on a card journal and on another journal as well can satisfy the alarm from
// the other one, because readShape records members per event name and not per
// path template, so a capture on a workstream or workbench journal is enough
// and this test, which walks cardJournalTemplates alone, never sees it;
// workstream_updated is the live illustration of an event the alarm accepts
// from a journal this test does not walk. A name in contract.Events that
// nothing writes to a card journal is caught in neither direction, because no
// test asserts the containment that way, and EventRestored and
// EventManualCorrection already sit in that position. The pairing therefore
// holds for every event whose card-journal path the population sequence
// exercises, and it does not reach every event the tool can declare.
func TestEveryEventACardJournalCarriesIsOneAQueryCanName(t *testing.T) {
	root := sampleFixture(t)
	nameable := map[string]bool{}
	for _, event := range contract.Events {
		nameable[event] = true
	}
	seen := 0
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != bench.JournalName {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !cardJournalTemplates[pathTemplate(filepath.ToSlash(relative))] {
			return nil
		}
		events := map[string]map[string]bool{}
		readJournalMembers(t, path, events)
		for event := range events {
			seen++
			if nameable[event] {
				continue
			}
			t.Errorf("the sample fixture's %s carries the %s event and contract.Events does not name it, so a card carries history dinah query 'event:%s' refuses as an unknown value. Add the name to contract.Events, or say in that list's doc comment which journal other than a card's it lands on", relative, event, event)
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if seen == 0 {
		t.Fatalf("the sample fixture carries no card journal event at all, so this test proved nothing about %s", root)
	}
}

// declaredEvents reads the event names internal/contract declares out of the
// source rather than out of a list kept here, so a constant added there reaches
// this test without anybody remembering to add it.
func declaredEvents(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "contract", "contract.go"))
	if err != nil {
		t.Fatalf("read internal/contract/contract.go: %v", err)
	}
	var names []string
	for _, match := range eventConstant.FindAllStringSubmatch(string(data), -1) {
		names = append(names, match[1])
	}
	if len(names) == 0 {
		t.Fatal("internal/contract declares no event-name constant this test can read, so its own pattern has gone stale")
	}
	return names
}

// eventConstant matches one event-name constant declaration in
// internal/contract, capturing the name the journal actually carries.
var eventConstant = regexp.MustCompile(`(?m)^\tEvent[A-Za-z]+\s+= "([a-z_]+)"`)

// sampleFixture returns the directory of the fixture the manifest marks as the
// sample for the revision this build stamps.
func sampleFixture(t *testing.T) string {
	t.Helper()
	manifest, err := compattest.ReadFixtureManifest(compatDir)
	if err != nil {
		t.Fatalf("read the fixture manifest: %v", err)
	}
	var marked []string
	for _, row := range manifest.Fixtures {
		if !row.Sample {
			continue
		}
		if anchorProfile(t, filepath.Join(compatDir, row.Directory)) != bench.ProfileVersion {
			continue
		}
		marked = append(marked, row.Directory)
	}
	if len(marked) != 1 {
		t.Fatalf("%d fixtures declaring %s are marked sample: true (%v), wanted exactly one", len(marked), bench.ProfileVersion, marked)
	}
	return filepath.Join(compatDir, marked[0])
}

// anchorProfile reads the profile string an anchor declares. Reads through
// bench.ReadText and bench.ParseAnchor directly rather than through
// compattest, because compattest deliberately imports nothing from
// internal/bench (see its doc comment): internal/bench's own compat tests
// import compattest too, and a bench-facing helper living there would close
// an import cycle back through that test binary.
func anchorProfile(t *testing.T, root string) string {
	t.Helper()
	text, err := bench.ReadText(filepath.Join(root, bench.WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the anchor at %s: %v", root, err)
	}
	fm, _ := bench.ParseAnchor(text)
	return fm.Value("profile")
}

// replayPopulation creates a workbench with the build under test, replays the
// population sequence against it, and returns the anchor directory of the
// result.
func replayPopulation(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "sam")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range populateInputs {
		data, err := os.ReadFile(filepath.Join(compatDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	sequence, err := os.ReadFile(filepath.Join(compatDir, populateName))
	if err != nil {
		t.Fatalf("read %s: %v", populateName, err)
	}
	for number, line := range strings.Split(strings.ReplaceAll(string(sequence), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if pause, ok := strings.CutPrefix(line, "wait "); ok {
			waited, err := verb.ParseDuration(strings.TrimSpace(pause))
			if err != nil {
				t.Fatalf("%s line %d: %v", populateName, number+1, err)
			}
			time.Sleep(waited)
			continue
		}
		argv, err := tokenize(line)
		if err != nil {
			t.Fatalf("%s line %d: %v", populateName, number+1, err)
		}
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("%s line %d (%s): exit %d, %s", populateName, number+1, line, got.code, got.errw)
		}
	}
	return benchDir(t, root)
}

// tokenize splits a populate.txt line into arguments under shell quoting, so a
// title or a block reason carrying spaces travels as one argument.
func tokenize(line string) ([]string, error) {
	var argv []string
	var current strings.Builder
	open := false
	quote := rune(0)
	for _, r := range line {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			current.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
			open = true
		case r == ' ' || r == '\t':
			if open {
				argv = append(argv, current.String())
				current.Reset()
				open = false
			}
		default:
			current.WriteRune(r)
			open = true
		}
	}
	if quote != 0 {
		return nil, errUnbalancedQuote
	}
	if open {
		argv = append(argv, current.String())
	}
	return argv, nil
}

// errUnbalancedQuote reports a populate.txt line whose quoting does not close.
var errUnbalancedQuote = errors.New("a quoted argument does not close")

// readShape reads the three sets a tree is compared on.
func readShape(t *testing.T, root string) shape {
	t.Helper()
	found := shape{
		templates: map[string]bool{},
		keys:      map[string]map[string]bool{},
		members:   map[string]map[string]bool{},
	}
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		template := pathTemplate(filepath.ToSlash(relative))
		found.templates[template] = true
		if strings.HasSuffix(path, ".md") {
			readFrontmatterKeys(t, path, template, found.keys)
		}
		if entry.Name() == bench.JournalName {
			readJournalMembers(t, path, found.members)
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return found
}

// pathTemplate replaces every identifier and every attachment payload filename
// in a path, leaving the shape of the path and none of its instance.
func pathTemplate(relative string) string {
	parts := strings.Split(relative, "/")
	for i, part := range parts {
		switch {
		case identifier.MatchString(part):
			parts[i] = "<id>"
		case i > 0 && parts[i-1] == bench.PayloadDir:
			parts[i] = "<file>"
		}
	}
	return strings.Join(parts, "/")
}

// readFrontmatterKeys adds one file's top-level frontmatter keys to the union
// its file kind carries.
func readFrontmatterKeys(t *testing.T, path, template string, into map[string]map[string]bool) {
	t.Helper()
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	fm, _ := bench.ParseAnchor(text)
	if into[template] == nil {
		into[template] = map[string]bool{}
	}
	for _, key := range fm.Keys() {
		into[template][key] = true
	}
}

// readJournalMembers adds one journal's member names to the union each event
// name carries. The lines are decoded as raw objects rather than as the head's
// own event type, because a member the head does not model is exactly what this
// comparison exists to notice.
func readJournalMembers(t *testing.T, path string, into map[string]map[string]bool) {
	t.Helper()
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		object := map[string]json.RawMessage{}
		if err := json.Unmarshal([]byte(line), &object); err != nil {
			t.Fatalf("parse a journal line in %s: %v", path, err)
		}
		var event string
		if err := json.Unmarshal(object["event"], &event); err != nil {
			t.Fatalf("a journal line in %s carries no event name: %v", path, err)
		}
		if into[event] == nil {
			into[event] = map[string]bool{}
		}
		for member := range object {
			into[event][member] = true
		}
	}
}

// TestAWorkbenchDeclaringEachRevisionOpensOrIsRefused drives the window
// through the head, one declared string at a time, which is what a person meets
// when their own workbench declares one of them.
func TestAWorkbenchDeclaringEachRevisionOpensOrIsRefused(t *testing.T) {
	opens := []string{"dinah-core/0.1", "dinah-core/0.2", "dinah-core/0.3", "dinah-core/0.4", "dinah-core/0.5", "dinah-core/0.6", "dinah-core/1.0"}
	refuses := []string{"dinah-core/0.0", "dinah-core/0.7", "dinah-core/1.1", "dinah-core/2.0", "dinah-core/3.0", "dinah-core/4.0", "dinah-core/9.9"}
	for _, declared := range opens {
		root := newBench(t)
		editAnchor(t, root, "profile: "+bench.ProfileVersion, "profile: "+declared)
		if got := runCLI(t, root, "status"); got.code != 0 {
			t.Errorf("a workbench declaring %s was refused: %d %s", declared, got.code, got.errw)
		}
	}
	for _, declared := range refuses {
		root := newBench(t)
		editAnchor(t, root, "profile: "+bench.ProfileVersion, "profile: "+declared)
		got := runCLI(t, root, "status")
		if got.code != 2 {
			t.Errorf("a workbench declaring %s exited %d, wanted 2", declared, got.code)
		}
		leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
		if leading != contract.UnsupportedVer {
			t.Errorf("a workbench declaring %s was refused %q, wanted %s", declared, got.errw, contract.UnsupportedVer)
		}
	}
}

// TestTheUnsupportedVersionRefusalNamesTheWindow asserts the clause a person
// reads when their revision falls outside the window, and that the same refusal
// name raised for a storage format carries no window and reads as it did
// before.
func TestTheUnsupportedVersionRefusalNamesTheWindow(t *testing.T) {
	root := newBench(t)
	editAnchor(t, root, "profile: "+bench.ProfileVersion, "profile: dinah-core/9.9")
	got := runCLI(t, root, "status")
	wanted := "; this build reads dinah-core 0.1 through dinah-core 0.6"
	if !strings.Contains(got.errw, wanted) {
		t.Errorf("the refusal reads %q, wanted it to carry %q", got.errw, wanted)
	}

	other := newBench(t)
	editAnchor(t, other, "format: 1", "format: 99")
	storage := runCLI(t, other, "status")
	if storage.code != 2 {
		t.Fatalf("a workbench declaring a newer storage format exited %d, wanted 2", storage.code)
	}
	if strings.Contains(storage.errw, "this build reads") {
		t.Errorf("the storage-format refusal gained the window clause: %q", storage.errw)
	}
}

// TestAMalformedProfileKeepsItsTwoSentences asserts that the two admission
// sites still refuse an unparseable string differently: Open names the file and
// the repair, and ReadDefinition, which is handed bytes and has no path, names
// neither.
func TestAMalformedProfileKeepsItsTwoSentences(t *testing.T) {
	for _, declared := range []string{"dinah/1.0", "dinah-core/1"} {
		root := newBench(t)
		editAnchor(t, root, "profile: "+bench.ProfileVersion, "profile: "+declared)
		got := runCLI(t, root, "status")
		if got.code != 2 {
			t.Errorf("a workbench declaring %s exited %d, wanted 2", declared, got.code)
		}
		if !strings.Contains(got.errw, contract.Malformed) {
			t.Errorf("a workbench declaring %s was refused %q, wanted %s", declared, got.errw, contract.Malformed)
		}
		if !strings.Contains(got.errw, bench.WorkbenchAnchor) || !strings.Contains(got.errw, "dinah check") {
			t.Errorf("the refusal through Open lost its path or its repair clause: %q", got.errw)
		}
	}

	root := newBench(t)
	template := filepath.Join(root, "template.json")
	definition := `{"profile":"dinah/1.0","title":"a template","states":[{"id":"004acda2c28a","title":"Intake","kind":"intake"}]}`
	if err := os.WriteFile(template, []byte(definition), 0o644); err != nil {
		t.Fatalf("write the template: %v", err)
	}
	elsewhere := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := runCLI(t, elsewhere, "init", "--from", template)
	if got.code != 2 {
		t.Fatalf("init from a malformed template exited %d, wanted 2", got.code)
	}
	if !strings.Contains(got.errw, contract.Malformed) {
		t.Errorf("init from a malformed template was refused %q, wanted %s", got.errw, contract.Malformed)
	}
	if strings.Contains(got.errw, "dinah check") {
		t.Errorf("the refusal through ReadDefinition gained a repair clause it has no path for: %q", got.errw)
	}
}

// TestInitFromAnOlderFixtureClonesItAndStampsThisBuildsRevision drives both
// forms dinah init --from accepts, the anchor directory and an exported .json
// file, against a fixture declaring the retired spelling. The clone declares
// this build's own revision, because a workbench names the revision it was
// written against.
func TestInitFromAnOlderFixtureClonesItAndStampsThisBuildsRevision(t *testing.T) {
	source, err := filepath.Abs(filepath.Join(compatDir, "dinah-core-1.0"))
	if err != nil {
		t.Fatalf("resolve the fixture: %v", err)
	}
	exported := filepath.Join(t.TempDir(), "definition.json")
	b, err := bench.Open(source)
	if err != nil {
		t.Fatalf("open the fixture: %v", err)
	}
	data, err := b.Export()
	if err != nil {
		t.Fatalf("export the fixture: %v", err)
	}
	if err := os.WriteFile(exported, data, 0o644); err != nil {
		t.Fatalf("write the exported definition: %v", err)
	}
	for _, from := range []string{source, exported} {
		base := t.TempDir()
		root := filepath.Join(base, "clone")
		t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
		t.Setenv("DINAH_ACTOR", "sam")
		t.Setenv("DINAH_LANG", "")
		t.Setenv("DINAH_FORMAT", "")
		t.Setenv("DINAH_WORKBENCH", "")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if got := runCLI(t, root, "init", "--from", from, "--slug", "clone", "--operator", "sam"); got.code != 0 {
			t.Fatalf("init --from %s: %d %s", from, got.code, got.errw)
		}
		anchor := benchDir(t, root)
		if declared := anchorProfile(t, anchor); declared != bench.ProfileVersion {
			t.Errorf("the clone from %s declares %s, wanted %s", from, declared, bench.ProfileVersion)
		}
		if got := runCLI(t, root, "status"); got.code != 0 {
			t.Errorf("the clone from %s does not open: %d %s", from, got.code, got.errw)
		}
	}
}

// TestThePreSlugFixtureOpensAndReportsItsMissingSlugs asserts that a workbench
// written before state slugs and creation ordinals existed opens under this
// build, lists its states, and has its defects reported rather than repaired,
// with every file left byte-identical afterwards. That last clause is what
// proves no migration ran on open.
func TestThePreSlugFixtureOpensAndReportsItsMissingSlugs(t *testing.T) {
	fixture, err := filepath.Abs(filepath.Join(compatDir, "dinah-core-1.0-pre-slug"))
	if err != nil {
		t.Fatalf("resolve the fixture: %v", err)
	}
	before := fixtureContents(t, fixture)
	root := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(root, "home"))
	t.Setenv("DINAH_ACTOR", "sam")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if got := runCLI(t, root, "--workbench", fixture, "states"); got.code != 0 {
		t.Fatalf("states: %d %s", got.code, got.errw)
	}
	checked := runCLI(t, root, "--workbench", fixture, "check")
	for _, wanted := range []string{"1a2b3c4d5e6f", "2b3c4d5e6f70", "carries no slug"} {
		if !strings.Contains(checked.out, wanted) {
			t.Errorf("check did not report %q: %q", wanted, checked.out)
		}
	}
	after := fixtureContents(t, fixture)
	for path, content := range after {
		if before[path] != content {
			t.Errorf("%s changed under a read", path)
		}
	}
	if len(before) != len(after) {
		t.Errorf("the fixture held %d files before the read and %d after", len(before), len(after))
	}
}

// fixtureContents reads every file of a tree, so a test can assert that a read
// left the tree byte-identical.
func fixtureContents(t *testing.T, root string) map[string]string {
	t.Helper()
	contents := map[string]string{}
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		contents[filepath.ToSlash(relative)] = string(data)
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return contents
}
