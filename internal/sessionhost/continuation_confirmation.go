package sessionhost

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"studio/internal/acp"
	"studio/internal/layout"
)

// warningEntry retains only limitations after a transcript is validated.
type warningEntry struct{ Text string }

// confirmationTranscript retains metadata and warnings after schema validation.
type confirmationTranscript struct {
	SessionID, Provider, Workspace, ParentSessionID, ParentWorkspace string
	Complete, Deleted                                                bool
	CaptureError, Summary                                            string
	ParentHistory                                                    []warningEntry
	Messages                                                         []warningEntry
}

type confirmationFile struct {
	digest [32]byte
	meta   confirmationTranscript
}

// The digest is verified from file contents on every lookup. This cache saves
// JSON traversal when a conversation has not changed, including files written
// by this process; neither timestamps nor file size authorize a cache hit.
var confirmationFiles = struct {
	sync.Mutex
	byPath map[string]confirmationFile
}{byPath: map[string]confirmationFile{}}

func cacheConfirmation(path string, digest [32]byte, meta confirmationTranscript) {
	confirmationFiles.Lock()
	defer confirmationFiles.Unlock()
	if len(confirmationFiles.byPath) >= 1024 {
		clear(confirmationFiles.byPath)
	}
	confirmationFiles.byPath[path] = confirmationFile{digest, meta}
}

func confirmationMetadata(tr *Transcript) confirmationTranscript {
	m := confirmationTranscript{
		SessionID: tr.SessionID, Provider: tr.Provider, Workspace: tr.Workspace,
		ParentSessionID: tr.ParentSessionID, ParentWorkspace: tr.ParentWorkspace,
		Complete: tr.Complete, Deleted: tr.Deleted, CaptureError: tr.CaptureError,
		Summary: tr.Summary,
	}
	if tr.ParentHistory != nil {
		m.ParentHistory = []warningEntry{}
	}
	for _, entry := range tr.ParentHistory {
		if entry.Role == "limitation" {
			m.ParentHistory = append(m.ParentHistory, warningEntry{Text: entry.Text})
		}
	}
	for _, entry := range tr.Messages {
		if entry.Role == "limitation" {
			m.Messages = append(m.Messages, warningEntry{Text: entry.Text})
		}
	}
	return m
}

func readConfirmation(path, id string) (confirmationTranscript, [32]byte, error) {
	data, err := os.ReadFile(path)
	digest := sha256.Sum256(data)
	if err != nil {
		return confirmationTranscript{}, digest, errors.New("the complete chat history is unavailable; reopen the chat to load its history before changing provider")
	}
	confirmationFiles.Lock()
	f, hit := confirmationFiles.byPath[path]
	confirmationFiles.Unlock()
	if hit && f.digest == digest && f.meta.SessionID == id {
		return f.meta, digest, nil
	}
	var saved Transcript
	if json.Unmarshal(data, &saved) != nil || saved.SessionID != id {
		return confirmationTranscript{}, digest, errors.New("the saved chat history is invalid")
	}
	tr := confirmationMetadata(&saved)
	cacheConfirmation(path, digest, tr)
	return tr, digest, nil
}

func (c *Continuations) confirmation(s *Conversation, provider, cwd string) (SessionContinuationPreview, error) {
	c.mu.Lock()
	p, err := c.confirmationLocked(s, provider, cwd)
	c.mu.Unlock()
	if err == nil {
		p.Facts = c.facts(s, provider, cwd, p, compactResult{}, 0)
		p.Facts.Handoff = nil
	}
	return p, err
}

