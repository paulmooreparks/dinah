package bench

import "testing"

// The block every case below reads, written as a person would type it into
// workbench.md. It carries the three shapes the declaration admits: a scheme
// whose mapping demands an observation, a scheme whose mapping does not, and a
// scheme written as a bare hint string with no mapping at all.
const evidenceBlock = `evidence:
  test:
    hint: A Go test, written as the file path, a hash, and the test function name.
    observed: required
  attachment:
    hint: The 12-hex id of an attachment on this card.
    resolves: attachments
  record: The inspection record's reference number as the county issues it.
`

// TestAWorkbenchDeclaringNoEvidenceImposesNoObligation asserts that the
// presence test answers false where the key is absent, which is what leaves an
// undeclaring workbench free of the citation obligation.
func TestAWorkbenchDeclaringNoEvidenceImposesNoObligation(t *testing.T) {
	opened := benchDeclaring(t, "")
	if opened.EvidenceDeclared() {
		t.Error("a workbench carrying no evidence key reads as declaring one")
	}
	if opened.EvidenceObservedRequired("test") {
		t.Error("a scheme of a block that is not there reads as requiring an observation")
	}
}

// TestAnEmptyEvidenceBlockStillCountsAsDeclared asserts the rule the accessor's
// own comment states: the obligation turns on the block's presence rather than
// on any scheme under it being well formed.
func TestAnEmptyEvidenceBlockStillCountsAsDeclared(t *testing.T) {
	opened := benchDeclaring(t, "evidence:\n")
	if !opened.EvidenceDeclared() {
		t.Error("a workbench carrying the evidence key with no scheme beneath it reads as declaring none")
	}
	if opened.EvidenceObservedRequired("test") {
		t.Error("a scheme an empty block does not declare reads as requiring an observation")
	}
}

// TestOnlyASchemeDeclaringObservedRequiredDemandsOne asserts the read the cite
// refusal rests on, across the three shapes a declaration admits and one name
// the block does not carry at all.
func TestOnlyASchemeDeclaringObservedRequiredDemandsOne(t *testing.T) {
	opened := benchDeclaring(t, evidenceBlock)
	if !opened.EvidenceDeclared() {
		t.Fatal("the block is declared and reads as absent")
	}
	for _, c := range []struct {
		scheme string
		want   bool
		why    string
	}{
		{"test", true, "its mapping carries observed: required"},
		{"attachment", false, "its mapping carries no observed member"},
		{"record", false, "it is a bare hint string with no mapping"},
		{"invoice", false, "the block does not declare it"},
	} {
		if got := opened.EvidenceObservedRequired(c.scheme); got != c.want {
			t.Errorf("%s: wanted %v because %s, got %v", c.scheme, c.want, c.why, got)
		}
	}
}
