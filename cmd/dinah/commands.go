package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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

// commandExemptions names every library command this head deliberately does
// not dispatch, with the reason it is absent. It is empty because the terminal
// serves the whole verb table today, and it exists anyway so that a future
// omission has somewhere to be argued for: the roster check requires every
// command to be either dispatched here or named here with a reason, and an
// empty map is the strongest reading of that rule rather than the absence of
// one.
var commandExemptions = map[string]string{}

func init() {
	commands = []*command{
		{name: "add", group: groupWork, run: runAdd, openTail: true},
		{name: "claim", group: groupWork, run: runClaim, bounded: 1},
		{name: "move", group: groupWork, run: runMove, bounded: 2},
		{name: "pull", group: groupWork, run: runPull, bounded: 1},
		{name: "release", group: groupWork, run: runRelease, bounded: 1},
		{name: "block", group: groupWork, run: runBlock, bounded: 1, openTail: true},
		{name: "unblock", group: groupWork, run: runUnblock, bounded: 1},
		{name: "comment", group: groupWork, run: runComment, bounded: 1, openTail: true},
		{name: "attach", group: groupWork, run: runAttach, bounded: 2},
		{name: "join", group: groupWork, run: runJoin, bounded: 2},
		{name: "leave", group: groupWork, run: runLeave, bounded: 2},
		{name: "archive", group: groupWork, run: runArchive, bounded: 1},
		{name: "delete", group: groupWork, run: runDelete, bounded: 1},
		{name: "rename", group: groupWork, run: runRename, bounded: 2},
		// card dispatches on its own first word the way workbench does, so it
		// declares an open tail here and runs its own arity and mistyped-flag
		// checks (see runCard). It sits under groupWork rather than beside
		// its grammar siblings because the groups split on what a command
		// acts on: groupWork holds every command that acts on a card, and a
		// reader scanning the block for how to change something about a card
		// finds it beside the other twelve.
		{name: "card", group: groupWork, run: runCard, openTail: true},

		{name: "status", group: groupRead, run: runStatus},
		{name: "columns", group: groupRead, run: runColumns},
		{name: "ls", group: groupRead, run: runList, bounded: 1},
		{name: "next", group: groupRead, run: runNext, bounded: 1},
		{name: "query", group: groupRead, run: runQuery, openTail: true},
		{name: "tree", group: groupRead, run: runTree, openTail: true},
		{name: "contents", group: groupRead, run: runContents, bounded: 1},
		{name: "attachments", group: groupRead, run: runAttachments, bounded: 1},
		{name: "show", group: groupRead, run: runShow, bounded: 1},
		{name: "log", group: groupRead, run: runLog, bounded: 1},
		// changes declares no bounded positional at all, so a stray word
		// anywhere in the invocation is refused rather than silently
		// ignored. Every argument it reads is a flag, the cursor included.
		{name: "changes", group: groupRead, run: runChanges},
		{name: "instructions", group: groupRead, run: runInstructions, bounded: 1},
		{name: "guide", group: groupRead, run: runGuide, bounded: 1},

		{name: "init", group: groupBench, run: runInit, bounded: 1},
		{name: "export", group: groupBench, run: runExport},
		{name: "extract", group: groupBench, run: runExtract, bounded: 1},
		{name: "path", group: groupBench, run: runPath, bounded: 1},
		{name: "edit", group: groupBench, run: runEdit, bounded: 1},
		// config dispatches on its own first word and runs the same
		// mistyped-flag check itself (see runConfig), so it declares an open
		// tail here to keep the generic walk in run() out of its way entirely.
		{name: "config", group: groupBench, run: runConfig, openTail: true},
		{name: "check", group: groupBench, run: runCheck},
		{name: "whoami", group: groupBench, run: runWhoami},
		// workbench dispatches on its own first word the way config does, so
		// it declares an open tail here and runs its own arity and
		// mistyped-flag checks (see runWorkbench).
		{name: "workbench", group: groupBench, run: runWorkbench, openTail: true},
		// workstream dispatches on its own first word the way workbench does,
		// so it declares an open tail here and runs its own arity and
		// mistyped-flag checks (see runWorkstream).
		{name: "workstream", group: groupBench, run: runWorkstream, openTail: true},
		// workbenches takes one positional, which is the directory to walk
		// downward from. Without it the command keeps answering what is
		// reachable from here, which is the upward search it has always run.
		{name: "workbenches", group: groupBench, run: runWorkbenches, bounded: 1},
		{name: "version", group: groupBench, run: runVersion},

		{name: "mcp", group: groupServe, run: runMCP},

		{name: "help", run: runHelp, bounded: 1},
	}
}

// request builds the library request a command hands over, carrying the
// resolved actor and whatever flags the command reads.
func (s *session) request(name string, parsed *arguments) *verb.Request {
	req := &verb.Request{
		Verb:              name,
		Actor:             s.actor,
		Column:            parsed.value("column"),
		Kind:              parsed.value("kind"),
		Description:       parsed.value("description"),
		Override:          parsed.has("override"),
		Replace:           parsed.has("replace"),
		Confirm:           parsed.has("yes"),
		ReadyOnly:         parsed.has("ready"),
		Finish:            parsed.has("finish"),
		MigrateOrdinals:   parsed.has("migrate-ordinals"),
		MigrateSlugs:      parsed.has("migrate-slugs"),
		MigrateColumns:    parsed.has("migrate-columns"),
		MigrateVocabulary: parsed.has("migrate-vocabulary"),
		MigrateContainer:  parsed.has("migrate-container"),
		Remint:            parsed.value("remint"),

		MigrateWorkstreams: parsed.has("migrate-workstreams"),
		MigrateWitness:     parsed.has("witness"),
		NoClaim:            parsed.has("no-claim"),
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

// runMove carries a card to another column.
func runMove(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request(verb.Move, parsed)
	req.Card = at(words, 0)
	if req.Column == "" {
		req.Column = at(words, 1)
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
	reason, refusal := s.freeText([]string{"block", req.Card}, words[min(1, len(words)):], "slot.reason")
	if refusal != nil {
		return s.reportError(refusal)
	}
	req.Reason = reason
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
	title, refusal := s.freeText([]string{"add"}, parsed.rest(), "slot.title")
	if refusal != nil {
		return s.reportError(refusal)
	}
	req.Title = title
	req.Severity = parsed.value("severity")
	req.Priority = parsed.value("priority")
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Add(req))
	})
}

