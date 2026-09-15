package bench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// newlineFixture is a clean workbench with a second card, so that a case can
// dirty one file and hold every other file in the store to being untouched.
func newlineFixture(t *testing.T) string {
	t.Helper()
	root := newFixture(t)
	write(t, filepath.Join(root, CardNumbersName), "1 c00000000001\n2 c00000000002\n")
	write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor), cleanCard)
	write(t, filepath.Join(root, CardsDir, "c00000000002", JournalName), cleanJournal)
	return root
}

// migrate runs one pass of the repair over a fixture and fails the test where
// the run answered an error it was not asked for.
func migrate(t *testing.T, root string, apply bool) *NewlineMigration {
	t.Helper()
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	report, err := opened.MigrateNewlines("alka", Stamp(time.Now()), apply)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return report
}

// destinations answers the set of paths one run planned, so a test can compare
// a set against a set rather than one count against another.
func destinations(report *NewlineMigration) map[string]bool {
	set := map[string]bool{}
	for _, rewrite := range report.Rewrites {
		set[rewrite.Path] = true
	}
	return set
}

// sealDestination makes one destination genuinely impossible to write, in the
// way that is real on the platform the test runs on, and restores it when the
// case ends.
//
// Windows and POSIX disagree about what governs replacing a file, and both
// sides of the disagreement are documented rather than measured. Every write in
// this format goes through a temporary beside the destination and a rename.
// POSIX rename(2) requires write permission on the DIRECTORY holding each name
// and says nothing at all about the mode of the file being replaced, so a
// read-only file there is replaced without complaint. Windows refuses to
// replace a file carrying the read-only attribute.
//
// A case that sealed the file on every platform would therefore assert a
// Windows fact and pass vacuously on the other two, which is exactly what
// happened: this case was green on Windows and red on Linux and on macOS, and
// the red was the honest answer. The rehearsal had genuinely succeeded.
func sealDestination(t *testing.T, path string) {
	t.Helper()
	target, sealed, open := path, os.FileMode(0o444), os.FileMode(0o644)
	if runtime.GOOS != "windows" {
		target, sealed, open = filepath.Dir(path), os.FileMode(0o555), os.FileMode(0o755)
	}
	if err := os.Chmod(target, sealed); err != nil {
		t.Fatalf("seal %s: %v", target, err)
	}
	t.Cleanup(func() { os.Chmod(target, open) })
}

// TestTheTransformIsTheDetectorOverARealPopulation is the criterion the whole
// design rests on. The migration has no separate classification pass, so a
// clean file the transform changes would select itself for repair and be
// rewritten, and the claim to establish is not that some number of files is
// dirty but that the set of files the transform changes equals, file by file,
// the set of files carrying a carriage return.
//
// The corpus is a workbench driven through every anchor shape this tool writes,
// plus the plain files a workbench carries beside them.
//
// It is armed by breaking it rather than by trusting it: the second half runs a
// deliberately lossy variant of the anchor transform over the same corpus and
// requires the set equality to fail.
func TestTheTransformIsTheDetectorOverARealPopulation(t *testing.T) {
	root := everyAnchorShape(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	paths, err := opened.newlineFiles()
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("the corpus carries no file, so the comparison below asserts nothing")
	}
	changed, carrying := map[string]bool{}, map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if result := transformNewlines(path, data); !bytes.Equal(result.Out, data) {
			changed[path] = true
		}
		// The set to compare against is the files carrying a carriage return
		// that STANDS FOR A LINE ENDING, read independently of the transform
		// through the three stored forms. Comparing against every file
		// carrying any carriage return at all was this case's claim when it
		// was written, and it is not the claim the code should keep: a
		// carriage return this format keeps is one the transform is required
		// to leave, so such a file is correctly unchanged and the old
		// comparison called that a defect. It passed only because the corpus
		// carried no such file, which is the same easy-position avoidance this
		// card has now found three times, so the corpus carries one
		// deliberately.
		if storedNewlineForm(string(data), path) != "" {
			carrying[path] = true
		}
	}
	t.Logf("the corpus carries %d files, of which %d are changed by their own transform and %d carry a stored line ending", len(paths), len(changed), len(carrying))
	// Both sides have a member, so the equality below is a comparison rather
	// than two empty sets agreeing.
	if len(changed) == 0 || len(carrying) == 0 {
		t.Error("one side of the equality is empty, so it is satisfied by carrying nothing rather than by agreeing")
	}
	for path := range changed {
		if !carrying[path] {
			t.Errorf("%s is changed by its own transform and carries no stored line ending, so the detector produces a false destination", path)
		}
	}
	for path := range carrying {
		if !changed[path] {
			t.Errorf("%s carries a stored line ending and its own transform leaves it alone", path)
		}
	}

	// Arm it. A lossy variant has to break the equality, or the comparison
	// above would pass against an implementation that changed nothing.
	lossy := 0
	for _, path := range paths {
		if filepath.Ext(path) != ".md" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fm, body := ParseAnchor(string(data))
		if fm.Render(body) != NormalizeNewlines(string(data)) {
			continue
		}
		if fm.Render(body+"\nan appended line\n") != string(data) && !carrying[path] {
			lossy++
		}
	}
	if lossy == 0 {
		t.Error("the lossy variant changed no clean file, so the set equality above was not armed by anything")
	}
}

// everyAnchorShape builds a workbench carrying one of each anchor this tool
// writes, together with the plain files that sit beside them, so the fixed
// point claim above is measured over a real population rather than over one
// hand-written anchor.
func everyAnchorShape(t *testing.T) string {
	t.Helper()
	root := newlineFixture(t)
	card := filepath.Join(root, CardsDir, "c00000000001")
	write(t, filepath.Join(card, CardAnchor), "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\nlinks:\n  - \"relates_to c00000000002\"\nfields:\n  git.branch: \"dinah-514\"\n---\nFraming.\n")
	write(t, filepath.Join(card, "items", "i00000000001", ItemAnchor), "---\nkind: acceptance_criterion\nstate: pending\nowner: alka\nordinal: 1\ncitations:\n  - scheme: \"test\"\n    target: \"a/b_test.go\"\n---\nAn item.\n")
	write(t, filepath.Join(card, CommentsDir, "m00000000001", CommentAnchor), "---\nactor: alka\nts: \"2026-08-17T09:00:00Z\"\n---\nA comment.\n")
	write(t, filepath.Join(card, AttachmentsDir, "a00000000001", AttachmentAnchor), "---\nfilename: \"note.md\"\ndescription: \"A note\"\n---\n")
	write(t, filepath.Join(card, AttachmentsDir, "a00000000001", PayloadDir, "note.md"), "payload prose\r\nwith its own endings\r\n")
	write(t, filepath.Join(root, WorkstreamsDir, "f00000000001", WorkstreamAnchor), "---\ntitle: A workstream\nslug: ws\n---\nWorkstream text.\n")
	write(t, filepath.Join(root, WorkstreamsDir, "f00000000001", JournalName), cleanJournal)
	write(t, filepath.Join(root, JournalName), cleanJournal)
	write(t, filepath.Join(root, "README.md"), "A note somebody dropped here.\n")
	// Two notes the equality needs, one on each side of it. The first carries a
	// carriage return this format keeps, so the transform must leave it and it
	// must not be counted dirty; comparing against every file carrying any
	// carriage return called exactly this file a defect. The second carries a
	// real stored line ending, so both sets have a member and the equality is
	// not satisfied by being empty on both sides.
	write(t, filepath.Join(root, "LOOSE.md"), "A note carrying one\rinside a line.\n")
	write(t, filepath.Join(root, "DIRTY.md"), "A note an editor wrote.\r\nWith two lines.\r\n")
	return root
}

