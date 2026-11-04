package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"studio/internal/bus"
	"testing"
)

func TestResponsivenessTimingIsAbsentFromBusTools(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	me, err := b.RegisterSession(ctx, "window-timing", t.TempDir(), "Timing test")
	if err != nil {
		t.Fatal(err)
	}
	tools, err := connect(t, b, me.ID).ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SetResponsivenessRecording", "GetResponsivenessTimingSnapshot"} {
		if strings.Contains(string(data), name) {
			t.Fatal("shell-only method exposed by bus", name)
		}
	}
}
