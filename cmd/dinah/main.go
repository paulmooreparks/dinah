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
	// cfg is the user's own settings.
	cfg *bench.Config
	// actor is the owner resolved by the ladder, empty when no layer
	// carried one. It is empty rather than refused here, because the
	// refusal belongs inside the verb's own order.
	actor string
	// benchFlag is the bench named by --bench or DINAH_BENCH.
	benchFlag string
	// cwd is where bench discovery starts.
	cwd string
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
	s := &session{
		out:       out,
		errw:      errw,
		in:        in,
		r:         msg.For(bench.ResolveLang(parsed.value("lang"), cfg)),
		json:      parsed.has("json") || os.Getenv("DINAH_FORMAT") == "json",
		quiet:     parsed.has("quiet"),
		home:      home,
		cfg:       cfg,
		benchFlag: bench.Ladder(parsed.value("bench"), os.Getenv("DINAH_BENCH")),
		cwd:       cwd,
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
	return command.run(s, parsed)
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
	key := "refusal." + name
	if !s.r.Has(key) {
		return s.r.T("refusal.unknown", "name", name, "detail", detail)
	}
	return s.r.T(key, "detail", detail)
}

// reportError turns an error from a layer below into a report on stderr and
// an exit code, without a session, for the failures that happen before one
// can be built.
func reportError(errw io.Writer, r *msg.Renderer, err error) int {
	if refusal, ok := err.(*contract.Refusal); ok {
		io.WriteString(errw, refusal.Name+" "+r.T("refusal."+refusal.Name, "detail", refusal.Detail)+"\n")
		return contract.ExitCode(contract.OutcomeRefused)
	}
	io.WriteString(errw, contract.OutcomeUnreachable+" "+err.Error()+"\n")
	return contract.ExitCode(contract.OutcomeUnreachable)
}

// open discovers and opens the bench this invocation serves.
func (s *session) open() (*verb.Library, error) {
	root, err := bench.Discover(s.cwd, s.benchFlag, s.home)
	if err != nil {
		return nil, err
	}
	opened, err := bench.Open(root)
	if err != nil {
		return nil, err
	}
	return verb.New(opened, s.home), nil
}

// withBench opens the bench and hands it to a command, reporting the failure
// as a refusal when the bench cannot be opened at all.
func (s *session) withBench(do func(*verb.Library) int) int {
	library, err := s.open()
	if err != nil {
		return reportError(s.errw, s.r, err)
	}
	return do(library)
}
