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
		row:  "FE-14",
		what: "an HTML escaper written by hand; use dinahHTML.text, which escapes all five characters markup reads",
		// A character reference that opens a quoted string in any of the
		// three quote marks, named or numeric, is how an escaper's table
		// starts; a reference inside longer text is not one.
		pattern: regexp.MustCompile("['\"`]&(amp|lt|gt|quot|apos|#[0-9]+|#[xX][0-9a-fA-F]+);"),
		home:    []string{"safe-html.js"},
	},
}

// frontendScripts lists Dinah's own scripts, in every folder beneath the
// desktop page and the web pages Go serves, without the browser suites,
// PUDL and the vendored libraries.
var notDinahs = map[string]bool{"e2e": true, "pudl": true, "vendor": true, "node_modules": true}

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
