// Package answer composes the payload a machine head publishes for one
// library call, so the MCP head and the HTTP head answer every question with
// one byte sequence.
//
// It holds the runner each command is answered by, the request builder that
// turns a map of named arguments into a library request, the composition of
// a refusal the head raises itself, the translation of affordance names into
// the vocabulary both machine surfaces publish, and the encoding. A head calls
// these and composes no payload of its own. Identity (the actor and the four
// declared facts) and the transport's own envelope stay with each head,
// because the two heads read them from different places.
package answer

import (
	"encoding/json"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// runner answers one command with a value that marshals to the canonical
// form.
type runner func(*verb.Library, *verb.Request) any

// runners maps each library command a machine head answers to its runner.
var runners = map[string]runner{
	verb.Claim:            doVerb,
	verb.Move:             doVerb,
	verb.Release:          doVerb,
	verb.Block:            doVerb,
	verb.Unblock:          doVerb,
	verb.Raise:            func(l *verb.Library, r *verb.Request) any { return l.Raise(r) },
	verb.Join:             doVerb,
	verb.Leave:            doVerb,
	"add":                 func(l *verb.Library, r *verb.Request) any { return l.Add(r) },
	"comment":             func(l *verb.Library, r *verb.Request) any { return l.Comment(r) },
	"attach":              func(l *verb.Library, r *verb.Request) any { return l.Attach(r) },
	"file":                func(l *verb.Library, r *verb.Request) any { return l.File(r) },
	"cite":                func(l *verb.Library, r *verb.Request) any { return l.Cite(r) },
	"resolve":             func(l *verb.Library, r *verb.Request) any { return l.Resolve(r) },
	"verify":              func(l *verb.Library, r *verb.Request) any { return l.Verify(r) },
	"fail":                func(l *verb.Library, r *verb.Request) any { return l.Fail(r) },
	"waive":               func(l *verb.Library, r *verb.Request) any { return l.Waive(r) },
	"withdraw":            func(l *verb.Library, r *verb.Request) any { return l.Withdraw(r) },
	"reopen":              func(l *verb.Library, r *verb.Request) any { return l.Reopen(r) },
	verb.GrantPermission:  doVerb,
	verb.RevokePermission: doVerb,
	"settle":              func(l *verb.Library, r *verb.Request) any { return l.Settle(r) },
	"link":                func(l *verb.Library, r *verb.Request) any { return l.Link(r) },
	"unlink":              func(l *verb.Library, r *verb.Request) any { return l.Unlink(r) },
	"archive":             func(l *verb.Library, r *verb.Request) any { return l.Archive(r) },
	"restore":             func(l *verb.Library, r *verb.Request) any { return l.Restore(r) },
	"delete":              func(l *verb.Library, r *verb.Request) any { return l.Delete(r) },
	"accept-divergence":   func(l *verb.Library, r *verb.Request) any { return l.AcceptDivergence(r) },
	"rename":              func(l *verb.Library, r *verb.Request) any { return l.Rename(r) },
	"status":              readStatus,
	"list":                readList,
	"next":                readNext,
	verb.Pull:             func(l *verb.Library, r *verb.Request) any { return l.Pull(r) },
	"query":               readQuery,
	"search":              readSearch,
	"tree":                readTree,
	"view":                readView,
	"show":                readShow,
	"changes":             readChanges,
	"instructions":        readInstructions,
	"whoami":              readWhoami,
	"prime":               readPrime,
	"workbench":           doWorkbench,
	"workstream":          doWorkstream,
	"get":                 readField,
	"set":                 func(l *verb.Library, r *verb.Request) any { return l.SetField(r) },
	"column":              doColumn,
	"version":             readVersion,
	"export":              readExport,
	"check":               readCheck,
}

// Runs reports whether a machine head can answer a command, which is what a
// head's roster guard asserts for every command it serves.
func Runs(command string) bool {
	_, ok := runners[command]
	return ok
}

// Run answers one command. payload is the value a head encodes, with its
// affordances already in the surface vocabulary. response is that same value
// when the payload is a *verb.Response, which is every act, every refusal and
// every read the library answered as a response, and nil when the payload is
// a read's own answer, which a runner produces only on success.
//
// A command with no runner is a programming error the heads' roster guards
// rule out. Run answers it with a usage refusal naming the command rather
// than panicking inside a request.
func Run(command string, l *verb.Library, r *verb.Request) (payload any, response *verb.Response) {
	run, ok := runners[command]
	if !ok {
		refused := Refusal(r, contract.Refuse(contract.Usage, command))
		return refused, refused
	}
	payload = run(l, r)
	if typed, ok := payload.(*verb.Response); ok {
		typed.Affordances = Affordances(typed.Affordances)
		return typed, typed
	}
	return payload, nil
}

// Refusal composes the answer to a refusal the head raised itself, before or
// instead of calling the library, with the affordances in the surface
// vocabulary.
func Refusal(r *verb.Request, refusal *contract.Refusal) *verb.Response {
	response := verb.ComposeRefusal(r, refusal)
	response.Affordances = Affordances(response.Affordances)
	return response
}

// FromError composes the answer to an error the head met while resolving
// something on the library's behalf, with the affordances in the surface
// vocabulary.
func FromError(l *verb.Library, r *verb.Request, err error) *verb.Response {
	response := l.FromError(r, err)
	response.Affordances = Affordances(response.Affordances)
	return response
}

// Encode is the one encoding both machine heads publish a payload in.
func Encode(payload any) ([]byte, error) {
	return json.MarshalIndent(payload, "", "  ")
}

// commandTool maps a library command name to the name the machine surfaces
// publish for it. Most affordances keep the same spelling in both
// vocabularies; the one read the surfaces name in full form is the exception,
// and a refusal that carried the library's short form would point a caller at
// a name neither surface answers to. The list entry left this table when the
// collapse gave the command and the tool one spelling, and Affordances passes
// an unmapped name through unchanged.
var commandTool = map[string]string{
	"next": "next_card",
}

// Translation returns a copy of the table Affordances translates by, keyed
// by library command, for a guard that holds it against a head's roster.
func Translation() map[string]string {
	copied := make(map[string]string, len(commandTool))
	for command, name := range commandTool {
		copied[command] = name
	}
	return copied
}

// Affordances translates a list of affordances from the library's command
// spellings to the names the machine surfaces publish. The reads that write
// their own list already spell the surface names; the refusal path, and
// every read that asks the library instead, can carry the library's own
// spellings. Translating here keeps the two vocabularies from disagreeing,
// which is the promise the mcp guide makes when it says that following the
// affordances cannot dead-end. Every spelling it does not remap passes
// through unchanged, so it is safe to run over an already-translated list.
func Affordances(affordances []string) []string {
	translated := make([]string, 0, len(affordances))
	for _, affordance := range affordances {
		if name, ok := commandTool[affordance]; ok {
			translated = append(translated, name)
		} else {
			translated = append(translated, affordance)
		}
	}
	return translated
}

// readAffordances are what a caller may do next after a read of the bench
// rather than of one card.
//
// Every name here is a tool the MCP head serves, and
// TestEveryPublishedAffordanceNamesAServedTool holds it to that off the same
// tools table tools/list is built from. The list lost columns and gained list
// in one move, because the collapse left one tool answering both questions.
var readAffordances = []string{"status", "list", "next_card"}

// ReadAffordances returns a copy of the affordances a read of the bench
// rather than of one card publishes, for a head answering such a read
// itself.
func ReadAffordances() []string {
	return append([]string(nil), readAffordances...)
}

// Wrap adds the affordances member every answer carries, so a caller never
// has to learn which answers name what it may do next and which do not.
func Wrap(payload map[string]any, affordances []string) map[string]any {
	payload["affordances"] = affordances
	return payload
}

// doVerb runs one of the contract verbs.
func doVerb(l *verb.Library, r *verb.Request) any {
	return l.Do(r)
}

// readStatus answers status.
func readStatus(l *verb.Library, r *verb.Request) any {
	status, err := l.Status(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"status": status}, readAffordances)
}

