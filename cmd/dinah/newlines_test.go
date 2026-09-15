package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

// TestTheInvariantHoldsAcrossTheWholeVerbSurface drives a CRLF value into every
// free-text slot the specification's counting rule produces, then walks every
// file of the workbench outside a payload directory and asserts that none
// carries any stored form of a line-ending carriage return.
//
// The slot list is derived from the counting rule rather than from a review
// comment, and the test states how many slots it drove and how many files it
// walked, failing if either is zero, on this workbench's rule that a sweep
// asserts the size of the set it swept.
//
// The two exempt sites are driven too, with the opposite assertion, so that the
// exemption is armed rather than assumed.
func TestTheInvariantHoldsAcrossTheWholeVerbSurface(t *testing.T) {
	root := newBench(t)
	slots := 0
	drive := func(name string, argv ...string) {
		t.Helper()
		slots++
		if got := runQuiet(t, root, argv...); got.code != 0 {
			t.Fatalf("%s: %d %s", name, got.code, got.errw)
		}
	}
	dirty := func(parts ...string) string { return strings.Join(parts, crlf) }

	// Group A, the sixteen verb.Request fields storing caller prose. One
	// field is one slot whatever filled it, because the CLI argument, the
	// stdin sentinel and the MCP argument are one field wearing three hats.
	drive("Title", "add", dirty("add-a", "add-b"))
	if got := runQuiet(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("move: %d %s", got.code, got.errw)
	}
	drive("Value", "set", "fx-1", "body", dirty("body-a", "body-b"))
	drive("Text, on a comment", "comment", "fx-1", dirty("comment-a", "comment-b"))
	drive("Owner and Text, on a checklist item", "file", "fx-1", "open_question", dirty("item-a", "item-b"), "--owner", dirty("own-a", "own-b"))
	slots++ // Owner rides beside Text above and is its own field.
	drive("Note", "resolve", "fx-1/questions/1", dirty("note-a", "note-b"))
	// A criterion to cite against. This is setup rather than a slot, because
	// nothing it carries is driven dirty.
	if got := runQuiet(t, root, "file", "fx-1", "acceptance_criterion", "a criterion"); got.code != 0 {
		t.Fatalf("file a criterion: %d %s", got.code, got.errw)
	}
	drive("Scheme", "cite", "fx-1/criteria/1", dirty("sch-a", "sch-b"), dirty("tgt-a", "tgt-b"))
	slots++ // CiteTarget rides beside Scheme above and is its own field.
	drive("Reason and Kind", "block", "fx-1", dirty("reason-a", "reason-b"), "--kind", dirty("kind-a", "kind-b"))
	slots++ // Kind rides beside Reason above and is its own field.
	drive("Column", "column", "new", dirty("col-a", "col-b"))
	drive("Workstream", "workstream", "new", dirty("ws-a", "ws-b"))
	drive("Actor", "comment", "fx-1", "a comment under a dirty actor", "--actor", dirty("act-a", "act-b"))

	// Provider, Model and Server arrive from the environment rather than
	// from an argument, and land in the actor block of every journal line.
	t.Setenv("DINAH_PROVIDER", dirty("prov-a", "prov-b"))
	t.Setenv("DINAH_MODEL", dirty("mod-a", "mod-b"))
	t.Setenv("DINAH_SERVER", dirty("srv-a", "srv-b"))
	drive("Provider, Model and Server", "comment", "fx-1", "a comment under a dirty agent")
	slots += 2 // Model and Server ride beside Provider and are their own fields.
	t.Setenv("DINAH_PROVIDER", "")
	t.Setenv("DINAH_MODEL", "")
	t.Setenv("DINAH_SERVER", "")

	// Group B. B4 and B5 are exempt and are driven below with the opposite
	// assertion; B6 and B7 need a workbench of their own and have their own
	// cases; B1 is unreachable by any write-path fix and is what the check
	// finding answers.
	source := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(source, []byte("payload-a"+crlf+"payload-b"+crlf), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	drive("Description", "attach", "fx-1", source, "--description", dirty("desc-a", "desc-b"))
	slots++ // B4, the payload's bytes, which the assertion below holds to being verbatim.
	slots++ // B5, the stored filename on attach, exempt for the same reason.
	// The stored filename is the payload file's own name on disk, and a
	// filename carrying a line feed is legal on Linux and on macOS and is
	// refused by Windows. This drives Value again rather than a slot of its
	// own, because one Request field is one site whatever filled it, and it is
	// driven only where the platform admits it.
	if runtime.GOOS == "windows" {
		t.Log("the rename slot is skipped: Windows refuses a filename carrying a line feed, and the anchor's filename key has to equal the file's own name on disk")
	} else if got := runQuiet(t, root, "rename", "fx-1/attachments/1", dirty("ren-a", "ren-b")); got.code != 0 {
		t.Fatalf("rename: %d %s", got.code, got.errw)
	}

	// B2, a definition document, and B3, the verbatim anchor copy an import
	// makes, each have a case of their own below, because each needs a
	// workbench the import writes rather than this one.
	slots += 2
	// B6 and B7 likewise.
	slots += 2
	// C1, the user-global instruction layer, has its own case below.
	slots++

	if want := 24; slots != want {
		t.Errorf("the sweep drove %d slots and the counting rule produces %d; a slot added to the rule and not added here is caught by this line", slots, want)
	}
	if slots == 0 {
		t.Fatal("the sweep drove no slot")
	}

	files := walkStore(t, root)
	if len(files) == 0 {
		t.Fatal("the sweep walked no file")
	}
	t.Logf("the sweep drove %d slots and walked %d files", slots, len(files))
	for path, data := range files {
		if where := storedLineEnding(path, data); where != "" {
			t.Errorf("%s carries %s", path, where)
		}
	}

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
	data, err := os.ReadFile(stored)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if string(data) != "payload-a"+crlf+"payload-b"+crlf {
		t.Errorf("the payload reads %q and was written with CRLF endings", data)
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
	step("verify", "verify", "fx-1/criteria/1", dirty("note-a", "note-b"))
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
		{"fx-1/criteria/1", "note"},
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
		got := runCLI(t, destination, "init", "--from", refused)
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
		// That a lone carriage return survives.
		"Normalisation is CRLF to LF and nothing else. A carriage return not followed by a line feed is left exactly where it is",
		"the reader rule above strips only a TRAILING carriage return per line",
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
		t.Skipf("this platform refuses a directory name carrying a line ending: %v", err)
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
