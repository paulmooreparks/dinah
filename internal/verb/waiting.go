package verb

import (
	"path/filepath"
	"strconv"
	"time"

	"dinah/internal/durable"
	"dinah/internal/msg"
)

// WaitingNoticeKey is the catalog key of the notice every head shows each time
// another retry budget passes while an act that has already written part of
// itself waits for the operating system to stop refusing a file.
const WaitingNoticeKey = "notice.durable.waiting"

// WaitingNotice renders that notice in r's language.
func WaitingNotice(r *msg.Renderer, roots []string, wait durable.Wait) string {
	return r.T(WaitingNoticeKey, WaitingNoticeValues(roots, wait)...)
}

// WaitingNoticeValues answers the named values the notice fills: the path,
// written relative to the first of roots that holds it with forward slashes
// and as it came when none does, the whole seconds waited, and the last error
// the operating system gave. A head that renders through its own catalogue
// call hands these to it.
func WaitingNoticeValues(roots []string, wait durable.Wait) []string {
	path := wait.Path
	for _, root := range roots {
		if root == "" {
			continue
		}
		relative, err := filepath.Rel(root, wait.Path)
		if err == nil && filepath.IsLocal(relative) {
			path = filepath.ToSlash(relative)
			break
		}
	}
	seconds := strconv.Itoa(int(wait.Elapsed / time.Second))
	last := ""
	if wait.Last != nil {
		last = wait.Last.Error()
	}
	return []string{"path", path, "seconds", seconds, "error", last}
}
