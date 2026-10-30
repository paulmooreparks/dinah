package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

// A fatal error or a panic nothing recovers ends the program with the
// runtime's report on standard error, and a program built for the Windows
// GUI has no standard error to read, so the report was lost and the app
// simply disappeared. captureCrashes gives the runtime a file for the report
// (runtime/debug.SetCrashOutput): logs/crash.log, emptied at each start. A
// report the last run left there is first moved to crash-<time>.log, and its
// path is returned so that the person can be told.
func captureCrashes() (previous string) {
	dir := logDir()
	if os.MkdirAll(dir, 0o700) != nil {
		return ""
	}
	pruneCrashReports(dir, 30*24*time.Hour)
	p := filepath.Join(dir, "crash.log")
	if fi, err := os.Stat(p); err == nil && fi.Size() > 0 {
		kept := filepath.Join(dir, "crash-"+fi.ModTime().Format("20060102-150405")+".log")
		if os.Rename(p, kept) == nil {
			previous = kept
		}
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return previous
	}
	// SetCrashOutput duplicates the file, so this handle can be closed.
	_ = debug.SetCrashOutput(f, debug.CrashOptions{})
	_ = f.Close()
	return previous
}

// pruneCrashReports removes the kept crash reports older than maxAge.
func pruneCrashReports(dir string, maxAge time.Duration) {
	old, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	for _, p := range old {
		if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) > maxAge {
			_ = os.Remove(p)
		}
	}
}

// reportPreviousCrash puts a report under Notifications, and an entry in the
// log, when the last run ended in a crash.
func (a *App) reportPreviousCrash(path string) {
	if path == "" {
		return
	}
	raw, _ := os.ReadFile(path)
	first := strings.TrimSpace(strings.SplitN(string(raw), "\n", 2)[0])
	logf("core", "error", "the last run of %s ended in a crash: %s (the report is %s)", product.Name, first, path)
	at := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "crash-"), ".log")
	body := fmt.Sprintf("The runtime said: %s. The whole report is %s; please send it with what you were doing.", first, path)
	_, _ = a.SaveNotification(Notification{ID: "crash-" + at, Kind: "app", Severity: "error",
		Title: product.Name + " stopped unexpectedly last time.", Body: body})
}
