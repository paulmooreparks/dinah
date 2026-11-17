//go:build quality_request_settlement && quality_typed_questions && quality_task_host

package main

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"studio/internal/bus"
	"studio/internal/requestsettlement"
	"studio/internal/taskhost"
)

func (x *requestSettlementFixture) answer(in requestsettlement.Input, source RequestDetails) (any, error) {
	a := x.typed.f.a
	if strings.HasPrefix(in.ID, "task-q-") {
		q := source.Request.(taskhost.Question)
		if q.AnswerType == "" || q.AnswerType == "single" {
			return nil, a.Answer(q.Key, q.Options[0].OptionID)
		}
		value := map[string]any{"type": "text", "text": "Invented answer"}
		if q.AnswerType == "multiple" {
			value = map[string]any{"type": "multiple", "optionIds": []string{q.Options[0].OptionID}}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return nil, a.AnswerTaskQuestion(q.Key, string(raw))
	}
	input := bus.AnswerDecisionInput{ID: in.ID, Revision: in.Revision, OperationID: in.OperationID}
	if request, ok := source.Request.(bus.Decision); ok {
		for _, q := range request.Questions {
			if !q.Answered {
				input.Answers = append(input.Answers, bus.DecisionAnswer{Number: q.Number, Text: "Invented answer"})
			}
		}
	}
	return a.AnswerDecision(input)
}

func (x *requestSettlementFixture) race(in requestsettlement.Input) (any, error) {
	source, err := x.typed.f.a.RequestState(in.ID)
	if err != nil {
		return nil, err
	}
	commands := []string{"Answer", "Dismiss", "Withdraw"}
	if source.Kind == "attention-report" {
		commands = []string{"Dismiss", "Withdraw"}
	}
	start := make(chan struct{})
	results := make([]map[string]any, len(commands))
	var joined sync.WaitGroup
	for i, command := range commands {
		joined.Add(1)
		go func(i int, command string) {
			defer joined.Done()
			<-start
			attempt := in
			attempt.OperationID += "-" + command
			var result any
			var err error
			if command == "Answer" {
				result, err = x.answer(attempt, source)
			} else {
				raw, encodeErr := json.Marshal(attempt)
				if encodeErr != nil {
					err = encodeErr
				} else {
					result, err = x.call(strings.ToLower(command), raw)
				}
			}
			results[i] = map[string]any{"command": command, "result": result}
			if err != nil {
				results[i]["error"] = err.Error()
				var taskError *taskhost.Error
				if errors.As(err, &taskError) {
					results[i]["errorCode"] = taskError.Code
				}
			}
		}(i, command)
	}
	close(start)
	joined.Wait()
	out := map[string]any{"commands": commands, "responses": results}
	if source.Kind == "attention-report" {
		result, err := x.answer(in, source)
		refused := err != nil
		if receipt, ok := result.(bus.DecisionReceipt); ok {
			refused = refused || receipt.State == "rejected"
		}
		if !refused {
			return nil, errors.New("unsupported actual Answer unexpectedly succeeded")
		}
		unsupported := map[string]any{"result": result}
		if err != nil {
			unsupported["error"] = err.Error()
		}
		out["unsupportedAnswer"] = unsupported
	}
	return out, nil
}
