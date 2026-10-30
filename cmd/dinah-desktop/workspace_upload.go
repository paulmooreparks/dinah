package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/resource/localfs"
	"studio/internal/workspaces"
)

// Uploading puts files from anywhere on the computer into a folder of a
// workspace: chosen with Upload Files…, which gives their paths, or dropped
// onto a folder from the system's file manager, which gives the page their
// contents and names but not their paths, so they arrive at POST /upload.

// UploadReport says what an upload did with each file, by name.
type UploadReport struct {
	Added    []string `json:"added"`
	Replaced []string `json:"replaced"`
	Kept     []string `json:"kept"`
	Skipped  []string `json:"skipped"`
	// Refused are files that were not uploaded, each with the reason, such
	// as a folder, which is not uploaded, or a name the folder cannot take.
	Refused []string `json:"refused"`
}

// ChooseUploadFiles asks for the files to upload, several at once. It gives
// the dialog no starting folder, so it opens where the person last chose
// files, such as Downloads, rather than in the folder being uploaded to.
func (a *App) ChooseUploadFiles() ([]string, error) {
	if a.ctx == nil {
		return nil, errors.New("files can be chosen only in the open application")
	}
	return wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Choose Files to Upload"})
}

// uploadFolder is the folder on the computer a workspace folder is, or why
// it takes no files.
func (a *App) uploadFolder(folder ResourceRef) (string, WorkspaceSource, error) {
	s := a.resourceService()
	source, err := s.source(folder)
	if err != nil {
		return "", WorkspaceSource{}, err
	}
	if source.Kind != "local-folder" {
		return "", WorkspaceSource{}, errors.New("files can be uploaded only to a folder on the computer for now")
	}
	dir, isDir, err := workspaceVFS{service: s}.localPath(folder)
	if err != nil {
		return "", WorkspaceSource{}, err
	}
	if !isDir {
		return "", WorkspaceSource{}, errors.New("choose a folder to upload to")
	}
	return dir, source, nil
}

// UploadConflicts lists the names of the files the folder already has, so
// that the person can choose what happens to them before anything is copied.
func (a *App) UploadConflicts(folder ResourceRef, paths []string) ([]string, error) {
	dir, _, err := a.uploadFolder(folder)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range paths {
		name := filepath.Base(p)
		if info, err := os.Stat(p); err != nil || info.IsDir() {
			continue
		}
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil && !samePath(p, filepath.Join(dir, name)) {
			out = append(out, name)
		}
	}
	return out, nil
}

// UploadToWorkspaceFolder copies files from the computer into a workspace
// folder. existing says what happens to a file the folder already has a
// file of the same name for: "replace" replaces it, "keep-both" keeps both,
// naming the new one "name (2).ext", and "skip", or anything else, leaves
// it. A folder is not uploaded, and a file is never copied onto itself.
func (a *App) UploadToWorkspaceFolder(folder ResourceRef, paths []string, existing string) (UploadReport, error) {
	report := newUploadReport()
	dir, source, err := a.uploadFolder(folder)
	if err != nil {
		return report, err
	}
	for _, p := range paths {
		name := filepath.Base(p)
		info, err := os.Stat(p)
		switch {
		case err != nil:
			report.Refused = append(report.Refused, name+": "+err.Error())
			continue
		case info.IsDir():
			report.Refused = append(report.Refused, name+": a folder is not uploaded; upload the files in it")
			continue
		case !info.Mode().IsRegular():
			report.Refused = append(report.Refused, name+": not an ordinary file")
			continue
		}
		if samePath(p, filepath.Join(dir, name)) {
			report.Skipped = append(report.Skipped, name)
			continue
		}
		place(&report, dir, source, name, existing, func() (io.ReadCloser, error) { return os.Open(p) })
	}
	return report, nil
}

// UploadNameConflicts lists which of the names the folder already has, for
// files dropped onto it from the system's file manager, whose names the
// page knows but whose paths it does not.
func (a *App) UploadNameConflicts(folder ResourceRef, names []string) ([]string, error) {
	dir, _, err := a.uploadFolder(folder)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(dir, filepath.Base(name))); err == nil {
			out = append(out, filepath.Base(name))
		}
	}
	return out, nil
}

func newUploadReport() UploadReport {
	return UploadReport{Added: []string{}, Replaced: []string{}, Kept: []string{}, Skipped: []string{}, Refused: []string{}}
}

