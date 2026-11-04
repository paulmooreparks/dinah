package bridge

import (
	"encoding/json"
	"strings"
	"testing"

	"studio/internal/responsiveness"
)

func TestBridgeTimingCannotExportPayloadAndMeasuresQueueOverflow(t *testing.T) {
	b := New(struct{}{})
	r := &responsiveness.Recorder{}
	r.Set(true)
	defer r.Close()
	b.SetTimingRecorder(r)
	ch := b.subscribe("")
	canary := "PRIVATE-title-path-prompt-provider-output-url-command"
	for i := 0; i < subscriberQueue+1; i++ {
		b.Emit(canary, map[string]any{"title": canary, "path": canary, "text": canary, "url": canary, "command": canary})
	}
	_ = ch
	view := r.Snapshot()
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), canary) {
		t.Fatal("event content reached timing export")
	}
	if len(view.Slots) == 0 || *view.Slots[0].Queues.Drops.Value != 1 || *view.Slots[0].Queues.Depth.Value != subscriberQueue || *view.Slots[0].Categories.Other.Bridge.Count.Value != subscriberQueue+1 {
		t.Fatalf("queue evidence missing: %+v", view)
	}
}

func BenchmarkBridgeResponsiveness(b *testing.B) {
	for _, mode := range []string{"none", "off", "on"} {
		b.Run(mode, func(b *testing.B) {
			bridge := New(struct{}{})
			bridge.ReplayEvents = 1
			if mode != "none" {
				r := &responsiveness.Recorder{}
				bridge.SetTimingRecorder(r)
				if mode == "on" {
					r.Set(true)
					defer r.Close()
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				bridge.Emit("bus:changed", nil)
			}
		})
	}
}