// TestARepairChangesOnlyLineEndingsAndIsIdempotent builds a store carrying all
// three stored forms and an unrecognised frontmatter key, and holds the repair
// to changing line endings and nothing else, per file rather than by a count.
func TestARepairChangesOnlyLineEndingsAndIsIdempotent(t *testing.T) {
	root := newlineFixture(t)
	first := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	// Form 2, the split frontmatter form: a raw carriage return followed by
	// the two-character escape of a line feed, which carries no CRLF pair.
	// Beside it an unrecognised key a clean re-render must not disturb, and a
	// lone carriage return that must survive.
	write(t, first, "---\ntitle: \"add-a\r\\nadd-b\"\ncolumn: b00000000001\nstate: ready\nsomething_else:\n  nested: \"a value\"\nloose: \"one\rtwo\"\n---\nBody line one.\r\nBody line two.\n")
	journal := filepath.Join(root, CardsDir, "c00000000002", JournalName)
	record, err := json.Marshal(map[string]string{"ts": "2026-08-17T09:00:00Z", "event": "blocked", "actor": "alka", "reason": "a\r\nb", "kind": "c\rd"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	write(t, journal, string(record)+"\n")

	before := everyFileUnder(t, root)
	report := migrate(t, root, true)
	if len(report.Conflicts) != 0 {
		t.Fatalf("the run met %d conflicts: %+v", len(report.Conflicts), report.Conflicts)
	}
	if len(report.Rewrites) != 2 {
		t.Fatalf("wanted two destinations, got %d: %+v", len(report.Rewrites), report.Rewrites)
	}

	fm, body := ParseAnchor(readFile(t, first))
	if got := fm.Value("title"); got != "add-a\nadd-b" {
		t.Errorf("the title reads %q, wanted the same prose with its line ending reduced", got)
	}
	if got := fm.Value("loose"); got != "one\rtwo" {
		t.Errorf("the loose carriage return reads %q and must survive untouched", got)
	}
	if got := body; got != "Body line one.\nBody line two.\n" {
		t.Errorf("the body reads %q", got)
	}
	wasFM, _ := ParseAnchor(before[first])
	if got, want := strings.Join(fm.Raw("something_else"), "\n"), strings.Join(wasFM.Raw("something_else"), "\n"); got != want {
		t.Errorf("the unrecognised key reads %q and was %q, and a clean key is never re-rendered", got, want)
	}

	var repaired map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(readFile(t, journal))), &repaired); err != nil {
		t.Fatalf("the repaired record does not decode: %v", err)
	}
	if repaired["reason"] != "a\nb" {
		t.Errorf("the repaired reason reads %q", repaired["reason"])
	}
	if repaired["kind"] != "c\rd" {
		t.Errorf("the repaired kind reads %q, and a lone carriage return survives everywhere", repaired["kind"])
	}

	second := migrate(t, root, true)
	if len(second.Rewrites) != 0 || len(second.Conflicts) != 0 {
		t.Errorf("the second run reported %d rewrites and %d conflicts, and the repair is idempotent by construction", len(second.Rewrites), len(second.Conflicts))
	}
}

// TestAPreviewWritesOnlyItsDestinations holds the preview to the bargain the
// operator ruled for: it changes no content, it touches exactly the
// destinations, and it says so.
//
// Both halves of the modification-time assertion are required. A permission
// admitting both outcomes is passed by an implementation that never rehearses
// at all, which is the whole behaviour this test exists to arm.
func TestAPreviewWritesOnlyItsDestinations(t *testing.T) {
	root := newlineFixture(t)
	dirty := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	write(t, dirty, "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\nFraming.\r\n")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	paths, err := opened.newlineFiles()
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	before := everyFileUnder(t, root)
	stamps := map[string]time.Time{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		stamps[path] = info.ModTime()
	}
	// The clock on a fast filesystem has a coarse enough tick that a write in
	// the same instant reports the same time, so the fixture is aged rather
	// than the test slowed.
	for _, path := range paths {
		aged := stamps[path].Add(-2 * time.Hour)
		if err := os.Chtimes(path, aged, aged); err != nil {
			t.Fatalf("chtimes %s: %v", path, err)
		}
		stamps[path] = aged
	}

	report := migrate(t, root, false)
	if len(report.Rewrites) != 1 {
		t.Fatalf("wanted one destination, got %d", len(report.Rewrites))
	}
	if report.Applied {
		t.Error("a preview reports itself as applied")
	}
	if after := everyFileUnder(t, root); len(after) != len(before) {
		t.Fatalf("the preview changed the file set")
	} else {
		for path, text := range before {
			if after[path] != text {
				t.Errorf("%s changed under a preview", path)
			}
		}
	}
	planned := destinations(report)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		moved := info.ModTime().After(stamps[path])
		if planned[path] && !moved {
			t.Errorf("%s is a destination and its modification time did not move, so nothing was rehearsed for it", path)
		}
		if !planned[path] && moved {
			t.Errorf("%s is not a destination and its modification time moved", path)
		}
	}
}

// TestAReadOnlyDestinationRefusesTheWholeRun is the criterion a probe cannot
// pass. The destination is made genuinely unwritable and nothing is injected,
// because a writability check hard-coded to succeed passes any test that
// injects its failure.
//
// What unwritable has to mean is platform-specific, and sealDestination is
// where that is stated and argued. The criterion's own wording asks for
// os.Chmod(path, 0o444) on the file, which is the Windows spelling of it and is
// very nearly a no-op on POSIX, where the containing directory governs instead;
// following that wording literally is what made this case pass on Windows and
// fail on Linux and on macOS.
//
// The writable half is carried beside it so that an implementation refusing
// everything cannot pass either.
func TestAReadOnlyDestinationRefusesTheWholeRun(t *testing.T) {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("root ignores the mode bits this case rests on")
	}
	plant := func(t *testing.T) (string, []string) {
		t.Helper()
		root := newlineFixture(t)
		var dirty []string
		for _, id := range []string{"c00000000001", "c00000000002"} {
			path := filepath.Join(root, CardsDir, id, CardAnchor)
			write(t, path, "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\nFraming.\r\n")
			dirty = append(dirty, path)
			journal := filepath.Join(root, CardsDir, id, JournalName)
			write(t, journal, strings.TrimSuffix(cleanJournal, "\n")+"\r\n")
			dirty = append(dirty, journal)
		}
		column := filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor)
		write(t, column, "---\ntitle: Only\nslug: only\nkind: work\n---\nColumn text.\r\n")
		dirty = append(dirty, column)
		return root, dirty
	}

	root, dirty := plant(t)
	locked := dirty[0]
	sealDestination(t, locked)
	before := everyFileUnder(t, root)
	preview := migrate(t, root, false)
	refused := false
	for _, conflict := range preview.Conflicts {
		if conflict.Path == locked && conflict.Condition == NewlineConflictUnwritable {
			refused = true
		}
	}
	if !refused {
		t.Fatalf("the preview did not report %s as unwritable: %+v", locked, preview.Conflicts)
	}
	applied := migrate(t, root, true)
	for _, rewrite := range applied.Rewrites {
		if rewrite.Written {
			t.Errorf("%s was written, and one refusal stops the whole run", rewrite.Path)
		}
	}
	// The criterion asks the run to exit non-zero, and Clean is the mechanism
	// that carries a conflict out to the command's exit code. It is asserted
	// here rather than left to the renderer, because Clean stopped counting
	// rewrites while this card was in review and a conflict has to keep
	// answering unclean on its own.
	if applied.Clean() || preview.Clean() {
		t.Error("a run that refused a destination reports itself clean, so the command would exit zero over a repair that did not happen")
	}
	for path, text := range everyFileUnder(t, root) {
		if before[path] != text {
			t.Errorf("%s changed, and a refused run writes nothing at all", path)
		}
	}

	// The other half: the same five destinations, all writable, are all
	// repaired, so a build that refuses everything cannot pass.
	clean, cleanDirty := plant(t)
	report := migrate(t, clean, true)
	if len(report.Conflicts) != 0 {
		t.Fatalf("the writable run met conflicts: %+v", report.Conflicts)
	}
	if len(report.Rewrites) != len(cleanDirty) {
		t.Fatalf("wanted %d destinations, got %d", len(cleanDirty), len(report.Rewrites))
	}
	for _, rewrite := range report.Rewrites {
		if !rewrite.Written {
			t.Errorf("%s was planned and not written", rewrite.Path)
		}
	}
}

// TestALockedFileIsSkippedAndTheRestAreRepaired holds the one relaxation of the
// all-or-nothing rule to its terms, against a real lock file rather than an
// injected one, and pins the lock a file answers to against the format's own
// rule rather than against the implementation.
func TestALockedFileIsSkippedAndTheRestAreRepaired(t *testing.T) {
	root := newlineFixture(t)
	held := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	free := filepath.Join(root, CardsDir, "c00000000002", CardAnchor)
	column := filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor)
	for _, path := range []string{held, free} {
		write(t, path, "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\nFraming.\r\n")
	}
	write(t, column, "---\ntitle: Only\nslug: only\nkind: work\n---\nColumn text.\r\n")
	lock, err := Acquire(filepath.Join(root, CardsDir, "c00000000001"), "somebody-else", Stamp(time.Now()))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer lock.Release()

	report := migrate(t, root, true)
	busy := false
	for _, conflict := range report.Conflicts {
		if conflict.Condition != NewlineConflictLocked {
			t.Errorf("a held lock produced the condition %q, and only a lock may be skipped", conflict.Condition)
		}
		if conflict.Path == held {
			busy = true
			if conflict.Detail != "somebody-else" {
				t.Errorf("the busy report names %q rather than the lock's own holder", conflict.Detail)
			}
		}
	}
	if !busy {
		t.Fatalf("the held file was not reported busy: %+v", report.Conflicts)
	}
	if strings.Contains(readFile(t, held), "\r") {
		// The file is untouched, which is what "left for the next run" means.
	} else {
		t.Error("the held file was repaired while its lock stood")
	}
	repaired := 0
	for _, rewrite := range report.Rewrites {
		if rewrite.Written {
			repaired++
		}
	}
	if repaired == 0 {
		t.Error("a busy file stopped the whole run, and only an unreadable, unwritable or unsupported destination may do that")
	}
	// A column anchor's lock IS the workbench root, so it is covered by the
	// run's outer acquisition and takes no second lock. It could not have been
	// repaired above if the run had tried to take the root lock twice, because
	// Acquire refuses rather than recursing.
	if strings.Contains(readFile(t, column), "\r") {
		t.Error("the column anchor was not repaired, which is what a second acquisition of the root lock would cost")
	}
}

