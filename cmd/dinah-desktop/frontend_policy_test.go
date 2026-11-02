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
	home    []string
}

var frontendRules = []frontendRule{
	{
		row:     "FE-14",
		what:    "an HTML escaper written by hand; use dinahHTML.text, which escapes all five characters markup reads",
		pattern: regexp.MustCompile(`'&amp;'|"&amp;"|'&lt;'|"&lt;"|'&quot;'|"&quot;"`),
		home:    []string{"safe-html.js"},
	},
}

// frontendScripts lists Dinah's own scripts: those of the desktop page and
// of the web pages Go serves, without the browser suites and the vendored
// libraries.
func frontendScripts(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"frontend", filepath.Join("..", "..", "internal", "web", "static")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
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
				if rule.pattern.MatchString(line) {
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
