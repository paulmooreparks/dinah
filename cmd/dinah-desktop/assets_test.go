package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestReleaseAssetsExcludeLocalFixtures(t *testing.T) {
	err := fs.WalkDir(assets, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(path, "/e2e") || strings.Contains(path, "/node_modules") || strings.HasSuffix(path, ".md") {
			t.Errorf("non-runtime file embedded in release: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"frontend/index.html", "frontend/app.js", "frontend/pudl/pudl.css"} {
		if _, err := fs.Stat(assets, path); err != nil {
			t.Errorf("required runtime asset %s: %v", path, err)
		}
	}
}
