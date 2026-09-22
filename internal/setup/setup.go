// Package setup connects a harness to a workbench by applying a recipe.
//
// A recipe is a directory holding a manifest, a list of steps and a prompt.
// The steps are data this package interprets, so that it can show exactly
// what an apply would change, refuse a conflict before anything is written,
// change nothing on a rerun, and take back only what it wrote. One opt-in
// step kind runs a program instead, and every one of those promises stops at
// it. The prompt is printed for whoever ran setup, carrying what a step
// cannot do.
//
// The package reads no environment. The command hands it the home
// directory, the user base and the resolved workbench, so a test passes
// throwaway directories and nothing reaches a real home.
package setup

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The change tokens a report carries. They are canonical and appear
// untranslated in the machine form.
const (
	ChangeCreate    = "create"
	ChangeAdd       = "add"
	ChangeUpdate    = "update"
	ChangeUnchanged = "unchanged"
	ChangeRemove    = "remove"
	ChangeGone      = "gone"
	ChangeRan       = "ran"
	ChangeWouldRun  = "would-run"
)

// Options are everything one run of setup needs, gathered by the command.
type Options struct {
	// Harness is the recipe named by position, empty when --recipe names one
	// by path.
	Harness string
	// RecipeDir is the directory --recipe names.
	RecipeDir string
	// Agent, Tools, Provider, Model and Server are the identity flags, each
	// empty when not given.
	Agent, Tools, Provider, Model, Server string
	// Scope is --scope, empty for the default.
	Scope string
	// Target is --target, empty for the default.
	Target string
	// TrustProjectRecipe, AllowRun, DryRun and Remove are the markers.
	TrustProjectRecipe, AllowRun, DryRun, Remove bool
	// Home is the machine's own home directory, which DINAH_HOME does not
	// move. It is the user scope's base.
	Home string
	// UserBase is Dinah's user base, where user recipes and the ledger live.
	UserBase string
	// DinahHome is DINAH_HOME, empty when it is not set.
	DinahHome string
	// Workbench is the resolved workbench's directory, empty when discovery
	// found none, in which case WorkbenchErr says why.
	Workbench string
	// WorkbenchTitle and Operator are the resolved workbench's own.
	WorkbenchTitle, Operator string
	// WorkbenchErr is discovery's refusal, raised only where a workbench is
	// needed.
	WorkbenchErr error
	// ConfiguredActor is the actor setting in the user's own config.md.
	ConfiguredActor string
	// Describe renders a catalog key in the reader's language, for the two
	// phrases a refusal's detail carries.
	Describe func(key string) string
	// Programs receives a run step's standard output and standard error.
	Programs io.Writer
}

// RecipeInfo names the recipe a report was produced from.
type RecipeInfo struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Source string `json:"source"`
	Path   string `json:"path"`
}

// Change is one row of the change report.
type Change struct {
	// Step is the step's id, empty for a location a recipe no longer
	// produces.
	Step string `json:"step"`
	// Kind is the step kind.
	Kind string `json:"kind"`
	// File is the file, relative to the base with forward slashes, empty for
	// a run step.
	File string `json:"file"`
	// Key is the JSON pointer, the section's <recipe>/<id>, the empty string
	// for a whole file, or a run step's command line.
	Key string `json:"key"`
	// Change is one of the change tokens.
	Change string `json:"change"`
	// Before and After are the owned span's text, empty where it is absent.
	Before string `json:"before"`
	After  string `json:"after"`
}

// Report is what one run of setup did, or on a dry run would do.
type Report struct {
	Recipe    RecipeInfo `json:"recipe"`
	Scope     string     `json:"scope"`
	Base      string     `json:"base"`
	Workbench string     `json:"workbench"`
	DryRun    bool       `json:"dry_run"`
	Remove    bool       `json:"remove"`
	Changes   []Change   `json:"changes"`
	Prompt    string     `json:"prompt"`
	// BaseDir is the base in the platform's own spelling, for a heading.
	BaseDir string `json:"-"`
	// Nothing says a removal found no ledger entry for the run.
	Nothing bool `json:"-"`
}

