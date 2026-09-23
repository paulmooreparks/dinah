package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// workstreamAttachmentFixture builds a workbench carrying one workstream with
// one card joined to it. It answers the container the commands are run from,
// the workbench directory the on-disk assertions are written against, the
// card's alias, the workstream's own directory and a source file to attach.
func workstreamAttachmentFixture(t *testing.T, slug string) (string, string, string, string) {
	t.Helper()
	root := newBench(t)
	card := addCard(t, root, "a card in the stream")
	mustRunCLI(t, root, "workstream", "new", "Portfolio work", "--slug", slug)
	mustRunCLI(t, root, "join", card, "workstream/"+slug)
	source := filepath.Join(t.TempDir(), "evidence.txt")
	if err := os.WriteFile(source, []byte("the evidence"), 0o644); err != nil {
		t.Fatalf("write the source: %v", err)
	}
	return root, card, workstreamDir(t, root, slug), source
}

// workstreamDir is the directory `dinah path` answers for a workstream, which
// is the one named for its identifier. Every on-disk assertion below is
// written against it, and the slug-rename test rests on it not moving.
func workstreamDir(t *testing.T, root, slug string) string {
	t.Helper()
	got := runCLI(t, root, "path", "workstream/"+slug)
	if got.code != 0 {
		t.Fatalf("path workstream/%s: %d %s", slug, got.code, got.errw)
	}
	dir := strings.TrimSpace(got.out)
	if len(filepath.Base(dir)) != 12 {
		t.Fatalf("path answered %q, whose last segment is not a twelve-hex identifier", got.out)
	}
	return dir
}

// benchOf is the workbench directory itself, which is where the archive mirror
// and the workbench journal live. It is read off the workstream's own
// directory rather than off the container, because a workbench sits under a
// container's .dinah and the tests here already hold a path inside it.
func benchOf(stream string) string {
	return filepath.Dir(filepath.Dir(stream))
}

// TestAWorkstreamTakesAnAttachmentAndEveryVerbReachesIt walks the address
// dinah-583 opens through the terminal: attach writes it, the collection
// reads it, show refuses the collection, path answers the anchor's directory
// and the payload file, and the containment walk draws the attachments ahead
// of the membership. Which file edit opens is asserted in internal/bench,
// where the answer can be read rather than inferred from an editor launch.
//
// The collection's header is asserted under both spellings a caller can type,
// and it reads the same under each. AttachmentListing.Ref carries the
// reference the resolver composed rather than the one the reader typed, which
// is what that field's own doc comment states and what every other holder
// already does, so the identifier-headed listing heads with the slug.
//
// Arming: removing KindWorkstream's mount from the containment table reddens
// the attach, the listing, the path and the tree assertions together.
func TestAWorkstreamTakesAnAttachmentAndEveryVerbReachesIt(t *testing.T) {
	root, card, stream, source := workstreamAttachmentFixture(t, "portfolio")

	attached := mustRunCLI(t, root, "attach", "workstream/portfolio", source)
	var response struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(attached.out), &response); err != nil {
		t.Fatalf("decode attach: %v\n%s", err, attached.out)
	}
	if len(response.Detail) != 12 {
		t.Errorf("attach answered %q, wanted the new attachment's twelve-hex identifier", response.Detail)
	}
	payload := filepath.Join(stream, bench.AttachmentsDir, response.Detail, bench.PayloadDir, "evidence.txt")
	if !bench.Exists(payload) {
		t.Fatalf("wanted the payload at %s", payload)
	}

	// The collection's two answers. show refuses it, list heads its table
	// with the reference the caller typed.
	refused := runCLI(t, root, "show", "workstream/portfolio/attachments")
	if refused.code == 0 {
		t.Errorf("show on the collection succeeded, and a collection is not an entity:\n%s", refused.out)
	}
	if !strings.HasPrefix(refused.errw, contract.IsACollection) {
		t.Errorf("show on the collection refuses %q, wanted %s", refused.errw, contract.IsACollection)
	}
	for _, spelling := range []string{"workstream/portfolio", "workstream/" + filepath.Base(stream)} {
		listed := runCLI(t, root, "list", spelling+"/attachments")
		if listed.code != 0 {
			t.Errorf("list %s/attachments: %d %s", spelling, listed.code, listed.errw)
			continue
		}
		if want := "workstream/portfolio carries 1 attachments."; !strings.Contains(listed.out, want) {
			t.Errorf("the listing heads with something other than %q:\n%s", want, listed.out)
		}
	}

	// path answers the attachment's directory and its payload file, and edit
	// opens the anchor rather than the bytes.
	dir := runCLI(t, root, "path", "workstream/portfolio/attachments/1")
	if !strings.Contains(dir.out, response.Detail) {
		t.Errorf("path on the attachment answered:\n%s\nwanted the directory named for %s", dir.out, response.Detail)
	}
	bytesPath := runCLI(t, root, "path", "workstream/portfolio/attachments/1/payload")
	if !strings.Contains(bytesPath.out, "evidence.txt") {
		t.Errorf("path on the payload answered:\n%s\nwanted the payload file", bytesPath.out)
	}

	// The containment walk draws the attachment before the member card, and
	// the root count covers both.
	tree := mustRunCLI(t, root, "list", "workstream/portfolio", "--depth", "entities")
	var drawn struct {
		Root struct {
			Count    int `json:"count"`
			Children []struct {
				Kind string `json:"kind"`
				Ref  string `json:"ref"`
			} `json:"children"`
		} `json:"root"`
	}
	if err := json.Unmarshal([]byte(tree.out), &drawn); err != nil {
		t.Fatalf("decode the tree: %v\n%s", err, tree.out)
	}
	children := drawn.Root.Children
	if len(children) != 2 {
		t.Fatalf("the walk drew %d children, wanted the attachment and the card: %+v", len(children), children)
	}
	if children[0].Kind != bench.KindAttachment {
		t.Errorf("the first child is a %s, and the attachments are drawn before the membership", children[0].Kind)
	}
	if want := "workstream/portfolio/attachments/1"; children[0].Ref != want {
		t.Errorf("the attachment is drawn as %q, wanted %q", children[0].Ref, want)
	}
	if children[1].Kind != bench.KindCard || children[1].Ref != card {
		t.Errorf("the second child is %s %q, wanted the card %q", children[1].Kind, children[1].Ref, card)
	}
	// One attachment plus one card, each contributing itself and what it
	// holds; the card holds nothing here.
	if drawn.Root.Count != 2 {
		t.Errorf("the root counts %d, wanted 2: one attachment and one card", drawn.Root.Count)
	}
}

