package workspaces

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"studio/internal/agents"
	"studio/internal/layout"
)

// A package collection is a folder of packages of one kind: skills, and any
// kind a build registers, such as document types or flows. A workspace's
// managed folder and the PackageFolder of its main folder are searched for
// every kind, and each collection found becomes a source of the workspace (a
// VFS mount) whose ID begins with the kind's key, so applets find their
// packages through the VFS without knowing where collections live.
//
// The same kinds are what a content package may contain (internal/setup,
// docs/design/packages.md). A package keeps the items of a kind in
// PackageFolder/<key>/, and an install for the person copies them to the
// kind's personal folder, so that every workspace has them.
type PackageKind struct {
	// Key is the folder name and the prefix of the collection's source ID.
	// RegisterPackageKind sets it.
	Key string
	// Label is how people see the kind, as in "Workspace Skills".
	Label string
	// Personal returns the folder an install for the person copies the
	// kind's items to. state is the state folder in use, and isolated says
	// it was given for a test or a trial run, in which case the folder must
	// be inside it, so that the person's own folder is never touched. Nil
	// leaves the kind's items in the package's own folder.
	Personal func(state string, isolated bool) (string, error)
	// FileItems says that each file in the kind's folder is one item, as a
	// flow is. Otherwise each folder in it is one item, as a skill is. A
	// personal install refuses to overwrite an item it did not write.
	FileItems bool
	// Rewrite changes the text of a file copied for the person, after the
	// package's paths in it are made absolute. items are the kind's items in
	// the package, by their names in its folder. Nil changes nothing.
	Rewrite func(text string, items []string) string
}

// PackageFolder is the folder of a workspace's main folder, and of a
// package, that holds a folder for each kind of package. It is the folder
// the agents read skills from.
const PackageFolder = ".devin"

// Skills is the kind every build knows.
const Skills = "skills"

var (
	packageKindsMu sync.RWMutex
	packageKinds   = []PackageKind{{Key: Skills, Label: "Skills", Personal: PersonalSkillsFolder}}
)

// packageKeyPattern is what a package kind's key may look like, which is
// also a folder's name.
var packageKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// RegisterPackageKind registers a kind of package collection and content,
// in the way RegisterSourceKind registers a kind of source. Registering a
// kind twice, or registering skills, which is built in, panics.
func RegisterPackageKind(key string, k PackageKind) {
	packageKindsMu.Lock()
	defer packageKindsMu.Unlock()
	if !packageKeyPattern.MatchString(key) {
		panic("workspaces: cannot register the package kind " + key)
	}
	for _, kind := range packageKinds {
		if kind.Key == key {
			panic("workspaces: the package kind " + key + " is registered twice")
		}
	}
	k.Key = key
	if k.Label == "" {
		k.Label = key
	}
	packageKinds = append(packageKinds, k)
}

// PackageKinds returns the kinds of package this build knows, skills first
// and then in the order they were registered.
func PackageKinds() []PackageKind {
	packageKindsMu.RLock()
	defer packageKindsMu.RUnlock()
	return append([]PackageKind(nil), packageKinds...)
}

// LookupPackageKind returns the kind of package with the key, and whether
// this build knows it.
func LookupPackageKind(key string) (PackageKind, bool) {
	for _, kind := range PackageKinds() {
		if kind.Key == key {
			return kind, true
		}
	}
	return PackageKind{}, false
}

// PersonalSkillsFolder is the person's own skills folder, which the agents
// read in every workspace. For a state folder given for a test or a trial
// run it is the devin-skills folder inside it.
func PersonalSkillsFolder(state string, isolated bool) (string, error) {
	if isolated {
		return filepath.Join(state, "devin-skills"), nil
	}
	return agents.PersonalSkillsDir()
}

// CollectionKind returns the kind of a collection's source ID.
func CollectionKind(sourceID string) (PackageKind, bool) {
	for _, kind := range PackageKinds() {
		if strings.HasPrefix(sourceID, kind.Key+"-") {
			return kind, true
		}
	}
	return PackageKind{}, false
}

// Collections finds a workspace's collections. Their IDs include the owning
// source's identity, so replacing a source cannot redirect a window open on
// one of its packages.
func Collections(l layout.Layout, record Record) []Source {
	var result []Source
	add := func(id, name, path string) {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return
		}
		result = append(result, Source{ID: id, Kind: "local-folder", Name: name, Path: path})
	}
	managed := sha256.Sum256([]byte(record.ID))
	for _, kind := range PackageKinds() {
		add(kind.Key+"-managed", "Workspace "+kind.Label, filepath.Join(l.State, "workspaces", fmt.Sprintf("%x", managed[:16]), kind.Key))
		for _, source := range record.Sources {
			// Only the main folder contributes packages: the agents start
			// there and load skills from its PackageFolder alone, so a package
			// from another folder would run without what it needs.
			if source.Kind != "local-folder" || !layout.SamePath(source.Path, ExecutionPath(record)) {
				continue
			}
			// Reject a redirected provider directory before making a collection of it.
			parent, err := os.Lstat(filepath.Join(source.Path, PackageFolder))
			if err != nil || !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 {
				continue
			}
			digest := sha256.Sum256([]byte(source.ID + "\x00" + filepath.Clean(source.Path)))
			add(fmt.Sprintf("%s-devin-%x", kind.Key, digest[:16]), source.Name+" "+kind.Label, filepath.Join(source.Path, PackageFolder, kind.Key))
		}
	}
	return result
}

// WithCollections adds a workspace's collections to its sources.
func WithCollections(l layout.Layout, record Record) Record {
	used := map[string]bool{}
	for _, source := range record.Sources {
		used[source.MountAlias] = true
	}
	record.Sources = append([]Source{}, record.Sources...)
	for _, source := range Collections(l, record) {
		kind, _ := CollectionKind(source.ID)
		source.MountAlias = MountAlias(source.Name, kind.Label, used)
		record.Sources = append(record.Sources, source)
	}
	return record
}
