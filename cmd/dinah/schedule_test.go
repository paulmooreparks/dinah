package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// The terminal reads the machine's clock, which no variable or flag moves, so
// every date below is written relative to today as a workbench declaring no
// zone reads it: the UTC calendar.

// dayFrom is the calendar date n days from today in UTC, as YYYY-MM-DD.
func dayFrom(n int) string {
	return time.Now().UTC().AddDate(0, 0, n).Format(bench.FieldDateLayout)
}

// datedCard files a card at doing with the dates named, as flag and value
// pairs of dinah add, and answers its reference.
func datedCard(t *testing.T, root, title string, flags ...string) string {
	t.Helper()
	ref := fileCard(t, root, title, flags...)
	carryToDoing(t, root, ref)
	return ref
}

// uncommittedCard files a card with the dates named and leaves it in Intake,
// before the workbench's commitment column, which on the flow dinah init
// creates is Doing. Since dinah-608's decision 10 a card standing at or past
// the commitment column has started, so the two conditions start_by drives
// read only on a card like this one.
func uncommittedCard(t *testing.T, root, title string, flags ...string) string {
	t.Helper()
	return fileCard(t, root, title, flags...)
}

// declareScheduleOn writes a dinah.schedule block, given whole, into the
// workbench anchor.
func declareScheduleOn(t *testing.T, root, block string) {
	t.Helper()
	path := filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.ScheduleKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// TestShowPrintsEachDateWithItsCondition is the text half of
// dinah-605/criteria/13: one line per date, each carrying the phrase of the
// highest condition the date drives, in the one-day, many-day and today forms,
// and bare lines on a card in a done column.
//
// The start_by cases file their card in Intake since dinah-608's decision 10:
// a card in Doing, the commitment column, has started, so its start_by line is
// bare, which the last case pins.
func TestShowPrintsEachDateWithItsCondition(t *testing.T) {
	root := newBench(t)
	english := msg.For(msg.Base)
	line := func(key string, n int, date string) string {
		return english.TN(key, n, "date", date, "n", strconv.Itoa(n))
	}
	cases := []struct {
		flag, date string
		want       string
	}{
		{"--start-after", dayFrom(3), line("card.start-after.not-yet", 3, dayFrom(3))},
		{"--start-after", dayFrom(1), line("card.start-after.not-yet", 1, dayFrom(1))},
		{"--start-by", dayFrom(-2), line("card.start-by.late", 2, dayFrom(-2))},
		{"--start-by", dayFrom(-1), line("card.start-by.late", 1, dayFrom(-1))},
		{"--start-by", dayFrom(4), line("card.start-by.soon", 4, dayFrom(4))},
		{"--start-by", dayFrom(1), line("card.start-by.soon", 1, dayFrom(1))},
		{"--start-by", dayFrom(0), english.T("card.start-by.soon.today", "date", dayFrom(0))},
		{"--start-by", dayFrom(30), english.T("card.start-by", "date", dayFrom(30))},
		{"--due", dayFrom(-3), line("card.due.overdue", 3, dayFrom(-3))},
		{"--due", dayFrom(-1), line("card.due.overdue", 1, dayFrom(-1))},
		{"--due", dayFrom(5), line("card.due.soon", 5, dayFrom(5))},
		{"--due", dayFrom(1), line("card.due.soon", 1, dayFrom(1))},
		{"--due", dayFrom(0), english.T("card.due.soon.today", "date", dayFrom(0))},
		{"--due", dayFrom(30), english.T("card.due", "date", dayFrom(30))},
	}
	for _, c := range cases {
		file := datedCard
		if c.flag == "--start-by" {
			file = uncommittedCard
		}
		ref := file(t, root, "dated "+c.flag+" "+c.date, c.flag, c.date)
		shown := mustRunHere(t, root, "show", ref, "--fields", "card")
		if !strings.Contains(shown.out, "\n"+c.want+"\n") {
			t.Errorf("%s %s: show does not print %q:\n%s", c.flag, c.date, c.want, shown.out)
		}
	}
	committed := datedCard(t, root, "late but committed", "--start-by", dayFrom(-2))
	if shown := mustRunHere(t, root, "show", committed, "--fields", "card"); !strings.Contains(shown.out, "\n"+english.T("card.start-by", "date", dayFrom(-2))+"\n") {
		t.Errorf("a card standing in the commitment column prints a start_by condition:\n%s", shown.out)
	}
	for _, want := range []string{"(not yet, 3 days to go)", "(not yet, 1 day to go)", "(late to start by 2 days)", "(start today)", "(overdue by 1 day)", "(due today)"} {
		found := false
		for _, c := range cases {
			if strings.Contains(c.want, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no case printed %q, so the catalog's English is not what the specification's table says", want)
		}
	}
	finished := fileCard(t, root, "finished late", "--start-after", dayFrom(2), "--start-by", dayFrom(3), "--due", dayFrom(4))
	mustRunHere(t, root, "move", finished, "done")
	shown := mustRunHere(t, root, "show", finished, "--fields", "card")
	for _, want := range []string{
		english.T("card.start-after", "date", dayFrom(2)),
		english.T("card.start-by", "date", dayFrom(3)),
		english.T("card.due", "date", dayFrom(4)),
	} {
		if !strings.Contains(shown.out, "\n"+want+"\n") {
			t.Errorf("a finished card does not print the bare line %q:\n%s", want, shown.out)
		}
	}
	if strings.Contains(shown.out, "(") {
		t.Errorf("a finished card prints a condition:\n%s", shown.out)
	}
	// The day count is taken from the day the view's conditions were
	// computed against, not from a second reading of the clock. A session
	// with no workbench open has no clock to read, so only the view's own
	// day can give it the count.
	s := &session{r: english}
	view := &verb.CardView{Due: "2026-10-07", Schedule: []string{contract.ScheduleOverdue}, ScheduleDay: bench.DateOf(2026, time.October, 10)}
	if got, want := s.scheduleLines(view), []string{line("card.due.overdue", 3, "2026-10-07")}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the lines drawn against the view's day read %q, want %q", got, want)
	}
}

// TestTheMachineFormsCarryTheDates is the machine half of
// dinah-605/criteria/13: the JSON card view carries the four members after
// the route, and the compact card record carries them after route, with the
// conditions joined by commas, under compact version 6. The card stands in
// Intake since dinah-608's decision 10, where late_start still reads.
func TestTheMachineFormsCarryTheDates(t *testing.T) {
	root := newBench(t)
	ref := uncommittedCard(t, root, "late and due", "--start-by", dayFrom(-2), "--due", dayFrom(3))
	shown := mustRunHere(t, root, "show", ref, "--json")
	order := []string{`"state"`, `"start_by"`, `"due"`, `"schedule": [`, `"late_start"`, `"due_soon"`, `"revision"`}
	at := -1
	for _, member := range order {
		next := strings.Index(shown.out, member)
		if next < 0 || next < at {
			t.Fatalf("the JSON view does not carry %s after the member before it:\n%s", member, shown.out)
		}
		at = next
	}
	compact := mustRunHere(t, root, "--format", "compact", "list", "intake")
	if !strings.HasPrefix(compact.out, "fmt|compact|6\n") {
		t.Errorf("the compact payload opens on %q", strings.SplitN(compact.out, "\n", 2)[0])
	}
	found := false
	for _, record := range strings.Split(compact.out, "\n") {
		fields := strings.Split(record, "|")
		if fields[0] != "card" {
			continue
		}
		found = true
		if got := strings.Join(fields[9:14], "|"); got != "||"+dayFrom(-2)+"|"+dayFrom(3)+"|late_start,due_soon" {
			t.Errorf("the card record carries route and the dates as %q: %s", got, record)
		}
	}
	if !found {
		t.Fatalf("the compact listing carries no card record:\n%s", compact.out)
	}
}

// scheduleTable is a listing's rows as heading-to-cell maps, read by the
// column offsets the heading row fixes, so a case asserts the Schedule cell of
// a named card without depending on the widths the lay-out chose. Every
// heading these listings draw is one word, which is what lets a run of
// spaces before a word mark where a column starts.
func scheduleTable(t *testing.T, out string) (headings []string, rows map[string]map[string]string) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("the listing draws no table:\n%s", out)
	}
	header := lines[0]
	var starts []int
	for i := 0; i < len(header); i++ {
		if header[i] != ' ' && (i == 0 || header[i-1] == ' ') {
			starts = append(starts, i)
		}
	}
	for c, start := range starts {
		end := len(header)
		if c+1 < len(starts) {
			end = starts[c+1]
		}
		headings = append(headings, strings.TrimSpace(header[start:end]))
	}
	rows = map[string]map[string]string{}
	for _, line := range lines[2:] {
		row := map[string]string{}
		for c, start := range starts {
			end := len(line)
			if c+1 < len(starts) && starts[c+1] < len(line) {
				end = starts[c+1]
			}
			if start < len(line) {
				row[headings[c]] = strings.TrimSpace(line[start:end])
			}
		}
		rows[row["Card"]] = row
	}
	return headings, rows
}

