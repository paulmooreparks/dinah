package mcp

import (
	"sort"

	"dinah/internal/msg"
	"dinah/internal/verb"
)

// tool is one entry of the MCP surface: the name an agent calls, the library
// command it projects, and the call that runs it.
//
// Tool names are the cli head's own verb spellings, expanded where the short
// unixy form reads as an abbreviation to a reader who never saw the cli. That
// gives list_cards, add_card and next_card, and leaves every other name as it
// stands.
//
// summaryKey overrides the catalog key toolList reads to describe a tool. It
// is empty on every tool except one whose command's own summary would be
// false on this surface: workbenches, whose command answers "what is
// reachable from here" and whose MCP tool answers "what this server may
// serve".
type tool struct {
	// name is what an agent calls.
	name string
	// command is the library command whose parameter list generates this
	// tool's input schema.
	command string
	// run answers the call with a value that marshals to the canonical form.
	run func(*verb.Library, *verb.Request) any
	// summaryKey, when set, replaces "cmd.<command>.summary" as the catalog
	// key toolList reads.
	summaryKey string
	// wrapper names the single member this tool publishes its answer under,
	// and is empty on a tool that publishes the answer's own object with no
	// member around it. The name is part of what an agent reads, since a
	// caller that decodes the payload and looks the answer up by member gets
	// nothing when the member is spelled differently, so it is declared here
	// where a test can read it rather than left implicit in the wrap call
	// that produces it.
	//
	// It is filled on the commands verb.CrossHeadIdentical names, which is
	// where the cross-head guard checks it. Elsewhere it is empty, and empty
	// carries no claim: a tool this field says nothing about is one no test
	// reads it for.
	wrapper string
}

// tools is the whole MCP surface. A command that exists only because a shell
// and a filesystem exist gets no tool, and every command this head leaves out
// is named in toolExemptions below with the reason, which is where a reader
// should go for the current set rather than to this sentence.
//
// workbench falls inside that rule rather than outside it, even though config
// does not. A workbench's own fields are workbench data that travels with the
// repository, where a user setting is a machine artifact, and the operator
// check guards the write here exactly as it does at a terminal, because the
// library holds it.
//
// workbenches is the one tool that sat inside the rule while the head served
// one workbench and sits outside it now. The rule's reason is unchanged:
// every other excluded command needs a shell or a filesystem to make sense.
// workbenches needs neither, and the workbench argument creates an address
// space an agent cannot enumerate from any other tool. The exception is the
// reason for the rule, not a contradiction of it.
var tools = []tool{
	{name: "claim", command: verb.Claim, run: doVerb},
	{name: "move", command: verb.Move, run: doVerb},
	{name: "release", command: verb.Release, run: doVerb},
	{name: "block", command: verb.Block, run: doVerb},
	{name: "unblock", command: verb.Unblock, run: doVerb},
	{name: "join_workstream", command: verb.Join, run: doVerb},
	{name: "leave_workstream", command: verb.Leave, run: doVerb},
	{name: "add_card", command: "add", run: func(l *verb.Library, r *verb.Request) any { return l.Add(r) }},
	{name: "comment", command: "comment", run: func(l *verb.Library, r *verb.Request) any { return l.Comment(r) }},
	{name: "attach", command: "attach", run: func(l *verb.Library, r *verb.Request) any { return l.Attach(r) }},
	{name: "archive", command: "archive", run: func(l *verb.Library, r *verb.Request) any { return l.Archive(r) }},
	{name: "delete", command: "delete", run: func(l *verb.Library, r *verb.Request) any { return l.Delete(r) }},
	{name: "rename", command: "rename", run: func(l *verb.Library, r *verb.Request) any { return l.Rename(r) }},
	{name: "status", command: "status", run: readStatus},
	{name: "columns", command: "columns", run: readColumns},
	{name: "list_cards", command: "ls", run: readList},
	{name: "next_card", command: "next", run: readNext},
	{name: "pull", command: verb.Pull, run: func(l *verb.Library, r *verb.Request) any { return l.Pull(r) }},
	{name: "query", command: "query", run: readQuery, wrapper: "matches"},
	{name: "tree", command: "tree", run: readTree, wrapper: "tree"},
	{name: "contents", command: "contents", run: readContents},
	{name: "attachments", command: "attachments", run: readAttachments},
	{name: "show", command: "show", run: readShow},
	{name: "log", command: "log", run: readLog},
	{name: "changes", command: "changes", run: readChanges},
	{name: "instructions", command: "instructions", run: readInstructions},
	{name: "whoami", command: "whoami", run: readWhoami},
	{name: "card", command: "card", run: doCard},
	{name: "workbench", command: "workbench", run: doWorkbench},
	{name: "workstream", command: "workstream", run: doWorkstream},
	// The tool sets the action itself, since column carries exactly one, and
	// argumentExemptions holds that argument back so the schema never asks a
	// caller for a value this closure has already decided. A later card adding
	// get and set has two ways out of the single-purpose shape, and either one
	// deletes the exemption: dispatch on the action here the way workstream
	// does, or publish the other two acts as tools of their own.
	{name: "new_column", command: "column", run: func(l *verb.Library, r *verb.Request) any { r.Action = "new"; return l.NewColumn(r) }},
	{name: "version", command: "version", run: readVersion},
	{name: "export", command: "export", run: readExport},
	{name: "check", command: "check", run: readCheck},
	{name: "workbenches", command: "workbenches", run: nil, summaryKey: "tool.workbenches.summary"},
}

