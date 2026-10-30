// Package ollamaagent connects Dinah to Ollama as an agent provider.
// Ollama answers chat requests and asks for tool calls but runs no tools and
// keeps no conversations, so this package runs the tool loop, carries out
// the tools through the host, and keeps each conversation itself. It speaks
// the same ACP-shaped interface as the other providers, so the workbench
// treats it as it treats Devin and Codex.
//
// Everything here follows Ollama's documented HTTP API
// (https://docs.ollama.com/api/chat, /api/tags, /api/show, and the
// capability pages on tool calling, thinking and context length).
package ollamaagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultHost is Ollama's documented local address.
const DefaultHost = "http://localhost:11434"

// Host is the Ollama server to use: OLLAMA_HOST, Ollama's own setting for
// where its server is, or the local default.
func Host() string {
	h := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if h == "" {
		return DefaultHost
	}
	if !strings.Contains(h, "://") {
		h = "http://" + h
	}
	if u, err := url.Parse(h); err == nil && u.Port() == "" && u.Scheme == "http" {
		u.Host += ":11434"
		h = u.String()
	}
	return strings.TrimRight(h, "/")
}

// client is a small client of Ollama's API.
type client struct {
	base string
	http *http.Client
}

func newClient(base string) *client {
	return &client{base: strings.TrimRight(base, "/"), http: &http.Client{}}
}

func (c *client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama did not answer at %s: %w", c.base, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return apiError(res)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// apiError is Ollama's error body, {"error": "..."}, or the status.
func apiError(res *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(b, &e) == nil && e.Error != "" {
		return errors.New("Ollama: " + e.Error)
	}
	return fmt.Errorf("Ollama answered %s", res.Status)
}

func (c *client) version(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"version"`
	}
	err := c.do(ctx, http.MethodGet, "/api/version", nil, &v)
	return v.Version, err
}

// tagged is one model of /api/tags. A model with a remote host runs in
// Ollama's cloud rather than on this computer.
type tagged struct {
	Name        string `json:"name"`
	Model       string `json:"model"`
	RemoteModel string `json:"remote_model"`
	RemoteHost  string `json:"remote_host"`
	Details     struct {
		ParameterSize string `json:"parameter_size"`
	} `json:"details"`
}

func (c *client) tags(ctx context.Context) ([]tagged, error) {
	var r struct {
		Models []tagged `json:"models"`
	}
	err := c.do(ctx, http.MethodGet, "/api/tags", nil, &r)
	return r.Models, err
}

// LoadedModels returns the names of the models the server at base has loaded in
// memory now, which /api/ps documents. Running one of them loads nothing new.
func LoadedModels(ctx context.Context, base string) ([]string, error) {
	var r struct {
		Models []tagged `json:"models"`
	}
	if err := newClient(base).do(ctx, http.MethodGet, "/api/ps", nil, &r); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(r.Models))
	for _, m := range r.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// ListModels returns the names of the models the server at base has installed.
func ListModels(ctx context.Context, base string) ([]string, error) {
	models, err := newClient(base).tags(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(models))
	for _, m := range models {
		names = append(names, m.Name)
	}
	return names, nil
}

// shown is what /api/show says of a model that matters here: what it can
// do, its thinking controls, and the context length it was trained for.
type shown struct {
	Capabilities []string `json:"capabilities"`
	Thinking     *struct {
		Values  []any `json:"values"`
		Default any   `json:"default"`
	} `json:"thinking"`
	ModelInfo map[string]any `json:"model_info"`
}

func (s shown) can(capability string) bool {
	for _, c := range s.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// contextLength is the model's <architecture>.context_length, or 0.
func (s shown) contextLength() int {
	arch, _ := s.ModelInfo["general.architecture"].(string)
	if n, ok := s.ModelInfo[arch+".context_length"].(float64); ok && arch != "" {
		return int(n)
	}
	return 0
}

func (c *client) show(ctx context.Context, model string) (shown, error) {
	var s shown
	err := c.do(ctx, http.MethodPost, "/api/show", map[string]string{"model": model}, &s)
	return s, err
}

// message is one message of a chat, as /api/chat takes and gives it.
type message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Thinking  string     `json:"thinking,omitempty"`
	Images    []string   `json:"images,omitempty"`
	ToolCalls []toolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`

	// Turn marks the message that begins a turn, Cost is what the turn added to
	// the conversation as Ollama counted it, recorded on that message when the
	// turn ends, and Aside says the message is set aside: the request leaves it
	// out and the record keeps it. They are Dinah's and are never sent.
	Turn  bool `json:"turn,omitempty"`
	Cost  int  `json:"cost,omitempty"`
	Aside bool `json:"aside,omitempty"`
}

type toolCall struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []message      `json:"messages"`
	Tools    []any          `json:"tools,omitempty"`
	Stream   bool           `json:"stream"`
	Think    any            `json:"think,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
}

// chunk is one line of a streamed chat answer; the last has done set and
// the token counts.
type chunk struct {
	Message         message `json:"message"`
	Done            bool    `json:"done"`
	DoneReason      string  `json:"done_reason"`
	PromptEvalCount int     `json:"prompt_eval_count"`
	EvalCount       int     `json:"eval_count"`
	Error           string  `json:"error"`
}

// chat streams one answer, calling each for every chunk, and returns the
// whole assistant message with its tool calls, and the final chunk.
func (c *client) chat(ctx context.Context, req chatRequest, each func(chunk)) (message, chunk, error) {
	req.Stream = true
	b, err := json.Marshal(req)
	if err != nil {
		return message{}, chunk{}, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return message{}, chunk{}, err
	}
	r.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(r)
	if err != nil {
		return message{}, chunk{}, fmt.Errorf("Ollama did not answer at %s: %w", c.base, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return message{}, chunk{}, apiError(res)
	}
	whole := message{Role: "assistant"}
	var content, thinking strings.Builder
	var last chunk
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var ch chunk
		if err := json.Unmarshal(line, &ch); err != nil {
			return message{}, chunk{}, fmt.Errorf("Ollama sent a line that could not be read: %w", err)
		}
		if ch.Error != "" {
			return message{}, chunk{}, errors.New("Ollama: " + ch.Error)
		}
		content.WriteString(ch.Message.Content)
		thinking.WriteString(ch.Message.Thinking)
		// Tool calls are gathered across the stream, as Ollama's tool
		// calling guide does.
		whole.ToolCalls = append(whole.ToolCalls, ch.Message.ToolCalls...)
		if each != nil {
			each(ch)
		}
		if ch.Done {
			last = ch
			break
		}
	}
	if err := sc.Err(); err != nil {
		return message{}, chunk{}, err
	}
	if !last.Done {
		if ctx.Err() != nil {
			return message{}, chunk{}, ctx.Err()
		}
		return message{}, chunk{}, errors.New("Ollama ended the answer before it was done")
	}
	whole.Content, whole.Thinking = content.String(), thinking.String()
	return whole, last, nil
}

// probe says whether Ollama answers at base within a short time.
func probe(base string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return newClient(base).version(ctx)
}

// Probe reports Ollama's version at its host, or why it did not answer.
func Probe() (string, error) { return probe(Host()) }
