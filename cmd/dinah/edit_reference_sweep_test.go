package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The sizes of the swept set, declared here rather than counted from the
// generator's own output or from the resolver under test. A sweep that reads
// nothing reports success and reads exactly like a sweep that found nothing
// wrong, so the three numbers below are what tell a shrinking subject set
// from a clean run. dinah-467.
//
// They are the fixture's own arithmetic, and the fixture holds: an
// attachment on the workbench, one column carrying an attachment, one card
// carrying a comment (which carries an attachment of its own), one checklist
// item of each of the three kinds and an attachment, one live workstream and
// one archived workstream.
//
// Group A walks the containment table from the workbench and emits eight
// collection shapes (columns, cards and attachments under the workbench,
// attachments under the column, comments, checklist and attachments under
// the card, attachments under the comment) and eight member shapes. Two of
// those collections hold a kind addressed in its own right, so they refuse
// unknown-path where the other six refuse is-a-collection.
//
// Group B emits the fourteen shapes the grammar declares outside that table:
// the workbench's two spellings, an attachment reached through the bare
// workbench slug, the card's own two file segments, each of the three
// checklist words as a collection and as a member, an attachment's payload,
// and the workstream's two spellings.
//
// Group C emits the seventeen degenerate references of the spec's own table
// that are not already a Group A or Group B shape.
const (
	wantEditShapes   = 47
	wantEditOpens    = 24
	wantEditRefusals = 23
)

// editShape is one generated reference and the verdict declared for it. The
// verdict is declared by the generator from the containment table and from
// the spec's degenerate table, never read back from the resolver under test,
// so code that answered every reference the same way could not make its own
// answer the expectation.
type editShape struct {
	// ref is the reference as a reader would type it.
	ref string
	// kind is the containment kind the reference names, empty for a shape
	// naming no entity of the format.
	kind string
	// opens says whether edit is required to open a file for this shape.
	opens bool
	// refusal is the refusal name required when opens is false.
	refusal string
}

// editFixture is what the generator needs from the fixture that the
// containment table cannot tell it: the addresses of the two kinds addressed
// in their own right, and the spellings of the entities the grammar reaches
// through a prefix of their own.
type editFixture struct {
	root           string
	benchDir       string
	slug           string
	card           string
	cardDir        string
	column         string
	columnDir      string
	workstream     string
	workstreamID   string
	archived       string
	archivedID     string
	archivedAnchor string
}