// Count reports how many rows of the report carry a change token.
func (r *Report) Count(token string) int {
	count := 0
	for _, c := range r.Changes {
		if c.Change == token {
			count++
		}
	}
	return count
}

// StepFailed is a run step that could not be found or exited non-zero. The
// report carries every change that completed before it.
type StepFailed struct {
	// Refusal is the dinah.setup-step-failed refusal.
	Refusal *contract.Refusal
	// Step is the failing step's id.
	Step string
	// Report is what the run did before the failure.
	Report *Report
}

// Error renders the refusal.
func (e *StepFailed) Error() string {
	return e.Refusal.Error()
}

// Run applies a recipe, or removes what it wrote, after every check of the
// command's precondition list has passed. Nothing is written until then, and
// nothing at all on a dry run.
func Run(opts Options) (*Report, error) {
	r, err := resolveRecipe(opts)
	if err != nil {
		return nil, err
	}
	if r.Source == SourceProject && !opts.TrustProjectRecipe {
		return nil, contract.Refuse(contract.UntrustedRecipe, r.Dir)
	}
	scope := opts.Scope
	if scope == "" {
		scope = ScopeProject
	}
	if !contains(r.Scopes, scope) {
		return nil, contract.Refuse(contract.UnknownScope, scope)
	}
	tools := firstOf(opts.Tools, r.Tools)
	if !contains(toolProfiles, tools) {
		return nil, contract.Refuse(contract.UnknownToolProfile, tools)
	}
	if flag := misplacedFlag(opts, scope); flag != "" {
		return nil, contract.Refuse(contract.Usage, flag)
	}
	if !opts.Remove && !opts.DryRun && !opts.AllowRun && r.hasRun(scope) {
		return nil, refuseListing(contract.SetupRunNotAllowed, "steps", runListing(r, scope, opts, tools))
	}
	if scope == ScopeProject && opts.Workbench == "" {
		if opts.WorkbenchErr != nil {
			return nil, opts.WorkbenchErr
		}
		return nil, contract.Refuse(contract.NoWorkbenchFound, "")
	}
	if scope == ScopeUser && opts.DinahHome != "" && !sameDirectory(opts.DinahHome, opts.Home) {
		return nil, contract.Refuse(contract.SetupRelocatedHome, opts.DinahHome)
	}
	base, err := resolveBase(opts, scope)
	if err != nil {
		return nil, err
	}
	facts := Facts{
		Harness:  r.Harness,
		Agent:    firstOf(opts.Agent, r.Agent),
		Tools:    tools,
		Provider: firstOf(opts.Provider, r.Provider),
		Model:    opts.Model,
		Server:   opts.Server,
		Scope:    scope,
		Base:     base,
		Recipe:   r.Name,
	}
	if scope == ScopeProject {
		facts.Workbench = opts.Workbench
		facts.WorkbenchTitle = opts.WorkbenchTitle
	}
	if !opts.Remove {
		if err := checkValues(opts, facts); err != nil {
			return nil, err
		}
		if err := checkAgent(opts, facts, scope); err != nil {
			return nil, err
		}
	}
	p := &planner{
		opts:    opts,
		recipe:  r,
		facts:   facts,
		scope:   scope,
		base:    base,
		baseKey: slashPath(base),
		files:   map[string]*fileState{},
	}
	ledger, ledgerErr := readLedger(opts.UserBase)
	if ledgerErr == nil && !opts.Remove && scope == ScopeProject {
		if other := p.otherWorkbench(ledger); other != "" {
			return nil, contract.Refuse(contract.SetupOtherWorkbench, other)
		}
	}
	if ledgerErr != nil {
		return nil, contract.Refuse(contract.SetupUnreadableTarget, ledgerErr.Error())
	}
	p.ledger = ledger
	if opts.Remove {
		return p.remove()
	}
	return p.apply()
}

