// Package judge makes judgement calls: small, self-contained questions that Dinah
// puts to a model and whose answer it uses in place of a rule
// (docs/plans/judgement-agents.md). A kind of call is declared once, with the
// structure of the only input the model is ever shown, the form of the answer,
// a check on it, a deadline, and the rule that Dinah used before there was a
// judge. Call asks the judge and returns the rule's answer on any failure, so a
// slow, missing or wrong model can lower the quality of a decision and can never
// stall or break the program. Every call is recorded (store.go).
//
// The package knows nothing of the window, the bus or any chat.
package judge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"studio/internal/appletcontext"
)

// Why a call used the rule's answer and not the judge's.
const (
	WhyOff     = "off"      // the person turned the judges off
	WhyNoJudge = "no-judge" // no judge is set up
	WhyLate    = "late"     // the deadline passed, in the queue or while the judge worked
	WhyForm    = "form"     // the answer was not in the declared form
	WhyCheck   = "check"    // the answer failed the kind's check
	WhyError   = "error"    // the judge failed
	// WhyBudget is in budget.go: the provider's budget did not allow the call.
)

// Whose answer a call used.
const (
	UsedJudge = "judge"
	UsedRule  = "rule"
)

// Form is the shape of a kind's answer.
type Form[Out any] struct {
	// Doc says the form in words, for the system prompt's reader and for the
	// kind's version.
	Doc string
	// JSON asks the judge to answer in JSON only.
	JSON bool
	// Parse reads the judge's text into the answer, or says it is not in form.
	Parse func(text string) (Out, error)
}

// TextForm is plain text of at most max characters, on one line.
func TextForm(max int) Form[string] {
	return Form[string]{
		Doc: fmt.Sprintf("plain text, one line, at most %d characters", max),
		Parse: func(text string) (string, error) {
			line := strings.Join(strings.Fields(text), " ")
			if line == "" {
				return "", errors.New("the answer is empty")
			}
			if len([]rune(line)) > max {
				return "", fmt.Errorf("the answer is %d characters, more than %d", len([]rune(line)), max)
			}
			return line, nil
		},
	}
}

// ParagraphsForm is plain text of several lines, such as a summary. Its length is
// not checked here, since a call may allow a different length each time; the
// kind's check does it.
func ParagraphsForm() Form[string] {
	return Form[string]{
		Doc: "plain text of several short lines",
		Parse: func(text string) (string, error) {
			text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
			if text == "" {
				return "", errors.New("the answer is empty")
			}
			return text, nil
		},
	}
}

// JSONForm is a JSON document that decodes into Out with no unknown fields.
// doc says what it holds.
func JSONForm[Out any](doc string) Form[Out] {
	return Form[Out]{
		Doc:  "JSON: " + doc,
		JSON: true,
		Parse: func(text string) (Out, error) {
			var out Out
			dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(text)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&out); err != nil {
				return out, err
			}
			if dec.More() {
				return out, errors.New("the answer has text after its JSON")
			}
			return out, nil
		},
	}
}

// Kind is a declared kind of call. In is the structure the judge is shown, and
// is the whole of what it may ever see; each field describes itself with a doc
// tag. Out is the answer.
type Kind[In, Out any] struct {
	// Name identifies the kind in the record, such as chat-title.
	Name string
	// Describe says in plain words what the rule does, so Settings and the page
	// can say what the fallback is without reading Go.
	Describe string
	// Correctable is true where the person's correction can be applied to the
	// decision, such as a title.
	Correctable bool
	// System is the short orienting prompt.
	System string
	// Form is the form of the answer.
	Form Form[Out]
	// Check refuses an answer that is in form and still unusable, such as a
	// set-aside that includes the current turn. It may be nil.
	Check func(In, Out) error
	// Deadline is how long a call may take, counted from the call and including
	// any time in the queue.
	Deadline time.Duration
	// Rule is the answer Dinah gives without a judge. It must be cheap.
	Rule func(In) Out
	// InputCap is the most bytes of the rendered input a call of the kind may send,
	// declared with the kind and shown wherever it is listed, so the person can see
	// what bounds it. A call with a larger input is answered by its rule. Zero is no
	// cap, which is for the kinds whose input is one short line.
	InputCap int
	// DefaultOff says the kind is turned off until the person turns it on, for a
	// kind that shows a judge much more of a chat than a line.
	DefaultOff bool

	version string
}

