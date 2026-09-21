package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
)

// runQuiet drives the binary through runCLI, asking for the JSON answer, which
// is what keeps a case that plants a line ending in a title off the table
// renderer.
//
// The reason is not cosmetic. A table cell carrying a line feed wraps onto a
// second physical line, and runCLI's own alignment check reads that as a column
// starting in two places. That is the renderer meeting prose no renderer can
// lay out on one row, which is the condition this card exists to stop reaching
// the store rather than anything these cases assert about; what they assert
// about is the bytes, which they read off disk.
func runQuiet(t *testing.T, dir string, argv ...string) invocation {
	t.Helper()
	return runCLI(t, dir, append([]string{"--json"}, argv...)...)
}

// storeRoot answers the workbench directory inside a fixture's own directory,
// which is where the format puts one: the immediate child of a .dinah
// container, under a name Dinah itself minted.
func storeRoot(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, bench.UserBaseName))
	if err != nil {
		t.Fatalf("read container: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(dir, bench.UserBaseName, entry.Name())
		}
	}
	t.Fatalf("no workbench under %s", dir)
	return ""
}

// crlf is what every slot below is driven with. It is spelled once so that a
// case cannot quietly drive an LF and pass.
const crlf = "\r\n"

// walkStore answers every file under a root outside a payload directory,
// together with its bytes, read with os.ReadFile rather than through
// bench.ReadText, which now normalises and would strip the very condition
// these cases exist to see.
func walkStore(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == bench.PayloadDir {
				return filepath.SkipDir
			}
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		files[path] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return files
}

// storedLineEnding reports the first stored form of a line-ending carriage
// return a file carries, and the empty string where it carries none.
//
// Asserting the absence of 0x0D alone is not enough and was an earlier defect
// here: a journal stores a carriage return as an escape and carries no 0x0D at
// all, and the split frontmatter form carries a 0x0D that no search for a CRLF
// pair finds. Asserting only the two-character backslash-r spelling is not
// enough either, and was the defect after that one, because JSON admits a
// six-character unicode escape for the same code point. So the journal half of
// this decodes each string literal and tests the decoded value rather than
// testing bytes.
func storedLineEnding(path string, data []byte) string {
	if filepath.Ext(path) == ".ndjson" {
		for index, record := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(record) == "" {
				continue
			}
			if strings.Contains(strings.TrimSuffix(record, "\r"), "\r") {
				return "a raw carriage return inside record " + itoa(index+1)
			}
			if strings.HasSuffix(record, "\r") {
				return "records separated by CRLF at record " + itoa(index+1)
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(record), &decoded); err != nil {
				continue
			}
			if where := decodedCarries(decoded); where != "" {
				return "an encoded CRLF in record " + itoa(index+1) + ", at " + where
			}
		}
		return ""
	}
	if i := strings.Index(string(data), "\r\n"); i >= 0 {
		return "a CRLF pair"
	}
	if i := strings.Index(string(data), "\r\\n"); i >= 0 {
		return "the split frontmatter form"
	}
	return ""
}

