package responsiveness

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func manualRecorder(anchor time.Time) *Recorder {
	r := &Recorder{anchor: anchor}
	r.generation.Store(1)
	r.enabled.Store(true)
	return r
}
func TestTimingRingBoundsWrapFloodAndClear(t *testing.T) {
	if bytes := unsafe.Sizeof(Recorder{}) + unsafe.Sizeof(runtimeReader{}); bytes > StorageLimit {
		t.Fatalf("owned storage %d exceeds %d", bytes, StorageLimit)
	}
	anchor := time.Now().Add(-time.Duration(3*Capacity-1)*Period - 250*time.Millisecond)
	r := manualRecorder(anchor)
	for i := 0; i < 3*Capacity; i++ {
		end := anchor.Add(time.Duration(i)*Period + 250*time.Millisecond)
		r.record(Token{1, end.Add(-2 * time.Millisecond), Other, App}, end, 0, 0, 0)
	}
	s := r.Snapshot()
	if len(s.Slots) != Capacity || s.Coverage.Overwritten != 2*Capacity {
		t.Fatalf("ring did not wrap: slots %d coverage %+v", len(s.Slots), s.Coverage)
	}
	end := anchor.Add(time.Duration(3*Capacity-1)*Period + 300*time.Millisecond)
	if allocs := testing.AllocsPerRun(10000, func() { r.record(Token{1, end.Add(-3 * time.Millisecond), Other, App}, end, 0, 0, 0) }); allocs != 0 {
		t.Fatalf("event flood allocated %g", allocs)
	}
	if len(r.Snapshot().Slots) != Capacity {
		t.Fatal("flood grew the ring")
	}
	before := r.slots[(3*Capacity-1)%Capacity]
	r.record(Token{1, anchor, Other, App}, anchor.Add(time.Duration(Capacity-1)*Period+time.Millisecond), 0, 0, 0)
	if r.slots[(3*Capacity-1)%Capacity] != before {
		t.Fatal("expired completion evicted a current slot")
	}
	r.Set(false)
	if r.slots != ([Capacity]storedSlot{}) || len(r.Snapshot().Slots) != 0 {
		t.Fatal("disable retained timings")
	}
}

