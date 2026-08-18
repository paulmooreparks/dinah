package bench

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/testenv"
)

// TestMain redirects this binary's temporary directory outside the
// developer's home before any test runs, so the ancestor walk this
// package's Discover and Reachable tests exercise cannot climb out of its
// own synthetic fixture tree and reach the real workbenches sitting above
// it. See internal/testenv's package comment for what this does and does
// not cover.
func TestMain(m *testing.M) {
	restore := testenv.IsolateTempDir()
	code := m.Run()
	restore()
	os.Exit(code)
}

// benchDefinition is the smallest bench check can be run against.
const benchDefinition = `---
format: 1
profile: dinah-core/1.0
title: Fixture
slug: fx
operator: alka
states:
  - b00000000001
---
Standing text.
`

// stateDefinition is the one state that bench declares.
const stateDefinition = `---
title: Only
slug: only
kind: work
---
State text.
`

// cleanCard is a card carrying no defect, which every case below breaks in
// exactly one way.
const cleanCard = `---
title: A card
number: 1
state: b00000000001
substate: ready
---
Framing.
`

// cleanJournal is that card's history, opened with its created event.
const cleanJournal = `{"ts":"2026-08-17T09:00:00Z","event":"created","actor":"alka","title":"A card","to":"b00000000001","to_title":"Only"}
`

// newFixture writes a clean bench and returns its root.
func newFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, WorkbenchAnchor), benchDefinition)
	write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor), stateDefinition)
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), cleanCard)
	write(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), cleanJournal)
	return root
}

// write puts a file on disk, creating the directories above it.
func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestCheckFindsEachInvariantViolation asserts that check detects each defect
// the format document names, reports it against the file it sits in, and
// finds nothing at all on a clean bench.
func TestCheckFindsEachInvariantViolation(t *testing.T) {
	cases := []struct {
		name    string
		breakIt func(*testing.T, string)
		key     string
	}{
		{
			name:    "a clean workbench",
			breakIt: func(t *testing.T, root string) {},
			key:     "",
		},
		{
			name: "claim fields without substate active",
			breakIt: func(t *testing.T, root string) {
				edit(t, root, "substate: ready", "substate: ready\nclaim_holder: alka\nclaim_since: 2026-08-17T09:00:00Z")
			},
			key: FindingClaimWithoutActive,
		},
		{
			name: "substate active without claim fields",
			breakIt: func(t *testing.T, root string) {
				edit(t, root, "substate: ready", "substate: active")
			},
			key: FindingActiveWithoutClaim,
		},
		{
			name: "a block missing its reason",
			breakIt: func(t *testing.T, root string) {
				edit(t, root, "substate: ready", "substate: blocked")
			},
			key: FindingBlockWithoutReason,
		},
		{
			name: "a card naming a state the workbench does not declare",
			breakIt: func(t *testing.T, root string) {
				edit(t, root, "state: b00000000001", "state: b00000000009")
			},
			key: FindingUnknownState,
		},
		{
			name: "a link whose to resolves to no card",
			breakIt: func(t *testing.T, root string) {
				edit(t, root, "substate: ready", "substate: ready\nlinks:\n  - kind: relates\n    to: d00000000009")
			},
			key: FindingDanglingLink,
		},
		{
			name: "a frontmatter position diverging from the journal",
			breakIt: func(t *testing.T, root string) {
				path := filepath.Join(root, CardsDir, "c00000000001", JournalName)
				line := `{"ts":"2026-08-17T10:00:00Z","event":"moved","actor":"alka","from":"b00000000001","to":"b00000000009"}` + "\n"
				appendText(t, path, line)
			},
			key: FindingPositionDiverges,
		},
		{
			name: "a directory carrying no anchor",
			breakIt: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, CardsDir, "c00000000002"), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			},
			key: FindingMissingAnchor,
		},
		{
			name: "a torn journal tail",
			breakIt: func(t *testing.T, root string) {
				appendText(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), `{"ts":"2026-08-1`)
			},
			key: FindingTornJournal,
		},
		{
			name: "an entity carrying no creation ordinal",
			breakIt: func(t *testing.T, root string) {
				writeComment(t, root, "e00000000001", "2026-08-17T09:01:00Z", 0, "unstamped")
			},
			key: FindingOrdinalMissing,
		},
		{
			name: "two entities of one collection sharing an ordinal",
			breakIt: func(t *testing.T, root string) {
				writeComment(t, root, "e00000000001", "2026-08-17T09:01:00Z", 1, "first")
				writeComment(t, root, "e00000000002", "2026-08-17T09:02:00Z", 1, "second")
			},
			key: FindingOrdinalDuplicate,
		},
		{
			// A deletion leaves a gap, and closing one would renumber every
			// entity after it and move every positional reference already
			// written down, so a gap is not a defect. The empty key asserts
			// that the whole bench reports nothing at all.
			name: "a gap left where an entity was deleted",
			breakIt: func(t *testing.T, root string) {
				writeComment(t, root, "e00000000001", "2026-08-17T09:01:00Z", 1, "first")
				writeComment(t, root, "e00000000003", "2026-08-17T09:03:00Z", 3, "third")
			},
			key: "",
		},
		{
			name: "a card carrying no creation ordinal",
			breakIt: func(t *testing.T, root string) {
				edit(t, root, "number: 1", "number: 0")
			},
			key: FindingOrdinalMissing,
		},
		{
			name: "a workbench carrying no slug",
			breakIt: func(t *testing.T, root string) {
				editWorkbench(t, root, "slug: fx\n", "")
			},
			key: FindingWorkbenchSlugMissing,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			c.breakIt(t, root)
			opened, err := Open(root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			findings, err := opened.Check()
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if c.key == "" {
				if len(findings) != 0 {
					t.Fatalf("a clean workbench should report nothing, got %+v", findings)
				}
				return
			}
			for _, finding := range findings {
				if finding.Key != c.key {
					continue
				}
				if finding.Path == "" {
					t.Error("a finding should name the file it sits in")
				}
				return
			}
			t.Errorf("wanted a %s finding, got %+v", c.key, findings)
		})
	}
}

