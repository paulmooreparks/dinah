package bench

import (
	"strings"
)

// The frontmatter keys `dinah run` reads. RunKey names the workbench's block
// of named recipes and, on a column, the one recipe that column runs; the two
// share a name because a column's value is a name the workbench's block
// declares, so a reader meets one word for one idea.
const (
	RunKey    = "run"
	WorkerKey = "worker"
)

// The two values a column's worker key may take. Continue resumes the agent
// session that last worked the card, and fresh starts a new one, which is what
// a review station wants because its value is independence. A column that
// declares nothing continues.
const (
	WorkerContinue = "continue"
	WorkerFresh    = "fresh"
)

// RunRecipe is one named recipe from the workbench's run block: the command
// that starts a fresh session, the command that resumes a stored one, the
// working directory and the names under which the harness's receipt reports
// what `dinah run` reads back.
//
// Nothing here knows any one harness. A recipe is argv templates and field
// names, so a recipe for Claude Code, one for Codex and a script that runs a
// test suite and prints a receipt are the same shape.
type RunRecipe struct {
	// Name is the key the recipe is declared under, which a column names.
	Name string
	// Command is the argv that starts a fresh session. It is never empty on a
	// recipe ParseRunRecipes answers.
	Command []string
	// Resume is the argv that continues a stored session, empty where the
	// recipe declares none, in which case every run is fresh.
	Resume []string
	// Cwd is the working-directory template, empty for the default.
	Cwd string
	// Receipt names the receipt members the run reads.
	Receipt RunReceipt
}

// RunReceipt names, by a dotted path into the JSON object the harness prints
// last on its standard output, each member a run reads. Text is required,
// because the agent's result block travels in it; the others are optional.
type RunReceipt struct {
	// Session is the member carrying the harness's own session identifier.
	Session string
	// Text is the member carrying the agent's final text.
	Text string
	// Error is the member carrying a boolean the harness sets when the
	// session ended in error, empty where the recipe names none.
	Error string
	// Spend is how the receipt reports what the session cost, nil where the
	// recipe records no spend.
	Spend *RunSpend
}

// RunSpend is the spend member of a receipt: the unit the figure is counted
// in, the dotted path to the figure, and whether the harness reports the
// figure cumulatively across a resumed session.
type RunSpend struct {
	Unit       string
	Field      string
	Cumulative bool
}

// RunRecipeDefect names one recipe the run block declares and that cannot be
// used, with the member at fault. A defective recipe is not answered as a
// recipe, so a column naming it is refused rather than run on a guess.
type RunRecipeDefect struct {
	Name   string
	Member string
}

// RunRecipes reads the workbench's run block, answering the usable recipes
// by name and the defects of the rest. The block is read on demand rather
// than at Open, because only `dinah run` reads it and a workbench nobody runs
// pays nothing for it.
func (b *Bench) RunRecipes() (map[string]*RunRecipe, []RunRecipeDefect) {
	if b.FM == nil {
		return map[string]*RunRecipe{}, nil
	}
	return ParseRunRecipes(b.FM)
}

// ColumnRunRecipe is the name of the recipe a column runs, empty where it
// names none, which is how a station a person works says so.
func ColumnRunRecipe(column *Column) string {
	if column == nil || column.FM == nil {
		return ""
	}
	return column.FM.Value(RunKey)
}

// ColumnWorker answers a column's worker declaration, defaulting to continue,
// and false where the column declares a value that is neither of the two.
func ColumnWorker(column *Column) (string, bool) {
	if column == nil || column.FM == nil {
		return WorkerContinue, true
	}
	switch declared := column.FM.Value(WorkerKey); declared {
	case "":
		return WorkerContinue, true
	case WorkerContinue, WorkerFresh:
		return declared, true
	default:
		return declared, false
	}
}

// ParseRunRecipes reads a run block out of a header. Each recipe is a mapping
// under its name carrying command, and optionally resume, cwd and receipt.
// Argv members are written as a flow sequence or as dashed entries; the
// receipt and its spend member as a flow mapping or as a nested block. The
// reader is the small subset of YAML the documented example needs, written
// here rather than borrowed from the interchange's block reader, because that
// reader splits a dashed entry at its first colon and a flow sequence at every
// comma, and an argv element may carry either.
func ParseRunRecipes(fm *Frontmatter) (map[string]*RunRecipe, []RunRecipeDefect) {
	recipes := map[string]*RunRecipe{}
	lines := fm.Raw(RunKey)
	if len(lines) < 2 {
		return recipes, nil
	}
	tree, ok := runNode(readChildren(lines[1:])).(map[string]any)
	if !ok {
		return recipes, []RunRecipeDefect{{Name: RunKey, Member: RunKey}}
	}
	var defects []RunRecipeDefect
	for _, name := range runKeysOf(lines[1:]) {
		body, ok := tree[name].(map[string]any)
		if !ok {
			defects = append(defects, RunRecipeDefect{Name: name, Member: name})
			continue
		}
		recipe, member := runRecipeOf(name, body)
		if member != "" {
			defects = append(defects, RunRecipeDefect{Name: name, Member: member})
			continue
		}
		recipes[name] = recipe
	}
	return recipes, defects
}

