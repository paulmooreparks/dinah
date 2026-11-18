package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	pathpkg "path"
	"strings"

	"studio/internal/applet"
	"studio/internal/drafts"
	"studio/internal/layout"
	"studio/internal/resource"
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
   person's, and agents have no action for them.

   A suggestion may refer to a file the document does not have yet, such as
   a diagram. Attach a File keeps a file of the workspace with the
   document's draft, as Attach File in the document's window does. The file
   is written with the document only when the document's text refers to it
   and the person saves or publishes (drafts.Service.Save), so attaching
   changes nothing a reader sees either, and asks no consent. An agent
   proposed fifteen diagram references to a page whose draft had none of
   the diagram files, and the person saw fifteen missing diagrams (Paul, 12
   November 2026). */

/* An agent corrects its own suggestions with Revise and Withdraw (Paul, 18
   November 2026). Only the chat that made a suggestion may change it, so
   Suggest Changes records the chat with each suggestion (suggestionCaller),
   and a suggestion the person has edited is theirs and is refused. Both
   change suggestions and not the document's text, so they are offered where
   a product's agents propose, as Suggest Changes is. */

const (
	documentsReadAction     = "dinah.documents/read@1"
	documentsSuggestAction  = "dinah.documents/suggest@1"
	documentsAttachAction   = "dinah.documents/attach@1"
	documentsReviseAction   = "dinah.documents/revise@1"
	documentsWithdrawAction = "dinah.documents/withdraw@1"
)