// edit rewrites the fixture card, replacing one line with another.
func edit(t *testing.T, root, from, to string) {
	t.Helper()
	path := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	text, err := ReadText(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(text, from) {
		t.Fatalf("the fixture card carries no %q", from)
	}
	write(t, path, strings.Replace(text, from, to, 1))
}

// editWorkbench rewrites the fixture's own workbench anchor, replacing one
// line with another, the way edit does for the fixture card.
func editWorkbench(t *testing.T, root, from, to string) {
	t.Helper()
	path := filepath.Join(root, WorkbenchAnchor)
	text, err := ReadText(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(text, from) {
		t.Fatalf("the fixture workbench carries no %q", from)
	}
	write(t, path, strings.Replace(text, from, to, 1))
}

// appendText adds to the end of a file, which is how the journal cases build
// the state they check.
func appendText(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatalf("append: %v", err)
	}
}

// writeComment puts a comment on the fixture card by hand, which is how a
// test chooses both the identifier and the ordinal. An ordinal of zero writes
// no ordinal field, standing for a comment written before the field existed.
func writeComment(t *testing.T, root, id, ts string, ordinal int, body string) {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set("ts", ts)
	fm.Set("author", "alka")
	if ordinal > 0 {
		fm.Set(OrdinalField, strconv.Itoa(ordinal))
	}
	path := filepath.Join(root, CardsDir, "c00000000001", CommentsDir, id, CommentAnchor)
	write(t, path, fm.Render(body+"\n"))
}

// writeAttachment puts an attachment on the fixture card by hand, with the
// same ordinal convention writeComment uses.
func writeAttachment(t *testing.T, root, id string, ordinal int) {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set("filename", id+".txt")
	fm.Set("provenance", "alka")
	if ordinal > 0 {
		fm.Set(OrdinalField, strconv.Itoa(ordinal))
	}
	path := filepath.Join(root, CardsDir, "c00000000001", AttachmentsDir, id, AttachmentAnchor)
	write(t, path, fm.Render(""))
}

// writeItem puts a checklist item on the fixture card by hand. No verb creates
// one yet, so the ordinal on an item is a contract on whatever eventually
// does, and the migration and the checker have to hold it meanwhile.
func writeItem(t *testing.T, root, id string, ordinal int) {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set("kind", "decision")
	fm.Set("state", "pending")
	if ordinal > 0 {
		fm.Set(OrdinalField, strconv.Itoa(ordinal))
	}
	path := filepath.Join(root, CardsDir, "c00000000001", ChecklistDir, id, ItemAnchor)
	write(t, path, fm.Render("An item.\n"))
}

// commentedEvent is the journal line recording that a comment was written,
// which is what the migration recovers write order from.
func commentedEvent(id, ts string) string {
	return `{"ts":"` + ts + `","event":"commented","actor":"alka","comment":"` + id + `"}` + "\n"
}

// TestPositionFollowsWriteOrderRatherThanIdentifier asserts that a positional
// reference below a card counts in creation order.
//
// The fixture claims its identifiers in the reverse of its write order, so the
// directory listing and the creation order disagree on every position. That is
// the whole point: a listing is ascending hex, a hex identifier is random, and
// a reference that resolved through the listing named a different comment on
// every workbench.
func TestPositionFollowsWriteOrderRatherThanIdentifier(t *testing.T) {
	root := newFixture(t)
	writeComment(t, root, "e00000000009", "2026-08-17T09:01:00Z", 1, "written first")
	writeComment(t, root, "e00000000001", "2026-08-17T09:02:00Z", 2, "written second")

	collection := filepath.Join(root, CardsDir, "c00000000001", CommentsDir)
	if listed := ListIDs(collection); listed[0] != "e00000000001" {
		t.Fatalf("the fixture no longer disagrees with the listing, which leads with %s", listed[0])
	}

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	wanted := map[string]string{"1": "e00000000009", "2": "e00000000001"}
	for position, id := range wanted {
		path, err := opened.ResolvePath("fx-1/" + CommentsDir + "/" + position)
		if err != nil {
			t.Fatalf("resolve position %s: %v", position, err)
		}
		if got := filepath.Base(filepath.Dir(path)); got != id {
			t.Errorf("position %s resolved to %s, wanted %s", position, got, id)
		}
	}

	comments, err := Comments(filepath.Join(root, CardsDir, "c00000000001"))
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(comments) != 2 || !strings.Contains(comments[0].Body, "written first") {
		t.Errorf("the reading order is not the writing order: %+v", comments)
	}
}

// TestOrdinalMigrationReplaysTheJournalAndIsIdempotent asserts the one-time
// backfill: it recovers write order from the card's journal rather than from
// the directory listing, it leaves an entity already carrying an ordinal
// alone, and a second run changes nothing on disk.
func TestOrdinalMigrationReplaysTheJournalAndIsIdempotent(t *testing.T) {
	root := newFixture(t)
	journal := filepath.Join(root, CardsDir, "c00000000001", JournalName)

	// The journal records the high identifier first, so a migration reading
	// the listing instead would number these the other way around.
	writeComment(t, root, "e00000000009", "2026-08-17T09:01:00Z", 0, "written first")
	writeComment(t, root, "e00000000001", "2026-08-17T09:02:00Z", 0, "written second")
	appendText(t, journal, commentedEvent("e00000000009", "2026-08-17T09:01:00Z"))
	appendText(t, journal, commentedEvent("e00000000001", "2026-08-17T09:02:00Z"))

	// An attachment the journal never mentions, and a checklist item no verb
	// creates, both fall to listing order for their own uncovered stretch.
	writeAttachment(t, root, "f00000000001", 0)
	writeItem(t, root, "d00000000001", 0)
	// A comment already stamped keeps the value it carries, and the ordinal
	// it holds is stepped over rather than handed out twice.
	writeComment(t, root, "e00000000005", "2026-08-17T09:03:00Z", 3, "already stamped")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stamped, reported, err := opened.BackfillOrdinals("alka", "2026-08-17T10:00:00Z")
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if stamped != 4 {
		t.Errorf("wanted four entities stamped, got %d", stamped)
	}
	// The attachment and the checklist item have no creation event, so the
	// migration placed each of them by the directory listing and has to say
	// so. The two comments the journal covers are recovered facts and are
	// not reported.
	guessed := map[string]bool{}
	for _, finding := range reported {
		if finding.Key != FindingOrdinalGuessed {
			t.Errorf("the backfill reported %s, wanted only guessed ordinals", finding.Key)
			continue
		}
		guessed[finding.Detail] = true
	}
	if len(guessed) != 2 || !guessed["f00000000001"] || !guessed["d00000000001"] {
		t.Errorf("wanted the attachment and the item reported as guessed, got %+v", reported)
	}

	comments := filepath.Join(root, CardsDir, "c00000000001", CommentsDir)
	wanted := map[string]int{"e00000000009": 1, "e00000000001": 2, "e00000000005": 3}
	for id, ordinal := range wanted {
		if got := EntityOrdinal(comments, id, CommentAnchor); got != ordinal {
			t.Errorf("comment %s carries ordinal %d, wanted %d", id, got, ordinal)
		}
	}
	attachments := filepath.Join(root, CardsDir, "c00000000001", AttachmentsDir)
	if got := EntityOrdinal(attachments, "f00000000001", AttachmentAnchor); got != 1 {
		t.Errorf("the attachment carries ordinal %d, wanted 1", got)
	}
	checklist := filepath.Join(root, CardsDir, "c00000000001", ChecklistDir)
	if got := EntityOrdinal(checklist, "d00000000001", ItemAnchor); got != 1 {
		t.Errorf("the checklist item carries ordinal %d, wanted 1", got)
	}
	if findings, err := opened.Check(); err != nil || len(findings) != 0 {
		t.Errorf("a migrated workbench should check clean, got %+v (%v)", findings, err)
	}

	before := snapshot(t, root)
	again, reportedAgain, err := opened.BackfillOrdinals("alka", "2026-08-17T11:00:00Z")
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if again != 0 {
		t.Errorf("a second run stamped %d entities, wanted none", again)
	}
	if len(reportedAgain) != 0 {
		t.Errorf("a second run reported %+v, wanted nothing: it stamped nothing, so it guessed nothing", reportedAgain)
	}
	after := snapshot(t, root)
	if len(before) != len(after) {
		t.Fatalf("the second run changed the file count, %d then %d", len(before), len(after))
	}
	for path, text := range before {
		if after[path] != text {
			t.Errorf("the second run rewrote %s", path)
		}
	}
}

// snapshot reads every anchor file under a bench, keyed by path relative to
// the root, which is how the idempotence check compares two runs byte for
// byte rather than field by field.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	found := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return err
		}
		text, readErr := ReadText(path)
		if readErr != nil {
			return readErr
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		found[filepath.ToSlash(relative)] = text
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return found
}

