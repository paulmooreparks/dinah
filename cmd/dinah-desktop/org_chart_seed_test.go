package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Set DINAH_ORG_CHART_SEED to a fresh directory below TEMP to retain the
// invented state for the hidden runner. The ordinary package run skips export.
func TestOrgChartExportHiddenSeed(t *testing.T) {
	requested := os.Getenv("DINAH_ORG_CHART_SEED")
	if requested == "" {
		t.Skip("no invented-state export requested")
	}
	output, err := filepath.Abs(requested)
	if err != nil {
		t.Fatal(err)
	}
	temp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(temp, parent)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		t.Fatal("seed output must be a fresh child below TEMP")
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("seed output already exists or cannot be checked", err)
	}
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(output, "workspace")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	orgChartForbidProviderStarts(t)
	a, connection := providerTestApp(t)
	seedOrgChartFixture(t, a, workspace)
	if _, err := a.OrgChart(); err != nil {
		t.Fatal(err)
	}
	if len(connection.calls.Snapshot()) != 0 {
		t.Fatal("seed contacted a provider")
	}
	a.busHost().CloseHandle()
	a.queueStore.Close()
	home := filepath.Join(output, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(configDir(), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name, err := filepath.Rel(configDir(), path)
		if err != nil {
			return err
		}
		target := filepath.Join(home, name)
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !entry.Type().IsRegular() {
			return errors.New("seed contains a nonregular state entry")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	}); err != nil {
		t.Fatal(err)
	}
	t.Logf("Invented state home: %s; workspace: %s", home, workspace)
}
