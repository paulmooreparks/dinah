package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"dinah/internal/bench"
	"dinah/internal/completion"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
	"dinah/internal/setup"
	"dinah/internal/verb"
)

// completionScript is the machine form of dinah completion: the shell named,
// the protocol the script speaks, and the script itself.
type completionScript struct {
	// Shell is the shell the script is for.
	Shell string `json:"shell"`
	// Protocol is the version of the callback conversation the script speaks.
	Protocol int `json:"protocol"`
	// Script is the script text, exactly as the human form prints it.
	Script string `json:"script"`
}

// runCompletion prints the completion script for one shell. It opens no
// workbench and resolves no actor, so it answers the same from anywhere.
func runCompletion(s *session, parsed *arguments) int {
	shell := at(parsed.rest(), 0)
	script, ok := completion.Script(shell)
	if !ok {
		return s.reportError(contract.Refuse(contract.UnknownShell, shell))
	}
	if s.format != formatHuman {
		return s.emitMachine(completionScript{Shell: shell, Protocol: completion.Protocol, Script: script})
	}
	io.WriteString(s.out, script)
	return 0
}

// completeCallback is the hidden word the completion scripts call back on.
// The leading double underscore keeps it out of every namespace a person or
// an alias uses.
const completeCallback = "__complete"

// completeWordsVariable is the environment variable the PowerShell script
// hands the words in, because Windows PowerShell does not escape an embedded
// quotation mark when it passes an argument to a native program.
const completeWordsVariable = "DINAH_COMPLETE_WORDS"

// completeLimit is the most candidates one answer carries.
const completeLimit = 200

// The deadline is what keeps a Tab from hanging on a slow disk. The callback
// checks it before each file its own reads open, and once it has passed the
// answer is the header alone, because a partial list reads as the whole one.
//
// A filesystem call that never returns blocks the callback past the deadline
// all the same, and no script can interrupt it portably. That case is not
// handled, and nothing here claims it is.
var (
	// completeDeadline is how long a callback may read before it gives up.
	completeDeadline = 1000 * time.Millisecond
	// completeExpireAfterOpens is -1 in the build. A test setting it to n
	// has the deadline count as passed once the callback's own reads have
	// opened n files, so it can run out of time halfway through a read.
	completeExpireAfterOpens = -1
	// completeOpens counts the files the callback's own reads opened in the
	// current call, so a test can see where the expiry fired.
	completeOpens int
)

// errCompletionExpired is the deadline passing, which discards every
// candidate collected and answers the header alone.
var errCompletionExpired = errors.New("completion ran out of time")

// runComplete is the callback every completion script runs on a Tab. It is
// intercepted in run ahead of alias expansion and parsing, because what it is
// handed is an unfinished command line the parser would refuse. It writes to
// out alone, and only once the whole answer is known, so a malformed call and
// a recovered panic both leave the terminal untouched and exit 1.
func runComplete(argv []string, out io.Writer, home string, cfg *bench.Config) (code int) {
	defer func() {
		if recover() != nil {
			code = 1
		}
	}()
	completeOpens = 0
	call, ok := readCompletionCall(argv)
	if !ok {
		return 1
	}
	call.home = home
	call.cfg = cfg
	call.start = time.Now()
	mode, candidates, err := call.answer()
	if err != nil {
		mode = completion.ModeWords
		candidates = nil
	}
	var answer bytes.Buffer
	describe := call.shell != "bash"
	if err := completion.Write(&answer, mode, candidates, call.replaceFrom, describe); err != nil {
		return 1
	}
	out.Write(answer.Bytes())
	return 0
}

