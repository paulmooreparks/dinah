package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// calibrationServer is an Ollama with three models, one in its cloud, one local and
// already loaded, and one local and not loaded, that answers every judgement well
// and says which models were asked to answer anything.
type calibrationServer struct {
	mu     sync.Mutex
	called map[string]int
}

func (c *calibrationServer) count(model string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.called[model]
}

func calibrationApp(t *testing.T) (*App, *calibrationServer) {
	t.Helper()
	a, _, _ := judgementApp(t)
	cs := &calibrationServer{called: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"tiny:4b","model":"tiny:4b"},{"name":"big:cloud","model":"big:cloud"},{"name":"loaded:7b","model":"loaded:7b"}]}`)
		case "/api/ps":
			io.WriteString(w, `{"models":[{"name":"loaded:7b","model":"loaded:7b"}]}`)
		case "/api/chat":
			var req struct {
				Model    string `json:"model"`
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			cs.mu.Lock()
			cs.called[req.Model]++
			cs.mu.Unlock()
			input := req.Messages[len(req.Messages)-1].Content
			var in struct {
				Line   string   `json:"line"`
				Lines  []string `json:"lines"`
				Models []string `json:"models"`
				Turns  []struct {
					N    int `json:"n"`
					Cost int `json:"cost"`
				} `json:"turns"`
				Needed int `json:"needed"`
			}
			_ = json.Unmarshal([]byte(input), &in)
			text := "ready"
			switch {
			case in.Models != nil:
				b, _ := json.Marshal(map[string]any{"candidates": in.Models})
				text = string(b)
			case len(in.Turns) > 0:
				// Choose turns that are not the oldest, enough to free what is needed,
				// but never the second turn, which a case records a decision in.
				chosen, freed := []int{}, 0
				for _, t := range in.Turns {
					if t.N == 1 || t.N == 2 || freed >= in.Needed {
						continue
					}
					chosen = append(chosen, t.N)
					freed += t.Cost
				}
				b, _ := json.Marshal(map[string]any{"turns": chosen})
				text = string(b)
			case len(in.Lines) > 0:
				text = in.Lines[0]
			case in.Line != "":
				text = in.Line
				if len(text) > 38 {
					text = text[:38]
				}
			}
			b, _ := json.Marshal(map[string]any{"message": map[string]string{"role": "assistant", "content": text}, "done": true, "prompt_eval_count": 10, "eval_count": 2})
			w.Write(b)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OLLAMA_HOST", srv.URL)
	return a, cs
}

// An automatic run, started when no model is chosen, tries only the models that load
// nothing new, the cloud one and the one already loaded, chooses one of them, says
// it was the program's choice, keeps the report, and does not run again for the same
// models.
func TestAnAutomaticCalibrationNeverLoadsAModelAndChoosesOne(t *testing.T) {
	a, cs := calibrationApp(t)
	a.judging.noAuto = false
	if _, err := a.judges(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the calibration", func() bool {
		s, _, _ := a.judgementSettings()
		return s.Model != "" && a.calibrationState().State == "chosen"
	})
	s, _, _ := a.judgementSettings()
	if !slices.Contains([]string{"big:cloud", "loaded:7b"}, s.Model) || s.ModelSource != "calibration" {
		t.Fatalf("chose %q (%s)", s.Model, s.ModelSource)
	}
	if cs.count("tiny:4b") != 0 || cs.count("big:cloud") == 0 || cs.count("loaded:7b") == 0 {
		t.Fatalf("calls by model: %v", cs.called)
	}
	view, _ := a.JudgementCalibration()
	if view.State != "chosen" || view.At == nil || len(view.Models) != 3 || len(view.Cases) < 6 || view.Running {
		t.Fatalf("view %+v", view)
	}
	for _, m := range view.Models {
		if m.Name == "tiny:4b" && (m.Candidate || m.Loaded) {
			t.Fatalf("the local model that is not loaded was considered: %+v", m)
		}
	}
	st, _ := a.JudgementState()
	if st.Calibration.State != "chosen" || st.Calibration.Chosen != s.Model || st.State != "on" {
		t.Fatalf("state %+v", st)
	}
	// The calls are recorded as calibration, with the model as the id.
	rows, _ := a.JudgementRecords(JudgementFilter{EffectKind: "calibration", EffectID: s.Model, Limit: 500})
	if len(rows.Records) < 7 {
		t.Fatalf("%d calibration records for the chosen model", len(rows.Records))
	}
	// A new window for the same models does not run it again.
	before := cs.count("big:cloud") + cs.count("loaded:7b")
	second := NewApp()
	second.sink = func(string, any) {}
	t.Cleanup(second.closeJudges)
	if _, err := second.judges(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if after := cs.count("big:cloud") + cs.count("loaded:7b"); after != before {
		t.Fatalf("a second run for the same models made %d calls", after-before)
	}
}

// A model the person chose stands: an automatic run does not happen, and a run the
// person starts tries every model, says which it loaded, and changes nothing.
func TestThePersonsModelStandsAndAManualRunMayTryEveryModel(t *testing.T) {
	a, cs := calibrationApp(t)
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	a.judging.noAuto = false
	a.maybeCalibrate()
	if cs.count("big:cloud") != 0 {
		t.Fatalf("an automatic run happened with the person's model chosen: %v", cs.called)
	}
	if err := a.RunJudgementCalibration(); err != nil {
		t.Fatal(err)
	}
	if err := a.RunJudgementCalibration(); err == nil {
		// A second start while one runs is refused; if the run already ended it was
		// allowed, which is not a failure.
		waitFor(t, "the second run", func() bool { return a.calibrationState().State != "running" })
	}
	waitFor(t, "the run", func() bool { return a.calibrationState().State == "chosen" })
	if cs.count("tiny:4b") == 0 {
		t.Fatalf("a run the person started did not try every model: %v", cs.called)
	}
	view, _ := a.JudgementCalibration()
	var tiny, big *bool
	for _, m := range view.Models {
		l := m.Loaded
		switch m.Name {
		case "tiny:4b":
			tiny = &l
		case "big:cloud":
			big = &l
		}
	}
	if tiny == nil || !*tiny || big == nil || *big {
		t.Fatalf("the report does not say which model the run loaded: %+v", view.Models)
	}
	s, _, _ := a.judgementSettings()
	if s.Model != "tiny:4b" || s.ModelSource != "person" {
		t.Fatalf("the person's model was replaced: %q (%s)", s.Model, s.ModelSource)
	}
}

// With no model chosen, a run the person starts chooses one and marks it as the
// program's, and saving a different model makes it the person's.
func TestAManualRunChoosesAModelAndTheSourceFollowsWhoChoseIt(t *testing.T) {
	a, _ := calibrationApp(t)
	if err := a.RunJudgementCalibration(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the choice", func() bool { s, _, _ := a.judgementSettings(); return s.Model != "" })
	s, _, _ := a.judgementSettings()
	if s.ModelSource != "calibration" {
		t.Fatalf("source %q", s.ModelSource)
	}
	snap, _ := a.JudgementSettings()
	// The page cannot set the source: an unchanged model keeps it, whatever is sent.
	snap.Settings.ModelSource = "person"
	saved, err := a.SetJudgementSettings(snap.Settings, snap.Revision)
	if err != nil || saved.Settings.ModelSource != "calibration" {
		t.Fatalf("an unchanged model changed its source: %+v %v", saved.Settings, err)
	}
	other := "tiny:4b"
	if saved.Settings.Model == other {
		other = "big:cloud"
	}
	saved.Settings.Model = other
	saved.Settings.ModelSource = "calibration"
	saved, err = a.SetJudgementSettings(saved.Settings, saved.Revision)
	if err != nil || saved.Settings.ModelSource != "person" {
		t.Fatalf("a model the person saved is not theirs: %+v %v", saved.Settings, err)
	}
	// With the judges off, there is nothing to calibrate.
	choose(t, a, func(s *JudgementSettings) { s.Enabled = false })
	if err := a.RunJudgementCalibration(); err == nil {
		t.Fatal("a calibration was started with the judges off")
	}
}

// A kind cannot be added that calibration would give a judge for without a case
// that tests it.
func TestEveryDeclaredKindHasACalibrationCase(t *testing.T) {
	for _, name := range judgeKindsWithoutCases() {
		if strings.HasPrefix(name, "test-") || strings.HasPrefix(name, "window-test") {
			continue
		}
		t.Errorf("the kind %s has no calibration case", name)
	}
}
