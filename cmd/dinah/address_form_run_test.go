package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/addressform"
	"dinah/internal/bench"
)

// addressFormFixture is the workbench the running guards drive, with the
// spellings and the answers each declared form is measured against.
//
// Every value here was chosen so that no two of them can coincide, because a
// fixture whose values coincide gives a probe that cannot fail. Two such
// probes were caught in this card's spec before any code was written, and both
// came from the same cause.
//
// columnTitle is "In Flight", which is no column's slug under ASCII lowering.
// ColumnByRef tries every column's slug before any column's title and
// lowercases both, and dinah init titles each column with its own slug
// capitalised, so an example of "Doing" matches on the slug arm and goes on
// resolving against a build with the title arm deleted.
//
// No column carries the card's number as its slug, its title or its
// identifier. resolveReferenceBody asks columnByRefIn before resolveCardIn for
// a bare head, so a column called "1" would answer the card-number probe. The
// fixture's columns are intake, doing and the retitled done, and a column
// identifier is twelve hex characters.
//
// A third resolver has the same ordering without the same exposure: pick tries
// an identifier, then a position, then a name, so the member-name example must
// not be a bare integer, which an attachment filename carrying an extension
// never is. member-position is safe against the arm ahead of it because an
// identifier is twelve hex characters.
//
// The two bare workstream forms are joined to two different cards, so that the
// second probe cannot be answered by the membership the first one wrote.
type addressFormFixture struct {
	root       string
	benchDir   string
	slug       string
	card       string
	cardID     string
	cardDir    string
	secondCard string

	columnSlug       string
	columnSlugDir    string
	columnTitle      string
	columnTitleDir   string
	columnIdentifier string
	columnIDDir      string

	workstreamSlug string
	workstreamID   string
	workstreamDir  string

	commentDir     string
	commentID      string
	attachmentName string
	attachmentDir  string

	workbenchDirName string
}

// buildAddressFormFixture files everything the example generator reaches: two
// cards, a comment and an attachment below the first, a workstream, and a
// column retitled so that its title is reachable only through the title arm.
func buildAddressFormFixture(t *testing.T, root string) addressFormFixture {
	t.Helper()
	card := addCard(t, root, "a card addressed several ways")
	second := addCard(t, root, "a second card for the second join")
	mustRun(t, root, "comment", card, "a note below the card")

	source := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(source, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}
	mustRun(t, root, "attach", card, source)
	mustRun(t, root, "workstream", "new", "Addressing", "--slug", "addressing")
	mustRun(t, root, "set", "done", "title", "In Flight", "--yes")

	dir := resolvedDir(t, benchDir(t, root))
	fixture := addressFormFixture{
		root:             root,
		benchDir:         dir,
		slug:             "fx",
		card:             card,
		cardID:           cardID(t, root, card),
		secondCard:       second,
		columnSlug:       "doing",
		columnTitle:      "In Flight",
		workstreamSlug:   "addressing",
		attachmentName:   "notes.md",
		workbenchDirName: filepath.Base(dir),
	}
	fixture.cardDir = filepath.Join(dir, bench.CardsDir, fixture.cardID)
	fixture.columnSlugDir = columnDirBySlug(t, dir, "doing")
	fixture.columnTitleDir = columnDirBySlug(t, dir, "done")
	fixture.columnIDDir = fixture.columnSlugDir
	fixture.columnIdentifier = filepath.Base(fixture.columnSlugDir)
	fixture.workstreamID = workstreamIDBySlug(t, filepath.Join(dir, bench.WorkstreamsDir), "addressing")
	fixture.workstreamDir = filepath.Join(dir, bench.WorkstreamsDir, fixture.workstreamID)
	fixture.commentDir = soleMemberDir(t, filepath.Join(fixture.cardDir, bench.CommentsDir))
	fixture.commentID = filepath.Base(fixture.commentDir)
	fixture.attachmentDir = soleMemberDir(t, filepath.Join(fixture.cardDir, bench.AttachmentsDir))

	// The values that must not coincide are compared here rather than only
	// being reasoned about above, so that a later edit to the fixture fails
	// loudly instead of quietly disarming a probe.
	for _, columnDir := range []string{fixture.columnSlugDir, fixture.columnTitleDir} {
		if filepath.Base(columnDir) == cardNumberOf(t, card) {
			t.Fatalf("a column's identifier is %q, which is the card-number example, so that probe could be answered by the column lookup standing ahead of the card lookup", cardNumberOf(t, card))
		}
	}
	if strings.EqualFold(fixture.columnTitle, fixture.columnSlug) || strings.EqualFold(fixture.columnTitle, "intake") || strings.EqualFold(fixture.columnTitle, "done") {
		t.Fatalf("the column-title example %q is a column's slug under ASCII lowering, so the slug arm would answer it before the title arm ran", fixture.columnTitle)
	}
	if isAllDigits(fixture.attachmentName) {
		t.Fatalf("the attachment filename %q parses as a number, so pick's position arm would answer the member-name probe", fixture.attachmentName)
	}
	return fixture
}

