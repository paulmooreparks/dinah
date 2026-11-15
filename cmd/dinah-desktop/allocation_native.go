package main

import (
	"context"
	"errors"

	"studio/internal/acp"
)

// allocationRequest stays inside the host and never enters a wire encoding.
type allocationRequest struct {
	params   any
	received func(string) error
}

type receiptConnection interface {
	NewSessionWithReceipt(context.Context, any, *acp.NewSessionResult, func(string) error) error
}

func (a *App) allocateNativeSession(conn agentConnection, params any, received func(string) error) (acp.NewSessionResult, error) {
	var result acp.NewSessionResult
	err := a.call(conn, "session/new", allocationRequest{params: params, received: received}, &result)
	return result, err
}

func callNativeAllocation(ctx context.Context, conn agentConnection, request allocationRequest, out any) error {
	if specialized, ok := conn.(receiptConnection); ok {
		result, ok := out.(*acp.NewSessionResult)
		if !ok {
			return errors.New("allocation result must be a new-session result")
		}
		return specialized.NewSessionWithReceipt(ctx, request.params, result, request.received)
	}
	if err := conn.Call(ctx, "session/new", request.params, out); err != nil {
		return err
	}
	result, ok := out.(*acp.NewSessionResult)
	if !ok {
		return errors.New("allocation result must be a new-session result")
	}
	return request.received(result.SessionID)
}