// runCard reads or writes one of a card's own fields.
//
// The grammar is `dinah workbench`'s over a card, so the third entity that
// carries writable fields is reached the same way as the other two. There is
// no bare invocation, since a card reference is needed before the command
// means anything and `dinah show` already prints what a card holds.
//
// Like runWorkbench, this dispatches on its own first word rather than reading
// fixed positions, so it runs its own arity and mistyped-flag checks: once on
// the first word before the switch, once on the reference and the field inside
// the branches that read them, and once on get's fourth word, which get never
// reads. The parameter list's Required marks are read by the syntax line and
// by the mcp head's schema and by nothing here.
func runCard(s *session, parsed *arguments) int {
	words := parsed.rest()
	first := at(words, 0)
	if looksLikeMistypedFlag(first) {
		return s.fail(contract.Usage, first)
	}
	switch first {
	case "get":
		return s.runCardGet(parsed, words)
	case "set":
		return s.runCardSet(parsed, words)
	}
	return s.fail(contract.Usage, first)
}

// runCardGet prints one field of one card on a line of its own, so a script
// reads it, and prints an empty line for a card carrying no level.
func (s *session) runCardGet(parsed *arguments, words []string) int {
	reference := at(words, 1)
	field := at(words, 2)
	for _, word := range []string{reference, field} {
		if looksLikeMistypedFlag(word) {
			return s.fail(contract.Usage, word)
		}
	}
	if extra := at(words, 3); extra != "" {
		return s.fail(contract.Usage, extra)
	}
	return s.withBench(func(l *verb.Library) int {
		req := s.request("card", parsed)
		req.Action, req.Card, req.Field = "get", reference, field
		value, err := l.CardField(req)
		if err != nil {
			return s.reportError(err)
		}
		s.line(value)
		return 0
	})
}

// runCardSet writes one field of one card, and clears it when the invocation
// leaves the value off. Clearing is spelled that way because a card without a
// level is a legal card, so the `config set` grammar applies where the
// `workbench set` refusal of an empty value does not.
func (s *session) runCardSet(parsed *arguments, words []string) int {
	reference := at(words, 1)
	field := at(words, 2)
	for _, word := range []string{reference, field} {
		if looksLikeMistypedFlag(word) {
			return s.fail(contract.Usage, word)
		}
	}
	lead := []string{"card", "set", reference, field}
	value, refusal := s.freeText(lead, words[min(3, len(words)):], "slot.value")
	if refusal != nil {
		return s.reportError(refusal)
	}
	return s.withBench(func(l *verb.Library) int {
		req := s.request("card", parsed)
		req.Action, req.Card, req.Field, req.Value = "set", reference, field, value
		return s.emit(l.SetCardField(req))
	})
}

// runComment records a comment on a card, reading the text from stdin when
// the caller wrote a single dash in its place.
func runComment(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request("comment", parsed)
	req.Card = at(words, 0)
	text, refusal := s.freeText([]string{"comment", req.Card}, words[min(1, len(words)):], "slot.comment")
	if refusal != nil {
		return s.reportError(refusal)
	}
	req.Text = text
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

// runAttach records a file against the bench, a column, a card or a comment.
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

// runRename carries an attachment's payload under a new filename.
func runRename(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request("rename", parsed)
	req.Ref = at(words, 0)
	name, refusal := s.freeText([]string{"rename", req.Ref}, words[min(1, len(words)):], "slot.name")
	if refusal != nil {
		return s.reportError(refusal)
	}
	req.Value = name
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Rename(req))
	})
}

// runStatus reports where the bench stands and what the reader holds.
func runStatus(s *session, parsed *arguments) int {
	req := s.request("status", parsed)
	walk, refusal := s.rootWalkFor(parsed, parsed.value("root"))
	if refusal != nil {
		return s.reportError(refusal)
	}
	if walk != nil {
		return emitForest(s,
			func() (*verb.RootStatus, error) { return verb.StatusForest(walk.Root, s.home, req, walk.Depth) },
			s.renderRootStatus)
	}
	return s.withBench(func(l *verb.Library) int {
		req.WorkbenchSource = s.workbenchSource
		status, err := l.Status(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(status)
		}
		s.renderStatus(status)
		return 0
	})
}

// runColumns reports the flow in order.
func runColumns(s *session, parsed *arguments) int {
	return s.withBench(func(l *verb.Library) int {
		columns, err := l.Columns()
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(columns)
		}
		s.renderColumns(columns)
		return 0
	})
}

// runList presents a column's cards in queue order.
func runList(s *session, parsed *arguments) int {
	req := s.request("ls", parsed)
	if req.Column == "" {
		req.Column = at(parsed.rest(), 0)
	}
	walk, refusal := s.rootWalkFor(parsed, parsed.value("root"))
	if refusal != nil {
		return s.reportError(refusal)
	}
	if walk != nil {
		return emitForest(s,
			func() (*verb.RootListing, error) { return verb.ListForest(walk.Root, s.home, req, walk.Depth) },
			s.renderRootListing)
	}
	return s.withBench(func(l *verb.Library) int {
		listing, err := l.List(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(listing)
		}
		s.renderListing(listing)
		return 0
	})
}

// runNext reports the card each column offers next, and changes nothing.
func runNext(s *session, parsed *arguments) int {
	req := s.request("next", parsed)
	if req.Column == "" {
		req.Column = at(parsed.rest(), 0)
	}
	walk, refusal := s.rootWalkFor(parsed, parsed.value("root"))
	if refusal != nil {
		return s.reportError(refusal)
	}
	if walk != nil {
		return emitForest(s,
			func() (*verb.RootOffers, error) { return verb.NextForest(walk.Root, s.home, req, walk.Depth) },
			s.renderRootOffers)
	}
	return s.withBench(func(l *verb.Library) int {
		offers, err := l.Next(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(offers)
		}
		s.renderOffers(offers)
		return 0
	})
}

// runPull picks the destination, claims a card from its upstream, and moves
// it there in one transaction. The named form names the destination; the bare
// form chooses the one column that qualifies and refuses when more than one
// does. --no-claim weakens what pull writes, not what pull allows.
func runPull(s *session, parsed *arguments) int {
	req := s.request(verb.Pull, parsed)
	if req.Column == "" {
		req.Column = at(parsed.rest(), 0)
	}
	expires, err := verb.ParseDuration(parsed.value("expires"))
	if err != nil {
		return s.reportError(err)
	}
	req.Expires = expires
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Pull(req))
	})
}

