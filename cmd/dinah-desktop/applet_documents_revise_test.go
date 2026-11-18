package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"studio/internal/applet"
	"studio/internal/documents"
	"studio/internal/drafts"
)

const reviseTestText = "# Harbor\n\nThe docks open at six.\n\nThe checkout closes at noon.\n\nKeyboards are on loan.\n"

// reviseTestApp is a workspace with plan.md and two open chats of it,
// Harbor Review and Docks Review, the first with an identity on the bus and
// the second without one. It returns the app, the workspace, the Documents
// actions and the events the page is told.
func reviseTestApp(t *testing.T) (*App, string, *applet.Registry, func() []any) {
	t.Helper()
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(reviseTestText), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetWorkspace(path); err != nil {
		t.Fatal(err)
	}
	for id, title := range map[string]string{"codex:harbor": "Harbor Review", "codex:docks": "Docks Review"} {
		a.runtime.PutConversationLocked(id, newSessionState(&sessionSeed{ID: id, Provider: "codex", Cwd: path, Title: title}, sessionInitial{Open: true}))
	}
	a.saveBusSession("codex:harbor", "window-harbor")
	var mu sync.Mutex
	var events []any
	a.sink = func(name string, data any) {
		if name == "draft:changed" {
			mu.Lock()
			events = append(events, data)
			mu.Unlock()
		}
	}
	bundle, err := a.documentsAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	return a, path, r, func() []any {
		mu.Lock()
		defer mu.Unlock()
		out := events
		events = nil
		return out
	}
}

func chatContext(path, chat string) context.Context {
	return applet.WithInvocation(context.Background(), applet.Invocation{Workspace: path, OwnerKind: "chat", OwnerID: chat, ID: "call-" + chat})
}

type revisedOut struct {
	Withdrawn   int             `json:"withdrawn"`
	Suggestions []suggestionOut `json:"suggestions"`
}

func runDocuments(t *testing.T, r *applet.Registry, ctx context.Context, action string, in map[string]any) (revisedOut, error) {
	t.Helper()
	data, _ := json.Marshal(in)
	res, err := r.Execute(ctx, action, string(data))
	if err != nil {
		return revisedOut{}, err
	}
	var out revisedOut
	if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
		t.Fatalf("%s answered %s: %v", action, res.OutputJSON, err)
	}
	return out, nil
}

// suggestAs proposes the three paragraphs changed as the chat given, and
// returns the suggestions it added.
func suggestAs(t *testing.T, r *applet.Registry, path, chat, proposed string) []suggestionOut {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"path": "plan.md", "base": reviseTestText, "text": proposed, "reason": "clearer"})
	res, err := r.Execute(chatContext(path, chat), documentsSuggestAction, string(data))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Suggestions []suggestionOut `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
		t.Fatal(err)
	}
	return out.Suggestions
}

func readSuggestions(t *testing.T, r *applet.Registry, path string) (string, []suggestionOut) {
	t.Helper()
	res, err := r.Execute(chatContext(path, "codex:harbor"), documentsReadAction, `{"path":"plan.md"}`)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Text        string          `json:"text"`
		Suggestions []suggestionOut `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
		t.Fatal(err)
	}
	return out.Text, out.Suggestions
}

func toldOfSuggestions(t *testing.T, events []any) {
	t.Helper()
	for _, e := range events {
		if c, ok := e.(documents.Change); ok && c.ID != "" && c.Kind == "suggested" {
			return
		}
	}
	t.Fatalf("the document's window was not told: %+v", events)
}

