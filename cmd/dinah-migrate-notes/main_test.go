package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// The tests below drive the whole program through run rather than its parts,
// on dinah-migrate-actors' own terms: the thing under test is a one-shot
// command whose answer is an exit code and a report, and a caller reading
// either gets both together.
//
// Every fixture is built in a temporary directory. No test here opens a path
// outside t.TempDir, because the program's whole purpose is to rewrite a store
// in place and a fixture reaching live data would rewrite it.

// plantedItem is one checklist item a fixture carries.
type plantedItem struct {
	// id is the item's own identifier, written out so a test can name the
	// item it asserts about.
	id string
	// kind and state are the item's own two fields.
	kind  string
	state string
	// ordinal is the item's position among the card's items.
	ordinal int
	// note is the retired key's value, empty on an item the migration has
	// no work for.
	note string
	// text is the item's body.
	text string
}

// plantStore writes a contained workbench carrying one card and the items a
// test names, and returns the workbench root and the card's directory.
//
// The anchor declares format 5, which is the format every store this program
// is written for declares, and the workbench sits inside a .dinah container
// under a minted identifier, because the containment rule binds from format 2
// and an uncontained store is refused before this program's own work begins.
func plantStore(t *testing.T, items ...plantedItem) (root, cardDir string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), bench.UserBaseName, "0199a1b2c3d47abc8000000000000001")
	write(t, filepath.Join(root, bench.WorkbenchAnchor), strings.Join([]string{
		"---",
		"format: 5",
		"profile: dinah-core/0.17",
		"title: Fixture",
		"slug: fx",
		"operator: ana",
		"columns:",
		"  - c00000000001",
		"---",
		"",
		"Standing text.",
		"",
	}, "\n"))
	write(t, filepath.Join(root, bench.ColumnsDir, "c00000000001", bench.ColumnAnchor), strings.Join([]string{
		"---",
		"title: Doing",
		"slug: doing",
		"kind: work",
		"---",
		"",
		"Column text.",
		"",
	}, "\n"))
	write(t, filepath.Join(root, bench.CardNumbersName), "1 d00000000001\n")
	cardDir = filepath.Join(root, bench.CardsDir, "d00000000001")
	write(t, filepath.Join(cardDir, bench.CardAnchor), strings.Join([]string{
		"---",
		"title: A card",
		"column: c00000000001",
		"state: ready",
		"---",
		"",
		"The card's body.",
		"",
	}, "\n"))
	for _, item := range items {
		header := []string{
			"---",
			"kind: " + item.kind,
			"state: " + item.state,
			"ordinal: " + itoa(item.ordinal),
			"ts: 2026-08-01T09:00:00Z",
		}
		if item.note != "" {
			header = append(header, "note: "+item.note)
		}
		header = append(header, "---", "", item.text, "")
		write(t, filepath.Join(cardDir, bench.ChecklistDir, item.id, bench.ItemAnchor), strings.Join(header, "\n"))
	}
	return root, cardDir
}

// plantJournal writes the card's journal from the lines a test names.
func plantJournal(t *testing.T, cardDir string, lines ...string) {
	t.Helper()
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	write(t, filepath.Join(cardDir, bench.JournalName), body)
}

// settledLine is one journal line recording that an item was settled.
func settledLine(event, item, actor, ts string) string {
	return `{"ts":"` + ts + `","event":"` + event + `","actor":{"name":"` + actor + `"},"item":"` + item + `"}`
}

// write lays one file down, making its parents.
func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// itoa is strconv.Itoa under a shorter name, because the fixture builder above
// reads better without the package qualifier on every ordinal.
func itoa(n int) string {
	digits := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// runProgram drives one invocation and returns its exit code and what it wrote
// to each stream.
func runProgram(t *testing.T, args ...string) (code int, out, errw string) {
	t.Helper()
	outFile := tempFile(t, "out")
	errFile := tempFile(t, "err")
	code = run(args, outFile, errFile)
	return code, readBack(t, outFile), readBack(t, errFile)
}

// tempFile opens a file the program can write to as an *os.File, which is what
// run takes so that a shipped binary hands it the real console.
func tempFile(t *testing.T, name string) *os.File {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	t.Cleanup(func() { file.Close() })
	return file
}

// readBack reads what a stream file holds.
func readBack(t *testing.T, file *os.File) string {
	t.Helper()
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read %s: %v", file.Name(), err)
	}
	return string(data)
}

