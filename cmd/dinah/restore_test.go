package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide/guidepin"
)

// foldedStderr is a refusal's stderr with every run of whitespace folded to
// one space. Every fragment match below runs over this rather than over the
// raw stream, because the renderer wraps a sentence at the terminal's width
// and a test matching a wrapped sentence reports a failure the reader would
// never see.
func foldedStderr(errw string) string {
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(errw, " "))
}

// firstToken is the refusal name a refused run printed, which is the first
// word of its stderr.
func firstToken(errw string) string {
	fields := strings.Fields(errw)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// detailOfRun reads the identifier a successful machine-format run reported,
// which is the entity's own twelve-hex identifier for archive and restore.
func detailOfRun(t *testing.T, got invocation) string {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("the run exited %d: %s", got.code, got.errw)
	}
	var answer struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(got.out), &answer); err != nil {
		t.Fatalf("the answer does not parse: %v\n%s", err, got.out)
	}
	if !bench.IsID(answer.Detail) {
		t.Fatalf("the answer carries %q as its detail, and an archive or a restore reports a twelve-hex identifier", answer.Detail)
	}
	return answer.Detail
}

// pathOf resolves a reference through the tool itself, so no test here has to
// know where an entity's directory sits.
func pathOf(t *testing.T, root string, argv ...string) string {
	t.Helper()
	got := runCLI(t, root, append([]string{"path"}, argv...)...)
	if got.code != 0 {
		t.Fatalf("path %v: %d %s", argv, got.code, got.errw)
	}
	return strings.TrimSpace(got.out)
}

// treeUnder reads every file below a directory, keyed by its path relative to
// that directory, so two snapshots can be compared byte for byte. A restore
// that returned an empty directory fails a comparison against this where a
// check for the directory's existence would pass.
func treeUnder(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		files[filepath.ToSlash(relative)] = string(body)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return files
}

