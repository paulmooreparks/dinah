package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/drafts"
	"studio/internal/workspaces"
)

// Opening a document by where it is: a file in a workspace's folders or
// elsewhere on the computer.

// DocumentRefAt is the resource address of a document a chat refers to by
// its path inside a workspace's folder, so the Editor opens its draft.
func (a *App) DocumentRefAt(workspace, rel string) (ResourceRef, error) {
	return a.documentResourceRef(workspace, rel)
}

// ChooseDocumentFile asks for a local file to open, starting in the
// workspace's main folder. It returns "" when the person cancels.
func (a *App) ChooseDocumentFile(workspaceID string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("choosing a file needs the " + product.Name + " window")
	}
	dir := ""
	if record, err := a.workspaceSourceRecord(workspaceID); err == nil {
		dir = workspaces.ExecutionPath(record)
	}
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            "Open a Document",
		DefaultDirectory: dir,
		Filters:          []wruntime.FileFilter{{DisplayName: "Markdown and Text (*.md, *.markdown, *.txt)", Pattern: "*.md;*.markdown;*.txt"}, {DisplayName: "All Files (*.*)", Pattern: "*.*"}},
	})
}

// OpenComputerDocument asks for a file on the computer and opens it for
// editing, for the Open dialog's From Your Computer button. A file in a
// workspace's folder or in the home folder opens as that file; any other
// text file opens as a new document with the file's text, which Save As
// puts where the person chooses. It returns a view with no ID when the
// person cancels.
func (a *App) OpenComputerDocument(workspaceID string) (drafts.View, error) {
	path, err := a.ChooseDocumentFile(workspaceID)
	if err != nil || path == "" {
		return drafts.View{}, err
	}
	return a.OpenComputerFile(workspaceID, path)
}

// maxComputerDocument is the largest file the Editor opens from the
// computer, as it is for files in a workspace's folders.
const maxComputerDocument = 1 << 20

// NewEmptyDraft starts an empty, untitled document in a workspace, or in
// none, for Tools › Editor: notes or a script started from nothing, which
// Save As… later puts where the person chooses.
func (a *App) NewEmptyDraft(workspaceID string) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	text := ""
	d, err := o.Drafts.Store.Create(drafts.Origin{Kind: drafts.OriginNew, WorkspaceID: workspaceID}, drafts.Baseline{}, "Untitled", &text)
	if err != nil {
		return drafts.View{}, draftErrorText(err)
	}
	return o.View(d, "created"), nil
}

// OpenComputerFile opens a file on the computer by its path, as
// OpenComputerDocument does with the file the person chooses.
func (a *App) OpenComputerFile(workspaceID, path string) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	if ref, ok := a.localDocumentRef(workspaceID, path); ok {
		v, err := o.Open(ref)
		if err == nil {
			a.recordOpened(v)
		}
		return v, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return drafts.View{}, errors.New("there is no file at " + path)
	}
	if info.Size() > maxComputerDocument {
		return drafts.View{}, errors.New(filepath.Base(path) + " is larger than 1 MB, which is more than the Editor opens")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return drafts.View{}, errors.New(filepath.Base(path) + " could not be read")
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return drafts.View{}, errors.New(filepath.Base(path) + " is not a text file")
	}
	text := string(data)
	d, err := o.Drafts.Store.Create(drafts.Origin{Kind: drafts.OriginNew, WorkspaceID: workspaceID}, drafts.Baseline{}, filepath.Base(path), &text)
	if err != nil {
		return drafts.View{}, draftErrorText(err)
	}
	if abs, err := filepath.Abs(path); err == nil {
		a.recordComputerFile(workspaceID, abs)
	}
	return o.View(d, "created"), nil
}

// localDocumentRef finds a file in the folders of the workspace given, then
// of the other workspaces, then in the home folder, /bin and /lib.
func (a *App) localDocumentRef(workspaceID, path string) (ResourceRef, bool) {
	a.mu.Lock()
	records := workspaces.Copy(a.settings.WorkspaceRecords)
	a.mu.Unlock()
	sort.SliceStable(records, func(i, j int) bool { return records[i].ID == workspaceID && records[j].ID != workspaceID })
	for _, id := range []string{workspaces.HomeSourceID, servedRoots["bin"], servedRoots["lib"]} {
		if source, err := personalSource(id); err == nil {
			records = append(records, WorkspaceRecord{Sources: []WorkspaceSource{source}})
		}
	}
	for _, record := range records {
		if ref, err := workspaces.LocalRef(record, path); err == nil {
			return ref, true
		}
	}
	return ResourceRef{}, false
}

// ErrComputerFileExists is SaveDraftToComputer's refusal to replace a file
// the person has not agreed to replace.
var ErrComputerFileExists = errors.New("a file with that name already exists")

// ChooseComputerSavePath asks where on the computer to save a copy of a
// document, suggesting name, for the Save As dialog's To Your Computer
// button. It returns "" when the person cancels.
func (a *App) ChooseComputerSavePath(workspaceID, name string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("choosing a file needs the " + product.Name + " window")
	}
	dir := ""
	if record, err := a.workspaceSourceRecord(workspaceID); err == nil {
		dir = workspaces.ExecutionPath(record)
	}
	return wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:            "Save to Your Computer",
		DefaultDirectory: dir,
		DefaultFilename:  name,
		Filters:          []wruntime.FileFilter{{DisplayName: "Markdown and Text (*.md, *.markdown, *.txt)", Pattern: "*.md;*.markdown;*.txt"}, {DisplayName: "All Files (*.*)", Pattern: "*.*"}},
	})
}

// SaveDraftToComputer writes a copy of a draft's text to a file on the
// computer. The draft keeps its origin: the copy is not tracked. A file that
// exists is replaced only when replace is true; otherwise the answer is
// ErrComputerFileExists, so the Editor can ask first.
func (a *App) SaveDraftToComputer(id, path string, replace bool) error {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return err
	}
	d, err := o.Draft(id)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		return errors.New("choose a place on the computer to save the copy")
	}
	flag := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if replace {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flag, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return ErrComputerFileExists
	}
	if err != nil {
		return errors.New(filepath.Base(path) + " could not be written")
	}
	if _, err := f.WriteString(d.Text); err != nil {
		f.Close()
		return errors.New(filepath.Base(path) + " could not be written")
	}
	return f.Close()
}
