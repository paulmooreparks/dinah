// Package applet discovers and executes reviewed bundles shipped with the app.
package applet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"studio/internal/applet/contract"
)

type Action = contract.Action
type Manifest = contract.Manifest
type Artifact = contract.Artifact
type Field = contract.Field
type Request = contract.Request
type Result = contract.Result
type Handler = contract.Handler
type Bundle = contract.Bundle
type Invocation = contract.Invocation

var WithInvocation = contract.WithInvocation
var InvocationFromContext = contract.InvocationFromContext

// Effectful says whether an action with this capability changes something
// the person owns or acts on their behalf: writing workspace files,
// changing settings, running a window's command, or putting a question to
// another chat. Reading, posting a notification, asking the person and
// creating an artifact do not.
func Effectful(capability string) bool {
	if strings.HasPrefix(capability, "process.run:") {
		return true
	}
	switch capability {
	case "workspace.write", "settings.write", "window.command", "chat.ask", "terminal.open":
		return true
	}
	return false
}

// Asks says whether Dinah asks the person before a chat runs this action.
// An action with a declared effect level asks for "history" and "network"
// only; one without a level asks when any capability is Effectful.
func Asks(action Action) bool {
	switch action.Effect {
	case "read", "local":
		return false
	case "history", "network":
		return true
	}
	return slices.ContainsFunc(action.Capabilities, Effectful)
}

func SupportedCapability(capability string) bool {
	if strings.HasPrefix(capability, "process.run:") && regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`).MatchString(strings.TrimPrefix(capability, "process.run:")) {
		return true
	}
	switch capability {
	case "artifact.create", "attention.request", "notification.post", "workspace.read", "workspace.write", "chat.read", "settings.read", "settings.write", "window.read", "window.command", "chat.ask", "terminal.open", "team.manage":
		return true
	}
	return false
}

type entry struct {
	action  Action
	handler Handler
	digest  string
}
type Registry struct {
	manifests []Manifest
	actions   map[string]entry
}

var providerID = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)
var actionID = regexp.MustCompile(`^[a-z][a-z0-9-]*@[1-9][0-9]*$`)
var outputKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
var packageVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// New assembles statically registered bundles. It does not install external code.
func New(bundles ...Bundle) (*Registry, error) {
	r := &Registry{actions: map[string]entry{}}
	ids := map[string]bool{}
	for _, b := range bundles {
		m := b.Manifest
		if !providerID.MatchString(m.ID) || ids[m.ID] || m.APIVersion != 1 || !packageVersion.MatchString(m.Version) || len(m.Actions) == 0 {
			return nil, fmt.Errorf("invalid or duplicate applet manifest %q", m.ID)
		}
		ids[m.ID] = true
		encoded, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		h.Write(encoded)
		keys := make([]string, 0, len(b.Sources))
		for k := range b.Sources {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(h, "\x00%d:%s:%d:", len(k), k, len(b.Sources[k]))
			h.Write(b.Sources[k])
		}
		digest := hex.EncodeToString(h.Sum(nil))
		if len(keys) == 0 {
			return nil, errors.New("a bundle must identify its implementation sources")
		}
		for _, a := range m.Actions {
			local, ok := strings.CutPrefix(a.Ref, m.ID+"/")
			if !ok || !actionID.MatchString(local) || r.actions[a.Ref].handler != nil || b.Handlers[a.Ref] == nil {
				return nil, fmt.Errorf("invalid or duplicate applet action %q", a.Ref)
			}
			if a.Replay != "pure" && a.Replay != "human" && a.Replay != "once" {
				return nil, fmt.Errorf("invalid replay policy for %s", a.Ref)
			}
			for _, cap := range a.Capabilities {
				if !SupportedCapability(cap) {
					return nil, fmt.Errorf("unsupported applet capability %q", cap)
				}
			}
			if _, err = compileSchema(a.InputSchema); err != nil {
				return nil, fmt.Errorf("%s input schema: %w", a.Ref, err)
			}
			if _, err = compileSchema(a.OutputSchema); err != nil {
				return nil, fmt.Errorf("%s output schema: %w", a.Ref, err)
			}
			r.actions[a.Ref] = entry{clone(a), b.Handlers[a.Ref], digest}
		}
		r.manifests = append(r.manifests, clone(m))
	}
	return r, nil
}
func clone[T any](v T) T {
	data, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(data, &out)
	return out
}
func (r *Registry) Catalog() []Manifest { return clone(r.manifests) }
func (r *Registry) Describe(ref string) (Action, error) {
	e, ok := r.actions[ref]
	if !ok {
		return Action{}, fmt.Errorf("unknown applet action %q", ref)
	}
	return clone(e.action), nil
}
func (r *Registry) Digest(ref string) (string, error) {
	e, ok := r.actions[ref]
	if !ok {
		return "", fmt.Errorf("unknown applet action %q", ref)
	}
	return e.digest, nil
}
func (r *Registry) Execute(ctx context.Context, ref, inputJSON string) (Result, error) {
	e, ok := r.actions[ref]
	if !ok {
		return Result{}, fmt.Errorf("unknown applet action %q", ref)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := Validate(e.action.InputSchema, inputJSON); err != nil {
		return Result{}, fmt.Errorf("invalid %s inputs: %w", ref, err)
	}
	result, err := e.handler(ctx, inputJSON)
	if err != nil {
		return Result{}, err
	}
	if err = ctx.Err(); err != nil {
		return Result{}, err
	}
	value, err := decodeJSON(result.OutputJSON)
	if err != nil {
		return Result{}, fmt.Errorf("invalid action output: %w", err)
	}
	output, ok := value.(map[string]any)
	if !ok {
		return Result{}, errors.New("action output must be a JSON object")
	}
	if result.Request != nil && (!slices.Contains(e.action.Capabilities, "attention.request") || e.action.Replay != "human") {
		return Result{}, errors.New("action returned an undeclared human request")
	}
	if len(result.Artifacts) > 0 && !slices.Contains(e.action.Capabilities, "artifact.create") {
		return Result{}, errors.New("action returned undeclared artifacts")
	}
	if result.Request != nil {
		if _, exists := output["response"]; exists {
			return Result{}, errors.New("provider cannot supply its own human response")
		}
		output["response"] = nil
	}
	if len(result.Artifacts) > 32 {
		return Result{}, errors.New("action returned too many artifacts")
	}
	for _, artifact := range result.Artifacts {
		if !outputKey.MatchString(artifact.Key) || len(artifact.Data) > 4<<20 {
			return Result{}, errors.New("invalid action artifact")
		}
		if _, exists := output[artifact.Key]; exists {
			return Result{}, errors.New("action artifact overwrites an output property")
		}
		output[artifact.Key] = nil
	}
	if result.Request == nil && len(result.Artifacts) == 0 {
		if err = Validate(e.action.OutputSchema, result.OutputJSON); err != nil {
			return Result{}, fmt.Errorf("invalid action output: %w", err)
		}
	}
	return result, nil
}