// readList answers list, which is every read of a collection the machine
// surfaces serve.
//
// The member each shape is published under is the member the retired tool
// published it under, so a caller that read columns off the columns tool goes
// on reading columns off the same key. The one new member is rosters, which no
// retired tool had, because no retired tool answered the bare question.
func readList(l *verb.Library, r *verb.Request) any {
	result, err := l.ListRef(r)
	if err != nil {
		return l.FromError(r, err)
	}
	switch result.Shape {
	case verb.ShapeRosters:
		return Wrap(map[string]any{"rosters": result.Rosters}, readAffordances)
	case verb.ShapeColumns:
		return Wrap(map[string]any{"columns": result.Columns}, readAffordances)
	case verb.ShapeWorkstreams:
		return Wrap(map[string]any{"listing": result.Workstreams}, readAffordances)
	case verb.ShapeAttachments:
		return Wrap(map[string]any{"attachments": result.Attachments}, readAffordances)
	case verb.ShapeMatches:
		return Wrap(map[string]any{"matches": result.Matches}, readAffordances)
	case verb.ShapeQueue:
		return Wrap(map[string]any{"listing": result.Queue}, readAffordances)
	case verb.ShapeHistory:
		return Wrap(map[string]any{"events": result.History}, readAffordances)
	}
	return Wrap(map[string]any{"tree": result.Contents}, readAffordances)
}