// journalEvents reads an ndjson journal and returns one entry per line, in the
// order the file carries them.
func journalEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var events []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("the journal line %q does not parse: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

// carriesInOrder reports whether the journal carries an archived line and then
// a restored line, each with note equal to the identifier given.
func carriesInOrder(events []map[string]any, id string) bool {
	archivedAt := -1
	for at, event := range events {
		if event["event"] == contract.EventArchived && event["note"] == id {
			archivedAt = at
		}
		if event["event"] == contract.EventRestored && event["note"] == id && archivedAt >= 0 && at > archivedAt {
			return true
		}
	}
	return false
}

// journalKey is the key treeUnder files a journal under when that journal sits
// inside the directory being compared, and a key no snapshot carries when it
// sits outside one. Deleting it from a snapshot is a no-op in the second case,
// which is what lets both cases run through one comparison.
func journalKey(t *testing.T, dir, journal string) string {
	t.Helper()
	relative, err := filepath.Rel(dir, journal)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(relative)
}

// archivableKind is one kind of the six a restore has to return, together with
// the reference that names it and the journal the pair of events lands on.
type archivableKind struct {
	// kind is the containment grammar's own word for it, which is what the
	// failure names so a reader knows which of the six broke.
	kind string
	// ref is the reference the archive and the restore are run against.
	ref string
	// journal is the path of the journal the two events land on, resolved
	// through the tool after the fixture is built.
	journal func(t *testing.T, root string) string
}

// TestRestoreIsTheInverseOfArchiveForEveryArchivableKind is dinah-461 AC-1.
//
// The subject set is named rather than counted, and the run fails when it
// covers fewer than six kinds, so a fixture that quietly stops creating one of
// them stops the run instead of passing for free.
//
// The accepting case is the whole of this test, which is why it stands ahead
// of the refusal checks below: a Restore that refused every reference would
// pass no arm of it.
func TestRestoreIsTheInverseOfArchiveForEveryArchivableKind(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card to archive"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "a card that holds things"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "fx-2", "a comment to archive"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "fx-2", "decision", "a decision to archive"); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	payload := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(payload, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if got := runCLI(t, root, "attach", "fx-2", payload); got.code != 0 {
		t.Fatalf("attach: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "workstream", "new", "A stream", "--slug", "stream"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "column", "new", "Spare", "--slug", "spare"); got.code != 0 {
		t.Fatalf("column new: %d %s", got.code, got.errw)
	}

	cardJournal := func(t *testing.T, root string) string { return pathOf(t, root, "fx-2/journal") }
	benchJournal := func(t *testing.T, root string) string {
		return filepath.Join(filepath.Dir(pathOf(t, root, "workbench")), bench.JournalName)
	}
	kinds := []archivableKind{
		{kind: bench.KindColumn, ref: "spare", journal: benchJournal},
		{kind: bench.KindCard, ref: "fx-1", journal: func(t *testing.T, root string) string {
			return pathOf(t, root, "fx-1/journal")
		}},
		{kind: bench.KindComment, ref: "fx-2/comments/1", journal: cardJournal},
		{kind: bench.KindItem, ref: "fx-2/decisions/1", journal: cardJournal},
		{kind: bench.KindAttachment, ref: "fx-2/attachments/1", journal: cardJournal},
		{kind: bench.KindWorkstream, ref: "workstream/stream", journal: func(t *testing.T, root string) string {
			return filepath.Join(pathOf(t, root, "workstream/stream"), bench.JournalName)
		}},
	}
	if len(kinds) < 6 {
		t.Fatalf("this check drives %d kinds and the six archivable kinds are the subject set", len(kinds))
	}

	for _, kind := range kinds {
		t.Run(kind.kind, func(t *testing.T) {
			// A card's own reference resolves to its anchor and a workstream's
			// to its directory, so the directory under comparison is taken
			// from whichever of the two the tool answered.
			resolved := pathOf(t, root, kind.ref)
			directory := resolved
			if info, err := os.Stat(resolved); err == nil && !info.IsDir() {
				directory = filepath.Dir(resolved)
			}
			// The journal the two events land on is inside this directory
			// for a card and for a workstream, and the format appends the
			// event before the move, so that one file is compared by the
			// events it carries rather than byte for byte. Every other file
			// has to come back unchanged.
			journal := kind.journal(t, root)
			before := treeUnder(t, directory)
			delete(before, journalKey(t, directory, journal))
			if len(before) == 0 {
				t.Fatalf("%s sits at %s and that directory holds no files beside its journal, so a comparison against it would pass for free", kind.kind, directory)
			}

			archived := runCLI(t, root, "archive", kind.ref, "--json")
			id := detailOfRun(t, archived)
			if _, err := os.Stat(directory); !os.IsNotExist(err) {
				t.Fatalf("%s is archived and %s is still there", kind.kind, directory)
			}

			restored := runCLI(t, root, "restore", kind.ref, "--json")
			if back := detailOfRun(t, restored); back != id {
				t.Errorf("the archive reported %s and the restore reported %s, and one entity carries one identifier", id, back)
			}
			after := treeUnder(t, directory)
			delete(after, journalKey(t, directory, journal))
			if len(after) != len(before) {
				t.Fatalf("%s came back with %d files and it went away with %d", kind.kind, len(after), len(before))
			}
			for name, body := range before {
				if after[name] != body {
					t.Errorf("%s came back with %s changed:\nwanted %q\ngot    %q", kind.kind, name, body, after[name])
				}
			}

			events := journalEvents(t, journal)
			if !carriesInOrder(events, id) {
				t.Errorf("the journal for %s carries no archived line followed by a restored line noting %s", kind.kind, id)
			}
		})
	}
}

// archivedHalfCommand is one of the four commands that take --archived,
// together with the argv the sweep runs it with. The flag rides on three of
// them and restore always reads the mirror, so the fourth takes none.
type archivedHalfCommand struct {
	name string
	argv func(ref string) []string
}

// archivedHalfCommands are the four. AC-2 drives every reference against every
// one of them, which is what makes the check see a command whose call chain
// never reaches the refusal: a criterion exercising restore alone cannot.
var archivedHalfCommands = []archivedHalfCommand{
	{name: "restore", argv: func(ref string) []string { return []string{"restore", ref} }},
	{name: "show", argv: func(ref string) []string { return []string{"show", "--archived", ref} }},
	{name: "path", argv: func(ref string) []string { return []string{"path", "--archived", ref} }},
	{name: "contents", argv: func(ref string) []string { return []string{"contents", "--archived", ref} }},
}

// archivedHalfCase is one reference the sweep drives, with the branch of the
// refusal's alternation it is expected to land on.
type archivedHalfCase struct {
	// name is what the failure calls this reference.
	name string
	// ref is the reference as typed.
	ref string
	// clauses are fragments the folded stderr must carry, one per command
	// keyed by command name, plus the empty key for the fragment every
	// command shares.
	clauses map[string]string
	// excluded are strings the folded stderr must not carry.
	excluded []string
}

// TestEveryArchivedHalfCommandRefusesALiveReference is dinah-461 AC-2.
//
// Three references against four commands is twelve runs, generated from the
// cross product of the two tables above rather than written out, so a command
// gaining or losing the flag changes the run count rather than leaving a stale
// hand-written row behind.
//
// Every match is on the refusal token plus one backticked command spelling or
// one short fragment rather than on a whole sentence, and the stderr is folded
// first, so a re-wrap at a different terminal width reports nothing.
func TestEveryArchivedHalfCommandRefusesALiveReference(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a live card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "fx-1", "a live comment"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}

	cases := []archivedHalfCase{
		{
			name: "a live card",
			ref:  "fx-1",
			clauses: map[string]string{
				"restore":  "`dinah archive fx-1`",
				"show":     "drop `--archived`",
				"path":     "drop `--archived`",
				"contents": "drop `--archived`",
			},
		},
		{
			name: "a live comment",
			ref:  "fx-1/comments/1",
			clauses: map[string]string{
				"": "`dinah show --archived fx-1/comments`",
			},
		},
		{
			name: "the workbench",
			ref:  "workbench",
			clauses: map[string]string{
				"": "is never archived",
			},
			// dinah archive workbench is refused by design, so a branch
			// advising it would advise an act the tool will not perform.
			// Asserting the absence as well as the presence is what makes
			// this arm a catch rather than a pin: a build rendering neither
			// clause fails it too.
			excluded: []string{"dinah archive workbench", "dinah archive ."},
		},
	}
	if len(cases) < 3 || len(archivedHalfCommands) < 4 {
		t.Fatalf("this sweep drives %d references against %d commands and the subject set is the three references against the four commands that take the archived half", len(cases), len(archivedHalfCommands))
	}

	ran := 0
	for _, subject := range cases {
		for _, command := range archivedHalfCommands {
			ran++
			got := runCLI(t, root, command.argv(subject.ref)...)
			folded := foldedStderr(got.errw)
			if got.code != 2 {
				t.Errorf("%s against %s exited %d and a refusal exits 2: %s", command.name, subject.name, got.code, folded)
				continue
			}
			if token := firstToken(got.errw); token != contract.NotArchived {
				t.Errorf("%s against %s refused %s and the refusal is %s: %s", command.name, subject.name, token, contract.NotArchived, folded)
				continue
			}
			want, named := subject.clauses[command.name]
			if !named {
				want = subject.clauses[""]
			}
			if want != "" && !strings.Contains(folded, want) {
				t.Errorf("%s against %s carries no %q: %s", command.name, subject.name, want, folded)
			}
			for _, absent := range subject.excluded {
				if strings.Contains(folded, absent) {
					t.Errorf("%s against %s advises %q, which the tool refuses by design: %s", command.name, subject.name, absent, folded)
				}
			}

			// The base sentence is split by act, so a restore never reads the
			// sentence written for somebody who asked to look, and no read
			// ever reads the one written for somebody who asked to restore.
			restoreSense := strings.Contains(folded, "is not archived, so there is nothing to restore")
			readSense := strings.Contains(folded, "nothing in the archive answers to")
			if command.name == "restore" && (!restoreSense || readSense) {
				t.Errorf("restore against %s reads in the wrong sense: %s", subject.name, folded)
			}
			if command.name != "restore" && (restoreSense || !readSense) {
				t.Errorf("%s against %s reads in the wrong sense: %s", command.name, subject.name, folded)
			}
		}
	}
	if want := len(cases) * len(archivedHalfCommands); ran != want {
		t.Fatalf("the sweep ran %d invocations and the two tables cross to %d", ran, want)
	}
	t.Logf("%d invocations ran, %d references against %d commands", ran, len(cases), len(archivedHalfCommands))
}

