package sessionhost

import (
	_ "embed"
	"encoding/json"
	"sort"
	"time"
)

//go:embed background.go
var BackgroundSource []byte

// BackgroundTask is one piece of an agent's background work as the page
// shows it.
type BackgroundTask struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Summary     string `json:"summary,omitempty"`
	State       string `json:"state"`
	CanStop     bool   `json:"canStop"`
	ToolCallID  string `json:"toolCallId,omitempty"`
	Started     string `json:"started"`
	// seq is the task's place in the order the session's tasks were
	// announced. Two tasks can share a start time, since the clock does not
	// advance between every update.
	seq uint64
}

// backgroundUpdate takes one of the three async task updates into the
// session, and says whether it was one.
func (s *Details) BackgroundUpdate(update json.RawMessage) bool {
	var u struct {
		SessionUpdate string  `json:"sessionUpdate"`
		ID            string  `json:"asyncTaskId"`
		Name          *string `json:"name"`
		TaskType      *string `json:"taskType"`
		Description   *string `json:"description"`
		Summary       *string `json:"summary"`
		State         *string `json:"state"`
		CanStop       *bool   `json:"canStop"`
		ToolCallID    *string `json:"toolCallId"`
	}
	if json.Unmarshal(update, &u) != nil || u.ID == "" {
		return false
	}
	switch u.SessionUpdate {
	case "async_task_spawned", "async_task_progress", "async_task_state_update":
	default:
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.background[u.ID]
	if t == nil {
		if u.SessionUpdate != "async_task_spawned" {
			// Progress for a task this run never saw begin, such as one from
			// before Dinah restarted, is not shown: the adapter's own list
			// decides what is alive, and it announces what it keeps.
			return true
		}
		s.backgroundSeq++
		t = &BackgroundTask{ID: u.ID, State: "running", Started: time.Now().UTC().Format(time.RFC3339Nano), seq: s.backgroundSeq}
		if s.background == nil {
			s.background = map[string]*BackgroundTask{}
		}
		s.background[u.ID] = t
	}
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&t.Name, u.Name)
	set(&t.Type, u.TaskType)
	set(&t.Description, u.Description)
	set(&t.Summary, u.Summary)
	set(&t.ToolCallID, u.ToolCallID)
	set(&t.State, u.State)
	if u.CanStop != nil {
		t.CanStop = *u.CanStop
	}
	switch t.State {
	case "completed", "failed", "stopped":
		delete(s.background, u.ID)
	}
	return true
}

// runningBackground is the session's background work that is not over,
// oldest first. The caller holds s.mu.
func (s *Details) BackgroundLocked() []BackgroundTask {
	out := make([]BackgroundTask, 0, len(s.background))
	for _, t := range s.background {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].seq < out[j].seq })
	return out
}

func (s *Details) BackgroundCountLocked() int { return len(s.background) }
func (s *Details) ClearBackgroundLocked()     { s.background = nil }
func (s *Details) BackgroundTaskLocked(id string) (BackgroundTask, bool) {
	t := s.background[id]
	if t == nil {
		return BackgroundTask{}, false
	}
	return *t, true
}
