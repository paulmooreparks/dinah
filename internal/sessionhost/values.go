// Package sessionhost holds conversation data and runtime prerequisites.
package sessionhost

import (
	_ "embed"
	"encoding/json"
	"studio/internal/acp"
	"studio/internal/workspaces"
)

//go:embed values.go
var ValuesSource []byte

type DeliveredMessage struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	From        string `json:"from"`
	FromSession string `json:"fromSession,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
	Time        string `json:"time"`
	Status      string `json:"status,omitempty"`
	// Room is the key of the group chat the message was posted in, empty for a message that was not a group's,
	// so a window can fold a run of a team's traffic (docs/plans/teams-ui.md).
	Room      string `json:"room,omitempty"`
	RoomTitle string `json:"roomTitle,omitempty"`
	// Prompt is on the first entry of a delivery only: the visible text of
	// the prompt the agent was given, as the saved copy keeps a person's
	// words. When the chat reopens and the agent replays that prompt as a
	// user message, it is matched against this, Dinah's own record, so that
	// the replay keeps the delivery as these entries (settleReplayedPrompt).
	Prompt string `json:"prompt,omitempty"`
}

type ConversationMessage struct {
	Images    []acp.ContentBlock `json:"images,omitempty"`
	Role      string             `json:"role"`
	Text      string             `json:"text"`
	Provider  string             `json:"provider"`
	SessionID string             `json:"sessionId"`
	// Delivered is set on an entry with the role "delivered": a message the
	// bus delivered into the chat (delivered.go).
	Delivered *DeliveredMessage `json:"delivered,omitempty"`
	// Kind is how a status row was said, such as "warning" for a line that
	// warns, so that a chat reopened from its saved copy draws it as it was.
	Kind string `json:"kind,omitempty"`
}

type Transcript struct {
	SessionID       string `json:"sessionId"`
	Provider        string `json:"provider"`
	Workspace       string `json:"workspace"`
	ParentSessionID string `json:"parentSessionId,omitempty"`
	// ParentWorkspace is the workspace of the chat this one continues, when it is
	// not this chat's own: a chat moved to another workspace records where it came
	// from, so a link that crosses workspaces is one that was chosen.
	ParentWorkspace string                `json:"parentWorkspace,omitempty"`
	Complete        bool                  `json:"complete"`
	PendingHandoff  bool                  `json:"pendingHandoff,omitempty"`
	Handoff         string                `json:"handoff,omitempty"`
	Summary         string                `json:"summary,omitempty"`
	ParentHistory   []ConversationMessage `json:"parentHistory,omitempty"`
	Unsupported     bool                  `json:"unsupported,omitempty"`
	CaptureError    string                `json:"captureError,omitempty"`
	Deleted         bool                  `json:"deleted,omitempty"`
	// StartedQueueIDs are the ids of the last queued messages whose turns started, kept so
	// that a message carried to this chat from another is known to have started here
	// (queue_restore.go). Copies saved before the list was kept lack it.
	StartedQueueIDs []string              `json:"startedQueueIds,omitempty"`
	Messages        []ConversationMessage `json:"messages"`
}

// startedQueueIDsKept is how many ids of started messages a chat's saved copy keeps.
const startedQueueIDsKept = 256

type Attachment struct {
	Kind string `json:"kind"` // "link", "file", "applet", "window", "chat" or "draft"
	// Window is the tool a window reference is to, by its launcher's ID
	// (the Files applet is "files"); Provider is then the applet that
	// gives the same tool's data, when it has one.
	Window string `json:"window,omitempty"`
	// SessionID is the chat a chat reference is to.
	SessionID string `json:"sessionId,omitempty"`
	Name      string `json:"name"`
	DraftID   string `json:"draftId,omitempty"` // draft: a document open in the Editor
	// Ref is a "resource": something in a workspace, chosen with + in Open.
	Ref      *workspaces.Ref `json:"ref,omitempty"`
	Provider string          `json:"provider,omitempty"` // applet: registered provider ID
	URI      string          `json:"uri,omitempty"`      // link: file:// or https://
	MimeType string          `json:"mimeType,omitempty"`
	Data     string          `json:"data,omitempty"` // file: base64
}

type DocRef struct {
	Path      string `json:"path"`
	Workspace string `json:"workspace,omitempty"`
	Title     string `json:"title"`
	// DraftID is the draft of a document open in the Editor.
	DraftID string `json:"draftId,omitempty"`
	// Selection is the passage selected in the Editor, widened to whole
	// paragraphs. It goes to the agent and not into the history.
	Selection string `json:"selection,omitempty"`
}

type SubagentInfo struct {
	ID       string `json:"id"`
	ParentID string `json:"parentId"`
	RootID   string `json:"rootId"`
	Name     string `json:"name"`
	Task     string `json:"task"`
	// Prompt is the exact prompt the agent gave the subagent, when it says;
	// only SubagentUpdates carries it, since it can be long.
	Prompt string `json:"prompt,omitempty"`
	State  string `json:"state"`
	// Local marks a disconnected state that Dinah gave the subagent itself,
	// because its chat's turn or its agent ended before the agent reported
	// its end (the RFD's "unknown outcomes on a live connection").
	Local     bool `json:"local,omitempty"`
	CanCancel bool `json:"canCancel"`
	// Started and Ended are when Dinah saw the subagent begin and end. A
	// subagent the agent replayed with its chat's history has neither, and
	// is Replayed.
	Started  string `json:"started,omitempty"`
	Ended    string `json:"ended,omitempty"`
	Replayed bool   `json:"replayed,omitempty"`
	// ThisRun says that Dinah saw the subagent start since Dinah itself
	// started. The chat's Subagents menu and tree show only these; the
	// others, which a chat's history brought back from an earlier run, are
	// the chat's earlier subagents (Paul, 28 October 2026). A subagent of
	// this run that the agent replays when its chat is opened again in the
	// same run keeps its times and stays this run's.
	ThisRun bool `json:"thisRun,omitempty"`
	// Trimmed says that some of the subagent's earliest updates are no
	// longer kept.
	Trimmed bool `json:"trimmed,omitempty"`
}

type SubagentRecord struct {
	Subagent SubagentInfo      `json:"subagent"`
	Updates  []json.RawMessage `json:"updates"`
	// Seq is the number of the last update the copy holds; live updates
	// carry their own.
	Seq int64 `json:"seq"`
}
