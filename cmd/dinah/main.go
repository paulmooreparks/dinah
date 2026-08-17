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
	if name != contract.Malformed || extra["path"] == "" {
		return text
	}
	return text + r.T("refusal.malformed.at", "path", extra["path"]) + r.T("refusal.malformed.fix")
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
