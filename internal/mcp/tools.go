package mcp

import (
	"sort"

	"dinah/internal/msg"
	"dinah/internal/verb"
)

// tool is one entry of the MCP surface: the name an agent calls and the library
// command it projects. The runner that answers the command lives in
// internal/answer, which the HTTP head calls too, so the two heads publish one
// payload for one call.
//
// Tool names are the cli head's own verb spellings, expanded where the short
// unixy form reads as an abbreviation to a reader who never saw the cli. That
// gives add_card and next_card, and leaves every other name as it
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
	// action, when set, is the action word this tool fills in on the request
	// itself, for a tool that is one action of a command taking several.
	action string
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
// The three tool-surface profiles a connection may be served under.
// ProfileStation is what one agent working one card through one column
// needs, ProfileOperator adds the workbench and column verbs beside it, and
// ProfileAll, the default, serves the whole surface unfiltered. The set is
// closed: a caller asking for a fourth name is refused before the server
// opens any workbench.
const (
	ProfileStation  = "station"
	ProfileOperator = "operator"
	ProfileAll      = "all"
)

var tools = []tool{
	{name: "claim", command: verb.Claim},
	{name: "move", command: verb.Move},
	{name: "release", command: verb.Release},
	{name: "block", command: verb.Block},
	{name: "unblock", command: verb.Unblock},
	{name: "raise", command: verb.Raise},
	{name: "join_workstream", command: verb.Join},
	{name: "leave_workstream", command: verb.Leave},
	{name: "add_card", command: "add"},
	{name: "comment", command: "comment"},
	{name: "attach", command: "attach"},
	{name: "file_item", command: "file"},
	{name: "cite_item", command: "cite"},
	{name: "resolve_item", command: "resolve"},
	{name: "verify_item", command: "verify"},
	{name: "fail_item", command: "fail"},
	{name: "waive_item", command: "waive"},
	{name: "withdraw_item", command: "withdraw"},
	{name: "reopen_item", command: "reopen"},
	{name: "grant", command: verb.GrantPermission},
	{name: "revoke", command: verb.RevokePermission},
	{name: "settle", command: "settle"},
	{name: "link_card", command: "link"},
	{name: "unlink_card", command: "unlink"},
	{name: "archive", command: "archive"},
	{name: "restore", command: "restore"},
	{name: "delete", command: "delete"},
	{name: "accept_divergence", command: "accept-divergence"},
	{name: "rename", command: "rename"},
	{name: "status", command: "status"},
	{name: "list", command: "list"},
	{name: "next_card", command: "next"},
	{name: "pull", command: verb.Pull},
	{name: "query", command: "query", wrapper: "matches"},
	{name: "search_cards", command: "search", wrapper: "results"},
	{name: "tree", command: "tree", wrapper: "tree"},
	// view answers two shapes, the listing and a drawn view, and no one
	// wrapper can name both, so each shape carries its own member inside the
	// payload on both heads and the tool declares none.
	{name: "view", command: "view"},
	{name: "show", command: "show"},
	{name: "changes", command: "changes"},
	{name: "instructions", command: "instructions"},
	{name: "whoami", command: "whoami"},
	{name: "prime", command: "prime"},
	{name: "workbench", command: "workbench"},
	{name: "workstream", command: "workstream"},
	{name: "get_field", command: "get"},
	{name: "set_field", command: "set"},
	// The tool sets the action itself, since column carries exactly one, and
	// argumentExemptions holds that argument back so the schema never asks a
	// caller for a value the tool has already decided. A later card adding
	// get and set has two ways out of the single-purpose shape, and either one
	// deletes the exemption: dispatch on the action here the way workstream
	// does, or publish the other two acts as tools of their own.
	{name: "new_column", command: "column", action: "new"},
	{name: "version", command: "version"},
	{name: "export", command: "export"},
	{name: "check", command: "check"},
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
	"path":       {GroundShellOrFilesystem, "resolves a filesystem path for a shell to consume, so it means nothing over a protocol"},
	"edit":       {GroundShellOrFilesystem, "opens a file in the reader's own editor, which needs a terminal this head does not have"},
	"init":       {GroundShellOrFilesystem, "creates a workbench in a directory, which is a filesystem act rather than a workbench act"},
	"extract":    {GroundShellOrFilesystem, "copies a workbench definition out to a directory, which is the same filesystem act"},
	"reshape":    {GroundShellOrFilesystem, "reads its new column layout from a definition file or another workbench's directory, which is the same filesystem act init and extract are held out for"},
	"config":     {GroundMachineNotWorkbench, "writes the user's own machine settings, which travel with the person rather than the workbench"},
	"completion": {GroundShellOrFilesystem, "prints a script for the caller's own interactive shell to load, which means nothing over a protocol"},
	"setup":      {GroundMachineNotWorkbench, "writes a harness's configuration files on the caller's machine, which belong to the machine and the person rather than to any workbench"},
	"mcp":        {GroundTheHeadItself, "starts this head, so a tool for it would be the server offering to start itself"},
	"lsp":        {GroundTheHeadItself, "starts a second head, which serves one workbench to an editor over its own protocol on its own stream; a tool for it would be one server offering to start another"},
	"serve":      {GroundTheHeadItself, "starts the HTTP head, so a tool for it would be one server offering to start another"},
	"guide":      {GroundProtocolServesIt, "served as a resource rather than a tool, because a guide is read rather than run"},
	"help":       {GroundProtocolServesIt, "the surface's own tools/list carries every tool's schema and description, which is what help prints at a terminal"},
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
// check's twelve repair markers are held back for the neighbouring reason, and
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
		"root":                 "aims the terminal's two repair sweeps at a tree, and this head runs neither sweep",
		"max-depth":            "bounds the walk root names, and this head takes no root for check",
		"finish":               "completes a half-written store repair, which is operator work taken at a terminal against a copy",
		"remint":               "rewrites the identifier of one of two directories claiming it, which is an irreversible repair an operator decides",
		"yes":                  "confirms the four repairs that read it, and this head offers none of them",
		"witness":              "rebuilds the witness records of an ageing store, which is a one-time repair rather than a reading of it",
		"migrate-ordinals":     "rewrites every card's ordinal in place, which is a one-time repair of an ageing store",
		"migrate-slugs":        "rewrites every column's slug in place, which is a one-time repair of an ageing store",
		"migrate-columns":      "rewrites the column layout of an ageing store, which is a one-time repair of it",
		"migrate-vocabulary":   "rewrites the vocabulary of every workbench under a root, and its rewrite has no undo",
		"migrate-container":    "rewrites the container layout of every workbench under a root, and its rewrite has no undo",
		"migrate-branches":     "lifts a retired heading out of every card body into a declared field and stamps the store's format, which is a one-time repair of an ageing store",
		"migrate-newlines":     "repairs every workbench text file storing a carriage return that stands for a line ending, and its preview rewrites each destination with that destination's own bytes",
		"file-standing":        "mints the standing items every card standing in a declaring column is missing, which is the operator's own repair of a store whose declarations postdate its cards",
		"migrate-numbers":      "builds the card-number registry and strips the number key from every anchor, which is a one-time repair of an ageing store",
		"renumber":             "renumbers the later claimant of a number two cards hold, and a reference somebody wrote down for that card stops resolving",
		"migrate-workstreams":  "rewrites the workstream records of an ageing store, which is a one-time repair of it",
		"migrate-designations": "converts every checklist item's answer of record and stamps the store's format, which is a cutover the workbench operator runs at a moment he picks and which locks every older build out of the store",
		"migrate-applies-when": "stamps the store's format at the one that declares applies_when conditions, which locks every older build out of the store and is the operator's to run at a terminal",
		"migrate-schedule":     "stamps the store's format at the one from which a card carries scheduling dates, which locks every older build out of the store and is the operator's to run at a terminal",
		"migrate-holds":        "stamps the store's format at the one from which a declared link holds a card back from selection, which locks every older build out of the store and is the operator's to run at a terminal",
		"migrate-raw-lines":    "rewrites every quoted raw line on the workbench and column anchors and stamps the store's format, which locks every older build out of the store and is the operator's to run at a terminal",
		"rehearse":             "turns that conversion into a rehearsal, which is the form an agent may run and which this head offers no conversion to rehearse",
		"force-claims":         "carries that conversion past a card somebody still holds, which is a judgement about whose session has died and is the operator's to make at a terminal",
	},
	"new_column": {
		"action": "names the first word of `dinah column new`, and this tool is that one action, so the head fills the field in and a published argument would be a value it overwrites",
	},
	// changes is the fourth case, and it arrives from serveWith's own shape
	// (mcp.go:105-129): one JSON-RPC line read, dispatch blocked on, answer
	// encoded, and only then the next line read, with no context.Context
	// anywhere in this package and no goroutine per call. A blocking wait on
	// this transport would either need request concurrency and cancellation
	// handling built from nothing, or would wedge an agent's only channel to
	// the workbench for up to the caller's own --timeout, a regression this
	// command's "it reports and never dispatches" framing does not license.
	// An MCP caller keeps building its own driving loop, which
	// internal/guide/guides/mcp.md documents. dinah-546.
	"changes": {
		"wait":    "holds the call open until the cursor advances, and this head answers one call before reading the next line on stdin, so a caller blocked here cannot be reached by the cancellation notification the MCP specification defines until the wait itself ends",
		"timeout": "bounds a wait this head does not offer, so a caller has no wait to bound",
	},
	// view's three markers shape a drawing, and this head draws nothing: it
	// answers the view object, which carries every card uncapped. watch is
	// held back for the reason changes' wait is, since it holds the call open
	// until the caller interrupts it. dinah-288.
	"view": {
		"all":   "lifts the per-column cap of a drawing this head never makes, and the answer it returns already carries every card",
		"plain": "chooses the marks of a drawing this head never makes",
		"watch": "redraws the view until interrupted, which holds the call open, and this head answers one call before reading the next line on stdin",
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

// stationMembers are the thirty tools ProfileStation serves: what one agent
// needs to work one card through one column, and nothing that reaches past
// the card it is standing on.
var stationMembers = []string{
	"claim", "move", "release", "block", "comment", "attach",
	"add_card", "file_item", "cite_item", "settle",
	"link_card", "unlink_card", "join_workstream", "leave_workstream", "workstream",
	"get_field", "set_field", "raise",
	"show", "list", "query", "search_cards", "tree", "view", "changes",
	"next_card", "pull", "instructions", "whoami", "prime",
}

// operatorOnlyMembers are the fourteen tools ProfileOperator adds beside
// every station tool: the workbench and column verbs, plus the acts whose
// blast radius is the whole board rather than one card.
//
// grant and revoke join it on the second of those two grounds rather than the
// first. A grant reaches one card, so its blast radius is not the board; both
// verbs are refused to anybody but the workbench operator, and a station
// profile serving a tool every station agent is refused is a tool that exists
// only to produce a refusal.
var operatorOnlyMembers = []string{
	"unblock", "workbench", "new_column",
	"status", "version", "export", "check",
	"archive", "restore", "delete", "rename", "accept_divergence",
	"grant", "revoke",
}

// profileMembership names every tool one of the two narrowed profiles
// serves. ProfileAll names no entry here and is handled as its own case in
// toolsFor: every registered tool, unfiltered, which is what this head
// serves today and what a caller naming no --tools flag, or naming all
// explicitly, goes on seeing.
var profileMembership = map[string][]string{
	ProfileStation:  stationMembers,
	ProfileOperator: append(append([]string{}, stationMembers...), operatorOnlyMembers...),
}

// init validates profileMembership the same way namedTools already validates
// injectedProperties' consumer sets: a name naming no tool this head serves
// panics at package init rather than silently narrowing a profile.
func init() {
	for profile, names := range profileMembership {
		for _, name := range names {
			if _, served := toolsByName[name]; !served {
				panic("mcp: profile " + profile + " names " + name + ", which this head serves no tool for")
			}
		}
	}
}

// toolsFor returns the tools one profile serves, in registry order.
// ProfileAll (or any name profileMembership carries no entry for) returns
// every tool.
func toolsFor(profile string) []tool {
	names, narrowed := profileMembership[profile]
	if !narrowed {
		return tools
	}
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	filtered := make([]tool, 0, len(names))
	for _, t := range tools {
		if set[t.name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// toolsByNameFor is toolsFor indexed for dispatch.
func toolsByNameFor(profile string) map[string]tool {
	index := map[string]tool{}
	for _, t := range toolsFor(profile) {
		index[t.name] = t
	}
	return index
}

// toolList renders one profile's surface for tools/list, with each input
// schema generated from the library's own parameter list.
func toolList(profile string) []map[string]any {
	catalog := msg.For(msg.Base)
	served := toolsFor(profile)
	list := make([]map[string]any, 0, len(served))
	for _, t := range served {
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

// injectedProperties are the seven such properties. None is a parameter, so
// each is resolved by name: actor takes the sentence the global flag row
// already prints, basis takes one written for it, workbench takes one written
// for the address space the MCP head binds, and the four identity properties
// take one each.
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
//
// harness, provider, model and server are read on every call, on actor's own
// argument: the four facts are stamped on whatever the call writes, and a read
// still reports them through whoami. Publishing them on every tool and
// consuming them on every tool is what keeps the schema test honest without an
// exemption.
//
// That test also fails where an injected property and a command's own declared
// parameter answer to one name, which is worth knowing here because model,
// provider, harness and server are ordinary words a later command could want
// as an argument. Any command needing one of the four as a parameter of its own
// has to pick another spelling, and the test is what says so at build time.
var injectedProperties = []injectedProperty{
	{name: "actor", key: "flag.actor.summary", consumers: everyTool()},
	{name: "harness", key: "schema.harness.description", consumers: everyTool()},
	{name: "provider", key: "schema.provider.description", consumers: everyTool()},
	{name: "model", key: "schema.model.description", consumers: everyTool()},
	{name: "server", key: "schema.server.description", consumers: everyTool()},
	{name: "basis", key: "schema.basis.description", consumers: namedTools(
		"claim", "move", "release", "block", "unblock",
		"join_workstream", "leave_workstream", "pull",
	)},
	{name: "workbench", key: "schema.workbench.description",
		consumers: everyTool()},
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
// enum and keeps a strict client free to compose them. A parameter whose set a
// head resolves and which also fixes members in the source is that same case
// read from the other side: the resolved set is wider than the members, so the
// members are published and the enum is not.
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
			// A source that also fixes members in the source publishes them
			// beside it rather than as an enum, because the resolved set is
			// wider than those members and a strict validator told to enforce
			// an enum would reject a legal answer.
			if len(set.Values) > 0 {
				keys["x-dinah-vocabulary-members"] = set.Values
			}
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
