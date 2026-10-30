package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"strings"

	"studio/internal/applet"
	"studio/internal/resource"
	"studio/internal/resource/localfs"
)

//go:embed applet_workspace_write.go
var workspaceWriteAppletSource []byte

// The workspace applet's write actions. They write only in the calling
// workspace's main folder, never in a dot folder or an AGENTS.md, and never replace a file unless the caller gives the revision
// it read, so an agent cannot overwrite what it did not see.
const (
	workspaceMakeFolderAction = "dinah.workspace/make-folder@1"
	workspaceWriteFileAction  = "dinah.workspace/write-file@1"
	workspaceWriteJSONAction  = "dinah.workspace/write-json@1"
)

// writablePath checks a main-folder path against the write policy and
// returns it in the provider's form.
func writablePath(raw string) (string, error) {
	clean, err := workspaceRelativePath(raw)
	if err != nil {
		return "", err
	}
	if clean == "." {
		return "", errors.New("give a path inside the workspace's main folder")
	}
	parts := strings.Split(strings.ReplaceAll(clean, "\\", "/"), "/")
	// Windows strips a name's trailing dots and takes a name with "~" as an
	// 8.3 short alias, so "AGENTS.md." and "AGENTS~1.MD" would reach what the
	// policy refuses; names of either shape are refused outright.
	for _, p := range parts {
		if strings.HasSuffix(p, ".") || strings.Contains(p, "~") {
			return "", errors.New("a name in the path ends in a dot or has a ~, which Windows reads as another name")
		}
	}
	if strings.EqualFold(parts[len(parts)-1], "AGENTS.md") {
		return "", errors.New("AGENTS.md is not written by actions")
	}
	return clean, nil
}

// mainFolder is the calling workspace's primary local folder.
func (a *App) mainFolder(ctx context.Context) (localfs.Folder, WorkspaceRecord, error) {
	record, err := a.callingWorkspaceRecord(ctx)
	if err != nil {
		return localfs.Folder{}, record, err
	}
	for _, s := range record.Sources {
		if s.ID == record.PrimarySourceID && s.Kind == "local-folder" {
			return localfs.Folder{Root: s.Path}, record, nil
		}
	}
	return localfs.Folder{}, record, errors.New("the workspace has no main local folder to write in")
}

