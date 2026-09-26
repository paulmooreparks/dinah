package verb

import (
	"runtime/debug"
	"testing"
)

// TestTheBuildIdentityNamesTheReleaseAndTheRevision holds identityOf in the
// three cases section 15.5 names: a clean revision, a modified one, and no
// VCS settings at all.
func TestTheBuildIdentityNamesTheReleaseAndTheRevision(t *testing.T) {
	revision := debug.BuildSetting{Key: "vcs.revision", Value: "0123456789abcdef0123"}
	cases := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{"a clean revision", []debug.BuildSetting{revision, {Key: "vcs.modified", Value: "false"}}, "0.1.0+0123456789ab"},
		{"a modified revision", []debug.BuildSetting{revision, {Key: "vcs.modified", Value: "true"}}, "0.1.0+0123456789ab.dirty"},
		{"no VCS settings", []debug.BuildSetting{{Key: "GOOS", Value: "linux"}}, "0.1.0"},
	}
	for _, c := range cases {
		if got := identityOf("0.1.0", c.settings); got != c.want {
			t.Errorf("%s: the identity is %q, wanted %q", c.name, got, c.want)
		}
	}
	if got := BuildIdentity(); len(got) < len(ToolRelease) || got[:len(ToolRelease)] != ToolRelease {
		t.Errorf("this binary's identity %q does not begin with ToolRelease %q", got, ToolRelease)
	}
}
