package localfs

import (
	"embed"

	"studio/internal/implsource"
)

//go:embed localfs.go mutate.go hidden_windows.go hidden_other.go
var implementation embed.FS

// ImplementationSource is this provider's source, for the implementation
// digests of the applet actions that use it.
var ImplementationSource = implsource.Of(implementation)
