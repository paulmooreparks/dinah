package bench

import "encoding/json"

// EvidenceKey is the frontmatter key carrying the workbench's declared
// evidence schemes, one nested block whose members name the schemes an item's
// citations entry may draw on.
const EvidenceKey = "evidence"

// ObservedRequired is the value a scheme's observed member carries when the
// declaration demands that a citation record what the check showed before the
// work and after it.
const ObservedRequired = "required"

// EvidenceDeclared reports whether the workbench declares an evidence block at
// all, on the presence test Has already answers for any other key.
//
// An empty block, meaning the key present with no scheme beneath it, still
// counts as declared. The citation obligation the format states turns on the
// block being there rather than on any scheme under it being well formed, and
// judging a scheme's own shape is what dinah check does.
func (b *Bench) EvidenceDeclared() bool {
	return b.FM.Has(EvidenceKey)
}

// EvidenceObservedRequired reports whether the named scheme's declaration
// carries observed: required.
//
// The block is read through blockValue, the generic reader the interchange
// already uses, rather than through parsing rules of this file's own. A scheme
// the block does not declare, one declared as a bare hint string with no
// nested mapping, and one whose mapping carries no observed member all answer
// false, which is the same absence-is-legal reading the citation obligation
// itself follows. Nothing here reports an error, because this is a boolean
// read of an optional declaration.
//
// Membership of the block is not asked about. A citation naming a scheme the
// workbench never declared is check.unknown-scheme's finding, and answering
// false here leaves it to that check rather than raising a second, competing
// one at write time.
func (b *Bench) EvidenceObservedRequired(scheme string) bool {
	var schemes map[string]json.RawMessage
	if err := json.Unmarshal(blockValue(b.FM, EvidenceKey), &schemes); err != nil {
		return false
	}
	declaration, declared := schemes[scheme]
	if !declared {
		return false
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(declaration, &members); err != nil {
		return false
	}
	var observed string
	if err := json.Unmarshal(members["observed"], &observed); err != nil {
		return false
	}
	return observed == ObservedRequired
}