// TestEveryReferenceShapeEditAcceptsNamesAFile holds `edit` to answering every
// reference the grammar admits with a regular file or with a named refusal.
// dinah-467.
//
// The defect this replaces a guard for was that `edit` handed a workstream's
// directory to the editor. The guard that should have caught it,
// TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt,
// could not see it three times over: it walks the tree `contents` draws, which
// never carries a workstream; its verdict reads only whether stderr led with a
// resolution refusal; and it points the editor at a name no machine carries, so
// a directory and a file fail the launch identically. This test resolves
// through bench.ResolveEditTarget and stats what comes back, and its sibling
// below watches the argument a real launched process was handed.
func TestEveryReferenceShapeEditAcceptsNamesAFile(t *testing.T) {
	root := newBench(t)
	fixture := buildEditFixture(t, root)
	shapes := editReferenceShapes(t, fixture)

	opened, err := bench.Open(fixture.benchDir)
	if err != nil {
		t.Fatalf("open %s: %v", fixture.benchDir, err)
	}

	opens, refusals := 0, 0
	for _, shape := range shapes {
		answer, err := opened.ResolveEditTarget(shape.ref)
		if shape.opens {
			opens++
			if err != nil {
				t.Errorf("edit %q is required to open a file, and the resolver refused: %v", shape.ref, err)
				continue
			}
			info, statErr := os.Stat(answer)
			if statErr != nil {
				t.Errorf("edit %q answered %q, which nothing stands at: %v", shape.ref, answer, statErr)
				continue
			}
			if !info.Mode().IsRegular() {
				t.Errorf("edit %q answered %q, which is not a regular file", shape.ref, answer)
			}
			continue
		}
		refusals++
		if err == nil {
			t.Errorf("edit %q is required to refuse %s, and it answered %q", shape.ref, shape.refusal, answer)
			continue
		}
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Errorf("edit %q refused with %v, which is not a refusal a reader is given", shape.ref, err)
			continue
		}
		if refusal.Name != shape.refusal {
			t.Errorf("edit %q refused %s, wanted %s", shape.ref, refusal.Name, shape.refusal)
		}
	}

	if len(shapes) != wantEditShapes {
		t.Errorf("the generator emitted %d shapes, wanted %d: the fixture or the containment table has moved, and the counts here have to move with it", len(shapes), wantEditShapes)
	}
	if opens != wantEditOpens {
		t.Errorf("%d shapes were declared to open a file, wanted %d", opens, wantEditOpens)
	}
	if refusals != wantEditRefusals {
		t.Errorf("%d shapes were declared to refuse, wanted %d", refusals, wantEditRefusals)
	}
	if wantEditShapes <= 0 || wantEditOpens <= 0 || wantEditRefusals <= 0 {
		t.Fatalf("one of the declared counts is not positive (%d, %d, %d), so this sweep could prove nothing", wantEditShapes, wantEditOpens, wantEditRefusals)
	}
	if wantEditOpens+wantEditRefusals != wantEditShapes {
		t.Errorf("the declared halves sum to %d, not to the declared total %d", wantEditOpens+wantEditRefusals, wantEditShapes)
	}

	// The kinds are named one at a time rather than counted, so a fixture
	// that quietly stopped creating one leaves a failure saying which.
	reached := map[string]bool{}
	for _, shape := range shapes {
		if shape.kind != "" {
			reached[shape.kind] = true
		}
	}
	for _, kind := range []string{
		bench.KindWorkbench, bench.KindColumn, bench.KindCard,
		bench.KindComment, bench.KindItem, bench.KindAttachment, bench.KindWorkstream,
	} {
		if !reached[kind] {
			t.Errorf("no shape in the swept set names a %s, so this sweep proves nothing about that kind", kind)
		}
	}

	// The containment table is walked a second time, independently of the
	// generator, and every mount it declares is required to have produced a
	// shape. Generating from the table makes that true by construction
	// today; asserting it is what catches a later hand-edit that skips a
	// mount, and it is the property that would have caught this card's
	// defect had a workstream been mounted.
	for _, mount := range mountsReachableFromTheWorkbench() {
		carried := false
		for _, shape := range shapes {
			if strings.HasSuffix(shape.ref, "/"+mount.Dir) {
				carried = true
				break
			}
		}
		if !carried {
			t.Errorf("the containment table declares a %s collection at %s, and the generated set carries no shape ending in that segment", mount.Kind, mount.Dir)
		}
	}

	// Group C is asserted on its own, because a group that silently stopped
	// generating is the failure mode this card is repairing.
	group := editDegenerateShapes(t, fixture)
	if len(group) == 0 {
		t.Fatal("the degenerate group emitted no shape, so the empty reference is guarded by nothing")
	}
	empty := false
	for _, shape := range group {
		if shape.ref == "" {
			empty = true
			if shape.opens {
				t.Error("the empty reference is declared to open a file, and `dinah edit` with nothing after it is somebody who forgot the argument")
			}
			if shape.refusal != contract.UnknownCard {
				t.Errorf("the empty reference is declared to refuse %s, wanted %s", shape.refusal, contract.UnknownCard)
			}
		}
	}
	if !empty {
		t.Error("the degenerate group carries no shape whose reference is the empty string")
	}
}

