package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// tuiHead runs the terminal head in this process. It is nil in the main
// binary, whose runTUI launches dinah-tui instead, and set by tui_head.go in
// the build tagged tui.
var tuiHead func(s *session, parsed *arguments) int

// tuiEntry is true only in the build tagged tui, where main reads its
// arguments as those of dinah tui when no launcher started it, and where run
// refuses a launcher from another build before any command runs.
var tuiEntry bool

// tuiLauncherVariable is the environment variable dinah tui sets for
// dinah-tui to its own build identity. Its presence says a launcher started
// dinah-tui, and its value is what the skew check compares.
const tuiLauncherVariable = "DINAH_TUI_LAUNCHER"

// tuiProgram is the file name of the terminal UI's program on this GOOS.
func tuiProgram() string {
	if runtime.GOOS == "windows" {
		return "dinah-tui.exe"
	}
	return "dinah-tui"
}

// runTUI refuses a machine format, then runs the terminal head where this
// binary carries it, and otherwise finds dinah-tui and hands it the command
// line. The main dinah binary carries no terminal head, so no command pays
// for the terminal UI's library's start-up work.
func runTUI(s *session, parsed *arguments) int {
	if s.format != formatHuman {
		// The detail names the command and the flag it refuses, as dinah
		// view names --watch --json.
		detail := strings.Join([]string{s.command, "--json"}, " ")
		return s.reportError(contract.Refuse(contract.Malformed, detail))
	}
	if tuiHead != nil {
		return tuiHead(s, parsed)
	}
	path, first, found := systemTUIFinder().find(tuiProgram())
	if !found {
		return s.reportError(contract.Refuse(contract.TUIMissing, first))
	}
	return s.launchTUI(path)
}

// tuiFinder looks for dinah-tui in the three places section 15.3 names, in
// order. Its functions are the documented calls a shipped binary uses, and a
// test replaces them.
type tuiFinder struct {
	// executable answers this program's own path, as os.Executable does.
	executable func() (string, error)
	// resolve answers a path once every link along it is resolved, as
	// bench.FinalPath does.
	resolve func(string) (string, error)
	// lookPath searches PATH, as exec.LookPath does.
	lookPath func(string) (string, error)
	// isProgram reports whether a path names a program that can be run.
	isProgram func(string) bool
}

// systemTUIFinder is the finder a shipped binary uses.
func systemTUIFinder() tuiFinder {
	return tuiFinder{
		executable: os.Executable,
		resolve:    bench.FinalPath,
		lookPath:   exec.LookPath,
		isProgram:  isRunnable,
	}
}

// find answers where the program named name is, and the directory of this
// program's own executable, which is where the search began and what a
// refusal names. It tries the directory of the executable as os.Executable
// answers it, then the directory of that path once every link along it is
// resolved, then PATH, where an answer found only through the current
// directory, which exec.LookPath reports as exec.ErrDot, counts as not found,
// as the os/exec documentation advises.
func (f tuiFinder) find(name string) (path, first string, found bool) {
	executable, err := f.executable()
	if err == nil {
		first = filepath.Dir(executable)
		if candidate := filepath.Join(first, name); f.isProgram(candidate) {
			return candidate, first, true
		}
		if resolved, err := f.resolve(executable); err == nil {
			if candidate := filepath.Join(filepath.Dir(resolved), name); f.isProgram(candidate) {
				return candidate, first, true
			}
		}
	}
	// An exec.ErrDot answer carries a path, and is refused with every other
	// error.
	onPath, err := f.lookPath(strings.TrimSuffix(name, ".exe"))
	if err != nil || !f.isProgram(onPath) {
		return "", first, false
	}
	return onPath, first, true
}

// isRunnable reports whether a path names a regular file, and outside
// Windows one with an execute bit set.
func isRunnable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}

// tuiEnviron is the environment dinah tui hands dinah-tui: its own, with any
// earlier DINAH_TUI_LAUNCHER removed and one naming this build added.
func tuiEnviron(environ []string, identity string) []string {
	kept := make([]string, 0, len(environ)+1)
	for _, variable := range environ {
		name, _, _ := strings.Cut(variable, "=")
		if strings.EqualFold(name, tuiLauncherVariable) {
			continue
		}
		kept = append(kept, variable)
	}
	return append(kept, tuiLauncherVariable+"="+identity)
}

// refuseTUISkew refuses, in the build tagged tui, a command line a dinah of
// another build handed on. It answers false where no launcher named a build
// or the launcher's build is this one.
func (s *session) refuseTUISkew() (int, bool) {
	launcher, launched := os.LookupEnv(tuiLauncherVariable)
	identity := verb.BuildIdentity()
	if !launched || launcher == identity {
		return 0, false
	}
	path, err := os.Executable()
	if err != nil {
		path = tuiProgram()
	}
	values := map[string]string{"launcher": launcher, "tui": identity}
	return s.reportError(contract.RefuseWith(contract.TUISkew, path, values)), true
}

// tuiArguments answers the command line dinah-tui runs. Started by a
// launcher, it is the launcher's own command line, which names tui. Started
// directly, the arguments are those of dinah tui, so tui is prepended, except
// where the first word is version, which dinah-tui answers as dinah version
// does so that a person and the release workflow can ask which build it is.
// A view named version is still opened with dinah tui version.
func tuiArguments(argv []string, launched bool) []string {
	if launched {
		return argv
	}
	valued := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
	}
	parsed, err := parseArgs(argv, valued)
	if err == nil && at(parsed.positional, 0) == "version" {
		return argv
	}
	return append([]string{"tui"}, argv...)
}
