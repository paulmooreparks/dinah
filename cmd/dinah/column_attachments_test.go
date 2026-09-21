package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// attachToColumn writes a file outside the workbench and attaches it to a
// column as the operator, failing the test unless the attach succeeded. It
// answers the bytes it attached, so a caller can compare the payload later.
func attachToColumn(t *testing.T, root, column, name, body, description string) []byte {
	t.Helper()
	source := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(source, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", source, err)
	}
	argv := []string{"attach", column, source}
	if description != "" {
		argv = append(argv, "--description", description)
	}
	if got := runCLI(t, root, argv...); got.code != 0 {
		t.Fatalf("attach %s to %s: %d %s", name, column, got.code, got.errw)
	}
	return []byte(body)
}

// servedListing decodes the instruction chain a --json answer carries,
// whether a coordination act published it at the top level or the
// instructions command published it as its own member, and answers the
// listing with a report of whether the member was present at all.
func servedListing(t *testing.T, got invocation) ([]verb.AttachmentView, bool) {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("the run exited %d: %s", got.code, got.errw)
	}
	var answer struct {
		Instructions map[string]json.RawMessage `json:"instructions"`
	}
	if err := json.Unmarshal([]byte(got.out), &answer); err != nil {
		t.Fatalf("the answer does not parse: %v\n%s", err, got.out)
	}
	if answer.Instructions == nil {
		t.Fatalf("the answer carries no instructions member:\n%s", got.out)
	}
	raw, present := answer.Instructions[verb.LayerColumnAttachments]
	if !present {
		return nil, false
	}
	var listing []verb.AttachmentView
	if err := json.Unmarshal(raw, &listing); err != nil {
		t.Fatalf("the listing does not parse: %v\n%s", err, raw)
	}
	return listing, true
}

// wantListingBlock fails unless a rendered serve prints the listing block
// after the column's instructions and before the moves, with one row per
// reference named and the line saying how to read one.
func wantListingBlock(t *testing.T, what, printed string, refs ...string) {
	t.Helper()
	label := msg.For(msg.Base).T("instructions.column-attachments")
	read := msg.For(msg.Base).T("instructions.column-attachments.read")
	at := strings.Index(printed, label)
	if at < 0 {
		t.Fatalf("%s printed no listing:\n%s", what, printed)
	}
	column := strings.Index(printed, msg.For(msg.Base).T("instructions.column"))
	if column < 0 || column > at {
		t.Errorf("%s printed the listing ahead of the column's instructions:\n%s", what, printed)
	}
	if moves := strings.Index(printed, msg.For(msg.Base).T("instructions.moves")); moves >= 0 && moves < at {
		t.Errorf("%s printed the listing after the moves:\n%s", what, printed)
	}
	block := printed[at:]
	for _, ref := range refs {
		if !strings.Contains(block, "  "+ref+"  ") {
			t.Errorf("%s printed no row for %s:\n%s", what, ref, block)
		}
	}
	if !strings.Contains(block, read) {
		t.Errorf("%s printed no line saying how to read an attachment:\n%s", what, block)
	}
}

