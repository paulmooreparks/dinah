package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// The tests below drive the whole program through run rather than its parts,
// because the thing under test is a one-shot command whose answer is an exit
// code and a report, and a caller reading either gets both together.
//
// Every fixture is built in a temporary directory. No test here opens a path
// outside t.TempDir, because the program's whole purpose is to rewrite journals
// in place and a fixture reaching live data would rewrite it.

// planted is one journal a fixture carries: where it goes, and the lines it
// holds before the run.
type planted struct {
	// path is relative to the workbench root.
	path string
	// lines are the journal's lines, without their newlines.
	lines []string
}

// plantStore writes a workbench carrying the journals a test names, and returns
// its root. The anchor declares format 4, which is the format every store this
// program is written for declares.
func plantStore(t *testing.T, journals ...planted) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workbench")
	anchor := strings.Join([]string{
		"---",
		"format: 4",
		"profile: dinah-core/0.16",
		"title: Fixture",
		"operator: ana",
		"columns:",
		"  - c00000000001",
		"---",
		"",
		"Standing text.",
		"",
	}, "\n")
	write(t, filepath.Join(root, bench.WorkbenchAnchor), anchor)
	for _, journal := range journals {
		body := ""
		if len(journal.lines) > 0 {
			body = strings.Join(journal.lines, "\n") + "\n"
		}
		write(t, filepath.Join(root, filepath.FromSlash(journal.path)), body)
	}
	return root
}

// write creates a file and every directory above it.
func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// captured is what one run answered: its exit code and both its streams.
type captured struct {
	code int
	out  string
	errw string
}

// drive runs the program with its two streams redirected to temporary files,
// which is what lets run keep taking *os.File and still be readable from a
// test.
func drive(t *testing.T, args ...string) captured {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("create the output file: %v", err)
	}
	errw, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatalf("create the error file: %v", err)
	}
	code := run(args, out, errw)
	out.Close()
	errw.Close()
	return captured{code: code, out: read(t, filepath.Join(dir, "out")), errw: read(t, filepath.Join(dir, "err"))}
}

// read is the whole of a file as text.
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// snapshot is every file under a directory, keyed by its path relative to that
// directory, which is what a before-and-after comparison is made of.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	held := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		held[filepath.ToSlash(relative)] = read(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return held
}

// cardJournal is where a card's journal sits, which most fixtures need one of.
func cardJournal(id string) string {
	return bench.CardsDir + "/" + id + "/" + bench.JournalName
}

// TestAPreviewClassifiesAndWritesNothing drives dinah-496's preview criterion.
// A run without --apply reports the three populations separately and leaves the
// store byte for byte as it stood, and a store with nothing to migrate reports
// zero string actors beside a non-zero count of journals read, so it cannot be
// read as a pass that read nothing.
func TestAPreviewClassifiesAndWritesNothing(t *testing.T) {
	root := plantStore(t, planted{path: cardJournal("c00000000001"), lines: []string{
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana","title":"One"}`,
		`{"ts":"2026-09-15T09:01:00Z","event":"claimed","actor":{"name":"claude"}}`,
	}})
	before := snapshot(t, root)

	got := drive(t, root)
	if got.code != exitClassified {
		t.Fatalf("the preview exited %d, wanted %d: %s%s", got.code, exitClassified, got.out, got.errw)
	}
	for _, sentence := range []string{
		"1 lines carry a string actor.",
		"1 lines already carry an object.",
		"0 journals could not be read.",
		"Read 1 journals",
	} {
		if !strings.Contains(got.out, sentence) {
			t.Errorf("the report does not carry %q:\n%s", sentence, got.out)
		}
	}
	if after := snapshot(t, root); !sameStore(before, after) {
		t.Errorf("the preview wrote to the store:\n%v", differences(before, after))
	}

	// A store already across reports the same two counts, and the count of
	// journals read is what tells it from a run that read nothing at all.
	crossed := plantStore(t, planted{path: cardJournal("c00000000001"), lines: []string{
		`{"ts":"2026-09-15T09:01:00Z","event":"claimed","actor":{"name":"claude"}}`,
	}})
	done := drive(t, crossed)
	if done.code != exitDone {
		t.Fatalf("a store with nothing to migrate exited %d, wanted %d: %s%s", done.code, exitDone, done.out, done.errw)
	}
	if !strings.Contains(done.out, "0 lines carry a string actor.") {
		t.Errorf("the report does not say that nothing needs migrating:\n%s", done.out)
	}
	if !strings.Contains(done.out, "Read 1 journals") {
		t.Errorf("the report does not say how many journals it read, so it reads like a pass that read nothing:\n%s", done.out)
	}
}