// toolExemptions names every library command this head deliberately does not
// serve, with the reason it is absent. The comment above tools states the same
// reasoning in prose; this map is what a test can read, and the roster check
// requires every command to be either served or named here with a reason, so a
// gap nobody has argued for cannot reach a green build.
var toolExemptions = map[string]string{
	"path":    "resolves a filesystem path for a shell to consume, so it means nothing over a protocol",
	"edit":    "opens a file in the reader's own editor, which needs a terminal this head does not have",
	"init":    "creates a workbench in a directory, which is a filesystem act rather than a workbench act",
	"extract": "copies a workbench definition out to a directory, which is the same filesystem act",
	"config":  "writes the user's own machine settings, which travel with the person rather than the workbench",
	"mcp":     "starts this head, so a tool for it would be the server offering to start itself",
	"guide":   "served as a resource rather than a tool, because a guide is read rather than run",
	"help":    "the surface's own tools/list carries every tool's schema and description, which is what help prints at a terminal",
}

// argumentExemptions names, per tool, the parameters this head deliberately
// does not publish, with the reason each is held back. Everything else a
// command declares is published, so a parameter reaches a caller unless
// somebody has written down why it should not.
//
// check's two scope flags are the case it was written for. They select which
// tree the terminal's two repair sweeps walk down, and this head runs neither
// sweep: its check tool reads the workbench it was given and answers what it
// found. Publishing them would offer an agent an argument that changes nothing
// about the answer it gets back, which is the silent drop dinah-362 exists to
// close, arriving on the other head.
// new_column is the second case, and it arrives from the other direction.
// Its command's action is the first word of `dinah column new`, and new is the
// only word that command takes, so the tool is that one action and fills the
// field in itself. Publishing the argument anyway would ask a caller to send a
// value the run closure overwrites one frame later, so a call naming any other
// action would be answered ok by a head that had quietly done something else,
// where the terminal refuses the same word under Usage. Held back, the two
// surfaces agree again: neither creates a column for a caller who asked for
// something other than a creation, because checkArguments reads this table and
// refuses the name outright.
var argumentExemptions = map[string]map[string]string{
	"check": {
		"root":      "aims the terminal's two repair sweeps at a tree, and this head runs neither sweep",
		"max-depth": "bounds the walk root names, and this head takes no root for check",
	},
	"new_column": {
		"action": "names the first word of `dinah column new`, and this tool is that one action, so the head fills the field in and a published argument would be a value it overwrites",
	},
}

