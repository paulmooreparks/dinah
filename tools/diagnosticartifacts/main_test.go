package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const debuggerFixture = `private banner C:\private\credential
   0  Id: 1234.5678 Suspend: 1 Teb: 00000000 Unfrozen
00 ntdll!NtWaitForSingleObject+0x14
01 node!uv_run+0x2a
02 node!bad(argument-secret)
03 privateaddon!PrivateEntry+0x4
04 node!Bad C:\private\credential
`

func TestMain(m *testing.M) {
	switch os.Getenv("DINAH_CDB_HELPER") {
	case "frames":
		fmt.Print(debuggerFixture)
		os.Exit(0)
	case "hang":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func writeFixture(t *testing.T, dir, name, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiagnosticUploadRejectsDumps(t *testing.T) {
	for _, name := range []string{"raw.dmp", "raw.DMP", "renamed.log", "binary.log"} {
		t.Run(name, func(t *testing.T) {
			source, parent := t.TempDir(), t.TempDir()
			data := "MDMP private-memory"
			if name == "binary.log" {
				data = "binary\x00memory"
			}
			writeFixture(t, source, name, data)
			if _, err := stage(source, parent); err == nil {
				t.Fatal("dump or binary input reached the uploaded set")
			}
		})
	}
	source, parent := t.TempDir(), t.TempDir()
	writeFixture(t, source, "output.log", "safe text\n")
	snapshot, err := stage(source, parent)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, source, "late.dmp", "MDMP late source write")
	if _, err := checkedFiles(snapshot); err != nil {
		t.Fatal("late source write changed the isolated snapshot:", err)
	}
	writeFixture(t, snapshot, "planted.dmp", "MDMP")
	if _, err := checkedFiles(snapshot); err == nil {
		t.Fatal("dump in the actual upload set was accepted")
	}
	if err := os.Remove(filepath.Join(source, "late.dmp")); err != nil {
		t.Fatal(err)
	}
	if _, err := stage(source, source); err == nil {
		t.Fatal("snapshot inside its source was accepted")
	}
}

func TestNativeExtractionPublishesOnlyFramesAndCleansDumps(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"frames", "missing", "hang"} {
		t.Run(mode, func(t *testing.T) {
			scratch, diagnostics := t.TempDir(), t.TempDir()
			dump := writeFixture(t, scratch, "raw.dmp", "MDMP private-memory")
			debugger := exe
			if mode == "missing" {
				debugger = ""
			}
			t.Setenv("DINAH_CDB_HELPER", mode)
			started := time.Now()
			err := extract(scratch, diagnostics, debugger, func(ctx context.Context, debugger, dump, work string) stackResult {
				if mode == "hang" {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 150*time.Millisecond)
					defer cancel()
				}
				return runCDB(ctx, debugger, dump, work)
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dump); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("raw dump survived extraction cleanup:", err)
			}
			data, err := os.ReadFile(filepath.Join(diagnostics, "native-stack-frames.json"))
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"credential", "argument-secret", "privateaddon", "private-memory"} {
				if strings.Contains(string(data), forbidden) {
					t.Fatalf("unsafe debugger content escaped: %s", data)
				}
			}
			var result extractionReport
			if err := json.Unmarshal(data, &result); err != nil || len(result.Captures) != 1 {
				t.Fatalf("missing extraction result: %v: %s", err, data)
			}
			capture := result.Captures[0]
			switch mode {
			case "frames":
				if len(capture.Frames) != 2 || capture.Frames[0].Function != "NtWaitForSingleObject" || capture.Frames[1].Module != "node" {
					t.Fatalf("wrong restricted frames: %s", data)
				}
			case "missing":
				if capture.Outcome != "debugger-unavailable" {
					t.Fatalf("missing debugger was not explicit: %s", data)
				}
			case "hang":
				if capture.Outcome != "debugger-deadline" || time.Since(started) > 3*time.Second {
					t.Fatalf("debugger was not bounded: %s", data)
				}
			}
		})
	}
	collector := &frameCollector{thread: -1}
	_, _ = collector.Write([]byte(strings.Repeat("private", 1000) + "\n" + debuggerFixture))
	if len(collector.pending) > 4096 || len(collector.frames) != 2 {
		t.Fatal("oversized debugger line was retained or broke subsequent frames")
	}
}
