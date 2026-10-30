package profile

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"studio/internal/providers"
)

// Shared engine code asks what a provider can do, from its row in the provider
// table, and does not compare with a provider's name (docs/plans/
// provider-names-in-go.md). This test finds every Go string literal in the
// programs and their packages that is a provider's ID, or begins with one and a
// colon as a chat's session ID does, and fails for one that is not allowed.
//
// The permanent exceptions are a provider's own files in the window
// (provider_<id>.go, for that provider's name only), the adapters, which are each provider's own
// code, the table that declares the providers, the profile that offers them,
// the command line's launchers for two of them and one stored value. The
// sites still to change are listed by file with their count, and the list may
// only shrink: a literal added anywhere fails at once, and so does a count that
// fell without the list being lowered, so each step deletes its own lines and
// the list shows how much is left.

var providerIDs = []string{"devin", "codex", "claude", "ollama"}

// adapterDirs are each provider's own code: the adapters.
var adapterDirs = []string{"internal/claudeagent", "internal/codexagent", "internal/ollamaagent", "internal/agents"}

// ownFileProvider is the provider a file belongs to when it is named
// provider_<id>.go in the window, or judge_<id>.go in the judgement package, for
// an id in the table: its literals of that provider's own name are that
// provider's own work, and any other provider's name in it counts.
func ownFileProvider(rel string) string {
	base := filepath.Base(rel)
	var prefix string
	switch filepath.ToSlash(filepath.Dir(rel)) {
	case "cmd/dinah-desktop":
		prefix = "provider_"
	case "internal/judge":
		prefix = "judge_"
	}
	if prefix == "" || !strings.HasPrefix(base, prefix) {
		return ""
	}
	id := strings.TrimSuffix(strings.TrimPrefix(base, prefix), ".go")
	if _, ok := providers.Registry[id]; ok {
		return id
	}
	return ""
}

// permanentExceptions are files whose literals are the point of the file, with
// the reason: the table is where the providers are declared, the profile is
// where the product says which it offers, the command line's launchers are the
// two programs' own, and ApprovalsDevin is a stored value that cannot change
// without a migration.
var permanentExceptions = map[string]string{
	"internal/providers/providers.go": "the provider table",
	"cmd/dinah/main.go":               "the command line's launchers for claude and codex",
	"cmd/dinah-desktop/settings.go":   "ApprovalsDevin, a stored value",
}

// stillToChange is the number of provider-name literals left in each file that
// is to change, by file. It may only shrink, and a file leaves it when its count
// reaches 0.
var stillToChange = map[string]int{}

func isProviderName(value string) bool {
	for _, id := range providerIDs {
		if value == id || strings.HasPrefix(value, id+":") {
			return true
		}
	}
	return false
}

func TestNoProviderNamesInSharedCodeBeyondTheList(t *testing.T) {
	root := filepath.Join("..", "..")
	counts := map[string]int{}
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if d.Name() == "frontend" || d.Name() == "static" || d.Name() == "profile" {
					return filepath.SkipDir
				}
				for _, adapter := range adapterDirs {
					if rel == adapter {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if _, ok := permanentExceptions[rel]; ok {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if value, err := strconv.Unquote(lit.Value); err == nil && isProviderName(value) {
					if own := ownFileProvider(rel); own != "" && (value == own || strings.HasPrefix(value, own+":")) {
						return true
					}
					counts[rel]++
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]bool{}
	for f := range counts {
		files[f] = true
	}
	for f := range stillToChange {
		files[f] = true
	}
	var names []string
	for f := range files {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		have, allowed := counts[f], stillToChange[f]
		switch {
		case have > allowed && allowed == 0:
			t.Errorf("%s has %d provider-name literals and is not on the list: ask the provider's row for a fact, or put the code in a file of that provider", f, have)
		case have > allowed:
			t.Errorf("%s has %d provider-name literals, and the list allows %d: do not add another", f, have, allowed)
		case have < allowed && have == 0:
			t.Errorf("%s has none left: remove it from stillToChange", f)
		case have < allowed:
			t.Errorf("%s is down to %d literals: lower its count in stillToChange from %d", f, have, allowed)
		}
	}
}
