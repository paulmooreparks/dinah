package main

import (
	"encoding/json"
	"os"
	"testing"

	"studio/internal/applet"
)

// The focused headless suite writes actual responses from the host and
// packaged applets. Check them again with the broker's Go validator.
func TestContentHostActionOutputs(t *testing.T) {
	path := os.Getenv("DINAH_CONTENT_OUTPUTS")
	if path == "" {
		t.Skip("set DINAH_CONTENT_OUTPUTS to the focused headless outputs.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var records []struct {
		Ref    string `json:"ref"`
		Error  string `json:"error"`
		Result struct {
			Output json.RawMessage `json:"output"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	want := map[string]json.RawMessage{}
	for _, name := range []string{"diagrams", "docs", "slides"} {
		bundle, err := NewApp().contentAppletBundle(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range bundle.Manifest.Actions {
			want[action.Ref] = action.OutputSchema
		}
	}
	seen := map[string]bool{}
	for _, record := range records {
		if record.Error != "" {
			t.Errorf("%s: %s", record.Ref, record.Error)
		}
		schema, ok := want[record.Ref]
		if !ok {
			t.Errorf("undeclared action %s", record.Ref)
			continue
		}
		if err := applet.Validate(schema, string(record.Result.Output)); err != nil {
			t.Errorf("%s: %v; output %s", record.Ref, err, record.Result.Output)
		}
		seen[record.Ref] = true
	}
	for _, ref := range []string{
		"yavc.diagrams/new@1", "yavc.diagrams/open-file@1", "yavc.diagrams/open@1", "yavc.diagrams/save@1", "yavc.diagrams/close@1",
		"yavc.docs/new@1", "yavc.docs/new-template@1", "yavc.docs/open-file@1", "yavc.docs/open@1", "yavc.docs/save@1", "yavc.docs/close@1",
		"yavc.slides/present@1", "yavc.slides/save@1", "yavc.slides/close@1",
	} {
		if !seen[ref] {
			t.Errorf("missing headless response for %s", ref)
		}
	}
}
