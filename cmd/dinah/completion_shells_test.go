package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"dinah/internal/completion"
)

// The script tests drive each shell's real completion script against two
// programs called dinah: a build of this package, and a stub built from
// stubSource. The stub prints what the test asks for and records that it ran,
// which is how a test proves a script never ran the binary, and how it hands a
// script the answer an older build would give.
const (
	// stubOutVariable and stubErrVariable are what the stub writes to its
	// two streams, and stubExitVariable is the code it exits with.
	stubOutVariable  = "DINAH_COMPLETION_STUB_OUT"
	stubErrVariable  = "DINAH_COMPLETION_STUB_ERR"
	stubExitVariable = "DINAH_COMPLETION_STUB_EXIT"
	// recordVariable names the file the stub appends to each time it runs,
	// with its arguments and the words the PowerShell script handed it. The
	// bash and zsh harnesses write their recorded calls to the same file.
	recordVariable = "DINAH_COMPLETION_RECORD"
)

// stubSource is the stub program. It is a program of its own rather than this
// test binary under another name, because this package's initialisation reads
// the repository around it and a stub runs from a fixture directory.
const stubSource = `package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	if record := os.Getenv("DINAH_COMPLETION_RECORD"); record != "" {
		if file, err := os.OpenFile(record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(file, "called %s\n", strings.Join(os.Args[1:], " "))
			fmt.Fprintf(file, "words %s\n", os.Getenv("DINAH_COMPLETE_WORDS"))
			file.Close()
		}
	}
	os.Stdout.WriteString(os.Getenv("DINAH_COMPLETION_STUB_OUT"))
	os.Stderr.WriteString(os.Getenv("DINAH_COMPLETION_STUB_ERR"))
	code, _ := strconv.Atoi(os.Getenv("DINAH_COMPLETION_STUB_EXIT"))
	os.Exit(code)
}
`

// executableName is what a program called dinah is named on this platform.
func executableName() string {
	if runtime.GOOS == "windows" {
		return "dinah.exe"
	}
	return "dinah"
}

// scriptBinaries are the two directories holding a program called dinah,
// built once per test binary because a build takes seconds.
var scriptBinaries struct {
	once sync.Once
	real string
	stub string
	err  error
}

// completionPrograms answers the directory holding this build and the one
// holding the stub.
func completionPrograms(t *testing.T) (string, string) {
	t.Helper()
	scriptBinaries.once.Do(func() {
		base, err := os.MkdirTemp("", "dinah-completion-programs-*")
		if err != nil {
			scriptBinaries.err = err
			return
		}
		real := filepath.Join(base, "real")
		stub := filepath.Join(base, "stub")
		os.MkdirAll(real, 0o755)
		os.MkdirAll(stub, 0o755)
		build := exec.Command("go", "build", "-o", filepath.Join(real, executableName()), ".")
		if out, err := build.CombinedOutput(); err != nil {
			scriptBinaries.err = fmt.Errorf("go build: %v\n%s", err, out)
			return
		}
		source := filepath.Join(base, "stub.go")
		if err := os.WriteFile(source, []byte(stubSource), 0o644); err != nil {
			scriptBinaries.err = err
			return
		}
		stubBuild := exec.Command("go", "build", "-o", filepath.Join(stub, executableName()), source)
		stubBuild.Dir = base
		if out, err := stubBuild.CombinedOutput(); err != nil {
			scriptBinaries.err = fmt.Errorf("go build the stub: %v\n%s", err, out)
			return
		}
		scriptBinaries.real, scriptBinaries.stub = real, stub
	})
	if scriptBinaries.err != nil {
		t.Fatalf("build the programs the scripts run: %v", scriptBinaries.err)
	}
	return scriptBinaries.real, scriptBinaries.stub
}

// skipTheCoverageRun skips a script test in the run that writes the coverage
// profile, which measures the rendering head and would only run every shell a
// second time.
func skipTheCoverageRun(t *testing.T) {
	t.Helper()
	if os.Getenv(coverageChildMarker) != "" {
		t.Skip("the coverage run measures the rendering head, and these tests draw no table")
	}
}