// soleMemberDir answers the one member directory a collection holds, and fails
// rather than answering an empty string, because a fixture that stopped
// creating the member would otherwise leave the sweep addressing nothing.
func soleMemberDir(t *testing.T, collection string) string {
	t.Helper()
	entries, err := os.ReadDir(collection)
	if err != nil {
		t.Fatalf("read %s: %v", collection, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(collection, entry.Name()))
		}
	}
	if len(dirs) != 1 {
		t.Fatalf("%s holds %d member directories, and this fixture files exactly one", collection, len(dirs))
	}
	return dirs[0]
}

// cardNumberOf answers the number a card's reference ends in, which is the
// card-number example.
func cardNumberOf(t *testing.T, ref string) string {
	t.Helper()
	cut := strings.LastIndex(ref, "-")
	if cut < 0 {
		t.Fatalf("the card reference %q carries no dash, so its number cannot be read off it", ref)
	}
	return ref[cut+1:]
}

// isAllDigits reports whether a string parses as a bare run of digits, which
// is what pick's position arm would catch.
func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// addressFormExample is one generated example: the spelling, the command it is
// run through, and the answer that command must give.
type addressFormExample struct {
	// ref is the spelling being probed.
	ref string
	// wantPath is the file `dinah path` must answer, on a path example.
	wantPath string
	// joinCard is the card a join example joins the workstream to.
	joinCard string
}

