package answer

import (
	"maps"
	"slices"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/msg"
)

// RefusalText is a refusal's sentence composed from the catalog, in the
// pieces a head lays out. The terminal prints a listing between the sentence
// and its fragments where the shape declares one, and the pages write the
// whole sentence as one line, so the composition stops short of joining them.
type RefusalText struct {
	// Shape is the refusal's declared shape, nil for a name the contract
	// does not declare.
	Shape *contract.Shape
	// Values are the values every piece was rendered from.
	Values map[string]string
	// Sentence is the base entry rendered: the raising command's variant
	// where the shape declares one, the absent form where the shape's
	// subject carries no value. For an undeclared name it is the
	// refusal.unknown sentence.
	Sentence string
	// Fragments are the fragments whose condition holds, rendered, in the
	// order the shape declares them. The one next step the alternation
	// picked stands among them at its declared position, and each carries
	// the leading punctuation its position needs.
	Fragments []string
	// Next is the index in Fragments of the next step, or -1 where the
	// shape picked none.
	Next int
}

// Tail joins the fragments, with the next step or without it.
func (c RefusalText) Tail(withNext bool) string {
	var joined strings.Builder
	for i, fragment := range c.Fragments {
		if i == c.Next && !withNext {
			continue
		}
		joined.WriteString(fragment)
	}
	return joined.String()
}

// RefusalSentence composes a refusal's sentence for a person to read. The
// terminal and the pages both compose through it, so the two cannot word one
// refusal two ways. The values are RefusalValues' for the command that raised
// the refusal, and card, where it is not empty, is the reference of the card
// the answer carried, which fills the card slot over any value the raise site
// attached, because the raise site could not always name the card.
func RefusalSentence(r *msg.Renderer, command, card string, refusal *contract.Refusal) RefusalText {
	values := RefusalValues(command, refusal)
	if card != "" {
		values[contract.ValueCard] = card
	}
	composed := RefusalText{Shape: contract.ShapeOf(refusal.Name), Values: values, Next: -1}
	if composed.Shape == nil {
		composed.Sentence = r.T("refusal.unknown", "name", refusal.Name, "detail", refusal.Detail)
		return composed
	}
	shape := composed.Shape
	pairs := make([]string, 0, 2*len(values))
	for _, key := range slices.Sorted(maps.Keys(values)) {
		pairs = append(pairs, key, values[key])
	}
	key := "refusal." + shape.Name
	if shape.Variant(values[contract.ValueCommand]) {
		key = shape.VariantKeyOf(values[contract.ValueCommand])
	}
	if shape.Subject != "" && values[shape.Subject] == "" {
		key = shape.AbsentKeyOf(key)
	}
	composed.Sentence = r.T(key, pairs...)
	next := nextStepOf(shape, values)
	for _, fragment := range shape.Fragments {
		named := shape.NamedInNextStep(fragment.Key)
		if named && fragment.Key != next {
			continue
		}
		if !named && !holds(fragment, values) {
			continue
		}
		if fragment.Key == next {
			composed.Next = len(composed.Fragments)
		}
		composed.Fragments = append(composed.Fragments, r.T(fragment.Key, pairs...))
	}
	return composed
}

// nextStepOf reads the alternation and returns the key of the one fragment
// that renders: the first whose condition holds, and never more than one. The
// last member carries no condition, so a shape the guard has passed always
// answers with a key.
//
// The winner renders at the position its fragment holds in the declared list
// rather than after every other fragment, because a clause split out of a base
// entry sat where the sentence put it. dinah.usage is where that is visible:
// its next step was written ahead of the dash hint, so it is declared ahead of
// it and it renders ahead of it.
func nextStepOf(shape *contract.Shape, values map[string]string) string {
	for _, named := range shape.NextStep {
		fragment := shape.Fragment(named)
		if fragment != nil && holds(*fragment, values) {
			return named
		}
	}
	return ""
}

// holds reports whether a fragment's condition is satisfied: a When names a
// value that is present and non-empty, an Unless names one that is not, a
// WhenCommand names the command the reader typed, and a fragment carrying none
// of the three always renders.
func holds(fragment contract.Fragment, values map[string]string) bool {
	if fragment.When != "" {
		return values[fragment.When] != ""
	}
	if fragment.Unless != "" {
		return values[fragment.Unless] == ""
	}
	if fragment.WhenCommand != "" {
		return values[contract.ValueCommand] == fragment.WhenCommand
	}
	return true
}
