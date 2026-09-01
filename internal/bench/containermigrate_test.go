package bench

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// populatedBench writes a workbench carrying one of everything the format
// nests: a card with a journal, a comment under that card, an attachment with
// a payload beside it, an archived card, and the workbench's own journal.
//
// The migration is asserted by what a workbench holds afterwards rather than by
// what exists afterwards, which takes a fixture with something in it. A fixture
// carrying an anchor and nothing else would let a migration that moved the
// anchor and dropped every collection pass.
func populatedBench(t *testing.T, root, definition string) string {
	t.Helper()
	plantBench(t, root, definition)
	write(t, filepath.Join(root, JournalName), "{\"ts\":\"2026-01-01T00:00:00Z\",\"event\":\"created\"}\n")
	card := filepath.Join(root, CardsDir, "c00000000001")
	write(t, filepath.Join(card, CardAnchor), cleanCard)
	write(t, filepath.Join(card, JournalName), "{\"ts\":\"2026-01-01T00:00:01Z\",\"event\":\"created\"}\n")
	write(t, filepath.Join(card, CommentsDir, "e00000000001", CommentAnchor), "---\nauthor: alka\n---\nA remark.\n")
	attachment := filepath.Join(card, AttachmentsDir, "e00000000002")
	write(t, filepath.Join(attachment, AttachmentAnchor), "---\nfilename: notes.txt\n---\n")
	write(t, filepath.Join(attachment, PayloadDir, "notes.txt"), "payload bytes\n")
	archived := filepath.Join(root, ArchiveDir, CardsDir, "c00000000002")
	write(t, filepath.Join(archived, CardAnchor), cleanCard)
	write(t, filepath.Join(archived, JournalName), "{\"ts\":\"2026-01-01T00:00:02Z\",\"event\":\"created\"}\n")
	return root
}

// contents answers every file of a tree keyed on its path relative to that
// tree, with the digest of its bytes, which is what a comparison of two
// workbenches has to be made of. Comparing what exists rather than what a file
// holds is the shape of the migration that destroyed every card's position on
// dinah-287 while its own test passed.
func contents(t *testing.T, root string) map[string]string {
	t.Helper()
	digests, err := treeDigest(root)
	if err != nil {
		t.Fatalf("digest %s: %v", root, err)
	}
	return digests
}

