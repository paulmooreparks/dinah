package antigravityagent

import "testing"

// The command line writes a Windows folder with a slash before its drive.
func TestFolderDropsTheSlashBeforeADrive(t *testing.T) {
	for in, want := range map[string]string{
		"/C:/Users/paul/source/repos/studio": "C:/Users/paul/source/repos/studio",
		"/c:":                                "c:",
		"C:/Users/paul":                      "C:/Users/paul",
		"/home/paul/work":                    "/home/paul/work",
		"":                                   "",
	} {
		if got := folder(in); got != want {
			t.Errorf("folder(%q) = %q, want %q", in, got, want)
		}
	}
}
