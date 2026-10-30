package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"studio/internal/providers"
)

// providerNameInPage matches the name of a provider or of a model family in a
// line of the page. A provider's name and what it can do belong in that
// provider's own module and in the registry the engine sends (provider_registry.go),
// and everything else reads them from there.
var providerNameInPage = regexp.MustCompile(`(?i)\b(devin|claude|codex|ollama|opus|sonnet|haiku|gpt-?[0-9]|swe-[0-9]|kimi|deepseek|gemini|anthropic|openai)`)

// pageProviderNames is the number of lines, outside comments, that still spell
// such a name, by file. It may only shrink: a step that removes names from a
// file lowers its number in the same commit, and a file not listed here may
// have none. The list is the page's inventory of what is left to change
// (docs/plans/group-chat.md, "No Hard-Coded Providers or Models").
var pageProviderNames = map[string]int{}

// withoutComments removes what a reader of the page does not see as words:
// whole-line and trailing // comments and /* */ blocks, in the page's scripts
// and styles. It is a line scanner, not a parser, and leaves a // inside a
// string such as a URL alone by requiring the // to follow a space or the
// start of the line.
func withoutComments(path, text string) []string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasSuffix(path, ".html") {
			out = append(out, line)
			continue
		}
		rest := line
		var kept strings.Builder
		for len(rest) > 0 {
			if inBlock {
				end := strings.Index(rest, "*/")
				if end < 0 {
					rest = ""
					break
				}
				rest = rest[end+2:]
				inBlock = false
				continue
			}
			start := strings.Index(rest, "/*")
			line2 := strings.Index(rest, "//")
			if line2 >= 0 && (line2 == 0 || rest[line2-1] == ' ' || rest[line2-1] == '\t' || rest[line2-1] == ';') && (start < 0 || line2 < start) {
				kept.WriteString(rest[:line2])
				rest = ""
				break
			}
			if start < 0 {
				kept.WriteString(rest)
				rest = ""
				break
			}
			kept.WriteString(rest[:start])
			rest = rest[start+2:]
			inBlock = true
		}
		out = append(out, kept.String())
	}
	return out
}

// moduleProvider is the id of the provider a file named provider-<id>.js belongs to, when the engine's table has
// that id, and empty for any other file.
func moduleProvider(base string) string {
	if !strings.HasPrefix(base, "provider-") || !strings.HasSuffix(base, ".js") {
		return ""
	}
	id := strings.TrimSuffix(strings.TrimPrefix(base, "provider-"), ".js")
	if _, ok := providers.Registry[id]; !ok {
		return ""
	}
	return id
}

// otherProviderNames matches the id or the name of every provider but the one given.
func otherProviderNames(own string) *regexp.Regexp {
	var words []string
	for id, row := range providers.Registry {
		if id != own {
			words = append(words, regexp.QuoteMeta(id), regexp.QuoteMeta(row.Name))
		}
	}
	return regexp.MustCompile(`(?i)\b(` + strings.Join(words, "|") + `)\b`)
}

func TestPageNamesNoProviderOutsideTheList(t *testing.T) {
	root := "frontend"
	got := map[string]int{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "e2e", "vendor", "pudl", "node_modules", "fonts":
				return fs.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".js", ".html", ".css":
		default:
			return nil
		}
		// A provider's own module, provider-<id>.js with the id of a provider in the engine's table, is that provider's
		// surface, as its adapter is in the engine, and may name it and no other. A file with the name of a module and
		// an id that is not in the table is not exempt.
		if own := moduleProvider(filepath.Base(path)); own != "" {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			other := otherProviderNames(own)
			for i, line := range withoutComments(path, string(raw)) {
				if other.MatchString(line) {
					t.Errorf("%s line %d names a provider other than its own (%s): %s", path, i+1, own, strings.TrimSpace(line))
				}
			}
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		n := 0
		for _, line := range withoutComments(path, string(raw)) {
			// The page's own list of the provider modules it includes is the one place their file names appear.
			if strings.Contains(line, `<script src="provider-`) {
				continue
			}
			if providerNameInPage.MatchString(line) {
				n++
			}
		}
		if n > 0 {
			got[filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))] = n
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for f := range got {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		allowed, listed := pageProviderNames[f]
		switch {
		case !listed:
			t.Errorf("%s spells a provider or model name on %d lines and is not on the list; read the name from the provider record instead", f, got[f])
		case got[f] > allowed:
			t.Errorf("%s spells a provider or model name on %d lines, more than the %d the list allows", f, got[f], allowed)
		case got[f] < allowed:
			t.Errorf("%s now has %d such lines, fewer than the %d on the list; lower its number in provider_names_in_page_test.go", f, got[f], allowed)
		}
	}
	for f, allowed := range pageProviderNames {
		if _, ok := got[f]; !ok {
			t.Errorf("%s is on the list with %d lines but has none now; remove it from the list", f, allowed)
		}
	}
	if t.Failed() {
		var lines []string
		for _, f := range files {
			lines = append(lines, fmt.Sprintf("\t%q: %d,", f, got[f]))
		}
		t.Logf("current counts:\n%s", strings.Join(lines, "\n"))
	}
}

// TestPageNameFinderSeesAndSkips pins what the scan counts: a name in code
// or markup, and not one in a comment.
func TestPageNameFinderSeesAndSkips(t *testing.T) {
	code := "var a = 'Claude'; // Devin says so\n/* Codex\n   Ollama */ var b = 1;\nvar c = 'x';\n"
	count := 0
	for _, line := range withoutComments("x.js", code) {
		if providerNameInPage.MatchString(line) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("the scan found %d lines with a name, want only the one in code", count)
	}
}

// TestPageModuleExemptionIsNarrow pins the exemption: a file is a provider's module only when it is named
// provider-<id>.js for an id in the engine's table, and what a module may not say is another provider's name.
func TestPageModuleExemptionIsNarrow(t *testing.T) {
	if moduleProvider("provider-ollama.js") != "ollama" {
		t.Fatal("provider-ollama.js is not seen as Ollama's module")
	}
	for _, name := range []string{"provider-something.js", "provider-facts.js", "settings-providers.js", "provider-ollama.css", "ollama.js"} {
		if id := moduleProvider(name); id != "" {
			t.Fatalf("%s is seen as the module of %q", name, id)
		}
	}
	other := otherProviderNames("ollama")
	if !other.MatchString("send it to Codex") || !other.MatchString("a Claude chat") {
		t.Fatal("another provider's name is not caught in a module")
	}
	if other.MatchString("Ollama's own words") {
		t.Fatal("a module's own provider is caught")
	}
}
