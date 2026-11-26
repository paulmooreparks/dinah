package codexagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// These read-only calls are documented by the app-server guide and the 0.162.0
// generated experimental schema. They observe server-owned execution records;
// OS PIDs and command output never decide whether a task exists or ended.
type observedTask struct{ Name, Kind, Thread string }

// ObserveBackgroundWork enables passive observation for a connection the desktop
// owns. Calls coalesce, and one worker serializes snapshots with turn-end reads.
func (c *Conn) ObserveBackgroundWork(id string) {
	if !c.enableBackgroundObservation(id) {
		return
	}
	c.backgroundOnce.Do(func() { go c.observeBackgroundLoop() })
	select {
	case c.backgroundWake <- struct{}{}:
	default:
	}
}

func (c *Conn) enableBackgroundObservation(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[id]
	if s == nil || s.ephemeral {
		return false
	}
	s.observeBackground = true
	return true
}

func (c *Conn) observeBackgroundLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Done():
			return
		case <-ticker.C:
		case <-c.backgroundWake:
		}
		c.mu.Lock()
		var ids []string
		for id, s := range c.sessions {
			if s.observeBackground && !s.ephemeral {
				ids = append(ids, id)
			}
		}
		c.mu.Unlock()
		for _, id := range ids {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = c.RefreshBackgroundWork(ctx, id)
			cancel()
			select {
			case <-c.Done():
				return
			default:
			}
		}
	}
}

// RefreshBackgroundWork publishes only a complete successful snapshot. A failed
// or unsupported read retains the last observation, never invents a completion.
func (c *Conn) RefreshBackgroundWork(ctx context.Context, id string) error {
	c.backgroundMu.Lock()
	defer c.backgroundMu.Unlock()
	c.mu.Lock()
	s := c.sessions[id]
	c.mu.Unlock()
	if s == nil {
		return errors.New("background observation requires a loaded thread")
	}
	if s.ephemeral {
		return nil
	}
	root, err := native(id)
	if err != nil {
		return err
	}
	tasks, err := c.backgroundSnapshot(ctx, root, s.background)
	if err != nil {
		return err
	}
	c.mu.Lock()
	current := c.sessions[id] == s
	c.mu.Unlock()
	if !current {
		return errors.New("thread changed during background observation")
	}
	prior := s.background
	for key, task := range tasks {
		if old, ok := prior[key]; !ok {
			c.update(id, map[string]any{"sessionUpdate": "async_task_spawned", "asyncTaskId": key, "name": task.Name, "taskType": task.Kind, "canStop": false})
		} else if old != task {
			c.update(id, map[string]any{"sessionUpdate": "async_task_progress", "asyncTaskId": key, "name": task.Name})
		}
	}
	for key := range prior {
		if _, ok := tasks[key]; !ok {
			c.update(id, map[string]any{"sessionUpdate": "async_task_state_update", "asyncTaskId": key, "state": "completed"})
		}
	}
	s.background = tasks
	return nil
}

func (c *Conn) backgroundSnapshot(ctx context.Context, root string, prior map[string]observedTask) (map[string]observedTask, error) {
	tasks := map[string]observedTask{}
	threads := []string{root}
	cursor := ""
	seen := map[string]bool{}
	parents := map[string]string{}
	unknown := map[string]bool{}
	for {
		var page struct {
			Data *[]struct {
				ID, Name, ParentThreadID string
				Status                   struct{ Type string }
			} `json:"data"`
			NextCursor string `json:"nextCursor"`
		}
		err := c.Conn.Call(ctx, "thread/list", map[string]any{"ancestorThreadId": root, "cursor": cursor, "limit": 100,
			"sourceKinds": []string{"subAgent", "subAgentReview", "subAgentCompact", "subAgentThreadSpawn", "subAgentOther"}}, &page)
		if err != nil {
			return nil, err
		}
		if page.Data == nil {
			return nil, errors.New("missing descendant thread data")
		}
		for _, child := range *page.Data {
			if child.ID == "" || child.ID == root {
				return nil, errors.New("invalid descendant identity")
			}
			if child.ParentThreadID == "" {
				return nil, errors.New("missing descendant ownership")
			}
			if prior, ok := parents[child.ID]; ok && prior != child.ParentThreadID {
				return nil, errors.New("conflicting descendant ownership")
			}
			parents[child.ID] = child.ParentThreadID
			switch child.Status.Type {
			case "active":
				name := child.Name
				if strings.TrimSpace(name) == "" {
					name = "Subagent"
				}
				tasks["helper:"+child.ID] = observedTask{Name: name, Kind: "subagent", Thread: child.ID}
				fallthrough
			case "idle":
				threads = append(threads, child.ID)
			case "notLoaded", "systemError":
				unknown[child.ID] = true
			default:
				return nil, fmt.Errorf("unknown descendant status %q", child.Status.Type)
			}
		}
		if page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return nil, errors.New("repeated descendant cursor")
		}
		seen[page.NextCursor], cursor = true, page.NextCursor
	}
	for id := range parents {
		visited := map[string]bool{}
		for id != root {
			if id == "" || visited[id] {
				return nil, errors.New("unproven descendant ownership")
			}
			visited[id] = true
			id = parents[id]
		}
	}
	// An unloaded, missing or errored child does not prove that previously
	// reported work ended. Retain it until a live observation can settle it.
	for key, task := range prior {
		if _, listed := parents[task.Thread]; task.Thread != root && (!listed || unknown[task.Thread]) {
			tasks[key] = task
		}
	}
	for _, thread := range threads {
		cursor, seen = "", map[string]bool{}
		for {
			var page struct {
				Data       *[]struct{ ProcessID, Command string } `json:"data"`
				NextCursor string                                 `json:"nextCursor"`
			}
			if err := c.Conn.Call(ctx, "thread/backgroundTerminals/list", map[string]any{"threadId": thread, "cursor": cursor, "limit": 100}, &page); err != nil {
				return nil, err
			}
			if page.Data == nil {
				return nil, errors.New("missing background terminal data")
			}
			for _, terminal := range *page.Data {
				if terminal.ProcessID == "" {
					return nil, errors.New("missing background terminal identity")
				}
				pair, _ := json.Marshal([]string{thread, terminal.ProcessID})
				key := "terminal:" + string(pair)
				tasks[key] = observedTask{Name: strings.TrimSpace(terminal.Command), Kind: "shell", Thread: thread}
			}
			if page.NextCursor == "" {
				break
			}
			if seen[page.NextCursor] {
				return nil, errors.New("repeated terminal cursor")
			}
			seen[page.NextCursor], cursor = true, page.NextCursor
		}
	}
	return tasks, nil
}