// sameContents fails the test naming the first file that differs, which is
// what a caller needs in order to act on a failure rather than to rerun it
// under a debugger.
func sameContents(t *testing.T, what string, want, got map[string]string) {
	t.Helper()
	names := map[string]bool{}
	for name := range want {
		names[name] = true
	}
	for name := range got {
		names[name] = true
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	for _, name := range ordered {
		switch {
		case want[name] == "":
			t.Errorf("%s: %s appeared and nothing wrote it", what, name)
		case got[name] == "":
			t.Errorf("%s: %s is gone", what, name)
		case want[name] != got[name]:
			t.Errorf("%s: %s holds different bytes now", what, name)
		}
	}
}

// treeShape answers every path beneath a directory, relative to it and sorted,
// so two runs of a migration can be compared as trees rather than as counts.
func treeShape(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(found)
	return found
}

// migrateTree carries every workbench beneath a root into a container, the way
// the command does, and answers where each one went.
func migrateTree(t *testing.T, root string) map[string]string {
	t.Helper()
	candidates, err := ScanContainers(root)
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
	moved := map[string]string{}
	for _, candidate := range candidates {
		to, err := MigrateContainer(candidate.Path)
		if err != nil {
			t.Fatalf("migrate %s: %v", candidate.Path, err)
		}
		moved[candidate.Path] = to
	}
	return moved
}

// TestTheContainerMigrationIsIdempotentAndKeepsEveryFile asserts dinah-285
// AC-8. A tree holding one of each shape is migrated twice, and the tree after
// the second run is the tree after the first one, path for path. Every file of
// every workbench is compared by the digest of its bytes against what the
// workbench held before the migration, so a value the migration destroyed
// cannot read back as a reader's default the way dinah-287's did.
func TestTheContainerMigrationIsIdempotentAndKeepsEveryFile(t *testing.T) {
	tree := t.TempDir()
	legacy := populatedBench(t, filepath.Join(tree, "a", UserBaseName, "d00000000001"), benchDefinition)
	bare := populatedBench(t, filepath.Join(tree, "b"), currentBenchDefinition)
	stray := populatedBench(t, filepath.Join(tree, "c", UserBaseName, "notes"), benchDefinition)

	before := map[string]map[string]string{
		"legacy": contents(t, legacy),
		"bare":   contents(t, bare),
		"stray":  contents(t, stray),
	}

	first := migrateTree(t, tree)
	if len(first) != 3 {
		t.Fatalf("the first run moved %d workbenches, wanted three: %v", len(first), first)
	}
	for name, source := range map[string]string{"legacy": legacy, "bare": bare, "stray": stray} {
		to := first[source]
		if !Contained(to) {
			t.Errorf("%s landed at %s, which is not where a workbench lives", name, to)
		}
		after := contents(t, to)
		// The anchor is the one file the migration rewrites, since it stamps
		// the storage format the rule arrived at, so it is compared on its
		// content rather than on its digest.
		delete(after, WorkbenchAnchor)
		want := before[name]
		delete(want, WorkbenchAnchor)
		sameContents(t, name, want, after)
		opened, err := Open(to)
		if err != nil {
			t.Errorf("%s does not open after the migration: %v", name, err)
			continue
		}
		if opened.Format != ContainerFormat {
			t.Errorf("%s declares format %d after the migration, wanted %d", name, opened.Format, ContainerFormat)
		}
	}

	shape := treeShape(t, tree)
	second := migrateTree(t, tree)
	for source, to := range second {
		if source != to {
			t.Errorf("the second run moved %s to %s, and the first run had already put it where the rule wants it", source, to)
		}
	}
	if got := treeShape(t, tree); strings.Join(got, "\n") != strings.Join(shape, "\n") {
		t.Errorf("the tree after the second run differs from the tree after the first:\nwanted %v\ngot    %v", shape, got)
	}
}

// TestAReminItInPlaceTouchesNothingInsideTheWorkbench asserts the property the
// whole identifier design rests on: a workbench's identifier is its directory
// name and is repeated nowhere inside it, so reminting is a rename and every
// file beneath it keeps its bytes.
func TestAReminItInPlaceTouchesNothingInsideTheWorkbench(t *testing.T) {
	root := populatedBench(t, filepath.Join(t.TempDir(), UserBaseName, "d00000000001"), currentBenchDefinition)
	before := contents(t, root)
	moved, err := Remint(root)
	if err != nil {
		t.Fatalf("remint: %v", err)
	}
	if moved == root {
		t.Fatal("the remint left the workbench under the name it already had")
	}
	if !Contained(moved) {
		t.Errorf("the reminted workbench sits at %s, which is not where a workbench lives", moved)
	}
	if Exists(root) {
		t.Errorf("the old directory %s is still there, so the remint copied rather than renamed", root)
	}
	sameContents(t, "reminted", before, contents(t, moved))
}

// TestACrossDeviceLiftRefusesToDeleteASourceItCouldNotVerify asserts dinah-285
// AC-9. The lift is forced onto the copy-then-verify-then-delete path, a file
// is corrupted after the copy and before the verification, and the migration
// refuses: it deletes nothing and leaves both trees on disk, because a source
// it could not show had arrived is a source it must not remove.
func TestACrossDeviceLiftRefusesToDeleteASourceItCouldNotVerify(t *testing.T) {
	root := populatedBench(t, filepath.Join(t.TempDir(), "workbench"), currentBenchDefinition)
	before := contents(t, root)
	forceCrossDevice(t)
	corrupted := ""
	afterContainerCopy = func(source, target string) {
		corrupted = filepath.Join(target, CardsDir, "c00000000001", CardAnchor)
		write(t, corrupted, "---\ntitle: not what was copied\n---\n")
	}
	t.Cleanup(func() { afterContainerCopy = nil })

	_, err := MigrateContainer(root)
	refusal, refused := err.(*contract.Refusal)
	if !refused {
		t.Fatalf("a copy that did not arrive should refuse, got %v", err)
	}
	if refusal.Name != contract.Unconfirmed {
		t.Errorf("refusal name: wanted %s, got %s", contract.Unconfirmed, refusal.Name)
	}
	if !Exists(root) {
		t.Fatal("the migration deleted a source it could not verify")
	}
	sameContents(t, "the source", before, contents(t, root))
	if !Exists(corrupted) {
		t.Error("the copy was removed, and leaving both trees is what lets an operator see what happened")
	}
}

// TestAnInterruptedLiftIsFinishedByTheNextRun asserts dinah-285 AC-10. A crash
// is planted between the copy and the delete of the original, which leaves both
// trees on disk; re-running the migration deletes the now-redundant original
// and copies nothing, and the workbench that survives holds the files it held
// before, byte for byte.
func TestAnInterruptedLiftIsFinishedByTheNextRun(t *testing.T) {
	root := populatedBench(t, filepath.Join(t.TempDir(), "workbench"), currentBenchDefinition)
	before := contents(t, root)
	forceCrossDevice(t)
	crash := errors.New("the process died between the copy and the delete")
	afterContainerVerify = func(source, target string) error { return crash }
	t.Cleanup(func() { afterContainerVerify = nil })

	if _, err := MigrateContainer(root); !errors.Is(err, crash) {
		t.Fatalf("the planted crash did not reach the caller, got %v", err)
	}
	if !Exists(root) {
		t.Fatal("the interrupted run deleted the original, and the crash was planted before the delete")
	}
	container := filepath.Join(filepath.Dir(root), UserBaseName)
	copied := ListWorkbenchIDs(container)
	if len(copied) != 1 {
		t.Fatalf("the interrupted run left %v in the container, wanted the one copy it made", copied)
	}
	landed := filepath.Join(container, copied[0])

	afterContainerVerify = nil
	moved, err := MigrateContainer(root)
	if err != nil {
		t.Fatalf("the second run refused: %v", err)
	}
	if moved != landed {
		t.Errorf("the second run answered %s, wanted the copy the first run made at %s", moved, landed)
	}
	if Exists(root) {
		t.Error("the second run left the original in place, so the migration never finished")
	}
	if got := ListWorkbenchIDs(container); len(got) != 1 {
		t.Errorf("the container holds %v, so the second run copied again rather than finishing the first", got)
	}
	after := contents(t, moved)
	delete(after, WorkbenchAnchor)
	want := before
	delete(want, WorkbenchAnchor)
	sameContents(t, "the finished workbench", want, after)
}

// forceCrossDevice makes every rename this package performs report the errno a
// rename across two filesystems reports, which is how the copy-then-verify
// fallback is reached without arranging two filesystems on the machine running
// the suite.
func forceCrossDevice(t *testing.T) {
	t.Helper()
	previous := containerRename
	containerRename = func(from, to string) error {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: errCrossDevice}
	}
	t.Cleanup(func() { containerRename = previous })
}