// runQuery reports the live cards matching a query.
//
// The query is the command's free-text slot rather than a flag, so a caller
// who types the terms unquoted meets dinah-100's own refusal with the quoted
// line rebuilt for them, instead of a second refusal saying the same thing.
func runQuery(s *session, parsed *arguments) int {
	req := s.request("query", parsed)
	text, refusal := s.freeText([]string{"query"}, parsed.rest(), "slot.query")
	if refusal != nil {
		return s.reportError(refusal)
	}
	req.Query = text
	return s.withBench(func(l *verb.Library) int {
		matches, err := l.Query(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(matches)
		}
		s.renderMatches(matches)
		return 0
	})
}

// runTree presents the workbench's cards nested along a chain of axes.
//
// The query is the command's free-text slot rather than a flag, so a caller
// who types the terms unquoted meets the same refusal `dinah query` gives them
// with the quoted line rebuilt.
func runTree(s *session, parsed *arguments) int {
	req := s.request("tree", parsed)
	text, refusal := s.freeText([]string{"query"}, parsed.rest(), "slot.query")
	if refusal != nil {
		return s.reportError(refusal)
	}
	req.Query = text
	chain := verb.ParseChain(parsed.value("group-by"))
	level := depthOr(parsed, verb.LevelCards)
	walk, scopeRefusal := s.rootWalkFor(parsed, parsed.value("root"))
	if scopeRefusal != nil {
		return s.reportError(scopeRefusal)
	}
	if walk != nil {
		return emitForest(s,
			func() (*verb.Forest, error) {
				return verb.TreeForest(walk.Root, s.home, req, chain, level, walk.Depth)
			},
			s.renderForest)
	}
	return s.withBench(func(l *verb.Library) int {
		tree, err := l.Tree(req, chain, level)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(tree)
		}
		s.renderTree(tree)
		return 0
	})
}

// runContents walks the containment grammar down from any entity.
func runContents(s *session, parsed *arguments) int {
	req := s.request("contents", parsed)
	req.Ref = at(parsed.rest(), 0)
	level := depthOr(parsed, verb.LevelEntities)
	return s.withBench(func(l *verb.Library) int {
		tree, err := l.Contents(req, level)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(tree)
		}
		s.renderTree(tree)
		return 0
	})
}

// runAttachments reports the attachments of any entity that can carry one.
//
// An omitted reference asks about the workbench itself, which is what the
// empty reference already means to the entity resolver, so the reader who
// types the command bare is answered rather than refused.
func runAttachments(s *session, parsed *arguments) int {
	req := s.request("attachments", parsed)
	req.Ref = at(parsed.rest(), 0)
	return s.withBench(func(l *verb.Library) int {
		listing, err := l.Attachments(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(listing)
		}
		s.renderAttachmentListing(listing)
		return 0
	})
}

// depthOr reads the depth flag, falling back to the command's own default when
// the caller named none.
func depthOr(parsed *arguments, fallback string) string {
	if level := parsed.value("depth"); level != "" {
		return level
	}
	return fallback
}

// rootWalk is the downward walk one root-scoped read runs: the directory to
// walk from, resolved to an absolute path, and how many rungs below it to
// descend, where zero is unbounded.
type rootWalk struct {
	Root  string
	Depth int
}

// rootWalkFor reads the scope an invocation named and returns the walk it asks
// for, or nil when the invocation named no root at all, which is the ordinary
// single-workbench call every one of these commands still answers.
//
// named is where the root arrived, which is --root on the five read verbs and
// the positional path on workbenches. Everything after that is the same rule
// wherever it is applied, so it is written once here rather than six times at
// the call sites: two scopes is a refusal, a depth with nothing to bound is a
// refusal, a depth that is not a count of rungs is a refusal, and a root is
// resolved to an absolute path before any walk begins.
//
// The conflict check reads s.benchFlag rather than looking at --workbench and
// DINAH_WORKBENCH separately. That field is already bench.Resolve applied to
// the flag and the environment variable together, before the session is built,
// so a caller naming either one shows up here as a non-empty benchFlag and
// checking both would test one fact twice under two names.
func (s *session) rootWalkFor(parsed *arguments, named string) (*rootWalk, *contract.Refusal) {
	depth := parsed.value("max-depth")
	if named == "" {
		if depth != "" {
			return nil, contract.Refuse(contract.DepthWithoutRoot, depth)
		}
		return nil, nil
	}
	if s.benchFlag != "" {
		return nil, contract.RefuseWith(contract.ConflictingScope, named, map[string]string{
			"workbench": s.benchFlag,
		})
	}
	rungs := bench.DefaultEnumerateDepth
	if depth != "" {
		parsedDepth, err := strconv.Atoi(strings.TrimSpace(depth))
		if err != nil || parsedDepth < 0 {
			return nil, contract.Refuse(contract.MalformedDepth, depth)
		}
		rungs = parsedDepth
	}
	abs, err := filepath.Abs(named)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownRoot, named)
	}
	return &rootWalk{Root: abs, Depth: rungs}, nil
}

// emitForest runs one root-scoped read and writes its answer in whichever form
// the invocation asked for. The output forms and the refusal handling are one
// rule over all five verbs, so they are written once; each command supplies
// only the builder that asks its own question and the renderer that draws the
// answer that comes back.
//
// The machine branch goes through emitMachine, which serves the compact
// projection where compactEncode defines one for the value's own type and the
// canonical JSON everywhere else. No root-scoped answer has a compact
// rendering, so --format compact on a root-scoped read answers canonical JSON
// today. That is the documented per-type fallback rather than a gap here, and
// a card giving these five shapes a compact rendering adds a case there and
// changes nothing in this function.
func emitForest[T any](s *session, build func() (*T, error), render func(*T)) int {
	answer, err := build()
	if err != nil {
		return s.reportError(err)
	}
	if s.format != formatHuman {
		return s.emitMachine(answer)
	}
	render(answer)
	return 0
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
			return s.emitWorkbenches(rows, "")
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
		if s.format != formatHuman {
			return s.emitMachine(detail)
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
		if s.format != formatHuman {
			return s.emitMachine(events)
		}
		s.renderHistory(events)
		return 0
	})
}