// itemAnchor reads one item's header and body back off disk.
func itemAnchor(t *testing.T, cardDir, id string) (*bench.Frontmatter, string) {
	t.Helper()
	fm, body, err := bench.ReadItemAnchor(filepath.Join(cardDir, bench.ChecklistDir, id))
	if err != nil {
		t.Fatalf("read the item %s: %v", id, err)
	}
	return fm, body
}

// itemComments reads the comments one item carries.
func itemComments(t *testing.T, cardDir, id string) []*bench.Comment {
	t.Helper()
	comments, err := bench.Comments(filepath.Join(cardDir, bench.ChecklistDir, id))
	if err != nil {
		t.Fatalf("read the comments of %s: %v", id, err)
	}
	return comments
}

// TestTheMigrationCarriesBothArms asserts dinah-525/criteria/10: every item
// carrying a note is migrated whatever state it is in, a settled item gains a
// resolution aimed at the new comment, a pending item carrying the note of an
// undone settling gains the comment and no resolution, and no note key
// survives in either arm.
//
// The two arms are asserted together because that is what makes "no note key
// behind" true. Scoped to settled items alone, the claim and the demand
// contradict each other: a pending item carrying a note would either keep it or
// lose its words.
func TestTheMigrationCarriesBothArms(t *testing.T) {
	root, cardDir := plantStore(t,
		plantedItem{id: "e00000000001", kind: "acceptance_criterion", state: "verified", ordinal: 1,
			note: "the endpoint answers 404", text: "The endpoint returns 404 for an unknown id."},
		plantedItem{id: "e00000000002", kind: "open_question", state: "pending", ordinal: 2,
			note: "the operator ruled it does not move", text: "Does the deadline move?"},
	)
	plantJournal(t, cardDir,
		settledLine("item_verified", "e00000000001", "ana", "2026-08-02T10:00:00Z"),
		settledLine("item_resolved", "e00000000002", "bo", "2026-08-02T11:00:00Z"),
	)

	code, out, errw := runProgram(t, root, "--apply")
	if code != exitDone {
		t.Fatalf("the run exited %d, wanted %d\n%s\n%s", code, exitDone, out, errw)
	}

	// The settled arm.
	fm, _ := itemAnchor(t, cardDir, "e00000000001")
	if got := fm.Value(bench.ItemNoteRetiredField); got != "" {
		t.Errorf("the settled item kept its note %q", got)
	}
	if got := fm.Value(bench.ItemResolutionField); got != "fx-1/criteria/1/comments/1" {
		t.Errorf("the settled item designates %q, wanted the comment the run wrote", got)
	}
	settled := itemComments(t, cardDir, "e00000000001")
	if len(settled) != 1 || settled[0].Body != "the endpoint answers 404" {
		t.Fatalf("the settled item carries %+v, wanted one comment holding the note's words", settled)
	}
	if settled[0].Author != "ana" {
		t.Errorf("the comment's author is %q, wanted the actor the journal records as having settled the item", settled[0].Author)
	}
	if settled[0].TS != "2026-08-02T10:00:00Z" {
		t.Errorf("the comment's timestamp is %q, wanted the settling's own", settled[0].TS)
	}

	// The pending arm.
	pending, _ := itemAnchor(t, cardDir, "e00000000002")
	if got := pending.Value(bench.ItemNoteRetiredField); got != "" {
		t.Errorf("the pending item kept its note %q", got)
	}
	if got := pending.Value(bench.ItemResolutionField); got != "" {
		t.Errorf("the pending item designates %q, and a pending item has no answer of record", got)
	}
	undone := itemComments(t, cardDir, "e00000000002")
	if len(undone) != 1 || undone[0].Body != "the operator ruled it does not move" {
		t.Fatalf("the pending item carries %+v, wanted one comment holding the words the reopen left behind", undone)
	}
}

