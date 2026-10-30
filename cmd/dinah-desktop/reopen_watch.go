package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// reopenPatience is how long a step of reopening a chat may wait before the
// log says so. A long chat's replay can take a while; a minute with nothing
// back is a hang worth finding.
const reopenPatience = time.Minute

// stacksSaved keeps the window to one stack file a run, the first hang's.
var stacksSaved sync.Once

// watchReopen logs an error when a step of reopening a chat is still waiting
// after reopenPatience, and again every further reopenPatience, and the first
// time in a run it saves every goroutine's stack beside the log, so the file
// shows whether the window is waiting on the agent or on itself. The
// returned function ends the watch.
func watchReopen(id, step string) (stop func()) {
	began := time.Now()
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(reopenPatience)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
			}
			waited := time.Since(began).Round(time.Second)
			logf("chat", "error", "reopening chat %s: %s has waited %s", id, step, waited)
			stacksSaved.Do(func() { saveStacks(fmt.Sprintf("reopening chat %s: %s had waited %s", id, step, waited)) })
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done) }) }
}

// saveStacks writes every goroutine's stack (runtime.Stack) to a file in the
// log folder, and logs where.
func saveStacks(why string) {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	p := filepath.Join(logDir(), "stacks-"+time.Now().Format("20060102-150405")+".txt")
	if err := os.WriteFile(p, append([]byte(why+"\n\n"), buf...), 0o600); err != nil {
		logf("chat", "error", "saving the window's stacks: %v", err)
		return
	}
	logf("chat", "error", "saved the window's stacks to %s", p)
}