// TestEditHandsTheEditorTheFileTheResolverNames runs the command rather than
// the resolver, and watches the argument the launched editor was actually
// handed. dinah-467.
//
// Reading the argument is the whole point. The existing guard points
// DINAH_EDITOR at a name no machine carries, which proves that resolution did
// not refuse and nothing else, and that is exactly why `edit` handed over a
// directory for as long as it did.
func TestEditHandsTheEditorTheFileTheResolverNames(t *testing.T) {
	root := newBench(t)
	fixture := buildEditFixture(t, root)
	shapes := editReferenceShapes(t, fixture)

	opened, err := bench.Open(fixture.benchDir)
	if err != nil {
		t.Fatalf("open %s: %v", fixture.benchDir, err)
	}
	refused := contract.ExitCode(contract.OutcomeRefused)
	logs := t.TempDir()

	// The editor is this test binary, which TestMain turns into a recorder
	// when it finds the recording variable set. A real process is launched,
	// so the argument is observed rather than inferred.
	t.Setenv("DINAH_EDITOR", os.Args[0])

	for i, shape := range shapes {
		log := filepath.Join(logs, "shape-"+strconv.Itoa(i)+".log")
		t.Setenv(editorRecordVar, log)
		got := runCLI(t, root, "edit", shape.ref)
		launched := editorLaunches(t, log)
		if !shape.opens {
			if got.code != refused {
				t.Errorf("`dinah edit %q` exited %d, wanted the refused code %d: %s", shape.ref, got.code, refused, got.errw)
			}
			if len(launched) != 0 {
				t.Errorf("`dinah edit %q` is required to refuse, and it launched the editor on %v", shape.ref, launched)
			}
			continue
		}
		if got.code != 0 {
			t.Errorf("`dinah edit %q` exited %d: %s", shape.ref, got.code, got.errw)
			continue
		}
		if len(launched) != 1 {
			t.Errorf("`dinah edit %q` recorded %d editor launches, wanted one: %v", shape.ref, len(launched), launched)
			continue
		}
		answer, err := opened.ResolveEditTarget(shape.ref)
		if err != nil {
			t.Errorf("the resolver refused %q where the command opened %q: %v", shape.ref, launched[0], err)
			continue
		}
		// The two are compared as files rather than as strings. The command
		// resolves from the directory runCLI stands in, which the operating
		// system may report under a spelling of its own (/private/var against
		// /var on macOS), while this call resolves from the path t.TempDir
		// handed out. os.SameFile answers the question actually being asked,
		// which is whether the editor was handed the file the resolver names.
		if !sameFile(launched[0], answer) {
			t.Errorf("`dinah edit %q` handed the editor %q, and the resolver names %q", shape.ref, launched[0], answer)
		}
	}

	// The workstream's own three spellings, held to the path rather than to
	// the resolver, because the resolver agreeing with itself is what let
	// this defect ship. The live pair is the defect the card was filed for
	// and the archived one is the same reference through the mirror.
	live := filepath.Join(fixture.benchDir, bench.WorkstreamsDir, fixture.workstreamID, bench.WorkstreamAnchor)
	for _, want := range []struct {
		ref  string
		path string
	}{
		{bench.WorkstreamRefPrefix + fixture.workstream, live},
		{bench.WorkstreamRefPrefix + fixture.workstreamID, live},
		{bench.WorkstreamRefPrefix + fixture.archived, fixture.archivedAnchor},
	} {
		log := filepath.Join(logs, "workstream-"+strings.ReplaceAll(want.ref, "/", "_")+".log")
		t.Setenv(editorRecordVar, log)
		got := runCLI(t, root, "edit", want.ref)
		if got.code != 0 {
			t.Errorf("`dinah edit %s` exited %d: %s", want.ref, got.code, got.errw)
			continue
		}
		launched := editorLaunches(t, log)
		if len(launched) != 1 {
			t.Errorf("`dinah edit %s` recorded %d editor launches, wanted one: %v", want.ref, len(launched), launched)
			continue
		}
		if !sameFile(launched[0], want.path) {
			t.Errorf("`dinah edit %s` handed the editor %q, wanted %q", want.ref, launched[0], want.path)
		}
	}

	// The refusals `edit` raises today, spelled exactly as they are spelled
	// today. A reference naming a workstream this workbench does not carry
	// must not become an unknown card, which is what reversing the fallback
	// arm's precedence would do to it.
	for _, want := range []struct {
		ref  string
		name string
	}{
		{bench.WorkstreamRefPrefix + "nosuch", contract.UnknownWorkstream},
		{"", contract.UnknownCard},
		{"fx-99", contract.UnknownCard},
	} {
		log := filepath.Join(logs, "refusal-"+strings.ReplaceAll(want.ref, "/", "_")+".log")
		t.Setenv(editorRecordVar, log)
		got := runCLI(t, root, "edit", want.ref)
		if got.code != refused {
			t.Errorf("`dinah edit %q` exited %d, wanted the refused code %d: %s", want.ref, got.code, refused, got.errw)
		}
		if leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]; leading != want.name {
			t.Errorf("`dinah edit %q` refused %q, wanted %q", want.ref, leading, want.name)
		}
		if launched := editorLaunches(t, log); len(launched) != 0 {
			t.Errorf("`dinah edit %q` refused and launched the editor on %v", want.ref, launched)
		}
	}

	// The argument nobody typed. A shape carries a reference by
	// construction, so the absent argument is only reachable here, and it is
	// the case round 2 of this card's spec turned on.
	log := filepath.Join(logs, "no-argument.log")
	t.Setenv(editorRecordVar, log)
	got := runCLI(t, root, "edit")
	if got.code != refused {
		t.Errorf("`dinah edit` with no reference exited %d, wanted the refused code %d: %s", got.code, refused, got.errw)
	}
	if launched := editorLaunches(t, log); len(launched) != 0 {
		t.Errorf("`dinah edit` with no reference launched the editor on %v", launched)
	}
}

