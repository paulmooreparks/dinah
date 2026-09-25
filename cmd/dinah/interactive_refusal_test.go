package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/screen"
)

// refusedWithoutDrawing fails unless a run exited 2 with the refusal named
// first on stderr, wrote nothing to stdout, and drew nothing on the terminal.
func refusedWithoutDrawing(t *testing.T, run tuiRun, name string) {
	t.Helper()
	if run.code != 2 {
		t.Errorf("exited %d, wanted 2: %s", run.code, run.errw)
	}
	if got := refusalNameOf(run.errw); got != name {
		t.Errorf("refused %q, wanted %s: %s", got, name, run.errw)
	}
	if run.out != "" {
		t.Errorf("stdout carried %q", run.out)
	}
	if run.output != "" || run.finished {
		t.Errorf("the terminal was drawn on: %q", run.output)
	}
}

// TestTheCommandDrawsItsViewAndRefusesWhatItCannotStart is
// dinah-603/criteria/2. dinah tui draws the board with no view named and the
// agenda when it is named; a name no layer declares is refused as dinah view
// refuses it, before anything is drawn; --json is refused dinah.malformed
// with detail tui --json before the workbench is opened, with the machine
// refusal report on stdout and nothing else; a second positional is refused
// by the parser. The accepting case stands beside them: a 60 by 12 seam
// terminal starts the interface.
func TestTheCommandDrawsItsViewAndRefusesWhatItCannotStart(t *testing.T) {
	root := tuiBench(t)
	board := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("q"), 60, 12))
	if board.code != 0 || board.model == nil || board.model.req.View != "board" {
		t.Fatalf("dinah tui at 60x12 exited %d without drawing the board: %s", board.code, board.errw)
	}
	agenda := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("q"), 100, 30), "agenda")
	if agenda.code != 0 || agenda.model == nil || agenda.model.answer.View.Name != "agenda" {
		t.Fatalf("dinah tui agenda exited %d without drawing the agenda: %s", agenda.code, agenda.errw)
	}

	unknown := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("q"), 100, 30), "nosuch")
	refusedWithoutDrawing(t, unknown, contract.UnknownView)
	view := runCLI(t, root, "view", "nosuch")
	if unknown.errw != view.errw || unknown.code != view.code {
		t.Errorf("dinah tui nosuch said %q (%d), and dinah view nosuch says %q (%d)", unknown.errw, unknown.code, view.errw, view.code)
	}

	outside := t.TempDir()
	machine := runTUIThrough(t, outside, tuiSeam(t, strings.NewReader("q"), 100, 30), "--json")
	if machine.code != 2 || refusalNameOf(machine.errw) != contract.Malformed {
		t.Fatalf("--json in a directory holding no workbench exited %d: %s", machine.code, machine.errw)
	}
	var report refusalReport
	if err := json.Unmarshal([]byte(machine.out), &report); err != nil {
		t.Fatalf("stdout under --json is not the machine refusal report: %v\n%s", err, machine.out)
	}
	if report.Refusal != contract.Malformed || report.Detail != "tui --json" || report.Outcome != contract.OutcomeRefused {
		t.Errorf("the machine report is %+v, wanted malformed with detail tui --json", report)
	}
	if machine.output != "" {
		t.Errorf("--json drew on the terminal: %q", machine.output)
	}

	second := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("q"), 100, 30), "board", "agenda")
	if second.code != 2 || second.out != "" || second.output != "" {
		t.Errorf("a second positional exited %d, wrote %q and drew %q", second.code, second.out, second.output)
	}
	if got := refusalNameOf(second.errw); got != contract.Usage {
		t.Errorf("a second positional was refused %q, wanted %s", got, contract.Usage)
	}
}

