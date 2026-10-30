// Package setup installs a content package, a ZIP of skills and of any other
// kind of package the build knows (workspaces.PackageKinds), into a workspace
// or for the person, and updates or removes it again: a fresh install and an
// update are the same operation (docs/design/packages.md). A build that
// registers its own package kinds installs its existing setup bundles unchanged.
//
// Extracting the ZIP over the workspace cannot do two things an update needs.
// It never removes a file that a newer package stopped shipping, so a retired
// skill went on being offered, and it silently overwrites a file the person
// changed. Both need to know what the previous package shipped, so every
// package has a manifest (ManifestPath) listing each file it ships with its
// SHA-256, and the manifest is left in the workspace as the record of what is
// installed.
//
// Apply then works from three facts per path: the hash the previous manifest
// recorded, the hash of the file in the workspace now, and the hash in the new
// manifest. A file whose workspace hash differs from the previous manifest's
// was changed by the person, and it is copied to the backup before it is
// replaced or removed. Every file replaced or removed is backed up, changed or
// not, so an update can always be undone by hand.
//
// The manifest format is a compatibility contract between the tool that
// builds a package and every installed copy of this program, since a person
// may run an older program against a newer package. Fields are only ever
// added, never renamed or given a new meaning, and Schema changes only for a
// change an older reader would misread, which it then refuses.
package setup

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"studio/internal/layout"
	"studio/internal/profile"
)

// ManifestPath is where a package keeps its manifest and where an installed
// package's manifest is kept, relative to the package's root and the
// workspace's: .dinah/setup-manifest.json in Dinah, from the profile.
func ManifestPath() string { return profile.Current().PackageManifest }

// SchemaVersion is the manifest schema this program reads.
const SchemaVersion = 1

