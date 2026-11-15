package main

type ChatProgramCopy struct {
	SessionID    string `json:"sessionId"`
	Provider     string `json:"provider"`
	ProviderName string `json:"providerName"`
	Deletion     string `json:"deletion"`
	CopyRemains  bool   `json:"copyRemains"`
	Reason       string `json:"reason"`
}

type ChatCleanupRow struct {
	SessionID               string            `json:"sessionId"`
	SessionIDs              []string          `json:"sessionIds"`
	Title                   string            `json:"title"`
	WorkspaceID             string            `json:"workspaceId"`
	Workspace               string            `json:"workspace"`
	WorkspaceName           string            `json:"workspaceName"`
	Provider                string            `json:"provider"`
	ProviderName            string            `json:"providerName"`
	LastActivity            string            `json:"lastActivity"`
	MessageCount            int               `json:"messageCount"`
	MessageCountKnown       bool              `json:"messageCountKnown"`
	Open                    bool              `json:"open"`
	Working                 bool              `json:"working"`
	Pinned                  bool              `json:"pinned"`
	Eligible                bool              `json:"eligible"`
	Refusal                 string            `json:"refusal"`
	RemovedAt               string            `json:"removedAt"`
	ExpiresAt               string            `json:"expiresAt"`
	Restorable              bool              `json:"restorable"`
	RestoreRefusal          string            `json:"restoreRefusal"`
	ProviderDeletion        string            `json:"providerDeletion"`
	FinalDeleteConfirmation string            `json:"finalDeleteConfirmation"`
	ProgramCopies           []ChatProgramCopy `json:"programCopies"`
	RuntimeNote             string            `json:"runtimeNote"`
}

type ChatCleanupListing struct {
	Rows                    []ChatCleanupRow `json:"rows"`
	Removed                 []ChatCleanupRow `json:"removed"`
	RetentionDays           int              `json:"retentionDays"`
	GeneratedAt             string           `json:"generatedAt"`
	ProviderDeletionEnabled bool             `json:"providerDeletionEnabled"`
	FinalDeleteConfirmation string           `json:"finalDeleteConfirmation"`
}

type ChatCleanupResult struct {
	SessionID              string            `json:"sessionId"`
	OK                     bool              `json:"ok"`
	Status                 string            `json:"status"`
	Error                  string            `json:"error"`
	Row                    *ChatCleanupRow   `json:"row"`
	ProviderCopyRemains    bool              `json:"providerCopyRemains"`
	ProviderDeletionReason string            `json:"providerDeletionReason"`
	ProgramCopies          []ChatProgramCopy `json:"programCopies"`
}
