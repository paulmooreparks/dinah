package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The page's scripts keep each job that has a shared home in that home
// (docs/review/ledger.md, the Frontend rows). Each rule below says what a
// second copy of a job looks like in source, which files are its home, and
// the ledger row it guards. A rule fails the test with the file and line of
// every copy outside its home, so a new copy is caught when it is written
// and a deliberate exception has to be added to the rule by name.
type frontendRule struct {
	row     string
	what    string
	pattern *regexp.Regexp
	// match, when set, decides a line in place of pattern, for a rule a
	// regular expression without look-ahead cannot state.
	match func(line string) bool
	home  []string
}

var frontendRules = []frontendRule{
	{
		row:  "FE-14",
		what: "an HTML escaper written by hand; use dinahHTML.text, which escapes all five characters markup reads",
		// A character reference that opens a quoted string in any of the
		// three quote marks, named or numeric, is how an escaper's table
		// starts; a reference inside longer text is not one.
		pattern: regexp.MustCompile("['\"`]&(amp|lt|gt|quot|apos|#[0-9]+|#[xX][0-9a-fA-F]+);"),
		home:    []string{"safe-html.js"},
	},
	{
		row:  "FE-16",
		what: "a regular expression for a terminal escape sequence; use dinahTerminalText, which strips, converts and filters them in one place",
		// A regular expression literal, after the punctuation that can come
		// before one, holding an escape character in any of its spellings.
		// The Terminal's key codes and its own output are strings, not
		// patterns, and are not matched.
		pattern: regexp.MustCompile(`[(=,:!&|?]\s*/(?:[^/\\\n]|\\.)*\\(?:x1[bB]|u001[bB]|033)`),
		home:    []string{"terminal-text.js"},
	},
	{
		row:  "FE-17",
		what: "an @ list row drawn by hand; use dinahMentionList.draw, which draws a chat's and a group's alike",
		// The row's class, or the class of the title inside it, in a string.
		pattern: regexp.MustCompile(`['"]([^'"]* )?mention-(row|name)( [^'"]*)?['"]`),
		home:    []string{"mention-list.js"},
	},
	{
		row:   "FE-42",
		what:  "a value put into an attribute selector without CSS.escape",
		match: unescapedSelectorValue,
	},
}

// unescapedSelectorValue reports a line that builds an attribute selector,
// in querySelector, querySelectorAll, closest or matches, from a value
// joined in with + that is not passed through CSS.escape. A value with a
// quote in it would otherwise end the selector early or throw.
func unescapedSelectorValue(line string) bool {
	if !strings.Contains(line, "querySelector") && !strings.Contains(line, "closest(") && !strings.Contains(line, "matches(") {
		return false
	}
	for _, open := range []string{`="' + `, `=\"' + `} {
		rest := line
		for {
			i := strings.Index(rest, open)
			if i < 0 {
				break
			}
			rest = rest[i+len(open):]
			if !strings.HasPrefix(rest, "CSS.escape(") && !markup(line) {
				return true
			}
		}
	}
	return false
}

// markup reports a line that builds tags rather than a selector, as a
// selector never holds one; escaping values in markup is the FE-14 rule's
// concern.
var tag = regexp.MustCompile(`<[a-z]`)

func markup(line string) bool { return tag.MatchString(line) }

// frontendScripts lists Dinah's own scripts, in every folder beneath the
// desktop page and the web pages Go serves, without the browser suites,
// PUDL, the vendored libraries and the yavc.app content release (YAVC.md),
// which is shipped as its producer built it.
var notDinahs = map[string]bool{"e2e": true, "pudl": true, "vendor": true, "node_modules": true, "yavc": true}

func frontendScripts(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"frontend", filepath.Join("..", "..", "internal", "web", "static")} {
		err := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() && path != dir && notDinahs[e.Name()] {
				return filepath.SkipDir
			}
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(files) < 50 {
		t.Fatalf("found only %d frontend scripts; the test is not reading the page's folder", len(files))
	}
	return files
}

func TestFrontendJobsStayInTheirHomes(t *testing.T) {
	files := frontendScripts(t)
	for _, rule := range frontendRules {
		home := map[string]bool{}
		for _, h := range rule.home {
			home[h] = true
		}
		for _, file := range files {
			if home[filepath.Base(file)] && filepath.Base(filepath.Dir(file)) == "frontend" {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for n, line := range strings.Split(string(data), "\n") {
				if rule.match != nil && rule.match(line) || rule.pattern != nil && rule.pattern.MatchString(line) {
					t.Errorf("%s:%d: %s (%s)", filepath.ToSlash(file), n+1, rule.what, rule.row)
				}
			}
		}
	}
}

// Each rule must still find what it looks for in its home, or the rule
// guards nothing: a renamed home or a changed pattern would pass silently.
func TestFrontendRulesFindTheirHomes(t *testing.T) {
	for _, rule := range frontendRules {
		if rule.pattern == nil {
			continue
		}
		found := false
		for _, h := range rule.home {
			data, err := os.ReadFile(filepath.Join("frontend", h))
			if err != nil {
				t.Fatalf("%s: home %s: %v", rule.row, h, err)
			}
			if rule.pattern.Match(data) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: the pattern matches nothing in its home %v", rule.row, rule.home)
		}
	}
}
