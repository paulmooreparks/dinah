//go:build windows

package main

import "testing"

func TestWindowsNotificationLeavesActivationToNativeNotifier(t *testing.T) {
	if callback := (&App{}).hostNoticeActivation(); callback != nil {
		t.Fatal("Windows notification would invoke an earlier Wails restore")
	}
}
