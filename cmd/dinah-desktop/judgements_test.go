package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/judge"
)

type windowTestIn struct {
	Line string `json:"line" doc:"a line of test input"`
}

var windowTestKind = judge.Declare(judge.Kind[windowTestIn, string]{
	Name:        "window-test-kind",
	Describe:    "Uses the line as it is.",
	Correctable: true,
	System:      "You repeat lines.",
	Form:        judge.TextForm(40),
	Deadline:    2 * time.Second,
	Rule:        func(in windowTestIn) string { return in.Line },
})

// judgementApp is an App whose Ollama server is a stand-in that answers every
// judgement with the reply it is given, or fails when failing is set.
func judgementApp(t *testing.T) (a *App, events *[]map[string]any, failing *atomic.Bool) {
	t.Helper()
	a, _ = providerTestApp(t)
	failing = &atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"tiny:4b","model":"tiny:4b"},{"name":"big:cloud","model":"big:cloud"}]}`)
		case "/api/chat":
			if failing.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, `{"error":"the model crashed"}`)
				return
			}
			io.WriteString(w, `{"message":{"role":"assistant","content":"Judged line"},"done":true,"prompt_eval_count":12,"eval_count":3}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OLLAMA_HOST", srv.URL)
	events = &[]map[string]any{}
	a.sink = func(name string, data any) {
		if name != "judgements:changed" {
			return
		}
		raw, _ := json.Marshal(data)
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			*events = append(*events, m)
		}
	}
	return a, events, failing
}

func choose(t *testing.T, a *App, edit func(*JudgementSettings)) JudgementSettingsSnapshot {
	t.Helper()
	snap, err := a.JudgementSettings()
	if err != nil {
		t.Fatal(err)
	}
	edit(&snap.Settings)
	saved, err := a.SetJudgementSettings(snap.Settings, snap.Revision)
	if err != nil || saved.Conflict {
		t.Fatalf("saving %+v: conflict %v, %v", snap.Settings, saved.Conflict, err)
	}
	return saved
}

// Before anything is chosen or judged the settings are the defaults, the widget
// is off for want of a model, and the reads are empty and not null.
func TestTheJudgementsStartOffForWantOfAModel(t *testing.T) {
	a, _, _ := judgementApp(t)
	snap, err := a.JudgementSettings()
	if err != nil {
		t.Fatal(err)
	}
	s := snap.Settings
	if !s.Enabled || s.Model != "" || s.RetentionDays != 90 || s.WarnStreak != 3 || s.Provider == "" || s.KindsOff == nil || snap.Revision == "" || snap.Conflict {
		t.Fatalf("defaults %+v", snap)
	}
	st, err := a.JudgementState()
	if err != nil || st.State != "off" || st.Why != "no model chosen" || st.Calls != 0 || st.WarnStreak != 3 || st.StateGen == "" || st.StateVer == 0 {
		t.Fatalf("state %+v %v", st, err)
	}
	if page, err := a.JudgementRecords(JudgementFilter{}); err != nil || page.Records == nil || len(page.Records) != 0 || page.Next != 0 {
		t.Fatalf("records %+v %v", page, err)
	}
	if sum, err := a.JudgementSummary("", ""); err != nil || sum.Calls != 0 || sum.Groups == nil {
		t.Fatalf("summary %+v %v", sum, err)
	}
	kinds, err := a.JudgementKinds()
	found := false
	for _, k := range kinds {
		if k.Name == "window-test-kind" {
			found = k.Enabled && k.Correctable && k.Describe != "" && len(k.Input) == 1 && k.Input[0].Doc != ""
		}
	}
	if err != nil || !found {
		t.Fatalf("kinds %+v %v", kinds, err)
	}
	// Without a model the call is answered by its rule and recorded.
	set, _ := a.judges()
	if r := judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "Rule line"}, judge.Effect{}); r.Out != "Rule line" || r.Why != judge.WhyNoJudge {
		t.Fatalf("call %+v", r)
	}
}

// The models that could judge come from the provider, and choosing one makes the
// judge answer, record, and tell the page, in numbered events.
func TestAChosenModelJudgesAndEveryChangeIsAnEvent(t *testing.T) {
	a, events, _ := judgementApp(t)
	models, err := a.JudgementModels()
	if err != nil || len(models) != 2 || models[0].Name != "tiny:4b" || models[0].Provider == "" {
		t.Fatalf("models %+v %v", models, err)
	}
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	set, _ := a.judges()
	r := judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "Rule line"}, judge.Effect{Kind: "chat", ID: "c1"})
	if r.Out != "Judged line" || r.Used != judge.UsedJudge {
		t.Fatalf("call %+v", r)
	}
	st, _ := a.JudgementState()
	if st.State != "on" || st.Calls != 1 || st.Fallbacks != 0 || st.Latest == "" {
		t.Fatalf("state %+v", st)
	}
	page, _ := a.JudgementRecords(JudgementFilter{EffectKind: "chat", EffectID: "c1"})
	if len(page.Records) != 1 || page.Records[0].Model != "tiny:4b" || page.Records[0].Seed == 0 || page.Records[0].CostIn != 12 {
		t.Fatalf("records %+v", page.Records)
	}
	// One event for the save and one for the call, each numbered after the last,
	// and the read after them is numbered after both.
	if len(*events) != 2 {
		t.Fatalf("%d events: %v", len(*events), *events)
	}
	v1, v2 := (*events)[0]["stateVer"].(float64), (*events)[1]["stateVer"].(float64)
	if (*events)[0]["stateGen"] == "" || (*events)[0]["stateGen"] != (*events)[1]["stateGen"] || v2 <= v1 || float64(st.StateVer) <= v2 || (*events)[1]["calls"] != float64(1) {
		t.Fatalf("events %v, read %d", *events, st.StateVer)
	}
}

