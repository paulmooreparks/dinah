package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/testenv"
	"dinah/internal/verb"
)

// runBench is a workbench whose doing column runs a recipe launching this
// test binary as a stand-in harness, which is how `dinah run` meets a user:
// a configured command that reads a prompt and prints a receipt.
type runBench struct {
	root   string
	script string
	log    string
}

// newRunBench builds the workbench: the run fields declared, a recipe named
// fake whose fresh and resumed commands are told apart by their first word,
// and the doing column naming it with intake as its reject_to.
func newRunBench(t *testing.T) *runBench {
	t.Helper()
	root := newBench(t)
	dir := t.TempDir()
	rb := &runBench{root: root, script: filepath.Join(dir, "script.json"), log: filepath.Join(dir, "launches.ndjson")}
	t.Setenv(testenv.HarnessScriptVar, rb.script)
	exe := filepath.ToSlash(os.Args[0])
	block := "fields:\n" +
		"  run.session:\n    type: string\n    meaning: the harness session that last worked the card\n    on: [card]\n" +
		"  run.recipe:\n    type: string\n    meaning: the recipe that session belongs to\n    on: [card]\n" +
		"  run.cumulative.usd:\n    type: number\n    meaning: the last cumulative cost the session reported\n    on: [card]\n" +
		"run:\n" +
		"  fake:\n" +
		"    command: ['" + exe + "', fresh, --system, \"{instructions}\", --card, \"{card}\"]\n" +
		"    resume:\n" +
		"      - '" + exe + "'\n" +
		"      - resumed\n" +
		"      - --resume\n" +
		"      - \"{session}\"\n" +
		"    receipt: {session: session_id, text: result, error: is_error, spend: {unit: usd, field: total_cost_usd, cumulative: true}}\n" +
		"  other:\n" +
		"    command: ['" + exe + "', fresh-other]\n" +
		"    resume: ['" + exe + "', resumed-other, \"{session}\"]\n" +
		"    receipt: {session: session_id, text: result, spend: {unit: usd, field: total_cost_usd, cumulative: true}}\n"
	editAnchor(t, root, "columns:", block+"columns:")
	column := strings.TrimSpace(runCLI(t, root, "path", "doing").out)
	editAnchorAt(t, column, "kind: work", "kind: work\nrun: fake\nreject_to: intake")
	text, err := os.ReadFile(column)
	if err != nil {
		t.Fatalf("read %s: %v", column, err)
	}
	if err := os.WriteFile(column, append(text, []byte(runColumnText+"\n")...), 0o644); err != nil {
		t.Fatalf("write %s: %v", column, err)
	}
	return rb
}

// runColumnText is the doing column's instructions, which a recipe taking
// them as a file finds there and not in the prompt.
const runColumnText = "Write the file the card names, and nothing else."

// say writes what the next launch of the stand-in does.
func (rb *runBench) say(t *testing.T, script testenv.HarnessScript) {
	t.Helper()
	script.Log = rb.log
	data, err := json.Marshal(script)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(rb.script, data, 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
}

// launches reads every launch the stand-in recorded.
func (rb *runBench) launches(t *testing.T) []testenv.HarnessLaunch {
	t.Helper()
	data, err := os.ReadFile(rb.log)
	if err != nil {
		return nil
	}
	var all []testenv.HarnessLaunch
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var launch testenv.HarnessLaunch
		if err := json.Unmarshal([]byte(line), &launch); err != nil {
			t.Fatalf("launch line %q: %v", line, err)
		}
		all = append(all, launch)
	}
	return all
}

// card reads a card as the store holds it, live or archived, since a card
// carried into a done column is archived by the move.
func (rb *runBench) card(t *testing.T, ref string) (*bench.Card, *bench.Bench) {
	t.Helper()
	opened, err := bench.Open(benchDir(t, rb.root))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	found, err := opened.ResolveCard(ref)
	if err != nil {
		found, err = opened.ResolveArchivedCard(ref)
	}
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	return found.Card, opened
}

