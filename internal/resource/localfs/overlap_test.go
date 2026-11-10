package localfs

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/resource"
	"studio/internal/savelock"
)

// overlap starts one save, holds it between its last check and its rename,
// starts a second save of the same file from the same revision, waits to see
// whether the second finishes while the first is held, and then lets the
// first go on. It returns each save's error, whether the second finished
// while the first was held, and the file's text at the end.
func overlap(t *testing.T) (first, second error, overtook bool, text string) {
	t.Helper()
	t.Setenv(savelock.TestDirEnv, t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := Folder{Root: dir}
	ctx := context.Background()
	base := resource.ContentRevision("base\n")

	reached, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	betweenCheckAndReplace = func(string) {
		held := false
		once.Do(func() { held = true })
		if held {
			close(reached)
			<-release
		}
	}
	t.Cleanup(func() { betweenCheckAndReplace = nil })

	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	go func() { _, err := f.WriteText(ctx, "notes.md", base, "first\n"); firstDone <- err }()
	<-reached
	go func() { _, err := f.WriteText(ctx, "notes.md", base, "second\n"); secondDone <- err }()
	select {
	case second = <-secondDone:
		overtook = true
	case <-time.After(400 * time.Millisecond):
	}
	close(release)
	first = <-firstDone
	if !overtook {
		second = <-secondDone
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return first, second, overtook, string(data)
}

// Two saves of one file, from the same revision, that overlap in time: the
// second waits for the first, then finds the file changed and is refused, so
// the first one's text is not replaced unseen. A test that saves one after
// the other cannot show this (PUDL Team Lead's review, 11 November 2026).
func TestOverlappingSavesDoNotBothSucceed(t *testing.T) {
	first, second, overtook, text := overlap(t)
	if overtook {
		t.Fatalf("the second save finished while the first was between its check and its rename: %v", second)
	}
	if first != nil || !errors.Is(second, resource.ErrConflict) {
		t.Fatalf("first %v, second %v; the second should be a conflict", first, second)
	}
	if text != "first\n" {
		t.Fatalf("the file holds %q", text)
	}
}

// Without the lock both saves pass their checks and the first one's text is
// replaced with no conflict, which is the fault the lock is for. The test
// keeps the fault on record, so that the one above cannot pass by accident.
func TestWithoutTheLockOverlappingSavesBothSucceed(t *testing.T) {
	holdSaveLock = func(context.Context, time.Duration) (func(), error) { return func() {}, nil }
	t.Cleanup(func() { holdSaveLock = savelockHold })
	first, second, overtook, text := overlap(t)
	if !overtook || first != nil || second != nil {
		t.Fatalf("overtook %v, first %v, second %v", overtook, first, second)
	}
	if text != "first\n" {
		t.Fatalf("the file holds %q: the first save, which finished last, replaced the second's text unseen", text)
	}
}

// The same contention between two processes, which is the case the lock is
// for: another process is held between its check and its rename, this one
// saves the same file from the same revision, and this one is refused once
// the other has written.
func TestOverlappingSavesFromTwoProcesses(t *testing.T) {
	ctx := context.Background()
	base := resource.ContentRevision("base\n")
	if dir := os.Getenv("LOCALFS_TEST_OVERLAP"); dir != "" {
		betweenCheckAndReplace = func(string) {
			os.Stdout.WriteString("checked\n")
			buf := make([]byte, 1)
			os.Stdin.Read(buf)
		}
		_, err := Folder{Root: dir}.WriteText(ctx, "notes.md", base, "other process\n")
		if err != nil {
			os.Stdout.WriteString("failed " + err.Error() + "\n")
		} else {
			os.Stdout.WriteString("written\n")
		}
		return
	}
	t.Setenv(savelock.TestDirEnv, t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestOverlappingSavesFromTwoProcesses$")
	cmd.Env = append(os.Environ(), "LOCALFS_TEST_OVERLAP="+dir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	lines := bufio.NewReader(stdout)
	if line, err := lines.ReadString('\n'); err != nil || strings.TrimSpace(line) != "checked" {
		t.Fatalf("the other process said %q %v", line, err)
	}
	mine := make(chan error, 1)
	go func() { _, err := Folder{Root: dir}.WriteText(ctx, "notes.md", base, "this process\n"); mine <- err }()
	select {
	case err := <-mine:
		t.Fatalf("this process saved while the other was between its check and its rename: %v", err)
	case <-time.After(400 * time.Millisecond):
	}
	stdin.Write([]byte("\n"))
	if line, err := lines.ReadString('\n'); err != nil || strings.TrimSpace(line) != "written" {
		t.Fatalf("the other process said %q %v", line, err)
	}
	if err := <-mine; !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("this process's save: %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "other process\n" {
		t.Fatalf("the file holds %q", data)
	}
}
