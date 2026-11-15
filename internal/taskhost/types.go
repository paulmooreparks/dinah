// Package taskhost admits reviewed work and owns its revocable execution handles
// and durable operation receipts. Native work remains in the window's adapters.
package taskhost

import (
	"context"
	"encoding/json"
	"time"
)

type Error struct{ Code, Reason string }

func (e *Error) Error() string          { return e.Code + ": " + e.Reason }
func refusal(code, reason string) error { return &Error{code, reason} }

type Scope struct {
	Service    string `json:"service"`
	Workspace  string `json:"workspace"`
	Task       string `json:"task"`
	Generation string `json:"generation"`
	OwnerChat  string `json:"ownerChat,omitempty"`
}

// Binding is a captured local workspace identity, including its mount generation.
type Binding struct{ Path, Generation string }

type Inspection struct {
	Scope      Scope    `json:"scope"`
	Binding    Binding  `json:"binding"`
	Operations []string `json:"operations"`
	Rooms      bool     `json:"rooms"`
}

type ChatDescription struct {
	ChatID    string `json:"chatId"`
	Head      string `json:"head"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
	Program   string `json:"program"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
}

type ProgramDescription struct {
	Program string `json:"program"`
	Offered bool   `json:"offered"`
	Reason  string `json:"reason,omitempty"`
}

type ActionDescription struct {
	Reference    string          `json:"reference"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Replay       string          `json:"replay"`
	Capabilities []string        `json:"capabilities"`
	Digest       string          `json:"digest"`
}

type Permit struct {
	Kind    string          `json:"kind"`
	Target  string          `json:"target"`
	Program string          `json:"program,omitempty"`
	Digest  string          `json:"digest,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
}

type Plan struct {
	Scope    Scope     `json:"scope"`
	Permits  []Permit  `json:"permits"`
	Deadline time.Time `json:"deadline"`
	Rooms    bool      `json:"rooms"`
}

type Review struct {
	ID      string    `json:"id"`
	Plan    Plan      `json:"plan"`
	Expires time.Time `json:"expires"`
}

type Operation struct {
	ID      string          `json:"id"`
	Kind    string          `json:"kind"`
	Target  string          `json:"target"`
	Program string          `json:"program,omitempty"`
	Digest  string          `json:"digest,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
}

type Receipt struct {
	Disposition   string          `json:"disposition,omitempty"`
	ID            string          `json:"id"`
	Scope         Scope           `json:"scope"`
	Operation     string          `json:"operation"`
	Kind          string          `json:"kind"`
	Target        string          `json:"target"`
	Fingerprint   string          `json:"fingerprint"`
	State         string          `json:"state"`
	Code          string          `json:"code,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	Identity      string          `json:"identity,omitempty"`
	Dispatched    bool            `json:"dispatched,omitempty"`
	SetupFinished bool            `json:"setupFinished,omitempty"`
	SetupError    string          `json:"setupError,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
	Sequence      uint64          `json:"sequence"`
	Updated       time.Time       `json:"updated"`
}

type Outcome struct {
	State, Code, Reason, Identity string
	Result                        json.RawMessage
}

// Capture persists an identity at its first success boundary. The adapter must
// stop subsequent work when it returns an error and retain the returned identity.
type Progress struct {
	Identity                  string
	Dispatched, SetupFinished bool
	SetupError                string
	Result                    json.RawMessage
}

type Capture func(Progress) error

type Backend interface {
	Execute(context.Context, Scope, Operation, Capture) Outcome
}

type RoomResult struct {
	Identity string
	Result   json.RawMessage
}

// RoomAdapter is optional. Claim, root-post deduplication and correlated
// observation must all exist before a host advertises room work.
type RoomAdapter interface {
	Review(context.Context, Scope, Permit) (string, error)
	Ensure(context.Context, Scope, Operation, Capture) (RoomResult, error)
	Claim(context.Context, Scope, Operation, Capture) (RoomResult, error)
	Post(context.Context, Scope, Operation, Capture) (RoomResult, error)
	Observe(context.Context, Scope, Operation, Capture) (RoomResult, error)
	Release(context.Context, Scope, Operation, Capture) (RoomResult, error)
	Recover(context.Context, Scope, Operation, Capture) (RoomResult, error)
}

type Options struct {
	State string
	Now   func() time.Time
	// Resolve and Check read current local authority and never start a provider.
	Resolve func(context.Context, Scope) (Binding, error)
	Check   func(context.Context, Scope) error
	// QuestionRecipient resolves the asking chat once, when its question is admitted.
	QuestionRecipient func(Scope) string
	// HoldTask acquires the consumer record lock while reconciling receipts or
	// committing a person's question settlement against its current generation.
	HoldTask       func(context.Context, Scope) (func() error, error)
	Admit          func(context.Context, Scope, Operation) error
	TargetRevision func(context.Context, Scope, Permit) (string, error)
	Resource       func(context.Context, Scope, Operation) (string, error)
	Operations     func(context.Context, Scope) ([]string, error)
	DescribeChats  func(context.Context, Scope) ([]ChatDescription, error)
	CheckProgram   func(context.Context, Scope, string) (ProgramDescription, error)
	DescribeAction func(context.Context, Scope, string) (ActionDescription, error)
	// Confirm obtains actual person consent or validates current invocation grants.
	// It must never trust a caller label in request JSON.
	Confirm        func(context.Context, Review) error
	Backend        Backend
	Rooms          bool
	RoomsAvailable func() bool
	// Track registers work with the window's exit guard before launch.
	Track     func(Scope, Operation) (done func(), err error)
	Emit      func(Receipt)
	Lifecycle func(string)
	// Save injects receipt persistence failures in tests before the real write.
	Save func(Receipt) error
	// AfterSave observes the completed durable write before publication. Tests
	// can lose its response while reading the committed record independently.
	AfterSave func(Receipt) error
}

// Handle contains no serializable authority. A handle belongs to one Window.
type Handle struct {
	window *Window
	lease  *lease
}

type Recovery struct {
	Scope    Scope
	Receipts []Receipt
}

type Snapshot struct {
	AdmissionClosed bool
	Handles         int
	Workers         int
}