// TestTheListingsCarryAScheduleCell is dinah-605/criteria/14: list <column>,
// list cards and query each draw a Schedule column before Title, whose cell
// is the highest condition with its date, the not-yet phrase after a comma
// where the card also holds not_yet, and the due date where nothing holds.
//
// The late and the starting cards stand in Intake since dinah-608's decision
// 10, and a card past its start_by in Doing, the commitment column, pins that
// it reads neither condition there.
func TestTheListingsCarryAScheduleCell(t *testing.T) {
	root := newBench(t)
	// A window wide enough that no cell is cut, so the comparison reads what
	// the listing composed rather than what the lay-out kept of it.
	t.Setenv("COLUMNS", "200")
	english := msg.For(msg.Base)
	soon := datedCard(t, root, "due soon and not yet", "--start-after", dayFrom(2), "--due", dayFrom(4))
	overdue := datedCard(t, root, "overdue and not yet", "--start-after", dayFrom(2), "--due", dayFrom(-1))
	late := uncommittedCard(t, root, "late to start", "--start-by", dayFrom(-2))
	starting := uncommittedCard(t, root, "start soon", "--start-by", dayFrom(3))
	committed := datedCard(t, root, "late but committed", "--start-by", dayFrom(-2))
	waiting := datedCard(t, root, "not yet", "--start-after", dayFrom(9))
	distant := datedCard(t, root, "due far off", "--due", dayFrom(40))
	plain := datedCard(t, root, "no dates")
	want := map[string]string{
		soon:     english.T("schedule.cell.and-not-yet", "cell", english.T("schedule.cell.due-soon", "date", dayFrom(4)), "date", dayFrom(2)),
		overdue:  english.T("schedule.cell.and-not-yet", "cell", english.T("schedule.cell.overdue", "date", dayFrom(-1)), "date", dayFrom(2)),
		late:     english.T("schedule.cell.late-start", "date", dayFrom(-2)),
		starting: english.T("schedule.cell.start-soon", "date", dayFrom(3)),
		waiting:  english.T("schedule.cell.not-yet", "date", dayFrom(9)),
		distant:  english.T("schedule.cell.due", "date", dayFrom(40)),
		plain:    "",
		// A card in the commitment column has started, and a start_by that
		// has passed drives no condition on it and names no due date.
		committed: "",
	}
	inIntake := map[string]bool{late: true, starting: true}
	if want[soon] != "due soon "+dayFrom(4)+", not before "+dayFrom(2) {
		t.Errorf("the English cell for due soon and not yet reads %q", want[soon])
	}
	for _, argv := range [][]string{{"list", "doing"}, {"list", "cards"}, {"query", "column:doing"}, {"query", "column:intake"}} {
		got := mustRunHere(t, root, argv...)
		headings, rows := scheduleTable(t, got.out)
		joined := strings.Join(headings, " ")
		if !strings.Contains(joined, "Schedule Title") {
			t.Errorf("%v draws the headings %q, without Schedule directly before Title", argv, joined)
		}
		for ref, cell := range want {
			if argv[1] == "doing" || argv[1] == "column:doing" {
				if inIntake[ref] {
					continue
				}
			}
			if argv[1] == "column:intake" && !inIntake[ref] {
				continue
			}
			if rows[ref]["Schedule"] != cell {
				t.Errorf("%v draws %s's Schedule cell as %q, want %q", argv, ref, rows[ref]["Schedule"], cell)
			}
		}
	}
}