// requiredShells are the shells COMPLETION_REQUIRE_SHELLS names, which a run
// fails rather than skips when it cannot find.
func requiredShells() map[string]bool {
	required := map[string]bool{}
	for _, name := range strings.Split(os.Getenv("COMPLETION_REQUIRE_SHELLS"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			required[name] = true
		}
	}
	return required
}

// findShell locates one shell, and skips the test with the shell's name when
// it is absent, or fails it when COMPLETION_REQUIRE_SHELLS names the shell.
//
// bash is taken from COMPLETION_BASH where that is set and from PATH
// otherwise. On Windows the bash found on PATH can be the WSL launcher rather
// than Git Bash, so a bash there counts as absent unless it reports the
// MSYSTEM variable MSYS2 documents as naming its environment.
func findShell(t *testing.T, name string) string {
	t.Helper()
	path, why := locateShell(name)
	if path == "" {
		if requiredShells()[name] {
			t.Fatalf("%s is required by COMPLETION_REQUIRE_SHELLS and was not found: %s", name, why)
		}
		t.Skipf("%s was not found, so its completion script is not tested here: %s", name, why)
	}
	t.Logf("testing the %s script with %s", name, path)
	return path
}

// locateShell answers where a shell is and, when it is nowhere, why.
func locateShell(name string) (string, string) {
	if name == "bash" && os.Getenv("COMPLETION_BASH") != "" {
		path := os.Getenv("COMPLETION_BASH")
		if _, err := os.Stat(path); err != nil {
			return "", "COMPLETION_BASH names " + path + ", which does not exist"
		}
		return checkBash(path)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", name + " is not on PATH"
	}
	if name == "bash" {
		return checkBash(path)
	}
	return path, ""
}

// checkBash refuses a bash on Windows that does not report MSYSTEM, which is
// how the WSL launcher is told apart from Git Bash.
func checkBash(path string) (string, string) {
	if runtime.GOOS != "windows" {
		return path, ""
	}
	out, err := exec.Command(path, "-c", `printf %s "$MSYSTEM"`).Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "", path + " reports no MSYSTEM, so it is not Git Bash"
	}
	return path, ""
}

// scriptRun is how one harness is started: the directory it stands in, the
// program directory put first on PATH, and what the stub is told to do.
type scriptRun struct {
	dir     string
	program string
	stubOut string
	stubErr string
	stubRun bool
	exit    int
}

// shellEnvironment is the environment a shell under test runs in: this
// process's own, with PATH led by the program directory, HOME and the fish
// data directories moved into a throwaway folder, and the stub's settings.
func shellEnvironment(t *testing.T, run scriptRun, record string) []string {
	t.Helper()
	throwaway := t.TempDir()
	drop := map[string]bool{"PATH": true, "HOME": true, "XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_CACHE_HOME": true}
	var env []string
	for _, pair := range os.Environ() {
		name, _, _ := strings.Cut(pair, "=")
		if drop[strings.ToUpper(name)] || strings.HasPrefix(name, "DINAH_COMPLETION") {
			continue
		}
		env = append(env, pair)
	}
	env = append(env,
		"PATH="+run.program+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+throwaway,
		"XDG_CONFIG_HOME="+filepath.Join(throwaway, "config"),
		"XDG_DATA_HOME="+filepath.Join(throwaway, "data"),
		"XDG_CACHE_HOME="+filepath.Join(throwaway, "cache"),
		recordVariable+"="+filepath.ToSlash(record),
	)
	if run.stubRun {
		env = append(env,
			stubOutVariable+"="+run.stubOut,
			stubErrVariable+"="+run.stubErr,
			stubExitVariable+"="+strconv.Itoa(run.exit),
		)
	}
	return env
}