// TestFrontmatterPreservesWhatItDoesNotKnow asserts the reader posture the
// format asks for: a key the tool has never heard of survives a read and a
// write back, whatever shape its value has.
func TestFrontmatterPreservesWhatItDoesNotKnow(t *testing.T) {
	original := `---
title: A card
acme.department: catering
acme.people:
  - ada
  - grace
substate: ready
---
Body text.
`
	fm, body := ParseAnchor(original)
	fm.Set("substate", "active")
	rewritten := fm.Render(body)
	for _, wanted := range []string{"acme.department: catering", "  - ada", "  - grace", "Body text."} {
		if !strings.Contains(rewritten, wanted) {
			t.Errorf("the rewrite lost %q:\n%s", wanted, rewritten)
		}
	}
	if !strings.Contains(rewritten, "substate: active") {
		t.Errorf("the rewrite did not apply the change:\n%s", rewritten)
	}
	again, _ := ParseAnchor(rewritten)
	if got := again.Seq("acme.people"); len(got) != 2 || got[0] != "ada" {
		t.Errorf("the sequence did not read back, got %v", got)
	}
}

// TestReadersTolerateCarriageReturns asserts the encoding rule: a writer
// emits LF everywhere and a reader strips a trailing carriage return per line
// rather than failing on it.
func TestReadersTolerateCarriageReturns(t *testing.T) {
	fm, body := ParseAnchor("---\r\ntitle: A card\r\nsubstate: ready\r\n---\r\nBody.\r\n")
	if fm.Value("title") != "A card" {
		t.Errorf("title: got %q", fm.Value("title"))
	}
	if strings.Contains(body, "\r") {
		t.Errorf("body: carriage returns survived, got %q", body)
	}
	if strings.Contains(fm.Render(body), "\r") {
		t.Error("the writer emitted a carriage return")
	}
}

// TestAsciiCaseRulesIgnoreTheLocale asserts CORE-TEXT-2 through the one place
// the tool lowercases a name a person typed: a state reference. The Turkish
// dotless i is the case that separates ASCII rules from locale-aware ones.
func TestAsciiCaseRulesIgnoreTheLocale(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor), "---\ntitle: INTAKE\nkind: work\n---\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if state := opened.StateByRef("intake"); state == nil {
		t.Error("a state title should match without regard to ASCII case")
	}
	if got := asciiLower("I"); got != "i" {
		t.Errorf("ASCII lowercasing of I: wanted i, got %q", got)
	}
	if got := asciiUpper("i"); got != "I" {
		t.Errorf("ASCII uppercasing of i: wanted I, got %q", got)
	}
}

// TestEditorLadderResolvesFirstHitWins asserts the editor ladder the operator
// ruled: DINAH_EDITOR, then the user config, then VISUAL, then EDITOR, then
// the platform fallback, with a value at a higher layer winning over every
// layer below it.
//
// Each case also asserts the rung the ladder names for the value it returned.
// The five rungs answer under five distinct names, so a reader whose VISUAL
// won can tell it from a reader whose EDITOR won, which is what a source
// collapsed to "environment" would hide.
func TestEditorLadderResolvesFirstHitWins(t *testing.T) {
	home := t.TempDir()
	cfg := LoadConfig(home)
	present := func(string) bool { return true }

	cases := []struct {
		name   string
		env    map[string]string
		config string
		goos   string
		wanted string
		source string
	}{
		{name: "the platform fallback on Windows", goos: "windows", wanted: "notepad", source: SourceFallback},
		{name: "the platform fallback elsewhere", goos: "linux", wanted: "nano", source: SourceFallback},
		{
			name:   "EDITOR beats the fallback",
			env:    map[string]string{"EDITOR": "ed"},
			goos:   "linux",
			wanted: "ed",
			source: SourceEnvironment,
		},
		{
			name:   "VISUAL beats EDITOR",
			env:    map[string]string{"EDITOR": "ed", "VISUAL": "vim"},
			goos:   "linux",
			wanted: "vim",
			source: SourceVisual,
		},
		{
			name:   "the config beats VISUAL",
			env:    map[string]string{"EDITOR": "ed", "VISUAL": "vim"},
			config: "kak",
			goos:   "linux",
			wanted: "kak",
			source: SourceConfig,
		},
		{
			name:   "DINAH_EDITOR beats every layer below it",
			env:    map[string]string{"DINAH_EDITOR": "helix", "EDITOR": "ed", "VISUAL": "vim"},
			config: "kak",
			goos:   "linux",
			wanted: "helix",
			source: SourceEditorVar,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, name := range []string{"DINAH_EDITOR", "VISUAL", "EDITOR"} {
				t.Setenv(name, c.env[name])
			}
			settings := cfg
			if c.config != "" {
				settings = LoadConfig(t.TempDir())
				if err := settings.Set("editor", c.config); err != nil {
					t.Fatalf("config: %v", err)
				}
			}
			got, source, err := ResolveEditorSource(settings, c.goos, present)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got != c.wanted {
				t.Errorf("wanted %s, got %s", c.wanted, got)
			}
			if source != c.source {
				t.Errorf("the rung that answered: wanted %s, got %s", c.source, source)
			}
			plain, err := ResolveEditor(settings, c.goos, present)
			if err != nil {
				t.Fatalf("resolve without the source: %v", err)
			}
			if plain != got {
				t.Errorf("the two forms disagree: %s against %s", plain, got)
			}
		})
	}

	// With no layer set and nothing on the path, the ladder refuses and names
	// what it looked for.
	for _, name := range []string{"DINAH_EDITOR", "VISUAL", "EDITOR"} {
		t.Setenv(name, "")
	}
	absent := func(string) bool { return false }
	if _, err := ResolveEditor(LoadConfig(t.TempDir()), "linux", absent); err == nil {
		t.Error("with no editor anywhere, the ladder should refuse")
	}
	// The listing reads the same refusal as an absence, so the source form
	// reports the rung as unset rather than naming one that answered.
	editor, source, err := ResolveEditorSource(LoadConfig(t.TempDir()), "linux", absent)
	if err == nil {
		t.Error("with no editor anywhere, the source form should refuse too")
	}
	if editor != "" || source != SourceUnset {
		t.Errorf("a refused editor: wanted an empty value at %s, got %q at %s", SourceUnset, editor, source)
	}
}

// TestLockRefusesRatherThanBreaking asserts the concurrency rule: a lock
// another process holds is refused loudly with the holder named from the
// lock's own content, and nothing breaks one silently.
func TestLockRefusesRatherThanBreaking(t *testing.T) {
	dir := t.TempDir()
	held, err := Acquire(dir, "alka", "2026-08-17T09:00:00Z")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, err := Acquire(dir, "bob", "2026-08-17T09:00:01Z"); err == nil {
		t.Fatal("a second acquisition should be refused")
	} else if !strings.Contains(err.Error(), "alka") {
		t.Errorf("the refusal should name the holder, got %v", err)
	}
	held.Release()
	second, err := Acquire(dir, "bob", "2026-08-17T09:00:02Z")
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	second.Release()
}

