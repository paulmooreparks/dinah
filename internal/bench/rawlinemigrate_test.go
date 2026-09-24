package bench

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// unspellableLevels is a levels member the block renderer cannot spell, so an
// import writes it as one raw line. It is spread over several lines the way a
// hand-written definition carries it, because the writer of the day quoted
// the bytes it was handed and the migration has to undo that escape before
// it compacts.
const unspellableLevels = "{\n  \"severity\": [{\"name\": \"major\", \"rank\": 2}]\n}"

// plantQuotedRawLines writes the quoted raw lines an import below
// RawLineFormat produced onto the fixture's two anchors, through the scalar
// writer that import used, and stamps the store at the format given. The
// workbench anchor gains a levels member and an unrecognised member, plus a
// quoted title that spells JSON and must be left alone; the column anchor
// gains a field_values member and an unrecognised member.
func plantQuotedRawLines(t *testing.T, root string, format int) {
	t.Helper()
	anchor := filepath.Join(root, WorkbenchAnchor)
	text, err := ReadText(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	fm, body := ParseAnchor(text)
	fm.Set(LevelsKey, unspellableLevels)
	fm.Set("acme.extra", `[1, 2]`)
	fm.Set("title", `{"not": "a member"}`)
	fm.Set("format", strconv.Itoa(format))
	if err := WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the anchor: %v", err)
	}
	column := filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor)
	text, err = ReadText(column)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	fm, body = ParseAnchor(text)
	fm.Set(FieldValuesKey, `{"acme.owner": "ops"}`)
	fm.Set("acme.note", `{"a": [{"b": 1}]}`)
	if err := WriteText(column, fm.Render(body)); err != nil {
		t.Fatalf("write the column anchor: %v", err)
	}
}

// exportedMember answers one member of a bench's export, or nil where the
// export carries no such member.
func exportedMember(t *testing.T, b *Bench, name string) json.RawMessage {
	t.Helper()
	encoded, err := b.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("unmarshal the export: %v", err)
	}
	return object[name]
}

// exportedColumnMember answers one member of the first column element of a
// bench's export.
func exportedColumnMember(t *testing.T, b *Bench, name string) json.RawMessage {
	t.Helper()
	columns := []map[string]json.RawMessage{}
	if err := json.Unmarshal(exportedMember(t, b, "columns"), &columns); err != nil {
		t.Fatalf("the export's columns member is not an array of objects: %v", err)
	}
	if len(columns) == 0 {
		t.Fatal("the export carries no column")
	}
	return columns[0][name]
}

