package verb

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestEveryCommandDerivesOrIsExempted asserts that every command the library
// defines either carries the field bindings DeriveCommand reads its arguments
// through, or names in derivationExemptions why no Request is ever built for
// it. The guard is what lets DeriveCommand assume a parameter's Field names a
// real Request field rather than checking it at every call, and it is what
// stops a command added later from acquiring a silently empty command log
// entry.
//
// The four rules run in order. A command declaring no parameters passes
// outright: it has nothing that could carry a stray binding or a missing one,
// and demanding an exemption for it would tell a caller that no log entry is
// possible for whoami when a correct empty one always was.
func TestEveryCommandDerivesOrIsExempted(t *testing.T) {
	names := append([]string(nil), Commands()...)
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("the command table is empty, so this check would pass against anything")
	}
	for _, name := range names {
		declared := Params(name)
		if len(declared) == 0 {
			continue
		}
		var bound, unbound int
		for _, p := range declared {
			if p.Field != "" {
				bound++
				continue
			}
			unbound++
		}
		_, exempted := derivationExemptions[name]
		switch {
		case bound > 0 && unbound > 0:
			t.Errorf("%s: split coverage, %d of its %d parameters name a Request field and the rest name none, so DeriveCommand would render part of the call and drop the rest",
				name, bound, len(declared))
		case bound == 0 && !exempted:
			t.Errorf("%s: declares parameters with no Field and names no exemption, so DeriveCommand would derive a command line carrying none of the values the caller gave",
				name)
		case bound > 0 && exempted:
			t.Errorf("%s: stale exemption, it names a Request field on %d of its parameters and is still listed in derivationExemptions",
				name, bound)
		}
	}
}

// TestEveryExemptionNamesACommandAndAReason asserts the exemption map is a
// list of live commands rather than a list of names, so an exemption outliving
// the command it excused fails here instead of quietly widening what
// DeriveCommand refuses.
func TestEveryExemptionNamesACommandAndAReason(t *testing.T) {
	defined := map[string]bool{}
	for _, name := range Commands() {
		defined[name] = true
	}
	for name, reason := range DerivationExemptions() {
		if !defined[name] {
			t.Errorf("derivationExemptions names %q, which is not a command this library defines", name)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s: exempted with no reason written down", name)
		}
	}
}

// TestAZeroParameterCommandDerivesWithoutAnExemption asserts a command with
// nothing to type derives an empty command rather than being excused from
// derivation. whoami takes a *Request like any other verb, so an exemption
// entry for it would tell a caller that no log entry is possible when a
// correct empty one always was, and the coverage guard above must not be
// satisfied by adding one.
func TestAZeroParameterCommandDerivesWithoutAnExemption(t *testing.T) {
	for _, name := range []string{"whoami", "columns"} {
		if len(Params(name)) != 0 {
			t.Fatalf("%s now declares parameters, so this check no longer reads the case it was written for", name)
		}
		if reason, exempted := derivationExemptions[name]; exempted {
			t.Errorf("%s declares no parameters and is exempted anyway (%q), which reports a derivable call as underivable", name, reason)
		}
		cmd, ok, reason := DeriveCommand(&Request{Verb: name})
		if !ok {
			t.Errorf("%s refused derivation: %s", name, reason)
			continue
		}
		if cmd.Verb != name || len(cmd.Args) != 0 {
			t.Errorf("%s derived %#v, want the bare command", name, cmd)
		}
		if got := cmd.Line(); got != name {
			t.Errorf("%s rendered as %q", name, got)
		}
	}
}

// TestDerivationExemptionsIsACopy asserts a caller cannot reach the table
// itself, since a surface deciding whether a log entry is possible has no
// business being able to change what is exempt.
func TestDerivationExemptionsIsACopy(t *testing.T) {
	taken := DerivationExemptions()
	taken["config"] = "rewritten by a caller"
	delete(taken, "mcp")
	if got := DerivationExemptions()["config"]; got == "rewritten by a caller" {
		t.Error("a caller's write to the returned map reached derivationExemptions")
	}
	if _, still := DerivationExemptions()["mcp"]; !still {
		t.Error("a caller's delete on the returned map reached derivationExemptions")
	}
}