// field reads one declared field off a card's own anchor.
func (rb *runBench) field(t *testing.T, ref, key string) string {
	t.Helper()
	card, _ := rb.card(t, ref)
	text, err := os.ReadFile(card.AnchorPath())
	if err != nil {
		t.Fatalf("read %s: %v", card.AnchorPath(), err)
	}
	fm, _ := bench.ParseAnchor(string(text))
	return bench.FieldValue(fm, key)
}

// comments reads the full text of every comment on a card.
func (rb *runBench) comments(t *testing.T, ref string) []string {
	t.Helper()
	argv := []string{"--json", "show", ref, "--fields", "comments.full"}
	if opened, err := bench.Open(benchDir(t, rb.root)); err == nil {
		if _, err := opened.ResolveCard(ref); err != nil {
			argv = append(argv, "--archived")
		}
	}
	got := runCLI(t, rb.root, argv...)
	if got.code != 0 {
		t.Fatalf("show %s: %d %s", ref, got.code, got.errw)
	}
	var detail struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(got.out), &detail); err != nil {
		t.Fatalf("decode: %v\n%s", err, got.out)
	}
	var bodies []string
	for _, comment := range detail.Comments {
		bodies = append(bodies, comment.Body)
	}
	return bodies
}

// spends reads a card's spend lines off its journal, which reaches an
// archived card the spend report does not.
func (rb *runBench) spends(t *testing.T, ref string) []verb.SpendRecord {
	t.Helper()
	card, opened := rb.card(t, ref)
	events, _, err := opened.ReadJournal(card.JournalPath())
	if err != nil {
		t.Fatalf("journal %s: %v", ref, err)
	}
	var records []verb.SpendRecord
	for _, ev := range events {
		if ev.Event != contract.EventSpend {
			continue
		}
		record := verb.SpendRecord{Unit: ev.Unit, Unreported: ev.Unreported, Round: ev.Round, Column: ev.Column, Note: ev.Note}
		record.Total = ev.Total
		records = append(records, record)
	}
	return records
}

// columnOf answers the slug of the column a card stands in, and its state.
func (rb *runBench) columnOf(t *testing.T, ref string) (string, string, *bench.Card) {
	t.Helper()
	card, opened := rb.card(t, ref)
	return opened.Column(card.Column).Slug, card.State, card
}

// result composes an agent's final text ending in a result block.
func result(handoff, outcome, reason string) string {
	return handoff + "\n\n```json\n{\"outcome\": \"" + outcome + "\", \"reason\": \"" + reason + "\"}\n```\n"
}

