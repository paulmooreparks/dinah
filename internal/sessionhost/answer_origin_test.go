package sessionhost

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"studio/internal/bus"
)

func TestAnswerOriginDeliveryCopiesAndTranscriptRoundTrip(t *testing.T) {
	origin := &bus.AnswerOrigin{Version: 1, RequestID: "request:1", MessageID: 1, Kind: "request", Questions: []bus.AnsweredQuestion{{Number: 2, Text: "Which?", Answer: "Proceed.", AsRecommended: true}}}
	entries := DeliveredEntries([]bus.Message{{ID: 2, Text: "Question 2. Which? Answer: Proceed.", AnswerOrigin: origin}})
	want := origin.Clone()
	prompt := NewPrompt("bus prompt", nil).WithDelivery("bus", entries, nil)
	origin.Questions[0].Answer = "source changed"
	entries[0].Delivered.AnswerOrigin.Questions[0].Text = "entry changed"
	copied := prompt.Delivered()
	if !reflect.DeepEqual(copied[0].Delivered.AnswerOrigin, want) {
		t.Fatal("delivery aliases source")
	}
	copied[0].Delivered.AnswerOrigin.Questions[0].Answer = "copy changed"
	if !reflect.DeepEqual(prompt.Delivered()[0].Delivered.AnswerOrigin, want) {
		t.Fatal("copy aliases prompt")
	}
	s, c, dir := identifiedFixture(t)
	c.RecordDelivered(s, "bus prompt", prompt.Delivered())
	c.FinishTurn(s, nil, "end_turn")
	transcript, err := ReadTranscript(dir, s.ID())
	if err != nil {
		t.Fatal(err)
	}
	if len(transcript.Messages) < 1 || !reflect.DeepEqual(transcript.Messages[0].Delivered.AnswerOrigin, want) {
		t.Fatalf("transcript %+v", transcript)
	}
}

func TestAnswerOriginArrivingMarkerIsBoundedAndOptional(t *testing.T) {
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx := context.Background()
	asker, err := b.RegisterSession(ctx, "target", t.TempDir(), "Target")
	if err != nil {
		t.Fatal(err)
	}
	request, err := b.AskPerson(ctx, asker, bus.AskPersonInput{OperationID: "arrival", Questions: []bus.PersonQuestion{{Number: 1, Text: strings.Repeat("Question. ", 60), Recommendation: "Proceed."}}})
	if err != nil {
		t.Fatal(err)
	}
	answer, err := b.AnswerDecision(ctx, bus.AnswerDecisionInput{OperationID: "arrival-answer", ID: request.ID, Revision: 1, Answers: []bus.DecisionAnswer{{Number: 1, AsRecommended: true}}})
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := b.Send(ctx, bus.Sender{}, asker.Address(), "Ordinary", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	s := NewConversation(Identity{ID: "target"}, InitialActivity{}, false, false)
	var payload map[string]any
	owner := NewBusChats(nil, nil, BusChatsHost{Emit: func(name string, data any) {
		if name == EventArriving {
			payload = data.(map[string]any)
		}
	}})
	owner.announceArriving(ctx, b, s, asker.ID)
	if payload == nil {
		t.Fatal("no arriving event")
	}
	foundAnswer, foundOrdinary := false, false
	for _, entry := range payload["messages"].([]map[string]any) {
		if _, present := entry["answerOrigin"]; present {
			t.Fatal("preview leaked full snapshots")
		}
		if len([]rune(entry["text"].(string))) > ArrivingText+1 {
			t.Fatal("preview exceeded text bound")
		}
		if entry["id"] == answer.MessageIDs[0] {
			foundAnswer = entry["decisionAnswer"] == true
		}
		if entry["id"] == ordinary.ID {
			_, hasMarker := entry["decisionAnswer"]
			foundOrdinary = !hasMarker
		}
	}
	if !foundAnswer || !foundOrdinary {
		t.Fatalf("markers %+v", payload)
	}
}