// TestCommandLineQuotesWhatAReaderWouldHaveToType asserts Line quotes an
// argument carrying a space, an empty argument, and an argument carrying a
// double quote, and leaves a plain word alone.
func TestCommandLineQuotesWhatAReaderWouldHaveToType(t *testing.T) {
	cmd := Command{Verb: "add", Args: []string{"a title with spaces"}}
	// The word count is the assertion rather than the presence of a quote
	// character, because an unquoted title reads as several arguments and
	// still carries every character the quoted one does.
	if got := cmd.Line(); got != `add "a title with spaces"` {
		t.Errorf("a spaced argument rendered as %q", got)
	}
	if got := len(shellWords(cmd.Line())); got != 2 {
		t.Errorf("a spaced argument rendered as %d words, so the line reads as a different call", got)
	}

	empty := Command{Verb: "add", Args: []string{""}}
	if got := empty.Line(); got != `add ""` {
		t.Errorf("an empty argument rendered as %q", got)
	}

	quoted := Command{Verb: "comment", Args: []string{`he said "no"`}}
	if got := quoted.Line(); got != `comment "he said \"no\""` {
		t.Errorf("an argument carrying a double quote rendered as %q", got)
	}

	plain := Command{Verb: "show", Args: []string{"card-7", "--column", "spec"}}
	if got := plain.Line(); got != "show card-7 --column spec" {
		t.Errorf("plain arguments were quoted anyway: %q", got)
	}
}

