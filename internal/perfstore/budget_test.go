package perfstore_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/httphead"
	"dinah/internal/perfstore"
	"dinah/internal/verb"
)

// budget is one row of the budget table: an operation, the most its median
// may take, the CI median that limit was computed from, and the card that
// last set it. docs/design/performance-budgets.md states the rule that turns
// a basis into a limit and when a row changes.
type budget struct {
	op    string        // "status-warm", "show", "page-card", "status-cold"
	limit time.Duration // the budget
	basis time.Duration // the CI median the limit was computed from
	setBy string        // the card that last set it, "dinah-621"
}

// budgets holds the pinned rows per GOOS. Only windows carries any, because
// the perf job runs on windows-latest alone and a budget is a number about
// one runner image.
//
// dinah-621 calibrated every row from three perf-job runs on its own pull
// request, taking each operation's median across the three runs' medians as
// the basis: status-warm 362, 376 and 367ms; show 30, 33 and 32ms; page-card
// 2,137, 2,287 and 2,170ms; status-cold 488, 485 and 457ms.
var budgets = map[string][]budget{
	"windows": {
		{op: "status-warm", limit: 1110 * time.Millisecond, basis: 367 * time.Millisecond, setBy: "dinah-621"},
		{op: "show", limit: 100 * time.Millisecond, basis: 32 * time.Millisecond, setBy: "dinah-621"},
		{op: "page-card", limit: 6510 * time.Millisecond, basis: 2170 * time.Millisecond, setBy: "dinah-621"},
		{op: "status-cold", limit: 1460 * time.Millisecond, basis: 485 * time.Millisecond, setBy: "dinah-621"},
	},
}

// The judging constants. A budget is three times its basis rounded up to the
// next 10ms and never below 30ms; a budget more than six times the median,
// and above 30ms, is slack the tightening rule removes.
const (
	budgetMultiple = 3
	slackMultiple  = 6
	budgetRounding = 10 * time.Millisecond
	budgetFloor    = 30 * time.Millisecond
	measuredRuns   = 10
)

// reproduce is the command the design note and every failure name.
const reproduce = "DINAH_PERF=measure go test -count=1 -run '^TestReadBudgets$' -v ./internal/perfstore"

// operation is one measured read. run performs one execution, measuring
// exactly its span, and checks the answer it produced outside that span.
type operation struct {
	name string
	run  func() (time.Duration, error)
}

// sample is ten measured runs of one operation, after one discarded warm-up.
type sample struct {
	runs   []time.Duration // ascending
	median time.Duration
}

// TestReadBudgets generates the development shape, confirms dinah check finds
// nothing in it, and holds four reads to the budgets pinned for this GOOS:
// a warm status, a show of perf-1, the HTML card page served through the
// head's handler, and a status in a fresh process. DINAH_PERF selects the
// mode: unset skips, measure fails over budget, and ci also fails on slack.
func TestReadBudgets(t *testing.T) {
	mode := os.Getenv("DINAH_PERF")
	switch mode {
	case "":
		t.Skip("set DINAH_PERF=measure to run the read budgets")
	case "measure", "ci":
	default:
		t.Fatalf("DINAH_PERF is %q; the accepted values are measure and ci", mode)
	}
	shape := perfstore.DevelopmentShape()
	store := generateForBudgets(t, shape)
	b, err := bench.Open(store.Root)
	if err != nil {
		t.Fatalf("open the generated store: %v", err)
	}
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check the generated store: %v", err)
	}
	for _, finding := range findings {
		t.Errorf("check finding %s %s at %s", finding.Key, finding.Detail, finding.Path)
	}
	if len(findings) > 0 {
		t.FailNow()
	}
	t.Logf("perfstore: check clean")
	binary := buildBinary(t)
	home := t.TempDir()
	operations := budgetOperations(t, store, b, binary, home)
	pinned, calibrated := budgets[runtime.GOOS]
	var measured []string
	for _, op := range operations {
		if !calibrated {
			first := measureOp(t, op)
			line := fmt.Sprintf("%s median %s", op.name, millis(first.median))
			measured = append(measured, line)
			t.Logf("%-12s median %s  min %s  max %s", op.name, millis(first.median), millis(first.runs[0]), millis(first.runs[len(first.runs)-1]))
			continue
		}
		judge(t, mode, store, op, rowFor(t, pinned, op.name))
	}
	if !calibrated {
		t.Skipf("no budgets pinned for %s; measured: %s", runtime.GOOS, strings.Join(measured, ", "))
	}
}

