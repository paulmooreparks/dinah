package main

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"studio/internal/services"
)

/* A product built from this source adds applets of its own: page files in
   the product directory, listed in product-applets.json, and the Go
   services they call, registered with internal/services at start. The page
   reaches a service through CallService alone, so the App's own methods
   stay Dinah's (docs/design/product-applets.md). */

// serviceCallTimeout bounds one call of a product service, which may reach
// a site.
const serviceCallTimeout = 2 * time.Minute

var serviceMethodPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// CallService calls a method of a registered product service with its
// arguments, a JSON array, and returns the answer as JSON.
func (a *App) CallService(service, method, args string) (string, error) {
	s, ok := services.Lookup(service)
	if !ok {
		return "", errors.New("this build has no service named " + service)
	}
	if !serviceMethodPattern.MatchString(method) {
		return "", errors.New("invalid service method name")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, serviceCallTimeout)
	defer cancel()
	raw := json.RawMessage(args)
	if len(raw) == 0 {
		raw = json.RawMessage("[]")
	}
	result, err := s.Call(ctx, method, raw)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ProductServices lists the services this build registered, for the page
// to know what it may call.
func (a *App) ProductServices() []string { return services.Names() }
