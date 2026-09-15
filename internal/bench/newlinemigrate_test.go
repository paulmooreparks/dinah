package bench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
		if bytes.Contains(data, []byte("\r")) {
			carrying[path] = true
		}
	}
	t.Logf("the corpus carries %d files, of which %d are changed by their own transform and %d carry a carriage return", len(paths), len(changed), len(carrying))
	for path := range changed {
		if !carrying[path] {
			t.Errorf("%s is changed by its own transform and carries no carriage return, so the detector produces a false destination", path)
		}
	}
	for path := range carrying {
		if !changed[path] {
			t.Errorf("%s carries a carriage return and its own transform leaves it alone", path)
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
// pass. The destination is made genuinely read-only and nothing is injected,
// because a writability check hard-coded to succeed passes any test that
// injects its failure, and because a probe of the containing directory passes
// this exact case: os.CreateTemp there answers nil while the write then fails
// on the rename.
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
	if err := os.Chmod(locked, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
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
		if err := os.Chmod(second, 0o444); err != nil {
			t.Errorf("chmod: %v", err)
		}
	}
	t.Cleanup(func() { newlineHook = nil; os.Chmod(second, 0o644) })

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
		if conflict.Condition == NewlineConflictUnsupported && conflict.Detail == "header does not round-trip" {
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
// finding to the three forms, to agreeing with the migration about which files
// are dirty, and to leaving a file whose only carriage returns are loose ones
// out of the destinations.
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
		if finding.Key != FindingStoredCarriageReturn {
			t.Errorf("the sweep reported %s", finding.Key)
		}
		reported[finding.Path] = finding.Detail
	}
	for _, path := range []string{pair, split, escaped, loose} {
		if _, named := reported[path]; !named {
			t.Errorf("%s was not reported", path)
		}
	}
	if got := reported[loose]; !strings.HasPrefix(got, "0 line-ending") {
		t.Errorf("the file carrying only loose carriage returns reads %q, and it is reported with a line-ending count of zero", got)
	}

	// The finding and the preview agree about which files are dirty, asserted
	// as a set against a set.
	preview := migrate(t, root, false)
	planned := destinations(preview)
	for path, detail := range reported {
		dirty := !strings.HasPrefix(detail, "0 line-ending")
		if dirty != planned[path] {
			t.Errorf("%s is reported as %q and the preview %v it as a destination", path, detail, planned[path])
		}
	}

	migrate(t, root, true)
	after, err := opened.checkStoredNewlines()
	if err != nil {
		t.Fatalf("check again: %v", err)
	}
	for _, finding := range after {
		if finding.Path != loose {
			t.Errorf("%s is still reported after the repair: %s", finding.Path, finding.Detail)
		}
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

// TestALoneCarriageReturnSurvivesEverywhere holds the one deliberate keep: a
// 0x0D not followed by a 0x0A is a character the prose meant to carry rather
// than a line ending, because SplitLines strips only a TRAILING carriage return
// per line, so an interior one is returned to the caller today.
func TestALoneCarriageReturnSurvivesEverywhere(t *testing.T) {
	root := newlineFixture(t)
	anchor := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	journal := filepath.Join(root, CardsDir, "c00000000001", JournalName)
	write(t, anchor, "---\ntitle: \"one\rtwo\"\ncolumn: b00000000001\nstate: ready\n---\nbody one\rbody two\n")
	record, err := json.Marshal(map[string]string{"ts": "2026-08-17T09:00:00Z", "event": "blocked", "actor": "alka", "reason": "a\rb"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	write(t, journal, string(record)+"\n")
	before := everyFileUnder(t, root)

	migrate(t, root, true)
	if got := readFile(t, anchor); got != before[anchor] {
		t.Errorf("the anchor came back as %q and was %q", got, before[anchor])
	}
	if got := readFile(t, journal); got != before[journal] {
		t.Errorf("the journal came back as %q and was %q", got, before[journal])
	}
	fm, body := ParseAnchor(readFile(t, anchor))
	if got := fm.Value("title"); got != "one\rtwo" {
		t.Errorf("the value reads back as %q", got)
	}
	if !strings.Contains(body, "body one\rbody two") {
		t.Errorf("the body reads back as %q", body)
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
