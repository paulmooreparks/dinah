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
}

// tools is the whole MCP surface. A command that exists only because a shell
// and a filesystem exist gets no tool, which leaves out path, edit, init,
// extract, config and mcp itself, and guide is a resource rather than a tool.
//
// workbench falls inside that rule rather than outside it, even though config
// does not. A workbench's own fields are workbench state that travels with the
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
	{name: "status", command: "status", run: readStatus},
	{name: "states", command: "states", run: readStates},
	{name: "list_cards", command: "ls", run: readList},
	{name: "next_card", command: "next", run: readNext},
	{name: "query", command: "query", run: readQuery},
	{name: "tree", command: "tree", run: readTree},
	{name: "contents", command: "contents", run: readContents},
	{name: "show", command: "show", run: readShow},
	{name: "log", command: "log", run: readLog},
	{name: "instructions", command: "instructions", run: readInstructions},
	{name: "whoami", command: "whoami", run: readWhoami},
	{name: "workbench", command: "workbench", run: doWorkbench},
	{name: "workstream", command: "workstream", run: doWorkstream},
	{name: "version", command: "version", run: readVersion},
	{name: "export", command: "export", run: readExport},
	{name: "check", command: "check", run: readCheck},
	{name: "workbenches", command: "workbenches", run: nil, summaryKey: "tool.workbenches.summary"},
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

// schemaFor generates one tool's input schema from the parameter list the cli
// head composes its syntax line from, so the two heads cannot drift.
//
// Every property carries the same sentence the cli head prints beside the
// argument, so an agent reading the schema and a person reading the help page
// are told the same thing. The three properties beyond the parameter list are
// resolved by name, because none is a parameter: actor takes the sentence
// the global flag row already prints, basis takes one written for it, and
// workbench takes one written for the address space the MCP head binds.
//
// The workbench property is held out for workbenches itself, which would
// otherwise carry a property whose value the tool does not consume; the
// exclusion is the one case the uniformity rule gives up, since a caller
// asking what the property may say cannot already know the answer.
//
// No property carries an enum. A description is additive and constrains no
// caller, where an enum changes what a strict client will send, which is a
// change to a published machine interface.
func schemaFor(t tool) map[string]any {
	catalog := msg.For(msg.Base)
	properties := map[string]any{}
	var required []string
	for _, param := range verb.Params(t.command) {
		description := catalog.T(param.SummaryKey(t.command))
		properties[param.Name] = map[string]any{"type": param.Type(), "description": description}
		if param.Required {
			required = append(required, param.Name)
		}
	}
	properties["actor"] = map[string]any{"type": "string", "description": catalog.T("flag.actor.summary")}
	properties["basis"] = map[string]any{"type": "string", "description": catalog.T("schema.basis.description")}
	if t.name != "workbenches" {
		properties["workbench"] = map[string]any{"type": "string", "description": catalog.T("schema.workbench.description")}
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
var readAffordances = []string{"status", "states", "list_cards", "next_card"}

// readStatus answers the status tool.
func readStatus(l *verb.Library, r *verb.Request) any {
	status, err := l.Status(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"status": status}, readAffordances)
}

// readStates answers the states tool.
func readStates(l *verb.Library, r *verb.Request) any {
	states, err := l.States()
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"states": states}, readAffordances)
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
func readNext(l *verb.Library, r *verb.Request) any {
	offers, err := l.Next(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"offers": offers}, []string{"claim", "show", "log"})
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

// readShow answers the show tool.
func readShow(l *verb.Library, r *verb.Request) any {
	detail, text, err := l.Show(r)
	if err != nil {
		return l.FromError(r, err)
	}
	if detail == nil {
		return wrap(map[string]any{"text": text}, readAffordances)
	}
	return wrap(map[string]any{"detail": detail}, []string{"claim", "move", "comment", "log"})
}

// readLog answers the log tool.
func readLog(l *verb.Library, r *verb.Request) any {
	events, err := l.History(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return journalView{Events: events, Affordances: []string{"show", "claim"}}
}

// readInstructions answers the instructions tool.
func readInstructions(l *verb.Library, r *verb.Request) any {
	served, err := l.Instructions(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"served": served}, []string{"claim", "move", "show"})
}

// readWhoami answers the whoami tool.
func readWhoami(l *verb.Library, r *verb.Request) any {
	identity, err := l.Whoami(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return wrap(map[string]any{"identity": identity}, readAffordances)
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
	answer := map[string]any{"findings": report.Findings}
	if report.StampedOrdinals != nil {
		answer["stamped_ordinals"] = *report.StampedOrdinals
	}
	return wrap(answer, readAffordances)
}
