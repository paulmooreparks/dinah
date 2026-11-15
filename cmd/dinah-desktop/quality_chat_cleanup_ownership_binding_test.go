//go:build quality_chat_cleanup && chatcleanup_acceptance

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"studio/internal/diagrampicture"
	"studio/internal/drafts"
	"sync/atomic"
	"time"
)

type qualityLinkedState struct {
	draftID     string
	workspaceID string
	leases      map[string]*qualityLease
	writes      atomic.Int32
	snapshots   int
}
type qualityLease struct {
	env       map[string]string
	entered   chan struct{}
	cancelled chan struct{}
	finished  chan error
}
type qualityTransport func(*http.Request) (*http.Response, error)

func (r qualityTransport) RoundTrip(q *http.Request) (*http.Response, error) { return r(q) }

func (f *qualityHostCleanup) SeedLinkedRecords(ids []string) {
	f.linked = &qualityLinkedState{leases: map[string]*qualityLease{}}
	record := newWorkspaceRecord(f.ws)
	f.linked.workspaceID = record.ID
	if err := f.a.updateSettings(func(s *Settings) error {
		s.WorkspaceRecords = append(s.WorkspaceRecords, record)
		return ensureWorkspaceMountAliases(s.WorkspaceRecords)
	}); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.ws, "linked.md"), []byte("# Invented document\n"), 0600); err != nil {
		f.t.Fatal(err)
	}
	ref := ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "linked.md"}
	v, err := f.a.OpenDraft(ref)
	if err != nil {
		f.t.Fatal(err)
	}
	v, err = f.a.UpdateDraft(v.ID, v.Revision, "# Unsaved document draft\n")
	if err != nil {
		f.t.Fatal(err)
	}
	f.linked.draftID = v.ID
	for _, id := range ids {
		tr, err := readTranscript(id)
		if err != nil {
			f.t.Fatal(err)
		}
		tr.Messages = append(tr.Messages, ConversationMessage{Role: "you", Text: "Reference linked.md and document draft " + v.ID})
		if err = writeTranscript(tr); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.ws, "flow.mmd"), []byte("graph TD; A-->B"), 0600); err != nil {
		f.t.Fatal(err)
	}
	// Observe HTTP mutations separately from authorized local drawing requests.
	prior := http.DefaultTransport
	http.DefaultTransport = qualityTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != diagrampicture.Route && r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			f.linked.writes.Add(1)
		}
		if r.URL.Hostname() != "127.0.0.1" && r.URL.Hostname() != "localhost" {
			return nil, errors.New("isolated acceptance blocks external HTTP")
		}
		return prior.RoundTrip(r)
	})
	f.t.Cleanup(func() { http.DefaultTransport = prior })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	f.t.Cleanup(server.Close)
	response, err := http.Post(server.URL+"/write-control", "text/plain", nil)
	if err != nil {
		f.t.Fatal(err)
	}
	response.Body.Close()
	if f.linked.writes.Load() != 1 {
		f.t.Fatal("remote write observer positive control failed")
	}
	f.linked.writes.Store(0)
}
func (f *qualityHostCleanup) LinkedRecords() map[string]string {
	data, err := os.ReadFile(filepath.Join(f.ws, "linked.md"))
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(stateLayout().State, "drafts", f.linked.draftID+".json"))
	if err != nil {
		f.t.Fatal(err)
	}
	var draft drafts.Draft
	if err = json.Unmarshal(raw, &draft); err != nil {
		f.t.Fatal(err)
	}
	var record []byte
	for _, r := range f.a.settings.WorkspaceRecords {
		if r.ID == f.linked.workspaceID {
			record, err = json.Marshal(r)
			if err != nil {
				f.t.Fatal(err)
			}
		}
	}
	if len(record) == 0 {
		f.t.Fatal("workspace document record disappeared")
	}
	recent, err := os.ReadFile(filepath.Join(stateLayout().State, "recent-documents.json"))
	if err != nil {
		f.t.Fatal(err)
	}
	out := map[string]string{"document": string(data), "document-record": string(recent), "workspace-record": string(record), "document-draft": draft.Text, "document-draft-record": string(raw)}
	if dir := os.Getenv("QUALITY_CLEANUP_EVIDENCE"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			f.t.Fatal(err)
		}
		encoded, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			f.t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("linked-%02d.json", f.linked.snapshots)), encoded, 0600); err != nil {
			f.t.Fatal(err)
		}
		f.linked.snapshots++
	}
	return out
}
func (f *qualityHostCleanup) RemoteWrites() int { return int(f.linked.writes.Load()) }
func (f *qualityHostCleanup) OpenWithLeases(id string) map[string]string {
	if _, err := f.a.OpenSessionAt(f.ws, id); err != nil {
		f.t.Fatal(err)
	}
	l := &qualityLease{entered: make(chan struct{}), cancelled: make(chan struct{}), finished: make(chan error, 1)}
	l.env = f.a.chatCleanupAcceptanceDrawingLease(f.t, id, f.ws, func(ctx context.Context, _ diagrampicture.Drawing) (diagrampicture.Picture, error) {
		close(l.entered)
		<-ctx.Done()
		close(l.cancelled)
		return diagrampicture.Picture{}, ctx.Err()
	})
	token := l.env["DIAGRAM_PICTURE_LEASE"]
	f.linked.leases[token] = l
	return map[string]string{"drawing": token}
}
func (f *qualityHostCleanup) UseLease(token string) error {
	l := f.linked.leases[token]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "OPTIONS", l.env["DIAGRAM_PICTURE_ENDPOINT"]+diagrampicture.Route, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Helper-Protocols", "1")
	req.Header.Set("X-Helper-Channel", "diagram-picture")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lease refused: %d", resp.StatusCode)
	}
	return nil
}
func (f *qualityHostCleanup) StartPendingLeaseWork(token string) {
	l := f.linked.leases[token]
	client, err := diagrampicture.New(l.env["DIAGRAM_PICTURE_ENDPOINT"], token, f.linked.workspaceID)
	if err != nil {
		f.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	f.t.Cleanup(cancel)
	go func() {
		p, err := client.Draw(ctx, diagrampicture.Request{RequestID: "bf1496d2-f919-4710-97eb-40d8a339fd20", WorkspaceID: f.linked.workspaceID, Source: diagrampicture.Snapshot{Path: "flow.mmd", SHA256: diagrampicture.Hash([]byte("graph TD; A-->B"))}, Format: "png", Scale: 1})
		if len(p.Data) != 0 {
			err = nil
		}
		l.finished <- err
	}()
	select {
	case <-l.entered:
	case <-ctx.Done():
		f.t.Fatal("pending draw did not enter renderer")
	}
}
func (f *qualityHostCleanup) PendingLeaseWork(token string) (bool, bool) {
	l := f.linked.leases[token]
	cancelled := false
	select {
	case <-l.cancelled:
		cancelled = true
	default:
	}
	select {
	case err := <-l.finished:
		return cancelled, err == nil
	case <-time.After(5 * time.Second):
		f.t.Fatal("cancelled request did not return")
		return false, false
	}
}
