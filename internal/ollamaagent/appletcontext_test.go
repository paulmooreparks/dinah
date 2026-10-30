package ollamaagent

import (
	"context"
	"os"
	"strings"
	"testing"

	"studio/internal/acp"
)

const liveContext = "dinah-applet-v1:ABCDEFGHIJKLMNOPQRSTUVWXYZ.ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// A saved record holds no applet context, while the request sent to the model
// still has the current turn's.
func TestASavedRecordHoldsNoAppletContext(t *testing.T) {
	srv, seen := fakeOllama(t, "read_file")
	dir := t.TempDir()
	c, err := Start(srv.URL, dir, &host{answer: "once"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	var res acp.PromptResult
	prompt := []acp.ContentBlock{{Type: "text", Text: "When does it ship?"}, {Type: "text", Text: "[Dinah Applet Context]\nUse " + liveContext}}
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: prompt}, &res); err != nil {
		t.Fatal(err)
	}
	sent := ""
	for _, req := range *seen {
		for _, m := range req.Messages {
			sent += m.Content
		}
	}
	if !strings.Contains(sent, liveContext) {
		t.Fatalf("the model was not given the current context: %q", sent)
	}
	b, err := os.ReadFile(c.file(opened.SessionID))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "ABCDEFGHIJ") || !strings.Contains(string(b), "[private applet context]") {
		t.Fatalf("the saved record: %s", b)
	}
}

// Records saved before contexts were stripped are stripped when the connection
// starts, and a record without one is left as it is.
func TestStoredRecordsAreStrippedAtStart(t *testing.T) {
	srv, _ := fakeOllama(t, "read_file")
	state := t.TempDir()
	c, err := Start(srv.URL, state, &host{answer: "once"})
	if err != nil {
		t.Fatal(err)
	}
	dirty, clean := c.file("ollama:dirty"), c.file("ollama:clean")
	os.WriteFile(dirty, []byte(`{"id":"ollama:dirty","messages":[{"role":"user","content":"`+liveContext+`"}]}`), 0o600)
	os.WriteFile(clean, []byte(`{"id":"ollama:clean","messages":[]}`), 0o600)
	c.Close()
	c2, err := Start(srv.URL, state, &host{answer: "once"})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	if b, _ := os.ReadFile(dirty); strings.Contains(string(b), "ABCDEFGHIJ") || !strings.Contains(string(b), "[private applet context]") {
		t.Fatalf("the stored record: %s", b)
	}
	if b, _ := os.ReadFile(clean); string(b) != `{"id":"ollama:clean","messages":[]}` {
		t.Fatalf("a clean record was changed: %s", b)
	}
}

// A first message of attachments only has no text of the person's, so the title
// is not made from the heading Dinah injects; and a stored title made that way
// is repaired when the connection starts.
func TestATitleIsMadeOnlyFromThePersonsWords(t *testing.T) {
	srv, _ := fakeOllama(t, "read_file")
	state := t.TempDir()
	c, err := Start(srv.URL, state, &host{answer: "once"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	var res acp.PromptResult
	prompt := []acp.ContentBlock{{Type: "text", Text: ""}, {Type: "text", Text: "[Dinah Applet Context]\nUse " + liveContext}}
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: prompt}, &res); err != nil {
		t.Fatal(err)
	}
	rec, err := c.load(opened.SessionID)
	if err != nil || rec.Title != "" {
		t.Fatalf("an attachments-only first message gave the title %q (%v)", rec.Title, err)
	}
	// A title with the person's own words is made from them, not the block.
	prompt = []acp.ContentBlock{{Type: "text", Text: "[Dinah Applet Context]\nUse " + liveContext}, {Type: "text", Text: "When does the release ship?\nMore."}}
	var second acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &second); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: second.SessionID, Prompt: prompt}, &res); err != nil {
		t.Fatal(err)
	}
	if rec, _ := c.load(second.SessionID); rec.Title != "When does the release ship?" {
		t.Fatalf("the title is %q", rec.Title)
	}
	// A record stored with the heading as its title is repaired at start.
	bad := c.file("ollama:badtitle")
	os.WriteFile(bad, []byte(`{"id":"ollama:badtitle","title":"[Dinah Applet Context]","messages":[{"role":"user","content":"[Dinah Applet Context]\n\nWhat is the plan?"}]}`), 0o600)
	c.Close()
	c2, err := Start(srv.URL, state, &host{answer: "once"})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	if rec, err := c2.load("ollama:badtitle"); err != nil || rec.Title != "What is the plan?" {
		t.Fatalf("the repaired title is %q (%v)", rec.Title, err)
	}
}
