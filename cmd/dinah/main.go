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
	// mcpRoot is the directory the mcp command was bound to, empty when mcp
	// was not the command that ran.
	mcpRoot string
	// cwd is where bench discovery starts.
	cwd string
	// workbenchSource names the rung that resolved the active workbench for
	// this invocation, set by open() once discovery has run.
	workbenchSource string
	// width is how many columns the window gives, zero when no documented
	// source answers and the layout is then unbounded. It is resolved once
	// per invocation, so every row of one run is laid out against one width.
	width int
	// command is the command word this invocation named, empty until one is
	// looked up. It is what a next-step sentence names when the refusal it
	// belongs to is raised from several commands, and what verb.Usage
	// composes the syntax line from.
	command string
	// library is the bench this invocation opened, nil until one is. The
	// composer reads the columns off it for the listing unknown-column prints.
	library *verb.Library
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
	parsed, parseErr := parseArgs(argv, valued)
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
	if parseErr != nil {
		// The parse failed, so the language ladder reads what parseArgs
		// managed to take apart before it stopped: a --lang written ahead
		// of the offending word is honoured, and one written after it is
		// not, because the scan never reached it. The report is text alone,
		// since the flags that would have carried --json are what failed.
		s.json = false
		return s.reportError(parseErr)
	}
	if actor, err := bench.ResolveActor(parsed.value("actor"), cfg); err == nil {
		s.actor = actor
	}
	// A request for help is answered before any command runs, so
	// `dinah move --help` prints move's page rather than refusing that move
	// was given no card. It is read ahead of --version for the same reason
	// every tool reads it first: a caller who wrote both wants to be told
	// what the tool does.
	if parsed.has("help") {
		return s.helpFor(at(parsed.positional, 0))
	}
	if parsed.has("version") {
		// A first word naming no command refuses here exactly as it does on
		// the help path. The two flags are answered before the command walk
		// and would otherwise disagree about a word neither of them reads:
		// `dinah --help bogus` refused and `dinah bogus --version` did not.
		if word := at(parsed.positional, 0); word != "" {
			if _, ok := lookup(word); !ok {
				return s.fail(contract.UnknownVerb, word)
			}
		}
		s.command = "version"
		return runVersion(s, parsed)
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
	s.command = command.name
	// resolveOpenTailFlags exists to decide whether a flag-shaped word
	// sitting inside a multi-word free-text zone is prose or a flag
	// (dinah-96). add, block, and comment have no such zone left to be mid
	// of: dinah-100 bounds each to exactly one free-text word, so a domain
	// flag typed anywhere is applied by parseArgs itself, correctly, with
	// nothing here left to correct. config declares no domain flag of its
	// own and keeps calling this function only to splice an unrecognized
	// flag-shaped word back into its value as literal text.
	// workbench joins them: dinah-100's one-word rule bounds its value too,
	// and it declares --yes, so a flag typed after the value would otherwise
	// take the peeling branch, which nothing shipped exercises today.
	// workstream sits with workbench for the same two reasons: dinah-100's
	// one-word rule bounds its title and its value, and it declares --yes.
	if command.name != "add" && command.name != "block" && command.name != "comment" && command.name != "workbench" && command.name != "workstream" {
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

// errLine puts one already-rendered line on stderr, for a report that is not a
// refusal and so carries no name in front of it. A caller reading the leading
// token of stderr for a refusal name sees the first word of a sentence here,
// which is what a report of something that did not stop the command looks
// like.
func (s *session) errLine(text string) {
	io.WriteString(s.errw, text+"\n")
}

// fail reports a refusal and returns its exit code. The refusal name is the
// first whitespace-delimited token on stderr, followed by the sentence a
// person reads, which is the contract the plumbing guarantee rests on. It
// builds the refusal rather than composing a sentence, so the twelve sites
// that reach it get the same composition every other refusal gets.
func (s *session) fail(name, detail string) int {
	return s.reportError(contract.Refuse(name, detail))
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

// reportError reports an error from a layer below as a refusal on stderr and
// an exit code. The machine form reaches stdout first under --json, and the
// composed lines follow it on stderr, exactly as a verb's own refusal is
// emitted.
//
// dinah.ambiguous-workbench is the one refusal whose machine form carries
// more than its named values: the candidates the walk found reach a script as
// structured rows rather than as a prose string it would have to split. The
// text form needs no branch here at all, because the shape declares the same
// candidates as its listing and the composer draws them.
func (s *session) reportError(err error) int {
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		io.WriteString(s.errw, contract.OutcomeUnreachable+" "+err.Error()+"\n")
		return contract.ExitCode(contract.OutcomeUnreachable)
	}
	if s.json {
		report := refusalReport{
			Outcome: contract.OutcomeRefused,
			Refusal: refusal.Name,
			Detail:  refusal.Detail,
			Context: refusal.Extra,
		}
		if refusal.Name == contract.AmbiguousWorkbench {
			report.Detail = ""
			report.Workbenches, _ = bench.Reachable(s.cwd, s.benchFlag, s.home, s.nativeHome)
		}
		s.emitJSON(report)
	}
	for _, line := range s.composeRefusal(refusal) {
		io.WriteString(s.errw, line+"\n")
	}
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
	library := verb.New(opened, s.home)
	s.library = library
	return library, nil
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
