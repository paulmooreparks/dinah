package agents

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// ResolvePath resolves the links and junctions in a path, through Windows
// rather than by interpreting reparse buffers.
// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlew
func ResolvePath(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buffer := make([]uint16, 512)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return "", err
		}
		if n >= uint32(len(buffer)) {
			buffer = make([]uint16, n+1)
			continue
		}
		resolved := windows.UTF16ToString(buffer[:n])
		if strings.HasPrefix(resolved, `\\?\UNC\`) {
			return `\\` + strings.TrimPrefix(resolved, `\\?\UNC\`), nil
		}
		return strings.TrimPrefix(resolved, `\\?\`), nil
	}
}

// CreateEnvironmentBlock retrieves the user's current environment without
// inheriting the stale environment of this process. Token.Environ frees it.
// https://learn.microsoft.com/en-us/windows/win32/api/userenv/nf-userenv-createenvironmentblock
func installedPath() string {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &token); err != nil {
		return ""
	}
	defer token.Close()
	env, err := token.Environ(false)
	if err != nil {
		return ""
	}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(key, "PATH") {
			return value
		}
	}
	return ""
}