// TestLanguageLadderResolvesFirstHitWins asserts the display-language ladder
// the format's own section fixes: the flag, then DINAH_LANG, then the user
// config, then the operating system locale as a hint, then English, with each
// layer set in turn while the layers above it are unset.
//
// Each case also asserts the rung the ladder names. English reached because
// nothing carried a tag reports the default rung, which is what separates a
// language nobody chose from one somebody wrote into the config file.
func TestLanguageLadderResolvesFirstHitWins(t *testing.T) {
	cases := []struct {
		name   string
		flag   string
		env    map[string]string
		config string
		wanted string
		source string
	}{
		{name: "English when no layer carries one", wanted: "en", source: SourceDefault},
		{
			name:   "the OS locale as a hint, region and all",
			env:    map[string]string{"LANG": "cs_CZ.UTF-8"},
			wanted: "cs-CZ",
			source: SourceLocale,
		},
		{
			name:   "the config beats the OS locale",
			env:    map[string]string{"LANG": "cs_CZ.UTF-8"},
			config: "es",
			wanted: "es",
			source: SourceConfig,
		},
		{
			name:   "DINAH_LANG beats the config",
			env:    map[string]string{"LANG": "cs_CZ.UTF-8", "DINAH_LANG": "de"},
			config: "es",
			wanted: "de",
			source: SourceEnvironment,
		},
		{
			name:   "the flag beats every layer below it",
			flag:   "hi",
			env:    map[string]string{"LANG": "cs_CZ.UTF-8", "DINAH_LANG": "de"},
			config: "es",
			wanted: "hi",
			source: SourceFlag,
		},
		{
			name:   "a regional tag keeps its region in the catalogs' spelling",
			flag:   "en-uk",
			wanted: "en-GB",
			source: SourceFlag,
		},
		{
			name:   "the C locale describes no reader, so it is not a hint",
			env:    map[string]string{"LANG": "C.UTF-8"},
			wanted: "en",
			source: SourceDefault,
		},
		{
			name:   "a config carrying the default tag still reads as set",
			config: "en",
			wanted: "en",
			source: SourceConfig,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, name := range []string{"DINAH_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
				t.Setenv(name, c.env[name])
			}
			settings := LoadConfig(t.TempDir())
			if c.config != "" {
				if err := settings.Set("lang", c.config); err != nil {
					t.Fatalf("config: %v", err)
				}
			}
			if got := ResolveLang(c.flag, settings); got != c.wanted {
				t.Errorf("wanted %s, got %s", c.wanted, got)
			}
			got, source := ResolveLangSource(c.flag, settings)
			if got != c.wanted {
				t.Errorf("the source form resolved %s, wanted %s", got, c.wanted)
			}
			if source != c.source {
				t.Errorf("the rung that answered: wanted %s, got %s", c.source, source)
			}
		})
	}
}

