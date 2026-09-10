package mcp

import (
	"strings"
	"testing"

	"dinah/internal/msg"
	"dinah/internal/verb"
)

// schemaKindsSentinel stands in for {kinds} while the help.reference-kinds
// template is rendered backwards, so that the kinds region is isolated before
// anything is split. It carries no letter of any language and no separator.
const schemaKindsSentinel = "\x01KINDS\x01"

// TestEveryReferenceTakingToolPublishesTheKindsItsReferenceMayName is the
// independent proof for this head, the way
// TestEveryHelpPageNamesTheReferenceKindsItDeclares is for the terminal.
//
// It builds its expected clause from the catalogue entries and the declaration,
// and it names verb.ArgumentMeaning nowhere. Its sibling
// TestTheFieldToolsCarryTheSameSentencesTheTerminalPrints compares the schema
// against that function, so once tools.go calls it too the two sides of that
// check are one expression and a defect inside the function passes it. This one
// cannot pass that way, because nothing it computes goes through the renderer
// under test.
//
// An agent reading a tool schema asks the same question a person asks a help
// page, so a schema that published the bare sentence while the terminal printed
// the clause would be this card's own defect one head over.
func TestEveryReferenceTakingToolPublishesTheKindsItsReferenceMayName(t *testing.T) {
	library := newLevelledLibrary(t)
	catalog := msg.For(msg.Base)

	roster := map[string]bool{}
	for _, command := range verb.ReferenceTakingCommands() {
		roster[command] = true
	}
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}

	read := 0
	for _, entry := range tools {
		if !roster[entry.command] {
			continue
		}
		kinds, declared := verb.ReferenceKindsFor(entry.command)
		if !declared || len(kinds) == 0 {
			t.Errorf("%s is on the references roster and internal/verb declares no kinds for it", entry.command)
			continue
		}
		var param verb.Param
		found := false
		for _, candidate := range verb.Params(entry.command) {
			if candidate.Guide == "references" {
				param = candidate
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is on the references roster and declares no parameter naming that guide", entry.command)
			continue
		}
		if exemptArgument(entry.name, param.Name) {
			continue
		}
		read++

		summary := catalog.T(param.SummaryKey(entry.command))
		rendered := catalog.T("help.reference-kinds", "summary", summary, "kinds", schemaKindsSentinel)
		prefix, suffix, split := strings.Cut(rendered, schemaKindsSentinel)
		if !split {
			t.Fatal("the base help.reference-kinds entry does not substitute {kinds}, so the clause cannot be reversed")
		}

		published := propertyDescription(t, schemaOf(t, library, entry.name), param.Name)
		if !strings.HasPrefix(published, prefix) || !strings.HasSuffix(published, suffix) {
			t.Errorf("the %s tool describes %s as %q, which is not the summary wrapped in the kinds template", entry.name, param.Name, published)
			continue
		}
		region := published[len(prefix) : len(published)-len(suffix)]

		printed := map[string]bool{}
		for _, label := range strings.Split(region, verb.ReferenceKindSeparator) {
			printed[label] = true
		}
		for _, kind := range kinds {
			label := catalog.T(kind.MessageKey())
			if !printed[label] {
				t.Errorf("internal/verb declares that %s's reference may name %s and the %s tool's %s property does not publish the label %q", entry.command, kind, entry.name, param.Name, label)
			}
			delete(printed, label)
		}
		for label := range printed {
			t.Errorf("the %s tool's %s property publishes the label %q and internal/verb declares no kind of %s carrying it", entry.name, param.Name, label, entry.command)
		}
	}

	t.Logf("%d reference-taking tools read", read)
	if read == 0 {
		t.Fatal("the tool roster carried no reference-taking command, so this check swept nothing")
	}
}
