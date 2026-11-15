package diagrampicture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"studio/internal/helpercompat"
)

var policy = helpercompat.For(helpercompat.DiagramPicture)

type Client struct {
	endpoint, lease, workspace string
	http                       *http.Client
}

func New(endpoint, lease, workspaceID string) (*Client, error) {
	if lease == "" {
		return nil, refused(403, "picture_lease_unavailable", managedMessage)
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || (u.Path != "" && u.Path != "/") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || workspaceID == "" || strings.ContainsAny(lease, "\r\n") {
		return nil, errors.New("invalid diagram picture endpoint or authority")
	}
	return &Client{endpoint: strings.TrimRight(endpoint, "/"), lease: lease, workspace: workspaceID, http: &http.Client{
		Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("drawing redirects are forbidden") },
	}}, nil
}
func (c *Client) request(ctx context.Context, method, route string, body []byte) (*http.Response, error) {
	r, err := http.NewRequestWithContext(ctx, method, c.endpoint+route, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+c.lease)
	r.Header.Set(helpercompat.ChannelHeader, "diagram-picture")
	r.Header.Set(helpercompat.OfferHeader, policy.Offer())
	r.Header.Set("Content-Type", "application/json")
	return c.http.Do(r)
}
func readResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponse+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxResponse {
		return nil, errors.New("drawing response exceeds its limit")
	}
	return b, nil
}
func responseError(resp *http.Response, b []byte) error {
	if resp.StatusCode == 404 {
		return refused(503, "picture_service_unavailable", managedMessage)
	}
	var r Refusal
	if json.Unmarshal(b, &r) != nil || r.Code == "" || r.Message == "" {
		return refused(resp.StatusCode, "invalid_response", "The window refused the picture request.")
	}
	r.HTTPStatus = resp.StatusCode
	return &r
}
func (c *Client) Draw(ctx context.Context, r Request) (Picture, error) {
	if err := validateRequest(r); err != nil {
		return Picture{}, err
	}
	if r.WorkspaceID != c.workspace {
		return Picture{}, refused(403, "workspace_refused", "The picture lease belongs to another workspace.")
	}
	body, err := json.Marshal(r)
	if err != nil {
		return Picture{}, err
	}
	if len(body) > MaxRequest {
		return Picture{}, refused(413, "request_limit", "The diagram request exceeds 64 KiB.")
	}
	probe, cancel := context.WithTimeout(ctx, time.Second)
	resp, err := c.request(probe, http.MethodOptions, Route, nil)
	if err != nil {
		cancel()
		return Picture{}, refused(503, "window_unavailable", managedMessage)
	}
	b, err := readResponse(resp)
	cancel()
	if err != nil {
		return Picture{}, err
	}
	if resp.StatusCode != 204 {
		return Picture{}, responseError(resp, b)
	}
	if _, err = policy.Response(resp); err != nil {
		return Picture{}, err
	}
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err = c.request(ctx, http.MethodPost, Route, body)
	if err != nil {
		// The request context cancels the transport. The host reads the body
		// before drawing and disposes the page on connection cancellation.
		return Picture{}, err
	}
	b, err = readResponse(resp)
	if err != nil {
		return Picture{}, err
	}
	if _, err = policy.Response(resp); err != nil {
		return Picture{}, err
	}
	if resp.StatusCode != 200 {
		return Picture{}, responseError(resp, b)
	}
	var p Picture
	if err = json.Unmarshal(b, &p); err != nil {
		return Picture{}, err
	}
	if err = validatePicture(r, p); err != nil {
		return Picture{}, err
	}
	return p, nil
}