// TestRunFreshThenResumed is the case `dinah run` exists for: a first run
// starts a fresh session, posts the handoff, keeps the session and releases
// the card on stay; a second run resumes that session by the recipe's own
// resume argv, records only what the second pass cost, and moves the card on
// forward.
func TestRunFreshThenResumed(t *testing.T) {
	rb := newRunBench(t)
	card := addCard(t, rb.root, "Write hello.txt containing hello")
	carryToDoing(t, rb.root, card)

	rb.say(t, testenv.HarnessScript{Session: "sess-A", Cost: 0.25, Text: result("I wrote a first draft.", "stay", "needs a second pass")})
	first := runCLI(t, rb.root, "run", card)
	if first.code != 0 {
		t.Fatalf("first run: %d %s %s", first.code, first.out, first.errw)
	}
	column, state, held := rb.columnOf(t, card)
	if column != "doing" || state != contract.StateReady || held.Holder != "" {
		t.Errorf("after stay the card stands at %s/%s held by %q, wanted doing/ready and nobody", column, state, held.Holder)
	}
	if got := rb.field(t, card, "run.session"); got != "sess-A" {
		t.Errorf("run.session is %q, wanted sess-A", got)
	}
	if got := rb.field(t, card, "run.recipe"); got != "fake" {
		t.Errorf("run.recipe is %q, wanted fake", got)
	}
	if got := rb.field(t, card, "run.cumulative.usd"); got != "0.25" {
		t.Errorf("run.cumulative.usd is %q, wanted 0.25", got)
	}
	launches := rb.launches(t)
	if len(launches) != 1 {
		t.Fatalf("the harness was launched %d times, wanted once", len(launches))
	}
	fresh := launches[0]
	if len(fresh.Args) != 5 || fresh.Args[0] != "fresh" || fresh.Args[1] != "--system" || fresh.Args[3] != "--card" || fresh.Args[4] != card {
		t.Errorf("the fresh argv is %q, wanted the recipe's command with its placeholders filled", fresh.Args)
	}
	if body, written := fresh.Files[fresh.Args[2]]; !written {
		t.Errorf("the {instructions} placeholder %q named no file the harness could read", fresh.Args[2])
	} else if !strings.Contains(body, runColumnText) || strings.Contains(fresh.Stdin, runColumnText) {
		t.Errorf("the column's instructions should travel in the file alone: file %q, prompt %q", body, fresh.Stdin)
	}
	if !strings.Contains(fresh.Stdin, card) || !strings.Contains(fresh.Stdin, "```json") {
		t.Errorf("the prompt does not carry the brief and the result instructions: %q", fresh.Stdin)
	}
	if canonical(t, fresh.Workbench) != canonical(t, benchDir(t, rb.root)) {
		t.Errorf("the harness was handed DINAH_WORKBENCH %q, wanted %q", fresh.Workbench, benchDir(t, rb.root))
	}
	if canonical(t, fresh.Cwd) != canonical(t, rb.root) {
		t.Errorf("the harness ran in %q, wanted the project directory %q", fresh.Cwd, rb.root)
	}

	rb.say(t, testenv.HarnessScript{Session: "sess-A", Cost: 0.75, Text: result("hello.txt now holds hello.", "forward", "written and checked")})
	second := runCLI(t, rb.root, "run", card)
	if second.code != 0 {
		t.Fatalf("second run: %d %s %s %q", second.code, second.out, second.errw, rb.comments(t, card))
	}
	launches = rb.launches(t)
	if len(launches) != 2 {
		t.Fatalf("the harness was launched %d times, wanted twice", len(launches))
	}
	if got := strings.Join(launches[1].Args, " "); got != "resumed --resume sess-A" {
		t.Errorf("the second argv is %q, wanted the recipe's resume argv naming sess-A", got)
	}
	column, state, held = rb.columnOf(t, card)
	if column != "done" || held.Holder != "" {
		t.Errorf("after forward the card stands at %s/%s held by %q, wanted done", column, state, held.Holder)
	}
	comments := rb.comments(t, card)
	if len(comments) != 2 || comments[0] != "I wrote a first draft." || comments[1] != "hello.txt now holds hello." {
		t.Errorf("the handoffs posted are %q, wanted the two texts before the result blocks", comments)
	}
	spends := rb.spends(t, card)
	if len(spends) != 2 {
		t.Fatalf("the card carries %d spend lines, wanted 2", len(spends))
	}
	for at, want := range []float64{0.25, 0.5} {
		line := spends[at]
		if line.Unit != "usd" || line.Total == nil || *line.Total != want || line.Unreported {
			t.Errorf("spend line %d is %+v, wanted total %v usd", at+1, line, want)
		}
		if line.Round != at+1 {
			t.Errorf("spend line %d carries round %d, wanted %d", at+1, line.Round, at+1)
		}
	}
	if got := rb.field(t, card, "run.cumulative.usd"); got != "0.75" {
		t.Errorf("run.cumulative.usd is %q after the resumed run, wanted 0.75", got)
	}
}

// canonical resolves a directory the way the operating system names it, so a
// short name and a long name for one directory compare equal.
func canonical(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve %s: %v", dir, err)
	}
	return strings.ToLower(filepath.Clean(resolved))
}