// TestAWorkstreamAttachmentJournalsOnItsWorkstream asserts the defect
// dinah-583 section 5.2 fixes. attach already landed on the workstream's own
// journal, because the entity it is handed is the workstream; rename,
// --replace, archive and restore landed on the workbench's, because the entity
// those are handed is the attachment, so one attachment's life was split
// across two journals.
//
// Both journals are counted, separately. A single total would pass against a
// build that wrote every line to one of them. The workbench's journal carries
// none of the five, and it is not enough to observe that: a journal nobody
// writes to reads empty for every reason at once, so the test goes on to
// attach to the workbench itself and hold that journal to exactly the one line
// that act writes. Without the control, a build that had stopped journalling
// altogether would pass this half.
//
// Arming: deleting the attachmentWorkstream arm from journalFor puts four of
// the five events on the workbench's journal, and both counts go red.
func TestAWorkstreamAttachmentJournalsOnItsWorkstream(t *testing.T) {
	root, _, stream, source := workstreamAttachmentFixture(t, "portfolio")
	replacement := filepath.Join(t.TempDir(), "evidence.txt")
	if err := os.WriteFile(replacement, []byte("other bytes"), 0o644); err != nil {
		t.Fatalf("write the replacement: %v", err)
	}

	mustRunCLI(t, root, "attach", "workstream/portfolio", source)
	mustRunCLI(t, root, "rename", "workstream/portfolio/attachments/1", "renamed.txt")
	mustRunCLI(t, root, "attach", "workstream/portfolio/attachments/1", replacement, "--replace")
	mustRunCLI(t, root, "archive", "workstream/portfolio/attachments/1")
	mustRunCLI(t, root, "restore", "workstream/portfolio/attachments/1", "--archived")

	written := eventNames(t, filepath.Join(stream, bench.JournalName))
	workbench := eventNames(t, filepath.Join(benchOf(stream), bench.JournalName))
	// Six rather than five: `workstream new` already wrote the created line
	// onto this journal before any of the five acts above.
	if len(written) != 6 {
		t.Errorf("the workstream's journal carries %d lines, wanted six: %v", len(written), written)
	}
	for _, want := range []string{
		contract.EventCreated,
		contract.EventAttached,
		contract.EventAttachmentRenamed,
		contract.EventAttachmentReplaced,
		contract.EventArchived,
		contract.EventRestored,
	} {
		if !containsEvent(written, want) {
			t.Errorf("the workstream's journal carries no %s line: %v", want, written)
		}
	}
	for _, unwanted := range []string{
		contract.EventAttached,
		contract.EventAttachmentRenamed,
		contract.EventAttachmentReplaced,
		contract.EventArchived,
		contract.EventRestored,
	} {
		if containsEvent(workbench, unwanted) {
			t.Errorf("the workbench's journal carries a %s line about a workstream's attachment: %v", unwanted, workbench)
		}
	}
	if len(workbench) != 0 {
		t.Errorf("the workbench's journal carries %d lines, and none of the five acts belongs there: %v", len(workbench), workbench)
	}

	// The control. An attachment on the workbench itself does journal here,
	// so an empty answer above is the five lines going elsewhere rather than
	// nothing being written anywhere.
	mustRunCLI(t, root, "attach", "workbench", source)
	control := eventNames(t, filepath.Join(benchOf(stream), bench.JournalName))
	if len(control) != 1 || control[0] != contract.EventAttached {
		t.Errorf("attaching to the workbench wrote %v to its journal, wanted one %s line", control, contract.EventAttached)
	}
	if again := eventNames(t, filepath.Join(stream, bench.JournalName)); len(again) != 6 {
		t.Errorf("the workstream's journal moved to %d lines when the workbench was written to: %v", len(again), again)
	}
}

