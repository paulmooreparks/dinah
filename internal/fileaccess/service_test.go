package fileaccess

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"studio/internal/layout"
)

func TestReadPolicyAuditsWithoutReturningUnauditedContent(t *testing.T) {
	store, _ := testStore(t)
	root := t.TempDir()
	outside := t.TempDir()
	home := t.TempDir()
	state := t.TempDir()
	for _, dir := range []string{root, outside, filepath.Join(home, ".ssh"), state} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "value.txt"), []byte("synthetic text"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	scope := Scope{Chat: "chat-a", Workspace: "workspace-a", Roots: []string{root}, Layout: layout.Layout{State: state}}
	svc := &Service{Store: store, Defaults: DefaultRules("windows", home, "", "", nil)}
	ctx := context.Background()
	if text, err := svc.Read(ctx, filepath.Join(root, "value.txt"), nil, nil, scope); err != nil || text != "synthetic text" {
		t.Fatalf("workspace read: %q %v", text, err)
	}
	if page, err := store.Log(Filter{}); err != nil || len(page.Rows) != 0 {
		t.Fatal("workspace read was unnecessarily audited")
	}
	if text, err := svc.Read(ctx, filepath.Join(outside, "value.txt"), nil, nil, scope); err != nil || text != "synthetic text" {
		t.Fatalf("unconfined read: %q %v", text, err)
	}
	if text, err := svc.Read(ctx, filepath.Join(home, ".ssh", "value.txt"), nil, nil, scope); err == nil || text != "" {
		t.Fatal("SSH denial failed")
	}
	if text, err := svc.Read(ctx, filepath.Join(state, "value.txt"), nil, nil, scope); err == nil || text != "" {
		t.Fatal("protected denial failed")
	}
	page, err := store.Log(Filter{})
	if err != nil || len(page.Rows) != 3 || page.Rows[0].RuleID != "protected-state" || page.Rows[1].RuleID != "ssh" || page.Rows[2].Bytes != len("synthetic text") {
		t.Fatalf("audit attribution: %+v %v", page, err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_audit BEFORE INSERT ON audit BEGIN SELECT RAISE(ABORT,'test audit unavailable'); END;`); err != nil {
		t.Fatal(err)
	}
	if text, err := svc.Read(ctx, filepath.Join(outside, "value.txt"), nil, nil, scope); err == nil || text != "" {
		t.Fatal("content escaped without its audit commit")
	}
}

func TestReadPolicyChangesAndSymlinkAliases(t *testing.T) {
	store, _ := testStore(t)
	home, root := t.TempDir(), t.TempDir()
	denied := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(denied, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(denied, "key")
	if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "linked")
	if err := os.Symlink(denied, alias); err != nil {
		t.Fatal("symlink guard requires supported test host", err)
	}
	scope := Scope{Chat: "a", Workspace: "w", Roots: []string{root}, Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	svc := &Service{Store: store, Defaults: DefaultRules("windows", home, "", "", nil)}
	if text, err := svc.Read(context.Background(), filepath.Join(alias, "key"), nil, nil, scope); err == nil || text != "" {
		t.Fatal("alias bypassed denial")
	}
	if _, err := svc.Remove("protected-state", 1, scope); err == nil {
		t.Fatal("protected rule removed")
	}
	if _, err := svc.SetDefault("protected-state", false, 1, scope); err == nil {
		t.Fatal("protected rule disabled")
	}
	policy, err := svc.SetDefault("ssh", false, 1, scope)
	if err != nil {
		t.Fatal(err)
	}
	if text, err := svc.Read(context.Background(), path, nil, nil, scope); err != nil || text != "synthetic" {
		t.Fatal("disabled default stayed active", err)
	}
	policy, err = svc.Add(denied, policy.Version, scope)
	if err != nil {
		t.Fatal(err)
	}
	if text, err := svc.Read(context.Background(), path, nil, nil, scope); err == nil || text != "" {
		t.Fatal("custom entry did not deny")
	}
	custom := ""
	for _, r := range policy.Rules {
		if r.Kind == "custom" {
			custom = r.ID
		}
	}
	if _, err = svc.Remove(custom, policy.Version, scope); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Add("relative/path", policy.Version, scope); err == nil {
		t.Fatal("relative custom entry accepted")
	}
	check, err := svc.Check(filepath.Join(root, "new", "leaf"), scope)
	if err != nil || !check.Valid || !check.InsideWorkspace {
		t.Fatalf("new path preview: %+v %v", check, err)
	}
}

func TestDefaultResolversCoverEveryPlatformAndEntry(t *testing.T) {
	for _, platform := range []string{"windows", "linux", "darwin"} {
		home := "/home/person"
		roaming, local := "", ""
		if platform == "windows" {
			home = "C:/Users/person"
			roaming = home + "/AppData/Roaming"
			local = home + "/AppData/Local"
		}
		rules := DefaultRules(platform, home, roaming, local, map[string]string{"AWS_SHARED_CREDENTIALS_FILE": "/custom/aws", "NPM_CONFIG_USERCONFIG": "/custom/npm", "SSH_AUTH_SOCK": "/custom/socket"})
		seen := map[string]bool{}
		for _, r := range rules {
			if seen[r.ID] || r.Kind != "default" || !r.Enabled || len(r.Actions) != 1 || r.Actions[0] != "disable" || r.Reason == "" {
				t.Fatalf("invalid default %s/%s", platform, r.ID)
			}
			seen[r.ID] = true
			if r.ID != "other-workspaces" && r.ID != "password-overrides" && len(r.ResolvedPaths) == 0 {
				t.Fatalf("unresolved default %s/%s", platform, r.ID)
			}
			if r.ID == "ssh" && !strings.Contains(strings.Join(r.ResolvedPaths, ";"), "/custom/socket") {
				t.Fatal("agent endpoint override missing")
			}
		}
		if len(seen) != 20 {
			t.Fatalf("default fixture lost entries: %s %d", platform, len(seen))
		}
	}
}

func TestEveryNativeDefaultDeniesItsResolvedPath(t *testing.T) {
	home := t.TempDir()
	rules := DefaultRules(runtime.GOOS, home, filepath.Join(home, "roaming"), filepath.Join(home, "local"), nil)
	scope := Scope{Layout: layout.Layout{State: filepath.Join(home, "state")}}
	policy := Policy{Rules: rules}
	for _, r := range rules {
		if r.ID == "other-workspaces" {
			continue
		}
		for _, root := range r.ResolvedPaths {
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "synthetic.txt")
			if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
				t.Fatal(err)
			}
			check := evaluate(path, scope, policy)
			found := false
			for _, id := range check.MatchedRuleIDs {
				if id == r.ID {
					found = true
				}
			}
			if !found {
				t.Fatalf("default %s did not deny its resolved path: %+v", r.ID, check)
			}
			if runtime.GOOS == "windows" {
				check = evaluate(strings.ToUpper(path), scope, policy)
				if len(check.MatchedRuleIDs) == 0 {
					t.Fatal("case variant bypassed default", r.ID)
				}
			}
		}
	}
}

func TestDefaultResolversIgnoreRelativeXDGPaths(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		home := "/home/person"
		if platform == "windows" {
			home = "C:/Users/person"
		}
		rules := DefaultRules(platform, home, "", "", map[string]string{"XDG_CONFIG_HOME": "relative-config", "XDG_DATA_HOME": "relative-data"})
		for _, rule := range rules {
			for _, path := range rule.ResolvedPaths {
				if strings.Contains(path, "relative-config") || strings.Contains(path, "relative-data") {
					t.Fatalf("relative override: %s %s", platform, path)
				}
			}
		}
	}
}

func TestReadPolicyProtectsCanonicalStateDirectory(t *testing.T) {
	store, _ := testStore(t)
	actual := t.TempDir()
	alias := filepath.Join(t.TempDir(), "state")
	if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(actual, "state.txt")
	if err := os.WriteFile(path, []byte("synthetic protected state"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{Store: store}
	scope := Scope{Chat: "chat", Workspace: "workspace", Layout: layout.Layout{State: alias}}
	if text, err := svc.Read(context.Background(), path, nil, nil, scope); err == nil || text != "" {
		t.Fatal("canonical state escaped protection")
	}
	policy, err := svc.Policy(scope)
	if err != nil || !layout.SamePath(policy.Rules[0].ResolvedPaths[0], actual) {
		t.Fatalf("canonical representation: %+v %v", policy, err)
	}
	log, err := store.Log(Filter{Outcome: "denied"})
	if err != nil || len(log.Rows) != 1 || log.Rows[0].RuleID != "protected-state" {
		t.Fatalf("denied canonical audit: %+v %v", log, err)
	}
}

func TestReadPolicyDirectoryMetadataUsesDenyAndAudit(t *testing.T) {
	store, _ := testStore(t)
	home, workspace, outside := t.TempDir(), t.TempDir(), t.TempDir()
	denied := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(denied, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(denied, "private-name"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "public-name"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{Store: store, Defaults: DefaultRules(runtime.GOOS, home, "", "", nil)}
	scope := Scope{Chat: "chat", Workspace: "workspace", Roots: []string{workspace}, Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}, Source: "list_directory"}
	if entries, err := svc.ReadDirectory(context.Background(), denied, scope); err == nil || len(entries) != 0 {
		t.Fatal("denied directory metadata escaped")
	}
	scope.Source = "find_files"
	if entries, err := svc.ReadDirectory(context.Background(), outside, scope); err != nil || len(entries) != 1 || entries[0].Name() != "public-name" {
		t.Fatalf("outside metadata: %v %v", entries, err)
	}
	log, err := store.Log(Filter{})
	if err != nil || len(log.Rows) != 2 || log.Rows[0].Source != "find_files" || log.Rows[0].Bytes == 0 || log.Rows[1].Outcome != "denied" || log.Rows[1].RuleID != "ssh" {
		t.Fatalf("directory audit: %+v %v", log, err)
	}
}

func TestReadPolicyNamespaceAliasesCannotReadDeniedFiles(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows namespace fixture")
	}
	store, _ := testStore(t)
	home := t.TempDir()
	path := filepath.Join(home, "denied.txt")
	if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{Store: store}
	scope := Scope{Chat: "chat", Workspace: "workspace", Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	if _, err := svc.Add(path, 1, scope); err != nil {
		t.Fatal(err)
	}
	share := `\\localhost\` + string(path[0]) + `$` + path[2:]
	for _, alias := range []string{path, `\\?\` + path, `\\.\` + path, share, strings.ToUpper(path)} {
		if text, err := svc.Read(context.Background(), alias, nil, nil, scope); err == nil || text != "" {
			t.Fatal("namespace escaped custom denial", alias)
		}
	}
	log, err := store.Log(Filter{Outcome: "denied"})
	if err != nil || len(log.Rows) < 2 {
		t.Fatalf("namespace audit: %+v %v", log, err)
	}
	counts, err := store.RefusedCounts()
	if err != nil || counts["chat"] != 5 {
		t.Fatalf("denial count: %v %v", counts, err)
	}
}
