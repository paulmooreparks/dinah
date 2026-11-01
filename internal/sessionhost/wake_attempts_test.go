package sessionhost

import (
	"context"
	"errors"
	"maps"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func wakeAttemptFixture() (*WakeAttempts, *map[string]WakeFailure) {
	stored := map[string]WakeFailure{}
	w := &WakeAttempts{Read: func() (map[string]WakeFailure, error) { return maps.Clone(stored), nil }, Write: func(next map[string]WakeFailure) error { stored = maps.Clone(next); return nil }, Now: func() time.Time { return time.Unix(1234, 0) }}
	return w, &stored
}

func safeWakeRoute() WakeRoute {
	return WakeRoute{Origin: "old", Target: "next", Recipient: "bus", TargetRecipient: "bus", Cwd: "workspace"}
}
func unchangedWake(r WakeRoute) (WakeRoute, error) { return r, nil }

func TestWakeAttemptJoinsPageAndFallbackAndRetiresOldGeneration(t *testing.T) {
	w, _ := wakeAttemptFixture()
	route := safeWakeRoute()
	id, fresh, err := w.Begin(route, false)
	if err != nil || !fresh {
		t.Fatal(id, fresh, err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	open := func(r WakeRoute) (Info, error) {
		calls.Add(1)
		close(entered)
		<-release
		return Info{ID: r.Target}, nil
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	wg.Add(1)
	go func() { defer wg.Done(); _, e, _ := w.Open(route.Origin, id, unchangedWake, open); results <- e }()
	<-entered
	if !w.DeliveryHeld(route.Target) {
		t.Fatal("delivery became available before completion")
	}
	wg.Add(1)
	go func() { defer wg.Done(); _, e, _ := w.Open(route.Origin, id, unchangedWake, open); results <- e }()
	close(release)
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal(e)
		}
	}
	if calls.Load() != 1 || w.DeliveryHeld(route.Target) {
		t.Fatal("join repeated open or retained successful hold")
	}
	newID, _, err := w.Begin(route, true)
	if err != nil || newID == id {
		t.Fatal(newID, err)
	}
	if _, err, _ := w.Open(route.Origin, id, unchangedWake, func(WakeRoute) (Info, error) { t.Fatal("retired fallback opened provider"); return Info{}, nil }); err == nil {
		t.Fatal("old generation was admitted")
	}
}

func TestWakeRecoverySurvivesRestartAndCannotBeBypassedByNewDeliveryOrRetry(t *testing.T) {
	w, stored := wakeAttemptFixture()
	route := safeWakeRoute()
	route.TargetRecipient = "different-bus"
	if _, _, err := w.Begin(route, false); err == nil {
		t.Fatal("different identity was admitted")
	}
	if (*stored)[route.Origin].Stage != "routing-recovery" {
		t.Fatal(*stored)
	}
	restarted := &WakeAttempts{Read: w.Read, Write: w.Write, Now: w.Now}
	for _, retry := range []bool{false, true, false} {
		if _, _, err := restarted.Begin(route, retry); err == nil {
			t.Fatal("unchanged recovery requirement was bypassed")
		}
	}
	if !restarted.DeliveryHeld(route.Target) {
		t.Fatal("recovery lost hold after restart")
	}
	route.TargetRecipient = route.Recipient
	if _, fresh, err := restarted.Begin(route, false); err != nil || !fresh {
		t.Fatal("verified ownership change remained blocked", err)
	}
}

func TestWakeAttemptCrashCancelAndExplicitRetryRetainOwnedState(t *testing.T) {
	w, stored := wakeAttemptFixture()
	route := safeWakeRoute()
	if _, _, err := w.Begin(route, false); err != nil {
		t.Fatal(err)
	}
	restarted := &WakeAttempts{Read: w.Read, Write: w.Write, Now: w.Now}
	if _, _, err := restarted.Begin(route, false); err == nil {
		t.Fatal("interrupted opening retried automatically")
	}
	id, _, err := restarted.Begin(route, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err, _ = restarted.Open(route.Origin, id, unchangedWake, func(WakeRoute) (Info, error) { return Info{}, context.Canceled })
	if !errors.Is(err, context.Canceled) || (*stored)[route.Origin].Stage != "failed" || !restarted.DeliveryHeld(route.Target) {
		t.Fatal(err, *stored)
	}
	if _, _, err := restarted.Begin(route, false); err == nil {
		t.Fatal("new delivery bypassed cancelled failure")
	}
}

func TestWakeAttemptSaveFailuresNeverAdmitOrReportDurableCompletion(t *testing.T) {
	for _, failureAt := range []int{1, 2} {
		t.Run(string(rune('0'+failureAt)), func(t *testing.T) {
			w, stored := wakeAttemptFixture()
			write := w.Write
			count := 0
			disk := errors.New("disk full")
			w.Write = func(records map[string]WakeFailure) error {
				count++
				if count == failureAt {
					return disk
				}
				return write(records)
			}
			route := safeWakeRoute()
			id, _, err := w.Begin(route, false)
			if failureAt == 1 {
				if !errors.Is(err, disk) || len(*stored) != 0 || w.ActiveTarget(route.Target) {
					t.Fatal(err, *stored)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err, _ = w.Open(route.Origin, id, unchangedWake, func(r WakeRoute) (Info, error) { return Info{ID: r.Target}, nil })
			if !errors.Is(err, disk) || !w.DeliveryHeld(route.Target) || (*stored)[route.Origin].Stage != "opening" {
				t.Fatal(err, *stored)
			}
		})
	}
}

func TestWakeConcurrentOriginsRetainSeparateOwnershipAndRouteRevalidation(t *testing.T) {
	w, stored := wakeAttemptFixture()
	first := safeWakeRoute()
	second := first
	second.Origin = "other"
	second.Recipient = "other-bus"
	second.TargetRecipient = "other-bus"
	old, _, err := w.Begin(first, false)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := w.Begin(second, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err, _ = w.Open(first.Origin, old, func(r WakeRoute) (WakeRoute, error) { r.TargetRecipient = "changed"; return r, nil }, func(WakeRoute) (Info, error) { t.Fatal("changed route opened"); return Info{}, nil })
	if err == nil || len(*stored) != 2 {
		t.Fatal(err, *stored)
	}
	_, err, _ = w.Open(second.Origin, other, unchangedWake, func(r WakeRoute) (Info, error) { return Info{ID: r.Target}, nil })
	if err != nil || len(*stored) != 1 || (*stored)[first.Origin].Stage != "routing-recovery" {
		t.Fatal(err, *stored)
	}
}

func TestWakeCoordinatorSettlesOriginOnceAndLateFallbackCannotReopen(t *testing.T) {
	w, _ := wakeAttemptFixture()
	var mu sync.Mutex
	var runtime Runtime
	now := time.Unix(1234, 0)
	route := safeWakeRoute()
	runtime.SetWindowsLocked([]string{route.Origin})
	runtime.ExpireWakeLocked(now)
	if _, connect := runtime.ConsiderWakeLocked(route.Origin, route.Cwd, 1, false, now); !connect {
		t.Fatal("wake was not admitted")
	}
	var request map[string]any
	var owner *BusChats
	var calls, failures int
	missing := errors.New("Resource not found: actual target")
	owner = NewBusChats(&mu, &runtime, BusChatsHost{WakeAttempts: w, ResolveWake: func(origin, cwd string) (WakeRoute, error) { return route, nil }, Clock: func() time.Time { return now },
		Emit: func(name string, value any) {
			if name == "chat:connect" {
				request = value.(map[string]any)
			}
		},
		OpenDeliveryChat: func(WakeRoute) (Info, error) { calls++; owner.Connected(route.Target, missing); return Info{}, missing },
		ConnectFailed: func(origin string, err error) {
			failures++
			if origin != route.Origin || !errors.Is(err, missing) {
				t.Fatal(origin, err)
			}
		}, Log: func(string, string, ...any) {},
	})
	owner.AskToConnect(route.Origin, route.Cwd, true)
	if request["sessionId"] != route.Target || request["deliveryOrigin"] != route.Origin {
		t.Fatal(request)
	}
	token := request["deliveryAttempt"].(string)
	if _, err := owner.OpenDeliveryAttempt(route.Origin, token); !errors.Is(err, missing) {
		t.Fatal(err)
	}
	if _, err := owner.OpenDeliveryAttempt(route.Origin, token); !errors.Is(err, missing) {
		t.Fatal(err)
	}
	if calls != 1 || failures != 1 {
		t.Fatal(calls, failures)
	}
	if !errors.Is(owner.ConnectFailedSince(route.Origin, now), missing) {
		t.Fatal("original wake did not receive completion")
	}
}

func TestWakeCachedStateAndExplicitReadErrorRecovery(t *testing.T) {
	w, _ := wakeAttemptFixture()
	read := w.Read
	reads := 0
	broken := true
	w.Read = func() (map[string]WakeFailure, error) {
		reads++
		if broken {
			return nil, errors.New("corrupt recovery store")
		}
		return read()
	}
	for range 100 {
		if !w.DeliveryHeld("unrelated", "recipient") {
			t.Fatal("unknown durable holds were ignored")
		}
	}
	if reads != 1 {
		t.Fatal("delivery path reread the store", reads)
	}
	if _, err := w.Snapshot(); err == nil {
		t.Fatal("read error was hidden")
	}
	broken = false
	if err := w.Reload(); err != nil {
		t.Fatal(err)
	}
	if w.DeliveryHeld("unrelated", "recipient") || reads != 2 {
		t.Fatal(reads)
	}
	route := safeWakeRoute()
	route.TargetRecipient = "target-own-bus"
	if _, _, err := w.Begin(route, false); err == nil {
		t.Fatal("conflict admitted")
	}
	if !w.DeliveryHeld(route.Target, route.Recipient) || w.DeliveryHeld(route.Target, route.TargetRecipient) || w.DeliveryHeld("other", "unrelated") {
		t.Fatal("hold escaped original recipient ownership")
	}
	for range 100 {
		w.DeliveryHeld(route.Target, route.TargetRecipient)
	}
	if reads != 2 {
		t.Fatal("write-through state was reread", reads)
	}
}