// TestBudgetsFollowTheRule asserts that every pinned row's limit is the one
// the rule gives its basis, three times it rounded up to the next 10ms and
// never below 30ms, and that the windows table carries a row for each of the
// four operations. It runs in the ordinary suite, so a row edited by hand
// away from its basis fails there rather than waiting for the perf job.
func TestBudgetsFollowTheRule(t *testing.T) {
	windows := budgets["windows"]
	if len(windows) != 4 {
		t.Errorf("the windows table carries %d rows, wanted one for each of the four operations", len(windows))
	}
	for goos, rows := range budgets {
		for _, row := range rows {
			if want := tightened(row.basis); row.limit != want {
				t.Errorf("%s %s: limit %s, the rule gives %s from a basis of %s", goos, row.op, millis(row.limit), millis(want), millis(row.basis))
			}
			if row.setBy == "" {
				t.Errorf("%s %s names no card that set it", goos, row.op)
			}
		}
	}
}

// generateForBudgets writes the development shape into a temporary directory,
// or into the directory DINAH_PERF_KEEP names, which is left in place for
// profiling, and logs the seed, the file count, the digest and the time taken.
func generateForBudgets(t *testing.T, shape perfstore.Shape) *perfstore.Store {
	t.Helper()
	dir := t.TempDir()
	keep := os.Getenv("DINAH_PERF_KEEP")
	if keep != "" {
		if err := os.MkdirAll(keep, 0o755); err != nil {
			t.Fatalf("DINAH_PERF_KEEP %s: %v", keep, err)
		}
		dir = keep
	}
	start := time.Now()
	store, err := perfstore.Generate(dir, perfstore.DefaultSeed, shape)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	elapsed := time.Since(start)
	t.Logf("perfstore: seed %d, %s files, digest %s, generated in %.1fs",
		store.Seed, thousands(int64(store.Files)), store.Digest, elapsed.Seconds())
	if keep != "" {
		t.Logf("perfstore: kept at %s", store.Root)
	}
	return store
}

// buildBinary builds cmd/dinah into a temporary directory, as the serve tests
// do, and logs how long the build took.
func buildBinary(t *testing.T) string {
	t.Helper()
	name := "dinah"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	start := time.Now()
	build := exec.Command("go", "build", "-o", path, "dinah/cmd/dinah")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build dinah: %v\n%s", err, out)
	}
	t.Logf("perfstore: built the binary in %.1fs", time.Since(start).Seconds())
	return path
}

// budgetOperations composes the four measured reads over one store, in the
// order they run.
func budgetOperations(t *testing.T, store *perfstore.Store, b *bench.Bench, binary, home string) []operation {
	t.Helper()
	probe := probeCard(t, b)
	attachments, err := bench.Attachments(probe.Dir)
	if err != nil {
		t.Fatalf("attachments of %s: %v", store.ProbeCard, err)
	}
	statusWarm := func() (time.Duration, error) {
		start := time.Now()
		opened, err := bench.Open(store.Root)
		if err != nil {
			return 0, err
		}
		status, err := verb.New(opened, home).Status(&verb.Request{Actor: "perf"})
		elapsed := time.Since(start)
		if err != nil {
			return 0, err
		}
		return elapsed, checkStatus(status, store.Shape.Cards)
	}
	show := func() (time.Duration, error) {
		start := time.Now()
		opened, err := bench.Open(store.Root)
		if err != nil {
			return 0, err
		}
		detail, _, _, _, err := verb.New(opened, home).Show(&verb.Request{Actor: "perf", Card: store.ProbeCard})
		elapsed := time.Since(start)
		if err != nil {
			return 0, err
		}
		return elapsed, checkDetail(detail, probe, len(attachments))
	}
	config := httphead.Config{Root: store.Root, Home: home, DefaultActor: "perf", Host: "127.0.0.1", Port: 8080, Lang: "en"}
	handler := httphead.Handler(config)
	pageCard := func() (time.Duration, error) {
		request := httptest.NewRequest(http.MethodGet, "/cards/"+store.ProbeCard, nil)
		request.Host = "127.0.0.1:8080"
		request.Header.Set("Accept", "text/html")
		recorder := httptest.NewRecorder()
		start := time.Now()
		handler.ServeHTTP(recorder, request)
		elapsed := time.Since(start)
		return elapsed, checkPage(recorder, probe.Title)
	}
	childDir := t.TempDir()
	statusCold := func() (time.Duration, error) {
		command := exec.Command(binary, "status", "--workbench", store.Root)
		command.Env = childEnv(home)
		command.Dir = childDir
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		start := time.Now()
		if err := command.Start(); err != nil {
			return 0, err
		}
		err := command.Wait()
		elapsed := time.Since(start)
		if err != nil {
			return 0, fmt.Errorf("dinah status: %v\n%s", err, stderr.String())
		}
		if stdout.Len() == 0 {
			return 0, fmt.Errorf("dinah status printed nothing")
		}
		return elapsed, nil
	}
	return []operation{
		{name: "status-warm", run: statusWarm},
		{name: "show", run: show},
		{name: "page-card", run: pageCard},
		{name: "status-cold", run: statusCold},
	}
}

