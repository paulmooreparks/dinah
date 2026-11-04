// nodepreflight records ordinary-handle startup evidence without changing the
// result of the product tests that follow it.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const outputLimit = 4096

type report struct {
	Executable          string   `json:"executable"`
	Arguments           []string `json:"arguments"`
	EnvironmentNames    []string `json:"environmentNames"`
	VersionOutput       string   `json:"versionOutput"`
	OtherOutputOmitted  bool     `json:"otherOutputOmitted"`
	OutputBytes         int64    `json:"outputBytes"`
	OutputTruncated     bool     `json:"outputTruncated"`
	ExitCode            *int     `json:"exitCode"`
	DurationMillis      int64    `json:"durationMillis"`
	TimeoutMillis       int64    `json:"timeoutMillis"`
	Outcome             string   `json:"outcome"`
	ContextResult       string   `json:"contextResult"`
	CancellationOutcome string   `json:"cancellationOutcome"`
}

// Separate stdout/stderr copying goroutines may write concurrently. The cap
// limits retained memory while allowing both streams to drain until WaitDelay.
type boundedOutput struct {
	sync.Mutex
	data  []byte
	total int64
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	b.total += int64(len(p))
	n := min(len(p), outputLimit-len(b.data))
	b.data = append(b.data, p[:n]...)
	return len(p), nil
}

var versionLine = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func probe(parent context.Context, executable string, environment []string, timeout time.Duration) report {
	r := report{
		Arguments: []string{"--version"}, EnvironmentNames: []string{},
		TimeoutMillis: timeout.Milliseconds(), ContextResult: "none",
		CancellationOutcome: "not-requested",
	}
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "NODE_") {
			r.EnvironmentNames = append(r.EnvironmentNames, name)
		}
	}
	sort.Strings(r.EnvironmentNames)
	path, err := exec.LookPath(executable)
	if err != nil {
		r.Outcome = "executable-lookup-failed"
		return r
	}
	path, err = filepath.Abs(path)
	if err != nil {
		r.Outcome = "executable-path-failed"
		return r
	}
	r.Executable = path
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, r.Arguments...)
	command.Env = environment
	command.WaitDelay = time.Second
	output := &boundedOutput{}
	command.Stdout, command.Stderr = output, output
	started := time.Now()
	err = command.Run()
	r.DurationMillis = time.Since(started).Milliseconds()
	contextErr := ctx.Err()
	if command.ProcessState != nil {
		code := command.ProcessState.ExitCode()
		r.ExitCode = &code
	}
	r.Outcome = "exited"
	if err != nil && r.ExitCode == nil {
		r.Outcome = "process-start-failed"
	} else if errors.Is(err, exec.ErrWaitDelay) {
		r.Outcome = "output-drain-limit"
	}
	if contextErr != nil {
		r.ContextResult = "cancelled"
		r.Outcome = "cancelled"
		if errors.Is(contextErr, context.DeadlineExceeded) {
			r.ContextResult, r.Outcome = "deadline-exceeded", "timed-out"
		}
		r.CancellationOutcome = "wait-returned-without-process-state"
		if r.ExitCode != nil {
			r.CancellationOutcome = "process-exited"
		}
		if err == nil {
			// The context can expire just as an ordinary exit is observed.
			// Retain both observations without relabelling success as a timeout.
			r.Outcome, r.CancellationOutcome = "exited", "ordinary-exit-observed"
		}
	}
	output.Lock()
	defer output.Unlock()
	r.OutputBytes = output.total
	r.OutputTruncated = output.total > int64(len(output.data))
	for _, line := range strings.Split(string(output.data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if r.VersionOutput == "" && versionLine.MatchString(line) {
			r.VersionOutput = line
		} else {
			r.OtherOutputOmitted = true
		}
	}
	return r
}

func main() {
	out := flag.String("out", "", "diagnostic JSON file")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "nodepreflight requires -out")
		os.Exit(2)
	}
	r := probe(context.Background(), "node", os.Environ(), 5*time.Second)
	data, err := json.MarshalIndent(r, "", "  ")
	if err == nil {
		err = os.WriteFile(*out, append(data, '\n'), 0600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nodepreflight could not retain its diagnostic:", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
	// A failed or timed-out probe is evidence. The unchanged full Go suite
	// remains responsible for deciding whether the product tests pass.
}
