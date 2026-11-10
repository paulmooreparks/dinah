package workspaces

import "testing"

// A kind's word that a source changed goes to the function the program gave,
// and nowhere when it gave none.
func TestSourceChangedCallsTheFunctionGiven(t *testing.T) {
	t.Cleanup(func() { OnSourceChanged(nil) })
	if SourceChangesHeard() {
		t.Fatal("a function is given before any was")
	}
	SourceChanged("src_one")
	var got []string
	OnSourceChanged(func(id string) { got = append(got, id) })
	if !SourceChangesHeard() {
		t.Fatal("the function given is not heard")
	}
	SourceChanged("src_one")
	SourceChanged("")
	if len(got) != 1 || got[0] != "src_one" {
		t.Fatalf("the function was called with %v", got)
	}
	OnSourceChanged(nil)
	SourceChanged("src_two")
	if len(got) != 1 || SourceChangesHeard() {
		t.Fatalf("a function taken away was still called: %v", got)
	}
}
