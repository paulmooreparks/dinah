package acp

import "encoding/json"

// The request and answer shapes of the ACP methods the client uses, from the
// protocol's schema. Fields the client does not read are left out.

type InitializeParams struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities"`
	ClientInfo         *Implementation    `json:"clientInfo,omitempty"`
}

type ClientCapabilities struct {
	FS       FSCapabilities       `json:"fs"`
	Terminal bool                 `json:"terminal"`
	Session  *SessionCapabilities `json:"session,omitempty"`
	Meta     *ClientMeta          `json:"_meta,omitempty"`
}

// ClientMeta is the protocol's extension point on the client's
// capabilities. Dinah uses it for the AIR extensions that the Claude adapter
// documents (docs/air-extensions.md in agentclientprotocol/claude-agent-acp);
// agents that do not know a key ignore it.
type ClientMeta struct {
	JetBrains *JetBrainsMeta `json:"jetbrains,omitempty"`
}

type JetBrainsMeta struct {
	AIR *AIRCapabilities `json:"air,omitempty"`
}

// AIRCapabilities lists the AIR extensions the client takes. "asyncTasks"
// has the agent report work that runs on after a turn, such as a backgrounded
// command, a background subagent or a turn the agent starts of its own, as
// async_task_spawned, async_task_progress and async_task_state_update
// session updates. "nativeSubagentSessions" has the agent report each
// subagent as a child session of its own, with subagent_spawned and
// subagent_state_update on the parent, as the ACP subagent RFD's draft of
// 31 August 2026 describes. Dinah does not take "backgroundSubagents", so
// the agent keeps a turn open while a subagent it started runs.
//
// The RFD's own client field, clientCapabilities.subagents, is not
// declared: its revised draft has a different wire form, so declaring it
// to an agent other than the Claude adapter could bring updates Dinah does
// not read.
type AIRCapabilities struct {
	Version      int      `json:"version"`
	Capabilities []string `json:"capabilities"`
}

// SessionCapabilities advertise, among other things, that the client can
// show boolean config options
// (https://agentclientprotocol.com/protocol/session-config-options).
type SessionCapabilities struct {
	ConfigOptions *ConfigOptionCapabilities `json:"configOptions,omitempty"`
}
type ConfigOptionCapabilities struct {
	Boolean *struct{} `json:"boolean,omitempty"`
}

// ClientCaps are the capabilities Dinah advertises: file reads and
// writes inside the workspace, boolean config options, and reports of
// background work and of subagents.
func ClientCaps() ClientCapabilities {
	return ClientCapabilities{
		FS:      FSCapabilities{ReadTextFile: true, WriteTextFile: true},
		Session: &SessionCapabilities{ConfigOptions: &ConfigOptionCapabilities{Boolean: &struct{}{}}},
		Meta:    &ClientMeta{JetBrains: &JetBrainsMeta{AIR: &AIRCapabilities{Version: 1, Capabilities: []string{"asyncTasks", "nativeSubagentSessions"}}}},
	}
}

// ConfigOption is one of the session's settings the agent offers, such as
// its model or thinking level. Options holds either values or groups of
// values, as the protocol allows; Values flattens them.
type ConfigOption struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Category     string          `json:"category,omitempty"`
	Type         string          `json:"type"`
	CurrentValue json.RawMessage `json:"currentValue"`
	Options      json.RawMessage `json:"options,omitempty"`
}

type SetConfigOptionParams struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Type      string `json:"type,omitempty"`
	Value     any    `json:"value"`
}

type ConfigOptionsResult struct {
	ConfigOptions []ConfigOption `json:"configOptions"`
}

type FSCapabilities struct {
	ReadTextFile  bool `json:"readTextFile"`
	WriteTextFile bool `json:"writeTextFile"`
}

type Implementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AgentInfo         *Implementation   `json:"agentInfo,omitempty"`
	AuthMethods       []AuthMethod      `json:"authMethods,omitempty"`
}

