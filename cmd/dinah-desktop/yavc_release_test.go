package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The yavc.app content release Dinah vendors (frontend/YAVC.md). Changing
// it means replacing frontend/yavc whole with another reviewed release.
const yavcReleaseDigest = "544ae0314d379d1e48eed05abe3f37c8051b0ef1520e016c21f7d7eb6e90b718"

// yavcReleaseOmitted are the release's files Dinah does not ship: its
// demonstration hosts and their Go module files. Its PUDL is checked
// against Dinah's own copy instead.
var yavcReleaseOmitted = map[string]bool{"go.mod": true, "go.sum": true, "lab": true, "workspace-lab": true}

type yavcAsset struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

func TestTheVendoredYavcReleaseMatchesItsManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("frontend", "yavc", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Kind         string         `json:"kind"`
		Version      int            `json:"version"`
		Digest       string         `json:"digest"`
		Entries      []string       `json:"entries"`
		Requirements map[string]any `json:"requirements"`
		Files        []yavcAsset    `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Kind != "portable-content-release-set" || manifest.Version != 1 || manifest.Digest != yavcReleaseDigest {
		t.Fatalf("the manifest is not the pinned release: %s %d %s", manifest.Kind, manifest.Version, manifest.Digest)
	}
	// The aggregate digest is SHA-256 of the sorted file list as JSON, as
	// yavc.app's cmd/yavc-content writes it.
	sorted := append([]yavcAsset(nil), manifest.Files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	encoded, err := json.Marshal(sorted)
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(encoded); hex.EncodeToString(sum[:]) != manifest.Digest {
		t.Fatal("the file list does not produce the manifest's digest")
	}
	listed := map[string]bool{}
	for _, f := range manifest.Files {
		top, rest, _ := strings.Cut(f.Path, "/")
		var local string
		switch {
		case yavcReleaseOmitted[top]:
			continue
		case top == "pudl":
			local = filepath.Join("frontend", "pudl", filepath.FromSlash(rest))
		default:
			local = filepath.Join("frontend", "yavc", filepath.FromSlash(f.Path))
			listed[f.Path] = true
		}
		got, err := os.ReadFile(local)
		if err != nil {
			t.Errorf("%s: %v", f.Path, err)
			continue
		}
		sum := sha256.Sum256(got)
		if len(got) != f.Bytes || hex.EncodeToString(sum[:]) != f.SHA256 {
			t.Errorf("%s differs from the release", f.Path)
		}
	}
	for _, entry := range manifest.Entries {
		if !listed[entry] {
			t.Errorf("entry %s is not shipped", entry)
		}
	}
	// Nothing is added to the release tree beyond its manifest.
	root := filepath.Join("frontend", "yavc")
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel != "manifest.json" && !listed[rel] {
			t.Errorf("%s is not in the release", path.Clean(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