// exampleForAddressForm builds one example per declared form from the fixture.
//
// The default arm fails the run, so a form added to the roster with no example
// is a refusal rather than a silent skip.
func exampleForAddressForm(t *testing.T, fixture addressFormFixture, form addressform.AddressForm) addressFormExample {
	t.Helper()
	workbenchAnchor := filepath.Join(fixture.benchDir, bench.WorkbenchAnchor)
	cardAnchor := filepath.Join(fixture.cardDir, bench.CardAnchor)
	switch form {
	case addressform.WorkbenchWord:
		return addressFormExample{ref: "workbench", wantPath: workbenchAnchor}
	case addressform.WorkbenchDot:
		return addressFormExample{ref: ".", wantPath: workbenchAnchor}
	case addressform.WorkbenchSlugHead:
		return addressFormExample{ref: fixture.slug + "/" + bench.AttachmentsDir, wantPath: filepath.Join(fixture.benchDir, bench.AttachmentsDir)}
	case addressform.CardReference:
		return addressFormExample{ref: fixture.card, wantPath: cardAnchor}
	case addressform.CardIdentifier:
		return addressFormExample{ref: fixture.cardID, wantPath: cardAnchor}
	case addressform.CardNumber:
		return addressFormExample{ref: cardNumberOf(t, fixture.card), wantPath: cardAnchor}
	case addressform.CardStalePrefix:
		return addressFormExample{ref: "wasnamedthis-" + cardNumberOf(t, fixture.card), wantPath: cardAnchor}
	case addressform.ColumnSlug:
		return addressFormExample{ref: fixture.columnSlug, wantPath: filepath.Join(fixture.columnSlugDir, bench.ColumnAnchor)}
	case addressform.ColumnTitle:
		return addressFormExample{ref: fixture.columnTitle, wantPath: filepath.Join(fixture.columnTitleDir, bench.ColumnAnchor)}
	case addressform.ColumnIdentifier:
		return addressFormExample{ref: fixture.columnIdentifier, wantPath: filepath.Join(fixture.columnIDDir, bench.ColumnAnchor)}
	case addressform.WorkstreamPrefixedSlug:
		return addressFormExample{ref: bench.WorkstreamRefPrefix + fixture.workstreamSlug, wantPath: fixture.workstreamDir}
	case addressform.WorkstreamPrefixedIdentifier:
		return addressFormExample{ref: bench.WorkstreamRefPrefix + fixture.workstreamID, wantPath: fixture.workstreamDir}
	case addressform.WorkstreamBareSlug:
		return addressFormExample{ref: fixture.workstreamSlug, joinCard: fixture.card}
	case addressform.WorkstreamBareIdentifier:
		return addressFormExample{ref: fixture.workstreamID, joinCard: fixture.secondCard}
	case addressform.MemberPosition:
		return addressFormExample{ref: fixture.card + "/" + bench.CommentsDir + "/1", wantPath: filepath.Join(fixture.commentDir, bench.CommentAnchor)}
	case addressform.MemberIdentifier:
		return addressFormExample{ref: fixture.card + "/" + bench.CommentsDir + "/" + fixture.commentID, wantPath: filepath.Join(fixture.commentDir, bench.CommentAnchor)}
	case addressform.MemberName:
		return addressFormExample{ref: fixture.card + "/" + bench.AttachmentsDir + "/" + fixture.attachmentName, wantPath: filepath.Join(fixture.attachmentDir, bench.AttachmentAnchor)}
	}
	t.Fatalf("the roster declares the form %q and this generator builds no example for it, so nothing probes it", form)
	return addressFormExample{}
}

// TestEveryDeclaredAddressFormResolvesToTheEntityItNames runs one example per
// declared form through the command that declaration names, and asserts both a
// zero exit and that the answer names the fixture entity the example was built
// from.
//
// Asserting the entity rather than merely the exit is what separates the three
// column forms and the two prefixed workstream forms, which the guide guards
// cannot separate because those forms share one sentence. Its stated cost is
// that two spellings reaching one entity through different arms are
// indistinguishable to it, which is why the fixture above is built so that no
// example can be caught by an arm tried ahead of its own.
func TestEveryDeclaredAddressFormResolvesToTheEntityItNames(t *testing.T) {
	roster := addressform.Declarations()
	if len(roster) == 0 {
		t.Fatal("the address-form roster is empty, so this sweep read nothing")
	}
	root := newBench(t)
	fixture := buildAddressFormFixture(t, root)

	probed := 0
	for _, entry := range roster {
		example := exampleForAddressForm(t, fixture, entry.Form)
		switch entry.Command {
		case addressform.CommandPath:
			got := runCLI(t, root, "path", example.ref)
			if got.code != 0 {
				t.Errorf("%s: `dinah path %s` is a spelling the guide teaches and it was refused: %d %s", entry.Form, example.ref, got.code, got.errw)
				continue
			}
			if answered := strings.TrimSpace(got.out); answered != example.wantPath {
				t.Errorf("%s: `dinah path %s` answered %q, and the entity it was built from stands at %q", entry.Form, example.ref, answered, example.wantPath)
				continue
			}
		case addressform.CommandJoin:
			got := runCLI(t, root, "--json", "join", example.joinCard, example.ref)
			if got.code != 0 {
				t.Errorf("%s: `dinah join %s %s` is a spelling the guide teaches and it was refused: %d %s", entry.Form, example.joinCard, example.ref, got.code, got.errw)
				continue
			}
			var payload struct {
				Card struct {
					Workstreams []string `json:"workstreams"`
				} `json:"card"`
			}
			if err := json.Unmarshal([]byte(got.out), &payload); err != nil {
				t.Errorf("%s: decoding the join payload: %v\n%s", entry.Form, err, got.out)
				continue
			}
			if !carriesWorkstream(payload.Card.Workstreams, fixture.workstreamID) {
				t.Errorf("%s: `dinah join %s %s` left the card carrying %v, and the workstream it named is %s", entry.Form, example.joinCard, example.ref, payload.Card.Workstreams, fixture.workstreamID)
				continue
			}
		default:
			t.Errorf("%s declares the command %q, and this guard drives %q and %q", entry.Form, entry.Command, addressform.CommandPath, addressform.CommandJoin)
			continue
		}
		probed++
	}
	if probed == 0 {
		t.Fatal("no declared form was probed, so this sweep proves nothing")
	}
	if probed != len(roster) {
		t.Errorf("%d of the %d declared forms were probed, so this sweep read less than the roster it claims", probed, len(roster))
	}
	t.Logf("%d declared forms probed against the fixture", probed)
}

