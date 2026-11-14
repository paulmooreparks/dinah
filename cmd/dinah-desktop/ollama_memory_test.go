package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaMemoryUsesSavedCeilingAndIsAPageQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ps":
			io.WriteString(w, `{"models":[{"name":"fixture:4b","size":3000000000,"size_vram":2000000000,"context_length":8192}]}`)
		case "/api/show":
			io.WriteString(w, `{"model_info":{"general.architecture":"fixture","fixture.context_length":131072}}`)
		default:
			t.Errorf("unexpected call: %s", r.URL.Path)
			http.Error(w, "not permitted", 500)
		}
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_HOST", srv.URL)
	a, _ := providerTestApp(t)
	if err := a.SetOllamaContextCap(32768); err != nil {
		t.Fatal(err)
	}
	reading, err := a.OllamaMemory()
	if err != nil || reading.Limit != 32768 || len(reading.Models) != 1 || reading.Models[0].Target != 32768 || reading.Models[0].Context != 8192 {
		t.Fatalf("memory = %+v, %v", reading, err)
	}
	if class, ok := pageMethods["OllamaMemory"]; !ok || class != pageQuery {
		t.Fatal("memory reading is not a page query")
	}
}