// firstOf returns the first non-empty value.
func firstOf(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// resolveRecipe finds and reads the recipe a run names, which is rows 2 to 5
// of the precondition list.
func resolveRecipe(opts Options) (*Recipe, error) {
	if opts.RecipeDir != "" {
		dir, err := filepath.Abs(opts.RecipeDir)
		if err != nil {
			return nil, contract.Refuse(contract.UnknownPath, opts.RecipeDir)
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return nil, contract.Refuse(contract.UnknownPath, dir)
		}
		r, err := readRecipe(os.DirFS(dir), filepath.Base(dir), SourcePath, dir)
		if err != nil {
			return nil, contract.Refuse(contract.MalformedRecipe, err.Error())
		}
		return r, nil
	}
	if !bench.HarnessName(opts.Harness) {
		return nil, contract.Refuse(contract.MalformedHarness, opts.Harness)
	}
	projectScope := opts.Scope == "" || opts.Scope == ScopeProject
	for _, p := range places(containerOf(opts.Workbench), opts.UserBase, projectScope) {
		if !p.holds(opts.Harness) {
			continue
		}
		r, err := p.open(opts.Harness)
		if err != nil {
			return nil, contract.Refuse(contract.MalformedRecipe, err.Error())
		}
		return r, nil
	}
	return nil, contract.Refuse(contract.UnknownRecipe, opts.Harness)
}

// containerOf is the .dinah directory holding a workbench, or empty when the
// workbench does not stand directly inside one.
func containerOf(workbench string) string {
	if workbench == "" {
		return ""
	}
	parent := filepath.Dir(workbench)
	if filepath.Base(parent) != bench.UserBaseName {
		return ""
	}
	return parent
}

// Container is the .dinah directory holding a workbench, for the command's
// listing, or empty when the workbench does not stand directly inside one.
func Container(workbench string) string {
	return containerOf(workbench)
}

// misplacedFlag names the first flag given beside another it does not belong
// with, which is row 9 of the precondition list.
func misplacedFlag(opts Options, scope string) string {
	switch {
	case opts.AllowRun && opts.Remove:
		return "--allow-run"
	case opts.Target != "" && scope == ScopeUser:
		return "--target"
	case opts.TrustProjectRecipe && opts.RecipeDir != "":
		return "--trust-project-recipe"
	case opts.TrustProjectRecipe && scope == ScopeUser:
		return "--trust-project-recipe"
	}
	if !opts.Remove {
		return ""
	}
	identity := []struct{ flag, value string }{
		{"--agent", opts.Agent},
		{"--tools", opts.Tools},
		{"--provider", opts.Provider},
		{"--model", opts.Model},
		{"--server", opts.Server},
	}
	for _, given := range identity {
		if given.value != "" {
			return given.flag
		}
	}
	return ""
}

// runListing is the detail of the run-not-allowed refusal, one run step per
// line, each rendered with the facts known by row 10. The scope's base and the
// workbench are not resolved until rows 11 and 13, so a placeholder naming
// one of them stands in the listing as the recipe wrote it.
func runListing(r *Recipe, scope string, opts Options, tools string) string {
	known := Facts{
		Harness:        r.Harness,
		Agent:          firstOf(opts.Agent, r.Agent),
		Tools:          tools,
		Provider:       firstOf(opts.Provider, r.Provider),
		Model:          opts.Model,
		Server:         opts.Server,
		Scope:          scope,
		Base:           "{{base}}",
		Workbench:      "{{workbench}}",
		WorkbenchTitle: "{{workbench_title}}",
		Recipe:         r.Name,
	}
	var lines []string
	for _, step := range r.Steps {
		if step.Scope != scope || step.Kind != KindRun {
			continue
		}
		program, args := renderCommand(step, known)
		lines = append(lines, step.ID+": "+CommandLine(program, args))
	}
	return strings.Join(lines, "\n")
}

// refuseListing raises a refusal whose subject is a list, one member per line.
// The list rides as the detail, which the machine form carries, and as the
// named value the refusal's shape prints as rows beneath its sentence.
func refuseListing(name, value, lines string) *contract.Refusal {
	return contract.RefuseWith(name, lines, map[string]string{value: lines})
}

// resolveBase finds the directory a scope writes into, which is row 13 of the
// precondition list.
func resolveBase(opts Options, scope string) (string, error) {
	if scope == ScopeUser {
		return opts.Home, nil
	}
	var base string
	detail := opts.Workbench
	if opts.Target != "" {
		abs, err := filepath.Abs(opts.Target)
		if err != nil {
			return "", contract.Refuse(contract.UnknownPath, opts.Target)
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return "", contract.Refuse(contract.UnknownPath, abs)
		}
		base = abs
		detail = abs
	} else {
		container := containerOf(opts.Workbench)
		if container == "" {
			return "", contract.Refuse(contract.SetupNoTarget, opts.Workbench)
		}
		base = filepath.Dir(container)
	}
	if homeOrAbove(base, opts.Home) {
		return "", contract.Refuse(contract.SetupNoTarget, detail)
	}
	return base, nil
}

// homeOrAbove reports whether a directory is the home directory or one of its
// ancestors, compared segment by segment after both are resolved to the path
// the filesystem finally reaches, so a junction or a symbolic link to the home
// counts as the home. A directory that cannot be resolved counts as the home,
// because a check that cannot answer must not let the write through. A home
// that cannot be resolved is compared as it is spelled.
func homeOrAbove(dir, home string) bool {
	if home == "" {
		return false
	}
	resolvedDir, err := finalPath(dir)
	if err != nil {
		return true
	}
	resolvedHome, err := finalPath(home)
	if err != nil {
		resolvedHome = filepath.Clean(home)
	}
	return within(resolvedDir, resolvedHome)
}

// within reports whether a path lies at or under a directory, segment by
// segment. filepath.Rel is lexical, and on Windows it compares volume and
// segments without regard to case, as the filesystem does.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// checkValues refuses a value setup would write that is not one word, which
// is row 14 of the precondition list.
func checkValues(opts Options, facts Facts) error {
	values := []struct {
		flag, value string
		given       bool
	}{
		{"--agent", facts.Agent, true},
		{"--provider", facts.Provider, true},
		{"--model", facts.Model, opts.Model != ""},
		{"--server", facts.Server, opts.Server != ""},
	}
	for _, v := range values {
		if v.given && !oneWord(v.value) {
			return contract.Refuse(contract.Malformed, v.flag+" "+v.value)
		}
	}
	return nil
}

// checkAgent refuses an agent name that is the operator's own, which is row
// 15 of the precondition list.
func checkAgent(opts Options, facts Facts, scope string) error {
	which := ""
	switch {
	case scope == ScopeProject && opts.Operator != "" && facts.Agent == opts.Operator:
		which = "setup.operator.workbench"
	case opts.ConfiguredActor != "" && facts.Agent == opts.ConfiguredActor:
		which = "setup.operator.config"
	}
	if which == "" {
		return nil
	}
	described := which
	if opts.Describe != nil {
		described = opts.Describe(which)
	}
	return contract.Refuse(contract.SetupAgentIsOperator, facts.Agent+" ("+described+")")
}

// slashPath is a path absolute, cleaned and written with forward slashes,
// which is how the ledger and the machine form spell one.
func slashPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// fileState is one file a run reads, and what the run would leave in it.
type fileState struct {
	// abs is the file's absolute path; rel is its path under the base, with
	// forward slashes.
	abs, rel string
	// existed says the file was there before the run.
	existed bool
	// data is the file's bytes as the run has left them so far, and exists
	// says the run leaves a file there at all.
	data   []byte
	exists bool
	// created says this run created the file, or an earlier run of this
	// recipe did.
	created bool
	// unreadable is why the file cannot be read, empty when it can.
	unreadable string
	// onDisk and onDiskExists are what the file holds on disk now, which a
	// write compares against so that a step changing nothing writes nothing.
	onDisk       []byte
	onDiskExists bool
}

// snapshot is a file's state after one step, which is what the step writes.
type snapshot struct {
	file   *fileState
	data   []byte
	exists bool
}

// plannedStep is one step as the plan will carry it out.
type plannedStep struct {
	step Step
	// writes are the files the step leaves changed, in order.
	writes []snapshot
	// changes are the step's rows of the report.
	changes []Change
	// entries are the ledger entries the step records.
	entries []ledgerEntry
	// program and args are a run step's, rendered.
	program string
	args    []string
}

// planner works out every change a run makes before any is made.
type planner struct {
	opts    Options
	recipe  *Recipe
	facts   Facts
	scope   string
	base    string
	baseKey string
	ledger  *ledger
	files   map[string]*fileState
	// produced are the locations the current steps produce, by file and key.
	produced map[[2]string]bool
	// unreadable is the first file that cannot be read, which row 17 names.
	unreadable string
	// conflicts are every location somebody else wrote, which row 18 names.
	conflicts []string
	steps     []*plannedStep
	// parents are the parent objects this run created, by file.
	parents map[string][]string
}

// otherWorkbench names the workbench this recipe, scope and base were already
// set up for, when that is not the one this run resolved.
func (p *planner) otherWorkbench(l *ledger) string {
	current := slashPath(p.opts.Workbench)
	for _, e := range l.ofRun(p.recipe.Name, p.scope, p.baseKey) {
		if e.Workbench != "" && e.Workbench != current {
			return filepath.FromSlash(e.Workbench)
		}
	}
	return ""
}

// file loads a file the run touches, once, after checking that its path does
// not leave the base.
func (p *planner) file(rel string) *fileState {
	abs := filepath.Join(p.base, filepath.FromSlash(rel))
	if f, loaded := p.files[abs]; loaded {
		return f
	}
	f := &fileState{abs: abs, rel: rel}
	p.files[abs] = f
	if !p.contained(abs) {
		f.unreadable = "resolves to a place outside " + p.base
		p.noteUnreadable(f)
		return f
	}
	data, err := os.ReadFile(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		f.unreadable = "cannot be read (" + unwrapPathError(err).Error() + ")"
		p.noteUnreadable(f)
	default:
		f.existed = true
		f.exists = true
		f.data = data
		f.onDisk = data
		f.onDiskExists = true
	}
	return f
}

// fileByKey loads a file the ledger names by its absolute forward-slash path.
func (p *planner) fileByKey(key string) *fileState {
	abs := filepath.FromSlash(key)
	rel, err := filepath.Rel(p.base, abs)
	if err != nil {
		rel = abs
	}
	return p.file(filepath.ToSlash(rel))
}

// contained reports whether a path stays under the base. The longest prefix
// of the path that exists is resolved by finalPath, which resolves every
// component of it, links and junctions alike, to the place the filesystem
// finally reaches; the components below it do not exist yet and are created
// as plain directories. A prefix or a base that cannot be resolved, such as a
// link whose target is missing, is refused rather than guessed at.
func (p *planner) contained(abs string) bool {
	resolvedBase, err := finalPath(p.base)
	if err != nil {
		return false
	}
	existing := abs
	var rest []string
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return false
		}
		rest = append([]string{filepath.Base(existing)}, rest...)
		existing = parent
	}
	resolved, err := finalPath(existing)
	if err != nil {
		return false
	}
	full := filepath.Join(append([]string{resolved}, rest...)...)
	return within(resolvedBase, full)
}

