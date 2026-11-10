package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"studio/internal/docservice"
)

// runDocumentService is the document service's process: the one program the
// stand-alone applications' windows share (internal/docservice). The first
// window starts it, later windows find it by the file it writes in its
// folder, and it ends a little after the last window has closed. It has no
// window of its own.
//
// With --headless it starts no window processes and stays until it is
// stopped, for a test that drives the page in a browser of its own: the
// test asks the control interface for a document and opens the address it
// is given, reads the windows the service would have started, and says what
// the system's dialogs will choose. With --dir the service's folder is the
// one given, and nothing of the person's is read.
func runDocumentService(args []string) int {
	flags := flag.NewFlagSet("document-service", flag.ContinueOnError)
	headless := flags.Bool("headless", false, "start no windows and stay until stopped")
	dir := flags.String("dir", "", "the service's folder")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, product.WindowProgram, documentServiceOption, "takes --headless and --dir FOLDER.")
		return 2
	}
	if *dir == "" {
		if err := initializeStateDirectory(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		*dir = documentsDir()
	}
	frontend, err := productFrontend()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if data, ok := productThemeData(); ok {
		themeBuiltinsSource = data
	}
	own := *dir != documentsDir()
	service, err := docservice.New(docservice.Options{
		Dir: *dir, Version: appVersion, Files: http.FileServerFS(frontend), Page: documentPagePath, Cookie: documentCookie,
		Policy: shellPolicy, Policies: map[string]string{d2WorkerPath: d2WorkerPolicy},
		Applications: documentApplications, Headless: *headless, Idle: 20 * time.Second, Product: productInfo(),
		Start: startDocumentWindow,
		Appearance: func() (any, error) {
			// A service on a folder of its own, as under test, reads
			// nothing of the person's, their appearance included.
			if own {
				return nil, errors.New("this service keeps no appearance")
			}
			return readAppearance()
		},
		Log: func(format string, args ...any) { logf("documents", "info", format, args...) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		logf("documents", "error", "the document service could not start: %v", err)
		return 1
	}
	if err := service.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logf("documents", "error", "the document service ended: %v", err)
		return 1
	}
	return 0
}

// startDocumentWindow starts a window process for the service: a window one
// of its pages, or another program, asked for. The window is a process of
// its own that outlives whoever started it.
func startDocumentWindow(application, path, ticket string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{documentOption, application}
	if ticket != "" {
		args = append(args, "--ticket", ticket)
	}
	if path != "" {
		args = append(args, path)
	}
	return startDetached(exe, args...)
}

// startDocumentService starts the service for a window that found none.
func startDocumentService() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return startDetached(exe, documentServiceOption)
}
