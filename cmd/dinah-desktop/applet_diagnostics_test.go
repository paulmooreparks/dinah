package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"studio/internal/applet"
)

func diagnosticContext() context.Context {
	return applet.WithInvocation(context.Background(), applet.Invocation{ID: "capture-test", Workspace: "C:/Fixture", OwnerKind: "chat", OwnerID: "diagnostics-test"})
}

const diagnosticPageFixture = `{"anchor":{"utc":"2026-11-06T00:00:00Z","monoMs":1},"capture":{"id":"","state":"idle","startedMonoMs":0,"deadlineUtcMs":0},"visible":true,"features":{"input":"unavailable","longTask":"unavailable","longFrame":"unavailable"},"coverage":{"retained":0,"overwritten":0,"rateOmissions":0,"scriptOmissions":0},"counts":{"elements":1,"windows":0,"transcriptRows":0,"toolRows":0},"scheduler":{"pending":0,"blocked":0,"watchedRoots":0,"framePending":false,"oldestWaitMs":0},"rendererHeap":"requires_profiler","events":[],"collectionMs":0}`

func TestDiagnosticsAppletSnapshotAndContract(t *testing.T) {
	a, _ := providerTestApp(t)
	r, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	a.sink = func(name string, data any) {
		if name != "page:ask" {
			return
		}
		ask := data.(map[string]any)
		args := ask["args"].(map[string]any)
		if ask["op"] != "diagnostics" || args["operation"] != "snapshot" {
			t.Errorf("unexpected operation: %v", ask)
		}
		if expires := args["expiresAt"].(int64); expires <= time.Now().UnixMilli() || expires > time.Now().Add(3*time.Second).UnixMilli() {
			t.Error("invalid expiry")
		}
		go a.AnswerPage(ask["id"].(string), diagnosticPageFixture, "")
	}
	out, err := r.Execute(diagnosticContext(), "dinah.diagnostics/snapshot@1", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.OutputJSON, `"state":"measured"`) || !strings.Contains(out.OutputJSON, `"heapObjectsBytes":`) {
		t.Fatal(out.OutputJSON)
	}
	if len(a.PendingPageAsks()) != 0 {
		t.Fatal("completed request retained")
	}
	for _, input := range []string{`{"seconds":0}`, `{"seconds":301}`, `{"seconds":1,"script":"secret"}`} {
		if _, err := r.Execute(diagnosticContext(), "dinah.diagnostics/start@1", input); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	if !applet.SupportedCapability("diagnostics.read") || applet.Effectful("diagnostics.read") || !applet.Effectful("diagnostics.control") {
		t.Fatal("incorrect diagnostics consent boundary")
	}
}

func TestDiagnosticsBusyCancellationAndUnavailable(t *testing.T) {
	a, _ := providerTestApp(t)
	a.diagnosticsMu.Lock()
	out, err := a.executeDiagnostics(diagnosticContext(), "dinah.diagnostics/snapshot@1", `{}`)
	a.diagnosticsMu.Unlock()
	if err != nil || !strings.Contains(out.OutputJSON, `"state":"busy"`) {
		t.Fatalf("busy = %s, %v", out.OutputJSON, err)
	}
	ctx, cancel := context.WithCancel(diagnosticContext())
	a.sink = func(name string, _ any) {
		if name == "page:ask" {
			cancel()
		}
	}
	if _, err := a.executeDiagnostics(ctx, "dinah.diagnostics/snapshot@1", `{}`); err == nil {
		t.Fatal("cancellation not returned")
	}
	if len(a.PendingPageAsks()) != 0 {
		t.Fatal("cancelled request retained")
	}
	a.diagnosticsNext = time.Time{}
	a.sink = func(string, any) {}
	out, err = a.executeDiagnostics(diagnosticContext(), "dinah.diagnostics/snapshot@1", `{}`)
	if err != nil || !strings.Contains(out.OutputJSON, `"state":"unavailable"`) || !strings.Contains(out.OutputJSON, `"goroutines":`) {
		t.Fatalf("unavailable = %s, %v", out.OutputJSON, err)
	}
	if len(a.PendingPageAsks()) != 0 {
		t.Fatal("timed-out request retained")
	}
}

func TestDiagnosticsRejectsUnexpectedPageData(t *testing.T) {
	a, _ := providerTestApp(t)
	for _, answer := range []string{`{"privateContent":"secret"}`, strings.Repeat("x", diagnosticsPageLimit+1)} {
		a.diagnosticsNext = time.Time{}
		a.sink = func(name string, data any) {
			if name == "page:ask" {
				go a.AnswerPage(data.(map[string]any)["id"].(string), answer, "")
			}
		}
		out, err := a.executeDiagnostics(diagnosticContext(), "dinah.diagnostics/snapshot@1", `{}`)
		if err == nil || strings.Contains(out.OutputJSON, "secret") {
			t.Fatalf("invalid response accepted: %v", err)
		}
	}
	data, err := json.Marshal(diagnosticsRuntime())
	if err != nil || strings.Contains(string(data), "Fixture") {
		t.Fatal("runtime counters contain context")
	}
}

func TestDiagnosticsBrowserContract(t *testing.T) {
	// Captured from the isolated real-browser 90 ms stall reproduction.
	data, err := os.ReadFile("testdata/diagnostics-page.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := diagnosticsPageSchema()
	if err != nil {
		t.Fatal(err)
	}
	if err = applet.Validate(schema, string(data)); err != nil {
		t.Fatal(err)
	}
	a, _ := providerTestApp(t)
	r, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	a.sink = func(name string, payload any) {
		if name == "page:ask" {
			go a.AnswerPage(payload.(map[string]any)["id"].(string), string(data), "")
		}
	}
	if _, err = r.Execute(diagnosticContext(), "dinah.diagnostics/snapshot@1", `{}`); err != nil {
		t.Fatal(err)
	}
	// Rate limiting must not enqueue work in the page.
	a.sink = func(string, any) { t.Error("rate-limited snapshot reached the page") }
	out, err := r.Execute(diagnosticContext(), "dinah.diagnostics/snapshot@1", `{}`)
	if err != nil || !strings.Contains(out.OutputJSON, `"state":"busy"`) {
		t.Fatalf("rate limit: %s, %v", out.OutputJSON, err)
	}
	if _, err = a.executeDiagnostics(context.Background(), "dinah.diagnostics/snapshot@1", `{}`); err == nil {
		t.Fatal("unauthenticated handler accepted")
	}
}