// Manifest describes one package.
type Manifest struct {
	Schema  int    `json:"schema"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Built   string `json:"built"`
	// MinTool is the oldest version of the profile's PackageTool that the
	// package's skills work with, as X.Y.Z.
	MinTool string `json:"min_tool"`
	// Roots are the top-level folders and files the package owns. Every file
	// it lists lies under one of them, and Apply never writes or removes
	// anything outside them, which keeps the person's own folders out of
	// reach.
	Roots []string `json:"roots"`
	// Files maps each shipped path, slash-separated and relative to the
	// workspace root, to the SHA-256 of its content in hex. The manifest
	// itself is not listed.
	Files map[string]string `json:"files"`
}

// Kinds of action in a Plan.
const (
	Add       = "add"       // not in the workspace; written
	Unchanged = "unchanged" // already identical; left alone
	Update    = "update"    // differs; backed up and replaced
	Remove    = "remove"    // shipped before, not now; backed up and removed
)

// Action is what Apply does to one path.
type Action struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	// Modified is true when the workspace copy differs from what the previous
	// package installed, so the person changed it. It is false, and Unknown
	// true, when there is no previous manifest to tell.
	Modified bool `json:"modified,omitempty"`
	Unknown  bool `json:"unknown,omitempty"`
}

// Plan is what Apply will do, computed without writing anything.
type Plan struct {
	Workspace string   `json:"workspace"`
	Name      string   `json:"name"`
	From      string   `json:"from"` // installed version, "" when none is recorded
	To        string   `json:"to"`
	MinTool   string   `json:"min_tool"`
	Backup    string   `json:"backup,omitempty"`
	Actions   []Action `json:"actions"`
	Warnings  []string `json:"warnings,omitempty"`

	bundle   *Manifest
	contents map[string][]byte
}

// Counts returns the number of actions of each kind.
func (p *Plan) Counts() map[string]int {
	n := map[string]int{}
	for _, a := range p.Actions {
		n[a.Kind]++
	}
	return n
}

// Installed reads the manifest of the package installed in workspace, or nil
// when there is none.
func Installed(workspace string) (*Manifest, error) {
	b, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(ManifestPath())))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m, err := parseManifest(b)
	if err != nil {
		return nil, fmt.Errorf("the installed manifest %s: %w", ManifestPath(), err)
	}
	return m, nil
}

func parseManifest(b []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}
	if m.Schema != SchemaVersion {
		return nil, fmt.Errorf("manifest schema %d; this program reads schema %d, install a newer %s", m.Schema, SchemaVersion, profile.Current().PackageTool)
	}
	if m.Name == "" || m.Version == "" || len(m.Roots) == 0 || len(m.Files) == 0 {
		return nil, errors.New("manifest lacks name, version, roots or files")
	}
	for _, r := range m.Roots {
		if err := checkRel(strings.TrimSuffix(r, "/")); err != nil {
			return nil, fmt.Errorf("root %q: %w", r, err)
		}
	}
	for p := range m.Files {
		if err := checkRel(p); err != nil {
			return nil, fmt.Errorf("file %q: %w", p, err)
		}
		if !underRoots(p, m.Roots) {
			return nil, fmt.Errorf("file %q lies outside the package's roots", p)
		}
		if p == ManifestPath() {
			return nil, errors.New("the manifest lists itself")
		}
	}
	return &m, nil
}

// checkRel refuses a path that is absolute, escapes the workspace, or is not
// in clean slash form, which is what keeps a hostile or damaged package from
// writing outside the workspace.
func checkRel(p string) error {
	if p == "" || strings.Contains(p, `\`) || strings.Contains(p, ":") || path.IsAbs(p) || path.Clean(p) != p ||
		p == ".." || strings.HasPrefix(p, "../") {
		return errors.New("not a clean relative path")
	}
	return nil
}

func underRoots(p string, roots []string) bool {
	for _, r := range roots {
		if strings.HasSuffix(r, "/") {
			if strings.HasPrefix(p, r) {
				return true
			}
		} else if p == r {
			return true
		}
	}
	return false
}

func digest(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// The most a package may hold: entries, the size of one file, and the size
// of all of them, uncompressed. A typical package is a few megabytes.
const (
	bundleMaxEntries       = 20000
	bundleMaxEntry   int64 = 64 << 20
	bundleMaxTotal   int64 = 256 << 20
)

// readBundle reads a package's ZIP: its manifest and the content of every
// file it lists, checked against the manifest's hashes. A ZIP whose entries
// and manifest disagree in any way is refused, because Apply would otherwise
// record an installation that is not what was written.
func readBundle(zipPath string) (*Manifest, map[string][]byte, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	if len(r.File) > bundleMaxEntries {
		return nil, nil, fmt.Errorf("the package holds %d entries, more than the %d a package may", len(r.File), bundleMaxEntries)
	}
	contents := map[string][]byte{}
	var manifest []byte
	var total int64
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		if err := checkRel(name); err != nil {
			return nil, nil, fmt.Errorf("the package holds an unsafe path %q: %w", name, err)
		}
		// The record of a personal install's copies is this program's own,
		// kept in the package's home; a package that brought one could have
		// files removed anywhere when the next update retires its copies.
		if strings.EqualFold(name, CopiesPath) {
			return nil, nil, fmt.Errorf("the package holds %s, which only %s writes", CopiesPath, profile.Current().Name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		// Each entry and the whole are read up to a limit, so that a package
		// whose files expand without end cannot use up the computer's memory.
		b, err := io.ReadAll(io.LimitReader(rc, bundleMaxEntry+1))
		rc.Close()
		if err != nil {
			return nil, nil, err
		}
		if int64(len(b)) > bundleMaxEntry {
			return nil, nil, fmt.Errorf("%s in the package is larger than the %d MB a file of a package may be", name, bundleMaxEntry>>20)
		}
		if total += int64(len(b)); total > bundleMaxTotal {
			return nil, nil, fmt.Errorf("the package's files come to more than the %d MB a package may hold", bundleMaxTotal>>20)
		}
		if name == ManifestPath() {
			manifest = b
			continue
		}
		contents[name] = b
	}
	if manifest == nil {
		return nil, nil, fmt.Errorf("the ZIP has no %s, so it is not a package %s can install", ManifestPath(), profile.Current().Name)
	}
	m, err := parseManifest(manifest)
	if err != nil {
		return nil, nil, fmt.Errorf("the package's manifest: %w", err)
	}
	for p, want := range m.Files {
		b, ok := contents[p]
		if !ok {
			return nil, nil, fmt.Errorf("the manifest lists %s, which the package does not hold", p)
		}
		if digest(b) != want {
			return nil, nil, fmt.Errorf("%s does not match its hash in the manifest", p)
		}
	}
	for p := range contents {
		if _, ok := m.Files[p]; !ok {
			return nil, nil, fmt.Errorf("the package holds %s, which its manifest does not list", p)
		}
	}
	return m, contents, nil
}

// BundleManifest reads and checks the manifest of the package at zipPath.
func BundleManifest(zipPath string) (*Manifest, error) {
	m, _, err := readBundle(zipPath)
	return m, err
}

// PlanApply works out what applying the package at zipPath to workspace
// would do, writing nothing. toolVersion is this program's version; a
// package needing a newer one is refused.
func PlanApply(zipPath, workspace, toolVersion string) (*Plan, error) {
	fi, err := os.Stat(workspace)
	if err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("the workspace %s is not a folder", workspace)
	}
	m, contents, err := readBundle(zipPath)
	if err != nil {
		return nil, err
	}
	plan := &Plan{Workspace: workspace, Name: m.Name, To: m.Version, MinTool: m.MinTool, bundle: m, contents: contents}
	if m.MinTool != "" {
		tool := profile.Current().PackageTool
		ok, known := versionAtLeast(toolVersion, m.MinTool)
		if known && !ok {
			return nil, fmt.Errorf("the package needs %s %s or later, and this is %s; install the newer version first", tool, m.MinTool, toolVersion)
		}
		if !known {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("this %s reports version %q, which cannot be compared with the %s the package needs", tool, toolVersion, m.MinTool))
		}
	}
	old, err := Installed(workspace)
	if err != nil {
		return nil, err
	}
	if old != nil {
		if old.Name != m.Name {
			return nil, fmt.Errorf("the workspace holds %s, and this package is %s", old.Name, m.Name)
		}
		plan.From = old.Version
	} else if root := existingRoot(workspace, m.Roots); root != "" {
		plan.Warnings = append(plan.Warnings, "the workspace has "+root+" but no record of an installed package, so changes of your own cannot be told apart from older package files, and files earlier versions shipped but this one does not cannot be found and are left in place; every file replaced is backed up")
	}

	paths := make([]string, 0, len(m.Files))
	for p := range m.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		cur, exists, err := fileHash(workspace, p)
		if err != nil {
			return nil, err
		}
		switch {
		case !exists:
			plan.Actions = append(plan.Actions, Action{Path: p, Kind: Add})
		case cur == m.Files[p]:
			plan.Actions = append(plan.Actions, Action{Path: p, Kind: Unchanged})
		default:
			a := Action{Path: p, Kind: Update}
			if old == nil {
				a.Unknown = true
			} else if was, ok := old.Files[p]; !ok || was != cur {
				a.Modified = true
			}
			plan.Actions = append(plan.Actions, a)
		}
	}
	if old != nil {
		var retired []string
		for p := range old.Files {
			if _, still := m.Files[p]; !still {
				retired = append(retired, p)
			}
		}
		sort.Strings(retired)
		for _, p := range retired {
			if !underRoots(p, m.Roots) && !underRoots(p, old.Roots) {
				continue
			}
			cur, exists, err := fileHash(workspace, p)
			if err != nil {
				return nil, err
			}
			if !exists {
				continue
			}
			plan.Actions = append(plan.Actions, Action{Path: p, Kind: Remove, Modified: cur != old.Files[p]})
		}
	}
	return plan, nil
}

// existingRoot returns the first of a package's roots that is already in the
// workspace, as people write it, or "" when none is.
func existingRoot(workspace string, roots []string) string {
	for _, r := range roots {
		if _, err := os.Lstat(filepath.Join(workspace, filepath.FromSlash(strings.TrimSuffix(r, "/")))); err == nil {
			return r
		}
	}
	return ""
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func fileHash(workspace, rel string) (string, bool, error) {
	b, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(rel)))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return digest(b), true, nil
}

// DefaultBackup is the folder Apply backs up into when the caller gives
// none: under the person's home folder, outside every workspace, named for
// the workspace, the installed version and the time.
func DefaultBackup(workspace, from string, now time.Time) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	v := from
	if v == "" {
		v = "unrecorded"
	}
	return filepath.Join(home, "setup-backups", filepath.Base(filepath.Clean(workspace))+"-v"+v+"-"+now.Format("2006-01-02-150405")), nil
}

// Apply carries out a plan, backing up into backup first. The backup must lie
// outside the workspace: a copy of the package inside it would put a second
// set of skills where the agents look for them.
//
// The files are backed up, then written or removed, and the manifest is
// written last, so a run that fails part-way leaves the previous manifest in
// place and can simply be run again. Each file is written to a temporary file
// in its folder and renamed into place.
func Apply(plan *Plan, backup string) error {
	ws, err := filepath.Abs(plan.Workspace)
	if err != nil {
		return err
	}
	bk, err := filepath.Abs(backup)
	if err != nil {
		return err
	}
	if layout.Inside(ws, bk) {
		return fmt.Errorf("the backup folder %s is inside the workspace; choose one outside it", backup)
	}
	plan.Backup = bk
	for _, a := range plan.Actions {
		if a.Kind != Update && a.Kind != Remove {
			continue
		}
		b, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(a.Path)))
		if err != nil {
			return fmt.Errorf("backing up %s: %w", a.Path, err)
		}
		if err := writeFile(filepath.Join(bk, filepath.FromSlash(a.Path)), b); err != nil {
			return fmt.Errorf("backing up %s: %w", a.Path, err)
		}
	}
	for _, a := range plan.Actions {
		target := filepath.Join(ws, filepath.FromSlash(a.Path))
		switch a.Kind {
		case Add, Update:
			if err := writeFile(target, plan.contents[a.Path]); err != nil {
				return fmt.Errorf("writing %s: %w (the files replaced so far are backed up in %s)", a.Path, err, bk)
			}
		case Remove:
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("removing %s: %w (backed up in %s)", a.Path, err, bk)
			}
			pruneEmpty(ws, filepath.Dir(target))
		}
	}
	m, err := json.MarshalIndent(plan.bundle, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(ws, filepath.FromSlash(ManifestPath())), append(m, '\n'))
}

// PlanRemove works out what removing the package installed in workspace
// would do: remove every file its manifest records that is still there,
// noting those changed since, writing nothing. With nothing installed it is
// refused.
func PlanRemove(workspace string) (*Plan, error) {
	m, err := Installed(workspace)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("no package is installed in %s", workspace)
	}
	plan := &Plan{Workspace: workspace, Name: m.Name, From: m.Version, bundle: m}
	paths := make([]string, 0, len(m.Files))
	for p := range m.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if !underRoots(p, m.Roots) {
			continue
		}
		cur, exists, err := fileHash(workspace, p)
		if err != nil {
			return nil, err
		}
		if exists {
			plan.Actions = append(plan.Actions, Action{Path: p, Kind: Remove, Modified: cur != m.Files[p]})
		}
	}
	return plan, nil
}

// Uninstall carries out a removal plan: every file is backed up, then
// removed, and the manifest last, so a removal that fails part-way can be
// run again.
func Uninstall(plan *Plan, backup string) error {
	if plan.To != "" {
		return errors.New("that is not a plan to remove a package")
	}
	ws, err := filepath.Abs(plan.Workspace)
	if err != nil {
		return err
	}
	bk, err := filepath.Abs(backup)
	if err != nil {
		return err
	}
	if layout.Inside(ws, bk) {
		return fmt.Errorf("the backup folder %s is inside the workspace; choose one outside it", backup)
	}
	plan.Backup = bk
	for _, a := range plan.Actions {
		b, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(a.Path)))
		if err != nil {
			return fmt.Errorf("backing up %s: %w", a.Path, err)
		}
		if err := writeFile(filepath.Join(bk, filepath.FromSlash(a.Path)), b); err != nil {
			return fmt.Errorf("backing up %s: %w", a.Path, err)
		}
	}
	for _, a := range plan.Actions {
		target := filepath.Join(ws, filepath.FromSlash(a.Path))
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w (backed up in %s)", a.Path, err, bk)
		}
		pruneEmpty(ws, filepath.Dir(target))
	}
	manifest := filepath.Join(ws, filepath.FromSlash(ManifestPath()))
	if err := os.Remove(manifest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	pruneEmpty(ws, filepath.Dir(manifest))
	return nil
}

// writeFile writes a file whole: to a temporary file in its folder, which is
// then renamed into place, making the folder first.
func writeFile(target string, b []byte) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".package-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

// pruneEmpty removes dir and its empty parents, stopping at the workspace.
func pruneEmpty(ws, dir string) {
	for dir != ws && strings.HasPrefix(dir, ws) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// versionAtLeast compares two X.Y.Z versions. known is false when have is not
// of that form, as a development build's version is not.
func versionAtLeast(have, want string) (ok, known bool) {
	h, hok := parseVersion(have)
	w, wok := parseVersion(want)
	if !hok || !wok {
		return false, false
	}
	for i := range 3 {
		if h[i] != w[i] {
			return h[i] > w[i], true
		}
	}
	return true, true
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
