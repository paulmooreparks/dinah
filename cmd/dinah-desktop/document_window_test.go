//go:build desktop

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/docservice"
)

// A dialog the person leaves open must not stop the window asking the
// service for commands, because the service takes a window that has stopped
// asking to have ended and closes its document.
func TestDocumentWindowKeepsAskingWhileADialogIsOpen(t *testing.T) {
	var mu sync.Mutex
	asked := 0
	var answer *string
	mux := http.NewServeMux()
	mux.HandleFunc("/control/ping", func(rw http.ResponseWriter, r *http.Request) {
		json.NewEncoder(rw).Encode(docservice.Endpoint{Addr: r.Host, Token: "t"})
	})
	mux.HandleFunc("/control/window/w1/next", func(rw http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked++
		first := asked == 1
		mu.Unlock()
		commands := []docservice.Command{}
		if first {
			commands = append(commands, docservice.Command{Seq: 1, Kind: "pick-open", Data: json.RawMessage(`{"folder":"f"}`)})
		} else {
			time.Sleep(20 * time.Millisecond)
		}
		json.NewEncoder(rw).Encode(commands)
	})
	mux.HandleFunc("/control/window/w1/reply", func(rw http.ResponseWriter, r *http.Request) {
		var req struct {
			Seq    int64  `json:"seq"`
			Result string `json:"result"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		answer = &req.Result
		mu.Unlock()
		rw.Write([]byte("{}"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	dir := t.TempDir()
	data, _ := json.Marshal(docservice.Endpoint{Addr: strings.TrimPrefix(server.URL, "http://"), Token: "t"})
	if err := os.WriteFile(filepath.Join(dir, "service.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := docservice.Find(dir)
	if err != nil {
		t.Fatal(err)
	}

	opened := make(chan struct{})
	release := make(chan struct{})
	w := &documentWindow{client: client, id: "w1", application: "editor"}
	w.pickOpen = func(context.Context, string) string {
		close(opened)
		<-release
		return "chosen.md"
	}
	done := make(chan struct{})
	go func() { w.listen(context.Background()); close(done) }()
	wait := func(what string, met func() bool) {
		t.Helper()
		for end := time.Now().Add(10 * time.Second); !met(); {
			if time.Now().After(end) {
				t.Fatal(what)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	select {
	case <-opened:
	case <-time.After(10 * time.Second):
		t.Fatal("the dialog was never shown")
	}
	mu.Lock()
	before := asked
	mu.Unlock()
	wait("the window stopped asking for commands while its dialog was open", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return asked >= before+3
	})
	mu.Lock()
	early := answer != nil
	mu.Unlock()
	if early {
		t.Fatal("the window answered before the person chose")
	}
	close(release)
	wait("the window never said what was chosen", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return answer != nil && *answer == "chosen.md"
	})
	w.mu.Lock()
	w.closing = true
	w.mu.Unlock()
	<-done
}
