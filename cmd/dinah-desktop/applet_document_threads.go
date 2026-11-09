package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"strings"

	"studio/internal/applet"
	"studio/internal/documents"
	"studio/internal/threads"
)

//go:embed applet_document_threads.go
var documentThreadsAppletSource []byte

/* Comment threads for agents (docs/design/document-comments.md). An agent
   lists the drafts in progress, reads a draft's threads placed in its text,
   opens a thread on words of the document, replies to one and resolves one,
   and brings in the comments from the document's site. A thread is kept
   beside the document and reaches its site with the next save or publish,
   which only the person makes, so these actions declare reading alone. */

const (
	documentsDraftsAction  = "dinah.document-comments/drafts@1"
	documentsThreadsAction = "dinah.document-comments/threads@1"
	documentsCommentAction = "dinah.document-comments/comment@1"
	documentsCollectAction = "dinah.document-comments/collect@1"
)

// threadOut is a thread as an agent reads it.
type threadOut struct {
	ID       string          `json:"id"`
	Key      string          `json:"key"`
	Kind     string          `json:"kind"`
	Severity string          `json:"severity"`
	Quote    string          `json:"quote"`
	Resolved bool            `json:"resolved"`
	Due      bool            `json:"due"`
	Waiting  bool            `json:"waiting"`
	Lost     bool            `json:"lost"`
	Unsent   bool            `json:"unsent"`
	Messages []threadMessage `json:"messages"`
	Placed   [2]int          `json:"placed"`
}

