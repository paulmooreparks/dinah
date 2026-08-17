package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/mcp"
	"dinah/internal/verb"
)

// commands is the whole surface. A command absent from this table does not
// exist, and one present but carrying no group is reachable without being
// listed, which is what `help` is.
//
// The table is filled at start rather than in its declaration because `help`
// looks a command up, and a declaration that reached its own lookup would be
// an initialisation cycle.
var commands []*command

func init() {
	commands = []*command{
		{name: "add", group: groupWork, run: runAdd},
		{name: "claim", group: groupWork, run: runClaim},
		{name: "move", group: groupWork, run: runMove},
		{name: "release", group: groupWork, run: runRelease},
		{name: "block", group: groupWork, run: runBlock},
		{name: "unblock", group: groupWork, run: runUnblock},
		{name: "comment", group: groupWork, run: runComment},
		{name: "attach", group: groupWork, run: runAttach},
		{name: "archive", group: groupWork, run: runArchive},
		{name: "delete", group: groupWork, run: runDelete},

		{name: "status", group: groupRead, run: runStatus},
		{name: "states", group: groupRead, run: runStates},
		{name: "ls", group: groupRead, run: runList},
		{name: "next", group: groupRead, run: runNext},
		{name: "show", group: groupRead, run: runShow},
		{name: "log", group: groupRead, run: runLog},
		{name: "instructions", group: groupRead, run: runInstructions},
		{name: "guide", group: groupRead, run: runGuide},

		{name: "init", group: groupBench, run: runInit},
		{name: "export", group: groupBench, run: runExport},
		{name: "extract", group: groupBench, run: runExtract},
		{name: "path", group: groupBench, run: runPath},
		{name: "edit", group: groupBench, run: runEdit},
		{name: "config", group: groupBench, run: runConfig},
		{name: "check", group: groupBench, run: runCheck},
		{name: "whoami", group: groupBench, run: runWhoami},
		{name: "workbenches", group: groupBench, run: runWorkbenches},
		{name: "version", group: groupBench, run: runVersion},

		{name: "mcp", group: groupServe, run: runMCP},

		{name: "help", run: runHelp},
	}
}

// request builds the library request a command hands over, carrying the
// resolved actor and whatever flags the command reads.
func (s *session) request(name string, parsed *arguments) *verb.Request {
	req := &verb.Request{
		Verb:            name,
		Actor:           s.actor,
		State:           parsed.value("state"),
		Kind:            parsed.value("kind"),
		Description:     parsed.value("description"),
		Override:        parsed.has("override"),
		Replace:         parsed.has("replace"),
		Confirm:         parsed.has("yes"),
		ReadyOnly:       parsed.has("ready"),
		Finish:          parsed.has("finish"),
		MigrateOrdinals: parsed.has("migrate-ordinals"),
		MigrateSlugs:    parsed.has("migrate-slugs"),
		MigrateStates:   parsed.has("migrate-states"),
	}
	return req
}

// runClaim takes up a ready card.
func runClaim(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request(verb.Claim, parsed)
	req.Card = at(words, 0)
	expires, err := verb.ParseDuration(parsed.value("expires"))
	if err != nil {
		return s.reportError(err)
	}
	req.Expires = expires
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Do(req))
	})
}

// runMove carries a card to another state.
func runMove(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request(verb.Move, parsed)
	req.Card = at(words, 0)
	if req.State == "" {
		req.State = at(words, 1)
	}
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Do(req))
	})
}

// runRelease gives a card back to its queue.
func runRelease(s *session, parsed *arguments) int {
	req := s.request(verb.Release, parsed)
	req.Card = at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Do(req))
	})
}

// runBlock raises an obstacle and frees the card. The reason is positional,
// because an obstacle worth recording is worth typing without a flag.
func runBlock(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request(verb.Block, parsed)
	req.Card = at(words, 0)
	req.Reason = strings.Join(words[min(1, len(words)):], " ")
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Do(req))
	})
}

// runUnblock lifts a block, which is the operator's act alone.
func runUnblock(s *session, parsed *arguments) int {
	req := s.request(verb.Unblock, parsed)
	req.Card = at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Do(req))
	})
}

