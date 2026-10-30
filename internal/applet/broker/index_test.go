package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio/internal/layout"
)

func ids(list []Invocation) map[string]bool {
	out := map[string]bool{}
	for _, v := range list {
		out[v.ID] = true
	}
	return out
}

func all(Summary) bool { return true }

// The index follows what the broker saves: a new invocation, its change of
// status, a dismissal, and a record saved through another Broker of the
// same state folder, as the app's store helpers do.
func TestIndexFollowsSaves(t *testing.T) {
	state, ws := t.TempDir(), t.TempDir()
	b := &Broker{Registry: quickRegistry(t), State: state}
	if list, err := b.Attention(all); err != nil || len(list) != 0 {
		t.Fatalf("empty state: %v %v", list, err)
	}
	if _, err := b.Run(context.Background(), ws, "chat", "run", "first", quickRef, `{"value":1}`, "", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	done := func(s Summary) bool { return s.Status == "done" && !s.Dismissed }
	if list, err := b.Attention(done); err != nil || !ids(list)["first"] || list[0].OutputJSON != `{"value":1}` {
		t.Fatalf("a finished invocation is not listed in full: %+v %v", list, err)
	}
	waiting := Invocation{ID: "asks", Workspace: ws, OwnerKind: "chat", OwnerID: "c1", Status: "waiting", Deadline: time.Now().Add(time.Hour), Created: time.Now()}
	if err := (&Broker{State: state}).Save(&waiting); err != nil {
		t.Fatal(err)
	}
	if list, err := b.Attention(func(s Summary) bool { return s.Status == "waiting" }); err != nil || len(list) != 1 || list[0].ID != "asks" {
		t.Fatalf("a record saved through another broker is missing: %+v %v", list, err)
	}
	if err := b.Dismiss(ws, "first"); err != nil {
		t.Fatal(err)
	}
	if list, err := b.Attention(done); err != nil || len(list) != 0 {
		t.Fatalf("a dismissed result is still listed: %+v %v", list, err)
	}
	if list, err := b.Attention(all); err != nil || len(list) != 2 {
		t.Fatalf("the index lost a record: %+v %v", list, err)
	}
}

// writeElsewhere saves an invocation's record as another program does,
// without this process's broker.
func writeElsewhere(t *testing.T, state string, v Invocation) {
	t.Helper()
	b := &Broker{State: state}
	data, _ := json.Marshal(v)
	if err := (layout.Layout{State: state}).Runs(b.StoreRoot(v.Workspace)).Write(v.ID, data); err != nil {
		t.Fatal(err)
	}
}

// Watch brings in what another program saves or removes, in a workspace
// folder that existed and in one made after watching began, and tells Emit.
func TestIndexFollowsAnotherProgram(t *testing.T) {
	state, ws, later := t.TempDir(), t.TempDir(), t.TempDir()
	changed := make(chan struct{}, 16)
	b := &Broker{State: state, Emit: func(event string, _ any) {
		if event == "applet:invocation" {
			changed <- struct{}{}
		}
	}}
	notice := Invocation{ID: "notice", Workspace: ws, OwnerKind: "chat", OwnerID: "r", Action: "dinah.notifications/post@1", Status: "waiting", Version: 1, Deadline: time.Now().Add(time.Hour), Created: time.Now()}
	writeElsewhere(t, state, notice)
	if _, err := b.Attention(all); err != nil {
		t.Fatal(err)
	}
	if err := b.Watch(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	wait := func(what string, ok func([]Invocation) bool) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			list, _ := b.Attention(func(s Summary) bool { return TerminalStatus(s.Status) })
			if ok(list) {
				return
			}
			select {
			case <-changed:
			case <-time.After(100 * time.Millisecond):
			case <-deadline:
				t.Fatalf("%s: %+v", what, list)
			}
		}
	}
	notice.Status, notice.Version = "done", 2
	writeElsewhere(t, state, notice)
	wait("a change another program made", func(list []Invocation) bool { return ids(list)["notice"] })
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("Emit was not told of another program's change")
	}
	other := Invocation{ID: "later", Workspace: later, OwnerKind: "chat", OwnerID: "c", Status: "failed", Version: 1, Created: time.Now()}
	writeElsewhere(t, state, other)
	wait("a record in a new workspace folder", func(list []Invocation) bool { return ids(list)["later"] })
	if err := (layout.Layout{State: state}).Runs(b.StoreRoot(ws)).Remove("notice"); err != nil {
		t.Fatal(err)
	}
	wait("a record another program removed", func(list []Invocation) bool { return !ids(list)["notice"] && ids(list)["later"] })
}

