package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"studio/internal/resource"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// The files a draft shows and keeps: an image or file added in the Editor is
// kept with the draft until it is saved, when it is written with the text
// (drafts.Service.Save). The Editor loads a draft's images from
// /draft-file, which serves a file kept with the draft, or else the one of
// that name beside the document the draft was opened from.

// AddDraftFile keeps a file, given as base64, with a draft, as when an image
// is pasted or dropped into the Editor, and returns the name it is kept
// under, which the text refers to.
func (a *App) AddDraftFile(id, name, data string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", errors.New("the file's content could not be read")
	}
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return "", err
	}
	return o.AddFile(id, name, raw)
}

// PickDraftFiles asks for files in the native dialog, images only when
// images is true, keeps each with a draft, and returns the names they are
// kept under. It returns none when the dialog is cancelled.
func (a *App) PickDraftFiles(id string, images bool) ([]string, error) {
	opts := wruntime.OpenDialogOptions{Title: "Choose Files to Attach", DefaultDirectory: a.currentWorkspace()}
	if images {
		opts.Title = "Choose Images to Insert"
		opts.Filters = []wruntime.FileFilter{{DisplayName: "Images (*.png, *.jpg, *.gif, *.svg, *.webp)", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.svg;*.webp"}}
	}
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, opts)
	if err != nil {
		return nil, err
	}
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		data, err := readDraftFile(path)
		if err != nil {
			return names, err
		}
		name, err := o.AddFile(id, filepath.Base(path), data)
		if err != nil {
			return names, err
		}
		names = append(names, name)
	}
	return names, nil
}

func readDraftFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a file that can be attached", filepath.Base(path))
	}
	data, err := io.ReadAll(io.LimitReader(f, resource.MaxFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > resource.MaxFile {
		return nil, fmt.Errorf("%s is larger than %d MB, so it was not attached", filepath.Base(path), resource.MaxFile>>20)
	}
	return data, nil
}

// draftFileHandler serves /draft-file?draft=<id>&name=<file>: an image a
// draft shows. Only images are served, under the same headers as a source's
// attachments.
func (a *App) draftFileHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		o, err := a.documentOps(r.Context(), "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		name := r.URL.Query().Get("name")
		data, err := o.File(r.URL.Query().Get("draft"), name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		mime, err := attachmentImageType(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
			return
		}
		w.Header().Set("Content-Type", mime)
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}
