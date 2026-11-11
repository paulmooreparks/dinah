//go:build desktop

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"studio/internal/docservice"
)

// A person may leave the system's Open or Save As dialog on the screen for
// any length of time. The service takes a window that has stopped asking
// for commands to have ended, and closes its document after six seconds,
// so the window has to go on asking while its dialog is open. Each case
// here runs the window's own loop against a real service, with a dialog
// that stays open longer than that and is then answered or cancelled.
func TestDocumentWindowOutlivesAnOpenDialog(t *testing.T) {
	const held = 8 * time.Second
	cases := []struct {
		name, method string
		choose       bool
	}{
		{"open chosen", "PickOpen", true},
		{"open cancelled", "PickOpen", false},
		{"save as chosen", "PickSave", true},
		{"save as cancelled", "PickSave", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			files := fstest.MapFS{"document.html": {Data: []byte("<!doctype html><title>page</title>")}}
			service, err := docservice.New(docservice.Options{Dir: dir, Version: "test", Files: http.FileServerFS(files), Page: "document.html",
				Cookie: "doc", Policy: "default-src 'self'", Idle: time.Hour,
				Applications: map[string]docservice.Application{"editor": {Applet: "editor", Name: "Test Editor"}}})
			if err != nil {
				t.Fatal(err)
			}
			go service.Serve()
			defer service.Stop()
			var client *docservice.Client
			for end := time.Now().Add(5 * time.Second); client == nil; time.Sleep(10 * time.Millisecond) {
				if client, err = docservice.Find(dir); err != nil && time.Now().After(end) {
					t.Fatal(err)
				}
			}
			folder := t.TempDir()
			other := filepath.Join(folder, "other.md")
			if err := os.WriteFile(other, []byte("other\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			opened, err := client.Open(context.Background(), docservice.OpenRequest{Application: "editor"})
			if err != nil {
				t.Fatal(err)
			}

			shown := make(chan struct{}, 1)
			pick := func() string {
				shown <- struct{}{}
				time.Sleep(held)
				if c.choose {
					return other
				}
				return ""
			}
			w := &documentWindow{client: client, id: opened.ID, application: "editor"}
			w.pickOpen = func(context.Context, string) string { return pick() }
			w.pickSave = func(context.Context, string, string) string { return pick() }
			ended := make(chan struct{})
			go func() { w.listen(context.Background()); close(ended) }()
			defer func() {
				w.mu.Lock()
				w.closing = true
				w.mu.Unlock()
				client.Focus(context.Background(), opened.ID)
				<-ended
			}()

			jar, _ := cookiejar.New(nil)
			page := &http.Client{Jar: jar}
			res, err := page.Get(opened.Entry)
			if err != nil || res.StatusCode != http.StatusOK {
				t.Fatalf("the way in: %v %v", res, err)
			}
			res.Body.Close()
			args := []json.RawMessage{}
			if c.method == "PickSave" {
				args = append(args, json.RawMessage(`"other.md"`))
			}
			payload, _ := json.Marshal(map[string]any{"method": c.method, "args": args})
			req, _ := http.NewRequest(http.MethodPost, "http://"+service.Addr()+"/d/"+opened.ID+"/bridge/call", bytes.NewReader(payload))
			req.Header.Set("Origin", "http://"+service.Addr())
			began := time.Now()
			res, err = page.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(res.Body)
			res.Body.Close()
			select {
			case <-shown:
			default:
				t.Fatalf("no dialog was shown: %s", data)
			}
			if time.Since(began) < held {
				t.Fatalf("the page was answered before the person chose: %s", data)
			}
			var answer struct {
				Result json.RawMessage `json:"result"`
				Error  string          `json:"error"`
			}
			if err := json.Unmarshal(data, &answer); err != nil || res.StatusCode != http.StatusOK || answer.Error != "" {
				t.Fatalf("%s after a dialog left open: %s %s", c.method, res.Status, data)
			}
			if chosen := strings.Contains(string(answer.Result), "other.md"); chosen != c.choose {
				t.Fatalf("%s returned %s", c.method, answer.Result)
			}
			select {
			case <-ended:
				t.Fatal("the window gave up on the service")
			default:
			}
			if list := service.Documents(); len(list) != 1 || list[0].ID != opened.ID {
				t.Fatalf("the service closed the document: %+v", list)
			}
		})
	}
}
