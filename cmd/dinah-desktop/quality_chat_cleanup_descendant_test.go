//go:build quality_chat_cleanup && chatcleanup_acceptance

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"studio/internal/acp"
	"studio/internal/codexagent"
	"testing"
	"time"
)

func TestQualityCodexDeletionPeer(t *testing.T) {
	path := os.Getenv("QUALITY_CLEANUP_DELETE_LOG")
	if path == "" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var q struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &q) != nil {
			os.Exit(2)
		}
		var result any = map[string]any{}
		switch q.Method {
		case "thread/list":
			result = map[string]any{"data": []any{map[string]string{"id": "quality-root"}, map[string]string{"id": "quality-child", "parentThreadId": "quality-root"}}}
		case "thread/delete":
			file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				os.Exit(3)
			}
			file.WriteString(string(q.Params) + "\n")
			file.Close()
		default:
			os.Exit(4)
		}
		if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"id": q.ID, "result": result}); err != nil {
			os.Exit(5)
		}
	}
	os.Exit(0)
}

func TestQualityCleanupIndependentDescendant(t *testing.T) {
	for _, kind := range []string{"separate-recipient", "damaged-transcript-only", "owned-control"} {
		t.Run(kind, func(t *testing.T) {
			f := qualityCleanupNew(t).(*qualityHostCleanup)
			root, child := "codex:quality-root", "codex:quality-child"
			f.Seed(qualityCleanupSeed{ID: root, ProviderDelete: "supported"})
			switch kind {
			case "separate-recipient":
				f.Seed(qualityCleanupSeed{ID: child, Queued: 1})
			case "damaged-transcript-only":
				if err := os.MkdirAll(filepath.Dir(transcriptPath(child)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(transcriptPath(child), []byte("damaged retained identity"), 0600); err != nil {
					t.Fatal(err)
				}
			case "owned-control":
				f.Seed(qualityCleanupSeed{ID: child, Chain: []string{root, child}, ProviderDelete: "supported"})
			}
			original, err := os.ReadFile(transcriptPath(child))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "native-deletes.txt")
			t.Setenv("QUALITY_CLEANUP_DELETE_LOG", path)
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			c, err := codexagent.Start(exe, []string{"-test.run=^TestQualityCodexDeletionPeer$"}, f.ws, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { c.Close(); <-c.Done() })
			c.SetDeletionGuard(f.a.codexDeletionGuard)
			ctx, cancel := context.WithTimeout(context.Background(), testWait)
			defer cancel()
			if kind == "owned-control" {
				if err := c.Call(ctx, "session/delete", acp.DeleteSessionParams{SessionID: root}, nil); err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(path)
				if err != nil || len(strings.Split(strings.TrimSpace(string(raw)), "\n")) != 1 {
					t.Fatal("native deletion positive control failed", err)
				}
				return
			}
			f.a.chatCleanupAcceptanceInstall(chatCleanupAcceptanceHooks{Now: func() time.Time { return f.now }, DeleteProvider: func(_ context.Context, _ string, id string) (bool, error) {
				return true, c.Call(ctx, "session/delete", acp.DeleteSessionParams{SessionID: id}, nil)
			}})
			qualityCleanupResultFor(t, f.Act("remove", root), root, "removed")
			r := qualityCleanupResultFor(t, f.Act("delete", root), root, "deleted")
			if !r.ProviderCopyRemains || !strings.Contains(r.ProviderDeletionReason, "independently owned") {
				t.Fatalf("missing native refusal outcome: %+v", r)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("refused cascade sent a native delete", err)
			}
			after, err := os.ReadFile(transcriptPath(child))
			if err != nil || string(after) != string(original) {
				t.Fatal("independent child changed", err)
			}
		})
	}
}
