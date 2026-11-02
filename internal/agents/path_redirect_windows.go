package agents

import (
	"os"
	"syscall"
)

func redirectsFileInfo(info os.FileInfo) bool {
	attrs, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return !ok || info.Mode()&os.ModeSymlink != 0 || attrs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
