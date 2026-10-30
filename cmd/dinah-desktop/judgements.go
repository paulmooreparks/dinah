package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"studio/internal/judge"
	"studio/internal/providers"
	"studio/internal/sessionhost"
	"studio/internal/statefile"
)

// The window's side of the judgement calls (docs/plans/judgement-agents.md): the
// settings the person chooses, the set of judges they apply to, and the
// methods the page reads the record and the state through. The calls
// themselves are made by the code that needs a judgement, through judges().

// JudgementSettings are the person's choices for the judges, kept in
// judgements.json. A provider's own name is never written here: Provider is the
// ID of a provider module that can judge (providerModule.Judge), and empty
// means the first one.
type JudgementSettings struct {
	// Enabled is the off switch. Off, every call is answered by its rule.
	Enabled bool `json:"enabled"`
	// Provider and Model say who judges. An empty Model means no judge is
	// chosen, and every call uses its rule until the person or calibration
	// chooses one.
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// ModelSource says who chose the model: "person", or "calibration", or empty
	// when none is chosen. The engine sets it and the page does not: saving a
	// different model makes it the person's, and a model the person chose stands
	// until they change it (judge_calibration.go).
	ModelSource string `json:"modelSource"`
	// RetentionDays is how long records are kept.
	RetentionDays int `json:"retentionDays"`
	// WarnStreak is how many calls in a row must fall back to the rule before the
	// widget warns.
	WarnStreak int `json:"warnStreak"`
	// KindsOff are the kinds of call turned off, by name. As the page reads them
	// they are every kind that is off, including a kind that is off until the
	// person turns it on; as they are kept, only the kinds the person turned off, and
	// KindsOn are the kinds that start off which the person turned on.
	KindsOff []string `json:"kindsOff"`
	KindsOn  []string `json:"kindsOn"`
	// Budgets are what each provider's judgement calls may cost in a period, one
	// row for each provider that can judge. Period is when the weekly period
	// begins, and Pool how many calls may be with the judge at once.
	Budgets []JudgementBudget `json:"budgets"`
	Period  JudgementPeriod   `json:"period"`
	Pool    JudgementPool     `json:"pool"`
}

// JudgementBudget is a provider's budget. Unit is the provider's own, read from
// its row in the provider table and never written by the page; a nil Limit is no
// budget, which is the default, and 0 is a budget that refuses every call.
type JudgementBudget struct {
	Provider string `json:"provider"`
	Unit     string `json:"unit"`
	Limit    *int   `json:"limit"`
}

// JudgementPeriod is when the weekly period begins, in the computer's local time:
// the weekday, 0 for Sunday, and the hour.
type JudgementPeriod struct {
	Weekday int `json:"weekday"`
	Hour    int `json:"hour"`
}

// JudgementPool is the smallest and largest number of judge calls in flight at
// once. The largest has no ceiling of its own.
type JudgementPool struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

const (
	defaultJudgementRetentionDays = 90
	defaultJudgementWarnStreak    = 3
)

// JudgementSettingsSnapshot is the settings as they are now, with their
// revision. A save made against another revision writes nothing and comes back
// with Conflict set and the settings as they are.
type JudgementSettingsSnapshot struct {
	Settings JudgementSettings `json:"settings"`
	Revision string            `json:"revision"`
	Conflict bool              `json:"conflict"`
}

// JudgementState is what the widget shows. Its counts are today's, from local
// midnight, and are not all-time. StateGen and StateVer number it like the
// window's other state (internal/sessionhost/state_events.go).
type JudgementState struct {
	// State is "on", "off" or "warning".
	State      string `json:"state"`
	Why        string `json:"why"`
	Calls      int    `json:"calls"`
	Fallbacks  int    `json:"fallbacks"`
	Streak     int    `json:"streak"`
	WarnStreak int    `json:"warnStreak"`
	Latest     string `json:"latest"`
	// Budget has a row for each provider that has a limit, and Pool is the pool
	// as it is now, with the two fractions it grows and shrinks by.
	Budget []judge.BudgetUse `json:"budget"`
	Pool   judge.PoolState   `json:"pool"`
	// Calibration is where the choice of the judge's model stands.
	Calibration JudgementCalibrationState `json:"calibration"`
	StateGen    string                    `json:"stateGen"`
	StateVer    uint64                    `json:"stateVer"`
}

