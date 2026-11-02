package ollamaagent

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	if len(h.paths) != 3 {
		t.Fatalf("not every candidate reached policy: %v", h.paths)
	}
}

func TestHostDirectoryPolicyCoversListAndFind(t *testing.T) {
	work, _, _ := layoutFor(t)
	calls := []string{}
	c := &Conn{handler: &policyReadHandler{}}
	c.SetHostFileReads()
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
	if len(calls) != 2 || !strings.HasPrefix(calls[0], "list_directory:") || !strings.HasPrefix(calls[1], "find_files:") {
		t.Fatalf("directory mediation: %v", calls)
	}
}