// TestRunCarriesOutEachResult drives back, block and a text ending without a
// result block, each on a card of its own.
func TestRunCarriesOutEachResult(t *testing.T) {
	rb := newRunBench(t)

	back := addCard(t, rb.root, "Sent back")
	carryToDoing(t, rb.root, back)
	rb.say(t, testenv.HarnessScript{Session: "sess-B", Cost: 0.1, Text: result("The framing is wrong.", "back", "intake should reframe it")})
	if got := runCLI(t, rb.root, "run", back); got.code != 0 {
		t.Fatalf("run %s: %d %s", back, got.code, got.errw)
	}
	if column, state, _ := rb.columnOf(t, back); column != "intake" || state != contract.StateReady {
		t.Errorf("after back the card stands at %s/%s, wanted intake/ready", column, state)
	}

	blocked := addCard(t, rb.root, "Blocked")
	carryToDoing(t, rb.root, blocked)
	rb.say(t, testenv.HarnessScript{Session: "sess-C", Cost: 0.1, Text: result("I cannot reach the share.", "block", "the share is offline")})
	if got := runCLI(t, rb.root, "run", blocked); got.code != 0 {
		t.Fatalf("run %s: %d %s", blocked, got.code, got.errw)
	}
	if _, state, card := rb.columnOf(t, blocked); state != contract.StateBlocked || card.BlockReason != "the share is offline" {
		t.Errorf("after block the card is %s with reason %q, wanted blocked with the agent's reason", state, card.BlockReason)
	}
	again := runCLI(t, rb.root, "run", blocked)
	if again.code != contract.ExitCode(contract.OutcomeRefused) || !strings.HasPrefix(again.errw, contract.Blocked) {
		t.Errorf("a run on a blocked card was not refused blocked: %d %s", again.code, again.errw)
	}

	silent := addCard(t, rb.root, "No result block")
	carryToDoing(t, rb.root, silent)
	rb.say(t, testenv.HarnessScript{Session: "sess-D", Cost: 0.1, Text: "I did some of it and stopped."})
	got := runCLI(t, rb.root, "run", silent)
	if got.code != 0 {
		t.Fatalf("run %s: %d %s", silent, got.code, got.errw)
	}
	if column, state, card := rb.columnOf(t, silent); column != "doing" || state != contract.StateReady || card.Holder != "" {
		t.Errorf("a text with no result block left the card at %s/%s held by %q, wanted it released in doing", column, state, card.Holder)
	}
	if comments := rb.comments(t, silent); len(comments) != 1 || comments[0] != "I did some of it and stopped." {
		t.Errorf("the handoff of a text with no block is %q, wanted the whole text", comments)
	}
	machine := runCLI(t, rb.root, "--json", "run", silent)
	var report runReport
	if err := json.Unmarshal([]byte(machine.out), &report); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if report.Outcome != verb.RunStay || report.Reported || report.Resumed != true {
		t.Errorf("the machine report of a resumed run with no block is %+v", report)
	}
}

