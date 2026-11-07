package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"runtime/metrics"
	"time"

	"studio/internal/applet"
)

//go:embed applet_diagnostics.go diagnostics.schema.json frontend/agent-diagnostics.js frontend/render-scheduler.js frontend/workbench.js
var diagnosticsSources embed.FS

const diagnosticsPageLimit = 1024 * 1024

func (a *App) diagnosticsAppletBundle() (applet.Bundle, error) {
	output, err := diagnosticsSources.ReadFile("diagnostics.schema.json")
	if err != nil {
		return applet.Bundle{}, err
	}
	bundle := applet.Bundle{
		Manifest: applet.Manifest{ID: "dinah.diagnostics", Version: "1.0.0", APIVersion: 1, Title: "Diagnostics",
			Description: "Bounded process-wide desktop telemetry. Counts and timing attribution omit document contents, identities, commands and workspace paths. Pair captures with the Windows applet's scoped commands to reproduce operations.",
			Actions: []applet.Action{
				{Ref: "dinah.diagnostics/snapshot@1", Title: "Read Desktop Diagnostics", Replay: "pure", Capabilities: []string{"diagnostics.read"},
					Description: "Read Go runtime counters and a live page snapshot including DOM counts, render queue and retained capture events. Page busy or unavailable is explicit. Go memory excludes WebView memory; renderer heap requires a profiler. Do not poll faster than once per second.",
					InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), OutputSchema: output},
				{Ref: "dinah.diagnostics/start@1", Title: "Start Desktop Capture", Replay: "once", Capabilities: []string{"diagnostics.control"},
					Description: "Start a bounded process-wide timing capture for 1 to 300 seconds. Refuses overlapping captures. Clears previous capture evidence but does not change the person's responsiveness preference. Retains at most 200 events with overflow counts. Save the returned capture ID for Stop.",
					InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["seconds"],"properties":{"seconds":{"type":"integer","minimum":1,"maximum":300}}}`), OutputSchema: output},
				{Ref: "dinah.diagnostics/stop@1", Title: "Stop Desktop Capture", Replay: "once", Capabilities: []string{"diagnostics.control"},
					Description: "Stop the identified capture, retaining its evidence. A stale capture ID cannot stop a newer recording. Inspect page state in the result; an unavailable result does not confirm that the operation ran.",
					InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["captureId"],"properties":{"captureId":{"type":"string","pattern":"^[a-zA-Z0-9-]{1,128}$"}}}`), OutputSchema: output},
			}}, Handlers: map[string]applet.Handler{}, Sources: map[string][]byte{},
	}
	for _, action := range bundle.Manifest.Actions {
		ref := action.Ref
		bundle.Handlers[ref] = func(ctx context.Context, input string) (applet.Result, error) {
			return a.executeDiagnostics(ctx, ref, input)
		}
	}
	err = fs.WalkDir(diagnosticsSources, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := diagnosticsSources.ReadFile(path)
		bundle.Sources[path] = data
		return err
	})
	return bundle, err
}

// Read only documented runtime counters. These describe the Go process, not
// the renderer's heap or the cause of a particular input delay.
func diagnosticsRuntime() map[string]any {
	names := []struct{ key, metric string }{
		{"heapObjectsBytes", "/memory/classes/heap/objects:bytes"},
		{"heapObjects", "/gc/heap/objects:objects"},
		{"heapUnusedBytes", "/memory/classes/heap/unused:bytes"},
		{"heapReleasedBytes", "/memory/classes/heap/released:bytes"},
		{"memoryTotalBytes", "/memory/classes/total:bytes"},
		{"goroutines", "/sched/goroutines:goroutines"},
		{"gcCycles", "/gc/cycles/total:gc-cycles"},
		{"gcCPUSeconds", "/cpu/classes/gc/total:cpu-seconds"},
		{"mutexWaitSeconds", "/sync/mutex/wait/total:seconds"},
	}
	samples := make([]metrics.Sample, len(names))
	for i, name := range names {
		samples[i].Name = name.metric
	}
	metrics.Read(samples)
	out := make(map[string]any, len(names))
	for i, sample := range samples {
		out[names[i].key] = nil
		switch sample.Value.Kind() {
		case metrics.KindUint64:
			out[names[i].key] = sample.Value.Uint64()
		case metrics.KindFloat64:
			out[names[i].key] = sample.Value.Float64()
		}
	}
	return out
}

func (a *App) executeDiagnostics(ctx context.Context, ref, input string) (applet.Result, error) {
	v, ok := applet.InvocationFromContext(ctx)
	if !ok {
		return applet.Result{}, errors.New("a host invocation is required")
	}
	if err := ctx.Err(); err != nil {
		return applet.Result{}, err
	}
	args := map[string]any{}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return applet.Result{}, err
	}
	switch ref {
	case "dinah.diagnostics/snapshot@1":
		args["operation"] = "snapshot"
	case "dinah.diagnostics/start@1":
		args["operation"], args["captureId"] = "start", v.ID
	case "dinah.diagnostics/stop@1":
		args["operation"] = "stop"
	default:
		return applet.Result{}, errors.New("unknown diagnostic action")
	}
	page := map[string]any{"state": "busy", "roundTripMs": float64(0), "data": nil}
	result := func() (applet.Result, error) {
		out, err := json.Marshal(map[string]any{"schemaVersion": 1, "utc": time.Now().UTC().Format(time.RFC3339Nano), "build": appVersion, "scope": "process_wide", "backend": diagnosticsRuntime(), "page": page})
		return applet.Result{OutputJSON: string(out)}, err
	}
	if a.diagnosticsMu.TryLock() {
		defer a.diagnosticsMu.Unlock()
		if ref == "dinah.diagnostics/snapshot@1" && time.Now().Before(a.diagnosticsNext) {
			return result()
		}
		a.diagnosticsNext = time.Now().Add(time.Second)
		requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		deadline, _ := requestCtx.Deadline()
		args["expiresAt"] = deadline.UnixMilli()
		start := time.Now()
		data, err := a.askPage(requestCtx, "diagnostics", args)
		page["roundTripMs"] = float64(time.Since(start)) / float64(time.Millisecond)
		page["state"] = "unavailable"
		if ctx.Err() != nil {
			return applet.Result{}, ctx.Err()
		}
		if err == nil {
			if len(data) > diagnosticsPageLimit {
				return applet.Result{}, errors.New("diagnostic page response exceeded its size limit")
			}
			schema, err := diagnosticsPageSchema()
			if err != nil {
				return applet.Result{}, err
			}
			if err := applet.Validate(schema, data); err != nil {
				return applet.Result{}, errors.New("invalid diagnostic page response")
			}
			page["state"], page["data"] = "measured", json.RawMessage(data)
		} else if requestCtx.Err() == nil {
			// Only our fixed page diagnostic handlers can produce this error.
			return applet.Result{}, err
		}
	}
	return result()
}

// Use the same page definition for the bridge boundary and the MCP result.
func diagnosticsPageSchema() (json.RawMessage, error) {
	data, err := diagnosticsSources.ReadFile("diagnostics.schema.json")
	if err != nil {
		return nil, err
	}
	var document map[string]json.RawMessage
	if err = json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"$defs": document["$defs"], "$ref": "#/$defs/page"})
}
