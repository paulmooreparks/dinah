package bench

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

// contents answers every file a workbench owns, keyed on its path relative to
// the directory the workbench sits in, with the digest of its bytes, which is
// what a comparison of two workbenches has to be made of. Comparing what exists
// rather than what a file holds is the shape of the migration that destroyed
// every card's position on dinah-287 while its own test passed.
//
// It answers for the workbench and says nothing about the ground the workbench
// stands on, which is the blindness aroundTheWorkbench exists to cover.
func contents(t *testing.T, root string) map[string]string {
	t.Helper()
	digests, err := memberDigest(root)
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

// benchMemberNames is every top-level name a workbench owns inside the
// directory it sits in, which is the set the lift is entitled to move. The
// test spells it out rather than importing the production list, so that a
// production list which quietly loses a member fails here instead of agreeing
// with itself.
var benchMemberNames = []string{
	WorkbenchAnchor,
	JournalName,
	ColumnsDir,
	PreVocabularyDir,
	CardsDir,
	WorkstreamsDir,
	AttachmentsDir,
	ArchiveDir,
}

// aroundTheWorkbench answers the digest of every file in a directory that the
// workbench sitting there does not own, keyed on its path relative to that
// directory. The container the migration creates is left out, because it is
// the one thing the migration is entitled to add.
//
// This is the question a content proof over the workbench cannot ask. A digest
// keyed on paths inside the workbench matches after a migration that moved the
// whole directory holding it, because no byte of the workbench changed; what
// changed was everything around it. So the fixture puts something around it and
// this reads that ground back.
func aroundTheWorkbench(t *testing.T, dir string) map[string]string {
	t.Helper()
	owned := map[string]bool{UserBaseName: true}
	for _, member := range benchMemberNames {
		owned[member] = true
	}
	ground := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		slashed := filepath.ToSlash(relative)
		if slashed == "." {
			return nil
		}
		first := strings.SplitN(slashed, "/", 2)[0]
		if owned[first] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			ground[slashed] = "a directory"
			return nil
		}
		revision, err := Revision(path)
		if err != nil {
			return err
		}
		ground[slashed] = revision
		return nil
	})
	if err != nil {
		t.Fatalf("read the ground around %s: %v", dir, err)
	}
	return ground
}