// runHarness runs a shell over a harness script and answers what each case
// printed and the lines printed before the first case, failing on anything
// the shell wrote to stderr. The record file the stub appends to is read back
// into lastRecord, for the one test that reads what the stub was handed.
func runHarness(t *testing.T, shell string, args []string, run scriptRun) (map[string][]string, string) {
	t.Helper()
	record := filepath.Join(t.TempDir(), "record.txt")
	command := exec.Command(shell, args...)
	command.Dir = run.dir
	command.Env = shellEnvironment(t, run, record)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("%s %v: %v\nstdout:\n%s\nstderr:\n%s", shell, args, err, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("%s printed to stderr:\n%s", shell, stderr.String())
	}
	recorded, _ := os.ReadFile(record)
	lastRecord = string(recorded)
	cases := map[string][]string{}
	current := ""
	var preamble []string
	for _, line := range strings.Split(strings.ReplaceAll(stdout.String(), "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "case "):
			current = strings.TrimPrefix(line, "case ")
			cases[current] = []string{}
		case line == "end":
			current = ""
		case current != "":
			cases[current] = append(cases[current], line)
		case strings.TrimSpace(line) != "":
			preamble = append(preamble, line)
		}
	}
	return cases, strings.Join(preamble, "\n")
}

// lastRecord is what the stub recorded during the last harness run.
var lastRecord string

// lines picks the lines of one case that begin with a prefix, with the prefix
// taken off.
func lines(caseLines []string, prefix string) []string {
	var picked []string
	for _, line := range caseLines {
		if strings.HasPrefix(line, prefix) {
			picked = append(picked, strings.TrimPrefix(line, prefix))
		}
	}
	return picked
}