// documentDraft opens the draft of a document of the calling workspace,
// with the operations scoped to that workspace. With no source it is a
// document in the workspace's main folder, by its path there. With a source
// it is a document of that attached source, by the path the source gives
// it (List Source Resources), so a page of a wiki is proposed to as a file
// is.
func (a *App) documentDraft(ctx context.Context, sourceID, raw string) (documentOps, drafts.View, error) {
	clean, err := workspaceRelativePath(raw)
	if err != nil || clean == "." {
		return documentOps{}, drafts.View{}, errors.New("give the path of a document in the workspace, such as notes/plan.md")
	}
	var record WorkspaceRecord
	if sourceID == "" {
		if _, record, err = a.mainFolder(ctx); err != nil {
			return documentOps{}, drafts.View{}, err
		}
		sourceID = record.PrimarySourceID
	} else {
		if record, err = a.callingWorkspaceRecord(ctx); err != nil {
			return documentOps{}, drafts.View{}, err
		}
		attached := false
		for _, s := range record.Sources {
			attached = attached || s.ID == sourceID
		}
		if !attached {
			return documentOps{}, drafts.View{}, errors.New("the workspace has no source with that ID; List Workspace Sources gives them")
		}
	}
	o, err := a.documentOps(ctx, record.ID)
	if err != nil {
		return documentOps{}, drafts.View{}, err
	}
	v, err := o.Open(ResourceRef{WorkspaceID: record.ID, SourceID: sourceID, Path: toSlash(clean)})
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
	source := `{"type":"string","maxLength":128}`
	suggestion := `{"type":"object","additionalProperties":false,"required":["id","before","after","author","stale","edited"],"properties":{"id":{"type":"string"},"before":{"type":"string"},"after":{"type":"string"},"author":{"type":"string"},"reason":{"type":"string"},"stale":{"type":"boolean"},"edited":{"type":"boolean"}}}`
	target := `"path":` + path + `,"sourceId":` + source
	ids := `{"type":"array","minItems":1,"maxItems":500,"items":{"type":"string","minLength":1,"maxLength":64}}`
	manifest := applet.Manifest{ID: "dinah.documents", Version: "1.3.0", APIVersion: 1, Title: "Documents",
		Description: "A document's text as its draft holds it, and suggestions to it that the person reviews and accepts or rejects in the document's window.",
		Actions: []applet.Action{
			{Ref: documentsAttachAction, Title: "Attach a File to a Document", Replay: "once", Capabilities: []string{"workspace.read"},
				Description:  "Keep a file of the calling workspace's main folder with a document's draft, so that a change you suggest can refer to it: a diagram file, a picture, a file to download. Do this before Suggest Changes for every file your proposed text refers to that the document does not have yet. The answer gives the name the file is kept under, which differs from the file's own name when the document already has a file of that name; refer to the file by the name in the answer. The document shows the file at once, and the file is saved or published with the document only if the document's text refers to it when the person saves or publishes. Nothing is published by this action. Give the sourceId you read the document with, for a document of an attached source.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","file"],"properties":{"path":` + path + `,"sourceId":` + source + `,"file":` + path + `}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["name","bytes"],"properties":{"name":{"type":"string"},"bytes":{"type":"integer"}}}`)},
			{Ref: documentsReadAction, Title: "Read a Document", Replay: "pure", Capabilities: []string{"workspace.read"},
				Description:  "Read a document of the calling workspace as its draft holds it, with the suggestions not yet accepted or rejected. The text is what Suggest Changes takes as base. Without sourceId the path is in the workspace's main folder; with one it is a document of that attached source, such as a page of a wiki, by the path List Source Resources gives.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path"],"properties":{"path":` + path + `,"sourceId":` + source + `}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","text","suggestions"],"properties":{"path":{"type":"string"},"text":{"type":"string"},"suggestions":{"type":"array","items":` + suggestion + `}}}`)},
			// Proposing changes nothing until the person accepts it, so it
			// declares only reading.
			{Ref: documentsSuggestAction, Title: "Suggest Changes", Replay: "once", Capabilities: []string{"workspace.read"},
				Description: "Propose changes to a document without making them: give the text you read (base, from Read a Document) and the text you propose, and each changed paragraph becomes a suggestion the person reviews, edits if they like, and accepts or rejects in the document's window, one at a time or all at once. The document's text is not changed, and the suggestions are placed in it as it is now even if the person has typed since you read it. Give a reason the person will read beside the suggestions. Give the sourceId you read with, for a document of an attached source. To correct a few of your suggestions later, change their text with Revise Suggestions or remove them with Withdraw Suggestions; do not propose the whole batch again, and do not ask the person to reject it.",
				InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","base","text"],"properties":{"path":` + path + `,"sourceId":` + source +
					`,"base":{"type":"string","maxLength":4194304},"text":{"type":"string","maxLength":4194304},"reason":{"type":"string","maxLength":4096},"sources":{"type":"array","maxItems":50,"items":{"type":"string","maxLength":1024}}}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["added","suggestions"],"properties":{"added":{"type":"integer"},"suggestions":{"type":"array","items":` + suggestion + `}}}`)},
			// Revising and withdrawing change suggestions only, which change
			// nothing until the person accepts one, so they declare only
			// reading, as Suggest Changes does.
			{Ref: documentsReviseAction, Title: "Revise Suggestions", Replay: "once", Capabilities: []string{"workspace.read"},
				Description: "Replace the proposed text of some of your own suggestions to a document, and their reason if you give one, leaving every other suggestion as it is. Give each suggestion's ID, from Read a Document or Suggest Changes, with its whole new text, as after shows it; a line break the old text ended with is kept if you leave it out. The text each suggestion would replace stays the same: to change which paragraphs a suggestion covers, withdraw it and suggest again. Only the chat that made a suggestion may revise it; ask that chat to revise another's. A suggestion the person has edited is theirs, so ask the person. A stale suggestion, whose text has changed under it, cannot be revised; withdraw it and suggest again. If any suggestion is refused, none is changed, and the error says which and why. The document's text is not changed, and the window shows the new text in place of the old with no mark, so say in the reason what changed. Give the sourceId you read with, for a document of an attached source.",
				InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","suggestions"],"properties":{` + target +
					`,"suggestions":{"type":"array","minItems":1,"maxItems":500,"items":{"type":"object","additionalProperties":false,"required":["id","after"],"properties":{"id":{"type":"string","minLength":1,"maxLength":64},"after":{"type":"string","maxLength":4194304}}}},"reason":{"type":"string","maxLength":4096}}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["suggestions"],"properties":{"suggestions":{"type":"array","items":` + suggestion + `}}}`)},
			{Ref: documentsWithdrawAction, Title: "Withdraw Suggestions", Replay: "once", Capabilities: []string{"workspace.read"},
				Description: "Remove some of your own suggestions to a document, by their IDs from Read a Document or Suggest Changes, leaving every other suggestion as it is. Only the chat that made a suggestion may withdraw it; ask that chat to withdraw another's. A suggestion the person has edited is theirs, so ask the person. If any suggestion is refused, none is removed, and the error says which and why. The document's text is not changed. The answer gives how many were withdrawn and the suggestions that remain. Give the sourceId you read with, for a document of an attached source.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","ids"],"properties":{` + target + `,"ids":` + ids + `}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["withdrawn","suggestions"],"properties":{"withdrawn":{"type":"integer"},"suggestions":{"type":"array","items":` + suggestion + `}}}`)},
		}}
	handlers := map[string]applet.Handler{
		documentsAttachAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				Path     string `json:"path"`
				SourceID string `json:"sourceId"`
				File     string `json:"file"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			clean, err := workspaceRelativePath(in.File)
			if err != nil || clean == "." {
				return applet.Result{}, errors.New("give the path of a file in the workspace's main folder, such as diagrams/plan.canvas")
			}
			folder, _, err := a.mainFolder(ctx)
			if err != nil {
				return applet.Result{}, err
			}
			data, err := folder.ReadBlob(ctx, toSlash(clean))
			if errors.Is(err, resource.ErrNotFound) {
				return applet.Result{}, errors.New(toSlash(clean) + " was not found in the workspace's main folder")
			}
			if err != nil {
				return applet.Result{}, err
			}
			o, v, err := a.documentDraft(ctx, in.SourceID, in.Path)
			if err != nil {
				return applet.Result{}, err
			}
			name, err := o.AddFile(v.ID, pathpkg.Base(toSlash(clean)), data)
			if err != nil {
				return applet.Result{}, err
			}
			// A window that shows the document reads its files again.
			a.emitWorkspaceMountChanged("content-changed", v.Origin.WorkspaceID, v.Origin.SourceID)
			return builtinJSON(map[string]any{"name": name, "bytes": len(data)})
		},
		documentsReadAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				Path     string `json:"path"`
				SourceID string `json:"sourceId"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			_, v, err := a.documentDraft(ctx, in.SourceID, in.Path)
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]any{"path": v.Origin.Path, "text": v.Text, "suggestions": suggestionsOut(v.Suggestions)})
		},
		documentsSuggestAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				Path     string   `json:"path"`
				SourceID string   `json:"sourceId"`
				Base     string   `json:"base"`
				Text     string   `json:"text"`
				Reason   string   `json:"reason"`
				Sources  []string `json:"sources"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			o, v, err := a.documentDraft(ctx, in.SourceID, in.Path)
			if err != nil {
				return applet.Result{}, err
			}
			caller, _ := a.suggestionCaller(ctx)
			_, added, err := o.Suggest(v.ID, in.Base, in.Text, drafts.Proposal{Author: a.suggestionAuthor(ctx), Owner: caller.owner, Reason: in.Reason, Sources: in.Sources})
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]any{"added": len(added), "suggestions": suggestionsOut(added)})
		},
		documentsReviseAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				Path        string  `json:"path"`
				SourceID    string  `json:"sourceId"`
				Reason      *string `json:"reason"`
				Suggestions []struct {
					ID    string `json:"id"`
					After string `json:"after"`
				} `json:"suggestions"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			caller, err := a.ownSuggestionsCaller(ctx)
			if err != nil {
				return applet.Result{}, err
			}
			o, v, err := a.documentDraft(ctx, in.SourceID, in.Path)
			if err != nil {
				return applet.Result{}, err
			}
			changes := make([]drafts.AgentRevision, len(in.Suggestions))
			for i, s := range in.Suggestions {
				changes[i] = drafts.AgentRevision{ID: s.ID, After: s.After}
			}
			_, revised, err := o.ReviseOwn(v.ID, caller.owns, changes, in.Reason)
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]any{"suggestions": suggestionsOut(revised)})
		},
		documentsWithdrawAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				Path     string   `json:"path"`
				SourceID string   `json:"sourceId"`
				IDs      []string `json:"ids"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			caller, err := a.ownSuggestionsCaller(ctx)
			if err != nil {
				return applet.Result{}, err
			}
			o, v, err := a.documentDraft(ctx, in.SourceID, in.Path)
			if err != nil {
				return applet.Result{}, err
			}
			left, removed, err := o.WithdrawOwn(v.ID, caller.owns, in.IDs)
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]any{"withdrawn": removed, "suggestions": suggestionsOut(left.Suggestions)})
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
	return unknownSuggestionAuthor
}