// runChanges reports what has happened on the workbench since a cursor, and
// changes nothing. A call naming no cursor mints one and reports nothing,
// which is the answer to "what happens from now" rather than to "what ever
// happened", and the second question is what log answers.
func runChanges(s *session, parsed *arguments) int {
	req := s.request("changes", parsed)
	req.Since = parsed.value("since")
	req.Card = parsed.value("card")
	walk, refusal := s.rootWalkFor(parsed, parsed.value("root"))
	if refusal != nil {
		return s.reportError(refusal)
	}
	if walk != nil {
		return emitForest(s,
			func() (*verb.RootChangeSet, error) { return verb.ChangesForest(walk.Root, s.home, req, walk.Depth) },
			s.renderRootChanges)
	}
	return s.withBench(func(l *verb.Library) int {
		set, err := l.Changes(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			return s.emitMachine(set)
		}
		s.renderChanges(set)
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
		if s.format != formatHuman {
			return s.emitMachine(served)
		}
		s.renderInstructions(&served.Instructions, served.LegalMoves, served.Loop)
		return 0
	})
}

// runGuide lists the embedded guides, or prints one.
func runGuide(s *session, parsed *arguments) int {
	topic := at(parsed.rest(), 0)
	if topic == "" {
		s.line(s.r.T("guide.reading"))
		s.line("")
		topics := table{indent: 2, columns: s.columns("guide", "topic", "title")}
		for _, known := range guide.Topics() {
			topics.rows = append(topics.rows, tableRow{fields: []string{known, guide.Title(known)}})
		}
		s.table(topics)
		return 0
	}
	text, err := guide.Text(topic)
	if err != nil {
		return s.reportError(err)
	}
	s.write(wrapGuideText(text, s.width))
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
		return s.reportError(malformedSlug(slug))
	}
	written, err := verb.Init(root, slug, operator, parsed.value("from"), s.benchFlag, s.benchFlagSource)
	if err != nil {
		return s.reportError(err)
	}
	s.line(s.r.T("init.done", "root", written))
	// Read before the write, because recordActor fills the field it tests.
	recorded := s.actor == ""
	if err := recordActor(s, operator); err != nil {
		// The workbench exists by now, and init's exit code answers for
		// the workbench, so a configuration Dinah could not write is
		// reported rather than turned into a refusal of something that
		// did happen. The report is one sentence carrying the cause and
		// the command that recovers, and it leads with neither an
		// outcome name nor a refusal name, because a caller reading the
		// leading token of stderr would otherwise read a failure off a
		// run that returned 0.
		s.errLine(s.r.T("init.actor.unrecorded", "path", s.cfg.Path, "reason", err.Error()))
		return 0
	}
	if recorded {
		s.line(s.r.T("init.actor.recorded", "actor", operator, "path", s.cfg.Path))
	}
	return 0
}

// malformedSlug refuses a slug that does not conform, carrying the clause that
// names the card reference on only the one shape that reads as one.
//
// The distinction is what keeps the spliced clause honest. ValidSlug is
// ValidColumnSlug and a final segment carrying a letter, so a slug the column
// grammar accepts and this one refuses is exactly a slug ending in a dash and
// digits alone. A slug refused for any other reason gets the bare sentence,
// since the clause would otherwise tell a reader that "My Project" is a card
// reference.
func malformedSlug(slug string) error {
	if bench.ValidColumnSlug(slug) {
		return contract.RefuseWith(contract.Malformed, "slug", map[string]string{"cardRef": slug})
	}
	return contract.Refuse(contract.Malformed, "slug")
}