// runKeysOf answers the recipe names in the order the block declares them,
// which is the order a defect list reports them in.
func runKeysOf(lines []string) []string {
	children := readChildren(lines)
	if len(children) == 0 {
		return nil
	}
	shallowest := children[0].indent
	for _, child := range children {
		if child.indent < shallowest {
			shallowest = child.indent
		}
	}
	var names []string
	seen := map[string]bool{}
	for _, child := range children {
		if child.indent != shallowest {
			continue
		}
		name, _, found := runCutMember(child.text)
		if !found || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// runRecipeOf builds one recipe from its parsed mapping, answering the member
// at fault where the mapping will not do.
func runRecipeOf(name string, body map[string]any) (*RunRecipe, string) {
	recipe := &RunRecipe{Name: name}
	command, ok := runArgv(body["command"])
	if !ok || len(command) == 0 {
		return nil, "command"
	}
	recipe.Command = command
	if raw, declared := body["resume"]; declared {
		resume, ok := runArgv(raw)
		if !ok || len(resume) == 0 {
			return nil, "resume"
		}
		recipe.Resume = resume
	}
	if raw, declared := body["cwd"]; declared {
		cwd, ok := raw.(string)
		if !ok {
			return nil, "cwd"
		}
		recipe.Cwd = cwd
	}
	receipt, ok := runMapping(body["receipt"])
	if !ok {
		return nil, "receipt"
	}
	for _, member := range []struct {
		key  string
		into *string
	}{
		{"session", &recipe.Receipt.Session},
		{"text", &recipe.Receipt.Text},
		{"error", &recipe.Receipt.Error},
	} {
		raw, declared := receipt[member.key]
		if !declared {
			continue
		}
		value, ok := raw.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, "receipt." + member.key
		}
		*member.into = value
	}
	if recipe.Receipt.Text == "" {
		return nil, "receipt.text"
	}
	if raw, declared := receipt["spend"]; declared {
		spend, ok := runMapping(raw)
		if !ok {
			return nil, "receipt.spend"
		}
		unit, _ := spend["unit"].(string)
		field, _ := spend["field"].(string)
		if !runUnitLegal(unit) {
			return nil, "receipt.spend.unit"
		}
		if strings.TrimSpace(field) == "" {
			return nil, "receipt.spend.field"
		}
		recipe.Receipt.Spend = &RunSpend{Unit: unit, Field: field}
		switch cumulative, _ := spend["cumulative"].(string); cumulative {
		case "", "false":
		case "true":
			recipe.Receipt.Spend.Cumulative = true
		default:
			return nil, "receipt.spend.cumulative"
		}
	}
	return recipe, ""
}

// runUnitLegal admits a spend unit that is also a legal segment of a declared
// field key, because a cumulative unit is stored under run.cumulative.<unit>
// and a unit that could not be stored there could not be accounted for.
func runUnitLegal(unit string) bool {
	return unit != "" && DeclaredFieldKey("run.cumulative."+unit)
}

// RunCumulativeField is the declared card field a cumulative unit's last
// reported figure is stored under.
func RunCumulativeField(unit string) string {
	return "run.cumulative." + unit
}

// The two declared card fields every run writes.
const (
	RunSessionField = "run.session"
	RunRecipeField  = "run.recipe"
)

// runArgv reads an argv member: a sequence of strings, or a string that is
// itself a flow sequence, which is what an interchange round trip leaves a
// flow sequence as.
func runArgv(raw any) ([]string, bool) {
	if text, isText := raw.(string); isText && strings.HasPrefix(strings.TrimSpace(text), "[") {
		raw = runFlow(strings.TrimSpace(text))
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	argv := make([]string, 0, len(list))
	for _, item := range list {
		word, ok := item.(string)
		if !ok {
			return nil, false
		}
		argv = append(argv, word)
	}
	return argv, true
}

// runMapping reads a mapping member, accepting a string that is itself a flow
// mapping for the reason runArgv accepts a flow sequence.
func runMapping(raw any) (map[string]any, bool) {
	if text, isText := raw.(string); isText && strings.HasPrefix(strings.TrimSpace(text), "{") {
		raw = runFlow(strings.TrimSpace(text))
	}
	mapping, ok := raw.(map[string]any)
	return mapping, ok
}

// runNode reads a block of child lines as a mapping, a sequence or nothing.
// The shallowest indent decides the shape, as childValue decides it for the
// interchange.
func runNode(children []blockLine) any {
	if len(children) == 0 {
		return nil
	}
	shallowest := children[0].indent
	for _, child := range children {
		if child.indent < shallowest {
			shallowest = child.indent
		}
	}
	first := ""
	for _, child := range children {
		if child.indent == shallowest {
			first = child.text
			break
		}
	}
	if strings.HasPrefix(first, "-") {
		var list []any
		for _, child := range children {
			if child.indent != shallowest || !strings.HasPrefix(child.text, "-") {
				continue
			}
			list = append(list, runScalar(strings.TrimSpace(strings.TrimPrefix(child.text, "-"))))
		}
		return list
	}
	mapping := map[string]any{}
	for i, child := range children {
		if child.indent != shallowest {
			continue
		}
		name, rest, found := runCutMember(child.text)
		if !found {
			continue
		}
		if _, seen := mapping[name]; seen {
			continue
		}
		if strings.TrimSpace(rest) != "" {
			mapping[name] = runScalar(strings.TrimSpace(rest))
			continue
		}
		var deeper []blockLine
		for _, follower := range children[i+1:] {
			if follower.indent <= shallowest {
				break
			}
			deeper = append(deeper, follower)
		}
		mapping[name] = runNode(deeper)
	}
	return mapping
}

// runCutMember splits a `name: value` line at the colon ending its name. A
// quoted name is unquoted, and a line with no colon is not a member.
func runCutMember(text string) (string, string, bool) {
	name, rest, found := strings.Cut(text, ":")
	if !found {
		return "", "", false
	}
	name = unquote(strings.TrimSpace(name))
	if name == "" || strings.HasPrefix(name, "-") {
		return "", "", false
	}
	return name, rest, true
}

// runScalar reads the text after a colon or a dash: a flow collection, a
// quoted string, or a bare word with any trailing annotation comment dropped.
func runScalar(text string) any {
	if strings.HasPrefix(text, "[") || strings.HasPrefix(text, "{") {
		return runFlow(text)
	}
	if quoted(text) {
		return unquote(text)
	}
	return stripComment(text)
}

// runFlow parses one flow collection. A collection that does not close, or
// text left over after it, reads as nil, which every caller treats as a
// malformed member rather than a guess.
func runFlow(text string) any {
	p := &runFlowParser{text: text}
	value, ok := p.value(false)
	if !ok {
		return nil
	}
	p.space()
	if p.at < len(p.text) && !strings.HasPrefix(p.text[p.at:], "#") {
		return nil
	}
	return value
}

// runFlowParser walks a flow collection one byte at a time. Every delimiter
// it looks for is ASCII, so a byte walk never splits a multi-byte character
// that matters.
type runFlowParser struct {
	text string
	at   int
}

func (p *runFlowParser) space() {
	for p.at < len(p.text) && (p.text[p.at] == ' ' || p.text[p.at] == '\t') {
		p.at++
	}
}

// value reads one value. A key is read with key set, which stops a bare word
// at the colon that ends it.
func (p *runFlowParser) value(key bool) (any, bool) {
	p.space()
	if p.at >= len(p.text) {
		return nil, false
	}
	switch p.text[p.at] {
	case '[':
		return p.sequence()
	case '{':
		return p.mapping()
	case '"', '\'':
		return p.quotedText()
	}
	start := p.at
	for p.at < len(p.text) {
		c := p.text[p.at]
		if c == ',' || c == ']' || c == '}' || key && c == ':' {
			break
		}
		p.at++
	}
	word := strings.TrimSpace(p.text[start:p.at])
	return word, word != ""
}

func (p *runFlowParser) quotedText() (any, bool) {
	open := p.text[p.at]
	start := p.at
	p.at++
	for p.at < len(p.text) {
		c := p.text[p.at]
		if open == '"' && c == '\\' {
			p.at += 2
			continue
		}
		if c == open {
			p.at++
			return unquote(p.text[start:p.at]), true
		}
		p.at++
	}
	return nil, false
}

func (p *runFlowParser) sequence() (any, bool) {
	p.at++
	list := []any{}
	for {
		p.space()
		if p.at < len(p.text) && p.text[p.at] == ']' {
			p.at++
			return list, true
		}
		item, ok := p.value(false)
		if !ok {
			return nil, false
		}
		list = append(list, item)
		p.space()
		if p.at >= len(p.text) {
			return nil, false
		}
		switch p.text[p.at] {
		case ',':
			p.at++
		case ']':
			p.at++
			return list, true
		default:
			return nil, false
		}
	}
}

func (p *runFlowParser) mapping() (any, bool) {
	p.at++
	mapping := map[string]any{}
	for {
		p.space()
		if p.at < len(p.text) && p.text[p.at] == '}' {
			p.at++
			return mapping, true
		}
		rawKey, ok := p.value(true)
		name, isText := rawKey.(string)
		if !ok || !isText {
			return nil, false
		}
		p.space()
		if p.at >= len(p.text) || p.text[p.at] != ':' {
			return nil, false
		}
		p.at++
		item, ok := p.value(false)
		if !ok {
			return nil, false
		}
		if _, seen := mapping[name]; !seen {
			mapping[name] = item
		}
		p.space()
		if p.at >= len(p.text) {
			return nil, false
		}
		switch p.text[p.at] {
		case ',':
			p.at++
		case '}':
			p.at++
			return mapping, true
		default:
			return nil, false
		}
	}
}
