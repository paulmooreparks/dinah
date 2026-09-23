package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// declareFieldsOn writes a fields block into a workbench's own anchor, which
// is the only way one arrives: no verb declares a field, and the block is
// frontmatter a person edits.
func declareFieldsOn(t *testing.T, root, block string) {
	t.Helper()
	dir := soleBenchDir(t, root)
	path := filepath.Join(dir, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.FieldsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// cliDeclaration is the block the cases below start from: one key on a card
// alone, and one on every kind because it names none.
const cliDeclaration = `fields:
  git.branch:
    type: string
    meaning: the branch the card's code lives on
    on: [card]
  venue.deposit-paid:
    type: date
    meaning: the day the deposit fell due
`

// cliWorkstreamDeclaration is cliDeclaration with a key declared on a
// workstream alone, which is the one kind dinah-582 added to the set an `on`
// member may name. It is a block of its own rather than a row added to the
// one above, so the cases already reading that block keep the field set they
// were written against.
const cliWorkstreamDeclaration = cliDeclaration + `  trip.destination:
    type: string
    meaning: the city the trip is to
    on: [workstream]
`

// TestShowPrintsTheDeclaredFieldsInDeclarationOrder asserts what a reader
// sees. A card prints one line per declared field it carries, in the order the
// workbench declares them, after the two levels and before the holder; a
// declared field the card does not carry prints nothing, and a key the card
// stores that the workbench does not declare prints nothing either.
func TestShowPrintsTheDeclaredFieldsInDeclarationOrder(t *testing.T) {
	root := newBench(t)
	declareFieldsOn(t, root, cliDeclaration)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, write := range [][]string{
		{"set", "fx-1", "venue.deposit-paid", "2026-09-14"},
		{"set", "fx-1", "git.branch", "dinah-498-declared-fields"},
	} {
		if got := runCLI(t, root, write...); got.code != 0 {
			t.Fatalf("%v: %d %s", write, got.code, got.errw)
		}
	}
	// A key nothing declares is planted by hand, because no write would land
	// one, and it is what the "prints nothing" half is read against.
	dir := soleBenchDir(t, root)
	anchor := filepath.Join(dir, bench.CardsDir, soleCardID(t, dir), bench.CardAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the card anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	bench.SetFieldValue(fm, "imported.key", "kept")
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the card anchor: %v", err)
	}

	got := runCLI(t, root, "show", "fx-1")
	if got.code != 0 {
		t.Fatalf("show: %d %s", got.code, got.errw)
	}
	lines := strings.Split(got.out, "\n")
	at := func(want string) int {
		for i, line := range lines {
			if strings.TrimSpace(line) == want {
				return i
			}
		}
		t.Errorf("the output carries no line %q:\n%s", want, got.out)
		return -1
	}
	branch := at("git.branch: dinah-498-declared-fields")
	deposit := at("venue.deposit-paid: 2026-09-14")
	if branch < 0 || deposit < 0 {
		return
	}
	if branch > deposit {
		t.Errorf("the two lines print at %d and %d, wanted the order the workbench declares them in:\n%s", branch, deposit, got.out)
	}
	if strings.Contains(got.out, "imported.key") {
		t.Errorf("show prints a key the workbench does not declare:\n%s", got.out)
	}

	// A second card carries no value at all, so no field line prints.
	if added := runCLI(t, root, "add", "Another card"); added.code != 0 {
		t.Fatalf("add: %d %s", added.code, added.errw)
	}
	bare := runCLI(t, root, "show", "fx-2")
	if bare.code != 0 {
		t.Fatalf("show: %d %s", bare.code, bare.errw)
	}
	if strings.Contains(bare.out, "git.branch") || strings.Contains(bare.out, "venue.deposit-paid") {
		t.Errorf("a card carrying no value still prints a field line:\n%s", bare.out)
	}
}

// TestAnUndeclaredWriteIsRefusedWithTheKeysItCouldHaveUsed asserts the
// refusal a person meets: the name, the key they typed, the keys this
// workbench does declare on that kind, and what to do next.
func TestAnUndeclaredWriteIsRefusedWithTheKeysItCouldHaveUsed(t *testing.T) {
	root := newBench(t)
	declareFieldsOn(t, root, cliDeclaration)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	got := runCLI(t, root, "set", "fx-1", "git.upstream", "origin")
	if got.code == 0 {
		t.Fatalf("the undeclared write succeeded:\n%s", got.out)
	}
	for _, want := range []string{"undeclared-field", "git.upstream", "git.branch", "venue.deposit-paid", "workbench.md"} {
		if !strings.Contains(got.errw, want) {
			t.Errorf("the refusal does not carry %q:\n%s", want, got.errw)
		}
	}

	// The same key written where the declaration does not reach names the
	// kinds it does reach, which is the other half of this one refusal.
	elsewhere := runCLI(t, root, "set", "doing", "git.branch", "main")
	if elsewhere.code == 0 {
		t.Fatalf("a write to a column the declaration does not name succeeded:\n%s", elsewhere.out)
	}
	if !strings.Contains(elsewhere.errw, "card") {
		t.Errorf("the refusal does not name the kind the declaration lists:\n%s", elsewhere.errw)
	}
}

// TestTheBranchMigrationReportsItsClassificationAndItsWrites asserts the
// command surface of the migration: the preview writes nothing and says so,
// the confirmed run lifts, empties, declares and stamps, and a conflict stops
// the run with a non-zero exit and names every card it cannot carry across.
func TestTheBranchMigrationReportsItsClassificationAndItsWrites(t *testing.T) {
	root := newBench(t)
	dir := soleBenchDir(t, root)
	for _, title := range []string{"A card", "An empty heading", "No heading"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	setBody(t, root, "fx-1", "Framing.\n\n## Branch\n\ndinah-498-declared-fields\n\nMore framing.\n")
	setBody(t, root, "fx-2", "Framing.\n\n## Branch\n")
	// The workbench a fresh init writes already declares the format the
	// retirement arrived at, so it is wound back to the one before it: the
	// migration exists for a workbench standing where the retirement finds
	// one, and a fixture already stamped would exercise nothing.
	//
	// The winding happens after the cards are filed rather than before,
	// because since dinah-525 an ordinary command is refused outright on a
	// store below the current format. Filing first and winding back leaves
	// the fixture in the state the migration is for, which is what this case
	// was always about; filing afterwards would be testing that refusal.
	windBackFormat(t, dir)
	// The store is below the current format from here on, so every read below
	// goes through check, which opens such a store because it is the
	// diagnostic, or through the anchor files, rather than through an
	// ordinary command, which is refused.
	preview := runCLI(t, root, "check", "--migrate-branches")
	if preview.code != 0 {
		t.Fatalf("the preview exited %d: %s", preview.code, preview.errw)
	}
	for _, want := range []string{"Nothing was written", "dinah-498-declared-fields"} {
		if !strings.Contains(preview.out, want) {
			t.Errorf("the preview does not say %q:\n%s", want, preview.out)
		}
	}
	// Every card the preview names is named under a sentence about what a
	// confirmed run would do. The written phrasings are read off the catalog
	// rather than typed here, so a rewording moves the assertion with the
	// output, and each is asked for in both directions: a preview carrying one
	// of them contradicts its own first line, and a confirmed run carrying the
	// conditional one describes work it did do as work it might.
	catalog := msg.For(msg.Base)
	written := []string{
		catalog.T("check.branch-lift", "card", "fx-1", "branch", "dinah-498-declared-fields"),
		catalog.T("check.branch-emptied", "card", "fx-2"),
		catalog.TN("check.branches-emptied", 1),
	}
	conditional := []string{
		catalog.T("check.branch-would-lift", "card", "fx-1", "branch", "dinah-498-declared-fields"),
		catalog.T("check.branch-would-empty", "card", "fx-2"),
		catalog.TN("check.branches-would-empty", 1),
	}
	for _, line := range written {
		if strings.Contains(preview.out, line) {
			t.Errorf("the preview says %q, and it wrote nothing:\n%s", line, preview.out)
		}
	}
	for _, line := range conditional {
		if !strings.Contains(preview.out, line) {
			t.Errorf("the preview does not say %q:\n%s", line, preview.out)
		}
	}
	if anchorBodyOf(t, dir, "fx-1") == "" || !strings.Contains(anchorBodyOf(t, dir, "fx-1"), "## Branch") {
		t.Error("the preview rewrote a body")
	}

	applied := runCLI(t, root, "check", "--migrate-branches", "--yes")
	if applied.code != 0 {
		t.Fatalf("the migration exited %d: %s", applied.code, applied.errw)
	}
	for _, want := range []string{"dinah-498-declared-fields", "git.branch", "4"} {
		if !strings.Contains(applied.out, want) {
			t.Errorf("the migration does not report %q:\n%s", want, applied.out)
		}
	}
	for _, line := range written {
		if !strings.Contains(applied.out, line) {
			t.Errorf("the confirmed run does not say %q:\n%s", line, applied.out)
		}
	}
	for _, line := range conditional {
		if strings.Contains(applied.out, line) {
			t.Errorf("the confirmed run says %q, and it did write:\n%s", line, applied.out)
		}
	}
	if strings.Contains(anchorBodyOf(t, dir, "fx-1"), "## Branch") {
		t.Error("the migrated card still carries the heading")
	}
	// The lifted value prints, which needs the rest of the chain: the branch
	// migration stamps its own format, which is still below the current one,
	// and an ordinary command is refused until the note migration has run.
	migrateNotes(t, dir)
	shown := runCLI(t, root, "show", "fx-1")
	if !strings.Contains(shown.out, "git.branch: dinah-498-declared-fields") {
		t.Errorf("the lifted value does not print:\n%s", shown.out)
	}

	// A fresh workbench with a conflict on it, so the refusing case is read
	// against a run that would otherwise have written. The cards are filed
	// before the format is wound back, because an ordinary command is refused
	// on a store below the current format.
	other := newBench(t)
	otherDir := soleBenchDir(t, other)
	for _, title := range []string{"Two headings", "Would have been lifted"} {
		if got := runCLI(t, other, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	setBody(t, other, "fx-1", "Framing.\n\n## Branch\n\nfirst\n\n## Branch\n\nsecond\n")
	setBody(t, other, "fx-2", "Framing.\n\n## Branch\n\nwould-have-been-lifted\n")
	windBackFormat(t, otherDir)
	conflicted := runCLI(t, other, "check", "--migrate-branches", "--yes")
	if conflicted.code == 0 {
		t.Fatalf("a run that met a conflict exited zero:\n%s", conflicted.out)
	}
	if !strings.Contains(conflicted.out, "fx-1") {
		t.Errorf("the conflict report does not name the card:\n%s", conflicted.out)
	}
	if !strings.Contains(anchorBodyOf(t, otherDir, "fx-2"), "## Branch") {
		t.Error("a run that met a conflict rewrote a card it could have lifted")
	}
}

// windBackFormat stamps a workbench with the format before the retirement, so
// a fixture the tool has just written stands where the migration finds one.
func windBackFormat(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set("format", "3")
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// setBody writes a card's whole body, which is what the migration reads and
// rewrites.
func setBody(t *testing.T, root, card, body string) {
	t.Helper()
	if got := runCLIWithInput(t, root, strings.NewReader(body), "set", card, "body", "-"); got.code != 0 {
		t.Fatalf("set the body of %s: %d %s", card, got.code, got.errw)
	}
}

// bodyOf reads a card's body back off disk.
func bodyOf(t *testing.T, root, card string) string {
	t.Helper()
	got := runCLI(t, root, "get", card, "body")
	if got.code != 0 {
		t.Fatalf("get the body of %s: %d %s", card, got.code, got.errw)
	}
	return got.out
}

// soleCardID answers the identifier of the one card a workbench carries, for
// a case that has to reach an anchor no command writes.
func soleCardID(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, bench.CardsDir))
	if err != nil {
		t.Fatalf("list the cards: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return entry.Name()
		}
	}
	t.Fatal("the workbench carries no card")
	return ""
}

// TestHelpForSetOffersTheDeclaredKeysBesideTheBuiltInNames asserts the second
// vocabulary source. The field argument of `set` reaches a field of a kind's
// own set and a key the reader's own workbench declares alike, so the help
// page offers both; asked where no workbench opens, it offers the half that
// lives in the binary rather than nothing at all.
func TestHelpForSetOffersTheDeclaredKeysBesideTheBuiltInNames(t *testing.T) {
	root := newBench(t)
	declareFieldsOn(t, root, cliDeclaration)
	got := runCLI(t, root, "help", "set")
	if got.code != 0 {
		t.Fatalf("help set: %d %s", got.code, got.errw)
	}
	for _, want := range []string{"severity", "git.branch", "venue.deposit-paid"} {
		if !strings.Contains(got.out, want) {
			t.Errorf("the help page for set does not offer %q:\n%s", want, got.out)
		}
	}

	// The page away from a workbench offers no vocabulary at all rather than
	// half of one. vocabularyValues opens the workbench before it resolves a
	// source that lives in one and answers nothing when that open fails, so
	// this is the head's own rule rather than this source's.
	bare := t.TempDir()
	away := runCLI(t, bare, "help", "set")
	if away.code != 0 {
		t.Fatalf("help set away from a workbench: %d %s", away.code, away.errw)
	}
	if strings.Contains(away.out, "git.branch") {
		t.Errorf("the help page away from a workbench offers a key no workbench declared:\n%s", away.out)
	}
}

// TestThePublishedCheckListsCarryTheFieldRowWhereCanLandRunsIt asserts both
// renumbered lists on the surface a reader meets them on.
//
// The printed position of a row is its catalog key's number plus two, because
// each page prefixes the two workbench rows Checks supplies, so the two are
// asserted apart rather than conflated. The printed count is asserted as well,
// which is what catches a row inserted in the table and dropped by the
// renderer.
//
// The rows are read out of verb.Checks rather than written out here, so a
// reordering the renderer does not follow fails rather than passing against a
// list this test carries its own copy of.
func TestThePublishedCheckListsCarryTheFieldRowWhereCanLandRunsIt(t *testing.T) {
	cases := []struct {
		command  string
		rows     int
		position int
	}{
		// Each count and each position moved by one at dinah-496, which put the
		// harness row at the head of every writing command's list, and pull's
		// count moved by three because the tier gate's other two answers were
		// appended behind its own tier row. move's count and position moved by
		// one more at dinah-540, which inserted a no-owner row ahead of the
		// field row.
		{command: "move", rows: 16, position: 14},
		{command: "pull", rows: 22, position: 15},
	}
	numbered := regexp.MustCompile(`^  (\d+) `)
	root := newBench(t)
	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			declared := verb.Checks(c.command)
			if len(declared) != c.rows {
				t.Fatalf("Checks(%q) answers %d rows, wanted %d", c.command, len(declared), c.rows)
			}
			if got := declared[c.position-1].Refusal; got != contract.MissingField {
				t.Errorf("row %d of %s refuses %s, wanted %s", c.position, c.command, got, contract.MissingField)
			}
			got := runCLI(t, root, "help", c.command)
			if got.code != 0 {
				t.Fatalf("help %s: %d %s", c.command, got.code, got.errw)
			}
			printed := 0
			at := 0
			catalog := msg.For(msg.Base)
			want := catalog.T(declared[c.position-1].Key)
			for _, line := range strings.Split(got.out, "\n") {
				m := numbered.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				printed++
				if strings.Contains(line, want) {
					number, err := strconv.Atoi(m[1])
					if err != nil {
						t.Fatalf("reading the row number of %q: %v", line, err)
					}
					at = number
				}
			}
			if printed != c.rows {
				t.Errorf("help %s prints %d rows, wanted %d", c.command, printed, c.rows)
			}
			if at != c.position {
				t.Errorf("help %s prints the field row at %d, wanted %d", c.command, at, c.position)
			}
		})
	}
}

// TestTheRenumberedCheckKeysCarryTheirNewTextInEveryCatalogue asserts what the
// renumbering owed the catalogs. Moving a row is a rewrite in eight files
// rather than a rename in one, and a key that moved and kept the text of its
// old position is the defect this catches.
//
// The expectation is the English text of each key, read off the base catalog,
// which is the text the check list itself is generated from. Every shipped
// catalog is then asked whether it carries that key at all, because
// TestEveryDeclaredLanguageShips answers presence and says nothing about which
// sentence a moved key came to rest on.
func TestTheRenumberedCheckKeysCarryTheirNewTextInEveryCatalogue(t *testing.T) {
	moved := map[string]string{
		"check.move.14": "the card carries a value for every field the destination requires",
		"check.move.15": "the card has not reached the departure column's loop limit",
		"check.move.16": "the card carries no unresolved item that names the departure column",
		"check.pull.12": "the card carries a value for every field the destination requires",
		"check.pull.13": "the card carries no unresolved item that names the departure column",
		"check.pull.14": "taking the card up at the destination is legal for whoever asks",
		"check.pull.15": "the destination is not being retired",
		"check.pull.16": "every unresolved item the card carries names a declared column",
		"check.pull.17": "your declared tier is at or above what the card asks there",
	}
	base := msg.For(msg.Base)
	checked := 0
	for key, want := range moved {
		if got := base.T(key); got != want {
			t.Errorf("%s reads %q in English, wanted %q", key, got, want)
		}
		for _, tag := range msg.Tags() {
			entry, carried := msg.CatalogEntry(tag, key)
			if !carried || entry.Text == "" {
				t.Errorf("%s carries no %s", tag, key)
				continue
			}
			checked++
		}
	}
	if want := len(moved) * len(msg.Tags()); checked != want {
		t.Errorf("the sweep read %d entries, wanted %d", checked, want)
	}
	if checked == 0 {
		t.Fatal("the sweep read no entry, so it asserts nothing")
	}

	// The keys that arrived rather than moved get their own count, because the
	// two populations fail apart: a key added to English alone leaves the
	// sweep above untouched, and a key that moved and kept its old text leaves
	// this one untouched.
	arrived := []string{
		"refusal.undeclared-field",
		"refusal.undeclared-field.elsewhere",
		"refusal.undeclared-field.next",
		"refusal.missing-field",
		"refusal.missing-field.next",
		"check.field-declaration-malformed",
		"check.required-field-undeclared",
		"check.branch-heading-in-body",
		"card.field",
		"check.branches-preview",
		"check.branches-lifted.one",
		"check.branches-lifted.other",
		"check.branches-untouched.one",
		"check.branches-untouched.other",
		"check.branches-conflicted.one",
		"check.branches-conflicted.other",
		"check.branch-conflict.anchor-differs",
		"check.branch-conflict.two-headings",
		"check.branch-conflict.unreadable",
		"check.branch-lift",
		"check.branch-would-lift",
		"check.branch-emptied",
		"check.branch-would-empty",
		"check.branches-would-empty.one",
		"check.branches-would-empty.other",
		"check.branches-emptied.one",
		"check.branches-emptied.other",
		"check.branches-declared",
		"check.branches-stamped",
		"param.check.migrate-branches.summary",
	}
	if len(arrived) == 0 {
		t.Fatal("this card added no catalog key, so the sweep below asserts nothing")
	}
	present := 0
	for _, key := range arrived {
		for _, tag := range msg.Tags() {
			entry, carried := msg.CatalogEntry(tag, key)
			if !carried || entry.Text == "" {
				t.Errorf("%s carries no %s", tag, key)
				continue
			}
			present++
		}
	}
	if want := len(arrived) * len(msg.Tags()); present != want {
		t.Errorf("the sweep over the keys this card added read %d entries, wanted %d", present, want)
	}
}

// anchorBodyOf reads one card's body off its own anchor, for a case whose
// store stands below the current format and which no ordinary command will
// therefore open.
//
// The card is found through the number registry, which is how a reference
// resolves to an identifier, so this reads the store the way the resolver does
// rather than guessing at a directory name.
func anchorBodyOf(t *testing.T, dir, ref string) string {
	t.Helper()
	number := ref[strings.LastIndex(ref, "-")+1:]
	lines, err := bench.ReadText(filepath.Join(dir, bench.CardNumbersName))
	if err != nil {
		t.Fatalf("read the number registry: %v", err)
	}
	for _, line := range strings.Split(lines, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != number {
			continue
		}
		text, err := bench.ReadText(filepath.Join(dir, bench.CardsDir, fields[1], bench.CardAnchor))
		if err != nil {
			t.Fatalf("read the anchor of %s: %v", ref, err)
		}
		_, body := bench.ParseAnchor(text)
		return body
	}
	t.Fatalf("the registry carries no line for %s", ref)
	return ""
}

// TestAWorkstreamKeyIsWrittenReadAndShownThroughTheHead is dinah-582 end to
// end, through the surface a person actually types. A workbench declares a key
// on a workstream alone, an actor who is not the operator writes it, a read
// answers it back, and show draws a row for it under the built-in ones.
//
// The head is where the resolver and the renderer meet, and neither is
// exercised by a library test: the library is handed a reference already
// parsed and answers a Record nobody has drawn. The non-operator actor is the
// point of the write, because the operator's ruling was about an agent
// provisioning a workstream on his behalf.
func TestAWorkstreamKeyIsWrittenReadAndShownThroughTheHead(t *testing.T) {
	root := newBench(t)
	declareFieldsOn(t, root, cliWorkstreamDeclaration)
	if got := runCLI(t, root, "--actor", "bob", "workstream", "new", "Berlin trip", "--slug", "berlin"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "--actor", "bob", "set", "workstream/berlin", "trip.destination", "Berlin"); got.code != 0 {
		t.Fatalf("a non-operator writing a declared key on a workstream: %d %s", got.code, got.errw)
	}
	read := runCLI(t, root, "get", "workstream/berlin", "trip.destination")
	if read.code != 0 {
		t.Fatalf("get: %d %s", read.code, read.errw)
	}
	if strings.TrimSpace(read.out) != "Berlin" {
		t.Errorf("get answers %q, wanted Berlin", read.out)
	}
	shown := runCLI(t, root, "show", "workstream/berlin")
	if shown.code != 0 {
		t.Fatalf("show: %d %s", shown.code, shown.errw)
	}
	rowAt := func(out, name string) int {
		for i, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), name+" ") {
				return i
			}
		}
		return -1
	}
	declared := rowAt(shown.out, "trip.destination")
	if declared < 0 || !strings.Contains(shown.out, "Berlin") {
		t.Fatalf("show draws no row for the declared key:\n%s", shown.out)
	}
	if status := rowAt(shown.out, "status"); status < 0 || declared < status {
		t.Errorf("the declared row prints at %d and the built-in status row at %d, wanted the declared one after:\n%s", declared, status, shown.out)
	}

	// A second workstream carries no value, so no row prints for it, which
	// is what parts a declared row from a built-in one at the renderer.
	if got := runCLI(t, root, "--actor", "bob", "workstream", "new", "Oslo trip", "--slug", "oslo"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	bare := runCLI(t, root, "show", "workstream/oslo")
	if bare.code != 0 {
		t.Fatalf("show: %d %s", bare.code, bare.errw)
	}
	if strings.Contains(bare.out, "trip.destination") {
		t.Errorf("a workstream carrying no value still draws a field row:\n%s", bare.out)
	}

	// The key reaches a workstream and nothing else, and the refusal on a
	// card names the kind it does reach.
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	refused := runCLI(t, root, "set", "fx-1", "trip.destination", "Berlin")
	if refused.code == 0 {
		t.Fatalf("a workstream key written on a card succeeded:\n%s", refused.out)
	}
	for _, want := range []string{"undeclared-field", "trip.destination", "workstream"} {
		if !strings.Contains(refused.errw, want) {
			t.Errorf("the refusal does not carry %q:\n%s", want, refused.errw)
		}
	}
}
