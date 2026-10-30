package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"studio/internal/judge"
	"studio/internal/providers"
	"studio/internal/statefile"
)

// Calibration chooses the judge's model by measurement (docs/plans/
// judgement-agents.md, step 5). It runs in the background, never interrupts, and
// never replaces a model the person chose. An automatic run, started when no model
// is chosen and again when the models installed change, considers only models that
// load nothing new into memory: the ones that run in a provider's cloud and the
// ones the provider already has loaded. A run the person starts may try any
// installed model, and its report says which it loaded.

// The fixed set of calls a model is scored by, for the kinds the window declares.
// Each answer must also pass its kind's own check. The keyword tests are coarse,
// and cheap, deterministic and reviewable.
func init() {
	titleWord := func(words ...string) func(string) bool {
		return func(out string) bool {
			out = strings.ToLower(out)
			for _, w := range words {
				if strings.Contains(out, w) {
					return true
				}
			}
			return false
		}
	}
	judge.DeclareCase(chatTitleKind, "a rename", chatTitleIn{Line: "Please rename the parser's tokenizer module to lexer and update the imports"},
		"a title that contains lexer or tokenizer", titleWord("lexer", "tokenizer"))
	judge.DeclareCase(chatTitleKind, "a failing build", chatTitleIn{Line: "Why does the nightly build fail on the Windows runner only?"},
		"a title that contains nightly, build or Windows", titleWord("nightly", "build", "windows"))
	judge.DeclareCase(chatTitleKind, "a haiku", chatTitleIn{Line: "Write a haiku about debugging at 3 a.m."},
		"a title that contains haiku or debug", titleWord("haiku", "debug"))
	judge.DeclareCase(documentNameKind, "a cost summary", documentNameIn{Line: "Quarterly maintenance cost summary for the Lisbon office"},
		"a valid file name that contains maintenance, cost or Lisbon", titleWord("maintenance", "cost", "lisbon"))
	judge.DeclareCase(documentNameKind, "a checklist", documentNameIn{Line: "# Release checklist for version 2.4"},
		"a valid file name that contains release or checklist", titleWord("release", "checklist"))
}

// calibrationFile is where the last run's report is kept, with the key of the
// models it ran for.
func calibrationPath() string { return filepath.Join(configDir(), "calibration.json") }

type calibrationRecord struct {
	Report judge.Report `json:"report"`
	// Key says which installed models the report is for, so an automatic run
	// happens once for each set of them.
	Key string `json:"key"`
}

// JudgementCalibrationState is what the widget's state says of calibration: idle
// before it has run, running, and then what it found. Chosen is the model it chose,
// or empty.
type JudgementCalibrationState struct {
	State  string     `json:"state"`
	At     *time.Time `json:"at,omitempty"`
	Chosen string     `json:"chosen"`
}

// JudgementCalibrationView is the last run's report and the fixed set it scored by.
type JudgementCalibrationView struct {
	At      *time.Time          `json:"at,omitempty"`
	State   string              `json:"state"`
	Note    string              `json:"note"`
	Running bool                `json:"running"`
	Models  []judge.ModelReport `json:"models"`
	Cases   []judge.CaseInfo    `json:"cases"`
}

func (a *App) calibrationState() JudgementCalibrationState {
	j := &a.judging
	j.mu.Lock()
	defer j.mu.Unlock()
	st := JudgementCalibrationState{State: "idle"}
	if j.report != nil {
		at := j.report.At
		st.State, st.At, st.Chosen = j.report.State, &at, j.report.Chosen
	}
	if j.calibrating {
		st.State = "running"
	}
	return st
}

func (a *App) loadCalibration() {
	data, err := statefile.Read(calibrationPath())
	if err != nil {
		return
	}
	var rec calibrationRecord
	if json.Unmarshal(data, &rec) != nil {
		return
	}
	j := &a.judging
	j.report, j.calibKey = &rec.Report, rec.Key
}

// JudgementCalibration returns the last run's report and the fixed set of calls a
// model is scored by.
func (a *App) JudgementCalibration() (JudgementCalibrationView, error) {
	if _, err := a.judges(); err != nil {
		return JudgementCalibrationView{}, err
	}
	j := &a.judging
	j.mu.Lock()
	defer j.mu.Unlock()
	v := JudgementCalibrationView{State: "idle", Running: j.calibrating, Models: []judge.ModelReport{}, Cases: judge.Cases()}
	if j.report != nil {
		at := j.report.At
		v.At, v.State, v.Note = &at, j.report.State, j.report.Note
		if j.report.Models != nil {
			v.Models = j.report.Models
		}
	}
	return v, nil
}

// RunJudgementCalibration starts a run that may try any installed model, and
// returns at once. The report says which models the run loaded. A model the person
// chose is not replaced, and the report then only offers a better one.
func (a *App) RunJudgementCalibration() error {
	if _, err := a.judges(); err != nil {
		return err
	}
	settings, _, err := a.judgementSettings()
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return errors.New("the judges are turned off, so there is nothing to calibrate")
	}
	if !a.beginCalibration() {
		return errors.New("a calibration is already running")
	}
	go a.runCalibration(false)
	return nil
}

func (a *App) beginCalibration() bool {
	j := &a.judging
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.calibrating || j.set == nil {
		return false
	}
	j.calibrating = true
	return true
}