// readNext answers next, which changes nothing: offering a card is not
// assigning it.
//
// The affordances carry pull beside claim, because an offer from a column where
// no owner takes work up is taken by a pull into the column beyond and a claim
// there is refused. A caller reading only claim would meet that refusal with
// nothing telling it what to reach for instead.
func readNext(l *verb.Library, r *verb.Request) any {
	offers, err := l.Next(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"offers": offers}, []string{"claim", "pull", "show", "list"})
}

// readQuery answers query. It carries the same Matches object the cli head
// emits under --json for the same query string, since every head hands the
// one library call the one string.
func readQuery(l *verb.Library, r *verb.Request) any {
	matches, err := l.Query(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"matches": matches}, readAffordances)
}

// readSearch answers search, which is what puts a duplicate check in front of
// every caller that reaches this workbench over a protocol and has no
// filesystem to grep.
func readSearch(l *verb.Library, r *verb.Request) any {
	results, err := l.Search(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"results": results}, readAffordances)
}

// readTree answers tree. It carries the same Tree object the cli head emits
// under --json for the same arguments, since every head hands the one library
// call the one chain, the one level and the one query string.
func readTree(l *verb.Library, r *verb.Request) any {
	chain := verb.ParseChain(r.GroupBy)
	level := r.Depth
	if level == "" {
		level = verb.LevelCards
	}
	tree, err := l.Tree(r, chain, level)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"tree": tree}, readAffordances)
}

// readView answers view. With no view named it carries the listing under
// views, and with one named it carries the drawn view under view, which is
// the object the cli head prints under --json in each case, since every head
// hands the one library call the one view name and the one actor.
func readView(l *verb.Library, r *verb.Request) any {
	if r.View == "" {
		listing, err := l.ListViews(r)
		if err != nil {
			return l.FromError(r, err)
		}
		return Wrap(map[string]any{"views": listing.Views}, readAffordances)
	}
	drawn, err := l.DrawView(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"view": drawn.View}, readAffordances)
}

// cardAffordances asks the library what a caller may do with the card a
// request names, and puts the answer into the surface vocabulary before it is
// published. A read never returns a *verb.Response, so nothing else
// translates it, and the library's no-card fallback spells one of its reads
// as the command next, which names nothing on a machine surface. Translating
// at the boundary keeps that fallback from dead-ending a reader whichever
// head reaches it.
func cardAffordances(l *verb.Library, r *verb.Request) []string {
	return Affordances(l.CardAffordances(r))
}

// readShow answers show. The card branch asks the library what a caller may
// do with the card it just read, rather than carrying a list of its own that
// would go on naming claim at a column where a claim is refused.
//
// The request's Fields travels through untouched, and no head supplies a
// default of its own, so a machine head's default shape for show is the shape
// show has always answered with. Which members an answer carries is the
// caller's choice on the call rather than the head's choice on every call,
// and the head stays a projection of the library and nothing else.
func readShow(l *verb.Library, r *verb.Request) any {
	detail, record, item, text, err := l.Show(r)
	if err != nil {
		return l.FromError(r, err)
	}
	if record != nil {
		return Wrap(map[string]any{"record": record}, readAffordances)
	}
	if item != nil {
		return Wrap(map[string]any{"item": item}, cardAffordances(l, r))
	}
	if detail == nil {
		return Wrap(map[string]any{"text": text}, readAffordances)
	}
	return Wrap(map[string]any{"detail": detail}, cardAffordances(l, r))
}

