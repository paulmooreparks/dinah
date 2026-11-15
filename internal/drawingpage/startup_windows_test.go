//go:build windows && native_drawing

package drawingpage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNativeUnresolvedStartup(t *testing.T) {
	root := os.Getenv("DRAWING_PAGE_EVIDENCE")
	if root == "" {
		t.Fatal("set DRAWING_PAGE_EVIDENCE")
	}
	_, file, _, _ := runtime.Caller(0)
	assets := os.DirFS(filepath.Join(filepath.Dir(file), "../../cmd/dinah-desktop/frontend"))
	for _, name := range []string{"late-handler", "withheld-handler"} {
		t.Run(name, func(t *testing.T) {
			gate := make(chan struct{})
			held := make(chan struct{}, 1)
			closed := make(chan struct{}, 1)
			delayStartupCompletion = func() <-chan struct{} { return gate }
			defer func() { delayStartupCompletion = nil }()
			released := false
			defer func() {
				if !released {
					close(gate)
				}
			}()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := Draw(ctx, assets, Request{Kind: "mermaid", Text: "graph TD; A-->B", Scale: 1}, Options{DataFolder: filepath.Join(root, name, "browser"), Event: func(event string) {
					if event == "startup-held" {
						held <- struct{}{}
					}
					if event == "controller-closed" {
						closed <- struct{}{}
					}
				}})
				result <- err
			}()
			select {
			case <-held:
			case <-time.After(10 * time.Second):
				t.Fatal("completion did not reach the delayed seam")
			}
			cancel()
			var pending *StartupError
			select {
			case err := <-result:
				if !errors.As(err, &pending) {
					t.Fatalf("unresolved startup: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("unresolved startup did not return a bounded refusal")
			}
			// The never-arriving case is observed while the fixture withholds delivery.
			// Only final fixture cleanup releases it, after this assertion.
			if name == "withheld-handler" {
				select {
				case <-pending.Settled:
					t.Fatal("withheld handler reported disposal")
				case <-time.After(100 * time.Millisecond):
				}
			}
			close(gate)
			released = true
			select {
			case err := <-pending.Settled:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("late completion was not disposed")
			}
			select {
			case <-closed:
			default:
				t.Fatal("late controller was not closed")
			}
			delayStartupCompletion = nil
			p, err := Draw(context.Background(), assets, Request{Kind: "mermaid", Text: "graph TD; A-->B", Scale: 1}, Options{DataFolder: filepath.Join(root, name, "fresh-browser")})
			if err != nil || len(p.PNG) == 0 {
				t.Fatalf("fresh draw after startup disposal: %v", err)
			}
		})
	}
}
