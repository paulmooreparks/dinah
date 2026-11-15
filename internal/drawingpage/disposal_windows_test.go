//go:build windows && native_drawing

package drawingpage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"
)

// The injected failure is returned after real native cleanup, so the fixture
// does not leave a renderer running. Production code cannot select this seam.
func TestNativeCloseFailureIsTyped(t *testing.T) {
	root := os.Getenv("DRAWING_PAGE_EVIDENCE")
	if root == "" {
		t.Fatal("set DRAWING_PAGE_EVIDENCE")
	}
	original := closeNativeController
	defer func() { closeNativeController = original }()
	closeNativeController = func(controller unsafe.Pointer) error {
		if err := original(controller); err != nil {
			return err
		}
		return errors.New("injected Controller.Close HRESULT failure")
	}
	_, file, _, _ := runtime.Caller(0)
	assets := os.DirFS(filepath.Join(filepath.Dir(file), "../../cmd/dinah-desktop/frontend"))
	p, err := Draw(context.Background(), assets, Request{Kind: "mermaid", Text: "graph TD; A-->B", Scale: 1}, Options{DataFolder: filepath.Join(root, "forced-close", "browser")})
	var disposal *DisposalError
	if !errors.As(err, &disposal) || len(p.PNG) != 0 {
		t.Fatalf("Close failure returned bytes or lost its type: %v", err)
	}
}