type threadMessage struct {
	Author string `json:"author"`
	By     string `json:"by"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	At     string `json:"at"`
}

func threadsOut(list []documents.PlacedThread) []threadOut {
	out := make([]threadOut, 0, len(list))
	for _, t := range list {
		ms := make([]threadMessage, 0, len(t.Messages))
		for _, m := range t.Messages {
			ms = append(ms, threadMessage{Author: m.Author, By: m.By, Kind: m.Kind, Text: m.Text, At: m.At.UTC().Format("2006-01-02T15:04:05Z07:00")})
		}
		out = append(out, threadOut{ID: t.ID, Key: t.Key, Kind: t.Kind, Severity: t.Severity, Quote: t.Quote, Resolved: t.Resolved, Due: t.Due, Waiting: t.Waiting, Lost: t.Lost, Unsent: t.Unsent, Messages: ms, Placed: [2]int{t.From, t.To}})
	}
	return out
}

// threadOps is the document operations for a draft an agent names, scoped
// to the calling workspace.
func (a *App) threadOps(ctx context.Context, id string) (documentOps, error) {
	if strings.TrimSpace(id) == "" {
		return documentOps{}, errors.New("give the id of a draft, from List Drafts")
	}
	_, record, err := a.mainFolder(ctx)
	if err != nil {
		return documentOps{}, err
	}
	o, err := a.documentOps(ctx, record.ID)
	if err != nil {
		return documentOps{}, err
	}
	if _, err := o.Draft(id); err != nil {
		return documentOps{}, err
	}
	return o, nil
}

func (a *App) documentThreadsAppletBundle() (applet.Bundle, error) {
	str := `{"type":"string"}`
	message := `{"type":"object","additionalProperties":false,"required":["author","by","kind","text","at"],"properties":{"author":` + str + `,"by":` + str + `,"kind":` + str + `,"text":` + str + `,"at":` + str + `}}`
	thread := `{"type":"object","additionalProperties":false,"required":["id","key","kind","severity","quote","resolved","due","waiting","lost","unsent","messages","placed"],"properties":{"id":` + str + `,"key":` + str + `,"kind":` + str + `,"severity":` + str + `,"quote":` + str + `,"resolved":{"type":"boolean"},"due":{"type":"boolean"},"waiting":{"type":"boolean"},"lost":{"type":"boolean"},"unsent":{"type":"boolean"},"messages":{"type":"array","items":` + message + `},"placed":{"type":"array","items":{"type":"integer"},"minItems":2,"maxItems":2}}}`
	draft := `{"type":"object","additionalProperties":false,"required":["id","title","kind","sourceId","path","changed"],"properties":{"id":` + str + `,"title":` + str + `,"kind":` + str + `,"sourceId":` + str + `,"path":` + str + `,"changed":{"type":"boolean"}}}`
	id := `{"type":"string","minLength":1,"maxLength":128}`
	manifest := applet.Manifest{ID: "dinah.document-comments", Version: "1.0.0", APIVersion: 1, Title: "Document Comments",
		Description: "The comment threads on the documents open in the calling workspace: read them, open one on words of a document, reply and resolve, and bring in the comments from a document's site. Threads are kept beside the document and reach its site when the person saves or publishes.",
		Actions: []applet.Action{
			{Ref: documentsDraftsAction, Title: "List Drafts", Replay: "pure", Capabilities: []string{"workspace.read"},
				Description:  "List the documents open as drafts in the calling workspace, most recent first, with the id the other actions take, the title, the kind of source and the path within it.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["drafts"],"properties":{"drafts":{"type":"array","items":` + draft + `}}}`)},
			{Ref: documentsThreadsAction, Title: "List Comment Threads", Replay: "pure", Capabilities: []string{"workspace.read"},
				Description:  "Read a draft's comment threads, each with its words (quote), where they are in the text (placed, as UTF-16 offsets), its messages, whether it is resolved, due (its last word is a person's, so it waits on an agent) or waiting (on the person), lost (its words are gone from the text) and unsent (its site has not seen it yet).",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":` + id + `}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["threads","open","due"],"properties":{"threads":{"type":"array","items":` + thread + `},"open":{"type":"integer"},"due":{"type":"integer"}}}`)},
			{Ref: documentsCommentAction, Title: "Comment on a Draft", Replay: "once", Capabilities: []string{"workspace.read"},
				Description:  "Open a comment thread on words of a draft (op open: quote the words exactly as the text has them, and nth when they occur more than once, 1 being the first), reply to a thread (op reply, with its thread id), or resolve one (op resolve, with an optional last word). Give the text the person reads. A thread opened with a key is not opened twice. Kind is comment, question (you ask the person) or finding (a review's), with a severity of MUST, SHOULD or CONSIDER for a finding. The thread is kept beside the document and reaches its site when the person saves or publishes.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["id","op","text"],"properties":{"id":` + id + `,"op":{"type":"string","enum":["open","reply","resolve"]},"thread":{"type":"string","maxLength":64},"quote":{"type":"string","maxLength":4000},"nth":{"type":"integer","minimum":1},"key":{"type":"string","maxLength":256},"kind":{"type":"string","enum":["comment","question","finding"]},"severity":{"type":"string","enum":["MUST","SHOULD","CONSIDER"]},"text":{"type":"string","maxLength":20000},"author":{"type":"string","maxLength":200}}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["thread"],"properties":{"thread":` + thread + `}}`)},
			{Ref: documentsCollectAction, Title: "Bring In the Site's Comments", Replay: "once", Capabilities: []string{"workspace.read"},
				Description:  "Bring the comments on a draft's document from its site into its threads, so they can be read and answered here; a comment already brought in is not brought in twice. A document whose source keeps no comments brings in none.",
				InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":` + id + `}}`),
				OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["added"],"properties":{"added":{"type":"integer"}}}`)},
		}}
	handlers := map[string]applet.Handler{
		documentsDraftsAction: func(ctx context.Context, input string) (applet.Result, error) {
			_, record, err := a.mainFolder(ctx)
			if err != nil {
				return applet.Result{}, err
			}
			o, err := a.documentOps(ctx, record.ID)
			if err != nil {
				return applet.Result{}, err
			}
			list, err := o.List()
			if err != nil {
				return applet.Result{}, err
			}
			out := []map[string]any{}
			for _, v := range list {
				out = append(out, map[string]any{"id": v.ID, "title": v.Title, "kind": v.Origin.Kind, "sourceId": v.Origin.SourceID, "path": v.Origin.Path, "changed": v.Changed})
			}
			return builtinJSON(map[string]any{"drafts": out})
		},
		documentsThreadsAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			o, err := a.threadOps(ctx, in.ID)
			if err != nil {
				return applet.Result{}, err
			}
			list, err := o.Threads(in.ID)
			if err != nil {
				return applet.Result{}, err
			}
			open, due := 0, 0
			for _, t := range list {
				if !t.Resolved {
					open++
				}
				if t.Due {
					due++
				}
			}
			return builtinJSON(map[string]any{"threads": threadsOut(list), "open": open, "due": due})
		},
		documentsCommentAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				ID       string `json:"id"`
				Op       string `json:"op"`
				Thread   string `json:"thread"`
				Quote    string `json:"quote"`
				Nth      int    `json:"nth"`
				Key      string `json:"key"`
				Kind     string `json:"kind"`
				Severity string `json:"severity"`
				Text     string `json:"text"`
				Author   string `json:"author"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			o, err := a.threadOps(ctx, in.ID)
			if err != nil {
				return applet.Result{}, err
			}
			author := strings.TrimSpace(in.Author)
			if author == "" {
				author = a.suggestionAuthor(ctx)
			}
			m := threads.Message{Author: author, By: threads.ByAgent, Text: strings.TrimSpace(in.Text)}
			var t documents.PlacedThread
			switch in.Op {
			case "open":
				kind := in.Kind
				if kind == "" {
					kind = threads.KindComment
				}
				t, err = o.OpenThread(in.ID, documents.ThreadOpen{Quote: in.Quote, Nth: in.Nth, Key: in.Key, Kind: kind, Severity: in.Severity, Message: m})
			case "reply":
				t, err = o.ReplyThread(in.ID, in.Thread, m)
			case "resolve":
				t, err = o.ResolveThread(in.ID, in.Thread, m)
			default:
				err = errors.New("op is open, reply or resolve")
			}
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]any{"thread": threadsOut([]documents.PlacedThread{t})[0]})
		},
		documentsCollectAction: func(ctx context.Context, input string) (applet.Result, error) {
			var in struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return applet.Result{}, err
			}
			o, err := a.threadOps(ctx, in.ID)
			if err != nil {
				return applet.Result{}, err
			}
			n, err := o.BringInComments(in.ID)
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]any{"added": n})
		},
	}
	return applet.Bundle{Manifest: manifest, Handlers: handlers, Sources: map[string][]byte{"applet_document_threads.go": documentThreadsAppletSource}}, nil
}
