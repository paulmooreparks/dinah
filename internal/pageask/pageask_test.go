package pageask

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAskGoesToTheWindowProgramsAsker(t *testing.T) {
	Set(nil)
	if _, err := Ask(context.Background(), "picture", nil, time.Second); !errors.Is(err, ErrNoPage) {
		t.Fatalf("with no window: %v", err)
	}
	var kind string
	var args any
	var within time.Duration
	Set(func(_ context.Context, k string, a any, w time.Duration) (string, error) {
		kind, args, within = k, a, w
		return `{"ok":true}`, nil
	})
	t.Cleanup(func() { Set(nil) })
	out, err := Ask(context.Background(), "picture", map[string]string{"format": "png"}, 3*time.Second)
	if err != nil || out != `{"ok":true}` || kind != "picture" || within != 3*time.Second || args.(map[string]string)["format"] != "png" {
		t.Fatalf("asked %q %v %v, answered %q %v", kind, args, within, out, err)
	}
}
