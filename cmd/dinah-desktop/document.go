package main

import (
	"bytes"
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"studio/internal/layout"
	"studio/internal/workspaces"
)

// Problem is one thing the converter or a check found, at its line in the
// Markdown.
type Problem struct {
	Line     int    `json:"line"`
	Severity string `json:"severity"` // error, warning or info
	Message  string `json:"message"`
	Rule     string `json:"rule,omitempty"`
}

// documentPath turns a workspace-relative path into an absolute one inside
// the workspace, or fails.
func documentPath(ws, rel string) (string, string, error) {
	if ws == "" {
		return "", "", errors.New("no workspace is chosen")
	}
	if !filepath.IsAbs(ws) {
		return "", "", errors.New("the workspace path must be absolute")
	}
	p := filepath.Join(ws, filepath.FromSlash(rel))
	if !layout.Inside(ws, p) {
		return "", "", errors.New("that file is outside the workspace")
	}
	return ws, p, nil
}

// docFiles serves the images a document refers to, at
// /doc-file?doc=<document>&name=<file>. The preview leaves an image as
// data-attachment="<file>" and binds nothing (internal/preview), so the
// page asks for it here; the file is looked for beside the document and in
// its attachments/ and images/ folders, never outside the workspace.
func (a *App) docFiles() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/resource-attachment" {
			a.resourceService().attachmentHandler().ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/draft-file" {
			a.draftFileHandler().ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/bus-attachment" {
			a.busAttachmentFiles(w, r)
			return
		}
		if r.URL.Path != "/doc-file" {
			http.NotFound(w, r)
			return
		}
		workspace := r.URL.Query().Get("workspace")
		if workspace == "" {
			a.mu.Lock()
			workspace = a.settings.Workspace
			a.mu.Unlock()
		}
		ref, err := a.documentResourceRef(workspace, r.URL.Query().Get("doc"))
		name := r.URL.Query().Get("name")
		if err != nil || !workspaces.ValidAttachmentName(name) {
			http.NotFound(w, r)
			return
		}
		service := a.resourceService()
		service.ctx = r.Context()
		data, err := (workspaceVFS{service: service}).resourceAttachment(ref, name)
		if err == nil {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
			return
		}
		http.NotFound(w, r)
	})
}
