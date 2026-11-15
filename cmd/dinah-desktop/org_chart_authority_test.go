//go:build chatcleanup_acceptance

package main

import (
	"context"
	"net/http"
	"testing"

	"studio/internal/diagrampicture"
)

func TestOrgChartActionsDeleteRevokesRealDrawingLease(t *testing.T) {
	a, c, ws := orgActionsFixture(t)
	p := a.runtime.ProviderLocked("codex")
	go a.lifecycle().Watch("codex", p)
	t.Cleanup(func() { c.Close(); p.Wait() })
	id := "codex:a"
	a.runtime.PutConversationLocked(id, newSessionState(&sessionSeed{ID: id, Provider: "codex", Cwd: ws}, sessionInitial{Open: true}))
	env := a.chatCleanupAcceptanceDrawingLease(t, id, ws, func(context.Context, diagrampicture.Drawing) (diagrampicture.Picture, error) {
		t.Error("revoked request reached renderer")
		return diagrampicture.Picture{}, nil
	})
	v := applyOrg(t, a, OrgChartActionRequest{Kind: "delete_chat", ChatID: id})
	if v.Status != "succeeded" {
		t.Fatal(v)
	}
	r, _ := http.NewRequest("OPTIONS", env["DIAGRAM_PICTURE_ENDPOINT"]+diagrampicture.Route, nil)
	r.Header.Set("Authorization", "Bearer "+env["DIAGRAM_PICTURE_LEASE"])
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("old drawing authority returned %d", res.StatusCode)
	}
}
