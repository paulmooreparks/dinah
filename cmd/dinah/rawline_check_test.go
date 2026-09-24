package main

import (
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/msg"
)

// plantQuotedRawLine writes one member's JSON onto the workbench anchor
// through the scalar writer, which quotes it, as an import below storage
// format 9 did for a member the renderer could not spell.
func plantQuotedRawLine(t *testing.T, dir, key, value string) {
	t.Helper()
	anchor := filepath.Join(dir, bench.WorkbenchAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set(key, value)
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// TestTheRawLineMigrationAdviceIsACommandThatWorks follows the sentence
// check.raw-line-quoted prints: a format-8 workbench carrying a quoted raw
// line is reported, the preview names the line and writes nothing, the
// command the sentence names rewrites the line and stamps the format, after
// which check is clean and a second confirmed run says the format is current.
func TestTheRawLineMigrationAdviceIsACommandThatWorks(t *testing.T) {
	root := newBench(t)
	dir := soleBenchDir(t, root)
	plantQuotedRawLine(t, dir, "acme.levels", `{"a": [{"b": 1}]}`)
	stampFormat(t, dir, bench.RawLineFormat-1)
	english := msg.For(msg.Base)
	anchor := filepath.Join(dir, bench.WorkbenchAnchor)
	// The report names the anchor by the path the binary resolved, which on
	// macOS is the temporary directory with its symbolic link followed, so
	// the line is matched on its key and on the anchor's own tail rather
	// than on the whole path.
	rewriting := english.T("check.raw-line-rewriting", "key", "acme.levels", "path", "")
	anchorTail := filepath.Join(filepath.Base(dir), bench.WorkbenchAnchor)
	namesTheLine := func(out string) bool {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, rewriting) && strings.HasSuffix(line, anchorTail) {
				return true
			}
		}
		return false
	}

	reported := runCLI(t, root, "check")
	if reported.code != 5 {
		t.Fatalf("check exits %d on a format-8 workbench carrying a quoted raw line:\n%s%s", reported.code, reported.out, reported.errw)
	}
	advice := english.T("check.raw-line-quoted", "detail", "acme.levels")
	if !strings.Contains(reported.out, advice) {
		t.Fatalf("check does not print %q:\n%s", advice, reported.out)
	}

	// Without the confirmation the migration names the line, says what it
	// would stamp, and writes nothing, and the finding stands.
	before, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	preview := runCLI(t, root, "check", "--migrate-raw-lines")
	for _, want := range []string{
		english.TN("check.raw-lines-would-rewrite", 1),
		english.T("check.format-would-stamp", "from", "8", "format", "9"),
		advice,
	} {
		if !strings.Contains(preview.out, want) {
			t.Errorf("the preview does not print %q:\n%s", want, preview.out)
		}
	}
	if !namesTheLine(preview.out) {
		t.Errorf("the preview does not name acme.levels on %s:\n%s", anchorTail, preview.out)
	}
	if preview.code != 5 {
		t.Errorf("the preview exits %d, wanted 5 while the finding stands", preview.code)
	}
	if after, _ := bench.ReadText(anchor); after != before {
		t.Errorf("the preview wrote the anchor:\n%s", after)
	}

	// The command is cut out of the sentence rather than typed here, so
	// this test follows whatever the catalog says.
	opened := strings.Index(advice, "`")
	closed := strings.LastIndex(advice, "`")
	if opened < 0 || closed <= opened {
		t.Fatalf("the advice names no command in backticks: %q", advice)
	}
	command := strings.Fields(strings.TrimPrefix(advice[opened+1:closed], "dinah "))
	followed := runCLI(t, root, command...)
	if followed.code != 0 {
		t.Fatalf("following the advice %v exits %d:\n%s%s", command, followed.code, followed.out, followed.errw)
	}
	for _, want := range []string{
		english.TN("check.raw-lines-rewritten", 1),
		english.T("check.format-stamped", "format", "9"),
	} {
		if !strings.Contains(followed.out, want) {
			t.Errorf("the migration does not print %q:\n%s", want, followed.out)
		}
	}
	if !namesTheLine(followed.out) {
		t.Errorf("the migration does not name acme.levels on %s:\n%s", anchorTail, followed.out)
	}
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if !strings.Contains(text, "\nacme.levels: {\"a\":[{\"b\":1}]}\n") {
		t.Errorf("the anchor does not carry the bare compact line after the migration:\n%s", text)
	}
	if !strings.Contains(text, "\nformat: 9\n") {
		t.Errorf("the anchor does not declare format 9 after the migration:\n%s", text)
	}
	if again := runCLI(t, root, "check"); again.code != 0 {
		t.Errorf("check exits %d after the migration:\n%s", again.code, again.out)
	}
	// The export carries the member as the object the earlier build
	// exported for it, which is what the rewrite is for.
	exported := runCLI(t, root, "export")
	if exported.code != 0 || !strings.Contains(exported.out, `"acme.levels": {`) {
		t.Errorf("export exits %d and does not carry acme.levels as an object:\n%s", exported.code, exported.out)
	}
	// A second confirmed run finds the format already declared and says so.
	current := runCLI(t, root, "check", "--migrate-raw-lines", "--yes")
	if want := english.T("check.format-current", "format", "9"); !strings.Contains(current.out, want) || current.code != 0 {
		t.Errorf("the second run exits %d and does not print %q:\n%s", current.code, want, current.out)
	}
}