// AgentCapabilities say, among other things, whether the agent can reopen a
// session (loadSession) and list its sessions (sessionCapabilities.list).
// The client must not call session/load or session/list without them
// (https://agentclientprotocol.com/protocol/session-setup,
// https://agentclientprotocol.com/protocol/session-list).
type AgentCapabilities struct {
	LoadSession         bool               `json:"loadSession"`
	PromptCapabilities  PromptCapabilities `json:"promptCapabilities"`
	SessionCapabilities struct {
		List *struct{} `json:"list,omitempty"`
		// Delete says the agent supports session/delete, which removes a
		// session from its session/list results
		// (https://agentclientprotocol.com/protocol/v1/session-delete).
		Delete *struct{} `json:"delete,omitempty"`
		// Close says the agent supports session/close, which cancels any
		// work of an active session and frees what the agent holds for it
		// (https://agentclientprotocol.com/protocol/session-setup).
		Close *struct{} `json:"close,omitempty"`
		// Resume says the agent supports session/resume, which reopens a
		// session without returning its previous messages
		// (https://agentclientprotocol.com/protocol/session-setup).
		Resume *struct{} `json:"resume,omitempty"`
	} `json:"sessionCapabilities"`
}

type DeleteSessionParams struct {
	SessionID string `json:"sessionId"`
}

// CloseSessionParams are session/close's: the active session to close. The
// agent answers with an empty result.
type CloseSessionParams struct {
	SessionID string `json:"sessionId"`
}

type LoadSessionParams struct {
	SessionID  string `json:"sessionId"`
	Cwd        string `json:"cwd"`
	McpServers []any  `json:"mcpServers"`
}

type LoadSessionResult struct {
	ConfigOptions []ConfigOption `json:"configOptions,omitempty"`
}

type ListSessionsParams struct {
	Cwd    string `json:"cwd,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type SessionInfo struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	Title     string `json:"title,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type ListSessionsResult struct {
	Sessions   []SessionInfo `json:"sessions"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// AuthMethod is a way of signing in that the agent offers at initialize. A
// method without a type is of type "agent": the client asks the agent to
// carry it out with authenticate, and the agent does the rest, such as
// opening the browser (https://agentclientprotocol.com/protocol/v1/authentication).
type AuthMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
}

// AuthenticateParams ask the agent to sign the person in with one of the
// methods it offered. The answer is an empty object once it has.
type AuthenticateParams struct {
	MethodID string `json:"methodId"`
}

type NewSessionParams struct {
	Cwd        string `json:"cwd"`
	McpServers []any  `json:"mcpServers"`
}

type NewSessionResult struct {
	SessionID     string         `json:"sessionId"`
	ConfigOptions []ConfigOption `json:"configOptions,omitempty"`
}

type PromptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

// ContentBlock is one part of a prompt
// (https://agentclientprotocol.com/protocol/content): text, which every agent
// takes; a resource_link, which every agent takes too; an image, which needs
// the agent's image prompt capability; or an embedded resource, which needs
// its embeddedContext capability.
type ContentBlock struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	Data     string            `json:"data,omitempty"`     // image: base64
	MimeType string            `json:"mimeType,omitempty"` // image, resource_link
	URI      string            `json:"uri,omitempty"`      // resource_link
	Name     string            `json:"name,omitempty"`     // resource_link
	Title    string            `json:"title,omitempty"`    // resource_link
	Resource *EmbeddedResource `json:"resource,omitempty"` // resource
}

type EmbeddedResource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// PromptCapabilities say which content beyond text and links the agent
// takes in a prompt.
type PromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type PromptResult struct {
	StopReason string `json:"stopReason"`
}

type CancelParams struct {
	SessionID string `json:"sessionId"`
}

type ReadTextFileParams struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Line      *int   `json:"line,omitempty"`
	Limit     *int   `json:"limit,omitempty"`
}

type ReadTextFileResult struct {
	Content string `json:"content"`
}

type WriteTextFileParams struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Content   string `json:"content"`
}

type RequestPermissionParams struct {
	SessionID string             `json:"sessionId"`
	ToolCall  json.RawMessage    `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

// PermissionOutcome is the answer to session/request_permission: either the
// option the person chose, or cancelled when the turn was stopped first.
type PermissionOutcome struct {
	Outcome struct {
		Outcome  string `json:"outcome"`
		OptionID string `json:"optionId,omitempty"`
	} `json:"outcome"`
}

func Selected(optionID string) PermissionOutcome {
	var o PermissionOutcome
	o.Outcome.Outcome = "selected"
	o.Outcome.OptionID = optionID
	return o
}

func Cancelled() PermissionOutcome {
	var o PermissionOutcome
	o.Outcome.Outcome = "cancelled"
	return o
}
