package ollamaagent

import "context"

// DefaultContextCap bounds the memory requested for a local model by default.
const DefaultContextCap = 65536

// SetContextCap supplies the Settings limit, read before each model exchange.
// Zero or an invalid saved value uses DefaultContextCap.
func (c *Conn) SetContextCap(limit func() int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.contextCap = limit
}

func (c *Conn) contextLimit() int {
	c.mu.Lock()
	f := c.contextCap
	c.mu.Unlock()
	if f != nil {
		if n := f(); n > 0 {
			return n
		}
	}
	return DefaultContextCap
}

// localContext keeps a saved chat choice intact while applying both ceilings.
// A model without context metadata gets a conservative explicit 4K request.
func localContext(chosen, model, cap int) int {
	n := model
	if n <= 0 {
		n = 4096
	}
	if chosen > 0 {
		n = chosen
		if model > 0 {
			n = min(n, model)
		}
	}
	return min(n, cap)
}

func (c *Conn) localWindow(ctx context.Context, rec record) (int, string, bool) {
	model := 0
	if s, err := c.details(ctx, rec.Model); err == nil {
		model = s.contextLength()
	}
	kind := "model"
	if rec.Context > 0 {
		kind = "chosen"
	}
	return localContext(rec.Context, model, c.contextLimit()), kind, true
}

// contextFullNotice uses the documented token counts, rather than guessing
// what an unenumerated done_reason means. Other reported reasons are shown
// separately by the caller.
func contextFullNotice(used, size int) string {
	if size <= 0 || used < size {
		return ""
	}
	return "The turn ended with its context window full. The task may be unfinished. Use the chat's context controls to make room before continuing."
}
