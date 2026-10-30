package codexagent

import (
	"studio/internal/acp"

	"encoding/json"
	"strings"
	"testing"
)

func TestMCPResultsExposePublicContentOnly(t *testing.T) {
	b := json.RawMessage(`{"type":"mcpToolCall","arguments":{"credential":"argument-secret"},"result":{"content":[{"type":"text","text":"Invocation done"},{"type":"image","data":"aW1hZ2U=","mimeType":"image/png"}],"structuredContent":{"invocationId":"inv-123","status":"done"},"_meta":{"credential":"private-secret"}}}`)
	content := toolResultContent(b)
	if len(content) != 3 {
		t.Fatalf("content count %d", len(content))
	}
	encoded := string(acp.Raw(content))
	for _, want := range []string{"Invocation done", "inv-123", "image/png"} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("missing %s in %s", want, encoded)
		}
	}
	for _, secret := range []string{"argument-secret", "private-secret"} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("exposed %s", secret)
		}
	}
}

func TestToolFailuresAndFileDiffsRemainVisible(t *testing.T) {
	errorContent := string(acp.Raw(toolResultContent(json.RawMessage(`{"type":"mcpToolCall","error":{"message":"write rejected"}}`))))
	if !strings.Contains(errorContent, "write rejected") {
		t.Fatal(errorContent)
	}
	diff := json.RawMessage(`{"type":"fileChange","changes":[{"path":"old.txt","kind":{"type":"update","move_path":"new.txt"},"diff":"@@ -1 +1 @@\n-old\n+new"}]}`)
	content := string(acp.Raw(toolResultContent(diff)))
	for _, want := range []string{"old.txt", "new.txt", "@@ -1 +1 @@"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(content, `"type":"diff"`) {
		t.Fatal("unified diff misrepresented as complete replacement text")
	}
}

func TestReplaySignalsOmittedImageContext(t *testing.T) {
	h := &testHandler{}
	c := &Conn{handler: h}
	c.item("codex:thread", json.RawMessage(`{"id":"u","type":"userMessage","content":[{"type":"text","text":"What is shown?"},{"type":"localImage","path":"C:/private/image.png"},{"type":"image","url":"https://example.invalid/image"}]}`), true, true)
	if len(h.events) != 3 {
		t.Fatalf("replay events %d", len(h.events))
	}
	for i, kind := range []string{"text", "localImage", "image"} {
		var event struct {
			Update struct {
				Content struct {
					Type string `json:"type"`
				} `json:"content"`
			} `json:"update"`
		}
		if err := json.Unmarshal(h.events[i], &event); err != nil {
			t.Fatal(err)
		}
		if event.Update.Content.Type != kind {
			t.Fatalf("content type %s", event.Update.Content.Type)
		}
	}
}

func TestMCPCompletedItemUsesNormalizedContent(t *testing.T) {
	h := &testHandler{}
	c := &Conn{handler: h}
	c.item("codex:thread", json.RawMessage(`{"id":"tool","type":"mcpToolCall","server":"managed","tool":"save","status":"completed","result":{"content":[{"type":"text","text":"Saved once"}]}}`), true, false)
	var event struct {
		Update struct {
			Content []struct {
				Content struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"content"`
		} `json:"update"`
	}
	if len(h.events) != 1 || json.Unmarshal(h.events[0], &event) != nil || len(event.Update.Content) != 1 || event.Update.Content[0].Content.Text != "Saved once" {
		t.Fatalf("event %s", h.events)
	}
}