// exemptArgument reports whether a tool holds a parameter back rather than
// publishing it.
func exemptArgument(tool, param string) bool {
	held, named := argumentExemptions[tool]
	if !named {
		return false
	}
	_, exempt := held[param]
	return exempt
}

// ToolNameFor returns the name this head publishes for a library command, and
// the empty string when the head serves no tool for it. A caller outside this
// package that has a command and needs the tool behind it asks here rather
// than keeping its own copy of the mapping, which would be one more surface to
// keep in step with this one.
func ToolNameFor(command string) string {
	for _, entry := range tools {
		if entry.command == command {
			return entry.name
		}
	}
	return ""
}

// WrapperMemberFor returns the member name this head publishes a library
// command's answer under, and the empty string both for a command whose answer
// carries no wrapping member and for one this surface serves no tool for.
//
// The two empties are not distinguished here because nothing is served by
// distinguishing them in this function. The caller that matters is the
// cross-head comparison, which already knows the tool exists before it asks,
// and which proves an empty answer right or wrong by comparing the unwrapped
// payload against the terminal's rather than by trusting this declaration.
func WrapperMemberFor(command string) string {
	for _, entry := range tools {
		if entry.command == command {
			return entry.wrapper
		}
	}
	return ""
}

// toolsByName indexes the surface for dispatch.
var toolsByName = indexTools()

// indexTools builds the dispatch index.
func indexTools() map[string]tool {
	index := map[string]tool{}
	for _, t := range tools {
		index[t.name] = t
	}
	return index
}

// toolList renders the surface for tools/list, with each input schema
// generated from the library's own parameter list.
func toolList() []map[string]any {
	catalog := msg.For(msg.Base)
	list := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		descriptionKey := "cmd." + t.command + ".summary"
		if t.summaryKey != "" {
			descriptionKey = t.summaryKey
		}
		entry := map[string]any{
			"name":        t.name,
			"description": catalog.T(descriptionKey),
			"inputSchema": schemaFor(t),
		}
		list = append(list, entry)
	}
	return list
}

// injectedProperties names the schema properties that come from no command's
// parameter list, mapped to the catalog key describing each. None is a
// parameter, so each is resolved by name: actor takes the sentence the global
// flag row already prints, basis takes one written for it, and workbench takes
// one written for the address space the MCP head binds.
var injectedProperties = map[string]string{
	"actor":     "flag.actor.summary",
	"basis":     "schema.basis.description",
	"workbench": "schema.workbench.description",
}

// declaredArgNames is the set of argument names one tool accepts: every
// parameter verb.Params declares for its command, plus the injected names
// above.
//
// The workbench name is held out for workbenches itself, whose scope argument
// is the positional path instead, so it would otherwise carry a name whose
// value the tool does not consume. That exception lives here and nowhere else.
//
// The published schema and the check that refuses an unrecognized argument
// both read this one function, so a caller cannot be refused a name tools/list
// offered it, nor have a name accepted that tools/list never offered.
func declaredArgNames(t tool) map[string]bool {
	names := map[string]bool{}
	for _, param := range verb.Params(t.command) {
		if exemptArgument(t.name, param.Name) {
			continue
		}
		names[param.Name] = true
	}
	for name := range injectedProperties {
		if name == "workbench" && t.name == "workbenches" {
			continue
		}
		names[name] = true
	}
	return names
}