// editReferenceShapes generates the swept set rather than listing it, so a
// mount added to the containment table later produces new shapes with no edit
// to this file. That is the property that would have caught this card's
// defect early, and it is weaker than it looks: a kind reached by a prefix of
// its own rather than by the containment table is invisible to it, which is
// what a workstream is and why the kinds are also named one at a time above.
func editReferenceShapes(t *testing.T, fixture editFixture) []editShape {
	t.Helper()
	shapes := editContainedShapes(t, fixture)
	shapes = append(shapes, editDeclaredShapes(t, fixture)...)
	shapes = append(shapes, editDegenerateShapes(t, fixture)...)
	return shapes
}

// mountsReachableFromTheWorkbench is the containment table read for the
// expectation rather than for the generation, so the sweep's coverage of that
// table is asserted rather than assumed. A kind is walked once, since the
// attachment collection hangs from four kinds and the walk would otherwise
// not terminate on a table that ever mounted a cycle.
func mountsReachableFromTheWorkbench() []bench.Mount {
	var mounts []bench.Mount
	seen := map[string]bool{}
	var walk func(kind string)
	walk = func(kind string) {
		if seen[kind] {
			return
		}
		seen[kind] = true
		for _, mount := range bench.Contains(kind) {
			mounts = append(mounts, mount)
			walk(mount.Kind)
		}
	}
	walk(bench.KindWorkbench)
	return mounts
}

// editContainedShapes is group A: every collection the containment table
// mounts, reachable from the workbench, and the member the fixture put in it.
//
// The refusal a collection carries is read off bench.AddressedInItsOwnRight
// rather than written out here, because a test naming card and column for
// itself would be a second copy of a rule `descend` already declares and the
// two would drift.
func editContainedShapes(t *testing.T, fixture editFixture) []editShape {
	t.Helper()
	direct := map[string]struct{ ref, dir string }{
		bench.KindColumn: {fixture.column, fixture.columnDir},
		bench.KindCard:   {fixture.card, fixture.cardDir},
	}
	var shapes []editShape
	var walk func(kind, dir, ref string)
	walk = func(kind, dir, ref string) {
		for _, mount := range bench.Contains(kind) {
			collection := ref + "/" + mount.Dir
			refusal := contract.IsACollection
			if bench.AddressedInItsOwnRight(mount.Kind) {
				refusal = contract.UnknownPath
			}
			shapes = append(shapes, editShape{ref: collection, refusal: refusal})
			entries, err := os.ReadDir(filepath.Join(dir, mount.Dir))
			if err != nil || len(entries) == 0 {
				continue
			}
			memberRef := collection + "/1"
			memberDir := filepath.Join(dir, mount.Dir, entries[0].Name())
			if named, ok := direct[mount.Kind]; ok {
				memberRef, memberDir = named.ref, named.dir
			}
			shapes = append(shapes, editShape{ref: memberRef, kind: mount.Kind, opens: true})
			walk(mount.Kind, memberDir, memberRef)
		}
	}
	walk(bench.KindWorkbench, fixture.benchDir, "workbench")
	return shapes
}

