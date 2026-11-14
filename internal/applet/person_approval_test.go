package applet

import "testing"

func TestFreshApprovalCapabilityAlwaysAsks(t *testing.T) {
	if !SupportedCapability("person.approval") {
		t.Fatal("fresh approval is not supported")
	}
	for _, effect := range []string{"", "read", "local", "history", "network"} {
		action := Action{Effect: effect, Capabilities: []string{"person.approval"}}
		if !RequiresFreshApproval(action) || !Asks(action) {
			t.Fatalf("effect %q bypassed fresh approval", effect)
		}
	}
	if RequiresFreshApproval(Action{Capabilities: []string{"settings.write"}}) {
		t.Fatal("ordinary actions changed policy")
	}
}
