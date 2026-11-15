package main

import (
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/agents"
)

func TestDevinPricesWaitForRequestAndUseInitializedExecutable(t *testing.T) {
	a, reads, events := pricesFixture(t, func(path string) (map[string]agents.DevinModelPrice, error) {
		if path != "invented-current-devin.exe" {
			t.Errorf("prices requested from stale executable %q", path)
		}
		return map[string]agents.DevinModelPrice{"swe-2-high": {CostSummary: "Invented price"}}, nil
	})
	conn := newTestProviderConnection()
	t.Cleanup(conn.Close)
	init := acp.InitializeResult{}
	init.AgentInfo = &acp.Implementation{Name: "Invented Devin", Version: "current-version"}
	a.runtime.PutProviderLocked("devin", newTestProvider(conn, init))
	a.devinPrices.path, a.devinPrices.version, a.devinPrices.at = "invented-old-devin.exe", "old-version", time.Now()
	a.noteDevinPriceSource("invented-current-devin.exe")
	noMorePrices(t, events)
	if reads() != 0 || len(conn.calls) != 0 {
		t.Fatal("recording initialized provider identity fetched prices")
	}
	_ = a.ModelPrices("devin")
	prices := nextPrices(t, events)
	if reads() != 1 || prices["swe-2-high"].CostSummary != "Invented price" {
		t.Fatalf("explicit request lost open-chat price behavior: %+v", prices)
	}
}