// TestTheLockOfAFileIsTheNearestJournalBearingEntity pins lockDirForFile
// against the format's own sentence rather than against the implementation:
// "Lock scope is the nearest enclosing journal-bearing entity, a card for
// anything inside a card and the workbench for everything else."
func TestTheLockOfAFileIsTheNearestJournalBearingEntity(t *testing.T) {
	root := newlineFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card := filepath.Join(root, CardsDir, "c00000000001")
	archived := filepath.Join(root, ArchiveDir, CardsDir, "c00000000003")
	workstream := filepath.Join(root, WorkstreamsDir, "f00000000001")
	cases := []struct{ path, want string }{
		{filepath.Join(card, CardAnchor), card},
		{filepath.Join(card, "items", "i00000000001", ItemAnchor), card},
		{filepath.Join(card, CommentsDir, "m00000000001", CommentAnchor), card},
		{filepath.Join(card, JournalName), card},
		{filepath.Join(archived, CardAnchor), archived},
		{filepath.Join(workstream, WorkstreamAnchor), workstream},
		{filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), root},
		{filepath.Join(root, WorkbenchAnchor), root},
		{filepath.Join(root, JournalName), root},
	}
	for _, c := range cases {
		if got := opened.lockDirForFile(c.path); got != c.want {
			t.Errorf("the lock for %s is %s, wanted %s", c.path, got, c.want)
		}
	}
}

// TestAMidRunWriteFailureReportsWhatLanded holds the partial-failure account to
// its shape: the file written carries Written true, the file not reached
// carries false, and no file is half written.
//
// The failure is a real one rather than an injected one. The plan pass
// rehearses both writes while both files are writable, and the second is made
// read-only between the passes, which is the state a run meets when somebody
// changes a permission underneath it.
func TestAMidRunWriteFailureReportsWhatLanded(t *testing.T) {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("root ignores the mode bits this case rests on")
	}
	root := newlineFixture(t)
	for _, id := range []string{"c00000000001", "c00000000002"} {
		write(t, filepath.Join(root, CardsDir, id, CardAnchor), dirtyCard)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	report, err := opened.MigrateNewlines("alka", Stamp(time.Now()), false)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(report.Rewrites) != 2 {
		t.Fatalf("wanted two destinations, got %d", len(report.Rewrites))
	}
	first, second := report.Rewrites[0].Path, report.Rewrites[1].Path
	was := readFile(t, second)
	// The second destination is sealed when the write pass reaches the first,
	// which is after both rehearsals have already succeeded.
	newlineHook = func(phase, path string) {
		if phase != newlinePhaseWrite || path != first {
			return
		}
		sealDestination(t, second)
	}
	t.Cleanup(func() { newlineHook = nil })

	applied, err := opened.MigrateNewlines("alka", Stamp(time.Now()), true)
	if err == nil {
		t.Fatal("the run answered no error and the second destination could not be written")
	}
	if applied == nil {
		t.Fatal("the run answered an error and no report, and the report travels beside the error")
	}
	written := map[string]bool{}
	for _, rewrite := range applied.Rewrites {
		written[rewrite.Path] = rewrite.Written
	}
	if !written[first] {
		t.Errorf("%s is reported as not written", first)
	}
	if written[second] {
		t.Errorf("%s is reported as written", second)
	}
	if strings.Contains(readFile(t, first), "\r") {
		t.Errorf("%s was reported written and still carries its old bytes", first)
	}
	if got := readFile(t, second); got != was {
		t.Errorf("%s is half written: %q", second, got)
	}
}

// TestAMarkdownFileThatIsNotARecordFileComesBackByteForByte holds the gate to
// the half of it that is a refusal as well as the half that is a pass-through.
func TestAMarkdownFileThatIsNotARecordFileComesBackByteForByte(t *testing.T) {
	root := newlineFixture(t)
	note := filepath.Join(root, "note.md")
	plain := filepath.Join(root, "plain.md")
	unterminated := filepath.Join(root, "unterminated.md")
	write(t, note, "---\n\nA note someone dropped in the workbench directory.\n\n---\n\nMore prose.\n")
	write(t, plain, "No fence at all.\n")
	write(t, unterminated, "---\r\ntitle: a\r\nbody with no closing fence\r\n")
	dirty := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	write(t, dirty, "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\nFraming.\r\n")

	before := everyFileUnder(t, root)
	report := migrate(t, root, true)
	if len(report.Conflicts) != 0 {
		t.Fatalf("the run met conflicts: %+v", report.Conflicts)
	}
	if readFile(t, note) != before[note] {
		t.Errorf("the note came back as %q", readFile(t, note))
	}
	if readFile(t, plain) != before[plain] {
		t.Errorf("the fenceless file came back as %q", readFile(t, plain))
	}
	if got, want := readFile(t, unterminated), "---\ntitle: a\nbody with no closing fence\n"; got != want {
		t.Errorf("the unterminated fence came back as %q, wanted %q, which is all the whole-file branch can do", got, want)
	}
	if strings.Contains(readFile(t, dirty), "\r") {
		t.Error("the genuine dirty anchor was not repaired, so a build that repairs nothing would pass this")
	}

	// The other half of the dispatch: a file NAMED card.md whose header does
	// not round-trip is refused, and the run then rewrites nothing at all.
	damaged := newlineFixture(t)
	write(t, filepath.Join(damaged, CardsDir, "c00000000002", CardAnchor), "---\n\nA card anchor somebody broke.\n\n---\n\nMore.\n")
	alsoDirty := filepath.Join(damaged, CardsDir, "c00000000001", CardAnchor)
	write(t, alsoDirty, "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\nFraming.\r\n")
	was := everyFileUnder(t, damaged)
	refusal := migrate(t, damaged, true)
	named := false
	for _, conflict := range refusal.Conflicts {
		if conflict.Condition == NewlineConflictUnsupported && conflict.Detail == "does not round-trip through the anchor reader" {
			named = true
		}
	}
	if !named {
		t.Fatalf("the damaged card.md was not refused: %+v", refusal.Conflicts)
	}
	for path, text := range everyFileUnder(t, damaged) {
		if was[path] != text {
			t.Errorf("%s changed, and a refused run rewrites nothing at all", path)
		}
	}
}

// TestAJournalWhoseRecordsAreSeparatedByCRLFIsRepaired is the case a transform
// that looked only inside string literals left dirty and reported clean, since
// a record separator is outside every literal.
func TestAJournalWhoseRecordsAreSeparatedByCRLFIsRepaired(t *testing.T) {
	root := newlineFixture(t)
	journal := filepath.Join(root, CardsDir, "c00000000001", JournalName)
	records := []string{
		`{"ts":"2026-08-17T09:00:00Z","event":"created","actor":"alka","title":"A card"}`,
		`{"ts":"2026-08-17T09:01:00Z","event":"moved","actor":"alka","to":"b00000000001"}`,
	}
	write(t, journal, strings.Join(records, "\r\n")+"\r\n")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.checkStoredNewlines()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	named := false
	for _, finding := range findings {
		if finding.Path == journal && strings.HasPrefix(finding.Detail, "2 line-ending") {
			named = true
		}
	}
	if !named {
		t.Fatalf("check did not name the journal with a line-ending count of two: %+v", findings)
	}

	migrate(t, root, true)
	after := readFile(t, journal)
	if strings.Contains(after, "\r") {
		t.Errorf("the repaired journal still carries a carriage return: %q", after)
	}
	for index, line := range strings.Split(strings.TrimSuffix(after, "\n"), "\n") {
		var was, is map[string]any
		if err := json.Unmarshal([]byte(records[index]), &was); err != nil {
			t.Fatalf("original record %d: %v", index, err)
		}
		if err := json.Unmarshal([]byte(line), &is); err != nil {
			t.Fatalf("repaired record %d: %v", index, err)
		}
		if len(was) != len(is) {
			t.Errorf("record %d carries %d members and carried %d", index, len(is), len(was))
		}
		for name, value := range was {
			if is[name] != value {
				t.Errorf("record %d member %s reads %v and read %v", index, name, is[name], value)
			}
		}
	}
	if second := migrate(t, root, true); len(second.Rewrites) != 0 {
		t.Errorf("the second run planned %d rewrites", len(second.Rewrites))
	}
}

