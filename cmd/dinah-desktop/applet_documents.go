package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"

	"studio/internal/applet"
	"studio/internal/drafts"
)

//go:embed applet_documents.go
var documentsAppletSource []byte

/* Suggestions to a document, for agents (docs/design/content-applets.md,
   ruling 2). An agent reads a document as its draft holds it and proposes
   new text; each changed paragraph becomes a suggestion that the person
   reviews in the document's window, edits if they like, and accepts or
   rejects, one at a time or all at once. Nothing reaches the document's
   text until the person accepts it, so proposing asks no consent: the
   person decides on each change instead. Accepting and rejecting are the
   person's, and agents have no action for them. */

const (
	documentsReadAction    = "dinah.documents/read@1"
	documentsSuggestAction = "dinah.documents/suggest@1"
)

// documentDraft opens the draft of a document in the calling workspace's
// main folder, by its path there, with the operations scoped to that
// workspace.
func (a *App) documentDraft(ctx context.Context, raw string) (documentOps, drafts.View, error) {
	clean, err := workspaceRelativePath(raw)
	if err != nil || clean == "." {
		return documentOps{}, drafts.View{}, errors.New("give the path of a document in the workspace, such as notes/plan.md")
	}
	_, record, err := a.mainFolder(ctx)
	if err != nil {
		return documentOps{}, drafts.View{}, err
	}
	o, err := a.documentOps(ctx, record.ID)
	if err != nil {
		return documentOps{}, drafts.View{}, err
	}
	v, err := o.Open(ResourceRef{WorkspaceID: record.ID, SourceID: record.PrimarySourceID, Path: toSlash(clean)})
	return o, v, err
}

// suggestionOut is a suggestion as an agent reads it.
type suggestionOut struct {
	ID     string `json:"id"`
	Before string `json:"before"`
	After  string `json:"after"`
	Author string `json:"author"`
	Reason string `json:"reason,omitempty"`
	Stale  bool   `json:"stale"`
	Edited bool   `json:"edited"`
}

func suggestionsOut(list []drafts.Suggestion) []suggestionOut {
	out := make([]suggestionOut, 0, len(list))
	for _, s := range list {
		out = append(out, suggestionOut{ID: s.ID, Before: s.Before, After: s.After, Author: s.Author, Reason: s.Reason, Stale: s.Stale, Edited: s.Edited})
	}
	return out
}

func (a *App) documentsAppletBundle() (applet.Bundle, error) {
	path := `{"type":"string","minLength":1,"maxLength":1024}`
	suggestion := `{"type":"object","additionalProperties":false,"required":["id","before","after","author","stale","edited"],"properties":{"id":{"type":"string"},"before":{"type":"string"},"after":{"type":"string"},"author":{"type":"string"},"reason":{"type":"string"},"stale":{"type":"boolean"},"edited":{"type":"boolean"}}}`
	manifest := applet.Manifest{ID: "dinah.documents", Version: "1.0.0", APIVersion: 1, Title: "Documents",
		Description: "A document's text as its draft holds it, and suggestions to it that the person reviews and accepts or rejects in the document's window.",
		Actions: []applet.Action{
			{Ref: documentsReadAction, Title: "Read a Document", Replay: "pure", Capabilities: []string{"workspace.read"},
				Description:  "Read a document in the calling workspace's main folder as its draft holds it, with the suggestions not yet accepted or rejected. The text is what Suggest Changes takes as base.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path"],"properties":{"path":` + path + `}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","text","suggestions"],"properties":{"path":{"type":"string"},"text":{"type":"string"},"suggestions":{"type":"array","items":` + suggestion + `}}}`)},
			// Proposing changes nothing until the person accepts it, so it
			// declares only reading.
			{Ref: documentsSuggestAction, Title: "Suggest Changes", Replay: "once", Capabilities: []string{"workspace.read"},
				Description: "Propose changes to a document without making them: give the text you read (base, from Read a Document) and the text you propose, and each changed paragraph becomes a suggestion the person reviews, edits if they like, and accepts or rejects in the document's window, one at a time or all at once. The document's text is not changed, and the suggestions are placed in it as it is now even if the person has typed since you read it. Give a reason the person will read beside the suggestions.",
				InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","base","text"],"properties":{"path":` + path +
					`,"base":{"type":"string","maxLength":4194304},"text":{"type":"string","maxLength":4194304},"reason":{"type":"string","maxLength":4096},"sources":{"type":"array","maxItems":50,"items":{"type":"string","maxLength":1024}}}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["added","suggestions"],"properties":{"added":{"type":"integer"},"suggestions":{"type":"array","items":` + suggestion + `}}}`)},
		}}
	handlers := map[string]applet.Handler{
		documentsReadAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			_, v, err := a.documentDraft(ctx, in.Path)
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]any{"path": v.Origin.Path, "text": v.Text, "suggestions": suggestionsOut(v.Suggestions)})
		},
		documentsSuggestAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				Path    string   `json:"path"`
				Base    string   `json:"base"`
				Text    string   `json:"text"`
				Reason  string   `json:"reason"`
				Sources []string `json:"sources"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			o, v, err := a.documentDraft(ctx, in.Path)
			if err != nil {
				return applet.Result{}, err
			}
			_, added, err := o.Suggest(v.ID, in.Base, in.Text, drafts.Proposal{Author: a.suggestionAuthor(ctx), Reason: in.Reason, Sources: in.Sources})
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]any{"added": len(added), "suggestions": suggestionsOut(added)})
		},
	}
	return applet.Bundle{Manifest: manifest, Handlers: handlers, Sources: map[string][]byte{"applet_documents.go": documentsAppletSource}}, nil
}

// suggestionAuthor is who the person is told made a suggestion: the calling
// chat as they see it, or an agent.
func (a *App) suggestionAuthor(ctx context.Context) string {
	if v, ok := applet.InvocationFromContext(ctx); ok && v.OwnerKind == "chat" {
		if s := a.session(v.OwnerID); s != nil {
			return a.chatTitle(s)
		}
	}
	return "An agent"
}
