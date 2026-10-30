package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAFolderListsItsFoldersBeforeItsFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"README.md", "_drafts", "SplitDocs", "designs", "Annual Report.pdf", "config.json"} {
		if strings.Contains(name, ".") {
			os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644)
		} else {
			os.Mkdir(filepath.Join(dir, name), 0o755)
		}
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	entries, err := sortedWorkspaceDirectory(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := "_drafts,designs,SplitDocs,Annual Report.pdf,config.json,README.md"
	if strings.Join(got, ",") != want {
		t.Fatalf("got  %s\nwant %s", strings.Join(got, ","), want)
	}
}