// TestAClaimServesTheColumnsListing asserts dinah-545/criteria/1 and the
// command half of dinah-545/criteria/6. A claim at a column carrying two
// attachments answers both, in ordinal order, each with the reference, the
// filename, the description and a path whose bytes are the ones attached; the
// same serve rendered prints the block after the column's text and before
// the moves; and instructions naming the column and naming the card both print
// it.
func TestAClaimServesTheColumnsListing(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "set", "doing", "instructions", "Work the card."); got.code != 0 {
		t.Fatalf("set the column's text: %d %s", got.code, got.errw)
	}
	first := attachToColumn(t, root, "doing", "merge.md", "Merge after the checks pass.\n", "how to merge")
	second := attachToColumn(t, root, "doing", "blob.bin", "a\r\nb\x00c", "a binary")
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")

	listing, present := servedListing(t, runCLI(t, root, "claim", "fx-1", "--json"))
	if !present || len(listing) != 2 {
		t.Fatalf("wanted two entries served, got %v %+v", present, listing)
	}
	wanted := []struct {
		ref, filename, description string
		bytes                      []byte
	}{
		{"doing/attachments/1", "merge.md", "how to merge", first},
		{"doing/attachments/2", "blob.bin", "a binary", second},
	}
	for position, want := range wanted {
		got := listing[position]
		if got.Ordinal != position+1 || got.Ref != want.ref || got.Filename != want.filename || got.Description != want.description {
			t.Errorf("entry %d is %+v, wanted %s %s %q", position+1, got, want.ref, want.filename, want.description)
		}
		data, err := os.ReadFile(got.Path)
		if err != nil {
			t.Fatalf("entry %d's path does not read: %v", position+1, err)
		}
		if string(data) != string(want.bytes) {
			t.Errorf("entry %d's path holds %q, wanted %q", position+1, data, want.bytes)
		}
	}

	if got := runCLI(t, root, "release", "fx-1"); got.code != 0 {
		t.Fatalf("release: %d %s", got.code, got.errw)
	}
	claimed := runCLI(t, root, "claim", "fx-1")
	if claimed.code != 0 {
		t.Fatalf("claim: %d %s", claimed.code, claimed.errw)
	}
	wantListingBlock(t, "the rendered claim", claimed.out, "doing/attachments/1", "doing/attachments/2")
	for _, ref := range []string{"doing", "fx-1"} {
		asked := runCLI(t, root, "instructions", ref)
		if asked.code != 0 {
			t.Fatalf("instructions %s: %d %s", ref, asked.code, asked.errw)
		}
		wantListingBlock(t, "instructions "+ref, asked.out, "doing/attachments/1", "doing/attachments/2")
	}
}

// TestAMoveServesTheListingOfTheColumnItEnters asserts dinah-545/criteria/2: a
// move into a column carrying one attachment answers that one entry, and a
// move into a column carrying none answers no column_attachments member at
// all, rather than an empty array, and prints no listing.
func TestAMoveServesTheListingOfTheColumnItEnters(t *testing.T) {
	root := newBench(t)
	attachToColumn(t, root, "done", "release.md", "Tag the release.\n", "how to release")
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")

	into := runCLI(t, root, "move", "fx-1", "done", "--json")
	listing, present := servedListing(t, into)
	if !present || len(listing) != 1 || listing[0].Ref != "done/attachments/1" || listing[0].Filename != "release.md" {
		t.Fatalf("the move into done served %v %+v", present, listing)
	}
	back := runCLI(t, root, "move", "fx-1", "doing", "--json")
	if _, present := servedListing(t, back); present {
		t.Errorf("a move into a column carrying no attachment served the member:\n%s", back.out)
	}
	if strings.Contains(back.out, verb.LayerColumnAttachments) {
		t.Errorf("a move into a column carrying no attachment named the listing:\n%s", back.out)
	}
	rendered := runCLI(t, root, "move", "fx-1", "intake")
	if strings.Contains(rendered.out, msg.For(msg.Base).T("instructions.column-attachments")) {
		t.Errorf("a rendered move into a column carrying no attachment printed a listing:\n%s", rendered.out)
	}
}

