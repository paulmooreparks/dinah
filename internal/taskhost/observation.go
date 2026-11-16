package taskhost

import "context"

// PendingQuestions reads only the client's current task generation. The host
// supplies the keys; a consumer cannot manufacture person answer authority.
func (c Client) PendingQuestions(ctx context.Context, scope Scope) ([]Question, error) {
	scope, _, err := c.observe(ctx, scope)
	if err != nil {
		return nil, err
	}
	if c.window.options.PendingQuestions == nil {
		return nil, refusal("unsupported", "pending question inspection is unavailable")
	}
	questions, err := c.window.options.PendingQuestions(ctx, scope)
	if err != nil {
		return nil, err
	}
	for _, q := range questions {
		if q.Service != scope.Service || q.Workspace != scope.Workspace || q.Task != scope.Task || q.Generation != scope.Generation {
			return nil, refusal("not_authorized", "pending question belongs to another task scope")
		}
	}
	if _, _, err := c.observe(ctx, scope); err != nil {
		return nil, err
	}
	if questions == nil {
		questions = []Question{}
	}
	return clone(questions), nil
}

// PublishChange emits an identity and sequence after the consumer saves its
// record. The client's registered identity determines the event namespace.
func (c Client) PublishChange(ctx context.Context, scope Scope, sequence uint64) error {
	scope, _, err := c.observe(ctx, scope)
	if err != nil {
		return err
	}
	if sequence == 0 {
		return refusal("stale", "a saved change requires a positive sequence")
	}
	if c.window.options.SavedChange == nil {
		return refusal("unsupported", "saved change notification is unavailable")
	}
	c.window.options.SavedChange(Change{Service: scope.Service, Workspace: scope.Workspace, Task: scope.Task, Generation: scope.Generation, Sequence: sequence})
	return nil
}