// completionCall is one Tab: the shell, the words the shell handed over, and
// what the callback has opened while answering.
type completionCall struct {
	// shell is the shell the script calling back runs in.
	shell string
	// prior are the words before the one being completed, after the program.
	prior []string
	// current is the word being completed, up to the cursor.
	current string
	// replaceFrom is the rune index in current at which the shell's own
	// replacement begins, so the insert is a candidate less that many runes.
	replaceFrom int
	// silent marks a well-formed call that completes nothing, such as a
	// PowerShell word that does not end with the text the shell replaces.
	silent bool
	// home and cfg are the user base and its settings, as run loaded them.
	home string
	cfg  *bench.Config
	// start is when the callback began, which the deadline counts from.
	start time.Time
	// walk is what the words before the current one said.
	walk *completionWalk
	// r renders descriptions in the language the line resolves to.
	r *msg.Renderer
	// s is the session discovery and the fields listing run through, built
	// the first time a completer needs one.
	s *session
	// opened says the workbench has been tried, and library is what the
	// attempt gave, nil when it refused.
	opened  bool
	library *verb.Library
}

// completeWords is the object the PowerShell script hands over.
type completeWords struct {
	// Words are the prior words followed by the current one.
	Words *[]string `json:"words"`
	// Replacing is PowerShell's own word to complete, which is the part of
	// the current word a completion result replaces.
	Replacing *string `json:"replacing"`
}

// readCompletionCall reads the callback's arguments, and answers false for
// any shape the protocol does not define.
func readCompletionCall(argv []string) (*completionCall, bool) {
	if len(argv) < 2 || argv[0] != strconv.Itoa(completion.Protocol) {
		return nil, false
	}
	call := &completionCall{shell: argv[1]}
	rest := argv[2:]
	switch call.shell {
	case "bash":
		if len(rest) != 2 {
			return nil, false
		}
		words, current, replaceFrom, found := completion.SplitBash(rest[1], rest[0])
		call.prior, call.current, call.replaceFrom = words, current, replaceFrom
		call.silent = !found
	case "zsh", "fish":
		if len(rest) < 2 || rest[0] != "--" {
			return nil, false
		}
		words := rest[1:]
		call.prior = words[:len(words)-1]
		call.current = words[len(words)-1]
	case "powershell":
		if len(rest) != 0 {
			return nil, false
		}
		return readPowerShellWords(call)
	default:
		return nil, false
	}
	return call, true
}

// readPowerShellWords reads the words the PowerShell script put in the
// environment, and computes how much of the current word PowerShell replaces.
func readPowerShellWords(call *completionCall) (*completionCall, bool) {
	raw, set := os.LookupEnv(completeWordsVariable)
	if !set {
		return nil, false
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var handed completeWords
	if err := decoder.Decode(&handed); err != nil {
		return nil, false
	}
	if decoder.More() || handed.Words == nil || handed.Replacing == nil || len(*handed.Words) == 0 {
		return nil, false
	}
	words := *handed.Words
	call.prior = words[:len(words)-1]
	call.current = words[len(words)-1]
	replacing := *handed.Replacing
	if !strings.HasSuffix(call.current, replacing) {
		call.silent = true
		return call, true
	}
	call.replaceFrom = utf8.RuneCountInString(call.current) - utf8.RuneCountInString(replacing)
	return call, true
}

// completionWalk is what the words before the current one said, read the way
// parseArgs would read them.
type completionWalk struct {
	// dashDash says the end-of-options marker has been seen.
	dashDash bool
	// command is the command word, empty until one is seen.
	command string
	// positionals are the command's own positional words after it.
	positionals []string
	// given are the flags already on the line, and values the value each
	// valued one carried.
	given  map[string]bool
	values map[string]string
	// pending is the valued flag the current word is the value of, empty
	// when it is not a flag's value.
	pending string
	// nothing says the line cannot be completed past where it stands, which
	// is an alias carrying a placeholder or a defect.
	nothing bool
}

// walkWords reads the prior words. An alias carrying no placeholder is
// replaced by its tokens and the words read again, once, which is the one
// expansion run itself makes.
func walkWords(prior []string, cfg *bench.Config) *completionWalk {
	valued := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
	}
	return walkFrom(prior, cfg, valued, false)
}

