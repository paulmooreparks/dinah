package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"

	"studio/internal/applet"
)

//go:embed applet_skills.go
var skillsAppletSource []byte

func (a *App) skillsAppletBundle() (applet.Bundle, error) {
	item := `{"type":"object","additionalProperties":false,"required":["id","name","description","hint","origin","revision"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"description":{"type":"string"},"hint":{"type":"string"},"origin":{"type":"string"},"revision":{"type":"string"}}}`
	manifest := applet.Manifest{ID: "dinah.skills", Version: "1.0.0", APIVersion: 1, Title: "Skills", Description: "Discover and read skill packages in the calling workspace through the VFS.", Actions: []applet.Action{
		{Ref: "dinah.skills/list@1", Title: "List Skills", Description: "List skills owned by the calling workspace, including their stable resource identities and instruction revisions.", Replay: "pure", Capabilities: []string{"workspace.read"}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["skills","problems"],"properties":{"skills":{"type":"array","items":` + item + `},"problems":{"type":"array","items":{"type":"string"}}}}`)},
		{Ref: "dinah.skills/read@1", Title: "Read Skill", Description: "Read instructions for a skill returned by List Skills. This does not run the skill or change permissions.", Replay: "pure", Capabilities: []string{"workspace.read"}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string","minLength":1,"maxLength":8192}}}`), OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["id","text","revision"],"properties":{"id":{"type":"string"},"text":{"type":"string"},"revision":{"type":"string"}}}`)},
	}}
	handlers := map[string]applet.Handler{}
	handlers["dinah.skills/list@1"] = func(ctx context.Context, _ string) (applet.Result, error) {
		record, err := a.callingWorkspaceRecord(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		catalog := a.skillCatalog(record.ID, false)
		items := []map[string]string{}
		for _, skill := range catalog.Skills {
			items = append(items, map[string]string{"id": skill.ID, "name": skill.Name, "description": skill.Description, "hint": skill.Hint, "origin": skill.Origin, "revision": skill.Revision})
		}
		return builtinJSON(map[string]any{"skills": items, "problems": catalog.Problems})
	}
	handlers["dinah.skills/read@1"] = func(ctx context.Context, input string) (applet.Result, error) {
		record, err := a.callingWorkspaceRecord(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		var in struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal([]byte(input), &in); err != nil {
			return applet.Result{}, err
		}
		for _, skill := range a.skillCatalog(record.ID, false).Skills {
			if skill.ID != in.ID {
				continue
			}
			read, err := (workspaceVFS{service: a.resourceService(), allowedWorkspace: record.ID}).readResource(*skill.Ref)
			if err != nil {
				return applet.Result{}, err
			}
			return builtinJSON(map[string]string{"id": skill.ID, "text": read.Text, "revision": read.Revision})
		}
		return applet.Result{}, errors.New("this skill is unavailable in the calling workspace")
	}
	return applet.Bundle{Manifest: manifest, Handlers: handlers, Sources: map[string][]byte{"applet_skills.go": skillsAppletSource, "skill_catalog.go": skillCatalogSource, "workspace_resources.go": workspaceResourcesSource, "workspace_vfs.go": workspaceVFSSource, "workspace_resource_gateway.go": workspaceResourceGatewaySource}}, nil
}