// editDeclaredShapes is group B: the shapes the grammar declares outside the
// containment table, each named beside where the resolver declares it.
//
// The three Short spellings of the checklist segments (oq, ac, d) are
// deliberately not emitted. They resolve the same three collections the Word
// spellings do, so they would add shapes without adding reach, and
// internal/bench holds the two spellings together already.
func editDeclaredShapes(t *testing.T, fixture editFixture) []editShape {
	t.Helper()
	shapes := []editShape{
		// bench.IsWorkbenchRef admits both spellings of the workbench.
		{ref: "workbench", kind: bench.KindWorkbench, opens: true},
		{ref: ".", kind: bench.KindWorkbench, opens: true},
		// The bare workbench slug is a head resolveBelowLanding admits.
		{ref: fixture.slug + "/" + bench.AttachmentsDir + "/1", kind: bench.KindAttachment, opens: true},
		// bench.cardOwnFileSegment admits the card's own two files. The
		// journal names no entity of the format, so ResolveReference
		// refuses it and the fallback arm answers it.
		{ref: fixture.card + "/" + bench.KindCard, kind: bench.KindCard, opens: true},
		{ref: fixture.card + "/journal", opens: true},
	}
	// bench.WordForItemKind is the one declaration of how a checklist kind
	// is spelled in a reference, so the words are read from it rather than
	// written out again here.
	for _, kind := range bench.ItemKinds {
		word, ok := bench.WordForItemKind(kind)
		if !ok {
			t.Fatalf("the format declares the item kind %q and no reference segment for it", kind)
		}
		shapes = append(shapes,
			editShape{ref: fixture.card + "/" + word, refusal: contract.IsACollection},
			editShape{ref: fixture.card + "/" + word + "/1", kind: bench.KindItem, opens: true},
		)
	}
	shapes = append(shapes,
		// bench.PayloadDir names the file an attachment wraps, which carries
		// no anchor and so names no entity.
		editShape{ref: fixture.card + "/" + bench.AttachmentsDir + "/1/" + bench.PayloadDir, opens: true},
		// bench.WorkstreamRefPrefix reaches a workstream by either spelling.
		editShape{ref: bench.WorkstreamRefPrefix + fixture.workstream, kind: bench.KindWorkstream, opens: true},
		editShape{ref: bench.WorkstreamRefPrefix + fixture.workstreamID, kind: bench.KindWorkstream, opens: true},
	)
	return shapes
}

// editDegenerateShapes is group C: every degenerate reference the design
// admits, with the verdict the spec's own table declares for it. The empty
// reference is guarded here as one member of a class rather than as the one
// instance somebody happened to repair.
//
// The absent argument is not a shape, because a shape carries a reference by
// construction. It is run on its own in
// TestEditHandsTheEditorTheFileTheResolverNames.
func editDegenerateShapes(t *testing.T, fixture editFixture) []editShape {
	t.Helper()
	prefix := bench.WorkstreamRefPrefix
	return []editShape{
		// Both resolvers trim, so an argument of whitespace alone takes the
		// same arm as an argument nobody typed.
		{ref: "", refusal: contract.UnknownCard},
		{ref: "   ", refusal: contract.UnknownCard},
		{ref: "\t", refusal: contract.UnknownCard},
		{ref: "/", refusal: contract.UnknownCard},
		{ref: "//", refusal: contract.UnknownCard},
		{ref: "..", refusal: contract.UnknownCard},
		// A trailing slash makes the entity resolver miss where ResolvePath
		// hits, so these four reach the fallback arm and keep opening what
		// they open today.
		{ref: fixture.card + "/", kind: bench.KindCard, opens: true},
		{ref: fixture.card + "/" + bench.KindCard + "/", kind: bench.KindCard, opens: true},
		{ref: "workbench/", kind: bench.KindWorkbench, opens: true},
		{ref: fixture.column + "/", kind: bench.KindColumn, opens: true},
		{ref: fixture.card + "//" + bench.KindCard, refusal: contract.UnknownPath},
		// The workstream prefix swallows everything after it, so each of
		// these names one workstream nothing answers to.
		{ref: prefix, refusal: contract.UnknownWorkstream},
		{ref: prefix + "/", refusal: contract.UnknownWorkstream},
		{ref: strings.TrimSuffix(prefix, "/"), refusal: contract.UnknownCard},
		{ref: prefix + fixture.workstream + "/", refusal: contract.UnknownWorkstream},
		{ref: prefix + prefix + fixture.workstream, refusal: contract.UnknownWorkstream},
		{ref: " " + fixture.card + " ", kind: bench.KindCard, opens: true},
	}
}