// unknownSuggestionAuthor is the author of a suggestion that no chat made.
const unknownSuggestionAuthor = "An agent"

// suggestionCaller is the chat that calls a Documents action, as its
// suggestions record it and as Revise and Withdraw check it.
type suggestionCaller struct {
	// owner is the chat's identity on the bus when it has one, which stays
	// with the chat when it continues in another session (moveBusSession),
	// and otherwise its session ID.
	owner string
	// session is the chat's session ID now, which a suggestion records as
	// its owner when the chat had no identity on the bus yet.
	session string
	// title is the chat's title as the person sees it now, for the
	// suggestions made before owners were recorded; it is empty when the
	// chat has no title of its own or is not of the calling workspace.
	title string
}

// suggestionCaller is the calling chat, or false when no chat calls.
func (a *App) suggestionCaller(ctx context.Context) (suggestionCaller, bool) {
	v, ok := applet.InvocationFromContext(ctx)
	if !ok || v.OwnerKind != "chat" || v.OwnerID == "" {
		return suggestionCaller{}, false
	}
	c := suggestionCaller{owner: v.OwnerID, session: v.OwnerID}
	if s := a.session(v.OwnerID); s != nil {
		if id := a.busIdentityOf(s); id != "" {
			c.owner = id
		}
		if strings.TrimSpace(a.shownTitle(s)) != "" && v.Workspace != "" && layout.SamePath(s.Cwd(), v.Workspace) {
			c.title = a.chatTitle(s)
		}
	} else if id := a.knownBusID(v.OwnerID); id != "" {
		c.owner = id
	}
	return c, true
}

