package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/durable"
	"dinah/internal/verb"
)

// runTimeoutDefault bounds a run whose caller named no --timeout. An agent
// session that never returns would otherwise hold its claim for as long as
// the process lived, and an hour is longer than any one station's pass and
// short enough that an unattended loop notices a hung harness the same day.
const runTimeoutDefault = time.Hour

// runClaimMargin is how long a run's claim outlasts its timeout, which covers
// the bookkeeping after the agent returns and a stopped harness's pipes.
const runClaimMargin = 10 * time.Minute

// runWaitDelay is how long a run waits for a stopped harness's output pipes
// to close. A harness that started children of its own can leave them holding
// the pipes after the harness itself is stopped, and the run reports the
// timeout rather than waiting on them.
const runWaitDelay = 5 * time.Second

// runStderrLines is how many lines of a failed harness's standard error a run
// posts on the card, and runStderrBytes caps them, so a harness that printed
// a megabyte of trace leaves a comment a person can read.
const (
	runStderrLines = 20
	runStderrBytes = 4000
)

// runReport is what dinah run answers, in both its forms.
type runReport struct {
	// Card is the card's reference.
	Card string `json:"card"`
	// Column is the reference of the column the run worked in.
	Column string `json:"column"`
	// Recipe is the recipe the column named.
	Recipe string `json:"recipe"`
	// Resumed is true where the run continued a stored session.
	Resumed bool `json:"resumed"`
	// Session is the session the harness reported, empty where it reported
	// none.
	Session string `json:"session,omitempty"`
	// Outcome is what the agent reported, stay where it reported nothing.
	Outcome string `json:"outcome,omitempty"`
	// Reported is false where the agent's text carried no result block.
	Reported bool `json:"reported"`
	// Reason is the agent's reason.
	Reason string `json:"reason,omitempty"`
	// Applied is true where the outcome was carried out as the agent asked.
	Applied bool `json:"applied"`
	// Destination is the column the card was moved to, where it was moved.
	Destination string `json:"destination,omitempty"`
	// Failure names why the run stopped short, empty where it did not:
	// exit, timeout, start, receipt, error or refused.
	Failure string `json:"failure,omitempty"`
	// ExitCode is the harness's exit status where it exited.
	ExitCode int `json:"exit_code"`
	// Spend is the spend line the run recorded, absent where the recipe
	// records none.
	Spend *verb.RunSpent `json:"spend,omitempty"`
}

// The failures a run report names. They are machine vocabulary and are not
// translated.
const (
	runFailedExit    = "exit"
	runFailedTimeout = "timeout"
	runFailedStart   = "start"
	runFailedReceipt = "receipt"
	runFailedError   = "error"
	runFailedRefused = "refused"
)

// runLaunched is what launching the harness produced.
type runLaunched struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	failure  string
	startErr error
}

// runRun runs one agent session on a card: it claims the card, composes the
// prompt once, launches the column's recipe, reads the harness's receipt,
// records the session and the spend, posts the agent's handoff and carries
// out the result the agent reported. Everything the run decides from the
// workbench is the library's; launching the process is this head's, which is
// why the command is a terminal command and no protocol head serves it.
func runRun(s *session, parsed *arguments) int {
	req := s.request(verb.Run, parsed)
	req.Card = at(parsed.rest(), 0)
	req.Timeout = runTimeoutDefault
	if given := parsed.value("timeout"); given != "" {
		timeout, err := verb.ParseDuration(given)
		if err != nil {
			return s.reportError(err)
		}
		if timeout <= 0 {
			return s.fail(contract.Malformed, "--timeout")
		}
		req.Timeout = timeout
	}
	return s.withBench(func(l *verb.Library) int {
		plan, refused := l.PlanRun(req)
		if refused != nil {
			return s.emit(refused)
		}
		claim := s.request(verb.Claim, parsed)
		claim.Card = plan.Ref
		// The claim expires a margin after the run's own timeout, so a run
		// killed between its claim and its release leaves a claim that lapses
		// on its own rather than one only a person can clear.
		claim.Expires = req.Timeout + runClaimMargin
		if response := l.Do(claim); response.Outcome != contract.OutcomeOK {
			return s.emit(response)
		}
		report := &runReport{
			Card:    plan.Ref,
			Column:  plan.Column.Ref(),
			Recipe:  plan.Recipe.Name,
			Resumed: plan.Resume,
		}
		launched := s.launchRun(l, req, plan)
		report.ExitCode = launched.exitCode
		read, readable := verb.ReadRunReceipt(launched.stdout, plan.Recipe.Receipt)
		if !readable {
			read = nil
		}
		if read != nil {
			report.Session = read.Session
		}
		spent, refused := l.RecordRun(req, plan, read)
		report.Spend = spent
		if refused != nil {
			s.reportOutcome(refused)
		}
		failure := launched.failure
		switch {
		case failure != "":
		case read == nil:
			failure = runFailedReceipt
		case read.Error:
			failure = runFailedError
		}
		if failure != "" {
			report.Failure = failure
			s.failRun(l, parsed, plan, report, launched)
			return s.finishRun(report)
		}
		outcome := verb.ParseRunOutcome(read.Text)
		report.Outcome, report.Reason, report.Reported = outcome.Outcome, outcome.Reason, outcome.Reported
		if handoff := runHandoff(outcome); handoff != "" {
			s.runComment(l, parsed, plan.Ref, handoff)
		}
		s.actOnOutcome(l, parsed, plan, outcome, report)
		return s.finishRun(report)
	})
}