// TestAConflictStopsTheRunBeforeAnyWrite drives the all-or-nothing posture.
// Three conditions stand in one store, every one is reported rather than the
// first, and a byte-for-byte comparison of the whole store before and after
// asserts that nothing was written, the format stamp included.
func TestAConflictStopsTheRunBeforeAnyWrite(t *testing.T) {
	root := plantStore(t,
		planted{path: cardJournal("c00000000001"), lines: []string{
			`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana"}`,
			`{"ts":"2026-09-15T09:01:00Z","event":"claimed","actor":"ana"`,
		}},
		planted{path: cardJournal("c00000000002"), lines: []string{
			`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":42}`,
		}},
		planted{path: cardJournal("c00000000003"), lines: []string{
			`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana"}`,
		}},
	)
	// The unreadable journal is a directory standing where a file belongs,
	// which os.ReadFile refuses on every platform this tool runs on.
	unreadable := filepath.Join(root, filepath.FromSlash(cardJournal("c00000000004")))
	if err := os.MkdirAll(unreadable, 0o755); err != nil {
		t.Fatalf("plant the unreadable journal: %v", err)
	}
	before := snapshot(t, root)

	got := drive(t, root, "--apply")
	if got.code != exitConflicts {
		t.Fatalf("a run meeting conflicts exited %d, wanted %d: %s%s", got.code, exitConflicts, got.out, got.errw)
	}
	for _, token := range []string{tornJournal, actorNotString, unreadable2()} {
		if !strings.Contains(got.out, token) {
			t.Errorf("the report names no %s conflict, so it stopped at the first:\n%s", token, got.out)
		}
	}
	if !strings.Contains(got.out, "3 conflicts:") {
		t.Errorf("the report does not count the conflicts it found:\n%s", got.out)
	}
	after := snapshot(t, root)
	if !sameStore(before, after) {
		t.Errorf("a refused run wrote to the store:\n%v", differences(before, after))
	}
	if !strings.Contains(after[bench.WorkbenchAnchor], "format: 4") {
		t.Errorf("the format stamp moved on a run that wrote nothing:\n%s", after[bench.WorkbenchAnchor])
	}
}

// unreadable2 names the unreadable token without shadowing the constant, which
// a range variable of the same name would.
func unreadable2() string { return unreadable }

