//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
	"studio/internal/applet"
)

func TestWorkspaceAppletWindowsAttributes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		attribute uint32
	}{
		{"hidden", windows.FILE_ATTRIBUTE_HIDDEN},
		{"system", windows.FILE_ATTRIBUTE_SYSTEM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			file := filepath.Join(ws, "restricted.txt")
			dir := filepath.Join(ws, "restricted-folder")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{file, filepath.Join(dir, "child.txt")} {
				if err := os.WriteFile(path, []byte("private"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{file, dir} {
				p, err := windows.UTF16PtrFromString(path)
				if err != nil {
					t.Fatal(err)
				}
				attributes, err := windows.GetFileAttributes(p)
				if err != nil {
					t.Fatal(err)
				}
				if err := windows.SetFileAttributes(p, attributes|tc.attribute); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = windows.SetFileAttributes(p, attributes) })
			}
			a := NewApp()
			a.settings.WorkspaceRecords = []WorkspaceRecord{newWorkspaceRecord(ws)}
			bundle, err := a.workspaceAppletBundle()
			if err != nil {
				t.Fatal(err)
			}
			r, err := applet.New(bundle)
			if err != nil {
				t.Fatal(err)
			}
			ctx := builtinTestContext(ws)
			for _, path := range []string{"restricted.txt", "restricted-folder/child.txt"} {
				input, _ := json.Marshal(map[string]string{"path": path})
				if _, err := r.Execute(ctx, workspaceReadAction, string(input)); err == nil {
					t.Errorf("read %s path %s", tc.name, path)
				}
			}
			if _, err := r.Execute(ctx, workspaceListAction, `{"path":"restricted-folder"}`); err == nil {
				t.Fatal("listed restricted parent")
			}
			result, err := r.Execute(ctx, workspaceListAction, `{}`)
			if err != nil || strings.Contains(result.OutputJSON, "restricted") {
				t.Fatalf("restricted entries visible: %s, %v", result.OutputJSON, err)
			}
			f, err := os.Open(file)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			hidden, err := workspaceFileHidden(f)
			if err != nil || !hidden {
				t.Fatalf("handle attribute check: %v, %v", hidden, err)
			}
		})
	}
}
