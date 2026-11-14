package ollamaagent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestMemoryReadsAllocationsWithoutLoadingOrPrompting(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/ps":
			io.WriteString(w, `{"models":[{"name":"small:4b","digest":"fixture","size":3000000000,"size_vram":2000000000,"context_length":8192},{"name":"remote:cloud","size":9}]}`)
		case "/api/show":
			io.WriteString(w, `{"model_info":{"general.architecture":"small","small.context_length":16384}}`)
		default:
			t.Errorf("unexpected call: %s", r.URL.Path)
			http.Error(w, "not allowed", 500)
		}
	}))
	defer srv.Close()
	models, err := Memory(context.Background(), srv.URL, 32768)
	if err != nil || len(models) != 1 {
		t.Fatalf("memory = %+v, %v", models, err)
	}
	m := models[0]
	if m.Name != "small:4b" || m.Digest != "fixture" || m.Size != 3000000000 || m.VRAM != 2000000000 || m.Context != 8192 || m.Target != 16384 {
		t.Fatalf("reported allocation or effective ceiling changed: %+v", m)
	}
	if !reflect.DeepEqual(calls, []string{"GET /api/ps", "POST /api/show"}) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestMemoryDoesNotInventAnAllocationOrTargetWhenUnavailable(t *testing.T) {
	for _, mode := range []string{"empty", "ps-failed", "show-failed"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "ps-failed" || r.URL.Path == "/api/show" {
					http.Error(w, "unavailable", 503)
					return
				}
				if mode == "empty" {
					io.WriteString(w, `{"models":[]}`)
				} else {
					io.WriteString(w, `{"models":[{"name":"small:4b","size":3000000000,"size_vram":0,"context_length":8192}]}`)
				}
			}))
			defer srv.Close()
			models, err := Memory(context.Background(), srv.URL, 65536)
			switch mode {
			case "empty":
				if err != nil || models == nil || len(models) != 0 {
					t.Fatalf("empty = %+v, %v", models, err)
				}
			case "ps-failed":
				if err == nil {
					t.Fatal("ps failure was hidden")
				}
			case "show-failed":
				if err != nil || len(models) != 1 || models[0].Target != 0 || models[0].Size != 3000000000 {
					t.Fatalf("unavailable target = %+v, %v", models, err)
				}
			}
		})
	}
}