// modelsKey says which models are installed and which of them run without loading
// anything, so an automatic run happens once for each such set.
func modelsKey(provider string, models []judge.Model) string {
	parts := make([]string, 0, len(models))
	for _, m := range models {
		parts = append(parts, fmt.Sprintf("%s=%v", m.Name, m.Resident))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(provider + "\n" + strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// maybeCalibrate starts the automatic run when it is due: the judges are on, no
// model is chosen and the person has not chosen one, and the models that run
// without loading anything are not the set a run has already been made for.
func (a *App) maybeCalibrate() {
	if a.judging.noAuto {
		return
	}
	settings, _, err := a.judgementSettings()
	if err != nil || !settings.Enabled || settings.Model != "" || settings.ModelSource == "person" {
		return
	}
	if !a.beginCalibration() {
		return
	}
	a.runCalibration(true)
}

// runCalibration lists the models, runs the calibration, keeps the report and, when
// the model is the program's to choose, applies the one chosen. automatic runs
// consider only models that are resident.
func (a *App) runCalibration(automatic bool) {
	j := &a.judging
	defer func() {
		j.mu.Lock()
		j.calibrating = false
		j.mu.Unlock()
		a.emitJudgementsChanged()
	}()
	a.emitJudgementsChanged()
	settings, _, err := a.judgementSettings()
	if err != nil {
		return
	}
	module, ok := providerModules[settings.Provider]
	if !ok || module.Judge == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	j.mu.Lock()
	j.calibCancel = cancel
	set := j.set
	j.mu.Unlock()
	defer cancel()
	if set == nil {
		return
	}
	listCtx, listCancel := context.WithTimeout(ctx, 10*time.Second)
	installed, err := module.Judge.Installed(a, listCtx, settings.Provider)
	listCancel()
	if err != nil {
		logf("judge", "warning", "the models could not be listed for calibration: %v", err)
		return
	}
	key := modelsKey(settings.Provider, installed)
	models := installed
	if automatic {
		j.mu.Lock()
		done := j.calibKey == key
		j.mu.Unlock()
		if done {
			return
		}
		models = slices.DeleteFunc(slices.Clone(installed), func(m judge.Model) bool { return !m.Resident })
	}
	unit := providers.Registry[settings.Provider].BudgetUnit
	var report judge.Report
	if len(models) == 0 {
		report = judge.Report{At: time.Now(), Provider: settings.Provider, State: "none", Models: []judge.ModelReport{},
			Note: "No model runs without loading anything into memory, and an automatic run does not load one. Start a calibration to try the others."}
	} else {
		report = judge.Calibrate(ctx, set, settings.Provider, unit, models, func(model string) judge.Judge { return module.Judge.Build(a, settings.Provider, model) })
	}
	if automatic {
		// The models left out are in the report too, with the reason.
		for _, m := range installed {
			if !m.Resident {
				report.Models = append(report.Models, judge.ModelReport{Name: m.Name, Why: "not tried: an automatic run does not load a model into memory"})
			}
		}
	}
	if ctx.Err() != nil {
		return
	}
	j.mu.Lock()
	j.report = &report
	if automatic || report.State != "stopped" {
		j.calibKey = key
	}
	j.mu.Unlock()
	if data, err := json.MarshalIndent(calibrationRecord{Report: report, Key: key}, "", "  "); err == nil {
		if err := statefile.Write(calibrationPath(), data); err != nil {
			logf("judge", "warning", "the calibration report could not be kept: %v", err)
		}
	}
	if report.State == "chosen" {
		a.applyCalibrated(report.Chosen, automatic)
	}
}

// applyCalibrated makes the model calibration chose the judge, unless the person has
// chosen one, which stands until they change it. An automatic run also leaves a
// model that was chosen while it ran.
func (a *App) applyCalibrated(model string, automatic bool) {
	set, err := a.judges()
	if err != nil {
		return
	}
	var applied JudgementSettings
	changed := false
	err = statefile.Update(judgementsPath(), func(previous []byte) ([]byte, error) {
		fresh, err := decodeJudgementSettings(previous)
		if err != nil {
			return nil, err
		}
		if fresh.ModelSource == "person" || (fresh.Model != "" && (automatic || fresh.ModelSource != "calibration")) {
			return nil, nil
		}
		fresh.Model, fresh.ModelSource = model, "calibration"
		data, err := json.MarshalIndent(fresh, "", "  ")
		if err != nil {
			return nil, err
		}
		applied, changed = fresh, true
		return data, nil
	})
	if err != nil || !changed {
		if err != nil {
			logf("judge", "warning", "the model calibration chose could not be kept: %v", err)
		}
		return
	}
	data, _ := json.MarshalIndent(applied, "", "  ")
	j := &a.judging
	j.mu.Lock()
	j.cur, j.rev = applied, judgementRevision(data)
	j.mu.Unlock()
	a.applyJudgementSettings(set, applied)
	a.forgetHandoffs()
	a.emitJudgementsChanged()
}

// stopCalibration ends a run that is going on.
func (a *App) stopCalibration() {
	j := &a.judging
	j.mu.Lock()
	cancel := j.calibCancel
	j.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// judgeKindsWithoutCases lists the declared kinds the fixed set does not test.
func judgeKindsWithoutCases() []string { return judge.KindsWithoutCases() }