// walkFrom is walkWords with the valued set built and the expansion state
// carried, so an alias is expanded at most once.
func walkFrom(prior []string, cfg *bench.Config, valued map[string]bool, expanded bool) *completionWalk {
	w := &completionWalk{given: map[string]bool{}, values: map[string]string{}}
	for i := 0; i < len(prior); i++ {
		word := prior[i]
		if !w.dashDash && word == "--" {
			w.dashDash = true
			continue
		}
		if !w.dashDash && strings.HasPrefix(word, "--") {
			name, value, joined := strings.Cut(word[2:], "=")
			w.given[name] = true
			switch {
			case joined:
				w.values[name] = value
			case valued[name] && i+1 < len(prior):
				i++
				w.values[name] = prior[i]
			case valued[name]:
				w.pending = name
			}
			continue
		}
		if w.command != "" {
			w.positionals = append(w.positionals, word)
			continue
		}
		if commandNames()[word] || expanded {
			w.command = word
			continue
		}
		alias, found := cfg.AliasNamed(word)
		if !found {
			w.command = word
			continue
		}
		if alias.Defect != "" || alias.Highest > 0 {
			w.nothing = true
			return w
		}
		spliced := append(append(append([]string{}, prior[:i]...), alias.Tokens...), prior[i+1:]...)
		return walkFrom(spliced, cfg, valued, true)
	}
	return w
}

// answer completes the current word by the first rule that applies.
func (c *completionCall) answer() (string, []completion.Candidate, error) {
	if c.silent {
		return completion.ModeWords, nil, nil
	}
	c.walk = walkWords(c.prior, c.cfg)
	c.r = msg.For(bench.ResolveLang(c.walk.values["lang"], c.cfg))
	w := c.walk
	if w.nothing {
		return completion.ModeWords, nil, nil
	}
	// A flag's value is read ahead of the rule for the first word, so that
	// dinah --workbench <Tab> completes a directory rather than a command.
	if w.pending != "" {
		return c.flagValue(w.pending, "", c.current)
	}
	if !w.dashDash && strings.HasPrefix(c.current, "--") {
		name, value, joined := strings.Cut(c.current[2:], "=")
		if joined {
			return c.flagValue(name, "--"+name+"=", value)
		}
	}
	if w.command == "" {
		if strings.HasPrefix(c.current, "-") && !w.dashDash {
			return c.keep(completion.ModeWords, c.flagList(""), true)
		}
		return c.keep(completion.ModeWords, c.commandsAndAliases(), false)
	}
	if _, known := lookup(w.command); !known {
		return completion.ModeWords, nil, nil
	}
	if strings.HasPrefix(c.current, "-") && !w.dashDash {
		return c.keep(completion.ModeWords, c.flagList(w.command), true)
	}
	return c.positional()
}

// positional completes the current word as the command's positional argument
// in the slot it stands in.
func (c *completionCall) positional() (string, []completion.Candidate, error) {
	slot := len(c.walk.positionals)
	var owner *verb.Param
	index := 0
	for _, param := range verb.Params(c.walk.command) {
		if param.Flag {
			continue
		}
		if index == slot || (param.Rest && index <= slot) {
			found := param
			owner = &found
			break
		}
		index++
	}
	if owner == nil {
		if c.current == "" && !c.walk.dashDash {
			return c.keep(completion.ModeWords, c.flagList(c.walk.command), true)
		}
		return completion.ModeWords, nil, nil
	}
	if owner.AlsoFlag && c.walk.given[owner.Name] {
		return completion.ModeWords, nil, nil
	}
	return c.valueOf(*owner, "", c.current)
}

// flagValue completes the current word as the value of a valued flag, which
// is one the command declares or one of the global flags. outer is what the
// word carries ahead of the value, the flag itself in the --name= spelling.
func (c *completionCall) flagValue(name, outer, value string) (string, []completion.Candidate, error) {
	for _, param := range verb.Params(c.walk.command) {
		if param.Name == name && (param.AlsoFlag || (param.Flag && !param.Marker)) {
			return c.valueOf(param, outer, value)
		}
	}
	for _, flag := range globalFlags {
		if flag.name != name || flag.marker {
			continue
		}
		mode, values, err := c.resolveGlobal(flag.complete)
		if err != nil {
			return "", nil, err
		}
		return c.compose(mode, values, outer, "")
	}
	return completion.ModeWords, nil, nil
}