// schemaFor generates one tool's input schema from the parameter list the cli
// head composes its syntax line from, so the two heads cannot drift.
//
// Every property carries the same sentence the cli head prints beside the
// argument, so an agent reading the schema and a person reading the help page
// are told the same thing. Which properties the schema publishes beyond the
// parameter list is declaredArgNames' answer, since a property offered here
// and refused there would tell a caller two different things.
//
// No property carries an enum. A description is additive and constrains no
// caller, where an enum changes what a strict client will send, which is a
// change to a published machine interface.
func schemaFor(t tool) map[string]any {
	catalog := msg.For(msg.Base)
	properties := map[string]any{}
	var required []string
	for _, param := range verb.Params(t.command) {
		if exemptArgument(t.name, param.Name) {
			continue
		}
		description := catalog.T(param.SummaryKey(t.command))
		properties[param.Name] = map[string]any{"type": param.Type(), "description": description}
		if param.Required {
			required = append(required, param.Name)
		}
	}
	declared := declaredArgNames(t)
	for name, key := range injectedProperties {
		if !declared[name] {
			continue
		}
		properties[name] = map[string]any{"type": "string", "description": catalog.T(key)}
	}
	sort.Strings(required)
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// doVerb runs one of the five contract verbs.
func doVerb(l *verb.Library, r *verb.Request) any {
	return l.Do(r)
}

// wrap adds the affordances member every tool response carries, so an agent
// never has to learn which responses answer the question and which do not.
func wrap(payload map[string]any, affordances []string) map[string]any {
	payload["affordances"] = affordances
	return payload
}

// readAffordances are what a caller may do next after a read of the bench
// rather than of one card.
var readAffordances = []string{"status", "columns", "list_cards", "next_card"}

// readStatus answers the status tool.
func readStatus(l *verb.Library, r *verb.Request) any {
	status, err := l.Status(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"status": status}, readAffordances)
}

// readColumns answers the columns tool.
func readColumns(l *verb.Library, r *verb.Request) any {
	columns, err := l.Columns()
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"columns": columns}, readAffordances)
}

// readList answers the list_cards tool.
func readList(l *verb.Library, r *verb.Request) any {
	listing, err := l.List(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"listing": listing}, readAffordances)
}

// readNext answers the next_card tool, which changes nothing: offering a card
// is not assigning it.
//
// The affordances carry pull beside claim, because an offer from a column where
// no owner takes work up is taken by a pull into the column beyond and a claim
// there is refused. An agent reading only claim would meet that refusal with
// nothing telling it what to reach for instead.
func readNext(l *verb.Library, r *verb.Request) any {
	offers, err := l.Next(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"offers": offers}, []string{"claim", "pull", "show", "log"})
}

// readQuery answers the query tool. It carries the same Matches object the
// cli head emits under --json for the same query string, since both heads hand
// the one library call the one string.
func readQuery(l *verb.Library, r *verb.Request) any {
	matches, err := l.Query(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"matches": matches}, readAffordances)
}

// readTree answers the tree tool. It carries the same Tree object the cli head
// emits under --json for the same arguments, since both heads hand the one
// library call the one chain, the one level and the one query string.
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
	return wrap(map[string]any{"tree": tree}, readAffordances)
}

// readContents answers the contents tool.
func readContents(l *verb.Library, r *verb.Request) any {
	level := r.Depth
	if level == "" {
		level = verb.LevelEntities
	}
	tree, err := l.Contents(r, level)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"tree": tree}, readAffordances)
}

// readAttachments answers the attachments tool. It publishes the same listing
// the terminal head prints, so the two heads report one entity's attachments
// identically and neither carries a shape of its own.
func readAttachments(l *verb.Library, r *verb.Request) any {
	listing, err := l.Attachments(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"attachments": listing}, readAffordances)
}

// cardAffordances asks the library what a caller may do with the card a
// request names, and puts the answer into this surface's vocabulary before it
// is published. A read never returns a *verb.Response, so nothing else
// translates it, and the library's no-card fallback spells two of its reads
// as the commands ls and next, which name no tool here. Translating at the
// boundary keeps that fallback from dead-ending a reader whichever head
// reaches it.
func cardAffordances(l *verb.Library, r *verb.Request) []string {
	return surfaceAffordances(l.CardAffordances(r))
}

// readShow answers the show tool. The card branch asks the library what a
// caller may do with the card it just read, rather than carrying a list of its
// own that would go on naming claim at a column where a claim is refused.
func readShow(l *verb.Library, r *verb.Request) any {
	detail, text, err := l.Show(r)
	if err != nil {
		return l.FromError(r, err)
	}
	if detail == nil {
		return wrap(map[string]any{"text": text}, readAffordances)
	}
	return wrap(map[string]any{"detail": detail}, cardAffordances(l, r))
}

