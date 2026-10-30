// Package contract contains the provider API shared by bundled applets.
package contract

import (
	"context"
	"encoding/json"
)

type Action struct {
	Ref          string          `json:"ref"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Capabilities []string        `json:"capabilities"`
	Replay       string          `json:"replay"`
	// Effect is a declared effect level ("read", "local", "history" or
	// "network") from a signed extension manifest. Bundled applets leave it
	// empty, and consent then follows the capabilities.
	Effect string `json:"effect,omitempty"`
}
type Manifest struct {
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	APIVersion  int      `json:"apiVersion"`
	Actions     []Action `json:"actions"`
}
type Artifact struct {
	Key, Name, MIME string
	Data            []byte
}
type Field struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Choices  []string `json:"choices,omitempty"`
}
type Request struct {
	Title  string  `json:"title"`
	Body   string  `json:"body"`
	Fields []Field `json:"fields"`
}

// Result is what an action returns. OutputJSON is the structured result,
// checked against the action's output schema. Text, when the action gives
// it, is the same result written in Markdown for a person or an agent to
// read, as MCP splits a tool result into content and structuredContent. A
// result with no text is shown as its output, indented, by whoever shows it.
type Result struct {
	OutputJSON string
	Text       string
	Artifacts  []Artifact
	Request    *Request
}
type Handler func(context.Context, string) (Result, error)

// Invocation is supplied by the host, never decoded from action inputs.
type Invocation struct {
	Workspace, OwnerKind, OwnerID, ID string
}

type invocationKey struct{}

func WithInvocation(ctx context.Context, invocation Invocation) context.Context {
	return context.WithValue(ctx, invocationKey{}, invocation)
}

func InvocationFromContext(ctx context.Context) (Invocation, bool) {
	v, ok := ctx.Value(invocationKey{}).(Invocation)
	return v, ok
}

// Bundle is assembled only by reviewed, statically linked application code.
// Sources includes every implementation dependency in the provider digest.
type Bundle struct {
	Manifest Manifest
	Handlers map[string]Handler
	Sources  map[string][]byte
}

const ArtifactSchema = `{"type":"object","additionalProperties":false,"required":["id","invocationId","name","mime","size","sha256"],"properties":{"id":{"type":"string"},"invocationId":{"type":"string"},"name":{"type":"string"},"mime":{"type":"string"},"size":{"type":"integer","minimum":0},"sha256":{"type":"string","pattern":"^[0-9a-f]{64}$"}}}`
