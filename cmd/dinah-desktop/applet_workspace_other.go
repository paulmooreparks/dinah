//go:build !windows

package main

import (
	_ "embed"
	"os"

	"studio/internal/resource/localfs"
)

//go:embed applet_workspace_other.go
var workspaceAttributeSource []byte

// The attribute checks are the local folder provider's; these names remain for
// the callers in this package.
func workspacePathHidden(path string) (bool, error) { return localfs.PathHidden(path) }
func workspaceFileHidden(f *os.File) (bool, error)  { return localfs.FileHidden(f) }
