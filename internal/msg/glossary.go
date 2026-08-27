package msg

import (
	_ "embed"
	"encoding/json"
)

//go:embed glossary.json
var glossaryData []byte

// glossaryTerm is one concept a translator repeatedly meets. EN is the
// phrase whose presence in the base English text triggers the check; it is
// deliberately a phrase rather than always a bare word, because a bare word
// can name a second, unrelated sense in this catalog (see the note on
// "root" in glossary.json's own card, dinah-252), and a trigger that fires
// on the wrong sense produces a false failure rather than a caught defect.
// Forms lists, per language tag, every rendering a translator is allowed to
// use; more than one entry lets a declared grammatical variant, or a
// compound the term is spelled inside, pass without loosening the check for
// an actual wrong word.
type glossaryTerm struct {
	EN    string              `json:"en"`
	Forms map[string][]string `json:"forms"`
}

// glossary is the declared term list, read once at package init.
// TestATranslationUsesTheDeclaredWord fails outright when it is empty, so a
// glossary that will not parse cannot quietly turn that guard into a no-op.
var glossary = loadGlossary()

// loadGlossary reads the embedded glossary. A file that will not parse
// yields no terms rather than panicking at init, and the guard over the
// catalogs is what reports the empty list.
func loadGlossary() []glossaryTerm {
	var terms []glossaryTerm
	if err := json.Unmarshal(glossaryData, &terms); err != nil {
		return nil
	}
	return terms
}