// runAdd files a new card.
func runAdd(s *session, parsed *arguments) int {
	req := s.request("add", parsed)
	req.Title = strings.Join(parsed.rest(), " ")
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Add(req))
	})
}

// runComment records a comment on a card, reading the text from stdin when
// the caller wrote a single dash in its place.
func runComment(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request("comment", parsed)
	req.Card = at(words, 0)
	req.Text = strings.Join(words[min(1, len(words)):], " ")
	if req.Text == "-" {
		piped, err := io.ReadAll(s.in)
		if err != nil {
			return s.reportError(err)
		}
		req.Text = string(piped)
	}
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Comment(req))
	})
}

// runAttach records a file against the bench, a state, a card or a comment.
func runAttach(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request("attach", parsed)
	req.Ref = at(words, 0)
	req.File = at(words, 1)
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Attach(req))
	})
}

// runArchive moves an entity out of the live set.
func runArchive(s *session, parsed *arguments) int {
	req := s.request("archive", parsed)
	req.Ref = at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Archive(req))
	})
}

// runDelete destroys an entity and its history.
func runDelete(s *session, parsed *arguments) int {
	req := s.request("delete", parsed)
	req.Ref = at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Delete(req))
	})
}

// runStatus reports where the bench stands and what the reader holds.
func runStatus(s *session, parsed *arguments) int {
	req := s.request("status", parsed)
	return s.withBench(func(l *verb.Library) int {
		req.WorkbenchSource = s.workbenchSource
		status, err := l.Status(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.json {
			return s.emitJSON(status)
		}
		s.renderStatus(status)
		return 0
	})
}

// runStates reports the flow in order.
func runStates(s *session, parsed *arguments) int {
	return s.withBench(func(l *verb.Library) int {
		states, err := l.States()
		if err != nil {
			return s.reportError(err)
		}
		if s.json {
			return s.emitJSON(states)
		}
		s.renderStates(states)
		return 0
	})
}

// runList presents a state's cards in queue order.
func runList(s *session, parsed *arguments) int {
	req := s.request("ls", parsed)
	if req.State == "" {
		req.State = at(parsed.rest(), 0)
	}
	return s.withBench(func(l *verb.Library) int {
		listing, err := l.List(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.json {
			return s.emitJSON(listing)
		}
		s.renderListing(listing)
		return 0
	})
}

// runNext reports the card each state offers next, and changes nothing.
func runNext(s *session, parsed *arguments) int {
	req := s.request("next", parsed)
	if req.State == "" {
		req.State = at(parsed.rest(), 0)
	}
	return s.withBench(func(l *verb.Library) int {
		offers, err := l.Next(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.json {
			return s.emitJSON(offers)
		}
		s.renderOffers(offers)
		return 0
	})
}

// runShow reads a card, or anything below it.
//
// A bare invocation where several workbenches are reachable lists them instead
// of refusing. The reader asked to be shown something and the tool cannot know
// which workbench they meant, so the choices themselves are what it has to
// show. Every other case is unchanged: one workbench reachable leaves nothing
// to choose between and still refuses over the empty card reference, and none
// reachable still refuses over the search that found nothing.
func runShow(s *session, parsed *arguments) int {
	req := s.request("show", parsed)
	req.Card = at(parsed.rest(), 0)
	if req.Card == "" {
		if rows, ok := s.ambiguousWorkbenches(); ok {
			return s.emitWorkbenches(rows)
		}
	}
	return s.withBench(func(l *verb.Library) int {
		detail, text, err := l.Show(req)
		if err != nil {
			return s.reportError(err)
		}
		if detail == nil {
			s.write(text)
			return 0
		}
		if s.json {
			return s.emitJSON(detail)
		}
		s.renderDetail(detail)
		return 0
	})
}

// runLog reports a card's recorded acts, oldest first.
func runLog(s *session, parsed *arguments) int {
	req := s.request("log", parsed)
	req.Card = at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		events, err := l.History(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.json {
			return s.emitJSON(events)
		}
		s.renderHistory(events)
		return 0
	})
}

// runInstructions serves the instruction chain at a position.
func runInstructions(s *session, parsed *arguments) int {
	req := s.request("instructions", parsed)
	req.Card = at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		served, err := l.Instructions(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.json {
			return s.emitJSON(served)
		}
		s.renderInstructions(&served.Instructions, served.LegalMoves)
		return 0
	})
}

// runGuide lists the embedded guides, or prints one.
func runGuide(s *session, parsed *arguments) int {
	topic := at(parsed.rest(), 0)
	if topic == "" {
		for _, known := range guide.Topics() {
			s.line("  " + pad(known, 20) + guide.Title(known))
		}
		return 0
	}
	text, err := guide.Text(topic)
	if err != nil {
		return s.reportError(err)
	}
	s.write(text)
	return 0
}

// runInit creates a bench in the .dinah container here, optionally from a
// template, and reports the directory it was written to.
func runInit(s *session, parsed *arguments) int {
	root := s.cwd
	if named := at(parsed.rest(), 0); named != "" {
		root = named
	}
	operator := bench.Ladder(parsed.value("operator"), s.actor)
	if operator == "" {
		return s.fail(contract.NoOwner, "")
	}
	// A slug given by hand is taken as given and checked; one derived from
	// the directory name is made to conform, since a directory is named by
	// whoever made it and the grammar is not theirs to satisfy.
	slug := parsed.value("slug")
	if slug == "" {
		slug = bench.Slugify(filepath.Base(root))
	}
	if !bench.ValidSlug(slug) {
		return s.fail(contract.Malformed, "slug")
	}
	written, err := verb.Init(root, slug, operator, parsed.value("from"), s.benchFlag, s.benchFlagSource)
	if err != nil {
		return s.reportError(err)
	}
	s.line(s.r.T("init.done", "root", written))
	return 0
}

// runExport writes the bench's interchange form to stdout, which composes
// with the shell the way `path` does and needs no output-file flag.
func runExport(s *session, parsed *arguments) int {
	return s.withBench(func(l *verb.Library) int {
		data, err := l.Export()
		if err != nil {
			return s.reportError(err)
		}
		s.write(string(data))
		return 0
	})
}

// runExtract copies the bench's definition out as a template.
func runExtract(s *session, parsed *arguments) int {
	target := at(parsed.rest(), 0)
	if target == "" {
		return s.fail(contract.Usage, "extract")
	}
	return s.withBench(func(l *verb.Library) int {
		if err := l.Extract(target); err != nil {
			return s.reportError(err)
		}
		s.line(s.r.T("extract.done", "dir", target))
		return 0
	})
}

// runPath writes the resolved absolute path alone to stdout.
//
// This is the plumbing guarantee: one line, no prefix, no quoting, no
// commentary, whatever the language setting and whatever --json says. On
// refusal stdout stays empty, the refusal name leads stderr, and the exit
// code carries the outcome.
func runPath(s *session, parsed *arguments) int {
	ref := at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		resolved, err := l.Bench.ResolvePath(ref)
		if err != nil {
			return s.reportError(err)
		}
		io.WriteString(s.out, resolved+"\n")
		return 0
	})
}

