package verb

import (
	"path/filepath"
	"strconv"
	"time"

	"dinah/internal/durable"
	"dinah/internal/msg"
)

// WaitingNotice renders the notice every head shows each time another retry
// budget passes while an act that has already written part of itself waits
// for the operating system to stop refusing a file. The path is written
// relative to the first of roots that holds it, with forward slashes, and
// stays as it came when none does.
func WaitingNotice(r *msg.Renderer, roots []string, wait durable.Wait) string {
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
	return r.T("notice.durable.waiting", "path", path, "seconds", seconds, "error", last)
}
