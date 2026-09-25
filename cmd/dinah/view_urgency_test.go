package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/mcp"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// rankedDefinition is a flow with an operator-owned station and both level
// sets, which is what every term of the urgency order needs to be reachable
// at the terminal.
const rankedDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Ranked",
  "levels": { "severity": ["trivial", "minor", "major", "critical"], "priority": ["later", "soon", "next", "now"] },
  "columns": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Doing", "kind": "work" },
    { "id": "b00000000003", "title": "Review", "kind": "work", "operator_owned": true },
    { "id": "b00000000004", "title": "Done", "kind": "done" }
  ]
}`

// everythingView ranks every live card by urgency, so a term can be read on
// a card the operator's agenda would not hold.
const everythingView = bench.ViewsKey + ":\n  everything:\n    order: urgency\n    sections:\n      - query: state:ready,active,blocked\n" +
	"  plain:\n    sections:\n      - query: state:ready\n"

// declareUrgencyIn writes a dinah.urgency block, given whole with its key
// line, into a workbench's definition.
func declareUrgencyIn(t *testing.T, root, block string) {
	t.Helper()
	path := filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.UrgencyKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// backdate moves every stamp in a card's journal back by the given span, so a
// card stands in its column as long as a test needs without the test waiting.
func backdate(t *testing.T, root, ref string, by time.Duration) {
	t.Helper()
	anchor := strings.TrimSpace(mustRunHere(t, root, "path", ref).out)
	journal := filepath.Join(filepath.Dir(anchor), bench.JournalName)
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal of %s: %v", ref, err)
	}
	stamp := regexp.MustCompile(`"ts":"([^"]+)"`)
	shifted := stamp.ReplaceAllStringFunc(string(data), func(match string) string {
		at := bench.ParseStamp(stamp.FindStringSubmatch(match)[1])
		return `"ts":"` + bench.Stamp(at.Add(-by)) + `"`
	})
	if err := os.WriteFile(journal, []byte(shifted), 0o644); err != nil {
		t.Fatalf("write the journal of %s: %v", ref, err)
	}
}

// rankedFixture builds nine cards whose scores exercise every term: two at
// the operator's own station, one of them a day and a half in its column; a
// blocked card under a negative weight; a card carrying the operator's
// question; a card whose claim lapsed; two at intake; a card ten days in its
// column, past the age cap; and a card carrying a blocks link. It answers the
// workbench and the score each card should draw.
func rankedFixture(t *testing.T) (string, map[string]string) {
	t.Helper()
	root := newBenchFromDefinition(t, rankedDefinition)
	// The window is set wide enough for the ranked table to draw as a table
	// rather than as a stack, which is what the tests reading its cells need.
	t.Setenv("COLUMNS", "200")
	declareViewsIn(t, root, everythingView)
	declareUrgencyIn(t, root, bench.UrgencyKey+":\n  blocked: -1.5\n")
	for i := 1; i <= 9; i++ {
		mustRunHere(t, root, "add", "Card number "+strconv.Itoa(i))
	}
	for _, ref := range []string{"fx-1", "fx-2"} {
		mustRunHere(t, root, "move", ref, "review")
	}
	for _, ref := range []string{"fx-3", "fx-4", "fx-5", "fx-8", "fx-9"} {
		mustRunHere(t, root, "move", ref, "doing")
	}
	mustRunHere(t, root, "set", "fx-1", "priority", "now")
	mustRunHere(t, root, "set", "fx-1", "severity", "major")
	mustRunHere(t, root, "set", "fx-2", "priority", "next")
	mustRunHere(t, root, "set", "fx-2", "severity", "major")
	mustRunHere(t, root, "block", "fx-3", "waiting on a supplier", "--kind", "supplier")
	mustRunHere(t, root, "file", "fx-4", "open_question", "Which vendor?", "--owner", "operator")
	mustRunHere(t, root, "set", "fx-4", "severity", "critical")
	mustRunHere(t, root, "set", "fx-6", "priority", "later")
	mustRunHere(t, root, "set", "fx-6", "severity", "minor")
	mustRunHere(t, root, "set", "fx-9", "priority", "soon")
	mustRunHere(t, root, "link", "fx-9", "blocks", "fx-7")
	mustRunHere(t, root, "claim", "fx-5", "--expires", "1s")
	backdate(t, root, "fx-1", 36*time.Hour)
	backdate(t, root, "fx-8", 10*24*time.Hour)
	time.Sleep(2 * time.Second)
	return root, map[string]string{
		"fx-1": "18.5", "fx-2": "16.0", "fx-3": "-1.5", "fx-4": "8.0", "fx-5": "3.0",
		"fx-6": "1.0", "fx-7": "0.0", "fx-8": "2.5", "fx-9": "2.0",
	}
}