// Prune deletes old finished records, with their artifacts and hold files,
// and keeps what is still running or waiting, what another holder has, and
// what is recent.
func TestPrune(t *testing.T) {
	state, ws := t.TempDir(), t.TempDir()
	b := &Broker{Registry: quickRegistry(t), State: state}
	save := func(v Invocation) {
		t.Helper()
		v.Workspace, v.Created, v.Deadline = ws, time.Now(), time.Now().Add(time.Hour)
		if err := b.Save(&v); err != nil {
			t.Fatal(err)
		}
	}
	save(Invocation{ID: "old", OwnerKind: "chat", OwnerID: "c", Status: "done", Artifacts: []Artifact{{ID: "old_0", InvocationID: "old", Name: "a.svg", MIME: "image/svg+xml", DataBase64: "PHN2Zy8+"}}})
	save(Invocation{ID: "asks", OwnerKind: "chat", OwnerID: "c", Status: "waiting"})
	save(Invocation{ID: "busy", OwnerKind: "chat", OwnerID: "c", Status: "running"})
	save(Invocation{ID: "held", OwnerKind: "chat", OwnerID: "c", Status: "failed"})
	save(Invocation{ID: "finished", OwnerKind: "chat", OwnerID: "d", Status: "done"})
	store := b.store(ws)
	release, err := store.Hold("held")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if n, err := b.Prune(time.Now()); err != nil || n != 0 {
		t.Fatalf("recent records were pruned: %d %v", n, err)
	}
	n, err := b.Prune(time.Now().Add(Retention + time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("pruned %d, %v", n, err)
	}
	list, err := b.Attention(func(s Summary) bool { return TerminalStatus(s.Status) })
	if err != nil {
		t.Fatal(err)
	}
	got := ids(list)
	if got["old"] || got["finished"] || !got["held"] {
		t.Fatalf("pruning kept or removed the wrong records: %v", got)
	}
	for _, id := range []string{"asks", "busy"} {
		if _, err := b.Read(ws, id); err != nil {
			t.Fatalf("%s was removed: %v", id, err)
		}
	}
	for _, id := range []string{"old", "finished"} {
		if store.Exists(id) {
			t.Fatalf("%s is still saved", id)
		}
		if _, err := os.Stat(store.HoldPath(id)); !os.IsNotExist(err) {
			t.Fatalf("%s's hold file is left: %v", id, err)
		}
	}
	if _, err := b.Artifact(ws, "chat", "c", "old_0"); err == nil {
		t.Fatal("a pruned record's artifact is still available")
	}
	entries, err := os.ReadDir(filepath.Join(state, "applets"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("the workspace folder is gone: %v %v", entries, err)
	}
}

// seedNotifications saves count finished invocations of about the size seen
// in a real state folder, most of which Notifications does not show, and a
// question and two posted notifications that it does.
func seedNotifications(tb testing.TB, b *Broker, count int) {
	tb.Helper()
	seed(tb, b, count)
	ws := tb.TempDir()
	for i := range 2 {
		v := Invocation{ID: fmt.Sprintf("posted-%d", i), Workspace: ws, OwnerKind: "chat", OwnerID: "r", Action: "dinah.notifications/post@1", OutputJSON: `{"title":"T","body":"B"}`, Status: "done", Deadline: time.Now().Add(time.Hour), Created: time.Now()}
		if err := b.Save(&v); err != nil {
			tb.Fatal(err)
		}
	}
	q := Invocation{ID: "question", Workspace: ws, OwnerKind: "chat", OwnerID: "c", Action: "dinah.notifications/request@1", OutputJSON: `{}`, Status: "waiting", Deadline: time.Now().Add(time.Hour), Created: time.Now()}
	if err := b.Save(&q); err != nil {
		tb.Fatal(err)
	}
}

func notificationsRule(s Summary) bool {
	return s.Status == "waiting" || (s.Status == "done" && !s.Dismissed && s.Action == "dinah.notifications/post@1")
}

// BenchmarkNotificationsRefresh is one Notifications refresh in a state
// folder of 600 finished invocations: one Attention pass with the app's
// rule, after the index is built. Its first, building pass is reported as
// build-ms.
func BenchmarkNotificationsRefresh(b *testing.B) {
	state := b.TempDir()
	br := &Broker{Registry: quickRegistry(b), State: state}
	seedNotifications(b, br, 600)
	// A fresh process has no index; the first refresh builds it.
	indexes.Delete(stateKey(state))
	start := time.Now()
	list, err := br.Attention(notificationsRule)
	if err != nil || len(list) != 3 {
		b.Fatalf("%d %v", len(list), err)
	}
	build := time.Since(start)
	indexes.Delete(stateKey(state))
	start = time.Now()
	if _, err := br.Attention(notificationsRule); err != nil {
		b.Fatal(err)
	}
	rebuild := time.Since(start)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := br.Attention(notificationsRule); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(build.Microseconds())/1000, "build-ms")
	b.ReportMetric(float64(rebuild.Microseconds())/1000, "rebuild-ms")
}
