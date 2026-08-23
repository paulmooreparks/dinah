package verb

import (
	"sort"
	"strings"

	"dinah/internal/bench"
)

// Param is one argument of a command. The list of them is the single
// definition both heads project: the cli head composes its syntax line from
// it, and the mcp head generates its input schema from it, so the two
// surfaces cannot drift apart.
type Param struct {
	// Name is the argument's name, which is the member name on the machine
	// surface and the catalog key's last segment.
	Name string
	// Display is what the syntax line shows inside its brackets, when that
	// differs from the name because the argument accepts two shapes.
	Display string
	// Flag marks an argument given as --name rather than by position.
	Flag bool
	// Marker marks a flag that carries no value.
	Marker bool
	// Required marks an argument the command cannot run without.
	Required bool
	// Value is the placeholder a valued flag shows.
	Value string
	// Rest marks a positional argument that takes every remaining word,
	// which is what a prose reason or a title needs.
	Rest bool
	// Shared names the argument whose written meaning this parameter takes,
	// which is the parameter's own name wherever one sentence serves several
	// commands. Empty means the meaning is written for this command alone, at
	// param.<command>.<name>.summary. A parameter resolving neither form, or
	// both, fails its test rather than printing a sentence nobody wrote for
	// it.
	Shared string
	// Vocabulary names the closed set of values this argument accepts, as a
	// key into vocabularies. Empty means the argument takes free text, or
	// that its own Display already spells the set.
	Vocabulary string
	// Guide is the topic of the guide that explains this argument's language,
	// when one argument needs more than a table cell.
	Guide string
	// AlsoFlag marks a positional argument the parser also accepts in its
	// --name <value> spelling.
	AlsoFlag bool
}

// Type is the JSON schema type of a parameter on a machine surface.
func (p Param) Type() string {
	if p.Marker {
		return "boolean"
	}
	return "string"
}

// shown is what the syntax line prints for a parameter, without its brackets.
func (p Param) shown() string {
	name := p.Name
	if p.Display != "" {
		name = p.Display
	}
	if !p.Flag {
		return name
	}
	if p.Marker {
		return "--" + name
	}
	return "--" + name + " <" + p.Value + ">"
}

// Token is how a parameter is spelled wherever a reader meets it: on the
// syntax line, in the arguments table of its command's help, and in any repair
// a refusal quotes. One function composes it, so no two surfaces can spell one
// argument two ways.
//
// A word written exactly as shown carries no brackets, a word the reader
// replaces sits in angle brackets, and an argument the reader may leave out
// sits in square brackets whatever is inside them. Nothing else marks either
// category.
func (p Param) Token() string {
	shown := p.shown()
	if p.Required && p.Flag {
		return shown
	}
	if p.Required {
		return "<" + shown + ">"
	}
	return "[" + shown + "]"
}

// SummaryKey is where an argument's written meaning lives: the shared key the
// parameter names, or the key written for this command alone. Both heads
// resolve it here, the way each already resolves a check's key here, so the
// page a person reads and the schema an agent reads carry one sentence.
func (p Param) SummaryKey(command string) string {
	if p.Shared != "" {
		return "param." + p.Shared + ".summary"
	}
	return "param." + command + "." + p.Name + ".summary"
}

// Tokens is a command's argument spellings, in the order the syntax line
// prints them.
func Tokens(name string) []string {
	declared := params[name]
	tokens := make([]string, 0, len(declared))
	for _, p := range declared {
		tokens = append(tokens, p.Token())
	}
	return tokens
}

// Vocabulary is a closed set of values an argument accepts.
type Vocabulary struct {
	// Values are the members when the set is fixed in the source.
	Values []string
	// Source names a set a head resolves when it runs, because the set lives
	// outside this package: "states" reads the workbench's own states, and
	// "guides" reads the topics embedded in the binary.
	Source string
}

// vocabularies are the closed sets an argument may declare. Two of them name
// the sets the commands themselves check against, so neither can drift from
// what a command accepts, and two name a set only a head can resolve.
var vocabularies = map[string]Vocabulary{
	"key":   {Values: bench.ConfigKeys},
	"field": {Values: bench.WorkbenchFields},
	"topic": {Source: "guides"},
	"state": {Source: "states"},
}

// VocabularyFor returns the set one argument accepts, and whether it declares
// one at all, so a head asks for one argument's vocabulary without reaching
// into the map.
func VocabularyFor(command, param string) (Vocabulary, bool) {
	for _, p := range params[command] {
		if p.Name != param {
			continue
		}
		if p.Vocabulary == "" {
			return Vocabulary{}, false
		}
		set, ok := vocabularies[p.Vocabulary]
		return set, ok
	}
	return Vocabulary{}, false
}