// TestADuplicateIdentifierIsReportedAndNeverRepaired asserts dinah-285 AC-11.
// Two directories carrying one identifier are reported together, the sweep
// alters neither of them, and the explicit remint renames exactly one and
// leaves the other and its contents untouched.
func TestADuplicateIdentifierIsReportedAndNeverRepaired(t *testing.T) {
	tree := t.TempDir()
	first := populatedBench(t, filepath.Join(tree, "one", UserBaseName, fixtureWorkbenchID), currentBenchDefinition)
	second := populatedBench(t, filepath.Join(tree, "two", UserBaseName, fixtureWorkbenchID), currentBenchDefinition)
	untouched := contents(t, second)

	candidates, err := ScanContainers(tree)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	duplicates := DuplicateWorkbenchIDs(candidates)
	paths, reported := duplicates[fixtureWorkbenchID]
	if !reported {
		t.Fatalf("the shared identifier was not reported: %v", duplicates)
	}
	sort.Strings(paths)
	want := []string{first, second}
	sort.Strings(want)
	if strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Errorf("the report names %v, wanted both %v", paths, want)
	}

	// The sweep leaves both alone, which is what MigrateContainer does with a
	// workbench already sitting where the rule puts one: neither name changes,
	// so neither copy is chosen over the other.
	for _, where := range []string{first, second} {
		moved, err := MigrateContainer(where)
		if err != nil {
			t.Fatalf("migrate %s: %v", where, err)
		}
		if moved != where {
			t.Errorf("the sweep moved %s to %s, and choosing between two copies is not its to make", where, moved)
		}
	}

	moved, err := Remint(first)
	if err != nil {
		t.Fatalf("remint: %v", err)
	}
	if moved == first {
		t.Error("the remint left the identifier it was asked to replace")
	}
	if filepath.Base(moved) == fixtureWorkbenchID {
		t.Error("the reminted workbench still carries the shared identifier")
	}
	if !Exists(second) {
		t.Fatal("the remint touched the other copy")
	}
	sameContents(t, "the other copy", untouched, contents(t, second))
}