// noteUnreadable records the first file that cannot be read.
func (p *planner) noteUnreadable(f *fileState) {
	if p.unreadable == "" {
		p.unreadable = f.rel + ": " + f.unreadable
	}
}

// markUnreadable records a defect found while reading a file's content.
func (p *planner) markUnreadable(f *fileState, defect string) {
	if f.unreadable == "" {
		f.unreadable = defect
	}
	p.noteUnreadable(f)
}

// conflict records a location somebody else wrote.
//
// A whole file has the empty key, and its line is the file alone, since a
// line ending in a space reads the same as one that does not and leaves the
// trailing space for a reader's terminal to carry.
func (p *planner) conflict(f *fileState, key string) {
	if key == "" {
		p.conflicts = append(p.conflicts, f.rel)
		return
	}
	p.conflicts = append(p.conflicts, f.rel+" "+key)
}

// entryFor is the ledger's entry for a location of this run.
func (p *planner) entryFor(f *fileState, key string) *ledgerEntry {
	return p.ledger.find(p.recipe.Name, p.scope, p.baseKey, slashPath(f.abs), key)
}

// newEntry builds a ledger entry for a location this run owns.
func (p *planner) newEntry(f *fileState, kind, key, digest string) ledgerEntry {
	workbench := ""
	if p.scope == ScopeProject {
		workbench = slashPath(p.opts.Workbench)
	}
	return ledgerEntry{
		Recipe:    p.recipe.Name,
		Scope:     p.scope,
		Base:      p.baseKey,
		File:      slashPath(f.abs),
		Kind:      kind,
		Key:       key,
		Digest:    digest,
		Workbench: workbench,
	}
}

