package main

import (
	"context"
	"html"
	"strings"

	"studio/internal/documents"
	"studio/internal/drafts"
	"studio/internal/preview"
	"studio/internal/recent"
	"studio/internal/resource/localfs"
	"studio/internal/workspaces"
)

// The Editor's drafts, through package documents. This file binds them to
// the app's state folder, registry and events, and adds what only the Editor
// does: previewing a draft.

type (
	DraftChange = documents.Change
	DraftSaved  = documents.Saved
)

func draftErrorText(err error) error { return documents.ErrorText(err) }

func (a *App) draftService() (*drafts.Service, error) {
	return documents.Service(stateLayout(), a.resourceService().resolver())
}

// documentOps are the document operations bound to the app.
type documentOps struct {
	documents.Ops
	app *App
}

func (a *App) documentOps(ctx context.Context, workspace string) (documentOps, error) {
	svc, err := a.draftService()
	if err != nil {
		return documentOps{}, err
	}
	s := a.resourceService()
	s.ctx = ctx
	return documentOps{Ops: documents.Ops{Ctx: ctx, Workspace: workspace, Drafts: svc, Resolver: s.resolver(), Emit: a.emit}, app: a}, nil
}

func (a *App) bindingContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// DraftPreview is a draft's text rendered as the resource preview renders it.
type DraftPreview struct {
	Revision string          `json:"revision"`
	Blocks   []preview.Block `json:"blocks"`
	Problems []Problem       `json:"problems"`
}

// preview renders a draft's current text. Only Markdown is rendered; other
// text is shown as it is.
func (o documentOps) preview(id string) (DraftPreview, error) {
	d, err := o.Draft(id)
	if err != nil {
		return DraftPreview{}, err
	}
	out := DraftPreview{Revision: d.Revision, Problems: []Problem{}}
	if d.Origin.Kind == "local-folder" && localfs.Format(d.Origin.Path) != "markdown" {
		out.Blocks = []preview.Block{{Line: 1, HTML: "<pre><code>" + html.EscapeString(d.Text) + "</code></pre>"}}
		return out, nil
	}
	out.Blocks, out.Problems = markdownPreview(d.Text)
	// A skill's problems come with its preview, so the Editor's problems
	// pane has them wherever SKILL.md is edited.
	if folder, ok := skillOf(d.Origin); ok {
		out.Problems = append(out.Problems, skillProblemsOf(o.app.skillProblems(d, folder))...)
	}
	return out, nil
}

// OpenDraft opens a workspace resource for editing: its draft in progress, or
// a new draft from the resource as it is now. The person opens documents
// through it, so the document is put at the top of Documents › Recent.
func (a *App) OpenDraft(ref ResourceRef) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	v, err := o.Open(ref)
	if err == nil {
		a.recordOpened(v)
	}
	return v, err
}

// GetDraft reads a draft.
func (a *App) GetDraft(id string) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	return o.Get(id)
}

// DraftStamp says, in a few bytes, which state a draft is in: its revision,
// its baseline's and its suggestions. An Editor compares it every few
// seconds to notice a change made by another program, and reads the whole
// draft only when it differs.
func (a *App) DraftStamp(id string) (string, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return "", err
	}
	d, err := o.Draft(id)
	if err != nil {
		return "", err
	}
	ids := make([]string, len(d.Suggestions))
	for i, s := range d.Suggestions {
		ids[i] = s.ID
	}
	return d.Revision + "|" + d.Baseline.Revision + "|" + strings.Join(ids, ","), nil
}

// UpdateDraft replaces a draft's text, provided it is still at expected.
func (a *App) UpdateDraft(id, expected, text string) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	return o.Update(id, expected, text, nil)
}

// SaveDraft writes a draft back to its resource; see drafts.Service.Save.
func (a *App) SaveDraft(id, expected string) (DraftSaved, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return DraftSaved{}, err
	}
	return o.Save(id, expected)
}

