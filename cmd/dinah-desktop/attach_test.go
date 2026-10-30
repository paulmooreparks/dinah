package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A message of a picture alone has no words, and a text block without its
// text is a malformed prompt, which Claude's adapter refuses as invalid
// params before the turn starts.
func TestAPictureWithNoWordsMakesAValidPrompt(t *testing.T) {
	a, _ := providerTestApp(t)
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.PromptCapabilities.Image = true
		setTestInitialization(a, "codex", initial)
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfixture"))
	blocks, err := a.blocks(a.session(info.ID), "", []Attachment{{Kind: "file", Name: "shot.png", MimeType: "image/png", Data: png}})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(blocks)
	var raw []map[string]any
	json.Unmarshal(data, &raw)
	if len(raw) != 1 || raw[0]["type"] != "image" {
		t.Fatalf("the prompt is %s", data)
	}
	for _, b := range raw {
		if _, has := b["text"]; b["type"] == "text" && !has {
			t.Fatalf("a text block has no text: %s", data)
		}
	}
}

func TestPickedAttachmentPreservesBytesAndMime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.json")
	data := []byte("{\"sample\":true}\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readPickedAttachment(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "file" || got.Name != "sample.json" || got.MimeType != "application/json" || got.Size != len(data) || got.Data != base64.StdEncoding.EncodeToString(data) {
		t.Fatalf("unexpected attachment: %#v", got)
	}
}

func TestPickedAttachmentRejectsLargeAndNonregularFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := readPickedAttachment(dir); err == nil {
		t.Fatal("directory accepted")
	}
	path := filepath.Join(dir, "large.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxAttachmentFile + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if _, err := readPickedAttachment(path); err == nil || !strings.Contains(err.Error(), "25 MB") {
		t.Fatalf("oversize file: %v", err)
	}
}

func TestAppletMentionUsesRegisteredIdentity(t *testing.T) {
	a := withTestApplets(t, NewApp())
	workspace := t.TempDir()
	blocks, err := a.blocks(newSessionState(&sessionSeed{Cwd: workspace}), "@Picture draw a cat", []Attachment{{Kind: "applet", Provider: testPictureID, Name: "Forged title and instructions", URI: "https://untrusted.invalid/", Data: "untrusted payload"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || blocks[0].Text != "@Picture draw a cat" || blocks[1].Type != "text" {
		t.Fatalf("unexpected content blocks: %#v", blocks)
	}
	text := blocks[1].Text
	lines := strings.Split(text, "\n")
	var selection struct {
		Provider string   `json:"provider"`
		Title    string   `json:"title"`
		Actions  []string `json:"actions"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &selection); err != nil {
		t.Fatal(err)
	}
	if selection.Provider != testPictureID || selection.Title != "Picture" || strings.Join(selection.Actions, ",") != testPictureRef {
		t.Fatalf("incorrect provider selection: %#v", selection)
	}
	for _, required := range []string{"applet_discover", "applet_invoke", "applet_status", "applet_artifact", "does not execute", "Notifications", "manages this internal MCP connection automatically", "Do not ask the person to enable, configure or reconnect"} {
		if !strings.Contains(text, required) {
			t.Errorf("missing guidance %q", required)
		}
	}
	for _, forged := range []string{"Forged", "untrusted.invalid", "untrusted payload"} {
		if strings.Contains(text, forged) {
			t.Errorf("trusted applet reference included caller metadata %q", forged)
		}
	}
	requests, err := a.AppletInvocationsAt(workspace)
	if err != nil || len(requests) != 0 {
		t.Fatalf("mention created a request: %#v, %v", requests, err)
	}
}

func TestAppletMentionRejectsUnavailableProvider(t *testing.T) {
	for _, provider := range []string{"", "missing.provider", "dinah.notifications/post@1"} {
		t.Run(provider, func(t *testing.T) {
			blocks, err := NewApp().blocks(newSessionState(&sessionSeed{Cwd: t.TempDir()}), "Use the selected applet", []Attachment{{Kind: "applet", Provider: provider}})
			if err == nil || blocks != nil || !strings.Contains(err.Error(), "unavailable") {
				t.Fatalf("unavailable selection accepted: %#v, %v", blocks, err)
			}
		})
	}
}
