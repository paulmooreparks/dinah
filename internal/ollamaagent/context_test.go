package ollamaagent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"studio/internal/acp"
)

// A conversation saved before Ollama's own setting was an option holds the old
// default, which becomes that setting once, and so does one that had the old
// default put back by the options the window saved (revision 2). A length the
// person chose is kept, and so is one chosen after the change, even if it equals
// the old default.
func TestAnOldDefaultBecomesOllamasSettingOnce(t *testing.T) {
	srv, _ := fakeOllama(t, "read_file")
	dir := t.TempDir()
	c, err := Start(srv.URL, dir, &host{answer: "once"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	write := func(id string, context, rev int) {
		b, _ := json.Marshal(record{ID: id, Cwd: t.TempDir(), Model: "nano:4b", Context: context, ContextRev: rev})
		if err := os.WriteFile(c.file(id), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("ollama:olddefault", 65536, 0)
	write("ollama:oldchosen", 131072, 0)
	write("ollama:putback", 65536, 2)
	write("ollama:newchosen", 65536, contextRevision)
	for id, want := range map[string]int{"ollama:olddefault": 0, "ollama:putback": 0, "ollama:oldchosen": 131072, "ollama:newchosen": 65536} {
		rec, err := c.load(id)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Context != want {
			t.Errorf("%s holds %d, not %d", id, rec.Context, want)
		}
	}
}

// A length the person chose is sent as num_ctx and reported as chosen; one left
// to Ollama is not sent, and is reported against the model's own limit.
func TestAChosenSizeIsSentAndOllamasSettingIsNot(t *testing.T) {
	srv, seen := fakeOllama(t, "read_file")
	h := &host{answer: "once"}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	var set acp.ConfigOptionsResult
	if err := c.Call(ctx, "session/set_config_option", acp.SetConfigOptionParams{SessionID: opened.SessionID, ConfigID: "context", Value: "16384"}, &set); err != nil {
		t.Fatal(err)
	}
	var res acp.PromptResult
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Hello"}}}, &res); err != nil {
		t.Fatal(err)
	}
	said := h.said()
	if (*seen)[0].Options["num_ctx"] != float64(16384) || !strings.Contains(said, `"sizeKind":"chosen"`) || !strings.Contains(said, `"size":16384`) {
		t.Fatalf("a chosen size: %+v\n%s", (*seen)[0].Options, said)
	}
}