func (a *App) addWorkspaceWriteActions(manifest *applet.Manifest, handlers map[string]applet.Handler) {
	str := `{"type":"string","minLength":1,"maxLength":1024}`
	written := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","revision"],"properties":{"path":{"type":"string"},"revision":{"type":"string"}}}`)
	manifest.Actions = append(manifest.Actions,
		applet.Action{Ref: workspaceMakeFolderAction, Title: "Make a Folder", Description: "Make a folder, and the folders above it, in the calling workspace's main folder. A folder that exists is left as it is. Dot folders and AGENTS.md are refused.",
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path"],"properties":{"path":` + str + `}}`), OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path"],"properties":{"path":{"type":"string"}}}`), Capabilities: []string{"workspace.write"}, Replay: "once"},
		applet.Action{Ref: workspaceWriteFileAction, Title: "Write a File", Description: "Write a text file in the calling workspace's main folder. Without a revision the file must not exist yet; with one, the file is replaced only while it is still at that revision (from Read Workspace Text). Dot folders and AGENTS.md are refused.",
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","text"],"properties":{"path":` + str + `,"text":{"type":"string","maxLength":1048576},"revision":{"type":"string","pattern":"^([0-9a-f]{64})?$"}}}`), OutputSchema: written, Capabilities: []string{"workspace.write"}, Replay: "once"},
		applet.Action{Ref: workspaceWriteJSONAction, Title: "Write a JSON File", Description: "Write a value as an indented JSON file in the calling workspace's main folder, under the same rules as Write a File. The value is a JSON object or array, or a text that contains one, such as an agent's reply, in its one fenced json block or as the whole text. The result gives the value written, so later steps can use its fields.",
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","value"],"properties":{"path":` + str + `,"value":{},"revision":{"type":"string","pattern":"^([0-9a-f]{64})?$"}}}`), OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["path","revision","value"],"properties":{"path":{"type":"string"},"revision":{"type":"string"},"value":{}}}`), Capabilities: []string{"workspace.write"}, Replay: "once"},
	)
	handlers[workspaceMakeFolderAction] = func(ctx context.Context, input string) (applet.Result, error) {
		var in struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return applet.Result{}, err
		}
		p, err := writablePath(in.Path)
		if err != nil {
			return applet.Result{}, err
		}
		folder, record, err := a.mainFolder(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		if err := folder.MakeFolder(ctx, toSlash(p), true); err != nil && !errors.Is(err, localfs.ErrExists) {
			return applet.Result{}, vfsError(in.Path, err)
		}
		a.emitWorkspaceMountChanged("content-changed", record.ID, record.PrimarySourceID)
		return builtinJSON(map[string]any{"path": toSlash(p)})
	}
	write := func(ctx context.Context, raw, text, revision string) (applet.Result, error) {
		p, err := writablePath(raw)
		if err != nil {
			return applet.Result{}, err
		}
		folder, record, err := a.mainFolder(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		if dir := path.Dir(toSlash(p)); dir != "." {
			if err := folder.MakeFolder(ctx, dir, true); err != nil && !errors.Is(err, localfs.ErrExists) {
				return applet.Result{}, vfsError(raw, err)
			}
		}
		got, err := folder.WriteText(ctx, toSlash(p), revision, text)
		if errors.Is(err, localfs.ErrExists) || errors.Is(err, resource.ErrConflict) {
			if revision == "" {
				return applet.Result{}, errors.New(raw + " already exists; read it and give its revision to replace it")
			}
			return applet.Result{}, errors.New(raw + " changed since it was read; read it again")
		}
		if err != nil {
			return applet.Result{}, vfsError(raw, err)
		}
		a.emitWorkspaceMountChanged("content-changed", record.ID, record.PrimarySourceID)
		return builtinJSON(map[string]any{"path": toSlash(p), "revision": got})
	}
	handlers[workspaceWriteFileAction] = func(ctx context.Context, input string) (applet.Result, error) {
		var in struct {
			Path     string `json:"path"`
			Text     string `json:"text"`
			Revision string `json:"revision"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return applet.Result{}, err
		}
		return write(ctx, in.Path, in.Text, in.Revision)
	}
	handlers[workspaceWriteJSONAction] = func(ctx context.Context, input string) (applet.Result, error) {
		var in struct {
			Path     string          `json:"path"`
			Value    json.RawMessage `json:"value"`
			Revision string          `json:"revision"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return applet.Result{}, err
		}
		v, err := jsonValue(in.Value)
		if err != nil {
			return applet.Result{}, err
		}
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return applet.Result{}, err
		}
		res, err := write(ctx, in.Path, string(b)+"\n", in.Revision)
		if err != nil {
			return res, err
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
			return applet.Result{}, err
		}
		out["value"] = v
		return builtinJSON(out)
	}
}

var fencedJSON = regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```")

// jsonValue is the value write-json writes: a JSON object or array as it is,
// or a text that contains one, such as an agent's reply that the person
// approved, which gives it in its one fenced json block or as the whole text.
func jsonValue(raw json.RawMessage) (any, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	text, ok := v.(string)
	if !ok {
		return v, nil
	}
	blocks := fencedJSON.FindAllStringSubmatch(text, -1)
	switch {
	case len(blocks) == 1:
		text = blocks[0][1]
	case len(blocks) > 1:
		return nil, errors.New("the text has more than one json block, so which to write is not clear")
	}
	var inner any
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &inner); err != nil {
		return nil, errors.New("the text has no JSON value to write: " + err.Error())
	}
	switch inner.(type) {
	case map[string]any, []any:
		return inner, nil
	}
	return nil, errors.New("the JSON to write must be an object or an array")
}

func toSlash(p string) string { return strings.ReplaceAll(p, "\\", "/") }
