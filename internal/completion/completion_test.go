package completion

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// bashBreaks is bash's default COMP_WORDBREAKS, which is what every
// interactive bash hands the script unless the person changed it.
const bashBreaks = " \t\n\"'><=;|&(:"

// TestSplitBashReadsTheLineTheWayReadlineDoes asserts the splitter's words,
// current word and replacement point on the lines the completion scripts
// meet, the comma list and the field:value term among them, where the
// replacement starts after the last colon and not after a comma.
func TestSplitBashReadsTheLineTheWayReadlineDoes(t *testing.T) {
	cases := []struct {
		line        string
		words       []string
		current     string
		replaceFrom int
	}{
		{"dinah query colu", []string{"query"}, "colu", 0},
		{"dinah query column:sp", []string{"query"}, "column:sp", 7},
		{"dinah query column:spec,te", []string{"query"}, "column:spec,te", 7},
		{"dinah show x --fields=card,bo", []string{"show", "x"}, "--fields=card,bo", 9},
		{"dinah show ", []string{"show"}, "", 0},
		{"FOO=1 BAR=2 dinah show fx-", []string{"show"}, "fx-", 0},
		{"echo a; dinah sh", nil, "sh", 0},
		{"true && dinah move fx-1 d", []string{"move", "fx-1"}, "d", 0},
		{`dinah add "Café au lait`, []string{"add"}, "Café au lait", 0},
		{`dinah comment fx-1 'a b' --kin`, []string{"comment", "fx-1", "a b"}, "--kin", 0},
		{`dinah query a\:b`, []string{"query"}, "a:b", 0},
		{`dinah query "column:sp`, []string{"query"}, "column:sp", 0},
		{`dinah query x"y\"z"w`, []string{"query"}, `xy"zw`, 0},
	}
	for _, c := range cases {
		words, current, replaceFrom, found := SplitBash(c.line, bashBreaks)
		if !found {
			t.Errorf("%q: no program word found", c.line)
			continue
		}
		if strings.Join(words, "|") != strings.Join(c.words, "|") || current != c.current || replaceFrom != c.replaceFrom {
			t.Errorf("%q: got words %q, current %q, replaceFrom %d; wanted %q, %q, %d",
				c.line, words, current, replaceFrom, c.words, c.current, c.replaceFrom)
		}
	}
	for _, line := range []string{"", "FOO=1 ", "dinah"} {
		_, current, _, found := SplitBash(line, bashBreaks)
		if line == "dinah" {
			if found {
				t.Errorf("%q: the program word itself was read as an argument, current %q", line, current)
			}
			continue
		}
		if found {
			t.Errorf("%q: a program word was found where none stands", line)
		}
	}
}

// TestSafeWordAdmitsOnlyBareWords asserts the one test every candidate
// passes: letters, digits and the eight punctuation characters no shell
// quotes, and nothing else, whatever script the letters are in.
func TestSafeWordAdmitsOnlyBareWords(t *testing.T) {
	for _, word := range []string{"dinah-59", "column:spec,test", "card.kind", "a_b/c=d+e", "café", "カード"} {
		if !SafeWord(word) {
			t.Errorf("%q was refused and is a bare word in every shell", word)
		}
	}
	for _, word := range []string{"", "very high", `say"hi"`, "it's", "a$b", "a`b", "a;b", "a\tb", "a@b", "(x)", "a*"} {
		if SafeWord(word) {
			t.Errorf("%q was admitted and needs quoting in at least one shell", word)
		}
	}
}

// TestSanitizeMakesADescriptionOneShortLine asserts that control characters
// become spaces, runs of space collapse, quotation marks survive, and a long
// description is cut to sixty display columns ending in an ellipsis.
func TestSanitizeMakesADescriptionOneShortLine(t *testing.T) {
	if got := Sanitize("Say \"hi\"\tto \"them\"\r\n"); got != `Say "hi" to "them"` {
		t.Errorf("got %q", got)
	}
	long := strings.Repeat("abcdefghij", 8)
	cut := Sanitize(long)
	if !strings.HasSuffix(cut, "…") || utf8.RuneCountInString(cut) != 60 {
		t.Errorf("a description of %d columns became %q, %d runes", len(long), cut, utf8.RuneCountInString(cut))
	}
	if got := Sanitize(strings.Repeat("x", 60)); got != strings.Repeat("x", 60) {
		t.Errorf("a description of exactly sixty columns was cut: %q", got)
	}
}

// TestWriteTrimsWhatTheShellKeeps asserts the wire form: the header, then one
// line per candidate carrying the insert, one TAB and the description, with
// the runes the shell leaves on the line removed from the front.
func TestWriteTrimsWhatTheShellKeeps(t *testing.T) {
	var out bytes.Buffer
	candidates := []Candidate{{Word: "column:spec,test", Description: "Test\tcolumn"}, {Word: "column:spec,done"}}
	if err := Write(&out, ModeWords, candidates, 12, true); err != nil {
		t.Fatal(err)
	}
	want := "dinah-complete 1 words\ntest\tTest column\ndone\t\n"
	if out.String() != want {
		t.Errorf("got %q, wanted %q", out.String(), want)
	}
	out.Reset()
	if err := Write(&out, ModeNospace, candidates[:1], 0, false); err != nil {
		t.Fatal(err)
	}
	if out.String() != "dinah-complete 1 nospace\ncolumn:spec,test\t\n" {
		t.Errorf("a description reached a shell that shows none: %q", out.String())
	}
}

// TestEveryScriptIsItsFileWithTheProtocolSubstituted asserts that each of the
// four scripts is the embedded file with the protocol token replaced and
// nothing else changed, and that an unknown shell has no script.
func TestEveryScriptIsItsFileWithTheProtocolSubstituted(t *testing.T) {
	files := map[string]string{"powershell": "dinah.ps1", "bash": "dinah.bash", "zsh": "dinah.zsh", "fish": "dinah.fish"}
	if len(Shells) != len(files) {
		t.Fatalf("Shells names %d shells and this test knows %d", len(Shells), len(files))
	}
	for _, shell := range Shells {
		raw, err := os.ReadFile(filepath.Join("scripts", files[shell]))
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		if !bytes.Contains(raw, []byte(protocolToken)) {
			t.Errorf("%s: the file carries no protocol token, so a script would not check the header's version", shell)
		}
		script, ok := Script(shell)
		if !ok {
			t.Fatalf("%s: no script", shell)
		}
		want := strings.ReplaceAll(string(raw), protocolToken, "1")
		if script != want {
			t.Errorf("%s: the printed script differs from the file with only the token replaced", shell)
		}
	}
	if _, ok := Script("cmd"); ok {
		t.Error("a shell outside Shells was given a script")
	}
}
