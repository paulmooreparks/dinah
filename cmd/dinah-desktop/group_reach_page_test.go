package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// liveReachMethods are the App's calls the group composer and Settings make for
// group attention, and the bus calls they stand on; the live page check
// answers these from a real App and every other call from its stubs.
var liveReachMethods = map[string]bool{
	"BusOverview": true, "BusGroupMessages": true, "BusMarkGroupRead": true, "BusSendToGroup": true,
	"BusGroupReach": true, "BusGroupAttentionSettings": true, "SetBusGroupAttentionSettings": true,
}

// TestGroupReachPageAgainstEngine runs the page's group-reach check
// (frontend/e2e/live/group-reach-live.js) against a real App on a bus of its
// own, in a temporary folder, rather than against stubs. It starts a browser,
// so it runs only when DINAH_LIVE_PAGE is set; the browser takes the shared
// browser lock as every suite does.
func TestGroupReachPageAgainstEngine(t *testing.T) {
	if os.Getenv("DINAH_LIVE_PAGE") == "" {
		t.Skip("set DINAH_LIVE_PAGE=1 to run the page against the engine")
	}
	a, _, s, _, _ := busChat(t)
	var mu sync.Mutex
	var events []map[string]any
	a.sink = func(name string, data any) {
		if strings.HasPrefix(name, "bus:") {
			mu.Lock()
			events = append(events, map[string]any{"name": name, "data": data})
			mu.Unlock()
		}
	}
	if _, err := a.RenameSession(s.ID(), "Alpha"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("Live Reach", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Alpha"}, {ID: "closed-live-reach", Cwd: t.TempDir(), Title: "Beta"}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/events" {
			mu.Lock()
			out := events
			events = nil
			mu.Unlock()
			json.NewEncoder(w).Encode(out)
			return
		}
		var call struct {
			Method string            `json:"method"`
			Args   []json.RawMessage `json:"args"`
		}
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil || !liveReachMethods[call.Method] {
			http.Error(w, "no such call", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(callApp(a, call.Method, call.Args))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join("e2e", "live", "group-reach-live.js"), srv.URL, t.TempDir(), g.Name)
	cmd.Dir = "frontend"
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil || strings.Contains(string(out), "FAIL ") {
		t.Fatalf("the page's check against the engine failed: %v", err)
	}
}

// callApp calls one of the App's methods with arguments decoded from JSON,
// as the desktop bridge does, and gives back its result or its error.
func callApp(a *App, method string, raw []json.RawMessage) map[string]any {
	m := reflect.ValueOf(a).MethodByName(method)
	ft := m.Type()
	if len(raw) != ft.NumIn() {
		return map[string]any{"error": fmt.Sprintf("%s takes %d arguments, not %d", method, ft.NumIn(), len(raw))}
	}
	in := make([]reflect.Value, len(raw))
	for i := range raw {
		v := reflect.New(ft.In(i))
		if err := json.Unmarshal(raw[i], v.Interface()); err != nil {
			return map[string]any{"error": err.Error()}
		}
		in[i] = v.Elem()
	}
	res := m.Call(in)
	if e, ok := res[len(res)-1].Interface().(error); ok && e != nil {
		return map[string]any{"error": e.Error()}
	}
	if len(res) == 2 {
		return map[string]any{"result": res[0].Interface()}
	}
	return map[string]any{"result": nil}
}
