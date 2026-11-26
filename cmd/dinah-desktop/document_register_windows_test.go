//go:build windows

package main

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// A stand-alone application is registered for the person with a ProgID, the
// program's own name and kinds of file, and a place in each kind's Open
// With list, and it takes no kind for itself. Removing it takes away what
// was added and leaves what was there. The test writes under a key of its
// own, never the real associations.
func TestAStandaloneApplicationIsOfferedInOpenWithAndTakesNoKind(t *testing.T) {
	root := `Software\` + identityWord(product.Name) + `Test-` + rand.Text() + `\Classes`
	documentClassesRoot = root
	t.Cleanup(func() {
		documentClassesRoot = `Software\Classes`
		deleteTree(registry.CURRENT_USER, filepath.Dir(root))
	})
	// Another program is already the default for .md and listed for it.
	if err := setString(registry.CURRENT_USER, root+`\.md`, "", "Other.Markdown"); err != nil {
		t.Fatal(err)
	}
	if err := setString(registry.CURRENT_USER, root+`\.md\OpenWithProgids`, "Other.Markdown", ""); err != nil {
		t.Fatal(err)
	}
	program := `C:\Programs\` + documentProgramName("editor") + `.exe`
	app := documentInstalled{Application: "editor", Name: documentApplications["editor"].Name, Program: program, Extensions: documentExtensions["editor"]}
	if _, err := registerDocumentApplication(app); err != nil {
		t.Fatal(err)
	}
	read := func(path, name string) (string, bool) {
		k, err := registry.OpenKey(registry.CURRENT_USER, root+path, registry.QUERY_VALUE)
		if err != nil {
			return "", false
		}
		defer k.Close()
		v, _, err := k.GetStringValue(name)
		return v, err == nil
	}
	progID := documentProgID(app)
	command := `"` + program + `" "%1"`
	for path, want := range map[string][2]string{
		`\` + progID:                                                      {"", app.Name + " Document"},
		`\` + progID + `\shell\open\command`:                              {"", command},
		`\Applications\` + filepath.Base(program):                         {"FriendlyAppName", app.Name},
		`\Applications\` + filepath.Base(program) + `\shell\open\command`: {"", command},
	} {
		if got, ok := read(path, want[0]); !ok || got != want[1] {
			t.Errorf("%s %q is %q %v, want %q", path, want[0], got, ok, want[1])
		}
	}
	for _, ext := range app.Extensions {
		if _, ok := read(`\`+ext+`\OpenWithProgids`, progID); !ok {
			t.Errorf("%s does not list the application in Open With", ext)
		}
		if _, ok := read(`\Applications\`+filepath.Base(program)+`\SupportedTypes`, ext); !ok {
			t.Errorf("the application does not say it supports %s", ext)
		}
	}
	if v, _ := read(`\.md`, ""); v != "Other.Markdown" {
		t.Errorf(".md's default became %q", v)
	}
	if v, ok := read(`\.txt`, ""); ok && v != "" {
		t.Errorf(".txt was given a default: %q", v)
	}

	if err := unregisterDocumentApplication(app); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{`\` + progID, `\Applications\` + filepath.Base(program)} {
		if k, err := registry.OpenKey(registry.CURRENT_USER, root+path, registry.QUERY_VALUE); !errors.Is(err, registry.ErrNotExist) {
			k.Close()
			t.Errorf("%s is still there after removal: %v", path, err)
		}
	}
	for _, ext := range app.Extensions {
		if _, ok := read(`\`+ext+`\OpenWithProgids`, progID); ok {
			t.Errorf("%s still lists the application", ext)
		}
	}
	if v, _ := read(`\.md`, ""); v != "Other.Markdown" {
		t.Errorf(".md's default became %q after removal", v)
	}
	if _, ok := read(`\.md\OpenWithProgids`, "Other.Markdown"); !ok {
		t.Error("the other program's place in Open With was removed")
	}
	// Removing what is not installed is no error.
	if err := unregisterDocumentApplication(app); err != nil {
		t.Errorf("removing it twice: %v", err)
	}
}

// A product that does not offer the document applications removes the ones
// an earlier installation added beside its program: the program file and
// its registration. A second run finds nothing and says nothing.
func TestAnEarlierInstallationsDocumentApplicationsAreRemoved(t *testing.T) {
	root := `Software\` + identityWord(product.Name) + `Test-` + rand.Text() + `\Classes`
	documentClassesRoot = root
	t.Cleanup(func() {
		documentClassesRoot = `Software\Classes`
		deleteTree(registry.CURRENT_USER, filepath.Dir(root))
	})
	dir := t.TempDir()
	exe := filepath.Join(dir, product.WindowProgram+".exe")
	apps := documentInstalls(exe)
	if len(apps) == 0 {
		t.Fatal("no document applications to install")
	}
	for _, app := range apps {
		if err := os.WriteFile(app.Program, []byte("program"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := registerDocumentApplication(app); err != nil {
			t.Fatal(err)
		}
	}
	if !removeEarlierDocumentApplications(exe) {
		t.Fatal("the removal failed")
	}
	for _, app := range apps {
		if _, err := os.Stat(app.Program); !os.IsNotExist(err) {
			t.Errorf("%s is still there: %v", app.Program, err)
		}
		if k, err := registry.OpenKey(registry.CURRENT_USER, root+`\`+documentProgID(app), registry.QUERY_VALUE); !errors.Is(err, registry.ErrNotExist) {
			k.Close()
			t.Errorf("%s is still registered: %v", app.Name, err)
		}
	}
	if !removeEarlierDocumentApplications(exe) {
		t.Error("removing nothing failed")
	}
}

// The target read from a shortcut must identify the program it was written
// for. Windows can return a long path when the input used an 8.3 alias.
func TestAShortcutsTargetIsReadBack(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "program.exe")
	if err := os.WriteFile(program, []byte("program"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "Program.lnk")
	if err := writeShortcut(link, "Program", program); err != nil {
		t.Fatal(err)
	}
	got, err := shortcutTarget(link)
	if err != nil {
		t.Fatal(err)
	}
	wantInfo, err := os.Stat(program)
	if err != nil {
		t.Fatal(err)
	}
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Errorf("the shortcut starts %q, want %q", got, program)
	}
}

func TestAShortcutCanBeRetargeted(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "old.exe"), filepath.Join(dir, "new.exe")
	for _, path := range []string{from, to} {
		if err := os.WriteFile(path, []byte("program"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "Program.lnk")
	if err := writeShortcut(link, "Program", from); err != nil {
		t.Fatalf("write shortcut: %v", err)
	}
	changed, err := retarget(link, from, to)
	if err != nil || !changed {
		t.Fatalf("retarget: changed=%v, error=%v", changed, err)
	}
	got, err := shortcutTarget(link)
	if err != nil {
		t.Fatalf("read shortcut: %v", err)
	}
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	wantInfo, err := os.Stat(to)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("shortcut starts %q, want %q", got, to)
	}
	changed, err = retarget(link, from, to)
	if err != nil || changed {
		t.Fatalf("unrelated target changed: %v, %v", changed, err)
	}
}

// Started under an application's installed name, the program is that
// application; under the window program's own name it is not.
func TestAnInstalledNameIsItsApplication(t *testing.T) {
	name := documentProgramName("editor")
	for arg0, want := range map[string]string{
		`C:\bin\` + name + `.exe`: "editor",
		name:                      "editor",
		`C:\bin\` + product.WindowProgram + `.exe`: "",
		`C:\bin\editor.exe`:                        "",
	} {
		got, ok := documentProgramApplication(arg0)
		if got != want || ok != (want != "") {
			t.Errorf("%s is %q %v, want %q", arg0, got, ok, want)
		}
	}
}
