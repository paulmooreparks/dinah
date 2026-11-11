package main

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/workspaces"
)

// maxComputerFile bounds a file the page made that is saved to the
// computer, such as a picture of what a window draws.
const maxComputerFile = 64 << 20

// SaveFileToComputer saves a file the page made, such as a picture, to a
// place on the computer the person chooses in the system's Save dialog,
// which suggests name and starts in the workspace's folder. data is the
// file's bytes in base64. It returns the path written, or "" when the
// person cancelled. The system's dialog asks before an existing file is
// chosen, so the file chosen is replaced.
func (a *App) SaveFileToComputer(workspaceID, name, data string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("choosing a file needs the " + product.Name + " window")
	}
	content, err := computerFileBytes(data)
	if err != nil {
		return "", err
	}
	name = filepath.Base(filepath.FromSlash(strings.TrimSpace(name)))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "file"
	}
	dir := ""
	if record, err := a.workspaceSourceRecord(workspaceID); err == nil {
		dir = workspaces.ExecutionPath(record)
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:            "Save to Your Computer",
		DefaultDirectory: dir,
		DefaultFilename:  name,
		Filters:          computerFileFilters(name),
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, writeComputerFile(path, content)
}

// computerFileBytes decodes a file the page sent and refuses one too large
// to be something a page made for the person.
func computerFileBytes(data string) ([]byte, error) {
	if base64.StdEncoding.DecodedLen(len(data)) > maxComputerFile+3 {
		return nil, errors.New("the file is too large to save this way")
	}
	content, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, errors.New("the file could not be read from the page")
	}
	if len(content) > maxComputerFile {
		return nil, errors.New("the file is too large to save this way")
	}
	return content, nil
}

// computerFileFilters offers the kind of the suggested name first, by its
// extension, and every file after it.
func computerFileFilters(name string) []wruntime.FileFilter {
	all := wruntime.FileFilter{DisplayName: "All Files (*.*)", Pattern: "*.*"}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if ext == "" || strings.ContainsAny(ext, "*?;") {
		return []wruntime.FileFilter{all}
	}
	return []wruntime.FileFilter{{DisplayName: strings.ToUpper(ext) + " Files (*." + ext + ")", Pattern: "*." + ext}, all}
}

// writeComputerFile writes the file whole: to a temporary name beside it,
// then into place, so a failed write leaves what was there.
func writeComputerFile(path string, content []byte) error {
	if !filepath.IsAbs(path) {
		return errors.New("choose a place on the computer to save the file")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".saving-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(content)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}
