package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/msg"
)

// requireOn writes a require_fields declaration into one column's anchor, which
// is frontmatter a person edits rather than anything a verb writes.
func requireOn(t *testing.T, root, column string, keys ...string) {
	t.Helper()
	got := runCLI(t, root, "path", column)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", column, got.code, got.errw)
	}
	path := strings.TrimSpace(got.out)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetSeq(bench.RequireFieldsKey, keys)
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the column anchor: %v", err)
	}
}

// checkReport is the machine form of check, decoded far enough to read what
// this file asserts.
type checkReport struct {
	Outcome  string          `json:"outcome"`
	Findings []bench.Finding `json:"findings"`
	Notices  []bench.Finding `json:"notices"`
}

// checkJSON runs check in the machine form and decodes its report, whatever
// its exit was.
func checkJSON(t *testing.T, root string) (checkReport, invocation) {
	t.Helper()
	got := runCLI(t, root, "--json", "check")
	var report checkReport
	if err := json.Unmarshal([]byte(got.out), &report); err != nil {
		t.Fatalf("decode check's answer: %v\n%s", err, got.out)
	}
	return report, got
}

// keysOf is the keys a list of findings carries, in order.
func keysOf(findings []bench.Finding) []string {
	keys := make([]string, 0, len(findings))
	for _, finding := range findings {
		keys = append(keys, finding.Key)
	}
	return keys
}

// TestANoticeIsPrintedAndNeverCounted is dinah-590/criteria/14 and
// specification section 11.2a: a workbench whose only report is the
// required-field notice checks clean, exits 0, prints the notice under its
// heading, and carries it in the machine form under notices with empty
// findings and outcome ok.
//
// Arming: appending the notice to Findings rather than Notices turns the
// exit 5, and the first assertion fails.
func TestANoticeIsPrintedAndNeverCounted(t *testing.T) {
	root := newBench(t)
	declareBlocksOn(t, root, constructionLevelsBlock, constructionFieldsBlock)
	requireOn(t, root, "doing", "task.trade")

	human := runCLI(t, root, "check")
	if human.code != 0 {
		t.Fatalf("check exits %d on a workbench whose only report is a notice:\n%s%s", human.code, human.out, human.errw)
	}
	english := msg.For(msg.Base)
	clean := english.T("check.clean")
	heading := english.T("check.notices")
	row := english.T("check.required-field-conditioned", "detail", "doing (task.trade)")
	for _, want := range []string{clean, heading, row} {
		if !strings.Contains(human.out, want) {
			t.Errorf("check does not print %q:\n%s", want, human.out)
		}
	}
	if strings.Index(human.out, clean) > strings.Index(human.out, heading) {
		t.Errorf("the notices heading is printed before the clean line:\n%s", human.out)
	}

	report, machine := checkJSON(t, root)
	if machine.code != 0 {
		t.Errorf("the machine form exits %d", machine.code)
	}
	if report.Outcome != "ok" {
		t.Errorf("the report's outcome is %q, wanted ok", report.Outcome)
	}
	if len(report.Findings) != 0 {
		t.Errorf("the report carries findings: %v", keysOf(report.Findings))
	}
	if got := keysOf(report.Notices); strings.Join(got, ",") != bench.NoticeRequiredFieldConditioned {
		t.Errorf("the report's notices are %v, wanted the one required-field notice", got)
	}

	// A move into the column is still refused for the card the condition
	// excludes, which is what the notice is about.
	own := fileCard(t, root, "Frame the extension")
	mustSet(t, root, own, "task.type", "own")
	assertRefusal(t, mustRefuse(t, root, "move", own, "doing"), "missing-field", "a move of an excluded card into the requiring column")
}

// TestAnOrphanedValueIsAFindingBesideTheNotice is the second case of
// specification section 11.2a: with one orphaned value added, check exits 5,
// the finding sits in findings at cleanup severity, and the notice stays in
// notices and out of findings.
func TestAnOrphanedValueIsAFindingBesideTheNotice(t *testing.T) {
	root := newBench(t)
	declareBlocksOn(t, root, constructionLevelsBlock, constructionFieldsBlock)
	requireOn(t, root, "doing", "task.trade")
	lighting := fileCard(t, root, "Garden lighting")
	mustSet(t, root, lighting, "task.type", "subcontracted")
	mustSet(t, root, lighting, "task.trade", "electrical")
	mustSet(t, root, lighting, "task.type", "own")

	report, machine := checkJSON(t, root)
	if machine.code != 5 {
		t.Errorf("check exits %d with an orphaned value on the workbench, wanted 5", machine.code)
	}
	if got := keysOf(report.Findings); strings.Join(got, ",") != bench.FindingInapplicableValue {
		t.Errorf("the findings are %v, wanted the one orphaned value", got)
	}
	if len(report.Findings) == 1 && bench.SeverityOf(report.Findings[0]) != bench.SeverityCleanup {
		t.Errorf("the orphaned value carries the severity %q", bench.SeverityOf(report.Findings[0]))
	}
	if got := keysOf(report.Notices); strings.Join(got, ",") != bench.NoticeRequiredFieldConditioned {
		t.Errorf("the notices are %v, wanted the one required-field notice", got)
	}
	if report.Outcome != "findings" {
		t.Errorf("the report's outcome is %q", report.Outcome)
	}
}