// fileChange is the token for a location added to a file: create when the run
// created the file, and add when it was already there.
func fileChange(f *fileState) string {
	if f.existed {
		return ChangeAdd
	}
	return ChangeCreate
}

// renderPath renders a step's path and validates the result.
func (p *planner) renderPath(step Step) (string, error) {
	rendered, err := renderText(step.Path, p.facts, false)
	if err != nil {
		return "", err
	}
	if err := checkStepPath(rendered); err != nil {
		return "", err
	}
	return rendered, nil
}

// apply plans every step of the scope, refuses what rows 17 and 18 refuse,
// and then carries the plan out unless the run is a dry run.
func (p *planner) apply() (*Report, error) {
	p.produced = map[[2]string]bool{}
	for _, step := range p.recipe.Steps {
		if step.Scope != p.scope {
			continue
		}
		planned := &plannedStep{step: step}
		p.steps = append(p.steps, planned)
		if step.Kind == KindRun {
			p.planRun(planned)
			continue
		}
		rel, err := p.renderPath(step)
		if err != nil {
			return nil, contract.Refuse(contract.MalformedRecipe, stepsFile+": step "+step.ID+" "+err.Error())
		}
		f := p.file(rel)
		if f.unreadable != "" {
			continue
		}
		switch step.Kind {
		case KindJSONMerge:
			p.planJSONMerge(planned, f)
		case KindMarkedSection:
			p.planSection(planned, f)
		case KindWriteFile:
			p.planWriteFile(planned, f)
		}
	}
	stale := &plannedStep{}
	p.planStale(stale)
	if p.unreadable != "" {
		return nil, contract.Refuse(contract.SetupUnreadableTarget, p.unreadable)
	}
	if len(p.conflicts) > 0 {
		return nil, refuseListing(contract.SetupConflict, "locations", strings.Join(p.conflicts, "\n"))
	}
	p.steps = append(p.steps, stale)
	p.recordCreation()
	report := p.report(p.recipe.Prompt)
	if p.opts.DryRun {
		for _, planned := range p.steps {
			report.Changes = append(report.Changes, planned.changes...)
		}
		return report, nil
	}
	return p.execute(report)
}

