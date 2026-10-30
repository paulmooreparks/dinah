package main

import (
	"bytes"
	"image"
	"image/png"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func testAttachmentPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func attachmentRequest(s workspaceResourceService, ref ResourceRef, name string) *httptest.ResponseRecorder {
	q := url.Values{"workspaceId": {ref.WorkspaceID}, "sourceId": {ref.SourceID}, "path": {ref.Path}, "name": {name}}
	w := httptest.NewRecorder()
	s.attachmentHandler().ServeHTTP(w, httptest.NewRequest("GET", "/resource-attachment?"+q.Encode(), nil))
	return w
}
func TestWorkspaceAttachmentLocalContainmentAndMIME(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nested")
	os.MkdirAll(filepath.Join(dir, "images"), 0700)
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte("![test](image.png)"), 0600)
	data := testAttachmentPNG(t)
	os.WriteFile(filepath.Join(dir, "images", "image.png"), data, 0600)
	s, ref := resourceTestService(t, WorkspaceSource{ID: "s", Kind: "local-folder", Path: root})
	ref.Path = "nested/doc.md"
	w := attachmentRequest(s, ref, "image.png")
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("response %d %v", w.Code, w.Header())
	}
	for _, name := range []string{"../image.png", "C:\\image.png", ".env", "missing.png"} {
		if w := attachmentRequest(s, ref, name); w.Code == 200 {
			t.Errorf("accepted %s", name)
		}
	}
	original := ref
	ref.Path = "nested/missing.md"
	if w := attachmentRequest(s, ref, "image.png"); w.Code == 200 {
		t.Fatal("missing owner accepted")
	}
	ref = original
	os.WriteFile(filepath.Join(dir, "bad.png"), []byte("<html>not an image</html>"), 0600)
	if w := attachmentRequest(s, ref, "bad.png"); w.Code != 415 {
		t.Fatalf("bad MIME %d", w.Code)
	}
	os.WriteFile(filepath.Join(dir, "vector.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), 0600)
	if w := attachmentRequest(s, ref, "vector.svg"); w.Code != 200 || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("SVG sandbox absent")
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	os.WriteFile(outside, data, 0600)
	if err := os.Symlink(outside, filepath.Join(dir, "escape.png")); err == nil {
		if w := attachmentRequest(s, ref, "escape.png"); w.Code == 200 {
			t.Fatal("symlink escaped source")
		}
	}
}
