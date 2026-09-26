package verb

import "runtime/debug"

// BuildIdentity names the build this binary is: ToolRelease, followed by +
// and the first twelve characters of the vcs.revision setting
// runtime/debug.ReadBuildInfo reports, with .dirty appended when its
// vcs.modified setting is true. A binary carrying no VCS settings answers
// ToolRelease alone.
//
// dinah and dinah-tui are built from one tree and share the workbench format
// and this library, so two builds of one tree answer the same identity and
// two builds of different trees answer different ones. dinah tui hands its
// identity to dinah-tui, which refuses to run for another.
func BuildIdentity() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ToolRelease
	}
	return identityOf(ToolRelease, info.Settings)
}

// identityOf composes a build identity from a release and the build settings
// ReadBuildInfo reports, which is BuildIdentity with its inputs passed in so
// a test can hold every case.
func identityOf(release string, settings []debug.BuildSetting) string {
	revision, modified := "", false
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		return release
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	identity := release + "+" + revision
	if modified {
		identity += ".dirty"
	}
	return identity
}