// valueOf completes a value of one parameter. A parameter whose value is a
// comma list completes the part after the last comma, and every candidate
// carries what comes before it.
func (c *completionCall) valueOf(param verb.Param, outer, value string) (string, []completion.Candidate, error) {
	list := ""
	if param.Value == "list" {
		if cut := strings.LastIndex(value, ","); cut >= 0 {
			list = value[:cut+1]
			value = value[cut+1:]
		}
	}
	mode, values, err := c.resolve(param, value)
	if err != nil {
		return "", nil, err
	}
	return c.compose(mode, values, outer, list)
}

// compose puts what the word carries ahead of each value back in front of it,
// and keeps what matches the whole current word. A value may not begin with a
// dash, since only the flag list offers a word that does.
func (c *completionCall) compose(mode string, values []completion.Candidate, outer, list string) (string, []completion.Candidate, error) {
	whole := make([]completion.Candidate, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(value.Word, "-") {
			continue
		}
		whole = append(whole, completion.Candidate{Word: outer + list + value.Word, Description: value.Description})
	}
	return c.keep(mode, whole, true)
}

// keep filters candidates to those starting with the current word under ASCII
// case folding and passing the safe-word test, and cuts the list at the limit.
// dashes says a word beginning with a dash may stand, which is true of the
// flag list and of a value already checked for one.
func (c *completionCall) keep(mode string, candidates []completion.Candidate, dashes bool) (string, []completion.Candidate, error) {
	var kept []completion.Candidate
	for _, candidate := range candidates {
		if !hasFoldPrefix(candidate.Word, c.current) || !completion.SafeWord(candidate.Word) {
			continue
		}
		if !dashes && strings.HasPrefix(candidate.Word, "-") {
			continue
		}
		kept = append(kept, candidate)
		if len(kept) == completeLimit {
			break
		}
	}
	return mode, kept, nil
}

// hasFoldPrefix reports whether s starts with prefix, folding ASCII letters
// and nothing else.
func hasFoldPrefix(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if asciiLower(s[i]) != asciiLower(prefix[i]) {
			return false
		}
	}
	return true
}

// asciiLower folds one ASCII capital to its lower case and leaves every other
// byte as it is.
func asciiLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// flagList offers the command's own flags and then the global flags, each
// with its meaning as the description. An empty command offers the global
// flags alone.
func (c *completionCall) flagList(command string) []completion.Candidate {
	var flags []completion.Candidate
	for _, param := range verb.Params(command) {
		if !param.Flag {
			continue
		}
		meaning := verb.ArgumentMeaning(command, param, c.r.T)
		flags = append(flags, completion.Candidate{Word: "--" + param.Spelling(), Description: meaning})
	}
	for _, flag := range globalFlags {
		summary := c.r.T("flag." + flag.name + ".summary")
		flags = append(flags, completion.Candidate{Word: "--" + flag.name, Description: summary})
	}
	return flags
}

// commandsAndAliases offers every command in the table's order, then every
// alias carrying no defect, by name.
func (c *completionCall) commandsAndAliases() []completion.Candidate {
	var names []completion.Candidate
	for _, command := range commands {
		summary := c.r.T("cmd." + command.name + ".summary")
		names = append(names, completion.Candidate{Word: command.name, Description: summary})
	}
	for _, alias := range c.cfg.Aliases() {
		if alias.Defect != "" {
			continue
		}
		names = append(names, completion.Candidate{Word: alias.Name, Description: alias.Template})
	}
	return names
}

// resolve completes one parameter's value by the completer it declares, or by
// its vocabulary where it declares none.
func (c *completionCall) resolve(param verb.Param, value string) (string, []completion.Candidate, error) {
	if param.Complete != "" {
		resolver, ok := completers[param.Complete]
		if !ok {
			return completion.ModeWords, nil, nil
		}
		return resolver(c, param, value)
	}
	set, ok := verb.VocabularyFor(c.walk.command, param.Name)
	if !ok {
		return completion.ModeWords, nil, nil
	}
	switch set.Source {
	case "":
		return completion.ModeWords, plainWords(set.Values), nil
	case columnsVocabulary:
		return completers[verb.CompleteColumn](c, param, value)
	case fieldsVocabulary:
		return completers[verb.CompleteEntityField](c, param, value)
	case guidesVocabulary:
		return completion.ModeWords, plainWords(guide.Topics()), nil
	}
	return completion.ModeWords, nil, nil
}

