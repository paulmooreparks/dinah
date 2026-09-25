package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// licenceFingerprints are the phrases each declared licence's text carries,
// all of which a module's licence file must hold.
var licenceFingerprints = map[string][]string{
	"MIT":          {"Permission is hereby granted, free of charge"},
	"BSD-3-Clause": {"Redistribution and use in source and binary forms", "Neither the name"},
	"Apache-2.0":   {"Apache License", "Version 2.0"},
}

// TestEveryLinkedModuleIsLicensedAsDeclared is dinah-603/criteria/5. It reads
// cmd/dinah/testdata/licenses.txt, one row per module as its path, version and
// licence, and asks go list which modules the binary links for windows, linux
// and darwin. The union of those modules, Dinah's own excluded, must be
// exactly the rows, and each module's directory must hold a file whose name
// begins LICENSE, LICENCE or COPYING carrying the declared licence's
// fingerprint.
func TestEveryLinkedModuleIsLicensedAsDeclared(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH, so the linked modules cannot be listed")
	}
	data, err := os.ReadFile(filepath.Join("testdata", "licenses.txt"))
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string][2]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("the row %q is not a module, a version and a licence", line)
		}
		if _, known := licenceFingerprints[fields[2]]; !known {
			t.Errorf("%s declares the licence %s, which this test has no fingerprint for", fields[0], fields[2])
		}
		declared[fields[0]] = [2]string{fields[1], fields[2]}
	}
	linked := map[string][2]string{}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		list := exec.Command(gobin, "list", "-deps", "-f", "{{with .Module}}{{.Path}} {{.Version}} {{.Dir}}{{end}}", ".")
		list.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64")
		out, err := list.Output()
		if err != nil {
			t.Fatalf("go list for %s: %v", goos, err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fields := strings.SplitN(line, " ", 3)
			if len(fields) != 3 || fields[0] == "dinah" {
				continue
			}
			linked[fields[0]] = [2]string{fields[1], fields[2]}
		}
	}
	t.Logf("%d rows declared, %d modules linked across windows, linux and darwin", len(declared), len(linked))
	var paths []string
	for path := range linked {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		module := linked[path]
		row, ok := declared[path]
		if !ok {
			t.Errorf("the binary links %s %s, and licenses.txt declares no licence for it", path, module[0])
			continue
		}
		if row[0] != module[0] {
			t.Errorf("licenses.txt declares %s at %s, and the binary links %s", path, row[0], module[0])
		}
		file := licenceFile(module[1])
		if file == "" {
			t.Errorf("%s carries no file named LICENSE, LICENCE or COPYING", path)
			continue
		}
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range licenceFingerprints[row[1]] {
			if !strings.Contains(string(text), phrase) {
				t.Errorf("%s is declared %s, and its %s lacks %q", path, row[1], filepath.Base(file), phrase)
			}
		}
	}
	for path := range declared {
		if _, ok := linked[path]; !ok {
			t.Errorf("licenses.txt declares %s, which the binary no longer links", path)
		}
	}
	if len(linked) == 0 || len(declared) == 0 {
		t.Fatal("one side of the comparison is empty, so it proves nothing")
	}
}

// licenceFile finds the licence file in a module's directory.
func licenceFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		name := strings.ToUpper(entry.Name())
		if !entry.IsDir() && (strings.HasPrefix(name, "LICENSE") || strings.HasPrefix(name, "LICENCE") || strings.HasPrefix(name, "COPYING")) {
			return filepath.Join(dir, entry.Name())
		}
	}
	return ""
}
