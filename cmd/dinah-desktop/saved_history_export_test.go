package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSavedHistoryExportKeepsFullEntries(t *testing.T) {
	dir := t.TempDir()
	large := "oldest saved text\n" + strings.Repeat("long tool result\n", 1<<16) + "```\nend of oldest"
	view := SavedConversationView{Available: true, Complete: false, Warning: "Earlier capture was incomplete.", Messages: []ConversationMessage{
		{Role: "tool", Provider: "old-provider", SessionID: "ancestor", Text: large},
		{Role: "provider_switch", Provider: "new-provider", SessionID: "child", Text: "middle boundary"},
		{Role: "agent", Provider: "new-provider", SessionID: "child", Text: "newest saved text"},
	}}
	at := time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)
	p, err := writeSavedHistoryExport(dir, "child", view, at)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{large, "middle boundary", "newest saved text", view.Warning, `"sessionId": "ancestor"`, `"role": "provider_switch"`, at.Format(time.RFC3339Nano), "This file does not update"} {
		if !strings.Contains(s, want) {
			t.Fatalf("export omitted expected content of length %d", len(want))
		}
	}
	if strings.Index(s, large) >= strings.Index(s, "middle boundary") || strings.Index(s, "middle boundary") >= strings.Index(s, "newest saved text") {
		t.Fatal("saved entry order changed")
	}
	if view.Messages[0].Text != large {
		t.Fatal("export mutated its source")
	}
	second, err := writeSavedHistoryExport(dir, "child", view, at)
	if err != nil || second == p {
		t.Fatalf("a later export replaced the first: %s %v", second, err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(files) != 2 {
		t.Fatalf("unexpected export files: %v", files)
	}
}

func TestSavedHistoryExportRedactsContexts(t *testing.T) {
	view := SavedConversationView{Available: true, Complete: true, Messages: []ConversationMessage{{Role: "agent", Text: "before dinah-applet-v1:AAAAAAAAAAAAAAAAAAAAAAAAAA.BBBBBBBBBBBBBBBBBBBBBBBBBB after"}}}
	var out strings.Builder
	if err := renderSavedHistory(&out, "chat", view, time.Now()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "dinah-applet-v1:") {
		t.Fatal("export exposed private applet authorization")
	}
}
