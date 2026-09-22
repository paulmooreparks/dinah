package main

import (
	"errors"
	"os"
	"strconv"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/setup"
	"dinah/internal/verb"
)

// runSetup connects a harness to a workbench by applying a recipe, takes back
// what an earlier run wrote, or lists the recipes setup can find.
//
// Setup is not an act on a workbench. It writes no journal line and resolves
// no caller actor, so the global --actor does not reach it; the name it writes
// is the agent's, which is --agent. The workbench it resolves is the one
// every command resolves, and a discovery refusal is held back until the
// check that needs a workbench, since a user-scope run and a listing need
// none.
func runSetup(s *session, parsed *arguments) int {
	harness := at(parsed.rest(), 0)
	list := parsed.has("list")
	recipeDir := parsed.value("recipe")
	if refusal := setupShape(parsed, harness, list, recipeDir); refusal != nil {
		return s.reportError(refusal)
	}
	workbench, title, operator, discovered := s.setupWorkbench()
	userBase := bench.UserBase(s.home)
	if list {
		return s.emitSetupList(setup.List(setup.Container(workbench), userBase))
	}
	opts := setup.Options{
		Harness:            harness,
		RecipeDir:          recipeDir,
		Agent:              parsed.value("agent"),
		Tools:              parsed.value("tools"),
		Scope:              parsed.value("scope"),
		Target:             parsed.value("target"),
		Provider:           parsed.value("provider"),
		Model:              parsed.value("model"),
		Server:             parsed.value("server"),
		TrustProjectRecipe: parsed.has("trust-project-recipe"),
		AllowRun:           parsed.has("allow-run"),
		DryRun:             parsed.has("dry-run"),
		Remove:             parsed.has("remove"),
		Home:               s.nativeHome,
		UserBase:           userBase,
		DinahHome:          os.Getenv("DINAH_HOME"),
		Workbench:          workbench,
		WorkbenchTitle:     title,
		Operator:           operator,
		WorkbenchErr:       discovered,
		ConfiguredActor:    s.cfg.Get("actor"),
		Describe:           func(key string) string { return s.r.T(key) },
		Programs:           s.errw,
	}
	report, err := setup.Run(opts)
	var failed *setup.StepFailed
	if errors.As(err, &failed) {
		return s.emitSetupFailure(failed)
	}
	if err != nil {
		return s.reportError(err)
	}
	return s.emitSetupReport(report)
}

// setupShape is row 1 of setup's precondition list, together with the part of
// row 9 a listing owns: the invocation names a harness, --list or --recipe,
// and exactly one of them, and a listing carries no other argument of setup's
// own.
func setupShape(parsed *arguments, harness string, list bool, recipeDir string) *contract.Refusal {
	named := []string{}
	if harness != "" {
		named = append(named, harness)
	}
	if list {
		named = append(named, "--list")
	}
	if recipeDir != "" {
		named = append(named, "--recipe")
	}
	if len(named) == 0 {
		return contract.Refuse(contract.Usage, "setup")
	}
	if len(named) > 1 {
		return contract.Refuse(contract.Usage, named[1])
	}
	if !list {
		return nil
	}
	for _, param := range verb.Params("setup") {
		if !param.Flag || param.Name == "list" {
			continue
		}
		if parsed.has(param.Name) {
			return contract.Refuse(contract.Usage, "--"+param.Name)
		}
	}
	return nil
}

// setupWorkbench resolves the workbench this invocation stands in and reads
// its title and operator, answering discovery's refusal rather than raising
// it.
func (s *session) setupWorkbench() (root, title, operator string, refused error) {
	root, _, _, err := s.discoverRoot()
	if err != nil {
		return "", "", "", err
	}
	opened, err := bench.Open(root)
	if err != nil {
		return "", "", "", err
	}
	return root, opened.Title, opened.Operator, nil
}

// setupListing is the machine form of dinah setup --list.
type setupListing struct {
	Recipes []setup.Listing `json:"recipes"`
}

// emitSetupList answers dinah setup --list in whichever form was asked for.
func (s *session) emitSetupList(rows []setup.Listing) int {
	if s.format != formatHuman {
		return s.emitMachine(setupListing{Recipes: rows})
	}
	listing := table{indent: 2, columns: s.columns("setup-list", "recipe", "title", "source", "used", "path")}
	for _, row := range rows {
		title := row.Title
		if row.Malformed != "" {
			title = s.r.T("setup.list.malformed")
		}
		path := row.Path
		if path == "" {
			path = "-"
		}
		fields := []string{row.Name, title, row.Source, s.yesNo(row.Used), path}
		listing.rows = append(listing.rows, tableRow{fields: fields})
	}
	s.table(listing)
	return 0
}

