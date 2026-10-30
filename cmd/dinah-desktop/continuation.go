package main

import (
	"errors"
	"strings"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

const continuationContextHeading = sessionhost.ContinuationContextHeading

type ConversationMessage = sessionhost.ConversationMessage

type conversationTranscript = sessionhost.Transcript

type SessionContinuationView = sessionhost.SessionContinuationView

type SessionContinuationPreview = sessionhost.SessionContinuationPreview

type SessionContinuationResult = sessionhost.SessionContinuationResult

func transcriptPath(id string) string { return sessionhost.TranscriptPath(configDir(), id) }
func readTranscript(id string) (*conversationTranscript, error) {
	return sessionhost.ReadTranscript(configDir(), id)
}
func writeTranscript(tr *conversationTranscript) error {
	if folderClosed() {
		return errFolderClosed
	}
	return sessionhost.WriteTranscript(configDir(), tr)
}

// errOutsideHistory is what saved copies said when the agent's replay held
// messages they lacked, before such a replay became the saved copy. Saved
// copies keep this text and it is compared with them, so it keeps Dinah's
// name whatever the product is.
const errOutsideHistory = sessionhost.OutsideHistory

// errInterruptedTurn is what a saved copy says when its last turn never
// finished being captured, most often because Dinah closed during it. The
// copy holds everything up to that point, so the chat can still be
// recovered or continued, with a note that the turn's end may be missing
// (Paul, 26 October 2026).
const errInterruptedTurn = sessionhost.InterruptedTurn

// interruptedTurn reports whether a saved copy's only flaw is a last turn
// that was cut off. A turn in progress also leaves the copy marked
// incomplete, so callers check that no turn is running first.
func interruptedTurn(tr *conversationTranscript) bool {
	return sessionhost.InterruptedHistory(tr)
}

// interruptedTurnNote is the line a handoff carries after a cut-off turn.
var interruptedTurnNote = "The last turn of this chat was cut off before it finished, when " + product.Name + " closed or the agent stopped, so anything the agent did after the last message above is missing from this history."

func replayRepresented(saved, replayed []ConversationMessage) bool {
	return sessionhost.ReplayRepresented(saved, replayed)
}

func visibleConversationText(text string) string {
	for _, marker := range []string{appletContextHeading, continuationContextHeading} {
		if at := strings.Index(text, marker); at >= 0 {
			text = text[:at]
		}
	}
	// The marker for the hooks follows the applet context, but an agent may
	// replay it as a part of its own (steers.go).
	return redactAppletContexts(bus.StripMarkers(text))
}

func transcriptHistory(id, workspace string, seen map[string]bool) ([]ConversationMessage, error) {
	return sessionhost.TranscriptHistory(configDir, id, workspace, seen)
}

// continuationContext is the handoff text for a history, built to the
// budget of the provider it goes to (continuation_budget.go). It returns the
// text and how many earlier messages it summarised.
func continuationContext(history []ConversationMessage, budget int) (string, int) {
	return compactHandoff(history, budget)
}

func (a *App) PreviewSessionContinuation(id, provider string) (SessionContinuationPreview, error) {
	return a.continuations().PreviewSession(id, provider)
}

func (a *App) ContinueSessionWithProvider(id, provider, revision string) (SessionContinuationResult, error) {
	return a.continuations().ContinueSession(id, provider, revision)
}

// PreviewSessionWorkspaceMove is what continuing the chat in another workspace would
// hand the new chat, with the facts the confirmation shows. workspace is the
// workspace's folder, as NewSessionAt takes it.
func (a *App) PreviewSessionWorkspaceMove(id, workspace string) (SessionContinuationPreview, error) {
	return a.continuations().PreviewWorkspace(id, workspace)
}

// ContinueSessionInWorkspace moves a chat to another workspace by continuing it
// there with the same provider. The old chat stays where it was, linked to the new
// one, and when the move fails it is as it was.
func (a *App) ContinueSessionInWorkspace(id, workspace, revision string) (SessionContinuationResult, error) {
	return a.continuations().ContinueInWorkspace(id, workspace, revision)
}

// DismissSwitchFailure clears the failure a chat's switch left in its information.
func (a *App) DismissSwitchFailure(id string) error {
	s := a.session(id)
	if s == nil {
		return errors.New("that chat is not open")
	}
	s.DismissSwitchFailure()
	sessionhost.EmitSession(a.emit, s)
	return nil
}

func (a *App) SessionContinuation(id string) (SessionContinuationView, error) {
	return a.continuations().View(id)
}

// SavedConversationView is a chat as Dinah saved it, for showing
// when its window is restored without asking the agent to replay it.
// Available is false when
// there is no complete saved copy to show, and the agent's replay is then
// the only history.
type SavedConversationView = sessionhost.SavedConversationView

// SavedConversation is the saved copy of a chat.
func (a *App) SavedConversation(id string) SavedConversationView {
	return a.continuations().Saved(id)
}

func conversationChild(id string) string {
	links, _ := conversationLinks()
	return links[id]
}

type transcriptHead = sessionhost.TranscriptHead

func conversationLinks() (map[string]string, map[string]*transcriptHead) {
	return sessionhost.ConversationLinks(configDir())
}

// savedSummary is the nearest summary kept beside a chat or its ancestors:
// everything before that chat, one line a message. It stands in for the
// earlier history when that history cannot be read.
func savedSummary(id string) string {
	return sessionhost.SavedSummary(configDir, id)
}

func conversationAppletInvocations(workspace, id string) ([]AppletInvocation, error) {
	saved, err := appletStore().Saved(workspace)
	if err != nil {
		return nil, err
	}
	var out []AppletInvocation
	for _, v := range saved {
		if v.OwnerKind == "chat" && v.OwnerID == id {
			out = append(out, v)
		}
	}
	return out, nil
}

func (a *App) ResolveSessionContinuation(id string) (string, error) {
	return a.continuations().Resolve(id)
}