// planRun records a run step's rendered command.
func (p *planner) planRun(planned *plannedStep) {
	planned.program, planned.args = renderCommand(planned.step, p.facts)
	change := ChangeRan
	if p.opts.DryRun {
		change = ChangeWouldRun
	}
	planned.changes = append(planned.changes, Change{
		Step:   planned.step.ID,
		Kind:   KindRun,
		Key:    CommandLine(planned.program, planned.args),
		Change: change,
	})
}

// planJSONMerge plans every owned member of a json-merge step.
func (p *planner) planJSONMerge(planned *plannedStep, f *fileState) {
	tokens, _ := splitPointer(planned.step.Pointer)
	rendered, _ := renderValue(planned.step.value, p.facts)
	for _, member := range rendered.members {
		key := joinPointer(append(append([]string{}, tokens...), member.name))
		p.produced[[2]string{slashPath(f.abs), key}] = true
		desired := member.value.compact()
		change, ok := p.mergeMember(f, tokens, member.name, key, desired)
		if !ok {
			continue
		}
		change.Step = planned.step.ID
		planned.changes = append(planned.changes, change)
		planned.writes = append(planned.writes, snapshot{file: f, data: f.data, exists: f.exists})
		planned.entries = append(planned.entries, p.newEntry(f, entryJSONMember, key, digestOf(desired)))
	}
}
