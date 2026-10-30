//go:build windows

package agents

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// DevinDesktop is the installed Devin Desktop's name and version as Windows
// lists it among the installed programs, or "" when it is not listed. It
// reads the uninstall entries that Settings › Apps reads
// (https://learn.microsoft.com/windows/win32/msi/uninstall-registry-key), for
// the machine and for the person, and starts nothing. It only shapes what
// DevinMissing says; nothing is found through it.
func DevinDesktop() string {
	roots := []struct {
		key  registry.Key
		path string
	}{
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	}
	for _, root := range roots {
		k, err := registry.OpenKey(root.key, root.path, registry.ENUMERATE_SUB_KEYS|registry.READ)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, name := range names {
			entry, err := registry.OpenKey(root.key, root.path+`\`+name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			display, _, _ := entry.GetStringValue("DisplayName")
			version, _, _ := entry.GetStringValue("DisplayVersion")
			entry.Close()
			// Devin Desktop is listed as "Devin", with "(User)" for an install
			// for one person; a separately installed command line is not it.
			lower := strings.ToLower(strings.TrimSpace(display))
			if strings.HasPrefix(lower, "devin") && !strings.Contains(lower, "cli") {
				if version != "" {
					return strings.TrimSpace(display) + " " + version
				}
				return strings.TrimSpace(display)
			}
		}
	}
	return ""
}
