package verb

import (
	"runtime/debug"
	"testing"
)

// TestTheBuildIdentityNamesTheReleaseAndTheRevision holds identityOf in the
// three cases section 15.5 names: a clean revision, a modified one, and no
// VCS settings at all, and finds that BuildIdentity is IdentityOfBuild of
// this binary's own build information. It compares identities composed from
// a release it passes in, never the tool's own release number.
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
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("this test binary carries no build information")
	}
	if got, want := BuildIdentity(), IdentityOfBuild(info); got != want {
		t.Errorf("BuildIdentity answers %q, and IdentityOfBuild of this binary's own build information %q", got, want)
	}
}