// drawnAnswer is the JSON a view draw answers, decoded with every figure
// kept as the literal it travelled as.
type drawnAnswer struct {
	View struct {
		Explained bool `json:"explained"`
		Sections  []struct {
			Cards []struct {
				Ref string `json:"ref"`
			} `json:"cards"`
			Urgency map[string]verb.UrgencyAnswer `json:"urgency"`
		} `json:"sections"`
	} `json:"view"`
}

// drawJSON draws a view under --json and decodes it.
func drawJSON(t *testing.T, root string, argv ...string) drawnAnswer {
	t.Helper()
	var answer drawnAnswer
	out := mustView(t, root, append(argv, "--json")...)
	if err := json.Unmarshal([]byte(out), &answer); err != nil {
		t.Fatalf("decode %v: %v\n%s", argv, err, out)
	}
	return answer
}

// tenths reads a figure written with one decimal digit as integer tenths.
func tenths(t *testing.T, figure string) int64 {
	t.Helper()
	whole, tenth, found := strings.Cut(strings.TrimPrefix(figure, "-"), ".")
	if !found || len(tenth) != 1 {
		t.Fatalf("%q is not written with one decimal digit", figure)
	}
	value, err := strconv.ParseInt(whole+tenth, 10, 64)
	if err != nil {
		t.Fatalf("%q does not read: %v", figure, err)
	}
	if strings.HasPrefix(figure, "-") {
		return -value
	}
	return value
}

// tableCells reads a drawn table by its heading row: every row's cell under
// each heading, keyed by the heading's text and by the row's card
// reference. The table is found under the section heading that opens it.
func tableCells(t *testing.T, drawn, card string) map[string]map[string]string {
	t.Helper()
	lines := strings.Split(drawn, "\n")
	cardHeading := msg.For(msg.Base).T("column.view.card")
	header := -1
	for i, line := range lines {
		if strings.Contains(line, cardHeading) && strings.Contains(line, msg.For(msg.Base).T("column.view.urgency")) {
			header = i
			break
		}
	}
	if header < 0 || header+2 >= len(lines) {
		t.Fatalf("no ranked table was drawn:\n%s", drawn)
	}
	headings := regexp.MustCompile(`\S+`).FindAllStringIndex(lines[header], -1)
	rows := map[string]map[string]string{}
	for _, line := range lines[header+2:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		cells := map[string]string{}
		for c, at := range headings {
			end := len(line)
			if c+1 < len(headings) && headings[c+1][0] < end {
				end = headings[c+1][0]
			}
			if at[0] >= len(line) {
				continue
			}
			cells[lines[header][at[0]:at[1]]] = strings.TrimSpace(line[at[0]:end])
		}
		rows[cells[cardHeading]] = cells
	}
	return rows
}

// explainedTotals reads the total line of every block an explained draw
// prints, keyed by the reference on the line that opens the block.
func explainedTotals(drawn string) map[string]string {
	totals := map[string]string{}
	label := msg.For(msg.Base).T("view.urgency.total")
	current := ""
	opener := regexp.MustCompile(`^  \d+\. (\S+): `)
	for _, line := range strings.Split(drawn, "\n") {
		if m := opener.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		fields := strings.Fields(line)
		if current != "" && len(fields) > 1 && fields[0] == label {
			totals[current] = fields[len(fields)-1]
		}
	}
	return totals
}