// A judge that keeps failing turns the widget to a warning at the person's
// threshold, and a changed threshold or the off switch changes it at once.
func TestARunOfFallbacksIsAWarningAtThePersonsThreshold(t *testing.T) {
	a, events, failing := judgementApp(t)
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	set, _ := a.judges()
	call := func() judge.Result[string] {
		return judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "x"}, judge.Effect{})
	}
	call()
	failing.Store(true)
	call()
	call()
	if st, _ := a.JudgementState(); st.State != "on" || st.Streak != 2 {
		t.Fatalf("after two failures %+v", st)
	}
	call()
	// Three errors in a row also pause the judge, which is a condition shown as the state line and not as a count; the
	// warnings are the three calls.
	if st, _ := a.JudgementState(); st.State != "warning" || st.Streak != 3 || st.Unseen != 3 || len(st.Warnings) != 3 || st.Paused == nil || st.Why != "Paused: "+st.Paused.Reason || st.Paused.Reason != "ollama: the model crashed" {
		t.Fatalf("after three failures %+v", st)
	}
	before := len(*events)
	// Dismissing every warning clears the count and keeps it cleared, and the state line stays while the judge is paused.
	if err := a.DismissJudgementWarnings(); err != nil {
		t.Fatal(err)
	}
	if st, _ := a.JudgementState(); st.Unseen != 0 || len(st.Warnings) != 0 || st.Paused == nil || st.State != "warning" || len(*events) != before+1 || (*events)[before]["unseen"] != float64(0) {
		t.Fatalf("after dismissing all %+v (%d events)", st, len(*events))
	}
	choose(t, a, func(s *JudgementSettings) { s.WarnStreak = 5 })
	choose(t, a, func(s *JudgementSettings) { s.Enabled = false })
	if st, _ := a.JudgementState(); st.State != "off" || st.Why != "turned off" {
		t.Fatalf("turned off %+v", st)
	}
	failing.Store(false)
	choose(t, a, func(s *JudgementSettings) { s.Enabled = true })
	if r := call(); r.Used != judge.UsedJudge {
		t.Fatalf("after turning on %+v", r)
	}
	if st, _ := a.JudgementState(); st.State != "on" || st.Streak != 0 {
		t.Fatalf("after an answer %+v", st)
	}
	// A kind that is turned off is answered by its rule.
	choose(t, a, func(s *JudgementSettings) { s.KindsOff = []string{"window-test-kind"} })
	if r := call(); r.Used != judge.UsedRule || r.Why != judge.WhyOff {
		t.Fatalf("a kind turned off %+v", r)
	}
	kinds, _ := a.JudgementKinds()
	for _, k := range kinds {
		if k.Name == "window-test-kind" && k.Enabled {
			t.Fatal("the kind still shows as on")
		}
	}
}

// Settings are saved against a revision: a stale save writes nothing and returns
// what is there, a bad value is refused, and the choices outlast a restart.
func TestJudgementSettingsAreSavedAgainstARevision(t *testing.T) {
	a, _, _ := judgementApp(t)
	first, _ := a.JudgementSettings()
	saved := choose(t, a, func(s *JudgementSettings) { s.Model = "big:cloud"; s.RetentionDays = 30 })
	if saved.Revision == first.Revision {
		t.Fatal("the revision did not change")
	}
	stale := first.Settings
	stale.Model = "tiny:4b"
	got, err := a.SetJudgementSettings(stale, first.Revision)
	if err != nil || !got.Conflict || got.Settings.Model != "big:cloud" || got.Revision != saved.Revision {
		t.Fatalf("stale save %+v %v", got, err)
	}
	if now, _ := a.JudgementSettings(); now.Settings.Model != "big:cloud" {
		t.Fatalf("a stale save wrote: %+v", now)
	}
	for name, edit := range map[string]func(*JudgementSettings){
		"no retention": func(s *JudgementSettings) { s.RetentionDays = 0 },
		"no threshold": func(s *JudgementSettings) { s.WarnStreak = 0 },
		"unknown kind": func(s *JudgementSettings) { s.KindsOff = []string{"nothing"} },
		"cannot judge": func(s *JudgementSettings) { s.Provider = "devin" },
	} {
		bad := saved.Settings
		edit(&bad)
		if _, err := a.SetJudgementSettings(bad, saved.Revision); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if now, _ := a.JudgementSettings(); now.Settings.RetentionDays != 30 {
		t.Fatalf("settings after refusals %+v", now)
	}
	// A new window reads the same choices from the file.
	second := NewApp()
	second.sink = func(string, any) {}
	second.judging.noAuto = true
	t.Cleanup(func() {
		if second.judging.set != nil { // closed by closeJudges below
			second.judging.set.Store().Close()
		}
	})
	again, err := second.JudgementSettings()
	if err != nil || again.Settings.Model != "big:cloud" || again.Settings.RetentionDays != 30 || again.Revision != saved.Revision {
		t.Fatalf("after a restart %+v %v", again, err)
	}
}

// A correction is kept with the record and tells the page.
func TestACorrectionIsKeptAndTellsThePage(t *testing.T) {
	a, events, _ := judgementApp(t)
	set, _ := a.judges()
	r := judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "Rule line"}, judge.Effect{Kind: "chat", ID: "c1"})
	before := len(*events)
	rec, err := a.CorrectJudgement(r.ID, `"My line"`)
	if err != nil || rec.Correction != `"My line"` || rec.CorrectedAt == nil || len(*events) != before+1 {
		t.Fatalf("%+v %v (%d events)", rec, err, len(*events))
	}
	if _, err := a.CorrectJudgement(9999, "x"); err == nil {
		t.Fatal("a decision that does not exist was corrected")
	}
}
