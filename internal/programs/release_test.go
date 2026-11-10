package programs

import (
	"runtime"
	"testing"
)

func TestTheReleaseIsWhatTheProgramSaid(t *testing.T) {
	defer SetRelease(Release())
	SetRelease("")
	if got := Release(); got != "" {
		t.Fatalf("a program that has not said its release has %q", got)
	}
	SetRelease("1.2.3")
	if got := Release(); got != "1.2.3" {
		t.Fatalf("the release is %q", got)
	}
}

func TestAFileNameHasThePlatformsEnding(t *testing.T) {
	want := "tool"
	if runtime.GOOS == "windows" {
		want = "tool.exe"
	}
	if got := FileName("tool"); got != want {
		t.Fatalf("the file name is %q", got)
	}
}
