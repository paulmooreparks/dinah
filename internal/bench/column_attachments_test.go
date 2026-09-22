package bench

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// carryDefinition declares two columns, the second of which the tests below
// hang attachments on, and a column_body_limit the check tests read.
const carryDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Carrying",
  "column_body_limit": 4096,
  "columns": [
    { "id": "d00000000001", "title": "Intake", "kind": "intake" },
    { "id": "d00000000002", "title": "Doing", "kind": "work",
      "instructions": "Doing text.\n" }
  ]
}`

// binaryPayload carries a carriage return, a line feed and a NUL byte, which
// is what a path through ReadText or WriteText would change.
const binaryPayload = "a\r\nb\x00c\n"

// newCarryBench instantiates carryDefinition under a fresh directory and
// hangs two attachments on its second column, the second of them binary.
func newCarryBench(t *testing.T) *Bench {
	t.Helper()
	root := containedPath(filepath.Join(t.TempDir(), "source"))
	definition, err := ReadDefinition([]byte(carryDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := Instantiate(root, "cy", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	column := filepath.Join(root, ColumnsDir, "d00000000002")
	if _, err := AddAttachmentBytes(column, "notes.md", []byte("notes\n"), "the notes", "alka"); err != nil {
		t.Fatalf("attach notes: %v", err)
	}
	if _, err := AddAttachmentBytes(column, "blob.bin", []byte(binaryPayload), "", "bo"); err != nil {
		t.Fatalf("attach blob: %v", err)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return opened
}

// payloadsOf reads a column's live attachments back as filename, description
// and payload, in order, which is what a round trip has to preserve.
func payloadsOf(t *testing.T, columnDir string) []string {
	t.Helper()
	attachments, err := Attachments(columnDir)
	if err != nil {
		t.Fatalf("list %s: %v", columnDir, err)
	}
	var carried []string
	for _, attachment := range attachments {
		data, err := os.ReadFile(attachment.Path)
		if err != nil {
			t.Fatalf("read %s: %v", attachment.Path, err)
		}
		carried = append(carried, attachment.Filename+"|"+attachment.Description+"|"+strconv.Quote(string(data)))
	}
	return carried
}

// TestAColumnsAttachmentsRideTheInterchange asserts dinah-545/criteria/16. The
// export carries the column's two attachments in ordinal order with filename,
// description, provenance and a base64 payload, and a column carrying none
// carries no member. init --from the export yields a column whose listing and
// payloads match byte for byte, and exporting that workbench gives back the
// first export's bytes.
func TestAColumnsAttachmentsRideTheInterchange(t *testing.T) {
	source := newCarryBench(t)
	exported, err := source.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	columns := exportedColumns(t, exported)
	if len(columns) != 2 {
		t.Fatalf("wanted two columns in the export, got %d", len(columns))
	}
	if _, carried := columns[0][AttachmentsMember]; carried {
		t.Error("a column carrying no attachment exported an attachments member")
	}
	var members []map[string]string
	if err := json.Unmarshal(columns[1][AttachmentsMember], &members); err != nil {
		t.Fatalf("the attachments member: %v in %s", err, columns[1][AttachmentsMember])
	}
	if len(members) != 2 {
		t.Fatalf("wanted two attachments exported, got %d", len(members))
	}
	wanted := []map[string]string{
		{"filename": "notes.md", "description": "the notes", "provenance": "alka", "payload": base64.StdEncoding.EncodeToString([]byte("notes\n"))},
		{"filename": "blob.bin", "provenance": "bo", "payload": base64.StdEncoding.EncodeToString([]byte(binaryPayload))},
	}
	for position, want := range wanted {
		got := members[position]
		if len(got) != len(want) {
			t.Errorf("attachment %d exported the members %v, wanted %v", position+1, got, want)
		}
		for key, value := range want {
			if got[key] != value {
				t.Errorf("attachment %d exported %s as %q, wanted %q", position+1, key, got[key], value)
			}
		}
	}

	target := containedPath(filepath.Join(t.TempDir(), "target"))
	reread, err := ReadDefinition(exported)
	if err != nil {
		t.Fatalf("read the export back: %v", err)
	}
	if err := Instantiate(target, "cy", "alka", reread); err != nil {
		t.Fatalf("instantiate the import: %v", err)
	}
	want := payloadsOf(t, source.ColumnDir("d00000000002"))
	got := payloadsOf(t, filepath.Join(target, ColumnsDir, "d00000000002"))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the import's listing differs:\ngot  %v\nwant %v", got, want)
	}
	imported, err := Open(target)
	if err != nil {
		t.Fatalf("open the import: %v", err)
	}
	again, err := imported.Export()
	if err != nil {
		t.Fatalf("export the import: %v", err)
	}
	if !bytes.Equal(again, exported) {
		t.Errorf("the round trip is not byte for byte:\nfirst:\n%s\nsecond:\n%s", exported, again)
	}
}

// TestExtractCarriesTheLiveAttachmentsAndNothingElse asserts
// dinah-545/criteria/17. The source column also carries an archived
// attachment and a comment, and the extracted directory holds the live
// attachment's anchor and byte-identical payload and nothing under the
// column's archive or comments. init --from that directory, which reads it
// through OpenUncontained, Export and ReadDefinition as readSource does,
// yields the source's live listing byte for byte.
func TestExtractCarriesTheLiveAttachmentsAndNothingElse(t *testing.T) {
	source := newCarryBench(t)
	column := source.ColumnDir("d00000000002")
	archived := filepath.Join(column, ArchiveDir, AttachmentsDir, "e00000000009")
	write(t, filepath.Join(archived, AttachmentAnchor), "---\nfilename: old.md\nprovenance: alka\nordinal: 1\n---\n")
	write(t, filepath.Join(archived, PayloadDir, "old.md"), "old\n")
	if _, err := AddComment(column, "alka", "2026-09-21T09:00:00Z", "a remark"); err != nil {
		t.Fatalf("comment: %v", err)
	}

	target := filepath.Join(t.TempDir(), "template")
	if err := source.Extract(target); err != nil {
		t.Fatalf("extract: %v", err)
	}
	extracted := filepath.Join(target, ColumnsDir, "d00000000002")
	for _, absent := range []string{ArchiveDir, CommentsDir} {
		if Exists(filepath.Join(extracted, absent)) {
			t.Errorf("the extract copied the column's %s", absent)
		}
	}
	live, err := Attachments(column)
	if err != nil || len(live) != 2 {
		t.Fatalf("the source's live attachments: %v %d", err, len(live))
	}
	for _, attachment := range live {
		copied := filepath.Join(extracted, AttachmentsDir, attachment.ID)
		if !Exists(filepath.Join(copied, AttachmentAnchor)) {
			t.Errorf("the extract left out the anchor of %s", attachment.Filename)
		}
		want, err := os.ReadFile(attachment.Path)
		if err != nil {
			t.Fatalf("read %s: %v", attachment.Path, err)
		}
		got, err := os.ReadFile(filepath.Join(copied, PayloadDir, attachment.Filename))
		if err != nil {
			t.Fatalf("read the extracted %s: %v", attachment.Filename, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("the extracted %s holds %q, wanted %q", attachment.Filename, got, want)
		}
	}

	template, err := OpenUncontained(target)
	if err != nil {
		t.Fatalf("open the template: %v", err)
	}
	exported, err := template.Export()
	if err != nil {
		t.Fatalf("export the template: %v", err)
	}
	definition, err := ReadDefinition(exported)
	if err != nil {
		t.Fatalf("read the template's definition: %v", err)
	}
	fresh := containedPath(filepath.Join(t.TempDir(), "fresh"))
	if err := Instantiate(fresh, "fr", "alka", definition); err != nil {
		t.Fatalf("instantiate from the template: %v", err)
	}
	want := payloadsOf(t, column)
	got := payloadsOf(t, filepath.Join(fresh, ColumnsDir, "d00000000002"))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("init --from the template listed:\n%v\nwanted\n%v", got, want)
	}
}

// TestAMalformedAttachmentsMemberRefusesBeforeWriting asserts the library half
// of dinah-545/criteria/18: a member that is a string, an element whose
// payload is not base64, and an element whose filename carries a slash are
// each refused malformed with the detail attachments, and the corrected member
// is read. The other rules the member carries are driven beside them, so each
// clause of ColumnAttachmentsOf has a case that reaches it.
func TestAMalformedAttachmentsMemberRefusesBeforeWriting(t *testing.T) {
	good := `{ "filename": "notes.md", "payload": "bm90ZXMK" }`
	cases := map[string]string{
		"a string":                       `"notes.md"`,
		"a payload that is not base64":   `[ { "filename": "notes.md", "payload": "not base64!" } ]`,
		"a filename carrying a slash":    `[ { "filename": "a/notes.md", "payload": "bm90ZXMK" } ]`,
		"null":                           `null`,
		"an element that is not object":  `[ "notes.md" ]`,
		"no filename":                    `[ { "payload": "bm90ZXMK" } ]`,
		"no payload":                     `[ { "filename": "notes.md" } ]`,
		"a description that is a number": `[ { "filename": "notes.md", "payload": "bm90ZXMK", "description": 3 } ]`,
		"a provenance that is null":      `[ { "filename": "notes.md", "payload": "bm90ZXMK", "provenance": null } ]`,
	}
	if len(cases) != 9 {
		t.Fatalf("wanted nine malformed shapes, got %d", len(cases))
	}
	definition := func(member string) string {
		return strings.Replace(carryDefinition, `"instructions": "Doing text.\n"`, `"instructions": "Doing text.\n", "attachments": `+member, 1)
	}
	for name, member := range cases {
		text := definition(member)
		if !strings.Contains(text, member) {
			t.Fatalf("%s: the fixture did not take the member", name)
		}
		_, err := ReadDefinition([]byte(text))
		var refusal *contract.Refusal
		if !errors.As(err, &refusal) || refusal.Name != contract.Malformed || refusal.Detail != AttachmentsMember {
			t.Errorf("%s: wanted malformed attachments, got %v", name, err)
		}
	}
	if _, err := ReadDefinition([]byte(definition("[ " + good + " ]"))); err != nil {
		t.Errorf("the corrected member was refused: %v", err)
	}
}

// writeColumnBody rewrites the fixture column's anchor with the frontmatter
// and body given, which is what a person editing column.md by hand does.
func writeColumnBody(t *testing.T, root, extraFrontmatter, body string) {
	t.Helper()
	anchor := "---\ntitle: Only\nslug: only\nkind: work\n" + extraFrontmatter + "---\n" + body
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), anchor)
}

// sizeFindings runs check and answers the findings about column body sizes.
func sizeFindings(t *testing.T, root string) []Finding {
	t.Helper()
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	var sized []Finding
	for _, finding := range findings {
		if finding.Key == FindingColumnBodyOverLimit || finding.Key == FindingColumnBodyLimitMalformed {
			sized = append(sized, finding)
		}
	}
	return sized
}

// declareLimit writes column_body_limit into the fixture's workbench anchor.
func declareLimit(t *testing.T, root, value string) {
	t.Helper()
	editWorkbench(t, root, "operator: alka\n", "operator: alka\ncolumn_body_limit: "+value+"\n")
}

// TestCheckReportsABodyOverTheDeclaredLimit asserts dinah-545/criteria/11 and
// dinah-545/criteria/12. At a limit of 100 bytes a body of exactly 100 bytes
// is not reported and one of 101 is, once, at cleanup severity on the
// column's anchor; the same holds when the bytes are two-byte characters,
// because the unit is bytes rather than characters; and a frontmatter of
// several hundred bytes over a short body is not measured.
func TestCheckReportsABodyOverTheDeclaredLimit(t *testing.T) {
	cases := []struct {
		name  string
		front string
		body  string
		size  int
		want  string
	}{
		{name: "at the limit", body: strings.Repeat("a", 99) + "\n", size: 100},
		{name: "one byte over", body: strings.Repeat("a", 100) + "\n", size: 101, want: "only 101/100"},
		{name: "fifty two-byte characters", body: strings.Repeat("é", 50), size: 100},
		{name: "fifty-one two-byte characters", body: strings.Repeat("é", 51), size: 102, want: "only 102/100"},
		{name: "a long frontmatter", front: "note: " + strings.Repeat("x", 400) + "\n", body: "Short.\n", size: 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			declareLimit(t, root, "100")
			writeColumnBody(t, root, c.front, c.body)
			opened, err := Open(root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			if got := len(opened.Columns[0].Instructions); got != c.size {
				t.Fatalf("the fixture's body parses to %d bytes, not the %d the case names", got, c.size)
			}
			findings := sizeFindings(t, root)
			if c.want == "" {
				if len(findings) != 0 {
					t.Errorf("wanted no size finding, got %+v", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("wanted one size finding, got %+v", findings)
			}
			finding := findings[0]
			if finding.Key != FindingColumnBodyOverLimit || finding.Detail != c.want {
				t.Errorf("wanted %s %q, got %s %q", FindingColumnBodyOverLimit, c.want, finding.Key, finding.Detail)
			}
			if SeverityOf(finding) != SeverityCleanup {
				t.Errorf("the finding's severity is %s, wanted %s", SeverityOf(finding), SeverityCleanup)
			}
			if finding.Path != filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor) {
				t.Errorf("the finding names %s rather than the column's anchor", finding.Path)
			}
		})
	}
}

// TestAnAbsentLimitMeasuresNothingAndAMalformedOneSaysSo asserts
// dinah-545/criteria/13: with no declaration a body of 100000 bytes is not
// reported, and with 0, -5 and abc each run reports one malformed finding at
// defect severity on workbench.md carrying the value as stored, and no size
// finding beside it.
func TestAnAbsentLimitMeasuresNothingAndAMalformedOneSaysSo(t *testing.T) {
	root := newFixture(t)
	writeColumnBody(t, root, "", strings.Repeat("a", 100000))
	if findings := sizeFindings(t, root); len(findings) != 0 {
		t.Errorf("an absent declaration measured something: %+v", findings)
	}
	values := []string{"0", "-5", "abc"}
	for _, value := range values {
		root := newFixture(t)
		declareLimit(t, root, value)
		writeColumnBody(t, root, "", strings.Repeat("a", 100000))
		findings := sizeFindings(t, root)
		if len(findings) != 1 {
			t.Fatalf("%s: wanted exactly one finding, got %+v", value, findings)
		}
		finding := findings[0]
		if finding.Key != FindingColumnBodyLimitMalformed || finding.Detail != value {
			t.Errorf("%s: wanted %s %q, got %s %q", value, FindingColumnBodyLimitMalformed, value, finding.Key, finding.Detail)
		}
		if SeverityOf(finding) != SeverityDefect {
			t.Errorf("%s: the finding's severity is %s", value, SeverityOf(finding))
		}
		if finding.Path != filepath.Join(root, WorkbenchAnchor) {
			t.Errorf("%s: the finding names %s rather than workbench.md", value, finding.Path)
		}
	}
}

// TestTheLimitSurvivesTheTrip asserts dinah-545/criteria/15: a workbench
// declaring column_body_limit, exported and instantiated, and separately
// extracted and instantiated from the directory, declares the same limit in
// both new workbenches, and check reports the same size findings on each as
// on the source.
func TestTheLimitSurvivesTheTrip(t *testing.T) {
	source := newCarryBench(t)
	long := "---\ntitle: Doing\nslug: doing\nkind: work\n---\n" + strings.Repeat("a", 5000)
	write(t, source.ColumnAnchorPath("d00000000002"), long)
	source, err := Open(source.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	wantFindings := sizeDetails(t, source.Root)
	if len(wantFindings) != 1 || wantFindings[0] != "doing 5000/4096" {
		t.Fatalf("the source reports %v, and the fixture meant one finding", wantFindings)
	}

	exported, err := source.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	fromExport := containedPath(filepath.Join(t.TempDir(), "exported"))
	definition, err := ReadDefinition(exported)
	if err != nil {
		t.Fatalf("read the export: %v", err)
	}
	if err := Instantiate(fromExport, "ex", "alka", definition); err != nil {
		t.Fatalf("instantiate the export: %v", err)
	}

	extracted := filepath.Join(t.TempDir(), "extracted")
	if err := source.Extract(extracted); err != nil {
		t.Fatalf("extract: %v", err)
	}
	template, err := OpenUncontained(extracted)
	if err != nil {
		t.Fatalf("open the template: %v", err)
	}
	templateExport, err := template.Export()
	if err != nil {
		t.Fatalf("export the template: %v", err)
	}
	fromTemplate := containedPath(filepath.Join(t.TempDir(), "templated"))
	definition, err = ReadDefinition(templateExport)
	if err != nil {
		t.Fatalf("read the template: %v", err)
	}
	if err := Instantiate(fromTemplate, "tp", "alka", definition); err != nil {
		t.Fatalf("instantiate the template: %v", err)
	}

	for _, root := range []string{fromExport, fromTemplate} {
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open %s: %v", root, err)
		}
		if got := opened.FM.Value(ColumnBodyLimitField); got != "4096" {
			t.Errorf("%s declares column_body_limit %q, wanted 4096", root, got)
		}
		if got := sizeDetails(t, root); strings.Join(got, ",") != strings.Join(wantFindings, ",") {
			t.Errorf("%s reports %v, the source reports %v", root, got, wantFindings)
		}
	}
}

// sizeDetails answers the detail of every over-limit finding check reports,
// and fails the test on a malformed-limit finding, which no caller expects.
func sizeDetails(t *testing.T, root string) []string {
	t.Helper()
	var details []string
	for _, finding := range sizeFindings(t, root) {
		if finding.Key != FindingColumnBodyOverLimit {
			t.Fatalf("%s: unexpected finding %s %q", root, finding.Key, finding.Detail)
		}
		details = append(details, finding.Detail)
	}
	return details
}
