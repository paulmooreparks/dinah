package resident_test

import (
	"path/filepath"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// lapseDefinition is a workbench with a column that takes work up, so a
// card there can be claimed with an expiry.
const lapseDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Lapse",
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Build", "kind": "work" },
    { "id": "c00000000003", "title": "Finished", "kind": "done" }
  ]
}`

// TestALapseIsDueAtTheEarliestExpiry is part of dinah-619/criteria/10. With
// two claimed cards expiring at T1 before T2, Current answers the snapshot an
// instant before T1, and at T1 answers none, naming the first card's
// directory alone as lapsing.
func TestALapseIsDueAtTheEarliestExpiry(t *testing.T) {
	root := filepath.Join(t.TempDir(), bench.UserBaseName, "0199a1b2c3d47abc8000000000000619")
	definition, err := bench.ReadDefinition([]byte(lapseDefinition))
	if err != nil {
		t.Fatal(err)
	}
	if err := bench.Instantiate(root, "lp", "alka", definition); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	claimed := func(title string, expires time.Duration) *bench.Card {
		opened, err := bench.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		library := verb.New(opened, home)
		added := library.Add(&verb.Request{Verb: "add", Actor: "alka", Title: title, Column: "build"})
		if added.Card == nil {
			t.Fatalf("add %s: %+v", title, added)
		}
		opened, _ = bench.Open(root)
		library = verb.New(opened, home)
		if claim := library.Do(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: added.Card.Ref, Expires: expires}); claim.Outcome != "ok" {
			t.Fatalf("claim %s: %+v", title, claim)
		}
		opened, _ = bench.Open(root)
		resolved, err := opened.ResolveCard(added.Card.Ref)
		if err != nil {
			t.Fatal(err)
		}
		return resolved.Card
	}
	first := claimed("first", time.Hour)
	second := claimed("second", 2*time.Hour)
	t1, t2 := bench.ParseStamp(first.Expires), bench.ParseStamp(second.Expires)
	if t1.IsZero() || !t1.Before(t2) {
		t.Fatalf("the expiries are %s and %s, wanted the first before the second", first.Expires, second.Expires)
	}
	v := watch(t, root)
	if pick := v.w.Current(t1.Add(-time.Nanosecond)); pick.Snapshot == nil {
		t.Errorf("an instant before the first expiry Current answered no snapshot, lapsing %v", pick.Lapsing)
	}
	pick := v.w.Current(t1)
	if pick.Snapshot != nil {
		t.Error("at the first expiry Current answered a snapshot")
	}
	if len(pick.Lapsing) != 1 || pick.Lapsing[0] != first.Dir {
		t.Errorf("at the first expiry Current named %v as lapsing, wanted %s alone", pick.Lapsing, first.Dir)
	}
}
