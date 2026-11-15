package sessionhost

import (
	"errors"
	"sync"
)

// HoldRuntime retains a loaded conversation without creating a page window.
// Release returns it to the ordinary closed-session policy without deleting history.
func (c *Conversation) HoldRuntime() (func(), error) {
	c.mu.Lock()
	if !c.activity.OpenLocked() || c.activity.ending != nil {
		c.mu.Unlock()
		return nil, errors.New("the conversation is closed or ending")
	}
	c.details.runtimeInterests++
	c.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			c.details.runtimeInterests--
			c.mu.Unlock()
		})
	}, nil
}
