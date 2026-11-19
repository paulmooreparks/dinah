package programs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MoveOut moves a product's programs, and the copies of them that updates
// set aside, from the folder they were installed in before, from, to the
// program folder, to. It is how an installation leaves the home folder's bin
// to the standard scripts (docs/design/installation.md). names are the
// programs, such as "dinah-desktop"; files are other files of an
// installation, such as its notices, which are moved under their own names.
//
// A program found under its own name is moved as a set-aside copy, since
// the program folder has the installed one under that name. A set-aside
// copy keeps its name. Nothing is replaced in to: a name already there gets
// a suffix. A file that cannot be moved, as a running program cannot be when
// to is on another drive, is left where it is and listed in left. Every
// other file in from is left alone.
func MoveOut(from, to string, names, files []string) (moved, left []string, err error) {
	entries, err := os.ReadDir(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if same(from, to) {
		return nil, nil, nil
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return nil, nil, err
	}
	stamp := time.Now().UnixNano()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		target, ok := movedName(e.Name(), names, files, stamp)
		if !ok {
			continue
		}
		src := filepath.Join(from, e.Name())
		dst := free(filepath.Join(to, target))
		if err := moveFile(src, dst); err != nil {
			left = append(left, e.Name())
			continue
		}
		moved = append(moved, e.Name())
	}
	return moved, left, nil
}

// movedName is the name a file of from has in the program folder, and
// whether it is one MoveOut moves.
func movedName(name string, names, files []string, stamp int64) (string, bool) {
	for _, f := range files {
		if strings.EqualFold(name, f) {
			return name, true
		}
	}
	for _, program := range names {
		file := FileName(program)
		switch {
		case strings.EqualFold(name, file):
			return fmt.Sprintf("%s.old-%d-moved", file, stamp), true
		case hasPrefixFold(name, file+".old-"), hasPrefixFold(name, file+".before-"):
			return name, true
		case strings.EqualFold(name, file+".new"):
			// A copy that an installation stopped before it was renamed
			// into place is no program to keep.
			return fmt.Sprintf("%s.old-%d-unfinished", file, stamp), true
		}
	}
	return "", false
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// free is path, or path with a number added when something is there.
func free(path string) string {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	for i := 2; ; i++ {
		p := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
			return p
		}
	}
}

// moveFile renames src to dst, and copies and removes it when the two are
// on different drives. A copy whose original cannot be removed, such as a
// running program's, is removed again, so the file is in one place only.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		in.Close()
		return err
	}
	_, err = io.Copy(out, in)
	in.Close()
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Remove(src)
	}
	if err != nil {
		os.Remove(dst)
	}
	return err
}

func same(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && strings.EqualFold(filepath.Clean(x), filepath.Clean(y))
}