// TestTheWritePassReEncodesTheActorAndNothingElse drives CORE-HIST-7 and the
// promise the whole mechanism exists for: every string actor becomes an object carrying name and
// nothing else, the line order stands, the format stamp is written last, every
// rewritten line parses, and every byte outside the actor value survives.
//
// The fixture is chosen for what it would break. One line carries a member
// bench.Event does not declare, one carries a key order the struct would not
// produce, and one carries an escaped quotation mark inside a note.
func TestTheWritePassReEncodesTheActorAndNothingElse(t *testing.T) {
	lines := []string{
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana","spice":"a member this build does not declare"}`,
		`{"actor":"ana","event":"claimed","ts":"2026-09-15T09:01:00Z"}`,
		`{"ts":"2026-09-15T09:02:00Z","event":"commented","actor":"ana","note":"she said \"no\" twice"}`,
	}
	root := plantStore(t, planted{path: cardJournal("c00000000001"), lines: lines})
	got := drive(t, root, "--apply")
	if got.code != exitDone {
		t.Fatalf("the write pass exited %d, wanted %d: %s%s", got.code, exitDone, got.out, got.errw)
	}
	rewritten := strings.Split(strings.TrimRight(read(t, filepath.Join(root, filepath.FromSlash(cardJournal("c00000000001")))), "\n"), "\n")
	if len(rewritten) != len(lines) {
		t.Fatalf("the journal holds %d lines, wanted %d:\n%s", len(rewritten), len(lines), strings.Join(rewritten, "\n"))
	}
	compared := 0
	for at, line := range rewritten {
		compared++
		if !json.Valid([]byte(line)) {
			t.Errorf("line %d does not parse after the rewrite: %s", at+1, line)
			continue
		}
		if !strings.Contains(line, `"actor":{"name":"ana"}`) {
			t.Errorf("line %d does not carry the re-encoded actor: %s", at+1, line)
		}
		if outside(t, line) != outside(t, lines[at]) {
			t.Errorf("line %d changed outside the actor value:\n before %s\n  after %s", at+1, lines[at], line)
		}
	}
	if compared != len(lines) {
		t.Fatalf("the test compared %d lines, wanted %d", compared, len(lines))
	}
	if !strings.Contains(read(t, filepath.Join(root, bench.WorkbenchAnchor)), "format: 5") {
		t.Error("the run did not stamp the new format on the workbench anchor")
	}
}

// outside is a line with its actor member's own value cut out, which is what a
// comparison of everything the rewrite promised to leave alone is made of.
func outside(t *testing.T, line string) string {
	t.Helper()
	at, refused := actorSpan(line)
	if refused != nil {
		t.Fatalf("the line carries no readable actor span: %s", line)
	}
	return line[:at.start] + line[at.end:]
}

// TestTheSpanIsTakenAfterTheReadAndIsWhitespaceProof drives the mechanism the
// contract fixes: the span is the offset after the read minus the length of the
// decoded raw message.
//
// Four shapes are covered, and each is one the wrong order or a tighter rule
// would break. The actor first among the members, the actor last, a space on
// each side of the colon before the value, and an escaped quotation mark inside
// a neighbouring member. All four are rewritten, all four parse, and in all
// four every byte outside the actor value is identical, the spaces included.
func TestTheSpanIsTakenAfterTheReadAndIsWhitespaceProof(t *testing.T) {
	lines := []string{
		`{"actor":"ana","ts":"2026-09-15T09:00:00Z","event":"created"}`,
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana"}`,
		`{"ts":"x","actor" : "claude","note":"n"}`,
		`{"ts":"x","actor":"ana","note":"she said \"no\""}`,
	}
	root := plantStore(t, planted{path: cardJournal("c00000000001"), lines: lines})
	got := drive(t, root, "--apply")
	if got.code != exitDone {
		t.Fatalf("the write pass exited %d, wanted %d: %s%s", got.code, exitDone, got.out, got.errw)
	}
	rewritten := strings.Split(strings.TrimRight(read(t, filepath.Join(root, filepath.FromSlash(cardJournal("c00000000001")))), "\n"), "\n")
	if len(rewritten) != 4 {
		t.Fatalf("the journal holds %d lines, wanted 4:\n%s", len(rewritten), strings.Join(rewritten, "\n"))
	}
	for at, line := range rewritten {
		if !json.Valid([]byte(line)) {
			t.Errorf("shape %d does not parse after the rewrite: %s", at+1, line)
			continue
		}
		if outside(t, line) != outside(t, lines[at]) {
			t.Errorf("shape %d changed outside the actor value:\n before %s\n  after %s", at+1, lines[at], line)
		}
	}
	// The spaced line is the one an earlier draft of this contract called a
	// conflict, so it is asserted by itself: it is rewritten, and the spaces
	// stand where they stood.
	if want := `{"ts":"x","actor" : {"name":"claude"},"note":"n"}`; rewritten[2] != want {
		t.Errorf("the spaced line came out as %s, wanted %s", rewritten[2], want)
	}
}

