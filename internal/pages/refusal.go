package pages

import (
	"strings"

	"dinah/internal/contract"
	"dinah/internal/msg"
)

// RefusalSentence renders a refusal's catalog sentence and its next step
// from the values answer.RefusalValues composed, choosing the entry the way
// the terminal chooses it: the raising command's variant where the shape
// declares one, the absent form where the shape's subject carries no value,
// the fragments whose condition holds spliced on, and the one next step the
// shape's alternation picks, returned apart and carrying its own leading
// punctuation, so the two read as one sentence when written together. The
// listing the terminal prints under some refusals is not drawn, because it is
// composed from the terminal's own session rather than from the refusal.
func RefusalSentence(r *msg.Renderer, name string, values map[string]string) (sentence, next string) {
	shape := contract.ShapeOf(name)
	if shape == nil {
		return r.T("refusal.unknown", "name", name, "detail", values["detail"]), ""
	}
	pairs := make([]string, 0, 2*len(values))
	for _, key := range sortedKeys(values) {
		pairs = append(pairs, key, values[key])
	}
	key := "refusal." + shape.Name
	if shape.Variant(values[contract.ValueCommand]) {
		key = shape.VariantKeyOf(values[contract.ValueCommand])
	}
	if shape.Subject != "" && values[shape.Subject] == "" {
		key = shape.AbsentKeyOf(key)
	}
	var spliced strings.Builder
	spliced.WriteString(r.T(key, pairs...))
	chosen := ""
	for _, named := range shape.NextStep {
		if fragment := shape.Fragment(named); fragment != nil && holds(*fragment, values) {
			chosen = named
			break
		}
	}
	for _, fragment := range shape.Fragments {
		if shape.NamedInNextStep(fragment.Key) {
			continue
		}
		if holds(fragment, values) {
			spliced.WriteString(r.T(fragment.Key, pairs...))
		}
	}
	if chosen != "" {
		next = r.T(chosen, pairs...)
	}
	return spliced.String(), next
}

// holds reports whether a fragment's condition is satisfied, by the rule the
// terminal applies.
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