// TestAMigrationDeclinesAWorkbenchSomebodyIsHolding asserts what happens when a
// workbench is open elsewhere, which is the third question this migration owes
// an answer to beside interruption and a target that already exists. A lock
// standing anywhere in the tree means a writer is inside a directory the
// migration is about to move, so the migration refuses and changes nothing.
func TestAMigrationDeclinesAWorkbenchSomebodyIsHolding(t *testing.T) {
	root := populatedBench(t, filepath.Join(t.TempDir(), UserBaseName, "d00000000001"), benchDefinition)
	before := contents(t, root)
	write(t, filepath.Join(root, CardsDir, "c00000000001", LockName), "{\"holder\":\"alka\"}\n")

	_, err := MigrateContainer(root)
	refusal, refused := err.(*contract.Refusal)
	if !refused {
		t.Fatalf("a held workbench should be refused, got %v", err)
	}
	if refusal.Name != contract.Locked {
		t.Errorf("refusal name: wanted %s, got %s", contract.Locked, refusal.Name)
	}
	if !Exists(root) {
		t.Fatal("the refused migration moved the workbench anyway")
	}
	sameContents(t, "the held workbench", before, contents(t, root))
}

// TestAFreshWorkbenchCarriesTheUnionMergeAttributes asserts the file half of
// dinah-285 AC-12: Instantiate writes a .gitattributes naming the union merge
// driver for the workbench's own journal and for every card's. The git half,
// that two branches appending to one journal merge without a conflict, is
// asserted in cmd/dinah where a repository can be built around it.
func TestAFreshWorkbenchCarriesTheUnionMergeAttributes(t *testing.T) {
	root := containedPath(t.TempDir())
	definition, err := ReadDefinition([]byte(rootDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := Instantiate(root, "fx", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	text, err := ReadText(filepath.Join(root, AttributesName))
	if err != nil {
		t.Fatalf("read the attributes: %v", err)
	}
	for _, want := range []string{JournalName + " merge=union", "*/" + JournalName + " merge=union"} {
		if !strings.Contains(text, want) {
			t.Errorf("the attributes file carries no %q:\n%s", want, text)
		}
	}
}

// TestTheStorageFormatMovedAndTheProfileDidNot asserts dinah-285 AC-14 in this
// package: the constant reads 2, and a workbench declaring the older format
// beside a profile revision inside the window still opens in the bare layout it
// was written in. The other half of that criterion, that docs/spec/core-profile.md
// carries no hunk on this card, is a property of the diff rather than of the
// build and is asserted in cmd/dinah.
func TestTheStorageFormatMovedAndTheProfileDidNot(t *testing.T) {
	if StorageFormat != 2 {
		t.Errorf("StorageFormat is %d, wanted 2", StorageFormat)
	}
	older := strings.Replace(benchDefinition, "profile: dinah-core/0.7", "profile: dinah-core/0.9", 1)
	root := plantBench(t, filepath.Join(t.TempDir(), "workbench"), older)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("a format 1 workbench declaring dinah-core/0.9 should still open bare, got %v", err)
	}
	if opened.Format != 1 {
		t.Errorf("the workbench reports format %d, wanted the 1 it declares", opened.Format)
	}
	if opened.Profile != "dinah-core/0.9" {
		t.Errorf("the workbench reports the profile %q, wanted the one it declares", opened.Profile)
	}
}
