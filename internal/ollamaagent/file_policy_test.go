package ollamaagent

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"studio/internal/fileaccess"
	"studio/internal/layout"
	"testing"

	"studio/internal/acp"
)

type policyReadHandler struct{ paths []string }

func (*policyReadHandler) Notify(string, json.RawMessage) {}
func (h *policyReadHandler) Request(_ context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	if method != "fs/read_text_file" {
		return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: "unexpected request"}
	}
	var p acp.ReadTextFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &acp.Error{Code: -32602, Message: "bad params"}
	}
	h.paths = append(h.paths, p.Path)
	return nil, &acp.Error{Code: acp.CodeInternal, Message: "synthetic policy denial"}
}
func TestHostReadPolicyCoversLimitedReadAndSearch(t *testing.T) {
	work, _, _ := layoutFor(t)
	h := &policyReadHandler{}
	c := &Conn{handler: h}
	c.SetHostFileReads()
	operationCalls := 0
	c.SetHostFileOperation(func(ctx context.Context, session, path, source string, work func(context.Context) (string, bool)) (string, bool) {
		operationCalls++
		if path == "" || source == "" {
			t.Fatal("operation lost binding")
		}
		return work(ctx)
	})
	c.SetHostDirectoryReads(func(_ context.Context, _ string, path, _ string) ([]fs.DirEntry, error) {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return []fs.DirEntry{fs.FileInfoToDirEntry(info)}, nil
		}
		return os.ReadDir(path)
	})
	tools := limited(work, fixedFolders(work))
	tools.c = c
	if text, ok := tools.run(context.Background(), "1", "read_file", map[string]any{"path": filepath.Join(work, "a.txt")}); ok || !strings.Contains(text, "policy denial") {
		t.Fatal("limited read bypassed host policy")
	}
	text, _ := tools.run(context.Background(), "2", "search_text", map[string]any{"path": work, "pattern": "needle"})
	if strings.Contains(text, "needle in work") || strings.Contains(text, "needle in sub") {
		t.Fatal("search returned policy-denied contents")
	}
	if !strings.Contains(text, "2 files could not be read") {
		t.Fatalf("search hid its incomplete coverage: %s", text)
	}
	if operationCalls != 1 {
		t.Fatalf("search was not one operation: %d", operationCalls)
	}
	if len(h.paths) != 3 {
		t.Fatalf("not every candidate reached policy: %v", h.paths)
	}
}

func TestHostDirectoryPolicyCoversListAndFind(t *testing.T) {
	work, _, _ := layoutFor(t)
	calls := []string{}
	c := &Conn{handler: &policyReadHandler{}}
	c.SetHostFileReads()
	operationCalls := 0
	c.SetHostFileOperation(func(ctx context.Context, session, path, source string, work func(context.Context) (string, bool)) (string, bool) {
		operationCalls++
		if path == "" || source == "" {
			t.Fatal("operation lost binding")
		}
		return work(ctx)
	})
	c.SetHostDirectoryReads(func(_ context.Context, _ string, path, source string) ([]fs.DirEntry, error) {
		calls = append(calls, source+":"+path)
		return nil, errors.New("synthetic directory policy refusal")
	})
	tools := limited(work, fixedFolders(work))
	tools.c = c
	for _, name := range []string{"list_directory", "find_files"} {
		text, ok := tools.run(context.Background(), "test", name, map[string]any{"path": work, "pattern": "*"})
		if ok || !strings.Contains(text, "synthetic directory policy refusal") {
			t.Fatalf("refusal hidden for %s: %s", name, text)
		}
		if strings.Contains(text, "a.txt") || strings.Contains(text, "needle") {
			t.Fatal("directory policy bypass", text)
		}
	}
	if operationCalls != 2 {
		t.Fatalf("list and find were not separate operations: %d", operationCalls)
	}
	if len(calls) != 2 || !strings.HasPrefix(calls[0], "list_directory:") || !strings.HasPrefix(calls[1], "find_files:") {
		t.Fatalf("directory mediation: %v", calls)
	}
}

func TestRealDirectoryPolicyOmitsDeniedNamesFromTools(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{".netrc", "ordinary.txt"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "private-name"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := fileaccess.Open(filepath.Join(t.TempDir(), "file-access.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	svc := &fileaccess.Service{Store: store, Defaults: fileaccess.DefaultRules(runtime.GOOS, home, "", "", nil)}
	scope := fileaccess.Scope{Chat: "chat", Workspace: "workspace", Roots: []string{home}, Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	c := &Conn{handler: &policyReadHandler{}}
	c.SetHostFileReads()
	c.SetHostDirectoryReads(func(ctx context.Context, session, path, source string) ([]fs.DirEntry, error) {
		if session != scope.Chat {
			t.Fatal("caller changed")
		}
		s := scope
		s.Source = source
		return svc.ReadDirectory(ctx, path, s)
	})
	c.SetHostFileOperation(func(ctx context.Context, session, path, source string, work func(context.Context) (string, bool)) (string, bool) {
		if session != scope.Chat {
			t.Fatal("caller changed")
		}
		s := scope
		s.Source = source
		return svc.RunOperation(ctx, path, s, work)
	})
	tools := limited(home, fixedFolders(home))
	tools.c = c
	for _, name := range []string{"list_directory", "find_files"} {
		text, ok := tools.run(context.Background(), "test", name, map[string]any{"path": home, "pattern": "*"})
		if !ok || !strings.Contains(text, "ordinary.txt") || strings.Contains(text, ".netrc") || strings.Contains(text, ".ssh") || strings.Contains(text, "private-name") {
			t.Fatalf("%s leaked or failed: %v %s", name, ok, text)
		}
	}
	log, err := store.Log(fileaccess.Filter{})
	if err != nil || len(log.Rows) != 2 {
		t.Fatalf("operation audit: %+v %v", log, err)
	}
	for _, row := range log.Rows {
		if !row.Operation || row.Denied != 2 {
			t.Fatalf("missing real denials: %+v", row)
		}
	}
}
