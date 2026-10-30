package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"studio/internal/applet/broker"
	"studio/internal/rotlog"
)

// The log: one line for each entry in
// logs/dinah.log in Dinah's own folder, with its time, its
// level and the part of Dinah it came from, such as core, an applet
// (applet:dinah.settings), the Terminal, a script or the page.
// Settings › General sets how much is kept: nothing, errors and warnings, or
// everything. Entries are buffered and written a moment later, through
// package rotlog; when the file would pass logMaxSize it becomes
// dinah.1.log, and so on up to logKeep files; the oldest is removed.

// The log levels, from the setting.
const (
	LogOff      = "off"
	LogProblems = "problems" // errors and warnings; the default
	LogAll      = "all"
	// LogDebug keeps everything LogAll does and the debug entries of work that
	// would otherwise fill the log, such as a pane's waits for a change; it is
	// set in the settings file and has no control.
	LogDebug = "debug"
)

const (
	logMaxSize = 4 << 20
	logKeep    = 4
)

// logSource is the shape of an entry's source: core, terminal, ui,
// applet:<id>, script:<name> and the like.
var logSource = regexp.MustCompile(`^[a-z][a-z0-9-]*(:[A-Za-z0-9._@/-]{1,128})?$`)

var appLog struct {
	sync.Mutex
	level string
	// w is the log file, opened on the first entry, for the folder logDir
	// gave then; dir is that folder, so that tests with another one get
	// their own file.
	w   *rotlog.Writer
	dir string
	// closedDir is the folder whose log was closed as its app ended, which later entries
	// are not written to.
	closedDir string
}

// openAppLog lets the log of the current folder be written, as an app starts in it.
func openAppLog() {
	appLog.Lock()
	appLog.closedDir = ""
	appLog.Unlock()
}

// closeAppLog writes the entries the log holds and takes no more for its folder, as the
// app ends: a goroutine still running then writes nothing into a folder the app has
// finished with.
func closeAppLog() {
	appLog.Lock()
	defer appLog.Unlock()
	appLog.closedDir = logDir()
	if appLog.w != nil {
		_ = appLog.w.Close()
		appLog.w = nil
	}
}

// flushAppLog writes the entries the log holds, as the app ends.
func flushAppLog() {
	appLog.Lock()
	defer appLog.Unlock()
	if appLog.w != nil {
		_ = appLog.w.Flush()
	}
}

func setLogLevel(level string) {
	appLog.Lock()
	appLog.level = level
	appLog.Unlock()
}

func validLogLevel(level string) bool {
	return level == LogOff || level == LogProblems || level == LogAll || level == LogDebug
}

func logDir() string { return filepath.Join(configDir(), "logs") }

// acpTracePath is the trace of the Agent Client Protocol traffic with Devin,
// beside the log, which each start of Devin writes afresh.
func acpTracePath() string { return filepath.Join(logDir(), "acp-trace.log") }

// logEntry adds an entry to the log when the setting keeps entries of its
// level, which is error, warning, info or debug. The log never fails what it
// records, so an entry that cannot be written is dropped.
func logEntry(source, level, message string) {
	appLog.Lock()
	defer appLog.Unlock()
	switch appLog.level {
	case LogAll, LogDebug:
		if level == "debug" && appLog.level != LogDebug {
			return
		}
	case LogProblems, "":
		if level == "debug" {
			return
		}
		if level == "info" {
			return
		}
	default:
		return
	}
	if !logSource.MatchString(source) {
		source = "core"
	}
	// One entry is one line.
	message = strings.NewReplacer("\r\n", " | ", "\n", " | ", "\r", " | ").Replace(strings.TrimSpace(message))
	line := fmt.Sprintf("%s %-5s [%s] %s\n", time.Now().Format("2006-01-02T15:04:05.000Z07:00"), strings.ToUpper(level), source, message)
	dir := logDir()
	if dir == appLog.closedDir {
		return
	}
	if appLog.w == nil || appLog.dir != dir {
		if appLog.w != nil {
			_ = appLog.w.Close()
		}
		appLog.w = rotlog.Open(filepath.Join(dir, product.LogFile), rotlog.Options{MaxSize: logMaxSize, Keep: logKeep})
		appLog.dir = dir
	}
	_, _ = appLog.w.WriteString(line)
	// An error is written at once, so that it is in the file even if the app
	// ends before the buffer is written.
	if level == "error" {
		_ = appLog.w.Flush()
	}
}

func logf(source, level, format string, args ...any) {
	logEntry(source, level, fmt.Sprintf(format, args...))
}

// Log adds an entry from the page: the Terminal, a script, an applet's view
// or the page's own errors.
func (a *App) Log(source, level, message string) error {
	if level != "error" && level != "warning" && level != "info" {
		return errors.New("a log entry's level is error, warning or info")
	}
	if !logSource.MatchString(source) {
		return errors.New("a log entry's source is a word such as terminal, or a kind and a name such as script:wiki")
	}
	if len(message) > 16<<10 {
		message = message[:16<<10] + "…"
	}
	logEntry(source, level, message)
	return nil
}

// SetLogLevel saves how much the log keeps.
func (a *App) SetLogLevel(level string) error {
	if !validLogLevel(level) {
		return errors.New("the log level is off, problems or all")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.LogLevel = level
		return nil
	}); err != nil {
		return err
	}
	setLogLevel(level)
	logf("core", "info", "the log level is now %s", level)
	a.emitSettingsChanged()
	return nil
}

// ShowLogFolder shows the log's folder in the system's file manager.
func (a *App) ShowLogFolder() error {
	dir := logDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, product.LogFile)
	if _, err := os.Stat(path); err != nil {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			f.Close()
		}
	}
	return showInFolder(path)
}

// logAppletEvent records an applet action as it starts, waits for a person
// and ends, under the applet it belongs to.
func logAppletEvent(name string, data any) {
	switch name {
	case "applet:error":
		if m, ok := data.(map[string]string); ok {
			logf("core", "error", "an applet action %s could not be saved: %s", m["id"], m["error"])
		}
	case "applet:invocation":
		v, ok := data.(broker.Invocation)
		if !ok {
			return
		}
		source := "applet"
		if i := strings.Index(v.Action, "/"); i > 0 {
			source = "applet:" + v.Action[:i]
		}
		who := v.OwnerKind
		if v.OwnerID != "" {
			who += " " + v.OwnerID
		}
		switch v.Status {
		case "failed", "timeout":
			logf(source, "error", "%s %s for %s: %s", v.Action, v.Status, who, v.Error)
		case "cancelled":
			logf(source, "warning", "%s cancelled for %s", v.Action, who)
		case "done":
			logf(source, "info", "%s done for %s in %s", v.Action, who, v.Updated.Sub(v.Created).Round(time.Millisecond))
		default:
			logf(source, "info", "%s %s for %s", v.Action, v.Status, who)
		}
	}
}