// TestThePrintedFigureEqualsTheExplainedTotal is dinah-602/criteria/15: on a
// fixture of nine ranked cards exercising every term, a negative weight and
// a fractional age among them, the table's figure, the explained total, the
// JSON score and the sum of the JSON term points agree on every row.
func TestThePrintedFigureEqualsTheExplainedTotal(t *testing.T) {
	root, want := rankedFixture(t)
	table := tableCells(t, mustView(t, root, "everything"), "")
	totals := explainedTotals(mustView(t, root, "everything", "--explain"))
	answer := drawJSON(t, root, "everything", "--explain")
	urgency := msg.For(msg.Base).T("column.view.urgency")
	if len(answer.View.Sections[0].Cards) != len(want) {
		t.Fatalf("the view ranked %d cards, want %d", len(answer.View.Sections[0].Cards), len(want))
	}
	for ref, score := range want {
		ranked, ok := answer.View.Sections[0].Urgency[ref]
		if !ok {
			t.Errorf("%s is not ranked", ref)
			continue
		}
		sum := int64(0)
		for _, term := range ranked.Terms {
			sum += tenths(t, term.Points.String())
		}
		printed := table[ref][urgency]
		figures := []string{printed, totals[ref], ranked.Score.String(), bench.FormatTenths(sum)}
		for _, figure := range figures {
			if figure != score {
				t.Errorf("%s: table %q, explained total %q, JSON score %q and term sum %q, want every one %s", ref, printed, totals[ref], ranked.Score, bench.FormatTenths(sum), score)
				break
			}
		}
	}
}

// rawFigures is every score and points literal a JSON text carries, in the
// order it carries them, read off the text rather than off a decoded value,
// since a decoder would read 18.5 and 18.50 as one number.
func rawFigures(text string) []string {
	return regexp.MustCompile(`"(?:score|points)":\s*(-?[0-9.eE+]+)`).FindAllString(text, -1)
}

// TestFiguresTravelWithOneDecimalDigitOnBothHeads is dinah-602/criteria/16.
func TestFiguresTravelWithOneDecimalDigitOnBothHeads(t *testing.T) {
	for tenths, want := range map[int64]string{185: "18.5", 160: "16.0", -15: "-1.5", 0: "0.0", 5: "0.5"} {
		if got := bench.FormatTenths(tenths); got != want {
			t.Errorf("%d tenths is written %s, want %s", tenths, got, want)
		}
	}
	root, _ := rankedFixture(t)
	table := tableCells(t, mustView(t, root, "everything"), "")
	urgency := msg.For(msg.Base).T("column.view.urgency")
	for ref, figure := range map[string]string{"fx-1": "18.5", "fx-2": "16.0", "fx-3": "-1.5"} {
		if got := table[ref][urgency]; got != figure {
			t.Errorf("the table draws %s's figure as %q, want %s", ref, got, figure)
		}
	}
	card := mustView(t, root, "everything", "--explain", "fx-3")
	if !regexp.MustCompile(`(?m)^  blocked +blocked \(supplier\) +-1\.5$`).MatchString(card) ||
		!regexp.MustCompile(`(?m)^  urgency +the sum of the terms above +-1\.5$`).MatchString(card) ||
		!regexp.MustCompile(`(?m)^  waits on you .* \+0\.0$`).MatchString(card) {
		t.Errorf("the negative card's explanation does not sign its figures as specified:\n%s", card)
	}
	top := mustView(t, root, "everything", "--explain", "fx-1")
	if !regexp.MustCompile(`(?m)^  urgency +the sum of the terms above +18\.5$`).MatchString(top) || !regexp.MustCompile(`(?m)^  waits on you +Review is yours +\+10\.0$`).MatchString(top) {
		t.Errorf("the top card's explanation reads:\n%s", top)
	}

	terminal := mustView(t, root, "agenda", "--explain", "--json")
	protocol := viewOverTheProtocol(t, root, map[string]any{"view": "agenda", "explain": true})
	got, wantFigures := rawFigures(protocol), rawFigures(terminal)
	if len(wantFigures) < 16 || strings.Join(got, "|") != strings.Join(wantFigures, "|") {
		t.Errorf("the terminal carries %d figures %v and the protocol %d %v", len(wantFigures), wantFigures, len(got), got)
	}
	if !strings.Contains(terminal, `"score": 18.5`) || !strings.Contains(terminal, `"points": -1.5`) {
		t.Errorf("the JSON literals are not written with one decimal digit:\n%s", terminal)
	}
}