// TestAnUndatedWorkbenchListsAsItDidBefore is the byte-for-byte half of
// dinah-605/criteria/14: on a workbench whose cards carry no date the three
// listings print the tables trunk printed before this card, with no Schedule
// column. The expected text is the output of the build before dinah-605 on
// this fixture.
func TestAnUndatedWorkbenchListsAsItDidBefore(t *testing.T) {
	root := newBench(t)
	mustRunHere(t, root, "add", "First card")
	mustRunHere(t, root, "add", "Second card")
	carryToDoing(t, root, "fx-2")
	for argv, want := range map[string]string{
		"list doing": "  Card  Standing  Title\n  ----  --------  -----------\n  fx-2  ready     Second card\n",
		"list cards": "  Card  Column  Standing  Title\n  ----  ------  --------  -----------\n  fx-1  Intake  ready     First card\n  fx-2  Doing   ready     Second card\n",
		"query":      "  Card  Column  Standing  Title\n  ----  ------  --------  -----------\n  fx-1  Intake  ready     First card\n  fx-2  Doing   ready     Second card\n",
	} {
		if got := mustRunHere(t, root, strings.Fields(argv)...); got.out != want {
			t.Errorf("%s prints:\n%s\nwant:\n%s", argv, got.out, want)
		}
	}
}

