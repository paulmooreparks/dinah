package durable

import (
	"io/fs"
	"os"
	"path/filepath"
)

// DirFS answers a read-only file system rooted at dir, standing in for
// os.DirFS, whose Open is os.Open. A file is opened through Open, so it shares
// delete access on Windows as every other read here does; a directory is
// opened for listing as os.DirFS opens one.
func DirFS(dir string) fs.FS {
	return dirFS(dir)
}

// dirFS is the file system DirFS answers, rooted at the directory it names.
type dirFS string

// Open opens the file or directory name names below the root.
func (d dirFS) Open(name string) (fs.File, error) {
	full, err := d.join("open", name)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err == nil && info.IsDir() {
		return os.Open(full)
	}
	return Open(full)
}

// ReadDir lists the directory name names below the root.
func (d dirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	full, err := d.join("readdir", name)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(full)
}

// Stat describes the file name names below the root.
func (d dirFS) Stat(name string) (fs.FileInfo, error) {
	full, err := d.join("stat", name)
	if err != nil {
		return nil, err
	}
	return os.Stat(full)
}

// join refuses a name fs.ValidPath refuses and answers the operating-system
// path it names below the root.
func (d dirFS) join(op, name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	return filepath.Join(string(d), filepath.FromSlash(name)), nil
}
