package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/judge"
)

func TestEnabledJudgementStateReadsStayPassiveUntilFirstRealCall(t *testing.T) {
	a, _ := providerTestApp(t)
	a.sink = func(string, any) {}
	a.judging.noAuto = false
	var requests atomic.Int32
	inventory := make(chan struct{}, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/api/tags", "/api/ps":
			io.WriteString(w, `{"models":[]}`)
			inventory <- struct{}{}
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OLLAMA_HOST", srv.URL)
	if _, err := a.JudgementState(); err != nil {
		t.Fatal(err)
	}
	settings, err := a.JudgementSettings()
	if err != nil || !settings.Settings.Enabled {
		t.Fatalf("fixture is not enabled: %+v %v", settings, err)
	}
	if _, err := a.JudgementCalibration(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.judges(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-inventory:
		t.Fatal("passive state or store read launched calibration inventory")
	case <-time.After(100 * time.Millisecond):
	}
	if requests.Load() != 0 || a.calibrationState().State == "running" {
		t.Fatal("unused enabled judging contacted Ollama")
	}
	a.judgeChatTitle("invented-trigger", "An actual chat needs a title")
	select {
	case <-inventory:
	case <-time.After(testWait):
		t.Fatal("the first real judgement did not trigger calibration")
	}
	waitFor(t, "requested calibration to finish", func() bool { return a.calibrationState().State == "none" })
	rows, err := a.JudgementRecords(JudgementFilter{EffectKind: "chat", EffectID: "invented-trigger", Limit: 10})
	if err != nil || len(rows.Records) != 1 {
		t.Fatalf("the first judgement lost its existing rule result: %+v %v", rows, err)
	}
}

func TestVisibleJudgementInventoryCanTriggerDueCalibration(t *testing.T) {
	a, _ := providerTestApp(t)
	a.sink = func(string, any) {}
	a.judging.noAuto = false
	previous := providerModules
	providerModules = map[string]providerModule{}
	var inventories atomic.Int32
	providerModules["ollama"] = providerModule{Judge: &judgeModule{
		Build: func(*App, string, string) judge.Judge { return nil },
		Installed: func(*App, context.Context, string) ([]judge.Model, error) {
			inventories.Add(1)
			return []judge.Model{}, nil
		},
	}}
	t.Cleanup(func() { providerModules = previous })
	if _, err := a.JudgementState(); err != nil || inventories.Load() != 0 {
		t.Fatalf("state read started inventory: %v", err)
	}
	if _, err := a.JudgementModels(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "visible-settings calibration to finish", func() bool { return a.calibrationState().State == "none" })
	if inventories.Load() != 2 {
		t.Fatalf("expected explicit picker inventory and due calibration, got %d", inventories.Load())
	}
}