// TestNextAndPrimeNameTheDate is the text half of dinah-605/criteria/5 and
// criteria/20: next and prime print next.not-yet with the date for a column
// whose only ready work waits on one, and the compact off record carries the
// flag and the date as its two fields after ready_count.
func TestNextAndPrimeNameTheDate(t *testing.T) {
	root := newBench(t)
	english := msg.For(msg.Base)
	datedCard(t, root, "later", "--start-after", dayFrom(5))
	datedCard(t, root, "sooner", "--start-after", dayFrom(2))
	notYet := english.T("next.not-yet", "date", dayFrom(2))
	next := mustRunHere(t, root, "next", "--column", "doing")
	if !strings.Contains(next.out, notYet) || strings.Contains(next.out, english.T("next.no-taker")) {
		t.Errorf("next does not print %q:\n%s", notYet, next.out)
	}
	prime := mustRunHere(t, root, "prime")
	if !strings.Contains(prime.out, "Doing: "+notYet) {
		t.Errorf("prime does not list the column with %q:\n%s", notYet, prime.out)
	}
	compact := mustRunHere(t, root, "--format", "compact", "next", "--column", "doing")
	found := false
	for _, record := range strings.Split(compact.out, "\n") {
		fields := strings.Split(record, "|")
		if fields[0] != "off" {
			continue
		}
		found = true
		if len(fields) != 13 || fields[8] != "2" || fields[9] != "1" || fields[10] != dayFrom(2) || fields[11] != "" || fields[12] != "" {
			t.Errorf("the off record reads %q, want ready_count 2, not_yet 1 and %s, and no waiting", record, dayFrom(2))
		}
	}
	if !found {
		t.Fatalf("the compact next carries no off record:\n%s", compact.out)
	}
	var primed verb.Primer
	if err := json.Unmarshal([]byte(mustRunHere(t, root, "prime", "--json").out), &primed); err != nil {
		t.Fatalf("prime --json: %v", err)
	}
	if len(primed.Ready) != 1 || !primed.Ready[0].NotYet || primed.Ready[0].StartableFrom != dayFrom(2) {
		t.Errorf("the JSON ready list is %+v", primed.Ready)
	}
}

