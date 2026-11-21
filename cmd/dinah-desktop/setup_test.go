package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"studio/internal/codexagent"
	"studio/internal/profile"
	"studio/internal/providers"
)

// The set-up sets what was answered and Undo puts each of those back: the
// agent, what it asks, the name, and the workspace, which is removed again
// when the set-up made it. Judgements are turned off by Undo, since their
// question cannot be made unanswered.
func TestTheSetUpSetsWhatWasAnsweredAndUndoPutsItBack(t *testing.T) {
	a, _, _ := judgementApp(t)
	folder := t.TempDir()
	if a.SetupAsking("codex") == nil || a.SetupAsking("codex").Current != codexagent.ApprovalsAsk {
		t.Fatalf("what Codex asks at the start: %+v", a.SetupAsking("codex"))
	}
	if err := a.UndoSetup(); err == nil {
		t.Fatal("Undo with no set-up before it was not refused")
	}
	a.mu.Lock()
	first := a.settings.DefaultProvider
	a.mu.Unlock()
	if first == "claude" {
		t.Fatal("the fixture already has the agent this test sets")
	}
	if err := a.ApplySetup(SetupChoices{Agent: "claude", Asking: "nonsense"}); err == nil {
		t.Fatal("a choice the agent does not offer was taken")
	}
	a.mu.Lock()
	kept := a.settings.DefaultProvider
	a.mu.Unlock()
	if kept != first {
		t.Fatalf("a set-up that failed left the agent set: %q", kept)
	}
	err := a.ApplySetup(SetupChoices{Agent: "codex", Asking: codexagent.ApprovalsWorkspace, Folder: folder, Name: "Sam", SetName: true, Judge: true})
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	s := a.settings
	a.mu.Unlock()
	snap, _ := a.JudgementSettings()
	if s.DefaultProvider != "codex" || codexApprovals(s) != codexagent.ApprovalsWorkspace || !sameFolder(s.Workspace, folder) || a.chosenPersonName() != "Sam" || !snap.Settings.Enabled || snap.Settings.Setup != "on" {
		t.Fatalf("after the set-up: provider %q, approvals %q, workspace %q, name %q, judgements %+v", s.DefaultProvider, s.CodexApprovals, s.Workspace, a.chosenPersonName(), snap.Settings)
	}
	if err := a.UndoSetup(); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	s = a.settings
	a.mu.Unlock()
	snap, _ = a.JudgementSettings()
	if s.DefaultProvider != first || codexApprovals(s) != codexagent.ApprovalsAsk || sameFolder(s.Workspace, folder) || len(s.WorkspaceRecords) != 0 || a.chosenPersonName() != "" || snap.Settings.Enabled {
		t.Fatalf("after Undo: provider %q, approvals %q, workspace %q (%d records), name %q, judgements on %v", s.DefaultProvider, s.CodexApprovals, s.Workspace, len(s.WorkspaceRecords), a.chosenPersonName(), snap.Settings.Enabled)
	}
	if err := a.UndoSetup(); err == nil {
		t.Fatal("a second Undo was not refused")
	}
}

// A folder that is a workspace already is only put in front: no second
// workspace is made of it, and Undo puts the earlier one in front again and
// removes neither.
func TestTheSetUpPutsAFolderThatIsAWorkspaceAlreadyInFront(t *testing.T) {
	a, _, _ := judgementApp(t)
	known, front := t.TempDir(), t.TempDir()
	for _, folder := range []string{known, front} {
		if _, err := a.SetWorkspace(folder); err != nil {
			t.Fatal(err)
		}
	}
	if offer, err := a.SetupOffer(); err != nil || offer.Workspaces != 2 {
		t.Fatalf("the offer counts %d workspaces: %v", offer.Workspaces, err)
	}
	a.mu.Lock()
	agent := defaultProviderOf(a.settings)
	a.mu.Unlock()
	if err := a.ApplySetup(SetupChoices{Agent: agent, Folder: known}); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	s := a.settings
	a.mu.Unlock()
	if !sameFolder(s.Workspace, known) || len(s.WorkspaceRecords) != 2 {
		t.Fatalf("after the set-up: workspace %q, %d records", s.Workspace, len(s.WorkspaceRecords))
	}
	if err := a.UndoSetup(); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	s = a.settings
	a.mu.Unlock()
	if !sameFolder(s.Workspace, front) || len(s.WorkspaceRecords) != 2 {
		t.Fatalf("after Undo: workspace %q, %d records", s.Workspace, len(s.WorkspaceRecords))
	}
}

func sameFolder(a, b string) bool {
	x, _ := filepath.EvalSymlinks(a)
	y, _ := filepath.EvalSymlinks(b)
	return x != "" && filepath.Clean(x) == filepath.Clean(y)
}

