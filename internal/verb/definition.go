package verb

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

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

// spelling is the name a reader meets, which is the parameter's own name
// unless it accepts two shapes and Display names the pair.
func (p Param) spelling() string {
	if p.Display != "" {
		return p.Display
	}
	return p.Name
}

// flagWord is the "--name" word a flag is typed as, without the placeholder
// the syntax line shows after a valued one. DeriveCommand renders a real
// argument rather than a syntax line, so it needs the word alone, and taking
// it from here keeps the Display-over-Name resolution in one place.
func (p Param) flagWord() string {
	return "--" + p.spelling()
}

// shown is what the syntax line prints for a parameter, without its brackets.
func (p Param) shown() string {
	if !p.Flag {
		return p.spelling()
	}
	if p.Marker {
		return p.flagWord()
	}
	return p.flagWord() + " <" + p.Value + ">"
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
		{Name: "max-depth", Flag: true, Value: "n", Shared: "max-depth"},
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

// Command is the CLI spelling of a call made through a *Request: the words a
// person would have typed at a terminal to produce the identical request. A
// surface that wants to show its reader what it just did (DinahVisor's command
// log, the pages' own panel, an editor extension) derives one from the request
// it was about to make rather than writing the line out beside every call
// site, which is the hand-maintained mapping table dinah-282 exists to
// prevent.
type Command struct {
	// Verb is the command name, req.Verb unchanged.
	Verb string
	// Args are the argument tokens, in the command's declared order, each
	// spelled the way the syntax line spells it with the placeholder replaced
	// by the value the request actually carries. A valued flag contributes
	// two entries, its "--name" and its value, once per value the field
	// carries; a positional or a bare marker contributes one. Nothing here is
	// shell-quoted, so a caller feeds Args straight to the same argument
	// parser os.Args would reach, with no split-on-whitespace step in
	// between.
	Args []string
}

// Line joins Verb and Args into one string for display, quoting an argument
// that is empty, that carries whitespace, or that carries one of the characters
// shellSpecial names, so a reader can see where each argument begins and where
// it ends. That boundary is the property Line holds. It holds in a POSIX
// shell, in cmd.exe and the Windows C runtime, and in PowerShell, for every
// token that carries none of a double quote, a backtick, an exclamation mark
// or a caret, and it holds in cmd.exe for such a token only while the token
// also carries no line break and no %VAR% whose value carries a double quote
// of its own.
//
// Those four characters are the shapes the quoting does not rescue, and each
// one fails by its own mechanism. A double quote is escaped as \", which a
// POSIX shell and the Windows C runtime read back and PowerShell does not, so
// it costs the boundary in PowerShell alone. A backtick is PowerShell's escape
// character wherever one stands, including against a closing delimiter, and
// inside POSIX double quotes it opens a command substitution whose unmatched
// form runs past that delimiter, so it costs the boundary in both. An
// interactive bash expands an exclamation mark inside double quotes, where the
// one position its manual exempts is immediately before the closing quote, and
// it abandons the line before running it where the designator names no event,
// so the quoting decides neither what the line means nor whether it runs. A
// caret is cmd.exe's escape character, and Microsoft's cmd documentation names
// it as that without saying what one does inside a quoted token, so Line makes
// no claim for it rather than resting one on a measurement of a single console
// build.
//
// The two cmd.exe conditions are not about those four characters, and the
// paragraph on line breaks below carries the first. The second is a value
// arriving from the environment rather than anything in the token, since
// cmd.exe substitutes a %VAR% into the line before it parses one, so a value
// carrying a double quote reopens the quoting after the rendering has been
// read and no rendering of the token can prevent it.
//
// Read the four as the boundary answer, because there is a second question
// with a different answer. shellSpecial's comment gives that one character by
// character: it asks whether the quoting leaves a character meaning something
// other than itself, and the $ and the % are yes there while being quoted
// here. Quoting them is what makes the argument count right, and a shell
// substitutes them inside the quoting all the same, so what a reader retypes
// carries the value the variable holds rather than the value the request did.
//
// The promise is bounded in one more way, which is the physical shape of the
// output rather than what a shell does with it. A value carrying a line break
// is quoted, because a line break is whitespace, and the break is then emitted
// as it stands, so the rendering spans as many lines as the value does and
// cmd.exe cannot carry it at all. A --note or a description value is the
// realistic source. Rendering the break as \n would put characters in the
// output that none of the three shells reads back as a newline, which spends
// the value to buy the display, so Line keeps the value and a surface with one
// row to draw in truncates or elides what it draws.
//
// Line is not a shell-safe rendering and does not try to be one: shellSpecial's
// comment says which characters keep their meaning inside the quoting and where
// a retyped value comes back changed. Re-running a command re-parses Args and
// never this string. It carries no leading program name, because "dinah" is how
// the displaying surface's own reader invokes the binary rather than something
// this library asserts about itself.
func (c Command) Line() string {
	parts := make([]string, 0, len(c.Args)+1)
	parts = append(parts, c.Verb)
	for _, arg := range c.Args {
		parts = append(parts, quoteArgument(arg))
	}
	return strings.Join(parts, " ")
}

// shellSpecial is the set of characters that make Line quote a token. One
// question decides whether a character may be a member, and it is the
// boundary: nothing belongs here unless wrapping the token in double quotes
// leaves the argument count right in all three shells. That is the test the
// three exclusions below fail, and it is a bar to clear rather than a reason
// to be here.
//
// Most members are here because a bare token carrying one changes the count.
// The double quote, the single quote and the backslash each escape or open
// something that swallows the space after the token; the pipe, the ampersand,
// the semicolon, the angle brackets and the parentheses are POSIX operators
// that break the word or consume the token beside it; the star, the question
// mark and the bracket pair expand to as many words as match on disc; a hash
// at the head of a word turns the rest of the line into a comment; and the $
// and the % are substituted, which the paragraphs below take separately
// because they were kept out for four rounds on the wrong question.
//
// The tilde and the equals sign are the two members that clear the bar without
// needing it, and saying so is cheaper than inventing a hazard for them. A
// bare tilde is expanded to a home directory whose text POSIX does not
// field-split, and a bare name=value word is an assignment only in command
// position, which no token Line renders ever occupies, because Line writes the
// verb first. Both change what a token can be read as rather than how many
// tokens there are, and quoting either costs nothing, so both stay.
//
// Whether the quoting also leaves a character meaning itself is a second
// question with a different answer, and reading the two as one list is what
// kept the $ out of this set until the fourth review of this card found it.
//
// The double quote is the delimiter, so quoteArgument escapes it as \", which
// POSIX shells and the Windows C runtime read back and PowerShell does not. It
// is the one member whose boundary the quoting fails to restore on the
// strength of the token alone, and Line's comment names it there beside the
// three shapes that are not members at all. The % fails it too on a value that
// carries a double quote, which is a condition on the environment rather than
// on the token, and the residue paragraph below records that one.
//
// The backslash is here because a bare token ending in one escapes the space
// after it in a POSIX shell and swallows the argument that follows, and
// because quoting a backslash costs nothing: inside double quotes cmd.exe and
// PowerShell read it literally, and a POSIX shell reads it literally too
// unless a ", a backslash, a $ or a backtick follows it.
//
// The $ and the % are here on the boundary reasoning, and they were kept out
// on the second question's answer. A POSIX shell field-splits the result of an
// expansion that did not happen inside double quotes, so a bare $HOME is as
// many arguments as its value has fields, which is none where the variable is
// unset and one only on a machine whose value happens to be a single word.
// Inside double quotes there is no field splitting and no re-parse of what the
// expansion produced, so "$HOME" is one word whatever it holds. cmd.exe
// substitutes %PATH% into the line before it parses one, so a bare %PATH%
// arrives as as many arguments as the substituted text has words, and the
// quoting holds them together. PowerShell expands a $ in both places and
// field-splits neither.
//
// Membership does not mean the quoting silences the character, and for those
// two it does not. A POSIX shell and PowerShell still expand a $ inside double
// quotes, and cmd.exe still expands a %VAR% there, so the token a reader
// retypes still names a variable rather than carrying the text the request
// held. That is the same cost the residue paragraph below records for the
// backslash, and it buys the one property Line promises.
//
// Three characters a shell reads specially stay out, each for its own reason.
// A backtick stays out because quoting does not restore the boundary for one
// at all: PowerShell escapes the delimiter a backtick stands against, and a
// POSIX shell reading an unmatched substitution runs straight past it, so the
// word never terminates. An exclamation mark stays out because an interactive
// bash expands one inside double quotes, exempting only the position
// immediately before the closing quote, and abandons the line where the
// designator names no event, so quoting changes which failure happens rather
// than whether one does. The caret, which is cmd.exe's escape character, stays
// out because Microsoft's cmd documentation names it as that without saying
// what one does inside a quoted token, and a membership decision resting on a
// measurement of one console build expires without announcing itself.
//
// Read what leaving a character out does and does not mean, because the
// obvious reading is wrong. It means no token is quoted on account of that
// character. It does not mean such a token is printed bare, because whitespace
// and every character in the set quote a token too, and a card title is
// usually several words. So `ship it! by friday` renders as
// "ship it! by friday", quoted for its spaces, with the ! sitting inside the
// quoting and its meaning to an interactive bash intact. That is the ordinary
// input rather than a corner, and it is the reason Line promises a readable
// boundary and nothing about retyping.
//
// What quoteArgument escapes, it escapes narrowly. Doubling every backslash
// would repair a Windows path for a POSIX shell and double every separator of
// it for the two shells this tool is most often used from, which is the trade
// the previous version of this rendering got wrong in both directions. So an
// interior backslash is left alone, and only a run sitting immediately before
// a double quote is doubled. That is the rule CommandLineToArgvW documents,
// and a POSIX shell reads it the same way. Without it a token ending in a
// backslash escapes its own closing delimiter, and `dir C:\temp\` followed by
// `next` prints as one argument instead of two.
//
// The residue is worth stating rather than implying away, because no rendering
// removes it: the three shells disagree about what a backslash inside quotes
// means, so a retyped value can come back changed even where the boundary is
// right. A POSIX shell drops a backslash that precedes another backslash, a $
// or a backtick. PowerShell keeps both halves of a run this rendering doubled,
// so a token ending in a backslash comes back from PowerShell carrying one too
// many. A $ and a % come back carrying whatever the variable holds, for the
// reason the membership paragraphs give. The % owes one caveat more, because
// cmd.exe substitutes before it parses: a value holding a double quote of its own
// reopens the quoting and costs the boundary after all, which no rendering of
// the token can prevent, since that text arrives after the rendering has been
// read. A caller that wants to re-run the call uses Args, which is unquoted
// and passes through none of this.
const shellSpecial = "\"'\\|&;<>()*?[]#~=$%"

// quoteArgument renders one token so that a reader can tell where it begins
// and where it ends. An empty token has no bare spelling at all, so it becomes
// a pair of quotes. Inside the quoting a double quote is escaped, and a run of
// backslashes is doubled when a double quote follows it and left alone
// otherwise, for the reasons shellSpecial gives.
func quoteArgument(arg string) string {
	if arg == "" {
		return `""`
	}
	plain := !strings.ContainsAny(arg, shellSpecial)
	if plain && strings.IndexFunc(arg, unicode.IsSpace) < 0 {
		return arg
	}
	var quoted strings.Builder
	quoted.WriteByte('"')
	// writeBackslashes emits a run one byte at a time rather than through
	// strings.Repeat, which TestNoRowIsLaidOutOutsideTheOneRenderer reads as
	// the way a run of column padding is built. This run is an escape sequence
	// and not a column, so the loop states that rather than earning an
	// exemption from a guard whose reasoning is right.
	writeBackslashes := func(n int) {
		for ; n > 0; n-- {
			quoted.WriteByte('\\')
		}
	}
	// backslashes counts the run currently open. An ordinary byte ends the run
	// and flushes it as it stands, a double quote ends it and flushes it
	// doubled, and the end of the token flushes it doubled as well, because
	// the closing delimiter is the double quote that follows it.
	backslashes := 0
	for i := 0; i < len(arg); i++ {
		switch arg[i] {
		case '\\':
			backslashes++
		case '"':
			writeBackslashes(2*backslashes + 1)
			quoted.WriteByte('"')
			backslashes = 0
		default:
			writeBackslashes(backslashes)
			backslashes = 0
			quoted.WriteByte(arg[i])
		}
	}
	writeBackslashes(2 * backslashes)
	quoted.WriteByte('"')
	return quoted.String()
}

// durationType is the one non-string scalar a parameter's Request field is
// declared as today. It is compared by type rather than by kind because a
// Duration is an int64 underneath, and any other int64 field would render as a
// bare number ParseDuration refuses.
var durationType = reflect.TypeOf(time.Duration(0))

// DeriveCommand composes the CLI spelling of req: the command name followed
// by, for every parameter Params(req.Verb) declares, the token or tokens that
// parameter's own current value would have been typed as. Each parameter reads
// its value off the Request field its own Field names, through the same
// declaration the two heads already resolve their arguments through, so a
// command gains a log entry by being declared rather than by anybody writing a
// second table.
//
// ok is false in two cases. First, when req is nil, when req.Verb names no
// command Params declares, or when it names one in derivationExemptions: no
// Request is ever built for such a command by any head today, so there is
// nothing here to read a value from, and reason carries that exemption's own
// text verbatim. Second, when a parameter's Field names a Request field of a
// type renderParamValue does not know how to render. Rather than guess at a
// plausible-looking value for a type nobody taught this function, DeriveCommand
// refuses the whole derivation, because a Command that renders every argument
// but one wrong is worse than no Command at all, and reason names the command,
// the parameter and the field's type.
func DeriveCommand(req *Request) (cmd Command, ok bool, reason string) {
	if req == nil {
		return Command{}, false, "there is no request to derive a command from"
	}
	if exemption, exempt := derivationExemptions[req.Verb]; exempt {
		return Command{}, false, exemption
	}
	declared, defined := params[req.Verb]
	if !defined {
		return Command{}, false, req.Verb + " names no command this library defines"
	}
	fields := reflect.ValueOf(req).Elem()
	var args []string
	for _, p := range declared {
		field := fields.FieldByName(p.Field)
		if p.Marker {
			if field.Bool() {
				args = append(args, p.flagWord())
			}
			continue
		}
		values, known := renderParamValue(field)
		if !known {
			unrenderable := "%s's %q parameter declares the field %s of type %s, which DeriveCommand does not know how to render"
			return Command{}, false, fmt.Sprintf(unrenderable, req.Verb, p.Name, p.Field, field.Type())
		}
		if len(values) == 0 {
			if !p.Required {
				continue
			}
			// A required argument the request arrived without is still
			// typed, as an empty word, because the verb is what refuses an
			// incomplete request and a line that quietly dropped the
			// argument would not reproduce that refusal.
			values = []string{""}
		}
		for _, value := range values {
			if p.Flag {
				args = append(args, p.flagWord())
			}
			args = append(args, value)
		}
	}
	return Command{Verb: req.Verb, Args: args}, true, ""
}

// renderParamValue reads one non-marker parameter's resolved value off its
// Request field and returns the token or tokens it contributes, in the order a
// caller would have typed them, and whether the field's type is one this
// function knows how to render at all. ok is false only for a type nobody has
// taught it, never for an empty value of a known type: an empty string, a zero
// duration and a zero-length slice all return ok=true and no values, and
// DeriveCommand's own Required handling decides what that means for the
// command line.
//
// A zero duration counts as an empty value for the same reason an empty string
// does. Nothing in a Request distinguishes a flag given empty from a flag left
// out, so a claim that named no lease would otherwise derive a line carrying
// "--expires 0s", which reproduces the call and is not what anybody typed.
//
// The empty-value rule meets DeriveCommand's Required handling in one place
// that has no case today. A Required duration field left at zero would be
// typed as an empty word, and ParseDuration refuses that, so the derived line
// would not reparse. No parameter declares one: expires is the only duration
// in the table and it is Required on neither claim nor pull. The round trip in
// cmd/dinah catches it the day one appears.
//
// Three types are known today: string, time.Duration (rendered as ParseDuration
// accepts it straight back), and a slice whose element type is string. A
// repeatable flag like reshape's --map is declared as a []string field for
// exactly this reason: the flag's own repetition is carried by the slice's
// length rather than by a second Param.
func renderParamValue(field reflect.Value) (values []string, ok bool) {
	if field.Type() == durationType {
		lease := time.Duration(field.Int())
		if lease == 0 {
			return nil, true
		}
		return []string{lease.String()}, true
	}
	switch field.Kind() {
	case reflect.String:
		if field.String() == "" {
			return nil, true
		}
		return []string{field.String()}, true
	case reflect.Slice:
		if field.Type().Elem().Kind() != reflect.String {
			return nil, false
		}
		rendered := make([]string, 0, field.Len())
		for i := 0; i < field.Len(); i++ {
			rendered = append(rendered, field.Index(i).String())
		}
		return rendered, true
	}
	return nil, false
}

// derivationExemptions names every command DeriveCommand cannot compose a
// spelling for, mapped to why. A command earns its place here the same way a
// head's own toolExemptions and argumentExemptions do: by somebody writing the
// reason down, checked by TestEveryCommandDerivesOrIsExempted.
var derivationExemptions = map[string]string{
	"config":      "the terminal dispatches config on its own parsed arguments and never builds a Request for it",
	"edit":        "opens a path in the reader's own editor; the terminal never builds a Request for it",
	"extract":     "Library.Extract takes a target string, not a *Request; there is no request to read a value from",
	"guide":       "prints an embedded guide; the terminal never builds a Request for it",
	"help":        "prints a command's own help; the terminal never builds a Request for it",
	"init":        "creates a workbench in a directory; the terminal never builds a Request for it",
	"mcp":         "starts this head; the terminal never builds a Request for it",
	"path":        "resolves a filesystem path for a shell; the terminal never builds a Request for it",
	"version":     "runVersion reads catalogs straight off the parsed arguments; no Request carries it",
	"workbenches": "enumerates the workbenches on disk directly rather than through a Library verb (dinah-282); no Request is ever built for it",
	"export":      "Library.Export takes no arguments at all; there is no request to read a value from",
}

// DerivationExemptions returns a copy of derivationExemptions, for a caller
// deciding ahead of a call whether a log entry is possible at all.
func DerivationExemptions() map[string]string {
	exempted := make(map[string]string, len(derivationExemptions))
	for name, reason := range derivationExemptions {
		exempted[name] = reason
	}
	return exempted
}