// JudgementKind is a declared kind of call and whether it is on.
type JudgementKind struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Describe    string `json:"describe"`
	Correctable bool   `json:"correctable"`
	DeadlineMs  int64  `json:"deadlineMs"`
	Enabled     bool   `json:"enabled"`
	// InputCap is the most bytes of input a call of the kind may send, or 0, and
	// DefaultOff says the kind starts turned off.
	InputCap   int           `json:"inputCap"`
	DefaultOff bool          `json:"defaultOff"`
	Input      []judge.Field `json:"input"`
}

// JudgementModel is a model that could judge, for the picker.
type JudgementModel struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// JudgementFilter narrows the list of records. Times are RFC 3339.
type JudgementFilter struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Used       string `json:"used"`
	Why        string `json:"why"`
	Model      string `json:"model"`
	EffectKind string `json:"effectKind"`
	EffectID   string `json:"effectId"`
	Before     int64  `json:"before"`
	Limit      int    `json:"limit"`
}

// JudgementPage is a page of records and the cursor for the next, or 0.
type JudgementPage struct {
	Records []judge.Record `json:"records"`
	Next    int64          `json:"next"`
}

// judging is the window's set of judges.
type judging struct {
	mu    sync.Mutex // guards the fields below
	set   *judge.Judges
	err   error
	cur   JudgementSettings
	rev   string
	clock *sessionhost.StateClock
	// seen are the chats whose first message has been looked at in this run
	// (judge_titles.go).
	seen map[string]bool
	// work counts the title judgements still running, which closing the judges waits for, and closing says they
	// are being waited for, so none starts after (both guarded by mu).
	work    sync.WaitGroup
	closing bool
	// The last calibration, the key of the models it ran for, whether one is going
	// on, how to stop it, and whether the automatic run is left out (tests).
	report      *judge.Report
	calibKey    string
	calibrating bool
	calibCancel context.CancelFunc
	noAuto      bool
	// sessions are the sessions that judgement calls through the protocol have open
	// (judge_acp.go).
	sessions *sessionhost.JudgeSessions
	// handoffs are the judged summaries kept for the run (judge_handoff.go).
	handoffs map[string]handoffEntry

	// numMu makes a state and its number one step, so an older state never has
	// the higher number.
	numMu sync.Mutex
}

func judgementsPath() string { return filepath.Join(configDir(), "judgements.json") }