// TestLineQuotesOnlyTheCharactersQuotingActuallyHandles holds shellSpecial to
// what quoteArgument can honestly do with it. Every character the set declares
// has to survive a round trip through the quoting and back out again, and the
// characters whose argument boundary a double-quoted token does not restore
// have to stay outside the set, since declaring one of them is how Line comes
// to present a boundary it does not hold.
//
// The exclusion runs on the boundary rather than on inertness, and the two
// answer differently. Quoting makes neither a $ nor a % mean itself, and both
// are members, because quoting is what makes the argument count right for
// them. The rows below name the characters for which even that is untrue.
func TestLineQuotesOnlyTheCharactersQuotingActuallyHandles(t *testing.T) {
	excluded := []struct {
		char rune
		why  string
	}{
		{'`', "PowerShell escapes the closing delimiter a backtick stands against, and a POSIX shell opens a command substitution inside double quotes whose unmatched form runs past that delimiter"},
		{'!', "an interactive bash expands one inside double quotes as readily as outside, and abandons the line where the designator names no event"},
		{'^', "cmd.exe reads it as its escape character and Microsoft documents no rule for one inside a quoted token, so no membership decision here can rest on documented behaviour"},
	}
	for _, out := range excluded {
		if strings.ContainsRune(shellSpecial, out.char) {
			t.Errorf("shellSpecial declares %q special and quoting does not restore the argument boundary for it: %s", out.char, out.why)
		}
	}

	for _, special := range shellSpecial {
		token := "a" + string(special) + "b"
		rendered := quoteArgument(token)
		if rendered == token {
			t.Errorf("a token carrying %q rendered unquoted as %q", special, rendered)
			continue
		}
		if got := splitWindowsCRT(rendered); len(got) != 1 || got[0] != token {
			t.Errorf("a token carrying %q rendered as %q, which reads back as %q rather than %q", special, rendered, got, token)
		}
	}

	// The backslash earns its place in the set from the other side, so it gets
	// its own assertion: a bare token ending in one escapes the space after it
	// in a POSIX shell, and quoting is what stops that.
	if got := quoteArgument(`C:\temp\`); got != `"C:\temp\\"` {
		t.Errorf(`a token ending in a backslash rendered as %s, which does not terminate where it appears to`, got)
	}
	// The $ and the % earn their places from the same side as the backslash and
	// against an earlier reading of this test, which pinned the bare rendering
	// of $HOME on the ground that quoting cannot make a $ inert. Quoting does
	// not make it inert and does make the count right, so these two assertions
	// hold the rendering to the boundary the exclusion rule now asks about. A
	// bare $HOME is no arguments in a POSIX shell where the variable is unset
	// and two where its value carries a space; a bare %PATH% is as many as the
	// value has spaces in cmd.exe.
	if got := quoteArgument("$HOME"); got != `"$HOME"` {
		t.Errorf("a token carrying $ rendered as %s, which a POSIX shell field-splits into a number of arguments its environment decides", got)
	}
	if got := quoteArgument("%PATH%"); got != `"%PATH%"` {
		t.Errorf("a token carrying %% rendered as %s, which cmd.exe substitutes into the line before it parses one", got)
	}
}

// TestLineHoldsItsArgumentBoundaryUnderCombinedTriggers is the guard for the
// defect a one-character-at-a-time test cannot see. Quoting is triggered by
// whitespace as well as by shellSpecial, and the two interact: a token quoted
// for its spaces carries whatever else it holds inside the quoting, and a
// trailing backslash then lands against the closing delimiter and escapes it.
// `dir C:\temp\` followed by `next` printed as one argument for exactly that
// reason, and no single-character row reaches it, because the defect needs a
// space and a backslash in the same token.
//
// So this walks combinations rather than characters. Twenty-one fragments,
// each carrying one trigger or one hazard, are concatenated pairwise into 441
// tokens, and each token is rendered beside a following argument and read back
// by three readers: the rule CommandLineToArgvW documents, a POSIX shell's
// double-quote rules, and PowerShell's. The invariant every reader has to
// agree on is how many arguments the line describes.
//
// The lone backtick fragment is here because the balanced `id` was the only
// backtick this table used to carry, so every combination held an even count
// and none ever put one against a closing delimiter. A backtick is the shape
// no quoting makes inert, and the sweep therefore holds only the Windows C
// runtime to it; what a POSIX shell and PowerShell do with it is pinned
// literally in backticks below, because the honest claim is that the boundary
// breaks rather than that it survives.
//
// The lone $ and % fragments are here for the reverse reason. The table used
// to reach those two characters only through $HOME and %PATH%, and no reader
// in this file models an expansion, so a bare rendering of either read back as
// three tidy arguments and the sweep saw nothing. Neither is rendered bare any
// more, and the named rows below pin that, because the sweep cannot tell a
// quoted token from a bare one whose expansion it does not perform.
func TestLineHoldsItsArgumentBoundaryUnderCombinedTriggers(t *testing.T) {
	fragments := []string{
		"", "a", "a b", "  ", "\t",
		`\`, `\\`, `C:\temp`, `C:\temp\`,
		`"`, `a"b`,
		"$HOME", "$", "`id`", "`", "%PATH%", "%",
		"|", "*", "#", "'",
	}

	// The named rows come first, because a combination sweep says a property
	// held and a literal expectation says what the rendering actually is. The
	// first two are the review findings this test was written for.
	named := []struct {
		token string
		line  string
	}{
		{`dir C:\temp\`, `add "dir C:\temp\\" next`},
		{"raise the $HOME limit", `add "raise the $HOME limit" next`},
		{`C:\Program Files\dinah`, `add "C:\Program Files\dinah" next`},
		{`C:\temp\`, `add "C:\temp\\" next`},
		{`a"b`, `add "a\"b" next`},
		{`a\"b`, `add "a\\\"b" next`},
		{"", `add "" next`},
		// The $ and the % are quoted although quoting leaves both meaningful,
		// because the boundary is the only thing Line promises and quoting is
		// what makes the count right. A bare $HOME is field-split by a POSIX
		// shell into however many words the environment decides, and a bare
		// %PATH% is substituted into the line by cmd.exe before it parses one.
		// These four rows are what an edit reverting either character has to
		// come through.
		{"$HOME", `add "$HOME" next`},
		{"$", `add "$" next`},
		{"%PATH%", `add "%PATH%" next`},
		{"%", `add "%" next`},
		// The ! and the ^ are the reverse case, and they are rendered bare on
		// purpose. Quoting does not stop an interactive bash expanding a ! or
		// abandoning the line over one, and no documented rule says what
		// cmd.exe does with a ^ inside a quoted token, so neither is a member
		// and Line's comment names both as shapes it makes no claim for. A row
		// each so that adding one to the set has to be argued here.
		{"a!b", "add a!b next"},
		{"ship it! by friday", `add "ship it! by friday" next`},
		{"a^b", "add a^b next"},
		// A line break is whitespace, so it quotes the token and is then
		// emitted as it stands. The rendering spans two physical lines, which
		// Line's comment now says outright rather than promising one line and
		// leaving a --note value to break the promise. This row pins the
		// decision: escaping the break would put characters in the output that
		// no shell reads back as a newline.
		{"two\nlines", "add \"two\nlines\" next"},
	}
	for _, row := range named {
		got := Command{Verb: "add", Args: []string{row.token, "next"}}.Line()
		if got != row.line {
			t.Errorf("%q rendered the line %s, want %s", row.token, got, row.line)
		}
	}

	// The backtick rows pin the boundary breaking rather than holding, which is
	// the only honest thing to pin: quoting cannot make a backtick inert, so
	// there is no rendering for these rows to be held to. They exist so that a
	// later edit claiming the backtick is handled, by adding it to shellSpecial
	// or by escaping it, has to come through here and say what changed. Each
	// row carries the rendering, what PowerShell reads out of it, and what a
	// POSIX shell reads, where a nil means the line does not parse at all.
	backticks := []struct {
		token string
		line  string
		pwsh  []string
		posix []string
		why   string
	}{
		{
			token: "a b`",
			line:  "add \"a b`\" next",
			pwsh:  []string{"add", "a b\" next"},
			posix: nil,
			why:   "a backtick against the closing delimiter escapes it in PowerShell, and opens an unmatched substitution that runs past it in a POSIX shell",
		},
		{
			token: "a`",
			line:  "add a` next",
			pwsh:  []string{"add", "a next"},
			posix: nil,
			why:   "a bare token ending in a backtick escapes the separating space in PowerShell, and opens an unmatched substitution in a POSIX shell",
		},
		{
			token: "`id`",
			line:  "add `id` next",
			pwsh:  []string{"add", "id next"},
			posix: []string{"add", "", "next"},
			why:   "a balanced pair is no safer: PowerShell escapes the character after each backtick, and a POSIX shell runs the substitution and puts its output in the word",
		},
	}
	for _, row := range backticks {
		got := Command{Verb: "add", Args: []string{row.token, "next"}}.Line()
		if got != row.line {
			t.Errorf("%q rendered the line %s, want %s", row.token, got, row.line)
			continue
		}
		// The Windows C runtime reads a backtick as an ordinary character, so
		// it is the one reader whose boundary survives all three rows.
		if want := []string{"add", row.token, "next"}; !reflect.DeepEqual(splitWindowsCRT(got), want) {
			t.Errorf("%q rendered the line %s, which the Windows C runtime reads as %q rather than %q", row.token, got, splitWindowsCRT(got), want)
		}
		if pwsh := splitPowerShell(got); !reflect.DeepEqual(pwsh, row.pwsh) {
			t.Errorf("%q rendered the line %s, which PowerShell reads as %q rather than %q; %s", row.token, got, pwsh, row.pwsh, row.why)
		}
		if posix := splitPOSIX(got); !reflect.DeepEqual(posix, row.posix) {
			t.Errorf("%q rendered the line %s, which a POSIX shell reads as %q rather than %q; %s", row.token, got, posix, row.posix, row.why)
		}
	}

	for _, head := range fragments {
		for _, tail := range fragments {
			token := head + tail
			line := Command{Verb: "add", Args: []string{token, "next"}}.Line()
			want := []string{"add", token, "next"}

			// The Windows C runtime reads back both the boundary and the
			// value, because its rule is the one quoteArgument writes to.
			if got := splitWindowsCRT(line); !reflect.DeepEqual(got, want) {
				t.Errorf("%q rendered the line %s, which the Windows C runtime reads as %q rather than %q", token, line, got, want)
				continue
			}

			// A backtick leaves the other two readers nothing to prove. No
			// quoting makes one inert, so what a POSIX shell and PowerShell do
			// with the rendering depends on what stands beside the backtick
			// rather than on anything this function could have done, and the
			// backticks rows above pin those outcomes literally. Asserting a
			// boundary here would assert something false for some of these
			// tokens and something accidental for the rest.
			if strings.Contains(token, "`") {
				continue
			}

			// A POSIX shell reads the boundary. It does not always read the
			// value back, because it drops a backslash standing before another
			// backslash, a $ or a backtick, and because it expands a $ inside
			// double quotes as well as outside. That is the residue
			// shellSpecial records and which no rendering removes, and
			// posixReadsBackExactly names both halves of it.
			posix := splitPOSIX(line)
			if len(posix) != 3 || posix[0] != "add" || posix[2] != "next" {
				t.Errorf("%q rendered the line %s, which a POSIX shell reads as %d arguments, %q", token, line, len(posix), posix)
			} else if posixReadsBackExactly(token) && posix[1] != token {
				t.Errorf("%q rendered the line %s, which a POSIX shell reads the first argument of as %q", token, line, posix[1])
			}

			// PowerShell reads the boundary for every token that carries no
			// double quote of its own. It does not read the value back, since
			// it never undoes the \" escape.
			if strings.Contains(token, `"`) {
				continue
			}
			pwsh := splitPowerShell(line)
			if len(pwsh) != 3 || pwsh[0] != "add" || pwsh[2] != "next" {
				t.Errorf("%q rendered the line %s, which PowerShell reads as %d arguments, %q", token, line, len(pwsh), pwsh)
			}
		}
	}
}