// TestEverySpellingOfAnEncodedLineEndingIsRepaired feeds the journal transform
// byte-exact fixtures and asserts the resulting bytes, not a decoded
// approximation of them.
//
// The mixed-spelling cases fail against any rule that recognises one spelling
// of each escape, and that rule was written from a belief about Go's encoder
// that Go documents nowhere. The untouched cases are the ones an earlier byte
// substitution destroyed, where a four-byte match landed one byte inside a
// doubled backslash.
func TestEverySpellingOfAnEncodedLineEndingIsRepaired(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		changed bool
	}{
		{"plain escape pair", `{"r":"a\r\nb"}`, `{"r":"a\nb"}`, true},
		{"unicode carriage return", `{"r":"a\u000d\nb"}`, `{"r":"a\nb"}`, true},
		{"unicode line feed", `{"r":"a\r\u000ab"}`, `{"r":"a\nb"}`, true},
		{"both unicode, uppercase hex", `{"r":"a\u000D\u000Ab"}`, `{"r":"a\nb"}`, true},
		{"after an escaped quotation mark", `{"r":"a\"\r\nb"}`, `{"r":"a\"\nb"}`, true},
		{"prose ending in backslash r", `{"r":"as \\r\nand"}`, `{"r":"as \\r\nand"}`, false},
		{"two literal backslashes then r", `{"r":"a\\\\r\\nb"}`, `{"r":"a\\\\r\\nb"}`, false},
		{"a lone carriage return escape", `{"r":"a\rb"}`, `{"r":"a\rb"}`, false},
		{"a carriage return then a literal backslash n", `{"r":"a\r\\nb"}`, `{"r":"a\r\\nb"}`, false},
		{"an escaped less-than beside a repaired pair", `{"r":"\u003c\r\nb"}`, `{"r":"\u003c\nb"}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := transformJournal([]byte(c.in + "\n"))
			if result.Condition != "" {
				t.Fatalf("refused as %s (%s)", result.Condition, result.Detail)
			}
			if got, want := string(result.Out), c.want+"\n"; got != want {
				t.Errorf("got  %s\nwant %s", got, want)
			}
			if changed := string(result.Out) != c.in+"\n"; changed != c.changed {
				t.Errorf("the transform changed the record: %v, wanted %v", changed, c.changed)
			}
		})
	}
}

// TestALiteralThatCannotBeReEncodedRefusesTheRecord is the defect the design
// review filed against this station. The verification decodes the transformed
// record and the original through the same decoder, so where that decoder is
// lossy on the original both sides are lossy alike and the comparison passes,
// reporting success over a string that came back permanently altered.
//
// json.Unmarshal documents that invalid UTF-8 and invalid UTF-16 surrogate
// pairs are replaced by U+FFFD rather than refused, so a decoded value carrying
// no U+FFFD carries no replacement and re-encoding it is safe. A literal
// carrying one is refused rather than guessed at, because this repair has no
// way to tell prose that genuinely carries the character from prose the decoder
// replaced.
func TestALiteralThatCannotBeReEncodedRefusesTheRecord(t *testing.T) {
	// A lone high surrogate beside a pair the repair would otherwise fix.
	lossy := `{"r":"a\ud800\r\nb"}` + "\n"
	result := transformJournal([]byte(lossy))
	if result.Condition != NewlineConflictUnsupported {
		t.Fatalf("the record was not refused: condition %q, out %q", result.Condition, result.Out)
	}
	if !strings.Contains(result.Detail, "record 1") {
		t.Errorf("the detail reads %q and does not name the record", result.Detail)
	}
	if string(result.Out) != lossy {
		t.Error("a refused file answers its own bytes")
	}

	// The same literal with no pair in it is copied verbatim and is safe, so
	// the refusal above is about the re-encode rather than about the value.
	safe := `{"r":"a\ud800b"}` + "\n"
	untouched := transformJournal([]byte(safe))
	if untouched.Condition != "" {
		t.Fatalf("a literal the transform does not have to change was refused: %s", untouched.Detail)
	}
	if string(untouched.Out) != safe {
		t.Errorf("the literal came back as %q", untouched.Out)
	}
}

// TestATornFinalRecordIsCopiedAndTheFileIsNotRefused is the other defect filed
// against this station. ReadJournal tolerates a torn tail by design and check
// carries its own repair for one, so a repair that refused the whole file over
// it would stop the run everywhere on a store this format expects.
//
// A record that does not decode and is NOT the last is a damaged store rather
// than a torn tail, which is a different piece of work from deciding what a
// line ending should be, so that file is refused.
func TestATornFinalRecordIsCopiedAndTheFileIsNotRefused(t *testing.T) {
	torn := `{"r":"a\r\nb"}` + "\n" + `{"ts":"2026-08-17T09:0`
	result := transformJournal([]byte(torn))
	if result.Condition != "" {
		t.Fatalf("a torn tail refused the file: %s (%s)", result.Condition, result.Detail)
	}
	if got, want := string(result.Out), `{"r":"a\nb"}`+"\n"+`{"ts":"2026-08-17T09:0`; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	damaged := `{"ts":"2026-08-17T09:0` + "\n" + `{"r":"a\r\nb"}` + "\n"
	refused := transformJournal([]byte(damaged))
	if refused.Condition != NewlineConflictUnsupported {
		t.Fatalf("a record that does not decode and is not the last was not refused: %q", refused.Condition)
	}
	if !strings.Contains(refused.Detail, "record 1") {
		t.Errorf("the detail reads %q and does not name the record", refused.Detail)
	}
	if string(refused.Out) != damaged {
		t.Error("a refused file answers its own bytes")
	}
}

// TestProseThatLooksLikeAnEncodedLineEndingSurvivesEndToEnd is the same case as
// one row of the table above, carried through a real store, because the byte
// substitution it guards against was reproduced on a real store rather than on
// a fixture.
func TestProseThatLooksLikeAnEncodedLineEndingSurvivesEndToEnd(t *testing.T) {
	root := newlineFixture(t)
	journal := filepath.Join(root, CardsDir, "c00000000001", JournalName)
	reason := "a journal escapes a carriage return as \\r\nand a line feed as \\n"
	record, err := json.Marshal(map[string]string{"ts": "2026-08-17T09:00:00Z", "event": "blocked", "actor": "alka", "reason": reason})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	write(t, journal, string(record)+"\n")
	before := readFile(t, journal)

	// Beside it, in another card, a reason that genuinely carries a CRLF, so
	// an implementation that repairs nothing cannot pass either.
	genuine := filepath.Join(root, CardsDir, "c00000000002", JournalName)
	other, err := json.Marshal(map[string]string{"ts": "2026-08-17T09:00:00Z", "event": "blocked", "actor": "alka", "reason": "one\r\ntwo"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	write(t, genuine, string(other)+"\n")

	report := migrate(t, root, true)
	if destinations(report)[journal] {
		t.Error("the prose journal was reported as a destination")
	}
	if !destinations(report)[genuine] {
		t.Error("the journal carrying a genuine pair was not reported as a destination")
	}
	if readFile(t, journal) != before {
		t.Errorf("the prose journal changed:\n was %q\n now %q", before, readFile(t, journal))
	}
	var repaired map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(readFile(t, journal))), &repaired); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if repaired["reason"] != reason {
		t.Errorf("the reason reads %q and went in as %q", repaired["reason"], reason)
	}
	var fixed map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(readFile(t, genuine))), &fixed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fixed["reason"] != "one\ntwo" {
		t.Errorf("the genuine pair reads %q", fixed["reason"])
	}
}

// TestCheckReportsEachStoredFormAndCanTellACleanStoreFromADirtyOne holds the
// finding to the three stored forms of a line-ending carriage return, to
// agreeing with the migration about which files are dirty, and to leaving a
// file whose only carriage returns are loose ones out of the destinations and
// out of the report.
//
// The report was once asked for on a loose-only file too. The operator ruled it
// out on 2026-09-15, because such a file conforms and reporting it made dinah
// check exit non-zero for ever over a store nothing could clear, and
// dinah-514/criteria/5 changed with the ruling rather than being left
// contradicting the code. TestAConformingStoreChecksClean carries that half,
// including the case the ruling must not silence.
func TestCheckReportsEachStoredFormAndCanTellACleanStoreFromADirtyOne(t *testing.T) {
	root := newlineFixture(t)
	pair := filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor)
	split := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	escaped := filepath.Join(root, CardsDir, "c00000000002", JournalName)
	loose := filepath.Join(root, CardsDir, "c00000000002", CardAnchor)
	write(t, pair, "---\ntitle: Only\nslug: only\nkind: work\n---\nColumn text.\r\n")
	write(t, split, "---\ntitle: \"add-a\r\\nadd-b\"\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n")
	record, err := json.Marshal(map[string]string{"ts": "2026-08-17T09:00:00Z", "event": "blocked", "actor": "alka", "reason": "a\r\nb"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	write(t, escaped, string(record)+"\n")
	write(t, loose, "---\ntitle: \"one\rtwo\"\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.checkStoredNewlines()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	reported := map[string]string{}
	for _, finding := range findings {
		// Two keys are legal here, one per shape the sweep reports, and which
		// shape gets which is asserted by
		// TestEachShapeTheSweepReportsGetsASentenceTrueOfIt.
		switch finding.Key {
		case FindingStoredCarriageReturn, FindingNewlineRepairUnsupported:
		default:
			t.Errorf("the sweep reported %s", finding.Key)
		}
		reported[finding.Path] = finding.Detail
	}
	for _, path := range []string{pair, split, escaped} {
		if _, named := reported[path]; !named {
			t.Errorf("%s was not reported", path)
		}
	}
	if got, named := reported[loose]; named {
		t.Errorf("the file whose only carriage returns are loose ones is reported as %q, and such a file conforms", got)
	}

	// The finding and the preview agree about which files are dirty, asserted
	// as a set against a set. A file nothing reports is a file the preview does
	// not plan, which is the same agreement read from the other side.
	preview := migrate(t, root, false)
	planned := destinations(preview)
	for path := range reported {
		if !planned[path] {
			t.Errorf("%s is reported and the preview does not plan it", path)
		}
	}
	for path := range planned {
		if _, named := reported[path]; !named {
			t.Errorf("%s is planned by the preview and reported by nothing", path)
		}
	}

	migrate(t, root, true)
	after, err := opened.checkStoredNewlines()
	if err != nil {
		t.Fatalf("check again: %v", err)
	}
	for _, finding := range after {
		t.Errorf("%s is still reported after the repair: %s", finding.Path, finding.Detail)
	}
	if strings.Contains(readFile(t, loose), "\r") {
		// Correct. The loose byte survives.
	} else {
		t.Error("the loose carriage return was removed")
	}
}

// TestAnAttachmentPayloadIsUntouched guards the exemption the format itself
// carries: a payload is any bytes and is never inspected. It fails if a later
// pass extends normalisation to payloads and starts corrupting binaries.
func TestAnAttachmentPayloadIsUntouched(t *testing.T) {
	root := newlineFixture(t)
	payload := filepath.Join(root, CardsDir, "c00000000001", AttachmentsDir, "a00000000001", PayloadDir, "note.md")
	write(t, filepath.Join(root, CardsDir, "c00000000001", AttachmentsDir, "a00000000001", AttachmentAnchor), "---\nfilename: \"note.md\"\n---\n")
	source := "one\r\ntwo\r\n"
	write(t, payload, source)
	// A dirty anchor beside it, so a run that repairs nothing cannot pass.
	write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor), "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\nFraming.\r\n")

	report := migrate(t, root, true)
	if destinations(report)[payload] {
		t.Error("the payload was reported as a destination")
	}
	if got := readFile(t, payload); got != source {
		t.Errorf("the payload reads %q and was written as %q", got, source)
	}
	if strings.Contains(readFile(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor)), "\r") {
		t.Error("the anchor beside the payload was not repaired")
	}
}

