package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"path/filepath"

	"studio/internal/applet"
	"studio/internal/resource/localfs"
)

//go:embed applet_workspace.go
var workspaceAppletSource []byte

const workspaceDescribeAction = "dinah.workspace/describe@1"
const workspaceIdentityAction = "dinah.workspace/describe@2"
const workspaceListAction = "dinah.workspace/list@1"
const workspaceReadAction = "dinah.workspace/read@1"

func builtinWorkspace(ctx context.Context) (string, error) {
	v, ok := applet.InvocationFromContext(ctx)
	if !ok || v.Workspace == "" || !filepath.IsAbs(v.Workspace) {
		return "", errors.New("this action works on a workspace, and it was not started in one (in the Terminal, run it from a folder under /workspaces)")
	}
	return v.Workspace, ctx.Err()
}

func builtinJSON(value any) (applet.Result, error) {
	b, err := json.Marshal(value)
	return applet.Result{OutputJSON: string(b)}, err
}

func (a *App) workspaceAppletBundle() (applet.Bundle, error) {
	describeOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["name"],"properties":{"name":{"type":"string"}}}`)
	identityOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["id","name","sources"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"sources":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["id","kind","name"],"properties":{"id":{"type":"string"},"kind":{"type":"string","enum":["local-folder"]},"name":{"type":"string"}}}}}}`)
	listOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","entries","truncated"],"properties":{"path":{"type":"string"},"truncated":{"type":"boolean"},"entries":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["name","path","directory"],"properties":{"name":{"type":"string"},"path":{"type":"string"},"directory":{"type":"boolean"}}}}}}`)
	readOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","text","revision"],"properties":{"path":{"type":"string"},"text":{"type":"string"},"revision":{"type":"string","pattern":"^[0-9a-f]{64}$"}}}`)
	manifest := applet.Manifest{ID: "dinah.workspace", Version: "1.4.0", APIVersion: 1, Title: "Workspace", Description: "Inspect the calling workspace and read its ordinary text files; make folders and write files in its main folder.", Actions: []applet.Action{
		{Ref: workspaceDescribeAction, Title: "Describe Workspace", Description: "Return the calling workspace name. This does not change the selected workspace.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), OutputSchema: describeOutput, Capabilities: []string{"workspace.read"}, Replay: "pure"},
		{Ref: workspaceIdentityAction, Title: "Describe Workspace Identity", Description: "Return the registered calling workspace identity, display name and source identities. Host paths are excluded. This does not select or modify a workspace.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), OutputSchema: identityOutput, Capabilities: []string{"workspace.read"}, Replay: "pure"},
		{Ref: workspaceListAction, Title: "List Workspace Files", Description: "List a directory in the calling workspace's primary local VFS source. Hidden files and symbolic links are excluded. Return the first 500 directory entries through the shared VFS.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"path":{"type":"string","maxLength":1024}}}`), OutputSchema: listOutput, Capabilities: []string{"workspace.read"}, Replay: "pure"},
		{Ref: workspaceReadAction, Title: "Read Workspace Text", Description: "Read a regular UTF-8 text file of at most 1 MiB. Use a workspace-relative path. Hidden paths and symbolic links are excluded.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path"],"properties":{"path":{"type":"string","minLength":1,"maxLength":1024}}}`), OutputSchema: readOutput, Capabilities: []string{"workspace.read"}, Replay: "pure"},
	}}
	handlers := map[string]applet.Handler{}
	handlers[workspaceDescribeAction] = func(ctx context.Context, _ string) (applet.Result, error) {
		ws, err := builtinWorkspace(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		return builtinJSON(map[string]any{"name": filepath.Base(ws)})
	}
	handlers[workspaceListAction] = func(ctx context.Context, input string) (applet.Result, error) {
		return a.workspaceReadOperation(ctx, input, true)
	}
	handlers[workspaceIdentityAction] = func(ctx context.Context, _ string) (applet.Result, error) {
		vfs, err := a.scopedWorkspaceVFS(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		records := vfs.records()
		if len(records) != 1 {
			return applet.Result{}, errors.New("the calling workspace is unavailable")
		}
		record := records[0]
		sources := make([]map[string]string, 0, len(record.Sources))
		for _, source := range record.Sources {
			if source.Kind == "local-folder" {
				name := source.Name
				if name == "" {
					name = record.Name
				}
				sources = append(sources, map[string]string{"id": source.ID, "kind": source.Kind, "name": name})
			}
		}
		return builtinJSON(map[string]any{"id": record.ID, "name": record.Name, "sources": sources})
	}
	handlers[workspaceReadAction] = func(ctx context.Context, input string) (applet.Result, error) {
		return a.workspaceReadOperation(ctx, input, false)
	}
	a.addWorkspaceResourceActions(&manifest, handlers)
	a.addWorkspaceMountActions(&manifest, handlers)
	a.addWorkspaceWriteActions(&manifest, handlers)
	return applet.Bundle{Manifest: manifest, Handlers: handlers, Sources: map[string][]byte{"applet_workspace.go": workspaceAppletSource, "applet_workspace_write.go": workspaceWriteAppletSource, "workspace_identity.go": workspaceIdentitySource, "workspace_sources.go": workspaceSourcesSource, "workspace_resources.go": workspaceResourcesSource, "workspace_resource_gateway.go": workspaceResourceGatewaySource, "workspace_attachments.go": workspaceAttachmentsSource, "applet_workspace_resources.go": workspaceResourceAppletSource, "workspace_vfs.go": workspaceVFSSource, "applet_workspace_vfs.go": workspaceVFSAppletSource, "attributes.go": workspaceAttributeSource, "resource/localfs": localfs.ImplementationSource}}, nil
}

// workspaceRelativePath applies the local folder provider's path rules.
func workspaceRelativePath(raw string) (string, error) { return localfs.CleanPath(raw) }

func (a *App) workspaceReadOperation(ctx context.Context, input string, directory bool) (applet.Result, error) {
	vfs, err := a.scopedWorkspaceVFS(ctx)
	if err != nil {
		return applet.Result{}, err
	}
	var in struct {
		Path string `json:"path"`
	}
	if err = json.Unmarshal([]byte(input), &in); err != nil {
		return applet.Result{}, err
	}
	name, err := workspaceRelativePath(in.Path)
	if err != nil {
		return applet.Result{}, err
	}
	records := vfs.records()
	if len(records) != 1 {
		return applet.Result{}, errors.New("the calling workspace is unavailable")
	}
	record := records[0]
	ref := ResourceRef{WorkspaceID: record.ID, SourceID: record.PrimarySourceID, Path: filepath.ToSlash(name)}
	if name == "." {
		ref.Path = ""
	}
	source, err := vfs.resourceSource(ref)
	if err != nil || source.Kind != "local-folder" {
		return applet.Result{}, errors.New("this relative-path action needs a primary local source; use mounted resource actions for other sources")
	}
	if directory {
		type entry struct {
			Name      string `json:"name"`
			Path      string `json:"path"`
			Directory bool   `json:"directory"`
		}
		out := []entry{}
		truncated := false
		for start := 0; start >= 0 && start < 500; {
			page, err := vfs.listResource(ref, start, 100)
			if err != nil {
				return applet.Result{}, err
			}
			for _, item := range page.Entries {
				out = append(out, entry{item.Title, item.Ref.Path, item.Directory})
			}
			start = page.NextStart
			truncated = start >= 500
		}
		return builtinJSON(map[string]any{"path": filepath.ToSlash(name), "entries": out, "truncated": truncated})
	}
	read, err := vfs.readResource(ref)
	if err != nil {
		return applet.Result{}, err
	}
	return builtinJSON(map[string]any{"path": filepath.ToSlash(name), "text": read.Text, "revision": read.Revision})
}