// TestTheMigrationRefusesRatherThanInventingAnActor asserts
// dinah-525/criteria/11: an item whose journal records no settling is named in
// a manifest and nothing is written; a rerun with --unattributed writes that
// comment with no author at all and author_unrecoverable on its anchor; a
// truncated journal reads the same way; and a store that needs neither is
// carried through untouched by the refusal.
func TestTheMigrationRefusesRatherThanInventingAnActor(t *testing.T) {
	t.Run("no settling event", func(t *testing.T) {
		root, cardDir := plantStore(t,
			plantedItem{id: "e00000000001", kind: "decision", state: "resolved", ordinal: 1,
				note: "it mirrors the lock", text: "Whose lock covers the write."},
		)
		plantJournal(t, cardDir)

		code, out, _ := runProgram(t, root, "--apply")
		if code != exitUnattributed {
			t.Fatalf("the run exited %d, wanted %d\n%s", code, exitUnattributed, out)
		}
		if !strings.Contains(out, "fx-1/decisions/1") {
			t.Errorf("the manifest does not name the item it cannot attribute:\n%s", out)
		}
		// Nothing was written, which is the half of the refusal that
		// matters: a manifest beside a half-converted store would be a
		// report of what the run had already done.
		fm, _ := itemAnchor(t, cardDir, "e00000000001")
		if got := fm.Value(bench.ItemNoteRetiredField); got != "it mirrors the lock" {
			t.Errorf("the refused run rewrote the item, whose note now reads %q", got)
		}
		if comments := itemComments(t, cardDir, "e00000000001"); len(comments) != 0 {
			t.Errorf("the refused run wrote %d comments", len(comments))
		}

		code, out, _ = runProgram(t, root, "--apply", "--unattributed")
		if code != exitDone {
			t.Fatalf("the rerun exited %d, wanted %d\n%s", code, exitDone, out)
		}
		comments := itemComments(t, cardDir, "e00000000001")
		if len(comments) != 1 {
			t.Fatalf("the rerun wrote %d comments, wanted one", len(comments))
		}
		if comments[0].Author != "" {
			t.Errorf("the comment's author is %q, and an author the store cannot recover is recorded as absent rather than as a name", comments[0].Author)
		}
		if !comments[0].AuthorUnrecoverable {
			t.Error("the comment does not record author_unrecoverable, so its missing author reads as an omission rather than as a finding")
		}
		// The timestamp comes from the item's own anchor, which is a fact
		// the store holds, rather than from a clock this program reads.
		if comments[0].TS != "2026-08-01T09:00:00Z" {
			t.Errorf("the comment's timestamp is %q, wanted the item's own", comments[0].TS)
		}
	})

	t.Run("a truncated journal", func(t *testing.T) {
		root, cardDir := plantStore(t,
			plantedItem{id: "e00000000001", kind: "decision", state: "resolved", ordinal: 1,
				note: "it mirrors the lock", text: "Whose lock covers the write."},
		)
		// The settling line is cut off mid-way, which is what a process
		// killed during an append leaves.
		write(t, filepath.Join(cardDir, bench.JournalName),
			`{"ts":"2026-08-02T10:00:00Z","event":"item_resol`)

		code, out, _ := runProgram(t, root, "--apply")
		if code != exitUnattributed {
			t.Fatalf("the run exited %d, wanted %d\n%s", code, exitUnattributed, out)
		}
		if !strings.Contains(out, "fx-1/decisions/1") {
			t.Errorf("the manifest does not name the item the torn journal cannot attribute:\n%s", out)
		}
	})

	t.Run("a clean store", func(t *testing.T) {
		root, cardDir := plantStore(t,
			plantedItem{id: "e00000000001", kind: "decision", state: "resolved", ordinal: 1,
				note: "it mirrors the lock", text: "Whose lock covers the write."},
		)
		plantJournal(t, cardDir, settledLine("item_resolved", "e00000000001", "ana", "2026-08-02T10:00:00Z"))

		code, out, errw := runProgram(t, root, "--apply")
		if code != exitDone {
			t.Fatalf("a store that needs no choice exited %d\n%s\n%s", code, out, errw)
		}
		if strings.Contains(out, "cannot attribute") {
			t.Errorf("a store the journal attributes wholly was reported as needing a choice:\n%s", out)
		}
	})
}

