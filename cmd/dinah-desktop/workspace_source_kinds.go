package main

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"studio/internal/workspaces"
)

/* The Add Source dialog offers, after a folder on this computer, every kind
   of source the build registered with a form (workspaces.Attach). The page
   draws the form from WorkspaceSourceKinds and sends what the person filled
   in to AttachWorkspaceSource, which has the kind build the source and
   attaches it (docs/design/source-kinds.md). */

// SourceKindInfo is a kind of source the person can add, as the page draws it.
type SourceKindInfo struct {
	Kind   string             `json:"kind"`
	Label  string             `json:"label"`
	Help   string             `json:"help"`
	Fields []workspaces.Field `json:"fields"`
}

// WorkspaceSourceKinds lists the kinds of source that can be added besides
// a local folder, in the order of their labels.
func (a *App) WorkspaceSourceKinds() []SourceKindInfo {
	out := []SourceKindInfo{}
	for _, k := range workspaces.AttachableSourceKinds() {
		fields := k.Attach.Fields
		if fields == nil {
			fields = []workspaces.Field{}
		}
		out = append(out, SourceKindInfo{Kind: k.Kind, Label: k.Label, Help: k.Attach.Help, Fields: fields})
	}
	return out
}

// attachTimeout bounds a kind's check of a source against its site.
const attachTimeout = 20 * time.Second

// AttachWorkspaceSource adds a source of a registered kind to a workspace
// from the Add Source form's values. The kind checks the values and makes
// the source; the source gets its ID here.
func (a *App) AttachWorkspaceSource(workspaceID, kind string, values map[string]string) (WorkspaceRecord, error) {
	if _, err := a.workspaceSourceRecord(workspaceID); err != nil {
		return WorkspaceRecord{}, err
	}
	k, ok := workspaces.LookupSourceKind(kind)
	if !ok || k.Attach == nil || k.Attach.Build == nil {
		return WorkspaceRecord{}, errors.New("that kind of source cannot be added here")
	}
	if values == nil {
		values = map[string]string{}
	}
	for _, f := range k.Attach.Fields {
		v := strings.TrimSpace(values[f.ID])
		if f.Kind == "check" {
			if v != "true" {
				v = "false"
			}
		} else if f.Required && v == "" {
			return WorkspaceRecord{}, errors.New("fill in " + f.Label)
		}
		if f.MaxLength > 0 && len(v) > f.MaxLength {
			return WorkspaceRecord{}, errors.New(f.Label + " is too long")
		}
		values[f.ID] = v
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, attachTimeout)
	defer cancel()
	source, err := k.Attach.Build(ctx, values)
	if err != nil {
		return WorkspaceRecord{}, err
	}
	if source.Kind != kind || source.Path != "" {
		return WorkspaceRecord{}, errors.New("the source kind made a source of another kind")
	}
	source.Name = strings.TrimSpace(source.Name)
	if source.Name == "" {
		source.Name = k.Label
	}
	if !validWorkspaceName(source.Name) {
		source.Name = defaultWorkspaceLabel(source.Name)
	}
	source.ID = "src_" + rand.Text()
	source.MountAlias = ""
	return a.attachWorkspaceSource(workspaceID, source)
}
