package release

// Target is one platform and architecture pair a CLI release builds and ships.
// The JSON tags spell the keys release.yml's build matrix reads, so the
// serialized form of Targets can be handed straight to fromJSON as the
// matrix's include list.
type Target struct {
	// GOOS is the operating system the binary is built for, spelled as Go
	// spells it.
	GOOS string `json:"goos"`
	// GOARCH is the architecture the binary is built for, spelled as Go
	// spells it.
	GOARCH string `json:"goarch"`
	// Ext is the filename extension the platform's executables carry,
	// including its leading dot, and it is empty everywhere but Windows.
	Ext string `json:"ext"`
}

// BinaryName is the dist/ filename release.yml's build job writes for t,
// which is dinah-<goos>-<goarch><ext>. It matches the -o path that job's
// build step composes from the same three fields.
func (t Target) BinaryName() string {
	return "dinah-" + t.GOOS + "-" + t.GOARCH + t.Ext
}

// Targets is the CLI's six supported platform and architecture pairs, in the
// order release.yml's build matrix and its release-time asset check both read
// them. That workflow names no platform itself, so an entry added or dropped
// here reaches its matrix and its asset check together.
//
// This is not the only copy of the list in the repository.
// .github/workflows/promote.yml spells the same six pairs in a shell loop of
// its own, builds the stable channel's binaries from that loop, and has no
// check holding it against this declaration. A platform change therefore has
// to be made there by hand as well, and nothing fails if it is not.
var Targets = []Target{
	{GOOS: "windows", GOARCH: "amd64", Ext: ".exe"},
	{GOOS: "windows", GOARCH: "arm64", Ext: ".exe"},
	{GOOS: "linux", GOARCH: "amd64"},
	{GOOS: "linux", GOARCH: "arm64"},
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "arm64"},
}
