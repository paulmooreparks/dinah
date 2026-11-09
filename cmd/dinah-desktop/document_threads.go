package main

import (
	"strings"

	"studio/internal/documents"
	"studio/internal/threads"
)

// The comment threads a document's window draws in its margin, kept beside
// the document and never in its text (docs/design/document-comments.md).
// The person writes here; agents write through the Documents applet.

// DraftThreads returns the threads of a draft's document, placed in its text.
func (a *App) DraftThreads(id string) ([]documents.PlacedThread, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return nil, err
	}
	return o.Threads(id)
}

// OpenDraftThread opens a thread about the span [from, to) of a draft's text,
// in UTF-16 code units, as the person's comment.
func (a *App) OpenDraftThread(id string, from, to int, text string) (documents.PlacedThread, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return documents.PlacedThread{}, err
	}
	return o.OpenThread(id, documents.ThreadOpen{From: &from, To: &to, Kind: threads.KindComment, Message: a.personMessage(text)})
}

// ReplyDraftThread adds the person's reply to a thread; a reply to a
// resolved thread opens it again.
func (a *App) ReplyDraftThread(id, thread, text string) (documents.PlacedThread, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return documents.PlacedThread{}, err
	}
	return o.ReplyThread(id, thread, a.personMessage(text))
}

// ResolveDraftThread closes a thread, with the person's optional last word.
func (a *App) ResolveDraftThread(id, thread, text string) (documents.PlacedThread, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return documents.PlacedThread{}, err
	}
	return o.ResolveThread(id, thread, a.personMessage(text))
}

// BringInDocumentComments brings the comments on a draft's document, from
// its source, into its threads, returning how many were new. A document
// whose source keeps no comments has none.
func (a *App) BringInDocumentComments(id string) (int, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return 0, err
	}
	return o.BringInComments(id)
}

// personMessage is a message the person writes, under their name as a
// heading shows it (person_name.go).
func (a *App) personMessage(text string) threads.Message {
	return threads.Message{Author: a.personName(), By: threads.ByPerson, Text: strings.TrimSpace(text)}
}