// launchRun fills the recipe's templates and runs its command once, with the
// prompt on standard input.
func (s *session) launchRun(l *verb.Library, req *verb.Request, plan *verb.RunPlan) runLaunched {
	argv := plan.Recipe.Command
	if plan.Resume {
		argv = plan.Recipe.Resume
	}
	instructions := s.runInstructions(l, req, plan)
	inFile := false
	for _, word := range argv {
		if strings.Contains(word, "{instructions}") {
			inFile = true
		}
	}
	values := map[string]string{
		"{card}":      plan.Ref,
		"{column}":    plan.Column.Ref(),
		"{session}":   plan.Session,
		"{workbench}": l.Bench.Root,
	}
	if inFile {
		// The name carries the card and the moment, so two runs on one
		// machine never share a file, and the file goes when the run does.
		path := filepath.Join(os.TempDir(), "dinah-run-"+plan.Card.ID+"-"+strconv.FormatInt(time.Now().UnixNano(), 36)+".md")
		if err := durable.WriteFile(path, []byte(instructions), 0o600); err != nil {
			return runLaunched{failure: runFailedStart, startErr: err, exitCode: -1}
		}
		defer durable.Remove(path)
		values["{instructions}"] = path
	}
	filled := make([]string, len(argv))
	for i, word := range argv {
		filled[i] = runFill(word, values)
	}
	dir := runProjectDir(l.Bench.Root)
	if plan.Recipe.Cwd != "" {
		dir = runFill(plan.Recipe.Cwd, values)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(runProjectDir(l.Bench.Root), dir)
		}
	}
	prompt := s.runPrompt(l, req, plan, instructions, inFile)
	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, filled[0], filled[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "DINAH_WORKBENCH="+l.Bench.Root)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = runWaitDelay
	launched := runLaunched{}
	err := cmd.Run()
	launched.stdout, launched.stderr = stdout.Bytes(), stderr.Bytes()
	if cmd.ProcessState != nil {
		launched.exitCode = cmd.ProcessState.ExitCode()
	} else {
		launched.exitCode = -1
	}
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		launched.failure = runFailedTimeout
	case cmd.ProcessState == nil && err != nil:
		launched.failure = runFailedStart
		launched.startErr = err
	case launched.exitCode != 0:
		launched.failure = runFailedExit
	}
	return launched
}

