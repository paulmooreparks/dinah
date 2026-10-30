package judge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The Ollama judge asks /api/chat at temperature 0 with the fixed seed and with
// thinking off, bounds the answer, asks for JSON only when the form is JSON, and
// reports the tokens Ollama counted.
func TestTheOllamaJudgeAsksAtTemperatureZeroWithAFixedSeed(t *testing.T) {
	var bodies []string
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		paths = append(paths, r.URL.Path)
		io.WriteString(w, `{"message":{"role":"assistant","content":"Parser release"},"done":true,"prompt_eval_count":41,"eval_count":3}`)
	}))
	defer srv.Close()
	o := &Ollama{Host: srv.URL + "/", Model: "tiny:4b"}
	resp, err := o.Ask(context.Background(), Request{System: "You write titles.", Input: `{"line":"x"}`, MaxTokens: 60})
	if err != nil || resp.Text != "Parser release" || resp.Model != "tiny:4b" || resp.Temperature != 0 || resp.Seed != DefaultSeed ||
		resp.Cost != (Cost{In: 41, Out: 3}) {
		t.Fatalf("%+v %v", resp, err)
	}
	if _, err := o.Ask(context.Background(), Request{System: "You write titles.", Input: `{"line":"x"}`, MaxTokens: 60}); err != nil {
		t.Fatal(err)
	}
	if bodies[0] != bodies[1] || paths[0] != "/api/chat" {
		t.Fatalf("the same request differs:\n%s\n%s (%s)", bodies[0], bodies[1], paths[0])
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(bodies[0]), &sent); err != nil {
		t.Fatal(err)
	}
	opts := sent["options"].(map[string]any)
	msgs := sent["messages"].([]any)
	if sent["model"] != "tiny:4b" || sent["stream"] != false || sent["think"] != false || opts["temperature"] != float64(0) || opts["seed"] != float64(DefaultSeed) ||
		opts["num_predict"] != float64(60) || sent["format"] != nil || len(msgs) != 2 || msgs[1].(map[string]any)["content"] != `{"line":"x"}` {
		t.Fatalf("the request was %s", bodies[0])
	}
	if _, err := (&Ollama{Host: srv.URL, Model: "tiny:4b", Seed: 99}).Ask(context.Background(), Request{System: "s", Input: "i", JSON: true, MaxTokens: 10}); err != nil {
		t.Fatal(err)
	}
	if last := bodies[len(bodies)-1]; !strings.Contains(last, `"format":"json"`) || !strings.Contains(last, `"seed":99`) {
		t.Fatalf("the JSON request was %s", last)
	}
}

// A server that refuses, or answers with something that is not JSON, or is not
// there, is an error to the call and never a panic, and one that is slow costs
// no more than the deadline.
func TestTheOllamaJudgeReportsFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":"model not found"}`)
	}))
	defer srv.Close()
	if _, err := (&Ollama{Host: srv.URL, Model: "tiny:4b"}).Ask(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("a refusal: %v", err)
	}
	junk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "<html>") }))
	defer junk.Close()
	if _, err := (&Ollama{Host: junk.URL, Model: "m"}).Ask(context.Background(), Request{}); err == nil {
		t.Fatal("text that is not JSON was accepted")
	}
	if _, err := (&Ollama{Host: "http://127.0.0.1:1", Model: "m"}).Ask(context.Background(), Request{}); err == nil {
		t.Fatal("a server that is not there answered")
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer slow.Close()
	j := newSet(t, &Ollama{Host: slow.URL, Model: "m"})
	start := time.Now()
	if r := Call(context.Background(), j, shortTitle, titleIn{Line: "Please tag the parser release"}, Effect{}); r.Why != WhyLate || time.Since(start) > 2*time.Second {
		t.Fatalf("a slow server: %+v after %v", r, time.Since(start))
	}
}
