package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
)

type logCanaryHandler struct{ seen chan struct{} }

func (h logCanaryHandler) Notify(string, json.RawMessage) { h.seen <- struct{}{} }
func (h logCanaryHandler) Request(context.Context, string, json.RawMessage) (any, *acp.Error) {
	return nil, nil
}

type logCanaryWriter struct{ io.Writer }

func (logCanaryWriter) Close() error { return nil }

func TestMalformedProtocolCannotPersistCapabilities(t *testing.T) {
	old := acp.Logf
	defer func() { acp.Logf = old; setLogLevel(LogProblems) }()
	acp.Logf = func(format string, args ...any) { logf("agent", "info", format, args...) }
	for _, level := range []string{LogOff, LogProblems, LogAll, LogDebug} {
		for _, headers := range []bool{false, true} {
			name := level + "/full"
			if headers {
				name = level + "/headers"
			}
			t.Run(name, func(t *testing.T) {
				isolateWorkspaceSettings(t)
				setLogLevel(level)
				secret, replayed := "synthetic-live-capability", "synthetic-replayed-capability"
				tracePath := filepath.Join(t.TempDir(), "protocol.log")
				trace, err := os.Create(tracePath)
				if err != nil {
					t.Fatal(err)
				}
				defer trace.Close()
				r, w := io.Pipe()
				defer r.Close()
				defer w.Close()
				seen := make(chan struct{}, 1)
				// Agent metadata also passes through the diagnostic sink. Including both
				// canaries here tests sink redaction separately from payload omission.
				c := acp.Connect(r, logCanaryWriter{io.Discard}, logCanaryHandler{seen}, acp.Options{Agent: secret + " " + replayed, Trace: trace, TraceHeadersOnly: headers})
				c.RegisterTraceSecret(secret)
				c.RegisterTraceRedactor(func(s string) string { return strings.ReplaceAll(s, replayed, "[redacted]") })
				if _, err := io.WriteString(w, "malformed "+secret+" "+replayed+"\n{\"method\":\"session/update\",\"params\":{}}\n"); err != nil {
					t.Fatal(err)
				}
				select {
				case <-seen:
				case <-time.After(3 * time.Second):
					t.Fatal("connection stopped serving notifications")
				}
				w.Close()
				select {
				case <-c.Done():
				case <-time.After(3 * time.Second):
					t.Fatal("reader did not end")
				}
				flushAppLog()
				trace.Close()
				data, err := os.ReadFile(tracePath)
				if err != nil {
					t.Fatal(err)
				}
				app := readLog(t)
				for _, canary := range []string{secret, replayed} {
					if strings.Contains(app, canary) || strings.Contains(string(data), canary) {
						t.Fatal("capability reached persisted diagnostic")
					}
				}
				if (level == LogAll || level == LogDebug) && !strings.Contains(app, "[redacted] [redacted] wrote a line that is not protocol") {
					t.Fatalf("generic diagnostic missing: %q", app)
				}
				if headers && strings.Contains(string(data), "malformed") {
					t.Fatal("headers-only trace included malformed payload")
				}
				if !headers && !strings.Contains(string(data), "malformed [redacted] [redacted]") {
					t.Fatal("full trace lost its redacted diagnostic")
				}
			})
		}
	}
}
