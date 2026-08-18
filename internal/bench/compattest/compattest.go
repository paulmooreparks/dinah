// Package compattest exists solely to be shared between the compatibility
// test files in internal/bench and cmd/dinah, the way internal/testenv
// exists solely to be shared between the test files that need it. No
// production file imports it. It reads the compat-fixture manifest, so that
// shape has exactly one definition rather than one per test package that
// can drift apart.
//
// It deliberately imports nothing from internal/bench. internal/bench's own
// compat tests are an internal test file (package bench), so a package this
// package's own bench-facing helper would need to import bench from would
// close an import cycle back through that test binary; each test package
// instead reads an anchor's declared profile through its own few-line
// helper calling bench.ReadText and bench.ParseAnchor directly, which the
// cycle-1 review accepted as a nit precisely because cross-package sharing
// here is awkward for this reason.
package compattest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// manifestName is the file listing each compat fixture with the digest of
// its contents.
const manifestName = "manifest.json"

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
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return FixtureManifest{}, err
	}
	manifest := FixtureManifest{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return FixtureManifest{}, err
	}
	return manifest, nil
}
