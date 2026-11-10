package main

import (
	"io/fs"
	"path"
	"testing"
	"unicode/utf8"
)

// Every text file of the page is UTF-8, which is what the page is served
// and read as. A file saved in another encoding shows a replacement mark
// where its character was: the launcher's "Open Document…" ended in one
// byte of Windows-1252 and was drawn with that mark (Paul, 11 November
// 2026).
func TestThePagesTextFilesAreUTF8(t *testing.T) {
	text := map[string]bool{".js": true, ".mjs": true, ".css": true, ".html": true, ".json": true, ".svg": true, ".md": true, ".txt": true}
	checked := 0
	err := fs.WalkDir(assets, "frontend", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !text[path.Ext(name)] {
			return err
		}
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			return err
		}
		checked++
		if !utf8.Valid(data) {
			t.Errorf("%s is not UTF-8", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 100 {
		t.Fatalf("only %d text files were found, so the walk missed the page", checked)
	}
}