// The set-up opens by itself once, and the judgements' own flyout then does
// not open by itself as well.
func TestTheSetUpNotesThatItOpenedByItself(t *testing.T) {
	a, _, _ := judgementApp(t)
	if err := a.NoteSetupShown(); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	shown := a.settings.SetupShown
	a.mu.Unlock()
	snap, _ := a.JudgementSettings()
	if !shown || !snap.Settings.SetupShown {
		t.Fatalf("shown %v, the judgements' question shown %v", shown, snap.Settings.SetupShown)
	}
}

// A person's own words are read by the chosen agent, and only answers an
// open question can take are kept. When the agent does not answer, the
// words are matched.
func TestTheSetUpReadsAPersonsOwnWords(t *testing.T) {
	a, _, _ := judgementApp(t)
	folder := t.TempDir()
	open := []SetupQuestion{
		{ID: "use", Choices: []SetupChoice{{"code", "Writing code with an agent"}, {"docs", "Writing documents and diagrams"}}},
		{ID: "ask", Choices: []SetupChoice{{"ask", "Ask Before Commands"}, {"full", "Full Access, Unattended"}}},
		{ID: "folder"}, {ID: "name"}, {ID: "judge"},
	}
	old := setupRead
	defer func() { setupRead = old }()
	setupRead = func(_ *App, _ context.Context, provider, input string) (string, error) {
		if provider != "codex" {
			t.Errorf("the words went to %q", provider)
		}
		return "```json\n{\"use\":\"docs\",\"ask\":\"everything\",\"folder\":" + quoted(folder) + ",\"judge\":\"maybe\",\"other\":\"x\"}\n```", nil
	}
	got, err := a.ReadSetupWords("I write requirement documents", "codex", open)
	if err != nil || got.Used != "model" || len(got.Answers) != 2 || got.Answers["use"] != "docs" || got.Answers["folder"] != folder {
		t.Fatalf("read by the agent: %+v %v", got, err)
	}
	setupRead = func(*App, context.Context, string, string) (string, error) { return "", errors.New("no answer") }
	got, err = a.ReadSetupWords("I write code in "+folder+", call me Sam Lee, ask before commands, and yes to the background", "codex", open)
	want := map[string]string{"use": "code", "ask": "ask", "folder": folder, "name": "Sam Lee", "judge": "on"}
	if err != nil || got.Used != "rule" || len(got.Answers) != len(want) {
		t.Fatalf("read by matching: %+v %v", got, err)
	}
	for k, v := range want {
		if got.Answers[k] != v {
			t.Errorf("%s is %q, want %q", k, got.Answers[k], v)
		}
	}
	// A question that is not open is not answered.
	got, _ = a.ReadSetupWords("call me Sam", "codex", []SetupQuestion{{ID: "judge"}})
	if len(got.Answers) != 0 {
		t.Fatalf("an answer to a closed question: %+v", got)
	}
	if _, err := a.ReadSetupWords("  ", "codex", open); err == nil {
		t.Fatal("empty words were read")
	}
}

func quoted(s string) string {
	out := `"`
	for _, r := range s {
		if r == '\\' || r == '"' {
			out += `\`
		}
		out += string(r)
	}
	return out + `"`
}

// Every provider the product offers has a page that says how to install
// it, and a product's own page for one takes its maker's place.
func TestEveryProviderHasAnInstallPage(t *testing.T) {
	for _, id := range profile.Source().Providers {
		if providers.Registry[id].InstallPage == "" {
			t.Errorf("%s has no install page", id)
		}
	}
	p := profile.Profile{InstallPages: map[string]string{"codex": "https://example.test/install"}}
	if got := installPage("codex", p); got != "https://example.test/install" {
		t.Errorf("the product's own page was not used: %q", got)
	}
	if got := installPage("ollama", p); got != providers.Registry["ollama"].InstallPage {
		t.Errorf("the maker's page was not used: %q", got)
	}
}

// The uses the set-up offers are its own unless the product gives others,
// and each opens windows.
func TestTheSetUpsUsesOpenWindows(t *testing.T) {
	if got := setupUsesOf(profile.Profile{SetupAsksNoUse: true}); len(got) != 0 {
		t.Fatalf("a product that leaves the question out was given uses: %+v", got)
	}
	if got := setupUsesOf(profile.Profile{SetupUses: []profile.SetupUse{{ID: "own", Label: "Own", Windows: []string{"files"}, Opens: "Files"}}}); len(got) != 1 || got[0].ID != "own" {
		t.Fatalf("a product's own uses were not offered: %+v", got)
	}
	uses := offeredSetupUses()
	if len(uses) == 0 {
		t.Fatal("no uses")
	}
	for _, u := range uses {
		if u.ID == "" || u.Label == "" || u.Opens == "" || len(u.Windows) == 0 {
			t.Errorf("an incomplete use: %+v", u)
		}
	}
}