// runFill replaces every placeholder a template carries.
func runFill(template string, values map[string]string) string {
	for placeholder, value := range values {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	return template
}

// runProjectDir is the directory a workbench serves: the one holding its
// .dinah container, or the workbench's own directory where it sits in none.
// It is where an agent is started unless the recipe names somewhere else,
// because it is where the work the card describes lives.
func runProjectDir(root string) string {
	container := filepath.Dir(root)
	if filepath.Base(container) == bench.UserBaseName {
		return filepath.Dir(container)
	}
	return root
}

// runInstructions is the instruction chain served at the card's position,
// each layer's text in the order the chain serves it.
func (s *session) runInstructions(l *verb.Library, req *verb.Request, plan *verb.RunPlan) string {
	ask := *req
	ask.Verb = "instructions"
	ask.Card = plan.Ref
	served, err := l.Instructions(&ask)
	if err != nil || served == nil {
		return ""
	}
	var layers []string
	for _, text := range []string{served.Instructions.Global, served.Instructions.Standing, served.Instructions.Column} {
		if text = strings.TrimSpace(text); text != "" {
			layers = append(layers, text)
		}
	}
	return strings.Join(layers, "\n\n") + "\n"
}

// runPrompt composes the prompt the agent reads: the card's brief, the
// instructions where the recipe does not take them as a file, and the closing
// paragraph saying how to report the result.
func (s *session) runPrompt(l *verb.Library, req *verb.Request, plan *verb.RunPlan, instructions string, inFile bool) string {
	var parts []string
	if brief := s.runBrief(l, req, plan); brief != "" {
		parts = append(parts, brief)
	}
	if !inFile && strings.TrimSpace(instructions) != "" {
		parts = append(parts, strings.TrimSpace(instructions))
	}
	parts = append(parts, s.r.T("run.prompt.outcome"))
	return strings.Join(parts, "\n\n") + "\n"
}

// runBrief renders the card's brief the way `dinah show --brief` prints it to
// a person, which is the opening read the brief was built to be.
func (s *session) runBrief(l *verb.Library, req *verb.Request, plan *verb.RunPlan) string {
	ask := *req
	ask.Verb = "show"
	ask.Card = plan.Ref
	ask.Brief = true
	detail, _, _, text, err := l.Show(&ask)
	if err != nil {
		return ""
	}
	if detail == nil {
		return strings.TrimSpace(text)
	}
	// The footer naming what the brief withheld is advice for a reader who
	// can ask again, and an agent told to issue no bookkeeping reads it as a
	// question to answer, so the prompt carries the brief without it.
	detail.Withheld, detail.Reread = nil, ""
	var out bytes.Buffer
	child := *s
	child.out, child.errw = &out, &bytes.Buffer{}
	child.format = formatHuman
	child.width, child.rawWidth = 0, 0
	child.renderDetail(detail)
	return strings.TrimSpace(out.String())
}

// runHandoff is the comment a run posts for what the agent wrote: its text
// before the result block, or its reason where it wrote nothing else.
func runHandoff(outcome verb.RunOutcome) string {
	if outcome.Handoff != "" {
		return outcome.Handoff
	}
	return outcome.Reason
}

// runComment posts one comment on the card as the run's owner, and reports a
// refusal on standard error without stopping the run, because the card still
// has to be moved or released whatever happened to the comment.
func (s *session) runComment(l *verb.Library, parsed *arguments, ref, text string) {
	comment := s.request("comment", parsed)
	comment.Card, comment.Text = ref, text
	if response := l.Comment(comment); response.Outcome != contract.OutcomeOK {
		s.reportOutcome(response)
	}
}

// runRelease gives the card back in place where this run's owner still holds
// it, reporting a refusal on standard error.
func (s *session) runRelease(l *verb.Library, parsed *arguments, ref string) {
	if found, err := l.Bench.ResolveCard(ref); err == nil && found.Card.Holder != s.actor {
		return
	}
	release := s.request(verb.Release, parsed)
	release.Card = ref
	if response := l.Do(release); response.Outcome != contract.OutcomeOK {
		s.reportOutcome(response)
	}
}

// failRun is the path a run takes when the harness did not deliver a result:
// it posts what went wrong with the tail of standard error, and releases the
// card where it stands.
func (s *session) failRun(l *verb.Library, parsed *arguments, plan *verb.RunPlan, report *runReport, launched runLaunched) {
	what := s.runFailureSentence(report, launched)
	text := s.r.T("run.comment.failed", "failure", what)
	if tail := runTail(launched.stderr); tail != "" {
		text += "\n\n```\n" + tail + "\n```"
	} else {
		text += " " + s.r.T("run.comment.no-stderr")
	}
	s.runComment(l, parsed, plan.Ref, text)
	s.runRelease(l, parsed, plan.Ref)
}

// runFailureSentence names a failure in a sentence a person reads.
func (s *session) runFailureSentence(report *runReport, launched runLaunched) string {
	switch report.Failure {
	case runFailedExit:
		return s.r.T("run.failed.exit", "code", strconv.Itoa(launched.exitCode))
	case runFailedTimeout:
		return s.r.T("run.failed.timeout")
	case runFailedStart:
		detail := ""
		if launched.startErr != nil {
			detail = launched.startErr.Error()
		}
		return s.r.T("run.failed.start", "error", detail)
	case runFailedError:
		return s.r.T("run.failed.error")
	}
	return s.r.T("run.failed.receipt")
}

// runTail is the last lines of a stream, capped in bytes.
func runTail(stream []byte) string {
	text := strings.TrimSpace(strings.ReplaceAll(string(stream), "\r\n", "\n"))
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > runStderrLines {
		lines = lines[len(lines)-runStderrLines:]
	}
	text = strings.Join(lines, "\n")
	if len(text) > runStderrBytes {
		text = text[len(text)-runStderrBytes:]
	}
	return strings.ReplaceAll(text, "```", "'''")
}

// actOnOutcome carries out what the agent reported: forward moves the card
// along its route, back moves it to the column's reject_to, block blocks it
// with the reason and stay releases it. A move with nowhere to go, or one the
// workbench refuses, releases the card and posts why, so a person sees where
// it stopped.
func (s *session) actOnOutcome(l *verb.Library, parsed *arguments, plan *verb.RunPlan, outcome verb.RunOutcome, report *runReport) {
	var target *bench.Column
	switch outcome.Outcome {
	case verb.RunStay:
		s.runRelease(l, parsed, plan.Ref)
		report.Applied = outcome.Reported
		return
	case verb.RunBlock:
		block := s.request(verb.Block, parsed)
		block.Card = plan.Ref
		block.Reason = outcome.Reason
		if block.Reason == "" {
			block.Reason = s.r.T("run.block.no-reason")
		}
		s.runAct(l, parsed, plan, block, report)
		return
	case verb.RunForward:
		card := plan.Card
		if current, err := l.Bench.ResolveCard(plan.Ref); err == nil {
			card = current.Card
		}
		target = bench.RouteForwardOf(l.Bench.RouteOf(card), plan.Column)
		if target == nil {
			s.runRefused(l, parsed, plan, report, s.r.T("run.comment.no-forward", "column", plan.Column.Ref()))
			return
		}
	case verb.RunBack:
		target = l.Bench.RejectTarget(plan.Column)
		if target == nil {
			s.runRefused(l, parsed, plan, report, s.r.T("run.comment.no-reject", "column", plan.Column.Ref()))
			return
		}
	}
	// The card is released before it moves, for two reasons. A held card is
	// refused entry to a column no owner takes work up at, which is where a
	// done column and an intake stand; and a move keeps the claim, where the
	// run's part is over and the next station's run has to find the card
	// ready.
	s.runRelease(l, parsed, plan.Ref)
	move := s.request(verb.Move, parsed)
	move.Card = plan.Ref
	move.Column = target.ID
	if s.runAct(l, parsed, plan, move, report) {
		report.Destination = target.Ref()
	}
}

// runAct performs one move or block and answers whether it landed. A refused
// act releases the card and posts the refusal's own sentence; a move reaches
// here with the card already released, and runRelease leaves a card this run
// no longer holds alone.
func (s *session) runAct(l *verb.Library, parsed *arguments, plan *verb.RunPlan, act *verb.Request, report *runReport) bool {
	response := l.Do(act)
	if response.Outcome == contract.OutcomeOK {
		report.Applied = true
		return true
	}
	s.runRefused(l, parsed, plan, report, strings.Join(s.outcomeLines(response), "\n"))
	return false
}

// runRefused releases the card and posts why the agent's result could not be
// carried out.
func (s *session) runRefused(l *verb.Library, parsed *arguments, plan *verb.RunPlan, report *runReport, why string) {
	report.Failure = runFailedRefused
	s.runComment(l, parsed, plan.Ref, s.r.T("run.comment.refused", "outcome", report.Outcome, "why", why))
	s.runRelease(l, parsed, plan.Ref)
}

// finishRun prints the report and answers the exit code: 0 where the agent's
// result was carried out, and 1 where the run stopped short and released the
// card, so a loop driving runs can tell the two apart without parsing.
func (s *session) finishRun(report *runReport) int {
	code := 0
	if report.Failure != "" {
		code = 1
	}
	if s.format != formatHuman {
		s.emitMachine(report)
		return code
	}
	if report.Resumed {
		s.line(s.r.T("run.session.resumed", "card", report.Card, "recipe", report.Recipe, "column", report.Column))
	} else {
		s.line(s.r.T("run.session.fresh", "card", report.Card, "recipe", report.Recipe, "column", report.Column))
	}
	switch {
	case report.Failure != "" && report.Failure != runFailedRefused:
		s.line(s.r.T("run.result.failed", "card", report.Card, "failure", report.Failure))
	case report.Failure == runFailedRefused:
		s.line(s.r.T("run.result.refused", "card", report.Card, "outcome", report.Outcome))
	case !report.Reported:
		s.line(s.r.T("run.result.unreported", "card", report.Card))
	case report.Destination != "":
		s.line(s.r.T("run.result.moved", "card", report.Card, "outcome", report.Outcome, "column", report.Destination))
	case report.Outcome == verb.RunBlock:
		s.line(s.r.T("run.result.blocked", "card", report.Card))
	default:
		s.line(s.r.T("run.result.stayed", "card", report.Card))
	}
	if spend := report.Spend; spend != nil {
		if spend.Figure != nil {
			s.line(s.r.T("run.result.spent", "figure", strconv.FormatFloat(*spend.Figure, 'f', -1, 64), "unit", spend.Unit))
		} else {
			s.line(s.r.T("run.result.unreported-spend", "unit", spend.Unit))
		}
	}
	return code
}
