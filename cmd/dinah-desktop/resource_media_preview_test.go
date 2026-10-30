package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResourceDiagramFilesUseSharedRenderers(t *testing.T) {
	root := t.TempDir()
	service, ref := resourceTestService(t, WorkspaceSource{ID: "media", Kind: "local-folder", Path: root})
	for name, language := range map[string]string{"chart.d2": "d2", "chart.mmd": "mermaid", "chart.mermaid": "mermaid"} {
		t.Run(name, func(t *testing.T) {
			text := "a -> b\nlabel: \"<script> and ```\"\n"
			if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			ref.Path = name
			preview, err := service.preview(ref)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Format != language || len(preview.Blocks) != 1 || len(preview.Blocks[0].Diagrams) != 1 || preview.Blocks[0].Diagrams[0].Source != text || preview.Blocks[0].Diagrams[0].Language != language || strings.Contains(preview.Blocks[0].HTML, "<script>") || !slices.Contains(preview.Resource.Actions, "preview") {
				t.Fatalf("diagram preview: %+v", preview)
			}
			read, err := service.read(ref)
			if err != nil || read.Format != "text" || read.Text != text {
				t.Fatalf("rendering changed the source representation: %+v %v", read, err)
			}
		})
	}
}

func TestResourceImagePreviewUsesRegisteredSourceIdentity(t *testing.T) {
	root := t.TempDir()
	service, ref := resourceTestService(t, WorkspaceSource{ID: "media", Kind: "local-folder", Path: root})
	if err := os.WriteFile(filepath.Join(root, "picture.png"), []byte("binary fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	ref.Path = "picture.png"
	preview, err := service.preview(ref)
	if err != nil || preview.Format != "image" || preview.Resource.Ref != ref || !slices.Contains(preview.Resource.Actions, "preview") || slices.Contains(preview.Resource.Actions, "read") || !strings.Contains(preview.Blocks[0].HTML, `data-attachment="picture.png"`) || strings.Contains(preview.Blocks[0].HTML, "src=") {
		t.Fatalf("image preview: %+v %v", preview, err)
	}
	f, err := os.OpenFile(filepath.Join(root, "large.png"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate((20 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	ref.Path = "large.png"
	if _, err := service.preview(ref); err == nil {
		t.Fatal("oversized image preview was accepted")
	}
	ref.Path = "../picture.png"
	if _, err := service.preview(ref); err == nil {
		t.Fatal("image path escaped its source")
	}
}
