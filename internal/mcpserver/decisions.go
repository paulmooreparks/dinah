package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"studio/internal/bus"
)

type personQuestionInput struct {
	Number         int      `json:"number"`
	Text           string   `json:"text"`
	Recommendation string   `json:"recommendation"`
	Choices        []string `json:"choices,omitempty"`
}

type askPersonInput struct {
	OperationID string                `json:"operationId" jsonschema:"caller-generated identity; reuse only for an identical retry"`
	Group       string                `json:"group,omitempty" jsonschema:"optional group address; you must be a member"`
	Questions   []personQuestionInput `json:"questions"`
}

func (s *Server) addAskPersonTool(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "ask_person", Description: "Ask for a decision only the person can make. Always give a recommendation for each numbered question, with optional choices. Never use this for progress reports. This stores a durable request in Decisions and returns its receipt immediately; the person's answers arrive over the bus. Use a unique operationId and reuse it only to retry identical input after a lost receipt. Supply group to keep the decision and answer in that group; otherwise it goes directly to the person."}, func(ctx context.Context, _ *mcp.CallToolRequest, in askPersonInput) (*mcp.CallToolResult, any, error) {
		me, err := s.current(ctx)
		if err != nil {
			return nil, nil, err
		}
		request := bus.AskPersonInput{OperationID: in.OperationID, Group: in.Group, Questions: []bus.PersonQuestion{}}
		for _, q := range in.Questions {
			request.Questions = append(request.Questions, bus.PersonQuestion{Number: q.Number, Text: q.Text, Recommendation: q.Recommendation, Choices: q.Choices})
		}
		result, err := s.bus.AskPerson(ctx, me, request)
		if err != nil {
			return nil, nil, err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return nil, nil, err
		}
		return text("Decision request recorded. Answers arrive automatically; do not poll or repeat the request.\n%s", encoded), nil, nil
	})
}