// TestTheMigrationIsIdempotentOverItsOwnHalfStates asserts
// dinah-525/criteria/14: the run is killed between the two writes of a single
// item, and after each kill a rerun completes, the item ends with exactly one
// converted comment, no note survives, and the report names what had landed.
//
// The kill is planted rather than simulated by stopping the process, because
// what the criterion is about is the state on disk rather than the mechanism
// that produced it. Each subtest builds the exact half-state a kill at that
// point leaves and then reruns over it.
func TestTheMigrationIsIdempotentOverItsOwnHalfStates(t *testing.T) {
	plant := func(t *testing.T) (string, string) {
		t.Helper()
		root, cardDir := plantStore(t,
			plantedItem{id: "e00000000001", kind: "acceptance_criterion", state: "verified", ordinal: 1,
				note: "the endpoint answers 404", text: "The endpoint returns 404 for an unknown id."},
		)
		plantJournal(t, cardDir, settledLine("item_verified", "e00000000001", "ana", "2026-08-02T10:00:00Z"))
		return root, cardDir
	}
	itemDir := func(cardDir string) string {
		return filepath.Join(cardDir, bench.ChecklistDir, "e00000000001")
	}
	assertFinished := func(t *testing.T, cardDir string) {
		t.Helper()
		fm, _ := itemAnchor(t, cardDir, "e00000000001")
		if got := fm.Value(bench.ItemNoteRetiredField); got != "" {
			t.Errorf("a note survived the rerun: %q", got)
		}
		if got := fm.Value(bench.ItemResolutionField); got != "fx-1/criteria/1/comments/1" {
			t.Errorf("the item designates %q after the rerun", got)
		}
		comments := itemComments(t, cardDir, "e00000000001")
		if len(comments) != 1 {
			t.Fatalf("the item carries %d comments after the rerun, wanted exactly one", len(comments))
		}
		if comments[0].Body != "the endpoint answers 404" {
			t.Errorf("the converted comment reads %q", comments[0].Body)
		}
		// The ordinal is stamped only on the branch that actually writes a
		// comment. A rerun that stamped it again on the branch finding one
		// already there would move this comment off position one, and on a
		// collection holding two it would hand the second the same position,
		// which dinah check reports as a duplicate.
		if comments[0].Ordinal != 1 {
			t.Errorf("the converted comment stands at ordinal %d, wanted the one the run that wrote it stamped", comments[0].Ordinal)
		}
		if got := bench.CommentDigest(comments[0].Body); got != comments[0].Digest {
			t.Errorf("the converted comment's digest is %q and its body hashes to %q", comments[0].Digest, got)
		}
	}

	t.Run("killed before either write", func(t *testing.T) {
		root, cardDir := plant(t)
		code, out, errw := runProgram(t, root, "--apply")
		if code != exitDone {
			t.Fatalf("the rerun exited %d\n%s\n%s", code, out, errw)
		}
		assertFinished(t, cardDir)
		if !strings.Contains(out, "fx-1/criteria/1") {
			t.Errorf("the report does not name the item that landed:\n%s", out)
		}
	})

	t.Run("killed between the comment and the item", func(t *testing.T) {
		root, cardDir := plant(t)
		// The state a kill after the comment write and before the item
		// write leaves: the comment is there and the item still carries its
		// note. An idempotence resting on the note key alone would read this
		// item as unmigrated and write the text a second time.
		dir := filepath.Join(itemDir(cardDir), bench.CommentsDir, convertedCommentID("e00000000001"))
		fm := bench.NewFrontmatter()
		fm.Set("ts", "2026-08-02T10:00:00Z")
		fm.Set("author", "ana")
		fm.Set(bench.OrdinalField, "1")
		if err := bench.WriteCommentAnchor(dir, fm, "the endpoint answers 404"); err != nil {
			t.Fatalf("plant the half-written comment: %v", err)
		}

		code, out, errw := runProgram(t, root, "--apply")
		if code != exitDone {
			t.Fatalf("the rerun exited %d\n%s\n%s", code, out, errw)
		}
		assertFinished(t, cardDir)
	})

	t.Run("killed mid-write with the directory standing and empty", func(t *testing.T) {
		root, cardDir := plant(t)
		// A run killed while writing the comment can leave the directory
		// there and no anchor in it. A restart check reading the directory
		// would call the item converted and skip writing the comment it
		// never wrote, which is why the check reads the anchor file.
		dir := filepath.Join(itemDir(cardDir), bench.CommentsDir, convertedCommentID("e00000000001"))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("plant the empty directory: %v", err)
		}

		code, out, errw := runProgram(t, root, "--apply")
		if code != exitDone {
			t.Fatalf("the rerun exited %d\n%s\n%s", code, out, errw)
		}
		assertFinished(t, cardDir)
	})

	t.Run("run twice over", func(t *testing.T) {
		root, cardDir := plant(t)
		if code, out, errw := runProgram(t, root, "--apply"); code != exitDone {
			t.Fatalf("the first run exited %d\n%s\n%s", code, out, errw)
		}
		if code, out, errw := runProgram(t, root, "--apply"); code != exitDone {
			t.Fatalf("the second run exited %d\n%s\n%s", code, out, errw)
		}
		assertFinished(t, cardDir)
	})
}

