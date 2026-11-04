package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const privateValue = "private-preflight-fixture-value"

func TestMain(m *testing.M) {
	switch os.Getenv("DINAH_PREFLIGHT_HELPER") {
	case "success":
		fmt.Println("v24.4.1")
		os.Exit(0)
	case "error":
		fmt.Fprintln(os.Stderr, privateValue)
		os.Exit(17)
	case "mixed":
		fmt.Println(privateValue)
		fmt.Println("v24.4.1")
		os.Exit(0)
	case "flood":
		fmt.Print(strings.Repeat(privateValue, outputLimit))
		os.Exit(0)
	case "hang":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNodePreflightIsBoundedAndRedactsValues(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "error", "mixed", "flood", "hang", "cancelled", "missing"} {
		t.Run(mode, func(t *testing.T) {
			environment := append(os.Environ(), "DINAH_PREFLIGHT_HELPER="+mode, "NODE_OPTIONS="+privateValue, "NODE_FIXTURE_SECRET="+privateValue)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			path, timeout := executable, 5*time.Second
			if mode == "hang" {
				timeout = 150 * time.Millisecond
			}
			if mode == "cancelled" {
				cancel()
			}
			if mode == "missing" {
				path = filepath.Join(t.TempDir(), "missing-node")
			}
			r := probe(ctx, path, environment, timeout)
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), privateValue) || !strings.Contains(string(data), "NODE_FIXTURE_SECRET") {
				t.Fatalf("environment/output redaction failed: %s", data)
			}
			switch mode {
			case "success", "mixed":
				if r.VersionOutput != "v24.4.1" || r.ExitCode == nil || *r.ExitCode != 0 || r.ContextResult != "none" || r.OtherOutputOmitted != (mode == "mixed") {
					t.Fatalf("wrong successful probe: %s", data)
				}
			case "error":
				if r.ExitCode == nil || *r.ExitCode != 17 || r.VersionOutput != "" || !r.OtherOutputOmitted || r.ContextResult != "none" {
					t.Fatalf("wrong failed probe: %s", data)
				}
			case "flood":
				if !r.OutputTruncated || r.OutputBytes <= outputLimit || !r.OtherOutputOmitted {
					t.Fatalf("output was not bounded: %s", data)
				}
			case "hang":
				if r.Outcome != "timed-out" || r.ContextResult != "deadline-exceeded" || r.DurationMillis > 3000 || r.CancellationOutcome != "process-exited" {
					t.Fatalf("timeout was not explicit/bounded: %s", data)
				}
			case "cancelled":
				if r.ContextResult != "cancelled" || r.Outcome != "cancelled" {
					t.Fatalf("cancellation was not distinguished: %s", data)
				}
			case "missing":
				if r.Outcome != "executable-lookup-failed" || r.ExitCode != nil {
					t.Fatalf("lookup failure was not distinguished: %s", data)
				}
			}
		})
	}
}