// namesIn answers the sorted names of a directory's own entries, which is how
// a test asks whether a migration created anything beside the directory it was
// pointed at.
func namesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
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
	candidates, _, err := ScanContainers(root)
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
	// The bare workbench sits in a project repository, which is the
	// arrangement the format explicitly allows and the one the lift has to
	// leave standing. The source file, the readme and the git directory are
	// the ground the migration has no business touching, and they exist here
	// because a fixture whose bare workbench sits alone in a directory of its
	// own gives a migration that moves that directory nothing to destroy.
	bare := populatedBench(t, filepath.Join(tree, "b", "myproject"), currentBenchDefinition)
	stray := populatedBench(t, filepath.Join(tree, "c", UserBaseName, "notes"), benchDefinition)

	before := map[string]map[string]string{
		"legacy": contents(t, legacy),
		"bare":   contents(t, bare),
		"stray":  contents(t, stray),
	}

	write(t, filepath.Join(bare, "README.md"), "A project that happens to hold a workbench.\n")
	write(t, filepath.Join(bare, "src", "main.go"), "package main\n\nfunc main() {}\n")
	write(t, filepath.Join(bare, ".git", "HEAD"), "ref: refs/heads/main\n")
	ground := aroundTheWorkbench(t, bare)

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

	// What the workbench stood on. A lift moves the workbench's own members
	// and creates the container inside the directory holding them, so that
	// directory still exists, everything else in it is untouched, and nothing
	// at all appeared beside it.
	if !Exists(bare) {
		t.Fatalf("the directory holding the bare workbench, %s, stopped existing", bare)
	}
	sameContents(t, "the ground around the bare workbench", ground, aroundTheWorkbench(t, bare))
	if got := strings.Join(namesIn(t, filepath.Join(tree, "b")), " "); got != "myproject" {
		t.Errorf("the lift created something beside the project directory: %s holds %q, wanted only myproject", filepath.Join(tree, "b"), got)
	}
	if landed := first[bare]; filepath.Dir(filepath.Dir(landed)) != bare {
		t.Errorf("the bare workbench landed at %s, and the container belongs inside %s", landed, bare)
	}
	if Exists(filepath.Join(bare, WorkbenchAnchor)) {
		t.Errorf("%s still carries a workbench.md, so the anchor never moved into the container", bare)
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
	if !Exists(filepath.Join(root, WorkbenchAnchor)) {
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
	if !Exists(filepath.Join(root, WorkbenchAnchor)) {
		t.Fatal("the interrupted run deleted the original, and the crash was planted before the delete")
	}
	container := filepath.Join(root, UserBaseName)
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
	if Exists(filepath.Join(root, WorkbenchAnchor)) {
		t.Error("the second run left the original in place, so the migration never finished")
	}
	if !Exists(root) {
		t.Errorf("the second run removed %s, and a lift moves the workbench's members rather than the directory holding them", root)
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

	candidates, _, err := ScanContainers(tree)
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

// TestALiftStoppedPartWayFinishesIntoTheSameDirectory asserts the crash story
// of the ordinary path, where the members move one rename at a time and no
// filesystem offers to move eight names as one act. A stop is planted on the
// anchor, which is the member the lift moves last: the workbench is still
// recognised where it stood, the container carries a directory holding the
// members that did move and no anchor of its own, and the next run carries on
// into that same directory rather than minting a second one and stranding the
// first.
func TestALiftStoppedPartWayFinishesIntoTheSameDirectory(t *testing.T) {
	root := populatedBench(t, filepath.Join(t.TempDir(), "myproject"), currentBenchDefinition)
	write(t, filepath.Join(root, "README.md"), "the project's own file\n")
	before := contents(t, root)

	stop := errors.New("the process died before the anchor moved")
	previous := containerRename
	containerRename = func(from, to string) error {
		if filepath.Base(from) == WorkbenchAnchor {
			return stop
		}
		return previous(from, to)
	}
	t.Cleanup(func() { containerRename = previous })

	if _, err := MigrateContainer(root); !errors.Is(err, stop) {
		t.Fatalf("the planted stop did not reach the caller, got %v", err)
	}
	if !Exists(filepath.Join(root, WorkbenchAnchor)) {
		t.Fatal("the anchor moved before the stop, so a sweep would no longer find the workbench where it left it")
	}
	container := filepath.Join(root, UserBaseName)
	partial := ListWorkbenchIDs(container)
	if len(partial) != 1 {
		t.Fatalf("the stopped run left %v in the container, wanted the one directory it was filling", partial)
	}
	landed := filepath.Join(container, partial[0])

	containerRename = previous
	moved, err := MigrateContainer(root)
	if err != nil {
		t.Fatalf("the second run refused: %v", err)
	}
	if moved != landed {
		t.Errorf("the second run answered %s, wanted the directory the first run was already filling at %s", moved, landed)
	}
	if got := ListWorkbenchIDs(container); len(got) != 1 {
		t.Errorf("the container holds %v, so the second run minted a second directory and stranded what the first one moved", got)
	}
	after := contents(t, moved)
	delete(after, WorkbenchAnchor)
	want := before
	delete(want, WorkbenchAnchor)
	sameContents(t, "the finished workbench", want, after)
	if _, err := Revision(filepath.Join(root, "README.md")); err != nil {
		t.Errorf("the project's own file did not survive the lift: %v", err)
	}
}

// TestAMigrationThatMovedAndThenFailedAnswersWithWhereItWent asserts what a
// half-finished migration tells its caller. The move and the format stamp are
// two writes, so a workbench declaring a format this build cannot open moves
// and then fails to be stamped. The answer carries the directory it moved to,
// because a failure reporting only where the workbench used to be sends a
// reader looking for it there.
func TestAMigrationThatMovedAndThenFailedAnswersWithWhereItWent(t *testing.T) {
	unsupported := strings.Replace(benchDefinition, "format: 1", "format: 99", 1)
	root := populatedBench(t, filepath.Join(t.TempDir(), "myproject"), unsupported)

	moved, err := MigrateContainer(root)
	if err == nil {
		t.Fatal("a workbench declaring a format this build cannot open should not stamp cleanly")
	}
	if moved == "" {
		t.Fatal("the failure named no destination, and the workbench is not where it was")
	}
	if !Exists(filepath.Join(moved, WorkbenchAnchor)) {
		t.Errorf("the failure names %s and no workbench is there", moved)
	}
	if Exists(filepath.Join(root, WorkbenchAnchor)) {
		t.Errorf("%s still carries the anchor, so nothing moved and the destination is a fiction", root)
	}
}

// TestTwoCopiesOfALegacyWorkbenchShareOneIdentifier asserts that the older
// width counts as an identity claim. Two clones of a repository carrying a
// workbench that has not been carried forward yet are two directories claiming
// one identifier, and a sweep blind to that would mint a fresh identifier for
// each of them without saying anything, which is the choice this report exists
// to refuse. A name a person typed claims nothing, so two of those are not a
// duplicate however alike they look.
func TestTwoCopiesOfALegacyWorkbenchShareOneIdentifier(t *testing.T) {
	tree := t.TempDir()
	first := populatedBench(t, filepath.Join(tree, "one", UserBaseName, "d00000000001"), benchDefinition)
	second := populatedBench(t, filepath.Join(tree, "two", UserBaseName, "d00000000001"), benchDefinition)
	populatedBench(t, filepath.Join(tree, "three", UserBaseName, "notes"), benchDefinition)
	populatedBench(t, filepath.Join(tree, "four", UserBaseName, "notes"), benchDefinition)

	candidates, _, err := ScanContainers(tree)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	duplicates := DuplicateWorkbenchIDs(candidates)
	paths, reported := duplicates["d00000000001"]
	if !reported {
		t.Fatalf("the shared legacy identifier was not reported: %v", duplicates)
	}
	sort.Strings(paths)
	want := []string{first, second}
	sort.Strings(want)
	if strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Errorf("the report names %v, wanted both %v", paths, want)
	}
	if named, reported := duplicates["notes"]; reported {
		t.Errorf("two directories under a name a person typed were reported as claiming one identifier: %v", named)
	}
}

// TestALiftStoppedBeforeTheStampIsFinishedByTheNextRun asserts the interruption
// that sits after every member has moved and before the format is stamped. The
// two are separate writes, so a process can die between them, and what it
// leaves is a workbench sitting exactly where the containment rule puts one
// while it still declares the format that predates the rule. That workbench
// opens, is found and works, which is what makes the state hide: a sweep
// classifies it as already contained, and one that returned without writing
// would walk past it on that run and on every run after it.
//
// The stop is planted on the anchor's rename, which is the member the lift
// moves last, and it is taken after the rename has been performed rather than
// instead of it. That is what puts the cut between the last move and the
// stamp rather than before the last move, which is the interruption
// TestALiftStoppedPartWayFinishesIntoTheSameDirectory already covers.
func TestALiftStoppedBeforeTheStampIsFinishedByTheNextRun(t *testing.T) {
	tree := t.TempDir()
	root := populatedBench(t, filepath.Join(tree, "myproject"), benchDefinition)
	write(t, filepath.Join(root, "README.md"), "the project's own file\n")
	before := contents(t, root)

	stop := errors.New("the process died after the last member moved and before the stamp")
	previous := containerRename
	containerRename = func(from, to string) error {
		if err := previous(from, to); err != nil {
			return err
		}
		if filepath.Base(from) == WorkbenchAnchor {
			return stop
		}
		return nil
	}
	t.Cleanup(func() { containerRename = previous })

	if _, err := MigrateContainer(root); !errors.Is(err, stop) {
		t.Fatalf("the planted stop did not reach the caller, got %v", err)
	}
	containerRename = previous

	if Exists(filepath.Join(root, WorkbenchAnchor)) {
		t.Fatal("the anchor is still at the old path, so the stop landed before the last move rather than after it")
	}
	container := filepath.Join(root, UserBaseName)
	ids := ListWorkbenchIDs(container)
	if len(ids) != 1 {
		t.Fatalf("the stopped run left %v in the container, wanted the one directory it filled", ids)
	}
	landed := filepath.Join(container, ids[0])
	stopped, err := Open(landed)
	if err != nil {
		t.Fatalf("the workbench the stopped run left behind will not open: %v", err)
	}
	if stopped.Format != 1 {
		t.Fatalf("the stopped run left format %d, so the cut did not land before the stamp", stopped.Format)
	}

	moved := migrateTree(t, tree)
	if moved[landed] != landed {
		t.Errorf("the sweep answered %q for the interrupted workbench, wanted the directory it already sits in at %s", moved[landed], landed)
	}
	finished, err := Open(landed)
	if err != nil {
		t.Fatalf("open after the sweep: %v", err)
	}
	if finished.Format != ContainerFormat {
		t.Errorf("the workbench declares format %d after the sweep, so the sweep walked past the one interruption it had to finish", finished.Format)
	}
	if got := ListWorkbenchIDs(container); len(got) != 1 {
		t.Errorf("the container holds %v, so the sweep minted a second directory rather than finishing the first", got)
	}
	after := contents(t, landed)
	delete(after, WorkbenchAnchor)
	delete(before, WorkbenchAnchor)
	sameContents(t, "the finished workbench", before, after)
	if _, err := Revision(filepath.Join(root, "README.md")); err != nil {
		t.Errorf("the project's own file did not survive the lift: %v", err)
	}
}

// TestALockRefusalNamesTheEntityThatIsHeld asserts what the migration tells a
// person whose board is in use. The refusal the sweep raises is the message an
// operator running this repair over boards he is working is most likely to
// meet, and naming the workbench tells him nothing he did not already know,
// since he named the workbench himself. The lock's own path is what says which
// card is held and what has to be released, and the check already has it.
func TestALockRefusalNamesTheEntityThatIsHeld(t *testing.T) {
	root := populatedBench(t, filepath.Join(t.TempDir(), UserBaseName, "d00000000001"), benchDefinition)
	card := filepath.Join(root, CardsDir, "c00000000001")
	write(t, filepath.Join(card, LockName), "{\"holder\":\"alka\"}\n")

	_, err := MigrateContainer(root)
	refusal, refused := err.(*contract.Refusal)
	if !refused {
		t.Fatalf("a held workbench should be refused, got %v", err)
	}
	if refusal.Detail != card {
		t.Errorf("the refusal names %q, wanted the card whose lock is held at %s", refusal.Detail, card)
	}

	sibling := filepath.Join(root, CardsDir, "c00000000003"+SiblingSuffix)
	write(t, sibling, "{\"holder\":\"alka\",\"op\":\"archive\"}\n")
	if err := os.RemoveAll(filepath.Join(card, LockName)); err != nil {
		t.Fatalf("clear the card's lock: %v", err)
	}
	_, err = MigrateContainer(root)
	refusal, refused = err.(*contract.Refusal)
	if !refused {
		t.Fatalf("a workbench carrying a sibling lock should be refused, got %v", err)
	}
	if want := filepath.Join(root, CardsDir, "c00000000003"); refusal.Detail != want {
		t.Errorf("the refusal names %q, wanted the entity the sibling lock stands beside at %s", refusal.Detail, want)
	}
}

// TestTheStampDeclinesAContainedWorkbenchSomebodyIsHolding asserts that the one
// repair which moves nothing refuses a held workbench exactly as the two that
// move one do.
//
// The stamp is a read-modify-write of the whole anchor, so a writer holding the
// lock and saving the anchor either loses its edit or swallows the stamp. The
// fixture is the shape a crash between the last move and the stamp leaves,
// which is the only workbench this arm has work to do on: contained under a
// minted name and still declaring the format the rule replaced.
func TestTheStampDeclinesAContainedWorkbenchSomebodyIsHolding(t *testing.T) {
	root := populatedBench(t, containedPath(t.TempDir()), benchDefinition)
	before := contents(t, root)
	card := filepath.Join(root, CardsDir, "c00000000001")
	write(t, filepath.Join(card, LockName), "{\"holder\":\"alka\"}\n")

	_, err := MigrateContainer(root)
	refusal, refused := err.(*contract.Refusal)
	if !refused {
		t.Fatalf("a held workbench should be refused before the stamp, got %v", err)
	}
	if refusal.Name != contract.Locked {
		t.Errorf("refusal name: wanted %s, got %s", contract.Locked, refusal.Name)
	}
	if refusal.Detail != card {
		t.Errorf("the refusal names %q, wanted the card whose lock is held at %s", refusal.Detail, card)
	}
	after := contents(t, root)
	delete(after, filepath.Join(CardsDir, "c00000000001", LockName))
	sameContents(t, "the held workbench", before, after)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open after the refusal: %v", err)
	}
	if opened.Format != 1 {
		t.Errorf("the anchor declares format %d, so the refused stamp was written anyway", opened.Format)
	}
}

// TestAContainedWorkbenchThisBuildCannotOpenIsLeftAlone asserts that the sweep
// answers for a healthy workbench a newer build wrote without reading anything
// about it beyond the format it declares.
//
// A workbench already declaring the format the containment rule arrived at is
// owed no stamp, so the question of whether this build can open it never
// arises. Deciding that question through Open instead would turn every board a
// later release wrote into a migration failure on a machine still running this
// one, and would make a preview, which opens nothing, contradict the run that
// applies.
//
// The lock in the second half is what fixes the order of the two guards. A
// workbench owed no stamp is not refused for a lock it holds, because nothing
// is going to be written to it.
func TestAContainedWorkbenchThisBuildCannotOpenIsLeftAlone(t *testing.T) {
	beyond := ProfileName + "/" + strconv.Itoa(ProfileMajor+99) + ".0"
	definition := strings.Replace(benchDefinition, "format: 1", "format: "+strconv.Itoa(ContainerFormat), 1)
	definition = strings.Replace(definition, "profile: dinah-core/0.7", "profile: "+beyond, 1)
	root := populatedBench(t, containedPath(t.TempDir()), definition)
	_, opening := Open(root)
	refusal, refused := opening.(*contract.Refusal)
	if !refused || refusal.Name != contract.UnsupportedVer {
		t.Fatalf("this build answers %v for the fixture, wanted %s, so the fixture does not stand for a workbench a newer build wrote", opening, contract.UnsupportedVer)
	}
	before := contents(t, root)

	landed, err := MigrateContainer(root)
	if err != nil {
		t.Fatalf("a workbench owed no stamp was reported as a migration failure: %v", err)
	}
	if landed != root {
		t.Errorf("the migration answered %q, wanted the directory the workbench already sits in at %s", landed, root)
	}
	sameContents(t, "the workbench this build cannot open", before, contents(t, root))

	write(t, filepath.Join(root, CardsDir, "c00000000001", LockName), "{\"holder\":\"alka\"}\n")
	if _, err := MigrateContainer(root); err != nil {
		t.Errorf("a workbench owed no stamp was refused for a lock the sweep was never going to write past: %v", err)
	}
}
