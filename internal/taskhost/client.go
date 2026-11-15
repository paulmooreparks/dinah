package taskhost

import "context"

// Client fixes the registered service identity at construction. Request JSON
// cannot select another service's reviews, tasks or receipts.
type Client struct {
	window  *Window
	service string
}

func (w *Window) ForService(service string) Client { return Client{window: w, service: service} }

func (c Client) scope(s Scope) (Scope, error) {
	if c.window == nil || c.service == "" || (s.Service != "" && s.Service != c.service) {
		return Scope{}, refusal("not_authorized", "foreign service identity")
	}
	s.Service = c.service
	return s, nil
}

func (c Client) Review(ctx context.Context, p Plan) (Review, error) {
	s, err := c.scope(p.Scope)
	if err != nil {
		return Review{}, err
	}
	p.Scope = s
	return c.window.Review(ctx, p)
}

func (c Client) Begin(ctx context.Context, id string) (*Handle, error) {
	if c.window == nil {
		return nil, refusal("unavailable", "no window binding")
	}
	c.window.mu.Lock()
	r := c.window.reviews[id]
	c.window.mu.Unlock()
	if r.Plan.Scope.Service != c.service {
		return nil, refusal("not_authorized", "foreign service review")
	}
	return c.window.Begin(ctx, id)
}

func (c Client) Start(ctx context.Context, h *Handle, o Operation) (Receipt, error) {
	if h == nil || h.lease == nil || h.lease.plan.Scope.Service != c.service {
		return Receipt{}, refusal("not_authorized", "foreign service handle")
	}
	return c.window.Start(ctx, h, o)
}

func (c Client) Rebind(ctx context.Context, s Scope) (Recovery, error) {
	s, err := c.scope(s)
	if err != nil {
		return Recovery{}, err
	}
	return c.window.Rebind(ctx, s)
}

func (c Client) Read(ctx context.Context, s Scope, operation string) (Receipt, error) {
	s, _, err := c.observe(ctx, s)
	if err != nil {
		return Receipt{}, err
	}
	return c.window.Read(ctx, s, operation)
}

func (c Client) Cancel(h *Handle) error {
	if h == nil || h.lease == nil || h.lease.plan.Scope.Service != c.service {
		return refusal("not_authorized", "foreign service handle")
	}
	return c.window.Cancel(h)
}
