package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// compatManifestName is the file listing each compat fixture with the digest
// of its contents.
const compatManifestName = "manifest.json"

// FixtureRow is one row of the compat-fixture manifest.
type FixtureRow struct {
	// Directory is the fixture directory's name under testdata/compat.
	Directory string `json:"directory"`
	// Digest is the SHA-256 of the fixture's files, as the manifest's own
	// digest procedure computes it.
	Digest string `json:"digest"`
	// Sample marks the fixture the shape comparison in cmd/dinah reads.
	// Exactly one fixture declaring a given revision may carry it.
	Sample bool `json:"sample,omitempty"`
}

// FixtureManifest is the compat-fixture manifest file's whole shape.
type FixtureManifest struct {
	// Fixtures are the rows, one per fixture directory.
	Fixtures []FixtureRow `json:"fixtures"`
}

// ReadFixtureManifest reads and parses the compat-fixture manifest under dir.
// internal/bench's own compat tests and cmd/dinah's compat tests both read
// the manifest through this one function, so its shape has exactly one
// definition rather than two that can drift apart.
func ReadFixtureManifest(dir string) (FixtureManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, compatManifestName))
	if err != nil {
		return FixtureManifest{}, err
	}
	manifest := FixtureManifest{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return FixtureManifest{}, err
	}
	return manifest, nil
}

// DeclaredProfile reads the profile string an anchor at root declares,
// without opening the workbench. internal/bench's own compat tests and
// cmd/dinah's compat tests both read it through this one function rather
// than each keeping its own copy.
func DeclaredProfile(root string) (string, error) {
	text, err := ReadText(filepath.Join(root, WorkbenchAnchor))
	if err != nil {
		return "", err
	}
	fm, _ := ParseAnchor(text)
	return fm.Value("profile"), nil
}