// confirmationLocked hashes the actual saved content, rather than file size or
// modification time, so same-size edits cannot evade final validation. It never
// formats messages or asks a model. The caller holds the transcript mutex.
func (c *Continuations) confirmationLocked(s *Conversation, provider, cwd string) (SessionContinuationPreview, error) {
	p := SessionContinuationPreview{SourceSessionID: s.ID(), Provider: provider, History: []ConversationMessage{}, Warnings: []string{}}
	if err := s.HistoryErrorLocked(); err != nil {
		return p, fmt.Errorf("the chat history could not be saved: %w", err)
	}
	if c.child(s.ID()) != "" {
		return p, errors.New("this chat already continues in another session")
	}
	hash := sha256.New()
	p.options = s.Options()
	if p.options == nil {
		p.options = []acp.ConfigOption{}
	}
	identity, _ := json.Marshal([]any{s.ID(), provider, s.Cwd(), cwd, p.options, c.host.Budget(provider, nil)})
	hash.Write(identity)
	seen, warnings := map[string]bool{}, map[string]bool{}
	addWarning := func(text string) {
		if text != "" && !warnings[text] {
			p.Warnings = append(p.Warnings, text)
			warnings[text] = true
		}
	}
	id, workspace := s.ID(), s.Cwd()
	var fallback string
	var unavailable error
	var source confirmationTranscript
	missingSnapshot := false
	for id != "" {
		if seen[id] || len(seen) >= 100 {
			if unavailable == nil {
				unavailable = errors.New("the chat continuation links contain a cycle or exceed 100 segments")
			}
			break
		}
		seen[id] = true
		tr, digest, err := readConfirmation(TranscriptPath(c.host.StateDir(), id), id)
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		part, _ := json.Marshal([]any{id, digest, failure})
		hash.Write(part)
		if err != nil {
			if id == s.ID() {
				return p, err
			}
			if unavailable == nil {
				unavailable = err
			}
			break
		}
		if id == s.ID() {
			source = tr
			if tr.Deleted {
				return p, errors.New("this conversation was deleted")
			}
			if tr.CaptureError != "" && tr.CaptureError != InterruptedTurn {
				return p, errors.New(tr.CaptureError)
			}
		}
		if fallback == "" {
			fallback = tr.Summary
		}
		// Match TranscriptHistory's validation order. SavedSummary can still
		// find a fallback beyond an incomplete copy or workspace mismatch, so
		// continue reading metadata after remembering the first history fault.
		if unavailable == nil {
			switch {
			case !tr.Complete && id != s.ID():
				unavailable = errors.New("the complete chat history has not finished loading")
			case !layout.SamePath(tr.Workspace, workspace):
				unavailable = errors.New("chat continuation cannot cross workspaces")
			}
		}
		missingSnapshot = missingSnapshot || tr.ParentSessionID != "" && tr.ParentHistory == nil
		id = tr.ParentSessionID
		if tr.ParentWorkspace != "" {
			workspace = tr.ParentWorkspace
		}
	}
	// Parent snapshots are checked as recursive history reads return, after
	// ancestor read/validation errors have taken precedence.
	if unavailable == nil && missingSnapshot {
		unavailable = errors.New("the saved continuation history is incomplete")
	}
	for _, m := range source.Messages {
		addWarning(m.Text)
	}
	if unavailable != nil {
		if fallback == "" {
			return p, unavailable
		}
		addWarning("The earlier chats could not be read in full, so the summary saved beside this chat is handed over instead. " + unavailable.Error())
	} else if source.ParentSessionID != "" {
		// The source's saved parent snapshot replaces the ancestor bodies in
		// TranscriptHistory, so only its inherited limitations are disclosed.
		for _, m := range source.ParentHistory {
			addWarning(m.Text)
		}
	}
	if source.CaptureError == InterruptedTurn || !source.Complete {
		addWarning(c.host.InterruptedNote)
	}
	invocations, err := c.host.Invocations(s.Cwd(), s.ID())
	if err != nil {
		return p, err
	}
	encoded, err := json.Marshal(invocations)
	if err != nil {
		return p, err
	}
	hash.Write(encoded)
	p.invocations = invocations
	for _, v := range invocations {
		if v.Status == "running" || v.Status == "waiting" {
			return p, errors.New("finish or cancel this chat's pending applet work before changing provider")
		}
		if len(v.Artifacts) > 0 {
			addWarning("Applet artifacts remain owned by the earlier chat. Their binary contents and access permissions are not transferred. Open the result in Notifications or attach it to the continued chat when needed.")
		}
	}
	p.Revision = hex.EncodeToString(hash.Sum(nil))
	return p, nil
}