// Info describes a declared kind without its types, for the registry.
type Info struct {
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Describe    string  `json:"describe"`
	Correctable bool    `json:"correctable"`
	System      string  `json:"system"`
	Form        string  `json:"form"`
	DeadlineMs  int64   `json:"deadlineMs"`
	InputCap    int     `json:"inputCap"`
	DefaultOff  bool    `json:"defaultOff"`
	Input       []Field `json:"input"`
}

// Field is one field of a kind's input structure.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Doc  string `json:"doc"`
}

var (
	regMu    sync.Mutex
	registry = map[string]Info{}
)

// Declare registers a kind and returns it for use with Call. It panics when the
// declaration is unusable, as a declaration in a package's variables is read
// when the program starts.
func Declare[In, Out any](k Kind[In, Out]) *Kind[In, Out] {
	if k.Name == "" || k.System == "" || k.Deadline <= 0 || k.Rule == nil || k.Form.Parse == nil || k.Describe == "" {
		panic(fmt.Sprintf("judge: the kind %q needs a name, a description, a system prompt, a form, a deadline and a rule", k.Name))
	}
	fields := inputFields(reflect.TypeFor[In]())
	if len(fields) == 0 {
		panic(fmt.Sprintf("judge: the input of %q must be a struct with json-tagged fields", k.Name))
	}
	for _, f := range fields {
		if f.Doc == "" {
			panic(fmt.Sprintf("judge: field %s of the input of %q has no doc tag", f.Name, k.Name))
		}
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%v\x00", k.System, k.Form.Doc, k.Form.JSON)
	for _, f := range fields {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", f.Name, f.Type, f.Doc)
	}
	k.version = hex.EncodeToString(h.Sum(nil))[:12]
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[k.Name]; dup {
		panic(fmt.Sprintf("judge: the kind %q is declared twice", k.Name))
	}
	registry[k.Name] = Info{Name: k.Name, Version: k.version, Describe: k.Describe, Correctable: k.Correctable,
		System: k.System, Form: k.Form.Doc, DeadlineMs: k.Deadline.Milliseconds(), InputCap: k.InputCap, DefaultOff: k.DefaultOff, Input: fields}
	return &k
}

// Version is a hash of the kind's system prompt, input structure and answer
// form, so records made before and after a change are told apart.
func (k *Kind[In, Out]) Version() string { return k.version }

// Kinds lists every declared kind: the complete list of what a judge may ever
// be shown.
func Kinds() []Info {
	regMu.Lock()
	defer regMu.Unlock()
	out := make([]Info, 0, len(registry))
	for _, i := range registry {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

func inputFields(t reflect.Type) []Field {
	if t.Kind() != reflect.Struct {
		return nil
	}
	var out []Field
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		out = append(out, Field{Name: name, Type: f.Type.String(), Doc: f.Tag.Get("doc")})
	}
	return out
}

// Cost is what a call used, in the provider's own unit.
type Cost struct {
	Unit string `json:"unit"` // tokens for most providers, acu for Devin
	In   int    `json:"in"`
	Out  int    `json:"out"`
	// Unknown says the provider did not report what the call used. The record
	// keeps the unit UnitUnknown and no figure, and a budget cannot count the call.
	Unknown bool `json:"unknown,omitempty"`
}

// UnitUnknown is the unit of a call whose cost the provider did not report.
const UnitUnknown = "unknown"

// costUnit is the unit a call's record is kept in: the provider's, or UnitUnknown
// when the call's cost is not known.
func costUnit(unit string, c Cost) string {
	if c.Unknown {
		return UnitUnknown
	}
	return unit
}

// Request is what the judge is asked.
type Request struct {
	System string
	// Input is the declared input as JSON, with applet contexts removed.
	Input string
	// JSON asks for an answer in JSON only.
	JSON bool
	// MaxTokens bounds the answer.
	MaxTokens int
}

// Response is the judge's answer and what it was made with.
type Response struct {
	Text        string
	Provider    string
	Model       string
	Temperature float64
	Seed        int
	Cost        Cost
}

// Judge is a model that answers requests. Ask must return when ctx ends.
type Judge interface {
	Ask(ctx context.Context, r Request) (Response, error)
}

// Effect says where a decision was applied, so the page can link to it and a
// correction can be written back: a kind of thing, such as chat, and its id.
type Effect struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Result is a call's answer and its record.
type Result[Out any] struct {
	Out  Out
	Used string
	Why  string
	// ID is the call's record, or 0 when it could not be recorded.
	ID int64
}

// Judges is the set of judges and the record of what they decided.
type Judges struct {
	store *Store

	mu      sync.Mutex
	judge   Judge
	enabled bool
	off     map[string]bool
	// provider and unit are who judges and the unit the provider counts cost in,
	// from its row in the provider table.
	provider string
	unit     string
	budgets  map[string]Budget
	period   Period
	pool     *pool
	now      func() time.Time
	logf     func(format string, args ...any)
	onRec    func(id int64)
	// health holds what repeated errors have paused (health.go).
	health health
}

// Options are the parts of New that tests and callers may set.
type Options struct {
	Store *Store
	Judge Judge
	// PoolMin and PoolMax are the smallest and largest number of calls that may be
	// with the judge at once; zero means DefaultPoolMin and DefaultPoolMax.
	PoolMin, PoolMax int
	Now              func() time.Time
	Logf             func(format string, args ...any)
	// OnRecord is called with a call's record id after it is written, so the
	// window can tell the page. It runs on the caller's goroutine.
	OnRecord func(id int64)
}

// New makes the set. The judges are on; the person turns them off with SetEnabled.
func New(o Options) *Judges {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	logf := o.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Judges{store: o.Store, judge: o.Judge, enabled: true, off: map[string]bool{}, period: DefaultPeriod, now: now, logf: logf, onRec: o.OnRecord,
		pool: newPool(now, longestDeadline, o.PoolMin, o.PoolMax)}
}

// Store is the record the judges write.
func (j *Judges) Store() *Store { return j.store }

// SetJudge sets the judge, or with nil removes it, with the provider it belongs to
// and the unit that provider counts cost in, which are what its budget is kept in.
func (j *Judges) SetJudge(x Judge, provider, unit string) {
	j.mu.Lock()
	j.judge, j.provider, j.unit = x, provider, unit
	j.mu.Unlock()
	// A judge that is set again, as it is whenever the person changes the judges' settings, is tried afresh.
	j.health.reset()
}

// SetPool sets the smallest and largest size of the pool.
func (j *Judges) SetPool(min, max int) error { return j.pool.setBounds(min, max) }

// PoolState is the pool as it is now.
func (j *Judges) PoolState() PoolState { return j.pool.state() }

// longestDeadline is the longest deadline of any declared kind, which the pool's
// idle time is counted in.
func longestDeadline() time.Duration {
	var longest int64
	for _, k := range Kinds() {
		longest = max(longest, k.DeadlineMs)
	}
	if longest == 0 {
		return time.Second
	}
	return time.Duration(longest) * time.Millisecond
}

// SetEnabled turns the judges on or off. Off, every call is answered by its rule.
func (j *Judges) SetEnabled(on bool) {
	j.mu.Lock()
	j.enabled = on
	j.mu.Unlock()
}

// SetKindsOff turns off the kinds of call named, which are then answered by
// their rules, and turns on every other.
func (j *Judges) SetKindsOff(names []string) {
	off := map[string]bool{}
	for _, n := range names {
		off[n] = true
	}
	j.mu.Lock()
	j.off = off
	j.mu.Unlock()
}

// state is the judge, whether calls of the kind may use it, and the provider and
// unit it counts in.
func (j *Judges) state(kind string) (judge Judge, on bool, provider, unit string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.judge, j.enabled && !j.off[kind], j.provider, j.unit
}

// maxAnswerTokens bounds a judge's answer; a judgement is short.
const maxAnswerTokens = 200

// LateFactor is how many times its deadline a request may run on after the call
// has been answered by its rule for being late. The request is not cancelled at
// the deadline, so that what it cost is known and counted against the budget; this
// only keeps a request that hangs from holding its place in the pool forever. It is
// the same multiple the pool waits idle before it shrinks.
const LateFactor = ShrinkFactor

type judgeAnswer struct {
	resp Response
	err  error
}

// lateCall settles who writes a late call's cost to its record: the caller, which
// has the record's number only after it has answered by the rule, or the request's
// goroutine, whichever finishes second. Exactly one of them does.
type lateCall struct {
	mu       sync.Mutex
	late     bool
	finished bool
	id       int64
	ans      judgeAnswer
}

// finish is called by the request's goroutine with its answer, and says whether the
// caller has already given up and recorded the call, so the goroutine completes it.
func (l *lateCall) finish(a judgeAnswer) (id int64, complete bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.finished, l.ans = true, a
	return l.id, l.late
}

// giveUp is called by the caller once it has answered by the rule and has the
// record's number, and says whether the request has already ended, so the caller
// completes the record.
func (l *lateCall) giveUp(id int64) (judgeAnswer, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.late, l.id = true, id
	return l.ans, l.finished
}

// completeLate writes a late call's cost, and the answer it gave, to its record, and
// tells the window, so that the budget counts what the provider spent.
func (j *Judges) completeLate(id int64, a judgeAnswer, unit string) {
	if id == 0 || j.store == nil {
		return
	}
	text := a.resp.Text
	if a.err != nil && text == "" {
		text = a.err.Error()
	}
	if err := j.store.CompleteLate(id, appletcontext.Redact(text), a.resp.Model, a.resp.Temperature, a.resp.Seed, costUnit(unit, a.resp.Cost), a.resp.Cost.In, a.resp.Cost.Out); err != nil {
		j.logf("judge: the cost of a late %d call could not be written: %v", id, err)
		return
	}
	if j.onRec != nil {
		j.onRec(id)
	}
}

// Call asks the judge the question a kind declares and returns the answer to
// use. It never fails: when the judge is off, missing, late, out of form or
// refused by the kind's check, it returns the rule's answer, and the record says
// which. It returns within the kind's deadline, counted from the call, whether
// the call waits in the queue or with the judge. The rule's answer is computed
// and recorded on every call, so how often the two disagree can be measured.
func Call[In, Out any](ctx context.Context, j *Judges, k *Kind[In, Out], in In, effect Effect) Result[Out] {
	start := time.Now()
	rec := Record{Time: start, Kind: k.Name, Version: k.version, EffectKind: effect.Kind, EffectID: effect.ID, DeadlineMs: k.Deadline.Milliseconds()}
	rule := k.Rule(in)
	rec.RuleAnswer = encode(rule)
	rec.Used = UsedRule
	finish := func(why string, out Out) Result[Out] {
		rec.Why = why
		rec.ElapsedMs = time.Since(start).Milliseconds()
		if rec.Used == UsedRule {
			out = rule
		}
		rec.Out = encode(out)
		return Result[Out]{Out: out, Used: rec.Used, Why: why, ID: j.record(rec)}
	}
	if j == nil {
		return finish(WhyNoJudge, rule)
	}
	rec.Time = j.now()
	judge, on, provider, unit := j.state(k.Name)
	input, err := json.Marshal(in)
	if err != nil {
		rec.Answer = err.Error()
		return finish(WhyError, rule)
	}
	// The declared structures hold no applet context, and the judge and the record
	// must never see one, so this is the place a leak would do the most harm.
	rendered := appletcontext.Redact(string(input))
	rec.Input = rendered
	switch {
	case !on:
		return finish(WhyOff, rule)
	case judge == nil:
		return finish(WhyNoJudge, rule)
	}
	if k.InputCap > 0 && len(rendered) > k.InputCap {
		rec.Answer = fmt.Sprintf("the input is %d bytes, more than the %d this kind may send", len(rendered), k.InputCap)
		return finish(WhyError, rule)
	}
	rec.Provider = provider
	if j.overBudget(provider) {
		return finish(WhyBudget, rule)
	}
	ok, probe, paused := j.health.admit(provider, j.now())
	if !ok {
		rec.Answer = "paused until " + paused.Until.Local().Format("15:04") + ": " + paused.Reason
		return finish(WhyPaused, rule)
	}
	// A probe that ends without the judge having answered or failed lets the next call probe.
	resolved := false
	if probe {
		defer func() {
			if !resolved {
				j.health.released(provider)
			}
		}()
	}
	dctx, cancel := context.WithTimeout(ctx, k.Deadline)
	defer cancel()
	release, got := j.pool.acquire(dctx, k.Deadline)
	if !got {
		return finish(WhyLate, rule)
	}
	// The judge works in its own goroutine, which holds the place until its request
	// ends, so the pool counts what the provider is doing, and a judge that ignores
	// its context cannot make the call run past its deadline. The request is not
	// cancelled at the deadline: the caller is answered by the rule, and when the
	// late answer arrives its cost is written to the call's record and counted
	// against the budget, the answer kept and marked as not used.
	askCtx, askCancel := context.WithTimeout(context.WithoutCancel(ctx), k.Deadline*LateFactor)
	done := make(chan judgeAnswer, 1)
	lt := &lateCall{}
	go func() {
		defer release()
		defer askCancel()
		r, err := judge.Ask(askCtx, Request{System: k.System + "\nAnswer as " + k.Form.Doc + ".", Input: rendered, JSON: k.Form.JSON, MaxTokens: maxAnswerTokens})
		a := judgeAnswer{r, err}
		if id, complete := lt.finish(a); complete {
			j.completeLate(id, a, unit)
			return
		}
		done <- a
	}()
	var resp Response
	select {
	case a := <-done:
		resp, err = a.resp, a.err
	case <-dctx.Done():
		res := finish(WhyLate, rule)
		if a, ended := lt.giveUp(res.ID); ended {
			j.completeLate(res.ID, a, unit)
		}
		return res
	}
	rec.Provider, rec.Model, rec.Temperature, rec.Seed = provider, resp.Model, resp.Temperature, resp.Seed
	rec.CostUnit, rec.CostIn, rec.CostOut = costUnit(unit, resp.Cost), resp.Cost.In, resp.Cost.Out
	rec.Answer = appletcontext.Redact(resp.Text)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || dctx.Err() != nil {
			return finish(WhyLate, rule)
		}
		rec.Answer = appletcontext.Redact(err.Error())
		resolved = true
		j.health.failed(provider, ShortReason(rec.Answer), j.now())
		return finish(WhyError, rule)
	}
	resolved = true
	j.health.answered(provider)
	if dctx.Err() != nil {
		return finish(WhyLate, rule)
	}
	out, err := k.Form.Parse(resp.Text)
	if err != nil {
		return finish(WhyForm, rule)
	}
	if k.Check != nil {
		if err := k.Check(in, out); err != nil {
			return finish(WhyCheck, rule)
		}
	}
	rec.Used = UsedJudge
	return finish("", out)
}

// record writes a call's record and returns its id, or 0 when it cannot be
// written, which never changes the call's result.
func (j *Judges) record(r Record) int64 {
	if j == nil || j.store == nil {
		return 0
	}
	id, err := j.store.Add(r)
	if err != nil {
		j.logf("judge: the record of a %s call could not be written: %v", r.Kind, err)
		return 0
	}
	if j.onRec != nil {
		j.onRec(id)
	}
	return id
}

func encode(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return appletcontext.Redact(string(b))
}
