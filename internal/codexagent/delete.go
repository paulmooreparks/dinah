package codexagent

import (
	"context"
	"errors"
	"sort"

	"studio/internal/acp"
)

// deleteSession implements the documented permanent thread/delete operation.
// The native operation also deletes spawned descendants. Fork provenance is
// deliberately absent from this traversal because forks are separate roots.
func (c *Conn) deleteSession(ctx context.Context, params any) error {
	var p acp.DeleteSessionParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	id, err := native(p.SessionID)
	if err != nil {
		return err
	}
	c.mu.Lock()
	guard := c.deletionGuard
	busy := c.sessions[p.SessionID] != nil && c.sessions[p.SessionID].completion != nil
	c.mu.Unlock()
	if guard == nil {
		return errors.New("Codex deletion ownership check is unavailable")
	}
	if busy {
		return errors.New("wait for the Codex turn to finish before deleting this chat")
	}
	descendants, err := c.deletionDescendants(ctx, id)
	if err != nil {
		return err
	}
	if err := guard(ctx, p.SessionID, descendants); err != nil {
		return err
	}
	if err := c.Conn.Call(ctx, "thread/delete", map[string]string{"threadId": id}, nil); err != nil {
		return err
	}
	c.mu.Lock()
	delete(c.sessions, p.SessionID)
	for _, descendant := range descendants {
		delete(c.sessions, descendant)
	}
	c.mu.Unlock()
	return nil
}

func (c *Conn) deletionDescendants(ctx context.Context, root string) ([]string, error) {
	parents := map[string]string{}
	for _, archived := range []bool{false, true} {
		cursor, cursors := "", map[string]bool{}
		for page := 0; ; page++ {
			if page >= 1000 {
				return nil, errors.New("Codex deletion inventory exceeds its bound")
			}
			q := map[string]any{"limit": 100, "archived": archived,
				"sourceKinds": []string{"cli", "vscode", "exec", "appServer", "subAgent", "subAgentReview", "subAgentCompact", "subAgentThreadSpawn", "subAgentOther", "unknown"}}
			if cursor != "" {
				q["cursor"] = cursor
			}
			var r struct {
				Data []struct {
					ID     string `json:"id"`
					Parent string `json:"parentThreadId"`
				} `json:"data"`
				Next string `json:"nextCursor"`
			}
			if err := c.Conn.Call(ctx, "thread/list", q, &r); err != nil {
				return nil, err
			}
			for _, thread := range r.Data {
				if _, err := native("codex:" + thread.ID); err != nil {
					return nil, err
				}
				if thread.Parent != "" {
					if _, err := native("codex:" + thread.Parent); err != nil {
						return nil, err
					}
				}
				if previous, ok := parents[thread.ID]; ok && previous != thread.Parent {
					return nil, errors.New("Codex deletion inventory has conflicting parents")
				}
				parents[thread.ID] = thread.Parent
			}
			if r.Next == "" {
				break
			}
			if cursors[r.Next] {
				return nil, errors.New("Codex repeated the deletion inventory cursor")
			}
			cursors[r.Next], cursor = true, r.Next
		}
	}
	children := map[string][]string{}
	for id, parent := range parents {
		if parent != "" {
			children[parent] = append(children[parent], id)
		}
	}
	seen, pending, result := map[string]bool{root: true}, []string{root}, []string{}
	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]
		for _, child := range children[id] {
			if seen[child] {
				return nil, errors.New("Codex deletion inventory has cyclic ownership")
			}
			seen[child] = true
			result, pending = append(result, "codex:"+child), append(pending, child)
		}
	}
	sort.Strings(result)
	return result, nil
}
