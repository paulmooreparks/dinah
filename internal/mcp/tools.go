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
var tools = []tool{
	{name: "claim", command: verb.Claim, run: doVerb},
	{name: "move", command: verb.Move, run: doVerb},
	{name: "release", command: verb.Release, run: doVerb},
	{name: "block", command: verb.Block, run: doVerb},
	{name: "unblock", command: verb.Unblock, run: doVerb},
	{name: "raise", command: verb.Raise, run: func(l *verb.Library, r *verb.Request) any { return l.Raise(r) }},
	{name: "join_workstream", command: verb.Join, run: doVerb},
	{name: "leave_workstream", command: verb.Leave, run: doVerb},
	{name: "add_card", command: "add", run: func(l *verb.Library, r *verb.Request) any { return l.Add(r) }},
	{name: "comment", command: "comment", run: func(l *verb.Library, r *verb.Request) any { return l.Comment(r) }},
	{name: "attach", command: "attach", run: func(l *verb.Library, r *verb.Request) any { return l.Attach(r) }},
	{name: "file_item", command: "file", run: func(l *verb.Library, r *verb.Request) any { return l.File(r) }},
	{name: "cite_item", command: "cite", run: func(l *verb.Library, r *verb.Request) any { return l.Cite(r) }},
	{name: "resolve_item", command: "resolve", run: func(l *verb.Library, r *verb.Request) any { return l.Resolve(r) }},
	{name: "verify_item", command: "verify", run: func(l *verb.Library, r *verb.Request) any { return l.Verify(r) }},
	{name: "fail_item", command: "fail", run: func(l *verb.Library, r *verb.Request) any { return l.Fail(r) }},
	{name: "reopen_item", command: "reopen", run: func(l *verb.Library, r *verb.Request) any { return l.Reopen(r) }},
	{name: "link_card", command: "link", run: func(l *verb.Library, r *verb.Request) any { return l.Link(r) }},
	{name: "unlink_card", command: "unlink", run: func(l *verb.Library, r *verb.Request) any { return l.Unlink(r) }},
	{name: "archive", command: "archive", run: func(l *verb.Library, r *verb.Request) any { return l.Archive(r) }},
	{name: "restore", command: "restore", run: func(l *verb.Library, r *verb.Request) any { return l.Restore(r) }},
	{name: "delete", command: "delete", run: func(l *verb.Library, r *verb.Request) any { return l.Delete(r) }},
	{name: "rename", command: "rename", run: func(l *verb.Library, r *verb.Request) any { return l.Rename(r) }},
	{name: "status", command: "status", run: readStatus},
	{name: "columns", command: "columns", run: readColumns},
	{name: "list_cards", command: "ls", run: readList},
	{name: "next_card", command: "next", run: readNext},
	{name: "pull", command: verb.Pull, run: func(l *verb.Library, r *verb.Request) any { return l.Pull(r) }},
	{name: "query", command: "query", run: readQuery, wrapper: "matches"},
	{name: "search_cards", command: "search", run: readSearch, wrapper: "results"},
	{name: "tree", command: "tree", run: readTree, wrapper: "tree"},
	{name: "contents", command: "contents", run: readContents},
	{name: "attachments", command: "attachments", run: readAttachments},
	{name: "show", command: "show", run: readShow},
	{name: "log", command: "log", run: readLog},
	{name: "changes", command: "changes", run: readChanges},
	{name: "instructions", command: "instructions", run: readInstructions},
	{name: "whoami", command: "whoami", run: readWhoami},
	{name: "workbench", command: "workbench", run: doWorkbench},
	{name: "workstream", command: "workstream", run: doWorkstream},
	{name: "get_field", command: "get", run: readField},
	{name: "set_field", command: "set", run: func(l *verb.Library, r *verb.Request) any { return l.SetField(r) }},
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

// exemption is one command this head does not serve: the ground it is held out
// on, drawn from the closed set below, and the prose a reader wants.
type exemption struct {
	// ground is why the command is held out, and is one of toolGrounds.
	ground string
	// reason is the sentence a reader meets, which says what about this
	// particular command puts it on that ground.
	reason string
}

// The grounds an exemption may stand on. The set is closed. A fifth needs a
// card arguing for it, which is the point: the guard's job is to make an
// unargued exemption impossible rather than to decide that only one argument
// can ever be made.
const (
	GroundShellOrFilesystem   = "shell-or-filesystem"
	GroundMachineNotWorkbench = "machine-not-workbench"
	GroundTheHeadItself       = "the-head-itself"
	GroundProtocolServesIt    = "protocol-serves-it"
)

// toolGrounds lists the closed set, in the order the constants declare it.
var toolGrounds = []string{
	GroundShellOrFilesystem, GroundMachineNotWorkbench,
	GroundTheHeadItself, GroundProtocolServesIt,
}

// toolExemptions names every library command this head deliberately does not
// serve, with the ground it is held out on and the reason it is absent. The
// comment above tools states the same reasoning in prose; this map is what a
// test can read, and the roster check requires every command to be either
// served or named here with a ground from the closed set and a reason, so a
// gap nobody has argued for cannot reach a green build.
var toolExemptions = map[string]exemption{
	"path":    {GroundShellOrFilesystem, "resolves a filesystem path for a shell to consume, so it means nothing over a protocol"},
	"edit":    {GroundShellOrFilesystem, "opens a file in the reader's own editor, which needs a terminal this head does not have"},
	"init":    {GroundShellOrFilesystem, "creates a workbench in a directory, which is a filesystem act rather than a workbench act"},
	"extract": {GroundShellOrFilesystem, "copies a workbench definition out to a directory, which is the same filesystem act"},
	"reshape": {GroundShellOrFilesystem, "reads its new column layout from a definition file or another workbench's directory, which is the same filesystem act init and extract are held out for"},
	"config":  {GroundMachineNotWorkbench, "writes the user's own machine settings, which travel with the person rather than the workbench"},
	"mcp":     {GroundTheHeadItself, "starts this head, so a tool for it would be the server offering to start itself"},
	"guide":   {GroundProtocolServesIt, "served as a resource rather than a tool, because a guide is read rather than run"},
	"help":    {GroundProtocolServesIt, "the surface's own tools/list carries every tool's schema and description, which is what help prints at a terminal"},
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
//
// check's ten repair markers are held back for the neighbouring reason, and
// the operator ruled on them at Operator Design Review on 2026-09-06. Each one
// selects a one-time repair of an ageing store, and this workbench's own
// standing rule is that a repair is never run against a live workbench and
// that a copy is taken first. A repair an agent should never run is an
// available wrong turn sitting permanently in front of every agent that
// connects, so the head does not offer it. The markers stay in the verb
// library and stay reachable at a terminal, where a person is present to take
// that copy, so this is a decision about what the head publishes rather than a
// deprecation of store repair.
//
// new_column is the third case, and it arrives from the other direction.
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
		"root":                "aims the terminal's two repair sweeps at a tree, and this head runs neither sweep",
		"max-depth":           "bounds the walk root names, and this head takes no root for check",
		"finish":              "completes a half-written store repair, which is operator work taken at a terminal against a copy",
		"remint":              "rewrites the identifier of one of two directories claiming it, which is an irreversible repair an operator decides",
		"yes":                 "confirms the two repairs whose rewrites have no undo, and this head offers neither of them",
		"witness":             "rebuilds the witness records of an ageing store, which is a one-time repair rather than a reading of it",
		"migrate-ordinals":    "rewrites every card's ordinal in place, which is a one-time repair of an ageing store",
		"migrate-slugs":       "rewrites every column's slug in place, which is a one-time repair of an ageing store",
		"migrate-columns":     "rewrites the column layout of an ageing store, which is a one-time repair of it",
		"migrate-vocabulary":  "rewrites the vocabulary of every workbench under a root, and its rewrite has no undo",
		"migrate-container":   "rewrites the container layout of every workbench under a root, and its rewrite has no undo",
		"migrate-workstreams": "rewrites the workstream records of an ageing store, which is a one-time repair of it",
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

// injectedProperty is one schema property that comes from no command's
// parameter list, carrying the catalog key that describes it and the set of
// tools that consume it.
//
// The set is what the declaration exists for. A property published on a tool
// that never reads it is an argument accepted and dropped without a word,
// which is the silent drop argumentExemptions was written to close, arriving
// through the injected-property door rather than the parameter door. So an
// injected property is published on exactly the tools that consume it, and a
// tool outside the set refuses the name the way it refuses any name the
// surface never offered.
type injectedProperty struct {
	// name is the property a schema publishes and a call may carry.
	name string
	// key is the catalog key whose sentence describes the property.
	key string
	// consumers names every tool that reads the property.
	consumers map[string]bool
}

// injectedProperties are the three such properties. None is a parameter, so
// each is resolved by name: actor takes the sentence the global flag row
// already prints, basis takes one written for it, and workbench takes one
// written for the address space the MCP head binds.
//
// Where each is consumed, and where the consumption is proved:
//
// actor is read on every call, because the lapse sweep and the witness writes
// consult the acting name on a read as well as on a write.
//
// basis is read at exactly two sites, the guard in Library.Do at
// internal/verb/mutate.go and the guard in the pull transaction at
// internal/verb/pull.go. Library.Do serves the seven contract verbs, so those
// two sites are the eight tools named below and nothing else. The other
// twenty-eight tools copied the value onto the request and dropped it, and
// they now refuse the name instead. TestBasisIsPublishedExactlyWhereItIsConsumed
// sends an impossible basis to every tool and fails in both directions, so the
// set below is checked against the code rather than trusted.
//
// workbench is read by every tool but workbenches, whose scope argument is the
// positional path instead, so on that one tool the name would carry a value
// the tool does not consume. That exception lives here and nowhere else.
var injectedProperties = []injectedProperty{
	{name: "actor", key: "flag.actor.summary", consumers: everyTool()},
	{name: "basis", key: "schema.basis.description", consumers: namedTools(
		"claim", "move", "release", "block", "unblock",
		"join_workstream", "leave_workstream", "pull",
	)},
	{name: "workbench", key: "schema.workbench.description",
		consumers: everyToolExcept("workbenches")},
}

// namedTools is the consumer set of a property only some tools read. Every
// name must be a tool this head serves, so a rename that leaves a stale name
// behind stops the package rather than quietly narrowing the surface.
func namedTools(names ...string) map[string]bool {
	set := map[string]bool{}
	for _, name := range names {
		if _, served := toolsByName[name]; !served {
			panic("mcp: an injected property names " + name + ", which this head serves no tool for")
		}
		set[name] = true
	}
	return set
}

// everyTool is the consumer set of a property every tool reads.
func everyTool() map[string]bool {
	return everyToolExcept()
}

// everyToolExcept is the consumer set of a property every tool but the named
// ones reads.
func everyToolExcept(names ...string) map[string]bool {
	held := map[string]bool{}
	for _, name := range names {
		held[name] = true
	}
	set := map[string]bool{}
	for _, t := range tools {
		if held[t.name] {
			continue
		}
		set[t.name] = true
	}
	return set
}

// declaredArgNames is the set of argument names one tool accepts: every
// parameter verb.Params declares for its command, plus the injected names
// this tool is a consumer of.
//
// Which tools consume which injected name is injectedProperties' answer and is
// argued there, so a reader asking why one tool takes basis and another does
// not goes to that declaration rather than to this function.
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
	for _, injected := range injectedProperties {
		if !injected.consumers[t.name] {
			continue
		}
		names[injected.name] = true
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
// A property this head injects rather than reads off the parameter table
// carries "x-dinah-injected": true. A client that composes a call needs to
// tell an argument its caller supplies from the plumbing the transport fills
// in, and without that key the only way to draw the line is to name actor,
// basis and workbench in the client's own source. Such a list goes stale on
// the day a fourth injected property is added here, and every client carrying
// one goes stale with it, so the head states the fact instead.
//
// Four keys beyond the type and the description say what a generic client
// cannot infer from either, and vocabularyKeys below derives all four from the
// same parameter table the rest of this function reads. An enum does change
// what a strict client will send, and this head published none until
// dinah-420, which reversed that decision. A parameter whose whole value is
// one member of a set fixed in the source is refused at the far end when it
// names anything else, so publishing the set moves that refusal from run time
// to composition time, and a client that cannot see the set has to ask a
// person to type a member from memory. A parameter whose value is a
// comma-separated list of those members is the case that reversal does not
// cover, because a legal answer naming two of them is not itself a member, so
// such a parameter publishes the members under a vendor key rather than under
// enum and keeps a strict client free to compose them.
func schemaFor(t tool) map[string]any {
	catalog := msg.For(msg.Base)
	properties := map[string]any{}
	var required []string
	for _, param := range verb.Params(t.command) {
		if exemptArgument(t.name, param.Name) {
			continue
		}
		description := verb.ArgumentMeaning(t.command, param, catalog.T)
		property := map[string]any{"type": param.Type(), "description": description}
		for key, value := range vocabularyKeys(t.command, param) {
			property[key] = value
		}
		properties[param.Name] = property
		if param.Required {
			required = append(required, param.Name)
		}
	}
	declared := declaredArgNames(t)
	for _, injected := range injectedProperties {
		if !declared[injected.name] {
			continue
		}
		properties[injected.name] = map[string]any{
			"type":             "string",
			"description":      catalog.T(injected.key),
			"x-dinah-injected": true,
		}
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

// vocabularyKeys are the schema keys one parameter earns beyond its type and
// its description, keyed by the key each is published under.
//
// The four answer questions a client asking "what may I send here?" cannot
// settle from a bare string type. A parameter whose whole value is one member
// of a vocabulary fixed in the source publishes that set as "enum", so a
// client offers the members rather than a blank field. A parameter whose
// vocabulary is resolved when a head runs publishes the source's own name
// under "x-dinah-vocabulary-source", because the members are workbench data
// this process cannot put in a static schema; a client that recognises the
// source resolves it with a call of its own, and one that does not sees a key
// it may ignore, which is what the "x-" prefix is for. A duration-typed
// parameter publishes "format": "duration", because the value is a string
// whose grammar is verb.ParseDuration's rather than free text.
//
// The fourth key exists because one parameter takes a list rather than a
// value. show's fields argument is declared with the list placeholder and is
// split on commas by verb.parseDetailFields, which accepts any subset of the
// vocabulary and reads a blank value as every member. Publishing that
// vocabulary as an enum would make the legal answer "card,body" invalid on
// this surface, so a list-valued parameter publishes "x-dinah-value-list":
// true and carries its members under "x-dinah-vocabulary-members" instead. A
// strict validator is then told what the members are without being told the
// value has to be one of them, which is the truth about this argument.
//
// Nothing here is hand-maintained. The vocabulary comes from
// verb.VocabularyFor, which reads the same table the cli head's own
// completion reads, and the list and duration markers come from the
// parameter's declared placeholder, so a parameter that gains or loses any of
// them changes this schema without anybody editing this function.
func vocabularyKeys(command string, param verb.Param) map[string]any {
	keys := map[string]any{}
	list := param.Value == listPlaceholder
	if list {
		keys["x-dinah-value-list"] = true
	}
	if set, declared := verb.VocabularyFor(command, param.Name); declared {
		switch {
		case set.Source != "":
			keys["x-dinah-vocabulary-source"] = set.Source
		case list:
			keys["x-dinah-vocabulary-members"] = set.Values
		default:
			keys["enum"] = set.Values
		}
	}
	if param.Value == durationPlaceholder {
		keys["format"] = "duration"
	}
	return keys
}

// listPlaceholder is the placeholder a parameter declares when its value is a
// comma-separated list rather than a single value, which is the only signal
// the parameter table gives that a vocabulary bounds the members rather than
// the whole argument.
const listPlaceholder = "list"

// durationPlaceholder is the placeholder a duration-typed parameter declares,
// which is the only signal the parameter table gives that a value is parsed as
// a duration rather than taken as text.
const durationPlaceholder = "duration"

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

// readSearch answers the search_cards tool, which is what puts a duplicate
// check in front of every agent that reaches this workbench over the protocol
// and has no filesystem to grep.
func readSearch(l *verb.Library, r *verb.Request) any {
	results, err := l.Search(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"results": results}, readAffordances)
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
//
// The request's Fields travels through untouched, and this head supplies no
// default of its own, so the MCP head's default shape for show is the shape
// show has always answered with. Which members an answer carries is the
// caller's choice on the call rather than this head's choice on every call,
// and the head stays a projection of the library and nothing else.
func readShow(l *verb.Library, r *verb.Request) any {
	detail, listing, text, err := l.Show(r)
	if err != nil {
		return l.FromError(r, err)
	}
	if listing != nil {
		return wrap(map[string]any{"collection": listing}, readAffordances)
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

// readField answers the get_field tool, which reads one field of whatever
// entity the reference names, exactly as the command does.
//
// It wraps with readAffordances rather than with a list of its own. The tool
// answers for a workbench, a column, a card, a comment, an item, an attachment
// and a workstream alike, so an affordance naming what a reader does next with
// a card would be wrong wherever the reference is not one, and every other
// bench-level read in this file wraps with the same four.
func readField(l *verb.Library, r *verb.Request) any {
	value, err := l.GetField(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"value": value}, readAffordances)
}

// doWorkbench answers the workbench tool, which reads the workbench's own
// fields exactly as the bare command does, so an agent reads the same three
// fields the terminal listing prints.
func doWorkbench(l *verb.Library, r *verb.Request) any {
	fields, err := l.Workbench(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"workbench": fields}, readAffordances)
}

// doWorkstream answers the workstream tool, which lists the workbench's
// workstreams or creates one, exactly as the command does. The action selects,
// so an agent reaches the same two acts a person reaches from a terminal, and
// a workstream's own fields are read and written through get_field and
// set_field.
func doWorkstream(l *verb.Library, r *verb.Request) any {
	if r.Action == "new" {
		return l.NewWorkstream(r)
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