func judgementRevision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// judgeProviders are the IDs of the provider modules that can judge, in order: the
// default judge first, so that the page lists it first, and then the rest by ID.
func judgeProviders() []string {
	var ids []string
	for id, m := range providerModules {
		if m.Judge != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	first := defaultJudgeProvider(ids)
	if i := slices.Index(ids, first); i > 0 {
		ids = slices.Insert(slices.Delete(ids, i, i+1), 0, first)
	}
	return ids
}

// defaultJudgeProvider is the provider that judges until the person chooses: the first
// whose calls do not use a plan, and otherwise the first.
func defaultJudgeProvider(ids []string) string {
	for _, id := range ids {
		if providerModules[id].Judge.Note == "" {
			return id
		}
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

// judgeModule is how a provider's models make judgement calls.
type judgeModule struct {
	// Build makes the judge for a model of a provider.
	Build func(a *App, provider, model string) judge.Judge
	// Installed lists the models that could judge, with whether running each loads
	// nothing new into memory.
	Installed func(a *App, ctx context.Context, provider string) ([]judge.Model, error)
	// Available says why the provider cannot judge now, or is "" when it can; nil
	// for a provider with nothing to check.
	Available func(a *App, provider string) string
	// Note is a sentence for the page beside the provider, such as that its calls use
	// the person's plan; empty for none.
	Note string
}

func defaultJudgementSettings() JudgementSettings {
	s := JudgementSettings{Enabled: true, RetentionDays: defaultJudgementRetentionDays, WarnStreak: defaultJudgementWarnStreak, KindsOff: []string{},
		Period: JudgementPeriod{Weekday: judge.DefaultPeriod.Weekday, Hour: judge.DefaultPeriod.Hour},
		Pool:   JudgementPool{Min: judge.DefaultPoolMin, Max: judge.DefaultPoolMax}}
	if ids := judgeProviders(); len(ids) > 0 {
		s.Provider = ids[0]
	}
	s.Budgets = budgetRows(nil)
	return s
}

// budgetRows is one row for each provider that can judge, with the limits already
// chosen, and the unit of each from its row in the provider table.
func budgetRows(have []JudgementBudget) []JudgementBudget {
	rows := []JudgementBudget{}
	for _, id := range judgeProviders() {
		row := JudgementBudget{Provider: id, Unit: providers.Registry[id].BudgetUnit}
		for _, h := range have {
			if h.Provider == id {
				row.Limit = h.Limit
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// normalizeJudgementSettings fills what a settings file from an earlier release
// or by hand leaves out.
func normalizeJudgementSettings(s JudgementSettings) JudgementSettings {
	d := defaultJudgementSettings()
	if s.Provider == "" {
		s.Provider = d.Provider
	}
	if s.RetentionDays < 1 {
		s.RetentionDays = d.RetentionDays
	}
	if s.WarnStreak < 1 {
		s.WarnStreak = d.WarnStreak
	}
	if s.KindsOff == nil {
		s.KindsOff = []string{}
	}
	if s.KindsOn == nil {
		s.KindsOn = []string{}
	}
	// A model chosen before the source was kept was the person's.
	if s.Model != "" && s.ModelSource == "" {
		s.ModelSource = "person"
	}
	if s.Period.Weekday < 0 || s.Period.Weekday > 6 || s.Period.Hour < 0 || s.Period.Hour > 23 {
		s.Period = d.Period
	}
	if s.Pool.Min < 1 || s.Pool.Max < s.Pool.Min {
		s.Pool = d.Pool
	}
	for i := range s.Budgets {
		if s.Budgets[i].Limit != nil && *s.Budgets[i].Limit < 0 {
			s.Budgets[i].Limit = nil
		}
	}
	s.Budgets = budgetRows(s.Budgets)
	return s
}

func decodeJudgementSettings(data []byte) (JudgementSettings, error) {
	s := defaultJudgementSettings()
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("judgements.json is not readable: %w", err)
	}
	return normalizeJudgementSettings(s), nil
}

func validateJudgementSettings(s JudgementSettings) error {
	if s.RetentionDays < 1 {
		return errors.New("records are kept for at least one day")
	}
	if s.WarnStreak < 1 {
		return errors.New("the warning needs at least one fallback in a row")
	}
	if s.Provider != "" && !slices.Contains(judgeProviders(), s.Provider) {
		return fmt.Errorf("%q cannot judge", s.Provider)
	}
	if s.Period.Weekday < 0 || s.Period.Weekday > 6 {
		return errors.New("the period begins on a weekday from 0, Sunday, to 6, Saturday")
	}
	if s.Period.Hour < 0 || s.Period.Hour > 23 {
		return errors.New("the period begins at an hour from 0 to 23")
	}
	if s.Pool.Min < 1 || s.Pool.Max < s.Pool.Min {
		return errors.New("the pool needs a smallest size of 1 or more and a largest that is not below it")
	}
	for _, b := range s.Budgets {
		if !slices.Contains(judgeProviders(), b.Provider) {
			return fmt.Errorf("%q cannot judge, so it has no budget", b.Provider)
		}
		if b.Limit != nil && *b.Limit < 0 {
			return errors.New("a budget is a number of 0 or more, or none")
		}
	}
	known := map[string]bool{}
	for _, k := range judge.Kinds() {
		known[k.Name] = true
	}
	for _, name := range s.KindsOff {
		if !known[name] {
			return fmt.Errorf("there is no kind of judgement called %q", name)
		}
	}
	return nil
}

// judges returns the set of judges, opening the record and applying the
// person's settings the first time. A failure to open the record is kept and
// returned each time, and never stops a caller from using its rule.
func (a *App) judges() (*judge.Judges, error) {
	j := &a.judging
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.set != nil || j.err != nil {
		return j.set, j.err
	}
	data, err := statefile.ReadShared(judgementsPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		j.err = err
		return nil, err
	}
	settings, err := decodeJudgementSettings(data)
	if err != nil {
		logf("judge", "warning", "%v; the defaults are used", err)
		settings = defaultJudgementSettings()
	}
	store, err := judge.OpenStore(filepath.Join(configDir(), "judgements.db"))
	if err != nil {
		j.err = err
		return nil, err
	}
	j.set = judge.New(judge.Options{
		Store:    store,
		OnRecord: func(int64) { a.emitJudgementsChanged() },
		Logf:     func(format string, args ...any) { logf("judge", "warning", format, args...) },
	})
	j.cur, j.rev = settings, judgementRevision(data)
	a.applyJudgementSettings(j.set, settings)
	a.loadCalibration()
	if !j.noAuto {
		go a.maybeCalibrate()
	}
	return j.set, nil
}

// closeJudges closes the record, and the set is not opened again: a call that is
// still running when the window ends finds it closed, and uses its rule.
func (a *App) closeJudges() {
	a.stopCalibration()
	// A title judgement in flight writes the settings and the judgement store when it ends, so it is waited for
	// (for a limit, since a judge that never answers must not hold up the app's ending).
	a.judging.mu.Lock()
	a.judging.closing = true
	a.judging.mu.Unlock()
	done := make(chan struct{})
	go func() { a.judging.work.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	j := &a.judging
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.set != nil {
		_ = j.set.Store().Close()
		j.set = nil
	}
	j.err = errors.New("the judgements are closed")
}

// applyJudgementSettings makes the set do what the settings say.
func (a *App) applyJudgementSettings(set *judge.Judges, s JudgementSettings) {
	set.SetEnabled(s.Enabled)
	set.SetKindsOff(effectiveKindsOff(s))
	set.Store().SetRetention(time.Duration(s.RetentionDays) * 24 * time.Hour)
	var judging judge.Judge
	if m, ok := providerModules[s.Provider]; ok && m.Judge != nil && s.Model != "" {
		judging = m.Judge.Build(a, s.Provider, s.Model)
	}
	set.SetJudge(judging, s.Provider, providers.Registry[s.Provider].BudgetUnit)
	budgets := map[string]judge.Budget{}
	for _, b := range s.Budgets {
		budgets[b.Provider] = judge.Budget{Limit: b.Limit, Unit: providers.Registry[b.Provider].BudgetUnit}
	}
	set.SetBudgets(budgets, judge.Period{Weekday: s.Period.Weekday, Hour: s.Period.Hour})
	if err := set.SetPool(s.Pool.Min, s.Pool.Max); err != nil {
		logf("judge", "warning", "the pool's size could not be set: %v", err)
	}
}

func (a *App) judgementSettings() (JudgementSettings, string, error) {
	if _, err := a.judges(); err != nil {
		return JudgementSettings{}, "", err
	}
	a.judging.mu.Lock()
	defer a.judging.mu.Unlock()
	return a.judging.cur, a.judging.rev, nil
}

// today is local midnight.
func today() time.Time {
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, n.Location())
}

func parseJudgementTime(text string, fallback time.Time) (time.Time, error) {
	if text == "" {
		return fallback, nil
	}
	return time.Parse(time.RFC3339, text)
}

// stateOf is the state the widget shows for the settings and a day's summary.
func stateOf(s JudgementSettings, sum judge.Summary) JudgementState {
	st := JudgementState{State: "on", Calls: sum.Calls, Fallbacks: sum.Fallbacks, Streak: sum.Streak, WarnStreak: s.WarnStreak}
	if sum.Latest != nil {
		st.Latest = sum.Latest.Format(time.RFC3339)
	}
	switch {
	case !s.Enabled:
		st.State, st.Why = "off", "turned off"
	case s.Model == "":
		st.State, st.Why = "off", "no model chosen"
	case sum.Streak >= s.WarnStreak:
		st.State, st.Why = "warning", fmt.Sprintf("%d fallbacks in a row", sum.Streak)
	}
	return st
}

// JudgementState is what the widget shows: on, off or warning, with today's
// counts (from local midnight, not all-time), the current run of fallbacks and
// the threshold that makes it a warning. It is numbered like the events that
// carry the same value.
func (a *App) JudgementState() (JudgementState, error) {
	set, err := a.judges()
	if err != nil {
		return JudgementState{}, err
	}
	settings, _, err := a.judgementSettings()
	if err != nil {
		return JudgementState{}, err
	}
	a.judging.numMu.Lock()
	defer a.judging.numMu.Unlock()
	sum, err := set.Store().Summarize(today(), time.Time{})
	if err != nil {
		return JudgementState{}, err
	}
	st := stateOf(settings, sum)
	if st.Budget, err = set.BudgetUses(); err != nil {
		return JudgementState{}, err
	}
	if st.Budget == nil {
		st.Budget = []judge.BudgetUse{}
	}
	st.Pool = set.PoolState()
	st.Calibration = a.calibrationState()
	// A call answered by its rule because the budget did not allow it is a warning
	// of its own, whatever the run of fallbacks is, unless the person turned the
	// judges off.
	if st.State == "on" {
		if rows, _, err := set.Store().List(judge.Filter{Limit: 1}); err == nil && len(rows) == 1 && rows[0].Why == judge.WhyBudget {
			st.State, st.Why = "warning", "the budget is spent for "+rows[0].Provider
		}
	}
	st.StateGen, st.StateVer = a.judgementClock().Next()
	return st, nil
}

func (a *App) judgementClock() *sessionhost.StateClock {
	j := &a.judging
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.clock == nil {
		j.clock = sessionhost.NewStateClock()
	}
	return j.clock
}

// emitJudgementsChanged tells the page the state as it is now. It is sent after
// a call is recorded and after the settings are saved, with the state and its
// number taken in one step.
func (a *App) emitJudgementsChanged() {
	st, err := a.JudgementState()
	if err != nil {
		return
	}
	a.emit("judgements:changed", sessionhost.Stamp(map[string]any{
		"state": st.State, "why": st.Why, "calls": st.Calls, "fallbacks": st.Fallbacks, "streak": st.Streak,
		"warnStreak": st.WarnStreak, "latest": st.Latest, "budget": st.Budget, "pool": st.Pool, "calibration": st.Calibration,
	}, st.StateGen, st.StateVer))
}

// JudgementSummary totals the calls from from to to (both RFC 3339; empty means
// from local midnight, and until now) by kind and model.
func (a *App) JudgementSummary(from, to string) (judge.Summary, error) {
	set, err := a.judges()
	if err != nil {
		return judge.Summary{}, err
	}
	f, err := parseJudgementTime(from, today())
	if err != nil {
		return judge.Summary{}, fmt.Errorf("from is not a time: %w", err)
	}
	t, err := parseJudgementTime(to, time.Time{})
	if err != nil {
		return judge.Summary{}, fmt.Errorf("to is not a time: %w", err)
	}
	sum, err := set.Store().Summarize(f, t)
	if sum.Groups == nil {
		sum.Groups = []judge.Group{}
	}
	return sum, err
}

// JudgementRecords returns a page of the record, newest first.
func (a *App) JudgementRecords(f JudgementFilter) (JudgementPage, error) {
	set, err := a.judges()
	if err != nil {
		return JudgementPage{}, err
	}
	filter := judge.Filter{Kind: f.Kind, Used: f.Used, Why: f.Why, Model: f.Model, EffectKind: f.EffectKind, EffectID: f.EffectID, Before: f.Before, Limit: f.Limit}
	if filter.From, err = parseJudgementTime(f.From, time.Time{}); err != nil {
		return JudgementPage{}, fmt.Errorf("from is not a time: %w", err)
	}
	if filter.To, err = parseJudgementTime(f.To, time.Time{}); err != nil {
		return JudgementPage{}, fmt.Errorf("to is not a time: %w", err)
	}
	rows, next, err := set.Store().List(filter)
	if rows == nil {
		rows = []judge.Record{}
	}
	return JudgementPage{Records: rows, Next: next}, err
}

// JudgementKinds lists every declared kind of call with its input fields, which
// is everything a judge may be shown, and whether it is on.
func (a *App) JudgementKinds() ([]JudgementKind, error) {
	settings, _, err := a.judgementSettings()
	if err != nil {
		return nil, err
	}
	out := []JudgementKind{}
	for _, k := range judge.Kinds() {
		out = append(out, JudgementKind{Name: k.Name, Version: k.Version, Describe: k.Describe, Correctable: k.Correctable, DeadlineMs: k.DeadlineMs,
			Enabled: !slices.Contains(effectiveKindsOff(settings), k.Name), InputCap: k.InputCap, DefaultOff: k.DefaultOff, Input: k.Input})
	}
	return out, nil
}

// JudgementSettings returns the person's choices and their revision.
func (a *App) JudgementSettings() (JudgementSettingsSnapshot, error) {
	s, rev, err := a.judgementSettings()
	return JudgementSettingsSnapshot{Settings: presentJudgementSettings(s), Revision: rev}, err
}

// effectiveKindsOff is every kind that is off: the ones the person turned off and
// the ones that start off which the person has not turned on.
func effectiveKindsOff(s JudgementSettings) []string {
	off := slices.Clone(s.KindsOff)
	for _, k := range judge.Kinds() {
		if k.DefaultOff && !slices.Contains(s.KindsOn, k.Name) && !slices.Contains(off, k.Name) {
			off = append(off, k.Name)
		}
	}
	return off
}

// presentJudgementSettings is the settings as the page reads them, with every kind
// that is off in KindsOff.
func presentJudgementSettings(s JudgementSettings) JudgementSettings {
	s.KindsOff = effectiveKindsOff(s)
	slices.Sort(s.KindsOff)
	return s
}

// keepJudgementKinds turns the page's list of the kinds that are off back into what
// is kept: the kinds the person turned off, and the kinds that start off which they
// turned on, which are the ones the list leaves out.
func keepJudgementKinds(s *JudgementSettings) {
	startsOff := map[string]bool{}
	for _, k := range judge.Kinds() {
		if k.DefaultOff {
			startsOff[k.Name] = true
		}
	}
	off, on := []string{}, []string{}
	for _, name := range s.KindsOff {
		if !startsOff[name] {
			off = append(off, name)
		}
	}
	for name := range startsOff {
		if !slices.Contains(s.KindsOff, name) {
			on = append(on, name)
		}
	}
	slices.Sort(on)
	s.KindsOff, s.KindsOn = off, on
}

// SetJudgementSettings saves the settings if the revision is the current one.
// Against any other revision it writes nothing and returns the current settings
// with Conflict set.
func (a *App) SetJudgementSettings(settings JudgementSettings, revision string) (JudgementSettingsSnapshot, error) {
	set, err := a.judges()
	if err != nil {
		return JudgementSettingsSnapshot{}, err
	}
	// The numbers are checked as given: a zero is refused and not raised to the
	// default, which only a file from an earlier release is.
	if err := validateJudgementSettings(settings); err != nil {
		return JudgementSettingsSnapshot{}, err
	}
	// The source of the model is the engine's: a model the person saves is theirs,
	// and an unchanged one keeps the source it had.
	j0 := &a.judging
	j0.mu.Lock()
	before := j0.cur
	j0.mu.Unlock()
	if settings.Model != before.Model {
		settings.ModelSource = "person"
	} else {
		settings.ModelSource = before.ModelSource
	}
	settings = normalizeJudgementSettings(settings)
	slices.Sort(settings.KindsOff)
	settings.KindsOff = slices.Compact(settings.KindsOff)
	keepJudgementKinds(&settings)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return JudgementSettingsSnapshot{}, err
	}
	conflict := false
	err = statefile.Update(judgementsPath(), func(previous []byte) ([]byte, error) {
		if judgementRevision(previous) != revision {
			conflict = true
			return nil, nil
		}
		return data, nil
	})
	if err != nil {
		return JudgementSettingsSnapshot{}, err
	}
	j := &a.judging
	if conflict {
		fresh, err := statefile.ReadShared(judgementsPath())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return JudgementSettingsSnapshot{}, err
		}
		current, err := decodeJudgementSettings(fresh)
		if err != nil {
			return JudgementSettingsSnapshot{}, err
		}
		return JudgementSettingsSnapshot{Settings: presentJudgementSettings(current), Revision: judgementRevision(fresh), Conflict: true}, nil
	}
	j.mu.Lock()
	j.cur, j.rev = settings, judgementRevision(data)
	j.mu.Unlock()
	a.applyJudgementSettings(set, settings)
	a.forgetHandoffs()
	a.emitJudgementsChanged()
	return JudgementSettingsSnapshot{Settings: presentJudgementSettings(settings), Revision: judgementRevision(data)}, nil
}

// JudgementModels lists the models that could judge, for the picker. A provider
// that cannot be reached adds none and is logged.
func (a *App) JudgementModels() ([]JudgementModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out := []JudgementModel{}
	settings, _, _ := a.judgementSettings()
	for _, id := range judgeProviders() {
		// A provider whose calls use the person's plan is listed only when it is the
		// chosen judge: listing its models starts it and a session.
		if providerModules[id].Judge.Note != "" && id != settings.Provider {
			continue
		}
		installed, err := providerModules[id].Judge.Installed(a, ctx, id)
		if err != nil {
			logf("judge", "warning", "the models of %s could not be listed: %v", id, err)
			continue
		}
		for _, m := range installed {
			out = append(out, JudgementModel{Name: m.Name, Provider: id})
		}
	}
	return out, nil
}

// CorrectJudgement keeps the person's replacement for a decision. It records
// the correction; applying it to the thing the decision was about is done by
// the code that made the decision.
func (a *App) CorrectJudgement(id int64, value string) (judge.Record, error) {
	set, err := a.judges()
	if err != nil {
		return judge.Record{}, err
	}
	rec, err := set.Store().Get(id)
	if err != nil {
		return judge.Record{}, err
	}
	// The correction is applied to what the decision was about first, so one that
	// cannot be applied is refused and not recorded as if it had been.
	if apply := correctors[rec.Kind]; apply != nil {
		if err := apply(a, rec, value); err != nil {
			return judge.Record{}, err
		}
	}
	if err := set.Store().Correct(id, value); err != nil {
		return judge.Record{}, err
	}
	a.emitJudgementsChanged()
	return set.Store().Get(id)
}

// JudgementProvider is a provider that could judge, for the choice of provider:
// whether it can judge now and, if not, why in words, the unit its calls are counted
// in, and a sentence to show beside it when its calls use the person's plan.
type JudgementProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Available bool   `json:"available"`
	Why       string `json:"why"`
	Note      string `json:"note"`
	// TypicalCost is the median cost of a call in Unit among the calls the provider
	// has recorded, and Calls how many it rests on; both are 0 when none is
	// recorded, and the page then uses its own wording.
	TypicalCost int `json:"typicalCost"`
	Calls       int `json:"calls"`
}

// JudgementProviders lists the providers that could judge, in order, with whether each
// can now. A provider that is not installed or not running is listed, with the reason,
// so that the page can say why it is not offered.
func (a *App) JudgementProviders() ([]JudgementProvider, error) {
	out := []JudgementProvider{}
	for _, id := range judgeProviders() {
		m := providerModules[id].Judge
		p := JudgementProvider{ID: id, Name: providerName(id), Unit: providers.Registry[id].BudgetUnit, Available: true, Note: m.Note}
		if m.Available != nil {
			p.Why = m.Available(a, id)
			p.Available = p.Why == ""
		}
		if m.Note != "" {
			if set, err := a.judges(); err == nil {
				if median, calls, err := set.TypicalCost(id, p.Unit); err == nil {
					p.TypicalCost, p.Calls = median, calls
				}
			}
		}
		out = append(out, p)
	}
	return out, nil
}