// TestAnArchivedColumnsAttachmentComesBackWithIt asserts dinah-545/criteria/8.
// Archiving a column carrying an attachment makes instructions naming it
// refuse dinah.unknown-path and takes the attachment out of every serve, and
// restoring the column brings the attachment back under the same reference
// with byte-identical payload, served to a card moved in.
func TestAnArchivedColumnsAttachmentComesBackWithIt(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "column", "new", "Spare", "--slug", "spare"); got.code != 0 {
		t.Fatalf("column new: %d %s", got.code, got.errw)
	}
	attached := attachToColumn(t, root, "spare", "spare.bin", "a\r\nb\x00c", "kept aside")
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "archive", "spare"); got.code != 0 {
		t.Fatalf("archive: %d %s", got.code, got.errw)
	}
	refused := runCLI(t, root, "instructions", "spare", "--json")
	if refused.code == 0 || !strings.Contains(refused.out+refused.errw, "dinah.unknown-path") {
		t.Errorf("instructions naming an archived column: wanted dinah.unknown-path, got %d %s %s", refused.code, refused.out, refused.errw)
	}
	for _, ref := range []string{"fx-1", "intake", "doing", "done"} {
		served := runCLI(t, root, "instructions", ref)
		if strings.Contains(served.out, "spare.bin") {
			t.Errorf("instructions %s served the archived column's attachment:\n%s", ref, served.out)
		}
	}
	if got := runCLI(t, root, "restore", "spare"); got.code != 0 {
		t.Fatalf("restore: %d %s", got.code, got.errw)
	}
	listing, present := servedListing(t, runCLI(t, root, "move", "fx-1", "spare", "--json"))
	if !present || len(listing) != 1 || listing[0].Ref != "spare/attachments/1" {
		t.Fatalf("the move into the restored column served %v %+v", present, listing)
	}
	data, err := os.ReadFile(listing[0].Path)
	if err != nil {
		t.Fatalf("read the restored payload: %v", err)
	}
	if string(data) != string(attached) {
		t.Errorf("the restored payload holds %q, wanted %q", data, attached)
	}
}

// TestColumnBodyLimitIsAWorkbenchField asserts dinah-545/criteria/14. The
// operator writes 4096 and reads it back, and the workbench journal records
// the write; 0 and x are refused malformed and write nothing; a non-operator
// is refused not-operator; and clearing the field removes the key, after
// which check measures no body.
func TestColumnBodyLimitIsAWorkbenchField(t *testing.T) {
	root := newBench(t)
	anchor := pathOf(t, root, "workbench")
	journal := filepath.Join(filepath.Dir(anchor), bench.JournalName)

	if got := runCLI(t, root, "set", "workbench", bench.ColumnBodyLimitField, "4096"); got.code != 0 {
		t.Fatalf("set: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "get", "workbench", bench.ColumnBodyLimitField); strings.TrimSpace(got.out) != "4096" {
		t.Errorf("get answered %q", got.out)
	}
	recorded := false
	for _, event := range journalEvents(t, journal) {
		if event["event"] == "workbench_updated" && event["field"] == bench.ColumnBodyLimitField && event["to"] == "4096" {
			recorded = true
		}
	}
	if !recorded {
		t.Error("the workbench journal carries no workbench_updated line for the write")
	}

	before, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	for _, value := range []string{"0", "x"} {
		got := runCLI(t, root, "set", "workbench", bench.ColumnBodyLimitField, value, "--json")
		if got.code == 0 || !strings.Contains(got.out, `"malformed"`) {
			t.Errorf("set %s: wanted malformed, got %d %s", value, got.code, got.out)
		}
	}
	refused := runCLI(t, root, "set", "workbench", bench.ColumnBodyLimitField, "8192", "--actor", "bo", "--json")
	if refused.code == 0 || !strings.Contains(refused.out, `"not-operator"`) {
		t.Errorf("a non-operator's set: wanted not-operator, got %d %s", refused.code, refused.out)
	}
	after, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("a refused write changed the anchor:\n%s", after)
	}

	// A body far over the declared limit is reported while the key stands
	// and is not once it is cleared, which is what shows the clear removed
	// the declaration rather than leaving an empty value check still reads.
	column := pathOf(t, root, "doing")
	if err := os.WriteFile(column, []byte("---\ntitle: Doing\nslug: doing\nkind: work\n---\n"+strings.Repeat("a", 5000)), 0o644); err != nil {
		t.Fatalf("write the long body: %v", err)
	}
	if got := runCLI(t, root, "check"); !strings.Contains(got.out+got.errw, "doing 5000/4096") {
		t.Errorf("check did not report the long body while the limit stood:\n%s%s", got.out, got.errw)
	}
	if got := runCLI(t, root, "set", "workbench", bench.ColumnBodyLimitField); got.code != 0 {
		t.Fatalf("clear: %d %s", got.code, got.errw)
	}
	cleared, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if strings.Contains(string(cleared), bench.ColumnBodyLimitField) {
		t.Errorf("the clear left the key in the anchor:\n%s", cleared)
	}
	if got := runCLI(t, root, "check"); strings.Contains(got.out+got.errw, "5000/") {
		t.Errorf("check measured a body after the limit was cleared:\n%s%s", got.out, got.errw)
	}
}