// probeCard finds perf-1 among the live cards.
func probeCard(t *testing.T, b *bench.Bench) *bench.Card {
	t.Helper()
	cards, err := b.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	for _, card := range cards {
		if card.Number == 1 {
			return card
		}
	}
	t.Fatalf("no live card numbered 1")
	return nil
}

// checkStatus is status-warm's answer check: fourteen columns holding every
// live card between them.
func checkStatus(status *verb.Status, cards int) error {
	if len(status.Columns) != 14 {
		return fmt.Errorf("status reported %d columns, the generated flow has 14", len(status.Columns))
	}
	total := 0
	for _, column := range status.Columns {
		total += column.Count
	}
	if total != cards {
		return fmt.Errorf("status counted %d cards, the store holds %d", total, cards)
	}
	return nil
}

// checkDetail is show's answer check. A show naming no fields serves the card,
// its body, its links and its attachments and leaves the checklist out, so the
// check asks for those: the detail is perf-1's and carries every link and
// attachment the card holds.
func checkDetail(detail *verb.Detail, probe *bench.Card, attachments int) error {
	if detail == nil {
		return fmt.Errorf("show answered no card detail")
	}
	if detail.Card.ID != probe.ID {
		return fmt.Errorf("show answered card %s, not %s", detail.Card.ID, probe.ID)
	}
	if len(detail.Links) != len(probe.Links) {
		return fmt.Errorf("show carried %d links, the card holds %d", len(detail.Links), len(probe.Links))
	}
	if len(detail.Attachments) != attachments {
		return fmt.Errorf("show carried %d attachments, the card holds %d", len(detail.Attachments), attachments)
	}
	return nil
}

// checkPage is page-card's answer check: a 200 carrying HTML that names the
// card.
func checkPage(recorder *httptest.ResponseRecorder, title string) error {
	if recorder.Code != http.StatusOK {
		return fmt.Errorf("GET the card page answered %d", recorder.Code)
	}
	if kind := recorder.Header().Get("Content-Type"); !strings.HasPrefix(kind, "text/html") {
		return fmt.Errorf("GET the card page answered %s, not text/html", kind)
	}
	if !strings.Contains(recorder.Body.String(), title) {
		return fmt.Errorf("the card page does not carry the card's title %q", title)
	}
	return nil
}

// childEnv builds the fresh process's environment from nothing but PATH,
// SYSTEMROOT on Windows, the actor and the user base.
func childEnv(home string) []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "DINAH_ACTOR=perf", "DINAH_HOME=" + home}
	if runtime.GOOS == "windows" {
		env = append(env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
	}
	return env
}

// rowFor answers the pinned row for one operation, failing the test when the
// table carries none, which would leave the operation unjudged.
func rowFor(t *testing.T, rows []budget, name string) budget {
	t.Helper()
	for _, row := range rows {
		if row.op == name {
			return row
		}
	}
	t.Fatalf("no budget row for %s on %s", name, runtime.GOOS)
	return budget{}
}