// TestActorLadderNamesTheRungOrReportsNone asserts the owner ladder: the flag,
// then DINAH_ACTOR, then the user config, each named when it answers, and an
// owner nothing carries reported as unset rather than refused. ResolveActor is
// the form that refuses, and it keeps refusing.
func TestActorLadderNamesTheRungOrReportsNone(t *testing.T) {
	cases := []struct {
		name   string
		flag   string
		env    string
		config string
		wanted string
		source string
	}{
		{name: "nobody carries one", wanted: "", source: SourceUnset},
		{name: "the config alone", config: "alka", wanted: "alka", source: SourceConfig},
		{name: "DINAH_ACTOR beats the config", env: "bob", config: "alka", wanted: "bob", source: SourceEnvironment},
		{
			name:   "the flag beats every layer below it",
			flag:   "cass",
			env:    "bob",
			config: "alka",
			wanted: "cass",
			source: SourceFlag,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("DINAH_ACTOR", c.env)
			settings := LoadConfig(t.TempDir())
			if c.config != "" {
				if err := settings.Set("actor", c.config); err != nil {
					t.Fatalf("config: %v", err)
				}
			}
			got, source := ResolveActorSource(c.flag, settings)
			if got != c.wanted {
				t.Errorf("wanted %q, got %q", c.wanted, got)
			}
			if source != c.source {
				t.Errorf("the rung that answered: wanted %s, got %s", c.source, source)
			}
			resolved, err := ResolveActor(c.flag, settings)
			if c.wanted == "" {
				if err == nil {
					t.Error("an owner nothing carries should still refuse through ResolveActor")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if resolved != c.wanted {
				t.Errorf("the two forms disagree: %s against %s", resolved, c.wanted)
			}
		})
	}
}

// TestSlugsAreDerivedToTheGrammar asserts that a slug taken from a directory
// name is made to conform to the reference grammar, and that a name yielding
// nothing usable yields nothing rather than something invented.
func TestSlugsAreDerivedToTheGrammar(t *testing.T) {
	cases := []struct {
		name   string
		wanted string
	}{
		{name: "proj", wanted: "proj"},
		{name: "My Project", wanted: "myproject"},
		{name: "dinah-3", wanted: "dinah3"},
		{name: "2026-notes", wanted: "notes"},
		{name: "PROJ", wanted: "proj"},
		{name: "...", wanted: ""},
		{name: "42", wanted: ""},
	}
	for _, c := range cases {
		got := Slugify(c.name)
		if got != c.wanted {
			t.Errorf("%q: wanted %q, got %q", c.name, c.wanted, got)
		}
		if got != "" && !ValidSlug(got) {
			t.Errorf("%q: the derived slug %q does not conform", c.name, got)
		}
	}
	if ValidSlug("") {
		t.Error("an empty slug should not be valid")
	}
	if ValidSlug("My Project") {
		t.Error("a slug outside the grammar should not be valid")
	}
}

// TestDiscoveryTellsAnEmptySearchFromAnAmbiguousOne asserts that a base
// holding several workbenches is never reported as holding none. The walk
// still climbs past it rather than guessing, and the ambiguity it passed is
// what the refusal names when nothing closer resolves, together with the
// candidates and the base they sit in.
//
// The reported ambiguity is the first one met, closest to the starting
// directory, which is the base a reader is likeliest to have meant.
func TestDiscoveryTellsAnEmptySearchFromAnAmbiguousOne(t *testing.T) {
	tree := t.TempDir()
	outer := filepath.Join(tree, "outer")
	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeWorkbench(t, filepath.Join(outer, UserBaseName, "d00000000001"), "Far one")
	writeWorkbench(t, filepath.Join(outer, UserBaseName, "d00000000002"), "Far two")
	writeWorkbench(t, filepath.Join(inner, UserBaseName, "d00000000003"), "Near one")
	writeWorkbench(t, filepath.Join(inner, UserBaseName, "d00000000004"), "Near two")

	_, _, err := Discover(inner, "", filepath.Join(tree, "home"), "")
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.AmbiguousWorkbench {
		t.Errorf("refusal name: wanted %s, got %s", contract.AmbiguousWorkbench, refusal.Name)
	}
	if got := refusal.Extra["base"]; got != filepath.Join(inner, UserBaseName) {
		t.Errorf("the base named: wanted the closest one, got %q", got)
	}
	rows, err := Reachable(inner, "", filepath.Join(tree, "home"), "")
	if err != nil {
		t.Fatalf("Reachable: %v", err)
	}
	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}
	joined := strings.Join(titles, "; ")
	for _, title := range []string{"Near one", "Near two"} {
		if !strings.Contains(joined, title) {
			t.Errorf("the candidates should name %q, got %q", title, joined)
		}
	}
	if strings.Contains(joined, "Far ") {
		t.Errorf("the further ambiguity should not be reported, got %q", joined)
	}

	// One workbench in a base is not ambiguous, and the search takes it.
	sole := filepath.Join(t.TempDir(), "solo")
	if err := os.MkdirAll(sole, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeWorkbench(t, filepath.Join(sole, UserBaseName, "d00000000005"), "The only one")
	found, _, err := Discover(sole, "", filepath.Join(tree, "home"), "")
	if err != nil {
		t.Fatalf("a base holding one workbench should resolve, got %v", err)
	}
	if found != filepath.Join(sole, UserBaseName, "d00000000005") {
		t.Errorf("the sole workbench: got %q", found)
	}
}

// TestDiscoveryNamesTheDirectoryBenchWasPointedAt asserts that a --workbench
// override carrying no workbench is refused against the path the caller gave,
// which is the one scenario dinah.no-workbench still covers.
func TestDiscoveryNamesTheDirectoryBenchWasPointedAt(t *testing.T) {
	empty := t.TempDir()
	_, _, err := Discover(empty, empty, "", "")
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.NoWorkbench {
		t.Errorf("refusal name: wanted %s, got %s", contract.NoWorkbench, refusal.Name)
	}
	if refusal.Detail != empty {
		t.Errorf("the refusal should name the directory given, wanted %q, got %q", empty, refusal.Detail)
	}
}

// TestConfiguredWorkbenchAnswersOnlyWhenSearchFindsNothing asserts the whole
// shape dinah-70 adds: a configured default opens a workbench when the walk
// finds nothing local, is never consulted when the walk resolves a sole
// workbench of its own, refuses by its own name when the configured path no
// longer carries a workbench.md, and never breaks an ambiguous base's tie.
func TestConfiguredWorkbenchAnswersOnlyWhenSearchFindsNothing(t *testing.T) {
	t.Run("answers when nothing local is reachable", func(t *testing.T) {
		nowhere := t.TempDir()
		configured := t.TempDir()
		writeWorkbench(t, configured, "Configured one")
		root, source, _, err := DiscoverSource(nowhere, "", "", filepath.Join(t.TempDir(), "home"), "", configured)
		if err != nil {
			t.Fatalf("wanted the configured default to answer, got %v", err)
		}
		if root != configured {
			t.Errorf("root: wanted %q, got %q", configured, root)
		}
		if source != SourceConfig {
			t.Errorf("source: wanted %s, got %s", SourceConfig, source)
		}
	})

	t.Run("a local workbench wins over a configured default pointing elsewhere", func(t *testing.T) {
		local := t.TempDir()
		writeWorkbench(t, local, "Local one")
		configured := t.TempDir()
		writeWorkbench(t, configured, "Elsewhere")
		root, source, _, err := DiscoverSource(local, "", "", filepath.Join(t.TempDir(), "home"), "", configured)
		if err != nil {
			t.Fatalf("a local workbench should resolve without consulting the configured default, got %v", err)
		}
		if root != local {
			t.Errorf("the local workbench should win, wanted %q, got %q", local, root)
		}
		if source != SourceSearch {
			t.Errorf("source: wanted %s, got %s", SourceSearch, source)
		}
	})

	t.Run("a local workbench wins even when the configured default is unreachable", func(t *testing.T) {
		local := t.TempDir()
		writeWorkbench(t, local, "Local one")
		configured := filepath.Join(t.TempDir(), "does-not-exist")
		root, source, _, err := DiscoverSource(local, "", "", filepath.Join(t.TempDir(), "home"), "", configured)
		if err != nil {
			t.Fatalf("a local workbench should resolve without consulting the configured default, got %v", err)
		}
		if root != local {
			t.Errorf("root: wanted %q, got %q", local, root)
		}
		if source != SourceSearch {
			t.Errorf("source: wanted %s, got %s", SourceSearch, source)
		}
	})

	t.Run("an ambiguous base still refuses and the configured default does not break the tie", func(t *testing.T) {
		base := t.TempDir()
		writeWorkbench(t, filepath.Join(base, UserBaseName, "d00000000006"), "One")
		writeWorkbench(t, filepath.Join(base, UserBaseName, "d00000000007"), "Two")
		configured := t.TempDir()
		writeWorkbench(t, configured, "Configured one")
		_, _, _, err := DiscoverSource(base, "", "", filepath.Join(t.TempDir(), "home"), "", configured)
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("wanted a refusal, got %v", err)
		}
		if refusal.Name != contract.AmbiguousWorkbench {
			t.Errorf("refusal name: wanted %s, got %s", contract.AmbiguousWorkbench, refusal.Name)
		}
	})

	t.Run("a configured path with no workbench.md refuses by its own name and does not fall through", func(t *testing.T) {
		nowhere := t.TempDir()
		configured := t.TempDir() // exists, but writeWorkbench was never called here
		_, _, _, err := DiscoverSource(nowhere, "", "", filepath.Join(t.TempDir(), "home"), "", configured)
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("wanted a refusal, got %v", err)
		}
		if refusal.Name != contract.NoConfiguredWorkbench {
			t.Errorf("refusal name: wanted %s, got %s (dinah.no-workbench-found would be the silent fall-through this guards against)", contract.NoConfiguredWorkbench, refusal.Name)
		}
		if refusal.Detail != configured {
			t.Errorf("the refusal should name the configured path, wanted %q, got %q", configured, refusal.Detail)
		}
	})

	t.Run("a configured path that does not exist at all refuses the same way", func(t *testing.T) {
		nowhere := t.TempDir()
		configured := filepath.Join(t.TempDir(), "gone")
		_, _, _, err := DiscoverSource(nowhere, "", "", filepath.Join(t.TempDir(), "home"), "", configured)
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("wanted a refusal, got %v", err)
		}
		if refusal.Name != contract.NoConfiguredWorkbench {
			t.Errorf("refusal name: wanted %s, got %s", contract.NoConfiguredWorkbench, refusal.Name)
		}
	})

	t.Run("nothing local and nothing configured still refuses no-workbench-found", func(t *testing.T) {
		nowhere := t.TempDir()
		_, _, _, err := DiscoverSource(nowhere, "", "", filepath.Join(t.TempDir(), "home"), "", "")
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("wanted a refusal, got %v", err)
		}
		if refusal.Name != contract.NoWorkbenchFound {
			t.Errorf("refusal name: wanted %s, got %s", contract.NoWorkbenchFound, refusal.Name)
		}
	})

	t.Run("an override names its own source and is never overridden by the configured default", func(t *testing.T) {
		override := t.TempDir()
		writeWorkbench(t, override, "Overridden")
		configured := t.TempDir()
		writeWorkbench(t, configured, "Configured one")
		root, source, _, err := DiscoverSource(t.TempDir(), override, SourceFlag, filepath.Join(t.TempDir(), "home"), "", configured)
		if err != nil {
			t.Fatalf("an override should resolve without consulting the configured default, got %v", err)
		}
		if root != override {
			t.Errorf("root: wanted %q, got %q", override, root)
		}
		if source != SourceFlag {
			t.Errorf("source: wanted %s, got %s", SourceFlag, source)
		}
	})

	t.Run("Discover itself is unchanged: no rung named, no configured default consulted", func(t *testing.T) {
		nowhere := t.TempDir()
		configured := t.TempDir()
		writeWorkbench(t, configured, "Configured one")
		// Discover has no way to be handed a configured default at all, so a
		// search that finds nothing still refuses dinah.no-workbench-found
		// even though a workbench sits at `configured`.
		_, _, err := Discover(nowhere, "", filepath.Join(t.TempDir(), "home"), "")
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("wanted a refusal, got %v", err)
		}
		if refusal.Name != contract.NoWorkbenchFound {
			t.Errorf("refusal name: wanted %s, got %s", contract.NoWorkbenchFound, refusal.Name)
		}
	})
}