// TestInitFromAMalformedAttachmentsMemberWritesNothing asserts the command
// half of dinah-545/criteria/18: init --from a definition whose column
// element carries an attachments member that is a string is refused
// malformed naming attachments, and the target holds no workbench afterwards,
// while the same definition with the member corrected is accepted.
func TestInitFromAMalformedAttachmentsMemberWritesNothing(t *testing.T) {
	newBench(t)
	base := t.TempDir()
	definition := func(member string) string {
		return `{"profile": "dinah-core/0.7", "title": "Carried", "columns": [` +
			`{"id": "d00000000001", "title": "Intake", "kind": "intake"},` +
			`{"id": "d00000000002", "title": "Doing", "kind": "work", "attachments": ` + member + `}]}`
	}
	shapes := map[string]string{
		"refused":  `"notes.md"`,
		"accepted": `[{"filename": "notes.md", "payload": "bm90ZXMK"}]`,
	}
	for name, member := range shapes {
		source := filepath.Join(base, name+".json")
		if err := os.WriteFile(source, []byte(definition(member)), 0o644); err != nil {
			t.Fatalf("write %s: %v", source, err)
		}
		target := filepath.Join(base, name)
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		got := runCLI(t, target, "init", "--from", source, "--slug", "cy", "--operator", "alka", "--json")
		made := bench.Exists(filepath.Join(target, bench.UserBaseName))
		if name == "refused" {
			if got.code == 0 || !strings.Contains(got.out, `"malformed"`) || !strings.Contains(got.out, `"attachments"`) {
				t.Errorf("init --from a string member: wanted malformed attachments, got %d %s", got.code, got.out)
			}
			if made {
				t.Error("the refused init left a workbench directory behind")
			}
			continue
		}
		if got.code != 0 || !made {
			t.Errorf("init --from the corrected member: %d %s %s", got.code, got.out, got.errw)
		}
	}
}

// operatorRows are the rows dinah-545 adds to five precondition lists, each
// with the position its help page draws it at. The positions count the
// harness row every writing command's list opens with.
var operatorRows = []struct {
	command, key string
	position     int
}{
	{"attach", "check.attach.5", 5},
	{"rename", "check.rename.6", 5},
	{"archive", "check.archive.4", 3},
	{"restore", "check.restore.4", 4},
	{"delete", "check.delete.5", 4},
}

// TestTheHelpPagesShowTheOperatorRows asserts dinah-545/criteria/21: each of
// the five pages draws its new row, with the catalog's text, at the position
// the contract places it, and verb.Checks lists not-operator there.
func TestTheHelpPagesShowTheOperatorRows(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "80")
	if len(operatorRows) != 5 {
		t.Fatalf("the criterion names five pages and this sweep drives %d", len(operatorRows))
	}
	for _, row := range operatorRows {
		entry, ok := msg.BaseEntry(row.key)
		if !ok {
			t.Fatalf("the catalog carries no %s", row.key)
		}
		checks := verb.Checks(row.command)
		if len(checks) < row.position || checks[row.position-1].Key != row.key || checks[row.position-1].Refusal != "not-operator" {
			t.Errorf("%s lists %+v, wanted %s at %d", row.command, checks, row.key, row.position)
		}
		page := runCLI(t, root, "help", row.command)
		if page.code != 0 {
			t.Fatalf("help %s: %d %s", row.command, page.code, page.errw)
		}
		drawn := regexp.MustCompile(`(?m)^  ` + strconv.Itoa(row.position) + `\s+` + regexp.QuoteMeta(entry.Text))
		if !drawn.MatchString(page.out) {
			t.Errorf("help %s draws no row %d reading %q:\n%s", row.command, row.position, entry.Text, page.out)
		}
	}
}