// TestAQuotedRawLineIsReportedRewrittenAndReadAsTheJSON is the repair the
// second code review of dinah-593 asked for. A store an earlier import wrote
// carries a member's JSON inside quotes, which this build reads as a string;
// check names each such line on a store below RawLineFormat, the migration
// without confirmation names them and writes nothing, the confirmed run
// rewrites each to the bare compact line and stamps the format, and the
// reopened store reads and exports each member as the JSON the earlier build
// exported for it. A quoted title spelling JSON is a text key and is neither
// reported nor rewritten. A second confirmed run finds nothing and stamps
// nothing.
func TestAQuotedRawLineIsReportedRewrittenAndReadAsTheJSON(t *testing.T) {
	root := newFixture(t)
	plantQuotedRawLines(t, root, RawLineFormat-1)
	anchor := filepath.Join(root, WorkbenchAnchor)
	column := filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor)
	b := openConditioned(t, root)

	findings := findingsOf(t, b)
	if n := countOf(findings, FindingRawLineQuoted); n != 4 {
		t.Fatalf("check reports %d quoted raw lines, wanted 4:\n%+v", n, findings)
	}
	for _, want := range []struct{ path, key string }{
		{anchor, LevelsKey}, {anchor, "acme.extra"}, {column, FieldValuesKey}, {column, "acme.note"},
	} {
		found := false
		for _, finding := range findings {
			if finding.Key == FindingRawLineQuoted && finding.Path == want.path && finding.Detail == want.key {
				found = true
			}
		}
		if !found {
			t.Errorf("no %s finding names %s on %s", FindingRawLineQuoted, want.key, want.path)
		}
	}
	if findingWith(findings, FindingRawLineQuoted, "title") {
		t.Error("the quoted title is reported, and a text key is not a raw line")
	}
	// The premise: before the rewrite this build exports the member as the
	// string the quotes spell, which is the retyping the migration exists
	// for.
	if before := exportedMember(t, b, LevelsKey); jsonShape(before) != '"' {
		t.Errorf("before the rewrite the levels member exports as %s, and the premise of this test is that it exports as a string", before)
	}

	beforeAnchor, beforeColumn := readFile(t, anchor), readFile(t, column)
	preview, err := b.MigrateRawLines(false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !preview.Preview || preview.Stamped || preview.From != RawLineFormat-1 || len(preview.Rewritten) != 4 {
		t.Errorf("the preview answers %+v", preview)
	}
	if readFile(t, anchor) != beforeAnchor || readFile(t, column) != beforeColumn {
		t.Error("the preview wrote an anchor")
	}

	applied, err := b.MigrateRawLines(true)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if applied.Preview || !applied.Stamped || len(applied.Rewritten) != 4 {
		t.Errorf("the confirmed run answers %+v", applied)
	}
	if got := []string{applied.Rewritten[0].Key, applied.Rewritten[1].Key, applied.Rewritten[2].Key, applied.Rewritten[3].Key}; strings.Join(got, " ") != "levels acme.extra field_values acme.note" {
		t.Errorf("the rewritten lines are %v, wanted the anchors' own order", got)
	}
	afterAnchor := readFile(t, anchor)
	for _, line := range []string{
		"\nlevels: {\"severity\":[{\"name\":\"major\",\"rank\":2}]}\n",
		"\nacme.extra: [1,2]\n",
		"\ntitle: \"{\\\"not\\\": \\\"a member\\\"}\"\n",
		"\nformat: 9\n",
		"\nStanding text.\n",
	} {
		if !strings.Contains(afterAnchor, line) {
			t.Errorf("the rewritten anchor does not carry %q:\n%s", line, afterAnchor)
		}
	}
	afterColumn := readFile(t, column)
	for _, line := range []string{
		"\nfield_values: {\"acme.owner\":\"ops\"}\n",
		"\nacme.note: {\"a\":[{\"b\":1}]}\n",
		"\nColumn text.\n",
	} {
		if !strings.Contains(afterColumn, line) {
			t.Errorf("the rewritten column anchor does not carry %q:\n%s", line, afterColumn)
		}
	}

	reopened := openConditioned(t, root)
	if reopened.Format != RawLineFormat {
		t.Errorf("the store declares format %d after the migration, wanted %d", reopened.Format, RawLineFormat)
	}
	if n := countOf(findingsOf(t, reopened), FindingRawLineQuoted); n != 0 {
		t.Errorf("%d findings survive the migration", n)
	}
	if got := blockValue(reopened.FM, LevelsKey); !sameJSON(got, json.RawMessage(unspellableLevels)) {
		t.Errorf("the rewritten levels line reads back as %s, wanted the object", got)
	}
	// What the earlier build exported for each member is the JSON the
	// definition carried, since that build read the quoted line as the JSON
	// inside it; the export after the rewrite carries the same value.
	if got := exportedMember(t, reopened, LevelsKey); !sameJSON(got, json.RawMessage(unspellableLevels)) {
		t.Errorf("the export carries levels as %s, wanted the object the earlier build exported", got)
	}
	if got := exportedMember(t, reopened, "acme.extra"); !sameJSON(got, json.RawMessage(`[1,2]`)) {
		t.Errorf("the export carries acme.extra as %s, wanted the array", got)
	}
	if got := exportedMember(t, reopened, "title"); !sameJSON(got, mustMarshal(`{"not": "a member"}`)) {
		t.Errorf("the export carries the title as %s, wanted the text", got)
	}
	if got := exportedColumnMember(t, reopened, FieldValuesKey); !sameJSON(got, json.RawMessage(`{"acme.owner":"ops"}`)) {
		t.Errorf("the export carries the column's field_values as %s, wanted the object", got)
	}
	if got := exportedColumnMember(t, reopened, "acme.note"); !sameJSON(got, json.RawMessage(`{"a":[{"b":1}]}`)) {
		t.Errorf("the export carries the column's acme.note as %s, wanted the object", got)
	}

	again, err := reopened.MigrateRawLines(true)
	if err != nil {
		t.Fatalf("migrate again: %v", err)
	}
	if again.Stamped || len(again.Rewritten) != 0 {
		t.Errorf("a second confirmed run answers %+v on a store already at the format", again)
	}
	if readFile(t, anchor) != afterAnchor || readFile(t, column) != afterColumn {
		t.Error("a second confirmed run wrote an anchor")
	}
}

