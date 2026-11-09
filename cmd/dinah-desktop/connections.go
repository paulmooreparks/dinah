package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"studio/internal/connections"
)

/* Settings, Connections: the services the build connects to with an
   address and a token, as the build registered them (internal/connections).
   The page shows each connection's address and whether a token is saved,
   never the token, and offers Save, Test and Forget. Dinah registers no
   connection, so the section is hidden in a Dinah build. */

// ConnectionView is a connection as Settings shows it.
type ConnectionView struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Help           string `json:"help"`
	Address        string `json:"address"`
	DefaultAddress string `json:"defaultAddress"`
	HasToken       bool   `json:"hasToken"`
	Testable       bool   `json:"testable"`
	// Problem says why the saved settings could not be read, when they
	// could not: the keychain refusing, for one.
	Problem string `json:"problem,omitempty"`
}

// Connections lists the registered connections with their saved addresses.
func (a *App) Connections() []ConnectionView {
	out := []ConnectionView{}
	for _, c := range connections.All() {
		view := ConnectionView{ID: c.ID, Label: c.Label, Help: c.Help, DefaultAddress: c.DefaultAddress, Testable: c.Test != nil}
		saved, err := c.Load()
		if err != nil {
			view.Problem = err.Error()
		} else {
			view.Address = saved.Address
			view.HasToken = saved.Token != ""
		}
		out = append(out, view)
	}
	return out
}

// SetConnection saves a connection's address and, when one is given, its
// token; an empty token keeps the saved one.
func (a *App) SetConnection(id, address, token string) error {
	c, ok := connections.Lookup(id)
	if !ok {
		return connections.ErrUnknown
	}
	address = strings.TrimRight(strings.TrimSpace(address), "/")
	if address == "" {
		address = c.DefaultAddress
	}
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return errors.New("use a web address, such as https://example.com")
	}
	token = strings.TrimSpace(token)
	if err := c.Save(connections.Settings{Address: address, Token: token}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// RemoveConnection forgets a connection's saved address and token.
func (a *App) RemoveConnection(id string) error {
	c, ok := connections.Lookup(id)
	if !ok {
		return connections.ErrUnknown
	}
	if c.Remove == nil {
		return errors.New("this connection cannot be forgotten")
	}
	if err := c.Remove(); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// TestConnection reaches a connection's service with its saved settings and
// returns a line about the sign-in.
func (a *App) TestConnection(id string) (string, error) {
	c, ok := connections.Lookup(id)
	if !ok {
		return "", connections.ErrUnknown
	}
	if c.Test == nil {
		return "", errors.New("this connection cannot be tested")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return c.Test(ctx)
}