// TestTheTerminalRefusalsNameTheirReason is dinah-603/criteria/3. Each reason
// the terminal check gives is raised by the seam state that raises it, exits
// 2, writes the refusal to stderr and nothing to stdout, and draws nothing;
// the too-small refusal carries the size and the minimum; a POSIX TERM of
// dumb and an entry lacking kcuu1 are refused off Windows; and a 60 by 12
// window with both streams terminals and a full entry is accepted.
func TestTheTerminalRefusalsNameTheirReason(t *testing.T) {
	root := tuiBench(t)
	type refusedCase struct {
		name   string
		seam   func() *interactiveSeams
		term   string
		reason string
		values map[string]string
		posix  bool
	}
	noKcuu1, err := os.ReadFile(filepath.Join("..", "..", "internal", "screen", "testdata", "terminfo", "64", "dinah-no-cup"))
	if err != nil {
		t.Fatal(err)
	}
	withoutArrows, err := screen.ParseTerminfo(noKcuu1)
	if err != nil {
		t.Fatal(err)
	}
	cases := []refusedCase{
		{name: "stdin is not a terminal", reason: contract.WatchNotATerminal, seam: func() *interactiveSeams {
			seam := tuiSeam(t, strings.NewReader("q"), 100, 30)
			seam.stdinTerminal = false
			return seam
		}},
		{name: "stdout is not a terminal", reason: contract.WatchNotATerminal, seam: func() *interactiveSeams {
			seam := tuiSeam(t, strings.NewReader("q"), 100, 30)
			seam.stdoutTerminal = false
			return seam
		}},
		{name: "no size", reason: contract.WatchNoSize, seam: func() *interactiveSeams {
			return tuiSeam(t, strings.NewReader("q"), 0, 0)
		}},
		{name: "59 columns", reason: contract.WatchTooSmall, values: map[string]string{"size": "59x12", "minimum": "60x12"}, seam: func() *interactiveSeams {
			return tuiSeam(t, strings.NewReader("q"), 59, 12)
		}},
		{name: "11 rows", reason: contract.WatchTooSmall, values: map[string]string{"size": "60x11", "minimum": "60x12"}, seam: func() *interactiveSeams {
			return tuiSeam(t, strings.NewReader("q"), 60, 11)
		}},
		{name: "TERM is dumb", reason: contract.TUIDumbTerminal, term: "dumb", posix: true, seam: func() *interactiveSeams {
			return tuiSeam(t, strings.NewReader("q"), 100, 30)
		}},
		{name: "the entry lacks kcuu1", reason: contract.TUINoKeyDescription, values: map[string]string{"capability": "kcuu1"}, posix: true, seam: func() *interactiveSeams {
			seam := tuiSeam(t, strings.NewReader("q"), 100, 30)
			seam.terminfo = withoutArrows
			return seam
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.posix && runtime.GOOS == "windows" {
				t.Skip("the terminfo reasons are checked on every GOOS but windows")
			}
			seam := c.seam()
			if c.term != "" {
				tuiTerm = c.term
				defer func() { tuiTerm = "xterm" }()
			}
			run := runTUIThrough(t, root, seam)
			refusedWithoutDrawing(t, run, contract.TUIUnavailable)
			first := strings.SplitN(run.errw, "\n", 2)[0]
			if !strings.Contains(first, "("+c.reason+")") {
				t.Errorf("the refusal does not name the reason %s: %q", c.reason, first)
			}
			for _, want := range c.values {
				if !strings.Contains(first, want) {
					t.Errorf("the refusal does not carry %q: %q", want, first)
				}
			}
		})
	}
	accepted := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("q"), 60, 12))
	if accepted.code != 0 || !accepted.finished {
		t.Errorf("a 60x12 window with both streams terminals and a full entry exited %d: %s", accepted.code, accepted.errw)
	}
}

// TestARedirectedStdoutLeavesTheFileEmpty is the last clause of
// dinah-603/criteria/3: the built binary run with its stdout redirected to a
// file refuses and leaves the file empty.
func TestARedirectedStdoutLeavesTheFileEmpty(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH, so the binary cannot be built")
	}
	root := tuiBench(t)
	dir := t.TempDir()
	binary := filepath.Join(dir, "dinah"+exeSuffix())
	if out, err := exec.Command(gobin, "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	file, err := os.Create(filepath.Join(dir, "redirected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	child := exec.Command(binary, "tui")
	child.Dir = root
	child.Stdout = file
	var stderr strings.Builder
	child.Stderr = &stderr
	child.Env = append(os.Environ(), "DINAH_ACTOR=alka", "DINAH_HOME="+filepath.Join(dir, "home"))
	err = child.Run()
	exit, _ := err.(*exec.ExitError)
	if exit == nil || exit.ExitCode() != 2 {
		t.Fatalf("dinah tui with stdout redirected answered %v, wanted exit 2: %s", err, stderr.String())
	}
	if !strings.HasPrefix(stderr.String(), contract.TUIUnavailable+" ") {
		t.Errorf("stderr is %q, wanted the tui-unavailable refusal", stderr.String())
	}
	written, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Errorf("the redirected file holds %q", written)
	}
}

// exeSuffix is what a built binary's name ends in on this GOOS.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
