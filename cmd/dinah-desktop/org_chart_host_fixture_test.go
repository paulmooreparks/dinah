//go:build org_chart_acceptance && startup_page_acceptance

package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/resource"
	"studio/internal/workspaces"
)

var orgChartReceivingFixtures sync.Map
var orgChartDocumentPeers sync.Map
var orgChartDocumentRegistration sync.Once

type orgChartHostFixture struct {
	mu             sync.Mutex
	a              *App
	manifest       map[string]any
	protected      map[string]string
	document       *orgChartDocumentPeer
	eventMu        sync.Mutex
	events         []map[string]any
	observed       []map[string]any
	barrier        bool
	cleanupChanges int
	cleanupReads   int
	cleanupWrites  int
	cleanupChat    string
	mentionIDs     []int64
}

type orgChartDocumentPeer struct {
	path   string
	mu     sync.Mutex
	writes atomic.Int32
}

func (p *orgChartDocumentPeer) Snapshot(ctx context.Context, _ string) (resource.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return resource.Snapshot{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	data, err := os.ReadFile(p.path)
	return resource.Snapshot{Text: string(data), Revision: resource.ContentRevision(string(data)), Format: "markdown", Title: "Invented protected remote document"}, err
}

func (p *orgChartDocumentPeer) WriteDocument(_ context.Context, _ string, _ resource.Snapshot, _ resource.Edit) (resource.Written, error) {
	p.writes.Add(1)
	return resource.Written{}, errors.New("invented remote peer refuses writes")
}

func init() {
	startupPageFixtureSetup = func(t *testing.T, a *App) { newOrgChartHostFixture(t, a) }
}

func newOrgChartHostFixture(t *testing.T, a *App) *orgChartHostFixture {
	t.Helper()
	f := &orgChartHostFixture{a: a, protected: map[string]string{}, manifest: map[string]any{}}
	f.installEvents(a)
	orgChartReceivingFixtures.Store(a, f)
	for _, name := range []string{"OrgChartHostFixtureManifest", "OrgChartHostFixtureSnapshot", "OrgChartHostFixtureActions", "OrgChartHostFixtureApply", "OrgChartHostFixtureControl", "OrgChartHostFixtureCleanup", "OrgChartHostFixtureMentionSeed", "OrgChartHostFixtureMentionState", "OrgChartHostFixtureAcknowledgeMention", "OrgChartHostFixtureMentionCall"} {
		pageMethods[name] = pageQuery
		if name == "OrgChartHostFixtureApply" || name == "OrgChartHostFixtureControl" || name == "OrgChartHostFixtureMentionSeed" || name == "OrgChartHostFixtureAcknowledgeMention" || name == "OrgChartHostFixtureMentionCall" {
			pageMethods[name] = pageWork
		}
		t.Cleanup(func() { delete(pageMethods, name) })
	}
	if reflect.ValueOf(a).MethodByName("OrgChartHostFixtureMentionTask").IsValid() {
		pageMethods["OrgChartHostFixtureMentionTask"] = pageWork
		t.Cleanup(func() { delete(pageMethods, "OrgChartHostFixtureMentionTask") })
	}
	t.Cleanup(func() { f.a.shutdown(context.Background()); orgChartReceivingFixtures.Delete(a) })
	workspaces := []WorkspaceRecord{newWorkspaceRecord(t.TempDir()), newWorkspaceRecord(t.TempDir()), newWorkspaceRecord(t.TempDir())}
	workspaces[0].Name, workspaces[1].Name = "Invented Alpha", "Invented Beta"
	workspaces[2].Name = "Invented empty workspace"
	if err := a.updateSettings(func(s *Settings) error {
		s.WorkspaceRecords = workspaces
		s.Workspace = workspaces[0].ExecutionPath
		s.WorkspaceID = workspaces[0].ID
		s.Recent = []string{s.Workspace}
		s.Workspaces = []string{s.Workspace}
		s.DefaultContextReleased = true
		s.DefaultProvider = product.Providers[0]
		return ensureWorkspaceMountAliases(s.WorkspaceRecords)
	}); err != nil {
		t.Fatal(err)
	}
	roles := []string{"lead-a", "member-a", "member-b", "lead-b", "member-c", "nested-lead", "nested-member", "free-a", "free-b", "closed"}
	ids := map[string]string{}
	refs := map[string]BusChatRef{}
	for i, role := range roles {
		provider := product.Providers[i%len(product.Providers)]
		id := provider + ":org-fixture-" + role
		workspace := workspaces[i%2].ExecutionPath
		cleanupFixtureChat(t, a, id, provider, workspace, "org-fixture-"+role)
		if err := a.updateSettings(func(s *Settings) error { s.SessionTitles[id] = "Invented " + role; return nil }); err != nil {
			t.Fatal(err)
		}
		ids[role] = id
		refs[role] = BusChatRef{ID: id, Cwd: workspace, Title: "Invented " + role}
	}
	// The fixture owns one open conversation. Subsequent state changes enter
	// through the actual provider notification handler, never a page event stub.
	openID := ids["free-a"]
	openProvider := product.Providers[7%len(product.Providers)]
	a.mu.Lock()
	a.runtime.PutConversationLocked(openID, newSessionState(&sessionSeed{ID: openID, Provider: openProvider, Cwd: refs["free-a"].Cwd, Title: refs["free-a"].Title, UpdatedAt: "2026-11-15T00:00:00Z"}, sessionInitial{Open: true}))
	a.mu.Unlock()
	groups := map[string]string{}
	expectedGroups := []map[string]any{}
	for _, seed := range []struct {
		title   string
		members []string
	}{{"Invented overlapping group", []string{"lead-a", "member-a", "free-a"}}, {"Invented second group", []string{"member-a", "lead-b", "free-b"}}} {
		title, members := seed.title, seed.members
		chats := []BusChatRef{}
		memberIDs := []string{}
		for _, role := range members {
			chats = append(chats, refs[role])
			memberIDs = append(memberIDs, ids[role])
		}
		group, err := a.BusStartGroup(title, chats)
		if err != nil {
			t.Fatal(err)
		}
		groups[title] = group.Name
		expectedGroups = append(expectedGroups, map[string]any{"id": group.Name, "title": title, "memberIds": memberIDs})
	}
	teams := map[string]string{}
	for _, pair := range [][2]string{{"member-a", "lead-a"}, {"member-b", "lead-a"}, {"nested-lead", "lead-a"}, {"member-c", "lead-b"}} {
		group, err := a.SetTeamLead(ids[pair[0]], ids[pair[1]])
		if err != nil {
			t.Fatal(err)
		}
		teams[pair[1]] = group
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		if err := b.AcceptanceSeedLegacyTeam(context.Background(), a.knownBusID(ids["nested-member"]), a.knownBusID(ids["nested-lead"])); err != nil {
			return err
		}
		group, err := b.TeamGroup(context.Background(), a.knownBusID(ids["nested-lead"]))
		teams["nested-lead"] = group
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		for _, role := range []string{"lead-a", "lead-b", "nested-lead"} {
			if err := b.SetTeamAllowances(context.Background(), a.knownBusID(ids[role]), []bus.TeamAllowance{{Provider: product.Providers[0], Unit: "invented-turns", Allowance: 37}}); err != nil {
				return err
			}
			if err := b.SetGroupBudget(context.Background(), teams[role], 19); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.manifest = map[string]any{"product": product.Name, "chats": ids, "refs": refs, "teams": teams, "groups": groups, "workspaces": workspaces, "closed": ids["closed"]}
	f.manifest["open"] = openID
	expectedIDs := []string{}
	for _, role := range roles {
		expectedIDs = append(expectedIDs, ids[role])
	}
	f.manifest["expectedInitialInventoryIds"] = expectedIDs
	f.manifest["expectedGroups"] = expectedGroups
	f.manifest["expectedTeams"] = []map[string]any{
		{"id": "team:" + teams["lead-a"], "channelId": teams["lead-a"], "leadId": ids["lead-a"], "memberIds": []string{ids["member-a"], ids["member-b"], ids["nested-lead"]}},
		{"id": "team:" + teams["lead-b"], "channelId": teams["lead-b"], "leadId": ids["lead-b"], "memberIds": []string{ids["member-c"]}},
		{"id": "team:" + teams["nested-lead"], "channelId": teams["nested-lead"], "leadId": ids["nested-lead"], "memberIds": []string{ids["nested-member"]}},
	}
	f.seedDocuments(t, workspaces[0])
	a.mu.Lock()
	f.manifest["workspaces"] = append([]WorkspaceRecord(nil), a.settings.WorkspaceRecords...)
	a.mu.Unlock()
	// Materialize the seeded open chat's local observation before any receiving
	// snapshot. This is fixture setup, not a mutation outcome or provider query.
	if _, err := a.SavedChats(); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *orgChartHostFixture) installEvents(a *App) {
	a.sink = func(name string, value any) {
		data, err := json.Marshal(value)
		if err != nil {
			return
		}
		f.eventMu.Lock()
		f.events = append(f.events, map[string]any{"name": name, "data": json.RawMessage(data)})
		f.eventMu.Unlock()
	}
}

func orgChartFixture(a *App) (*orgChartHostFixture, error) {
	if f, ok := orgChartReceivingFixtures.Load(a); ok {
		return f.(*orgChartHostFixture), nil
	}
	return nil, errors.New("org chart receiving fixture is absent")
}

func (a *App) OrgChartHostFixtureManifest() (map[string]any, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.manifest, nil
}

func (f *orgChartHostFixture) records(relations bus.AcceptanceRelations) (map[string]any, error) {
	f.a.mu.Lock()
	settings, err := json.Marshal(f.a.settings)
	workspaceRecords := workspaces.Copy(f.a.settings.WorkspaceRecords)
	f.a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for name, path := range f.protected {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files[name] = string(data)
	}
	transcripts := map[string]string{}
	artifacts := map[string]map[string]any{}
	locations := map[string]map[string]string{}
	for _, id := range f.manifest["chats"].(map[string]string) {
		artifacts[id], locations[id] = map[string]any{}, map[string]string{}
		for kind, path := range map[string]string{"transcript": transcriptPath(id), "journal": journalPath(id), "queue": filepath.Join(configDir(), "queues", base64.RawURLEncoding.EncodeToString([]byte(id))+".json"), "log": f.a.chatLogs().Path(id)} {
			data, location, err := f.chatArtifact(id, path)
			if err != nil {
				return nil, err
			}
			locations[id][kind] = location
			artifacts[id][kind] = nil
			if data != nil {
				artifacts[id][kind] = string(data)
			}
			if kind == "transcript" {
				if data == nil {
					return nil, fmt.Errorf("fixture transcript is absent for %s", id)
				}
				transcripts[id] = string(data)
			}
		}
	}
	return map[string]any{"relations": relations, "settings": json.RawMessage(settings), "workspaces": workspaceRecords, "protectedRecords": files, "remoteWrites": f.document.writes.Load(), "transcripts": transcripts, "chatArtifacts": artifacts, "artifactLocations": locations}, nil
}

// chatArtifact follows the actual recycle manifest when binning moves a file.
// Bytes are compared across that intended move; raw locations retain the move.
func (f *orgChartHostFixture) chatArtifact(id, path string) ([]byte, string, error) {
	if path == "" {
		return nil, "absent", nil
	}
	data, err := os.ReadFile(path)
	if err == nil {
		return data, "active", nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	f.a.mu.Lock()
	removal := f.a.settings.RemovedChats[id]
	f.a.mu.Unlock()
	if removal.Group == "" {
		return nil, "absent", nil
	}
	archive, err := readChatCleanupArchive(removal.Group)
	if err != nil {
		return nil, "", err
	}
	for _, file := range archive.Files {
		if filepath.Join(configDir(), file.Path) == path {
			retained, err := cleanupLocalPath(chatCleanupArchiveDir(removal.Group), file.Slot)
			if err != nil {
				return nil, "", err
			}
			data, err := os.ReadFile(retained)
			return data, "retained:" + file.Slot, err
		}
	}
	return nil, "absent", nil
}

func (f *orgChartHostFixture) snapshot() (map[string]any, error) {
	var relations bus.AcceptanceRelations
	err := f.a.busHost().With(func(b *bus.Bus) error {
		var err error
		relations, err = b.AcceptanceRelations(context.Background())
		return err
	})
	if err != nil {
		return nil, err
	}
	out, err := f.records(relations)
	if err != nil {
		return nil, err
	}
	view, err := f.a.OrgChart()
	if err != nil {
		return nil, err
	}
	cleanup, err := f.a.ListChatCleanup()
	if err != nil {
		return nil, err
	}
	out["view"], out["cleanup"] = view, cleanup
	out["trace"] = f.a.StartupPageTrace()
	f.eventMu.Lock()
	out["events"] = append([]map[string]any{}, f.events...)
	f.eventMu.Unlock()
	out["observedCommittedSnapshots"] = append([]map[string]any{}, f.observed...)
	return out, nil
}

func (a *App) OrgChartHostFixtureSnapshot() (map[string]any, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshot()
}

func (a *App) OrgChartHostFixtureActions(id string) (OrgChartActionView, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return OrgChartActionView{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.a.OrgChartActions(id)
}

func (a *App) OrgChartHostFixtureApply(r OrgChartActionRequest) (OrgChartActionResult, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return OrgChartActionResult{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.barrier && r.Kind == "replace_team_lead" {
		if err := f.a.busHost().With(func(b *bus.Bus) error {
			before, err := b.AcceptanceRelations(context.Background())
			if err != nil {
				return err
			}
			records, err := f.records(before)
			if err != nil {
				return err
			}
			f.observed = append(f.observed, records)
			b.SetTeamLeadAcceptanceCheckpoint(func() error {
				observed, err := b.AcceptanceCommittedRelations(context.Background())
				if err != nil {
					return err
				}
				records, err := f.records(observed)
				if err == nil {
					f.observed = append(f.observed, records)
				}
				return err
			})
			return nil
		}); err != nil {
			return OrgChartActionResult{}, err
		}
		defer f.a.busHost().With(func(b *bus.Bus) error { b.SetTeamLeadAcceptanceCheckpoint(nil); return nil })
	}
	result, err := f.a.ApplyOrgChartAction(r)
	if err == nil && f.barrier && r.Kind == "replace_team_lead" {
		if e := f.a.busHost().With(func(b *bus.Bus) error {
			relations, e := b.AcceptanceRelations(context.Background())
			if e != nil {
				return e
			}
			records, e := f.records(relations)
			if e == nil {
				f.observed = append(f.observed, records)
			}
			return e
		}); e != nil {
			return result, e
		}
	}
	return result, err
}

type OrgChartFixtureControl struct {
	Kind   string   `json:"kind"`
	ChatID string   `json:"chatId"`
	LeadID string   `json:"leadId"`
	Title  string   `json:"title"`
	IDs    []string `json:"ids"`
	Count  int      `json:"count"`
}

func (a *App) OrgChartHostFixtureControl(r OrgChartFixtureControl) (any, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Kind {
	case "cleanup-store-unavailable":
		// Close the actual isolated database handle without replacing query
		// results. The existing restart control reopens the persisted store.
		return nil, f.a.busHost().With(func(b *bus.Bus) error { return b.Close() })
	case "cleanup-identity-changes":
		if r.Count < 0 || r.Count > 3 {
			return nil, errors.New("receiving accepts zero to three identity changes")
		}
		ids := f.manifest["chats"].(map[string]string)
		if r.ChatID == "" {
			r.ChatID = ids["free-a"]
		}
		known := false
		for _, id := range ids {
			known = known || id == r.ChatID
		}
		if !known {
			return nil, errors.New("identity control needs a fixture chat")
		}
		f.cleanupChanges, f.cleanupReads, f.cleanupChat = r.Count, 0, r.ChatID
		return map[string]int{"remaining": f.cleanupChanges, "reads": f.cleanupReads, "writes": f.cleanupWrites}, nil
	case "cleanup-read-observations":
		return map[string]int{"remaining": f.cleanupChanges, "reads": f.cleanupReads, "writes": f.cleanupWrites}, nil
	case "observe-swap":
		f.barrier = true
		f.observed = nil
		return nil, nil
	case "set-lead":
		return f.a.SetTeamLead(r.ChatID, r.LeadID)
	case "clear-lead":
		return nil, f.a.ClearTeamLead(r.ChatID)
	case "delete":
		return nil, f.a.DeleteSession(r.ChatID)
	case "bulk-delete":
		return f.a.RemoveChats(r.IDs), nil
	case "rename":
		return f.a.RenameSession(r.ChatID, r.Title)
	case "provider-live-update":
		s := f.a.session(r.ChatID)
		if s == nil {
			return nil, errors.New("live update needs the fixture's open conversation")
		}
		if _, err := f.a.RenameSession(r.ChatID, r.Title); err != nil {
			return nil, err
		}
		handler := f.a.newAgentHandler()
		for _, update := range []map[string]any{
			{"sessionUpdate": "session_info_update", "updatedAt": time.Now().UTC().Format(time.RFC3339Nano)},
			{"sessionUpdate": "async_task_spawned", "asyncTaskId": "invented-live-work", "name": "Invented live work", "state": "running"},
		} {
			startupCount(s.Provider(), "fixture/session/update")
			data, err := json.Marshal(map[string]any{"sessionId": r.ChatID, "update": update})
			if err != nil {
				return nil, err
			}
			handler.Notify("session/update", data)
		}
		f.a.sendChatStates()
		return s.InfoVersioned(), nil
	case "provider-mutation-positive-control":
		s := f.a.session(r.ChatID)
		if s == nil {
			return nil, errors.New("positive control needs the fixture open chat")
		}
		peer := newTestProviderConnection()
		conn := &startupCountedConnection{agentConnection: peer, provider: s.Provider()}
		f.a.mu.Lock()
		generation := newTestProvider(conn, acp.InitializeResult{})
		f.a.runtime.PutProviderLocked(s.Provider(), generation)
		f.a.mu.Unlock()
		// This injected connection has no process watcher. Its owner completes
		// the generation after closing the peer, before App shutdown can join it.
		defer func() { peer.Close(); generation.Finish() }()
		if err := f.a.setOptions(s, peer.options); err != nil {
			return nil, err
		}
		return f.a.SetSessionOption(r.ChatID, "model", json.RawMessage(`"changed"`))
	case "restart":
		f.a.shutdown(context.Background())
		openFolder()
		f.a = NewApp()
		f.a.ctx = context.Background()
		f.installEvents(f.a)
		return f.snapshot()
	default:
		return nil, errors.New("unknown receiving control")
	}
}

// OrgChartHostFixtureCleanup is ListChatCleanup with a deterministic real
// identity writer inside its validation boundary. It never manufactures a
// refusal. The page binding forwards its original read to this tagged port.
func (a *App) OrgChartHostFixtureCleanup() (ChatCleanupListing, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return ChatCleanupListing{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupReads++
	var writeErr error
	listing, err := f.a.listChatCleanupAtBoundary(func() {
		if f.cleanupChanges == 0 {
			return
		}
		f.cleanupChanges--
		f.cleanupWrites++
		_, writeErr = f.a.RenameSession(f.cleanupChat, fmt.Sprintf("Invented concurrent rename %d", f.cleanupWrites))
	})
	if writeErr != nil {
		return ChatCleanupListing{}, writeErr
	}
	return listing, err
}

func TestCleanupReceivingReadsRefuseActualIdentityWrites(t *testing.T) {
	orgChartForbidProviderStarts(t)
	a := newFileAccessFixtureApp(t)
	a.ctx = context.Background()
	f := newOrgChartHostFixture(t, a)
	for _, count := range []int{1, 3} {
		if _, err := a.OrgChartHostFixtureControl(OrgChartFixtureControl{Kind: "cleanup-identity-changes", Count: count}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < count; i++ {
			if _, err := a.OrgChartHostFixtureCleanup(); err == nil || err.Error() != "Chat inventory changed while cleanup was read. Refresh the list." {
				t.Fatal("real rename did not produce the actual consistency refusal", err)
			}
		}
		listing, err := a.OrgChartHostFixtureCleanup()
		if err != nil || len(listing.Rows) == 0 {
			t.Fatal("read did not recover after the writes stopped", err)
		}
		if f.cleanupReads != count+1 || f.cleanupChanges != 0 {
			t.Fatal("read counters lost actual attempts")
		}
	}
}

func TestCleanupReceivingInitialStoreFailureAndRestart(t *testing.T) {
	orgChartForbidProviderStarts(t)
	a := newFileAccessFixtureApp(t)
	a.ctx = context.Background()
	f := newOrgChartHostFixture(t, a)
	before, err := f.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.OrgChartHostFixtureControl(OrgChartFixtureControl{Kind: "cleanup-store-unavailable"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OrgChartHostFixtureCleanup(); err == nil || err.Error() != "sql: database is closed" {
		t.Fatal("closed actual store did not refuse the read", err)
	}
	if _, err := a.OrgChartHostFixtureControl(OrgChartFixtureControl{Kind: "restart"}); err != nil {
		t.Fatal(err)
	}
	listing, err := a.OrgChartHostFixtureCleanup()
	if err != nil || len(listing.Rows) == 0 {
		t.Fatal("Refresh could not read the reopened store", err)
	}
	after, err := f.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before["protectedRecords"], after["protectedRecords"]) {
		t.Fatal("store fault changed protected records")
	}
}

func (f *orgChartHostFixture) seedDocuments(t *testing.T, workspace WorkspaceRecord) {
	t.Helper()
	orgChartDocumentRegistration.Do(func() {
		workspaces.RegisterSourceKind("org-chart-document-peer", workspaces.SourceKind{Provider: func(_ workspaces.Resolver, _ context.Context, source workspaces.Source) (any, error) {
			if peer, ok := orgChartDocumentPeers.Load(source.ID); ok {
				return peer, nil
			}
			return nil, errors.New("invented document peer is absent")
		}})
	})
	f.document = &orgChartDocumentPeer{path: filepath.Join(t.TempDir(), "remote.md")}
	local := filepath.Join(workspace.ExecutionPath, "protected.md")
	for _, path := range []string{local, f.document.path} {
		if err := os.WriteFile(path, []byte("# Invented protected document\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := "org-doc-" + filepath.Base(filepath.Dir(f.document.path))
	orgChartDocumentPeers.Store(source, f.document)
	t.Cleanup(func() { orgChartDocumentPeers.Delete(source) })
	if err := f.a.updateSettings(func(s *Settings) error {
		s.WorkspaceRecords[0].Sources = append(s.WorkspaceRecords[0].Sources, WorkspaceSource{ID: source, Kind: "org-chart-document-peer", Name: "Invented remote documents"})
		return ensureWorkspaceMountAliases(s.WorkspaceRecords)
	}); err != nil {
		t.Fatal(err)
	}
	f.protected["local-document"], f.protected["remote-document"] = local, f.document.path
	for _, ref := range []ResourceRef{{WorkspaceID: workspace.ID, SourceID: workspace.PrimarySourceID, Path: "protected.md"}, {WorkspaceID: workspace.ID, SourceID: source, Path: "protected-page"}} {
		draft, err := f.a.OpenDraft(ref)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.a.UpdateDraft(draft.ID, draft.Revision, "# Invented unsaved work\n"); err != nil {
			t.Fatal(err)
		}
		f.protected["draft-"+ref.SourceID] = filepath.Join(configDir(), "drafts", draft.ID+".json")
		if ref.SourceID == source {
			f.manifest["remoteDraftId"] = draft.ID
		}
	}
}

func TestOrgChartHostFixtureActualSwapBarrierAndRestart(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a := newFileAccessFixtureApp(t)
	a.ctx = context.Background()
	f := newOrgChartHostFixture(t, a)
	ids := f.manifest["chats"].(map[string]string)
	teams := f.manifest["teams"].(map[string]string)
	live, err := a.OrgChartHostFixtureControl(OrgChartFixtureControl{Kind: "provider-live-update", ChatID: ids["free-a"], Title: "Invented changed title with enough words to clip in a narrow card"})
	if err != nil || live.(SessionInfo).UpdatedAt == "2026-11-15T00:00:00Z" || len(live.(SessionInfo).Background) != 1 {
		t.Fatal("normal provider handler did not update state and time", live, err)
	}
	view, err := a.OrgChart()
	if err != nil {
		t.Fatal(err)
	}
	foundLive := false
	for _, chat := range view.Chats {
		if chat.ChatID == ids["free-a"] {
			foundLive = true
			if chat.UpdatedAt != live.(SessionInfo).UpdatedAt || chat.State == nil || chat.State.Exec == nil || chat.State.Exec.State != execBackground {
				t.Fatal("OrgChart omitted the actual provider update", chat, live)
			}
		}
	}
	if !foundLive {
		t.Fatal("live control changed inventory membership")
	}
	before, err := a.OrgChartHostFixtureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(before["relations"].(bus.AcceptanceRelations).Rooms) != 5 {
		t.Fatal("expected three team channels and two ordinary groups", before["relations"])
	}
	if _, err := a.OrgChartHostFixtureControl(OrgChartFixtureControl{Kind: "observe-swap"}); err != nil {
		t.Fatal(err)
	}
	action := orgAction(t, a, ids["member-a"], "replace_team_lead")
	result, err := a.OrgChartHostFixtureApply(OrgChartActionRequest{Kind: "replace_team_lead", ChatID: ids["member-a"], GroupID: teams["lead-a"], Token: action.Token})
	if err != nil || result.Status != "succeeded" {
		t.Fatal(result, err)
	}
	after, err := a.OrgChartHostFixtureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(f.observed) != 6 {
		t.Fatal("checkpoint did not observe each real write", len(f.observed))
	}
	for i, observed := range f.observed {
		want := before["relations"]
		if i == 5 {
			want = after["relations"]
		}
		if !reflect.DeepEqual(observed["relations"], want) {
			t.Fatalf("checkpoint %d saw a partial commit: %+v", i, observed["relations"])
		}
	}
	if !reflect.DeepEqual(before["protectedRecords"], after["protectedRecords"]) || f.document.writes.Load() != 0 {
		t.Fatal("team mutation changed protected records")
	}
	restarted, err := a.OrgChartHostFixtureControl(OrgChartFixtureControl{Kind: "restart"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restarted.(map[string]any)["relations"], after["relations"]) {
		t.Fatal("restart changed committed relations")
	}
	if starts.Load() != 0 {
		t.Fatal("local fixture contacted a provider", starts.Load())
	}
	// A refused write proves the counter observes the real resource boundary.
	draft, err := f.a.GetDraft(f.manifest["remoteDraftId"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.a.SaveDraft(draft.ID, draft.Revision); err == nil || f.document.writes.Load() != 1 {
		t.Fatal("remote write positive control was not observed", err, f.document.writes.Load())
	}
}

// TestOrgChartHostFixtureRPC supplies an isolated line protocol for checking the
// host binding without a browser. Rendered observations remain Front End's gate.
func TestOrgChartHostFixtureRPC(t *testing.T) {
	if os.Getenv("DINAH_ORG_FIXTURE_RPC") != "1" {
		t.Skip("explicit isolated binding runner only")
	}
	orgChartForbidProviderStarts(t)
	a := newFileAccessFixtureApp(t)
	a.ctx = context.Background()
	newOrgChartHostFixture(t, a)
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		var request struct {
			ID   int               `json:"id"`
			Name string            `json:"name"`
			Args []json.RawMessage `json:"args"`
		}
		if err := json.Unmarshal(scan.Bytes(), &request); err != nil {
			t.Fatal(err)
		}
		response := map[string]any{"id": request.ID}
		method := reflect.ValueOf(a).MethodByName(request.Name)
		if !method.IsValid() {
			response["error"] = "unknown fixture method"
		} else {
			args := []reflect.Value{}
			for i, raw := range request.Args {
				value := reflect.New(method.Type().In(i))
				if err := json.Unmarshal(raw, value.Interface()); err != nil {
					t.Fatal(err)
				}
				args = append(args, value.Elem())
			}
			values := method.Call(args)
			if len(values) > 1 && !values[len(values)-1].IsNil() {
				response["error"] = values[len(values)-1].Interface().(error).Error()
			} else if len(values) > 0 {
				response["value"] = values[0].Interface()
			}
		}
		data, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("ORG_REPLY " + string(data))
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
}
