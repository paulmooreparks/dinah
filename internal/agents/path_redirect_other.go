//go:build !windows

package agents

import "os"

func redirectsFileInfo(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
