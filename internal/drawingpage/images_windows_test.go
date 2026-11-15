//go:build windows && native_drawing

package drawingpage

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNativeVerifiedCanvasImage(t *testing.T) {
	root := os.Getenv("DRAWING_PAGE_EVIDENCE")
	if root == "" {
		t.Fatal("set DRAWING_PAGE_EVIDENCE")
	}
	_, file, _, _ := runtime.Caller(0)
	assets := os.DirFS(filepath.Join(filepath.Dir(file), "../../cmd/dinah-desktop/frontend"))
	var picture bytes.Buffer
	_ = png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	r := Request{Kind: "canvas", Scale: 1, SourcePath: "diagrams/draw.canvas", Text: `{"nodes":[{"id":"p","type":"file","file":"../images/logo.png","x":0,"y":0,"width":80,"height":80}],"edges":[]}`, Images: map[string][]byte{"images/logo.png": picture.Bytes()}}
	p, err := Draw(context.Background(), assets, r, Options{DataFolder: filepath.Join(root, "verified-image", "browser")})
	if err != nil || len(p.PNG) == 0 {
		t.Fatalf("verified image: %v", err)
	}
	r.Images = nil
	p, err = Draw(context.Background(), assets, r, Options{DataFolder: filepath.Join(root, "undeclared-image", "browser")})
	if err == nil || len(p.PNG) != 0 {
		t.Fatal("missing declared image produced bytes")
	}
}
