package ollamaagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
)

// limited builds the tools of a turn whose file tools may go only to the
// folders the function gives. It has no connection, so a call that tried to
// ask the person would fail the test.
func limited(cwd string, folders func() []string) turnTools {
	var refusal string
	return turnTools{session: "chat", cwd: cwd, commands: ApprovalsEvery, allowed: map[string]bool{}, mu: &sync.Mutex{}, refusal: &refusal,
		scope: func(string, string) ([]string, bool) { return folders(), true }}
}

func fixedFolders(folders ...string) func() []string { return func() []string { return folders } }

// layoutFor makes work, work-secrets beside it and elsewhere, each with a file
// in it, and returns their paths.
func layoutFor(t *testing.T) (work, sibling, elsewhere string) {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"work/sub", "work-secrets", "elsewhere"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, text string) {
		if err := os.WriteFile(filepath.Join(root, p), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("work/a.txt", "needle in work\n")
	write("work/sub/b.txt", "needle in sub\n")
	write("work-secrets/s.txt", "needle in secrets\n")
	write("elsewhere/e.txt", "needle elsewhere\n")
	return filepath.Join(root, "work"), filepath.Join(root, "work-secrets"), filepath.Join(root, "elsewhere")
}

// every tool call that names a path, as the model would make it.
func pathCalls(p string) map[string]map[string]any {
	return map[string]map[string]any{
		"read_file":      {"path": p},
		"list_directory": {"path": p},
		"find_files":     {"path": p, "pattern": "**/*"},
		"search_text":    {"path": p, "pattern": "needle"},
		"write_file":     {"path": filepath.Join(p, "new.txt"), "content": "x"},
	}
}

func refused(t *testing.T, text string, ok bool) bool {
	t.Helper()
	return !ok && strings.Contains(text, "outside the folders this chat may use") && strings.Contains(text, settingsPlace)
}

// A path with ".." that leaves the folder, and the folder next to it whose
// name begins with the same letters, are refused by every tool; the same
// paths inside the folder are not.
func TestFileToolsRefuseWhatLeavesTheFolders(t *testing.T) {
	work, sibling, elsewhere := layoutFor(t)
	tt := limited(work, fixedFolders(work))
	for name, outside := range map[string]string{
		"dot dot":     filepath.Join(work, "..", "elsewhere"),
		"sibling":     sibling,
		"unrelated":   elsewhere,
		"relative up": filepath.Join("..", "elsewhere"),
	} {
		for tool, args := range pathCalls(outside) {
			text, ok := tt.run(context.Background(), "1", tool, args)
			if !refused(t, text, ok) {
				t.Errorf("%s: %s on %s gave %v %q", name, tool, outside, ok, text)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "new.txt")); err == nil {
		t.Fatal("a write outside the folders was made")
	}
	// Inside, and a path that goes out and back in, work.
	if text, ok := tt.run(context.Background(), "1", "read_file", map[string]any{"path": filepath.Join(work, "..", "work", "a.txt")}); !ok || !strings.Contains(text, "needle in work") {
		t.Fatalf("a path that returns inside: %v %q", ok, text)
	}
	if text, ok := tt.run(context.Background(), "1", "search_text", map[string]any{"pattern": "needle"}); !ok || !strings.Contains(text, "a.txt:1") || !strings.Contains(text, "sub/b.txt:1") || strings.Contains(text, "secrets") {
		t.Fatalf("a search of the chat's own folder: %v %q", ok, text)
	}
}

// The refusal names the path, lists the folders in force and says where the
// limit is set.
func TestTheRefusalSaysWhereTheLimitIsSet(t *testing.T) {
	work, _, elsewhere := layoutFor(t)
	other := t.TempDir()
	tt := limited(work, fixedFolders(work, other))
	text, ok := tt.run(context.Background(), "1", "read_file", map[string]any{"path": filepath.Join(elsewhere, "e.txt")})
	for _, want := range []string{filepath.Join(elsewhere, "e.txt"), work, other, "Settings > Agents > Ollama > File Access"} {
		if ok || !strings.Contains(text, want) {
			t.Fatalf("the refusal lacks %q: %v %q", want, ok, text)
		}
	}
}

// A link inside the folder that leads out is not followed, by any tool, and
// one that stays inside is.
func TestFileToolsDoNotFollowALinkOutOfTheFolders(t *testing.T) {
	work, _, elsewhere := layoutFor(t)
	if err := os.Symlink(elsewhere, filepath.Join(work, "out")); err != nil {
		t.Skipf("this system will not make a link here: %v", err)
	}
	if err := os.Symlink("sub", filepath.Join(work, "in")); err != nil {
		t.Fatal(err)
	}
	tt := limited(work, fixedFolders(work))
	out := filepath.Join(work, "out")
	if text, ok := tt.run(context.Background(), "1", "read_file", map[string]any{"path": filepath.Join(out, "e.txt")}); ok || strings.Contains(text, "elsewhere") && strings.Contains(text, "needle") {
		t.Fatalf("a file read through a link out: %v %q", ok, text)
	}
	if text, ok := tt.run(context.Background(), "1", "list_directory", map[string]any{"path": out}); ok {
		t.Fatalf("a folder listed through a link out: %q", text)
	}
	if text, ok := tt.run(context.Background(), "1", "write_file", map[string]any{"path": filepath.Join(out, "made.txt"), "content": "x"}); ok {
		t.Fatalf("a file written through a link out: %q", text)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "made.txt")); err == nil {
		t.Fatal("the file was made outside the folders")
	}
	// A search of the folder does not go through the link to what is outside.
	text, ok := tt.run(context.Background(), "1", "search_text", map[string]any{"pattern": "needle"})
	if !ok || strings.Contains(text, "elsewhere") {
		t.Fatalf("a search went through a link out: %v %q", ok, text)
	}
	// A link that stays inside works.
	if text, ok := tt.run(context.Background(), "1", "read_file", map[string]any{"path": filepath.Join(work, "in", "b.txt")}); !ok || !strings.Contains(text, "needle in sub") {
		t.Fatalf("a link that stays inside: %v %q", ok, text)
	}
}

// A listed folder is reachable as well as the chat's own, and no other.
func TestOtherListedFoldersAreReachable(t *testing.T) {
	work, sibling, elsewhere := layoutFor(t)
	tt := limited(work, fixedFolders(work, elsewhere))
	if text, ok := tt.run(context.Background(), "1", "read_file", map[string]any{"path": filepath.Join(elsewhere, "e.txt")}); !ok || !strings.Contains(text, "needle elsewhere") {
		t.Fatalf("a listed folder: %v %q", ok, text)
	}
	if text, ok := tt.run(context.Background(), "1", "read_file", map[string]any{"path": filepath.Join(sibling, "s.txt")}); !refused(t, text, ok) {
		t.Fatalf("an unlisted folder: %v %q", ok, text)
	}
}

// A new file is written inside the folders, in a folder that is not there
// yet, without asking whatever the approvals are; the same write outside is
// refused and not asked.
func TestWritesInsideTheFoldersAreMadeAndNeverAsked(t *testing.T) {
	work, _, elsewhere := layoutFor(t)
	tt := limited(work, fixedFolders(work))
	target := filepath.Join(work, "deep", "er", "new.txt")
	if text, ok := tt.run(context.Background(), "1", "write_file", map[string]any{"path": target, "content": "hello"}); !ok {
		t.Fatalf("a write inside: %q", text)
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "hello" {
		t.Fatalf("the file: %q %v", b, err)
	}
	if text, ok := tt.run(context.Background(), "1", "write_file", map[string]any{"path": filepath.Join(elsewhere, "x.txt"), "content": "no"}); !refused(t, text, ok) {
		t.Fatalf("a write outside: %v %q", ok, text)
	}
}

// A command is not started in a folder outside the folders; one inside is
// not stopped by this.
func TestACommandsStartingFolderIsChecked(t *testing.T) {
	work, _, elsewhere := layoutFor(t)
	tt := limited(work, fixedFolders(work))
	text, ok := tt.run(context.Background(), "1", "run_command", map[string]any{"command": "echo hi", "cwd": elsewhere})
	if !refused(t, text, ok) {
		t.Fatalf("a command started outside: %v %q", ok, text)
	}
	if got := tt.checkDir(filepath.Join(work, "sub")); got != "" {
		t.Fatalf("a folder inside was refused: %q", got)
	}
}

// What the folders are is asked at each call, so a change the person makes
// applies from the next call of a chat that is running.
func TestTheFoldersAreAskedAtEveryCall(t *testing.T) {
	work, _, elsewhere := layoutFor(t)
	var mu sync.Mutex
	folders := []string{work}
	tt := limited(work, func() []string { mu.Lock(); defer mu.Unlock(); return folders })
	args := map[string]any{"path": filepath.Join(elsewhere, "e.txt")}
	if text, ok := tt.run(context.Background(), "1", "read_file", args); !refused(t, text, ok) {
		t.Fatalf("before: %v %q", ok, text)
	}
	mu.Lock()
	folders = []string{work, elsewhere}
	mu.Unlock()
	if text, ok := tt.run(context.Background(), "1", "read_file", args); !ok || !strings.Contains(text, "needle elsewhere") {
		t.Fatalf("after the folder was added: %v %q", ok, text)
	}
}

// With no limit the tools go where they always did.
func TestWithNoLimitTheFileToolsGoAnywhere(t *testing.T) {
	_, _, elsewhere := layoutFor(t)
	var refusal string
	tt := turnTools{session: "chat", cwd: t.TempDir(), allowed: map[string]bool{}, mu: &sync.Mutex{}, refusal: &refusal}
	if text, ok := tt.run(context.Background(), "1", "list_directory", map[string]any{"path": elsewhere}); !ok || !strings.Contains(text, "e.txt") {
		t.Fatalf("a listing with no limit: %v %q", ok, text)
	}
	if text, ok := tt.run(context.Background(), "1", "search_text", map[string]any{"path": elsewhere, "pattern": "needle"}); !ok || !strings.Contains(text, "needle elsewhere") {
		t.Fatalf("a search with no limit: %v %q", ok, text)
	}
	unlimited := tt
	unlimited.scope = func(string, string) ([]string, bool) { return nil, false }
	if text, ok := unlimited.run(context.Background(), "1", "list_directory", map[string]any{"path": elsewhere}); !ok {
		t.Fatalf("a scope that says anywhere: %q", text)
	}
}

// Through a whole turn: the connection's scope reaches the tools, so a model
// that asks for a file outside the folders is told so, and one inside is
// given the file.
func TestTheConnectionsScopeReachesAWholeTurn(t *testing.T) {
	for _, tc := range []struct {
		name    string
		allowed func(ws string) []string
		want    string
	}{
		{"outside", func(string) []string { return []string{t.TempDir()} }, "outside the folders this chat may use"},
		{"inside", func(ws string) []string { return []string{ws} }, "The release ships on Friday."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := fakeOllama(t, "read_file")
			ws := t.TempDir()
			os.WriteFile(filepath.Join(ws, "notes.md"), []byte("The release ships on Friday."), 0o644)
			h := &host{answer: "once"}
			c, err := Start(srv.URL, t.TempDir(), h)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			folders := tc.allowed(ws)
			c.SetFileScope(func(string, string) ([]string, bool) { return folders, true })
			var opened acp.NewSessionResult
			if err := c.Call(context.Background(), "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: ws}}, &opened); err != nil {
				t.Fatal(err)
			}
			var res acp.PromptResult
			if err := c.Call(context.Background(), "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "When does the release ship?"}}}, &res); err != nil {
				t.Fatal(err)
			}
			if said := h.said(); !strings.Contains(said, tc.want) {
				t.Fatalf("the turn did not say %q:\n%s", tc.want, said)
			}
		})
	}
}

// A limited search still stops at its file limit, and its walk stays in the
// folder.
func TestALimitedSearchKeepsItsWorkLimit(t *testing.T) {
	withSearchLimits(t, 1, SearchMaxBytes)
	work, _, _ := layoutFor(t)
	tt := limited(work, fixedFolders(work))
	text, ok := tt.run(context.Background(), "1", "search_text", map[string]any{"pattern": "needle"})
	if !ok || !strings.Contains(text, "[The search stopped early.") {
		t.Fatalf("a limited search past its limit: %v %q", ok, text)
	}
}