// guidesVocabulary is the vocabulary source that reads the embedded guides.
const guidesVocabulary = "guides"

// resolveGlobal completes the value of a global flag by the completer its row
// in globalFlags names.
func (c *completionCall) resolveGlobal(name string) (string, []completion.Candidate, error) {
	if resolver, ok := sessionCompleters[name]; ok {
		return completion.ModeWords, resolver(), nil
	}
	resolver, ok := completers[name]
	if !ok {
		return completion.ModeWords, nil, nil
	}
	return resolver(c, verb.Param{}, "")
}

// The two completers only a global flag names, since the values they offer
// belong to the invocation rather than to any command's argument.
const (
	completeFormats   = "formats"
	completeLanguages = "languages"
)

// sessionCompleters resolve the global flags whose values no parameter takes.
var sessionCompleters = map[string]func() []completion.Candidate{
	completeFormats: func() []completion.Candidate {
		return plainWords([]string{string(formatJSON), string(formatCompact)})
	},
	completeLanguages: func() []completion.Candidate { return plainWords(msg.Tags()) },
}

// plainWords are candidates carrying no description.
func plainWords(words []string) []completion.Candidate {
	candidates := make([]completion.Candidate, 0, len(words))
	for _, word := range words {
		candidates = append(candidates, completion.Candidate{Word: word})
	}
	return candidates
}

// completer resolves one parameter's value: the mode the script works in and
// the candidates, each a whole value the person means. value is the part of
// the current word the value occupies, which a completer reading files uses
// to filter before it opens any.
type completer func(c *completionCall, param verb.Param, value string) (string, []completion.Candidate, error)

// completers resolve every member of verb.Completers, one entry each.
var completers = map[string]completer{
	verb.CompleteNone: func(*completionCall, verb.Param, string) (string, []completion.Candidate, error) {
		return completion.ModeWords, nil, nil
	},
	verb.CompleteFiles: func(*completionCall, verb.Param, string) (string, []completion.Candidate, error) {
		return completion.ModeFiles, nil, nil
	},
	verb.CompleteDirs: func(*completionCall, verb.Param, string) (string, []completion.Candidate, error) {
		return completion.ModeDirs, nil, nil
	},
	verb.CompleteCommand: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		return completion.ModeWords, c.commandsAndAliases(), nil
	},
	verb.CompleteCard: func(c *completionCall, _ verb.Param, value string) (string, []completion.Candidate, error) {
		cards, err := c.cards(value, "")
		return completion.ModeWords, cards, err
	},
	verb.CompleteItem: func(c *completionCall, _ verb.Param, value string) (string, []completion.Candidate, error) {
		cards, err := c.cards(value, "/")
		return completion.ModeNospace, cards, err
	},
	verb.CompleteCardOrColumn: func(c *completionCall, _ verb.Param, value string) (string, []completion.Candidate, error) {
		cards, err := c.cards(value, "")
		return completion.ModeWords, append(cards, c.columns()...), err
	},
	verb.CompleteReference: func(c *completionCall, _ verb.Param, value string) (string, []completion.Candidate, error) {
		references, err := c.references(value)
		return completion.ModeWords, references, err
	},
	verb.CompleteListing: func(c *completionCall, _ verb.Param, value string) (string, []completion.Candidate, error) {
		references, err := c.references(value)
		return completion.ModeWords, append(plainWords(verb.RosterWords), references...), err
	},
	verb.CompleteColumn: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		return completion.ModeWords, c.columns(), nil
	},
	verb.CompleteMoveDestination: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		destinations, err := c.moveDestinations()
		return completion.ModeWords, destinations, err
	},
	verb.CompleteWorkstream: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		return completion.ModeWords, c.workstreams(), nil
	},
	verb.CompleteSeverity: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		return completion.ModeWords, c.levels(bench.SeverityField), nil
	},
	verb.CompletePriority: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		return completion.ModeWords, c.levels(bench.PriorityField), nil
	},
	verb.CompleteTier: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		return completion.ModeWords, c.levels(bench.TierField), nil
	},
	verb.CompleteRoute: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		library := c.bench()
		if library == nil {
			return completion.ModeWords, nil, nil
		}
		return completion.ModeWords, plainWords(library.Bench.RouteNames), nil
	},
	verb.CompleteRecipe: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		return completion.ModeWords, c.recipes(), nil
	},
	verb.CompleteSetting: func(*completionCall, verb.Param, string) (string, []completion.Candidate, error) {
		return completion.ModeWords, plainWords(bench.ConfigKeys), nil
	},
	verb.CompleteEntityField: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		s := c.session()
		c.bench()
		return completion.ModeWords, plainWords(refusalListings[fieldsVocabulary](s)), nil
	},
	verb.CompleteSetValue: func(c *completionCall, _ verb.Param, _ string) (string, []completion.Candidate, error) {
		values, err := c.setValues()
		return completion.ModeWords, values, err
	},
	verb.CompleteQuery: func(c *completionCall, _ verb.Param, value string) (string, []completion.Candidate, error) {
		return c.queryTerm(value)
	},
	verb.CompleteDisplay: func(_ *completionCall, param verb.Param, _ string) (string, []completion.Candidate, error) {
		var words []string
		for _, word := range strings.Split(param.Display, "|") {
			if word != "-" {
				words = append(words, word)
			}
		}
		return completion.ModeWords, plainWords(words), nil
	},
}

