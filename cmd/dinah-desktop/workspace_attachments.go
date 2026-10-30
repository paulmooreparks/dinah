package main

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"studio/internal/resource/localfs"
	"studio/internal/workspaces"
)

//go:embed workspace_attachments.go
var workspaceAttachmentsSource []byte

const workspaceAttachmentLimit = 20 << 20

func attachmentImageType(data []byte) (string, error) {
	if cfg, format, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 100000000 {
			return "", errors.New("image dimensions exceed the preview limit")
		}
		switch format {
		case "png":
			return "image/png", nil
		case "jpeg":
			return "image/jpeg", nil
		case "gif":
			return "image/gif", nil
		case "webp":
			return "image/webp", nil
		}
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if start, ok := token.(xml.StartElement); ok {
			if start.Name.Local == "svg" && (start.Name.Space == "http://www.w3.org/2000/svg" || start.Name.Space == "") {
				return "image/svg+xml", nil
			}
			break
		}
	}
	return "", errors.New("the attachment is not a supported image")
}

// attachment reads an image the document at ref shows, by its name or its
// path from the document's folder (localfs.FilePlaces).
func (s workspaceResourceService) attachment(ref ResourceRef, name string) ([]byte, error) {
	places := localfs.FilePlaces(ref.Path, name)
	if len(places) == 0 || (!strings.Contains(name, "/") && !workspaces.ValidAttachmentName(name)) {
		return nil, errors.New("invalid attachment filename")
	}
	if s.ctx != nil && s.ctx.Err() != nil {
		return nil, s.ctx.Err()
	}
	source, err := s.source(ref)
	if err != nil {
		return nil, err
	}
	if source.Kind != "local-folder" {
		return nil, errors.New("this source does not provide image attachments")
	}
	doc, err := openWorkspaceResource(source, ref.Path)
	if err != nil {
		return nil, err
	}
	info, err := doc.Stat()
	doc.Close()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("the attachment owner is not a document")
	}
	for _, candidate := range places {
		file, err := openWorkspaceResource(source, candidate)
		if err != nil {
			continue
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			file.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(file, workspaceAttachmentLimit+1))
		file.Close()
		if err != nil {
			return nil, errors.New("the image could not be read")
		}
		if len(data) > workspaceAttachmentLimit {
			return nil, errors.New("the image exceeds the 20 MiB preview limit")
		}
		return data, nil
	}
	return nil, errAttachmentNotFound
}

// errAttachmentNotFound is the answer for an attachment no folder beside
// its document has.
var errAttachmentNotFound = errors.New("attachment not found")

func (s workspaceResourceService) attachmentHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		name := q.Get("name")
		if !workspaces.ValidAttachmentName(name) || q.Get("workspaceId") == "" || q.Get("sourceId") == "" {
			http.Error(w, "invalid attachment reference", http.StatusBadRequest)
			return
		}
		requestService := s
		requestService.ctx = r.Context()
		data, err := (workspaceVFS{service: requestService}).resourceAttachment(ResourceRef{WorkspaceID: q.Get("workspaceId"), SourceID: q.Get("sourceId"), Path: q.Get("path")}, name)
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, errAttachmentNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
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