// viewOverTheProtocol calls the view tool through the mcp head and answers
// the text of its one content member, which is the payload as it travelled.
func viewOverTheProtocol(t *testing.T, root string, arguments map[string]any) string {
	t.Helper()
	dir := soleBenchDir(t, root)
	opened, err := bench.Open(dir)
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	arguments["actor"] = os.Getenv("DINAH_ACTOR")
	call, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "view", "arguments": arguments},
	})
	if err != nil {
		t.Fatalf("marshal the call: %v", err)
	}
	out := &strings.Builder{}
	if err := mcp.Serve(dir, library, map[string]*verb.Library{}, strings.NewReader(string(call)+"\n"), out, mcp.ProfileAll); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var answer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &answer); err != nil || len(answer.Result.Content) != 1 {
		t.Fatalf("the view tool answered %s (%v)", out.String(), err)
	}
	return answer.Result.Content[0].Text
}

// termLabels are the eight term labels --explain prints, in term order.
func termLabels() []string {
	catalog := msg.For(msg.Base)
	var labels []string
	for _, term := range bench.UrgencyTerms {
		labels = append(labels, catalog.T("view.urgency.term."+term))
	}
	return labels
}

// TestExplainHasTheFormsSpecified is dinah-602/criteria/20 at the terminal.
func TestExplainHasTheFormsSpecified(t *testing.T) {
	root, _ := rankedFixture(t)
	before := mustView(t, root, "everything", "--explain", "fx-2")
	after := mustView(t, root, "everything", "fx-2", "--explain")
	if before != after {
		t.Errorf("--explain before and after the card differ:\n%s\n%s", before, after)
	}
	lines := strings.Split(strings.TrimRight(before, "\n"), "\n")
	labels := termLabels()
	if len(lines) != 1+len(labels)+2 || !strings.HasPrefix(lines[0], "fx-2: ") {
		t.Fatalf("the one-card form prints %d lines, want the heading, eight terms, a rule and a total:\n%s", len(lines), before)
	}
	for i, label := range labels {
		if !strings.HasPrefix(lines[1+i], "  "+label+" ") {
			t.Errorf("term line %d reads %q, want it to open with %q", i+1, lines[1+i], label)
		}
	}
	if strings.Trim(lines[9], " -") != "" || !strings.HasPrefix(strings.TrimSpace(lines[10]), msg.For(msg.Base).T("view.urgency.total")) {
		t.Errorf("the rule and total read %q and %q", lines[9], lines[10])
	}

	every := mustView(t, root, "everything", "--explain")
	answer := drawJSON(t, root, "everything", "--explain")
	var opened []string
	for _, match := range regexp.MustCompile(`(?m)^  \d+\. (\S+): `).FindAllStringSubmatch(every, -1) {
		opened = append(opened, match[1])
	}
	var order []string
	for _, card := range answer.View.Sections[0].Cards {
		order = append(order, card.Ref)
	}
	if strings.Join(opened, " ") != strings.Join(order, " ") || len(order) != 9 {
		t.Errorf("the blocks open in the order %v, and the section ranks %v", opened, order)
	}
	if !answer.View.Explained {
		t.Errorf("an explained draw carries explained false")
	}
	for ref, ranked := range answer.View.Sections[0].Urgency {
		if len(ranked.Terms) != 8 {
			t.Errorf("%s carries %d terms explained", ref, len(ranked.Terms))
		}
	}
	plain := drawJSON(t, root, "everything")
	for ref, ranked := range plain.View.Sections[0].Urgency {
		if plain.View.Explained || ranked.Terms == nil || len(ranked.Terms) != 0 {
			t.Errorf("an unexplained draw carries explained %v and %s's terms %v", plain.View.Explained, ref, ranked.Terms)
		}
	}
	if raw := mustView(t, root, "everything", "--json"); !strings.Contains(raw, `"terms": []`) {
		t.Errorf("an unexplained draw does not write terms as an empty array:\n%s", raw)
	}
	refused := runCLI(t, root, "view", "plain", "--explain")
	if refused.code == 0 || !strings.HasPrefix(refused.errw, "dinah.view-not-ranked ") {
		t.Errorf("--explain on a view ordered by arrival answered %d:\n%s", refused.code, refused.errw)
	}
}

