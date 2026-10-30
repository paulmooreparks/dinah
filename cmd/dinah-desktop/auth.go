package main

import (
	"context"
	"errors"
	"time"

	"studio/internal/acp"
)

// Signing in to Devin goes through the protocol. Devin offers its ways of
// signing in when the app initializes it, and answers a request with the
// documented "Authentication required" error (-32000) when the person is not
// signed in. The page then offers Sign In to Devin, and SignIn asks Devin to
// carry out the method it offered (authenticate), which for Devin opens the
// browser on the organisation's sign-in page. The app never sees a password or
// a token; Devin keeps its own.
//
// The page hears about it through the agent:auth event:
//
//	{state: "required", method}   a request was refused until the person signs in
//	{state: "signing-in", method} SignIn is waiting for the browser
//	{state: "signed-in"}          Devin reports the sign-in done
//	{state: "failed", error}      the sign-in did not complete

// errSignIn is what a refused request returns to the page.
var errSignIn = errors.New("Devin needs you to sign in first. Choose Sign In to Devin, then try again.")

// call makes a request of Devin and, when Devin answers that the person must
// sign in first, tells the page so that it can offer the sign-in.
func (a *App) call(conn agentConnection, method string, params, out any) error {
	return a.callWithin(conn, 0, method, params, out)
}

// callWithin is call with a time limit, for a request that should answer
// quickly, such as a list, so that an agent that hangs is reported rather
// than waited on; 0 is no limit.
func (a *App) callWithin(conn agentConnection, limit time.Duration, method string, params, out any) error {
	ctx := context.Background()
	if limit > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, limit)
		defer cancel()
	}
	err := conn.Call(ctx, method, params, out)
	if acp.IsAuthRequired(err) {
		provider := a.providerOf(conn)
		if m, ok := providerModules[provider]; ok && m.AuthRequired != nil {
			return m.AuthRequired(a)
		}
		// A provider that signs in outside Dinah (AgentSignIn).
		name := providerName(provider)
		if name == "" {
			name = "The agent"
		}
		a.emit("agent:auth", map[string]any{"state": "required", "provider": provider})
		return errors.New(name + " needs you to sign in. Open Settings > Agents")
	}
	return err
}

// SignIn asks Devin to sign the person in, starting Devin first if it is not
// running. It returns when Devin reports the sign-in done or failed, which
// takes as long as the person needs in the browser.
func (a *App) SignIn() error {
	return a.AgentSignIn(a.defaultProvider())
}
