package bench

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// ContainerShape names how one workbench directory sits relative to the rule
// that a workbench is the immediately-named child of a .dinah container.
//
// The four values are exhaustive over the directories a sweep can meet, and
// each names a different repair. They travel outward on the report, so they
// are spelled the way the machine surfaces spell every other closed
// vocabulary: lowercase words a translator never touches.
type ContainerShape string

const (
	// ShapeContained is a workbench already sitting where the rule puts one,
	// under a name this build minted. Nothing is done to it.
	ShapeContained ContainerShape = "contained"
	// ShapeLegacy is a workbench inside a container under the 12-hex name
	// every workbench carried before the rule. The repair is a rename.
	ShapeLegacy ContainerShape = "legacy-name"
	// ShapeBare is a workbench.md sitting directly in a project directory,
	// with no container anywhere above it. The repair creates the container
	// and moves the whole tree into it.
	ShapeBare ContainerShape = "bare"
	// ShapeStray is a workbench inside a container under a name a person
	// typed, which is neither width. The repair is the rename ShapeLegacy
	// gets, and the shape is separate because no listing in this format has
	// ever been able to see such a directory, so a report calling it legacy
	// would be telling the reader it had been visible all along.
	ShapeStray ContainerShape = "stray-name"
)

// ContainerCandidate is one workbench directory a container sweep found,
// carrying how it sits rather than what is inside it.
type ContainerCandidate struct {
	// Path is the workbench directory, the one holding workbench.md.
	Path string `json:"path"`
	// Shape is how that directory sits relative to the containment rule.
	Shape ContainerShape `json:"shape"`
}

// containerRename is os.Rename by default. A test overrides it to force the
// cross-device fallback without needing two filesystems on the machine running
// the suite, which is the reason readAnchorContent and statPath are package
// variables in this same package.
var containerRename = os.Rename

// afterContainerCopy runs on the cross-device path once every file has been
// copied and before any of them is read back. It is nil outside the tests,
// where it stands in for a payload that did not survive the copy, so that the
// verification has something wrong to find.
var afterContainerCopy func(source, target string)

// afterContainerVerify runs on the cross-device path once the copy has been
// verified and before the original is removed. It is nil outside the tests,
// where returning an error from it stands in for a crash at that instant, the
// one moment at which this migration leaves two trees on disk.
var afterContainerVerify func(source, target string) error