// SaveDraftAs writes a draft to a new resource, such as a new file in a local
// source, which becomes the draft's origin. Nothing that exists is replaced.
// The new resource is put at the top of Documents › Recent.
func (a *App) SaveDraftAs(id, expected string, ref ResourceRef) (DraftSaved, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return DraftSaved{}, err
	}
	saved, err := o.SaveAs(id, expected, ref)
	if err == nil {
		a.recordOpened(saved.View)
	}
	return saved, err
}

// RenameDocumentFile gives a file of a local source a new name in its own
// folder; see documents.Ops.Rename. Its drafts, its entry under Documents ›
// Recent and its pin go with it.
func (a *App) RenameDocumentFile(ref ResourceRef, name string) (documents.Renamed, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return documents.Renamed{}, err
	}
	renamed, err := o.Rename(ref, name)
	if renamed.Ref == ref || renamed.Ref.Path == "" {
		return renamed, err
	}
	// Files and the workspace trees list the folder again.
	a.emitWorkspaceMountChanged("content-changed", ref.WorkspaceID, ref.SourceID)
	if rerr := recent.For(stateLayout()).Move(ref, renamed.Ref, renamed.Name); rerr != nil {
		a.emit("documents:recent", RecentChange{Error: "The file is renamed, but Recent still shows its old name: " + rerr.Error()})
	} else {
		a.emit("documents:recent", RecentChange{})
	}
	a.movePin(ref, renamed.Ref, renamed.Name)
	return renamed, err
}

// movePin points a pin of the resource from at to instead, keeping its
// place among the workspace's pins.
func (a *App) movePin(from, to ResourceRef, title string) {
	a.mu.Lock()
	index := workspaces.Index(a.settings.WorkspaceRecords, from.WorkspaceID)
	pinned := false
	if index >= 0 {
		for _, p := range a.settings.WorkspaceRecords[index].Pins {
			pinned = pinned || p.Kind == "resource" && p.Ref != nil && *p.Ref == from
		}
	}
	a.mu.Unlock()
	if !pinned {
		return
	}
	_, _ = a.changeWorkspacePins(from.WorkspaceID, func(_ WorkspaceRecord, pins []WorkspacePin) ([]WorkspacePin, error) {
		for i, p := range pins {
			if p.Kind == "resource" && p.Ref != nil && *p.Ref == from {
				moved := to
				pins[i].Ref, pins[i].Title = &moved, title
			}
		}
		return pins, nil
	})
}

// AcceptSuggestion applies a suggestion to a draft, or every suggestion of
// its proposal when all is true, provided the draft is still at expected.
func (a *App) AcceptSuggestion(id, expected, suggestion string, all bool) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	return o.Decide(id, expected, suggestion, true, all)
}

// RejectSuggestion removes a suggestion from a draft, or every suggestion of
// its proposal when all is true. The text is not changed.
func (a *App) RejectSuggestion(id, suggestion string, all bool) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	return o.Decide(id, "", suggestion, false, all)
}

// ReviseSuggestion changes the text a suggestion proposes, as the person
// edited it in the Editor before accepting or rejecting it.
func (a *App) ReviseSuggestion(id, suggestion, after string) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	return o.Revise(id, suggestion, after)
}

// CompareDraft compares a draft with its resource as it is now, for a draft
// whose save or publication was refused because the resource moved on.
func (a *App) CompareDraft(id string) (drafts.Comparison, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.Comparison{}, err
	}
	return o.Compare(id)
}

// MergeIntoDraft brings the changes made to a draft's resource since it was
// opened into the draft, when the two changed no part differently.
func (a *App) MergeIntoDraft(id, expected string) (drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	return o.Merge(id, expected)
}

// DiscardDraft deletes a draft, provided it is still at expected. The resource
// is not changed.
func (a *App) DiscardDraft(id, expected string) error {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return err
	}
	return o.Discard(id, expected)
}

// PreviewDraft renders a draft's current text for the Editor's Preview view.
func (a *App) PreviewDraft(id string) (DraftPreview, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return DraftPreview{}, err
	}
	return o.preview(id)
}

// ListDrafts lists the drafts in progress, most recent first.
func (a *App) ListDrafts() ([]drafts.View, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return nil, err
	}
	return o.List()
}
