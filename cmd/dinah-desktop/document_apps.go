package main

import (
	"os"
	"path/filepath"
	"strings"

	"studio/internal/docservice"
	"studio/internal/profile"
	"studio/internal/programs"
)

// The stand-alone document applications
// (docs/proposals/standalone-document-apps.md, and
// docs/design/standalone-documents.md). An application shows one document in
// one window. Each window is this program started with documentOption
// (document_window.go), and what the windows share is one more copy of it
// started with documentServiceOption (document_service.go).

// documentOption starts a document's window:
//
//	--document APPLICATION [--ticket TICKET] [FILE]
//
// With no file the window is a blank one. The ticket is the service's own,
// for a window one of its pages asked for.
const documentOption = "--document"

// documentServiceOption starts the document service, which has no window:
//
//	--document-service [--headless] [--dir FOLDER]
const documentServiceOption = "--document-service"

// documentInstallOption installs the applications for the person, and
// documentUninstallOption removes them again (document_install.go).
const (
	documentInstallOption   = "--document-install"
	documentUninstallOption = "--document-uninstall"
)

// documentBrand is the name the applications share. Paul chose "YAVC" for
// now and is open to another (11 November 2026), so the applications' names
// are made here and nowhere else.
const documentBrand = "YAVC"

// documentApplications are the applications a window can be started as, by
// the word that follows documentOption. The Editor is the one proved so
// far; Docs, Slides and Diagrams join it here when they are.
var documentApplications = map[string]docservice.Application{
	"editor": {Applet: "editor", Name: documentBrand + " Editor", FileApplets: map[string]string{".md": "docs", ".markdown": "docs"}},
}

// documentExtensions are the kinds of file the Editor offers itself for in
// the system's Open With. It takes no kind of file for itself: the person
// chooses what opens what.
var documentExtensions = map[string][]string{
	"editor": {".md", ".markdown", ".txt"},
}

// documentProgramName is the file name, without an extension, of the
// program an application is installed as, beside the window program: the
// same program under a name of its own, so that the system has a program to
// show by that name, to pin and to offer in Open With. Started under that
// name it is the application's window, with what follows as the file. The
// source product's are named for the brand, and another product's for its
// own command, so that two products on one computer keep their own.
func documentProgramName(application string) string {
	prefix := strings.ToLower(documentBrand)
	if profile.Current().ID != profile.Source().ID {
		prefix = product.CommandProgram
	}
	return prefix + "-" + application
}

// documentProgramApplication is the application a program's file name
// stands for, when it is one of the installed names.
func documentProgramApplication(arg0 string) (string, bool) {
	name := strings.ToLower(filepath.Base(arg0))
	name = strings.TrimSuffix(name, ".exe")
	for application := range documentApplications {
		if name == documentProgramName(application) {
			return application, true
		}
	}
	return "", false
}

// documentExecutable is the program a window of an application is started
// from: the application's own installed name when it is there beside this
// program, so that every window of the application is the same program to
// the system, and otherwise this program itself.
func documentExecutable(application string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	named := filepath.Join(filepath.Dir(exe), programs.FileName(documentProgramName(application)))
	if info, err := os.Stat(named); err == nil && !info.IsDir() {
		return named, nil
	}
	return exe, nil
}

// documentsDir is the document service's folder in the product's state
// folder: its announcement, the recovery records and the applications'
// settings.
func documentsDir() string { return filepath.Join(configDir(), "documents") }

// documentCookie signs a document's page in. It is a cookie of its own,
// sent only under that document's path.
var documentCookie = product.ShellCookie + "-document"

// documentPagePath is the stand-alone page among the page's files.
const documentPagePath = "document.html"

// documentEntryVar names a file for a test: a window started with it set
// stays hidden, loads no page, and adds a line to the file with its
// document and the address of its page, which the test opens in a browser
// of its own, as the main window does with its own entry file.
func documentEntryVar() string { return product.EnvPrefix + "DOCUMENT_ENTRY_FILE" }

// documentHiddenVar keeps a window off the screen for a check of the real
// window: with it set the window loads its page as always and is never
// shown, so a check sees the page start in the window's own browser view
// without anything appearing in front of the person.
func documentHiddenVar() string { return product.EnvPrefix + "DOCUMENT_HIDDEN" }
