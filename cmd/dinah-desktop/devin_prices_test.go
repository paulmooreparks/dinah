package main

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/agents"
)

// pricesFixture is an App whose Devin catalog offers three models, and a
// price reader that counts its reads and never starts Devin.
func pricesFixture(t *testing.T, read func(string) (map[string]agents.DevinModelPrice, error)) (*App, func() int, chan map[string]agents.DevinModelPrice) {
	t.Helper()
	saved := readDevinPrices
	t.Cleanup(func() { readDevinPrices = saved })
	var mu sync.Mutex
	reads := 0
	readDevinPrices = func(path string) (map[string]agents.DevinModelPrice, error) {
		mu.Lock()
		reads++
		mu.Unlock()
		return read(path)
	}
	a := &App{}
	a.settings.Catalog = []acp.ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"adaptive"`),
		Options: json.RawMessage(`[{"value":"adaptive","name":"Adaptive"},{"value":"swe-2-high","name":"SWE-2 High"},{"value":"claude-opus-5-5-medium","name":"Claude Opus 5.5 Medium"}]`)}}
	events := make(chan map[string]agents.DevinModelPrice, 8)
	a.sink = func(name string, data any) {
		if name == "models:prices" {
			events <- data.(map[string]any)["prices"].(map[string]agents.DevinModelPrice)
		}
	}
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return reads
	}
	return a, count, events
}

func nextPrices(t *testing.T, events chan map[string]agents.DevinModelPrice) map[string]agents.DevinModelPrice {
	t.Helper()
	select {
	case p := <-events:
		return p
	case <-time.After(testWait):
		t.Fatal("no prices arrived")
		return nil
	}
}

func noMorePrices(t *testing.T, events chan map[string]agents.DevinModelPrice) {
	t.Helper()
	select {
	case p := <-events:
		t.Fatalf("prices arrived again: %+v", p)
	case <-time.After(100 * time.Millisecond):
	}
}

// Prices are read in the background when Devin starts, matched to the
// models the catalog offers, and read again only when Devin's executable or
// version changes or when the list is opened after an hour.
func TestDevinPricesAreReadOncePerDevin(t *testing.T) {
	listed := map[string]agents.DevinModelPrice{
		"swe-2-high":   {CostSummary: "$0.5 / MTok In · $2.5 / MTok Out", CostTier: "High cost", ContextTokens: 262000},
		"not-offered":  {CostSummary: "$9 / MTok In"},
		"swe-2-medium": {CostSummary: "$0.5 / MTok In"},
	}
	a, reads, events := pricesFixture(t, func(string) (map[string]agents.DevinModelPrice, error) { return listed, nil })
	if got := a.ModelPrices("codex"); len(got) != 0 {
		t.Fatalf("Codex got prices %+v", got)
	}
	a.refreshDevinPrices(`C:\devin.exe`, "3000.4.25")
	got := nextPrices(t, events)
	if len(got) != 1 || got["swe-2-high"].CostTier != "High cost" {
		t.Fatalf("matched %+v", got)
	}
	if now := a.ModelPrices("devin"); len(now) != 1 {
		t.Fatalf("ModelPrices %+v", now)
	}
	a.refreshDevinPrices(`C:\devin.exe`, "3000.4.25")
	noMorePrices(t, events)
	if reads() != 1 {
		t.Fatalf("read %d times", reads())
	}
	// Another version reads again; its prices are the same, so nothing is
	// sent to the page.
	a.refreshDevinPrices(`C:\devin.exe`, "3000.5.0")
	noMorePrices(t, events)
	if reads() != 2 {
		t.Fatalf("read %d times after a new version", reads())
	}
	// After an hour, opening the list reads again.
	a.devinPrices.mu.Lock()
	a.devinPrices.at = time.Now().Add(-2 * time.Hour)
	a.devinPrices.mu.Unlock()
	a.ModelPrices("devin")
	deadline := time.Now().Add(testWait)
	for reads() != 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if reads() != 3 {
		t.Fatalf("read %d times after an hour", reads())
	}
}

// A read that fails leaves no prices at all, so the model list looks as it
// did before prices existed, and prices read before are taken away.
func TestDevinPricesFailQuietly(t *testing.T) {
	fail := false
	a, _, events := pricesFixture(t, func(string) (map[string]agents.DevinModelPrice, error) {
		if fail {
			return nil, errors.New("the Devin command line did not list its models in time")
		}
		return map[string]agents.DevinModelPrice{"adaptive": {CostSummary: "$1 / MTok In"}}, nil
	})
	a.refreshDevinPrices(`C:\devin.exe`, "1")
	if got := nextPrices(t, events); len(got) != 1 {
		t.Fatalf("matched %+v", got)
	}
	fail = true
	a.refreshDevinPrices(`C:\devin.exe`, "2")
	if got := nextPrices(t, events); len(got) != 0 {
		t.Fatalf("after a failure %+v", got)
	}
	if got := a.ModelPrices("devin"); got == nil || len(got) != 0 {
		t.Fatalf("ModelPrices after a failure %+v", got)
	}
}
