//go:build windows

package main

import (
	"crypto/rand"
	"errors"
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