// ScanContainers reports every workbench directory at or beneath root,
// together with the shape each one sits in.
//
// It runs its own walk rather than reusing Enumerate, and both halves of that
// are deliberate. Enumerate answers through benchIn, which since the
// containment rule no longer returns a bare workbench as found, so the shape
// this migration exists for is invisible to it. And no listing in this format
// has ever seen a container subdirectory whose name is neither width, because
// ListIDs filters on IsID before an anchor is read at all, so a walk built on
// any existing listing cannot find ShapeStray either.
//
// The walk enters .dinah, which every other walk in this package skips,
// because a container's children are exactly what it has come to look at. It
// skips every other dot-prefixed name and every symbolic link, on the same
// reasoning walkableDir states: a dotfile holds user state, and a link can
// point anywhere this sweep is not entitled to reach.
//
// A directory whose anchor exists and will not read fails the whole walk. A
// migration that moves directories cannot proceed over a workbench it could
// not identify, since the file it could not read might be the one anchor
// telling it that two directories are the same workbench.
func ScanContainers(root string) ([]ContainerCandidate, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var found []ContainerCandidate
	if err := scanContainersIn(abs, &found); err != nil {
		return nil, err
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found, nil
}

// scanContainersIn tests one directory and then descends into the entries it
// is entitled to walk.
func scanContainersIn(dir string, found *[]ContainerCandidate) error {
	anchorPath := filepath.Join(dir, WorkbenchAnchor)
	recognition, err := readAnchor(anchorPath)
	if err != nil {
		return contract.Refuse(contract.UnreadableBench, anchorPath)
	}
	if recognition == anchorOurs {
		*found = append(*found, ContainerCandidate{Path: dir, Shape: shapeOf(dir)})
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return contract.Refuse(contract.UnknownRoot, dir)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != UserBaseName {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		if err := scanContainersIn(filepath.Join(dir, name), found); err != nil {
			return err
		}
	}
	return nil
}

// shapeOf classifies one workbench directory by where it sits and what it is
// called, which is the whole of its identity under this rule.
func shapeOf(dir string) ContainerShape {
	if filepath.Base(filepath.Dir(dir)) != UserBaseName {
		return ShapeBare
	}
	name := filepath.Base(dir)
	switch {
	case IsWorkbenchID(name):
		return ShapeContained
	case IsID(name):
		return ShapeLegacy
	default:
		return ShapeStray
	}
}

// DuplicateWorkbenchIDs reports every identifier carried by more than one of
// the candidates, with the paths sharing it, sorted so that two runs over one
// tree report the same thing in the same order.
//
// It is reported and never repaired. A sweep cannot tell a git clone of one
// workbench, where both working trees are meant to keep the identifier, from a
// copy somebody made when they meant to create a second workbench, and the two
// want opposite repairs. Remint is the explicit act that settles it, and it
// names one directory rather than guessing between two.
func DuplicateWorkbenchIDs(candidates []ContainerCandidate) map[string][]string {
	byID := map[string][]string{}
	for _, candidate := range candidates {
		name := filepath.Base(candidate.Path)
		if !IsWorkbenchID(name) {
			continue
		}
		byID[name] = append(byID[name], candidate.Path)
	}
	for id, paths := range byID {
		if len(paths) < 2 {
			delete(byID, id)
			continue
		}
		sort.Strings(paths)
	}
	return byID
}

// MigrateContainer carries one workbench into the shape the containment rule
// fixes, and answers with the directory it now lives in.
//
// Every shape is one of two primitives, or both. Remint-in-place mints a fresh
// identifier and renames the workbench directory to it, and nothing inside the
// workbench is touched, because no card, column, comment or journal line
// repeats the workbench's own identifier: it is the enclosing directory's name
// and it lives nowhere else. Lift-into-container creates the .dinah beside a
// bare workbench and moves the whole tree into it under a fresh identifier.
//
// A workbench already contained under a minted name is returned unchanged,
// which is what makes a second run over one tree a no-op rather than a second
// rename.
func MigrateContainer(path string) (string, error) {
	switch shapeOf(path) {
	case ShapeContained:
		return path, nil
	case ShapeBare:
		return liftIntoContainer(path)
	default:
		return remintInPlace(path)
	}
}

// Remint gives one workbench directory a fresh identifier, which is the repair
// for a workbench whose identifier another directory also claims. It is
// unconditional and it names exactly one path, on the footing Subversion's
// svnadmin setuuid already has: an operator who has read a duplicate report
// knows which copy is the accidental one, and no rule this tool could write
// knows it for him.
//
// A bare workbench is refused rather than quietly lifted, because reminting
// and migrating are different acts and a caller who asked for one did not ask
// for the other.
func Remint(path string) (string, error) {
	if shapeOf(path) == ShapeBare {
		return "", contract.Refuse(contract.NeedsContainerMigration, path)
	}
	return remintInPlace(path)
}

// remintInPlace renames a contained workbench directory to a freshly minted
// identifier and stamps the storage format the containment rule arrived at.
//
// ClaimWorkbenchID is not the primitive here, and the reason is worth stating:
// its mkdir is what makes a claim atomic, and a rename needs a destination that
// does not exist. So the name is minted and tested for absence instead, which
// is a weaker guarantee bought back by the identifier's own width. The
// destination carries 74 random bits, so two sweeps colliding on a name in the
// window between the test and the rename is not a risk anybody has to reason
// about.
func remintInPlace(path string) (string, error) {
	if held := heldLocks(path); len(held) > 0 {
		return "", contract.RefuseWith(contract.Locked, path, map[string]string{"lock": held[0]})
	}
	container := filepath.Dir(path)
	target, err := freshTarget(container)
	if err != nil {
		return "", err
	}
	if err := containerRename(path, target); err != nil {
		return "", err
	}
	if err := stampContainerFormat(target); err != nil {
		return "", err
	}
	return target, nil
}

// liftIntoContainer creates the container beside a bare workbench and moves the
// whole tree into it under a fresh identifier.
//
// This is the one migration that can genuinely fail partway, because it moves
// an unbounded number of files rather than renaming one directory, and it is
// the one that runs against workbenches somebody is using. So the whole move is
// a single os.Rename wherever the source and the container share a filesystem,
// which is the ordinary case and is as close to atomic as a directory move gets
// on either platform. Only a rename refused across devices falls back to
// copying, and that fallback reads every copied file back and compares its
// digest against the original before it deletes anything. A digest that
// disagrees leaves both trees where they are.
//
// A crash between the copy and the delete also leaves both trees, and a later
// run finishes rather than copying again: the container is searched first for a
// workbench whose contents already match this one, and finding one means the
// copy half already happened.
func liftIntoContainer(path string) (string, error) {
	if held := heldLocks(path); len(held) > 0 {
		return "", contract.RefuseWith(contract.Locked, path, map[string]string{"lock": held[0]})
	}
	container := filepath.Join(filepath.Dir(path), UserBaseName)
	resumed, err := completedLift(path, container)
	if err != nil {
		return "", err
	}
	if resumed != "" {
		if err := os.RemoveAll(path); err != nil {
			return "", err
		}
		if err := stampContainerFormat(resumed); err != nil {
			return "", err
		}
		return resumed, nil
	}
	if err := os.MkdirAll(container, 0o755); err != nil {
		return "", err
	}
	target, err := freshTarget(container)
	if err != nil {
		return "", err
	}
	err = containerRename(path, target)
	if err == nil {
		if err := stampContainerFormat(target); err != nil {
			return "", err
		}
		return target, nil
	}
	if !isCrossDevice(err) {
		return "", err
	}
	if err := mirrorTree(path, target); err != nil {
		return "", err
	}
	if afterContainerCopy != nil {
		afterContainerCopy(path, target)
	}
	if err := verifyTree(path, target); err != nil {
		return "", err
	}
	if afterContainerVerify != nil {
		if err := afterContainerVerify(path, target); err != nil {
			return "", err
		}
	}
	if err := os.RemoveAll(path); err != nil {
		return "", err
	}
	if err := stampContainerFormat(target); err != nil {
		return "", err
	}
	return target, nil
}

// completedLift answers whether an earlier interrupted lift already copied this
// workbench into the container, by finding a workbench there whose files are
// byte for byte the ones sitting at the source. It answers with that directory,
// or with the empty string when the container holds no such workbench.
//
// The comparison is over content rather than over any recorded intention,
// because a migration interrupted between two writes has recorded nothing. A
// workbench with the same contents at two paths is the state this migration
// leaves behind and the only state it leaves behind, so recognising it is what
// makes the second run finish the first one rather than start a second copy.
func completedLift(source, container string) (string, error) {
	want, err := treeDigest(source)
	if err != nil {
		return "", err
	}
	for _, id := range ListWorkbenchIDs(container) {
		candidate := filepath.Join(container, id)
		recognition, err := readAnchor(filepath.Join(candidate, WorkbenchAnchor))
		if err != nil || recognition != anchorOurs {
			continue
		}
		got, err := treeDigest(candidate)
		if err != nil {
			return "", err
		}
		if sameDigests(want, got) {
			return candidate, nil
		}
	}
	return "", nil
}

// freshTarget mints an identifier no directory in the container is using and
// answers with the path it names.
func freshTarget(container string) (string, error) {
	for attempt := 0; attempt < 16; attempt++ {
		id, err := NewWorkbenchID()
		if err != nil {
			return "", err
		}
		target := filepath.Join(container, id)
		if !Exists(target) {
			return target, nil
		}
	}
	return "", fmt.Errorf("could not mint a free workbench identifier in %s after 16 attempts", container)
}

// stampContainerFormat records on a migrated workbench's own anchor that it is
// held to the containment rule from now on. It opens the workbench through the
// containment check rather than around it, so a directory this function is
// asked to stamp and cannot open as contained fails loudly instead of being
// written to.
func stampContainerFormat(root string) error {
	opened, err := Open(root)
	if err != nil {
		return err
	}
	if opened.FM.Value("format") == strconv.Itoa(ContainerFormat) {
		return nil
	}
	opened.FM.Set("format", strconv.Itoa(ContainerFormat))
	return opened.Save()
}

// heldLocks names every lock file standing inside a workbench tree. A migration
// renames or moves the directory holding them, and a writer holding one is
// writing into a path that is about to stop existing, so the migration declines
// rather than moving the ground from under it. The answer carries the paths
// themselves so a refusal can name one.
func heldLocks(root string) []string {
	var held []string
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if name == LockName || strings.HasSuffix(name, SiblingSuffix) {
			held = append(held, path)
		}
		return nil
	})
	sort.Strings(held)
	return held
}

