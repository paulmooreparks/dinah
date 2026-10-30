package builtin

import (
	"context"
	"encoding/json"
	"errors"

	"studio/internal/applet"
)

// Notifications is the applet that posts a message to the person, or asks
// them a question and waits for the answer. Where the question is put is
// the program's: the app's Notifications, or the terminal.
func Notifications() (applet.Bundle, error) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["title","body"],"properties":{"title":{"type":"string","minLength":1,"maxLength":200},"body":{"type":"string","minLength":1,"maxLength":20000}}}`)
	output := `{"type":"object","additionalProperties":false,"required":["title","body"],"properties":{"title":{"type":"string"},"body":{"type":"string"}}}`
	questionOutput := `{"type":"object","additionalProperties":false,"required":["title","body","response"],"properties":{"title":{"type":"string"},"body":{"type":"string"},"response":{"type":"object","additionalProperties":false,"required":["answer"],"properties":{"answer":{"type":"string","minLength":1}}}}}`
	actions := []applet.Action{
		{Ref: "dinah.notifications/post@1", Title: "Post Notification", Description: "Add a durable notification with a title and message. It remains visible until the person dismisses it.", Replay: "pure", Capabilities: []string{"notification.post"}, InputSchema: input, OutputSchema: json.RawMessage(output)},
		{Ref: "dinah.notifications/request@1", Title: "Ask a Question", Description: "Ask the person a question in Notifications and wait for their answer. Agents cannot answer it.", Replay: "human", Capabilities: []string{"attention.request"}, InputSchema: input, OutputSchema: json.RawMessage(questionOutput)},
	}
	handlers := map[string]applet.Handler{}
	for _, action := range actions {
		question := action.Replay == "human"
		handlers[action.Ref] = func(ctx context.Context, input string) (applet.Result, error) {
			if _, ok := applet.InvocationFromContext(ctx); !ok {
				return applet.Result{}, errors.New("a host invocation is required")
			}
			var message struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			}
			if err := json.Unmarshal([]byte(input), &message); err != nil {
				return applet.Result{}, err
			}
			result := applet.Result{OutputJSON: input}
			if question {
				result.Request = &applet.Request{Title: message.Title, Body: message.Body, Fields: []applet.Field{{Name: "answer", Label: "Your Answer", Type: "string", Required: true}}}
			}
			return result, nil
		}
	}
	src, err := sources([]string{"notifications.go"})
	if err != nil {
		return applet.Bundle{}, err
	}
	return applet.Bundle{Manifest: applet.Manifest{ID: "dinah.notifications", Version: "1.1.0", APIVersion: 1, Title: "Notifications", Description: "Post messages or ask the person a question through Notifications.", Actions: actions}, Handlers: handlers, Sources: src}, nil
}