// emitSetupReport answers an apply, a dry run or a removal.
func (s *session) emitSetupReport(report *setup.Report) int {
	if s.format != formatHuman {
		return s.emitMachine(report)
	}
	s.renderSetupReport(report)
	if report.Nothing {
		return 0
	}
	if report.DryRun {
		s.renderSetupBlocks(report)
	}
	if count := report.Count(setup.ChangeRan); count > 0 {
		s.line("")
		s.line(s.r.T("setup.run.unverified", "count", strconv.Itoa(count)))
	}
	if count := report.Count(setup.ChangeWouldRun); count > 0 {
		s.line("")
		s.line(s.r.T("setup.run.unverified.dry-run", "count", strconv.Itoa(count)))
	}
	if report.Prompt == "" {
		return 0
	}
	s.line("")
	if report.Recipe.Source == setup.SourceProject {
		s.line(s.r.T("setup.prompt.heading.project", "detail", report.Recipe.Path))
	} else {
		s.line(s.r.T("setup.prompt.heading"))
	}
	s.line("")
	s.write(report.Prompt)
	return 0
}

// renderSetupReport prints the heading and the change table, or the sentence
// saying a removal found nothing to take back.
func (s *session) renderSetupReport(report *setup.Report) {
	if report.DryRun {
		s.line(s.r.T("setup.dry-run"))
	}
	heading := s.r.T("setup.heading",
		"recipe", report.Recipe.Name,
		"source", report.Recipe.Source,
		"scope", report.Scope,
		"base", report.BaseDir,
	)
	s.line(heading)
	if report.Nothing {
		s.line(s.r.T("setup.remove.nothing"))
		return
	}
	changes := table{indent: 2, columns: s.columns("setup", "step", "file", "key", "change")}
	for _, c := range report.Changes {
		fields := []string{dashIfEmpty(c.Step), dashIfEmpty(c.File), dashIfEmpty(c.Key), s.r.T("setup.change." + c.Change)}
		changes.rows = append(changes.rows, tableRow{fields: fields})
	}
	s.table(changes)
}

// renderSetupBlocks prints a dry run's before and after of every location
// that would change, and the command line of every program that would run.
func (s *session) renderSetupBlocks(report *setup.Report) {
	for _, c := range report.Changes {
		if c.Change == setup.ChangeUnchanged {
			continue
		}
		s.line("")
		if c.Change == setup.ChangeWouldRun {
			s.line("=== " + c.Step + " would run: " + c.Key)
			continue
		}
		label := c.File
		if c.Key != "" {
			label += " " + c.Key
		}
		s.line("--- " + label + " (before)")
		s.write(absentIfEmpty(c.Before))
		s.line("+++ " + label + " (after)")
		s.write(absentIfEmpty(c.After))
	}
}

// dashIfEmpty prints a dash in a table cell that has nothing to say.
func dashIfEmpty(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// absentIfEmpty prints the marker a dry run shows for a location that does
// not exist.
func absentIfEmpty(value string) string {
	if value == "" {
		return "(absent)"
	}
	return value
}

// setupFailure is the machine form of a run step that failed: the standard
// refusal shape, the change records of every step that completed before it,
// and the failing step's id.
type setupFailure struct {
	refusalReport
	Changes    []setup.Change `json:"changes"`
	FailedStep string         `json:"failed_step"`
}

// emitSetupFailure answers a run stopped by a failing program. A person reads
// what completed and then the refusal; a script reads the refusal shape with
// the completed changes beside it.
func (s *session) emitSetupFailure(failed *setup.StepFailed) int {
	if s.format != formatHuman {
		failure := setupFailure{
			refusalReport: refusalReport{
				Outcome: contract.OutcomeRefused,
				Refusal: failed.Refusal.Name,
				Detail:  failed.Refusal.Detail,
			},
			Changes:    failed.Report.Changes,
			FailedStep: failed.Step,
		}
		s.emitMachine(failure)
		return s.writeRefusal(failed.Refusal)
	}
	s.renderSetupReport(failed.Report)
	return s.writeRefusal(failed.Refusal)
}
