package docservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"studio/internal/filelock"
)

// Endpoint is how the service is found: its address and its control token,
// in a file in the service's folder, which is the person's own. Whoever can
// read that file can already read the person's documents.
type Endpoint struct {
	Addr    string `json:"addr"`
	Token   string `json:"token,omitempty"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

func endpointFile(dir string) string { return filepath.Join(dir, "service.json") }

func writeEndpoint(dir string, e Endpoint) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return writeWhole(endpointFile(dir), data)
}

// removeEndpoint removes the announcement, if it is still this service's.
func removeEndpoint(dir, token string) {
	if e, err := readEndpoint(dir); err == nil && e.Token == token {
		os.Remove(endpointFile(dir))
	}
}

func readEndpoint(dir string) (Endpoint, error) {
	var e Endpoint
	data, err := os.ReadFile(endpointFile(dir))
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(data, &e); err != nil || e.Addr == "" || e.Token == "" {
		return e, errors.New("the document service's file could not be read")
	}
	return e, nil
}

// Client speaks the service's control interface.
type Client struct {
	Endpoint Endpoint
	// PID is the process a window's questions are asked for, which the
	// service tells whoever wants that window brought forward.
	PID  int
	http *http.Client
}

// Find returns a client of the running service, or an error when none
// answers.
func Find(dir string) (*Client, error) {
	e, err := readEndpoint(dir)
	if err != nil {
		return nil, err
	}
	c := &Client{Endpoint: e, http: &http.Client{}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var pong Endpoint
	if err := c.do(ctx, http.MethodGet, "/ping", nil, &pong); err != nil {
		return nil, err
	}
	return c, nil
}

// Ensure returns a client of the running service, starting one with start
// when none answers. Two programs that start at once take turns here, so
// one starts the service and the other finds it.
func Ensure(dir string, start func() error) (*Client, error) {
	if c, err := Find(dir); err == nil {
		return c, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "start.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	deadline := time.Now().Add(30 * time.Second)
	for {
		held, err := filelock.TryLock(lock)
		if err != nil {
			return nil, err
		}
		if held {
			break
		}
		if time.Now().After(deadline) {
			return nil, errors.New("the document service is still being started by another window")
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer filelock.Unlock(lock)
	if c, err := Find(dir); err == nil {
		return c, nil
	}
	// A file left by a service that ended by force would be found, and
	// fail, until the new one replaces it.
	os.Remove(endpointFile(dir))
	if err := start(); err != nil {
		return nil, err
	}
	for time.Now().Before(deadline) {
		if c, err := Find(dir); err == nil {
			return c, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, errors.New("the document service did not start")
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var reader io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://"+c.Endpoint.Addr+"/control"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Endpoint.Token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		text := strings.TrimSpace(string(data))
		if text == "" {
			text = res.Status
		}
		if res.StatusCode == http.StatusGone {
			return ErrClosed
		}
		return errors.New(text)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// ErrClosed means the document's window is no longer one the service has.
var ErrClosed = errors.New("the document's window has closed")

// Open asks the service for a document's window, for the window process
// that will show it.
func (c *Client) Open(ctx context.Context, r OpenRequest) (Opened, error) {
	var out Opened
	return out, c.do(ctx, http.MethodPost, "/open", r, &out)
}

// Start asks the service to have a window started for a file, or a blank
// one for no file. It is how another program opens a document in a
// stand-alone application.
func (c *Client) Start(ctx context.Context, application, path string) error {
	return c.do(ctx, http.MethodPost, "/start", OpenRequest{Application: application, Path: path}, nil)
}

// Documents lists the open documents.
func (c *Client) Documents(ctx context.Context) ([]Document, error) {
	var out []Document
	return out, c.do(ctx, http.MethodGet, "/documents", nil, &out)
}

// Invoke sends an action to an open document's applet, in the name of from,
// and returns the applet's answer. The document's window asks the person
// first.
func (c *Client) Invoke(ctx context.Context, id string, request json.RawMessage, from string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodPost, "/documents/"+id+"/invoke", map[string]any{"request": request, "from": from}, &out)
	return out, err
}

// Next waits for the commands for a window that come after the one
// numbered after.
func (c *Client) Next(ctx context.Context, id string, after int64) ([]Command, error) {
	var out []Command
	return out, c.do(ctx, http.MethodGet, "/window/"+id+"/next?after="+strconv.FormatInt(after, 10)+"&pid="+strconv.Itoa(c.PID), nil, &out)
}

// Reply answers a command.
func (c *Client) Reply(ctx context.Context, id string, seq int64, result any) error {
	return c.do(ctx, http.MethodPost, "/window/"+id+"/reply", map[string]any{"seq": seq, "result": result}, nil)
}

// Focus asks a document's window to come forward.
func (c *Client) Focus(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/window/"+id+"/focus", struct{}{}, nil)
}

// Entry gets a new way in to a window's page.
func (c *Client) Entry(ctx context.Context, id string) (Opened, error) {
	var out Opened
	return out, c.do(ctx, http.MethodPost, "/window/"+id+"/entry", struct{}{}, &out)
}

// Closing asks whether a window may close, and reports whether the person
// should be asked first.
func (c *Client) Closing(ctx context.Context, id string) (ask bool, err error) {
	var out struct {
		Ask bool `json:"ask"`
	}
	err = c.do(ctx, http.MethodPost, "/window/"+id+"/closing", struct{}{}, &out)
	return out.Ask, err
}

// Closed tells the service a window has closed with the person's leave.
func (c *Client) Closed(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/window/"+id+"/closed", struct{}{}, nil)
}

// Stop ends the service.
func (c *Client) Stop(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/stop", struct{}{}, nil)
}

// String describes the service a client found.
func (c *Client) String() string {
	return fmt.Sprintf("the document service at %s (version %s)", c.Endpoint.Addr, c.Endpoint.Version)
}
