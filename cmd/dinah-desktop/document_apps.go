package main

import (
	"path/filepath"

	"studio/internal/docservice"
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

// documentBrand is the name the applications share. Paul chose "YAVC" for
// now and is open to another (11 November 2026), so the applications' names
// are made here and nowhere else.
const documentBrand = "YAVC"

// documentApplications are the applications a window can be started as, by
// the word that follows documentOption. The Editor is the one proved so
// far; Docs, Slides and Diagrams join it here when they are.
var documentApplications = map[string]docservice.Application{
	"editor": {Applet: "editor", Name: documentBrand + " Editor"},
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