// TestEveryJournalPopulationIsReached asserts that the walk reaches all five
// places a journal lives: a card's, a workstream's, the workbench's own, an
// archived card's and an archived workstream's.
func TestEveryJournalPopulationIsReached(t *testing.T) {
	line := `{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana"}`
	root := plantStore(t,
		planted{path: cardJournal("c00000000001"), lines: []string{line}},
		planted{path: bench.WorkstreamsDir + "/a00000000001/" + bench.JournalName, lines: []string{line}},
		planted{path: bench.JournalName, lines: []string{line}},
		planted{path: bench.ArchiveDir + "/" + bench.CardsDir + "/c00000000002/" + bench.JournalName, lines: []string{line}},
		planted{path: bench.ArchiveDir + "/" + bench.WorkstreamsDir + "/a00000000002/" + bench.JournalName, lines: []string{line}},
	)
	got := drive(t, root, "--apply")
	if got.code != exitDone {
		t.Fatalf("the write pass exited %d, wanted %d: %s%s", got.code, exitDone, got.out, got.errw)
	}
	if !strings.Contains(got.out, "Rewrote 5 journals") {
		t.Errorf("the report does not say it rewrote five journals:\n%s", got.out)
	}
	if !strings.Contains(got.out, "5 lines re-encoded") {
		t.Errorf("the report does not say it re-encoded five lines:\n%s", got.out)
	}
	held := snapshot(t, root)
	reached := 0
	for path, text := range held {
		if !strings.HasSuffix(path, bench.JournalName) {
			continue
		}
		reached++
		if !strings.Contains(text, `"actor":{"name":"ana"}`) {
			t.Errorf("%s was not re-encoded: %s", path, text)
		}
	}
	if reached != 5 {
		t.Errorf("the walk reached %d journals, wanted the five populations", reached)
	}
}

// TestNoCommentAnchorIsRewritten asserts that a comment's own actor field is
// left exactly as it stands. A comment records who wrote a piece of prose; the
// act that wrote it is on the journal, where the record of what was acting
// belongs.
func TestNoCommentAnchorIsRewritten(t *testing.T) {
	anchors := map[string]string{
		bench.CardsDir + "/c00000000001/comments/m00000000001/comment.md": "---\nts: 2026-09-15T09:00:00Z\nactor: ana\nordinal: 1\n---\n\nFirst.\n",
		bench.CardsDir + "/c00000000001/comments/m00000000002/comment.md": "---\nts: 2026-09-15T09:01:00Z\nactor: ana\nordinal: 2\n---\n\nSecond.\n",
	}
	root := plantStore(t, planted{path: cardJournal("c00000000001"), lines: []string{
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana"}`,
	}})
	for path, text := range anchors {
		write(t, filepath.Join(root, filepath.FromSlash(path)), text)
	}
	got := drive(t, root, "--apply")
	if got.code != exitDone {
		t.Fatalf("the write pass exited %d, wanted %d: %s%s", got.code, exitDone, got.out, got.errw)
	}
	compared := 0
	for path, text := range anchors {
		compared++
		if after := read(t, filepath.Join(root, filepath.FromSlash(path))); after != text {
			t.Errorf("%s was rewritten:\n before %q\n  after %q", path, text, after)
		}
	}
	if compared != 2 {
		t.Fatalf("the test compared %d comment anchors, wanted 2", compared)
	}
	journal := read(t, filepath.Join(root, filepath.FromSlash(cardJournal("c00000000001"))))
	if !strings.Contains(journal, `"actor":{"name":"ana"}`) {
		t.Errorf("the journal was not re-encoded while the comments were left alone: %s", journal)
	}
}

