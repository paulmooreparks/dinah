//go:build quality_task_host

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"studio/internal/resource"
	"studio/internal/workspaces"
)

var frozenDocumentRegistration sync.Once
var frozenDocumentPeers sync.Map

// This peer is reached through the actual workspace resource resolver and
// document service. It counts attempts before validating or writing anything.
type frozenDocumentPeer struct {
	path   string
	mu     sync.Mutex
	writes atomic.Int32
	refuse atomic.Bool
}

func (p *frozenDocumentPeer) Snapshot(ctx context.Context, _ string) (resource.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return resource.Snapshot{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.snapshot()
}

func (p *frozenDocumentPeer) snapshot() (resource.Snapshot, error) {
	data, err := os.ReadFile(p.path)
	return resource.Snapshot{Text: string(data), Revision: resource.ContentRevision(string(data)), Title: "Invented linked remote document", Format: "markdown"}, err
}

func (p *frozenDocumentPeer) WriteDocument(ctx context.Context, _ string, base resource.Snapshot, edit resource.Edit) (resource.Written, error) {
	p.writes.Add(1)
	if err := ctx.Err(); err != nil {
		return resource.Written{}, err
	}
	if p.refuse.Load() {
		return resource.Written{}, errors.New("invented remote peer refuses write")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	current, err := p.snapshot()
	if err != nil {
		return resource.Written{}, err
	}
	if current.Revision != base.Revision {
		return resource.Written{}, resource.ErrConflict
	}
	if err := os.WriteFile(p.path, []byte(edit.Text), 0600); err != nil {
		return resource.Written{}, err
	}
	written, err := p.snapshot()
	return resource.Written{Snapshot: written}, err
}

func (f *frozenTaskHost) seedProtectedDocuments() {
	frozenDocumentRegistration.Do(func() {
		workspaces.RegisterSourceKind("frozen-document-peer", workspaces.SourceKind{Provider: func(_ workspaces.Resolver, _ context.Context, s workspaces.Source) (any, error) {
			peer, ok := frozenDocumentPeers.Load(s.ID)
			if !ok {
				return nil, errors.New("invented document peer is absent")
			}
			return peer, nil
		}})
	})
	f.docPeer = &frozenDocumentPeer{path: filepath.Join(f.t.TempDir(), "remote-document.md")}
	localPath := filepath.Join(f.workspace, "linked.md")
	for _, path := range []string{localPath, f.docPeer.path} {
		if err := os.WriteFile(path, []byte("# Invented linked document\n"), 0600); err != nil {
			f.t.Fatal(err)
		}
	}
	sourceID := "frozen-remote-" + filepath.Base(filepath.Dir(f.docPeer.path))
	frozenDocumentPeers.Store(sourceID, f.docPeer)
	f.t.Cleanup(func() { frozenDocumentPeers.Delete(sourceID) })
	if err := f.a.updateSettings(func(s *Settings) error {
		s.WorkspaceRecords[0].Sources = append(s.WorkspaceRecords[0].Sources, WorkspaceSource{ID: sourceID, Kind: "frozen-document-peer", Name: "Invented remote documents"})
		return nil
	}); err != nil {
		f.t.Fatal(err)
	}
	record := f.a.settings.WorkspaceRecords[0]
	refs := []ResourceRef{{WorkspaceID: record.ID, SourceID: record.PrimarySourceID, Path: "linked.md"}, {WorkspaceID: record.ID, SourceID: sourceID, Path: "linked-page"}}
	f.protected = map[string]string{"local-document": localPath, "remote-document": f.docPeer.path}
	links := make([]map[string]any, 0, len(refs))
	for i, ref := range refs {
		v, err := f.a.OpenDraft(ref)
		if err != nil {
			f.t.Fatal(err)
		}
		updated, err := f.a.UpdateDraft(v.ID, v.Revision, "# Invented unsaved draft\n")
		if err != nil {
			f.t.Fatal(err)
		}
		if !updated.Changed || updated.Baseline.Text == updated.Text {
			f.t.Fatal("protected draft has no distinct saved baseline")
		}
		f.protected["draft-"+ref.SourceID] = filepath.Join(configDir(), "drafts", v.ID+".json")
		links = append(links, map[string]any{"resource": ref, "draft": v.ID})
		if i == 1 {
			f.remoteDraft = v.ID
		}
	}
	data, err := json.Marshal(map[string]any{"task": "task-a", "generation": "generation-1", "documents": links})
	if err != nil {
		f.t.Fatal(err)
	}
	linkPath := filepath.Join(f.store.path, "document-links.json")
	if err := os.WriteFile(linkPath, data, 0600); err != nil {
		f.t.Fatal(err)
	}
	f.protected["task-document-links"] = linkPath
}

func (f *frozenTaskHost) protectedSnapshot() string {
	rows := map[string]any{"remoteWriteAttempts": f.docPeer.writes.Load()}
	for name, path := range f.protected {
		data, err := os.ReadFile(path)
		if err != nil {
			f.t.Fatal(err)
		}
		rows[name] = string(data)
	}
	data, err := json.Marshal(rows)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func TestTaskHostProtectedDocumentBindingDetectsWrites(t *testing.T) {
	f := newFrozenTaskHost(t).(*frozenTaskHost)
	before := f.protectedSnapshot()
	v, err := f.a.GetDraft(f.remoteDraft)
	if err != nil {
		t.Fatal(err)
	}
	f.docPeer.refuse.Store(true)
	if _, err := f.a.SaveDraft(v.ID, v.Revision); err == nil {
		t.Fatal("invented peer accepted refused write")
	}
	if f.docPeer.writes.Load() != 1 || before == f.protectedSnapshot() {
		t.Fatal("snapshot did not detect a refused remote write attempt")
	}
	f.docPeer.refuse.Store(false)
	saved, err := f.a.SaveDraft(v.ID, v.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if f.docPeer.writes.Load() != 2 || saved.Changed {
		t.Fatal("actual document save did not reach the counted peer")
	}
	data, err := os.ReadFile(f.docPeer.path)
	if err != nil || string(data) != v.Text {
		t.Fatalf("remote document was not written: %q %v", data, err)
	}
	stored, err := f.a.GetDraft(v.ID)
	if err != nil || stored.Baseline.Text != v.Text {
		t.Fatalf("actual draft baseline was not advanced: %+v %v", stored, err)
	}
}
