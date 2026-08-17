package bench

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

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
			name:    "a clean bench",
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
			name: "a card naming a state the bench does not declare",
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
					t.Fatalf("a clean bench should report nothing, got %+v", findings)
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
	stamped, err := opened.BackfillOrdinals("alka", "2026-08-17T10:00:00Z")
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if stamped != 4 {
		t.Errorf("wanted four entities stamped, got %d", stamped)
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
	if findings, err := opened.Fsck(); err != nil || len(findings) != 0 {
		t.Errorf("a migrated bench should check clean, got %+v (%v)", findings, err)
	}

	before := snapshot(t, root)
	again, err := opened.BackfillOrdinals("alka", "2026-08-17T11:00:00Z")
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if again != 0 {
		t.Errorf("a second run stamped %d entities, wanted none", again)
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
	}{
		{name: "the platform fallback on Windows", goos: "windows", wanted: "notepad"},
		{name: "the platform fallback elsewhere", goos: "linux", wanted: "nano"},
		{name: "EDITOR beats the fallback", env: map[string]string{"EDITOR": "ed"}, goos: "linux", wanted: "ed"},
		{name: "VISUAL beats EDITOR", env: map[string]string{"EDITOR": "ed", "VISUAL": "vim"}, goos: "linux", wanted: "vim"},
		{name: "the config beats VISUAL", env: map[string]string{"EDITOR": "ed", "VISUAL": "vim"}, config: "kak", goos: "linux", wanted: "kak"},
		{
			name:   "DINAH_EDITOR beats every layer below it",
			env:    map[string]string{"DINAH_EDITOR": "helix", "EDITOR": "ed", "VISUAL": "vim"},
			config: "kak",
			goos:   "linux",
			wanted: "helix",
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
			got, err := ResolveEditor(settings, c.goos, present)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got != c.wanted {
				t.Errorf("wanted %s, got %s", c.wanted, got)
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
func TestLanguageLadderResolvesFirstHitWins(t *testing.T) {
	cases := []struct {
		name   string
		flag   string
		env    map[string]string
		config string
		wanted string
	}{
		{name: "English when no layer carries one", wanted: "en"},
		{name: "the OS locale as a hint, region and all", env: map[string]string{"LANG": "cs_CZ.UTF-8"}, wanted: "cs-CZ"},
		{
			name:   "the config beats the OS locale",
			env:    map[string]string{"LANG": "cs_CZ.UTF-8"},
			config: "es",
			wanted: "es",
		},
		{
			name:   "DINAH_LANG beats the config",
			env:    map[string]string{"LANG": "cs_CZ.UTF-8", "DINAH_LANG": "de"},
			config: "es",
			wanted: "de",
		},
		{
			name:   "the flag beats every layer below it",
			flag:   "hi",
			env:    map[string]string{"LANG": "cs_CZ.UTF-8", "DINAH_LANG": "de"},
			config: "es",
			wanted: "hi",
		},
		{
			name:   "a regional tag keeps its region in the catalogs' spelling",
			flag:   "en-uk",
			wanted: "en-GB",
		},
		{
			name:   "the C locale describes no reader, so it is not a hint",
			env:    map[string]string{"LANG": "C.UTF-8"},
			wanted: "en",
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
