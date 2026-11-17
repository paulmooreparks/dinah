package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func execGit(args ...string) ([]byte, error) { return exec.Command("git", args...).CombinedOutput() }

// The notices kept in the repository are the ones this tool writes today. A
// dependency added, removed or moved to another version changes them, and
// this fails until they are written again.
func TestTheThirdPartyNoticesAreUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip("lists every program's dependencies for every published platform")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	o, err := load()
	if err != nil {
		t.Fatal(err)
	}
	text, err := build(o)
	if err != nil {
		t.Fatal(err)
	}
	if !current(o.Out, text) {
		t.Fatalf("%s is out of date; run go run ./tools/notices and commit the result", o.Out)
	}
	for _, want := range []string{"## Built into the programs", "## Bundled with the page", "The Go runtime and standard library", "github.com/wailsapp/wails/v2"} {
		if !strings.Contains(text, want) {
			t.Errorf("the notices lack %q", want)
		}
	}
}

func TestLicencesAreNamedFromTheirTexts(t *testing.T) {
	for text, want := range map[string]string{
		"Apache License\nVersion 2.0, January 2004": "Apache-2.0",
		"Mozilla Public License Version 2.0":        "MPL-2.0",
		"Permission is hereby granted, free of charge, to any person ... THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY":   "MIT",
		"Permission is hereby granted, free of charge, to any person ... The Software is provided “as is”, without warranty":     "MIT",
		"Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted": "ISC",
		"Permission to use, copy, modify, and distribute this software for any purpose with or without fee is hereby granted":    "ISC",
		"Redistribution and use in source and binary forms ... Neither the name of the copyright holder":                         "BSD-3-Clause",
		"Redistribution and use in source and binary forms, with or without modification, are permitted":                         "BSD-2-Clause",
		"This Font Software is licensed under the SIL Open Font License, Version 1.1.":                                           "OFL-1.1",
		"Some terms nobody has seen before.": "",
	} {
		if got := licenceName(text); got != want {
			t.Errorf("licenceName(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestAComponentIsNamedByItsPlaceAndItsFoldersVersion(t *testing.T) {
	roots := []string{"cmd/dinah-desktop/frontend", "internal/web/static"}
	for dir, want := range map[string][2]string{
		"cmd/dinah-desktop/frontend/vendor/katex-0.19.1": {"vendor/katex", "0.19.1"},
		"cmd/dinah-desktop/frontend/pudl":                {"pudl", ""},
		"internal/web/static/pudl/fonts":                 {"pudl/fonts", ""},
		"cmd/dinah-desktop/frontend/vendor/mermaid":      {"vendor/mermaid", ""},
	} {
		name, version := nameAndVersion(dir, roots)
		if name != want[0] || version != want[1] {
			t.Errorf("nameAndVersion(%q) = %q %q, want %q %q", dir, name, version, want[0], want[1])
		}
	}
}

// A notice's own text cannot end the block it is printed in.
func TestANoticeIsFencedPastItsOwnTildes(t *testing.T) {
	got := fenced("before\n~~~~\nafter")
	if !strings.HasPrefix(got, "~~~~~text\n") || !strings.HasSuffix(got, "\n~~~~~") {
		t.Fatalf("the fence is no longer than the text's own: %q", got)
	}
}

// A vendored folder with no licence file stops the notices being written,
// since it could not be listed and nobody would know.
func TestAVendoredComponentWithoutALicenceIsRefused(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	run := func(args ...string) {
		t.Helper()
		if out, err := execGit(args...); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	write("page/vendor/known-1.2.3/known.js", "x")
	write("page/vendor/known-1.2.3/LICENSE", "Permission is hereby granted, free of charge ... THE SOFTWARE IS PROVIDED \"AS IS\"")
	write("page/vendor/known-1.2.3/NOTICE.txt", "Copied on some day from somewhere.")
	write("page/vendor/single.min.js", "x")
	write("page/vendor/single.LICENSE", "Redistribution and use in source and binary forms")
	run("add", ".")
	page, err := pageComponents([]string{"page"}, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Title != "vendor/known" || page[0].Version != "1.2.3" || page[1].Title != "vendor/single" {
		t.Fatalf("components: %+v", page)
	}
	if len(page[0].Notices) != 1 || page[0].Notices[0].Name != "vendor/known-1.2.3/LICENSE" {
		t.Fatalf("the vendoring record was printed as the component's notice: %+v", page[0].Notices)
	}
	write("page/vendor/bare/bare.js", "x")
	run("add", ".")
	if _, err := pageComponents([]string{"page"}, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "page/vendor/bare") {
		t.Fatalf("a vendored folder with no licence was not refused: %v", err)
	}
}