// TestTheWhyCellNamesExactlyTheNonZeroTerms is dinah-602/criteria/21 at the
// terminal: each row's Why cell is the non-zero terms in term order, the
// negatively weighted one included, labelled as the catalog labels them.
func TestTheWhyCellNamesExactlyTheNonZeroTerms(t *testing.T) {
	root, _ := rankedFixture(t)
	table := tableCells(t, mustView(t, root, "everything"), "")
	answer := drawJSON(t, root, "everything", "--explain")
	catalog := msg.For(msg.Base)
	why := catalog.T("column.view.why")
	checked := 0
	for ref, ranked := range answer.View.Sections[0].Urgency {
		var labels []string
		for _, term := range ranked.Terms {
			if tenths(t, term.Points.String()) == 0 {
				continue
			}
			switch term.Term {
			case bench.UrgencyPriority, bench.UrgencySeverity:
				labels = append(labels, term.Basis["level"])
			case bench.UrgencyYourQuestion:
				counted, _ := strconv.Atoi(term.Basis["counted"])
				labels = append(labels, catalog.TN("view.urgency.why.your-question", counted))
			case bench.UrgencyAge:
				days, _ := strconv.Atoi(term.Basis["days"])
				labels = append(labels, catalog.TN("view.urgency.why.age", days))
			default:
				labels = append(labels, catalog.T("view.urgency.why."+term.Term))
			}
		}
		checked++
		if got := table[ref][why]; got != strings.Join(labels, ", ") {
			t.Errorf("%s's Why cell reads %q, want %q", ref, got, strings.Join(labels, ", "))
		}
	}
	if checked != 9 || !strings.Contains(table["fx-3"][why], catalog.T("view.urgency.why.blocked")) {
		t.Errorf("checked %d rows, and the negatively weighted blocked term must be named on fx-3: %q", checked, table["fx-3"][why])
	}
}

// TestTheCardPositionalDrawsOneRow is dinah-602/criteria/19 at the terminal.
func TestTheCardPositionalDrawsOneRow(t *testing.T) {
	root, _ := rankedFixture(t)
	narrowed := mustView(t, root, "everything", "fx-2")
	table := tableCells(t, narrowed, "")
	if len(table) != 1 || table["fx-2"][msg.For(msg.Base).T("column.view.rank")] != "2" {
		t.Errorf("the narrowed view draws %v, want fx-2 alone at rank 2:\n%s", table, narrowed)
	}
	missing := runCLI(t, root, "view", "agenda", "fx-7")
	if missing.code == 0 || !strings.HasPrefix(missing.errw, "dinah.card-not-in-view ") {
		t.Errorf("a card the agenda does not select answered %d:\n%s", missing.code, missing.errw)
	}
	unknown := runCLI(t, root, "view", "agenda", "fx-99")
	if unknown.code == 0 || !strings.HasPrefix(unknown.errw, "unknown-card ") {
		t.Errorf("a reference to no card answered %d:\n%s", unknown.code, unknown.errw)
	}
}

// urgencyExample is the dinah.urgency example a document prints, as the
// Markdown code block carrying it defines its content: everything between a
// yaml fence and the fence closing it, or every line of an indented block with
// its four leading spaces removed, as the guides write their examples. Nothing
// else about the lines is touched.
func urgencyExample(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)
	if start := strings.Index(text, "```yaml\n"+bench.UrgencyKey+":\n"); start >= 0 {
		block := text[start+len("```yaml\n"):]
		end := strings.Index(block, "```")
		if end < 0 {
			t.Fatalf("%s leaves its dinah.urgency example unclosed", path)
		}
		return block[:end]
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "    "+bench.UrgencyKey+":" {
			continue
		}
		var block strings.Builder
		for _, inside := range lines[i:] {
			content, indented := strings.CutPrefix(inside, "    ")
			if !indented {
				break
			}
			block.WriteString(content + "\n")
		}
		return block.String()
	}
	t.Fatalf("%s prints no dinah.urgency example", path)
	return ""
}

// TestThePrintedWeightsExampleParsesAsWritten is dinah-602/criteria/24.
func TestThePrintedWeightsExampleParsesAsWritten(t *testing.T) {
	documents := []string{
		filepath.Join("..", "..", "internal", "guide", "guides", "views.md"),
		filepath.Join("..", "..", "docs", "design", "format.md"),
	}
	for _, document := range documents {
		t.Run(filepath.Base(document), func(t *testing.T) {
			block := urgencyExample(t, document)
			root := newBenchFromDefinition(t, rankedDefinition)
			declareUrgencyIn(t, root, block)
			mustRunHere(t, root, "add", "At the operator's station")
			mustRunHere(t, root, "move", "fx-1", "review")
			mustRunHere(t, root, "set", "fx-1", "priority", "now")
			backdate(t, root, "fx-1", 36*time.Hour)
			answer := drawJSON(t, root, "agenda", "--explain")
			ranked := answer.View.Sections[0].Urgency["fx-1"]
			if ranked.Score.String() != "16.5" {
				t.Errorf("the example's weights score the card %s, want 16.5 (10 waiting, 6 at now, 0.5 for one day):\n%+v", ranked.Score, ranked)
			}
			checked := mustRunHere(t, root, "check", "--json").out
			if strings.Contains(checked, "check.urgency-") {
				t.Errorf("check raised an urgency finding on the printed example:\n%s", checked)
			}
		})
	}
}

