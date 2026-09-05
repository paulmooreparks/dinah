package verb

import (
	"sort"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
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
	// Field names the Request field this parameter's resolved value is
	// assigned to. Empty means the parameter never reaches a Request field,
	// which is true of every parameter belonging to a command that builds no
	// Request at all (guide, init, extract, path, edit, config, mcp, help,
	// and version's own rendering), and of catalogs on version, whose marker
	// runVersion reads straight off the parsed arguments and whose mcp
	// counterpart is deliberately discarded because that head always reports
	// full catalog coverage.
	//
	// The binding is declared here rather than inferred from the parameter's
	// spelling because the spelling does not carry it: yes fills Confirm,
	// ready fills ReadyOnly, and catalogs fills nothing. A parameter that
	// does reach a Request field and carries no Field here is a half-written
	// table rather than a legal state, and the argument-coverage tests on
	// both heads fail on it by name.
	Field string
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
	// outside this package: "columns" reads the workbench's own columns, and
	// "guides" reads the topics embedded in the binary.
	Source string
}

// vocabularies are the closed sets an argument may declare. Two of them name
// the sets the commands themselves check against, so neither can drift from
// what a command accepts, and two name a set only a head can resolve.
var vocabularies = map[string]Vocabulary{
	"key":         {Values: bench.ConfigKeys},
	"field":       {Values: bench.WorkbenchFields},
	"column-kind": {Values: contract.Kinds},
	"topic":       {Source: "guides"},
	"column":      {Source: "columns"},
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
	"attachments":  {"references"},
	"query":        {"query"},
	"search":       {"query"},
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
		{Name: "title", Required: true, Rest: true, Field: "Title"},
		{Name: "column", Flag: true, Value: "column", Vocabulary: "column", Field: "Column"},
		{Name: "severity", Flag: true, Value: "level", Field: "Severity"},
		{Name: "priority", Flag: true, Value: "level", Field: "Priority"},
	},
	Claim: {
		{Name: "card", Required: true, Field: "Card"},
		{Name: "expires", Flag: true, Value: "duration", Field: "Expires"},
	},
	Move: {
		{Name: "card", Required: true, Shared: "card", Field: "Card"},
		{Name: "column", Required: true, Vocabulary: "column", AlsoFlag: true, Field: "Column"},
		{Name: "override", Flag: true, Marker: true, Field: "Override"},
	},
	Release: {{Name: "card", Required: true, Shared: "card", Field: "Card"}},
	Block: {
		{Name: "card", Required: true, Field: "Card"},
		{Name: "reason", Required: true, Rest: true, Field: "Reason"},
		{Name: "kind", Flag: true, Value: "kind", Field: "Kind"},
	},
	Unblock: {{Name: "card", Required: true, Shared: "card", Field: "Card"}},
	Join: {
		{Name: "card", Required: true, Shared: "card", Field: "Card"},
		{Name: "workstream", Required: true, Shared: "workstream", Field: "Workstream"},
	},
	Leave: {
		{Name: "card", Required: true, Shared: "card", Field: "Card"},
		{Name: "workstream", Required: true, Shared: "workstream", Field: "Workstream"},
	},
	"comment": {
		{Name: "card", Required: true, Shared: "card", Field: "Card"},
		{Name: "text", Display: "text|-", Required: true, Rest: true, Field: "Text"},
	},
	"attach": {
		{Name: "ref", Required: true, Guide: "references", Field: "Ref"},
		{Name: "file", Required: true, Field: "File"},
		{Name: "description", Flag: true, Value: "text", Field: "Description"},
		{Name: "replace", Flag: true, Marker: true, Field: "Replace"},
	},
	"archive": {{Name: "ref", Required: true, Shared: "ref", Guide: "references", Field: "Ref"}},
	"delete": {
		{Name: "ref", Required: true, Shared: "ref", Guide: "references", Field: "Ref"},
		{Name: "yes", Flag: true, Marker: true, Required: true, Shared: "yes", Field: "Confirm"},
	},
	// rename writes its own sentence for ref rather than taking the shared
	// one, because the shared sentence names a column, a card or anything
	// below one, and rename renames an attachment alone.
	"rename": {
		{Name: "ref", Required: true, Guide: "references", Field: "Ref"},
		{Name: "name", Required: true, Rest: true, Field: "Value"},
	},
	"status": {
		{Name: "root", Flag: true, Value: "path", Shared: "root", Field: "Root"},
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth", Field: "MaxDepth"},
	},
	"columns": {},
	"ls": {
		{Name: "column", Vocabulary: "column", AlsoFlag: true, Field: "Column"},
		{Name: "ready", Flag: true, Marker: true, Field: "ReadyOnly"},
		{Name: "root", Flag: true, Value: "path", Shared: "root", Field: "Root"},
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth", Field: "MaxDepth"},
	},
	"next": {
		{Name: "column", Vocabulary: "column", AlsoFlag: true, Field: "Column"},
		{Name: "root", Flag: true, Value: "path", Shared: "root", Field: "Root"},
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth", Field: "MaxDepth"},
	},
	// Pull combines a claim and a move into one atomic act. The column is
	// the destination; the named form names it, the bare form chooses the
	// one column that qualifies and refuses when more than one does. The
	// three flags sit on the move's own shape: --no-claim drops the
	// claim half, --override lets the operator pick a column at its limit,
	// and --expires sets the claim's lease exactly as it does on claim.
	Pull: {
		{Name: "column", Vocabulary: "column", AlsoFlag: true, Field: "Column"},
		{Name: "no-claim", Flag: true, Marker: true, Field: "NoClaim"},
		{Name: "expires", Flag: true, Value: "duration", Field: "Expires"},
		{Name: "override", Flag: true, Marker: true, Field: "Override"},
	},
	"query": {{Name: "query", Rest: true, Field: "Query"}},
	// The bare positional is named phrase rather than text because the mcp
	// head's assignValue is one flat switch on parameter name, shared across
	// every command and carrying no way to tell which one originated a call,
	// and comment's own text parameter has already claimed that name there.
	// Every Rest command picks a name the switch has not already taken, for
	// the same reason. The name is never typed by a caller at a terminal,
	// where a Rest positional is resolved by position, so what it changes is
	// the syntax line and the mcp schema's own key, both generated from this
	// one table.
	"search": {
		{Name: "phrase", Required: true, Rest: true, Field: "SearchText"},
		{Name: "query", Flag: true, Value: "terms", Field: "Query"},
		{Name: "archived", Flag: true, Marker: true, Field: "Archived"},
		{Name: "root", Flag: true, Value: "path", Shared: "root", Field: "Root"},
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth", Field: "MaxDepth"},
	},
	"tree": {
		{Name: "query", Rest: true, Field: "Query"},
		{Name: "group-by", Flag: true, Value: "axes", Field: "GroupBy"},
		{Name: "depth", Flag: true, Value: "level", Field: "Depth"},
		{Name: "root", Flag: true, Value: "path", Shared: "root", Field: "Root"},
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth", Field: "MaxDepth"},
	},
	// contents writes its own sentence for ref rather than taking the shared
	// one, because the shared sentence ends "not this workbench" and the
	// workbench is the one reference contents is most often given.
	"contents": {
		{Name: "ref", Required: true, Guide: "references", Field: "Ref"},
		{Name: "depth", Flag: true, Value: "level", Field: "Depth"},
	},
	// attachments writes its own sentence for ref, the way contents does and
	// for the same reason: the shared sentence ends "not this workbench" and
	// the workbench is one of the four kinds this read is asked about.
	"attachments": {{Name: "ref", Guide: "references", Field: "Ref"}},
	"show":        {{Name: "card", Display: "ref", Required: true, Guide: "references", Field: "Card"}},
	"log":         {{Name: "card", Required: true, Shared: "card", Field: "Card"}},
	// Every argument of changes is a flag, including the two a read usually
	// takes positionally, because the cursor is the argument a caller reaches
	// for and a positional slot ahead of it would be the one they type by
	// accident. The card slot takes the shared sentence, since the argument
	// means here what it means everywhere.
	"changes": {
		{Name: "since", Flag: true, Value: "cursor", Field: "Since"},
		{Name: "card", Flag: true, Value: "ref", Shared: "card", Field: "Card"},
		{Name: "column", Flag: true, Value: "column", Vocabulary: "column", Field: "Column"},
		{Name: "root", Flag: true, Value: "path", Shared: "root", Field: "Root"},
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth", Field: "MaxDepth"},
	},
	// instructions keeps its own display, since the two kinds it takes are
	// the whole of what it takes and the spelling says so.
	"instructions": {{Name: "card", Display: "card|column", Required: true, Guide: "references", Field: "Card"}},
	"guide":        {{Name: "topic", Vocabulary: "topic"}},
	// init has always read a positional directory and never declared one, so
	// the syntax line omitted an argument the command honours.
	"init": {
		{Name: "dir"},
		{Name: "from", Flag: true, Value: "source"},
		{Name: "slug", Flag: true, Value: "slug"},
		{Name: "operator", Flag: true, Value: "actor"},
	},
	"export": {},
	// reshape declares no positional at all, so a stray word anywhere in the
	// invocation is refused rather than silently ignored, which is the shape
	// changes already takes for the same reason. --map is the one repeatable
	// argument the tool has: one run retires as many columns as the new
	// definition drops, and each retirement needs its own destination, so a
	// last-value-wins flag would silently discard every entry but the last.
	"reshape": {
		{Name: "from", Flag: true, Required: true, Value: "source", Field: "From"},
		{Name: "map", Flag: true, Value: "retired=destination", Field: "Map"},
		{Name: "yes", Flag: true, Marker: true, Shared: "yes", Field: "Confirm"},
	},
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
		{Name: "action", Display: "get|set", Field: "Action"},
		{Name: "field", Vocabulary: "field", Field: "Field"},
		{Name: "value", Field: "Value"},
		{Name: "yes", Flag: true, Marker: true, Shared: "yes", Field: "Confirm"},
	},
	// The bare invocation lists every live workstream, so neither the action
	// nor the workstream is required; new, get and set still need one, which
	// the command refuses over rather than the syntax line. The workstream
	// slot carries a reference on get and set and the title on new, which is
	// what its two-word display says.
	// The bare invocation is not offered: dinah columns already lists the
	// flow, and a bare dinah column would either duplicate that listing or
	// read as a typo for it. new is the only action this build implements,
	// so the action is required, and get and set are left for the cards that
	// need them.
	"column": {
		{Name: "action", Display: "new", Required: true, Field: "Action"},
		{Name: "column", Display: "title", Required: true, Field: "Column"},
		{Name: "kind", Flag: true, Value: "kind", Vocabulary: "column-kind", Field: "Kind"},
		{Name: "capacity", Flag: true, Value: "n", Field: "Capacity"},
		{Name: "slug", Flag: true, Value: "slug", Field: "Slug"},
		{Name: "before", Flag: true, Value: "column", Field: "Before"},
	},
	"workstream": {
		{Name: "action", Display: "new|get|set", Field: "Action"},
		{Name: "workstream", Display: "workstream|title", Field: "Workstream"},
		{Name: "field", Field: "Field"},
		{Name: "value", Field: "Value"},
		{Name: "slug", Flag: true, Value: "slug", Field: "Slug"},
		{Name: "yes", Flag: true, Marker: true, Shared: "yes", Field: "Confirm"},
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
		{Name: "action", Display: "get|set", Required: true, Field: "Action"},
		{Name: "card", Required: true, Shared: "card", Field: "Card"},
		{Name: "field", Required: true, Field: "Field"},
		{Name: "value", Field: "Value"},
	},
	"check": {
		{Name: "finish", Flag: true, Marker: true, Field: "Finish"},
		{Name: "migrate-ordinals", Flag: true, Marker: true, Field: "MigrateOrdinals"},
		{Name: "migrate-slugs", Flag: true, Marker: true, Field: "MigrateSlugs"},
		{Name: "migrate-columns", Flag: true, Marker: true, Field: "MigrateColumns"},
		{Name: "migrate-vocabulary", Flag: true, Marker: true, Field: "MigrateVocabulary"},
		{Name: "migrate-container", Flag: true, Marker: true, Field: "MigrateContainer"},
		// remint takes a path rather than standing alone, because it repairs
		// the one condition the tree sweep refuses to decide: two directories
		// claiming one identifier. Naming the directory is the whole of the
		// operator's decision, so the flag carries it.
		{Name: "remint", Flag: true, Value: "dir", Field: "Remint"},
		{Name: "migrate-workstreams", Flag: true, Marker: true, Field: "MigrateWorkstreams"},
		{Name: "witness", Flag: true, Marker: true, Field: "MigrateWitness"},
		// Read by migrate-vocabulary and by migrate-container, which are the
		// two repairs here that walk a whole tree of workbenches under --root
		// rather than acting on the one the caller is standing in, and whose
		// rewrites have no undo. Without it either repair reports what it
		// would carry forward and writes nothing. None of the other markers
		// reads it, and check accepts it beside them rather than refusing,
		// because a marker that runs unconditionally is not made more or less
		// dangerous by a confirmation nothing consults.
		{Name: "yes", Flag: true, Marker: true, Shared: "yes", Field: "Confirm"},
		// root and max-depth are the same pair status, ls, next, tree and
		// changes declare, and they carry the same meaning here: absent, the
		// command climbs to the workbench the caller stands in, and present,
		// it walks downward from the named directory instead. They select
		// scope for the two tree sweeps and runCheck refuses them beside
		// anything else, because no other form of check has a downward walk
		// for either flag to bound or to aim.
		{Name: "root", Flag: true, Value: "path", Shared: "root", Field: "Root"},
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth", Field: "MaxDepth"},
	},
	"whoami": {},
	// workbenches takes its scope as a positional rather than as --workbench,
	// because the two name different things: --workbench names one workbench to
	// act on, and this path names a directory to walk downward from. The
	// positional declares no request field, since neither head puts it on a
	// verb.Request; each reads it where it arrives, the terminal off the parsed
	// words and the machine surface off the tool call's own arguments.
	"workbenches": {
		{Name: "path"},
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth", Field: "MaxDepth"},
	},
	"version": {
		{Name: "catalogs", Flag: true, Marker: true},
	},
	"mcp":  {{Name: "root", Flag: true, Value: "dir"}},
	"help": {{Name: "command", Required: true}},
}

// crossHeadIdentical names every command whose reader is required to answer
// with the payload the cli head prints under --json for the same arguments,
// and records the one clause that makes the guarantee hold. The property is
// declared here, beside the parameter table, rather than left in a doc comment
// on the reader itself, because a doc comment is prose that nothing reads and
// a guard built on its wording is a guard that a rewording breaks. A command
// earns its place here by somebody writing the clause and standing behind it,
// which is the same shape each head's exemption map already has.
var crossHeadIdentical = map[string]string{
	"query":   "both heads hand the one library call the one query string",
	"tree":    "both heads hand the one library call the one chain, the one level and the one query string",
	"changes": "both heads hand the one library call the one cursor, the one card and the one state",
}

// CrossHeadIdentical returns the commands whose payload is declared to match
// across the heads, each mapped to the reason the match holds.
func CrossHeadIdentical() map[string]string {
	declared := make(map[string]string, len(crossHeadIdentical))
	for name, reason := range crossHeadIdentical {
		declared[name] = reason
	}
	return declared
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