// TestMalformedCarriesTheFileItWasRaisedOver asserts that every malformed
// refusal Open and readState raise names the file a reader has to repair,
// which is what the sentence's location fragment renders from.
func TestMalformedCarriesTheFileItWasRaisedOver(t *testing.T) {
	cases := []struct {
		name   string
		damage func(t *testing.T, root string)
		detail string
		path   func(root string) string
	}{
		{
			name: "a workbench predating the profile line",
			damage: func(t *testing.T, root string) {
				write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(benchDefinition, "profile: dinah-core/1.0\n", "", 1))
			},
			detail: "profile",
			path:   func(root string) string { return filepath.Join(root, WorkbenchAnchor) },
		},
		{
			name: "a workbench declaring no title",
			damage: func(t *testing.T, root string) {
				write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(benchDefinition, "title: Fixture\n", "", 1))
			},
			detail: "title",
			path:   func(root string) string { return filepath.Join(root, WorkbenchAnchor) },
		},
		{
			name: "a workbench anchor that will not read at all",
			damage: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, WorkbenchAnchor)); err != nil {
					t.Fatalf("remove: %v", err)
				}
			},
			detail: WorkbenchAnchor,
			path:   func(root string) string { return filepath.Join(root, WorkbenchAnchor) },
		},
		{
			name: "a state whose kind is outside the three",
			damage: func(t *testing.T, root string) {
				write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor), "---\ntitle: Only\nkind: dawdling\n---\n")
			},
			detail: "state b00000000001",
			path: func(root string) string {
				return filepath.Join(root, StatesDir, "b00000000001", StateAnchor)
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			c.damage(t, root)
			_, err := Open(root)
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("wanted a refusal, got %v", err)
			}
			if refusal.Name != contract.Malformed {
				t.Errorf("refusal name: wanted %s, got %s", contract.Malformed, refusal.Name)
			}
			if refusal.Detail != c.detail {
				t.Errorf("detail: wanted %q, got %q", c.detail, refusal.Detail)
			}
			if got := refusal.Extra["path"]; got != c.path(root) {
				t.Errorf("path: wanted %q, got %q", c.path(root), got)
			}
		})
	}
}

// writeWorkbench puts a minimal workbench anchor carrying a title at a path,
// which is all discovery reads of a candidate.
func writeWorkbench(t *testing.T, root, title string) {
	t.Helper()
	write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(benchDefinition, "title: Fixture", "title: "+title, 1))
}

// TestDiscoveryReportsAnExhaustedWalk asserts what a search that finds nothing
// says: the directory it started from, the user base it fell back to, and the
// two ways out of the situation.
//
// The search starts at the volume root, because a walk from anywhere deeper
// climbs through directories the test does not control and can meet somebody
// else's workbench on the way.
func TestDiscoveryReportsAnExhaustedWalk(t *testing.T) {
	tree := t.TempDir()
	root := filepath.VolumeName(tree) + string(filepath.Separator)
	if found, ambiguous, _, err := benchIn(root, false); found != "" || len(ambiguous) > 0 || err != nil {
		t.Skip("the volume root carries a workbench of its own")
	}
	home := filepath.Join(tree, "home")
	_, _, err := Discover(root, "", home, "")
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.NoWorkbenchFound {
		t.Errorf("refusal name: wanted %s, got %s", contract.NoWorkbenchFound, refusal.Name)
	}
	if refusal.Detail != root {
		t.Errorf("the refusal should name where the search began, wanted %q, got %q", root, refusal.Detail)
	}
	if got := refusal.Extra["home"]; got != filepath.Join(home, UserBaseName) {
		t.Errorf("the refusal should name the user base, wanted %q, got %q", filepath.Join(home, UserBaseName), got)
	}
}

// TestDiscoveryLeavesTheNativeHomeBaseToTheFallback asserts the boundary the
// ancestor walk observes once the user base has been pointed elsewhere. A
// working directory nested under the machine's own home no longer resolves to
// the workbench under that home's .dinah, because relocating the user base is
// meant to relocate all of it rather than only the fallback's half.
//
// The other two assertions fence the boundary in. Every .dinah above and below
// the native home is consulted exactly as before, which is what keeps the
// repository-nested convention working, and the anchor check still runs at the
// native home itself, so a repository checked out there is still found.
func TestDiscoveryLeavesTheNativeHomeBaseToTheFallback(t *testing.T) {
	native := filepath.Join(t.TempDir(), "native")
	deep := filepath.Join(native, "src", "repos", "project", "internal", "part")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeWorkbench(t, filepath.Join(native, UserBaseName, "d00000000010"), "The real one")
	relocated := t.TempDir()

	_, _, err := Discover(deep, "", relocated, native)
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("the relocated search should find nothing, got %v", err)
	}
	if refusal.Name != contract.NoWorkbenchFound {
		t.Errorf("refusal name: wanted %s, got %s", contract.NoWorkbenchFound, refusal.Name)
	}
	if got := refusal.Extra["home"]; got != filepath.Join(relocated, UserBaseName) {
		t.Errorf("the refusal should name the relocated user base, wanted %q, got %q", filepath.Join(relocated, UserBaseName), got)
	}

	// An ordinary ancestor's own .dinah is untouched by the boundary, so the
	// repository-nested convention resolves from the same starting directory.
	nested := filepath.Join(native, "src", "repos", "project", UserBaseName, "d00000000011")
	writeWorkbench(t, nested, "The repository one")
	found, _, err := Discover(deep, "", relocated, native)
	if err != nil {
		t.Fatalf("a .dinah below the native home should still resolve, got %v", err)
	}
	if found != nested {
		t.Errorf("the nested workbench: wanted %q, got %q", nested, found)
	}

	// Only the .dinah check is skipped at the native home. A workbench sitting
	// there as a bare anchor is still discovered.
	anchored := filepath.Join(t.TempDir(), "anchored")
	inside := filepath.Join(anchored, "sub")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeWorkbench(t, anchored, "Checked out at home")
	found, _, err = Discover(inside, "", t.TempDir(), anchored)
	if err != nil {
		t.Fatalf("an anchor at the native home should still resolve, got %v", err)
	}
	if found != anchored {
		t.Errorf("the workbench at the native home: wanted %q, got %q", anchored, found)
	}
}

// TestDiscoveryUnrelocatedStillFindsTheUserBase asserts that the boundary
// costs nothing when nobody moved the user base. The walk skips the native
// home's .dinah and the fallback then reads the same directory one step later,
// so a person working under their own home reaches their own workbenches
// exactly as they did before.
func TestDiscoveryUnrelocatedStillFindsTheUserBase(t *testing.T) {
	native := filepath.Join(t.TempDir(), "native")
	deep := filepath.Join(native, "src", "project")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := filepath.Join(native, UserBaseName, "d00000000012")
	writeWorkbench(t, want, "The only one")

	found, _, err := Discover(deep, "", native, native)
	if err != nil {
		t.Fatalf("an unrelocated search should resolve, got %v", err)
	}
	if found != want {
		t.Errorf("the user base workbench: wanted %q, got %q", want, found)
	}
}