// TestTheAboveTierAnswerOutranksTheDateInText is the render-chain half of
// dinah-605/criteria/5 and criteria/20: a column holding one card above the
// caller's tier and one not yet startable prints next.above-tier in both next
// and prime, because a more senior caller would get work now.
func TestTheAboveTierAnswerOutranksTheDateInText(t *testing.T) {
	var out strings.Builder
	s := &session{out: &out, errw: &out, r: msg.For(msg.Base)}
	offers := []verb.Offer{{Column: "c1", Title: "Doing", AboveTier: true, NotYet: true, StartableFrom: "2026-10-06", ReadyCount: 2}}
	s.renderOffers(offers)
	s.renderPrimeReady(offers)
	text := out.String()
	if !strings.Contains(text, s.r.T("next.above-tier")) || strings.Contains(text, s.r.T("next.not-yet", "date", "2026-10-06")) {
		t.Errorf("both flags printed:\n%s", text)
	}
	out.Reset()
	offers[0].AboveTier = false
	s.renderOffers(offers)
	s.renderPrimeReady(offers)
	if strings.Count(out.String(), s.r.T("next.not-yet", "date", "2026-10-06")) != 2 {
		t.Errorf("not-yet alone printed:\n%s", out.String())
	}
	// A session with no workbench open knows of no done column, so a card
	// holding no condition shows its due date.
	if got := s.scheduleCell(&verb.CardView{Due: "2026-10-10"}); got != s.r.T("schedule.cell.due", "date", "2026-10-10") {
		t.Errorf("with no workbench open the cell reads %q", got)
	}
}

// TestCheckPrintsTheScheduleReports is the terminal half of
// dinah-605/criteria/15: the block findings and the zone notice each end in
// the workbench's workbench.md path, and the notice leaves the exit status at
// 0 on an otherwise clean workbench.
func TestCheckPrintsTheScheduleReports(t *testing.T) {
	english := msg.For(msg.Base)
	root := newBench(t)
	datedCard(t, root, "dated", "--due", dayFrom(3))
	anchor := filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor)
	// The path the tool prints is the one discovery resolved, which on a host
	// whose temporary directory sits behind a symbolic link, as macOS's
	// /var does, is the link's target rather than the spelling the test
	// built. Either spelling names the one file.
	anchors := []string{anchor}
	if resolved, err := filepath.EvalSymlinks(anchor); err == nil && resolved != anchor {
		anchors = append(anchors, resolved)
	}
	endsInAnchor := func(out, sentence string) bool {
		for _, path := range anchors {
			if strings.Contains(out, sentence+" ("+path+")") {
				return true
			}
		}
		return false
	}
	clean := runCLI(t, root, "check")
	notice := english.T("check.schedule-zone-undeclared", "detail", "")
	if clean.code != 0 || !endsInAnchor(clean.out, notice) || !strings.Contains(clean.out, english.T("check.notices")) {
		t.Errorf("check exits %d and does not print the notice %q followed by %v under its heading:\n%s", clean.code, notice, anchors, clean.out)
	}
	if !strings.Contains(notice, "dinah.schedule") || !strings.Contains(notice, "time_zone") {
		t.Errorf("the notice does not name the key and the member: %q", notice)
	}
	declareScheduleOn(t, root, "dinah.schedule:\n  time_zone: Mars/Olympus\n  zone: UTC\n")
	reported := runCLI(t, root, "check")
	for _, want := range []string{
		english.T("check.schedule-malformed", "detail", "dinah.schedule time_zone Mars/Olympus"),
		english.T("check.schedule-member-unknown", "detail", "zone"),
	} {
		if !endsInAnchor(reported.out, want) {
			t.Errorf("check does not print %q followed by %v:\n%s", want, anchors, reported.out)
		}
	}
	if reported.code == 0 {
		t.Errorf("an unusable zone left check at exit 0:\n%s", reported.out)
	}
	declareScheduleOn(t, root, "dinah.schedule:\n  time_zone: UTC\n")
	if zoned := runCLI(t, root, "check"); zoned.code != 0 || strings.Contains(zoned.out, notice) {
		t.Errorf("a declared zone still draws the notice or a finding, exit %d:\n%s", zoned.code, zoned.out)
	}
}