// TestEachSpellingRuleHoldsAtTheTerminal is dinah-602/criteria/25 and
// criteria/26, driven through the head the binary runs.
func TestEachSpellingRuleHoldsAtTheTerminal(t *testing.T) {
	accepted := []string{
		bench.UrgencyKey + ":\n  priority: [0, 2, 4, 6]\n  blocked: 3\n  age:\n    per-day: 0.5\n    cap: 5\n",
		bench.UrgencyKey + ":\n",
		bench.UrgencyKey + ":\n  # every weight switched off\n",
	}
	for _, block := range accepted {
		root := newBenchFromDefinition(t, rankedDefinition)
		declareUrgencyIn(t, root, block)
		mustRunHere(t, root, "add", "At the operator's station")
		mustRunHere(t, root, "move", "fx-1", "review")
		if got := drawJSON(t, root, "agenda").View.Sections[0].Urgency["fx-1"].Score.String(); got != "10.0" {
			t.Errorf("%q scores the card %s, want the default 10.0", block, got)
		}
		if checked := mustRunHere(t, root, "check", "--json").out; strings.Contains(checked, "check.urgency-") {
			t.Errorf("%q raised an urgency finding:\n%s", block, checked)
		}
	}
	commented := newBenchFromDefinition(t, rankedDefinition)
	declareUrgencyIn(t, commented, bench.UrgencyKey+":\n  # the weights\n  priority: [0, 2, 4, 6]\n  # the age\n  age:\n    # per day\n    per-day: 0.5\n")
	mustRunHere(t, commented, "add", "At the operator's station")
	mustRunHere(t, commented, "move", "fx-1", "review")
	if got := drawJSON(t, commented, "agenda").View.Sections[0].Urgency["fx-1"].Score.String(); got != "10.0" {
		t.Errorf("comment lines inside the block changed the score to %s", got)
	}
	refused := []struct {
		block string
		term  string
		read  string
	}{
		{block: "  priority:\n    - 0\n    - 2\n    - 4\n    - 6\n", term: "priority", read: `[\"0\",\"2\",\"4\",\"6\"]`},
		{block: "  blocked: \"3\"\n", term: "blocked", read: `\"3\"`},
		{block: "  priority: [0, 2, 4, 6]  # note\n", term: "priority", read: `\"[0, 2, 4, 6]  # note\"`},
		{block: "  age: {per-day: 0.5, cap: 5}\n", term: "age", read: `\"{per-day: 0.5, cap: 5}\"`},
	}
	for _, c := range refused {
		root := newBenchFromDefinition(t, rankedDefinition)
		declareUrgencyIn(t, root, bench.UrgencyKey+":\n"+c.block)
		got := runCLI(t, root, "view", "agenda", "--json")
		wants := []string{`"refusal": "dinah.malformed-urgency"`, `"defect": "malformed-term"`, `"term": "` + c.term + `"`, `"read": "` + c.read + `"`}
		for _, want := range wants {
			if got.code == 0 || !strings.Contains(got.out, want) {
				t.Errorf("%q answered %d without %s:\n%s", c.block, got.code, want, got.out)
			}
		}
		human := runCLI(t, root, "view", "agenda")
		if !strings.HasPrefix(human.errw, "dinah.malformed-urgency ") || !strings.Contains(human.errw, c.term+" ") {
			t.Errorf("%q is refused at the terminal as:\n%s", c.block, human.errw)
		}
	}
	flat := newBenchFromDefinition(t, rankedDefinition)
	declareUrgencyIn(t, flat, bench.UrgencyKey+": flat\n")
	if got := runCLI(t, flat, "view", "agenda", "--json"); got.code == 0 || !strings.Contains(got.out, `"defect": "not-a-mapping"`) {
		t.Errorf("dinah.urgency: flat answered %d:\n%s", got.code, got.out)
	}
}
