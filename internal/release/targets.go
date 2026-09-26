package release

import "encoding/json"

// Target is one platform and architecture pair a CLI release builds and ships.
// Its JSON form, which MarshalJSON writes, carries the keys release.yml's
// build matrix reads, so the serialized form of Targets can be handed straight
// to fromJSON as the matrix's include list.
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

// BinaryName is the dist/ filename release.yml's build job writes dinah to
// for t, which is dinah-<goos>-<goarch><ext>. The build step takes it from
// the matrix's binary key.
func (t Target) BinaryName() string {
	return "dinah-" + t.GOOS + "-" + t.GOARCH + t.Ext
}

// TUIBinaryName is the dist/ filename release.yml's build job writes
// dinah-tui, the terminal UI's program, to for t, which is
// dinah-tui-<goos>-<goarch><ext>. The build step takes it from the matrix's
// tui_binary key.
func (t Target) TUIBinaryName() string {
	return "dinah-tui-" + t.GOOS + "-" + t.GOARCH + t.Ext
}

// Names are the dist/ filenames a release publishes for t: dinah first, then
// dinah-tui.
func (t Target) Names() []string {
	return []string{t.BinaryName(), t.TUIBinaryName()}
}

// MarshalJSON writes t as one entry of the build matrix: its three fields,
// and the two filenames the build step writes, under binary and tui_binary.
func (t Target) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		GOOS      string `json:"goos"`
		GOARCH    string `json:"goarch"`
		Ext       string `json:"ext"`
		Binary    string `json:"binary"`
		TUIBinary string `json:"tui_binary"`
	}{t.GOOS, t.GOARCH, t.Ext, t.BinaryName(), t.TUIBinaryName()})
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
