package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultSeed is the seed every Ollama judgement uses, so that the same input
// gives the same answer.
const DefaultSeed = 7

// Ollama is a judge that asks an Ollama server. It runs at temperature 0 with a
// fixed seed, both documented request options of /api/chat, and with thinking
// off, since a judgement is short and a thinking model spends its whole answer
// cap thinking.
type Ollama struct {
	// Host is the server's address, such as http://localhost:11434.
	Host string
	// Model is the model that judges.
	Model string
	// Seed is the seed; zero means DefaultSeed.
	Seed int
	// Client makes the requests; nil means http.DefaultClient.
	Client *http.Client
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Think    bool            `json:"think"`
	Format   string          `json:"format,omitempty"`
	Options  map[string]any  `json:"options"`
}

type ollamaReply struct {
	Message         ollamaMessage `json:"message"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
	Error           string        `json:"error"`
}

// Ask sends the request to the server's /api/chat and returns the answer with
// the tokens counted for it.
func (o *Ollama) Ask(ctx context.Context, r Request) (Response, error) {
	seed := o.Seed
	if seed == 0 {
		seed = DefaultSeed
	}
	resp := Response{Model: o.Model, Temperature: 0, Seed: seed}
	body := ollamaRequest{
		Model: o.Model,
		Messages: []ollamaMessage{
			{Role: "system", Content: r.System},
			{Role: "user", Content: r.Input},
		},
		Options: map[string]any{"temperature": 0, "seed": seed, "num_predict": r.MaxTokens},
	}
	if r.JSON {
		body.Format = "json"
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return resp, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.Host, "/")+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return resp, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := o.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return resp, err
	}
	var reply ollamaReply
	if err := json.Unmarshal(data, &reply); err != nil {
		return resp, fmt.Errorf("ollama answered %s with text that is not JSON", res.Status)
	}
	if res.StatusCode != http.StatusOK {
		if reply.Error != "" {
			return resp, fmt.Errorf("ollama: %s", reply.Error)
		}
		return resp, fmt.Errorf("ollama answered %s", res.Status)
	}
	resp.Text = reply.Message.Content
	resp.Cost = Cost{In: reply.PromptEvalCount, Out: reply.EvalCount}
	return resp, nil
}
