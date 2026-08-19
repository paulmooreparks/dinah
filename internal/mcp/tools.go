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
type tool struct {
	// name is what an agent calls.
	name string
	// command is the library command whose parameter list generates this
	// tool's input schema.
	command string
	// run answers the call with a value that marshals to the canonical form.
	run func(*verb.Library, *verb.Request) any
}

// tools is the whole MCP surface. A command that exists only because a shell
// and a filesystem exist gets no tool, which leaves out path, edit, init,
// extract, config and mcp itself, and guide is a resource rather than a tool.
var tools = []tool{
	{name: "claim", command: verb.Claim, run: doVerb},
	{name: "move", command: verb.Move, run: doVerb},
	{name: "release", command: verb.Release, run: doVerb},
	{name: "block", command: verb.Block, run: doVerb},
	{name: "unblock", command: verb.Unblock, run: doVerb},
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
	{name: "show", command: "show", run: readShow},
	{name: "log", command: "log", run: readLog},
	{name: "instructions", command: "instructions", run: readInstructions},
	{name: "whoami", command: "whoami", run: readWhoami},
	{name: "version", command: "version", run: readVersion},
	{name: "export", command: "export", run: readExport},
	{name: "check", command: "check", run: readCheck},
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
		entry := map[string]any{
			"name":        t.name,
			"description": catalog.T("cmd." + t.command + ".summary"),
			"inputSchema": schemaFor(t.command),
		}
		list = append(list, entry)
	}
	return list
}

// schemaFor generates one tool's input schema from the parameter list the cli
// head composes its syntax line from, so the two heads cannot drift.
func schemaFor(command string) map[string]any {
	properties := map[string]any{}
	var required []string
	for _, param := range verb.Params(command) {
		properties[param.Name] = map[string]any{"type": param.Type()}
		if param.Required {
			required = append(required, param.Name)
		}
	}
	properties["actor"] = map[string]any{"type": "string"}
	properties["basis"] = map[string]any{"type": "string"}
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
