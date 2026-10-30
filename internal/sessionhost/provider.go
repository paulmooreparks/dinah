package sessionhost

import (
	"context"
	_ "embed"
	"slices"
	"sync"

	"studio/internal/acp"
)

//go:embed provider.go
var ProviderSource []byte

type Connection interface {
	Call(context.Context, string, any, any) error
	Notify(string, any) error
	Done() <-chan struct{}
	Err() error
	Close()
	RegisterTraceSecret(string)
	RegisterTraceRedactor(func(string) string)
}
type Trace interface{ Flush() error }

// Provider is an opaque generation. Its initialization and connection identity
// never change. Finish follows the host's cleanup and events, then releases Wait.
type Provider struct {
	conn  Connection
	init  acp.InitializeResult
	gone  chan struct{}
	once  sync.Once
	trace Trace
}

func NewProvider(conn Connection, init acp.InitializeResult, trace Trace) *Provider {
	return &Provider{conn: conn, init: copyInitialization(init), gone: make(chan struct{}), trace: trace}
}
func (p *Provider) Connection() Connection               { return p.conn }
func (p *Provider) Initialization() acp.InitializeResult { return copyInitialization(p.init) }
func (p *Provider) Wait()                                { <-p.gone }
func (p *Provider) Finished() <-chan struct{}            { return p.gone }
func (p *Provider) Finish()                              { p.once.Do(func() { close(p.gone) }) }
func (p *Provider) Flush() {
	if p.trace != nil {
		_ = p.trace.Flush()
	}
}
func copyInitialization(in acp.InitializeResult) acp.InitializeResult {
	out := in
	if in.AgentInfo != nil {
		v := *in.AgentInfo
		out.AgentInfo = &v
	}
	out.AuthMethods = slices.Clone(in.AuthMethods)
	cap := &out.AgentCapabilities.SessionCapabilities
	if cap.List != nil {
		cap.List = &struct{}{}
	}
	if cap.Delete != nil {
		cap.Delete = &struct{}{}
	}
	if cap.Close != nil {
		cap.Close = &struct{}{}
	}
	if cap.Resume != nil {
		cap.Resume = &struct{}{}
	}
	return out
}