// recordActor records the operator as the actor in the person's own
// configuration, so that somebody who has just named themselves the operator
// of a new workbench can act in it without typing a second command.
//
// Dinah writes only when the actor ladder resolved nothing at any rung, which
// on this path means the person supplied the operator with --operator and the
// machine knows them by no other name. An actor already carried by the flag,
// by DINAH_ACTOR, or by the configuration is left exactly as it was, and the
// workbench's own operator is never consulted, because the operator of a
// workbench and the owner of an act stay separate.
func recordActor(s *session, operator string) error {
	if s.actor != "" {
		return nil
	}
	if err := s.cfg.Set("actor", operator); err != nil {
		return err
	}
	s.actor = operator
	return nil
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

// editCmd builds the command runEdit executes, without running it. Stdout
// and Stderr are s.rawOut and s.rawErr when those are set, which is a real
// console or a real redirected file, either way a genuine *os.File the
// child can use directly. exec.Cmd dups an *os.File straight into the
// child but builds a pipe and a copying goroutine around any other
// io.Writer, and an interactive editor needs the real handle for its own
// cursor control and raw input (dinah-199). They fall back to s.out and
// s.errw otherwise, which is every session a test builds by hand.
func editCmd(s *session, editor, path string) *exec.Cmd {
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = s.out
	if s.rawOut != nil {
		cmd.Stdout = s.rawOut
	}
	cmd.Stderr = s.errw
	if s.rawErr != nil {
		cmd.Stderr = s.rawErr
	}
	return cmd
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
		if err := editCmd(s, editor, resolved).Run(); err != nil {
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
// The bare invocation lists, the way `columns` and `whoami` report everything
// with no argument. The listing resolves each setting through its own ladder,
// so it answers a question `get` cannot: a key nobody ever set and a key set
// to the value the default carries read alike through the stored value alone.
//
// config does not fit the generic bounded/openTail shape every other command
// declares in the commands table, since it dispatches on its own first word
// rather than reading fixed positions, so it runs its own arity and
// mistyped-flag checks: once on the first word before the switch, once more
// on the second word inside get and set before bench.KnownConfigKey ever sees
// it, and once more on get's third word, which get never reads and now
// refuses rather than silently drops.
func runConfig(s *session, parsed *arguments) int {
	words := parsed.rest()
	first := at(words, 0)
	if looksLikeMistypedFlag(first) {
		return s.fail(contract.Usage, first)
	}
	switch first {
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
		if s.format != formatHuman {
			return s.emitMachine(settings)
		}
		s.renderSettings(settings)
		return 0
	case "get":
		key := at(words, 1)
		if looksLikeMistypedFlag(key) {
			return s.fail(contract.Usage, key)
		}
		if extra := at(words, 2); extra != "" {
			return s.fail(contract.Usage, extra)
		}
		if !bench.KnownConfigKey(key) {
			return s.fail(contract.UnknownKey, key)
		}
		s.line(s.cfg.Get(key))
		return 0
	case "set":
		key := at(words, 1)
		if looksLikeMistypedFlag(key) {
			return s.fail(contract.Usage, key)
		}
		value, refusal := s.freeText([]string{"config", "set", key}, words[min(2, len(words)):], "slot.value")
		if refusal != nil {
			return s.reportError(refusal)
		}
		if err := s.cfg.Set(key, value); err != nil {
			return s.reportError(err)
		}
		return 0
	}
	return s.fail(contract.Usage, first)
}

// checkModalFlags are the three flags that decide what check does before it
// ever builds a request, in the order a refusal names them. Each one answers
// on its own path and returns, so a second modal flag beside the first is
// never reached.
var checkModalFlags = []string{"migrate-vocabulary", "migrate-container", "remint"}

// checkStarvedMarkers are the markers a modal flag silently swallows: they are
// read off the request Library.Check receives, and a modal flag returns before
// that request is built.
var checkStarvedMarkers = []string{
	"finish",
	"migrate-ordinals",
	"migrate-slugs",
	"migrate-columns",
	"migrate-workstreams",
	"witness",
}

// checkFlagConflict answers the flag combinations check refuses, and it
// answers them all in one place because they are one question: which of the
// words the caller typed is this run going to act on, and is every other word
// he typed still going to mean something.
//
// A modal flag beside a second modal flag, or beside any of the six markers,
// is the first half. runCheck takes the first modal flag it finds and returns
// on that path, so everything else the caller asked for is dropped without a
// word. That silent drop is the complaint this command was rewritten over,
// one flag combination sideways, so it refuses and names every flag it found
// in conflict rather than picking one and going quiet about the rest.
//
// --root or --max-depth beside anything that is not a tree sweep is the second
// half. Neither flag has a downward walk to aim or to bound outside the two
// sweeps, so accepting one there would either drop a word the caller typed or
// give it a meaning nothing else on check gives it.
//
// The two halves are ordered rather than merged. A run naming --root, a modal
// flag and a starved marker together has two defects, and the modal conflict
// is the one that decides what the command would have done, so it is reported
// first.
//
// The two scope flags arrive as values rather than being read here, because
// runCheck reads them by name in its own body and args_coverage_test.go looks
// no deeper than that.
func checkFlagConflict(parsed *arguments, root, maxDepth string) string {
	modal := make([]string, 0, len(checkModalFlags))
	for _, name := range checkModalFlags {
		if name == "remint" {
			if parsed.value("remint") != "" {
				modal = append(modal, name)
			}
			continue
		}
		if parsed.has(name) {
			modal = append(modal, name)
		}
	}
	starved := make([]string, 0, len(checkStarvedMarkers))
	for _, name := range checkStarvedMarkers {
		if parsed.has(name) {
			starved = append(starved, name)
		}
	}
	if len(modal) > 1 || (len(modal) == 1 && len(starved) > 0) {
		conflicting := append(append([]string(nil), modal...), starved...)
		for i, name := range conflicting {
			conflicting[i] = "--" + name
		}
		return strings.Join(conflicting, " ")
	}
	if len(modal) == 1 && modal[0] != "remint" {
		return ""
	}
	if root != "" {
		return root
	}
	return maxDepth
}

// runCheck checks the bench for structural defects.
//
// The vocabulary migration is answered before the workbench is opened, which
// no other migration marker needs. Every one of its siblings repairs an
// additive gap in a workbench this build can already read; this one repairs
// the key names the reader itself is looking for, so the ordinary open would
// refuse the very workbench the migration exists to carry forward.
//
// Where the command acts is decided once, by one rule, on every invocation:
// --root names a directory to walk downward from, and its absence means the
// ordinary climb to the workbench the caller is standing in. Before this the
// two tree sweeps descended from the current directory while every other form
// of check climbed, so one flag silently changed what "here" meant and the
// output said nothing about it. An operator standing above thirteen
// workbenches now gets the same answer from `dinah check --migrate-container`
// as he gets from a bare `dinah check`, and the old reach is written
// `--root .`, which says out loud what tree is about to be touched.
//
// Both flags are read by name here rather than inside the sweep functions.
// args_coverage_test.go walks one call level down from a command's run
// function, so a read living in rootWalkFor, which the sweeps would reach
// through a second call, is a read that guard cannot see.
func runCheck(s *session, parsed *arguments) int {
	root := parsed.value("root")
	maxDepth := parsed.value("max-depth")
	if conflict := checkFlagConflict(parsed, root, maxDepth); conflict != "" {
		return s.fail(contract.Usage, conflict)
	}
	walk, refusal := s.rootWalkFor(parsed, root)
	if refusal != nil {
		return s.reportError(refusal)
	}
	if parsed.has("migrate-vocabulary") {
		return runMigrateVocabulary(s, parsed, walk)
	}
	if path := parsed.value("remint"); path != "" {
		return runRemint(s, path)
	}
	if parsed.has("migrate-container") {
		return runMigrateContainer(s, parsed, walk)
	}
	req := s.request("check", parsed)
	return s.withBench(func(l *verb.Library) int {
		report, err := l.Check(req)
		if err != nil {
			return s.reportError(err)
		}
		if s.format != formatHuman {
			s.emitMachine(report)
			return contract.ExitCodeForRead(report.Outcome)
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
		if s.format != formatHuman {
			return s.emitMachine(identity)
		}
		s.renderIdentity(identity)
		return 0
	})
}

// runWorkbench lists the workbench's own fields, or reads or writes one of
// them.
//
// The grammar is `config`'s, copied word for word, over a different file. The
// bare invocation lists, `get` prints one stored value alone so a script can
// read it, and `set` writes one field. The two commands stay apart because
// they write different files for different owners: `config` writes the user's
// own settings, which follow a person from workbench to workbench, and this
// writes the anchor, which travels with the repository to everybody who
// clones it.
//
// Like runConfig, this dispatches on its own first word rather than reading
// fixed positions, so it runs its own arity and mistyped-flag checks: once on
// the first word before the switch, once on the field name inside get and set,
// and once on get's third word, which get never reads.
func runWorkbench(s *session, parsed *arguments) int {
	words := parsed.rest()
	first := at(words, 0)
	if looksLikeMistypedFlag(first) {
		return s.fail(contract.Usage, first)
	}
	switch first {
	case "":
		return s.withBench(func(l *verb.Library) int {
			return s.emitWorkbenchFields(l, s.request("workbench", parsed))
		})
	case "get":
		field := at(words, 1)
		if looksLikeMistypedFlag(field) {
			return s.fail(contract.Usage, field)
		}
		if extra := at(words, 2); extra != "" {
			return s.fail(contract.Usage, extra)
		}
		return s.withBench(func(l *verb.Library) int {
			req := s.request("workbench", parsed)
			req.Action, req.Field = first, field
			fields, err := l.Workbench(req)
			if err != nil {
				return s.reportError(err)
			}
			s.line(fields.Field(field))
			return 0
		})
	case "set":
		field := at(words, 1)
		if looksLikeMistypedFlag(field) {
			return s.fail(contract.Usage, field)
		}
		value, refusal := s.freeText([]string{"workbench", "set", field}, words[min(2, len(words)):], "slot.value")
		if refusal != nil {
			return s.reportError(refusal)
		}
		return s.withBench(func(l *verb.Library) int {
			req := s.request("workbench", parsed)
			req.Action, req.Field, req.Value = first, field, value
			return s.emit(l.SetWorkbench(req))
		})
	}
	return s.fail(contract.Usage, first)
}

// emitWorkbenchFields answers the bare invocation in whichever form it asked
// for, which is how `whoami` and `config` with no argument both branch.
func (s *session) emitWorkbenchFields(l *verb.Library, req *verb.Request) int {
	fields, err := l.Workbench(req)
	if err != nil {
		return s.reportError(err)
	}
	if s.format != formatHuman {
		return s.emitMachine(fields)
	}
	s.renderWorkbenchFields(fields)
	return 0
}

// runWorkbenches answers where the workbenches are, in either of the two
// directions that question has.
//
// With no positional it lists what is reachable from here, which is the upward
// search it has always run: it opens nothing and never refuses over what the
// search found, because a question about what is reachable is answered by zero
// rows as truthfully as by several. A --workbench naming a directory that holds
// no workbench is the one refusal that path has, and it belongs to the caller's
// argument.
//
// With a positional it walks downward from that directory instead, listing
// every workbench beneath it. The two are different questions rather than two
// spellings of one, which is why the path is a positional and not a value for
// --workbench: that flag names a workbench to act on, and naming both is a
// refusal rather than a preference.
func runWorkbenches(s *session, parsed *arguments) int {
	walk, refusal := s.rootWalkFor(parsed, at(parsed.rest(), 0))
	if refusal != nil {
		return s.reportError(refusal)
	}
	if walk != nil {
		rows, err := bench.EnumerateDeep(walk.Root, walk.Depth)
		if err != nil {
			return s.reportError(err)
		}
		return s.emitWorkbenches(rows, walk.Root)
	}
	rows, err := bench.Reachable(s.cwd, s.benchFlag, s.home, s.nativeHome)
	if err != nil {
		return s.reportError(err)
	}
	return s.emitWorkbenches(rows, "")
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
//
// root is the directory a downward walk was asked about, empty on the upward
// search, and it selects the sentence an empty listing gets: a walk that found
// nothing names the directory it walked, where the search names here.
func (s *session) emitWorkbenches(rows []bench.Candidate, root string) int {
	if s.format != formatHuman {
		return s.emitMachine(rows)
	}
	s.renderWorkbenches(rows, root)
	return 0
}

// runVersion reports what this binary is and what it conforms to.
func runVersion(s *session, parsed *arguments) int {
	release := verb.Version(parsed.has("catalogs"))
	if s.format != formatHuman {
		return s.emitMachine(release)
	}
	s.renderVersion(release)
	return 0
}

// runMCP serves workbenches over MCP on stdio. Four situations decide
// whether the process exits before serving, and if it serves, what root and
// what default library it serves with:
//
//  1. A root was named (--root or DINAH_MCP_ROOT) and no directory sits at
//     the resolved path. The process writes dinah.unknown-root to stderr and
//     exits 2 without serving.
//  2. An explicit pointer (--workbench or DINAH_WORKBENCH) failed to open.
//     The process writes dinah.no-workbench to stderr and exits 2.
//  3. A root was named, an explicit pointer resolved, and the resolved
//     workbench lies outside that root. The process writes dinah.outside-root
//     to stderr and exits 2.
//  4. Nothing above refused. The process serves. Its root is whatever
//     --root or DINAH_MCP_ROOT gave, or "" (unbounded) when neither did;
//     unlike before dinah-307, discovering a workbench with no root no
//     longer narrows the root to that workbench's directory. A caller
//     naming a workbench per call is checked against the root through
//     bench.PathUnderRoot, and an unbounded root admits any candidate that
//     resolves and opens.
//
// The default library the fourth situation serves with is settled three
// ways. It is nil when discovery found nothing at all, meaning no explicit
// pointer and neither the ancestor walk nor the user config resolved a
// workbench; every unqualified call then refuses dinah.no-workbench-found,
// and so does the workbenches tool itself, since bench.Enumerate("") never
// has a filesystem to search. It is nil again when discovery found something
// through the ancestor walk or the user config that lies outside an
// explicitly given root, in which case the process writes one line to stderr
// from mcp.no-default naming what was dropped and stays bounded by the given
// root. Otherwise it is the discovered workbench, whether discovery was
// explicit, the ancestor walk, or the user config, and whether or not a root
// was given. When no root was given, that default answers every unqualified
// call, but the server is not otherwise bounded to it: a call naming a
// different, resolvable workbench is admitted rather than refused
// outside-root (dinah-307).
//
// The root is resolved through the same ladder --workbench and DINAH_WORKBENCH
// already climb, so the precedence is the board's own rather than one mcp
// invented.
func runMCP(s *session, parsed *arguments) int {
	root, _ := bench.Resolve(
		bench.Layer{Source: bench.SourceFlag, Value: parsed.value("root")},
		bench.Layer{Source: bench.SourceEnvironment, Value: os.Getenv("DINAH_MCP_ROOT")},
	)
	if root != "" {
		abs, err := filepath.Abs(root)
		if err != nil || !bench.Exists(abs) {
			path := root
			if abs != "" {
				path = abs
			}
			s.errLine(contract.UnknownRoot + " " + path)
			return contract.ExitCode(contract.OutcomeRefused)
		}
		root = abs
	}
	s.mcpRoot = root

	explicit := s.benchFlag != ""
	library, openErr := s.open()
	noWorkbench := refusalNamed(openErr, contract.NoWorkbench)
	switch {
	case noWorkbench != nil && explicit:
		// The refusal name and its detail are joined by a space, the way the
		// unknown-root and outside-root lines below are joined and the way
		// composeRefusal joins a name to its sentence. Refusal.Error puts a
		// colon after the name instead, and a caller that splits this line on
		// whitespace and takes the first field would read that colon as part
		// of the name, so it would match two of the three refusals mcp can
		// raise before it serves and miss the third.
		s.errLine(contract.NoWorkbench + " " + noWorkbench.Detail)
		return contract.ExitCode(contract.OutcomeRefused)
	case openErr != nil:
		libraries := map[string]*verb.Library{}
		if err := mcp.Serve(s.mcpRoot, nil, libraries, s.in, s.out); err != nil {
			return s.reportError(err)
		}
		return 0
	case library != nil && root != "":
		abs, _ := filepath.Abs(library.Bench.Root)
		contained, err := bench.PathUnderRoot(root, abs)
		if err != nil || !contained {
			if explicit {
				s.errLine(contract.OutsideRoot + " " + abs)
				return contract.ExitCode(contract.OutcomeRefused)
			}
			s.errLine(s.r.T("mcp.no-default", "detail", abs))
			library = nil
		}
	}
	libraries := map[string]*verb.Library{}
	if err := mcp.Serve(s.mcpRoot, library, libraries, s.in, s.out); err != nil {
		return s.reportError(err)
	}
	return 0
}

// serveMCPLoop's body is inlined inside runMCP, since the guard against hand-
// laid rows only exempts runMCP from naming the stream.

// refusalNamed returns the refusal err carries when err is a contract.Refusal
// with the given name, and nil in every other case. The startup path reads the
// typed value directly because FromError and Report already wrap the error and
// the call site has no library to hand to them, and because it renders the
// refusal's name and its detail into separate fields of the stderr line.
func refusalNamed(err error, name string) *contract.Refusal {
	refusal, ok := err.(*contract.Refusal)
	if !ok || refusal.Name != name {
		return nil
	}
	return refusal
}

// runHelp prints one command's arguments, refusals and exit codes. It is the
// command the help block's own last line names, and it is reachable without
// being listed among the groups.
func runHelp(s *session, parsed *arguments) int {
	return s.helpFor(at(parsed.rest(), 0))
}

// helpFor prints the whole surface, or one command's page when a command was
// named. It is what the help command runs and what a help flag written
// anywhere on the line runs, so the two reach one composition rather than two
// that can drift: `dinah ls --help` and `dinah help ls` print the same page,
// and a first word naming no command refuses the same way whichever spelling
// asked.
//
// The two doors part company past that first word, and only there. The command
// goes through run()'s own arity walk, so `dinah help ls extra` refuses the
// stray word; the flag is answered ahead of that walk, so `dinah ls -h extra`
// prints the page. A caller who asks what a command takes is told what it
// takes rather than told they asked wrongly, which is the whole point of the
// card, so the flag is the door that gets this right.
func (s *session) helpFor(name string) int {
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

// runJoin adds a card to a workstream. The card is the subject because the
// card's frontmatter is the file that changes.
func runJoin(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request(verb.Join, parsed)
	req.Card = at(words, 0)
	req.Workstream = at(words, 1)
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Do(req))
	})
}

// runLeave takes a card out of a workstream.
func runLeave(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request(verb.Leave, parsed)
	req.Card = at(words, 0)
	req.Workstream = at(words, 1)
	return s.withBench(func(l *verb.Library) int {
		return s.emit(l.Do(req))
	})
}

// runWorkstream lists the workbench's workstreams, creates one, or reads or
// writes one's fields.
//
// The grammar is `workbench`'s with a `new` action added and a reference in
// front of the field, because this command names one of many entities where
// that one names the workbench it is already serving. The bare invocation
// lists, `get` prints one workstream or one stored value alone so a script can
// read it, and `set` writes one field.
//
// Like runWorkbench, this dispatches on its own first word rather than reading
// fixed positions, so it runs its own arity and mistyped-flag checks: once on
// the first word before the switch, once on the reference and the field inside
// the branches that read them, and once on get's fourth word, which get never
// reads.
func runWorkstream(s *session, parsed *arguments) int {
	words := parsed.rest()
	first := at(words, 0)
	if looksLikeMistypedFlag(first) {
		return s.fail(contract.Usage, first)
	}
	switch first {
	case "":
		return s.withBench(func(l *verb.Library) int {
			listing, err := l.Workstreams()
			if err != nil {
				return s.reportError(err)
			}
			if s.format != formatHuman {
				return s.emitMachine(listing)
			}
			s.renderWorkstreams(listing)
			return 0
		})
	case "new":
		title, refusal := s.freeText([]string{"workstream", "new"}, words[min(1, len(words)):], "slot.title")
		if refusal != nil {
			return s.reportError(refusal)
		}
		return s.withBench(func(l *verb.Library) int {
			req := s.request("workstream", parsed)
			req.Action, req.Workstream = first, title
			return s.emitWorkstream(l.NewWorkstream(req))
		})
	case "get":
		return s.runWorkstreamGet(parsed, words)
	case "set":
		return s.runWorkstreamSet(parsed, words)
	}
	return s.fail(contract.Usage, first)
}

// runWorkstreamGet reads one workstream, or one field of it alone.
func (s *session) runWorkstreamGet(parsed *arguments, words []string) int {
	reference := at(words, 1)
	field := at(words, 2)
	for _, word := range []string{reference, field} {
		if looksLikeMistypedFlag(word) {
			return s.fail(contract.Usage, word)
		}
	}
	if extra := at(words, 3); extra != "" {
		return s.fail(contract.Usage, extra)
	}
	return s.withBench(func(l *verb.Library) int {
		req := s.request("workstream", parsed)
		req.Action, req.Workstream, req.Field = "get", reference, field
		detail, err := l.Workstream(req)
		if err != nil {
			return s.reportError(err)
		}
		if field != "" {
			s.line(detail.Workstream.Field(field))
			return 0
		}
		if s.format != formatHuman {
			return s.emitMachine(detail)
		}
		s.renderWorkstreamDetail(detail)
		return 0
	})
}

// runWorkstreamSet writes one field of one workstream.
func (s *session) runWorkstreamSet(parsed *arguments, words []string) int {
	reference := at(words, 1)
	field := at(words, 2)
	for _, word := range []string{reference, field} {
		if looksLikeMistypedFlag(word) {
			return s.fail(contract.Usage, word)
		}
	}
	lead := []string{"workstream", "set", reference, field}
	value, refusal := s.freeText(lead, words[min(3, len(words)):], "slot.value")
	if refusal != nil {
		return s.reportError(refusal)
	}
	return s.withBench(func(l *verb.Library) int {
		req := s.request("workstream", parsed)
		req.Action, req.Workstream, req.Field, req.Value = "set", reference, field, value
		return s.emitWorkstream(l.SetWorkstream(req))
	})
}

// emitWorkstream reports a workstream act: the machine form under --json, the
// one line a person reads otherwise, and on any non-zero outcome the refusal's
// own composition, which is what emit already does for a card act.
func (s *session) emitWorkstream(response *verb.Response) int {
	if response.Outcome != contract.OutcomeOK {
		s.reportOutcome(response)
		if s.format != formatHuman {
			s.emitMachine(response)
		}
		return contract.ExitCode(response.Outcome)
	}
	if s.format != formatHuman {
		return s.emitMachine(response)
	}
	s.renderWorkstreamLine(response.Workstream)
	return 0
}

// runMigrateContainer carries every workbench at or beneath a root into a
// .dinah container under an identifier Dinah minted.
//
// The root is resolved exactly the way runMigrateVocabulary resolves its own,
// and for the same reason: the two commands answer the same question about
// different repairs, and an operator who has learned where one of them acts
// should not have to learn a second rule for the other.
//
// A walk arrives here when the invocation named --root, and the sweep then
// descends from that directory. Nothing named a root when the walk is nil, so
// the command climbs like every other form of check and repairs the one
// workbench the climb found. The climb goes through s.open rather than
// through bench.Discover, which no command in this package calls directly,
// and the opened library carries the workbench's own directory. Handing that
// directory to the tree sweep walks a tree of exactly one node, so a single
// workbench is repaired by the same code that repairs a thousand.
//
// The rewrite waits for --yes for the reason the vocabulary sweep does, and
// with one more behind it: this repair moves directories rather than editing
// files inside them, so a preview is the only way to read the blast radius
// before it happens.
func runMigrateContainer(s *session, parsed *arguments, walk *rootWalk) int {
	confirmed := parsed.has("yes")
	resolved, err := s.sweepRoot(walk)
	if err != nil {
		return s.reportError(err)
	}
	report, err := verb.MigrateContainerTree(resolved, confirmed)
	if err != nil {
		return s.reportError(err)
	}
	code := contract.ExitCodeForRead(report.Outcome)
	if s.format != formatHuman {
		s.emitMachine(report)
		return code
	}
	s.renderContainer(report)
	return code
}

// runRemint gives one workbench directory a fresh identifier.
//
// It takes the path from the flag rather than from discovery, on the same
// override trust the sweep applies, and it acts on that one directory and no
// other. It is not gated on the directory appearing in a current duplicate
// report: an operator who has read one and decided which copy is the
// accidental one is performing an administrative act, and gating it on a
// finding that may have gone stale would make the escape hatch racier than
// the condition it repairs.
func runRemint(s *session, path string) int {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return s.reportError(err)
	}
	if !bench.Exists(filepath.Join(resolved, bench.WorkbenchAnchor)) {
		return s.fail(contract.NoWorkbench, resolved)
	}
	report, err := verb.RemintWorkbench(resolved)
	if err != nil {
		return s.reportError(err)
	}
	if s.format != formatHuman {
		return s.emitMachine(report)
	}
	s.renderRemint(report)
	return 0
}

// sweepRoot answers the directory a tree sweep walks down from, and it is the
// one place either sweep decides that.
//
// A caller who named --root has already had it resolved to an absolute path by
// rootWalkFor, along with every refusal that pair of flags can raise, so there
// is nothing left to decide here. A caller who named no root gets the ordinary
// climb, and the workbench it lands on is the whole of the tree the sweep then
// walks.
//
// The climb discovers and stops there, where every other command's climb goes
// on to open what it found. It stops because one of the two sweeps repairs the
// key names the reader itself looks for, so opening a workbench that needs the
// vocabulary carried forward refuses that workbench by name, and the repair
// would refuse to run on exactly the workbenches it exists for. Discovery
// resolves an override, an ambiguous base, a configured default and a failed
// walk identically to s.open, which is where these arguments are copied from,
// and only the open is left out.
//
// Putting the open back is four lines, and cmd/dinah's
// TestTheClimbingSweepRepairsRatherThanRefuses fails on them. What it costs is
// more than the refusal: the refusal is composed on a path that resolved no
// workbenchRoot, so its advice names no workbench and recommends the command
// that just refused.
func (s *session) sweepRoot(walk *rootWalk) (string, error) {
	if walk != nil {
		return walk.Root, nil
	}
	root, source, _, err := bench.DiscoverSource(
		s.cwd,
		s.benchFlag,
		s.benchFlagSource,
		s.home,
		s.nativeHome,
		s.cfg.Get("workbench"),
	)
	if err != nil {
		return "", err
	}
	s.workbenchSource = source
	return root, nil
}

// runMigrateVocabulary carries every workbench at or beneath a root across the
// vocabulary rename.
//
// --root names the directory to walk down from, and the walk arrives here
// already resolved. Without it the command climbs to the workbench the caller
// is standing in and carries that one workbench forward, which is what every
// other form of check does with the same silence.
//
// The two questions have different answers wherever a person keeps several
// workbenches side by side, and the command used to answer the second one
// while its own siblings answered the first. An operator sweeping a directory
// of customer boards still gets that reach, by writing --root and naming the
// directory, and what he gives up is a walk he did not ask for.
//
// The rewrite waits for --yes because it is irreversible, so a bare run
// reports what it would carry forward and writes nothing. The flag is the one
// this command's siblings already use for a deliberate act, so the preview
// costs no new vocabulary.
func runMigrateVocabulary(s *session, parsed *arguments, walk *rootWalk) int {
	confirmed := parsed.has("yes")
	resolved, err := s.sweepRoot(walk)
	if err != nil {
		return s.reportError(err)
	}
	report, err := verb.MigrateVocabularyTree(resolved, confirmed)
	if err != nil {
		return s.reportError(err)
	}
	code := contract.ExitCodeForRead(report.Outcome)
	if s.format != formatHuman {
		s.emitMachine(report)
		return code
	}
	s.renderVocabulary(report)
	return code
}