// TestTheScheduleMigrationAdviceIsACommandThatWorks follows the sentence
// check.schedule-below-format prints, on the pattern of the applies_when
// migration's own test: a format-9 workbench with a dated card is reported,
// the preview writes nothing, and the command the sentence names stamps the
// format, after which the finding is gone. It is also the terminal half of
// dinah-605/criteria/10: the date write before it left the format at 9, and
// a store below format 7 is refused naming the designation migration.
func TestTheScheduleMigrationAdviceIsACommandThatWorks(t *testing.T) {
	english := msg.For(msg.Base)
	root := newBench(t)
	dir := soleBenchDir(t, root)
	stampFormat(t, dir, bench.ScheduleFormat-1)
	if clean := runCLI(t, root, "check"); clean.code != 0 {
		t.Fatalf("check exits %d on a format-9 workbench with no dated card:\n%s", clean.code, clean.out)
	}
	datedCard(t, root, "dated", "--start-after", dayFrom(3))
	text, _ := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor))
	if !strings.Contains(text, "\nformat: 9\n") {
		t.Fatalf("a date write moved the format off 9:\n%s", text)
	}
	advice := english.T("check.schedule-below-format", "detail", "9")
	reported := runCLI(t, root, "check")
	if reported.code != 5 || !strings.Contains(reported.out, advice) {
		t.Fatalf("check exits %d and does not print %q:\n%s", reported.code, advice, reported.out)
	}
	before, _ := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor))
	preview := runCLI(t, root, "check", "--migrate-schedule")
	if want := english.T("check.format-would-stamp", "from", "9", "format", "10"); !strings.Contains(preview.out, want) {
		t.Errorf("the preview does not print %q:\n%s", want, preview.out)
	}
	if after, _ := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor)); after != before {
		t.Error("the preview wrote the anchor")
	}
	opened := strings.Index(advice, "`")
	closed := strings.LastIndex(advice, "`")
	if opened < 0 || closed <= opened {
		t.Fatalf("the advice names no command in backticks: %q", advice)
	}
	command := strings.Fields(strings.TrimPrefix(advice[opened+1:closed], "dinah "))
	followed := runCLI(t, root, command...)
	if followed.code != 0 || !strings.Contains(followed.out, english.T("check.format-stamped", "format", "10")) {
		t.Fatalf("following the advice %v exits %d:\n%s%s", command, followed.code, followed.out, followed.errw)
	}
	after, _ := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor))
	if after != strings.Replace(before, "\nformat: 9\n", "\nformat: 10\n", 1) {
		t.Errorf("the stamp wrote more than the format line:\n%s", after)
	}
	if again := runCLI(t, root, "check"); again.code != 0 || strings.Contains(again.out, advice) {
		t.Errorf("check exits %d after the stamp:\n%s", again.code, again.out)
	}
	// A second confirmed run finds the format already declared and says so.
	current := runCLI(t, root, "check", "--migrate-schedule", "--yes")
	if want := english.T("check.format-current", "format", "10"); !strings.Contains(current.out, want) || current.code != 0 {
		t.Errorf("the second run exits %d and does not print %q:\n%s", current.code, want, current.out)
	}

	old := newBench(t)
	stampFormat(t, soleBenchDir(t, old), bench.DesignationFormat-1)
	refused := runCLI(t, old, "check", "--migrate-schedule", "--yes")
	if refused.code == 0 || !strings.Contains(refused.errw, "--migrate-designations") {
		t.Errorf("a format-6 store answered %d:\n%s%s", refused.code, refused.out, refused.errw)
	}
}

