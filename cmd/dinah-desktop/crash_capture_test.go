package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

const testDiagnosticsDir = "DINAH_TEST_DIAGNOSTICS_DIR"

// This setup must precede ReleaseStandardHandles. SetCrashOutput duplicates
// the file descriptor, so closing our file does not disable crash reporting.
// https://pkg.go.dev/runtime/debug#SetCrashOutput
func captureTestCrashes() error {
	dir := os.Getenv(testDiagnosticsDir)
	if dir == "" {
		return nil
	}
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("%s must name an absolute directory", testDiagnosticsDir)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create test diagnostics: %w", err)
	}
	file, err := os.CreateTemp(dir, fmt.Sprintf("desktop-crash-%d-*.log", os.Getpid()))
	if err != nil {
		return fmt.Errorf("create test crash output: %w", err)
	}
	defer file.Close()
	if err := debug.SetCrashOutput(file, debug.CrashOptions{}); err != nil {
		return fmt.Errorf("enable test crash output: %w", err)
	}
	return nil
}

func TestCrashOutputSurvivesHandleRelease(t *testing.T) {
	const child = "DINAH_TEST_CRASH_CHILD"
	const marker = "dinah-crash-capture-regression"
	if os.Getenv(child) == "1" {
		panic(marker)
	}
	dir := t.TempDir()
	if parent := os.Getenv(testDiagnosticsDir); parent != "" {
		var err error
		dir, err = os.MkdirTemp(parent, "crash-guard-")
		if err != nil {
			t.Fatal(err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, exe, "-test.run=^TestCrashOutputSurvivesHandleRelease$", "-test.count=1")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, testDiagnosticsDir+"=") && !strings.HasPrefix(entry, child+"=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, testDiagnosticsDir+"="+dir, child+"=1")
	output, err := command.CombinedOutput()
	if err == nil || ctx.Err() != nil {
		t.Fatalf("intentional crash did not finish as a failed process: %v %v %s", err, ctx.Err(), output)
	}
	files, err := filepath.Glob(filepath.Join(dir, "desktop-crash-*.log"))
	if err != nil || len(files) != 1 {
		t.Fatalf("unique child crash file: %v %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "panic: "+marker) || !strings.Contains(string(data), "TestCrashOutputSurvivesHandleRelease") || !strings.Contains(string(data), "goroutine ") {
		t.Fatalf("crash marker or stack missing: %s", data)
	}
}
