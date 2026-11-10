package main

import (
	"errors"
	"os"
	"strconv"

	"studio/internal/resource"
)

// MountText is the text of the file at a virtual path, read for a document
// that refers to the file. Found is false when nothing is at the path.
type MountText struct {
	Found bool   `json:"found"`
	Text  string `json:"text"`
}

// ReadWorkspaceMountText reads the file a document refers to by its virtual
// path, as a deck does for a saved diagram it shows. The path is under /home
// or /workspaces. A file longer than maxBytes is refused when maxBytes is
// above zero. A file that is not there is reported as not found and is no
// error, so the document can say which of its references is missing.
func (a *App) ReadWorkspaceMountText(path string, maxBytes int) (MountText, error) {
	v := workspaceVFS{service: a.resourceService()}
	target, err := v.resolve(path)
	if err != nil {
		return MountText{}, err
	}
	if root := parts0(target.segments); root != "home" && root != "workspaces" {
		return MountText{}, errors.New("a document can refer to files under /home and /workspaces only")
	}
	if target.ref == nil {
		return MountText{}, errors.New(target.path + " is not a file")
	}
	read, err := v.read(target.path)
	if mountMissing(err) {
		return MountText{}, nil
	}
	if err != nil {
		return MountText{}, err
	}
	if maxBytes > 0 && len(read.Text) > maxBytes {
		return MountText{}, errors.New(target.path + " is longer than " + strconv.Itoa(maxBytes) + " bytes")
	}
	return MountText{Found: true, Text: read.Text}, nil
}

func mountMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, resource.ErrNotFound)
}
