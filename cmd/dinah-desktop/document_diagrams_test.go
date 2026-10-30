package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/preview"
)

func TestDraftAndResourcePreviewDiagramsRenderD2(t *testing.T) {
	root := t.TempDir()
	source := "# Diagrams\n\n```d2\nstart -> finish\n```\n\n```mermaid\ngraph TD; A-->B\n```\n"
	if err := os.WriteFile(filepath.Join(root, "diagrams.md"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	service, ref := resourceTestService(t, WorkspaceSource{ID: "local", Kind: "local-folder", Name: "Documents", Path: root})
	service.app.settings.WorkspaceRecords[0].ExecutionPath = root
	service.app.settings.WorkspaceRecords[0].PrimarySourceID = "local"
	ref.Path = "diagrams.md"
	draft, err := service.app.OpenDraft(ref)
	if err != nil {
		t.Fatal(err)
	}
	document, err := service.app.PreviewDraft(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := service.preview(ref)
	if err != nil {
		t.Fatal(err)
	}
	for name, blocks := range map[string][]preview.Block{"draft": document.Blocks, "resource": resource.Blocks} {
		t.Run(name, func(t *testing.T) {
			var diagrams []preview.Diagram
			for _, block := range blocks {
				diagrams = append(diagrams, block.Diagrams...)
			}
			if len(diagrams) != 2 {
				t.Fatalf("diagram metadata missing %+v", blocks)
			}
			svg, err := service.app.RenderD2(diagrams[0].Source)
			if err != nil || !strings.Contains(svg, "<svg") || !strings.Contains(svg, "finish") {
				t.Fatalf("real renderer: %v %s", err, svg)
			}
		})
	}
}