// session is the session discovery and the listings run through, built the
// first time a completer needs one exactly as run builds its own, with both
// streams discarded so nothing it does can reach the answer.
func (c *completionCall) session() *session {
	if c.s != nil {
		return c.s
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	benchFlag, benchFlagSource := bench.Resolve(
		bench.Layer{Source: bench.SourceFlag, Value: c.walk.values["workbench"]},
		bench.Layer{Source: bench.SourceEnvironment, Value: os.Getenv("DINAH_WORKBENCH")},
	)
	c.s = &session{
		out:             io.Discard,
		errw:            io.Discard,
		in:              strings.NewReader(""),
		r:               c.r,
		home:            c.home,
		nativeHome:      bench.NativeHome(),
		cfg:             c.cfg,
		benchFlag:       benchFlag,
		benchFlagSource: benchFlagSource,
		cwd:             cwd,
	}
	return c.s
}

// bench opens the workbench the line names or the directory climbs to, at
// most once per callback, and answers nil where it will not open. The
// strict opener is the one used, so a store the ordinary commands refuse
// offers no candidates from it either.
func (c *completionCall) bench() *verb.Library {
	if c.opened {
		return c.library
	}
	c.opened = true
	library, err := c.session().open()
	if err != nil {
		return nil
	}
	library.Bench.BeforeHeaderRead = c.gate
	c.library = library
	return library
}

// gate is asked before each file the callback's own reads open. It answers
// the expiry once the deadline has passed, and counts the open otherwise.
func (c *completionCall) gate() error {
	if completeExpireAfterOpens >= 0 && completeOpens >= completeExpireAfterOpens {
		return errCompletionExpired
	}
	if time.Since(c.start) > completeDeadline {
		return errCompletionExpired
	}
	completeOpens++
	return nil
}

// describes says whether the shell shows descriptions, which decides whether
// a card's title is worth opening its anchor for.
func (c *completionCall) describes() bool {
	return c.shell != "bash"
}

// cards offers the live cards whose reference starts with the value, highest
// number first, each followed by suffix. The match runs on the number
// registry before any card file is opened, an archived card costs one stat,
// and a title is read only for a shell that shows it.
func (c *completionCall) cards(value, suffix string) ([]completion.Candidate, error) {
	library := c.bench()
	if library == nil {
		return nil, nil
	}
	b := library.Bench
	if b.Slug == "" || b.Numbers == nil {
		return c.cardsByIdentifier(b, value, suffix)
	}
	var numbers []int
	for number := range b.Numbers.ByNumber {
		reference := b.Slug + "-" + strconv.Itoa(number)
		if hasFoldPrefix(reference, value) {
			numbers = append(numbers, number)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(numbers)))
	var cards []completion.Candidate
	for _, number := range numbers {
		live, err := c.firstLive(b, b.Numbers.ByNumber[number])
		if err != nil {
			return nil, err
		}
		if live == "" {
			continue
		}
		title, err := c.title(b, live)
		if err != nil {
			return nil, err
		}
		reference := b.Slug + "-" + strconv.Itoa(number)
		cards = append(cards, completion.Candidate{Word: reference + suffix, Description: title})
		if len(cards) == completeLimit {
			break
		}
	}
	return cards, nil
}

// cardsByIdentifier is cards for a workbench whose references are bare
// identifiers, in the order ListIDs answers them.
func (c *completionCall) cardsByIdentifier(b *bench.Bench, value, suffix string) ([]completion.Candidate, error) {
	ids, err := bench.ListIDs(b.CardsRoot())
	if err != nil {
		return nil, nil
	}
	var cards []completion.Candidate
	for _, id := range ids {
		if !hasFoldPrefix(id, value) {
			continue
		}
		title, err := c.title(b, id)
		if err != nil {
			return nil, err
		}
		cards = append(cards, completion.Candidate{Word: id + suffix, Description: title})
		if len(cards) == completeLimit {
			break
		}
	}
	return cards, nil
}

// firstLive is the first of a number's identifiers whose directory stands in
// the live half, in the registry's file order, and empty when none does.
func (c *completionCall) firstLive(b *bench.Bench, ids []string) (string, error) {
	for _, id := range ids {
		if err := c.gate(); err != nil {
			return "", err
		}
		if bench.Exists(filepath.Join(b.CardsRoot(), id)) {
			return id, nil
		}
	}
	return "", nil
}

// title reads a card's title from its header, for a shell that shows one,
// and answers the empty string for bash without opening anything.
func (c *completionCall) title(b *bench.Bench, id string) (string, error) {
	if !c.describes() {
		return "", nil
	}
	if err := c.gate(); err != nil {
		return "", err
	}
	anchor := filepath.Join(b.CardsRoot(), id, bench.CardAnchor)
	fm, err := bench.ReadCardHeader(anchor)
	if err != nil {
		return "", nil
	}
	return fm.Value("title"), nil
}

// columns offers the columns in flow order, each with its title.
func (c *completionCall) columns() []completion.Candidate {
	library := c.bench()
	if library == nil {
		return nil
	}
	columns := make([]completion.Candidate, 0, len(library.Bench.Columns))
	for _, column := range library.Bench.Columns {
		columns = append(columns, completion.Candidate{Word: column.Ref(), Description: column.Title})
	}
	return columns
}

// references offers the cards, then the columns, then the workstreams.
func (c *completionCall) references(value string) ([]completion.Candidate, error) {
	cards, err := c.cards(value, "")
	if err != nil {
		return nil, err
	}
	references := append(cards, c.columns()...)
	return append(references, c.workstreams()...), nil
}

// workstreams offers the live workstreams by reference, each with its title.
func (c *completionCall) workstreams() []completion.Candidate {
	library := c.bench()
	if library == nil {
		return nil
	}
	live, err := library.Bench.Workstreams()
	if err != nil {
		return nil
	}
	workstreams := make([]completion.Candidate, 0, len(live))
	for _, workstream := range live {
		workstreams = append(workstreams, completion.Candidate{Word: workstream.Ref(), Description: workstream.Title})
	}
	sort.Slice(workstreams, func(i, j int) bool { return workstreams[i].Word < workstreams[j].Word })
	return workstreams
}

// levels offers one axis's declared levels in declaration order, each with
// its hint.
func (c *completionCall) levels(axis string) []completion.Candidate {
	library := c.bench()
	if library == nil {
		return nil
	}
	declared := library.Bench.Levels(axis)
	levels := make([]completion.Candidate, 0, len(declared))
	for _, level := range declared {
		levels = append(levels, completion.Candidate{Word: level.Name, Description: level.Hint})
	}
	return levels
}

// recipes offers each setup recipe a name resolves to and that reads, by
// name, with its title.
func (c *completionCall) recipes() []completion.Candidate {
	root, _, _, err := c.session().discoverRoot()
	if err != nil {
		root = ""
	}
	var recipes []completion.Candidate
	for _, row := range setup.List(setup.Container(root), bench.UserBase(c.home)) {
		if !row.Used || row.Malformed != "" {
			continue
		}
		recipes = append(recipes, completion.Candidate{Word: row.Name, Description: row.Title})
	}
	sort.Slice(recipes, func(i, j int) bool { return recipes[i].Word < recipes[j].Word })
	return recipes
}

// moveDestinations offers the columns a move of the card in the first slot
// would be accepted into, for the actor the line resolves and the override
// it carries.
func (c *completionCall) moveDestinations() ([]completion.Candidate, error) {
	library := c.bench()
	if library == nil {
		return nil, nil
	}
	agent := bench.ResolveAgent()
	actor, err := bench.ResolveActor(c.walk.values["actor"], agent.Harness, c.cfg)
	if err != nil {
		actor = ""
	}
	req := &verb.Request{
		Verb:     verb.Move,
		Card:     at(c.walk.positionals, 0),
		Actor:    actor,
		Harness:  agent.Harness,
		Provider: agent.Provider,
		Model:    agent.Model,
		Server:   agent.Server,
		Override: c.walk.given["override"],
	}
	rows, err := library.MoveDestinations(req)
	if errors.Is(err, errCompletionExpired) {
		return nil, err
	}
	destinations := make([]completion.Candidate, 0, len(rows))
	for _, row := range rows {
		destinations = append(destinations, completion.Candidate{Word: row.Ref, Description: row.Title})
	}
	return destinations, nil
}

// setValues offers what dinah set takes for the field in the second slot,
// when the reference in the first resolves to a card: the values a query
// offers for a field of the same name, and the declared tiers for tier.
func (c *completionCall) setValues() ([]completion.Candidate, error) {
	library := c.bench()
	if library == nil {
		return nil, nil
	}
	if _, err := library.Bench.ResolveCard(at(c.walk.positionals, 0)); err != nil {
		return nil, nil
	}
	field := at(c.walk.positionals, 1)
	if field == bench.TierField {
		return plainWords(bench.LevelNames(library.Bench.Levels(bench.TierField))), nil
	}
	values, err := library.QueryFieldValues(field)
	if err != nil {
		return nil, nil
	}
	return plainWords(values), nil
}

// queryTerm completes one term of the query language. A term with no
// operator yet completes a field name followed by a colon, and one with an
// operator completes the whole term with a value of that field after the
// values already typed.
func (c *completionCall) queryTerm(term string) (string, []completion.Candidate, error) {
	library := c.bench()
	field, operator, hasOperator := verb.SplitQueryTerm(term)
	if !hasOperator {
		var names []completion.Candidate
		for _, name := range verb.QueryFields {
			if name == verb.FieldAt {
				continue
			}
			names = append(names, completion.Candidate{Word: name + ":"})
		}
		if library != nil {
			for _, key := range library.Bench.DeclaredFieldKeysOn(bench.KindCard) {
				names = append(names, completion.Candidate{Word: key + ":"})
			}
		}
		return completion.ModeNospace, names, nil
	}
	if library == nil {
		return completion.ModeWords, nil, nil
	}
	values, err := library.QueryFieldValues(field)
	if err != nil {
		return completion.ModeWords, nil, nil
	}
	typed := term[len(field)+len(operator):]
	lead := field + operator
	if cut := strings.LastIndex(typed, ","); cut >= 0 {
		lead += typed[:cut+1]
	}
	terms := make([]completion.Candidate, 0, len(values))
	for _, value := range values {
		terms = append(terms, completion.Candidate{Word: lead + value})
	}
	return completion.ModeWords, terms, nil
}
