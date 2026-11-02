package agents

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/layout"
)

func writeIdentityFile(l layout.Layout, path, content string, managedRoots []string) error {
	base := filepath.VolumeName(path) + string(filepath.Separator)
	rel, err := filepath.Rel(base, path)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return errors.New("a regular file path is required")
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return err
	}
	defer func() { root.Close() }()
	info, err := root.Stat(".")
	if err != nil {
		return err
	}
	chain := []os.FileInfo{info}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		before, err := root.Lstat(part)
		if os.IsNotExist(err) {
			if err := protectedIdentity(l, chain, managedRoots...); err != nil {
				return err
			}
			if err := root.Mkdir(part, 0700); err != nil {
				return err
			}
			before, err = root.Lstat(part)
		}
		if err != nil {
			return err
		}
		if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return errors.New("the write directory redirects or is not a directory")
		}
		next, err := root.OpenRoot(part)
		if err != nil {
			return err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(before, opened) {
			next.Close()
			return errors.New("the write directory changed")
		}
		chain = append(chain, opened)
		root.Close()
		root = next
	}
	if err := protectedIdentity(l, chain, managedRoots...); err != nil {
		return err
	}
	name := parts[len(parts)-1]
	before, err := root.Lstat(name)
	flags := os.O_WRONLY
	if os.IsNotExist(err) {
		flags |= os.O_CREATE | os.O_EXCL
	} else if err != nil {
		return err
	} else if !before.Mode().IsRegular() {
		return errors.New("only regular files can be written")
	}
	// Do not truncate until the opened object's identity has been checked.
	file, err := root.OpenFile(name, flags, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || before != nil && !os.SameFile(before, opened) {
		return errors.New("the write target changed")
	}
	chain = append(chain, opened)
	if err := protectedIdentity(l, chain, managedRoots...); err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		return err
	}
	return file.Close()
}