// carriesWorkstream reports whether a card's membership names a workstream.
func carriesWorkstream(membership []string, want string) bool {
	for _, held := range membership {
		if held == want {
			return true
		}
	}
	return false
}

// exampleForNearMiss builds one refused spelling per declared near miss.
func exampleForNearMiss(t *testing.T, fixture addressFormFixture, key string) string {
	t.Helper()
	switch key {
	case "card-slug-and-identifier":
		return fixture.slug + "-" + fixture.cardID
	case "card-through-its-holder":
		return fixture.slug + "/" + bench.CardsDir + "/" + cardNumberOf(t, fixture.card)
	case "workbench-bare-slug":
		return fixture.slug
	case "workbench-directory-name":
		return fixture.workbenchDirName
	case "column-with-a-kind-prefix":
		return "column/" + fixture.columnSlug
	case "workstream-bare-handle-as-an-address":
		return fixture.workstreamSlug
	}
	t.Fatalf("the near-miss roster declares %q and this generator builds no example for it, so nothing probes it", key)
	return ""
}

// TestEveryDeclaredNearMissRefusesWithItsDeclaredRefusal runs one example per
// near miss through the command that row names and asserts the refusal name
// its machine payload carries.
//
// The refusal name is asserted rather than merely a non-zero exit, because a
// resolver that had started refusing everything would pass a check that read
// only the exit code. The accepting case stands beside the refusing one for
// the same reason: workstream-bare-handle-as-an-address and the
// WorkstreamBareSlug head form are one spelling, refused through path and
// accepted through join.
func TestEveryDeclaredNearMissRefusesWithItsDeclaredRefusal(t *testing.T) {
	roster := addressform.NearMisses()
	if len(roster) == 0 {
		t.Fatal("the near-miss roster is empty, so this sweep read nothing")
	}
	root := newBench(t)
	fixture := buildAddressFormFixture(t, root)

	probed := 0
	for _, entry := range roster {
		if entry.Command != addressform.CommandPath {
			t.Errorf("the near miss %q declares the command %q, and this guard drives %q", entry.Key, entry.Command, addressform.CommandPath)
			continue
		}
		ref := exampleForNearMiss(t, fixture, entry.Key)
		got := runCLI(t, root, "--json", "path", ref)
		if got.code == 0 {
			t.Errorf("%s: `dinah path %s` is declared refused and it exited 0: %s", entry.Key, ref, got.out)
			continue
		}
		var payload struct {
			Refusal string `json:"refusal"`
		}
		if err := json.Unmarshal([]byte(got.out), &payload); err != nil {
			t.Errorf("%s: decoding the refusal payload: %v\n%s", entry.Key, err, got.out)
			continue
		}
		if payload.Refusal != entry.Refusal {
			t.Errorf("%s: `dinah path %s` refused %q, and the roster declares %q", entry.Key, ref, payload.Refusal, entry.Refusal)
			continue
		}
		probed++
	}
	if probed == 0 {
		t.Fatal("no declared near miss was probed, so this sweep proves nothing")
	}
	if probed != len(roster) {
		t.Errorf("%d of the %d declared near misses were probed, so this sweep read less than the roster it claims", probed, len(roster))
	}
	t.Logf("%d declared near misses probed against the fixture", probed)
}