// runEdit opens a path in the reader's editor.
func runEdit(s *session, parsed *arguments) int {
	ref := at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		resolved, err := l.Bench.ResolvePath(ref)
		if err != nil {
			return s.reportError(err)
		}
		editor, err := bench.ResolveEditor(s.cfg, runtime.GOOS, onPath)
		if err != nil {
			return s.reportError(err)
		}
		cmd := exec.Command(editor, resolved)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, s.out, s.errw
		if err := cmd.Run(); err != nil {
			return s.reportError(err)
		}
		return 0
	})
}

// onPath reports whether a binary is on the search path, which is what the
// editor ladder's fallback rung asks before choosing one.
func onPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runConfig lists the user's own settings, or reads or writes one of them.
//
// The bare invocation lists, the way `states` and `whoami` report everything
// with no argument. The listing resolves each setting through its own ladder,
// so it answers a question `get` cannot: a key nobody ever set and a key set
// to the value the default carries read alike through the stored value alone.
func runConfig(s *session, parsed *arguments) int {
	words := parsed.rest()
	switch at(words, 0) {
	case "":
		settings := verb.Settings(s.cfg, verb.SettingsContext{
			LangFlag:      parsed.value("lang"),
			ActorFlag:     parsed.value("actor"),
			WorkbenchFlag: parsed.value("workbench"),
			WorkbenchEnv:  os.Getenv("DINAH_WORKBENCH"),
			GOOS:          runtime.GOOS,
			LookPath:      onPath,
			CWD:           s.cwd,
			Home:          s.home,
			NativeHome:    s.nativeHome,
		})
		if s.json {
			return s.emitJSON(settings)
		}
		s.renderSettings(settings)
		return 0
	case "get":
		key := at(words, 1)
		if !bench.KnownConfigKey(key) {
			return s.fail(contract.UnknownKey, key)
		}
		s.line(s.cfg.Get(key))
		return 0
	case "set":
		key, value := at(words, 1), strings.Join(words[min(2, len(words)):], " ")
		if err := s.cfg.Set(key, value); err != nil {
			return s.reportError(err)
		}
		return 0
	}
	return s.fail(contract.Usage, "config")
}

