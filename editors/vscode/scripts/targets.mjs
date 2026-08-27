// The six platform targets and the binary each one carries, plus the
// universal artifact that carries none.
//
// The six are an exact one-to-one match for the six GOOS/GOARCH pairs
// release.yml already builds, so a target added on either side without the
// other turns the packaging check red.

/** One vsix target and the published binary staged into it. */
export const PLATFORM_TARGETS = [
	{ target: "win32-x64", binary: "dinah-windows-amd64.exe", goos: "windows", goarch: "amd64" },
	{ target: "win32-arm64", binary: "dinah-windows-arm64.exe", goos: "windows", goarch: "arm64" },
	{ target: "linux-x64", binary: "dinah-linux-amd64", goos: "linux", goarch: "amd64" },
	{ target: "linux-arm64", binary: "dinah-linux-arm64", goos: "linux", goarch: "arm64" },
	{ target: "darwin-x64", binary: "dinah-darwin-amd64", goos: "darwin", goarch: "amd64" },
	{ target: "darwin-arm64", binary: "dinah-darwin-arm64", goos: "darwin", goarch: "arm64" },
];

/**
 * The universal artifact carries no binary. It exists for the platforms with
 * no specific build, of which alpine-x64, alpine-arm64 and linux-armhf are the
 * ones VS Code names. A user on one of those installs it and supplies their
 * own binary, which is an admitted gap on three platforms rather than a
 * contradiction of the six.
 */
export const UNIVERSAL = "universal";
