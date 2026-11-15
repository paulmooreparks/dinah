package diagrampicture

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixturePicture() Picture {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	return Picture{Data: b.Bytes(), Width: 2, Height: 2}
}
func fixtureRequest(work string) Request {
	return Request{RequestID: "bf1496d2-f919-4710-97eb-40d8a339fd20", WorkspaceID: work, Source: Snapshot{Path: "draw.mmd", SHA256: Hash([]byte("graph TD; A-->B"))}, Pictures: []Snapshot{}, Format: "png", Scale: 1}
}

type fixture struct {
	s          *Service
	url, token string
}

func serviceFixture(t *testing.T, render Renderer) *fixture {
	t.Helper()
	s := NewService("", render)
	server := httptest.NewServer(s)
	u, _ := url.Parse(server.URL)
	s.host = u.Host
	token, err := s.Issue("work", "tools", func(context.Context, string, int) ([]byte, error) { return []byte("graph TD; A-->B"), nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(); server.Close() })
	return &fixture{s: s, url: server.URL, token: token}
}
func (f *fixture) call(method, route, token, offer string, body []byte) (int, []byte) {
	r, _ := http.NewRequest(method, f.url+route, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Helper-Channel", "diagram-picture")
	r.Header.Set("X-Helper-Protocols", offer)
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := io.ReadAll(response.Body)
	return response.StatusCode, b
}
func encoded(r Request) []byte { b, _ := json.Marshal(r); return b }
func TestServiceAuthorityNegotiationAndSnapshot(t *testing.T) {
	var draws atomic.Int32
	f := serviceFixture(t, func(ctx context.Context, d Drawing) (Picture, error) {
		draws.Add(1)
		if string(d.Source) != "graph TD; A-->B" {
			t.Error("wrong retained source")
		}
		return fixturePicture(), nil
	})
	for _, tc := range []struct {
		token, offer string
		status       int
	}{{"", "1", 401}, {"applet-context", "1", 401}, {f.token, "2", 412}, {f.token, "0", 400}, {f.token, "1,2", 204}, {f.token, "", 204}} {
		status, _ := f.call("OPTIONS", Route, tc.token, tc.offer, []byte("not JSON"))
		if status != tc.status {
			t.Fatalf("negotiation status %d want %d", status, tc.status)
		}
	}
	r := fixtureRequest("other")
	status, _ := f.call("POST", Route, f.token, "1", encoded(r))
	if status != 403 {
		t.Fatal(status)
	}
	r = fixtureRequest("work")
	r.Source.SHA256 = strings.Repeat("0", 64)
	status, _ = f.call("POST", Route, f.token, "1", encoded(r))
	if status != 409 {
		t.Fatal(status)
	}
	r = fixtureRequest("work")
	r.RequestID = "cf1496d2-f919-4710-97eb-40d8a339fd20"
	c, err := New(f.url, f.token, "work")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Draw(context.Background(), r)
	if err != nil || len(p.Data) == 0 {
		t.Fatalf("draw: %v", err)
	}
	status, _ = f.call("POST", Route, f.token, "1", encoded(r))
	if status != 409 || draws.Load() != 1 {
		t.Fatalf("replay status %d draws %d", status, draws.Load())
	}
	f.s.Revoke(f.token)
	status, _ = f.call("OPTIONS", Route, f.token, "1", nil)
	if status != 401 {
		t.Fatal(status)
	}
}
func TestServiceCapacityCancelAndDisposeFailure(t *testing.T) {
	started := make(chan struct{}, 2)
	disposed := make(chan struct{}, 2)
	f := serviceFixture(t, func(ctx context.Context, d Drawing) (Picture, error) {
		started <- struct{}{}
		<-ctx.Done()
		disposed <- struct{}{}
		return Picture{}, ctx.Err()
	})
	done := make(chan int, 1)
	go func() {
		status, _ := f.call("POST", Route, f.token, "1", encoded(fixtureRequest("work")))
		done <- status
	}()
	<-started
	other, _ := f.s.Issue("work", "tools", func(context.Context, string, int) ([]byte, error) { return []byte("graph TD; A-->B"), nil })
	r := fixtureRequest("work")
	r.RequestID = "cf1496d2-f919-4710-97eb-40d8a339fd20"
	status, _ := f.call("POST", Route, other, "1", encoded(r))
	if status != 429 {
		t.Fatal(status)
	}
	// Cancellation from another lease must not reach the active request.
	status, _ = f.call("DELETE", Route+"/"+fixtureRequest("work").RequestID, other, "1", nil)
	if status != 204 {
		t.Fatal(status)
	}
	select {
	case <-disposed:
		t.Fatal("another lease cancelled the page")
	default:
	}
	status, _ = f.call("DELETE", Route+"/"+fixtureRequest("work").RequestID, f.token, "1", nil)
	if status != 204 {
		t.Fatal(status)
	}
	<-disposed
	if <-done != 422 {
		t.Fatal("cancel did not return refusal")
	}
	f.s.mu.Lock()
	f.s.render = func(context.Context, Drawing) (Picture, error) {
		return Picture{}, refused(503, "renderer_unavailable", "forced Close failure")
	}
	f.s.mu.Unlock()
	status, _ = f.call("POST", Route, f.token, "1", encoded(r))
	if status != 503 {
		t.Fatal(status)
	}
	f.s.mu.Lock()
	f.s.render = func(context.Context, Drawing) (Picture, error) {
		t.Error("disabled renderer was entered")
		return fixturePicture(), nil
	}
	f.s.mu.Unlock()
	r.RequestID = "df1496d2-f919-4710-97eb-40d8a339fd20"
	status, _ = f.call("POST", Route, f.token, "1", encoded(r))
	if status != 503 {
		t.Fatal(status)
	}
}
func TestServiceSnapshotRetainsBytesAndBounds(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "draw.mmd")
	original := []byte("graph TD; A-->B")
	_ = os.WriteFile(name, original, 0600)
	f := serviceFixture(t, func(ctx context.Context, d Drawing) (Picture, error) {
		_ = os.WriteFile(name, []byte("changed"), 0600)
		if !bytes.Equal(d.Source, original) {
			t.Error("snapshot changed after read")
		}
		return fixturePicture(), nil
	})
	token, _ := f.s.Issue("snapshot", "tools", func(ctx context.Context, p string, limit int) ([]byte, error) { return os.ReadFile(name) })
	c, _ := New(f.url, token, "snapshot")
	_, err := c.Draw(context.Background(), fixtureRequest("snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	r := fixtureRequest("work")
	r.Source.Path = "../outside.mmd"
	status, _ := f.call("POST", Route, f.token, "1", encoded(r))
	if status != 400 {
		t.Fatal(status)
	}
	status, _ = f.call("POST", Route, f.token, "1", bytes.Repeat([]byte(" "), MaxRequest+1))
	if status != 413 {
		t.Fatal(status)
	}
	f.s.mu.Lock()
	for _, l := range f.s.leases {
		l.expires = time.Now().Add(-time.Second)
	}
	f.s.mu.Unlock()
	status, _ = f.call("OPTIONS", Route, f.token, "1", nil)
	if status != 401 {
		t.Fatal(status)
	}
}

func TestServiceUnresolvedStartupBlocksAndRecovers(t *testing.T) {
	for _, permanent := range []bool{false, true} {
		t.Run(map[bool]string{false: "late success", true: "late Close failure"}[permanent], func(t *testing.T) {
			settled := make(chan error, 1)
			var calls atomic.Int32
			f := serviceFixture(t, func(context.Context, Drawing) (Picture, error) {
				if calls.Add(1) == 1 {
					return Picture{}, RecoverAfterStartup(settled)
				}
				return fixturePicture(), nil
			})
			r := fixtureRequest("work")
			status, b := f.call("POST", Route, f.token, "1", encoded(r))
			if status != 503 || !bytes.Contains(b, []byte("earlier page")) {
				t.Fatalf("startup refusal %d %s", status, b)
			}
			r.RequestID = "cf1496d2-f919-4710-97eb-40d8a339fd20"
			status, _ = f.call("POST", Route, f.token, "1", encoded(r))
			if status != 503 || calls.Load() != 1 {
				t.Fatal("unresolved startup admitted another page")
			}
			if permanent {
				settled <- context.Canceled
			} else {
				settled <- nil
			}
			deadline := time.After(time.Second)
			for {
				f.s.mu.Lock()
				pending := f.s.unresolved
				f.s.mu.Unlock()
				if pending == 0 {
					break
				}
				select {
				case <-deadline:
					t.Fatal("startup completion was not observed")
				default:
					time.Sleep(time.Millisecond)
				}
			}
			status, _ = f.call("POST", Route, f.token, "1", encoded(r))
			if permanent {
				if status != 503 || calls.Load() != 1 {
					t.Fatal("Close failure re-enabled drawing")
				}
			} else if status != 200 || calls.Load() != 2 {
				t.Fatalf("startup did not recover %d", status)
			}
		})
	}
}

func TestServiceWindowCapacityAndWorkspaceRevocation(t *testing.T) {
	started := make(chan string, 3)
	f := serviceFixture(t, func(ctx context.Context, d Drawing) (Picture, error) {
		started <- d.Request.WorkspaceID
		<-ctx.Done()
		return Picture{}, ctx.Err()
	})
	read := func(context.Context, string, int) ([]byte, error) { return []byte("graph TD; A-->B"), nil }
	token2, _ := f.s.Issue("second", "tools", read)
	token3, _ := f.s.Issue("third", "tools", read)
	done := make(chan int, 3)
	launch := func(work, token string) {
		go func() { status, _ := f.call("POST", Route, token, "1", encoded(fixtureRequest(work))); done <- status }()
		<-started
	}
	launch("work", f.token)
	launch("second", token2)
	status, _ := f.call("POST", Route, token3, "1", encoded(fixtureRequest("third")))
	if status != 429 {
		t.Fatal("third window page was admitted")
	}
	f.s.RevokeWorkspace("work")
	if <-done != 422 {
		t.Fatal("workspace revocation did not cancel drawing")
	}
	status, _ = f.call("OPTIONS", Route, f.token, "1", nil)
	if status != 401 {
		t.Fatal("workspace lease survived revocation")
	}
	launch("third", token3)
	f.s.Close()
	if <-done != 422 || <-done != 422 {
		t.Fatal("window close did not dispose active work")
	}
}

func TestServiceReplayBudgetRequiresRenewal(t *testing.T) {
	f := serviceFixture(t, func(context.Context, Drawing) (Picture, error) { return fixturePicture(), nil })
	f.s.mu.Lock()
	l := f.s.leases[Hash([]byte(f.token))]
	for i := 0; i < 256; i++ {
		l.terminal[strconv.Itoa(i)] = time.Now()
	}
	f.s.mu.Unlock()
	status, _ := f.call("POST", Route, f.token, "1", encoded(fixtureRequest("work")))
	if status != 403 {
		t.Fatal("exhausted lease admitted a page")
	}
}
