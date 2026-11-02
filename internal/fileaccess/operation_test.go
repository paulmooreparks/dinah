package fileaccess

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"studio/internal/layout"
)

func TestSearchAuditCommitsOneSummaryAndPreservesEarlierReads(t *testing.T) {
	store, _ := testStore(t)
	old, err := store.Append(Record{Chat: "chat", Path: "/earlier", Outcome: "allowed", Bytes: 3})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	denied := filepath.Join(home, ".netrc")
	if err := os.WriteFile(denied, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Chat: "chat", Workspace: "workspace", Source: "find_files", Roots: []string{t.TempDir()}, Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	svc := &Service{Store: store, Defaults: []Rule{{ID: "network-credentials", Kind: "default", Enabled: true, ResolvedPaths: []string{denied}, Reason: "Synthetic credential rule"}}}
	var before, after int
	if err := store.db.QueryRow(`SELECT total_changes()`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	text, ok := svc.RunOperation(context.Background(), home, scope, func(ctx context.Context) (string, bool) {
		for i := 0; i < 20000; i++ {
			path := filepath.Join(home, fmt.Sprintf("missing-%d", i))
			if i == 0 {
				path = denied
			}
			if _, err := svc.Read(ctx, path, nil, nil, scope); err == nil {
				t.Fatal("synthetic path unexpectedly returned content")
			}
		}
		return "No file matches.", true
	})
	if !ok || strings.Contains(text, "secret") || !strings.Contains(text, "20000 paths visited; 1 denied") {
		t.Fatalf("search: %q %v", text, ok)
	}
	if err := store.db.QueryRow(`SELECT total_changes()`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after-before > 8 {
		t.Fatalf("per-candidate writes returned: %d", after-before)
	}
	log, err := store.Log(Filter{})
	if err != nil || len(log.Rows) != 2 || log.Rows[1].ID != old.ID {
		t.Fatalf("earlier read lost: %+v %v", log, err)
	}
	summary := log.Rows[0]
	if !summary.Operation || summary.Visited != 20000 || summary.Denied != 1 || summary.Errors != 19999 || summary.Bytes != len(text) {
		t.Fatalf("summary: %+v", summary)
	}
}

func TestDirectoryListingOmitsDeniedChildren(t *testing.T) {
	store, _ := testStore(t)
	home := t.TempDir()
	for _, name := range []string{".netrc", "visible"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Chat: "chat", Workspace: "workspace", Source: "list_directory", Roots: []string{home}, Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	svc := &Service{Store: store, Defaults: DefaultRules(runtime.GOOS, home, "", "", nil)}
	_, ok := svc.RunOperation(context.Background(), home, scope, func(ctx context.Context) (string, bool) {
		entries, err := svc.ReadDirectory(ctx, home, scope)
		if err != nil || len(entries) != 1 || entries[0].Name() != "visible" {
			t.Fatalf("listed denied name: %v %v", entries, err)
		}
		return "visible", true
	})
	if !ok {
		t.Fatal("listing failed")
	}
	log, err := store.Log(Filter{})
	if err != nil || len(log.Rows) != 1 || log.Rows[0].Denied != 2 {
		t.Fatalf("listing summary: %+v %v", log, err)
	}
}

func TestWindowsPipeDenyRootDoesNotRefuseOrdinaryReads(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows pipe policy")
	}
	store, _ := testStore(t)
	home := t.TempDir()
	file := filepath.Join(home, "ordinary")
	if err := os.WriteFile(file, []byte("allowed"), 0600); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Chat: "chat", Workspace: "workspace", Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	svc := &Service{Store: store, Defaults: DefaultRules("windows", home, "", "", map[string]string{"SSH_AUTH_SOCK": `\\.\pipe\openssh-ssh-agent`})}
	text, err := svc.Read(context.Background(), file, nil, nil, scope)
	if err != nil || text != "allowed" {
		t.Fatalf("pipe root refused ordinary file: %q %v", text, err)
	}
}

func TestAcknowledgementPreservesUnshownRefusalGroups(t *testing.T) {
	store, _ := testStore(t)
	for i := 0; i < 5; i++ {
		if _, err := store.Append(Record{Chat: "chat", Source: "read", Path: "/denied", RuleID: "rule", Outcome: "denied"}); err != nil {
			t.Fatal(err)
		}
	}
	var high int64
	for i := 0; i < 60; i++ {
		r, err := store.Append(Record{Chat: "chat", Path: "/read", Outcome: "allowed", Bytes: 1})
		if err != nil {
			t.Fatal(err)
		}
		high = r.ID
	}
	if _, err := store.Acknowledge("chat", high); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(Record{Chat: "chat", Source: "read", Path: "/denied", RuleID: "rule", Outcome: "denied"}); err != nil {
		t.Fatal(err)
	}
	log, err := store.Log(Filter{Outcome: "denied"})
	if err != nil || len(log.Rows) != 2 || log.Rows[0].Count != 1 || log.Rows[1].Count != 5 {
		t.Fatalf("acknowledgement erased history: %+v %v", log, err)
	}
}

func TestErrorFloodCollapsesWithoutEvictingReads(t *testing.T) {
	store, _ := testStore(t)
	old, err := store.Append(Record{Chat: "chat", Path: "/read", Outcome: "allowed", Bytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := store.Append(Record{Chat: "chat", Path: fmt.Sprintf("/missing/%d", i), Source: "stat", Outcome: "error"}); err != nil {
			t.Fatal(err)
		}
	}
	log, err := store.Log(Filter{})
	if err != nil || len(log.Rows) != 2 || log.Rows[0].Count != 1000 || log.Rows[1].ID != old.ID {
		t.Fatalf("error flood displaced reads: %+v %v", log, err)
	}
	counts, err := store.RefusedCounts()
	if err != nil || counts["chat"] != 0 {
		t.Fatalf("errors counted as refusals: %v %v", counts, err)
	}
}

func TestDefaultOverridesAndSandboxProfiles(t *testing.T) {
	for _, platform := range []string{"windows", "linux", "darwin"} {
		home, roaming, local := "/home/person", "/roaming", "/local"
		if platform == "windows" {
			home, roaming, local = "C:/Users/person", "C:/Roaming", "C:/Local"
		}
		env := map[string]string{"CLAUDE_CONFIG_DIR": home + "/account-a", "CODEX_HOME": home + "/account-b"}
		rules := DefaultRules(platform, home, roaming, local, env)
		all := []string{}
		for _, rule := range rules {
			all = append(all, rule.ResolvedPaths...)
		}
		expected := []string{home + "/account-a", home + "/account-b"}
		if platform == "windows" {
			expected = append(expected, roaming+"/gnupg", local+"/1Password")
		}
		if platform == "linux" {
			expected = append(expected, home+"/snap/firefox/common/.mozilla/firefox", home+"/snap/chromium/common/chromium", home+"/.var/app/org.mozilla.firefox", home+"/.var/app/org.chromium.Chromium", home+"/.var/app/com.google.Chrome", home+"/.var/app/com.microsoft.Edge", home+"/.var/app/com.brave.Browser", home+"/.var/app/com.opera.Opera", home+"/.var/app/com.vivaldi.Vivaldi")
		}
		for _, want := range expected {
			if platform == "windows" {
				want = strings.ReplaceAll(want, "/", `\`)
			}
			found := false
			for _, got := range all {
				if got == want {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s default %s", platform, want)
			}
		}
	}
}

func TestOperationWithholdsResultWhenAuditCannotCommit(t *testing.T) {
	store, _ := testStore(t)
	svc := &Service{Store: store}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	text, ok := svc.RunOperation(context.Background(), t.TempDir(), Scope{Chat: "chat", Workspace: "workspace", Source: "find_files"}, func(context.Context) (string, bool) { return "synthetic private result", true })
	if ok || strings.Contains(text, "synthetic private result") {
		t.Fatalf("unaudited result escaped: %q", text)
	}
}