// A chat revises and withdraws some of its own suggestions; the rest of its
// batch, the other chat's suggestion and the document's text stay as they
// were, the person's mark of an edit is not set, and a window showing the
// document is told each time.
func TestAChatRevisesAndWithdrawsItsOwnSuggestions(t *testing.T) {
	a, path, r, told := reviseTestApp(t)
	mine := suggestAs(t, r, path, "codex:harbor", "# Harbor\n\nThe docks open at seven.\n\nThe checkout closes at one.\n\nKeyboards are for sale.\n")
	theirs := suggestAs(t, r, path, "codex:docks", "# Harbor\n\nThe docks open at six.\n\nThe checkout closes at noon.\n\nKeyboards are on loan until Friday.\n")
	if len(mine) != 3 || len(theirs) != 1 || mine[0].Author != "Harbor Review" {
		t.Fatalf("suggested %+v and %+v", mine, theirs)
	}
	told()
	out, err := runDocuments(t, r, chatContext(path, "codex:harbor"), documentsReviseAction, map[string]any{
		"path": "plan.md", "reason": "The checkout hour was wrong.",
		"suggestions": []map[string]string{{"id": mine[1].ID, "after": "The checkout closes at two."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Suggestions) != 1 || out.Suggestions[0].After != "The checkout closes at two.\n" || out.Suggestions[0].Edited || out.Suggestions[0].Reason != "The checkout hour was wrong." || out.Suggestions[0].ID != mine[1].ID {
		t.Fatalf("revised %+v", out.Suggestions)
	}
	toldOfSuggestions(t, told())
	text, all := readSuggestions(t, r, path)
	if text != reviseTestText || len(all) != 4 {
		t.Fatalf("after revising: %q %+v", text, all)
	}
	for _, s := range all {
		if s.ID == mine[0].ID && (s.After != mine[0].After || s.Reason != "clearer") || s.ID == theirs[0].ID && s.After != theirs[0].After {
			t.Fatalf("a suggestion not chosen changed: %+v", s)
		}
	}
	out, err = runDocuments(t, r, chatContext(path, "codex:harbor"), documentsWithdrawAction, map[string]any{"path": "plan.md", "ids": []string{mine[2].ID}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Withdrawn != 1 || len(out.Suggestions) != 3 {
		t.Fatalf("withdrew %+v", out)
	}
	for _, s := range out.Suggestions {
		if s.ID == mine[2].ID {
			t.Fatal("the withdrawn suggestion remains")
		}
	}
	toldOfSuggestions(t, told())
	if data, _ := os.ReadFile(filepath.Join(path, "plan.md")); string(data) != reviseTestText {
		t.Fatalf("the file changed: %q", data)
	}
	// The chat that has no identity on the bus is known by its session.
	if _, err := runDocuments(t, r, chatContext(path, "codex:docks"), documentsWithdrawAction, map[string]any{"path": "plan.md", "ids": []string{theirs[0].ID}}); err != nil {
		t.Fatal(err)
	}
	// The chat continues in another session, and its suggestions stay its
	// own, since its identity on the bus goes with it.
	a.runtime.PutConversationLocked("codex:harbor-2", newSessionState(&sessionSeed{ID: "codex:harbor-2", Provider: "codex", Cwd: path, Title: "Harbor Review"}, sessionInitial{Open: true}))
	if err := a.moveBusSession("codex:harbor", "codex:harbor-2", "window-harbor"); err != nil {
		t.Fatal(err)
	}
	if _, err := runDocuments(t, r, chatContext(path, "codex:harbor-2"), documentsWithdrawAction, map[string]any{"path": "plan.md", "ids": []string{mine[0].ID}}); err != nil {
		t.Fatalf("the continued chat withdrew its own suggestion: %v", err)
	}
}

// Another chat's suggestion, one the person edited and an unknown ID are each
// refused with the reason, and a request with any of them changes nothing,
// although its other suggestion is the caller's.
func TestAChatsCorrectionIsRefusedAsAWhole(t *testing.T) {
	a, path, r, told := reviseTestApp(t)
	mine := suggestAs(t, r, path, "codex:harbor", "# Harbor\n\nThe docks open at seven.\n\nThe checkout closes at one.\n\nKeyboards are on loan.\n")
	theirs := suggestAs(t, r, path, "codex:docks", "# Harbor\n\nThe docks open at six.\n\nThe checkout closes at noon.\n\nKeyboards are for sale.\n")
	if len(mine) != 2 || len(theirs) != 1 {
		t.Fatalf("suggested %+v and %+v", mine, theirs)
	}
	// The person edits the second of Harbor Review's suggestions.
	var draftID string
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		t.Fatal(err)
	}
	list, err := o.Drafts.Store.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range list {
		if d.Origin.Path == "plan.md" {
			draftID = d.ID
		}
	}
	if _, err := a.ReviseSuggestion(draftID, mine[1].ID, "The checkout closes at three.\n"); err != nil {
		t.Fatal(err)
	}
	_, before := readSuggestions(t, r, path)
	told()
	cases := []struct{ name, id, says string }{
		{"another chat's", theirs[0].ID, "another chat made this suggestion"},
		{"edited by the person", mine[1].ID, "ask the person"},
		{"unknown", "0123456789abcdef", "no longer there"},
	}
	for _, c := range cases {
		_, err := runDocuments(t, r, chatContext(path, "codex:harbor"), documentsReviseAction, map[string]any{
			"path": "plan.md", "reason": "changed",
			"suggestions": []map[string]string{{"id": mine[0].ID, "after": "The docks open at eight.\n"}, {"id": c.id, "after": "Changed.\n"}},
		})
		if err == nil || !strings.Contains(err.Error(), c.says) || !strings.Contains(err.Error(), c.id) || !strings.Contains(err.Error(), "nothing was changed") {
			t.Fatalf("revising %s: %v", c.name, err)
		}
		_, err = runDocuments(t, r, chatContext(path, "codex:harbor"), documentsWithdrawAction, map[string]any{"path": "plan.md", "ids": []string{mine[0].ID, c.id}})
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Fatalf("withdrawing %s: %v", c.name, err)
		}
		_, after := readSuggestions(t, r, path)
		if len(after) != len(before) {
			t.Fatalf("a refused request (%s) removed a suggestion: %+v", c.name, after)
		}
		for i := range before {
			if after[i] != before[i] {
				t.Fatalf("a refused request (%s) changed %+v to %+v", c.name, before[i], after[i])
			}
		}
	}
	if events := told(); len(events) != 0 {
		t.Fatalf("a refused request told the window of a change: %+v", events)
	}
	// A call from no chat may change no suggestion.
	if _, err := runDocuments(t, r, applet.WithInvocation(context.Background(), applet.Invocation{Workspace: path, ID: "call"}), documentsWithdrawAction, map[string]any{"path": "plan.md", "ids": []string{mine[0].ID}}); err == nil {
		t.Fatal("a call from no chat withdrew a suggestion")
	}
}

// A suggestion made before owners were recorded is the calling chat's when
// its author is the chat's title as it is now, and no other chat's. A chat
// renamed since, a chat of another folder, and a suggestion no chat made are
// refused.
func TestASuggestionWithNoOwnerIsTheChatsOfItsTitle(t *testing.T) {
	a, path, r, _ := reviseTestApp(t)
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		t.Fatal(err)
	}
	_, record, err := a.mainFolder(chatContext(path, "codex:harbor"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := o.Open(ResourceRef{WorkspaceID: record.ID, SourceID: record.PrimarySourceID, Path: "plan.md"})
	if err != nil {
		t.Fatal(err)
	}
	propose := func(author, proposed string) string {
		_, added, err := o.Suggest(v.ID, reviseTestText, proposed, drafts.Proposal{Author: author})
		if err != nil || len(added) != 1 || added[0].Owner != "" {
			t.Fatalf("proposed %+v %v", added, err)
		}
		return added[0].ID
	}
	old := propose("Harbor Review", "# Harbor\n\nThe docks open at seven.\n\nThe checkout closes at noon.\n\nKeyboards are on loan.\n")
	nobody := propose(unknownSuggestionAuthor, "# Harbor\n\nThe docks open at six.\n\nThe checkout closes at one.\n\nKeyboards are on loan.\n")
	withdraw := func(chat, id string) error {
		_, err := runDocuments(t, r, chatContext(path, chat), documentsWithdrawAction, map[string]any{"path": "plan.md", "ids": []string{id}})
		return err
	}
	if err := withdraw("codex:docks", old); err == nil {
		t.Fatal("another chat withdrew a suggestion with no owner")
	}
	// A chat titled as the author but working in another folder.
	a.runtime.PutConversationLocked("codex:elsewhere", newSessionState(&sessionSeed{ID: "codex:elsewhere", Provider: "codex", Cwd: t.TempDir(), Title: "Harbor Review"}, sessionInitial{Open: true}))
	if err := withdraw("codex:elsewhere", old); err == nil {
		t.Fatal("a chat of another folder withdrew a suggestion with no owner")
	}
	// A chat titled as no chat's suggestion is named.
	a.runtime.PutConversationLocked("codex:agent", newSessionState(&sessionSeed{ID: "codex:agent", Provider: "codex", Cwd: path, Title: unknownSuggestionAuthor}, sessionInitial{Open: true}))
	if err := withdraw("codex:agent", nobody); err == nil {
		t.Fatal("a chat withdrew a suggestion that no chat made")
	}
	// The chat renamed by the person since is no longer the author.
	a.mu.Lock()
	if a.settings.SessionTitles == nil {
		a.settings.SessionTitles = map[string]string{}
	}
	a.settings.SessionTitles["codex:harbor"] = "Harbor Copy"
	a.mu.Unlock()
	if err := withdraw("codex:harbor", old); err == nil {
		t.Fatal("a renamed chat withdrew a suggestion of its old title")
	}
	a.mu.Lock()
	delete(a.settings.SessionTitles, "codex:harbor")
	a.mu.Unlock()
	out, err := runDocuments(t, r, chatContext(path, "codex:harbor"), documentsReviseAction, map[string]any{
		"path": "plan.md", "suggestions": []map[string]string{{"id": old, "after": "The docks open at eight.\n"}},
	})
	if err != nil || len(out.Suggestions) != 1 || out.Suggestions[0].After != "The docks open at eight.\n" {
		t.Fatalf("the chat of the author's title revised: %+v %v", out, err)
	}
	if err := withdraw("codex:harbor", old); err != nil {
		t.Fatalf("the chat of the author's title withdrew: %v", err)
	}
}

// Where a product's agents propose, Revise and Withdraw are offered beside
// Read and Suggest, ask the person nothing, and the applications' notes tell
// an agent of them.
func TestReviseAndWithdrawAreOfferedWhereAgentsPropose(t *testing.T) {
	previous := product
	product.AgentsProposeDocumentChanges = true
	t.Cleanup(func() { product = previous })
	a := NewApp()
	bundles, err := a.builtinAppletBundles()
	if err != nil {
		t.Fatal(err)
	}
	offered := map[string]applet.Action{}
	notes := map[string]string{}
	for _, b := range bundles {
		for _, act := range b.Manifest.Actions {
			offered[act.Ref] = act
		}
		notes[b.Manifest.ID] = b.Manifest.Description
	}
	for _, ref := range []string{documentsReadAction, documentsSuggestAction, documentsReviseAction, documentsWithdrawAction} {
		act, ok := offered[ref]
		if !ok {
			t.Fatalf("%s is not offered where agents propose", ref)
		}
		if applet.Asks(applet.Action{Capabilities: act.Capabilities}) {
			t.Fatalf("%s asks first, though it changes nothing until the person accepts", ref)
		}
	}
	if d := offered[documentsSuggestAction].Description; !strings.Contains(d, "Revise Suggestions") || !strings.Contains(d, "Withdraw Suggestions") {
		t.Fatalf("Suggest Changes does not tell of the corrections: %q", d)
	}
	told := 0
	for id, note := range notes {
		if strings.Contains(note, proposeNote) {
			told++
			if !strings.Contains(note, documentsReviseAction) || !strings.Contains(note, documentsWithdrawAction) {
				t.Fatalf("%s's note does not tell of revise and withdraw: %q", id, note)
			}
		}
	}
	if told == 0 {
		t.Fatal("no application's note tells an agent to propose")
	}
}
