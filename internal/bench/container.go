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

// benchMembers is every top-level name a workbench owns inside the directory
// it sits in: its anchor, its journal, and the collections the format's
// grammar declares. PreVocabularyDir is here beside ColumnsDir because a
// workbench that never crossed the column rename still carries its columns
// under the older name, and a migration that left them behind would strand
// every column of it.
//
// The list is what a lift moves, and everything not on it stays where it
// stands. That matters because the format explicitly allows these members to
// sit directly in a project repository, so the directory holding them usually
// holds source code, a readme and a git directory as well, none of which is
// the workbench's to carry anywhere.
//
// A .gitignore or a .gitattributes in that directory is deliberately not a
// member. It is git's configuration for the directory git sees, and this
// migration cannot tell a copy Dinah wrote from one the project wrote. Leaving
// it costs nothing, because the container is created inside that same
// directory, so a pattern written for the workbench still reaches the workbench
// after the move.
var benchMembers = []string{
	WorkbenchAnchor,
	JournalName,
	ColumnsDir,
	PreVocabularyDir,
	CardsDir,
	WorkstreamsDir,
	AttachmentsDir,
	ArchiveDir,
}

// memberPaths answers the members a workbench actually carries, in an order
// that puts the anchor last.
//
// The order is the whole of this migration's crash story on the ordinary path,
// where the members are moved one rename at a time and no filesystem offers to
// move eight names as one act. While the anchor is still at the old path the
// workbench is still found there, still recognised as bare, and still swept, so
// a run that stops halfway is a run the next sweep picks up. Move the anchor
// first and the half-emptied container would read as a finished workbench while
// the collections still at the old path became invisible.
func memberPaths(root string) []string {
	var present []string
	for _, member := range benchMembers {
		if member == WorkbenchAnchor {
			continue
		}
		path := filepath.Join(root, member)
		if Exists(path) {
			present = append(present, path)
		}
	}
	anchor := filepath.Join(root, WorkbenchAnchor)
	if Exists(anchor) {
		present = append(present, anchor)
	}
	return present
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
// containment rule landed no longer returns a bare workbench as found, so the
// shape this migration exists for is invisible to it. And no listing in this format
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
//
// Both minted widths count, and leaving the older one out was a real hole. A
// workbench that has not been carried forward yet still carries an identifier
// this tool minted, so two clones of a repository holding one are two
// directories claiming one identity, and a sweep blind to that gives each of
// them a fresh identifier without saying anything. That is precisely the choice
// this function exists to refuse. A bare workbench and a workbench under a name
// a person typed carry no minted identity at all, so two of them sharing a
// directory name have not claimed anything and are not duplicates.
func DuplicateWorkbenchIDs(candidates []ContainerCandidate) map[string][]string {
	byID := map[string][]string{}
	for _, candidate := range candidates {
		if candidate.Shape != ShapeContained && candidate.Shape != ShapeLegacy {
			continue
		}
		byID[filepath.Base(candidate.Path)] = append(byID[filepath.Base(candidate.Path)], candidate.Path)
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
// A workbench already contained under a minted name keeps the directory it is
// in, and the one thing this repair still owes it is the format stamp. The
// move and the stamp are two writes, so a process that died between them left
// a workbench sitting exactly where the rule puts one and still declaring the
// format that predates the rule, and a sweep that answered here without
// writing would walk past that workbench on every later run. Stamping instead
// costs a read of an anchor that is already open, does nothing when the format
// is already current, and cannot reach a workbench that is legitimately older,
// because nothing else in this format writes a contained workbench under a
// minted name declaring the older format. A second run over a tree already
// carried forward still moves nothing.
//
// A failure after the workbench has moved answers with the directory it moved
// to as well as the error. The move and the format stamp are two writes, and a
// stamp that cannot be written leaves a workbench that really is somewhere
// else, so an answer of nothing would send a reader looking for it where it
// used to be. A caller that reads a non-empty path beside an error is reading
// exactly that: the migration did not finish, and here is where the workbench
// is now.
func MigrateContainer(path string) (string, error) {
	switch shapeOf(path) {
	case ShapeContained:
		return path, stampContainerFormat(path)
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
		return "", contract.Refuse(contract.Locked, lockedEntity(held[0]))
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
		return target, err
	}
	return target, nil
}

// liftIntoContainer creates the container inside the directory a bare
// workbench sits in and moves the workbench's own members into it under a fresh
// identifier.
//
// Where the container goes decides what this repair costs when it is wrong. The
// format allows a workbench to sit directly in a project repository, so the
// directory holding workbench.md commonly holds source code and a git directory
// too. Creating the container beside that directory and renaming the directory
// into it would carry the whole project inside what the tool then calls a
// workbench, would leave the container outside the repository where every
// sibling repository discovers it, and would leave nothing at the path the
// project used to occupy. So the container is created inside that directory and
// the members named in benchMembers move into it, one rename each, with
// everything else left exactly where it stands.
//
// This is the one migration that can genuinely fail partway, and it is the one
// that runs against workbenches somebody is using. Each member moves by
// os.Rename, which is atomic for that member, and the anchor moves last, so an
// interrupted run leaves a workbench still recognised at the old path and a
// container directory carrying members and no anchor. resumableLift is what
// reads that state on the next run and carries on into the same directory
// rather than minting a second one and stranding the members already moved.
//
// Only a rename refused across devices falls back to copying, and that fallback
// reads every copied file back and compares its digest against the original
// before it deletes anything. A digest that disagrees leaves both trees where
// they are. A crash between the copy and the delete also leaves both trees, and
// completedLift recognises that state by content on the next run.
func liftIntoContainer(path string) (string, error) {
	if held := heldLocks(path); len(held) > 0 {
		return "", contract.Refuse(contract.Locked, lockedEntity(held[0]))
	}
	container := filepath.Join(path, UserBaseName)
	resumed, err := completedLift(path, container)
	if err != nil {
		return "", err
	}
	if resumed != "" {
		if err := removeMembers(path); err != nil {
			return "", err
		}
		if err := stampContainerFormat(resumed); err != nil {
			return resumed, err
		}
		return resumed, nil
	}
	if err := os.MkdirAll(container, 0o755); err != nil {
		return "", err
	}
	target, err := resumableLift(container)
	if err != nil {
		return "", err
	}
	if target == "" {
		if target, err = freshTarget(container); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	moved, err := moveMembers(path, target)
	if err != nil {
		return "", err
	}
	if !moved {
		if err := copyMembers(path, target); err != nil {
			return "", err
		}
	}
	if err := stampContainerFormat(target); err != nil {
		return target, err
	}
	return target, nil
}

// moveMembers renames every member of a workbench into the directory it is
// being carried into, and answers whether the renames were performed at all. A
// rename refused because the two paths sit on different filesystems answers
// false with no error, which is the caller's signal to copy instead. Every
// member shares one source directory and one destination directory, so that
// verdict is the same for all of them and the first member's refusal is the
// whole move's refusal, reached with nothing yet moved.
func moveMembers(root, target string) (bool, error) {
	for _, member := range memberPaths(root) {
		destination := filepath.Join(target, filepath.Base(member))
		if err := containerRename(member, destination); err != nil {
			if isCrossDevice(err) {
				return false, nil
			}
			return false, err
		}
	}
	return true, nil
}

// copyMembers is the cross-device half of a lift: it copies every member,
// reads back what it wrote and compares each file against the original, and
// only then removes the originals. A file that did not arrive refuses and
// leaves both trees, because a source it could not show had arrived is a source
// it must not remove.
func copyMembers(root, target string) error {
	for _, member := range memberPaths(root) {
		if err := mirrorTree(member, filepath.Join(target, filepath.Base(member))); err != nil {
			return err
		}
	}
	if afterContainerCopy != nil {
		afterContainerCopy(root, target)
	}
	if err := verifyMembers(root, target); err != nil {
		return err
	}
	if afterContainerVerify != nil {
		if err := afterContainerVerify(root, target); err != nil {
			return err
		}
	}
	return removeMembers(root)
}

// removeMembers deletes the members a workbench owns from the directory it sat
// in, and nothing else in that directory.
func removeMembers(root string) error {
	for _, member := range memberPaths(root) {
		if err := os.RemoveAll(member); err != nil {
			return err
		}
	}
	return nil
}

// resumableLift answers the directory an interrupted lift left behind in this
// container: one carrying members of a workbench and no anchor of its own. It
// answers with the empty string when the container holds no such directory.
//
// Nothing else in this format ever writes such a directory. Every workbench is
// created with its anchor, and this migration moves the anchor last precisely
// so that a stopped move is distinguishable from a finished one. Two of them
// would mean two interrupted lifts into one container, which cannot arise from
// the one bare workbench a directory can hold, so the ambiguity is refused
// rather than guessed at.
func resumableLift(container string) (string, error) {
	entries, err := os.ReadDir(container)
	if err != nil {
		return "", nil
	}
	var partial []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(container, entry.Name())
		if Exists(filepath.Join(candidate, WorkbenchAnchor)) {
			continue
		}
		if !IsWorkbenchID(entry.Name()) {
			continue
		}
		if len(memberPaths(candidate)) == 0 {
			continue
		}
		partial = append(partial, candidate)
	}
	if len(partial) == 0 {
		return "", nil
	}
	if len(partial) > 1 {
		sort.Strings(partial)
		return "", fmt.Errorf("%s holds more than one directory an interrupted lift could have left, %s, and choosing between them is not this repair's to make", container, strings.Join(partial, " and "))
	}
	return partial[0], nil
}

// completedLift answers whether an earlier interrupted lift already copied this
// workbench into the container, by finding a workbench there whose files are
// byte for byte the ones sitting at the source. It answers with that directory,
// or with the empty string when the container holds no such workbench.
//
// The comparison is over content rather than over any recorded intention,
// because a migration interrupted between two writes has recorded nothing. A
// workbench with the same contents at two paths is the state the copying half
// of this migration leaves behind, so recognising it is what makes the second
// run finish the first one rather than start a second copy.
func completedLift(source, container string) (string, error) {
	want, err := memberDigest(source)
	if err != nil {
		return "", err
	}
	for _, id := range ListWorkbenchIDs(container) {
		candidate := filepath.Join(container, id)
		recognition, err := readAnchor(filepath.Join(candidate, WorkbenchAnchor))
		if err != nil || recognition != anchorOurs {
			continue
		}
		got, err := memberDigest(candidate)
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

// heldLocks names every lock file a workbench is carrying. A migration renames
// or moves the paths holding them, and a writer holding one is writing into a
// path that is about to stop existing, so the migration declines rather than
// moving the ground from under it. The answer carries the paths themselves so a
// refusal can name one.
//
// It reads the workbench's own members, plus the files sitting directly in the
// directory the workbench occupies, since the workbench's own lock and the
// sibling lock of a structural act on the workbench both sit there. It does not
// descend into anything the workbench does not own, because a file called lock
// inside a project's source tree is not this tool's business and refusing on it
// would close the migration to a repository that happens to hold one.
//
// This is a check read once, before anything moves, rather than a lock held
// across the whole migration, and the choice is deliberate. A lock in this
// format is a file inside the directory it protects, so both primitives here
// would move the lock they were holding: a remint renames the directory the
// lock file sits in, and a lift empties the directory the lock file was left
// in, so a release would go looking for it at a path that no longer holds a
// workbench. The check is also wider than a hold would be, because Acquire
// takes the workbench's own root lock and this walk refuses on a lock held
// anywhere in the tree, which is where a writer working one card holds one.
//
// What a snapshot leaves open is a writer that starts after the check and
// before the move. The copying path answers that: it digests the source after
// the copy and refuses to delete anything it cannot show arrived, so a write
// that landed late costs the delete rather than the data. The renaming path
// cannot be made to answer it here, because closing that window wants a lock
// the migration can carry across the rename of the directory holding it, and
// no such lock exists in this format. The migration is a repair an operator
// runs deliberately, and it refuses on every lock it can see rather than
// breaking one.
func heldLocks(root string) []string {
	var held []string
	if entries, err := os.ReadDir(root); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				continue
			}
			if name == LockName || strings.HasSuffix(name, SiblingSuffix) {
				held = append(held, filepath.Join(root, name))
			}
		}
	}
	for _, member := range memberPaths(root) {
		filepath.WalkDir(member, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if name == LockName || strings.HasSuffix(name, SiblingSuffix) {
				held = append(held, path)
			}
			return nil
		})
	}
	sort.Strings(held)
	return held
}

// lockedEntity names the directory a lock file stands for, which is what a
// refusal has to say if the reader is to know which of his cards is held and
// what to release. An entity's own lock sits inside the directory it protects
// and a sibling lock stands beside that directory in the same collection, so
// the two spellings answer with a path of the same kind.
func lockedEntity(lock string) string {
	name := filepath.Base(lock)
	if name == LockName {
		return filepath.Dir(lock)
	}
	return filepath.Join(filepath.Dir(lock), strings.TrimSuffix(name, SiblingSuffix))
}

// memberDigest maps every regular file a workbench owns to a digest of its
// bytes, keyed on the path relative to the directory the workbench sits in, so
// that a workbench spread through a project directory compares file for file
// with the same workbench sitting alone inside a container.
//
// It reads the members rather than the whole directory for the reason the lift
// moves the members rather than the whole directory. A digest taken over
// everything at the source would carry a project's source tree into the
// comparison, and the copy this comparison verifies never touched it.
//
// Locks are left out. They are coordination state on one machine rather than
// part of the workbench, the workbench's own .gitignore already says so, and a
// lock acquired and released between two reads would otherwise make one tree
// look unlike itself.
func memberDigest(root string) (map[string]string, error) {
	digests := map[string]string{}
	for _, member := range memberPaths(root) {
		err := filepath.WalkDir(member, func(path string, entry fs.DirEntry, walkErr error) error {
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

// verifyMembers reads back every file a copy wrote and compares it against the
// original, refusing when any of them disagrees. The refusal is what stops the
// caller deleting a source it could not show had arrived, which is the one
// irreversible step in this whole migration.
func verifyMembers(source, target string) error {
	want, err := memberDigest(source)
	if err != nil {
		return err
	}
	got, err := memberDigest(target)
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