// readLog answers the log tool, and asks for its affordances for the same
// reason readShow does.
func readLog(l *verb.Library, r *verb.Request) any {
	events, err := l.History(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return journalView{Events: events, Affordances: cardAffordances(l, r)}
}

// readChanges answers the changes tool. It carries the same ChangeSet the cli
// head emits under --json for the same arguments, since both heads hand the
// one library call the one cursor, the one card and the one column.
//
// The answer is the ChangeSet itself rather than a wrapped payload, the way
// log's is, because the shape already declares its own affordances member.
func readChanges(l *verb.Library, r *verb.Request) any {
	changes, err := l.Changes(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return changes
}

// readInstructions answers the instructions tool, and asks the library what
// the caller may do where the chain was served rather than carrying a list of
// its own.
//
// This is the tool an agent calls precisely to learn what it may do where the
// card is standing, so it is the worst place on the surface to name an act the
// tool refuses. A written-out claim told a caller at a buffer, at an intake
// column or at a done column to claim, and the claim then answered
// dinah.takes-no-work.
func readInstructions(l *verb.Library, r *verb.Request) any {
	served, err := l.Instructions(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"served": served}, surfaceAffordances(l.ServedAffordances(r, served)))
}

// readWhoami answers the whoami tool.
func readWhoami(l *verb.Library, r *verb.Request) any {
	identity, err := l.Whoami(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"identity": identity}, readAffordances)
}

// doCard answers the card tool, which reads one of a card's own fields or
// writes one, exactly as the command does. A call naming the set action takes
// the write, and every other call is the read, so an agent reaches the same
// two acts a person reaches from a terminal.
func doCard(l *verb.Library, r *verb.Request) any {
	if r.Action == "set" {
		return l.SetCardField(r)
	}
	value, err := l.CardField(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"value": value}, []string{"show", "log", "card"})
}

// doWorkbench answers the workbench tool, which reads the workbench's own
// fields or writes one of them, exactly as the command does. A call naming the
// set action takes the write, and every other call is the read, so an agent
// reads the same three fields the terminal listing prints.
func doWorkbench(l *verb.Library, r *verb.Request) any {
	if r.Action == "set" {
		return l.SetWorkbench(r)
	}
	fields, err := l.Workbench(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"workbench": fields}, readAffordances)
}

// doWorkstream answers the workstream tool, which lists the workbench's
// workstreams, reads one, creates one or writes a field of one, exactly as the
// command does. The action selects, so an agent reaches the same four acts a
// person reaches from a terminal.
func doWorkstream(l *verb.Library, r *verb.Request) any {
	switch r.Action {
	case "new":
		return l.NewWorkstream(r)
	case "set":
		return l.SetWorkstream(r)
	case "get":
		detail, err := l.Workstream(r)
		if err != nil {
			return l.FromError(r, err)
		}
		return wrap(map[string]any{"detail": detail}, readAffordances)
	}
	listing, err := l.Workstreams()
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"listing": listing}, readAffordances)
}

// readVersion answers the version tool, always with catalog coverage, since a
// machine surface has no reason to withhold it.
func readVersion(l *verb.Library, r *verb.Request) any {
	return wrap(map[string]any{"version": verb.Version(true)}, readAffordances)
}

// readExport answers the export tool.
func readExport(l *verb.Library, r *verb.Request) any {
	data, err := l.Export()
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"interchange": string(data)}, readAffordances)
}

// readCheck answers the check tool.
func readCheck(l *verb.Library, r *verb.Request) any {
	report, err := l.Check(r)
	if err != nil {
		return l.FromError(r, err)
	}
	answer := map[string]any{"outcome": report.Outcome, "findings": report.Findings}
	if report.StampedOrdinals != nil {
		answer["stamped_ordinals"] = *report.StampedOrdinals
	}
	return wrap(answer, readAffordances)
}