// TestInitCreatesTheCurrentFormat is the last part of dinah-605/criteria/10.
func TestInitCreatesTheCurrentFormat(t *testing.T) {
	root := newBench(t)
	text, err := bench.ReadText(filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor))
	if err != nil || !strings.Contains(text, "\nformat: 11\n") {
		t.Errorf("init wrote %v:\n%s", err, text)
	}
}

// TestTheScheduleBlockTravelsThroughExport is dinah-605/criteria/16: the block
// rides export as a member of the workbench object, and init --from on the
// export writes it back, so both workbenches answer one set of settings.
func TestTheScheduleBlockTravelsThroughExport(t *testing.T) {
	root := newBench(t)
	declareScheduleOn(t, root, "dinah.schedule:\n  time_zone: Asia/Singapore\n  soon_days: 3\n")
	exported := mustRunHere(t, root, "export")
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(exported.out), &object); err != nil {
		t.Fatalf("the export is not JSON: %v", err)
	}
	var member struct {
		TimeZone string `json:"time_zone"`
		SoonDays int    `json:"soon_days"`
	}
	if err := json.Unmarshal(object[bench.ScheduleKey], &member); err != nil || member.TimeZone != "Asia/Singapore" || member.SoonDays != 3 {
		t.Fatalf("the export carries %s as %s (%v)", bench.ScheduleKey, object[bench.ScheduleKey], err)
	}
	file := filepath.Join(t.TempDir(), "definition.json")
	if err := os.WriteFile(file, []byte(exported.out), 0o644); err != nil {
		t.Fatalf("write the export: %v", err)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustRunHere(t, clone, "init", "--from", file, "--slug", "fx")
	for _, dir := range []string{soleBenchDir(t, root), soleBenchDir(t, clone)} {
		opened, err := bench.Open(dir)
		if err != nil {
			t.Fatalf("open %s: %v", dir, err)
		}
		settings, defects := opened.Schedule()
		if settings.ZoneName != "Asia/Singapore" || settings.SoonDays != 3 || len(defects) != 0 {
			t.Errorf("%s answers %+v %+v", dir, settings, defects)
		}
	}
}

// TestAddTakesTheThreeDateFlags is the terminal half of dinah-605/criteria/9
// and criteria/8: the flags store their values, a malformed one refuses the
// filing and leaves no card, and an out-of-order filing warns on stderr.
func TestAddTakesTheThreeDateFlags(t *testing.T) {
	root := newBench(t)
	added := mustRunHere(t, root, "add", "a trip", "--start-after", "2026-10-06", "--start-by", "2026-10-08", "--due", "2026-10-10")
	ref := strings.Fields(added.out)[0]
	for field, want := range map[string]string{bench.StartAfterField: "2026-10-06", bench.StartByField: "2026-10-08", bench.DueField: "2026-10-10"} {
		if got := mustRunHere(t, root, "get", ref, field); strings.TrimSpace(got.out) != want {
			t.Errorf("get %s reads %q", field, got.out)
		}
	}
	refused := runCLI(t, root, "add", "a bad trip", "--due", "2026-10-1")
	if refused.code != 2 || !strings.HasPrefix(refused.errw, "malformed") {
		t.Errorf("a malformed due date answered %d %s", refused.code, refused.errw)
	}
	if listed := mustRunHere(t, root, "list", "cards"); strings.Contains(listed.out, "a bad trip") {
		t.Errorf("the refused filing left a card:\n%s", listed.out)
	}
	warned := mustRunHere(t, root, "add", "a backwards trip", "--start-after", "2026-10-20", "--due", "2026-10-01")
	want := msg.For(msg.Base).T("warn.schedule-order", "detail", "start_after 2026-10-20 is after due 2026-10-01")
	if !strings.Contains(warned.errw, want) {
		t.Errorf("the out-of-order filing does not warn %q on stderr:\n%s", want, warned.errw)
	}
	help := mustRunHere(t, root, "help", "add")
	for _, flag := range []string{"--start-after <date>", "--start-by <date>", "--due <date>"} {
		if !strings.Contains(help.out, flag) {
			t.Errorf("dinah help add does not list %s:\n%s", flag, help.out)
		}
	}
}