// readChanges answers changes. It carries the same ChangeSet the cli head
// emits under --json for the same arguments, since every head hands the one
// library call the one cursor, the one card and the one column.
//
// The answer is the ChangeSet itself rather than a wrapped payload, because
// the shape already declares its own affordances member.
func readChanges(l *verb.Library, r *verb.Request) any {
	changes, err := l.Changes(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return changes
}

// readInstructions answers instructions, and asks the library what the
// caller may do where the chain was served rather than carrying a list of its
// own.
//
// This is the read a caller makes precisely to learn what it may do where the
// card is standing, so it is the worst place on a surface to name an act the
// surface refuses. A written-out claim told a caller at a buffer, at an intake
// column or at a done column to claim, and the claim then answered
// dinah.takes-no-work.
func readInstructions(l *verb.Library, r *verb.Request) any {
	served, err := l.Instructions(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"served": served}, Affordances(l.ServedAffordances(r, served)))
}

// readWhoami answers whoami.
func readWhoami(l *verb.Library, r *verb.Request) any {
	identity, err := l.Whoami(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"identity": identity}, readAffordances)
}

// readPrime answers prime, wrapping the answer under "primer" rather than
// composing a second envelope: a caller reading it already has the one call it
// asked for.
func readPrime(l *verb.Library, r *verb.Request) any {
	primer, err := l.Prime(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"primer": primer}, readAffordances)
}

// readField answers get, which reads one field of whatever entity the
// reference names, exactly as the command does.
//
// It wraps with readAffordances rather than with a list of its own. The read
// answers for a workbench, a column, a card, a comment, an item, an attachment
// and a workstream alike, so an affordance naming what a reader does next with
// a card would be wrong wherever the reference is not one, and every other
// bench-level read in this file wraps with the same list.
func readField(l *verb.Library, r *verb.Request) any {
	value, err := l.GetField(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"value": value}, readAffordances)
}

// doWorkbench answers workbench, which reads the workbench's own fields
// exactly as the bare command does, so a caller reads the same three fields
// the terminal listing prints.
func doWorkbench(l *verb.Library, r *verb.Request) any {
	fields, err := l.Workbench(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"workbench": fields}, readAffordances)
}

// doWorkstream answers workstream, which creates a workstream and does
// nothing else, exactly as the command does. Its listing action retired into
// list under the reference workstreams, so the runner refuses the retired
// action by name the way workbench refuses its own two, and a workstream's
// fields are read and written through get and set.
func doWorkstream(l *verb.Library, r *verb.Request) any {
	if r.Action == "new" {
		return l.NewWorkstream(r)
	}
	return l.FromError(r, contract.Refuse(contract.Usage, r.Action))
}

// doColumn answers column, whose one action in this build is new. The MCP
// head's new_column tool fills the action in itself, and any other action is
// refused by name, as the terminal refuses it.
func doColumn(l *verb.Library, r *verb.Request) any {
	if r.Action == "new" {
		return l.NewColumn(r)
	}
	return l.FromError(r, contract.Refuse(contract.Usage, r.Action))
}

// readVersion answers version, always with catalog coverage, since a machine
// surface has no reason to withhold it.
func readVersion(l *verb.Library, r *verb.Request) any {
	return Wrap(map[string]any{"version": verb.Version(true)}, readAffordances)
}

// readExport answers export.
func readExport(l *verb.Library, r *verb.Request) any {
	data, err := l.Export()
	if err != nil {
		return l.FromError(r, err)
	}
	return Wrap(map[string]any{"interchange": string(data)}, readAffordances)
}

// readCheck answers check with the report the library composed, carrying
// every member the terminal's machine form carries.
//
// The answer is the report's own JSON encoding decoded back into a map, so
// that the affordances can be added beside it. Naming the members here one
// by one is what dropped notices on the floor (dinah-590/criteria/14): the
// report gained a member and this projection went on copying the two it
// knew, so a live check over a machine head answered without the one report
// the notice tier was minted to show. Going through the encoding means a
// member added to CheckReport reaches the heads the day it is added, and the
// omitempty each optional member declares is honoured here exactly as the
// terminal honours it.
func readCheck(l *verb.Library, r *verb.Request) any {
	report, err := l.Check(r)
	if err != nil {
		return l.FromError(r, err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return l.FromError(r, err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return l.FromError(r, err)
	}
	return Wrap(decoded, readAffordances)
}