// measureOp runs one operation once as a discarded warm-up and then ten
// times, failing the test when any run's answer check fails.
func measureOp(t *testing.T, op operation) sample {
	t.Helper()
	if _, err := op.run(); err != nil {
		t.Fatalf("%s warm-up: %v", op.name, err)
	}
	runs := make([]time.Duration, 0, measuredRuns)
	for i := 0; i < measuredRuns; i++ {
		elapsed, err := op.run()
		if err != nil {
			t.Fatalf("%s run %d: %v", op.name, i+1, err)
		}
		runs = append(runs, elapsed)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
	median := (runs[measuredRuns/2-1] + runs[measuredRuns/2]) / 2
	return sample{runs: runs, median: median}
}

// judge measures one operation and holds it to its row. An over-budget median
// is measured once more and fails only when the retry is over too; a slack
// budget is measured once more and reported when the retry is slack too,
// failing only in ci mode. Every operation logs one line either way.
func judge(t *testing.T, mode string, store *perfstore.Store, op operation, row budget) {
	t.Helper()
	first := measureOp(t, op)
	multiple := float64(row.limit) / float64(row.basis)
	t.Logf("%-12s median %s  min %s  max %s  budget %s  (%.1fx of %s, %s)",
		op.name, millis(first.median), millis(first.runs[0]), millis(first.runs[len(first.runs)-1]),
		millis(row.limit), multiple, millis(row.basis), row.setBy)
	if first.median > row.limit {
		retry := measureOp(t, op)
		t.Logf("%-12s retry median %s", op.name, millis(retry.median))
		if retry.median > row.limit {
			t.Error(overBudget(op.name, first, retry, row, multiple, store))
		}
		return
	}
	if !slack(row.limit, first.median) {
		return
	}
	retry := measureOp(t, op)
	t.Logf("%-12s retry median %s", op.name, millis(retry.median))
	if !slack(row.limit, retry.median) {
		return
	}
	message := slackMessage(op.name, first, retry, row)
	if mode == "ci" {
		t.Error(message)
		return
	}
	t.Log(message)
}

// slack reports whether a budget is more than six times a median and above
// the 30ms the floor exempts.
func slack(limit, median time.Duration) bool {
	return limit > slackMultiple*median && limit > budgetFloor
}

// tightened is the budget the rule gives a median: three times it, rounded up
// to the next 10ms, and never below 30ms.
func tightened(median time.Duration) time.Duration {
	limit := budgetMultiple * median
	if rest := limit % budgetRounding; rest != 0 {
		limit += budgetRounding - rest
	}
	if limit < budgetFloor {
		limit = budgetFloor
	}
	return limit
}

// overBudget is the failure text for an operation whose two medians both
// exceeded its budget.
func overBudget(name string, first, retry sample, row budget, multiple float64, store *perfstore.Store) string {
	lines := []string{
		fmt.Sprintf("%s over budget: median %s (retry %s) against a budget of %s",
			name, millis(first.median), millis(retry.median), millis(row.limit)),
		"  runs:  " + runList(first.runs),
		"  retry: " + runList(retry.runs),
		fmt.Sprintf("  budget set by %s at %.1fx a CI median of %s", row.setBy, multiple, millis(row.basis)),
		"  reproduce: " + reproduce,
		fmt.Sprintf("  the store is seed %d, DevelopmentShape, digest %s", store.Seed, store.Digest),
	}
	return strings.Join(lines, "\n")
}

// slackMessage is the report for an operation whose two medians both sit more
// than six times under its budget.
func slackMessage(name string, first, retry sample, row budget) string {
	larger := first.median
	if retry.median > larger {
		larger = retry.median
	}
	lines := []string{
		fmt.Sprintf("%s has slack: median %s (retry %s) against a budget of %s, %.0fx",
			name, millis(first.median), millis(retry.median), millis(row.limit), float64(row.limit)/float64(larger)),
		"  a card that makes a read faster tightens its budget in the same pull request",
		fmt.Sprintf("  tighten to about %s (3x the larger of the two medians, rounded up to 10ms), take the final basis",
			millis(tightened(larger))),
		"  from three perf-job runs as the calibration does, and name this card",
	}
	return strings.Join(lines, "\n")
}

// runList renders measured runs as milliseconds, ascending.
func runList(runs []time.Duration) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = thousands(run.Milliseconds())
	}
	return strings.Join(parts, " ") + " ms"
}

// millis renders a duration as whole milliseconds with thousands separated.
func millis(d time.Duration) string {
	return thousands(d.Milliseconds()) + "ms"
}

// thousands renders a whole number with a comma between each group of three
// digits.
func thousands(n int64) string {
	digits := strconv.FormatInt(n, 10)
	if n < 0 {
		return "-" + thousands(-n)
	}
	var out strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}