// splitWindowsCRT reads a line the way the Windows C runtime's command-line
// parser documents: a run of backslashes is literal unless a double quote
// follows it, 2n backslashes and a quote yield n backslashes and a delimiter,
// and 2n+1 yield n backslashes and a literal quote. It is the rule
// quoteArgument's doubling is written to, so it reads back both the boundary
// and the value.
func splitWindowsCRT(line string) []string {
	var args []string
	var cur strings.Builder
	started, quoted := false, false
	for i := 0; i < len(line); {
		switch {
		case line[i] == '\\':
			run := 0
			for i+run < len(line) && line[i+run] == '\\' {
				run++
			}
			if i+run < len(line) && line[i+run] == '"' {
				cur.WriteString(strings.Repeat(`\`, run/2))
				if run%2 == 1 {
					cur.WriteByte('"')
				} else {
					quoted = !quoted
				}
				i += run + 1
			} else {
				cur.WriteString(strings.Repeat(`\`, run))
				i += run
			}
			started = true
		case line[i] == '"':
			quoted = !quoted
			started = true
			i++
		case (line[i] == ' ' || line[i] == '\t') && !quoted:
			if started {
				args = append(args, cur.String())
				cur.Reset()
				started = false
			}
			i++
		default:
			cur.WriteByte(line[i])
			started = true
			i++
		}
	}
	if started {
		args = append(args, cur.String())
	}
	return args
}

// splitPOSIX reads a line the way a POSIX shell splits words. A double quote
// opens and closes a quoted run. Outside one a backslash escapes whatever
// follows it, including the space that would otherwise end the word, which is
// why a bare token ending in a backslash swallows the argument after it and
// why the backslash is a quoting trigger. Inside one a backslash escapes only
// a double quote, another backslash, a $ or a backtick. Everything else,
// including a backslash before an ordinary character, is literal. Modelling
// that difference from splitWindowsCRT is the point of having both, since the
// disagreement between them is what a naive single reader would hide.
//
// An unescaped backtick opens a command substitution, inside double quotes as
// well as outside them, and it runs to the next backtick. The substituted text
// is the output of a command this reader cannot run, so it contributes nothing
// to the word, which is why a value carrying a balanced pair does not come
// back. An unmatched backtick runs past the closing quote and past the end of
// the line, and the word never terminates, so the line does not parse and this
// reader returns nil rather than a split it cannot justify. That case is why
// the doc comment on Line names the backtick beside the double quote: the
// double quote costs the boundary in PowerShell alone, and the backtick costs
// it here too.
//
// It expands a parameter against posixEnv, and it does so because a reader
// that skipped expansion answered three tidy arguments for a bare $HOME and
// let the sweep pass over the defect the fourth review found. POSIX 2.6.5
// field-splits the result of an expansion that happened outside double quotes
// and never re-parses what an expansion produced, so an unquoted $HOME becomes
// as many words as its value holds, or none where the value is empty, while a
// quoted one is a single word whatever the value is. Modelling that is what
// lets the sweep see a bare $-bearing token rather than take one on trust.
//
// The model covers $name and nothing else. A $ that no name follows is
// literal, which is what a shell does with one, and the special parameters
// ($$, $?, $@ and the rest) and the ${...} form are read as literal text as
// well, which is not what a shell does with those. Every token this rendering
// produces quotes its $ rather than leaving one bare, so no unmodelled form
// reaches a field-splitting context here; a later edit that stopped quoting
// the $ would need those forms modelled before this reader could be trusted
// about them.
//
// Expansion also decides the value rather than only the boundary, and the two
// answers differ: a quoted "$HOME" is one argument, correctly, and that one
// argument carries the variable's value rather than the token, which is why
// posixReadsBackExactly refuses a token carrying a $.
func splitPOSIX(line string) []string {
	var args []string
	var cur strings.Builder
	started, quoted := false, false
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '`':
			end := i + 1
			for end < len(line) && line[end] != '`' {
				if line[end] == '\\' {
					end++
				}
				end++
			}
			if end >= len(line) {
				return nil
			}
			i = end
			started = true
		case quoted && line[i] == '\\' && i+1 < len(line) && strings.IndexByte("\"\\$`", line[i+1]) >= 0:
			cur.WriteByte(line[i+1])
			i++
			started = true
		case !quoted && line[i] == '\\' && i+1 < len(line):
			cur.WriteByte(line[i+1])
			i++
			started = true
		case line[i] == '$' && posixNameAt(line, i+1) != "":
			name := posixNameAt(line, i+1)
			i += len(name)
			value := posixEnv[name]
			if quoted {
				cur.WriteString(value)
				started = true
				break
			}
			// An unquoted expansion is field-split, so the value contributes
			// as many words as it holds fields, and a value holding none
			// contributes nothing at all: the word disappears where the
			// expansion was everything it had. That is the whole of the
			// difference between a bare $HOME and a quoted one, and it is why
			// the $ is a quoting trigger.
			for n, field := range strings.Fields(value) {
				if n > 0 {
					args = append(args, cur.String())
					cur.Reset()
				}
				cur.WriteString(field)
				started = true
			}
		case line[i] == '"':
			quoted = !quoted
			started = true
		case (line[i] == ' ' || line[i] == '\t') && !quoted:
			if started {
				args = append(args, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteByte(line[i])
			started = true
		}
	}
	if started {
		args = append(args, cur.String())
	}
	return args
}

// splitPowerShell reads a line the way PowerShell parses one: a backtick
// escapes the character after it, a double quote opens and closes a quoted
// run, and a backslash is an ordinary character everywhere. It reads no \"
// escape, which is one of the two reasons this rendering cannot hold a
// boundary in every shell. The backtick is the other, and it is worse: it
// escapes whatever follows it here, including a closing delimiter and a
// separating space, and a POSIX shell reads it as a command substitution
// rather than as text, so no quoting neutralises it in either reader.
func splitPowerShell(line string) []string {
	var args []string
	var cur strings.Builder
	started, quoted := false, false
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '`' && i+1 < len(line):
			cur.WriteByte(line[i+1])
			i++
			started = true
		case line[i] == '"':
			quoted = !quoted
			started = true
		case (line[i] == ' ' || line[i] == '\t') && !quoted:
			if started {
				args = append(args, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteByte(line[i])
			started = true
		}
	}
	if started {
		args = append(args, cur.String())
	}
	return args
}

// posixEnv is the environment splitPOSIX expands a parameter against. HOME
// carries a space on purpose, because a value of one word makes a bare and a
// quoted expansion agree and hides the very difference the sweep is here to
// catch. Every name absent from the map is unset, which a shell expands to
// nothing, and an unquoted expansion to nothing removes the word.
var posixEnv = map[string]string{
	"HOME": "/home/a b",
	"PATH": "/usr/bin:/opt/a b/bin",
}

// posixNameAt returns the parameter name beginning at i, or the empty string
// where no name begins there. The grammar is POSIX 3.235's name production, an
// underscore or a letter followed by underscores, letters and digits, and a $
// standing before anything else is literal. I searched for prior art with
// grep -rn '^func ' cmd internal before writing it and found no name scanner
// in the tree; the three shell readers beside it are this file's own.
func posixNameAt(line string, i int) string {
	j := i
	for j < len(line) {
		c := line[j]
		alpha := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		digit := j > i && c >= '0' && c <= '9'
		if !alpha && !digit {
			break
		}
		j++
	}
	return line[i:j]
}

// posixReadsBackExactly says whether a POSIX shell recovers token unchanged
// from quoteArgument's rendering. It does for almost everything, because a
// backslash standing before an ordinary character is literal inside POSIX
// double quotes and a run standing before a double quote is doubled by the
// rendering, which POSIX pairs back. It does not when a run of two or more
// backslashes stands before an ordinary character, or when a single backslash
// stands before a $ or a backtick, because POSIX pairs or consumes those and
// the rendering leaves them as they are. That is the residue shellSpecial
// records rather than a defect this test should hide.
//
// It also does not for any token carrying a $, and that answer is not
// something splitPOSIX can see. A POSIX shell expands a parameter inside
// double quotes, so the argument it builds carries the variable's value rather
// than the token, while splitPOSIX performs no expansion and hands the token
// straight back. Reporting true here would let the sweep assert a value round
// trip that a real shell does not make, which is the shape of every defect
// this card has produced, so the exclusion is stated rather than inherited
// from the reader's silence.
func posixReadsBackExactly(token string) bool {
	if strings.ContainsRune(token, '$') {
		return false
	}
	for i := 0; i < len(token); {
		if token[i] != '\\' {
			i++
			continue
		}
		run := 0
		for i+run < len(token) && token[i+run] == '\\' {
			run++
		}
		next := byte(0)
		if i+run < len(token) {
			next = token[i+run]
		}
		if next != '"' {
			if run > 1 {
				return false
			}
			if next == '$' || next == '`' {
				return false
			}
		}
		i += run
		if next != 0 {
			i++
		}
	}
	return true
}

// shellWords counts the words a line reads as, taking a double quote as the
// start and end of one word the way a shell does. strings.Fields cannot stand
// in for it: it splits on whitespace whether or not the whitespace sits inside
// quotes, so it reports the same count for a quoted title and an unquoted one,
// which is the difference this check exists to see.
func shellWords(line string) []string {
	var words []string
	var word strings.Builder
	inWord, quoted, escaped := false, false, false
	for _, r := range line {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case r == '\\' && quoted:
			escaped = true
		case r == '"':
			quoted = !quoted
			inWord = true
		case r == ' ' && !quoted:
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words
}

// TestDeriveCommandTypesARequiredArgumentTheRequestArrivedWithout asserts a
// required argument the request carries no value for is still typed, as an
// empty word, rather than dropped. Dropping it would derive a line that runs a
// different command from the one the caller made.
func TestDeriveCommandTypesARequiredArgumentTheRequestArrivedWithout(t *testing.T) {
	cmd, ok, reason := DeriveCommand(&Request{Verb: "add"})
	if !ok {
		t.Fatalf("add refused derivation: %s", reason)
	}
	// The length is the assertion rather than the rendered line, because
	// `add` and `add ""` read almost the same in a casual diff.
	if len(cmd.Args) != 1 {
		t.Fatalf("an empty required title derived %d arguments, want 1: %#v", len(cmd.Args), cmd.Args)
	}
	if cmd.Args[0] != "" {
		t.Errorf("the empty required title derived %q", cmd.Args[0])
	}
	if got := cmd.Line(); got != `add ""` {
		t.Errorf("the line rendered as %q", got)
	}
}

// TestDeriveCommandRefusesAnUnknownOrExemptedCommand asserts DeriveCommand
// says no rather than returning an empty command for a verb it cannot spell,
// and that an exemption's own written reason travels back to the caller.
func TestDeriveCommandRefusesAnUnknownOrExemptedCommand(t *testing.T) {
	cmd, ok, reason := DeriveCommand(&Request{Verb: "frobnicate"})
	if ok {
		t.Errorf("an unknown verb derived %#v", cmd)
	}
	if !strings.Contains(reason, "frobnicate") {
		t.Errorf("the refusal did not name the verb: %q", reason)
	}

	cmd, ok, reason = DeriveCommand(&Request{Verb: "config"})
	if ok {
		t.Errorf("an exempted command derived %#v", cmd)
	}
	if reason != derivationExemptions["config"] {
		t.Errorf("the refusal carried %q rather than the exemption's own text", reason)
	}

	if _, ok, reason = DeriveCommand(nil); ok || reason == "" {
		t.Errorf("a nil request derived a command (ok=%v, reason=%q)", ok, reason)
	}
}

// TestDeriveCommandDerivesTheShapesTheTableDeclares asserts the four shapes a
// parameter can take reach the argument list the way a person types them: a
// positional, a valued flag, a marker only when it is set, and a repeatable
// flag once per element in the order the field holds them.
func TestDeriveCommandDerivesTheShapesTheTableDeclares(t *testing.T) {
	claim := &Request{Verb: Claim, Card: "card-7", Expires: 8 * time.Hour}
	cmd, ok, reason := DeriveCommand(claim)
	if !ok {
		t.Fatalf("claim refused derivation: %s", reason)
	}
	want := []string{"card-7", "--expires", "8h0m0s"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("claim derived %#v, want %#v", cmd.Args, want)
	}

	unleased := &Request{Verb: Claim, Card: "card-7"}
	cmd, ok, reason = DeriveCommand(unleased)
	if !ok {
		t.Fatalf("a claim naming no lease refused derivation: %s", reason)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"card-7"}) {
		t.Errorf("a claim naming no lease derived %#v, so the line carries a lease nobody typed", cmd.Args)
	}

	reshape := &Request{
		Verb:    "reshape",
		From:    "other/workbench",
		Map:     []string{"retired-a=destination-a", "retired-b=destination-b"},
		Confirm: true,
	}
	cmd, ok, reason = DeriveCommand(reshape)
	if !ok {
		t.Fatalf("reshape refused derivation: %s", reason)
	}
	want = []string{
		"--from", "other/workbench",
		"--map", "retired-a=destination-a",
		"--map", "retired-b=destination-b",
		"--yes",
	}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("reshape derived %#v, want %#v", cmd.Args, want)
	}

	unconfirmed := &Request{Verb: "reshape", From: "other/workbench"}
	cmd, ok, reason = DeriveCommand(unconfirmed)
	if !ok {
		t.Fatalf("an unconfirmed reshape refused derivation: %s", reason)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"--from", "other/workbench"}) {
		t.Errorf("an unset marker reached the line: %#v", cmd.Args)
	}
}

// TestDeriveCommandRendersAFlagUnderItsDisplayedName asserts a flag that
// declares a Display is typed under that name rather than under its own member
// name, which is the difference between a line that reruns and a line the
// parser turns away.
//
// No shipped flag declares one today: every Display in the table sits on a
// positional, where it names the placeholder the syntax line prints and never
// reaches an argument DeriveCommand renders. Walking the table for a case to
// check would therefore assert nothing at all, so the case is wired here for
// the length of the test, which is what keeps the rule from being discovered
// the day a flag first carries a Display.
func TestDeriveCommandRendersAFlagUnderItsDisplayedName(t *testing.T) {
	for _, name := range Commands() {
		for _, p := range Params(name) {
			if p.Flag && p.Display != "" {
				t.Fatalf("%s's %q flag now declares a Display, so this fixture is no longer the only case: the walk it replaced should come back, and reparse in cmd/dinah's TestDeriveCommandRoundTrips has to look a flag up by its displayed spelling rather than by p.Name", name, p.Name)
			}
		}
	}

	const fixture = "display-fixture"
	params[fixture] = []Param{{Name: "card", Display: "ref", Flag: true, Value: "ref", Field: "Card"}}
	defer delete(params, fixture)

	cmd, ok, reason := DeriveCommand(&Request{Verb: fixture, Card: "card-7"})
	if !ok {
		t.Fatalf("the fixture refused derivation: %s", reason)
	}
	want := []string{"--ref", "card-7"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("a flag declaring a Display derived %#v, want %#v", cmd.Args, want)
	}
}

// TestRenderParamValueRefusesATypeNobodyTaughtIt asserts an unrecognized kind
// is refused rather than rendered. reflect.Value.String returns a placeholder
// such as "<int Value>" for a kind with no case of its own, so falling through
// to it would compose a plausible-looking argument that reruns as something
// else entirely.
func TestRenderParamValueRefusesATypeNobodyTaughtIt(t *testing.T) {
	unknown := []reflect.Value{
		reflect.ValueOf(42),
		reflect.ValueOf(map[string]string{}),
		reflect.ValueOf([]int{1, 2}),
		reflect.ValueOf(true),
	}
	for _, value := range unknown {
		values, ok := renderParamValue(value)
		if ok {
			t.Errorf("%s was rendered as %#v rather than refused", value.Type(), values)
		}
		if values != nil {
			t.Errorf("%s returned %#v alongside its refusal", value.Type(), values)
		}
	}
}

// TestRenderParamValueReadsAnEmptyValueOfAKnownType asserts emptiness is not
// mistaken for an unknown type: a known type with nothing in it is renderable
// and contributes no token.
func TestRenderParamValueReadsAnEmptyValueOfAKnownType(t *testing.T) {
	empty := []reflect.Value{
		reflect.ValueOf(""),
		reflect.ValueOf(time.Duration(0)),
		reflect.ValueOf([]string(nil)),
	}
	for _, value := range empty {
		values, ok := renderParamValue(value)
		if !ok {
			t.Errorf("an empty %s was refused as an unknown type", value.Type())
		}
		if len(values) != 0 {
			t.Errorf("an empty %s contributed %#v", value.Type(), values)
		}
	}
}

// TestDeriveCommandRefusesAParameterItCannotRender asserts the refusal reaches
// the whole derivation rather than one argument. No shipped command declares a
// field of an unrenderable type, so the check wires one for the length of the
// test: a bool field declared without Marker, which is a real shape the table
// could take and one renderParamValue has no case for.
func TestDeriveCommandRefusesAParameterItCannotRender(t *testing.T) {
	const fixture = "derive-fixture"
	params[fixture] = []Param{
		{Name: "card", Required: true, Field: "Card"},
		{Name: "override", Flag: true, Field: "Override"},
	}
	defer delete(params, fixture)

	cmd, ok, reason := DeriveCommand(&Request{Verb: fixture, Card: "card-7", Override: true})
	if ok {
		t.Fatalf("an unrenderable field derived %#v", cmd)
	}
	if cmd.Args != nil {
		t.Errorf("the refusal carried a partly composed argument list: %#v", cmd.Args)
	}
	for _, want := range []string{fixture, "override", "Override", "bool"} {
		if !strings.Contains(reason, want) {
			t.Errorf("the refusal does not name %q, so a reader cannot tell what to fix: %q", want, reason)
		}
	}
}
