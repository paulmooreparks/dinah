package template

import (
	"strings"
	"testing"

	"dinah/internal/bench"
)

// TestTheDeclaredNamesMatchTheEmbeddedTemplates holds the embedded
// templates/*.json set and the declared names list equal in both directions,
// the way internal/guide's TestTheReadingOrderPlacesEveryEmbeddedGuide holds
// its own two sets. Each direction loses something different: a template
// embedded and not named is served by nothing, since Known and Definition
// both read names rather than the directory; a name declared and never
// embedded is offered by nothing and refused as UnknownPath the moment
// anybody asks for it.
func TestTheDeclaredNamesMatchTheEmbeddedTemplates(t *testing.T) {
	entries, err := templates.ReadDir("templates")
	if err != nil {
		t.Fatalf("read the embedded templates: %v", err)
	}
	embedded := map[string]bool{}
	for _, entry := range entries {
		embedded[strings.TrimSuffix(entry.Name(), ".json")] = true
	}
	if len(embedded) == 0 {
		t.Fatal("the binary embeds no template, so this test reads nothing")
	}
	declared := map[string]bool{}
	for _, name := range names {
		declared[name] = true
	}
	for name := range embedded {
		if declared[name] {
			continue
		}
		t.Errorf("internal/template/templates/%s.json is embedded and names does not declare it, so no surface serves it", name)
	}
	for name := range declared {
		if embedded[name] {
			continue
		}
		t.Errorf("names declares %s and no internal/template/templates/%s.json is embedded, so a lookup refuses it", name, name)
	}
}

// TestThePipelineTemplateInstantiatesCleanly is dinah-547's acceptance
// criterion "init --from the template produces a workbench check reports
// clean", proven directly against the library rather than through the CLI: it
// instantiates the embedded pipeline template into a fresh directory, opens
// the result, runs Check, and asserts zero findings.
func TestThePipelineTemplateInstantiatesCleanly(t *testing.T) {
	definition, err := Definition("pipeline")
	if err != nil {
		t.Fatalf("read the pipeline template: %v", err)
	}
	container := t.TempDir()
	id, err := bench.ClaimWorkbenchID(container)
	if err != nil {
		t.Fatalf("claim a workbench id: %v", err)
	}
	root := container + "/" + bench.UserBaseName + "/" + id
	if err := bench.Instantiate(root, "px", "ops", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("open the instantiated workbench: %v", err)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("wanted zero findings, got %d: %+v", len(findings), findings)
	}
}
