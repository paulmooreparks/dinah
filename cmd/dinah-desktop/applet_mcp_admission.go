package main

import (
	"context"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// appletMCPAdmission bounds tool work while SDK control messages bypass it.
// All active/queue transitions share mu. No handler or transport runs under mu.
//
// A call waiting here does not stop the SDK reading further messages: the
// Concurrency section of the SDK's docs/protocol.md (v1.8.0) documents that
// calls such as tools/call are handled asynchronously with respect to each
// other. Cancellation and ping therefore still arrive while the queue is full.
type appletMCPAdmission struct {
	mu     sync.Mutex
	active int
	queue  []*appletMCPWaiting
}

type appletMCPWaiting struct {
	ctx      context.Context
	ready    chan struct{}
	admitted bool
}

func (a *appletMCPAdmission) acquire(ctx context.Context) error {
	a.mu.Lock()
	if err := ctx.Err(); err != nil {
		a.mu.Unlock()
		return err
	}
	if a.active < 4 {
		a.active++
		a.mu.Unlock()
		return nil
	}
	if len(a.queue) >= 32 {
		a.mu.Unlock()
		return &jsonrpc.Error{Code: -32000, Message: "applet helper overloaded; try again later"}
	}
	w := &appletMCPWaiting{ctx: ctx, ready: make(chan struct{})}
	a.queue = append(a.queue, w)
	a.mu.Unlock()
	select {
	case <-w.ready:
		if err := ctx.Err(); err != nil {
			a.release()
			return err
		}
		return nil
	case <-ctx.Done():
		a.mu.Lock()
		if w.admitted {
			a.mu.Unlock()
			a.release()
		} else {
			for i, q := range a.queue {
				if q == w {
					a.queue = append(a.queue[:i], a.queue[i+1:]...)
					break
				}
			}
			a.mu.Unlock()
		}
		return ctx.Err()
	}
}

func (a *appletMCPAdmission) release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for len(a.queue) > 0 {
		w := a.queue[0]
		a.queue = a.queue[1:]
		if w.ctx.Err() != nil {
			continue
		}
		w.admitted = true
		close(w.ready)
		return
	}
	a.active--
}

func (a *appletMCPAdmission) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != "tools/call" {
			return next(ctx, method, req)
		}
		if err := a.acquire(ctx); err != nil {
			return nil, err
		}
		defer a.release()
		return next(ctx, method, req)
	}
}