// TestARerunLeavesAnEditedConversionAlone asserts the last row of
// dinah-525/criteria/14: a converted comment somebody has since edited is not
// overwritten by a rerun, and the item write completes pointing at what is
// there.
//
// The operator's own words outrank the migration's copy of them, and the
// property rests on the restart check rather than on a comparison of bodies:
// the comment is at the derived path, so the run writes no comment in its
// place whatever it says.
func TestARerunLeavesAnEditedConversionAlone(t *testing.T) {
	root, cardDir := plantStore(t,
		plantedItem{id: "e00000000001", kind: "acceptance_criterion", state: "verified", ordinal: 1,
			note: "the endpoint answers 404", text: "The endpoint returns 404 for an unknown id."},
	)
	plantJournal(t, cardDir, settledLine("item_verified", "e00000000001", "ana", "2026-08-02T10:00:00Z"))
	if code, out, errw := runProgram(t, root, "--apply"); code != exitDone {
		t.Fatalf("the first run exited %d\n%s\n%s", code, out, errw)
	}

	// The operator rewrites the converted comment, then somebody reruns the
	// migration over the store.
	dir := filepath.Join(cardDir, bench.ChecklistDir, "e00000000001", bench.CommentsDir, convertedCommentID("e00000000001"))
	fm, _, err := bench.ReadCommentAnchor(dir)
	if err != nil {
		t.Fatalf("read the converted comment: %v", err)
	}
	const edited = "the endpoint answers 404, and the fixture that proves it is new"
	if err := bench.WriteCommentAnchor(dir, fm, edited); err != nil {
		t.Fatalf("edit the converted comment: %v", err)
	}
	if code, out, errw := runProgram(t, root, "--apply"); code != exitDone {
		t.Fatalf("the rerun exited %d\n%s\n%s", code, out, errw)
	}

	comments := itemComments(t, cardDir, "e00000000001")
	if len(comments) != 1 {
		t.Fatalf("the item carries %d comments after the rerun, wanted one", len(comments))
	}
	if comments[0].Body != edited {
		t.Errorf("the rerun overwrote the edited comment, which now reads %q", comments[0].Body)
	}
}

// TestThePreviewWritesNothing asserts the two-phase property the migration
// rests on in place of atomicity: a run without --apply classifies exactly as
// the write pass does and leaves the store as it found it.
func TestThePreviewWritesNothing(t *testing.T) {
	root, cardDir := plantStore(t,
		plantedItem{id: "e00000000001", kind: "acceptance_criterion", state: "verified", ordinal: 1,
			note: "the endpoint answers 404", text: "The endpoint returns 404 for an unknown id."},
	)
	plantJournal(t, cardDir, settledLine("item_verified", "e00000000001", "ana", "2026-08-02T10:00:00Z"))

	code, out, _ := runProgram(t, root)
	if code != exitClassified {
		t.Fatalf("the preview exited %d, wanted %d\n%s", code, exitClassified, out)
	}
	if !strings.Contains(out, "fx-1/criteria/1") {
		t.Errorf("the preview does not name the item it would convert:\n%s", out)
	}
	fm, _ := itemAnchor(t, cardDir, "e00000000001")
	if got := fm.Value(bench.ItemNoteRetiredField); got != "the endpoint answers 404" {
		t.Errorf("the preview rewrote the item, whose note now reads %q", got)
	}
	if comments := itemComments(t, cardDir, "e00000000001"); len(comments) != 0 {
		t.Errorf("the preview wrote %d comments", len(comments))
	}
}

// TestTheRunStampsTheFormatLast asserts that the store a finished run leaves
// declares the format the change arrived at, which is what makes an ordinary
// read of it stop refusing.
func TestTheRunStampsTheFormatLast(t *testing.T) {
	root, cardDir := plantStore(t,
		plantedItem{id: "e00000000001", kind: "acceptance_criterion", state: "verified", ordinal: 1,
			note: "the endpoint answers 404", text: "The endpoint returns 404 for an unknown id."},
	)
	plantJournal(t, cardDir, settledLine("item_verified", "e00000000001", "ana", "2026-08-02T10:00:00Z"))

	if _, err := bench.Open(root); err == nil {
		t.Fatal("the store opened before the migration ran, so the refusal this run clears is not in force")
	}
	if code, out, errw := runProgram(t, root, "--apply"); code != exitDone {
		t.Fatalf("the run exited %d\n%s\n%s", code, out, errw)
	}
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("the migrated store still refuses an ordinary read: %v", err)
	}
	if opened.Format != bench.ResolutionFormat {
		t.Errorf("the migrated store declares format %d, wanted %d", opened.Format, bench.ResolutionFormat)
	}
}