// TestALoneCarriageReturnSurvivesEverywhere holds the one deliberate keep of
// dinah-514/decisions/7: a 0x0D that no 0x0A follows is a character the prose
// meant to carry rather than a line ending, and the repair must leave it.
//
// "Everywhere" is the word this case kept failing to earn. Every fixture it
// carried put the byte in the interior of a line, which is the easiest position
// there is, and the repair deleted it at the end of a file for a whole round
// while this stayed green. The positions below are the ones an interior fixture
// never reaches, and each is planted in an anchor body, in a frontmatter value
// and in a journal record so that no branch of the repair is left untested:
//
//   - the interior of a line, which was all this case had;
//   - the last byte of the file, with no line feed after it, which is what
//     dinah comment writes and what the repair deleted;
//   - a run of them at the last byte of the file;
//   - a final line consisting of nothing else;
//   - immediately before another carriage return, inside a line.
//
// A carriage return at the end of a line that a line feed DOES follow is a line
// ending and is not in this list: that one must go, and
// TestTheRepairIsAFixedPointOverEveryShape holds it to going.
func TestALoneCarriageReturnSurvivesEverywhere(t *testing.T) {
	bodies := []struct{ name, body string }{
		{"in the interior of a line", "body one\rbody two\n"},
		{"as the last byte of the file", "body one\r"},
		{"as a run at the last byte of the file", "body one\r\r"},
		{"as a final line of its own", "body one\n\r"},
		{"beside another carriage return", "body\r\rone\n"},
	}
	values := []struct{ name, value string }{
		{"in the interior", "one\rtwo"},
		{"at the end", "one\r"},
		{"twice over", "one\r\rtwo"},
	}
	for _, b := range bodies {
		for _, v := range values {
			t.Run(b.name+", with a value "+v.name, func(t *testing.T) {
				root := newlineFixture(t)
				anchor := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
				journal := filepath.Join(root, CardsDir, "c00000000001", JournalName)
				fm := NewFrontmatter()
				fm.Set("title", v.value)
				fm.Set("column", "b00000000001")
				fm.Set("state", "ready")
				write(t, anchor, fm.Render(b.body))
				record, err := json.Marshal(map[string]string{
					"ts": "2026-08-17T09:00:00Z", "event": "blocked", "actor": "alka",
					"reason": b.body, "kind": v.value,
				})
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				write(t, journal, string(record)+"\n")
				before := everyFileUnder(t, root)

				// Nothing here is a line ending, so the repair has nothing to
				// do and every file comes back byte for byte.
				report := migrate(t, root, true)
				for _, conflict := range report.Conflicts {
					t.Errorf("%s was refused as %s (%s)", conflict.Path, conflict.Condition, conflict.Detail)
				}
				for path, was := range before {
					if got := readFile(t, path); got != was {
						t.Errorf("%s came back as %q and was %q", path, got, was)
					}
				}
				if len(report.Rewrites) != 0 {
					t.Errorf("the repair planned %d rewrites over a store carrying no line ending at all", len(report.Rewrites))
				}

				// And the store reports nothing, since a file whose carriage
				// returns are all of this kind conforms.
				opened, err := Open(root)
				if err != nil {
					t.Fatalf("open: %v", err)
				}
				findings, err := opened.checkStoredNewlines()
				if err != nil {
					t.Fatalf("check: %v", err)
				}
				for _, finding := range findings {
					t.Errorf("%s is reported as %s (%s), and every carriage return in this store is one the format keeps", finding.Path, finding.Key, finding.Detail)
				}

				// The reader still hands back what it handed back before, for
				// the positions the reader can carry. It cannot carry one at
				// the end of a file, which is exactly why the repair must not
				// be allowed to decide that it therefore does not exist.
				parsed, body := ParseAnchor(readFile(t, anchor))
				if got := parsed.Value("title"); got != v.value {
					t.Errorf("the value reads back as %q and was written as %q", got, v.value)
				}
				if trimmed := strings.TrimRight(b.body, "\r"); !strings.Contains(body, strings.TrimRight(trimmed, "\n")) {
					t.Errorf("the body reads back as %q", body)
				}
				var decoded map[string]string
				if err := json.Unmarshal([]byte(strings.TrimSpace(readFile(t, journal))), &decoded); err != nil {
					t.Fatalf("the journal record does not decode: %v", err)
				}
				if decoded["reason"] != b.body || decoded["kind"] != v.value {
					t.Errorf("the journal reads back reason %q and kind %q, written as %q and %q", decoded["reason"], decoded["kind"], b.body, v.value)
				}
			})
		}
	}
}

// TestAJournalLineCarriesNoCarriageReturn arms the write path rather than the
// repair: AppendEvent normalises the event's string members before encoding,
// because json.Marshal would otherwise store a carriage return as an escape
// with the record structure intact.
//
// The walk recurses, and the actor is why: Actor is a struct of five strings,
// so a walk over top-level string members alone would reach the name on every
// line not at all.
func TestAJournalLineCarriesNoCarriageReturn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, JournalName)
	event := Event{
		TS:    "2026-08-17T09:00:00Z",
		Event: "blocked",
		Actor: Actor{Name: "one\r\ntwo", Provider: "p\r\nq", Model: "m\r\nn", Server: "s\r\nt"},
		Note:  "a\r\nb",
	}
	if err := AppendEvent(path, event); err != nil {
		t.Fatalf("append: %v", err)
	}
	raw := readFile(t, path)
	if strings.Contains(raw, `\r`) {
		t.Errorf("the line carries an escaped carriage return: %s", raw)
	}
	events, torn, err := ReadJournal(path)
	if err != nil || torn {
		t.Fatalf("read: %v torn=%v", err, torn)
	}
	if events[0].Actor.Name != "one\ntwo" {
		t.Errorf("the actor's name reads %q, so the walk did not recurse into the actor", events[0].Actor.Name)
	}
	if events[0].Note != "a\nb" {
		t.Errorf("the note reads %q", events[0].Note)
	}
}

// TestADefinitionDocumentIsNormalisedAtItsOneReadBoundary holds ReadDefinition
// to normalising every string the document carries, at any depth, and to
// refusing a member NAME carrying a line ending rather than normalising it.
func TestADefinitionDocumentIsNormalisedAtItsOneReadBoundary(t *testing.T) {
	document := `{"profile":"dinah-core/0.7","title":"A\r\nworkbench","instructions":"one\r\ntwo","odd":{"deep":["a\r\nb"]},"loose":"x\ry","columns":[{"id":"b00000000001","title":"C\r\nD","kind":"work","instructions":"e\r\nf","require_fields":["g\r\nh"]}]}`
	definition, err := ReadDefinition([]byte(document))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if definition.Title != "A\nworkbench" {
		t.Errorf("the title reads %q", definition.Title)
	}
	var instructions string
	if err := json.Unmarshal(definition.Object["instructions"], &instructions); err != nil {
		t.Fatalf("instructions: %v", err)
	}
	if instructions != "one\ntwo" {
		t.Errorf("the standing text reads %q", instructions)
	}
	if bytes.Contains(definition.Object["odd"], []byte(`\r`)) {
		t.Errorf("an unrecognised member at depth reads %s", definition.Object["odd"])
	}
	if !bytes.Contains(definition.Object["loose"], []byte(`\r`)) {
		t.Errorf("a lone carriage return did not survive: %s", definition.Object["loose"])
	}
	if bytes.Contains(definition.Columns[0]["require_fields"], []byte(`\r`)) {
		t.Errorf("a require_fields entry reads %s", definition.Columns[0]["require_fields"])
	}

	for _, ending := range []string{`\r\n`, `\n`, `\r`} {
		name := `odd` + ending + `member`
		refused := `{"profile":"dinah-core/0.7","title":"A workbench","` + name + `":1,"columns":[{"id":"b00000000001","title":"C","kind":"work"}]}`
		_, err := ReadDefinition([]byte(refused))
		if err == nil {
			t.Fatalf("a member name carrying %s was admitted", ending)
		}
		if !strings.Contains(err.Error(), "malformed-member-name") {
			t.Errorf("a member name carrying %s was refused %v", ending, err)
		}
	}
}