// treeDigest maps every regular file in a tree to a digest of its bytes, keyed
// on the path relative to the tree's own root so that two trees at two paths
// compare.
//
// Locks are left out. They are coordination state on one machine rather than
// part of the workbench, the workbench's own .gitignore already says so, and a
// lock acquired and released between two reads would otherwise make one tree
// look unlike itself.
func treeDigest(root string) (map[string]string, error) {
	digests := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if name == LockName || strings.HasSuffix(name, SiblingSuffix) {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		revision, err := Revision(path)
		if err != nil {
			return err
		}
		digests[filepath.ToSlash(relative)] = revision
		return nil
	})
	if err != nil {
		return nil, err
	}
	return digests, nil
}

// sameDigests reports whether two trees hold the same files with the same
// bytes.
func sameDigests(want, got map[string]string) bool {
	if len(want) != len(got) {
		return false
	}
	for name, digest := range want {
		if got[name] != digest {
			return false
		}
	}
	return true
}

// mirrorTree copies every regular file of a tree, and every directory of it, to a
// new root. It is the cross-device half of a lift and nothing else calls it.
func mirrorTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		return copyFile(path, destination)
	})
}

// verifyTree reads back every file a copy wrote and compares it against the
// original, refusing when any of them disagrees. The refusal is what stops the
// caller deleting a source it could not show had arrived, which is the one
// irreversible step in this whole migration.
func verifyTree(source, target string) error {
	want, err := treeDigest(source)
	if err != nil {
		return err
	}
	got, err := treeDigest(target)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if got[name] != want[name] {
			extra := map[string]string{"source": filepath.Join(source, name)}
			return contract.RefuseWith(contract.Unconfirmed, filepath.Join(target, name), extra)
		}
	}
	return nil
}

// isCrossDevice reports whether a rename failed because its two paths sit on
// different filesystems, which is the one failure the lift answers by copying
// instead of by giving up.
//
// The test is on the documented errno rather than on any message text: the
// os package documents that a *LinkError wraps the underlying error, and both
// platforms name this condition with an errno the syscall package exports.
// The tests reach the same branch through containerRename rather than by
// arranging two filesystems, so this function has one production caller and no
// platform-specific fixture behind it.
func isCrossDevice(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errCrossDevice) {
		return true
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, errCrossDevice)
	}
	return false
}