// runCheck checks the bench for structural defects.
func runCheck(s *session, parsed *arguments) int {
	req := s.request("check", parsed)
	return s.withBench(func(l *verb.Library) int {
		report, err := l.Check(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.json {
			code := 0
			if len(report.Findings) > 0 {
				code = contract.ExitCode(contract.OutcomeRefused)
			}
			s.emitJSON(report)
			return code
		}
		return s.renderCheck(report)
	})
}

// runWhoami reports the actor and whether it is the operator of this bench.
func runWhoami(s *session, parsed *arguments) int {
	req := s.request("whoami", parsed)
	return s.withBench(func(l *verb.Library) int {
		identity, err := l.Whoami(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.json {
			return s.emitJSON(identity)
		}
		s.renderIdentity(identity)
		return 0
	})
}

// runWorkbenches lists the workbenches reachable from here.
//
// It opens nothing and it never refuses over what the search found, because a
// question about what is reachable is answered by zero rows as truthfully as
// by several. A --workbench naming a directory that holds no workbench is
// the one refusal left, and it belongs to the caller's argument.
func runWorkbenches(s *session, parsed *arguments) int {
	rows, err := bench.Reachable(s.cwd, s.benchFlag, s.home, s.nativeHome)
	if err != nil {
		return s.reportError(err)
	}
	return s.emitWorkbenches(rows)
}

// ambiguousWorkbenches reports the reachable workbenches when there is a
// choice to be made, which is the one case where a command that wanted a
// single workbench has a listing to offer in place of its refusal. One row or
// none leaves the caller's own refusal in place.
func (s *session) ambiguousWorkbenches() ([]bench.Candidate, bool) {
	rows, err := bench.Reachable(s.cwd, s.benchFlag, s.home, s.nativeHome)
	if err != nil || len(rows) < 2 {
		return nil, false
	}
	return rows, true
}

// emitWorkbenches writes a listing in whichever form the invocation asked for.
func (s *session) emitWorkbenches(rows []bench.Candidate) int {
	if s.json {
		return s.emitJSON(rows)
	}
	s.renderWorkbenches(rows)
	return 0
}

// runVersion reports what this binary is and what it conforms to.
func runVersion(s *session, parsed *arguments) int {
	release := verb.Version(parsed.has("catalogs"))
	if s.json {
		return s.emitJSON(release)
	}
	s.renderVersion(release)
	return 0
}

// runMCP serves this bench over MCP on stdio.
func runMCP(s *session, parsed *arguments) int {
	library, err := s.open()
	if err != nil {
		return s.reportError(err)
	}
	if err := mcp.Serve(library, s.in, s.out); err != nil {
		return s.reportError(err)
	}
	return 0
}

// runHelp prints one command's arguments, refusals and exit codes. It is the
// command the help block's own last line names, and it is reachable without
// being listed among the groups.
func runHelp(s *session, parsed *arguments) int {
	name := at(parsed.rest(), 0)
	if name == "" {
		s.write(s.helpBlock())
		return 0
	}
	if _, ok := lookup(name); !ok {
		return s.fail(contract.UnknownVerb, name)
	}
	s.write(s.verbHelp(name))
	return 0
}
