package bus

import (
	"testing"

	"studio/internal/profile"
)

// A process is hosted when any of the profile's hosted variables is set.
func TestHostedReadsEveryHostedVariable(t *testing.T) {
	vars := profile.Current().HostedVars()
	for _, name := range vars {
		t.Setenv(name, "")
	}
	if Hosted() {
		t.Fatal("hosted with no variable set")
	}
	for _, name := range vars {
		t.Setenv(name, "1")
		if !Hosted() {
			t.Fatalf("%s did not say hosted", name)
		}
		t.Setenv(name, "")
	}
}
