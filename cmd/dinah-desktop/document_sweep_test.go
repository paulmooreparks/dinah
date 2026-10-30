package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// The drafts of the Editor windows in a saved layout are found from the
// windows' keys, as the workbench writes them; other windows are not.
func TestEditorDraftsInTheLayout(t *testing.T) {
	key := func(kind, ws, path string) string {
		b, _ := json.Marshal([]string{kind, ws, path})
		return "item-" + base64.RawURLEncoding.EncodeToString(b)
	}
	editor, resource := key("editor", "w1", "0123456789abcdef0123456789abcdef"), key("workspace-resource", "w1", `["src","a.md"]`)
	layout := []byte(`{"version":1,"values":{"dinah-workbench-layout":"?open=workspace,` + editor + `,` + resource + `&top=` + editor + `","dinah-scroll:` + editor + `:document":{"top":3}}}`)
	got := editorDrafts(layout)
	if len(got) != 1 || !got["0123456789abcdef0123456789abcdef"] {
		t.Fatalf("drafts in the layout: %v", got)
	}
}
