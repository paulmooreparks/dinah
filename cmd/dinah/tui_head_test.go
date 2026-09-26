//go:build tui

package main

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// TestDinahTUIRefusesAnotherBuild is dinah-603/criteria/41. dinah-tui
// started with DINAH_TUI_LAUNCHER naming a build other than its own refuses
// dinah.tui-skew naming both identities, draws nothing and exits 2; started
// with its own identity it starts the interface, which draws the board and
// quits on q. The identity's own composition is held by
// TestTheBuildIdentityNamesTheReleaseAndTheRevision in internal/verb.
func TestDinahTUIRefusesAnotherBuild(t *testing.T) {
	own := verb.BuildIdentity()
	other := "0.0.0+0123456789ab"
	// The workbenches are made first, since every command this binary runs
	// makes the same check.
	root, again := tuiBench(t), tuiBench(t)
	t.Setenv(tuiLauncherVariable, other)
	refused := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("q"), 100, 30))
	if refused.code != 2 || refused.out != "" || refused.output != "" || refused.model != nil {
		t.Errorf("with another build's identity dinah-tui exited %d, wrote %q to standard output and %d bytes to the terminal", refused.code, refused.out, len(refused.output))
	}
	for _, want := range []string{"dinah.tui-skew", own, other} {
		if !strings.Contains(refused.errw, want) {
			t.Errorf("the refusal does not name %q: %q", want, refused.errw)
		}
	}
	t.Logf("the refusal: %s", strings.TrimSpace(refused.errw))

	t.Setenv(tuiLauncherVariable, own)
	started := runTUIThrough(t, again, tuiSeam(t, strings.NewReader("q"), 100, 30))
	if started.code != 0 || started.model == nil || !strings.Contains(visible(started.output), "Build the parser") {
		t.Errorf("with its own identity dinah-tui exited %d and drew %q", started.code, visible(started.output))
	}
}

// TestDinahTUIReadsItsArgumentsAsTui is dinah-603/criteria/42. Started by no
// launcher, dinah-tui reads dinah-tui, dinah-tui agenda and dinah-tui mine
// --plain as dinah tui, dinah tui agenda and dinah tui mine --plain, and
// answers dinah-tui version and dinah-tui --json version as dinah version
// does. Built programs are compared: each pair, run on a workbench with
// standard streams that are not a terminal, exits with the same code and
// writes the same bytes to both streams, except that the executable member
// of --json version names each program's own path.
func TestDinahTUIReadsItsArgumentsAsTui(t *testing.T) {
	cases := []struct {
		given, want []string
		launched    bool
	}{
		{nil, []string{"tui"}, false},
		{[]string{"agenda"}, []string{"tui", "agenda"}, false},
		{[]string{"mine", "--plain"}, []string{"tui", "mine", "--plain"}, false},
		{[]string{"version"}, []string{"version"}, false},
		{[]string{"--json", "version"}, []string{"--json", "version"}, false},
		{[]string{"tui", "mine"}, []string{"tui", "mine"}, true},
	}
	for _, c := range cases {
		if got := tuiArguments(c.given, c.launched); !slices.Equal(got, c.want) {
			t.Errorf("tuiArguments(%q, %v) is %q, wanted %q", c.given, c.launched, got, c.want)
		}
	}

	gobin := goOnPath(t)
	dir := t.TempDir()
	dinah := buildProgram(t, gobin, dir, "dinah", "")
	tui := buildProgram(t, gobin, dir, "dinah-tui", "tui")
	root := tuiBench(t)
	pairs := []struct{ direct, through []string }{
		{nil, []string{"tui"}},
		{[]string{"agenda"}, []string{"tui", "agenda"}},
		{[]string{"mine", "--plain"}, []string{"tui", "mine", "--plain"}},
		{[]string{"version"}, []string{"version"}},
		{[]string{"--json", "version"}, []string{"--json", "version"}},
	}
	for _, pair := range pairs {
		code, out, errw := runBuilt(t, tui, root, pair.direct)
		wantCode, wantOut, wantErrw := runBuilt(t, dinah, root, pair.through)
		out = strings.Replace(out, jsonPath(tui), jsonPath(dinah), 1)
		if code != wantCode || out != wantOut || errw != wantErrw {
			t.Errorf("dinah-tui %q answered %d, %q, %q, and dinah %q answered %d, %q, %q", pair.direct, code, out, errw, pair.through, wantCode, wantOut, wantErrw)
		}
		t.Logf("dinah-tui %q and dinah %q both answered %d: %s%s", pair.direct, pair.through, code, strings.TrimSpace(out), strings.TrimSpace(errw))
	}
}

// runBuilt runs a built program in dir with the arguments given, under
// launcherEnviron with the actor alka, and answers its exit code and both
// streams.
func runBuilt(t *testing.T, program, dir string, argv []string) (int, string, string) {
	t.Helper()
	run := exec.Command(program, argv...)
	run.Dir = dir
	run.Env = launcherEnviron(t, map[string]string{"DINAH_ACTOR": "alka"})
	var out, errw strings.Builder
	run.Stdout, run.Stderr = &out, &errw
	code := exitCodeOf(t, run.Run())
	return code, out.String(), errw.String()
}