// TestTheRawLineMigrationIsANoOpAtTheFormat is the floor: a store already
// declaring RawLineFormat is neither read nor written, a quoted line on it is
// text by declaration, and check reports nothing. A store above the format
// cannot be opened by this build at all, so the floor is proved at the format
// itself, where the same comparison decides.
func TestTheRawLineMigrationIsANoOpAtTheFormat(t *testing.T) {
	root := newFixture(t)
	plantQuotedRawLines(t, root, RawLineFormat)
	anchor := filepath.Join(root, WorkbenchAnchor)
	column := filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor)
	b := openConditioned(t, root)
	if n := countOf(findingsOf(t, b), FindingRawLineQuoted); n != 0 {
		t.Errorf("check reports %d quoted raw lines on a store at the format", n)
	}
	beforeAnchor, beforeColumn := readFile(t, anchor), readFile(t, column)
	applied, err := b.MigrateRawLines(true)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if applied.Stamped || len(applied.Rewritten) != 0 || applied.From != RawLineFormat {
		t.Errorf("the confirmed run answers %+v on a store at the format", applied)
	}
	if readFile(t, anchor) != beforeAnchor || readFile(t, column) != beforeColumn {
		t.Error("the confirmed run wrote an anchor on a store at the format")
	}
	if got := blockValue(b.FM, LevelsKey); jsonShape(got) != '"' {
		t.Errorf("at the format the quoted levels line reads as %s, and a quoted scalar is text there", got)
	}
}

// TestQuotedRawJSONNamesOnlyAQuotedObjectOrArray pins the classification
// rule on the spellings it must and must not take: a quoted object or array,
// in either quote and with the writer's escape, is a raw line; a quoted
// number, boolean, null or text is not, nor is a bare line, a multi-line
// block, or quoted text that merely opens with a brace.
func TestQuotedRawJSONNamesOnlyAQuotedObjectOrArray(t *testing.T) {
	cases := []struct {
		lines   []string
		compact string
		raw     bool
	}{
		{[]string{`k: "{\"a\": 1}"`}, `{"a":1}`, true},
		{[]string{`k: "[1, 2]"`}, `[1,2]`, true},
		{[]string{`k: '{"a":[1]}'`}, `{"a":[1]}`, true},
		{[]string{`k: "{\n  \"a\": 1\n}"`}, `{"a":1}`, true},
		// A JSON string member carrying a newline (the JSON escape `\n`,
		// two characters, not the frontmatter's own escaped-newline
		// spelling) is itself wrapped in the frontmatter's quoting and
		// escaping when it is written, which doubles the backslash ahead
		// of the JSON escape's own `n`. Reading it back is the case that
		// broke unquote's three sequential ReplaceAll calls, filed at
		// dinah-594/comments/10: the first call matched the `\n` sitting
		// inside the doubled `\\n` before the second call reached the
		// doubled backslash, turning a literal backslash-n back into an
		// actual newline instead of leaving it as the two characters the
		// writer was handed.
		{[]string{`k: "{\"a\":\"x\\ny\"}"`}, `{"a":"x\ny"}`, true},
		{[]string{`k: "12"`}, "", false},
		{[]string{`k: "true"`}, "", false},
		{[]string{`k: "null"`}, "", false},
		{[]string{`k: "text"`}, "", false},
		{[]string{`k: "{not json"`}, "", false},
		{[]string{`k: {"a": 1}`}, "", false},
		{[]string{`k:`, `  a: 1`}, "", false},
		{[]string{`k: ""`}, "", false},
	}
	for _, c := range cases {
		compact, raw := quotedRawJSON(c.lines)
		if raw != c.raw || compact != c.compact {
			t.Errorf("%q reads as (%q, %v), wanted (%q, %v)", strings.Join(c.lines, "\\n"), compact, raw, c.compact, c.raw)
		}
	}
}