// TestTheBoundaryHoldsAgainstAnotherSpellingOfTheHome asserts that the walk
// recognises its boundary when the home directory is named in a spelling that
// differs from the one the climb produces. Windows gives a user name holding a
// space a short 8.3 form and ignores case, and macOS mounts a case-insensitive
// volume, so the home directory arrives under one spelling and the ancestor
// under another; a boundary compared as text misses, and the search then reads
// the real user base.
//
// The second spelling comes from anotherSpelling, which confirms it names the
// same directory before the test asserts anything. A platform offering the
// directory one spelling only has no aliasing to reproduce, so the test skips
// there.
func TestTheBoundaryHoldsAgainstAnotherSpellingOfTheHome(t *testing.T) {
	tree := t.TempDir()
	native := filepath.Join(tree, "native")
	deep := filepath.Join(native, "src", "repos", "project")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	alias := anotherSpelling(t, tree, native)
	if alias == "" {
		t.Skip("this platform offers the directory one spelling only")
	}
	writeWorkbench(t, filepath.Join(native, UserBaseName, "d00000000013"), "The real one")
	relocated := t.TempDir()

	_, _, err := Discover(deep, "", relocated, alias)
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("the boundary should hold under the aliased spelling, got %v", err)
	}
	if refusal.Name != contract.NoWorkbenchFound {
		t.Errorf("refusal name: wanted %s, got %s", contract.NoWorkbenchFound, refusal.Name)
	}
}

// anotherSpelling returns a second path naming the directory at native, or an
// empty string where the platform offers none. A symlink beside it is the
// spelling macOS hands out for its own temporary directory, and no folding of
// case absorbs it. Where the privilege to create one is missing, a spelling
// differing only in case stands in, which names the same directory on a
// case-insensitive volume and a different one anywhere else.
func anotherSpelling(t *testing.T, tree, native string) string {
	t.Helper()
	linked := filepath.Join(tree, "linked")
	if err := os.Symlink(native, linked); err == nil && oneDirectory(t, native, linked) {
		return linked
	}
	cased := filepath.Join(tree, "NATIVE")
	if oneDirectory(t, native, cased) {
		return cased
	}
	return ""
}

// oneDirectory reports whether two paths name the same directory, asking the
// filesystem the way samePath does so that the test skips on a filesystem that
// has no aliasing rather than asserting something it cannot produce.
func oneDirectory(t *testing.T, a, b string) bool {
	t.Helper()
	mine, err := os.Stat(a)
	if err != nil {
		t.Fatalf("stat %s: %v", a, err)
	}
	theirs, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(mine, theirs)
}

// TestTheUserBaseBeatsABaseAboveTheHome asserts the precedence the user base
// has always had. A .dinah sitting above the home directory is reached by the
// climb after the home rung, and the home rung is where the walk consults the
// user base, so the person's own workbench still answers rather than the one
// belonging to a directory they share with every other account.
func TestTheUserBaseBeatsABaseAboveTheHome(t *testing.T) {
	accounts := t.TempDir()
	native := filepath.Join(accounts, "native")
	deep := filepath.Join(native, "src", "project")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeWorkbench(t, filepath.Join(accounts, UserBaseName, "d00000000014"), "The one above home")
	want := filepath.Join(native, UserBaseName, "d00000000015")
	writeWorkbench(t, want, "The user base one")

	found, _, err := Discover(deep, "", native, native)
	if err != nil {
		t.Fatalf("the search should resolve to the user base, got %v", err)
	}
	if found != want {
		t.Errorf("the user base should win over a base above the home, wanted %q, got %q", want, found)
	}
}

// TestAWriteBeforeTheMigrationKeepsItsPlaceInTheOrder replays the whole
// sequence a live workbench is in the middle of: entities written before the
// ordinal field existed, one new comment written by the tool while the
// workbench is still unmigrated, and only then the migration.
//
// The new comment must not end up first. It is written last, so it takes the
// ordinal past the members already there, and the migration then numbers the
// older comments underneath it. Counting only the ordinals in use would hand
// the new comment 1 and push the two older ones to 2 and 3, which puts the
// newest comment permanently at the head of the order with nothing to report,
// and that is the disorder this whole card exists to remove.
func TestAWriteBeforeTheMigrationKeepsItsPlaceInTheOrder(t *testing.T) {
	root := newFixture(t)
	cardDir := filepath.Join(root, CardsDir, "c00000000001")
	journal := filepath.Join(cardDir, JournalName)

	// The identifiers run against the write order, so the directory listing
	// cannot accidentally produce the right answer.
	writeComment(t, root, "e00000000009", "2026-08-17T09:01:00Z", 0, "written first")
	writeComment(t, root, "e00000000005", "2026-08-17T09:02:00Z", 0, "written second")
	appendText(t, journal, commentedEvent("e00000000009", "2026-08-17T09:01:00Z"))
	appendText(t, journal, commentedEvent("e00000000005", "2026-08-17T09:02:00Z"))

	third, err := AddComment(cardDir, "alka", "2026-08-17T09:03:00Z", "written third")
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	appendText(t, journal, commentedEvent(third.ID, "2026-08-17T09:03:00Z"))
	if third.Ordinal != 3 {
		t.Errorf("the comment written third took ordinal %d, which lands it ahead of the two it followed", third.Ordinal)
	}

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stamped, reported, err := opened.BackfillOrdinals("alka", "2026-08-17T10:00:00Z")
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if stamped != 2 {
		t.Errorf("wanted the two older comments stamped, got %d", stamped)
	}
	if len(reported) != 0 {
		t.Errorf("the journal covers every comment here, so nothing was guessed, got %+v", reported)
	}

	collection := filepath.Join(cardDir, CommentsDir)
	for position, id := range []string{"e00000000009", "e00000000005", third.ID} {
		if got := EntityOrdinal(collection, id, CommentAnchor); got != position+1 {
			t.Errorf("comment %s carries ordinal %d, wanted %d", id, got, position+1)
		}
	}

	comments, err := Comments(cardDir)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	for position, body := range []string{"written first", "written second", "written third"} {
		if position >= len(comments) || !strings.Contains(comments[position].Body, body) {
			t.Fatalf("the reading order is not the writing order: %+v", comments)
		}
	}
	if findings, err := opened.Check(); err != nil || len(findings) != 0 {
		t.Errorf("a migrated workbench should check clean, got %+v (%v)", findings, err)
	}
}

// TestTheMigrationStepsOverALockedCardAndFinishesTheWalk asserts that one
// locked card costs the operator that card and nothing else.
//
// A lock on a card is ordinary: another process holds it right now. Returning
// on the first one would end the walk at whatever card the listing happened to
// put first, leave every card after it unstamped, and report a bare refusal
// that names neither the card nor what had already been done.
func TestTheMigrationStepsOverALockedCardAndFinishesTheWalk(t *testing.T) {
	root := newFixture(t)
	second := filepath.Join(root, CardsDir, "c00000000002")
	write(t, filepath.Join(second, CardAnchor), cleanCard)
	write(t, filepath.Join(second, JournalName), cleanJournal)
	write(t, filepath.Join(second, CommentsDir, "e00000000001", CommentAnchor),
		"---\nts: 2026-08-17T09:05:00Z\nauthor: alka\n---\nOn the second card.\n")

	// The locked card is the first the walk reaches, so a walk that carried
	// on is the only way the second card gets stamped at all.
	locked := filepath.Join(root, CardsDir, "c00000000001")
	writeComment(t, root, "e00000000009", "2026-08-17T09:01:00Z", 0, "never stamped")
	write(t, filepath.Join(locked, LockName), `{"actor":"someone-else","pid":1,"ts":"2026-08-17T09:00:00Z"}`+"\n")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stamped, reported, err := opened.BackfillOrdinals("alka", "2026-08-17T10:00:00Z")
	if err != nil {
		t.Fatalf("a locked card should not end the walk: %v", err)
	}
	if stamped != 1 {
		t.Errorf("wanted the unlocked card's comment stamped, got %d", stamped)
	}
	if got := EntityOrdinal(filepath.Join(second, CommentsDir), "e00000000001", CommentAnchor); got != 1 {
		t.Errorf("the comment on the unlocked card carries ordinal %d, wanted 1", got)
	}
	if got := EntityOrdinal(filepath.Join(locked, CommentsDir), "e00000000009", CommentAnchor); got != 0 {
		t.Errorf("the locked card was written to anyway, its comment carries %d", got)
	}
	detail, found := "", false
	for _, finding := range reported {
		if finding.Key == FindingOrdinalLocked {
			detail, found = finding.Detail, true
		}
	}
	if !found || detail != "c00000000001" {
		t.Errorf("wanted the locked card reported by identifier, got %+v", reported)
	}
}