// eventNames reads one journal through restore_test.go's own journalEvents and
// answers the event names in order, which is all this file compares.
//
// A journal nothing has written to does not exist yet, and that is the answer
// rather than a failure: the caller's own count is what decides whether an
// empty journal is right there. The control in the test above is what stops
// that leniency from hiding a build that journals nowhere.
func eventNames(t *testing.T, path string) []string {
	t.Helper()
	names := []string{}
	if !bench.Exists(path) {
		return names
	}
	for _, event := range journalEvents(t, path) {
		name, _ := event["event"].(string)
		names = append(names, name)
	}
	return names
}

// containsEvent reports whether a journal's event names carry one.
func containsEvent(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

// TestRenamingAWorkstreamSlugLeavesItsAttachmentsWhereTheyAre asserts the
// position that breaks: the attachment lives under the workstream's
// identifier-named directory, so a slug rename moves nothing on disk and the
// new spelling reaches the same bytes while the old one stops resolving.
//
// The directory is asserted unchanged as well as the bytes, so an
// implementation that helpfully moved the collection to follow the slug would
// fail here rather than passing on the reads alone.
//
// Arming: moving the attachments directory on a slug write reddens the
// unchanged-path assertion while the byte comparison stays green.
func TestRenamingAWorkstreamSlugLeavesItsAttachmentsWhereTheyAre(t *testing.T) {
	root, _, stream, source := workstreamAttachmentFixture(t, "portfolio")
	mustRunCLI(t, root, "attach", "workstream/portfolio", source)
	before := runCLI(t, root, "path", "workstream/portfolio/attachments/1/payload")
	beforeBytes := readFileAt(t, strings.TrimSpace(before.out))

	mustRunCLI(t, root, "set", "workstream/portfolio", "slug", "renamed", "--yes")

	after := runCLI(t, root, "path", "workstream/renamed/attachments/1/payload")
	if strings.TrimSpace(after.out) != strings.TrimSpace(before.out) {
		t.Errorf("the payload moved from %s to %s, and the collection hangs off the identifier rather than the slug",
			strings.TrimSpace(before.out), strings.TrimSpace(after.out))
	}
	if got := readFileAt(t, strings.TrimSpace(after.out)); got != beforeBytes {
		t.Errorf("the payload reads %q after the rename, wanted %q", got, beforeBytes)
	}
	if !strings.HasPrefix(strings.TrimSpace(after.out), stream) {
		t.Errorf("the payload sits at %s, wanted a path under the workstream's own directory %s", after.out, stream)
	}
	stale := runCLI(t, root, "path", "workstream/portfolio/attachments/1")
	if stale.code == 0 {
		t.Errorf("the old slug still resolves:\n%s", stale.out)
	}
	if !strings.HasPrefix(stale.errw, contract.UnknownWorkstream) {
		t.Errorf("the old slug refuses %q, wanted %s", stale.errw, contract.UnknownWorkstream)
	}
}

// readFileAt reads one file and answers its bytes as a string.
func readFileAt(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(text)
}

// TestDeletingAWorkstreamTakesItsAttachmentsWithIt asserts both halves of the
// deletion rule: a workstream no live card references goes, and its
// attachments go with the directory, with the deleted line on the workbench's
// journal because the act destroys the journal inside what it removes; a
// workstream a live card still references is refused as before, and its
// attachments survive the refusal.
//
// Arming: making delete refuse over a workstream carrying attachments reddens
// the first half; making it succeed over a referenced one reddens the second.
func TestDeletingAWorkstreamTakesItsAttachmentsWithIt(t *testing.T) {
	root, card, stream, source := workstreamAttachmentFixture(t, "portfolio")
	mustRunCLI(t, root, "attach", "workstream/portfolio", source)
	collection := filepath.Join(stream, bench.AttachmentsDir)
	if !bench.Exists(collection) {
		t.Fatalf("the fixture wrote no collection at %s", collection)
	}

	referenced := runCLI(t, root, "delete", "workstream/portfolio", "--yes")
	if referenced.code == 0 {
		t.Errorf("deleting a workstream a live card still references succeeded:\n%s", referenced.out)
	}
	if !bench.Exists(collection) {
		t.Errorf("the refused delete removed %s anyway", collection)
	}

	mustRunCLI(t, root, "leave", card, "workstream/portfolio")
	mustRunCLI(t, root, "delete", "workstream/portfolio", "--yes")
	if bench.Exists(collection) {
		t.Errorf("%s survived the delete, and a workstream's attachments go with its directory", collection)
	}
	if bench.Exists(stream) {
		t.Errorf("the workstream's directory survived the delete")
	}
	if !containsEvent(eventNames(t, filepath.Join(benchOf(stream), bench.JournalName)), contract.EventDeleted) {
		t.Error("the workbench's journal carries no deleted line, and the act destroys the journal inside what it removes")
	}
}

// TestArchivingReachesAWorkstreamsAttachmentsAtBothGranularities asserts the
// two archive granularities and, for each, the address that answers.
//
// The address is the half a reader guesses wrong. An archived workstream's
// attachments are read by the plain reference with no flag, because
// WorkstreamByRef scans both workstream roots and the collection step then
// reads the live mirror of a directory that is itself under the archive. The
// same reference under --archived reads that workstream's own archive half,
// which holds only what was individually archived before the workstream was,
// and here that is nothing.
//
// Arming: resolving the workstream head in the caller's half instead of
// through headHalf reddens the no-flag read of an archived workstream's
// attachments.
func TestArchivingReachesAWorkstreamsAttachmentsAtBothGranularities(t *testing.T) {
	root, card, stream, source := workstreamAttachmentFixture(t, "portfolio")
	mustRunCLI(t, root, "attach", "workstream/portfolio", source)
	mustRunCLI(t, root, "leave", card, "workstream/portfolio")

	// Granularity one: the whole workstream.
	mustRunCLI(t, root, "archive", "workstream/portfolio")
	moved := filepath.Join(benchOf(stream), bench.ArchiveDir, bench.WorkstreamsDir, filepath.Base(stream), bench.AttachmentsDir)
	if !bench.Exists(moved) {
		t.Errorf("the attachments did not travel to %s with the workstream's directory", moved)
	}
	plain := runCLI(t, root, "list", "workstream/portfolio/attachments")
	if plain.code != 0 {
		t.Errorf("the plain address does not read an archived workstream's attachments: %d %s", plain.code, plain.errw)
	} else if !carriesOneAttachment(plain.out) {
		t.Errorf("the plain address does not reach an archived workstream's attachments:\n%s", plain.out)
	}
	flagged := runCLI(t, root, "list", "workstream/portfolio/attachments", "--archived")
	if flagged.code != 0 {
		t.Errorf("the flagged address refused: %d %s", flagged.code, flagged.errw)
	} else if carriesOneAttachment(flagged.out) {
		t.Errorf("--archived read the workstream's live collection; it names the workstream's own archive half:\n%s", flagged.out)
	}

	mustRunCLI(t, root, "restore", "workstream/portfolio", "--archived")
	back := runCLI(t, root, "list", "workstream/portfolio/attachments")
	if back.code != 0 || !carriesOneAttachment(back.out) {
		t.Errorf("the restored workstream's attachments do not read live: %d %s\n%s", back.code, back.errw, back.out)
	}

	// Granularity two: the attachment alone.
	mustRunCLI(t, root, "archive", "workstream/portfolio/attachments/1")
	mirror := filepath.Join(stream, bench.ArchiveDir, bench.AttachmentsDir)
	if !bench.Exists(mirror) {
		t.Errorf("archiving the attachment alone did not put it under %s", mirror)
	}
	live := runCLI(t, root, "list", "workstream/portfolio/attachments")
	if carriesOneAttachment(live.out) {
		t.Errorf("the live collection still carries the archived attachment:\n%s", live.out)
	}
	inMirror := runCLI(t, root, "list", "workstream/portfolio/attachments", "--archived")
	if inMirror.code != 0 || !carriesOneAttachment(inMirror.out) {
		t.Errorf("--archived does not read the individually archived attachment: %d %s\n%s", inMirror.code, inMirror.errw, inMirror.out)
	}
	mustRunCLI(t, root, "restore", "workstream/portfolio/attachments/1", "--archived")
	restored := runCLI(t, root, "list", "workstream/portfolio/attachments")
	if !carriesOneAttachment(restored.out) {
		t.Errorf("the restored attachment does not read live:\n%s", restored.out)
	}
}

// carriesOneAttachment reports whether a collection listing found exactly one
// member. The live listing and the archived one head their tables with
// different sentences, so the row is read rather than the heading.
func carriesOneAttachment(listing string) bool {
	return strings.Count(listing, "workstream/portfolio/attachments/1") == 1
}

// TestAnyOwnerMayAttachToAWorkstreamAndTheColumnRuleDoesNotReachIt pins the
// accepting case beside the refusing one, because a criterion asserting that
// something is refused passes against code that refuses everything.
//
// The operator-only rule that guards a definition's attachments covers the
// workbench's own and a column's, and it does not reach a workstream. So a
// non-operator owner attaches to a workstream, renames that attachment and
// deletes it, while the same actor attaching to a column is still refused
// not-operator, which is what shows the rule alive rather than absent.
//
// The refusing row is a column attach rather than a field write on the
// workstream. Since dinah-582 a workstream's field write is any owner's, so a
// title written through `set workstream/<slug>` is accepted for this actor and
// would assert nothing. The refusal's code is read rather than the exit status
// alone, so a refusal for some other reason cannot stand in for the authority
// one.
//
// Arming: the rule has two arms and each one is pinned by a different row, so
// breaking either reddens its own rows and no others. I broke both and wrote
// down what the run printed.
//
// Adding bench.KindWorkstream to definitionAttachmentWrite's kind arm reddens
// the attach row alone, refusing not-operator. That row is a t.Fatalf, so the
// run stops there and the rename and delete rows are never reached; demoting
// it only makes those two fail dinah.unknown-path for the attachment that was
// never created, which is not the authority they are there to pin.
//
// Making definitionAttachmentWrite answer true for an attachment whose holder
// is a workstream reddens the rename and delete rows, both refusing
// not-operator, and leaves the attach row green, because attach is given the
// workstream and those two are given the attachment.
//
// The refusing column row stays green under both breaks, which is what shows
// it pinning the rule rather than the arms.
func TestAnyOwnerMayAttachToAWorkstreamAndTheColumnRuleDoesNotReachIt(t *testing.T) {
	root, _, _, source := workstreamAttachmentFixture(t, "portfolio")

	attached := runCLI(t, root, "attach", "workstream/portfolio", source, "--actor", "bo")
	if attached.code != 0 {
		t.Fatalf("a non-operator owner may not attach to a workstream: %d %s", attached.code, attached.errw)
	}
	if renamed := runCLI(t, root, "rename", "workstream/portfolio/attachments/1", "renamed.txt", "--actor", "bo"); renamed.code != 0 {
		t.Errorf("a non-operator owner may not rename a workstream's attachment: %d %s", renamed.code, renamed.errw)
	}
	if removed := runCLI(t, root, "delete", "workstream/portfolio/attachments/1", "--yes", "--actor", "bo"); removed.code != 0 {
		t.Errorf("a non-operator owner may not delete a workstream's attachment: %d %s", removed.code, removed.errw)
	}
	column := runCLI(t, root, "attach", "doing", source, "--actor", "bo")
	if column.code == 0 {
		t.Errorf("a non-operator owner attached to a column, and a column's attachments are the operator's:\n%s", column.out)
	} else if name := refusalNameOf(column.errw); name != contract.NotOperator {
		t.Errorf("the column attach refuses %s, wanted %s:\n%s", name, contract.NotOperator, column.errw)
	}
}

// TestTheAttachHelpPageNamesTheWorkstream asserts that the one declaration in
// internal/verb reaches the published help page, which is the surface a reader
// actually consults before typing the reference.
//
// Arming: removing ReferenceKindWorkstream from attach's row reddens this and
// TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream together.
func TestTheAttachHelpPageNamesTheWorkstream(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "help", "attach")
	if got.code != 0 {
		t.Fatalf("help attach: %d %s", got.code, got.errw)
	}
	folded := strings.Join(strings.Fields(got.out), " ")
	if !strings.Contains(folded, "a workstream, written as `workstream/<slug>`") {
		t.Errorf("attach's help page does not name the workstream in its reference-kinds clause:\n%s", got.out)
	}
}