// VocabularySources lists the distinct source names the declared vocabularies
// name, sorted. A head resolving a source reads this to check that it can
// answer every one of them, rather than finding out in front of a reader that
// it cannot.
func VocabularySources() []string {
	seen := map[string]bool{}
	for _, set := range vocabularies {
		if set.Source == "" {
			continue
		}
		seen[set.Source] = true
	}
	sources := make([]string, 0, len(seen))
	for source := range seen {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return sources
}

// guides are the guide topics a command as a whole points its reader at, where
// a parameter's own Guide points at one for a single argument.
var guides = map[string][]string{
	"path":         {"references"},
	"edit":         {"references"},
	"show":         {"references"},
	"instructions": {"references"},
	"attach":       {"references"},
	"archive":      {"references"},
	"delete":       {"references"},
	"rename":       {"references"},
	"contents":     {"references"},
	"query":        {"query"},
}

// Guides lists the guide topics a command's help points at: the command's own,
// then any a parameter declares, each named once and in declaration order.
func Guides(name string) []string {
	var topics []string
	seen := map[string]bool{}
	for _, topic := range guides[name] {
		if seen[topic] {
			continue
		}
		seen[topic] = true
		topics = append(topics, topic)
	}
	for _, p := range params[name] {
		if p.Guide == "" || seen[p.Guide] {
			continue
		}
		seen[p.Guide] = true
		topics = append(topics, p.Guide)
	}
	return topics
}

// params is every command's argument list.
var params = map[string][]Param{
	// Neither level flag declares a Vocabulary. The two vocabularies fixed in
	// this source are sets the source itself holds, and a level set is per
	// workbench, so the refusal is what tells a reader the set.
	"add": {
		{Name: "title", Required: true, Rest: true},
		{Name: "state", Flag: true, Value: "state", Vocabulary: "state"},
		{Name: "severity", Flag: true, Value: "level"},
		{Name: "priority", Flag: true, Value: "level"},
	},
	Claim: {
		{Name: "card", Required: true},
		{Name: "expires", Flag: true, Value: "duration"},
	},
	Move: {
		{Name: "card", Required: true, Shared: "card"},
		{Name: "state", Required: true, Vocabulary: "state", AlsoFlag: true},
		{Name: "override", Flag: true, Marker: true},
	},
	Release: {{Name: "card", Required: true, Shared: "card"}},
	Block: {
		{Name: "card", Required: true},
		{Name: "reason", Required: true, Rest: true},
		{Name: "kind", Flag: true, Value: "kind"},
	},
	Unblock: {{Name: "card", Required: true, Shared: "card"}},
	Join: {
		{Name: "card", Required: true, Shared: "card"},
		{Name: "workstream", Required: true, Shared: "workstream"},
	},
	Leave: {
		{Name: "card", Required: true, Shared: "card"},
		{Name: "workstream", Required: true, Shared: "workstream"},
	},
	"comment": {
		{Name: "card", Required: true, Shared: "card"},
		{Name: "text", Display: "text|-", Required: true, Rest: true},
	},
	"attach": {
		{Name: "ref", Required: true, Guide: "references"},
		{Name: "file", Required: true},
		{Name: "description", Flag: true, Value: "text"},
		{Name: "replace", Flag: true, Marker: true},
	},
	"archive": {{Name: "ref", Required: true, Shared: "ref", Guide: "references"}},
	"delete": {
		{Name: "ref", Required: true, Shared: "ref", Guide: "references"},
		{Name: "yes", Flag: true, Marker: true, Required: true, Shared: "yes"},
	},
	// rename writes its own sentence for ref rather than taking the shared
	// one, because the shared sentence names a state, a card or anything
	// below one, and rename renames an attachment alone.
	"rename": {
		{Name: "ref", Required: true, Guide: "references"},
		{Name: "name", Required: true, Rest: true},
	},
	"status": {},
	"states": {},
	"ls": {
		{Name: "state", Vocabulary: "state", AlsoFlag: true},
		{Name: "ready", Flag: true, Marker: true},
	},
	"next": {{Name: "state", Vocabulary: "state", AlsoFlag: true}},
	// Pull combines a claim and a move into one atomic act. The state is
	// the destination; the named form names it, the bare form chooses the
	// one state that qualifies and refuses when more than one does. The
	// three flags sit on the move's own shape: --no-claim drops the
	// claim half, --override lets the operator pick a state at its limit,
	// and --expires sets the claim's lease exactly as it does on claim.
	Pull: {
		{Name: "state", Vocabulary: "state", AlsoFlag: true},
		{Name: "no-claim", Flag: true, Marker: true},
		{Name: "expires", Flag: true, Value: "duration"},
		{Name: "override", Flag: true, Marker: true},
	},
	"query": {{Name: "query", Rest: true}},
	"tree": {
		{Name: "query", Rest: true},
		{Name: "group-by", Flag: true, Value: "axes"},
		{Name: "depth", Flag: true, Value: "level"},
	},
	// contents writes its own sentence for ref rather than taking the shared
	// one, because the shared sentence ends "not this workbench" and the
	// workbench is the one reference contents is most often given.
	"contents": {
		{Name: "ref", Required: true, Guide: "references"},
		{Name: "depth", Flag: true, Value: "level"},
	},
	"show": {{Name: "card", Display: "ref", Required: true, Guide: "references"}},
	"log":  {{Name: "card", Required: true, Shared: "card"}},
	// Every argument of changes is a flag, including the two a read usually
	// takes positionally, because the cursor is the argument a caller reaches
	// for and a positional slot ahead of it would be the one they type by
	// accident. The card slot takes the shared sentence, since the argument
	// means here what it means everywhere.
	"changes": {
		{Name: "since", Flag: true, Value: "cursor"},
		{Name: "card", Flag: true, Value: "ref", Shared: "card"},
		{Name: "state", Flag: true, Value: "state", Vocabulary: "state"},
	},
	// instructions keeps its own display, since the two kinds it takes are
	// the whole of what it takes and the spelling says so.
	"instructions": {{Name: "card", Display: "card|state", Required: true, Guide: "references"}},
	"guide":        {{Name: "topic", Vocabulary: "topic"}},
	// init has always read a positional directory and never declared one, so
	// the syntax line omitted an argument the command honours.
	"init": {
		{Name: "dir"},
		{Name: "from", Flag: true, Value: "source"},
		{Name: "slug", Flag: true, Value: "slug"},
		{Name: "operator", Flag: true, Value: "actor"},
	},
	"export":  {},
	"extract": {{Name: "dir", Required: true}},
	"path":    {{Name: "card", Display: "ref", Required: true, Guide: "references"}},
	"edit":    {{Name: "card", Display: "ref", Required: true, Guide: "references"}},
	// The bare invocation lists every setting, so neither the action nor the
	// key is required; `get` and `set` still need a key, which the command
	// refuses over rather than the syntax line.
	"config": {
		{Name: "action", Display: "get|set"},
		{Name: "key", Vocabulary: "key"},
		{Name: "value"},
	},
	// The bare invocation lists all three fields, so neither the action nor
	// the field is required; get and set still need one, which the command
	// refuses over rather than the syntax line. The confirmation flag is
	// unrequired because a title change and an operator change take none.
	"workbench": {
		{Name: "action", Display: "get|set"},
		{Name: "field", Vocabulary: "field"},
		{Name: "value"},
		{Name: "yes", Flag: true, Marker: true, Shared: "yes"},
	},
	// The bare invocation lists every live workstream, so neither the action
	// nor the workstream is required; new, get and set still need one, which
	// the command refuses over rather than the syntax line. The workstream
	// slot carries a reference on get and set and the title on new, which is
	// what its two-word display says.
	"workstream": {
		{Name: "action", Display: "new|get|set"},
		{Name: "workstream", Display: "workstream|title"},
		{Name: "field"},
		{Name: "value"},
		{Name: "yes", Flag: true, Marker: true, Shared: "yes"},
	},
	// Three of the four slots are required, which is where card departs from
	// its three grammar siblings. Each of those has a bare invocation that
	// lists something, so neither the action nor the entity can be required
	// there; card has no bare form, since a card reference is needed before
	// the command means anything and `dinah show` already prints what a card
	// holds. Required is read by Param.Token and by the mcp head's schema
	// generator and by nothing in the cli parser, so runCard still runs its
	// own arity check; what the declaration buys is a schema that refuses a
	// call naming no action, no card or no field before the tool runs.
	"card": {
		{Name: "action", Display: "get|set", Required: true},
		{Name: "card", Required: true, Shared: "card"},
		{Name: "field", Required: true},
		{Name: "value"},
	},
	"check": {
		{Name: "finish", Flag: true, Marker: true},
		{Name: "migrate-ordinals", Flag: true, Marker: true},
		{Name: "migrate-slugs", Flag: true, Marker: true},
		{Name: "migrate-states", Flag: true, Marker: true},
		{Name: "migrate-workstreams", Flag: true, Marker: true},
	},
	"whoami":      {},
	"workbenches": {},
	"version": {
		{Name: "catalogs", Flag: true, Marker: true},
	},
	"mcp":  {{Name: "root", Flag: true, Value: "dir"}},
	"help": {{Name: "command", Required: true}},
}

// Params returns a command's argument list.
func Params(name string) []Param {
	return params[name]
}

// Usage composes a command's syntax line: the command's own name followed by
// its argument spellings, each composed by Token so that the line and the
// arguments table of the command's help cannot spell one argument two ways.
func Usage(name string) string {
	parts := append([]string{name}, Tokens(name)...)
	return strings.TrimSpace(strings.Join(parts, " "))
}

// Commands lists every command name the library defines, which is what a head
// enumerates when it projects the surface.
func Commands() []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	return names
}