// readFile is the whole of what a case needs to read one file back, and it
// reads the bytes rather than going through ReadText, which now normalises and
// would strip the very condition these cases exist to see.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// TestTheRehearsalHoldsTheLockItRewritesUnder asserts the protocol by
// construction rather than by timing. A hook runs at the moment between the
// rehearsal's read and its rename and attempts to take the file's own lock; a
// hook that succeeds proves the rehearsal is not holding it, which is the
// defect that let a concurrent write be reverted with no report and no journal
// line. The refusal is what a second process would meet, so the window that
// write could land in does not exist.
func TestTheRehearsalHoldsTheLockItRewritesUnder(t *testing.T) {
	root := newlineFixture(t)
	card := filepath.Join(root, CardsDir, "c00000000001")
	write(t, filepath.Join(card, CardAnchor), dirtyCard)

	probed := 0
	newlineHook = func(phase, path string) {
		if phase != newlinePhaseRehearse || filepath.Dir(path) != card {
			return
		}
		probed++
		held, err := Acquire(card, "somebody-else", Stamp(time.Now()))
		if err == nil {
			held.Release()
			t.Errorf("the hook took %s's own lock, so the rehearsal is not holding it", card)
		}
	}
	t.Cleanup(func() { newlineHook = nil })
	migrate(t, root, false)
	if probed == 0 {
		t.Fatal("the hook never ran, so it proves nothing")
	}
}

// dirtyCard is a card anchor whose body line ends CRLF, which is the smallest
// destination a case can plant.
const dirtyCard = "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\nFraming.\r\n"

// TestEachShapeTheSweepReportsGetsASentenceTrueOfIt is the first of the two
// findings code review pushed this card back for.
//
// Three shapes reach checkStoredNewlines and one finding key covered all three,
// whose sentence says the file stores a carriage return standing for a line
// ending. That is false of two of them. A file whose header does not round-trip
// was reported that way while carrying no carriage return of any kind, so
// somebody acting on the report would have repaired a file that was never
// dirty, and a file whose only carriage returns are the loose ones this card
// decided to keep was reported the same way.
//
// The assertion is on the key rather than on the rendered sentence, because the
// key is what selects the sentence and the catalogue is asserted separately.
func TestEachShapeTheSweepReportsGetsASentenceTrueOfIt(t *testing.T) {
	root := newlineFixture(t)
	dirty := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	loose := filepath.Join(root, CardsDir, "c00000000002", CardAnchor)
	damaged := filepath.Join(root, CardsDir, "c00000000001", CommentsDir, "m00000000001", CommentAnchor)
	write(t, dirty, dirtyCard)
	write(t, loose, "---\ntitle: \"one\rtwo\"\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n")
	// A comment anchor whose header does not round-trip, and which carries no
	// carriage return anywhere in it.
	write(t, damaged, "---\n\nA comment anchor somebody broke.\n\n---\n\nMore.\n")
	if bytes.Contains([]byte(readFile(t, damaged)), []byte("\r")) {
		t.Fatal("the damaged fixture carries a carriage return, so it cannot show what this case is for")
	}

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.checkStoredNewlines()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	reported := map[string]Finding{}
	for _, finding := range findings {
		reported[finding.Path] = finding
	}
	// A file whose carriage returns are all loose ones is reported by nothing
	// at all, on the operator's ruling, and TestAConformingStoreChecksClean is
	// where that is asserted along with the case the ruling must not silence.
	if got, named := reported[loose]; named {
		t.Errorf("a file whose only carriage returns are loose ones is reported as %s", got.Key)
	}
	for _, c := range []struct {
		path string
		key  string
		what string
	}{
		{dirty, FindingStoredCarriageReturn, "a file that really does store a line ending"},
		{damaged, FindingNewlineRepairUnsupported, "a file the repair will not decide"},
	} {
		got, named := reported[c.path]
		if !named {
			t.Errorf("%s was not reported at all", c.what)
			continue
		}
		if got.Key != c.key {
			t.Errorf("%s is reported under %s, wanted %s: a sentence written for one shape is false of the other two", c.what, got.Key, c.key)
		}
	}
	if got := reported[damaged].Detail; got != "does not round-trip through the anchor reader" {
		t.Errorf("the refused file's detail reads %q", got)
	}

}

// TestAConfirmedRepairThatSucceededExitsClean is the second finding code review
// pushed this card back for.
//
// A confirmed run that repaired every destination left the store with no defect
// at all, and the report said so, and the run then reported itself unclean
// anyway, because Clean counted the rewrites it had just performed. The sibling
// migration this one says it copies counts only its conflicts. The cost is an
// exit code, which is the part of a command a script reads and a person does
// not, so the human-readable output said success while the status said failure.
func TestAConfirmedRepairThatSucceededExitsClean(t *testing.T) {
	root := newlineFixture(t)
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), dirtyCard)

	preview := migrate(t, root, false)
	if len(preview.Rewrites) != 1 {
		t.Fatalf("wanted one destination, got %d", len(preview.Rewrites))
	}
	if !preview.Clean() {
		t.Error("a preview that met no conflict reports itself unclean, and the files it found are already named by the check finding that reports them")
	}

	applied := migrate(t, root, true)
	if len(applied.Conflicts) != 0 {
		t.Fatalf("the confirmed run met conflicts: %+v", applied.Conflicts)
	}
	if !applied.Clean() {
		t.Error("a confirmed run that repaired every destination and met no conflict reports itself unclean, so a check that ran it exits non-zero over work that succeeded")
	}

	// The store really is clean afterwards, which is what makes the exit code
	// above the whole of what was wrong.
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.checkStoredNewlines()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("the repaired store still reports %+v", findings)
	}

	// A conflict is the thing that is genuinely unclean, so the same function
	// has to keep saying so.
	busy := newlineFixture(t)
	write(t, filepath.Join(busy, CardsDir, "c00000000001", CardAnchor), dirtyCard)
	lock, err := Acquire(filepath.Join(busy, CardsDir, "c00000000001"), "somebody-else", Stamp(time.Now()))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer lock.Release()
	if held := migrate(t, busy, true); held.Clean() {
		t.Error("a run that skipped a busy file reports itself clean, and nothing else tells a reader that file was left behind")
	}
}

// TestALockedFileTheRunWouldNotHaveTouchedIsNotReportedBusy is the nit from the
// same review. A clean file whose lock another process holds was listed beside
// the dirty file next to it, under a line telling the reader to run the command
// again, and a second run finds nothing to do with it.
func TestALockedFileTheRunWouldNotHaveTouchedIsNotReportedBusy(t *testing.T) {
	root := newlineFixture(t)
	card := filepath.Join(root, CardsDir, "c00000000001")
	dirty := filepath.Join(card, CardAnchor)
	clean := filepath.Join(card, JournalName)
	write(t, dirty, dirtyCard)
	write(t, clean, cleanJournal)
	lock, err := Acquire(card, "somebody-else", Stamp(time.Now()))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer lock.Release()

	report := migrate(t, root, true)
	busy := map[string]bool{}
	for _, conflict := range report.Conflicts {
		busy[conflict.Path] = true
	}
	if !busy[dirty] {
		t.Errorf("the dirty file under a held lock was not reported busy: %+v", report.Conflicts)
	}
	if busy[clean] {
		t.Error("a clean file under a held lock was reported busy, and a second run has nothing to do with it")
	}
}

