package testenv

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// HarnessScriptVar names the environment variable that turns a test binary
// into a stand-in agent harness for `dinah run`. The variable names a JSON
// file, a HarnessScript, saying what the next launch does; a test rewrites
// the file between runs to script a fresh session and then a resumed one.
//
// The stand-in reads its prompt from standard input, appends one line to the
// script's log recording what it was handed, and prints a receipt shaped the
// way the run recipe in the test names it. It lives here for the reason the
// stand-in editor does: the launched child starts in a fixture directory, and
// this package's variable initialisers run before the test package's own.
const HarnessScriptVar = "DINAH_TEST_HARNESS"

// HarnessScript is what one launch of the stand-in harness does.
type HarnessScript struct {
	// Log is the file each launch appends one HarnessLaunch line to.
	Log string `json:"log"`
	// Session is the session identifier the receipt reports.
	Session string `json:"session"`
	// Text is the final text the receipt reports.
	Text string `json:"text"`
	// Cost is the cumulative cost the receipt reports.
	Cost float64 `json:"cost"`
	// Error is the receipt's error flag.
	Error bool `json:"error,omitempty"`
	// Exit is the status the stand-in exits with.
	Exit int `json:"exit,omitempty"`
	// Stderr is written to standard error before the receipt.
	Stderr string `json:"stderr,omitempty"`
	// SleepMS is how long the stand-in waits before it answers.
	SleepMS int `json:"sleep_ms,omitempty"`
	// Raw, where set, is printed in place of the receipt.
	Raw string `json:"raw,omitempty"`
}

// HarnessLaunch is one line of the stand-in's log.
type HarnessLaunch struct {
	Args  []string          `json:"args"`
	Stdin string            `json:"stdin"`
	Cwd   string            `json:"cwd"`
	Files map[string]string `json:"files,omitempty"`
	// Workbench is the DINAH_WORKBENCH the launch inherited.
	Workbench string `json:"workbench"`
}

var _ = standInHarness()

// standInHarness plays one launch of the stand-in harness and exits, or
// answers false in the ordinary case where the binary is running its suite.
func standInHarness() bool {
	path := os.Getenv(HarnessScriptVar)
	if path == "" {
		return false
	}
	fail := func(err error) {
		fmt.Fprintf(os.Stderr, "stand-in harness: %v\n", err)
		os.Exit(70)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	var script HarnessScript
	if err := json.Unmarshal(data, &script); err != nil {
		fail(err)
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(err)
	}
	cwd, _ := os.Getwd()
	launch := HarnessLaunch{Args: os.Args[1:], Stdin: string(stdin), Cwd: cwd, Workbench: os.Getenv("DINAH_WORKBENCH")}
	for _, arg := range os.Args[1:] {
		if body, err := os.ReadFile(arg); err == nil {
			if launch.Files == nil {
				launch.Files = map[string]string{}
			}
			launch.Files[arg] = string(body)
		}
	}
	line, err := json.Marshal(launch)
	if err != nil {
		fail(err)
	}
	if script.Log != "" {
		file, err := os.OpenFile(script.Log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fail(err)
		}
		if _, err := file.Write(append(line, '\n')); err != nil {
			fail(err)
		}
		file.Close()
	}
	if script.SleepMS > 0 {
		time.Sleep(time.Duration(script.SleepMS) * time.Millisecond)
	}
	if script.Stderr != "" {
		fmt.Fprintln(os.Stderr, script.Stderr)
	}
	if script.Raw != "" {
		fmt.Fprintln(os.Stdout, script.Raw)
	} else {
		receipt, err := json.Marshal(map[string]any{
			"type":           "result",
			"is_error":       script.Error,
			"result":         script.Text,
			"session_id":     script.Session,
			"total_cost_usd": script.Cost,
		})
		if err != nil {
			fail(err)
		}
		fmt.Fprintln(os.Stdout, `{"type":"system","note":"an event line before the receipt"}`)
		fmt.Fprintln(os.Stdout, string(receipt))
	}
	os.Exit(script.Exit)
	return true
}