// buildEditFixture files everything the generator reaches: an attachment on
// the workbench, one column carrying an attachment, one card carrying a
// comment with an attachment of its own, one checklist item of each of the
// three kinds, an attachment on the card, one live workstream and one
// archived workstream.
func buildEditFixture(t *testing.T, root string) editFixture {
	t.Helper()
	card := addCard(t, root, "a card with things below it")
	mustRun(t, root, "comment", card, "a first thought")
	for _, kind := range bench.ItemKinds {
		mustRun(t, root, "file", card, kind, "something to settle about "+kind)
	}
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}
	mustRun(t, root, "attach", "workbench", source)
	mustRun(t, root, "attach", "doing", source)
	mustRun(t, root, "attach", card, source)
	mustRun(t, root, "attach", card+"/"+bench.CommentsDir+"/1", source)
	mustRun(t, root, "workstream", "new", "Autumn release", "--slug", "autumn")
	mustRun(t, root, "workstream", "new", "Retired work", "--slug", "retired")

	dir := benchDir(t, root)
	fixture := editFixture{
		root:       root,
		benchDir:   dir,
		slug:       "fx",
		card:       card,
		cardDir:    filepath.Join(dir, bench.CardsDir, cardID(t, root, card)),
		column:     "doing",
		columnDir:  columnDirBySlug(t, dir, "doing"),
		workstream: "autumn",
		archived:   "retired",
	}
	fixture.workstreamID = workstreamIDBySlug(t, filepath.Join(dir, bench.WorkstreamsDir), "autumn")
	fixture.archivedID = workstreamIDBySlug(t, filepath.Join(dir, bench.WorkstreamsDir), "retired")
	mustRun(t, root, "archive", bench.WorkstreamRefPrefix+"retired")
	fixture.archivedAnchor = filepath.Join(dir, bench.ArchiveDir, bench.WorkstreamsDir, fixture.archivedID, bench.WorkstreamAnchor)
	if !bench.Exists(fixture.archivedAnchor) {
		t.Fatalf("the archived workstream's anchor is not at %s, so this fixture proves nothing about the mirror", fixture.archivedAnchor)
	}
	return fixture
}

// columnDirBySlug finds a column's directory by reading the slug off each
// column anchor. The resolver is not asked, because the fixture's own
// addresses must not come from the code the sweep is holding to account.
func columnDirBySlug(t *testing.T, dir, slug string) string {
	t.Helper()
	return anchorDirByField(t, filepath.Join(dir, bench.ColumnsDir), bench.ColumnAnchor, "slug", slug)
}

// workstreamIDBySlug finds a workstream's identifier the same way, which is
// what the identifier spelling of the reference needs.
func workstreamIDBySlug(t *testing.T, dir, slug string) string {
	t.Helper()
	return filepath.Base(anchorDirByField(t, dir, bench.WorkstreamAnchor, "slug", slug))
}

// anchorDirByField scans a collection for the one member whose anchor carries
// a field value, and fails rather than answering an empty string, since a
// fixture that stopped creating the entity would otherwise leave the sweep
// resolving references built out of nothing.
func anchorDirByField(t *testing.T, collection, anchor, field, value string) string {
	t.Helper()
	entries, err := os.ReadDir(collection)
	if err != nil {
		t.Fatalf("read %s: %v", collection, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		text, err := bench.ReadText(filepath.Join(collection, entry.Name(), anchor))
		if err != nil {
			continue
		}
		fm, _ := bench.ParseAnchor(text)
		if fm.Value(field) == value {
			return filepath.Join(collection, entry.Name())
		}
	}
	t.Fatalf("%s holds no member whose %s is %q", collection, field, value)
	return ""
}

// editorLaunches reads back what the launched editors recorded, one line per
// launch. A missing file is no launch at all, which is what a refusal must
// leave behind.
func editorLaunches(t *testing.T, log string) []string {
	t.Helper()
	text, err := os.ReadFile(log)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read %s: %v", log, err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimRight(string(text), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

// sameFile reports whether two paths name one file, which is the comparison
// this sweep needs because a temporary directory is reachable under more than
// one spelling: macOS reports the same tree as /var/folders/... and as
// /private/var/folders/..., and a launched process resolving from its own
// working directory picks the second where a caller joining t.TempDir's answer
// picks the first. The comparison falls back to the strings when either path
// cannot be stat'ed, so a mismatch is reported rather than swallowed.
func sameFile(got, want string) bool {
	gotInfo, gotErr := os.Stat(got)
	wantInfo, wantErr := os.Stat(want)
	if gotErr != nil || wantErr != nil {
		return got == want
	}
	return os.SameFile(gotInfo, wantInfo)
}