// TestTheAppliesWhenMigrationAdviceIsACommandThatWorks follows the sentence
// check.applies-when-below-format prints: a format-7 workbench carrying a
// condition is reported, and the command the sentence names stamps the
// format, after which check is clean.
func TestTheAppliesWhenMigrationAdviceIsACommandThatWorks(t *testing.T) {
	root := newBench(t)
	declareBlocksOn(t, root, constructionLevelsBlock, constructionFieldsBlock)
	dir := soleBenchDir(t, root)
	stampFormat(t, dir, bench.AppliesWhenFormat-1)

	reported := runCLI(t, root, "check")
	if reported.code != 5 {
		t.Fatalf("check exits %d on a format-7 workbench carrying a condition:\n%s%s", reported.code, reported.out, reported.errw)
	}
	// Without the confirmation the migration says what it would write and
	// writes nothing, and the finding stands.
	preview := runCLI(t, root, "check", "--migrate-applies-when")
	if want := msg.For(msg.Base).T("check.format-would-stamp", "from", "7", "format", "8"); !strings.Contains(preview.out, want) || preview.code != 5 {
		t.Errorf("the preview exits %d and does not print %q:\n%s", preview.code, want, preview.out)
	}
	advice := msg.For(msg.Base).T("check.applies-when-below-format", "detail", "7")
	if !strings.Contains(reported.out, advice) {
		t.Fatalf("check does not print %q:\n%s", advice, reported.out)
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
	if want := msg.For(msg.Base).T("check.format-stamped", "format", "8"); !strings.Contains(followed.out, want) {
		t.Errorf("the migration does not print %q:\n%s", want, followed.out)
	}
	text, err := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if !strings.Contains(text, "\nformat: 8\n") {
		t.Errorf("the anchor does not declare format 8 after the migration:\n%s", text)
	}
	if again := runCLI(t, root, "check"); again.code != 0 {
		t.Errorf("check exits %d after the migration:\n%s", again.code, again.out)
	}
	// A second confirmed run finds the format already declared and says so.
	current := runCLI(t, root, "check", "--migrate-applies-when", "--yes")
	if want := msg.For(msg.Base).T("check.format-current", "format", "8"); !strings.Contains(current.out, want) || current.code != 0 {
		t.Errorf("the second run exits %d and does not print %q:\n%s", current.code, want, current.out)
	}
}

// TestShowNamesEveryInapplicableSlotAndAKeptValue asserts specification
// section 6.2 on the human surface: show prints one line per inapplicable
// slot the card stores nothing for, in the order severity, priority, then
// declared keys, and every card line prints a kept value in place of the
// slot's ordinary line, naming the gate as unset where it stores nothing.
func TestShowNamesEveryInapplicableSlotAndAKeptValue(t *testing.T) {
	root := newBench(t)
	const levels = "levels:\n  severity:\n    values: [cosmetic, functional, safety]\n    applies_when:\n      field: task.type\n      is: [snag]\n  priority:\n    values: [later, now]\n    applies_when:\n      field: task.type\n      is: [snag]\n"
	declareBlocksOn(t, root, levels, constructionFieldsBlock)
	english := msg.For(msg.Base)
	untyped := fileCard(t, root, "Loose handrail")
	shown := runCLI(t, root, "show", untyped, "--fields", "card")
	if shown.code != 0 {
		t.Fatalf("show: %d %s", shown.code, shown.errw)
	}
	var want []string
	for _, slot := range []string{"severity", "priority", "task.trade"} {
		want = append(want, english.T("card.inapplicable.unset", "field", slot, "gate", "task.type"))
	}
	if !strings.Contains(shown.out, strings.Join(want, "\n")) {
		t.Errorf("show does not print the three unset lines in order:\n%s", shown.out)
	}
	mustSet(t, root, untyped, "task.type", "own")
	typed := runCLI(t, root, "show", untyped, "--fields", "card")
	if want := english.T("card.inapplicable", "field", "severity", "gate", "task.type", "value", "own"); !strings.Contains(typed.out, want) {
		t.Errorf("show does not print %q:\n%s", want, typed.out)
	}
	// A trade stored while the task was subcontracted, then left behind by
	// clearing the type, prints as kept with the gate unset on the card line
	// the clearing write itself answers with.
	lighting := fileCard(t, root, "Garden lighting")
	mustSet(t, root, lighting, "task.type", "subcontracted")
	mustSet(t, root, lighting, "task.trade", "electrical")
	cleared := mustSet(t, root, lighting, "task.type", "")
	if want := english.T("card.inapplicable.stored-unset", "field", "task.trade", "stored", "electrical", "gate", "task.type"); !strings.Contains(cleared.out, want) {
		t.Errorf("the clearing write's card line does not print %q:\n%s", want, cleared.out)
	}
	if !strings.Contains(cleared.errw, english.T("warn.inapplicable-value", "detail", "task.trade")) {
		t.Errorf("the clearing write does not warn:\n%s", cleared.errw)
	}
}