// TestRestoreAcceptsAnArchivedReference is the accepting half AC-2 pairs with
// the refusing one, so a build that refused every reference cannot satisfy the
// refusal sweep above on its own.
func TestRestoreAcceptsAnArchivedReference(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "fx-1", "a comment"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "archive", "fx-1/comments/1"); got.code != 0 {
		t.Fatalf("archive the comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "restore", "fx-1/comments/1"); got.code != 0 {
		t.Errorf("restoring the archived comment exited %d: %s", got.code, foldedStderr(got.errw))
	}
	if got := runCLI(t, root, "archive", "fx-1"); got.code != 0 {
		t.Fatalf("archive the card: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "restore", "fx-1"); got.code != 0 {
		t.Errorf("restoring the archived card exited %d: %s", got.code, foldedStderr(got.errw))
	}
}

// TestRestoringAColumnReturnsItToTheOrderAndRepairsAStrandedCard is
// dinah-461 AC-7.
//
// The second arm is what stops the first passing against a build that refuses
// every restore of an occupied column, which is exactly the case a stranded
// card creates and exactly the repair this command is for.
func TestRestoringAColumnReturnsItToTheOrderAndRepairsAStrandedCard(t *testing.T) {
	if err := guidepin.Carries("references", guidepin.ARestoredColumnLandsAtTheEndOfTheOrder); err != nil {
		t.Error(err)
	}
	root := newBench(t)
	if got := runCLI(t, root, "column", "new", "Spare", "--slug", "spare"); got.code != 0 {
		t.Fatalf("column new: %d %s", got.code, got.errw)
	}
	anchor := filepath.Join(filepath.Dir(pathOf(t, root, "workbench")), bench.WorkbenchAnchor)
	spareID := filepath.Base(filepath.Dir(pathOf(t, root, "spare")))
	if !bench.IsID(spareID) {
		t.Fatalf("the spare column resolves to %q, which is no identifier", spareID)
	}

	if got := runCLI(t, root, "archive", "spare"); got.code != 0 {
		t.Fatalf("archive spare: %d %s", got.code, got.errw)
	}
	if listing := runCLI(t, root, "columns"); strings.Contains(listing.out, "spare") {
		t.Errorf("spare is archived and `dinah columns` still lists it:\n%s", listing.out)
	}
	if body, err := os.ReadFile(anchor); err != nil {
		t.Fatalf("read the anchor: %v", err)
	} else if strings.Contains(string(body), spareID) {
		t.Errorf("spare is archived and the anchor's columns sequence still names %s", spareID)
	}

	if got := runCLI(t, root, "restore", "spare"); got.code != 0 {
		t.Fatalf("restore spare: %d %s", got.code, got.errw)
	}
	listing := runCLI(t, root, "columns")
	rows := strings.Split(strings.TrimRight(listing.out, "\n"), "\n")
	if len(rows) == 0 || !strings.Contains(rows[len(rows)-1], "spare") {
		t.Errorf("a restored column lands at the end of the order and the listing ends with %q:\n%s", rows[len(rows)-1], listing.out)
	}
	body, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	sequence := columnSequenceOf(t, string(body))
	if len(sequence) == 0 || sequence[len(sequence)-1] != spareID {
		t.Errorf("the anchor's columns sequence is %v and it ends with %s", sequence, spareID)
	}

	// The second arm. A live card naming a column the workbench does not list
	// is the stranded state check reports, and restoring the column is the
	// repair, so the occupancy scan archive runs must not fire here.
	//
	// It is here because the first arm alone passes against a build that
	// refuses every restore of an occupied column, which is exactly the case
	// a stranded card creates.
	if got := runCLI(t, root, "add", "a card that will be stranded"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "spare"); got.code != 0 {
		t.Fatalf("move fx-1 to spare: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "archive", "spare"); got.code == 0 {
		t.Fatal("archiving a column a live card stands in was accepted, and the occupancy scan is what stops it")
	}
	// The card is carried out of the way so the column can be archived, and
	// its anchor is then written to name the archived column by hand, which
	// is the state the pre-restore tool could reach and could not repair.
	if got := runCLI(t, root, "move", "fx-1", "intake"); got.code != 0 {
		t.Fatalf("move fx-1 back to intake: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "archive", "spare"); got.code != 0 {
		t.Fatalf("archive the now-empty spare: %d %s", got.code, got.errw)
	}
	pointCardAtColumn(t, root, "fx-1", spareID)
	if !checkReports(t, root, bench.FindingUnknownColumn) {
		t.Fatal("the fixture did not strand the card, so the repair below would prove nothing")
	}
	if got := runCLI(t, root, "restore", "spare"); got.code != 0 {
		t.Errorf("restoring the column a stranded card names exited %d, and refusing the repair because the damage exists is the shape this arm catches: %s",
			got.code, foldedStderr(got.errw))
	}
	if checkReports(t, root, bench.FindingUnknownColumn) {
		t.Error("the restore left the card stranded")
	}
}

// checkReports reports whether `dinah check` names one finding key. The key is
// read off the machine answer rather than off the rendered sentence, so the
// question asked is which finding the tool raised rather than which words it
// chose to print it in.
func checkReports(t *testing.T, root, key string) bool {
	t.Helper()
	got := runCLI(t, root, "check", "--json")
	var answer struct {
		Findings []struct {
			Key string `json:"key"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(got.out), &answer); err != nil {
		t.Fatalf("the check answer does not parse: %v\n%s", err, got.out)
	}
	for _, finding := range answer.Findings {
		if finding.Key == key {
			return true
		}
	}
	return false
}

// columnSequenceOf reads the identifiers of the anchor's columns sequence, in
// the order the file carries them.
func columnSequenceOf(t *testing.T, anchor string) []string {
	t.Helper()
	var sequence []string
	inside := false
	for _, line := range strings.Split(anchor, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "columns:") {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		sequence = append(sequence, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
	}
	return sequence
}

// pointCardAtColumn writes a card's anchor to name a column identifier by
// hand, which is the stranded state a column archived out from under a card
// leaves and the one `dinah check` reports as an unknown column. It is the
// other half of main_test.go's strandColumn, which strands the column rather
// than the card, and neither can be written in terms of the other.
func pointCardAtColumn(t *testing.T, root, card, column string) {
	t.Helper()
	anchor := pathOf(t, root, card)
	body, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read %s: %v", anchor, err)
	}
	lines := strings.Split(string(body), "\n")
	written := false
	for at, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "column:") {
			continue
		}
		lines[at] = "column: " + column
		written = true
		break
	}
	if !written {
		t.Fatalf("%s carries no column key, so this fixture could not strand it", anchor)
	}
	if err := os.WriteFile(anchor, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", anchor, err)
	}
}

// TestRestoreRefusedByAnOccupiedSlotRendersItsOwnSentence is dinah-461 AC-8.
//
// The second arm is here because the first alone is satisfied by breaking the
// sentence init depends on, which is the cheapest wrong way to pass it.
func TestRestoreRefusedByAnOccupiedSlotRendersItsOwnSentence(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "fx-1", "a comment"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	target := filepath.Dir(pathOf(t, root, "fx-1/comments/1"))
	if got := runCLI(t, root, "archive", "fx-1/comments/1"); got.code != 0 {
		t.Fatalf("archive: %d %s", got.code, got.errw)
	}
	// Something live stands where the archived comment goes back, built by
	// hand under the archived comment's own identifier, which is the only way
	// to reach a refusal identifiers do not otherwise collide on.
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", target, err)
	}
	if err := os.WriteFile(filepath.Join(target, bench.CommentAnchor), []byte("---\nts: 2026-01-01T00:00:00Z\n---\nsomething else\n"), 0o644); err != nil {
		t.Fatalf("write the standing comment: %v", err)
	}

	got := runCLI(t, root, "restore", "fx-1/comments/1")
	folded := foldedStderr(got.errw)
	if token := firstToken(got.errw); token != contract.Exists {
		t.Fatalf("the restore refused %s and an occupied slot refuses %s: %s", token, contract.Exists, folded)
	}
	if !strings.Contains(folded, "has nowhere to go back to") {
		t.Errorf("the refusal carries no restore-sense fragment: %s", folded)
	}
	for _, absent := range []string{"workbench.md", "choose a different directory"} {
		if strings.Contains(folded, absent) {
			t.Errorf("the refusal carries %q, which is written for init and extract: %s", absent, folded)
		}
	}

	// init still renders the sentence it has always rendered, so the variant
	// cannot be landed by rewriting the shared entry.
	again := runCLI(t, root, "init", filepath.Dir(pathOf(t, root, "workbench")), "--slug", "second", "--operator", "alka")
	initFolded := foldedStderr(again.errw)
	if token := firstToken(again.errw); token != contract.Exists {
		t.Fatalf("a second init refused %s and it refuses %s: %s", token, contract.Exists, initFolded)
	}
	if !strings.Contains(initFolded, "workbench.md") {
		t.Errorf("a second init no longer names workbench.md, so the shared entry was rewritten: %s", initFolded)
	}
}

// TestAnArchivedReadShowsOneHalfAndWritesNothing is dinah-461 AC-10.
func TestAnArchivedReadShowsOneHalfAndWritesNothing(t *testing.T) {
	if err := guidepin.Carries("references", guidepin.AnArchivedContentsRowIsTheAddressAfterRestore); err != nil {
		t.Error(err)
	}
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card with comments"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, body := range []string{"first comment", "second comment", "third comment"} {
		if got := runCLI(t, root, "comment", "fx-1", body); got.code != 0 {
			t.Fatalf("comment: %d %s", got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "archive", "fx-1/comments/3"); got.code != 0 {
		t.Fatalf("archive the third comment: %d %s", got.code, got.errw)
	}

	// Asserting the flagged count alone passes against a build that returns
	// the archived half twice, and asserting only that the two differ passes
	// against a build that returns nothing under the flag, so both counts are
	// asserted and so is the absence of any shared member.
	archived := collectionRefs(t, runCLI(t, root, "show", "--archived", "fx-1/comments", "--json"))
	live := collectionRefs(t, runCLI(t, root, "show", "fx-1/comments", "--json"))
	if len(archived) != 1 {
		t.Errorf("the flagged listing carries %d members and the archive holds one: %v", len(archived), archived)
	}
	if len(live) != 2 {
		t.Errorf("the unflagged listing carries %d members and the live half holds two: %v", len(live), live)
	}
	archivedText := collectionTexts(t, runCLI(t, root, "show", "--archived", "fx-1/comments", "--json"))
	liveText := collectionTexts(t, runCLI(t, root, "show", "fx-1/comments", "--json"))
	for _, text := range archivedText {
		for _, other := range liveText {
			if text == other {
				t.Errorf("the two listings share a member, and the flag names one half rather than widening: %q", text)
			}
		}
	}

	// A read under the flag writes nothing to an archived card's journal. An
	// archived card is out of the flow by construction, so expiring its claim
	// would be a write to history nobody asked for.
	if got := runCLI(t, root, "add", "a card whose claim lapses"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-2")
	if got := runCLI(t, root, "claim", "fx-2", "--expires", "1ms"); got.code != 0 {
		t.Fatalf("claim: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "archive", "fx-2"); got.code != 0 {
		t.Fatalf("archive fx-2: %d %s", got.code, got.errw)
	}
	journal := pathOf(t, root, "--archived", "fx-2/journal")
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the archived journal: %v", err)
	}
	if got := runCLI(t, root, "show", "--archived", "fx-2"); got.code != 0 {
		t.Fatalf("show --archived fx-2: %d %s", got.code, got.errw)
	}
	after, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the archived journal again: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("reading an archived card wrote to its journal: %d bytes before, %d after", len(before), len(after))
	}

	// contents says, once, that the addresses below the root do not resolve
	// yet, and the machine view carries the half on the tree.
	machine := runCLI(t, root, "contents", "--archived", "fx-2", "--json")
	if machine.code != 0 {
		t.Fatalf("contents --archived --json: %d %s", machine.code, machine.errw)
	}
	var tree struct {
		Archived bool `json:"archived"`
		Root     struct {
			Ref string `json:"ref"`
		} `json:"root"`
	}
	if err := json.Unmarshal([]byte(machine.out), &tree); err != nil {
		t.Fatalf("the tree does not parse: %v\n%s", err, machine.out)
	}
	if !tree.Archived {
		t.Error("the tree of an archived read does not carry archived true")
	}
	human := runCLI(t, root, "contents", "--archived", "fx-2")
	if human.code != 0 {
		t.Fatalf("contents --archived: %d %s", human.code, human.errw)
	}
	// The guide says the notice stands on the line under the sentence naming
	// the root, so the assertion reads that line rather than the whole
	// listing. A strings.Contains over the whole output passes wherever the
	// notice appears, including above the sentence it is meant to follow,
	// which is the shape the guide's own previous wording got wrong.
	lines := strings.Split(strings.TrimRight(human.out, "\r\n"), "\n")
	notice := "resolve once " + tree.Root.Ref + " is restored"
	if len(lines) < 2 || !strings.Contains(lines[1], notice) {
		second := "(the listing has no second line)"
		if len(lines) > 1 {
			second = lines[1]
		}
		t.Errorf("the references guide says the archived listing carries its notice on the line under the sentence naming the root; the second line reads %q and the whole listing is:\n%s", second, human.out)
	}
}

// collectionRefs reads the member references out of a machine-format
// collection listing.
func collectionRefs(t *testing.T, got invocation) []string {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("the listing exited %d: %s", got.code, got.errw)
	}
	var listing struct {
		Members []struct {
			Ref string `json:"ref"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(got.out), &listing); err != nil {
		t.Fatalf("the listing does not parse: %v\n%s", err, got.out)
	}
	refs := make([]string, 0, len(listing.Members))
	for _, member := range listing.Members {
		refs = append(refs, member.Ref)
	}
	return refs
}

// collectionTexts reads the member anchors out of a machine-format collection
// listing. A reference names a position and a position is counted per half, so
// the two halves are compared on what the members say rather than on what they
// are called.
func collectionTexts(t *testing.T, got invocation) []string {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("the listing exited %d: %s", got.code, got.errw)
	}
	var listing struct {
		Members []struct {
			Text string `json:"text"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(got.out), &listing); err != nil {
		t.Fatalf("the listing does not parse: %v\n%s", err, got.out)
	}
	texts := make([]string, 0, len(listing.Members))
	for _, member := range listing.Members {
		texts = append(texts, member.Text)
	}
	return texts
}

// TestANestedArchiveRestoresInTwoActs is dinah-461 AC-11.
//
// The comment's body is captured before the first archive and compared at the
// end, because a check asserting that a directory is present proves presence
// rather than correctness.
func TestANestedArchiveRestoresInTwoActs(t *testing.T) {
	if err := guidepin.Carries("references", guidepin.AnEntityComesBackWithItsHolder); err != nil {
		t.Error(err)
	}
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card that holds a comment"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, body := range []string{"the comment that travels", "a comment that stays"} {
		if got := runCLI(t, root, "comment", "fx-1", body); got.code != 0 {
			t.Fatalf("comment: %d %s", got.code, got.errw)
		}
	}
	travellerText := collectionTexts(t, runCLI(t, root, "show", "fx-1/comments", "--json"))[0]

	if got := runCLI(t, root, "archive", "fx-1/comments/1"); got.code != 0 {
		t.Fatalf("archive the comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "archive", "fx-1"); got.code != 0 {
		t.Fatalf("archive the card: %d %s", got.code, got.errw)
	}

	// The head is not the reference's deepest collection step, so it resolves
	// live and fails; the refusal probe then finds the comment by reading the
	// mirror at the head and points at the holder.
	got := runCLI(t, root, "show", "--archived", "fx-1/comments/1")
	folded := foldedStderr(got.errw)
	if token := firstToken(got.errw); token != contract.NotArchived {
		t.Fatalf("the read refused %s and it refuses %s: %s", token, contract.NotArchived, folded)
	}
	if !strings.Contains(folded, "nothing in the archive answers to") {
		t.Errorf("the read does not refuse in the read sense: %s", folded)
	}
	if !strings.Contains(folded, "`dinah restore fx-1`") {
		t.Errorf("the read does not name the holder to restore: %s", folded)
	}
	for _, absent := range []string{"nothing to restore", "dinah show --archived"} {
		if strings.Contains(folded, absent) {
			t.Errorf("the read carries %q, which belongs to another branch: %s", absent, folded)
		}
	}

	if back := runCLI(t, root, "restore", "fx-1"); back.code != 0 {
		t.Fatalf("restore the card: %d %s", back.code, foldedStderr(back.errw))
	}
	// The card comes back carrying its own mirror, because the archive is
	// local to its holder and the whole directory travelled.
	inner := filepath.Join(filepath.Dir(pathOf(t, root, "fx-1")), bench.ArchiveDir, bench.CommentsDir)
	entries, err := os.ReadDir(inner)
	if err != nil {
		t.Fatalf("the restored card carries no %s: %v", inner, err)
	}
	if len(entries) != 1 {
		t.Fatalf("the restored card's own mirror holds %d entries and it held one", len(entries))
	}
	if back := runCLI(t, root, "restore", "fx-1/comments/1"); back.code != 0 {
		t.Fatalf("restore the comment: %d %s", back.code, foldedStderr(back.errw))
	}

	texts := collectionTexts(t, runCLI(t, root, "show", "fx-1/comments", "--json"))
	if len(texts) != 2 {
		t.Fatalf("the card carries %d comments and both came back", len(texts))
	}
	found := false
	for _, text := range texts {
		if text == travellerText {
			found = true
		}
	}
	if !found {
		t.Errorf("the restored comment's body is not what was written before either archive:\nwanted %q\ngot    %v", travellerText, texts)
	}
}
