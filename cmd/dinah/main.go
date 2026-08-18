// Command dinah keeps work moving: a single-seat, local tool that gives an
// agent or a person the coordination discipline of a board with no board.
//
// The binary is two heads over one library. The cli head renders the library's
// canonical answers for a person and emits them unrendered under --json; the
// mcp head passes the same answers through to an agent. Neither head computes
// a refusal, an ordering or an instruction composition of its own.
package main

import (
	"io"
	"os"
	"sort"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// session is one invocation: where it writes, who it acts as, which bench it
// found and which language it renders in.
type session struct {
	// out and errw are the two streams. The refusal name leads errw on
	// every non-zero outcome, so a script reads the reason with cut.
	out  io.Writer
	errw io.Writer
	// in is where a command reading its argument from a pipe finds it.
	in io.Reader
	// r renders every string a person reads.
	r *msg.Renderer
	// json says the canonical machine form is wanted rather than a
	// rendering.
	json bool
	// quiet suppresses the served instructions on the human rendering of a
	// claim and a move. The machine forms always carry them.
	quiet bool
	// home is the user base.
	home string
	// nativeHome is the machine's own home directory, which DINAH_HOME does
	// not move. Discovery bounds its ancestor walk there.
	nativeHome string
	// cfg is the user's own settings.
	cfg *bench.Config
	// actor is the owner resolved by the ladder, empty when no layer
	// carried one. It is empty rather than refused here, because the
	// refusal belongs inside the verb's own order.
	actor string
	// benchFlag is the bench named by --workbench or DINAH_WORKBENCH.
	benchFlag string
	// benchFlagSource names which of the two named it, SourceFlag or
	// SourceEnvironment, empty when neither did.
	benchFlagSource string
	// cwd is where bench discovery starts.
	cwd string
	// workbenchSource names the rung that resolved the active workbench for
	// this invocation, set by open() once discovery has run.
	workbenchSource string
	// width is how many columns the window gives, zero when no documented
	// source answers and the layout is then unbounded. It is resolved once
	// per invocation, so every row of one run is laid out against one width.
	width int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main with its streams and arguments passed in, so a test drives the
// whole head without building or exec-ing the binary.
func run(argv []string, in io.Reader, out, errw io.Writer) int {
	valued := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
	}
	parsed, err := parseArgs(argv, valued)
	if err != nil {
		return reportError(errw, msg.For(msg.Base), err)
	}
	cwd, wdErr := os.Getwd()
	if wdErr != nil {
		cwd = "."
	}
	home := bench.Home()
	cfg := bench.LoadConfig(home)
	benchFlag, benchFlagSource := bench.Resolve(
		bench.Layer{Source: bench.SourceFlag, Value: parsed.value("workbench")},
		bench.Layer{Source: bench.SourceEnvironment, Value: os.Getenv("DINAH_WORKBENCH")},
	)
	s := &session{
		out:             out,
		errw:            errw,
		in:              in,
		r:               msg.For(bench.ResolveLang(parsed.value("lang"), cfg)),
		json:            parsed.has("json") || os.Getenv("DINAH_FORMAT") == "json",
		quiet:           parsed.has("quiet"),
		home:            home,
		nativeHome:      bench.NativeHome(),
		cfg:             cfg,
		benchFlag:       benchFlag,
		benchFlagSource: benchFlagSource,
		cwd:             cwd,
		width:           windowWidth(),
	}
	if actor, err := bench.ResolveActor(parsed.value("actor"), cfg); err == nil {
		s.actor = actor
	}
	if len(parsed.positional) == 0 {
		s.write(s.helpBlock())
		return 0
	}
	name := parsed.positional[0]
	command, ok := lookup(name)
	if !ok {
		return s.fail(contract.UnknownVerb, name)
	}
	// resolveOpenTailFlags exists to decide whether a flag-shaped word
	// sitting inside a multi-word free-text zone is prose or a flag
	// (dinah-96). add, block, and comment have no such zone left to be mid
	// of: dinah-100 bounds each to exactly one free-text word, so a domain
	// flag typed anywhere is applied by parseArgs itself, correctly, with
	// nothing here left to correct. config declares no domain flag of its
	// own and keeps calling this function only to splice an unrecognized
	// flag-shaped word back into its value as literal text.
	if command.name != "add" && command.name != "block" && command.name != "comment" {
		if refusal := resolveOpenTailFlags(parsed, command); refusal != nil {
			return s.reportError(refusal)
		}
	}
	if word := unreadWordIn(command, parsed.rest()); word != "" {
		return s.fail(contract.Usage, word)
	}
	return command.run(s, parsed)
}

// unreadWordIn reports the first word in a command's own arguments that the
// command never reads, empty when none does. Within the declared bounded
// count, a word is the command's own argument, so only its shape is checked:
// looksLikeMistypedFlag catches a word that looks like a mistyped long flag,
// and anything else is left for the command and its domain check to read and
// validate. Past the bounded count, a command declaring an open tail has
// nothing more checked, since everything from there onward is free text the
// caller composed on purpose, dash or no dash. A command declaring no open
// tail reads none of what comes after its bounded count, so any word there is
// unread by construction and is returned regardless of shape, dash-led or
// plain; this is what closes the zero-bounded case (a stray word anywhere
// after the command name) and what closes it the same way for a command a
// caller overshoots past its own declared arity.
func unreadWordIn(c *command, words []string) string {
	for i, word := range words {
		if i < c.bounded {
			if looksLikeMistypedFlag(word) {
				return word
			}
			continue
		}
		if c.openTail {
			return ""
		}
		return word
	}
	return ""
}

// write puts a block of text on stdout, adding the trailing newline a block
// composed of lines does not carry.
func (s *session) write(text string) {
	io.WriteString(s.out, text)
	if !strings.HasSuffix(text, "\n") {
		io.WriteString(s.out, "\n")
	}
}

// line puts one already-rendered line on stdout.
func (s *session) line(text string) {
	io.WriteString(s.out, text+"\n")
}

// fail reports a refusal and returns its exit code. The refusal name is the
// first whitespace-delimited token on stderr, followed by the sentence a
// person reads, which is the contract the plumbing guarantee rests on.
func (s *session) fail(name, detail string) int {
	io.WriteString(s.errw, name+" "+s.sentence(name, detail)+"\n")
	return contract.ExitCode(contract.OutcomeRefused)
}

// sentence renders the prose that follows a refusal name.
func (s *session) sentence(name, detail string) string {
	return refusalSentence(s.r, name, detail, nil)
}

// refusalSentence renders the prose that follows a refusal name, filling the
// named values the catalog entry references beyond the detail.
//
// A malformed refusal raised over a file on disk carries the path it was
// raised over, and the location and the repair reach the reader as two
// fragments spliced on here rather than as a second template. The name cannot
// split, because CORE-OUT-5 gives one name to a broken definition and to a
// request missing what the definition demands, so the difference rides on
// whether a path is present.
//
// A usage refusal raised because a word looked like an unknown or
// value-starved flag carries dashHint, and the fragment naming the "--"
// end-of-options marker is spliced on the same way. Only parseArgs's two
// flag-scan refusals ever set dashHint; every other dinah.usage site is
// unchanged.
//
// A multiple-words refusal (freeText, cmd/dinah/args.go) carries either
// example, when the free text has no quotation mark and a rebuilt,
// paste-ready command line reads back correctly in bash, cmd.exe and
// PowerShell alike, or quoteInText, when it does and no single escaping is
// correct in all three, so the caller is asked to quote the text themselves
// instead. Exactly one of the two is ever set.
func refusalSentence(r *msg.Renderer, name, detail string, extra map[string]string) string {
	key := "refusal." + name
	if !r.Has(key) {
		return r.T("refusal.unknown", "name", name, "detail", detail)
	}
	pairs := []string{"detail", detail}
	for _, named := range sortedKeys(extra) {
		pairs = append(pairs, named, extra[named])
	}
	text := r.T(key, pairs...)
	if name == contract.Malformed && extra["path"] != "" {
		return text + r.T("refusal.malformed.at", "path", extra["path"]) + r.T("refusal.malformed.fix")
	}
	if name == contract.Usage && extra["dashHint"] != "" {
		return text + r.T("refusal.dinah.usage.dash-hint")
	}
	if name == contract.MultipleWords {
		if extra["quoteInText"] != "" {
			return text + r.T("refusal.dinah.multiple-words.quote-yourself")
		}
		return text + r.T("refusal.dinah.multiple-words.example", "example", extra["example"])
	}
	return text
}

// sortedKeys returns a map's keys in order, so that one refusal renders the
// same way twice however the map was built.
func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// refusalReport is the machine form of a refusal raised before any verb ran.
// It carries the members a verb's own refusal response carries, so a caller
// parsing --json reads one shape whichever layer said no.
type refusalReport struct {
	// Outcome is refused, which is the only outcome this form reports.
	Outcome string `json:"outcome"`
	// Refusal is the refusal name.
	Refusal string `json:"refusal"`
	// Detail names what the refusal was about.
	Detail string `json:"detail,omitempty"`
	// Context carries the refusal's named values as data, absent on a
	// refusal that needs none.
	Context map[string]string `json:"context,omitempty"`
	// Workbenches carries the candidates a dinah.ambiguous-workbench refusal
	// found, the same rows dinah workbenches would print for this same
	// invocation, so a script reads them as structured fields rather than
	// splitting a prose string. Every other refusal leaves this nil, and
	// omitempty drops it.
	Workbenches []bench.Candidate `json:"workbenches,omitempty"`
}

// reportError turns an error from a layer below into a report on stderr and
// an exit code, without a session, for the failures that happen before one
// can be built. It writes text alone, because the flags that would have
// carried --json are what failed to parse.
func reportError(errw io.Writer, r *msg.Renderer, err error) int {
	if refusal, ok := err.(*contract.Refusal); ok {
		sentence := refusalSentence(r, refusal.Name, refusal.Detail, refusal.Extra)
		io.WriteString(errw, refusal.Name+" "+sentence+"\n")
		return contract.ExitCode(contract.OutcomeRefused)
	}
	io.WriteString(errw, contract.OutcomeUnreachable+" "+err.Error()+"\n")
	return contract.ExitCode(contract.OutcomeUnreachable)
}

// reportError is the same report from inside a session, which is where every
// discovery-time failure is raised. The machine form reaches stdout first
// under --json, and the sentence follows it on stderr, exactly as a verb's own
// refusal is emitted.
func (s *session) reportError(err error) int {
	refusal, ok := err.(*contract.Refusal)
	if ok && refusal.Name == contract.AmbiguousWorkbench {
		return s.reportAmbiguousWorkbench(refusal)
	}
	if ok && s.json {
		s.emitJSON(refusalReport{
			Outcome: contract.OutcomeRefused,
			Refusal: refusal.Name,
			Detail:  refusal.Detail,
			Context: refusal.Extra,
		})
	}
	return reportError(s.errw, s.r, err)
}

// reportAmbiguousWorkbench reports dinah.ambiguous-workbench by pairing it
// with the rows the workbenches listing would print for this same
// invocation, since bench.Reachable shares Discover's own walk and cannot
// name a different set of candidates. The rows are re-fetched rather than
// carried on the refusal, so no plumbing beyond the base directory travels
// through contract.Refusal for what is, today, the one refusal that wants
// this.
//
// The text form prints an opening sentence naming the base, the candidate
// rows beneath it in the shape dinah workbenches renders them, and a closing
// line naming the two ways forward. The machine form drops the now-redundant
// detail string and carries the same rows as a workbenches array instead.
func (s *session) reportAmbiguousWorkbench(refusal *contract.Refusal) int {
	rows, _ := bench.Reachable(s.cwd, s.benchFlag, s.home, s.nativeHome)
	if s.json {
		s.emitJSON(refusalReport{
			Outcome:     contract.OutcomeRefused,
			Refusal:     refusal.Name,
			Context:     refusal.Extra,
			Workbenches: rows,
		})
		return contract.ExitCode(contract.OutcomeRefused)
	}
	io.WriteString(s.errw, refusal.Name+" "+s.r.T("refusal."+refusal.Name, "base", refusal.Extra["base"])+"\n")
	for _, row := range s.formatCandidateRows(rows) {
		io.WriteString(s.errw, row+"\n")
	}
	io.WriteString(s.errw, s.r.T("refusal."+refusal.Name+".next")+"\n")
	return contract.ExitCode(contract.OutcomeRefused)
}

// open discovers and opens the bench this invocation serves.
func (s *session) open() (*verb.Library, error) {
	root, source, passed, err := bench.DiscoverSource(
		s.cwd,
		s.benchFlag,
		s.benchFlagSource,
		s.home,
		s.nativeHome,
		s.cfg.Get("workbench"),
	)
	if err != nil {
		return nil, err
	}
	s.workbenchSource = source
	opened, err := bench.Open(root)
	if err != nil {
		return nil, err
	}
	opened.Passed = passed
	return verb.New(opened, s.home), nil
}

// withBench opens the bench and hands it to a command, reporting the failure
// as a refusal when the bench cannot be opened at all.
func (s *session) withBench(do func(*verb.Library) int) int {
	library, err := s.open()
	if err != nil {
		return s.reportError(err)
	}
	return do(library)
}
