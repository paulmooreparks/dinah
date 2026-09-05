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
// four characters a double-quoted token does not render inert have to stay
// outside the set, since declaring one of them is how Line comes to present a
// token as handled when retyping it would run something else.
func TestLineQuotesOnlyTheCharactersQuotingActuallyHandles(t *testing.T) {
	excluded := []struct {
		char rune
		why  string
	}{
		{'$', "a POSIX shell expands it inside double quotes"},
		{'`', "a POSIX shell runs a command substitution inside double quotes"},
		{'%', "cmd.exe expands a variable inside double quotes"},
		{'\\', "cmd.exe and PowerShell read it literally inside double quotes, so escaping it doubles every separator of a Windows path"},
	}
	for _, out := range excluded {
		if strings.ContainsRune(shellSpecial, out.char) {
			t.Errorf("shellSpecial declares %q special and quoting cannot render it inert: %s", out.char, out.why)
		}
	}

	for _, special := range shellSpecial {
		token := "a" + string(special) + "b"
		rendered := quoteArgument(token)
		if rendered == token {
			t.Errorf("a token carrying %q rendered unquoted as %q", special, rendered)
			continue
		}
		if got := readDoubleQuoted(rendered); got != token {
			t.Errorf("a token carrying %q rendered as %q, which reads back as %q rather than %q", special, rendered, got, token)
		}
	}

	// The two excluded characters that reach a real argument get their own
	// assertion, because what the exclusion buys is visible in the rendering
	// rather than in the constant.
	if got := quoteArgument(`C:\Users\paul`); got != `C:\Users\paul` {
		t.Errorf("a Windows path rendered as %q, so a reader retyping it into cmd.exe or PowerShell gets a different path", got)
	}
	if got := quoteArgument("$HOME"); got != "$HOME" {
		t.Errorf("a token carrying $ rendered as %q, which quoting does not make inert and must not appear to", got)
	}
}

// readDoubleQuoted reads back one token quoteArgument rendered: it strips the
// wrapping quotes and undoes the one escape quoteArgument writes. It stops at
// the first unescaped quote and returns what follows it unread, so a rendering
// whose quoting ends early comes back as more than the token that went in.
// The naive version, which stripped the outer characters and unescaped the
// middle, healed exactly the defect this round trip exists to catch: it read
// "a"b" back as a"b and reported the missing escape as correct.
func readDoubleQuoted(rendered string) string {
	if len(rendered) < 2 || rendered[0] != '"' {
		return rendered
	}
	var token strings.Builder
	for i := 1; i < len(rendered); i++ {
		switch {
		case rendered[i] == '\\' && i+1 < len(rendered) && rendered[i+1] == '"':
			token.WriteByte('"')
			i++
		case rendered[i] == '"':
			return token.String() + rendered[i+1:]
		default:
			token.WriteByte(rendered[i])
		}
	}
	return token.String()
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