// TestThePathNamesOneWorkbench asserts the two ways a path names one workbench
// and the two ways it names none: a workbench directory names itself, a
// container holding exactly one names that one, a container holding several is
// refused with every workbench directory listed, and a path naming no workbench
// at all is refused.
func TestThePathNamesOneWorkbench(t *testing.T) {
	inner := plantStore(t, planted{path: cardJournal("c00000000001"), lines: []string{
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":{"name":"ana"}}`,
	}})
	if got := drive(t, inner); got.code != exitDone {
		t.Errorf("a workbench directory exited %d, wanted %d: %s%s", got.code, exitDone, got.out, got.errw)
	}

	// A container holding one workbench names that workbench.
	container := filepath.Join(t.TempDir(), bench.UserBaseName)
	sole := filepath.Join(container, "4fda9c9ca7794b38abe7ed6f0789fd2d")
	copyTree(t, inner, sole)
	if got := drive(t, container); got.code != exitDone {
		t.Errorf("a container holding one workbench exited %d, wanted %d: %s%s", got.code, exitDone, got.out, got.errw)
	}

	// A container holding several is refused, and the refusal lists them.
	for _, id := range []string{"6c5b9d6f4414b69abf918c42aa6cd1c6", "c9428b3bc9210d86ad99cdbc5729d457"} {
		copyTree(t, inner, filepath.Join(container, id))
	}
	refused := drive(t, container)
	if refused.code != exitUnusable {
		t.Fatalf("a container holding three workbenches exited %d, wanted %d: %s%s", refused.code, exitUnusable, refused.out, refused.errw)
	}
	listed := 0
	for _, line := range strings.Split(refused.errw, "\n") {
		if strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" {
			listed++
		}
	}
	if listed != 3 {
		t.Errorf("the refusal listed %d workbench directories, wanted 3:\n%s", listed, refused.errw)
	}

	// A path naming no workbench is refused too.
	if got := drive(t, t.TempDir()); got.code != exitUnusable {
		t.Errorf("a path naming no workbench exited %d, wanted %d: %s%s", got.code, exitUnusable, got.out, got.errw)
	}
}

// copyTree copies a directory recursively, which is how the container fixture
// above stands three workbenches up from one.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		write(t, target, read(t, path))
		return nil
	})
	if err != nil {
		t.Fatalf("copy %s to %s: %v", from, to, err)
	}
}

// TestTheFourExitCodesAreDistinctAndLeaveOneAndTwoToTheRuntime drives all four
// outcomes and asserts that none of them is 1 or 2, which the Go runtime
// reserves for a fatal log and a panic.
func TestTheFourExitCodesAreDistinctAndLeaveOneAndTwoToTheRuntime(t *testing.T) {
	clean := plantStore(t, planted{path: cardJournal("c00000000001"), lines: []string{
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":{"name":"ana"}}`,
	}})
	pending := plantStore(t, planted{path: cardJournal("c00000000001"), lines: []string{
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana"}`,
	}})
	torn := plantStore(t, planted{path: cardJournal("c00000000001"), lines: []string{
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana"`,
	}})

	answers := map[string]int{
		"finished":   drive(t, clean).code,
		"classified": drive(t, pending).code,
		"conflicts":  drive(t, torn, "--apply").code,
		"unusable":   drive(t, t.TempDir()).code,
	}
	wanted := map[string]int{
		"finished":   exitDone,
		"classified": exitClassified,
		"conflicts":  exitConflicts,
		"unusable":   exitUnusable,
	}
	for outcome, code := range answers {
		if code != wanted[outcome] {
			t.Errorf("the %s outcome answered %d, wanted %d", outcome, code, wanted[outcome])
		}
		if code == 1 || code == 2 {
			t.Errorf("the %s outcome answered %d, which the Go runtime reserves for a fatal log and a panic", outcome, code)
		}
	}
	distinct := map[int]bool{}
	for _, code := range answers {
		distinct[code] = true
	}
	if len(distinct) != 4 {
		t.Errorf("the four outcomes answered %d distinct codes, wanted 4: %v", len(distinct), answers)
	}
}

// sameStore reports whether two snapshots hold the same files with the same
// bytes.
func sameStore(before, after map[string]string) bool {
	return len(differences(before, after)) == 0
}

// differences names every path the two snapshots disagree about, so a failure
// says which file moved rather than that something did.
func differences(before, after map[string]string) []string {
	var moved []string
	for path, text := range before {
		if held, present := after[path]; !present || held != text {
			moved = append(moved, path)
		}
	}
	for path := range after {
		if _, present := before[path]; !present {
			moved = append(moved, path+" (new)")
		}
	}
	sort.Strings(moved)
	return moved
}
