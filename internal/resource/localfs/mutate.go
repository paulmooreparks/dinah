package localfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/resource"
)

// The operations a shell needs beyond reading and writing text: making a
// folder, removing, moving and copying. Each stays inside its folder through
// os.Root and refuses symbolic links and hidden or system entries along the
// path, as reading does, and none of them removes, moves or replaces the
// folder's own root. A tree that holds an entry this package cannot show (a
// hidden or system entry, or a symbolic link) is refused as a whole by a
// recursive removal or copy, so nothing the person cannot see is removed or
// copied with it.

// MaxTreeEntries bounds the entries a recursive removal or copy visits.
const MaxTreeEntries = 50000

// ErrExists is returned when the destination of a new folder, a move or a copy
// already exists and may not be replaced.
var ErrExists = errors.New("the destination already exists")

// open opens the folder's root and checks name's path within it, returning
// the root, the cleaned name and the final segment's information, which is
// nil when it does not exist.
func (f Folder) open(ctx context.Context, path string) (*os.Root, string, os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", nil, err
	}
	name, err := f.clean(path)
	if err != nil {
		return nil, "", nil, err
	}
	root, err := os.OpenRoot(f.Root)
	if err != nil {
		return nil, "", nil, errors.New("the local source is unavailable")
	}
	info, err := f.checkAncestors(root, name)
	if err != nil {
		root.Close()
		return nil, "", nil, err
	}
	return root, name, info, nil
}

// existingPart checks the segments of name that exist, as checkAncestors
// does, stopping at the first that does not. It returns the last segment's
// information, or nil when some segment is missing.
func (f Folder) existingPart(root *os.Root, name string) (os.FileInfo, error) {
	part := ""
	var info os.FileInfo
	for _, segment := range strings.Split(name, string(filepath.Separator)) {
		part = filepath.Join(part, segment)
		st, err := root.Lstat(part)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil || st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("the source path is unavailable or is a symbolic link")
		}
		if hidden, err := PathHidden(filepath.Join(f.Root, part)); err != nil || hidden {
			return nil, errors.New("hidden and system paths are unavailable")
		}
		info = st
	}
	return info, nil
}

// MakeFolder makes a folder. With parents, missing folders above it are made
// too and an existing folder is not an error, as mkdir -p does.
func (f Folder) MakeFolder(ctx context.Context, path string, parents bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := f.clean(path)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(f.Root)
	if err != nil {
		return errors.New("the local source is unavailable")
	}
	defer root.Close()
	var info os.FileInfo
	if name != "." {
		if parents {
			info, err = f.existingPart(root, name)
		} else {
			info, err = f.checkAncestors(root, name)
		}
		if err != nil {
			return err
		}
	}
	if name == "." {
		if parents {
			return nil
		}
		return ErrExists
	}
	if info != nil {
		if parents && info.IsDir() {
			return nil
		}
		return ErrExists
	}
	if parents {
		return root.MkdirAll(name, 0o755)
	}
	if parent := filepath.Dir(name); parent != "." {
		if st, err := root.Stat(parent); err != nil || !st.IsDir() {
			return errors.New("the folder above it does not exist")
		}
	}
	return root.Mkdir(name, 0o755)
}

// Remove removes a file, or a folder: an empty one, or with recursive, one
// and everything in it. It returns how many entries it removed.
func (f Folder) Remove(ctx context.Context, path string, recursive bool) (int, error) {
	root, name, info, err := f.open(ctx, path)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	if name == "." {
		return 0, errors.New("a source's own folder cannot be removed")
	}
	if info == nil {
		return 0, os.ErrNotExist
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return 0, errors.New("only files and folders can be removed")
		}
		return 1, root.Remove(name)
	}
	n, err := f.walk(ctx, root, name, nil)
	if err != nil {
		return 0, err
	}
	if n > 1 && !recursive {
		return 0, errors.New("the folder is not empty")
	}
	if err := root.RemoveAll(name); err != nil {
		return 0, fmt.Errorf("the folder could not be removed: %w", err)
	}
	return n, nil
}