// TestRunReleasesOnAFailedHarness drives the three ways a harness fails to
// deliver: a non-zero exit, a timeout and a receipt that will not read. Each
// releases the card, posts standard error, and records the spend line, as
// unreported where no receipt was read.
func TestRunReleasesOnAFailedHarness(t *testing.T) {
	rb := newRunBench(t)
	for _, row := range []struct {
		name       string
		script     testenv.HarnessScript
		flags      []string
		comment    string
		unreported bool
	}{
		{
			name:    "exit",
			script:  testenv.HarnessScript{Session: "sess-E", Cost: 0.3, Exit: 3, Stderr: "the model refused the request", Text: result("partial", "forward", "x")},
			comment: "the model refused the request",
		},
		{
			name:       "timeout",
			script:     testenv.HarnessScript{Session: "sess-F", Cost: 0.3, SleepMS: 20000, Text: result("late", "forward", "x")},
			flags:      []string{"--timeout", "1s"},
			comment:    "",
			unreported: true,
		},
		{
			name:       "receipt",
			script:     testenv.HarnessScript{Raw: "this is not a receipt", Stderr: "printed something odd"},
			comment:    "printed something odd",
			unreported: true,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			card := addCard(t, rb.root, "Failing "+row.name)
			carryToDoing(t, rb.root, card)
			rb.say(t, row.script)
			argv := append([]string{"--json", "run", card}, row.flags...)
			got := runCLI(t, rb.root, argv...)
			if got.code != 1 {
				t.Fatalf("a failed run exited %d, wanted 1: %s %s", got.code, got.out, got.errw)
			}
			var report runReport
			if err := json.Unmarshal([]byte(got.out), &report); err != nil {
				t.Fatalf("decode: %v\n%s", err, got.out)
			}
			if report.Failure != row.name {
				t.Errorf("the run reports failure %q, wanted %q", report.Failure, row.name)
			}
			if column, state, held := rb.columnOf(t, card); column != "doing" || state != contract.StateReady || held.Holder != "" {
				t.Errorf("a failed run left the card at %s/%s held by %q, wanted it released in doing", column, state, held.Holder)
			}
			comments := rb.comments(t, card)
			if len(comments) != 1 || !strings.Contains(comments[0], row.comment) {
				t.Errorf("the failure comment is %q, wanted it to carry %q", comments, row.comment)
			}
			spends := rb.spends(t, card)
			if len(spends) != 1 || spends[0].Unreported != row.unreported {
				t.Errorf("the spend lines are %+v, wanted one with unreported %v", spends, row.unreported)
			}
			if !row.unreported && (spends[0].Total == nil || *spends[0].Total != row.script.Cost) {
				t.Errorf("a readable receipt's cost was not recorded: %+v", spends[0])
			}
		})
	}
}

// TestRunRefusesBeforeItClaims covers the rows the run checks before it
// launches anything: a column naming no recipe, and a workbench that does not
// declare the fields the run writes.
func TestRunRefusesBeforeItClaims(t *testing.T) {
	rb := newRunBench(t)
	card := addCard(t, rb.root, "Standing at intake")
	got := runCLI(t, rb.root, "run", card)
	if got.code != contract.ExitCode(contract.OutcomeRefused) || !strings.HasPrefix(got.errw, contract.NoRunRecipe) {
		t.Errorf("a run at a column naming no recipe was not refused %s: %d %s", contract.NoRunRecipe, got.code, got.errw)
	}
	editAnchor(t, rb.root, "  run.session:\n", "  run.sessions:\n")
	carryToDoing(t, rb.root, card)
	got = runCLI(t, rb.root, "run", card)
	if got.code != contract.ExitCode(contract.OutcomeRefused) || !strings.HasPrefix(got.errw, contract.UndeclaredField) {
		t.Errorf("a run on a workbench not declaring run.session was not refused: %d %s", got.code, got.errw)
	}
	if _, state, _ := rb.columnOf(t, card); state != contract.StateReady {
		t.Errorf("a refused run left the card %s", state)
	}
	if launches := rb.launches(t); len(launches) != 0 {
		t.Errorf("a refused run launched the harness %d times", len(launches))
	}
}