func BenchmarkTimingRuntimeSample(b *testing.B) {
	reader := runtimeReader{}
	anchor := time.Now()
	reader.read(anchor, anchor)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.read(time.Now(), anchor)
	}
	b.ReportMetric(float64(unsafe.Sizeof(Recorder{})+unsafe.Sizeof(runtimeReader{})), "retained-B")
}
func TestTimingUnknownIsNotZeroAndMaximumKeepsItsOffset(t *testing.T) {
	anchor := time.Now().Add(-100 * time.Millisecond)
	r := manualRecorder(anchor)
	end := anchor.Add(75 * time.Millisecond)
	r.record(Token{1, end.Add(-2 * time.Second), Other, App}, end, 0, 0, 0)
	s := r.Snapshot().Slots[0]
	if *s.Categories.Other.App.Max.Value != 2000 || *s.Categories.Other.App.Offset.Value != 75 {
		t.Fatalf("maximum lost: %+v", s.Categories.Other.App)
	}
	if s.Runtime.GCCount.State != Unavailable || s.Queues.Depth.Value != nil || s.Categories.Text.App.Max.Value != nil || *s.Categories.Text.App.Count.Value != 0 {
		t.Fatal("unknown became measured zero")
	}
	if runtimeNumber(valueUnsupported, 0).State != NotSupported || runtimeNumber(valueMeasured, math.Inf(1)).Value != nil {
		t.Fatal("unsupported/nonfinite metric became a value")
	}
	x := stat{count: maxCounter, total: math.MaxUint64}
	x.add(time.Second, 0)
	if !x.saturated || x.count != maxCounter || x.total != math.MaxUint64 {
		t.Fatal("counter wrapped")
	}
}
func TestTimingGenerationRejectsOldCompletionAndJoinsSampler(t *testing.T) {
	r := &Recorder{}
	defer r.Close()
	first := r.Set(true)
	token := r.Begin("private-canary", App)
	if repeated := r.Set(true); repeated.Generation != first.Generation {
		t.Fatal("idempotent enable changed generation")
	}
	r.Set(false)
	second := r.Set(true)
	r.End(token, 0, 0, 0)
	if second.Generation <= first.Generation || *r.Snapshot().Slots[0].Categories.Other.App.Count.Value != 0 {
		t.Fatal("old completion entered a new recording")
	}
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 50; j++ {
				token := r.Begin("canary", App)
				r.End(token, 0, 0, 0)
				r.Set(j%2 == 0)
				_ = r.Snapshot()
			}
		}()
	}
	workers.Wait()
	r.Set(false)
	if r.stop != nil || r.done != nil {
		t.Fatal("disabled recorder retained a sampler")
	}
	r.Close()
	if r.Set(true).Enabled {
		t.Fatal("closed recorder restarted")
	}
}
func TestTimingExportHasOnlyClosedFieldsAndNoCanary(t *testing.T) {
	r := manualRecorder(time.Now().Add(-time.Millisecond))
	canary := "PRIVATE title/path/prompt/output https://private.invalid --private-command"
	token := r.Begin(canary, App)
	r.End(token, 0, 0, 0)
	data, err := json.Marshal(r.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "private.invalid") {
		t.Fatal("canary reached timing export")
	}
	// JSON field tags are the entire export vocabulary. Adding a field
	// requires a corresponding contract change rather than silently leaking it.
	expected := map[reflect.Type]string{
		reflect.TypeFor[Snapshot]():        "schemaVersion recording capturedAt coverage slots",
		reflect.TypeFor[State]():           "enabled generation anchor state",
		reflect.TypeFor[Clock]():           "utc monoMs",
		reflect.TypeFor[Span]():            "anchor startMonoMs durationMs complete",
		reflect.TypeFor[Coverage]():        "retainedSlots overwrittenSlots missedPeriods disabledObservations",
		reflect.TypeFor[Slot]():            "time categories queues runtime",
		reflect.TypeFor[Categories]():      "agent_text agent_tool chat_state team_state bus_change other",
		reflect.TypeFor[CategoryStats]():   "appEmit bridgeEnqueue",
		reflect.TypeFor[Stats]():           "count totalMs maxMs maxCompletedOffsetMs saturated",
		reflect.TypeFor[Queues]():          "subscriberPeak depthPeak overflowDisconnects",
		reflect.TypeFor[RuntimeInterval](): "scope time gcPauseCount gcPauseMaxBucketLowerMs gcPauseMaxBucketUpperMs gcPauseUpperOpen mutexWaitMs",
		reflect.TypeFor[Number]():          "state value",
	}
	for typ, fields := range expected {
		got := []string{}
		for i := 0; i < typ.NumField(); i++ {
			got = append(got, typ.Field(i).Tag.Get("json"))
		}
		if strings.Join(got, " ") != fields {
			t.Fatalf("%s changed closed fields: %v", typ, got)
		}
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
}
func TestTimingDisabledHotPathDoesNoAllocationOrLocking(t *testing.T) {
	r := &Recorder{}
	r.mu.Lock()
	defer r.mu.Unlock()
	if allocs := testing.AllocsPerRun(1000, func() { token := r.Begin("private", App); r.End(token, 0, 0, 0) }); allocs != 0 {
		t.Fatalf("disabled allocated %g", allocs)
	}
}
func BenchmarkTimingEvent(b *testing.B) {
	for _, on := range []bool{false, true} {
		name := "off"
		if on {
			name = "on"
		}
		b.Run(name, func(b *testing.B) {
			r := &Recorder{}
			if on {
				r.Set(true)
				defer r.Close()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				token := r.Begin("bus:changed", App)
				r.End(token, 0, 0, 0)
			}
		})
	}
}