// ownSuggestionsCaller is the calling chat, for an action that changes only
// the caller's own suggestions.
func (a *App) ownSuggestionsCaller(ctx context.Context) (suggestionCaller, error) {
	c, ok := a.suggestionCaller(ctx)
	if !ok {
		return suggestionCaller{}, errors.New("only the chat that made a suggestion may change it, and this call comes from no chat")
	}
	return c, nil
}

// owns reports whether the calling chat made a suggestion.
func (c suggestionCaller) owns(s drafts.Suggestion) bool {
	if s.Owner != "" {
		return s.Owner == c.owner || s.Owner == c.session
	}
	// A suggestion made before owners were recorded (18 November 2026) has
	// only the name the person was shown. It is the caller's when that name
	// is the calling chat's title as it is now. The chat must have a title
	// of its own, since an untitled chat is shown by its workspace's name,
	// which every untitled chat there shares, and the name given to a
	// suggestion of a caller that was no chat matches no chat. The chat must
	// be of the calling workspace (suggestionCaller), and the suggestion is
	// in a draft of that workspace's documents, since documentDraft reaches
	// no other. A chat renamed since then has to ask the person, as for a
	// suggestion of another chat.
	return c.title != "" && s.Author != unknownSuggestionAuthor && s.Author == c.title
}