// TestNormalisationIsTrueOfItsOwnAnswer is the first blocker code review pushed
// this card back for, and it is the card's headline claim.
//
// The pass replaced the CRLF pair once, without overlapping, so in a run of
// carriage returns it consumed the one adjacent to the line feed and left the
// one in front of it sitting against the new line feed. CR CR LF came back as
// CR LF. The writer therefore stored the exact thing this card exists to stop
// it storing, through the ordinary comment verb and with no editor involved.
//
// The property to hold is not that some particular input works. It is that the
// function's answer is one it would not change again, for every input, because
// every claim of idempotence made of the repair rests on that.
func TestNormalisationIsTrueOfItsOwnAnswer(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a\r\nb", "a\nb"},
		{"a\r\r\nb", "a\nb"},
		{"a\r\r\r\r\r\nb", "a\nb"},
		{"\r\n", "\n"},
		{"\r\r\n", "\n"},
		// A carriage return that ends at no line feed is prose and survives,
		// however many of them there are.
		{"a\rb", "a\rb"},
		{"a\r\rb", "a\r\rb"},
		{"a\r", "a\r"},
		{"\r", "\r"},
		{"", ""},
		{"a\nb", "a\nb"},
		// A run that ends at a line feed goes whole, and a run that does not
		// stays whole, in the same text.
		{"a\r\rb\r\r\nc", "a\r\rb\nc"},
	} {
		got := NormalizeNewlines(c.in)
		if got != c.want {
			t.Errorf("NormalizeNewlines(%q) = %q, wanted %q", c.in, got, c.want)
		}
		if again := NormalizeNewlines(got); again != got {
			t.Errorf("NormalizeNewlines(%q) = %q and answers %q on the second pass, so its own answer is not settled", c.in, got, again)
		}
		if strings.Contains(got, "\r\n") {
			t.Errorf("NormalizeNewlines(%q) = %q, which still carries a pair", c.in, got)
		}
	}
}

// TestTheRepairIsAFixedPointOverEveryShape is the second blocker, at the level
// the claim is made. Four places say a second run finds nothing: the
// specification, newlineTransform's own doc comment, the catalogue context on
// check.newlines-nothing, and the last clause of dinah-514/criteria/21. Each of
// them is true only if a transform's output is a fixed point of that transform,
// and it was not: a file carrying a run of three carriage returns took three
// confirmed runs to clean, each reporting success.
//
// This asserts the property rather than the number of runs, over every branch
// the transform has, because the number of runs is a symptom and the property
// is the thing the four sentences claim.
func TestTheRepairIsAFixedPointOverEveryShape(t *testing.T) {
	cases := []struct{ name, file, text string }{
		{"an anchor body carrying a run", CardAnchor, "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\none\r\r\rtwo\r\r\nthree\r\n"},
		{"a frontmatter value carrying a run", CardAnchor, "---\ntitle: \"add-a\r\r\\nadd-b\"\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n"},
		{"a journal record carrying a run", JournalName, `{"ts":"2026-08-17T09:00:00Z","event":"blocked","actor":"alka","reason":"a\r\r\nb"}` + "\n"},
		{"a journal separated by runs", JournalName, `{"ts":"2026-08-17T09:00:00Z","event":"created","actor":"alka"}` + "\r\r\n" + `{"ts":"2026-08-17T09:01:00Z","event":"moved","actor":"alka"}` + "\r\r\n"},
		{"a note that is not a record file", "note.md", "A note.\r\r\nMore prose.\r\n"},
		{"a body whose final byte is a bare carriage return", CommentAnchor, "---\nactor: alka\nts: \"2026-08-17T09:00:00Z\"\n---\ntext\r"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			once := transformNewlines(c.file, []byte(c.text))
			if once.Condition != "" {
				t.Fatalf("refused as %s (%s)", once.Condition, once.Detail)
			}
			twice := transformNewlines(c.file, once.Out)
			if !bytes.Equal(once.Out, twice.Out) {
				t.Errorf("the transform's answer is not settled:\n in    %q\n once  %q\n twice %q", c.text, once.Out, twice.Out)
			}
			if twice.Returns != 0 {
				t.Errorf("a second pass over %q still removes %d, so the first pass did not finish", once.Out, twice.Returns)
			}
			if where := storedNewlineForm(string(once.Out), c.file); where != "" {
				t.Errorf("the repaired form of %q is %q, which carries %s", c.text, once.Out, where)
			}
			// The carriage returns at the very end of the file are prose, and
			// deleting them satisfies both assertions above: a deletion is
			// settled and carries no stored line ending. This case reached the
			// end-of-file position for a whole round while the repair deleted
			// the byte, because it asked only those two questions.
			if got, want := trailingCarriageReturns(string(once.Out)), trailingCarriageReturns(NormalizeNewlines(c.text)); got != want {
				t.Errorf("the repair left %d carriage returns at the end of %q and there were %d: %q", got, c.text, want, once.Out)
			}
		})
	}
}

// storedNewlineForm reports the first stored form of a line-ending carriage
// return a repaired file still carries, and the empty string where it carries
// none. It is the bench-side twin of the sweep's own reader in cmd/dinah.
func storedNewlineForm(text, name string) string {
	if strings.Contains(text, "\r\n") {
		return "a CRLF pair"
	}
	if strings.Contains(text, "\r\\n") {
		return "the split frontmatter form"
	}
	if filepath.Ext(name) != ".ndjson" {
		return ""
	}
	for index, record := range strings.Split(text, "\n") {
		if strings.TrimSpace(record) == "" {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(record), &decoded); err != nil {
			continue
		}
		for member, value := range decoded {
			if carried, ok := value.(string); ok && strings.Contains(carried, "\r\n") {
				return "an encoded CRLF in record " + strconv.Itoa(index+1) + " at " + member
			}
		}
	}
	return ""
}

// TestAnAnchorDinahWroteIsNeverCalledDamaged holds the repair to never calling
// a file the tool itself wrote damaged, and to never quietly improving one
// either.
//
// A body whose final byte is a bare carriage return is something this format
// keeps on purpose and something Dinah writes through its own comment verb.
// ParseAnchor strips a trailing carriage return from the last line whether or
// not a line feed follows it, so Render cannot put it back.
//
// What the repair does about that now is keep the byte away from the reader:
// the carriage returns at the very end of a file are set aside before the parse
// and put back after the render, and the gate then compares the render against
// the file's own normalised bytes, which is exact. A file Dinah wrote parses,
// renders back to itself, and is left alone.
//
// Both wrong answers to it were shipped before that one, so this case asserts
// against both. The gate first compared against the normalised bytes with the
// byte still in them, so the parse looked unfaithful, and a file bearing a
// fixed anchor name was refused as damaged under a detail blaming a header that
// was perfectly well formed, which stopped every other file in the store from
// being repaired. The gate was then widened to compare against what the anchor
// reader yields, which asks whether the reader agrees with itself, so the
// answer was always yes and the lossy render was written back: the byte was
// deleted instead. A refusal and a deletion are the two halves of one mistake,
// which is handing a byte to a reader that cannot carry it.
func TestAnAnchorDinahWroteIsNeverCalledDamaged(t *testing.T) {
	root := newlineFixture(t)
	comment := filepath.Join(root, CardsDir, "c00000000001", CommentsDir, "m00000000001", CommentAnchor)
	write(t, comment, "---\nactor: alka\nts: \"2026-08-17T09:00:00Z\"\n---\ntext\r")
	// A dirty file elsewhere in the store, so the denial of repair to
	// everything else is what fails if the refusal comes back.
	elsewhere := filepath.Join(root, CardsDir, "c00000000002", CardAnchor)
	write(t, elsewhere, dirtyCard)

	report := migrate(t, root, true)
	for _, conflict := range report.Conflicts {
		t.Errorf("%s was refused as %s (%s), and Dinah wrote it through its own verb", conflict.Path, conflict.Condition, conflict.Detail)
	}
	if strings.Contains(readFile(t, elsewhere), "\r") {
		t.Error("a file elsewhere in the store was not repaired, so one comment denied the repair to everything around it")
	}
	// The byte survives. This assertion said the opposite when it was written,
	// which is how the deletion got past a green suite: the file was held to
	// coming back WITHOUT the carriage return, on the reasoning that no reader
	// of this format returns it. That reasoning is true of the reader and says
	// nothing about what the repair may delete, and dinah-514/decisions/7 says
	// the repair may not.
	if got := readFile(t, comment); !strings.HasSuffix(got, "text\r") {
		t.Errorf("the repair deleted the trailing carriage return, which this format keeps: %q", got)
	}

	// A file that really is damaged still refuses, so the fix has not turned
	// the gate off.
	damaged := newlineFixture(t)
	write(t, filepath.Join(damaged, CardsDir, "c00000000002", CardAnchor), "---\n\nA card anchor somebody broke.\n\n---\n\nMore.\n")
	refused := migrate(t, damaged, true)
	named := false
	for _, conflict := range refused.Conflicts {
		if conflict.Condition == NewlineConflictUnsupported {
			named = true
		}
	}
	if !named {
		t.Errorf("a genuinely damaged anchor was not refused: %+v", refused.Conflicts)
	}
}

