package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"dinah/internal/addressform"
	"dinah/internal/bench"
	"dinah/internal/mcp"
	"dinah/internal/verb"
)

// storageFixture is the store the storage migration's goldens were captured
// over, and storageGoldens the answers the build at the branch's merge base
// gave on it.
var (
	storageFixture = filepath.Join("..", "..", filepath.FromSlash(addressform.PackageDir), "testdata", "storagemigrate")
	storageGoldens = filepath.Join(storageFixture, "golden", "golden.json")
)

// goldenRow is one captured answer: a command line and what it printed, or
// the MCP show tool's answer for one reference.
type goldenRow struct {
	Args    []string `json:"args"`
	Exit    int      `json:"exit"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
	MCPShow string   `json:"mcp_show"`
}

// migratedFixture copies the migration fixture into a .dinah container under
// its own identifier, migrates it with the layout switched on, and answers
// the store's directory and the directory a command runs from.
func migratedFixture(t *testing.T) (store, root string) {
	t.Helper()
	bench.EnableCardUnitForTest(t)
	id, err := os.ReadFile(filepath.Join(storageFixture, "workbench-id.txt"))
	if err != nil {
		t.Fatalf("read the fixture's identifier: %v", err)
	}
	base := t.TempDir()
	root = filepath.Join(base, "wb")
	store = filepath.Join(root, bench.UserBaseName, strings.TrimSpace(string(id)))
	copyFixtureTree(t, filepath.Join(storageFixture, "before"), store)
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "sam")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	got := runCLI(t, root, "--workbench", store, "check", "--migrate-storage", "--backup", filepath.Join(base, "backup"))
	if got.code != 0 {
		t.Fatalf("migrate the fixture: %d\n%s%s", got.code, got.out, got.errw)
	}
	return store, root
}

// copyFixtureTree copies every file below one directory into another.
func copyFixtureTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", from, err)
	}
}

// normalisedAnswer replaces the store's own path with <store>, in both
// spellings a path reaches an answer in, and turns every separator after it
// into a slash, which is the normalisation the capture script applied.
func normalisedAnswer(text, store string) string {
	escaped := strings.ReplaceAll(store, `\`, `\\`)
	text = strings.ReplaceAll(text, escaped, "<store>")
	text = strings.ReplaceAll(text, store, "<store>")
	return storePath.ReplaceAllStringFunc(text, func(path string) string {
		return strings.ReplaceAll(strings.ReplaceAll(path, `\\`, "/"), `\`, "/")
	})
}

// storePath matches a normalised store path up to the next space or quote.
var storePath = regexp.MustCompile(`<store>[^\s"']*`)

// itemCommentHome matches where the old layout kept an item comment's
// attachments, which is the one path the migration moves.
var itemCommentHome = regexp.MustCompile(`(<store>/cards/[0-9a-f]{12})/checklist/[0-9a-f]{12}/comments/`)

// TestTheMigratedFixtureAnswersEveryGolden drives dinah-637/criteria/3 and
// criteria/4. Every answer the merge-base build gave on the fixture, the
// member reference rows and their refusals included, is reproduced byte for
// byte by this build on the fixture once migrated, after the capture's own
// normalisation. The one path the migration moves, an item comment's
// attachment payload, is compared at the home it moved to.
//
// Arming: composing an item's anchor in a fixed key order rather than the
// order its writers put the keys in reddens the show rows of every item whose
// history set a key after filing, which is the order the goldens pin.
func TestTheMigratedFixtureAnswersEveryGolden(t *testing.T) {
	data, err := os.ReadFile(storageGoldens)
	if err != nil {
		t.Fatalf("read the goldens: %v", err)
	}
	var rows []goldenRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("decode the goldens: %v", err)
	}
	store, root := migratedFixture(t)
	resolving, refusing, mcpRows := 0, 0, 0
	for _, row := range rows {
		if row.MCPShow != "" {
			mcpRows++
			got := normalisedAnswer(mcpShow(t, store, row.MCPShow), store)
			want := itemCommentHome.ReplaceAllString(row.Stdout, "$1/comments/")
			if got != want {
				t.Errorf("the MCP show tool on %s differs:\n%s", row.MCPShow, diffLines(want, got))
			}
			continue
		}
		args := append([]string{"--workbench", store}, row.Args...)
		got := runCLI(t, root, args...)
		stdout := normalisedAnswer(got.out, store)
		stderr := normalisedAnswer(got.errw, store)
		want := itemCommentHome.ReplaceAllString(row.Stdout, "$1/comments/")
		if row.Exit == 0 {
			resolving++
		} else {
			refusing++
		}
		if got.code != row.Exit {
			t.Errorf("dinah %s exited %d, the golden %d: %s", strings.Join(row.Args, " "), got.code, row.Exit, stderr)
			continue
		}
		if stdout != want {
			t.Errorf("dinah %s differs on stdout:\n%s", strings.Join(row.Args, " "), diffLines(want, stdout))
		}
		if stderr != row.Stderr {
			t.Errorf("dinah %s differs on stderr:\n%s", strings.Join(row.Args, " "), diffLines(row.Stderr, stderr))
		}
	}
	t.Logf("%d answering rows, %d refusal rows and %d MCP rows compared", resolving, refusing, mcpRows)
	if resolving != 93 || refusing != 13 || mcpRows != 2 {
		t.Errorf("the goldens held %d answering rows, %d refusal rows and %d MCP rows, wanted 93, 13 and 2", resolving, refusing, mcpRows)
	}
}

// mcpShow answers the text of the MCP show tool's answer for one reference,
// over a server bounded by the store.
func mcpShow(t *testing.T, store, ref string) string {
	t.Helper()
	opened, err := bench.Open(store)
	if err != nil {
		t.Fatalf("open %s: %v", store, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	call, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "show", "arguments": map[string]any{"card": ref, "workbench": store}},
	})
	if err != nil {
		t.Fatalf("marshal the call: %v", err)
	}
	out := &strings.Builder{}
	if err := mcp.Serve(store, library, map[string]*verb.Library{}, strings.NewReader(string(call)+"\n"), out, mcp.ProfileAll); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var answer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &answer); err != nil {
		t.Fatalf("decode the MCP answer for %s: %v (%s)", ref, err, out.String())
	}
	var text strings.Builder
	for _, part := range answer.Result.Content {
		text.WriteString(part.Text)
	}
	return text.String()
}