// TestTheMigrationStepsOverAnUnwritableEntityAndFinishesTheWalk asserts that
// one entity the run cannot write to costs the operator that entity and
// nothing else: the walk finishes, the obstruction is named, and the guesses
// the run did manage to make before and after it are still reported.
//
// Before this test, a stamp failure aborted backfillCollection outright and
// the caller threw the whole report away with the error, so a run that met
// one unwritable file printed nothing: not the count, not the obstruction,
// and not the guess it had already made on the same collection.
//
// The obstruction is injected through the bench's stamp hook rather than
// through file permissions. An earlier form of this test made the anchor
// read-only, which stands for an unwritable entity on Windows and stands for
// nothing at all on POSIX, where the migration's rename is governed by the
// containing directory and replaced the file anyway. A test whose meaning
// changes with the operating system cannot be the one carrying this
// behaviour, so the hook provokes the same error path everywhere and
// TestAnUnwritableDirectoryIsAnUnwritableEntity covers the POSIX case where a
// permission genuinely stops the write.
func TestTheMigrationStepsOverAnUnwritableEntityAndFinishesTheWalk(t *testing.T) {
	root := newFixture(t)
	first := filepath.Join(root, CardsDir, "c00000000001")
	firstJournal := filepath.Join(first, JournalName)

	// A journalled comment: the migration recovers its order and stamps it,
	// same as any other run.
	writeComment(t, root, "e00000000001", "2026-08-17T09:01:00Z", 0, "journalled")
	appendText(t, firstJournal, commentedEvent("e00000000001", "2026-08-17T09:01:00Z"))

	// A hand-created comment with no journal event: a guess, and this run is
	// the one that makes it. Its write is the one the hook fails, so the walk
	// must name it and step over it rather than stamping it or aborting on it.
	writeComment(t, root, "e00000000002", "2026-08-17T09:02:00Z", 0, "unwritable guess")

	// A second hand-created comment, writable, on the same collection: it
	// must still be stamped and reported as a guess even though the walk met
	// an obstruction on its neighbour first.
	writeComment(t, root, "e00000000003", "2026-08-17T09:03:00Z", 0, "writable guess")

	// A second card, unlocked and unobstructed: reaching it at all is what
	// proves the walk did not abort on the first card's obstruction.
	second := filepath.Join(root, CardsDir, "c00000000002")
	write(t, filepath.Join(second, CardAnchor), cleanCard)
	write(t, filepath.Join(second, JournalName), cleanJournal)
	write(t, filepath.Join(second, CommentsDir, "e00000000004", CommentAnchor),
		"---\nts: 2026-08-17T09:05:00Z\nauthor: alka\n---\nOn the second card.\n")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	opened.Hooks = &Hooks{
		BeforeOrdinalStamp: func(id string) error {
			if id == "e00000000002" {
				return errors.New("the filesystem refused this anchor")
			}
			return nil
		},
	}
	stamped, reported, err := opened.BackfillOrdinals("alka", "2026-08-17T10:00:00Z")
	if err != nil {
		t.Fatalf("an unwritable entity should not end the walk: %v", err)
	}
	if stamped != 3 {
		t.Errorf("wanted the journalled comment, the writable guess and the second card's comment stamped, got %d", stamped)
	}

	firstComments := filepath.Join(first, CommentsDir)
	if got := EntityOrdinal(firstComments, "e00000000001", CommentAnchor); got == 0 {
		t.Error("the journalled comment was not stamped")
	}
	if got := EntityOrdinal(firstComments, "e00000000002", CommentAnchor); got != 0 {
		t.Errorf("the unwritable comment carries ordinal %d, wanted it left alone", got)
	}
	if got := EntityOrdinal(firstComments, "e00000000003", CommentAnchor); got == 0 {
		t.Error("the writable guess after the obstruction was not stamped")
	}
	secondComments := filepath.Join(second, CommentsDir)
	if got := EntityOrdinal(secondComments, "e00000000004", CommentAnchor); got != 1 {
		t.Errorf("the second card's comment carries ordinal %d, wanted 1; the walk did not reach it", got)
	}

	unwritable, guessedWritable, unwritableDetail := false, false, ""
	for _, finding := range reported {
		if finding.Key == FindingOrdinalUnwritable {
			unwritable, unwritableDetail = true, finding.Detail
		}
		if finding.Key == FindingOrdinalGuessed && finding.Detail == "e00000000003" {
			guessedWritable = true
		}
	}
	if !unwritable || unwritableDetail != "e00000000002" {
		t.Errorf("wanted the unwritable comment named by identifier, got %+v", reported)
	}
	if !guessedWritable {
		t.Errorf("wanted the writable guess made after the obstruction still reported, got %+v", reported)
	}
}

// TestAnUnwritableDirectoryIsAnUnwritableEntity checks that a real permission
// the migration cannot get past produces the same report the hook-driven test
// drives, so the finding is not an artefact of the seam it is provoked
// through.
//
// The permission is on the entity's DIRECTORY, which is the one POSIX asks
// about. The migration writes a temporary beside the anchor and renames it
// over, so it is the right to create and rename a name in that directory that
// decides the write, and a read-only anchor in a writable directory is
// replaced rather than refused. This test therefore says nothing on Windows,
// where the file's own attribute governs instead, and nothing under a user who
// ignores the permission, so it skips in both cases rather than asserting
// something it cannot mean there.
func TestAnUnwritableDirectoryIsAnUnwritableEntity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not govern replacement on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root is not stopped by a directory permission")
	}
	root := newFixture(t)
	writeComment(t, root, "e00000000001", "2026-08-17T09:01:00Z", 0, "unwritable")
	dir := filepath.Join(root, CardsDir, "c00000000001", CommentsDir, "e00000000001")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stamped, reported, err := opened.BackfillOrdinals("alka", "2026-08-17T10:00:00Z")
	if err != nil {
		t.Fatalf("an unwritable entity should not end the walk: %v", err)
	}
	if stamped != 0 {
		t.Errorf("stamped %d entities, wanted none: the only one was unwritable", stamped)
	}
	named := false
	for _, finding := range reported {
		if finding.Key == FindingOrdinalUnwritable && finding.Detail == "e00000000001" {
			named = true
		}
	}
	if !named {
		t.Errorf("wanted the entity in the unwritable directory named, got %+v", reported)
	}
}