// quoteForShell single-quotes a word for bash, zsh and fish, which all read a
// single-quoted word literally when it carries no single quote and no
// backslash, which none of these lines does.
func quoteForShell(t *testing.T, word string) string {
	t.Helper()
	if strings.ContainsAny(word, `'\`) {
		t.Fatalf("the harness cannot quote %q", word)
	}
	return "'" + word + "'"
}

// scriptFixture is a completion workbench with a file and a directory beside
// it, for the file and directory modes to find.
func scriptFixture(t *testing.T) string {
	t.Helper()
	root := completionBench(t)
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// stubControl is what a stub answering like a current build prints, which is
// the accepting case every refusing stub case stands beside.
const stubControl = "dinah-complete 1 words\nfoo\t\n"

// skewCases are the stub answers every script must offer nothing for: an
// older build refusing the callback word, and a header of another protocol.
var skewCases = map[string]scriptRun{
	"older": {stubRun: true, stubErr: "dinah.unknown-command Dinah offers no command called __complete\n", exit: 2},
	"other": {stubRun: true, stubOut: "dinah-complete 2 words\nfoo\t\n"},
	"none":  {stubRun: true, stubOut: "complete me\nfoo\t\n"},
}

// bashHarness is the bash harness. compopt is replaced by a recorder only
// where bash has it as a builtin, so a bash without it, which is what macOS
// ships, runs the script's degraded path for real.
const bashHarness = `if [[ $(type -t compopt) == builtin ]]; then
  compopt() { printf 'compopt %s\n' "$*" >> "$DINAH_COMPLETION_RECORD"; }
  echo "compopt builtin"
else
  echo "compopt absent"
fi
echo "bash $BASH_VERSION"
eval "$(dinah completion bash)"
t() {
  COMP_LINE=$2
  if [[ -n $3 ]]; then COMP_POINT=$3; else COMP_POINT=${#COMP_LINE}; fi
  COMP_WORDS=(dinah)
  COMP_CWORD=1
  : > "$DINAH_COMPLETION_RECORD"
  _dinah_complete
  echo "case $1"
  local r
  for r in "${COMPREPLY[@]}"; do echo "reply $r"; done
  cat "$DINAH_COMPLETION_RECORD"
  echo end
}
`

// runBash runs the bash harness with one call per case, each case a name, a
// line and an optional cursor.
func runBash(t *testing.T, bash string, run scriptRun, calls [][3]string) (map[string][]string, string) {
	t.Helper()
	var script strings.Builder
	script.WriteString(bashHarness)
	for _, call := range calls {
		fmt.Fprintf(&script, "t %s %s %s\n", call[0], quoteForShell(t, call[1]), quoteForShell(t, call[2]))
	}
	path := filepath.Join(t.TempDir(), "harness.bash")
	if err := os.WriteFile(path, []byte(script.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return runHarness(t, bash, []string{"--norc", "--noprofile", filepath.ToSlash(path)}, run)
}

// loadedScript is a script a stub run sources from a file, since the stub
// cannot print it.
func loadedScript(t *testing.T, shell string) string {
	t.Helper()
	script, _ := completionScriptFor(shell)
	path := filepath.Join(t.TempDir(), "script."+shell)
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(path)
}

// TestCompletionScriptBash drives the bash script: dinah-601/criteria/6, 9,
// 19 and 27, and the bash halves of 8 and 11.
func TestCompletionScriptBash(t *testing.T) {
	skipTheCoverageRun(t)
	bash := findShell(t, "bash")
	real, stub := completionPrograms(t)
	root := scriptFixture(t)
	cases, preamble := runBash(t, bash, scriptRun{dir: root, program: real}, [][3]string{
		{"colu", "dinah query colu", ""},
		{"sp", "dinah query column:sp", ""},
		{"re", "dinah query state:re", ""},
		{"comma", "dinah query column:spec,te", ""},
		{"files", "dinah attach fx-1 ", ""},
		{"dirs", "dinah init ", ""},
	})
	t.Logf("harness preamble:\n%s", preamble)
	builtin := strings.Contains(preamble, "compopt builtin")
	want := map[string]string{"colu": "column:", "sp": "spec", "re": "ready", "comma": "spec,test", "files": "", "dirs": ""}
	for name, reply := range want {
		if got := strings.Join(lines(cases[name], "reply "), " "); got != reply {
			t.Errorf("%s: COMPREPLY (%s), wanted (%s)", name, got, reply)
		}
	}
	options := map[string]string{"colu": "-o nospace", "files": "-o default", "dirs": "-o dirnames", "sp": ""}
	for name, option := range options {
		got := strings.Join(lines(cases[name], "compopt "), " ")
		if builtin && got != option {
			t.Errorf("%s: compopt %q, wanted %q", name, got, option)
		}
		if !builtin && got != "" {
			t.Errorf("%s: compopt was called on a bash that has none: %q", name, got)
		}
	}
	moves, _ := runBash(t, bash, scriptRun{dir: moveBench(t), program: real}, [][3]string{{"move", "dinah move fx-2 ", ""}})
	if got := lines(moves["move"], "reply "); len(got) != 1 || got[0] != "intake" {
		t.Errorf("the move with one legal destination gave COMPREPLY %q, wanted the single entry intake", got)
	}
	empty := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
	none, _ := runBash(t, bash, scriptRun{dir: empty, program: real}, [][3]string{{"show", "dinah show ", ""}, {"first", "dinah sh", ""}})
	if got := lines(none["show"], "reply "); len(got) != 0 {
		t.Errorf("with no workbench show offered %q", got)
	}
	if got := lines(none["first"], "reply "); strings.Join(got, " ") != "show" {
		t.Errorf("with no workbench the first word offered %q, wanted show", got)
	}
	stubbed := func(run scriptRun, calls [][3]string) map[string][]string {
		run.dir, run.program = root, stub
		script := loadedScript(t, "bash")
		var harness strings.Builder
		harness.WriteString(strings.Replace(bashHarness, `eval "$(dinah completion bash)"`, "source "+quoteForShell(t, script), 1))
		for _, call := range calls {
			fmt.Fprintf(&harness, "t %s %s %s\n", call[0], quoteForShell(t, call[1]), quoteForShell(t, call[2]))
		}
		path := filepath.Join(t.TempDir(), "harness.bash")
		os.WriteFile(path, []byte(harness.String()), 0o644)
		got, _ := runHarness(t, bash, []string{"--norc", "--noprofile", filepath.ToSlash(path)}, run)
		return got
	}
	control := stubbed(scriptRun{stubRun: true, stubOut: stubControl}, [][3]string{{"control", "dinah query colu", ""}})
	if strings.Join(lines(control["control"], "reply "), " ") != "foo" || len(lines(control["control"], "called ")) != 1 {
		t.Fatalf("the stub harness does not work, so nothing below proves anything: %q", control["control"])
	}
	cursor := stubbed(scriptRun{stubRun: true, stubOut: stubControl}, [][3]string{
		{"inside", "dinah query colu", "8"},
		{"accent", "dinah query colümn:", ""},
		{"early", "dinah add Café --sev", ""},
	})
	for name, got := range cursor {
		if len(lines(got, "reply ")) != 0 || len(lines(got, "called ")) != 0 {
			t.Errorf("%s: the script ran the program or offered %q", name, lines(got, "reply "))
		}
	}
	if len(cursor) != 3 {
		t.Errorf("read %d cursor cases, wanted 3", len(cursor))
	}
	t.Logf("the ASCII end-of-line rule ran %d cases on %s", len(cursor), strings.TrimSpace(strings.SplitN(preamble, "\n", 3)[1]))
	for name, run := range skewCases {
		got := stubbed(run, [][3]string{{name, "dinah query colu", ""}})
		if len(lines(got[name], "reply ")) != 0 || len(lines(got[name], "called ")) != 1 {
			t.Errorf("%s: the script offered %q, and ran the program %d times", name, lines(got[name], "reply "), len(lines(got[name], "called ")))
		}
	}
}

// completionScriptFor is the printed script for a shell, which a harness that
// runs the stub sources from a file. dinah completion prints exactly this, as
// TestTheCompletionVerbPrintsEachScript holds.
func completionScriptFor(shell string) (string, bool) {
	return completion.Script(shell)
}

// zshHarness is the zsh harness: compadd, _files and compdef are recorders,
// and compadd also records the display array the script built, which zsh's
// dynamic scoping lets it read.
const zshHarness = `compadd() { print -r -- "compadd $*" >> $DINAH_COMPLETION_RECORD; local s; for s in "${shown[@]}"; do print -r -- "shown $s" >> $DINAH_COMPLETION_RECORD; done }
_files() { print -r -- "_files $*" >> $DINAH_COMPLETION_RECORD }
compdef() { print -r -- "compdef $*" >> $DINAH_COMPLETION_RECORD }
print -r -- "zsh $ZSH_VERSION"
LOAD
t() {
  local name=$1
  shift
  words=("$@")
  CURRENT=$#words
  PREFIX=${words[-1]}
  : >| $DINAH_COMPLETION_RECORD
  _dinah
  local rc=$?
  print -r -- "case $name"
  print -r -- "status $rc"
  cat $DINAH_COMPLETION_RECORD
  print -r -- end
}
`

// runZsh runs the zsh harness, loading the script by the line given.
func runZsh(t *testing.T, zsh, load string, run scriptRun, calls [][]string) map[string][]string {
	t.Helper()
	var script strings.Builder
	script.WriteString(strings.Replace(zshHarness, "LOAD", load, 1))
	for _, call := range calls {
		quoted := make([]string, 0, len(call))
		for _, word := range call {
			quoted = append(quoted, quoteForShell(t, word))
		}
		script.WriteString("t " + strings.Join(quoted, " ") + "\n")
	}
	path := filepath.Join(t.TempDir(), "harness.zsh")
	if err := os.WriteFile(path, []byte(script.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cases, preamble := runHarness(t, zsh, []string{"-f", filepath.ToSlash(path)}, run)
	t.Logf("harness preamble: %s", preamble)
	return cases
}

// TestCompletionScriptZsh drives the zsh script: dinah-601/criteria/25, and
// the zsh halves of 7, 8, 11 and 19.
func TestCompletionScriptZsh(t *testing.T) {
	skipTheCoverageRun(t)
	zsh := findShell(t, "zsh")
	real, stub := completionPrograms(t)
	root := scriptFixture(t)
	cases := runZsh(t, zsh, `eval "$(dinah completion zsh)"`, scriptRun{dir: root, program: real}, [][]string{
		{"comma", "dinah", "query", "column:spec,te"},
		{"fields", "dinah", "show", "x", "--fields", "card,bo"},
		{"files", "dinah", "attach", "fx-1", ""},
		{"dirs", "dinah", "init", ""},
		{"title", "dinah", "show", "fx-3"},
	})
	if got := lines(cases["comma"], "compadd "); len(got) != 1 || !strings.Contains(got[0], "-U") || !strings.HasSuffix(got[0], "-- column:spec,test") {
		t.Errorf("comma: compadd called as %q, wanted -U and the word column:spec,test", got)
	}
	if got := lines(cases["fields"], "compadd "); len(got) != 1 || !strings.HasSuffix(got[0], "-- card,body") {
		t.Errorf("fields: compadd called as %q, wanted the word card,body", got)
	}
	if got := lines(cases["files"], "_files"); len(got) != 1 || strings.TrimSpace(got[0]) != "" {
		t.Errorf("files: _files called as %q", got)
	}
	if got := lines(cases["dirs"], "_files "); len(got) != 1 || got[0] != "-/" {
		t.Errorf("dirs: _files called as %q, wanted -/", got)
	}
	if got := lines(cases["title"], "shown "); len(got) != 1 || got[0] != `fx-3 -- Say "hi" to "them"` {
		t.Errorf("title: displayed %q", got)
	}
	empty := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
	none := runZsh(t, zsh, `eval "$(dinah completion zsh)"`, scriptRun{dir: empty, program: real}, [][]string{{"show", "dinah", "show", ""}})
	if got := lines(none["show"], "compadd "); len(got) != 0 {
		t.Errorf("with no workbench compadd was called as %q", got)
	}
	load := "source " + quoteForShell(t, loadedScript(t, "zsh"))
	control := runZsh(t, zsh, load, scriptRun{dir: root, program: stub, stubRun: true, stubOut: stubControl}, [][]string{{"control", "dinah", "query", "colu"}})
	if got := lines(control["control"], "compadd "); len(got) != 1 || !strings.HasSuffix(got[0], "-- foo") {
		t.Fatalf("the stub harness does not work, so nothing below proves anything: %q", control["control"])
	}
	for name, run := range skewCases {
		run.dir, run.program = root, stub
		got := runZsh(t, zsh, load, run, [][]string{{name, "dinah", "query", "colu"}})
		if len(lines(got[name], "compadd ")) != 0 || len(lines(got[name], "called ")) != 1 {
			t.Errorf("%s: compadd was called as %q", name, lines(got[name], "compadd "))
		}
	}
}

// fishHarness is the fish harness, loading the script by LOAD and printing
// what complete -C answers for each line.
const fishHarness = `echo "fish $version"
LOAD
function t
  echo "case $argv[1]"
  complete -C $argv[2]
  echo end
end
`

// runFish runs the fish harness.
func runFish(t *testing.T, fish, load string, run scriptRun, calls [][2]string) map[string][]string {
	t.Helper()
	var script strings.Builder
	script.WriteString(strings.Replace(fishHarness, "LOAD", load, 1))
	for _, call := range calls {
		fmt.Fprintf(&script, "t %s %s\n", call[0], quoteForShell(t, call[1]))
	}
	path := filepath.Join(t.TempDir(), "harness.fish")
	if err := os.WriteFile(path, []byte(script.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cases, preamble := runHarness(t, fish, []string{"--no-config", filepath.ToSlash(path)}, run)
	t.Logf("harness preamble: %s", preamble)
	return cases
}

// TestCompletionScriptFish drives the fish script: dinah-601/criteria/26, and
// the fish halves of 7, 8, 11 and 19.
func TestCompletionScriptFish(t *testing.T) {
	skipTheCoverageRun(t)
	fish := findShell(t, "fish")
	real, stub := completionPrograms(t)
	root := scriptFixture(t)
	cases := runFish(t, fish, "dinah completion fish | source", scriptRun{dir: root, program: real}, [][2]string{
		{"comma", "dinah query column:spec,te"},
		{"fields", "dinah show x --fields card,bo"},
		{"files", "dinah attach fx-1 al"},
		{"dirs", "dinah init be"},
		{"title", "dinah show fx-3"},
	})
	want := map[string]string{
		"comma":  "column:spec,test",
		"fields": "card,body",
		"files":  "alpha.txt",
		"dirs":   "beta/",
		"title":  "fx-3\tSay \"hi\" to \"them\"",
	}
	for name, line := range want {
		if got := strings.Join(cases[name], "\n"); got != line {
			t.Errorf("%s: complete -C printed %q, wanted %q", name, got, line)
		}
	}
	empty := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
	none := runFish(t, fish, "dinah completion fish | source", scriptRun{dir: empty, program: real}, [][2]string{{"show", "dinah show "}})
	if len(none["show"]) != 0 {
		t.Errorf("with no workbench show offered %q", none["show"])
	}
	load := "source " + quoteForShell(t, loadedScript(t, "fish"))
	control := runFish(t, fish, load, scriptRun{dir: root, program: stub, stubRun: true, stubOut: stubControl}, [][2]string{{"control", "dinah query "}})
	if strings.Join(control["control"], " ") != "foo" {
		t.Fatalf("the stub harness does not work, so nothing below proves anything: %q", control["control"])
	}
	for name, run := range skewCases {
		run.dir, run.program = root, stub
		got := runFish(t, fish, load, run, [][2]string{{name, "dinah query "}})
		if len(got[name]) != 0 {
			t.Errorf("%s: fish offered %q", name, got[name])
		}
	}
}

// powershellHarness is the PowerShell harness. For each line it sets
// $LASTEXITCODE to a value nothing else sets, reads the output encoding, runs
// TabExpansion2, and prints each match with the state the script had to
// restore.
const powershellHarness = `$ErrorActionPreference = 'Stop'
Write-Output ("powershell {0}" -f $PSVersionTable.PSVersion)
LOAD
function T([string]$name, [string]$line) {
  $global:LASTEXITCODE = 42
  $before = [Console]::OutputEncoding.WebName
  $r = TabExpansion2 -inputScript $line -cursorColumn $line.Length
  Write-Output "case $name"
  Write-Output ("index {0} {1}" -f $r.ReplacementIndex, $r.ReplacementLength)
  foreach ($m in $r.CompletionMatches) {
    Write-Output ("match {0}` + "`t" + `{1}` + "`t" + `{2}" -f $m.CompletionText, $m.ToolTip, $m.ResultType)
    Write-Output ("line {0}" -f ($line.Substring(0, $r.ReplacementIndex) + $m.CompletionText + $line.Substring($r.ReplacementIndex + $r.ReplacementLength)))
  }
  Write-Output ("exit {0}" -f $global:LASTEXITCODE)
  Write-Output ("encoding {0} {1}" -f $before, [Console]::OutputEncoding.WebName)
  Write-Output ("variable {0}" -f (Test-Path env:DINAH_COMPLETE_WORDS))
  Write-Output "end"
}
`

// runPowerShell runs the PowerShell harness under one edition.
func runPowerShell(t *testing.T, edition, load string, run scriptRun, calls [][2]string) map[string][]string {
	t.Helper()
	var script strings.Builder
	script.WriteString(strings.Replace(powershellHarness, "LOAD", load, 1))
	for _, call := range calls {
		fmt.Fprintf(&script, "T '%s' '%s'\n", call[0], strings.ReplaceAll(call[1], "'", "''"))
	}
	path := filepath.Join(t.TempDir(), "harness.ps1")
	if err := os.WriteFile(path, []byte(script.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path}
	cases, preamble := runHarness(t, edition, args, run)
	t.Logf("harness preamble: %s", preamble)
	return cases
}

// TestCompletionScriptPowerShell drives the PowerShell script under Windows
// PowerShell and PowerShell 7 wherever each is present:
// dinah-601/criteria/20 and 24, and the PowerShell halves of 7, 8, 11 and 19.
func TestCompletionScriptPowerShell(t *testing.T) {
	skipTheCoverageRun(t)
	for _, edition := range []string{"powershell", "pwsh"} {
		t.Run(edition, func(t *testing.T) {
			shell := findShell(t, edition)
			real, stub := completionPrograms(t)
			root := scriptFixture(t)
			load := "dinah completion powershell | Out-String | Invoke-Expression"
			cases := runPowerShell(t, shell, load, scriptRun{dir: root, program: real}, [][2]string{
				{"comma", "dinah query column:spec,te"},
				{"fields", "dinah show x --fields card,bo"},
				{"title", "dinah show fx-3"},
				{"files", "dinah attach fx-1 al"},
				{"dirs", "dinah init be"},
				{"quote", `dinah --actor 'a"b' show fx-3`},
			})
			expect := map[string][2]string{
				"comma":  {"match test\ttest\tParameterValue", "line dinah query column:spec,test"},
				"fields": {"match body\tbody\tParameterValue", "line dinah show x --fields card,body"},
				"title":  {"match fx-3\tSay \"hi\" to \"them\"\tParameterValue", ""},
				"files":  {"match alpha.txt\talpha.txt\tProviderItem", ""},
				"dirs":   {"match beta\tbeta\tProviderContainer", ""},
				"quote":  {"match fx-3\tSay \"hi\" to \"them\"\tParameterValue", ""},
			}
			for name, want := range expect {
				got := cases[name]
				if matches := lines(got, "match "); len(matches) != 1 || "match "+matches[0] != want[0] {
					t.Errorf("%s: matches %q, wanted %q", name, matches, want[0])
				}
				if want[1] != "" && len(lines(got, "line ")) == 1 && "line "+lines(got, "line ")[0] != want[1] {
					t.Errorf("%s: the completed line reads %q, wanted %q", name, lines(got, "line "), want[1])
				}
				if exit := lines(got, "exit "); len(exit) != 1 || exit[0] != "42" {
					t.Errorf("%s: $LASTEXITCODE was %q after the call, wanted 42", name, exit)
				}
				encoding := strings.Fields(strings.Join(lines(got, "encoding "), ""))
				if len(encoding) != 2 || encoding[0] != encoding[1] {
					t.Errorf("%s: the output encoding went from %q", name, encoding)
				}
				if variable := lines(got, "variable "); len(variable) != 1 || variable[0] != "False" {
					t.Errorf("%s: DINAH_COMPLETE_WORDS exists after the call: %q", name, variable)
				}
			}
			if index := lines(cases["comma"], "index "); len(index) != 1 || index[0] != "24 2" {
				t.Errorf("comma: the replacement starts at %q, wanted 24 2, the start of te", index)
			}
			empty := t.TempDir()
			t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
			none := runPowerShell(t, shell, load, scriptRun{dir: empty, program: real}, [][2]string{{"show", "dinah show "}})
			if got := lines(none["show"], "match "); len(got) != 0 {
				t.Errorf("with no workbench show offered %q", got)
			}
			script := filepath.Join(t.TempDir(), "script.ps1")
			text, _ := completionScriptFor("powershell")
			os.WriteFile(script, []byte(text), 0o644)
			stubLoad := ". '" + script + "'"
			control := runPowerShell(t, shell, stubLoad, scriptRun{dir: root, program: stub, stubRun: true, stubOut: "dinah-complete 1 words\ntest\t\n"}, [][2]string{{"control", "dinah query column:spec,te"}})
			if got := lines(control["control"], "match "); len(got) != 1 || got[0] != "test\ttest\tParameterValue" {
				t.Fatalf("the stub harness does not work, so nothing below proves anything: %q", control["control"])
			}
			recorded := stubWords(t, lastRecord)
			if len(recorded.Words) == 0 || recorded.Words[len(recorded.Words)-1] != "column:spec,te" || recorded.Replacing != "te" {
				t.Errorf("the callback was handed %+v, wanted words ending in column:spec,te and replacing te", recorded)
			}
			for name, run := range skewCases {
				run.dir, run.program = root, stub
				got := runPowerShell(t, shell, stubLoad, run, [][2]string{{name, "dinah query colu"}})
				if matches := lines(got[name], "match "); len(matches) != 0 {
					t.Errorf("%s: PowerShell offered %q", name, matches)
				}
			}
		})
	}
}

// handedWords is what the PowerShell script put in DINAH_COMPLETE_WORDS.
type handedWords struct {
	Words     []string `json:"words"`
	Replacing string   `json:"replacing"`
}

// stubWords reads the words the stub recorded being handed.
func stubWords(t *testing.T, record string) handedWords {
	t.Helper()
	var handed handedWords
	found := false
	for _, line := range strings.Split(record, "\n") {
		raw, ok := strings.CutPrefix(line, "words ")
		if !ok {
			continue
		}
		found = true
		if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &handed); err != nil {
			t.Fatalf("the recorded words %q do not read: %v", raw, err)
		}
	}
	if !found {
		t.Fatalf("the stub recorded no words:\n%s", record)
	}
	return handed
}
