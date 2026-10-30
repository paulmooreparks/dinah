package ollamaagent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"studio/internal/acp"
)

// The conversation's model says whether it reads pictures: "vision" among its
// capabilities is yes, its absence is no, and a model Ollama cannot describe is
// unknown.
func TestImageInputFollowsTheModelsVisionCapability(t *testing.T) {
	models := map[string]string{
		"seer:7b":  `{"capabilities":["completion","vision"],"model_info":{"general.architecture":"seer","seer.context_length":8192}}`,
		"blind:7b": `{"capabilities":["completion","tools"],"model_info":{"general.architecture":"blind","blind.context_length":8192}}`,
		// A server too old to report capabilities answers without the list, or
		// with an empty one.
		"old:7b":   `{"model_info":{"general.architecture":"old","old.context_length":8192}}`,
		"empty:7b": `{"capabilities":[],"model_info":{"general.architecture":"empty","empty.context_length":8192}}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			io.WriteString(w, `{"version":"0.34.1"}`)
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"seer:7b","model":"seer:7b","details":{}},{"name":"blind:7b","model":"blind:7b","details":{}},{"name":"gone:7b","model":"gone:7b","details":{}},{"name":"old:7b","model":"old:7b","details":{}},{"name":"empty:7b","model":"empty:7b","details":{}}]}`)
		case "/api/show":
			b, _ := io.ReadAll(r.Body)
			for name, body := range models {
				if string(b) == `{"model":"`+name+`"}` {
					io.WriteString(w, body)
					return
				}
			}
			http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, err := Start(srv.URL, t.TempDir(), &host{answer: "once"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	for model, want := range map[string]string{"seer:7b": "yes", "blind:7b": "no", "gone:7b": "unknown", "old:7b": "unknown", "empty:7b": "unknown"} {
		c.mu.Lock()
		c.sessions["ollama:"+model] = &session{rec: record{ID: "ollama:" + model, Model: model}}
		c.mu.Unlock()
		var res ImageInputResult
		if err := c.Call(ctx, ImageInputMethod, acp.CancelParams{SessionID: "ollama:" + model}, &res); err != nil {
			t.Fatal(err)
		}
		if res.ImageInput != want {
			t.Errorf("%s: %q, not %q", model, res.ImageInput, want)
		}
	}
	var res ImageInputResult
	if err := c.Call(ctx, ImageInputMethod, acp.CancelParams{SessionID: "ollama:closed"}, &res); err != nil || res.ImageInput != "unknown" {
		t.Errorf("a conversation that is not open: %q %v", res.ImageInput, err)
	}
}
