package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/sessionhost"
)

// HistoryEntry is one prompt the person sent: as a new turn, as a steering
// message, or queued for the end of a turn.
type HistoryEntry struct {
	Time      time.Time `json:"time"`
	Kind      string    `json:"kind"` // "send", "steer" or "queue"
	Text      string    `json:"text"`
	Doc       *DocRef   `json:"doc,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	// Chat is the session the prompt was sent to. Entries written before
	// it was recorded have none and belong to no chat.
	Chat string `json:"chat,omitempty"`
}

// DocRef is the document the person picked as the context of a prompt.
type DocRef = sessionhost.DocRef

// draftDocRef gives a document open in the Editor the full path of the file
// its draft was opened from, when it was opened from a local folder; the
// chat may belong to another workspace.
func (a *App) draftDocRef(doc *DocRef) *DocRef {
	if doc == nil || doc.DraftID == "" {
		return doc
	}
	svc, err := a.draftService()
	if err != nil {
		return doc
	}
	d, err := svc.Store.Get(doc.DraftID)
	if err != nil || d.Origin.Kind != "local-folder" {
		return doc
	}
	source, err := a.resourceService().source(ResourceRef{WorkspaceID: d.Origin.WorkspaceID, SourceID: d.Origin.SourceID})
	if err != nil {
		return doc
	}
	out := *doc
	out.Path, out.Workspace = filepath.Join(source.Path, filepath.FromSlash(d.Origin.Path)), ""
	return &out
}

// draftContext is what the agent is told about a document open in the
// Editor: the document, its file when it has one, and the passage selected.
func draftContext(doc *DocRef) string {
	s := "\n\nThe person has the document \"" + doc.Title + "\" open in the Editor"
	if p := docFile(doc); p != "" {
		s += ", the file " + p
	}
	s += "; take it to be what this message is about unless they say otherwise."
	if doc.Selection != "" {
		s += "\n\nThe person has selected this passage of it, which is what the message is about:\n\n````markdown\n" + doc.Selection + "\n````"
	}
	return s
}

// draftMention is what the agent is told about a document open in the Editor
// that the person mentioned with @ in the message.
func draftMention(doc *DocRef) string {
	s := "The person mentioned the document \"" + doc.Title + "\", open in the Editor"
	if p := docFile(doc); p != "" {
		s += ", the file " + p
	}
	return s + "."
}

// docFile is a document's file as the agent reaches it: its path inside its
// workspace's folder, or its path as given when no workspace is known.
func docFile(doc *DocRef) string {
	if doc.Path == "" || doc.Workspace == "" {
		return doc.Path
	}
	return filepath.Join(doc.Workspace, filepath.FromSlash(doc.Path))
}

// withDoc is the prompt as the agent gets it: the person's words and, when a
// document is open, what the agent needs to know about it.
func withDoc(text string, doc *DocRef) string {
	if doc != nil && doc.DraftID != "" {
		return text + draftContext(doc)
	}
	if doc == nil || doc.Path == "" {
		return text
	}
	return text + "\n\nThe document is " + docFile(doc) + "."
}

// The history is a file of one JSON entry per line in the app's settings
// folder, newest last. The page reads at most historyLimit entries.
const historyLimit = 1000

func historyPath() string { return filepath.Join(configDir(), "history.jsonl") }

func (a *App) remember(ws, chat, kind, text string, doc *DocRef) {
	if doc != nil && doc.Selection != "" {
		// A selected passage is for the agent, not for the history.
		d := *doc
		d.Selection = ""
		doc = &d
	}
	b, err := json.Marshal(HistoryEntry{Time: time.Now(), Kind: kind, Text: text, Doc: doc, Workspace: ws, Chat: chat})
	if err != nil {
		return
	}
	a.hmu.Lock()
	defer a.hmu.Unlock()
	f, err := os.OpenFile(historyPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

// History returns the prompts sent so far, newest last.
func (a *App) History() ([]HistoryEntry, error) {
	a.hmu.Lock()
	defer a.hmu.Unlock()
	f, err := os.Open(historyPath())
	if errors.Is(err, os.ErrNotExist) {
		return []HistoryEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []HistoryEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for sc.Scan() {
		var e HistoryEntry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Text != "" {
			out = append(out, e)
		}
	}
	if len(out) > historyLimit {
		out = out[len(out)-historyLimit:]
	}
	return out, sc.Err()
}

// ChatHistory returns the text of the prompts sent to one chat, newest last,
// for Up and Down in its prompt box. A chat continued from another, with
// another provider or after recovery, includes the prompts of the chats it
// was continued from.
func (a *App) ChatHistory(id string) ([]string, error) {
	if id == "" {
		return []string{}, nil
	}
	lineage := map[string]bool{}
	a.cmu.Lock()
	_, records := conversationLinks()
	a.cmu.Unlock()
	for next := id; next != "" && !lineage[next] && len(lineage) < 100; {
		lineage[next] = true
		if r := records[next]; r != nil {
			next = r.ParentSessionID
		} else {
			next = ""
		}
	}
	all, err := a.History()
	out := []string{}
	for _, e := range all {
		if lineage[e.Chat] {
			out = append(out, e.Text)
		}
	}
	return out, err
}

// ClearHistory forgets every prompt.
func (a *App) ClearHistory() error {
	a.hmu.Lock()
	defer a.hmu.Unlock()
	err := os.Remove(historyPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// SaveHistory writes the prompt history as Markdown to a file the person
// chooses, and returns its path, or "" when they cancel.
func (a *App) SaveHistory() (string, error) {
	h, err := a.History()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# " + product.Name + " Prompt History\n\n")
	kinds := map[string]string{"send": "Sent", "steer": "Steering", "queue": "Queued"}
	for i := len(h) - 1; i >= 0; i-- {
		e := h[i]
		fmt.Fprintf(&b, "## %s, %s\n\n", e.Time.Format("2006-01-02 15:04"), kinds[e.Kind])
		if e.Workspace != "" {
			fmt.Fprintf(&b, "Workspace: `%s`\n\n", e.Workspace)
		}
		if e.Doc != nil {
			fmt.Fprintf(&b, "Document: %s, `%s`\n\n", e.Doc.Title, e.Doc.Path)
		}
		b.WriteString(fence(e.Text))
	}
	return a.saveText("Save the prompt history", product.ID+"-history-"+time.Now().Format("2006-01-02")+".md", b.String())
}

// SaveRun writes the run, as the page has written it up in Markdown, to a
// file the person chooses.
func (a *App) SaveRun(markdown string) (string, error) {
	return a.saveText("Save the run", product.ID+"-run-"+time.Now().Format("2006-01-02-1504")+".md", markdown)
}

func (a *App) saveText(title, name, text string) (string, error) {
	a.mu.Lock()
	dir := a.settings.Workspace
	a.mu.Unlock()
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title: title, DefaultDirectory: dir, DefaultFilename: name,
		Filters: []wruntime.FileFilter{{DisplayName: "Markdown (*.md)", Pattern: "*.md"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(text), 0o644)
}

// fence puts text in a code fence long enough that no run of backticks in
// the text closes it.
func fence(text string) string {
	n := 3
	for strings.Contains(text, strings.Repeat("`", n)) {
		n++
	}
	f := strings.Repeat("`", n)
	return f + "text\n" + strings.TrimRight(text, "\n") + "\n" + f + "\n\n"
}