// walk visits name and everything under it, calling visit for each entry
// with its path relative to name ("" for name itself), and refuses a tree
// that holds a hidden or system entry, a symbolic link or anything that is
// neither a file nor a folder. It returns the number of entries.
func (f Folder) walk(ctx context.Context, root *os.Root, name string, visit func(rel string, info os.FileInfo) error) (int, error) {
	count := 0
	var step func(rel string) error
	step = func(rel string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		full := filepath.Join(name, rel)
		info, err := root.Lstat(full)
		if err != nil {
			return errors.New("the folder changed while it was being read")
		}
		count++
		if count > MaxTreeEntries {
			return fmt.Errorf("the folder holds more than %d entries", MaxTreeEntries)
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("%s is a link or a special file, which is not shown or changed", filepath.ToSlash(full))
		}
		if rel != "" {
			hidden, err := PathHidden(filepath.Join(f.Root, full))
			if err != nil || hidden || HiddenName(info.Name(), f.Dotfiles) {
				return fmt.Errorf("%s holds hidden or system entries, which are not shown or changed", filepath.ToSlash(name))
			}
		}
		if visit != nil {
			if err := visit(rel, info); err != nil {
				return err
			}
		}
		if !info.IsDir() {
			return nil
		}
		dir, err := root.Open(full)
		if err != nil {
			return errors.New("a folder could not be opened")
		}
		entries, err := dir.ReadDir(-1)
		dir.Close()
		if err != nil {
			return errors.New("a folder could not be read")
		}
		for _, e := range entries {
			if err := step(filepath.Join(rel, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := step(""); err != nil {
		return 0, err
	}
	return count, nil
}

// Move moves or renames a file or folder within the folder. An existing file
// at the destination is replaced by a file, as mv does; an existing folder is
// never replaced.
func (f Folder) Move(ctx context.Context, from, to string) error {
	root, src, info, err := f.open(ctx, from)
	if err != nil {
		return err
	}
	defer root.Close()
	if src == "." {
		return errors.New("a source's own folder cannot be moved")
	}
	if info == nil {
		return os.ErrNotExist
	}
	dst, err := f.clean(to)
	if err != nil {
		return err
	}
	if dst == "." {
		return ErrExists
	}
	if dst == src {
		return nil
	}
	if info.IsDir() && strings.HasPrefix(dst+string(filepath.Separator), src+string(filepath.Separator)) {
		return errors.New("a folder cannot be moved into itself")
	}
	dinfo, err := f.checkAncestors(root, dst)
	if err != nil {
		return err
	}
	if dinfo != nil && (dinfo.IsDir() || info.IsDir()) {
		return ErrExists
	}
	if info.IsDir() {
		if _, err := f.walk(ctx, root, src, nil); err != nil {
			return err
		}
	}
	if parent := filepath.Dir(dst); parent != "." {
		if st, err := root.Stat(parent); err != nil || !st.IsDir() {
			return errors.New("the folder for the destination does not exist")
		}
	}
	return root.Rename(src, dst)
}

// Rename gives a file a new name in its own folder, without replacing
// anything: a name another entry has is refused with resource.ErrConflict.
// A name that differs only in case from the file's own is the same file on
// a folder that ignores case, and is renamed.
func (f Folder) Rename(ctx context.Context, from, to string) error {
	root, src, info, err := f.open(ctx, from)
	if err != nil {
		return err
	}
	defer root.Close()
	if info == nil {
		return resource.ErrNotFound
	}
	if info.IsDir() {
		return errors.New("only a file is renamed here")
	}
	dst, err := f.clean(to)
	if err != nil {
		return err
	}
	if filepath.Dir(dst) != filepath.Dir(src) {
		return errors.New("a renamed file stays in its folder")
	}
	if dst == src {
		return nil
	}
	dinfo, err := f.checkAncestors(root, dst)
	if err != nil {
		return err
	}
	if dinfo != nil && !os.SameFile(info, dinfo) {
		return resource.ErrConflict
	}
	return root.Rename(src, dst)
}

// Copy copies a file, or with recursive a folder and everything in it, from
// one folder to another, which may be the same folder. An existing file at
// the destination is replaced by a file, as cp does; an existing folder is
// never replaced or merged into. It returns how many entries it copied.
func Copy(ctx context.Context, src Folder, from string, dst Folder, to string, recursive bool) (int, error) {
	sroot, sname, info, err := src.open(ctx, from)
	if err != nil {
		return 0, err
	}
	defer sroot.Close()
	if info == nil && sname != "." {
		return 0, os.ErrNotExist
	}
	droot, dname, dinfo, err := dst.open(ctx, to)
	if err != nil {
		return 0, err
	}
	defer droot.Close()
	if dname == "." {
		return 0, ErrExists
	}
	isDir := sname == "." || info.IsDir()
	if isDir && !recursive {
		return 0, errors.New("copying a folder needs -r")
	}
	if dinfo != nil && (dinfo.IsDir() || isDir) {
		return 0, ErrExists
	}
	if parent := filepath.Dir(dname); parent != "." {
		if st, err := droot.Stat(parent); err != nil || !st.IsDir() {
			return 0, errors.New("the folder for the destination does not exist")
		}
	}
	if isDir && filepath.Clean(src.Root) == filepath.Clean(dst.Root) && (sname == "." || strings.HasPrefix(dname+string(filepath.Separator), sname+string(filepath.Separator))) {
		return 0, errors.New("a folder cannot be copied into itself")
	}
	if !isDir {
		return 1, copyFile(sroot, sname, droot, dname, info)
	}
	// The whole tree is checked before anything is written, so a refused
	// copy leaves nothing behind.
	if _, err := src.walk(ctx, sroot, sname, nil); err != nil {
		return 0, err
	}
	return src.walk(ctx, sroot, sname, func(rel string, info os.FileInfo) error {
		target := filepath.Join(dname, rel)
		if info.IsDir() {
			return droot.Mkdir(target, 0o755)
		}
		return copyFile(sroot, filepath.Join(sname, rel), droot, target, info)
	})
}

// copyFile copies one regular file's bytes, through a temporary file renamed
// into place, so the destination holds the old content or the new.
func copyFile(sroot *os.Root, sname string, droot *os.Root, dname string, info os.FileInfo) error {
	in, err := sroot.Open(sname)
	if err != nil {
		return errors.New("the file could not be opened")
	}
	defer in.Close()
	if st, err := in.Stat(); err != nil || !os.SameFile(st, info) {
		return errors.New("the file changed while it was being copied")
	}
	tmp, err := tempName(droot, dname)
	if err != nil {
		return err
	}
	out, err := droot.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("the copy could not be written: %w", err)
	}
	_, cerr := io.Copy(out, in)
	serr := out.Sync()
	xerr := out.Close()
	if err := errors.Join(cerr, serr, xerr); err != nil {
		droot.Remove(tmp)
		return fmt.Errorf("the copy could not be written: %w", err)
	}
	if err := droot.Rename(tmp, dname); err != nil {
		droot.Remove(tmp)
		return fmt.Errorf("the copy could not be put in place: %w", err)
	}
	return nil
}
