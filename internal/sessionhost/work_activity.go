package sessionhost

import (
	"fmt"
	"strings"
)

// WorkActivity is the shared observation used by status and exit readiness.
type WorkActivity struct {
	Foreground string
	Tasks      int
	Subagents  int
	Detail     string
}

func (a WorkActivity) Busy() bool { return a.Foreground != "" || a.Tasks > 0 || a.Subagents > 0 }

func (s State) WorkActivity() WorkActivity {
	var out WorkActivity
	var name string
	for _, task := range s.Background {
		if !s.Open || s.Parked {
			break
		}
		if task.Type == "turn" && task.State == "running" {
			out.Foreground = "Agent's own turn in progress"
			continue
		}
		out.Tasks++
		if out.Tasks == 1 {
			name = task.Name
		} else {
			name = ""
		}
	}
	if s.Open && !s.Parked {
		out.Subagents = s.RunningSubagents
	}
	switch {
	case s.Working:
		out.Foreground = "Turn in progress"
	case s.Switching:
		out.Foreground = "Switching agent"
	case s.HasSteer:
		out.Foreground = "Steering pending"
	case len(s.Queued) > 0:
		out.Foreground = "Messages queued"
	}
	var parts []string
	if out.Tasks > 0 {
		word := "background task"
		if out.Tasks != 1 {
			word += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", out.Tasks, word))
	}
	if out.Subagents > 0 {
		word := "subagent"
		if out.Subagents != 1 {
			word += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", out.Subagents, word))
	}
	out.Detail = strings.Join(parts, ", ")
	if name != "" {
		out.Detail += ": " + name
	}
	return out
}