// place puts one file into a workspace folder under the upload's rules: a
// name the folder may hold, no hidden name where the workspace hides them,
// and what existing says for a name the folder already has. It is the one
// set of rules for files chosen by path and files dropped as content.
func place(report *UploadReport, dir string, source WorkspaceSource, name, existing string, open func() (io.ReadCloser, error)) {
	if problem := localfs.NameProblem(name); problem != "" {
		report.Refused = append(report.Refused, name+": "+problem)
		return
	}
	if !workspaces.Dotfiles(source) && strings.HasPrefix(name, ".") {
		report.Refused = append(report.Refused, name+": the workspace hides names that start with a dot")
		return
	}
	target := filepath.Join(dir, name)
	_, statErr := os.Lstat(target)
	exists := statErr == nil
	write := func(to string, replace bool) error {
		in, err := open()
		if err != nil {
			return err
		}
		defer in.Close()
		return copyFrom(in, to, replace)
	}
	switch {
	case exists && existing == "replace":
		if err := write(target, true); err != nil {
			report.Refused = append(report.Refused, name+": "+err.Error())
			return
		}
		report.Replaced = append(report.Replaced, name)
	case exists && existing == "keep-both":
		other := freeName(dir, name)
		if err := write(filepath.Join(dir, other), false); err != nil {
			report.Refused = append(report.Refused, name+": "+err.Error())
			return
		}
		report.Kept = append(report.Kept, other)
	case exists:
		report.Skipped = append(report.Skipped, name)
	default:
		if err := write(target, false); err != nil {
			report.Refused = append(report.Refused, name+": "+err.Error())
			return
		}
		report.Added = append(report.Added, name)
	}
}

// uploadDropped answers POST /upload: files dropped onto a workspace folder
// from the system's file manager, sent by the page as a multipart form with
// the folder's reference in "folder" and the choice for existing names in
// "existing", ahead of the files. Each file is streamed to disk as it
// arrives, so its size is not held in memory.
func (a *App) uploadDropped(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, err error) { http.Error(w, err.Error(), status) }
	mr, err := r.MultipartReader()
	if err != nil {
		fail(http.StatusBadRequest, err)
		return
	}
	var folder ResourceRef
	existing := ""
	var dir string
	var source WorkspaceSource
	report := newUploadReport()
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
		switch part.FormName() {
		case "folder":
			if err := json.NewDecoder(io.LimitReader(part, 1<<20)).Decode(&folder); err != nil {
				fail(http.StatusBadRequest, fmt.Errorf("the folder could not be read: %w", err))
				return
			}
			if dir, source, err = a.uploadFolder(folder); err != nil {
				fail(http.StatusUnprocessableEntity, err)
				return
			}
		case "existing":
			b, _ := io.ReadAll(io.LimitReader(part, 64))
			existing = string(b)
		case "file":
			if dir == "" {
				fail(http.StatusBadRequest, errors.New("the folder must come before the files"))
				return
			}
			name := filepath.Base(part.FileName())
			place(&report, dir, source, name, existing, func() (io.ReadCloser, error) { return io.NopCloser(part), nil })
		}
		part.Close()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// WorkspaceLocalPath is where a workspace file or folder is on this
// computer, for Copy Path.
func (a *App) WorkspaceLocalPath(ref ResourceRef) (string, error) {
	p, _, err := workspaceVFS{service: a.resourceService()}.localPath(ref)
	return p, err
}

// samePath says whether two paths are the same file.
func samePath(a, b string) bool {
	ai, err1 := os.Stat(a)
	bi, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ai, bi)
}

// freeName is name, or "name (2).ext" and so on, whichever the folder does
// not have yet.
func freeName(dir, name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", stem, i, ext)
		if _, err := os.Lstat(filepath.Join(dir, candidate)); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

// copyFrom writes what in holds to a file, through a temporary file beside
// the target so that a copy cut short leaves no half file; replace says
// whether an existing target may be replaced.
func copyFrom(in io.Reader, to string, replace bool) error {
	tmp, err := os.CreateTemp(filepath.Dir(to), ".upload-*")
	if err != nil {
		return err
	}
	done := false
	defer func() {
		if !done {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err := io.Copy(tmp, in); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if !replace {
		// The name may have been taken while the file was copied.
		if _, err := os.Lstat(to); err == nil {
			return errors.New("a file of this name appeared while it was being copied")
		}
	}
	if err := os.Rename(tmp.Name(), to); err != nil {
		return err
	}
	done = true
	return nil
}