// TestANonCanonicalNumberSpellingAnswersWithTheReferenceDinahWrites holds the
// tolerated spellings to answering with the reference Dinah writes back.
//
// strconv.Atoi accepts a leading zero, splitRef cuts at the last dash so any
// prefix is a prefix, and the reference is trimmed before anything else runs,
// so all four of these resolve. Dinah writes none of them back, and that is
// the property this pins: what a reader is shown is unchanged by which
// spelling was read. The four are named rather than counted, because a
// fixture that quietly stopped running one proves nothing about it.
func TestANonCanonicalNumberSpellingAnswersWithTheReferenceDinahWrites(t *testing.T) {
	root := newBench(t)
	addCard(t, root, "a card named several careless ways")

	// The expected reference is composed from what stands on disk rather than
	// from what the tool answered, because what the tool answers is the
	// subject of this check. Reading it back off Card.Ref would compare the
	// composition against itself, and every plant on that composition would
	// then move both sides together and prove nothing.
	card := workbenchSlugOnDisk(t, root) + "-" + firstCardNumberOnDisk(t, root)

	spellings := []string{"fx-01", "fx--1", "FX-1", " fx-1 "}
	ran := 0
	for _, spelling := range spellings {
		got := runCLI(t, root, "--json", "show", spelling)
		if got.code != 0 {
			t.Errorf("`dinah show %q` is a tolerated spelling and it was refused: %d %s", spelling, got.code, got.errw)
			continue
		}
		var payload struct {
			Card struct {
				Ref string `json:"ref"`
			} `json:"card"`
		}
		if err := json.Unmarshal([]byte(got.out), &payload); err != nil {
			t.Errorf("decoding the show payload for %q: %v\n%s", spelling, err, got.out)
			continue
		}
		if payload.Card.Ref != card {
			t.Errorf("`dinah show %q` answered the reference %q, and Dinah writes %q whichever spelling was read", spelling, payload.Card.Ref, card)
			continue
		}
		ran++
	}
	if ran != len(spellings) {
		t.Fatalf("%d of the %d named spellings were run, so this check read less than it claims", ran, len(spellings))
	}
	t.Logf("%d tolerated spellings each answered with %s", ran, card)
}

// workbenchSlugOnDisk reads the workbench's slug out of its own anchor, so the
// expectation above comes from the file rather than from the composition it is
// holding to account.
func workbenchSlugOnDisk(t *testing.T, root string) string {
	t.Helper()
	dir := resolvedDir(t, benchDir(t, root))
	text, err := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, _ := bench.ParseAnchor(text)
	slug := fm.Value("slug")
	if slug == "" {
		t.Fatal("the workbench anchor carries no slug, so the reference this check expects cannot be composed")
	}
	return slug
}

// firstCardNumberOnDisk reads the number off the one card the fixture files,
// for the same reason.
func firstCardNumberOnDisk(t *testing.T, root string) string {
	t.Helper()
	dir := resolvedDir(t, benchDir(t, root))
	cardDir := soleMemberDir(t, filepath.Join(dir, bench.CardsDir))
	text, err := bench.ReadText(filepath.Join(cardDir, bench.CardAnchor))
	if err != nil {
		t.Fatalf("read the card anchor: %v", err)
	}
	fm, _ := bench.ParseAnchor(text)
	number := fm.Value("number")
	if number == "" {
		t.Fatal("the card anchor carries no number, so the reference this check expects cannot be composed")
	}
	return number
}