// TestAConformingStoreChecksClean is the operator's ruling of 2026-09-15 on
// dinah-514/checklist/388cbe456fbe, taken as option A.
//
// A carriage return not followed by a line feed is legal prose under
// dinah-514/decisions/7 and the repair is required to leave it exactly where it
// is. Reporting such a file made dinah check exit non-zero for ever over a
// store with nothing wrong with it, and the only act that could clear the
// report was editing the prose the decision exists to protect.
//
// Three things are asserted, and the second is the one the ruling is easy to
// implement too widely against.
//
//  1. A file whose carriage returns are ALL loose is reported by nothing.
//  2. A file carrying BOTH kinds is still reported, and reported for the real
//     one. The ruling silences a file whose carriage returns are all loose, not
//     any file that carries one, and widening it to the second would be silent.
//  3. The repair still leaves every loose carriage return exactly where it is,
//     which is what dinah-514/decisions/7 says and what this ruling does not
//     touch.
func TestAConformingStoreChecksClean(t *testing.T) {
	root := newlineFixture(t)
	loose := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	both := filepath.Join(root, CardsDir, "c00000000002", CardAnchor)
	// One in the interior of a line and one as the last byte of the file. The
	// end-of-file position is here because it is the one a repair can delete
	// while every other assertion in this case stays green, and because it is
	// what dinah comment writes.
	write(t, loose, "---\ntitle: \"one\rtwo\"\ncolumn: b00000000001\nstate: ready\n---\nbody one\rbody two\r")
	// The same, and a real stored line ending beside it.
	write(t, both, "---\ntitle: \"one\rtwo\"\ncolumn: b00000000001\nstate: ready\n---\nbody one\rbody two\r\nbody three\n")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.checkStoredNewlines()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	reported := map[string]Finding{}
	for _, finding := range findings {
		reported[finding.Path] = finding
	}
	if got, named := reported[loose]; named {
		t.Errorf("a file whose carriage returns are all loose ones is reported as %s (%s), and such a file conforms", got.Key, got.Detail)
	}
	got, named := reported[both]
	if !named {
		t.Fatalf("a file carrying a real stored line ending is reported by nothing, so the ruling was taken too widely: %+v", findings)
	}
	if got.Key != FindingStoredCarriageReturn {
		t.Errorf("a file carrying both kinds is reported under %s, and it is reported for the one that is not legal", got.Key)
	}
	if !strings.HasPrefix(got.Detail, "1 line-ending") {
		t.Errorf("the detail reads %q and the file carries one stored line ending", got.Detail)
	}
	if !strings.HasSuffix(got.Detail, "2 loose") {
		t.Errorf("the detail reads %q and the file carries two carriage returns the repair keeps", got.Detail)
	}

	// The repair leaves every loose carriage return where it is, which is the
	// half of the contract this ruling does not touch.
	before := readFile(t, loose)
	migrate(t, root, true)
	if after := readFile(t, loose); after != before {
		t.Errorf("the repair touched a file it was to leave alone:\n was %q\n now %q", before, after)
	}
	repaired := readFile(t, both)
	if strings.Contains(repaired, "\r\n") {
		t.Errorf("the file carrying both kinds was not repaired: %q", repaired)
	}
	if !strings.HasSuffix(readFile(t, loose), "body two\r") {
		t.Errorf("the repair deleted the carriage return at the end of the file: %q", readFile(t, loose))
	}
	if strings.Count(repaired, "\r") != 2 {
		t.Errorf("the repair kept %d loose carriage returns and the file carried two: %q", strings.Count(repaired, "\r"), repaired)
	}

	// Nothing is reported once the one real defect is gone, so the store checks
	// clean while still carrying legal prose no tool will ever remove.
	after, err := opened.checkStoredNewlines()
	if err != nil {
		t.Fatalf("check again: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("a conforming store still reports %+v", after)
	}
}

// TestTheGateAndTheRepairOverEveryShapeThisToolWrites is the probe that hunted
// for a route into the refusal, promoted from a throwaway to a case that ships.
//
// It ran once as a scratch program over 294 combinations of body and
// frontmatter value and reported no failure, and that was honest evidence which
// simply did not contain the shape that mattered: every body in it put a
// carriage return inside a line. The sixth route into the gate was a carriage
// return at the END of a body, which the anchor reader cannot carry, and the
// answer to finding it was not to add one case for it but to add the positions
// the old set never reached and keep the search.
//
// The positions now driven, for a body and for a frontmatter value alike:
// inside a line, at the end of a line that a line feed follows, at the very end
// of the file with no line feed after it, as a run at the end of the file, as a
// final line consisting of nothing else, and doubled inside a line.
//
// Four properties, each stated over every combination.
//
//  1. The writer stores no carriage return that stands for a line ending. That
//     is this card's headline claim, made of the tool's own writer.
//  2. The gate routes the file somewhere that can carry it. A file this tool
//     wrote is never refused.
//  3. The repair is settled: its answer is one it would not change again.
//  4. The repair keeps every carriage return the format keeps. Counted at the
//     end of the file, which is the position a repair can delete while every
//     other property here stays true.
func TestTheGateAndTheRepairOverEveryShapeThisToolWrites(t *testing.T) {
	bodies := []string{
		"", "plain", "plain\n", "two\n\nlines",
		"inside\rline", "doubled\r\rinside",
		"ends with a return\r", "ends with a run\r\r",
		"own line\n\r", "own line run\n\r\r",
		"line ending\r\nafter", "run ending\r\r\nafter",
		"mixed\rinterior and ending\r\nand tail\r",
		"---", "---\nmore", "- dashed", "ends with backslash \\",
		"  leading space", "trailing space  ", "tab\tin it",
	}
	values := []string{
		"plain", "", "inside\rvalue", "doubled\r\rvalue",
		"ends with a return\r", "ends with a run\r\r",
		"value ending\r\nafter", "value run ending\r\r\nafter",
		`literal \n backslash`, `quote " inside`, `back \ slash`,
		"colon: space", " leading", "trailing ", "#hash", "-dash",
	}
	names := []string{CardAnchor, CommentAnchor, "note.md"}

	shapes := 0
	for _, name := range names {
		for _, body := range bodies {
			for _, value := range values {
				shapes++
				fm := NewFrontmatter()
				fm.Set("title", value)
				fm.Set("column", "b00000000001")
				// What the tool's own writer would put on disk.
				stored := NormalizeNewlines(fm.Render(body))

				if strings.Contains(stored, "\r\n") {
					t.Errorf("%s: the writer stored a CRLF pair for body=%q value=%q: %q", name, body, value, stored)
				}
				if strings.Contains(stored, "\r\\n") {
					t.Errorf("%s: the writer stored the split frontmatter form for body=%q value=%q: %q", name, body, value, stored)
				}

				once := transformNewlines(name, []byte(stored))
				if once.Condition != "" {
					t.Errorf("%s: a file this tool wrote was refused as %s (%s) for body=%q value=%q: %q",
						name, once.Condition, once.Detail, body, value, stored)
					continue
				}
				twice := transformNewlines(name, once.Out)
				if !bytes.Equal(once.Out, twice.Out) {
					t.Errorf("%s: the repair is not settled for body=%q value=%q:\n once  %q\n twice %q",
						name, body, value, once.Out, twice.Out)
				}
				if got, want := trailingCarriageReturns(string(once.Out)), trailingCarriageReturns(stored); got != want {
					t.Errorf("%s: the repair left %d carriage returns at the end of the file and there were %d, for body=%q value=%q: %q",
						name, got, want, body, value, once.Out)
				}
				// A file the tool wrote carries nothing to repair, so the
				// repair has nothing to do with it at all. This is the
				// strongest of the four and the one that would have caught the
				// deletion on its own.
				if !bytes.Equal(once.Out, []byte(stored)) {
					t.Errorf("%s: the repair changed a file this tool wrote, for body=%q value=%q:\n was %q\n now %q",
						name, body, value, stored, once.Out)
				}

				// The same file as an external editor would leave it, which is
				// the one site no write-path change reaches and the shape the
				// fifth route into the refusal lived in: an anchor whose fence
				// lines carried a doubled carriage return was refused as
				// damaged and stopped the repair of the whole store.
				//
				// Normalising an editor's variant gives back the bytes the
				// tool wrote, so the repair has to land on exactly those, and
				// nothing here may be refused.
				for _, ending := range []string{"\r\n", "\r\r\n", "\r\r\r\n"} {
					mangled := strings.ReplaceAll(stored, "\n", ending)
					result := transformNewlines(name, []byte(mangled))
					if result.Condition != "" {
						t.Errorf("%s: an editor's variant was refused as %s (%s) for body=%q value=%q: %q",
							name, result.Condition, result.Detail, body, value, mangled)
						continue
					}
					if !bytes.Equal(result.Out, []byte(stored)) {
						t.Errorf("%s: an editor's variant did not come back to what the tool wrote, for body=%q value=%q:\n wrote   %q\n mangled %q\n got     %q",
							name, body, value, stored, mangled, result.Out)
					}
					if settled := transformNewlines(name, result.Out); !bytes.Equal(settled.Out, result.Out) {
						t.Errorf("%s: an editor's variant is not settled for body=%q value=%q: %q then %q",
							name, body, value, result.Out, settled.Out)
					}
				}
			}
		}
	}
	if shapes == 0 {
		t.Fatal("the probe drove no shape")
	}
	t.Logf("drove %d shapes across %d file names", shapes, len(names))
}

// trailingCarriageReturns counts the carriage returns at the very end of a
// text, which no line feed follows and which are therefore prose rather than a
// line ending.
//
// It exists because a deletion of them satisfies every other question these
// cases ask: a deleted byte leaves a file that is settled, that carries no
// stored line ending, and that a second run finds nothing to do with. Counting
// them is the one question that separates a repair from a deletion.
func trailingCarriageReturns(text string) int {
	return len(text) - len(strings.TrimRight(text, "\r"))
}
