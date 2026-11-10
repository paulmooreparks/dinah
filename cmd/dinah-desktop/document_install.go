package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"studio/internal/programs"
)

// documentInstalled is one stand-alone application as it is installed: its
// name, the program it is installed as, and the kinds of file it offers
// itself for.
type documentInstalled struct {
	Application string
	Name        string
	Program     string
	Extensions  []string
}

// documentInstalls lists what documentInstallOption installs, beside the
// program at exe.
func documentInstalls(exe string) []documentInstalled {
	var out []documentInstalled
	for id, app := range documentApplications {
		out = append(out, documentInstalled{Application: id, Name: app.Name,
			Program:    filepath.Join(filepath.Dir(exe), programs.FileName(documentProgramName(id))),
			Extensions: documentExtensions[id]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// runDocumentInstall installs the stand-alone applications for the person,
// or removes them. The window program runs it on itself, where it is
// installed: each application becomes a copy of this program under a name
// of its own beside it, and that program is made known to the system, for
// the person only, as the system documents it (document_register_*.go). It
// is offered in Open With for its kinds of file and takes none of them for
// itself. Removing takes away what installing added and nothing else.
func runDocumentInstall(remove bool) int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	failed := false
	for _, app := range documentInstalls(exe) {
		if remove {
			if err := unregisterDocumentApplication(app); err != nil {
				fmt.Fprintf(os.Stderr, "%s could not be removed from the system's lists: %v\n", app.Name, err)
				failed = true
			}
			if err := os.Remove(app.Program); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "%s could not be removed: %v\n", app.Program, err)
				failed = true
			}
			fmt.Printf("Removed %s.\n", app.Name)
			continue
		}
		if err := programs.Place(exe, app.Program); err != nil {
			fmt.Fprintf(os.Stderr, "%s could not be installed: %v\n", app.Name, err)
			failed = true
			continue
		}
		note, err := registerDocumentApplication(app)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s is installed as %s, but the system was not told of it: %v\n", app.Name, app.Program, err)
			failed = true
			continue
		}
		fmt.Printf("Installed %s as %s. %s\n", app.Name, app.Program, note)
	}
	if failed {
		return 1
	}
	return 0
}
