//go:build !windows && !linux && !darwin

package agents

import "os"

func snapshotOpenedIdentity(_ *os.File, info os.FileInfo) (os.FileInfo, error) { return info, nil }