// TestRunRecipesParse pins the recipe reader on the spellings the format
// documents and on the defects it refuses.
func TestRunRecipesParse(t *testing.T) {
	fm, _ := bench.ParseAnchor("---\n" +
		"run:\n" +
		"  claude:\n" +
		"    command: [claude, -p, --output-format, json, --allowedTools, \"Read,Write\", --append-system-prompt-file, \"{instructions}\"]\n" +
		"    resume: [claude, -p, --output-format, json, --resume, \"{session}\"]\n" +
		"    cwd: \"{workbench}/../..\"\n" +
		"    receipt: {session: session_id, text: result, spend: {unit: usd, field: total_cost_usd, cumulative: true}}\n" +
		"  block:\n" +
		"    command:\n" +
		"      - 'C:/tools/run suite.exe'\n" +
		"      - --flag=a:b\n" +
		"    receipt:\n" +
		"      text: summary.text\n" +
		"      spend:\n" +
		"        unit: minutes\n" +
		"        field: elapsed\n" +
		"  broken:\n" +
		"    command: [unclosed\n" +
		"    receipt: {text: result}\n" +
		"  textless:\n" +
		"    command: [x]\n" +
		"    receipt: {session: id}\n" +
		"---\n")
	recipes, defects := bench.ParseRunRecipes(fm)
	claude := recipes["claude"]
	if claude == nil {
		t.Fatalf("the flow-spelled recipe did not parse: %+v", defects)
	}
	if got := strings.Join(claude.Command, "|"); got != "claude|-p|--output-format|json|--allowedTools|Read,Write|--append-system-prompt-file|{instructions}" {
		t.Errorf("the command reads %q", got)
	}
	if claude.Receipt.Session != "session_id" || claude.Receipt.Text != "result" || claude.Receipt.Spend == nil || !claude.Receipt.Spend.Cumulative || claude.Receipt.Spend.Unit != "usd" {
		t.Errorf("the receipt reads %+v", claude.Receipt)
	}
	if claude.Cwd != "{workbench}/../.." {
		t.Errorf("the cwd reads %q", claude.Cwd)
	}
	block := recipes["block"]
	if block == nil {
		t.Fatalf("the block-spelled recipe did not parse: %+v", defects)
	}
	if got := strings.Join(block.Command, "|"); got != "C:/tools/run suite.exe|--flag=a:b" {
		t.Errorf("the dashed command reads %q", got)
	}
	if block.Receipt.Text != "summary.text" || block.Receipt.Spend == nil || block.Receipt.Spend.Cumulative || block.Receipt.Spend.Unit != "minutes" {
		t.Errorf("the nested receipt reads %+v", block.Receipt)
	}
	want := map[string]string{"broken": "command", "textless": "receipt.text"}
	if len(defects) != len(want) {
		t.Errorf("the defects are %+v, wanted %v", defects, want)
	}
	for _, defect := range defects {
		if want[defect.Name] != defect.Member {
			t.Errorf("recipe %s is reported at %q, wanted %q", defect.Name, defect.Member, want[defect.Name])
		}
	}
}

// TestRunOutcomeAndReceiptReaders pins the two readers on the positions that
// break a careless one: a block that is not last, a fence that is not json, an
// outcome outside the four, and a receipt behind a line of events.
func TestRunOutcomeAndReceiptReaders(t *testing.T) {
	for _, row := range []struct {
		text, outcome, handoff string
		reported               bool
	}{
		{result("done", "forward", "ok"), verb.RunForward, "done", true},
		{result("done", "sideways", "ok"), verb.RunStay, strings.TrimSpace(result("done", "sideways", "ok")), false},
		{result("done", "back", "ok") + "\nand one more line", verb.RunStay, strings.TrimSpace(result("done", "back", "ok") + "\nand one more line"), false},
		{"done\n\n```\n{\"outcome\": \"forward\"}\n```", verb.RunStay, "done\n\n```\n{\"outcome\": \"forward\"}\n```", false},
		{"```json\n{\"outcome\": \"block\", \"reason\": \"why\"}\n```", verb.RunBlock, "", true},
	} {
		got := verb.ParseRunOutcome(row.text)
		if got.Outcome != row.outcome || got.Handoff != row.handoff || got.Reported != row.reported {
			t.Errorf("ParseRunOutcome(%q) = %+v, wanted %s %q reported %v", row.text, got, row.outcome, row.handoff, row.reported)
		}
	}
	receipt := bench.RunReceipt{Session: "session_id", Text: "result", Error: "is_error", Spend: &bench.RunSpend{Unit: "usd", Field: "usage.cost"}}
	read, ok := verb.ReadRunReceipt([]byte("{\"type\":\"event\"}\n{\"session_id\":\"s\",\"result\":\"t\",\"is_error\":true,\"usage\":{\"cost\":1.5}}\n"), receipt)
	if !ok || read.Session != "s" || read.Text != "t" || !read.Error || read.Figure == nil || *read.Figure != 1.5 {
		t.Errorf("the receipt behind an event line read as %+v, %v", read, ok)
	}
	if _, ok := verb.ReadRunReceipt([]byte("{\"session_id\":\"s\"}"), receipt); ok {
		t.Errorf("a receipt carrying no text member read as readable")
	}
}