// decodedCarries answers the member path at which a decoded record carries a
// CRLF, and the empty string where none does.
func decodedCarries(value any) string {
	switch typed := value.(type) {
	case string:
		if strings.Contains(typed, crlf) {
			return "a string"
		}
	case []any:
		for i, element := range typed {
			if where := decodedCarries(element); where != "" {
				return itoa(i) + "." + where
			}
		}
	case map[string]any:
		for name, element := range typed {
			if where := decodedCarries(element); where != "" {
				return name + "." + where
			}
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// newlineSite is one place the counting rule names.
type newlineSite struct {
	// Name is the site, spelled as the counting rule spells it.
	Name string
	// WindowsRefuses marks the one site whose slot is a name the operating
	// system itself will not accept on Windows, so the sweep records the
	// platform's refusal there instead of a drive.
	WindowsRefuses bool
}

// newlineSites is the counting rule's answer written down once: the
// twenty-three places a byte sequence originating outside the Dinah process is
// stored in a workbench, or is served as a workbench's own text.
//
// Every site is driven by the one case below, and the accounting is a set of
// names that case records as it drives them. There is no delegation to another
// test, and that is deliberate rather than tidy. The first version of this
// table discharged five sites with a sentence naming the test that covered
// them, and two of those five sentences were false: one named a test that
// passed no --operator at all, and one named a test whose init --from never
// reaches the function the site is about. Prose in a table is not an assertion,
// and a site discharged by prose is a site nothing drives. Anything this case
// cannot drive is either marked as the platform refusing it or is not in the
// table.
//
// What the table still cannot do is read the specification. The counting rule
// is prose there, so this list is its transcription and a twenty-fifth site
// nobody transcribes reddens nothing. Two independent walks of the request
// surface have now produced twenty-four, which is the only real check on it.
//
// Request.Note left the table at dinah-525, which stopped it holding prose.
// The three terminal checklist verbs read their answer out of it, and what
// they read is a reference to a comment of the item rather than a caller's
// own sentence, so the field is one line by construction and the words that
// used to reach it now reach Request.Text, which the table already carries.
var newlineSites = []newlineSite{
	// Group A, the fifteen verb.Request fields that store caller prose. One
	// field is one site whatever filled it, because the CLI argument, the
	// standard-input sentinel and the MCP argument are one field wearing three
	// hats and a fix at the field covers all three.
	{Name: "Request.Title"},
	{Name: "Request.Text"},
	{Name: "Request.Value"},
	{Name: "Request.Reason"},
	{Name: "Request.Description"},
	{Name: "Request.Kind"},
	{Name: "Request.Owner"},
	{Name: "Request.Scheme"},
	{Name: "Request.CiteTarget"},
	{Name: "Request.Column"},
	{Name: "Request.Workstream"},
	{Name: "Request.Actor"},
	{Name: "Request.Provider"},
	{Name: "Request.Model"},
	{Name: "Request.Server"},
	// Group B, text reaching workbench files without passing a request.
	{Name: "B1 dinah edit"},
	{Name: "B2 a definition document"},
	{Name: "B3 the verbatim anchor copy dinah extract makes"},
	{Name: "B4 an attachment's payload bytes"},
	{Name: "B5 an attachment's stored filename on attach"},
	{Name: "B6 dinah init --operator"},
	{Name: "B7 the workbench title a bare init takes from its own directory name", WindowsRefuses: true},
	// Group C, served but never stored.
	{Name: "C1 the user-global instruction layer"},
}

// dirtyValues are what every site is driven with. Each carries a line ending
// the store must not keep, and the doubled one is there because a single
// non-overlapping replacement of the pair left a fresh pair behind and the
// writer stored it.
func dirtyValues(stem string) []string {
	return []string{stem + "-a" + crlf + stem + "-b", stem + "-c" + crlf + crlf + stem + "-d"}
}

// TestTheInvariantHoldsAcrossTheWholeVerbSurface drives a value carrying a line
// ending into every site the counting rule names, and asserts after each drive
// that no file of the store carries any stored form of one.
//
// Every value carries a doubled carriage return as well as a single one,
// because a pass that reduced one pair at a time left a fresh pair behind and
// the writer stored it.
//
// The two exempt sites are driven too, with the opposite assertion, so that the
// exemption is armed rather than assumed, and the sweep states how many sites
// it drove and how many files it walked, failing if either is zero.
func TestTheInvariantHoldsAcrossTheWholeVerbSurface(t *testing.T) {
	root := newBench(t)
	stores := []string{root}
	drove := map[string]bool{}
	mark := func(names ...string) {
		t.Helper()
		for _, name := range names {
			if drove[name] {
				t.Errorf("%s is driven twice", name)
			}
			drove[name] = true
		}
	}
	// Each drive is checked where it lands rather than only at the end of the
	// case. A later verb reads an anchor through ParseAnchor and renders it
	// back, and that read strips a trailing carriage return per line, so one
	// verb's damage is cleaned by the next verb that touches the same file. A
	// sweep that walked the store only once at the end would call such a site
	// covered while the site stored a carriage return at the moment it was
	// driven. Value was exactly that: set wrote a dirty body and the block
	// that followed it rewrote the anchor clean.
	drive := func(sites []string, argv ...string) {
		t.Helper()
		mark(sites...)
		if got := runQuiet(t, root, argv...); got.code != 0 {
			t.Fatalf("%v: %d %s", sites, got.code, got.errw)
		}
		for path, data := range walkStore(t, root) {
			if where := storedLineEnding(path, data); where != "" {
				t.Errorf("driving %v left %s carrying %s", sites, path, where)
			}
		}
	}
	setup := func(argv ...string) {
		t.Helper()
		if got := runQuiet(t, root, argv...); got.code != 0 {
			t.Fatalf("setup %v: %d %s", argv, got.code, got.errw)
		}
	}
	// Every value carries a doubled carriage return as well as a single one,
	// so a pass that reduces only one pair per run stores the other.
	dirty := func(stem string) string { return strings.Join(dirtyValues(stem), crlf+crlf) }

	drive([]string{"Request.Title"}, "add", dirty("add"))
	setup("move", "fx-1", "doing")
	drive([]string{"Request.Value"}, "set", "fx-1", "body", dirty("body"))
	drive([]string{"Request.Text"}, "comment", "fx-1", dirty("comment"))
	drive([]string{"Request.Owner"}, "file", "fx-1", "open_question", "an open question", "--owner", dirty("own"))
	setup("file", "fx-1", "acceptance_criterion", "a criterion")
	drive([]string{"Request.Scheme", "Request.CiteTarget"}, "cite", "fx-1/criteria/1", dirty("sch"), dirty("tgt"))
	drive([]string{"Request.Reason", "Request.Kind"}, "block", "fx-1", dirty("reason"), "--kind", dirty("kind"))
	drive([]string{"Request.Column"}, "column", "new", dirty("col"))
	drive([]string{"Request.Workstream"}, "workstream", "new", dirty("ws"))
	drive([]string{"Request.Actor"}, "comment", "fx-1", "a comment under a dirty actor", "--actor", dirty("act"))

	// Provider, Model and Server arrive from the environment rather than from
	// an argument, and land in the actor block of every journal line.
	t.Setenv("DINAH_PROVIDER", dirty("prov"))
	t.Setenv("DINAH_MODEL", dirty("mod"))
	t.Setenv("DINAH_SERVER", dirty("srv"))
	drive([]string{"Request.Provider", "Request.Model", "Request.Server"}, "comment", "fx-1", "a comment under a dirty agent")
	t.Setenv("DINAH_PROVIDER", "")
	t.Setenv("DINAH_MODEL", "")
	t.Setenv("DINAH_SERVER", "")

	payload := "payload-a" + crlf + "payload-b" + crlf + crlf + "payload-c" + crlf
	source := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(source, []byte(payload), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	// The attach carries the description, which is normalised, and brings the
	// two exempt sites with it, which the assertions below hold to the opposite
	// rule so the exemption is armed rather than assumed.
	drive([]string{"Request.Description", "B4 an attachment's payload bytes", "B5 an attachment's stored filename on attach"},
		"attach", "fx-1", source, "--description", dirty("desc"))

	// The stored filename is the payload file's own name on disk, and a
	// filename carrying a line feed is legal on Linux and on macOS and refused
	// by Windows. This is a second route into Value rather than a site of its
	// own, because one Request field is one site whatever filled it.
	if runtime.GOOS != "windows" {
		if got := runQuiet(t, root, "rename", "fx-1/attachments/1", dirty("ren")); got.code != 0 {
			t.Fatalf("rename: %d %s", got.code, got.errw)
		}
	}

	// B1, an external editor writing an anchor, which no write-path change can
	// reach. It is driven by planting what such an editor would leave and
	// asserting that the check reports it and the repair clears it, which is
	// the whole of this card's answer to the site.
	editor := newBench(t)
	stores = append(stores, editor)
	mark("B1 dinah edit")
	editorAnchor := filepath.Join(storeRoot(t, editor), bench.WorkbenchAnchor)
	planted, err := os.ReadFile(editorAnchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	if err := os.WriteFile(editorAnchor, []byte(strings.ReplaceAll(string(planted), "\n", crlf)), 0o644); err != nil {
		t.Fatalf("plant an editor's bytes: %v", err)
	}
	if got := runCLI(t, editor, "check"); !strings.Contains(got.out, "carriage return") {
		t.Errorf("the check does not report what an external editor wrote:\n%s", got.out)
	}
	if got := runCLI(t, editor, "check", "--migrate-newlines", "--yes"); got.code != 0 {
		t.Errorf("the repair of an editor's bytes exited %d:\n%s", got.code, got.out)
	}

	// B2, a definition document, and B6, the operator a bare init records,
	// which is not a Request field at all. One init drives both.
	imported := filepath.Join(t.TempDir(), "imported")
	if err := os.MkdirAll(imported, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	document := filepath.Join(t.TempDir(), "definition.json")
	body := `{"profile":"dinah-core/0.7","title":"Imported","slug":"im","instructions":"stand-a\r\nstand-b\r\r\nstand-c","columns":[{"id":"b00000000001","title":"Only","kind":"work","slug":"only","instructions":"inst-a\r\r\ninst-b"}]}`
	if err := os.WriteFile(document, []byte(body), 0o644); err != nil {
		t.Fatalf("write the definition: %v", err)
	}
	mark("B2 a definition document", "B6 dinah init --operator")
	if got := runQuiet(t, imported, "init", "--from", document, "--operator", dirty("op")); got.code != 0 {
		t.Fatalf("init --from: %d %s", got.code, got.errw)
	}
	stores = append(stores, imported)

	// B3, the verbatim anchor copy dinah extract makes, which reads each
	// anchor with ReadText and writes it with WriteText. It is driven against
	// a store whose column anchor an editor has dirtied, so the copy has
	// something to normalise.
	donor := newBench(t)
	stores = append(stores, donor)
	var donorColumn string
	filepath.Walk(filepath.Join(storeRoot(t, donor), bench.ColumnsDir), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(path) == bench.ColumnAnchor {
			donorColumn = path
		}
		return nil
	})
	if donorColumn == "" {
		t.Fatal("the donor store carries no column anchor")
	}
	donorBytes, err := os.ReadFile(donorColumn)
	if err != nil {
		t.Fatalf("read the donor column: %v", err)
	}
	if err := os.WriteFile(donorColumn, []byte(strings.ReplaceAll(string(donorBytes), "\n", crlf)), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	extracted := filepath.Join(t.TempDir(), "extracted")
	mark("B3 the verbatim anchor copy dinah extract makes")
	if got := runQuiet(t, donor, "extract", extracted); got.code != 0 {
		t.Fatalf("extract: %d %s", got.code, got.errw)
	}
	for path, data := range walkStore(t, extracted) {
		if where := storedLineEnding(path, data); where != "" {
			t.Errorf("the extracted copy of %s carries %s", path, where)
		}
	}
	// The donor is repaired so the walk below reads a store this card's own
	// write path produced rather than the editor's plant.
	if got := runCLI(t, donor, "check", "--migrate-newlines", "--yes"); got.code != 0 {
		t.Errorf("repairing the donor exited %d:\n%s", got.code, got.out)
	}

	// B7, the workbench title a bare init takes from the directory it runs in.
	if runtime.GOOS == "windows" {
		mark("B7 the workbench title a bare init takes from its own directory name")
		t.Log("B7 records the platform's refusal rather than a drive: Windows will not accept a directory name carrying a line ending, and Linux and macOS do")
	} else {
		named := filepath.Join(t.TempDir(), "wb-a"+crlf+"wb-b")
		if err := os.MkdirAll(named, 0o755); err != nil {
			t.Fatalf("%s refuses a directory name carrying a line ending, which the counting rule says it accepts: %v", runtime.GOOS, err)
		}
		mark("B7 the workbench title a bare init takes from its own directory name")
		if got := runQuiet(t, named, "init", "--slug", "nm"); got.code != 0 {
			t.Fatalf("bare init: %d %s", got.code, got.errw)
		}
		stores = append(stores, named)
	}

	// C1, the user-global instruction layer, which is normalised on read
	// rather than on write and is served rather than stored.
	mark("C1 the user-global instruction layer")
	home := t.TempDir()
	base := bench.UserBase(home)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir the user base: %v", err)
	}
	prose := "one\ntwo\nthree\n"
	// The middle line ends with a doubled carriage return, so a pass that
	// reduces one pair at a time serves a carriage return the layer's own
	// revision would then differ over.
	if err := os.WriteFile(filepath.Join(base, bench.InstructionsName), []byte("one"+crlf+"two\r"+crlf+"three"+crlf), 0o644); err != nil {
		t.Fatalf("write the user-global instructions: %v", err)
	}
	if served := bench.GlobalInstructions(home); strings.Contains(served, "\r") {
		t.Errorf("the user-global layer serves %q", served)
	} else if bench.TextRevision(served) != bench.TextRevision(prose) {
		t.Errorf("the layer's revision is %s and the same prose in LF is %s", bench.TextRevision(served), bench.TextRevision(prose))
	}

	// Every site the table names is driven exactly once, and nothing is driven
	// that the table does not name.
	for _, site := range newlineSites {
		if !drove[site.Name] {
			t.Errorf("%s is named by the counting rule and driven by nothing", site.Name)
		}
		delete(drove, site.Name)
	}
	for name := range drove {
		t.Errorf("%s was driven and the counting rule does not name it", name)
	}
	if want := 23; len(newlineSites) != want {
		t.Errorf("the table carries %d sites and the counting rule produces %d", len(newlineSites), want)
	}
	t.Logf("the counting rule names %d sites and this case drove all of them across %d stores", len(newlineSites), len(stores))

	walked := 0
	for _, store := range stores {
		files := walkStore(t, store)
		walked += len(files)
		for path, data := range files {
			if where := storedLineEnding(path, data); where != "" {
				t.Errorf("%s carries %s", path, where)
			}
		}
	}
	if walked == 0 {
		t.Fatal("the sweep walked no file")
	}
	t.Logf("the sweep walked %d files", walked)

	// The exemption, armed rather than assumed: the payload's bytes are the
	// source file's bytes, exactly.
	var stored string
	filepath.Walk(filepath.Join(storeRoot(t, root), bench.CardsDir), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(filepath.Dir(path)) == bench.PayloadDir {
			stored = path
		}
		return nil
	})
	if stored == "" {
		t.Fatal("no payload file was found, so the exemption is not armed")
	}
	storedBytes, err := os.ReadFile(stored)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if string(storedBytes) != payload {
		t.Errorf("the payload reads %q and was written as %q", storedBytes, payload)
	}
}

// TestTheFixReachesEveryFrontmatterBorneSlot is the criterion a normalisation
// inside WriteText could not pass. The frontmatter writer escapes a line feed
// into two characters and leaves a carriage return raw, so a value carrying
// CRLF that reached it unnormalised is stored as a carriage return followed by
// that escape and carries no CRLF pair for any search to find.
//
// Each slot is read back through dinah get as well, because unquote restores
// the escaped line feed behind the stored carriage return and hands it straight
// to the caller.
func TestTheFixReachesEveryFrontmatterBorneSlot(t *testing.T) {
	root := newBench(t)
	dirty := func(a, b string) string { return a + crlf + b }
	step := func(name string, argv ...string) {
		t.Helper()
		if got := runQuiet(t, root, argv...); got.code != 0 {
			t.Fatalf("%s: %d %s", name, got.code, got.errw)
		}
	}
	read := func(ref, field string) string {
		t.Helper()
		got := runCLI(t, root, "get", ref, field)
		if got.code != 0 {
			t.Fatalf("get %s %s: %d %s", ref, field, got.code, got.errw)
		}
		return got.out
	}

	step("add", "add", dirty("add-a", "add-b"))
	step("move", "move", "fx-1", "doing")
	step("block", "block", "fx-1", dirty("reason-a", "reason-b"), "--kind", dirty("kind-a", "kind-b"))
	step("unblock", "unblock", "fx-1", "--actor", "alka")
	step("claim", "claim", "fx-1", "--actor", dirty("act-a", "act-b"))
	step("file", "file", "fx-1", "acceptance_criterion", "a criterion", "--owner", dirty("own-a", "own-b"))
	step("cite", "cite", "fx-1/criteria/1", dirty("sch-a", "sch-b"), dirty("tgt-a", "tgt-b"))
	step("verify", "verify", "fx-1/criteria/1", "--text", dirty("note-a", "note-b"))
	source := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	step("attach", "attach", "fx-1", source, "--description", dirty("desc-a", "desc-b"))
	step("column new", "column", "new", dirty("col-a", "col-b"))
	step("workstream new", "workstream", "new", dirty("ws-a", "ws-b"))

	for path, data := range walkStore(t, root) {
		if strings.Contains(string(data), "\r") {
			t.Errorf("%s carries a carriage return: %q", path, string(data))
		}
	}
	for _, c := range []struct{ ref, field string }{
		{"fx-1", "title"},
		{"fx-1/criteria/1", "owner"},
		{"fx-1/criteria/1", "resolution"},
		{"fx-1/attachments/1", "description"},
	} {
		if value := read(c.ref, c.field); strings.Contains(value, "\r") {
			t.Errorf("%s %s reads back carrying a carriage return: %q", c.ref, c.field, value)
		}
	}
	if runtime.GOOS == "windows" {
		t.Log("B7, the workbench title a bare init takes from its own directory name, is skipped: Windows refuses a directory name carrying a carriage return, and Linux and macOS admit one")
	}
}

// TestABodyWrittenThroughStandardInputLandsAsLF is the card's own defect. The
// four column anchors it diagnosed were written exactly this way.
func TestABodyWrittenThroughStandardInputLandsAsLF(t *testing.T) {
	root := newBench(t)
	body := "one" + crlf + "two" + crlf + "three" + crlf
	got := runCLIWithInput(t, root, strings.NewReader(body), "set", "doing", "instructions", "-")
	if got.code != 0 {
		t.Fatalf("set: %d %s", got.code, got.errw)
	}
	anchor := filepath.Join(root, bench.ColumnsDir)
	var stored string
	filepath.Walk(anchor, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.Contains(path, "doing") {
			stored = path
		}
		return nil
	})
	for path, data := range walkStore(t, root) {
		if strings.Contains(string(data), "\r") {
			t.Errorf("%s carries a carriage return: %q", path, data)
		}
	}
	_ = stored
}

// TestAGetSetRoundTripChangesOnlyWhatWasEdited holds all three clauses of the
// round trip: the call answers ok, the anchor's bytes are byte-identical to
// before, and no line is appended to the journal.
//
// The third clause is the one that fails against a normalisation living below
// writeField's comparison. That comparison reads the parsed value, which
// carries no carriage return, against the raw request value, finds them
// unequal, and appends an updated line whose from and to are the same prose.
func TestAGetSetRoundTripChangesOnlyWhatWasEdited(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "set", "doing", "instructions", "one\ntwo\nthree"); got.code != 0 {
		t.Fatalf("set: %d %s", got.code, got.errw)
	}
	before := walkStore(t, root)
	read := runCLI(t, root, "get", "doing", "instructions")
	if read.code != 0 {
		t.Fatalf("get: %d %s", read.code, read.errw)
	}
	crlfed := strings.ReplaceAll(strings.TrimSuffix(read.out, "\n"), "\n", crlf)
	again := runCLIWithInput(t, root, strings.NewReader(crlfed), "set", "doing", "instructions", "-")
	if again.code != 0 {
		t.Fatalf("set again: %d %s", again.code, again.errw)
	}
	after := walkStore(t, root)
	for path, data := range after {
		was, carried := before[path]
		if !carried {
			t.Errorf("%s did not exist before the round trip", path)
			continue
		}
		if string(was) != string(data) {
			t.Errorf("%s changed under a round trip:\n was %q\n now %q", path, was, data)
		}
	}
}

// TestThreeReadersOfOneFieldAgree is the disagreement the card did not name.
// Against a column anchor whose stored bytes carry CRLF, show, get and
// instructions returned three answers, because show returns raw read output
// with no anchor parse between while the other two strip.
func TestThreeReadersOfOneFieldAgree(t *testing.T) {
	root := newBench(t)
	var anchor string
	filepath.Walk(filepath.Join(storeRoot(t, root), bench.ColumnsDir), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Base(path) != bench.ColumnAnchor {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if fm, _ := bench.ParseAnchor(string(data)); fm.Value("slug") == "doing" {
			anchor = path
		}
		return nil
	})
	if anchor == "" {
		t.Fatal("the fixture carries no doing column anchor")
	}
	if err := os.WriteFile(anchor, []byte("---\ntitle: Doing\nslug: doing\nkind: work\n---\none\r\ntwo\r\n"), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	countCR := func(s string) int { return strings.Count(s, "\r") }
	show := runCLI(t, root, "show", "doing")
	get := runCLI(t, root, "get", "doing", "instructions")
	served := runCLI(t, root, "instructions", "doing")
	for _, c := range []struct {
		name string
		out  string
	}{{"show", show.out}, {"get", get.out}, {"instructions", served.out}} {
		if n := countCR(c.out); n != 0 {
			t.Errorf("%s returns %d carriage returns, and the three readers of one field have to agree", c.name, n)
		}
	}

	// The check reports the file whatever the readers say, because the sweep
	// reads the bytes rather than going through the normalising reader.
	report := runCLI(t, root, "check")
	if !strings.Contains(report.out, "carriage return") {
		t.Errorf("check did not report the planted file:\n%s", report.out)
	}
}

// TestTheUserGlobalLayerServesLFAndItsRevisionMatches is site C1, the one site
// normalised on read rather than on write. A file authored on Windows served
// CRLF into the instruction chain, and the revision over that text differed
// from the revision of the same prose in LF, so one layer had two identities
// depending on which machine wrote the file.
func TestTheUserGlobalLayerServesLFAndItsRevisionMatches(t *testing.T) {
	home := t.TempDir()
	base := bench.UserBase(home)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prose := "one\ntwo\nthree\n"
	if err := os.WriteFile(filepath.Join(base, bench.InstructionsName), []byte(strings.ReplaceAll(prose, "\n", crlf)), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	served := bench.GlobalInstructions(home)
	if strings.Contains(served, "\r") {
		t.Errorf("the layer serves %q", served)
	}
	if got, want := bench.TextRevision(served), bench.TextRevision(prose); got != want {
		t.Errorf("the layer's revision is %s and the same prose in LF is %s", got, want)
	}
}

// TestADefinitionDocumentReachesItsAnchorsAsLF is site B2, driven end to end
// through dinah init --from rather than through the reader alone, because what
// this site costs is what lands in the anchors the import writes.
//
// Three things beyond the sweep, each a place this site has already gone wrong:
// the unrecognised member survives as a member in its rendered block shape
// rather than being dropped or turned into a raw JSON line, a lone carriage
// return inside a document's string survives into the anchor, and a member NAME
// carrying a line ending is refused rather than normalised.
func TestADefinitionDocumentReachesItsAnchorsAsLF(t *testing.T) {
	base := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")

	document := filepath.Join(base, "definition.json")
	body := `{"profile":"dinah-core/0.7","title":"Imported","slug":"im","instructions":"stand-a\r\nstand-b","odd":{"deep":"odd-a\r\nodd-b"},"loose":"one\ry","columns":[{"id":"b00000000001","title":"Col-a\r\nCol-b","kind":"work","slug":"only","instructions":"inst-a\r\ninst-b","require_fields":["git.branch"]}]}`
	if err := os.WriteFile(document, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	root := filepath.Join(base, "workbench")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "init", "--from", document); got.code != 0 {
		t.Fatalf("init --from: %d %s", got.code, got.errw)
	}
	loose := false
	for path, data := range walkStore(t, root) {
		if filepath.Ext(path) != ".md" {
			continue
		}
		if strings.Contains(string(data), crlf) || strings.Contains(string(data), "\r\\n") {
			t.Errorf("%s carries a stored line ending: %q", path, data)
		}
		if strings.Contains(string(data), "\r") {
			loose = true
		}
	}
	if !loose {
		t.Error("the lone carriage return inside the document's own string did not survive into an anchor")
	}
	anchor, err := os.ReadFile(filepath.Join(storeRoot(t, root), bench.WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read anchor: %v", err)
	}
	if !strings.Contains(string(anchor), "odd:") {
		t.Errorf("the unrecognised member was dropped:\n%s", anchor)
	}
	if !strings.Contains(string(anchor), "deep:") {
		t.Errorf("the unrecognised member was stored as a raw JSON line rather than in its rendered block shape:\n%s", anchor)
	}

	// The refusal, and the accepting case beside it, so a build refusing
	// every document cannot pass.
	for _, ending := range []string{`\r\n`, `\n`, `\r`} {
		refused := filepath.Join(base, "refused.json")
		name := `odd` + ending + `member`
		if err := os.WriteFile(refused, []byte(`{"profile":"dinah-core/0.7","title":"Refused","slug":"rf","`+name+`":1,"columns":[{"id":"b00000000001","title":"Only","kind":"work"}]}`), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		destination := filepath.Join(base, "refused-"+strings.Trim(ending, `\`))
		if err := os.MkdirAll(destination, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		// One iteration's trimmed ending can carry a literal backslash (the
		// \r\n case trims only its leading slash), which filepath.Join reads
		// as a separator on this platform and leaves a stray subdirectory
		// behind under a sibling iteration's own destination; --here is what
		// this loop is not testing, so it is passed unconditionally rather
		// than special-cased per ending.
		got := runCLI(t, destination, "init", "--from", refused, "--here")
		if got.code == 0 {
			t.Errorf("a member name carrying %s was admitted", ending)
			continue
		}
		if !strings.Contains(got.errw, "malformed-member-name") {
			t.Errorf("a member name carrying %s was refused with %q", ending, got.errw)
		}
		if _, err := os.Stat(filepath.Join(destination, bench.UserBaseName)); err == nil {
			t.Errorf("a refused document left a workbench behind at %s", destination)
		}
	}
}

// TestARenamedAttachmentAndItsPayloadAgree guards the one place where
// normalising a stored value alone would leave an anchor naming a file that is
// not there, which is why the normalisation for this field sits inside
// RenameAttachment rather than at the anchor write.
func TestARenamedAttachmentAndItsPayloadAgree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows refuses a filename carrying a line feed, and this case is about the anchor and the file on disk agreeing on one")
	}
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	carryToDoing(t, root, "fx-1")
	cards := filepath.Join(storeRoot(t, root), bench.CardsDir)
	source := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := runCLI(t, root, "attach", "fx-1", source); got.code != 0 {
		t.Fatalf("attach: %d %s", got.code, got.errw)
	}
	if got := runQuiet(t, root, "rename", "fx-1/attachments/1", "ren-a"+crlf+"ren-b"); got.code != 0 {
		t.Fatalf("rename: %d %s", got.code, got.errw)
	}
	var anchor, payload string
	filepath.Walk(cards, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) == bench.AttachmentAnchor {
			anchor = path
		}
		if filepath.Base(filepath.Dir(path)) == bench.PayloadDir {
			payload = path
		}
		return nil
	})
	if anchor == "" || payload == "" {
		t.Fatal("the attachment was not written")
	}
	data, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read anchor: %v", err)
	}
	if strings.Contains(string(data), "\r") {
		t.Errorf("the anchor's filename key carries a carriage return: %q", data)
	}
	entries, err := os.ReadDir(filepath.Dir(payload))
	if err != nil {
		t.Fatalf("read payload dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the payload directory holds %d files", len(entries))
	}
	fm, _ := bench.ParseAnchor(string(data))
	if got, want := fm.Value("filename"), entries[0].Name(); got != want {
		t.Errorf("the anchor names %q and the file on disk is %q", got, want)
	}
}

// TestTheNewlineRepairReadsAsItsOwnAccount drives the repair through the
// binary, in every shape its report takes: a preview, a confirmed run, a
// second run with nothing left to do, a run that met a busy file, and a plan a
// refusal stopped.
//
// It is the criterion the preview's own heading answers to. A reader meets the
// sentence saying that each destination was rewritten with its own bytes and
// that its modification time is now before running the command, rather than
// discovering it afterwards.
func TestTheNewlineRepairReadsAsItsOwnAccount(t *testing.T) {
	plant := func(t *testing.T) (string, string) {
		t.Helper()
		root := newBench(t)
		runCLI(t, root, "add", "A card")
		var anchor string
		filepath.Walk(filepath.Join(storeRoot(t, root), bench.CardsDir), func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && filepath.Base(path) == bench.CardAnchor {
				anchor = path
			}
			return nil
		})
		if anchor == "" {
			t.Fatal("the fixture carries no card anchor")
		}
		data, err := os.ReadFile(anchor)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if err := os.WriteFile(anchor, []byte(strings.ReplaceAll(string(data), "\n", crlf)), 0o644); err != nil {
			t.Fatalf("plant: %v", err)
		}
		return root, anchor
	}

	root, anchor := plant(t)
	preview := runCLI(t, root, "check", "--migrate-newlines")
	for _, want := range []string{"preview", "rewritten with its own bytes", "modification time is now", "text files examined", "would be rewritten", "--yes"} {
		if !strings.Contains(preview.out, want) {
			t.Errorf("the preview does not say %q:\n%s", want, preview.out)
		}
	}
	if data, err := os.ReadFile(anchor); err != nil || strings.Contains(string(data), "\r") == false {
		t.Errorf("the preview changed the file's content: %v %q", err, data)
	}

	applied := runCLI(t, root, "check", "--migrate-newlines", "--yes")
	if !strings.Contains(applied.out, "rewritten.") {
		t.Errorf("the confirmed run does not say what it wrote:\n%s", applied.out)
	}
	if strings.Contains(applied.out, "preview") {
		t.Errorf("the confirmed run reads as a preview:\n%s", applied.out)
	}
	if data, err := os.ReadFile(anchor); err != nil || strings.Contains(string(data), "\r") {
		t.Errorf("the anchor was not repaired: %v %q", err, data)
	}

	again := runCLI(t, root, "check", "--migrate-newlines", "--yes")
	if !strings.Contains(again.out, "Nothing to rewrite") {
		t.Errorf("the second run does not say the store is clean:\n%s", again.out)
	}

	// A busy file is reported and skipped, and the run says that running it
	// again picks it up.
	busyRoot, busyAnchor := plant(t)
	lock, err := bench.Acquire(filepath.Dir(busyAnchor), "somebody-else", bench.Stamp(timeNowForTest()))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	busy := runCLI(t, busyRoot, "check", "--migrate-newlines", "--yes")
	lock.Release()
	if !strings.Contains(busy.out, "busy") || !strings.Contains(busy.out, "somebody-else") {
		t.Errorf("the busy run does not name the file and its holder:\n%s", busy.out)
	}

	// A refused plan writes nothing at all and names the condition.
	refusedRoot, refusedAnchor := plant(t)
	damaged := filepath.Join(filepath.Dir(refusedAnchor), bench.JournalName)
	// A record that does not decode and is not the last is a damaged store
	// rather than the torn tail this format expects, so the run refuses it and
	// writes nothing, the card anchor above included.
	torn := `{"ts":"2026-08-17T09:0` + "\n" + `{"ts":"2026-08-17T09:00:00Z","event":"created","reason":"a\r\nb"}` + "\n"
	if err := os.WriteFile(damaged, []byte(torn), 0o644); err != nil {
		t.Fatalf("plant a damaged record: %v", err)
	}
	refused := runCLI(t, refusedRoot, "check", "--migrate-newlines", "--yes")
	if !strings.Contains(refused.out, "refused") || !strings.Contains(refused.out, "will not decide") {
		t.Errorf("the refused run does not name the condition:\n%s", refused.out)
	}
	if refused.code == 0 {
		t.Error("the refused run exited clean")
	}
	if data, err := os.ReadFile(refusedAnchor); err != nil || !strings.Contains(string(data), "\r") {
		t.Errorf("the refused run repaired a destination: %v %q", err, data)
	}

}

// timeNowForTest is the clock a lock record is stamped with, named so the case
// above reads without a second import standing in one line.
func timeNowForTest() time.Time { return time.Now() }

// TestTheFormatDocumentAndTheHelpTextSayWhatTheToolNowDoes is the criterion
// that pins the prose rather than the behaviour, and it is asserted rather than
// read, because a document nobody checks drifts from the code it describes and
// this one is the only statement of the storage contract a later implementer
// will have.
//
// Three things are checked in the document. It says that a writer normalises
// rather than merely emitting LF, and it names each of the five functions that
// do it together with a clause saying why that function is the narrowest one
// every path to its kind of destination passes through. It says that a carriage
// return not followed by a line feed is left alone. And it names the three
// forms a stored line-ending carriage return takes, with the warning that the
// table describes what the writers produce and is not a way of detecting them,
// which is the sentence three separate drafts of this card were broken for
// lacking.
//
// Two are checked in the tool. `dinah help check` lists the flag, and the
// flag's own text carries the sentence about the file an interrupted run can
// leave behind.
func TestTheFormatDocumentAndTheHelpTextSayWhatTheToolNowDoes(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "design", "format.md"))
	if err != nil {
		t.Fatalf("read the format document: %v", err)
	}
	prose := flattenWords(string(doc))
	for _, clause := range []string{
		// That a writer normalises rather than merely emitting LF.
		"A writer emits LF everywhere by normalising rather than by remembering",
		"each is the narrowest function every path to its own kind of destination passes through",
		// The five functions, each with the clause saying why it is the one.
		"`ReadText` normalises what it read, because a file an external editor wrote is the one destination no write-path change can reach",
		"`WriteText` normalises the text it is handed, which covers every anchor body and every plain text file the tool writes",
		"`AppendEvent` normalises the event's string members before encoding, because a carriage return survives JSON encoding as an escape",
		"`quote` normalises a frontmatter scalar, and it is the only function that can. Every scalar in the tool becomes a frontmatter line by passing through it",
		"`ReadDefinition` normalises an interchange document at its single read boundary",
		// The member name, which is the one place the answer is a refusal.
		"refused `dinah.malformed-member-name`",
		// That a carriage return which is not a line ending survives.
		"A carriage return that ends at no line feed is left exactly where it is, on read, on write and by the repair",
		// That the unit is the run rather than the pair, which is what makes
		// the rule true of its own answer and every claim of idempotence
		// below it true.
		"The unit is the whole run rather than one pair, and the difference is whether the rule is true of its own answer",
		"Reducing the run is a fixed point",
		// That one confirmed run finishes.
		"one confirmed run finishes and a second rewrites nothing",
		// What the checker reports, and what it deliberately does not, which
		// is the operator's ruling of 2026-09-15.
		"A file whose carriage returns are all of the kind this format keeps is reported by nothing",
		"A file carrying one of each is still reported, for the one that is not legal",
		// The three forms, and the warning that they are not a detector.
		"the store carries no carriage return standing for a line ending, and such a carriage return takes three stored forms",
		"| A CRLF pair |",
		"| Split across a frontmatter escape |",
		"| A pair of JSON escapes |",
		"**That table describes what the writers produce. It is not a way of detecting the forms, and nothing in the tool detects them by pattern.**",
	} {
		if !strings.Contains(prose, flattenWords(clause)) {
			t.Errorf("docs/design/format.md does not state %q", clause)
		}
	}

	root := newBench(t)
	help := flattenWords(runCLI(t, root, "help", "check").out)
	if !strings.Contains(help, "--migrate-newlines") {
		t.Errorf("dinah help check does not list the flag:\n%s", help)
	}
	for _, clause := range []string{
		"repair every text file storing a carriage return that stands for a line ending",
		"rewriting each destination with its own bytes first to prove it can be written",
		"an interrupted run may leave a .dinah-* file beside an anchor, which is safe to delete because nothing reads it",
	} {
		if !strings.Contains(help, flattenWords(clause)) {
			t.Errorf("dinah help check does not say %q:\n%s", clause, help)
		}
	}
}

// TestTheWorkbenchTitleTakenFromItsOwnDirectoryNameIsNormalised is site B7, the
// one site of the counting rule that no test could reach while this card was
// worked on Windows.
//
// A bare `dinah init` derives the workbench's title from the name of the
// directory it is run in, so the slot is a directory name, and a directory name
// carrying a line ending is legal on Linux and on macOS and refused outright by
// Windows. The site was counted anyway rather than excluded, because the
// alternative was an exclusion of the form "not reachable on the operator's own
// machine", which is not a rule this format can hold. Now that the checks run
// on three platforms, the two that admit the byte drive it and the one that
// does not says so.
//
// B7 is also the one site the interchange document's own fix does not cover:
// defaultDefinition builds its definition in memory and never passes
// ReadDefinition, so what covers it is quote, through the frontmatter writer,
// which is the same thing that covers every other frontmatter-borne site.
func TestTheWorkbenchTitleTakenFromItsOwnDirectoryNameIsNormalised(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows refuses a directory name carrying a line ending, so this site is unreachable here; Linux and macOS drive it")
	}
	base := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")

	root := filepath.Join(base, "wb-a"+crlf+"wb-b")
	if err := os.MkdirAll(root, 0o755); err != nil {
		// Not a skip. The counting rule admits this site on the claim that
		// Linux and macOS accept the byte, so a platform that is neither
		// Windows nor accepts it contradicts the rule rather than excusing the
		// case, and the rule is what would need changing.
		t.Fatalf("%s refuses a directory name carrying a line ending, which the counting rule says it accepts: %v", runtime.GOOS, err)
	}
	if got := runQuiet(t, root, "init", "--slug", "fx"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	anchor := filepath.Join(storeRoot(t, root), bench.WorkbenchAnchor)
	data, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	if strings.Contains(string(data), "\r") {
		t.Errorf("the workbench anchor carries a carriage return: %q", data)
	}
	fm, _ := bench.ParseAnchor(string(data))
	title := fm.Value("title")
	if !strings.Contains(title, "wb-a") || !strings.Contains(title, "wb-b") {
		t.Fatalf("the title is %q and did not come from the directory name", title)
	}
	if strings.Contains(title, "\r") {
		t.Errorf("the title reads back carrying a carriage return: %q", title)
	}
}

// TestAConfirmedRepairAgreesWithItsOwnExitCode drives the second review finding
// through the binary, which is where it bites: the exit code is the half of a
// command a script reads and a person does not, so the store said success while
// the status said failure and nothing pointed at the disagreement.
//
// The tense of each destination line is asserted in the same run, which is the
// review's second minor finding. A confirmed run said "{path} carries N stored
// line endings to repair" about a file whose bytes it had already replaced.
func TestAConfirmedRepairAgreesWithItsOwnExitCode(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	var anchor string
	filepath.Walk(filepath.Join(storeRoot(t, root), bench.CardsDir), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(path) == bench.CardAnchor {
			anchor = path
		}
		return nil
	})
	if anchor == "" {
		t.Fatal("the fixture carries no card anchor")
	}
	data, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(anchor, []byte(strings.ReplaceAll(string(data), "\n", crlf)), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}

	preview := runCLI(t, root, "check", "--migrate-newlines")
	if !strings.Contains(preview.out, "to repair") {
		t.Errorf("a preview does not say what it would do:\n%s", preview.out)
	}
	if strings.Contains(preview.out, "repaired.") {
		t.Errorf("a preview speaks of a repair in the past tense:\n%s", preview.out)
	}
	if preview.code == 0 {
		t.Errorf("a preview that found a dirty file exited clean:\n%s", preview.out)
	}

	applied := runCLI(t, root, "check", "--migrate-newlines", "--yes")
	if !strings.Contains(applied.out, "repaired.") {
		t.Errorf("a confirmed run does not say what it did:\n%s", applied.out)
	}
	if strings.Contains(applied.out, "to repair") {
		t.Errorf("a confirmed run speaks of a repair still to come, over bytes it has already replaced:\n%s", applied.out)
	}
	if !strings.Contains(applied.out, "rewritten.") {
		t.Errorf("a confirmed run does not count what it wrote:\n%s", applied.out)
	}
	// The whole of the finding: the last line says the store is sound and the
	// status says it is not.
	if applied.code != 0 {
		t.Errorf("a confirmed run that repaired everything exited %d while printing:\n%s", applied.code, applied.out)
	}

	// A plain check on the same store agrees, which is what makes the exit
	// code above the defect rather than the store.
	after := runCLI(t, root, "check")
	if after.code != 0 {
		t.Errorf("the repaired store does not check clean: %d\n%s", after.code, after.out)
	}
	if again := runCLI(t, root, "check", "--migrate-newlines", "--yes"); again.code != 0 {
		t.Errorf("a second confirmed run over a clean store exited %d:\n%s", again.code, again.out)
	}
}

// TestTheRewriteLineCountsInWordsThatMatchTheNumber is the review's first minor
// finding: every sibling line of this report carries plural forms and this one
// did not, so a single destination read "carries 1 stored line endings".
func TestTheRewriteLineCountsInWordsThatMatchTheNumber(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	var anchor string
	filepath.Walk(filepath.Join(storeRoot(t, root), bench.CardsDir), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(path) == bench.CardAnchor {
			anchor = path
		}
		return nil
	})
	data, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Exactly one line ending, so the singular has to be chosen, and exactly
	// one loose carriage return beside it, so the clause reporting those is
	// drawn and is itself in the singular. A loose carriage return is one the
	// repair keeps, so it survives into the transform's output and is counted
	// there.
	planted := strings.Replace(string(data), "\n---\n", "\n---\r\n", 1)
	planted = strings.Replace(planted, "\ntitle: ", "\ntitle: \"one\rtwo\"\nformer_title: ", 1)
	if err := os.WriteFile(anchor, []byte(planted), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	preview := runCLI(t, root, "check", "--migrate-newlines")
	if strings.Contains(preview.out, "1 stored line endings") {
		t.Errorf("one line ending is counted in the plural:\n%s", preview.out)
	}
	if !strings.Contains(preview.out, "1 stored line ending to repair") {
		t.Errorf("the singular form was not chosen:\n%s", preview.out)
	}
	if !strings.Contains(preview.out, "1 carriage return that is not a line ending") {
		t.Errorf("the loose clause was not drawn in the singular:\n%s", preview.out)
	}
	if strings.Contains(preview.out, "1 carriage returns that are not") {
		t.Errorf("one loose carriage return is counted in the plural:\n%s", preview.out)
	}
}

// TestOneConfirmedRunSuffices is the end-to-end half of the idempotence
// blocker, and it is written against the promise rather than against the cause.
//
// Four places say a second run finds nothing: the specification, the doc
// comment on newlineTransform, the catalogue context on check.newlines-nothing,
// and the last clause of dinah-514/criteria/21. A file carrying a run of three
// carriage returns took three confirmed runs to clean, and each of the first
// two reported "1 file rewritten." So the promise was false four times over and
// the report said otherwise every time.
//
// The assertion is that the SECOND run has nothing to do, whatever the length
// of the run, which is what all four sentences claim. Counting runs would pass
// a build that needed two.
func TestOneConfirmedRunSuffices(t *testing.T) {
	for _, returns := range []int{1, 2, 3, 6} {
		t.Run("a run of "+strconv.Itoa(returns), func(t *testing.T) {
			root := newBench(t)
			runCLI(t, root, "add", "A card")
			var anchor string
			filepath.Walk(filepath.Join(storeRoot(t, root), bench.CardsDir), func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && filepath.Base(path) == bench.CardAnchor {
					anchor = path
				}
				return nil
			})
			if anchor == "" {
				t.Fatal("the fixture carries no card anchor")
			}
			data, err := os.ReadFile(anchor)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			ending := strings.Repeat("\r", returns) + "\n"
			if err := os.WriteFile(anchor, []byte(strings.ReplaceAll(string(data), "\n", ending)), 0o644); err != nil {
				t.Fatalf("plant: %v", err)
			}

			first := runCLI(t, root, "check", "--migrate-newlines", "--yes")
			if !strings.Contains(first.out, "rewritten.") {
				t.Fatalf("the first run wrote nothing:\n%s", first.out)
			}
			if first.code != 0 {
				t.Errorf("the first run exited %d:\n%s", first.code, first.out)
			}
			second := runCLI(t, root, "check", "--migrate-newlines", "--yes")
			if !strings.Contains(second.out, "Nothing to rewrite") {
				t.Errorf("a run of %d carriage returns needs more than one confirmed run:\n%s", returns, second.out)
			}
			if second.code != 0 {
				t.Errorf("the second run exited %d:\n%s", second.code, second.out)
			}
			if repaired, err := os.ReadFile(anchor); err != nil {
				t.Fatalf("read back: %v", err)
			} else if strings.Contains(string(repaired), "\r") {
				t.Errorf("one confirmed run left %q", repaired)
			}
		})
	}
}

// TestTheWriterNeverStoresWhatThisCardForbids is the first blocker at the level
// a person meets it: the ordinary comment verb, a body carrying a run of
// carriage returns, no editor and no hand editing anywhere.
//
// Before the pass reduced the whole run, this stored a CRLF pair and dinah
// check then reported the file the tool had just written.
func TestTheWriterNeverStoresWhatThisCardForbids(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	carryToDoing(t, root, "fx-1")
	body := "before" + strings.Repeat("\r", 3) + "\nafter" + crlf + crlf + "end"
	if got := runCLIWithInput(t, root, strings.NewReader(body), "--json", "comment", "fx-1", "-"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	for path, data := range walkStore(t, root) {
		if where := storedLineEnding(path, data); where != "" {
			t.Errorf("the tool's own comment verb stored %s in %s: %q", where, path, data)
		}
	}
	report := runCLI(t, root, "check")
	if strings.Contains(report.out, "carriage return") {
		t.Errorf("the check reports a file the tool had just written:\n%s", report.out)
	}
	if report.code != 0 {
		t.Errorf("a store the tool had just written does not check clean: %d\n%s", report.code, report.out)
	}
}

// TestAnAnchorWhoseBodyEndsInACarriageReturnIsRepairedNotRefused is the second
// blocker at the level a person meets it. Dinah writes such a file through its
// own comment verb, because a lone carriage return is a byte this format keeps,
// and its own repair then called the file damaged, blamed a header that was
// fine, and refused to repair anything else in the store.
func TestAnAnchorWhoseBodyEndsInACarriageReturnIsRepairedNotRefused(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	carryToDoing(t, root, "fx-1")
	if got := runCLIWithInput(t, root, strings.NewReader("text\r"), "--json", "comment", "fx-1", "-"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	// Something genuinely dirty elsewhere, so a refusal that denies the repair
	// to the rest of the store is what fails rather than going unnoticed.
	var column string
	filepath.Walk(filepath.Join(storeRoot(t, root), bench.ColumnsDir), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(path) == bench.ColumnAnchor {
			column = path
		}
		return nil
	})
	data, err := os.ReadFile(column)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	if err := os.WriteFile(column, []byte(strings.ReplaceAll(string(data), "\n", crlf)), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}

	applied := runCLI(t, root, "check", "--migrate-newlines", "--yes")
	if strings.Contains(applied.out, "will not decide") {
		t.Errorf("the repair called a file Dinah wrote damaged:\n%s", applied.out)
	}
	if applied.code != 0 {
		t.Errorf("the repair exited %d:\n%s", applied.code, applied.out)
	}
	if repaired, err := os.ReadFile(column); err != nil {
		t.Fatalf("read back: %v", err)
	} else if strings.Contains(string(repaired), "\r") {
		t.Error("one comment denied the repair to the column anchor beside it")
	}

	// The comment's own byte survives, which this case did not ask when it was
	// written. It asked only that the run was not refused and that the file
	// beside it was repaired, and a repair that deleted the byte satisfied
	// both, so the deletion reached a push with this test green.
	var comment string
	filepath.Walk(filepath.Join(storeRoot(t, root), bench.CardsDir), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(path) == bench.CommentAnchor {
			comment = path
		}
		return nil
	})
	if comment == "" {
		t.Fatal("the comment was not written, so nothing here is about a file Dinah wrote")
	}
	if stored, err := os.ReadFile(comment); err != nil {
		t.Fatalf("read the comment: %v", err)
	} else if !strings.HasSuffix(string(stored), "text\r") {
		t.Errorf("the repair deleted the carriage return Dinah itself stored, which dinah-514/decisions/7 keeps: %q", stored)
	}
	// A second confirmed run still finds nothing, so the byte is not being
	// removed and put back, and the store checks clean while carrying it.
	if again := runCLI(t, root, "check", "--migrate-newlines", "--yes"); !strings.Contains(again.out, "Nothing to rewrite") {
		t.Errorf("a second run found work:\n%s", again.out)
	}
	if report := runCLI(t, root, "check"); report.code != 0 {
		t.Errorf("a store carrying only a byte this format keeps does not check clean: %d\n%s", report.code, report.out)
	}
}
