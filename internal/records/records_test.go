package records

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func payload(id, status string) []byte {
	b, _ := json.Marshal(map[string]string{"id": id, "status": status})
	return b
}

// Remove deletes both generations and refuses a record it cannot read.
func TestRemove(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	for _, st := range []string{"running", "done", "done"} {
		if err := s.Write("r", payload("r", st)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Remove("r"); err != nil || s.Exists("r") {
		t.Fatalf("the record is still saved: %v", err)
	}
	if err := s.Remove("never"); err != nil {
		t.Fatalf("removing a record that is not saved: %v", err)
	}
	if err := os.WriteFile(s.SlotPath("bad", 0), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("bad"); err == nil || !s.Exists("bad") {
		t.Fatal("a damaged record was removed unseen")
	}
}

func status(t *testing.T, s Store, id string) string {
	t.Helper()
	data, err := s.Read(id)
	if err != nil {
		t.Fatal(err)
	}
	var r struct{ Status string }
	json.Unmarshal(data, &r)
	return r.Status
}

func TestSelectsLatestCompleteGeneration(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	for _, st := range []string{"first", "second", "third"} {
		if err := s.Write("run", payload("run", st)); err != nil {
			t.Fatal(err)
		}
	}
	if got := status(t, s, "run"); got != "third" {
		t.Fatalf("got %s", got)
	}
	// The third save used slot zero; an incomplete newest write leaves slot one readable.
	os.WriteFile(s.SlotPath("run", 0), []byte(`{"sequence":3,"payload":`), 0600)
	if got := status(t, s, "run"); got != "second" {
		t.Fatalf("a torn write returned %s", got)
	}
	if err := s.Write("run", payload("run", "recovered")); err != nil || status(t, s, "run") != "recovered" {
		t.Fatalf("recovery: %v", err)
	}
}

func TestChecksSequencePayloadAndChecksum(t *testing.T) {
	for _, corruption := range []string{"payload", "sequence", "id"} {
		t.Run(corruption, func(t *testing.T) {
			s := Store{Dir: t.TempDir()}
			s.Write("run", payload("run", "original"))
			data, _ := os.ReadFile(s.SlotPath("run", 0))
			switch corruption {
			case "payload":
				data = []byte(strings.Replace(string(data), "original", "corrupt", 1))
			case "sequence":
				data = []byte(strings.Replace(string(data), `"sequence":1`, `"sequence":2`, 1))
			case "id":
				data = Generation(1, payload("different", "original"))
			}
			os.WriteFile(s.SlotPath("run", 0), data, 0600)
			if _, err := s.Read("run"); err == nil {
				t.Fatal("a corrupt generation was accepted")
			}
			if err := s.Write("run", payload("run", "replace")); err == nil {
				t.Fatal("corrupt history was silently replaced")
			}
		})
	}
}

func TestLegacyOnlyWithoutGenerations(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	legacy := payload("run", "legacy")
	os.WriteFile(filepath.Join(s.Dir, "run.json"), legacy, 0600)
	if data, err := s.Read("run"); err != nil || string(data) != string(legacy) {
		t.Fatalf("legacy %s: %v", data, err)
	}
	s.Write("run", payload("run", "current"))
	if status(t, s, "run") != "current" {
		t.Fatal("the generation did not win over the legacy record")
	}
	os.WriteFile(s.SlotPath("run", 0), []byte("torn"), 0600)
	if _, err := s.Read("run"); err == nil {
		t.Fatal("a stale legacy record was used after the generation was damaged")
	}
	if !s.Exists("run") || s.Exists("absent") {
		t.Fatal("a damaged record was mistaken for an absent one, or the reverse")
	}
}

func TestFailedReplacementKeepsThePrevious(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	s.Write("run", payload("run", "saved"))
	os.Mkdir(s.SlotPath("run", 1), 0700)
	if err := s.Write("run", payload("run", "new")); err == nil {
		t.Fatal("an invalid target was accepted")
	}
	data, _ := os.ReadFile(s.SlotPath("run", 0))
	if p, err := Payload(data); err != nil || !strings.Contains(string(p), "saved") {
		t.Fatalf("the previous generation changed: %s %v", p, err)
	}
}

func TestUnknownVersionIsNotDowngraded(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	s.Write("run", payload("run", "saved"))
	s.Write("run", payload("run", "saved"))
	data, _ := os.ReadFile(s.SlotPath("run", 1))
	os.WriteFile(s.SlotPath("run", 1), []byte(strings.Replace(string(data), `"version":1`, `"version":2`, 1)), 0600)
	if _, err := s.Read("run"); err == nil {
		t.Fatal("an unknown version was downgraded to the older slot")
	}
	if err := s.Write("run", payload("run", "overwrite")); err == nil {
		t.Fatal("an unknown version was overwritten")
	}
}

// Saves of one record in this process take turns, so each gets its own
// generation, even when the folder is spelled two ways; saves of other
// records do not wait for them.
func TestConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	spellings := []Store{{Dir: dir}, {Dir: dir + string(filepath.Separator) + "."}}
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := spellings[i%2]
			if err := s.Write("shared", payload("shared", "saved")); err != nil {
				t.Error(err)
			}
			id := fmt.Sprintf("own-%d", i)
			if err := s.Write(id, payload(id, "saved")); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	unlock := spellings[0].lock("shared")
	st, err := spellings[0].read("shared")
	unlock()
	if err != nil || st.latest == nil || st.latest.Sequence != 40 {
		t.Fatalf("the shared record ended at %+v %v; want generation 40", st.latest, err)
	}
	held := spellings[0].lock("held")
	defer held()
	done := make(chan error, 1)
	go func() { done <- spellings[0].Write("other", payload("other", "saved")) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a save waited on another record's lock")
	}
}

func TestID(t *testing.T) {
	for name, want := range map[string]string{"a.run0.json": "a", "a.run1.json": "a", "a.json": "a", "a.txt": ""} {
		if got := ID(name); got != want {
			t.Errorf("%s: %q", name, got)
		}
	}
}
