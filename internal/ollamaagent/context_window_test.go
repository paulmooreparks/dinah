package ollamaagent

import (
	"context"
	"slices"
	"strings"
	"testing"

	"studio/internal/acp"
)

// The sizes offered double from 8K up to the model's own limit, which is always
// the top entry, and a model that reports none gets a short range.
func TestTheLadderDoublesToTheModelsOwnLimit(t *testing.T) {
	if !isCloudModel("kimi-k3:cloud") || !isCloudModel("gpt-oss:120b-cloud") || isCloudModel("nano:4b") || isCloudModel("cloudy:7b") || isCloudModel("llama-clouded:8b") {
		t.Error("cloud models are those whose name ends in :cloud or -cloud")
	}
	for _, tc := range []struct {
		limit int
		want  []int
	}{
		{1_000_000, []int{8192, 16384, 32768, 65536, 131072, 262144, 524288, 1_000_000}},
		{1 << 20, []int{8192, 16384, 32768, 65536, 131072, 262144, 524288, 1 << 20}},
		{65536, []int{8192, 16384, 32768, 65536}},
		{8192, []int{8192}},
		{4096, []int{4096}},
		{0, []int{8192, 16384, 32768}},
	} {
		if got := contextLadder(tc.limit); !slices.Equal(got, tc.want) {
			t.Errorf("limit %d: %v, not %v", tc.limit, got, tc.want)
		}
	}
	for n, want := range map[int]string{8192: "8K tokens", 1 << 20: "1M tokens", 1_000_000: "1000000 tokens", 131072: "128K tokens"} {
		if got := sizeName(n); got != want {
			t.Errorf("%d is named %q, not %q", n, got, want)
		}
	}
}

// A model with no reported limit says so in the option's description, and a
// model with one offers its sizes and nothing above it.
func TestTheContextOptionSaysWhenTheLimitIsNotReported(t *testing.T) {
	srv, _ := fakeOllama(t, "read_file")
	c, err := Start(srv.URL, t.TempDir(), &host{answer: "once"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	rec := record{Model: "nano:4b"}
	c.settle(context.Background(), &rec)
	option := func(rec record) acp.ConfigOption {
		for _, o := range c.options(rec) {
			if o.ID == "context" {
				return o
			}
		}
		t.Fatal("no context option")
		return acp.ConfigOption{}
	}
	with := option(rec)
	if strings.Contains(with.Description, "did not report") || strings.Contains(string(with.Options), "65536") || !strings.Contains(string(with.Options), "32768") {
		t.Fatalf("a model with a limit: %s %s", with.Description, with.Options)
	}
	c.mu.Lock()
	c.shown["nano:4b"] = shown{Capabilities: []string{"completion"}}
	c.mu.Unlock()
	without := option(rec)
	if !strings.Contains(without.Description, "did not report") || strings.Contains(string(without.Options), "65536") {
		t.Fatalf("a model with none: %s %s", without.Description, without.Options)
	}
}

// The meter says whether the window is known: a chosen size is, a cloud model on
// Ollama's setting is (by Paul's decision of 30 October 2026 that a name ending in
// :cloud or -cloud is one), and a local model on Ollama's setting is not.
func TestTheMeterSaysWhetherTheWindowIsKnown(t *testing.T) {
	for _, tc := range []struct {
		name, model, size string
		known, kind       string
	}{
		{"a local model on Ollama's setting", "nano:4b", "", `"windowKnown":false`, `"sizeKind":"model"`},
		{"a cloud model on Ollama's setting", "glm:cloud", "", `"windowKnown":true`, `"sizeKind":"model"`},
		{"a size chosen for a local model", "nano:4b", "16384", `"windowKnown":true`, `"sizeKind":"chosen"`},
	} {
		srv, _ := fakeOllama(t, "read_file")
		h := &host{answer: "once"}
		c, err := Start(srv.URL, t.TempDir(), h)
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		var opened acp.NewSessionResult
		if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
			t.Fatal(err)
		}
		var set acp.ConfigOptionsResult
		if err := c.Call(ctx, "session/set_config_option", acp.SetConfigOptionParams{SessionID: opened.SessionID, ConfigID: "model", Value: tc.model}, &set); err != nil {
			t.Fatal(err)
		}
		if tc.size != "" {
			if err := c.Call(ctx, "session/set_config_option", acp.SetConfigOptionParams{SessionID: opened.SessionID, ConfigID: "context", Value: tc.size}, &set); err != nil {
				t.Fatal(err)
			}
		}
		var res acp.PromptResult
		if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Hello"}}}, &res); err != nil {
			t.Fatal(err)
		}
		said := h.said()
		if !strings.Contains(said, tc.known) || !strings.Contains(said, tc.kind) {
			t.Errorf("%s: %s", tc.name, said)
		}
		c.Close()
	}
}
