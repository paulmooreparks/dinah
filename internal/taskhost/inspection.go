package taskhost

import "context"

func (c Client) observe(ctx context.Context, scope Scope) (Scope, Binding, error) {
	scope, err := c.scope(scope)
	if err != nil {
		return Scope{}, Binding{}, err
	}
	c.window.mu.Lock()
	closed := c.window.closed
	c.window.mu.Unlock()
	if closed {
		return Scope{}, Binding{}, refusal("unavailable", "task admission is closed")
	}
	binding, err := c.window.check(ctx, scope)
	return scope, binding, err
}

func (c Client) Inspect(ctx context.Context, scope Scope) (Inspection, error) {
	scope, binding, err := c.observe(ctx, scope)
	if err != nil {
		return Inspection{}, err
	}
	ops := []string{}
	if c.window.options.Operations != nil {
		ops, err = c.window.options.Operations(ctx, scope)
		if err != nil {
			return Inspection{}, err
		}
	}
	return clone(Inspection{Scope: scope, Binding: binding, Operations: ops, Rooms: c.window.roomsAvailable()}), nil
}

func (c Client) DescribeChats(ctx context.Context, scope Scope) ([]ChatDescription, error) {
	scope, _, err := c.observe(ctx, scope)
	if err != nil {
		return nil, err
	}
	if c.window.options.DescribeChats == nil {
		return nil, refusal("unsupported", "local chat inspection is unavailable")
	}
	value, err := c.window.options.DescribeChats(ctx, scope)
	return clone(value), err
}

func (c Client) CheckProgram(ctx context.Context, scope Scope, program string) (ProgramDescription, error) {
	scope, _, err := c.observe(ctx, scope)
	if err != nil {
		return ProgramDescription{}, err
	}
	if c.window.options.CheckProgram == nil {
		return ProgramDescription{}, refusal("unsupported", "local program inspection is unavailable")
	}
	return c.window.options.CheckProgram(ctx, scope, program)
}

func (c Client) DescribeAction(ctx context.Context, scope Scope, reference string) (ActionDescription, error) {
	scope, _, err := c.observe(ctx, scope)
	if err != nil {
		return ActionDescription{}, err
	}
	if c.window.options.DescribeAction == nil {
		return ActionDescription{}, refusal("unsupported", "local action inspection is unavailable")
	}
	value, err := c.window.options.DescribeAction(ctx, scope, reference)
	return clone(value), err
}
