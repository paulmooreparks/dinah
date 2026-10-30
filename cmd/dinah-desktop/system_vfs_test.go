package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/workspaces"
)

// /bin and /lib are the folders bin and lib in Dinah's own folder,
// listed at the root, with the standard files installed in them, changed
// like any folder, and hidden from an action confined to a workspace; the
// other reserved names hold nothing.
func TestSystemFolders(t *testing.T) {
	isolateWorkspaceSettings(t)
	v, _ := mountTestVFS(t)
	a := v.service.app
	root, err := v.list("/", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range root.Entries {
		names = append(names, e.Path)
	}
	if !strings.Contains(strings.Join(names, " "), "/bin") || !strings.Contains(strings.Join(names, " "), "/lib") {
		t.Fatalf("root %v", names)
	}
	bin, err := v.list("/bin", 0, 100)
	if err != nil || len(bin.Entries) < 10 {
		t.Fatalf("/bin %+v %v", bin, err)
	}
	read, err := v.read("/bin/seq")
	if err != nil || !strings.HasPrefix(read.Text, "#!/bin/lua") || read.Resource.Ref == nil || read.Resource.Ref.SourceID != "dinah-bin" {
		t.Fatalf("read %+v %v", read.Resource, err)
	}
	if data, err := os.ReadFile(filepath.Join(configDir(), "bin", "seq")); err != nil || string(data) != read.Text {
		t.Fatalf("on disk: %v", err)
	}
	if st, err := v.stat("/lib/lua/json.lua"); err != nil || st.Directory || st.Size == 0 {
		t.Fatalf("stat %+v %v", st, err)
	}
	// A script is opened in the Editor through its reference, and found again
	// from it.
	if item, err := v.statRef(ResourceRef{SourceID: "dinah-lib", Path: "lua/cli.lua"}); err != nil || item.Path != "/lib/lua/cli.lua" {
		t.Fatalf("statRef %+v %v", item, err)
	}
	if !strings.Contains(strings.Join(read.Resource.Actions, " "), "edit") {
		t.Fatalf("a script cannot be edited: %v", read.Resource.Actions)
	}
	if _, err := a.WriteWorkspaceMount("/bin/mine.sh", "echo mine\n", WriteCreate, "", ResourceRef{}); err != nil {
		t.Fatalf("a write to /bin: %v", err)
	}
	if _, err := v.list("/etc", 0, 10); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("/etc: %v", err)
	}
	for _, name := range []string{"bin", "LIB", "etc", "usr", "var", "tmp", "home", "workspaces"} {
		if !ReservedRoot(name) {
			t.Errorf("%s is not reserved", name)
		}
	}
	if ReservedRoot("projects") {
		t.Error("projects is reserved")
	}
	if src, err := personalSource(workspaces.HomeSourceID); err != nil || src.ID != workspaces.HomeSourceID {
		t.Fatalf("home %+v %v", src, err)
	}
	if _, err := personalSource("elsewhere"); err == nil {
		t.Fatal("an unknown source with no workspace was found")
	}
}

// Names starting with a dot are ordinary names in Home, so ~/.profile can be
// written, listed, read and edited, and stay out of reach in a workspace's
// folders, where .devin and .git are kept.
func TestDotfilesInHomeOnly(t *testing.T) {
	isolateWorkspaceSettings(t)
	v, root := mountTestVFS(t)
	a := v.service.app
	home := a.homeMountPath()
	if _, err := a.WriteWorkspaceMount(home+"/.profile", "export GREETING=hi\n", WriteCreate, "", ResourceRef{}); err != nil {
		t.Fatalf("writing ~/.profile: %v", err)
	}
	page, err := v.list(home, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range page.Entries {
		if e.Path == home+"/.profile" {
			found = true
			if !strings.Contains(strings.Join(e.Actions, " "), "edit") {
				t.Errorf("~/.profile cannot be edited: %v", e.Actions)
			}
		}
	}
	if !found {
		t.Fatalf("~/.profile is not listed: %+v", page.Entries)
	}
	if read, err := v.read(home + "/.profile"); err != nil || read.Text != "export GREETING=hi\n" {
		t.Fatalf("reading ~/.profile: %v", err)
	}
	if _, err := v.read(home + "/../x"); err == nil {
		t.Fatal(".. was taken in Home")
	}
	// A workspace's folder keeps its dotfiles out of reach.
	os.MkdirAll(filepath.Join(root, ".devin"), 0o700)
	os.WriteFile(filepath.Join(root, ".devin", "config.json"), []byte("{}"), 0o600)
	if _, err := v.read("/workspaces/first/local/.devin/config.json"); err == nil {
		t.Fatal("a workspace's .devin was read")
	}
	if _, err := a.WriteWorkspaceMount("/workspaces/first/local/.devin/config.json", "{\"x\":1}", WriteReplace, "", ResourceRef{}); err == nil {
		t.Fatal("a workspace's .devin was written")
	}
	listed, _ := v.list("/workspaces/first/local", 0, 100)
	for _, e := range listed.Entries {
		if strings.Contains(e.Path, ".devin") {
			t.Fatalf("a workspace's .devin is listed: %s", e.Path)
		}
	}
}

// A release replaces the standard files nobody changed, keeps the ones
// somebody changed and the ones somebody added, puts back one that was
// deleted, and removes one it no longer has when nobody changed it.
func TestSystemFilesUpgrade(t *testing.T) {
	dir := t.TempDir()
	if kept, err := installSystemFiles(dir); err != nil || len(kept) != 0 {
		t.Fatalf("first install: %v %v", kept, err)
	}
	seq, cal, tac := filepath.Join(dir, "bin", "seq"), filepath.Join(dir, "bin", "cal"), filepath.Join(dir, "bin", "tac")
	standard, _ := os.ReadFile(seq)
	// As an older release left them: seq and an old script unchanged since,
	// cal changed by the person, and a script of their own.
	old := "#!/bin/lua\nprint('old seq')\n"
	os.WriteFile(seq, []byte(old), 0o600)
	os.WriteFile(cal, []byte("-- mine\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "bin", "gone"), []byte("old\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "bin", "own"), []byte("own\n"), 0o600)
	os.Remove(tac)
	manifest, _ := os.ReadFile(filepath.Join(dir, "system-files.json"))
	text := strings.Replace(string(manifest), `"bin/seq": "`+systemHash(standard)+`"`, `"bin/seq": "`+systemHash([]byte(old))+`"`, 1)
	text = strings.Replace(text, `"files": {`, `"files": {"bin/gone": "`+systemHash([]byte("old\n"))+`",`, 1)
	os.WriteFile(filepath.Join(dir, "system-files.json"), []byte(text), 0o600)
	systemInstalls.Lock()
	delete(systemInstalls.done, dir)
	systemInstalls.Unlock()

	kept, err := installSystemFiles(dir)
	if err != nil || strings.Join(kept, ",") != "bin/cal" {
		t.Fatalf("second install: %v %v", kept, err)
	}
	if data, _ := os.ReadFile(seq); string(data) != string(standard) {
		t.Error("an unchanged file was not replaced")
	}
	if data, _ := os.ReadFile(cal); string(data) != "-- mine\n" {
		t.Error("a changed file was replaced")
	}
	if _, err := os.Stat(tac); err != nil {
		t.Error("a deleted standard file was not put back")
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", "gone")); !os.IsNotExist(err) {
		t.Error("a file this release no longer has was kept")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "bin", "own")); string(data) != "own\n" {
		t.Error("a file of the person's own was touched")
	}
}
